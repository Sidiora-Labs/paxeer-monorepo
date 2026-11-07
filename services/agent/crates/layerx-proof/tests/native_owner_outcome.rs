use layerx_proof::receipt::{
    verify_outcome, verify_sequencer_signature, AuthorizedBatch, ReceiptCheck,
};
use layerx_types::verify::VerificationLevel;

const ROOT: &str = "../../../platform/hosted/authority/tests/fixtures/native-sessions/lifecycle";

fn fixture(index: usize) -> (Vec<u8>, [u8; 32]) {
    let path = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join(ROOT);
    let receipt = std::fs::read(path.join(format!("grant-{index}.receipt")))
        .unwrap_or_else(|error| panic!("native receipt: {error}"));
    let key = std::fs::read(path.join("sequencer-public"))
        .unwrap_or_else(|error| panic!("native key: {error}"))
        .try_into()
        .unwrap_or_else(|_| panic!("native key length"));
    (receipt, key)
}

#[test]
fn native_owner_receipt_verifies_through_verify_outcome() {
    for index in 1..=10 {
        let (raw_receipt, key) = fixture(index);
        let receipt = verify_sequencer_signature(&raw_receipt, key)
            .unwrap_or_else(|error| panic!("native receipt: {error:?}"));
        let protocol = receipt.protocol().unwrap_or_else(|| panic!("protocol"));
        let authorised = AuthorizedBatch::new(
            protocol.batch_id(),
            protocol.asset(),
            protocol.previous_state_root(),
            protocol.resulting_state_root(),
            key,
        );
        assert_eq!(protocol.operation(), 0);
        let verified = verify_outcome(&raw_receipt, &authorised)
            .unwrap_or_else(|error| panic!("native owner outcome {index}: {error:?}"));
        assert_eq!(verified.canonical_bytes(), raw_receipt);
        assert_eq!(verified.level(), VerificationLevel::SEQUENCER_SIGNED);
        let result = verified
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("verified protocol"))
            .result_code();
        assert_eq!(result, protocol.result_code());
        if matches!(index, 7 | 8 | 10) {
            assert_ne!(result, 0);
        } else {
            assert_eq!(result, 0);
        }
        for (previous, resulting, expected) in [
            (
                [91; 32],
                authorised.resulting_state_root(),
                ReceiptCheck::PreviousStateRoot,
            ),
            (
                authorised.previous_state_root(),
                [92; 32],
                ReceiptCheck::ResultingStateRoot,
            ),
        ] {
            let other = AuthorizedBatch::new(
                authorised.batch_id(),
                authorised.asset(),
                previous,
                resulting,
                key,
            );
            assert_eq!(
                verify_outcome(&raw_receipt, &other)
                    .err()
                    .map(|failure| failure.check),
                Some(expected)
            );
        }
        let mut changed_receipt = raw_receipt.clone();
        let last = changed_receipt.len() - 1;
        changed_receipt[last] ^= 1;
        assert_eq!(
            verify_outcome(&changed_receipt, &authorised)
                .err()
                .map(|failure| failure.check),
            Some(ReceiptCheck::SequencerSignature)
        );
    }
}
