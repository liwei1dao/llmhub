use thiserror::Error;

/// SDK-wide error type. Maps 1:1 onto the codes the platform's
/// `sdkapi` layer returns, plus a few client-only variants
/// (network/timeout). Variants that the FFI layer needs to map to
/// platform-specific exceptions carry stable `code()` strings — never
/// renumber, only add.
#[derive(Debug, Error)]
pub enum Error {
    #[error("missing api key")]
    MissingApiKey,

    #[error("invalid api key")]
    Unauthorized,

    #[error("sku not found: {0}")]
    SkuNotFound(String),

    #[error("sku is deprecated")]
    SkuDeprecated,

    #[error("user is not subscribed to this sku")]
    NotSubscribed,

    #[error("subscription quota exhausted")]
    QuotaExceeded,

    #[error("no healthy upstream binding available")]
    NoBindingAvailable,

    #[error("lease expired or revoked")]
    LeaseExpired,

    #[error("platform error: {code}: {message}")]
    Platform { code: String, message: String },

    #[error("upstream {status}: {body}")]
    Upstream { status: u16, body: String },

    #[error("network: {0}")]
    Network(String),

    #[error("decode: {0}")]
    Decode(String),

    #[error("cancelled")]
    Cancelled,
}

impl Error {
    /// Stable machine-readable code used by FFI to construct
    /// platform-specific exceptions / error enums.
    pub fn code(&self) -> &'static str {
        match self {
            Error::MissingApiKey       => "missing_api_key",
            Error::Unauthorized        => "unauthorized",
            Error::SkuNotFound(_)      => "sku_not_found",
            Error::SkuDeprecated       => "sku_deprecated",
            Error::NotSubscribed       => "not_subscribed",
            Error::QuotaExceeded       => "quota_exceeded",
            Error::NoBindingAvailable  => "no_binding_available",
            Error::LeaseExpired        => "lease_expired",
            Error::Platform { .. }     => "platform_error",
            Error::Upstream { .. }     => "upstream_error",
            Error::Network(_)          => "network_error",
            Error::Decode(_)           => "decode_error",
            Error::Cancelled           => "cancelled",
        }
    }
}

pub type Result<T> = std::result::Result<T, Error>;

impl From<ureq::Error> for Error {
    fn from(err: ureq::Error) -> Self {
        match err {
            ureq::Error::Status(status, resp) => {
                let body = resp.into_string().unwrap_or_default();
                if status == 401 || status == 403 {
                    Error::Unauthorized
                } else {
                    Error::Upstream { status, body }
                }
            }
            ureq::Error::Transport(t) => Error::Network(t.to_string()),
        }
    }
}

impl From<serde_json::Error> for Error {
    fn from(err: serde_json::Error) -> Self {
        Error::Decode(err.to_string())
    }
}

impl From<std::io::Error> for Error {
    fn from(err: std::io::Error) -> Self {
        Error::Network(err.to_string())
    }
}
