use std::fs;
use std::path::PathBuf;

#[test]
fn human_service_names_no_retired_intent_kind() {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"));
    let targets: [(&str, &[&str]); 2] = [
        (
            "tests/render.rs",
            &[
                "BridgeDepositCredit",
                "EvmPayoutBinding",
                "NativeCustodyCredit",
            ],
        ),
        ("src/server/onboarding_sponsor.rs", &["NativeCustodyCredit"]),
    ];
    let mut violations = Vec::new();
    for (relative_path, retired) in targets {
        let path = root.join(relative_path);
        let source = fs::read_to_string(&path)
            .unwrap_or_else(|error| panic!("read {}: {error}", path.display()));
        for (line_number, line) in source.lines().enumerate() {
            for name in retired {
                if line.contains(name) {
                    violations.push(format!("{relative_path}:{}: {name}", line_number + 1));
                }
            }
        }
    }
    violations.sort();
    assert!(
        violations.is_empty(),
        "human service names retired intent kinds:\n{}",
        violations.join("\n")
    );
}
