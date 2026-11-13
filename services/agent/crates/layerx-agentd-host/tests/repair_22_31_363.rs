//! Task 22.31.363 closes `payer_payment_asset` with
//! `payer_payment_asset_accepts_node_payment` and `payer_payment_asset_refuses_node_revert`;
//! `payer_payment_asset_retains_independent_bindings` proves denomination and fee checks.
//! Baseline verification was not run: the ordered gate instructions forbid testing
//! before implementation or invoking the verify command outside hf-done.
//! Out-of-path wiring requests: none.

use layerx_agentd_host::payer;

use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::sync::OnceLock;
use std::time::Duration;

use layerx_agentd::prepare::evm::{AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext, Rung};
use layerx_client::sequencer::{ExecutedReceipt, ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_mcp::tools::web::WebPayerError;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn unhex(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .unwrap_or_else(|| panic!("hex string: {value}"));
    let digits = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(digits.len() % 2, 0);
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| must(u8::from_str_radix(must(std::str::from_utf8(pair)), 16)))
        .collect()
}

fn fixed<const N: usize>(value: &Value) -> [u8; N] {
    must(unhex(value).try_into())
}

fn producer() -> &'static Value {
    static PRODUCER: OnceLock<Value> = OnceLock::new();
    PRODUCER.get_or_init(|| {
        must(serde_json::from_str(include_str!(
            "../src/repair_22_31_363_receipts.json"
        )))
    })
}

fn authenticated(case: &Value) -> ExecutedReceipt {
    let view = case["receiptJson"].clone();
    let tx_hash = fixed(&view["txHash"]);
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let endpoint = must(SequencerEndpoint::new(
        &format!("http://{}/", must(listener.local_addr())),
        &[],
    ));
    must(listener.set_nonblocking(true));
    let server = std::thread::spawn(move || {
        for method in ["pax_sequencerReceipt", "eth_getTransactionReceipt"] {
            let deadline = std::time::Instant::now() + Duration::from_secs(10);
            let stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(std::time::Instant::now() < deadline, "waiting for {method}");
                        std::thread::sleep(Duration::from_millis(1));
                    }
                    Err(error) => panic!("{error}"),
                }
            };
            must(stream.set_read_timeout(Some(Duration::from_secs(10))));
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
            let mut body = vec![0; length.unwrap_or_else(|| panic!("content length"))];
            must(reader.read_exact(&mut body));
            let request: Value = must(serde_json::from_slice(&body));
            assert_eq!(request["jsonrpc"], "2.0");
            assert_eq!(request["method"], method);
            assert_eq!(request["params"], json!([view["txHash"]]));
            let result = if method == "pax_sequencerReceipt" {
                view.clone()
            } else {
                Value::Null
            };
            let response =
                json!({"jsonrpc":"2.0", "id":request["id"], "result": result}).to_string();
            must(write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()));
            must(reader.get_mut().flush());
        }
    });
    let keys = [published_key(case)];
    let lookup = lookup_authenticated(
        &endpoint,
        tx_hash,
        LookupContext {
            receipt_keys: &keys,
        },
    );
    must(server.join());
    let Lookup::Verified(receipt) = must(lookup) else {
        panic!("authenticated node receipt")
    };
    assert_eq!(receipt.rung(), Rung::Receipted);
    let encoded = unhex(&case["signedReceipt"]);
    assert_eq!(receipt.canonical_bytes(), encoded);
    assert_eq!(
        must(layerx_wire::receipt::encode(receipt.receipt())),
        encoded
    );
    let unsigned = must(layerx_wire::receipt::encode_unsigned(receipt.receipt()));
    assert_eq!(
        must(layerx_wire::hash::receipt_digest(&unsigned)),
        fixed::<32>(&case["receiptDigest"])
    );
    let execution = receipt.execution();
    assert_ne!(execution.payment_denom, execution.fee_denom);
    assert_eq!(
        receipt
            .receipt()
            .protocol()
            .unwrap_or_else(|| panic!("protocol"))
            .asset(),
        <[u8; 32]>::from(Sha256::digest(execution.payment_denom.as_bytes()))
    );
    execution.clone()
}

fn published_key(case: &Value) -> ReceiptKeyEntry {
    let pin = &case["pinnedKey"];
    ReceiptKeyEntry {
        key_id: fixed(&pin["keyId"]),
        public_key: fixed(&pin["publicKey"]),
        first_height: pin["firstHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("first height")),
        last_height: pin["lastHeight"]
            .as_u64()
            .unwrap_or_else(|| panic!("last height")),
    }
}

fn payment_call(case: &Value) -> ([u8; 32], [u8; 20], PrecompileCall) {
    let raw = unhex(&case["rawTransaction"]);
    let (transaction, signature) = must(decode_signed(&raw));
    let sender = must(recover_sender(&transaction, &signature));
    assert_eq!(transaction.to, PrecompileTarget::Ledger.address());
    assert_eq!(&transaction.data[..4], &[0x78, 0xea, 0x11, 0xdb]);
    assert_eq!(transaction.data.len(), 4 + 6 * 32);
    let call = PrecompileCall {
        target: PrecompileTarget::Ledger,
        selector: must(transaction.data[..4].try_into()),
        arguments: transaction.data[4..]
            .chunks_exact(32)
            .map(|word| AbiArgument::Word(must(word.try_into())))
            .collect(),
    };
    assert_eq!(must(call.encode()), transaction.data);
    (transaction_hash(&raw), sender, call)
}

#[test]
fn payer_payment_asset_accepts_node_payment() {
    let case = &producer()["success"];
    let execution = authenticated(case);
    assert!(execution.success);
    let (hash, sender, call) = payment_call(case);
    let payment = must(payer::receipt(
        execution.clone(),
        hash,
        sender,
        &call,
        &[published_key(case)],
    ));
    assert_eq!(payment.tx_hash, hash);
    assert_eq!(payment.receipt, execution.receipt);
    assert_eq!(payment.logs.len(), 1);
    assert_eq!(payment.logs[0].topics(), execution.logs[0].topics);
    assert_eq!(payment.logs[0].data(), execution.logs[0].data);
}

#[test]
fn payer_payment_asset_refuses_node_revert() {
    let case = &producer()["reverted"];
    let execution = authenticated(case);
    assert!(!execution.success);
    assert_eq!(execution.vm_error, "execution reverted");
    assert!(execution.logs.is_empty());
    let (hash, sender, call) = payment_call(case);
    let mut false_success = execution.clone();
    false_success.success = true;
    assert_eq!(
        payer::receipt(false_success, hash, sender, &call, &[published_key(case)]),
        Err(WebPayerError::Refused)
    );
    assert_eq!(
        payer::receipt(execution, hash, sender, &call, &[published_key(case)]),
        Err(WebPayerError::Refused)
    );
}

#[test]
fn payer_payment_asset_retains_independent_bindings() {
    let case = &producer()["success"];
    let execution = authenticated(case);
    let (hash, sender, call) = payment_call(case);
    let key = published_key(case);
    let mut changed = Vec::new();
    let mut payment_denom = execution.clone();
    payment_denom.payment_denom.clone_from(&execution.fee_denom);
    changed.push(payment_denom);
    let mut fee_denom = execution.clone();
    fee_denom.fee_denom.clone_from(&execution.payment_denom);
    changed.push(fee_denom);
    let mut fee_amount = execution.clone();
    fee_amount.fee_amount[31] ^= 1;
    changed.push(fee_amount);
    let mut gas = execution.clone();
    gas.gas_used += 1;
    changed.push(gas);
    let mut signature = execution.clone();
    let last = signature.receipt.len() - 1;
    signature.receipt[last] ^= 1;
    changed.push(signature);
    let mut log = execution.clone();
    log.logs[0].data[0] ^= 1;
    changed.push(log);
    for invalid in changed {
        assert_eq!(
            payer::receipt(invalid, hash, sender, &call, &[key]),
            Err(WebPayerError::Refused)
        );
    }
    assert_eq!(
        payer::receipt(execution.clone(), hash, sender, &call, &[]),
        Err(WebPayerError::Refused)
    );
    let mut invalid_key = key;
    invalid_key.last_height = execution.height - 1;
    assert_eq!(
        payer::receipt(execution, hash, sender, &call, &[invalid_key]),
        Err(WebPayerError::Refused)
    );
}
