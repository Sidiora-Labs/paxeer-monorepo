use super::chain::ChainFixture;
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
        vote::{vote_sign_bytes, Timestamp},
    },
    interface_state_key, interface_state_value, DeploymentExecutionProof, DeploymentProof,
    ProgramId, ProgramInterface, ProgramLifecycleProof, ProgramStateProof, StateLeafWitness,
    UpgradePolicy,
};
use sha2::{Digest, Sha256};
use std::{
    fs,
    io::Write,
    path::PathBuf,
    process::{Command, Stdio},
    sync::{
        atomic::{AtomicU64, Ordering},
        OnceLock,
    },
};

pub fn raw_transaction(proof: &DeploymentProof) -> Vec<u8> {
    DeploymentExecutionProof::decode(&proof.state.transaction)
        .expect("deployment execution transport")
        .transaction
}

pub fn sign(chain: &mut ChainFixture, seeds: &[[u8; 32]]) {
    chain.commit.height = chain.header.height;
    chain.commit.block_id.hash = chain.header.hash().expect("header hash").to_vec();
    for signature in &mut chain.commit.signatures {
        let signer = seeds
            .iter()
            .map(SigningKey::from_bytes)
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .expect("validator signer");
        signature.timestamp = Timestamp {
            seconds: chain.header.time.seconds,
            nanos: chain.header.time.nanos,
        };
        let bytes = vote_sign_bytes(
            &chain.header.chain_id,
            chain.commit.height,
            chain.commit.round,
            Some(&chain.commit.block_id),
            signature.timestamp,
        )
        .expect("vote bytes");
        signature.signature = signer.sign(&bytes).to_bytes().to_vec();
    }
}

pub fn transaction(
    program: ProgramId,
    abi: u16,
    policy: UpgradePolicy,
    code: &[u8],
    interface: Option<&ProgramInterface>,
    old_hash: Option<[u8; 32]>,
) -> Vec<u8> {
    static PRODUCER: OnceLock<(PathBuf, [u8; 32])> = OnceLock::new();
    static NEXT: AtomicU64 = AtomicU64::new(0);
    let (producer, binary_hash) = PRODUCER.get_or_init(|| {
        let path = std::env::var_os("LAYERX_REGISTRY_TRANSACTION_PRODUCER")
            .map(PathBuf::from)
            .unwrap_or_else(|| {
                PathBuf::from(env!("CARGO_MANIFEST_DIR"))
                    .join("../../../build/tests/programs_registry_transactions")
            });
        let bytes = fs::read(&path)
            .expect("programs-registry-fixtures must build the node transaction producer");
        (path, Sha256::digest(bytes).into())
    });
    let (flag, authority) = match policy {
        UpgradePolicy::Immutable => (0, [0; 32]),
        UpgradePolicy::Authority(a) => (1, a),
    };
    let request = format!(
        "{}\n{abi}\n{flag}\n{}\n{}\n{}\n{}\n",
        layerx_programs::hex::encode(&program.bytes()),
        layerx_programs::hex::encode(&authority),
        layerx_programs::hex::encode(code),
        interface
            .map(|i| layerx_programs::hex::encode(i.canonical_encoding()))
            .unwrap_or_default(),
        old_hash
            .map(|h| layerx_programs::hex::encode(&h))
            .unwrap_or_else(|| "-".to_owned())
    );
    let mut hasher = Sha256::new();
    hasher.update(binary_hash);
    hasher.update(request.as_bytes());
    let digest: [u8; 32] = hasher.finalize().into();
    let directory = producer
        .parent()
        .expect("producer directory")
        .join("programs_registry_transactions_data");
    fs::create_dir_all(&directory).expect("transaction artifact directory");
    let output = directory.join(format!("{}.hex", layerx_programs::hex::encode(&digest)));
    if !output.exists() {
        let mut child = Command::new(producer)
            .current_dir(PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../.."))
            .arg("transaction")
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .expect("node transaction producer");
        child
            .stdin
            .take()
            .expect("producer input")
            .write_all(request.as_bytes())
            .expect("transaction request");
        let result = child
            .wait_with_output()
            .expect("transaction producer result");
        assert!(
            result.status.success(),
            "node transaction producer: {}",
            String::from_utf8_lossy(&result.stderr)
        );
        let temporary = directory.join(format!(
            "{}.{}.tmp",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::write(&temporary, &result.stdout).expect("transaction artifact");
        match fs::hard_link(&temporary, &output) {
            Ok(()) => {}
            Err(e) if e.kind() == std::io::ErrorKind::AlreadyExists => {}
            Err(e) => panic!("publish transaction: {e}"),
        }
        fs::remove_file(temporary).expect("remove transaction staging file");
    }
    layerx_programs::hex::decode(
        fs::read_to_string(output)
            .expect("signed transaction artifact")
            .trim(),
    )
    .expect("signed transaction encoding")
}

pub struct Parameters<'a> {
    pub program: ProgramId,
    pub version: u32,
    pub policy: UpgradePolicy,
    pub code: &'a [u8],
    pub interface: Option<&'a ProgramInterface>,
    pub chain_id: &'a str,
    pub seeds: &'a [[u8; 32]],
    pub height: i64,
    pub timestamp: u64,
    pub old_hash: Option<[u8; 32]>,
    pub status: Option<Vec<u8>>,
    pub raw: Option<&'a [u8]>,
    pub execution: bool,
}

pub fn fixture(p: Parameters<'_>) -> (ChainFixture, DeploymentProof, StateLeafWitness) {
    let abi = p.interface.map_or(1, ProgramInterface::abi_version);
    fixture_with_abi(p, abi)
}

pub fn fixture_with_abi(
    p: Parameters<'_>,
    abi: u16,
) -> (ChainFixture, DeploymentProof, StateLeafWitness) {
    let mut program_key = b"program\0".to_vec();
    program_key.extend_from_slice(&p.program.bytes());
    let program_key = programs_state_key(&program_key);
    let interface_key = interface_state_key(p.program);
    let host_interface_key = programs_state_key(&interface_key);
    let mut status_key = b"wind-down\0s".to_vec();
    status_key.extend_from_slice(&p.program.bytes());
    let status_key = programs_state_key(&status_key);
    let (flag, authority) = match p.policy {
        UpgradePolicy::Immutable => (0, [0; 32]),
        UpgradePolicy::Authority(a) => (1, a),
    };
    let mut record = vec![flag];
    record.extend_from_slice(&authority);
    record.extend_from_slice(&Sha256::digest(p.code));
    record.extend_from_slice(&abi.to_be_bytes());
    record.extend_from_slice(&p.version.to_be_bytes());
    let interface_value = p
        .interface
        .map(|i| interface_state_value(p.program, p.version, i).expect("interface state"));
    let mut rows = vec![(program_key.clone(), record.clone())];
    if let Some(value) = &interface_value {
        rows.push((host_interface_key.clone(), value.clone()));
    }
    if let Some(status) = &p.status {
        rows.push((status_key.clone(), status.clone()));
    }
    let raw = p
        .raw
        .map(Vec::from)
        .unwrap_or_else(|| transaction(p.program, abi, p.policy, p.code, p.interface, p.old_hash));
    let tx_hash: [u8; 32] = Sha256::digest(&raw).into();
    let tx_root = hash_from_byte_slices(&[&tx_hash]);
    let prior_rows = match p.old_hash {
        None => vec![(b"unrelated".to_vec(), vec![1])],
        Some(hash) => {
            let mut previous = record.clone();
            previous[33..65].copy_from_slice(&hash);
            previous[67..71].copy_from_slice(&(p.version - 1).to_be_bytes());
            vec![(program_key.clone(), previous)]
        }
    };
    let mut prior = ChainFixture::new(p.chain_id, p.seeds, prior_rows).expect("predecessor state");
    prior.header.height = p.height - 1;
    prior.header.time.seconds = i64::try_from(p.timestamp / 1000).expect("timestamp") - 1;
    prior.header.time.nanos =
        i32::try_from(p.timestamp % 1000 * 1_000_000).expect("timestamp nanos");
    prior.header.data_hash = tx_root.to_vec();
    sign(&mut prior, p.seeds);
    let mut chain = ChainFixture::new(p.chain_id, p.seeds, rows).expect("program state");
    chain.header.height = p.height;
    chain.header.time.seconds = prior.header.time.seconds + 1;
    chain.header.time.nanos = prior.header.time.nanos;
    chain.header.data_hash = tx_root.to_vec();
    chain.header.last_block_id.hash = prior.commit.block_id.hash.clone();
    chain.header.last_block_id.part_set_total = prior.commit.block_id.part_set_header.total;
    chain.header.last_block_id.part_set_hash = prior.commit.block_id.part_set_header.hash.clone();
    let transcript = include_str!("../../src/deployment_transcripts.hex");
    let value = |name: &str| {
        let (_, hex) = transcript
            .lines()
            .filter_map(|l| l.split_once('='))
            .find(|(key, _)| *key == name)
            .expect("node result transcript");
        layerx_programs::hex::decode(hex).expect("node result bytes")
    };
    let result_root = hash_from_byte_slices(&[&value("success.leaf")]);
    chain.header.last_results_hash = result_root.to_vec();
    sign(&mut chain, p.seeds);
    let commitment =
        decode_commitment_proof(&chain.encode_key(&program_key).expect("program commitment"))
            .expect("decoded program commitment");
    let interface_commitment = decode_commitment_proof(
        &chain
            .encode_key(&host_interface_key)
            .expect("interface commitment"),
    )
    .expect("decoded interface commitment");
    let state = ChainProgramStateProof {
        program: p.program,
        ops: chain.prove(&program_key).expect("program membership"),
    };
    let status_ops = if p.status.is_some() {
        chain.prove(&status_key)
    } else {
        chain.prove_absent(&status_key)
    }
    .expect("lifecycle proof");
    let interface_ops = if p.interface.is_some() {
        chain.prove(&host_interface_key)
    } else {
        chain.prove_absent(&host_interface_key)
    }
    .expect("interface proof");
    let witness = StateLeafWitness {
        key: interface_key,
        value: interface_value.unwrap_or_default(),
        proof: interface_ops.clone(),
    };
    let mut proof = DeploymentProof {
        state: ProgramStateProof {
            commitment,
            state: state.clone(),
            lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                program: p.program,
                record: Some(record),
                record_ops: state.ops.clone(),
                status: p.status.clone(),
                status_ops: status_ops.clone(),
            }),
            wind_down: ChainWindDownProof {
                program: p.program,
                value: p.status,
                ops: status_ops,
            },
            transaction: raw.clone(),
            transaction_proof: SimpleProof {
                total: 1,
                index: 0,
                leaf_hash: tx_root,
                aunts: vec![],
            },
        },
        deployment: ChainDeploymentProof::from_state_proofs(
            state,
            p.code.to_vec(),
            &interface_commitment,
        )
        .expect("interface witness transport"),
    };
    if p.execution {
        proof
            .bind_execution(&DeploymentExecutionProof {
                transaction: raw,
                previous: decode_commitment_proof(
                    &prior
                        .encode_key(&program_key)
                        .expect("predecessor commitment"),
                )
                .expect("decoded predecessor commitment"),
                result: TxResult {
                    code: 0,
                    data: value("success"),
                    gas_wanted: 0,
                    gas_used: 0,
                },
                result_proof: SimpleProof {
                    total: 1,
                    index: 0,
                    leaf_hash: result_root,
                    aunts: vec![],
                },
                interface_ops,
                migration: None,
                migration_ops: vec![],
                migration_previous_ops: vec![],
            })
            .expect("deployment execution transport");
    }
    (chain, proof, witness)
}
