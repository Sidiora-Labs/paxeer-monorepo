mod support;

use layerx_mirror::committed::{
    CommittedBlockSource, CommittedCoordinate, CommittedHeader, CommittedRefusal,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use serde_json::{json, Value};

fn committed_height(document: &Value) -> u64 {
    u64::from_str_radix(
        document["committedBlock"]["number"]
            .as_str()
            .unwrap_or_else(|| panic!("block number"))
            .strip_prefix("0x")
            .unwrap_or_else(|| panic!("hex number")),
        16,
    )
    .unwrap_or_else(|error| panic!("height: {error}"))
}

#[test]
fn a_committed_block_is_acquired_with_every_receipt_verified() {
    let document = support::fixture();
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    let source =
        CommittedBlockSource::new(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    let height = committed_height(&document);
    let block = source
        .acquire(height)
        .unwrap_or_else(|error| panic!("acquire: {error:?}"));
    assert_eq!(
        block.header,
        CommittedHeader::from_block_json(&document["committedBlock"])
            .unwrap_or_else(|error| panic!("header: {error:?}"))
    );
    let expected = document["receipts"]
        .as_array()
        .unwrap_or_else(|| panic!("receipts"));
    assert_eq!(block.receipts.len(), expected.len());
    let key = &rpc.config.receipt_keys[0];
    let pins = [SequencerAuthorization::new(
        support::bytes(&json!(key.sequencer_id_hex))
            .try_into()
            .unwrap_or_else(|_| panic!("key ID")),
        support::bytes(&json!(key.public_key_hex))
            .try_into()
            .unwrap_or_else(|_| panic!("public key")),
        key.first_height,
        key.last_height,
    )];
    for (index, (bytes, answer)) in block.receipts.iter().zip(expected).enumerate() {
        assert_eq!(*bytes, support::bytes(&answer["receipt"]));
        let receipt = verify_sequencer_signature(bytes, ReceiptKeys::HeightRanges(&pins))
            .unwrap_or_else(|error| panic!("verified receipt: {error:?}"));
        assert_eq!(
            receipt
                .protocol()
                .unwrap_or_else(|| panic!("protocol receipt"))
                .activity_id(),
            block.header.transactions[index]
        );
    }
    let requests = rpc
        .requests
        .lock()
        .unwrap_or_else(|error| panic!("requests: {error}"));
    for endpoint in 0..2 {
        let calls: Vec<_> = requests
            .iter()
            .filter(|(index, _)| *index == endpoint)
            .map(|(_, request)| request)
            .collect();
        assert_eq!(calls.len(), 3 + expected.len());
        assert_eq!(calls[0]["method"], "eth_chainId");
        assert_eq!(calls[1]["params"], json!(["0x0", false]));
        assert_eq!(calls[2]["method"], "eth_getBlockByNumber");
        assert_eq!(calls[2]["params"], json!([format!("0x{height:x}"), false]));
        for (index, transaction) in document["committedBlock"]["transactions"]
            .as_array()
            .unwrap_or_else(|| panic!("transactions"))
            .iter()
            .enumerate()
        {
            assert_eq!(calls[index + 3]["method"], "pax_sequencerReceipt");
            assert_eq!(calls[index + 3]["params"], json!([transaction]));
        }
    }
    drop(requests);
    let mut config = rpc.config.clone();
    config.chain_id += 1;
    assert!(matches!(
        CommittedBlockSource::new(&config),
        Err(CommittedRefusal::ChainMismatch)
    ));
    config = rpc.config.clone();
    config.genesis_hash_hex = format!("0x{}", "ff".repeat(32));
    assert!(matches!(
        CommittedBlockSource::new(&config),
        Err(CommittedRefusal::GenesisMismatch)
    ));

    let mut different = document.clone();
    different["committedBlock"]["timestamp"] = json!("0x1");
    let divergent = support::TestRpc::new(vec![document.clone(), different]);
    let source = CommittedBlockSource::new(&divergent.config)
        .unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(source.acquire(height), Err(CommittedRefusal::Divergence));
    let mut different = document.clone();
    different["receipts"][1]["rung"] = json!("receipted");
    let divergent = support::TestRpc::new(vec![document, different]);
    let source = CommittedBlockSource::new(&divergent.config)
        .unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(source.acquire(height), Err(CommittedRefusal::Divergence));
}

#[test]
fn a_transaction_not_yet_committed_refuses_the_block() {
    let document = support::fixture();
    let height = committed_height(&document);
    for (field, value) in [
        ("rung", json!("submitted")),
        ("rung", json!("receipted")),
        ("blockNumber", json!(format!("0x{:x}", height + 1))),
        ("blockHash", json!(format!("0x{}", "ff".repeat(32)))),
        ("blockNumber", Value::Null),
        ("blockHash", Value::Null),
    ] {
        let mut changed = document.clone();
        changed["receipts"][1][field] = value;
        let rpc = support::TestRpc::new(vec![changed.clone(), changed]);
        let source = CommittedBlockSource::new(&rpc.config)
            .unwrap_or_else(|error| panic!("source: {error:?}"));
        assert_eq!(
            source.acquire(height),
            Err(CommittedRefusal::NotCommitted),
            "{field}"
        );
        let requests = rpc
            .requests
            .lock()
            .unwrap_or_else(|error| panic!("requests: {error}"));
        assert_eq!(
            requests
                .iter()
                .filter(|(_, request)| request["method"] == "pax_sequencerReceipt")
                .count(),
            4
        );
    }
}

#[test]
fn a_receipt_outside_every_key_window_refuses_the_block() {
    let document = support::fixture();
    let height = committed_height(&document);
    let rpc = support::TestRpc::new(vec![document.clone(), document.clone()]);
    for (first, last) in [(height + 1, height + 2), (0, 999)] {
        let mut config = rpc.config.clone();
        config.receipt_keys[0].first_height = first;
        config.receipt_keys[0].last_height = last;
        let source =
            CommittedBlockSource::new(&config).unwrap_or_else(|error| panic!("source: {error:?}"));
        assert_eq!(
            source.acquire(height),
            Err(CommittedRefusal::ReceiptSignature)
        );
    }
    let mut invalid = document.clone();
    let signed = support::bytes(&invalid["receipts"][1]["receipt"]);
    let mut altered = signed;
    let last = altered
        .last_mut()
        .unwrap_or_else(|| panic!("signed receipt"));
    *last ^= 1;
    invalid["receipts"][1]["receipt"] = json!(format!(
        "0x{}",
        altered
            .iter()
            .map(|byte| format!("{byte:02x}"))
            .collect::<String>()
    ));
    let rpc = support::TestRpc::new(vec![invalid.clone(), invalid]);
    let source =
        CommittedBlockSource::new(&rpc.config).unwrap_or_else(|error| panic!("source: {error:?}"));
    assert_eq!(
        source.acquire(height),
        Err(CommittedRefusal::ReceiptSignature)
    );
}

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
