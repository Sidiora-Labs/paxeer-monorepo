use layerx_portable::{
    verify_portable_json, PortableReceiptError, PortableReceiptV2, PortableVerified,
    PortableVerifiedReceiptV2, ReceiptKeyEntry,
};

pub fn export_receipt(
    answer: &serde_json::Value,
    pinned_keys: &[ReceiptKeyEntry],
    raw_transaction: Option<&[u8]>,
) -> Result<Vec<u8>, PortableReceiptError> {
    PortableReceiptV2::from_sequencer_receipt(answer, pinned_keys, raw_transaction)?.to_json()
}

pub fn verify_receipt(
    json: &[u8],
    pinned_keys: &[ReceiptKeyEntry],
) -> Result<PortableVerifiedReceiptV2, PortableReceiptError> {
    let PortableVerified::V2(receipt) = verify_portable_json(json, pinned_keys)?;
    Ok(receipt)
}
