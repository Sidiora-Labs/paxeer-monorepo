pub use layerx_client::{
    px,
    rpc::{self, account, batch, module_state, submit},
    subscription,
};

#[test]
fn rpc_module_tree_and_new_reads_are_public() {
    let library = include_str!("../src/lib.rs");
    for module in ["rpc", "px", "subscription"] {
        assert!(
            library
                .lines()
                .any(|line| line == format!("pub mod {module};")),
            "layerx_client::{module} must remain public"
        );
    }

    let rpc = include_str!("../src/rpc/mod.rs");
    for module in [
        "account",
        "history",
        "receipt",
        "module_state",
        "submit",
        "batch",
        "payments",
        "withdrawal",
        "evidence",
    ] {
        assert!(
            rpc.lines().any(|line| line == format!("pub mod {module};")),
            "layerx_client::rpc::{module} must be public"
        );
    }
}
