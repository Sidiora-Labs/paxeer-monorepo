use std::collections::BTreeSet;
use std::fmt::Debug;
use std::fs;
use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agent_api::error::RequestId;
use layerx_agentd::identity::ProtocolAuthority;
use layerx_agentd::session::{
    invalidate_on_revocation, InvalidationReason, OpenRequest, PendingActivity, PreparationState,
    RevocationEvent, SessionCredential, SessionId, SessionRecord, SessionRef, SessionRegistry,
};
use layerx_agentd::session_keys::{ProvisionedEvmSessionKey, SessionKeyRegistry};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_client::sequencer::{SequencerLog, TransactionReceipt};
use layerx_crypto::session::{
    issue_session_key, IssuedSessionKey, SessionKeyRequest, SessionPurpose,
};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId};

static NEXT: AtomicU64 = AtomicU64::new(1);
const AUTH: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1c,
];

fn must<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("session grant operation: {error:?}"))
}

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "layerxauth-session-grants-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        must(fs::create_dir(&path));
        must(fs::set_permissions(
            &path,
            fs::Permissions::from_mode(0o700),
        ));
        Self(path)
    }

    fn keys(&self) -> SessionKeyRegistry {
        must(SessionKeyRegistry::open(
            self.0.clone(),
            vec![0x61; 32],
            77,
            must(fs::metadata(&self.0)).uid(),
        ))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        must(fs::remove_dir_all(&self.0));
    }
}

fn principal() -> Did {
    must(Did::new(b"did:layerx:session-grants"))
}

fn provision(directory: &Directory) -> (IssuedSessionKey, ProvisionedEvmSessionKey) {
    let keys = directory.keys();
    let seed = must(keys.prepare_seed(b"session-grants", [0x31; 32]));
    let mut address_word = [0; 32];
    address_word[12..].copy_from_slice(&seed.evm_address());
    let issued = must(issue_session_key(&SessionKeyRequest {
        grantor: must(layerx_wire::hash::did_id_for_protocol(&principal(), 3)),
        session_public_key: address_word,
        not_before: 100,
        expires_at: Some(1000),
        permitted_activity_types: vec![must(ActivityType::new(ModuleId::Asset, 5))],
        revocation_sequence: Some(3),
        fee_budget: None,
        purpose: SessionPurpose::Activity,
    }));
    must(keys.provision(issued.grant_id, seed.secret(), &issued));
    let key = must(keys.load(issued.grant_id, issued.clone()));
    (issued, key)
}

// A persisted LXSR05 local session is the migration input; no identity resolver or node double.
fn persisted_session(store: &mut Store, id: u8, grant: [u8; 32]) -> SessionRecord {
    let record = SessionRecord {
        request: OpenRequest {
            session_id: SessionId([id; 32]),
            token_id: [id + 20; 32],
            tenant: must(TenantId::new("session-grants")),
            agent: principal(),
            authority: ProtocolAuthority::SessionKey(grant),
            permitted_activity_types: BTreeSet::from([5]),
            scopes: BTreeSet::from(["prepare".to_owned()]),
            expiry_sequence: 100,
            expiry_seconds: None,
            opening_client: "sdk".to_owned(),
            policy_version: "v1".to_owned(),
        },
        open: true,
        sequence: 0,
        budget_reserved: 0,
        subscription_cursor: 0,
        generation: 1,
        retired_token_ids: BTreeSet::new(),
    };
    let mut bytes = b"LXSR05".to_vec();
    bytes.extend(record.request.session_id.0);
    bytes.extend(record.request.token_id);
    bytes.extend(record.request.expiry_sequence.to_be_bytes());
    bytes.push(0); // No public expiry.
    bytes.push(1); // Open.
    bytes.extend(record.sequence.to_be_bytes());
    bytes.extend(record.budget_reserved.to_be_bytes());
    bytes.extend(record.subscription_cursor.to_be_bytes());
    bytes.extend(record.generation.to_be_bytes());
    for text in [
        record.request.opening_client.as_bytes(),
        record.request.policy_version.as_bytes(),
        record.request.agent.as_bytes(),
    ] {
        bytes.extend(must(u16::try_from(text.len())).to_be_bytes());
        bytes.extend(text);
    }
    bytes.push(2); // SessionKey authority.
    bytes.extend(grant);
    bytes.extend(1_u16.to_be_bytes());
    bytes.extend(5_u16.to_be_bytes());
    bytes.extend(1_u16.to_be_bytes());
    bytes.extend(7_u16.to_be_bytes());
    bytes.extend(b"prepare");
    bytes.extend(0_u16.to_be_bytes());
    let key = must(TenantKey::new(
        record.request.tenant.clone(),
        ObjectKind::Session,
        record.request.session_id.0.to_vec(),
    ));
    must(store.put_local(key, bytes));
    record
}

fn restore(store: &Store, record: &SessionRecord) -> SessionRegistry {
    let mut registry = SessionRegistry::default();
    must(registry.restore_tenant(store, &record.request.tenant));
    assert_eq!(
        registry.get(&record.request.tenant, record.request.session_id),
        Some(record)
    );
    registry
}

fn session_ref(record: &SessionRecord) -> SessionRef {
    SessionRef::new(record.request.tenant.clone(), record.request.session_id)
}

fn credential(record: &SessionRecord) -> SessionCredential {
    SessionCredential::new(
        record.request.tenant.clone(),
        record.request.session_id,
        record.request.token_id,
        record.generation,
    )
}

// Event vectors follow precompiles/layerxauth/abi.json and the keeper's canonical records.
fn receipt(event_topic: &str, grant_id: [u8; 32], record: &[u8]) -> TransactionReceipt {
    let mut data = vec![0; 64 + record.len().div_ceil(32) * 32];
    data[31] = 32;
    data[63] = must(u8::try_from(record.len()));
    data[64..64 + record.len()].copy_from_slice(record);
    TransactionReceipt {
        status: true,
        block_number: 12,
        transaction_index: 1,
        gas_used: 5000,
        logs: vec![SequencerLog {
            address: AUTH,
            topics: vec![must(must(hex::decode(event_topic)).try_into()), grant_id],
            data,
            log_index: 0,
        }],
    }
}

fn revoke_receipt(grant: [u8; 32]) -> TransactionReceipt {
    let mut record = grant.to_vec();
    record.push(1);
    record.extend(43_u64.to_be_bytes());
    receipt(
        "56ae84eff6bf7f5e201e33f3e37e1e3bf3f3470dacdb55801842589a3d156419",
        grant,
        &record,
    )
}

fn grant_receipt(issued: &IssuedSessionKey) -> TransactionReceipt {
    let mut summary = b"LXGS2".to_vec();
    summary.extend(issued.grant_id);
    summary.extend(issued.grantor);
    summary.extend([0x41; 32]); // Owner authority.
    summary.extend([0x42; 32]); // Action key.
    summary.extend(issued.session_public_key);
    summary.extend(100_u64.to_be_bytes());
    summary.extend((1_u64 << (ModuleId::Asset as u16)).to_be_bytes());
    summary.extend(5_u16.to_be_bytes());
    summary.extend(5_u16.to_be_bytes());
    summary.extend(issued.not_before.to_be_bytes());
    summary.extend(issued.expires_at.to_be_bytes());
    summary.extend(issued.revocation_sequence.to_be_bytes());
    assert_eq!(summary.len(), 209);
    receipt(
        "cd2f11433d26596e4c5d214c177bd03c817c9e19d9850ee61bda3442aa0e3b9a",
        issued.grant_id,
        &summary,
    )
}

#[test]
fn revoke_grant_receipt_invalidates_pending_preparations() {
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let key_directory = Directory::new();
    let (issued, key) = provision(&key_directory);
    let record = persisted_session(&mut store, 1, issued.grant_id);
    let sibling = persisted_session(&mut store, 2, [0x55; 32]);
    let mut registry = restore(&store, &record);
    must(registry.record_grant_session_receipt(
        &mut store,
        &session_ref(&record),
        &key,
        &grant_receipt(&issued),
    ));
    let bearer = must(registry.authenticate(&credential(&record)));
    let states = [
        PreparationState::Prepared,
        PreparationState::Signed,
        PreparationState::Queued,
        PreparationState::Unknown,
        PreparationState::NonceConsumed,
        PreparationState::Executed,
        PreparationState::Failed,
    ];
    let mut pending: Vec<_> = states
        .into_iter()
        .map(|state| PendingActivity {
            session: session_ref(&record),
            state,
            cancelled: false,
            resolution_continues: false,
        })
        .collect();
    pending.push(PendingActivity {
        session: session_ref(&sibling),
        state: PreparationState::Prepared,
        cancelled: false,
        resolution_continues: false,
    });
    let valid = revoke_receipt(issued.grant_id);
    let event = must(RevocationEvent::from_revoke_grant_receipt(
        principal(),
        issued.grant_id,
        &valid,
    ));
    assert_eq!(event.reason, InvalidationReason::GrantRevokedOnChain);
    assert_eq!(event.observed_sequence, valid.block_number);
    let mut malformed = Vec::new();
    let mut failed = valid.clone();
    failed.status = false;
    malformed.push(failed);
    let mut foreign = valid.clone();
    foreign.logs[0].address[19] ^= 1;
    malformed.push(foreign);
    let mut wrong_grant = valid.clone();
    wrong_grant.logs[0].topics[1][0] ^= 1;
    malformed.push(wrong_grant);
    let mut wrong_record = valid.clone();
    wrong_record.logs[0].data[64] ^= 1;
    malformed.push(wrong_record);
    let mut duplicate = valid.clone();
    duplicate.logs.push(valid.logs[0].clone());
    malformed.push(duplicate);
    let mut padding = valid.clone();
    padding.logs[0].data[127] = 1;
    malformed.push(padding);
    let mut truncated = valid.clone();
    truncated.logs[0].data.clear();
    malformed.push(truncated);
    for receipt in malformed {
        assert!(
            RevocationEvent::from_revoke_grant_receipt(principal(), issued.grant_id, &receipt)
                .is_err()
        );
    }
    let report = must(invalidate_on_revocation(
        &mut store,
        &mut registry,
        &mut pending,
        &event,
    ));
    assert_eq!(report.reason, InvalidationReason::GrantRevokedOnChain);
    assert_eq!(report.invalidated_sessions, vec![session_ref(&record)]);
    assert_eq!(report.cancelled_preparations, 4);
    assert_eq!(report.unresolved_left_for_resolution, 3);
    assert_eq!(report.executed_untouched, 2);
    assert!(pending[..4].iter().all(|activity| activity.cancelled));
    assert!(pending[4..].iter().all(|activity| !activity.cancelled));
    assert!(pending[2..5]
        .iter()
        .all(|activity| activity.resolution_continues));
    assert!(bearer.boundary(&registry).is_err());
    assert_eq!(
        must(registry.authenticate(&credential(&sibling))).generation(),
        1
    );
    assert!(
        matches!(registry.session_refresh(&mut store, &credential(&record), [0x71; 32], 13, RequestId(1)),
        Err(error) if error.reason.as_str() == "session.revoked")
    );
    let closed = must(
        registry
            .get(&record.request.tenant, record.request.session_id)
            .ok_or("session"),
    )
    .clone();
    assert!(!closed.open);
    assert_eq!(closed.generation, 2);
    drop(store);
    let store = must(Store::open(&directory.0));
    let restored = restore(&store, &closed);
    assert!(restored.authenticate(&credential(&record)).is_err());
}

#[test]
fn grant_session_receipt_is_required_before_refresh() {
    let directory = Directory::new();
    let key_directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let (issued, key) = provision(&key_directory);
    let record = persisted_session(&mut store, 1, issued.grant_id);
    let sibling = persisted_session(&mut store, 2, issued.grant_id);
    let mut registry = restore(&store, &record);
    let original = credential(&record);
    assert_eq!(
        key.evm_address().as_slice(),
        &issued.session_public_key[12..]
    );
    must(key.with_secret(|secret| assert_ne!(secret, &[0; 32])));
    let error = match registry.session_refresh(&mut store, &original, [0x61; 32], 13, RequestId(9))
    {
        Err(error) => error,
        Ok(_) => panic!("refresh accepted a provisioned key without a grantSession receipt"),
    };
    assert_eq!(error.reason.as_str(), "session.grant_missing");
    assert_eq!(error.request_id, RequestId(9));
    assert_eq!(
        registry.get(&record.request.tenant, record.request.session_id),
        Some(&record)
    );
    let valid = grant_receipt(&issued);
    let mut invalid = vec![revoke_receipt(issued.grant_id)];
    let mut reverted = valid.clone();
    reverted.status = false;
    invalid.push(reverted);
    let mut address = valid.clone();
    address.logs[0].data[64 + 145] ^= 1;
    invalid.push(address);
    let mut principal = valid.clone();
    principal.logs[0].data[64 + 37] ^= 1;
    invalid.push(principal);
    for receipt in invalid {
        assert!(registry
            .record_grant_session_receipt(&mut store, &session_ref(&record), &key, &receipt)
            .is_err());
        let error = registry.session_refresh(&mut store, &original, [0x61; 32], 13, RequestId(9));
        assert!(matches!(error, Err(error) if error.reason.as_str() == "session.grant_missing"));
    }
    must(registry.record_grant_session_receipt(&mut store, &session_ref(&record), &key, &valid));
    must(registry.record_grant_session_receipt(&mut store, &session_ref(&record), &key, &valid));
    assert!(
        matches!(registry.session_refresh(&mut store, &credential(&sibling), [0x62; 32], 13, RequestId(9)),
        Err(error) if error.reason.as_str() == "session.grant_missing")
    );
    drop(store);
    let mut store = must(Store::open(&directory.0));
    let mut registry = restore(&store, &record);
    let refreshed =
        must(registry.session_refresh(&mut store, &original, [0x61; 32], 13, RequestId(10)));
    assert_eq!(refreshed.generation(), 2);
    assert!(registry.authenticate(&original).is_err());
    assert!(registry.authenticate(&refreshed.credential()).is_ok());
    assert_eq!(
        registry
            .get(&record.request.tenant, record.request.session_id)
            .map(|r| &r.request.scopes),
        Some(&record.request.scopes)
    );
    assert!(
        matches!(registry.session_refresh(&mut store, &refreshed.credential(), [0x63; 32], 100, RequestId(11)),
        Err(error) if error.reason.as_str() == "session.expired")
    );
    assert!(registry.authenticate(&refreshed.credential()).is_ok());
}
