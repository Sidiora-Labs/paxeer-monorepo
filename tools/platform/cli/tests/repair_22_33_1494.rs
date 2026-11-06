//! Task 22.33.1494 repair record (scope prohibits editing the spec task record).
//! wallet_finality_option -> wallet_finality_option_committed_receipt_for_every_wait.
//! retired_endpoint_wallet -> retired_endpoint_wallet_separates_root_and_hosted_routes.
//! wallet_submission_recovery -> wallet_submission_recovery_reopens_journal_without_resubmission.
//! wallet_snapshot_proof_label -> wallet_snapshot_proof_label_binds_canonical_fields_and_pinned_root.
//! wallet_key_file_open -> wallet_key_file_open_qualifies_protected_derived_exports.
//! workspace_retired_core -> workspace_retired_core_uses_current_domain_targets.
//! Previous attempt: four cases passed; finality and recovery failed before submission on the estimate validator.
//! Continuation: the unchanged baseline is not rerun under the implementation-first and hf-done-only order.
//! Ruling: use a standard-library socket file for bounded nonregular-input refusal instead of the unavailable FIFO constructor.
//! Wiring: services/agent/crates/layerx-sdk/src/rpc.rs (agent SDK domain 9): expose submitted/receipted/committed names and the pinned account-proof consumer; the CLI uses the shared proof codec directly meanwhile.
//! Wiring: services/platform/hosted/gateway/src (platform domain 8): publish canonical AccountStateRecord and HeaderRecord with account snapshots; current observed balances remain explicitly unverified.
//! Wiring: tools/platform/cli/src/main.rs and rpc_subscription.rs (platform domain 8): update global endpoint help and websocket routing; these files are outside this task.
//! Wiring: tools/platform/cli/tests/workspace.rs (platform domain 8): assert paxd/kernel targets in its historical workspace suite; the scoped test below covers the current plan.

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::account_derivation::{derive_from_mnemonic, DerivedAccount};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_platform_cli::wallet_signing::EvmSigningKey;
use layerx_proof::export_codec::{
    AccountStateRecord, AccountStateVariant, FactSelector, HeaderRecord,
};
use layerx_proof::merkle::{build_proof, Proof};
use layerx_proof::state::{decode_account_value, NestedAccountProof};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{account_id_for_protocol, batch_header_digest, receipt_digest};
use ring::{aead, pbkdf2};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::fs::{self, OpenOptions};
use std::io::{BufRead as _, Read as _, Write as _};
use std::net::TcpListener;
use std::num::NonZeroU32;
use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _, PermissionsExt as _};
use std::path::Path;
use std::process::{Command, Output};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread;
use std::time::{Duration, Instant};

const ACCOUNT_PROTOCOL_VERSION: u16 = 3;
const NETWORK_ID: u32 = 125;
const BATCH_NUMBER: u64 = 7;
const RECEIPT_SEQUENCE: u64 = 10;
const ASSET: [u8; 32] = [0x22; 32];
const KEY_ID: [u8; 32] = [0xa1; 32];
const PASSPHRASE: &str = "scoped wallet repair fixture passphrase";
const MNEMONIC: &str =
    "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about";
fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}
fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("hex"), 16).expect("hex byte")
        })
        .collect()
}
fn sequencer() -> SigningKey {
    SigningKey::from_bytes(&[0x51; 32])
}

fn state_leaf(key: &[u8], value: &[u8]) -> [u8; 32] {
    let mut bytes = Vec::with_capacity(26 + key.len() + value.len());
    bytes.extend_from_slice(b"LXP/v1/state-leaf\0");
    bytes.extend_from_slice(
        &u32::try_from(key.len())
            .unwrap_or_else(|_| panic!("test key length"))
            .to_be_bytes(),
    );
    bytes.extend_from_slice(
        &u32::try_from(value.len())
            .unwrap_or_else(|_| panic!("test value length"))
            .to_be_bytes(),
    );
    bytes.extend_from_slice(key);
    bytes.extend_from_slice(value);
    Sha256::digest(bytes).into()
}

fn state_node(left: [u8; 32], right: [u8; 32]) -> [u8; 32] {
    let mut bytes = Vec::with_capacity(82);
    bytes.extend_from_slice(b"LXP/v1/state-node\0");
    bytes.extend_from_slice(&left);
    bytes.extend_from_slice(&right);
    Sha256::digest(bytes).into()
}

fn receipt_bytes(resulting_state_root: [u8; 32], sequencer: &SigningKey) -> Vec<u8> {
    let encode = |signature: Option<[u8; 64]>| {
        let mut encoder = Encoder::new(4096);
        assert_eq!(
            encoder.structure_header_version(0x5201, ACCOUNT_PROTOCOL_VERSION),
            Ok(())
        );
        assert_eq!(encoder.u16(ACCOUNT_PROTOCOL_VERSION), Ok(()));
        assert_eq!(encoder.bytes(&[0x71; 32], 32), Ok(()));
        assert_eq!(encoder.u64(RECEIPT_SEQUENCE), Ok(()));
        assert_eq!(encoder.bytes(&[0x21; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&resulting_state_root, 32), Ok(()));
        assert_eq!(encoder.bytes(&[0x22; 32], 32), Ok(()));
        assert_eq!(encoder.i32(0), Ok(()));
        assert_eq!(encoder.sequence_length(0, 512), Ok(()));
        assert_eq!(encoder.u128(1), Ok(()));
        assert_eq!(encoder.bytes(&[0x23; 32], 32), Ok(()));
        assert_eq!(encoder.u16(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u32(1), Ok(()));
        assert_eq!(encoder.u8(1), Ok(()));
        assert_eq!(encoder.bytes(&[0x24; 32], 32), Ok(()));
        assert_eq!(encoder.u128(25), Ok(()));
        assert_eq!(encoder.bytes(&[0x25; 32], 32), Ok(()));
        assert_eq!(encoder.u128(100), Ok(()));
        assert_eq!(encoder.u128(75), Ok(()));
        assert_eq!(encoder.u64(1), Ok(()));
        assert_eq!(encoder.bytes(&[0x26; 32], 32), Ok(()));
        assert_eq!(encoder.u128(10), Ok(()));
        assert_eq!(encoder.u128(35), Ok(()));
        assert_eq!(encoder.bytes(&[0x27; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[0x28; 32], 32), Ok(()));
        assert_eq!(encoder.bytes(&[0x29; 32], 32), Ok(()));
        assert_eq!(encoder.u64(1_000), Ok(()));
        assert_eq!(encoder.u8(u8::from(signature.is_some())), Ok(()));
        if let Some(signature) = signature {
            assert_eq!(encoder.bytes(&signature, 64), Ok(()));
        }
        encoder.finish()
    };
    let unsigned = encode(None);
    let digest = receipt_digest(&unsigned)
        .unwrap_or_else(|error| panic!("receipt digest failed: {error:?}"));
    encode(Some(sequencer.sign(&digest).to_bytes()))
}

fn header_bytes(
    resulting_state_root: [u8; 32],
    receipt_root: [u8; 32],
    sequencer_id: [u8; 32],
) -> Vec<u8> {
    let mut encoder = Encoder::new(354);
    assert_eq!(
        encoder.structure_header_version(0x1701, ACCOUNT_PROTOCOL_VERSION),
        Ok(())
    );
    assert_eq!(encoder.u8(15), Ok(()));
    let digests: [(u8, [u8; 32]); 8] = [
        (7, [0x31; 32]),
        (8, resulting_state_root),
        (9, [0x32; 32]),
        (10, receipt_root),
        (11, [0x33; 32]),
        (12, [0x34; 32]),
        (13, [0x35; 32]),
        (15, sequencer_id),
    ];
    let counters: [(u8, u64); 5] = [(3, 2), (4, BATCH_NUMBER), (5, 9), (6, 10), (14, 1_000)];
    for field in 1..=15_u8 {
        assert_eq!(encoder.tag(field, 15), Ok(()));
        match field {
            1 => assert_eq!(encoder.u16(ACCOUNT_PROTOCOL_VERSION), Ok(())),
            2 => assert_eq!(encoder.u32(NETWORK_ID), Ok(())),
            _ => {
                if let Some((_, counter)) = counters.iter().find(|(tag, _)| *tag == field) {
                    assert_eq!(encoder.u64(*counter), Ok(()));
                } else {
                    let (_, digest) = digests
                        .iter()
                        .find(|(tag, _)| *tag == field)
                        .unwrap_or_else(|| panic!("header field {field} is undefined"));
                    assert_eq!(encoder.bytes(digest, 32), Ok(()));
                }
            }
        }
    }
    let bytes = encoder.finish();
    assert_eq!(bytes.len(), 354);
    bytes
}

fn nested_fixture(
    account_id: [u8; 32],
    account_value: &[u8],
    sequencer: &SigningKey,
) -> NestedAccountProof {
    let sequencer_id = sequencer.verifying_key().to_bytes();
    let mut account_key = [0_u8; 33];
    account_key[0] = 4;
    account_key[1..].copy_from_slice(&account_id);
    let account_leaf = state_leaf(&account_key, account_value);
    let mut other_account_key = [0xff_u8; 33];
    other_account_key[0] = 4;
    assert!(account_key < other_account_key);
    let other_account_leaf = state_leaf(&other_account_key, b"other-account");
    let account_root = state_node(account_leaf, other_account_leaf);
    let account_proof = Proof::new(0, 2, vec![other_account_leaf])
        .unwrap_or_else(|error| panic!("account proof: {error:?}"));

    let account_tree_leaf = state_leaf(b"account-tree", &account_root);
    let sequence_leaf = state_leaf(b"sequence", &11_u64.to_be_bytes());
    let universal_root = state_node(account_tree_leaf, sequence_leaf);
    let account_tree_proof = Proof::new(0, 2, vec![sequence_leaf])
        .unwrap_or_else(|error| panic!("account-tree proof: {error:?}"));

    let universal_leaf = state_leaf(&0_u16.to_be_bytes(), &universal_root);
    let module_leaf = state_leaf(&1_u16.to_be_bytes(), &[0x61; 32]);
    let resulting_state_root = state_node(universal_leaf, module_leaf);
    let universal_root_proof = Proof::new(0, 2, vec![module_leaf])
        .unwrap_or_else(|error| panic!("universal proof: {error:?}"));

    let receipt = receipt_bytes(resulting_state_root, sequencer);
    let (receipt_proof, receipt_root) = build_proof(&[receipt.as_slice()], 0)
        .unwrap_or_else(|error| panic!("receipt proof: {error:?}"));
    let header = header_bytes(resulting_state_root, receipt_root, sequencer_id);
    let header_digest =
        batch_header_digest(&header).unwrap_or_else(|error| panic!("header digest: {error:?}"));
    NestedAccountProof {
        account_id,
        account_root,
        universal_root,
        resulting_state_root,
        account_proof,
        account_tree_proof,
        universal_root_proof,
        receipt_bytes: receipt,
        receipt_proof,
        header_bytes: header,
        header_signature: sequencer.sign(&header_digest).to_bytes(),
    }
}

fn transaction_receipt(raw: &[u8], reverted: bool) -> Value {
    let (call, signature) = decode_signed(raw).expect("signed EVM transaction");
    let from_address = recover_sender(&call, &signature).expect("secp256k1 recovery");
    let hash = transaction_hash(raw);
    let return_data = if reverted { Vec::new() } else { Vec::new() };
    let vm_error = if reverted { "execution reverted" } else { "" };
    let digest = |bytes: &[u8]| -> [u8; 32] { Sha256::digest(bytes).into() };
    let mut body = Vec::new();
    body.extend_from_slice(&21_000_u64.to_be_bytes());
    body.extend_from_slice(&21_000_u64.to_be_bytes());
    body.extend_from_slice(&2_u64.to_be_bytes());
    body.extend_from_slice(&0_u32.to_be_bytes());
    body.extend_from_slice(&[0; 40]);
    body.extend_from_slice(&digest(vm_error.as_bytes()));
    body.extend_from_slice(&(return_data.len() as u32).to_be_bytes());
    body.extend_from_slice(&0_u32.to_be_bytes());
    body.push(4);
    body.extend_from_slice(b"uhpx");
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&7_u64.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&from_address);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    let encode = |signature: Option<[u8; 64]>| {
        let mut e = Encoder::new(4096);
        e.structure_header_version(0x5201, 2).expect("header");
        e.u16(2).expect("protocol");
        e.bytes(&hash, 32).expect("hash");
        e.u64(10).expect("sequence");
        e.bytes(&[0x21; 32], 32).expect("previous root");
        e.bytes(&[0x22; 32], 32).expect("resulting root");
        e.bytes(&digest(&return_data), 32).expect("return digest");
        e.i32(i32::from(reverted)).expect("result");
        e.sequence_length(1, layerx_wire::limits::MAX_SEQUENCER_EFFECTS)
            .expect("effects");
        e.u16(12).expect("module");
        e.u16(0).expect("ordinal");
        e.u16(0).expect("event");
        e.tag(1, 3).expect("kind");
        e.u8(0).expect("monetary");
        e.bytes(&[0; 32], 32).expect("transfer root");
        e.bytes(&body, 256).expect("summary");
        e.u128(42_000).expect("fee");
        e.bytes(&batch_id, 32).expect("batch id");
        e.u16(12).expect("module");
        e.u32(1).expect("module version");
        e.u32(0).expect("parameter version");
        e.u8(2).expect("operation");
        e.bytes(&digest(b"uhpx"), 32).expect("denom");
        e.u128(0).expect("amount");
        e.bytes(&from, 32).expect("from");
        e.u128(0).expect("balance before");
        e.u128(0).expect("balance after");
        e.u64(call.nonce).expect("nonce");
        e.bytes(&to, 32).expect("to");
        e.u128(0).expect("credit before");
        e.u128(0).expect("credit after");
        e.bytes(&[0; 32], 32).expect("transfer root");
        e.bytes(&[0; 32], 32).expect("authorization");
        e.bytes(&KEY_ID, 32).expect("key id");
        e.u64(1000).expect("timestamp");
        e.u8(u8::from(signature.is_some())).expect("signature flag");
        if let Some(signature) = signature {
            e.bytes(&signature, 64).expect("signature");
        }
        e.finish()
    };
    let unsigned = encode(None);
    let signature = sequencer()
        .sign(&receipt_digest(&unsigned).expect("receipt digest"))
        .to_bytes();
    let canonical = encode(Some(signature));
    assert_eq!(
        layerx_wire::receipt::encode(
            &layerx_wire::receipt::decode(&canonical).expect("receipt decode")
        )
        .expect("receipt encode"),
        canonical
    );
    json!({
        "txHash":format!("0x{}",hex(&hash)), "nonce":format!("0x{:x}",call.nonce),
        "rung":"receipted", "receipt":format!("0x{}",hex(&canonical)), "logs":[],
        "height":"0x7", "seq":"0xa", "timestamp":"0x3e8",
        "keyId":format!("0x{}",hex(&KEY_ID)),
        "signingKey":format!("0x{}",hex(&sequencer().verifying_key().to_bytes())),
        "from":format!("0x{}",hex(&from_address)), "to":format!("0x{}",hex(&call.to)),
        "status":if reverted { "0x0" } else { "0x1" }, "gasUsed":"0x5208",
        "cumulativeGasUsed":"0x5208", "effectiveGasPrice":"0x2", "index":"0x0",
        "contractAddress":format!("0x{}",hex(&[0;20])), "sponsor":format!("0x{}",hex(&[0;20])),
        "feeDenom":"uhpx", "feeAmount":"0xa410", "returnData":format!("0x{}",hex(&return_data)),
        "vmError":vm_error
    })
}
fn credential_vault(
    root: &std::path::Path,
    credential: &str,
) -> Result<(), Box<dyn std::error::Error>> {
    let directory = root.join("credentials");
    fs::DirBuilder::new().mode(0o700).create(&directory)?;
    let mut salt = [0; 16];
    let mut nonce = [0; 12];
    getrandom::fill(&mut salt)?;
    getrandom::fill(&mut nonce)?;
    let mut key = zeroize::Zeroizing::new([0; 32]);
    pbkdf2::derive(
        pbkdf2::PBKDF2_HMAC_SHA256,
        NonZeroU32::new(600_000).ok_or("invalid KDF iterations")?,
        &salt,
        PASSPHRASE.as_bytes(),
        key.as_mut(),
    );
    let key = aead::LessSafeKey::new(
        aead::UnboundKey::new(&aead::AES_256_GCM, key.as_ref()).map_err(|_| "invalid vault key")?,
    );
    let mut ciphertext = serde_json::to_vec(&json!({"gateway:watch": credential}))?;
    key.seal_in_place_append_tag(
        aead::Nonce::assume_unique_for_key(nonce),
        aead::Aad::from(b"LXVAULT1"),
        &mut ciphertext,
    )
    .map_err(|_| "vault encryption failed")?;
    let mut file = OpenOptions::new()
        .write(true)
        .create_new(true)
        .mode(0o600)
        .open(directory.join("vault"))?;
    for bytes in [b"LXVAULT1".as_slice(), &salt, &nonce, &ciphertext] {
        file.write_all(bytes)?;
    }
    Ok(())
}

struct Fixture {
    directory: tempfile::TempDir,
    account: DerivedAccount,
}
impl Fixture {
    fn new() -> Self {
        let directory = tempfile::tempdir().expect("fixture directory");
        let account = derive_from_mnemonic(MNEMONIC, "", 0).expect("derived account");
        let fixture = Self { directory, account };
        fs::write(fixture.path("mnemonic"), MNEMONIC).expect("phrase fixture");
        let output = fixture
            .command()
            .args(["wallet", "derive", "--mnemonic-file"])
            .arg(fixture.path("mnemonic"))
            .arg("--export-private-keys")
            .arg(fixture.path("evm.json"))
            .output()
            .expect("derive CLI");
        assert!(
            output.status.success(),
            "derive failed without exporting an account"
        );
        // Only public identity metadata is installed; no OS credential backend is needed.
        fs::write(
            fixture.path("config.json"),
            json!({"version":1,"current_environment":"beta",
            "default_key":"wallet","keys":{"wallet":{"did":fixture.account.did(),
            "public_key":hex(&fixture.account.layerx_public_key())}},
            "environments":{"beta":{"endpoint":"http://127.0.0.1:1","network_id":125}}})
            .to_string(),
        )
        .expect("public configuration");
        fs::write(fixture.path("policy.json"),json!({"protocol_version":2,"network_id":125,
            "sequencer_id":hex(&KEY_ID),"sequencer_key":hex(&sequencer().verifying_key().to_bytes()),
            "first_batch":1,"last_batch":100}).to_string()).expect("pinned receipt policy");
        fixture
    }
    fn path(&self, name: &str) -> std::path::PathBuf {
        self.directory.path().join(name)
    }
    fn command(&self) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx"));
        command
            .env_clear()
            .env("LAYERX_CONFIG", self.path("config.json"))
            .env("LAYERX_CREDENTIAL_STORE", "file")
            .env("LAYERX_CREDENTIAL_PASSPHRASE", PASSPHRASE)
            .args(["--json"]);
        command
    }
    fn write(&self, endpoint: &str, wait: &str) -> Output {
        self.command()
            .args([
                "--rpc",
                endpoint,
                "wallet",
                "open-account",
                "--asset",
                &hex(&ASSET),
                "--evm-key-file",
            ])
            .arg(self.path("evm.json"))
            .arg("--receipt-policy")
            .arg(self.path("policy.json"))
            .args([
                "--fee-limit",
                "100000",
                "--wait",
                wait,
                "--timeout-seconds",
                "3",
            ])
            .output()
            .expect("wallet write CLI")
    }
    fn receipt(&self, endpoint: &str, hash: &str, wait: &str) -> Output {
        self.command()
            .args([
                "--rpc",
                endpoint,
                "wallet",
                "receipt",
                hash,
                "--wait",
                wait,
                "--receipt-policy",
            ])
            .arg(self.path("policy.json"))
            .output()
            .expect("receipt CLI")
    }
}

struct Server {
    endpoint: String,
    done: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
    requests: Arc<Mutex<Vec<(String, String, Value)>>>,
}
impl Server {
    fn new(mut handle: impl FnMut(&str, &str, &Value) -> Option<Value> + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback listener");
        let endpoint = format!(
            "http://{}",
            listener.local_addr().expect("listener address")
        );
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let done = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&done);
        let requests = Arc::new(Mutex::new(Vec::new()));
        let captured = Arc::clone(&requests);
        let worker = thread::spawn(move || {
            let deadline = Instant::now() + Duration::from_secs(30);
            while !stop.load(Ordering::SeqCst) {
                assert!(Instant::now() < deadline, "fixture listener deadline");
                let socket = match listener.accept() {
                    Ok((socket, _)) => socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("listener: {error}"),
                };
                socket
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .expect("socket deadline");
                let mut reader = std::io::BufReader::new(socket);
                let mut line = String::new();
                reader.read_line(&mut line).expect("request line");
                let path = line
                    .split_whitespace()
                    .nth(1)
                    .expect("request path")
                    .to_owned();
                let mut length = None;
                let mut authorization = String::new();
                loop {
                    line.clear();
                    assert_ne!(reader.read_line(&mut line).expect("header"), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(value.trim().parse::<usize>().expect("body length"));
                        }
                        if name.eq_ignore_ascii_case("authorization") {
                            authorization = value.trim().to_owned();
                        }
                    }
                }
                let length = length.expect("body length header");
                assert!(length <= 1_048_576);
                let mut body = vec![0; length];
                reader.read_exact(&mut body).expect("request body");
                let request: Value = serde_json::from_slice(&body).expect("JSON-RPC request");
                captured.lock().expect("requests").push((
                    path.clone(),
                    authorization.clone(),
                    request.clone(),
                ));
                if let Some(result) = handle(&path, &authorization, &request) {
                    let body =
                        json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                    write!(reader.get_mut(),"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",body.len(),body).expect("response");
                }
            }
        });
        Self {
            endpoint,
            done,
            worker: Some(worker),
            requests,
        }
    }
    fn stop(mut self) -> Vec<(String, String, Value)> {
        self.done.store(true, Ordering::SeqCst);
        self.worker
            .take()
            .expect("worker")
            .join()
            .expect("loopback producer");
        self.requests.lock().expect("requests").clone()
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.done.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            worker.join().expect("loopback producer");
        }
    }
}

fn node_server(raw: Arc<Mutex<Option<Vec<u8>>>>, lose_reply: bool, tamper: bool) -> Server {
    Server::new(move |path, _, request| {
        assert_eq!(path, "/", "Ethereum traffic must use the node root");
        match request["method"].as_str().expect("method") {
            "eth_getTransactionCount" => Some(json!("0x11")),
            "eth_gasPrice" => Some(json!("0x2")),
            "eth_estimateGas" => {
                assert_eq!(request["params"][1], "pending");
                let transaction = &request["params"][0];
                assert_eq!(transaction["type"], "0x2");
                assert_eq!(transaction["chainId"], "0x7d");
                assert_eq!(transaction["nonce"], "0x11");
                assert_eq!(transaction["maxPriorityFeePerGas"], "0x0");
                assert_eq!(transaction["maxFeePerGas"], "0x2");
                assert_eq!(transaction["gas"], "0xc350");
                assert_eq!(transaction["value"], "0x0");
                assert_eq!(transaction["accessList"], json!([]));
                layerx_platform_cli::rpc::request("eth_estimateGas", &request["params"])
                    .expect("complete production estimate contract");
                Some(json!("0x5208"))
            }
            "eth_sendRawTransaction" => {
                let bytes = unhex(request["params"][0].as_str().expect("raw bytes"));
                decode_signed(&bytes).expect("production signed transaction");
                let hash = transaction_hash(&bytes);
                assert!(
                    raw.lock()
                        .expect("accepted transactions")
                        .replace(bytes)
                        .is_none(),
                    "no second signed transaction"
                );
                if lose_reply {
                    None
                } else {
                    Some(json!(format!("0x{}", hex(&hash))))
                }
            }
            "pax_sequencerReceipt" => {
                let raw = raw.lock().expect("accepted transaction");
                let bytes = raw.as_ref().expect("accepted bytes");
                assert_eq!(
                    request["params"][0],
                    format!("0x{}", hex(&transaction_hash(bytes)))
                );
                let mut receipt = transaction_receipt(bytes, false);
                receipt["rung"] = json!("committed");
                receipt["blockNumber"] = json!("0x8");
                receipt["blockHash"] = json!(format!("0x{}", hex(&[0x91; 32])));
                if tamper {
                    receipt["gasUsed"] = json!("0x1");
                }
                Some(receipt)
            }
            "eth_getTransactionReceipt" => Some(json!({"transactionHash":request["params"][0],
                "blockNumber":"0x8","blockHash":format!("0x{}",hex(&[0x91;32])),"status":"0x1","transactionIndex":"0x0"})),
            method => panic!("unexpected node method {method}"),
        }
    })
}

fn public_result(output: &Output) -> Value {
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    serde_json::from_slice::<Value>(&output.stdout).expect("CLI JSON")["data"].clone()
}

#[test]
fn wallet_finality_option_committed_receipt_for_every_wait() {
    for wait in [
        "submitted",
        "receipted",
        "committed",
        "executed",
        "batched",
        "finalised",
    ] {
        let fixture = Fixture::new();
        let raw = Arc::new(Mutex::new(None));
        let server = node_server(raw, false, false);
        let result = public_result(&fixture.write(&server.endpoint, wait));
        if wait == "submitted" {
            assert_eq!(result["rung"], "submitted");
            assert_eq!(result["final"], false);
        } else {
            assert_eq!(result["rung"], "committed");
            assert_eq!(result["final"], true);
        }
        let hash = result["txHash"].as_str().expect("retained hash");
        let receipt = public_result(&fixture.receipt(&server.endpoint, hash, "committed"));
        assert_eq!(receipt["rung"], "committed");
        assert_eq!(receipt["final"], true);
        let requests = server.stop();
        assert_eq!(
            requests
                .iter()
                .filter(|(_, _, r)| r["method"] == "eth_sendRawTransaction")
                .count(),
            1
        );
        assert!(requests
            .iter()
            .any(|(_, _, r)| r["method"] == "eth_getTransactionReceipt"));
    }
    let fixture = Fixture::new();
    let raw = Arc::new(Mutex::new(None));
    let server = node_server(raw, false, true);
    let output = fixture.write(&server.endpoint, "committed");
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("receipt_verification_failed"));
    server.stop();
}

#[test]
fn wallet_submission_recovery_reopens_journal_without_resubmission() {
    let fixture = Fixture::new();
    let raw = Arc::new(Mutex::new(None));
    let server = node_server(Arc::clone(&raw), true, false);
    let output = fixture.write(&server.endpoint, "committed");
    assert!(!output.status.success());
    let bytes = raw
        .lock()
        .expect("accepted bytes")
        .clone()
        .expect("accepted transaction");
    let hash = format!("0x{}", hex(&transaction_hash(&bytes)));
    let error = String::from_utf8_lossy(&output.stderr);
    assert!(error.contains("transaction_indeterminate"));
    assert!(error.contains(&hash));
    let journal = fixture
        .path("config.wallet-submissions")
        .join(format!("{}.json", hex(&transaction_hash(&bytes))));
    let retained: Value = serde_json::from_slice(&fs::read(&journal).expect("durable journal"))
        .expect("journal JSON");
    assert_eq!(retained["tx_hash"], hash);
    assert_eq!(retained["state"], "unknown");
    assert_eq!(retained["nonce"], 17);
    let requests = server.stop();
    assert_eq!(
        requests
            .iter()
            .filter(|(_, _, r)| r["method"] == "eth_sendRawTransaction")
            .count(),
        1
    );
    // The writer process has exited and the listener has stopped. Reopen both from retained state.
    let server = node_server(raw, false, false);
    let result = public_result(&fixture.receipt(&server.endpoint, &hash, "committed"));
    assert_eq!(result["txHash"], hash);
    assert_eq!(result["rung"], "committed");
    let recovered: Value = serde_json::from_slice(&fs::read(journal).expect("reopened journal"))
        .expect("journal JSON");
    assert_eq!(recovered["state"], "committed");
    assert!(server
        .stop()
        .iter()
        .all(|(_, _, r)| r["method"] != "eth_sendRawTransaction"));
}

#[test]
fn retired_endpoint_wallet_separates_root_and_hosted_routes() {
    let fixture = Fixture::new();
    let did = fixture.account.did();
    let account = layerx_platform_cli::wallet_gateway::account_document(
        Some(fixture.account.evm_address()),
        None,
        Some(fixture.account.layerx_public_key()),
        None,
    );
    let document = layerx_platform_cli::wallet_gateway::balances(
        account,
        8,
        vec![json!({"denom":"uhpx","amount":"100"})],
    );
    credential_vault(
        fixture.directory.path(),
        &format!("watch:lxp_live_{}", "a".repeat(64)),
    )
    .expect("protected credential fixture");
    let server = Server::new(move |path, authorization, request| {
        assert_eq!(path, "/services/accounts");
        assert_eq!(
            authorization,
            format!("LayerX-Key watch:lxp_live_{}", "a".repeat(64))
        );
        assert_eq!(request["method"], "px_getBalances");
        assert_eq!(request["params"], json!([did]));
        Some(document.clone())
    });
    let node = Server::new(|_, _, _| panic!("hosted balances must not reach the node"));
    let output = fixture
        .command()
        .args([
            "--rpc",
            &node.endpoint,
            "--gateway-credential",
            "watch",
            "wallet",
            "balance",
            "--did",
            &fixture.account.did(),
            "--gateway-rpc",
            &format!("{}/services/accounts", server.endpoint),
        ])
        .output()
        .expect("hosted read CLI");
    let result = public_result(&output);
    assert_eq!(result["balances"][0]["amount"], "100");
    assert_eq!(result["verification"], "unverified_node_observation");
    assert!(node.stop().is_empty());
    assert_eq!(server.stop().len(), 1);
    let refused = fixture.write("http://example.invalid", "committed");
    assert!(!refused.status.success());
    assert!(String::from_utf8_lossy(&refused.stderr).contains("InvalidEndpoint"));
}

fn account_snapshot_fixture(did: &str) -> (Value, Value) {
    let name = format!("agent:{did}:asset:{}", hex(&ASSET));
    let parsed = layerx_types::account::AccountId::parse(&name).expect("account name");
    let id = account_id_for_protocol(&parsed, 3).expect("account identifier");
    let mut canonical = (name.len() as u16).to_be_bytes().to_vec();
    canonical.extend_from_slice(name.as_bytes());
    canonical.push(14);
    canonical.extend_from_slice(&1000_u128.to_be_bytes());
    canonical.extend_from_slice(&ASSET);
    canonical.push(1);
    canonical.extend_from_slice(&19_u64.to_be_bytes());
    canonical.extend_from_slice(&3_u64.to_be_bytes());
    canonical.extend_from_slice(&[0, 1]);
    canonical.extend_from_slice(&[0; 32]);
    canonical.push(0);
    assert_eq!(
        decode_account_value(id, &canonical)
            .expect("canonical account")
            .balance(),
        1000
    );
    let proof = nested_fixture(id, &canonical, &sequencer());
    let header = HeaderRecord {
        canonical_header: proof.header_bytes.clone(),
        signature: proof.header_signature,
        sequencer_id: sequencer().verifying_key().to_bytes(),
        public_key: sequencer().verifying_key().to_bytes(),
        first_batch: 1,
        last_batch: 100,
    };
    let state = AccountStateRecord {
        reference: FactSelector::State {
            activity_id: [0x71; 32],
            account_id: id,
        },
        variant: AccountStateVariant::Activity,
        account_id: id,
        account_value: canonical.clone(),
        account_root: proof.account_root,
        universal_root: proof.universal_root,
        resulting_state_root: proof.resulting_state_root,
        account_proof: proof.account_proof,
        account_tree_proof: proof.account_tree_proof,
        universal_root_proof: proof.universal_root_proof,
        receipt_bytes: proof.receipt_bytes,
        receipt_proof: proof.receipt_proof,
        header_digest: header.digest().expect("header digest"),
    };
    let snapshot = json!({"did":did,"verification":"state_proven","accounts":[{
        "account_id":hex(&id),"name":name,"asset_id":hex(&ASSET),"balance":"1000","next_sequence":"19",
        "observed_head_sequence":"10","batch_number":"7","canonical_value":hex(&canonical),
        "proof_material":hex(&state.encode().expect("production account proof codec")),
        "header_material":hex(&header.encode().expect("production header codec"))}]});
    let trust = json!({"protocol_version":3,"network_id":NETWORK_ID,"batch_number":7,
        "state_root":hex(&proof.resulting_state_root),"sequencer_id":hex(&sequencer().verifying_key().to_bytes()),
        "sequencer_key":hex(&sequencer().verifying_key().to_bytes()),"first_batch":1,"last_batch":100});
    (snapshot, trust)
}

#[test]
fn wallet_snapshot_proof_label_binds_canonical_fields_and_pinned_root() {
    let fixture = Fixture::new();
    let did = fixture.account.did();
    let (snapshot, trust) = account_snapshot_fixture(&did);
    let run = |snapshot: &Value, trust: &Value| {
        fs::write(fixture.path("snapshot.json"), snapshot.to_string())
            .expect("snapshot codec output");
        fs::write(fixture.path("account-policy.json"), trust.to_string())
            .expect("independent root pin");
        fixture
            .command()
            .args(["wallet", "balance", "--did", &did, "--proof-snapshot"])
            .arg(fixture.path("snapshot.json"))
            .arg("--proof-policy")
            .arg(fixture.path("account-policy.json"))
            .output()
            .expect("proof consumer CLI")
    };
    let result = public_result(&run(&snapshot, &trust));
    assert_eq!(result["verification"], "state_proven");
    assert_eq!(result["accounts"][0]["balance"], "1000");
    assert_eq!(result["accounts"][0]["next_sequence"], "19");
    for (field, value) in [
        ("balance", json!("1001")),
        ("next_sequence", json!("20")),
        ("asset_id", json!(hex(&[0x33; 32]))),
        ("observed_head_sequence", json!("11")),
        ("batch_number", json!("8")),
    ] {
        let mut changed = snapshot.clone();
        changed["accounts"][0][field] = value;
        assert_eq!(changed["verification"], "state_proven");
        let output = run(&changed, &trust);
        assert!(!output.status.success(), "tampered {field} accepted");
        assert!(
            String::from_utf8_lossy(&output.stderr).contains("account_proof_verification_failed")
        );
    }
    let mut changed = snapshot.clone();
    let mut bytes = unhex(
        changed["accounts"][0]["canonical_value"]
            .as_str()
            .expect("canonical value"),
    );
    let balance_offset = 2
        + changed["accounts"][0]["name"]
            .as_str()
            .expect("account name")
            .len()
        + 1;
    bytes[balance_offset + 15] ^= 1;
    changed["accounts"][0]["canonical_value"] = json!(hex(&bytes));
    assert!(
        !run(&changed, &trust).status.success(),
        "canonical tampering accepted"
    );
    let mut wrong_root = trust.clone();
    wrong_root["state_root"] = json!(hex(&[0x81; 32]));
    assert!(
        !run(&snapshot, &wrong_root).status.success(),
        "untrusted root accepted"
    );
    let mut wrong_key = trust.clone();
    wrong_key["sequencer_key"] = json!(hex(&SigningKey::from_bytes(&[0x19; 32])
        .verifying_key()
        .to_bytes()));
    assert!(
        !run(&snapshot, &wrong_key).status.success(),
        "untrusted signer accepted"
    );
}

#[test]
fn wallet_key_file_open_qualifies_protected_derived_exports() {
    let fixture = Fixture::new();
    let did = fixture.account.did();
    let public = fixture.account.layerx_public_key();
    let valid = fixture.path("evm.json");
    let key = EvmSigningKey::read(&valid, &did, public).expect("real derived export");
    assert_eq!(key.address(), fixture.account.evm_address());
    let link = fixture.path("symlink");
    std::os::unix::fs::symlink(&valid, &link).expect("symlink fixture");
    assert!(EvmSigningKey::read(&link, &did, public).is_err());
    let socket = fixture.path("socket");
    let _listener = std::os::unix::net::UnixListener::bind(&socket)
        .expect("standard-library nonregular file fixture");
    let (sender, receiver) = std::sync::mpsc::sync_channel(1);
    let owner = did.clone();
    thread::spawn(move || {
        sender
            .send(EvmSigningKey::read(&socket, &owner, public).is_err())
            .expect("nonregular file result");
    });
    assert!(receiver
        .recv_timeout(Duration::from_secs(1))
        .expect("nonregular key input must never block at open"));
    let foreign = fixture.path("foreign");
    fs::copy(&valid, &foreign).expect("foreign-owner fixture");
    rustix::fs::chown(
        &foreign,
        Some(rustix::process::Uid::from_raw(
            rustix::process::geteuid().as_raw() + 1,
        )),
        None,
    )
    .expect("foreign owner");
    fs::set_permissions(&foreign, fs::Permissions::from_mode(0o600))
        .expect("owner-only foreign fixture");
    assert!(EvmSigningKey::read(&foreign, &did, public).is_err());
    fs::set_permissions(&valid, fs::Permissions::from_mode(0o640)).expect("unsafe mode");
    assert!(EvmSigningKey::read(&valid, &did, public).is_err());
    fs::set_permissions(&valid, fs::Permissions::from_mode(0o600)).expect("restore protected mode");
    fs::write(&valid, vec![b'x'; 4097]).expect("oversized export");
    let error = EvmSigningKey::read(&valid, &did, public)
        .err()
        .expect("byte bound");
    assert!(error.contains("4096"));
    assert!(!error.contains(&"x".repeat(64)));
}

#[test]
fn workspace_retired_core_uses_current_domain_targets() {
    let fixture = Fixture::new();
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../..");
    let output = fixture
        .command()
        .env("LAYERX_REPO_ROOT", root)
        .args(["workspace", "test", "--module", "core", "--dry-run"])
        .output()
        .expect("workspace plan CLI");
    let result = public_result(&output);
    let steps = result["steps"].as_array().expect("workspace steps");
    assert!(steps.iter().all(|step| step["status"] == "planned"));
    let commands: Vec<_> = steps
        .iter()
        .map(|step| step["command"].as_str().expect("planned command"))
        .collect();
    assert!(commands
        .iter()
        .any(|command| command.contains("test-paxd-domain")));
    assert!(commands
        .iter()
        .any(|command| command.contains("test-kernel-symbol-isolation")));
    assert!(!result.to_string().contains("test-layerxd"));
    assert!(!result.to_string().contains("guarantor"));
}
