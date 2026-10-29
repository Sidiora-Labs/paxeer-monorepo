use std::collections::{BTreeMap, BTreeSet};

use layerx_agent_api::budget::{AuthorityDescription, AuthorityResponse};
use layerx_agent_api::capability::{AmountCeiling, CapabilityDimensions, ExplicitSet};
use layerx_agent_api::error::{ErrorClass, RequestId, Retriability};
use layerx_agent_api::generated::{Amount, TimestampSeconds};
use layerx_agent_api::identity::{
    ActivityType, AgentDid, Asset, AuthorityRef, Counterparty, Purpose, TenantId,
};
use layerx_agent_api::verify::{Rung, RungStatus};
use layerx_agentd::agent_rpc_adapters::{budget_state_operation, capability_operation_error};
use layerx_agentd::capability::timed::TimedCapability;
use layerx_agentd::capability::{Dimension, ProtocolScope};
use layerx_agentd::human::{HumanOperationError, HumanRefusal};
use layerx_agentd::human_runtime::{budget_state_from_get_budget, check_capability_grant};
use layerx_agentd::identity::ProtocolAuthority;
use serde_json::json;

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("identity operation: {error:?}"))
}

// The dynamic tuple layout of the deployed ILayerXPay.getBudget result.
fn get_budget_result() -> Vec<u8> {
    let mut bytes = vec![0; 22 * 32];
    for (index, value) in [
        (0, 32_u128),
        (5, 1_000),
        (6, 900),
        (7, 100),
        (8, 350),
        (9, 100),
        (10, 60_000),
        (11, 1_000),
        (12, 90_000),
        (13, 7),
        (14, 2),
        (17, 18 * 32),
        (19, 2),
    ] {
        bytes[index * 32 + 16..(index + 1) * 32].copy_from_slice(&value.to_be_bytes());
    }
    for (index, value) in [
        (1, 0x11),
        (2, 0x22),
        (3, 0x33),
        (4, 0x44),
        (18, 0x55),
        (20, 0x66),
        (21, 0x77),
    ] {
        bytes[index * 32..(index + 1) * 32].fill(value);
    }
    bytes
}

#[test]
fn budget_state_operation_reports_committed_rung() {
    let bytes = get_budget_result();
    let state = must(budget_state_from_get_budget([9; 32], 42, &bytes, 650));
    let authority = must(AuthorityDescription::new(
        must(TenantId::new("identity-ops")),
        must(AgentDid::new("did:layerx:identity-ops")),
        must(AuthorityRef::new(hex::encode([8; 32]))),
        [vec![2], vec![8; 32]].concat(),
    ));
    let (response, verification) = budget_state_operation(&AuthorityResponse {
        authority,
        value: state,
    });
    assert_eq!(verification, Some(RungStatus::Achieved(Rung::Committed)));
    assert_eq!(response["authority"]["authority_ref"], hex::encode([8; 32]));
    assert_eq!(response["value"]["balance"], "650");
    assert_eq!(response["value"]["proven_head"], "42");
    assert_eq!(response["value"]["activity_id"], json!(null));
    let record = &response["value"]["record"];
    assert_eq!(record["achieved_rung"], "committed");
    assert_eq!(record["enforcement"], "ProtocolBudget");
    for (field, byte) in [
        ("budget_id", 9),
        ("owner", 0x11),
        ("budget_account", 0x22),
        ("asset_id", 0x33),
        ("purpose_hash", 0x44),
        ("source_account", 0x55),
    ] {
        assert_eq!(record[field], hex::encode([byte; 32]), "{field}");
    }
    for (field, amount) in [
        ("per_period_limit", "1000"),
        ("configured_period_limit", "900"),
        ("carry_cap", "100"),
        ("spent_this_period", "350"),
        ("carried", "100"),
        ("period_length_ms", "60000"),
        ("period_start_ms", "1000"),
        ("expiry_ms", "90000"),
        ("revocation_counter", "7"),
        ("rollover_policy", "2"),
        ("head", "42"),
    ] {
        assert_eq!(record[field], amount, "{field}");
    }
    assert_eq!(record["closed"], false);
    assert_eq!(record["revoked"], false);
    assert_eq!(
        record["delegates"],
        json!([hex::encode([0x66; 32]), hex::encode([0x77; 32])])
    );
    assert!(matches!(
        budget_state_from_get_budget([9; 32], 42, &bytes[..bytes.len() - 1], 650),
        Err(HumanOperationError::Typed(HumanRefusal::BudgetCodec))
    ));
}

fn granted_capability() -> (TimedCapability, ProtocolScope) {
    let asset = hex::encode([3; 32]);
    let dimensions = CapabilityDimensions {
        activity_types: ExplicitSet::allow(vec![ActivityType(5)]),
        counterparties: ExplicitSet::allow(vec![must(Counterparty::new(hex::encode([2; 32])))]),
        assets: ExplicitSet::allow(vec![must(Asset::new(asset.clone()))]),
        amount_ceilings: ExplicitSet::allow(vec![AmountCeiling {
            asset: must(Asset::new(asset)),
            amount: Amount(500),
        }]),
        rate_ceilings: ExplicitSet::allow(vec![]),
        purpose_constraints: ExplicitSet::allow(vec![must(Purpose::new("service"))]),
        expiry: TimestampSeconds(90),
    };
    let capability = must(TimedCapability::from_public(
        [1; 32],
        None,
        must(layerx_agentd::store::TenantId::new("identity-ops")),
        "did:layerx:identity-ops",
        ProtocolAuthority::CapabilityGrant([8; 32]),
        &dimensions,
        90_000,
        1_000,
        42,
        RequestId(9),
    ));
    let grant = ProtocolScope {
        activity_types: BTreeSet::from([5]),
        counterparties: BTreeSet::from([[2; 32]]),
        assets: BTreeSet::from([[3; 32]]),
        amount_ceiling: 500,
        expires_at_sequence: 100,
        not_before_ms: 1_000,
        not_after_ms: 90_000,
        enforceable_dimensions: BTreeSet::from([
            Dimension::ActivityType,
            Dimension::Counterparty,
            Dimension::Asset,
            Dimension::Amount,
            Dimension::Expiry,
        ]),
    };
    (capability, grant)
}

#[test]
fn capability_create_cannot_exceed_the_on_chain_grant() {
    let (capability, grant) = granted_capability();
    assert_eq!(check_capability_grant(&capability, &grant, 1_000), Ok(()));
    let mut wider = Vec::new();
    let mut activity = capability.clone();
    activity.activity_types.insert(6);
    wider.push((activity, Dimension::ActivityType));
    let mut counterparty = capability.clone();
    counterparty.counterparties.insert([4; 32]);
    wider.push((counterparty, Dimension::Counterparty));
    let mut asset = capability.clone();
    asset.assets.insert([4; 32]);
    wider.push((asset, Dimension::Asset));
    let mut amount = capability.clone();
    amount.amount_ceilings = BTreeMap::from([([3; 32], 501)]);
    wider.push((amount, Dimension::Amount));
    let mut expiry = capability.clone();
    expiry.expiry_seconds = 91;
    wider.push((expiry, Dimension::Expiry));
    for (requested, dimension) in wider {
        let result = check_capability_grant(&requested, &grant, 1_000);
        assert_eq!(
            result,
            Err(HumanOperationError::CapabilityRefused(dimension))
        );
        let error = capability_operation_error(
            RequestId(9),
            result
                .err()
                .unwrap_or_else(|| panic!("wider grant admitted")),
        );
        assert_eq!(error.class, ErrorClass::CapabilityRefusal);
        assert_eq!(error.reason, "capability.exceeds_grant");
        assert_eq!(error.request_id, RequestId(9));
        assert_eq!(error.retriability, Retriability::Terminal);
    }
    let unavailable = capability_operation_error(RequestId(9), HumanOperationError::Unavailable);
    assert_eq!(unavailable.class, ErrorClass::UnavailableCapability);
    assert_eq!(unavailable.reason, "owner.unavailable");
}
