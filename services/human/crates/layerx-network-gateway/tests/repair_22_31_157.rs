// Gateway amount reads retain full-width quantities across ABI, transport, and consumers.

include!("support/required.rs");

#[allow(dead_code)]
mod hosted_codec {
    include!(concat!(env!("OUT_DIR"), "/hosted_evm.rs"));
}

use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Arc;
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_network_gateway::{
    counted, decode_account, decode_account_balances, decode_assets, decode_hex, optional_quantity,
    quantity, AccountIdentifier, Evidence, GatewayEndpoint, GatewayError, Quantity,
    WalletObservation,
};
use serde_json::{json, Value};

const MAXIMUM: &str =
    "115792089237316195423570985008687907853269984665640564039457584007913129639935";
const TWO_TO_200: &str = "1606938044258990275541962092341162602522202993782792835301376";

fn vector(name: &str) -> Value {
    let document: Value = serde_json::from_str(include_str!(
        "../../../../platform/hosted/gateway/tests/fixtures/paxeer-abi-vectors.json"
    ))
    .required_with("producer's recorded ABI vectors");
    document["cases"]
        .as_array()
        .required_with("producer cases")
        .iter()
        .find(|case| case["name"] == name)
        .required_with("recorded case")
        .clone()
}

struct AbiProducer {
    bank_answer: Vec<u8>,
    identity_answer: Vec<u8>,
}

impl AbiProducer {
    fn new(maximum: bool) -> Self {
        let mut bank_answer = decode_hex(vector("bank.all_balances")["result"].as_str().required())
            .required_with("recorded ABI bytes");
        if maximum {
            // Follow the recorded ABI's dynamic offsets to the second tuple's
            // uint256 word. Denomination, offsets and the smaller coin stay intact.
            let base = usize::try_from(hosted_codec::Answer::new(&bank_answer).u64(0).required())
                .required();
            let elements = base + 32;
            let second = usize::try_from(
                hosted_codec::Answer::new(&bank_answer[elements..])
                    .u64(1)
                    .required(),
            )
            .required();
            bank_answer[elements + second..elements + second + 32].fill(0xff);
        }
        Self {
            bank_answer,
            identity_answer: decode_hex(
                vector("addr.getUnifiedAccount")["result"]
                    .as_str()
                    .required(),
            )
            .required_with("recorded account ABI bytes"),
        }
    }

    fn coins(&self) -> Vec<(String, [u8; 32])> {
        hosted_codec::decode_coins(&self.bank_answer).required_with("real hosted bank decoder")
    }

    fn account(&self) -> Value {
        let account = hosted_codec::decode_unified_account(&self.identity_answer)
            .required_with("real hosted account decoder");
        json!({
            "evm_address": hosted_codec::address_hex(&account.evm),
            "pax_address": account.pax_address,
            "layerx_did": format!("did:layerx:{}", layerx_network_gateway::encode_hex(&account.did_public_key)),
            "layerx_account": layerx_network_gateway::encode_hex(&account.layerx_main_account_id),
            "bound": true
        })
    }

    fn balances(&self) -> Value {
        // The actual hosted codecs produce every amount put on the JSON wire.
        let coins: Vec<Value> = self
            .coins()
            .into_iter()
            .map(|(denom, amount)| {
                json!({"denom": denom, "amount": hosted_codec::uint256_decimal(&amount)})
            })
            .collect();
        json!({"account": self.account(), "height": 16, "balances": coins})
    }

    fn account_join(&self) -> Value {
        json!({
            "account": self.account(),
            "paxeer": {
                "address": self.account()["evm_address"],
                "balance": hosted_codec::uint256_quantity(&self.coins()[1].1),
                "nonce": "0x2a", "height": "0x10"
            }
        })
    }

    fn assets(&self) -> Value {
        let mut assets: Vec<Value> = self
            .coins()
            .into_iter()
            .map(|(denom, amount)| {
                json!({
                    "asset_id": format!("paxeer:{denom}"), "denom": denom,
                    "kind": "bank", "name": null, "symbol": null, "decimals": 18,
                    "supply": hosted_codec::uint256_decimal(&amount)
                })
            })
            .collect();
        assets.push(json!({
            "asset_id": "paxeer:unavailable", "denom": "unavailable", "kind": "bank",
            "name": null, "symbol": null, "decimals": null, "supply": null
        }));
        json!({"height": 16, "joined_limit": 16, "assets": assets})
    }

    fn serve(self) -> ProducerTransport {
        let listener =
            TcpListener::bind("127.0.0.1:0").required_with("bind ABI producer transport");
        listener.set_nonblocking(true).required();
        let endpoint =
            GatewayEndpoint::parse(&format!("http://{}/", listener.local_addr().required()))
                .required_with("local producer endpoint");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let deadline = Instant::now() + Duration::from_secs(10);
            let mut served = 0;
            while !stopped.load(Ordering::Acquire) && Instant::now() < deadline && served < 3 {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .required();
                stream
                    .set_write_timeout(Some(Duration::from_secs(2)))
                    .required();
                let request = read_request(&mut stream);
                let result = match request["method"].as_str().required() {
                    "px_getBalances" => {
                        assert_eq!(request["params"], json!([self.account()["evm_address"]]));
                        self.balances()
                    }
                    "px_getAccount" => {
                        assert_eq!(request["params"], json!([self.account()["evm_address"]]));
                        self.account_join()
                    }
                    "px_listAssets" => {
                        assert_eq!(request["params"], json!([]));
                        self.assets()
                    }
                    other => panic!("unexpected method {other}"),
                };
                let body = serde_json::to_vec(&json!({
                    "jsonrpc": "2.0", "id": request["id"], "result": result
                }))
                .required_with("serialize actual codec results");
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).required();
                stream.write_all(&body).required();
                served += 1;
            }
            served
        });
        ProducerTransport {
            endpoint,
            stop,
            worker: Some(worker),
        }
    }
}

fn read_request(stream: &mut TcpStream) -> Value {
    let mut bytes = Vec::new();
    let mut chunk = [0; 1024];
    loop {
        let count = stream
            .read(&mut chunk)
            .required_with("read real HTTP request");
        assert_ne!(count, 0, "complete request required");
        bytes.extend_from_slice(&chunk[..count]);
        assert!(bytes.len() <= 8192, "bounded request");
        if let Some(header_end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let header = std::str::from_utf8(&bytes[..header_end]).required();
            assert!(header.starts_with("POST / HTTP/1.1\r\n"));
            let length: usize = header
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse().required())
                })
                .required_with("framed request");
            if bytes.len() >= header_end + 4 + length {
                return serde_json::from_slice(&bytes[header_end + 4..header_end + 4 + length])
                    .required_with("real client JSON-RPC request");
            }
        }
    }
}

struct ProducerTransport {
    endpoint: GatewayEndpoint,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<usize>>,
}

impl ProducerTransport {
    fn finish(mut self) {
        assert_eq!(
            self.worker
                .take()
                .required()
                .join()
                .required_with("producer served requests"),
            3
        );
    }
}

impl Drop for ProducerTransport {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let _ = worker.join();
        }
    }
}

#[test]
fn gateway_amount_width_producer_transport_consumer() {
    for maximum in [false, true] {
        let producer = AbiProducer::new(maximum);
        let coins = producer.coins();
        if !maximum {
            assert_eq!(
                producer.balances()["balances"],
                vector("bank.all_balances")["out"]["response"]
            );
        }
        let expected_large = if maximum { MAXIMUM } else { TWO_TO_200 };
        assert_eq!(hosted_codec::uint256_decimal(&coins[1].1), expected_large);
        let account =
            AccountIdentifier::parse(producer.account()["evm_address"].as_str().required())
                .required();
        let wire = producer.serve();
        let balances = wire
            .endpoint
            .get_balances(account)
            .required_with("full uint256 balance read");
        let joined = wire
            .endpoint
            .get_account(account)
            .required_with("full uint256 EVM balance read");
        let assets = wire
            .endpoint
            .list_assets()
            .required_with("full uint256 supply read");
        wire.finish();

        let observed =
            WalletObservation::accept(balances).required_with("real Human balance consumer");
        assert_eq!(observed.joined().height, 16);
        assert_eq!(observed.joined().evidence, Evidence::GatewayReported);
        assert_eq!(observed.joined().balances.len(), 2);
        assert_eq!(Some(observed.wallet()), account.evm_address());
        assert_eq!(joined.account, observed.joined().account);
        assert_eq!(joined.evidence, Evidence::GatewayReported);
        assert_eq!(assets.evidence, Evidence::GatewayReported);
        assert_eq!(assets.height, 16);
        assert_eq!(assets.joined_limit, 16);
        for ((denom, word), asset) in coins.iter().zip(&assets.assets) {
            let amount = observed
                .balance(denom)
                .required_with("every produced denomination retained");
            assert_eq!(amount.to_be_bytes(), *word);
            assert_eq!(amount.to_string(), hosted_codec::uint256_decimal(word));
            assert_eq!(asset.supply, Some(amount));
            assert_eq!(asset.decimals, Some(18));
        }
        let large = observed.balance(&coins[1].0).required();
        assert_eq!(large.to_string(), expected_large);
        let paxeer = joined.paxeer.required();
        assert_eq!(paxeer.balance, large);
        assert_eq!(paxeer.height, 16);
        assert_eq!(paxeer.nonce, 42);
        assert_eq!(assets.assets[2].supply, None);
        assert_eq!(
            observed.balance_for_operation(&coins[0].0, "native-send"),
            Ok(Some(123_456_789))
        );
        assert_eq!(
            observed.balance_for_operation(&coins[1].0, "native-send"),
            Err(GatewayError::AmountOutOfRange {
                operation: "native-send"
            })
        );
        assert_eq!(observed.balance("missing"), None);
        assert_eq!(
            observed.balance_for_operation("missing", "native-send"),
            Ok(None)
        );
        // Refusing one operation must not damage or hide the account's read view.
        assert_eq!(observed.balance(&coins[1].0), Some(large));
        assert_eq!(observed.joined().balances.len(), 2);
    }
}

fn assert_quantity_bit_positions() {
    for bit in 0..256 {
        let mut word = [0; 32];
        word[31 - bit / 8] = 1 << (bit % 8);
        for rendered in [
            hosted_codec::uint256_decimal(&word),
            hosted_codec::uint256_quantity(&word),
        ] {
            let value =
                quantity(&json!(rendered)).required_with("all uint256 bit positions decode");
            assert_eq!(value.to_be_bytes(), word);
            assert_eq!(value.to_string(), hosted_codec::uint256_decimal(&word));
        }
    }
}

#[test]
fn gateway_amount_width_quantity_and_operation_bounds() {
    assert_quantity_bit_positions();
    assert_eq!(
        quantity(&json!(MAXIMUM)),
        Ok(Quantity::from_be_bytes([0xff; 32]))
    );
    assert_eq!(
        quantity(&json!(u64::MAX)),
        Ok(Quantity::from(u128::from(u64::MAX)))
    );
    assert_eq!(quantity(&json!("0")), Ok(Quantity::from(0)));
    assert_eq!(quantity(&json!("0x0")), Ok(Quantity::from(0)));
    assert_eq!(optional_quantity(&Value::Null), Ok(None));
    assert_eq!(optional_quantity(&json!("0")), Ok(Some(Quantity::from(0))));
    assert_eq!(
        Quantity::from(u128::MAX).for_operation("native-send"),
        Ok(u128::MAX)
    );
    let too_large = quantity(&json!("340282366920938463463374607431768211456")).required();
    assert_eq!(
        too_large.for_operation("native-send"),
        Err(GatewayError::AmountOutOfRange {
            operation: "native-send"
        })
    );
    for value in [
        json!("115792089237316195423570985008687907853269984665640564039457584007913129639936"),
        json!(format!("0x1{}", "0".repeat(64))),
        json!("-1"),
        json!("+1"),
        json!(""),
        json!("0x"),
        json!("0xgg"),
        json!("1.0"),
        json!("1e2"),
        json!("1 2"),
        json!(true),
        json!(-1),
        json!(1.5),
        Value::Null,
    ] {
        assert_eq!(
            quantity(&value),
            Err(GatewayError::MalformedAnswer),
            "{value}"
        );
    }
    assert_eq!(counted(&json!(u64::MAX.to_string())), Ok(u64::MAX));
    assert_eq!(
        counted(&json!("18446744073709551616")),
        Err(GatewayError::MalformedAnswer)
    );
    assert_eq!(counted(&json!(MAXIMUM)), Err(GatewayError::MalformedAnswer));

    let producer = AbiProducer::new(true);
    let overflow =
        json!("115792089237316195423570985008687907853269984665640564039457584007913129639936");
    let mut balances = producer.balances();
    balances["balances"][1]["amount"] = overflow.clone();
    assert_eq!(
        decode_account_balances(&balances),
        Err(GatewayError::MalformedAnswer)
    );
    balances = producer.balances();
    balances["balances"][1]["amount"] = json!("0");
    let zero = WalletObservation::accept(decode_account_balances(&balances).required()).required();
    let denom = producer.coins()[1].0.clone();
    assert_eq!(
        zero.balance_for_operation(&denom, "native-send"),
        Ok(Some(0))
    );
    balances["balances"][1]["denom"] = balances["balances"][0]["denom"].clone();
    assert_eq!(
        decode_account_balances(&balances),
        Err(GatewayError::MalformedAnswer)
    );
    let mut account = producer.account_join();
    account["paxeer"]["balance"] = overflow.clone();
    assert_eq!(decode_account(&account), Err(GatewayError::MalformedAnswer));
    account = producer.account_join();
    account["paxeer"]["nonce"] = json!("18446744073709551616");
    assert_eq!(decode_account(&account), Err(GatewayError::MalformedAnswer));
    let mut assets = producer.assets();
    assets["assets"][1]["supply"] = overflow;
    assert_eq!(decode_assets(&assets), Err(GatewayError::MalformedAnswer));
    assets = producer.assets();
    assets["assets"][1]["decimals"] = json!(256);
    assert_eq!(decode_assets(&assets), Err(GatewayError::MalformedAnswer));
    assets["assets"][1]["decimals"] = json!(255);
    assert_eq!(
        decode_assets(&assets).required().assets[1].decimals,
        Some(255)
    );
}
