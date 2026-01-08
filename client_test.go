package modelsocket

import (
	"context"
	"sync"
	"testing"
	"time"
)

// mockTransport implements Transport for testing.
type mockTransport struct {
	mu       sync.Mutex
	requests []*MSRequest
	events   chan *MSEvent
	closed   bool
	sendErr  error
	recvErr  error

	// Channel signaled when a request is sent
	onSend chan *MSRequest
}

func newMockTransport() *mockTransport {
	return &mockTransport{
		events: make(chan *MSEvent, 100),
		onSend: make(chan *MSRequest, 100),
	}
}

func (m *mockTransport) Send(ctx context.Context, req *MSRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return ErrClosed
	}
	if m.sendErr != nil {
		return m.sendErr
	}
	m.requests = append(m.requests, req)

	// Signal that a request was sent
	select {
	case m.onSend <- req:
	default:
	}
	return nil
}

func (m *mockTransport) Receive(ctx context.Context) (*MSEvent, error) {
	if m.recvErr != nil {
		return nil, m.recvErr
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case event, ok := <-m.events:
		if !ok {
			return nil, ErrClosed
		}
		return event, nil
	}
}

func (m *mockTransport) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.closed {
		m.closed = true
		close(m.events)
	}
	return nil
}

func (m *mockTransport) pushEvent(event *MSEvent) {
	m.events <- event
}

func (m *mockTransport) getRequests() []*MSRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.requests
}

// waitForRequest waits for a request to be sent and returns it.
func (m *mockTransport) waitForRequest(t *testing.T, timeout time.Duration) *MSRequest {
	t.Helper()
	select {
	case req := <-m.onSend:
		return req
	case <-time.After(timeout):
		t.Fatal("timeout waiting for request")
		return nil
	}
}

func TestClient_Open(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)
	defer client.Close(ctx)

	// Respond to seq_open request
	go func() {
		req := transport.waitForRequest(t, time.Second)
		if req.Request == "seq_open" {
			transport.pushEvent(&MSEvent{
				Event: "seq_opened",
				CID:   req.CID,
				SeqID: "seq-123",
			})
		}
	}()

	seq, err := client.Open(ctx, "test-model")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	if seq.ID() != "seq-123" {
		t.Errorf("seq.ID() = %s, want seq-123", seq.ID())
	}
}

func TestClient_Open_WithOpts(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)
	defer client.Close(ctx)

	go func() {
		// Handle seq_open request
		req := transport.waitForRequest(t, time.Second)
		if req.Request == "seq_open" {
			transport.pushEvent(&MSEvent{
				Event: "seq_opened",
				CID:   req.CID,
				SeqID: "seq-456",
			})
		}
		// Handle system prompt append request
		req = transport.waitForRequest(t, time.Second)
		if req.Request == "seq_command" {
			transport.pushEvent(&MSEvent{
				Event: "seq_append_finish",
				CID:   req.CID,
				SeqID: "seq-456",
			})
		}
	}()

	toolbox := NewToolbox()
	toolbox.SetToolDefinitionPrompt("Use tools wisely")
	seq, err := client.Open(ctx, "test-model",
		WithSkipPrelude(),
		WithToolbox(toolbox),
	)
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	// Verify requests were sent correctly
	reqs := transport.getRequests()
	if len(reqs) < 2 {
		t.Fatalf("expected 2 requests, got %d", len(reqs))
	}

	// Check seq_open request
	openReq := reqs[0]
	if openReq.Request != "seq_open" {
		t.Errorf("Request = %s, want seq_open", openReq.Request)
	}

	data := openReq.Data.(SeqOpenData)
	if data.Model != "test-model" {
		t.Errorf("Model = %s, want test-model", data.Model)
	}
	if !data.ToolsEnabled {
		t.Error("ToolsEnabled = false, want true")
	}
	if !data.SkipPrelude {
		t.Error("SkipPrelude = false, want true")
	}

	// Check system prompt append request
	appendReq := reqs[1]
	if appendReq.Request != "seq_command" {
		t.Errorf("Request = %s, want seq_command", appendReq.Request)
	}
	appendData := appendReq.Data.(appendCommandData)
	if appendData.Command != "append" {
		t.Errorf("Command = %s, want append", appendData.Command)
	}
	if appendData.Text != "Use tools wisely" {
		t.Errorf("Text = %s, want 'Use tools wisely'", appendData.Text)
	}
	if appendData.Role != string(RoleSystem) {
		t.Errorf("Role = %s, want system", appendData.Role)
	}

	if seq.toolbox != toolbox {
		t.Error("toolbox not set on sequence")
	}
}

func TestClient_Open_Error(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)
	defer client.Close(ctx)

	go func() {
		req := transport.waitForRequest(t, time.Second)
		transport.pushEvent(&MSEvent{
			Event:   "error",
			CID:     req.CID,
			Message: "model not found",
		})
	}()

	_, err := client.Open(ctx, "nonexistent")
	if err == nil {
		t.Fatal("expected error")
	}

	protocolErr, ok := err.(*ProtocolError)
	if !ok {
		t.Fatalf("expected ProtocolError, got %T", err)
	}
	if protocolErr.Message != "model not found" {
		t.Errorf("Message = %s, want model not found", protocolErr.Message)
	}
}

func TestClient_Open_Timeout(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)
	defer client.Close(ctx)

	ctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()

	_, err := client.Open(ctx, "test-model")
	if err != context.DeadlineExceeded {
		t.Errorf("err = %v, want DeadlineExceeded", err)
	}
}

func TestClient_Close(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)

	// Open a sequence first
	go func() {
		req := transport.waitForRequest(t, time.Second)
		transport.pushEvent(&MSEvent{
			Event: "seq_opened",
			CID:   req.CID,
			SeqID: "seq-123",
		})
	}()

	seq, err := client.Open(ctx, "test-model")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	// Close client
	if err := client.Close(ctx); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	// Verify sequence is closed
	if seq.State() != StateClosed {
		t.Errorf("seq.State() = %s, want closed", seq.State())
	}

	// Verify can't open new sequences
	_, err = client.Open(ctx, "test-model")
	if err != ErrClosed {
		t.Errorf("err = %v, want ErrClosed", err)
	}
}

func TestSeq_Append(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)
	defer client.Close(ctx)

	// Open sequence
	go func() {
		req := transport.waitForRequest(t, time.Second)
		transport.pushEvent(&MSEvent{
			Event: "seq_opened",
			CID:   req.CID,
			SeqID: "seq-123",
		})
	}()

	seq, err := client.Open(ctx, "test-model")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	// Append message
	go func() {
		req := transport.waitForRequest(t, time.Second)
		if req.Request == "seq_command" {
			transport.pushEvent(&MSEvent{
				Event: "seq_append_finish",
				CID:   req.CID,
				SeqID: "seq-123",
			})
		}
	}()

	err = seq.Append(ctx, "Hello!", AsUser())
	if err != nil {
		t.Fatalf("Append error: %v", err)
	}

	// Verify request
	reqs := transport.getRequests()
	var appendReq *MSRequest
	for _, req := range reqs {
		if req.Request == "seq_command" {
			appendReq = req
			break
		}
	}

	if appendReq == nil {
		t.Fatal("no append request found")
	}

	data := appendReq.Data.(appendCommandData)
	if data.Text != "Hello!" {
		t.Errorf("Text = %s, want Hello!", data.Text)
	}
	if data.Role != "user" {
		t.Errorf("Role = %s, want user", data.Role)
	}
}

func TestSeq_Generate(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)
	defer client.Close(ctx)

	// Setup: Open sequence
	go func() {
		req := transport.waitForRequest(t, time.Second)
		transport.pushEvent(&MSEvent{
			Event: "seq_opened",
			CID:   req.CID,
			SeqID: "seq-123",
		})
	}()

	seq, err := client.Open(ctx, "test-model")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	// Start generation and simulate streaming response
	go func() {
		req := transport.waitForRequest(t, time.Second)

		// Send text chunks
		transport.pushEvent(&MSEvent{
			Event: "seq_text",
			SeqID: "seq-123",
			Text:  "Hello ",
		})
		transport.pushEvent(&MSEvent{
			Event: "seq_text",
			SeqID: "seq-123",
			Text:  "world!",
		})

		// Send finish
		transport.pushEvent(&MSEvent{
			Event:        "seq_gen_finish",
			CID:          req.CID,
			SeqID:        "seq-123",
			OutputTokens: 5,
		})
	}()

	stream, err := seq.Generate(ctx, GenerateAsAssistant())
	if err != nil {
		t.Fatalf("Generate error: %v", err)
	}

	text, err := stream.Text(ctx)
	if err != nil {
		t.Fatalf("Text error: %v", err)
	}

	if text != "Hello world!" {
		t.Errorf("text = %s, want Hello world!", text)
	}
}

func TestSeq_Fork(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)
	defer client.Close(ctx)

	// Setup: Open sequence
	go func() {
		req := transport.waitForRequest(t, time.Second)
		transport.pushEvent(&MSEvent{
			Event: "seq_opened",
			CID:   req.CID,
			SeqID: "seq-123",
		})
	}()

	seq, err := client.Open(ctx, "test-model")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	// Fork
	go func() {
		req := transport.waitForRequest(t, time.Second)
		if req.Request == "seq_command" && req.SeqID == "seq-123" {
			transport.pushEvent(&MSEvent{
				Event:      "seq_fork_finish",
				CID:        req.CID,
				SeqID:      "seq-123",
				ChildSeqID: "seq-456",
			})
		}
	}()

	forked, err := seq.Fork(ctx)
	if err != nil {
		t.Fatalf("Fork error: %v", err)
	}

	if forked.ID() != "seq-456" {
		t.Errorf("forked.ID() = %s, want seq-456", forked.ID())
	}
}

func TestSeq_Close(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	client := NewWithTransport(ctx, transport)
	defer client.Close(ctx)

	// Setup: Open sequence
	go func() {
		req := transport.waitForRequest(t, time.Second)
		transport.pushEvent(&MSEvent{
			Event: "seq_opened",
			CID:   req.CID,
			SeqID: "seq-123",
		})
	}()

	seq, err := client.Open(ctx, "test-model")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	// Close sequence
	go func() {
		req := transport.waitForRequest(t, time.Second)
		if req.Request == "seq_command" && req.SeqID == "seq-123" {
			transport.pushEvent(&MSEvent{
				Event: "seq_closed",
				CID:   req.CID,
				SeqID: "seq-123",
			})
		}
	}()

	err = seq.Close(ctx)
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if seq.State() != StateClosed {
		t.Errorf("State = %s, want closed", seq.State())
	}
}

func TestClient_WithObservability(t *testing.T) {
	transport := newMockTransport()
	ctx := context.Background()

	var sentRequests []*MSRequest
	var receivedEvents []*MSEvent

	client := NewWithTransport(ctx, transport,
		WithOnSend(func(req *MSRequest) {
			sentRequests = append(sentRequests, req)
		}),
		WithOnReceive(func(event *MSEvent) {
			receivedEvents = append(receivedEvents, event)
		}),
	)
	defer client.Close(ctx)

	go func() {
		req := transport.waitForRequest(t, time.Second)
		transport.pushEvent(&MSEvent{
			Event: "seq_opened",
			CID:   req.CID,
			SeqID: "seq-123",
		})
	}()

	_, err := client.Open(ctx, "test-model")
	if err != nil {
		t.Fatalf("Open error: %v", err)
	}

	if len(sentRequests) != 1 {
		t.Errorf("sentRequests = %d, want 1", len(sentRequests))
	}
	if len(receivedEvents) != 1 {
		t.Errorf("receivedEvents = %d, want 1", len(receivedEvents))
	}
}
