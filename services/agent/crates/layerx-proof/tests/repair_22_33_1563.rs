//! Repair record 22.33.1563, audit 22.30.98, acceptance 22998.1.
//! Base revision: 8c4a3e7e0bd8b236f61b0f9e0dd18f3cfc360fce.
//! Qualification revision, command, exit code and private log are recorded by hf-done.
//!
//! registry-withdrawal-verifier-removed is proved by
//! `withdrawal_fixture::paid_withdrawal_receipt_verifies_without_the_signed_activity`,
//! `retained_withdrawal_outcome_decodes_the_real_fixture`, and
//! `retained_withdrawal_path_refuses_changed_bytes_and_batch_authority`.
//! Wiring request: domain 9, spec/layerx-merge/interfaces.kvx: retire 9.rs-proof-receipt-withdrawal-verify and its stale caller edges; publish `receipt::verify_outcome` plus `receipt::withdrawal::WithdrawalOutcome` with tests/withdrawal.rs and this integration target as retained consumer evidence.
//!
//! Baseline verification was not run: the explicit gate order allows hf-build
//! only after full implementation and hf-done after that build. No previous gate
//! ledger existed, so no test is claimed to have failed before these changes.
//! Fixture authorization selects the activity transition through the signed
//! maintenance evidence; sealed batch roots also cover the maintenance transition.

use layerx_proof::receipt::{
    verify_outcome, verify_sequencer_signature, withdrawal::WithdrawalOutcome, AuthorizedBatch,
};

#[path = "withdrawal.rs"]
mod withdrawal_fixture;

const RECEIPT: &[u8] =
    include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt");
const KEY: [u8; 32] =
    *include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/sequencer.public");

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> Result<T, String> {
    result.map_err(|error| format!("{error:?}"))
}

fn proof(bytes: &[u8]) -> Result<layerx_proof::merkle::Proof, String> {
    let decoded = checked(layerx_wire::receipt::decode_merkle_proof(bytes))?;
    checked(layerx_proof::merkle::Proof::new(
        decoded.leaf_index(),
        decoded.leaf_count(),
        decoded.siblings().to_vec(),
    ))
}

fn authorized_batch() -> Result<AuthorizedBatch, String> {
    let header_bytes =
        include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/header");
    let header = checked(layerx_wire::receipt::decode_batch_header(header_bytes))?;
    let receipt = checked(verify_sequencer_signature(RECEIPT, KEY))?;
    let protocol = receipt.protocol().ok_or("protocol receipt required")?;
    let sealed_batch = AuthorizedBatch::new(
        protocol.batch_id(),
        protocol.asset(),
        header.previous_state_root(),
        header.resulting_state_root(),
        KEY,
    );
    let authorization = layerx_proof::inclusion::SequencerAuthorization::new(
        header.sequencer_id(),
        KEY,
        header.batch_number(),
        header.batch_number(),
    );
    let receipt_proof = proof(include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt.proof"
    ))?;
    let maintenance_proof = proof(include_bytes!(
        "../../../../../tests/fixtures/asset/bound-native-withdrawal/maintenance.proof"
    ))?;
    let evidence = layerx_proof::receipt::MaintainedOutcomeEvidence {
        header: header_bytes,
        header_signature: include_bytes!(
            "../../../../../tests/fixtures/asset/bound-native-withdrawal/header.signature"
        ),
        activity_proof: &receipt_proof,
        maintenance: include_bytes!(
            "../../../../../tests/fixtures/asset/bound-native-withdrawal/maintenance.receipt"
        ),
        maintenance_proof: &maintenance_proof,
        authorization: &authorization,
    };
    checked(
        layerx_proof::receipt::authorized_maintained_activity_batch_chain(
            RECEIPT,
            &sealed_batch,
            &evidence,
            &[RECEIPT.to_vec()],
        ),
    )
}

#[test]
fn retained_withdrawal_outcome_decodes_the_real_fixture() -> Result<(), String> {
    let verified = checked(verify_outcome(RECEIPT, &authorized_batch()?))?;
    let outcome = checked(WithdrawalOutcome::from_verified(&verified))?;
    let protocol = verified
        .receipt()
        .protocol()
        .ok_or("protocol receipt required")?;
    assert_eq!(verified.canonical_bytes(), RECEIPT);
    assert_eq!(verified.rung(), layerx_types::rung::Rung::Submitted);
    assert_eq!(protocol.result_code(), 0);
    assert_eq!(outcome.amount(), 1);
    assert_eq!(outcome.asset(), protocol.asset());
    assert_eq!(outcome.nullifier(), protocol.context_hash());
    assert_ne!(outcome.nullifier(), [0; 32]);
    let [transfer, event] = protocol.effects() else {
        return Err("withdrawal transfer and event required".into());
    };
    assert!(transfer.monetary());
    assert_eq!(event.event_type(), 9);
    assert_eq!(outcome.recipient().as_slice(), &event.body()[130..150]);
    assert_ne!(outcome.recipient(), [0; 20]);
    assert_eq!(outcome.amount().to_be_bytes(), event.body()[102..118]);
    assert_eq!(outcome.asset().as_slice(), &event.body()[70..102]);
    Ok(())
}

#[test]
fn retained_withdrawal_path_refuses_changed_bytes_and_batch_authority() -> Result<(), String> {
    let batch = authorized_batch()?;
    let mut changed_receipt = RECEIPT.to_vec();
    let signature_byte = changed_receipt.last_mut().ok_or("receipt required")?;
    *signature_byte ^= 1;
    assert!(verify_outcome(&changed_receipt, &batch).is_err());
    let mut trailing_bytes = RECEIPT.to_vec();
    trailing_bytes.push(0);
    assert!(verify_outcome(&trailing_bytes, &batch).is_err());

    let mut wrong_key = KEY;
    wrong_key[0] ^= 1;
    let mut wrong_batch_id = batch.batch_id();
    wrong_batch_id[0] ^= 1;
    let mut wrong_asset = batch.asset();
    wrong_asset[0] ^= 1;
    let mut wrong_previous_root = batch.previous_state_root();
    wrong_previous_root[0] ^= 1;
    let mut wrong_resulting_root = batch.resulting_state_root();
    wrong_resulting_root[0] ^= 1;
    for (batch_id, asset, previous, resulting, key) in [
        (
            wrong_batch_id,
            batch.asset(),
            batch.previous_state_root(),
            batch.resulting_state_root(),
            KEY,
        ),
        (
            batch.batch_id(),
            wrong_asset,
            batch.previous_state_root(),
            batch.resulting_state_root(),
            KEY,
        ),
        (
            batch.batch_id(),
            batch.asset(),
            wrong_previous_root,
            batch.resulting_state_root(),
            KEY,
        ),
        (
            batch.batch_id(),
            batch.asset(),
            batch.previous_state_root(),
            wrong_resulting_root,
            KEY,
        ),
        (
            batch.batch_id(),
            batch.asset(),
            batch.previous_state_root(),
            batch.resulting_state_root(),
            wrong_key,
        ),
    ] {
        let unauthorized = AuthorizedBatch::new(batch_id, asset, previous, resulting, key);
        assert!(verify_outcome(RECEIPT, &unauthorized).is_err());
    }
    Ok(())
}
