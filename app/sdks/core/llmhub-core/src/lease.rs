use std::collections::HashMap;
use std::sync::Mutex;
use std::time::{Duration, Instant};

use litcrypt2::lc;
use serde::{Deserialize, Serialize};
use ureq::Agent;

use crate::error::{Error, Result};
use crate::transport::build_agent;

/// Mirror of the platform-side `sdkapi.IssueRequest`.
#[derive(Debug, Serialize)]
pub struct IssueRequest<'a> {
    pub sku_id: &'a str,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub client_fingerprint: Option<&'a str>,
}

/// Mirror of the platform-side `sdkapi.IssueResponse`. `auth_payload`
/// is the *real* upstream credential. Never log this, never expose it
/// across FFI; consumers should read fields they need (most often
/// `auth_payload["api_key"]` for OpenAI-compatible providers) and let
/// the rest stay in this struct.
#[derive(Debug, Clone, Deserialize)]
pub struct Lease {
    pub lease_id: String,
    pub expires_at: String,
    pub issued_at: String,
    pub vendor: String,
    pub vendor_product: String,
    pub capability: String,
    #[serde(default)]
    pub upstream_model: String,
    pub endpoint: String,
    pub protocol_family: String,
    #[serde(default)]
    pub auth_payload: HashMap<String, String>,
}

#[derive(Debug, Clone, Deserialize, Serialize)]
pub struct ServiceEntry {
    pub sku_id: String,
    #[serde(default)]
    pub plan_kind: String,
    #[serde(default)]
    pub plan_name: String,
    #[serde(default)]
    pub display_name: String,
    #[serde(default)]
    pub category_id: String,
    #[serde(default)]
    pub billing_unit: String,
    #[serde(default)]
    pub capability: String,
    #[serde(default)]
    pub quota_total: i64,
    #[serde(default)]
    pub quota_used: i64,
    #[serde(default)]
    pub quota_remaining: i64,
    #[serde(default)]
    pub qps_limit: i32,
    #[serde(default)]
    pub period_end: String,
}

#[derive(Deserialize)]
struct PlatformError {
    error: PlatformErrorBody,
}

#[derive(Deserialize)]
struct PlatformErrorBody {
    #[serde(rename = "type", default)]
    code: String,
    #[serde(default)]
    message: String,
}

#[derive(Deserialize)]
struct ServicesEnvelope {
    data: Vec<ServiceEntry>,
}

/// In-process lease cache. Returns the same lease for the same sku
/// until it gets within `refresh_skew` of expiry. Thread-safe; the
/// FFI layer expects to share one PlatformClient between threads.
struct LeaseCache {
    entries: Mutex<HashMap<String, CachedLease>>,
}

struct CachedLease {
    lease: Lease,
    /// Approximate wall-clock instant when we issued. Used together
    /// with a fixed 14-min lifetime as a coarse "is it still fresh"
    /// check; the platform's authoritative expiry is in `lease.expires_at`
    /// but we don't bother parsing RFC3339 on the client.
    issued_local: Instant,
}

const LEASE_FRESH_FOR: Duration = Duration::from_secs(14 * 60);

/// The platform-facing client. Owns the http agent + lease cache.
pub struct PlatformClient {
    base_url: String,
    api_key: String,
    agent: Agent,
    cache: LeaseCache,
}

impl PlatformClient {
    /// Construct a new client. `base_url` is the LLMHub deployment
    /// (no trailing slash); the SDK appends well-known paths itself
    /// — we don't take a full URL because the paths are part of the
    /// security contract, not user input.
    pub fn new(base_url: impl Into<String>, api_key: impl Into<String>) -> Result<Self> {
        let api_key = api_key.into();
        if api_key.is_empty() {
            return Err(Error::MissingApiKey);
        }
        Ok(Self {
            base_url: trim_trailing_slash(base_url.into()),
            api_key,
            agent: build_agent(),
            cache: LeaseCache { entries: Mutex::new(HashMap::new()) },
        })
    }

    /// Exposed so capability crates (chat-openai, …) can reuse the
    /// shared connection pool when calling upstream providers.
    pub fn agent_ref(&self) -> &Agent { &self.agent }
    pub(crate) fn base_url(&self) -> &str { &self.base_url }
    pub(crate) fn api_key(&self) -> &str { &self.api_key }

    /// Returns the user's subscribed SKUs. Maps to `GET /sdk/services`.
    pub fn list_services(&self) -> Result<Vec<ServiceEntry>> {
        let url = format!("{}{}", self.base_url, lc!("/sdk/services"));
        let resp = self
            .agent
            .get(&url)
            .set(&lc!("Authorization"), &format!("{}{}", lc!("Bearer "), self.api_key))
            .call()
            .map_err(map_platform_error)?;
        let env: ServicesEnvelope = resp.into_json()?;
        Ok(env.data)
    }

    /// Returns a fresh-or-cached lease for `sku_id`. The auth_payload
    /// inside is the platform-resolved upstream credential.
    pub fn issue_lease(&self, sku_id: &str, fingerprint: Option<&str>) -> Result<Lease> {
        if let Some(cached) = self.cached(sku_id) {
            return Ok(cached);
        }
        let url = format!("{}{}", self.base_url, lc!("/sdk/credentials/issue"));
        let body = IssueRequest { sku_id, client_fingerprint: fingerprint };
        let resp = self
            .agent
            .post(&url)
            .set(&lc!("Authorization"), &format!("{}{}", lc!("Bearer "), self.api_key))
            .send_json(serde_json::to_value(&body)?)
            .map_err(map_platform_error)?;
        let lease: Lease = resp.into_json()?;
        self.store(sku_id, &lease);
        Ok(lease)
    }

    /// Force-drop the cached lease for `sku_id`. Call after upstream
    /// auth_failed so the next request re-issues against the platform.
    pub fn invalidate(&self, sku_id: &str) {
        if let Ok(mut map) = self.cache.entries.lock() {
            map.remove(sku_id);
        }
    }

    fn cached(&self, sku_id: &str) -> Option<Lease> {
        let map = self.cache.entries.lock().ok()?;
        let entry = map.get(sku_id)?;
        if entry.issued_local.elapsed() < LEASE_FRESH_FOR {
            Some(entry.lease.clone())
        } else {
            None
        }
    }

    fn store(&self, sku_id: &str, lease: &Lease) {
        if let Ok(mut map) = self.cache.entries.lock() {
            map.insert(
                sku_id.to_string(),
                CachedLease { lease: lease.clone(), issued_local: Instant::now() },
            );
        }
    }
}

fn trim_trailing_slash(mut s: String) -> String {
    while s.ends_with('/') {
        s.pop();
    }
    s
}

/// Turn a ureq error into our typed Error, preserving the
/// platform-side code from the body when present.
fn map_platform_error(err: ureq::Error) -> Error {
    match err {
        ureq::Error::Status(status, resp) => {
            let body = resp.into_string().unwrap_or_default();
            if let Ok(pe) = serde_json::from_str::<PlatformError>(&body) {
                return match pe.error.code.as_str() {
                    "unauthorized"          => Error::Unauthorized,
                    "sku_not_found"         => Error::SkuNotFound(pe.error.message),
                    "sku_deprecated"        => Error::SkuDeprecated,
                    "not_subscribed"        => Error::NotSubscribed,
                    "quota_exceeded"        => Error::QuotaExceeded,
                    "daily_quota_exceeded"  => Error::QuotaExceeded,
                    "no_binding_available"  => Error::NoBindingAvailable,
                    "lease_not_found"       => Error::LeaseExpired,
                    code if !code.is_empty() => Error::Platform { code: code.into(), message: pe.error.message },
                    _ => Error::Upstream { status, body },
                };
            }
            Error::Upstream { status, body }
        }
        ureq::Error::Transport(t) => Error::Network(t.to_string()),
    }
}
