#[path = "support/gateway_22_33_1546.rs"]
mod producer_repair;

use std::fmt::Write as _;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::px::{PxAssetEntry, PxAssetTable, PxEndpoint, Uint256};
use layerx_client::rpc::payments::{
    estimate_fee, get_asset, list_assets, CallRequest, FeeEstimate,
};
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, Eip1559Call};
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::from("0x"), |mut text, byte| {
        must(write!(text, "{byte:02x}"));
        text
    })
}

fn serve(exchanges: Vec<(&'static str, Value)>) -> (SequencerEndpoint, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    let server = std::thread::spawn(move || {
        let mut requests = Vec::new();
        for (method, mut response) in exchanges {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            must(stream.set_write_timeout(Some(Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let mut body = vec![0; length.unwrap_or_else(|| panic!("missing content length"))];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            response["jsonrpc"] = json!("2.0");
            response["id"] = request["id"].clone();
            requests.push(request);
            let answer = response.to_string();
            let mut stream = reader.into_inner();
            must(write!(stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                answer.len()
            ));
            must(stream.flush());
        }
        requests
    });
    (must(SequencerEndpoint::new(&url, &[])), server)
}

#[test]
fn list_assets_and_get_asset_read_px_list_assets() {
    producer_repair::assets();
    let result = json!({"assets": [
        {"asset_id": "bank:unavailable", "kind": "bank", "denom": "unavailable",
         "name": null, "symbol": null, "decimals": null, "supply": null},
        {"asset_id": "bank:upax", "kind": "bank", "denom": "upax",
         "name": "Paxeer", "symbol": "PAX", "decimals": 18, "supply": "123456789",
         "paused": false, "verification": "authenticated_committed_snapshot"}
    ], "height": 150, "joined_limit": 16});
    let empty = json!({"assets": [], "height": 151, "joined_limit": 16});
    let mut malformed = result.clone();
    malformed["assets"][1]["asset_id"] = json!("");
    let refusal = RpcRefusal {
        code: -32004,
        message: "assets unavailable".to_owned(),
        data: Some(json!({"reason": "unavailable"})),
    };
    let (endpoint, server) = serve(vec![
        ("px_listAssets", json!({"result": result})),
        ("px_listAssets", json!({"result": result})),
        ("px_listAssets", json!({"result": result})),
        ("px_listAssets", json!({"result": result})),
        ("px_listAssets", json!({"result": empty})),
        ("px_listAssets", json!({"result": empty})),
        ("px_listAssets", json!({"result": malformed})),
        ("px_listAssets", json!({"result": malformed})),
        (
            "px_listAssets",
            json!({"error": {
                "code": refusal.code, "message": refusal.message, "data": refusal.data
            }}),
        ),
    ]);
    let endpoint = PxEndpoint::from(endpoint);
    let native = PxAssetEntry {
        asset_id: "bank:unavailable".to_owned(),
        kind: "bank".to_owned(),
        denom: Some("unavailable".to_owned()),
        name: None,
        symbol: None,
        decimals: None,
        supply: None,
        paused: None,
        verification: None,
    };
    let custody = PxAssetEntry {
        asset_id: "bank:upax".to_owned(),
        kind: "bank".to_owned(),
        denom: Some("upax".to_owned()),
        name: Some("Paxeer".to_owned()),
        symbol: Some("PAX".to_owned()),
        decimals: Some(18),
        supply: Some(Uint256::from_decimal("123456789").expect("exact supply")),
        paused: Some(false),
        verification: Some("authenticated_committed_snapshot".to_owned()),
    };
    assert_eq!(
        must(list_assets(&endpoint)),
        PxAssetTable {
            assets: vec![native.clone(), custody.clone()],
            height: 150,
            joined_limit: 16
        }
    );
    assert_eq!(must(get_asset(&endpoint, "bank:upax")), custody);
    assert_eq!(must(get_asset(&endpoint, "bank:unavailable")), native);
    assert_eq!(
        get_asset(&endpoint, "bank:missing"),
        Err(SequencerError::Unavailable)
    );
    assert_eq!(
        must(list_assets(&endpoint)),
        PxAssetTable {
            assets: vec![],
            height: 151,
            joined_limit: 16
        }
    );
    assert_eq!(
        get_asset(&endpoint, "bank:upax"),
        Err(SequencerError::Unavailable)
    );
    assert_eq!(list_assets(&endpoint), Err(SequencerError::Malformed));
    assert_eq!(
        get_asset(&endpoint, "bank:upax"),
        Err(SequencerError::Malformed)
    );
    assert_eq!(list_assets(&endpoint), Err(SequencerError::Rpc(refusal)));
    let requests = must(server.join());
    assert_eq!(requests.len(), 9);
    for (index, request) in requests.iter().enumerate() {
        assert_eq!(request["params"], json!([]));
        assert_eq!(request["id"], json!(index + 1));
    }
}

#[test]
fn estimate_fee_uses_eth_estimate_gas_on_the_signed_transaction() {
    let transaction = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1_000_000_000,
        max_fee_per_gas: 3_000_000_000,
        gas_limit: 90_000,
        to: [0x10; 20],
        data: vec![0, 0x12, 0xab, 0xff],
    };
    let raw = must(transaction.sign(&[0x46; 32]));
    let (decoded, signature) = must(decode_signed(&raw));
    assert_eq!(decoded, transaction);
    let sender = must(recover_sender(&decoded, &signature));
    let call = CallRequest {
        from: Some(sender),
        to: Some(decoded.to),
        value: 0,
        data: decoded.data,
    };
    let refusal = RpcRefusal {
        code: 3,
        message: "execution reverted".to_owned(),
        data: Some(json!("0xdeadbeef")),
    };
    let (endpoint, server) = serve(vec![
        ("eth_estimateGas", json!({"result": "0x12345"})),
        ("eth_estimateGas", json!({"result": "0xffffffffffffffff"})),
        ("eth_estimateGas", json!({"result": "0x10000000000000000"})),
        (
            "eth_estimateGas",
            json!({"error": {
                "code": refusal.code, "message": refusal.message, "data": refusal.data
            }}),
        ),
        ("eth_estimateGas", json!({"result": "0x5208"})),
    ]);
    assert_eq!(
        must(estimate_fee(&endpoint, &call)),
        FeeEstimate {
            fee: 0x12345,
            parameter_version: 0,
            canonical_schedule: vec![]
        }
    );
    assert_eq!(
        must(estimate_fee(&endpoint, &call)).fee,
        u128::from(u64::MAX)
    );
    assert_eq!(
        estimate_fee(&endpoint, &call),
        Err(SequencerError::Malformed)
    );
    assert_eq!(
        estimate_fee(&endpoint, &call),
        Err(SequencerError::Rpc(refusal))
    );
    let creation = CallRequest {
        value: u128::MAX,
        data: vec![0x60, 0],
        ..CallRequest::default()
    };
    assert_eq!(must(estimate_fee(&endpoint, &creation)).fee, 21_000);
    let requests = must(server.join());
    assert_eq!(requests.len(), 5);
    for (index, request) in requests.iter().enumerate() {
        assert_eq!(request["id"], json!(index + 1));
        let expected = if index == 4 {
            json!([{"value": "0xffffffffffffffffffffffffffffffff", "data": "0x6000"}, "pending"])
        } else {
            json!([{"from": hex(&sender), "to": hex(&transaction.to), "data": hex(&transaction.data)}, "pending"])
        };
        assert_eq!(request["params"], expected);
    }
}
