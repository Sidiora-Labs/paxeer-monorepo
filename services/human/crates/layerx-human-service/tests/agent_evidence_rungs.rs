use layerx_human_service::agents::{
    AgentCreationError, AgentEvidence, CreateAgentRequest, CreationContext, CreationJourney,
    CreationStage, PurposePresetCatalog,
};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, tenancy,
};
use layerx_types::payload::{ActivityType, ModuleId};
use layerx_types::rung::Rung;
use serde_json::{json, Value};

#[test]
fn agent_creation_is_complete_only_at_committed() {
    for stage in [
        CreationStage::SessionProvision,
        CreationStage::CapabilityNarrowing,
    ] {
        let mut evidence = AgentEvidence {
            action_key: [1; 32],
            object_id: [2; 32],
            observed_sequence: 42,
            rung: Rung::Submitted,
            receipt_digest: [0; 32],
        };
        assert!(!evidence.creation_complete());
        evidence.rung = Rung::Receipted;
        evidence.receipt_digest = evidence.expected_digest(stage);
        assert!(
            !evidence.creation_complete(),
            "receipted creation remains pending"
        );
        let pending_digest = evidence.receipt_digest;
        evidence.rung = Rung::Committed;
        evidence.receipt_digest = evidence.expected_digest(stage);
        assert!(
            evidence.creation_complete(),
            "committed creation is complete"
        );
        assert_ne!(
            pending_digest, evidence.receipt_digest,
            "evidence binds its rung"
        );
        evidence.rung = Rung::Receipted;
        assert!(
            !evidence.creation_complete(),
            "completion depends on the current rung"
        );
    }
}

fn config() -> Value {
    let counterparty = [17_u8; 32];
    let asset = [34_u8; 32];
    json!({
        "version": "selectors-v1",
        "presets": [{
            "id": "payments",
            "call_selectors": ["0xa9059cbb", "0x095ea7b3"],
            "counterparties": [counterparty],
            "assets": [asset],
            "amount_ceiling": 1000,
            "rate_maximum_uses": 12,
            "rate_window_sequences": 50,
            "purposes": ["payments"],
            "expiry_sequence": 50000,
            "session_scopes": ["prepare", "submit"],
            "session_lifetime_seconds": 5000,
            "budget_asset": asset,
            "budget_period_seconds": 2592000,
            "budget_expiry_seconds": 31536000,
            "initial_funding": 250
        }]
    })
}

fn parse(value: &Value) -> Result<PurposePresetCatalog, AgentCreationError> {
    let bytes = serde_json::to_vec(value).unwrap_or_else(|error| panic!("JSON: {error}"));
    PurposePresetCatalog::from_json(&bytes)
}

#[test]
fn purpose_preset_with_activity_types_is_refused() {
    let valid = config();
    let catalog = parse(&valid).unwrap_or_else(|error| panic!("selectors: {error}"));
    assert_eq!(catalog.version(), "selectors-v1");
    assert_eq!(
        catalog
            .call_selectors("payments")
            .unwrap_or_else(|error| panic!("selectors: {error}")),
        vec![[0xa9, 0x05, 0x9c, 0xbb], [0x09, 0x5e, 0xa7, 0xb3]]
    );
    assert!(matches!(
        catalog.call_selectors("unknown"),
        Err(AgentCreationError::UnknownPurpose)
    ));
    let activity = ActivityType::new(ModuleId::Asset, 5).expect("native Send activity");
    let request = CreateAgentRequest::new("Payments agent", "payments", 100, "LXP")
        .expect("creation request")
        .with_activity_types(vec![activity])
        .expect("explicit kernel activity types");
    assert!(matches!(
        request.clone().with_activity_types(Vec::new()),
        Err(AgentCreationError::InvalidRequest)
    ));
    assert!(matches!(
        request
            .clone()
            .with_activity_types(vec![activity, activity]),
        Err(AgentCreationError::InvalidRequest)
    ));
    let root = directory("agent-evidence-rungs");
    let (mut store, _) = install_and_open(
        &root,
        &tenancy(&[("alice", "tenant-alice")]),
        retention_uniform(100),
    );
    let alice = principal("alice");
    let mut scope = store.principal(&alice).expect("principal scope");
    let context = CreationContext {
        idempotency_key: [1; 32],
        owner_account: "agent:did:layerx:alice:main".to_owned(),
        human_recovery_root: [2; 32],
        recovery_threshold: 1,
        network_id: 77,
        protocol_time: 1000,
    };
    let journey = CreationJourney::start(&mut scope, &request, &context, &catalog, 1)
        .expect("selector preset with explicit kernel activities");
    let loaded = CreationJourney::load(&scope, journey.agent_id())
        .expect("load journey")
        .expect("retained journey");
    assert_eq!(loaded.projection().activity_types, vec![activity.value()]);
    assert_eq!(
        loaded.projection().call_selectors,
        vec!["0xa9059cbb", "0x095ea7b3"]
    );
    let changed = request
        .clone()
        .with_activity_types(vec![
            ActivityType::new(ModuleId::Asset, 6).expect("asset activity")
        ])
        .expect("changed kernel activity types");
    assert!(matches!(
        CreationJourney::start(&mut scope, &changed, &context, &catalog, 1),
        Err(AgentCreationError::IdempotencyConflict)
    ));
    drop(scope);
    drop(store);
    std::fs::remove_dir_all(&root).expect("remove test store");
    let mut legacy = valid.clone();
    legacy["presets"][0]["activity_types"] = json!([65541]);
    assert!(matches!(
        parse(&legacy),
        Err(AgentCreationError::InvalidPresetConfig)
    ));
    legacy["presets"][0]
        .as_object_mut()
        .unwrap_or_else(|| panic!("preset object"))
        .remove("call_selectors");
    assert!(matches!(
        parse(&legacy),
        Err(AgentCreationError::InvalidPresetConfig)
    ));
    for selectors in [
        json!([]),
        json!(["a9059cbb"]),
        json!(["0xa9059c"]),
        json!(["0xa9059cbb00"]),
        json!(["0xa9059cgg"]),
        json!(["0xa9059cbb", "0xA9059CBB"]),
        json!([65541]),
    ] {
        let mut invalid = valid.clone();
        invalid["presets"][0]["call_selectors"] = selectors;
        assert!(matches!(
            parse(&invalid),
            Err(AgentCreationError::InvalidPresetConfig)
        ));
    }
}
