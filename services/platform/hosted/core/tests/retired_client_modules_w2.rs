#[test]
fn no_retired_layerx_client_module_paths_w2() {
    let sources = [("src/main.rs", include_str!("../src/main.rs"))];
    let retired_modules = [
        "head",
        "resolver",
        "handover",
        "handover_policy",
        "availability",
        "caps",
        "execution_prestate",
        "arbiter_admission",
        "arbiter_prestate",
        "receipt",
        "stream",
        "submit",
        "batch",
        "payments",
        "withdrawal",
        "evidence",
    ];

    for (file, source) in sources {
        let source: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for module in retired_modules {
            let retired_path = format!("layerx_client::{module}");
            for (offset, _) in source.match_indices(&retired_path) {
                let suffix = &source[offset + retired_path.len()..];
                assert!(
                    suffix.starts_with(|ch: char| ch.is_ascii_alphanumeric() || ch == '_'),
                    "{file} still names the retired wiring.J44.20 path {retired_path}"
                );
            }
        }
    }
}
