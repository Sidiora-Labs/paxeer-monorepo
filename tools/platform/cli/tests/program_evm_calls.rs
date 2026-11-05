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

use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use serde_json::{json, Value};

const ADDRESS: &str = "0x000000000000000000000000000000000000101d";
const VECTOR: &str =
    include_str!("../../../../sdk/platform/conformance/fixtures/native-program-call-v4.json");

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

fn native_options(vector: &Value) -> programs::NativeCallOptions {
    let payload = encoding::hex_decode(
        "payload",
        vector["payload_hex"].as_str().unwrap_or_default(),
    )
    .unwrap_or_else(|error| panic!("{error}"));
    let call = layerx_types::program_call::NativeProgramCall::decode(&payload)
        .unwrap_or_else(|error| panic!("{error:?}"));
    programs::NativeCallOptions {
        abi_version: call.guest_abi,
        entrypoint: String::from_utf8(call.entrypoint.to_vec())
            .unwrap_or_else(|error| panic!("{error}")),
        access_declaration: Some(encoding::hex_encode(call.access_declaration)),
        response_capacity: call.response_capacity,
        memory_bytes: call.resources.0[1],
        storage_read_bytes: call.resources.0[2],
        storage_write_bytes: call.resources.0[3],
        output_values: call.resources.0[4],
        output_bytes: call.resources.0[5],
        table_elements: call.resources.0[6],
    }
}

#[test]
fn program_simulate_uses_eth_call_on_layerxprograms() -> Result<(), String> {
    let directory = tempfile::tempdir().map_err(|error| error.to_string())?;
    let vector = vector();
    let native = native_options(&vector);
    let capability = format!("transfer402:{}:{}:7", "22".repeat(32), "33".repeat(32));
    let (endpoint, recorded) = capture();
    let result = cli(
        directory.path(),
        &[
            "--json",
            "--rpc",
            &format!("{endpoint}/rpc"),
            "program",
            "simulate",
            &"11".repeat(32),
            "--fuel",
            "1000000",
            "--calldata",
            "0061ff10",
            "--fee-limit",
            "1000",
            "--response-capacity",
            "16",
            "--capability",
            &capability,
            "--access-declaration",
            native
                .access_declaration
                .as_deref()
                .ok_or("missing access declaration")?,
        ],
    );
    assert!(
        !result.status.success(),
        "the capture does not serve node responses"
    );
    let request = recorded.join().map_err(|_| "capture thread failed")?;
    assert_eq!(request["method"], "eth_call");
    assert_eq!(request["params"][0]["to"], ADDRESS);
    assert_eq!(request["params"][1], "pending");
    let data = request["params"][0]["data"]
        .as_str()
        .ok_or("missing calldata")?;
    assert!(data.starts_with("0x3044bd56"));
    assert_eq!(&data[10..74], "11".repeat(32));
    let expected = vector["evm"]["calldata"]
        .as_str()
        .ok_or("missing EVM vector")?;
    assert_eq!(&data[10..], &expected[10..]);

    // Decode the actual precompile ABI tuple, including signed result words.
    let mut answer = vec![0; 160];
    answer[63] = 2;
    answer[95] = 96;
    answer[127] = 3;
    answer[128..131].copy_from_slice(&[0, 0xff, 0x10]);
    let rendered =
        programs::simulated_program_result(&json!(format!("0x{}", encoding::hex_encode(&answer))))?;
    assert_eq!(
        rendered,
        json!({"outcome":{"terminal_result":0,"terminal_kind":2,"response":"0x00ff10"}})
    );
    answer[..32].fill(0xff);
    let refused =
        programs::simulated_program_result(&json!(format!("0x{}", encoding::hex_encode(&answer))))?;
    assert_eq!(refused["outcome"]["terminal_result"], -1);
    answer[159] = 1;
    assert!(programs::decode_call_outcome(&answer).is_err());
    assert!(programs::simulated_program_result(&json!({"outcome":"not ABI"})).is_err());
    Ok(())
}

#[test]
fn program_call_submits_raw_transaction() -> Result<(), String> {
    let vector = vector();
    let native = native_options(&vector);
    let capabilities = [format!(
        "transfer402:{}:{}:7",
        "22".repeat(32),
        "33".repeat(32)
    )];
    let request = programs::EvmCallRequest {
        program_id: &"11".repeat(32),
        calldata: "0061ff10",
        fuel: 1_000_000,
        native: &native,
        fee_limit: "1000",
        capabilities: &capabilities,
    };
    let data = programs::call_calldata(&request, [0x54, 0xbd, 0x2b, 0x48])?;
    assert_eq!(
        format!("0x{}", encoding::hex_encode(&data)),
        vector["evm"]["calldata"]
    );
    let account = layerx_crypto::account_derivation::derive_from_mnemonic(
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", "", 0
    ).map_err(|error| error.to_string())?;
    let transaction = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 200,
        gas_limit: 300_000,
        to: encoding::fixed_hex("programs", &ADDRESS[2..])?,
        data,
    };
    let raw = programs::sign_program_transaction(
        &transaction,
        account.evm_secret().ok_or("missing EVM secret")?,
        &account.evm_address(),
    )?;
    assert_eq!(raw[0], 2);
    let (decoded, signature) = decode_signed(&raw).map_err(|error| error.to_string())?;
    assert_eq!(decoded, transaction);
    assert_eq!(
        recover_sender(&decoded, &signature).map_err(|error| error.to_string())?,
        account.evm_address()
    );
    assert!(programs::sign_program_transaction(
        &transaction,
        account.evm_secret().ok_or("missing EVM secret")?,
        &[0; 20]
    )
    .is_err());
    let (endpoint, recorded) = capture();
    let client = http::Client::new(&endpoint, None)?;
    assert!(programs::submit_program_transaction(&client, &raw).is_err());
    let request = recorded.join().map_err(|_| "capture thread failed")?;
    assert_eq!(request["method"], "eth_sendRawTransaction");
    assert_eq!(
        request["params"],
        json!([format!("0x{}", encoding::hex_encode(&raw))])
    );
    let hash = format!("0x{}", encoding::hex_encode(&transaction_hash(&raw)));
    let rendered = programs::submitted_program_result(&raw, &json!(hash))?;
    assert_eq!(rendered, json!({"tx_hash":hash,"rung":"submitted"}));
    assert!(
        programs::submitted_program_result(&raw, &json!(format!("0x{}", "00".repeat(32)))).is_err()
    );
    Ok(())
}

#[test]
fn receipt_get_accepts_transaction_hash() -> Result<(), String> {
    let directory = tempfile::tempdir().map_err(|error| error.to_string())?;
    let hash = format!("0x{}", "ab".repeat(32));
    let (endpoint, recorded) = capture();
    let result = cli(
        directory.path(),
        &[
            "--json",
            "--rpc",
            &format!("{endpoint}/rpc"),
            "receipt",
            "get",
            &hash,
        ],
    );
    assert!(
        !result.status.success(),
        "the capture does not serve node responses"
    );
    let request = recorded.join().map_err(|_| "capture thread failed")?;
    assert_eq!(request["method"], "pax_sequencerReceipt");
    assert_eq!(request["params"], json!([hash]));
    for invalid in [
        "ab".repeat(32),
        format!("0x{}", "ab".repeat(31)),
        format!("0x{}", "ab".repeat(33)),
        format!("0x{}", "gg".repeat(32)),
        format!("0x{}", "+a".repeat(32)),
        "0x".to_owned(),
    ] {
        assert!(programs::transaction_hash(&invalid).is_err());
        let refused = cli(
            directory.path(),
            &[
                "--json",
                "--rpc",
                "http://127.0.0.1:1/rpc",
                "receipt",
                "get",
                &invalid,
            ],
        );
        assert!(!refused.status.success());
        let error: Value =
            serde_json::from_slice(&refused.stderr).map_err(|error| error.to_string())?;
        assert!(error["error"]["detail"]
            .as_str()
            .ok_or("missing error")?
            .contains("transaction hash"));
    }
    Ok(())
}
