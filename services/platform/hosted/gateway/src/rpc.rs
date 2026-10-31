use super::{
    json_response, media_type_is, response, Config, IncomingRequest, KernelBackend,
    OutgoingResponse,
};
use serde_json::{json, Value};

pub(super) fn error(id: &Value, code: i32, message: &str) -> Value {
    json!({"jsonrpc":"2.0", "id":id, "error":{"code":code,"message":message}})
}

pub(super) fn invalid_request(value: &Value) -> Option<Value> {
    let id = value.get("id").unwrap_or(&Value::Null);
    if !value.is_object()
        || value.get("jsonrpc") != Some(&json!("2.0"))
        || !value.get("method").is_some_and(Value::is_string)
        || !(id.is_null() || id.is_string() || id.is_number())
    {
        Some(error(&Value::Null, -32600, "Invalid Request"))
    } else {
        None
    }
}

fn kernel_backend(method: &str) -> Option<KernelBackend> {
    (method == crate::rpc_faucet::px_requestFunds).then_some(KernelBackend::Identity)
}

pub(super) fn dispatch(config: &Config, request: &IncomingRequest, value: &Value) -> Option<Value> {
    if let Some(refusal) = invalid_request(value) {
        return Some(refusal);
    }
    let id = value.get("id").cloned().unwrap_or(Value::Null);
    let method = value["method"].as_str()?;
    if method.starts_with("lx_") {
        return value
            .get("id")
            .map(|_| error(&id, -32601, "Method not found"));
    }
    if method == "px_getRouteCatalogue" {
        let result = crate::routes::rpc_catalogue(config, request, &id, value.get("params"));
        return value.get("id").map(|_| result);
    }
    if let Some(unavailable) =
        kernel_backend(method).and_then(|backend| config.backend(backend).err())
    {
        return value.get("id").map(|_| unavailable.rpc(&id));
    }
    if let Some(result) =
        crate::rpc_register::dispatch(config, request, method, &id, value.get("params"))
    {
        return value.get("id").map(|_| result);
    }
    if let Some(result) =
        crate::rpc_faucet::dispatch(config, request, method, &id, value.get("params"))
    {
        return value.get("id").map(|_| result);
    }
    if let Some(result) = crate::history::dispatch(config, method, &id, value.get("params")) {
        return value.get("id").map(|_| result);
    }
    if let Some(result) = crate::chain_reads::dispatch(config, method, &id, value.get("params")) {
        return value.get("id").map(|_| result);
    }
    if let Some(result) = crate::paxeer::dispatch(config, method, &id, value.get("params")) {
        return value.get("id").map(|_| result);
    }
    value
        .get("id")
        .map(|_| error(&id, -32601, "Method not found"))
}

pub(super) fn route(config: &Config, request: &IncomingRequest) -> OutgoingResponse {
    if request.path == "/rpc/schema" {
        return if request.method == "GET" {
            OutgoingResponse {
                content_type: "application/json".to_owned(),
                headers: Vec::new(),
                status: 200,
                body: include_bytes!("../openrpc.json").to_vec(),
                retry_after: None,
            }
        } else {
            response(405, "method_not_allowed", None)
        };
    }
    if request.method != "POST" {
        return response(405, "method_not_allowed", None);
    }
    if !media_type_is(request, "application/json") {
        return response(415, "json_content_type_required", None);
    }
    let Ok(value) = serde_json::from_slice::<Value>(&request.body) else {
        return json_response(200, &error(&Value::Null, -32700, "Parse error"));
    };
    let result = if let Value::Array(batch) = &value {
        if batch.is_empty() || batch.len() > 32 {
            Some(error(&Value::Null, -32600, "Invalid Request"))
        } else {
            let results: Vec<_> = batch
                .iter()
                .filter_map(|entry| dispatch(config, request, entry))
                .collect();
            if results.is_empty() {
                None
            } else {
                Some(Value::Array(results))
            }
        }
    } else {
        dispatch(config, request, &value)
    };
    result.map_or_else(
        || OutgoingResponse {
            content_type: "application/json".to_owned(),
            headers: Vec::new(),
            status: 204,
            body: Vec::new(),
            retry_after: None,
        },
        |value| json_response(200, &value),
    )
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn lx_names_answer_method_not_found() {
        use std::collections::BTreeMap;
        use std::net::TcpListener;
        use std::sync::Arc;
        use std::time::Duration;
        use zeroize::Zeroizing;

        let backend = TcpListener::bind("127.0.0.1:0").expect("backend listener");
        backend.set_nonblocking(true).expect("nonblocking backend");
        let address = backend.local_addr().expect("backend address");
        let endpoint = crate::Endpoint::plain(address.ip().to_string(), address.port());
        let ca = native_tls::Certificate::from_pem(include_bytes!(
            "../../../../../consensus/rpc/jsonrpc/server/test.crt"
        ))
        .expect("test certificate");
        let mut config = Config {
            wallet_caps: None,
            routes: crate::routes::Registry::configured("layerx-testnet", "1").expect("routes"),
            listen: "127.0.0.1:0".parse().expect("listen address"),
            listener: crate::Listener::Plain,
            client: crate::Client::without_identity(ca.clone()),
            client_identity: false,
            kernel: None,
            receipt_keys: None,
            paxeer: None,
            capabilities: crate::capabilities::Cache::new(Duration::from_secs(1)),
            indexer: None,
            registration_token: None,
            trusted_proxies: Vec::new(),
            faucet: None,
            store: Arc::new(crate::RedisStore::new(
                crate::RedisEndpoint::parse(&format!("rediss://localhost:{}", address.port()))
                    .expect("Redis endpoint"),
                ca,
                Zeroizing::new("gateway".to_owned()),
                Zeroizing::new(String::new()),
            )),
            event_producer: false,
            network_id: "layerx-testnet".to_owned(),
            wire_version: "1".to_owned(),
            protocol_version: 1,
            protocol_network_id: 1,
            idempotency_seconds: 3600,
        };
        let request = IncomingRequest {
            method: "POST".into(),
            path: "/rpc".into(),
            headers: BTreeMap::from([("content-type".into(), "application/json".into())]),
            body: Vec::new(),
        };
        let names = [
            "lx_",
            "lx_unknown",
            "lx_register",
            "lx_getHistory",
            "lx_requestFunds",
            "lx_getAccount",
            "lx_getBalance",
            "lx_getBalances",
            "lx_getSequence",
            "lx_estimateFee",
            "lx_getProgramEvents",
            "lx_sendActivity",
            "lx_getReceipt",
            "lx_getActivityStatus",
            "lx_getBatchHeader",
            "lx_getCheckpoint",
            "lx_getProof",
            "lx_listAssets",
            "lx_getAsset",
            "lx_getNodeInfo",
            "lx_subscribe",
            "lx_unsubscribe",
            "lx_subscription",
            "lx_getWalletCaps",
        ];
        for configured in [false, true] {
            if configured {
                config.kernel = Some(crate::Kernel {
                    component: endpoint.clone(),
                    component_token: Zeroizing::new("token".into()),
                    public_core: Some(endpoint.clone()),
                    authority: endpoint.clone(),
                    authority_token: Zeroizing::new("token".into()),
                    identity: endpoint.clone(),
                    identity_token: Zeroizing::new("token".into()),
                    registry: endpoint.clone(),
                    registry_token: Zeroizing::new("token".into()),
                    key_provisioning_key: Zeroizing::new([0; 32]),
                    modules: layerx_types::payload::ModuleRegistry::new(&[]).expect("modules"),
                });
                config.paxeer = Some(vec![endpoint.clone()]);
                config.indexer = Some(
                    crate::history::Indexer::parse(&format!("http://{address}")).expect("indexer"),
                );
                config.registration_token = Some(Zeroizing::new("token".into()));
            }
            for backend in KernelBackend::ALL {
                assert_eq!(config.backend(backend).is_ok(), configured, "{backend:?}");
            }
            for method in names {
                assert_eq!(kernel_backend(method), None, "{method}");
                for params in [
                    Value::Null,
                    json!([]),
                    json!(["ab".repeat(32)]),
                    json!({"invalid": true}),
                ] {
                    let mut call = json!({"jsonrpc": "2.0", "id": "lx-refusal", "method": method, "params": params});
                    assert_eq!(
                        dispatch(&config, &request, &call),
                        Some(error(&call["id"], -32601, "Method not found")),
                        "{method}, configured={configured}"
                    );
                    call["id"] = Value::Null;
                    assert_eq!(
                        dispatch(&config, &request, &call),
                        Some(error(&Value::Null, -32601, "Method not found"))
                    );
                    call.as_object_mut().expect("call").remove("id");
                    assert_eq!(
                        dispatch(&config, &request, &call),
                        None,
                        "{method} notification"
                    );
                }
            }
            assert_eq!(
                kernel_backend(crate::rpc_faucet::px_requestFunds),
                Some(KernelBackend::Identity)
            );
            let funds =
                json!({"jsonrpc": "2.0", "id": 1, "method": "px_requestFunds", "params": []});
            if !configured {
                assert_eq!(
                    dispatch(&config, &request, &funds).expect("faucet refusal")["error"]["code"],
                    -32010
                );
            }
            match backend.accept() {
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
                _ => panic!("retired lx_ method contacted a backend"),
            }
        }
    }

    #[test]
    fn schema_lists_every_public_method() {
        let schema: Value = serde_json::from_slice(include_bytes!("../openrpc.json"))
            .unwrap_or_else(|e| panic!("{e}"));
        let methods = schema["methods"]
            .as_array()
            .unwrap_or_else(|| panic!("methods missing"));
        let published = [
            "px_register",
            "px_getCommit",
            "px_getValidators",
            "px_getReceiptKeys",
            "px_getSequenceBatch",
            "px_requestFunds",
            "pax_sequencerReceipt",
            "px_resolveAccount",
            "px_getAccount",
            "px_getBalances",
            "px_listAssets",
            "px_getNetwork",
            "px_getHistory",
            "px_getUnifiedHistory",
            "px_listHolderRecords",
            "px_getCapabilities",
            "px_getRouteCatalogue",
        ];
        for name in published {
            assert_eq!(
                methods
                    .iter()
                    .filter(|method| method["name"] == name)
                    .count(),
                1,
                "{name}"
            );
        }
        assert_eq!(methods.len(), published.len());
    }

    #[test]
    fn invalid_envelopes_have_json_rpc_errors_and_null_ids() {
        for value in [
            json!(null),
            json!(4),
            json!([]),
            json!({}),
            json!({"jsonrpc":"1.0","method":"lx_getNodeInfo","id":7}),
            json!({"jsonrpc":"2.0","method":7,"id":7}),
            json!({"jsonrpc":"2.0","method":"lx_getNodeInfo","id":true}),
        ] {
            assert_eq!(
                invalid_request(&value),
                Some(
                    json!({"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"Invalid Request"}})
                )
            );
        }
        for value in [
            json!({"jsonrpc":"2.0","method":"lx_getNodeInfo"}),
            json!({"jsonrpc":"2.0","method":"lx_getNodeInfo","id":null}),
            json!({"jsonrpc":"2.0","method":"lx_getNodeInfo","id":"abc"}),
            json!({"jsonrpc":"2.0","method":"lx_getNodeInfo","id":1}),
        ] {
            assert_eq!(invalid_request(&value), None);
        }
    }
}
