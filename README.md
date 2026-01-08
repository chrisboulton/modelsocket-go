# modelsocket-go

Go client for the ModelSocket protocol - WebSocket-based LLM integration with streaming, tool calling, and sequence forking.

## Install

```bash
go get github.com/chrisboulton/modelsocket-go
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

## Client

The `Client` manages the WebSocket connection and routes events to sequences. It's safe for concurrent use.

### Client Options

Pass options to `Connect()` to configure the client:

```go
client, err := modelsocket.Connect(ctx, url, apiKey,
    modelsocket.WithLogger(slog.Default()),
    modelsocket.WithOnSend(func(req *modelsocket.MSRequest) {
        // Called before each request is sent
    }),
    modelsocket.WithOnReceive(func(evt *modelsocket.MSEvent) {
        // Called after each event is received
    }),
)
```

| Option | Description |
|--------|-------------|
| `WithLogger(*slog.Logger)` | Structured logger for debug output |
| `WithOnSend(func(*MSRequest))` | Hook called before sending requests |
| `WithOnReceive(func(*MSEvent))` | Hook called after receiving events |

### Open Options

Configure sequences when calling `client.Open()`:

```go
seq, err := client.Open(ctx, "meta/llama3.1-8b-instruct-free",
    modelsocket.WithSkipPrelude(),
    modelsocket.WithToolbox(toolbox),
)
```

| Option | Description |
|--------|-------------|
| `WithSkipPrelude()` | Skip the model's default system prompt |
| `WithToolbox(*Toolbox)` | Enable tool calling with the provided toolbox |

### Custom Transport

Use `NewWithTransport()` to provide your own transport implementation:

```go
client := modelsocket.NewWithTransport(ctx, myTransport,
    modelsocket.WithLogger(logger),
)
```

Custom transports must implement the `Transport` interface:

```go
type Transport interface {
    Send(ctx context.Context, req *MSRequest) error
    Receive(ctx context.Context) (*MSEvent, error)
    Close() error
}
```

Use cases for custom transports:
- **Testing** - Mock transport for unit tests without network calls
- **Proxying** - Route through a custom proxy or middleware
- **Alternative protocols** - Use HTTP/SSE or other transports instead of WebSocket

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

// Optionally, you can customize the tool call instructions - what informs the
// model how to format tool calls. You should pretty much never need to do this
// toolbox.SetToolInstructions("....")

// You can also bring your own presenter for decorating tool definitions when
// passed into the model. It's up to you to ensure all tools with their definitions
// are listed.
// toolbox.SetToolDefinitionPrompt("You have access to the following)

// Open sequence with tools
seq, _ := client.Open(ctx, model,
    modelsocket.WithToolbox(toolbox),
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
