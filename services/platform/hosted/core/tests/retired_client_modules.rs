#[test]
fn no_retired_layerx_client_module_paths() {
    assert!(
        !std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("src/caps_reads.rs")
            .exists(),
        "caps discovery has no replacement in decision.1813"
    );
    const RETIRED: &[&str] = &[
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
    let sources = [
        (
            "src/public_reads.rs",
            include_str!("../src/public_reads.rs"),
        ),
        ("src/withdrawal.rs", include_str!("../src/withdrawal.rs")),
    ];
    let mut remaining = Vec::new();
    for (path, source) in sources {
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        let mut check = |module_path: &str| {
            let module = module_path
                .split(|ch: char| !ch.is_ascii_alphanumeric() && ch != '_')
                .next()
                .unwrap_or_default();
            if RETIRED.contains(&module) {
                remaining.push(format!("{path}: layerx_client::{module}"));
            }
        };
        for suffix in compact.split("layerx_client::").skip(1) {
            if let Some(group) = suffix.strip_prefix('{') {
                let mut depth = 0;
                let mut start = 0;
                for (index, ch) in group.char_indices() {
                    match ch {
                        '{' => depth += 1,
                        '}' if depth == 0 => {
                            check(&group[start..index]);
                            break;
                        }
                        '}' => depth -= 1,
                        ',' if depth == 0 => {
                            check(&group[start..index]);
                            start = index + 1;
                        }
                        _ => {}
                    }
                }
            } else {
                check(suffix);
            }
        }
    }
    assert!(
        remaining.is_empty(),
        "retired wiring.J44.20 paths remain:\n{}",
        remaining.join("\n")
    );
}
