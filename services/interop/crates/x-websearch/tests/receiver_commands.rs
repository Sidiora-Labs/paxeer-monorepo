#[path = "support/ledger_assets.rs"]
mod ledger_assets;

mod test_network {
    include!(concat!(env!("CARGO_MANIFEST_DIR"), "/../test_network.rs"));
}

use std::collections::BTreeMap;
use std::io::{Read as _, Write as _};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Output, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::account_derivation::derive_from_mnemonic;
use serde_json::{json, Value};
use x_websearch::keys::{ATTESTOR_KEY_FILE, RECEIVER_KEY_FILE, SUBMITTER_KEY_FILE};
use x_websearch::payment::{hex, unhex, LEDGER_PRECOMPILE};
use x_websearch::receiver::{binding_calldata, binding_message, ADDR_PRECOMPILE};
use x_websearch::watch::{hex0x, keccak, unhex0x, XWEB_PRECOMPILE};

type Outcome<T = ()> = Result<T, Box<dyn std::error::Error>>;

const ACCOUNT_VECTORS: &str =
    include_str!("../../../../../sdk/platform/conformance/fixtures/account-derivation-v1.json");
const BIND_VECTORS: &str =
    include_str!("../../../../../layerxproof/testdata/paxeer_bind_vectors.json");
const CONFIG: &str = include_str!("fixtures/binary/config.json");
const GAS: u64 = 200_000;
const GAS_PRICE: u64 = 3;
const TX_NONCE: u64 = 7;

struct Scratch(PathBuf);

impl Scratch {
    fn new() -> Outcome<Self> {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        test_network::isolate()?;
        let directory = std::env::temp_dir().join(format!(
            "x-websearch-receiver-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&directory)?;
        Ok(Self(directory))
    }

    fn key(&self, name: &str, bytes: &[u8]) -> Outcome<PathBuf> {
        let path = self.0.join(name);
        std::fs::write(&path, format!("0x{}\r\n", hex(bytes)))?;
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o600))?;
        Ok(path)
    }
}

impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn word(value: u64) -> Vec<u8> {
    [&[0; 24][..], &value.to_be_bytes()].concat()
}

fn fixed<const N: usize>(text: &str) -> Outcome<[u8; N]> {
    unhex(text.strip_prefix("0x").unwrap_or(text))
        .ok_or("hexadecimal bytes")?
        .try_into()
        .map_err(|_| "byte count".into())
}

fn rlp_item(bytes: &[u8]) -> Outcome<(&[u8], usize)> {
    let tag = *bytes.first().ok_or("empty RLP")?;
    if tag < 128 {
        return Ok((&bytes[..1], 1));
    }
    let short = if tag < 192 { 128 } else { 192 };
    let (start, length) = if tag < short + 56 {
        (1, usize::from(tag - short))
    } else {
        let width = usize::from(tag - short - 55);
        let encoded = bytes.get(1..1 + width).ok_or("RLP length")?;
        let mut length = 0_usize;
        for byte in encoded {
            length = length
                .checked_mul(256)
                .and_then(|n| n.checked_add(usize::from(*byte)))
                .ok_or("RLP overflow")?;
        }
        (1 + width, length)
    };
    let end = start.checked_add(length).ok_or("RLP overflow")?;
    Ok((bytes.get(start..end).ok_or("RLP payload")?, end))
}

fn transaction_fields(raw: &[u8]) -> Outcome<Vec<&[u8]>> {
    if raw.first().copied().ok_or("transaction")? < 192 {
        return Err("legacy transaction list required".into());
    }
    let (mut body, used) = rlp_item(raw)?;
    if used != raw.len() {
        return Err("trailing transaction bytes".into());
    }
    let mut fields = Vec::new();
    while !body.is_empty() {
        let (field, used) = rlp_item(body)?;
        fields.push(field);
        body = &body[used..];
    }
    Ok(fields)
}

fn rlp(bytes: &[u8], list: bool) -> Outcome<Vec<u8>> {
    if !list && bytes.len() == 1 && bytes[0] < 128 {
        return Ok(bytes.to_vec());
    }
    let base = if list { 192 } else { 128 };
    let mut out = Vec::new();
    if bytes.len() < 56 {
        out.push(base + u8::try_from(bytes.len())?);
    } else {
        let length = bytes.len().to_be_bytes();
        let digits: Vec<_> = length.into_iter().skip_while(|byte| *byte == 0).collect();
        out.push(base + 55 + u8::try_from(digits.len())?);
        out.extend_from_slice(&digits);
    }
    out.extend_from_slice(bytes);
    Ok(out)
}

struct State {
    address: [u8; 20],
    identity: [u8; 32],
    bound: Option<[u8; 32]>,
    balance: u64,
    calls: Vec<(String, Value)>,
    transactions: Vec<Vec<u8>>,
    hash: Option<[u8; 32]>,
    fail_receipt: bool,
    pending_receipts: usize,
    wrong_confirmation: bool,
    malformed_identity: bool,
    malformed_account: bool,
    malformed_nonce: bool,
    chain_id: u64,
    assets: Vec<[u8; 32]>,
}

fn dispatch(state: &mut State, method: &str, params: &Value) -> Outcome<Value> {
    match method {
        "eth_chainId" => Ok(json!(format!("0x{:x}", state.chain_id))),
        "eth_getBalance" => {
            assert_eq!(params[0], hex0x(&state.address));
            Ok(json!(format!("0x{:x}", state.balance)))
        }
        "eth_gasPrice" => Ok(json!(format!("0x{GAS_PRICE:x}"))),
        "eth_estimateGas" => {
            assert_eq!(params[0]["from"], hex0x(&state.address));
            assert_eq!(params[0]["to"], hex0x(&ADDR_PRECOMPILE));
            assert_eq!(params[0]["value"], "0x0");
            Ok(json!(format!("0x{GAS:x}")))
        }
        "eth_getTransactionCount" => {
            assert_eq!(*params, json!([hex0x(&state.address), "pending"]));
            Ok(json!(format!("0x{TX_NONCE:x}")))
        }
        "eth_sendRawTransaction" => {
            let raw =
                unhex0x(params[0].as_str().ok_or("raw transaction")?).ok_or("transaction hex")?;
            let hash = keccak(&raw);
            state.hash = Some(hash);
            state.transactions.push(raw);
            Ok(json!(hex0x(&hash)))
        }
        "eth_getTransactionReceipt" => {
            let hash = state.hash.ok_or("no broadcast")?;
            assert_eq!(*params, json!([hex0x(&hash)]));
            if state.pending_receipts > 0 {
                state.pending_receipts -= 1;
                return Ok(Value::Null);
            }
            if !state.fail_receipt {
                state.bound = Some(if state.wrong_confirmation {
                    [0x32; 32]
                } else {
                    state.identity
                });
            }
            Ok(
                json!({"transactionHash": hex0x(&hash), "status": if state.fail_receipt { "0x0" } else { "0x1" }}),
            )
        }
        "pax_sequencerReceipt" => Ok(Value::Null),
        "eth_call" => {
            let data = unhex0x(params[0]["data"].as_str().ok_or("call data")?).ok_or("call hex")?;
            let selector = data.get(..4).ok_or("call selector")?;
            if params[0]["to"] == hex0x(&XWEB_PRECOMPILE)
                && selector == &keccak(b"getKernelNetworkId()")[..4]
            {
                return Ok(json!(hex0x(&word(1))));
            }
            if let Some(record) = ledger_assets::get_asset_answer(params) {
                return Ok(record);
            }
            if params[0]["to"] == hex0x(&LEDGER_PRECOMPILE) {
                assert_eq!(selector, &keccak(b"accountId(bytes32,bytes32)")[..4]);
                assert_eq!(
                    &data[4..36],
                    state.bound.ok_or("unbound account read")?.as_slice()
                );
                let index = state
                    .assets
                    .iter()
                    .position(|asset| asset.as_slice() == &data[36..])
                    .ok_or("asset")?;
                let account = if state.malformed_account {
                    [0; 32]
                } else {
                    keccak(&state.assets[index])
                };
                return Ok(json!(hex0x(&account)));
            }
            assert_eq!(params[0]["to"], hex0x(&ADDR_PRECOMPILE));
            assert_eq!(&data[4..16], &[0; 12]);
            assert_eq!(&data[16..], state.address.as_slice());
            if selector == &keccak(b"getUnifiedAccount(address)")[..4] {
                if state.malformed_identity {
                    return Ok(json!("0x00"));
                }
                return Ok(json!(hex0x(
                    &[
                        &[0; 12][..],
                        &state.address,
                        &word(128),
                        &state.bound.unwrap_or([0; 32]),
                        &[0; 32],
                        &word(0),
                    ]
                    .concat()
                )));
            }
            if selector == &keccak(b"layerXBindNonce(address)")[..4] {
                let nonce = if state.malformed_nonce {
                    vec![1; 32]
                } else {
                    word(0)
                };
                return Ok(json!(hex0x(&nonce)));
            }
            if selector == &keccak(b"getLayerXDid(address)")[..4] {
                let key = state.bound.ok_or("address is not bound to a LayerX DID")?;
                let did = format!("did:layerx:{}", hex(&key));
                let mut answer = [
                    &key[..],
                    &word(64),
                    &word(u64::try_from(did.len())?),
                    did.as_bytes(),
                ]
                .concat();
                answer.resize(192, 0);
                return Ok(json!(hex0x(&answer)));
            }
            Err("unknown address call".into())
        }
        _ => Err("unsupported method".into()),
    }
}

fn serve(mut stream: TcpStream, state: &Mutex<State>) -> Outcome {
    stream.set_read_timeout(Some(Duration::from_secs(3)))?;
    let mut bytes = Vec::new();
    let mut buffer = [0; 2048];
    let (head_end, length) = loop {
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let header = String::from_utf8_lossy(&bytes[..end]).to_ascii_lowercase();
            let length: usize = header
                .split("\r\n")
                .find_map(|line| line.strip_prefix("content-length:"))
                .ok_or("content length")?
                .trim()
                .parse()?;
            if bytes.len() >= end + 4 + length {
                break (end + 4, length);
            }
        }
        let count = stream.read(&mut buffer)?;
        if count == 0 {
            return Err("incomplete request".into());
        }
        bytes.extend_from_slice(&buffer[..count]);
    };
    let request: Value = serde_json::from_slice(&bytes[head_end..head_end + length])?;
    let method = request["method"].as_str().ok_or("method")?;
    let params = &request["params"];
    let mut state = state.lock().map_err(|_| "state lock")?;
    state.calls.push((method.to_owned(), params.clone()));
    let response = match dispatch(&mut state, method, params) {
        Ok(value) => json!({"jsonrpc":"2.0", "id":request["id"], "result":value}),
        Err(error) => json!({"jsonrpc":"2.0", "id":request["id"], "error":{"code":-32000,"message":error.to_string()}}),
    }.to_string();
    stream.write_all(format!("HTTP/1.1 200 OK\r\nConnection: close\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{response}", response.len()).as_bytes())?;
    Ok(())
}

struct Endpoint {
    address: SocketAddr,
    state: Arc<Mutex<State>>,
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
}

impl Endpoint {
    fn start(state: State) -> Outcome<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let state = Arc::new(Mutex::new(state));
        let stop = Arc::new(AtomicBool::new(false));
        let (worker_state, worker_stop) = (Arc::clone(&state), Arc::clone(&stop));
        let worker = thread::spawn(move || {
            for stream in listener.incoming() {
                if worker_stop.load(Ordering::SeqCst) {
                    break;
                }
                if let Ok(stream) = stream {
                    let _ = serve(stream, &worker_state);
                }
            }
        });
        Ok(Self {
            address,
            state,
            stop,
            worker: Some(worker),
        })
    }

    fn url(&self) -> String {
        format!("http://{}/rpc", self.address)
    }
}

impl Drop for Endpoint {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            assert!(worker.join().is_ok());
        }
    }
}

struct Setup {
    scratch: Scratch,
    endpoint: Endpoint,
    occupied: TcpListener,
    receiver: PathBuf,
    identity: PathBuf,
    config: Value,
    expected_data: Vec<u8>,
}

impl Setup {
    fn new() -> Outcome<Self> {
        let scratch = Scratch::new()?;
        let fixture: Value = serde_json::from_str(ACCOUNT_VECTORS)?;
        let vector = &fixture["mnemonic_vectors"][0];
        let account = derive_from_mnemonic(
            vector["mnemonic"].as_str().ok_or("mnemonic")?,
            vector["passphrase"].as_str().ok_or("passphrase")?,
            0,
        )?;
        let receiver = scratch.key("receiver.key", account.evm_secret().ok_or("EVM secret")?)?;
        let identity = scratch.key("identity.key", account.layerx_seed())?;
        let mut config: Value = serde_json::from_str(CONFIG)?;
        let assets = ["SID", "PAX", "USDC", "USDL"]
            .iter()
            .map(|name| fixed(config["assets"][name]["asset_id"].as_str().ok_or("asset")?))
            .collect::<Outcome<Vec<_>>>()?;
        let endpoint = Endpoint::start(State {
            address: account.evm_address(),
            identity: account.layerx_public_key(),
            bound: None,
            balance: GAS * GAS_PRICE,
            calls: Vec::new(),
            transactions: Vec::new(),
            hash: None,
            fail_receipt: false,
            pending_receipts: 0,
            wrong_confirmation: false,
            malformed_identity: false,
            malformed_account: false,
            malformed_nonce: false,
            chain_id: 125,
            assets,
        })?;
        let occupied = TcpListener::bind("127.0.0.1:0")?;
        config["listen"] = json!(occupied.local_addr()?.to_string());
        config["data_dir"] = json!(scratch.0.join("data"));
        config["evm"]["endpoint"] = json!("http://127.0.0.1:1");
        config["evm"]["chain_id"] = json!(125);
        config["gateway"]["endpoint"] = json!(endpoint.url());
        let expected_data = unhex(
            vector["accounts"][0]["bind"]["calldata"]
                .as_str()
                .ok_or("calldata")?,
        )
        .ok_or("calldata hex")?;
        let setup = Self {
            scratch,
            endpoint,
            occupied,
            receiver,
            identity,
            config,
            expected_data,
        };
        setup.save()?;
        Ok(setup)
    }

    fn save(&self) -> Outcome {
        Ok(std::fs::write(
            self.scratch.0.join("config.json"),
            self.config.to_string(),
        )?)
    }

    fn command(&self) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_x-websearch"));
        command
            .arg("--config")
            .arg(self.scratch.0.join("config.json"))
            .env(RECEIVER_KEY_FILE, &self.receiver)
            .env(ATTESTOR_KEY_FILE, self.scratch.0.join("absent-attestor"))
            .env(SUBMITTER_KEY_FILE, self.scratch.0.join("absent-submitter"));
        command
    }

    fn bind(&self) -> Outcome<Output> {
        Ok(self
            .command()
            .arg("bind-receiver")
            .arg("--identity-key-file")
            .arg(&self.identity)
            .output()?)
    }

    fn no_store(&self) {
        assert!(!self.scratch.0.join("data").exists());
        assert!(self.occupied.local_addr().is_ok());
    }

    fn no_transaction(&self) {
        let state = self
            .endpoint
            .state
            .lock()
            .unwrap_or_else(std::sync::PoisonError::into_inner);
        assert!(state.transactions.is_empty());
        assert!(!state
            .calls
            .iter()
            .any(|(method, _)| method == "eth_sendRawTransaction"));
        self.no_store();
    }

    fn no_secrets(&self, output: &Output) -> Outcome {
        let printed = format!(
            "{}{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        for path in [&self.receiver, &self.identity] {
            let text = std::fs::read_to_string(path)?;
            assert!(!printed.contains(text.trim().trim_start_matches("0x")));
        }
        assert!(!printed.contains(&hex(&self.expected_data[100..])));
        Ok(())
    }
}

fn report(output: &Output) -> Outcome<Value> {
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(output.stderr.is_empty());
    assert_eq!(String::from_utf8_lossy(&output.stdout).lines().count(), 1);
    Ok(serde_json::from_slice(&output.stdout)?)
}

fn store_files(directory: &Path) -> Outcome<BTreeMap<PathBuf, Vec<u8>>> {
    let mut files = BTreeMap::new();
    for entry in std::fs::read_dir(directory)? {
        let path = entry?.path();
        if path.is_dir() {
            files.extend(store_files(&path)?);
        } else {
            files.insert(path.clone(), std::fs::read(path)?);
        }
    }
    Ok(files)
}

#[test]
fn receiver_command_binding_messages_match_precompile_vectors() -> Outcome {
    let vectors: Value = serde_json::from_str(BIND_VECTORS)?;
    let key = SigningKey::from_bytes(&[0x61; 32]);
    let mut checked = 0;
    for vector in vectors["binds"].as_array().ok_or("binding vectors")? {
        if vector["valid"] != true {
            continue;
        }
        let chain = vector["chain_id"].as_str().ok_or("chain")?.parse()?;
        let nonce = vector["nonce"].as_str().ok_or("nonce")?.parse()?;
        let address = fixed(vector["evm_address"].as_str().ok_or("address")?)?;
        let message = binding_message(chain, &address, nonce);
        assert_eq!(hex(&message), vector["message"]);
        assert_eq!(hex(&key.sign(&message).to_bytes()), vector["signature"]);
        let expected = format!(
            "dd9aa628{}{}{}{}",
            vectors["public_key"].as_str().ok_or("public key")?,
            hex(&word(64)),
            hex(&word(64)),
            vector["signature"].as_str().ok_or("signature")?
        );
        assert_eq!(
            hex(&binding_calldata(&key, chain, &address, nonce)),
            expected
        );
        checked += 1;
    }
    assert_eq!(checked, 2);
    Ok(())
}

#[test]
fn receiver_command_status_unbound() -> Outcome {
    let setup = Setup::new()?;
    let value = report(&setup.command().arg("receiver-status").output()?)?;
    let state = setup.endpoint.state.lock().map_err(|_| "state")?;
    assert_eq!(
        value,
        json!({"address":hex0x(&state.address), "bound":false, "identity":null, "ledger_accounts":{}, "payable_assets":[], "withheld_assets":[], "balance":(GAS * GAS_PRICE).to_string()})
    );
    assert!(!state.calls.iter().any(
        |(method, params)| method == "eth_call" && params[0]["to"] == hex0x(&LEDGER_PRECOMPILE)
    ));
    drop(state);
    setup.no_transaction();
    Ok(())
}

#[test]
fn receiver_command_status_bound_with_store_locked() -> Outcome {
    let setup = Setup::new()?;
    {
        let mut state = setup.endpoint.state.lock().map_err(|_| "state")?;
        state.bound = Some(state.identity);
    }
    let index = x_websearch::index::WebIndex::open(&setup.scratch.0.join("data"))?;
    let files_before = store_files(&setup.scratch.0.join("data"))?;
    let value = report(&setup.command().arg("receiver-status").output()?)?;
    let state = setup.endpoint.state.lock().map_err(|_| "state")?;
    assert_eq!(value["address"], hex0x(&state.address));
    assert_eq!(value["bound"], true);
    assert_eq!(
        value["identity"],
        format!("did:layerx:{}", hex(&state.identity))
    );
    assert_eq!(value["balance"], (GAS * GAS_PRICE).to_string());
    let accounts = value["ledger_accounts"]
        .as_object()
        .ok_or("ledger accounts")?;
    assert_eq!(accounts.len(), 4);
    for (symbol, asset) in ["SID", "PAX", "USDC", "USDL"].iter().zip(&state.assets) {
        assert_eq!(accounts[*symbol], hex0x(&keccak(asset)));
    }
    assert!(state.transactions.is_empty());
    assert_eq!(files_before, store_files(&setup.scratch.0.join("data"))?);
    drop(state);
    drop(index);
    Ok(())
}

fn assert_receiver_transaction(raw: &[u8], expected_data: &[u8], address: [u8; 20]) -> Outcome {
    let fields = transaction_fields(raw)?;
    assert_eq!(fields.len(), 9);
    assert_eq!(fields[0], &[u8::try_from(TX_NONCE)?]);
    assert_eq!(fields[1], &[u8::try_from(GAS_PRICE)?]);
    assert_eq!(fields[2], &[0x03, 0x0d, 0x40]);
    assert_eq!(fields[3], ADDR_PRECOMPILE.as_slice());
    assert!(fields[4].is_empty());
    assert_eq!(fields[5], expected_data);
    let v = fields[6]
        .iter()
        .fold(0_u64, |n, byte| n * 256 + u64::from(*byte));
    assert_eq!((v - 35) / 2, 125);
    let mut payload = Vec::new();
    for field in &fields[..6] {
        payload.extend_from_slice(&rlp(field, false)?);
    }
    payload.extend_from_slice(&[125, 128, 128]);
    let digest = keccak(&rlp(&payload, true)?);
    let mut signature = [0; 64];
    signature[32 - fields[7].len()..32].copy_from_slice(fields[7]);
    signature[64 - fields[8].len()..].copy_from_slice(fields[8]);
    let signature = k256::ecdsa::Signature::from_slice(&signature)?;
    let recovery = k256::ecdsa::RecoveryId::try_from(u8::try_from((v - 35) % 2)?)?;
    let public = k256::ecdsa::VerifyingKey::recover_from_prehash(&digest, &signature, recovery)?;
    let recovered = keccak(&public.to_encoded_point(false).as_bytes()[1..]);
    assert_eq!(&recovered[12..], address.as_slice());
    Ok(())
}

#[test]
fn receiver_command_bind_sends_one_receiver_signed_transaction() -> Outcome {
    let setup = Setup::new()?;
    setup
        .endpoint
        .state
        .lock()
        .map_err(|_| "state")?
        .pending_receipts = 1;
    let output = setup.bind()?;
    let value = report(&output)?;
    setup.no_secrets(&output)?;
    let state = setup.endpoint.state.lock().map_err(|_| "state")?;
    assert_eq!(state.transactions.len(), 1);
    assert_receiver_transaction(&state.transactions[0], &setup.expected_data, state.address)?;
    let estimate = state
        .calls
        .iter()
        .find(|(method, _)| method == "eth_estimateGas")
        .ok_or("gas estimate")?;
    assert_eq!(estimate.1[0]["data"], hex0x(&setup.expected_data));
    assert_eq!(
        value,
        json!({"address":hex0x(&state.address), "identity":format!("did:layerx:{}", hex(&state.identity)), "bound":true, "already_bound":false, "transaction_hash":hex0x(&state.hash.ok_or("hash")?)})
    );
    assert!(state
        .calls
        .iter()
        .any(|(method, params)| method == "eth_call"
            && params[0]["data"].as_str().is_some_and(
                |data| data.starts_with(&hex0x(&keccak(b"getLayerXDid(address)")[..4]))
            )));
    assert_eq!(
        state
            .calls
            .iter()
            .filter(|(method, _)| method == "eth_getTransactionReceipt")
            .count(),
        2
    );
    assert!(!String::from_utf8_lossy(&output.stdout).contains(&hex(&state.transactions[0])));
    drop(state);
    setup.no_store();
    Ok(())
}

#[test]
fn receiver_command_already_bound_is_idempotent() -> Outcome {
    let setup = Setup::new()?;
    {
        let mut state = setup.endpoint.state.lock().map_err(|_| "state")?;
        state.bound = Some(state.identity);
    }
    let value = report(&setup.bind()?)?;
    assert_eq!(value["already_bound"], true);
    assert_eq!(value["bound"], true);
    assert!(value["transaction_hash"].is_null());
    let state = setup.endpoint.state.lock().map_err(|_| "state")?;
    assert_eq!(value["address"], hex0x(&state.address));
    assert_eq!(
        value["identity"],
        format!("did:layerx:{}", hex(&state.identity))
    );
    drop(state);
    setup.no_transaction();
    Ok(())
}

#[test]
fn receiver_command_refuses_another_identity() -> Outcome {
    let setup = Setup::new()?;
    setup.endpoint.state.lock().map_err(|_| "state")?.bound = Some([0x21; 32]);
    let output = setup.bind()?;
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("is bound to another identity"));
    setup.no_transaction();
    Ok(())
}

#[test]
fn receiver_command_refuses_insufficient_balance() -> Outcome {
    let setup = Setup::new()?;
    setup.endpoint.state.lock().map_err(|_| "state")?.balance = GAS * GAS_PRICE - 1;
    let output = setup.bind()?;
    assert!(!output.status.success());
    let error = String::from_utf8_lossy(&output.stderr);
    let state = setup.endpoint.state.lock().map_err(|_| "state")?;
    assert!(error.contains(&hex0x(&state.address)));
    assert!(error.contains(&format!("needs {} base units", GAS * GAS_PRICE)));
    drop(state);
    setup.no_transaction();
    Ok(())
}

#[test]
fn receiver_command_refuses_identity_key_files_before_sending() -> Outcome {
    let setup = Setup::new()?;
    for (bytes, mode, text) in [
        (
            b"not hexadecimal".to_vec(),
            0o600,
            "does not hold 32 hexadecimal bytes",
        ),
        (vec![b'1'; 300], 0o600, "too large"),
        (vec![b'1'; 64], 0o644, "readable by group or others"),
    ] {
        std::fs::write(&setup.identity, &bytes)?;
        std::fs::set_permissions(&setup.identity, std::fs::Permissions::from_mode(mode))?;
        let output = setup.bind()?;
        assert!(!output.status.success());
        let error = String::from_utf8_lossy(&output.stderr);
        assert!(error.contains("--identity-key-file"));
        assert!(error.contains(text));
        assert!(!error.contains(&String::from_utf8_lossy(&bytes).into_owned()));
        assert!(output.stdout.is_empty());
        setup.no_transaction();
    }
    std::fs::remove_file(&setup.identity)?;
    let output = setup.bind()?;
    assert!(!output.status.success());
    assert!(String::from_utf8_lossy(&output.stderr).contains("unreadable"));
    setup.no_transaction();
    Ok(())
}

#[test]
fn receiver_command_failed_receipt_reports_transaction_hash() -> Outcome {
    let setup = Setup::new()?;
    setup
        .endpoint
        .state
        .lock()
        .map_err(|_| "state")?
        .fail_receipt = true;
    let output = setup.bind()?;
    setup.no_secrets(&output)?;
    assert!(!output.status.success());
    let state = setup.endpoint.state.lock().map_err(|_| "state")?;
    assert_eq!(state.transactions.len(), 1);
    let error = String::from_utf8_lossy(&output.stderr);
    assert!(error.contains(&hex0x(&state.hash.ok_or("hash")?)));
    assert!(error.contains("binding transaction receipt failed"));
    assert!(output.stdout.is_empty());
    drop(state);
    setup.no_store();
    Ok(())
}

#[test]
fn receiver_command_checks_the_confirmed_identity() -> Outcome {
    let setup = Setup::new()?;
    setup
        .endpoint
        .state
        .lock()
        .map_err(|_| "state")?
        .wrong_confirmation = true;
    let output = setup.bind()?;
    assert!(!output.status.success());
    let state = setup.endpoint.state.lock().map_err(|_| "state")?;
    assert_eq!(state.transactions.len(), 1);
    let error = String::from_utf8_lossy(&output.stderr);
    assert!(error.contains(&hex0x(&state.hash.ok_or("hash")?)));
    assert!(error.contains("different identity"));
    drop(state);
    setup.no_store();
    Ok(())
}

#[test]
fn receiver_command_refuses_malformed_reads_and_wrong_chain() -> Outcome {
    let setup = Setup::new()?;
    for mode in 0..4 {
        {
            let mut state = setup.endpoint.state.lock().map_err(|_| "state")?;
            state.malformed_identity = mode == 0;
            state.malformed_account = mode == 1;
            state.bound = (mode == 1).then_some(state.identity);
            state.chain_id = if mode == 2 { 126 } else { 125 };
            state.malformed_nonce = mode == 3;
        }
        let output = if mode == 3 {
            setup.bind()?
        } else {
            setup.command().arg("receiver-status").output()?
        };
        assert!(!output.status.success());
        assert!(output.stdout.is_empty());
        setup.no_transaction();
    }
    Ok(())
}

#[test]
fn receiver_command_unknown_words_and_missing_options_are_refused() -> Outcome {
    let setup = Setup::new()?;
    for arguments in [
        vec!["unknown"],
        vec!["receiver-status", "extra"],
        vec!["bind-receiver"],
        vec!["bind-receiver", "--identity-key-file"],
        vec!["bind-receiver", "--other", "key"],
        vec!["bind-receiver", "--identity-key-file", ""],
    ] {
        let output = setup.command().args(arguments).output()?;
        assert_eq!(output.status.code(), Some(2));
        assert!(String::from_utf8_lossy(&output.stderr).contains("usage:"));
        setup.no_transaction();
    }
    assert!(setup
        .endpoint
        .state
        .lock()
        .map_err(|_| "state")?
        .calls
        .is_empty());
    Ok(())
}

struct Running(Child);

impl Drop for Running {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

#[test]
fn receiver_command_without_a_command_starts_the_sidecar() -> Outcome {
    let mut setup = Setup::new()?;
    setup.config["evm"]["endpoint"] = json!(setup.endpoint.url());
    setup.save()?;
    let address = setup.occupied.local_addr()?;
    drop(setup.occupied);
    let mut command = Command::new(env!("CARGO_BIN_EXE_x-websearch"));
    command
        .arg("--config")
        .arg(setup.scratch.0.join("config.json"))
        .env(RECEIVER_KEY_FILE, &setup.receiver)
        .env_remove(ATTESTOR_KEY_FILE)
        .env_remove(SUBMITTER_KEY_FILE)
        .stdout(Stdio::null())
        .stderr(Stdio::from(std::fs::File::create(
            setup.scratch.0.join("sidecar.stderr"),
        )?));
    let mut child = Running(command.spawn()?);
    let deadline = Instant::now() + Duration::from_secs(20);
    loop {
        if let Some(status) = child.0.try_wait()? {
            return Err(format!(
                "sidecar exited {status}: {}",
                std::fs::read_to_string(setup.scratch.0.join("sidecar.stderr"))?
            )
            .into());
        }
        if let Ok(mut stream) = TcpStream::connect_timeout(&address, Duration::from_millis(100)) {
            stream.set_read_timeout(Some(Duration::from_secs(2)))?;
            stream.write_all(
                b"GET /health HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n",
            )?;
            let mut response = String::new();
            stream.read_to_string(&mut response)?;
            if response.starts_with("HTTP/1.1 200") {
                let body = response.split_once("\r\n\r\n").ok_or("health body")?.1;
                assert_eq!(serde_json::from_str::<Value>(body)?["status"], "ok");
                assert!(setup.scratch.0.join("data").exists());
                return Ok(());
            }
        }
        if Instant::now() >= deadline {
            return Err("sidecar listener did not start".into());
        }
        thread::sleep(Duration::from_millis(20));
    }
}
