#[test]
fn no_retired_layerx_client_module_paths_w2() {
    let sources = [("src/routes.rs", include_str!("../src/routes.rs"))];
    let retired_modules = [
        "lni",
        "client",
        "head",
        "resolver",
        "handover",
        "handover_policy",
        "availability",
        "caps",
        "execution_prestate",
        "arbiter_admission",
        "arbiter_prestate",
        "read",
        "receipt",
        "stream",
        "submit",
        "batch",
        "payments",
        "withdrawal",
        "evidence",
    ];

    for (path, source) in sources {
        let source: String = source.split_whitespace().collect();
        for module in retired_modules {
            let retired_path = format!("layerx_client::{module}");
            assert!(
                !source.contains(&retired_path),
                "{path} still names retired module {retired_path}"
            );
        }
    }
}
