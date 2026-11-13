//! Task 22.33.1439 finding/test record:
//! ai-views-unmounted -> `ai_views_ingest_restart_cursor_freshness_and_revocation`.
//! node-root-test -> `node_root_and_explicit_chain_paths_reject_router_paths`.
//! validator-pagination-bound -> `validator_boundary_refusal_deadline_and_recovery`.
//! history-unneeded-registry, history-native-test-gap -> `history_native_and_complete_metadata_skip_registry`.
//! history-registry-errors -> `history_registry_preserves_exact_failure_categories`.
//! history-requirement-conflict -> `history_preserves_transaction_hash_and_exact_recording`.
//! readiness-kernel-drift -> `readiness_ignores_legacy_backends_and_observes_live_dependencies`.
//! Prior attempt: 27 passed; eight failed during fixture setup (four single-node
//! configurations and four source-success records without head observations).
//! Wiring request: spec/layerx-merge/spec.kvx, history owner/domain 8: reconcile 181092.1 with 99713.1 retaining `tx_hash`.
//! Wiring request: services/platform/hosted/gateway/openrpc.json, gateway contract owner/domain 8: publish `tx_hash` in both closed history schemas.
//! Wiring request: services/agent/sdk/typescript/src/history.ts and apps/web/src/api/gateway.ts, SDK/web consumer owners: decode the unchanged producer history recording including `tx_hash`.

// Compile the committed producer codecs in this integration crate because
// their manifests are outside the repair scope. No replacement codecs are used.
extern crate alloc;
extern crate self as layerx_program_sdk;
extern crate self as layerx_programs_ai_market;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/abi.rs"]
pub mod abi;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/abi_policy.rs"]
pub mod abi_policy;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/access.rs"]
pub mod access;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/amount.rs"]
pub mod amount;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/arbiter.rs"]
pub mod arbiter;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/balance.rs"]
pub mod balance;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/buffer.rs"]
pub mod buffer;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/call.rs"]
pub mod call;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/capability.rs"]
pub mod capability;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/context.rs"]
pub mod context;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/crypto.rs"]
pub mod crypto;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/error.rs"]
pub mod error;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/event.rs"]
pub mod event;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/ids.rs"]
pub mod ids;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/lxt20.rs"]
pub mod lxt20;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/lxt721.rs"]
pub mod lxt721;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/naming.rs"]
pub mod naming;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/oracle.rs"]
pub mod oracle;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/payments.rs"]
pub mod payments;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/proposal.rs"]
pub mod proposal;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/receipt.rs"]
pub mod receipt;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/storage.rs"]
pub mod storage;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/transfer.rs"]
pub mod transfer;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/sdk/rust/src/web.rs"]
pub mod web;
pub use abi::{
    HostFunction, ABI_MANIFEST, ABI_MODULE, ABI_VERSION, CALL_ENTRY_EXPORT, CALL_RESERVE_EXPORT, CANDIDATE_ABI_MANIFEST, CANDIDATE_ABI_MODULE, CANDIDATE_HOST_FUNCTIONS, CANDIDATE_REFUSAL_SENTINEL, ENTRYPOINT, HOST_FUNCTIONS, MAX_CALL_INPUT_BYTES, MAX_CALL_RESPONSE_BYTES,
    MAX_CANONICAL_CAPABILITY_SET_BYTES, MAX_CAPABILITIES, MAX_CAPABILITY_ENCODING_BYTES, MAX_CAPABILITY_ENCODING_GRANT_BYTES, MAX_CAPABILITY_ENCODING_HEADER_BYTES, MAX_EVENTS_PER_ACTIVITY, MAX_EVENT_DATA_BYTES, MAX_EVENT_TOPIC_BYTES, MAX_PROGRAM_ACCOUNT_SEED_BYTES, MAX_REFUSAL_REASON_BYTES,
    MAX_STORAGE_KEY_BYTES, MAX_STORAGE_VALUE_BYTES, MEMORY_EXPORT, RECEIPT_ENCODING_BYTES,
};
pub use access::{AccessEntry, AccessMode, AccessRecipe, AccessScope, ActivityAccess, KeyAccess};
pub use amount::{Amount, ProtocolInteger};
pub use buffer::Bytes;
pub use call::{CallInput, CallResponse, CallResult, GrantedCapabilities};
pub use capability::{Capability, CapabilitySet};
pub use context::{Context, Principal};
#[cfg(target_arch = "wasm32")]
pub use entry::EntryResponse;
pub use error::{
    Field, HostRefusal, ProgramError, ProgramRefusal, Reason, RefusalClass, RefusalReason, ValueError, REFUSAL_CLASS_MANIFEST, STATUS_BOUNDS, STATUS_BUFFER_TOO_SMALL, STATUS_CAPABILITY_BYTES, STATUS_CAPABILITY_LIMIT, STATUS_DATA_TOO_LARGE, STATUS_DENIED, STATUS_DUPLICATE_CAPABILITY,
    STATUS_EMPTY_KEY, STATUS_EMPTY_TOPIC, STATUS_EVIDENCE, STATUS_INPUT_TOO_LARGE, STATUS_INVALID, STATUS_KEY_TOO_LARGE, STATUS_METER, STATUS_NULL_ARGUMENT, STATUS_OVERFLOW, STATUS_RECEIPT_ENCODING, STATUS_RESERVED_IDENTIFIER, STATUS_TOPIC_TOO_LARGE, STATUS_UNDERFLOW, STATUS_VALUE_TOO_LARGE,
    STATUS_VERIFY_FAILED, STATUS_ZERO_AMOUNT,
};
pub use event::{EventData, EventTopic};
pub use ids::{AccountId, AssetId, ProgramId, ReceiptDigest};
pub use oracle::{Observation, OBSERVATION_BYTES};
pub use receipt::Receipt;
pub use storage::{StorageKey, StorageValue};
pub use transfer::{Payment, ProgramAccountPayment, ProgramAccountSeed, ProgramDeposit};
pub use web::{Answer, ANSWER_HEADER_BYTES, MAX_RESPONSE_BYTES, RECORD_BYTES};

#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/admission.rs"]
pub mod admission;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/aggregation.rs"]
pub mod aggregation;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/aggregation_codec.rs"]
pub mod aggregation_codec;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/canary.rs"]
pub mod canary;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/closure.rs"]
pub mod closure;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/codec.rs"]
pub mod codec;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src"]
mod ai_nested {
    pub mod commit_reveal;
}
pub use ai_nested::commit_reveal;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/dispatch.rs"]
pub mod dispatch;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/epoch.rs"]
pub mod epoch;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/errors.rs"]
pub mod errors;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/evidence.rs"]
pub mod evidence;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/evaluators"]
pub mod evaluators {
    #[path = "admission.rs"]
    pub mod admission;
    #[path = "authority.rs"]
    pub mod authority;
    #[path = "codec.rs"]
    pub mod codec;
    #[path = "model.rs"]
    pub mod model;
}
#[cfg(target_arch = "wasm32")]
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/guest_cell.rs"]
mod guest_cell;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/host_adapter.rs"]
pub mod host_adapter;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/laps.rs"]
pub mod laps;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/policy.rs"]
pub mod policy;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/queries.rs"]
pub mod queries;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/registry.rs"]
pub mod registry;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/registry_ops.rs"]
pub mod registry_ops;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/reputation.rs"]
pub mod reputation;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/reputation_codec.rs"]
pub mod reputation_codec;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/reputation_transition.rs"]
pub mod reputation_transition;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/reward_math.rs"]
pub mod reward_math;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/rewards.rs"]
pub mod rewards;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/rooms.rs"]
pub mod rooms;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/roster.rs"]
pub mod roster;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/state.rs"]
pub mod state;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/tasks.rs"]
pub mod tasks;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/types.rs"]
pub mod types;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/value_adapter.rs"]
pub mod value_adapter;
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/src/workers.rs"]
pub mod workers;
pub use errors::{ApplicationError, CodecResult};
pub const MERGED_CHAIN_DOMAIN: [u8; 32] = [
    0x62, 0xb6, 0xb1, 0x47, 0x34, 0xfa, 0x76, 0x9c, 0x3e, 0x0f, 0xbc, 0x94, 0xde, 0x49, 0x5a, 0x3d, 0x03, 0x8b, 0xe4, 0x9a, 0x25, 0x24, 0xb1, 0xc0, 0x71, 0xd7, 0x5d, 0x0a, 0x4e, 0xed, 0x71, 0xf6,
];
pub const SCHEMA_VERSION: u16 = 1;
pub const SHARED_STATE_KEY: &[u8] = b"paxai/state/v1";
pub const REWARDS_ACCOUNT_SEED: &[u8] = b"paxai/rewards/v1";
pub const MAX_ENVELOPE_BYTES: usize = 16_384;
pub const MAX_PAYLOAD_BYTES: usize = 15_965;
pub const MAX_RESULT_BYTES: usize = 16_384;
pub const RESULT_HEADER_BYTES: usize = 82;
pub const MAX_RESULT_PAYLOAD_BYTES: usize = 16_302;
pub const MAX_STATE_BYTES: usize = 196_608;
pub const MAX_EVENT_BYTES: usize = 2_048;
pub const MAX_EVENTS: usize = 2;
pub const MAX_WORKERS: usize = 32;
pub const MAX_EVALUATORS: usize = 8;
pub const MAX_TASKS: usize = 64;
pub const MAX_RETAINED_EPOCHS: usize = 32;
pub const MAX_PAYOUT_IDENTITIES: usize = 256;
pub const MAX_CHUNK_BYTES: usize = 8_192;

#[cfg(test)]
#[allow(unused, clippy::all, clippy::pedantic, clippy::unwrap_used, clippy::expect_used, clippy::float_arithmetic, clippy::lossy_float_literal)]
#[path = "../../../../../programs/crates/layerx-programs-ai-market/tests/support/repair_478_fixture.rs"]
mod repair_478_fixture;

use layerx_indexer::{
    api,
    store::{AssetRow, Store, TransferRow, Unit},
};
use layerx_platform_gateway::gateway_digest;
use layerx_platform_gateway::http::{self};
use layerx_platform_gateway::runtime::Server;
use layerx_platform_gateway::store::{KeyRecord, RedisEndpoint, RedisStore};
use rustls::{
    pki_types::{PrivateKeyDer, PrivatePkcs8KeyDer},
    ServerConfig,
};
use serde_json::{json, Value};
use std::{
    ffi::OsString,
    fs,
    io::{Read, Write},
    net::{SocketAddr, TcpStream},
    sync::{
        atomic::{AtomicUsize, Ordering},
        Arc, Mutex,
    },
    time::{Duration, Instant},
};
use tempfile::TempDir;
use zeroize::Zeroizing;

#[path = "support/repair_22_31_202.rs"]
pub mod support;
use support::{fixture, json_response, Peer, RedisMemory};
static ENVIRONMENT: Mutex<()> = Mutex::new(());
const ACCOUNT: &str = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee";
const TOKEN: &str = "evm:0xcccccccccccccccccccccccccccccccccccccccc";

struct Environment(Vec<(OsString, OsString)>);
impl Environment {
    fn clear() -> Self {
        let old: Vec<_> = std::env::vars_os().filter(|(k, _)| k.to_string_lossy().starts_with("LAYERX_GATEWAY_")).collect();
        for (k, _) in &old {
            std::env::remove_var(k);
        }
        Self(old)
    }
}
impl Drop for Environment {
    fn drop(&mut self) {
        for (k, _) in std::env::vars_os().filter(|(k, _)| k.to_string_lossy().starts_with("LAYERX_GATEWAY_")) {
            std::env::remove_var(k);
        }
        for (k, v) in &self.0 {
            std::env::set_var(k, v);
        }
    }
}
struct Fixture {
    directory: TempDir,
    ca: native_tls::Certificate,
    tls: Arc<ServerConfig>,
    redis: Peer,
    node: Peer,
    second_node: Peer,
    node_paths: Arc<Mutex<Vec<String>>>,
}
impl Fixture {
    fn start() -> Self {
        let directory = tempfile::tempdir().unwrap_or_else(|error| panic!("{}: {error:?}", "fixture directory"));
        let certificate = rcgen::generate_simple_self_signed(vec!["localhost".into()]).unwrap_or_else(|error| panic!("{}: {error:?}", "certificate"));
        fs::write(directory.path().join("ca.der"), certificate.cert.der()).unwrap_or_else(|error| panic!("{}: {error:?}", "CA file"));
        let ca = native_tls::Certificate::from_der(certificate.cert.der()).unwrap_or_else(|error| panic!("{}: {error:?}", "CA"));
        let tls = Arc::new(
            ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
                .with_safe_default_protocol_versions()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "TLS versions"))
                .with_no_client_auth()
                .with_single_cert(vec![certificate.cert.der().clone()], PrivateKeyDer::from(PrivatePkcs8KeyDer::from(certificate.signing_key.serialize_der())))
                .unwrap_or_else(|error| panic!("{}: {error:?}", "TLS")),
        );
        let memory = Arc::new(Mutex::new(RedisMemory::default()));
        let redis = RedisMemory::serve(Arc::clone(&memory), Arc::clone(&tls));
        let node_paths = Arc::new(Mutex::new(Vec::new()));
        let paths = Arc::clone(&node_paths);
        let node_handler = move |request: http::IncomingRequest| {
            paths.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "paths")).push(request.path.clone());
            if !matches!(request.path.as_str(), "/" | "/chain/125") {
                return json_response(404, &json!({"error":"unsupported node path"}));
            }
            assert_eq!(request.method, "POST");
            let call: Value = serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{}: {error:?}", "node call"));
            let result = match call["method"].as_str().unwrap_or_else(|| panic!("{}", "method")) {
                "eth_chainId" => json!("0x7d"),
                "eth_blockNumber" => json!("0xb"),
                "eth_call" => {
                    if call["params"][0]["data"] == format!("0x{}", support::hex(&layerx_platform_gateway::evm::calldata_address(layerx_platform_gateway::evm::SELECTOR_GET_UNIFIED_ACCOUNT, &[0xee; 20])))
                        || call["params"][0]["data"] == format!("0x{}", support::hex(&layerx_platform_gateway::evm::calldata_word(layerx_platform_gateway::evm::SELECTOR_GET_EVM_ADDR_BY_LAYERX, &history_fixture::binding().0)))
                    {
                        return json_response(200, &history_fixture::chain(&call));
                    }
                    let views = fixture("views");
                    views["calls"]
                        .as_array()
                        .unwrap_or_else(|| panic!("{}", "views"))
                        .iter()
                        .find(|v| call["params"][0]["to"] == v["to"] && call["params"][0]["data"] == v["data"] && call["params"][1] == v["block"])
                        .unwrap_or_else(|| panic!("unrecorded view {call}"))
                        .get("result")
                        .unwrap_or_else(|| panic!("{}", "result"))
                        .clone()
                }
                other => panic!("unexpected node method {other}"),
            };
            json_response(200, &json!({"jsonrpc":"2.0","id":call["id"],"result":result}))
        };
        let node = Peer::https(Arc::clone(&tls), node_handler.clone());
        let second_node = Peer::https(Arc::clone(&tls), node_handler);
        let result = Self {
            directory,
            ca,
            tls,
            redis,
            node,
            second_node,
            node_paths,
        };
        result.configure();
        result
    }
    fn secret(&self, name: &str, contents: &str) -> std::path::PathBuf {
        let path = self.directory.path().join(name);
        fs::write(&path, contents).unwrap_or_else(|error| panic!("{}: {error:?}", "secret file"));
        path
    }
    fn configure(&self) {
        for (k, v) in [("LAYERX_GATEWAY_LISTEN", "127.0.0.1:0"), ("LAYERX_GATEWAY_LISTENER", "plain"), ("LAYERX_GATEWAY_NETWORK_ID", "chain-reads"), ("LAYERX_GATEWAY_PROTOCOL_NETWORK_ID", "7")] {
            std::env::set_var(k, v);
        }
        std::env::set_var("LAYERX_GATEWAY_TRUSTED_PROXIES", "127.0.0.0/8");
        std::env::set_var("LAYERX_GATEWAY_LXP_WIRE_VERSION", layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string());
        std::env::set_var("LAYERX_GATEWAY_OUTBOUND_CA_DER", self.directory.path().join("ca.der"));
        std::env::set_var("LAYERX_GATEWAY_REDIS_URL", format!("rediss://localhost:{}", self.redis.address.port()));
        std::env::set_var("LAYERX_GATEWAY_REDIS_USERNAME_FILE", self.secret("username", "gateway"));
        std::env::set_var("LAYERX_GATEWAY_REDIS_PASSWORD_FILE", self.secret("password", "gateway-secret"));
        std::env::set_var("LAYERX_GATEWAY_PAXEER_RPC_URLS", json!([self.node.url(), self.second_node.url()]).to_string());
    }
    fn store(&self) -> RedisStore {
        RedisStore::new(
            RedisEndpoint::parse(&format!("rediss://localhost:{}", self.redis.address.port())).unwrap_or_else(|error| panic!("{}: {error:?}", "Redis URL")),
            self.ca.clone(),
            Zeroizing::new("gateway".into()),
            Zeroizing::new("gateway-secret".into()),
        )
    }
    fn key(&self) -> (KeyRecord, String) {
        let id = "0123456789abcdef01234567";
        let secret = format!("lxp_live_{}", "42".repeat(32));
        let salt = "12".repeat(32);
        let record = KeyRecord {
            key_id: id.into(),
            principal_digest: "34".repeat(32),
            secret_digest: gateway_digest(&[b"gateway-key-v1", salt.as_bytes(), secret.as_bytes()]),
            salt,
            signer_public_key: "56".repeat(32),
            scopes: "read".into(),
            quota_requests: 1000,
            quota_window_seconds: 60,
            epoch: 1,
            disabled: false,
        };
        self.store().issue_key(&record, "repair-key-issued").unwrap_or_else(|error| panic!("{}: {error:?}", "real durable key issuance"));
        (record, format!("LayerX-Key {id}:{secret}"))
    }
}
struct Gateway(Peer);
impl Gateway {
    fn start() -> Self {
        let server = Server::from_environment().unwrap_or_else(|error| panic!("{}: {error:?}", "production gateway configuration"));
        Self(Peer::start(move |tcp| {
            server.serve_connection(tcp).unwrap_or_else(|error| panic!("{}: {error:?}", "production gateway request"));
        }))
    }
    fn at(clock: fn() -> Result<u64, String>) -> Self {
        let server = Server::from_environment().unwrap_or_else(|error| panic!("{}: {error:?}", "production gateway configuration")).with_state_clock(clock);
        Self(Peer::start(move |tcp| {
            server.serve_connection(tcp).unwrap_or_else(|error| panic!("{}: {error:?}", "production gateway request"));
        }))
    }
    fn request(&self, method: &str, path: &str, authorization: Option<&str>, body: &Value) -> (u16, Value) {
        static CLIENT: AtomicUsize = AtomicUsize::new(0);
        let number = CLIENT.fetch_add(1, Ordering::Relaxed);
        let forwarded = format!("192.0.{}.{}", (number / 250) % 250, number % 250 + 1);
        let mut headers = authorization.map(|v| vec![("Authorization", v)]).unwrap_or_default();
        headers.push(("X-Forwarded-For", &forwarded));
        request(self.0.address, method, path, &headers, body)
    }
    fn rpc(&self, method: &str, params: &Value) -> Value {
        let (status, answer) = self.request("POST", "/", None, &json!({"jsonrpc":"2.0","id":1439,"method":method,"params":params}));
        assert_eq!(status, 200, "{answer}");
        assert_eq!(answer["id"], 1439);
        answer
    }
}
fn indexer(store: Arc<Store>, registry_mode: Arc<AtomicUsize>, registry_reads: Arc<AtomicUsize>) -> Peer {
    Peer::start(move |mut tcp| {
        let request = http::read_request(&mut tcp, 16384).unwrap_or_else(|error| panic!("{}: {error:?}", "indexer HTTP request"));
        if request.path.split('?').next() == Some("/v1/assets") {
            registry_reads.fetch_add(1, Ordering::AcqRel);
            let mode = registry_mode.load(Ordering::Acquire);
            if mode == 3 {
                return;
            }
            let response = match mode {
                1 => Some(json_response(503, &json!({"error":"registry unavailable"}))),
                2 => Some(json_response(429, &json!({"error":"rate limited"}))),
                4 => Some(json_response(200, &json!({"version":1,"next_cursor":null}))),
                5 => Some(json_response(200, &json!({"version":1,"items":[],"next_cursor":"bad"}))),
                6 => Some(json_response(200, &json!({"version":1,"items":[],"next_cursor":"1"}))),
                _ => None,
            };
            if let Some(response) = response {
                http::write_response(&mut tcp, &response).unwrap_or_else(|error| panic!("{}: {error:?}", "registry failure response"));
                return;
            }
        }
        let response = api::route(&store, &request.method, &request.path);
        layerx_platform_internal::http::write_response(&mut tcp, &response).unwrap_or_else(|error| panic!("{}: {error:?}", "real indexer API response"));
    })
}
fn history_store(complete: bool) -> Arc<Store> {
    let store = Arc::new(Store::open_in_memory().unwrap_or_else(|error| panic!("{}: {error:?}", "real indexer store")));
    store.start_source("paxeer").unwrap_or_else(|error| panic!("{}: {error:?}", "source session start"));
    let assets = vec![
        AssetRow {
            asset: "evm:native".into(),
            chain: "paxeer".into(),
            kind: "native".into(),
            address: None,
            denom: None,
            metadata: json!({}),
        },
        AssetRow {
            asset: TOKEN.into(),
            chain: "paxeer".into(),
            kind: "erc20".into(),
            address: Some(TOKEN[4..].into()),
            denom: Some("utoken".into()),
            metadata: if complete { json!({"symbol":"TOKEN","decimals":6}) } else { json!({}) },
        },
    ];
    let transfers = ["evm:native", TOKEN]
        .iter()
        .enumerate()
        .map(|(i, asset)| TransferRow {
            height_or_seq: 10,
            tx_index: u32::try_from(i).unwrap_or_else(|_| panic!("transaction index range")),
            kind: if i == 0 { "native_transfer" } else { "erc20_transfer" }.into(),
            direction: "out",
            account: ACCOUNT.into(),
            counterparty: Some("0xffffffffffffffffffffffffffffffffffffffff".into()),
            asset: (*asset).into(),
            amount: (i + 1).to_string(),
            tx_id: format!("0x{:064x}", i + 1),
            ordinal: i as u64,
            decoded: json!({"producer":"indexer"}),
        })
        .collect();
    store
        .commit(
            &Unit {
                chain: "paxeer".into(),
                position: 10,
                hash: "paxeer-10".into(),
                parent: "paxeer-9".into(),
                link: "paxeer-10".into(),
                boundary: 10,
                transfers,
                assets,
                ..Unit::default()
            },
            64,
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real indexer ingestion"));
    store.record_source_head("paxeer", Some(10)).unwrap_or_else(|error| panic!("{}: {error:?}", "source head observation"));
    store.record_source_success("paxeer", Some(10), Some(10)).unwrap_or_else(|error| panic!("{}: {error:?}", "source observation"));
    store
}

#[test]
fn node_root_and_explicit_chain_paths_reject_router_paths() {
    let _lock = ENVIRONMENT.lock().unwrap_or_else(std::sync::PoisonError::into_inner);
    let _env = Environment::clear();
    let fixture = Fixture::start();
    for suffix in ["", "/", "/chain/125"] {
        std::env::set_var("LAYERX_GATEWAY_PAXEER_RPC_URLS", json!([format!("{}{suffix}", fixture.node.url()), format!("{}{suffix}", fixture.second_node.url())]).to_string());
        let gateway = Gateway::start();
        let block_number = gateway.rpc("eth_blockNumber", &json!([]));
        if block_number["result"].is_null() {
            eprintln!("node suffix {suffix:?}: eth_blockNumber response {block_number}");
        }
        assert_eq!(block_number["result"], "0xb");
        assert!(gateway.rpc("px_getCapabilities", &json!([])).get("error").is_none());
        assert_eq!(gateway.request("POST", "/rpc", None, &json!({})).0, 404);
        assert_eq!(gateway.request("GET", "/rpc/schema", None, &Value::Null).0, 404);
    }
    let paths = fixture.node_paths.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "paths")).clone();
    assert!(paths.iter().any(|p| p == "/"));
    assert!(paths.iter().any(|p| p == "/chain/125"));
    assert!(!paths.iter().any(|p| p == "/rpc"));
    std::env::set_var("LAYERX_GATEWAY_PAXEER_RPC_URLS", json!([format!("{}/rpc", fixture.node.url()), format!("{}/rpc", fixture.second_node.url())]).to_string());
    assert!(Gateway::start().rpc("eth_blockNumber", &json!([])).get("error").is_some(), "an obsolete router path must fail against the node listener");
}

#[test]
fn history_native_and_complete_metadata_skip_registry() {
    let _lock = ENVIRONMENT.lock().unwrap_or_else(std::sync::PoisonError::into_inner);
    let _env = Environment::clear();
    let fixture = Fixture::start();
    let store = history_store(true);
    let reads = Arc::new(AtomicUsize::new(0));
    let peer = indexer(Arc::clone(&store), Arc::new(AtomicUsize::new(1)), Arc::clone(&reads));
    std::env::set_var("LAYERX_GATEWAY_INDEXER_URL", format!("http://{}", peer.address));
    let gateway = Gateway::start();
    for method in ["px_getHistory", "px_getUnifiedHistory"] {
        let answer = gateway.rpc(method, &json!([ACCOUNT, null, 100]));
        assert!(answer.get("error").is_none(), "{answer}");
        let rows = answer["result"]["items"].as_array().unwrap_or_else(|| panic!("{}", "rows"));
        assert_eq!(rows.len(), 2);
        assert_eq!(rows[0]["asset_metadata"]["kind"], "native");
        assert_eq!(rows[0]["asset_metadata"]["decimals"], 18);
        assert_eq!(rows[1]["asset_metadata"]["symbol"], "TOKEN");
        assert_eq!(rows[1]["asset_metadata"]["decimals"], 6);
        for row in rows {
            assert_eq!(row["tx_hash"], row["tx_id"]);
        }
    }
    assert_eq!(reads.load(Ordering::Acquire), 0);
    drop(gateway);
    drop(peer);
    drop(fixture);
}

#[test]
fn history_registry_preserves_exact_failure_categories() {
    let _lock = ENVIRONMENT.lock().unwrap_or_else(std::sync::PoisonError::into_inner);
    let _env = Environment::clear();
    let _fixture = Fixture::start();
    let mode = Arc::new(AtomicUsize::new(0));
    let reads = Arc::new(AtomicUsize::new(0));
    let peer = indexer(history_store(false), Arc::clone(&mode), Arc::clone(&reads));
    std::env::set_var("LAYERX_GATEWAY_INDEXER_URL", format!("http://{}", peer.address));
    let gateway = Gateway::start();
    for (failure, code) in [
        (1, "registry_unavailable"),
        (2, "registry_rate_limited"),
        (3, "indexer_unreachable"),
        (4, "invalid_indexer_response"),
        (5, "invalid_indexer_response"),
        (6, "invalid_indexer_response"),
    ] {
        mode.store(failure, Ordering::Release);
        for method in ["px_getHistory", "px_getUnifiedHistory"] {
            let answer = gateway.rpc(method, &json!([ACCOUNT, null, 100]));
            assert_eq!(answer["error"], json!({"code":-32001,"message":"History unavailable","data":{"code":code}}), "{answer}");
        }
    }
    assert_eq!(reads.load(Ordering::Acquire), 12);
    mode.store(0, Ordering::Release);
    let answer = gateway.rpc("px_getHistory", &json!([ACCOUNT, null, 100]));
    assert!(answer.get("error").is_none(), "recovery {answer}");
}

#[test]
fn validator_boundary_refusal_deadline_and_recovery() {
    let _lock = ENVIRONMENT.lock().unwrap_or_else(std::sync::PoisonError::into_inner);
    let _env = Environment::clear();
    let fixture = Fixture::start();
    let mode = Arc::new(AtomicUsize::new(0));
    let calls = Arc::new(AtomicUsize::new(0));
    let m = Arc::clone(&mode);
    let c = Arc::clone(&calls);
    let consensus = Peer::https(Arc::clone(&fixture.tls), move |request| {
        assert_eq!(request.path, "/");
        let call: Value = serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{}: {error:?}", "consensus call"));
        assert_eq!(call["method"], "validators");
        let page = call["params"]["page"].as_str().unwrap_or_else(|| panic!("{}", "page")).parse::<usize>().unwrap_or_else(|error| panic!("{}: {error:?}", "page number"));
        assert_eq!(call["params"]["per_page"], "100");
        c.fetch_add(1, Ordering::AcqRel);
        let mode = m.load(Ordering::Acquire);
        if mode == 4 {
            std::thread::sleep(Duration::from_millis(75));
        }
        let total = if mode == 1 { 10001 } else { 10000 };
        let validators: Vec<_> = if mode == 3 {
            vec![]
        } else {
            (0..100)
                .map(|i| json!({"address":format!("{:040X}",if mode==2 {i}else{(page-1)*100+i}),"pub_key":{"type":"tendermint/PubKeyEd25519","value":layerx_platform_internal::base64::encode(&[u8::try_from(i % 250 + 1).unwrap_or_else(|_| panic!("signer byte range"));32])},"voting_power":"1"}))
                .collect()
        };
        json_response(200, &json!({"jsonrpc":"2.0","id":call["id"],"result":{"block_height":"10","count":validators.len().to_string(),"total":total.to_string(),"validators":validators}}))
    });
    std::env::set_var("LAYERX_GATEWAY_CONSENSUS_RPC_URL", consensus.url());
    let gateway = Gateway::start();
    let answer = gateway.rpc("px_getValidators", &json!(["10"]));
    assert_eq!(answer["result"]["total"], "10000");
    assert_eq!(answer["result"]["validators"].as_array().unwrap_or_else(|| panic!("{}", "validators")).len(), 10000);
    assert_eq!(calls.load(Ordering::Acquire), 100);
    for (refused, requests) in [(1, 1), (2, 2), (3, 1)] {
        mode.store(refused, Ordering::Release);
        let before = calls.load(Ordering::Acquire);
        let answer = gateway.rpc("px_getValidators", &json!(["10"]));
        assert_eq!(answer["error"]["data"]["code"], "consensus_unavailable");
        assert_eq!(answer["error"]["code"], -32003);
        assert_eq!(calls.load(Ordering::Acquire) - before, requests);
    }
    mode.store(4, Ordering::Release);
    let started = Instant::now();
    let answer = gateway.rpc("px_getValidators", &json!(["10"]));
    assert_eq!(answer["error"]["data"]["code"], "consensus_unavailable");
    assert!(started.elapsed() < Duration::from_millis(5400));
    mode.store(0, Ordering::Release);
    assert_eq!(gateway.rpc("px_getValidators", &json!(["10"]))["result"]["total"], "10000");
}

fn request(address: SocketAddr, method: &str, route: &str, headers: &[(&str, &str)], body: &Value) -> (u16, Value) {
    let bytes = if method == "GET" { Vec::new() } else { serde_json::to_vec(body).unwrap_or_else(|error| panic!("{}: {error:?}", "request")) };
    let mut stream = TcpStream::connect(address).unwrap_or_else(|error| panic!("{}: {error:?}", "gateway TCP"));
    stream.set_read_timeout(Some(Duration::from_secs(10))).unwrap_or_else(|error| panic!("{}: {error:?}", "response timeout"));
    write!(stream, "{method} {route} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n", bytes.len()).unwrap_or_else(|error| panic!("{}: {error:?}", "headers"));
    for (key, value) in headers {
        write!(stream, "{key}: {value}\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "header"));
    }
    stream.write_all(b"\r\n").unwrap_or_else(|error| panic!("{}: {error:?}", "end headers"));
    stream.write_all(&bytes).unwrap_or_else(|error| panic!("{}: {error:?}", "body"));
    let mut answer = Vec::new();
    stream.read_to_end(&mut answer).unwrap_or_else(|error| panic!("{}: {error:?}", "gateway response"));
    let split = answer.windows(4).position(|s| s == b"\r\n\r\n").unwrap_or_else(|| panic!("{}", "HTTP response"));
    let status = std::str::from_utf8(&answer[..split])
        .unwrap_or_else(|error| panic!("{}: {error:?}", "headers"))
        .split_whitespace()
        .nth(1)
        .unwrap_or_else(|| panic!("{}", "status"))
        .parse()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "status number"));
    (status, serde_json::from_slice(&answer[split + 4..]).unwrap_or_else(|error| panic!("{}: {error:?}", "gateway response JSON")))
}

#[allow(dead_code, unused_imports)]
mod ai_producer {
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_indexer::ai_market::ViewError;
    use layerx_indexer::codec::hex;
    use layerx_platform_gateway::committed_proof::{
        commit::{BlockIdFlag, Commit, CommitSig},
        header::{validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator},
        proto::{encode_length_delimited, encode_varint, encode_varint_field},
        ProofError,
    };
    use layerx_programs_ai_market::{
        codec::{self, decode_envelope, derive_market, derive_worker, encode_envelope, Envelope},
        dispatch,
        errors::ApplicationError,
        policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
        queries::{read_state_chunk, CaptureFacts, QueryError, ReadProof, StateCapture},
        registry::{derive_rewards_account, F01_SECTION_CAP},
        registry_ops::{self, Outcome},
        state::{self, Section, SharedState},
        types::{AssetId, Authentication, ChainDomain, Digest32, MarketId, MetadataDigest, Presence, PrincipalId, ProgramId, PublicKey32, RequestId, RubricDigest, StateDigest},
        workers::{WorkerCurrent, WorkerState, WorkerTable, WORKER_TABLE_MAX_BYTES},
        MAX_EVENT_BYTES, MAX_STATE_BYTES,
    };
    use serde_json::{json, Value};
    use sha2::{Digest as _, Sha256};
    use std::fs;

    const CHAIN: [u8; 32] = [10; 32];
    const PROGRAM: [u8; 32] = [11; 32];
    const OWNER: [u8; 32] = [12; 32];
    const ASSET: [u8; 32] = [13; 32];
    const REFUND: [u8; 32] = [14; 32];
    const CHUNK_RESPONSE_MAX: usize = 8_244;
    const CHAIN_ID: &str = "ai-market-finality";
    const PUBLISHED_AT_MS: u64 = 2_000;
    const NOW_MS: u64 = 5_000_000;
    const SECRET: [u8; 32] = [0x5A; 32];

    #[derive(Debug)]
    struct Failure(String);
    type Checked<T = ()> = Result<T, Failure>;
    macro_rules! convert { ($($error:ty),+ $(,)?) => {$ (
    impl From<$error> for Failure { fn from(error:$error)->Self {Self(format!("{error:?}"))} }
)+}; }
    convert!(ApplicationError, QueryError, ProofError, ViewError, layerx_wire::WireError, std::io::Error, &'static str);
    fn chain() -> Checked<ChainDomain> {
        Ok(ChainDomain::new(CHAIN)?)
    }
    fn program() -> Checked<ProgramId> {
        Ok(ProgramId::new(PROGRAM)?)
    }
    fn market() -> Checked<MarketId> {
        Ok(derive_market(chain()?, program()?)?)
    }
    fn market_hex() -> Checked<String> {
        Ok(hex(market()?.as_bytes()))
    }
    fn principal(bytes: [u8; 32]) -> Checked<PrincipalId> {
        Ok(PrincipalId::new(bytes)?)
    }
    fn digest(byte: u8) -> Checked<Digest32> {
        Ok(Digest32::new([byte; 32])?)
    }

    fn policy() -> Checked<TaskPolicyV1> {
        Ok(TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: digest(1)?,
                dataset_artifact: [2; 32],
                benchmark_suite: digest(3)?,
                rubric: RubricDigest::new([4; 32])?,
                task_schema: digest(5)?,
                result_schema: digest(6)?,
                service_terms: digest(7)?,
            },
            100,
            1,
        )?)
    }

    fn encode_shared(shared: &SharedState<'_>) -> Checked<Vec<u8>> {
        let mut out = vec![0; shared.encoded_len()?];
        let mut scratch = vec![0; Section::Control.payload_cap()];
        let n = state::encode_shared_state(shared, &mut out, &mut scratch)?;
        out.truncate(n);
        Ok(out)
    }

    /// Real F01 CREATE through `registry_ops::apply`.
    fn created_state() -> Checked<Vec<u8>> {
        let mut encoded_policy = vec![0; TASK_POLICY_BYTES];
        policy()?.encode(&mut encoded_policy)?;
        let rewards = derive_rewards_account(program()?, AssetId::new(ASSET)?)?;
        let mut payload = OWNER.to_vec();
        payload.extend_from_slice(&ASSET);
        payload.extend_from_slice(rewards.as_bytes());
        payload.extend_from_slice(&REFUND);
        payload.push(0);
        payload.extend_from_slice(&encoded_policy);
        payload.extend_from_slice(&[16; 32]);
        let envelope = Envelope {
            operation: dispatch::CREATE,
            chain: chain()?,
            program: program()?,
            market: market()?,
            actor: principal(OWNER)?,
            epoch: 0,
            config: 1,
            roster: Presence::Absent,
            sequence: 1,
            expiry: 1_000_000,
            request: RequestId::new([1; 32])?,
            payload: &payload,
            authentication: Authentication::Native,
        };
        let mut encoded = vec![0; 16_384];
        let n = encode_envelope(&envelope, &mut encoded)?;
        let validated = decode_envelope(encoded.get(..n).ok_or("envelope")?)?;
        let ctx = registry_ops::CallContext {
            chain: chain()?,
            program: program()?,
            principal: principal(OWNER)?,
            height: 1000,
        };
        let mut section = vec![0; F01_SECTION_CAP];
        let mut event = vec![0; MAX_EVENT_BYTES];
        match registry_ops::apply(&ctx, None, &validated, &mut section, &mut event)? {
            Outcome::Applied { state, .. } => encode_shared(&state),
            Outcome::AlreadyApplied(_) => Err(Failure("fresh create retried".to_owned())),
        }
    }

    fn worker(slot: u8, nonce: u8, state: WorkerState) -> Checked<WorkerCurrent> {
        Ok(WorkerCurrent {
            worker: derive_worker(market()?, principal([20; 32])?, [nonce; 32])?,
            owner: principal([20; 32])?,
            delegate: PublicKey32([slot + 50; 32]),
            metadata: MetadataDigest::new([30; 32])?,
            generation: 2,
            key_version: 2,
            metadata_revision: 4,
            valid_from: 200,
            expiry: 400,
            revocation_sequence: 0,
            effective_epoch: 1,
            last_sequence: 0,
            last_request_id: [0; 32],
            last_request_digest: [0; 32],
            last_result_digest: [0; 32],
            state,
            slot,
            last_metadata_height: 200,
        })
    }

    /// Created market plus an F02 worker table; even slots available, odd slots retired. Seat
    /// `slot` is held by the worker derived from nonce `slot + nonce_base`.
    fn market_with_workers(count: u8, nonce_base: u8) -> Checked<Vec<u8>> {
        let base = created_state()?;
        let shared = state::decode_shared_state(&base)?;
        let mut table = WorkerTable::new();
        for slot in 0..count {
            let status = if slot % 2 == 0 { WorkerState::Available } else { WorkerState::Retired };
            table.insert(&worker(slot, slot + nonce_base, status)?)?;
        }
        let mut section = vec![0; WORKER_TABLE_MAX_BYTES];
        let n = table.encode(&mut section)?;
        let identity = section.get(..n).ok_or("worker table")?;
        encode_shared(&shared.replace_section(Section::IdentityRoster, identity)?)
    }

    fn chunk_payload(revision: u64, pinned: Option<StateDigest>, offset: u32, requested: u16) -> Vec<u8> {
        let mut payload = revision.to_be_bytes().to_vec();
        payload.extend_from_slice(&pinned.map_or([0; 32], StateDigest::bytes));
        payload.extend_from_slice(&offset.to_be_bytes());
        payload.extend_from_slice(&requested.to_be_bytes());
        payload
    }

    fn read_all(state_bytes: &[u8]) -> Checked<Vec<Vec<u8>>> {
        let mut out = vec![0; CHUNK_RESPONSE_MAX];
        let n = read_state_chunk(state_bytes, &chunk_payload(0, None, 0, 8192), &mut out)?;
        let first = codec::decode_chunk_response(&out[..n])?;
        let (revision, pinned, total) = (first.revision, first.digest, first.total_bytes);
        let mut bodies = vec![out[..n].to_vec()];
        for offset in (8192..total).step_by(8192) {
            let payload = chunk_payload(revision, Some(pinned), offset, 8192);
            let n = read_state_chunk(state_bytes, &payload, &mut out)?;
            bodies.push(out[..n].to_vec());
        }
        Ok(bodies)
    }

    fn validators() -> Vec<Validator> {
        (1..=4)
            .map(|value| Validator {
                pub_key: SigningKey::from_bytes(&[value; 32]).verifying_key().to_bytes(),
                voting_power: 10,
            })
            .collect()
    }

    fn signed(header: &Header, signers: &[(u8, bool)]) -> Commit {
        let mut commit = Commit {
            height: u64::try_from(header.height).unwrap_or_else(|_| panic!("nonnegative height")),
            round: 0,
            block_id: BlockId {
                hash: header.hash().to_vec(),
                part_set_header: PartSetHeader { total: 1, hash: vec![0xA5; 32] },
            },
            signatures: Vec::new(),
        };
        for value in 1..=4 {
            let Some((_, tamper)) = signers.iter().find(|(signer, _)| *signer == value) else {
                commit.signatures.push(CommitSig::absent());
                continue;
            };
            let key = SigningKey::from_bytes(&[value; 32]);
            let timestamp = Timestamp { seconds: 1_000, nanos: 123 };
            let mut vote = Vec::new();
            encode_varint_field(1, 2, &mut vote);
            vote.push(0x11);
            vote.extend_from_slice(&commit.height.to_le_bytes());
            let mut parts = Vec::new();
            encode_varint_field(1, u64::from(commit.block_id.part_set_header.total), &mut parts);
            encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
            let mut block = Vec::new();
            encode_length_delimited(1, &commit.block_id.hash, &mut block);
            encode_length_delimited(2, &parts, &mut block);
            encode_length_delimited(4, &block, &mut vote);
            let mut time = Vec::new();
            encode_varint_field(1, u64::try_from(timestamp.seconds).unwrap_or_else(|_| panic!("nonnegative timestamp")), &mut time);
            encode_varint_field(2, u64::from(timestamp.nanos), &mut time);
            encode_length_delimited(5, &time, &mut vote);
            encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
            let mut sign_bytes = Vec::new();
            encode_varint(vote.len() as u64, &mut sign_bytes);
            sign_bytes.extend_from_slice(&vote);
            let mut signature = key.sign(&sign_bytes).to_bytes();
            if *tamper {
                signature[40] ^= 1;
            }
            commit.signatures.push(CommitSig {
                block_id_flag: BlockIdFlag::Commit,
                validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
                timestamp,
                signature: Some(signature),
            });
        }
        commit
    }

    fn header(height: u64, root: Digest32) -> Header {
        Header {
            chain_id: CHAIN_ID.to_owned(),
            height: i64::try_from(height).unwrap_or_else(|_| panic!("height range")),
            app_hash: root.bytes().to_vec(),
            version: ConsensusVersion { block: 11, app: 1 },
            time: Timestamp { seconds: 1_000, nanos: 0 },
            last_block_id: BlockId::default(),
            last_commit_hash: vec![1; 32],
            data_hash: vec![2; 32],
            validators_hash: validators_hash(&validators()),
            next_validators_hash: validators_hash(&validators()).to_vec(),
            consensus_hash: vec![3; 32],
            last_results_hash: vec![4; 32],
            evidence_hash: vec![5; 32],
            proposer_address: vec![6; 20],
        }
    }

    struct Material {
        header: Header,
        commit: Commit,
        proof: Vec<u8>,
    }
    // Canonical native fixture: the namespace head/manifest layout emitted by lxphost,
    // with ICS23 leaf operations encoded by the production protobuf codec.
    fn native_membership(state: &[u8], program: [u8; 32], cell: &[u8]) -> (Vec<u8>, [u8; 32]) {
        fn leaf(key: &[u8], value: &[u8], prefix: &[u8]) -> (Vec<u8>, [u8; 32]) {
            let mut operation = Vec::new();
            for (field, value) in [(1, 1), (2, 0), (3, 1), (4, 1)] {
                encode_varint_field(field, value, &mut operation);
            }
            encode_length_delimited(5, prefix, &mut operation);
            let mut proof = Vec::new();
            encode_length_delimited(1, key, &mut proof);
            encode_length_delimited(2, value, &mut proof);
            encode_length_delimited(3, &operation, &mut proof);
            let mut preimage = prefix.to_vec();
            encode_varint(key.len() as u64, &mut preimage);
            preimage.extend(key);
            encode_varint(32, &mut preimage);
            preimage.extend(Sha256::digest(value));
            (proof, Sha256::digest(preimage).into())
        }
        let mut manifest = vec![0, 1];
        manifest.extend(1u32.to_be_bytes());
        manifest.extend(u16::try_from(cell.len()).unwrap_or_else(|_| panic!("cell length range")).to_be_bytes());
        manifest.extend(cell);
        manifest.extend(Sha256::digest(state));
        manifest.extend(u32::try_from(state.len()).unwrap_or_else(|_| panic!("state length range")).to_be_bytes());
        let mut head = vec![0, 1];
        head.extend(1u32.to_be_bytes());
        head.extend(Sha256::digest(&manifest));
        let mut key = b"\x01\x00\x09progstor".to_vec();
        key.extend(program);
        key.push(1);
        let (proof, store_root) = leaf(&key, &head, &[0, 2, 2]);
        let (outer, app_hash) = leaf(b"lxphost", &store_root, &[0]);
        let mut multistore = Vec::new();
        encode_length_delimited(1, &outer, &mut multistore);
        (
            serde_json::to_vec(&json!({"membership": {"proof": proof, "multistore": multistore}, "manifest": manifest})).unwrap_or_else(|error| panic!("{}: {error:?}", "fixture JSON")),
            app_hash,
        )
    }

    fn certify_at(state: &[u8], height: u64) -> Checked<Material> {
        let program = PROGRAM;
        let cell = layerx_programs_ai_market::SHARED_STATE_KEY;
        let (proof, root) = native_membership(state, program, cell);
        let header = header(height, Digest32::new(root)?);
        let commit = signed(&header, &[(1, false), (2, false), (3, false), (4, false)]);
        Ok(Material { header, commit, proof })
    }

    fn capture_state(state: &[u8], material: &Material) -> Checked<(ReadProof, Vec<Vec<u8>>, layerx_programs_ai_market::queries::CaptureFacts)> {
        let proof = ReadProof {
            chain: chain()?,
            program: program()?,
            native_state_root: Digest32::new(material.header.app_hash.as_slice().try_into().unwrap_or_else(|error| panic!("{}: {error:?}", "root")))?,
            observed_sequence: u64::try_from(material.header.height).unwrap_or_else(|_| panic!("nonnegative height")),
            execution_height: u64::try_from(material.header.height).unwrap_or_else(|_| panic!("nonnegative height")),
            block_hash: Digest32::new(material.header.hash())?,
        };
        let chunks = read_all(state)?;
        let mut bytes = vec![0; MAX_STATE_BYTES];
        let mut capture = StateCapture::new(&mut bytes);
        for chunk in &chunks {
            capture.accept(&proof, chunk)?;
        }
        let (assembled, facts) = capture.finish()?;
        assert_eq!(assembled, state);
        Ok((proof, chunks, facts))
    }

    pub(super) fn consensus(height: u64) -> (Value, Value, Value) {
        let state = market_with_workers(4, 0).unwrap_or_else(|error| panic!("{}: {error:?}", "real market state"));
        let material = certify_at(&state, height).unwrap_or_else(|error| panic!("{}: {error:?}", "signed consensus material"));
        let h = &material.header;
        let c = &material.commit;
        let block = |b: &BlockId| json!({"hash":hex(&b.hash),"parts":{"total":b.part_set_header.total,"hash":hex(&b.part_set_header.hash)}});
        let header = json!({"version":{"block":h.version.block.to_string(),"app":h.version.app.to_string()},
            "chain_id":h.chain_id,"height":h.height.to_string(),"time":"1970-01-01T00:16:40Z",
            "last_block_id":block(&h.last_block_id),"last_commit_hash":hex(&h.last_commit_hash),"data_hash":hex(&h.data_hash),
            "validators_hash":hex(&h.validators_hash),"next_validators_hash":hex(&h.next_validators_hash),
            "consensus_hash":hex(&h.consensus_hash),"app_hash":hex(&h.app_hash),"last_results_hash":hex(&h.last_results_hash),
            "evidence_hash":hex(&h.evidence_hash),"proposer_address":hex(&h.proposer_address)});
        let commit = json!({"height":c.height.to_string(),"round":c.round,"block_id":block(&c.block_id),
            "signatures":c.signatures.iter().map(|v|json!({"block_id_flag":2,"validator_address":hex(&v.validator_address),
                "timestamp":"1970-01-01T00:16:40.000000123Z","signature":v.signature.as_ref().map(|v|layerx_platform_internal::base64::encode(v))})).collect::<Vec<_>>()});
        let status = json!({"sync_info":{"latest_block_height":h.height.to_string(),"latest_block_hash":hex(&h.hash()),"latest_app_hash":hex(&h.app_hash),"catching_up":false}});
        let authority = json!({"chain_id":CHAIN_ID,"validators":validators().iter().map(|v|json!({"public_key":hex(&v.pub_key),"voting_power":v.voting_power})).collect::<Vec<_>>()});
        (authority, status, json!({"signed_header":{"header":header,"commit":commit}}))
    }

    pub(super) fn ingest(database: &std::path::Path, height: u64, count: u8) -> (String, Value) {
        let state = market_with_workers(count, 0).unwrap_or_else(|error| panic!("{}: {error:?}", "real market producer"));
        let material = certify_at(&state, height).unwrap_or_else(|error| panic!("{}: {error:?}", "native membership and signed commit"));
        let (_, chunks, _) = capture_state(&state, &material).unwrap_or_else(|error| panic!("{}: {error:?}", "real whole-root chunk codec"));
        let config = serde_json::json!({"chain":CHAIN,"program":PROGRAM,"consensus_chain_id":CHAIN_ID,
            "validators": validators().iter().map(|v|serde_json::json!({"pub_key":v.pub_key,"voting_power":v.voting_power})).collect::<Vec<_>>(),
            "source":"http://127.0.0.1:1","ca_der":null,"allow_remote_plaintext":false});
        let path = database.with_extension("config.json");
        fs::write(&path, serde_json::to_vec(&config).unwrap_or_else(|error| panic!("{}: {error:?}", "runtime config"))).unwrap_or_else(|error| panic!("{}: {error:?}", "runtime config file"));
        let (mut runtime, _) = layerx_indexer::ai_runtime::RuntimeConfig::read(&path)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "runtime config"))
            .open(database)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "real projection runtime"));
        let h = &material.header;
        let c = &material.commit;
        let block = |b: &BlockId| json!({"hash":b.hash,"total":b.part_set_header.total,"parts_hash":b.part_set_header.hash});
        // The transport carries the real read_state_chunk output and signed
        // native membership evidence; AiRuntime authenticates the entire capture.
        let payload = serde_json::to_vec(&json!({"version":1,"chain":CHAIN,"program":PROGRAM,"observed_sequence":height,
            "header":{"version":[h.version.block,h.version.app],"chain_id":h.chain_id,"height":h.height,"time":[h.time.seconds,h.time.nanos],
            "last_block_id":block(&h.last_block_id),"last_commit_hash":h.last_commit_hash,"data_hash":h.data_hash,"validators_hash":h.validators_hash,
            "next_validators_hash":h.next_validators_hash,"consensus_hash":h.consensus_hash,"app_hash":h.app_hash,"last_results_hash":h.last_results_hash,
            "evidence_hash":h.evidence_hash,"proposer_address":h.proposer_address},
            "commit":{"height":c.height,"round":c.round,"block_id":block(&c.block_id),"signatures":c.signatures.iter().map(|v|json!({
                "flag":2,"validator_address":v.validator_address,"time":[v.timestamp.seconds,v.timestamp.nanos],"signature":v.signature.map(Vec::from)})).collect::<Vec<_>>()},
            "store_proof":material.proof,"chunks":chunks}))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "committed producer transport"));
        let record = runtime.ingest(&payload, 1_000_000).unwrap_or_else(|error| panic!("{}: {error:?}", "committed market ingestion"));
        assert!(record.projection.finalized());
        let market = market_hex().unwrap_or_else(|error| panic!("{}: {error:?}", "market identifier"));
        (market, serde_json::json!({"height":height,"id":hex(record.snapshot_id.as_bytes())}))
    }
}

#[allow(dead_code)]
mod history_fixture {
    use super::*;
    use layerx_indexer::store::{HolderRow, Position};
    use layerx_platform_gateway::evm;
    use sha2::{Digest, Sha256};
    const LXP: &str = "1111111111111111111111111111111111111111111111111111111111111111";
    const ALICE: &str = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa";
    const BOB: &str = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb";
    const EVE: &str = "0xeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee";
    /// Eve's address in bech32 under the `pax` prefix.
    const PAX_EVE: &str = "pax1amhwamhwamhwamhwamhwamhwamhwamhw3rt4xd";
    const FRANK: &str = "0xffffffffffffffffffffffffffffffffffffffff";
    const POINTER: &str = "0xcccccccccccccccccccccccccccccccccccccccc";
    const TICKET: &str = "denom:factory/pax1issuer/ticket";
    const BUDGET: &str = "0101010101010101010101010101010101010101010101010101010101010101";

    fn transaction_hash(kind: &str, position: u64, tx_index: u32) -> String {
        format!("0x{}", support::hex(&Sha256::digest(format!("{kind}-{position}-{tx_index}"))))
    }

    /// One transfer leg of transaction `tx_index` at `position`: a native
    /// value row when `ordinal` is 0, else the log at index `ordinal - 1`.
    fn leg((position, tx_index, ordinal): (u64, u32, u64), kind: &str, direction: &'static str, account: &str, counterparty: &str, asset: &str, amount: &str) -> TransferRow {
        TransferRow {
            height_or_seq: position,
            tx_index,
            kind: kind.to_owned(),
            direction,
            account: account.to_owned(),
            counterparty: Some(counterparty.to_owned()),
            asset: asset.to_owned(),
            amount: amount.to_owned(),
            tx_id: transaction_hash(kind, position, tx_index),
            ordinal,
            decoded: json!({"position": position.to_string()}),
        }
    }

    fn asset(asset: &str, chain: &str, kind: &str, address: Option<&str>, denom: Option<&str>) -> AssetRow {
        AssetRow {
            asset: asset.to_owned(),
            chain: chain.to_owned(),
            kind: kind.to_owned(),
            address: address.map(str::to_owned),
            denom: denom.map(str::to_owned),
            metadata: json!({}),
        }
    }

    fn unit(chain: &str, position: u64, transfers: Vec<TransferRow>, assets: Vec<AssetRow>) -> Unit {
        Unit {
            chain: chain.to_owned(),
            position,
            hash: format!("{chain}-{position}"),
            parent: format!("{chain}-{}", position - 1),
            link: format!("{chain}-{position}"),
            boundary: position,
            transfers,
            assets,
            ..Unit::default()
        }
    }

    fn at(height: u64, tx_index: u32, log_index: u32) -> Position {
        Position { height, tx_index, log_index }
    }

    /// A real indexer store and read API on a loopback port, filled with both
    /// chains interleaved in commit order. Row ids: 1-2 `LayerX` transfer
    /// alice->bob at (1,0,0), 3-4 Paxeer native eve->frank at (10,0,0), 5
    /// `LayerX` credit to alice at (2,0,0), 6-7 Paxeer pointer frank->eve at
    /// (11,0,0), 8 Paxeer custody deposit into alice at (11,1,0), 9-10 Paxeer
    /// bank ticket eve->frank at (12,0,0), 11 an unregistered asset to eve at
    /// (12,1,0). Eve holds one budget opened at (12,2,1).
    pub(super) fn indexer() -> Arc<Store> {
        let store = Store::open_in_memory().unwrap_or_else(|error| panic!("{error}"));
        store
            .register_assets(&[AssetRow {
                asset: format!("evm:{POINTER}"),
                ..asset("", "paxeer", "pointer", Some(POINTER), Some("ulxp"))
            }])
            .unwrap_or_else(|error| panic!("{error}"));
        let units = [
            unit(
                "layerx",
                1,
                vec![leg((1, 0, 0), "lxp_transfer", "out", ALICE, BOB, LXP, "5"), leg((1, 0, 0), "lxp_transfer", "in", BOB, ALICE, LXP, "5")],
                vec![AssetRow {
                    metadata: json!({"supply": "100"}),
                    ..asset(LXP, "layerx", "layerx", None, None)
                }],
            ),
            unit(
                "paxeer",
                10,
                vec![
                    leg((10, 0, 0), "native_transfer", "out", EVE, FRANK, "evm:native", "1000000000000000000"),
                    leg((10, 0, 0), "native_transfer", "in", FRANK, EVE, "evm:native", "1000000000000000000"),
                ],
                vec![asset("evm:native", "paxeer", "native", None, None)],
            ),
            unit(
                "layerx",
                2,
                vec![TransferRow {
                    counterparty: None,
                    ..leg((2, 0, 0), "lxp_credit", "in", ALICE, "", LXP, "7")
                }],
                Vec::new(),
            ),
            unit(
                "paxeer",
                11,
                vec![
                    leg((11, 0, 1), "pointer_transfer", "in", EVE, FRANK, &format!("evm:{POINTER}"), "3"),
                    leg((11, 0, 1), "pointer_transfer", "out", FRANK, EVE, &format!("evm:{POINTER}"), "3"),
                    leg((11, 1, 0), "custody_deposit", "in", ALICE, EVE, LXP, "9"),
                ],
                Vec::new(),
            ),
            Unit {
                holders: vec![HolderRow {
                    position: at(12, 2, 1),
                    holder: EVE.to_owned(),
                    kind: "budget",
                    contract: "layerxpay",
                    record_id: BUDGET.to_owned(),
                    event: "BudgetCreated",
                    state: "active",
                    tx_id: transaction_hash("budget", 12, 2),
                    decoded: json!({"args": {"amount": "500"}}),
                }],
                ..unit(
                    "paxeer",
                    12,
                    vec![
                        leg((12, 0, 0), "bank_transfer", "out", EVE, FRANK, TICKET, "2"),
                        leg((12, 0, 0), "bank_transfer", "in", FRANK, EVE, TICKET, "2"),
                        leg((12, 1, 1), "erc20_transfer", "in", EVE, FRANK, "evm:0x0000000000000000000000000000000000000abc", "4"),
                    ],
                    vec![asset(TICKET, "paxeer", "tokenfactory", None, Some("factory/pax1issuer/ticket"))],
                )
            },
        ];
        for unit in &units {
            store.commit(unit, 64).unwrap_or_else(|error| panic!("{error}"));
        }
        Arc::new(store)
    }

    pub(super) fn binding() -> ([u8; 32], [u8; 32]) {
        let did = ed25519_dalek::SigningKey::from_bytes(&[0x45; 32]).verifying_key().to_bytes();
        let name = format!("agent:did:layerx:{}:main", support::hex(&did));
        let account = layerx_types::account::AccountId::parse(&name).unwrap_or_else(|error| panic!("{error:?}"));
        let main = layerx_wire::hash::account_id_for_protocol(&account, 3).unwrap_or_else(|error| panic!("{error:?}"));
        (did, main)
    }

    /// The Paxeer chain the gateway resolves Eve on: `addr` `0x1004` answers
    /// `getUnifiedAccount(eve)` and `getEvmAddrByLayerX(did)` with Eve's
    /// [`binding`] and `pax1` address in the precompile's ABI layout, at head
    /// height 12.
    pub(super) fn chain(request: &Value) -> Value {
        let eve = [0xee_u8; 20];
        let (did, main) = binding();
        let mut padded = [0_u8; 32];
        padded[12..].copy_from_slice(&eve);
        let pax = PAX_EVE.as_bytes();
        let mut length = [0_u8; 32];
        length[31] = u8::try_from(pax.len()).unwrap_or_else(|error| panic!("{error}"));
        let mut text = pax.to_vec();
        text.resize(64, 0);
        let mut offset = [0_u8; 32];
        offset[31] = 0x80;
        let unified = [&padded[..], &offset, &did, &main, &length, &text].concat();
        let calldata = |data: Vec<u8>| format!("0x{}", support::hex(&data));
        let result = match request["method"].as_str() {
            Some("eth_blockNumber") => json!("0xc"),
            Some("eth_call") => {
                assert_eq!(request["params"][0]["to"], json!(evm::address_hex(&evm::ADDR_PRECOMPILE)));
                let data = request["params"][0]["data"].as_str().unwrap_or_default();
                let answer = if data == calldata(evm::calldata_address(evm::SELECTOR_GET_UNIFIED_ACCOUNT, &eve)) {
                    unified
                } else if data == calldata(evm::calldata_word(evm::SELECTOR_GET_EVM_ADDR_BY_LAYERX, &did)) {
                    padded.to_vec()
                } else {
                    panic!("unexpected addr call {request}")
                };
                json!(calldata(answer))
            }
            other => panic!("unexpected Paxeer method {other:?}"),
        };
        json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
    }
}

#[test]
fn history_preserves_transaction_hash_and_exact_recording() {
    let _lock = ENVIRONMENT.lock().unwrap_or_else(std::sync::PoisonError::into_inner);
    let _env = Environment::clear();
    let fixture = Fixture::start();
    let store = history_fixture::indexer();
    let registry = Store::open_in_memory().unwrap_or_else(|error| panic!("{}: {error:?}", "registry store"));
    registry
        .register_assets(&[AssetRow {
            asset: "11".repeat(32),
            chain: "layerx".into(),
            kind: "layerx".into(),
            address: None,
            denom: Some("ulxp".into()),
            metadata: json!({"symbol":"LXP","decimals":6}),
        }])
        .unwrap_or_else(|error| panic!("{}: {error:?}", "registry producer"));
    let api_peer = Peer::start(move |mut tcp| {
        let request = http::read_request(&mut tcp, 16384).unwrap_or_else(|error| panic!("{}: {error:?}", "indexer request"));
        let source = if request.path.split('?').next() == Some("/v1/assets") { &registry } else { store.as_ref() };
        let response = api::route(source, &request.method, &request.path);
        layerx_platform_internal::http::write_response(&mut tcp, &response).unwrap_or_else(|error| panic!("{}: {error:?}", "indexer producer response"));
    });
    let node_handler = |request: http::IncomingRequest| {
        assert_eq!(request.path, "/");
        let call: Value = serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{}: {error:?}", "node request"));
        if call["method"] == "eth_chainId" {
            json_response(200, &json!({"jsonrpc":"2.0","id":call["id"],"result":"0x7d"}))
        } else {
            json_response(200, &history_fixture::chain(&call))
        }
    };
    let node = Peer::https(Arc::clone(&fixture.tls), node_handler);
    let second_node = Peer::https(Arc::clone(&fixture.tls), node_handler);
    std::env::set_var("LAYERX_GATEWAY_INDEXER_URL", format!("http://{}", api_peer.address));
    std::env::set_var("LAYERX_GATEWAY_PAXEER_RPC_URLS", json!([node.url(), second_node.url()]).to_string());
    let gateway = Gateway::start();
    let (status, answer) = gateway.request("POST", "/", None, &json!({"jsonrpc":"2.0","id":1,"method":"px_getUnifiedHistory","params":[ACCOUNT,null,25,null]}));
    assert_eq!(status, 200);
    let recorded: Value = serde_json::from_str(include_str!("../../../../../apps/web/e2e/fixtures/px-get-unified-history.json")).unwrap_or_else(|error| panic!("{}: {error:?}", "unchanged producer recording"));
    assert_eq!(answer, recorded);
    for row in answer["result"]["items"].as_array().unwrap_or_else(|| panic!("{}", "recorded rows")) {
        assert_eq!(row["tx_hash"], row["tx_id"]);
        assert_eq!(row["tx_hash"].as_str().unwrap_or_else(|| panic!("{}", "transaction hash")).len(), 66);
    }
}

fn ai_clock() -> u64 {
    1000
}

#[test]
fn ai_views_ingest_restart_cursor_freshness_and_revocation() {
    let _lock = ENVIRONMENT.lock().unwrap_or_else(std::sync::PoisonError::into_inner);
    let _env = Environment::clear();
    let fixture = Fixture::start();
    let database = fixture.directory.path().join("ai.sqlite");
    let (market, first) = ai_producer::ingest(&database, 1000, 4);
    let (authority, status, commit) = ai_producer::consensus(1000);
    let observation = Arc::new(Mutex::new((status, commit)));
    let observed = Arc::clone(&observation);
    let consensus = Peer::https(Arc::clone(&fixture.tls), move |request| {
        let call: Value = serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{}: {error:?}", "consensus request"));
        let observed = observed.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "consensus state"));
        let result = match call["method"].as_str().unwrap_or_else(|| panic!("{}", "consensus method")) {
            "status" => observed.0.clone(),
            "commit" => observed.1.clone(),
            m => panic!("unexpected consensus call {m}"),
        };
        json_response(200, &json!({"jsonrpc":"2.0","id":call["id"],"result":result}))
    });
    std::env::set_var("LAYERX_GATEWAY_CONSENSUS_RPC_URL", consensus.url());
    std::env::set_var("LAYERX_GATEWAY_STATE_VALIDATORS_FILE", fixture.secret("ai-authority", &authority.to_string()));
    std::env::set_var("LAYERX_GATEWAY_AI_PROJECTION_DB", &database);
    std::env::set_var("LAYERX_GATEWAY_AI_CURSOR_GENERATION", "1");
    std::env::set_var("LAYERX_GATEWAY_AI_CURSOR_KEY_FILE", fixture.secret("cursor-key", &"5a".repeat(32)));
    let (key, credential) = fixture.key();
    let gateway = Gateway::at(|| Ok(ai_clock()));
    let snapshot_path = format!("/v1/ai/markets/{market}/snapshot");
    assert_eq!(gateway.request("GET", &snapshot_path, None, &Value::Null).0, 401);
    let (code, snapshot) = gateway.request("GET", &snapshot_path, Some(&credential), &Value::Null);
    assert_eq!(code, 200, "{snapshot}");
    assert_eq!(snapshot["result"]["committed"], true);
    assert_eq!(snapshot["result"]["freshness"]["label"], "current");
    assert_eq!(snapshot["result"]["snapshot_id"], format!("0x{}", first["id"].as_str().unwrap_or_else(|| panic!("{}", "snapshot"))));
    let participants = format!("/v1/ai/markets/{market}/participants?limit=1");
    let (code, page) = gateway.request("GET", &participants, Some(&credential), &Value::Null);
    assert_eq!(code, 200, "{page}");
    let cursor = page["result"]["cursor"].as_str().unwrap_or_else(|| panic!("{}", "durable authenticated cursor"));
    assert_eq!(page["result"]["rows"].as_array().unwrap_or_else(|| panic!("{}", "first rows")).len(), 1);
    let continue_path = format!("{participants}&cursor={cursor}");
    let (code, before) = gateway.request("GET", &continue_path, Some(&credential), &Value::Null);
    assert_eq!(code, 200, "{before}");
    assert_ne!(before["result"]["rows"], page["result"]["rows"]);
    let (_, second) = ai_producer::ingest(&database, 1001, 5);
    assert_ne!(first["id"], second["id"]);
    let (_, status, commit) = ai_producer::consensus(1001);
    *observation.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "live head")) = (status, commit);
    let (code, live) = gateway.request("GET", &snapshot_path, Some(&credential), &Value::Null);
    assert_eq!(code, 200, "{live}");
    assert_eq!(live["result"]["snapshot_id"], format!("0x{}", second["id"].as_str().unwrap_or_else(|| panic!("{}", "new head"))));
    assert_eq!(live["result"]["freshness"]["label"], "current");
    drop(gateway);
    // The producer's runtime is stopped at the end of ingest. Reopening it and
    // the gateway must keep the old cursor pinned while publishing a new head.
    let (_, recovered) = ai_producer::ingest(&database, 1001, 5);
    assert_eq!(second, recovered);
    let gateway = Gateway::at(|| Ok(ai_clock()));
    let (code, after) = gateway.request("GET", &continue_path, Some(&credential), &Value::Null);
    assert_eq!(code, 200, "{after}");
    assert_eq!(after, before);
    let (_, status, commit) = ai_producer::consensus(1020);
    *observation.lock().unwrap_or_else(|error| panic!("{}: {error:?}", "live head")) = (status, commit);
    let (code, stale) = gateway.request("GET", &snapshot_path, Some(&credential), &Value::Null);
    assert_eq!(code, 200, "{stale}");
    assert_eq!(stale["result"]["freshness"], json!({"label":"stale","lag":"19"}));
    let second_credential = credential.clone();
    for _ in 0..10 {
        let _ = gateway.request("GET", &snapshot_path, Some(&second_credential), &Value::Null);
    }
    assert_eq!(gateway.request("GET", &snapshot_path, Some(&credential), &Value::Null).0, 429, "per-principal limiter is mounted");
    // Revocation is an authoritative Redis write; even a live cursor and an
    // exhausted principal quota cannot bypass the next credential check.
    let replacement = KeyRecord {
        key_id: "89abcdef0123456789abcdef".into(),
        ..key.clone()
    };
    fixture.store().rotate_key(&key, &replacement, "repair-revoke-by-rotation").unwrap_or_else(|error| panic!("{}: {error:?}", "real durable revocation of the old credential"));
    assert_eq!(gateway.request("GET", &continue_path, Some(&credential), &Value::Null).0, 401);
    drop(gateway);
    let gateway = Gateway::at(|| Ok(ai_clock()));
    assert_eq!(gateway.request("GET", &continue_path, Some(&credential), &Value::Null).0, 401);
}

#[test]
fn readiness_ignores_legacy_backends_and_observes_live_dependencies() {
    let _lock = ENVIRONMENT.lock().unwrap_or_else(std::sync::PoisonError::into_inner);
    let _env = Environment::clear();
    let fixture = Fixture::start();
    let store = history_store(true);
    let indexed = Arc::clone(&store);
    let peer = Peer::start(move |mut tcp| {
        let request = http::read_request(&mut tcp, 16384).unwrap_or_else(|error| panic!("{}: {error:?}", "indexer readiness request"));
        let readiness = api::Readiness {
            sources: vec!["paxeer"],
            stall_after_secs: 300,
            started_at: 0,
            explorer: None,
        };
        let response = api::route_with(&indexed, &readiness, &request.method, &request.path);
        layerx_platform_internal::http::write_response(&mut tcp, &response).unwrap_or_else(|error| panic!("{}: {error:?}", "real indexer readiness"));
    });
    std::env::set_var("LAYERX_GATEWAY_INDEXER_URL", format!("http://{}", peer.address));
    let consensus = Peer::https(Arc::clone(&fixture.tls), |request| {
        let call: Value = serde_json::from_slice(&request.body).unwrap_or_else(|error| panic!("{}: {error:?}", "consensus call"));
        assert_eq!(call["method"], "status");
        json_response(200, &json!({"jsonrpc":"2.0","id":call["id"],"result":support::fixture("consensus")["status"]}))
    });
    std::env::set_var("LAYERX_GATEWAY_CONSENSUS_RPC_URL", consensus.url());
    let gateway = Gateway::start();
    let (status, mut without) = gateway.request("GET", "/readyz/core", None, &Value::Null);
    assert_eq!(status, 200, "{without}");
    drop(gateway);
    let retired_calls = Arc::new(AtomicUsize::new(0));
    let calls = Arc::clone(&retired_calls);
    let retired = Peer::https(Arc::clone(&fixture.tls), move |_| {
        calls.fetch_add(1, Ordering::AcqRel);
        json_response(503, &json!({"error":"retired"}))
    });
    for variable in ["LAYERX_GATEWAY_COMPONENT_URL", "LAYERX_GATEWAY_AUTHORITY_URL", "LAYERX_GATEWAY_PROGRAM_REGISTRY_URL", "LAYERX_GATEWAY_PUBLIC_CORE_URL"] {
        std::env::set_var(variable, retired.url());
    }
    for variable in ["LAYERX_GATEWAY_COMPONENT_TOKEN_FILE", "LAYERX_GATEWAY_AUTHORITY_TOKEN_FILE", "LAYERX_GATEWAY_PROGRAM_REGISTRY_TOKEN_FILE"] {
        std::env::set_var(variable, fixture.secret("legacy-token", "legacy-token"));
    }
    std::env::set_var(
        "LAYERX_GATEWAY_MODULE_REGISTRY_FILE",
        fixture.secret("modules", &json!({"schema_version":2,"assets":[{"asset":"11".repeat(32),"currency":"LXP","decimals":6,"symbol":"LXP"}],"modules":[{"module":1,"ordinals":[1]}]}).to_string()),
    );
    let identity=layerx_platform_internal::base64::decode("MIIJ7wIBAzCCCaUGCSqGSIb3DQEHAaCCCZYEggmSMIIJjjCCA/oGCSqGSIb3DQEHBqCCA+swggPnAgEAMIID4AYJKoZIhvcNAQcBMF8GCSqGSIb3DQEFDTBSMDEGCSqGSIb3DQEFDDAkBBBLBOUKCMVSVsCqP2hf2vrkAgIIADAMBggqhkiG9w0CCQUAMB0GCWCGSAFlAwQBKgQQDnbUBkLhDuz6TEQtuGEQ5YCCA3DOLQXNJjyTka4ZbNgas45Mwpq//QZyWZl2ZjQdmSAd7v2r3TMCzl4Ycdz3/z/0U5SL7iPa2BcyEu6ph/SyJnQzEjyb4ELIsxMt0AWOZP0JSpyTOsS3Jv2CKbjJE5fe6r0vjUmGgsjYwhHMq+I38OVIqcGOJlszeFTkV+yTmPft5dZvYjh9SaE+eczxk+rJ5sGTRhjjSaRfuleP+bwWHpDKaXidT+3SqzZaq/O0AsZKJnyPZqgWlDcyKIfxVkh72Muzn9Kwvmjx64hVhg/GzIMBgBey1AZHk6I2LiuTnwEgyQY//7DdTqe3A1XDC8i1KwwnihCI8gH363k64SZrshLKLEtXiu9cTkXC8JnzrVthMnsYmOHsOvJDEL4CGbg6ILyq8jZNyWOr36l+eC9b7dnvlGQ2kjzYtSzZc6QNVfKZlDjflmE9XMyqijzWvM+JNxOkut7aTQcVvZyEya5fIZP3beiyrL8ctujVTw9leHv2QNezXxIcY0dp7ELhy9uszsVPhiiz/YkDtPaMyeK+loGPvz2XtauKzAdJRkg4eo6bLVXGA+rLruVoa4Wma+UScrBx6bsCQqcizJXqRkGkSW3Pf4axot42EJAkmYhm86SVmc0BvC/Od2BR0gtAKtrKw5VPWPVP6Azp+kZSYJbKH14AIU19oCrgudWLSUOHVLuIzf7QuU15n+JmtP1nbM0nF1qqu7gc1uKy4I82sw/kpDMgudCFaOiHzPRiGoNoLDgDzWa4gApmH+5vY0pVjwVHHE/aXhpQg5VGa3PPL/YwJuktqm4JkyXIwgOlxThLnYdvR6xjy/EQ8Sl3W6VFQMrbUnFzryRy1cCb/fMlxmoNyf+HPwMnDlotSXrZ/o9KvgkYj4pp1xRgYKHKh3sGDnQzo6Zd6bjcIrIErpLXqS+7TylJk647RHth97xWxT75hSZ/7BmQpDRA+gaibM39rltfIXlsqBOVR2HCWAPrAatI74k+rquuOD69/FIHjsJYJiKWZYEPhR0aMis56qcsnRoyfH12hdwzCV5TtybauGUrJ+q1f2TO+hQ793fyk2PMEpGwijA1iAUHGe0NqSW5zdXZuQjPea/fG4/PG7nM0tPbfPY8rnoPMFYI8zORmwh2iaGOMmzAxsi08b6L/owKQdg1Gj10o5FvLYlt1S+w6JIK+QNmMIIFjAYJKoZIhvcNAQcBoIIFfQSCBXkwggV1MIIFcQYLKoZIhvcNAQwKAQKgggU5MIIFNTBfBgkqhkiG9w0BBQ0wUjAxBgkqhkiG9w0BBQwwJAQQWokFu2IZvOdkDUrEc9yNggICCAAwDAYIKoZIhvcNAgkFADAdBglghkgBZQMEASoEEC7HqzB8NW4B8LbQzAVC7dcEggTQxn/hUp2RHm3oPnhkXV1HH5WmsXiZMWFjp0nygFoeR33yIS408CXgJNAkPWm05smZhVJ/C7ikSLILtP3gSxVDRbDBzQjhqbpcMU3CocaGKV655cc7ux5qMCBpM4c82ZsqvtVv4XStFLAanzMy0/Iko0IZTARVko3/ae3FWtf93DNaBVfO0J3ri6RYc7zDbcQClHZ1k7KtCawoLv8slXhB0/GsQ44de5+M5Nm5oBXcoZbC2XjiuybVTycB/qq9AM3MSFVVNms/D4aOzi2qrp1bAYXyZrkeSeZ/udahGacd2fNWdHLeQIe+PP5wNynVbMSnkfNC/zI0ee4T4DhncP7D3t/qN8BRiSV+xLx4oLilwlnP3vnKrm7DXTqawJxUlDh7YxW75LmHm0KUYpPn3dvleE8nn3+gZ4JnCZTvo9OGavk0a3IRAo9X1xlls4exfkDdtoG6HhQAbGpgNMaOy/LDUR7d4sN+mPT2bRwzwt2FA8dd03NwhAgFIO3aW/VpXSArqwaYBFxi0zxOU4v72hMARM1ZFRMKrU0gTiFtbRvv9RoVDxgsrmiS17t9gq022BV5BfrUckmhN4QUWmRgkk3UHVbbTFOc3682nmpse+HbLuFa10LbShhGUxlW36NxBU0YrKWOn0aLgidCFZ9t+uP6XN6WAtBalLIQlCE/M3sbF1MbCnOCKM6KMnVvPN+SAt+1WmdSjOTk6y+9zK6GKKvNJNPwbk4b0o8KWKxxSXbMdvp/dmamJdP7wwmCITcZ+lh2inJxjnxJZh3vLwYIm96LIh0tEB6FZz6CILHMtyZRyChrPLUZJkTzW8R370sGe52a3GyZOc8XpQAb2O9AFhOf6sbYaHg5LpIa5idIbdzPl+Tef+sOjEN3MhLXN+VUifEBl/IH6vfuQLZuvirFsmp3pP2SCezP6Z2tu1nqlV61G0sG2yn6O/nZVcyOlilSc74R3MqrF5ASORqzKedmDdFaJMk9Nkw/CUjy6MenqdB5L7xrT8zXxxF/R3cB8x/8s/pOqVLLnQLSFApBO5FYnjLrmGArSS+OOwVm892oZPZMdBMBUhU3sawoTf5/9vB3x/dNNEXNKTjYJYuCYV2dtnAUHLbCXJexdALD01CAGA1DKkncZQoLmdbeSr7+0HuuDEXDpnGIMSPLrWf2LEAw/ct33RJICcyi/TTa9L0xUruYPqjsLl2+y1KonYSF5tU8UR7lLcpBJ6K/AV8GAJIJymDd2GDq16KiuuhKKBXXaq9tqZKjhlQ/2cyHSKhRq4WOAz7mrpzPTrzkLISPsQ+ThfWvGG7j0tvL2tDIqm5ewpYnFB9ui1RlBUlksX/REhkHeXYX+cZeheudWycitQx1WdOdifVsgvk80Re73PXGnsnthX6CrFVbTLfkZDJPeWpCNO9ajgILqzE+hkNxzoERZrHjf8HfhRekSrSrr8CsMNsJsPqYPEMYNKPadbeG8MuJSKYlJKU8MqPMJXWX8WRviIRGo0ZzblEIsB0lUQS6Pc43AOsFNK48uHIWYqdqP01iOWpbhOTAkEiUQTN6ikLnIK4G/Lhe0VbpJDSIiu+tXEOeC5zkTTF+LZydBE6Vy3O4y7v4C6uhNWmaFpsp9pYn1f66KQZz73XFNYp7Z5ae/2thXE4xJTAjBgkqhkiG9w0BCRUxFgQUOL3p/qdEKMFHB/DEsMbIYOt68ekwQTAxMA0GCWCGSAFlAwQCAQUABCAvUJXHx9kGyRLfvRBORPpfWrCSIDJ1aDJgowQUtQ3PEwQI866c/hc8AysCAggA").unwrap_or_else(|| panic!("{}", "recorded legacy client identity"));
    let path = fixture.directory.path().join("legacy.p12");
    fs::write(&path, identity).unwrap_or_else(|error| panic!("{}: {error:?}", "identity file"));
    std::env::set_var("LAYERX_GATEWAY_CLIENT_IDENTITY_PKCS12", path);
    std::env::set_var("LAYERX_GATEWAY_CLIENT_IDENTITY_PASSWORD_FILE", fixture.secret("identity-password", "legacy-password"));
    let gateway = Gateway::start();
    let (status, mut with) = gateway.request("GET", "/readyz/core", None, &Value::Null);
    assert_eq!(status, 200, "{with}");
    for document in [&mut without, &mut with] {
        let observed = document["observed_at_ms"].as_u64().unwrap_or_else(|| panic!("{}", "observation time"));
        assert_eq!(document["valid_until_ms"].as_u64(), Some(observed + 30_000));
        assert_eq!(document["capabilities"]["layerx_activity"], json!({"state":"not_configured","reason":"not_configured"}));
        for retired in ["core_agent_boundary", "public_core", "independent_receipt_authority", "program_registry", "identity", "principal_state_boundary"] {
            assert!(document["backends"].get(retired).is_none());
            assert!(document["components"].get(retired).is_none());
        }
        document.as_object_mut().unwrap_or_else(|| panic!("{}", "readiness document")).remove("observed_at_ms");
        document.as_object_mut().unwrap_or_else(|| panic!("{}", "readiness document")).remove("valid_until_ms");
    }
    assert_eq!(with, without);
    assert_eq!(retired_calls.load(Ordering::Acquire), 0);
    store.record_source_failure("paxeer", "recorded upstream failure").unwrap_or_else(|error| panic!("{}: {error:?}", "source failure"));
    let (status, failed) = gateway.request("GET", "/readyz/core", None, &Value::Null);
    assert_eq!(status, 503, "{failed}");
    assert_eq!(failed["backends"]["indexer"]["state"], "unavailable");
    store.record_source_success("paxeer", Some(10), Some(10)).unwrap_or_else(|error| panic!("{}: {error:?}", "indexer recovery"));
    assert_eq!(gateway.request("GET", "/readyz/core", None, &Value::Null).0, 200);
}

#[test]
fn history_registry_discovery_stops_when_resolved_and_refuses_exhaustion() {
    let _lock = ENVIRONMENT.lock().unwrap_or_else(std::sync::PoisonError::into_inner);
    let _env = Environment::clear();
    let _fixture = Fixture::start();
    for resolved in [true, false] {
        let store = history_store(false);
        let assets: Vec<_> = (0..3300)
            .map(|i| AssetRow {
                asset: format!("{i:064x}"),
                chain: "layerx".into(),
                kind: "layerx".into(),
                address: None,
                denom: Some(if resolved && i == 0 { "utoken".into() } else { format!("ucatalogue{i}") }),
                metadata: json!({"symbol":"TARGET","decimals":9}),
            })
            .collect();
        store.register_assets(&assets).unwrap_or_else(|error| panic!("{}: {error:?}", "real registry catalogue"));
        let reads = Arc::new(AtomicUsize::new(0));
        let peer = indexer(store, Arc::new(AtomicUsize::new(0)), Arc::clone(&reads));
        std::env::set_var("LAYERX_GATEWAY_INDEXER_URL", format!("http://{}", peer.address));
        let gateway = Gateway::start();
        let answer = gateway.rpc("px_getHistory", &json!([ACCOUNT, null, 100]));
        if resolved {
            assert!(answer.get("error").is_none(), "{answer}");
            assert_eq!(answer["result"]["items"][1]["asset_metadata"]["symbol"], "TARGET");
            assert_eq!(answer["result"]["items"][1]["asset_metadata"]["decimals"], 9);
            assert_eq!(reads.load(Ordering::Acquire), 1);
        } else {
            assert_eq!(answer["error"]["data"]["code"], "registry_unavailable");
            assert_eq!(reads.load(Ordering::Acquire), 32);
        }
    }
}
