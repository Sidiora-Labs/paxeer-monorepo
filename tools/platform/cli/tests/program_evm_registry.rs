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

use std::io::{BufRead as _, Read as _};
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
    let request = captured_command(&["registry", "list"])?;
    assert_eq!(request["method"], "eth_getLogs");
    assert_eq!(
        request["params"],
        json!([{
            "address": ADDRESS, "fromBlock": "earliest", "toBlock": "latest",
            "topics": ["0x2765c1020c93cab47e389edaa5b5c68834bdd76f7a5d384adb460fc86e688709"]
        }])
    );
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
    Ok(())
}
