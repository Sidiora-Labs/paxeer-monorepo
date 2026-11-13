use layerx_platform_authority::assertion::Must;

const BUDGET_READER_SOURCES: [(&str, &str); 2] = [
    (
        "src/human/budget_state.rs",
        include_str!("../src/human/budget_state.rs"),
    ),
    ("src/main.rs", include_str!("../src/main.rs")),
];

#[test]
fn no_retired_layerx_client_module_paths() {
    let migrated = [
        ("src/human.rs", include_str!("../src/human.rs")),
        ("src/trust.rs", include_str!("../src/trust.rs")),
        (
            "tests/budget_proof_exports.rs",
            include_str!("budget_proof_exports.rs"),
        ),
    ];
    let retired = [
        "client",
        "read",
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
        "evidence",
        "head",
        "handover",
        "lni",
    ];

    for (path, source) in migrated.into_iter().chain(BUDGET_READER_SOURCES) {
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for reference in compact.split("layerx_client::").skip(1) {
            let module = reference
                .split(|ch: char| !ch.is_ascii_alphanumeric() && ch != '_')
                .next()
                .must_with("module after client path");
            assert!(
                !retired.contains(&module),
                "{path} names retired path layerx_client::{module}"
            );
        }
    }
}

#[test]
fn budget_reader_uses_only_node_read_interfaces() {
    for (path, source) in BUDGET_READER_SOURCES {
        for retired in ["read", "evidence", "head", "handover", "lni"] {
            assert!(
                !source.contains(&format!("layerx_client::{retired}")),
                "{path} names retired path layerx_client::{retired}"
            );
        }
    }
    let committed_sources = [
        ("src/main.rs", include_str!("../src/main.rs")),
        ("src/human.rs", include_str!("../src/human.rs")),
        ("src/trust.rs", include_str!("../src/trust.rs")),
        ("src/committed.rs", include_str!("../src/committed.rs")),
        (
            "src/human/budget_state.rs",
            include_str!("../src/human/budget_state.rs"),
        ),
        (
            "src/human/budget_state/native.rs",
            include_str!("../src/human/budget_state/native.rs"),
        ),
    ];
    for (path, source) in committed_sources {
        for retired in [
            "latestFinalized()",
            "checkpoint(uint64)",
            "0000000000000000000000000000000000001014",
        ] {
            assert!(
                !source.contains(retired),
                "{path} names retired call {retired}"
            );
        }
    }
    for method in [
        "getBudget(bytes32)",
        "budgetRemaining(bytes32)",
        "layerx.human.budget-proof.v2",
    ] {
        assert!(
            BUDGET_READER_SOURCES
                .iter()
                .any(|(_, source)| source.contains(method)),
            "missing node interface {method}"
        );
    }
    for method in [
        "CommittedHeader::open",
        "commit_material",
        "block_by_number",
        "state_root",
        "abci_query",
        "/store/lxphost/key",
        "ics23:iavl",
        "ics23:simple",
        "verify_membership",
        "state_height",
        "bind_receipt",
    ] {
        assert!(
            committed_sources
                .iter()
                .any(|(_, source)| source.contains(method)),
            "missing committed-proof interface {method}"
        );
    }
}
