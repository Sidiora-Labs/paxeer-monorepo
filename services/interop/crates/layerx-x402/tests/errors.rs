use layerx_x402::model::X402Error;

#[test]
fn transaction_form_error_names_the_tx_hash_form() {
    assert_eq!(
        X402Error::TransactionForm.to_string(),
        "x402 settlement transaction is not a 0x-prefixed 32-byte transaction hash"
    );
    assert_ne!(X402Error::TransactionForm, X402Error::EvidenceMismatch);
    assert_ne!(
        X402Error::TransactionForm.to_string(),
        X402Error::EvidenceMismatch.to_string()
    );
}
