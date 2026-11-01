use std::fmt::Write as _;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::px::{PxAssetEntry, PxAssetTable, PxCustodyAsset, PxEndpoint};
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
    let metadata = json!({
        "asset_id": "ab".repeat(32), "name": "Paxeer", "symbol": "PAX",
        "decimals": 18, "supply": "123456789", "metadata": {"tags": ["native"]}
    });
    let result = json!({"assets": [
        {"asset_id": "11".repeat(32), "layerx": null, "paxeer": null},
        {"asset_id": "ab".repeat(32), "layerx": metadata, "paxeer": {
            "asset_id": "ab".repeat(32), "denom": "upax",
            "pointer": hex(&[0xcd; 20]), "enabled": true, "paused": false,
            "minimum_deposit": "7", "custody_cap": u128::MAX.to_string(),
            "custodied": "0x10000000000000001", "released": "9", "pending": 10
        }}
    ], "joined_limit": "0x10"});
    let empty = json!({"assets": [], "joined_limit": 16});
    let mut malformed = result.clone();
    malformed["assets"][1]["asset_id"] = json!("ab");
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
        asset_id: [0x11; 32],
        layerx: None,
        paxeer: None,
    };
    let custody = PxAssetEntry {
        asset_id: [0xab; 32],
        layerx: Some(
            metadata
                .as_object()
                .unwrap_or_else(|| panic!("metadata object"))
                .clone(),
        ),
        paxeer: Some(PxCustodyAsset {
            asset_id: [0xab; 32],
            denom: "upax".to_owned(),
            pointer: [0xcd; 20],
            enabled: true,
            paused: false,
            minimum_deposit: 7,
            custody_cap: u128::MAX,
            custodied: 0x1_0000_0000_0000_0001,
            released: 9,
            pending: 10,
        }),
    };
    assert_eq!(
        must(list_assets(&endpoint)),
        PxAssetTable {
            assets: vec![native.clone(), custody.clone()],
            joined_limit: 16
        }
    );
    assert_eq!(must(get_asset(&endpoint, [0xab; 32])), custody);
    assert_eq!(must(get_asset(&endpoint, [0x11; 32])), native);
    assert_eq!(
        get_asset(&endpoint, [0xff; 32]),
        Err(SequencerError::Unavailable)
    );
    assert_eq!(
        must(list_assets(&endpoint)),
        PxAssetTable {
            assets: vec![],
            joined_limit: 16
        }
    );
    assert_eq!(
        get_asset(&endpoint, [0xab; 32]),
        Err(SequencerError::Unavailable)
    );
    assert_eq!(list_assets(&endpoint), Err(SequencerError::Malformed));
    assert_eq!(
        get_asset(&endpoint, [0xab; 32]),
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
