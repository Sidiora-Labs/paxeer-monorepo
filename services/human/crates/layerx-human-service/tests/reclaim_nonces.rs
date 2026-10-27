use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_human_service::agents::{
    Reclaim, ReclaimAgentContext, ReclaimError, ReclaimMechanism, ReclaimRequest, ReclaimStage,
};
use layerx_human_service::custody::KeyId;
use layerx_human_service::journeys::JourneyEngine;
use layerx_human_service::notify::JourneyId;
use layerx_human_service::store::{PrincipalScope, PrincipalStore, Table};
use layerx_human_test_support::{directory, principal, retention_uniform, tenancy};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::AssetId;
use layerx_types::intent::{BudgetId, Sequence};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

fn registry() -> ModuleRegistry {
    ModuleRegistry::new(&[ModuleRegistration::new(
        ModuleId::Budget,
        &[ActivityType::new(ModuleId::Budget, 7).expect("defund activity")],
    )
    .expect("budget registration")])
    .expect("module registry")
}

fn request(key: u8, nonce: u64, agent: &str) -> ReclaimRequest {
    ReclaimRequest {
        journey_id: JourneyId::new(format!("jrn_reclaimnonce{key}")).expect("journey id"),
        idempotency_key: [key; 32],
        owner: AccountId::parse("agent:did:layerx:human:main").expect("owner account"),
        agent_account: AccountId::parse(&format!("agent:did:layerx:{agent}:main"))
            .expect("agent account"),
        asset: AssetId::new([0x33; 32]),
        amount: Amount::from_u128(1),
        mechanism: ReclaimMechanism::BudgetDefund {
            budget_account: AccountId::parse(&format!(
                "agent:did:layerx:{agent}:budget:operations"
            ))
            .expect("budget account"),
            budget_id: BudgetId::new([0x41; 32]),
            revocation_sequence: Sequence::from_u64(4),
        },
        agent: ReclaimAgentContext {
            label: "worker".to_owned(),
            actor: AgentDid::new(format!("did:layerx:{agent}")).expect("agent DID"),
            authority: AuthorityRef::new("custody-human-primary").expect("authority"),
            custody_key: KeyId::new("human-primary").expect("custody key"),
            nonce,
            not_before: 995,
            not_after: 1_010,
            fee_limit: 7,
        },
    }
}

fn assert_pending_nonce(scope: &PrincipalScope<'_>, request: &ReclaimRequest) {
    let records: Vec<serde_json::Value> = scope
        .keys(Table::Journeys)
        .iter()
        .filter(|key| key.as_str().starts_with("journey-"))
        .map(|key| {
            serde_json::from_slice(
                scope
                    .get(Table::Journeys, key)
                    .expect("journey row")
                    .bytes(),
            )
            .expect("stored journey JSON")
        })
        .filter(|record: &serde_json::Value| {
            record["journey_id"].as_str() == Some(request.journey_id.as_str())
        })
        .collect();
    assert_eq!(records.len(), 1);
    let legs = records[0]["legs"].as_array().expect("journey legs");
    assert_eq!(legs.len(), 1);
    assert_eq!(
        legs[0]["preparation"]["account_sequence"].as_u64(),
        Some(request.agent.nonce())
    );
    assert_eq!(
        legs[0]["preparation"]["actor"].as_str(),
        Some(request.agent.actor.as_str())
    );
    assert!(JourneyEngine::load(scope, &request.journey_id)
        .expect("valid durable engine")
        .is_some());
}

#[test]
fn reclaim_starts_with_the_pending_nonce() {
    let root = directory("reclaim-pending-nonce");
    let digest = tenancy(&[("alice", "tenant-a")])
        .install(&root)
        .expect("install tenancy");
    let owner = principal("alice");
    let mut store = PrincipalStore::open(&root, retention_uniform(10_000), digest)
        .expect("open principal store");
    {
        let mut scope = store.principal(&owner).expect("principal scope");
        for (key, nonce) in [(1, 0), (2, 42), (3, u64::MAX)] {
            let request = request(key, nonce, "worker");
            let reclaim = Reclaim::start(&mut scope, &request, &registry(), 1_000)
                .expect("start with pending nonce");
            assert_eq!(
                reclaim.status().expect("reclaim status").stage(),
                ReclaimStage::GettingReady
            );
            assert_pending_nonce(&scope, &request);
            assert_eq!(
                reclaim,
                Reclaim::start(&mut scope, &request, &registry(), 1_000)
                    .expect("idempotent restart with same nonce")
            );
        }
    }
    drop(store);
    let mut reopened = PrincipalStore::open(&root, retention_uniform(10_000), digest)
        .expect("reopen principal store");
    let scope = reopened.principal(&owner).expect("reopened scope");
    assert_pending_nonce(&scope, &request(3, u64::MAX, "worker"));
    assert!(
        Reclaim::load(&scope, &request(3, u64::MAX, "worker").journey_id)
            .expect("reload reclaim")
            .is_some()
    );
    drop(scope);
    drop(reopened);
    std::fs::remove_dir_all(root).expect("remove fixture");
}

#[test]
fn reclaim_refuses_a_stale_nonce() {
    let root = directory("reclaim-stale-nonce");
    let digest = tenancy(&[("alice", "tenant-a")])
        .install(&root)
        .expect("install tenancy");
    let owner = principal("alice");
    {
        let mut store = PrincipalStore::open(&root, retention_uniform(10_000), digest)
            .expect("open principal store");
        let mut scope = store.principal(&owner).expect("principal scope");
        Reclaim::start(&mut scope, &request(1, 42, "worker"), &registry(), 1_000)
            .expect("record first pending nonce");
    }
    let mut store = PrincipalStore::open(&root, retention_uniform(10_000), digest)
        .expect("reopen principal store");
    let mut scope = store.principal(&owner).expect("reopened scope");
    let mut stale = request(2, 41, "worker");
    stale.agent.label = "renamed-worker".to_owned();
    let keys_before = scope.keys(Table::Journeys);
    assert!(matches!(
        Reclaim::start(&mut scope, &stale, &registry(), 1_001),
        Err(ReclaimError::StaleNonce {
            nonce: 41,
            last_used: 42
        })
    ));
    assert_eq!(scope.keys(Table::Journeys), keys_before);
    assert!(Reclaim::load(&scope, &stale.journey_id)
        .expect("lookup refused reclaim")
        .is_none());
    assert!(JourneyEngine::load(&scope, &stale.journey_id)
        .expect("lookup refused engine")
        .is_none());

    let other = request(3, 1, "another");
    Reclaim::start(&mut scope, &other, &registry(), 1_001)
        .expect("another agent has an independent nonce");
    assert_pending_nonce(&scope, &other);
    let equal = request(4, 42, "worker");
    Reclaim::start(&mut scope, &equal, &registry(), 1_002)
        .expect("equal pending nonce is not stale");
    assert_pending_nonce(&scope, &equal);
    let newer = request(5, 45, "worker");
    Reclaim::start(&mut scope, &newer, &registry(), 1_003).expect("advance the agent nonce");
    assert_pending_nonce(&scope, &newer);
    assert!(matches!(
        Reclaim::start(&mut scope, &request(6, 44, "worker"), &registry(), 1_004),
        Err(ReclaimError::StaleNonce {
            nonce: 44,
            last_used: 45
        })
    ));
    drop(scope);
    drop(store);
    std::fs::remove_dir_all(root).expect("remove fixture");
}
