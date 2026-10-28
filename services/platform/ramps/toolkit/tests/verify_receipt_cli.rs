#![forbid(unsafe_code)]

use std::process::Command;

#[test]
fn verify_receipt_cli_refuses_activity_id_flag() {
    let output = Command::new(env!("CARGO_BIN_EXE_ramp-verify-receipt"))
        .args([
            "--config-file",
            "unread-config.json",
            "--order-file",
            "unread-order.json",
            "--activity-id",
            &"11".repeat(32),
            "--output-file",
            "unwritten-receipt.json",
        ])
        .output()
        .expect("run ramp-verify-receipt");

    let stderr = String::from_utf8(output.stderr).expect("UTF-8 CLI diagnostic");
    assert_eq!(output.status.code(), Some(2), "{stderr}");
    assert!(stderr.contains("usage: ramp-verify-receipt"), "{stderr}");
    assert!(
        stderr.contains("--transaction-hash TRANSACTION_HASH"),
        "{stderr}"
    );
}

#[test]
fn verify_receipt_cli_refuses_short_transaction_hash() {
    let invalid_hashes = [
        "0x1234".to_owned(),
        "0x".to_owned(),
        format!("0x{}", "11".repeat(31)),
        format!("0x{}", "11".repeat(33)),
        "11".repeat(32),
        format!("0X{}", "11".repeat(32)),
        format!("0x{}gg", "11".repeat(31)),
    ];
    for transaction_hash in invalid_hashes {
        let output = Command::new(env!("CARGO_BIN_EXE_ramp-verify-receipt"))
            .args([
                "--config-file",
                "unread-config.json",
                "--order-file",
                "unread-order.json",
                "--transaction-hash",
                &transaction_hash,
                "--output-file",
                "unwritten-receipt.json",
            ])
            .output()
            .expect("run ramp-verify-receipt");

        let stderr = String::from_utf8(output.stderr).expect("UTF-8 CLI diagnostic");
        assert_eq!(
            output.status.code(),
            Some(2),
            "hash {transaction_hash}: {stderr}"
        );
        assert!(
            stderr.contains("--transaction-hash must be 0x followed by 64 hex digits"),
            "hash {transaction_hash}: {stderr}"
        );
    }
}
