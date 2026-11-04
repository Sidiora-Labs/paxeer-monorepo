//! Project the merged node deployment's authority network into the developer manifest.

use serde::Deserialize;
use serde_yaml::Value;

fn documents(manifest: &str) -> Result<Vec<Value>, String> {
    serde_yaml::Deserializer::from_str(manifest)
        .map(|document| Value::deserialize(document).map_err(|error| error.to_string()))
        .collect()
}

/// Bind the webhook verifier to the same ConfigMap key the authority consumes.
///
/// # Errors
/// Refuses missing, duplicate, ambiguous or invalid network configuration.
pub fn bind_network(node: &str, webhooks: &str) -> Result<String, String> {
    let nodes = documents(node)?;
    let configs: Vec<_> = nodes
        .iter()
        .filter(|document| {
            document["kind"].as_str() == Some("ConfigMap")
                && document["metadata"]["name"].as_str() == Some("layerx-node-config")
        })
        .collect();
    if configs.len() != 1 {
        return Err("expected exactly one layerx-node-config ConfigMap".into());
    }
    let network = configs[0]["data"]["network-name"]
        .as_str()
        .ok_or("expected the node network-name scalar")?;
    if network.is_empty()
        || network.len() > 64
        || !network
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || b"_.-".contains(&byte))
    {
        return Err("invalid node network-name".into());
    }
    let bindings: Vec<_> = nodes
        .iter()
        .filter_map(|document| document["spec"]["template"]["spec"]["containers"].as_sequence())
        .flatten()
        .filter_map(|container| container["env"].as_sequence())
        .flatten()
        .filter(|entry| entry["name"].as_str() == Some("LAYERX_AUTHORITY_NETWORK_ID"))
        .collect();
    if bindings.len() != 1
        || !bindings[0]["value"].is_null()
        || bindings[0]["valueFrom"]["configMapKeyRef"]["name"].as_str()
            != Some("layerx-node-config")
        || bindings[0]["valueFrom"]["configMapKeyRef"]["key"].as_str() != Some("network-name")
    {
        return Err("authority must reference layerx-node-config/network-name".into());
    }
    let developers = documents(webhooks)?;
    let targets = developers
        .iter()
        .filter(|document| {
            document["kind"].as_str() == Some("ConfigMap")
                && document["data"]["LAYERX_WEBHOOKS_NETWORK_ID"]
                    .as_str()
                    .is_some()
        })
        .count();
    let prefix = "  LAYERX_WEBHOOKS_NETWORK_ID:";
    if targets != 1
        || webhooks.matches("LAYERX_WEBHOOKS_NETWORK_ID").count() != 1
        || webhooks
            .lines()
            .filter(|line| line.starts_with(prefix))
            .count()
            != 1
    {
        return Err("expected one webhook ConfigMap network binding without overrides".into());
    }
    let quoted = serde_json::to_string(network).map_err(|error| error.to_string())?;
    Ok(webhooks
        .split_inclusive('\n')
        .map(|line| {
            if line.starts_with(prefix) {
                format!(
                    "{prefix} {quoted}{}",
                    if line.ends_with('\n') { "\n" } else { "" }
                )
            } else {
                line.to_owned()
            }
        })
        .collect())
}
