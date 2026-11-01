#[test]
fn no_retired_layerx_client_module_paths() {
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
    for (path, source) in [
        ("src/config.rs", include_str!("../src/config.rs")),
        ("tests/kms/mod.rs", include_str!("kms/mod.rs")),
    ] {
        for suffix in source.split("layerx_client").skip(1) {
            let Some(suffix) = suffix.trim_start().strip_prefix("::") else {
                continue;
            };
            let mut depth = 0;
            let mut module_start = true;
            let mut name = String::new();
            for character in suffix.chars().chain(std::iter::once(';')) {
                if name.is_empty() && character.is_whitespace() {
                    continue;
                }
                if module_start && (character.is_ascii_alphanumeric() || character == '_') {
                    name.push(character);
                    continue;
                }
                if !name.is_empty() {
                    assert!(
                        !retired_modules.contains(&name.as_str()),
                        "{path} names retired layerx_client module {name} (wiring.J44.21)"
                    );
                    name.clear();
                    module_start = false;
                }
                match character {
                    '{' => depth += 1,
                    '}' => depth -= 1,
                    ',' if depth == 1 => module_start = true,
                    _ => {}
                }
                if depth == 0 {
                    break;
                }
            }
        }
    }
}
