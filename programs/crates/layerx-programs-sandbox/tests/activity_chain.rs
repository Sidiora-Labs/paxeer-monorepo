#[path = "../../layerx-programs-registry/tests/support/chain.rs"]
mod chain;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    decode::decode_commitment_proof,
    header::ChainHeader,
    merkle::{hash_from_byte_slices, SimpleProof},
    results::TxResult,
    vote::{vote_sign_bytes, BlockIdFlag, CommitSig, Timestamp},
    ChainProofError, Commit,
};
use layerx_programs_sandbox::{
    activity::SandboxInclusionProof, sandbox_state_key, SandboxActivityRefusal,
    SandboxProtocolContext, SandboxReceiptEvidence, SandboxSubmission, SandboxSubmissionReceipt,
    UsageReceipt, UsageRefusal, PROGRAMS_SANDBOX_ACTIVITY_TYPE,
};
use sha2::{Digest, Sha256};

#[derive(Debug)]
enum TestError {
    Chain(ChainProofError),
    Activity(SandboxActivityRefusal),
    Usage(UsageRefusal),
    Missing(&'static str),
}

impl From<ChainProofError> for TestError {
    fn from(error: ChainProofError) -> Self {
        Self::Chain(error)
    }
}
impl From<SandboxActivityRefusal> for TestError {
    fn from(error: SandboxActivityRefusal) -> Self {
        Self::Activity(error)
    }
}
impl From<UsageRefusal> for TestError {
    fn from(error: UsageRefusal) -> Self {
        Self::Usage(error)
    }
}
impl From<&'static str> for TestError {
    fn from(error: &'static str) -> Self {
        Self::Missing(error)
    }
}

const SEED: [u8; 32] = [17; 32];
const NOW: u64 = 1_700_000_001_000;

fn submission() -> Result<SandboxSubmission, SandboxActivityRefusal> {
    let mut call = vec![0; 106];
    call[..32].fill(3);
    call[32..34].copy_from_slice(&2_u16.to_be_bytes());
    call[34..36].copy_from_slice(&3_u16.to_be_bytes());
    call[40..42].copy_from_slice(&4_u16.to_be_bytes());
    for limit in call[50..106].chunks_exact_mut(8) {
        limit.copy_from_slice(&1_u64.to_be_bytes());
    }
    call.extend_from_slice(b"run");
    call.extend_from_slice(&[0, 2, 1, 2]);
    let mut payload = vec![0; 236];
    payload[..4].copy_from_slice(&[1, 1, 0, 0]);
    payload[4..36].fill(1);
    payload[36..44].copy_from_slice(&1_u64.to_be_bytes());
    payload[44..76].fill(2);
    payload[76..108].fill(4);
    payload[108..140].fill(5);
    payload[140..172].fill(6);
    payload[172..176].copy_from_slice(&1_u32.to_be_bytes());
    payload[232..236].copy_from_slice(
        &u32::try_from(call.len())
            .map_err(|_| SandboxActivityRefusal::LengthLimit)?
            .to_be_bytes(),
    );
    payload.extend_from_slice(&call);
    SandboxSubmission::decode(&payload, SandboxProtocolContext::new(2, 3)?)
}

fn usage_value(submission: &SandboxSubmission, tx_hash: [u8; 32]) -> Vec<u8> {
    let mut canonical = b"LayerX/programs/sandbox/usage-receipt/v4\0".to_vec();
    canonical.extend_from_slice(&submission.lease().bytes());
    canonical.extend_from_slice(&submission.expected_usage_sequence().to_be_bytes());
    canonical.extend_from_slice(&37_u64.to_be_bytes());
    canonical.extend_from_slice(&tx_hash);
    canonical.extend_from_slice(&[7; 32]);
    canonical.extend_from_slice(&submission.expected_lease_digest());
    canonical.extend_from_slice(&[8; 32]);
    canonical.extend_from_slice(&[6; 32]);
    canonical.extend_from_slice(&[0; 64]);
    canonical.push(1);
    canonical.extend_from_slice(&[3; 32]);
    for value in [1_u64, 0, 0, 0, 0, 0] {
        canonical.extend_from_slice(&value.to_be_bytes());
    }
    for value in [0_u128, 0, 1] {
        canonical.extend_from_slice(&value.to_be_bytes());
    }
    for value in [1_u64, 0, 0, 0, 0, 0, 0, 0] {
        canonical.extend_from_slice(&value.to_be_bytes());
    }
    canonical.extend_from_slice(&1_u32.to_be_bytes());
    for value in [1_u64, 0, 0, 0, 0, 0, 0] {
        canonical.extend_from_slice(&value.to_be_bytes());
    }
    canonical.extend_from_slice(&1_u128.to_be_bytes());
    canonical.extend_from_slice(&1_u128.to_be_bytes());
    canonical.extend_from_slice(&[9; 32]);
    let digest: [u8; 32] = Sha256::digest(&canonical).into();
    let decoded = UsageReceipt::decode(&canonical, digest)
        .unwrap_or_else(|error| panic!("usage fixture must verify: {error:?}"));
    assert_eq!(decoded.canonical_bytes(), canonical);
    let mut value = b"LXUR".to_vec();
    value.extend_from_slice(&canonical);
    value.extend_from_slice(&digest);
    value
}

fn signed_commit(header: &ChainHeader, mut commit: Commit) -> Result<Commit, ChainProofError> {
    let key = SigningKey::from_bytes(&SEED);
    commit.height = header.height;
    commit.block_id.hash = header.hash()?.to_vec();
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
    )?;
    commit.signatures = vec![CommitSig {
        flag: BlockIdFlag::Commit,
        validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
        timestamp,
        signature: key.sign(&bytes).to_bytes().to_vec(),
    }];
    Ok(commit)
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
    submission: &SandboxSubmission,
    code: u32,
) -> Result<(chain::ChainFixture, SandboxReceiptEvidence), TestError> {
    let tx = submission.payload().to_vec();
    let tx_hash: [u8; 32] = Sha256::digest(&tx).into();
    let value = usage_value(submission, tx_hash);
    let key = sandbox_state_key(1, submission.lease().bytes())?;
    let foreign_key = sandbox_state_key(1, [99; 32])?;
    let mut fixture = chain::ChainFixture::new(
        "sandbox-chain-test",
        &[SEED],
        vec![(key.clone(), value.clone()), (foreign_key, value)],
    )?;
    let mut header = fixture.header.clone();
    header.height -= 1;
    header.time.seconds -= 1;
    header.data_hash = hash_from_byte_slices(&[&tx_hash]).to_vec();
    let commit = signed_commit(&header, fixture.commit.clone())?;
    let result_leaf = if code == 0 { vec![] } else { vec![0x08, 5] };
    fixture.header.last_block_id.hash = header.hash()?.to_vec();
    fixture.header.last_results_hash = hash_from_byte_slices(&[&result_leaf]).to_vec();
    fixture.commit = signed_commit(&fixture.header, fixture.commit.clone())?;
    let commitment = decode_commitment_proof(&fixture.encode_key(&key)?)?;
    let evidence = SandboxReceiptEvidence {
        tx,
        tx_proof: SandboxInclusionProof {
            header,
            commit,
            proof: inclusion(&tx_hash),
        },
        tx_result: TxResult {
            code,
            ..TxResult::default()
        },
        result_proof: SandboxInclusionProof {
            header: fixture.header.clone(),
            commit: fixture.commit.clone(),
            proof: inclusion(&result_leaf),
        },
        usage_ops: commitment.proof_ops.clone(),
        commitment,
    };
    Ok((fixture, evidence))
}

#[test]
fn sandbox_receipt_verifies_tx_result_and_usage_state() -> Result<(), TestError> {
    let submission = submission()?;
    assert_eq!(submission.activity_type(), PROGRAMS_SANDBOX_ACTIVITY_TYPE);
    assert_eq!(PROGRAMS_SANDBOX_ACTIVITY_TYPE, 0x0009_0009);
    let (fixture, evidence) = evidence(&submission, 0)?;
    let expected_hash: [u8; 32] = Sha256::digest(&evidence.tx).into();
    let expected_value = evidence.commitment.value.clone().ok_or("usage value")?;
    let receipt =
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, evidence.clone(), NOW)?;
    assert_eq!(receipt.activity_id(), expected_hash);
    assert_eq!(receipt.lease(), submission.lease());
    assert_eq!(receipt.canonical_receipt(), expected_value);
    let authenticated = receipt.authenticated_usage_receipt()?;
    let usage = authenticated.receipt();
    assert_eq!(usage.activity_id(), expected_hash);
    assert_eq!(usage.lease(), submission.lease());
    assert_eq!(usage.sequence(), submission.expected_usage_sequence());
    assert_eq!(
        usage.expected_lease_digest(),
        submission.expected_lease_digest()
    );
    assert_eq!(usage.charged(), 1);
    usage.verify()?;

    let mut changed = evidence.clone();
    changed.tx.push(0);
    assert_eq!(
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, changed, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
    let mut changed = evidence.clone();
    changed.result_proof.commit.signatures[0].signature[0] ^= 1;
    assert_eq!(
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, changed, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
    let mut changed = evidence.clone();
    changed.tx_result.data.push(1);
    assert_eq!(
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, changed, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
    let mut changed = evidence.clone();
    changed.commitment.value.as_mut().ok_or("usage value")?[4] ^= 1;
    assert_eq!(
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, changed, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
    let mut changed = evidence;
    changed.result_proof.proof.index = 1;
    assert_eq!(
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, changed, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
    Ok(())
}

#[test]
fn sandbox_receipt_refuses_failed_tx_and_foreign_lease() -> Result<(), TestError> {
    let submission = submission()?;
    let (failed_fixture, failed) = evidence(&submission, 5)?;
    assert_eq!(
        SandboxSubmissionReceipt::verify(&failed_fixture.verifier, &submission, failed, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
    let (fixture, mut foreign) = evidence(&submission, 0)?;
    let foreign_key = sandbox_state_key(1, [99; 32])?;
    foreign.commitment = decode_commitment_proof(&fixture.encode_key(&foreign_key)?)?;
    foreign.usage_ops = foreign.commitment.proof_ops.clone();
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)?;
    layerx_programs::verify_state(
        &head,
        "layerx",
        &foreign_key,
        foreign.commitment.value.as_deref().ok_or("usage value")?,
        &foreign.usage_ops,
    )?;
    assert_eq!(
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, foreign, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
    Ok(())
}
