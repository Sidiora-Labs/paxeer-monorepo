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

#[test]
fn stored_journey_of_a_retired_kind_reads_as_closed() {
    let root = directory("journey-retired-kind");
    let retention = retention_uniform(10_000);
    let map = tenancy(&[("alice", "alice-tenant")]);
    let (mut store, digest) = install_and_open(&root, &map, retention);
    let owner = principal("alice");
    let id = JourneyId::new("jrn_retiredwithdraw").expect("journey id");
    let activity = ActivityType::new(ModuleId::Budget, 8).expect("activity");
    let registry = ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Budget, &[activity]).expect("module")
    ])
    .expect("registry");
    let intent = Intent::v1(IntentKind::BudgetDefund(
        BudgetDefund::new(
            BudgetId::new([0x21; 32]),
            AccountId::parse("agent:did:layerx:alice:budget:travel").expect("budget account"),
            AccountId::parse("agent:did:layerx:alice:main").expect("owner account"),
            AssetId::new([0x22; 32]),
            Amount::from_u128(37),
            Sequence::from_u64(7),
            IdempotencyKey::new([0x31; 32]),
        )
        .expect("intent"),
    ));
    let plan = JourneyPlan::new(
        id.clone(),
        JourneyKind::Move,
        [0x41; 32],
        KeyId::new("human-primary").expect("custody key"),
        Operation::ProtocolMutation,
        vec![JourneyLeg::new(
            intent,
            [0x31; 32],
            AgentDid::new("did:layerx:alice").expect("actor"),
            AuthorityRef::new("custody-human-primary").expect("authority"),
            7,
            19,
            100,
            200,
            1_000,
        )
        .expect("leg")],
    )
    .expect("plan");
    {
        let mut scope = store.principal(&owner).expect("scope");
        let live = JourneyEngine::start(&mut scope, &plan, &registry, 100).expect("persist plan");
        assert_eq!(
            live.status().expect("live status").state(),
            JourneyState::GettingReady
        );
        let row = scope
            .keys(Table::Journeys)
            .into_iter()
            .find(|key| key.as_str().starts_with("journey-"))
            .expect("journey row");
        let mut historical: Value = serde_json::from_slice(
            scope
                .get(Table::Journeys, &row)
                .expect("stored plan")
                .bytes(),
        )
        .expect("production record");
        assert_eq!(historical["legs"][0]["preparation"]["account_sequence"], 7);
        assert_eq!(historical["legs"][0]["preparation"]["pending_nonce"], 19);
        historical["kind"] = Value::String("withdraw".to_owned());
        historical["legs"][0]["preparation"]
            .as_object_mut()
            .expect("preparation")
            .remove("pending_nonce");
        scope
            .put(
                Table::Journeys,
                row,
                100,
                serde_json::to_vec(&historical).expect("legacy encoding"),
            )
            .expect("persist historical record");
    }
    drop(store);
    let mut store = PrincipalStore::open(&root, retention, digest).expect("restart store");
    {
        let mut scope = store.principal(&owner).expect("reopened scope");
        let historical = JourneyEngine::load(&scope, &id)
            .expect("load historical")
            .expect("journey");
        assert_eq!(historical.kind(), None);
        assert_eq!(historical.historical_kind(), Some("withdraw"));
        let status = historical.status().expect("historical status");
        assert_eq!(status.state(), JourneyState::Closed);
        assert_eq!(status.phases(), &[JourneyPhase::Compiled]);
        assert_eq!(status.receipt_digests(), &[None]);
        let listed = JourneyEngine::list(&scope).expect("list historical");
        assert_eq!(listed.len(), 1);
        assert_eq!(
            listed[0].status().expect("listed status").state(),
            JourneyState::Closed
        );
        assert!(matches!(
            JourneyEngine::start(&mut scope, &plan, &registry, 101),
            Err(JourneyError::IdempotencyConflict)
        ));
        assert_eq!(
            JourneyEngine::load(&scope, &id)
                .expect("reload")
                .expect("retained")
                .status()
                .expect("still closed")
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
        serde_json::from_str::<JourneyKind>("\"move\"").expect("live kind"),
        JourneyKind::Move
    );
    drop(store);
    std::fs::remove_dir_all(root).expect("remove test store");
}
