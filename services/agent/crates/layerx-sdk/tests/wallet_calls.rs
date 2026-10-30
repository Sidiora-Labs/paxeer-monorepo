use std::future::Future;
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::{bind_call, ledger_selector, ConstructMethod};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::ReceiptPolicy;
use layerx_sdk::wallet::{PaymentOptions, Wallet};
use layerx_types::clock::ClockError;
use layerx_types::result::{KnownResult, Retriability};
use serde_json::{json, Value};

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .unwrap_or(text)
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex bytes")
        })
        .collect()
}
fn run<T>(future: impl Future<Output = T>) -> T {
    struct ThreadWake;
    impl Wake for ThreadWake {
        fn wake(self: Arc<Self>) {
            thread::current().unpark();
        }
    }
    let waker = Waker::from(Arc::new(ThreadWake));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(result) => return result,
            Poll::Pending => thread::park_timeout(Duration::from_millis(1)),
        }
    }
}

// A bounded HTTP peer exercises the real RPC transport, codecs, signer and wallet.
fn listener(
    methods: Vec<&'static str>,
    refusal: Option<(&'static str, &'static str)>,
) -> (RpcClient, JoinHandle<Vec<Value>>) {
    listener_with_rungs(methods, refusal, vec!["receipted"])
}

fn listener_with_rungs(
    methods: Vec<&'static str>,
    refusal: Option<(&'static str, &'static str)>,
    rungs: Vec<&'static str>,
) -> (RpcClient, JoinHandle<Vec<Value>>) {
    let listener = TcpListener::bind("127.0.0.1:0").expect("loopback listener");
    let endpoint = format!(
        "http://{}/rpc",
        listener.local_addr().expect("local address")
    );
    listener.set_nonblocking(true).expect("nonblocking");
    let server = thread::spawn(move || {
        let mut requests = Vec::new();
        let mut submitted = None;
        let mut rungs = rungs.into_iter();
        for method in methods {
            let deadline = Instant::now() + Duration::from_secs(10);
            let mut socket = loop {
                match listener.accept() {
                    Ok((socket, _)) => break socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "missing {method}");
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("accept: {error}"),
                }
            };
            socket
                .set_read_timeout(Some(Duration::from_secs(5)))
                .expect("read timeout");
            socket
                .set_write_timeout(Some(Duration::from_secs(5)))
                .expect("write timeout");
            let mut header = Vec::new();
            while !header.ends_with(b"\r\n\r\n") {
                assert!(header.len() < 8192);
                let mut byte = [0];
                socket.read_exact(&mut byte).expect("header");
                header.push(byte[0]);
            }
            let length = std::str::from_utf8(&header)
                .expect("HTTP text")
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().expect("length"))
                })
                .expect("content length");
            assert!(length <= 1_048_576 + 4096);
            let mut body = vec![0; length];
            socket.read_exact(&mut body).expect("body");
            let request: Value = serde_json::from_slice(&body).expect("JSON request");
            assert_eq!(request["method"], method);
            let result = match method {
                "eth_getTransactionCount" => {
                    assert_eq!(request["params"][1], "pending");
                    json!("0x7")
                }
                "eth_chainId" => json!("0x7d"),
                "eth_gasPrice" => json!("0x2"),
                "eth_maxPriorityFeePerGas" => json!("0x1"),
                "eth_estimateGas" => json!("0x186a0"),
                "px_getAccount" => json!({"account_id": "01".repeat(32), "next_sequence": "7"}),
                "eth_sendRawTransaction" => {
                    let raw = unhex(request["params"][0].as_str().expect("raw transaction"));
                    let hash = format!("0x{}", hex(&transaction_hash(&raw)));
                    submitted = Some(hash.clone());
                    json!(hash)
                }
                "pax_sequencerReceipt" => {
                    let hash = submitted.as_ref().expect("submitted first");
                    assert_eq!(request["params"], json!([hash]));
                    let rung = rungs.next().expect("sequencer rung");
                    if let Some((at, reason)) = refusal {
                        assert_eq!(at, method);
                        json!({"txHash": hash, "rung": rung, "status": "0x0", "returnData": error_data(reason)})
                    } else {
                        json!({"txHash": hash, "rung": rung, "status": "0x1"})
                    }
                }
                _ => panic!("unexpected method"),
            };
            let response = if refusal
                .is_some_and(|(at, _)| at == method && at != "pax_sequencerReceipt")
            {
                json!({"jsonrpc": "2.0", "id": request["id"], "error": {"code": -32000, "message": "execution reverted", "data": error_data(refusal.expect("refusal").1)}})
            } else {
                json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
            };
            let body = serde_json::to_vec(&response).expect("response JSON");
            write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).expect("response header");
            socket.write_all(&body).expect("response body");
            requests.push(request);
        }
        requests
    });
    (
        RpcClient::connect(&endpoint, None).expect("RPC client"),
        server,
    )
}
fn error_data(reason: &str) -> String {
    let mut data = vec![0x08, 0xc3, 0x79, 0xa0];
    let mut offset = [0; 32];
    offset[31] = 32;
    data.extend(offset);
    let mut length = [0; 32];
    length[24..].copy_from_slice(&(reason.len() as u64).to_be_bytes());
    data.extend(length);
    data.extend(reason.as_bytes());
    data.resize(4 + (data.len() - 4).div_ceil(32) * 32, 0);
    format!("0x{}", hex(&data))
}
fn signer() -> LocalSigner {
    LocalSigner::from_derived_account(&derive_from_seed(&[0x42; 64], 0).expect("derived account"))
}
fn policy() -> ReceiptPolicy {
    ReceiptPolicy {
        protocol_version: 3,
        network_id: 125,
        sequencer: SequencerAuthorization::new([1; 32], [2; 32], 1, 100),
        trusted_checkpoint_context_digest: None,
    }
}
fn options() -> PaymentOptions {
    PaymentOptions {
        actor: "did:layerx:alice".into(),
        idempotency_key: [4; 32],
        fee_limit: 1_000_000,
        not_before: 0,
        not_after: 100,
        commitment: Commitment::Executed,
        wait_timeout: Duration::ZERO,
    }
}
fn preparation() -> Vec<&'static str> {
    vec![
        "eth_getTransactionCount",
        "eth_chainId",
        "eth_gasPrice",
        "eth_maxPriorityFeePerGas",
        "eth_estimateGas",
    ]
}

#[test]
fn send_signs_a_layerxledger_send_call_with_a_65_byte_signature() {
    let payload =
        unhex(include_str!("../../layerx-crypto/tests/fixtures/payments/native-1-5.hex").trim());
    let mut methods = preparation();
    methods.extend(["px_getAccount", "eth_sendRawTransaction"]);
    let (rpc, server) = listener(methods, None);
    let signer = signer();
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let hash = match run(wallet.send(&payload, &options())) {
        Err(RpcError::Pending { activity_id }) => activity_id,
        _ => panic!("submitted hash must remain pending without receipt evidence"),
    };
    let requests = server.join().expect("RPC peer");
    let writes: Vec<_> = requests
        .iter()
        .filter(|request| request["method"] == "eth_sendRawTransaction")
        .collect();
    assert_eq!(writes.len(), 1);
    let raw = unhex(writes[0]["params"][0].as_str().expect("raw bytes"));
    assert_eq!(raw[0], 2);
    assert_eq!(transaction_hash(&raw), hash);
    let (call, signature) = decode_signed(&raw).expect("EIP-1559 transaction");
    assert_eq!(signature.as_bytes().len(), 65);
    assert_eq!(
        recover_sender(&call, &signature).expect("recover sender"),
        signer.evm_address().expect("wallet address")
    );
    assert_eq!(call.to, [vec![0; 18], vec![0x10, 0x1a]].concat().as_slice());
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(call.gas_limit, 100_000);
    assert_eq!(call.max_fee_per_gas, 5);
    assert_eq!(&call.data[..4], &[0x2b, 0xb1, 0xae, 0x7c]);
    assert_eq!(
        bind_call(&call, signer.evm_address().expect("address"))
            .expect("disclosure")
            .construct_method()
            .expect("method"),
        Some(ConstructMethod::LedgerSend { payload })
    );
    assert_eq!(
        requests[0]["params"][0],
        format!("0x{}", hex(&signer.evm_address().expect("address")))
    );
    assert_eq!(requests[4]["params"][0]["from"], requests[0]["params"][0]);
}

#[test]
fn open_account_encodes_the_open_account_selector() {
    let mut methods = preparation();
    methods.push("eth_sendRawTransaction");
    let (rpc, server) = listener(methods, None);
    let signer = signer();
    let policy = policy();
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let hash = match run(wallet.open_account([0x33; 32], &options())) {
        Err(RpcError::Pending { activity_id }) => activity_id,
        _ => panic!("submitted hash"),
    };
    let requests = server.join().expect("RPC peer");
    let raw = unhex(
        requests.last().expect("submission")["params"][0]
            .as_str()
            .expect("raw"),
    );
    assert_eq!(hash, transaction_hash(&raw));
    let (call, signature) = decode_signed(&raw).expect("signed call");
    assert_eq!(
        call.data,
        [
            ledger_selector("openAccount(bytes32)").to_vec(),
            vec![0x33; 32]
        ]
        .concat()
    );
    assert_eq!(&call.to[18..], &[0x10, 0x1a]);
    assert!(call.to[..18].iter().all(|byte| *byte == 0));
    assert_eq!(
        recover_sender(&call, &signature).expect("sender"),
        signer.evm_address().expect("address")
    );
}

#[test]
fn submitted_revert_maps_through_the_registry() {
    for (at, reason, expected) in [
        (
            "pax_sequencerReceipt",
            "LXP_INSUFFICIENT_BALANCE: insufficient balance",
            Retriability::Retriable,
        ),
        (
            "eth_sendRawTransaction",
            "LXP_SEQUENCE_REUSED: sequence reused",
            Retriability::Terminal,
        ),
        (
            "eth_estimateGas",
            "LXP_INSUFFICIENT_BALANCE: insufficient balance",
            Retriability::Retriable,
        ),
    ] {
        let mut methods = preparation();
        if at != "eth_estimateGas" {
            methods.push("eth_sendRawTransaction");
        }
        if at == "pax_sequencerReceipt" {
            methods.push(at);
        }
        let (rpc, server) = listener(methods, Some((at, reason)));
        let signer = signer();
        let policy = policy();
        let wallet = Wallet {
            rpc: &rpc,
            signer: &signer,
            policy: &policy,
            native_asset: [3; 32],
        };
        let mut options = options();
        options.wait_timeout = Duration::from_secs(2);
        match run(wallet.open_account([3; 32], &options)) {
            Err(RpcError::Reverted {
                code,
                reason: actual,
                result,
                retriability,
            }) => {
                assert_eq!(code, reason.split_once(": ").expect("code").0);
                assert_eq!(actual, reason);
                assert_eq!(result, KnownResult::from_revert_reason(reason));
                assert!(result.is_some());
                assert_eq!(retriability, expected);
            }
            _ => panic!("registry-classified refusal from {at}"),
        }
        let requests = server.join().expect("RPC peer");
        assert_eq!(requests.last().expect("request")["method"], at);
    }
}

#[test]
fn receipted_rung_waits_on_to_committed() {
    for (commitment, rungs) in [
        (
            Commitment::Batched,
            vec!["submitted", "receipted", "committed"],
        ),
        (Commitment::Executed, vec!["submitted", "receipted"]),
    ] {
        let mut methods = preparation();
        methods.push("eth_sendRawTransaction");
        methods.extend(vec!["pax_sequencerReceipt"; rungs.len()]);
        let expected_requests = methods.len();
        let (rpc, server) = listener_with_rungs(methods, None, rungs);
        let signer = signer();
        let policy = policy();
        let wallet = Wallet {
            rpc: &rpc,
            signer: &signer,
            policy: &policy,
            native_asset: [3; 32],
        };
        let mut options = options();
        options.commitment = commitment;
        options.wait_timeout = Duration::from_secs(2);
        // The RPC verifier requires an explicit clock; reaching its refusal proves delegation.
        assert!(matches!(
            run(wallet.open_account([3; 32], &options)),
            Err(RpcError::Clock(ClockError::Unavailable))
        ));
        let requests = server.join().expect("RPC peer");
        assert_eq!(requests.len(), expected_requests);
        assert_eq!(
            requests.last().expect("committed or receipted answer")["method"],
            "pax_sequencerReceipt"
        );
    }
}

#[test]
fn receipted_rung_revert_maps_through_the_registry() {
    for commitment in [Commitment::Executed, Commitment::Batched] {
        let reason = "LXP_INSUFFICIENT_BALANCE: insufficient balance";
        let mut methods = preparation();
        methods.extend(["eth_sendRawTransaction", "pax_sequencerReceipt"]);
        let (rpc, server) = listener(methods, Some(("pax_sequencerReceipt", reason)));
        let signer = signer();
        let policy = policy();
        let wallet = Wallet {
            rpc: &rpc,
            signer: &signer,
            policy: &policy,
            native_asset: [3; 32],
        };
        let mut options = options();
        options.commitment = commitment;
        options.wait_timeout = Duration::from_secs(2);
        match run(wallet.open_account([3; 32], &options)) {
            Err(RpcError::Reverted {
                code,
                reason: actual,
                result,
                retriability,
            }) => {
                assert_eq!(code, "LXP_INSUFFICIENT_BALANCE");
                assert_eq!(actual, reason);
                assert_eq!(result, KnownResult::from_revert_reason(reason));
                assert!(result.is_some());
                assert_eq!(retriability, Retriability::Retriable);
            }
            _ => panic!("receipted revert must be classified before commitment verification"),
        }
        let requests = server.join().expect("RPC peer");
        assert_eq!(
            requests.last().expect("receipt request")["method"],
            "pax_sequencerReceipt"
        );
    }
}
