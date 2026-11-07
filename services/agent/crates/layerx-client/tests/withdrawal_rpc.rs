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

fn producer_asset_table() -> Value {
    let transcript: Value = must(serde_json::from_str(include_str!(
        "fixtures/repair_22_33_1546_gateway.json"
    )));
    assert_eq!(transcript["provenance"]["assets"], "services/platform/hosted/gateway/tests/one_source_reads.rs::px_list_assets_joins_indexer_assets_with_chain_metadata, exact asserted result");
    transcript["assets"].clone()
}

fn producer_custody_configuration() -> Value {
    let vectors: Value = must(serde_json::from_str(include_str!(
        "../src/rpc/withdrawal_get_asset.json"
    )));
    assert_eq!(
        vectors["provenance"]["producer"],
        "precompiles/layerxcustody/layerxcustody.go::Execute/GetAssetMethod and assetRecord"
    );
    assert_eq!(
        vectors["provenance"]["abi"],
        "precompiles/layerxcustody/abi.json::getAsset"
    );
    vectors
}

fn assert_custody_request(request: &Value) {
    assert_eq!(request["method"], "eth_call");
    let selector = &layerx_crypto::evm_transaction::transaction_hash(b"getAsset(bytes32)")[..4];
    assert_eq!(
        request["params"],
        json!([{
        "to": "0x0000000000000000000000000000000000001013",
        "data": hex(&[selector, &[0x11; 32]].concat())
    }, "0x10"])
    );
}

fn withdrawal_call() -> CallRequest {
    CallRequest {
        from: Some([0x12; 20]),
        to: Some([0x34; 20]),
        value: 7,
        data: vec![0xde, 0xad, 0xbe, 0xef, 0, 0xff],
    }
}

#[test]
fn withdrawal_registry_reads_the_fee_schedule_and_asset_through_json_rpc() {
    let call = withdrawal_call();
    let table = producer_asset_table();
    assert_eq!(table["height"], 16);
    assert_eq!(table["assets"][0]["asset_id"], hex(&[0x11; 32]));
    assert_eq!(table["assets"][0]["paused"], false);
    assert_eq!(
        table["assets"][0]["verification"],
        "authenticated_committed_snapshot"
    );
    assert!(table["assets"][0].get("paxeer").is_none());
    let (url, server) = serve(vec![
        ("eth_estimateGas", json!({"result":"0x12345"})),
        ("px_listAssets", json!({"result":table})),
        (
            "eth_call",
            json!({"result":producer_custody_configuration()["eligible"]}),
        ),
    ]);
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    let assets = must(PxEndpoint::new(&url, &[]));
    let result = registry(&endpoint, &call, &assets, [0x11; 32]);
    let requests = must(server.join());
    assert_eq!(requests.len(), 3);
    assert_custody_request(&requests[2]);
    assert_eq!(
        requests[0]["params"],
        json!([{
        "from":hex(&[0x12;20]),"to":hex(&[0x34;20]),"value":"0x7","data":hex(&call.data)
    }, "pending"])
    );
    assert_eq!(requests[1]["params"], json!([]));
    // The committed ABI contract vector exercises eligibility; live replacement
    // producer wiring remains recorded separately in repair_22_33_1547.kvx.
    let modules = must(result);
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
}

#[test]
fn withdrawal_registry_refuses_missing_assets_malformed_metadata_and_rpc_failures() {
    let call = withdrawal_call();
    let mut empty = producer_asset_table();
    empty["assets"] = json!([]);
    let mut unrelated = producer_asset_table();
    unrelated["assets"]
        .as_array_mut()
        .unwrap_or_else(|| panic!("assets"))
        .remove(0);
    for table in [empty, unrelated] {
        let (url, server) = serve(vec![
            ("eth_estimateGas", json!({"result":"0x5208"})),
            ("px_listAssets", json!({"result":table})),
        ]);
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let assets = must(PxEndpoint::new(&url, &[]));
        assert_eq!(
            registry(&endpoint, &call, &assets, [0x11; 32]),
            Err(WithdrawalConfigurationError::AssetUnavailable)
        );
        assert_eq!(must(server.join()).len(), 2);
    }
    for (field, invalid) in [
        ("joined_limit", json!("invalid")),
        ("height", json!(-1)),
        ("assets", Value::Null),
    ] {
        let mut table = producer_asset_table();
        table[field] = invalid;
        let (url, server) = serve(vec![
            ("eth_estimateGas", json!({"result":"0x5208"})),
            ("px_listAssets", json!({"result":table})),
        ]);
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let assets = must(PxEndpoint::new(&url, &[]));
        assert_eq!(
            registry(&endpoint, &call, &assets, [0x11; 32]),
            Err(WithdrawalConfigurationError::Unavailable)
        );
        assert_eq!(must(server.join()).len(), 2);
    }
    for failure in [
        json!({"error":{"code":3,"message":"execution reverted"}}),
        json!({"result":"invalid"}),
    ] {
        let (url, server) = serve(vec![("eth_estimateGas", failure)]);
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let assets = must(PxEndpoint::new(&url, &[]));
        assert_eq!(
            registry(&endpoint, &call, &assets, [0x11; 32]),
            Err(WithdrawalConfigurationError::Unavailable)
        );
        assert_eq!(must(server.join()).len(), 1);
    }
    let (url, server) = serve(vec![
        ("eth_estimateGas", json!({"result":"0x5208"})),
        (
            "px_listAssets",
            json!({"error":{"code":-32603,"message":"asset read failed"}}),
        ),
    ]);
    let endpoint = must(SequencerEndpoint::new(&url, &[]));
    let assets = must(PxEndpoint::new(&url, &[]));
    assert_eq!(
        registry(&endpoint, &call, &assets, [0x11; 32]),
        Err(WithdrawalConfigurationError::Unavailable)
    );
    assert_eq!(must(server.join()).len(), 2);
}

#[test]
fn withdrawal_registry_compares_asset_bytes_and_requires_authoritative_configuration() {
    let call = withdrawal_call();
    let vectors = producer_custody_configuration();
    // Equal byte identities need not share the gateway's hex presentation.
    for id in ["11".repeat(32), hex(&[0x11; 32])] {
        let mut table = producer_asset_table();
        table["assets"][0]["asset_id"] = json!(id);
        let (url, server) = serve(vec![
            ("eth_estimateGas", json!({"result":"0x5208"})),
            ("px_listAssets", json!({"result":table})),
            ("eth_call", json!({"result":vectors["eligible"]})),
        ]);
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let assets = must(PxEndpoint::new(&url, &[]));
        let modules = must(registry(&endpoint, &call, &assets, [0x11; 32]));
        assert!(modules.declares(must(ActivityType::new(ModuleId::Asset, 9))));
        assert!(modules.declares(must(ActivityType::new(ModuleId::Bridge, 1))));
        let requests = must(server.join());
        assert_eq!(requests.len(), 3);
        assert_custody_request(&requests[2]);
    }
    for id in [
        "11".repeat(31),
        "11".repeat(33),
        format!("0x{}zz", "11".repeat(31)),
        format!("0x0x{}", "11".repeat(32)),
    ] {
        let mut table = producer_asset_table();
        table["assets"][0]["asset_id"] = json!(id);
        let (url, server) = serve(vec![
            ("eth_estimateGas", json!({"result":"0x5208"})),
            ("px_listAssets", json!({"result":table})),
        ]);
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let assets = must(PxEndpoint::new(&url, &[]));
        assert_eq!(
            registry(&endpoint, &call, &assets, [0x11; 32]),
            Err(WithdrawalConfigurationError::AssetUnavailable)
        );
        assert_eq!(must(server.join()).len(), 2);
    }
    let mut refusals: Vec<_> = ["disabled", "paused", "zero_pointer", "wrong_asset"]
        .iter()
        .map(|case| {
            (
                json!({"result":vectors[case]}),
                WithdrawalConfigurationError::AssetUnavailable,
            )
        })
        .collect();
    refusals.extend([
        (json!({"result":"0x00"}), WithdrawalConfigurationError::Unavailable),
        (json!({"error":{"code":-32000,"message":"historical state unavailable"}}), WithdrawalConfigurationError::Unavailable),
        (json!({"error":{"code":3,"message":"execution reverted: LXP_MODULE_DISABLED: module disabled; layerxCustody retired"}}), WithdrawalConfigurationError::Unsupported),
    ]);
    for (answer, error) in refusals {
        let (url, server) = serve(vec![
            ("eth_estimateGas", json!({"result":"0x5208"})),
            ("px_listAssets", json!({"result":producer_asset_table()})),
            ("eth_call", answer),
        ]);
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let assets = must(PxEndpoint::new(&url, &[]));
        assert_eq!(registry(&endpoint, &call, &assets, [0x11; 32]), Err(error));
        let requests = must(server.join());
        assert_eq!(requests.len(), 3);
        assert_custody_request(&requests[2]);
    }
}
