#[test]
fn no_retired_layerx_client_module_paths() {
    let sources = [
        ("src/discovery.rs", include_str!("../src/discovery.rs")),
        ("src/main.rs", include_str!("../src/main.rs")),
        ("tests/job_admission.rs", include_str!("job_admission.rs")),
        ("tests/job_recovery.rs", include_str!("job_recovery.rs")),
        (
            "tests/membership_recovery.rs",
            include_str!("membership_recovery.rs"),
        ),
        ("tests/real_workload.rs", include_str!("real_workload.rs")),
        (
            "tests/task_chain_handoff.rs",
            include_str!("task_chain_handoff.rs"),
        ),
        (
            "tests/transport_identity.rs",
            include_str!("transport_identity.rs"),
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

    for (path, source) in sources {
        let compact: String = source.chars().filter(|c| !c.is_whitespace()).collect();
        for module in retired_modules {
            let retired = format!("layerx_client::{module}");
            for (offset, _) in compact.match_indices(&retired) {
                let suffix = &compact[offset + retired.len()..];
                assert!(
                    suffix.starts_with(|c: char| c.is_ascii_alphanumeric() || c == '_'),
                    "{path} still names retired path {retired}"
                );
            }
            assert!(
                !compact.contains(&format!("layerx_client::{{{module}::")),
                "{path} still imports retired module {module}"
            );
        }
    }
}
