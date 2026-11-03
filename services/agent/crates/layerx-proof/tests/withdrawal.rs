use layerx_proof::receipt::{verify_outcome, verify_sequencer_signature, AuthorizedBatch};

const RECEIPT: &[u8] =
    include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/receipt");
const KEY: [u8; 32] =
    *include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/sequencer.public");

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> Result<T, String> {
    result.map_err(|error| format!("{error:?}"))
}

fn proof(bytes: &[u8]) -> Result<layerx_proof::merkle::Proof, String> {
    let path = checked(layerx_wire::receipt::decode_merkle_proof(bytes))?;
    checked(layerx_proof::merkle::Proof::new(
        path.leaf_index(),
        path.leaf_count(),
        path.siblings().to_vec(),
    ))
}

#[test]
fn paid_withdrawal_receipt_verifies_without_the_signed_activity() -> Result<(), String> {
    let header_bytes =
        include_bytes!("../../../../../tests/fixtures/asset/bound-native-withdrawal/header");
    let header = checked(layerx_wire::receipt::decode_batch_header(header_bytes))?;
    let receipt = checked(verify_sequencer_signature(RECEIPT, KEY))?;
    let protocol = receipt.protocol().ok_or("protocol receipt required")?;
    assert_eq!(
        (
            protocol.module_id(),
            protocol.operation(),
            protocol.result_code()
        ),
        (1, 9, 0)
    );
    assert_eq!((protocol.amount(), protocol.fee_charged()), (1, 17));
    assert_eq!(protocol.effects().len(), 2);
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
    let batch = AuthorizedBatch::new(
        protocol.batch_id(),
        protocol.asset(),
        header.previous_state_root(),
        header.resulting_state_root(),
        KEY,
    );
    let selected = checked(
        layerx_proof::receipt::authorized_maintained_activity_batch_chain(
            RECEIPT,
            &batch,
            &evidence,
            &[RECEIPT.to_vec()],
        ),
    )?;
    let verified = checked(verify_outcome(RECEIPT, &selected))?;
    assert_eq!(verified.canonical_bytes(), RECEIPT);
    assert_eq!(verified.rung(), layerx_types::rung::Rung::Submitted);
    let outcome = verified
        .receipt()
        .protocol()
        .ok_or("verified protocol required")?;
    assert_eq!(outcome.result_code(), protocol.result_code());
    assert_eq!(outcome.result_code(), 0);
    assert_eq!((outcome.amount(), outcome.fee_charged()), (1, 17));
    assert_eq!(outcome.effects(), protocol.effects());
    assert!(
        layerx_proof::receipt::authorized_maintained_activity_batch_chain(
            RECEIPT,
            &batch,
            &evidence,
            &[],
        )
        .is_err()
    );
    Ok(())
}
