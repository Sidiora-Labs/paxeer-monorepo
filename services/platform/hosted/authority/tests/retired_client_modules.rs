#[test]
fn no_retired_layerx_client_module_paths() {
    let migrated = [
        ("src/human.rs", include_str!("../src/human.rs")),
        (
            "src/human/budget_state.rs",
            include_str!("../src/human/budget_state.rs"),
        ),
        ("src/trust.rs", include_str!("../src/trust.rs")),
        (
            "tests/budget_proof_exports.rs",
            include_str!("budget_proof_exports.rs"),
        ),
    ];
    let retired = [
        "client",
        "resolver",
        "handover_policy",
        "availability",
        "caps",
        "execution_prestate",
        "arbiter_admission",
        "arbiter_prestate",
        "receipt",
        "stream",
        "submit",
        "payments",
        "withdrawal",
    ];

    for (path, source) in migrated {
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for reference in compact.split("layerx_client::").skip(1) {
            let module = reference
                .split(|ch: char| !ch.is_ascii_alphanumeric() && ch != '_')
                .next()
                .expect("module after client path");
            assert!(
                !retired.contains(&module),
                "{path} names retired path layerx_client::{module}"
            );
        }
    }
}
