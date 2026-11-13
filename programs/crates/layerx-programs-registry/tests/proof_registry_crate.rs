//! Canonical deployment evidence is published, read, verified, and resolved by
//! the registry. Signed validator commitments authenticate the program,
//! lifecycle, and interface leaves. Identical publication is idempotent;
//! conflicting bytes preserve the original admission across restart.

#[allow(dead_code)]
#[path = "support/mod.rs"]
pub mod support;

#[path = "support/assertions.rs"]
mod assertions;
use assertions::TestResultExt;

use std::fs::{self, File};
use std::io::Write as _;
use std::os::unix::fs::DirBuilderExt as _;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use ed25519_dalek::SigningKey;
use layerx_programs::{
    chain_proof::validators::TrustedValidator, DeploymentProof, ProgramBundleError,
    ProgramHeadKind, ProgramId, ProgramLifecycle, ProtocolDeploymentVerifier,
    ProtocolEvidenceError, ReceiptEvidenceDigest, Registry, RegistryError,
    VerifiedDeploymentEvidence, VerifiedProgramCatalog,
};
use layerx_programs_runtime::{Deploy, LifecycleRefusal, UpgradePolicy};
use support::{
    code_hash, deploy_fixture, produced_deployment, program, verifier_for_fixture, AUTHORITY, NOW,
    WASM_V1,
};

struct Store(PathBuf);

impl Store {
    fn open() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let root = std::env::temp_dir().join(format!(
            "registry-crate-proof-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .or_panic("private proof store");
        Self(root)
    }
}

impl Drop for Store {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).or_panic("remove proof store");
    }
}

fn publish_producer(
    store: &Store,
    admission: &std::path::Path,
    bundle_path: &std::path::Path,
    policy: UpgradePolicy,
) -> (
    layerx_programs::DeploymentRecord,
    layerx_programs::chain_proof::ChainProofVerifier,
    [u8; 32],
    [u8; 32],
) {
    let (mut runtime, expected, fixture) = produced_deployment(WASM_V1, policy, 70, NOW - 100);
    let verifier = verifier_for_fixture(&fixture, 1_000);
    let evidence: VerifiedDeploymentEvidence = verifier
        .verify_deployment(&fixture.proof, NOW)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "producer authenticated by the public registry verifier"
            )
        });
    assert_eq!(evidence.record(), &expected);
    assert_eq!(evidence.module(), WASM_V1);
    assert_eq!(evidence.abi_version(), 1);
    assert_eq!(evidence.lifecycle(), ProgramLifecycle::Active);
    assert_eq!(
        evidence.proof().claimed_receipt_digest(),
        Ok(evidence.receipt_digest())
    );
    assert_eq!(
        runtime.deploy(Deploy {
            program: program(),
            code_hash: code_hash(WASM_V1),
            wasm: WASM_V1.to_vec(),
            abi_version: 1,
            upgrade_policy: policy,
        }),
        Err(LifecycleRefusal::AlreadyDeployed)
    );

    // These are the actual bytes consumed by the hosted journal and agent.
    fixture
        .proof
        .publish_canonical(admission)
        .or_panic("publish admission");
    let bundle = fixture
        .program_bundle()
        .canonical_encoding()
        .or_panic("producer bundle");
    let mut transport = File::create_new(bundle_path).or_panic("create bundle artifact");
    transport
        .write_all(&bundle)
        .or_panic("write producer bytes");
    transport.sync_all().or_panic("sync bundle artifact");
    drop(transport);
    File::open(&store.0)
        .or_panic("store directory")
        .sync_all()
        .or_panic("sync store");
    let trust = fixture.chain.verifier.clone();
    let receipt = evidence.receipt_digest();
    let state_root = evidence.state_root();
    // End every producer component before constructing fresh consumers.
    drop(evidence);
    drop(verifier);
    drop(fixture);
    drop(runtime);
    drop(bundle);

    (expected, trust, receipt, state_root)
}

fn verify_recovered_consumers(
    evidence: &VerifiedDeploymentEvidence,
    expected: &layerx_programs::DeploymentRecord,
    receipt: [u8; 32],
) -> (Registry, VerifiedProgramCatalog) {
    let mut registry = Registry::new();
    registry
        .record_verified_deployment(evidence)
        .or_panic("real registry consumer");
    let resolved = registry
        .resolve_deployment(evidence.clone())
        .or_panic("resolve persisted deployment");
    assert_eq!(resolved.module(), expected.module);
    assert_eq!(resolved.code_hash(), expected.new_code_hash);
    assert_eq!(resolved.receipt_digest(), receipt);
    assert_eq!(registry.latest_version(program()), Ok(1));
    assert_eq!(
        registry.record_verified_deployment(evidence),
        Err(RegistryError::VersionHistoryMismatch)
    );
    assert_eq!(
        registry
            .entry_for_wind_down(program())
            .or_panic("retained history")
            .versions
            .len(),
        1
    );
    let mut catalog = VerifiedProgramCatalog::declared().or_panic("real executable consumer");
    catalog
        .admit(resolved)
        .or_panic("validate and admit recovered module");
    assert_eq!(catalog.code_hash(program()), Some(expected.new_code_hash));
    assert_eq!(catalog.abi_version(program()), Some(1));
    assert_eq!(catalog.receipt_digest(program()), Some(receipt));

    (registry, catalog)
}

fn verify_bundle_consumers(
    verifier: &ProtocolDeploymentVerifier,
    recovered: &DeploymentProof,
    evidence: &VerifiedDeploymentEvidence,
    bundle_path: &std::path::Path,
    receipt: [u8; 32],
    state_root: [u8; 32],
) {
    let payload = fs::read(bundle_path).or_panic("reopen producer bundle");
    let state = &recovered.state;
    for kind in [ProgramHeadKind::Ordinary, ProgramHeadKind::Maintenance] {
        let chain = verifier
            .verify_current_chain_head(
                kind,
                &state.commitment,
                &state.transaction,
                &state.transaction_proof,
                NOW,
            )
            .or_panic("authenticate head");
        assert_eq!(
            ProgramHeadKind::from_wire_byte(kind.wire_byte()),
            Some(kind)
        );
        assert_eq!(chain.head_kind(), kind);
        assert_eq!(chain.receipt_digest(), receipt);
        assert_eq!(
            chain.receipt_evidence_digest(),
            match kind {
                ProgramHeadKind::Ordinary =>
                    ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(receipt),
                ProgramHeadKind::Maintenance =>
                    ReceiptEvidenceDigest::MaintenanceReceiptSha256(receipt),
            }
        );
        let observed = verifier
            .verify_current_program_bundle_bytes(&payload, &chain, program(), NOW)
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "agent reader's actual bundle verification function"
                )
            });
        assert_eq!(observed.verify_discovery(program(), NOW, 1_000), Ok(()));
        assert_eq!(
            observed.program_head().abi_version(),
            evidence.abi_version()
        );
        assert_eq!(observed.program_head().code_hash(), evidence.code_hash());
        assert_eq!(observed.program_head().state_root(), state_root);
        assert_eq!(
            observed.interface().interface,
            support::fixture_interface(WASM_V1)
        );
        let unknown = ProgramId::new([0x32; 32]).or_panic("nonzero identifier");
        assert_eq!(
            verifier.verify_current_program_bundle_bytes(&payload, &chain, unknown, NOW),
            Err(ProgramBundleError::ProgramMismatch)
        );
        assert_eq!(
            observed.verify_discovery(unknown, NOW, 1_000),
            Err(ProgramBundleError::ProgramMismatch)
        );
        assert_eq!(
            observed.verify_discovery(program(), NOW + 1_000, 1_000),
            Err(ProgramBundleError::Evidence(ProtocolEvidenceError::Stale))
        );
        assert!(verifier
            .verify_current_program_bundle_bytes(
                &payload[..payload.len() - 1],
                &chain,
                program(),
                NOW
            )
            .is_err());
    }
    for byte in 2..=u8::MAX {
        assert_eq!(ProgramHeadKind::from_wire_byte(byte), None);
    }
}

fn verify_conflicting_publication(
    admission: &std::path::Path,
    recovered: &DeploymentProof,
    verifier: &ProtocolDeploymentVerifier,
    trust: &layerx_programs::chain_proof::ChainProofVerifier,
    policy: UpgradePolicy,
    receipt: [u8; 32],
) {
    let wrong_validator = TrustedValidator::new(
        SigningKey::from_bytes(&[0x99; 32])
            .verifying_key()
            .to_bytes(),
        1,
    );
    let foreign =
        ProtocolDeploymentVerifier::from_chain_trust(&trust.chain_id, vec![wrong_validator], 1_000)
            .or_panic("different protected validator");
    assert_eq!(
        foreign.verify_deployment(recovered, NOW),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );

    let before = fs::read(admission).or_panic("committed admission");
    recovered
        .publish_canonical(admission)
        .or_panic("identical producer retry is idempotent");
    let alternate = deploy_fixture(WASM_V1, policy, 71, NOW - 50);
    let alternate_evidence = verifier
        .verify_deployment(&alternate.proof, NOW)
        .or_panic("second valid inclusion");
    assert_eq!(alternate_evidence.receipt_digest(), receipt);
    assert_ne!(&alternate.proof, recovered);
    assert_eq!(
        alternate
            .proof
            .publish_canonical(admission)
            .err()
            .unwrap_or_else(|| panic!("{}", "conflicting retry refuses"))
            .kind(),
        std::io::ErrorKind::AlreadyExists
    );
    assert_eq!(fs::read(admission).or_panic("preserved admission"), before);
}

#[test]
fn producer_transport_restart_and_real_registry_consumers() {
    let store = Store::open();
    let admission = store.0.join("deployment.admission");
    let bundle_path = store.0.join("program.bundle");
    let policy = UpgradePolicy::Authority(AUTHORITY);
    let (expected, trust, receipt, state_root) =
        publish_producer(&store, &admission, &bundle_path, policy);
    let recovered = DeploymentProof::read_canonical(&admission).or_panic("reopen admission");
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &trust.chain_id,
        trust.validators.validators().to_vec(),
        1_000,
    )
    .or_panic("restart protected verifier");
    let evidence = verifier
        .verify_deployment(&recovered, NOW)
        .or_panic("reverify recovered proof");
    assert_eq!(evidence.record(), &expected);
    assert_eq!(evidence.receipt_digest(), receipt);
    assert_eq!(evidence.state_root(), state_root);
    assert_eq!(evidence.module(), WASM_V1);
    assert_eq!(evidence.abi_version(), 1);

    let (registry, catalog) = verify_recovered_consumers(&evidence, &expected, receipt);
    verify_bundle_consumers(
        &verifier,
        &recovered,
        &evidence,
        &bundle_path,
        receipt,
        state_root,
    );
    verify_conflicting_publication(&admission, &recovered, &verifier, &trust, policy, receipt);
    drop(registry);
    drop(catalog);
    drop(verifier);
    assert_eq!(
        DeploymentProof::read_canonical(&admission).or_panic("second restart"),
        recovered
    );
}

#[test]
fn journal_commit_point_preserves_an_existing_envelope() {
    let store = Store::open();
    let path = store.0.join("committed.envelope");
    let staged = store.0.join("staged.envelope");
    let (_, _, fixture) = produced_deployment(WASM_V1, UpgradePolicy::Immutable, 80, NOW - 100);
    let bytes = fixture
        .proof
        .canonical_encoding()
        .or_panic("real producer bytes");
    // Exercise the exact commit function called by FileDeploymentJournal::publish,
    // including its duplicate and conflict branches, without a substitute writer.
    fs::write(&staged, &bytes).or_panic("stage producer output");
    File::open(&staged)
        .or_panic("staged file")
        .sync_all()
        .or_panic("sync staging");
    DeploymentProof::commit_staged_file(&staged, &path).or_panic("journal commit point");
    assert!(!staged.exists());
    assert_eq!(fs::read(&path).or_panic("committed bytes"), bytes);
    fixture
        .proof
        .publish_canonical(&path)
        .or_panic("idempotent commit");
    let other = deploy_fixture(WASM_V1, UpgradePolicy::Immutable, 81, NOW - 50);
    fs::write(
        &staged,
        other
            .proof
            .canonical_encoding()
            .or_panic("alternate producer bytes"),
    )
    .or_panic("stage conflicting admission");
    assert_eq!(
        DeploymentProof::commit_staged_file(&staged, &path)
            .err()
            .unwrap_or_else(|| panic!("{}", "immutable journal"))
            .kind(),
        std::io::ErrorKind::AlreadyExists
    );
    assert_eq!(fs::read(&path).or_panic("original bytes survive"), bytes);
    assert!(
        staged.exists(),
        "refused staging remains available for diagnosis"
    );
}

#[allow(dead_code)]
#[path = "support/repair_22_33_221.rs"]
mod repair_22_33_221;

#[test]
fn node_deployment_upgrade_replay_preserves_identity() {
    repair_22_33_221::restart_recovery(false);
}
