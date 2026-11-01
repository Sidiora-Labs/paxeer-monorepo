use serde_json::{json, Value};

use crate::http::Client;

#[cfg(test)]
const PROGRAM_EVENTS_MAX_PAGE: u64 = 256;
#[cfg(test)]
const PROGRAM_EVENT_MAX_DATA_BYTES: usize = 65_536;

pub struct RpcClient {
    client: Client,
    url: String,
    credential: Option<zeroize::Zeroizing<String>>,
    identity_session: bool,
}

impl RpcClient {
    /// # Errors
    /// Requires the gateway's explicit /rpc endpoint and the HTTP client's TLS rules.
    pub fn new(url: &str, credential: Option<zeroize::Zeroizing<String>>) -> Result<Self, String> {
        let base = url.strip_suffix("/rpc").ok_or("RPC URL must end in /rpc")?;
        let client = match credential.clone() {
            Some(value) => Client::new_gateway(base, value)?,
            None => Client::new(base, None)?,
        };
        Ok(Self {
            client,
            url: url.to_owned(),
            credential,
            identity_session: false,
        })
    }

    /// # Errors
    /// Requires an explicit identity session, an /rpc endpoint, and verified HTTPS off loopback.
    pub fn new_session(url: &str, session: zeroize::Zeroizing<String>) -> Result<Self, String> {
        let base = url.strip_suffix("/rpc").ok_or("RPC URL must end in /rpc")?;
        if session.is_empty() {
            return Err("identity_session_required: an empty identity session is invalid".into());
        }
        Ok(Self {
            client: Client::new(base, Some(session))?,
            url: url.to_owned(),
            credential: None,
            identity_session: true,
        })
    }

    /// # Errors
    /// Refuses unsupported methods, invalid parameters, transport failures and RPC errors.
    pub fn call(&self, method: &str, params: &Value) -> Result<Value, String> {
        if matches!(method, "eth_subscribe" | "eth_unsubscribe") {
            return Err(format!(
                "rpc_transport_required: {method} requires authenticated WebSocket transport"
            ));
        }
        let request = request(method, params)?;
        if method == "px_requestFunds" && !self.identity_session {
            return Err(
                "identity_session_required: px_requestFunds requires a bearer identity session"
                    .into(),
            );
        }
        decode_response(method, &self.client.post("/rpc", &request, None)?)
    }

    /// # Errors
    /// Requires a gateway credential and a valid live subscription; notifications are unverified.
    pub fn subscribe(&self, params: &Value, timeout: std::time::Duration) -> Result<Value, String> {
        let credential = self
            .credential
            .as_ref()
            .ok_or("gateway_credential_required: subscriptions require --gateway-credential")?;
        crate::rpc_subscription::next(&self.url, credential, params, timeout)
    }
}

/// # Errors
/// Validates positional parameters against the published `OpenRPC` contract.
pub fn request(method: &str, params: &Value) -> Result<Value, String> {
    if method.starts_with("lx_") {
        return Err(format!("rpc_method_unavailable: {method} is switched off"));
    }
    let args = params
        .as_array()
        .ok_or("RPC parameters must be positional")?;
    match method {
        "eth_sendRawTransaction" => raw_transaction_params(args)?,
        "eth_getTransactionCount" => transaction_count_params(args)?,
        "pax_sequencerReceipt" => {
            let [hash] = args.as_slice() else {
                return Err("pax_sequencerReceipt requires one transaction hash".into());
            };
            if !ethereum_data(hash, Some(32)) {
                return Err(
                    "pax_sequencerReceipt requires a 0x-prefixed 32-byte transaction hash".into(),
                );
            }
        }
        "eth_estimateGas" => estimate_gas_params(args)?,
        "eth_gasPrice" if args.is_empty() => {}
        "eth_gasPrice" => return Err("eth_gasPrice takes no parameters".into()),
        "px_register" => registration_params(args)?,
        "px_requestFunds" => faucet_params(args)?,
        "eth_subscribe" => logs_subscription_params(args)?,
        "eth_unsubscribe" => {
            let [Value::String(subscription)] = args.as_slice() else {
                return Err("eth_unsubscribe requires one subscription identifier".into());
            };
            if subscription.is_empty() || subscription.len() > 128 {
                return Err("invalid subscription identifier".into());
            }
        }
        _ => {
            return Err(format!(
                "rpc_method_unavailable: {method} is absent from the published OpenRPC contract"
            ))
        }
    }
    Ok(json!({"jsonrpc":"2.0","id":1,"method":method,"params":params}))
}

fn ethereum_quantity(value: &str) -> bool {
    value.strip_prefix("0x").is_some_and(|digits| {
        (1..=64).contains(&digits.len())
            && (digits == "0" || !digits.starts_with('0'))
            && digits
                .bytes()
                .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
    })
}

fn ethereum_data(value: &Value, bytes: Option<usize>) -> bool {
    value
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .is_some_and(|digits| {
            digits.len() <= 1_048_576
                && digits.len().is_multiple_of(2)
                && bytes.is_none_or(|bytes| digits.len() == bytes * 2)
                && digits
                    .bytes()
                    .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
        })
}

fn raw_transaction_params(args: &[Value]) -> Result<(), String> {
    let [raw] = args else {
        return Err("eth_sendRawTransaction requires one raw transaction".into());
    };
    if !ethereum_data(raw, None) || raw.as_str() == Some("0x") {
        return Err(
            "eth_sendRawTransaction requires non-empty 0x-prefixed transaction bytes".into(),
        );
    }
    Ok(())
}

fn transaction_count_params(args: &[Value]) -> Result<(), String> {
    let [address, Value::String(block)] = args else {
        return Err("eth_getTransactionCount requires an address and block selector".into());
    };
    if !ethereum_data(address, Some(20))
        || !(matches!(
            block.as_str(),
            "pending" | "latest" | "earliest" | "safe" | "finalized"
        ) || ethereum_quantity(block))
    {
        return Err(
            "eth_getTransactionCount requires a 20-byte address and valid block selector".into(),
        );
    }
    Ok(())
}

fn estimate_gas_params(args: &[Value]) -> Result<(), String> {
    let invalid = || {
        "eth_estimateGas requires an EIP-1559 transaction object and pending block tag".to_owned()
    };
    let [Value::Object(tx), Value::String(block)] = args else {
        return Err(invalid());
    };
    let quantities = [
        "chainId",
        "nonce",
        "maxPriorityFeePerGas",
        "maxFeePerGas",
        "gas",
        "value",
    ];
    if block != "pending"
        || tx.get("type").and_then(Value::as_str) != Some("0x2")
        || quantities.iter().any(|field| {
            !tx.get(*field)
                .and_then(Value::as_str)
                .is_some_and(ethereum_quantity)
        })
        || tx.get("chainId").and_then(Value::as_str) == Some("0x0")
        || !tx
            .get("data")
            .is_some_and(|value| ethereum_data(value, None))
        || tx
            .get("to")
            .is_some_and(|value| !ethereum_data(value, Some(20)))
        || tx
            .get("from")
            .is_some_and(|value| !ethereum_data(value, Some(20)))
        || tx.keys().any(|field| {
            !quantities.contains(&field.as_str())
                && !matches!(
                    field.as_str(),
                    "type" | "data" | "to" | "from" | "accessList"
                )
        })
    {
        return Err(invalid());
    }
    let access = tx
        .get("accessList")
        .and_then(Value::as_array)
        .ok_or_else(invalid)?;
    for entry in access {
        let entry = entry
            .as_object()
            .filter(|entry| entry.len() == 2)
            .ok_or_else(invalid)?;
        if !entry
            .get("address")
            .is_some_and(|value| ethereum_data(value, Some(20)))
            || !entry
                .get("storageKeys")
                .and_then(Value::as_array)
                .is_some_and(|keys| keys.iter().all(|key| ethereum_data(key, Some(32))))
        {
            return Err(invalid());
        }
    }
    Ok(())
}

fn registration_params(args: &[Value]) -> Result<(), String> {
    let [Value::String(signer), Value::String(signature)] = args else {
        return Err("px_register requires signer_public_key and registration_signature".into());
    };
    if !lowercase_hex(signer, 32) || !lowercase_hex(signature, 64) {
        return Err("px_register requires a lowercase 32-byte key and 64-byte signature".into());
    }
    Ok(())
}

fn faucet_params(args: &[Value]) -> Result<(), String> {
    let [Value::String(did), Value::String(signer)] = args else {
        return Err("px_requestFunds requires DID and signer_public_key".into());
    };
    layerx_types::ids::Did::new(did.as_bytes()).map_err(|e| format!("invalid DID: {e:?}"))?;
    let valid_did = did
        .strip_prefix("did:")
        .and_then(|value| value.split_once(':'))
        .is_some_and(|(method, identifier)| !method.is_empty() && !identifier.is_empty())
        && did.len() <= 512
        && did
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || b"-._:".contains(&byte));
    if !valid_did || !lowercase_hex(signer, 32) || signer.bytes().all(|byte| byte == b'0') {
        return Err(
            "px_requestFunds requires a canonical DID and nonzero lowercase signer key".into(),
        );
    }
    Ok(())
}

fn lowercase_hex(value: &str, bytes: usize) -> bool {
    value.len() == bytes * 2
        && value
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
}

/// # Errors
/// Rejects mismatched IDs, malformed envelopes, RPC errors, and non-object results.
pub fn decode_response(method: &str, response: &Value) -> Result<Value, String> {
    if response.get("jsonrpc") != Some(&json!("2.0"))
        || response.get("id") != Some(&json!(1))
        || response.get("result").is_some() == response.get("error").is_some()
    {
        return Err(format!("invalid JSON-RPC response for {method}"));
    }
    if let Some(error) = response.get("error") {
        let code = error
            .get("code")
            .and_then(Value::as_i64)
            .ok_or("malformed RPC error code")?;
        let message = error
            .get("message")
            .and_then(Value::as_str)
            .ok_or("malformed RPC error message")?;
        if code == -32601 {
            return Err(json!({
                "code": code,
                "message": format!("Method not found: {method}: {message}"),
                "data": error.get("data"),
                "method": method,
            })
            .to_string());
        }
        return Err(json!({"code":code,"message":message,"data":error.get("data")}).to_string());
    }
    if method.starts_with("lx_") {
        return Err(format!("rpc_method_unavailable: {method} is switched off"));
    }
    if matches!(
        method,
        "eth_estimateGas" | "eth_gasPrice" | "eth_getTransactionCount"
    ) {
        return response
            .get("result")
            .filter(|value| value.as_str().is_some_and(ethereum_quantity))
            .cloned()
            .ok_or_else(|| format!("{method} returned an invalid Ethereum quantity"));
    }
    if method == "eth_sendRawTransaction" {
        return response
            .get("result")
            .filter(|value| ethereum_data(value, Some(32)))
            .cloned()
            .ok_or_else(|| {
                "eth_sendRawTransaction returned an invalid transaction hash".to_owned()
            });
    }
    if method == "pax_sequencerReceipt" {
        return response
            .get("result")
            .filter(|value| {
                value
                    .get("txHash")
                    .is_some_and(|hash| ethereum_data(hash, Some(32)))
                    && value
                        .get("rung")
                        .and_then(Value::as_str)
                        .is_some_and(|rung| matches!(rung, "submitted" | "receipted" | "committed"))
            })
            .cloned()
            .ok_or_else(|| {
                "pax_sequencerReceipt returned an invalid transaction status".to_owned()
            });
    }
    if method == "eth_subscribe" {
        return response
            .get("result")
            .filter(|value| {
                value
                    .as_str()
                    .is_some_and(|id| !id.is_empty() && id.len() <= 128)
            })
            .cloned()
            .ok_or_else(|| "invalid subscription identifier".to_owned());
    }
    if method == "eth_unsubscribe" {
        return response
            .get("result")
            .filter(|value| value.as_bool() == Some(true))
            .cloned()
            .ok_or_else(|| "invalid unsubscribe acknowledgement".to_owned());
    }
    response
        .get("result")
        .filter(|value| value.is_object())
        .cloned()
        .ok_or_else(|| format!("{method} returned a non-object result"))
}

fn logs_subscription_params(args: &[Value]) -> Result<(), String> {
    let [Value::String(kind), Value::Object(filter)] = args else {
        return Err("eth_subscribe requires logs and an address/topics filter".into());
    };
    let hex = |value: &Value, bytes: usize| {
        value.as_str().is_some_and(|value| {
            value.strip_prefix("0x").is_some_and(|value| {
                value.len() == bytes * 2 && value.bytes().all(|byte| byte.is_ascii_hexdigit())
            })
        })
    };
    let address = filter.get("address").is_some_and(|address| {
        hex(address, 20)
            || address.as_array().is_some_and(|addresses| {
                !addresses.is_empty() && addresses.iter().all(|address| hex(address, 20))
            })
    });
    let topics = filter.get("topics").is_none_or(|topics| {
        topics.as_array().is_some_and(|topics| {
            topics.len() <= 4
                && topics.iter().all(|topic| {
                    topic.is_null()
                        || hex(topic, 32)
                        || topic.as_array().is_some_and(|alternatives| {
                            !alternatives.is_empty()
                                && alternatives.iter().all(|topic| hex(topic, 32))
                        })
                })
        })
    });
    if kind != "logs"
        || filter
            .keys()
            .any(|key| !matches!(key.as_str(), "address" | "topics"))
        || !address
        || !topics
    {
        return Err("invalid eth_subscribe logs address/topics filter".into());
    }
    Ok(())
}

#[cfg(test)]
mod tests {
    use layerx_sdk::rpc::{
        list_assets_params, AssetListSnapshot, LIST_ASSETS_DEFAULT_PAGE, LIST_ASSETS_MAX_PAGE,
    };

    use super::*;

    fn asset_value(asset_id: &str) -> Value {
        json!({
            "asset_id": asset_id,
            "symbol": "LXT",
            "name": "LayerX Test",
            "decimals": 6,
            "custody_kind": 0,
            "custody_reference": "",
            "paused": false,
            "supply_cap": "1000000",
            "issuer_did": "22".repeat(32),
            "issuer_kind": 1,
            "total_units": "100",
            "salt": "33".repeat(32)
        })
    }

    fn asset_page(assets: Vec<Value>, next_cursor: Value) -> Value {
        json!({
            "assets": assets,
            "next_cursor": next_cursor,
            "observed_head_sequence": "9",
            "state_root": "44".repeat(32),
            "verification": "authenticated_committed_snapshot"
        })
    }

    fn assert_published_methods(methods: &[Value]) -> Result<(), String> {
        assert_eq!(
            methods
                .iter()
                .map(|entry| entry["name"].as_str())
                .collect::<Vec<_>>(),
            [
                Some("px_register"),
                Some("px_getCommit"),
                Some("px_getValidators"),
                Some("px_getReceiptKeys"),
                Some("px_getSequenceBatch"),
                Some("px_requestFunds"),
                Some("pax_sequencerReceipt"),
                Some("px_resolveAccount"),
                Some("px_getAccount"),
                Some("px_getBalances"),
                Some("px_listAssets"),
                Some("px_getNetwork"),
                Some("px_getHistory"),
                Some("px_getUnifiedHistory"),
                Some("px_listHolderRecords"),
                Some("px_getCapabilities"),
                Some("px_getRouteCatalogue"),
            ]
        );
        assert!(methods.iter().all(|entry| !entry["name"]
            .as_str()
            .unwrap_or_default()
            .starts_with("lx_")));
        Ok(())
    }

    #[test]
    fn positional_requests_match_published_contract() -> Result<(), String> {
        let fixture = include_str!("../tests/fixtures/openrpc.json");
        let published: Value = serde_json::from_str(fixture).map_err(|error| error.to_string())?;
        let methods = published["methods"]
            .as_array()
            .ok_or("missing contract methods")?;
        assert_published_methods(methods)?;
        let id = "ab".repeat(32);
        for (method, params) in [
            ("lx_getAsset", json!([id])),
            ("lx_getAccount", json!([id])),
            ("lx_getBalance", json!([id])),
            ("lx_getSequence", json!([id])),
            ("lx_getSequence", json!(["did:layerx:alice", "identity"])),
            ("lx_getReceipt", json!([id])),
            ("lx_getActivityStatus", json!([id])),
            ("lx_getCheckpoint", json!([id])),
            ("lx_getBalances", json!(["did:layerx:alice"])),
            ("lx_getNodeInfo", json!([])),
            ("lx_listAssets", json!([null, LIST_ASSETS_MAX_PAGE])),
            ("lx_estimateFee", json!(["abcd"])),
            ("lx_subscribe", json!(["receipts", "0"])),
            ("lx_subscribe", json!(["checkpoints", "17"])),
            ("lx_subscribe", json!(["account", id, "3"])),
            ("lx_unsubscribe", json!(["1"])),
            ("lx_getBatchHeader", json!(["12"])),
            ("lx_getProof", json!(["account", id, id])),
            ("lx_sendActivity", json!(["abcd", "executed"])),
            ("lx_sendActivity", json!(["abcd", "batched"])),
            ("lx_sendActivity", json!(["abcd", "finalised"])),
        ] {
            assert!(!methods.iter().any(|entry| entry["name"] == method));
            for params in [params, json!([]), json!({"account_id":id})] {
                let error = request(method, &params).expect_err("retired method was admitted");
                assert_eq!(
                    error,
                    format!("rpc_method_unavailable: {method} is switched off")
                );
            }
        }
        for (method, args) in [
            ("lx_getSequence", json!(["did:layerx:alice"])),
            ("lx_getBatchHeader", json!(["01"])),
            ("lx_getBalances", json!(["../alice"])),
            ("lx_getProof", json!(["account", id])),
            ("lx_sendActivity", json!(["abc", "executed"])),
            ("lx_sendActivity", json!(["abcd", "ack"])),
            ("lx_estimateFee", json!([])),
            ("lx_estimateFee", json!(["abc"])),
            ("lx_subscribe", json!(["account"])),
            ("lx_subscribe", json!(["receipts", id])),
            ("lx_subscribe", json!(["unknown"])),
            ("lx_subscribe", json!(["receipts", "01"])),
            ("lx_subscribe", json!(["account", id, "-1"])),
            ("lx_subscribe", json!(["account", id, id])),
            ("lx_unsubscribe", json!([])),
            ("lx_unsubscribe", json!(["01"])),
            ("lx_unsubscribe", json!([1])),
            ("lx_unsubscribe", json!(["1", "2"])),
            ("lx_getAsset", json!([])),
            ("lx_listAssets", json!([id, 0])),
            (
                "lx_listAssets",
                json!([id, u64::from(LIST_ASSETS_MAX_PAGE) + 1]),
            ),
            ("lx_listAssets", json!([id, "64"])),
            ("lx_listAssets", json!([id, -1])),
            ("lx_listAssets", json!([id, LIST_ASSETS_DEFAULT_PAGE, id])),
            ("lx_listAssets", json!(["00".repeat(32)])),
            ("lx_listAssets", json!(["ab"])),
            ("lx_listAssets", json!([1])),
            ("lx_listAssets", json!([{"cursor":id}])),
        ] {
            assert_eq!(
                request(method, &args).err(),
                Some(format!("rpc_method_unavailable: {method} is switched off"))
            );
        }
        Ok(())
    }

    #[test]
    fn program_event_queries_are_bounded_before_transport() -> Result<(), String> {
        let topic = "504158454552585f5745425f524551554553545f5631";
        for params in [
            json!([{"topic": topic, "from_sequence": 0, "limit": 1}]),
            json!([{"limit": PROGRAM_EVENTS_MAX_PAGE, "from_sequence": u64::MAX, "topic": "ab".repeat(64)}]),
            json!([{"topic": "00", "from_sequence": 40, "limit": 2}]),
        ] {
            assert_eq!(
                request("lx_getProgramEvents", &params).err(),
                Some("rpc_method_unavailable: lx_getProgramEvents is switched off".into())
            );
        }
        let client = RpcClient::new("http://127.0.0.1:1/rpc", None)?;
        for params in [
            json!([]),
            json!({}),
            json!({"topic": topic, "from_sequence": 0, "limit": 1}),
            json!([{"topic": topic, "from_sequence": 0}]),
            json!([{"topic": topic, "limit": 1}]),
            json!([{"from_sequence": 0, "limit": 1}]),
            json!([{"topic": topic, "from_sequence": 0, "limit": 0}]),
            json!([{"topic": topic, "from_sequence": 0, "limit": PROGRAM_EVENTS_MAX_PAGE + 1}]),
            json!([{"topic": topic, "from_sequence": -1, "limit": 1}]),
            json!([{"topic": topic, "from_sequence": 1.5, "limit": 1}]),
            json!([{"topic": topic, "from_sequence": "1", "limit": 1}]),
            json!([{"topic": topic, "from_sequence": 0, "limit": "1"}]),
            json!([{"topic": "", "from_sequence": 0, "limit": 1}]),
            json!([{"topic": "ABCD", "from_sequence": 0, "limit": 1}]),
            json!([{"topic": "abc", "from_sequence": 0, "limit": 1}]),
            json!([{"topic": "+a", "from_sequence": 0, "limit": 1}]),
            json!([{"topic": "../", "from_sequence": 0, "limit": 1}]),
            json!([{"topic": 171, "from_sequence": 0, "limit": 1}]),
            json!([{"topic": "ab".repeat(65), "from_sequence": 0, "limit": 1}]),
            json!([{"topic": topic, "from_sequence": 0, "limit": 1, "extra": 1}]),
            json!([{"topic": topic, "from_sequence": 0, "limit": 1}, 1]),
            json!([topic, 0, 1]),
        ] {
            assert!(request("lx_getProgramEvents", &params).is_err(), "{params}");
            let refused = client
                .call("lx_getProgramEvents", &params)
                .err()
                .ok_or("malformed query reached transport")?;
            assert!(
                refused == "rpc_method_unavailable: lx_getProgramEvents is switched off",
                "{refused}"
            );
        }
        Ok(())
    }

    #[test]
    fn program_event_pages_decode_only_when_they_match_the_query() -> Result<(), String> {
        let topic = "504158454552585f5745425f524551554553545f5631";
        let query = json!([{"topic": topic, "from_sequence": 40, "limit": 2}]);
        let event = |sequence: u64, topic: &str| json!({"sequence": sequence, "program_id": "ab".repeat(32), "topic": topic, "data": "01"});
        let accepted = [
            json!({"events": [event(40, topic), event(41, topic)], "next_sequence": 42}),
            json!({"events": [event(41, topic)], "next_sequence": 44}),
            json!({"events": [], "next_sequence": 40}),
            json!({"events": [{"sequence": 43, "program_id": "ab".repeat(32), "topic": topic,
                "data": "ab".repeat(PROGRAM_EVENT_MAX_DATA_BYTES)}], "next_sequence": 44}),
        ];
        for page in &accepted {
            let response = json!({"jsonrpc":"2.0","id":1,"result":page});
            assert_eq!(
                decode_response("lx_getProgramEvents", &response).err(),
                Some("rpc_method_unavailable: lx_getProgramEvents is switched off".into())
            );
            assert!(request("lx_getProgramEvents", &query).is_err());
        }
        for refused in [
            json!({"events": [event(40, topic), event(41, topic), event(42, topic)],
                "next_sequence": 43}),
            json!({"events": [], "next_sequence": 39}),
            json!({"events": [event(44, topic)], "next_sequence": 44}),
            json!({"events": [event(39, topic)], "next_sequence": 44}),
            json!({"events": [event(42, topic), event(41, topic)], "next_sequence": 44}),
            json!({"events": [event(41, topic), event(41, topic)], "next_sequence": 44}),
            json!({"events": [event(41, "00")], "next_sequence": 44}),
            json!({"events": [{"sequence": 41, "program_id": "00".repeat(32), "topic": topic,
                "data": "01"}], "next_sequence": 44}),
            json!({"events": [{"sequence": 41, "program_id": "AB".repeat(32), "topic": topic,
                "data": "01"}], "next_sequence": 44}),
            json!({"events": [{"sequence": 41, "program_id": "ab".repeat(32), "topic": topic,
                "data": "0"}], "next_sequence": 44}),
            json!({"events": [{"sequence": 41, "program_id": "ab".repeat(32), "topic": topic,
                "data": "ab".repeat(PROGRAM_EVENT_MAX_DATA_BYTES + 1)}], "next_sequence": 44}),
            json!({"events": [{"sequence": 41, "program_id": "ab".repeat(32), "topic": topic}],
                "next_sequence": 44}),
            json!({"events": [{"sequence": 41, "program_id": "ab".repeat(32), "topic": topic,
                "data": "01", "extra": true}], "next_sequence": 44}),
            json!({"events": [], "next_sequence": 44, "extra": true}),
            json!({"events": {}, "next_sequence": 44}),
            json!({"events": []}),
            json!({"events": [], "next_sequence": "44"}),
            json!([]),
        ] {
            assert_eq!(
                decode_response(
                    "lx_getProgramEvents",
                    &json!({"jsonrpc":"2.0","id":1,"result":refused})
                )
                .err(),
                Some("rpc_method_unavailable: lx_getProgramEvents is switched off".into())
            );
        }
        assert!(request("lx_getProgramEvents", &json!([{"topic": topic}])).is_err());
        assert!(decode_response(
            "lx_getProgramEvents",
            &json!({"jsonrpc":"2.0","id":1,"result":[]})
        )
        .is_err());
        let unavailable = json!({"code":-32001,"message":"Read unavailable"});
        assert_eq!(
            serde_json::from_str::<Value>(
                &decode_response(
                    "lx_getProgramEvents",
                    &json!({"jsonrpc":"2.0","id":1,"error":unavailable})
                )
                .err()
                .ok_or("error lost")?
            )
            .map_err(|e| e.to_string())?,
            json!({"code":-32001,"message":"Read unavailable","data":null})
        );
        Ok(())
    }

    #[test]
    fn asset_pages_send_and_parse_the_published_cursor_contract() -> Result<(), String> {
        let cursor = [0xab; 32];
        let encoded = "ab".repeat(32);
        assert_eq!(list_assets_params(None, None), json!([]));
        assert_eq!(list_assets_params(Some(cursor), None), json!([encoded]));
        assert_eq!(
            list_assets_params(Some(cursor), Some(LIST_ASSETS_MAX_PAGE)),
            json!([encoded, LIST_ASSETS_MAX_PAGE])
        );
        assert_eq!(
            list_assets_params(None, Some(LIST_ASSETS_DEFAULT_PAGE)),
            json!([Value::Null, LIST_ASSETS_DEFAULT_PAGE])
        );
        for params in [
            list_assets_params(None, None),
            list_assets_params(Some(cursor), None),
            list_assets_params(Some(cursor), Some(1)),
            list_assets_params(Some(cursor), Some(LIST_ASSETS_MAX_PAGE)),
            list_assets_params(None, Some(LIST_ASSETS_DEFAULT_PAGE)),
        ] {
            assert_eq!(
                request("lx_listAssets", &params).err(),
                Some("rpc_method_unavailable: lx_listAssets is switched off".into())
            );
        }
        let first = "11".repeat(32);
        let second = "22".repeat(32);
        let truncated = asset_page(
            vec![asset_value(&first), asset_value(&second)],
            json!(second),
        );
        let page =
            AssetListSnapshot::try_from(truncated.clone()).map_err(|error| format!("{error:?}"))?;
        assert_eq!(
            page.assets
                .iter()
                .map(|asset| asset.asset_id)
                .collect::<Vec<_>>(),
            [[0x11; 32], [0x22; 32]]
        );
        assert_eq!(page.next_cursor, Some([0x22; 32]));
        assert_eq!(page.into_value(), truncated);
        let last = AssetListSnapshot::try_from(asset_page(vec![asset_value(&first)], Value::Null))
            .map_err(|error| format!("{error:?}"))?;
        assert_eq!(last.next_cursor, None);
        assert_eq!(last.assets.len(), 1);
        for refused in [
            asset_page(
                vec![asset_value(&first), asset_value(&second)],
                json!(first),
            ),
            asset_page(vec![asset_value(&second), asset_value(&first)], Value::Null),
            asset_page(Vec::new(), json!(first)),
            asset_page(vec![asset_value(&first)], json!("2".repeat(63))),
            json!({
                "assets": [asset_value(&first)],
                "observed_head_sequence": "9",
                "state_root": "44".repeat(32),
                "verification": "authenticated_committed_snapshot"
            }),
        ] {
            assert!(AssetListSnapshot::try_from(refused).is_err());
        }
        Ok(())
    }

    #[test]
    fn subscriptions_require_authenticated_websocket_and_string_ack() -> Result<(), String> {
        let client = RpcClient::new("http://127.0.0.1:1/rpc", None)?;
        assert!(client.call("eth_subscribe", &json!(["receipts"])).is_err());
        assert!(client.call("eth_unsubscribe", &json!(["1"])).is_err());
        assert!(client
            .subscribe(&json!(["receipts"]), std::time::Duration::from_secs(1))
            .is_err());
        assert!(RpcClient::new("http://example.com/rpc", None).is_err());
        let response = json!({"jsonrpc":"2.0","id":1,"result":"1"});
        assert_eq!(decode_response("eth_subscribe", &response)?, "1");
        assert!(decode_response("px_getAccount", &response).is_err());
        for result in [json!(null), json!({}), json!(""), json!(1)] {
            assert!(decode_response(
                "eth_subscribe",
                &json!({"jsonrpc":"2.0","id":1,"result":result})
            )
            .is_err());
        }
        let cancelled = json!({"jsonrpc":"2.0","id":1,"result":true});
        assert_eq!(decode_response("eth_unsubscribe", &cancelled)?, true);
        for result in [json!(false), json!("true"), json!({}), json!(null)] {
            assert!(decode_response(
                "eth_unsubscribe",
                &json!({"jsonrpc":"2.0","id":1,"result":result})
            )
            .is_err());
        }
        let error = json!({"code":-32005,"message":"feed unavailable","data":{"reason":"native_feed_unavailable"}});
        assert_eq!(
            serde_json::from_str::<Value>(
                &decode_response(
                    "eth_subscribe",
                    &json!({"jsonrpc":"2.0","id":1,"error":error})
                )
                .err()
                .ok_or("error lost")?
            )
            .map_err(|e| e.to_string())?,
            error
        );
        Ok(())
    }

    #[test]
    fn remote_enumeration_error_is_preserved() -> Result<(), String> {
        let fields = json!({"code":-32005,"message":"DID enumeration unavailable","data":{"reason":"native_index_unavailable"}});
        let response = json!({"jsonrpc":"2.0","id":1,"error":fields});
        let error = decode_response("px_getBalances", &response)
            .err()
            .ok_or("RPC error was hidden")?;
        assert_eq!(
            serde_json::from_str::<Value>(&error).map_err(|e| e.to_string())?,
            fields
        );
        Ok(())
    }

    #[test]
    fn response_validation_refuses_errors_and_unbound_results() -> Result<(), String> {
        let good = json!({"jsonrpc":"2.0","id":1,"result":{"state":"pending"}});
        assert_eq!(decode_response("px_register", &good)?["state"], "pending");
        for bad in [
            json!({"jsonrpc":"2.0","id":2,"result":{}}),
            json!({"jsonrpc":"2.0","id":1,"result":null}),
            json!({"jsonrpc":"2.0","id":1,"result":{},"error":{}}),
            json!({"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found"}}),
            json!({"id":1,"result":{}}),
        ] {
            assert!(decode_response("px_getBalances", &bad).is_err());
        }
        Ok(())
    }
}
