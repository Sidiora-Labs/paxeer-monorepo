use std::future::Future;
use std::io::{Read as _, Write as _};
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::time::Duration;

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::signer::LocalSigner;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_sdk::production::SecretBytes;
use layerx_sdk::programs::LayerXKeyCredential;
use layerx_sdk::rpc::{Commitment, RpcClient, RpcError};
use layerx_sdk::rpc_verification::ReceiptPolicy;
use layerx_sdk::wallet::{PaymentOptions, Wallet};
use layerx_types::payload::ModuleId;
use layerx_types::rung::Rung;
use serde_json::{json, Value};
use zeroize::Zeroizing;

struct ThreadWake(std::thread::Thread);
impl Wake for ThreadWake {
    fn wake(self: Arc<Self>) {
        self.0.unpark();
    }
    fn wake_by_ref(self: &Arc<Self>) {
        self.0.unpark();
    }
}
fn run<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => return value,
            Poll::Pending => std::thread::park(),
        }
    }
}
fn text<'a>(v: &'a Value, key: &str) -> Result<&'a str, ()> {
    v[key].as_str().ok_or(())
}
fn number(v: &Value, key: &str) -> Result<u64, ()> {
    let text = text(v, key)?;
    let n: u64 = text.parse().map_err(|_| ())?;
    if n.to_string() != text {
        return Err(());
    }
    Ok(n)
}
fn bytes(v: &Value, key: &str) -> Result<Vec<u8>, ()> {
    let s = text(v, key)?;
    if s.len() > 1_048_576
        || !s.len().is_multiple_of(2)
        || !s.bytes().all(|b| b.is_ascii_hexdigit())
    {
        return Err(());
    }
    s.as_bytes()
        .chunks_exact(2)
        .map(|b| {
            std::str::from_utf8(b)
                .ok()
                .and_then(|s| u8::from_str_radix(s, 16).ok())
                .ok_or(())
        })
        .collect()
}
fn fixed<const N: usize>(v: &Value, key: &str) -> Result<[u8; N], ()> {
    bytes(v, key)?.try_into().map_err(|_| ())
}
fn hex(bytes: &[u8]) -> String {
    const H: &[u8; 16] = b"0123456789abcdef";
    bytes
        .iter()
        .flat_map(|b| {
            [
                char::from(H[usize::from(b >> 4)]),
                char::from(H[usize::from(b & 15)]),
            ]
        })
        .collect()
}
fn transaction_hash(v: &Value) -> Result<[u8; 32], ()> {
    let hash = text(v, "txHash")?.strip_prefix("0x").ok_or(())?;
    fixed(&json!({"hash": hash}), "hash")
}
fn timeout(v: &Value) -> Result<Duration, ()> {
    let millis = number(v, "timeout_ms")?;
    if millis > 300_000 {
        return Err(());
    }
    Ok(Duration::from_millis(millis))
}
fn wait(v: &Value, rpc: RpcClient, policy: &ReceiptPolicy) -> Result<Value, BridgeError> {
    if v.get("commitment").is_some() {
        return Err(BridgeError::Invalid);
    }
    let requested = Rung::from_name(text(v, "rung")?).ok_or(())?;
    let hash = transaction_hash(v)?;
    if hash == [0; 32] {
        return Err(BridgeError::Invalid);
    }
    let duration = timeout(v)?;
    let hash_text = format!("0x{}", hex(&hash));
    if requested == Rung::Submitted {
        return Ok(json!({"txHash": hash_text, "rung": requested.name()}));
    }
    let deadline = std::time::Instant::now() + duration;
    let rpc = Arc::new(rpc);
    loop {
        if std::time::Instant::now() >= deadline {
            return Err(BridgeError::Rpc(RpcError::Pending { activity_id: hash }));
        }
        let (sender, receiver) = std::sync::mpsc::sync_channel(1);
        let client = Arc::clone(&rpc);
        std::thread::Builder::new()
            .name("wallet-receipt-read".into())
            .spawn(move || {
                let _ = sender.send(client.sequencer_receipt(hash));
            })
            .map_err(|_| BridgeError::Rpc(RpcError::Transport))?;
        let status = match receiver
            .recv_timeout(deadline.saturating_duration_since(std::time::Instant::now()))
        {
            Ok(result) => result.map_err(BridgeError::Rpc)?,
            Err(std::sync::mpsc::RecvTimeoutError::Timeout) => {
                return Err(BridgeError::Rpc(RpcError::Pending { activity_id: hash }));
            }
            Err(std::sync::mpsc::RecvTimeoutError::Disconnected) => {
                return Err(BridgeError::Rpc(RpcError::Transport));
            }
        };
        if std::time::Instant::now() >= deadline {
            return Err(BridgeError::Rpc(RpcError::Pending { activity_id: hash }));
        }
        if status["txHash"] != hash_text {
            return Err(BridgeError::Rpc(RpcError::InvalidResponse));
        }
        let achieved = Rung::from_name(text(&status, "rung")?).ok_or(())?;
        if achieved >= Rung::Receipted {
            let canonical = bytes(
                &json!({"receipt": text(&status, "receipt")?
                .strip_prefix("0x").ok_or(())?}),
                "receipt",
            )?;
            let keys = [policy.sequencer];
            let receipt = layerx_proof::receipt::verify_sequencer_signature(
                &canonical,
                layerx_proof::receipt::ReceiptKeys::HeightRanges(&keys),
            )
            .map_err(|_| BridgeError::Rpc(RpcError::Verification))?;
            let protocol = receipt.protocol().ok_or(())?;
            if protocol.activity_id() != hash
                || protocol.protocol_version() != policy.protocol_version
            {
                return Err(BridgeError::Rpc(RpcError::Verification));
            }
            if std::time::Instant::now() >= deadline {
                return Err(BridgeError::Rpc(RpcError::Pending { activity_id: hash }));
            }
            if requested == Rung::Receipted {
                return Ok(json!({"txHash": hash_text, "rung": Rung::Receipted.name(),
                    "receipt": hex(&canonical), "result_code": protocol.result_code(),
                    "state": if protocol.result_code() == 0 { "executed" } else { "refused" }}));
            }
            if achieved == Rung::Committed {
                return Err(BridgeError::Rpc(RpcError::MissingFinalityTrust));
            }
        }
        std::thread::sleep(
            Duration::from_millis(20)
                .min(deadline.saturating_duration_since(std::time::Instant::now())),
        );
    }
}
fn submit(v: &Value, rpc: &RpcClient, policy: &ReceiptPolicy) -> Result<Value, BridgeError> {
    use std::os::unix::fs::PermissionsExt as _;
    let c = &v["configuration"];
    let path = std::path::Path::new(text(c, "evm_key_ref")?);
    if !path.is_absolute() {
        return Err(BridgeError::Invalid);
    }
    let file = std::fs::File::open(path).map_err(|_| ())?;
    let metadata = file.metadata().map_err(|_| ())?;
    if !metadata.is_file() || metadata.permissions().mode() & 0o077 != 0 {
        return Err(BridgeError::Invalid);
    }
    let mut seed = Zeroizing::new(Vec::new());
    file.take(65).read_to_end(&mut seed).map_err(|_| ())?;
    let index = u32::try_from(number(c, "account_index")?).map_err(|_| ())?;
    let account = derive_from_seed(&seed, index).map_err(|_| ())?;
    let signer = LocalSigner::from_derived_account(&account);
    let wallet = Wallet {
        rpc,
        signer: &signer,
        policy,
        native_asset: fixed(c, "native_asset")?,
    };
    let options = PaymentOptions {
        actor: text(v, "actor")?.into(),
        idempotency_key: fixed(v, "idempotency_key")?,
        fee_limit: text(v, "fee_limit")?.parse().map_err(|_| ())?,
        not_before: number(v, "not_before")?,
        not_after: number(v, "not_after")?,
        commitment: Commitment::Executed,
        wait_timeout: timeout(v)?,
    };
    let ordinal = u16::try_from(number(v, "ordinal")?).map_err(|_| ())?;
    let payload = bytes(v, "payload")?;
    let prepared = wallet
        .prepare_payload(ModuleId::Asset, ordinal, &payload, &options)
        .map_err(BridgeError::Rpc)?;
    let hash = run(wallet.execute(prepared, &options)).map_err(BridgeError::Rpc)?;
    Ok(json!({"txHash": format!("0x{}", hex(&hash)), "rung": Rung::Submitted.name()}))
}
enum BridgeError {
    Invalid,
    Rpc(RpcError),
}
impl From<()> for BridgeError {
    fn from((): ()) -> Self {
        Self::Invalid
    }
}
fn execute(v: &Value) -> Result<Value, BridgeError> {
    let c = &v["configuration"];
    let credential = if c.get("key_id").is_some() {
        Some(
            LayerXKeyCredential::new(
                text(c, "key_id")?,
                SecretBytes::new(text(c, "key_secret")?.as_bytes()).map_err(|_| ())?,
            )
            .map_err(|_| ())?,
        )
    } else {
        None
    };
    let rpc = if c.get("ca_der").is_some() {
        RpcClient::connect_with_ca_der(text(c, "endpoint")?, credential, &bytes(c, "ca_der")?)
    } else {
        RpcClient::connect(text(c, "endpoint")?, credential)
    }
    .map_err(|_| ())?;
    let policy = ReceiptPolicy {
        protocol_version: u16::try_from(number(c, "protocol_version")?).map_err(|_| ())?,
        network_id: u32::try_from(number(c, "network_id")?).map_err(|_| ())?,
        sequencer: SequencerAuthorization::new(
            fixed(c, "sequencer_id")?,
            fixed(c, "sequencer_key")?,
            number(c, "first_batch")?,
            number(c, "last_batch")?,
        ),
        trusted_checkpoint_context_digest: if c.get("checkpoint_context_digest").is_some() {
            Some(fixed(c, "checkpoint_context_digest")?)
        } else {
            None
        },
    };
    match text(v, "action")? {
        "accounts" => {
            let rpc = rpc.with_clock(
                layerx_client::runtime_clock::RuntimeClock::from_environment().map_err(|_| ())?,
            );
            return rpc
                .get_balances(text(v, "did")?)
                .map(layerx_sdk::rpc::BalancesSnapshot::into_value)
                .map_err(BridgeError::Rpc);
        }
        "balance" => {
            let rpc = rpc.with_clock(
                layerx_client::runtime_clock::RuntimeClock::from_environment().map_err(|_| ())?,
            );
            return rpc
                .wallet(fixed(c, "native_asset")?)
                .balance(text(v, "did")?, fixed(v, "asset")?)
                .map_err(BridgeError::Rpc);
        }
        "wait" => wait(v, rpc, &policy),
        "submit" => submit(v, &rpc, &policy),
        _ => Err(BridgeError::Invalid),
    }
}
fn main() -> std::process::ExitCode {
    let mut input = Vec::new();
    let outcome = std::io::stdin()
        .take(2_097_153)
        .read_to_end(&mut input)
        .map_err(|_| BridgeError::Invalid)
        .and_then(|_| {
            if input.len() > 2_097_152 {
                return Err(BridgeError::Invalid);
            }
            let value: Value = serde_json::from_slice(&input).map_err(|_| ())?;
            execute(&value)
        });
    let success = outcome.is_ok();
    let output = match outcome {
        Ok(value) => json!({"ok":true,"result":value}),
        Err(BridgeError::Rpc(RpcError::Remote {
            code,
            message,
            data,
        })) => json!({"ok":false,"error":{"code":code,"message":message,"data":data}}),
        Err(BridgeError::Rpc(RpcError::Pending { activity_id })) => {
            json!({"ok":false,"error":{"kind":"pending","txHash":format!("0x{}", hex(&activity_id)),"rung":"submitted"}})
        }
        Err(_) => json!({"ok":false,"error":{"kind":"wallet_request_refused_or_unavailable"}}),
    };
    if let Ok(bytes) = serde_json::to_vec(&output) {
        if std::io::stdout().write_all(&bytes).is_err() {
            return std::process::ExitCode::FAILURE;
        }
    }
    if success {
        std::process::ExitCode::SUCCESS
    } else {
        std::process::ExitCode::FAILURE
    }
}
