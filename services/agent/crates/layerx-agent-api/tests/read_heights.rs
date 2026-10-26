use layerx_agent_api::availability::AvailabilityRequest;
use layerx_agent_api::export::{
    check_export_response, validate_export_request, FactRef, OfflineExport,
};
use layerx_agent_api::identity::{Asset, ContractError};
use layerx_agent_api::prepare::CanonicalBytes;
use layerx_agent_api::proof::{ProofBundle, ProofBundleTarget, MAX_PROOF_BUNDLE_BYTES};
use layerx_agent_api::read::{
    AccountRef, BalanceValue, CheckpointValue, FeeProjection, FeeProjectionRequest, Freshness,
    HistoryCursor, HistorySelector, HistoryValue, ModuleRef, ModuleStateSelector, ReadRequest,
    RelativeTo, VerifiedRead,
};
use layerx_agent_api::verify::Rung;
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_client::sequencer::ReceiptKeyEntry;

fn bytes(value: u8) -> CanonicalBytes {
    CanonicalBytes::new(vec![value]).expect("nonempty canonical bytes")
}

fn freshness() -> Freshness {
    Freshness {
        chain_head: u64::MAX,
        value_height: 9_007_199_254_740_993,
        relative_to: RelativeTo::Block {
            height: 9_007_199_254_740_993,
            hash: [7; 32],
        },
    }
}

fn bundle(rung: Rung) -> ProofBundle {
    let committed = rung == Rung::Committed;
    ProofBundle {
        format: "layerx-receipt-proof-v2".to_owned(),
        verification_level: rung,
        raw_transaction: bytes(1),
        signed_receipt: bytes(2),
        receipt_digest: [3; 32],
        receipt_key: ReceiptKeyEntry {
            key_id: [4; 32],
            public_key: [5; 32],
            first_height: 1,
            last_height: u64::MAX,
        },
        block_header: committed.then(|| bytes(6)),
        receipt_inclusion: committed.then(|| bytes(7)),
        commit: committed.then(|| bytes(8)),
        validator_set: committed.then(|| bytes(9)),
        account_proof: None,
    }
}

#[test]
fn verified_read_carries_achieved_rung_and_height_freshness() {
    let value = BalanceValue {
        account: AccountRef::new("account").unwrap(),
        asset: Asset::new("LXP").unwrap(),
        amount: Amount(u128::MAX),
        canonical_state: bytes(1),
        height: freshness().value_height,
    };
    let response = VerifiedRead::new(value, Rung::Committed, freshness());
    let VerifiedRead {
        value,
        achieved_rung,
        freshness:
            Freshness {
                chain_head,
                value_height,
                relative_to,
            },
    } = response;
    assert_eq!(achieved_rung, Rung::Committed);
    assert_eq!(chain_head, u64::MAX);
    assert_eq!(value_height, 9_007_199_254_740_993);
    assert_eq!(value.height, value_height);
    assert_eq!(value.amount, Amount(u128::MAX));
    assert_eq!(
        relative_to,
        RelativeTo::Block {
            height: value_height,
            hash: [7; 32]
        }
    );
    let request = ReadRequest {
        selector: value.account,
        requested_rung: Rung::Committed,
    };
    assert_eq!(request.requested_rung, achieved_rung);
    let availability = AvailabilityRequest {
        selector: value_height.to_string(),
        requested_rung: Rung::Receipted,
        maximum_bytes: 1024,
        maximum_chunks: 1,
        deadline: TimestampSeconds(1),
    }
    .validate()
    .unwrap();
    assert_eq!(availability.requested_rung, Rung::Receipted);
    let selector = ModuleStateSelector {
        module: ModuleRef::new("module").unwrap(),
        key: bytes(2),
    };
    assert_eq!(selector.key.as_bytes(), &[2]);
    let checkpoint = CheckpointValue {
        commit: bytes(3),
        validator_set: bytes(4),
        height: value_height,
    };
    assert_eq!(checkpoint.height, value_height);
    let projection = FeeProjection {
        request: FeeProjectionRequest {
            protocol_activity_type: 1,
            canonical_bytes: 1,
            execution_units: 2,
            storage_units: 3,
        },
        parameter_version: 1,
        fee: Amount(u128::MAX),
        gas_units: u64::MAX,
        denom: "sponsor-denom".to_owned(),
        canonical_schedule: bytes(5),
        snapshot_height: value_height,
        snapshot_state_root: [6; 32],
    };
    let mut observed = freshness();
    observed.chain_head = value_height;
    let estimate = projection
        .clone()
        .into_projection("gas estimate", observed)
        .unwrap();
    assert_eq!(estimate.projected.denom, "sponsor-denom");
    assert_eq!(estimate.projected.gas_units, u64::MAX);
    assert_eq!(
        estimate.projected.snapshot_height,
        estimate.observed_freshness.value_height
    );
    assert_eq!(
        projection.into_projection("gas estimate", freshness()),
        Err(ContractError::Mismatch("fee_snapshot_height"))
    );
}

#[test]
fn history_selector_pages_by_block_position_cursor() {
    let cursor = HistoryCursor {
        block_height: 9_007_199_254_740_993,
        tx_index: u64::MAX,
        log_index: 0,
    };
    let selector = HistorySelector {
        from_height: cursor.block_height,
        to_height: u64::MAX,
        cursor: Some(cursor),
        page_limit: 65_535,
    };
    assert_eq!(selector.clone().validate(), Ok(selector.clone()));
    let page = HistoryValue {
        records: vec![bytes(1)],
        next_cursor: Some(cursor),
        height: cursor.block_height,
    };
    assert_eq!(selector.cursor, page.next_cursor);
    let HistoryCursor {
        block_height,
        tx_index,
        log_index,
    } = page.next_cursor.unwrap();
    assert_eq!(
        (block_height, tx_index, log_index),
        (9_007_199_254_740_993, u64::MAX, 0)
    );
    assert!(
        cursor
            < HistoryCursor {
                log_index: 1,
                ..cursor
            }
    );
    assert!(HistorySelector {
        from_height: 10,
        to_height: 9,
        ..selector.clone()
    }
    .validate()
    .is_err());
    assert_eq!(
        HistorySelector {
            page_limit: 0,
            ..selector.clone()
        }
        .validate(),
        Err(ContractError::Zero("page_limit"))
    );
    assert_eq!(
        HistorySelector {
            page_limit: 65_536,
            ..selector.clone()
        }
        .validate(),
        Err(ContractError::OutOfRange("page_limit"))
    );
    assert!(HistorySelector {
        from_height: 0,
        to_height: 0,
        cursor: None,
        page_limit: 1
    }
    .validate()
    .is_ok());
    let end = HistoryValue {
        records: vec![],
        next_cursor: None,
        height: page.height,
    };
    assert_eq!(end.next_cursor, None);
}

#[test]
fn proof_bundle_mirror_requires_receipted_or_committed_rungs() {
    for target in [
        ProofBundleTarget::Activity([1; 32]),
        ProofBundleTarget::Receipt([1; 32]),
        ProofBundleTarget::AccountState {
            activity_id: [1; 32],
            account_id: [2; 32],
        },
    ] {
        let expected = if matches!(target, ProofBundleTarget::AccountState { .. }) {
            Rung::Committed
        } else {
            Rung::Receipted
        };
        for achieved in [Rung::Submitted, Rung::Receipted, Rung::Committed] {
            let mut value = bundle(achieved);
            if matches!(target, ProofBundleTarget::AccountState { .. })
                && achieved == Rung::Committed
            {
                value.account_proof = Some(bytes(10));
            }
            let request = ReadRequest {
                selector: target,
                requested_rung: expected,
            };
            let response = VerifiedRead::new(value, achieved, freshness());
            assert_eq!(
                ProofBundle::check_response(&request, &response).is_ok(),
                achieved == expected
            );
        }
    }
    let request = ReadRequest {
        selector: ProofBundleTarget::Receipt([1; 32]),
        requested_rung: Rung::Committed,
    };
    let response = VerifiedRead::new(bundle(Rung::Receipted), Rung::Receipted, freshness());
    assert!(ProofBundle::check_response(&request, &response).is_err());
    let request = ReadRequest {
        requested_rung: Rung::Receipted,
        ..request
    };
    let mut inconsistent = response.clone();
    inconsistent.value.verification_level = Rung::Committed;
    assert!(ProofBundle::check_response(&request, &inconsistent).is_err());
    let mut invalid = bundle(Rung::Committed);
    for field in 0..4 {
        let mut missing = invalid.clone();
        match field {
            0 => missing.block_header = None,
            1 => missing.receipt_inclusion = None,
            2 => missing.commit = None,
            _ => missing.validator_set = None,
        }
        assert!(missing.validate().is_err());
    }
    invalid.format = "LXPB1".to_owned();
    assert!(invalid.validate().is_err());
    invalid = bundle(Rung::Receipted);
    invalid.commit = Some(bytes(1));
    assert!(invalid.validate().is_err());
    invalid = bundle(Rung::Committed);
    invalid.raw_transaction = CanonicalBytes::new(vec![1; MAX_PROOF_BUNDLE_BYTES]).unwrap();
    assert!(invalid.validate().is_err());
}

#[test]
fn offline_export_mirror_has_no_certificates() {
    let facts = vec![FactRef::new(format!("receipt:{}", "01".repeat(32))).unwrap()];
    let request = validate_export_request(ReadRequest {
        selector: facts.clone(),
        requested_rung: Rung::Committed,
    })
    .unwrap();
    let export = OfflineExport {
        facts: facts.clone(),
        bundles: vec![bundle(Rung::Committed)],
        height: freshness().value_height,
    };
    assert_eq!(export.clone().validate(), Ok(export.clone()));
    // An exhaustive pattern ensures the mirror contains exactly the portable export fields.
    let OfflineExport {
        facts: carried_facts,
        bundles,
        height,
    } = export.clone();
    assert_eq!(carried_facts, facts);
    assert_eq!(bundles[0].signed_receipt.as_bytes(), &[2]);
    assert!(bundles[0].receipt_inclusion.is_some());
    assert_eq!(height, freshness().value_height);
    let response = VerifiedRead::new(export.clone(), Rung::Committed, freshness());
    assert_eq!(check_export_response(&request, &response), Ok(()));
    let mut wrong_height = response.clone();
    wrong_height.value.height -= 1;
    assert_eq!(
        check_export_response(&request, &wrong_height),
        Err(ContractError::Mismatch("export_height"))
    );
    let mut wrong_rung = response.clone();
    wrong_rung.achieved_rung = Rung::Receipted;
    assert!(check_export_response(&request, &wrong_rung).is_err());
    let mut wrong_facts = response;
    wrong_facts.value.facts.clear();
    assert_eq!(
        check_export_response(&request, &wrong_facts),
        Err(ContractError::Mismatch("export_facts"))
    );
    assert_eq!(
        OfflineExport {
            facts: vec![],
            ..export.clone()
        }
        .validate(),
        Err(ContractError::Empty("fact_set"))
    );
    assert_eq!(
        OfflineExport {
            bundles: vec![],
            ..export.clone()
        }
        .validate(),
        Err(ContractError::Empty("offline_evidence"))
    );
    assert_eq!(
        OfflineExport {
            bundles: vec![bundle(Rung::Receipted)],
            ..export
        }
        .validate(),
        Err(ContractError::OutOfRange("offline_rung"))
    );
}
