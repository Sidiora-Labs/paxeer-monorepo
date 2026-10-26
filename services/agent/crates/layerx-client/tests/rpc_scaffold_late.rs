pub use layerx_client::rpc::{self, evidence, history, payments, receipt, stream, withdrawal};

#[test]
fn late_rpc_modules_are_public() {
    let rpc = include_str!("../src/rpc/mod.rs");
    for module in [
        "history",
        "payments",
        "withdrawal",
        "receipt",
        "stream",
        "evidence",
    ] {
        assert!(
            rpc.lines().any(|line| line == format!("pub mod {module};")),
            "layerx_client::rpc::{module} must be public"
        );
    }
}
