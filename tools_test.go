package modelsocket

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestNewToolbox(t *testing.T) {
	tb := NewToolbox()
	if tb == nil {
		t.Fatal("NewToolbox returned nil")
	}
	if tb.tools == nil {
		t.Fatal("tools map is nil")
	}
}

func TestToolbox_Add_Get(t *testing.T) {
	tb := NewToolbox()

	tool := NewFuncTool(
		ToolDefinition{
			Name:        "test_tool",
			Description: "A test tool",
		},
		func(ctx context.Context, args string) (string, error) {
			return "result", nil
		},
	)

	tb.Add(tool)

	got, ok := tb.Get("test_tool")
	if !ok {
		t.Fatal("Get returned false")
	}
	if got.Definition().Name != "test_tool" {
		t.Errorf("Name = %s, want test_tool", got.Definition().Name)
	}
}

func TestToolbox_Get_NotFound(t *testing.T) {
	tb := NewToolbox()

	_, ok := tb.Get("nonexistent")
	if ok {
		t.Error("Get returned true for nonexistent tool")
	}
}

func TestToolbox_Call(t *testing.T) {
	tb := NewToolbox()

	tool := NewFuncTool(
		ToolDefinition{Name: "echo"},
		func(ctx context.Context, args string) (string, error) {
			return "echo: " + args, nil
		},
	)
	tb.Add(tool)

	result, err := tb.Call(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if result != "echo: hello" {
		t.Errorf("result = %s, want echo: hello", result)
	}
}

func TestToolbox_Call_NotFound(t *testing.T) {
	tb := NewToolbox()

	_, err := tb.Call(context.Background(), "nonexistent", "")
	if !errors.Is(err, ErrToolNotFound) {
		t.Errorf("err = %v, want ErrToolNotFound", err)
	}
}

func TestToolbox_Call_Error(t *testing.T) {
	tb := NewToolbox()

	expectedErr := errors.New("tool error")
	tool := NewFuncTool(
		ToolDefinition{Name: "failing"},
		func(ctx context.Context, args string) (string, error) {
			return "", expectedErr
		},
	)
	tb.Add(tool)

	_, err := tb.Call(context.Background(), "failing", "")
	if !errors.Is(err, expectedErr) {
		t.Errorf("err = %v, want %v", err, expectedErr)
	}
}

func TestToolbox_CallTools(t *testing.T) {
	tb := NewToolbox()

	tb.Add(NewFuncTool(
		ToolDefinition{Name: "add"},
		func(ctx context.Context, args string) (string, error) {
			return "sum", nil
		},
	))
	tb.Add(NewFuncTool(
		ToolDefinition{Name: "multiply"},
		func(ctx context.Context, args string) (string, error) {
			return "product", nil
		},
	))

	calls := []ToolCall{
		{Name: "add", Args: `{"a":1,"b":2}`},
		{Name: "multiply", Args: `{"a":3,"b":4}`},
	}

	results, err := tb.CallTools(context.Background(), calls)
	if err != nil {
		t.Fatalf("CallTools error: %v", err)
	}

	if len(results) != 2 {
		t.Fatalf("len(results) = %d, want 2", len(results))
	}

	if results[0].Name != "add" || results[0].Result != "sum" {
		t.Errorf("results[0] = %+v, want {add, sum}", results[0])
	}
	if results[1].Name != "multiply" || results[1].Result != "product" {
		t.Errorf("results[1] = %+v, want {multiply, product}", results[1])
	}
}

func TestToolbox_CallTools_WithError(t *testing.T) {
	tb := NewToolbox()

	tb.Add(NewFuncTool(
		ToolDefinition{Name: "failing"},
		func(ctx context.Context, args string) (string, error) {
			return "", errors.New("tool failed")
		},
	))

	calls := []ToolCall{{Name: "failing", Args: ""}}

	results, err := tb.CallTools(context.Background(), calls)
	if err != nil {
		t.Fatalf("CallTools error: %v", err)
	}

	// Error should be captured in result, not returned
	if len(results) != 1 {
		t.Fatalf("len(results) = %d, want 1", len(results))
	}
	if !strings.Contains(results[0].Result, "error") {
		t.Errorf("result should contain error: %s", results[0].Result)
	}
}

func TestToolbox_Definitions(t *testing.T) {
	tb := NewToolbox()

	tb.Add(NewFuncTool(
		ToolDefinition{Name: "tool1", Description: "First tool"},
		func(ctx context.Context, args string) (string, error) { return "", nil },
	))
	tb.Add(NewFuncTool(
		ToolDefinition{Name: "tool2", Description: "Second tool"},
		func(ctx context.Context, args string) (string, error) { return "", nil },
	))

	defs := tb.Definitions()
	if len(defs) != 2 {
		t.Fatalf("len(defs) = %d, want 2", len(defs))
	}

	names := make(map[string]bool)
	for _, def := range defs {
		names[def.Name] = true
	}
	if !names["tool1"] || !names["tool2"] {
		t.Errorf("missing expected tool names: %v", names)
	}
}

func TestToolbox_ToolDefPrompt(t *testing.T) {
	tb := NewToolbox()

	tb.Add(NewFuncTool(
		ToolDefinition{
			Name:        "get_weather",
			Description: "Get weather for a city",
			Parameters: ToolParameters{
				Type: "object",
				Properties: map[string]ToolProperty{
					"city": {Type: "string", Description: "City name"},
				},
				Required: []string{"city"},
			},
		},
		func(ctx context.Context, args string) (string, error) { return "", nil },
	))

	prompt := tb.ToolDefinitionPrompt()

	if !strings.Contains(prompt, "get_weather") {
		t.Error("prompt should contain tool name")
	}
	if !strings.Contains(prompt, "Get weather for a city") {
		t.Error("prompt should contain tool description")
	}
}

func TestToolbox_ToolPrompt_Empty(t *testing.T) {
	tb := NewToolbox()

	prompt := tb.ToolDefinitionPrompt()
	if prompt != "" {
		t.Errorf("prompt = %s, want empty", prompt)
	}
}

func TestFuncTool_Definition(t *testing.T) {
	def := ToolDefinition{
		Name:        "test",
		Description: "Test tool",
		Parameters: ToolParameters{
			Type: "object",
			Properties: map[string]ToolProperty{
				"input": {Type: "string"},
			},
		},
	}

	tool := NewFuncTool(def, func(ctx context.Context, args string) (string, error) {
		return "", nil
	})

	got := tool.Definition()
	if got.Name != def.Name {
		t.Errorf("Name = %s, want %s", got.Name, def.Name)
	}
	if got.Description != def.Description {
		t.Errorf("Description = %s, want %s", got.Description, def.Description)
	}
}

func TestFuncTool_Call(t *testing.T) {
	tool := NewFuncTool(
		ToolDefinition{Name: "parser"},
		func(ctx context.Context, args string) (string, error) {
			var input struct {
				Value string `json:"value"`
			}
			if err := json.Unmarshal([]byte(args), &input); err != nil {
				return "", err
			}
			return "parsed: " + input.Value, nil
		},
	)

	result, err := tool.Call(context.Background(), `{"value":"test"}`)
	if err != nil {
		t.Fatalf("Call error: %v", err)
	}
	if result != "parsed: test" {
		t.Errorf("result = %s, want parsed: test", result)
	}
}

func TestToolDefinition_JSON(t *testing.T) {
	def := ToolDefinition{
		Name:        "search",
		Description: "Search the web",
		Parameters: ToolParameters{
			Type: "object",
			Properties: map[string]ToolProperty{
				"query": {
					Type:        "string",
					Description: "Search query",
				},
				"limit": {
					Type:        "integer",
					Description: "Max results",
				},
			},
			Required: []string{"query"},
		},
	}

	data, err := json.Marshal(def)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	var parsed ToolDefinition
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.Name != def.Name {
		t.Errorf("Name = %s, want %s", parsed.Name, def.Name)
	}
	if len(parsed.Parameters.Properties) != 2 {
		t.Errorf("len(Properties) = %d, want 2", len(parsed.Parameters.Properties))
	}
	if len(parsed.Parameters.Required) != 1 {
		t.Errorf("len(Required) = %d, want 1", len(parsed.Parameters.Required))
	}
}
