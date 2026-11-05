use layerx_agent_api::budget::ProtocolBudgetView;
use layerx_agent_api::error::{ApiError, ErrorClass, RequestId, Retriability};
use layerx_agent_api::identity::ContractError;
use layerx_agent_api::subscription::ReceiptReference;
use layerx_agent_api::track::ReceiptRef;
use layerx_agent_api::verify::Rung;
use layerx_agent_api::Amount;
use layerx_client::budget::ProtocolBudgetRecord;
use layerx_types::error::ErrorClass as LayerErrorClass;
use layerx_types::result::ResultCode;

#[test]
fn budget_view_requires_the_committed_rung() {
    let record = ProtocolBudgetRecord {
        budget_id: [1; 32],
        owner: [2; 32],
        budget_account: [3; 32],
        asset_id: [4; 32],
        purpose_hash: [5; 32],
        per_period_limit: 1_000,
        configured_period_limit: 900,
        carry_cap: 200,
        spent_this_period: 100,
        carried: 50,
        period_length: 60_000,
        period_start: 120_000,
        expiry: 240_000,
        revocation_sequence: 7,
        rollover_policy: 1,
        closed: false,
        revoked: true,
        delegates: vec![[6; 32], [7; 32]],
        source_account: Some([8; 32]),
    };

    for rung in [Rung::Submitted, Rung::Receipted] {
        assert_eq!(
            ProtocolBudgetView::from_proven(&record, rung),
            Err(ContractError::OutOfRange("budget_rung")),
            "budget evidence at {rung:?} must be refused",
        );
    }

    let view = ProtocolBudgetView::from_proven(&record, Rung::Committed)
        .unwrap_or_else(|error| panic!("committed budget: {error:?}"));
    assert_eq!(view.achieved_rung, Rung::Committed);
    assert_eq!(view.budget_id, record.budget_id);
    assert_eq!(view.owner, record.owner);
    assert_eq!(view.budget_account, record.budget_account);
    assert_eq!(view.asset_id, record.asset_id);
    assert_eq!(view.purpose_hash, record.purpose_hash);
    assert_eq!(view.per_period_limit, Amount(record.per_period_limit));
    assert_eq!(
        view.configured_period_limit,
        Amount(record.configured_period_limit)
    );
    assert_eq!(view.carry_cap, Amount(record.carry_cap));
    assert_eq!(view.spent_this_period, Amount(record.spent_this_period));
    assert_eq!(view.carried, Amount(record.carried));
    assert_eq!(view.period_length_ms, record.period_length);
    assert_eq!(view.period_start_ms, record.period_start);
    assert_eq!(view.expiry_ms, record.expiry);
    assert_eq!(view.revocation_counter, record.revocation_sequence);
    assert_eq!(view.rollover_policy, record.rollover_policy);
    assert_eq!(view.closed, record.closed);
    assert_eq!(view.revoked, record.revoked);
    assert_eq!(view.delegates, record.delegates);
    assert_eq!(view.source_account, record.source_account);
}

#[test]
fn stream_receipt_reference_carries_a_rung() {
    let receipt =
        ReceiptRef::new("receipt-1").unwrap_or_else(|error| panic!("receipt reference: {error:?}"));
    for achieved in [Rung::Submitted, Rung::Receipted, Rung::Committed] {
        let reference = ReceiptReference::Verified {
            receipt_ref: receipt.clone(),
            rung: achieved,
        };
        match reference {
            ReceiptReference::Verified { receipt_ref, rung } => {
                assert_eq!(receipt_ref, receipt);
                assert_eq!(rung, achieved);
            }
            ReceiptReference::None => panic!("verified receipt reference was lost"),
        }
    }
}

#[test]
fn idempotent_replay_is_a_terminal_core_rejection() {
    let request_id = RequestId(42);
    let error = ApiError::idempotent_replay(request_id);
    assert_eq!(error.class, ErrorClass::CoreRejection);
    assert_ne!(error.class, ErrorClass::IdempotencyConflict);
    assert_eq!(error.protocol_result_code, Some(ResultCode::from_raw(-302)));
    assert_eq!(error.retriability, Retriability::Terminal);
    assert_eq!(error.request_id, request_id);
    assert_eq!(error.reason.as_str(), "idempotent_replay");

    let mappings = [
        (
            LayerErrorClass::TransportFailure,
            ErrorClass::TransportFailure,
        ),
        (LayerErrorClass::Deadline, ErrorClass::Deadline),
        (
            LayerErrorClass::ProtocolIncompatibility,
            ErrorClass::ProtocolIncompatibility,
        ),
        (
            LayerErrorClass::UnavailableCapability,
            ErrorClass::UnavailableCapability,
        ),
        (LayerErrorClass::CoreRejection, ErrorClass::CoreRejection),
        (
            LayerErrorClass::VerificationFailure,
            ErrorClass::VerificationFailure,
        ),
        (LayerErrorClass::PolicyRefusal, ErrorClass::PolicyRefusal),
        (
            LayerErrorClass::CapabilityRefusal,
            ErrorClass::CapabilityRefusal,
        ),
        (LayerErrorClass::BudgetRefusal, ErrorClass::BudgetRefusal),
        (LayerErrorClass::RateLimit, ErrorClass::RateLimit),
        (LayerErrorClass::InternalFault, ErrorClass::InternalFault),
    ];
    for (source, expected) in mappings {
        let mapped = ErrorClass::from(source);
        assert_eq!(mapped, expected);
        assert_ne!(mapped, ErrorClass::IdempotencyConflict);
    }
}
