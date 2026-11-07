use layerx_mcp::binding::external_signature_request;
use layerx_mcp::boundary::BoundaryRefusal;
use layerx_mcp::catalogue::{input_schema, validate, ArgumentError};
use serde_json::{json, Value};

#[test]
fn activity_sign_refuses_a_64_byte_signature() {
    let mut arguments = json!({
        "variant": "external_signature_v1",
        "preparation_ref": "11".repeat(32),
        "signature": "22".repeat(64),
    });
    assert_eq!(
        external_signature_request(&arguments),
        Err(BoundaryRefusal::Malformed("mcp.external_signature".into()))
    );
    assert_eq!(
        validate("activity.sign", &arguments),
        Err(ArgumentError::Malformed("mcp.external_signature"))
    );

    arguments["signature"] = json!("22".repeat(65));
    let request = external_signature_request(&arguments).expect("65-byte external signature");
    assert_eq!(request.get("variant"), None);
    assert_eq!(request["signature"], arguments["signature"]);
    assert_eq!(request["preparation_ref"], arguments["preparation_ref"]);
    assert!(validate("activity.sign", &arguments).is_ok());

    for signature in [
        json!("22".repeat(63)),
        json!("22".repeat(66)),
        json!("ZZ".repeat(65)),
        json!("AA".repeat(65)),
        Value::Null,
    ] {
        arguments["signature"] = signature;
        assert_eq!(
            external_signature_request(&arguments),
            Err(BoundaryRefusal::Malformed("mcp.external_signature".into()))
        );
    }
}

#[test]
fn activity_submit_schema_declares_signer_address() {
    let schema = input_schema("activity.submit").expect("activity.submit schema");
    let variants = schema["oneOf"].as_array().expect("submit variants");
    assert_eq!(variants.len(), 2);
    let evm = variants
        .iter()
        .find_map(|branch| branch["oneOf"].as_array())
        .expect("EVM submission variants");
    assert_eq!(evm.len(), 2);
    let native = evm
        .iter()
        .find(|branch| {
            branch.pointer("/properties/variant/enum/0") == Some(&json!("native_send_submit_v1"))
        })
        .expect("native submit schema");
    assert_eq!(native["type"], json!("object"));
    assert_eq!(native["additionalProperties"], json!(false));
    let properties = native["properties"].as_object().expect("submit properties");
    assert!(!properties.contains_key("signer_public_key"));
    let required = native["required"].as_array().expect("required fields");
    assert_eq!(
        required,
        json!(["variant", "preparation_ref", "signature", "signer_address"])
            .as_array()
            .expect("native required fields")
    );
    assert!(required.contains(&json!("signature")));
    assert!(required.contains(&json!("signer_address")));
    assert!(!required.contains(&json!("signer_public_key")));
    for (field, bytes) in [("signature", 65), ("signer_address", 20)] {
        let field_schema = &properties[field];
        assert_eq!(field_schema["type"], json!("string"));
        assert_eq!(field_schema["minLength"], json!(bytes * 2));
        assert_eq!(field_schema["maxLength"], json!(bytes * 2));
        assert_eq!(
            field_schema["pattern"],
            json!(format!("^[0-9a-f]{{{}}}$", bytes * 2))
        );
    }

    let raw = evm
        .iter()
        .find(|branch| branch.pointer("/properties/raw_transaction").is_some())
        .expect("raw EIP-1559 submission schema");
    assert_eq!(raw["type"], json!("object"));
    assert_eq!(raw["additionalProperties"], json!(false));
    assert_eq!(raw["required"], json!(["raw_transaction"]));
    assert_eq!(raw["properties"].as_object().unwrap().len(), 1);
    assert_eq!(
        raw["properties"]["raw_transaction"]["type"],
        json!("string")
    );
    assert_eq!(
        raw["properties"]["raw_transaction"]["pattern"],
        json!("^0x02([0-9a-f]{2})+$")
    );
    assert_eq!(
        raw["properties"]["raw_transaction"]["maxLength"],
        json!(65_536)
    );
}

#[test]
fn activity_submit_validates_a_real_signed_eip1559_transaction(
) -> Result<(), Box<dyn std::error::Error>> {
    use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
    use std::fmt::Write as _;

    let call = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 21_000,
        to: [0x11; 20],
        data: Vec::new(),
    };
    let signed = call.sign(&[0x42; 32])?;
    let (decoded, signature) = decode_signed(&signed)?;
    assert_eq!(decoded, call);
    assert_ne!(recover_sender(&decoded, &signature)?, [0; 20]);
    let mut raw = String::from("0x");
    for byte in signed {
        write!(raw, "{byte:02x}")?;
    }
    assert!(validate("activity.submit", &json!({"raw_transaction": raw})).is_ok());
    for rejected in [
        raw.trim_start_matches("0x").to_owned(),
        raw.to_uppercase(),
        raw.replacen("0x02", "0x01", 1),
        format!("{raw}0"),
        "0x02".to_owned(),
        format!("0x02{}", "00".repeat(32_768)),
    ] {
        assert!(validate("activity.submit", &json!({"raw_transaction": rejected})).is_err());
    }
    for field in ["signature", "signer_address", "variant"] {
        let mut widened = json!({"raw_transaction": raw});
        widened[field] = json!("11");
        assert_eq!(
            validate("activity.submit", &widened),
            Err(ArgumentError::Unknown(field.to_owned()))
        );
    }
    Ok(())
}
