use std::collections::BTreeMap;

pub mod support;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::{
    chain_evidence::{
        deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, programs_state_key, wind_down::ChainWindDownProof,
    },
    chain_proof::{
        decode::decode_commitment_proof,
        merkle::{hash_from_byte_slices, SimpleProof},
        validators::{TrustedValidator, ValidatorSet},
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
        ChainProofError,
    },
    DeploymentProof, ProgramInterface, ProgramLifecycleProof, ProgramStateProof,
    ProtocolDeploymentVerifier,
};
use layerx_programs::{
    BuildEnvironment, DeploymentJournal, DeploymentRecord, ExecutableAdmissionError,
    JournalReadAuthority, ObservedHead, ProgramLifecycle, ProtocolEvidenceError, PublishedSource,
    ReadFreshness, Registry, RegistryError, RegistryReadAuthority, ReproducibleBuild, SourceStatus,
    VerifiedProgramCatalog,
};
use layerx_programs_runtime::{
    hash_bytes, ActivityBudgetBinding, CompositionRules, Deploy, HashAlgorithm, Lifecycle,
    ProgramId, ProgramVersion, UpgradePolicy, ABI_V1_VERSION, ABI_V2_VERSION, ABI_VERSION,
};

use support::{
    chain::ChainFixture, code_hash as fixture_code_hash, deploy_fixture, deprecated_state,
    program as fixture_program, upgrade_fixture, verifier_for_fixture, wrong_abi_fixture,
    AUTHORITY, NOW, WASM_V1 as PROTOCOL_WASM_V1, WASM_V2 as PROTOCOL_WASM_V2,
};

const PROGRAM: [u8; 32] = [0x31; 32];
const WASM_V1: &[u8] = &[0, 97, 115, 109, 1, 0, 0, 0];
const WASM_V2: &[u8] = &[0, 97, 115, 109, 1, 0, 0, 0, 0, 3, 2, b'v', b'2'];
const VALIDATOR_SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const ROTATED_SEEDS: [[u8; 32]; 4] = [[11; 32], [12; 32], [13; 32], [14; 32]];
const INVALID_WASM: &[u8] = &[0, 97, 115, 109, 1, 0, 0, 0, 1, 1, 0xff];

struct WrongAuthority;

impl RegistryReadAuthority for WrongAuthority {
    fn verify_registry_read(
        &self,
        _program: ProgramId,
        _latest: &layerx_programs::RegistryVersion,
    ) -> Result<([u8; 32], ReadFreshness), RegistryError> {
        Ok((
            [0x99; 32],
            ReadFreshness {
                observed_sequence: 1,
                observed_at: 1,
            },
        ))
    }
}

#[derive(Clone)]
struct CanonicalJournal {
    records: BTreeMap<[u8; 32], Vec<u8>>,
    head: ObservedHead,
}

impl CanonicalJournal {
    fn new(records: &[DeploymentRecord], head: ObservedHead) -> Self {
        Self {
            records: records
                .iter()
                .map(|record| (record.digest(), record.canonical_encoding()))
                .collect(),
            head,
        }
    }
}

impl DeploymentJournal for CanonicalJournal {
    fn canonical_record(&self, receipt_digest: [u8; 32]) -> Result<Vec<u8>, RegistryError> {
        self.records
            .get(&receipt_digest)
            .cloned()
            .ok_or(RegistryError::JournalUnavailable)
    }

    fn observed_head(&self) -> Result<ObservedHead, RegistryError> {
        Ok(self.head)
    }
}

#[test]
fn registry_resolves_historical_and_latest_code_only_from_protocol_evidence() {
    let program = fixture_program();
    let policy = UpgradePolicy::Authority(AUTHORITY);
    let deploy = deploy_fixture(PROTOCOL_WASM_V1, policy, 70, NOW - 800);
    let upgrade = upgrade_fixture(PROTOCOL_WASM_V1, PROTOCOL_WASM_V2, 71, NOW - 700);
    let verifier = verifier_for_fixture(&deploy, 1_000);
    let first = verifier
        .verify_deployment(&deploy.proof, NOW)
        .unwrap_or_else(|error| panic!("deploy evidence: {error}"));
    let second = verifier
        .verify_deployment(&upgrade.proof, NOW)
        .unwrap_or_else(|error| panic!("upgrade evidence: {error}"));
    let mut registry = Registry::new();
    registry
        .record_verified_deployment(&first)
        .unwrap_or_else(|error| panic!("verified deploy: {error}"));
    registry
        .record_verified_deployment(&second)
        .unwrap_or_else(|error| panic!("verified upgrade: {error}"));
    let historical = registry
        .resolve_deployment(first.clone())
        .unwrap_or_else(|error| panic!("historical deployment: {error}"));
    let latest = registry
        .resolve_deployment(second.clone())
        .unwrap_or_else(|error| panic!("latest deployment: {error}"));

    assert_eq!(historical.program(), program);
    assert_eq!(historical.version(), 1);
    assert_eq!(historical.code_hash(), fixture_code_hash(PROTOCOL_WASM_V1));
    assert_eq!(historical.receipt_digest(), first.receipt_digest());
    assert_eq!(historical.module(), PROTOCOL_WASM_V1);
    assert_eq!(historical.freshness().observed_sequence, 71);
    assert_eq!(historical.lifecycle(), ProgramLifecycle::Active);
    assert_eq!(latest.version(), 2);
    assert_eq!(latest.code_hash(), fixture_code_hash(PROTOCOL_WASM_V2));
    assert_eq!(latest.receipt_digest(), second.receipt_digest());
    assert_eq!(latest.module(), PROTOCOL_WASM_V2);
    let alternate = verifier
        .verify_deployment(
            &deploy_fixture(PROTOCOL_WASM_V1, policy, 73, NOW - 500).proof,
            NOW,
        )
        .unwrap_or_else(|error| panic!("alternate receipt evidence: {error}"));
    assert_eq!(
        registry.resolve_deployment(alternate),
        Err(RegistryError::UnverifiedRead)
    );

    let mut executable = VerifiedProgramCatalog::declared()
        .unwrap_or_else(|error| panic!("verified catalog: {error}"));
    executable
        .admit(historical)
        .unwrap_or_else(|error| panic!("historical admission: {error}"));
    assert_eq!(executable.version(program), Some(1));
    executable
        .admit(latest)
        .unwrap_or_else(|error| panic!("latest admission: {error}"));
    assert_eq!(executable.version(program), Some(2));
    assert_eq!(
        executable.code_hash(program),
        Some(fixture_code_hash(PROTOCOL_WASM_V2))
    );
    assert_eq!(executable.abi_version(program), Some(ABI_V1_VERSION));
    assert_eq!(
        executable.receipt_digest(program),
        Some(second.receipt_digest())
    );
    let current = verifier
        .verify_current_program(&upgrade.proof.state, program, NOW)
        .unwrap_or_else(|error| panic!("current program head: {error}"));
    let binding = ActivityBudgetBinding::new([0xa1; 32])
        .unwrap_or_else(|error| panic!("activity binding: {error}"));
    let _composition = executable
        .authorize_activity(vec![current], binding, NOW, CompositionRules::declared())
        .unwrap_or_else(|error| panic!("activity-scoped catalog: {error}"));
}

#[test]
fn forged_batch_journal_and_stale_evidence_never_create_deployment_authority() {
    let mut fixture = deploy_fixture(PROTOCOL_WASM_V1, UpgradePolicy::Immutable, 70, NOW);
    let verifier = verifier_for_fixture(&fixture, 100);
    let valid = verifier
        .verify_deployment(&fixture.proof, NOW)
        .expect("fully signed deployment");
    fixture.chain.commit.signatures.truncate(2);
    assert_eq!(fixture.chain.verifier.validators.total_power(), 4);
    assert_eq!(fixture.chain.commit.signatures.len(), 2);
    assert_eq!(
        fixture
            .chain
            .verifier
            .verify_head(&fixture.chain.header, &fixture.chain.commit),
        Err(ChainProofError::InsufficientPower)
    );
    let mut forged = fixture.proof.clone();
    forged.state.commitment = decode_commitment_proof(
        &fixture
            .chain
            .encode_key(&forged.state.commitment.key)
            .expect("underpowered commitment"),
    )
    .expect("decode underpowered commitment");
    assert_eq!(
        verifier.verify_deployment(&forged, NOW),
        Err(ProtocolEvidenceError::ChainProof)
    );
    let registry = Registry::new();
    assert_eq!(
        registry.latest_version(fixture_program()),
        Err(RegistryError::UnknownProgram)
    );
    assert_eq!(
        registry.resolve_deployment(valid.clone()),
        Err(RegistryError::UnknownProgram)
    );
    assert_eq!(
        verifier.verify_deployment(&fixture.proof, NOW + 100),
        Ok(valid.clone())
    );
    assert_eq!(
        verifier.verify_deployment(&fixture.proof, NOW + 101),
        Err(ProtocolEvidenceError::Stale)
    );
    let mut wrong_transaction = fixture.proof.clone();
    wrong_transaction.state.transaction[0] ^= 1;
    assert_eq!(
        verifier.verify_deployment(&wrong_transaction, NOW),
        Err(ProtocolEvidenceError::ChainProof)
    );
    let wrong_abi = wrong_abi_fixture(70, NOW);
    assert_eq!(
        verifier.verify_deployment(&wrong_abi.proof, NOW),
        Err(ProtocolEvidenceError::DeploymentMismatch)
    );
    let mut impossible_module = fixture.proof.clone();
    impossible_module.deployment.code = b"not-wasm".to_vec();
    assert_eq!(
        verifier.verify_deployment(&impossible_module, NOW),
        Err(ProtocolEvidenceError::DeploymentMismatch)
    );
    let foreign_network = ProtocolDeploymentVerifier::from_chain_trust(
        "foreign-chain",
        fixture.chain.verifier.validators.validators().to_vec(),
        100,
    )
    .expect("foreign chain trust");
    assert_eq!(
        foreign_network.verify_deployment(&fixture.proof, NOW),
        Err(ProtocolEvidenceError::ChainProof)
    );
    let (foreign_set, _) =
        chain_deployment_fixture(fixture_program(), PROTOCOL_WASM_V1, &ROTATED_SEEDS, None);
    let foreign_domain = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.chain.verifier.chain_id,
        foreign_set.verifier.validators.validators().to_vec(),
        100,
    )
    .expect("different validator trust");
    assert_eq!(
        foreign_domain.verify_deployment(&fixture.proof, NOW),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );
    assert_eq!(verifier.verify_deployment(&fixture.proof, NOW), Ok(valid));
    let mut unauthorized = fixture.proof.clone();
    let (mut rotated, _) =
        chain_deployment_fixture(fixture_program(), PROTOCOL_WASM_V1, &ROTATED_SEEDS, None);
    rotated.header.height = fixture.chain.header.height + 1;
    rotated.header.time = fixture.chain.header.time;
    sign_chain(&mut rotated, &ROTATED_SEEDS);
    unauthorized.state.commitment = decode_commitment_proof(
        &rotated
            .encode_key(&unauthorized.state.commitment.key)
            .expect("rotated commitment"),
    )
    .expect("decode rotated commitment");
    assert_eq!(
        verifier.verify_deployment(&unauthorized, NOW),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );
}

#[test]
fn historical_deployments_replay_across_an_explicit_sequencer_rotation() {
    let historical_program = fixture_program();
    let current_program = ProgramId::new([0x32; 32]).expect("second program");
    let (mut historical, mut historical_proof) = chain_deployment_fixture(
        historical_program,
        PROTOCOL_WASM_V1,
        &VALIDATOR_SEEDS,
        Some(support::fixture_interface(PROTOCOL_WASM_V1)),
    );
    let (mut current, mut current_proof) = chain_deployment_fixture(
        current_program,
        PROTOCOL_WASM_V2,
        &VALIDATOR_SEEDS,
        Some(support::fixture_interface(PROTOCOL_WASM_V2)),
    );
    let mut rotated = ROTATED_SEEDS
        .iter()
        .map(|seed| {
            let key = SigningKey::from_bytes(seed);
            TrustedValidator::new(key.verifying_key().to_bytes(), 1)
        })
        .collect::<Vec<_>>();
    rotated.sort_by_key(|validator| validator.address);
    let rotated = ValidatorSet::new(rotated).expect("rotated validators");
    assert!(rotated.validators().iter().all(|validator| !historical
        .verifier
        .validators
        .validators()
        .contains(validator)));
    historical.header.height = 71;
    historical.header.time.seconds = 1_700_000_130;
    historical.header.time.nanos = 0;
    sign_chain(&mut historical, &VALIDATOR_SEEDS);
    historical_proof.state.commitment = decode_commitment_proof(
        &historical
            .encode_key(&historical_proof.state.commitment.key)
            .expect("historical commitment"),
    )
    .expect("decode historical commitment");
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &historical.verifier.chain_id,
        historical.verifier.validators.validators().to_vec(),
        50,
    )
    .expect("original validator trust");
    let original_head = historical
        .verifier
        .verify_head(&historical.header, &historical.commit)
        .expect("original deployment head");
    let original_step = (
        historical.header.clone(),
        historical.commit.clone(),
        historical.verifier.validators.clone(),
    );
    let original = verifier
        .verify_deployment(&historical_proof, NOW - 20_000)
        .expect("deployment before rotation");

    let mut transition = historical.header.clone();
    transition.height += 1;
    transition.time.seconds += 10;
    transition.last_block_id.hash = historical.commit.block_id.hash.clone();
    transition.last_block_id.part_set_total = historical.commit.block_id.part_set_header.total;
    transition.last_block_id.part_set_hash =
        historical.commit.block_id.part_set_header.hash.clone();
    transition.next_validators_hash = rotated.hash().to_vec();
    historical.header = transition;
    sign_chain(&mut historical, &VALIDATOR_SEEDS);
    let transition_trust = historical
        .verifier
        .advance(
            &original_head,
            &historical.header,
            &historical.commit,
            historical.verifier.validators.clone(),
        )
        .expect("old validators authorize next validator set");
    let transition_head = transition_trust
        .verify_head(&historical.header, &historical.commit)
        .expect("authenticated rotation announcement");
    current.header.height = historical.header.height + 1;
    current.header.time.seconds = 1_700_000_150;
    current.header.time.nanos = 0;
    current.header.last_block_id.hash = historical.commit.block_id.hash.clone();
    current.header.last_block_id.part_set_total = historical.commit.block_id.part_set_header.total;
    current.header.last_block_id.part_set_hash =
        historical.commit.block_id.part_set_header.hash.clone();
    current.verifier.validators = rotated;
    current.header.validators_hash = current.verifier.validators.hash().to_vec();
    current.header.next_validators_hash = current.header.validators_hash.clone();
    current.header.proposer_address = current.verifier.validators.validators()[0].address.to_vec();
    sign_chain(&mut current, &ROTATED_SEEDS);
    current.verifier = transition_trust
        .advance(
            &transition_head,
            &current.header,
            &current.commit,
            current.verifier.validators.clone(),
        )
        .expect("authenticate rotated validators through the chain");
    let current_head = current
        .verifier
        .verify_head(&current.header, &current.commit)
        .expect("head after rotation");
    current_proof.state.commitment = decode_commitment_proof(
        &current
            .encode_key(&current_proof.state.commitment.key)
            .expect("current commitment"),
    )
    .expect("decode current commitment");
    let history = vec![
        original_step,
        (
            historical.header.clone(),
            historical.commit.clone(),
            transition_trust.validators.clone(),
        ),
        (
            current.header.clone(),
            current.commit.clone(),
            current.verifier.validators.clone(),
        ),
    ];
    assert_eq!(
        verifier.verify_deployment(&historical_proof, NOW),
        Err(ProtocolEvidenceError::Stale)
    );
    let first = verifier
        .verify_historical_deployment(&historical_proof, &current_head, &history, NOW)
        .expect("historical deployment verified across rotation");
    assert_eq!(first, original);
    let current_verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &current.verifier.chain_id,
        current.verifier.validators.validators().to_vec(),
        50,
    )
    .expect("verifier derived from authenticated rotated validators");
    let second = current_verifier
        .verify_historical_deployment(&current_proof, &current_head, &history[2..], NOW)
        .expect("deployment after rotation");
    assert_eq!(
        current_verifier.verify_deployment(&current_proof, NOW),
        Ok(second.clone())
    );
    assert_eq!(
        current_verifier.verify_historical_deployment(
            &historical_proof,
            &current_head,
            &history,
            NOW
        ),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );
    let missing_rotation = [history[0].clone(), history[2].clone()];
    assert_eq!(
        verifier.verify_historical_deployment(
            &historical_proof,
            &current_head,
            &missing_rotation,
            NOW
        ),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    let mut forged_history = history.clone();
    forged_history[1].1.signatures.truncate(2);
    assert_eq!(
        verifier.verify_historical_deployment(
            &historical_proof,
            &current_head,
            &forged_history,
            NOW
        ),
        Err(ProtocolEvidenceError::HistoricalTrustAnchor)
    );
    let mut wrong_anchor = history.clone();
    wrong_anchor[0].2 = current.verifier.validators.clone();
    assert_eq!(
        verifier.verify_historical_deployment(&historical_proof, &current_head, &wrong_anchor, NOW),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );

    let records = [second.record().clone(), first.record().clone()];
    let mut registry = Registry::new();
    registry
        .replay_journal(&records)
        .expect("replay both chain-verified deployments");
    assert_eq!(
        registry.program_ids(),
        vec![historical_program, current_program]
    );
    for (evidence, wasm, height) in [
        (&first, PROTOCOL_WASM_V1, 71),
        (&second, PROTOCOL_WASM_V2, 73),
    ] {
        assert_eq!(evidence.version(), 1);
        assert_eq!(evidence.module(), wasm);
        assert_eq!(evidence.code_hash(), fixture_code_hash(wasm));
        assert_eq!(evidence.freshness().observed_sequence, height);
        assert_eq!(registry.latest_version(evidence.program()), Ok(1));
        let entry = registry
            .entry_for_wind_down(evidence.program())
            .expect("replayed entry");
        assert_eq!(entry.versions.len(), 1);
        assert_eq!(entry.versions[0].code_hash, evidence.code_hash());
        assert_eq!(entry.versions[0].abi_version, evidence.abi_version());
        assert_eq!(
            entry.versions[0].deployment_receipt_digest,
            evidence.record().digest()
        );
    }
    assert_eq!(first.state_root(), original_head.app_hash());
    assert_eq!(second.state_root(), current_head.app_hash());
    assert_ne!(first.state_root(), second.state_root());
    let authority = JournalReadAuthority::new(
        CanonicalJournal::new(
            &records,
            ObservedHead {
                sequence: 73,
                observed_at: NOW,
            },
        ),
        NOW,
        50,
    )
    .expect("canonical deployment journal authority");
    for evidence in [&first, &second] {
        let read = registry
            .read(evidence.program(), &authority)
            .expect("authenticated journal read");
        assert_eq!(read.entry.versions[0].code_hash, evidence.code_hash());
        assert_eq!(
            read.entry.versions[0].deployment_receipt_digest,
            evidence.record().digest()
        );
    }
}

#[test]
fn mismatched_receipt_invalid_module_stale_head_and_deprecation_are_fail_closed() {
    let (invalid_chain, invalid_proof) =
        chain_deployment_fixture(fixture_program(), INVALID_WASM, &VALIDATOR_SEEDS, None);
    let fixture = deploy_fixture(
        PROTOCOL_WASM_V1,
        UpgradePolicy::Authority(AUTHORITY),
        70,
        NOW - 800,
    );
    let other = deploy_fixture(
        PROTOCOL_WASM_V1,
        UpgradePolicy::Authority(AUTHORITY),
        71,
        NOW - 700,
    );
    let verifier = verifier_for_fixture(&fixture, 1_000);
    let mut swapped_receipt = fixture.proof.clone();
    swapped_receipt.state.transaction = other.proof.state.transaction.clone();
    swapped_receipt.state.transaction[0] ^= 1;
    let signed_header = layerx_programs::chain_proof::header::ChainHeader::decode(
        &swapped_receipt.state.commitment.signed_header,
    )
    .expect("decode signed chain header");
    let signed_head = fixture
        .chain
        .verifier
        .verify_head(&signed_header, &fixture.chain.commit)
        .expect("signed chain head");
    assert!(layerx_programs::verify_tx(
        &signed_head,
        &swapped_receipt.state.transaction,
        &swapped_receipt.state.transaction_proof,
    )
    .is_err());
    let swapped_head =
        verifier.verify_current_program(&swapped_receipt.state, fixture_program(), NOW);
    assert_eq!(swapped_head, Err(ProtocolEvidenceError::ChainProof));
    let swapped_result = verifier.verify_deployment(&swapped_receipt, NOW);
    assert_eq!(swapped_result, Err(ProtocolEvidenceError::ChainProof));
    let invalid_verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &invalid_chain.verifier.chain_id,
        invalid_chain.verifier.validators.validators().to_vec(),
        1_000,
    )
    .expect("invalid module chain verifier");
    let malformed_evidence = invalid_verifier
        .verify_deployment(&invalid_proof, NOW)
        .unwrap_or_else(|error| panic!("malformed module protocol evidence: {error}"));
    let mut executable = VerifiedProgramCatalog::declared()
        .unwrap_or_else(|error| panic!("verified catalog: {error}"));
    assert!(matches!(
        executable.admit(malformed_evidence),
        Err(ExecutableAdmissionError::Validation(_))
    ));
    assert!(executable.is_empty());

    let valid = deploy_fixture(
        PROTOCOL_WASM_V1,
        UpgradePolicy::Authority(AUTHORITY),
        72,
        NOW - 600,
    );
    let evidence = verifier
        .verify_deployment(&valid.proof, NOW)
        .unwrap_or_else(|error| panic!("valid deployment: {error}"));
    let active = verifier
        .verify_current_program(&valid.proof.state, fixture_program(), NOW)
        .unwrap_or_else(|error| panic!("active head: {error}"));
    let binding = ActivityBudgetBinding::new([0xa2; 32])
        .unwrap_or_else(|error| panic!("activity binding: {error}"));
    let mut stale_catalog =
        VerifiedProgramCatalog::declared().unwrap_or_else(|error| panic!("stale catalog: {error}"));
    stale_catalog
        .admit(evidence.clone())
        .unwrap_or_else(|error| panic!("catalog admission: {error}"));
    assert!(matches!(
        stale_catalog.authorize_activity(
            vec![active.clone()],
            binding,
            active.valid_until_ms() + 1,
            CompositionRules::declared(),
        ),
        Err(ExecutableAdmissionError::EvidenceExpired { .. })
    ));

    let deprecated = verifier
        .verify_current_program(&deprecated_state(&valid, NOW - 500), fixture_program(), NOW)
        .unwrap_or_else(|error| panic!("deprecated head: {error}"));
    let mut deprecated_catalog = VerifiedProgramCatalog::declared()
        .unwrap_or_else(|error| panic!("deprecated catalog: {error}"));
    deprecated_catalog
        .admit(evidence)
        .unwrap_or_else(|error| panic!("catalog admission: {error}"));
    assert!(matches!(
        deprecated_catalog.authorize_activity(
            vec![deprecated],
            binding,
            NOW,
            CompositionRules::declared(),
        ),
        Err(ExecutableAdmissionError::InactiveLifecycle {
            lifecycle: ProgramLifecycle::Deprecated,
        })
    ));
}

#[test]
fn direct_deployment_insertion_refuses_the_reserved_upgrade_authority() {
    let program = program();
    let code_hash = code_hash(WASM_V1);
    let version = ProgramVersion {
        code_hash,
        wasm: WASM_V1.to_vec(),
        abi_version: ABI_VERSION,
    };
    let mut lifecycle =
        Lifecycle::declared().unwrap_or_else(|error| panic!("lifecycle construction: {error}"));
    let receipt = lifecycle
        .deploy(Deploy {
            program,
            code_hash,
            wasm: WASM_V1.to_vec(),
            abi_version: ABI_VERSION,
            upgrade_policy: UpgradePolicy::Immutable,
        })
        .unwrap_or_else(|error| panic!("deployment: {error}"));
    let canonical = DeploymentRecord::from_deployment(
        &receipt,
        &version,
        UpgradePolicy::Immutable,
        70,
        1_700_000_070,
    )
    .unwrap_or_else(|error| panic!("canonical deployment: {error}"));
    let mut registry = Registry::new();
    assert_eq!(
        registry.record_deployment(
            &receipt,
            &version,
            UpgradePolicy::Authority([0; 32]),
            canonical.digest(),
        ),
        Err(RegistryError::InvalidUpgradeAuthority)
    );
    assert_eq!(
        registry.latest_version(program),
        Err(RegistryError::UnknownProgram)
    );
}

#[test]
fn registry_replay_refuses_downgrade_without_rewriting_historical_version() {
    let program = program();
    let first = record_with_abi(
        program,
        1,
        None,
        WASM_V1,
        UpgradePolicy::Authority([0x51; 32]),
        80,
        ABI_V2_VERSION,
    );
    let downgrade = record_with_abi(
        program,
        2,
        Some(first.new_code_hash),
        WASM_V2,
        UpgradePolicy::Authority([0x51; 32]),
        81,
        ABI_V1_VERSION,
    );
    let mut registry = Registry::new();
    assert_eq!(
        registry.replay_journal(&[first.clone(), downgrade]),
        Err(RegistryError::AbiVersion(
            layerx_programs_runtime::AbiVersionRefusal::Downgrade {
                current: ABI_V2_VERSION,
                requested: ABI_V1_VERSION,
            },
        )),
    );
    assert_eq!(registry.latest_version(program), Ok(1));
    assert_eq!(
        registry
            .entry_for_wind_down(program)
            .map(|entry| entry.versions[0].abi_version),
        Ok(ABI_V2_VERSION),
    );
}

#[test]
fn reproducible_build_verification_records_success_and_visible_mismatch() {
    let program = program();
    let first = record(program, 1, None, WASM_V1, UpgradePolicy::Immutable, 70);
    let mut registry = Registry::new();
    registry
        .replay_journal(std::slice::from_ref(&first))
        .unwrap_or_else(|error| panic!("deployment: {error}"));
    let verified = ReproducibleBuild::from_output(&source(), environment(), WASM_V1)
        .unwrap_or_else(|error| panic!("build evidence: {error}"));
    assert_eq!(verified.artifact_digest, code_hash(WASM_V1));
    assert!(matches!(
        registry.verify_source(program, 1, &verified),
        Ok(SourceStatus::Verified { .. })
    ));

    let altered = ReproducibleBuild::from_output(&source(), environment(), WASM_V2)
        .unwrap_or_else(|error| panic!("altered evidence: {error}"));
    let mismatch = registry
        .verify_source(program, 1, &altered)
        .unwrap_or_else(|error| panic!("mismatch status: {error}"));
    assert_eq!(
        mismatch,
        SourceStatus::Mismatch {
            expected: code_hash(WASM_V1),
            reproduced: altered.artifact_digest,
        }
    );
    let journal = CanonicalJournal::new(
        &[first],
        ObservedHead {
            sequence: 77,
            observed_at: 1_700_000_100,
        },
    );
    let authority = JournalReadAuthority::new(journal, 1_700_000_150, 100)
        .unwrap_or_else(|error| panic!("journal authority: {error}"));
    let read = registry
        .read(program, &authority)
        .unwrap_or_else(|error| panic!("registry read: {error}"));
    assert_eq!(read.entry.versions[0].source, mismatch);
}

#[test]
fn noncontiguous_or_unverified_registry_history_is_refused() {
    let program = program();
    let mut registry = Registry::new();
    let reserved_authority = record(
        program,
        1,
        None,
        WASM_V1,
        UpgradePolicy::Authority([0; 32]),
        70,
    );
    assert_eq!(
        registry.replay_journal(&[reserved_authority]),
        Err(RegistryError::InvalidUpgradeAuthority)
    );
    let second = record(
        program,
        2,
        Some(code_hash(WASM_V1)),
        WASM_V2,
        UpgradePolicy::Immutable,
        71,
    );
    assert_eq!(
        registry.replay_journal(&[second]),
        Err(RegistryError::VersionHistoryMismatch)
    );

    let first = record(program, 1, None, WASM_V1, UpgradePolicy::Immutable, 70);
    registry
        .replay_journal(&[first])
        .unwrap_or_else(|error| panic!("deployment: {error}"));
    assert_eq!(
        registry.read(program, &WrongAuthority),
        Err(RegistryError::UnverifiedRead)
    );
}

fn program() -> ProgramId {
    ProgramId::new(PROGRAM).unwrap_or_else(|error| panic!("program: {error}"))
}

fn code_hash(wasm: &[u8]) -> [u8; 32] {
    hash_bytes(HashAlgorithm::Sha256, wasm)
        .unwrap_or_else(|error| panic!("program code hash: {error}"))
}

fn record(
    program: ProgramId,
    version: u32,
    old_code_hash: Option<[u8; 32]>,
    wasm: &[u8],
    upgrade_policy: UpgradePolicy,
    sequence: u64,
) -> DeploymentRecord {
    record_with_abi(
        program,
        version,
        old_code_hash,
        wasm,
        upgrade_policy,
        sequence,
        ABI_VERSION,
    )
}

fn record_with_abi(
    program: ProgramId,
    version: u32,
    old_code_hash: Option<[u8; 32]>,
    wasm: &[u8],
    upgrade_policy: UpgradePolicy,
    sequence: u64,
    abi_version: u16,
) -> DeploymentRecord {
    let new_code_hash = code_hash(wasm);
    DeploymentRecord {
        program,
        version,
        abi_version,
        upgrade_policy,
        old_code_hash,
        new_code_hash,
        sequence,
        observed_at: 1_700_000_000 + sequence,
        module: wasm.to_vec(),
        migration: None,
    }
}

fn source() -> PublishedSource {
    PublishedSource {
        uri: "https://source.example/program.tar".to_owned(),
        canonical_archive: b"canonical-source-archive".to_vec(),
    }
}

fn environment() -> BuildEnvironment {
    BuildEnvironment {
        builder_image_digest: [0x71; 32],
        toolchain_digest: [0x72; 32],
        dependency_lock_digest: [0x73; 32],
        source_date_epoch: 1_700_000_000,
        command: vec![
            "cargo".to_owned(),
            "build".to_owned(),
            "--locked".to_owned(),
        ],
    }
}

#[test]
fn registry_replay_applies_every_supported_abi_transition_without_rewriting_history() {
    use layerx_programs_runtime::{AbiVersionRefusal, ABI_V3_VERSION, ABI_V4_VERSION};
    for current in [
        ABI_V1_VERSION,
        ABI_V2_VERSION,
        ABI_V3_VERSION,
        ABI_V4_VERSION,
    ] {
        for requested in [
            ABI_V1_VERSION,
            ABI_V2_VERSION,
            ABI_V3_VERSION,
            ABI_V4_VERSION,
        ] {
            let id = program();
            let policy = UpgradePolicy::Authority([0x51; 32]);
            let first = record_with_abi(id, 1, None, WASM_V1, policy, 80, current);
            let next = record_with_abi(
                id,
                2,
                Some(first.new_code_hash),
                WASM_V2,
                policy,
                81,
                requested,
            );
            let mut registry = Registry::new();
            let result = registry.replay_journal(&[first.clone(), next]);
            if requested < current {
                assert_eq!(
                    result,
                    Err(RegistryError::AbiVersion(AbiVersionRefusal::Downgrade {
                        current,
                        requested,
                    }))
                );
                assert_eq!(registry.latest_version(id), Ok(1));
            } else {
                assert_eq!(result, Ok(()));
                assert_eq!(registry.latest_version(id), Ok(2));
            }
            let entry = registry
                .entry_for_wind_down(id)
                .unwrap_or_else(|error| panic!("retained history: {error}"));
            assert_eq!(entry.versions[0].abi_version, current);
            assert_eq!(entry.versions[0].code_hash, first.new_code_hash);
            assert_eq!(
                entry.versions.len(),
                if requested < current { 1 } else { 2 }
            );
            if requested >= current {
                assert_eq!(entry.versions[1].abi_version, requested);
            }
        }
    }
}

fn sign_chain(fixture: &mut ChainFixture, seeds: &[[u8; 32]]) {
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture.header.hash().expect("chain header hash").to_vec();
    let timestamp = Timestamp {
        seconds: fixture.header.time.seconds,
        nanos: fixture.header.time.nanos,
    };
    let bytes = vote_sign_bytes(
        &fixture.header.chain_id,
        fixture.commit.height,
        fixture.commit.round,
        Some(&fixture.commit.block_id),
        timestamp,
    )
    .expect("canonical vote bytes");
    let mut keys = seeds.iter().map(SigningKey::from_bytes).collect::<Vec<_>>();
    keys.sort_by_key(|key| TrustedValidator::new(key.verifying_key().to_bytes(), 1).address);
    fixture.commit.signatures = keys
        .iter()
        .map(|key| CommitSig {
            flag: BlockIdFlag::Commit,
            validator_address: TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                .address
                .to_vec(),
            timestamp,
            signature: key.sign(&bytes).to_bytes().to_vec(),
        })
        .collect();
}

fn chain_deployment_fixture(
    program: ProgramId,
    wasm: &[u8],
    seeds: &[[u8; 32]],
    interface: Option<ProgramInterface>,
) -> (ChainFixture, DeploymentProof) {
    let mut program_key = b"program\0".to_vec();
    program_key.extend_from_slice(&program.bytes());
    let program_key = programs_state_key(&program_key);
    let mut status_key = b"wind-down\0s".to_vec();
    status_key.extend_from_slice(&program.bytes());
    let status_key = programs_state_key(&status_key);
    let mut record = vec![0; 33];
    record.extend_from_slice(&fixture_code_hash(wasm));
    record.extend_from_slice(&ABI_V1_VERSION.to_be_bytes());
    record.extend_from_slice(&1_u32.to_be_bytes());
    let mut fixture = ChainFixture::new(
        "paxeer-registry-replay",
        seeds,
        vec![(program_key.clone(), record.clone())],
    )
    .expect("real chain state fixture");
    let interface = interface.map(|value| value.canonical_encoding().to_vec());
    let mut transaction = program.bytes().to_vec();
    transaction.extend_from_slice(&ABI_V1_VERSION.to_be_bytes());
    transaction.extend_from_slice(&[0; 34]);
    transaction.extend_from_slice(&fixture_code_hash(wasm));
    transaction.extend_from_slice(
        &u32::try_from(wasm.len())
            .expect("module length")
            .to_be_bytes(),
    );
    let interface_length = interface.as_ref().map_or(0, Vec::len);
    transaction.extend_from_slice(
        &u32::try_from(interface_length)
            .expect("interface length")
            .to_be_bytes(),
    );
    if let Some(encoding) = &interface {
        transaction.extend_from_slice(encoding);
    }
    transaction.extend_from_slice(wasm);
    let transaction_root = hash_from_byte_slices(&[&fixture_code_hash(&transaction)]);
    fixture.header.time.seconds = 1_700_000_150;
    fixture.header.time.nanos = 0;
    fixture.header.data_hash = transaction_root.to_vec();
    sign_chain(&mut fixture, seeds);
    let state = ChainProgramStateProof {
        program,
        ops: fixture.prove(&program_key).expect("program membership"),
    };
    let status_ops = fixture
        .prove_absent(&status_key)
        .expect("active lifecycle absence");
    let proof = DeploymentProof {
        state: ProgramStateProof {
            commitment: decode_commitment_proof(
                &fixture.encode_key(&program_key).expect("chain commitment"),
            )
            .expect("decode chain commitment"),
            state: state.clone(),
            lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                program,
                record: Some(record),
                record_ops: state.ops.clone(),
                status: None,
                status_ops: status_ops.clone(),
            }),
            wind_down: ChainWindDownProof {
                program,
                value: None,
                ops: status_ops,
            },
            transaction,
            transaction_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: transaction_root,
                aunts: Vec::new(),
            },
        },
        deployment: ChainDeploymentProof {
            program_state: state,
            code: wasm.to_vec(),
            interface,
        },
    };
    (fixture, proof)
}
