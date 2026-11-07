#[test]
fn no_retired_layerx_crypto_codec_paths() {
    fn collect_paths(tokens: &[String], cursor: &mut usize, prefix: &str, paths: &mut Vec<String>) {
        let mut path = prefix.to_owned();
        while let Some(token) = tokens.get(*cursor) {
            if token == "{" {
                *cursor += 1;
                while tokens.get(*cursor).is_some_and(|token| token != "}") {
                    let previous = *cursor;
                    collect_paths(tokens, cursor, &path, paths);
                    if *cursor == previous || tokens.get(*cursor).is_some_and(|token| token == ",")
                    {
                        *cursor += 1;
                    }
                }
                *cursor += 1;
                return;
            }
            if !token
                .chars()
                .all(|ch| ch.is_ascii_alphanumeric() || ch == '_')
            {
                return;
            }
            if !path.is_empty() {
                path.push_str("::");
            }
            path.push_str(token);
            *cursor += 1;
            if tokens.get(*cursor).is_some_and(|token| token == ":")
                && tokens.get(*cursor + 1).is_some_and(|token| token == ":")
            {
                *cursor += 2;
            } else {
                paths.push(path);
                return;
            }
        }
    }

    let retired = [
        "layerx_crypto::disclosure::bind",
        "layerx_crypto::disclosure::bind_budget_mutation",
        "layerx_crypto::send::encode_send_envelope",
        "layerx_crypto::send::encode_payment_envelope",
        "layerx_crypto::onboarding",
        "layerx_crypto::rotation",
        "layerx_crypto::signer::sign_disclosed",
    ];
    let mut violations = Vec::new();
    for (file, source) in [
        ("src/server.rs", include_str!("../src/server.rs")),
        ("tests/provider.rs", include_str!("provider.rs")),
    ] {
        let mut tokens = Vec::new();
        let mut identifier = String::new();
        for ch in source.chars().chain(std::iter::once(' ')) {
            if ch.is_ascii_alphanumeric() || ch == '_' {
                identifier.push(ch);
            } else {
                if !identifier.is_empty() {
                    tokens.push(std::mem::take(&mut identifier));
                }
                if !ch.is_whitespace() {
                    tokens.push(ch.to_string());
                }
            }
        }
        let mut paths = Vec::new();
        for (mut cursor, token) in tokens.iter().enumerate() {
            if token == "layerx_crypto" {
                collect_paths(&tokens, &mut cursor, "", &mut paths);
            }
        }
        for path in paths {
            if retired.iter().any(|retired| {
                path == *retired
                    || path
                        .strip_prefix(retired)
                        .is_some_and(|suffix| suffix.starts_with("::"))
            }) {
                violations.push(format!("{file}: {path}"));
            }
        }
    }
    assert!(
        violations.is_empty(),
        "retired wiring.J44.25 paths remain:\n{}",
        violations.join("\n")
    );
}
