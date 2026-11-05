pub use layerx_agentd::{human, identity, managed_agent, session, store};

#[path = "../src/managed_agent_rotation.rs"]
mod managed_agent_rotation;

use identity::CoreIdentity;
use layerx_proof::receipt::AuthorizedBatch;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::verify::VerificationLevel;
use managed_agent::FinalizationEvidence;
use managed_agent_rotation::{prepare, OwnerRotation, Projection};
use session::{SessionId, SessionRegistry};
use std::fmt::Debug;
use store::{ObjectKind, Store, TenantId, TenantKey};

fn must<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("rotation refusal: {error:?}"))
}

fn active_session_bytes() -> Vec<u8> {
    let mut bytes = b"LXSR05".to_vec();
    bytes.extend_from_slice(&[1; 32]);
    bytes.extend_from_slice(&[2; 32]);
    bytes.extend_from_slice(&100_u64.to_be_bytes());
    bytes.push(0);
    bytes.push(1);
    bytes.extend_from_slice(&0_u64.to_be_bytes());
    bytes.extend_from_slice(&0_u128.to_be_bytes());
    bytes.extend_from_slice(&0_u64.to_be_bytes());
    bytes.extend_from_slice(&1_u64.to_be_bytes());
    for text in ["rotation-test", "v1", "did:layerx:rotation-agent"] {
        bytes.extend_from_slice(&must(u16::try_from(text.len())).to_be_bytes());
        bytes.extend_from_slice(text.as_bytes());
    }
    bytes.push(2);
    bytes.extend_from_slice(&[3; 32]);
    bytes.extend_from_slice(&1_u16.to_be_bytes());
    bytes.extend_from_slice(&7_u16.to_be_bytes());
    bytes.extend_from_slice(&1_u16.to_be_bytes());
    bytes.extend_from_slice(&7_u16.to_be_bytes());
    bytes.extend_from_slice(b"prepare");
    bytes.extend_from_slice(&0_u16.to_be_bytes());
    bytes
}

#[test]
fn owner_rotation_is_refused_as_unsupported() {
    let root = std::env::temp_dir().join(format!("layerx-rotation-refused-{}", std::process::id()));
    let tenant = must(TenantId::new("rotation-test"));
    let session_id = SessionId([1; 32]);
    let session_key = must(TenantKey::new(
        tenant.clone(),
        ObjectKind::Session,
        session_id.0.to_vec(),
    ));
    let mut store = must(Store::open(&root));
    must(store.put_local(session_key.clone(), active_session_bytes()));
    let mut sessions = SessionRegistry::default();
    must(sessions.restore_tenant(&store, &tenant));
    let session_before = sessions
        .get(&tenant, session_id)
        .cloned()
        .unwrap_or_else(|| panic!("restored session must exist"));
    assert!(session_before.open);
    let token = must(sessions.authenticate_bearer(&tenant, session_id, [2; 32]));
    let durable_before = must(std::fs::read(root.join("store.bin")));
    let record_before = store
        .get(&session_key)
        .unwrap_or_else(|| panic!("durable session must exist"))
        .bytes()
        .to_vec();
    let activity_type = must(ActivityType::new(ModuleId::Governance, 2));
    let registry = must(ModuleRegistry::new(&[must(ModuleRegistration::new(
        ModuleId::Governance,
        &[activity_type],
    ))]));
    let identity = CoreIdentity {
        canonical_bytes: Vec::new(),
        head_sequence: 0,
        revocation_sequence: 0,
        verification_level: VerificationLevel::UNVERIFIED,
        frozen: false,
        authorities: Vec::new(),
    };
    let authority = AuthorizedBatch::new([4; 32], [5; 32], [6; 32], [7; 32], [8; 32]);
    for signed_activity in [Vec::new(), vec![0xff; 32], vec![0; 16_385]] {
        let request = Projection {
            custody_key: "replacement-key",
            signed_activity: &signed_activity,
            evidence: FinalizationEvidence {
                action_key: [9; 32],
                activity_id: [10; 32],
                receipt_digest: [11; 32],
                observed_sequence: 10,
                verification: 4,
                finalized_at: 100,
            },
            identity: &identity,
            authority: &authority,
            registry: &registry,
        };
        let refusal = match prepare(&store, &sessions, &tenant, "managed-agent", &request) {
            Err(refusal) => refusal,
            Ok(_) => panic!("owner rotation must be refused before any decode"),
        };
        assert_eq!(refusal, OwnerRotation::Unsupported);
        assert_eq!(refusal.reason(), "rotation_unsupported");
        assert_eq!(refusal.to_string(), "rotation_unsupported");
        assert_eq!(sessions.get(&tenant, session_id), Some(&session_before));
        assert_eq!(
            must(sessions.authenticate_bearer(&tenant, session_id, [2; 32])).generation(),
            token.generation()
        );
        assert_eq!(
            store
                .get(&session_key)
                .unwrap_or_else(|| panic!("session removed"))
                .bytes(),
            record_before
        );
        assert_eq!(must(std::fs::read(root.join("store.bin"))), durable_before);
    }
    drop(store);
    let store = must(Store::open(&root));
    let mut restored = SessionRegistry::default();
    must(restored.restore_tenant(&store, &tenant));
    assert_eq!(restored.get(&tenant, session_id), Some(&session_before));
    drop(store);
    must(std::fs::remove_dir_all(root));
}
