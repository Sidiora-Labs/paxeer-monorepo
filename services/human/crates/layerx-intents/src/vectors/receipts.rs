use crate::canonical::STATE_COMMITMENT_PROTOCOL_VERSION;
use layerx_types::payload::ModuleId;
use layerx_wire::encode::Encoder;
use layerx_wire::WireError;

fn append_signature(encoder: &mut Encoder, signature: Option<[u8; 64]>) -> Result<(), WireError> {
    encoder.u8(u8::from(signature.is_some()))?;
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64)?;
    }
    Ok(())
}

/// Builds the canonical activity receipt vector with a height-encoded batch identifier.
///
/// # Errors
/// Returns a wire error if a receipt field exceeds its canonical bound.
///
/// # Panics
/// Panics if the activity vector is not a protocol receipt with empty effects.
pub fn activity_receipt(height: u64, signature: Option<[u8; 64]>) -> Result<Vec<u8>, WireError> {
    let bytes = super::receipt(None)?;
    let decoded = layerx_wire::receipt::decode(&bytes)?;
    let receipt = decoded
        .protocol()
        .unwrap_or_else(|| panic!("{}", "protocol receipt"));
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&height.to_be_bytes());
    let mut encoder = layerx_wire::encode::Encoder::new(4096);
    encoder.structure_header_version(0x5201, receipt.protocol_version())?;
    encoder.u16(receipt.protocol_version())?;
    encoder.bytes(&receipt.activity_id(), 32)?;
    encoder.u64(receipt.global_sequence())?;
    encoder.bytes(&receipt.previous_state_root(), 32)?;
    encoder.bytes(&receipt.resulting_state_root(), 32)?;
    encoder.bytes(&receipt.activity_root(), 32)?;
    encoder.i32(receipt.result_code())?;
    assert!(receipt.effects().is_empty());
    encoder.sequence_length(0, 512)?;
    encoder.u128(receipt.fee_charged())?;
    encoder.bytes(&batch_id, 32)?;
    encoder.u16(receipt.module_id())?;
    encoder.u32(receipt.module_version())?;
    encoder.u32(receipt.parameter_version())?;
    encoder.u8(receipt.operation())?;
    encoder.bytes(&receipt.asset(), 32)?;
    encoder.u128(receipt.amount())?;
    encoder.bytes(&receipt.from(), 32)?;
    encoder.u128(receipt.debit_balance_before())?;
    encoder.u128(receipt.debit_balance_after())?;
    encoder.u64(receipt.debit_sequence())?;
    encoder.bytes(&receipt.to(), 32)?;
    encoder.u128(receipt.credit_balance_before())?;
    encoder.u128(receipt.credit_balance_after())?;
    encoder.bytes(&receipt.transfer_set_root(), 32)?;
    encoder.bytes(&receipt.authorization_hash(), 32)?;
    encoder.bytes(&receipt.context_hash(), 32)?;
    encoder.u64(receipt.timestamp())?;
    append_signature(&mut encoder, signature)?;
    Ok(encoder.finish())
}

/// Builds a canonical governance receipt for an onboarding activity and result.
///
/// # Errors
/// Returns a wire error if a receipt field exceeds its canonical bound.
///
/// # Panics
/// Panics if the canonical receipt round-trip changes the encoded bytes.
pub fn onboarding_receipt(
    id: [u8; 32],
    result: i32,
    height: u64,
    signature: Option<[u8; 64]>,
) -> Result<Vec<u8>, WireError> {
    let mut encoder = Encoder::new(4096);
    encoder.structure_header_version(0x5201, STATE_COMMITMENT_PROTOCOL_VERSION)?;
    encoder.u16(STATE_COMMITMENT_PROTOCOL_VERSION)?;
    encoder.bytes(&id, 32)?;
    encoder.u64(7)?;
    for root in [[0x31; 32], [0x32; 32], [0x81; 32]] {
        encoder.bytes(&root, 32)?;
    }
    encoder.i32(result)?;
    encoder.u32(0)?;
    encoder.u128(1)?;
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&height.to_be_bytes());
    encoder.bytes(&batch_id, 32)?;
    encoder.u16(ModuleId::Governance as u16)?;
    encoder.u32(1)?;
    encoder.u32(1)?;
    encoder.u8(0)?;
    encoder.bytes(&[0; 32], 32)?;
    encoder.u128(0)?;
    encoder.bytes(&[0; 32], 32)?;
    encoder.u128(0)?;
    encoder.u128(0)?;
    encoder.u64(0)?;
    encoder.bytes(&[0; 32], 32)?;
    encoder.u128(0)?;
    encoder.u128(0)?;
    for _ in 0..3 {
        encoder.bytes(&[0; 32], 32)?;
    }
    encoder.u64(1000)?;
    append_signature(&mut encoder, signature)?;
    let bytes = encoder.finish();
    let decoded = layerx_wire::receipt::decode(&bytes)?;
    let canonical = layerx_wire::receipt::encode(&decoded)?;
    assert_eq!(canonical, bytes);
    Ok(canonical)
}
