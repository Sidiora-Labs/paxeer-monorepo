//! Task 22.33.1535 repair record (audit 22.30.95, acceptance 22998.1).
//!
//! Finding 3: `cache_payload_substitution_is_refused_before_revalidation_and_degraded_serving`.
//! Finding 4: `cache_keys_storage_and_miss_only_tenants_obey_admission_bounds`.
//! Finding 5: `repeated_regressions_block_writes_until_both_high_water_marks_recover`.
//! Finding 7: `finality_consumer_uses_rungs_and_reopens_authenticated_evidence`.
//! Finding 31: `protocol_evidence::native_terminal_tests::native_type2_bytes_and_matching_node_receipt_survive_restart`.
//! Finding 50: `registry_source_contract_drives_mtls_and_refuses_unbound_responses`.
//! Finding 49: `external_signing_consumer_compiles_and_submits_exact_type2_bytes`.
//!
//! Wiring request, agent domain 9: spec/layerx-merge/interfaces.kvx must retire
//! `InclusionBundle/wait_for_level` and link the current finality.rs rung contract.
//! Wiring request, agent domain 9: spec/layerx-merge/interfaces.kvx must replace
//! legacy signing rows/callers with the typed EIP-1559 contract in sign.rs.
//! Wiring request, agent domain 9: services/agent/README.md must link/publish the
//! `registry_source.rs` module contract; spec/layerx-merge/interfaces.kvx must
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

fn require_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    result.unwrap_or_else(|error| panic!("{message}: {error:?}"))
}

fn require_some<T>(value: Option<T>, message: &str) -> T {
    value.unwrap_or_else(|| panic!("{message}"))
}

use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::cache::{revalidate, CacheError, CacheMetrics, CacheValue, EvidenceCache};
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

use layerx_agentd::{outbox, sign};

type FinalityWaiter =
    fn(&mut SubmissionProgress, [u8; 32], Rung, u64, u64, u64) -> Result<WaitResult, FinalityError>;

type ReceiptVerifier =
    fn([u8; 32], &[u8], &[ReceiptKeyEntry]) -> Result<VerifiedReceipt, FinalityError>;

type BlockVerifier = fn(
    &SequencerBlock,
    &layerx_agentd::finality::BlockCommit,
    &layerx_agentd::read::CheckpointHeader,
    &str,
    &[layerx_agentd::finality::Validator],
) -> Result<VerifiedBlock, FinalityError>;

type SubmissionVerifier = fn(
    &[u8],
    &SigningPreparation,
    &[u8; 20],
    &ModuleRegistry,
) -> Result<VerifiedSubmission, SigningError>;

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
        require_ok(
            std::fs::DirBuilder::new().mode(0o700).create(&root),
            "isolated private test directory",
        );
        let root = require_ok(std::fs::canonicalize(root), "canonical test directory");
        assert_eq!(
            require_ok(std::fs::metadata(&root), "private directory metadata")
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
        require_ok(
            std::fs::remove_dir_all(&self.0),
            "remove isolated test directory",
        );
    }
}

fn tenant(value: &str) -> TenantId {
    require_ok(TenantId::new(value), "bounded tenant")
}

fn checked<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("native terminal evidence: {error:?}"))
}

fn recorded_native_fixture() -> (
    layerx_agentd::sign::SigningPreparation,
    layerx_types::payload::ModuleRegistry,
    Vec<u8>,
    serde_json::Value,
    Vec<layerx_client::sequencer::ReceiptKeyEntry>,
) {
    use layerx_agentd::prepare::{
        evm::{AbiArgument, PrecompileCall, PrecompileTarget},
        prepare_bound, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest, EvmPrepared,
    };
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
    use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
    let document: serde_json::Value = checked(serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))));
    let case = require_some(
        document["cases"]
            .as_array()
            .unwrap_or_else(|| panic!("{}", "recorded cases"))
            .iter()
            .find(|case| case["id"] == "V1-layerxledger-pay-exact"),
        "native call recording",
    );
    let raw = decode_hex(require_some(
        case["rawTransaction"].as_str(),
        "raw transaction",
    ));
    let (call, signature) = checked(decode_signed(&raw));
    let actor_address = checked(recover_sender(&call, &signature));
    assert_eq!(
        &call.data[..4],
        &layerx_crypto::disclosure::call::ledger_selector(
            "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)"
        )
    );
    let transaction: EvmPrepared = checked(prepare_bound(
        EvmPreparationState {
            chain_id: 125,
            pending_nonce: 2,
            base_fee_per_gas: 1_000_000_000,
            head_height: 1001,
        },
        EvmPreparationDefaults {
            max_priority_fee_per_gas: 1_000_000_000,
            max_fee_per_gas: 3_000_000_000,
            maximum_payload_bytes: 2048,
        },
        EvmPrepareRequest {
            from: actor_address,
            call: PrecompileCall {
                target: PrecompileTarget::Ledger,
                selector: checked(call.data[..4].try_into()),
                arguments: call.data[4..]
                    .chunks_exact(32)
                    .map(|word| AbiArgument::Word(checked(word.try_into())))
                    .collect(),
            },
            expected_nonce: Some(2),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 2048,
        },
        call.gas_limit,
    ));
    assert_eq!(transaction.call, call);
    let prepared = layerx_agentd::sign::SigningPreparation {
        transaction,
        actor_address,
    };
    let seed: [u8; 32] = checked(
        decode_hex("8f2a55949038a9610f50fb23b5883af3b4ecb3c3bb792cbcefbd1542c692be63").try_into(),
    );
    let signed = checked(prepared.transaction.call.sign(&seed));
    assert_eq!(
        signed, raw,
        "producer must reproduce the node recorder's exact bytes"
    );
    let kind = checked(ActivityType::new(ModuleId::Asset, 5));
    let registry = checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Asset,
        &[kind],
    ))]));
    let pin = &case["pinnedKey"];
    let keys = vec![layerx_client::sequencer::ReceiptKeyEntry {
        key_id: checked(decode_hex(require_some(pin["keyId"].as_str(), "key id")).try_into()),
        public_key: checked(
            decode_hex(require_some(pin["publicKey"].as_str(), "public key")).try_into(),
        ),
        first_height: require_some(pin["firstHeight"].as_u64(), "first height"),
        last_height: require_some(pin["lastHeight"].as_u64(), "last height"),
    }];
    (
        prepared,
        registry,
        signed,
        case["receiptJson"].clone(),
        keys,
    )
}

fn decode_hex(value: &str) -> Vec<u8> {
    checked(hex::decode(value.strip_prefix("0x").unwrap_or(value)))
}

fn verified_node_receipt() -> (Vec<u8>, layerx_proof::receipt::VerifiedReceipt) {
    let (_, _, _, view, keys) = recorded_native_fixture();
    let bytes = decode_hex(require_some(view["receipt"].as_str(), "receipt bytes"));
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
    let decoded = require_ok(
        layerx_proof::receipt::verify_sequencer_signature(&bytes, pins.as_slice()),
        "verify the node producer's exact signed receipt",
    );
    let protocol = require_some(decoded.protocol(), "node protocol receipt");
    let authority = require_ok(
        layerx_proof::receipt::AuthorizedBatch::from_receipt_keys(
            &bytes,
            pins.as_slice(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
        ),
        "node receipt's height-bound published key",
    );
    let proof = require_ok(
        layerx_proof::receipt::verify(&bytes, &authority),
        "fully verify the node producer's receipt and economic invariants",
    );
    (bytes, proof)
}

#[test]
fn cache_payload_substitution_is_refused_before_revalidation_and_degraded_serving() {
    let (bytes, verified) = verified_node_receipt();
    let owner = tenant("cache-payload");
    let mut cache = require_ok(EvidenceCache::new(2, 16 * 1024), "bounded cache");
    let original = require_ok(
        CacheValue::try_from_receipt(bytes.clone(), &verified, 100, [1; 32]),
        "verified receipt payload",
    );
    require_ok(
        cache.insert(owner.clone(), b"receipt".to_vec(), original.clone()),
        "retain receipt",
    );
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
    let retained = require_ok(
        revalidate(
            &mut cache,
            owner.clone(),
            b"receipt",
            VerificationLevel::SEQUENCER_SIGNED,
            100,
            [1; 32],
            || panic!("fresh payload must not fetch"),
        ),
        "failed refresh preserved the authenticated payload",
    );
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
    finish_cache_payload_substitution_is_refused_before_revalidation_and_degraded_serving_1_repair_22_33_1535(FinishCachePayloadSubstitutionIsRefusedBeforeRevalidationAndDegradedServing1Repair22331535Context { bytes, owner, cache, retained, controller, serving_ran, attempted });
}

struct FinishCachePayloadSubstitutionIsRefusedBeforeRevalidationAndDegradedServing1Repair22331535Context
{
    bytes: Vec<u8>,
    owner: TenantId,
    cache: EvidenceCache,
    retained: CacheValue,
    controller: Controller,
    serving_ran: bool,
    attempted: Result<
        Result<layerx_agentd::degraded::DegradedRead, layerx_agentd::degraded::ReadError>,
        CacheError,
    >,
}

fn finish_cache_payload_substitution_is_refused_before_revalidation_and_degraded_serving_1_repair_22_33_1535(
    context: FinishCachePayloadSubstitutionIsRefusedBeforeRevalidationAndDegradedServing1Repair22331535Context,
) {
    let FinishCachePayloadSubstitutionIsRefusedBeforeRevalidationAndDegradedServing1Repair22331535Context { bytes, owner, mut cache, retained, controller, serving_ran, attempted } = context;

    assert_eq!(attempted, Err(CacheError::PayloadMismatch));
    assert!(!serving_ran);
    let served = require_ok(
        controller.serve_cached(&retained, Rung::Submitted),
        "degraded verified read",
    );
    assert_eq!(served.canonical_core_bytes, bytes);
    assert!(served.staleness.stale);

    // InclusionEvidence retains a root, not the authenticated leaf. Recheck an
    // actual nontrivial path before constructing a cache value from that leaf.
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/custody/daemon-light-credit-receipt");
    let read = |name: &str| require_ok(std::fs::read(root.join(name)), "committed native fixture");
    let receipt = read("credit.receipt");
    let header = read("header");
    let decoded = require_ok(
        layerx_wire::receipt::decode_batch_header(&header),
        "producer header",
    );
    let signature = require_ok(read("header.signature").try_into(), "header signature");
    let public = require_ok(read("sequencer.public").try_into(), "public key");
    let path = require_ok(
        layerx_proof::merkle::decode_proof(&read("receipt.proof")),
        "producer inclusion path",
    );
    assert!(path.leaf_count() > 1);
    let evidence = require_ok(
        layerx_proof::inclusion::verify_receipt(
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
        ),
        "real receipt inclusion verifier",
    );
    let included = require_ok(
        CacheValue::try_from_inclusion(receipt.clone(), &evidence, &path, 100, [1; 32]),
        "bound multi-leaf payload",
    );
    let mut other = receipt.clone();
    other[30] ^= 1;
    assert_eq!(
        CacheValue::try_from_inclusion(other.clone(), &evidence, &path, 101, [2; 32]),
        Err(CacheError::PayloadMismatch)
    );
    require_ok(
        cache.insert(owner.clone(), b"included".to_vec(), included.clone()),
        "cache proven inclusion",
    );
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
    let read = require_ok(
        controller.serve_cached(&included, Rung::Receipted),
        "degraded inclusion read",
    );
    assert_eq!(read.canonical_core_bytes, receipt);
    assert_eq!(read.evidence_id, evidence.header().digest());
}

#[test]
fn cache_keys_storage_and_miss_only_tenants_obey_admission_bounds() {
    let (bytes, proof) = verified_node_receipt();
    let value = require_ok(
        CacheValue::try_from_receipt(bytes.clone(), &proof, 1, [1; 32]),
        "value",
    );
    let mut cache = require_ok(
        EvidenceCache::with_limits(2, bytes.len() + 5, 2, 8),
        "cache limits",
    );
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
    require_ok(
        cache.insert(owner.clone(), oversized_capacity_key, value.clone()),
        "exact key plus value ceiling",
    );
    assert_eq!(cache.metrics(&owner).bytes, bytes.len() + 5);
    require_ok(
        cache.insert(owner.clone(), b"entry".to_vec(), value.clone()),
        "replacement subtracts key and value charge",
    );
    assert_eq!(cache.metrics(&owner).bytes, bytes.len() + 5);
    assert_eq!(cache.metrics(&owner).entries, 1);
    assert_eq!(
        cache.insert(owner.clone(), b"second".to_vec(), value.clone()),
        Err(CacheError::QuotaExceeded)
    );
    assert_eq!(cache.metrics(&owner).entries, 1);
    let miss_only = tenant("bounded-b");
    require_ok(
        cache.register_tenant(miss_only.clone()),
        "explicit bounded miss admission",
    );
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
    complete_cache_keys_storage_and_miss_only_tenants_obey_admission_bounds_1_repair_22_33_1535(
        &bytes, value, cache, &owner, &miss_only,
    );
}

fn complete_cache_keys_storage_and_miss_only_tenants_obey_admission_bounds_1_repair_22_33_1535(
    bytes: &[u8],
    value: CacheValue,
    mut cache: EvidenceCache,
    owner: &TenantId,
    miss_only: &TenantId,
) {
    assert_eq!(cache.metrics(miss_only).misses, 1);
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
        assert_eq!(cache.metrics(&unknown), CacheMetrics::default());
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
    assert_eq!(cache.metrics(owner).bytes, bytes.len() + 5);
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
            require_some(status.reference, "current observation").head_sequence,
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
    type Wait = FinalityWaiter;
    let wait: Wait = wait_for_rung::<SubmissionProgress>;
    let verify_receipt: ReceiptVerifier = VerifiedReceipt::verify;
    let _: BlockVerifier = VerifiedBlock::verify_with_header;
    let (prepared, registry, raw, view, keys) = recorded_native_fixture();
    let verified = require_ok(
        verify_before_submit(&raw, &prepared, &prepared.actor_address, &registry),
        "real signer consumer",
    );
    let hash = transaction_hash(verified.exact_bytes());
    let idempotency = verified.idempotency_key();
    let receipt = decode_hex(require_some(
        view["receipt"].as_str(),
        "recorded signed receipt",
    ));
    let receipt = require_ok(
        verify_receipt(hash, &receipt, &keys),
        "published key receipt API",
    );
    let mut progress = SubmissionProgress::accepted(idempotency, hash);
    assert_eq!(
        require_ok(
            wait(&mut progress, idempotency, Rung::Submitted, 10, 20, 1),
            "accepted rung"
        )
        .reached,
        Rung::Submitted
    );
    require_ok(
        progress.observe_receipt(receipt.canonical_bytes(), &keys),
        "receipt consumer",
    );
    assert_eq!(
        require_ok(
            wait(&mut progress, idempotency, Rung::Receipted, 10, 20, 1),
            "receipt rung"
        )
        .reached,
        Rung::Receipted
    );
    complete_finality_consumer_uses_rungs_and_reopens_authenticated_evidence_1_repair_22_33_1535(
        CompleteFinalityConsumerUsesRungsAndReopensAuthenticatedEvidence1Repair22331535Context {
            wait,
            keys,
            hash,
            idempotency,
            receipt,
            progress,
        },
    );
}

struct CompleteFinalityConsumerUsesRungsAndReopensAuthenticatedEvidence1Repair22331535Context {
    wait: FinalityWaiter,
    keys: Vec<ReceiptKeyEntry>,
    hash: [u8; 32],
    idempotency: [u8; 32],
    receipt: VerifiedReceipt,
    progress: SubmissionProgress,
}

fn complete_finality_consumer_uses_rungs_and_reopens_authenticated_evidence_1_repair_22_33_1535(
    context: CompleteFinalityConsumerUsesRungsAndReopensAuthenticatedEvidence1Repair22331535Context,
) {
    let CompleteFinalityConsumerUsesRungsAndReopensAuthenticatedEvidence1Repair22331535Context {
        wait,
        keys,
        hash,
        idempotency,
        receipt,
        mut progress,
    } = context;

    let timeout = require_ok(
        wait(&mut progress, idempotency, Rung::Committed, 10, 20, 3),
        "deadline retains actual rung",
    );
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
    let mut store = require_ok(Store::open(&directory.0), "durable store");
    let record = require_ok(
        augment_verified(&mut store, tenant("finality"), idempotency, &receipt, None),
        "durable augmentation",
    );
    assert_eq!(record.tx_hash, hash);
    assert_eq!(record.rung, Rung::Receipted);
    drop(store);
    let store = require_ok(Store::open(&directory.0), "reopen finality store");
    let mut restored = require_ok(
        SubmissionProgress::restore(
            &store,
            tenant("finality"),
            idempotency,
            &keys[0],
            "paxeer-125",
            &[],
        ),
        "reverify durable evidence",
    );
    assert_eq!(
        require_ok(
            wait(&mut restored, idempotency, Rung::Receipted, 30, 40, 2),
            "restored rung"
        )
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
    use std::task::{Context, Poll, Wake, Waker};

    async fn consume_self_sign(
        prepared: &SigningPreparation,
        registry: &ModuleRegistry,
    ) -> Result<sign::SignedPreparation, SigningError> {
        sign::self_sign(None, prepared, registry, 1000, 1).await
    }

    use std::future::Future;

    struct WakeOnce;
    impl Wake for WakeOnce {
        fn wake(self: std::sync::Arc<Self>) {}
    }

    let package: fn(&SigningPreparation) -> Result<ExternalSigningPackage, SigningError> = external;
    let attach: fn(&SigningPreparation, [u8; 65]) -> Result<Vec<u8>, SigningError> =
        attach_external_signature;
    let verify: SubmissionVerifier = verify_before_submit;
    let (prepared, registry, recorded, _, _) = recorded_native_fixture();
    let external = require_ok(package(&prepared), "external package consumer");
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
        external.disclosure_digest,
        prepared.transaction.disclosure_digest
    );
    let (_, signature) = require_ok(decode_signed(&recorded), "external signer's real signature");
    let attached = require_ok(
        attach(&prepared, *signature.as_bytes()),
        "65-byte external signature consumer",
    );
    assert_eq!(attached, recorded);
    let submission = require_ok(
        verify(&attached, &prepared, &prepared.actor_address, &registry),
        "20-byte sender verification consumer",
    );
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
    let mut store = require_ok(Store::open(&directory.0), "external consumer store");
    let mut outbox = outbox::Outbox::default();
    let hash = require_ok(
        outbox.enqueue_verified(&mut store, tenant("external"), submission),
        "real outbox consumer",
    );
    assert_eq!(
        require_ok(outbox.bytes_for_transmission(hash), "exact transmit bytes"),
        attached
    );
    // Compile the async contract as well, with real provisioning refusal before signing.

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
        bn::{BigNum, MsbOption},
        pkey::PKey,
        rsa::Rsa,
        x509::{X509NameBuilder, X509},
    };
    let key = require_ok(
        PKey::from_rsa(require_ok(Rsa::generate(2048), "RSA key")),
        "TLS key",
    );
    let mut name = require_ok(X509NameBuilder::new(), "certificate name");
    require_ok(
        name.append_entry_by_text(
            "CN",
            if client {
                "registry-agent"
            } else {
                "localhost"
            },
        ),
        "identity name",
    );
    let name = name.build();
    let mut cert = require_ok(X509::builder(), "certificate builder");
    require_ok(cert.set_version(2), "certificate version");
    let mut serial = require_ok(BigNum::new(), "serial");
    require_ok(
        serial.rand(128, MsbOption::MAYBE_ZERO, false),
        "random serial",
    );
    complete_certificate_2_repair_22_33_1535(CompleteCertificate2Repair22331535Context {
        issuer,
        client,
        key,
        name,
        cert,
        serial,
    })
}

struct CompleteCertificate2Repair22331535Context<'a> {
    issuer: Option<(
        &'a openssl::x509::X509,
        &'a openssl::pkey::PKey<openssl::pkey::Private>,
    )>,
    client: bool,
    key: openssl::pkey::PKey<openssl::pkey::Private>,
    name: openssl::x509::X509Name,
    cert: openssl::x509::X509Builder,
    serial: openssl::bn::BigNum,
}

fn complete_certificate_2_repair_22_33_1535(
    context: CompleteCertificate2Repair22331535Context<'_>,
) -> (
    openssl::x509::X509,
    openssl::pkey::PKey<openssl::pkey::Private>,
) {
    use openssl::{
        asn1::Asn1Time,
        hash::MessageDigest,
        x509::extension::{BasicConstraints, KeyUsage},
    };

    let CompleteCertificate2Repair22331535Context {
        issuer,
        client,
        key,
        name,
        mut cert,
        serial,
    } = context;

    require_ok(
        cert.set_serial_number(
            &serial
                .to_asn1_integer()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "serial integer")),
        ),
        "serial number",
    );
    require_ok(cert.set_subject_name(&name), "subject");
    require_ok(
        cert.set_issuer_name(issuer.map_or(&name, |(ca, _)| ca.subject_name())),
        "issuer",
    );
    require_ok(cert.set_pubkey(&key), "public key");
    require_ok(
        cert.set_not_before(
            &Asn1Time::days_from_now(0)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "not before")),
        ),
        "validity start",
    );
    require_ok(
        cert.set_not_after(
            &Asn1Time::days_from_now(1)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "not after")),
        ),
        "validity end",
    );
    if let Some((ca, signing_key)) = issuer {
        require_ok(
            cert.append_extension(require_ok(
                BasicConstraints::new().critical().build(),
                "leaf constraints",
            )),
            "leaf extension",
        );
        complete_certificate_1_repair_22_33_1535(client, &mut cert, ca, signing_key);
    } else {
        require_ok(
            cert.append_extension(require_ok(
                BasicConstraints::new().critical().ca().build(),
                "CA constraints",
            )),
            "CA extension",
        );
        require_ok(
            cert.append_extension(require_ok(
                KeyUsage::new()
                    .critical()
                    .key_cert_sign()
                    .crl_sign()
                    .build(),
                "CA usage",
            )),
            "CA usage extension",
        );
        require_ok(cert.sign(&key, MessageDigest::sha256()), "root signature");
    }
    (cert.build(), key)
}

fn complete_certificate_1_repair_22_33_1535(
    client: bool,
    cert: &mut openssl::x509::X509Builder,
    ca: &openssl::x509::X509,
    signing_key: &openssl::pkey::PKey<openssl::pkey::Private>,
) {
    use openssl::{
        hash::MessageDigest,
        x509::extension::{ExtendedKeyUsage, KeyUsage, SubjectAlternativeName},
    };

    require_ok(
        cert.append_extension(require_ok(
            KeyUsage::new()
                .digital_signature()
                .key_encipherment()
                .build(),
            "leaf usage",
        )),
        "leaf usage extension",
    );
    let mut extended = ExtendedKeyUsage::new();
    if client {
        extended.client_auth();
    } else {
        extended.server_auth();
    }
    require_ok(
        cert.append_extension(require_ok(extended.build(), "extended key usage")),
        "extended usage extension",
    );
    if !client {
        let san = require_ok(
            SubjectAlternativeName::new()
                .dns("localhost")
                .ip("127.0.0.1")
                .build(&cert.x509v3_context(Some(ca), None)),
            "endpoint identities",
        );
        require_ok(cert.append_extension(san), "subject alternative names");
    }
    require_ok(
        cert.sign(signing_key, MessageDigest::sha256()),
        "CA signs identity",
    );
}

fn protected_file(directory: &Directory, name: &str, bytes: &[u8]) -> PathBuf {
    use std::io::Write;
    use std::os::unix::fs::OpenOptionsExt;
    let path = directory.0.join(name);
    let mut file = require_ok(
        std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&path),
        "protected material",
    );
    require_ok(file.write_all(bytes), "write protected material");
    path
}

#[test]
fn registry_source_contract_drives_mtls_and_refuses_unbound_responses() {
    use layerx_agentd::ops::program::{ProgramDiscovery, ProgramInterfaceRead};

    use layerx_programs::{
        InterfaceEntryPoint, ProgramId, ProgramInterface, ProgramLifecycle, ValueSchema, ValueType,
    };

    use serde_json::json;
    use sha2::{Digest, Sha256};

    // The production interface producer validates this committed Rust fixture's
    // WASM module and binds the exported call ABI before it crosses the registry.
    const WASM: &[u8] = &[
        0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3,
        2, 0, 1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a', b'y',
        b'e', b'r', b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm', b'e',
        b'm', b'o', b'r', b'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
    ];
    let interface = require_ok(
        ProgramInterface::bind(
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
        ),
        "real program-interface producer",
    );
    let head = ProgramDiscovery {
        program: require_ok(ProgramId::new([0x31; 32]), "program id"),
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
    finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_4_repair_22_33_1535(
        head, native, &valid,
    );
}

fn finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_4_repair_22_33_1535(
    head: layerx_agentd::ops::program::ProgramDiscovery,
    native: layerx_agentd::ops::program::ProgramInterfaceRead,
    valid: &serde_json::Value,
) {
    use openssl::ssl::{SslAcceptor, SslMethod, SslVerifyMode};
    use serde_json::{json, Value};

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
    responses.push((200, valid.clone()));
    // expired at the caller's now
    responses.push((403, valid.clone()));
    responses.push((200, valid.clone()));
    // exceeds a different consumer's body bound
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
    let expected_client = require_ok(client_certificate.to_der(), "client certificate identity");
    let mut acceptor = require_ok(
        SslAcceptor::mozilla_intermediate_v5(SslMethod::tls_server()),
        "real TLS server",
    );
    require_ok(
        acceptor.set_certificate(&server_certificate),
        "server certificate",
    );
    require_ok(acceptor.set_private_key(&server_key), "server key");
    require_ok(acceptor.check_private_key(), "server identity binding");
    require_ok(
        acceptor.cert_store_mut().add_cert(ca.clone()),
        "trusted client issuer",
    );
    acceptor.set_verify(SslVerifyMode::PEER | SslVerifyMode::FAIL_IF_NO_PEER_CERT);
    require_ok(
        acceptor.set_session_id_context(b"repair-22-33-1535-registry"),
        "bind resumed sessions to this authenticated registry",
    );
    finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_3_repair_22_33_1535(
        FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses3Repair22331535Context {
            head,
            native,
            responses,
            expected_requests,
            directory,
            ca,
            client_certificate,
            client_key,
            acceptor,
        },
        expected_client,
    );
}

struct FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses3Repair22331535Context {
    head: layerx_agentd::ops::program::ProgramDiscovery,
    native: layerx_agentd::ops::program::ProgramInterfaceRead,
    responses: Vec<(u16, serde_json::Value)>,
    expected_requests: usize,
    directory: Directory,
    ca: openssl::x509::X509,
    client_certificate: openssl::x509::X509,
    client_key: openssl::pkey::PKey<openssl::pkey::Private>,
    acceptor: openssl::ssl::SslAcceptorBuilder,
}

fn finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_3_repair_22_33_1535(
    context: FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses3Repair22331535Context,
    expected_client: Vec<u8>,
) {
    use std::io::{BufRead, BufReader, Write};

    let FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses3Repair22331535Context {
        head,
        native,
        responses,
        expected_requests,
        directory,
        ca,
        client_certificate,
        client_key,
        acceptor,
    } = context;

    let acceptor = acceptor.build();
    let listener = require_ok(
        std::net::TcpListener::bind("127.0.0.1:0"),
        "loopback registry",
    );
    let address = require_ok(listener.local_addr(), "registry address");
    let requested_program = hex::encode(head.program.bytes());
    let server = std::thread::spawn(move || {
        for (status, body) in responses {
            let (socket, _) = require_ok(listener.accept(), "registry connection");
            require_ok(
                socket.set_read_timeout(Some(std::time::Duration::from_secs(5))),
                "read timeout",
            );
            require_ok(
                socket.set_write_timeout(Some(std::time::Duration::from_secs(5))),
                "write timeout",
            );
            let mut tls = require_ok(acceptor.accept(socket), "mTLS handshake");
            assert_eq!(
                tls.ssl().verify_result(),
                openssl::x509::X509VerifyResult::OK
            );
            assert_eq!(
                require_ok(
                    tls.ssl()
                        .peer_certificate()
                        .unwrap_or_else(|| panic!("{}", "client identity is mandatory"))
                        .to_der(),
                    "peer DER"
                ),
                expected_client
            );
            let mut reader = BufReader::new(&mut tls);
            let mut line = String::new();
            require_ok(reader.read_line(&mut line), "request line");
            assert_eq!(
                line,
                format!("GET /v1/programs/registry/{requested_program}/interface HTTP/1.1\r\n")
            );
            let mut bearer = false;
            loop {
                line.clear();
                require_ok(reader.read_line(&mut line), "request header");
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
            let body = require_ok(serde_json::to_vec(&body), "registry body");
            require_ok(write!(tls, "HTTP/1.1 {status} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", if status == 200 { "OK" } else { "Forbidden" }, body.len()), "HTTP response");
            require_ok(tls.write_all(&body), "registry response bytes");
            require_ok(tls.flush(), "registry response flush");
        }
        expected_requests
    });
    complete_finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_3_repair_22_33_1535_1_repair_22_33_1535(CompleteFinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses3Repair223315351Repair22331535Context { head, native, expected_requests, directory, ca, client_certificate, client_key, address, server });
}

struct CompleteFinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses3Repair223315351Repair22331535Context
{
    head: layerx_agentd::ops::program::ProgramDiscovery,
    native: layerx_agentd::ops::program::ProgramInterfaceRead,
    expected_requests: usize,
    directory: Directory,
    ca: openssl::x509::X509,
    client_certificate: openssl::x509::X509,
    client_key: openssl::pkey::PKey<openssl::pkey::Private>,
    address: std::net::SocketAddr,
    server: std::thread::JoinHandle<usize>,
}

fn complete_finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_3_repair_22_33_1535_1_repair_22_33_1535(
    context: CompleteFinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses3Repair223315351Repair22331535Context,
) {
    let CompleteFinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses3Repair223315351Repair22331535Context { head, native, expected_requests, directory, ca, client_certificate, client_key, address, server } = context;

    let ca_path = protected_file(&directory, "ca.der", &require_ok(ca.to_der(), "CA DER"));
    let certificate_path = protected_file(
        &directory,
        "client.pem",
        &require_ok(client_certificate.to_pem(), "client PEM"),
    );
    finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_2_repair_22_33_1535(
        FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses2Repair22331535Context {
            head,
            native,
            expected_requests,
            directory,
            client_key,
            address,
            server,
            ca_path,
            certificate_path,
        },
    );
}

struct FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses2Repair22331535Context {
    head: layerx_agentd::ops::program::ProgramDiscovery,
    native: layerx_agentd::ops::program::ProgramInterfaceRead,
    expected_requests: usize,
    directory: Directory,
    client_key: openssl::pkey::PKey<openssl::pkey::Private>,
    address: std::net::SocketAddr,
    server: std::thread::JoinHandle<usize>,
    ca_path: PathBuf,
    certificate_path: PathBuf,
}

fn finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_2_repair_22_33_1535(
    context: FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses2Repair22331535Context,
) {
    use layerx_agentd::registry_source::{RegistrySourceError, RegistrySourceProvider};
    use layerx_programs::SourceStatus;

    use serde_json::json;

    let FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses2Repair22331535Context {
        head,
        native,
        expected_requests,
        directory,
        client_key,
        address,
        server,
        ca_path,
        certificate_path,
    } = context;

    let key_path = protected_file(
        &directory,
        "client.key",
        &require_ok(client_key.private_key_to_pem_pkcs8(), "private PEM"),
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
        &require_ok(serde_json::to_vec(&config), "config"),
    );
    let provider = require_ok(
        RegistrySourceProvider::from_protected_config(&config_path),
        "published protected config contract",
    );
    let source = require_ok(provider.read(&native, 1_500), "verified registry source");
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
    finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_1_repair_22_33_1535(
        FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses1Repair22331535Context {
            head,
            native,
            expected_requests,
            directory,
            address,
            server,
            bearer_path,
            config,
            config_path,
            provider,
        },
    );
}

struct FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses1Repair22331535Context {
    head: layerx_agentd::ops::program::ProgramDiscovery,
    native: layerx_agentd::ops::program::ProgramInterfaceRead,
    expected_requests: usize,
    directory: Directory,
    address: std::net::SocketAddr,
    server: std::thread::JoinHandle<usize>,
    bearer_path: PathBuf,
    config: serde_json::Value,
    config_path: PathBuf,
    provider: layerx_agentd::registry_source::RegistrySourceProvider,
}

fn finish_registry_source_contract_drives_mtls_and_refuses_unbound_responses_1_repair_22_33_1535(
    context: FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses1Repair22331535Context,
) {
    use layerx_agentd::registry_source::{RegistrySourceError, RegistrySourceProvider};
    use layerx_programs::SourceStatus;

    use serde_json::json;

    use std::os::unix::fs::PermissionsExt;

    let FinishRegistrySourceContractDrivesMtlsAndRefusesUnboundResponses1Repair22331535Context {
        head,
        native,
        expected_requests,
        directory,
        address,
        server,
        bearer_path,
        config,
        config_path,
        provider,
    } = context;

    let mut bounded_config = config.clone();
    bounded_config["maximum_response_bytes"] = json!(32);
    let bounded_path = protected_file(
        &directory,
        "bounded.json",
        &require_ok(serde_json::to_vec(&bounded_config), "bounded config"),
    );
    let bounded = require_ok(
        RegistrySourceProvider::from_protected_config(&bounded_path),
        "bounded source provider",
    );
    assert_eq!(
        bounded.read(&native, 1_500),
        Err(RegistrySourceError::Bounds)
    );
    assert_eq!(
        require_ok(provider.read(&native, 1_500), "unpublished source").source(),
        &SourceStatus::Unpublished
    );
    assert_eq!(
        require_ok(provider.read(&native, 1_500), "reproduction mismatch").source(),
        &SourceStatus::Mismatch {
            expected: head.code_hash,
            reproduced: [0x78; 32]
        }
    );
    assert_eq!(
        require_ok(server.join(), "real registry server completed"),
        expected_requests
    );
    require_ok(
        std::fs::set_permissions(&bearer_path, std::fs::Permissions::from_mode(0o644)),
        "make credential unprotected",
    );
    assert!(matches!(
        RegistrySourceProvider::from_protected_config(&config_path),
        Err(RegistrySourceError::ProtectedFile)
    ));
    require_ok(
        std::fs::set_permissions(&bearer_path, std::fs::Permissions::from_mode(0o600)),
        "restore protected credential",
    );
    let mut invalid_config = config.clone();
    invalid_config["endpoint"] = json!(format!("http://{address}"));
    let invalid_path = protected_file(
        &directory,
        "plaintext.json",
        &require_ok(serde_json::to_vec(&invalid_config), "invalid config"),
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
        &require_ok(serde_json::to_vec(&invalid_config), "invalid config"),
    );
    assert!(matches!(
        RegistrySourceProvider::from_protected_config(&invalid_path),
        Err(RegistrySourceError::Configuration)
    ));
}
