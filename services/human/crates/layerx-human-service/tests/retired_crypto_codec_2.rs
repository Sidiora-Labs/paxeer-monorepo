#[test]
fn no_retired_layerx_crypto_codec_paths_2() {
    let migrated_files = [
        ("intent_submit.rs", include_str!("intent_submit.rs")),
        ("journey_faults.rs", include_str!("journey_faults.rs")),
        ("key_export.rs", include_str!("key_export.rs")),
        ("move_money.rs", include_str!("move_money.rs")),
        ("reclaim.rs", include_str!("reclaim.rs")),
        ("render.rs", include_str!("render.rs")),
        ("session_keys.rs", include_str!("session_keys.rs")),
    ];
    let retired_identifiers = [
        "bind",
        "bind_budget_mutation",
        "encode_send_envelope",
        "encode_payment_envelope",
        "onboarding",
        "rotation",
        "sign_disclosed",
    ];

    let mut violations = Vec::new();
    for (path, source) in migrated_files {
        for statement in source.split(';') {
            let Some((_, crypto_path)) = statement.split_once("layerx_crypto") else {
                continue;
            };
            let identifiers: Vec<_> = crypto_path
                .split(|character: char| !character.is_alphanumeric() && character != '_')
                .filter(|identifier| !identifier.is_empty())
                .collect();
            for retired in retired_identifiers {
                if identifiers.contains(&retired) {
                    violations.push(format!("{path}: layerx_crypto path names {retired}"));
                }
            }
        }
    }
    assert!(
        violations.is_empty(),
        "retired wiring.J44.25 paths remain:\n{}",
        violations.join("\n")
    );
    assert!(
        include_str!("intent_submit.rs").contains("sign_disclosed_call("),
        "intent_submit.rs must use sign_disclosed_call (task 10.98.11 do_10); \
         expanding the retired signer into SigningRequest does not migrate the caller. \
         The existing binding caller uses a 0x7104 payout-binding payload, which \
         layerx_crypto::disclosure::call does not support, and imports the absent \
         layerx_human_service::binding module."
    );
}
