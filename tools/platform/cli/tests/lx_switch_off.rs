use std::io::ErrorKind;
use std::net::TcpListener;

use layerx_platform_cli::rpc::{decode_response, request, RpcClient};
use serde_json::{json, Value};
use zeroize::Zeroizing;

const SWITCHED_OFF_NAMES: [&str; 26] = [
    "lx_sendActivity",
    "lx_getSequence",
    "lx_getAccount",
    "lx_getBalance",
    "lx_getBalances",
    "lx_getReceipt",
    "lx_getActivityStatus",
    "lx_getBatchHeader",
    "lx_getCheckpoint",
    "lx_getProof",
    "lx_getAsset",
    "lx_listAssets",
    "lx_estimateFee",
    "lx_getProgramEvents",
    "lx_getNodeInfo",
    "lx_getWalletCaps",
    "lx_getHistory",
    "lx_register",
    "lx_requestFunds",
    "lx_subscribe",
    "lx_unsubscribe",
    "lx_subscription",
    "lx_getArchiveNetwork",
    "lx_getArchiveHead",
    "lx_listActivities",
    "lx_listBatches",
];

#[test]
fn request_refuses_every_switched_off_name() -> Result<(), String> {
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    listener
        .set_nonblocking(true)
        .map_err(|error| error.to_string())?;
    let url = format!(
        "http://{}/rpc",
        listener.local_addr().map_err(|error| error.to_string())?
    );
    let clients = [
        RpcClient::new(&url, None)?,
        RpcClient::new(
            &url,
            Some(Zeroizing::new(format!("key:lxp_live_{}", "a".repeat(64)))),
        )?,
        RpcClient::new_session(&url, Zeroizing::new("identity-session".into()))?,
    ];
    let id = "ab".repeat(32);
    for method in SWITCHED_OFF_NAMES.into_iter().chain(["lx_futureMethod"]) {
        let expected = format!("rpc_method_unavailable: {method} is switched off");
        for params in [
            json!([]),
            json!([id]),
            json!(["did:layerx:alice", id]),
            json!(["receipts"]),
            json!([{"topic":"ab","from_sequence":0,"limit":1}]),
            json!(["abcd", "executed"]),
            json!({"account_id":id}),
            Value::Null,
        ] {
            assert_eq!(
                request(method, &params).err().as_deref(),
                Some(expected.as_str())
            );
            for client in &clients {
                assert_eq!(
                    client.call(method, &params).err().as_deref(),
                    Some(expected.as_str())
                );
                match listener.accept() {
                    Err(error) => assert_eq!(error.kind(), ErrorKind::WouldBlock),
                    Ok(_) => panic!("{method} reached the endpoint"),
                }
            }
        }
    }
    assert_eq!(
        request("eth_gasPrice", &json!([]))?["method"],
        "eth_gasPrice"
    );
    Ok(())
}

#[test]
fn openrpc_fixture_lists_no_lx_method() -> Result<(), String> {
    let fixture: Value = serde_json::from_str(include_str!("fixtures/openrpc.json"))
        .map_err(|error| error.to_string())?;
    let methods = fixture["methods"]
        .as_array()
        .ok_or("missing OpenRPC methods")?;
    assert!(!methods.is_empty());
    for method in methods {
        let name = method["name"].as_str().ok_or("missing method name")?;
        assert!(
            !name.starts_with("lx_"),
            "retired method in OpenRPC: {name}"
        );
    }
    for name in [
        "px_register",
        "px_requestFunds",
        "px_getBalances",
        "px_listAssets",
        "pax_sequencerReceipt",
    ] {
        assert!(
            methods.iter().any(|method| method["name"] == name),
            "missing surviving method {name}"
        );
    }
    Ok(())
}

#[test]
fn server_minus_32601_is_reported_as_switched_off() -> Result<(), String> {
    let response: Value = serde_json::from_str(
        r#"{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"Method not found","data":{"reason":"switched_off"}}}"#,
    ).map_err(|error| error.to_string())?;
    for method in ["px_getBalances", "eth_gasPrice"]
        .into_iter()
        .chain(SWITCHED_OFF_NAMES)
    {
        let error =
            decode_response(method, &response).expect_err("method-not-found answer was accepted");
        let reported: Value = serde_json::from_str(&error).map_err(|error| error.to_string())?;
        assert_eq!(reported["code"], -32601);
        assert_eq!(reported["method"], method);
        assert!(reported["message"]
            .as_str()
            .ok_or("missing error message")?
            .contains("Method not found"));
        assert!(error.contains(method), "missing method name: {error}");
        assert_eq!(reported["data"], response["error"]["data"]);
    }
    for invalid in [
        json!({"jsonrpc":"2.0","id":2,"error":response["error"]}),
        json!({"jsonrpc":"2.0","id":1,"error":response["error"],"result":{}}),
        json!({"jsonrpc":"2.0","id":1,"error":{"code":"-32601","message":"Method not found"}}),
    ] {
        assert!(decode_response("px_getBalances", &invalid).is_err());
    }
    let other = json!({"code":-32005,"message":"rate limited","data":{"retry_after":1}});
    let error = decode_response(
        "px_getBalances",
        &json!({"jsonrpc":"2.0","id":1,"error":other}),
    )
    .expect_err("RPC error was accepted");
    assert_eq!(
        serde_json::from_str::<Value>(&error).map_err(|error| error.to_string())?,
        other
    );
    Ok(())
}
