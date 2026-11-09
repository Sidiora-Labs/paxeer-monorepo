//! Proves: `NativeReadRoute::read` -> `daemon::serve_read_routes` HTTP -> `ProgramReads::execute.`
//!
//! Starts the repository's disposable sequencer and reads its generated localnode.env;
//! no node, daemon route, HTTP client or consumer is replaced. The daemon is stopped
//! and reopened between reads, and the same consumer must refuse while it is down.
//! Repairs layerx-`mcp/src/boundary.rs::AgentSurface::native_read` accepting JSON null,
//! arrays and scalars as evidence by decoding through `decode_native_read_result`.
//! Repairs `produce_transaction` requesting a pending nonce from the proposer by
//! using layerx-`client/src/sequencer.rs::SequencerEndpoint::latest_nonce.`
//! Repairs `AgentSurface::native_read` losing the unavailable history method's
//! identity when the daemon answers HTTP 503: the consumer receives a typed
//! refusal naming history.list and `px_getHistory` instead of a transport status.
//!
//! The proof belongs to the consumer crate so layerx-agentd does not depend on
//! layerx-mcp. The shared HTTP implementation is also imported by main.rs through
//! `proof_serve_errors`, including its listener, parser, dispatcher and response writer.
//! Its daemon HTTP assertions remain in the separate agentd `proof_serve_errors`
//! target and are not included in this disposable-node proof.
//! Out-of-scope behavior: `src/read/native.rs::NativeReadRoute::receipt` labels even
//! committed receipts "receipted". Assert that current behavior below; the expected
//! eventual behavior is to preserve the authenticated receipt's strongest rung.
//! layerx-`client/src/sequencer.rs::SequencerEndpoint::history` calls `px_getHistory`,
//! which belongs to `services/platform/hosted/gateway/src/history.rs.` The local
//! node does not serve it, so history.list must remain explicitly unavailable.

use layerx_mcp_test_support::option_assertion;
use layerx_mcp_test_support::result_assertion;

#[path = "support/native_reads.rs"]
mod native_reads;

use std::net::{SocketAddr, TcpListener};
use std::process::Command;
use std::thread;
use std::time::{Duration, Instant};

use ed25519_dalek::SigningKey;
use layerx_agentd::node_rpc::NodeRpcConfig;
use layerx_agentd::read::{LayerxdProgramBalanceReader, ProgramAuthority, ProgramBalanceReadRoute};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_client::sequencer::{SequencerBlock, SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_mcp::boundary::{
    decode_native_read_result, AgentSurface, BoundaryRefusal, ProgramReads, ToolBoundary,
};
use layerx_mcp::server::catalogue;
use layerx_programs::chain_proof::validators::TrustedValidator;
use layerx_programs::{hex, ProtocolDeploymentVerifier, Registry};
use openssl::asn1::Asn1Time;
use openssl::hash::MessageDigest;
use openssl::pkey::PKey;
use openssl::x509::extension::BasicConstraints;
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};
use zeroize::Zeroizing;

use native_reads::disposable_node::LocalNode;

const BEARER: &str = "native-reads-proof-agent-credential";

#[track_caller]
fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("native reads proof: {error:?}"))
}

fn endpoint(config: &NodeRpcConfig) -> SequencerEndpoint {
    must(SequencerEndpoint::new(&config.url, &[]))
}

fn producer_read<T>(
    result: Result<T, SequencerError>,
    url: &str,
    method: &str,
    params: &Value,
    expected_fields: &[&str],
) -> Result<T, SequencerError> {
    result.inspect_err(|error| {
        let request = json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params});
        let answer = result_assertion::required(
            Command::new("curl")
                .args([
                    "--silent",
                    "--show-error",
                    "--max-time",
                    "5",
                    "-H",
                    "Content-Type: application/json",
                    "--data",
                ])
                .arg(request.to_string())
                .arg(url)
                .output(),
            "capture refused producer method's raw JSON answer",
        );
        eprintln!(
            "{method} refused: {error:?}; raw JSON answer: {}",
            String::from_utf8_lossy(&answer.stdout)
        );
        eprintln!(
            "raw answer capture status: {}; {}",
            answer.status,
            String::from_utf8_lossy(&answer.stderr)
        );
        eprintln!("layerx-client decoder expects fields: {expected_fields:?}");
        if let Ok(value) = serde_json::from_slice::<Value>(&answer.stdout) {
            let result = &value["result"];
            let fields = if method == "px_getReceiptKeys" {
                &result["keys"][0]
            } else {
                result
            };
            if let Some(object) = fields.as_object() {
                let actual: std::collections::BTreeSet<_> =
                    object.keys().map(String::as_str).collect();
                let expected: std::collections::BTreeSet<_> =
                    expected_fields.iter().copied().collect();
                eprintln!(
                    "{method} missing fields: {:?}; additional fields: {:?}",
                    expected.difference(&actual).collect::<Vec<_>>(),
                    actual.difference(&expected).collect::<Vec<_>>()
                );
            }
        }
    })
}

fn authority_ca() -> Vec<u8> {
    let key = must(PKey::generate_ed25519());
    let mut name = must(X509NameBuilder::new());
    must(name.append_entry_by_text("CN", "native-reads-proof"));
    let name = name.build();
    let mut certificate = must(X509::builder());
    must(certificate.set_version(2));
    must(certificate.set_subject_name(&name));
    must(certificate.set_issuer_name(&name));
    must(certificate.set_pubkey(&key));
    let not_before = must(Asn1Time::days_from_now(0));
    let not_after = must(Asn1Time::days_from_now(1));
    must(certificate.set_not_before(not_before.as_ref()));
    must(certificate.set_not_after(not_after.as_ref()));
    must(certificate.append_extension(must(BasicConstraints::new().critical().ca().build())));
    must(certificate.sign(&key, MessageDigest::null()));
    must(certificate.build().to_der())
}

struct Service {
    address: SocketAddr,
    _pool: native_reads::HttpPool,
}

impl Service {
    fn start(config: &NodeRpcConfig, address: SocketAddr, node: &LocalNode) -> Self {
        // The production read server also owns the program route. Construct its
        // real reader; native reads never contact this separate program authority.
        let ca = authority_ca();
        let validator = SigningKey::from_bytes(&[21; 32]).verifying_key().to_bytes();
        let verifier = must(ProtocolDeploymentVerifier::from_chain_trust(
            format!("paxeer-{}", config.chain_id),
            vec![TrustedValidator::new(validator, 1)],
            1_000,
        ));
        let reader = must(LayerxdProgramBalanceReader::connect(
            &config.url,
            "native-reads-node-credential".to_owned(),
            ProgramAuthority {
                endpoint: "https://localhost:1",
                authorization: "native-reads-authority-credential".to_owned(),
                replica_id: [21; 32],
                ca_der: &ca,
            },
            verifier,
            Registry::new(),
        ));
        let listener = must(TcpListener::bind(address));
        let address = must(listener.local_addr());
        let pool = must(native_reads::serve_read_routes(
            listener,
            BEARER.to_owned(),
            ProgramBalanceReadRoute::new(reader),
            Some(node.native_read_route(&config.url, config.chain_id)),
            config.clone(),
            ca,
            endpoint(config),
        ));
        must(pool.check());
        Self {
            address,
            _pool: pool,
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

fn execute(reads: &mut ProgramReads, address: SocketAddr, name: &str, arguments: &Value) -> Value {
    let tool = *option_assertion::required(
        catalogue().iter().find(|tool| tool.name == name),
        "real catalogue tool",
    );
    let result = reads.execute(tool, arguments);
    if let Err(error) = &result {
        let (kind, selector) = match name {
            "receipt.get" => ("receipt", "activity_id"),
            "proof.get" => ("proof", "activity_id"),
            "checkpoint.get" => ("checkpoint", "sequence"),
            "availability.get" => ("availability", "batch"),
            _ => panic!("unexpected proof tool: {name}"),
        };
        let path = format!(
            "/v1/reads/{kind}/{}",
            option_assertion::required(arguments[selector].as_str(), "proof read selector")
        );
        let answer = result_assertion::required(
            Command::new("curl")
                .args([
                    "--silent",
                    "--show-error",
                    "--include",
                    "--max-time",
                    "5",
                    "-H",
                ])
                .arg(format!("Authorization: Bearer {BEARER}"))
                .arg(format!("http://{address}{path}"))
                .output(),
            "capture unexpectedly refused daemon read's status and body",
        );
        eprintln!(
            "{name} {path} refused: {error:?}; raw daemon status and body: {}",
            String::from_utf8_lossy(&answer.stdout)
        );
        eprintln!(
            "daemon answer capture status: {}; {}",
            answer.status,
            String::from_utf8_lossy(&answer.stderr)
        );
    }
    result.unwrap_or_else(|error| panic!("{name} {arguments} unexpectedly refused: {error:?}"))
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

fn observe(
    reads: &mut ProgramReads,
    address: SocketAddr,
    node_url: &str,
    hash: [u8; 32],
    block: &SequencerBlock,
) -> Vec<Value> {
    let transaction = hex::encode(&hash);
    let receipt = execute(
        reads,
        address,
        "receipt.get",
        &json!({"activity_id": transaction}),
    );
    assert_eq!(receipt["transaction_hash"], transaction);
    assert_eq!(receipt["height"], block.number);
    assert_eq!(receipt["rung"], "receipted");
    assert!(receipt["receipt"]
        .as_str()
        .is_some_and(|value| !value.is_empty()));
    assert_receipt_block(&receipt, node_url, block);
    let proof = observe_receipt_proof(reads, address, &transaction, &receipt, hash, block);
    let checkpoint = execute(
        reads,
        address,
        "checkpoint.get",
        &json!({"sequence": block.number.to_string()}),
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
    assert_eq!(checkpoint["verified"], true);
    assert_eq!(checkpoint["rung"], "committed");
    let availability = execute(
        reads,
        address,
        "availability.get",
        &json!({"batch": block.number.to_string()}),
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
    let history_tool = *option_assertion::required(
        catalogue().iter().find(|tool| tool.name == "history.list"),
        "real history catalogue tool",
    );
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
    let (node, url, key) = LocalNode::start();
    let client = must(SequencerEndpoint::new(&url, &[]));
    let config = NodeRpcConfig {
        chain_id: result_assertion::required(
            producer_read(client.chain_id(), &url, "eth_chainId", &json!([]), &[]),
            "producer eth_chainId must decode as a canonical chain-id quantity",
        ),
        url,
        deadline: Duration::from_secs(5),
    };
    let (hash, height) = produce_transaction(&client, config.chain_id, &key);
    let block = option_assertion::required(
        result_assertion::required(
            producer_read(
                client.block_by_number(height),
                &config.url,
                "eth_getBlockByNumber",
                &json!([format!("{height:#x}"), true]),
                &[
                    "number",
                    "hash",
                    "parentHash",
                    "timestamp",
                    "transactionsRoot",
                    "receiptsRoot",
                    "transactions",
                ],
            ),
            "producer eth_getBlockByNumber must decode the committed block and its transactions",
        ),
        "committed producer block",
    );
    let keys = result_assertion::required(
        producer_read(
            client.receipt_keys(),
            &config.url,
            "px_getReceiptKeys",
            &json!([]),
            &["keyId", "publicKey", "firstHeight", "lastHeight"],
        ),
        "producer px_getReceiptKeys must decode the keys object and its numeric height ranges",
    );
    let Lookup::Verified(verified) = must(lookup_authenticated(
        &client,
        hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )) else {
        panic!("real sequencer must emit a signed receipt for the produced transaction");
    };

    let service = Service::start(
        &config,
        result_assertion::required("127.0.0.1:0".parse(), "loopback"),
        &node,
    );
    let address = service.address;
    let surface = service.surface(BEARER);
    let wrong_credential = service.surface("wrong-native-reads-agent-credential");
    let mut reads = ProgramReads::new(service.surface(BEARER));
    let first = observe(&mut reads, address, &config.url, hash, &block);
    assert_eq!(first[0]["receipt"], hex::encode(verified.canonical_bytes()));
    assert_eq!(
        first[0]["receipt_key"]["key_id"],
        hex::encode(&verified.execution().key_id)
    );
    assert_refused_native_inputs(&surface, &wrong_credential, height, &first);
    let path = format!("/v1/reads/checkpoint/{height}");

    drop(service);
    assert!(
        matches!(
            surface.native_read(&path),
            Err(BoundaryRefusal::Unavailable(_))
        ),
        "a stopped server must never be answered from a cache"
    );
    let reopened = Service::start(&config, address, &node);
    let second = observe(&mut reads, address, &config.url, hash, &block);
    assert_eq!(first, second);
    assert_eq!(must(surface.native_read(&path)), second[2]);
    drop(reopened);
}

fn assert_receipt_block(receipt: &Value, node_url: &str, block: &SequencerBlock) {
    let height =
        option_assertion::required(receipt["height"].as_u64(), "authenticated receipt height");
    let request = json!({
        "jsonrpc": "2.0", "id": 1, "method": "eth_getBlockByNumber",
        "params": [format!("{height:#x}"), true],
    });
    let answer = result_assertion::required(
        Command::new("curl")
            .args([
                "--silent",
                "--show-error",
                "--max-time",
                "5",
                "-H",
                "Content-Type: application/json",
                "--data",
            ])
            .arg(request.to_string())
            .arg(node_url)
            .output(),
        "read the receipt height directly from the disposable node",
    );
    eprintln!(
        "receipt height {height} eth_getBlockByNumber at {node_url}: {}; status: {}; {}",
        String::from_utf8_lossy(&answer.stdout),
        answer.status,
        String::from_utf8_lossy(&answer.stderr)
    );
    assert!(answer.status.success(), "direct receipt-height block read");
    let answer: Value = must(serde_json::from_slice(&answer.stdout));
    assert!(
        answer.get("error").is_none(),
        "receipt-height block RPC error: {answer}"
    );
    assert_eq!(
        answer["result"]["hash"],
        format!("0x{}", hex::encode(&block.hash)),
        "the node serving the route must retain the authenticated receipt's block"
    );
}

fn observe_receipt_proof(
    reads: &mut ProgramReads,
    address: SocketAddr,
    transaction: &str,
    receipt: &Value,
    hash: [u8; 32],
    block: &SequencerBlock,
) -> Value {
    let proof = execute(
        reads,
        address,
        "proof.get",
        &json!({"activity_id": transaction}),
    );
    assert_eq!(
        proof["signedReceipt"],
        format!(
            "0x{}",
            option_assertion::required(receipt["receipt"].as_str(), "signed receipt")
        )
    );
    let receipt_key = &receipt["receipt_key"];
    assert_eq!(
        proof["receiptKey"],
        json!({
            "keyId": format!("0x{}", option_assertion::required(receipt_key["key_id"].as_str(), "receipt key id")),
            "publicKey": format!("0x{}", option_assertion::required(receipt_key["public_key"].as_str(), "receipt public key")),
            "firstHeight": option_assertion::required(receipt_key["first_height"].as_u64(), "receipt key first height").to_string(),
            "lastHeight": option_assertion::required(receipt_key["last_height"].as_u64(), "receipt key last height").to_string(),
        })
    );
    assert_eq!(proof["kind"], "receipt");
    assert_eq!(proof["format"], "layerx-receipt-proof-v2");
    assert_eq!(proof["verificationLevel"], "committed");
    assert_eq!(proof["rung"], "committed");
    assert_eq!(proof["height"], block.number);
    assert_eq!(proof["commitment_height"], block.number + 1);
    assert_eq!(proof["transaction_hash"], transaction);
    let raw = option_assertion::required(
        block
            .transactions
            .iter()
            .find(|raw| transaction_hash(raw) == hash),
        "the committed block contains the produced transaction",
    );
    assert_eq!(proof["rawTransaction"], format!("0x{}", hex::encode(raw)));
    for field in ["blockHeader", "receiptInclusion", "commit", "validatorSet"] {
        let bytes = proof[field]
            .as_str()
            .and_then(|value| value.strip_prefix("0x"))
            .unwrap_or_else(|| panic!("committed proof must carry {field}"));
        assert!(!must(hex::decode(bytes)).is_empty(), "{field}");
    }
    proof
}

fn assert_refused_native_inputs(
    surface: &AgentSurface,
    wrong_credential: &AgentSurface,
    height: u64,
    first: &[Value],
) {
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
}
