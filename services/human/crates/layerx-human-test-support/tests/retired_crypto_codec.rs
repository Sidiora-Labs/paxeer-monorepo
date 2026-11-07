#[test]
fn no_retired_layerx_crypto_codec_paths() {
    let sources = [("src/lib.rs", include_str!("../src/lib.rs"))];
    let retired_names = [
        "bind_budget_mutation",
        "encode_send_envelope",
        "encode_payment_envelope",
        "sign_disclosed",
    ];
    let retired_paths: &[&[&str]] = &[
        &["layerx_crypto", "disclosure", "bind"],
        &["layerx_crypto", "onboarding"],
        &["layerx_crypto", "rotation"],
    ];

    for (path, source) in sources {
        let tokens: Vec<&str> = source
            .split(|character: char| !character.is_ascii_alphanumeric() && character != '_')
            .filter(|token| !token.is_empty())
            .collect();
        for retired in retired_names {
            assert!(
                !tokens.contains(&retired),
                "{path} still names retired codec {retired} from wiring.J44.25"
            );
        }
        for retired in retired_paths {
            assert!(
                !tokens
                    .windows(retired.len())
                    .any(|window| window == *retired),
                "{path} still names retired path {} from wiring.J44.25",
                retired.join("::")
            );
        }
        for statement in source.split(';') {
            let tokens: Vec<&str> = statement
                .split(|character: char| !character.is_ascii_alphanumeric() && character != '_')
                .filter(|token| !token.is_empty())
                .collect();
            if tokens.first() != Some(&"use") || !tokens.contains(&"layerx_crypto") {
                continue;
            }
            for retired in ["onboarding", "rotation"] {
                assert!(
                    !tokens.contains(&retired),
                    "{path} still imports retired layerx_crypto::{retired} from wiring.J44.25"
                );
            }
            if tokens.contains(&"disclosure") {
                assert!(
                    !tokens.contains(&"bind"),
                    "{path} still imports retired disclosure::bind from wiring.J44.25"
                );
            }
        }
    }
}
