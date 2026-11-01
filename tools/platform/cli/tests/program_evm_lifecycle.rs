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
const DEPLOY: &str =
    include_str!("../../../../sdk/platform/conformance/fixtures/native-program-deploy-v4.json");
const UPGRADE: &str =
    include_str!("../../../../sdk/platform/conformance/fixtures/native-program-upgrade-v4.json");
const WIND_DOWN: [&str; 4] = [
    include_str!(
        "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-route-v4.json"
    ),
    include_str!(
        "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-deprecate-v4.json"
    ),
    include_str!(
        "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-tombstone-v4.json"
    ),
    include_str!(
        "../../../../sdk/platform/conformance/fixtures/native-program-wind-down-exit-v4.json"
    ),
];

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

fn vector(source: &str) -> Result<(Value, Vec<u8>), String> {
    let value: Value = serde_json::from_str(source).map_err(|error| error.to_string())?;
    let payload = encoding::hex_decode(
        "payload",
        value["payload_hex"].as_str().ok_or("missing payload")?,
    )?;
    Ok((value, payload))
}

fn account() -> Result<layerx_crypto::account_derivation::DerivedAccount, String> {
    layerx_crypto::account_derivation::derive_from_mnemonic(
        "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about", "", 0
    ).map_err(|error| error.to_string())
}

fn submit_calldata(data: Vec<u8>) -> Result<String, String> {
    let account = account()?;
    let transaction = Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 200,
        gas_limit: 1_000_000,
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
    let (endpoint, recorded) = capture();
    let client = http::Client::new(&endpoint, None)?;
    assert!(
        programs::submit_lifecycle_transaction(&client, &raw).is_err(),
        "capture never supplies a node response"
    );
    let request = recorded.join().map_err(|_| "capture thread failed")?;
    assert_eq!(request["method"], "eth_sendRawTransaction");
    assert_eq!(
        request["params"],
        json!([format!("0x{}", encoding::hex_encode(&raw))])
    );
    let sent = encoding::hex_decode(
        "raw transaction",
        request["params"][0]
            .as_str()
            .ok_or("missing raw transaction")?
            .strip_prefix("0x")
            .ok_or("missing hex prefix")?,
    )?;
    let (sent, _) = decode_signed(&sent).map_err(|error| error.to_string())?;
    assert_eq!(sent.to, encoding::fixed_hex("programs", &ADDRESS[2..])?);
    assert_eq!(sent.data, transaction.data);
    Ok(format!(
        "0x{}",
        encoding::hex_encode(&transaction_hash(&raw))
    ))
}

fn cli(directory: &std::path::Path, arguments: &[&str]) -> Output {
    Command::new(env!("CARGO_BIN_EXE_layerx"))
        .args(arguments)
        .env("LAYERX_CONFIG", directory.join("config.json"))
        .env("LAYERX_CREDENTIAL_STORE", "file")
        .env(
            "LAYERX_CREDENTIAL_PASSPHRASE",
            "program lifecycle transport test",
        )
        .output()
        .unwrap_or_else(|error| panic!("{error}"))
}

fn assert_cli_rpc(directory: &std::path::Path, operation: &[&str]) -> Result<(), String> {
    let account = account()?;
    let key_file = directory.join("keys.json");
    let mut options = std::fs::OpenOptions::new();
    options.write(true).create_new(true);
    #[cfg(unix)]
    {
        use std::os::unix::fs::OpenOptionsExt as _;
        options.mode(0o600);
    }
    use std::io::Write as _;
    let mut file = options.open(&key_file).map_err(|error| error.to_string())?;
    file.write_all(serde_json::to_string(&json!({
        "evm_address":format!("0x{}", encoding::hex_encode(&account.evm_address())),
        "evm_private_key":encoding::hex_encode(account.evm_secret().ok_or("missing EVM secret")?),
    })).map_err(|error| error.to_string())?.as_bytes()).map_err(|error| error.to_string())?;
    let (endpoint, recorded) = capture();
    let url = format!("{endpoint}/rpc");
    let program = "11".repeat(32);
    let key_path = key_file.to_str().ok_or("key path is not UTF-8")?;
    let mut args = vec!["--json", "--rpc", &url, "program"];
    args.extend_from_slice(operation);
    args.extend(["--program-id", &program, "--evm-key-file", key_path]);
    let result = cli(directory, &args);
    assert!(
        !result.status.success(),
        "capture never supplies a node response"
    );
    let request = recorded.join().map_err(|_| {
        format!(
            "CLI did not reach EVM RPC: {}",
            String::from_utf8_lossy(&result.stderr)
        )
    })?;
    assert_eq!(request["method"], "eth_chainId");
    assert_eq!(request["params"], json!([]));
    Ok(())
}

#[test]
fn program_deploy_calldata_matches_vector() -> Result<(), String> {
    let (expected, payload) = vector(DEPLOY)?;
    let data = programs::lifecycle_calldata(1, &payload, "0")?;
    assert_eq!(
        format!("0x{}", encoding::hex_encode(&data)),
        expected["evm"]["calldata"]
    );
    let hash = submit_calldata(data)?;
    let (endpoint, recorded) = capture();
    let client = http::Client::new(&endpoint, None)?;
    assert!(programs::sequencer_receipt(&client, &hash).is_err());
    let request = recorded
        .join()
        .map_err(|_| "receipt capture thread failed")?;
    assert_eq!(request["method"], "pax_sequencerReceipt");
    assert_eq!(request["params"], json!([hash]));
    for rung in ["submitted", "receipted", "committed"] {
        let status = json!({"txHash":hash,"rung":rung});
        let rendered = programs::lifecycle_receipt_result(&hash, &status)?;
        assert_eq!(rendered["txHash"], hash);
        assert_eq!(rendered["rung"], rung);
        assert_eq!(rendered["sequencer_receipt"], status);
    }
    assert!(programs::lifecycle_receipt_result(&hash, &Value::Null).is_err());
    assert!(
        programs::lifecycle_receipt_result(&hash, &json!({"txHash":hash,"rung":"unknown"}))
            .is_err()
    );
    assert!(programs::lifecycle_receipt_result(
        &hash,
        &json!({"txHash":format!("0x{}", "ff".repeat(32)),"rung":"committed"})
    )
    .is_err());
    let deployment = layerx_types::program_lifecycle::NativeProgramDeploy::decode(&payload)
        .map_err(|error| format!("{error:?}"))?;
    assert_eq!(
        deployment.encode().map_err(|error| format!("{error:?}"))?,
        payload
    );
    let directory = tempfile::tempdir().map_err(|error| error.to_string())?;
    let artifact = directory.path().join("program.wasm");
    let interface = directory.path().join("interface.bin");
    std::fs::write(&artifact, deployment.wasm).map_err(|error| error.to_string())?;
    std::fs::write(
        &interface,
        deployment.interface.ok_or("fixture omitted interface")?,
    )
    .map_err(|error| error.to_string())?;
    let authority = "22".repeat(32);
    assert_cli_rpc(
        directory.path(),
        &[
            "deploy",
            artifact.to_str().ok_or("artifact path")?,
            "--upgrade-authority",
            &authority,
            "--interface",
            interface.to_str().ok_or("interface path")?,
        ],
    )?;

    let (expected, payload) = vector(UPGRADE)?;
    let data = programs::lifecycle_calldata(2, &payload, "1000")?;
    assert_eq!(
        format!("0x{}", encoding::hex_encode(&data)),
        expected["evm"]["calldata"]
    );
    submit_calldata(data)?;
    let upgraded = layerx_types::program_lifecycle::NativeProgramUpgrade::decode(&payload)
        .map_err(|error| format!("{error:?}"))?;
    let directory = tempfile::tempdir().map_err(|error| error.to_string())?;
    let artifact = directory.path().join("upgrade.wasm");
    std::fs::write(&artifact, upgraded.wasm).map_err(|error| error.to_string())?;
    let old_hash = encoding::hex_encode(&upgraded.old_hash);
    assert_cli_rpc(
        directory.path(),
        &[
            "upgrade",
            artifact.to_str().ok_or("artifact path")?,
            "--old-hash",
            &old_hash,
            "--fee-limit",
            "1000",
        ],
    )?;
    let fee = programs::lifecycle_calldata(2, &payload, &u128::MAX.to_string())?;
    assert_eq!(&fee[4 + 8 * 32 + 16..4 + 9 * 32], &u128::MAX.to_be_bytes());
    assert!(programs::lifecycle_calldata(2, &payload, "-1").is_err());
    assert!(programs::lifecycle_calldata(1, &payload, "0").is_err());
    Ok(())
}

#[test]
fn program_wind_down_modes_use_layerxprograms() -> Result<(), String> {
    let account = "33".repeat(32);
    let asset = "44".repeat(32);
    let destination = "55".repeat(32);
    let exit_program = "66".repeat(32);
    let commands: [&[&str]; 4] = [
        &[
            "wind-down",
            "route",
            "--account",
            &account,
            "--asset",
            &asset,
            "--destination",
            &destination,
            "--seed",
            "73656564",
        ],
        &[
            "wind-down",
            "deprecate",
            "--exit-program",
            &exit_program,
            "--deadline-batch",
            "42",
        ],
        &["wind-down", "tombstone"],
        &["wind-down", "exit", "--account", &account],
    ];
    for (source, command) in WIND_DOWN.into_iter().zip(commands) {
        let (expected, payload) = vector(source)?;
        let operation = layerx_types::program_lifecycle::NativeProgramWindDown::decode(&payload)
            .map_err(|error| format!("{error:?}"))?;
        assert_eq!(
            operation.encode().map_err(|error| format!("{error:?}"))?,
            payload
        );
        let data = programs::lifecycle_calldata(7, &payload, "0")?;
        assert_eq!(
            format!("0x{}", encoding::hex_encode(&data)),
            expected["evm"]["calldata"]
        );
        submit_calldata(data)?;
        let directory = tempfile::tempdir().map_err(|error| error.to_string())?;
        assert_cli_rpc(directory.path(), command)?;
    }
    let bounded = layerx_types::program_lifecycle::NativeProgramWindDown {
        program_id: layerx_types::intent::ProgramId::new([0x11; 32]),
        operation: layerx_types::program_lifecycle::ProgramWindDownOperation::BoundedExit {
            account: [0x33; 32],
            maximum_exit_amount: u128::MAX,
        },
    }
    .encode()
    .map_err(|error| format!("{error:?}"))?;
    let data = programs::lifecycle_calldata(7, &bounded, "0")?;
    assert_eq!(&data[..4], &[0xad, 0xe5, 0xd9, 0xe6]);
    assert_eq!(&data[4..36], &[0x11; 32]);
    assert_eq!(&data[36..68], &[0x33; 32]);
    assert_eq!(&data[68..84], &[0; 16]);
    assert_eq!(&data[84..], &u128::MAX.to_be_bytes());
    submit_calldata(data)?;
    assert!(programs::lifecycle_calldata(7, &[0; 32], "0").is_err());
    assert!(programs::lifecycle_calldata(0, &[], "0").is_err());
    Ok(())
}
