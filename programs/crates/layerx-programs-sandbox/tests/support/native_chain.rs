trait Checked<T> {
    fn checked(self) -> T;
}

impl<T, E: core::fmt::Debug> Checked<T> for Result<T, E> {
    #[track_caller]
    fn checked(self) -> T {
        match self {
            Ok(value) => value,
            Err(error) => panic!("{error:?}"),
        }
    }
}

impl<T> Checked<T> for Option<T> {
    #[track_caller]
    fn checked(self) -> T {
        match self {
            Some(value) => value,
            None => panic!("expected a value"),
        }
    }
}

#[allow(dead_code)]
pub(crate) mod chain {
    include!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../layerx-programs-registry/tests/support/chain.rs"
    ));
}

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    decode::{decode_commitment_proof, CommitmentProof, ProofOp},
    header::ChainHeader,
    merkle::{hash_from_byte_slices, SimpleProof},
    proto::{ProtoReader, ProtoValue},
    results::TxResult,
    vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
    Commit,
};
use layerx_programs_sandbox::{
    activity::SandboxInclusionProof,
    invocation::{AuthenticatedSandboxReceiptEvidence, SandboxInvocationEvidence},
    sandbox_state_key, sandbox_terminal_key, LeaseId, SandboxProtocolContext,
    SandboxReceiptEvidence, SandboxSubmission, TerminalReceiptEvidence,
};
use sha2::{Digest, Sha256};
use std::{fs, path::Path};

pub(crate) const SEED: [u8; 32] = [17; 32];
pub(crate) const NOW: u64 = 1_700_000_001_123;

pub(crate) fn read(name: &str) -> Vec<u8> {
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
pub(crate) fn signed(header: &ChainHeader, mut commit: Commit) -> Commit {
    commit.height = header.height;
    commit.block_id.hash = header.hash().checked().to_vec();
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
    .checked();
    commit.signatures = vec![CommitSig {
        flag: BlockIdFlag::Commit,
        validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
        timestamp,
        signature: key.sign(&bytes).to_bytes().to_vec(),
    }];
    commit
}
pub(crate) fn varint(mut n: u64, out: &mut Vec<u8>) {
    while n >= 128 {
        out.push(u8::try_from(n & 127).checked() | 128);
        n >>= 7;
    }
    out.push(u8::try_from(n).checked());
}
pub(crate) fn field(n: u8, value: &[u8], out: &mut Vec<u8>) {
    out.push(n << 3 | 2);
    varint(value.len() as u64, out);
    out.extend_from_slice(value);
}
pub(crate) fn ops(encoded: &[u8]) -> Vec<ProofOp> {
    let mut reader = ProtoReader::new(encoded);
    let mut out = Vec::new();
    while let Some(f) = reader.next_field().checked() {
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
        while let Some(f) = r.next_field().checked() {
            let ProtoValue::Bytes(b) = f.value else {
                panic!("proof op field")
            };
            match f.number {
                1 => op.op_type = String::from_utf8(b.to_vec()).checked(),
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
pub(crate) fn commitment(
    fixture: &chain::ChainFixture,
    prefix: &str,
    label: &str,
    store: &str,
    key: Vec<u8>,
) -> CommitmentProof {
    let mut proof = decode_commitment_proof(&fixture.encode().checked()).checked();
    proof.store_name.clear();
    proof.store_name.push_str(store);
    proof.key = key;
    proof.value = Some(read(&format!("{prefix}.{label}.value")));
    proof.proof_ops = ops(&read(&format!("{prefix}.{label}.ops")));
    proof
}
pub(crate) fn inclusion(leaf: &[u8]) -> SimpleProof {
    SimpleProof {
        total: 1,
        index: 0,
        leaf_hash: hash_from_byte_slices(&[leaf]),
        aunts: Vec::new(),
    }
}

pub(crate) fn evidence(
    name: &str,
) -> (
    chain::ChainFixture,
    SandboxSubmission,
    AuthenticatedSandboxReceiptEvidence,
) {
    let submission = SandboxSubmission::decode(
        &read(&format!("{name}.payload")),
        SandboxProtocolContext::new(3, 4).checked(),
    )
    .checked();
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
    .checked();
    fixture.header.height = 101;
    fixture.header.time.seconds = 1_700_000_001;
    fixture.header.time.nanos = 123_000_000;
    let mut current = fixture.header.clone();
    current.height = 100;
    current.time.seconds -= 1;
    current.app_hash = read(&format!("{name}.before.root"));
    current.data_hash = hash_from_byte_slices(&[&tx_hash]).to_vec();
    let current_commit = signed(&current, fixture.commit.clone());
    fixture.header.last_block_id.hash = current.hash().checked().to_vec();
    fixture.header.last_results_hash = hash_from_byte_slices(&[&result_leaf]).to_vec();
    fixture.header.app_hash = read(&format!("{name}.after.root"));
    fixture.commit = signed(&fixture.header, fixture.commit.clone());
    let kind = if name == "sandboxExecute" { 2 } else { 0 };
    let proven = commitment(
        &fixture,
        &format!("{name}.after"),
        "state",
        "lxphost",
        sandbox_state_key(kind, submission.lease().bytes()).checked(),
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
            sandbox_state_key(0, submission.lease().bytes()).checked(),
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

pub(crate) fn terminal_evidence() -> (chain::ChainFixture, LeaseId, TerminalReceiptEvidence) {
    let mut fixture = chain::ChainFixture::new_in_store(
        "lxphost",
        "repair-native",
        &[SEED],
        vec![(b"encoder".to_vec(), vec![1])],
    )
    .checked();
    fixture.header.height = 101;
    fixture.header.time.seconds = 1_700_000_001;
    fixture.header.time.nanos = 123_000_000;
    fixture.header.app_hash = read("terminal.root");
    fixture.commit = signed(&fixture.header, fixture.commit.clone());
    let recorded_key = read("terminal.state.key");
    let lease = LeaseId::new(recorded_key[5..].try_into().checked()).checked();
    assert_eq!(recorded_key, sandbox_terminal_key(lease.bytes()));
    let proof = commitment(&fixture, "terminal", "state", "lxphost", recorded_key);
    let evidence = TerminalReceiptEvidence {
        terminal_ops: proof.proof_ops.clone(),
        commitment: proof,
    };
    (fixture, lease, evidence)
}

// Deliberately re-prove hostile state so refusal tests reach semantic checks,
// rather than merely failing an unrelated stale membership proof.
pub(crate) fn reprove_state(
    evidence: &mut AuthenticatedSandboxReceiptEvidence,
    store: &str,
    key: &[u8],
    value: Vec<u8>,
) {
    let mut fixture = chain::ChainFixture::new_in_store(
        store,
        "repair-native",
        &[SEED],
        vec![(key.to_vec(), value)],
    )
    .checked();
    let mut header = evidence.chain.result_proof.header.clone();
    header.app_hash.clone_from(&fixture.header.app_hash);
    fixture.header.clone_from(&header);
    fixture.commit = signed(&header, fixture.commit.clone());
    evidence.chain.commitment =
        decode_commitment_proof(&fixture.encode_key(key).checked()).checked();
    evidence.chain.usage_ops = evidence.chain.commitment.proof_ops.clone();
    evidence.chain.result_proof.header = header;
    evidence.chain.result_proof.commit = fixture.commit;
}

pub(crate) fn frames() -> Vec<Vec<u8>> {
    let bytes = read("usage.frames");
    let mut input = bytes.as_slice();
    let mut out = Vec::new();
    while !input.is_empty() {
        let length = u32::from_be_bytes(input[..4].try_into().checked()) as usize;
        out.push(input[4..4 + length].to_vec());
        input = &input[4 + length..];
    }
    out
}
