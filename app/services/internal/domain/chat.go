package domain

// ChatRequest is the platform-internal (OpenAI-shaped) chat request.
// Provider adapters translate this into their upstream format.
type ChatRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	Stream      bool          `json:"stream,omitempty"`
	Temperature *float32      `json:"temperature,omitempty"`
	TopP        *float32      `json:"top_p,omitempty"`
	MaxTokens   *int          `json:"max_tokens,omitempty"`
	Tools       []ChatTool    `json:"tools,omitempty"`
	ToolChoice  any           `json:"tool_choice,omitempty"`
	Stop        []string      `json:"stop,omitempty"`
	User        string        `json:"user,omitempty"`
}

// ChatMessage is a single message in a chat conversation.
type ChatMessage struct {
	Role       string        `json:"role"` // system / user / assistant / tool
	Content    any           `json:"content,omitempty"`
	Name       string        `json:"name,omitempty"`
	ToolCallID string        `json:"tool_call_id,omitempty"`
	ToolCalls  []ChatToolCall `json:"tool_calls,omitempty"`
}

// ChatTool declares an available tool/function.
type ChatTool struct {
	Type     string       `json:"type"`
	Function ChatFunction `json:"function"`
}

// ChatFunction is the JSON-schema style function description.
type ChatFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

// ChatToolCall is a tool invocation produced by the model.
type ChatToolCall struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"`
	Function ChatToolCallFunc  `json:"function"`
}

// ChatToolCallFunc carries the tool call's name + JSON args.
type ChatToolCallFunc struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// ChatResponse is the non-streaming response shape.
type ChatResponse struct {
	ID      string       `json:"id"`
	Object  string       `json:"object"`
	Created int64        `json:"created"`
	Model   string       `json:"model"`
	Choices []ChatChoice `json:"choices"`
	Usage   Usage        `json:"usage"`
}

// ChatChoice is a single completion choice.
type ChatChoice struct {
	Index        int          `json:"index"`
	Message      ChatMessage  `json:"message"`
	FinishReason string       `json:"finish_reason,omitempty"`
	Delta        *ChatMessage `json:"delta,omitempty"` // streaming only
}
