use layerx_intents::canonical::{
    batch_header_bytes, decode_batch_header, decode_receipt, receipt_bytes, unsigned_receipt_bytes,
    BatchHeader, ProtocolReceipt,
};
use layerx_proof::receipt::verify_sequencer_signature;
use std::fmt::Debug;
use std::path::Path;

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("canonical receipt surface: {error:?}"))
}

#[test]
fn receipt_reexports_round_trip_after_activity_retirement() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../../tests/fixtures/programs/maintained-multicall");
    let key: [u8; 32] = checked(checked(std::fs::read(root.join("sequencer.public"))).try_into());
    let header_bytes = checked(std::fs::read(root.join("header")));
    let header: BatchHeader = checked(decode_batch_header(&header_bytes));
    assert_eq!(checked(batch_header_bytes(&header)), header_bytes);
    assert_eq!(header.protocol_version(), 3);
    assert_ne!(header.network_id(), 0);
    assert_ne!(header.resulting_state_root(), [0; 32]);

    for name in ["receipt-0", "receipt-1"] {
        let bytes = checked(std::fs::read(root.join(name)));
        let receipt = checked(decode_receipt(&bytes));
        let protocol: &ProtocolReceipt = receipt.protocol().expect("full protocol receipt");
        assert_eq!(protocol.protocol_version(), header.protocol_version());
        assert!((header.first_sequence()..=header.last_sequence())
            .contains(&protocol.global_sequence()));
        assert!(protocol.sequencer_signature().is_some());
        assert_eq!(checked(verify_sequencer_signature(&bytes, key)), receipt);
        let encoded = checked(receipt_bytes(&receipt));
        assert_eq!(encoded, bytes);
        assert_eq!(checked(decode_receipt(&encoded)), receipt);
        assert_eq!(
            checked(unsigned_receipt_bytes(&receipt)),
            [&bytes[..bytes.len() - 69], &[0]].concat()
        );
        assert!(decode_receipt(&bytes[..bytes.len() - 1]).is_err());
        let mut trailing = bytes;
        trailing.push(0);
        assert!(decode_receipt(&trailing).is_err());
    }

    assert!(decode_batch_header(&header_bytes[..header_bytes.len() - 1]).is_err());
    let mut trailing = header_bytes;
    trailing.push(0);
    assert!(decode_batch_header(&trailing).is_err());
}
