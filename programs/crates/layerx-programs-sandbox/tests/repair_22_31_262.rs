#[path = "../../layerx-programs-registry/tests/support/chain.rs"]
mod chain;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::{
    chain_proof::{
        decode::{decode_commitment_proof, CommitmentProof, ProofOp},
        header::ChainHeader,
        merkle::{hash_from_byte_slices, SimpleProof},
        proto::{ProtoReader, ProtoValue},
        results::TxResult,
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
        Commit,
    },
    ProtocolDeploymentVerifier,
};
use layerx_programs_sandbox::{
    activity::SandboxInclusionProof,
    invocation::{
        canonical_precompile_calldata, AuthenticatedSandboxReceiptEvidence,
        SandboxInvocationEvidence,
    },
    sandbox_state_key, sandbox_terminal_key, AuthenticatedTerminalRecord, Lease, LeaseId,
    LeaseState, SandboxProtocolContext, SandboxReceiptEvidence, SandboxSubmission,
    SandboxSubmissionReceipt, TerminalReceiptEvidence, UsageLedger,
};
use sha2::{Digest, Sha256};
use std::{fs, path::Path};

const SEED: [u8; 32] = [17; 32];
const NOW: u64 = 1_700_000_001_123;

fn read(name: &str) -> Vec<u8> {
    let directory = Path::new(env!("CARGO_MANIFEST_DIR")).join("src/repair_22_31_262_recordings");
    let path = directory.join(format!("{name}.hex"));
    let encoded = fs::read_to_string(&path).unwrap_or_else(|error| {
        panic!("missing committed native producer transcript {}: {error}; regenerate with the native Programs/precompile owner before qualification", path.display())
    });
    let digits: Vec<_> = encoded
        .bytes()
        .filter(|byte| !byte.is_ascii_whitespace())
        .collect();
    assert_eq!(digits.len() % 2, 0, "odd transcript length: {name}");
    digits
        .chunks_exact(2)
        .map(|pair| {
            let digit = |byte: u8| match byte {
                b'0'..=b'9' => byte - b'0',
                b'a'..=b'f' => byte - b'a' + 10,
                _ => panic!("invalid native transcript hex: {name}"),
            };
            digit(pair[0]) * 16 + digit(pair[1])
        })
        .collect()
}
fn signed(header: &ChainHeader, mut commit: Commit) -> Commit {
    commit.height = header.height;
    commit.block_id.hash = header.hash().unwrap().to_vec();
    let key = SigningKey::from_bytes(&SEED);
    let timestamp = Timestamp {
        seconds: header.time.seconds,
        nanos: header.time.nanos,
    };
    let bytes = vote_sign_bytes(
        &header.chain_id,
        commit.height,
        commit.round,
        Some(&commit.block_id),
        timestamp,
    )
    .unwrap();
    commit.signatures = vec![CommitSig {
        flag: BlockIdFlag::Commit,
        validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
        timestamp,
        signature: key.sign(&bytes).to_bytes().to_vec(),
    }];
    commit
}
fn varint(mut n: u64, out: &mut Vec<u8>) {
    while n >= 128 {
        out.push((n as u8 & 127) | 128);
        n >>= 7;
    }
    out.push(n as u8);
}
fn field(n: u8, value: &[u8], out: &mut Vec<u8>) {
    out.push(n << 3 | 2);
    varint(value.len() as u64, out);
    out.extend_from_slice(value);
}
fn ops(encoded: &[u8]) -> Vec<ProofOp> {
    let mut reader = ProtoReader::new(encoded);
    let mut out = Vec::new();
    while let Some(f) = reader.next_field().unwrap() {
        let ProtoValue::Bytes(bytes) = f.value else {
            panic!("proof op bytes")
        };
        assert_eq!(f.number, 1);
        let mut r = ProtoReader::new(bytes);
        let mut op = ProofOp {
            op_type: String::new(),
            key: Vec::new(),
            data: Vec::new(),
        };
        while let Some(f) = r.next_field().unwrap() {
            let ProtoValue::Bytes(b) = f.value else {
                panic!("proof op field")
            };
            match f.number {
                1 => op.op_type = String::from_utf8(b.to_vec()).unwrap(),
                2 => op.key = b.to_vec(),
                3 => op.data = b.to_vec(),
                _ => panic!("unknown proof op"),
            }
        }
        out.push(op);
    }
    assert_eq!(out.len(), 2);
    out
}
fn commitment(
    fixture: &chain::ChainFixture,
    prefix: &str,
    label: &str,
    store: &str,
    key: Vec<u8>,
) -> CommitmentProof {
    let mut proof = decode_commitment_proof(&fixture.encode().unwrap()).unwrap();
    proof.store_name = store.to_owned();
    proof.key = key;
    proof.value = Some(read(&format!("{prefix}.{label}.value")));
    proof.proof_ops = ops(&read(&format!("{prefix}.{label}.ops")));
    proof
}
fn inclusion(leaf: &[u8]) -> SimpleProof {
    SimpleProof {
        total: 1,
        index: 0,
        leaf_hash: hash_from_byte_slices(&[leaf]),
        aunts: Vec::new(),
    }
}

fn evidence(
    name: &str,
) -> (
    chain::ChainFixture,
    SandboxSubmission,
    AuthenticatedSandboxReceiptEvidence,
) {
    let submission = SandboxSubmission::decode(
        &read(&format!("{name}.payload")),
        SandboxProtocolContext::new(3, 4).unwrap(),
    )
    .unwrap();
    let tx = read(&format!("{name}.tx"));
    let tx_hash = Sha256::digest(&tx);
    let result = TxResult {
        data: read(&format!("{name}.result")),
        ..TxResult::default()
    };
    let mut result_leaf = Vec::new();
    field(2, &result.data, &mut result_leaf);
    let mut fixture = chain::ChainFixture::new_in_store(
        "lxphost",
        "repair-native",
        &[SEED],
        vec![(b"encoder".to_vec(), vec![1])],
    )
    .unwrap();
    fixture.header.height = 101;
    fixture.header.time.seconds = 1_700_000_001;
    fixture.header.time.nanos = 123_000_000;
    let mut current = fixture.header.clone();
    current.height = 100;
    current.time.seconds -= 1;
    current.app_hash = read(&format!("{name}.before.root"));
    current.data_hash = hash_from_byte_slices(&[&tx_hash]).to_vec();
    let current_commit = signed(&current, fixture.commit.clone());
    fixture.header.last_block_id.hash = current.hash().unwrap().to_vec();
    fixture.header.last_results_hash = hash_from_byte_slices(&[&result_leaf]).to_vec();
    fixture.header.app_hash = read(&format!("{name}.after.root"));
    fixture.commit = signed(&fixture.header, fixture.commit.clone());
    let kind = if name == "sandboxExecute" { 2 } else { 0 };
    let proven = commitment(
        &fixture,
        &format!("{name}.after"),
        "state",
        "lxphost",
        sandbox_state_key(kind, submission.lease().bytes()).unwrap(),
    );
    let chain = SandboxReceiptEvidence {
        tx,
        tx_proof: SandboxInclusionProof {
            header: current.clone(),
            commit: current_commit.clone(),
            proof: inclusion(&tx_hash),
        },
        tx_result: result,
        result_proof: SandboxInclusionProof {
            header: fixture.header.clone(),
            commit: fixture.commit.clone(),
            proof: inclusion(&result_leaf),
        },
        usage_ops: proven.proof_ops.clone(),
        commitment: proven,
    };
    let next = fixture.header.clone();
    let next_commit = fixture.commit.clone();
    fixture.header = current;
    fixture.commit = current_commit;
    let signer_binding = commitment(
        &fixture,
        &format!("{name}.before"),
        "binding",
        "evm",
        read(&format!("{name}.before.binding.key")),
    );
    let previous_lease = if name == "sandboxFund" {
        None
    } else {
        Some(commitment(
            &fixture,
            &format!("{name}.before"),
            "lease",
            "lxphost",
            sandbox_state_key(0, submission.lease().bytes()).unwrap(),
        ))
    };
    fixture.header = next;
    fixture.commit = next_commit;
    (
        fixture,
        submission,
        AuthenticatedSandboxReceiptEvidence {
            chain,
            invocation: SandboxInvocationEvidence {
                ethereum_tx: read(&format!("{name}.ethereum")),
                native_activity: read(&format!("{name}.activity")),
                signer_binding,
                previous_lease,
            },
        },
    )
}
fn accepted(name: &str) -> SandboxSubmissionReceipt {
    let (fixture, submission, evidence) = evidence(name);
    let receipt = SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, evidence, NOW)
        .unwrap_or_else(|e| panic!("{name}: {e:?}"));
    assert_eq!(
        receipt.activity_id().as_slice(),
        read(&format!("{name}.id"))
    );
    receipt
}

#[test]
fn live_registration_native_funding_activation_execution() {
    assert!(SandboxProtocolContext::new(2, 3).is_err());
    assert!(SandboxProtocolContext::new(3, 3).is_err());
    assert!(SandboxProtocolContext::new(2, 4).is_err());
    for length in 0..36 {
        let mut truncated = vec![0; length];
        if length >= 4 {
            truncated[..4].copy_from_slice(&[1, 1, 0, 0]);
        }
        assert!(
            SandboxSubmission::decode(&truncated, SandboxProtocolContext::new(3, 4).unwrap())
                .is_err()
        );
    }
    let (_, submission, proof) = evidence("sandboxFund");
    let payload = submission.payload();
    let length = u32::from_be_bytes(payload[248..252].try_into().unwrap()) as usize;
    let lease = Lease::decode_state(&payload[252..252 + length]).unwrap();
    let key: [u8; 32] = proof
        .invocation
        .signer_binding
        .value
        .unwrap()
        .try_into()
        .unwrap();
    let constructed = layerx_programs_sandbox::activity::canonical_fund_for_did(
        SandboxProtocolContext::new(3, 4).unwrap(),
        &lease,
        key,
        &payload[256 + length..],
    )
    .unwrap();
    assert_eq!(constructed, submission);
    let mut foreign_key = key;
    foreign_key[0] ^= 1;
    assert!(layerx_programs_sandbox::activity::canonical_fund_for_did(
        SandboxProtocolContext::new(3, 4).unwrap(),
        &lease,
        foreign_key,
        &payload[256 + length..]
    )
    .is_err());
    for name in ["sandboxFund", "sandboxActivate", "sandboxExecute"] {
        let receipt = accepted(name);
        if name != "sandboxExecute" {
            let lease = Lease::decode_state(receipt.canonical_receipt()).unwrap();
            assert_eq!(
                lease.state(),
                if name == "sandboxFund" {
                    LeaseState::Funded
                } else {
                    LeaseState::Active
                }
            );
        }
    }
}

#[test]
fn included_transaction_binds_signer_target_payload_result_and_native_id() {
    let (fixture, submission, evidence) = evidence("sandboxExecute");
    assert_ne!(evidence.chain.tx, submission.payload());
    assert_ne!(
        Sha256::digest(&evidence.chain.tx).as_slice(),
        read("sandboxExecute.id")
    );
    assert!(SandboxSubmissionReceipt::verify(
        &fixture.verifier,
        &submission,
        evidence.clone(),
        NOW
    )
    .is_ok());
    assert!(SandboxSubmissionReceipt::verify(
        &fixture.verifier,
        &submission,
        evidence.chain.clone(),
        NOW
    )
    .is_err());
    for index in 0..8 {
        let mut bad = evidence.clone();
        match index {
            0 => bad.chain.tx = submission.payload().to_vec(),
            1 => bad.invocation.ethereum_tx[30] ^= 1,
            2 => bad.invocation.native_activity[25] ^= 1,
            3 => bad.invocation.signer_binding.value.as_mut().unwrap()[0] ^= 1,
            4 => bad.chain.tx_result.code = 1,
            5 => bad.chain.tx_result.data[10] ^= 1,
            6 => {
                bad.invocation
                    .previous_lease
                    .as_mut()
                    .unwrap()
                    .value
                    .as_mut()
                    .unwrap()[50] ^= 1
            }
            _ => bad.invocation.native_activity.pop().map(|_| ()).unwrap(),
        }
        assert!(
            SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, bad, NOW).is_err(),
            "tamper {index}"
        );
    }
    let fee = 100_000_000u128.to_be_bytes();
    assert_eq!(
        canonical_precompile_calldata(&submission, fee).unwrap(),
        read("sandboxExecute.input")
    );
}

fn frames() -> Vec<Vec<u8>> {
    let bytes = read("usage.frames");
    let mut input = bytes.as_slice();
    let mut out = Vec::new();
    while !input.is_empty() {
        let n = u32::from_be_bytes(input[..4].try_into().unwrap()) as usize;
        out.push(input[4..4 + n].to_vec());
        input = &input[4 + n..];
    }
    out
}
#[test]
fn usage_state_shape_native_kind_two_ledger_and_chunked_events() {
    let receipt = accepted("sandboxExecute");
    let ledger = UsageLedger::decode_canonical(receipt.canonical_receipt()).unwrap();
    assert_eq!(ledger.receipt_count(), 1);
    assert_eq!(
        ledger.latest().unwrap(),
        receipt.authenticated_usage_receipt().unwrap().receipt()
    );
    let original = frames();
    assert!(original.len() > 1);
    receipt.verify_native_usage_frames(&original).unwrap();
    for index in 0..4 {
        let mut bad = original.clone();
        match index {
            0 => {
                bad.pop();
            }
            1 => bad.reverse(),
            2 => bad[0][4] ^= 1,
            _ => bad[0][48] ^= 1,
        }
        assert!(receipt.verify_native_usage_frames(&bad).is_err());
    }
    assert!(UsageLedger::decode_canonical(&original[0]).is_err());
    let (fixture, submission, mut bad) = evidence("sandboxExecute");
    bad.chain.commitment.key = sandbox_state_key(1, submission.lease().bytes()).unwrap();
    assert!(SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, bad, NOW).is_err());
}

#[test]
fn activation_receipt_authenticates_native_funded_to_active_history() {
    let (fixture, submission, proof) = evidence("sandboxActivate");
    let previous = Lease::decode_state(
        proof
            .invocation
            .previous_lease
            .as_ref()
            .unwrap()
            .value
            .as_ref()
            .unwrap(),
    )
    .unwrap();
    assert_eq!(previous.state(), LeaseState::Funded);
    let receipt =
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, proof.clone(), NOW)
            .unwrap();
    let active = Lease::decode_state(receipt.canonical_receipt()).unwrap();
    assert_eq!(active.state(), LeaseState::Active);
    assert_eq!(active.tenant(), previous.tenant());
    assert_eq!(active.history().len(), previous.history().len() + 1);
    assert_eq!(
        &active.history()[..previous.history().len()],
        previous.history()
    );
    assert_eq!(
        active.history().last().unwrap().transition.activity_id,
        receipt.activity_id()
    );
    assert!(receipt.authenticated_usage_receipt().is_err());
    let mut bad = proof;
    bad.invocation.previous_lease = None;
    assert!(SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, bad, NOW).is_err());
}

#[test]
fn terminal_key_native_destruction_survives_node_store_reopen() {
    let mut fixture = chain::ChainFixture::new_in_store(
        "lxphost",
        "repair-native",
        &[SEED],
        vec![(b"encoder".to_vec(), vec![1])],
    )
    .unwrap();
    fixture.header.height = 101;
    fixture.header.time.seconds = 1_700_000_001;
    fixture.header.time.nanos = 123_000_000;
    fixture.header.app_hash = read("terminal.root");
    fixture.commit = signed(&fixture.header, fixture.commit.clone());
    let lease = LeaseId::new([
        1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0,
        0, 0,
    ])
    .unwrap();
    let proof = commitment(
        &fixture,
        "terminal",
        "state",
        "lxphost",
        sandbox_terminal_key(lease.bytes()),
    );
    let evidence = TerminalReceiptEvidence {
        terminal_ops: proof.proof_ops.clone(),
        commitment: proof,
    };
    let trust = ProtocolDeploymentVerifier::from_chain_trust(
        "repair-native",
        fixture.verifier.validators.validators().to_vec(),
        1000,
    )
    .unwrap();
    let terminal = AuthenticatedTerminalRecord::verify(&trust, &evidence, NOW, lease).unwrap();
    assert_eq!(
        terminal.canonical_bytes().as_slice(),
        read("native.terminal")
    );
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap();
    let mut archive_key = sandbox_state_key(0, lease.bytes()).unwrap();
    archive_key[3] = b'd';
    let archive_proof = commitment(&fixture, "terminal", "archive", "lxphost", archive_key);
    let descriptor = archive_proof.value.as_ref().unwrap();
    assert_eq!(descriptor.len(), 41);
    assert_eq!(&descriptor[..5], b"LXDA1");
    let blob_key = [&[5, 0, 9][..], &descriptor[9..]].concat();
    let blob_proof = commitment(&fixture, "terminal", "archive_blob", "lxphost", blob_key);
    for proof in [&archive_proof, &blob_proof] {
        layerx_programs::verify_state(
            &head,
            "lxphost",
            &proof.key,
            proof.value.as_ref().unwrap(),
            &proof.proof_ops,
        )
        .unwrap();
    }
    let archived = blob_proof.value.as_ref().unwrap();
    assert_eq!(
        u32::from_be_bytes(descriptor[5..9].try_into().unwrap()) as usize,
        archived.len()
    );
    assert!(
        archived.len() > 1024,
        "native archive retains history exceeding a module value"
    );
    assert_eq!(Sha256::digest(archived).as_slice(), &descriptor[9..]);
    assert_eq!(&descriptor[9..], &terminal.canonical_bytes()[197..229]);
    let destroyed = Lease::decode_state(archived).unwrap();
    assert_eq!(destroyed.id(), lease);
    assert_eq!(destroyed.state(), LeaseState::Destroyed);
    assert_ne!(
        sandbox_terminal_key(lease.bytes()),
        sandbox_state_key(2, lease.bytes()).unwrap()
    );
    let mut bad = evidence.clone();
    bad.commitment.key = sandbox_state_key(2, lease.bytes()).unwrap();
    assert!(AuthenticatedTerminalRecord::verify(&trust, &bad, NOW, lease).is_err());
    let mut bad = evidence;
    bad.commitment.value = Some(read("sandboxExecute.after.state.value"));
    assert!(AuthenticatedTerminalRecord::verify(&trust, &bad, NOW, lease).is_err());
}

mod restore_case {
    use super::chain;
    use ed25519_dalek::{Signer, SigningKey};
    use layerx_programs::chain_proof::{
        decode::decode_commitment_proof, header::ChainHeader, validators::TrustedValidator,
        vote::vote_sign_bytes,
    };
    use sha2::{Digest, Sha256};

    use layerx_programs_runtime::test_support::{
        code_section, func_body, function_section, module, raw_section, type_section, unsigned_leb,
        OP_END, OP_I32_ADD, OP_I32_CONST, TYPE_I32,
    };
    use layerx_programs_runtime::{
        Abi, AuthorizationContext, CapabilitySet, FeeSchedule, Meter, PrincipalId, ProgramId,
        ProgramInstance, ResourceBudget, Storage, UnavailableReceiptOracle, ValidatedModule,
        ValidationLimits, WasmEngine, WasmValue, ABI_V2_VERSION,
    };
    use layerx_programs_sandbox::{
        restore, Lease, LeaseActivity, LeaseId, LeaseLimits, LeaseState, LeaseTransition,
        SandboxState, Snapshot, SnapshotRefusal, TransitionEvidence,
    };

    use layerx_programs_sandbox::snapshot::{RestoreRequest, RestoredSandbox};

    const ENTRYPOINT: &[u8] = b"sandbox_transition";
    const CALL_PREFIX: usize = 106;
    const VALIDATOR_SEEDS: [[u8; 32]; 3] = [[1; 32], [2; 32], [3; 32]];

    fn candidate(id: u8, tenant: u8, opened: u64, expiry: u64) -> Lease {
        Lease::request(
            LeaseId::new([id; 32]).unwrap_or_else(|error| panic!("lease id: {error}")),
            PrincipalId::new([tenant; 32]).unwrap_or_else(|error| panic!("tenant: {error}")),
            ProgramId::new([3; 32]).unwrap_or_else(|error| panic!("host: {error}")),
            layerx_programs_runtime::hash_bytes(
                layerx_programs_runtime::HashAlgorithm::Sha256,
                &continuation_module(),
            )
            .unwrap_or_else(|error| panic!("hash: {error}")),
            [4; 32],
            10_000,
            LeaseLimits {
                cpu_fuel: 10_000,
                memory_bytes: 1 << 20,
                storage_read_bytes: 1 << 20,
                storage_write_bytes: 1 << 20,
                output_values: 1_000,
                output_bytes: 1 << 20,
                table_elements: 1_000,
                namespace_bytes: 1 << 20,
            },
            opened,
            expiry,
        )
        .unwrap_or_else(|error| panic!("lease: {error}"))
    }

    fn candidate_namespace(id: u8, tenant: u8, namespace_bytes: u64) -> Lease {
        Lease::request(
            LeaseId::new([id; 32]).unwrap_or_else(|error| panic!("lease id: {error}")),
            PrincipalId::new([tenant; 32]).unwrap_or_else(|error| panic!("tenant: {error}")),
            ProgramId::new([3; 32]).unwrap_or_else(|error| panic!("host: {error}")),
            layerx_programs_runtime::hash_bytes(
                layerx_programs_runtime::HashAlgorithm::Sha256,
                &continuation_module(),
            )
            .unwrap_or_else(|error| panic!("hash: {error}")),
            [4; 32],
            10_000,
            LeaseLimits {
                cpu_fuel: 10_000,
                memory_bytes: 1 << 20,
                storage_read_bytes: 1 << 20,
                storage_write_bytes: 1 << 20,
                output_values: 1_000,
                output_bytes: 1 << 20,
                table_elements: 1_000,
                namespace_bytes,
            },
            10,
            30,
        )
        .unwrap_or_else(|error| panic!("lease: {error}"))
    }

    fn call_payload(host: ProgramId, transition: LeaseTransition) -> Vec<u8> {
        let mut calldata = vec![0; 101];
        calldata[..2].copy_from_slice(&1u16.to_be_bytes());
        calldata[2..34].copy_from_slice(&transition.lease.bytes());
        calldata[34..66].copy_from_slice(&transition.tenant.bytes());
        calldata[66] = transition.activity as u8;
        calldata[67] = transition.from as u8;
        calldata[68] = transition.to as u8;
        calldata[69..].copy_from_slice(&transition.usage_observation_digest);
        let mut payload = vec![0; CALL_PREFIX];
        payload[..32].copy_from_slice(&host.bytes());
        payload[32..34].copy_from_slice(&2u16.to_be_bytes());
        payload[34..36].copy_from_slice(
            &u16::try_from(ENTRYPOINT.len())
                .unwrap_or_else(|error| panic!("entrypoint length: {error}"))
                .to_be_bytes(),
        );
        payload[36..40].copy_from_slice(
            &u32::try_from(calldata.len())
                .unwrap_or_else(|error| panic!("calldata length: {error}"))
                .to_be_bytes(),
        );
        payload[46..50].copy_from_slice(&1u32.to_be_bytes());
        payload.extend_from_slice(ENTRYPOINT);
        payload.extend_from_slice(&calldata);
        payload
    }

    fn proven_transition(
        lease: &Lease,
        mut transition: LeaseTransition,
        block_height: u64,
    ) -> (LeaseTransition, Vec<u8>, chain::ChainFixture) {
        transition.activity_id =
            Sha256::digest(call_payload(lease.host_program(), transition)).into();
        let mut canonical = lease
            .canonical_state_bytes()
            .unwrap_or_else(|error| panic!("lease state: {error}"));
        // The fixed fields precede the state tag and the length-prefixed receipt history.
        let state_offset = b"LayerX/programs/sandbox/lease-state/v3\0".len()
            + 8 * 32
            + 16
            + 18 * 8
            + 4
            + 7 * 8
            + 16;
        canonical[state_offset] = transition.to as u8;
        canonical[state_offset + 1] = u8::try_from(lease.history().len() + 1)
            .unwrap_or_else(|error| panic!("history length: {error}"));
        let history_end = state_offset + 2 + lease.history().len() * (3 + 3 * 32 + 8);
        let mut receipt = vec![
            transition.activity as u8,
            transition.from as u8,
            transition.to as u8,
        ];
        receipt.extend_from_slice(&transition.activity_id);
        receipt.extend_from_slice(&transition.usage_observation_digest);
        let mut digest = Sha256::new();
        digest.update(lease.id().bytes());
        digest.update(&receipt);
        digest.update(block_height.to_be_bytes());
        receipt.extend_from_slice(&digest.finalize());
        receipt.extend_from_slice(&block_height.to_be_bytes());
        canonical.splice(history_end..history_end, receipt);
        let key = layerx_programs_sandbox::sandbox_state_key(0, lease.id().bytes())
            .unwrap_or_else(|error| panic!("lease key: {error}"));
        let mut fixture = chain::ChainFixture::new_in_store(
            "lxphost",
            "paxeer-lease-chain",
            &VALIDATOR_SEEDS,
            vec![(key, canonical.clone())],
        )
        .unwrap_or_else(|error| panic!("chain: {error:?}"));
        assert!(block_height > 0, "state height must be positive");
        fixture.header.height = i64::try_from(block_height)
            .ok()
            .and_then(|height| height.checked_add(1))
            .unwrap_or_else(|| panic!("block height overflow"));
        fixture.commit.height = fixture.header.height;
        fixture.commit.block_id.hash = fixture
            .header
            .hash()
            .unwrap_or_else(|error| panic!("header hash: {error:?}"))
            .to_vec();
        let keys = VALIDATOR_SEEDS.map(|seed| SigningKey::from_bytes(&seed));
        for signature in &mut fixture.commit.signatures {
            let key = keys
                .iter()
                .find(|key| {
                    TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                        .address
                        .as_slice()
                        == signature.validator_address
                })
                .unwrap_or_else(|| panic!("validator signing key absent"));
            let bytes = vote_sign_bytes(
                &fixture.header.chain_id,
                fixture.commit.height,
                fixture.commit.round,
                Some(&fixture.commit.block_id),
                signature.timestamp,
            )
            .unwrap_or_else(|error| panic!("vote sign bytes: {error:?}"));
            signature.signature = key.sign(&bytes).to_bytes().to_vec();
        }
        (transition, canonical, fixture)
    }

    fn evidence(
        lease: &Lease,
        transition: LeaseTransition,
        block_height: u64,
    ) -> (LeaseTransition, TransitionEvidence) {
        let (transition, canonical, fixture) = proven_transition(lease, transition, block_height);
        let key = layerx_programs_sandbox::sandbox_state_key(0, lease.id().bytes())
            .unwrap_or_else(|error| panic!("lease key: {error}"));
        let commitment = decode_commitment_proof(
            &fixture
                .encode_key(&key)
                .unwrap_or_else(|error| panic!("encode lease commitment: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("decode lease commitment: {error:?}"));
        assert_eq!(commitment.store_name, "lxphost");
        assert_eq!(commitment.key, key);
        assert_eq!(commitment.value.as_deref(), Some(canonical.as_slice()));
        let header = ChainHeader::decode(&commitment.signed_header)
            .unwrap_or_else(|error| panic!("decode header: {error:?}"));
        let head = fixture
            .verifier
            .verify_head(&header, &fixture.commit)
            .unwrap_or_else(|error| panic!("head: {error:?}"));
        let call_evidence = TransitionEvidence::verify_call(
            &head,
            lease,
            transition,
            &AuthorizationContext::new(lease.tenant(), CapabilitySet::empty()),
            commitment
                .value
                .as_deref()
                .unwrap_or_else(|| panic!("lease commitment value absent")),
            &commitment.proof_ops,
        )
        .unwrap_or_else(|error| panic!("evidence: {error}"));
        assert_eq!(call_evidence.state_height(), block_height);
        assert_eq!(call_evidence.tx_hash(), transition.activity_id);
        (transition, call_evidence)
    }

    fn advance(mut lease: Lease, edges: &[(LeaseActivity, LeaseState, LeaseState, u64)]) -> Lease {
        let request = LeaseTransition {
            lease: lease.id(),
            tenant: lease.tenant(),
            activity: LeaseActivity::Request,
            from: LeaseState::Requested,
            to: LeaseState::Requested,
            activity_id: [0; 32],
            usage_observation_digest: lease
                .request_binding_digest()
                .unwrap_or_else(|error| panic!("request digest: {error}")),
        };
        let (request, proof) = evidence(&lease, request, lease.opened_at());
        let mut book = layerx_programs_sandbox::LeaseBook::new();
        book.insert_requested(lease, request, proof)
            .unwrap_or_else(|error| panic!("request: {error}"));
        for &(activity, from, to, block_height) in edges {
            lease = book
                .get(request.lease)
                .unwrap_or_else(|| panic!("lease"))
                .clone();
            let transition = LeaseTransition {
                lease: lease.id(),
                tenant: lease.tenant(),
                activity,
                from,
                to,
                activity_id: [0; 32],
                usage_observation_digest: [0; 32],
            };
            let (transition, proof) = evidence(&lease, transition, block_height);
            book.transition(lease.id(), transition, proof)
                .unwrap_or_else(|error| panic!("advance: {error}"));
        }
        book.get(request.lease)
            .unwrap_or_else(|| panic!("advanced lease"))
            .clone()
    }

    fn advance_existing(
        mut lease: Lease,
        edges: &[(LeaseActivity, LeaseState, LeaseState, u64)],
    ) -> Lease {
        for &(activity, from, to, block_height) in edges {
            let transition = LeaseTransition {
                lease: lease.id(),
                tenant: lease.tenant(),
                activity,
                from,
                to,
                activity_id: [0; 32],
                usage_observation_digest: [0; 32],
            };
            let (transition, proof) = evidence(&lease, transition, block_height);
            lease
                .transition(transition, proof)
                .unwrap_or_else(|error| panic!("advance existing: {error}"));
        }
        lease
    }

    fn active(id: u8, tenant: u8) -> Lease {
        advance(
            candidate(id, tenant, 10, 30),
            &[
                (
                    LeaseActivity::Fund,
                    LeaseState::Requested,
                    LeaseState::Funded,
                    11,
                ),
                (
                    LeaseActivity::Activate,
                    LeaseState::Funded,
                    LeaseState::Active,
                    12,
                ),
            ],
        )
    }

    fn funded(id: u8, tenant: u8) -> Lease {
        advance(
            candidate(id, tenant, 20, 40),
            &[(
                LeaseActivity::Fund,
                LeaseState::Requested,
                LeaseState::Funded,
                21,
            )],
        )
    }

    fn activation(lease: &Lease, block_height: u64) -> (LeaseTransition, TransitionEvidence) {
        evidence(
            lease,
            LeaseTransition {
                lease: lease.id(),
                tenant: lease.tenant(),
                activity: LeaseActivity::Activate,
                from: LeaseState::Funded,
                to: LeaseState::Active,
                activity_id: [0; 32],
                usage_observation_digest: [0; 32],
            },
            block_height,
        )
    }

    fn meter() -> Meter {
        Meter::new(
            ResourceBudget::new_complete(1_000, 1 << 20, 1 << 20, 1 << 20, 100, 1 << 20, 100),
            FeeSchedule::declared(),
        )
    }

    fn continuation_module() -> Vec<u8> {
        use layerx_programs_runtime::test_support::{import_section, TYPE_I64};
        let imports = [
            ("layerx_v1", "storage_read", 0),
            ("layerx_v1", "storage_write", 0),
            ("layerx_v1", "transfer_402", 1),
            ("layerx_v1", "event_emit", 0),
            ("layerx_v2", "storage_read_scoped", 2),
            ("layerx_v2", "storage_write_scoped", 2),
            ("layerx_v1", "program_call", 3),
        ];
        let mut body = Vec::new();
        let constant = |value: i64, body: &mut Vec<u8>| {
            body.push(OP_I32_CONST);
            let mut value = value;
            loop {
                let mut byte = (value as u8) & 127;
                value >>= 7;
                let done = (value == 0 && byte & 64 == 0) || (value == -1 && byte & 64 != 0);
                if !done {
                    byte |= 128;
                }
                body.push(byte);
                if done {
                    break;
                }
            }
        };
        // Every check contributes one to the return value; all eight must hold.
        constant(0, &mut body);
        let check = |index: u8, args: &[i64], expected: i64, body: &mut Vec<u8>| {
            for arg in args {
                constant(*arg, body);
            }
            body.extend([0x10, index]);
            constant(expected, body);
            body.extend([0x46, OP_I32_ADD]);
        };
        check(0, &[0, 1, 128, 1], 2, &mut body);
        constant(128, &mut body);
        body.extend([0x2d, 0, 0]);
        constant(41, &mut body);
        body.extend([0x46, OP_I32_ADD]);
        check(1, &[0, 1, 64, 1], 0, &mut body);
        // transfer_402 has two i64 amount words followed by four i32 pointers.
        body.extend([0x42, 0, 0x42, 1]);
        for arg in [32, 32, 32, 32] {
            constant(arg, &mut body);
        }
        body.extend([0x10, 2]);
        constant(-1, &mut body);
        body.extend([0x46, OP_I32_ADD]);
        check(3, &[0, 1, 0, 1], -1, &mut body);
        check(4, &[2, 0, 1, 128, 1], -1, &mut body);
        check(5, &[2, 0, 1, 64, 1], -1, &mut body);
        check(6, &[32, 32, 0, 0, 96, 2], -1, &mut body);
        body.push(OP_END);
        let mut data = vec![0u8; 98];
        data[0] = b'k';
        data[32..64].fill(8);
        data[64] = 77;
        let mut segment = vec![1, 0, OP_I32_CONST, 0, OP_END];
        segment.extend(unsigned_leb(data.len() as u64));
        segment.extend(data);
        let mut exports = vec![2];
        for (name, kind, index) in [("continue", 0, 7), ("memory", 2, 0)] {
            exports.extend(unsigned_leb(name.len() as u64));
            exports.extend_from_slice(name.as_bytes());
            exports.push(kind);
            exports.push(index);
        }
        module(&[
            type_section(&[
                (&[TYPE_I32; 4], &[TYPE_I32]),
                (
                    &[TYPE_I64, TYPE_I64, TYPE_I32, TYPE_I32, TYPE_I32, TYPE_I32],
                    &[TYPE_I32],
                ),
                (&[TYPE_I32; 5], &[TYPE_I32]),
                (&[TYPE_I32; 6], &[TYPE_I32]),
                (&[], &[TYPE_I32]),
            ]),
            import_section(&imports),
            function_section(&[4]),
            raw_section(5, &[1, 0, 1]),
            raw_section(7, &exports),
            code_section(&[func_body(&[], &body)]),
            raw_section(11, &segment),
        ])
    }

    fn validated_continuation() -> layerx_programs_runtime::ValidatedModule {
        WasmEngine::new(ValidationLimits::declared())
            .unwrap_or_else(|error| panic!("engine: {error}"))
            .validate_v2(&continuation_module())
            .unwrap_or_else(|error| panic!("validation: {error}"))
    }

    fn running_instance(
        lease: &Lease,
        module: &layerx_programs_runtime::ValidatedModule,
        storage: Storage,
    ) -> layerx_programs_runtime::ProgramInstance {
        let authorization = AuthorizationContext::new(
            lease.namespace().execution_principal().unwrap(),
            CapabilitySet::new([
                layerx_programs_runtime::Capability::StorageRead,
                layerx_programs_runtime::Capability::StorageWrite,
            ])
            .unwrap(),
        );
        let abi = Abi::new(
            ABI_V2_VERSION,
            lease.host_program(),
            authorization,
            storage,
            &UnavailableReceiptOracle,
        )
        .unwrap_or_else(|error| panic!("abi: {error}"));
        module
            .instantiate_sandbox(meter(), abi)
            .unwrap_or_else(|error| panic!("sandbox instance: {error}"))
    }

    fn snapshot_transition(
        lease: &Lease,
        digest: [u8; 32],
        block_height: u64,
    ) -> (LeaseTransition, TransitionEvidence) {
        evidence(
            lease,
            LeaseTransition {
                lease: lease.id(),
                tenant: lease.tenant(),
                activity: LeaseActivity::Snapshot,
                from: LeaseState::Active,
                to: LeaseState::Active,
                activity_id: [0; 32],
                usage_observation_digest: digest,
            },
            block_height,
        )
    }

    #[test]
    fn module_registration_and_guest_abi_remain_distinct() {
        let lease = active(83, 9);
        let protocol = layerx_programs_sandbox::SandboxProtocolContext::new(3, 4).unwrap();
        let mut call = vec![0; CALL_PREFIX];
        call[..32].copy_from_slice(&lease.host_program().bytes());
        call[34..36].copy_from_slice(&8u16.to_be_bytes());
        call[40..42].copy_from_slice(&4u16.to_be_bytes());
        call[46..50].copy_from_slice(&1u32.to_be_bytes());
        for limit in call[50..106].chunks_exact_mut(8) {
            limit.copy_from_slice(&1u64.to_be_bytes());
        }
        call.extend_from_slice(b"continue");
        call.extend_from_slice(&[0, 2, 1, 2]);
        for guest in [1u16, 2, 3, 4] {
            call[32..34].copy_from_slice(&guest.to_be_bytes());
            let submission =
                layerx_programs_sandbox::canonical_execute(protocol, &lease, 1, &call).unwrap();
            assert_eq!(&submission.payload()[236..], call);
        }
        for guest in [0u16, 5, 6, u16::MAX] {
            call[32..34].copy_from_slice(&guest.to_be_bytes());
            assert!(
                layerx_programs_sandbox::canonical_execute(protocol, &lease, 1, &call).is_err()
            );
        }
        call[32..34].copy_from_slice(&3u16.to_be_bytes());
        *call.last_mut().unwrap() = 3;
        assert!(layerx_programs_sandbox::canonical_execute(protocol, &lease, 1, &call).is_err());
    }

    #[test]
    fn restore_authority_reopens_storage_and_restricts_resumed_abi() {
        use layerx_programs_runtime::{Capability, StorageNamespace};
        let module = validated_continuation();
        let mut source = active(81, 9);
        let mut storage = Storage::new();
        let namespace = source.namespace().storage_namespace().unwrap();
        let mut tx = storage.transaction(namespace);
        tx.write(b"k", &[41]).unwrap();
        assert_eq!(tx.commit(), 1);
        let mut instance = running_instance(&source, &module, storage);
        let capture = Snapshot::capture(&source, &mut instance, "continue", &[]).unwrap();
        let (edge, proof) = snapshot_transition(&source, capture.digest().unwrap(), 13);
        let snapshot = Snapshot::commit(&mut source, &mut instance, capture, edge, proof).unwrap();
        let bytes = instance
            .storage_snapshot()
            .unwrap()
            .replay_state_bytes(1 << 24)
            .unwrap();
        let directory = std::env::temp_dir().join(format!("layerx-restore-{}", std::process::id()));
        std::fs::create_dir_all(&directory).unwrap();
        let path = directory.join("storage");
        std::fs::write(&path, &bytes).unwrap();
        drop(instance);
        let mut reopened =
            Storage::decode_untrusted_replay_state(&std::fs::read(&path).unwrap(), 1 << 24)
                .unwrap();
        source = Lease::decode_state(&source.canonical_state_bytes().unwrap()).unwrap();
        let supplied =
            SandboxState::from_canonical(&source, &snapshot.state().canonical_bytes().unwrap())
                .unwrap();
        let mut target = funded(82, 9);
        let grants = CapabilitySet::new([
            Capability::StorageRead,
            Capability::StorageWrite,
            Capability::SharedStorageRead,
            Capability::SharedStorageWrite,
            Capability::EmitEvent,
            Capability::Transfer402 {
                asset: [8; 32],
                to: [8; 32],
                maximum_amount: 100,
            },
            Capability::Call {
                program: ProgramId::new([8; 32]).unwrap(),
            },
        ])
        .unwrap();
        let (activation, evidence) = activation(&target, 22);
        let original_target = target.clone();
        let original_storage = reopened.replay_state_bytes(1 << 24).unwrap();
        assert!(matches!(
            restore(
                &source,
                &mut target,
                &mut reopened,
                &mut meter(),
                RestoreRequest {
                    snapshot_digest: snapshot.digest(),
                    supplied: supplied.clone(),
                    authorization: AuthorizationContext::new(
                        PrincipalId::new([10; 32]).unwrap(),
                        grants.clone(),
                    ),
                    module: &module,
                    activation,
                    evidence,
                },
            ),
            Err(SnapshotRefusal::NotSnapshotOwner)
        ));
        assert_eq!(target, original_target);
        assert_eq!(
            reopened.replay_state_bytes(1 << 24).unwrap(),
            original_storage
        );
        let authorization = AuthorizationContext::new(target.tenant(), grants);
        let restored = restore(
            &source,
            &mut target,
            &mut reopened,
            &mut meter(),
            RestoreRequest {
                snapshot_digest: snapshot.digest(),
                supplied,
                authorization,
                module: &module,
                activation,
                evidence,
            },
        )
        .unwrap();
        assert_eq!(restored.outputs(), &[WasmValue::I32(8)]);
        let target_ns = target.namespace().storage_namespace().unwrap();
        assert_eq!(
            reopened.transaction(target_ns).read(b"k").unwrap(),
            Some(vec![77])
        );
        assert_eq!(
            reopened.transaction(namespace).read(b"k").unwrap(),
            Some(vec![41])
        );
        assert_eq!(
            reopened
                .transaction(StorageNamespace::principal(
                    target.host_program(),
                    target.tenant()
                ))
                .read(b"k")
                .unwrap(),
            None
        );
        assert_eq!(
            reopened
                .transaction(StorageNamespace::shared(target.host_program()))
                .read(b"k")
                .unwrap(),
            None
        );
        let bytes = reopened.replay_state_bytes(1 << 24).unwrap();
        drop(reopened);
        std::fs::write(&path, bytes).unwrap();
        let mut reopened =
            Storage::decode_untrusted_replay_state(&std::fs::read(&path).unwrap(), 1 << 24)
                .unwrap();
        assert_eq!(
            reopened.transaction(target_ns).read(b"k").unwrap(),
            Some(vec![77])
        );
        std::fs::remove_dir_all(directory).unwrap();
    }
}
