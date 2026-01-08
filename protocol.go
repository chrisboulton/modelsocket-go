package modelsocket

// SeqState represents the state of a sequence.
type SeqState string

const (
	StateReady      SeqState = "ready"
	StateAppending  SeqState = "appending"
	StateGenerating SeqState = "generating"
	StateToolCall   SeqState = "tool_call"
	StateForking    SeqState = "forking"
	StateClosed     SeqState = "closed"
)

// Role represents the role of a message in a conversation.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
	RoleSystem    Role = "system"
)

// --- Requests (Client -> Server) ---

// MSRequest represents a request sent to the server.
type MSRequest struct {
	Request string      `json:"request"`
	CID     string      `json:"cid"`
	SeqID   string      `json:"seq_id,omitempty"`
	Data    interface{} `json:"data"`
}

// SeqOpenData is the data for a seq_open request.
type SeqOpenData struct {
	Model        string `json:"model"`
	ToolsEnabled bool   `json:"tools_enabled,omitempty"`
	ToolPrompt   string `json:"tool_prompt,omitempty"`
	SkipPrelude  bool   `json:"skip_prelude,omitempty"`
}

// SeqAppendData is the data for an append command.
type SeqAppendData struct {
	Text   string `json:"text"`
	Role   string `json:"role,omitempty"`
	Echo   bool   `json:"echo,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
}

// SeqGenData is the data for a gen command.
type SeqGenData struct {
	Role          string   `json:"role,omitempty"`
	MaxTokens     *int     `json:"max_tokens,omitempty"`
	MaxLength     *int     `json:"max_length,omitempty"`
	Temperature   *float64 `json:"temperature,omitempty"`
	TopP          *float64 `json:"top_p,omitempty"`
	TopK          *int     `json:"top_k,omitempty"`
	RepeatPenalty *float64 `json:"repeat_penalty,omitempty"`
	Seed          *int64   `json:"seed,omitempty"`
	StopStrings   []string `json:"stop_strings,omitempty"`
	RegexMask     *string  `json:"regex_mask,omitempty"`
	Hidden        bool     `json:"hidden,omitempty"`
	PrefillText   *string  `json:"prefill_text,omitempty"`
	ReturnTokens  *bool    `json:"return_tokens,omitempty"`
}

// ToolResult represents the result of a tool call.
type ToolResult struct {
	Name   string `json:"name"`
	Result string `json:"result"`
}

// Command data wrappers for wire format.
type appendCommandData struct {
	Command string `json:"command"`
	SeqAppendData
}

type genCommandData struct {
	Command string `json:"command"`
	SeqGenData
}

type closeCommandData struct {
	Command string `json:"command"`
}

type forkCommandData struct {
	Command string `json:"command"`
}

type toolReturnCommandData struct {
	Command string       `json:"command"`
	Results []ToolResult `json:"results"`
	GenOpts SeqGenData   `json:"gen_opts"`
}

// NewSeqOpenRequest creates a new seq_open request.
func NewSeqOpenRequest(cid string, data SeqOpenData) *MSRequest {
	return &MSRequest{
		Request: "seq_open",
		CID:     cid,
		Data:    data,
	}
}

// NewAppendRequest creates a new append command request.
func NewAppendRequest(cid, seqID string, data SeqAppendData) *MSRequest {
	return &MSRequest{
		Request: "seq_command",
		CID:     cid,
		SeqID:   seqID,
		Data: appendCommandData{
			Command:       "append",
			SeqAppendData: data,
		},
	}
}

// NewGenRequest creates a new gen command request.
func NewGenRequest(cid, seqID string, data SeqGenData) *MSRequest {
	return &MSRequest{
		Request: "seq_command",
		CID:     cid,
		SeqID:   seqID,
		Data: genCommandData{
			Command:    "gen",
			SeqGenData: data,
		},
	}
}

// NewCloseRequest creates a new close command request.
func NewCloseRequest(cid, seqID string) *MSRequest {
	return &MSRequest{
		Request: "seq_command",
		CID:     cid,
		SeqID:   seqID,
		Data: closeCommandData{
			Command: "close",
		},
	}
}

// NewForkRequest creates a new fork command request.
func NewForkRequest(cid, seqID string) *MSRequest {
	return &MSRequest{
		Request: "seq_command",
		CID:     cid,
		SeqID:   seqID,
		Data: forkCommandData{
			Command: "fork",
		},
	}
}

// NewToolReturnRequest creates a new tool_return command request.
func NewToolReturnRequest(cid, seqID string, results []ToolResult, genOpts SeqGenData) *MSRequest {
	return &MSRequest{
		Request: "seq_command",
		CID:     cid,
		SeqID:   seqID,
		Data: toolReturnCommandData{
			Command: "tool_return",
			Results: results,
			GenOpts: genOpts,
		},
	}
}

// --- Events (Server -> Client) ---

// MSEvent represents an event received from the server.
type MSEvent struct {
	Event string `json:"event"`

	// Common fields
	SeqID string `json:"seq_id,omitempty"`
	CID   string `json:"cid,omitempty"`

	// SeqText fields
	Text            string `json:"text,omitempty"`
	Hidden          bool   `json:"hidden,omitempty"`
	NumInputTokens  int    `json:"num_input_tokens,omitempty"`
	NumOutputTokens int    `json:"num_output_tokens,omitempty"`
	Tokens          []int  `json:"tokens,omitempty"`

	// SeqToolCall fields
	ToolCalls []SeqToolCall `json:"tool_calls,omitempty"`

	// SeqForkFinish fields
	ChildSeqID string `json:"child_seq_id,omitempty"`

	// SeqState fields
	State SeqState `json:"state,omitempty"`

	// SeqClosed fields
	InputTokens  int    `json:"input_tokens,omitempty"`
	OutputTokens int    `json:"output_tokens,omitempty"`
	DurationMs   int64  `json:"duration_ms,omitempty"`
	ErrorMsg     string `json:"error,omitempty"`

	// Error fields
	Message string `json:"message,omitempty"`
}

// SeqToolCall represents a tool call from the model.
type SeqToolCall struct {
	Name string `json:"name"`
	Args string `json:"args"`
}

// Type returns the event type.
func (e *MSEvent) Type() string {
	return e.Event
}

// IsSeqOpened returns true if this is a seq_opened event.
func (e *MSEvent) IsSeqOpened() bool {
	return e.Event == "seq_opened"
}

// IsSeqText returns true if this is a seq_text event.
func (e *MSEvent) IsSeqText() bool {
	return e.Event == "seq_text"
}

// IsSeqToolCall returns true if this is a seq_tool_call event.
func (e *MSEvent) IsSeqToolCall() bool {
	return e.Event == "seq_tool_call"
}

// IsSeqAppendFinish returns true if this is a seq_append_finish event.
func (e *MSEvent) IsSeqAppendFinish() bool {
	return e.Event == "seq_append_finish"
}

// IsSeqGenFinish returns true if this is a seq_gen_finish event.
func (e *MSEvent) IsSeqGenFinish() bool {
	return e.Event == "seq_gen_finish"
}

// IsSeqForkFinish returns true if this is a seq_fork_finish event.
func (e *MSEvent) IsSeqForkFinish() bool {
	return e.Event == "seq_fork_finish"
}

// IsSeqState returns true if this is a seq_state event.
func (e *MSEvent) IsSeqState() bool {
	return e.Event == "seq_state"
}

// IsSeqClosed returns true if this is a seq_closed event.
func (e *MSEvent) IsSeqClosed() bool {
	return e.Event == "seq_closed"
}

// IsError returns true if this is an error event.
func (e *MSEvent) IsError() bool {
	return e.Event == "error"
}
