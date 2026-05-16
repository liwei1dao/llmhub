use litcrypt2::lc;
use serde::Serialize;

use crate::error::Result;
use crate::lease::PlatformClient;

/// SDK-reported call outcome. Mirrors the platform's `report.go`
/// status enum verbatim — values are wire-stable.
#[derive(Debug, Clone, Copy)]
pub enum UsageOutcome {
    Success,
    RateLimited,
    AuthFailed,
    Timeout,
    UpstreamError,
}

impl UsageOutcome {
    pub fn as_str(self) -> &'static str {
        match self {
            UsageOutcome::Success       => "success",
            UsageOutcome::RateLimited   => "rate_limited",
            UsageOutcome::AuthFailed    => "auth_failed",
            UsageOutcome::Timeout       => "timeout",
            UsageOutcome::UpstreamError => "upstream_error",
        }
    }
}

/// One usage event. Construct via the builder methods on the
/// capability crates; this struct stays close to the wire shape.
#[derive(Debug, Serialize)]
pub struct UsageReport<'a> {
    pub lease_id: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub request_id: Option<&'a str>,
    #[serde(skip_serializing_if = "is_zero")]
    pub input_units: i64,
    #[serde(skip_serializing_if = "is_zero")]
    pub output_units: i64,
    pub status: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error_code: Option<&'a str>,
    #[serde(skip_serializing_if = "is_zero")]
    pub latency_ms: i64,
    #[serde(skip_serializing_if = "is_zero")]
    pub ttfb_ms: i64,
}

fn is_zero(n: &i64) -> bool { *n == 0 }

impl PlatformClient {
    /// Submit a usage report. Best-effort: a non-2xx surfaces as an
    /// error so the caller can log it, but the SDK's main code path
    /// should never block on report success.
    pub fn report_usage(&self, report: &UsageReport<'_>) -> Result<()> {
        let url = format!("{}{}", self.base_url(), lc!("/sdk/usage/report"));
        self.agent_ref()
            .post(&url)
            .set(&lc!("Authorization"), &format!("{}{}", lc!("Bearer "), self.api_key()))
            .send_json(serde_json::to_value(report)?)?;
        Ok(())
    }
}
