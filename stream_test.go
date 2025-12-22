package modelsocket

import (
	"context"
	"testing"
)

func TestGenStream_Next(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx := context.Background()

	go func() {
		stream.handleText(&MSEvent{Event: "seq_text", Text: "Hello "})
		stream.handleText(&MSEvent{Event: "seq_text", Text: "world!"})
		stream.handleFinish(&MSEvent{Event: "seq_gen_finish", CID: "cid-1"})
	}()

	var text string
	for {
		chunk, err := stream.Next(ctx)
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if chunk == nil {
			break
		}
		text += chunk.Text
	}

	if text != "Hello world!" {
		t.Errorf("text = %s, want Hello world!", text)
	}
}

func TestGenStream_Next_ContextCancel(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := stream.Next(ctx)
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestGenStream_Text(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx := context.Background()

	go func() {
		stream.handleText(&MSEvent{Event: "seq_text", Text: "Hello "})
		stream.handleText(&MSEvent{Event: "seq_text", Text: "world!"})
		stream.handleFinish(&MSEvent{Event: "seq_gen_finish", CID: "cid-1"})
	}()

	text, err := stream.Text(ctx)
	if err != nil {
		t.Fatalf("Text error: %v", err)
	}

	if text != "Hello world!" {
		t.Errorf("text = %s, want Hello world!", text)
	}
}

func TestGenStream_TextAndTokens(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx := context.Background()

	go func() {
		stream.handleText(&MSEvent{Event: "seq_text", Text: "A", Tokens: []int{1}})
		stream.handleText(&MSEvent{Event: "seq_text", Text: "B", Tokens: []int{2, 3}})
		stream.handleFinish(&MSEvent{Event: "seq_gen_finish", CID: "cid-1"})
	}()

	text, tokens, err := stream.TextAndTokens(ctx)
	if err != nil {
		t.Fatalf("TextAndTokens error: %v", err)
	}

	if text != "AB" {
		t.Errorf("text = %s, want AB", text)
	}
	if len(tokens) != 3 {
		t.Errorf("len(tokens) = %d, want 3", len(tokens))
	}
}

func TestGenStream_ToolCall(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx := context.Background()

	go func() {
		stream.handleText(&MSEvent{Event: "seq_text", Text: "Calling tool..."})
		stream.handleToolCall(&MSEvent{
			Event: "seq_tool_call",
			SeqID: "seq-1",
			ToolCalls: []SeqToolCall{
				{Name: "get_weather", Args: `{"city":"NYC"}`},
			},
		})
		stream.handleFinish(&MSEvent{Event: "seq_gen_finish", CID: "cid-1"})
	}()

	// First chunk: text
	chunk1, err := stream.Next(ctx)
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if chunk1.Text != "Calling tool..." {
		t.Errorf("chunk1.Text = %s, want Calling tool...", chunk1.Text)
	}

	// Second chunk: tool call
	chunk2, err := stream.Next(ctx)
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if len(chunk2.ToolCalls) != 1 {
		t.Fatalf("len(ToolCalls) = %d, want 1", len(chunk2.ToolCalls))
	}
	if chunk2.ToolCalls[0].Name != "get_weather" {
		t.Errorf("ToolCalls[0].Name = %s, want get_weather", chunk2.ToolCalls[0].Name)
	}

	// Third: done
	chunk3, err := stream.Next(ctx)
	if err != nil {
		t.Fatalf("Next error: %v", err)
	}
	if chunk3 != nil {
		t.Error("expected nil chunk after finish")
	}
}

func TestGenStream_Chunks_Iterator(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx := context.Background()

	go func() {
		stream.handleText(&MSEvent{Event: "seq_text", Text: "Hello "})
		stream.handleText(&MSEvent{Event: "seq_text", Text: "world!"})
		stream.handleFinish(&MSEvent{Event: "seq_gen_finish", CID: "cid-1"})
	}()

	var text string
	for chunk, err := range stream.Chunks(ctx) {
		if err != nil {
			t.Fatalf("Chunks error: %v", err)
		}
		text += chunk.Text
	}

	if text != "Hello world!" {
		t.Errorf("text = %s, want Hello world!", text)
	}
}

func TestGenStream_HiddenText(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx := context.Background()

	go func() {
		stream.handleText(&MSEvent{Event: "seq_text", Text: "visible", Hidden: false})
		stream.handleText(&MSEvent{Event: "seq_text", Text: "hidden", Hidden: true})
		stream.handleText(&MSEvent{Event: "seq_text", Text: "visible2", Hidden: false})
		stream.handleFinish(&MSEvent{Event: "seq_gen_finish", CID: "cid-1"})
	}()

	text, err := stream.Text(ctx)
	if err != nil {
		t.Fatalf("Text error: %v", err)
	}

	if text != "visiblevisible2" {
		t.Errorf("text = %s, want visiblevisible2", text)
	}
}

func TestGenStream_TokenCounts(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx := context.Background()

	go func() {
		stream.handleText(&MSEvent{Event: "seq_text", Text: "test"})
		stream.handleFinish(&MSEvent{
			Event:        "seq_gen_finish",
			CID:          "cid-1",
			InputTokens:  10,
			OutputTokens: 5,
		})
	}()

	_, err := stream.Text(ctx)
	if err != nil {
		t.Fatalf("Text error: %v", err)
	}

	if stream.InputTokens() != 10 {
		t.Errorf("InputTokens = %d, want 10", stream.InputTokens())
	}
	if stream.OutputTokens() != 5 {
		t.Errorf("OutputTokens = %d, want 5", stream.OutputTokens())
	}
}

func TestGenStream_Close(t *testing.T) {
	stream := newGenStream(nil, "cid-1")
	ctx := context.Background()

	go func() {
		stream.handleText(&MSEvent{Event: "seq_text", Text: "test"})
		stream.handleClose()
	}()

	_, err := stream.Text(ctx)
	if err != ErrSeqClosed {
		t.Errorf("err = %v, want ErrSeqClosed", err)
	}
}

func TestGenStream_DoubleClose(t *testing.T) {
	stream := newGenStream(nil, "cid-1")

	// Should not panic
	stream.handleClose()
	stream.handleClose()
	stream.handleFinish(&MSEvent{Event: "seq_gen_finish"})
}
