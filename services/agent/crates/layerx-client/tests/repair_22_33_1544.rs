//! Retirement regression proof; finding mapping and baseline limits are in
//! src/lni/repair_22_33_1544.kvx. No retired protocol module is compiled.
use layerx_client::frame::framing::{read_frame, write_frame};
use layerx_client::frame::transport::{FrameViolation, TransportError};
use layerx_client::rpc::commit::{acquire_committed_header, decode_validators, CommitError};
use layerx_client::rpc::receipt::{
    lookup_authenticated, Lookup, LookupContext, ReceiptError, Rung,
};
use layerx_client::sequencer::decode_receipt_keys;
use layerx_programs_registry::chain_proof::ChainProofVerifier;
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use serde_json::{json, Value};
use std::io::ErrorKind;
use std::net::{TcpListener, TcpStream};
use std::path::Path;
use std::time::Duration;

#[path = "support/gateway_22_33_1546.rs"]
mod transport;
#[path = "support/witness_22_33_1546.rs"]
mod witness;
use transport::{must, Server};

fn retired(file: &str) {
    let path = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("src/lni")
        .join(file);
    let error = std::fs::symlink_metadata(&path)
        .expect_err("retired protocol source must remain absent, including symlinks");
    assert_eq!(error.kind(), ErrorKind::NotFound, "{}", path.display());
    let library = include_str!("../src/lib.rs");
    assert!(!library.contains("pub mod lni"));
    assert!(!library.contains("pub use lni"));
    assert!(library.lines().any(|line| line == "pub mod frame;"));
    assert!(library.lines().any(|line| line == "pub mod rpc;"));
}

fn receipt_case() -> Value {
    let corpus: Value = must(serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    assert_eq!(
        corpus["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    corpus["cases"][0].clone()
}

fn unhex(value: &Value) -> Vec<u8> {
    let digits = value.as_str().expect("producer hex string");
    let digits = digits.strip_prefix("0x").unwrap_or(digits);
    assert_eq!(digits.len() % 2, 0);
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
        .collect()
}

fn frame_crossing(payload: Vec<u8>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let address = must(listener.local_addr());
    let expected = payload.clone();
    let maximum = payload.len();
    let worker = std::thread::spawn(move || {
        let (mut stream, _) = must(listener.accept());
        must(stream.set_read_timeout(Some(Duration::from_secs(5))));
        must(stream.set_write_timeout(Some(Duration::from_secs(5))));
        let received = must(read_frame(&mut stream, maximum));
        assert_eq!(received, expected);
        must(write_frame(&mut stream, &received, maximum));
    });
    let mut stream = must(TcpStream::connect(address));
    must(stream.set_read_timeout(Some(Duration::from_secs(5))));
    must(stream.set_write_timeout(Some(Duration::from_secs(5))));
    must(write_frame(&mut stream, &payload, maximum));
    assert_eq!(must(read_frame(&mut stream, maximum)), payload);
    must(worker.join());
}

#[test]
fn f19_retired_module_leaves_real_generic_frame_transport() {
    retired("mod.rs");
    let case = receipt_case();
    frame_crossing(must(serde_json::to_vec(&case["receiptJson"])));
}

#[test]
fn f20_retired_provider_abi_leaves_bounded_canonical_frame_codec() {
    retired("abi.rs");
    let bytes = unhex(&receipt_case()["signedReceipt"]);
    let receipt = must(layerx_wire::receipt::decode(&bytes));
    assert_eq!(must(layerx_wire::receipt::encode(&receipt)), bytes);
    frame_crossing(bytes.clone());
    let mut encoded = Vec::new();
    must(write_frame(&mut encoded, &bytes, bytes.len()));
    let mut receiver = std::io::Cursor::new(encoded);
    let refusal = TransportError::Frame(FrameViolation::Oversized {
        declared: must(u32::try_from(bytes.len())),
        maximum: bytes.len() - 1,
    });
    assert_eq!(read_frame(&mut receiver, bytes.len() - 1), Err(refusal));
    assert_eq!(receiver.position(), 4, "refuse before reading the body");
    let mut output = Vec::new();
    assert_eq!(
        write_frame(&mut output, &bytes, bytes.len() - 1),
        Err(refusal)
    );
    assert!(output.is_empty());
}

#[test]
fn f21_capability_advertisements_cannot_replace_receipt_key_pins() {
    retired("capabilities.rs");
    let case = receipt_case();
    let served = case.clone();
    let server = Server::start(move |request| {
        let result = match request["method"].as_str().expect("method") {
            "px_getReceiptKeys" => {
                assert_eq!(request["params"], json!([]));
                json!({"keys":[served["pinnedKey"]]})
            }
            "pax_sequencerReceipt" => {
                assert_eq!(request["params"], json!([served["receiptJson"]["txHash"]]));
                served["receiptJson"].clone()
            }
            "eth_getTransactionReceipt" => {
                assert_eq!(request["params"], json!([served["receiptJson"]["txHash"]]));
                Value::Null
            }
            other => panic!("retired or unexpected method: {other}"),
        };
        json!({"result": result})
    });
    let endpoint = server.sequencer();
    // The fixture's governed pin is supplied independently of the endpoint.
    let pins = must(decode_receipt_keys(&json!({"keys":[case["pinnedKey"]]})));
    assert_eq!(must(endpoint.receipt_keys()), pins);
    let tx_hash = must(unhex(&case["receiptJson"]["txHash"]).try_into());
    let lookup = must(lookup_authenticated(
        &endpoint,
        tx_hash,
        LookupContext {
            receipt_keys: &pins,
        },
    ));
    let Lookup::Verified(receipt) = lookup else {
        panic!("recorded receipt must authenticate");
    };
    assert_eq!(receipt.rung(), Rung::Receipted);
    assert_eq!(receipt.canonical_bytes(), unhex(&case["signedReceipt"]));
    assert_eq!(receipt.committed_header(), None);
    let key = pins[0];
    for refused in [
        vec![],
        vec![layerx_client::sequencer::ReceiptKeyEntry {
            key_id: [0xff; 32],
            ..key
        }],
        vec![layerx_client::sequencer::ReceiptKeyEntry {
            public_key: [0xff; 32],
            ..key
        }],
        vec![layerx_client::sequencer::ReceiptKeyEntry {
            first_height: key.last_height + 1,
            last_height: key.last_height + 2,
            ..key
        }],
        vec![key, key],
    ] {
        assert_eq!(
            lookup_authenticated(
                &endpoint,
                tx_hash,
                LookupContext {
                    receipt_keys: &refused
                }
            ),
            Err(ReceiptError::Verification(VerificationFailure {
                check: ReceiptCheck::SequencerSignature,
            }))
        );
    }
    let requests = must(server.requests.lock());
    assert_eq!(requests.len(), 8);
    assert!(requests.iter().all(|r| matches!(
        r["method"].as_str(),
        Some("px_getReceiptKeys" | "pax_sequencerReceipt" | "eth_getTransactionReceipt")
    )));
}

#[test]
fn f22_handshake_is_replaced_by_explicit_chain_and_verified_commit() {
    retired("handshake.rs");
    let corpus: Value = must(serde_json::from_str(include_str!(
        "fixtures/repair_22_33_1546_node.json"
    )));
    let record = corpus["storage"].clone();
    let height = record["height"].as_u64().expect("height");
    let trust = ChainProofVerifier {
        chain_id: "hyperpax_125-1".to_owned(),
        validators: must(decode_validators(&record["validators"])),
    };
    let served = record.clone();
    let server = Server::start(move |request| {
        let result = match request["method"].as_str().expect("method") {
            "eth_chainId" => {
                assert_eq!(request["params"], json!([]));
                json!("0x7d")
            }
            "px_getCommit" => {
                assert_eq!(request["params"], json!([height.to_string()]));
                served["material"].clone()
            }
            "px_getValidators" => {
                assert_eq!(request["params"], json!([height.to_string()]));
                served["validators"].clone()
            }
            other => panic!("retired or unexpected method: {other}"),
        };
        json!({"result":result})
    });
    let endpoint = server.sequencer();
    assert_eq!(must(endpoint.chain_id()), 125);
    let head = must(acquire_committed_header(&endpoint, height, &trust));
    assert_eq!(must(u64::try_from(head.height())), height);
    assert_eq!(head.header_hash().as_slice(), unhex(&record["hash"]));
    assert_eq!(head.app_hash().as_slice(), unhex(&record["appHash"]));
    let foreign = ChainProofVerifier {
        chain_id: "foreign_126-1".to_owned(),
        ..trust
    };
    assert!(matches!(
        acquire_committed_header(&endpoint, height, &foreign),
        Err(CommitError::Proof(_))
    ));
    assert_eq!(server.count(), 5);
}

#[test]
fn f24_availability_report_cannot_authorize_uncommitted_evidence() {
    retired("report.rs");
    witness::f03_proof_bundle_retains_every_requested_slot_identity_and_value();
    witness::f03_proof_bundle_refuses_absent_mismatched_and_uncommitted_material();
}

#[test]
fn f25_retired_schema_is_replaced_by_real_rpc_storage_witnesses() {
    retired("schema.rs");
    witness::f02_module_state_returns_only_the_derived_slot_witness();
    witness::f02_module_state_refuses_slot_value_store_root_and_app_hash_substitution();
}
