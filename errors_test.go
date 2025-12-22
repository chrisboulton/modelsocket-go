package modelsocket

import (
	"errors"
	"testing"
)

func TestConnectionError(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &ConnectionError{Op: "dial", Err: underlying}

	if err.Error() != "modelsocket: dial: connection refused" {
		t.Errorf("Error() = %s", err.Error())
	}

	if !errors.Is(err, underlying) {
		t.Error("errors.Is should return true for underlying error")
	}
}

func TestConnectionError_WithURL(t *testing.T) {
	underlying := errors.New("connection refused")
	err := &ConnectionError{Op: "dial", URL: "wss://example.com", Err: underlying}

	expected := "modelsocket: dial wss://example.com: connection refused"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestSendError(t *testing.T) {
	underlying := errors.New("write failed")
	err := &SendError{Op: "append", Err: underlying}

	expected := "modelsocket: send append: write failed"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}

	if !errors.Is(err, underlying) {
		t.Error("errors.Is should return true for underlying error")
	}
}

func TestProtocolError(t *testing.T) {
	err := &ProtocolError{
		Code:    "ERR_001",
		Message: "something went wrong",
		SeqID:   "seq-123",
		CID:     "cmd-456",
	}

	expected := "modelsocket: protocol error [ERR_001]: something went wrong"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestProtocolError_NoCode(t *testing.T) {
	err := &ProtocolError{
		Message: "something went wrong",
	}

	expected := "modelsocket: protocol error: something went wrong"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestSeqError(t *testing.T) {
	err := &SeqError{
		SeqID:   "seq-123",
		Message: "sequence failed",
	}

	expected := "modelsocket: sequence seq-123: sequence failed"
	if err.Error() != expected {
		t.Errorf("Error() = %s, want %s", err.Error(), expected)
	}
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"ErrClosed", ErrClosed, "modelsocket: connection closed"},
		{"ErrSeqClosed", ErrSeqClosed, "modelsocket: sequence closed"},
		{"ErrTimeout", ErrTimeout, "modelsocket: operation timed out"},
		{"ErrInvalidState", ErrInvalidState, "modelsocket: invalid sequence state"},
		{"ErrToolNotFound", ErrToolNotFound, "modelsocket: tool not found"},
		{"ErrUnexpectedEvent", ErrUnexpectedEvent, "modelsocket: unexpected event"},
		{"ErrBufferFull", ErrBufferFull, "modelsocket: buffer full"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err.Error() != tt.want {
				t.Errorf("Error() = %s, want %s", tt.err.Error(), tt.want)
			}
		})
	}
}

func TestErrorsIs(t *testing.T) {
	// Verify sentinel errors work with errors.Is
	wrapped := &ConnectionError{Op: "dial", Err: ErrClosed}
	if !errors.Is(wrapped, ErrClosed) {
		t.Error("errors.Is should find ErrClosed in wrapped error")
	}

	// Verify errors.As works for typed errors
	var connErr *ConnectionError
	if !errors.As(wrapped, &connErr) {
		t.Error("errors.As should extract ConnectionError")
	}
}
