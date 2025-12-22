# modelsocket-go

Go client for the ModelSocket protocol - WebSocket-based LLM integration with streaming, tool calling, and sequence forking.

## Install

```bash
go get github.com/mixlayer/modelsocket-go
```

Requires Go 1.23+.

## Quick Start

```go
ctx := context.Background()

client, err := modelsocket.Connect(ctx, "wss://models.mixlayer.ai/ws", os.Getenv("API_KEY"))
if err != nil {
    log.Fatal(err)
}
defer client.Close(ctx)

seq, err := client.Open(ctx, "meta/llama3.1-8b-instruct-free")
if err != nil {
    log.Fatal(err)
}
defer seq.Close(ctx)

seq.Append(ctx, "Hello!", modelsocket.AsUser())

stream, _ := seq.Generate(ctx, modelsocket.GenerateAsAssistant())
for chunk, err := range stream.Chunks(ctx) {
    if err != nil {
        log.Fatal(err)
    }
    fmt.Print(chunk.Text)
}
```

## Features

- **Streaming generation** - Token-by-token output via iterator or Next()
- **Tool calling** - Register tools with Toolbox, handle ToolCall events
- **Sequence forking** - Branch conversations with `seq.Fork()`
- **Generation options** - Temperature, top_p, top_k, stop strings, max tokens

## Examples

```bash
export MODELSOCKET_API_KEY="your-key"

# Simple chat
go run ./examples/chat

# Tool calling
go run ./examples/tools
```

## Tool Calling

```go
// Define a tool
weatherTool := modelsocket.NewFuncTool(
    modelsocket.ToolDefinition{
        Name:        "get_weather",
        Description: "Get weather for a city",
        Parameters: modelsocket.ToolParameters{
            Type: "object",
            Properties: map[string]modelsocket.ToolProperty{
                "location": {Type: "string", Description: "City name"},
            },
            Required: []string{"location"},
        },
    },
    func(ctx context.Context, args string) (string, error) {
        return `{"temperature": 72, "units": "F"}`, nil
    },
)

// Register tools
toolbox := modelsocket.NewToolbox()
toolbox.Add(weatherTool)

// Open sequence with tools
seq, _ := client.Open(ctx, model,
    modelsocket.WithToolbox(toolbox),
    modelsocket.WithToolPrompt(toolbox.ToolDefPrompt()),
)

// Handle tool calls in stream
for chunk, err := range stream.Chunks(ctx) {
    if err != nil {
        break
    }
    fmt.Print(chunk.Text)
    if len(chunk.ToolCalls) > 0 {
        results, _ := toolbox.CallTools(ctx, chunk.ToolCalls)
        seq.ToolReturn(ctx, results)
    }
}
```

## API

| Type | Description |
|------|-------------|
| `Client` | Connection - Connect(), Open(), Close() |
| `Seq` | Sequence - Append(), Generate(), Fork(), Close() |
| `GenStream` | Stream - Next(), Chunks(), Text() |
| `Toolbox` | Tool registry - Add(), Call(), CallTools() |
