#[test]
fn no_retired_layerx_client_module_paths_w2() {
    let files = [("src/main.rs", include_str!("../src/main.rs"))];
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
    let retired_paths = retired_modules.map(|module| format!("layerx_client::{module}"));
    let retired_decision_paths = [
        "layerx_client::Client",
        "layerx_client::ClientConfig",
        "layerx_client::rpc::lni",
        "layerx_client::rpc::submit::Submission::Acknowledged",
        "layerx_crypto::disclosure::bind",
        "layerx_crypto::signer::sign_disclosed",
        "layerx_crypto::onboarding",
        "layerx_crypto::rotation",
    ];

    for (file, source) in files {
        let source: String = source.split_whitespace().collect();
        let remaining: Vec<_> = retired_paths
            .iter()
            .map(String::as_str)
            .chain(retired_decision_paths)
            .filter_map(|path| {
                source
                    .match_indices(path)
                    .any(|(offset, matched)| {
                        source[offset + matched.len()..]
                            .chars()
                            .next()
                            .is_none_or(|next| !next.is_alphanumeric() && next != '_')
                    })
                    .then_some(path)
            })
            .collect();
        assert!(
            remaining.is_empty(),
            "{file} retains wiring.J44.20 or decision.1813 retired paths: {remaining:?}"
        );
    }
}
