use serde_json::Value;
use std::collections::BTreeMap;

const ROUTER_METHODS: [&str; 17] = layerx_platform_gateway::ROUTER_METHODS;

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
