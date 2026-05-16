use std::io::{BufRead, BufReader, Read};

use crate::types::ChatCompletionChunk;
use llmhub_core::{Error, Result};

/// Drives an SSE stream off `reader` and pushes each chunk into `on_chunk`.
/// Returns when the upstream sends `data: [DONE]`, the reader closes, or
/// `on_chunk` returns `false` (cancellation).
///
/// Frame format follows OpenAI: lines are `data: <json>` separated by
/// blank lines. We tolerate keep-alive empty lines and `event:` /
/// `:comment` lines by ignoring them.
pub fn drive_sse<R, F>(reader: R, mut on_chunk: F) -> Result<()>
where
    R: Read,
    F: FnMut(ChatCompletionChunk) -> bool,
{
    let mut buf = BufReader::with_capacity(8192, reader);
    let mut line = String::new();
    loop {
        line.clear();
        let n = buf.read_line(&mut line).map_err(|e| Error::Network(e.to_string()))?;
        if n == 0 {
            return Ok(());
        }
        let trimmed = line.trim_end_matches(['\r', '\n']);
        if trimmed.is_empty() || trimmed.starts_with(':') || trimmed.starts_with("event:") {
            continue;
        }
        let payload = match trimmed.strip_prefix("data: ").or_else(|| trimmed.strip_prefix("data:")) {
            Some(p) => p.trim_start(),
            None => continue, // tolerate non-data fields
        };
        if payload == "[DONE]" {
            return Ok(());
        }
        let chunk: ChatCompletionChunk = match serde_json::from_str(payload) {
            Ok(c) => c,
            Err(e) => return Err(Error::Decode(e.to_string())),
        };
        if !on_chunk(chunk) {
            return Err(Error::Cancelled);
        }
    }
}
