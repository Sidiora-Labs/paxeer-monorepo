use std::fmt::Write as _;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::{Duration, Instant};

use layerx_client::px::PxEndpoint;
use layerx_client::rpc::withdrawal::{registry, WithdrawalConfigurationError};
use layerx_client::sequencer::{CallRequest, SequencerEndpoint};
use layerx_types::payload::{ActivityType, ModuleId};
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

fn serve(exchanges: Vec<(&'static str, Value)>) -> (String, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    let url = format!("http://{}/", must(listener.local_addr()));
    let server = std::thread::spawn(move || {
        let mut requests = Vec::new();
        for (method, mut response) in exchanges {
            let deadline = Instant::now() + Duration::from_secs(5);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing {method} request");
                        std::thread::sleep(Duration::from_millis(10));
                    }
                    Err(error) => panic!("{error}"),
                }
            };
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
    (url, server)
}

#[test]
fn withdrawal_registry_reads_the_fee_schedule_and_asset_through_json_rpc() {
    let asset_id = [0xab; 32];
    let call = CallRequest {
        from: Some([0x12; 20]),
        to: Some([0x34; 20]),
        value: 7,
        data: vec![0xde, 0xad, 0xbe, 0xef, 0, 0xff],
    };
    let custody = json!({
        "asset_id": "ab".repeat(32), "denom": "upax",
        "pointer": hex(&[0xcd; 20]), "enabled": true, "paused": false,
        "minimum_deposit": "7", "custody_cap": "1000000",
        "custodied": "100", "released": "9", "pending": "10"
    });
    let row = json!({
        "asset_id": "ab".repeat(32), "layerx": null, "paxeer": custody
    });
    let mut unavailable_rows = vec![
        vec![],
        vec![json!({"asset_id": "11".repeat(32), "layerx": null, "paxeer": null})],
        vec![json!({"asset_id": "ab".repeat(32), "layerx": null, "paxeer": null})],
    ];
    for (field, value) in [
        ("enabled", json!(false)),
        ("paused", json!(true)),
        ("pointer", json!(hex(&[0; 20]))),
        ("asset_id", json!("11".repeat(32))),
    ] {
        let mut unavailable = row.clone();
        unavailable["paxeer"][field] = value;
        unavailable_rows.push(vec![unavailable]);
    }
    let table = |rows: Vec<Value>| json!({"result": {"assets": rows, "joined_limit": 16}});
    let mut exchanges = vec![
        ("eth_estimateGas", json!({"result": "0x12345"})),
        (
            "px_listAssets",
            table(vec![
                json!({"asset_id": "11".repeat(32), "layerx": null, "paxeer": null}),
                row,
            ]),
        ),
    ];
    for rows in &unavailable_rows {
        exchanges.push(("eth_estimateGas", json!({"result": "0x5208"})));
        exchanges.push(("px_listAssets", table(rows.clone())));
    }
    exchanges.extend([
        (
            "eth_estimateGas",
            json!({"error": {"code": 3, "message": "execution reverted"}}),
        ),
        ("eth_estimateGas", json!({"result": "invalid"})),
        ("eth_estimateGas", json!({"result": "0x5208"})),
        (
            "px_listAssets",
            json!({"result": {"assets": [], "joined_limit": "invalid"}}),
        ),
        ("eth_estimateGas", json!({"result": "0x5208"})),
        (
            "px_listAssets",
            json!({"error": {"code": -32603, "message": "asset read failed"}}),
        ),
    ]);
    let expected_requests = exchanges.len();
    let (url, server) = serve(exchanges);
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    let assets = must(PxEndpoint::new(&url, &[]));

    let modules = must(registry(&endpoint, &call, &assets, asset_id));
    assert_eq!(modules.registrations().len(), 2);
    for (registration, (module, ordinal)) in modules
        .registrations()
        .iter()
        .zip([(ModuleId::Asset, 9), (ModuleId::Bridge, 1)])
    {
        let activity = must(ActivityType::new(module, ordinal));
        assert_eq!(registration.module(), module);
        assert_eq!(registration.activity_types(), &[activity]);
        assert!(modules.declares(activity));
    }
    assert!(!modules.declares(must(ActivityType::new(ModuleId::Asset, 1))));
    for _ in unavailable_rows {
        assert_eq!(
            registry(&endpoint, &call, &assets, asset_id),
            Err(WithdrawalConfigurationError::AssetUnavailable)
        );
    }
    for _ in 0..4 {
        assert_eq!(
            registry(&endpoint, &call, &assets, asset_id),
            Err(WithdrawalConfigurationError::Unavailable)
        );
    }

    let requests = must(server.join());
    assert_eq!(requests.len(), expected_requests);
    for request in requests {
        match request["method"].as_str() {
            Some("eth_estimateGas") => assert_eq!(
                request["params"],
                json!([{
                "from": hex(&[0x12; 20]), "to": hex(&[0x34; 20]),
                "value": "0x7", "data": hex(&call.data),
            }, "pending"])
            ),
            Some("px_listAssets") => assert_eq!(request["params"], json!([])),
            method => panic!("unexpected RPC method: {method:?}"),
        }
    }
}
