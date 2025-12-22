package modelsocket

import "log/slog"

// --- Client Options ---

// ClientOption configures a ModelSocket client.
type ClientOption func(*clientConfig)

type clientConfig struct {
	logger    *slog.Logger
	onSend    func(*MSRequest)
	onReceive func(*MSEvent)
}

// WithLogger sets a structured logger for the client.
func WithLogger(logger *slog.Logger) ClientOption {
	return func(c *clientConfig) {
		c.logger = logger
	}
}

// WithOnSend sets a callback invoked before each request is sent.
func WithOnSend(fn func(*MSRequest)) ClientOption {
	return func(c *clientConfig) {
		c.onSend = fn
	}
}

// WithOnReceive sets a callback invoked after each event is received.
func WithOnReceive(fn func(*MSEvent)) ClientOption {
	return func(c *clientConfig) {
		c.onReceive = fn
	}
}

// --- Open Options ---

// OpenOption configures sequence opening.
type OpenOption func(*openConfig)

type openConfig struct {
	toolPrompt  string
	skipPrelude bool
	toolbox     *Toolbox
}

// WithToolPrompt sets the system prompt describing available tools.
func WithToolPrompt(prompt string) OpenOption {
	return func(c *openConfig) {
		c.toolPrompt = prompt
	}
}

// WithSkipPrelude skips the model's default prelude/system prompt.
func WithSkipPrelude() OpenOption {
	return func(c *openConfig) {
		c.skipPrelude = true
	}
}

// WithToolbox registers a toolbox for tool calling.
func WithToolbox(tb *Toolbox) OpenOption {
	return func(c *openConfig) {
		c.toolbox = tb
	}
}

// --- Append Options ---

// AppendOption configures text appending.
type AppendOption func(*appendConfig)

type appendConfig struct {
	role Role
	echo bool
}

// AsUser marks the message as from the user.
func AsUser() AppendOption {
	return func(c *appendConfig) {
		c.role = RoleUser
	}
}

// AsAssistant marks the message as from the assistant.
func AsAssistant() AppendOption {
	return func(c *appendConfig) {
		c.role = RoleAssistant
	}
}

// AsSystem marks the message as a system message.
func AsSystem() AppendOption {
	return func(c *appendConfig) {
		c.role = RoleSystem
	}
}

// WithEcho echoes the appended text back in events.
func WithEcho() AppendOption {
	return func(c *appendConfig) {
		c.echo = true
	}
}

// --- Generate Options ---

// GenOption configures text generation.
type GenOption func(*genConfig)

type genConfig struct {
	role          Role
	maxTokens     *int
	maxLength     *int
	temperature   *float64
	topP          *float64
	topK          *int
	repeatPenalty *float64
	seed          *int64
	stopStrings   []string
	regexMask     *string
	hidden        bool
}

// GenerateAsUser generates text as the user role.
func GenerateAsUser() GenOption {
	return func(c *genConfig) {
		c.role = RoleUser
	}
}

// GenerateAsAssistant generates text as the assistant role.
func GenerateAsAssistant() GenOption {
	return func(c *genConfig) {
		c.role = RoleAssistant
	}
}

// GenerateAsSystem generates text as the system role.
func GenerateAsSystem() GenOption {
	return func(c *genConfig) {
		c.role = RoleSystem
	}
}

// WithMaxTokens sets the maximum number of tokens to generate.
func WithMaxTokens(n int) GenOption {
	return func(c *genConfig) {
		c.maxTokens = &n
	}
}

// WithMaxLength sets the maximum length in characters.
func WithMaxLength(n int) GenOption {
	return func(c *genConfig) {
		c.maxLength = &n
	}
}

// WithTemperature sets the sampling temperature.
func WithTemperature(t float64) GenOption {
	return func(c *genConfig) {
		c.temperature = &t
	}
}

// WithTopP sets the nucleus sampling parameter.
func WithTopP(p float64) GenOption {
	return func(c *genConfig) {
		c.topP = &p
	}
}

// WithTopK sets the top-k sampling parameter.
func WithTopK(k int) GenOption {
	return func(c *genConfig) {
		c.topK = &k
	}
}

// WithRepeatPenalty sets the repetition penalty.
func WithRepeatPenalty(p float64) GenOption {
	return func(c *genConfig) {
		c.repeatPenalty = &p
	}
}

// WithSeed sets the random seed for reproducible generation.
func WithSeed(seed int64) GenOption {
	return func(c *genConfig) {
		c.seed = &seed
	}
}

// WithStopStrings sets strings that will stop generation.
func WithStopStrings(stops ...string) GenOption {
	return func(c *genConfig) {
		c.stopStrings = stops
	}
}

// WithRegexMask constrains generation to match a regex pattern.
func WithRegexMask(pattern string) GenOption {
	return func(c *genConfig) {
		c.regexMask = &pattern
	}
}

// WithHidden hides the generated text from the conversation history.
func WithHidden() GenOption {
	return func(c *genConfig) {
		c.hidden = true
	}
}

// Helper to convert genConfig to SeqGenData for wire format.
func (c *genConfig) toSeqGenData() SeqGenData {
	return SeqGenData{
		Role:          string(c.role),
		MaxTokens:     c.maxTokens,
		MaxLength:     c.maxLength,
		Temperature:   c.temperature,
		TopP:          c.topP,
		TopK:          c.topK,
		RepeatPenalty: c.repeatPenalty,
		Seed:          c.seed,
		StopStrings:   c.stopStrings,
		RegexMask:     c.regexMask,
		Hidden:        c.hidden,
	}
}
