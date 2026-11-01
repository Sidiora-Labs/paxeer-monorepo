use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    decode::{decode_commitment_proof, CommitmentProof, ProofOp},
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, PartSetHeader, Timestamp},
};
use layerx_programs::ProtocolDeploymentVerifier;
use layerx_programs_sandbox::{
    sandbox_state_key, AuthenticatedTerminalRecord, ExpiryRefusal, LeaseId, TerminalReceiptEvidence,
};
use sha2::{Digest, Sha256};

const NOW_MS: u64 = 1_700_000_000_000;
const MAX_AGE_MS: u64 = 1_000;

fn varint(mut value: u64, output: &mut Vec<u8>) {
    while value >= 128 {
        output.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    output.push(value as u8);
}

fn scalar(number: u8, value: u64, output: &mut Vec<u8>) {
    if value != 0 {
        output.push(number << 3);
        varint(value, output);
    }
}

fn field(number: u8, value: &[u8], output: &mut Vec<u8>) {
    output.push(number << 3 | 2);
    varint(value.len() as u64, output);
    output.extend_from_slice(value);
}

fn leaf_hash(prefix: &[u8], key: &[u8], value: &[u8]) -> [u8; 32] {
    let mut bytes = prefix.to_vec();
    varint(key.len() as u64, &mut bytes);
    bytes.extend_from_slice(key);
    varint(32, &mut bytes);
    bytes.extend_from_slice(&Sha256::digest(value));
    Sha256::digest(bytes).into()
}

fn existence(key: &[u8], value: &[u8], prefix: &[u8]) -> Vec<u8> {
    let mut proof = Vec::new();
    field(1, key, &mut proof);
    field(2, value, &mut proof);
    let mut leaf = vec![8, 1, 24, 1, 32, 1];
    field(5, prefix, &mut leaf);
    field(3, &leaf, &mut proof);
    let mut commitment = Vec::new();
    field(1, &proof, &mut commitment);
    commitment
}

fn encode_header(header: &ChainHeader) -> Vec<u8> {
    let mut version = Vec::new();
    scalar(1, header.version.block, &mut version);
    scalar(2, header.version.app, &mut version);
    let mut time = Vec::new();
    scalar(1, header.time.seconds as u64, &mut time);
    scalar(2, header.time.nanos as u64, &mut time);
    let mut parts = Vec::new();
    scalar(
        1,
        u64::from(header.last_block_id.part_set_total),
        &mut parts,
    );
    field(2, &header.last_block_id.part_set_hash, &mut parts);
    let mut block = Vec::new();
    field(1, &header.last_block_id.hash, &mut block);
    field(2, &parts, &mut block);
    let mut bytes = Vec::new();
    field(1, &version, &mut bytes);
    field(2, header.chain_id.as_bytes(), &mut bytes);
    scalar(3, header.height as u64, &mut bytes);
    field(4, &time, &mut bytes);
    field(5, &block, &mut bytes);
    for (number, hash) in [
        (6, &header.last_commit_hash),
        (7, &header.data_hash),
        (8, &header.validators_hash),
        (9, &header.next_validators_hash),
        (10, &header.consensus_hash),
        (11, &header.app_hash),
        (12, &header.last_results_hash),
        (13, &header.evidence_hash),
        (14, &header.proposer_address),
    ] {
        field(number, hash, &mut bytes);
    }
    bytes
}

fn evidence(
    key_lease: LeaseId,
    record: Vec<u8>,
    kind: u8,
    signing_key: &SigningKey,
) -> (TerminalReceiptEvidence, TrustedValidator, [u8; 32]) {
    let key = sandbox_state_key(kind, key_lease.bytes())
        .unwrap_or_else(|error| panic!("sandbox key: {error}"));
    let store_root = leaf_hash(&[0, 2, 2], &key, &record);
    let app_hash = leaf_hash(&[0], b"lxphost", &store_root);
    let ops = vec![
        ProofOp {
            op_type: "ics23:iavl".to_owned(),
            key: key.clone(),
            data: existence(&key, &record, &[0, 2, 2]),
        },
        ProofOp {
            op_type: "ics23:simple".to_owned(),
            key: b"lxphost".to_vec(),
            data: existence(b"lxphost", &store_root, &[0]),
        },
    ];
    let validator = TrustedValidator::new(signing_key.verifying_key().to_bytes(), 10);
    let validators = ValidatorSet::new(vec![validator.clone()])
        .unwrap_or_else(|error| panic!("validators: {error:?}"));
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-expiry-test".to_owned(),
        height: 38,
        time: HeaderTime {
            seconds: (NOW_MS / 1_000) as i64,
            nanos: 0,
        },
        last_block_id: HeaderBlockId {
            hash: vec![1; 32],
            part_set_total: 1,
            part_set_hash: vec![2; 32],
        },
        last_commit_hash: vec![3; 32],
        data_hash: vec![4; 32],
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: validators.hash().to_vec(),
        consensus_hash: vec![5; 32],
        app_hash: app_hash.to_vec(),
        last_results_hash: vec![6; 32],
        evidence_hash: vec![7; 32],
        proposer_address: validator.address.to_vec(),
    };
    let header_hash = header
        .hash()
        .unwrap_or_else(|error| panic!("header hash: {error:?}"));
    let block_id = BlockId {
        hash: header_hash.to_vec(),
        part_set_header: PartSetHeader {
            total: 1,
            hash: vec![8; 32],
        },
    };
    let timestamp = Timestamp {
        seconds: header.time.seconds,
        nanos: 0,
    };
    let vote = vote_sign_bytes(
        &header.chain_id,
        header.height,
        0,
        Some(&block_id),
        timestamp,
    )
    .unwrap_or_else(|error| panic!("vote: {error:?}"));
    let mut time = Vec::new();
    scalar(1, timestamp.seconds as u64, &mut time);
    let mut signature = Vec::new();
    scalar(1, 2, &mut signature);
    field(2, &validator.address, &mut signature);
    field(3, &time, &mut signature);
    field(4, &signing_key.sign(&vote).to_bytes(), &mut signature);
    let mut parts = Vec::new();
    scalar(1, u64::from(block_id.part_set_header.total), &mut parts);
    field(2, &block_id.part_set_header.hash, &mut parts);
    let mut block = Vec::new();
    field(1, &block_id.hash, &mut block);
    field(2, &parts, &mut block);
    let mut commit = Vec::new();
    scalar(1, header.height as u64, &mut commit);
    field(3, &block, &mut commit);
    field(4, &signature, &mut commit);
    (
        TerminalReceiptEvidence {
            commitment: CommitmentProof {
                signed_header: encode_header(&header),
                commit,
                validator_set: Vec::new(),
                store_name: "lxphost".to_owned(),
                key,
                value: Some(record),
                proof_ops: ops.clone(),
            },
            terminal_ops: ops,
        },
        validator,
        header_hash,
    )
}

fn terminal_record(lease: LeaseId) -> Vec<u8> {
    let mut record = vec![0; 352];
    record[..5].copy_from_slice(b"LXSD1");
    record[5..37].copy_from_slice(&lease.bytes());
    record[37..69].copy_from_slice(&[11; 32]);
    record[69..77].copy_from_slice(&7_u64.to_be_bytes());
    record[77..85].copy_from_slice(&36_u64.to_be_bytes());
    record[85..117].copy_from_slice(&[12; 32]);
    record[117..149].copy_from_slice(&[13; 32]);
    record
}

#[test]
fn terminal_record_verifies_from_proven_state() {
    let lease = LeaseId::new([1; 32]).unwrap_or_else(|error| panic!("lease: {error}"));
    let other_lease = LeaseId::new([2; 32]).unwrap_or_else(|error| panic!("other lease: {error}"));
    let signing_key = SigningKey::from_bytes(&[17; 32]);
    let record = terminal_record(lease);
    let (valid, validator, header_hash) = evidence(lease, record.clone(), 2, &signing_key);
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        "paxeer-expiry-test".to_owned(),
        vec![validator],
        MAX_AGE_MS,
    )
    .unwrap_or_else(|error| panic!("chain trust: {error}"));
    let authenticated = AuthenticatedTerminalRecord::verify(&verifier, &valid, NOW_MS, lease)
        .unwrap_or_else(|error| panic!("terminal record: {error}"));
    assert_eq!(authenticated.canonical_bytes().as_slice(), record);
    assert_eq!(authenticated.canonical_bytes().len(), 352);
    assert_eq!(authenticated.lease(), lease);
    assert_eq!(authenticated.destroy_activity_id(), [11; 32]);
    assert_eq!(authenticated.receipt_activity_id(), header_hash);
    assert_ne!(
        authenticated.receipt_activity_id(),
        authenticated.destroy_activity_id()
    );

    let refuse = |proof: &TerminalReceiptEvidence, expected, now_ms| {
        assert_eq!(
            AuthenticatedTerminalRecord::verify(&verifier, proof, now_ms, expected),
            Err(ExpiryRefusal::InvalidProtocolReceipt)
        );
    };
    refuse(&valid, other_lease, NOW_MS);
    let (other, _, _) = evidence(other_lease, terminal_record(other_lease), 2, &signing_key);
    refuse(&other, lease, NOW_MS);
    let (wrong_record, _, _) = evidence(lease, terminal_record(other_lease), 2, &signing_key);
    refuse(&wrong_record, lease, NOW_MS);
    let mut wrong_proof = valid.clone();
    wrong_proof.terminal_ops = other.terminal_ops.clone();
    refuse(&wrong_proof, lease, NOW_MS);
    let mut wrong_root = valid.clone();
    wrong_root.terminal_ops = wrong_record.terminal_ops.clone();
    refuse(&wrong_root, lease, NOW_MS);
    for kind in 0..2 {
        let (wrong_kind, _, _) = evidence(lease, record.clone(), kind, &signing_key);
        refuse(&wrong_kind, lease, NOW_MS);
    }
    let mut tampered = valid.clone();
    tampered
        .commitment
        .value
        .as_mut()
        .unwrap_or_else(|| panic!("terminal value"))[85] ^= 1;
    refuse(&tampered, lease, NOW_MS);
    let mut tampered = valid.clone();
    *tampered
        .commitment
        .signed_header
        .last_mut()
        .unwrap_or_else(|| panic!("header")) ^= 1;
    refuse(&tampered, lease, NOW_MS);
    let mut tampered = valid.clone();
    *tampered
        .commitment
        .commit
        .last_mut()
        .unwrap_or_else(|| panic!("signature")) ^= 1;
    refuse(&tampered, lease, NOW_MS);
    let (untrusted, _, _) = evidence(lease, record.clone(), 2, &SigningKey::from_bytes(&[18; 32]));
    refuse(&untrusted, lease, NOW_MS);
    let mut wrong_store = valid.clone();
    wrong_store.commitment.store_name = "bank".to_owned();
    refuse(&wrong_store, lease, NOW_MS);
    let mut no_proof = valid.clone();
    no_proof.terminal_ops.clear();
    refuse(&no_proof, lease, NOW_MS);
    let mut absent = valid.clone();
    absent.commitment.value = None;
    refuse(&absent, lease, NOW_MS);
    for length in [351, 353] {
        let mut malformed = record.clone();
        malformed.resize(length, 0);
        let (proof, _, _) = evidence(lease, malformed, 2, &signing_key);
        refuse(&proof, lease, NOW_MS);
    }
    for range in [0..5, 37..69, 69..77, 77..85, 117..149] {
        let mut malformed = record.clone();
        malformed[range].fill(0);
        let (proof, _, _) = evidence(lease, malformed, 2, &signing_key);
        refuse(&proof, lease, NOW_MS);
    }
    refuse(&valid, lease, NOW_MS + MAX_AGE_MS + 1);
    refuse(&valid, lease, NOW_MS - 1_001);
    assert!(
        AuthenticatedTerminalRecord::verify(&verifier, &valid, NOW_MS + MAX_AGE_MS, lease).is_ok()
    );
    assert!(AuthenticatedTerminalRecord::verify(&verifier, &valid, NOW_MS - 1_000, lease).is_ok());
}

#[test]
fn terminal_record_refuses_kernel_receipt_bytes() {
    let kernel_receipt =
        include_bytes!("../../layerx-programs-registry/tests/fixtures/maintained-head/receipt");
    let kernel_key = *include_bytes!(
        "../../layerx-programs-registry/tests/fixtures/maintained-head/sequencer.public"
    );
    let receipt =
        layerx_proof::receipt::verify_sequencer_signature(kernel_receipt.as_slice(), kernel_key)
            .unwrap_or_else(|error| panic!("signed kernel receipt: {error:?}"));
    assert!(receipt
        .protocol()
        .unwrap_or_else(|| panic!("kernel protocol receipt"))
        .sequencer_signature()
        .is_some());
    assert_eq!(
        decode_commitment_proof(kernel_receipt),
        Err(layerx_programs::chain_proof::ChainProofError::NonCanonical)
    );

    let lease = LeaseId::new([1; 32]).unwrap_or_else(|error| panic!("lease: {error}"));
    let signing_key = SigningKey::from_bytes(&[17; 32]);
    let record = terminal_record(lease);
    let (valid, validator, header_hash) = evidence(lease, record.clone(), 2, &signing_key);
    let verifier = ProtocolDeploymentVerifier::from_chain_trust(
        "paxeer-expiry-test".to_owned(),
        vec![validator],
        MAX_AGE_MS,
    )
    .unwrap_or_else(|error| panic!("chain trust: {error}"));
    let authenticated = AuthenticatedTerminalRecord::verify(&verifier, &valid, NOW_MS, lease)
        .unwrap_or_else(|error| panic!("terminal record: {error}"));
    assert_eq!(authenticated.canonical_bytes().as_slice(), record);
    assert_eq!(authenticated.receipt_activity_id(), header_hash);

    let refuse = |proof: &TerminalReceiptEvidence| {
        assert_eq!(
            AuthenticatedTerminalRecord::verify(&verifier, proof, NOW_MS, lease),
            Err(ExpiryRefusal::InvalidProtocolReceipt)
        );
    };
    let mut kernel_header = valid.clone();
    kernel_header.commitment.signed_header = kernel_receipt.to_vec();
    refuse(&kernel_header);
    let mut kernel_commit = valid.clone();
    kernel_commit.commitment.commit = kernel_receipt.to_vec();
    refuse(&kernel_commit);
    for index in 0..valid.terminal_ops.len() {
        let mut kernel_proof = valid.clone();
        kernel_proof.terminal_ops[index].data = kernel_receipt.to_vec();
        refuse(&kernel_proof);
    }
    let (proven_kernel_receipt, _, _) = evidence(lease, kernel_receipt.to_vec(), 2, &signing_key);
    refuse(&proven_kernel_receipt);
}
