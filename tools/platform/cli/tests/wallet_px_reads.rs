include!("../src/main.rs");

mod px_reads {
    use std::io::{BufRead as _, Read as _, Write as _};
    use std::net::TcpListener;
    use std::path::Path;
    use std::process::{Command, Output};
    use std::time::{Duration, Instant};

    use serde_json::{json, Value};

    pub const DID: &str =
        "did:layerx:6161616161616161616161616161616161616161616161616161616161616161";

    pub fn balance_document() -> Value {
        use layerx_platform_cli::wallet_gateway as producer;
        let fixture: Value = serde_json::from_str(include_str!(
            "../../../../services/platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json"
        ))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "committed ABI vectors"));
        let case = |name: &str| {
            fixture["cases"]
                .as_array()
                .unwrap_or_else(|| panic!("{}", "cases"))
                .iter()
                .find(|row| row["name"] == name)
                .unwrap_or_else(|| panic!("{}", "ABI case"))
        };
        let bytes = |text: &str| {
            crate::encoding::hex_decode("ABI fixture", text)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "ABI bytes"))
        };
        let unified = producer::evm::decode_unified_account(&bytes(
            case("addr.getUnifiedAccount")["result"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "unified result")),
        ))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real addr codec"));
        let coins = producer::evm::decode_coins(&bytes(
            case("bank.all_balances")["result"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "bank result")),
        ))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real bank codec"));
        let rows = coins.into_iter().map(|(denom, amount)| json!({"denom":denom,"amount":producer::evm::uint256_decimal(&amount)})).collect();
        let document = producer::balances(
            producer::account_document(
                Some(unified.evm),
                Some(unified.pax_address),
                Some(unified.did_public_key),
                Some(unified.layerx_main_account_id),
            ),
            91,
            rows,
        );
        assert_eq!(
            document["balances"],
            case("bank.all_balances")["out"]["response"]
        );
        document
    }

    pub fn asset_row() -> Value {
        use layerx_platform_cli::wallet_gateway as producer;
        let word = |value: u128| {
            let mut word = vec![0; 16];
            word.extend_from_slice(&value.to_be_bytes());
            word
        };
        let asset = [0x22; 32];
        let mut abi = Vec::new();
        for field in [
            asset.to_vec(),
            word(384),
            word(448),
            word(6),
            word(1_000_000),
            vec![0x61; 32],
            vec![0x44; 32],
            word(1),
            word(0),
            word(512),
            word(0),
            word(1_000_000),
        ] {
            abi.extend(field);
        }
        for text in ["TST", "Test Token"] {
            abi.extend(word(text.len() as u128));
            abi.extend(text.as_bytes());
            abi.resize(abi.len().div_ceil(32) * 32, 0);
        }
        abi.extend(word(0));
        let mut row = json!({"asset_id":producer::hex(&asset),"kind":"ledger","denom":format!("layerx/{}",producer::hex(&asset))});
        row.as_object_mut()
            .unwrap_or_else(|| panic!("{}", "row"))
            .extend(
                producer::ledger_asset_metadata(&abi, asset)
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "real ledger metadata codec"))
                    .as_object()
                    .unwrap_or_else(|| panic!("{}", "metadata"))
                    .clone(),
            );
        row
    }

    pub fn capture() -> (String, std::thread::JoinHandle<Value>) {
        capture_at("/")
    }

    pub fn hosted() -> (String, std::thread::JoinHandle<Value>) {
        capture_at("/rpc")
    }

    fn capture_at(path: &'static str) -> (String, std::thread::JoinHandle<Value>) {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("{}: {error:?}", "wire capture listener"));
        let endpoint = format!(
            "http://{}{}",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "address")),
            path
        );
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "nonblocking capture"));
        let worker = std::thread::spawn(move || {
            let deadline = Instant::now() + Duration::from_secs(10);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "CLI issued no RPC request");
                        std::thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => panic!("capture accept: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap_or_else(|error| panic!("{}: {error:?}", "read timeout"));
            let mut reader = std::io::BufReader::new(stream);
            let mut line = String::new();
            reader
                .read_line(&mut line)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "request line"));
            assert_eq!(line, format!("POST {path} HTTP/1.1\r\n"));
            let mut length = None;
            loop {
                line.clear();
                assert_ne!(
                    reader
                        .read_line(&mut line)
                        .unwrap_or_else(|error| panic!("{}: {error:?}", "header")),
                    0
                );
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        length = Some(
                            value
                                .trim()
                                .parse::<usize>()
                                .unwrap_or_else(|error| panic!("{}: {error:?}", "content length")),
                        );
                    }
                }
            }
            let length = length.unwrap_or_else(|| panic!("{}", "content-length header"));
            assert!(length <= 1_048_576);
            let mut body = vec![0; length];
            reader
                .read_exact(&mut body)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "request body"));
            let request: Value = serde_json::from_slice(&body)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "JSON-RPC request"));
            assert_eq!(request["jsonrpc"], "2.0");
            assert!(!request["method"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "method"))
                .starts_with("lx_"));
            if path == "/" {
                // Root receipt transport is asserted independently of hosted service reads.
                return request;
            }
            let result = match request["method"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "method"))
            {
                "px_getBalances" => balance_document(),
                "px_listAssets" => {
                    let (cursor, limit) =
                        layerx_platform_cli::wallet_gateway::page_params(Some(&request["params"]))
                            .unwrap_or_else(|error| {
                                panic!("{}: {error:?}", "published page parameters")
                            });
                    layerx_platform_cli::wallet_gateway::assets_page(
                        vec![asset_row()],
                        94,
                        cursor,
                        limit,
                        None,
                    )
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "real gateway page producer"))
                }
                "pax_sequencerReceipt" => receipt(),
                method => panic!("unexpected read method {method}"),
            };
            let response = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
            write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", response.len(), response).unwrap_or_else(|error| panic!("{}: {error:?}", "producer response transport"));
            request
        });
        (endpoint, worker)
    }

    pub fn cli(directory: &Path, arguments: &[&str]) -> Output {
        Command::new(env!("CARGO_BIN_EXE_layerx"))
            .args(["--json"])
            .args(arguments)
            .env("LAYERX_CONFIG", directory.join("config.json"))
            .env("LAYERX_CREDENTIAL_STORE", "file")
            .env(
                "LAYERX_CREDENTIAL_PASSPHRASE",
                "wallet px read transport contract",
            )
            .output()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "CLI process"))
    }

    pub fn configure(directory: &Path, endpoint: &str) {
        std::fs::write(
            directory.join("config.json"),
            json!({
                "version":1, "current_environment":"beta", "default_key":null, "keys":{},
                "environments":{"beta":{"endpoint":endpoint,"network_id":125,
                    "sequencer_trust_anchor":"42".repeat(32)}}
            })
            .to_string(),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "isolated configuration"));
    }

    pub fn receipt() -> Value {
        let corpus: Value = serde_json::from_str(include_str!(
            "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "recorded node conformance receipts"));
        corpus["cases"][0]["receiptJson"].clone()
    }

    pub fn policy() -> layerx_proof::inclusion::SequencerAuthorization {
        layerx_proof::inclusion::SequencerAuthorization::new(
            [0x6b; 32],
            crate::encoding::fixed_hex(
                "recorded signer",
                "8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
            )
            .unwrap_or_else(|error| panic!("{}: {error:?}", "signer")),
            1000,
            1000,
        )
    }
}

#[test]
fn wallet_balance_reads_px_get_balances() {
    let directory = tempfile::tempdir()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "configuration directory"));
    for explicit_rpc in [true, false] {
        for asset in [None, Some("22".repeat(32))] {
            let (endpoint, captured) = px_reads::hosted();
            px_reads::configure(directory.path(), &endpoint);
            let mut args = Vec::new();
            if explicit_rpc {
                args.extend(["--rpc", endpoint.as_str()]);
            }
            args.extend(["wallet", "balance", "--did", px_reads::DID]);
            args.extend(["--gateway-rpc", endpoint.as_str()]);
            if let Some(asset) = &asset {
                args.extend(["--asset", asset.as_str()]);
            }
            let output = px_reads::cli(directory.path(), &args);
            assert!(
                output.status.success(),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
            let printed: Value = serde_json::from_slice(&output.stdout)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "CLI public output"));
            assert_eq!(printed["ok"], true);
            let request = captured
                .join()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "wire capture"));
            assert_eq!(request["method"], "px_getBalances");
            assert_eq!(request["params"], json!([px_reads::DID]));
            assert_eq!(printed["kind"], "wallet.balance");
            let mut expected = px_reads::balance_document();
            expected["verification"] = json!("unverified_node_observation");
            if asset.is_some() {
                expected["balances"] = json!([]);
            }
            assert_eq!(printed["data"], expected);
            assert_eq!(printed["data"]["height"], 91);
            assert_eq!(printed["data"]["units"], "base");
            assert_eq!(printed["data"]["evidence"], "committed_eth_call");
        }
    }

    let listener = std::net::TcpListener::bind("127.0.0.1:0")
        .unwrap_or_else(|error| panic!("{}: {error:?}", "unused node listener"));
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "nonblocking listener"));
    let endpoint = format!(
        "http://{}/",
        listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "node address"))
    );
    px_reads::configure(directory.path(), &endpoint);
    let output = px_reads::cli(
        directory.path(),
        &[
            "--rpc",
            &endpoint,
            "wallet",
            "balance",
            "--did",
            px_reads::DID,
        ],
    );
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    let printed: Value = serde_json::from_slice(&output.stderr)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "CLI public refusal"));
    assert_eq!(printed["ok"], false);
    assert_eq!(printed["error"]["code"], "gateway_rpc_required");
    assert_eq!(
        printed["error"]["detail"],
        "gateway_rpc_required: provide --gateway-rpc for hosted px_ balances"
    );
    assert!(matches!(
        listener.accept(),
        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock
    ));
}

#[test]
fn token_list_reads_px_list_assets() {
    let directory = tempfile::tempdir()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "configuration directory"));
    let cursor = "16".to_owned();
    let asset = "22".repeat(32);
    for (args, params) in [
        (vec!["token", "list"], json!([])),
        (vec!["token", "list", "--limit", "2"], json!([null, 2])),
        (
            vec!["token", "list", "--cursor", &cursor, "--limit", "2"],
            json!([cursor, 2]),
        ),
        (vec!["token", "info", &asset], json!([])),
    ] {
        for explicit_rpc in [true, false] {
            let (endpoint, captured) = px_reads::hosted();
            px_reads::configure(directory.path(), &endpoint);
            let mut arguments = Vec::new();
            if explicit_rpc {
                arguments.extend(["--rpc", endpoint.as_str()]);
            }
            arguments.extend(args.iter().copied());
            let output = px_reads::cli(directory.path(), &arguments);
            assert!(
                output.status.success(),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
            let printed: Value = serde_json::from_slice(&output.stdout)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "CLI public output"));
            assert_eq!(printed["ok"], true);
            let request = captured
                .join()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "wire capture"));
            assert_eq!(request["method"], "px_listAssets");
            assert_eq!(request["params"], params);
            assert_eq!(printed["kind"], "token.read");
            if args[1] == "info" {
                assert_eq!(printed["data"]["asset"], px_reads::asset_row());
            } else {
                assert_eq!(printed["data"]["assets"], json!([px_reads::asset_row()]));
            }
            assert_eq!(printed["data"]["height"], 94);
        }
    }
}

#[test]
fn wallet_receipt_reads_sequencer_receipt_by_tx_hash() -> Result<(), String> {
    let receipt = px_reads::receipt();
    let hash_text = receipt["txHash"].as_str().ok_or("recorded hash")?;
    let hash = programs::transaction_hash(hash_text)?;
    let policy = px_reads::policy();
    let (data, _) = wallet::verified_receipted_status(&receipt, hash, 0, &policy)?;
    assert_eq!(data["txHash"], hash_text);
    assert_eq!(data["rung"], "receipted");
    if std::env::var_os("LAYERX_PX_READ_PRINT_RECEIPT").is_some() {
        return wallet::receipted_output(hash, data).emit(true);
    }
    let directory = tempfile::tempdir().map_err(|error| error.to_string())?;
    let policy_path = directory.path().join("policy.json");
    std::fs::write(
        &policy_path,
        json!({
            "protocol_version":2,"network_id":125,"sequencer_id":"6b".repeat(32),
            "sequencer_key":encoding::hex_encode(&policy.public_key()),
            "first_batch":1000,"last_batch":1000,
        })
        .to_string(),
    )
    .map_err(|error| error.to_string())?;
    for explicit_rpc in [true, false] {
        let (endpoint, captured) = px_reads::capture();
        px_reads::configure(directory.path(), &endpoint);
        let mut args = Vec::new();
        if explicit_rpc {
            args.extend(["--rpc", endpoint.as_str()]);
        }
        args.extend([
            "wallet",
            "receipt",
            hash_text,
            "--receipt-policy",
            policy_path.to_str().ok_or("policy path")?,
        ]);
        let output = px_reads::cli(directory.path(), &args);
        assert!(
            !output.status.success(),
            "wire capture supplies no node response"
        );
        let request = captured.join().map_err(|_| "wire capture panicked")?;
        assert_eq!(request["method"], "pax_sequencerReceipt");
        assert_eq!(request["params"], json!([hash_text]));
    }
    let output = std::process::Command::new(std::env::current_exe().map_err(|e| e.to_string())?)
        .args([
            "--exact",
            "wallet_receipt_reads_sequencer_receipt_by_tx_hash",
            "--nocapture",
        ])
        .env("LAYERX_PX_READ_PRINT_RECEIPT", "1")
        .output()
        .map_err(|e| e.to_string())?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let stdout = String::from_utf8(output.stdout).map_err(|e| e.to_string())?;
    let printed = stdout
        .lines()
        .find_map(|line| serde_json::from_str::<Value>(line).ok())
        .ok_or("receipt output missing")?;
    assert_eq!(printed["data"], data);
    assert_eq!(printed["kind"], "wallet.receipt");
    assert!(printed["message"]
        .as_str()
        .ok_or("receipt message")?
        .contains("rung receipted"));
    assert_receipt_refusals(&receipt, hash, &policy, directory.path(), &policy_path)?;
    Ok(())
}

fn assert_receipt_refusals(
    receipt: &Value,
    hash: [u8; 32],
    policy: &layerx_proof::inclusion::SequencerAuthorization,
    directory: &std::path::Path,
    policy_path: &std::path::Path,
) -> Result<(), String> {
    assert!(wallet::verified_receipted_status(receipt, [0x44; 32], 0, policy).is_err());
    assert!(wallet::verified_receipted_status(receipt, hash, 1, policy).is_err());
    let unpinned =
        layerx_proof::inclusion::SequencerAuthorization::new([0x6b; 32], [0x42; 32], 1000, 1000);
    assert!(wallet::verified_receipted_status(receipt, hash, 0, &unpinned).is_err());
    let expired = layerx_proof::inclusion::SequencerAuthorization::new(
        [0x6b; 32],
        policy.public_key(),
        999,
        999,
    );
    assert!(wallet::verified_receipted_status(receipt, hash, 0, &expired).is_err());
    for field in [
        "nonce",
        "height",
        "status",
        "gasUsed",
        "returnData",
        "signingKey",
    ] {
        let mut altered = (*receipt).clone();
        altered[field] = json!(if field == "status" { "0x0" } else { "0x1" });
        assert!(
            wallet::verified_receipted_status(&altered, hash, 0, policy).is_err(),
            "accepted altered {field}"
        );
    }
    for invalid in [
        "ab".repeat(32),
        format!("0x{}", "ab".repeat(31)),
        format!("0x{}", "gg".repeat(32)),
    ] {
        let output = px_reads::cli(
            directory,
            &[
                "--rpc",
                "http://127.0.0.1:1",
                "wallet",
                "receipt",
                &invalid,
                "--receipt-policy",
                policy_path.to_str().ok_or("policy path")?,
            ],
        );
        assert!(!output.status.success());
        assert!(String::from_utf8_lossy(&output.stderr).contains("transaction hash"));
    }
    Ok(())
}
