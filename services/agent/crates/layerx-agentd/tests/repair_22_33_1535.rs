//! Task 22.33.1535 repair record (audit 22.30.95, acceptance 22998.1).
//!
//! Finding 3: cache_payload_substitution_is_refused_before_revalidation_and_degraded_serving.
//! Finding 4: cache_keys_storage_and_miss_only_tenants_obey_admission_bounds.
//! Finding 5: repeated_regressions_block_writes_until_both_high_water_marks_recover.
//! Finding 7: finality_consumer_uses_rungs_and_reopens_authenticated_evidence.
//! Finding 31: native_evidence::native_type2_bytes_and_matching_node_receipt_survive_restart.
//! Finding 50: registry_source_contract_drives_mtls_and_refuses_unbound_responses.
//! Finding 49: external_signing_consumer_compiles_and_submits_exact_type2_bytes.
//!
//! Wiring request, agent domain 9: spec/layerx-merge/interfaces.kvx must retire
//! InclusionBundle/wait_for_level and link the current finality.rs rung contract.
//! Wiring request, agent domain 9: spec/layerx-merge/interfaces.kvx must replace
//! legacy signing rows/callers with the typed EIP-1559 contract in sign.rs.
//! Wiring request, agent domain 9: services/agent/README.md must link/publish the
//! registry_source.rs module contract; spec/layerx-merge/interfaces.kvx must
//! replace its missing contracts/agent-{config,rust-api}.md links with that contract.
//!
//! Baseline: no earlier gate ledger or implementation existed at task start;
//! this integration target did not exist on the base. No baseline test result is
//! claimed: the explicit gate order prohibits a pre-implementation verify run.
//! The qualified revision/command/exit/private evidence path are the gate's
//! final JSON record; this source is not a declaration that qualification ran.
//!
//! Continuation baseline (attempt 2): seven cases passed; the finality and
//! external signing consumers failed store-root protection, and the registry
//! consumer failed a later mTLS handshake. Create private canonical store roots
//! and give the peer-verifying TLS server a session-id context for resumption.
//! Continuation ruling: the acceptor trusts the fixture client's issuing CA;
//! the protected provider configuration supplies that client's certificate/key.

use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::cache::{revalidate, CacheError, CacheValue, EvidenceCache};
use layerx_agentd::degraded::{enter, Controller, Mode, Observation, OperationError, Reference};
use layerx_agentd::finality::{
    augment_verified, wait_for_rung, FinalityError, SubmissionProgress, VerifiedBlock,
    VerifiedReceipt, WaitResult,
};
use layerx_agentd::sign::{
    attach_external_signature, external, verify_before_submit, ExternalSigningPackage,
    SigningError, SigningMode, SigningPreparation, VerifiedSubmission,
};
use layerx_agentd::store::{Store, TenantId};
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerBlock};
use layerx_crypto::evm_transaction::{decode_signed, transaction_hash};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_types::payload::ModuleRegistry;
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;

// Compile and execute the repaired production test source through the declared
// integration target as well as through the crate's unit-test target.
use layerx_agentd::protocol_evidence::{RawReceiptEvidence, VerifiedReceiptEvidence};
pub use layerx_agentd::{outbox, prepare, protocol_evidence, sign, store};
#[path = "../src/protocol_evidence_native_tests.rs"]
mod native_evidence;

static NEXT: AtomicU64 = AtomicU64::new(0);

struct Directory(PathBuf);
impl Directory {
    fn new(label: &str) -> Self {
        use std::os::unix::fs::{DirBuilderExt, PermissionsExt};

        let root = std::env::temp_dir().join(format!(
            "repair-22-33-1535-{label}-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .expect("isolated private test directory");
        let root = std::fs::canonicalize(root).expect("canonical test directory");
        assert_eq!(
            std::fs::metadata(&root)
                .expect("private directory metadata")
                .permissions()
                .mode()
                & 0o777,
            0o700
        );
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).expect("remove isolated test directory");
    }
}

fn tenant(value: &str) -> TenantId {
    TenantId::new(value).expect("bounded tenant")
}

fn verified_node_receipt() -> (Vec<u8>, layerx_proof::receipt::VerifiedReceipt) {
    let (_, _, _, view, keys) = native_evidence::native_fixture();
    let bytes = native_evidence::decode_hex(view["receipt"].as_str().expect("receipt bytes"));
    let pins: Vec<_> = keys
        .iter()
        .map(|key| {
            SequencerAuthorization::new(
                key.key_id,
                key.public_key,
                key.first_height,
                key.last_height,
            )
        })
        .collect();
    let decoded = layerx_proof::receipt::verify_sequencer_signature(&bytes, pins.as_slice())
        .expect("verify the node producer's exact signed receipt");
    let protocol = decoded.protocol().expect("node protocol receipt");
    let authority = layerx_proof::receipt::AuthorizedBatch::from_receipt_keys(
        &bytes,
        pins.as_slice(),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
    )
    .expect("node receipt's height-bound published key");
    let proof = layerx_proof::receipt::verify(&bytes, &authority)
        .expect("fully verify the node producer's receipt and economic invariants");
    (bytes, proof)
}

#[test]
fn cache_payload_substitution_is_refused_before_revalidation_and_degraded_serving() {
    let (bytes, verified) = verified_node_receipt();
    let owner = tenant("cache-payload");
    let mut cache = EvidenceCache::new(2, 16 * 1024).expect("bounded cache");
    let original = CacheValue::try_from_receipt(bytes.clone(), &verified, 100, [1; 32])
        .expect("verified receipt payload");
    cache
        .insert(owner.clone(), b"receipt".to_vec(), original.clone())
        .expect("retain receipt");
    let mut substituted = bytes.clone();
    substituted[30] ^= 1;
    assert_eq!(
        CacheValue::try_from_receipt(substituted.clone(), &verified, 100, [1; 32]),
        Err(CacheError::PayloadMismatch)
    );
    // The old infallible signature cannot be used to bypass checked construction.
    assert!(std::panic::catch_unwind(|| CacheValue::from_receipt(
        substituted.clone(),
        &verified,
        100,
        [1; 32]
    ))
    .is_err());
    assert_eq!(
        revalidate(
            &mut cache,
            owner.clone(),
            b"receipt",
            VerificationLevel::SEQUENCER_SIGNED,
            101,
            [2; 32],
            || { CacheValue::try_from_receipt(substituted.clone(), &verified, 101, [2; 32]) }
        ),
        Err(CacheError::PayloadMismatch)
    );
    let retained = revalidate(
        &mut cache,
        owner.clone(),
        b"receipt",
        VerificationLevel::SEQUENCER_SIGNED,
        100,
        [1; 32],
        || panic!("fresh payload must not fetch"),
    )
    .expect("failed refresh preserved the authenticated payload");
    assert_eq!(retained, original);
    assert_eq!(cache.metrics(&owner).revalidations, 0);
    let mut controller = Controller::default();
    enter(
        &mut controller,
        Observation::Ready {
            reference: Reference {
                head_sequence: 100,
                checkpoint: [1; 32],
            },
            maximum_rung: Rung::Receipted,
        },
    );
    enter(&mut controller, Observation::Unreachable);
    let mut serving_ran = false;
    let attempted =
        CacheValue::try_from_receipt(substituted, &verified, 100, [1; 32]).map(|candidate| {
            serving_ran = true;
            controller.serve_cached(&candidate, Rung::Submitted)
        });
    assert_eq!(attempted, Err(CacheError::PayloadMismatch));
    assert!(!serving_ran);
    let served = controller
        .serve_cached(&retained, Rung::Submitted)
        .expect("degraded verified read");
    assert_eq!(served.canonical_core_bytes, bytes);
    assert!(served.staleness.stale);

    // InclusionEvidence retains a root, not the authenticated leaf. Recheck an
    // actual nontrivial path before constructing a cache value from that leaf.
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/custody/daemon-light-credit-receipt");
    let read = |name: &str| std::fs::read(root.join(name)).expect("committed native fixture");
    let receipt = read("credit.receipt");
    let header = read("header");
    let decoded = layerx_wire::receipt::decode_batch_header(&header).expect("producer header");
    let signature = read("header.signature")
        .try_into()
        .expect("header signature");
    let public = read("sequencer.public").try_into().expect("public key");
    let path = layerx_proof::merkle::decode_proof(&read("receipt.proof"))
        .expect("producer inclusion path");
    assert!(path.leaf_count() > 1);
    let evidence = layerx_proof::inclusion::verify_receipt(
        &receipt,
        &path,
        &header,
        &signature,
        &SequencerAuthorization::new(
            decoded.sequencer_id(),
            public,
            decoded.batch_number(),
            decoded.batch_number(),
        ),
    )
    .expect("real receipt inclusion verifier");
    let included = CacheValue::try_from_inclusion(receipt.clone(), &evidence, &path, 100, [1; 32])
        .expect("bound multi-leaf payload");
    let mut other = receipt.clone();
    other[30] ^= 1;
    assert_eq!(
        CacheValue::try_from_inclusion(other.clone(), &evidence, &path, 101, [2; 32]),
        Err(CacheError::PayloadMismatch)
    );
    cache
        .insert(owner.clone(), b"included".to_vec(), included.clone())
        .expect("cache proven inclusion");
    assert_eq!(
        revalidate(
            &mut cache,
            owner,
            b"included",
            VerificationLevel::BATCH_INCLUDED,
            101,
            [2; 32],
            || { CacheValue::try_from_inclusion(other.clone(), &evidence, &path, 101, [2; 32]) }
        ),
        Err(CacheError::PayloadMismatch)
    );
    let read = controller
        .serve_cached(&included, Rung::Receipted)
        .expect("degraded inclusion read");
    assert_eq!(read.canonical_core_bytes, receipt);
    assert_eq!(read.evidence_id, evidence.header().digest());
}

#[test]
fn cache_keys_storage_and_miss_only_tenants_obey_admission_bounds() {
    let (bytes, proof) = verified_node_receipt();
    let value = CacheValue::try_from_receipt(bytes.clone(), &proof, 1, [1; 32]).expect("value");
    let mut cache = EvidenceCache::with_limits(2, bytes.len() + 5, 2, 8).expect("cache limits");
    let owner = tenant("bounded-a");
    assert_eq!(
        cache.insert(owner.clone(), vec![1; 9], value.clone()),
        Err(CacheError::KeyTooLong)
    );
    assert_eq!(
        cache.tenant_count(),
        0,
        "invalid keys do not consume admission"
    );
    let mut oversized_capacity_key = Vec::with_capacity(1_000_000);
    oversized_capacity_key.extend_from_slice(b"entry");
    cache
        .insert(owner.clone(), oversized_capacity_key, value.clone())
        .expect("exact key plus value ceiling");
    assert_eq!(cache.metrics(&owner).bytes, bytes.len() + 5);
    cache
        .insert(owner.clone(), b"entry".to_vec(), value.clone())
        .expect("replacement subtracts key and value charge");
    assert_eq!(cache.metrics(&owner).bytes, bytes.len() + 5);
    assert_eq!(cache.metrics(&owner).entries, 1);
    assert_eq!(
        cache.insert(owner.clone(), b"second".to_vec(), value.clone()),
        Err(CacheError::QuotaExceeded)
    );
    assert_eq!(cache.metrics(&owner).entries, 1);
    let miss_only = tenant("bounded-b");
    cache
        .register_tenant(miss_only.clone())
        .expect("explicit bounded miss admission");
    assert_eq!(
        revalidate(
            &mut cache,
            miss_only.clone(),
            b"missing",
            VerificationLevel::SEQUENCER_SIGNED,
            1,
            [1; 32],
            || panic!("a miss cannot fetch")
        ),
        Err(CacheError::Missing)
    );
    assert_eq!(cache.metrics(&miss_only).misses, 1);
    for index in 0..2048 {
        let unknown = tenant(&format!("miss-only-{index}"));
        assert_eq!(
            revalidate(
                &mut cache,
                unknown.clone(),
                b"missing",
                VerificationLevel::SEQUENCER_SIGNED,
                1,
                [1; 32],
                || panic!("unknown tenant cannot fetch")
            ),
            Err(CacheError::UnknownTenant)
        );
        assert_eq!(
            cache.register_tenant(unknown.clone()),
            Err(CacheError::TenantLimit)
        );
        assert_eq!(
            cache.insert(unknown.clone(), b"valid".to_vec(), value.clone()),
            Err(CacheError::TenantLimit)
        );
        assert_eq!(cache.metrics(&unknown), Default::default());
        assert_eq!(cache.tenant_count(), 2);
    }
    assert_eq!(
        revalidate(
            &mut cache,
            owner.clone(),
            &[0; 9],
            VerificationLevel::SEQUENCER_SIGNED,
            1,
            [1; 32],
            || panic!("oversized key cannot fetch")
        ),
        Err(CacheError::KeyTooLong)
    );
    assert_eq!(
        revalidate(
            &mut cache,
            owner.clone(),
            b"entry",
            VerificationLevel::SEQUENCER_SIGNED,
            1,
            [1; 32],
            || panic!("fresh")
        ),
        Ok(value)
    );
    assert_eq!(cache.metrics(&owner).bytes, bytes.len() + 5);
}

#[test]
fn repeated_regressions_block_writes_until_both_high_water_marks_recover() {
    let mut controller = Controller::default();
    let ready = |height, checkpoint, maximum_rung| Observation::Ready {
        reference: Reference {
            head_sequence: height,
            checkpoint: [checkpoint; 32],
        },
        maximum_rung,
    };
    assert_eq!(
        enter(&mut controller, ready(100, 10, Rung::Committed)).mode,
        Mode::Healthy
    );
    assert_eq!(controller.guard_preparation(|| 7), Ok(7));
    for (height, checkpoint, rung) in [
        (99, 9, Rung::Submitted),
        (99, 9, Rung::Submitted),
        (100, 10, Rung::Receipted),
        (100, 10, Rung::Receipted),
        (101, 11, Rung::Receipted),
        (100, 10, Rung::Committed),
        (101, 12, Rung::Committed),
    ] {
        let status = enter(&mut controller, ready(height, checkpoint, rung));
        assert_eq!(status.mode, Mode::Behind);
        assert_eq!(
            status.reference.expect("current observation").head_sequence,
            height
        );
        assert_eq!(status.maximum_rung, rung);
        assert!(!status.readiness.preparation);
        assert!(!status.readiness.submission_acknowledgement);
        assert!(!status.readiness.live_stream);
        assert_eq!(
            controller.guard_preparation(|| panic!("regressed preparation")),
            Err::<(), _>(OperationError::LiveCoreRequired(Mode::Behind))
        );
        assert_eq!(
            controller.guard_submission_acknowledgement(|| panic!("regressed acknowledgement")),
            Err::<(), _>(OperationError::LiveCoreRequired(Mode::Behind))
        );
        enter(&mut controller, Observation::Unreachable);
    }
    let caught_up = enter(&mut controller, ready(101, 11, Rung::Committed));
    assert_eq!(caught_up.mode, Mode::Healthy);
    assert!(caught_up.readiness.preparation);
    assert_eq!(controller.guard_preparation(|| 8), Ok(8));
    assert_eq!(controller.guard_submission_acknowledgement(|| 9), Ok(9));
}

#[test]
fn finality_consumer_uses_rungs_and_reopens_authenticated_evidence() {
    type Wait = fn(
        &mut SubmissionProgress,
        [u8; 32],
        Rung,
        u64,
        u64,
        u64,
    ) -> Result<WaitResult, FinalityError>;
    let wait: Wait = wait_for_rung::<SubmissionProgress>;
    let verify_receipt: fn(
        [u8; 32],
        &[u8],
        &[ReceiptKeyEntry],
    ) -> Result<VerifiedReceipt, FinalityError> = VerifiedReceipt::verify;
    let _verify_block: fn(
        &SequencerBlock,
        &layerx_agentd::finality::BlockCommit,
        &layerx_agentd::read::CheckpointHeader,
        &str,
        &[layerx_agentd::finality::Validator],
    ) -> Result<VerifiedBlock, FinalityError> = VerifiedBlock::verify_with_header;
    let (prepared, registry, raw, view, keys) = native_evidence::native_fixture();
    let verified = verify_before_submit(&raw, &prepared, &prepared.actor_address, &registry)
        .expect("real signer consumer");
    let hash = transaction_hash(verified.exact_bytes());
    let idempotency = verified.idempotency_key();
    let receipt =
        native_evidence::decode_hex(view["receipt"].as_str().expect("recorded signed receipt"));
    let receipt = verify_receipt(hash, &receipt, &keys).expect("published key receipt API");
    let mut progress = SubmissionProgress::accepted(idempotency, hash);
    assert_eq!(
        wait(&mut progress, idempotency, Rung::Submitted, 10, 20, 1)
            .expect("accepted rung")
            .reached,
        Rung::Submitted
    );
    progress
        .observe_receipt(receipt.canonical_bytes(), &keys)
        .expect("receipt consumer");
    assert_eq!(
        wait(&mut progress, idempotency, Rung::Receipted, 10, 20, 1)
            .expect("receipt rung")
            .reached,
        Rung::Receipted
    );
    let timeout = wait(&mut progress, idempotency, Rung::Committed, 10, 20, 3)
        .expect("deadline retains actual rung");
    assert_eq!(timeout.reached, Rung::Receipted);
    assert_eq!(timeout.observed_at_ms, 20);
    assert!(timeout.deadline_elapsed);
    assert!(matches!(
        wait(&mut progress, idempotency, Rung::Committed, 20, 10, 1),
        Err(FinalityError::InvalidDeadline)
    ));
    assert!(matches!(
        wait(&mut progress, idempotency, Rung::Committed, 10, 20, 0),
        Err(FinalityError::InvalidDeadline)
    ));
    let directory = Directory::new("finality");
    let mut store = Store::open(&directory.0).expect("durable store");
    let record = augment_verified(&mut store, tenant("finality"), idempotency, &receipt, None)
        .expect("durable augmentation");
    assert_eq!(record.tx_hash, hash);
    assert_eq!(record.rung, Rung::Receipted);
    drop(store);
    let store = Store::open(&directory.0).expect("reopen finality store");
    let mut restored = SubmissionProgress::restore(
        &store,
        tenant("finality"),
        idempotency,
        &keys[0],
        "paxeer-125",
        &[],
    )
    .expect("reverify durable evidence");
    assert_eq!(
        wait(&mut restored, idempotency, Rung::Receipted, 30, 40, 2)
            .expect("restored rung")
            .reached,
        Rung::Receipted
    );
    assert!(matches!(
        wait(&mut restored, [0; 32], Rung::Submitted, 30, 40, 2),
        Err(FinalityError::ProgressUnavailable)
    ));
    let mut wrong_key = keys[0];
    wrong_key.public_key[0] ^= 1;
    assert!(SubmissionProgress::restore(
        &store,
        tenant("finality"),
        idempotency,
        &wrong_key,
        "paxeer-125",
        &[]
    )
    .is_err());
}

#[test]
fn external_signing_consumer_compiles_and_submits_exact_type2_bytes() {
    let package: fn(&SigningPreparation) -> Result<ExternalSigningPackage, SigningError> = external;
    let attach: fn(&SigningPreparation, [u8; 65]) -> Result<Vec<u8>, SigningError> =
        attach_external_signature;
    let verify: fn(
        &[u8],
        &SigningPreparation,
        &[u8; 20],
        &ModuleRegistry,
    ) -> Result<VerifiedSubmission, SigningError> = verify_before_submit;
    let (prepared, registry, recorded, _, _) = native_evidence::native_fixture();
    let external = package(&prepared).expect("external package consumer");
    assert_eq!(external.mode, SigningMode::External);
    assert_eq!(
        external.canonical_bytes,
        prepared.transaction.call.encode_unsigned()
    );
    assert_eq!(
        external.signing_preimage,
        prepared.transaction.call.signing_hash()
    );
    assert_eq!(
        external.disclosure_digest.0,
        prepared.transaction.disclosure_digest
    );
    let (_, signature) = decode_signed(&recorded).expect("external signer's real signature");
    let attached =
        attach(&prepared, *signature.as_bytes()).expect("65-byte external signature consumer");
    assert_eq!(attached, recorded);
    let submission = verify(&attached, &prepared, &prepared.actor_address, &registry)
        .expect("20-byte sender verification consumer");
    assert_eq!(submission.exact_bytes(), recorded);
    assert_eq!(submission.activity_id(), transaction_hash(&recorded));
    assert_eq!(submission.signed_byte_length(), recorded.len());
    assert_eq!(
        attach(&prepared, [0; 65]),
        Err(SigningError::SignatureInvalid)
    );
    assert_eq!(
        verify(&attached, &prepared, &[0; 20], &registry),
        Err(SigningError::SignerMismatch)
    );
    let mut changed = prepared.clone();
    changed.transaction.call.nonce += 1;
    assert_eq!(package(&changed), Err(SigningError::PreparedBytesChanged));
    let directory = Directory::new("external");
    let mut store = Store::open(&directory.0).expect("external consumer store");
    let mut outbox = outbox::Outbox::default();
    let hash = outbox
        .enqueue_verified(&mut store, tenant("external"), submission)
        .expect("real outbox consumer");
    assert_eq!(
        outbox
            .bytes_for_transmission(hash)
            .expect("exact transmit bytes"),
        attached
    );
    // Compile the async contract as well, with real provisioning refusal before signing.
    async fn consume_self_sign(
        prepared: &SigningPreparation,
        registry: &ModuleRegistry,
    ) -> Result<sign::SignedPreparation, SigningError> {
        sign::self_sign(None, prepared, registry, 1000, 1).await
    }
    use std::future::Future;
    use std::task::{Context, Poll, Wake, Waker};
    struct WakeOnce;
    impl Wake for WakeOnce {
        fn wake(self: std::sync::Arc<Self>) {}
    }
    let waker = Waker::from(std::sync::Arc::new(WakeOnce));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(consume_self_sign(&prepared, &registry));
    assert_eq!(
        future.as_mut().poll(&mut context),
        Poll::Ready(Err(SigningError::NotProvisioned))
    );
}

fn certificate(
    issuer: Option<(
        &openssl::x509::X509,
        &openssl::pkey::PKey<openssl::pkey::Private>,
    )>,
    client: bool,
) -> (
    openssl::x509::X509,
    openssl::pkey::PKey<openssl::pkey::Private>,
) {
    use openssl::{
        asn1::Asn1Time,
        bn::{BigNum, MsbOption},
        hash::MessageDigest,
        pkey::PKey,
        rsa::Rsa,
        x509::{
            extension::{BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName},
            X509NameBuilder, X509,
        },
    };
    let key = PKey::from_rsa(Rsa::generate(2048).expect("RSA key")).expect("TLS key");
    let mut name = X509NameBuilder::new().expect("certificate name");
    name.append_entry_by_text(
        "CN",
        if client {
            "registry-agent"
        } else {
            "localhost"
        },
    )
    .expect("identity name");
    let name = name.build();
    let mut cert = X509::builder().expect("certificate builder");
    cert.set_version(2).expect("certificate version");
    let mut serial = BigNum::new().expect("serial");
    serial
        .rand(128, MsbOption::MAYBE_ZERO, false)
        .expect("random serial");
    cert.set_serial_number(&serial.to_asn1_integer().expect("serial integer"))
        .expect("serial number");
    cert.set_subject_name(&name).expect("subject");
    cert.set_issuer_name(issuer.map_or(&name, |(ca, _)| ca.subject_name()))
        .expect("issuer");
    cert.set_pubkey(&key).expect("public key");
    cert.set_not_before(&Asn1Time::days_from_now(0).expect("not before"))
        .expect("validity start");
    cert.set_not_after(&Asn1Time::days_from_now(1).expect("not after"))
        .expect("validity end");
    if let Some((ca, signing_key)) = issuer {
        cert.append_extension(
            BasicConstraints::new()
                .critical()
                .build()
                .expect("leaf constraints"),
        )
        .expect("leaf extension");
        cert.append_extension(
            KeyUsage::new()
                .digital_signature()
                .key_encipherment()
                .build()
                .expect("leaf usage"),
        )
        .expect("leaf usage extension");
        let mut extended = ExtendedKeyUsage::new();
        if client {
            extended.client_auth();
        } else {
            extended.server_auth();
        }
        cert.append_extension(extended.build().expect("extended key usage"))
            .expect("extended usage extension");
        if !client {
            let san = SubjectAlternativeName::new()
                .dns("localhost")
                .ip("127.0.0.1")
                .build(&cert.x509v3_context(Some(ca), None))
                .expect("endpoint identities");
            cert.append_extension(san)
                .expect("subject alternative names");
        }
        cert.sign(signing_key, MessageDigest::sha256())
            .expect("CA signs identity");
    } else {
        cert.append_extension(
            BasicConstraints::new()
                .critical()
                .ca()
                .build()
                .expect("CA constraints"),
        )
        .expect("CA extension");
        cert.append_extension(
            KeyUsage::new()
                .critical()
                .key_cert_sign()
                .crl_sign()
                .build()
                .expect("CA usage"),
        )
        .expect("CA usage extension");
        cert.sign(&key, MessageDigest::sha256())
            .expect("root signature");
    }
    (cert.build(), key)
}

fn protected_file(directory: &Directory, name: &str, bytes: &[u8]) -> PathBuf {
    use std::io::Write;
    use std::os::unix::fs::OpenOptionsExt;
    let path = directory.0.join(name);
    let mut file = std::fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(&path)
        .expect("protected material");
    file.write_all(bytes).expect("write protected material");
    path
}

#[test]
fn registry_source_contract_drives_mtls_and_refuses_unbound_responses() {
    use layerx_agentd::ops::program::{ProgramDiscovery, ProgramInterfaceRead};
    use layerx_agentd::registry_source::{RegistrySourceError, RegistrySourceProvider};
    use layerx_programs::{
        InterfaceEntryPoint, ProgramId, ProgramInterface, ProgramLifecycle, SourceStatus,
        ValueSchema, ValueType,
    };
    use openssl::ssl::{SslAcceptor, SslMethod, SslVerifyMode};
    use serde_json::{json, Value};
    use sha2::{Digest, Sha256};
    use std::io::{BufRead, BufReader, Write};
    use std::os::unix::fs::PermissionsExt;

    // The production interface producer validates this committed Rust fixture's
    // WASM module and binds the exported call ABI before it crosses the registry.
    const WASM: &[u8] = &[
        0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3,
        2, 0, 1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a', b'y',
        b'e', b'r', b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm', b'e',
        b'm', b'o', b'r', b'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
    ];
    let interface = ProgramInterface::bind(
        WASM,
        1,
        vec![InterfaceEntryPoint {
            name: "call".to_owned(),
            discriminator: [1, 2, 3, 4],
            calldata: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            response: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            capabilities: Vec::new(),
            event_topics: Vec::new(),
            failures: Vec::new(),
        }],
    )
    .expect("real program-interface producer");
    let head = ProgramDiscovery {
        program: ProgramId::new([0x31; 32]).expect("program id"),
        lifecycle: ProgramLifecycle::Active,
        observed_sequence: 7,
        observed_at: 1_000,
        valid_through: 2_000,
        receipt_digest: [0x71; 32],
        state_root: [0x72; 32],
        version: 3,
        abi_version: 1,
        code_hash: interface.code_hash(),
    };
    let native = ProgramInterfaceRead {
        discovery: head.clone(),
        version: 3,
        interface,
    };
    let valid = json!({
        "program_id": hex::encode(head.program.bytes()), "version": 3,
        "code_hash": hex::encode(head.code_hash), "abi_version": 1,
        "interface": hex::encode(native.interface.canonical_encoding()),
        "interface_digest": hex::encode(Sha256::digest(native.interface.canonical_encoding())),
        "deployment_receipt_digest": hex::encode([0x73; 32]),
        "state_root": hex::encode(head.state_root), "observed_sequence": 7,
        "observed_at": 1_000, "valid_through": 1_900,
        "source": {"status":"verified", "source_digest":hex::encode([0x74; 32]),
            "environment_digest":hex::encode([0x75; 32]), "pipeline":layerx_programs::programs_source_verification()},
        "verification":"deployment-interface-and-current-head-verified"
    });
    let mut responses: Vec<(u16, Value)> = vec![(200, valid.clone())];
    let mut wrong_head = valid.clone();
    wrong_head["observed_sequence"] = json!(8);
    responses.push((200, wrong_head));
    let mut wrong_code = valid.clone();
    wrong_code["code_hash"] = json!(hex::encode([0x76; 32]));
    responses.push((200, wrong_code));
    let mut wrong_interface = valid.clone();
    wrong_interface["interface_digest"] = json!(hex::encode([0x77; 32]));
    responses.push((200, wrong_interface));
    let mut unknown = valid.clone();
    unknown["unexpected"] = json!(true);
    responses.push((200, unknown));
    let mut uppercase = valid.clone();
    uppercase["program_id"] = json!("AA".repeat(32));
    responses.push((200, uppercase));
    let mut unprotected_claim = valid.clone();
    unprotected_claim["deployment_receipt_digest"] = json!("00".repeat(32));
    responses.push((200, unprotected_claim));
    let mut wrong_pipeline = valid.clone();
    wrong_pipeline["source"]["pipeline"] = json!("untrusted-build");
    responses.push((200, wrong_pipeline));
    responses.push((200, valid.clone())); // expired at the caller's now
    responses.push((403, valid.clone()));
    responses.push((200, valid.clone())); // exceeds a different consumer's body bound
    let mut unpublished = valid.clone();
    unpublished["source"] = json!({"status":"unpublished"});
    responses.push((200, unpublished));
    let mut mismatch = valid.clone();
    mismatch["source"] = json!({"status":"mismatch", "expected_code_hash":hex::encode(head.code_hash), "reproduced_artifact_digest":hex::encode([0x78;32])});
    responses.push((200, mismatch));
    let expected_requests = responses.len();
    let directory = Directory::new("registry");
    let (ca, ca_key) = certificate(None, false);
    let (server_certificate, server_key) = certificate(Some((&ca, &ca_key)), false);
    let (client_certificate, client_key) = certificate(Some((&ca, &ca_key)), true);
    let expected_client = client_certificate
        .to_der()
        .expect("client certificate identity");
    let mut acceptor =
        SslAcceptor::mozilla_intermediate_v5(SslMethod::tls_server()).expect("real TLS server");
    acceptor
        .set_certificate(&server_certificate)
        .expect("server certificate");
    acceptor.set_private_key(&server_key).expect("server key");
    acceptor
        .check_private_key()
        .expect("server identity binding");
    acceptor
        .cert_store_mut()
        .add_cert(ca.clone())
        .expect("trusted client issuer");
    acceptor.set_verify(SslVerifyMode::PEER | SslVerifyMode::FAIL_IF_NO_PEER_CERT);
    acceptor
        .set_session_id_context(b"repair-22-33-1535-registry")
        .expect("bind resumed sessions to this authenticated registry");
    let acceptor = acceptor.build();
    let listener = std::net::TcpListener::bind("127.0.0.1:0").expect("loopback registry");
    let address = listener.local_addr().expect("registry address");
    let requested_program = hex::encode(head.program.bytes());
    let server = std::thread::spawn(move || {
        for (status, body) in responses {
            let (socket, _) = listener.accept().expect("registry connection");
            socket
                .set_read_timeout(Some(std::time::Duration::from_secs(5)))
                .expect("read timeout");
            socket
                .set_write_timeout(Some(std::time::Duration::from_secs(5)))
                .expect("write timeout");
            let mut tls = acceptor.accept(socket).expect("mTLS handshake");
            assert_eq!(
                tls.ssl().verify_result(),
                openssl::x509::X509VerifyResult::OK
            );
            assert_eq!(
                tls.ssl()
                    .peer_certificate()
                    .expect("client identity is mandatory")
                    .to_der()
                    .expect("peer DER"),
                expected_client
            );
            let mut reader = BufReader::new(&mut tls);
            let mut line = String::new();
            reader.read_line(&mut line).expect("request line");
            assert_eq!(
                line,
                format!("GET /v1/programs/registry/{requested_program}/interface HTTP/1.1\r\n")
            );
            let mut bearer = false;
            loop {
                line.clear();
                reader.read_line(&mut line).expect("request header");
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("authorization") {
                        assert_eq!(
                            value.trim(),
                            "Bearer registry-source-test-bearer-material-1535"
                        );
                        bearer = true;
                    }
                }
            }
            assert!(bearer, "protected bearer reached the authenticated peer");
            let body = serde_json::to_vec(&body).expect("registry body");
            write!(tls, "HTTP/1.1 {status} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", if status == 200 { "OK" } else { "Forbidden" }, body.len()).expect("HTTP response");
            tls.write_all(&body).expect("registry response bytes");
            tls.flush().expect("registry response flush");
        }
        expected_requests
    });
    let ca_path = protected_file(&directory, "ca.der", &ca.to_der().expect("CA DER"));
    let certificate_path = protected_file(
        &directory,
        "client.pem",
        &client_certificate.to_pem().expect("client PEM"),
    );
    let key_path = protected_file(
        &directory,
        "client.key",
        &client_key.private_key_to_pem_pkcs8().expect("private PEM"),
    );
    let bearer_path = protected_file(
        &directory,
        "bearer",
        b"registry-source-test-bearer-material-1535\n",
    );
    let config = json!({"version":1, "endpoint":format!("https://{address}"),
        "server_ca_der":ca_path, "client_certificate_pem":certificate_path,
        "client_private_key_pem":key_path, "request_token_file":bearer_path,
        "deadline_ms":5_000, "maximum_response_bytes":16_384});
    let config_path = protected_file(
        &directory,
        "config.json",
        &serde_json::to_vec(&config).expect("config"),
    );
    let provider = RegistrySourceProvider::from_protected_config(&config_path)
        .expect("published protected config contract");
    let source = provider
        .read(&native, 1_500)
        .expect("verified registry source");
    assert_eq!(source.program(), head.program.bytes());
    assert_eq!(source.version(), 3);
    assert_eq!(source.code_hash(), head.code_hash);
    assert_eq!(source.state_root(), head.state_root);
    assert_eq!(source.observed_sequence(), 7);
    assert_eq!(source.observed_at(), 1_000);
    assert_eq!(source.valid_through(), 1_900);
    assert_eq!(source.current_head_receipt_digest(), head.receipt_digest);
    assert_eq!(source.deployment_receipt_digest(), [0x73; 32]);
    assert_eq!(
        source.pipeline(),
        Some(layerx_programs::programs_source_verification())
    );
    assert_eq!(
        source.source(),
        &SourceStatus::Verified {
            source_digest: [0x74; 32],
            environment_digest: [0x75; 32]
        }
    );
    for expected in [
        RegistrySourceError::Binding,
        RegistrySourceError::Binding,
        RegistrySourceError::Binding,
        RegistrySourceError::Malformed,
        RegistrySourceError::Malformed,
        RegistrySourceError::Binding,
        RegistrySourceError::Binding,
    ] {
        assert_eq!(provider.read(&native, 1_500), Err(expected));
    }
    assert_eq!(
        provider.read(&native, 1_901),
        Err(RegistrySourceError::Binding)
    );
    assert_eq!(
        provider.read(&native, 1_500),
        Err(RegistrySourceError::Refused)
    );
    let mut bounded_config = config.clone();
    bounded_config["maximum_response_bytes"] = json!(32);
    let bounded_path = protected_file(
        &directory,
        "bounded.json",
        &serde_json::to_vec(&bounded_config).expect("bounded config"),
    );
    let bounded = RegistrySourceProvider::from_protected_config(&bounded_path)
        .expect("bounded source provider");
    assert_eq!(
        bounded.read(&native, 1_500),
        Err(RegistrySourceError::Bounds)
    );
    assert_eq!(
        provider
            .read(&native, 1_500)
            .expect("unpublished source")
            .source(),
        &SourceStatus::Unpublished
    );
    assert_eq!(
        provider
            .read(&native, 1_500)
            .expect("reproduction mismatch")
            .source(),
        &SourceStatus::Mismatch {
            expected: head.code_hash,
            reproduced: [0x78; 32]
        }
    );
    assert_eq!(
        server.join().expect("real registry server completed"),
        expected_requests
    );
    std::fs::set_permissions(&bearer_path, std::fs::Permissions::from_mode(0o644))
        .expect("make credential unprotected");
    assert!(matches!(
        RegistrySourceProvider::from_protected_config(&config_path),
        Err(RegistrySourceError::ProtectedFile)
    ));
    std::fs::set_permissions(&bearer_path, std::fs::Permissions::from_mode(0o600))
        .expect("restore protected credential");
    let mut invalid_config = config.clone();
    invalid_config["endpoint"] = json!(format!("http://{address}"));
    let invalid_path = protected_file(
        &directory,
        "plaintext.json",
        &serde_json::to_vec(&invalid_config).expect("invalid config"),
    );
    assert!(matches!(
        RegistrySourceProvider::from_protected_config(&invalid_path),
        Err(RegistrySourceError::Configuration)
    ));
    let mut invalid_config = config;
    invalid_config["deadline_ms"] = json!(60_001);
    let invalid_path = protected_file(
        &directory,
        "deadline.json",
        &serde_json::to_vec(&invalid_config).expect("invalid config"),
    );
    assert!(matches!(
        RegistrySourceProvider::from_protected_config(&invalid_path),
        Err(RegistrySourceError::Configuration)
    ));
}
