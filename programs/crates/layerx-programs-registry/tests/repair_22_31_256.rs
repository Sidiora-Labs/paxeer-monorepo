#[path = "support/chain.rs"]
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
    deployment_envelope::DeploymentEnvelope,
    interface_state_key, interface_state_value, verify_interface_read, BuildAttempt,
    BuildEnvironment, BuildPlan, CanonicalAccountLeaf, DeploymentExecutionProof,
    DeploymentJournalEntry, DeploymentProof, DeploymentRecord, InterfaceEntryPoint,
    InterfaceStateWitness, ProgramId, ProgramInterface, ProgramLifecycleProof, ProgramStateProof,
    ProtocolDeploymentVerifier, Registry, SourceArchive, SourceFile, UpgradePolicy, ValueSchema,
    ValueType, VerifiedAccountSnapshot,
};
use layerx_programs_runtime::{Deploy, Lifecycle, ProgramVersion, Upgrade};
use sha2::{Digest, Sha256};
use std::{
    collections::BTreeMap,
    fs,
    path::PathBuf,
    sync::{
        atomic::{AtomicU64, Ordering},
        OnceLock,
    },
};

const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const WASM1: &[u8] = &[
    0, 97, 115, 109, 1, 0, 0, 0, 1, 12, 2, 96, 2, 127, 127, 1, 127, 96, 1, 127, 1, 127, 3, 3, 2, 0,
    1, 5, 3, 1, 0, 1, 7, 34, 3, 4, b'c', b'a', b'l', b'l', 0, 0, 14, b'l', b'a', b'y', b'e', b'r',
    b'x', b'_', b'r', b'e', b's', b'e', b'r', b'v', b'e', 0, 1, 6, b'm', b'e', b'm', b'o', b'r',
    b'y', 2, 0, 10, 11, 2, 4, 0, 65, 0, 11, 4, 0, 65, 0, 11,
];
fn wasm2() -> Vec<u8> {
    use layerx_programs_runtime::test_support::*;
    let mut exports = unsigned_leb(4);
    for (name, kind, index) in [
        ("call", 0, 0),
        ("layerx_reserve", 0, 1),
        ("migrate", 0, 2),
        ("memory", 2, 0),
    ] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.push(kind);
        exports.extend(unsigned_leb(index));
    }
    module(&[
        type_section(&[
            (&[TYPE_I32, TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[], &[TYPE_I32]),
        ]),
        function_section(&[0, 1, 2]),
        raw_section(5, &[1, 0, 1]),
        raw_section(7, &exports),
        code_section(&[
            func_body(&[], &[OP_I32_CONST, 0, OP_END]),
            func_body(&[], &[OP_I32_CONST, 0, OP_END]),
            func_body(&[], &[OP_I32_CONST, 0, OP_END]),
        ]),
    ])
}
fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
fn program() -> ProgramId {
    ProgramId::new([0x31; 32]).unwrap()
}
fn policy() -> UpgradePolicy {
    UpgradePolicy::Authority([0x51; 32])
}
fn interface(code: &[u8]) -> ProgramInterface {
    ProgramInterface::bind(
        code,
        1,
        vec![InterfaceEntryPoint {
            name: "call".into(),
            discriminator: [1, 2, 3, 4],
            calldata: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            response: ValueSchema::layerx(ValueType::Bytes { max_len: 64 }),
            capabilities: vec![],
            event_topics: vec![],
            failures: vec![],
        }],
    )
    .unwrap()
}
struct Temp(PathBuf);
impl Temp {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "registry-repair-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap();
        Self(path)
    }
}
impl Drop for Temp {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}
// Recorded from the node's TxRaw, EVM transaction, ABI and deterministic
// result serializers. Host rows were exported and restored by ctx.go's bridge;
// migration accounting was emitted by deploy.c's production stage. The gate
// consumes the committed transcript and executes no external producer.
fn node_producer() -> &'static BTreeMap<String, Vec<u8>> {
    static PRODUCED: OnceLock<BTreeMap<String, Vec<u8>>> = OnceLock::new();
    PRODUCED.get_or_init(|| {
        let transcript = include_str!("../src/repair_22_31_256.hex");
        assert_eq!(
            layerx_programs::hex::encode(&hash(transcript.as_bytes())),
            "5798c9304ce5dd774f153da4f4f3208f1205f4cbfd1854732a49636dc9d76c6e"
        );
        transcript
            .lines()
            .map(|line| {
                let (key, value) = line.split_once('=').expect("producer framing");
                (key.to_owned(), layerx_programs::hex::decode(value).unwrap())
            })
            .collect()
    })
}

fn sign(fixture: &mut ChainFixture) {
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture.header.hash().unwrap().to_vec();
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
    .unwrap();
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
fn program_key() -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program().bytes());
    programs_state_key(&key)
}
fn state_value(record: &DeploymentRecord) -> Vec<u8> {
    let mut bytes = vec![1];
    bytes.extend_from_slice(&[0x51; 32]);
    bytes.extend_from_slice(&record.new_code_hash);
    bytes.extend_from_slice(&record.abi_version.to_be_bytes());
    bytes.extend_from_slice(&record.version.to_be_bytes());
    bytes
}
fn runtime_records() -> [DeploymentRecord; 2] {
    let mut lifecycle = Lifecycle::declared().unwrap();
    let first = lifecycle
        .deploy(Deploy {
            program: program(),
            code_hash: hash(WASM1),
            wasm: WASM1.to_vec(),
            abi_version: 1,
            upgrade_policy: policy(),
        })
        .unwrap();
    let second = lifecycle
        .upgrade(Upgrade {
            program: program(),
            authority: [0x51; 32],
            code_hash: hash(&wasm2()),
            wasm: wasm2(),
            abi_version: 1,
            migration: None,
        })
        .unwrap();
    [
        (&first, WASM1.to_vec(), 39, 1_700_000_001_123),
        (&second, wasm2(), 40, 1_700_000_002_123),
    ]
    .map(|(receipt, wasm, sequence, observed_at)| {
        DeploymentRecord::from_deployment(
            receipt,
            &ProgramVersion {
                code_hash: hash(&wasm),
                wasm,
                abi_version: 1,
            },
            policy(),
            sequence,
            observed_at,
        )
        .unwrap()
    })
}
struct Admission {
    fixture: ChainFixture,
    proof: DeploymentProof,
    now: u64,
    verifier: ProtocolDeploymentVerifier,
}
fn admission(
    version: usize,
    transaction: &[u8],
    result_data: &[u8],
    result_code: u32,
) -> Admission {
    admission_with_records(
        &runtime_records(),
        version,
        transaction,
        result_data,
        result_code,
    )
}
fn admission_with_records(
    records: &[DeploymentRecord; 2],
    version: usize,
    transaction: &[u8],
    result_data: &[u8],
    result_code: u32,
) -> Admission {
    let record = &records[version - 1];
    let interface = interface(&record.module);
    let interface_key = programs_state_key(&interface_state_key(program()));
    let prior_rows = if version == 1 {
        vec![(b"unrelated".to_vec(), vec![1])]
    } else {
        vec![(program_key(), state_value(&records[0]))]
    };
    let mut prior = ChainFixture::new("repair-22-31-256", &SEEDS, prior_rows).unwrap();
    prior.header.height = 37 + version as i64;
    prior.header.time.seconds += version as i64 - 1;
    let tx_hash = hash(transaction);
    let tx_root = hash_from_byte_slices(&[&tx_hash]);
    prior.header.data_hash = tx_root.to_vec();
    sign(&mut prior);
    let mut rows = vec![
        (program_key(), state_value(record)),
        (
            interface_key.clone(),
            interface_state_value(program(), version as u32, &interface).unwrap(),
        ),
    ];
    let migration_key = record.migration.as_ref().map(|migration| {
        let accounting = migration
            .chain_accounting()
            .expect("host migration evidence");
        let mut key = b"progmig/account/v1/".to_vec();
        key.extend_from_slice(&accounting[61..93]);
        let key = programs_state_key(&key);
        rows.push((key.clone(), accounting.to_vec()));
        key
    });
    let mut fixture = ChainFixture::new("repair-22-31-256", &SEEDS, rows).unwrap();
    fixture.header.height = prior.header.height + 1;
    fixture.header.time.seconds = prior.header.time.seconds + 1;
    fixture.header.last_block_id.hash = prior.commit.block_id.hash.clone();
    fixture.header.last_block_id.part_set_total = prior.commit.block_id.part_set_header.total;
    fixture.header.last_block_id.part_set_hash = prior.commit.block_id.part_set_header.hash.clone();
    let result = TxResult {
        code: result_code,
        data: result_data.to_vec(),
        gas_wanted: 0,
        gas_used: 0,
    };
    // The consensus producer's actual deterministic result encoder supplies this leaf.
    let result_bytes = result_encoding(&result);
    let result_root = hash_from_byte_slices(&[&result_bytes]);
    fixture.header.last_results_hash = result_root.to_vec();
    sign(&mut fixture);
    let commitment = decode_commitment_proof(&fixture.encode_key(&program_key()).unwrap()).unwrap();
    let state = ChainProgramStateProof {
        program: program(),
        ops: fixture.prove(&program_key()).unwrap(),
    };
    let mut wind_down_key = b"wind-down\0s".to_vec();
    wind_down_key.extend_from_slice(&program().bytes());
    let wind_down = ChainWindDownProof {
        program: program(),
        value: None,
        ops: fixture
            .prove_absent(&programs_state_key(&wind_down_key))
            .unwrap(),
    };
    let mut proof = DeploymentProof {
        state: ProgramStateProof {
            commitment,
            state: state.clone(),
            lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                program: program(),
                record: Some(state_value(record)),
                record_ops: state.ops.clone(),
                status: None,
                status_ops: wind_down.ops.clone(),
            }),
            wind_down,
            transaction: vec![],
            transaction_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: tx_root,
                aunts: vec![],
            },
        },
        deployment: ChainDeploymentProof {
            program_state: state,
            code: record.module.clone(),
            interface: Some(interface.canonical_encoding().to_vec()),
        },
    };
    proof
        .bind_execution(&DeploymentExecutionProof {
            transaction: transaction.to_vec(),
            previous: decode_commitment_proof(&prior.encode_key(&program_key()).unwrap()).unwrap(),
            result,
            result_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: result_root,
                aunts: vec![],
            },
            interface_ops: fixture.prove(&interface_key).unwrap(),
            migration: record.migration.clone(),
            migration_ops: migration_key
                .as_ref()
                .map(|key| fixture.prove(key).unwrap())
                .unwrap_or_default(),
            migration_previous_ops: migration_key
                .as_ref()
                .map(|key| prior.prove_absent(key).unwrap())
                .unwrap_or_default(),
        })
        .unwrap();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.verifier.chain_id,
        fixture.verifier.validators.validators().to_vec(),
        5_000,
    )
    .unwrap();
    let now =
        fixture.header.time.seconds as u64 * 1000 + fixture.header.time.nanos as u64 / 1_000_000;
    Admission {
        fixture,
        proof,
        now,
        verifier,
    }
}
fn result_encoding(result: &TxResult) -> Vec<u8> {
    let producer = node_producer();
    let key = match (
        result.code,
        result.data == producer["success"],
        result.data == producer["failed"],
    ) {
        (0, true, false) => "success",
        (0, false, true) => "failed",
        (9, true, false) => "code",
        _ => panic!("test requested a result not emitted by the node producer"),
    };
    let bytes = producer[&format!("{key}.leaf")].clone();
    assert_eq!(
        hash_from_byte_slices(&[&bytes]).as_slice(),
        producer[&format!("{key}.root")]
    );
    bytes
}
fn verified(version: usize) -> (Admission, layerx_programs::VerifiedDeploymentEvidence) {
    let producer = node_producer();
    let fixture = admission(
        version,
        &producer[if version == 1 { "deploy" } else { "upgrade" }],
        &producer["success"],
        0,
    );
    let evidence = fixture
        .verifier
        .verify_deployment(&fixture.proof, fixture.now)
        .unwrap();
    assert_eq!(evidence.record(), &runtime_records()[version - 1]);
    (fixture, evidence)
}

#[test]
fn deployment_transaction_binding() {
    let (fixture, evidence) = verified(1);
    assert_eq!(node_producer()["wasm.1"], WASM1);
    assert_eq!(node_producer()["wasm.2"], wasm2());
    for (version, code) in [(1, WASM1.to_vec()), (2, wasm2())] {
        assert_eq!(
            node_producer()[&format!("interface.{version}")],
            interface(&code).canonical_encoding()
        );
    }
    assert_eq!(evidence.receipt_digest(), hash(&node_producer()["deploy"]));
    assert_eq!(evidence.version(), 1);
    for (transaction, data, code) in [
        (
            &node_producer()["unrelated"],
            &node_producer()["success"],
            0,
        ),
        (&node_producer()["deploy"], &node_producer()["failed"], 0),
        (&node_producer()["deploy"], &node_producer()["success"], 9),
    ] {
        let invalid = admission(1, transaction, data, code);
        assert!(invalid
            .verifier
            .verify_deployment(&invalid.proof, invalid.now)
            .is_err());
    }
    let mut changed = fixture.proof.clone();
    let mut execution = DeploymentExecutionProof::decode(&changed.state.transaction).unwrap();
    execution.result_proof.index = 1;
    changed.bind_execution(&execution).unwrap();
    assert!(fixture
        .verifier
        .verify_deployment(&changed, fixture.now)
        .is_err());
    let mut changed = fixture.proof.clone();
    changed.state.transaction = b"chain program deployment".to_vec();
    assert!(fixture
        .verifier
        .verify_deployment(&changed, fixture.now)
        .is_err());
    let mut changed = fixture.proof.clone();
    let mut alternate = interface(WASM1).entries().to_vec();
    alternate[0].discriminator = [9; 4];
    changed.deployment.interface = Some(
        ProgramInterface::bind(WASM1, 1, alternate)
            .unwrap()
            .canonical_encoding()
            .to_vec(),
    );
    assert!(fixture
        .verifier
        .verify_deployment(&changed, fixture.now)
        .is_err());
}

#[test]
fn deployment_interface_rebinding() {
    for version in [1, 2] {
        let (_, evidence) = verified(version);
        let authenticated = evidence.interface().unwrap();
        assert_eq!(
            ProgramInterface::bind_deployment(&evidence, authenticated.entries().to_vec()).unwrap(),
            *authenticated
        );
        let prior = interface(WASM1);
        for discriminator in [false, true] {
            let mut replacement = authenticated.entries().to_vec();
            if discriminator {
                replacement[0].discriminator = [8; 4];
            } else {
                replacement[0].calldata = ValueSchema::layerx(ValueType::Bytes { max_len: 65 });
            }
            assert!(ProgramInterface::bind(evidence.module(), 1, replacement.clone()).is_ok());
            assert!(ProgramInterface::bind_deployment(&evidence, replacement.clone()).is_err());
            assert!(
                ProgramInterface::bind_verified_upgrade(&evidence, replacement, &prior, true)
                    .is_err()
            );
        }
    }
}

#[test]
fn upgrade_journal_recovery() {
    let (first, initial) = verified(1);
    let (second, upgrade) = verified(2);
    assert_eq!(upgrade.record().old_code_hash, Some(initial.code_hash()));
    upgrade.record().validate().unwrap();
    let mut registry = Registry::new();
    registry.record_verified_deployment(&initial).unwrap();
    registry
        .record_verified_upgrade(&initial, &upgrade)
        .unwrap();
    let temp = Temp::new();
    first
        .proof
        .publish_canonical(&temp.0.join("1.admission"))
        .unwrap();
    second
        .proof
        .publish_canonical(&temp.0.join("2.admission"))
        .unwrap();
    fs::write(
        temp.0.join("1.record"),
        initial.record().canonical_encoding(),
    )
    .unwrap();
    fs::write(
        temp.0.join("2.record"),
        upgrade.record().canonical_encoding(),
    )
    .unwrap();
    for (version, evidence) in [(1, &initial), (2, &upgrade)] {
        let envelope = DeploymentEnvelope::from_evidence(evidence);
        fs::write(
            temp.0.join(format!("{version}.envelope")),
            envelope.canonical_encoding().unwrap(),
        )
        .unwrap();
    }
    drop(registry);
    drop(first);
    drop(second);
    let mut recovered = Registry::new();
    for (version, expected) in [(1, initial), (2, upgrade)] {
        let proof =
            DeploymentProof::read_canonical(&temp.0.join(format!("{version}.admission"))).unwrap();
        let producer = admission(
            version,
            &node_producer()[if version == 1 { "deploy" } else { "upgrade" }],
            &node_producer()["success"],
            0,
        );
        let evidence = producer
            .verifier
            .verify_deployment(&proof, producer.now)
            .unwrap();
        let record =
            DeploymentRecord::decode(&fs::read(temp.0.join(format!("{version}.record"))).unwrap())
                .unwrap();
        record.validate().unwrap();
        let envelope = DeploymentEnvelope::decode(
            &fs::read(temp.0.join(format!("{version}.envelope"))).unwrap(),
        )
        .unwrap();
        assert_eq!(envelope.record(), &record);
        assert_eq!(envelope.proof(), &proof);
        assert_eq!(envelope.receipt_digest(), evidence.receipt_digest());
        assert_eq!(
            envelope.journal_entry().unwrap(),
            DeploymentJournalEntry::from_evidence(&evidence)
        );
        assert_eq!(record, *evidence.record());
        assert_eq!(evidence, expected);
        recovered.record_verified_deployment(&evidence).unwrap();
        assert_eq!(
            recovered.resolve_deployment(evidence).unwrap().version(),
            version as u32
        );
    }
    assert_eq!(recovered.latest_version(program()), Ok(2));
    hooked_upgrade_recovery();
}

#[test]
fn replay_deployment_identity() {
    let (_, evidence) = verified(1);
    let mut registry = Registry::new();
    registry.record_verified_deployment(&evidence).unwrap();
    assert_eq!(
        registry.resolve_deployment(evidence.clone()).unwrap(),
        evidence
    );
    let entry = DeploymentJournalEntry::from_evidence(&evidence);
    assert_ne!(entry.transaction_id().bytes(), entry.record().digest());
    let temp = Temp::new();
    fs::write(temp.0.join("deployment"), entry.canonical_encoding()).unwrap();
    drop(registry);
    let entry =
        DeploymentJournalEntry::decode(&fs::read(temp.0.join("deployment")).unwrap()).unwrap();
    assert_eq!(entry.transaction_id().bytes(), evidence.receipt_digest());
    let mut registry = Registry::new();
    registry.replay_journal(&[entry]).unwrap();
    assert_eq!(
        registry.resolve_deployment(evidence.clone()).unwrap(),
        evidence
    );
}

#[test]
fn legacy_interface_root() {
    let (fixture, _) = verified(1);
    let mut state = fixture.proof.state.clone();
    // A current head's transaction inclusion is a separate read coordinate.
    let raw = node_producer()["deploy"].clone();
    let tx_root = hash_from_byte_slices(&[&hash(&raw)]);
    let mut chain = fixture.fixture;
    chain.header.data_hash = tx_root.to_vec();
    sign(&mut chain);
    state.commitment = decode_commitment_proof(&chain.encode_key(&program_key()).unwrap()).unwrap();
    state.transaction = raw;
    state.transaction_proof.leaf_hash = tx_root;
    let head = fixture
        .verifier
        .verify_current_program(&state, program(), fixture.now)
        .unwrap();
    let descriptor = interface(WASM1);
    let key = interface_state_key(program());
    let witness = InterfaceStateWitness {
        key: key.clone(),
        value: interface_state_value(program(), 1, &descriptor).unwrap(),
        proof: chain.prove(&programs_state_key(&key)).unwrap(),
    };
    let read = verify_interface_read(&head, &witness).unwrap();
    assert_eq!(read.interface, descriptor);
    assert_eq!(read.state_root, head.state_root());
    let mut corrupted = witness.clone();
    corrupted.proof[1].data[0] ^= 1;
    assert!(verify_interface_read(&head, &corrupted).is_err());
    let legacy = InterfaceStateWitness {
        key: witness.key,
        value: witness.value,
        proof: layerx_programs::StateProof {
            leaf_index: 0,
            leaf_count: 1,
            siblings: vec![],
        },
    };
    assert!(verify_interface_read(&head, &legacy).is_err());
}

#[test]
fn archive_bound_roundtrip() {
    const LIMIT: usize = 64 * 1024 * 1024;
    let files = || {
        (0..8)
            .map(|index| SourceFile {
                path: format!("file{index}"),
                executable: false,
                content: vec![
                    index;
                    if index == 7 {
                        8 * 1024 * 1024 - (12 + 8 * (7 + 5))
                    } else {
                        8 * 1024 * 1024
                    }
                ],
            })
            .collect::<Vec<_>>()
    };
    let archive = SourceArchive::new(files()).unwrap();
    let encoded = archive.encode();
    assert_eq!(encoded.len(), LIMIT);
    assert_eq!(SourceArchive::decode(&encoded).unwrap(), archive);
    drop(encoded);
    drop(archive);
    let mut over = files();
    over[7].content.push(0);
    assert_eq!(
        SourceArchive::new(over),
        Err(layerx_programs::ArchiveError::TooLarge)
    );
}

#[test]
fn build_command_roundtrip() {
    let temp = Temp::new();
    let plan = BuildPlan {
        environment: BuildEnvironment {
            builder_image_digest: [1; 32],
            toolchain_digest: [2; 32],
            dependency_lock_digest: [3; 32],
            source_date_epoch: 1,
            command: vec![
                "sh".into(),
                "-c".into(),
                "printf '%s' 'argument with spaces'".into(),
                "unicode λ,= ' \"".into(),
            ],
        },
        artifact_path: "artifact.wasm".into(),
        toolchain_manifest: "toolchain".into(),
        dependency_lock: "lock".into(),
    };
    let encoded = plan.encode();
    assert!(encoded.starts_with("version = 2\n"));
    assert_eq!(BuildPlan::parse(&encoded).unwrap(), plan);
    fs::write(temp.0.join("verification-plan"), encoded).unwrap();
    let recovered =
        BuildPlan::parse(&fs::read_to_string(temp.0.join("verification-plan")).unwrap()).unwrap();
    let archive = SourceArchive::new(vec![SourceFile {
        path: "source".into(),
        executable: false,
        content: vec![1],
    }])
    .unwrap();
    let attempt = BuildAttempt {
        archive: &archive,
        plan: &recovered,
        attempt: 0,
    };
    let arguments = &attempt.plan.environment.command;
    let command = attempt.command().unwrap();
    assert_eq!(command.get_program(), arguments[0].as_str());
    assert_eq!(
        command.get_args().collect::<Vec<_>>(),
        arguments[1..]
            .iter()
            .map(std::ffi::OsStr::new)
            .collect::<Vec<_>>()
    );
    assert_eq!(arguments[2], "printf '%s' 'argument with spaces'");
    assert_eq!(arguments, &plan.environment.command);
}

#[test]
fn host_account_proof_shape() {
    let temp = Temp::new();
    let exported = node_producer()["host.account"].clone();
    let host_reloaded = node_producer()["host.reloaded"].clone();
    assert_eq!(exported, host_reloaded);
    let path = temp.0.join("06-account-row");
    fs::write(&path, &exported).unwrap();
    let reloaded = fs::read(&path).unwrap();
    assert_eq!(exported, reloaded);
    let leaf = CanonicalAccountLeaf::decode_host_record(&exported[33..]).unwrap();
    assert_eq!(leaf.balance, (1u128 << 96) + 42);
    assert_eq!(leaf.host_record_encoding().unwrap(), exported[33..]);
    for row in [exported, reloaded] {
        assert_eq!(row[0], 6);
        assert_eq!(row[1..33], leaf.account_id);
        let chain = ChainFixture::new(
            "host-account-reload",
            &SEEDS,
            vec![(row[..33].to_vec(), row[33..].to_vec())],
        )
        .unwrap();
        let head = chain
            .verifier
            .verify_head(&chain.header, &chain.commit)
            .unwrap();
        let snapshot =
            VerifiedAccountSnapshot::verify_chain(&head, &leaf, &chain.prove(&row[..33]).unwrap())
                .unwrap();
        assert_eq!(snapshot.account(), &leaf);
        let legacy_key = programs_state_key(&leaf.primary_key());
        let legacy = ChainFixture::new(
            "host-account-reload",
            &SEEDS,
            vec![(legacy_key.clone(), leaf.primary_value().unwrap())],
        )
        .unwrap();
        let legacy_head = legacy
            .verifier
            .verify_head(&legacy.header, &legacy.commit)
            .unwrap();
        assert!(VerifiedAccountSnapshot::verify_chain(
            &legacy_head,
            &leaf,
            &legacy.prove(&legacy_key).unwrap()
        )
        .is_err());
    }
    let canonical = leaf.host_record_encoding().unwrap();
    for offset in [0, 36, canonical.len() - 1] {
        let mut corrupt = canonical.clone();
        corrupt[offset] = 2;
        assert!(CanonicalAccountLeaf::decode_host_record(&corrupt).is_err());
    }
    let mut trailing = canonical;
    trailing.push(0);
    assert!(CanonicalAccountLeaf::decode_host_record(&trailing).is_err());
}

fn hooked_upgrade_recovery() {
    use layerx_programs_runtime::Migration;
    let mut lifecycle = Lifecycle::declared().unwrap();
    lifecycle
        .deploy(Deploy {
            program: program(),
            code_hash: hash(WASM1),
            wasm: WASM1.to_vec(),
            abi_version: 1,
            upgrade_policy: policy(),
        })
        .unwrap();
    let receipt = lifecycle
        .upgrade(Upgrade {
            program: program(),
            authority: [0x51; 32],
            code_hash: hash(&wasm2()),
            wasm: wasm2(),
            abi_version: 1,
            migration: Some(Migration {
                export: "migrate".into(),
            }),
        })
        .unwrap();
    let execution = receipt.migration().expect("real runtime executed the hook");
    let temp = Temp::new();
    let accounting = node_producer()["migration.accounting"].clone();
    let migration =
        layerx_programs::MigrationExecutionEvidence::from_chain_accounting(&accounting).unwrap();
    assert_eq!(migration.usage().cpu_fuel, execution.usage.cpu_fuel);
    assert_eq!(migration.usage().memory_bytes, execution.usage.memory_bytes);
    assert_eq!(migration.usage(), execution.usage);
    let mut records = runtime_records();
    records[1].migration = Some(migration.clone());
    let producer = node_producer();
    let fixture = admission_with_records(&records, 2, &producer["hooked"], &producer["success"], 0);
    let evidence = fixture
        .verifier
        .verify_deployment(&fixture.proof, fixture.now)
        .unwrap();
    assert_eq!(evidence.record().migration.as_ref().unwrap(), &migration);
    fixture
        .proof
        .publish_canonical(&temp.0.join("hooked.admission"))
        .unwrap();
    fs::write(
        temp.0.join("hooked.record"),
        evidence.record().canonical_encoding(),
    )
    .unwrap();
    let envelope = DeploymentEnvelope::from_evidence(&evidence);
    fs::write(
        temp.0.join("hooked.envelope"),
        envelope.canonical_encoding().unwrap(),
    )
    .unwrap();
    drop(envelope);
    drop(fixture);
    let reopened_envelope =
        DeploymentEnvelope::decode(&fs::read(temp.0.join("hooked.envelope")).unwrap()).unwrap();
    let reopened = DeploymentProof::read_canonical(&temp.0.join("hooked.admission")).unwrap();
    let fixture = admission_with_records(&records, 2, &producer["hooked"], &producer["success"], 0);
    let recovered = fixture
        .verifier
        .verify_deployment(&reopened, fixture.now)
        .unwrap();
    let record =
        DeploymentRecord::decode(&fs::read(temp.0.join("hooked.record")).unwrap()).unwrap();
    record.validate().unwrap();
    assert_eq!(reopened_envelope.record(), &record);
    assert_eq!(reopened_envelope.proof(), &reopened);
    assert_eq!(record, *recovered.record());
    assert_eq!(recovered, evidence);
    assert_eq!(record.migration.unwrap().canonical_bytes(), accounting);
    let mut absent = reopened;
    let mut execution_proof = DeploymentExecutionProof::decode(&absent.state.transaction).unwrap();
    execution_proof.migration = None;
    absent.bind_execution(&execution_proof).unwrap();
    assert!(fixture
        .verifier
        .verify_deployment(&absent, fixture.now)
        .is_err());
}
