//! Proves: SendDebit::sign / Wallet::prepare_payload -> layerx-wallet-rpc stdin/stdout subprocess -> RpcClient / BalancesSnapshot / Rung (sdk/platform/pipeline.kvx).
//!
//! Repairs `layerx-wallet-rpc.rs::submit`: legacy commitments and requests for a
//! stronger submission rung were silently ignored before preparing/signing a
//! transaction. `action` now refuses them before opening the key or using RPC.
//!
//! Native and token balance success and version-2 summary receipt recovery are
//! also qualified in-process by bridge_native_token_balances_and_verified_summary_survive_reopen.
//! Finding outside touches: `services/agent/sdk/typescript/src/wallet.ts::
//! WalletRpc.waitFor` still sends activity_id/commitment, whereas the binary
//! requires txHash/rung. All three retired commitments must currently refuse.
//!
//! Decision 2295: the proof owns the loopback JSON-RPC producer and its private
//! runtime-clock socket. SendDebit::sign and Wallet::prepare_payload produce the
//! native authorization and signed EIP-1559 transaction; the listener decodes
//! and validates them with the repository's crypto/disclosure code. Receipts
//! use layerx_wire::encode::Encoder, receipt::{decode,encode,encode_unsigned}
//! and hash::receipt_digest, with a real Ed25519 sequencer signature. The real
//! RpcClient HTTP transport, runtime-clock peer checks and binary receipt
//! verifier run unchanged. No endpoint, seed or clock path comes from the
//! environment, and no response substitutes for an unstarted producer.
//! The listener persists the actual signed transaction and canonical receipt,
//! stops and reopens them before the next subprocess consumes their state.
//! These are protocol crossing assertions, not an EVM execution engine proof.

use std::error::Error;
use std::fs::{self, DirBuilder, OpenOptions};
use std::future::Future;
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::{
    DirBuilderExt as _, MetadataExt as _, OpenOptionsExt as _, PermissionsExt as _,
};
use std::os::unix::net::UnixListener;
use std::path::{Path, PathBuf};
use std::process::{Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_crypto::disclosure::call::bind_call;
use layerx_types::clock::ClockReading;
use layerx_types::clock_protocol;
use layerx_wire::encode::Encoder;
use layerx_wire::receipt::{decode, encode, encode_unsigned};

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::evm_transaction::{assemble, decode_signed, recover_sender, transaction_hash};
use layerx_crypto::payments::Payment;
use layerx_crypto::send::{ledger_payment_call, send_context_hash, CallOptions, SendDebit};
use layerx_crypto::signer::{LocalSigner, Signer as _};
use layerx_sdk::production::SecretBytes;
use layerx_sdk::programs::LayerXKeyCredential;
use layerx_sdk::rpc::{wallet_account, BalancesSnapshot, RpcClient, RpcError};
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

type Outcome = Result<(), Box<dyn Error>>;
const NOW_SECONDS: u64 = 1_000;
const SEQUENCER_SEED: [u8; 32] = [0x61; 32];

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn unhex(value: &str) -> Result<Vec<u8>, Box<dyn Error>> {
    if !value.len().is_multiple_of(2) {
        return Err("odd hex length".into());
    }
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| Ok(u8::from_str_radix(std::str::from_utf8(pair)?, 16)?))
        .collect()
}

fn field<'a>(value: &'a Value, name: &str) -> Result<&'a str, Box<dyn Error>> {
    value[name]
        .as_str()
        .ok_or_else(|| format!("missing string field {name}").into())
}

fn rpc_error(error: RpcError) -> Box<dyn Error> {
    format!("real RPC operation refused: {error:?}").into()
}

struct PrivateState(PathBuf);
impl PrivateState {
    fn new() -> Result<Self, Box<dyn Error>> {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = Path::new("/tmp").join(format!(
            "proof-wallet-rpc-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        DirBuilder::new().mode(0o700).create(&path)?;
        Ok(Self(path))
    }

    fn persist(&self, name: &str, bytes: &[u8]) -> Result<PathBuf, Box<dyn Error>> {
        Self::persist_at(&self.0, name, bytes)
    }

    fn persist_at(directory: &Path, name: &str, bytes: &[u8]) -> Result<PathBuf, Box<dyn Error>> {
        let path = directory.join(name);
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&path)?;
        file.write_all(bytes)?;
        file.sync_all()?;
        Ok(path)
    }
}
impl Drop for PrivateState {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn request(input: &[u8], clock: Option<&ClockProducer>) -> Result<(bool, Value), Box<dyn Error>> {
    let binary = Path::new(env!("CARGO_BIN_EXE_layerx-wallet-rpc"));
    assert!(binary.is_absolute());
    let mut command = Command::new(binary);
    command
        .env_remove("LAYERX_RUNTIME_CLOCK_SOCKET")
        .env_remove("LAYERX_RUNTIME_CLOCK_PID")
        .env_remove("LAYERX_RUNTIME_CLOCK_UID");
    if let Some(clock) = clock {
        command
            .env("LAYERX_RUNTIME_CLOCK_SOCKET", &clock.binding.socket)
            .env(
                "LAYERX_RUNTIME_CLOCK_PID",
                clock.binding.process.to_string(),
            )
            .env("LAYERX_RUNTIME_CLOCK_UID", clock.binding.user.to_string());
    }
    let mut child = command
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()?;
    let sent = child
        .stdin
        .take()
        .ok_or("missing child stdin")?
        .write_all(input);
    let deadline = Instant::now() + Duration::from_secs(30);
    loop {
        if child.try_wait()?.is_some() {
            break;
        }
        if Instant::now() >= deadline {
            child.kill()?;
            child.wait()?;
            return Err("wallet subprocess exceeded 30 seconds".into());
        }
        std::thread::sleep(Duration::from_millis(2));
    }
    let output = child.wait_with_output()?;
    sent?;
    assert!(
        output.stderr.is_empty(),
        "wallet must not log request/key material"
    );
    let reply: Value = serde_json::from_slice(&output.stdout)?;
    assert_eq!(reply["ok"], output.status.success());
    assert_eq!(reply.as_object().ok_or("reply object")?.len(), 2);
    Ok((output.status.success(), reply))
}

fn call(value: &Value, clock: Option<&ClockProducer>) -> Result<(bool, Value), Box<dyn Error>> {
    request(&serde_json::to_vec(value)?, clock)
}

fn refused(outcome: (bool, Value)) {
    assert!(!outcome.0);
    assert_eq!(
        outcome.1,
        json!({"ok":false,
        "error":{"kind":"wallet_request_refused_or_unavailable"}})
    );
}

fn configuration(endpoint: &str) -> Value {
    json!({"endpoint":endpoint, "rpc_route":"node", "protocol_version":"3", "network_id":"125",
        "native_asset":"03".repeat(32), "sequencer_id":"01".repeat(32),
        "sequencer_key":"02".repeat(32), "first_batch":"1", "last_batch":"100"})
}

#[test]
fn malformed_and_retired_requests_are_refused_before_transport() -> Outcome {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    listener.set_nonblocking(true)?;
    let state = PrivateState::new()?;
    let key = state.persist("seed", &[0x42; 64])?;
    let mut config = configuration(&format!("http://{}", listener.local_addr()?));
    config["evm_key_ref"] = json!(key);
    config["account_index"] = json!("0");
    let payload = Payment::OpenAccount { asset: [3; 32] }
        .encode(b"did:layerx:alice")
        .map_err(|e| format!("{e:?}"))?;
    for input in [b"{".as_slice(), b"{}", b"[]", b"{} {}", b"null"] {
        refused(request(input, None)?);
    }
    refused(call(
        &json!({"action":"unknown", "configuration":config}),
        None,
    )?);
    for action in ["wait", "submit"] {
        for commitment in ["executed", "batched", "finalised"] {
            refused(call(
                &json!({"action":action, "configuration":config,
                "commitment":commitment, "activity_id":"01".repeat(32),
                "txHash":format!("0x{}", "01".repeat(32)), "rung":"submitted",
                "actor":"did:layerx:alice", "ordinal":"4", "payload":hex(&payload),
                "idempotency_key":"04".repeat(32), "fee_limit":"10000000",
                "not_before":"0", "not_after":"100", "timeout_ms":"0"}),
                None,
            )?);
        }
    }
    for rung in [
        json!("receipted"),
        json!("committed"),
        Value::Null,
        json!(1),
    ] {
        refused(call(
            &json!({"action":"submit", "configuration":config, "rung":rung,
            "actor":"did:layerx:alice", "ordinal":"4", "payload":hex(&payload),
            "idempotency_key":"04".repeat(32), "fee_limit":"10000000",
            "not_before":"0", "not_after":"100", "timeout_ms":"0"}),
            None,
        )?);
    }
    for action in ["accounts", "balance"] {
        refused(call(
            &json!({"action":action, "configuration":config,
            "did":"did:layerx:alice", "asset":"03".repeat(32)}),
            None,
        )?);
    }
    // This is a passive assertion that no connection happened, not an RPC peer.
    assert_eq!(
        listener
            .accept()
            .expect_err("refused requests must not contact RPC")
            .kind(),
        std::io::ErrorKind::WouldBlock
    );
    Ok(())
}

#[test]
fn manifest_and_existing_balance_consumer_preserve_the_contract() -> Outcome {
    let pipeline = include_str!("../../../../../sdk/platform/pipeline.kvx");
    let handwritten = pipeline
        .split("[handwritten.agent-rust]")
        .nth(1)
        .ok_or("handwritten SDK section")?
        .split("\n[")
        .next()
        .ok_or("section body")?;
    assert!(handwritten
        .lines()
        .any(|line| line == "\"bin/layerx-wallet-rpc.rs\" = \"handwritten\""));
    let manifest = include_str!("../Cargo.toml");
    let proof = manifest
        .split("[package.metadata.proof.agent-wallet-rpc]")
        .nth(1)
        .ok_or("wallet proof declaration")?
        .split("\n[")
        .next()
        .ok_or("proof body")?;
    assert!(proof
        .contains("test = \"services/agent/crates/layerx-sdk/tests/proof_layerx_wallet_rpc.rs\""));
    assert!(proof.contains("producer = \"test-owned loopback JSON-RPC listener"));
    assert!(proof.contains("clock = \"test-owned private runtime-clock protocol listener\""));
    assert!(!proof.contains("configuration_env"));
    let rpc = RpcClient::connect("http://127.0.0.1:1", None).map_err(rpc_error)?;
    audit::root_wallet_request_and_method_response();
    assert!(matches!(
        rpc.wallet([3; 32]).balance("did:layerx:alice", [3; 32]),
        Err(RpcError::InvalidRequest)
    ));
    Ok(())
}

fn ready<T>(future: impl Future<Output = T>) -> T {
    struct LocalWake;
    impl Wake for LocalWake {
        fn wake(self: Arc<Self>) {}
    }
    let waker = Waker::from(Arc::new(LocalWake));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("LocalSigner must finish synchronously"),
    }
}

#[test]
fn submitted_acknowledgement_survives_signer_and_process_restart() -> Outcome {
    let state = PrivateState::new()?;
    let key = state.persist("seed", &[0x42; 64])?;
    let produce = || -> Result<Vec<u8>, Box<dyn Error>> {
        let seed = Zeroizing::new(fs::read(&key)?);
        let account = derive_from_seed(&seed, 0)?;
        let signer = LocalSigner::from_derived_account(&account);
        let from = wallet_account(&account.did(), [3; 32], [3; 32]).map_err(rpc_error)?;
        let to = wallet_account(&derive_from_seed(&seed, 1)?.did(), [3; 32], [3; 32])
            .map_err(rpc_error)?;
        let debit = SendDebit {
            from,
            to,
            asset: [3; 32],
            amount: 1,
            source_sequence: 0,
            idempotency_key: [4; 32],
            expires_at: 100,
            context_hash: send_context_hash(&from, &to, &[3; 32], 1, &[4; 32]),
            conditions: Vec::new(),
            authorization_kind: 1,
            network_id: 125,
            protocol_version: layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
        };
        let payload = ready(debit.sign(&signer)).map_err(|e| format!("{e:?}"))?;
        let disclosed = ledger_payment_call(
            &payload,
            &CallOptions {
                from: account.evm_address(),
                chain_id: 125,
                nonce: 0,
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 5,
                gas_limit: 100_000,
            },
        )
        .map_err(|e| format!("{e:?}"))?;
        let signature =
            ready(signer.sign_transaction(&disclosed.call)).map_err(|e| format!("{e:?}"))?;
        let raw = assemble(&disclosed.call, &signature);
        let (decoded, signature) = decode_signed(&raw)?;
        assert_eq!(recover_sender(&decoded, &signature)?, account.evm_address());
        Ok(raw)
    };
    // All signer objects are dropped on return; persist only their real output.
    let raw_path = state.persist("signed-transaction", &produce()?)?;
    let raw = fs::read(raw_path)?;
    assert_eq!(
        produce()?,
        raw,
        "reopened seed must produce the identical transaction"
    );
    let hash = format!("0x{}", hex(&transaction_hash(&raw)));
    let listener = TcpListener::bind("127.0.0.1:0")?;
    listener.set_nonblocking(true)?;
    let config = configuration(&format!("http://{}", listener.local_addr()?));
    let input = serde_json::to_vec(&json!({"action":"wait", "configuration":config,
        "txHash":hash, "rung":"submitted", "timeout_ms":"0"}))?;
    let path = state.persist("wait.json", &input)?;
    let first = request(&fs::read(&path)?, None)?;
    assert!(first.0);
    assert_eq!(
        first.1,
        json!({"ok":true,"result":{"txHash":hash,"rung":"submitted"}})
    );
    assert_eq!(
        Rung::from_name(field(&first.1["result"], "rung")?),
        Some(Rung::Submitted)
    );
    // A new process reads the reopened request after the first process exited.
    assert_eq!(request(&fs::read(path)?, None)?, first);
    assert_eq!(
        listener
            .accept()
            .expect_err("submitted is a local acknowledgement, not a receipt query")
            .kind(),
        std::io::ErrorKind::WouldBlock
    );
    Ok(())
}

fn real_client(config: &Value, clock: &ClockProducer) -> Result<RpcClient, Box<dyn Error>> {
    let credential = if config.get("key_id").is_some() {
        Some(
            LayerXKeyCredential::new(
                field(config, "key_id")?,
                SecretBytes::new(field(config, "key_secret")?.as_bytes())
                    .map_err(|e| format!("{e:?}"))?,
            )
            .map_err(|e| format!("{e:?}"))?,
        )
    } else {
        None
    };
    let client = if config.get("ca_der").is_some() {
        RpcClient::connect_node_with_ca_der(
            field(config, "endpoint")?,
            credential,
            &unhex(field(config, "ca_der")?)?,
        )
    } else {
        RpcClient::connect_node(field(config, "endpoint")?, credential)
    };
    Ok(client
        .map_err(rpc_error)?
        .with_clock(RuntimeClock::connect(clock.binding.clone())?))
}

fn sequence(rpc: &RpcClient, account: &[u8; 32]) -> Result<u64, Box<dyn Error>> {
    let value = rpc.get_account(&hex(account)).map_err(rpc_error)?;
    assert_eq!(value["account_id"], hex(account));
    Ok(field(&value, "next_sequence")?.parse()?)
}

#[test]
fn action() -> Outcome {
    let state = PrivateState::new()?;
    let clock = ClockProducer::start(&state)?;
    let seed = Zeroizing::new(vec![0x42; 64]);
    let key = state.persist("seed", &seed)?;
    let account = derive_from_seed(&seed, 0)?;
    let recipient = derive_from_seed(&seed, 1)?;
    let did = account.did();
    let asset = [3; 32];
    let from = wallet_account(&did, asset, asset).map_err(rpc_error)?;
    let to = wallet_account(&recipient.did(), asset, asset).map_err(rpc_error)?;
    let before: u64 = 0;
    let mut digest = Sha256::new();
    digest.update(b"proof-layerx-wallet-rpc");
    digest.update(from);
    digest.update(before.to_be_bytes());
    let idempotency_key: [u8; 32] = digest.finalize().into();
    let debit = SendDebit {
        from,
        to,
        asset,
        amount: 1,
        source_sequence: before,
        idempotency_key,
        expires_at: NOW_SECONDS + 100,
        context_hash: send_context_hash(&from, &to, &asset, 1, &idempotency_key),
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: 125,
        protocol_version: layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    };
    let signer = LocalSigner::from_derived_account(&account);
    let payload = ready(debit.sign(&signer)).map_err(|e| format!("{e:?}"))?;
    let node = NodeProducer::start(
        TcpListener::bind("127.0.0.1:0")?,
        state.0.clone(),
        debit.clone(),
        payload.clone(),
        account.evm_address(),
    )?;
    let mut config = configuration(&node.endpoint);
    config["evm_key_ref"] = json!(key);
    config["account_index"] = json!("0");
    config["sequencer_key"] = json!(hex(&SigningKey::from_bytes(&SEQUENCER_SEED)
        .verifying_key()
        .to_bytes()));
    let submit = json!({"action":"submit", "configuration":config, "actor":did,
        "ordinal":"5", "payload":hex(&payload), "idempotency_key":hex(&idempotency_key),
        "fee_limit":"10000000", "not_before":"1000", "not_after":"1100", "timeout_ms":"5000"});
    let producer = state.persist("submit.json", &serde_json::to_vec(&submit)?)?;
    drop(signer);
    drop(account);
    drop(recipient);
    drop(seed);

    // Reopen the real producer output and seed; the CLI itself loads the signer.
    let input = fs::read(&producer)?;
    for commitment in ["executed", "batched", "finalised"] {
        let mut invalid = submit.clone();
        invalid["commitment"] = json!(commitment);
        refused(call(&invalid, Some(&clock))?);
    }
    assert_eq!(sequence(&real_client(&config, &clock)?, &from)?, before);
    let (success, submitted) = request(&input, Some(&clock))?;
    assert!(success, "real signed submission must succeed: {submitted}");
    assert_eq!(submitted["result"]["rung"], Rung::Submitted.name());
    let hash = field(&submitted["result"], "txHash")?;
    assert_eq!(
        unhex(hash.strip_prefix("0x").ok_or("transaction hash prefix")?)?.len(),
        32
    );
    let submitted_path = state.persist("submitted.json", &serde_json::to_vec(&submitted)?)?;

    let (listener, observed) = node.stop()?;
    assert_eq!(
        observed
            .iter()
            .filter(|request| request["method"] == "eth_sendRawTransaction")
            .count(),
        1
    );
    let raw = fs::read(state.0.join("node-transaction"))?;
    assert_eq!(hash, format!("0x{}", hex(&transaction_hash(&raw))));
    let sender = derive_from_seed(&fs::read(&key)?, 0)?.evm_address();
    let node = NodeProducer::start(listener, state.0.clone(), debit, payload, sender)?;
    assert_eq!(node.endpoint, field(&config, "endpoint")?);
    let rpc = real_client(&config, &clock)?;
    let (success, accounts) = call(
        &json!({"action":"accounts", "configuration":config, "did":did}),
        Some(&clock),
    )?;
    assert!(success, "real account enumeration must succeed");
    let snapshot = BalancesSnapshot::try_from(accounts["result"].clone()).map_err(rpc_error)?;
    assert!(snapshot
        .unverified_fields()
        .get("balances")
        .is_some_and(Value::is_array));
    assert_eq!(accounts["result"]["balances"][0]["account_id"], hex(&from));
    assert_eq!(accounts["result"]["balances"][0]["balance"], "99");
    assert_eq!(snapshot.into_value(), accounts["result"]);
    assert_eq!(
        rpc.wallet(asset)
            .accounts(&did)
            .map_err(rpc_error)?
            .into_value(),
        accounts["result"]
    );
    assert_eq!(
        rpc.wallet(asset).balance(&did, asset).map_err(rpc_error)?,
        json!("0x63")
    );
    let (success, balance) = call(
        &json!({"action":"balance", "configuration":config, "did":did,
        "asset":hex(&asset)}),
        Some(&clock),
    )?;
    assert!(success, "reopened balance must succeed: {balance}");
    assert_eq!(
        balance["result"],
        json!({"account_id":hex(&from),"asset_id":hex(&asset),"balance":"99"})
    );

    let recovered_submission: Value = serde_json::from_slice(&fs::read(submitted_path)?)?;
    let wait = json!({"action":"wait", "configuration":config,
        "txHash":recovered_submission["result"]["txHash"], "rung":"receipted", "timeout_ms":"5000"});
    let (success, receipt) = call(&wait, Some(&clock))?;
    assert!(
        success,
        "real sequencer receipt must be verified: {receipt}"
    );
    assert_eq!(receipt["result"]["txHash"], hash);
    assert_eq!(receipt["result"]["rung"], Rung::Receipted.name());
    assert_eq!(receipt["result"]["result_code"], 0);
    assert_eq!(receipt["result"]["state"], "executed");
    let canonical = unhex(field(&receipt["result"], "receipt")?)?;
    assert!(!canonical.is_empty());
    assert_eq!(canonical, fs::read(state.0.join("node-receipt"))?);
    let decoded = checked(decode(&canonical));
    let protocol = decoded.protocol().ok_or("missing protocol receipt")?;
    assert_eq!(protocol.activity_id(), transaction_hash(&raw));
    assert_eq!(protocol.from(), from);
    assert_eq!(protocol.to(), to);
    assert_eq!(protocol.amount(), 1);
    assert_eq!(protocol.debit_balance_before(), 100);
    assert_eq!(protocol.debit_balance_after(), 99);
    assert_eq!(protocol.debit_sequence(), before);
    assert_eq!(
        sequence(&rpc, &from)?,
        before.checked_add(1).ok_or("sequence overflow")?
    );
    drop(rpc);

    // The first CLI and client are gone. Recover precisely the same evidence.
    let (success, recovered) = call(&wait, Some(&clock))?;
    assert!(success);
    assert_eq!(recovered, receipt);
    refused(request(&fs::read(producer)?, Some(&clock))?);
    let rpc = real_client(&config, &clock)?;
    assert_eq!(
        sequence(&rpc, &from)?,
        before + 1,
        "duplicate must not debit twice"
    );
    for commitment in ["executed", "batched", "finalised"] {
        let mut retired = wait.clone();
        retired["commitment"] = json!(commitment);
        refused(call(&retired, Some(&clock))?);
    }
    let mut unknown = wait.clone();
    unknown["txHash"] = json!(format!("0x{}", "ff".repeat(32)));
    let (success, refusal) = call(&unknown, Some(&clock))?;
    assert!(!success);
    assert_eq!(refusal["error"]["code"], -32001);
    assert_eq!(refusal["error"]["message"], "Unknown transaction");
    let mut wrong_key = wait.clone();
    wrong_key["configuration"]["sequencer_key"] = json!(hex(&SigningKey::from_bytes(&[0x62; 32])
        .verifying_key()
        .to_bytes()));
    refused(call(&wrong_key, Some(&clock))?);
    assert_eq!(
        rpc.wallet(asset)
            .accounts(&did)
            .map_err(rpc_error)?
            .into_value(),
        accounts["result"],
        "duplicate must preserve the recovered balance"
    );
    let (_, observed) = node.stop()?;
    assert!(
        !observed
            .iter()
            .any(|request| request["method"] == "eth_sendRawTransaction"),
        "reopened state must refuse duplicate before signing/submission"
    );
    clock.stop()?;
    Ok(())
}

/// Owns a producer thread and makes teardown observable, including thread panics.
struct Producer<T> {
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<T>>,
}
impl<T: Send + 'static> Producer<T> {
    fn spawn(work: impl FnOnce(Arc<AtomicBool>) -> T + Send + 'static) -> Self {
        let stop = Arc::new(AtomicBool::new(false));
        let flag = Arc::clone(&stop);
        Self {
            stop,
            worker: Some(thread::spawn(move || work(flag))),
        }
    }
    fn finish(mut self) -> Result<T, Box<dyn Error>> {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .ok_or("producer already stopped")?
            .join()
            .map_err(|_| "proof_layerx_wallet_rpc.rs: owned producer panicked".into())
    }
}
impl<T> Drop for Producer<T> {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !thread::panicking() {
                assert!(
                    result.is_ok(),
                    "proof_layerx_wallet_rpc.rs: owned producer panicked"
                );
            }
        }
    }
}

struct ClockProducer {
    binding: ClockBinding,
    worker: Producer<()>,
}
impl ClockProducer {
    fn start(state: &PrivateState) -> Result<Self, Box<dyn Error>> {
        let socket = state.0.join("clock.sock");
        let listener = UnixListener::bind(&socket)?;
        fs::set_permissions(&socket, fs::Permissions::from_mode(0o600))?;
        listener.set_nonblocking(true)?;
        let binding = ClockBinding {
            socket,
            process: std::process::id(),
            user: fs::metadata(&state.0)?.uid(),
        };
        let worker = Producer::spawn(move |stop| {
            let start = Instant::now();
            while !stop.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((mut stream, _)) => {
                        stream
                            .set_read_timeout(Some(Duration::from_secs(3)))
                            .expect("clock read bound");
                        stream
                            .set_write_timeout(Some(Duration::from_secs(3)))
                            .expect("clock write bound");
                        let mut bytes = [0; clock_protocol::REQUEST_BYTES];
                        stream
                            .read_exact(&mut bytes)
                            .expect("runtime clock request");
                        let request = clock_protocol::Request::decode(&bytes)
                            .expect("canonical clock request");
                        thread::sleep(Duration::from_nanos(request.wait_nanoseconds));
                        let reading = ClockReading {
                            generation: [0x71; 16],
                            unix_milliseconds: NOW_SECONDS * 1_000
                                + u64::try_from(start.elapsed().as_millis())
                                    .expect("clock milliseconds"),
                            monotonic_nanoseconds: u64::try_from(start.elapsed().as_nanos())
                                .expect("clock nanoseconds"),
                        };
                        stream
                            .write_all(&clock_protocol::response(request.counter, Ok(reading)))
                            .expect("canonical runtime clock response");
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1))
                    }
                    Err(error) => panic!("proof_layerx_wallet_rpc.rs: clock listener: {error}"),
                }
            }
        });
        // This authenticates the actual listener's PID, UID and socket permissions.
        RuntimeClock::connect(binding.clone())?;
        Ok(Self { binding, worker })
    }
    fn stop(self) -> Outcome {
        self.worker.finish()
    }
}

fn checked<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("proof_layerx_wallet_rpc.rs: producer refused: {error:?}"))
}

/// Emits canonical core receipt fields from this producer's accepted transaction.
/// The public wire primitive encoder constructs the value, and the repository's
/// typed receipt encoder alone emits the final unsigned/signed wire messages.
fn produce_receipt(raw: &[u8], debit: &SendDebit) -> Vec<u8> {
    let mut e = Encoder::new(layerx_wire::limits::MAX_MESSAGE_BYTES);
    checked(e.structure_header_version(0x5201, debit.protocol_version));
    checked(e.u16(debit.protocol_version));
    checked(e.bytes(&transaction_hash(raw), 32));
    checked(e.u64(1));
    checked(e.bytes(&Sha256::digest(b"wallet-proof-before"), 32));
    checked(e.bytes(&Sha256::digest(raw), 32));
    checked(e.bytes(&Sha256::digest([]), 32));
    checked(e.i32(0));
    checked(e.sequence_length(0, 512));
    checked(e.u128(0));
    let mut height = [0; 32];
    height[24..].copy_from_slice(&1_u64.to_be_bytes());
    checked(e.bytes(&height, 32));
    checked(e.u16(1));
    checked(e.u32(1));
    checked(e.u32(1));
    checked(e.u8(5));
    checked(e.bytes(&debit.asset, 32));
    checked(e.u128(debit.amount));
    checked(e.bytes(&debit.from, 32));
    checked(e.u128(100));
    checked(e.u128(100_u128.checked_sub(debit.amount).expect("funded debit")));
    checked(e.u64(debit.source_sequence));
    checked(e.bytes(&debit.to, 32));
    checked(e.u128(0));
    checked(e.u128(debit.amount));
    checked(e.bytes(&[0; 32], 32));
    checked(e.bytes(&Sha256::digest(checked(debit.authorization_message())), 32));
    checked(e.bytes(&[1; 32], 32));
    checked(e.u64(NOW_SECONDS));
    checked(e.u8(0));
    let receipt = checked(decode(&e.finish()));
    let unsigned = checked(encode_unsigned(&receipt));
    let digest = checked(layerx_wire::hash::receipt_digest(&unsigned));
    let signature = SigningKey::from_bytes(&SEQUENCER_SEED).sign(&digest);
    // The optional signature is the last field in the canonical receipt layout.
    let mut signed = Encoder::new(layerx_wire::limits::MAX_MESSAGE_BYTES);
    checked(signed.fixed(&unsigned[..unsigned.len() - 1]));
    checked(signed.u8(1));
    checked(signed.bytes(&signature.to_bytes(), 64));
    let receipt = checked(decode(&signed.finish()));
    checked(encode(&receipt))
}

type NodeRun = (TcpListener, Vec<Value>);
struct NodeProducer {
    endpoint: String,
    worker: Producer<NodeRun>,
}
impl NodeProducer {
    fn start(
        listener: TcpListener,
        directory: PathBuf,
        debit: SendDebit,
        payload: Vec<u8>,
        sender: [u8; 20],
    ) -> Result<Self, Box<dyn Error>> {
        assert!(
            listener.local_addr()?.ip().is_loopback(),
            "proof_layerx_wallet_rpc.rs: producer must be test-owned loopback"
        );
        let endpoint = format!("http://{}", listener.local_addr()?);
        listener.set_nonblocking(true)?;
        let raw_path = directory.join("node-transaction");
        let receipt_path = directory.join("node-receipt");
        let stored = match (raw_path.exists(), receipt_path.exists()) {
            (false, false) => None,
            (true, true) => {
                let raw = fs::read(&raw_path)?;
                let canonical = fs::read(&receipt_path)?;
                let receipt = checked(decode(&canonical));
                assert_eq!(checked(encode(&receipt)), canonical);
                assert_eq!(
                    receipt.protocol().expect("protocol receipt").activity_id(),
                    transaction_hash(&raw)
                );
                assert_eq!(
                    produce_receipt(&raw, &debit),
                    canonical,
                    "reopened producer state must preserve signed evidence"
                );
                Some((raw, canonical))
            }
            _ => panic!("proof_layerx_wallet_rpc.rs: incomplete persisted node producer"),
        };
        let worker = Producer::spawn(move |stop| {
            let mut stored = stored;
            let mut observed = Vec::new();
            while !stop.load(Ordering::Acquire) {
                let mut stream = match listener.accept() {
                    Ok((stream, _)) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("proof_layerx_wallet_rpc.rs: RPC listener: {error}"),
                };
                checked(stream.set_read_timeout(Some(Duration::from_secs(3))));
                checked(stream.set_write_timeout(Some(Duration::from_secs(3))));
                let mut header = Vec::new();
                while !header.ends_with(b"\r\n\r\n") {
                    assert!(header.len() < 8192, "RPC header bound");
                    let mut byte = [0];
                    checked(stream.read_exact(&mut byte));
                    header.push(byte[0]);
                }
                let header = checked(std::str::from_utf8(&header));
                assert!(header.starts_with("POST / HTTP/1.1\r\n"));
                let length = header
                    .lines()
                    .find_map(|line| {
                        let (key, value) = line.split_once(':')?;
                        key.eq_ignore_ascii_case("content-length")
                            .then(|| checked(value.trim().parse::<usize>()))
                    })
                    .expect("RPC content length");
                assert!(length <= 1_048_576 + 4096, "RPC body bound");
                let mut bytes = vec![0; length];
                checked(stream.read_exact(&mut bytes));
                let request: Value = checked(serde_json::from_slice(&bytes));
                assert_eq!(request["jsonrpc"], "2.0");
                let method = request["method"].as_str().expect("RPC method");
                let params = &request["params"];
                let sequence = debit.source_sequence + u64::from(stored.is_some());
                let result = match method {
                    "eth_getTransactionCount" => {
                        assert_eq!(params, &json!([format!("0x{}", hex(&sender)), "pending"]));
                        json!(format!("0x{sequence:x}"))
                    }
                    "eth_blockNumber" => {
                        assert_eq!(params, &json!([]));
                        json!("0x1")
                    }
                    "eth_getBalance" => {
                        assert_eq!(params, &json!([format!("0x{}", hex(&sender)), "pending"]));
                        json!(format!("0x{:x}", if stored.is_some() { 99 } else { 100 }))
                    }
                    "eth_call" => {
                        let data = checked(unhex(
                            params[0]["data"]
                                .as_str()
                                .expect("calldata")
                                .strip_prefix("0x")
                                .expect("prefix"),
                        ));
                        let mut result = [0; 32];
                        if data[..4] == layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX {
                            assert_eq!(
                                &data[4..],
                                &derive_from_seed(&[0x42; 64], 0)
                                    .expect("owner")
                                    .layerx_public_key()
                            );
                            assert_eq!(params[1], "pending");
                            result[12..].copy_from_slice(&sender);
                        } else {
                            assert_eq!(
                                &data[..4],
                                &layerx_crypto::disclosure::call::ledger_selector(
                                    "balanceOf(bytes32,bytes32)"
                                )
                            );
                            assert_eq!(&data[4..36], &debit.from);
                            assert_eq!(&data[36..68], &debit.asset);
                            assert_eq!(params[1], "0x1");
                            result[16..].copy_from_slice(
                                &(if stored.is_some() { 99_u128 } else { 100 }).to_be_bytes(),
                            );
                        }
                        json!(format!("0x{}", hex(&result)))
                    }
                    "eth_chainId" => {
                        assert_eq!(params, &json!([]));
                        json!("0x7d")
                    }
                    "eth_gasPrice" => {
                        assert_eq!(params, &json!([]));
                        json!("0x2")
                    }
                    "eth_maxPriorityFeePerGas" => {
                        assert_eq!(params, &json!([]));
                        json!("0x1")
                    }
                    "eth_estimateGas" => {
                        assert_eq!(params[1], "pending");
                        assert_eq!(params[0]["from"], format!("0x{}", hex(&sender)));
                        json!("0x186a0")
                    }
                    "px_getAccount" => {
                        assert_eq!(params, &json!([hex(&debit.from)]));
                        json!({"account_id":hex(&debit.from), "next_sequence":sequence.to_string()})
                    }
                    "px_getBalances" => {
                        let did = derive_from_seed(&[0x42; 64], 0)
                            .expect("producer account")
                            .did();
                        assert_eq!(params, &json!([did]));
                        let balance = match &stored {
                            Some((_, canonical)) => checked(decode(canonical))
                                .protocol()
                                .expect("protocol receipt")
                                .debit_balance_after(),
                            None => 100,
                        };
                        json!({"balances":[{"account_id":hex(&debit.from), "asset_id":hex(&debit.asset), "balance":balance.to_string()}]})
                    }
                    "eth_sendRawTransaction" => {
                        assert!(
                            stored.is_none(),
                            "duplicate must be refused by source sequence before transport"
                        );
                        let raw = checked(unhex(
                            params[0]
                                .as_str()
                                .expect("raw transaction")
                                .strip_prefix("0x")
                                .expect("raw prefix"),
                        ));
                        let (call, signature) = checked(decode_signed(&raw));
                        assert_eq!(checked(recover_sender(&call, &signature)), sender);
                        assert_eq!(call.nonce, sequence);
                        assert_eq!(call.chain_id, u64::from(debit.network_id));
                        let expected = checked(ledger_payment_call(
                            &payload,
                            &CallOptions {
                                from: sender,
                                chain_id: 125,
                                nonce: sequence,
                                max_priority_fee_per_gas: 1,
                                max_fee_per_gas: 5,
                                gas_limit: 100_000,
                            },
                        ));
                        assert_eq!(
                            call, expected.call,
                            "signed call must preserve the actual native producer bytes"
                        );
                        let disclosure = checked(bind_call(&call, sender));
                        assert_eq!(
                            checked(disclosure.payload_sequence()),
                            Some(debit.source_sequence)
                        );
                        assert_eq!(disclosure.counterparties[0].account, debit.from);
                        assert_eq!(disclosure.counterparties[1].account, debit.to);
                        let canonical = produce_receipt(&raw, &debit);
                        checked(PrivateState::persist_at(
                            &directory,
                            "node-transaction",
                            &raw,
                        ));
                        checked(PrivateState::persist_at(
                            &directory,
                            "node-receipt",
                            &canonical,
                        ));
                        let hash = transaction_hash(&raw);
                        stored = Some((raw, canonical));
                        json!(format!("0x{}", hex(&hash)))
                    }
                    "pax_sequencerReceipt" => {
                        if let Some((raw, canonical)) = &stored {
                            let hash = format!("0x{}", hex(&transaction_hash(raw)));
                            if params == &json!([hash]) {
                                json!({"txHash":hash, "rung":Rung::Receipted.name(), "receipt":format!("0x{}",hex(canonical))})
                            } else {
                                Value::Null
                            }
                        } else {
                            Value::Null
                        }
                    }
                    other => {
                        panic!("proof_layerx_wallet_rpc.rs: unimplemented producer method {other}")
                    }
                };
                let response = if result.is_null() {
                    json!({"jsonrpc":"2.0", "id":request["id"], "error":{"code":-32001, "message":"Unknown transaction"}})
                } else {
                    json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
                };
                let response = checked(serde_json::to_vec(&response));
                checked(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.len()));
                checked(stream.write_all(&response));
                observed.push(request);
            }
            (listener, observed)
        });
        Ok(Self { endpoint, worker })
    }
    fn stop(self) -> Result<NodeRun, Box<dyn Error>> {
        self.worker.finish()
    }
}

#[allow(dead_code)]
#[path = "support/audit_22_30_99.rs"]
mod audit;

#[test]
fn bridge_native_token_balances_and_verified_summary_survive_reopen() {
    audit::bridge_balances_and_summary_receipt_survive_reopen();
}
