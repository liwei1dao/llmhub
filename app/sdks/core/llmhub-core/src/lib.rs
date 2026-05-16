//! `llmhub-core` — the part of the SDK that every capability shares.
//!
//! The platform's wire contract lives in `app/services/internal/sdkapi`.
//! This crate mirrors three of those endpoints:
//!
//! * `POST /sdk/credentials/issue`  → [`PlatformClient::issue_lease`]
//! * `POST /sdk/usage/report`        → [`PlatformClient::report_usage`]
//! * `GET  /sdk/services`            → [`PlatformClient::list_services`]
//!
//! The capability crates (chat-openai, audio-asr, …) consume `Lease`
//! and never re-implement transport. This is also where compile-time
//! string encryption is applied so that the well-known endpoint paths
//! ("/sdk/credentials/issue", "Bearer ", …) don't sit as plaintext
//! constants in the shipped .so.
#![forbid(unsafe_code)]
#![deny(rust_2018_idioms)]

extern crate alloc; // litcrypt2 expands to `alloc::string::String::from(...)`

litcrypt2::use_litcrypt!();

pub mod error;
pub mod lease;
pub mod report;
pub mod transport;

pub use error::{Error, Result};
pub use lease::{IssueRequest, Lease, PlatformClient, ServiceEntry};
pub use report::{UsageOutcome, UsageReport};
