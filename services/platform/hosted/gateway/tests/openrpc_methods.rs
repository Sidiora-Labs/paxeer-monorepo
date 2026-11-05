use serde_json::Value;
use std::collections::BTreeMap;

const ROUTER_METHODS: [&str; 17] = layerx_platform_gateway::ROUTER_METHODS;

fn px_list_assets_schema() -> Value {
    let document: Value = serde_json::from_str(include_str!("../openrpc.json")).unwrap();
    document["methods"]
        .as_array()
        .unwrap()
        .iter()
        .find(|method| method["name"] == "px_listAssets")
        .expect("px_listAssets must be documented")["result"]["schema"]
        .clone()
}

pub(crate) fn px_list_assets_result_matches_schema(result: &Value) -> bool {
    matches_asset_schema(result, &px_list_assets_schema())
}

// Validate the keywords used by px_listAssets without adding a runtime dependency.
// Refuse unknown keywords so a schema extension cannot silently bypass validation.
fn matches_asset_schema(value: &Value, schema: &Value) -> bool {
    for keyword in schema.as_object().unwrap().keys() {
        assert!(
            matches!(
                keyword.as_str(),
                "type"
                    | "required"
                    | "additionalProperties"
                    | "properties"
                    | "items"
                    | "maxItems"
                    | "minimum"
                    | "maximum"
                    | "pattern"
                    | "enum"
                    | "const"
            ),
            "unsupported schema keyword: {keyword}"
        );
    }
    let matches_type = |kind: &Value| match kind.as_str().unwrap() {
        "object" => value.is_object(),
        "array" => value.is_array(),
        "string" => value.is_string(),
        "integer" => value.is_i64() || value.is_u64(),
        "boolean" => value.is_boolean(),
        "null" => value.is_null(),
        kind => panic!("unsupported schema type: {kind}"),
    };
    let valid_type = match &schema["type"] {
        Value::Array(types) => types.iter().any(matches_type),
        kind => matches_type(kind),
    };
    if !valid_type {
        return false;
    }
    if let Some(allowed) = schema.get("enum") {
        if !allowed.as_array().unwrap().contains(value) {
            return false;
        }
    }
    if let Some(expected) = schema.get("const") {
        if value != expected {
            return false;
        }
    }
    if let Some(object) = value.as_object() {
        let properties = schema["properties"].as_object().unwrap();
        if let Some(required) = schema["required"].as_array() {
            if required
                .iter()
                .any(|key| !object.contains_key(key.as_str().unwrap()))
            {
                return false;
            }
        }
        for (key, member) in object {
            match properties.get(key) {
                Some(property) if !matches_asset_schema(member, property) => return false,
                None if schema["additionalProperties"] == false => return false,
                _ => {}
            }
        }
    }
    if let Some(items) = value.as_array() {
        if let Some(maximum) = schema["maxItems"].as_u64() {
            if items.len() as u64 > maximum {
                return false;
            }
        }
        if !items
            .iter()
            .all(|item| matches_asset_schema(item, &schema["items"]))
        {
            return false;
        }
    }
    if let Some(number) = value.as_f64() {
        if schema["minimum"]
            .as_f64()
            .is_some_and(|minimum| number < minimum)
            || schema["maximum"]
                .as_f64()
                .is_some_and(|maximum| number > maximum)
        {
            return false;
        }
    }
    if let (Some(text), Some(pattern)) = (value.as_str(), schema.get("pattern")) {
        assert_eq!(pattern, "^[0-9]+$", "unsupported asset schema pattern");
        if text.is_empty() || !text.bytes().all(|byte| byte.is_ascii_digit()) {
            return false;
        }
    }
    true
}

#[test]
#[allow(non_snake_case)]
fn px_listAssets() {
    let schema = px_list_assets_schema();
    assert!(!px_list_assets_result_matches_schema(&Value::Null));
    assert_eq!(schema["additionalProperties"], false);
    let row = &schema["properties"]["assets"]["items"];
    assert_eq!(row["additionalProperties"], false);
    assert_eq!(
        row["required"],
        serde_json::json!(["asset_id", "denom", "kind", "name", "symbol", "decimals", "supply"])
    );
    assert_eq!(
        row["properties"]["paused"],
        serde_json::json!({"type": "boolean"})
    );
    assert_eq!(
        row["properties"]["verification"],
        serde_json::json!({
            "type": "string", "enum": ["authenticated_committed_snapshot"]
        })
    );
}

#[test]
fn history_kind_examples_name_only_indexed_kinds() -> Result<(), Box<dyn std::error::Error>> {
    let document: Value = serde_json::from_str(include_str!("../openrpc.json"))?;
    let methods = document["methods"]
        .as_array()
        .ok_or("OpenRPC methods must be an array")?;
    for name in ["px_getHistory", "px_getUnifiedHistory"] {
        let method = methods
            .iter()
            .find(|method| method["name"] == name)
            .ok_or_else(|| format!("OpenRPC method {name} must exist"))?;
        let kind = method["params"]
            .as_array()
            .ok_or_else(|| format!("{name} must declare its params"))?
            .iter()
            .find(|param| param["name"] == "kind")
            .ok_or_else(|| format!("{name} must declare its kind parameter"))?;
        let description = kind["description"]
            .as_str()
            .ok_or_else(|| format!("{name} kind must have a string description"))?;
        assert_eq!(
            description,
            "Only rows of this indexer kind, for example native_transfer, erc20_transfer, pointer_transfer or bank_transfer.",
            "{name} kind examples must name only indexed kinds"
        );
        assert!(!description.contains("custody_deposit"), "{name}");
        assert!(!description.contains("lxp_transfer"), "{name}");
    }
    Ok(())
}

#[test]
fn openrpc_lists_every_router_method_once() -> Result<(), Box<dyn std::error::Error>> {
    let document: Value = serde_json::from_str(include_str!("../openrpc.json"))?;
    assert_eq!(document["openrpc"], "1.3.2");
    let methods = document["methods"]
        .as_array()
        .ok_or("OpenRPC methods must be an array")?;
    let mut counts = BTreeMap::new();
    for method in methods {
        let name = method["name"]
            .as_str()
            .ok_or("every OpenRPC method must have a string name")?;
        *counts.entry(name).or_insert(0_usize) += 1;
    }
    for name in ROUTER_METHODS {
        assert_eq!(
            counts.get(name).copied().unwrap_or(0),
            1,
            "router method {name} must appear exactly once in /rpc/schema"
        );
        let method = methods
            .iter()
            .find(|method| method["name"] == name)
            .ok_or("router method must have an OpenRPC entry")?;
        assert_eq!(method["paramStructure"], "by-position", "{name}");
        assert!(
            method["params"].is_array(),
            "{name} must declare its params"
        );
        assert!(
            method["result"]["schema"].is_object(),
            "{name} must declare its result schema"
        );
    }
    Ok(())
}
