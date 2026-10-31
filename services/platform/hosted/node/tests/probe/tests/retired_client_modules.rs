#[test]
fn no_retired_layerx_client_module_paths() {
    let sources = [("src/main.rs", include_str!("../src/main.rs"))];
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
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for module in retired_modules {
            let retired_path = format!("layerx_client::{module}");
            if compact.match_indices(&retired_path).any(|(index, _)| {
                compact[index + retired_path.len()..]
                    .chars()
                    .next()
                    .is_none_or(|ch| !ch.is_alphanumeric() && ch != '_')
            }) {
                remaining.push(format!("{path}: {retired_path}"));
            }
        }
    }
    assert!(
        remaining.is_empty(),
        "retired wiring.J44.20 module paths remain:\n{}",
        remaining.join("\n")
    );
}
