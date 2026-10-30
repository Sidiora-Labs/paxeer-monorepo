#[test]
fn platform_names_no_kernel_disclosure_or_send_codec() {
    macro_rules! source {
        ($path:literal) => {
            (
                $path,
                include_str!(concat!(env!("CARGO_MANIFEST_DIR"), "/../../../../", $path)),
            )
        };
    }
    let paths = [
        source!("services/platform/hosted/authority/src/human/dynamic.rs"),
        source!("services/platform/hosted/authority/src/human/dynamic/tests.rs"),
        source!("services/platform/hosted/core/src/lib.rs"),
        source!("services/platform/hosted/gateway/src/lib.rs"),
        source!("services/platform/hosted/gateway/tests/local/funding.rs"),
        source!("services/platform/hosted/gateway/tests/local/withdrawal.rs"),
        source!("tools/platform/cli/examples/hosted-send.rs"),
        source!("tools/platform/cli/src/wallet.rs"),
        source!("tools/platform/cli/src/wallet_send.rs"),
        source!("tools/platform/cli/src/wallet_signing.rs"),
    ];
    let forbidden = [
        "layerx_crypto::disclosure",
        "layerx_crypto::send",
        "layerx_crypto::onboarding",
        "layerx_crypto::signer::sign_disclosed",
    ];
    let mut violations = Vec::new();
    for (path, source) in paths {
        for (index, line) in source.lines().enumerate() {
            for codec in forbidden {
                if line.match_indices(codec).any(|(start, name)| {
                    line.as_bytes()
                        .get(start + name.len())
                        .is_none_or(|byte| !byte.is_ascii_alphanumeric() && *byte != b'_')
                }) {
                    violations.push(format!("{path}:{} names {codec}", index + 1));
                }
            }
        }
    }
    assert!(
        violations.is_empty(),
        "platform still names kernel disclosure or send codecs:\n{}",
        violations.join("\n")
    );
}
