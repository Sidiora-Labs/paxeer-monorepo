use std::{path::Path, process::Command, sync::OnceLock};

fn real_guest_results() -> &'static String {
    static RESULTS: OnceLock<String> = OnceLock::new();
    RESULTS.get_or_init(|| {
        let runtime_manifest =
            Path::new(env!("CARGO_MANIFEST_DIR")).join("../layerx-programs-runtime/Cargo.toml");
        let output = Command::new(env!("CARGO"))
            .args(["test", "--locked", "--manifest-path"])
            .arg(runtime_manifest)
            .args([
                "--lib",
                "execute::market_tests::repair_22_31_260_",
                "--",
                "--nocapture",
            ])
            .output()
            .unwrap_or_else(|error| panic!("run real runtime/compiled-guest integration: {error}"));
        let log = format!(
            "{}\n{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr),
        );
        eprintln!("{log}");
        assert!(
            output.status.success(),
            "real guest integration failed: {log}"
        );
        assert!(
            log.contains("2 passed; 0 failed"),
            "both real cases must execute: {log}"
        );
        log
    })
}

#[test]
fn billing_unbound() {
    assert!(real_guest_results()
        .contains("test execute::market_tests::repair_22_31_260_billing_unbound ... ok"));
}

#[test]
fn challenge_account_overlap() {
    assert!(real_guest_results()
        .contains("test execute::market_tests::repair_22_31_260_challenge_account_overlap ... ok"));
}
