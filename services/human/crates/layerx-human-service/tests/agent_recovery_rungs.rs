use layerx_human_service::agents::{
    AgentKeyChallenge, AgentKeyChangeKind, AgentKeyChangeStage, AgentRecoveryError,
    ProtocolKeyChangeEvidence, ProtocolKeyChangeObservation, ProtocolKeyChangeState,
};
use layerx_human_service::store::{EvidenceRef, PrincipalScope, Table};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, row_key, tenancy,
};
use layerx_types::ids::Did;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const IDEMPOTENCY_KEY: [u8; 32] = [1; 32];
const PREVIOUS_KEY: [u8; 32] = [2; 32];
const PENDING_KEY: [u8; 32] = [3; 32];
const TX_HASH: [u8; 32] = [4; 32];
const AUTHORITY_DIGEST: [u8; 32] = [5; 32];
const HISTORY_KEY: &str = "agent-creation-receipt";

fn did(bytes: &[u8]) -> Did {
    Did::new(bytes).unwrap_or_else(|error| panic!("DID: {error:?}"))
}

fn hash_bytes(digest: &mut Sha256, bytes: &[u8]) {
    digest.update(u64::try_from(bytes.len()).unwrap().to_be_bytes());
    digest.update(bytes);
}

fn evidence() -> ProtocolKeyChangeEvidence {
    let mut evidence = ProtocolKeyChangeEvidence {
        kind: AgentKeyChangeKind::Recovery,
        did: did(b"did:layerx:agent"),
        recovery_authority: did(b"did:layerx:human"),
        previous_public_key: PREVIOUS_KEY,
        pending_public_key: PENDING_KEY,
        effective_at: 160,
        lapse_at: 220,
        effective_sequence: None,
        observed_at: 100,
        observed_sequence: 10,
        rung: Rung::Receipted,
        tx_hash: TX_HASH,
        receipt_digest: [0; 32],
        competing_rotation: None,
    };
    evidence.receipt_digest = evidence.expected_digest();
    evidence
}

fn record(evidence: &ProtocolKeyChangeEvidence) -> Value {
    let mut request_digest = Sha256::new();
    request_digest.update(b"layerx-human/agent-key-change-request/v1\0");
    request_digest.update([2]); // Recovery request kind.
    request_digest.update(IDEMPOTENCY_KEY);
    hash_bytes(&mut request_digest, b"agt_recoveryrungs");
    hash_bytes(&mut request_digest, evidence.did.as_bytes());
    hash_bytes(&mut request_digest, evidence.recovery_authority.as_bytes());
    request_digest.update(AUTHORITY_DIGEST);
    request_digest.update([1]); // Journeys table.
    hash_bytes(&mut request_digest, HISTORY_KEY.as_bytes());
    let request_digest: [u8; 32] = request_digest.finalize().into();
    json!({
        "version": 2,
        "request_digest": request_digest,
        "idempotency_key": IDEMPOTENCY_KEY,
        "agent_id": "agt_recoveryrungs",
        "did": evidence.did.as_bytes(),
        "recovery_authority": evidence.recovery_authority.as_bytes(),
        "authority_evidence_digest": AUTHORITY_DIGEST,
        "kind": "Recovery",
        "previous_public_key": evidence.previous_public_key,
        "pending_public_key": evidence.pending_public_key,
        "primary_public_key": evidence.previous_public_key,
        "superseded_public_key": null,
        "announced_at": evidence.observed_at,
        "challenge_delay_seconds": 60,
        "effective_at": evidence.effective_at,
        "lapse_at": evidence.lapse_at,
        "effective_sequence": evidence.effective_sequence,
        "stage": "ChallengeOpen",
        "announcement_receipt_digest": evidence.receipt_digest,
        "rung_rank": evidence.rung.wire_rank(),
        "tx_hash": evidence.tx_hash,
        "observed_at": evidence.observed_at,
        "observed_sequence": evidence.observed_sequence,
        "history": [{"table": 1, "key": HISTORY_KEY}],
        "competing_rotation": null,
        "created_at": 100,
        "updated_at": 100
    })
}

fn persist(scope: &mut PrincipalScope<'_>, record: &Value) {
    let key = format!("agent-key-change-{}", "01".repeat(32));
    scope
        .put(
            Table::Journeys,
            row_key(&key),
            160,
            serde_json::to_vec(record).unwrap(),
        )
        .unwrap();
}

#[test]
fn key_change_is_effective_only_at_committed() {
    let root = directory("agent-recovery-rungs");
    let map = tenancy(&[("alice", "tenant-a")]);
    let (mut store, _) = install_and_open(&root, &map, retention_uniform(1_000));
    let principal = principal("alice");
    let mut scope = store.principal(&principal).unwrap();
    scope
        .put(
            Table::Journeys,
            row_key(HISTORY_KEY),
            90,
            PREVIOUS_KEY.to_vec(),
        )
        .unwrap();
    let evidence = evidence();
    let mut record = record(&evidence);
    persist(&mut scope, &record);
    let pending = AgentKeyChallenge::load(&scope, IDEMPOTENCY_KEY)
        .unwrap()
        .unwrap();
    assert_eq!(pending.stage(), AgentKeyChangeStage::ChallengeOpen);
    assert_eq!(pending.rung(), Rung::Receipted);
    assert_eq!(pending.tx_hash(), TX_HASH);
    assert_eq!(pending.primary_public_key(), PREVIOUS_KEY);
    assert_eq!(pending.pending_public_key(), PENDING_KEY);
    assert_eq!(pending.superseded_public_key(), None);
    assert_eq!(
        pending.history(),
        &[EvidenceRef::new(Table::Journeys, row_key(HISTORY_KEY))]
    );

    record["stage"] = json!("ReadyToCommit");
    record["observed_at"] = json!(160);
    record["observed_sequence"] = json!(11);
    record["updated_at"] = json!(160);
    persist(&mut scope, &record);
    assert_eq!(
        AgentKeyChallenge::load(&scope, IDEMPOTENCY_KEY)
            .unwrap()
            .unwrap()
            .stage(),
        AgentKeyChangeStage::ReadyToCommit
    );

    let mut observation = ProtocolKeyChangeObservation {
        kind: evidence.kind,
        did: evidence.did.clone(),
        previous_public_key: PREVIOUS_KEY,
        pending_public_key: PENDING_KEY,
        primary_public_key: PENDING_KEY,
        superseded_public_key: Some(PREVIOUS_KEY),
        effective_at: evidence.effective_at,
        lapse_at: evidence.lapse_at,
        effective_sequence: None,
        state: ProtocolKeyChangeState::Effective,
        observed_at: 160,
        observed_sequence: 11,
        rung: Rung::Receipted,
        tx_hash: TX_HASH,
        competing_rotation: None,
    };
    record["stage"] = json!("Effective");
    record["primary_public_key"] = json!(observation.primary_public_key);
    record["superseded_public_key"] = json!(observation.superseded_public_key);
    record["rung_rank"] = json!(observation.rung.wire_rank());
    persist(&mut scope, &record);
    assert!(matches!(
        AgentKeyChallenge::load(&scope, IDEMPOTENCY_KEY),
        Err(AgentRecoveryError::Corrupt(
            "effective key-change state is inconsistent"
        ))
    ));

    observation.rung = Rung::Committed;
    record["rung_rank"] = json!(observation.rung.wire_rank());
    persist(&mut scope, &record);
    let effective = AgentKeyChallenge::load(&scope, IDEMPOTENCY_KEY)
        .unwrap()
        .unwrap();
    assert_eq!(effective.stage(), AgentKeyChangeStage::Effective);
    assert_eq!(effective.rung(), Rung::Committed);
    assert_eq!(effective.tx_hash(), observation.tx_hash);
    assert_eq!(effective.primary_public_key(), PENDING_KEY);
    assert_eq!(effective.superseded_public_key(), Some(PREVIOUS_KEY));
    assert_eq!(effective.did(), pending.did());
    assert_eq!(effective.history(), pending.history());

    let mut changed_evidence = evidence.clone();
    changed_evidence.tx_hash = [6; 32];
    assert_ne!(changed_evidence.expected_digest(), evidence.receipt_digest);
    changed_evidence = evidence.clone();
    changed_evidence.rung = Rung::Committed;
    assert_ne!(changed_evidence.expected_digest(), evidence.receipt_digest);
    drop(scope);
    drop(store);
    std::fs::remove_dir_all(root).unwrap();
}

#[test]
fn key_change_observation_rank_above_two_is_refused() {
    let root = directory("agent-recovery-invalid-rank");
    let map = tenancy(&[("alice", "tenant-a")]);
    let (mut store, _) = install_and_open(&root, &map, retention_uniform(1_000));
    let principal = principal("alice");
    let mut scope = store.principal(&principal).unwrap();
    let mut record = record(&evidence());
    for rank in [1, 2] {
        record["rung_rank"] = json!(rank);
        persist(&mut scope, &record);
        assert!(AgentKeyChallenge::load(&scope, IDEMPOTENCY_KEY).is_ok());
    }
    for rank in 3..=u8::MAX {
        record["rung_rank"] = json!(rank);
        persist(&mut scope, &record);
        assert!(
            matches!(
                AgentKeyChallenge::load(&scope, IDEMPOTENCY_KEY),
                Err(AgentRecoveryError::Corrupt("invalid rung rank"))
            ),
            "rank {rank} must be refused"
        );
    }
    drop(scope);
    drop(store);
    std::fs::remove_dir_all(root).unwrap();
}
