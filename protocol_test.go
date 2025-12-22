package modelsocket

import (
	"encoding/json"
	"testing"
)

func TestNewSeqOpenRequest_MarshalJSON(t *testing.T) {
	req := NewSeqOpenRequest("test-cid", SeqOpenData{
		Model:        "meta/llama3.1-8b-instruct-free",
		ToolsEnabled: true,
	})

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Verify structure
	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["request"] != "seq_open" {
		t.Errorf("request = %v, want seq_open", parsed["request"])
	}
	if parsed["cid"] != "test-cid" {
		t.Errorf("cid = %v, want test-cid", parsed["cid"])
	}

	dataField := parsed["data"].(map[string]interface{})
	if dataField["model"] != "meta/llama3.1-8b-instruct-free" {
		t.Errorf("data.model = %v, want meta/llama3.1-8b-instruct-free", dataField["model"])
	}
}

func TestNewAppendRequest_MarshalJSON(t *testing.T) {
	req := NewAppendRequest("cmd-456", "seq-123", SeqAppendData{
		Text: "Hello",
		Role: "user",
	})

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["request"] != "seq_command" {
		t.Errorf("request = %v, want seq_command", parsed["request"])
	}
	if parsed["cid"] != "cmd-456" {
		t.Errorf("cid = %v, want cmd-456", parsed["cid"])
	}
	if parsed["seq_id"] != "seq-123" {
		t.Errorf("seq_id = %v, want seq-123", parsed["seq_id"])
	}

	dataField := parsed["data"].(map[string]interface{})
	if dataField["command"] != "append" {
		t.Errorf("data.command = %v, want append", dataField["command"])
	}
	if dataField["text"] != "Hello" {
		t.Errorf("data.text = %v, want Hello", dataField["text"])
	}
}

func TestNewGenRequest_MarshalJSON(t *testing.T) {
	maxTokens := 100
	temp := 0.7
	req := NewGenRequest("cmd-789", "seq-123", SeqGenData{
		Role:        "assistant",
		MaxTokens:   &maxTokens,
		Temperature: &temp,
		StopStrings: []string{"STOP", "END"},
	})

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["request"] != "seq_command" {
		t.Errorf("request = %v, want seq_command", parsed["request"])
	}

	dataField := parsed["data"].(map[string]interface{})
	if dataField["command"] != "gen" {
		t.Errorf("data.command = %v, want gen", dataField["command"])
	}
	if dataField["role"] != "assistant" {
		t.Errorf("data.role = %v, want assistant", dataField["role"])
	}
	if dataField["max_tokens"].(float64) != 100 {
		t.Errorf("data.max_tokens = %v, want 100", dataField["max_tokens"])
	}
}

func TestMSEvent_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
		check    func(*MSEvent) bool
	}{
		{
			name:     "seq_opened",
			input:    `{"event":"seq_opened","cid":"c1","seq_id":"s1"}`,
			wantType: "seq_opened",
			check: func(e *MSEvent) bool {
				return e.IsSeqOpened() && e.CID == "c1" && e.SeqID == "s1"
			},
		},
		{
			name:     "seq_text",
			input:    `{"event":"seq_text","seq_id":"s1","cid":"c1","text":"hello","hidden":false}`,
			wantType: "seq_text",
			check: func(e *MSEvent) bool {
				return e.IsSeqText() && e.Text == "hello"
			},
		},
		{
			name:     "seq_tool_call",
			input:    `{"event":"seq_tool_call","seq_id":"s1","cid":"c1","tool_calls":[{"name":"get_weather","args":"{\"city\":\"NYC\"}"}]}`,
			wantType: "seq_tool_call",
			check: func(e *MSEvent) bool {
				return e.IsSeqToolCall() && len(e.ToolCalls) == 1 && e.ToolCalls[0].Name == "get_weather"
			},
		},
		{
			name:     "seq_gen_finish",
			input:    `{"event":"seq_gen_finish","cid":"c1","seq_id":"s1"}`,
			wantType: "seq_gen_finish",
			check: func(e *MSEvent) bool {
				return e.IsSeqGenFinish()
			},
		},
		{
			name:     "error",
			input:    `{"event":"error","message":"something went wrong"}`,
			wantType: "error",
			check: func(e *MSEvent) bool {
				return e.IsError() && e.Message == "something went wrong"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var event MSEvent
			if err := json.Unmarshal([]byte(tt.input), &event); err != nil {
				t.Fatalf("unmarshal error: %v", err)
			}

			if event.Type() != tt.wantType {
				t.Errorf("Type() = %s, want %s", event.Type(), tt.wantType)
			}

			if !tt.check(&event) {
				t.Errorf("check failed for %s", tt.name)
			}
		})
	}
}

func TestMSEvent_IsChecks(t *testing.T) {
	tests := []struct {
		name  string
		event string
		check func(*MSEvent) bool
	}{
		{"seq_opened", "seq_opened", func(e *MSEvent) bool { return e.IsSeqOpened() }},
		{"seq_text", "seq_text", func(e *MSEvent) bool { return e.IsSeqText() }},
		{"seq_tool_call", "seq_tool_call", func(e *MSEvent) bool { return e.IsSeqToolCall() }},
		{"seq_append_finish", "seq_append_finish", func(e *MSEvent) bool { return e.IsSeqAppendFinish() }},
		{"seq_gen_finish", "seq_gen_finish", func(e *MSEvent) bool { return e.IsSeqGenFinish() }},
		{"seq_fork_finish", "seq_fork_finish", func(e *MSEvent) bool { return e.IsSeqForkFinish() }},
		{"seq_state", "seq_state", func(e *MSEvent) bool { return e.IsSeqState() }},
		{"seq_closed", "seq_closed", func(e *MSEvent) bool { return e.IsSeqClosed() }},
		{"error", "error", func(e *MSEvent) bool { return e.IsError() }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := &MSEvent{Event: tt.event}
			if !tt.check(event) {
				t.Errorf("Is%s() returned false for event type %s", tt.name, tt.event)
			}
		})
	}
}

func TestNewCloseRequest(t *testing.T) {
	req := NewCloseRequest("cid-1", "seq-1")

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed["request"] != "seq_command" {
		t.Errorf("request = %v, want seq_command", parsed["request"])
	}

	dataField := parsed["data"].(map[string]interface{})
	if dataField["command"] != "close" {
		t.Errorf("data.command = %v, want close", dataField["command"])
	}
}

func TestNewForkRequest(t *testing.T) {
	req := NewForkRequest("cid-1", "seq-1")

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	dataField := parsed["data"].(map[string]interface{})
	if dataField["command"] != "fork" {
		t.Errorf("data.command = %v, want fork", dataField["command"])
	}
}
