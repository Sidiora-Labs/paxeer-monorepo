#[path = "binding.rs"]
mod binding;

#[test]
fn no_retired_layerx_crypto_codec_paths_w3() {
    let migrated_files = [("binding.rs", include_str!("binding.rs"))];
    let retired_identifiers = [
        "bind",
        "bind_budget_mutation",
        "encode_send_envelope",
        "encode_payment_envelope",
        "onboarding",
        "rotation",
        "sign_disclosed",
    ];

    for (path, source) in migrated_files {
        let identifiers: Vec<_> = source
            .split(|character: char| !character.is_alphanumeric() && character != '_')
            .filter(|identifier| !identifier.is_empty())
            .collect();
        for retired in retired_identifiers {
            assert!(
                !identifiers.contains(&retired),
                "{path} still names retired wiring.J44.25 codec identifier {retired}"
            );
        }
    }
}
