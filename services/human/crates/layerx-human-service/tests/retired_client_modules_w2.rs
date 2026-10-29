#[test]
fn no_retired_layerx_client_module_paths_w2() {
    let sources = [(
        "src/server/agent_runtime.rs",
        include_str!("../src/server/agent_runtime.rs"),
    )];
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
    let mut violations = Vec::new();
    for (path, source) in sources {
        let compact: String = source.chars().filter(|c| !c.is_whitespace()).collect();
        for module in retired_modules {
            let retired_path = format!("layerx_client::{module}");
            if compact.match_indices(&retired_path).any(|(offset, _)| {
                compact[offset + retired_path.len()..]
                    .chars()
                    .next()
                    .is_none_or(|c| !(c.is_alphanumeric() || c == '_'))
            }) {
                violations.push(format!("{path}: {retired_path}"));
            }
        }
    }
    assert!(
        violations.is_empty(),
        "retired layerx_client module paths remain:\n{}",
        violations.join("\n")
    );
}
