use layerx_interop_gateway::error::{GatewayError, Retriability};

#[test]
fn receipt_pending_is_retriable_with_its_own_code() {
    let error = GatewayError::ReceiptPending;

    assert_eq!(error.code(), "receipt-pending");
    assert_eq!(error.retriability(), Retriability::Retriable);
    assert_ne!(error.code(), GatewayError::ReceiptRequired.code());
    assert_eq!(
        error.to_string(),
        "transaction is held by the sequencer without a signed receipt"
    );
}
