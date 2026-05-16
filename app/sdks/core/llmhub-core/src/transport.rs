use std::time::Duration;

use ureq::{Agent, AgentBuilder};

/// Build the shared HTTP agent used for both platform calls (lease
/// issue / usage report) and upstream calls (chat completions). We
/// keep one agent so connection pooling is shared. Timeouts are tight
/// on connect, generous on read for streaming.
pub fn build_agent() -> Agent {
    AgentBuilder::new()
        .timeout_connect(Duration::from_secs(8))
        // Read timeout is only enforced between bytes, so SSE streams
        // that idle for >read will surface as a Network error and the
        // caller can decide whether to retry — usually they should.
        .timeout_read(Duration::from_secs(120))
        .user_agent(concat!("llmhub-sdk/", env!("CARGO_PKG_VERSION"), " (+rust)"))
        .build()
}
