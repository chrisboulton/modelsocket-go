package modelsocket

import (
	"errors"
	"fmt"
)

// Sentinel errors for common conditions.
var (
	ErrClosed          = errors.New("modelsocket: connection closed")
	ErrSeqClosed       = errors.New("modelsocket: sequence closed")
	ErrTimeout         = errors.New("modelsocket: operation timed out")
	ErrInvalidState    = errors.New("modelsocket: invalid sequence state")
	ErrToolNotFound    = errors.New("modelsocket: tool not found")
	ErrUnexpectedEvent = errors.New("modelsocket: unexpected event")
	ErrBufferFull      = errors.New("modelsocket: buffer full")
)

// ConnectionError represents a connection-level error.
type ConnectionError struct {
	Op  string
	URL string
	Err error
}

func (e *ConnectionError) Error() string {
	if e.URL != "" {
		return fmt.Sprintf("modelsocket: %s %s: %v", e.Op, e.URL, e.Err)
	}
	return fmt.Sprintf("modelsocket: %s: %v", e.Op, e.Err)
}

func (e *ConnectionError) Unwrap() error {
	return e.Err
}

// SendError represents an error during request sending.
type SendError struct {
	Op  string
	Err error
}

func (e *SendError) Error() string {
	return fmt.Sprintf("modelsocket: send %s: %v", e.Op, e.Err)
}

func (e *SendError) Unwrap() error {
	return e.Err
}

// ProtocolError represents a protocol-level error from the server.
type ProtocolError struct {
	Code    string
	Message string
	SeqID   string
	CID     string
}

func (e *ProtocolError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("modelsocket: protocol error [%s]: %s", e.Code, e.Message)
	}
	return fmt.Sprintf("modelsocket: protocol error: %s", e.Message)
}

// SeqError represents a sequence-level error.
type SeqError struct {
	SeqID   string
	Message string
}

func (e *SeqError) Error() string {
	return fmt.Sprintf("modelsocket: sequence %s: %s", e.SeqID, e.Message)
}
