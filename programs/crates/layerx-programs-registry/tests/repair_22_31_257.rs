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
        decode::{decode_commitment_proof, CommitmentProof},
        merkle::{hash_from_byte_slices, SimpleProof},
        results::TxResult,
        validators::TrustedValidator,
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
    },
    deployment_envelope::DeploymentEnvelope,
    interface_state_key, interface_state_value, DeploymentExecutionProof, DeploymentProof,
    DeploymentRecord, InterfaceCapability, ProgramId, ProgramInterface, ProgramLifecycleProof,
    ProgramStateProof, ProtocolDeploymentVerifier, Registry, UpgradePolicy, ValueSchema, ValueType,
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
    ProgramId::new([0x31; 32]).unwrap()
}
fn policy() -> UpgradePolicy {
    UpgradePolicy::Authority([0x51; 32])
}
fn transcript() -> BTreeMap<String, Vec<u8>> {
    let text = include_str!("../src/repair_22_31_256.hex");
    assert_eq!(
        layerx_programs::hex::encode(&hash(text.as_bytes())),
        "5798c9304ce5dd774f153da4f4f3208f1205f4cbfd1854732a49636dc9d76c6e"
    );
    text.lines()
        .map(|line| {
            let (key, value) = line.split_once('=').unwrap();
            (key.into(), layerx_programs::hex::decode(value).unwrap())
        })
        .collect()
}
fn descriptor(version: usize) -> ProgramInterface {
    ProgramInterface::decode(&transcript()[&format!("interface.{version}")]).unwrap()
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
fn records() -> [DeploymentRecord; 2] {
    let mut runtime = Lifecycle::declared().unwrap();
    let first = runtime
        .deploy(Deploy {
            program: program(),
            code_hash: hash(&code(1)),
            wasm: code(1),
            abi_version: 1,
            upgrade_policy: policy(),
        })
        .unwrap();
    let second = runtime
        .upgrade(Upgrade {
            program: program(),
            authority: [0x51; 32],
            code_hash: hash(&code(2)),
            wasm: code(2),
            abi_version: 1,
            migration: None,
        })
        .unwrap();
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
                    abi_version: 1,
                },
                policy(),
                39 + i as u64,
                1_700_000_001_123 + i as u64 * 1000,
            )
            .unwrap()
        })
        .collect::<Vec<_>>()
        .try_into()
        .unwrap()
}
fn program_value(version: usize) -> Vec<u8> {
    record_value(&records()[version - 1])
}
fn record_value(record: &DeploymentRecord) -> Vec<u8> {
    let mut value = vec![1];
    value.extend_from_slice(&[0x51; 32]);
    value.extend_from_slice(&record.new_code_hash);
    value.extend_from_slice(&record.abi_version.to_be_bytes());
    value.extend_from_slice(&record.version.to_be_bytes());
    value
}
fn capability_fixture() -> (ChainFixture, ChainDeploymentProof, ProgramInterface) {
    use layerx_programs_runtime::test_support::*;
    let mut exports = unsigned_leb(3);
    for (name, kind, index) in [("layerx_reserve", 0, 1), ("call", 0, 2), ("memory", 2, 0)] {
        exports.extend(unsigned_leb(name.len() as u64));
        exports.extend_from_slice(name.as_bytes());
        exports.push(kind);
        exports.extend(unsigned_leb(index));
    }
    let wasm = module(&[
        type_section(&[
            (&[TYPE_I32; 4], &[TYPE_I32]),
            (&[TYPE_I32], &[TYPE_I32]),
            (&[TYPE_I32; 2], &[TYPE_I32]),
        ]),
        import_section(&[("layerx_v1", "receipt_read", 0)]),
        function_section(&[1, 2]),
        raw_section(5, &[1, 0, 1]),
        raw_section(7, &exports),
        code_section(&[
            func_body(&[], &[OP_I32_CONST, 0, OP_END]),
            func_body(
                &[],
                &[
                    OP_I32_CONST,
                    0,
                    OP_I32_CONST,
                    0,
                    OP_I32_CONST,
                    0,
                    OP_I32_CONST,
                    0,
                    OP_CALL,
                    0,
                    OP_DROP,
                    OP_I32_CONST,
                    0,
                    OP_END,
                ],
            ),
        ]),
    ]);
    let mut entries = descriptor(1).entries().to_vec();
    entries[0].capabilities = vec![InterfaceCapability::ReceiptRead {
        receipt_digest: [1; 32],
    }];
    let published = ProgramInterface::bind(&wasm, 1, entries).unwrap();
    let mut lifecycle = Lifecycle::declared().unwrap();
    let receipt = lifecycle
        .deploy(Deploy {
            program: program(),
            code_hash: hash(&wasm),
            wasm: wasm.clone(),
            abi_version: 1,
            upgrade_policy: policy(),
        })
        .unwrap();
    let record = DeploymentRecord::from_deployment(
        &receipt,
        &ProgramVersion {
            code_hash: hash(&wasm),
            wasm: wasm.clone(),
            abi_version: 1,
        },
        policy(),
        38,
        1_700_000_000_123,
    )
    .unwrap();
    let chain = ChainFixture::new(
        CHAIN,
        &SEEDS,
        vec![
            (program_key(), record_value(&record)),
            (
                interface_key(),
                interface_state_value(program(), 1, &published).unwrap(),
            ),
        ],
    )
    .unwrap();
    let interface_state =
        decode_commitment_proof(&chain.encode_key(&interface_key()).unwrap()).unwrap();
    let proof = ChainDeploymentProof::from_state_proofs(
        ChainProgramStateProof {
            program: program(),
            ops: chain.prove(&program_key()).unwrap(),
        },
        wasm,
        &interface_state,
    )
    .unwrap();
    (chain, proof, published)
}
fn state_fixture(version: usize, value: Option<Vec<u8>>) -> (ChainFixture, ChainDeploymentProof) {
    let mut rows = vec![(program_key(), program_value(version))];
    if let Some(value) = value {
        rows.push((interface_key(), value));
    }
    let chain = ChainFixture::new(CHAIN, &SEEDS, rows).unwrap();
    let witness = decode_commitment_proof(&chain.encode_key(&interface_key()).unwrap()).unwrap();
    let deployment = ChainDeploymentProof::from_state_proofs(
        ChainProgramStateProof {
            program: program(),
            ops: chain.prove(&program_key()).unwrap(),
        },
        code(version),
        &witness,
    )
    .unwrap();
    (chain, deployment)
}
fn witness(proof: &ChainDeploymentProof) -> CommitmentProof {
    decode_commitment_proof(proof.interface.as_deref().unwrap()).unwrap()
}
fn set_witness(proof: &mut ChainDeploymentProof, witness: &CommitmentProof) {
    proof.interface = Some(witness.encode().unwrap());
}
fn value(version: usize) -> Vec<u8> {
    interface_state_value(program(), version as u32, &descriptor(version)).unwrap()
}
fn alternate() -> ProgramInterface {
    let mut entries = descriptor(1).entries().to_vec();
    entries[0].discriminator = [9; 4];
    entries[0].calldata = ValueSchema::layerx(ValueType::Bytes { max_len: 65 });
    ProgramInterface::bind(&code(1), 1, entries).unwrap()
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

struct Admission {
    chain: ChainFixture,
    proof: DeploymentProof,
    verifier: ProtocolDeploymentVerifier,
    now: u64,
}
fn admission(version: usize) -> Admission {
    let producer = transcript();
    let transaction = &producer[if version == 1 { "deploy" } else { "upgrade" }];
    let prior_rows = if version == 1 {
        vec![(b"unrelated".to_vec(), vec![1])]
    } else {
        vec![(program_key(), program_value(1))]
    };
    let mut prior = ChainFixture::new(CHAIN, &SEEDS, prior_rows).unwrap();
    prior.header.height = 37 + version as i64;
    prior.header.time.seconds += version as i64 - 1;
    let tx_root = hash_from_byte_slices(&[&hash(transaction)]);
    prior.header.data_hash = tx_root.to_vec();
    sign(&mut prior);
    let (mut chain, _) = state_fixture(version, Some(value(version)));
    chain.header.height = prior.header.height + 1;
    chain.header.time.seconds = prior.header.time.seconds + 1;
    chain.header.last_block_id.hash = prior.commit.block_id.hash.clone();
    chain.header.last_block_id.part_set_total = prior.commit.block_id.part_set_header.total;
    chain.header.last_block_id.part_set_hash = prior.commit.block_id.part_set_header.hash.clone();
    let result_root = hash_from_byte_slices(&[&producer["success.leaf"]]);
    assert_eq!(result_root.as_slice(), producer["success.root"]);
    chain.header.last_results_hash = result_root.to_vec();
    sign(&mut chain);
    let commitment = decode_commitment_proof(&chain.encode_key(&program_key()).unwrap()).unwrap();
    let interface_state =
        decode_commitment_proof(&chain.encode_key(&interface_key()).unwrap()).unwrap();
    let state = ChainProgramStateProof {
        program: program(),
        ops: chain.prove(&program_key()).unwrap(),
    };
    let mut status_key = b"wind-down\0s".to_vec();
    status_key.extend_from_slice(&program().bytes());
    let status_ops = chain
        .prove_absent(&programs_state_key(&status_key))
        .unwrap();
    let deployment =
        ChainDeploymentProof::from_state_proofs(state.clone(), code(version), &interface_state)
            .unwrap();
    let mut proof = DeploymentProof {
        state: ProgramStateProof {
            commitment,
            state: state.clone(),
            lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                program: program(),
                record: Some(program_value(version)),
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
            previous: decode_commitment_proof(&prior.encode_key(&program_key()).unwrap()).unwrap(),
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
            interface_ops: chain.prove(&interface_key()).unwrap(),
            migration: None,
            migration_ops: vec![],
            migration_previous_ops: vec![],
        })
        .unwrap();
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN,
        chain.verifier.validators.validators().to_vec(),
        5000,
    )
    .unwrap();
    let now = chain.header.time.seconds as u64 * 1000 + chain.header.time.nanos as u64 / 1_000_000;
    Admission {
        chain,
        proof,
        verifier,
        now,
    }
}

#[test]
fn deployment_interface_authentication_presence() {
    let (chain, proof) = state_fixture(1, Some(value(1)));
    let head = chain
        .verifier
        .verify_head(&chain.header, &chain.commit)
        .unwrap();
    let verified = proof.verify(&head).unwrap();
    assert_eq!(verified.interface(), Some(&descriptor(1)));
    assert_eq!(
        verified.interface().unwrap().digest(),
        descriptor(1).digest()
    );
    assert_eq!(verified.program(), program());
    assert_eq!(verified.version(), 1);
    assert_eq!(verified.code_hash(), hash(&code(1)));
    assert_eq!(verified.state_root(), head.app_hash());
    assert_eq!(verified.state_height(), head.height() - 1);
    for interface in [
        None,
        Some(descriptor(1).canonical_encoding().to_vec()),
        Some(alternate().canonical_encoding().to_vec()),
    ] {
        let mut changed = proof.clone();
        changed.interface = interface;
        assert!(changed.verify(&head).is_err());
    }
    let committed = witness(&proof);
    let mut replacements = vec![None];
    for field in 0..4 {
        let (candidate_chain, candidate_proof, original) = if field == 3 {
            capability_fixture()
        } else {
            let (chain, proof) = state_fixture(1, Some(value(1)));
            (chain, proof, descriptor(1))
        };
        let candidate_head = candidate_chain
            .verifier
            .verify_head(&candidate_chain.header, &candidate_chain.commit)
            .unwrap();
        assert_eq!(
            candidate_proof.verify(&candidate_head).unwrap().interface(),
            Some(&original)
        );
        let mut entries = original.entries().to_vec();
        match field {
            0 => entries[0].discriminator = [9; 4],
            1 => entries[0].calldata = ValueSchema::layerx(ValueType::Bytes { max_len: 65 }),
            2 => entries[0].response = ValueSchema::layerx(ValueType::Bytes { max_len: 65 }),
            _ => {
                entries[0].capabilities = vec![InterfaceCapability::ReceiptRead {
                    receipt_digest: [2; 32],
                }]
            }
        }
        let substitute = ProgramInterface::bind(&candidate_proof.code, 1, entries).unwrap();
        assert_ne!(substitute.digest(), original.digest());
        let mut changed = candidate_proof.clone();
        let mut substituted_witness = witness(&changed);
        substituted_witness.value = Some(interface_state_value(program(), 1, &substitute).unwrap());
        set_witness(&mut changed, &substituted_witness);
        assert!(changed.verify(&candidate_head).is_err());
        replacements.push(Some(
            interface_state_value(program(), 1, &substitute).unwrap(),
        ));
    }
    for replacement in replacements {
        let mut changed = proof.clone();
        let mut altered = committed.clone();
        altered.value = replacement;
        set_witness(&mut changed, &altered);
        assert!(changed.verify(&head).is_err());
    }
    let mut changed = proof;
    changed.code = code(2);
    assert!(changed.verify(&head).is_err());
}

#[test]
fn deployment_interface_authentication_absence() {
    let (chain, proof) = state_fixture(1, None);
    let head = chain
        .verifier
        .verify_head(&chain.header, &chain.commit)
        .unwrap();
    let verified = proof.verify(&head).unwrap();
    assert!(!verified.interface_present());
    assert_eq!(verified.interface(), None);
    let mut changed = proof.clone();
    changed.interface = None;
    assert!(changed.verify(&head).is_err());
    let mut altered = witness(&proof);
    altered.value = Some(value(1));
    set_witness(&mut changed, &altered);
    assert!(changed.verify(&head).is_err());
    let (present, presence) = state_fixture(1, Some(value(1)));
    changed.interface = presence.interface.clone();
    assert!(changed.verify(&head).is_err());
    let present_head = present
        .verifier
        .verify_head(&present.header, &present.commit)
        .unwrap();
    let mut hidden = presence;
    hidden.interface = proof.interface;
    assert!(hidden.verify(&present_head).is_err());
}

#[test]
fn deployment_interface_authentication_record_binding() {
    let good = value(1);
    let mut invalid_values = vec![
        interface_state_value(ProgramId::new([0x32; 32]).unwrap(), 1, &descriptor(1)).unwrap(),
        interface_state_value(program(), 2, &descriptor(1)).unwrap(),
        interface_state_value(program(), 1, &descriptor(2)).unwrap(),
    ];
    let abi = ProgramInterface::bind(&code(1), 2, descriptor(1).entries().to_vec()).unwrap();
    invalid_values.push(interface_state_value(program(), 1, &abi).unwrap());
    for offset in [36, 68] {
        let mut changed = good.clone();
        changed[offset] ^= 1;
        invalid_values.push(changed);
    }
    let mut trailing = good.clone();
    trailing.push(0);
    invalid_values.push(trailing);
    invalid_values.push(good[..71].to_vec());
    for committed in invalid_values {
        let (chain, proof) = state_fixture(1, Some(committed));
        let head = chain
            .verifier
            .verify_head(&chain.header, &chain.commit)
            .unwrap();
        // These are valid membership proofs; rejection must come from record binding.
        let witness = witness(&proof);
        layerx_programs::verify_state(
            &head,
            "lxphost",
            &interface_key(),
            witness.value.as_deref().unwrap(),
            &witness.proof_ops,
        )
        .unwrap();
        assert!(proof.verify(&head).is_err());
    }
}

#[test]
fn deployment_interface_authentication_proof_binding() {
    let (chain, proof) = state_fixture(1, Some(value(1)));
    let head = chain
        .verifier
        .verify_head(&chain.header, &chain.commit)
        .unwrap();
    for index in 0..2 {
        for field in 0..3 {
            let mut changed = proof.clone();
            let mut altered = witness(&proof);
            match field {
                0 => altered.proof_ops[index].data[0] ^= 1,
                1 => altered.proof_ops[index].key.push(0),
                _ => altered.proof_ops[index].op_type.push('x'),
            }
            set_witness(&mut changed, &altered);
            assert!(changed.verify(&head).is_err());
        }
    }
    for count in 0..2 {
        let mut changed = proof.clone();
        let mut altered = witness(&proof);
        altered.proof_ops.truncate(count);
        set_witness(&mut changed, &altered);
        assert!(changed.verify(&head).is_err());
    }
    let mut changed = proof.clone();
    let mut altered = witness(&proof);
    altered.store_name = "bank".into();
    set_witness(&mut changed, &altered);
    assert!(changed.verify(&head).is_err());
    altered = witness(&proof);
    altered.key[0] ^= 1;
    set_witness(&mut changed, &altered);
    assert!(changed.verify(&head).is_err());
    let (other, mut other_proof) = state_fixture(
        1,
        Some(interface_state_value(program(), 1, &alternate()).unwrap()),
    );
    assert_ne!(chain.header.app_hash, other.header.app_hash);
    other_proof.interface = proof.interface.clone();
    let other_head = other
        .verifier
        .verify_head(&other.header, &other.commit)
        .unwrap();
    assert!(other_proof.verify(&other_head).is_err());
    let mut same_root = chain;
    same_root.header.height += 1;
    same_root.header.time.seconds += 1;
    sign(&mut same_root);
    let new_head = same_root
        .verifier
        .verify_head(&same_root.header, &same_root.commit)
        .unwrap();
    assert_eq!(head.app_hash(), new_head.app_hash());
    assert!(proof.verify(&new_head).is_err());
    let mut trailing = proof;
    trailing.interface.as_mut().unwrap().push(0);
    assert!(trailing.verify(&head).is_err());
}

struct Temp(PathBuf);
impl Temp {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-31-257-{}-{}",
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

#[test]
fn deployment_interface_authentication_transport_recovery() {
    let temp = Temp::new();
    let mut live = Registry::new();
    let mut expected = Vec::new();
    for version in [1, 2] {
        let input = admission(version);
        let transported =
            DeploymentProof::decode(&input.proof.canonical_encoding().unwrap()).unwrap();
        assert_eq!(transported, input.proof);
        let evidence = input
            .verifier
            .verify_deployment(&transported, input.now)
            .unwrap();
        assert_eq!(evidence.record(), &records()[version - 1]);
        assert_eq!(evidence.interface(), Some(&descriptor(version)));
        assert_eq!(
            evidence.receipt_digest(),
            hash(&transcript()[if version == 1 { "deploy" } else { "upgrade" }])
        );
        live.record_verified_deployment(&evidence).unwrap();
        input
            .proof
            .publish_canonical(&temp.0.join(format!("{version}.admission")))
            .unwrap();
        fs::write(
            temp.0.join(format!("{version}.envelope")),
            DeploymentEnvelope::from_evidence(&evidence)
                .canonical_encoding()
                .unwrap(),
        )
        .unwrap();
        expected.push(evidence);
    }
    assert_eq!(live.latest_version(program()), Ok(2));
    drop(live);
    let mut recovered = Registry::new();
    for (i, expected) in expected.into_iter().enumerate() {
        let version = i + 1;
        let envelope = DeploymentEnvelope::decode(
            &fs::read(temp.0.join(format!("{version}.envelope"))).unwrap(),
        )
        .unwrap();
        let proof =
            DeploymentProof::read_canonical(&temp.0.join(format!("{version}.admission"))).unwrap();
        assert_eq!(envelope.proof(), &proof);
        let fresh = admission(version);
        let evidence = fresh
            .verifier
            .verify_deployment(envelope.proof(), fresh.now)
            .unwrap();
        assert_eq!(evidence, expected);
        assert_eq!(envelope.record(), evidence.record());
        assert_eq!(envelope.receipt_digest(), evidence.receipt_digest());
        assert_eq!(
            evidence.interface().unwrap().digest(),
            descriptor(version).digest()
        );
        recovered.record_verified_deployment(&evidence).unwrap();
        assert_eq!(
            recovered.resolve_deployment(evidence.clone()).unwrap(),
            expected
        );
        let mut changed = proof;
        changed.deployment.interface = None;
        assert!(fresh
            .verifier
            .verify_deployment(&changed, fresh.now)
            .is_err());
    }
    assert_eq!(recovered.latest_version(program()), Ok(2));
}

#[test]
fn deployment_interface_authentication_current_historical() {
    let input = admission(1);
    let old = input
        .chain
        .verifier
        .verify_head(&input.chain.header, &input.chain.commit)
        .unwrap();
    let mut chain = input.chain;
    let old_header = chain.header.clone();
    let old_commit = chain.commit.clone();
    chain.header.height += 1;
    chain.header.time.seconds += 1;
    chain.header.last_block_id.hash = old_commit.block_id.hash.clone();
    sign(&mut chain);
    let current = chain
        .verifier
        .verify_head(&chain.header, &chain.commit)
        .unwrap();
    let trust = vec![
        (old_header, old_commit, chain.verifier.validators.clone()),
        (
            chain.header.clone(),
            chain.commit.clone(),
            chain.verifier.validators.clone(),
        ),
    ];
    let now = current.time_ms() as u64;
    let current_evidence = input
        .verifier
        .verify_deployment(&input.proof, input.now)
        .unwrap();
    assert_eq!(
        input
            .verifier
            .verify_historical_deployment(&input.proof, &current, &trust, now)
            .unwrap(),
        current_evidence
    );
    assert_eq!(current_evidence.interface(), Some(&descriptor(1)));
    let mut invalid = Vec::new();
    let mut hidden = input.proof.clone();
    hidden.deployment.interface = None;
    invalid.push(hidden);
    let mut substituted = input.proof.clone();
    let mut altered = witness(&substituted.deployment);
    altered.value = Some(interface_state_value(program(), 1, &alternate()).unwrap());
    set_witness(&mut substituted.deployment, &altered);
    invalid.push(substituted);
    let mut cross_version = input.proof.clone();
    cross_version.deployment.interface = admission(2).proof.deployment.interface;
    invalid.push(cross_version);
    for changed in invalid {
        let transported = DeploymentProof::decode(&changed.canonical_encoding().unwrap()).unwrap();
        assert!(input
            .verifier
            .verify_deployment(&transported, old.time_ms() as u64)
            .is_err());
        assert!(input
            .verifier
            .verify_historical_deployment(&transported, &current, &trust, now)
            .is_err());
    }
}
