include!("../src/main.rs");

mod px_reads {
    use std::io::{BufRead as _, Read as _};
    use std::net::{Shutdown, TcpListener};
    use std::path::Path;
    use std::process::{Command, Output};
    use std::time::{Duration, Instant};

    use serde_json::{json, Value};

    pub fn capture() -> (String, std::thread::JoinHandle<Value>) {
        let listener = TcpListener::bind("127.0.0.1:0").expect("wire capture listener");
        let endpoint = format!("http://{}/rpc", listener.local_addr().expect("address"));
        listener.set_nonblocking(true).expect("nonblocking capture");
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
                .expect("read timeout");
            let mut reader = std::io::BufReader::new(stream);
            let mut line = String::new();
            reader.read_line(&mut line).expect("request line");
            assert_eq!(line, "POST /rpc HTTP/1.1\r\n");
            let mut length = None;
            loop {
                line.clear();
                assert_ne!(reader.read_line(&mut line).expect("header"), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        length = Some(value.trim().parse::<usize>().expect("content length"));
                    }
                }
            }
            let length = length.expect("content-length header");
            assert!(length <= 1_048_576);
            let mut body = vec![0; length];
            reader.read_exact(&mut body).expect("request body");
            reader
                .get_ref()
                .shutdown(Shutdown::Both)
                .expect("close capture without a node reply");
            let request: Value = serde_json::from_slice(&body).expect("JSON-RPC request");
            assert_eq!(request["jsonrpc"], "2.0");
            assert!(!request["method"]
                .as_str()
                .expect("method")
                .starts_with("lx_"));
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
            .expect("CLI process")
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
        .expect("isolated configuration");
    }

    pub fn receipt() -> Value {
        let corpus: Value = serde_json::from_str(include_str!(
            "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
        ))
        .expect("recorded node conformance receipts");
        corpus["cases"][0]["receiptJson"].clone()
    }

    pub fn policy() -> layerx_proof::inclusion::SequencerAuthorization {
        layerx_proof::inclusion::SequencerAuthorization::new(
            [0x6b; 32],
            crate::encoding::fixed_hex(
                "recorded signer",
                "8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
            )
            .expect("signer"),
            1000,
            1000,
        )
    }
}

#[test]
fn wallet_balance_reads_px_get_balances() {
    let directory = tempfile::tempdir().expect("configuration directory");
    for explicit_rpc in [true, false] {
        for asset in [None, Some("22".repeat(32))] {
            let (endpoint, captured) = px_reads::capture();
            px_reads::configure(directory.path(), &endpoint);
            let mut args = Vec::new();
            if explicit_rpc {
                args.extend(["--rpc", endpoint.as_str()]);
            }
            args.extend(["wallet", "balance", "--did", "did:layerx:alice"]);
            if let Some(asset) = &asset {
                args.extend(["--asset", asset.as_str()]);
            }
            let output = px_reads::cli(directory.path(), &args);
            assert!(
                !output.status.success(),
                "wire capture supplies no node response"
            );
            let request = captured.join().expect("wire capture");
            assert_eq!(request["method"], "px_getBalances");
            assert_eq!(request["params"], json!(["did:layerx:alice"]));
        }
    }
}

#[test]
fn token_list_reads_px_list_assets() {
    let directory = tempfile::tempdir().expect("configuration directory");
    let cursor = "11".repeat(32);
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
            let (endpoint, captured) = px_reads::capture();
            px_reads::configure(directory.path(), &endpoint);
            let mut arguments = Vec::new();
            if explicit_rpc {
                arguments.extend(["--rpc", endpoint.as_str()]);
            }
            arguments.extend(args.iter().copied());
            let output = px_reads::cli(directory.path(), &arguments);
            assert!(
                !output.status.success(),
                "wire capture supplies no node response"
            );
            let request = captured.join().expect("wire capture");
            assert_eq!(request["method"], "px_listAssets");
            assert_eq!(request["params"], params);
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
    assert!(wallet::verified_receipted_status(&receipt, [0x44; 32], 0, &policy).is_err());
    assert!(wallet::verified_receipted_status(&receipt, hash, 1, &policy).is_err());
    let unpinned =
        layerx_proof::inclusion::SequencerAuthorization::new([0x6b; 32], [0x42; 32], 1000, 1000);
    assert!(wallet::verified_receipted_status(&receipt, hash, 0, &unpinned).is_err());
    let expired = layerx_proof::inclusion::SequencerAuthorization::new(
        [0x6b; 32],
        policy.public_key(),
        999,
        999,
    );
    assert!(wallet::verified_receipted_status(&receipt, hash, 0, &expired).is_err());
    for field in [
        "nonce",
        "height",
        "status",
        "gasUsed",
        "returnData",
        "signingKey",
    ] {
        let mut altered = receipt.clone();
        altered[field] = json!(if field == "status" { "0x0" } else { "0x1" });
        assert!(
            wallet::verified_receipted_status(&altered, hash, 0, &policy).is_err(),
            "accepted altered {field}"
        );
    }
    for invalid in [
        "ab".repeat(32),
        format!("0x{}", "ab".repeat(31)),
        format!("0x{}", "gg".repeat(32)),
    ] {
        let output = px_reads::cli(
            directory.path(),
            &[
                "--rpc",
                "http://127.0.0.1:1/rpc",
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
