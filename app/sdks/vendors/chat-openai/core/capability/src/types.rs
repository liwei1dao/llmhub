use serde::{Deserialize, Serialize};
use serde_json::Value;

// =====================================================================
// Roles + content (text + multimodal parts)
// =====================================================================

#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "lowercase")]
pub enum ChatRole {
    System,
    User,
    Assistant,
    Tool,
}

/// Polymorphic message content. The OpenAI wire accepts either a plain
/// string (the legacy / 99%-of-cases shape) or an array of typed parts
/// (the multimodal shape). We model both via an untagged enum so the
/// JSON round-trips unchanged.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(untagged)]
pub enum MessageContent {
    Text(String),
    Parts(Vec<ContentPart>),
}

impl Default for MessageContent {
    fn default() -> Self { MessageContent::Text(String::new()) }
}

/// One element inside a multimodal `content` array. Tagged on `type`,
/// matching OpenAI / Volc Ark / DeepSeek vision conventions.
#[derive(Debug, Clone, Serialize, Deserialize)]
#[serde(tag = "type", rename_all = "snake_case")]
pub enum ContentPart {
    Text { text: String },
    ImageUrl {
        #[serde(rename = "image_url")]
        image_url: ImageUrl,
    },
    InputAudio {
        #[serde(rename = "input_audio")]
        input_audio: InputAudio,
    },
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ImageUrl {
    /// Either an `https://…` URL or a `data:image/<mime>;base64,…` URL.
    pub url: String,
    /// `"auto" | "low" | "high"`. Optional; controls vision tile budget.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub detail: Option<String>,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct InputAudio {
    /// Base64-encoded audio bytes.
    pub data: String,
    /// `"wav" | "mp3"` per OpenAI's audio spec.
    pub format: String,
}

// =====================================================================
// Messages
// =====================================================================

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatMessage {
    pub role: ChatRole,
    /// Polymorphic — text-only OR an array of parts.
    pub content: MessageContent,
    /// Used by `role=tool` messages to bind back to a `tool_call_id`,
    /// and by some providers to label `name`-scoped user/assistant
    /// messages.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    /// Assistant-side: the function calls the model wants the host to
    /// execute. The host runs them and feeds results back as
    /// `role=tool` messages keyed on `tool_call_id`.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ToolCall>>,
    /// Tool-result-side: the id from the assistant's `tool_calls`
    /// that this message answers.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tool_call_id: Option<String>,
}

impl ChatMessage {
    pub fn user(content: impl Into<String>) -> Self {
        Self::text(ChatRole::User, content.into())
    }
    pub fn system(content: impl Into<String>) -> Self {
        Self::text(ChatRole::System, content.into())
    }
    pub fn assistant(content: impl Into<String>) -> Self {
        Self::text(ChatRole::Assistant, content.into())
    }
    pub fn tool(call_id: impl Into<String>, content: impl Into<String>) -> Self {
        Self {
            role: ChatRole::Tool,
            content: MessageContent::Text(content.into()),
            name: None,
            tool_calls: None,
            tool_call_id: Some(call_id.into()),
        }
    }
    fn text(role: ChatRole, content: String) -> Self {
        Self {
            role,
            content: MessageContent::Text(content),
            name: None,
            tool_calls: None,
            tool_call_id: None,
        }
    }
}

// =====================================================================
// Tools (function calling)
// =====================================================================

/// One tool the model is allowed to call. Today only `kind="function"`
/// exists in the OpenAI wire; we keep the shape so future tool kinds
/// (e.g. `"file_search"`) drop in without a breaking change.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Tool {
    #[serde(rename = "type")]
    pub kind: String,
    pub function: FunctionDef,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct FunctionDef {
    pub name: String,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub description: Option<String>,
    /// Raw JSON Schema describing the function's parameters.
    pub parameters: Value,
}

/// Assistant-emitted call. `arguments` is JSON encoded as a string —
/// that is the upstream's wire convention; do not double-encode.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolCall {
    pub id: String,
    #[serde(rename = "type")]
    pub kind: String,
    pub function: ToolFunctionCall,
}

#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolFunctionCall {
    pub name: String,
    pub arguments: String,
}

// =====================================================================
// Request / response envelopes
// =====================================================================

/// Wire-shape mirror of the OpenAI `POST /v1/chat/completions` body.
/// `model` is filled by the SDK from the lease's `upstream_model` —
/// callers pass a sku_id, not a model name.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ChatCompletionRequest {
    pub model: String,
    pub messages: Vec<ChatMessage>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub temperature: Option<f32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub top_p: Option<f32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub max_tokens: Option<u32>,
    #[serde(skip_serializing_if = "std::ops::Not::not")]
    pub stream: bool,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub stop: Option<Vec<String>>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tools: Option<Vec<Tool>>,
    /// `"auto" | "none" | "required"` or
    /// `{"type":"function","function":{"name":"my_fn"}}`. Forwarded raw;
    /// the SDK doesn't validate (different upstreams accept slightly
    /// different shapes).
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tool_choice: Option<Value>,
    /// OpenAI's parallel tool-call toggle; default `true` on most
    /// upstreams, kept explicit so apps can serialise.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub parallel_tool_calls: Option<bool>,
    /// Optional response-format hint. `{"type":"json_object"}` for JSON
    /// mode, or `{"type":"json_schema", …}` for structured output.
    /// Forwarded as-is.
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub response_format: Option<Value>,
}

impl Default for ChatCompletionRequest {
    fn default() -> Self {
        Self {
            model: String::new(),
            messages: Vec::new(),
            temperature: None,
            top_p: None,
            max_tokens: None,
            stream: false,
            stop: None,
            tools: None,
            tool_choice: None,
            parallel_tool_calls: None,
            response_format: None,
        }
    }
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct Usage {
    #[serde(default)]
    pub prompt_tokens: i64,
    #[serde(default)]
    pub completion_tokens: i64,
    #[serde(default)]
    pub total_tokens: i64,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ChatCompletionChoice {
    #[serde(default)]
    pub index: i32,
    pub message: ChatMessage,
    #[serde(default)]
    pub finish_reason: Option<String>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ChatCompletion {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub model: String,
    pub choices: Vec<ChatCompletionChoice>,
    #[serde(default)]
    pub usage: Usage,
}

// =====================================================================
// Streaming
// =====================================================================

/// One SSE chunk in stream mode. Mirrors OpenAI's
/// `chat.completion.chunk` exactly.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ChatCompletionChunk {
    #[serde(default)]
    pub id: String,
    #[serde(default)]
    pub model: String,
    pub choices: Vec<ChatChunkChoice>,
    #[serde(default)]
    pub usage: Option<Usage>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ChatChunkChoice {
    #[serde(default)]
    pub index: i32,
    #[serde(default)]
    pub delta: ChatDelta,
    #[serde(default)]
    pub finish_reason: Option<String>,
}

/// Streaming delta. `content` accumulates text chunks; `tool_calls`
/// carries incremental updates to each tool call by `index`. The
/// consumer is responsible for re-stitching by index across chunks
/// (the SDK exposes the raw frames so it can stay protocol-thin).
#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct ChatDelta {
    #[serde(default)]
    pub role: Option<String>,
    #[serde(default)]
    pub content: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub tool_calls: Option<Vec<ToolCallDelta>>,
}

/// Partial tool call inside a streaming delta. The first chunk for a
/// given `index` typically carries `id` + `kind` + the start of
/// `function.name`; later chunks carry slices of `function.arguments`
/// to be string-concatenated.
#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ToolCallDelta {
    pub index: i32,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub id: Option<String>,
    #[serde(rename = "type", default, skip_serializing_if = "Option::is_none")]
    pub kind: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub function: Option<ToolFunctionCallDelta>,
}

#[derive(Debug, Clone, Default, Deserialize, Serialize)]
pub struct ToolFunctionCallDelta {
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub name: Option<String>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    pub arguments: Option<String>,
}
