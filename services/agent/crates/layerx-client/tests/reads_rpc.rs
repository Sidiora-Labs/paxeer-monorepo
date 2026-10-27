use std::fmt::Write as _;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::thread::JoinHandle;
use std::time::Duration;

use layerx_client::rpc::account::{account, balance, ReadContext, ReadError, Requested, Rung};
use layerx_client::sequencer::{AccountProof, RpcRefusal, SequencerEndpoint, SequencerError};
use layerx_types::amount::Amount;
use serde_json::{json, Value};

trait Must<T> {
    fn must(self) -> T;
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("{error:?}"))
    }
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::from("0x"), |mut text, byte| {
        write!(text, "{byte:02x}").must();
        text
    })
}

fn serve(exchanges: Vec<(&'static str, Value)>) -> (SequencerEndpoint, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("http://{}/", listener.local_addr().must());
    let server = std::thread::spawn(move || {
        let mut seen = Vec::new();
        for (method, response) in exchanges {
            let (stream, _) = listener.accept().must();
            stream.set_read_timeout(Some(Duration::from_secs(5))).must();
            let mut reader = BufReader::new(stream);
            let mut length = None;
            loop {
                let mut line = String::new();
                assert_ne!(reader.read_line(&mut line).must(), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = Some(value.trim().parse::<usize>().must());
                }
            }
            let mut body = vec![0; length.unwrap_or_else(|| panic!("missing content length"))];
            reader.read_exact(&mut body).must();
            let request: Value = serde_json::from_slice(&body).must();
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            seen.push(request["params"].clone());
            let mut answer = response;
            answer["jsonrpc"] = json!("2.0");
            answer["id"] = request["id"].clone();
            let answer = answer.to_string();
            let mut stream = reader.into_inner();
            write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()).must();
            stream.flush().must();
        }
        seen
    });
    (SequencerEndpoint::new(&url, &[]).must(), server)
}

fn context() -> ReadContext {
    ReadContext {
        committed_height: 0x1234,
        requested: Requested::new(Rung::Committed).must(),
    }
}

#[test]
fn balance_reads_layerxledger_balance_of_at_the_committed_height() {
    assert_eq!(Requested::new(Rung::Submitted), Err(ReadError::Submitted));
    assert_eq!(
        Requested::new(Rung::Receipted).must().level(),
        Rung::Receipted
    );
    assert_eq!(context().requested.level(), Rung::Committed);

    let account_id = [0x12; 32];
    let asset_id = [0xab; 32];
    let amount = (1_u128 << 100) + 0x1234;
    let mut word = [0; 32];
    word[16..].copy_from_slice(&amount.to_be_bytes());
    let mut overflow = [0; 32];
    overflow[15] = 1;
    let refusal = RpcRefusal {
        code: 3,
        message: "execution reverted".to_owned(),
        data: Some(json!("0xdeadbeef")),
    };
    let (endpoint, server) = serve(vec![
        ("eth_call", json!({"result": hex(&word)})),
        ("eth_call", json!({"result": hex(&[0; 32])})),
        (
            "eth_call",
            json!({"result": format!("0x{}{}", "00".repeat(16), "ff".repeat(16))}),
        ),
        ("eth_call", json!({"result": hex(&overflow)})),
        ("eth_call", json!({"result": hex(&[0; 31])})),
        ("eth_call", json!({"result": hex(&[0; 33])})),
        (
            "eth_call",
            json!({"error": {"code": refusal.code, "message": refusal.message, "data": refusal.data}}),
        ),
    ]);
    assert_eq!(
        balance(&endpoint, account_id, asset_id, context()).must(),
        Amount::from_u128(amount)
    );
    assert_eq!(
        balance(&endpoint, account_id, asset_id, context()).must(),
        Amount::ZERO
    );
    assert_eq!(
        balance(&endpoint, account_id, asset_id, context()).must(),
        Amount::from_u128(u128::MAX)
    );
    for _ in 0..3 {
        assert_eq!(
            balance(&endpoint, account_id, asset_id, context()),
            Err(ReadError::MalformedValue)
        );
    }
    assert_eq!(
        balance(&endpoint, account_id, asset_id, context()),
        Err(ReadError::Rpc(SequencerError::Rpc(refusal)))
    );
    let data = format!("0xb1ffc03e{}{}", "12".repeat(32), "ab".repeat(32));
    let expected =
        json!([{"to": "0x000000000000000000000000000000000000101a", "data": data}, "0x1234"]);
    assert_eq!(server.join().must(), vec![expected; 7]);
}

#[test]
fn account_reads_eth_get_proof_and_refuses_a_proof_for_another_address() {
    let address = [0x42; 20];
    let proof = json!({
        "address": hex(&address),
        "balance": "0x123456789abcdef",
        "nonce": "0x7",
        "codeHash": hex(&[0x51; 32]),
        "storageHash": hex(&[0x62; 32]),
        "accountProof": ["0xf843", "0xaabbcc"],
        "storageProof": []
    });
    let mut substituted = proof.clone();
    substituted["address"] = json!(hex(&[0x43; 20]));
    let (endpoint, server) = serve(vec![
        ("eth_getProof", json!({"result": proof})),
        ("eth_getProof", json!({"result": substituted})),
    ]);
    let mut balance_word = [0; 32];
    balance_word[24..].copy_from_slice(&0x0123_4567_89ab_cdef_u64.to_be_bytes());
    assert_eq!(
        account(&endpoint, address, context()).must(),
        AccountProof {
            address,
            balance: balance_word,
            nonce: 7,
            code_hash: [0x51; 32],
            storage_hash: [0x62; 32],
            account_proof: vec![vec![0xf8, 0x43], vec![0xaa, 0xbb, 0xcc]],
            storage: vec![],
        }
    );
    let mismatch = account(&endpoint, address, context());
    let seen = server.join().must();
    assert_eq!(seen, vec![json!([hex(&address), [], "0x1234"]); 2]);
    assert_eq!(mismatch, Err(ReadError::Rpc(SequencerError::Malformed)));
}
