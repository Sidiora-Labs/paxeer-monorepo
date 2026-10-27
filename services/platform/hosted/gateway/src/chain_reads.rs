use super::{public_reads, rpc, Config, Endpoint};
use layerx_platform_gateway::http::OutboundRequest;
use serde_json::{json, Value};

fn unavailable(id: &Value, code: &str) -> Value {
    let mut answer = rpc::error(id, -32003, "Chain read unavailable");
    answer["error"]["data"] = json!({"code": code});
    answer
}

fn decimal(value: &Value) -> Option<u64> {
    let text = value.as_str()?;
    if text.is_empty() || !text.bytes().all(|byte| byte.is_ascii_digit()) {
        return None;
    }
    text.parse().ok()
}

fn height(params: Option<&Value>) -> Option<u64> {
    let [value] = params?.as_array()?.as_slice() else {
        return None;
    };
    decimal(value).filter(|height| *height > 0)
}

fn consensus(config: &Config, method: &str, id: &Value, params: Value) -> Option<Value> {
    let mut endpoint = std::env::var("LAYERX_GATEWAY_CONSENSUS_RPC_URL")
        .ok()
        .and_then(|url| Endpoint::parse(&url).ok())?;
    let path = if endpoint.base_path.is_empty() {
        "/".to_owned()
    } else {
        std::mem::take(&mut endpoint.base_path)
    };
    let body = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": id, "method": method, "params": params
    }))
    .ok()?;
    let response = config
        .client
        .request_unauthenticated(
            &endpoint,
            &OutboundRequest {
                method: "POST",
                path: &path,
                idempotency: None,
                content_type: "application/json",
                body: &body,
            },
        )
        .ok()?;
    if response.status != 200 || response.content_type != "application/json" {
        return None;
    }
    let document: Value = serde_json::from_slice(&response.body).ok()?;
    if document["jsonrpc"] != "2.0"
        || document.get("id") != Some(id)
        || document.get("error").is_some()
    {
        return None;
    }
    document
        .get("result")
        .filter(|result| result.is_object())
        .cloned()
}

#[allow(non_snake_case)]
fn px_getCommit(config: &Config, id: &Value, height: u64) -> Option<Value> {
    let result = consensus(config, "commit", id, json!({"height": height.to_string()}))?;
    let signed_header = result
        .get("signed_header")
        .filter(|header| header.is_object())?;
    if decimal(&signed_header["header"]["height"]) != Some(height)
        || !signed_header["commit"].is_object()
    {
        return None;
    }
    Some(json!({"height": height.to_string(), "signedHeader": signed_header}))
}

#[allow(non_snake_case)]
fn px_getValidators(config: &Config, id: &Value, height: u64) -> Option<Value> {
    let mut validators = Vec::new();
    let mut page = 1_u64;
    let mut total = None;
    loop {
        let result = consensus(
            config,
            "validators",
            id,
            json!({
                "height": height.to_string(), "page": page.to_string(), "per_page": "100"
            }),
        )?;
        if decimal(&result["block_height"]) != Some(height) {
            return None;
        }
        let count = decimal(&result["total"])?;
        if total.is_some_and(|total| total != count) {
            return None;
        }
        total = Some(count);
        let entries = result["validators"].as_array()?;
        if decimal(&result["count"])? != entries.len() as u64 || entries.len() > 100 {
            return None;
        }
        for entry in entries {
            let address = entry.get("address").filter(|value| value.is_string())?;
            let public_key = entry.get("pub_key").filter(|value| value.is_object())?;
            let power = entry.get("voting_power")?;
            decimal(power)?;
            validators
                .push(json!({"address": address, "pubKey": public_key, "votingPower": power}));
        }
        if validators.len() as u64 == count {
            return Some(
                json!({"height": height.to_string(), "validators": validators, "total": count.to_string()}),
            );
        }
        if entries.is_empty() || validators.len() as u64 > count {
            return None;
        }
        page = page.checked_add(1)?;
    }
}

#[allow(non_snake_case)]
fn px_getReceiptKeys(config: &Config, id: &Value, params: Option<&Value>) -> Value {
    if !params.is_some_and(|params| params.as_array().is_some_and(Vec::is_empty)) {
        return rpc::error(id, -32602, "Invalid params");
    }
    let Some(list) = &config.receipt_keys else {
        return unavailable(id, "receipt_keys_unavailable");
    };
    let keys: Vec<_> = list
        .entries()
        .iter()
        .map(|key| {
            json!({
                "keyId": format!("0x{}", super::hex(&key.sequencer_id())),
                "publicKey": format!("0x{}", super::hex(&key.public_key())),
                "firstHeight": key.first_batch_number(),
                "lastHeight": key.last_batch_number(),
            })
        })
        .collect();
    json!({"jsonrpc": "2.0", "id": id, "result": {"keys": keys}})
}

pub(super) fn dispatch(
    config: &Config,
    method: &str,
    id: &Value,
    params: Option<&Value>,
) -> Option<Value> {
    if !matches!(
        method,
        "px_getCommit" | "px_getValidators" | "px_getReceiptKeys"
    ) {
        return None;
    }
    if !public_reads::consume_read() {
        let mut answer = rpc::error(id, -32005, "Read unavailable");
        answer["error"]["data"] = json!({"code": "public_read_rate_limit"});
        return Some(answer);
    }
    if method == "px_getReceiptKeys" {
        return Some(px_getReceiptKeys(config, id, params));
    }
    let Some(height) = height(params) else {
        return Some(rpc::error(id, -32602, "Invalid params"));
    };
    let result = match method {
        "px_getCommit" => px_getCommit(config, id, height),
        "px_getValidators" => px_getValidators(config, id, height),
        _ => unreachable!(),
    };
    Some(result.map_or_else(
        || unavailable(id, "consensus_unavailable"),
        |result| json!({"jsonrpc": "2.0", "id": id, "result": result}),
    ))
}
