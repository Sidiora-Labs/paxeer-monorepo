use layerx_mirror::committed::{CommittedCoordinate, CommittedHeader, CommittedRefusal};
use serde_json::{json, Value};

#[test]
fn committed_refusal_codes_are_the_published_strings() {
    const CODES: [(CommittedRefusal, &str); 9] = [
        (
            CommittedRefusal::Unavailable,
            CommittedRefusal::Unavailable.code(),
        ),
        (
            CommittedRefusal::ChainMismatch,
            CommittedRefusal::ChainMismatch.code(),
        ),
        (
            CommittedRefusal::GenesisMismatch,
            CommittedRefusal::GenesisMismatch.code(),
        ),
        (
            CommittedRefusal::Divergence,
            CommittedRefusal::Divergence.code(),
        ),
        (
            CommittedRefusal::NotCommitted,
            CommittedRefusal::NotCommitted.code(),
        ),
        (
            CommittedRefusal::ReceiptSignature,
            CommittedRefusal::ReceiptSignature.code(),
        ),
        (
            CommittedRefusal::ParentMismatch,
            CommittedRefusal::ParentMismatch.code(),
        ),
        (
            CommittedRefusal::HeightRegressed,
            CommittedRefusal::HeightRegressed.code(),
        ),
        (
            CommittedRefusal::Malformed,
            CommittedRefusal::Malformed.code(),
        ),
    ];
    let expected = [
        "committed_unavailable",
        "committed_chain_mismatch",
        "committed_genesis_mismatch",
        "committed_divergence",
        "committed_not_committed",
        "committed_receipt_signature",
        "committed_parent_mismatch",
        "committed_height_regressed",
        "committed_malformed",
    ];
    for ((refusal, code), expected) in CODES.into_iter().zip(expected) {
        assert_eq!(code, expected, "{refusal:?}");
    }
}

#[test]
fn a_recorded_block_answer_decodes_to_a_committed_header() {
    let answer: Value = serde_json::from_str(include_str!("fixtures/committed_blocks.json"))
        .unwrap_or_else(|error| panic!("recorded block JSON: {error}"));
    let block = &answer["result"];
    let header = CommittedHeader::from_block_json(block)
        .unwrap_or_else(|error| panic!("recorded block header: {error:?}"));
    assert_eq!(
        header,
        CommittedHeader {
            height: 42,
            block_hash: [0x11; 32],
            parent_hash: [0x22; 32],
            app_hash: [0x33; 32],
            timestamp: 1_700_000_000,
            transactions: vec![[0x44; 32], [0x55; 32]],
        }
    );
    let coordinate = CommittedCoordinate {
        height: header.height,
        block_hash: header.block_hash,
    };
    let copied = coordinate;
    assert_eq!(coordinate, copied);
    assert!(
        coordinate
            < CommittedCoordinate {
                height: 43,
                block_hash: [0; 32]
            }
    );
    assert!(
        coordinate
            < CommittedCoordinate {
                height: 42,
                block_hash: [0x12; 32]
            }
    );

    for field in [
        "number",
        "hash",
        "parentHash",
        "stateRoot",
        "timestamp",
        "transactions",
    ] {
        let mut missing = block.clone();
        missing
            .as_object_mut()
            .unwrap_or_else(|| panic!("block object"))
            .remove(field);
        assert_eq!(
            CommittedHeader::from_block_json(&missing),
            Err(CommittedRefusal::Malformed),
            "missing {field}"
        );
        for invalid in [Value::Null, json!(17), json!({}), json!(false)] {
            let mut malformed = block.clone();
            malformed[field] = invalid;
            assert_eq!(
                CommittedHeader::from_block_json(&malformed),
                Err(CommittedRefusal::Malformed),
                "invalid {field}"
            );
        }
    }

    let invalid_hashes = [
        "0x".to_owned(),
        format!("0x{}", "11".repeat(31)),
        format!("0x{}", "11".repeat(33)),
        "11".repeat(32),
        format!("0x{}gg", "11".repeat(31)),
        format!("0x{}é", "11".repeat(31)),
    ];
    for invalid in invalid_hashes {
        for field in ["hash", "parentHash", "stateRoot"] {
            let mut malformed = block.clone();
            malformed[field] = json!(invalid);
            assert_eq!(
                CommittedHeader::from_block_json(&malformed),
                Err(CommittedRefusal::Malformed),
                "invalid {field}: {invalid}"
            );
        }
        let mut malformed = block.clone();
        malformed["transactions"][1] = json!(invalid);
        assert_eq!(
            CommittedHeader::from_block_json(&malformed),
            Err(CommittedRefusal::Malformed),
            "invalid transaction hash: {invalid}"
        );
    }

    for invalid in [
        json!({"hash": block["transactions"][0]}),
        Value::Null,
        json!(17),
    ] {
        let mut malformed = block.clone();
        malformed["transactions"][1] = invalid;
        assert_eq!(
            CommittedHeader::from_block_json(&malformed),
            Err(CommittedRefusal::Malformed)
        );
    }

    for field in ["number", "timestamp"] {
        for invalid in [
            "",
            "0x",
            "42",
            "0x00",
            "0x-1",
            "0x+1",
            "0xgg",
            "0x10000000000000000",
        ] {
            let mut malformed = block.clone();
            malformed[field] = json!(invalid);
            assert_eq!(
                CommittedHeader::from_block_json(&malformed),
                Err(CommittedRefusal::Malformed),
                "invalid {field}: {invalid}"
            );
        }
        for (valid, expected) in [("0x0", 0), ("0xffffffffffffffff", u64::MAX)] {
            let mut boundary = block.clone();
            boundary[field] = json!(valid);
            let decoded = CommittedHeader::from_block_json(&boundary)
                .unwrap_or_else(|error| panic!("valid {field}: {error:?}"));
            assert_eq!(
                if field == "number" {
                    decoded.height
                } else {
                    decoded.timestamp
                },
                expected
            );
        }
    }

    for invalid in [Value::Null, json!([]), json!("block")] {
        assert_eq!(
            CommittedHeader::from_block_json(&invalid),
            Err(CommittedRefusal::Malformed)
        );
    }
    let mut empty = block.clone();
    empty["transactions"] = json!([]);
    let decoded = CommittedHeader::from_block_json(&empty)
        .unwrap_or_else(|error| panic!("empty block: {error:?}"));
    assert!(decoded.transactions.is_empty());
}
