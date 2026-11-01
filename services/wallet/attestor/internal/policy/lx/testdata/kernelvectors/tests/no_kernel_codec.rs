use std::process::Command;

#[test]
fn kernelvectors_use_no_kernel_codec() {
    let source = include_str!("../src/main.rs");
    for retired in [
        "layerx_crypto::disclosure",
        "layerx_crypto::send",
        "encode_send_envelope",
        "encode_payment_envelope",
        "bind_budget_mutation",
        "EnvelopeOptions",
    ] {
        assert!(!source.contains(retired), "generator still uses {retired}");
    }
    let output = Command::new(env!("CARGO_BIN_EXE_attestor-kernel-vectors"))
        .output()
        .expect("run the kernel vector generator");
    assert!(
        output.status.success(),
        "generator failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(
        output.stdout.as_slice(),
        include_bytes!("../../kernel_activities.json")
    );
    let vectors: serde_json::Value =
        serde_json::from_slice(&output.stdout).expect("generator writes JSON vectors");
    let activities = vectors["activities"].as_array().expect("activities array");
    let expected = [
        ("native-send", true),
        ("token-send", true),
        ("tampered-authorization-send", false),
        ("bare-transfer", false),
        ("budget-fund", true),
        ("untagged-allowance", false),
        ("grant-issue", true),
        ("program-transfer", true),
    ];
    assert_eq!(activities.len(), expected.len());
    for (activity, (name, accepted)) in activities.iter().zip(expected) {
        assert_eq!(activity["name"], name);
        assert_eq!(activity["accepted"], accepted);
        for field in ["payload", "unsigned", "signature_preimage"] {
            let bytes = activity[field].as_str().expect("hex vector field");
            assert!(!bytes.is_empty(), "{name}: empty {field}");
            assert_eq!(bytes.len() % 2, 0, "{name}: odd hex {field}");
            assert!(bytes.bytes().all(|byte| byte.is_ascii_hexdigit()));
        }
        assert_eq!(activity["signature_preimage"].as_str().unwrap().len(), 64);
    }
}
