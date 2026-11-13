mod chain {
    include!("chain.rs");
}

use chain::ChainFixture;
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::{
    chain_evidence::{
        deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, programs_state_key, wind_down::ChainWindDownProof,
    },
    chain_proof::header::ChainHeader,
    chain_proof::trust_store::TrustRoot,
    chain_proof::ChainProofVerifier,
    chain_proof::{
        decode::decode_commitment_proof,
        merkle::{hash_from_byte_slices, SimpleProof},
        results::TxResult,
        validators::TrustedValidator,
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
    },
    deployment_envelope::DeploymentEnvelope,
    interface_state_key, interface_state_value, AccountStateError, CanonicalAccountLeaf,
    DeploymentExecutionProof, DeploymentJournalEntry, DeploymentProof, DeploymentRecord,
    InterfaceStateWitness, ProgramId, ProgramInterface, ProgramLifecycleProof, ProgramStateProof,
    ProtocolDeploymentVerifier, Registry, UpgradePolicy, ValueSchema, ValueType,
    VerifiedAccountSnapshot, VerifiedInterfaceRead,
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
pub fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}
pub fn program() -> ProgramId {
    ProgramId::new([0x31; 32]).unwrap_or_else(|error| panic!("{error:?}"))
}
pub fn policy() -> UpgradePolicy {
    UpgradePolicy::Authority([0x51; 32])
}
pub fn descriptor(version: usize) -> ProgramInterface {
    ProgramInterface::decode(&node_producer()[&format!("interface.{version}")])
        .unwrap_or_else(|error| panic!("{error:?}"))
}
fn interface(code: &[u8]) -> ProgramInterface {
    let version = if code == node_producer()["wasm.1"] {
        1
    } else {
        2
    };
    assert_eq!(code, node_producer()[&format!("wasm.{version}")]);
    descriptor(version)
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
        fs::create_dir(&path).unwrap_or_else(|error| panic!("{error:?}"));
        Self(path)
    }
}
impl Drop for Temp {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap_or_else(|error| panic!("{error:?}"));
    }
}
// Recorded from the node's TxRaw, EVM transaction, ABI and deterministic
// result serializers. Host rows were exported and restored by ctx.go's bridge;
// migration accounting was emitted by deploy.c's production stage. The gate
// consumes the committed transcript and executes no external producer.
pub fn node_producer() -> &'static BTreeMap<String, Vec<u8>> {
    static PRODUCED: OnceLock<BTreeMap<String, Vec<u8>>> = OnceLock::new();
    PRODUCED.get_or_init(|| {
        let transcript = include_str!("../../src/repair_22_31_256.hex");
        assert_eq!(
            layerx_programs::hex::encode(&hash(transcript.as_bytes())),
            "5798c9304ce5dd774f153da4f4f3208f1205f4cbfd1854732a49636dc9d76c6e"
        );
        transcript
            .lines()
            .map(|line| {
                let (key, value) = line
                    .split_once('=')
                    .unwrap_or_else(|| panic!("{}", "producer framing"));
                (
                    key.to_owned(),
                    layerx_programs::hex::decode(value).unwrap_or_else(|error| panic!("{error:?}")),
                )
            })
            .collect()
    })
}

pub fn sign(fixture: &mut ChainFixture) {
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
pub fn program_key() -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program().bytes());
    programs_state_key(&key)
}
pub fn state_value(record: &DeploymentRecord) -> Vec<u8> {
    let mut bytes = vec![1];
    bytes.extend_from_slice(&[0x51; 32]);
    bytes.extend_from_slice(&record.new_code_hash);
    bytes.extend_from_slice(&record.abi_version.to_be_bytes());
    bytes.extend_from_slice(&record.version.to_be_bytes());
    bytes
}
pub fn runtime_records() -> [DeploymentRecord; 2] {
    let mut lifecycle = Lifecycle::declared().unwrap_or_else(|error| panic!("{error:?}"));
    let first = lifecycle
        .deploy(Deploy {
            program: program(),
            code_hash: hash(WASM1),
            wasm: WASM1.to_vec(),
            abi_version: 1,
            upgrade_policy: policy(),
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
    let second = lifecycle
        .upgrade(Upgrade {
            program: program(),
            authority: [0x51; 32],
            code_hash: hash(&wasm2()),
            wasm: wasm2(),
            abi_version: 1,
            migration: None,
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
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
        .unwrap_or_else(|error| panic!("{error:?}"))
    })
}
pub struct Admission {
    pub fixture: ChainFixture,
    pub proof: DeploymentProof,
    pub now: u64,
    pub verifier: ProtocolDeploymentVerifier,
}
pub fn admission(
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
fn admission_state_proof(
    fixture: &ChainFixture,
    record: &DeploymentRecord,
    interface_key: &[u8],
    tx_root: [u8; 32],
) -> DeploymentProof {
    let commitment = decode_commitment_proof(
        &fixture
            .encode_key(&program_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let state = ChainProgramStateProof {
        program: program(),
        ops: fixture
            .prove(&program_key())
            .unwrap_or_else(|error| panic!("{error:?}")),
    };
    let mut wind_down_key = b"wind-down\0s".to_vec();
    wind_down_key.extend_from_slice(&program().bytes());
    let wind_down = ChainWindDownProof {
        program: program(),
        value: None,
        ops: fixture
            .prove_absent(&programs_state_key(&wind_down_key))
            .unwrap_or_else(|error| panic!("{error:?}")),
    };
    let proof = DeploymentProof {
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
        deployment: ChainDeploymentProof::from_state_proofs(
            state,
            record.module.clone(),
            &decode_commitment_proof(
                &fixture
                    .encode_key(interface_key)
                    .unwrap_or_else(|error| panic!("{error:?}")),
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
        )
        .unwrap_or_else(|error| panic!("{error:?}")),
    };
    proof
}

fn bind_admission_execution(
    proof: &mut DeploymentProof,
    fixture: &ChainFixture,
    prior: &ChainFixture,
    interface_key: &[u8],
    transaction: &[u8],
    execution: (TxResult, [u8; 32]),
    migration: (&DeploymentRecord, &Option<Vec<u8>>),
) {
    proof
        .bind_execution(&DeploymentExecutionProof {
            transaction: transaction.to_vec(),
            previous: decode_commitment_proof(
                &prior
                    .encode_key(&program_key())
                    .unwrap_or_else(|error| panic!("{error:?}")),
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
            result: execution.0,
            result_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: execution.1,
                aunts: vec![],
            },
            interface_ops: fixture
                .prove(interface_key)
                .unwrap_or_else(|error| panic!("{error:?}")),
            migration: migration.0.migration.clone(),
            migration_ops: migration
                .1
                .as_ref()
                .map(|key| {
                    fixture
                        .prove(key)
                        .unwrap_or_else(|error| panic!("{error:?}"))
                })
                .unwrap_or_default(),
            migration_previous_ops: migration
                .1
                .as_ref()
                .map(|key| {
                    prior
                        .prove_absent(key)
                        .unwrap_or_else(|error| panic!("{error:?}"))
                })
                .unwrap_or_default(),
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
}

fn prior_chain_for_admission(
    records: &[DeploymentRecord; 2],
    version: usize,
    tx_root: [u8; 32],
) -> ChainFixture {
    let mut prior = if version == 1 {
        let mut chain = ChainFixture::new(
            "repair-22-31-256",
            &SEEDS,
            vec![(b"unrelated".to_vec(), vec![1])],
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        chain.header.data_hash = tx_root.to_vec();
        sign(&mut chain);
        chain
    } else {
        // H+1 for deployment is exactly H for upgrade, including its state root,
        // predecessor, result root and transaction commitment.
        admission_with_records(
            records,
            1,
            &node_producer()["deploy"],
            &node_producer()["success"],
            0,
        )
        .fixture
    };
    assert_eq!(prior.header.data_hash, tx_root);
    sign(&mut prior);
    prior
}

pub fn admission_with_records(
    records: &[DeploymentRecord; 2],
    version: usize,
    transaction: &[u8],
    result_data: &[u8],
    result_code: u32,
) -> Admission {
    let record = &records[version - 1];
    let interface = interface(&record.module);
    let interface_key = programs_state_key(&interface_state_key(program()));
    let tx_hash = hash(transaction);
    let tx_root = hash_from_byte_slices(&[&tx_hash]);
    let prior = prior_chain_for_admission(records, version, tx_root);
    let mut rows = vec![
        (program_key(), state_value(record)),
        (
            interface_key.clone(),
            interface_state_value(
                program(),
                u32::try_from(version).unwrap_or_else(|error| panic!("{error:?}")),
                &interface,
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
        ),
    ];
    let account = &node_producer()["host.account"];
    rows.push((account[..33].to_vec(), account[33..].to_vec()));
    let migration_key = record.migration.as_ref().map(|migration| {
        let accounting = migration
            .chain_accounting()
            .unwrap_or_else(|| panic!("{}", "host migration evidence"));
        let mut key = b"progmig/account/v1/".to_vec();
        key.extend_from_slice(&accounting[61..93]);
        let key = programs_state_key(&key);
        rows.push((key.clone(), accounting.to_vec()));
        key
    });
    let mut fixture = ChainFixture::new("repair-22-31-256", &SEEDS, rows)
        .unwrap_or_else(|error| panic!("{error:?}"));
    fixture.header.height = prior.header.height + 1;
    fixture.header.time.seconds = prior.header.time.seconds + 1;
    if version == 1 {
        let next_transaction = &node_producer()[if records[1].migration.is_some() {
            "hooked"
        } else {
            "upgrade"
        }];
        fixture.header.data_hash = hash_from_byte_slices(&[&hash(next_transaction)]).to_vec();
    }
    fixture
        .header
        .last_block_id
        .hash
        .clone_from(&prior.commit.block_id.hash);
    fixture.header.last_block_id.part_set_total = prior.commit.block_id.part_set_header.total;
    fixture
        .header
        .last_block_id
        .part_set_hash
        .clone_from(&prior.commit.block_id.part_set_header.hash);
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
    let mut proof = admission_state_proof(&fixture, record, &interface_key, tx_root);
    bind_admission_execution(
        &mut proof,
        &fixture,
        &prior,
        &interface_key,
        transaction,
        (result, result_root),
        (record, &migration_key),
    );
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        &fixture.verifier.chain_id,
        fixture.verifier.validators.validators().to_vec(),
        5_000,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let now = u64::from_le_bytes((fixture.header.time.seconds).to_le_bytes()) * 1000
        + u64::from_le_bytes(i64::from(fixture.header.time.nanos).to_le_bytes()) / 1_000_000;
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
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(evidence.record(), &runtime_records()[version - 1]);
    (fixture, evidence)
}

pub fn state_fixture(
    version: usize,
    published: Option<&ProgramInterface>,
) -> (ChainFixture, ChainDeploymentProof) {
    let records = runtime_records();
    let mut rows = vec![(program_key(), state_value(&records[version - 1]))];
    let key = programs_state_key(&interface_state_key(program()));
    if let Some(interface) = published {
        rows.push((
            key.clone(),
            interface_state_value(
                program(),
                u32::try_from(version).unwrap_or_else(|error| panic!("{error:?}")),
                interface,
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
        ));
    }
    let fixture = ChainFixture::new("repair-22-31-256", &SEEDS, rows)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let witness = decode_commitment_proof(
        &fixture
            .encode_key(&key)
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let proof = ChainDeploymentProof::from_state_proofs(
        ChainProgramStateProof {
            program: program(),
            ops: fixture
                .prove(&program_key())
                .unwrap_or_else(|error| panic!("{error:?}")),
        },
        records[version - 1].module.clone(),
        &witness,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    (fixture, proof)
}

fn substitute() -> ProgramInterface {
    let mut entries = descriptor(1).entries().to_vec();
    entries[0].discriminator = [9; 4];
    entries[0].calldata = ValueSchema::layerx(ValueType::Bytes { max_len: 65 });
    ProgramInterface::bind(&node_producer()["wasm.1"], 1, entries)
        .unwrap_or_else(|error| panic!("{error:?}"))
}

pub fn deployment_interface_fixture() {
    for version in [1, 2] {
        let published = descriptor(version);
        let (fixture, proof) = state_fixture(version, Some(&published));
        let head = fixture
            .verifier
            .verify_head(&fixture.header, &fixture.commit)
            .unwrap_or_else(|error| panic!("{error:?}"));
        let evidence = proof
            .verify(&head)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(evidence.interface(), Some(&published));
        assert_eq!(
            evidence.version(),
            u32::try_from(version).unwrap_or_else(|error| panic!("{error:?}"))
        );
        assert_eq!(
            evidence.code_hash(),
            hash(&node_producer()[&format!("wasm.{version}")])
        );
        let original = decode_commitment_proof(
            proof
                .interface
                .as_deref()
                .unwrap_or_else(|| panic!("expected value")),
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        let root = head.app_hash();
        for replacement in [
            None,
            Some(
                interface_state_value(
                    program(),
                    u32::try_from(version).unwrap_or_else(|error| panic!("{error:?}")),
                    &substitute(),
                )
                .unwrap_or_else(|error| panic!("{error:?}")),
            ),
            Some(
                interface_state_value(
                    program(),
                    u32::try_from(3 - version).unwrap_or_else(|error| panic!("{error:?}")),
                    &descriptor(3 - version),
                )
                .unwrap_or_else(|error| panic!("{error:?}")),
            ),
        ] {
            let mut changed = proof.clone();
            let mut witness = original.clone();
            witness.value = replacement;
            changed.interface = Some(witness.encode().unwrap_or_else(|error| panic!("{error:?}")));
            assert!(changed.verify(&head).is_err());
            assert_eq!(head.app_hash(), root);
        }
        let mut omitted = proof.clone();
        omitted.interface = None;
        assert!(omitted.verify(&head).is_err());
        let (absent_fixture, absent) = state_fixture(version, None);
        let absent_head = absent_fixture
            .verifier
            .verify_head(&absent_fixture.header, &absent_fixture.commit)
            .unwrap_or_else(|error| panic!("{error:?}"));
        let absent_evidence = absent
            .verify(&absent_head)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(absent_evidence.interface(), None);
        assert!(!absent_evidence.interface_present());
        let mut hidden = proof.clone();
        hidden.interface.clone_from(&absent.interface);
        assert!(hidden.verify(&head).is_err());
        let mut invented = absent.clone();
        invented.interface.clone_from(&proof.interface);
        assert!(invented.verify(&absent_head).is_err());
    }
    assert_ne!(substitute().digest(), descriptor(1).digest());
    assert_eq!(substitute().code_hash(), descriptor(1).code_hash());
}

fn verify_account_proof_refusals(
    restored_head: &layerx_programs::chain_proof::VerifiedChainHead,
    restored_leaf: &CanonicalAccountLeaf,
    reopened: &layerx_programs::chain_proof::decode::CommitmentProof,
    row: &[u8],
) {
    for change in 0..3 {
        let mut altered = restored_leaf.clone();
        match change {
            0 => altered.balance += 1,
            1 => altered.name.push(b'x'),
            _ => altered.asset_id[0] ^= 1,
        }
        assert_eq!(
            VerifiedAccountSnapshot::verify_chain(restored_head, &altered, &reopened.proof_ops),
            Err(AccountStateError::InvalidAccountLeaf)
        );
    }
    assert!(VerifiedAccountSnapshot::verify_chain(restored_head, restored_leaf, &[]).is_err());
    for index in 0..2 {
        let mut corrupt = reopened.proof_ops.clone();
        corrupt[index].data[0] ^= 1;
        assert!(
            VerifiedAccountSnapshot::verify_chain(restored_head, restored_leaf, &corrupt).is_err()
        );
    }
    let mut wrong_key = reopened.proof_ops.clone();
    wrong_key[0].key[0] ^= 1;
    assert!(
        VerifiedAccountSnapshot::verify_chain(restored_head, restored_leaf, &wrong_key).is_err()
    );
    let mut changed_value = row[33..].to_vec();
    changed_value[40] ^= 1;
    let other = ChainFixture::new(
        "repair-22-31-256",
        &SEEDS,
        vec![(row[..33].to_vec(), changed_value)],
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let other_head = other
        .verifier
        .verify_head(&other.header, &other.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_ne!(other_head.app_hash(), restored_head.app_hash());
    assert!(
        VerifiedAccountSnapshot::verify_chain(&other_head, restored_leaf, &reopened.proof_ops)
            .is_err()
    );
    let legacy_key = programs_state_key(&restored_leaf.primary_key());
    let legacy = ChainFixture::new(
        "legacy-account",
        &SEEDS,
        vec![(
            legacy_key.clone(),
            restored_leaf
                .primary_value()
                .unwrap_or_else(|error| panic!("{error:?}")),
        )],
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let legacy_head = legacy
        .verifier
        .verify_head(&legacy.header, &legacy.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(VerifiedAccountSnapshot::verify_chain(
        &legacy_head,
        restored_leaf,
        &legacy
            .prove(&legacy_key)
            .unwrap_or_else(|error| panic!("{error:?}"))
    )
    .is_err());
}

fn reopen_account_proof(
    temp: &Temp,
    transported: &[u8],
    original: &layerx_programs::VerifiedChainAccountSnapshot,
) -> (
    layerx_programs::chain_proof::VerifiedChainHead,
    CanonicalAccountLeaf,
    layerx_programs::chain_proof::decode::CommitmentProof,
) {
    let trust = TrustRoot::decode(
        &fs::read(temp.0.join("trust")).unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let verifier = ChainProofVerifier {
        chain_id: trust.chain_id,
        validators: trust.validators,
    };
    let bytes = fs::read(temp.0.join("account.lxcp")).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(bytes, transported);
    let reopened = decode_commitment_proof(&bytes).unwrap_or_else(|error| panic!("{error:?}"));
    let header =
        ChainHeader::decode(&reopened.signed_header).unwrap_or_else(|error| panic!("{error:?}"));
    // The commitment codec carries the original signed commit unchanged.
    let commit = decode_commit(&reopened.commit);
    let restored_head = verifier
        .verify_head(&header, &commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(restored_head.header_hash(), trust.header_hash);
    let restored_leaf = CanonicalAccountLeaf::decode_host_record(
        reopened
            .value
            .as_deref()
            .unwrap_or_else(|| panic!("expected value")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let recovered =
        VerifiedAccountSnapshot::verify_chain(&restored_head, &restored_leaf, &reopened.proof_ops)
            .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(&recovered, original);
    (restored_head, restored_leaf, reopened)
}

pub fn account_proof_fixture() {
    let temp = Temp::new();
    let row = &node_producer()["host.account"];
    assert_eq!(row, &node_producer()["host.reloaded"]);
    assert_eq!(row[0], 6);
    let leaf = CanonicalAccountLeaf::decode_host_record(&row[33..])
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(leaf.account_id, row[1..33]);
    assert_eq!(
        leaf.host_record_encoding()
            .unwrap_or_else(|error| panic!("{error:?}")),
        row[33..]
    );
    assert_eq!(leaf.balance, (1u128 << 96) + 42);
    let (input, deployment) = verified(1);
    let head = input
        .fixture
        .verifier
        .verify_head(&input.fixture.header, &input.fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let transported = input
        .fixture
        .encode_key(&row[..33])
        .unwrap_or_else(|error| panic!("{error:?}"));
    let witness = decode_commitment_proof(&transported).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(witness.key, row[..33]);
    assert_eq!(witness.value.as_deref(), Some(&row[33..]));
    let original = VerifiedAccountSnapshot::verify_chain(&head, &leaf, &witness.proof_ops)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(original.account().balance, leaf.balance);
    assert_eq!(original.account().asset_id, leaf.asset_id);
    assert_eq!(original.chain_head().app_hash(), deployment.state_root());
    assert_eq!(original.height(), head.height() - 1);
    fs::write(temp.0.join("account.lxcp"), &transported)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust = TrustRoot::from_head(&head, input.fixture.verifier.validators.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    fs::write(
        temp.0.join("trust"),
        trust
            .canonical_bytes()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    drop(input);
    drop(head);
    let (restored_head, restored_leaf, reopened) =
        reopen_account_proof(&temp, &transported, &original);
    verify_account_proof_refusals(&restored_head, &restored_leaf, &reopened, row);
}

fn decode_commit(bytes: &[u8]) -> layerx_programs::chain_proof::Commit {
    // The same strict production commitment-to-head path supplies the typed commit.
    // Use the producer fixture's framing decoder shared with the Go vector test.
    bootstrap::recorded_commit(bytes).unwrap_or_else(|error| panic!("{error:?}"))
}

#[path = "bootstrap_headers.rs"]
mod bootstrap;

pub fn header_bootstrap_test() {
    bootstrap::bootstrap_header_encoding_go_producer_transport_consumer()
        .unwrap_or_else(|error| panic!("{error:?}"));
    bootstrap::bootstrap_header_encoding_keeps_protocol_refusals()
        .unwrap_or_else(|error| panic!("{error:?}"));
}

fn write_unit(path: &std::path::Path, bytes: &[u8]) {
    use std::io::Write;
    let temporary = path.with_extension("tmp");
    let mut file = fs::File::create_new(&temporary).unwrap_or_else(|error| panic!("{error:?}"));
    file.write_all(bytes)
        .unwrap_or_else(|error| panic!("{error:?}"));
    file.sync_all().unwrap_or_else(|error| panic!("{error:?}"));
    drop(file);
    DeploymentProof::commit_staged_file(&temporary, path)
        .unwrap_or_else(|error| panic!("{error:?}"));
    fs::File::open(path.parent().unwrap_or_else(|| panic!("expected value")))
        .unwrap_or_else(|error| panic!("{error:?}"))
        .sync_all()
        .unwrap_or_else(|error| panic!("{error:?}"));
}

fn restart_records(hooked: bool) -> [DeploymentRecord; 2] {
    let mut records = runtime_records();
    if hooked {
        records[1].migration = Some(
            layerx_programs::MigrationExecutionEvidence::from_chain_accounting(
                &node_producer()["migration.accounting"],
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
        );
    }
    if hooked {
        let mut runtime = Lifecycle::declared().unwrap_or_else(|error| panic!("{error:?}"));
        runtime
            .deploy(Deploy {
                program: program(),
                code_hash: hash(WASM1),
                wasm: WASM1.to_vec(),
                abi_version: 1,
                upgrade_policy: policy(),
            })
            .unwrap_or_else(|error| panic!("{error:?}"));
        let receipt = runtime
            .upgrade(Upgrade {
                program: program(),
                authority: [0x51; 32],
                code_hash: hash(&wasm2()),
                wasm: wasm2(),
                abi_version: 1,
                migration: Some(layerx_programs_runtime::Migration {
                    export: "migrate".into(),
                }),
            })
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(
            records[1]
                .migration
                .as_ref()
                .unwrap_or_else(|| panic!("expected value"))
                .usage(),
            receipt
                .migration()
                .unwrap_or_else(|| panic!("expected value"))
                .usage
        );
        assert_eq!(
            records[1]
                .migration
                .as_ref()
                .unwrap_or_else(|| panic!("expected value"))
                .canonical_bytes(),
            node_producer()["migration.accounting"]
        );
    }
    records
}

fn publish_restart_records(
    temp: &Temp,
    first: Admission,
    second: Admission,
    hooked: bool,
) -> (
    u64,
    [layerx_programs::VerifiedDeploymentEvidence; 2],
    layerx_programs::RegistryEntry,
) {
    let initial = first
        .verifier
        .verify_deployment(&first.proof, first.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let upgraded = second
        .verifier
        .verify_deployment(&second.proof, second.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let execution = DeploymentExecutionProof::decode(&second.proof.state.transaction)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        execution.previous.signed_header,
        first.proof.state.commitment.signed_header
    );
    assert_eq!(
        execution.previous.commit,
        first.proof.state.commitment.commit
    );
    assert_eq!(upgraded.record().sequence, initial.record().sequence + 1);
    assert_eq!(upgraded.record().old_code_hash, Some(initial.code_hash()));
    assert_eq!(upgraded.record().migration.is_some(), hooked);
    let mut live = Registry::new();
    live.record_verified_deployment(&initial)
        .unwrap_or_else(|error| panic!("{error:?}"));
    live.record_verified_upgrade(&initial, &upgraded)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let expected_history = live
        .entry_for_wind_down(program())
        .unwrap_or_else(|error| panic!("{error:?}"))
        .clone();
    for (version, evidence) in [(1, &initial), (2, &upgraded)] {
        evidence
            .record()
            .validate()
            .unwrap_or_else(|error| panic!("{error:?}"));
        let envelope = DeploymentEnvelope::from_evidence(evidence);
        let bytes = envelope
            .canonical_encoding()
            .unwrap_or_else(|error| panic!("{error:?}"));
        write_unit(&temp.0.join(format!("{version}.envelope")), &bytes);
        evidence
            .proof()
            .publish_canonical(&temp.0.join(format!("{version}.admission")))
            .unwrap_or_else(|error| panic!("{error:?}"));
        let entry = envelope
            .journal_entry()
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(entry.transaction_id().bytes(), evidence.receipt_digest());
        assert_ne!(entry.transaction_id().bytes(), evidence.record().digest());
        fs::write(
            temp.0.join(format!("{version}.entry")),
            entry.canonical_encoding(),
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
    }
    let first_head = first
        .fixture
        .verifier
        .verify_head(&first.fixture.header, &first.fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust = TrustRoot::from_head(&first_head, first.fixture.verifier.validators.clone())
        .unwrap_or_else(|error| panic!("{error:?}"));
    fs::write(
        temp.0.join("trust"),
        trust
            .canonical_bytes()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let now = second.now;
    let expected = [initial, upgraded];
    drop(live);
    drop(first);
    drop(second);
    drop(execution);
    drop(first_head);
    drop(trust);

    (now, expected, expected_history)
}

fn verify_restart_replay(
    recovered: &Registry,
    mut entries: Vec<DeploymentJournalEntry>,
    expected: [layerx_programs::VerifiedDeploymentEvidence; 2],
    expected_history: &layerx_programs::RegistryEntry,
) {
    assert_eq!(
        recovered
            .entry_for_wind_down(program())
            .unwrap_or_else(|error| panic!("{error:?}")),
        expected_history
    );
    let mut replayed = Registry::new();
    entries.reverse();
    replayed
        .replay_journal(&entries)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(replayed.latest_version(program()), Ok(2));
    assert_eq!(
        replayed
            .entry_for_wind_down(program())
            .unwrap_or_else(|error| panic!("{error:?}")),
        expected_history
    );
    for evidence in expected {
        assert_eq!(
            replayed
                .resolve_deployment(evidence.clone())
                .unwrap_or_else(|error| panic!("{error:?}")),
            evidence
        );
    }
    let mut gap = Registry::new();
    assert!(gap.replay_journal(&entries[..1]).is_err());
    assert!(replayed.replay_journal(&entries).is_err());
}

type TrustHistoryStep = (
    ChainHeader,
    layerx_programs::chain_proof::Commit,
    layerx_programs::chain_proof::validators::ValidatorSet,
);

fn verify_interface_rejections(
    verifier: &ProtocolDeploymentVerifier,
    proof: &DeploymentProof,
    witness: &layerx_programs::chain_proof::decode::CommitmentProof,
    original: &layerx_programs::chain_proof::VerifiedChainHead,
    current: &layerx_programs::chain_proof::VerifiedChainHead,
    trust_chain: &[TrustHistoryStep],
) {
    let mut invalid = vec![];
    for replacement in [
        None,
        Some(
            interface_state_value(program(), 1, &substitute())
                .unwrap_or_else(|error| panic!("{error:?}")),
        ),
        Some(
            interface_state_value(program(), 2, &descriptor(2))
                .unwrap_or_else(|error| panic!("{error:?}")),
        ),
    ] {
        let mut changed = proof.clone();
        let mut altered = witness.clone();
        altered.value = replacement;
        changed.deployment.interface =
            Some(altered.encode().unwrap_or_else(|error| panic!("{error:?}")));
        invalid.push(changed);
    }
    let mut removed = proof.clone();
    removed.deployment.interface = None;
    invalid.push(removed);
    let mut cross_version = proof.clone();
    cross_version.deployment.interface = verified(2).0.proof.deployment.interface;
    invalid.push(cross_version);
    let (_, absence) = state_fixture(1, None);
    let mut hidden = proof.clone();
    hidden.deployment.interface = absence.interface;
    invalid.push(hidden);
    for changed in invalid {
        let encoded = changed
            .canonical_encoding()
            .unwrap_or_else(|error| panic!("{error:?}"));
        let changed = DeploymentProof::decode(&encoded).unwrap_or_else(|error| panic!("{error:?}"));
        assert!(verifier
            .verify_deployment(
                &changed,
                u64::from_le_bytes((original.time_ms()).to_le_bytes())
            )
            .is_err());
        assert!(verifier
            .verify_historical_deployment(
                &changed,
                current,
                trust_chain,
                u64::from_le_bytes((current.time_ms()).to_le_bytes())
            )
            .is_err());
    }
}

fn recover_restart_records(
    temp: &Temp,
    records: &[DeploymentRecord; 2],
    now: u64,
    expected: &[layerx_programs::VerifiedDeploymentEvidence; 2],
) -> (Registry, Vec<DeploymentJournalEntry>) {
    let trust = TrustRoot::decode(
        &fs::read(temp.0.join("trust")).unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        trust.chain_id,
        trust.validators.validators().to_vec(),
        5000,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let mut recovered = Registry::new();
    let mut entries = Vec::new();
    for (index, expected) in expected.iter().enumerate() {
        let version = index + 1;
        let bytes = fs::read(temp.0.join(format!("{version}.envelope")))
            .unwrap_or_else(|error| panic!("{error:?}"));
        // This is the production hosted journal's shared recovery entrypoint.
        let envelope =
            DeploymentEnvelope::decode(&bytes).unwrap_or_else(|error| panic!("{error:?}"));
        let proof = DeploymentProof::read_canonical(&temp.0.join(format!("{version}.admission")))
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(envelope.proof(), &proof);
        let evidence = verifier
            .verify_deployment(envelope.proof(), now)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(envelope.record(), evidence.record());
        assert_eq!(envelope.receipt_digest(), evidence.receipt_digest());
        assert_eq!(&evidence, expected);
        assert_eq!(evidence.interface(), Some(&descriptor(version)));
        assert_eq!(evidence.record().migration, records[index].migration);
        let entry = DeploymentJournalEntry::decode(
            &fs::read(temp.0.join(format!("{version}.entry")))
                .unwrap_or_else(|error| panic!("{error:?}")),
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(
            entry,
            envelope
                .journal_entry()
                .unwrap_or_else(|error| panic!("{error:?}"))
        );
        entries.push(entry);
        recovered
            .record_verified_deployment(&evidence)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(
            recovered
                .resolve_deployment(evidence)
                .unwrap_or_else(|error| panic!("{error:?}")),
            *expected
        );
        let mut corrupt = bytes.clone();
        let last = corrupt.len() - 1;
        corrupt[last] ^= 1;
        assert!(DeploymentEnvelope::decode(&corrupt).is_err());
        assert!(DeploymentEnvelope::decode(&bytes[..bytes.len() - 1]).is_err());
    }
    (recovered, entries)
}

pub fn restart_recovery(hooked: bool) {
    let temp = Temp::new();
    let records = restart_records(hooked);
    let first = admission_with_records(
        &records,
        1,
        &node_producer()["deploy"],
        &node_producer()["success"],
        0,
    );
    let second = admission_with_records(
        &records,
        2,
        &node_producer()[if hooked { "hooked" } else { "upgrade" }],
        &node_producer()["success"],
        0,
    );
    let (now, expected, expected_history) = publish_restart_records(&temp, first, second, hooked);
    let (recovered, entries) = recover_restart_records(&temp, &records, now, &expected);
    verify_restart_replay(&recovered, entries, expected, &expected_history);
}

pub fn verifier_interface_coverage() {
    let input = verified(1).0;
    let original = input
        .fixture
        .verifier
        .verify_head(&input.fixture.header, &input.fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut chain = input.fixture;
    let old_header = chain.header.clone();
    let old_commit = chain.commit.clone();
    chain.header.height += 1;
    chain.header.time.seconds += 1;
    chain
        .header
        .last_block_id
        .hash
        .clone_from(&old_commit.block_id.hash);
    sign(&mut chain);
    let current = chain
        .verifier
        .verify_head(&chain.header, &chain.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let trust_chain = vec![
        (old_header, old_commit, chain.verifier.validators.clone()),
        (
            chain.header.clone(),
            chain.commit.clone(),
            chain.verifier.validators.clone(),
        ),
    ];
    let canonical = input
        .proof
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let proof = DeploymentProof::decode(&canonical).unwrap_or_else(|error| panic!("{error:?}"));
    let accepted = input
        .verifier
        .verify_deployment(&proof, input.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        input
            .verifier
            .verify_historical_deployment(
                &proof,
                &current,
                &trust_chain,
                u64::from_le_bytes((current.time_ms()).to_le_bytes())
            )
            .unwrap_or_else(|error| panic!("{error:?}")),
        accepted
    );
    assert_eq!(accepted.interface(), Some(&descriptor(1)));
    let witness = decode_commitment_proof(
        proof
            .deployment
            .interface
            .as_deref()
            .unwrap_or_else(|| panic!("expected value")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    verify_interface_rejections(
        &input.verifier,
        &proof,
        &witness,
        &original,
        &current,
        &trust_chain,
    );
}

fn verify_native_interface_refusals(
    version: usize,
    key: &str,
    input: &Admission,
    decoded: &DeploymentProof,
    execution: &DeploymentExecutionProof,
) {
    for (data, code) in [
        (&node_producer()["failed"], 0),
        (&node_producer()["success"], 9),
    ] {
        let refused = admission(version, &node_producer()[key], data, code);
        assert!(refused
            .verifier
            .verify_deployment(&refused.proof, refused.now)
            .is_err());
    }
    for field in 0..6 {
        let mut changed = decoded.clone();
        let mut altered = execution.clone();
        match field {
            0 => altered.result.code = 9,
            1 => altered.result.data.clone_from(&node_producer()["failed"]),
            2 => altered.result_proof.index += 1,
            3 => changed.state.transaction_proof.index += 1,
            4 => altered.previous.signed_header[0] ^= 1,
            _ => altered.interface_ops[0].data[0] ^= 1,
        }
        changed
            .bind_execution(&altered)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert!(
            input
                .verifier
                .verify_deployment(&changed, input.now)
                .is_err(),
            "mutation {field}"
        );
    }
}

fn verify_native_interface_version(version: usize) {
    let (input, expected) = verified(version);
    let bytes = input
        .proof
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let decoded = DeploymentProof::decode(&bytes).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(decoded, input.proof);
    let execution = DeploymentExecutionProof::decode(&decoded.state.transaction)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let key = if version == 1 { "deploy" } else { "upgrade" };
    assert_eq!(execution.transaction, node_producer()[key]);
    assert_eq!(execution.result.data, node_producer()["success"]);
    assert_eq!(execution.result.code, 0);
    assert_eq!(
        execution.result_proof.index,
        decoded.state.transaction_proof.index
    );
    assert_eq!(
        execution.result_proof.total,
        decoded.state.transaction_proof.total
    );
    let previous = ChainHeader::decode(&execution.previous.signed_header)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let header = ChainHeader::decode(&decoded.state.commitment.signed_header)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(previous.height + 1, header.height);
    assert_eq!(
        header.last_block_id.hash,
        previous.hash().unwrap_or_else(|error| panic!("{error:?}"))
    );
    assert_eq!(header.last_results_hash, node_producer()["success.root"]);
    let evidence = input
        .verifier
        .verify_deployment(&decoded, input.now)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(evidence, expected);
    let witness = decode_commitment_proof(
        decoded
            .deployment
            .interface
            .as_deref()
            .unwrap_or_else(|| panic!("expected value")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(witness.proof_ops, execution.interface_ops);
    let descriptor_bytes = &node_producer()[&format!("interface.{version}")];
    assert_eq!(
        evidence
            .interface()
            .unwrap_or_else(|| panic!("expected value"))
            .canonical_encoding(),
        descriptor_bytes
    );
    assert_eq!(
        &witness
            .value
            .as_ref()
            .unwrap_or_else(|| panic!("expected value"))[72..],
        descriptor_bytes
    );
    let head = input
        .fixture
        .verifier
        .verify_head(&header, &input.fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut ops = witness.proof_ops.clone();
    ops.extend(decoded.state.state.ops.clone());
    let read = VerifiedInterfaceRead::verify_at_head(
        &head,
        program(),
        &InterfaceStateWitness {
            key: interface_state_key(program()),
            value: witness
                .value
                .clone()
                .unwrap_or_else(|| panic!("expected value")),
            proof: layerx_programs::StateProof {
                leaf_index: 0,
                leaf_count: 0,
                siblings: vec![],
            },
        },
        &ops,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        read.interface,
        *evidence
            .interface()
            .unwrap_or_else(|| panic!("expected value"))
    );
    assert_eq!(read.state_root, evidence.state_root());
    assert_eq!(
        read.version,
        u32::try_from(version).unwrap_or_else(|error| panic!("{error:?}"))
    );
    verify_native_interface_refusals(version, key, &input, &decoded, &execution);
}

pub fn native_interface_transport() {
    for version in [1, 2] {
        verify_native_interface_version(version);
    }
}
