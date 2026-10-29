use layerx_agent_api::availability::{
    AvailabilityClass, AvailabilityCompletion, AvailabilityReport, ClassReport, ProviderRef,
};
use layerx_agent_api::identity::Asset;
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::ProofBundle;
use layerx_agent_api::read::{
    AccountRef, BalanceValue, CheckpointValue, Freshness, HistoryCursor, HistoryValue, RelativeTo,
};
use layerx_agent_api::verify::Level;
use layerx_agent_api::Amount;
use layerx_mcp::tools::read::{
    availability, balance, checkpoint, history, proof, receipt, Continuation, Pagination,
    ReadToolError, ReceiptValue, StableCursor,
};

fn bytes(value: &[u8]) -> CanonicalBytes {
    CanonicalBytes::new(value.to_vec()).unwrap_or_else(|error| panic!("bytes: {error:?}"))
}

fn freshness() -> Freshness {
    Freshness {
        chain_head: 120,
        value_height: 117,
        relative_to: RelativeTo::Block {
            height: 117,
            hash: [0x41; 32],
        },
    }
}

fn proof_bundle() -> ProofBundle {
    let value = serde_json::json!({
        "facts": ["receipt:0x0101010101010101010101010101010101010101010101010101010101010101"],
        "bundles": [{
            "format": "layerx-receipt-proof-v2",
            "verificationLevel": "committed",
            "rawTransaction": "0x636f72652d726f6f74",
            "signedReceipt": "0x636f72652d72656365697074",
            "receiptDigest": format!("0x{}", "01".repeat(32)),
            "receiptKey": {
                "keyId": format!("0x{}", "02".repeat(32)),
                "publicKey": format!("0x{}", "03".repeat(32)),
                "firstHeight": "1",
                "lastHeight": "120"
            },
            "blockHeader": "0x70726f6f662d61",
            "receiptInclusion": "0x70726f6f662d62",
            "commit": "0x636f6d6d6974",
            "validatorSet": "0x76616c696461746f7273",
            "accountProof": "0x6163636f756e742d70726f6f66"
        }],
        "height": "117"
    });
    layerx_sdk::rpc_export::decode_offline_export(&value, layerx_sdk::Operation::ExportOffline)
        .unwrap_or_else(|error| panic!("proof bundle: {error:?}"))
        .bundles
        .remove(0)
}

#[test]
fn every_read_shape_requires_a_level_and_carries_the_exact_freshness() {
    let expected_freshness = freshness();
    let balance_result = balance(
        BalanceValue {
            account: AccountRef::new("did:layerx:reader")
                .unwrap_or_else(|error| panic!("account: {error:?}")),
            asset: Asset::new("LXP").unwrap_or_else(|error| panic!("asset: {error:?}")),
            amount: Amount(42),
            canonical_state: bytes(b"core-balance"),
            height: 117,
        },
        Level::StateProven,
        expected_freshness.clone(),
        1024,
    )
    .unwrap_or_else(|error| panic!("balance: {error:?}"));
    assert_eq!(balance_result.verification_level, Level::StateProven);
    assert_eq!(balance_result.freshness, expected_freshness);
    assert!(balance_result.page.complete);

    let receipt_result = receipt(
        ReceiptValue {
            canonical_receipt: bytes(b"core-receipt"),
            evidence_ids: vec![[1; 32]],
        },
        Level::BatchIncluded,
        freshness(),
        1024,
    )
    .unwrap_or_else(|error| panic!("receipt: {error:?}"));
    assert_eq!(receipt_result.verification_level, Level::BatchIncluded);

    let checkpoint_result = checkpoint(
        CheckpointValue {
            commit: bytes(b"core-certificate"),
            validator_set: bytes(b"core-validators"),
            height: 117,
        },
        Level::CheckpointFinalised,
        freshness(),
        1024,
    )
    .unwrap_or_else(|error| panic!("checkpoint: {error:?}"));
    assert_eq!(
        checkpoint_result.verification_level,
        Level::CheckpointFinalised
    );

    let proof_result = proof(proof_bundle(), Level::StateProven, freshness(), 1024)
        .unwrap_or_else(|error| panic!("proof: {error:?}"));
    assert_eq!(proof_result.verification_level, Level::StateProven);

    let provider =
        ProviderRef::new("provider-a").unwrap_or_else(|error| panic!("provider: {error:?}"));
    let availability_result = availability(
        AvailabilityReport {
            completion: AvailabilityCompletion::Complete { provider },
            classes: vec![ClassReport {
                class: AvailabilityClass::Receipts,
                complete: true,
                verified_chunks: 1,
                verified_bytes: 12,
                failure: None,
            }],
            providers: Vec::new(),
        },
        Level::BatchIncluded,
        freshness(),
        1024,
    )
    .unwrap_or_else(|error| panic!("availability: {error:?}"));
    assert_eq!(availability_result.page.returned_bytes, 12);
}

#[test]
fn unverified_or_evidence_free_values_never_enter_a_fact_result() {
    assert!(matches!(
        checkpoint(
            CheckpointValue {
                commit: bytes(b"unverified"),
                validator_set: bytes(b"core-validators"),
                height: 117,
            },
            Level::Unverified,
            freshness(),
            1024,
        ),
        Err(ReadToolError::Unverified)
    ));
    assert!(matches!(
        receipt(
            ReceiptValue {
                canonical_receipt: bytes(b"receipt"),
                evidence_ids: Vec::new(),
            },
            Level::SequencerSigned,
            freshness(),
            1024,
        ),
        Err(ReadToolError::MissingReceiptEvidence)
    ));
}

#[test]
fn history_truncation_is_explicit_and_uses_snapshot_bound_stable_cursors() {
    let query = [0x31; 32];
    let snapshot = [0x41; 32];
    let all = HistoryValue {
        records: vec![bytes(b"one"), bytes(b"two"), bytes(b"three")],
        next_cursor: None,
        height: 117,
    };
    let first = history(
        &all.clone(),
        Level::BatchIncluded,
        freshness(),
        query,
        snapshot,
        Pagination::new(2, 6, None).unwrap_or_else(|error| panic!("pagination: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("first page: {error:?}"));
    assert!(!first.page.complete);
    assert!(first.page.explicitly_truncated);
    let Continuation::Local(cursor) = first
        .page
        .next
        .unwrap_or_else(|| panic!("continuation absent"))
    else {
        panic!("wrong continuation kind");
    };
    assert_eq!(cursor.offset, 2);
    let second = history(
        &all,
        Level::BatchIncluded,
        freshness(),
        query,
        snapshot,
        Pagination::new(2, 6, Some(cursor)).unwrap_or_else(|error| panic!("pagination: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("second page: {error:?}"));
    assert!(second.page.complete);
    assert_eq!(second.value.records, vec![bytes(b"three")]);

    let wrong = StableCursor {
        query_digest: [9; 32],
        snapshot,
        offset: 0,
    };
    assert!(matches!(
        history(
            &HistoryValue {
                records: vec![bytes(b"one")],
                next_cursor: None,
                height: 117,
            },
            Level::BatchIncluded,
            freshness(),
            query,
            snapshot,
            Pagination::new(1, 10, Some(wrong))
                .unwrap_or_else(|error| panic!("pagination: {error:?}")),
        ),
        Err(ReadToolError::CursorMismatch)
    ));
}

#[test]
fn upstream_core_pagination_is_never_misreported_as_complete() {
    let result = history(
        &HistoryValue {
            records: vec![bytes(b"one")],
            next_cursor: Some(HistoryCursor {
                block_height: 117,
                tx_index: 2,
                log_index: 0,
            }),
            height: 117,
        },
        Level::BatchIncluded,
        freshness(),
        [1; 32],
        [2; 32],
        Pagination::new(10, 1024, None).unwrap_or_else(|error| panic!("pagination: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("history: {error:?}"));
    assert!(!result.page.complete);
    assert!(result.page.explicitly_truncated);
    assert!(matches!(result.page.next, Some(Continuation::Core(_))));
}
