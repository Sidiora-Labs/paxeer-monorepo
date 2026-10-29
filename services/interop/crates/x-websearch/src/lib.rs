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
pub mod index;
pub mod keys;
pub mod payment;
pub mod program;
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
    0x7c, 0xb1, 0x57, 0x18, 0x43, 0x97, 0x0b, 0x81, 0x5b, 0xeb, 0x4e, 0x77, 0x45, 0x77, 0xe7, 0x77,
    0x00, 0xd2, 0x9d, 0xb7, 0x2f, 0xe7, 0x76, 0x1b, 0x46, 0x15, 0xf1, 0x44, 0x1b, 0x59, 0x99, 0xc3,
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
