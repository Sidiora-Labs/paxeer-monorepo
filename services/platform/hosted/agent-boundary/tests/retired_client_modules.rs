#[test]
fn no_retired_layerx_client_module_paths() {
    let sources = [("src/deployment.rs", include_str!("../src/deployment.rs"))];
    let retired = [
        "Client",
        "ClientConfig",
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
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for suffix in compact.split("layerx_client::").skip(1) {
            let mut depth = 0;
            let mut module_start = true;
            let mut module = String::new();
            for ch in suffix.chars().chain(std::iter::once(';')) {
                if module_start && (ch.is_ascii_alphanumeric() || ch == '_') {
                    module.push(ch);
                    continue;
                }
                if !module.is_empty() {
                    if retired.contains(&module.as_str()) {
                        violations.push(format!("{path}: layerx_client::{module}"));
                    }
                    module.clear();
                    module_start = false;
                    if depth == 0 {
                        break;
                    }
                }
                match ch {
                    '{' => depth += 1,
                    '}' => {
                        depth -= 1;
                        if depth == 0 {
                            break;
                        }
                    }
                    ',' if depth == 1 => module_start = true,
                    ';' => break,
                    _ => {}
                }
            }
        }
        for retired_path in [
            "layerx_client::Client",
            "layerx_client::ClientConfig",
            "layerx_client::rpc::lni",
            "layerx_client::rpc::submit::Submission::Acknowledged",
            "layerx_crypto::disclosure::bind",
            "layerx_crypto::signer::sign_disclosed",
            "layerx_crypto::onboarding",
            "layerx_crypto::rotation",
        ] {
            if compact
                .match_indices(retired_path)
                .any(|(offset, matched)| {
                    compact[offset + matched.len()..]
                        .chars()
                        .next()
                        .is_none_or(|ch| !ch.is_alphanumeric() && ch != '_')
                })
            {
                violations.push(format!("{path}: {retired_path}"));
            }
        }
    }
    assert!(
        violations.is_empty(),
        "retired wiring.J44.20 or decision.1813 paths remain:\n{}",
        violations.join("\n")
    );
}
