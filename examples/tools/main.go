// Example demonstrating tool calling with ModelSocket.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/mixlayer/modelsocket-go"
)

// WeatherArgs is the input for the weather tool.
type WeatherArgs struct {
	Location string `json:"location"`
	City     string `json:"city"` // Model sometimes uses city instead of location
}

// WeatherResult is the output of the weather tool.
type WeatherResult struct {
	Temperature int    `json:"temperature,omitempty"`
	Units       string `json:"units,omitempty"`
	Error       string `json:"error,omitempty"`
}

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

	// Create the weather tool
	weatherTool := modelsocket.NewFuncTool(
		modelsocket.ToolDefinition{
			Name:        "get_weather",
			Description: "Retrieves the current weather for a city",
			Parameters: modelsocket.ToolParameters{
				Type: "object",
				Properties: map[string]modelsocket.ToolProperty{
					"city": {
						Type:        "string",
						Description: "City to retrieve the weather for.",
					},
				},
				Required: []string{"city"},
			},
		},
		func(ctx context.Context, argsJSON string) (string, error) {
			var args WeatherArgs
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				return "", err
			}

			// Handle both city and location params
			location := args.City
			if location == "" {
				location = args.Location
			}

			fmt.Printf("*** weather tool called with location: %s\n", location)

			var result WeatherResult
			if strings.Contains(strings.ToLower(location), "san francisco") {
				result = WeatherResult{Temperature: 60, Units: "F"}
			} else {
				result = WeatherResult{Error: "I don't know the weather in that city"}
			}

			resultJSON, _ := json.Marshal(result)
			return string(resultJSON), nil
		},
	)

	// Create toolbox and add the tool
	toolbox := modelsocket.NewToolbox()
	toolbox.Add(weatherTool)

	// Connect to server
	fmt.Printf("Connecting to %s...\n", url)
	client, err := modelsocket.Connect(ctx, url, apiKey)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to connect: %v\n", err)
		os.Exit(1)
	}
	defer client.Close(ctx)

	// Open a sequence with tools enabled
	fmt.Printf("Opening sequence with model %s and tools...\n", model)
	seq, err := client.Open(ctx, model,
		modelsocket.WithToolbox(toolbox),
		modelsocket.WithToolPrompt(toolbox.ToolDefPrompt()),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open sequence: %v\n", err)
		os.Exit(1)
	}
	defer seq.Close(ctx)

	// Send user message
	fmt.Println("\nUser: What's the weather in San Francisco?")
	err = seq.Append(ctx, "What's the weather in San Francisco?", modelsocket.AsUser())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to append: %v\n", err)
		os.Exit(1)
	}

	// Generate response (may include tool calls)
	fmt.Print("\nAssistant: ")
	stream, err := seq.Generate(ctx, modelsocket.GenerateAsAssistant())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to generate: %v\n", err)
		os.Exit(1)
	}

	var fullText strings.Builder
	var pendingToolCalls []modelsocket.ToolCall

	// Process the stream
	for {
		chunk, err := stream.Next(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
			break
		}
		if chunk == nil {
			break
		}

		// Print text as it streams
		if chunk.Text != "" && !chunk.Hidden {
			fmt.Print(chunk.Text)
			fullText.WriteString(chunk.Text)
		}

		// Collect tool calls - break immediately since server waits for tool return
		if len(chunk.ToolCalls) > 0 {
			pendingToolCalls = append(pendingToolCalls, chunk.ToolCalls...)
			break // Server won't send seq_gen_finish until we return tool results
		}
	}
	fmt.Println()

	// If there were tool calls, execute them and continue
	if len(pendingToolCalls) > 0 {
		fmt.Println("\n--- Executing tool calls ---")

		// Execute tools and collect results
		var results []modelsocket.ToolResult
		for _, tc := range pendingToolCalls {
			fmt.Printf("Calling tool: %s(%s)\n", tc.Name, tc.Args)
			result, err := toolbox.Call(ctx, tc.Name, tc.Args)
			if err != nil {
				result = fmt.Sprintf(`{"error": "%v"}`, err)
			}
			fmt.Printf("Result: %s\n", result)
			results = append(results, modelsocket.ToolResult{
				Name:   tc.Name,
				Result: result,
			})
		}

		// Return tool results to the model
		fmt.Println("\n--- Returning results to model ---")
		err = seq.ToolReturn(ctx, results)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to return tool results: %v\n", err)
			os.Exit(1)
		}

		// Generate the final response
		fmt.Print("\nAssistant (final): ")
		stream, err = seq.Generate(ctx, modelsocket.GenerateAsAssistant())
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to generate: %v\n", err)
			os.Exit(1)
		}

		text, err := stream.Text(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		fmt.Println(text)
	}

	fmt.Println("\nDone!")
}
