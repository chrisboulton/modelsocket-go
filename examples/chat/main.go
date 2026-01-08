// Example chat application using ModelSocket.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/chrisboulton/modelsocket-go"
)

func main() {
	url := os.Getenv("MODELSOCKET_URL")
	if url == "" {
		url = "wss://models.mixlayer.ai/ws"
	}

	apiKey := os.Getenv("MODELSOCKET_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "MODELSOCKET_API_KEY environment variable required")
		os.Exit(1)
	}

	model := os.Getenv("MODELSOCKET_MODEL")
	if model == "" {
		model = "meta/llama3.1-8b-instruct-free"
	}

	ctx := context.Background()

	// Connect to server
	fmt.Printf("Connecting to %s...\n", url)
	client, err := modelsocket.Connect(ctx, url, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close(ctx)

	// Open a sequence
	fmt.Printf("Opening sequence with model %s...\n", model)
	seq, err := client.Open(ctx, model)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open sequence: %v\n", err)
		os.Exit(1)
	}
	defer seq.Close(ctx)

	fmt.Println("Connected! Type your messages (Ctrl+D to exit)")
	fmt.Println()

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("You: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			break
		}

		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}

		// Append user message
		err = seq.Append(ctx, input, modelsocket.AsUser())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to append: %v\n", err)
			continue
		}

		// Generate response with streaming
		fmt.Print("Assistant: ")
		stream, err := seq.Generate(ctx, modelsocket.GenerateAsAssistant())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate: %v\n", err)
			continue
		}

		// Stream the response using iterator
		for chunk, err := range stream.Chunks(ctx) {
			if err != nil {
				fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
				break
			}
			if !chunk.Hidden {
				fmt.Print(chunk.Text)
			}
		}
		fmt.Println()
		fmt.Println()
	}

	fmt.Println("\nGoodbye!")
}
