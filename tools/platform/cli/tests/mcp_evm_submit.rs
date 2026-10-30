#![allow(dead_code)]

#[path = "../src/config.rs"]
mod config;
#[path = "../src/credential.rs"]
mod credential;
#[path = "../src/encoding.rs"]
mod encoding;
#[path = "../src/faucet.rs"]
mod faucet;
#[path = "../src/file_store.rs"]
mod file_store;
#[path = "../src/output.rs"]
mod output;

use layerx_platform_cli::{http, rpc};

mod toolset {
    include!("../src/toolset.rs");

    pub fn runtime(endpoint: &str) -> Result<Runtime, String> {
        Ok(Runtime {
            client: crate::http::Client::new(endpoint, None)?,
            network_id: 125,
            environment: "test".to_owned(),
            endpoint: endpoint.to_owned(),
        })
    }

    pub fn result(raw: &[u8], response: &Value) -> Result<Value, String> {
        submission_result(raw, response)
    }
}

use std::io::{BufRead as _, BufReader, Read as _};
use std::net::{Shutdown, TcpListener};
use std::time::{Duration, Instant};

use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_mcp::server::DeploymentMode;
use serde_json::{json, Value};

fn signed_transaction() -> Result<Vec<u8>, Box<dyn std::error::Error>> {
    Ok(Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [0x22; 20],
        data: vec![0xab; 512],
    }
    .sign(&[1; 32])?)
}

// Capture the actual outbound HTTP request without supplying a fabricated node response.
fn capture() -> Result<(String, std::thread::JoinHandle<Result<Value, String>>), String> {
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    let endpoint = format!(
        "http://{}",
        listener.local_addr().map_err(|error| error.to_string())?
    );
    listener
        .set_nonblocking(true)
        .map_err(|error| error.to_string())?;
    let worker = std::thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(10);
        let stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    if Instant::now() >= deadline {
                        return Err("timed out waiting for the raw transaction request".into());
                    }
                    std::thread::sleep(Duration::from_millis(5));
                }
                Err(error) => return Err(error.to_string()),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .map_err(|error| error.to_string())?;
        let mut reader = BufReader::new(stream);
        let mut line = String::new();
        reader
            .read_line(&mut line)
            .map_err(|error| error.to_string())?;
        assert_eq!(line, "POST /rpc HTTP/1.1\r\n");
        let mut length = None;
        loop {
            line.clear();
            if reader
                .read_line(&mut line)
                .map_err(|error| error.to_string())?
                == 0
            {
                return Err("truncated HTTP headers".into());
            }
            if line == "\r\n" {
                break;
            }
            if let Some((name, value)) = line.split_once(':') {
                if name.eq_ignore_ascii_case("content-length") {
                    length = Some(
                        value
                            .trim()
                            .parse::<usize>()
                            .map_err(|error| error.to_string())?,
                    );
                }
            }
        }
        let length = length
            .filter(|length| *length <= 1_048_576)
            .ok_or("invalid content length")?;
        let mut body = vec![0; length];
        reader
            .read_exact(&mut body)
            .map_err(|error| error.to_string())?;
        reader
            .get_ref()
            .shutdown(Shutdown::Both)
            .map_err(|error| error.to_string())?;
        serde_json::from_slice(&body).map_err(|error| error.to_string())
    });
    Ok((endpoint, worker))
}

#[test]
fn mcp_submit_tool_sends_raw_transaction() -> Result<(), Box<dyn std::error::Error>> {
    let raw = signed_transaction()?;
    let encoded = format!("0x{}", encoding::hex_encode(&raw));
    let arguments = json!({"raw_transaction": encoded});
    let tools = toolset::surface(DeploymentMode::Full)?;
    let tool = tools
        .iter()
        .find(|tool| tool.name == "activity.submit")
        .copied()
        .ok_or("submit tool absent")?;
    let (endpoint, captured) = capture()?;
    let runtime = toolset::runtime(&endpoint)?;
    let outcome = toolset::invoke(&runtime, tool, &arguments);
    let sent = captured.join().map_err(|_| "wire capture panicked")??;
    assert!(
        outcome.is_err(),
        "a closed transport must not report submission success"
    );
    assert_eq!(
        sent,
        json!({"jsonrpc":"2.0", "id":1, "method":"eth_sendRawTransaction", "params":[encoded]})
    );
    let bytes = encoding::hex_decode(
        "sent transaction",
        sent["params"][0]
            .as_str()
            .ok_or("raw hex absent")?
            .strip_prefix("0x")
            .ok_or("hex prefix absent")?,
    )?;
    assert_eq!(bytes, raw);
    let (call, signature) = decode_signed(&bytes)?;
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    recover_sender(&call, &signature)?;

    let hash = format!("0x{}", encoding::hex_encode(&transaction_hash(&raw)));
    assert_eq!(
        toolset::result(&raw, &json!({"jsonrpc":"2.0", "id":1, "result":hash}))?,
        json!({"txHash":hash, "rung":"submitted"})
    );
    for response in [
        json!({"jsonrpc":"2.0", "id":1, "result":format!("0x{}", "00".repeat(32))}),
        json!({"jsonrpc":"2.0", "id":1, "result":null}),
        json!({"jsonrpc":"2.0", "id":2, "result":hash}),
        json!({"jsonrpc":"2.0", "id":1, "error":{"code":-32000, "message":"nonce too low"}}),
    ] {
        assert!(toolset::result(&raw, &response).is_err());
    }
    let listener = TcpListener::bind("127.0.0.1:0")?;
    listener.set_nonblocking(true)?;
    let runtime = toolset::runtime(&format!("http://{}", listener.local_addr()?))?;
    for invalid in [
        json!({"activity":"00".repeat(64)}),
        json!({"raw_transaction":"0x02c0"}),
        json!({"raw_transaction":encoded, "extra":true}),
        json!({"raw_transaction":encoded.trim_start_matches("0x")}),
    ] {
        assert!(toolset::invoke(&runtime, tool, &invalid).is_err());
        assert!(
            matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
        );
    }
    let wrong_chain = Eip1559Call {
        chain_id: 126,
        ..call
    }
    .sign(&[1; 32])?;
    assert!(toolset::invoke(
        &runtime,
        tool,
        &json!({"raw_transaction":format!("0x{}", encoding::hex_encode(&wrong_chain))})
    )
    .is_err());
    assert!(
        matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
    );

    assert_eq!(
        toolset::schema("activity.submit")["required"],
        json!(["raw_transaction"])
    );
    assert!(toolset::surface(DeploymentMode::ReadOnly)?
        .iter()
        .all(|tool| tool.name != "activity.submit"));
    assert!(
        layerx_mcp::catalogue::validate("activity.submit", &arguments).is_ok(),
        "layerx mcp serve uses the daemon catalogue, which must admit raw transaction submission: {:?}",
        layerx_mcp::catalogue::validate("activity.submit", &arguments)
    );
    let kernel = json!({
        "preparation_ref": "prepared-transaction",
        "signature": "ab".repeat(64),
        "signer_public_key": "cd".repeat(32),
    });
    assert_eq!(
        layerx_mcp::catalogue::validate("activity.submit", &kernel),
        Ok(())
    );
    let mut mixed = kernel.clone();
    mixed["raw_transaction"] = json!(encoded);
    assert_eq!(
        layerx_mcp::catalogue::validate("activity.submit", &mixed),
        Err(layerx_mcp::catalogue::ArgumentError::Unknown(
            "preparation_ref".to_owned()
        ))
    );
    assert_eq!(
        layerx_mcp::catalogue::validate("activity.submit", &json!({})),
        Err(layerx_mcp::catalogue::ArgumentError::Missing(
            "preparation_ref"
        ))
    );
    for invalid in [
        json!({"raw_transaction": null}),
        json!({"raw_transaction": ""}),
        json!({"raw_transaction": "0x02"}),
        json!({"raw_transaction": "0x020"}),
        json!({"raw_transaction": "0x02zz"}),
        json!({"raw_transaction": "0x01ab"}),
        json!({"raw_transaction": encoded.trim_start_matches("0x")}),
        json!({"raw_transaction": encoded, "signature": "ab".repeat(64)}),
        json!({"raw_transaction": encoded, "variant": "native_send_submit_v1"}),
        json!({"raw_transaction": encoded, "preparation_ref": null}),
        json!({"raw_transaction": format!("0x02{}", "ab".repeat(32_768))}),
    ] {
        assert!(layerx_mcp::catalogue::validate("activity.submit", &invalid).is_err());
    }
    mixed["variant"] = json!("native_send_submit_v1");
    assert!(layerx_mcp::catalogue::validate("activity.submit", &mixed).is_err());
    let schema = layerx_mcp::catalogue::input_schema("activity.submit")
        .ok_or("catalogue submit schema absent")?;
    assert_eq!(
        schema["oneOf"][0]["required"],
        json!(["preparation_ref", "signature", "signer_public_key"])
    );
    let native = &schema["oneOf"][1]["oneOf"];
    assert_eq!(
        native[0]["required"],
        json!(["variant", "preparation_ref", "signature", "signer_address"])
    );
    assert_eq!(native[1]["required"], json!(["raw_transaction"]));
    assert_eq!(native[1]["additionalProperties"], json!(false));
    assert_eq!(
        native[1]["properties"]["raw_transaction"]["pattern"],
        toolset::schema("activity.submit")["properties"]["raw_transaction"]["pattern"]
    );
    Ok(())
}

#[test]
fn rpc_request_admits_major_two_names() -> Result<(), Box<dyn std::error::Error>> {
    let raw = format!("0x{}", encoding::hex_encode(&signed_transaction()?));
    let address = format!("0x{}", "11".repeat(20));
    let hash = format!("0x{}", "ab".repeat(32));
    let mut cases = vec![
        ("eth_sendRawTransaction", json!([raw])),
        ("pax_sequencerReceipt", json!([hash])),
    ];
    for block in [
        "pending",
        "latest",
        "earliest",
        "safe",
        "finalized",
        "0x0",
        "0x7d",
    ] {
        cases.push(("eth_getTransactionCount", json!([address, block])));
    }
    for (method, params) in cases {
        assert_eq!(
            rpc::request(method, &params)?,
            json!({"jsonrpc":"2.0", "id":1, "method":method, "params":params})
        );
    }
    for (method, params) in [
        ("eth_sendRawTransaction", json!([])),
        ("eth_sendRawTransaction", json!(["0x"])),
        ("eth_sendRawTransaction", json!(["0x0"])),
        ("eth_sendRawTransaction", json!(["0xzz"])),
        ("eth_sendRawTransaction", json!(["02ab"])),
        ("eth_sendRawTransaction", json!([raw, "pending"])),
        ("eth_sendRawTransaction", json!([17])),
        (
            "eth_sendRawTransaction",
            json!([format!("0x{}", "ab".repeat(524_289))]),
        ),
        ("eth_getTransactionCount", json!([address])),
        ("eth_getTransactionCount", json!(["0x11", "pending"])),
        ("eth_getTransactionCount", json!([address, "0x00"])),
        ("eth_getTransactionCount", json!([address, "unknown"])),
        ("eth_getTransactionCount", json!([address, 1])),
        ("eth_getTransactionCount", json!([address, "pending", true])),
        ("pax_sequencerReceipt", json!([])),
        ("pax_sequencerReceipt", json!(["ab".repeat(32)])),
        (
            "pax_sequencerReceipt",
            json!([format!("0x{}", "ab".repeat(31))]),
        ),
        ("pax_sequencerReceipt", json!([hash, "latest"])),
        ("pax_sequencerReceipt", json!({"txHash":hash})),
    ] {
        assert!(rpc::request(method, &params).is_err(), "{method}: {params}");
    }
    assert_eq!(
        rpc::decode_response(
            "eth_getTransactionCount",
            &json!({"jsonrpc":"2.0", "id":1, "result":"0x7"})
        )?,
        json!("0x7")
    );
    for rung in ["submitted", "receipted", "committed"] {
        let status = json!({"txHash":hash, "rung":rung});
        assert_eq!(
            rpc::decode_response(
                "pax_sequencerReceipt",
                &json!({"jsonrpc":"2.0", "id":1, "result":status})
            )?,
            status
        );
    }
    for status in [
        json!({"txHash":hash, "rung":"finalised"}),
        json!({"rung":"submitted"}),
        json!({"txHash":"0x11", "rung":"submitted"}),
    ] {
        assert!(rpc::decode_response(
            "pax_sequencerReceipt",
            &json!({"jsonrpc":"2.0", "id":1, "result":status})
        )
        .is_err());
    }
    Ok(())
}
