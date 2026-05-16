use std::sync::Arc;
use std::time::Instant;

use litcrypt2::lc;
use ureq::Agent;

use llmhub_core::{Error, Lease, PlatformClient, Result, UsageOutcome, UsageReport};

use crate::stream::drive_sse;
use crate::types::{ChatCompletion, ChatCompletionChunk, ChatCompletionRequest};

/// Stream callback. Return `false` to cancel the stream — driver will
/// return `Error::Cancelled` and the SDK will report the call with
/// `UsageOutcome::Success` and zero output_units.
pub type ChatStreamCallback<'a> = &'a mut dyn FnMut(ChatCompletionChunk) -> bool;

/// Chat completions client. One per (PlatformClient, sku_id) — the
/// caller decides how to share it. Internally it uses the shared
/// `PlatformClient`'s lease cache so cost of creating many of these
/// is tiny.
pub struct ChatClient {
    platform: Arc<PlatformClient>,
    sku_id: String,
}

impl ChatClient {
    pub fn new(platform: Arc<PlatformClient>, sku_id: impl Into<String>) -> Self {
        Self { platform, sku_id: sku_id.into() }
    }

    /// Non-streaming completion. Returns the full response object once
    /// the upstream finishes. Usage is auto-reported.
    pub fn create(&self, mut req: ChatCompletionRequest) -> Result<ChatCompletion> {
        let lease = self.platform.issue_lease(&self.sku_id, None)?;
        req.stream = false;
        if req.model.is_empty() {
            req.model = lease.upstream_model.clone();
        }

        let started = Instant::now();
        let result = self.send_blocking(&lease, &req);
        let latency_ms = started.elapsed().as_millis() as i64;

        match result {
            Ok(completion) => {
                self.report_success(&lease, &completion, latency_ms);
                Ok(completion)
            }
            Err(e) => {
                self.report_failure(&lease, &e, latency_ms, 0);
                if matches!(e, Error::Unauthorized) {
                    self.platform.invalidate(&self.sku_id);
                }
                Err(e)
            }
        }
    }

    /// Streaming completion. `on_chunk` is called once per SSE chunk;
    /// return `false` to cancel. Usage is auto-reported on completion.
    pub fn create_stream(
        &self,
        mut req: ChatCompletionRequest,
        on_chunk: ChatStreamCallback<'_>,
    ) -> Result<()> {
        let lease = self.platform.issue_lease(&self.sku_id, None)?;
        req.stream = true;
        if req.model.is_empty() {
            req.model = lease.upstream_model.clone();
        }

        let started = Instant::now();
        let mut ttfb_ms: i64 = 0;
        let mut output_chars: i64 = 0;
        let mut first = true;

        let result = self.send_stream(&lease, &req, &mut |chunk| {
            if first {
                ttfb_ms = started.elapsed().as_millis() as i64;
                first = false;
            }
            for ch in &chunk.choices {
                if let Some(c) = &ch.delta.content {
                    output_chars += c.chars().count() as i64;
                }
            }
            on_chunk(chunk)
        });
        let latency_ms = started.elapsed().as_millis() as i64;

        match result {
            Ok(()) => {
                self.report_stream_outcome(
                    &lease, UsageOutcome::Success, None, latency_ms, ttfb_ms, output_chars,
                );
                Ok(())
            }
            Err(Error::Cancelled) => {
                self.report_stream_outcome(
                    &lease, UsageOutcome::Success, None, latency_ms, ttfb_ms, output_chars,
                );
                Err(Error::Cancelled)
            }
            Err(e) => {
                let outcome = outcome_for(&e);
                let code = e.code();
                self.report_stream_outcome(
                    &lease, outcome, Some(code), latency_ms, ttfb_ms, output_chars,
                );
                if matches!(e, Error::Unauthorized) {
                    self.platform.invalidate(&self.sku_id);
                }
                Err(e)
            }
        }
    }

    fn send_blocking(&self, lease: &Lease, req: &ChatCompletionRequest) -> Result<ChatCompletion> {
        let url = chat_url(&lease.endpoint);
        let resp = self
            .agent()
            .post(&url)
            .set(&lc!("Content-Type"), &lc!("application/json"))
            .set(&lc!("Authorization"), &format!("{}{}", lc!("Bearer "), upstream_key(lease)?))
            .send_json(serde_json::to_value(req)?)
            .map_err(upstream_err)?;
        Ok(resp.into_json()?)
    }

    fn send_stream(
        &self,
        lease: &Lease,
        req: &ChatCompletionRequest,
        on_chunk: &mut dyn FnMut(ChatCompletionChunk) -> bool,
    ) -> Result<()> {
        let url = chat_url(&lease.endpoint);
        let resp = self
            .agent()
            .post(&url)
            .set(&lc!("Content-Type"), &lc!("application/json"))
            .set(&lc!("Accept"), &lc!("text/event-stream"))
            .set(&lc!("Authorization"), &format!("{}{}", lc!("Bearer "), upstream_key(lease)?))
            .send_json(serde_json::to_value(req)?)
            .map_err(upstream_err)?;
        drive_sse(resp.into_reader(), on_chunk)
    }

    fn agent(&self) -> &Agent { self.platform.agent_ref() }

    fn report_success(&self, lease: &Lease, c: &ChatCompletion, latency_ms: i64) {
        let report = UsageReport {
            lease_id: &lease.lease_id,
            request_id: None,
            input_units: c.usage.prompt_tokens,
            output_units: c.usage.completion_tokens,
            status: UsageOutcome::Success.as_str(),
            error_code: None,
            latency_ms,
            ttfb_ms: 0,
        };
        let _ = self.platform.report_usage(&report);
    }

    fn report_failure(&self, lease: &Lease, err: &Error, latency_ms: i64, output_units: i64) {
        let outcome = outcome_for(err);
        let report = UsageReport {
            lease_id: &lease.lease_id,
            request_id: None,
            input_units: 0,
            output_units,
            status: outcome.as_str(),
            error_code: Some(err.code()),
            latency_ms,
            ttfb_ms: 0,
        };
        let _ = self.platform.report_usage(&report);
    }

    fn report_stream_outcome(
        &self,
        lease: &Lease,
        outcome: UsageOutcome,
        error_code: Option<&str>,
        latency_ms: i64,
        ttfb_ms: i64,
        output_chars: i64,
    ) {
        // Without a final `usage` block we estimate output_tokens from
        // chars/4 — same rough conversion OpenAI's tokenizer-free SDKs use
        // when authoritative counts are absent. The server reconciles
        // against authoritative billing later.
        let output_units = output_chars / 4;
        let report = UsageReport {
            lease_id: &lease.lease_id,
            request_id: None,
            input_units: 0,
            output_units,
            status: outcome.as_str(),
            error_code,
            latency_ms,
            ttfb_ms,
        };
        let _ = self.platform.report_usage(&report);
    }
}

fn upstream_key(lease: &Lease) -> Result<&str> {
    lease
        .auth_payload
        .get("api_key")
        .map(String::as_str)
        .ok_or_else(|| Error::Platform {
            code: "auth_payload_missing".into(),
            message: "lease auth_payload has no api_key field".into(),
        })
}

fn chat_url(endpoint: &str) -> String {
    let base = endpoint.trim_end_matches('/');
    // Accept endpoints with or without a trailing /v1.
    if base.ends_with("/v1") {
        format!("{}{}", base, lc!("/chat/completions"))
    } else {
        format!("{}{}", base, lc!("/v1/chat/completions"))
    }
}

fn upstream_err(err: ureq::Error) -> Error {
    match err {
        ureq::Error::Status(status, resp) => {
            let body = resp.into_string().unwrap_or_default();
            match status {
                401 | 403 => Error::Unauthorized,
                429       => Error::Upstream { status, body },
                _         => Error::Upstream { status, body },
            }
        }
        ureq::Error::Transport(t) => Error::Network(t.to_string()),
    }
}

fn outcome_for(err: &Error) -> UsageOutcome {
    match err {
        Error::Unauthorized => UsageOutcome::AuthFailed,
        Error::Network(_)   => UsageOutcome::Timeout,
        Error::Upstream { status, .. } if *status == 429 => UsageOutcome::RateLimited,
        _ => UsageOutcome::UpstreamError,
    }
}
