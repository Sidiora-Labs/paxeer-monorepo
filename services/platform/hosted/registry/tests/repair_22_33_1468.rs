//! Registry integration cases preserve source bindings and deployment responses across restart.

#[allow(dead_code)]
#[path = "support/repair_22_33_1468_chain.rs"]
mod chain;

use chain::ChainFixture;
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::{
    chain_evidence::{
        deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, programs_state_key, wind_down::ChainWindDownProof,
    },
    chain_proof::{
        decode::decode_commitment_proof,
        merkle::{hash_from_byte_slices, SimpleProof},
        results::TxResult,
        validators::TrustedValidator,
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
    },
    interface_state_key, interface_state_value, DeploymentExecutionProof, DeploymentProof,
    DeploymentRecord, ProgramId, ProgramInterface, ProgramLifecycleProof, ProgramStateProof,
    ProtocolDeploymentVerifier, Registry, UpgradePolicy,
};
use layerx_programs_runtime::{Deploy, Lifecycle, ProgramVersion, Upgrade};
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    fs,
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const CHAIN: &str = "repair-22-31-257";
fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
fn program() -> ProgramId {
    ProgramId::new([0x31; 32]).unwrap_or_else(|error| panic!("{error:?}"))
}
fn policy() -> UpgradePolicy {
    UpgradePolicy::Authority([0x51; 32])
}
fn transcript() -> BTreeMap<String, Vec<u8>> {
    let text = include_str!(
        "../../../../../programs/crates/layerx-programs-registry/src/repair_22_31_256.hex"
    );
    assert_eq!(
        layerx_programs::hex::encode(&hash(text.as_bytes())),
        "5798c9304ce5dd774f153da4f4f3208f1205f4cbfd1854732a49636dc9d76c6e"
    );
    text.lines()
        .map(|line| {
            let (key, value) = line
                .split_once('=')
                .unwrap_or_else(|| panic!("expected a value"));
            (
                key.into(),
                layerx_programs::hex::decode(value).unwrap_or_else(|error| panic!("{error:?}")),
            )
        })
        .collect()
}
fn descriptor(version: usize) -> ProgramInterface {
    ProgramInterface::decode(&transcript()[&format!("interface.{version}")])
        .unwrap_or_else(|error| panic!("{error:?}"))
}
fn code(version: usize) -> Vec<u8> {
    transcript()[&format!("wasm.{version}")].clone()
}
fn program_key() -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program().bytes());
    programs_state_key(&key)
}
fn interface_key() -> Vec<u8> {
    programs_state_key(&interface_state_key(program()))
}
fn records(abi: u16) -> [DeploymentRecord; 2] {
    let mut runtime = Lifecycle::declared().unwrap_or_else(|error| panic!("{error:?}"));
    let first = runtime
        .deploy(Deploy {
            program: program(),
            code_hash: hash(&code(1)),
            wasm: code(1),
            abi_version: abi,
            upgrade_policy: policy(),
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
    let second = runtime
        .upgrade(Upgrade {
            program: program(),
            authority: [0x51; 32],
            code_hash: hash(&code(2)),
            wasm: code(2),
            abi_version: abi,
            migration: None,
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
    [first, second]
        .iter()
        .enumerate()
        .map(|(i, receipt)| {
            let wasm = code(i + 1);
            DeploymentRecord::from_deployment(
                receipt,
                &ProgramVersion {
                    code_hash: hash(&wasm),
                    wasm,
                    abi_version: abi,
                },
                policy(),
                39 + i as u64,
                1_700_000_001_123 + i as u64 * 1000,
            )
            .unwrap_or_else(|error| panic!("{error:?}"))
        })
        .collect::<Vec<_>>()
        .try_into()
        .unwrap_or_else(|error| panic!("{error:?}"))
}
fn program_value(version: usize, abi: u16) -> Vec<u8> {
    record_value(&records(abi)[version - 1])
}
fn record_value(record: &DeploymentRecord) -> Vec<u8> {
    let mut value = vec![1];
    value.extend_from_slice(&[0x51; 32]);
    value.extend_from_slice(&record.new_code_hash);
    value.extend_from_slice(&record.abi_version.to_be_bytes());
    value.extend_from_slice(&record.version.to_be_bytes());
    value
}
fn state_fixture(
    version: usize,
    abi: u16,
    interface: Option<Vec<u8>>,
) -> (ChainFixture, ChainDeploymentProof) {
    let mut rows = vec![(program_key(), program_value(version, abi))];
    if let Some(value) = interface {
        rows.push((interface_key(), value));
    }
    let chain = ChainFixture::new(CHAIN, &SEEDS, rows).unwrap_or_else(|error| panic!("{error:?}"));
    let witness = decode_commitment_proof(
        &chain
            .encode_key(&interface_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let deployment = ChainDeploymentProof::from_state_proofs(
        ChainProgramStateProof {
            program: program(),
            ops: chain
                .prove(&program_key())
                .unwrap_or_else(|error| panic!("{error:?}")),
        },
        code(version),
        &witness,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    (chain, deployment)
}
fn value(version: usize) -> Vec<u8> {
    interface_state_value(
        program(),
        u32::try_from(version).unwrap_or_else(|error| panic!("{error:?}")),
        &descriptor(version),
    )
    .unwrap_or_else(|error| panic!("{error:?}"))
}
fn sign(fixture: &mut ChainFixture) {
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture
        .header
        .hash()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .to_vec();
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
    .unwrap_or_else(|error| panic!("{error:?}"));
    let mut keys = SEEDS.iter().map(SigningKey::from_bytes).collect::<Vec<_>>();
    keys.sort_by_key(|key| TrustedValidator::new(key.verifying_key().to_bytes(), 1).address);
    fixture.commit.signatures = keys
        .into_iter()
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

struct Admission {
    chain: ChainFixture,
    proof: DeploymentProof,
    verifier: ProtocolDeploymentVerifier,
    now: u64,
}
fn admission(version: usize) -> Admission {
    admission_abi(version, 1)
}
fn prior_chain(version: usize, transaction: &[u8]) -> (ChainFixture, [u8; 32]) {
    let mut prior = if version == 1 {
        let mut prior = ChainFixture::new(CHAIN, &SEEDS, vec![(b"unrelated".to_vec(), vec![1])])
            .unwrap_or_else(|error| panic!("{error:?}"));
        prior.header.height = 38;
        prior
    } else {
        admission(1).chain
    };
    let tx_root = hash_from_byte_slices(&[&hash(transaction)]);
    prior.header.data_hash = tx_root.to_vec();
    sign(&mut prior);
    (prior, tx_root)
}

fn assemble_admission(chain: ChainFixture, proof: DeploymentProof) -> Admission {
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN,
        chain.verifier.validators.validators().to_vec(),
        5000,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let now = u64::try_from(chain.header.time.seconds).unwrap_or_else(|error| panic!("{error:?}"))
        * 1000
        + u64::try_from(chain.header.time.nanos).unwrap_or_else(|error| panic!("{error:?}"))
            / 1_000_000;
    Admission {
        chain,
        proof,
        verifier,
        now,
    }
}

fn current_chain(
    version: usize,
    abi: u16,
    producer: &BTreeMap<String, Vec<u8>>,
    prior: &ChainFixture,
) -> (ChainFixture, [u8; 32]) {
    let (mut chain, _) = state_fixture(version, abi, (abi == 1).then(|| value(version)));
    chain.header.height = prior.header.height + 1;
    chain.header.time.seconds = prior.header.time.seconds + 1;
    chain
        .header
        .last_block_id
        .hash
        .clone_from(&prior.commit.block_id.hash);
    chain.header.last_block_id.part_set_total = prior.commit.block_id.part_set_header.total;
    chain
        .header
        .last_block_id
        .part_set_hash
        .clone_from(&prior.commit.block_id.part_set_header.hash);
    let result_root = hash_from_byte_slices(&[&producer["success.leaf"]]);
    assert_eq!(result_root.as_slice(), producer["success.root"]);
    chain.header.last_results_hash = result_root.to_vec();
    chain.header.data_hash = hash_from_byte_slices(&[&hash(
        &producer[if version == 1 && abi == 1 {
            "upgrade"
        } else {
            "unrelated"
        }],
    )])
    .to_vec();
    sign(&mut chain);
    (chain, result_root)
}

fn wind_down_ops(chain: &ChainFixture) -> Vec<layerx_programs::chain_proof::decode::ProofOp> {
    let mut status_key = b"wind-down\0s".to_vec();
    status_key.extend_from_slice(&program().bytes());
    let status_ops = chain
        .prove_absent(&programs_state_key(&status_key))
        .unwrap_or_else(|error| panic!("{error:?}"));
    status_ops
}

fn admission_abi(version: usize, abi: u16) -> Admission {
    let producer = transcript();
    let abi_five: Value = serde_json::from_str(include_str!("../src/repair_22_31_206_abi5.json"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let five_tx = hex::decode(
        abi_five["transaction_hex"]
            .as_str()
            .unwrap_or_else(|| panic!("expected a value")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let transaction = if abi == 5 {
        &five_tx
    } else {
        &producer[if version == 1 { "deploy" } else { "upgrade" }]
    };
    let (prior, tx_root) = prior_chain(version, transaction);
    let (chain, result_root) = current_chain(version, abi, &producer, &prior);
    let commitment = decode_commitment_proof(
        &chain
            .encode_key(&program_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let interface_state = decode_commitment_proof(
        &chain
            .encode_key(&interface_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let state = ChainProgramStateProof {
        program: program(),
        ops: chain
            .prove(&program_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    };
    let status_ops = wind_down_ops(&chain);
    let deployment =
        ChainDeploymentProof::from_state_proofs(state.clone(), code(version), &interface_state)
            .unwrap_or_else(|error| panic!("{error:?}"));
    let mut proof = DeploymentProof {
        state: ProgramStateProof {
            commitment,
            state: state.clone(),
            lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                program: program(),
                record: Some(program_value(version, abi)),
                record_ops: state.ops,
                status: None,
                status_ops: status_ops.clone(),
            }),
            wind_down: ChainWindDownProof {
                program: program(),
                value: None,
                ops: status_ops,
            },
            transaction: vec![],
            transaction_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: tx_root,
                aunts: vec![],
            },
        },
        deployment,
    };
    proof
        .bind_execution(&DeploymentExecutionProof {
            transaction: transaction.clone(),
            previous: decode_commitment_proof(
                &prior
                    .encode_key(&program_key())
                    .unwrap_or_else(|error| panic!("{error:?}")),
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
            result: TxResult {
                code: 0,
                data: producer["success"].clone(),
                gas_wanted: 0,
                gas_used: 0,
            },
            result_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: result_root,
                aunts: vec![],
            },
            interface_ops: if abi == 1 {
                chain
                    .prove(&interface_key())
                    .unwrap_or_else(|error| panic!("{error:?}"))
            } else {
                chain
                    .prove_absent(&interface_key())
                    .unwrap_or_else(|error| panic!("{error:?}"))
            },
            migration: None,
            migration_ops: vec![],
            migration_previous_ops: vec![],
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
    assemble_admission(chain, proof)
}

use layerx_platform_registry::{
    Admission as RequestAdmission, MirrorRefusal, MirroredSource, Registrar, Response,
    SourceMirror, VerificationJournal, VerificationState, VerifiedSource, VerifiedSourceDeployment,
    VerifiedSourceStore,
};
use layerx_programs::{
    hex, BuildAttempt, BuildEnvironment, BuildPlan, BuildRefusal, BuildRunner, ReproducibleBuild,
    SourceArchive, SourceFile, SourceStatus, SourceVerifier,
};
use serde_json::{json, Value};
use std::time::{Duration, Instant};

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let root = std::env::temp_dir().join(format!(
            "registry-repair-22-33-1468-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap_or_else(|error| panic!("{error:?}"));
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap_or_else(|error| panic!("{error:?}"));
    }
}

fn source_fixture() -> (SourceArchive, BuildPlan) {
    let archive = SourceArchive::new(vec![
        SourceFile {
            path: "module.wasm".into(),
            content: code(2),
            executable: false,
        },
        SourceFile {
            path: "toolchain".into(),
            content: b"recorded-production-module".to_vec(),
            executable: false,
        },
        SourceFile {
            path: "dependencies.lock".into(),
            content: b"no-dependencies".to_vec(),
            executable: false,
        },
    ])
    .unwrap_or_else(|error| panic!("{error:?}"));
    let plan = BuildPlan {
        environment: BuildEnvironment {
            builder_image_digest: [1; 32],
            toolchain_digest: hash(b"recorded-production-module"),
            dependency_lock_digest: hash(b"no-dependencies"),
            source_date_epoch: 1,
            command: vec!["cp".into(), "module.wasm".into(), "artifact.wasm".into()],
        },
        artifact_path: "artifact.wasm".into(),
        toolchain_manifest: "toolchain".into(),
        dependency_lock: "dependencies.lock".into(),
    };
    (archive, plan)
}
const URI: &str = "ipfs://recorded-program-module";

#[test]
fn mutable_source_mirror_conflicts_preserve_reopened_binding() {
    let root = Directory::new();
    let (archive, plan) = source_fixture();
    let mirror = SourceMirror::open(root.0.clone()).unwrap_or_else(|error| panic!("{error:?}"));
    let digest = mirror
        .publish(URI, &plan, &archive)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let original = mirror
        .fetch(URI, digest)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let admitted = admission(2);
    let deployment = admitted
        .verifier
        .verify_deployment(&admitted.proof, admitted.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let verifier = SourceVerifier::new(
        BuildTranscript {
            attempts: std::cell::Cell::new(0),
        },
        2,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let build = verifier
        .reproduce_deployment(&original.source, &original.plan, &deployment)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let record = source_record(&deployment, &build, plan.clone());
    let store = VerifiedSourceStore::open(root.0.join("verified"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    store
        .record(&record)
        .unwrap_or_else(|error| panic!("{error:?}"));
    drop(store);
    let mut other = plan.clone();
    other.environment.source_date_epoch += 1;
    assert!(mirror.publish(URI, &other, &archive).is_err());
    assert!(mirror
        .publish("ipfs://different-location", &plan, &archive)
        .is_err());
    assert_eq!(
        mirror
            .publish(URI, &plan, &archive)
            .unwrap_or_else(|error| panic!("{error:?}")),
        digest
    );
    drop(mirror);
    let reopened = SourceMirror::open(root.0.clone()).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        reopened
            .fetch(URI, digest)
            .unwrap_or_else(|error| panic!("{error:?}")),
        original
    );
    let records = VerifiedSourceStore::open(root.0.join("verified"))
        .unwrap_or_else(|error| panic!("{error:?}"))
        .records()
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(records, vec![record.clone()]);
    let recovered = reopened
        .fetch(&records[0].source_uri, records[0].source_digest)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(recovered.plan, records[0].plan);
    let prior = admission(1);
    let predecessor = prior
        .verifier
        .verify_deployment(&prior.proof, prior.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut registry = Registry::new();
    registry
        .record_verified_deployment(&predecessor)
        .unwrap_or_else(|error| panic!("{error:?}"));
    registry
        .record_verified_upgrade(&predecessor, &deployment)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let restored = ReproducibleBuild::from_record(
        record.source_uri,
        record.source_digest,
        recovered.plan.environment,
        record.artifact_digest,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(matches!(
        registry
            .verify_source(program(), 2, &restored)
            .unwrap_or_else(|error| panic!("{error:?}")),
        SourceStatus::Verified { .. }
    ));
    assert!(matches!(
        reopened.fetch("ipfs://different-location", digest),
        Err(MirrorRefusal::UriMismatch { .. })
    ));
}

fn legacy(root: &Directory, archive: &SourceArchive, plan: &BuildPlan) -> [u8; 32] {
    let bytes = archive.encode();
    let digest = hash(&bytes);
    let key = hex::encode(&digest);
    fs::write(root.0.join(format!("{key}.archive")), bytes)
        .unwrap_or_else(|error| panic!("{error:?}"));
    fs::write(root.0.join(format!("{key}.plan")), plan.encode())
        .unwrap_or_else(|error| panic!("{error:?}"));
    fs::write(root.0.join(format!("{key}.uri")), URI).unwrap_or_else(|error| panic!("{error:?}"));
    digest
}

#[test]
fn mutable_source_mirror_recovers_every_commit_boundary() {
    let (archive, plan) = source_fixture();
    for step in [
        "create_temporary",
        "write_binding",
        "sync_temporary",
        "commit",
        "sync_directory",
    ] {
        for migrating in [false, true] {
            let root = Directory::new();
            let digest = if migrating {
                legacy(&root, &archive, &plan)
            } else {
                hash(&archive.encode())
            };
            let interrupted = SourceMirror::open(root.0.clone())
                .unwrap_or_else(|error| panic!("{error:?}"))
                .interrupt_before(step);
            let error = interrupted
                .publish(URI, &plan, &archive)
                .err()
                .unwrap_or_else(|| panic!("expected an error"));
            assert!(error.contains(step), "{error}");
            drop(interrupted);
            let reopened =
                SourceMirror::open(root.0.clone()).unwrap_or_else(|error| panic!("{error:?}"));
            if migrating || step == "sync_directory" {
                let restored = reopened
                    .fetch(URI, digest)
                    .unwrap_or_else(|error| panic!("{error:?}"));
                assert_eq!(restored.plan, plan);
                assert_eq!(restored.source.canonical_archive, archive.encode());
                let mut conflicting = plan.clone();
                conflicting.environment.source_date_epoch += 1;
                assert!(reopened.publish(URI, &conflicting, &archive).is_err());
            } else {
                assert_eq!(reopened.fetch(URI, digest), Err(MirrorRefusal::NotMirrored));
            }
            assert_eq!(
                reopened
                    .publish(URI, &plan, &archive)
                    .unwrap_or_else(|error| panic!("{error:?}")),
                digest
            );
            drop(reopened);
            let recovered = SourceMirror::open(root.0.clone())
                .unwrap_or_else(|error| panic!("{error:?}"))
                .fetch(URI, digest)
                .unwrap_or_else(|error| panic!("{error:?}"));
            assert_eq!(recovered.plan, plan);
            assert_eq!(recovered.source.uri, URI);
            assert_eq!(recovered.source.canonical_archive, archive.encode());
        }
    }
}

#[test]
fn mutable_source_mirror_serializes_publishers() {
    let root = Directory::new();
    let (archive, plan) = source_fixture();
    let barrier = std::sync::Arc::new(std::sync::Barrier::new(8));
    let handles: Vec<_> = (0..8)
        .map(|epoch| {
            let root = root.0.clone();
            let archive = archive.clone();
            let mut plan = plan.clone();
            plan.environment.source_date_epoch += epoch;
            let barrier = barrier.clone();
            std::thread::spawn(move || {
                let mirror = SourceMirror::open(root).unwrap_or_else(|error| panic!("{error:?}"));
                barrier.wait();
                (mirror.publish(URI, &plan, &archive), plan)
            })
        })
        .collect();
    let outcomes: Vec<_> = handles
        .into_iter()
        .map(|h| h.join().unwrap_or_else(|error| panic!("{error:?}")))
        .collect();
    let winners: Vec<_> = outcomes
        .iter()
        .filter(|(result, _)| result.is_ok())
        .collect();
    assert_eq!(winners.len(), 1);
    let restored = SourceMirror::open(root.0.clone())
        .unwrap_or_else(|error| panic!("{error:?}"))
        .fetch(URI, hash(&archive.encode()))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(restored.plan, winners[0].1);
}

#[test]
fn mutable_source_mirror_preserves_legacy_conflicts_and_corrupt_commits() {
    let root = Directory::new();
    let (archive, plan) = source_fixture();
    let digest = legacy(&root, &archive, &plan);
    let mirror = SourceMirror::open(root.0.clone()).unwrap_or_else(|error| panic!("{error:?}"));
    let mut conflicting = plan.clone();
    conflicting.artifact_path = "different.wasm".into();
    assert!(mirror.publish(URI, &conflicting, &archive).is_err());
    assert!(mirror.publish("ipfs://different", &plan, &archive).is_err());
    assert_eq!(
        mirror
            .fetch(URI, digest)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .plan,
        plan
    );
    mirror
        .publish(URI, &plan, &archive)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let binding_path = root.0.join(format!("{}.binding", hex::encode(&digest)));
    fs::write(&binding_path, b"{corrupt").unwrap_or_else(|error| panic!("{error:?}"));
    assert!(matches!(
        mirror.fetch(URI, digest),
        Err(MirrorRefusal::Unreadable { .. })
    ));
    assert!(mirror.publish(URI, &plan, &archive).is_err());
    assert_eq!(
        fs::read(binding_path).unwrap_or_else(|error| panic!("{error:?}")),
        b"{corrupt"
    );
}

// A peer transcript supplies committed producer outputs to the real build
// verifier. The mirror, outcome codec, journal and provenance store are the
// production components under repair, with no replacement implementations.
struct BuildTranscript {
    attempts: std::cell::Cell<u32>,
}
impl BuildRunner for BuildTranscript {
    fn run(&self, attempt: &BuildAttempt<'_>) -> Result<Vec<u8>, BuildRefusal> {
        assert_eq!(attempt.attempt, self.attempts.get());
        self.attempts.set(attempt.attempt + 1);
        let (archive, plan) = source_fixture();
        assert_eq!(*attempt.archive, archive);
        assert_eq!(*attempt.plan, plan);
        Ok(transcript()["wasm.2"].clone())
    }
}

fn source_record(
    deployment: &layerx_programs::VerifiedDeploymentEvidence,
    build: &ReproducibleBuild,
    plan: BuildPlan,
) -> VerifiedSource {
    let header = layerx_programs::chain_proof::header::ChainHeader::decode(
        &deployment.proof().state.commitment.signed_header,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    VerifiedSource {
        program: deployment.program(),
        version: deployment.version(),
        source_uri: build.source_uri.clone(),
        source_digest: build.source_digest,
        artifact_digest: build.artifact_digest,
        plan,
        deployment_provenance: VerifiedSourceDeployment {
            abi_version: deployment.abi_version(),
            code_hash: deployment.code_hash(),
            activity_id: deployment.activity_id(),
            receipt_digest: deployment.receipt_digest(),
            batch_header_digest: header.hash().unwrap_or_else(|error| panic!("{error:?}")),
            state_root: deployment.state_root(),
            programs_root: deployment.programs_root(),
            observed_sequence: deployment.freshness().observed_sequence,
            observed_at: deployment.freshness().observed_at,
        },
    }
}

fn assert_mismatch_response(response: &Response) {
    let body: Value =
        serde_json::from_str(&response.body).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(body["error"]["code"], "source_mismatch");
    assert_eq!(
        body["verification"]["reproduced_artifact_digest"],
        hex::encode(&hash(&code(2)))
    );
    assert_eq!(
        body["verification"]["code_hash"],
        hex::encode(&hash(&code(1)))
    );
}

fn assert_replayed_mismatch_after_reopen(
    root: &Directory,
    now: u64,
    scope: &str,
    request_digest: [u8; 32],
    response: &Response,
    bytes: &[u8],
) {
    let path = root.0.join("requests").join(format!("{scope}.request"));
    let journal = VerificationJournal::open(root.0.join("requests"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let reconciled = journal
        .reconcile(now + 2)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!((reconciled.live, reconciled.settled), (0, 1));
    assert!(reconciled.defects.is_empty());
    let lease = journal
        .lease(Instant::now() + Duration::from_secs(1))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        lease
            .admit(scope, "alice", program(), request_digest, now + 3)
            .unwrap_or_else(|error| panic!("{error:?}")),
        RequestAdmission::Replay(response.clone())
    );
    assert_eq!(
        fs::read(&path).unwrap_or_else(|error| panic!("{error:?}")),
        bytes
    );
    assert!(VerifiedSourceStore::open(root.0.join("verified"))
        .unwrap_or_else(|error| panic!("{error:?}"))
        .records()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .is_empty());
}

fn assert_mirror_reopened(root: &Directory, source_digest: [u8; 32], mirrored: &MirroredSource) {
    assert_eq!(
        SourceMirror::open(root.0.join("mirror"))
            .unwrap_or_else(|error| panic!("{error:?}"))
            .fetch(URI, source_digest)
            .unwrap_or_else(|error| panic!("{error:?}")),
        mirrored.clone()
    );
}

fn mismatched_source(
    root: &Directory,
    deployment: &layerx_programs::VerifiedDeploymentEvidence,
) -> (
    SourceMirror,
    MirroredSource,
    BuildPlan,
    [u8; 32],
    ReproducibleBuild,
) {
    let (archive, plan) = source_fixture();
    let mirror =
        SourceMirror::open(root.0.join("mirror")).unwrap_or_else(|error| panic!("{error:?}"));
    let source_digest = mirror
        .publish(URI, &plan, &archive)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mirrored = mirror
        .fetch(URI, source_digest)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let verifier = SourceVerifier::new(
        BuildTranscript {
            attempts: std::cell::Cell::new(0),
        },
        2,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let reproduced = verifier
        .reproduce(&mirrored.source, &mirrored.plan)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(verifier.runner().attempts.get(), 2);
    assert_ne!(reproduced.artifact_digest, deployment.code_hash());
    (mirror, mirrored, plan, source_digest, reproduced)
}

#[test]
fn mismatch_outcome_persistence_replays_original_409_after_restart() {
    let admitted = admission(1);
    let deployment = admitted
        .verifier
        .verify_deployment(&admitted.proof, admitted.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let root = Directory::new();
    let (mirror, mirrored, plan, source_digest, reproduced) = mismatched_source(&root, &deployment);
    let mut registry = Registry::new();
    registry
        .record_verified_deployment(&deployment)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let status = registry
        .verify_source(program(), 1, &reproduced)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(matches!(status, SourceStatus::Mismatch { .. }));
    let response = Registrar::verification_response(&deployment, &reproduced, status);
    assert_eq!(response.status, 409);
    assert_mismatch_response(&response);
    let candidate = source_record(&deployment, &reproduced, plan);
    let store = VerifiedSourceStore::open(root.0.join("verified"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(store.record(&candidate).is_err());
    let request_digest = hash(
        &serde_json::to_vec(
            &json!({"source_uri": URI, "source_digest": hex::encode(&source_digest)}),
        )
        .unwrap_or_else(|error| panic!("{error:?}")),
    );
    let scope = hex::encode(&hash(b"principal/program/idempotency-key"));
    let journal = VerificationJournal::open(root.0.join("requests"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let lease = journal
        .lease(Instant::now() + Duration::from_secs(1))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let RequestAdmission::Build(record) = lease
        .admit(&scope, "alice", program(), request_digest, admitted.now)
        .unwrap_or_else(|error| panic!("{error:?}"))
    else {
        panic!("expected first rebuild");
    };
    let original = Registrar::settle_rebuild_outcome(
        &lease,
        record,
        response.clone(),
        Some(candidate),
        admitted.now,
    )
    .err()
    .unwrap_or_else(|| panic!("expected an error"));
    assert_eq!(original, response);
    let path = root.0.join("requests").join(format!("{scope}.request"));
    let bytes = fs::read(&path).unwrap_or_else(|error| panic!("{error:?}"));
    let persisted: layerx_platform_registry::VerificationRecord =
        serde_json::from_slice(&bytes).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(persisted.attempt, 1);
    assert_eq!(
        persisted.state,
        VerificationState::Completed {
            response: response.clone()
        }
    );
    assert_eq!(
        lease
            .admit(&scope, "alice", program(), request_digest, admitted.now + 1)
            .unwrap_or_else(|error| panic!("{error:?}")),
        RequestAdmission::Replay(response.clone())
    );
    assert_eq!(
        lease
            .admit(
                &scope,
                "alice",
                program(),
                hash(b"changed request"),
                admitted.now + 1
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
        RequestAdmission::Conflict
    );
    assert_eq!(
        fs::read(&path).unwrap_or_else(|error| panic!("{error:?}")),
        bytes
    );
    assert!(store
        .records()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .is_empty());
    drop(lease);
    drop(journal);
    drop(store);
    drop(mirror);
    assert_replayed_mismatch_after_reopen(
        &root,
        admitted.now,
        &scope,
        request_digest,
        &response,
        &bytes,
    );
    assert_mirror_reopened(&root, source_digest, &mirrored);
}

#[test]
fn mutable_source_mirror_ignores_partial_staging_and_refuses_partial_legacy() {
    let producer = Directory::new();
    let (archive, plan) = source_fixture();
    let digest = SourceMirror::open(producer.0.clone())
        .unwrap_or_else(|error| panic!("{error:?}"))
        .publish(URI, &plan, &archive)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let name = format!("{}.binding", hex::encode(&digest));
    let binding = fs::read(producer.0.join(&name)).unwrap_or_else(|error| panic!("{error:?}"));
    for cut in [0, 1, binding.len() / 2, binding.len() - 1, binding.len()] {
        let root = Directory::new();
        fs::write(root.0.join(format!("{name}.tmp")), &binding[..cut])
            .unwrap_or_else(|error| panic!("{error:?}"));
        let mirror = SourceMirror::open(root.0.clone()).unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(mirror.fetch(URI, digest), Err(MirrorRefusal::NotMirrored));
        mirror
            .publish(URI, &plan, &archive)
            .unwrap_or_else(|error| panic!("{error:?}"));
        drop(mirror);
        assert_eq!(
            SourceMirror::open(root.0.clone())
                .unwrap_or_else(|error| panic!("{error:?}"))
                .fetch(URI, digest)
                .unwrap_or_else(|error| panic!("{error:?}"))
                .plan,
            plan
        );
    }
    let root = Directory::new();
    let archive_path = root.0.join(format!("{}.archive", hex::encode(&digest)));
    fs::write(&archive_path, archive.encode()).unwrap_or_else(|error| panic!("{error:?}"));
    let mirror = SourceMirror::open(root.0.clone()).unwrap_or_else(|error| panic!("{error:?}"));
    assert!(matches!(
        mirror.fetch(URI, digest),
        Err(MirrorRefusal::Unreadable { .. })
    ));
    assert!(mirror.publish(URI, &plan, &archive).is_err());
    assert_eq!(
        fs::read(archive_path).unwrap_or_else(|error| panic!("{error:?}")),
        archive.encode()
    );
    assert!(!root.0.join(name).exists());
}

#[test]
fn mismatch_outcome_persistence_settles_real_deployment_pipeline_refusal() {
    let admitted = admission(1);
    let deployment = admitted
        .verifier
        .verify_deployment(&admitted.proof, admitted.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let root = Directory::new();
    let (archive, plan) = source_fixture();
    let mirror =
        SourceMirror::open(root.0.join("mirror")).unwrap_or_else(|error| panic!("{error:?}"));
    let source_digest = mirror
        .publish(URI, &plan, &archive)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mirrored = mirror
        .fetch(URI, source_digest)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let verifier = SourceVerifier::new(
        BuildTranscript {
            attempts: std::cell::Cell::new(0),
        },
        2,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let refused = verifier
        .reproduce_deployment(&mirrored.source, &mirrored.plan, &deployment)
        .err()
        .unwrap_or_else(|| panic!("expected an error"));
    assert_eq!(
        refused,
        BuildRefusal::Registry(layerx_programs::RegistryError::DeploymentMismatch)
    );
    assert_eq!(verifier.runner().attempts.get(), 2);
    let response = Registrar::rebuild_refusal(&refused);
    assert_eq!(response.status, 409);
    assert_eq!(
        serde_json::from_str::<Value>(&response.body).unwrap_or_else(|error| panic!("{error:?}"))
            ["error"]["code"],
        "source_mismatch"
    );
    let digest = hash(&archive.encode());
    let scope = hex::encode(&hash(b"real-deployment-pipeline-mismatch"));
    let journal = VerificationJournal::open(root.0.join("requests"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let lease = journal
        .lease(Instant::now() + Duration::from_secs(1))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let RequestAdmission::Build(record) = lease
        .admit(&scope, "alice", program(), digest, admitted.now)
        .unwrap_or_else(|error| panic!("{error:?}"))
    else {
        panic!("expected build");
    };
    assert_eq!(
        Registrar::settle_rebuild_outcome(&lease, record, response.clone(), None, admitted.now),
        Err(response.clone())
    );
    assert_eq!(
        lease
            .admit(&scope, "alice", program(), digest, admitted.now + 1)
            .unwrap_or_else(|error| panic!("{error:?}")),
        RequestAdmission::Replay(response.clone())
    );
    drop(lease);
    drop(journal);
    let reopened = VerificationJournal::open(root.0.join("requests"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        reopened
            .reconcile(admitted.now + 2)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .settled,
        1
    );
    let lease = reopened
        .lease(Instant::now() + Duration::from_secs(1))
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        lease
            .admit(&scope, "alice", program(), digest, admitted.now + 2)
            .unwrap_or_else(|error| panic!("{error:?}")),
        RequestAdmission::Replay(response)
    );
    assert_eq!(verifier.runner().attempts.get(), 2);
    assert!(VerifiedSourceStore::open(root.0.join("verified"))
        .unwrap_or_else(|error| panic!("{error:?}"))
        .records()
        .unwrap_or_else(|error| panic!("{error:?}"))
        .is_empty());
}
