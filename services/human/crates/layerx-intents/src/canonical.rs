pub use layerx_wire::hash::{
    account_id_for_protocol, activity_id, batch_header_digest, did_id_for_protocol,
    execution_batch_id, payload_hash, payload_hash_for, program_execution_batch_id, receipt_digest,
    Domain,
};
pub use layerx_wire::limits::{
    protocol_version_supported, MAX_MESSAGE_BYTES, PROTOCOL_VERSION,
    STATE_COMMITMENT_PROTOCOL_VERSION,
};
pub use layerx_wire::maintenance::decode_occupancy_maintenance;
pub use layerx_wire::receipt::{
    decode as decode_receipt, decode_batch_header, decode_merkle_proof, encode as receipt_bytes,
    encode_batch_header as batch_header_bytes, encode_unsigned as unsigned_receipt_bytes,
    BatchHeader, ProgramOutcome, ProtocolReceipt,
};
pub use layerx_wire::WireError as CanonicalError;

/// # Errors
/// Refuses noncanonical debit fields before constructing the authorization preimage.
pub fn send_authorization_bytes(debit: &SendDebit) -> Result<Vec<u8>, DisclosureError> {
    debit.authorization_message()
}

/// # Errors
/// Refuses malformed debit fields or an authorization signature not bound to them.
pub fn signed_send_payload(
    debit: &SendDebit,
    public_key: [u8; 32],
    signature: [u8; 64],
) -> Result<Vec<u8>, DisclosureError> {
    debit.encode_signed(public_key, signature)
}
use layerx_crypto::{disclosure::DisclosureError, send::SendDebit};
