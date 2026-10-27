use std::fmt::Debug;

use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::receipt::{
    ReceiptError, ReceiptLog, ReceiptView, Rung, SequencerReceipt,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{
    verify_sequencer_signature, ReceiptCheck, ReceiptKeys, VerificationFailure,
};
use layerx_proof::sequencer_logs::{LogField, SequencerLogRefusal};
use serde::Deserialize;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use sha3::Keccak256;

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn fixture() -> Value {
    must(serde_json::from_str(include_str!(
        "fixtures/sequencer_receipts.json"
    )))
}

fn array<const N: usize>(value: &str) -> [u8; N] {
    must(hex::fixed(value))
}

fn key(document: &Value) -> SequencerAuthorization {
    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase")]
    struct Pin {
        key_id: String,
        public_key: String,
        first_height: u64,
        last_height: u64,
    }
    let pin: Pin = must(serde_json::from_value(document["key"].clone()));
    SequencerAuthorization::new(
        array(&pin.key_id),
        array(&pin.public_key),
        pin.first_height,
        pin.last_height,
    )
}

#[test]
fn a_committed_answer_decodes_every_field() {
    let document = fixture();
    let answer = must(SequencerReceipt::decode(&document["committed"]));
    assert_eq!(answer.rung, Rung::Committed);
    assert_eq!(
        answer.tx_hash,
        array("0xf36621d835bfc801ee639486d202b26733e6d138606dd0143cb07a498e2508db")
    );
    assert_eq!(answer.nonce, 1);
    assert_eq!(answer.block, Some((1000, [0xcd; 32])));
    let log = ReceiptLog {
        address: array("0x5fbdb2315678afecb367f032d93f642f64180aa3"),
        topics: vec![
            array("0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925"),
            array("0x000000000000000000000000fe3b557e8fb62b89f4916b721be55ceb828dbd73"),
            array("0x00000000000000000000000070997970c51812dc3a010c7d01b50e0d17dc79c8"),
        ],
        data: must(hex::decode(
            "0x00000000000000000000000000000000000000000000000d8d726b7177a80000",
        )),
        log_index: 0,
    };
    let receipt_bytes = must(hex::decode(
        document["committed"]["receipt"]
            .as_str()
            .unwrap_or_else(|| panic!("receipt bytes")),
    ));
    assert!(!receipt_bytes.is_empty());
    assert_eq!(
        answer.view,
        Some(ReceiptView {
            seq: 2,
            height: 1000,
            index: 2,
            timestamp: 1_750_000_400_000,
            status: 1,
            return_data: vec![0; 31].into_iter().chain([1]).collect(),
            logs: vec![log.clone()],
            gas_used: 46_220,
            cumulative_gas_used: 67_220,
            effective_gas_price: 2_000_000_000,
            fee_denom: "uhpx".to_owned(),
            fee_amount: 92_440_000_000_000,
            sponsor: [0; 20],
            from: array("0xfe3b557e8fb62b89f4916b721be55ceb828dbd73"),
            to: log.address,
            contract_address: [0; 20],
            vm_error: String::new(),
            key_id: [0x6b; 32],
            receipt: receipt_bytes,
        })
    );
    must(answer.verify(&[key(&document)]));

    // Decoding preserves the served order, even when log indices are out of order.
    let mut ordered = document["committed"].clone();
    let first = ordered["logs"][0].clone();
    let mut second = first.clone();
    second["logIndex"] = json!("0x1");
    second["data"] = json!("0xaabb");
    ordered["logs"] = json!([second, first]);
    let ordered = must(SequencerReceipt::decode(&ordered));
    let logs = ordered.view.unwrap_or_else(|| panic!("receipt view")).logs;
    let mut expected_first = log.clone();
    expected_first.log_index = 1;
    expected_first.data = vec![0xaa, 0xbb];
    assert_eq!(logs, vec![expected_first, log]);

    for rung in ["unknown", "Submitted", "finalized"] {
        let mut invalid = document["committed"].clone();
        invalid["rung"] = json!(rung);
        assert_eq!(
            SequencerReceipt::decode(&invalid),
            Err(ReceiptError::Malformed)
        );
    }
    for field in ["blockNumber", "blockHash"] {
        let mut invalid = document["committed"].clone();
        invalid
            .as_object_mut()
            .unwrap_or_else(|| panic!("object"))
            .remove(field);
        assert_eq!(
            SequencerReceipt::decode(&invalid),
            Err(ReceiptError::Malformed)
        );
    }
    let mut invalid = document["committed"].clone();
    invalid["blockNumber"] = json!("0x3e7");
    assert_eq!(
        SequencerReceipt::decode(&invalid),
        Err(ReceiptError::Malformed)
    );
    for field in [
        "seq",
        "height",
        "index",
        "timestamp",
        "status",
        "returnData",
        "logs",
        "gasUsed",
        "cumulativeGasUsed",
        "effectiveGasPrice",
        "feeDenom",
        "feeAmount",
        "sponsor",
        "from",
        "to",
        "contractAddress",
        "vmError",
        "keyId",
        "receipt",
    ] {
        let mut invalid = document["receipted"].clone();
        invalid
            .as_object_mut()
            .unwrap_or_else(|| panic!("object"))
            .remove(field);
        assert_eq!(
            SequencerReceipt::decode(&invalid),
            Err(ReceiptError::Malformed),
            "missing {field}"
        );
    }
    for (field, value) in [
        ("seq", json!(2)),
        ("height", json!("0x10000000000000000")),
        ("nonce", json!("0x01")),
        ("index", json!("0x+1")),
        ("status", json!("0x2")),
        ("txHash", json!("0x01")),
        ("keyId", Value::Null),
        ("receipt", json!("0xabc")),
        ("logs", Value::Null),
        ("feeAmount", json!("0x100000000000000000000000000000000")),
    ] {
        let mut invalid = document["receipted"].clone();
        invalid[field] = value;
        assert_eq!(
            SequencerReceipt::decode(&invalid),
            Err(ReceiptError::Malformed),
            "bad {field}"
        );
    }
    for (field, value) in [
        ("address", json!("0x01")),
        ("topics", json!(["0x01"])),
        ("data", json!("0xabc")),
        ("logIndex", json!("0x10000000000000000")),
    ] {
        let mut invalid = document["receipted"].clone();
        invalid["logs"][0][field] = value;
        assert_eq!(
            SequencerReceipt::decode(&invalid),
            Err(ReceiptError::Malformed)
        );
    }
    assert_eq!(
        SequencerReceipt::decode(&Value::Null),
        Err(ReceiptError::Malformed)
    );
}

#[test]
fn a_submitted_answer_carries_no_receipt() {
    let document = fixture();
    let answer = must(SequencerReceipt::decode(&document["submitted"]));
    assert_eq!(
        answer,
        SequencerReceipt {
            rung: Rung::Submitted,
            tx_hash: array("0xa52f2455a87ccb3b84a90ae4e3c9153e7122b66cd69cb080ae9ecd9732619277"),
            nonce: 0,
            view: None,
            block: None,
        }
    );
    assert_eq!(
        answer.verify(&[key(&document)]),
        Err(ReceiptError::Malformed)
    );
    for (field, value) in document["receipted"]
        .as_object()
        .unwrap_or_else(|| panic!("object"))
    {
        if ["rung", "txHash", "nonce"].contains(&field.as_str()) {
            continue;
        }
        for value in [value.clone(), Value::Null] {
            let mut invalid = document["submitted"].clone();
            invalid[field] = value;
            assert_eq!(
                SequencerReceipt::decode(&invalid),
                Err(ReceiptError::Malformed),
                "submitted {field}"
            );
        }
    }
    for rung in ["submitted", "receipted"] {
        for field in ["blockNumber", "blockHash"] {
            for value in [document["committed"][field].clone(), Value::Null] {
                let mut invalid = document[rung].clone();
                invalid[field] = value;
                assert_eq!(
                    SequencerReceipt::decode(&invalid),
                    Err(ReceiptError::Malformed)
                );
            }
        }
    }
    for rung in ["receipted", "committed"] {
        let mut invalid = document["submitted"].clone();
        invalid["rung"] = json!(rung);
        assert_eq!(
            SequencerReceipt::decode(&invalid),
            Err(ReceiptError::Malformed)
        );
    }
}

#[test]
fn receipt_bytes_must_verify_under_the_key_window_for_their_height() {
    let document = fixture();
    let pin = key(&document);
    let answer = must(SequencerReceipt::decode(&document["receipted"]));
    assert_eq!(answer.rung, Rung::Receipted);
    assert!(answer.block.is_none());
    must(answer.verify(&[pin]));
    let boundary = SequencerAuthorization::new(pin.sequencer_id(), pin.public_key(), 1000, 1000);
    must(answer.verify(&[boundary]));
    for invalid_pin in [
        SequencerAuthorization::new(pin.sequencer_id(), pin.public_key(), 1, 999),
        SequencerAuthorization::new(pin.sequencer_id(), pin.public_key(), 1001, 1999),
        SequencerAuthorization::new([0x11; 32], pin.public_key(), 1, 1999),
        SequencerAuthorization::new(pin.sequencer_id(), [0x11; 32], 1, 1999),
    ] {
        assert!(matches!(
            answer.verify(&[invalid_pin]),
            Err(ReceiptError::Verification(_))
        ));
    }
    assert!(matches!(
        answer.verify(&[]),
        Err(ReceiptError::Verification(_))
    ));
    assert!(matches!(
        answer.verify(&[pin, pin]),
        Err(ReceiptError::Verification(_))
    ));
    for (field, value) in [("seq", "0x3"), ("height", "0x3e9"), ("status", "0x0")] {
        let mut changed = document["receipted"].clone();
        changed[field] = json!(value);
        assert_eq!(
            must(SequencerReceipt::decode(&changed)).verify(&[pin]),
            Err(ReceiptError::Mismatch(field))
        );
    }
    let reverted = must(SequencerReceipt::decode(&document["reverted"]));
    assert_eq!(
        reverted
            .view
            .as_ref()
            .unwrap_or_else(|| panic!("view"))
            .status,
        0
    );
    must(reverted.verify(&[pin]));
    let mut changed = document["reverted"].clone();
    changed["status"] = json!("0x1");
    assert_eq!(
        must(SequencerReceipt::decode(&changed)).verify(&[pin]),
        Err(ReceiptError::Mismatch("status"))
    );

    let mut tampered = answer.clone();
    let view = tampered.view.as_mut().unwrap_or_else(|| panic!("view"));
    let last = view
        .receipt
        .last_mut()
        .unwrap_or_else(|| panic!("signature"));
    *last ^= 1;
    assert!(matches!(
        tampered.verify(&[pin]),
        Err(ReceiptError::Verification(_))
    ));
    let mut tampered = answer.clone();
    tampered
        .view
        .as_mut()
        .unwrap_or_else(|| panic!("view"))
        .logs[0]
        .data
        .push(0);
    assert!(matches!(
        tampered.verify(&[pin]),
        Err(ReceiptError::Logs(_))
    ));
    let mut tampered = answer.clone();
    tampered.tx_hash[0] ^= 1;
    assert_eq!(
        tampered.verify(&[pin]),
        Err(ReceiptError::Mismatch("txHash"))
    );
    let mut tampered = answer;
    tampered
        .view
        .as_mut()
        .unwrap_or_else(|| panic!("view"))
        .key_id[0] ^= 1;
    assert_eq!(
        tampered.verify(&[pin]),
        Err(ReceiptError::Mismatch("keyId"))
    );
}

#[test]
fn the_shared_sequencer_receipt_vector_decodes_and_verifies() {
    #[derive(Deserialize)]
    struct Vector {
        cases: Vec<Case>,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase")]
    struct Case {
        id: String,
        raw_transaction: String,
        prev_seq_hash: String,
        receipt_json: Value,
        pinned_key: Pin,
        expected: Expected,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase")]
    struct Pin {
        key_id: String,
        public_key: String,
        first_height: u64,
        last_height: u64,
    }

    #[derive(Deserialize)]
    #[serde(rename_all = "camelCase")]
    struct Expected {
        rung: Option<String>,
        refused_at: Option<String>,
        verifier: String,
    }

    let vector: Vector = must(serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))));
    assert!(
        !vector.cases.is_empty(),
        "shared receipt vector has no cases"
    );

    for case in vector.cases {
        let classification = match case.id.as_str() {
            "V1-pax-transfer"
            | "V1-erc20-approve"
            | "V1-layerxledger-pay-exact"
            | "V1-layerxbridge-bridge-out"
            | "V2-layerxledger-revert" => ("accepts", None),
            "V3-key-not-pinned" | "V3-height-outside-key-range" => {
                ("SequencerSignature", Some("R3"))
            }
            "V3-activity-id-not-raw-hash" => ("accepts", Some("R4")),
            "V3-tampered-log" => ("accepts", Some("R5")),
            "V3-seq-hash-mismatch" => ("accepts", Some("R6")),
            "V3-non-canonical-bytes" => ("Decode", Some("R2")),
            other => panic!("unclassified shared receipt case {other}"),
        };
        assert_eq!(
            (
                case.expected.verifier.as_str(),
                case.expected.refused_at.as_deref()
            ),
            classification,
            "{}: unexpected verifier or refusal rule",
            case.id
        );
        let pin = SequencerAuthorization::new(
            array(&case.pinned_key.key_id),
            array(&case.pinned_key.public_key),
            case.pinned_key.first_height,
            case.pinned_key.last_height,
        );
        let receipt = SequencerReceipt::decode(&case.receipt_json)
            .unwrap_or_else(|error| panic!("{}: decode: {error}", case.id));
        let view = receipt
            .view
            .as_ref()
            .unwrap_or_else(|| panic!("{}: receipt has no view", case.id));
        for (field, actual) in [("seq", view.seq), ("height", view.height)] {
            let expected = case.receipt_json[field]
                .as_str()
                .unwrap_or_else(|| panic!("{}: missing {field}", case.id));
            assert_eq!(
                actual,
                must(hex::parse_quantity(expected)),
                "{}: {field}",
                case.id
            );
        }
        let verification = receipt.verify(&[pin]);
        match case.expected.refused_at.as_deref() {
            Some("R2") => assert_eq!(
                verification,
                Err(ReceiptError::Verification(VerificationFailure {
                    check: ReceiptCheck::Decode,
                })),
                "{}: R2 must fail decoding the signed receipt bytes",
                case.id
            ),
            Some("R3") => assert_eq!(
                verification,
                Err(ReceiptError::Verification(VerificationFailure {
                    check: ReceiptCheck::SequencerSignature,
                })),
                "{}: R3 must fail signature authorization",
                case.id
            ),
            Some("R5") => assert_eq!(
                verification,
                Err(ReceiptError::Logs(SequencerLogRefusal::Log {
                    index: 0,
                    field: LogField::Data,
                })),
                "{}: R5 must detect the tampered log view",
                case.id
            ),
            None | Some("R4" | "R6") => {
                verification.unwrap_or_else(|error| panic!("{}: verify: {error}", case.id));
                if case.expected.refused_at.is_none() {
                    let rung = match case.expected.rung.as_deref() {
                        Some("receipted") => Rung::Receipted,
                        Some("committed") => Rung::Committed,
                        other => panic!("{}: unexpected accepted rung {other:?}", case.id),
                    };
                    assert_eq!(receipt.rung, rung, "{}", case.id);
                }
            }
            other => panic!("{}: unclassified refusal {other:?}", case.id),
        }

        if case.expected.verifier == "accepts" {
            let raw_transaction = must(hex::decode(&case.raw_transaction));
            let raw_hash: [u8; 32] = Keccak256::digest(&raw_transaction).into();
            assert_eq!(
                raw_hash == receipt.tx_hash,
                case.expected.refused_at.as_deref() != Some("R4"),
                "{}: R4 raw transaction hash binding",
                case.id
            );

            let verified = must(verify_sequencer_signature(
                &view.receipt,
                ReceiptKeys::HeightRanges(&[pin]),
            ));
            let protocol = verified
                .protocol()
                .unwrap_or_else(|| panic!("{}: missing protocol receipt", case.id));
            let previous_hash: [u8; 32] = array(&case.prev_seq_hash);
            let mut sequence_hash = Sha256::new();
            sequence_hash.update(b"PAXEER/v1/sequence\0");
            sequence_hash.update(previous_hash);
            sequence_hash.update(protocol.global_sequence().to_be_bytes());
            sequence_hash.update(protocol.activity_id());
            let resulting_hash: [u8; 32] = sequence_hash.finalize().into();
            assert_eq!(
                protocol.previous_state_root() == previous_hash
                    && protocol.resulting_state_root() == resulting_hash,
                case.expected.refused_at.as_deref() != Some("R6"),
                "{}: R6 sequence hash chain",
                case.id
            );
        }
    }
}
