use std::collections::BTreeSet;
use std::fs;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::audit::{
    export, read_entries, record, redact, review, verify_chain, verify_chain_material, AuditExport,
    Coverage, DataClass, Decision, Entry, EventClass, EvidenceStore, ExportError, Log,
    OutputSurface, PayloadEvidence, Query,
};
use layerx_agentd::identity::ProtocolAuthority;
use layerx_agentd::session::SessionId;
use layerx_agentd::store::TenantId;
use layerx_agentd::tenant::{Config, RedactionPolicy, Retention};
use layerx_types::ids::{ActivityId, Did, IdempotencyKey};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use sha2::{Digest, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "layerx-repair-22-31-365-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        )))
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0)
            .unwrap_or_else(|error| panic!("remove audit directory: {error}"));
    }
}

fn config() -> Config {
    Config {
        tenant: TenantId::new("repair-22-31-365").unwrap_or_else(|error| panic!("tenant: {error}")),
        policy_version: "policy-1".into(),
        redaction: RedactionPolicy::Standard,
        retention: Retention {
            events: 10,
            audit: 1,
            receipts: 10,
        },
        verification_default: VerificationLevel::UNVERIFIED,
        approval_required_for: BTreeSet::new(),
    }
}

fn entry(config: &Config) -> Entry {
    Entry {
        class: EventClass::AdministrativeAction,
        observed_at_ms: 100,
        tenant: config.tenant.clone(),
        agent: Did::new(b"did:layerx:repair-actor")
            .unwrap_or_else(|error| panic!("agent: {error:?}")),
        session: Some(SessionId([1; 32])),
        capability: Some([2; 32]),
        policy_version: config.policy_version.clone(),
        request_id: [3; 32],
        idempotency_key: Some(IdempotencyKey::new([4; 32])),
        decision: Decision::Allowed,
        reason: redact(
            config,
            &config.tenant,
            OutputSurface::Audit,
            DataClass::PublicText,
            b"operator authorization accepted",
            0,
        )
        .unwrap_or_else(|error| panic!("reason: {error}"))
        .value,
        resulting_activity_id: Some(ActivityId::new([5; 32])),
        verification_level: VerificationLevel::UNVERIFIED,
        protocol_authority: Some(ProtocolAuthority::PrimaryKey([6; 32])),
        submitted_bytes: Some(PayloadEvidence::Digest(
            Sha256::digest(b"expired signed payload").into(),
        )),
        receipt_id: None,
        tx_hash: Some([7; 32]),
        nonce: Some(8),
        rung: Some(Rung::Submitted),
    }
}

fn query(config: &Config) -> Query {
    Query {
        tenant: config.tenant.clone(),
        agent: None,
        from_observed_at_ms: None,
        through_observed_at_ms: None,
    }
}

fn produced_record(item: &Entry) -> Vec<u8> {
    let directory = Directory::new();
    let mut log = Log::open(&directory.0, &item.tenant)
        .unwrap_or_else(|error| panic!("open producer: {error}"));
    let path = log.path().to_path_buf();
    let (_, durable) = record(&mut log, &mut Coverage::default(), item, || {
        read_entries(&path)
    })
    .unwrap_or_else(|error| panic!("record: {error}"));
    assert_eq!(
        durable.unwrap_or_else(|error| panic!("durable reader: {error}")),
        vec![item.clone()]
    );
    drop(log);
    let bytes = fs::read(path).unwrap_or_else(|error| panic!("read frame: {error}"));
    let length = u32::from_be_bytes(
        bytes[84..88]
            .try_into()
            .unwrap_or_else(|error| panic!("frame length: {error}")),
    ) as usize;
    bytes[88..88 + length].to_vec()
}

fn recovered_export(config: &Config) -> AuditExport {
    let directory = Directory::new();
    let original = entry(config);
    let mut log =
        Log::open(&directory.0, &config.tenant).unwrap_or_else(|error| panic!("open: {error}"));
    let mut expected = Vec::new();
    for index in 0..3 {
        let mut item = original.clone();
        item.observed_at_ms += index * 10;
        if index == 1 {
            item.submitted_bytes = Some(PayloadEvidence::Digest(
                Sha256::digest(b"recent signed payload").into(),
            ));
        } else if index == 2 {
            item.submitted_bytes = Some(PayloadEvidence::Digest(
                Sha256::digest(b"unselected signed payload").into(),
            ));
        }
        if index == 2 {
            item.agent = Did::new(b"did:layerx:other-actor")
                .unwrap_or_else(|error| panic!("other actor: {error:?}"));
        }
        record(&mut log, &mut Coverage::default(), &item, || ())
            .unwrap_or_else(|error| panic!("append: {error}"));
        expected.push(item);
    }
    let path = log.path().to_path_buf();
    let anchored = verify_chain(&path).unwrap_or_else(|error| panic!("anchor: {error}"));
    drop(log);
    let log =
        Log::open(&directory.0, &config.tenant).unwrap_or_else(|error| panic!("reopen: {error}"));
    assert_eq!(log.entries(), 3);
    assert_eq!(
        read_entries(log.path()).unwrap_or_else(|error| panic!("recover: {error}")),
        expected
    );
    let mut selection = query(config);
    selection.agent = Some(original.agent);
    let exported = export(
        log.path(),
        config,
        selection,
        &EvidenceStore::new(config.tenant.clone()),
    )
    .unwrap_or_else(|error| panic!("export recovered records: {error}"));
    assert_eq!(exported.chain.anchor_entries, anchored.entries);
    assert_eq!(exported.chain.anchor_tail_hash, anchored.tail_hash);
    assert_eq!(exported.chain.tenant_hash, anchored.tenant_hash);
    assert!(exported.referenced_evidence.is_empty());
    assert_eq!(
        review(&exported)
            .unwrap_or_else(|error| panic!("review: {error}"))
            .exported_entries,
        2
    );
    exported
}

fn assert_no_digest(exported: &AuditExport, digest: &[u8; 32]) {
    for link in &exported.chain.links {
        if let Some(bytes) = &link.canonical_entry_bytes {
            assert!(!bytes.windows(32).any(|window| window == digest));
        }
        if let Some(proof) = &link.redaction_proof {
            assert!(!proof
                .redacted_entry_bytes
                .windows(32)
                .any(|window| window == digest));
            assert_ne!(&proof.payload_commitment, digest);
        }
        assert_ne!(&link.payload_digest, digest);
        assert_ne!(&link.entry_hash, digest);
    }
}

#[test]
fn redacted_audit_entry_binding_authenticates_every_retained_field() {
    let config = config();
    let exported = recovered_export(&config);
    let original = &exported.entries[0].entry;
    assert_eq!(original.submitted_bytes, Some(PayloadEvidence::Redacted));
    let mut mutations = Vec::new();
    macro_rules! changed {
        ($field:ident, $value:expr) => {{
            let mut item = original.clone();
            item.$field = $value;
            mutations.push((stringify!($field), item));
        }};
    }
    changed!(class, EventClass::PolicyDecision);
    changed!(observed_at_ms, 101);
    changed!(
        tenant,
        TenantId::new("forged-tenant").unwrap_or_else(|error| panic!("tenant: {error}"))
    );
    changed!(
        agent,
        Did::new(b"did:layerx:forged-actor").unwrap_or_else(|error| panic!("DID: {error:?}"))
    );
    changed!(session, Some(SessionId([11; 32])));
    changed!(capability, Some([12; 32]));
    changed!(policy_version, "policy-forged".into());
    changed!(request_id, [13; 32]);
    changed!(idempotency_key, Some(IdempotencyKey::new([14; 32])));
    changed!(decision, Decision::Failed);
    changed!(
        reason,
        redact(
            &config,
            &config.tenant,
            OutputSurface::Audit,
            DataClass::PublicText,
            b"forged reason",
            0
        )
        .unwrap_or_else(|error| panic!("reason: {error}"))
        .value
    );
    changed!(resulting_activity_id, Some(ActivityId::new([15; 32])));
    changed!(verification_level, VerificationLevel::STATE_PROVEN);
    changed!(
        protocol_authority,
        Some(ProtocolAuthority::SessionKey([16; 32]))
    );
    changed!(receipt_id, Some([17; 32]));
    changed!(tx_hash, Some([18; 32]));
    changed!(nonce, Some(19));
    changed!(rung, Some(Rung::Committed));
    changed!(submitted_bytes, None);
    changed!(submitted_bytes, Some(PayloadEvidence::Digest([20; 32])));
    for (field, item) in mutations {
        let mut altered = exported.clone();
        altered.entries[0].entry = item.clone();
        assert!(
            review(&altered).is_err(),
            "unbound mutation accepted: {field}"
        );
        let proof = altered.chain.links[0]
            .redaction_proof
            .as_mut()
            .unwrap_or_else(|| panic!("missing proof"));
        proof.redacted_entry_bytes = produced_record(&item);
        assert!(
            verify_chain_material(&altered.chain).is_err(),
            "coordinated proof mutation accepted: {field}"
        );
        assert!(
            review(&altered).is_err(),
            "coordinated mutation accepted: {field}"
        );
    }
}

#[test]
fn redacted_audit_entry_binding_survives_restart_and_hides_expired_evidence() {
    let config = config();
    let exported = recovered_export(&config);
    assert!(exported.chain.links[0].canonical_entry_bytes.is_none());
    assert!(exported.chain.links[0].redaction_proof.is_some());
    assert!(matches!(
        exported.entries[1].entry.submitted_bytes,
        Some(PayloadEvidence::Digest(_))
    ));
    assert!(exported.chain.links[1].canonical_entry_bytes.is_some());
    assert!(exported.chain.links[1].redaction_proof.is_none());
    assert!(!exported.chain.links[2].selected);
    assert!(exported.chain.links[2].canonical_entry_bytes.is_none());
    assert!(exported.chain.links[2].redaction_proof.is_none());
    let digest: [u8; 32] = Sha256::digest(b"expired signed payload").into();
    assert_no_digest(&exported, &digest);
    let mut receipt_only = config.clone();
    receipt_only.redaction = RedactionPolicy::ReceiptOnly;
    let exported = recovered_export(&receipt_only);
    for item in &exported.entries {
        assert_eq!(item.entry.submitted_bytes, Some(PayloadEvidence::Redacted));
    }
    assert_no_digest(&exported, &digest);
}

#[test]
fn redacted_audit_entry_binding_rejects_missing_and_altered_proofs() {
    let exported = recovered_export(&config());
    let mut altered = exported.clone();
    altered.chain.links[0].redaction_proof = None;
    assert!(verify_chain_material(&altered.chain).is_err());
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[0]
        .redaction_proof
        .as_mut()
        .unwrap_or_else(|| panic!("proof"))
        .payload_commitment[0] ^= 1;
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[0].payload_digest[0] ^= 1;
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[0].payload_length += 1;
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[0].field_commitments = false;
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[0].canonical_entry_bytes =
        Some(produced_record(&exported.entries[0].entry));
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[2].redaction_proof = exported.chain.links[0].redaction_proof.clone();
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[0]
        .redaction_proof
        .as_mut()
        .unwrap_or_else(|| panic!("proof"))
        .redacted_entry_bytes
        .push(0);
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[2].payload_digest[0] ^= 1;
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.query.tenant =
        TenantId::new("forged-tenant").unwrap_or_else(|error| panic!("tenant: {error}"));
    altered.entries[0].entry.tenant = altered.query.tenant.clone();
    altered.entries[1].entry.tenant = altered.query.tenant.clone();
    assert!(review(&altered).is_err());
    let mut altered = exported.clone();
    altered.chain.links[1].canonical_entry_bytes = None;
    altered.entries[1].entry.submitted_bytes = Some(PayloadEvidence::Redacted);
    assert!(review(&altered).is_err());
    assert!(review(&exported).is_ok());
    let mut altered = exported.clone();
    let proof = altered.chain.links[0]
        .redaction_proof
        .as_mut()
        .unwrap_or_else(|| panic!("proof"));
    proof.payload_commitment[0] ^= 1;
    let mut root = Sha256::new();
    root.update(b"layerx-audit-record-fields-v1");
    root.update(Sha256::digest(&proof.redacted_entry_bytes));
    root.update(proof.payload_commitment);
    altered.chain.links[0].payload_digest = root.finalize().into();
    assert!(
        review(&altered).is_err(),
        "recomputed root must still match original chain hash"
    );
    let mut receipt_only = config();
    receipt_only.redaction = RedactionPolicy::ReceiptOnly;
    let mut altered = recovered_export(&receipt_only);
    altered.chain.links[0].redaction_proof = altered.chain.links[1].redaction_proof.clone();
    assert!(
        review(&altered).is_err(),
        "proofs must not transfer between records"
    );
}

#[test]
fn redacted_audit_entry_binding_detects_stored_evidence_tampering() {
    let config = config();
    let directory = Directory::new();
    let mut log =
        Log::open(&directory.0, &config.tenant).unwrap_or_else(|error| panic!("open: {error}"));
    let item = entry(&config);
    record(&mut log, &mut Coverage::default(), &item, || ())
        .unwrap_or_else(|error| panic!("record: {error}"));
    let path = log.path().to_path_buf();
    drop(log);
    let mut bytes = fs::read(&path).unwrap_or_else(|error| panic!("read: {error}"));
    let digest = <[u8; 32]>::from(Sha256::digest(b"expired signed payload"));
    let offset = bytes
        .windows(32)
        .position(|window| window == digest)
        .unwrap_or_else(|| panic!("producer did not persist evidence digest"));
    bytes[offset] ^= 1;
    fs::write(&path, bytes).unwrap_or_else(|error| panic!("alter stored evidence: {error}"));
    assert!(verify_chain(&path).is_err());
    assert!(Log::open(&directory.0, &config.tenant).is_err());
}

#[test]
fn redacted_audit_entry_binding_refuses_unprovable_legacy_redaction() {
    let config = config();
    let directory = Directory::new();
    let item = entry(&config);
    let payload = produced_record(&item);
    let mut log = Log::open(&directory.0, &config.tenant)
        .unwrap_or_else(|error| panic!("legacy log: {error}"));
    log.before_operation(&payload, || ())
        .unwrap_or_else(|error| panic!("legacy frame: {error}"));
    let path = log.path().to_path_buf();
    drop(log);
    let mut log = Log::open(&directory.0, &config.tenant)
        .unwrap_or_else(|error| panic!("reopen legacy log: {error}"));
    let evidence = EvidenceStore::new(config.tenant.clone());
    let retained = export(&path, &config, query(&config), &evidence)
        .unwrap_or_else(|error| panic!("retained legacy export: {error}"));
    assert!(!retained.chain.links[0].field_commitments);
    assert!(review(&retained).is_ok());
    for _ in 0..2 {
        record(&mut log, &mut Coverage::default(), &item, || ())
            .unwrap_or_else(|error| panic!("new committed frame: {error}"));
    }
    drop(log);
    let log = Log::open(&directory.0, &config.tenant)
        .unwrap_or_else(|error| panic!("reopen mixed frame log: {error}"));
    assert_eq!(
        read_entries(log.path()).unwrap_or_else(|error| panic!("mixed reader: {error}")),
        vec![item.clone(), item.clone(), item]
    );
    assert_eq!(
        export(log.path(), &config, query(&config), &evidence),
        Err(ExportError::RedactionProofUnavailable { sequence: 0 })
    );
}
