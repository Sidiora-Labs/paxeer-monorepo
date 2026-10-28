use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_human_service::journeys::{
    authority_label, plan, verify_bindings, BalanceEntry, BindingExpectation, Constraints,
    CustodyContext, Endpoint, ExecutableShape, FeeSchedule, IntentLegBinding, LegMechanism,
    Mechanism, ObservedState, SubmitPlanRequest, SubmitRefusal, UnifiedIntent, UnifiedPlan,
};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::AssetId;
use layerx_types::intent::{EvmAddress, TimestampSeconds};
use serde_json::{json, Value};

fn nonce_plan(deposit: bool) -> UnifiedPlan {
    let home_account = AccountId::parse("agent:did:layerx:human:main").unwrap();
    let home = Endpoint::human(home_account.clone()).unwrap();
    let asset = AssetId::new([0x33; 32]);
    let custody = deposit.then(|| {
        CustodyContext::new(
            EvmAddress::new([0x55; 20]),
            AccountId::parse("system:treasury").unwrap(),
            AccountId::parse("system:paxeer-withdrawals").unwrap(),
        )
        .unwrap()
    });
    let source = if deposit {
        Endpoint::PaxeerWallet
    } else {
        home.clone()
    };
    let destination = if deposit {
        home
    } else {
        Endpoint::agent(AccountId::parse("agent:did:layerx:worker:main").unwrap()).unwrap()
    };
    let observed = ObservedState::new(
        TimestampSeconds::from_u64(1_000),
        home_account,
        custody,
        vec![BalanceEntry::new(
            source.clone(),
            asset,
            Amount::from_u128(500),
        )],
        Vec::new(),
        Vec::new(),
        FeeSchedule::new(vec![
            (LegMechanism::Protocol(Mechanism::Send), 2),
            (LegMechanism::PaxeerCustodyDeposit, 4),
            (LegMechanism::Protocol(Mechanism::BridgeDepositCredit), 3),
        ])
        .unwrap(),
    )
    .unwrap();
    let intent = UnifiedIntent::new(
        source,
        destination,
        asset,
        Amount::from_u128(25),
        Constraints::new(TimestampSeconds::from_u64(1_200), 20, false),
    )
    .unwrap();
    plan(&intent, &observed).unwrap()
}

fn pending_expectation(pending_nonce: u64) -> BindingExpectation {
    BindingExpectation {
        actor: AgentDid::new("did:layerx:owner").unwrap(),
        authority: AuthorityRef::new("custody-human-primary").unwrap(),
        account_sequence: pending_nonce,
        currency: "LXP".to_owned(),
        now: 1_000,
    }
}

fn binding_document(plan: &UnifiedPlan, index: usize, pending_nonce: u64) -> Value {
    let expectation = pending_expectation(pending_nonce);
    let action_key: String = plan
        .action_key(index)
        .unwrap()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    json!({
        "leg_index": index,
        "action_key": action_key,
        "actor": expectation.actor.as_str(),
        "authority": authority_label(plan.legs()[index].authority()),
        "relationship": "self",
        "account_sequence": pending_nonce,
        "not_before": 995,
        "not_after": 1_100,
        "fee_limit": {"amount": plan.legs()[index].fee().to_string(), "currency": "LXP"},
    })
}

fn bound_request(plan: &UnifiedPlan, pending_nonce: u64) -> SubmitPlanRequest {
    SubmitPlanRequest {
        plan_digest: plan.digest(),
        signed_digest: plan.digest(),
        bindings: (0..plan.legs().len())
            .map(|index| {
                IntentLegBinding::from_json(&binding_document(plan, index, pending_nonce)).unwrap()
            })
            .collect(),
    }
}

#[test]
fn intent_leg_binding_matches_the_pending_nonce() {
    let plan = nonce_plan(false);
    assert_eq!(plan.legs().len(), 1);
    assert_eq!(plan.executable_shape(), ExecutableShape::Kernel);
    for pending_nonce in [0, 42, u64::MAX] {
        let expectation = pending_expectation(pending_nonce);
        let mut request = bound_request(&plan, pending_nonce);
        assert_eq!(request.bindings[0].account_sequence, pending_nonce);
        assert_eq!(verify_bindings(&plan, &request, &expectation), Ok(()));

        request.bindings[0].account_sequence = 7;
        assert_eq!(
            verify_bindings(&plan, &request, &expectation),
            Err(SubmitRefusal::StaleSequence { index: 0 })
        );
    }
    let expectation = pending_expectation(42);
    for mismatched_nonce in [41, 43] {
        assert_eq!(
            verify_bindings(&plan, &bound_request(&plan, mismatched_nonce), &expectation),
            Err(SubmitRefusal::StaleSequence { index: 0 })
        );
    }

    let mut overflow = binding_document(&plan, 0, u64::MAX);
    overflow["account_sequence"] = serde_json::from_str("18446744073709551616").unwrap();
    assert_eq!(
        IntentLegBinding::from_json(&overflow),
        Err(SubmitRefusal::Malformed {
            field: "account_sequence"
        })
    );
}

#[test]
fn intent_legs_reusing_a_nonce_are_refused() {
    let plan = nonce_plan(true);
    assert_eq!(plan.executable_shape(), ExecutableShape::CustodyDeposit);
    assert_eq!(plan.legs().len(), 2);
    let expectation = pending_expectation(42);
    let mut request = bound_request(&plan, 42);
    assert_eq!(request.bindings[0].actor, request.bindings[1].actor);
    assert_eq!(
        request.bindings[0].account_sequence,
        request.bindings[1].account_sequence
    );
    assert_ne!(
        request.bindings[0].action_key,
        request.bindings[1].action_key
    );
    assert_eq!(
        verify_bindings(&plan, &request, &expectation),
        Err(SubmitRefusal::StaleSequence { index: 1 })
    );

    request.bindings[1].account_sequence = 43;
    assert_eq!(
        verify_bindings(&plan, &request, &expectation),
        Err(SubmitRefusal::StaleSequence { index: 1 })
    );
}
