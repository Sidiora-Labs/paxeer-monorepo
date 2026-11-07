use std::fmt::Write as _;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::rpc::receipt::ReconnectPolicy;
use layerx_client::rpc::receipt::{
    lookup, lookup_authenticated, lookup_authenticated_with_commit, resolve_unknown,
    AuthenticatedReceipt, Lookup, LookupContext, ReceiptError, Resolution, Rung,
};
use layerx_client::rpc::submit::{submit_signed, Submission};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::{transaction_hash, Eip1559Call};
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

#[path = "support/market_22_33_1546.rs"]
mod commit_producer;

use layerx_client::rpc::commit::CommitError;
use layerx_client::rpc::receipt_commit::{CoverageError, ReceiptCommitEvidence, SequenceBatch};
use layerx_programs_registry::chain_proof::merkle::{hash_from_byte_slices, SimpleProof};

const HEIGHT: u64 = 150;
const KEY_ID: [u8; 32] = [1; 32];
const TX_HASH: [u8; 32] = [3; 32];

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::from("0x"), |mut text, byte| {
        must(write!(text, "{byte:02x}"));
        text
    })
}

fn encoded_receipt(
    tx_hash: [u8; 32],
    success: bool,
    fee_denom: &str,
    payment_denom: &str,
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    let previous = [4; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&7_u64.to_be_bytes());
    sequence.extend_from_slice(&tx_hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut from = [0; 32];
    from[12..].fill(6);
    let mut to = [0; 32];
    to[12..].fill(7);
    let vm_error = if success { "" } else { "execution reverted" };
    let mut summary = Encoder::new(256);
    must(summary.u64(21_000));
    must(summary.u64(42_000));
    must(summary.u64(1));
    must(summary.u32(1));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&sha256(vm_error.as_bytes())));
    must(summary.u32(2));
    must(summary.u32(1));
    must(summary.u8(must(u8::try_from(fee_denom.len()))));
    must(summary.fixed(fee_denom.as_bytes()));
    let mut log = vec![8; 20];
    log.push(1);
    log.extend_from_slice(&[9; 32]);
    log.extend_from_slice(&sha256(&[10, 11]));

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
    must(encoder.u16(PROTOCOL_VERSION));
    must(encoder.bytes(&tx_hash, 32));
    must(encoder.u64(7));
    must(encoder.bytes(&previous, 32));
    must(encoder.bytes(&sha256(&sequence), 32));
    must(encoder.bytes(&sha256(&[0xab, 0xcd]), 32));
    must(encoder.i32(i32::from(!success)));
    must(encoder.sequence_length(2, 512));
    for (ordinal, kind, body) in [(0, 1, summary.finish()), (1, 3, log)] {
        must(encoder.u16(12));
        must(encoder.u16(ordinal));
        must(encoder.u16(0));
        must(encoder.u8(kind));
        must(encoder.u8(0));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&body, 256));
    }
    must(encoder.u128(21_000));
    must(encoder.bytes(&height, 32));
    must(encoder.u16(12));
    must(encoder.u32(1));
    must(encoder.u32(0));
    must(encoder.u8(2));
    must(encoder.bytes(&sha256(payment_denom.as_bytes()), 32));
    must(encoder.u128(1));
    must(encoder.bytes(&from, 32));
    must(encoder.u128(21_002));
    must(encoder.u128(if success { 1 } else { 2 }));
    must(encoder.u64(41));
    must(encoder.bytes(&to, 32));
    must(encoder.u128(0));
    must(encoder.u128(u128::from(success)));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&KEY_ID, 32));
    must(encoder.u64(1_000));
    must(encoder.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(encoder.bytes(&signature, 64));
    }
    encoder.finish()
}

fn signed_receipt(tx_hash: [u8; 32], success: bool) -> (Value, Vec<u8>, ReceiptKeyEntry) {
    signed_receipt_with_denoms(tx_hash, success, "uhpx", "uhpx")
}

fn signed_receipt_with_denoms(
    tx_hash: [u8; 32],
    success: bool,
    fee_denom: &str,
    payment_denom: &str,
) -> (Value, Vec<u8>, ReceiptKeyEntry) {
    let signer = SigningKey::from_bytes(&[8; 32]);
    let unsigned = encoded_receipt(tx_hash, success, fee_denom, payment_denom, None);
    let signature = signer.sign(&must(receipt_digest(&unsigned))).to_bytes();
    let bytes = encoded_receipt(tx_hash, success, fee_denom, payment_denom, Some(signature));
    let key = ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: signer.verifying_key().to_bytes(),
        first_height: 100,
        last_height: 199,
    };
    let view = json!({
        "rung": "receipted", "txHash": hex(&tx_hash), "nonce": "0x29",
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": hex(&[8; 20]), "topics": [hex(&[9; 32])],
            "data": "0x0a0b", "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": fee_denom, "paymentDenom": payment_denom, "feeAmount": "0x5208", "sponsor": hex(&[0; 20]),
        "from": hex(&[6; 20]), "to": hex(&[7; 20]), "contractAddress": hex(&[0; 20]),
        "vmError": if success { "" } else { "execution reverted" },
        "keyId": hex(&KEY_ID), "receipt": hex(&bytes), "signingKey": hex(&key.public_key)
    });
    (view, bytes, key)
}

type Exchange = (&'static str, Value, Option<Value>);

fn serve(exchanges: Vec<Exchange>) -> (SequencerEndpoint, JoinHandle<()>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    let handle = std::thread::spawn(move || {
        for (method, params, response) in exchanges {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            must(stream.set_write_timeout(Some(Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
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
            let mut body = vec![0; length.unwrap_or_else(|| panic!("missing content length"))];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            if let Some(result) = response {
                let answer =
                    json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()
                ));
                must(stream.flush());
            }
        }
    });
    (must(SequencerEndpoint::new(&url, &[])), handle)
}

fn verified(lookup: Lookup) -> AuthenticatedReceipt {
    let Lookup::Verified(receipt) = lookup else {
        panic!("expected an authenticated receipt, got {lookup:?}");
    };
    receipt
}

fn receipt_exchange(tx_hash: [u8; 32], view: Value) -> Exchange {
    ("pax_sequencerReceipt", json!([hex(&tx_hash)]), Some(view))
}

fn landing_exchange(tx_hash: [u8; 32], landing: Value) -> Exchange {
    (
        "eth_getTransactionReceipt",
        json!([hex(&tx_hash)]),
        Some(landing),
    )
}

#[test]
fn lookup_authenticated_reaches_receipted_from_pax_sequencer_receipt() {
    let (view, bytes, key) = signed_receipt(TX_HASH, true);
    let keys = [key];
    let context = LookupContext {
        receipt_keys: &keys,
    };
    let (endpoint, server) = serve(vec![
        receipt_exchange(TX_HASH, view.clone()),
        landing_exchange(TX_HASH, Value::Null),
    ]);
    let receipt = verified(must(lookup_authenticated(&endpoint, TX_HASH, context)));
    assert_eq!(receipt.rung(), Rung::Receipted);
    assert_eq!(receipt.canonical_bytes(), bytes);
    assert_eq!(receipt.activity_id(), TX_HASH);
    assert_eq!(receipt.global_sequence(), 7);
    assert_eq!(receipt.module_id(), 12);
    assert_eq!(receipt.height(), HEIGHT);
    assert_eq!(receipt.result_code().raw(), 0);
    assert_eq!(receipt.landing(), None);
    assert_eq!(receipt.execution().logs[0].data, [10, 11]);
    assert_eq!(
        receipt.receipt().protocol().map(|r| r.context_hash()),
        Some(KEY_ID)
    );
    must(server.join());

    for pins in [
        vec![],
        vec![ReceiptKeyEntry {
            last_height: HEIGHT - 1,
            ..key
        }],
        vec![ReceiptKeyEntry {
            first_height: HEIGHT + 1,
            ..key
        }],
        vec![ReceiptKeyEntry {
            key_id: [2; 32],
            ..key
        }],
        vec![ReceiptKeyEntry {
            public_key: SigningKey::from_bytes(&[9; 32]).verifying_key().to_bytes(),
            ..key
        }],
        vec![key, key],
    ] {
        let (endpoint, server) = serve(vec![receipt_exchange(TX_HASH, view.clone())]);
        assert_eq!(
            lookup_authenticated(
                &endpoint,
                TX_HASH,
                LookupContext {
                    receipt_keys: &pins
                }
            ),
            Err(ReceiptError::Verification(VerificationFailure {
                check: ReceiptCheck::SequencerSignature
            }))
        );
        must(server.join());
    }
    let mut altered = bytes;
    let last = altered.len() - 1;
    altered[last] ^= 1;
    let mut bad_signature = view.clone();
    bad_signature["receipt"] = json!(hex(&altered));
    let (endpoint, server) = serve(vec![receipt_exchange(TX_HASH, bad_signature)]);
    assert_eq!(
        lookup_authenticated(&endpoint, TX_HASH, context),
        Err(ReceiptError::Verification(VerificationFailure {
            check: ReceiptCheck::SequencerSignature
        }))
    );
    must(server.join());

    for (field, value) in [
        ("height", json!("0x95")),
        ("status", json!("0x0")),
        ("nonce", json!("0x2a")),
        ("returnData", json!("0xabce")),
    ] {
        let mut changed = view.clone();
        changed[field] = value;
        let (endpoint, server) = serve(vec![receipt_exchange(TX_HASH, changed)]);
        assert_eq!(
            lookup_authenticated(&endpoint, TX_HASH, context),
            Err(ReceiptError::FieldMismatch(field))
        );
        must(server.join());
    }
    let mut changed_log = view.clone();
    changed_log["logs"][0]["data"] = json!("0x0a0c");
    let (endpoint, server) = serve(vec![receipt_exchange(TX_HASH, changed_log)]);
    assert!(matches!(
        lookup_authenticated(&endpoint, TX_HASH, context),
        Err(ReceiptError::Logs(_))
    ));
    must(server.join());

    for pending in [
        Value::Null,
        json!({"rung": "submitted", "txHash": hex(&TX_HASH), "nonce": "0x29"}),
    ] {
        let (endpoint, server) = serve(vec![receipt_exchange(TX_HASH, pending.clone())]);
        let expected = if pending.is_null() {
            Lookup::Absent
        } else {
            Lookup::Submitted {
                tx_hash: TX_HASH,
                nonce: 41,
            }
        };
        assert_eq!(must(lookup(&endpoint, TX_HASH, context)), expected);
        must(server.join());
    }
}

#[test]
fn lookup_authenticated_reaches_committed_from_the_transaction_receipt() {
    for success in [true, false] {
        let (view, bytes, key) = signed_receipt(TX_HASH, success);
        let keys = [key];
        let context = LookupContext {
            receipt_keys: &keys,
        };
        for height in [HEIGHT, HEIGHT + 1] {
            let landing = json!({"status": if success { "0x1" } else { "0x0" },
                "blockNumber": format!("{height:#x}"), "transactionIndex": "0x1", "gasUsed": "0x5208", "logs": view["logs"]});
            let batch = covering_batch(height, 7);
            let bytes_batch = must(batch.encode());
            let root = hash_from_byte_slices(&[&sha256(&bytes_batch)]);
            let node = commit_producer::node_commit(height, [0x33; 32], root);
            let proof = SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: root,
                aunts: vec![],
            };
            let evidence = ReceiptCommitEvidence {
                verifier: &node.verifier,
                covering_height: height,
                batch_transaction: &bytes_batch,
                transaction_proof: &proof,
                activity_ids: &[TX_HASH],
            };
            let mut committed = view.clone();
            committed["rung"] = json!("committed");
            committed["blockNumber"] = json!(format!("{height:#x}"));
            committed["blockHash"] = json!(hex(&node.head.header_hash()));
            let mut exchanges = vec![
                receipt_exchange(TX_HASH, committed),
                landing_exchange(TX_HASH, landing),
            ];
            exchanges.extend(commit_exchanges(&node));
            let (endpoint, server) = serve(exchanges);
            let receipt = verified(must(lookup_authenticated_with_commit(
                &endpoint, TX_HASH, context, &evidence,
            )));
            assert_eq!(receipt.committed_header(), Some(&node.head));
            assert_eq!(
                receipt.covering_block(),
                Some((height, node.head.header_hash()))
            );
            assert_eq!(receipt.rung(), Rung::Committed);
            assert_eq!(receipt.canonical_bytes(), bytes);
            assert_eq!(receipt.result_code().raw(), i32::from(!success));
            let landing = receipt
                .landing()
                .unwrap_or_else(|| panic!("missing landing receipt"));
            assert_eq!(landing.block_number, height);
            assert_eq!(landing.status, success);
            must(server.join());
        }
        for (height, status, field) in [
            (HEIGHT - 1, success, "blockNumber"),
            (HEIGHT, !success, "status"),
        ] {
            let landing = json!({"status": if status { "0x1" } else { "0x0" },
                "blockNumber": format!("{height:#x}"), "transactionIndex": "0x1", "gasUsed": "0x5208", "logs": []});
            let (endpoint, server) = serve(vec![
                receipt_exchange(TX_HASH, view.clone()),
                landing_exchange(TX_HASH, landing),
            ]);
            assert_eq!(
                lookup_authenticated(&endpoint, TX_HASH, context),
                Err(ReceiptError::FieldMismatch(field))
            );
            must(server.join());
        }
        let mut claimed_commit = view.clone();
        claimed_commit["rung"] = json!("committed");
        claimed_commit["blockNumber"] = json!("0x97");
        claimed_commit["blockHash"] = json!(hex(&[0xab; 32]));
        let (endpoint, server) = serve(vec![
            receipt_exchange(TX_HASH, claimed_commit),
            landing_exchange(TX_HASH, Value::Null),
        ]);
        assert_eq!(
            verified(must(lookup_authenticated(&endpoint, TX_HASH, context))).rung(),
            Rung::Receipted
        );
        must(server.join());
        let (endpoint, server) = serve(vec![
            receipt_exchange(TX_HASH, view),
            ("eth_getTransactionReceipt", json!([hex(&TX_HASH)]), None),
        ]);
        let unavailable_landing = verified(must(lookup_authenticated(&endpoint, TX_HASH, context)));
        assert_eq!(unavailable_landing.rung(), Rung::Receipted);
        assert_eq!(unavailable_landing.landing(), None);
        assert_eq!(unavailable_landing.committed_header(), None);
        must(server.join());
    }

    let raw = must(
        Eip1559Call {
            chain_id: 125,
            nonce: 41,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 1,
            gas_limit: 21_000,
            to: [7; 20],
            data: vec![],
        }
        .sign(&[0x46; 32]),
    );
    let tx_hash = transaction_hash(&raw);
    let (view, _, key) = signed_receipt(tx_hash, true);
    let keys = [key];
    let context = LookupContext {
        receipt_keys: &keys,
    };
    let (endpoint, server) = serve(vec![
        ("eth_sendRawTransaction", json!([hex(&raw)]), None),
        receipt_exchange(
            tx_hash,
            json!({"rung": "submitted", "txHash": hex(&tx_hash), "nonce": "0x29"}),
        ),
        receipt_exchange(tx_hash, view),
        landing_exchange(
            tx_hash,
            json!({"status": "0x1", "blockNumber": "0x96", "transactionIndex": "0x1", "gasUsed": "0x5208", "logs": []}),
        ),
    ]);
    let Submission::Unknown(unknown) = must(submit_signed(&endpoint, &raw)) else {
        panic!("lost send must be unknown");
    };
    let policy = ReconnectPolicy {
        maximum_attempts: 2,
        base_delay: Duration::ZERO,
        maximum_delay: Duration::ZERO,
        jitter_percent: 0,
    };
    assert_eq!(
        must(resolve_unknown(
            &endpoint,
            &unknown,
            context,
            ReconnectPolicy {
                maximum_attempts: 0,
                ..policy
            }
        )),
        Resolution::Unknown(unknown.clone())
    );
    let Resolution::Resolved(receipt) = must(resolve_unknown(&endpoint, &unknown, context, policy))
    else {
        panic!("receipt must resolve the unknown send");
    };
    assert_eq!(receipt.activity_id(), tx_hash);
    assert_eq!(receipt.rung(), Rung::Receipted);
    assert_eq!(receipt.committed_header(), None);
    assert_eq!(
        receipt
            .landing()
            .unwrap_or_else(|| panic!("landing"))
            .block_number,
        HEIGHT
    );
    assert_eq!(unknown.retry_bytes(), raw);
    must(server.join());
}

#[test]
fn lookup_authenticated_accepts_the_regenerated_receipt_fixture() {
    fn decode_hex(value: &Value) -> Vec<u8> {
        let text = value.as_str().unwrap_or_else(|| panic!("hex string"));
        let digits = text
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("hex prefix"));
        assert_eq!(digits.len() % 2, 0);
        digits
            .as_bytes()
            .chunks_exact(2)
            .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
            .collect()
    }

    let fixture: Value = must(serde_json::from_str(include_str!(
        "../../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )));
    assert_eq!(
        fixture["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let case = &fixture["cases"][0];
    assert_eq!(case["id"], "V1-pax-transfer");
    let view = case["receiptJson"].clone();
    let tx_hash = must(decode_hex(&view["txHash"]).try_into());
    let keys = must(layerx_client::sequencer::decode_receipt_keys(
        &json!({"keys": [case["pinnedKey"]]}),
    ));
    let (endpoint, server) = serve(vec![
        receipt_exchange(tx_hash, view),
        landing_exchange(tx_hash, Value::Null),
    ]);
    let receipt = verified(must(lookup_authenticated(
        &endpoint,
        tx_hash,
        LookupContext {
            receipt_keys: &keys,
        },
    )));
    assert_eq!(receipt.execution().payment_denom, "uhpx");
    assert_eq!(receipt.execution().fee_denom, "uhpx");
    assert_eq!(
        receipt.canonical_bytes(),
        decode_hex(&case["signedReceipt"])
    );
    assert_eq!(
        receipt
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol receipt"))
            .asset(),
        sha256(b"uhpx")
    );
    assert_eq!(receipt.rung(), Rung::Receipted);
    must(server.join());
}

#[test]
fn lookup_authenticated_checks_the_payment_asset_when_fees_use_a_token() {
    let (view, bytes, key) = signed_receipt_with_denoms(TX_HASH, true, "usid", "uhpx");
    let keys = [key];
    let context = LookupContext {
        receipt_keys: &keys,
    };
    let (endpoint, server) = serve(vec![
        receipt_exchange(TX_HASH, view.clone()),
        landing_exchange(TX_HASH, Value::Null),
    ]);
    let receipt = verified(must(lookup_authenticated(&endpoint, TX_HASH, context)));
    assert_eq!(receipt.execution().payment_denom, "uhpx");
    assert_eq!(receipt.execution().fee_denom, "usid");
    assert_eq!(receipt.canonical_bytes(), bytes);
    assert_eq!(
        receipt
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol receipt"))
            .asset(),
        sha256(b"uhpx")
    );
    must(server.join());

    let (mut wrong_asset, _, _) = signed_receipt_with_denoms(TX_HASH, true, "usid", "usid");
    wrong_asset["paymentDenom"] = json!("uhpx");
    let mut missing = view;
    assert!(missing
        .as_object_mut()
        .unwrap_or_else(|| panic!("view object"))
        .remove("paymentDenom")
        .is_some());
    for invalid in [wrong_asset, missing] {
        let (endpoint, server) = serve(vec![receipt_exchange(TX_HASH, invalid)]);
        assert_eq!(
            lookup_authenticated(&endpoint, TX_HASH, context),
            Err(ReceiptError::Sequencer(SequencerError::Malformed))
        );
        must(server.join());
    }
}

fn covering_batch(height: u64, sequence: u64) -> SequenceBatch {
    let mut chain = b"PAXEER/v1/sequence\0".to_vec();
    chain.extend_from_slice(&[4; 32]);
    chain.extend_from_slice(&sequence.to_be_bytes());
    chain.extend_from_slice(&TX_HASH);
    let mut batch = SequenceBatch {
        height,
        from_sequence: sequence,
        to_sequence: sequence,
        previous_root: [4; 32],
        resulting_root: sha256(&chain),
        activity_root: sha256(&TX_HASH),
        key_id: KEY_ID,
        signature: [0; 64],
    };
    batch.signature = SigningKey::from_bytes(&[8; 32])
        .sign(&must(batch.signing_digest("paxeer-repair-125")))
        .to_bytes();
    batch
}

fn commit_exchanges(node: &commit_producer::NodeCommit) -> Vec<Exchange> {
    let height = node.head.height();
    vec![
        (
            "px_getCommit",
            json!([height.to_string()]),
            Some(node.material.clone()),
        ),
        (
            "px_getValidators",
            json!([height.to_string()]),
            Some(node.validators.clone()),
        ),
        (
            "eth_getBlockByNumber",
            json!([format!("{height:#x}"), true]),
            Some(json!({
                "number": format!("{height:#x}"), "hash": hex(&node.head.header_hash()),
                "parentHash": hex(&[1;32]), "timestamp": "0x6553f100",
                "transactionsRoot": hex(&node.head.data_hash()), "receiptsRoot": hex(&[2;32]), "transactions": []
            })),
        ),
    ]
}

#[test]
fn committed_receipt_refuses_wrong_hash_missing_commit_foreign_authority_and_uncovered_sequence() {
    use base64::Engine as _;
    use layerx_programs_registry::chain_proof::validators::TrustedValidator;
    let height = HEIGHT + 1;
    let (view, canonical, key) = signed_receipt(TX_HASH, true);
    let keys = [key];
    let context = LookupContext {
        receipt_keys: &keys,
    };
    for refusal in [
        "claimed hash",
        "block hash",
        "missing commit",
        "foreign authority",
        "uncovered sequence",
        "state root",
        "no commit service",
    ] {
        let mut batch = covering_batch(
            height,
            if refusal == "uncovered sequence" {
                8
            } else {
                7
            },
        );
        if refusal == "state root" {
            batch.resulting_root = [99; 32];
            batch.signature = SigningKey::from_bytes(&[8; 32])
                .sign(&must(batch.signing_digest("paxeer-repair-125")))
                .to_bytes();
        }
        let batch_bytes = must(batch.encode());
        let root = hash_from_byte_slices(&[&sha256(&batch_bytes)]);
        let node = commit_producer::node_commit(height, [0x33; 32], root);
        let proof = SimpleProof {
            total: 1,
            index: 0,
            leaf_hash: root,
            aunts: vec![],
        };
        let evidence = ReceiptCommitEvidence {
            verifier: &node.verifier,
            covering_height: height,
            batch_transaction: &batch_bytes,
            transaction_proof: &proof,
            activity_ids: &[TX_HASH],
        };
        let mut claimed = view.clone();
        claimed["rung"] = json!("committed");
        claimed["blockNumber"] = json!(format!("{height:#x}"));
        claimed["blockHash"] = json!(hex(&node.head.header_hash()));
        let landing = json!({"status":"0x1","blockNumber":"0x96","transactionIndex":"0x1","gasUsed":"0x5208","logs":view["logs"]});
        let mut commits = commit_exchanges(&node);
        let expected = match refusal {
            "claimed hash" => {
                claimed["blockHash"] = json!(hex(&[0xab; 32]));
                commits.pop();
                ReceiptError::FieldMismatch("blockHash")
            }
            "block hash" => {
                commits[2].2.as_mut().unwrap_or_else(|| panic!("block"))["hash"] =
                    json!(hex(&[0xab; 32]));
                ReceiptError::FieldMismatch("blockHash")
            }
            "missing commit" => {
                assert!(
                    commits[0].2.as_mut().unwrap_or_else(|| panic!("commit"))["signedHeader"]
                        .as_object_mut()
                        .unwrap_or_else(|| panic!("signed header"))
                        .remove("commit")
                        .is_some()
                );
                commits.truncate(1);
                ReceiptError::Commit(CommitError::Rpc(SequencerError::Malformed))
            }
            "foreign authority" => {
                let signer = SigningKey::from_bytes(&[0x75; 32]);
                let foreign = TrustedValidator::new(signer.verifying_key().to_bytes(), 100);
                let validators = commits[1]
                    .2
                    .as_mut()
                    .unwrap_or_else(|| panic!("validators"));
                validators["validators"][0]["address"] = json!(&hex(&foreign.address)[2..]);
                validators["validators"][0]["pubKey"]["value"] =
                    json!(base64::engine::general_purpose::STANDARD
                        .encode(signer.verifying_key().to_bytes()));
                commits.truncate(2);
                ReceiptError::Commit(CommitError::Invalid)
            }
            "uncovered sequence" => ReceiptError::Coverage(CoverageError::Sequence),
            "state root" => ReceiptError::Coverage(CoverageError::StateRoot),
            "no commit service" => {
                commits[0].2 = None;
                commits.truncate(1);
                // Unavailable evidence must never promote the authenticated receipt.
                let mut exchanges = vec![
                    receipt_exchange(TX_HASH, claimed),
                    landing_exchange(TX_HASH, landing),
                ];
                exchanges.extend(commits);
                let (endpoint, server) = serve(exchanges);
                let receipt = verified(must(lookup_authenticated_with_commit(
                    &endpoint, TX_HASH, context, &evidence,
                )));
                assert_eq!(receipt.rung(), Rung::Receipted);
                assert_eq!(receipt.committed_header(), None);
                assert_eq!(receipt.canonical_bytes(), canonical);
                must(server.join());
                continue;
            }
            _ => panic!("unknown refusal"),
        };
        let mut exchanges = vec![
            receipt_exchange(TX_HASH, claimed),
            landing_exchange(TX_HASH, landing),
        ];
        exchanges.extend(commits);
        let (endpoint, server) = serve(exchanges);
        assert_eq!(
            lookup_authenticated_with_commit(&endpoint, TX_HASH, context, &evidence),
            Err(expected),
            "{refusal}"
        );
        must(server.join());
    }
}
