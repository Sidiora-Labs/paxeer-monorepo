use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::time::Duration;

use layerx_client::rpc::batch::{self, BatchHeaderError, SequencerBlock};
use layerx_client::sequencer::{RpcRefusal, SequencerEndpoint};
use serde_json::{json, Value};

trait Must<T> {
    fn must(self) -> T;
}

impl<T, E: std::fmt::Debug> Must<T> for Result<T, E> {
    fn must(self) -> T {
        self.unwrap_or_else(|error| panic!("{error:?}"))
    }
}

const LEGACY_RAW: &str = "f86c098504a817c800825208943535353535353535353535353535353535353535880de0b6b3a76400008025a028ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276a067cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83";

fn block() -> Value {
    let hash = format!("0x{}", "11".repeat(32));
    json!({
        "number": "0x3e8",
        "hash": hash,
        "parentHash": format!("0x{}", "22".repeat(32)),
        "timestamp": "0x67a1b2c3",
        "transactionsRoot": format!("0x{}", "33".repeat(32)),
        "transactions": [{
            "blockHash": hash,
            "blockNumber": "0x3e8",
            "transactionIndex": "0x0",
            "from": "0x9d8a62f656a8d1615c1294fd71e9cfb3e4855a4f",
            "gas": "0x5208",
            "gasPrice": "0x4a817c800",
            "hash": "0x33469b22e9f636356c4160a87eb19df52b7412e8eac32a4a55ffe88ea8350788",
            "input": "0x",
            "nonce": "0x9",
            "to": "0x3535353535353535353535353535353535353535",
            "value": "0xde0b6b3a7640000",
            "type": "0x0",
            "chainId": "0x1",
            "v": "0x25",
            "r": "0x28ef61340bd939bc2195fe537567866003e1a15d3c71ff63e1590620aa636276",
            "s": "0x67cbe9d8997f761aecb703304b3800ccf555c9f3dc64214b297fb1966a3b6d83"
        }]
    })
}

#[test]
fn batch_lookup_reads_the_block_by_number_and_refuses_a_null_block() {
    type Lookup = fn(&SequencerEndpoint, u64) -> Result<SequencerBlock, BatchHeaderError>;
    let lookups: [Lookup; 3] = [
        batch::lookup,
        batch::lookup_untrusted,
        batch::lookup_with_history,
    ];
    let refusal = RpcRefusal {
        code: -32000,
        message: "block unavailable".to_owned(),
        data: Some(json!({"height": "0x3eb"})),
    };
    let exchanges: Vec<_> = (0..lookups.len())
        .flat_map(|_| {
            [
                (1000_u64, Some(json!({"result": block()}))),
                (1001, Some(json!({"result": null}))),
                (1002, Some(json!({"result": block()}))),
                (
                    1003,
                    Some(json!({"error": {
                        "code": refusal.code,
                        "message": refusal.message,
                        "data": refusal.data,
                    }})),
                ),
                (1004, None),
            ]
        })
        .collect();
    let listener = TcpListener::bind("127.0.0.1:0").must();
    let url = format!("http://{}/", listener.local_addr().must());
    let server = std::thread::spawn(move || {
        let mut requests = 0;
        for (height, response) in exchanges {
            let (stream, _) = listener.accept().must();
            stream.set_read_timeout(Some(Duration::from_secs(5))).must();
            let mut reader = BufReader::new(stream);
            let mut length = 0_usize;
            loop {
                let mut line = String::new();
                assert_ne!(reader.read_line(&mut line).must(), 0);
                if line == "\r\n" {
                    break;
                }
                if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                    length = value.trim().parse().must();
                }
            }
            let mut body = vec![0; length];
            reader.read_exact(&mut body).must();
            let request: Value = serde_json::from_slice(&body).must();
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], "eth_getBlockByNumber");
            assert_eq!(request["params"], json!([format!("{height:#x}"), true]));
            requests += 1;
            let Some(mut response) = response else {
                continue;
            };
            response["jsonrpc"] = json!("2.0");
            response["id"] = request["id"].clone();
            let answer = response.to_string();
            let mut stream = reader.into_inner();
            write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",
                answer.len()
            )
            .must();
            stream.flush().must();
        }
        requests
    });
    let endpoint = SequencerEndpoint::new(&url, &[]).must();
    let raw = (0..LEGACY_RAW.len())
        .step_by(2)
        .map(|index| u8::from_str_radix(&LEGACY_RAW[index..index + 2], 16).must())
        .collect();
    let expected = SequencerBlock {
        number: 1000,
        hash: [0x11; 32],
        parent_hash: [0x22; 32],
        timestamp: 0x67a1_b2c3,
        transactions_root: [0x33; 32],
        transactions: vec![raw],
    };
    for lookup in lookups {
        assert_eq!(lookup(&endpoint, 1000), Ok(expected.clone()));
        assert_eq!(lookup(&endpoint, 1001), Err(BatchHeaderError::Unavailable));
        assert_eq!(lookup(&endpoint, 1002), Err(BatchHeaderError::Malformed));
        assert_eq!(
            lookup(&endpoint, 1003),
            Err(BatchHeaderError::Rpc(refusal.clone()))
        );
        assert_eq!(lookup(&endpoint, 1004), Err(BatchHeaderError::Unavailable));
    }
    assert_eq!(server.join().must(), lookups.len() * 5);
}
