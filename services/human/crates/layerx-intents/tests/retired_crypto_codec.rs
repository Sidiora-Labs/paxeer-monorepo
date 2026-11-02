#[test]
fn no_retired_layerx_crypto_codec_paths() {
    let sources = [
        ("src/canonical.rs", include_str!("../src/canonical.rs")),
        (
            "src/vectors/send.rs",
            include_str!("../src/vectors/send.rs"),
        ),
        ("tests/compile.rs", include_str!("compile.rs")),
    ];
    let retired_paths = [
        concat!("layerx_crypto::disclosure::", "bind"),
        "layerx_crypto::disclosure::bind_budget_mutation",
        "layerx_crypto::send::encode_send_envelope",
        "layerx_crypto::send::encode_payment_envelope",
        "layerx_crypto::onboarding",
        "layerx_crypto::rotation",
        "layerx_crypto::signer::sign_disclosed",
    ];

    let mut violations = Vec::new();
    for (file, source) in sources {
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for retired in retired_paths {
            for (offset, matched) in compact.match_indices(retired) {
                let next = compact[offset + matched.len()..].chars().next();
                if next.is_none_or(|ch| ch != '_' && !ch.is_alphanumeric()) {
                    violations.push(format!("{file}: {retired}"));
                }
            }
        }
    }
    assert!(
        violations.is_empty(),
        "retired wiring.J44.25 paths remain:\n{}",
        violations.join("\n")
    );
}
