//! x-websearch, the Paxeer X Network web search sidecar.
//!
//! `config::load` reads the JSON configuration and refuses unknown fields,
//! missing fields, placeholders and key material, naming the field. Keys are
//! read only from the files `X_WEBSEARCH_ATTESTOR_KEY_FILE`,
//! `X_WEBSEARCH_SUBMITTER_KEY_FILE` and `X_WEBSEARCH_RECEIVER_KEY_FILE` name.
//! `server::Server` serves `GET /health` free, `GET /search` and `GET /fetch`
//! behind `payment::PaymentGate`, settling on the 0x101A ledger precompile.
//! `GET /content/<digest>` is served unpaid before serving eligibility.

pub mod api;
pub mod assets;
pub mod attest;
pub mod canonical;
pub mod config;
pub mod content;
pub mod crawl;
pub mod extract;
pub mod fetch;
mod http;
pub mod index;
pub mod keys;
pub mod payment;
pub mod program;
pub mod resolve;
pub mod robots;
pub mod search;
pub mod server;
pub mod settlement;
pub mod submit;
pub mod watch;

pub use config::{load, Config, ConfigError};
pub use keys::{KeyError, KeyFiles, Keys};
pub use server::{Limits, Request, Response, Route, RouteTable, RunningServer, Server};

use layerx_interop_gateway::adapter::{AdapterError, AdapterId, ConformanceSuite};

/// The conformance suite the sidecar's 402LXP seller is qualified against:
/// the recorded gateway exchange under `tests/fixtures/gateway`, including
/// the `eth_*` and `pax_sequencerReceipt` settlement recordings.
pub const CONFORMANCE_SUITE: &str = "x402-v2";

/// The number of recorded gateway answers in the suite.
pub const CONFORMANCE_VECTOR_COUNT: u64 = 131;

/// SHA-256 over the suite's recording files concatenated in file name order.
pub const CONFORMANCE_SUITE_DIGEST: [u8; 32] = [
    0x6a, 0xb6, 0x17, 0xfb, 0xe9, 0x8b, 0x36, 0x5d, 0x1d, 0xbc, 0xfa, 0x9e, 0x26, 0x68, 0xdd, 0x74,
    0x2c, 0x8b, 0xb3, 0x13, 0xe4, 0x5c, 0x01, 0x58, 0x80, 0xf3, 0xb8, 0xfd, 0x8f, 0xf5, 0x8f, 0xfb,
];

/// The pinned conformance suite the binary registers its x402 adapter with.
///
/// # Errors
/// Returns the gateway's refusal of the pinned identifier, count or digest.
pub fn conformance_suite() -> Result<ConformanceSuite, AdapterError> {
    ConformanceSuite::new(
        AdapterId::new(CONFORMANCE_SUITE)?,
        CONFORMANCE_VECTOR_COUNT,
        CONFORMANCE_SUITE_DIGEST,
    )
}
