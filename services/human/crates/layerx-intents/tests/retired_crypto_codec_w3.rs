#[test]
fn no_retired_layerx_crypto_codec_paths_w3() {
    let sources = [
        ("src/compile.rs", include_str!("../src/compile.rs")),
        ("src/disclosure.rs", include_str!("../src/disclosure.rs")),
        ("src/vocabulary.rs", include_str!("../src/vocabulary.rs")),
        ("tests/live_kinds.rs", include_str!("live_kinds.rs")),
    ];
    let retired_paths = [
        "disclosure::bind",
        "disclosure::bind_budget_mutation",
        "send::encode_send_envelope",
        "send::encode_payment_envelope",
        "onboarding",
        "rotation",
        "signer::sign_disclosed",
    ];

    let mut violations = Vec::new();
    for (file, source) in sources {
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for retired in retired_paths {
            let path = format!("layerx_crypto::{retired}");
            for (offset, matched) in compact.match_indices(&path) {
                let next = compact[offset + matched.len()..].chars().next();
                if next.is_none_or(|ch| ch != '_' && !ch.is_alphanumeric()) {
                    violations.push(format!("{file}: {path}"));
                }
            }
        }
    }
    let caller_sources = [
        (
            "agents/create_native.rs",
            include_str!("../../layerx-human-service/src/agents/create_native.rs"),
        ),
        (
            "server/agent_creation_native.rs",
            include_str!("../../layerx-human-service/src/server/agent_creation_native.rs"),
        ),
        (
            "server/production_onboarding.rs",
            include_str!("../../layerx-human-service/src/server/production_onboarding.rs"),
        ),
        (
            "server/production_rotation.rs",
            include_str!("../../layerx-human-service/src/server/production_rotation.rs"),
        ),
    ];
    for (file, source) in sources.into_iter().chain(caller_sources) {
        let compact: String = source.chars().filter(|ch| !ch.is_whitespace()).collect();
        for retired in [
            "IntentKind::NativeOnboarding(",
            "IntentKind::NativeOnboardingConsent(",
            "IntentKind::NativeOwnerRotation(",
        ] {
            if compact.contains(retired) {
                violations.push(format!("{file}: {retired}"));
            }
        }
    }
    assert!(
        violations.is_empty(),
        "retired wiring.J44.25 paths remain:\n{}",
        violations.join("\n")
    );
}
