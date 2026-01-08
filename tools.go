package modelsocket

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
)

// Tool defines the interface for a callable tool.
type Tool interface {
	Definition() ToolDefinition
	Call(ctx context.Context, args string) (string, error)
}

// ToolDefinition describes a tool for the model.
type ToolDefinition struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  ToolParameters `json:"parameters"`
}

// ToolParameters defines the JSON Schema for tool parameters.
type ToolParameters struct {
	Type       string                  `json:"type"`
	Properties map[string]ToolProperty `json:"properties,omitempty"`
	Required   []string                `json:"required,omitempty"`
}

// ToolProperty defines a single parameter property.
type ToolProperty struct {
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	Enum        []string `json:"enum,omitempty"`
}

// Toolbox manages a collection of tools.
type Toolbox struct {
	mu                   sync.RWMutex
	tools                map[string]Tool
	toolInstructions     string
	toolDefinitionPrompt string
}

// NewToolbox creates an empty toolbox.
func NewToolbox() *Toolbox {
	return &Toolbox{
		tools: make(map[string]Tool),
	}
}

// Add registers a tool.
func (t *Toolbox) Add(tool Tool) {
	def := tool.Definition()
	t.mu.Lock()
	t.tools[def.Name] = tool
	t.mu.Unlock()
}

// Get retrieves a tool by name.
func (t *Toolbox) Get(name string) (Tool, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	tool, ok := t.tools[name]
	return tool, ok
}

// Call executes a tool by name with the given arguments.
func (t *Toolbox) Call(ctx context.Context, name string, args string) (string, error) {
	tool, ok := t.Get(name)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrToolNotFound, name)
	}
	return tool.Call(ctx, args)
}

// CallTools executes multiple tool calls and returns results.
func (t *Toolbox) CallTools(ctx context.Context, calls []ToolCall) ([]ToolResult, error) {
	results := make([]ToolResult, 0, len(calls))

	for _, call := range calls {
		result, err := t.Call(ctx, call.Name, call.Args)
		if err != nil {
			// Return error as result instead of failing
			result = fmt.Sprintf("error: %v", err)
		}
		results = append(results, ToolResult{
			Name:   call.Name,
			Result: result,
		})
	}

	return results, nil
}

// Definitions returns all tool definitions.
func (t *Toolbox) Definitions() []ToolDefinition {
	t.mu.RLock()
	defer t.mu.RUnlock()

	defs := make([]ToolDefinition, 0, len(t.tools))
	for _, tool := range t.tools {
		defs = append(defs, tool.Definition())
	}
	return defs
}

func (t *Toolbox) SetToolInstructions(instructions string) {
	t.toolInstructions = instructions
}

// ToolInstructions returns the tool instructions.
func (t *Toolbox) ToolInstructions() string {
	return t.toolInstructions
}

func (t *Toolbox) SetToolDefinitionPrompt(prompt string) {
	t.toolDefinitionPrompt = prompt
}

// ToolPrompt returns the tool prompt. If a custom prompt was set via SetToolPrompt,
// it returns that; otherwise it returns an auto-generated prompt describing all tools.
func (t *Toolbox) ToolDefinitionPrompt() string {
	if t.toolDefinitionPrompt != "" {
		return t.toolDefinitionPrompt
	}

	defs := t.Definitions()
	if len(defs) == 0 {
		return ""
	}

	data, _ := json.MarshalIndent(defs, "", "  ")
	return fmt.Sprintf("You have access to the following tools:\n\n%s\n\nTo use a tool, respond with a tool call in the appropriate format.", string(data))

}

// FuncTool wraps a function as a Tool.
type FuncTool struct {
	def ToolDefinition
	fn  func(ctx context.Context, args string) (string, error)
}

// NewFuncTool creates a tool from a function.
func NewFuncTool(def ToolDefinition, fn func(ctx context.Context, args string) (string, error)) *FuncTool {
	return &FuncTool{def: def, fn: fn}
}

// Definition returns the tool definition.
func (f *FuncTool) Definition() ToolDefinition {
	return f.def
}

// Call invokes the tool function.
func (f *FuncTool) Call(ctx context.Context, args string) (string, error) {
	return f.fn(ctx, args)
}
