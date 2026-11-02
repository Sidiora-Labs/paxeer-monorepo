use std::io::{BufRead as _, Read as _, Write as _};
use std::net::TcpListener;
use std::path::Path;
use std::process::{Command, Output};
use std::time::{Duration, Instant};

use layerx_client::chains::{ChainEvent, CHAINS_PRECOMPILE};
use layerx_crypto::evm_transaction::transaction_hash;
use serde_json::{json, Value};

const VECTORS: &str = include_str!("../../../../tests/vectors/chains/abi-vectors.json");
const FROM: &str = "0x7e5f4552091a69125d5dfcb7b8c2659029395bdf";

fn hex(bytes: &[u8]) -> String {
    let mut text = String::from("0x");
    for byte in bytes {
        use std::fmt::Write as _;
        write!(text, "{byte:02x}").unwrap_or_else(|error| panic!("{error}"));
    }
    text
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("missing hex prefix"));
    assert!(text.len().is_multiple_of(2));
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("{error}")),
                16,
            )
            .unwrap_or_else(|error| panic!("{error}"))
        })
        .collect()
}

fn calldata(vectors: &Value, name: &str) -> Value {
    vectors["methods"]
        .as_array()
        .unwrap_or_else(|| panic!("missing methods"))
        .iter()
        .find(|method| method["name"] == name)
        .unwrap_or_else(|| panic!("missing {name}"))["calldata"]
        .clone()
}

fn eth_call(vectors: &Value, name: &str) -> Value {
    json!([{"to":vectors["address"], "data":calldata(vectors, name)}, "pending"])
}

fn rlp_item<'a>(input: &mut &'a [u8]) -> (bool, &'a [u8]) {
    let first = input[0];
    if first < 0x80 {
        let result = &input[..1];
        *input = &input[1..];
        return (false, result);
    }
    let list = first >= 0xc0;
    let short = first - if list { 0xc0 } else { 0x80 };
    let (start, length) = if short < 56 {
        (1, usize::from(short))
    } else {
        let width = usize::from(short - 55);
        assert_ne!(input[1], 0);
        let length = input[1..=width]
            .iter()
            .fold(0_usize, |n, byte| n * 256 + usize::from(*byte));
        assert!(length >= 56);
        (width + 1, length)
    };
    let result = &input[start..start + length];
    *input = &input[start + length..];
    if !list && result.len() == 1 {
        assert!(result[0] >= 0x80);
    }
    (list, result)
}

fn list_envelope(payload: &[u8]) -> Vec<u8> {
    let length = payload.len().to_be_bytes();
    let start = length
        .iter()
        .position(|byte| *byte != 0)
        .unwrap_or(length.len());
    let mut result = if payload.len() < 56 {
        vec![0xc0 + u8::try_from(payload.len()).unwrap_or_else(|error| panic!("{error}"))]
    } else {
        let mut prefix = vec![
            0xf7 + u8::try_from(length.len() - start).unwrap_or_else(|error| panic!("{error}")),
        ];
        prefix.extend_from_slice(&length[start..]);
        prefix
    };
    result.extend_from_slice(payload);
    result
}

fn assert_signed_creation(raw: &[u8], vectors: &Value) {
    assert_eq!(raw[0], 2);
    let mut envelope = &raw[1..];
    let (is_list, mut payload) = rlp_item(&mut envelope);
    assert!(is_list);
    assert!(envelope.is_empty());
    let full_payload = payload;
    let mut fields = Vec::new();
    for index in 0..12 {
        let (is_list, field) = rlp_item(&mut payload);
        assert_eq!(is_list, index == 8);
        fields.push(field);
    }
    assert!(payload.is_empty());
    assert_eq!(fields[0], &[125]);
    assert_eq!(fields[1], &[7]);
    assert_eq!(fields[2], &[2]);
    assert_eq!(fields[3], &[200]);
    assert_eq!(fields[4], &[0x04, 0x93, 0xe0]);
    assert_eq!(fields[5], CHAINS_PRECOMPILE);
    assert_eq!(fields[6], unhex("0x0579a814e10a740000"));
    assert_eq!(
        fields[7],
        unhex(
            calldata(vectors, "createChain")
                .as_str()
                .unwrap_or_else(|| panic!("missing calldata"))
        )
    );
    assert!(fields[8].is_empty());
    assert!(fields[9].is_empty() || fields[9] == [1]);
    let mut signature = [0; 64];
    assert!(!fields[10].is_empty() && fields[10].len() <= 32 && fields[10][0] != 0);
    assert!(!fields[11].is_empty() && fields[11].len() <= 32 && fields[11][0] != 0);
    signature[32 - fields[10].len()..32].copy_from_slice(fields[10]);
    signature[64 - fields[11].len()..64].copy_from_slice(fields[11]);
    // The first nine RLP fields are the actual signed transaction preimage.
    let mut remaining = full_payload;
    for _ in 0..9 {
        rlp_item(&mut remaining);
    }
    let mut unsigned = vec![2];
    unsigned.extend_from_slice(&list_envelope(
        &full_payload[..full_payload.len() - remaining.len()],
    ));
    let public_key = unhex(concat!(
        "0x04",
        "79be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
        "483ada7726a3c4655da4fbfc0e1108a8fd17b448a68554199c47d08ffb10d4b8"
    ));
    let parity = if fields[9].is_empty() {
        0
    } else {
        fields[9][0]
    };
    layerx_crypto::secp256k1::verify_recoverable_digest(
        &public_key,
        &signature,
        27 + parity,
        &transaction_hash(&unsigned),
    )
    .unwrap_or_else(|error| panic!("invalid payable transaction signature: {error:?}"));
    assert_eq!(
        hex(&layerx_crypto::secp256k1::evm_address(&public_key)
            .unwrap_or_else(|error| panic!("{error:?}"))),
        FROM
    );
}

enum Reply {
    Result(Value),
    Submit,
    Receipt,
}

struct Step {
    method: &'static str,
    params: Value,
    reply: Reply,
}

fn server(steps: Vec<Step>, vectors: Value) -> (String, std::thread::JoinHandle<()>) {
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
        let mut hash = None;
        for step in steps {
            let deadline = Instant::now() + Duration::from_secs(10);
            let socket = loop {
                match listener.accept() {
                    Ok((socket, _)) => break socket,
                    Err(error)
                        if error.kind() == std::io::ErrorKind::WouldBlock
                            && Instant::now() < deadline =>
                    {
                        std::thread::sleep(Duration::from_millis(5))
                    }
                    Err(error) => panic!("missing {} request: {error}", step.method),
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
            assert!(length < 65_536);
            let mut body = vec![0; length];
            reader
                .read_exact(&mut body)
                .unwrap_or_else(|error| panic!("{error}"));
            let request: Value =
                serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{error}"));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["id"], 1);
            assert_eq!(request["method"], step.method);
            let result = match step.reply {
                Reply::Result(result) => {
                    assert_eq!(request["params"], step.params);
                    result
                }
                Reply::Submit => {
                    assert_eq!(request["params"].as_array().map(Vec::len), Some(1));
                    let raw = unhex(
                        request["params"][0]
                            .as_str()
                            .unwrap_or_else(|| panic!("missing raw transaction")),
                    );
                    assert_signed_creation(&raw, &vectors);
                    hash = Some(hex(&transaction_hash(&raw)));
                    json!(hash)
                }
                Reply::Receipt => {
                    let hash = hash
                        .as_ref()
                        .unwrap_or_else(|| panic!("no submitted transaction"));
                    assert_eq!(request["params"], json!([hash]));
                    let operator = unhex(FROM)
                        .try_into()
                        .unwrap_or_else(|_| panic!("invalid operator"));
                    let event = ChainEvent::Created {
                        id: 1,
                        name: "demo".into(),
                        operator,
                        chain_id: 125_000_001,
                    };
                    let (topics, data) = event.encode();
                    assert_eq!(hex(&topics[0]), vectors["events"][0]["topic0"]);
                    json!({"transactionHash":hash, "status":"0x1", "logs":[
                        {"address":vectors["address"], "topics":topics.iter().map(|topic| hex(topic)).collect::<Vec<_>>(), "data":hex(&data)}
                    ]})
                }
            };
            let response = serde_json::to_vec(&json!({"jsonrpc":"2.0", "id":1, "result":result}))
                .unwrap_or_else(|error| panic!("{error}"));
            let socket = reader.get_mut();
            write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()).unwrap_or_else(|error| panic!("{error}"));
            socket
                .write_all(&response)
                .unwrap_or_else(|error| panic!("{error}"));
        }
    });
    (endpoint, worker)
}

fn cli(directory: &Path, args: &[&str]) -> Output {
    Command::new(env!("CARGO_BIN_EXE_layerx"))
        .arg("--json")
        .args(args)
        .env("LAYERX_CONFIG", directory.join("config.json"))
        .env("LAYERX_CREDENTIAL_STORE", "file")
        .env(
            "LAYERX_CREDENTIAL_PASSPHRASE",
            "chains command transport test",
        )
        .env("LAYERX_EVM_KEY_FILE", directory.join("wallet.json"))
        .output()
        .unwrap_or_else(|error| panic!("{error}"))
}

fn success(output: Output) -> Value {
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let value: Value =
        serde_json::from_slice(&output.stdout).unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(value["ok"], true);
    value
}

#[test]
fn chains_list_show_and_create_round_trip() -> Result<(), Box<dyn std::error::Error>> {
    use std::os::unix::fs::OpenOptionsExt as _;
    let directory = tempfile::tempdir()?;
    let vectors: Value = serde_json::from_str(VECTORS)?;
    let one = json!(format!("0x{:064x}", 1));
    let mut steps = Vec::new();
    for method in [
        "chainCount",
        "getChain",
        "getChain",
        "chainByName",
        "getChain",
        "params",
    ] {
        let result = match method {
            "chainCount" | "chainByName" => one.clone(),
            "getChain" => vectors["getChain"]["returndata"].clone(),
            _ => vectors["params"]["returndata"].clone(),
        };
        steps.push(Step {
            method: "eth_call",
            params: eth_call(&vectors, method),
            reply: Reply::Result(result),
        });
    }
    for (method, params, result) in [
        ("eth_chainId", json!([]), json!("0x7d")),
        (
            "eth_getTransactionCount",
            json!([FROM, "pending"]),
            json!("0x7"),
        ),
        ("eth_gasPrice", json!([]), json!("0x64")),
        ("eth_maxPriorityFeePerGas", json!([]), json!("0x2")),
        (
            "eth_estimateGas",
            json!([{"from":FROM, "to":vectors["address"], "data":calldata(&vectors, "createChain"), "value":"0x579a814e10a740000"}, "pending"]),
            json!("0x493e0"),
        ),
    ] {
        steps.push(Step {
            method,
            params,
            reply: Reply::Result(result),
        });
    }
    steps.push(Step {
        method: "eth_sendRawTransaction",
        params: Value::Null,
        reply: Reply::Submit,
    });
    steps.push(Step {
        method: "eth_getTransactionReceipt",
        params: Value::Null,
        reply: Reply::Receipt,
    });
    steps.push(Step {
        method: "eth_call",
        params: eth_call(&vectors, "chainByName"),
        reply: Reply::Result(json!(format!("0x{:064x}", 0))),
    });
    steps.push(Step {
        method: "eth_call",
        params: eth_call(&vectors, "getChain"),
        reply: Reply::Result(json!("0x00")),
    });
    steps.push(Step {
        method: "eth_call",
        params: eth_call(&vectors, "chainCount"),
        reply: Reply::Result(json!(format!("0x{:064x}", 0))),
    });
    let (endpoint, worker) = server(steps, vectors.clone());
    std::fs::write(
        directory.path().join("config.json"),
        serde_json::to_vec(&json!({
            "version":1, "current_environment":"emulator", "default_key":null, "keys":{},
            "environments":{"emulator":{"endpoint":endpoint, "network_id":125}}
        }))?,
    )?;
    let mut key_file = std::fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(directory.path().join("wallet.json"))?;
    serde_json::to_writer(
        &mut key_file,
        &json!({"evm_address":FROM, "evm_private_key":format!("{:064x}", 1)}),
    )?;
    drop(key_file);
    let rpc = format!("{endpoint}/rpc");
    let mut expected = vectors["getChain"]["decoded"].clone();
    expected["rpcUrl"] = json!(format!("{endpoint}/chain/1"));
    let listed = success(cli(directory.path(), &["--rpc", &rpc, "chains", "list"]));
    assert_eq!(listed["kind"], "chains.list");
    assert_eq!(listed["data"]["chains"], json!([expected]));
    let shown = success(cli(directory.path(), &["chains", "show", "1"]));
    assert_eq!(shown["kind"], "chains.show");
    assert_eq!(shown["data"], expected);
    assert_eq!(
        success(cli(
            directory.path(),
            &["--rpc", &rpc, "chains", "show", "demo"]
        ))["data"],
        expected
    );
    let created = success(cli(
        directory.path(),
        &[
            "--rpc",
            &rpc,
            "chains",
            "create",
            "--name",
            "demo",
            "--tier",
            "starter",
            "--fee-share-bps",
            "100",
        ],
    ));
    assert_eq!(created["kind"], "chains.created");
    assert_eq!(created["data"]["id"], 1);
    assert_eq!(created["data"]["chainId"], 125_000_001);
    assert_eq!(created["data"]["name"], "demo");
    assert_eq!(created["data"]["rpcUrl"], format!("{endpoint}/chain/1"));
    assert_eq!(
        created["data"]["transactionHash"].as_str().map(str::len),
        Some(66)
    );
    let missing = cli(directory.path(), &["chains", "show", "demo"]);
    assert!(!missing.status.success());
    let error: Value = serde_json::from_slice(&missing.stderr)?;
    assert_eq!(error["error"]["code"], "chain_not_found");
    let malformed = cli(directory.path(), &["chains", "show", "1"]);
    assert!(!malformed.status.success());
    assert!(
        String::from_utf8_lossy(&malformed.stderr).contains("invalid chain registry ABI encoding")
    );
    assert_eq!(
        success(cli(directory.path(), &["chains", "list"]))["data"]["chains"],
        json!([])
    );
    worker.join().map_err(|_| "RPC server thread failed")?;
    for (name, tier, fee) in [
        ("ab", "starter", "100"),
        ("demo", "unknown", "100"),
        ("demo", "starter", "5001"),
    ] {
        assert!(!cli(
            directory.path(),
            &[
                "chains",
                "create",
                "--name",
                name,
                "--tier",
                tier,
                "--fee-share-bps",
                fee
            ]
        )
        .status
        .success());
    }
    Ok(())
}
