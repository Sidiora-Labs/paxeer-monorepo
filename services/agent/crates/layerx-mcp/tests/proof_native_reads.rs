//! Proves: NativeReadRoute::read -> daemon::serve_read_routes HTTP -> ProgramReads::execute.
//!
//! Starts the repository's disposable sequencer and reads its generated localnode.env;
//! no node, daemon route, HTTP client or consumer is replaced. The daemon is stopped
//! and reopened between reads, and the same consumer must refuse while it is down.
//! Repairs layerx-mcp/src/boundary.rs::AgentSurface::native_read accepting JSON null,
//! arrays and scalars as evidence by decoding through decode_native_read_result.
//! Repairs produce_transaction requesting a pending nonce from the proposer by
//! using layerx-client/src/sequencer.rs::SequencerEndpoint::latest_nonce.
//! Repairs AgentSurface::native_read losing the unavailable history method's
//! identity when the daemon answers HTTP 503: the consumer receives a typed
//! refusal naming history.list and px_getHistory instead of a transport status.
//!
//! The proof belongs to the consumer crate so layerx-agentd does not depend on
//! layerx-mcp. The HTTP implementation below is the exact module main.rs imports,
//! including its listener, parser, dispatcher and response writer.
//! Out-of-scope behavior: src/read/native.rs::NativeReadRoute::receipt labels even
//! committed receipts "receipted". Assert that current behavior below; the expected
//! eventual behavior is to preserve the authenticated receipt's strongest rung.
//! layerx-client/src/sequencer.rs::SequencerEndpoint::history calls px_getHistory,
//! which belongs to services/platform/hosted/gateway/src/history.rs. The local
//! node does not serve it, so history.list must remain explicitly unavailable.

#[allow(dead_code)]
#[path = "../../layerx-agentd/tests/proof_native_reads.rs"]
mod native_reads;

use native_reads::Service;

use std::os::unix::fs::{DirBuilderExt, OpenOptionsExt};
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_agentd::node_rpc::NodeRpcConfig;
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{SequencerBlock, SequencerEndpoint};
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_mcp::boundary::{
    decode_native_read_result, AgentSurface, BoundaryRefusal, ProgramReads, ToolBoundary,
};
use layerx_mcp::server::catalogue;
use layerx_programs::hex;
use serde_json::{json, Value};
use zeroize::Zeroizing;

const BEARER: &str = "native-reads-proof-agent-credential";

#[track_caller]
fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("native reads proof: {error:?}"))
}

struct LocalNode {
    script: PathBuf,
    home: PathBuf,
}

impl LocalNode {
    fn start() -> (Self, String, Zeroizing<String>) {
        let root = Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .nth(4)
            .expect("repository root for tools/qualification/localnode/v6120-node.sh");
        let script = root.join("tools/qualification/localnode/v6120-node.sh");
        assert!(
            script.is_file(),
            "missing producer harness: {}",
            script.display()
        );
        let directory = Path::new("/tmp/lx-localnode").join(format!(
            "native-reads-{}-{}",
            std::process::id(),
            must(SystemTime::now().duration_since(UNIX_EPOCH)).as_nanos()
        ));
        must(
            std::fs::DirBuilder::new()
                .recursive(true)
                .mode(0o700)
                .create(&directory),
        );
        let environment = directory.join("localnode.env");
        let log_path = directory.join("startup.log");
        let log = must(
            std::fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(&log_path),
        );
        let status = Command::new("bash")
            .arg(&script)
            .args(["--sequencer", "--env-out"])
            .arg(&environment)
            .env("TMPDIR", "/tmp")
            .env_remove("PAXD")
            .stdout(Stdio::from(must(log.try_clone())))
            .stderr(Stdio::from(log))
            .status()
            .expect("start tools/qualification/localnode/v6120-node.sh producer");
        assert!(
            status.success(),
            "tools/qualification/localnode/v6120-node.sh failed ({status}); evidence: {}",
            log_path.display()
        );
        let text = Zeroizing::new(must(std::fs::read_to_string(&environment)));
        let field = |name: &str| {
            text.lines()
                .filter_map(|line| line.split_once('='))
                .find_map(|(key, value)| (key == name).then_some(value))
                .unwrap_or_else(|| panic!("producer localnode.env is missing {name}"))
        };
        // Install cleanup before decoding the endpoint or key so assertion failures
        // in the proof also stop the producer's node and feeder processes.
        let node = Self {
            script,
            home: PathBuf::from(field("LOCAL_NODE_HOME")),
        };
        let url = field("LOCAL_NODE_SEQUENCER_RPC").to_owned();
        let key = Zeroizing::new(field("LOCAL_NODE_KEY").to_owned());
        must(std::fs::remove_file(environment));
        (node, url, key)
    }
}

impl Drop for LocalNode {
    fn drop(&mut self) {
        let status = Command::new("bash")
            .arg(&self.script)
            .arg("--stop")
            .arg(&self.home)
            .env("TMPDIR", "/tmp")
            .status();
        if !matches!(status, Ok(status) if status.success()) {
            if thread::panicking() {
                eprintln!(
                    "failed to stop native reads producer: {}",
                    self.home.display()
                );
            } else {
                panic!(
                    "failed to stop native reads producer: {}",
                    self.home.display()
                );
            }
        }
    }
}

impl Service {
    fn surface(&self, bearer: &str) -> AgentSurface {
        must(AgentSurface::new(
            &self.address.to_string(),
            bearer.to_owned(),
            &"21".repeat(32),
            Duration::from_secs(5),
        ))
    }
}

fn execute(reads: &mut ProgramReads, name: &str, arguments: Value) -> Value {
    let tool = *catalogue()
        .iter()
        .find(|tool| tool.name == name)
        .expect("real catalogue tool");
    must(reads.execute(tool, &arguments))
}

fn produce_transaction(
    client: &SequencerEndpoint,
    chain_id: u64,
    encoded: &str,
) -> ([u8; 32], u64) {
    let secret = Zeroizing::new(must(hex::decode_digest(
        encoded.strip_prefix("0x").unwrap_or(encoded),
    )));
    let signing_key = must(k256::ecdsa::SigningKey::from_slice(secret.as_ref()));
    let sender = must(layerx_crypto::secp256k1::evm_address(
        signing_key
            .verifying_key()
            .to_encoded_point(false)
            .as_bytes(),
    ));
    let call = Eip1559Call {
        chain_id,
        nonce: must(client.latest_nonce(&sender)),
        max_priority_fee_per_gas: 1_000_000_000,
        max_fee_per_gas: 100_000_000_000,
        gas_limit: 21_000,
        to: [21; 20],
        data: Vec::new(),
    };
    let raw = must(call.sign(&secret));
    let hash = must(client.send_raw_transaction(&raw));
    assert_eq!(hash, transaction_hash(&raw));
    let deadline = Instant::now() + Duration::from_secs(30);
    loop {
        if let Some(receipt) = must(client.transaction_receipt(&hash)) {
            assert!(receipt.status, "producer transaction must succeed");
            return (hash, receipt.block_number);
        }
        assert!(Instant::now() < deadline, "real transaction did not commit");
        thread::sleep(Duration::from_millis(50));
    }
}

fn observe(reads: &mut ProgramReads, hash: [u8; 32], block: &SequencerBlock) -> Vec<Value> {
    let transaction = hex::encode(&hash);
    let receipt = execute(reads, "receipt.get", json!({"activity_id": transaction}));
    assert_eq!(receipt["transaction_hash"], transaction);
    assert_eq!(receipt["height"], block.number);
    assert_eq!(receipt["rung"], "receipted");
    assert!(receipt["receipt"]
        .as_str()
        .is_some_and(|value| !value.is_empty()));
    let proof = execute(reads, "proof.get", json!({"activity_id": transaction}));
    assert_eq!(proof["receipt"], receipt["receipt"]);
    assert_eq!(proof["receipt_key"], receipt["receipt_key"]);
    assert_eq!(proof["kind"], "receipt");
    assert_eq!(proof["proof_type"], "sequencer_signature");
    let checkpoint = execute(
        reads,
        "checkpoint.get",
        json!({"sequence": block.number.to_string()}),
    );
    assert_eq!(checkpoint["height"], block.number);
    assert_eq!(checkpoint["block_hash"], hex::encode(&block.hash));
    assert_eq!(checkpoint["parent_hash"], hex::encode(&block.parent_hash));
    assert_eq!(checkpoint["timestamp"], block.timestamp);
    assert_eq!(
        checkpoint["transactions_root"],
        hex::encode(&block.transactions_root)
    );
    assert_eq!(
        checkpoint["receipts_root"],
        hex::encode(&block.receipts_root)
    );
    assert_eq!(checkpoint["verified"], false);
    assert!(checkpoint["rung"].is_null());
    let availability = execute(
        reads,
        "availability.get",
        json!({"batch": block.number.to_string()}),
    );
    assert_eq!(availability["height"], block.number);
    assert_eq!(availability["complete"], true);
    assert_eq!(
        availability["transactions_root"],
        checkpoint["transactions_root"]
    );
    assert_eq!(availability["receipts_root"], checkpoint["receipts_root"]);
    assert_eq!(availability["transaction_count"], block.transactions.len());
    assert!(availability["rung"].is_null());
    let account = format!("0x{}", hex::encode(&[21; 20]));
    let history_tool = *catalogue()
        .iter()
        .find(|tool| tool.name == "history.list")
        .expect("real history catalogue tool");
    assert_eq!(
        reads.execute(history_tool, &json!({"account": account, "limit": "2"})),
        Err(BoundaryRefusal::Unavailable(
            "history.list evidence from px_getHistory is unavailable".to_owned()
        )),
        "the node's missing px_getHistory must reach the consumer as a typed refusal"
    );
    vec![receipt, proof, checkpoint, availability]
}

#[test]
fn native_reads_producer_http_consumer_reopens_and_refuses_malformed_results() {
    let (_node, url, key) = LocalNode::start();
    let client = must(SequencerEndpoint::new(&url, &[]));
    let config = NodeRpcConfig {
        url,
        chain_id: must(client.chain_id()),
        deadline: Duration::from_secs(5),
    };
    let (hash, height) = produce_transaction(&client, config.chain_id, &key);
    let block = must(client.block_by_number(height)).expect("committed producer block");
    let keys = must(client.receipt_keys());
    let Lookup::Verified(verified) = must(lookup_authenticated(
        &client,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )) else {
        panic!("real sequencer must emit a signed receipt for the produced transaction");
    };

    let service = Service::start(&config, "127.0.0.1:0".parse().expect("loopback"));
    let address = service.address;
    let surface = service.surface(BEARER);
    let wrong_credential = service.surface("wrong-native-reads-agent-credential");
    let mut reads = ProgramReads::new(service.surface(BEARER));
    let first = observe(&mut reads, hash, &block);
    assert_eq!(first[0]["receipt"], hex::encode(verified.canonical_bytes()));
    assert_eq!(
        first[0]["receipt_key"]["key_id"],
        hex::encode(&verified.execution().key_id)
    );
    for path in [
        "/v1/reads/receipt/not-a-digest",
        "/v1/reads/proof/not-a-digest",
        "/v1/reads/checkpoint/01",
        "/v1/reads/availability/01",
        "/v1/reads/history/account?limit=0",
    ] {
        assert!(
            matches!(
                surface.native_read(path),
                Err(BoundaryRefusal::Malformed(_))
            ),
            "{path}"
        );
    }
    let path = format!("/v1/reads/checkpoint/{height}");
    assert_eq!(
        wrong_credential.native_read(&path),
        Err(BoundaryRefusal::Unauthorized)
    );
    assert!(matches!(
        surface.native_read("/v1/reads/availability/18446744073709551615"),
        Err(BoundaryRefusal::Unavailable(_))
    ));

    // Corrupt an actual producer envelope at the consumer's decoding boundary.
    let body = must(serde_json::to_string(&first[2]));
    assert_eq!(must(decode_native_read_result(&body)), first[2]);
    for malformed in [
        body[..body.len() - 1].to_owned(),
        format!("[{body}]"),
        "null".to_owned(),
        "42".to_owned(),
    ] {
        assert!(matches!(
            decode_native_read_result(&malformed),
            Err(BoundaryRefusal::Malformed(_))
        ));
    }

    drop(service);
    assert!(
        matches!(
            surface.native_read(&path),
            Err(BoundaryRefusal::Unavailable(_))
        ),
        "a stopped server must never be answered from a cache"
    );
    let reopened = Service::start(&config, address);
    let second = observe(&mut reads, hash, &block);
    assert_eq!(first, second);
    assert_eq!(must(surface.native_read(&path)), second[2]);
    drop(reopened);
}
