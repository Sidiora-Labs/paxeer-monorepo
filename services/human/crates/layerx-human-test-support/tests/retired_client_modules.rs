#[test]
fn no_retired_layerx_client_module_paths() {
    let files = [
        ("src/lib.rs", include_str!("../src/lib.rs")),
        (
            "src/evidence_node.rs",
            include_str!("../src/evidence_node.rs"),
        ),
        (
            "tests/retired_client_modules.rs",
            include_str!("retired_client_modules.rs"),
        ),
    ];
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

    for (path, source) in files {
        let source: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for module in retired_modules {
            let retired_path = format!("layerx_client::{module}");
            assert!(
                !source.contains(&retired_path),
                "{path} still names retired path {retired_path} from wiring.J44.21"
            );
        }
    }
}
