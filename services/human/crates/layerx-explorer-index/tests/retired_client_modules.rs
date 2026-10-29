#[test]
fn no_retired_layerx_client_module_paths() {
    let files = [
        ("src/lib.rs", include_str!("../src/lib.rs")),
        ("src/query.rs", include_str!("../src/query.rs")),
        ("tests/rebuild.rs", include_str!("rebuild.rs")),
        ("tests/verify.rs", include_str!("verify.rs")),
        (
            "tests/retired_client_modules.rs",
            include_str!("retired_client_modules.rs"),
        ),
    ];
    let retired = [
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
    for (path, source) in files {
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for suffix in compact.split("layerx_client::").skip(1) {
            let module = suffix
                .split(|ch: char| !ch.is_ascii_alphanumeric() && ch != '_')
                .next()
                .expect("a split suffix always has a first component");
            if retired.contains(&module) {
                remaining.push(format!("{path}: layerx_client::{module}"));
            }
        }
    }
    assert!(
        remaining.is_empty(),
        "retired client module paths remain:\n{}",
        remaining.join("\n")
    );
}
