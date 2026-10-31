#![allow(dead_code)]

use std::collections::{BTreeMap, BTreeSet};
use std::future::Future;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::pin::pin;
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::boot::{Gate, GateError};
use layerx_agentd::config::{ReceiptKeySource, StartupConfig};
use layerx_agentd::prepare::{
    prepare_activity, prepare_activity_for_protocol, CorePreparationBoundary, CorePreparationState,
    CoreStateError, PreparationDefaults, PrepareRequest,
};
use layerx_agentd::protocol_evidence::{EvidenceAuthority, RawReceiptEvidence, RawStateEvidence};
use layerx_agentd::sign::{
    attach_kernel_signature, verify_kernel_before_submit, VerifiedSubmission,
};
use layerx_agentd::store::TenantId;
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::signer::{sign_disclosed, Signer};
use layerx_programs::hex;
use layerx_proof::merkle::build_proof;
use layerx_types::account::AccountId;
use layerx_types::activity::{Authority, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{Did, IdempotencyKey};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::verify::VerificationLevel;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{
    account_id_for_protocol, batch_header_digest, program_execution_batch_id, receipt_digest,
};
use layerx_wire::limits::{MAX_MESSAGE_BYTES, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest, Sha256};

#[path = "../../../../tests/support/monotonic_clock.rs"]
mod monotonic_clock;
mod real_authority;
pub(crate) mod send_authorization;
#[path = "../../../../tests/support/wall_clock.rs"]
mod wall_clock;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

struct NoopWake;

impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    let mut context = Context::from_waker(&waker);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local signing unexpectedly blocked"),
    }
}

struct RecordedCore(CorePreparationState);

impl CorePreparationBoundary for RecordedCore {
    fn preparation_state(&mut self, _actor: &Did) -> Result<CorePreparationState, CoreStateError> {
        Ok(self.0.clone())
    }
}

fn activity_type() -> ActivityType {
    ActivityType::new(ModuleId::Asset, 5).unwrap_or_else(|error| panic!("activity: {error:?}"))
}

fn registry() -> ModuleRegistry {
    ModuleRegistry::new(
        &[ModuleRegistration::new(ModuleId::Asset, &[activity_type()])
            .unwrap_or_else(|error| panic!("registration: {error:?}"))],
    )
    .unwrap_or_else(|error| panic!("registry: {error:?}"))
}

fn send_payload(id: u8) -> Vec<u8> {
    let mut encoder = Encoder::new(512);
    encoder
        .u16(0x5301)
        .unwrap_or_else(|error| panic!("tag: {error:?}"));
    encoder
        .u16(10)
        .unwrap_or_else(|error| panic!("fields: {error:?}"));
    for fixed in [[0x11; 32], [0x22; 32], [0x33; 32]] {
        encoder
            .fixed(&fixed)
            .unwrap_or_else(|error| panic!("fixed: {error:?}"));
    }
    encoder
        .u128(25)
        .unwrap_or_else(|error| panic!("amount: {error:?}"));
    encoder
        .u64(5)
        .unwrap_or_else(|error| panic!("sequence: {error:?}"));
    encoder
        .fixed(&[id; 32])
        .unwrap_or_else(|error| panic!("idempotency: {error:?}"));
    encoder
        .u64(1_010)
        .unwrap_or_else(|error| panic!("expiry: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("context: {error:?}"));
    encoder
        .u8(0)
        .unwrap_or_else(|error| panic!("conditions: {error:?}"));
    encoder
        .u8(1)
        .unwrap_or_else(|error| panic!("authority: {error:?}"));
    encoder
        .fixed(&[0x11; 32])
        .unwrap_or_else(|error| panic!("controller: {error:?}"));
    encoder
        .fixed(&[0x66; 32])
        .unwrap_or_else(|error| panic!("payload key: {error:?}"));
    encoder
        .fixed(&[0x77; 64])
        .unwrap_or_else(|error| panic!("payload signature: {error:?}"));
    encoder
        .fixed(&[0x55; 32])
        .unwrap_or_else(|error| panic!("signed context: {error:?}"));
    encoder
        .u32(17)
        .unwrap_or_else(|error| panic!("network: {error:?}"));
    encoder
        .u16(layerx_wire::limits::PROTOCOL_VERSION)
        .unwrap_or_else(|error| panic!("version: {error:?}"));
    send_authorization::sign(encoder.finish())
}

pub fn signed_transaction(id: u8) -> Vec<u8> {
    Eip1559Call {
        chain_id: 125,
        nonce: u64::from(id),
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 10,
        gas_limit: 100_000,
        to: [0x22; 20],
        data: vec![id],
    }
    .sign(&[0xa5; 32])
    .unwrap_or_else(|error| panic!("sign: {error:?}"))
}

pub fn verified_submission(id: u8) -> VerifiedSubmission {
    let signer = LocalSigner::new([0xa5; 32]);
    let mut core = RecordedCore(CorePreparationState {
        network_id: 17,
        account_sequence: 5,
        protocol_timestamp: 1_000,
        observed_head_sequence: 88,
        module_registry: registry(),
    });
    let prepared = prepare_activity(
        &mut core,
        PreparationDefaults {
            timestamp_span: 30,
            fee_limit: Amount::from_u128(12),
            maximum_payload_bytes: 1_024,
        },
        PrepareRequest {
            actor: Did::new(b"did:layerx:recovery")
                .unwrap_or_else(|error| panic!("DID: {error:?}")),
            authority: Authority::owner(&signer.public_key())
                .unwrap_or_else(|error| panic!("authority: {error:?}")),
            activity_type: activity_type(),
            expected_account_sequence: Some(5),
            timestamp_bound: Some(
                TimestampBound::new(995, 1_010)
                    .unwrap_or_else(|error| panic!("timestamp: {error:?}")),
            ),
            fee_limit: Some(Amount::from_u128(7)),
            idempotency_key: IdempotencyKey::new([id; 32]),
            payload: send_payload(id),
            declared_payload_limit: 1_024,
        },
    )
    .unwrap_or_else(|error| panic!("prepare: {error:?}"));
    let signature = ready(sign_disclosed(
        &signer,
        &prepared.canonical_bytes,
        &prepared.disclosure,
        &registry(),
    ))
    .unwrap_or_else(|error| panic!("sign: {error:?}"));
    let signed_bytes = attach_kernel_signature(&prepared, *signature.as_bytes())
        .unwrap_or_else(|error| panic!("attach: {error:?}"));
    verify_kernel_before_submit(&signed_bytes, &prepared, &signer.public_key(), &registry())
        .unwrap_or_else(|error| panic!("verify: {error:?}"))
}

pub const BUDGET_CREATE_ID: [u8; 32] = [0x0b; 32];
pub const BUDGET_CREATE_ACTOR: &str = "did:layerx:recovery";
pub const BUDGET_CREATE_PURPOSE: [u8; 32] = [0x54; 32];
pub const BUDGET_CREATE_PERIOD_LENGTH: u64 = 1_000;
pub const BUDGET_CREATE_PERIOD_START: u64 = 80;
pub const BUDGET_CREATE_SEQUENCE: u64 = 5;
const BUDGET_CREATE_PROTOCOL: u16 = 3;

fn budget_create_activity_type() -> ActivityType {
    ActivityType::new(ModuleId::Budget, 1).unwrap_or_else(|error| panic!("activity: {error:?}"))
}

fn protocol_account(name: &str) -> [u8; 32] {
    let account = AccountId::parse(name).unwrap_or_else(|error| panic!("account: {error:?}"));
    account_id_for_protocol(&account, BUDGET_CREATE_PROTOCOL)
        .unwrap_or_else(|error| panic!("account id: {error:?}"))
}

/// Main account core resolves as the owner of a budget created by the test actor.
pub fn budget_create_owner() -> [u8; 32] {
    protocol_account(&format!("agent:{BUDGET_CREATE_ACTOR}:main"))
}

/// Budget account core derives for one budget identifier of the test actor.
pub fn budget_create_account(budget_id: [u8; 32]) -> [u8; 32] {
    let id: String = budget_id.iter().map(|byte| format!("{byte:02x}")).collect();
    protocol_account(&format!("agent:{BUDGET_CREATE_ACTOR}:budget:{id}"))
}

/// Module registry declaring the asset send used by `verified_submission` and
/// the budget create activity used by `budget_create_submission`.
pub fn budget_registry() -> ModuleRegistry {
    ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Asset, &[activity_type()])
            .unwrap_or_else(|error| panic!("registration: {error:?}")),
        ModuleRegistration::new(ModuleId::Budget, &[budget_create_activity_type()])
            .unwrap_or_else(|error| panic!("registration: {error:?}")),
    ])
    .unwrap_or_else(|error| panic!("registry: {error:?}"))
}

/// Canonical 211-byte version-1 core budget-create payload. Core keys the
/// record by its `budget_id` and funds it from the actor's main account.
pub fn budget_create_payload(
    budget_id: [u8; 32],
    asset: [u8; 32],
    per_period_limit: u128,
    expiry_ms: u64,
) -> Vec<u8> {
    let mut bytes = Vec::with_capacity(211);
    bytes.extend_from_slice(&1_u16.to_be_bytes());
    bytes.extend_from_slice(&budget_id);
    bytes.extend_from_slice(&budget_create_account(budget_id));
    bytes.extend_from_slice(&asset);
    bytes.extend_from_slice(&BUDGET_CREATE_PURPOSE);
    bytes.extend_from_slice(&per_period_limit.to_be_bytes());
    bytes.extend_from_slice(&0_u128.to_be_bytes());
    bytes.extend_from_slice(&per_period_limit.to_be_bytes());
    bytes.extend_from_slice(&BUDGET_CREATE_PERIOD_LENGTH.to_be_bytes());
    bytes.extend_from_slice(&BUDGET_CREATE_PERIOD_START.to_be_bytes());
    bytes.extend_from_slice(&expiry_ms.to_be_bytes());
    bytes.extend_from_slice(&0_u64.to_be_bytes());
    bytes.push(1);
    assert_eq!(bytes.len(), 211);
    bytes
}

/// Prepares, externally signs and verifies one canonical protocol-3 budget-create
/// activity exactly as the daemon does for an owner-supplied signature.
pub fn budget_create_submission(
    id: u8,
    budget_id: [u8; 32],
    asset: [u8; 32],
    per_period_limit: u128,
    expiry_ms: u64,
) -> VerifiedSubmission {
    let registry = budget_registry();
    let signer = LocalSigner::new([0xa5; 32]);
    let mut core = RecordedCore(CorePreparationState {
        network_id: 17,
        account_sequence: BUDGET_CREATE_SEQUENCE,
        protocol_timestamp: 1_000,
        observed_head_sequence: 88,
        module_registry: registry.clone(),
    });
    let prepared = prepare_activity_for_protocol(
        &mut core,
        PreparationDefaults {
            timestamp_span: 30,
            fee_limit: Amount::from_u128(12),
            maximum_payload_bytes: 1_024,
        },
        PrepareRequest {
            actor: Did::new(BUDGET_CREATE_ACTOR.as_bytes())
                .unwrap_or_else(|error| panic!("DID: {error:?}")),
            authority: Authority::owner(&signer.public_key())
                .unwrap_or_else(|error| panic!("authority: {error:?}")),
            activity_type: budget_create_activity_type(),
            expected_account_sequence: Some(BUDGET_CREATE_SEQUENCE),
            timestamp_bound: Some(
                TimestampBound::new(995, 1_010)
                    .unwrap_or_else(|error| panic!("timestamp: {error:?}")),
            ),
            fee_limit: Some(Amount::from_u128(7)),
            idempotency_key: IdempotencyKey::new([id; 32]),
            payload: budget_create_payload(budget_id, asset, per_period_limit, expiry_ms),
            declared_payload_limit: 1_024,
        },
        BUDGET_CREATE_PROTOCOL,
    )
    .unwrap_or_else(|error| panic!("prepare: {error:?}"));
    let signature = ready(sign_disclosed(
        &signer,
        &prepared.canonical_bytes,
        &prepared.disclosure,
        &registry,
    ))
    .unwrap_or_else(|error| panic!("sign: {error:?}"));
    let signed_bytes = attach_kernel_signature(&prepared, *signature.as_bytes())
        .unwrap_or_else(|error| panic!("attach: {error:?}"));
    verify_kernel_before_submit(&signed_bytes, &prepared, &signer.public_key(), &registry)
        .unwrap_or_else(|error| panic!("verify: {error:?}"))
}

/// Encodes the 278-byte canonical version-1 core budget record core stores under
/// `budget_state_key(budget_id)` after executing `budget_create_payload`.
pub fn core_budget_record_for(
    budget_id: [u8; 32],
    asset: [u8; 32],
    per_period_limit: u128,
    expiry_ms: u64,
) -> Vec<u8> {
    let mut bytes = vec![0_u8; 278];
    bytes[1] = 1;
    bytes[2..34].copy_from_slice(&budget_id);
    bytes[34..66].copy_from_slice(&budget_create_owner());
    bytes[66..98].copy_from_slice(&budget_create_account(budget_id));
    bytes[98..130].copy_from_slice(&asset);
    bytes[130..162].copy_from_slice(&BUDGET_CREATE_PURPOSE);
    bytes[162..178].copy_from_slice(&per_period_limit.to_be_bytes());
    bytes[178..194].copy_from_slice(&per_period_limit.to_be_bytes());
    bytes[194..210].copy_from_slice(&0_u128.to_be_bytes());
    bytes[210..226].copy_from_slice(&0_u128.to_be_bytes());
    bytes[226..242].copy_from_slice(&0_u128.to_be_bytes());
    bytes[242..250].copy_from_slice(&BUDGET_CREATE_PERIOD_LENGTH.to_be_bytes());
    bytes[250..258].copy_from_slice(&BUDGET_CREATE_PERIOD_START.to_be_bytes());
    bytes[258..266].copy_from_slice(&expiry_ms.to_be_bytes());
    bytes[266..274].copy_from_slice(&0_u64.to_be_bytes());
    bytes[274] = 1;
    bytes
}

pub fn tenant() -> TenantId {
    TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}"))
}

pub fn directory(label: &str) -> std::path::PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-recovery-{label}-{}-{sequence}",
        std::process::id()
    ))
}

#[derive(Clone, Copy)]
pub struct TestAuthorityRecord {
    pub signing_seed: [u8; 32],
    pub epoch: u64,
    pub first_batch: u64,
    pub last_batch: u64,
    pub revoked_at_batch: Option<u64>,
}

#[derive(Clone, Copy)]
pub struct TestAuthorityPolicy<'a> {
    pub protocol_version: u16,
    pub network_id: u32,
    pub records: &'a [TestAuthorityRecord],
    pub handshake_signing_seed: [u8; 32],
    pub handshake_batch: u64,
}

pub fn evidence_authority(policy: TestAuthorityPolicy<'_>) -> EvidenceAuthority {
    try_evidence_authority(policy).unwrap_or_else(|error| panic!("evidence authority: {error:?}"))
}

pub fn evidence_authority_for_sequencer(
    policy: TestAuthorityPolicy<'_>,
    sequencer_id: [u8; 32],
) -> EvidenceAuthority {
    try_evidence_authority_with_sequencer(policy, Some(sequencer_id))
        .unwrap_or_else(|error| panic!("evidence authority: {error:?}"))
}

pub fn try_evidence_authority(
    policy: TestAuthorityPolicy<'_>,
) -> Result<EvidenceAuthority, GateError> {
    try_evidence_authority_with_sequencer(policy, None)
}

#[allow(clippy::too_many_lines)]
fn try_evidence_authority_with_sequencer(
    policy: TestAuthorityPolicy<'_>,
    sequencer_id: Option<[u8; 32]>,
) -> Result<EvidenceAuthority, GateError> {
    try_evidence_authority_mode(policy, sequencer_id, false)
}

pub fn try_evidence_authority_after_restart(
    policy: TestAuthorityPolicy<'_>,
) -> Result<EvidenceAuthority, GateError> {
    try_evidence_authority_mode(policy, None, true)
}

fn try_evidence_authority_mode(
    policy: TestAuthorityPolicy<'_>,
    sequencer_id: Option<[u8; 32]>,
    restart: bool,
) -> Result<EvidenceAuthority, GateError> {
    let authority_path = directory("evidence-authority").with_extension("csv");
    let mut authority_source = "layerx-sequencer-authority-v1\n".to_owned();
    for record in policy.records {
        let public_key = SigningKey::from_bytes(&record.signing_seed)
            .verifying_key()
            .to_bytes();
        authority_source.push_str(&hex::encode(&sequencer_id.unwrap_or(public_key)));
        authority_source.push(',');
        authority_source.push_str(&hex::encode(&public_key));
        authority_source.push(',');
        authority_source.push_str(&record.epoch.to_string());
        authority_source.push(',');
        authority_source.push_str(&record.first_batch.to_string());
        authority_source.push(',');
        authority_source.push_str(&record.last_batch.to_string());
        authority_source.push(',');
        authority_source.push_str(
            &record
                .revoked_at_batch
                .map_or_else(|| "active".to_owned(), |batch| batch.to_string()),
        );
        authority_source.push('\n');
    }
    std::fs::write(&authority_path, authority_source)
        .unwrap_or_else(|error| panic!("authority source: {error}"));
    std::fs::set_permissions(&authority_path, std::fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("authority source permissions: {error}"));
    let tenant = tenant();
    let readiness_node = real_authority::ReadinessNode::new();
    let config = StartupConfig {
        network_id: policy.network_id,
        node_endpoint: readiness_node.config.url.clone(),
        expected_chain_id: readiness_node.config.chain_id,
        tenants: BTreeSet::from([tenant.clone()]),
        policy_sources: BTreeMap::from([(
            tenant.clone(),
            PathBuf::from("/etc/layerx/policy-a.kvx"),
        )]),
        signer_configurations: BTreeMap::from([(
            tenant.clone(),
            PathBuf::from("/etc/layerx/signer-a.kvx"),
        )]),
        verification_defaults: BTreeMap::from([(tenant, VerificationLevel::STATE_PROVEN)]),
        receipt_key_source: ReceiptKeySource::PinnedFile(authority_path),
    };
    let mut gate = Gate::new(&config)?;
    real_authority::authorize(
        real_authority::Clock {
            wall_time: wall_clock::wall_time,
            monotonic_time: monotonic_clock::monotonic_time,
        },
        &mut gate,
        policy,
        sequencer_id,
        restart.then_some(&config),
        &readiness_node,
    )?;
    Ok(gate
        .evidence_authority()
        .unwrap_or_else(|error| panic!("write-ready evidence authority: {error:?}"))
        .clone())
}

pub fn pinned_evidence_verifier() -> EvidenceAuthority {
    EvidenceAuthority::pinned_to_handshake(
        PROTOCOL_VERSION,
        42,
        SigningKey::from_bytes(&[0x3a; 32])
            .verifying_key()
            .to_bytes(),
    )
    .unwrap_or_else(|error| panic!("pinned evidence authority: {error:?}"))
}

pub fn evidence_verifier() -> EvidenceAuthority {
    evidence_authority(TestAuthorityPolicy {
        protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
        network_id: 42,
        records: &[TestAuthorityRecord {
            signing_seed: [0x3a; 32],
            epoch: 2,
            first_batch: 7,
            last_batch: 7,
            revoked_at_batch: None,
        }],
        handshake_signing_seed: [0x3a; 32],
        handshake_batch: 7,
    })
}

pub fn raw_receipt(activity_id: [u8; 32], result_code: i32, amount: u128) -> RawReceiptEvidence {
    raw_receipt_at(activity_id, result_code, amount, 9)
}

pub fn raw_receipt_at(
    activity_id: [u8; 32],
    result_code: i32,
    amount: u128,
    global_sequence: u64,
) -> RawReceiptEvidence {
    raw_receipt_for_execution_batch(activity_id, result_code, amount, global_sequence, 7)
}

#[allow(clippy::too_many_lines)]
pub fn raw_receipt_for_execution_batch(
    activity_id: [u8; 32],
    result_code: i32,
    amount: u128,
    global_sequence: u64,
    execution_batch_number: u64,
) -> RawReceiptEvidence {
    let key = SigningKey::from_bytes(&[0x3a; 32]);
    let previous_state_root = [0x21; 32];
    let resulting_state_root = [0x22; 32];
    let batch_id = program_execution_batch_id(
        previous_state_root,
        [0x25; 32],
        global_sequence,
        global_sequence,
        execution_batch_number,
    )
    .unwrap_or_else(|error| panic!("execution batch id: {error:?}"));
    let asset = [0x24; 32];
    let encode = |signature: Option<[u8; 64]>| {
        let mut encoder = Encoder::new(4096);
        assert_eq!(
            encoder.structure_header_version(0x5201, layerx_wire::limits::PROTOCOL_VERSION,),
            Ok(())
        );
        assert_eq!(encoder.u16(layerx_wire::limits::PROTOCOL_VERSION), Ok(()));
        assert_eq!(encoder.bytes(&activity_id, 32), Ok(()));
        assert_eq!(encoder.u64(global_sequence), Ok(()));
        assert_eq!(encoder.bytes(&previous_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&resulting_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&[0x25; 32], 32), Ok(()));
        assert_eq!(encoder.i32(result_code), Ok(()));
        assert_eq!(encoder.sequence_length(0, 512), Ok(()));
        assert_eq!(encoder.u128(1), Ok(()));
        assert_eq!(encoder.bytes(&batch_id, 32), Ok(()));
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u8(1), Ok(()));
        assert_eq!(encoder.bytes(&asset, 32), Ok(()));
        assert_eq!(encoder.u128(amount), Ok(()));
        assert_eq!(encoder.bytes(&[0x26; 32], 32), Ok(()));
        assert_eq!(encoder.u128(1_000 + amount), Ok(()));
        assert_eq!(
            encoder.u128(if result_code == 0 {
                1_000
            } else {
                1_000 + amount
            }),
            Ok(())
        );
        assert_eq!(encoder.u64(1), Ok(()));
        assert_eq!(encoder.bytes(&[0x27; 32], 32), Ok(()));
        assert_eq!(encoder.u128(10), Ok(()));
        assert_eq!(
            encoder.u128(if result_code == 0 { 10 + amount } else { 10 }),
            Ok(())
        );
        for value in [[0x28; 32], [0x29; 32], [0x2a; 32]] {
            assert_eq!(encoder.bytes(&value, 32), Ok(()));
        }
        assert_eq!(encoder.u64(1_000), Ok(()));
        assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
        if let Some(signature) = signature {
            assert_eq!(encoder.bytes(&signature, 64), Ok(()));
        }
        encoder.finish()
    };
    let unsigned = encode(None);
    let digest =
        receipt_digest(&unsigned).unwrap_or_else(|error| panic!("receipt digest: {error:?}"));
    let canonical_receipt = encode(Some(key.sign(&digest).to_bytes()));
    let leaves = [canonical_receipt.as_slice()];
    let (proof, receipt_root) =
        build_proof(&leaves, 0).unwrap_or_else(|error| panic!("receipt proof: {error:?}"));
    let sequencer_id = key.verifying_key().to_bytes();
    let mut encoder = Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, layerx_wire::limits::PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    let fields: [(u8, Vec<u8>); 15] = [
        (
            1,
            layerx_wire::limits::PROTOCOL_VERSION.to_be_bytes().to_vec(),
        ),
        (2, 42_u32.to_be_bytes().to_vec()),
        (3, 2_u64.to_be_bytes().to_vec()),
        (4, 7_u64.to_be_bytes().to_vec()),
        (5, global_sequence.to_be_bytes().to_vec()),
        (6, global_sequence.to_be_bytes().to_vec()),
        (7, previous_state_root.to_vec()),
        (8, resulting_state_root.to_vec()),
        (9, [0x25; 32].to_vec()),
        (10, receipt_root.to_vec()),
        (11, [0x34; 32].to_vec()),
        (12, [0x35; 32].to_vec()),
        (13, [0x36; 32].to_vec()),
        (14, 1_000_u64.to_be_bytes().to_vec()),
        (15, sequencer_id.to_vec()),
    ];
    for (field, value) in fields {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        match field {
            1 => assert_eq!(
                encoder.u16(u16::from_be_bytes([value[0], value[1]])),
                Ok(())
            ),
            2 => assert_eq!(
                encoder.u32(u32::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("u32 field"))
                )),
                Ok(())
            ),
            3..=6 | 14 => assert_eq!(
                encoder.u64(u64::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("u64 field"))
                )),
                Ok(())
            ),
            _ => assert_eq!(encoder.bytes(&value, 32), Ok(())),
        }
    }
    let header = encoder.finish();
    let header_digest =
        batch_header_digest(&header).unwrap_or_else(|error| panic!("header digest: {error:?}"));
    RawReceiptEvidence::new(
        canonical_receipt,
        proof,
        header,
        key.sign(&header_digest).to_bytes(),
    )
}

pub fn corrupt_raw_receipt(
    raw: &RawReceiptEvidence,
    canonical_receipt: Vec<u8>,
) -> RawReceiptEvidence {
    RawReceiptEvidence::new(
        canonical_receipt,
        raw.proof().clone(),
        raw.canonical_header().to_vec(),
        raw.header_signature(),
    )
}

pub fn raw_state_leaf(canonical_state: Vec<u8>, observed_head: u64) -> RawStateEvidence {
    raw_state_leaf_with(
        canonical_state,
        observed_head,
        StateHeaderIdentity {
            signing_seed: [0x3a; 32],
            protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
            network_id: 42,
            epoch: 2,
            batch_number: 7,
        },
    )
}

#[derive(Clone, Copy)]
pub struct StateHeaderIdentity {
    pub signing_seed: [u8; 32],
    pub protocol_version: u16,
    pub network_id: u32,
    pub epoch: u64,
    pub batch_number: u64,
}

pub fn raw_state_leaf_with(
    canonical_state: Vec<u8>,
    observed_head: u64,
    identity: StateHeaderIdentity,
) -> RawStateEvidence {
    let sequencer_id = SigningKey::from_bytes(&identity.signing_seed)
        .verifying_key()
        .to_bytes();
    raw_state_leaf_with_sequencer_id(canonical_state, observed_head, identity, sequencer_id)
}

pub fn raw_state_leaf_with_sequencer_id(
    canonical_state: Vec<u8>,
    observed_head: u64,
    identity: StateHeaderIdentity,
    sequencer_id: [u8; 32],
) -> RawStateEvidence {
    let leaves = [canonical_state.as_slice()];
    let (proof, root) =
        build_proof(&leaves, 0).unwrap_or_else(|error| panic!("state proof: {error:?}"));
    let (header, signature) = signed_batch_header(identity, sequencer_id, observed_head, root);
    RawStateEvidence::new(canonical_state, proof, root, header, signature)
}

/// Builds and signs one canonical 15-field batch header whose resulting state
/// root is `resulting_state_root`, returning the header bytes and signature.
pub fn signed_batch_header(
    identity: StateHeaderIdentity,
    sequencer_id: [u8; 32],
    observed_head: u64,
    resulting_state_root: [u8; 32],
) -> (Vec<u8>, [u8; 64]) {
    let key = SigningKey::from_bytes(&identity.signing_seed);
    let mut encoder = Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, identity.protocol_version),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    let fields: [(u8, Vec<u8>); 15] = [
        (1, identity.protocol_version.to_be_bytes().to_vec()),
        (2, identity.network_id.to_be_bytes().to_vec()),
        (3, identity.epoch.to_be_bytes().to_vec()),
        (4, identity.batch_number.to_be_bytes().to_vec()),
        (5, 1_u64.to_be_bytes().to_vec()),
        (6, observed_head.to_be_bytes().to_vec()),
        (7, [0x31; 32].to_vec()),
        (8, resulting_state_root.to_vec()),
        (9, [0x32; 32].to_vec()),
        (10, [0x33; 32].to_vec()),
        (11, [0x34; 32].to_vec()),
        (12, [0x35; 32].to_vec()),
        (13, [0x36; 32].to_vec()),
        (14, 1_000_u64.to_be_bytes().to_vec()),
        (15, sequencer_id.to_vec()),
    ];
    for (field, value) in fields {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        match field {
            1 => assert_eq!(
                encoder.u16(u16::from_be_bytes([value[0], value[1]])),
                Ok(())
            ),
            2 => assert_eq!(
                encoder.u32(u32::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("u32 field"))
                )),
                Ok(())
            ),
            3..=6 | 14 => assert_eq!(
                encoder.u64(u64::from_be_bytes(
                    value
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("u64 field"))
                )),
                Ok(())
            ),
            _ => assert_eq!(encoder.bytes(&value, 32), Ok(())),
        }
    }
    let header = encoder.finish();
    let digest =
        batch_header_digest(&header).unwrap_or_else(|error| panic!("header digest: {error:?}"));
    (header, key.sign(&digest).to_bytes())
}

pub fn corrupt_raw_state(raw: &RawStateEvidence, canonical_state: Vec<u8>) -> RawStateEvidence {
    let (Some(proof), Some(resulting_state_root), Some(canonical_header), Some(header_signature)) = (
        raw.proof().cloned(),
        raw.resulting_state_root(),
        raw.canonical_header().map(<[u8]>::to_vec),
        raw.header_signature(),
    ) else {
        panic!("corrupt_raw_state requires state-leaf evidence");
    };
    RawStateEvidence::new(
        canonical_state,
        proof,
        resulting_state_root,
        canonical_header,
        header_signature,
    )
}

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

pub fn rpc_hex(bytes: &[u8]) -> String {
    format!("0x{}", hex::encode(bytes))
}

const HEIGHT: u64 = 150;
const KEY_ID: [u8; 32] = [1; 32];

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn encoded_receipt(
    tx_hash: [u8; 32],
    sender: [u8; 20],
    nonce: u64,
    success: bool,
    signature: Option<[u8; 64]>,
) -> Vec<u8> {
    let previous = [4; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&7_u64.to_be_bytes());
    sequence.extend_from_slice(&tx_hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&HEIGHT.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&sender);
    let mut to = [0; 32];
    to[12..].fill(0x22);
    let vm_error = if success { "" } else { "execution reverted" };
    let mut summary = Encoder::new(256);
    must(summary.u64(21_000));
    must(summary.u64(42_000));
    must(summary.u64(1));
    must(summary.u32(1));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&[0; 20]));
    must(summary.fixed(&sha256(vm_error.as_bytes())));
    must(summary.u32(2));
    must(summary.u32(1));
    must(summary.u8(4));
    must(summary.fixed(b"upax"));
    let mut log = vec![8; 20];
    log.push(1);
    log.extend_from_slice(&[9; 32]);
    log.extend_from_slice(&sha256(&[10, 11]));

    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    must(encoder.structure_header_version(0x5201, PROTOCOL_VERSION));
    must(encoder.u16(PROTOCOL_VERSION));
    must(encoder.bytes(&tx_hash, 32));
    must(encoder.u64(7));
    must(encoder.bytes(&previous, 32));
    must(encoder.bytes(&sha256(&sequence), 32));
    must(encoder.bytes(&sha256(&[0xab, 0xcd]), 32));
    must(encoder.i32(i32::from(!success)));
    must(encoder.sequence_length(2, 512));
    for (ordinal, kind, body) in [(0, 1, summary.finish()), (1, 3, log)] {
        must(encoder.u16(12));
        must(encoder.u16(ordinal));
        must(encoder.u16(0));
        must(encoder.u8(kind));
        must(encoder.u8(0));
        must(encoder.bytes(&[0; 32], 32));
        must(encoder.bytes(&body, 256));
    }
    must(encoder.u128(21_000));
    must(encoder.bytes(&height, 32));
    must(encoder.u16(12));
    must(encoder.u32(1));
    must(encoder.u32(0));
    must(encoder.u8(2));
    must(encoder.bytes(&sha256(b"upax"), 32));
    must(encoder.u128(1));
    must(encoder.bytes(&from, 32));
    must(encoder.u128(21_002));
    must(encoder.u128(if success { 1 } else { 2 }));
    must(encoder.u64(nonce));
    must(encoder.bytes(&to, 32));
    must(encoder.u128(0));
    must(encoder.u128(u128::from(success)));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&[0; 32], 32));
    must(encoder.bytes(&KEY_ID, 32));
    must(encoder.u64(1_000));
    must(encoder.u8(u8::from(signature.is_some())));
    if let Some(signature) = signature {
        must(encoder.bytes(&signature, 64));
    }
    encoder.finish()
}

pub fn sequencer_receipt(raw: &[u8], success: bool) -> (Value, ReceiptKeyEntry) {
    let (transaction, signature) = must(decode_signed(raw));
    let sender = must(recover_sender(&transaction, &signature));
    let tx_hash = transaction_hash(raw);
    let nonce = transaction.nonce;
    let signer = SigningKey::from_bytes(&[8; 32]);
    let unsigned = encoded_receipt(tx_hash, sender, nonce, success, None);
    let signature = signer.sign(&must(receipt_digest(&unsigned))).to_bytes();
    let bytes = encoded_receipt(tx_hash, sender, nonce, success, Some(signature));
    let key = ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: signer.verifying_key().to_bytes(),
        first_height: 100,
        last_height: 199,
    };
    let view = json!({
        "rung": "receipted", "txHash": rpc_hex(&tx_hash), "nonce": format!("{nonce:#x}"),
        "seq": "0x7", "height": "0x96", "index": "0x1", "timestamp": "0x3e8",
        "status": if success { "0x1" } else { "0x0" }, "returnData": "0xabcd",
        "logs": [{"address": rpc_hex(&[8; 20]), "topics": [rpc_hex(&[9; 32])],
            "data": "0x0a0b", "logIndex": "0x0"}],
        "gasUsed": "0x5208", "cumulativeGasUsed": "0xa410", "effectiveGasPrice": "0x1",
        "feeDenom": "upax", "feeAmount": "0x5208", "sponsor": rpc_hex(&[0; 20]),
        "from": rpc_hex(&sender), "to": rpc_hex(&[0x22; 20]), "contractAddress": rpc_hex(&[0; 20]),
        "vmError": if success { "" } else { "execution reverted" },
        "keyId": rpc_hex(&KEY_ID), "receipt": rpc_hex(&bytes),
        "signingKey": rpc_hex(&key.public_key)
    });
    (view, key)
}

pub type Exchange = (&'static str, Value, Option<Value>);

#[derive(Default)]
pub struct SequencerTraffic {
    pub lookups: usize,
    pub resends: usize,
    pub transmitted: Vec<([u8; 32], Vec<u8>)>,
}

pub fn serve(exchanges: Vec<Exchange>) -> (SequencerEndpoint, JoinHandle<SequencerTraffic>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let server = thread::spawn(move || {
        let mut traffic = SequencerTraffic::default();
        for (method, params, result) in exchanges {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing {method} request");
                        thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(2))));
            must(stream.set_write_timeout(Some(Duration::from_secs(2))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let length = length.unwrap_or_else(|| panic!("content length required"));
            assert!(length <= 64 * 1024);
            let mut body = vec![0; length];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], params);
            if method == "pax_sequencerReceipt" {
                traffic.lookups += 1;
            }
            if method == "eth_sendRawTransaction" {
                traffic.resends += 1;
                let encoded = request["params"][0]
                    .as_str()
                    .unwrap_or_else(|| panic!("raw transaction required"));
                let bytes = must(::hex::decode(encoded.strip_prefix("0x").unwrap_or(encoded)));
                traffic.transmitted.push((transaction_hash(&bytes), bytes));
            }
            let Some(result) = result else {
                continue;
            };
            let answer =
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result}).to_string();
            let mut stream = reader.into_inner();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()));
            must(stream.flush());
        }
        traffic
    });
    (must(SequencerEndpoint::new(&url, &[])), server)
}
