#[test]
fn no_retired_layerx_client_module_paths_w2() {
    let files = [
        (
            "src/main.rs",
            include_str!("../src/main.rs"),
            &["client", "head", "caps", "batch"][..],
        ),
        (
            "src/trust.rs",
            include_str!("../src/trust.rs"),
            &["client", "head", "caps"][..],
        ),
    ];

    for (path, source, retired_modules) in files {
        let source: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for module in retired_modules {
            let retired_path = format!("layerx_client::{module}");
            for (offset, _) in source.match_indices(&retired_path) {
                let next = source[offset + retired_path.len()..].chars().next();
                assert!(
                    next.is_some_and(|ch| ch.is_ascii_alphanumeric() || ch == '_'),
                    "{path} names retired path {retired_path} from wiring.J44.20"
                );
            }
        }
    }
}
