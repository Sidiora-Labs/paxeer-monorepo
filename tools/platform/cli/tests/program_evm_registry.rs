#[path = "../src/config.rs"]
mod config;
#[path = "../src/credential.rs"]
mod credential;
#[path = "../src/encoding.rs"]
mod encoding;
#[path = "../src/file_store.rs"]
mod file_store;
#[path = "../src/http.rs"]
mod http;
#[path = "../src/programs.rs"]
mod programs;

use std::io::{BufRead as _, Read as _, Write as _};
use std::net::{Shutdown, TcpListener};
use std::process::{Command, Output};
use std::time::{Duration, Instant};

use serde_json::{json, Value};

const ADDRESS: &str = "0x000000000000000000000000000000000000101d";
const VECTOR: &str =
    include_str!("../../../../sdk/platform/conformance/fixtures/native-program-deploy-v4.json");

fn vector() -> Value {
    serde_json::from_str(VECTOR).unwrap_or_else(|error| panic!("{error}"))
}

// This is an outbound wire capture, with no fabricated RPC replies or node.
fn capture() -> (String, std::thread::JoinHandle<Value>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap_or_else(|error| panic!("{error}"));
    let endpoint = format!(
        "http://{}",
        listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error}"))
    );
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("{error}"));
    let worker = std::thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(10);
        let (socket, _) = loop {
            match listener.accept() {
                Ok(connection) => break connection,
                Err(error)
                    if error.kind() == std::io::ErrorKind::WouldBlock
                        && Instant::now() < deadline =>
                {
                    std::thread::sleep(Duration::from_millis(5));
                }
                Err(error) => panic!("no CLI request: {error}"),
            }
        };
        socket
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap_or_else(|error| panic!("{error}"));
        let mut reader = std::io::BufReader::new(socket);
        let mut line = String::new();
        reader
            .read_line(&mut line)
            .unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(line, "POST /rpc HTTP/1.1\r\n");
        let mut length = None;
        loop {
            line.clear();
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("{error}"));
            if line == "\r\n" {
                break;
            }
            if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                length = Some(
                    value
                        .trim()
                        .parse::<usize>()
                        .unwrap_or_else(|error| panic!("{error}")),
                );
            }
        }
        let length = length.unwrap_or_else(|| panic!("request must have a content length"));
        assert!(length <= 4 * 1024 * 1024);
        let mut body = vec![0; length];
        reader
            .read_exact(&mut body)
            .unwrap_or_else(|error| panic!("{error}"));
        reader
            .get_ref()
            .shutdown(Shutdown::Both)
            .unwrap_or_else(|error| panic!("{error}"));
        let request: Value =
            serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{error}"));
        assert_eq!(request["jsonrpc"], "2.0");
        assert_eq!(request["id"], 1);
        assert!(
            matches!(listener.accept(), Err(error) if error.kind() == std::io::ErrorKind::WouldBlock)
        );
        request
    });
    (endpoint, worker)
}

fn cli(directory: &std::path::Path, arguments: &[&str]) -> Output {
    Command::new(env!("CARGO_BIN_EXE_layerx"))
        .args(arguments)
        .env("LAYERX_CONFIG", directory.join("config.json"))
        .env("LAYERX_CREDENTIAL_STORE", "file")
        .env(
            "LAYERX_CREDENTIAL_PASSPHRASE",
            "program transport contract test",
        )
        .output()
        .unwrap_or_else(|error| panic!("{error}"))
}

fn captured_command(arguments: &[&str]) -> Result<Value, String> {
    let directory = tempfile::tempdir().map_err(|error| error.to_string())?;
    let (endpoint, recorded) = capture();
    let mut command = vec!["--json", "--rpc"];
    let rpc = format!("{endpoint}/rpc");
    command.push(&rpc);
    command.push("program");
    command.extend_from_slice(arguments);
    let output = cli(directory.path(), &command);
    assert!(
        !output.status.success(),
        "wire capture supplies no RPC response"
    );
    recorded.join().map_err(|_| "request capture failed".into())
}

fn word(value: u64) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[24..].copy_from_slice(&value.to_be_bytes());
    bytes
}

fn abi_hex(bytes: &[u8]) -> Value {
    json!(format!("0x{}", encoding::hex_encode(bytes)))
}

fn registry_listing(
    head: u64,
    max_span: u64,
    earliest: u64,
    refusal: &'static str,
    logs: Vec<Value>,
) -> Result<(Output, Vec<Value>), String> {
    let directory = tempfile::tempdir().map_err(|error| error.to_string())?;
    let listener = TcpListener::bind("127.0.0.1:0").map_err(|error| error.to_string())?;
    let endpoint = format!(
        "http://{}/rpc",
        listener.local_addr().map_err(|e| e.to_string())?
    );
    listener
        .set_nonblocking(true)
        .map_err(|error| error.to_string())?;
    let (stop, stopped) = std::sync::mpsc::channel();
    let worker = std::thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(30);
        let mut requests = Vec::new();
        while stopped.try_recv().is_err() {
            assert!(Instant::now() < deadline, "registry listing did not finish");
            let (socket, _) = match listener.accept() {
                Ok(connection) => connection,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    std::thread::sleep(Duration::from_millis(1));
                    continue;
                }
                Err(error) => panic!("{error}"),
            };
            socket
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("{error}"));
            let mut reader = std::io::BufReader::new(socket);
            let mut line = String::new();
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("{error}"));
            assert_eq!(line, "POST /rpc HTTP/1.1\r\n");
            let mut length = None;
            loop {
                line.clear();
                assert!(
                    reader
                        .read_line(&mut line)
                        .unwrap_or_else(|error| panic!("{error}"))
                        > 0
                );
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(
                        value
                            .trim()
                            .parse::<usize>()
                            .unwrap_or_else(|error| panic!("{error}")),
                    );
                }
            }
            let length = length.unwrap_or_else(|| panic!("missing content length"));
            assert!(length <= 4 * 1024 * 1024);
            let mut body = vec![0; length];
            reader
                .read_exact(&mut body)
                .unwrap_or_else(|error| panic!("{error}"));
            let request: Value =
                serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{error}"));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["id"], 1);
            let mut response = json!({"jsonrpc": "2.0", "id": 1});
            match request["method"].as_str() {
                Some("eth_blockNumber") => {
                    assert!(requests.is_empty(), "the head must be pinned once, first");
                    assert_eq!(request["params"], json!([]));
                    response["result"] = json!(format!("0x{head:x}"));
                }
                Some("eth_getLogs") => {
                    let filter = &request["params"][0];
                    assert_eq!(filter["address"], ADDRESS);
                    assert_eq!(filter["topics"], json!([programs::PROGRAM_DEPLOYED_TOPIC]));
                    let lower = log_height(&filter["fromBlock"]);
                    let upper = log_height(&filter["toBlock"]);
                    assert!(lower <= upper && upper <= head);
                    assert!(upper - lower < 2000, "unbounded registry log request");
                    let error = if upper - lower + 1 > max_span {
                        Some(format!(
                            "block range too large ({}), maximum allowed is {max_span} blocks",
                            upper - lower + 1
                        ))
                    } else if lower < earliest {
                        Some(match refusal {
                            "pruned" => format!("requested block range [{lower}, {upper}] includes pruned blocks, earliest available block is {earliest}"),
                            "to_before" => format!("requested toBlock {upper} is before earliest available block {earliest}"),
                            _ => format!("requested fromBlock {lower} is before earliest available block {earliest}"),
                        })
                    } else if refusal == "other" {
                        Some("log query rate limit exceeded for large queries, please try again later".to_owned())
                    } else {
                        None
                    };
                    if let Some(message) = error {
                        response["error"] = json!({"code": -32000, "message": message});
                    } else {
                        response["result"] = json!(logs
                            .iter()
                            .filter(|entry| {
                                let height = log_height(&entry["blockNumber"]);
                                lower <= height && height <= upper
                            })
                            .collect::<Vec<_>>());
                    }
                }
                method => panic!("unexpected RPC method: {method:?}"),
            }
            requests.push(request);
            let body = serde_json::to_vec(&response).unwrap_or_else(|error| panic!("{error}"));
            let socket = reader.get_mut();
            write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())
                .unwrap_or_else(|error| panic!("{error}"));
            socket
                .write_all(&body)
                .unwrap_or_else(|error| panic!("{error}"));
        }
        requests
    });
    let output = cli(
        directory.path(),
        &["--json", "--rpc", &endpoint, "program", "registry", "list"],
    );
    stop.send(()).map_err(|error| error.to_string())?;
    let requests = worker.join().map_err(|_| "registry peer failed")?;
    Ok((output, requests))
}

fn log_height(value: &Value) -> u64 {
    let encoded = value
        .as_str()
        .and_then(|s| s.strip_prefix("0x"))
        .unwrap_or_else(|| panic!("expected a numeric hex block: {value}"));
    let height = u64::from_str_radix(encoded, 16).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(value, &json!(format!("0x{height:x}")));
    height
}

#[test]
fn program_discover_reads_get_program() -> Result<(), String> {
    let program_id = "11".repeat(32);
    for arguments in [
        vec!["discover", program_id.as_str()],
        vec!["registry", "get", program_id.as_str()],
    ] {
        let request = captured_command(&arguments)?;
        assert_eq!(request["method"], "eth_call");
        assert_eq!(
            request["params"],
            json!([
                {"to": ADDRESS, "data": format!("0xb1ffb3d4{program_id}")}, "pending"
            ])
        );
    }
    let fixture = vector();
    let payload = encoding::hex_decode(
        "deployment",
        fixture["payload_hex"].as_str().ok_or("missing payload")?,
    )?;
    let deploy = layerx_types::program_lifecycle::NativeProgramDeploy::decode(&payload)
        .map_err(|error| format!("{error:?}"))?;
    let mut bytes = Vec::new();
    for field in [
        word(1),
        [0x22; 32],
        deploy.new_hash,
        word(u64::from(deploy.guest_abi)),
        word(1),
        word(1),
    ] {
        bytes.extend_from_slice(&field);
    }
    let rendered = programs::decode_program_view(&program_id, &abi_hex(&bytes))?;
    assert_eq!(rendered["program_id"], program_id);
    assert_eq!(
        rendered["code_hash"],
        encoding::hex_encode(&deploy.new_hash)
    );
    assert_eq!(rendered["authority"], "22".repeat(32));
    assert_eq!(rendered["policy"], 1);
    assert_eq!(rendered["abi_version"], deploy.guest_abi);
    assert_eq!(rendered["version"], 1);
    assert_eq!(rendered["active"], true);
    bytes[191] = 0;
    assert_eq!(
        programs::decode_program_view(&program_id, &abi_hex(&bytes))?["active"],
        false
    );
    bytes[191] = 2;
    assert!(programs::decode_program_view(&program_id, &abi_hex(&bytes)).is_err());
    bytes[191] = 1;
    bytes[0] = 1;
    assert!(programs::decode_program_view(&program_id, &abi_hex(&bytes)).is_err());
    bytes[0] = 0;
    assert!(programs::decode_program_view(&program_id, &abi_hex(&bytes[..191])).is_err());
    assert!(
        programs::decode_program_view(&program_id, &json!({"program_id": program_id})).is_err()
    );
    let client = http::Client::new("http://127.0.0.1:1", None)?;
    assert!(programs::discover(&client, "../unsafe").is_err());
    Ok(())
}

#[test]
fn program_interface_get_reads_get_interface() -> Result<(), String> {
    let program_id = "11".repeat(32);
    let request = captured_command(&["interface", "get", &program_id])?;
    assert_eq!(request["method"], "eth_call");
    assert_eq!(
        request["params"],
        json!([
            {"to": ADDRESS, "data": format!("0x8efc7240{program_id}")}, "pending"
        ])
    );
    let fixture = vector();
    let payload = encoding::hex_decode(
        "deployment",
        fixture["payload_hex"].as_str().ok_or("missing payload")?,
    )?;
    let deploy = layerx_types::program_lifecycle::NativeProgramDeploy::decode(&payload)
        .map_err(|error| format!("{error:?}"))?;
    let encoded = deploy.interface.ok_or("fixture omitted interface")?;
    let interface =
        layerx_programs::ProgramInterface::decode(encoded).map_err(|error| error.to_string())?;
    let mut bytes = Vec::new();
    bytes.extend_from_slice(&word(1));
    bytes.extend_from_slice(interface.digest().as_bytes());
    bytes.extend_from_slice(&word(96));
    bytes.extend_from_slice(&word(encoded.len() as u64));
    bytes.extend_from_slice(encoded);
    bytes.resize(bytes.len().next_multiple_of(32), 0);
    let rendered = programs::decode_interface_view(&program_id, &abi_hex(&bytes))?;
    assert_eq!(rendered["interface"], encoding::hex_encode(encoded));
    assert_eq!(
        rendered["interface_digest"],
        encoding::hex_encode(interface.digest().as_bytes())
    );
    assert_eq!(
        rendered["code_hash"],
        encoding::hex_encode(&deploy.new_hash)
    );
    assert_eq!(rendered["abi_version"], deploy.guest_abi);
    assert_eq!(rendered["version"], 1);
    let mut changed = bytes.clone();
    changed[32] ^= 1;
    assert!(programs::decode_interface_view(&program_id, &abi_hex(&changed)).is_err());
    changed = bytes.clone();
    changed[95] = 64;
    assert!(programs::decode_interface_view(&program_id, &abi_hex(&changed)).is_err());
    changed = bytes.clone();
    *changed.last_mut().ok_or("missing padding")? = 1;
    assert!(programs::decode_interface_view(&program_id, &abi_hex(&changed)).is_err());
    changed = bytes.clone();
    changed[0] = 1;
    assert!(programs::decode_interface_view(&program_id, &abi_hex(&changed)).is_err());
    bytes.extend_from_slice(&word(0));
    assert!(programs::decode_interface_view(&program_id, &abi_hex(&bytes)).is_err());
    Ok(())
}

#[test]
fn program_registry_list_reads_logs() -> Result<(), String> {
    let (output, requests) = registry_listing(10, 2000, 0, "", vec![])?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(requests.len(), 2);
    let request = &requests[0];
    assert_eq!(request["method"], "eth_blockNumber");
    assert_eq!(request["params"], json!([]));
    assert_eq!(requests[1]["method"], "eth_getLogs");
    assert_eq!(requests[1]["params"][0]["fromBlock"], "0x0");
    assert_eq!(requests[1]["params"][0]["toBlock"], "0xa");
    let fixture = vector();
    let payload = encoding::hex_decode(
        "deployment",
        fixture["payload_hex"].as_str().ok_or("missing payload")?,
    )?;
    let deploy = layerx_types::program_lifecycle::NativeProgramDeploy::decode(&payload)
        .map_err(|error| format!("{error:?}"))?;
    let event = json!({
        "address": ADDRESS, "removed": false,
        "topics": [programs::PROGRAM_DEPLOYED_TOPIC, format!("0x{}", encoding::hex_encode(&deploy.program_id.bytes()))],
        "data": format!("0x{}", encoding::hex_encode(&deploy.new_hash))
    });
    let rendered = programs::decode_registry_logs(&json!([event, event]))?;
    assert_eq!(rendered["program_ids"], json!(["11".repeat(32)]));
    assert_eq!(
        rendered["events"].as_array().ok_or("missing events")?.len(),
        2
    );
    assert_eq!(
        programs::decode_registry_logs(&json!([]))?["program_ids"],
        json!([])
    );
    for changed in [
        {
            let mut e = event.clone();
            e["removed"] = json!(true);
            e
        },
        {
            let mut e = event.clone();
            e["address"] = json!("0x000000000000000000000000000000000000101c");
            e
        },
        {
            let mut e = event.clone();
            e["topics"][0] = json!(format!("0x{}", "00".repeat(32)));
            e
        },
        {
            let mut e = event.clone();
            e["topics"][1] = json!("0x11");
            e
        },
        {
            let mut e = event.clone();
            e["data"] = json!("0x00");
            e
        },
    ] {
        assert!(programs::decode_registry_logs(&json!([changed])).is_err());
    }
    assert!(programs::decode_registry_logs(&json!({"program_ids": []})).is_err());
    let mut first = event.clone();
    first["blockNumber"] = json!("0x1");
    let mut second = first.clone();
    second["topics"][1] = json!(format!("0x{}", "22".repeat(32)));
    let (output, requests) = registry_listing(10, 2000, 0, "", vec![first, second])?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(requests.len(), 2);
    assert_eq!(requests[0]["method"], "eth_blockNumber");
    assert_eq!(requests[0]["params"], json!([]));
    assert_eq!(requests[1]["method"], "eth_getLogs");
    assert_eq!(requests[1]["params"][0]["fromBlock"], "0x0");
    assert_eq!(requests[1]["params"][0]["toBlock"], "0xa");
    let listing: Value =
        serde_json::from_slice(&output.stdout).map_err(|error| error.to_string())?;
    let listing = &listing["data"];
    assert_eq!(
        listing["program_ids"],
        json!(["11".repeat(32), "22".repeat(32)])
    );
    assert_eq!(
        listing["events"].as_array().ok_or("missing events")?.len(),
        2
    );
    Ok(())
}

#[test]
fn program_registry_list_pages_bounded_ranges() -> Result<(), String> {
    program_registry_list_reads_logs()?;
    let early = json!({
        "address": ADDRESS, "removed": false,
        "topics": [programs::PROGRAM_DEPLOYED_TOPIC, format!("0x{}", "22".repeat(32))],
        "data": format!("0x{}", "33".repeat(32)),
        "transactionHash": format!("0x{}", "44".repeat(32)),
        "logIndex": "0x0", "blockNumber": "0x0"
    });
    let mut late = early.clone();
    late["topics"][1] = json!(format!("0x{}", "11".repeat(32)));
    late["transactionHash"] = json!(format!("0x{}", "55".repeat(32)));
    late["blockNumber"] = json!("0x7d0");
    let (output, requests) = registry_listing(
        2000,
        2000,
        0,
        "",
        vec![early.clone(), early.clone(), late.clone(), late.clone()],
    )?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(requests.len(), 3);
    assert_eq!(requests[0]["method"], "eth_blockNumber");
    assert_eq!(requests[0]["params"], json!([]));
    assert_eq!(requests[1]["params"][0]["fromBlock"], "0x1");
    assert_eq!(requests[1]["params"][0]["toBlock"], "0x7d0");
    assert_eq!(requests[2]["params"][0]["fromBlock"], "0x0");
    assert_eq!(requests[2]["params"][0]["toBlock"], "0x0");
    let listing: Value =
        serde_json::from_slice(&output.stdout).map_err(|error| error.to_string())?;
    let listing = &listing["data"];
    assert_eq!(
        listing["program_ids"],
        json!(["11".repeat(32), "22".repeat(32)])
    );
    assert_eq!(listing["events"], json!([early, late]));
    assert_eq!(listing["from_block"], 0);
    assert_eq!(listing["to_block"], 2000);
    assert_eq!(listing["earliest_scanned"], 0);
    assert_eq!(listing["history_complete"], true);
    assert!(listing.get("block_tag").is_none());

    for refusal in ["before", "pruned", "to_before"] {
        let mut retained = early.clone();
        retained["blockNumber"] = json!("0x5");
        let (output, requests) = registry_listing(
            2000,
            1,
            5,
            refusal,
            vec![retained.clone(), retained.clone(), late.clone()],
        )?;
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let listing: Value =
            serde_json::from_slice(&output.stdout).map_err(|error| error.to_string())?;
        let listing = &listing["data"];
        assert_eq!(listing["events"], json!([retained, late]));
        assert_eq!(
            listing["program_ids"],
            json!(["11".repeat(32), "22".repeat(32)])
        );
        assert_eq!(listing["to_block"], 2000);
        assert_eq!(listing["history_complete"], false);
        assert_eq!(listing["earliest_scanned"], 5);
        let mut expected_upper = 2000;
        let mut expected_span = 2000;
        for request in &requests[1..] {
            assert_eq!(request["method"], "eth_getLogs");
            let lower = log_height(&request["params"][0]["fromBlock"]);
            let upper = log_height(&request["params"][0]["toBlock"]);
            assert_eq!(upper, expected_upper);
            assert_eq!(upper - lower + 1, expected_span);
            if expected_span > 1 {
                expected_span /= 2;
            } else if lower >= 5 {
                expected_upper = lower - 1;
            } else {
                assert_eq!(lower, 4);
            }
        }
        assert_eq!(
            requests.last().ok_or("missing requests")?["params"][0]["toBlock"],
            "0x4"
        );
    }

    let (output, requests) = registry_listing(2000, 2000, 5, "before", vec![])?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(requests.len(), 2);
    let listing: Value =
        serde_json::from_slice(&output.stdout).map_err(|error| error.to_string())?;
    let listing = &listing["data"];
    assert_eq!(listing["events"], json!([]));
    assert_eq!(listing["history_complete"], false);
    assert!(listing["earliest_scanned"].is_null());

    for (head, max_span, refusal, message) in [
        (0, 0, "", "block range too large"),
        (2000, 2000, "other", "log query rate limit exceeded"),
    ] {
        let (output, requests) = registry_listing(head, max_span, 0, refusal, vec![])?;
        assert!(!output.status.success());
        assert_eq!(requests.len(), 2);
        let error = format!(
            "{}{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        assert!(error.contains(message), "{error}");
    }

    let mut invalid = late.clone();
    invalid["removed"] = json!(true);
    let (output, _) = registry_listing(2000, 2000, 0, "", vec![late, invalid])?;
    assert!(
        !output.status.success(),
        "duplicate identities must not bypass log validation"
    );
    Ok(())
}
