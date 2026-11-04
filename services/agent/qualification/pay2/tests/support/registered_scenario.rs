use super::{funded_inputs, funding};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_crypto::{
    account_derivation::derive_from_seed,
    disclosure::call::{bind_call, ConstructMethod},
    evm_transaction::{decode_signed, recover_sender, transaction_hash},
    payments::Payment,
    send::SendDebit,
    signer::{LocalSigner, Signer},
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::{
    rpc::{Commitment, RpcClient, RpcError},
    rpc_verification::ReceiptPolicy,
    wallet::{PaymentOptions, Wallet},
};
use serde_json::{json, Value};
use std::{
    future::Future,
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    path::PathBuf,
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc,
    },
    task::{Context, Poll, Wake, Waker},
    thread::JoinHandle,
    time::Duration,
};

pub(crate) struct Directory(pub(crate) PathBuf);

impl Directory {
    pub(crate) fn new() -> Self {
        let mut entropy = [0; 16];
        getrandom::fill(&mut entropy).expect("temporary directory entropy");
        let path = std::env::temp_dir().join(format!(
            "pay2-registered-{}",
            funded_inputs::hex_encode(&entropy)
        ));
        std::fs::create_dir(&path).expect("fresh scenario directory");
        Self(path)
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        std::fs::remove_dir_all(&self.0).expect("remove scenario directory");
    }
}

fn run<T>(future: impl Future<Output = T>) -> T {
    struct ThreadWake(std::thread::Thread);
    impl Wake for ThreadWake {
        fn wake(self: Arc<Self>) {
            self.0.unpark();
        }
    }
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(result) => return result,
            Poll::Pending => std::thread::park(),
        }
    }
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").expect("RPC hex prefix");
    assert!(text.is_ascii());
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|bytes| {
            u8::from_str_radix(std::str::from_utf8(bytes).expect("hex text"), 16)
                .expect("hex bytes")
        })
        .collect()
}

fn request(stream: &mut TcpStream) -> Value {
    let mut headers = Vec::new();
    while !headers.ends_with(b"\r\n\r\n") {
        assert!(headers.len() < 16_384);
        let mut byte = [0];
        stream.read_exact(&mut byte).expect("HTTP request headers");
        headers.push(byte[0]);
    }
    let length = std::str::from_utf8(&headers)
        .expect("HTTP text")
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            name.eq_ignore_ascii_case("content-length")
                .then(|| value.trim().parse::<usize>().expect("HTTP length"))
        })
        .expect("length-delimited request");
    assert!(length <= 1_048_576);
    let mut bytes = vec![0; length];
    stream.read_exact(&mut bytes).expect("HTTP request body");
    serde_json::from_slice(&bytes).expect("JSON-RPC request")
}

struct TransportPeer {
    url: String,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<usize>>,
}

impl TransportPeer {
    fn start(root: PathBuf, payer: [u8; 20], recipient: [u8; 20]) -> Self {
        let mut store = Store::open(&root).expect("real durable transport store");
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback transport");
        listener.set_nonblocking(true).expect("nonblocking accept");
        let url = format!("http://{}", listener.local_addr().expect("peer address"));
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = std::thread::spawn(move || {
            let mut submissions = 0;
            while !stopped.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("loopback accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("request deadline");
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .expect("response deadline");
                let request = request(&mut stream);
                let params = &request["params"];
                let result = match request["method"].as_str().expect("RPC method") {
                    "eth_getTransactionCount" => {
                        assert_eq!(params[1], "pending");
                        let address = unhex(params[0].as_str().expect("sender"));
                        if address == payer {
                            json!("0x25")
                        } else {
                            assert_eq!(address, recipient);
                            json!("0x5b")
                        }
                    }
                    "eth_chainId" => json!("0x7d"),
                    "eth_gasPrice" => json!("0x2"),
                    "eth_maxPriorityFeePerGas" => json!("0x1"),
                    "eth_estimateGas" => json!("0x186a0"),
                    "px_getAccount" => {
                        json!({"account_id": params[0], "next_sequence": "23"})
                    }
                    "eth_sendRawTransaction" => {
                        let bytes = unhex(params[0].as_str().expect("signed transaction"));
                        let (call, signature) = decode_signed(&bytes).expect("canonical EIP-1559");
                        let sender = recover_sender(&call, &signature).expect("real signature");
                        assert!(sender == payer || sender == recipient);
                        let disclosure = bind_call(&call, sender).expect("production consumer");
                        assert!(matches!(
                            disclosure.construct_method().expect("ledger method"),
                            Some(
                                ConstructMethod::LedgerSend { .. }
                                    | ConstructMethod::LedgerIssueGrant { .. }
                                    | ConstructMethod::LedgerDraw { .. }
                            )
                        ));
                        let hash = transaction_hash(&bytes);
                        let key = transaction_key(hash);
                        assert!(
                            store.get(&key).is_none(),
                            "each scenario message is distinct"
                        );
                        store.put_local(key, bytes).expect("durable signed message");
                        submissions += 1;
                        json!(format!("0x{}", funded_inputs::hex_encode(&hash)))
                    }
                    other => panic!("unexpected scenario RPC: {other}"),
                };
                let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).expect("RPC response");
            }
            submissions
        });
        Self {
            url,
            stop,
            worker: Some(worker),
        }
    }

    fn stop(mut self) -> usize {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .expect("running peer")
            .join()
            .expect("transport assertions")
    }
}

impl Drop for TransportPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            worker.join().expect("transport peer shutdown");
        }
    }
}

fn transaction_key(hash: [u8; 32]) -> TenantKey {
    TenantKey::new(
        TenantId::new("pay2-registered-scenario").expect("scenario tenant"),
        ObjectKind::Outbox,
        hash.to_vec(),
    )
    .expect("transaction key")
}

fn pending(result: Result<layerx_sdk::rpc_verification::VerifiedRpcReceipt, RpcError>) -> [u8; 32] {
    match result {
        Err(RpcError::Pending { activity_id }) => activity_id,
        Err(error) => panic!("expected transport acceptance awaiting receipt evidence: {error:?}"),
        Ok(_) => panic!("transport acceptance cannot produce verified receipt evidence"),
    }
}

pub(crate) fn pay2_signed_payments_survive_restart() {
    let directory = Directory::new();
    let root = directory.0.join("store");
    let payer = derive_from_seed(&[0x68; 32], 0).expect("payer derivation");
    let recipient = derive_from_seed(&[0x69; 32], 0).expect("recipient derivation");
    let payer_signer = LocalSigner::from_derived_account(&payer);
    let recipient_signer = LocalSigner::from_derived_account(&recipient);
    let payer_address = payer_signer.evm_address().expect("payer EVM key");
    let recipient_address = recipient_signer.evm_address().expect("recipient EVM key");
    let peer = TransportPeer::start(root.clone(), payer_address, recipient_address);
    let rpc = RpcClient::connect(&peer.url, None).expect("real SDK HTTP transport");
    let policy = ReceiptPolicy {
        protocol_version: funded_inputs::PROTOCOL_VERSION,
        network_id: funded_inputs::NETWORK_ID,
        sequencer: SequencerAuthorization::new([1; 32], [2; 32], 1, u64::MAX),
        trusted_checkpoint_context_digest: None,
    };
    let payer_wallet = Wallet {
        rpc: &rpc,
        signer: &payer_signer,
        policy: &policy,
        native_asset: [0x42; 32],
    };
    let recipient_wallet = Wallet {
        rpc: &rpc,
        signer: &recipient_signer,
        policy: &policy,
        native_asset: [0x42; 32],
    };
    let payer_did = payer.did();
    let recipient_did = recipient.did();
    let from = layerx_platform_core::main_account(&payer_did).expect("payer account");
    let to = layerx_platform_core::main_account(&recipient_did).expect("recipient account");
    let options = |actor: &str, key: u8| PaymentOptions {
        actor: actor.to_owned(),
        idempotency_key: [key; 32],
        fee_limit: 1_000_000,
        not_before: 1_000,
        not_after: 60_000,
        commitment: Commitment::Executed,
        wait_timeout: Duration::ZERO,
    };
    let send = SendDebit {
        from,
        to,
        asset: [0x42; 32],
        amount: 3,
        source_sequence: 23,
        idempotency_key: [3; 32],
        expires_at: 60_000,
        context_hash: layerx_platform_core::send_context_hash(&from, &to, &[0x42; 32], 3, &[3; 32]),
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: funded_inputs::NETWORK_ID,
        protocol_version: funded_inputs::PROTOCOL_VERSION,
    };
    let send_payload = run(send.sign(&payer_signer)).expect("real SEND authorization producer");
    let send_id = pending(run(payer_wallet.send_debit(&send, &options(&payer_did, 3))));
    let grant = funding::payer_grant(&funding::GrantRequest {
        payer_seed: payer.layerx_seed(),
        payer_did: &payer_did,
        recipient_did: &recipient_did,
        asset: [0x42; 32],
        per_draw_maximum: 5,
        allowance: 10,
        recurring_window: None,
        expiration: 300_000,
        purpose_hash: [4; 32],
        revocation_sequence: 37,
    })
    .expect("real signed grant producer");
    let issue_id = pending(run(
        payer_wallet.issue(grant.clone(), &options(&payer_did, 4))
    ));
    let receive = funding::receive(recipient.layerx_seed(), &grant, 23, [5; 32], 2)
        .expect("real signed Receive producer");
    let payload = receive
        .encode(recipient_did.as_bytes())
        .expect("Receive producer codec");
    assert_eq!(
        Payment::decode(
            layerx_types::payload::ModuleId::Asset,
            6,
            &payload,
            recipient_did.as_bytes()
        )
        .expect("payer and receiver signature consumer"),
        receive
    );
    let mut tampered = payload.clone();
    tampered[200] ^= 1;
    assert!(Payment::decode(
        layerx_types::payload::ModuleId::Asset,
        6,
        &tampered,
        recipient_did.as_bytes()
    )
    .is_err());
    let draw_id = pending(run(
        recipient_wallet.draw(receive, &options(&recipient_did, 5))
    ));
    assert_eq!(peer.stop(), 3);

    let reopened = Store::open(&root).expect("restart: reopen the real durable consumer store");
    let ids = [send_id, issue_id, draw_id];
    let recovered: Vec<_> = ids
        .iter()
        .map(|id| {
            let bytes = reopened
                .get(&transaction_key(*id))
                .expect("recovered signed transaction")
                .bytes()
                .to_vec();
            assert_eq!(transaction_hash(&bytes), *id);
            let (call, signature) = decode_signed(&bytes).expect("recovered transaction codec");
            let sender =
                recover_sender(&call, &signature).expect("recovered transaction signature");
            assert_eq!(call.chain_id, 125);
            assert_eq!(&call.to[18..], &[0x10, 0x1a]);
            assert!(call.to[..18].iter().all(|byte| *byte == 0));
            assert_eq!(call.nonce, if *id == draw_id { 91 } else { 37 });
            assert_ne!(
                call.nonce, 23,
                "identity nonce and account sequence are independent"
            );
            assert_eq!(
                sender,
                if *id == draw_id {
                    recipient_address
                } else {
                    payer_address
                }
            );
            bind_call(&call, sender)
                .expect("recovered production disclosure")
                .construct_method()
                .expect("recovered ledger ABI")
                .expect("ledger method")
        })
        .collect();
    assert_eq!(
        recovered[0],
        ConstructMethod::LedgerSend {
            payload: send_payload
        }
    );
    assert_eq!(
        recovered[1],
        ConstructMethod::LedgerIssueGrant {
            grant: grant.clone()
        }
    );
    let context_hash = match Payment::decode(
        layerx_types::payload::ModuleId::Asset,
        6,
        &payload,
        recipient_did.as_bytes(),
    )
    .expect("recovered receive authorization")
    {
        Payment::Receive { context_hash, .. } => context_hash,
        _ => panic!("Receive required"),
    };
    assert_eq!(
        recovered[2],
        ConstructMethod::LedgerDraw {
            recipient: to,
            purpose_hash: grant.purpose_hash,
            amount: 2,
            grant: grant.id,
            receiver_sequence: 23,
            idempotency_key: [5; 32],
            context_hash,
        }
    );
    assert_eq!(grant.allowance, 10);
    assert_eq!(grant.per_draw_maximum, 5);
    assert_eq!(grant.from, from);
    assert_eq!(grant.recipient, to);
    assert_eq!(grant.asset, [0x42; 32]);
    let retained: Vec<_> = ids
        .iter()
        .map(|id| {
            reopened
                .get(&transaction_key(*id))
                .expect("retained bytes")
                .bytes()
                .to_vec()
        })
        .collect();
    drop(reopened);
    let recovered_again = Store::open(&root).expect("second store restart");
    for (id, bytes) in ids.iter().zip(retained) {
        assert_eq!(
            recovered_again
                .get(&transaction_key(*id))
                .expect("byte-identical recovery")
                .bytes(),
            bytes
        );
    }
}
