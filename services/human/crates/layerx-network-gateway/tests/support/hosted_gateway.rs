#![allow(dead_code)]

#[path = "../../../../../platform/hosted/gateway/src/evm.rs"]
pub mod codec;

use layerx_network_gateway::{decode_hex, encode_hex, AccountIdentifier, GatewayEndpoint};
use serde_json::{json, Value};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

pub const TWO_TO_200: &str = "1606938044258990275541962092341162602522202993782792835301376";
pub const MAXIMUM: &str =
    "115792089237316195423570985008687907853269984665640564039457584007913129639935";
pub const OVERFLOW: &str =
    "115792089237316195423570985008687907853269984665640564039457584007913129639936";

pub fn vector(name: &str) -> Value {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../../platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json"
    ))
    .expect("committed hosted gateway ABI vectors");
    document["cases"]
        .as_array()
        .unwrap()
        .iter()
        .find(|case| case["name"] == name)
        .expect("recorded ABI case")
        .clone()
}

pub fn coins() -> Vec<(String, [u8; 32])> {
    let bytes = decode_hex(vector("bank.all_balances")["result"].as_str().unwrap()).unwrap();
    codec::decode_coins(&bytes).expect("production hosted bank ABI decoder")
}

pub fn identities() -> Value {
    let bytes = decode_hex(vector("addr.getUnifiedAccount")["result"].as_str().unwrap()).unwrap();
    let account = codec::decode_unified_account(&bytes).expect("production hosted identity codec");
    json!({
        "evm_address": codec::address_hex(&account.evm),
        "pax_address": account.pax_address,
        "layerx_did": format!("did:layerx:{}", encode_hex(&account.did_public_key)),
        "layerx_account": encode_hex(&account.layerx_main_account_id),
        "bound": true
    })
}

pub fn selector() -> AccountIdentifier {
    AccountIdentifier::parse(identities()["evm_address"].as_str().unwrap()).unwrap()
}

pub fn balances() -> Value {
    let rows: Vec<Value> = coins()
        .into_iter()
        .map(|(denom, word)| json!({"denom": denom, "amount": codec::uint256_decimal(&word)}))
        .collect();
    assert_eq!(json!(rows), vector("bank.all_balances")["out"]["response"]);
    json!({"account": identities(), "height": 16, "balances": rows})
}

pub fn account() -> Value {
    json!({
        "account": identities(),
        "paxeer": {
            "address": identities()["evm_address"],
            "balance": codec::uint256_quantity(&coins()[1].1),
            "nonce": "0x2a", "height": "0x10"
        }
    })
}

pub fn assets() -> Value {
    // custody_asset reads bank.supply through decode_word + uint256_decimal.
    // Generate the maximum ABI supply word independently of wallet holdings.
    let supply = codec::decode_word(&[0xff; 32]).expect("production bank supply ABI decoder");
    json!({
        "height": 16, "joined_limit": 16,
        "assets": [{
            "asset_id": "paxeer:ulxp", "denom": "ulxp", "kind": "bank",
            "name": "LayerX", "symbol": "LXP", "decimals": 18,
            "supply": codec::uint256_decimal(&supply)
        }, {
            "asset_id": format!("denom:{}", coins()[1].0), "denom": coins()[1].0,
            "kind": "bank", "name": null, "symbol": null, "decimals": null,
            "supply": null
        }]
    })
}

pub fn joined_page() -> Value {
    let mut page = assets();
    let rows = page["assets"].as_array_mut().unwrap();
    for index in 2..16 {
        let mut row = rows[0].clone();
        row["asset_id"] = json!(format!("denom:ufactory{index}"));
        row["denom"] = json!(format!("ufactory{index}"));
        row["kind"] = json!("token_factory");
        rows.push(row);
    }
    page
}

pub fn http(status: u16, body: &[u8]) -> Vec<u8> {
    let mut bytes = format!(
        "HTTP/1.1 {status} Gateway\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    )
    .into_bytes();
    bytes.extend_from_slice(body);
    bytes
}

pub struct Exchange {
    pub method: &'static str,
    pub params: Value,
    pub response: Response,
}

pub enum Response {
    Result(Value),
    Refusal(i64, &'static str),
    WrongId(Value),
    Raw(Vec<u8>),
}

pub struct Transcript {
    pub endpoint: GatewayEndpoint,
    worker: Option<JoinHandle<usize>>,
    expected: usize,
}

impl Transcript {
    pub fn serve(exchanges: Vec<Exchange>) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback transcript listener");
        listener.set_nonblocking(true).unwrap();
        let endpoint =
            GatewayEndpoint::parse(&format!("http://{}/", listener.local_addr().unwrap())).unwrap();
        let expected = exchanges.len();
        let worker = thread::spawn(move || {
            let deadline = Instant::now() + Duration::from_secs(10);
            let mut served = 0;
            for exchange in exchanges {
                let mut stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(
                                Instant::now() < deadline,
                                "all transcript calls must arrive"
                            );
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("accept transcript: {error}"),
                    }
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(2)))
                    .unwrap();
                let request = read_request(&mut stream);
                assert_eq!(request["jsonrpc"], "2.0");
                assert!(request["id"].as_u64().is_some());
                assert_eq!(request["method"], exchange.method);
                assert_eq!(request["params"], exchange.params);
                let document = match exchange.response {
                    Response::Result(result) => {
                        json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
                    }
                    Response::Refusal(code, message) => {
                        json!({"jsonrpc": "2.0", "id": request["id"], "error": {"code": code, "message": message}})
                    }
                    Response::WrongId(result) => {
                        json!({"jsonrpc": "2.0", "id": request["id"].as_u64().unwrap() + 1, "result": result})
                    }
                    Response::Raw(bytes) => {
                        stream.write_all(&bytes).unwrap();
                        served += 1;
                        continue;
                    }
                };
                stream
                    .write_all(&http(200, &serde_json::to_vec(&document).unwrap()))
                    .unwrap();
                served += 1;
            }
            served
        });
        Self {
            endpoint,
            worker: Some(worker),
            expected,
        }
    }

    pub fn finish(mut self) {
        assert_eq!(
            self.worker
                .take()
                .unwrap()
                .join()
                .expect("transcript assertions"),
            self.expected
        );
    }
}

impl Drop for Transcript {
    fn drop(&mut self) {
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

fn read_request(stream: &mut TcpStream) -> Value {
    let mut bytes = Vec::new();
    let mut chunk = [0; 1024];
    loop {
        let count = stream
            .read(&mut chunk)
            .expect("read production HTTP request");
        assert_ne!(count, 0, "complete request required");
        bytes.extend_from_slice(&chunk[..count]);
        assert!(bytes.len() <= 8192, "bounded transcript request");
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let head = std::str::from_utf8(&bytes[..end]).unwrap();
            assert!(head.starts_with("POST / HTTP/1.1\r\n"));
            let length: usize = head
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse().unwrap())
                })
                .expect("framed production request");
            if bytes.len() >= end + 4 + length {
                return serde_json::from_slice(&bytes[end + 4..end + 4 + length]).unwrap();
            }
        }
    }
}
