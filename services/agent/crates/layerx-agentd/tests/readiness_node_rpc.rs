use layerx_agentd::boot::{readiness_gate, Gate, GateError, Status, WriteGateError};
use layerx_agentd::compat::{
    matrix, verify, CompatibilityError, CONTRACT_VERSION, DAEMON_VERSION, SDK_VERSION,
};
use layerx_agentd::config::{ReceiptKeySource, StartupConfig};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig, NodeRpcError};
use layerx_client::sequencer::SequencerError;
use serde_json::{json, Value};
use std::collections::{BTreeMap, BTreeSet};
use std::fs::{self, OpenOptions};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::OpenOptionsExt;
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

// Committed peer fixtures use the gateway producer's result object and numeric
// heights (services/platform/hosted/gateway/src/chain_reads.rs::px_getReceiptKeys).
fn serve_node(chain_checks: usize, receipt_results: Vec<Value>) -> (NodeRpcConfig, JoinHandle<()>) {
    let mut exchanges = vec![("eth_chainId", json!([]), json!("0x7d")); chain_checks];
    exchanges.extend(
        receipt_results
            .into_iter()
            .map(|result| ("px_getReceiptKeys", json!([]), result)),
    );
    serve_rpc(exchanges)
}

pub(crate) fn serve_rpc(
    exchanges: Vec<(&'static str, Value, Value)>,
) -> (NodeRpcConfig, JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let config = NodeRpcConfig {
        url: format!("http://{}/", must(listener.local_addr())),
        chain_id: 125,
        deadline: Duration::from_secs(2),
    };
    must(listener.set_nonblocking(true));
    let server = thread::spawn(move || {
        for (method, params, result) in exchanges {
            let expires = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < expires, "missing {method} request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(2))));
            must(stream.set_write_timeout(Some(Duration::from_secs(2))));
            let mut reader = BufReader::new(stream);
            let mut request_line = String::new();
            assert_ne!(must(reader.read_line(&mut request_line)), 0);
            assert_eq!(request_line, "POST / HTTP/1.1\r\n");
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let length = length.unwrap_or_else(|| panic!("content length required"));
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
        }
    });
    (config, server)
}

fn receipt_key() -> [u8; 32] {
    ed25519_dalek::SigningKey::from_bytes(&[8; 32])
        .verifying_key()
        .to_bytes()
}

fn populated_keys() -> Value {
    json!({"keys": [{
        "keyId": format!("0x{}", hex::encode(receipt_key())),
        "publicKey": format!("0x{}", hex::encode(receipt_key())),
        "firstHeight": 1,
        "lastHeight": 100,
    }]})
}

fn gate() -> Gate {
    static NEXT: AtomicU64 = AtomicU64::new(0);
    let path = std::env::temp_dir().join(format!(
        "repair-22-33-1530-policy-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    ));
    let mut file = must(
        OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&path),
    );
    must(writeln!(
        file,
        "layerx-sequencer-authority-v1\n{0},{0},0,1,100,active",
        hex::encode(receipt_key())
    ));
    drop(file);
    let result = Gate::new(&StartupConfig {
        network_id: 125,
        node_endpoint: "http://127.0.0.1/".to_owned(),
        expected_chain_id: 125,
        tenants: BTreeSet::new(),
        policy_sources: BTreeMap::new(),
        signer_configurations: BTreeMap::new(),
        verification_defaults: BTreeMap::new(),
        receipt_key_source: ReceiptKeySource::PinnedFile(path.clone()),
    });
    must(fs::remove_file(path));
    must(result)
}

fn malformed_keys() -> NodeRpcError {
    NodeRpcError::Sequencer(SequencerError::Malformed)
}

fn assert_closed(gate: &Gate) {
    let operation_ran = std::cell::Cell::new(false);
    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(
        gate.guard_write(|| operation_ran.set(true)),
        Err(WriteGateError::NotReady)
    );
    assert!(!operation_ran.get());
    assert!(matches!(
        gate.evidence_authority(),
        Err(WriteGateError::NotReady)
    ));
}

#[test]
fn daemon_is_not_ready_without_receipt_keys() {
    let mut gate = gate();
    assert_eq!(gate.status(), &Status::NotReady);
    let (node_config, server) = serve_node(2, vec![json!({"keys": []}); 4]);
    let node = must(NodeRpc::connect(&node_config));
    assert_eq!(node.chain_id(), node_config.chain_id);
    let mut mismatched_config = node_config.clone();
    mismatched_config.chain_id ^= 1;
    assert_eq!(
        NodeRpc::connect(&mismatched_config).err(),
        Some(NodeRpcError::ChainIdMismatch {
            expected: mismatched_config.chain_id,
            observed: node_config.chain_id,
        })
    );
    assert_eq!(gate.status(), &Status::NotReady);
    assert_eq!(node.receipt_keys(), Err(malformed_keys()));
    assert_eq!(
        readiness_gate(&node),
        Err(GateError::NodeRpc(malformed_keys()))
    );
    let operation_ran = std::cell::Cell::new(false);
    for _ in 0..2 {
        assert_eq!(
            gate.readiness_gate(&node),
            Err(GateError::NodeRpc(malformed_keys()))
        );
        assert_eq!(gate.status(), &Status::NotReady);
        assert_eq!(
            gate.guard_write(|| operation_ran.set(true)),
            Err(WriteGateError::NotReady)
        );
        assert!(matches!(
            gate.evidence_authority(),
            Err(WriteGateError::NotReady)
        ));
        assert!(!operation_ran.get());
        gate.disconnected();
        assert_eq!(gate.status(), &Status::NotReady);
    }
    must(server.join());
}

#[test]
fn compatibility_matrix_names_contract_major_two() {
    assert_eq!(CONTRACT_VERSION, 2);
    assert!(!matrix().is_empty());
    for row in matrix() {
        assert_eq!(row.contract_version, 2);
        for client in [
            format!("paxd/v{}.0.0/linux-amd64/go1.25", row.node_interface_major),
            format!("paxd/{}.12.3", row.node_interface_major),
        ] {
            assert_eq!(verify(DAEMON_VERSION, &client, 2, SDK_VERSION), Ok(*row));
            assert_eq!(
                verify(DAEMON_VERSION, &client, 1, SDK_VERSION),
                Err(CompatibilityError::UnsupportedContract)
            );
        }
        assert_eq!(
            verify(
                DAEMON_VERSION,
                &format!("paxd/v{}.0.0", row.node_interface_major + 1),
                2,
                SDK_VERSION
            ),
            Err(CompatibilityError::UnsupportedNodeInterface)
        );
    }
    for client in [
        "",
        "paxd",
        "paxd/v",
        "paxd/v1",
        "paxd/v1.0",
        "paxd/v1.0.bad",
        "paxd/v1.0.-rc1",
        "Geth/linux-amd64/go1.25",
        "/v1.0.0",
    ] {
        assert_eq!(
            verify(DAEMON_VERSION, client, 2, SDK_VERSION),
            Err(CompatibilityError::UnsupportedNodeInterface)
        );
    }
    let client = format!("paxd/v{}.0.0", matrix()[0].node_interface_major);
    assert_eq!(
        verify("unsupported", &client, 2, SDK_VERSION),
        Err(CompatibilityError::UnsupportedDaemon)
    );
    assert_eq!(
        verify(DAEMON_VERSION, &client, 2, "unsupported"),
        Err(CompatibilityError::UnsupportedSdk)
    );
}

#[test]
fn empty_receipt_keys_invalidate_an_already_ready_gate() {
    let mut gate = gate();
    let (config, server) = serve_node(
        1,
        vec![populated_keys(), json!({"keys": []}), populated_keys()],
    );
    let node = must(NodeRpc::connect(&config));
    let status = must(gate.readiness_gate(&node));
    assert_eq!(status.chain_id, 125);
    assert_eq!(status.generation, 1);
    assert_eq!(status.receipt_keys.len(), 1);
    assert_eq!(status.receipt_keys[0].public_key, receipt_key());
    assert_eq!(status.receipt_keys[0].first_height, 1);
    assert_eq!(status.receipt_keys[0].last_height, 100);
    assert_eq!(gate.guard_write(|| 7), Ok(7));
    assert!(gate.evidence_authority().is_ok());
    assert_eq!(
        gate.readiness_gate(&node),
        Err(GateError::NodeRpc(malformed_keys()))
    );
    assert_closed(&gate);
    assert_eq!(must(gate.readiness_gate(&node)).generation, 2);
    assert_eq!(gate.guard_write(|| 8), Ok(8));
    must(server.join());
}

#[test]
fn receipt_key_validation_still_refuses_invalid_nonempty_lists() {
    let valid = populated_keys();
    let mut zero_id = valid.clone();
    zero_id["keys"][0]["keyId"] = json!(format!("0x{}", hex::encode([0; 32])));
    let mut bad_range = valid.clone();
    bad_range["keys"][0]["firstHeight"] = json!(101);
    let mut weak_key = valid.clone();
    weak_key["keys"][0]["publicKey"] = json!(format!("0x{}", hex::encode([0; 32])));
    let mut duplicate = valid.clone();
    duplicate["keys"]
        .as_array_mut()
        .unwrap_or_else(|| panic!("keys array"))
        .push(valid["keys"][0].clone());
    let mut string_height = valid.clone();
    string_height["keys"][0]["firstHeight"] = json!("0x1");
    for result in [
        json!([]),
        zero_id,
        bad_range,
        weak_key,
        duplicate,
        string_height,
    ] {
        let (config, server) = serve_node(1, vec![result]);
        let node = must(NodeRpc::connect(&config));
        let mut gate = gate();
        assert_eq!(
            gate.readiness_gate(&node),
            Err(GateError::NodeRpc(malformed_keys()))
        );
        assert_closed(&gate);
        must(server.join());
    }
}
