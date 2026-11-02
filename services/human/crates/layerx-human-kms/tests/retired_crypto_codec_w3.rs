#[test]
fn no_retired_layerx_crypto_codec_paths_w3() {
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
        ("src/attestor.rs", include_str!("../src/attestor.rs")),
        ("tests/attestor.rs", include_str!("attestor.rs")),
        (
            "custody/provider.rs",
            include_str!("../../layerx-human-service/src/custody/provider.rs"),
        ),
        (
            "custody/signer.rs",
            include_str!("../../layerx-human-service/src/custody/signer.rs"),
        ),
        (
            "custody/gateway.rs",
            include_str!("../../layerx-human-service/src/custody/gateway.rs"),
        ),
        (
            "server/production_components.rs",
            include_str!("../../layerx-human-service/src/server/production_components.rs"),
        ),
        (
            "server/agent_creation.rs",
            include_str!("../../layerx-human-service/src/server/agent_creation.rs"),
        ),
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

    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
    use sha3::{Digest as _, Keccak256};

    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1a]);
    let call = Eip1559Call {
        chain_id: 77,
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data: [
            Keccak256::digest(b"openAccount(bytes32)")[..4].to_vec(),
            vec![7; 32],
        ]
        .concat(),
    };
    let raw = call.sign(&[1; 32]).expect("valid secp256k1 key");
    let (decoded, signature) = decode_signed(&raw).expect("canonical signed call");
    let from = recover_sender(&decoded, &signature).expect("recover sender");
    assert_eq!(
        layerx_intents::owner_activity::attach_signature(&call, *signature.as_bytes(), from)
            .expect("attach verified call signature"),
        raw
    );
    let mut wrong_from = from;
    wrong_from[0] ^= 1;
    assert!(layerx_intents::owner_activity::attach_signature(
        &call,
        *signature.as_bytes(),
        wrong_from
    )
    .is_err());
    let mut changed = call.clone();
    changed.nonce += 1;
    assert!(layerx_intents::owner_activity::attach_signature(
        &changed,
        *signature.as_bytes(),
        from
    )
    .is_err());
    let mut undisclosable = call;
    undisclosable.to = [0; 20];
    assert!(layerx_intents::owner_activity::attach_signature(
        &undisclosable,
        *signature.as_bytes(),
        from
    )
    .is_err());
}
