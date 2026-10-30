use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::process::{Command, Output};
use std::time::{Duration, Instant};

use layerx_platform_cli::rpc::{decode_response, request};
use layerx_platform_cli::wallet_signing::{PreparedPayment, SigningFacts};
use serde_json::{json, Value};

const TRANSACTION: &str =
    "02e37d0701028252089422222222222222222222222222222222222222228084deadbeefc0";

fn transaction_object() -> Value {
    json!({
        "type":"0x2", "chainId":"0x7d", "nonce":"0x7",
        "maxPriorityFeePerGas":"0x1", "maxFeePerGas":"0x2", "gas":"0x5208",
        "to":"0x2222222222222222222222222222222222222222", "value":"0x0",
        "data":"0xdeadbeef", "accessList":[],
        "from":"0x1111111111111111111111111111111111111111",
    })
}

fn command(endpoint: &str, transaction: &str) -> Result<Output, Box<dyn std::error::Error>> {
    command_with_sender(
        endpoint,
        transaction,
        Some("0x1111111111111111111111111111111111111111"),
    )
}

fn command_with_sender(
    endpoint: &str,
    transaction: &str,
    from: Option<&str>,
) -> Result<Output, Box<dyn std::error::Error>> {
    let directory = tempfile::tempdir()?;
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx"));
    command
        .env("LAYERX_CONFIG", directory.path().join("config.json"))
        .args([
            "--json",
            "--rpc",
            endpoint,
            "wallet",
            "estimate-fee",
            transaction,
        ]);
    if let Some(from) = from {
        command.args(["--from", from]);
    }
    Ok(command.output()?)
}

fn quote(
    transaction: &str,
    responses: Vec<Value>,
) -> Result<(Output, Vec<Value>), Box<dyn std::error::Error>> {
    quote_with_sender(
        transaction,
        responses,
        "0x1111111111111111111111111111111111111111",
    )
}

fn quote_with_sender(
    transaction: &str,
    responses: Vec<Value>,
    from: &str,
) -> Result<(Output, Vec<Value>), Box<dyn std::error::Error>> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = format!("http://{}/rpc", listener.local_addr()?);
    listener.set_nonblocking(true)?;
    let server = std::thread::spawn(move || -> Result<Vec<Value>, String> {
        let mut requests = Vec::new();
        for response in responses {
            let deadline = Instant::now() + Duration::from_secs(10);
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        if Instant::now() >= deadline {
                            return Err("timed out waiting for a fee RPC request".into());
                        }
                        std::thread::sleep(Duration::from_millis(5));
                    }
                    Err(error) => return Err(error.to_string()),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .map_err(|e| e.to_string())?;
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .map_err(|e| e.to_string())?;
            let mut reader = BufReader::new(&stream);
            let mut line = String::new();
            reader.read_line(&mut line).map_err(|e| e.to_string())?;
            assert_eq!(line, "POST /rpc HTTP/1.1\r\n");
            let mut length = None;
            loop {
                line.clear();
                if reader.read_line(&mut line).map_err(|e| e.to_string())? == 0 {
                    return Err("truncated HTTP headers".into());
                }
                if line == "\r\n" {
                    break;
                }
                if let Some((name, value)) = line.split_once(':') {
                    if name.eq_ignore_ascii_case("content-length") {
                        length = Some(value.trim().parse::<usize>().map_err(|e| e.to_string())?);
                    }
                }
            }
            let length = length
                .filter(|length| *length <= 1_048_576)
                .ok_or("invalid content length")?;
            let mut body = vec![0; length];
            reader.read_exact(&mut body).map_err(|e| e.to_string())?;
            requests.push(serde_json::from_slice(&body).map_err(|e| e.to_string())?);
            let body = response.to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body)
                .map_err(|e| e.to_string())?;
        }
        Ok(requests)
    });
    let output = command_with_sender(&endpoint, transaction, Some(from))?;
    let requests = server.join().map_err(|_| "RPC server panicked")??;
    Ok((output, requests))
}

fn result(value: &str) -> Value {
    json!({"jsonrpc":"2.0", "id":1, "result":value})
}

#[test]
fn estimate_fee_sends_the_intended_sender() -> Result<(), Box<dyn std::error::Error>> {
    let from = "0x5aAeb6053F3E94C9b9A09f33669435E7Ef1BeAed";
    let (output, requests) =
        quote_with_sender(TRANSACTION, vec![result("0x5208"), result("0x2")], from)?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let mut expected = transaction_object();
    expected["from"] = json!("0x5aaeb6053f3e94c9b9a09f33669435e7ef1beaed");
    assert_eq!(
        requests,
        vec![
            json!({"jsonrpc":"2.0", "id":1, "method":"eth_estimateGas", "params":[expected, "pending"]}),
            json!({"jsonrpc":"2.0", "id":1, "method":"eth_gasPrice", "params":[]}),
        ]
    );

    let listener = TcpListener::bind("127.0.0.1:0")?;
    listener.set_nonblocking(true)?;
    let endpoint = format!("http://{}/rpc", listener.local_addr()?);
    for from in [
        None,
        Some("0x11"),
        Some("0xgggggggggggggggggggggggggggggggggggggggg"),
    ] {
        let output = command_with_sender(&endpoint, TRANSACTION, from)?;
        assert!(!output.status.success(), "accepted sender {from:?}");
        if from.is_some() {
            let error: Value = serde_json::from_slice(&output.stderr)?;
            assert_eq!(error["error"]["code"], "invalid_transaction");
        } else {
            assert!(String::from_utf8_lossy(&output.stderr).contains("--from"));
        }
        match listener.accept() {
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
            connection => panic!("invalid sender contacted the endpoint: {connection:?}"),
        }
    }
    Ok(())
}

#[test]
fn estimate_fee_multiplies_gas_by_gas_price() -> Result<(), Box<dyn std::error::Error>> {
    for transaction in [TRANSACTION.to_owned(), format!("0x{TRANSACTION}")] {
        let (output, requests) = quote(&transaction, vec![result("0x5208"), result("0x2")])?;
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let output: Value = serde_json::from_slice(&output.stdout)?;
        assert_eq!(output["ok"], true);
        assert_eq!(output["kind"], "wallet.fee");
        assert_eq!(
            output["data"],
            json!({"gas":"21000", "gas_price":"2", "fee":"42000"})
        );
        assert_eq!(
            requests,
            vec![
                json!({"jsonrpc":"2.0", "id":1, "method":"eth_estimateGas", "params":[transaction_object(), "pending"]}),
                json!({"jsonrpc":"2.0", "id":1, "method":"eth_gasPrice", "params":[]}),
            ]
        );
    }

    let access_transaction = format!(
        "02f85c7d0701028252089422222222222222222222222222222222222222220184deadbeeff838f794{}e1a0{}",
        "33".repeat(20), "44".repeat(32)
    );
    let (output, requests) = quote(&access_transaction, vec![result("0x5208"), result("0x2")])?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let mut expected = transaction_object();
    expected["value"] = json!("0x1");
    expected["accessList"] = json!([{"address":format!("0x{}", "33".repeat(20)), "storageKeys":[format!("0x{}", "44".repeat(32))]}]);
    assert_eq!(requests[0]["params"], json!([expected, "pending"]));

    let (output, _) = quote(TRANSACTION, vec![result("0x20000000000001"), result("0x2")])?;
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let output: Value = serde_json::from_slice(&output.stdout)?;
    assert_eq!(output["data"]["fee"], "18014398509481986");

    let (output, _) = quote(
        TRANSACTION,
        vec![result("0xffffffffffffffffffffffffffffffff"), result("0x2")],
    )?;
    assert!(!output.status.success());
    let error: Value = serde_json::from_slice(&output.stderr)?;
    assert_eq!(error["error"]["code"], "fee_quote_overflow");

    let (output, requests) = quote(
        TRANSACTION,
        vec![
            json!({"jsonrpc":"2.0", "id":1, "error":{"code":-32000,"message":"execution reverted"}}),
        ],
    )?;
    assert!(!output.status.success());
    assert_eq!(requests.len(), 1);
    assert!(String::from_utf8_lossy(&output.stderr).contains("execution reverted"));

    for bad in [
        json!(21000),
        json!({}),
        json!(null),
        json!("0x"),
        json!("0x05208"),
        json!("0xZZ"),
        json!("0x-1"),
    ] {
        for method in ["eth_estimateGas", "eth_gasPrice"] {
            assert!(
                decode_response(method, &json!({"jsonrpc":"2.0", "id":1, "result":bad})).is_err()
            );
        }
    }
    for parameters in [
        json!([]),
        json!([transaction_object()]),
        json!([transaction_object(), "latest"]),
        json!([TRANSACTION, "pending"]),
        json!([transaction_object(), "pending", 1]),
    ] {
        assert!(request("eth_estimateGas", &parameters).is_err());
    }
    assert!(request("eth_gasPrice", &json!(["pending"])).is_err());
    for (field, value) in [
        ("to", json!("0x11")),
        ("from", json!("0x11")),
        ("gas", json!(21000)),
        ("nonce", json!("0x00")),
        ("data", json!("0xf")),
        ("type", json!("0x1")),
        ("accessList", json!([{"address":"0x11", "storageKeys":[]} ])),
        ("extra", json!(true)),
    ] {
        let mut tx = transaction_object();
        tx[field] = value;
        assert!(
            request("eth_estimateGas", &json!([tx, "pending"])).is_err(),
            "{field}"
        );
    }
    Ok(())
}

#[test]
fn estimate_fee_refuses_kernel_activity_hex() -> Result<(), Box<dyn std::error::Error>> {
    let payment = layerx_crypto::payments::Payment::OpenAccount { asset: [1; 32] };
    let facts = SigningFacts {
        actor: "did:layerx:alice",
        public_key: [7; 32],
        network_id: 402,
        identity_next_sequence: 0,
        not_before_ms: 1000,
        expires_at_ms: 2000,
        fee_limit: 100,
        idempotency_key: [8; 32],
    };
    let options = layerx_crypto::payments::CallOptions {
        from: [0x11; 20],
        chain_id: 125,
        nonce: 0,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 50,
    };
    let prepared = PreparedPayment::new(&payment, &facts, &options)?;
    assert_eq!(prepared.call().nonce, facts.identity_next_sequence);
    assert_eq!(&prepared.call().to[18..], &[0x10, 0x1a]);
    assert_eq!(
        prepared.call().data,
        layerx_platform_cli::wallet_encoding::payment_calldata(&payment, facts.actor)?
    );
    assert_eq!(prepared.call().data.len(), 36);
    assert_eq!(&prepared.call().data[4..], &[1; 32]);
    let disclosure = layerx_crypto::payments::bind_call(prepared.call(), options.from)?;
    assert_eq!(disclosure.payment, Some(payment.clone()));
    assert_eq!(disclosure.fee_limit, facts.fee_limit);

    use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::{Did, IdempotencyKey};
    use layerx_types::payload::{
        ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload,
    };
    use sha2::Digest as _;
    let kind = ActivityType::new(ModuleId::Asset, 4).map_err(|e| format!("{e:?}"))?;
    let registry = ModuleRegistry::new(&[
        ModuleRegistration::new(ModuleId::Asset, &[kind]).map_err(|e| format!("{e:?}"))?
    ])
    .map_err(|e| format!("{e:?}"))?;
    let payload = payment
        .encode(facts.actor.as_bytes())
        .map_err(|e| format!("{e:?}"))?;
    let mut hash = sha2::Sha256::new();
    hash.update(layerx_wire::hash::Domain::PayloadHash.tag());
    hash.update(&payload);
    let mut builder = EnvelopeBuilder::new();
    builder.protocol_version(3).map_err(|e| format!("{e:?}"))?;
    builder
        .network_id(facts.network_id)
        .map_err(|e| format!("{e:?}"))?;
    builder.activity_type(kind).map_err(|e| format!("{e:?}"))?;
    builder
        .actor_did(Did::new(facts.actor.as_bytes()).map_err(|e| format!("{e:?}"))?)
        .map_err(|e| format!("{e:?}"))?;
    builder
        .authority(Authority::owner(&facts.public_key).map_err(|e| format!("{e:?}"))?)
        .map_err(|e| format!("{e:?}"))?;
    builder
        .account_sequence(facts.identity_next_sequence)
        .map_err(|e| format!("{e:?}"))?;
    builder
        .timestamp_bound(
            TimestampBound::new(facts.not_before_ms, facts.expires_at_ms)
                .map_err(|e| format!("{e:?}"))?,
        )
        .map_err(|e| format!("{e:?}"))?;
    builder
        .idempotency_key(IdempotencyKey::new(facts.idempotency_key))
        .map_err(|e| format!("{e:?}"))?;
    builder
        .fee_limit(Amount::from_u128(facts.fee_limit))
        .map_err(|e| format!("{e:?}"))?;
    builder
        .payload_hash(hash.finalize().into())
        .map_err(|e| format!("{e:?}"))?;
    builder
        .payload(Payload::new(&registry, kind, &payload).map_err(|e| format!("{e:?}"))?)
        .map_err(|e| format!("{e:?}"))?;
    let kernel = layerx_wire::activity::encode_unsigned_envelope(
        &builder.build().map_err(|e| format!("{e:?}"))?,
    )?
    .iter()
    .map(|byte| format!("{byte:02x}"))
    .collect::<String>();
    let signed = layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 21000,
        to: [0x22; 20],
        data: vec![0xde, 0xad, 0xbe, 0xef],
    }
    .sign(&[1; 32])?
    .iter()
    .map(|byte| format!("{byte:02x}"))
    .collect::<String>();
    let listener = TcpListener::bind("127.0.0.1:0")?;
    listener.set_nonblocking(true)?;
    let endpoint = format!("http://{}/rpc", listener.local_addr()?);
    for transaction in [
        kernel,
        signed,
        format!("0x{TRANSACTION}00"),
        TRANSACTION.replacen("02e37d", "02e4817d", 1),
        "02c0".into(),
        "02e301".into(),
        "zz".into(),
        "".into(),
    ] {
        let output = command(&endpoint, &transaction)?;
        assert!(!output.status.success(), "accepted {transaction}");
        let error: Value = serde_json::from_slice(&output.stderr)?;
        assert_eq!(error["error"]["code"], "invalid_transaction");
        match listener.accept() {
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {}
            connection => panic!("invalid transaction contacted the endpoint: {connection:?}"),
        }
    }
    Ok(())
}
