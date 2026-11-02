//! Proves: SendDebit/Prepared/ApprovalEvents -> public Rust signing API and Store -> Outbox/idempotency/EventIngestor consumers used by Human.
//!
//! The custody and journey_faults callers attach and verify native EVM signatures;
//! move_money also consumes the kernel owner-signature API. key_export consumes
//! Prepared canonical bytes and disclosure, intent_submit consumes the raw
//! transaction and its hash, and native_journey_approval reopens tenant storage.
//! These tests use those same public items, real signed Send producers, the real
//! durable store, and a real approval event producer; no boundary is substituted.
//!
//! Repairs: src/idempotency.rs::Store::open previously forgot replay state unless
//! callers supplied every old key to restore. src/prepare/mod.rs::verify_disclosure_binding
//! previously accepted a typed envelope or cached signer preimage inconsistent
//! with canonical bytes. src/outbox/mod.rs::Outbox::restore now validates retained
//! signed transactions and their activity identity before exposing them.

use std::fmt::Debug;
use std::future::Future;
use std::path::PathBuf;
use std::pin::pin;
use std::sync::atomic::{AtomicU64, Ordering};
use std::task::{Context, Poll, Waker};

use layerx_agentd::approval::{ApprovalEventKind, ApprovalEvents, ApprovalLifecycle};
use layerx_agentd::audit::{Coverage, Log};
use layerx_agentd::events::{ingest_local_restriction, CoreEvent, EventIngestor, IngestError};
use layerx_agentd::idempotency::{EconomicResult, IdempotencyError, Outcome, RetentionPolicy};
use layerx_agentd::outbox::{Outbox, OutboxError, SubmissionState};
use layerx_agentd::prepare::{
    evm, restore_canonical, verify_disclosure_binding, DisclosureBindingError, Prepared,
    RestorePreparationError,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::sign::{
    attach_external_signature, attach_kernel_signature, verify_before_submit,
    verify_kernel_before_submit, SigningError, SigningPreparation,
};
use layerx_agentd::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::send::{encode_send_envelope, EnvelopeOptions, SendDebit};
use layerx_crypto::signer::{Signer, SigningRequest};
use layerx_crypto::SignatureMessage;
use layerx_types::activity::Authority;
use layerx_types::ids::Did;
use layerx_types::payload::ModuleRegistry;
use layerx_types::verify::VerificationLevel;
use layerx_wire::hash::Domain;

static NEXT: AtomicU64 = AtomicU64::new(0);
const KEY: [u8; 32] = [4; 32];
const SEED: [u8; 32] = [0x66; 32];

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    match future
        .as_mut()
        .poll(&mut Context::from_waker(Waker::noop()))
    {
        Poll::Ready(result) => result,
        Poll::Pending => panic!("local cryptography must finish synchronously"),
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let mut random = [0; 16];
        checked(getrandom::fill(&mut random));
        let path = std::env::temp_dir().join(format!(
            "proof-agentd-{}-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed),
            hex::encode(random)
        ));
        checked(std::fs::create_dir(&path));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        checked(std::fs::remove_dir_all(&self.0));
    }
}

fn tenant() -> TenantId {
    checked(TenantId::new("proof-human"))
}
fn retention() -> RetentionPolicy {
    checked(RetentionPolicy::new(100, 50))
}
fn storage_key(kind: ObjectKind, id: Vec<u8>) -> TenantKey {
    checked(TenantKey::new(tenant(), kind, id))
}

/// The payload and envelope both come from the production Send encoder and signer.
fn producer() -> (Prepared, ModuleRegistry, Vec<u8>) {
    let signer = LocalSigner::new(SEED);
    let payload = checked(ready(
        SendDebit {
            from: [0x11; 32],
            to: [0x22; 32],
            asset: [0x33; 32],
            amount: 25,
            source_sequence: 7,
            idempotency_key: KEY,
            expires_at: 1_010,
            context_hash: [0x55; 32],
            conditions: Vec::new(),
            authorization_kind: 1,
            network_id: 125,
            protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
        }
        .sign(&signer),
    ));
    let encoded = checked(encode_send_envelope(
        &payload,
        &EnvelopeOptions {
            actor: "did:layerx:human-proof",
            public_key: signer.public_key(),
            protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
            network_id: 125,
            identity_sequence: 7,
            idempotency_key: KEY,
            fee_limit: 7,
            not_before: 995,
            not_after: 1_010,
        },
    ));
    let authority = checked(Authority::owner(&signer.public_key()));
    let prepared = checked(restore_canonical(
        &encoded.canonical,
        88,
        &encoded.registry,
        Some(&authority),
    ));
    assert_eq!(prepared.envelope, encoded.envelope);
    assert_eq!(prepared.disclosure, encoded.disclosure);
    (prepared, encoded.registry, payload)
}

fn signing_preparation(authorization: Prepared, payload: Vec<u8>) -> SigningPreparation {
    let key = checked(k256::ecdsa::SigningKey::from_slice(&SEED));
    let actor = checked(layerx_crypto::secp256k1::evm_address(
        key.verifying_key().to_encoded_point(false).as_bytes(),
    ));
    let transaction = checked(evm::prepare_bound(
        evm::EvmPreparationState {
            chain_id: 125,
            pending_nonce: 7,
            base_fee_per_gas: 2,
            head_height: 88,
        },
        evm::PreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 20,
            maximum_payload_bytes: 1_024,
        },
        evm::PrepareRequest {
            from: actor,
            call: evm::PrecompileCall::ledger_transfer(payload),
            expected_nonce: Some(7),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1_024,
        },
        100_000,
    ));
    SigningPreparation {
        transaction,
        actor_address: actor,
        authorization,
    }
}

#[test]
fn prepared_signatures_cross_the_human_api_and_recover_exact_outbox_bytes() {
    let root = Directory::new();
    let (prepared, registry, payload) = producer();
    let authority = prepared.envelope.authority().clone();
    let canonical = prepared.canonical_bytes.clone();
    let key = storage_key(ObjectKind::Configuration, b"human-prepared".to_vec());
    {
        let mut store = checked(Store::open(&root.0));
        checked(store.put_local(key.clone(), canonical.clone()));
    }
    // Drop the producer and reopen exactly the canonical bytes consumed by custody/key export.
    drop(prepared);
    let mut store = checked(Store::open(&root.0));
    let restored = checked(restore_canonical(
        store.get(&key).expect("durable preparation").bytes(),
        88,
        &registry,
        Some(&authority),
    ));
    assert_eq!(restored.canonical_bytes, canonical);
    checked(verify_disclosure_binding(&restored));
    assert!(matches!(
        restore_canonical(&canonical, 88, &registry, None),
        Err(RestorePreparationError::MissingAuthority)
    ));
    let foreign = checked(Authority::owner(&LocalSigner::new([0x67; 32]).public_key()));
    assert!(matches!(
        restore_canonical(&canonical, 88, &registry, Some(&foreign)),
        Err(RestorePreparationError::AuthorityMismatch)
    ));
    let signer = LocalSigner::new(SEED);
    let message = checked(SignatureMessage::new(
        Domain::SignaturePreimage,
        restored.envelope.protocol_version(),
        restored.envelope.network_id(),
        &canonical,
    ));
    let signature = checked(ready(
        signer.sign(checked(SigningRequest::new(message, &restored.disclosure))),
    ));
    let kernel = checked(attach_kernel_signature(&restored, *signature.as_bytes()));
    let verified_kernel = checked(verify_kernel_before_submit(
        &kernel,
        &restored,
        &signer.public_key(),
        &registry,
    ));
    assert_eq!(verified_kernel.exact_bytes(), kernel);
    assert_eq!(verified_kernel.idempotency_key(), KEY);
    assert!(matches!(
        verify_kernel_before_submit(
            &kernel,
            &restored,
            &LocalSigner::new([0x67; 32]).public_key(),
            &registry
        ),
        Err(SigningError::AuthorityMismatch)
    ));

    let signing = signing_preparation(restored, payload);
    let raw = checked(signing.transaction.call.sign(&SEED));
    let (call, signature) = checked(decode_signed(&raw));
    assert_eq!(
        checked(recover_sender(&call, &signature)),
        signing.actor_address
    );
    let attached = checked(attach_external_signature(&signing, *signature.as_bytes()));
    assert_eq!(attached, raw);
    let verified = checked(verify_before_submit(
        &attached,
        &signing,
        &signing.actor_address,
        &registry,
    ));
    assert_eq!(verified.exact_bytes(), raw);
    assert_eq!(verified.signed_byte_length(), raw.len());
    assert_eq!(verified.idempotency_key(), KEY);
    let hash = verified.activity_id();
    assert_eq!(hash, transaction_hash(&raw));
    let wrong_raw = checked(signing.transaction.call.sign(&[0x67; 32]));
    let (_, wrong_signature) = checked(decode_signed(&wrong_raw));
    assert_eq!(
        attach_external_signature(&signing, *wrong_signature.as_bytes()),
        Err(SigningError::SignerMismatch)
    );
    assert_eq!(
        verify_before_submit(&wrong_raw, &signing, &signing.actor_address, &registry),
        Err(SigningError::SignerMismatch)
    );
    let mut malformed = raw.clone();
    malformed.push(0);
    assert_eq!(
        verify_before_submit(&malformed, &signing, &signing.actor_address, &registry),
        Err(SigningError::SignatureInvalid)
    );

    let id_root = Directory::new();
    let idempotency = checked(layerx_agentd::idempotency::Store::open(
        &id_root.0,
        tenant(),
        retention(),
    ));
    let mut outbox = Outbox::default();
    let result = EconomicResult {
        response_bytes: hash.to_vec(),
        receipt_ref: None,
    };
    let first = checked(idempotency.execute(KEY, &canonical, 88, |attempt| {
        assert!(!attempt.retry);
        assert_eq!(attempt.exact_request_bytes, canonical);
        let id = outbox
            .enqueue(&mut store, tenant(), verified.into_exact_bytes())
            .map_err(|e| format!("{e:?}"))?;
        assert_eq!(id, hash);
        Ok(result.clone())
    }));
    assert_eq!(first, Outcome::First(result.clone()));
    let queued = outbox.status(hash).expect("queued consumer result").clone();
    assert_eq!(queued.state, SubmissionState::Queued);
    assert!(!queued.state.terminal());
    assert_eq!(queued.activity_id, hash);
    assert_eq!(queued.transitions.len(), 2);
    drop(outbox);
    drop(store);
    drop(idempotency);

    let mut store = checked(Store::open(&root.0));
    let mut outbox = Outbox::default();
    checked(outbox.restore(&store, tenant(), hash));
    assert_eq!(outbox.status(hash), Some(&queued));
    assert_eq!(checked(outbox.bytes_for_transmission(hash)), raw);
    assert!(matches!(
        outbox.enqueue(&mut store, tenant(), raw.clone()),
        Err(OutboxError::Duplicate)
    ));
    assert!(matches!(
        outbox.restore(&store, checked(TenantId::new("other-human")), hash),
        Err(OutboxError::NotFound)
    ));
    assert!(matches!(
        outbox.bytes_for_transmission([0; 32]),
        Err(OutboxError::NotFound)
    ));
    let idempotency = checked(layerx_agentd::idempotency::Store::open(
        &id_root.0,
        tenant(),
        retention(),
    ));
    assert_eq!(
        checked(idempotency.execute(KEY, &canonical, 89, |_| panic!(
            "settled effect repeated after restart"
        ))),
        Outcome::RepeatedOriginal(result)
    );
    assert!(matches!(
        idempotency.execute(KEY, &raw, 89, |_| panic!("conflicting effect executed")),
        Err(IdempotencyError::Conflict(_))
    ));
    let submitted = checked(outbox.transition(
        &mut store,
        hash,
        SubmissionState::Submitted,
        "Human transmitted exact retained transaction",
        None,
    ));
    assert_eq!(submitted.state, SubmissionState::Submitted);
    assert!(matches!(
        outbox.bytes_for_transmission(hash),
        Err(OutboxError::NotQueued)
    ));
    assert!(matches!(
        outbox.transition(
            &mut store,
            hash,
            SubmissionState::Executed,
            "receipt absent",
            None
        ),
        Err(OutboxError::SuccessWithoutVerifiedReceipt)
    ));
    assert_eq!(checked(outbox.exact_signed_bytes(hash)), raw);
    let signed_key = storage_key(ObjectKind::PreparedActivity, hash.to_vec());
    checked(store.put_local(signed_key, malformed));
    let mut corrupt_restore = Outbox::default();
    assert!(matches!(
        corrupt_restore.restore(&store, tenant(), hash),
        Err(OutboxError::Corrupt)
    ));
    assert!(corrupt_restore.status(hash).is_none());
}

#[test]
fn inconsistent_prepared_envelope_preimage_and_audit_are_refused() {
    let (prepared, registry, payload) = producer();
    let mut stale_envelope = prepared.clone();
    let mut signing = signing_preparation(prepared.clone(), payload);
    let raw = checked(signing.transaction.call.sign(&SEED));
    let (_, signature) = checked(decode_signed(&raw));
    for field in 0..3 {
        let mut stale = prepared.clone();
        match field {
            0 => stale.signing_preimage[0] ^= 1,
            1 => stale.audit.idempotency_key[0] ^= 1,
            _ => stale.audit.observed_head_sequence += 1,
        }
        assert_eq!(
            verify_disclosure_binding(&stale),
            Err(DisclosureBindingError::CanonicalMismatch)
        );
        assert!(matches!(
            attach_kernel_signature(&stale, [0; 64]),
            Err(SigningError::Disclosure(
                DisclosureBindingError::CanonicalMismatch
            ))
        ));
        signing.authorization = stale;
        assert!(matches!(
            attach_external_signature(&signing, *signature.as_bytes()),
            Err(SigningError::Disclosure(
                DisclosureBindingError::CanonicalMismatch
            ))
        ));
        assert!(matches!(
            verify_before_submit(&raw, &signing, &signing.actor_address, &registry),
            Err(SigningError::Disclosure(
                DisclosureBindingError::CanonicalMismatch
            ))
        ));
    }
    // A separately valid envelope with different identity state must not replace the retained one.
    stale_envelope.envelope = checked(encode_send_envelope(
        prepared.envelope.payload().as_bytes(),
        &EnvelopeOptions {
            actor: "did:layerx:human-proof",
            public_key: LocalSigner::new(SEED).public_key(),
            protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
            network_id: 125,
            identity_sequence: 8,
            idempotency_key: KEY,
            fee_limit: 7,
            not_before: 995,
            not_after: 1_010,
        },
    ))
    .envelope;
    assert_eq!(
        verify_disclosure_binding(&stale_envelope),
        Err(DisclosureBindingError::CanonicalMismatch)
    );
}

#[test]
fn pending_attempt_recovers_original_hash_height_and_exact_request() {
    let root = Directory::new();
    let (prepared, _, payload) = producer();
    let signing = signing_preparation(prepared, payload);
    let raw = checked(signing.transaction.call.sign(&SEED));
    let hash = transaction_hash(&raw);
    {
        let store = checked(layerx_agentd::idempotency::Store::open(
            &root.0,
            tenant(),
            retention(),
        ));
        checked(store.record_attempt(KEY, &raw, 88, hash));
    }
    let store = checked(layerx_agentd::idempotency::Store::open(
        &root.0,
        tenant(),
        retention(),
    ));
    let result = EconomicResult {
        response_bytes: hash.to_vec(),
        receipt_ref: None,
    };
    assert_eq!(
        checked(store.execute(KEY, &raw, 99, |attempt| {
            assert!(attempt.retry);
            assert_eq!(attempt.head_height, 88);
            assert_eq!(attempt.transaction_hash, Some(hash));
            assert_eq!(attempt.exact_request_bytes, raw);
            assert_eq!(transaction_hash(attempt.exact_request_bytes), hash);
            checked(decode_signed(attempt.exact_request_bytes));
            Ok(result.clone())
        })),
        Outcome::RepeatedOriginal(result.clone())
    );
    drop(store);
    let store = checked(layerx_agentd::idempotency::Store::open(
        &root.0,
        tenant(),
        retention(),
    ));
    assert_eq!(
        checked(store.execute(KEY, &raw, 100, |_| panic!("pending retry settled twice"))),
        Outcome::RepeatedOriginal(result)
    );
}

#[test]
fn real_approval_event_survives_restart_with_evidence_and_rejects_duplicate() {
    let root = Directory::new();
    let (prepared, _, _) = producer();
    let lifecycle = ApprovalLifecycle {
        tenant: tenant(),
        agent: checked(Did::new(b"did:layerx:human-proof")),
        session: SessionId([2; 32]),
        capability: [3; 32],
        policy_version: "policy-v1".into(),
        approval_id: KEY,
        canonical_digest: prepared.disclosure_digest.0,
        activity_type: prepared.envelope.activity_type().ordinal(),
        asset: "LXP".into(),
        kind: ApprovalEventKind::Created,
        principal: None,
        observed_at_ms: 1_000,
    };
    let mut ingestor = checked(EventIngestor::open(
        checked(Store::open(&root.0)),
        tenant(),
        2,
        0,
    ));
    let mut audit = checked(Log::open(&root.0, &tenant()));
    let mut coverage = Coverage::default();
    let emission = checked(ApprovalEvents::emit(
        &mut ingestor,
        &mut audit,
        &mut coverage,
        &lifecycle,
    ));
    let emitted: CoreEvent = ingestor.take_next().expect("real producer event");
    assert_eq!(emitted.canonical_bytes, emission.canonical_event_bytes);
    assert_eq!(emitted.receipt_reference, None);
    assert_eq!(
        emitted.receipt_verification_level,
        VerificationLevel::UNVERIFIED
    );
    let watermark = ingestor.watermark();
    drop(audit);
    drop(ingestor);

    let mut ingestor = checked(EventIngestor::open(
        checked(Store::open(&root.0)),
        tenant(),
        2,
        77,
    ));
    assert_eq!(ingestor.watermark(), watermark);
    assert_eq!(
        checked(ingestor.retained_event(emission.global_sequence)),
        emitted
    );
    assert_eq!(emitted.attributes.agent, "did:layerx:human-proof");
    assert_eq!(emitted.attributes.module, "approval");
    assert_eq!(emitted.attributes.counterparty, "human-approval");
    let key = storage_key(
        ObjectKind::Event,
        emission.global_sequence.to_be_bytes().to_vec(),
    );
    let stored = ingestor
        .store()
        .get(&key)
        .expect("Human durable event consumer");
    assert_eq!(stored.class(), StorageClass::LocalOnly);
    assert_eq!(stored.bytes(), emission.canonical_event_bytes);
    assert!(matches!(
        ingest_local_restriction(&mut ingestor, emitted.clone()),
        Err(IngestError::Repeated { sequence: 0 })
    ));
    assert_eq!(ingestor.watermark(), watermark);
    assert_eq!(checked(ingestor.retained_event(0)), emitted);
    assert!(matches!(
        ingestor.retained_event(1),
        Err(IngestError::CorruptEvent)
    ));
    let mut forged_evidence = emitted;
    forged_evidence.global_sequence = watermark.next_expected;
    forged_evidence.receipt_reference = Some([1; 32]);
    assert!(matches!(
        ingest_local_restriction(&mut ingestor, forged_evidence),
        Err(IngestError::ReceiptEvidenceMismatch)
    ));
    assert_eq!(ingestor.watermark(), watermark);
}
