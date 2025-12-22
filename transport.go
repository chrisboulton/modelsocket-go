package modelsocket

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"

	"github.com/coder/websocket"
)

// Transport provides the interface for sending and receiving messages.
// Implementations must be safe for concurrent use.
type Transport interface {
	Send(ctx context.Context, req *MSRequest) error
	Receive(ctx context.Context) (*MSEvent, error)
	Close() error
}

// DialOptions configures the WebSocket connection.
type DialOptions struct {
	// HTTPHeader specifies additional HTTP headers to send during handshake.
	HTTPHeader http.Header

	// HTTPClient is the HTTP client used for the handshake.
	// If nil, http.DefaultClient is used.
	HTTPClient *http.Client
}

// Dial connects to a ModelSocket server and returns a Transport.
func Dial(ctx context.Context, url string, apiKey string, opts *DialOptions) (Transport, error) {
	headers := http.Header{}
	if opts != nil && opts.HTTPHeader != nil {
		headers = opts.HTTPHeader.Clone()
	}
	if apiKey != "" {
		headers.Set("Authorization", "Bearer "+apiKey)
	}

	dialOpts := &websocket.DialOptions{
		HTTPHeader:   headers,
		Subprotocols: []string{"modelsocket.v0"},
	}
	if opts != nil && opts.HTTPClient != nil {
		dialOpts.HTTPClient = opts.HTTPClient
	}

	conn, _, err := websocket.Dial(ctx, url, dialOpts)
	if err != nil {
		return nil, &ConnectionError{Op: "dial", URL: url, Err: err}
	}

	// Set a large read limit for potentially large responses
	conn.SetReadLimit(32 * 1024 * 1024) // 32MB

	return &wsTransport{conn: conn}, nil
}

// wsTransport implements Transport over WebSocket.
type wsTransport struct {
	conn   *websocket.Conn
	mu     sync.Mutex
	closed bool
}

// Send sends a request to the server.
func (t *wsTransport) Send(ctx context.Context, req *MSRequest) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return ErrClosed
	}

	data, err := json.Marshal(req)
	if err != nil {
		return &SendError{Op: "marshal", Err: err}
	}

	if err := t.conn.Write(ctx, websocket.MessageText, data); err != nil {
		return &ConnectionError{Op: "write", Err: err}
	}

	return nil
}

// Receive receives an event from the server.
func (t *wsTransport) Receive(ctx context.Context) (*MSEvent, error) {
	_, data, err := t.conn.Read(ctx)
	if err != nil {
		t.mu.Lock()
		closed := t.closed
		t.mu.Unlock()
		if closed {
			return nil, ErrClosed
		}
		return nil, &ConnectionError{Op: "read", Err: err}
	}

	var event MSEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, &SendError{Op: "unmarshal", Err: err}
	}

	return &event, nil
}

// Close closes the transport.
func (t *wsTransport) Close() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.closed {
		return nil
	}
	t.closed = true

	return t.conn.Close(websocket.StatusNormalClosure, "")
}
