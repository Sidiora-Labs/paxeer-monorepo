#[path = "support/native_chain.rs"]
mod native;

use layerx_programs_sandbox::{
    activity::canonical_fund_for_did, invocation::AuthenticatedSandboxReceiptEvidence,
    sandbox_state_key, Lease, LeaseState, SandboxActivityRefusal, SandboxProtocolContext,
    SandboxSubmission, SandboxSubmissionReceipt, UsageLedger, PROGRAMS_SANDBOX_ACTIVITY_TYPE,
};
use native::{evidence, frames, read, reprove_state, NOW};
use sha2::{Digest, Sha256};

fn refuse(
    fixture: &native::chain::ChainFixture,
    submission: &SandboxSubmission,
    evidence: AuthenticatedSandboxReceiptEvidence,
) {
    assert_eq!(
        SandboxSubmissionReceipt::verify(&fixture.verifier, submission, evidence, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
}

#[test]
fn sandbox_funding_receipt_verifies_funded_lease_record() {
    let (fixture, submission, proof) = evidence("sandboxFund");
    let payload = submission.payload();
    let length = u32::from_be_bytes(payload[248..252].try_into().unwrap()) as usize;
    let requested = Lease::decode_state(&payload[252..252 + length]).unwrap();
    let tenant_key = proof
        .invocation
        .signer_binding
        .value
        .as_ref()
        .unwrap()
        .as_slice()
        .try_into()
        .unwrap();
    let constructed = canonical_fund_for_did(
        SandboxProtocolContext::new(3, 4).unwrap(),
        &requested,
        tenant_key,
        &payload[256 + length..],
    )
    .unwrap();
    assert_eq!(constructed, submission);
    assert_eq!(submission.expected_usage_sequence(), 0);
    assert_ne!(proof.chain.tx, submission.payload());
    assert_eq!(proof.chain.tx, read("sandboxFund.tx"));
    assert_eq!(
        proof.chain.commitment.key,
        read("sandboxFund.after.state.key")
    );
    assert_eq!(
        proof.chain.commitment.key,
        sandbox_state_key(0, requested.id().bytes()).unwrap()
    );
    let receipt =
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, proof.clone(), NOW)
            .unwrap();
    let funded = Lease::decode_state(receipt.canonical_receipt()).unwrap();
    assert_eq!(funded.state(), LeaseState::Funded);
    assert_eq!(
        &funded.history()[..requested.history().len()],
        requested.history()
    );
    assert_eq!(receipt.activity_id().as_slice(), read("sandboxFund.id"));
    assert_eq!(receipt.lease(), requested.id());
    assert_eq!(
        receipt.canonical_receipt(),
        funded.canonical_state_bytes().unwrap()
    );
    assert_eq!(
        receipt.authenticated_usage_receipt(),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );

    // Each malformed record has a valid signed state proof.
    let key = proof.chain.commitment.key.clone();
    let mut changed = proof.clone();
    reprove_state(
        &mut changed,
        "lxphost",
        key.clone(),
        requested.canonical_state_bytes().unwrap(),
    );
    refuse(&fixture, &submission, changed);
    let batch = funded.history().last().unwrap().batch_sequence;
    let foreign_activity = lease_record(
        &requested,
        layerx_programs_sandbox::LeaseActivity::Fund,
        [0x42; 32],
        batch,
    );
    let mut changed = proof.clone();
    reprove_state(
        &mut changed,
        "lxphost",
        key.clone(),
        foreign_activity.canonical_state_bytes().unwrap(),
    );
    refuse(&fixture, &submission, changed);
    let other = Lease::request_with_schedule(
        layerx_programs_sandbox::LeaseId::new([9; 32]).unwrap(),
        requested.tenant(),
        requested.host_program(),
        requested.image_code_hash(),
        requested.escrow_asset(),
        requested.escrow_amount(),
        requested.limits(),
        requested.opened_at(),
        requested.expiry(),
        requested.fee_schedule(),
    )
    .unwrap();
    let other = lease_record(
        &other,
        layerx_programs_sandbox::LeaseActivity::Request,
        [0x31; 32],
        other.opened_at(),
    );
    let foreign_lease = lease_record(
        &other,
        layerx_programs_sandbox::LeaseActivity::Fund,
        receipt.activity_id(),
        batch,
    );
    let mut changed = proof.clone();
    reprove_state(
        &mut changed,
        "lxphost",
        key,
        foreign_lease.canonical_state_bytes().unwrap(),
    );
    refuse(&fixture, &submission, changed);
    let mut changed = proof.clone();
    changed.chain.tx_result.code = 5;
    refuse(&fixture, &submission, changed);
    let mut changed = proof.clone();
    changed.chain.commitment.value.as_mut().unwrap()[0] ^= 1;
    refuse(&fixture, &submission, changed);
    let mut changed = proof;
    changed.chain.usage_ops.clear();
    refuse(&fixture, &submission, changed);
}

#[test]
fn sandbox_activation_receipt_preserves_native_funded_history() {
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
    assert_eq!(active.history().len(), previous.history().len() + 1);
    assert_eq!(
        &active.history()[..previous.history().len()],
        previous.history()
    );
    assert_eq!(
        active.history().last().unwrap().transition.activity_id,
        receipt.activity_id()
    );
    assert_eq!(receipt.activity_id().as_slice(), read("sandboxActivate.id"));
    assert_eq!(
        active.canonical_state_bytes().unwrap(),
        read("sandboxExecute.before.lease.value")
    );
    let mut bad = proof;
    bad.invocation.previous_lease = None;
    refuse(&fixture, &submission, bad);
}

#[test]
fn sandbox_receipt_verifies_tx_result_and_usage_state() {
    let (fixture, submission, proof) = evidence("sandboxExecute");
    assert_eq!(submission.activity_type(), PROGRAMS_SANDBOX_ACTIVITY_TYPE);
    assert_eq!(PROGRAMS_SANDBOX_ACTIVITY_TYPE, 0x0009_0009);
    assert_eq!(proof.chain.tx, read("sandboxExecute.tx"));
    assert_ne!(proof.chain.tx, submission.payload());
    assert_eq!(
        proof.chain.commitment.key,
        read("sandboxExecute.after.state.key")
    );
    assert_eq!(
        proof.chain.commitment.key,
        sandbox_state_key(2, submission.lease().bytes()).unwrap()
    );
    let expected_value = proof.chain.commitment.value.clone().unwrap();
    let receipt =
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, proof.clone(), NOW)
            .unwrap();
    assert_eq!(receipt.activity_id().as_slice(), read("sandboxExecute.id"));
    assert_ne!(
        receipt.activity_id().as_slice(),
        Sha256::digest(&proof.chain.tx).as_slice()
    );
    assert_eq!(receipt.lease(), submission.lease());
    assert_eq!(receipt.canonical_receipt(), expected_value);
    let authenticated = receipt.authenticated_usage_receipt().unwrap();
    let usage = authenticated.receipt();
    let ledger = UsageLedger::decode_canonical(&expected_value).unwrap();
    assert_eq!(ledger.receipt_count(), 1);
    assert_eq!(ledger.latest(), Some(usage));
    assert_eq!(usage.activity_id(), receipt.activity_id());
    assert_eq!(usage.lease(), submission.lease());
    assert_eq!(usage.sequence(), submission.expected_usage_sequence());
    assert_eq!(
        usage.expected_lease_digest(),
        submission.expected_lease_digest()
    );
    let committed = Lease::decode_state(&read("sandboxExecute.after.lease.value")).unwrap();
    assert_eq!(usage.charged(), committed.escrow_consumed());
    assert!(usage.charged() > 0);
    usage.verify().unwrap();
    let original = frames();
    assert!(original.len() > 1);
    receipt.verify_native_usage_frames(&original).unwrap();
    assert!(UsageLedger::decode_canonical(&original[0]).is_err());
    for index in 0..5 {
        let mut changed = proof.clone();
        match index {
            0 => changed.chain.tx.push(0),
            1 => changed.chain.result_proof.commit.signatures[0].signature[0] ^= 1,
            2 => changed.chain.tx_result.data.push(1),
            3 => changed.chain.commitment.value.as_mut().unwrap()[4] ^= 1,
            _ => changed.chain.result_proof.proof.index = 1,
        }
        refuse(&fixture, &submission, changed);
    }
    for index in 0..4 {
        let mut changed = original.clone();
        match index {
            0 => {
                changed.pop();
            }
            1 => changed.reverse(),
            2 => changed[0][4] ^= 1,
            _ => changed[0][48] ^= 1,
        }
        assert!(receipt.verify_native_usage_frames(&changed).is_err());
    }
    // Even a correctly proven event frame is not a native stored usage ledger.
    let mut changed = proof.clone();
    reprove_state(
        &mut changed,
        "lxphost",
        proof.chain.commitment.key.clone(),
        original[0].clone(),
    );
    refuse(&fixture, &submission, changed);
}

#[test]
fn sandbox_receipt_verifies_only_against_mounted_lxphost_store() {
    let (fixture, submission, proof) = evidence("sandboxExecute");
    let receipt =
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, proof.clone(), NOW)
            .unwrap();
    assert_eq!(receipt.activity_id().as_slice(), read("sandboxExecute.id"));
    let mut changed = proof.clone();
    reprove_state(
        &mut changed,
        "layerx",
        proof.chain.commitment.key.clone(),
        proof.chain.commitment.value.clone().unwrap(),
    );
    let head = fixture
        .verifier
        .verify_head(
            &changed.chain.result_proof.header,
            &changed.chain.result_proof.commit,
        )
        .unwrap();
    layerx_programs::verify_state(
        &head,
        "layerx",
        &changed.chain.commitment.key,
        changed.chain.commitment.value.as_ref().unwrap(),
        &changed.chain.usage_ops,
    )
    .unwrap();
    refuse(&fixture, &submission, changed);
}

#[test]
fn sandbox_receipt_refuses_failed_tx_and_foreign_lease() {
    let (fixture, submission, proof) = evidence("sandboxExecute");
    let mut failed = proof.clone();
    failed.chain.tx_result.code = 5;
    refuse(&fixture, &submission, failed);
    for key in [
        sandbox_state_key(2, [99; 32]).unwrap(),
        sandbox_state_key(1, submission.lease().bytes()).unwrap(),
    ] {
        let mut foreign = proof.clone();
        reprove_state(
            &mut foreign,
            "lxphost",
            key.clone(),
            proof.chain.commitment.value.clone().unwrap(),
        );
        let head = fixture
            .verifier
            .verify_head(
                &foreign.chain.result_proof.header,
                &foreign.chain.result_proof.commit,
            )
            .unwrap();
        layerx_programs::verify_state(
            &head,
            "lxphost",
            &key,
            foreign.chain.commitment.value.as_ref().unwrap(),
            &foreign.chain.usage_ops,
        )
        .unwrap();
        refuse(&fixture, &submission, foreign);
    }
    let legacy = proof.chain;
    assert_eq!(
        SandboxSubmissionReceipt::verify(&fixture.verifier, &submission, legacy, NOW),
        Err(SandboxActivityRefusal::InvalidReceipt)
    );
}

fn lease_record(
    lease: &Lease,
    activity: layerx_programs_sandbox::LeaseActivity,
    activity_id: [u8; 32],
    batch: u64,
) -> Lease {
    use layerx_programs_sandbox::LeaseActivity;
    let mut bytes = lease.canonical_state_bytes().unwrap();
    let state_offset = bytes.len() - 5 - lease.history().len() * 107;
    let state = if activity == LeaseActivity::Fund {
        LeaseState::Funded
    } else {
        LeaseState::Requested
    };
    bytes[state_offset] = state as u8;
    bytes[state_offset + 1] = u8::try_from(lease.history().len() + 1).unwrap();
    bytes.truncate(bytes.len() - 3);
    let observation = if activity == LeaseActivity::Request {
        lease.request_binding_digest().unwrap()
    } else {
        [0; 32]
    };
    let transition = [activity as u8, lease.state() as u8, state as u8];
    bytes.extend_from_slice(&transition);
    bytes.extend_from_slice(&activity_id);
    bytes.extend_from_slice(&observation);
    let mut receipt = Sha256::new();
    receipt.update(lease.id().bytes());
    receipt.update(transition);
    receipt.update(activity_id);
    receipt.update(observation);
    receipt.update(batch.to_be_bytes());
    bytes.extend_from_slice(&receipt.finalize());
    bytes.extend_from_slice(&batch.to_be_bytes());
    bytes.extend_from_slice(&[0, 0, 0]);
    Lease::decode_state(&bytes).unwrap()
}
