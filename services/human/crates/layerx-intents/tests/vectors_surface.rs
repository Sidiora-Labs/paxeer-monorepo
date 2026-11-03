#![cfg(feature = "test-vectors")]

use layerx_intents::canonical::{decode_receipt, PROTOCOL_VERSION};
use layerx_intents::vectors::{security_program_receipt, withdrawal_receipt};

#[test]
fn remaining_receipt_vectors_decode() {
    for signature in [None, Some([0x55; 64])] {
        let withdrawal = withdrawal_receipt(PROTOCOL_VERSION, [0x22; 32], 25, 100, signature);
        let security = security_program_receipt(
            [0x31; 32], [0x43; 32], 1, 1_000, [0x42; 32], [0x44; 32], signature,
        );

        for (name, bytes) in [("withdrawal", withdrawal), ("security program", security)] {
            let decoded = decode_receipt(&bytes);
            assert!(
                decoded.is_ok(),
                "{name} receipt with signature {signature:?} must decode: {decoded:?}"
            );
        }
    }
}
