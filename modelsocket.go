// Package modelsocket provides a Go client for the ModelSocket protocol.
//
// ModelSocket is a WebSocket-based protocol for efficiently integrating with
// Large Language Models (LLMs). It provides streaming text generation, tool
// calling, and sequence forking capabilities.
//
// # Thread Safety
//
// [Client] and [Seq] are safe for concurrent use by multiple goroutines.
// However, only one [Seq.Generate] call can be active per sequence at a time.
// [GenStream] should only be consumed by a single goroutine.
//
// # Basic Usage
//
//	ctx := context.Background()
//
//	// Connect to server
//	client, err := modelsocket.Connect(ctx, "wss://example.com/ws", "api-key")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer client.Close(ctx)
//
//	// Open a sequence
//	seq, err := client.Open(ctx, "model-name")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer seq.Close(ctx)
//
//	// Append user message
//	err = seq.Append(ctx, "Hello!", modelsocket.AsUser())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	// Generate response using iterator
//	stream, err := seq.Generate(ctx, modelsocket.GenerateAsAssistant())
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for chunk, err := range stream.Chunks(ctx) {
//	    if err != nil {
//	        log.Fatal(err)
//	    }
//	    fmt.Print(chunk.Text)
//	}
//
// # Observability
//
// Use [WithLogger], [WithOnSend], and [WithOnReceive] to add logging and
// monitoring to the client:
//
//	client, err := modelsocket.Connect(ctx, url, apiKey,
//	    modelsocket.WithLogger(slog.Default()),
//	    modelsocket.WithOnSend(func(req *modelsocket.MSRequest) {
//	        metrics.RequestsSent.Inc()
//	    }),
//	)
package modelsocket
