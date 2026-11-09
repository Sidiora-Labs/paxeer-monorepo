//! Proves ledger_payment_call -> call-bound daemon preparation/signing -> durable
//! Outbox, idempotency and ApprovalEvents consumers used by Human. The tests reopen
//! production stores and require exact EVM bytes, signature and disclosure binding.

use std::fmt::Debug;
use std::future::Future;
use std::os::unix::fs::DirBuilderExt;
use std::path::PathBuf;
use std::pin::pin;
use std::sync::atomic::{AtomicU64, Ordering};
use std::task::{Context, Poll, Waker};

use layerx_agentd::approval::{ApprovalEventKind, ApprovalEvents, ApprovalLifecycle};
use layerx_agentd::audit::{Coverage, Log};
use layerx_agentd::events::{ingest_local_restriction, CoreEvent, EventIngestor, IngestError};
use layerx_agentd::idempotency::{EconomicResult, IdempotencyError, Outcome, RetentionPolicy};
use layerx_agentd::outbox::{Outbox, OutboxError, SubmissionState};
use layerx_agentd::prepare::{evm, verify_call_binding, CallBindingError};
use layerx_agentd::session::SessionId;
use layerx_agentd::sign::{
    attach_external_signature, external, verify_before_submit, SigningError, SigningPreparation,
};
use layerx_agentd::store::{ObjectKind, StorageClass, Store, TenantId, TenantKey};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::send::{ledger_payment_call, CallOptions, SendDebit};
use layerx_types::ids::Did;
use layerx_types::payload::ModuleRegistry;
use layerx_types::verify::VerificationLevel;

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
        let path = checked(std::fs::canonicalize(std::env::temp_dir())).join(format!(
            "proof-agentd-{}-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed),
            hex::encode(random)
        ));
        checked(std::fs::DirBuilder::new().mode(0o700).create(&path));
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

/// The signed debit and EVM call both come from the current production producers.
fn producer() -> (SigningPreparation, ModuleRegistry) {
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
    let key = checked(k256::ecdsa::SigningKey::from_slice(&SEED));
    let actor = checked(layerx_crypto::secp256k1::evm_address(
        key.verifying_key().to_encoded_point(false).as_bytes(),
    ));
    let produced = checked(ledger_payment_call(
        &payload,
        &CallOptions {
            from: actor,
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1,
            // Match prepare_bound: twice the observed base fee plus the tip.
            max_fee_per_gas: 5,
            gas_limit: 100_000,
        },
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
    assert_eq!(transaction.call, produced.call);
    assert_eq!(transaction.disclosure, produced.disclosure);
    assert_eq!(
        checked(layerx_crypto::disclosure::call::bind_call(
            &transaction.call,
            actor
        )),
        transaction.disclosure
    );
    checked(verify_call_binding(&transaction));
    (
        SigningPreparation {
            transaction,
            actor_address: actor,
        },
        checked(ModuleRegistry::new(&[])),
    )
}

fn human_enqueue(
    store: &mut Store,
    outbox: &mut Outbox,
    prepared: &SigningPreparation,
    registry: &ModuleRegistry,
    raw: &[u8],
) -> [u8; 32] {
    let request = serde_json::json!({"raw_transaction": format!("0x{}", hex::encode(raw))});
    let (answer, rung) = checked(layerx_agentd::agent_rpc_dispatch::dispatch_raw_submit(
        request.as_object().expect("submit request"),
        layerx_agent_api::error::RequestId(7),
        |bytes| {
            assert_eq!(bytes, raw);
            let verified =
                verify_before_submit(&bytes, prepared, &prepared.actor_address, registry)
                    .map_err(|_| layerx_agentd::human::HumanOperationError::Refused)?;
            let hash = outbox
                .enqueue(store, tenant(), verified.into_exact_bytes())
                .map_err(|_| layerx_agentd::human::HumanOperationError::Unavailable)?;
            layerx_agentd::human_runtime::write_submission(
                outbox.status(hash).expect("durable submission"),
                None,
            )
        },
    ));
    let hash = transaction_hash(raw);
    assert_eq!(answer["tx_hash"], hex::encode(hash));
    assert_eq!(answer["submission_ref"], hex::encode(hash));
    assert_eq!(answer["state"], "Queued");
    assert_eq!(rung, layerx_types::rung::Rung::Submitted);
    assert_eq!(checked(outbox.exact_signed_bytes(hash)), raw);
    hash
}

#[test]
fn prepared_signatures_cross_the_human_api_and_recover_exact_outbox_bytes() {
    let root = Directory::new();
    let (prepared, registry) = producer();
    let canonical = prepared.transaction.call.encode_unsigned();
    let package = checked(external(&prepared));
    assert_eq!(package.canonical_bytes, canonical);
    assert_eq!(
        package.signing_preimage,
        prepared.transaction.call.signing_hash()
    );
    assert_eq!(
        package.disclosure_digest,
        prepared.transaction.disclosure_digest
    );
    {
        let mut store = checked(Store::open(&root.0));
        checked(prepared.retain(&mut store, tenant(), KEY));
        checked(prepared.retain(&mut store, tenant(), KEY));
        // The retained reference binds the observed head as well as the exact call.
        let mut changed_head = prepared.clone();
        changed_head.transaction.observed_head_height += 1;
        assert_eq!(
            changed_head.retain(&mut store, tenant(), KEY),
            Err(SigningError::PreparedBytesChanged)
        );
    }
    let expected = prepared.clone();
    drop(prepared);
    let mut store = checked(Store::open(&root.0));
    let signing = checked(SigningPreparation::restore(&store, tenant(), KEY));
    assert_eq!(signing, expected);
    assert_eq!(signing.transaction.call.encode_unsigned(), canonical);
    checked(verify_call_binding(&signing.transaction));
    assert_eq!(
        SigningPreparation::restore(&store, tenant(), [0; 32]),
        Err(SigningError::PreparedBytesChanged)
    );
    assert_eq!(
        SigningPreparation::restore(&store, checked(TenantId::new("other-human")), KEY),
        Err(SigningError::PreparedBytesChanged)
    );
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
    let first = checked(idempotency.execute(KEY, &raw, 88, |attempt| {
        assert!(!attempt.retry);
        assert_eq!(attempt.exact_request_bytes, raw);
        let id = human_enqueue(
            &mut store,
            &mut outbox,
            &signing,
            &registry,
            verified.exact_bytes(),
        );
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
        checked(idempotency.execute(KEY, &raw, 89, |_| panic!(
            "settled effect repeated after restart"
        ))),
        Outcome::RepeatedOriginal(result)
    );
    assert!(matches!(
        idempotency.execute(KEY, &canonical, 89, |_| panic!(
            "conflicting effect executed"
        )),
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
fn inconsistent_prepared_call_and_disclosure_are_refused() {
    let (signing, registry) = producer();
    let raw = checked(signing.transaction.call.sign(&SEED));
    checked(verify_before_submit(
        &raw,
        &signing,
        &signing.actor_address,
        &registry,
    ));
    let mut stale_digest = signing.clone();
    stale_digest.transaction.disclosure_digest[0] ^= 1;
    assert!(matches!(
        verify_before_submit(&raw, &stale_digest, &stale_digest.actor_address, &registry),
        Err(SigningError::CallBinding(CallBindingError::DigestMismatch))
    ));
    let mut stale_call = signing.clone();
    stale_call.transaction.call.data[68 + 4 + 32] ^= 1;
    assert_eq!(
        verify_before_submit(&raw, &stale_call, &stale_call.actor_address, &registry),
        Err(SigningError::PreparedBytesChanged)
    );
    let mut stale_disclosure = signing.clone();
    stale_disclosure.transaction.disclosure.idempotency_key[0] ^= 1;
    assert!(matches!(
        verify_before_submit(
            &raw,
            &stale_disclosure,
            &stale_disclosure.actor_address,
            &registry
        ),
        Err(SigningError::CallBinding(_))
    ));
    let mut stale_actor = signing.clone();
    stale_actor.actor_address[0] ^= 1;
    assert_eq!(
        verify_before_submit(&raw, &stale_actor, &stale_actor.actor_address, &registry),
        Err(SigningError::SignerMismatch)
    );
    // Check the current equivalents of the retired envelope/preimage/audit bindings.
    let mut stale_hash = signing.clone();
    stale_hash.transaction.signing_hash[0] ^= 1;
    assert_eq!(
        verify_call_binding(&stale_hash.transaction),
        Err(CallBindingError::SigningHashMismatch)
    );
    assert_eq!(
        external(&stale_hash),
        Err(SigningError::PreparedBytesChanged)
    );
    assert_eq!(
        attach_external_signature(&stale_hash, [0; 65]),
        Err(SigningError::PreparedBytesChanged)
    );
    let mut stale_sender = signing.clone();
    stale_sender.transaction.from[0] ^= 1;
    assert!(matches!(
        verify_call_binding(&stale_sender.transaction),
        Err(CallBindingError::Disclosure(_))
    ));
    assert!(matches!(
        external(&stale_disclosure),
        Err(SigningError::CallBinding(_))
    ));
    let mut changed_call = signing.clone();
    changed_call.transaction.call.nonce += 1;
    changed_call.transaction.signing_hash = changed_call.transaction.call.signing_hash();
    assert!(matches!(
        verify_call_binding(&changed_call.transaction),
        Err(CallBindingError::Disclosure(_))
    ));
    assert!(matches!(
        external(&changed_call),
        Err(SigningError::CallBinding(_))
    ));
    assert_eq!(
        verify_before_submit(&raw, &changed_call, &changed_call.actor_address, &registry),
        Err(SigningError::PreparedBytesChanged)
    );
}

#[test]
fn pending_attempt_recovers_original_hash_height_and_exact_request() {
    let root = Directory::new();
    let idempotency_root = Directory::new();
    let (signing, registry) = producer();
    let raw = checked(signing.transaction.call.sign(&SEED));
    let hash = transaction_hash(&raw);
    {
        let mut durable = checked(Store::open(&root.0));
        let mut outbox = Outbox::default();
        assert_eq!(
            human_enqueue(&mut durable, &mut outbox, &signing, &registry, &raw),
            hash
        );
    }
    {
        let durable = checked(Store::open(&root.0));
        let mut outbox = Outbox::default();
        checked(outbox.restore(&durable, tenant(), hash));
        assert_eq!(checked(outbox.exact_signed_bytes(hash)), raw);
    }
    {
        let store = checked(layerx_agentd::idempotency::Store::open(
            &idempotency_root.0,
            tenant(),
            retention(),
        ));
        checked(store.record_attempt(KEY, &raw, 88, hash));
    }
    let store = checked(layerx_agentd::idempotency::Store::open(
        &idempotency_root.0,
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
        &idempotency_root.0,
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
    let (prepared, registry) = producer();
    let raw = checked(prepared.transaction.call.sign(&SEED));
    let hash = {
        let mut durable = checked(Store::open(&root.0));
        human_enqueue(
            &mut durable,
            &mut Outbox::default(),
            &prepared,
            &registry,
            &raw,
        )
    };
    {
        let durable = checked(Store::open(&root.0));
        let mut outbox = Outbox::default();
        checked(outbox.restore(&durable, tenant(), hash));
        assert_eq!(hash, transaction_hash(&raw));
        assert_eq!(checked(outbox.exact_signed_bytes(hash)), raw);
    }
    let lifecycle = ApprovalLifecycle {
        tenant: tenant(),
        agent: checked(Did::new(b"did:layerx:human-proof")),
        session: SessionId([2; 32]),
        capability: [3; 32],
        policy_version: "policy-v1".into(),
        approval_id: KEY,
        canonical_digest: prepared.transaction.disclosure_digest,
        activity_type: prepared.transaction.disclosure.activity_type.ordinal(),
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
