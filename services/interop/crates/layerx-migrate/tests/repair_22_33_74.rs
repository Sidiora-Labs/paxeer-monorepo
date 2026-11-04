//! Task record 22.33.74, audit 22.30.109: migration-context-docs is proven by
//! migration_context_docs_key_id_and_native_payment_exact,
//! migration_context_docs_mapping_v2_checks_both_directions, and
//! migration_context_docs_funded_observation_is_not_network_credit.
//! Baseline: this target was absent at revision
//! 84b6cf09552744359c78f419900ba8fcd9bbeaf5; no pre-change failures were measured.
//! The explicit implementation/build/hf-done order prohibits the conflicting
//! request to run verification first. Qualification revision, command, exit and
//! private evidence location are recorded by hf-build/hf-done, not invented here.
//! No proposal step requires out-of-scope wiring. Production code is unchanged.
//! Peers replay the committed Go receipt vector or serve bounded loopback HTTPS;
//! the production receipt verifier, migration descriptor, gateway store, mapping
//! verifier, source codec and funded gateway client execute in process.

use std::fs::{self, OpenOptions};
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::Duration;

use base64::{engine::general_purpose::STANDARD, Engine as _};
use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite, PinnedSpec, SpecVersion};
use layerx_interop_gateway::evidence::{SequencerEvidence, SettlementRung};
use layerx_interop_gateway::gateway::{
    SequencerSettlementError, TranslationKind, TranslationRequest, TranslationStatus,
};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_migrate::ethereum::EthereumAssetClaim;
use layerx_migrate::gateway_client::{GatewayClient, GatewayClientConfig};
use layerx_migrate::mapping_v2::{PaxeerBindingConfigV2, PaxeerBindingVerifierV2};
use layerx_migrate::{
    migration_adapter_descriptor, MigrationError, RpcEndpointConfig, RpcQuorumConfig,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use layerx_proof::sequencer_logs::SequencerLog;
use rustls::pki_types::{CertificateDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use sha3::{Digest as _, Keccak256};

const OPERATIONS: &str = include_str!("../OPERATIONS.md");
const CERT: &[u8] = include_bytes!("fixtures/repair_22_31_163/tls-cert.der");
const KEY: &[u8] = include_bytes!("fixtures/repair_22_31_163/tls-key.der");
static NEXT: AtomicU64 = AtomicU64::new(0);

fn docs_contain(text: &str) {
    assert!(
        OPERATIONS
            .split_whitespace()
            .collect::<Vec<_>>()
            .join(" ")
            .contains(text),
        "operations contract missing: {text}"
    );
}

fn recorded_payment() -> Value {
    let fixture: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))
    .unwrap();
    assert_eq!(
        fixture["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    fixture["cases"]
        .as_array()
        .unwrap()
        .iter()
        .find(|case| case["id"] == "V1-layerxledger-pay-exact")
        .unwrap()
        .clone()
}

fn bytes(value: &Value) -> Vec<u8> {
    let text = value.as_str().unwrap().strip_prefix("0x").unwrap();
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|i| u8::from_str_radix(&text[i..i + 2], 16).unwrap())
        .collect()
}
fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    bytes(value).try_into().unwrap()
}
fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn trace() -> TraceId {
    TraceId::mint([0xaa; 16])
}

#[test]
fn migration_context_docs_key_id_and_native_payment_exact() {
    docs_contain("signed `contextHash` (`context_hash` in Rust) is the producer's `keyId`");
    docs_contain("Never substitute an external-claim digest for `keyId`");
    docs_contain("`[event signature, paymentReference, from, to]`");
    docs_contain("`[assetId, amount]`");
    let case = recorded_payment();
    let pin = &case["pinnedKey"];
    let key_id = fixed(&pin["keyId"]);
    let public_key = fixed(&pin["publicKey"]);
    let first = pin["firstHeight"].as_u64().unwrap();
    let last = pin["lastHeight"].as_u64().unwrap();
    let keys = [SequencerAuthorization::new(key_id, public_key, first, last)];
    let signed = bytes(&case["signedReceipt"]);
    let receipt = verify_sequencer_signature(&signed, ReceiptKeys::HeightRanges(&keys)).unwrap();
    assert_eq!(receipt.protocol().unwrap().context_hash(), key_id);
    let log = &case["receiptJson"]["logs"][0];
    let topics: Vec<[u8; 32]> = log["topics"]
        .as_array()
        .unwrap()
        .iter()
        .map(fixed)
        .collect();
    assert_eq!(
        topics[0],
        <[u8; 32]>::from(Keccak256::digest(
            b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"
        ))
    );
    let raw = bytes(&case["rawTransaction"]);
    let offsets: Vec<_> = raw
        .windows(4)
        .enumerate()
        .filter_map(|(i, b)| (b == [0x78, 0xea, 0x11, 0xdb]).then_some(i))
        .collect();
    assert_eq!(offsets.len(), 1);
    let calldata = &raw[offsets[0] + 4..];
    for (topic, word) in [(1, 5), (2, 0), (3, 1)] {
        assert_eq!(
            topics[topic].as_slice(),
            &calldata[word * 32..(word + 1) * 32]
        );
    }
    assert_ne!(topics[1], key_id);
    for bad_pin in [
        SequencerAuthorization::new(topics[1], public_key, first, last),
        SequencerAuthorization::new(key_id, [0x42; 32], first, last),
        SequencerAuthorization::new(key_id, public_key, last + 1, last + 2),
    ] {
        assert!(
            verify_sequencer_signature(&signed, ReceiptKeys::HeightRanges(&[bad_pin])).is_err()
        );
    }
    let evidence = SequencerEvidence {
        transaction_hash: fixed(&case["receiptJson"]["txHash"]),
        signed_receipt: signed,
        logs: vec![SequencerLog::new(
            fixed(&log["address"]),
            topics.clone(),
            bytes(&log["data"]),
            0,
        )],
        rung: SettlementRung::Receipted,
    };
    let mut gateway = GatewayCore::new();
    let descriptor = migration_adapter_descriptor(
        PinnedSpec::new(
            AdapterId::new("migration-spec").unwrap(),
            SpecVersion::parse("1.0.0").unwrap(),
            [1; 32],
        )
        .unwrap(),
        ConformanceSuite::new(AdapterId::new("migration-conformance").unwrap(), 1, [2; 32])
            .unwrap(),
    )
    .unwrap();
    let adapter = descriptor.id().clone();
    gateway.register_adapter(descriptor, &trace(), 1).unwrap();
    let principal = PrincipalId::new("migration-operator".to_owned()).unwrap();
    // Same signing context and unchanged signed evidence: only the reference
    // identifies this payment. The account topics must leave their rows pending.
    for index in [2, 3, 1] {
        let key = topics[index];
        let request = TranslationRequest::new(
            adapter.clone(),
            TranslationKind::StateChanging,
            key,
            [3; 32],
        )
        .unwrap();
        assert_eq!(
            gateway
                .begin_translation(&principal, &request, &trace(), 2)
                .unwrap(),
            TranslationStatus::Pending
        );
        let result =
            gateway.settle_with_sequencer_evidence(&principal, key, &evidence, &keys, &trace(), 3);
        if index == 1 {
            assert_eq!(
                result.unwrap(),
                TranslationStatus::ReceiptVerified {
                    receipt_digest: fixed(&case["receiptDigest"])
                }
            );
        } else {
            assert_eq!(
                result.unwrap_err().into_error(),
                SequencerSettlementError::PaymentUnmatched
            );
            assert_eq!(
                gateway.translation(&principal, key),
                Some(TranslationStatus::Pending)
            );
        }
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "migration-docs-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new().mode(0o700).create(&path).unwrap();
        Self(fs::canonicalize(path).unwrap())
    }
    fn write(&self, name: &str, data: &[u8]) -> PathBuf {
        let path = self.0.join(name);
        OpenOptions::new()
            .create_new(true)
            .write(true)
            .mode(0o600)
            .open(&path)
            .unwrap()
            .write_all(data)
            .unwrap();
        path
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

struct Peer {
    address: SocketAddr,
    stopping: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl Peer {
    fn start(handler: impl Fn(&str, Value) -> Value + Send + 'static) -> Self {
        let tls = Arc::new(
            ServerConfig::builder()
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(CERT.to_vec())],
                    PrivatePkcs8KeyDer::from(KEY.to_vec()).into(),
                )
                .unwrap(),
        );
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        let address = listener.local_addr().unwrap();
        let stopping = Arc::new(AtomicBool::new(false));
        let stop = stopping.clone();
        let worker = thread::spawn(move || {
            for socket in listener.incoming() {
                let socket = socket.unwrap();
                if stop.load(Ordering::Acquire) {
                    break;
                }
                socket
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                socket
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut stream =
                    StreamOwned::new(ServerConnection::new(tls.clone()).unwrap(), socket);
                let mut header = Vec::new();
                while !header.ends_with(b"\r\n\r\n") {
                    assert!(header.len() < 32768);
                    let mut byte = [0];
                    stream.read_exact(&mut byte).unwrap();
                    header.push(byte[0]);
                }
                let header = String::from_utf8(header).unwrap();
                assert!(header
                    .lines()
                    .any(|line| line
                        .eq_ignore_ascii_case("Authorization: Bearer public-docs-fixture")));
                let length: usize = header
                    .lines()
                    .find_map(|line| {
                        let (name, value) = line.split_once(':')?;
                        name.eq_ignore_ascii_case("Content-Length")
                            .then(|| value.trim().parse().unwrap())
                    })
                    .unwrap();
                assert!(length <= 32768);
                let mut body = vec![0; length];
                stream.read_exact(&mut body).unwrap();
                let response =
                    serde_json::to_vec(&handler(&header, serde_json::from_slice(&body).unwrap()))
                        .unwrap();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).unwrap();
                stream.write_all(&response).unwrap();
                stream.flush().unwrap();
            }
        });
        Self {
            address,
            stopping,
            worker: Some(worker),
        }
    }
    fn url(&self) -> String {
        format!("https://{}/", self.address)
    }
}
impl Drop for Peer {
    fn drop(&mut self) {
        self.stopping.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        let result = self.worker.take().unwrap().join();
        if !thread::panicking() {
            result.unwrap();
        }
    }
}

#[test]
fn migration_context_docs_mapping_v2_checks_both_directions() {
    docs_contain("`PaxeerBindingVerifierV2` to read both directions of the binding");
    docs_contain("`layerx_receipt: false`");
    let directory = Directory::new();
    let cert = directory.write("ca.der", CERT);
    let token = directory.write("token", b"public-docs-fixture");
    let identity = [0x21; 32];
    let address = [0x31; 20];
    let reverse_mismatch = Arc::new(AtomicBool::new(false));
    let peers: Vec<_> = (0..2).map(|_| {
        let mismatch = reverse_mismatch.clone();
        Peer::start(move |_, request| {
            let result = match request["method"].as_str().unwrap() {
                "eth_chainId" => json!("0x7d"),
                "eth_getBlockByNumber" => if request["params"][0] == "earliest" {
                    json!({"number":"0x0", "hash":format!("0x{}", hex(&[0x41;32])), "parentHash":format!("0x{}", hex(&[0;32]))})
                } else {
                    assert!(matches!(request["params"][0].as_str().unwrap(), "finalized" | "0x10"));
                    json!({"number":"0x10", "hash":format!("0x{}", hex(&[0x51;32])), "parentHash":format!("0x{}", hex(&[0x61;32]))})
                },
                "eth_call" => {
                    assert_eq!(request["params"][0]["to"], "0x0000000000000000000000000000000000001004");
                    assert_eq!(request["params"][1], "0x10");
                    let input = bytes(&request["params"][0]["data"]);
                    let result = if input[..4] == [0xc4, 0x13, 0xba, 0xd3] {
                        assert_eq!(input[4..], identity);
                        [vec![0;12], address.to_vec()].concat()
                    } else {
                        assert_eq!(input[..4], [0xd3, 0x60, 0x0b, 0x24]);
                        assert_eq!(input[4..], [vec![0;12], address.to_vec()].concat());
                        let returned = if mismatch.load(Ordering::Acquire) { [0x22;32] } else { identity };
                        let did = format!("did:layerx:{}", hex(&returned));
                        let mut output = returned.to_vec();
                        output.extend_from_slice(&[0;31]); output.push(64);
                        output.extend_from_slice(&[0;31]); output.push(u8::try_from(did.len()).unwrap());
                        output.extend_from_slice(did.as_bytes()); output.resize(96 + did.len().div_ceil(32) * 32, 0); output
                    };
                    json!(format!("0x{}", hex(&result)))
                },
                other => panic!("unexpected RPC {other}"),
            };
            json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
        })
    }).collect();
    let config = PaxeerBindingConfigV2 {
        chain_id: 125,
        genesis_hash: [0x41; 32],
        rpc: RpcQuorumConfig {
            endpoints: peers
                .iter()
                .enumerate()
                .map(|(i, peer)| RpcEndpointConfig {
                    url: peer.url(),
                    ca_certificate_der: cert.clone(),
                    bearer_token_file: token.clone(),
                    independent_backend: format!("binding-peer-{i}"),
                })
                .collect(),
            quorum: 2,
            connect_timeout_ms: 1000,
            request_timeout_ms: 3000,
            maximum_response_bytes: 32768,
        },
    };
    let verifier = PaxeerBindingVerifierV2::new(config.clone()).unwrap();
    let binding = verifier.verify_identity(identity).unwrap();
    assert_eq!(binding.identity(), identity);
    assert_eq!(binding.evm_address(), address);
    assert_eq!(binding.block_number(), 16);
    assert_eq!(binding.block_hash(), [0x51; 32]);
    reverse_mismatch.store(true, Ordering::Release);
    assert_eq!(
        verifier.verify_identity(identity),
        Err(MigrationError::EvidenceMismatch)
    );
    let mut wrong_network = config;
    wrong_network.chain_id = 1;
    assert!(matches!(
        PaxeerBindingVerifierV2::new(wrong_network),
        Err(MigrationError::InvalidNetwork)
    ));
}

#[test]
fn migration_context_docs_funded_observation_is_not_network_credit() {
    docs_contain("even a producer state of `done` is reported as `producer-observed`");
    docs_contain("`layerx_credit_verified: false`");
    let directory = Directory::new();
    let cert = directory.write("ca.der", CERT);
    let authorization = directory.write("authorization", b"Bearer public-docs-fixture");
    // The real source encoder supplies the envelope sent over the real client.
    let evidence = EthereumAssetClaim {
        chain_id: 1,
        transaction_hash: [1; 32],
        source: [2; 20],
        source_asset: [3; 32],
        source_amount: 100,
        custody_reference: [4; 32],
        layerx_asset: [5; 32],
        layerx_amount: 10,
        destination: [6; 32],
    }
    .evidence()
    .unwrap();
    let encoded = STANDARD.encode(evidence.canonical());
    let digest = evidence.digest();
    let order = [7; 32];
    let peer = Peer::start(move |header, request| {
        assert!(header.starts_with("POST /v2/migration/assets HTTP/1.1\r\n"));
        assert_eq!(
            request,
            json!({"order_digest":order, "chain":"ethereum", "source_evidence":encoded})
        );
        let mode = header
            .lines()
            .find_map(|line| line.strip_prefix("Idempotency-Key: "))
            .unwrap();
        let state = match mode {
            "bad-order" | "bad-evidence" | "claims-credit" => "done",
            state => state,
        };
        json!({"ok":true, "operation":"migration.docs.fixture", "trace":trace().as_str(), "result":{
            "source_settlement":{"version":"layerx-migration-source-v2", "order_digest":if mode == "bad-order" {[8;32]} else {order},
                "state":state, "source_evidence_digest":if mode == "bad-evidence" {[9;32]} else {digest},
                "source_claim_id":if state == "source_pending" {None} else {Some([10;32])}},
            "provenance":"external-custody", "custody_label":"External custody: this independent market maker controls the off-platform funds and payout.",
            "layerx_receipt":mode == "claims-credit"}})
    });
    let client = GatewayClient::new(&GatewayClientConfig {
        endpoint: peer.url(),
        ca_certificate_der: cert,
        customer_authorization_file: authorization,
        connect_timeout_ms: 1000,
        request_timeout_ms: 3000,
        maximum_response_bytes: 32768,
    })
    .unwrap();
    for state in [
        "source_pending",
        "source_settled",
        "layerx_pending",
        "layerx_refused",
        "done",
    ] {
        let result = client
            .migrate_asset(order, "ethereum", &evidence, state, &trace())
            .unwrap();
        let observed = serde_json::to_value(result).unwrap();
        assert_eq!(observed["state"], "producer-observed");
        assert_eq!(observed["producer_state"], state);
        assert_eq!(observed["provenance"], "external-custody");
        assert_eq!(observed["layerx_receipt"], false);
        assert_eq!(observed["layerx_credit_verified"], false);
        assert_eq!(observed["source_evidence_digest"], json!(digest));
    }
    for mode in ["bad-order", "bad-evidence"] {
        assert!(matches!(
            client.migrate_asset(order, "ethereum", &evidence, mode, &trace()),
            Err(MigrationError::EvidenceMismatch)
        ));
    }
    assert!(matches!(
        client.migrate_asset(order, "ethereum", &evidence, "claims-credit", &trace()),
        Err(MigrationError::RpcResponseMismatch)
    ));
}
