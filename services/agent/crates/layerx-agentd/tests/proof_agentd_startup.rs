//! Proves: StartupConfig producers -> protected key=value files and exact LAYERX_* overrides -> config::load, Gate::new, RuntimeConfig::load and explorer-receipt-reader.py::main.
//!
//! Repairs config.rs::reject_retired_setting: the removed sequencer authority
//! source must be classified as Retired, just like the removed protocol version.
//! Repairs runtime.rs::run: bound the actual configuration read, rather than
//! trusting a metadata length obtained before an unbounded read.
//! Repairs this proof's Python launch: retain the interpreter's toolchain
//! environment and use Python isolated mode to exclude ambient Python settings.
//!
//! The inventory describes the former LNI interface. At this revision the two
//! legacy keys are retired; the successful path uses expected_chain_id and
//! receipt_key_source. The explorer producer is retired and the mirror consumes
//! its own paxeer JSON configuration, not agentd's old keys. Both real consumers
//! must refuse those old inputs. No daemon, RPC transport or receipt is doubled.
//! Outside scope: services/agent/crates/layerx-agentd/src/main.rs::config
//! uses LAYERX_AGENT_NODE_ENDPOINT instead of config::load. This proves the
//! library startup boundary; it does not assert that main calls that loader.
//! Outside scope: protocol_evidence.rs::ProtocolEvidenceVerifier::load refuses
//! ReceiptKeySource::NodeRpc; the test asserts that current fail-closed behaviour.

#[path = "config.rs"]
mod config_consumer;

use std::collections::BTreeMap;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::Command;

use layerx_agentd::boot::{Gate, GateError, Status};
use layerx_agentd::config::{load, ConfigError, RejectionReason, StartupConfig};
use layerx_agentd::protocol_evidence::VerifierPolicyError;
use layerx_sdk::production::MirrorVerifier;
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("startup proof: {error:?}"))
}

struct Sources(PathBuf);

impl Sources {
    fn new() -> Self {
        let mut nonce = [0_u8; 16];
        must(getrandom::fill(&mut nonce));
        let root = std::env::temp_dir().join(format!(
            "proof-agentd-startup-{}-{}",
            std::process::id(),
            hex::encode(nonce)
        ));
        must(fs::create_dir(&root));
        Self(must(fs::canonicalize(root)))
    }

    fn path(&self, name: &str) -> PathBuf {
        self.0.join(name)
    }

    fn agent(&self, values: &BTreeMap<String, String>) -> PathBuf {
        let path = self.path("agent.kv");
        config_consumer::write_protected(&path, config_consumer::source(values));
        path
    }
}

impl Drop for Sources {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn publisher_source(config: &StartupConfig) -> Value {
    let rpc = json!({
        "endpoints": [{
            "url": config.node_endpoint,
            "ca_certificate_der": "/run/secrets/ca.der",
            "bearer_token_file": "/run/secrets/rpc.token",
            "independent_backend": "startup-proof"
        }],
        "quorum": 1,
        "connect_timeout_ms": 3000,
        "request_timeout_ms": 15000,
        "maximum_response_bytes": 16777216
    });
    json!({
        "state_directory": "/var/lib/layerx-mirror",
        "first_height": 1,
        "poll_interval_ms": 5000,
        "freshness_budget_blocks": 64,
        "status_listen": "127.0.0.1:9091",
        "paxeer": {
            "rpc": rpc,
            "chain_id": config.expected_chain_id,
            "genesis_hash_hex": hex::encode([1_u8; 32]),
            "receipt_keys": [{
                "sequencer_id_hex": hex::encode([2_u8; 32]),
                "public_key_hex": hex::encode(
                    ed25519_dalek::SigningKey::from_bytes(&[3; 32]).verifying_key().to_bytes()
                ),
                "first_height": 1,
                "last_height": 100
            }]
        },
        "ethereum": {
            "rpc": rpc,
            "chain_id": 1,
            "genesis_hash_hex": hex::encode([4_u8; 32]),
            "archive_contract_hex": hex::encode([5_u8; 20]),
            "archive_code_hash_hex": hex::encode([6_u8; 32]),
            "required_confirmations": 12,
            "maximum_reorg_depth": 256,
            "chunk_bytes": 65536,
            "transaction_gas_limit": 12000000,
            "maximum_fee_per_gas": 1000000000,
            "maximum_priority_fee_per_gas": 1000000,
            "signer": {
                "key_handle": "mirror/ethereum/alpha",
                "public_key": hex::encode([7_u8; 33]),
                "timeout_ms": 5000,
                "transport": {"kind": "uds", "socket": "/run/mirror-signer/signer.sock"}
            }
        }
    })
}

fn write_publisher(path: &PathBuf, source: &Value) {
    config_consumer::write_protected(path, must(serde_json::to_vec(source)));
}

#[test]
fn node_endpoint() {
    let sources = Sources::new();
    let values = config_consumer::values();
    let agent = sources.agent(&values);
    let publisher = sources.path("mirror.json");
    let configuration_error =
        MirrorVerifier::publisher_configuration(&sources.path("missing.json")).unwrap_err();
    assert_eq!(format!("{configuration_error:?}"), "Configuration");
    let from_file = must(load(&agent, &BTreeMap::new()));
    assert_eq!(from_file.node_endpoint, "https://node.example/rpc");

    for endpoint in ["http://127.0.0.1:8545", "http://127.0.0.1:8546"] {
        let environment =
            BTreeMap::from([("LAYERX_NODE_ENDPOINT".to_owned(), endpoint.to_owned())]);
        // Each call reopens the file; writing a new value also detects caching.
        for _ in 0..2 {
            let produced = must(load(&agent, &environment));
            assert_eq!(produced.node_endpoint, endpoint);
            let client = must(layerx_sdk::Client::direct_node(
                &produced.node_endpoint,
                &[],
            ));
            assert_eq!(client.deployment(), layerx_sdk::Deployment::DirectNode);
            assert!(client.direct_node_client().is_some());
            write_publisher(&publisher, &publisher_source(&produced));
            let observed = must(MirrorVerifier::publisher_configuration(&publisher));
            assert_eq!(observed.paxeer.chain_id, produced.expected_chain_id);
            assert_eq!(observed.paxeer.rpc.endpoints[0].url, endpoint);
        }
    }
    assert_eq!(must(load(&agent, &BTreeMap::new())), from_file);
    assert_eq!(
        load(
            &agent,
            &BTreeMap::from([(
                "LAYERX_NODE_ENDPOINT".to_owned(),
                "unix:///run/retired.sock".to_owned(),
            )])
        ),
        Err(ConfigError {
            setting: "node_endpoint".to_owned(),
            reason: RejectionReason::Invalid
        })
    );
    config_consumer::write_protected(&agent, b"node_endpoint=\xff");
    assert_eq!(
        load(&agent, &BTreeMap::new()).err(),
        Some(ConfigError {
            setting: "configuration_file".to_owned(),
            reason: RejectionReason::InvalidEncoding,
        })
    );
    config_consumer::write_protected(&publisher, b"{\"paxeer\":");
    assert_eq!(
        MirrorVerifier::publisher_configuration(&publisher).err(),
        Some(configuration_error)
    );
    assert_eq!(
        MirrorVerifier::publisher_configuration(&publisher).err(),
        Some(configuration_error)
    );
    config_consumer::write_protected(&publisher, vec![b' '; 1024 * 1024 + 1]);
    assert_eq!(
        MirrorVerifier::publisher_configuration(&publisher).err(),
        Some(configuration_error)
    );
}

fn assert_retired(file_key: &str, environment_key: &str) {
    let sources = Sources::new();
    let baseline = config_consumer::values();
    let publisher = sources.path("mirror.json");
    let configuration_error =
        MirrorVerifier::publisher_configuration(&sources.path("missing.json")).unwrap_err();
    assert_eq!(format!("{configuration_error:?}"), "Configuration");
    for value in ["2", "", "malformed"] {
        let mut producer = baseline.clone();
        producer.insert(file_key.to_owned(), value.to_owned());
        let agent = sources.agent(&producer);
        for _ in 0..2 {
            assert_eq!(
                load(&agent, &BTreeMap::new()).err(),
                Some(ConfigError {
                    setting: file_key.to_owned(),
                    reason: RejectionReason::Retired,
                })
            );
        }
        let agent = sources.agent(&baseline);
        let environment = BTreeMap::from([(environment_key.to_owned(), value.to_owned())]);
        for _ in 0..2 {
            assert_eq!(
                load(&agent, &environment).err(),
                Some(ConfigError {
                    setting: environment_key.to_owned(),
                    reason: RejectionReason::Retired,
                })
            );
        }
        let produced = must(load(&agent, &BTreeMap::new()));
        let mut source = publisher_source(&produced);
        source[file_key] = json!(value);
        write_publisher(&publisher, &source);
        assert_eq!(
            MirrorVerifier::publisher_configuration(&publisher).err(),
            Some(configuration_error)
        );
        assert_eq!(
            MirrorVerifier::publisher_configuration(&publisher).err(),
            Some(configuration_error)
        );
    }
}

#[test]
fn expected_protocol_version() {
    assert_retired(
        "expected_protocol_version",
        "LAYERX_EXPECTED_PROTOCOL_VERSION",
    );
}

#[test]
fn sequencer_authority_source() {
    assert_retired(
        "sequencer_authority_source",
        "LAYERX_SEQUENCER_AUTHORITY_SOURCE",
    );
    let sources = Sources::new();
    let authority = sources.path("authority.csv");
    let key = ed25519_dalek::SigningKey::from_bytes(&[8; 32])
        .verifying_key()
        .to_bytes();
    config_consumer::write_protected(
        &authority,
        format!(
            "layerx-sequencer-authority-v1\n{},{},0,1,100,active\n",
            hex::encode(key),
            hex::encode(key)
        ),
    );
    let mut producer = config_consumer::values();
    producer.insert(
        "receipt_key_source".to_owned(),
        must(authority.to_str().ok_or("UTF-8 path")).to_owned(),
    );
    let agent = sources.agent(&producer);
    for _ in 0..2 {
        let config = must(load(&agent, &BTreeMap::new()));
        let gate = must(Gate::new(&config));
        assert_eq!(gate.status(), &Status::NotReady);
    }
    config_consumer::write_protected(&authority, "malformed producer result\n");
    let config = must(load(&agent, &BTreeMap::new()));
    assert_eq!(
        Gate::new(&config).err(),
        Some(GateError::Evidence(
            VerifierPolicyError::AuthoritySourceMalformed
        ))
    );
    producer.insert(
        "receipt_key_source".to_owned(),
        "px_getReceiptKeys".to_owned(),
    );
    let agent = sources.agent(&producer);
    assert_eq!(
        Gate::new(&must(load(&agent, &BTreeMap::new()))).err(),
        Some(GateError::Evidence(
            VerifierPolicyError::AuthoritySourceUnavailable
        ))
    );
}

#[test]
fn explorer_consumer_refuses_the_retired_producer_twice() {
    let sources = Sources::new();
    let agent = sources.agent(&config_consumer::values());
    let root = must(
        Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .nth(4)
            .ok_or("repository root"),
    );
    let reader = root.join("tools/qualification/paxeer-x/explorer-receipt-reader.py");
    assert!(reader.is_file(), "missing consumer: {}", reader.display());
    for _ in 0..2 {
        let produced = must(load(&agent, &BTreeMap::new()));
        let output = Command::new("python3")
            .arg("-I")
            .arg(&reader)
            .env("LAYERX_NODE_ENDPOINT", &produced.node_endpoint)
            .env("LAYERX_EXPECTED_PROTOCOL_VERSION", "2")
            .env(
                "LAYERX_SEQUENCER_AUTHORITY_SOURCE",
                "/etc/layerx/authority.csv",
            )
            .output()
            .unwrap_or_else(|error| {
                panic!("run consumer {} with python3: {error}", reader.display())
            });
        assert_eq!(output.status.code(), Some(2));
        assert!(output.stdout.is_empty());
        assert!(must(String::from_utf8(output.stderr))
            .contains("daemon-backed receipt evidence production is retired; use chain receipts"));
    }
}
