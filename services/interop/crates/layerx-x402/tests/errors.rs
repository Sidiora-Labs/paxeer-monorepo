use layerx_x402::model::{SettlementResponse, X402Error};

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

#[test]
fn settlement_response_refuses_a_non_hash_transaction() {
    for success in [true, false] {
        let mut response = SettlementResponse {
            success,
            error_reason: (!success).then(|| "settlement_pending".to_owned()),
            payer: None,
            transaction: String::new(),
            network: "eip155:125".to_owned(),
            amount: None,
            extensions: Default::default(),
        };
        for transaction in [
            format!("lxp:{}", "a".repeat(64)),
            "0x1234".to_owned(),
            format!("0x{}", "a".repeat(63)),
            format!("0x{}", "a".repeat(65)),
            "a".repeat(64),
            format!("0X{}", "a".repeat(64)),
            format!("0x{}", "A".repeat(64)),
            format!("0x{}", "g".repeat(64)),
            format!("0x{}", "0".repeat(64)),
            format!("0x{}", "é".repeat(32)),
        ] {
            response.transaction = transaction;
            assert_eq!(
                response.validate_wire(),
                Err(X402Error::TransactionForm),
                "success: {success}, transaction: {}",
                response.transaction
            );
        }
        for transaction in [
            format!("0x{}", "0123456789abcdef".repeat(4)),
            format!("0x{}1", "0".repeat(63)),
        ] {
            response.transaction = transaction;
            assert_eq!(response.validate_wire(), Ok(()));
        }
        response.transaction.clear();
        assert_eq!(
            response.validate_wire(),
            Err(if success {
                X402Error::EvidenceMissing
            } else {
                X402Error::InvalidPayload
            })
        );
        if !success {
            response.error_reason = Some("insufficient_balance".to_owned());
            assert_eq!(response.validate_wire(), Ok(()));
        }
    }
}
