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
    let native = schema["oneOf"]
        .as_array()
        .expect("submit variants")
        .iter()
        .find(|branch| {
            branch.pointer("/properties/variant/enum/0") == Some(&json!("native_send_submit_v1"))
        })
        .expect("native submit schema");
    assert_eq!(native["additionalProperties"], json!(false));
    let properties = native["properties"].as_object().expect("submit properties");
    assert!(!properties.contains_key("signer_public_key"));
    let required = native["required"].as_array().expect("required fields");
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
}
