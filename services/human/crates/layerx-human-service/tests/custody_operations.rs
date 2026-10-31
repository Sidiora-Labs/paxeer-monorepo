use layerx_human_service::custody::Operation;

#[test]
fn custody_operation_has_no_emergency_exit() {
    assert_eq!(Operation::from_label("emergency-exit"), None);

    for (label, operation) in [
        ("protocol-mutation", Operation::ProtocolMutation),
        ("approval-decision", Operation::ApprovalDecision),
        ("security-settings", Operation::SecuritySettings),
        ("secret-reveal", Operation::SecretReveal),
        ("withdrawal", Operation::Withdrawal),
        ("wallet-rebinding", Operation::WalletRebinding),
        ("agent-archive", Operation::AgentArchive),
    ] {
        assert_eq!(Operation::from_label(label), Some(operation));
        assert_eq!(operation.label(), label);
    }
}
