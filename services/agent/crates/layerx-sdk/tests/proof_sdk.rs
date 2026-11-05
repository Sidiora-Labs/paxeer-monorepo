//! Proves: Wallet::{create,execute} -> RpcClient / paxd EVM RPC -> layerx-wallet-rpc::{submit,wait}.
//!
//! Starts tools/qualification/localnode/v6120-node.sh from this repository, using the
//! node installed by the declared build. All endpoints and keys come from files the
//! harness creates in this test's private directory. A real account is funded, bound,
//! and used to register an asset and open its account. The node is stopped and its
//! durable home reopened before fresh SDK and bridge consumers read the same state.
//!
//! Repairs: rpc.rs::estimate_fee_from omitted estimate_fee's calldata limit, and
//! production.rs omitted the contracted verify_receipt entry point. Wallet, MCP, CLI
//! and bridge preparation now share the same calldata bound. RpcClient::connect_node
//! repairs direct-node routing: connect appended the gateway-only `/rpc` suffix,
//! so node calls received HTTP 404 before any envelope could be decoded. The bridge
//! selects the same explicit node route with configuration.rpc_route = "node".
//! RpcClient::estimate_fee now documents that maxFeePerGas triggers the node's
//! payer balance check: the proof checks the funded balance at latest and uses
//! RpcClient::estimate_fee_from with that payer rather than an omitted sender.
//! In the separate version-3 policy mismatch regression, after Wallet::create refuses the sequencer receipt, the proof waits for the
//! committed nonce before reading the registered asset at latest.
//!
//! The positive in-process proof below uses a pinned version-2 policy and reopened
//! protocol peer for Wallet::create and wait_for. The real-node version mismatch
//! regression retains its explicit version-3 refusal.
//! rpc/sequencer_receipt.go::sequencerReceiptView publishes version-2
//! EVM summary receipts. Wallet::{create,wait_for} and bridge wait refuse them under
//! the version-3 receipt policy while successful EVM mutations remain visible.
//! services/agent/crates/layerx-proof/src/receipt.rs::verify_outcome accepts version 2
//! at line 344 and checks the authorized asset at line 375. production::verify_receipt
//! must economically verify the node-produced summary under its pinned key when
//! the batch carries the decoded asset, batch id and state roots, and refuse a
//! different asset at ReceiptCheck::Asset without changing the external producer.

use std::fs::{self, File, OpenOptions};
use std::future::Future;
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc,
};
use std::task::{Context, Poll, Wake, Waker};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::encode_payment_call;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_crypto::payments::{asset_id, Payment, Registration};
use layerx_crypto::signer::{LocalSigner, SignError};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{AuthorizedBatch, ReceiptCheck};
use layerx_sdk::production::verify_receipt;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError, MAX_TRANSACTION_CALLDATA_BYTES};
use layerx_sdk::rpc_verification::ReceiptPolicy;
use layerx_sdk::wallet::{PaymentOptions, Wallet};
use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::payload::ModuleId;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn run<F: Future>(future: F) -> F::Output {
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
            Poll::Ready(value) => return value,
            Poll::Pending => std::thread::park_timeout(Duration::from_millis(1)),
        }
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicUsize = AtomicUsize::new(0);
        let root = PathBuf::from(format!(
            "/tmp/proof-sdk-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::DirBuilder::new()
            .mode(0o700)
            .create(&root)
            .expect("private proof directory");
        Self(root)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        if !std::thread::panicking() {
            let _ = fs::remove_dir_all(&self.0);
        }
    }
}

fn call() -> Eip1559Call {
    let mut to = [0; 20];
    to[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
    Eip1559Call {
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to,
        data: encode_payment_call(
            &Payment::OpenAccount { asset: [3; 32] },
            b"did:layerx:alice",
        )
        .expect("real payment calldata producer"),
    }
}

fn configuration(endpoint: &str) -> Value {
    json!({"endpoint": endpoint, "protocol_version": "3", "network_id": "125",
        "native_asset": "03".repeat(32), "sequencer_id": "01".repeat(32),
        "sequencer_key": "02".repeat(32), "first_batch": "1", "last_batch": "100"})
}

fn bridge(request: &Value) -> (bool, Value) {
    let mut child = Command::new(env!("CARGO_BIN_EXE_layerx-wallet-rpc"))
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("services/agent/crates/layerx-sdk/src/bin/layerx-wallet-rpc.rs producer");
    child
        .stdin
        .take()
        .expect("bridge input")
        .write_all(&serde_json::to_vec(request).expect("bridge request JSON"))
        .expect("real bridge stdin transport");
    let deadline = Instant::now() + Duration::from_secs(5);
    loop {
        if child.try_wait().expect("bridge status").is_some() {
            break;
        }
        if Instant::now() >= deadline {
            child.kill().expect("stop stalled bridge");
            let _ = child.wait();
            panic!("layerx-wallet-rpc consumer exceeded its deadline");
        }
        std::thread::sleep(Duration::from_millis(2));
    }
    let output = child.wait_with_output().expect("closed bridge process");
    (
        output.status.success(),
        serde_json::from_slice(&output.stdout).expect("consumer reply"),
    )
}

#[test]
fn proof_sdk_signed_transaction_reopens_and_reaches_the_real_bridge() {
    let root = Directory::new();
    let transaction = call().sign(&[0x42; 32]).expect("real EIP-1559 signer");
    let path = root.0.join("transaction");
    let mut producer = OpenOptions::new()
        .create_new(true)
        .write(true)
        .mode(0o600)
        .open(&path)
        .expect("create producer state");
    producer
        .write_all(&transaction)
        .expect("persist signed transaction");
    producer.sync_all().expect("durable producer bytes");
    drop(producer);
    let reopened = fs::read(&path).expect("reopen producer state");
    assert_eq!(reopened, transaction);
    assert_eq!(call().sign(&[0x42; 32]).expect("repeat producer"), reopened);
    assert_eq!(
        OpenOptions::new()
            .create_new(true)
            .write(true)
            .open(&path)
            .expect_err("duplicate producer state must be refused")
            .kind(),
        std::io::ErrorKind::AlreadyExists
    );
    let (decoded, signature) = decode_signed(&reopened).expect("real transaction decoder");
    assert_eq!(decoded, call());
    let sender = recover_sender(&decoded, &signature).expect("real transaction recovery");
    assert_ne!(sender, [0; 20]);
    let listener = TcpListener::bind("127.0.0.1:0").expect("local no-traffic boundary");
    listener
        .set_nonblocking(true)
        .expect("nonblocking boundary");
    let endpoint = format!("http://{}", listener.local_addr().expect("bound address"));
    let hash = format!("0x{}", hex(&transaction_hash(&reopened)));
    let request = json!({"action": "wait", "configuration": configuration(&endpoint),
        "txHash": hash, "rung": "submitted", "timeout_ms": "1000"});
    let expected = json!({"ok":true,"result":{"txHash":hash,"rung":"submitted"}});
    assert_eq!(bridge(&request), (true, expected.clone()));
    // bridge() has reaped the process; this invocation starts a fresh consumer.
    drop(File::open(&path).expect("reopen durable producer before consumer restart"));
    assert_eq!(bridge(&request), (true, expected));
    let mut malformed = request;
    malformed["txHash"] = json!("0x00");
    let (success, observed) = bridge(&malformed);
    assert!(!success);
    assert_eq!(
        observed,
        json!({"ok":false,"error":{"kind":"wallet_request_refused_or_unavailable"}})
    );
    assert_eq!(
        listener
            .accept()
            .expect_err("submitted acknowledgement must not claim a node read")
            .kind(),
        std::io::ErrorKind::WouldBlock
    );
}

#[test]
fn proof_sdk_public_wallet_refusals_do_not_cross_the_transport() {
    let listener = TcpListener::bind("127.0.0.1:0").expect("local refusal boundary");
    listener
        .set_nonblocking(true)
        .expect("nonblocking boundary");
    let rpc = RpcClient::connect(
        &format!("http://{}", listener.local_addr().expect("address")),
        None,
    )
    .expect("real RpcClient");
    assert!(matches!(
        rpc.get_identity_sequence("did:layerx:alice"),
        Err(RpcError::InvalidRequest)
    ));
    let mut oversized = call();
    oversized.data.resize(MAX_TRANSACTION_CALLDATA_BYTES + 1, 0);
    assert!(matches!(
        rpc.estimate_fee(&oversized),
        Err(RpcError::InvalidRequest)
    ));
    assert!(matches!(
        rpc.estimate_fee_from(&oversized, [1; 20]),
        Err(RpcError::InvalidRequest)
    ));
    let signer = LocalSigner::new([0x42; 32]);
    let policy = ReceiptPolicy {
        protocol_version: 3,
        network_id: 125,
        sequencer: SequencerAuthorization::new([1; 32], [2; 32], 1, 100),
        trusted_checkpoint_context_digest: None,
    };
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let options = PaymentOptions {
        actor: "did:layerx:alice".into(),
        idempotency_key: [4; 32],
        fee_limit: 1_000_000,
        not_before: 0,
        not_after: 100,
        commitment: Commitment::Executed,
        wait_timeout: Duration::from_secs(1),
    };
    assert!(matches!(
        wallet.prepare_payload(ModuleId::Asset, 1, &oversized.data, &options),
        Err(RpcError::InvalidRequest)
    ));
    let registration = Registration {
        asset: [3; 32],
        salt: [4; 32],
        symbol: String::new(),
        name: "SDK proof".into(),
        decimals: 6,
        supply_cap: 1_000,
        issuer_kind: 1,
        custody_ref: Vec::new(),
    };
    assert!(matches!(
        run(wallet.create(registration, &options)),
        Err(RpcError::InvalidRequest)
    ));
    assert!(matches!(
        run(wallet.mint([3; 32], [4; 32], 1, &options)),
        Err(RpcError::Signing(SignError::EvmKeyUnavailable))
    ));
    assert!(matches!(
        wallet.wait_for([0; 32], Commitment::Executed, Duration::from_secs(1)),
        Err(RpcError::InvalidRequest)
    ));
    assert!(
        matches!(wallet.wait_for([9; 32], Commitment::Executed, Duration::ZERO),
        Err(RpcError::Pending { activity_id }) if activity_id == [9; 32])
    );
    let authority = AuthorizedBatch::new([1; 32], [3; 32], [4; 32], [5; 32], [2; 32]);
    assert!(verify_receipt(&[], &authority).is_err());
    assert_eq!(
        listener
            .accept()
            .expect_err("local refusals must not contact a replacement producer")
            .kind(),
        std::io::ErrorKind::WouldBlock
    );
}

// The clock measures actual OS time; no simulated readings or environment binding.
struct OsClock {
    origin: Instant,
    generation: [u8; 16],
}
impl OsClock {
    fn new() -> Self {
        let mut generation = [0; 16];
        File::open("/dev/urandom")
            .expect("OS entropy")
            .read_exact(&mut generation)
            .expect("clock generation");
        assert_ne!(generation, [0; 16]);
        Self {
            origin: Instant::now(),
            generation,
        }
    }
}
impl Clock for OsClock {
    fn sample(&self, budget: Duration) -> Result<ClockReading, ClockError> {
        if budget.is_zero() {
            return Err(ClockError::Unavailable);
        }
        Ok(ClockReading {
            generation: self.generation,
            unix_milliseconds: SystemTime::now()
                .duration_since(UNIX_EPOCH)
                .map_err(|_| ClockError::Regression)?
                .as_millis()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
            monotonic_nanoseconds: self
                .origin
                .elapsed()
                .as_nanos()
                .try_into()
                .map_err(|_| ClockError::Overflow)?,
        })
    }
}

fn repository() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .expect("repository root")
        .to_owned()
}

fn generated_setting(path: &Path, key: &str) -> String {
    let document = Zeroizing::new(fs::read_to_string(path).expect("harness-generated settings"));
    let value = document
        .lines()
        .find_map(|line| line.strip_prefix(&format!("{key}=")))
        .expect("generated setting exists");
    // The requested values are paths, hex or loopback URLs: bash %q emits these
    // without quoting. Refuse shell syntax instead of evaluating the env file.
    assert!(
        !value.is_empty()
            && value
                .bytes()
                .all(|b| b.is_ascii_alphanumeric() || b"/._:-".contains(&b)),
        "unexpected generated setting encoding"
    );
    value.to_owned()
}

struct LocalNode {
    directory: Directory,
    home: PathBuf,
    endpoint: String,
    executable: PathBuf,
    restarted: Option<Child>,
}
impl LocalNode {
    fn script(&self) -> Command {
        let mut command = Command::new("bash");
        command
            .current_dir(repository())
            .arg("tools/qualification/localnode/v6120-node.sh")
            .env("TMPDIR", &self.directory.0)
            .env_remove("PAXD");
        command
    }
    fn start() -> Self {
        let directory = Directory::new();
        let env_path = directory.0.join("lx-localnode/localnode.env");
        let log_path = directory.0.join("startup.log");
        let log = File::create(&log_path).expect("private startup evidence");
        let mut node = Self {
            directory,
            home: PathBuf::new(),
            endpoint: String::new(),
            executable: PathBuf::new(),
            restarted: None,
        };
        let status = node.script().args(["--sequencer", "--env-out"]).arg(&env_path)
            .stdout(log.try_clone().expect("log clone")).stderr(log).status()
            .expect("missing producer tools/qualification/localnode/v6120-node.sh (build: make -f chain.mk install)");
        assert!(status.success(), "producer tools/qualification/localnode/v6120-node.sh failed: {status}; private evidence {}", log_path.display());
        node.home = generated_setting(&env_path, "LOCAL_NODE_HOME").into();
        node.endpoint = generated_setting(&env_path, "LOCAL_NODE_RPC");
        assert!(node.home.starts_with(node.directory.0.join("lx-localnode")));
        node.executable = generated_setting(&node.home.join("process.env"), "PAXD").into();
        assert!(
            node.executable.is_file(),
            "missing installed producer daemon/paxd/main.go"
        );
        node
    }
    fn rpc(&self) -> RpcClient {
        RpcClient::connect_node(&self.endpoint, None)
            .expect("real node RpcClient")
            .with_clock(Arc::new(OsClock::new()))
    }
    fn stop(&mut self) {
        if self.home.as_os_str().is_empty() {
            return;
        }
        assert!(self
            .script()
            .arg("--stop")
            .arg(&self.home)
            .status()
            .expect("stop owned node")
            .success());
        if let Some(mut child) = self.restarted.take() {
            child.wait().expect("reap restarted node");
        }
    }
    fn restart(&mut self) {
        self.stop();
        let log = OpenOptions::new()
            .append(true)
            .open(self.home.join("node.log"))
            .expect("restart evidence");
        let child = Command::new(&self.executable)
            .arg("start")
            .arg("--home")
            .arg(&self.home)
            .args([
                "--log_level",
                "info",
                "--sequencer.enable",
                "--sequencer.mode",
                "sequential",
                "--sequencer.replication-timeout",
                "10s",
                "--sequencer.lease-duration",
                "30s",
                "--sequencer.nonce-queue-max-per-sender",
                "16",
                "--sequencer.nonce-queue-max-total",
                "128",
                "--sequencer.role",
                "primary",
                "--sequencer.wal-dir",
            ])
            .arg(self.home.join("sequencer-wal"))
            .arg("--sequencer.receipt-key-file")
            .arg(self.home.join("config/receipt-key.hex"))
            .stdin(Stdio::null())
            .stdout(log.try_clone().expect("log clone"))
            .stderr(log)
            .spawn()
            .expect("restart installed paxd");
        fs::write(self.home.join("node.pid"), child.id().to_string()).expect("owned restart pid");
        self.restarted = Some(child);
        let deadline = Instant::now() + Duration::from_secs(60);
        loop {
            if self.rpc().eip1559_parameters().is_ok() {
                break;
            }
            assert!(
                Instant::now() < deadline,
                "reopened node unavailable; evidence {}",
                self.home.join("node.log").display()
            );
            assert!(
                self.restarted
                    .as_mut()
                    .expect("restart child")
                    .try_wait()
                    .expect("restart status")
                    .is_none(),
                "reopened node exited; evidence {}",
                self.home.join("node.log").display()
            );
            std::thread::sleep(Duration::from_millis(100));
        }
    }
    fn fund(&self, address: &str) {
        let output = Command::new(&self.executable)
            .args(["tx", "evm", "send", address, "100000000000000000000"])
            .args(["--from", "test", "--home"])
            .arg(&self.home)
            .args([
                "--keyring-backend",
                "test",
                "--evm-rpc",
                &self.endpoint,
                "--gas-fee-cap",
                "1000000000000",
                "--gas-limit",
                "21000",
            ])
            .output()
            .expect("real funding producer");
        assert!(
            output.status.success(),
            "real funding failed: {}",
            String::from_utf8_lossy(&output.stderr)
        );
        let text = String::from_utf8(output.stdout).expect("funding output");
        let hash = text
            .lines()
            .find_map(|line| line.strip_prefix("Transaction hash: "))
            .expect("funding transaction hash");
        mined(&self.rpc(), &self.endpoint, hash);
    }
}
impl Drop for LocalNode {
    fn drop(&mut self) {
        // Keep cleanup non-panicking even when an assertion failed.
        if !self.home.as_os_str().is_empty() {
            let _ = self.script().arg("--stop").arg(&self.home).status();
            if let Some(mut child) = self.restarted.take() {
                let _ = child.wait();
            }
        }
    }
}

fn receipt_diagnostic(endpoint: &str, hash: &str) {
    let endpoint = url::Url::parse(endpoint).expect("owned node endpoint");
    let request = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": "receipt-diagnostic",
        "method": "eth_getTransactionReceipt", "params": [hash]
    }))
    .expect("receipt diagnostic request");
    match ureq::post(endpoint.as_str())
        .header("Content-Type", "application/json")
        .config()
        .http_status_as_error(false)
        .timeout_global(Some(Duration::from_secs(5)))
        .build()
        .send(request.as_slice())
    {
        Ok(mut response) => {
            eprintln!(
                "eth_getTransactionReceipt diagnostic HTTP {}: {:?}",
                response.status(),
                response.headers()
            );
            match response.body_mut().read_to_vec() {
                Ok(bytes) => eprintln!(
                    "eth_getTransactionReceipt raw envelope: {}",
                    String::from_utf8_lossy(&bytes)
                ),
                Err(error) => {
                    eprintln!("eth_getTransactionReceipt diagnostic body failed: {error}")
                }
            }
        }
        Err(error) => eprintln!("eth_getTransactionReceipt diagnostic request failed: {error}"),
    }
}

fn mined(rpc: &RpcClient, endpoint: &str, hash: &str) -> Value {
    let deadline = Instant::now() + Duration::from_secs(60);
    loop {
        let receipt = match rpc.get_receipt(hash) {
            Ok(receipt) => receipt,
            Err(RpcError::InvalidResponse) => {
                receipt_diagnostic(endpoint, hash);
                panic!("real EVM receipt read: InvalidResponse");
            }
            Err(error) => panic!("real EVM receipt read: {error:?}"),
        };
        if !receipt.is_null() {
            assert_eq!(receipt["transactionHash"], hash);
            assert_eq!(receipt["status"], "0x1", "EVM producer reverted: {receipt}");
            return receipt;
        }
        assert!(
            Instant::now() < deadline,
            "EVM transaction did not commit: {hash}"
        );
        std::thread::sleep(Duration::from_millis(100));
    }
}

fn eth_call(node: &LocalNode, to: &[u8; 20], data: &[u8]) -> Vec<u8> {
    let mut response = ureq::post(&node.endpoint).header("Content-Type", "application/json")
        .send(serde_json::to_vec(&json!({"jsonrpc":"2.0","id":1,"method":"eth_call","params":[{"to":format!("0x{}", hex(to)),"data":format!("0x{}",hex(data))},"latest"]})).expect("setup read request").as_slice()).expect("real precompile read");
    let document: Value =
        serde_json::from_slice(&response.body_mut().read_to_vec().expect("precompile reply"))
            .expect("precompile JSON");
    assert!(
        document.get("error").is_none(),
        "precompile refused: {document}"
    );
    unhex(document["result"].as_str().expect("precompile hex"))
}

fn quantity_latest(node: &LocalNode, method: &str, address: &str) -> u128 {
    let request = serde_json::to_vec(&json!({
        "jsonrpc": "2.0", "id": "latest-quantity",
        "method": method, "params": [address, "latest"]
    }))
    .expect("latest quantity request");
    let mut response = ureq::post(&node.endpoint)
        .header("Content-Type", "application/json")
        .send(request.as_slice())
        .expect("real latest quantity transport");
    let document: Value = serde_json::from_slice(
        &response
            .body_mut()
            .read_to_vec()
            .expect("latest quantity reply"),
    )
    .expect("latest quantity JSON");
    assert_eq!(document["jsonrpc"], "2.0");
    assert_eq!(document["id"], "latest-quantity");
    assert!(
        document.get("error").is_none(),
        "{method} at latest refused: {document}"
    );
    let quantity = document["result"]
        .as_str()
        .and_then(|value| value.strip_prefix("0x"))
        .expect("latest hex quantity");
    u128::from_str_radix(quantity, 16).expect("latest quantity fits u128")
}

fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex text"), 16).expect("hex byte")
        })
        .collect()
}
fn registration(actor: &str, salt: [u8; 32]) -> Registration {
    let did = layerx_types::ids::Did::new(actor.as_bytes()).expect("real DID");
    let issuer = layerx_wire::hash::did_id_for_protocol(&did, 3).expect("real issuer id");
    Registration {
        asset: asset_id(&issuer, &salt),
        salt,
        symbol: "SDK".into(),
        name: "SDK path proof".into(),
        decimals: 6,
        supply_cap: 1_000,
        issuer_kind: 1,
        custody_ref: vec![],
    }
}

#[test]
fn proof_sdk_requires_the_real_evm_producer_in_the_declared_build() {
    use layerx_client::account_binding::{
        bound_did_call, decode_bound_did, plan_bind, sign_bind_transaction, BindFees, BindPlan,
        ADDR_PRECOMPILE,
    };
    let mut node = LocalNode::start();
    let env_path = node.directory.0.join("lx-localnode/localnode.env");
    let secret = Zeroizing::new(generated_setting(&env_path, "LOCAL_NODE_KEY"));
    let seed = Zeroizing::new(unhex(&secret));
    let account = derive_from_seed(&seed, 0).expect("derive real wallet from harness key material");
    node.fund(&account.evm_address_text());
    let rpc = node.rpc();
    let (chain_id, tip, maximum) = rpc.eip1559_parameters().expect("real fee producer");
    assert_eq!(chain_id, 125);
    let balance = quantity_latest(&node, "eth_getBalance", &account.evm_address_text());
    let required_balance = maximum.checked_mul(21_000).expect("transfer fee fits u128");
    assert!(
        balance >= required_balance,
        "funded account balance at latest {balance} is below 21000 * fee cap {required_balance}"
    );
    let transfer = Eip1559Call {
        chain_id,
        nonce: 0,
        max_priority_fee_per_gas: tip,
        max_fee_per_gas: maximum,
        gas_limit: 21_000,
        to: account.evm_address(),
        data: vec![],
    };
    assert_eq!(
        rpc.estimate_fee_from(&transfer, account.evm_address())
            .expect("real public fee estimator"),
        21_000
    );
    assert_eq!(
        rpc.get_identity_sequence(&account.evm_address_text())
            .expect("new account nonce"),
        0
    );
    let BindPlan::Bind(binding) =
        plan_bind(&account, chain_id, None, 0).expect("real binding producer")
    else {
        panic!("fresh account already bound");
    };
    let raw = sign_bind_transaction(
        &account,
        chain_id,
        &binding,
        BindFees {
            evm_nonce: 0,
            max_priority_fee_per_gas: tip,
            max_fee_per_gas: maximum,
            gas_limit: 500_000,
        },
    )
    .expect("real binding signatures");
    let hash = rpc.send_raw_transaction(&raw).expect("submit real binding");
    mined(&rpc, &node.endpoint, &format!("0x{}", hex(&hash)));
    assert_eq!(
        decode_bound_did(&eth_call(
            &node,
            &ADDR_PRECOMPILE,
            &bound_did_call(&account.evm_address())
        ))
        .expect("binding consumer"),
        Some(account.layerx_public_key())
    );
    let public: Value = serde_json::from_slice(
        &fs::read(node.home.join("receipt-public.json")).expect("generated receipt key"),
    )
    .expect("receipt key JSON");
    let key: [u8; 32] = unhex(public["public_key"].as_str().expect("generated public key"))
        .try_into()
        .expect("receipt key width");
    let policy = ReceiptPolicy {
        protocol_version: 3,
        network_id: 125,
        sequencer: SequencerAuthorization::new(Sha256::digest(key).into(), key, 1, u64::MAX),
        trusted_checkpoint_context_digest: None,
    };
    let signer = LocalSigner::from_derived_account(&account);
    let wallet = Wallet {
        rpc: &rpc,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    let options = PaymentOptions {
        actor: account.did(),
        idempotency_key: [0x51; 32],
        fee_limit: 10_000_000_000_000_000_000,
        not_before: 0,
        not_after: u64::MAX,
        commitment: Commitment::Executed,
        wait_timeout: Duration::from_secs(60),
    };
    let first = registration(&options.actor, [0x51; 32]);
    // The mutation succeeds on the EVM; the external version-2 receipt is refused.
    assert!(matches!(
        run(wallet.create(first.clone(), &options)),
        Err(RpcError::Verification)
    ));
    let deadline = Instant::now() + Duration::from_secs(60);
    loop {
        let committed = quantity_latest(
            &node,
            "eth_getTransactionCount",
            &account.evm_address_text(),
        );
        if committed >= 2 {
            assert_eq!(committed, 2, "registration committed nonce");
            break;
        }
        assert!(Instant::now() < deadline, "registration never committed");
        std::thread::sleep(Duration::from_millis(100));
    }
    let mut asset_read =
        layerx_crypto::disclosure::call::ledger_selector("getAsset(bytes32)").to_vec();
    asset_read.extend_from_slice(&first.asset);
    let ledger = call().to;
    let state = eth_call(&node, &ledger, &asset_read);
    assert_eq!(&state[..32], &first.asset);
    let payload = Payment::OpenAccount { asset: first.asset }
        .encode(options.actor.as_bytes())
        .expect("canonical account producer");
    let prepared = wallet
        .prepare_payload(ModuleId::Asset, 4, &payload, &options)
        .expect("real wallet preparation and gas estimate");
    let hash = run(wallet.execute(prepared, &options)).expect("real wallet execute");
    let hash_text = format!("0x{}", hex(&hash));
    let receipt = mined(&rpc, &node.endpoint, &hash_text);
    assert!(matches!(
        wallet.wait_for(hash, Commitment::Executed, Duration::from_secs(60)),
        Err(RpcError::Verification)
    ));
    let evidence = rpc.sequencer_receipt(hash).expect("real receipt transport");
    let canonical = unhex(evidence["receipt"].as_str().expect("node-produced receipt"));
    let decoded = layerx_wire::receipt::decode(&canonical).expect("real EVM receipt decode");
    let protocol = decoded.protocol().expect("full producer receipt");
    assert_eq!(protocol.protocol_version(), 2);
    assert_eq!(protocol.activity_id(), hash);
    assert_eq!(protocol.result_code(), 0);
    let authority = AuthorizedBatch::new(
        protocol.batch_id(),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
        key,
    );
    let verified = verify_receipt(&canonical, &authority)
        .expect("node-produced summary verifies economically under the pinned key");
    assert_eq!(verified.canonical_bytes(), canonical);
    let mut different_asset = protocol.asset();
    different_asset[0] ^= 1;
    let wrong_authority = AuthorizedBatch::new(
        protocol.batch_id(),
        different_asset,
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
        key,
    );
    assert_eq!(
        verify_receipt(&canonical, &wrong_authority)
            .expect_err("a different authorized asset must be refused")
            .check,
        ReceiptCheck::Asset
    );
    let nonce = rpc
        .get_identity_sequence(&account.evm_address_text())
        .expect("post-producer nonce");
    assert_eq!(nonce, 3);
    drop(wallet);
    drop(rpc);
    node.restart();
    let reopened = node.rpc();
    assert_eq!(
        reopened
            .get_identity_sequence(&account.evm_address_text())
            .expect("reopened durable nonce"),
        nonce
    );
    assert_eq!(
        reopened
            .get_receipt(&hash_text)
            .expect("reopened durable transaction"),
        receipt
    );
    assert_eq!(
        unhex(
            reopened
                .sequencer_receipt(hash)
                .expect("reopened durable sequencer WAL")["receipt"]
                .as_str()
                .expect("persisted receipt")
        ),
        canonical
    );
    assert_eq!(eth_call(&node, &ledger, &asset_read), state);
    let wallet = Wallet {
        rpc: &reopened,
        signer: &signer,
        policy: &policy,
        native_asset: [3; 32],
    };
    assert!(
        matches!(run(wallet.create(first, &options)), Err(RpcError::Reverted { code, .. }) if code == "LXP_ASSET_ALREADY_REGISTERED")
    );
    assert_eq!(
        reopened
            .get_identity_sequence(&account.evm_address_text())
            .expect("duplicate never submitted"),
        nonce
    );
    let seed_path = node.directory.0.join("bridge-seed");
    let mut seed_file = OpenOptions::new()
        .create_new(true)
        .write(true)
        .mode(0o600)
        .open(&seed_path)
        .expect("bridge key custody");
    seed_file.write_all(&seed).expect("private bridge seed");
    seed_file.sync_all().expect("durable bridge seed");
    drop(seed_file);
    let second = registration(&options.actor, [0x52; 32]);
    let payload = Payment::Register(second.clone())
        .encode(options.actor.as_bytes())
        .expect("real registration codec");
    let mut config = configuration(&node.endpoint);
    config["rpc_route"] = json!("node");
    config["sequencer_key"] = json!(hex(&key));
    config["sequencer_id"] = json!(hex(&Sha256::digest(key)));
    config["last_batch"] = json!(u64::MAX.to_string());
    config["evm_key_ref"] = json!(seed_path);
    config["account_index"] = json!("0");
    let (success, result) = bridge(
        &json!({"action":"submit","configuration":config,"actor":options.actor,"idempotency_key": "52".repeat(32),"fee_limit":options.fee_limit.to_string(),"not_before":"0","not_after":u64::MAX.to_string(),"timeout_ms":"1000","ordinal":"1","payload":hex(&payload)}),
    );
    assert!(success, "real bridge submission refused: {result}");
    assert_eq!(result["result"]["rung"], "submitted");
    let bridge_hash = result["result"]["txHash"]
        .as_str()
        .expect("bridge consumer hash");
    mined(&reopened, &node.endpoint, bridge_hash);
    let mut second_read =
        layerx_crypto::disclosure::call::ledger_selector("getAsset(bytes32)").to_vec();
    second_read.extend_from_slice(&second.asset);
    assert_eq!(&eth_call(&node, &ledger, &second_read)[..32], &second.asset);
    let (success, observed) = bridge(
        &json!({"action":"wait","configuration":config,"txHash":bridge_hash,"rung":"receipted","timeout_ms":"1000"}),
    );
    assert!(
        !success,
        "incompatible EVM receipt must not become verified bridge evidence"
    );
    assert_eq!(
        observed,
        json!({"ok":false,"error":{"kind":"wallet_request_refused_or_unavailable"}})
    );
}

#[allow(dead_code)]
#[path = "support/audit_22_30_99.rs"]
mod audit;

#[test]
fn proof_sdk_verified_create_balance_and_wait_survive_reopen() {
    audit::wallet_native_token_and_node_summary_round_trip_after_reopen();
}
