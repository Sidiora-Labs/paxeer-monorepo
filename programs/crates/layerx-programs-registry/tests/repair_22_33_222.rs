mod support;

use std::{
    collections::BTreeMap,
    fs,
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_programs::{
    chain_evidence::{
        deployment::ChainDeploymentProof, lifecycle::ChainLifecycleProof,
        program_state::ChainProgramStateProof, programs_state_key, wind_down::ChainWindDownProof,
    },
    chain_proof::{
        decode::decode_commitment_proof,
        header::ChainHeader,
        merkle::{hash_from_byte_slices, SimpleProof},
        results::TxResult,
        validators::TrustedValidator,
        vote::{vote_sign_bytes, Timestamp},
    },
    deployment_envelope::DeploymentEnvelope,
    interface_state_key, interface_state_value, DeploymentExecutionProof, DeploymentProof,
    DeploymentRecord, ProgramInterface, ProgramLifecycleProof, ProgramStateProof,
    ProtocolDeploymentVerifier, Registry, VerifiedProgramCatalog,
};
use layerx_programs_runtime::{Deploy, Lifecycle, ProgramVersion, Upgrade, UpgradePolicy};
use sha2::{Digest as _, Sha256};
use support::{chain::ChainFixture, node_deployment_fixture, program, AUTHORITY};

const CHAIN: &str = "deployment-support-producer";
const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const TRANSCRIPT: &str = include_str!("../src/repair_22_31_256.hex");

fn hash(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

fn transcript() -> BTreeMap<String, Vec<u8>> {
    assert_eq!(
        layerx_programs::hex::encode(&hash(TRANSCRIPT.as_bytes())),
        "5798c9304ce5dd774f153da4f4f3208f1205f4cbfd1854732a49636dc9d76c6e",
        "review the committed node/host serializer transcript before updating its pin"
    );
    TRANSCRIPT
        .lines()
        .map(|line| {
            let (key, value) = line.split_once('=').unwrap();
            (key.to_owned(), layerx_programs::hex::decode(value).unwrap())
        })
        .collect()
}

fn program_key() -> Vec<u8> {
    let mut key = b"program\0".to_vec();
    key.extend_from_slice(&program().bytes());
    programs_state_key(&key)
}

// Consensus/store peer only. The production runtime emits the record used here;
// no test implements or substitutes the deployment verifier or admission codec.
fn state_rows(record: &DeploymentRecord, interface: &ProgramInterface) -> Vec<(Vec<u8>, Vec<u8>)> {
    let UpgradePolicy::Authority(authority) = record.upgrade_policy else {
        panic!("the recorded node transactions use an authority policy");
    };
    let mut value = vec![1];
    value.extend_from_slice(&authority);
    value.extend_from_slice(&record.new_code_hash);
    value.extend_from_slice(&record.abi_version.to_be_bytes());
    value.extend_from_slice(&record.version.to_be_bytes());
    vec![
        (program_key(), value),
        (
            programs_state_key(&interface_state_key(program())),
            interface_state_value(program(), record.version, interface).unwrap(),
        ),
    ]
}

fn sign(chain: &mut ChainFixture) {
    chain.commit.height = chain.header.height;
    chain.commit.block_id.hash = chain.header.hash().unwrap().to_vec();
    let timestamp = Timestamp {
        seconds: chain.header.time.seconds,
        nanos: chain.header.time.nanos,
    };
    let bytes = vote_sign_bytes(
        CHAIN,
        chain.commit.height,
        chain.commit.round,
        Some(&chain.commit.block_id),
        timestamp,
    )
    .unwrap();
    for signature in &mut chain.commit.signatures {
        let key = SEEDS
            .iter()
            .map(SigningKey::from_bytes)
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .unwrap();
        signature.timestamp = timestamp;
        signature.signature = key.sign(&bytes).to_bytes().to_vec();
    }
}

fn indexed_pair(first: &[u8], second: &[u8]) -> ([u8; 32], SimpleProof) {
    (
        hash_from_byte_slices(&[first, second]),
        SimpleProof {
            total: 2,
            index: 1,
            leaf_hash: hash_from_byte_slices(&[second]),
            aunts: vec![hash_from_byte_slices(&[first])],
        },
    )
}

struct PeerExport {
    admission: Vec<u8>,
    proof: DeploymentProof,
    execution: DeploymentExecutionProof,
    expected: DeploymentRecord,
}

// Replay original node serializer bytes into an in-memory consensus/store peer.
// A full host-executed admission export is an explicit out-of-path wiring request
// in support/repair_22_33_222.kvx; this peer does not claim to execute the Go host.
fn peer_exports() -> (ProtocolDeploymentVerifier, Vec<PeerExport>) {
    let source = transcript();
    assert_eq!(
        hash_from_byte_slices(&[&source["success.leaf"]]).as_slice(),
        source["success.root"]
    );
    let policy = UpgradePolicy::Authority(AUTHORITY);
    let mut runtime = Lifecycle::declared().unwrap();
    let first = runtime
        .deploy(Deploy {
            program: program(),
            code_hash: hash(&source["wasm.1"]),
            wasm: source["wasm.1"].clone(),
            abi_version: 1,
            upgrade_policy: policy,
        })
        .unwrap();
    let second = runtime
        .upgrade(Upgrade {
            program: program(),
            authority: AUTHORITY,
            code_hash: hash(&source["wasm.2"]),
            wasm: source["wasm.2"].clone(),
            abi_version: 1,
            migration: None,
        })
        .unwrap();
    let records: Vec<_> = [&first, &second]
        .into_iter()
        .enumerate()
        .map(|(index, receipt)| {
            let wasm = source[&format!("wasm.{}", index + 1)].clone();
            DeploymentRecord::from_deployment(
                receipt,
                &ProgramVersion {
                    code_hash: hash(&wasm),
                    wasm,
                    abi_version: 1,
                },
                policy,
                39 + index as u64,
                1_700_000_001_123 + index as u64 * 1_000,
            )
            .unwrap()
        })
        .collect();
    let unrelated = hash(&source["unrelated"]);
    let transactions = [&source["deploy"], &source["upgrade"]];
    let tx_proofs: Vec<_> = transactions
        .iter()
        .map(|transaction| indexed_pair(&unrelated, &hash(transaction)))
        .collect();
    let (result_root, result_proof) = indexed_pair(&source["failed.leaf"], &source["success.leaf"]);
    let mut chains =
        vec![ChainFixture::new(CHAIN, &SEEDS, vec![(b"unrelated".to_vec(), vec![1])]).unwrap()];
    for (index, record) in records.iter().enumerate() {
        let interface =
            ProgramInterface::decode(&source[&format!("interface.{}", index + 1)]).unwrap();
        chains.push(ChainFixture::new(CHAIN, &SEEDS, state_rows(record, &interface)).unwrap());
    }
    for index in 0..chains.len() {
        if index > 0 {
            let previous_id = chains[index - 1].commit.block_id.clone();
            chains[index].header.last_block_id.hash = previous_id.hash;
            chains[index].header.last_block_id.part_set_total = previous_id.part_set_header.total;
            chains[index].header.last_block_id.part_set_hash = previous_id.part_set_header.hash;
        }
        let chain = &mut chains[index];
        chain.header.height = 38 + index as i64;
        chain.header.time.seconds = 1_700_000_000 + index as i64;
        if index < tx_proofs.len() {
            chain.header.data_hash = tx_proofs[index].0.to_vec();
        }
        if index > 0 {
            chain.header.last_results_hash = result_root.to_vec();
        }
        sign(chain);
    }
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        CHAIN,
        chains[0].verifier.validators.validators().to_vec(),
        5_000,
    )
    .unwrap();
    let exports = records
        .into_iter()
        .enumerate()
        .map(|(index, expected)| {
            let previous = &chains[index];
            let next = &chains[index + 1];
            let commitment =
                decode_commitment_proof(&next.encode_key(&program_key()).unwrap()).unwrap();
            let state = ChainProgramStateProof {
                program: program(),
                ops: commitment.proof_ops.clone(),
            };
            let interface_key = programs_state_key(&interface_state_key(program()));
            let interface_state =
                decode_commitment_proof(&next.encode_key(&interface_key).unwrap()).unwrap();
            let mut status_key = b"wind-down\0s".to_vec();
            status_key.extend_from_slice(&program().bytes());
            let status_ops = next.prove_absent(&programs_state_key(&status_key)).unwrap();
            let mut proof = DeploymentProof {
                state: ProgramStateProof {
                    lifecycle: ProgramLifecycleProof(ChainLifecycleProof {
                        program: program(),
                        record: commitment.value.clone(),
                        record_ops: state.ops.clone(),
                        status: None,
                        status_ops: status_ops.clone(),
                    }),
                    commitment,
                    state: state.clone(),
                    wind_down: ChainWindDownProof {
                        program: program(),
                        value: None,
                        ops: status_ops,
                    },
                    transaction: Vec::new(),
                    transaction_proof: tx_proofs[index].1.clone(),
                },
                deployment: ChainDeploymentProof::from_state_proofs(
                    state,
                    expected.module.clone(),
                    &interface_state,
                )
                .unwrap(),
            };
            let execution = DeploymentExecutionProof {
                transaction: transactions[index].clone(),
                previous: decode_commitment_proof(&previous.encode_key(&program_key()).unwrap())
                    .unwrap(),
                result: TxResult {
                    code: 0,
                    data: source["success"].clone(),
                    gas_wanted: 0,
                    gas_used: 0,
                },
                result_proof: result_proof.clone(),
                interface_ops: interface_state.proof_ops,
                migration: None,
                migration_ops: Vec::new(),
                migration_previous_ops: Vec::new(),
            };
            proof.bind_execution(&execution).unwrap();
            PeerExport {
                admission: proof.canonical_encoding().unwrap(),
                proof,
                execution,
                expected,
            }
        })
        .collect();
    (verifier, exports)
}

#[test]
fn deployment_support_producer_transports_original_admission() {
    let (verifier, exports) = peer_exports();
    let source = transcript();
    let mut registry = Registry::new();
    for (index, export) in exports.iter().enumerate() {
        let fixture =
            node_deployment_fixture(&export.admission, &verifier, export.expected.observed_at)
                .unwrap();
        assert_eq!(fixture.admission(), export.admission);
        assert_eq!(fixture.proof(), &export.proof);
        assert_eq!(fixture.execution(), &export.execution);
        assert_eq!(
            fixture.proof().canonical_encoding().unwrap(),
            export.admission
        );
        assert_eq!(
            fixture.execution().transaction,
            source[if index == 0 { "deploy" } else { "upgrade" }]
        );
        assert_eq!(fixture.execution().result.data, source["success"]);
        assert_eq!(fixture.execution().result_proof.index, 1);
        assert_eq!(fixture.proof().state.transaction_proof.index, 1);
        assert_eq!(fixture.execution().result_proof.total, 2);
        let prior = ChainHeader::decode(&fixture.execution().previous.signed_header).unwrap();
        let next = ChainHeader::decode(&fixture.proof().state.commitment.signed_header).unwrap();
        assert_eq!(prior.height + 1, next.height);
        assert_eq!(next.last_block_id.hash, prior.hash().unwrap());
        assert_ne!(prior.app_hash, next.app_hash);
        assert_eq!(fixture.evidence().record(), &export.expected);
        assert_eq!(
            fixture.evidence().receipt_digest(),
            hash(&fixture.execution().transaction)
        );
        registry
            .record_verified_deployment(fixture.evidence())
            .unwrap();
        assert_eq!(
            registry
                .resolve_deployment(fixture.evidence().clone())
                .unwrap(),
            *fixture.evidence()
        );
        assert_eq!(registry.latest_version(program()), Ok((index + 1) as u32));
        let mut catalog = VerifiedProgramCatalog::declared().unwrap();
        catalog.admit(fixture.evidence().clone()).unwrap();
        assert_eq!(
            catalog.code_hash(program()),
            Some(export.expected.new_code_hash)
        );
    }
    assert_eq!(
        exports[0].proof.state.commitment.signed_header,
        exports[1].execution.previous.signed_header,
        "the upgraded prior state is exactly the initial deployment's committed state"
    );
    assert_eq!(
        exports[1].expected.old_code_hash,
        Some(exports[0].expected.new_code_hash)
    );
}

struct Store(PathBuf);

impl Store {
    fn create() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-33-222-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap();
        Self(path)
    }
}

impl Drop for Store {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

#[test]
fn deployment_support_producer_recovers_original_admission() {
    let store = Store::create();
    let (verifier, exports) = peer_exports();
    let trust = verifier.chain_trust().unwrap().clone();
    let mut accepted = Vec::new();
    for (index, export) in exports.iter().enumerate() {
        let fixture =
            node_deployment_fixture(&export.admission, &verifier, export.expected.observed_at)
                .unwrap();
        fixture
            .proof()
            .publish_canonical(&store.0.join(format!("{index}.admission")))
            .unwrap();
        let envelope = DeploymentEnvelope::from_evidence(fixture.evidence());
        fs::write(
            store.0.join(format!("{index}.envelope")),
            envelope.canonical_encoding().unwrap(),
        )
        .unwrap();
        accepted.push(fixture.evidence().clone());
    }
    drop(exports);
    drop(verifier);
    let reopened = ProtocolDeploymentVerifier::from_chain_trust(
        &trust.chain_id,
        trust.validators.validators().to_vec(),
        5_000,
    )
    .unwrap();
    let mut entries = Vec::new();
    for (index, expected) in accepted.iter().enumerate() {
        let bytes = fs::read(store.0.join(format!("{index}.admission"))).unwrap();
        let fixture =
            node_deployment_fixture(&bytes, &reopened, expected.record().observed_at).unwrap();
        assert_eq!(fixture.evidence(), expected);
        assert_eq!(
            fixture.admission(),
            expected.proof().canonical_encoding().unwrap()
        );
        let envelope = DeploymentEnvelope::decode(
            &fs::read(store.0.join(format!("{index}.envelope"))).unwrap(),
        )
        .unwrap();
        assert_eq!(envelope.proof(), fixture.proof());
        assert_eq!(envelope.record(), expected.record());
        assert_eq!(envelope.receipt_digest(), expected.receipt_digest());
        entries.push(envelope.journal_entry().unwrap());
    }
    let mut recovered = Registry::new();
    recovered.replay_journal(&entries).unwrap();
    assert_eq!(recovered.latest_version(program()), Ok(2));
    for expected in accepted {
        assert_eq!(
            recovered.resolve_deployment(expected.clone()).unwrap(),
            expected
        );
    }
}

#[test]
fn deployment_support_producer_refuses_synthetic_and_cross_wired_evidence() {
    let (verifier, exports) = peer_exports();
    let export = &exports[0];
    let now = export.expected.observed_at;
    let primitive = support::deploy_fixture(
        support::WASM_V1,
        UpgradePolicy::Authority(AUTHORITY),
        38,
        now,
    );
    assert!(node_deployment_fixture(
        &primitive.proof.canonical_encoding().unwrap(),
        &support::verifier_for_fixture(&primitive, 5_000),
        now,
    )
    .is_err());
    assert!(node_deployment_fixture(
        &export.admission[..export.admission.len() - 1],
        &verifier,
        now
    )
    .is_err());
    let mut trailing = export.admission.clone();
    trailing.push(0);
    assert!(node_deployment_fixture(&trailing, &verifier, now).is_err());

    let mut changes: Vec<(&str, DeploymentProof)> = Vec::new();
    for change in 0..7 {
        let mut proof = export.proof.clone();
        let mut execution = export.execution.clone();
        let name = match change {
            0 => {
                execution.transaction = exports[1].execution.transaction.clone();
                "different signed transaction"
            }
            1 => {
                execution.result.code = 9;
                "failed indexed result"
            }
            2 => {
                execution.result.data = transcript()["failed"].clone();
                "EVM failure data"
            }
            3 => {
                execution.result.gas_used = 1;
                "changed deterministic result bytes"
            }
            4 => {
                execution.result_proof.index = 0;
                "different result index"
            }
            5 => {
                execution.previous = exports[1].execution.previous.clone();
                "unlinked predecessor height"
            }
            6 => {
                execution.interface_ops = exports[1].execution.interface_ops.clone();
                "different committed interface"
            }
            _ => unreachable!(),
        };
        proof.bind_execution(&execution).unwrap();
        changes.push((name, proof));
    }
    let mut bare_payload = export.proof.clone();
    bare_payload.state.transaction = primitive.proof.state.transaction;
    changes.push(("bare native payload without execution result", bare_payload));
    let mut cross_state = export.proof.clone();
    cross_state.state.state.ops = exports[1].proof.state.state.ops.clone();
    cross_state.deployment.program_state = cross_state.state.state.clone();
    changes.push(("different host state proof", cross_state));
    let mut cross_interface = export.proof.clone();
    cross_interface.deployment.interface = exports[1].proof.deployment.interface.clone();
    changes.push(("different interface witness head", cross_interface));
    for (name, proof) in changes {
        assert!(
            node_deployment_fixture(&proof.canonical_encoding().unwrap(), &verifier, now).is_err(),
            "accepted {name}"
        );
    }
    assert_eq!(export.admission, export.proof.canonical_encoding().unwrap());
    assert!(node_deployment_fixture(&export.admission, &verifier, now).is_ok());
}

#[test]
fn deployment_support_producer_uses_consumer_trust() {
    let (verifier, exports) = peer_exports();
    let export = &exports[0];
    let now = export.expected.observed_at;
    let foreign_chain = ProtocolDeploymentVerifier::from_chain_trust(
        "foreign-chain",
        verifier
            .chain_trust()
            .unwrap()
            .validators
            .validators()
            .to_vec(),
        5_000,
    )
    .unwrap();
    assert!(node_deployment_fixture(&export.admission, &foreign_chain, now).is_err());
    let foreign_validator = TrustedValidator::new(
        SigningKey::from_bytes(&[0x99; 32])
            .verifying_key()
            .to_bytes(),
        1,
    );
    let foreign_trust =
        ProtocolDeploymentVerifier::from_chain_trust(CHAIN, vec![foreign_validator], 5_000)
            .unwrap();
    assert!(node_deployment_fixture(&export.admission, &foreign_trust, now).is_err());
    assert!(node_deployment_fixture(&export.admission, &verifier, now + 5_001).is_err());
    assert!(node_deployment_fixture(&export.admission, &verifier, now - 1_001).is_err());
}
