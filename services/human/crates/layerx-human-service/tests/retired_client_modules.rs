#[test]
fn no_retired_layerx_client_module_paths() {
    let sources = [
        (
            "src/bin/layerx-human-kms-gateway.rs",
            include_str!("../src/bin/layerx-human-kms-gateway.rs"),
        ),
        (
            "src/custody/gateway.rs",
            include_str!("../src/custody/gateway.rs"),
        ),
        (
            "src/custody/provider.rs",
            include_str!("../src/custody/provider.rs"),
        ),
        (
            "src/server/backend.rs",
            include_str!("../src/server/backend.rs"),
        ),
        (
            "src/server/component.rs",
            include_str!("../src/server/component.rs"),
        ),
        ("src/server/mod.rs", include_str!("../src/server/mod.rs")),
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
    let mut remaining = Vec::new();
    for (path, source) in sources {
        let compact: String = source.chars().filter(|c| !c.is_whitespace()).collect();
        for module in retired_modules {
            let retired = format!("layerx_client::{module}");
            for (offset, _) in compact.match_indices(&retired) {
                let suffix = &compact[offset + retired.len()..];
                if !suffix.starts_with(|c: char| c.is_alphanumeric() || c == '_') {
                    remaining.push(format!("{path}: {retired}"));
                }
            }
        }
    }
    assert!(
        remaining.is_empty(),
        "retired wiring.J44.21 module paths remain:\n{}",
        remaining.join("\n")
    );
}
