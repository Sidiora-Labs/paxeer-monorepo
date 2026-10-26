use std::fmt::Debug;

use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::receipt::{
    ReceiptError, ReceiptLog, ReceiptView, Rung, SequencerReceipt,
};
use layerx_proof::inclusion::SequencerAuthorization;
use serde::Deserialize;
use serde_json::{json, Value};

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
