#![cfg(feature = "test-vectors")]

use layerx_intents::{canonical, vectors};
use layerx_types::payload::ModuleId;
use layerx_wire::encode::Encoder;

fn activity_fields(
    height: u64,
    signature: Option<[u8; 64]>,
) -> Result<Vec<u8>, layerx_wire::WireError> {
    let bytes = layerx_intents::vectors::receipt(None)?;
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
    encoder.u8(u8::from(signature.is_some()))?;
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64)?;
    }
    Ok(encoder.finish())
}

fn onboarding_fields(
    id: [u8; 32],
    result: i32,
    height: u64,
    signature: Option<[u8; 64]>,
) -> Result<Vec<u8>, layerx_wire::WireError> {
    let mut encoder = Encoder::new(4096);
    encoder.structure_header_version(0x5201, canonical::STATE_COMMITMENT_PROTOCOL_VERSION)?;
    encoder.u16(canonical::STATE_COMMITMENT_PROTOCOL_VERSION)?;
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
    encoder.u8(u8::from(signature.is_some()))?;
    if let Some(signature) = signature {
        encoder.bytes(&signature, 64)?;
    }
    let bytes = encoder.finish();
    let decoded = layerx_wire::receipt::decode(&bytes)?;
    let canonical = layerx_wire::receipt::encode(&decoded)?;
    assert_eq!(canonical, bytes);
    Ok(canonical)
}

#[test]
fn activity_receipt_matches_canonical_fields() {
    for height in [0, 42, u64::MAX] {
        for signature in [None, Some([0; 64]), Some([0x51; 64])] {
            let bytes = vectors::activity_receipt(height, signature)
                .unwrap_or_else(|error| panic!("activity receipt: {error:?}"));
            assert_eq!(Ok(bytes.clone()), activity_fields(height, signature));
            let decoded = layerx_wire::receipt::decode(&bytes)
                .unwrap_or_else(|error| panic!("activity receipt: {error:?}"));
            assert_eq!(layerx_wire::receipt::encode(&decoded), Ok(bytes));
        }
    }
}

#[test]
fn onboarding_receipt_matches_canonical_fields() {
    for (id, result, height) in [
        ([0; 32], 0, 0),
        ([0x73; 32], -32_001, 42),
        ([0xff; 32], i32::MAX, u64::MAX),
    ] {
        for signature in [None, Some([0; 64]), Some([0x51; 64])] {
            let bytes = vectors::onboarding_receipt(id, result, height, signature)
                .unwrap_or_else(|error| panic!("onboarding receipt: {error:?}"));
            assert_eq!(Ok(bytes), onboarding_fields(id, result, height, signature));
        }
    }
}
