//! Chat completions over the OpenAI-compatible wire protocol.
//!
//! Flow:
//!
//! 1. [`ChatClient::new`] wraps a shared [`llmhub_core::PlatformClient`].
//! 2. On each call we [`PlatformClient::issue_lease`] for the sku,
//!    pulling the upstream `endpoint` + `auth_payload`.
//! 3. We POST to `{endpoint}/chat/completions` with the lease's
//!    `auth_payload["api_key"]` as Bearer — *this is the real upstream
//!    credential*; it must never escape into FFI / consumer code.
//! 4. For streaming we parse SSE inline and pump deltas via a callback.
//! 5. After the call completes (or fails) we
//!    [`PlatformClient::report_usage`].
#![forbid(unsafe_code)]
#![deny(rust_2018_idioms)]

extern crate alloc;

litcrypt2::use_litcrypt!();

mod client;
mod stream;
mod types;

pub use client::{ChatClient, ChatStreamCallback};
pub use types::{
    ChatChunkChoice, ChatCompletion, ChatCompletionChoice, ChatCompletionChunk,
    ChatCompletionRequest, ChatDelta, ChatMessage, ChatRole, ContentPart, FunctionDef,
    ImageUrl, InputAudio, MessageContent, Tool, ToolCall, ToolCallDelta, ToolFunctionCall,
    ToolFunctionCallDelta, Usage,
};
