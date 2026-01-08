package modelsocket

import (
	"context"
	"log/slog"
	"sync"

	"github.com/google/uuid"
)

// Client is the main client for connecting to a ModelSocket server.
// It is safe for concurrent use by multiple goroutines.
type Client struct {
	transport Transport
	cfg       clientConfig
	ctx       context.Context
	cancel    context.CancelFunc

	mu       sync.RWMutex
	seqs     map[string]*Seq          // active sequences by seq_id
	pending  map[string]chan *MSEvent // pending opens by cid
	closed   bool
	closeErr error
}

// Connect establishes a connection to a ModelSocket server.
func Connect(ctx context.Context, url string, apiKey string, opts ...ClientOption) (*Client, error) {
	transport, err := Dial(ctx, url, apiKey, nil)
	if err != nil {
		return nil, err
	}

	return NewWithTransport(ctx, transport, opts...), nil
}

// NewWithTransport creates a Client with a custom transport.
// This is useful for testing or custom transport implementations.
func NewWithTransport(ctx context.Context, transport Transport, opts ...ClientOption) *Client {
	ctx, cancel := context.WithCancel(ctx)

	cfg := clientConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	c := &Client{
		transport: transport,
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		seqs:      make(map[string]*Seq),
		pending:   make(map[string]chan *MSEvent),
	}

	go c.readLoop()

	return c
}

// Open creates a new sequence with the specified model.
func (c *Client) Open(ctx context.Context, model string, opts ...OpenOption) (*Seq, error) {
	cfg := openConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	cid := uuid.New().String()

	// Create channel to receive the SeqOpened event
	ch := make(chan *MSEvent, 1)
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil, ErrClosed
	}
	c.pending[cid] = ch
	c.mu.Unlock()

	defer func() {
		c.mu.Lock()
		delete(c.pending, cid)
		c.mu.Unlock()
	}()

	// Build the request
	data := SeqOpenData{
		Model:        model,
		SkipPrelude:  cfg.skipPrelude,
		ToolsEnabled: cfg.toolbox != nil,
	}

	if cfg.toolbox != nil && cfg.toolbox.toolInstructions != "" {
		data.ToolPrompt = cfg.toolbox.toolInstructions
	}

	req := NewSeqOpenRequest(cid, data)

	// Send the request
	if err := c.send(ctx, req); err != nil {
		return nil, &SendError{Op: "seq_open", Err: err}
	}

	// Wait for response
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.ctx.Done():
		return nil, ErrClosed
	case event := <-ch:
		if event.IsError() {
			return nil, &ProtocolError{
				Message: event.Message,
				SeqID:   event.SeqID,
				CID:     event.CID,
			}
		}
		if !event.IsSeqOpened() {
			return nil, ErrUnexpectedEvent
		}

		// Create and register the sequence
		seq := newSeq(c, event.SeqID, cfg.toolbox)
		c.mu.Lock()
		c.seqs[seq.id] = seq
		c.mu.Unlock()

		// If a toolbox is configured with instructions, send them as a system message
		if cfg.toolbox != nil && cfg.toolbox.ToolInstructions() != "" {
			if err := seq.Append(ctx, cfg.toolbox.ToolInstructions(), AsSystem()); err != nil {
				return nil, err
			}
		}

		return seq, nil
	}
}

// Close closes the connection and all sequences.
func (c *Client) Close(ctx context.Context) error {
	c.mu.Lock()
	if c.closed {
		c.mu.Unlock()
		return nil
	}
	c.closed = true
	c.mu.Unlock()

	c.cancel()

	// Close all sequences
	c.mu.RLock()
	seqs := make([]*Seq, 0, len(c.seqs))
	for _, seq := range c.seqs {
		seqs = append(seqs, seq)
	}
	c.mu.RUnlock()

	for _, seq := range seqs {
		seq.handleClose(nil)
	}

	return c.transport.Close()
}

// readLoop reads events from the transport and routes them.
func (c *Client) readLoop() {
	for {
		event, err := c.transport.Receive(c.ctx)
		if err != nil {
			c.mu.Lock()
			c.closeErr = err
			c.closed = true
			c.mu.Unlock()
			c.cancel()
			return
		}

		// Observability hook
		if c.cfg.onReceive != nil {
			c.cfg.onReceive(event)
		}

		// Log if logger configured
		if c.cfg.logger != nil {
			c.cfg.logger.Debug("received event",
				slog.String("event", event.Event),
				slog.String("seq_id", event.SeqID),
				slog.String("cid", event.CID),
			)
		}

		c.routeEvent(event)
	}
}

// routeEvent routes an event to the appropriate handler.
func (c *Client) routeEvent(event *MSEvent) {
	// Handle SeqOpened - route to pending channel
	if event.IsSeqOpened() {
		c.mu.RLock()
		ch, ok := c.pending[event.CID]
		c.mu.RUnlock()
		if ok {
			select {
			case ch <- event:
			default:
			}
		}
		return
	}

	// Handle errors that might be for pending opens
	if event.IsError() && event.CID != "" {
		c.mu.RLock()
		ch, ok := c.pending[event.CID]
		c.mu.RUnlock()
		if ok {
			select {
			case ch <- event:
			default:
			}
			return
		}
	}

	// Route to sequence
	seqID := event.SeqID
	if seqID == "" {
		return
	}

	c.mu.RLock()
	seq, ok := c.seqs[seqID]
	c.mu.RUnlock()

	if ok {
		seq.handleEvent(event)
	}
}

// send sends a request through the transport.
func (c *Client) send(ctx context.Context, req *MSRequest) error {
	c.mu.RLock()
	closed := c.closed
	c.mu.RUnlock()

	if closed {
		return ErrClosed
	}

	// Observability hook
	if c.cfg.onSend != nil {
		c.cfg.onSend(req)
	}

	// Log if logger configured
	if c.cfg.logger != nil {
		c.cfg.logger.Debug("sending request",
			slog.String("request", req.Request),
			slog.String("cid", req.CID),
			slog.String("seq_id", req.SeqID),
		)
	}

	return c.transport.Send(ctx, req)
}

// removeSeq removes a sequence from the client.
func (c *Client) removeSeq(seqID string) {
	c.mu.Lock()
	delete(c.seqs, seqID)
	c.mu.Unlock()
}
