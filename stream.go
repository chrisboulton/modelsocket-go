package modelsocket

import (
	"context"
	"iter"
	"strings"
	"sync"
)

// GenChunk represents a chunk of generated content.
type GenChunk struct {
	Text      string
	Hidden    bool
	Tokens    []int
	ToolCalls []ToolCall
}

// ToolCall represents a tool call from the model.
type ToolCall struct {
	Name string
	Args string
}

// GenStream provides streaming access to generated content.
type GenStream struct {
	seq *Seq
	cid string

	mu       sync.Mutex
	chunks   chan *GenChunk
	done     chan struct{}
	err      error
	finished bool

	closeOnce sync.Once

	// Stats from finish event
	inputTokens  int
	outputTokens int
}

// newGenStream creates a new generation stream.
func newGenStream(seq *Seq, cid string) *GenStream {
	return &GenStream{
		seq:    seq,
		cid:    cid,
		chunks: make(chan *GenChunk, 100),
		done:   make(chan struct{}),
	}
}

// Next returns the next chunk, or nil if done.
// Returns an error if one occurred during generation.
// The context can be used to cancel waiting for the next chunk.
func (g *GenStream) Next(ctx context.Context) (*GenChunk, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case chunk, ok := <-g.chunks:
		if !ok {
			g.mu.Lock()
			err := g.err
			g.mu.Unlock()
			return nil, err
		}
		return chunk, nil
	case <-g.done:
		// Drain any remaining chunks
		select {
		case chunk, ok := <-g.chunks:
			if ok {
				return chunk, nil
			}
		default:
		}
		g.mu.Lock()
		err := g.err
		g.mu.Unlock()
		return nil, err
	}
}

// Chunks returns an iterator over all chunks in the stream.
func (g *GenStream) Chunks(ctx context.Context) iter.Seq2[*GenChunk, error] {
	return func(yield func(*GenChunk, error) bool) {
		for {
			chunk, err := g.Next(ctx)
			if err != nil {
				yield(nil, err)
				return
			}
			if chunk == nil {
				return
			}
			if !yield(chunk, nil) {
				return
			}
		}
	}
}

// Text collects all generated text and returns it.
func (g *GenStream) Text(ctx context.Context) (string, error) {
	var sb strings.Builder

	for chunk, err := range g.Chunks(ctx) {
		if err != nil {
			return sb.String(), err
		}
		if !chunk.Hidden {
			sb.WriteString(chunk.Text)
		}
	}
	return sb.String(), nil
}

// TextAndTokens collects all generated text and tokens.
func (g *GenStream) TextAndTokens(ctx context.Context) (string, []int, error) {
	var sb strings.Builder
	var tokens []int

	for chunk, err := range g.Chunks(ctx) {
		if err != nil {
			return sb.String(), tokens, err
		}
		if !chunk.Hidden {
			sb.WriteString(chunk.Text)
		}
		tokens = append(tokens, chunk.Tokens...)
	}
	return sb.String(), tokens, nil
}

// InputTokens returns the input token count.
// Only valid after stream is exhausted.
func (g *GenStream) InputTokens() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.inputTokens
}

// OutputTokens returns the output token count.
// Only valid after stream is exhausted.
func (g *GenStream) OutputTokens() int {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.outputTokens
}

// handleText processes a text event.
func (g *GenStream) handleText(event *MSEvent) {
	g.mu.Lock()
	if g.finished {
		g.mu.Unlock()
		return
	}
	g.mu.Unlock()

	chunk := &GenChunk{
		Text:   event.Text,
		Hidden: event.Hidden,
		Tokens: event.Tokens,
	}

	// Block until chunk is consumed (backpressure)
	select {
	case g.chunks <- chunk:
	case <-g.done:
		// Stream was closed
	}
}

// handleToolCall processes a tool call event.
func (g *GenStream) handleToolCall(event *MSEvent) {
	g.mu.Lock()
	if g.finished {
		g.mu.Unlock()
		return
	}
	g.mu.Unlock()

	// Convert SeqToolCall to ToolCall
	var toolCalls []ToolCall
	for _, tc := range event.ToolCalls {
		toolCalls = append(toolCalls, ToolCall{
			Name: tc.Name,
			Args: tc.Args,
		})
	}

	chunk := &GenChunk{
		ToolCalls: toolCalls,
	}

	// Block until chunk is consumed (backpressure)
	select {
	case g.chunks <- chunk:
	case <-g.done:
		// Stream was closed
	}
}

// handleFinish processes a generation finish event.
func (g *GenStream) handleFinish(event *MSEvent) {
	g.closeOnce.Do(func() {
		g.mu.Lock()
		g.finished = true
		g.inputTokens = event.InputTokens
		g.outputTokens = event.OutputTokens
		g.mu.Unlock()

		close(g.chunks)
		close(g.done)
	})
}

// handleClose handles stream closure due to sequence close.
func (g *GenStream) handleClose() {
	g.closeOnce.Do(func() {
		g.mu.Lock()
		g.finished = true
		g.err = ErrSeqClosed
		g.mu.Unlock()

		close(g.chunks)
		close(g.done)
	})
}
