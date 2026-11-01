use serde_json::Value;
use x_websearch::config::{parse_hex32, Config};

const EXAMPLE: &str = include_str!("../config.example.json");
const RECEIPT_PUBLIC_KEY: &str = "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a";

fn substitute_placeholders(value: &mut Value) {
    match value {
        Value::Object(fields) => {
            for field in fields.values_mut() {
                substitute_placeholders(field);
            }
        }
        Value::Array(items) => {
            for item in items {
                substitute_placeholders(item);
            }
        }
        Value::String(text) if text.contains("${") => {
            *text = match text.as_str() {
                "${X_WEBSEARCH_SEED_URL}" => "https://paxeer.app/".to_owned(),
                "${X_WEBSEARCH_SID_ASSET_ID}" => format!("01{}", "23".repeat(31)),
                "${X_WEBSEARCH_PAX_ASSET_ID}" => format!("02{}", "23".repeat(31)),
                "${X_WEBSEARCH_USDC_ASSET_ID}" => format!("03{}", "23".repeat(31)),
                "${X_WEBSEARCH_USDL_ASSET_ID}" => format!("04{}", "23".repeat(31)),
                "${X_WEBSEARCH_SID_PRICE}" => "3114".to_owned(),
                "${X_WEBSEARCH_PAX_PRICE}" => "1000".to_owned(),
                "${X_WEBSEARCH_USDC_PRICE}" => "1000".to_owned(),
                "${X_WEBSEARCH_USDL_PRICE}" => "1000000000000000".to_owned(),
                "${X_WEBSEARCH_GATEWAY_ENDPOINT}" => "http://127.0.0.1:8545/rpc".to_owned(),
                "${X_WEBSEARCH_EVM_ENDPOINT}" => "http://127.0.0.1:8545".to_owned(),
                "${X_WEBSEARCH_RECEIPT_KEY_ID}" => format!("ab{}", "cd".repeat(31)),
                "${X_WEBSEARCH_RECEIPT_PUBLIC_KEY}" => RECEIPT_PUBLIC_KEY.to_owned(),
                "${X_WEBSEARCH_PAYER_ADDRESS}" => format!("0x{}", "3c".repeat(20)),
                other => panic!("unrecognized example placeholder: {other}"),
            };
        }
        _ => {}
    }
}

#[test]
fn the_committed_example_configuration_parses() -> Result<(), Box<dyn std::error::Error>> {
    let mut example: Value = serde_json::from_str(EXAMPLE)?;
    substitute_placeholders(&mut example);
    assert!(!example.to_string().contains("${"));
    assert!(example.get("kernel").is_none());

    let config = Config::parse(&example.to_string())?;
    assert_eq!(config.payment.payer_address, Some([0x3c; 20]));
    assert_eq!(config.gateway.endpoint, "http://127.0.0.1:8545/rpc");
    assert_eq!(config.gateway.receipt_keys.len(), 1);
    let key = &config.gateway.receipt_keys[0];
    assert_eq!(
        Some(key.sequencer_id()),
        parse_hex32(&format!("ab{}", "cd".repeat(31)))
    );
    assert_eq!(Some(key.public_key()), parse_hex32(RECEIPT_PUBLIC_KEY));
    assert_eq!(key.first_batch_number(), 0);
    assert_eq!(key.last_batch_number(), 1_000_000);
    Ok(())
}

fn assert_no_retired_field(value: &Value) {
    match value {
        Value::Object(fields) => {
            for (name, field) in fields {
                assert!(
                    !matches!(
                        name.as_str(),
                        "kernel" | "authorization_file" | "payer_did" | "sequencer_id"
                    ),
                    "example names retired field {name}"
                );
                assert_no_retired_field(field);
            }
        }
        Value::Array(items) => {
            for item in items {
                assert_no_retired_field(item);
            }
        }
        _ => {}
    }
}

#[test]
fn the_example_configuration_names_no_retired_field() -> Result<(), Box<dyn std::error::Error>> {
    let example: Value = serde_json::from_str(EXAMPLE)?;
    assert_no_retired_field(&example);
    let gateway = example["gateway"].as_object().ok_or("missing gateway")?;
    assert_eq!(gateway.len(), 2);
    assert!(gateway.contains_key("endpoint"));
    assert!(gateway.contains_key("receipt_keys"));
    assert!(example["payment"].get("payer_address").is_some());
    Ok(())
}
