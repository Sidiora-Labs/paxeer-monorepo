//! Repair record 22.33.1438, audit 22.30.80, finding `doc-gateway` (22998.1).
//!
//! Proving cases: `doc_gateway_node_root_and_native_balance_example`,
//! `doc_gateway_hosted_openrpc_is_a_separate_contract`, and
//! `doc_gateway_finality_requires_authenticated_block_commit`.
//! Attempt 1 failed before compilation because the hosted paxeer manifest was
//! absent; none of these cases executed. This continuation preserves those
//! cases and uses the restored workspace member. The required build-before-verify
//! order precludes an additional unchanged-tree baseline run.
//! The runner's private gate ledger records the exact built and verified
//! revision, commands, exit codes and logs. No out-of-path wiring is required
//! for the documentation proposal; workspace restoration belongs to 22.31.465.

use layerx_client::rpc::commit::verify_commit_material;
use layerx_client::rpc::receipt_commit::{verify_receipt_coverage, ReceiptCommitEvidence};
use layerx_client::sequencer::ReceiptKeyEntry;
use layerx_platform_gateway::evm;
use layerx_platform_gateway::http::{
    read_request, write_response, Client, Endpoint, OutboundRequest, OutgoingResponse,
};
use layerx_platform_gateway::{router_method, ReceiptKeys, SequencerAuthorization, ROUTER_METHODS};
use layerx_programs::chain_proof::{
    merkle::SimpleProof,
    validators::{TrustedValidator, ValidatorSet},
    ChainProofVerifier,
};
use layerx_proof::receipt::verify_sequencer_signature;
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};
use std::io::Write;
use std::net::TcpListener;
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

const README: &str = include_str!("../README.md");

fn json_examples() -> Vec<Value> {
    README
        .split("```json\n")
        .skip(1)
        .map(|section| {
            serde_json::from_str(section.split("```").next().expect("closed JSON fence"))
                .expect("executable README JSON")
        })
        .collect()
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("ASCII hex"), 16).expect("hex byte")
        })
        .collect()
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

// Only the node/consensus peer is a transcript. Both HTTP directions, TLS and
// outbound requests use the production gateway codecs and Client. No process,
// environment configuration or externally provisioned service is needed.
fn transcript(calls: Vec<(Value, Value)>) -> (Client, Endpoint, JoinHandle<()>) {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let certificate =
        rcgen::generate_simple_self_signed(vec!["localhost".into()]).expect("loopback certificate");
    let client = Client::without_identity(
        native_tls::Certificate::from_der(certificate.cert.der()).expect("peer trust"),
    );
    let tls = Arc::new(
        ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(
                vec![certificate.cert.der().clone()],
                rustls::pki_types::PrivatePkcs8KeyDer::from(
                    certificate.signing_key.serialize_der(),
                )
                .into(),
            )
            .expect("peer TLS"),
    );
    let listener = TcpListener::bind("127.0.0.1:0").expect("loopback transcript");
    let endpoint = Endpoint::parse(&format!(
        "https://localhost:{}/",
        listener.local_addr().expect("peer address").port()
    ))
    .expect("root endpoint");
    listener.set_nonblocking(true).expect("bounded accept");
    let join = thread::spawn(move || {
        for (call, result) in calls {
            let deadline = Instant::now() + Duration::from_secs(10);
            let tcp = loop {
                match listener.accept() {
                    Ok((tcp, _)) => break tcp,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing transcript request");
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            tcp.set_read_timeout(Some(Duration::from_secs(5)))
                .expect("read bound");
            tcp.set_write_timeout(Some(Duration::from_secs(5)))
                .expect("write bound");
            let mut stream = StreamOwned::new(
                ServerConnection::new(Arc::clone(&tls)).expect("TLS connection"),
                tcp,
            );
            let request = read_request(&mut stream, 1024 * 1024).expect("production request codec");
            assert_eq!(request.method, "POST");
            assert_eq!(request.path, "/", "node and consensus RPC use root");
            assert_eq!(request.headers["content-type"], "application/json");
            assert!(!request.headers.contains_key("authorization"));
            assert_eq!(
                serde_json::from_slice::<Value>(&request.body).expect("node request"),
                call
            );
            let response = OutgoingResponse {
                status: 200,
                content_type: "application/json".into(),
                headers: Vec::new(),
                body: serde_json::to_vec(
                    &json!({"jsonrpc":"2.0", "id":call["id"], "result":result}),
                )
                .expect("transcript envelope"),
                retry_after: None,
            };
            write_response(&mut stream, &response).expect("production response codec");
            stream.conn.send_close_notify();
            stream.flush().expect("TLS close");
        }
    });
    (client, endpoint, join)
}

fn exchange(client: &Client, endpoint: &Endpoint, call: &Value) -> Value {
    let body = serde_json::to_vec(call).expect("request JSON");
    let reply = client
        .request_unauthenticated(
            endpoint,
            &OutboundRequest {
                method: "POST",
                path: "/",
                idempotency: None,
                content_type: "application/json",
                body: &body,
            },
        )
        .expect("real gateway outbound client");
    assert_eq!(reply.status, 200);
    assert_eq!(reply.content_type, "application/json");
    let document: Value = serde_json::from_slice(&reply.body).expect("node response JSON");
    assert_eq!(document["jsonrpc"], "2.0");
    assert_eq!(document["id"], call["id"]);
    assert!(document.get("error").is_none());
    document["result"].clone()
}

#[test]
fn doc_gateway_node_root_and_native_balance_example() {
    let examples = json_examples();
    let calls: Vec<_> = examples
        .iter()
        .filter(|value| value.get("jsonrpc").is_some())
        .cloned()
        .collect();
    assert_eq!(calls.len(), 2, "network and native balance examples");
    assert_eq!(
        calls[0],
        json!({"jsonrpc":"2.0", "id":"network", "method":"eth_chainId", "params":[]})
    );
    let abi: Value = serde_json::from_str(include_str!("fixtures/paxeer-abi-vectors.json"))
        .expect("committed Go ABI producer vectors");
    let bank = abi["cases"]
        .as_array()
        .expect("ABI cases")
        .iter()
        .find(|case| case["name"] == "bank.all_balances")
        .expect("bank vector");
    let address = evm::parse_address(bank["in"]["acc"].as_str().expect("account"))
        .expect("native account address");
    let calldata = evm::calldata_address(evm::SELECTOR_BANK_ALL_BALANCES, &address);
    assert_eq!(hex(&calldata), bank["calldata"]);
    assert_eq!(
        calls[1],
        json!({"jsonrpc":"2.0", "id":"balances", "method":"eth_call", "params":[{
        "to":evm::address_hex(&evm::BANK_PRECOMPILE), "data":format!("0x{}", hex(&calldata))
    }, "latest"]})
    );
    let urls = examples
        .iter()
        .find(|value| value.is_array())
        .expect("relay URLs");
    for url in urls.as_array().expect("URL array") {
        assert!(Endpoint::parse(url.as_str().expect("URL"))
            .expect("canonical HTTPS")
            .base_path
            .is_empty());
    }
    let (client, endpoint, join) = transcript(vec![
        (calls[0].clone(), json!("0x7d")),
        (
            calls[1].clone(),
            json!(format!(
                "0x{}",
                bank["result"].as_str().expect("ABI output")
            )),
        ),
    ]);
    assert_eq!(exchange(&client, &endpoint, &calls[0]), "0x7d");
    let answer = exchange(&client, &endpoint, &calls[1]);
    let coins = evm::decode_coins(&unhex(answer.as_str().expect("ABI result")))
        .expect("production bank decoder");
    let expected = bank["out"]["response"].as_array().expect("expected coins");
    assert_eq!(coins.len(), expected.len());
    for ((denom, amount), expected) in coins.iter().zip(expected) {
        assert_eq!(denom, expected["denom"].as_str().expect("denom"));
        assert_eq!(evm::uint256_decimal(amount), expected["amount"]);
    }
    join.join().expect("node transcript assertions");
}

#[test]
fn doc_gateway_hosted_openrpc_is_a_separate_contract() {
    assert!(README.contains("Do not assume a public node implements `px_*`"));
    assert!(README.contains("`/rpc/schema` is not a discovery endpoint"));
    assert!(README.contains("are retired and return HTTP 404 at the gateway"));
    assert!(README.contains("There is no unified WebSocket"));
    let schema: Value =
        serde_json::from_str(include_str!("../openrpc.json")).expect("gateway-owned OpenRPC");
    let methods = schema["methods"].as_array().expect("hosted methods");
    assert_eq!(methods.len(), ROUTER_METHODS.len());
    for method in ROUTER_METHODS {
        assert!(router_method(method), "production router admits {method}");
        assert_eq!(
            methods
                .iter()
                .filter(|entry| entry["name"] == method)
                .count(),
            1
        );
    }
    for method in [
        "eth_chainId",
        "eth_call",
        "eth_sendRawTransaction",
        "pax_sequencerReceipt",
    ] {
        assert!(evm::relayable_method(method));
        assert!(README.contains(method));
    }
    for method in evm::NODE_SIGNING_METHODS
        .into_iter()
        .chain(evm::SUBSCRIPTION_METHODS)
    {
        assert!(!evm::relayable_method(method));
        assert!(README.contains(method));
    }
    assert!(!router_method("lx_sendActivity"));
    assert!(!evm::relayable_method("lx_sendActivity"));
    for (name, address) in [
        ("bank", evm::BANK_PRECOMPILE),
        ("addr", evm::ADDR_PRECOMPILE),
        ("ledger", evm::LEDGER_PRECOMPILE),
        ("pay", evm::PAY_PRECOMPILE),
        ("auth", evm::AUTH_PRECOMPILE),
        ("programs", evm::PROGRAMS_PRECOMPILE),
    ] {
        let vectors: Value = serde_json::from_str(include_str!("fixtures/paxeer-abi-vectors.json"))
            .expect("precompile producer vectors");
        assert_eq!(evm::address_hex(&address), vectors["precompiles"][name]);
        assert!(README.contains(&format!("0x{}", hex(&address[18..]))));
    }
}

#[test]
fn doc_gateway_finality_requires_authenticated_block_commit() {
    assert!(README.contains("No separate sequencer trust anchor"));
    assert!(README.contains("sequence-batch inclusion and coverage"));
    let recorded: Value =
        serde_json::from_str(include_str!("fixtures/repair_22_33_1335_committed.json"))
            .expect("recorded node, receipt and consensus evidence with provenance");
    let receipt_call = json!({"jsonrpc":"2.0", "id":"receipt", "method":"pax_sequencerReceipt", "params":[recorded["receipt"]["txHash"]]});
    let commit_call =
        json!({"jsonrpc":"2.0", "id":"commit", "method":"commit", "params":{"height":"10"}});
    let block_call =
        json!({"jsonrpc":"2.0", "id":"block", "method":"block", "params":{"height":"10"}});
    let (client, endpoint, join) = transcript(vec![
        (receipt_call.clone(), recorded["receipt"].clone()),
        (commit_call.clone(), recorded["commit"].clone()),
        (block_call.clone(), recorded["block"].clone()),
    ]);
    let receipt = exchange(&client, &endpoint, &receipt_call);
    assert_eq!(
        receipt["rung"], "receipted",
        "signed execution alone is not finality"
    );
    let key = &recorded["pinnedKey"];
    let entry = ReceiptKeyEntry {
        key_id: unhex(key["keyId"].as_str().expect("key ID"))
            .try_into()
            .expect("32 bytes"),
        public_key: unhex(key["publicKey"].as_str().expect("public key"))
            .try_into()
            .expect("32 bytes"),
        first_height: 10,
        last_height: 10,
    };
    let pins = [SequencerAuthorization::new(
        entry.key_id,
        entry.public_key,
        10,
        10,
    )];
    let bytes = unhex(receipt["receipt"].as_str().expect("signed receipt"));
    let verified = verify_sequencer_signature(&bytes, ReceiptKeys::HeightRanges(&pins))
        .expect("real receipt verifier");
    let commit = exchange(&client, &endpoint, &commit_call);
    let validator = &recorded["validators"]["validators"][0];
    let validator_key = layerx_platform_internal::base64::decode(
        validator["pub_key"]["value"]
            .as_str()
            .expect("validator key"),
    )
    .expect("base64 validator");
    let verifier = ChainProofVerifier {
        chain_id: "committed-fixture".into(),
        validators: ValidatorSet::new(vec![TrustedValidator::new(
            validator_key.try_into().expect("validator Ed25519 key"),
            validator["voting_power"]
                .as_str()
                .expect("power")
                .parse()
                .expect("integer power"),
        )])
        .expect("fixture's trusted consensus context"),
    };
    let head = verify_commit_material(&commit["signed_header"], &verifier)
        .expect("authenticate recorded block commit");
    assert_eq!(head.height(), 10);
    let block = exchange(&client, &endpoint, &block_call);
    let transactions: Vec<_> = block["block"]["data"]["txs"]
        .as_array()
        .expect("raw block transactions")
        .iter()
        .map(|tx| {
            layerx_platform_internal::base64::decode(tx.as_str().expect("transaction"))
                .expect("base64 transaction")
        })
        .collect();
    assert_eq!(transactions.len(), 2, "recorded block's exact Merkle shape");
    let leaf = |tx: &[u8]| -> [u8; 32] {
        Sha256::digest([&[0][..], Sha256::digest(tx).as_slice()].concat()).into()
    };
    let proof = SimpleProof {
        total: 2,
        index: 0,
        leaf_hash: leaf(&transactions[0]),
        aunts: vec![leaf(&transactions[1])],
    };
    let hash = unhex(receipt["txHash"].as_str().expect("transaction hash"))
        .try_into()
        .expect("32-byte hash");
    let activity_ids = [hash];
    let evidence = ReceiptCommitEvidence {
        verifier: &verifier,
        covering_height: 10,
        batch_transaction: &transactions[0],
        transaction_proof: &proof,
        activity_ids: &activity_ids,
    };
    verify_receipt_coverage(
        verified.protocol().expect("protocol receipt"),
        10,
        &head,
        &evidence,
        &[entry],
    )
    .expect("real consumer proves receipt covered by committed block");
    let mut bad_commit = commit["signed_header"].clone();
    bad_commit["commit"]["signatures"][0]["signature"] =
        json!(layerx_platform_internal::base64::encode(&[0; 64]));
    assert!(verify_commit_material(&bad_commit, &verifier).is_err());
    let mut bad_proof = proof.clone();
    bad_proof.aunts[0][0] ^= 1;
    let bad_evidence = ReceiptCommitEvidence {
        transaction_proof: &bad_proof,
        ..evidence
    };
    assert!(verify_receipt_coverage(
        verified.protocol().expect("protocol receipt"),
        10,
        &head,
        &bad_evidence,
        &[entry]
    )
    .is_err());
    let mut bad_receipt = bytes;
    *bad_receipt.last_mut().expect("signature") ^= 1;
    assert!(verify_sequencer_signature(&bad_receipt, ReceiptKeys::HeightRanges(&pins)).is_err());
    join.join().expect("evidence transport assertions");
}
