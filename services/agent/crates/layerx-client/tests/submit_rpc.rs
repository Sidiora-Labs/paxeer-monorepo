use std::fmt::Write as _;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::rpc::submit::{resubmit_unknown, submit_signed, Submission, SubmitError};
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint, SequencerError};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_types::result::KnownResult;
use layerx_types::rung::Rung;
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::from("0x"), |mut text, byte| {
        must(write!(text, "{byte:02x}"));
        text
    })
}

fn signed_transaction() -> Vec<u8> {
    must(
        Eip1559Call {
            chain_id: 125,
            nonce: 7,
            max_priority_fee_per_gas: 1_000_000_000,
            max_fee_per_gas: 3_000_000_000,
            gas_limit: 90_000,
            to: [0x10; 20],
            data: vec![0, 0x12, 0xab, 0xff],
        }
        .sign(&[0x46; 32]),
    )
}

fn serve(responses: Vec<Option<Value>>) -> (SequencerEndpoint, JoinHandle<Vec<Value>>) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    let handle = std::thread::spawn(move || {
        let mut requests = Vec::new();
        for response in responses {
            let (stream, _) = must(listener.accept());
            must(stream.set_read_timeout(Some(Duration::from_secs(5))));
            must(stream.set_write_timeout(Some(Duration::from_secs(5))));
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(must(reader.read_line(&mut line)), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(must(value.trim().parse::<usize>()));
                }
            }
            let mut body = vec![0; length.unwrap_or_else(|| panic!("missing content length"))];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], "eth_sendRawTransaction");
            let id = request["id"].clone();
            requests.push(request);
            if let Some(mut response) = response {
                response["jsonrpc"] = json!("2.0");
                response["id"] = id;
                let answer = response.to_string();
                let mut stream = reader.into_inner();
                must(write!(stream,
                    "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                    answer.len()
                ));
                must(stream.flush());
            }
        }
        requests
    });
    (must(SequencerEndpoint::new(&url, &[])), handle)
}

fn revert_data(reason: &str) -> String {
    let mut bytes = vec![0x08, 0xc3, 0x79, 0xa0];
    bytes.extend_from_slice(&[0; 31]);
    bytes.push(32);
    bytes.extend_from_slice(&[0; 24]);
    bytes.extend_from_slice(&must(u64::try_from(reason.len())).to_be_bytes());
    bytes.extend_from_slice(reason.as_bytes());
    bytes.resize(68 + reason.len().next_multiple_of(32), 0);
    hex(&bytes)
}

#[test]
fn submit_signed_sends_eth_send_raw_transaction_and_returns_the_hash_at_submitted() {
    let raw = signed_transaction();
    let tx_hash = [0x93; 32];
    let answer = json!({"result": hex(&tx_hash)});
    let (endpoint, server) = serve(vec![Some(answer.clone()), None, Some(answer)]);
    let expected = Submission::Submitted {
        tx_hash,
        rung: Rung::Submitted,
    };
    assert_eq!(must(submit_signed(&endpoint, &raw)), expected);

    let Submission::Unknown(unknown) = must(submit_signed(&endpoint, &raw)) else {
        panic!("a lost response must leave the submission unknown");
    };
    assert_eq!(unknown.retry_bytes(), raw);
    assert_eq!(must(resubmit_unknown(&endpoint, &unknown)), expected);
    let requests = must(server.join());
    assert_eq!(requests.len(), 3);
    for (index, request) in requests.iter().enumerate() {
        assert_eq!(request["params"], json!([hex(&raw)]));
        assert_eq!(request["id"], json!(index + 1));
    }
}

#[test]
fn submit_signed_maps_a_revert_refusal_through_known_result() {
    let raw = signed_transaction();
    for (reason, known, code) in [
        (
            "LXP_AUTH_EXPIRED: authorization expired",
            KnownResult::AuthExpired,
            -202,
        ),
        (
            "LXP_NON_CANONICAL: noncanonical transaction",
            KnownResult::NonCanonical,
            -3,
        ),
    ] {
        for data in [reason.to_owned(), revert_data(reason)] {
            let refusal = RpcRefusal {
                code: 3,
                message: "execution reverted".to_owned(),
                data: Some(json!(data)),
            };
            let (endpoint, server) = serve(vec![Some(json!({"error": {
                "code": refusal.code,
                "message": refusal.message,
                "data": refusal.data,
            }}))]);
            let Err(SubmitError::Refused {
                result,
                refusal: actual,
            }) = submit_signed(&endpoint, &raw)
            else {
                panic!("a registry revert must retain its protocol result code");
            };
            assert_eq!(KnownResult::from_revert_reason(reason), Some(known));
            assert_eq!(result.known(), Some(known));
            assert_eq!(result.raw(), code);
            assert_eq!(actual, refusal);
            let requests = must(server.join());
            assert_eq!(requests.len(), 1);
            assert_eq!(requests[0]["params"], json!([hex(&raw)]));
        }
    }

    let mut invalid_padding = revert_data("LXP_AUTH_EXPIRED: expired");
    invalid_padding.pop();
    invalid_padding.push('1');
    for data in [
        json!(revert_data("LXP_UNREGISTERED: unknown reason")),
        json!("0x08c379a0"),
        json!(invalid_padding),
        json!("0xé"),
        Value::Null,
    ] {
        let refusal = RpcRefusal {
            code: 3,
            message: "LXP_AUTH_EXPIRED: message alone is not revert data".to_owned(),
            data: Some(data),
        };
        let (endpoint, server) = serve(vec![Some(json!({"error": {
            "code": refusal.code,
            "message": refusal.message,
            "data": refusal.data,
        }}))]);
        assert_eq!(
            submit_signed(&endpoint, &raw),
            Err(SubmitError::Sequencer(SequencerError::Rpc(refusal)))
        );
        let requests = must(server.join());
        assert_eq!(requests.len(), 1);
        assert_eq!(requests[0]["params"], json!([hex(&raw)]));
    }
}
