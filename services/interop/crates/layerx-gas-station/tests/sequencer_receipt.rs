use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::path::PathBuf;
use std::time::Duration;

use layerx_gas_station::config::StationConfig;
use layerx_gas_station::quote::{word, Word, SIDIORA};
use layerx_gas_station::rpc::{
    bytes, decode_full_sequencer_receipt, hex, ConfiguredRpc, Execution, HttpsExchange, RpcFault,
    Rung, SequencerEndpoint, SequencerLog,
};
use serde_json::{json, Value};

type TestResult = Result<(), Box<dyn std::error::Error>>;

fn fixtures() -> Result<Value, serde_json::Error> {
    serde_json::from_str(include_str!("fixtures/sequencer_receipts.json"))
}

fn fixed<const N: usize>(value: &Value) -> Result<[u8; N], Box<dyn std::error::Error>> {
    Ok(bytes(value.as_str().ok_or("missing hex string")?)?
        .try_into()
        .map_err(|_| "incorrect byte length")?)
}

#[test]
fn a_committed_answer_decodes_logs_receipt_bytes_and_block() -> TestResult {
    let fixture = fixtures()?;
    let answer = &fixture["committed"];
    let hash: Word = fixed(&answer["txHash"])?;
    let receipt = decode_full_sequencer_receipt(answer, &hash)?;
    assert_eq!(receipt.tx_hash, hash);
    assert_eq!(receipt.nonce, 1);
    assert_eq!(
        receipt.rung,
        Rung::Committed(Execution {
            seq: 2,
            succeeded: true,
            sponsor: Some([0; 20]),
        })
    );
    assert_eq!(receipt.seq, Some(2));
    assert_eq!(receipt.height, Some(1000));
    assert_eq!(receipt.index, Some(2));
    assert_eq!(receipt.timestamp, Some(1_750_000_400_000));
    assert_eq!(receipt.status, Some(1));
    assert_eq!(receipt.gas_used, Some(46_220));
    assert_eq!(receipt.cumulative_gas_used, Some(67_220));
    assert_eq!(receipt.effective_gas_price, Some(2_000_000_000));
    assert_eq!(receipt.fee_denom.as_deref(), Some("uhpx"));
    assert_eq!(receipt.fee_amount, Some(word(92_440_000_000_000)));
    assert_eq!(receipt.sponsor, Some([0; 20]));
    assert_eq!(receipt.from, Some(fixed(&answer["from"])?));
    assert_eq!(receipt.to, Some(fixed(&answer["to"])?));
    assert_eq!(receipt.contract_address, Some([0; 20]));
    assert_eq!(receipt.vm_error.as_deref(), Some(""));
    assert_eq!(receipt.key_id, Some([0x6b; 32]));
    assert_eq!(receipt.return_data, word(1));
    let log = &answer["logs"][0];
    assert_eq!(
        receipt.logs,
        vec![SequencerLog {
            address: fixed(&log["address"])?,
            topics: log["topics"]
                .as_array()
                .ok_or("missing topics")?
                .iter()
                .map(fixed)
                .collect::<Result<_, _>>()?,
            data: word(250_000_000_000_000_000_000).to_vec(),
            log_index: 0,
        }]
    );
    let canonical = bytes(answer["receipt"].as_str().ok_or("missing receipt")?)?;
    assert_eq!(receipt.receipt, canonical);
    assert_eq!(receipt.receipt.len(), 968);
    assert_eq!(receipt.block_number, Some(1000));
    assert_eq!(receipt.block_hash, Some([0xab; 32]));

    let mut ordered = answer.clone();
    let mut second = log.clone();
    second["logIndex"] = json!("0x1");
    second["data"] = json!("0x0102");
    ordered["logs"] = json!([log, second]);
    let ordered = decode_full_sequencer_receipt(&ordered, &hash)?;
    assert_eq!(ordered.logs.len(), 2);
    assert_eq!(ordered.logs[0], receipt.logs[0]);
    assert_eq!(ordered.logs[1].log_index, 1);
    assert_eq!(ordered.logs[1].data, [1, 2]);

    let reverted = &fixture["reverted"];
    let reverted = decode_full_sequencer_receipt(reverted, &fixed(&reverted["txHash"])?)?;
    assert_eq!(reverted.status, Some(0));
    assert!(matches!(
        reverted.rung,
        Rung::Receipted(Execution {
            succeeded: false,
            ..
        })
    ));
    assert!(reverted.logs.is_empty());
    assert_eq!(reverted.vm_error.as_deref(), Some("execution reverted"));

    for (field, bad) in [
        ("txHash", json!(hex(&[0xff; 32]))),
        ("rung", json!("finalized")),
        ("status", json!("0x2")),
        ("nonce", json!("0x01")),
        ("seq", json!("0x10000000000000000")),
        ("gasUsed", json!(42)),
        ("keyId", json!(hex(&[0x6b; 31]))),
        ("keyId", json!(hex(&[0x6b; 33]))),
        ("from", json!("0x1234")),
        ("sponsor", Value::Null),
        ("receipt", json!("0x123")),
        ("logs", Value::Null),
    ] {
        let mut bad_answer = answer.clone();
        bad_answer[field] = bad;
        assert_eq!(
            decode_full_sequencer_receipt(&bad_answer, &hash),
            Err(RpcFault::Malformed),
            "{field}"
        );
    }
    for field in ["address", "topics", "data", "logIndex"] {
        let mut bad_answer = answer.clone();
        bad_answer["logs"][0][field] = Value::Null;
        assert_eq!(
            decode_full_sequencer_receipt(&bad_answer, &hash),
            Err(RpcFault::Malformed),
            "{field}"
        );
    }
    let mut bad_topic = answer.clone();
    bad_topic["logs"][0]["topics"][0] = json!("0x01");
    assert_eq!(
        decode_full_sequencer_receipt(&bad_topic, &hash),
        Err(RpcFault::Malformed)
    );
    Ok(())
}

#[test]
fn an_answer_with_a_block_below_committed_is_refused() -> TestResult {
    let fixture = fixtures()?;
    let hash = fixed(&fixture["committed"]["txHash"])?;
    for rung in ["submitted", "receipted"] {
        for field in ["blockNumber", "blockHash"] {
            for block in [fixture["committed"][field].clone(), Value::Null] {
                let mut answer = fixture[rung].clone();
                answer[field] = block;
                assert_eq!(
                    decode_full_sequencer_receipt(&answer, &hash),
                    Err(RpcFault::Malformed),
                    "{rung} {field}"
                );
            }
        }
    }
    for field in ["blockNumber", "blockHash"] {
        let mut missing = fixture["committed"].clone();
        missing
            .as_object_mut()
            .ok_or("missing receipt object")?
            .remove(field);
        assert_eq!(
            decode_full_sequencer_receipt(&missing, &hash),
            Err(RpcFault::Malformed)
        );
        missing[field] = Value::Null;
        assert_eq!(
            decode_full_sequencer_receipt(&missing, &hash),
            Err(RpcFault::Malformed)
        );
    }
    for (field, value) in [
        ("blockNumber", json!("0x3e7")),
        ("blockNumber", json!("0x10000000000000000")),
        ("blockHash", json!("0x1234")),
    ] {
        let mut answer = fixture["committed"].clone();
        answer[field] = value;
        assert_eq!(
            decode_full_sequencer_receipt(&answer, &hash),
            Err(RpcFault::Malformed)
        );
    }
    assert_eq!(
        decode_full_sequencer_receipt(&Value::Null, &hash),
        Err(RpcFault::Malformed)
    );
    Ok(())
}

struct TrustStore {
    path: PathBuf,
    previous: Option<std::ffi::OsString>,
}

impl Drop for TrustStore {
    fn drop(&mut self) {
        if let Some(previous) = &self.previous {
            std::env::set_var("SSL_CERT_FILE", previous);
        } else {
            std::env::remove_var("SSL_CERT_FILE");
        }
        let _ = std::fs::remove_file(&self.path);
    }
}

#[test]
fn full_receipt_reads_pax_sequencer_receipt_by_hash() -> TestResult {
    let fixture = fixtures()?;
    let certificate = fixture["tls"]["certificate"]
        .as_str()
        .ok_or("missing TLS certificate")?;
    let private_key = fixture["tls"]["privateKey"]
        .as_str()
        .ok_or("missing TLS key")?;
    let identity =
        native_tls::Identity::from_pkcs8(certificate.as_bytes(), private_key.as_bytes())?;
    let acceptor = native_tls::TlsAcceptor::new(identity)?;
    let trust = TrustStore {
        path: std::env::temp_dir().join(format!("sequencer-receipt-ca-{}.pem", std::process::id())),
        previous: std::env::var_os("SSL_CERT_FILE"),
    };
    std::fs::OpenOptions::new()
        .write(true)
        .create_new(true)
        .open(&trust.path)?
        .write_all(certificate.as_bytes())?;
    std::env::set_var("SSL_CERT_FILE", &trust.path);
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = format!("https://{}", listener.local_addr()?);
    let hash: Word = fixed(&fixture["committed"]["txHash"])?;
    let responses = [
        json!({"jsonrpc":"2.0", "id":1, "result":fixture["committed"]}),
        json!({"jsonrpc":"2.0", "id":1, "result":fixture["unknown"]}),
        json!({"jsonrpc":"2.0", "id":1, "result":fixture["submitted"]}),
        json!({"jsonrpc":"2.0", "id":1, "result":fixture["receipted"]}),
        json!({"jsonrpc":"2.0", "id":1, "error":{"code":-32000}}),
    ];
    let server = std::thread::spawn(move || -> Result<(), String> {
        for response in responses {
            let (stream, _) = listener.accept().map_err(|error| error.to_string())?;
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .map_err(|error| error.to_string())?;
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .map_err(|error| error.to_string())?;
            let mut stream = acceptor.accept(stream).map_err(|error| error.to_string())?;
            let mut request = Vec::new();
            let mut byte = [0; 1];
            while !request.ends_with(b"\r\n\r\n") {
                stream
                    .read_exact(&mut byte)
                    .map_err(|error| error.to_string())?;
                request.push(byte[0]);
                if request.len() > 16_384 {
                    return Err("request headers too large".into());
                }
            }
            let headers = std::str::from_utf8(&request).map_err(|error| error.to_string())?;
            assert!(headers.starts_with("POST / HTTP/1.1\r\n"));
            let length = headers
                .lines()
                .find_map(|line| line.strip_prefix("Content-Length: "))
                .ok_or("missing content length")?
                .parse::<usize>()
                .map_err(|error| error.to_string())?;
            if length > 16_384 {
                return Err("request body too large".into());
            }
            let mut body = vec![0; length];
            stream
                .read_exact(&mut body)
                .map_err(|error| error.to_string())?;
            let request: Value =
                serde_json::from_slice(&body).map_err(|error| error.to_string())?;
            assert_eq!(
                request,
                json!({"jsonrpc":"2.0", "id":1, "method":"pax_sequencerReceipt", "params":[hex(&hash)]})
            );
            let body = response.to_string();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).map_err(|error| error.to_string())?;
            stream.flush().map_err(|error| error.to_string())?;
            stream.shutdown().map_err(|error| error.to_string())?;
        }
        Ok(())
    });
    let config = StationConfig {
        chain_id: 1325,
        endpoints: vec![endpoint],
        paymaster: [0x44; 20],
        token: SIDIORA,
        decimals: 6,
        max_rate_age: 300,
        spread_bps: 500,
        margin_bps: 100,
        per_account_limit: 8_000_000,
        per_interval_limit: 16_000_000,
        per_quote_limit: 4_000_000,
        interval_seconds: 60,
        balance_floor: 100,
        relayer_key_env: "PAXEER_SEQUENCER_RECEIPT_TEST_KEY".into(),
    };
    let endpoint = SequencerEndpoint::new(ConfiguredRpc::new(&config, HttpsExchange)?);
    assert_eq!(
        endpoint.full_receipt(&hash)?,
        Some(decode_full_sequencer_receipt(&fixture["committed"], &hash)?)
    );
    assert_eq!(endpoint.full_receipt(&hash)?, None);
    let submitted = endpoint
        .full_receipt(&hash)?
        .ok_or("missing submitted receipt")?;
    assert_eq!(submitted.rung, Rung::Submitted);
    assert_eq!(submitted.tx_hash, hash);
    assert_eq!(submitted.nonce, 1);
    assert_eq!(submitted.seq, None);
    assert_eq!(submitted.key_id, None);
    assert!(submitted.logs.is_empty());
    assert!(submitted.receipt.is_empty());
    assert_eq!(
        endpoint.full_receipt(&hash)?,
        Some(decode_full_sequencer_receipt(&fixture["receipted"], &hash)?)
    );
    assert_eq!(
        endpoint.full_receipt(&hash),
        Err(RpcFault::Rejected { code: -32000 })
    );
    server
        .join()
        .map_err(|_| "TLS server panicked")?
        .map_err(std::io::Error::other)?;
    Ok(())
}
