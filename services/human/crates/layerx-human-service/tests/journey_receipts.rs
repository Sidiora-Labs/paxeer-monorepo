use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_human_service::custody::{KeyId, Operation};
use layerx_human_service::journeys::{
    JourneyEngine, JourneyError, JourneyKind, JourneyLeg, JourneyPhase, JourneyPlan, JourneyState,
};
use layerx_human_service::notify::JourneyId;
use layerx_human_service::store::{PrincipalStore, Table};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_intents::{BudgetDefund, Intent, IntentKind};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{BudgetId, Sequence};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use serde_json::Value;

fn retired_kind_plan() -> (JourneyId, ModuleRegistry, JourneyPlan) {
    let id = JourneyId::new("jrn_retiredwithdraw").or_fail("journey id");
    let activity = ActivityType::new(ModuleId::Budget, 8).or_fail("activity");
    let registry = ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Budget, &[activity]).or_fail("module")
    ])
    .or_fail("registry");
    let intent = Intent::v1(IntentKind::BudgetDefund(
        BudgetDefund::new(
            BudgetId::new([0x21; 32]),
            AccountId::parse("agent:did:layerx:alice:budget:travel").or_fail("budget account"),
            AccountId::parse("agent:did:layerx:alice:main").or_fail("owner account"),
            AssetId::new([0x22; 32]),
            Amount::from_u128(37),
            Sequence::from_u64(7),
            IdempotencyKey::new([0x31; 32]),
        )
        .or_fail("intent"),
    ));
    let plan = JourneyPlan::new(
        id.clone(),
        JourneyKind::Move,
        [0x41; 32],
        KeyId::new("human-primary").or_fail("custody key"),
        Operation::ProtocolMutation,
        vec![JourneyLeg::new(
            intent,
            [0x31; 32],
            AgentDid::new("did:layerx:alice").or_fail("actor"),
            AuthorityRef::new("custody-human-primary").or_fail("authority"),
            7,
            19,
            100,
            200,
            1_000,
        )
        .or_fail("leg")],
    )
    .or_fail("plan");
    (id, registry, plan)
}

#[test]
fn stored_journey_of_a_retired_kind_reads_as_closed() {
    let root = directory("journey-retired-kind");
    let retention = retention_uniform(10_000);
    let map = tenancy(&[("alice", "alice-tenant")]);
    let (mut store, digest) = install_and_open(&root, &map, retention);
    let owner = principal("alice");
    let (id, registry, plan) = retired_kind_plan();
    {
        let mut scope = store.principal(&owner).or_fail("scope");
        let live = JourneyEngine::start(&mut scope, &plan, &registry, 100).or_fail("persist plan");
        assert_eq!(
            live.status().or_fail("live status").state(),
            JourneyState::GettingReady
        );
        let row = scope
            .keys(Table::Journeys)
            .into_iter()
            .find(|key| key.as_str().starts_with("journey-"))
            .or_fail("journey row");
        let mut historical: Value = serde_json::from_slice(
            scope
                .get(Table::Journeys, &row)
                .or_fail("stored plan")
                .bytes(),
        )
        .or_fail("production record");
        assert_eq!(historical["legs"][0]["preparation"]["account_sequence"], 7);
        assert_eq!(historical["legs"][0]["preparation"]["pending_nonce"], 19);
        historical["kind"] = Value::String("withdraw".to_owned());
        historical["legs"][0]["preparation"]
            .as_object_mut()
            .or_fail("preparation")
            .remove("pending_nonce");
        scope
            .put(
                Table::Journeys,
                row,
                100,
                serde_json::to_vec(&historical).or_fail("legacy encoding"),
            )
            .or_fail("persist historical record");
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).or_fail("restart store");
    {
        let mut scope = store.principal(&owner).or_fail("reopened scope");
        let historical = JourneyEngine::load(&scope, &id)
            .or_fail("load historical")
            .or_fail("journey");
        assert_eq!(historical.kind(), None);
        assert_eq!(historical.historical_kind(), Some("withdraw"));
        let status = historical.status().or_fail("historical status");
        assert_eq!(status.state(), JourneyState::Closed);
        assert_eq!(status.phases(), &[JourneyPhase::Compiled]);
        assert_eq!(status.receipt_digests(), &[None]);
        let listed = JourneyEngine::list(&scope).or_fail("list historical");
        assert_eq!(listed.len(), 1);
        assert_eq!(
            listed[0].status().or_fail("listed status").state(),
            JourneyState::Closed
        );
        assert!(matches!(
            JourneyEngine::start(&mut scope, &plan, &registry, 101),
            Err(JourneyError::IdempotencyConflict)
        ));
        assert_eq!(
            JourneyEngine::load(&scope, &id)
                .or_fail("reload")
                .or_fail("retained")
                .status()
                .or_fail("still closed")
                .state(),
            JourneyState::Closed
        );
    }
    for retired in ["wallet-binding", "deposit", "withdraw", "exit"] {
        assert!(
            serde_json::from_value::<JourneyKind>(Value::String(retired.to_owned())).is_err(),
            "retired kind {retired} must not construct a new plan"
        );
    }
    assert_eq!(
        serde_json::from_str::<JourneyKind>("\"move\"").or_fail("live kind"),
        JourneyKind::Move
    );
    drop(store);
    std::fs::remove_dir_all(root).or_fail("remove test store");
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
