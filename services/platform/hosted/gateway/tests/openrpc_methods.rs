use serde_json::Value;
use std::collections::BTreeMap;

const ROUTER_METHODS: [&str; 17] = layerx_platform_gateway::ROUTER_METHODS;

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
