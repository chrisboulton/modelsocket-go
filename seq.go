package modelsocket

import (
	"context"
	"sync"

	"github.com/google/uuid"
)

// Seq represents an active conversation sequence.
// It is safe for concurrent use by multiple goroutines.
// However, only one Generate call can be active at a time.
type Seq struct {
	client  *Client
	id      string
	toolbox *Toolbox

	mu       sync.RWMutex
	state    SeqState
	closed   bool
	closeErr error

	// Command tracking
	cmdMu    sync.RWMutex
	commands map[string]chan *MSEvent

	// Active generation stream
	genStream *GenStream
}

// newSeq creates a new sequence.
func newSeq(client *Client, id string, toolbox *Toolbox) *Seq {
	return &Seq{
		client:   client,
		id:       id,
		toolbox:  toolbox,
		state:    StateReady,
		commands: make(map[string]chan *MSEvent),
	}
}

// ID returns the sequence ID.
func (s *Seq) ID() string {
	return s.id
}

// State returns the current sequence state.
func (s *Seq) State() SeqState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.state
}

// Append adds text to the sequence.
func (s *Seq) Append(ctx context.Context, text string, opts ...AppendOption) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSeqClosed
	}
	s.mu.RUnlock()

	cfg := appendConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	cid := uuid.New().String()
	ch := s.registerCommand(cid)
	defer s.unregisterCommand(cid)

	data := SeqAppendData{
		Text: text,
		Role: string(cfg.role),
		Echo: cfg.echo,
	}

	req := NewAppendRequest(cid, s.id, data)

	if err := s.client.send(ctx, req); err != nil {
		return err
	}

	// Wait for completion
	select {
	case <-ctx.Done():
		return ctx.Err()
	case event := <-ch:
		if event.IsError() {
			return &ProtocolError{
				Message: event.Message,
				SeqID:   event.SeqID,
				CID:     event.CID,
			}
		}
		return nil
	}
}

// Generate starts text generation and returns a stream.
func (s *Seq) Generate(ctx context.Context, opts ...GenOption) (*GenStream, error) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil, ErrSeqClosed
	}
	s.mu.Unlock()

	cfg := genConfig{}
	for _, opt := range opts {
		opt(&cfg)
	}

	cid := uuid.New().String()

	// Create the stream
	stream := newGenStream(s, cid)

	s.mu.Lock()
	s.genStream = stream
	s.mu.Unlock()

	// Build request
	data := cfg.toSeqGenData()
	req := NewGenRequest(cid, s.id, data)

	if err := s.client.send(ctx, req); err != nil {
		s.mu.Lock()
		s.genStream = nil
		s.mu.Unlock()
		return nil, err
	}

	return stream, nil
}

// Fork creates a new sequence with the same conversation history.
func (s *Seq) Fork(ctx context.Context) (*Seq, error) {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return nil, ErrSeqClosed
	}
	s.mu.RUnlock()

	cid := uuid.New().String()
	ch := s.registerCommand(cid)
	defer s.unregisterCommand(cid)

	req := NewForkRequest(cid, s.id)

	if err := s.client.send(ctx, req); err != nil {
		return nil, err
	}

	// Wait for completion
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case event := <-ch:
		if event.IsError() {
			return nil, &ProtocolError{
				Message: event.Message,
				SeqID:   event.SeqID,
				CID:     event.CID,
			}
		}
		if !event.IsSeqForkFinish() {
			return nil, ErrUnexpectedEvent
		}

		// Create and register the new sequence
		forked := newSeq(s.client, event.ChildSeqID, s.toolbox)
		s.client.mu.Lock()
		s.client.seqs[forked.id] = forked
		s.client.mu.Unlock()

		return forked, nil
	}
}

// Close closes the sequence.
func (s *Seq) Close(ctx context.Context) error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	cid := uuid.New().String()
	ch := s.registerCommand(cid)
	defer s.unregisterCommand(cid)

	req := NewCloseRequest(cid, s.id)

	if err := s.client.send(ctx, req); err != nil {
		return err
	}

	// Wait for completion
	select {
	case <-ctx.Done():
		return ctx.Err()
	case event := <-ch:
		if event.IsError() {
			return &ProtocolError{
				Message: event.Message,
				SeqID:   event.SeqID,
				CID:     event.CID,
			}
		}
		return nil
	}
}

// ToolReturn sends tool call results back to the model.
func (s *Seq) ToolReturn(ctx context.Context, results []ToolResult) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrSeqClosed
	}
	s.mu.RUnlock()

	cid := uuid.New().String()

	req := NewToolReturnRequest(cid, s.id, results, SeqGenData{})

	return s.client.send(ctx, req)
}

// handleEvent processes an incoming event for this sequence.
func (s *Seq) handleEvent(event *MSEvent) {
	// Update state
	if event.IsSeqState() {
		s.mu.Lock()
		s.state = event.State
		s.mu.Unlock()
	}

	// Route text events to generation stream
	if event.IsSeqText() {
		s.mu.RLock()
		stream := s.genStream
		s.mu.RUnlock()
		if stream != nil {
			stream.handleText(event)
		}
	}

	// Route tool calls to generation stream
	if event.IsSeqToolCall() {
		s.mu.RLock()
		stream := s.genStream
		s.mu.RUnlock()
		if stream != nil {
			stream.handleToolCall(event)
		}
	}

	// Handle generation finish
	if event.IsSeqGenFinish() {
		s.mu.Lock()
		stream := s.genStream
		// Only close stream if CID matches (avoid closing wrong stream after tool_return)
		if stream != nil && stream.cid == event.CID {
			s.genStream = nil
			s.mu.Unlock()
			stream.handleFinish(event)
		} else {
			s.mu.Unlock()
		}
	}

	// Handle command completions
	if cid := event.CID; cid != "" {
		s.cmdMu.RLock()
		ch, ok := s.commands[cid]
		s.cmdMu.RUnlock()
		if ok {
			select {
			case ch <- event:
			default:
			}
		}
	}

	// Handle close
	if event.IsSeqClosed() {
		s.handleClose(event)
	}
}

// handleClose handles sequence closure.
func (s *Seq) handleClose(event *MSEvent) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.state = StateClosed
	if event != nil && event.ErrorMsg != "" {
		s.closeErr = &SeqError{SeqID: s.id, Message: event.ErrorMsg}
	}
	stream := s.genStream
	s.genStream = nil
	s.mu.Unlock()

	// Close any active generation stream
	if stream != nil {
		stream.handleClose()
	}

	// Remove from client
	s.client.removeSeq(s.id)
}

// registerCommand registers a channel to receive a command response.
func (s *Seq) registerCommand(cid string) chan *MSEvent {
	ch := make(chan *MSEvent, 1)
	s.cmdMu.Lock()
	s.commands[cid] = ch
	s.cmdMu.Unlock()
	return ch
}

// unregisterCommand removes a command channel.
func (s *Seq) unregisterCommand(cid string) {
	s.cmdMu.Lock()
	delete(s.commands, cid)
	s.cmdMu.Unlock()
}
