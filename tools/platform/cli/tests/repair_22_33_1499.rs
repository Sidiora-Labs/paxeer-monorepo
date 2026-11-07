//! Task record 22.33.1499, audit 22.30.89.
//! readme_retired_endpoint -> readme_retired_endpoint (README commands through real CLI/HTTP).
//! readme_operator_contract -> readme_operator_contract, readme_receipt_policy_and_watch.
//! Baseline: not run; the ordered gate contract forbids verification before implementation,
//! and this test target did not exist. No baseline failures or native execution are claimed.
//! Wiring: tools/platform/cli/src/{rpc.rs,rpc_subscription.rs,wallet.rs} — domain 8:
//! separate root-node transport from hosted service transport before advertising public CLI onboarding.
//! Wiring: rpc/sequencer_receipt_vectors_test.go and precompiles/layerxledger — domain 2:
//! retain merged-node execution coverage; this gate uses committed Go receipts, real codecs and loopback peers.
//! Evidence: hf-build/hf-done retain the candidate revision, commands, exits and private logs.

use std::collections::BTreeMap;
use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _, PermissionsExt as _};
use std::process::{Command, Output, Stdio};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread;
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_platform_cli::{encoding::hex_decode, wallet_gateway as producer};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use ring::{aead, pbkdf2};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const README: &str = include_str!("../README.md");
const PASSPHRASE: &str = "readme integration temporary credential passphrase";
const KEY_ID: [u8; 32] = [0xa1; 32];

fn hex(bytes: &[u8]) -> String {
    producer::hex(bytes)
}
fn unhex(text: &str) -> Vec<u8> {
    hex_decode("fixture", text.strip_prefix("0x").unwrap_or(text)).unwrap()
}
fn sequencer() -> SigningKey {
    SigningKey::from_bytes(&[0x51; 32])
}

// Only shell tokenization and variable substitution: no shell or toolchain subprocess.
fn example(needle: &str, variables: &BTreeMap<String, String>) -> Vec<String> {
    let normalized = README.replace("\\\n", "");
    let line = normalized
        .lines()
        .find(|line| line.starts_with("layerx ") && line.contains(needle))
        .unwrap_or_else(|| panic!("missing README example {needle}"));
    let mut args = shlex::split(line).expect("README shell syntax");
    assert_eq!(args.remove(0), "layerx");
    for arg in &mut args {
        for (name, value) in variables {
            *arg = arg.replace(&format!("${name}"), value);
        }
        *arg = arg.replace("$(openssl rand -hex 32)", &"44".repeat(32));
        assert!(!arg.contains('$'), "unexpanded README input: {arg}");
    }
    args
}

fn successful(output: Output) -> Value {
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let result: Value = serde_json::from_slice(&output.stdout).unwrap();
    assert_eq!(result["ok"], true);
    result
}

fn refused(output: Output, reason: &str) {
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    assert!(
        String::from_utf8_lossy(&output.stderr).contains(reason),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
}

struct Cli {
    directory: tempfile::TempDir,
    variables: BTreeMap<String, String>,
}
impl Cli {
    fn new() -> Self {
        let directory = tempfile::tempdir().unwrap();
        let variables = BTreeMap::from([
            (
                "EVM_KEY_FILE".into(),
                directory.path().join("keys.json").to_str().unwrap().into(),
            ),
            (
                "MNEMONIC_FILE".into(),
                directory.path().join("mnemonic").to_str().unwrap().into(),
            ),
            (
                "RECEIPT_POLICY".into(),
                directory
                    .path()
                    .join("policy.json")
                    .to_str()
                    .unwrap()
                    .into(),
            ),
            (
                "RECEIPT_ANCHOR_FILE".into(),
                directory.path().join("anchor").to_str().unwrap().into(),
            ),
            ("FEE_LIMIT".into(), "200000".into()),
            ("ASSET_ID".into(), "22".repeat(32)),
            ("RECIPIENT_ACCOUNT".into(), "33".repeat(32)),
            ("WATCH_ADDRESS".into(), format!("0x{}", "12".repeat(20))),
        ]);
        let cli = Self {
            directory,
            variables,
        };
        cli.vault();
        std::fs::write(&cli.variables["MNEMONIC_FILE"], "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about").unwrap();
        std::fs::write(
            &cli.variables["RECEIPT_ANCHOR_FILE"],
            hex(&sequencer().verifying_key().to_bytes()),
        )
        .unwrap();
        successful(cli.documented("wallet derive --mnemonic-file", None));
        let exported: Value =
            serde_json::from_slice(&std::fs::read(&cli.variables["EVM_KEY_FILE"]).unwrap())
                .unwrap();
        successful(cli.run(
            &[
                "wallet",
                "import",
                "payments",
                "--did",
                exported["did"].as_str().unwrap(),
            ],
            exported["layerx_seed"].as_str(),
        ));
        successful(cli.documented("key default payments", None));
        std::fs::write(&cli.variables["RECEIPT_POLICY"], policy()).unwrap();
        cli
    }
    fn vault(&self) {
        // A credential fixture; the real CLI opens and updates it with its production file store.
        let directory = self.directory.path().join("credentials");
        std::fs::DirBuilder::new()
            .mode(0o700)
            .create(&directory)
            .unwrap();
        let mut key = [0; 32];
        let salt = [0x31; 16];
        let nonce = [0x32; 12];
        pbkdf2::derive(
            pbkdf2::PBKDF2_HMAC_SHA256,
            std::num::NonZeroU32::new(600_000).unwrap(),
            &salt,
            PASSPHRASE.as_bytes(),
            &mut key,
        );
        let key = aead::LessSafeKey::new(aead::UnboundKey::new(&aead::AES_256_GCM, &key).unwrap());
        let mut encrypted = serde_json::to_vec(
            &json!({"gateway:beta":format!("beta:lxp_live_{}", "a".repeat(64))}),
        )
        .unwrap();
        key.seal_in_place_append_tag(
            aead::Nonce::assume_unique_for_key(nonce),
            aead::Aad::from(b"LXVAULT1"),
            &mut encrypted,
        )
        .unwrap();
        let mut file = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(directory.join("vault"))
            .unwrap();
        for bytes in [b"LXVAULT1".as_slice(), &salt, &nonce, &encrypted] {
            file.write_all(bytes).unwrap();
        }
    }
    fn command(&self) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx"));
        command
            .env_clear()
            .env("LAYERX_CONFIG", self.directory.path().join("config.json"))
            .env("LAYERX_CREDENTIAL_STORE", "file")
            .env("LAYERX_CREDENTIAL_PASSPHRASE", PASSPHRASE)
            .args(["--json"])
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .stdin(Stdio::null());
        command
    }
    fn run(&self, args: &[&str], input: Option<&str>) -> Output {
        let mut command = self.command();
        command.args(args);
        if input.is_some() {
            command.stdin(Stdio::piped());
        }
        let mut child = command.spawn().unwrap();
        if let Some(input) = input {
            child
                .stdin
                .take()
                .unwrap()
                .write_all(input.as_bytes())
                .unwrap();
        }
        child.wait_with_output().unwrap()
    }
    fn documented(&self, needle: &str, input: Option<&str>) -> Output {
        let args = example(needle, &self.variables);
        self.run(&args.iter().map(String::as_str).collect::<Vec<_>>(), input)
    }
    fn connect(&mut self, server: &Server) {
        self.variables
            .insert("HOSTED_SERVICE_URL".into(), server.base.clone());
        self.variables
            .insert("CLI_RPC_URL".into(), format!("{}/rpc", server.base));
        successful(self.documented("environment use beta", None));
        let key: Value =
            serde_json::from_slice(&std::fs::read(&self.variables["EVM_KEY_FILE"]).unwrap())
                .unwrap();
        self.variables
            .insert("WALLET_DID".into(), key["did"].as_str().unwrap().into());
    }
}
fn policy() -> String {
    json!({"protocol_version":2,"network_id":125,"sequencer_id":hex(&KEY_ID),
        "sequencer_key":hex(&sequencer().verifying_key().to_bytes()),"first_batch":1,"last_batch":100,
        "checkpoint_context_digest":null}).to_string()
}

struct Server {
    base: String,
    stop: Arc<AtomicBool>,
    thread: Option<thread::JoinHandle<()>>,
    requests: Arc<Mutex<Vec<Value>>>,
}
impl Server {
    fn start(mut answer: impl FnMut(&Value, &str) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").unwrap();
        listener.set_nonblocking(true).unwrap();
        let base = format!("http://{}", listener.local_addr().unwrap());
        let stop = Arc::new(AtomicBool::new(false));
        let done = stop.clone();
        let requests = Arc::new(Mutex::new(Vec::new()));
        let seen = requests.clone();
        let thread = thread::spawn(move || {
            while !done.load(Ordering::Acquire) {
                let (mut socket, _) = match listener.accept() {
                    Ok(value) => value,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("{error}"),
                };
                socket
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                socket
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut header = Vec::new();
                while !header.ends_with(b"\r\n\r\n") {
                    assert!(header.len() < 8192);
                    let mut byte = [0];
                    socket.read_exact(&mut byte).unwrap();
                    header.push(byte[0]);
                }
                let header = std::str::from_utf8(&header).unwrap();
                assert!(header.starts_with("POST /rpc HTTP/1.1\r\n"));
                let length = header
                    .lines()
                    .find_map(|line| {
                        let (name, value) = line.split_once(':')?;
                        name.eq_ignore_ascii_case("content-length")
                            .then(|| value.trim().parse::<usize>().unwrap())
                    })
                    .unwrap();
                assert!(length <= 1_048_576);
                let mut bytes = vec![0; length];
                socket.read_exact(&mut bytes).unwrap();
                let request: Value = serde_json::from_slice(&bytes).unwrap();
                seen.lock().unwrap().push(request.clone());
                let result = answer(&request, header);
                let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                write!(socket, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len()).unwrap();
            }
        });
        Self {
            base,
            stop,
            thread: Some(thread),
            requests,
        }
    }
}
impl Drop for Server {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.thread.take().unwrap().join().unwrap();
    }
}

fn word(value: u128) -> Vec<u8> {
    let mut out = vec![0; 16];
    out.extend_from_slice(&value.to_be_bytes());
    out
}
fn source(did: &str, asset: &str, sequence: u64) -> Value {
    let owner = producer::key(did.strip_prefix("did:layerx:").unwrap()).unwrap();
    let asset_id = producer::key(asset).unwrap();
    let mut native = [0; 32];
    native[0] = 1;
    let name = layerx_wire::account::account_name_for_asset(did, asset_id, native).unwrap();
    let account = layerx_wire::hash::account_id_for_protocol(&name, 3).unwrap();
    producer::ledger_account(did, asset, 7, &mut |data| {
        Ok(match &data[..4] {
            [0x3f, 0x05, 0x0c, 0xbc] => account.to_vec(),
            [0x37, 0x95, 0x5a, 0xb2] => owner.to_vec(),
            [0xce, 0x2f, 0x92, 0x1f] => word(u128::from(sequence)),
            [0xb1, 0xff, 0xc0, 0x3e] => word(1000),
            selector => panic!("unexpected fixture ABI {selector:?}"),
        })
    })
    .unwrap()
}

#[test]
fn readme_retired_endpoint() {
    let public = README
        .lines()
        .find_map(|line| line.strip_prefix("export RPC_URL="))
        .unwrap();
    assert_eq!(public, "https://api1.mainnet-beta.paxeer.network/");
    assert!(README.contains("https://api-mainnet-beta.paxeer.network/"));
    assert!(!README.contains("/rpc/ws"));
    assert!(README.contains("Direct public root-node URLs are rejected"));
    assert!(layerx_platform_cli::rpc::RpcClient::new(public, None).is_err());
    let mut cli = Cli::new();
    let server = Server::start(|request, header| {
        assert!(header
            .to_ascii_lowercase()
            .contains("authorization: layerx-key beta:lxp_live_"));
        match request["method"].as_str().unwrap() {
            "px_getBalances" => {
                let did = request["params"][0].as_str().unwrap();
                let owner = producer::key(did.strip_prefix("did:layerx:").unwrap()).unwrap();
                producer::balances(
                    producer::account_document(
                        Some([0x12; 20]),
                        Some("pax1fixture".into()),
                        Some(owner),
                        None,
                    ),
                    7,
                    vec![json!({"denom":format!("layerx/{}", "22".repeat(32)),"amount":"1000"})],
                )
            }
            "px_listAssets" => {
                let (cursor, limit) = producer::page_params(Some(&request["params"])).unwrap();
                assert_eq!(cursor, None);
                assert_eq!(limit, 16);
                producer::assets_page(vec![json!({"asset_id":"22".repeat(32),"kind":"bank","denom":"factory/pay","name":"Payment Token","symbol":"PAY","decimals":6,"supply":"1000"})], 7, cursor, limit, None).unwrap()
            }
            method => panic!("unexpected README read {method}"),
        }
    });
    cli.connect(&server);
    let balance = successful(cli.documented("wallet balance --did", None));
    assert_eq!(balance["data"]["height"], 7);
    assert_eq!(balance["data"]["balances"][0]["amount"], "1000");
    assert_eq!(balance["data"]["evidence"], "committed_eth_call");
    assert!(balance["data"].get("verification").is_none());
    let filtered = successful(cli.documented("--did \"$WALLET_DID\" --asset", None));
    assert_eq!(filtered["data"]["balances"], balance["data"]["balances"]);
    let listed = successful(cli.documented("token list --limit", None));
    assert_eq!(listed["data"]["assets"][0]["symbol"], "PAY");
    let info = successful(cli.documented("token info", None));
    assert_eq!(info["data"]["asset"]["asset_id"], "22".repeat(32));
    refused(
        cli.run(
            &[
                "--rpc",
                public,
                "--gateway-credential",
                "beta",
                "wallet",
                "balance",
                "--did",
                &cli.variables["WALLET_DID"],
            ],
            None,
        ),
        "RPC URL must end in /rpc",
    );
    refused(
        cli.run(&["token", "list", "--limit", "17"], None),
        "invalid value",
    );
    assert_eq!(server.requests.lock().unwrap().len(), 4);
}

// Receipt peer fixture uses the production canonical encoder, hash and Ed25519 signer.
fn error_data(reason: &str) -> Vec<u8> {
    let mut data = vec![0x08, 0xc3, 0x79, 0xa0];
    data.extend_from_slice(&[0; 31]);
    data.push(32);
    data.extend_from_slice(&[0; 24]);
    data.extend_from_slice(&(reason.len() as u64).to_be_bytes());
    data.extend_from_slice(reason.as_bytes());
    data.resize(68 + reason.len().div_ceil(32) * 32, 0);
    data
}

fn transaction_receipt(raw: &[u8], reverted: bool) -> Value {
    let (call, signature) = decode_signed(raw).expect("signed EVM transaction");
    let from_address = recover_sender(&call, &signature).expect("secp256k1 recovery");
    let hash = transaction_hash(raw);
    let return_data = if reverted {
        error_data("LXP_INSUFFICIENT_BALANCE")
    } else {
        Vec::new()
    };
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

#[test]
fn readme_operator_contract() {
    let mut cli = Cli::new();
    assert_eq!(
        std::fs::metadata(&cli.variables["EVM_KEY_FILE"])
            .unwrap()
            .permissions()
            .mode()
            & 0o777,
        0o600
    );
    let export: Value =
        serde_json::from_slice(&std::fs::read(&cli.variables["EVM_KEY_FILE"]).unwrap()).unwrap();
    let did = export["did"].as_str().unwrap().to_owned();
    let evm_address = unhex(export["evm_address"].as_str().unwrap());
    let captured = Arc::new(Mutex::new(Vec::new()));
    let received = captured.clone();
    let mut submitted = BTreeMap::new();
    let mut next_nonce = 7_u64;
    let mut next_sequence = 19_u64;
    let server = Server::start(move |request, header| {
        let method = request["method"].as_str().unwrap();
        if method == "px_requestFunds" {
            assert!(header
                .to_ascii_lowercase()
                .contains("authorization: bearer readme-identity-session"));
            assert_eq!(
                request["params"],
                json!([
                    did,
                    hex(&SigningKey::from_bytes(
                        &unhex(export["layerx_seed"].as_str().unwrap())
                            .try_into()
                            .unwrap()
                    )
                    .verifying_key()
                    .to_bytes())
                ])
            );
            return json!({"funded":true,"funding_id":"ab".repeat(32),"transaction_id":"cd".repeat(32),"amount":"1000","network":"mainnet-beta"});
        }
        assert!(header
            .to_ascii_lowercase()
            .contains("authorization: layerx-key beta:lxp_live_"));
        match method {
            "eth_getTransactionCount" => {
                assert_eq!(request["params"][1], "pending");
                json!(format!("0x{next_nonce:x}"))
            }
            "eth_gasPrice" => json!("0x2"),
            "eth_estimateGas" => {
                assert_eq!(request["params"][0]["chainId"], "0x7d");
                assert_eq!(request["params"][0]["nonce"], format!("0x{next_nonce:x}"));
                assert_eq!(request["params"][1], "pending");
                json!("0x5208")
            }
            "px_getAccount" => {
                assert_eq!(request["params"], json!([did, "22".repeat(32)]));
                source(&did, &"22".repeat(32), next_sequence)
            }
            "eth_sendRawTransaction" => {
                let raw = unhex(request["params"][0].as_str().unwrap());
                let (call, signature) = decode_signed(&raw).unwrap();
                assert_eq!(call.chain_id, 125);
                assert_eq!(call.nonce, next_nonce);
                assert_eq!(call.gas_limit, 21_000);
                assert_eq!(
                    recover_sender(&call, &signature).unwrap().as_slice(),
                    evm_address
                );
                let disclosure = layerx_crypto::disclosure::call::bind_call(
                    &call,
                    recover_sender(&call, &signature).unwrap(),
                )
                .unwrap();
                // The source-account sequence remains independent of the EVM nonce.
                if disclosure.activity_type.ordinal() == 5 {
                    assert_eq!(&call.data[184..192], &next_sequence.to_be_bytes());
                    assert_ne!(call.nonce, next_sequence);
                    next_sequence += 1;
                }
                received
                    .lock()
                    .unwrap()
                    .push(disclosure.activity_type.ordinal());
                let hash = format!("0x{}", hex(&transaction_hash(&raw)));
                submitted.insert(hash.clone(), raw);
                next_nonce += 1;
                json!(hash)
            }
            "pax_sequencerReceipt" => {
                let raw = submitted
                    .get(request["params"][0].as_str().unwrap())
                    .unwrap();
                let mut receipt = transaction_receipt(raw, false);
                receipt["rung"] = json!("committed");
                receipt["blockNumber"] = json!("0x7");
                receipt["blockHash"] = json!(format!("0x{}", "ef".repeat(32)));
                receipt
            }
            "eth_getTransactionReceipt" => {
                assert!(submitted.contains_key(request["params"][0].as_str().unwrap()));
                json!({"transactionHash":request["params"][0],"blockNumber":"0x7","blockHash":format!("0x{}", "ef".repeat(32)),"status":"0x1","transactionIndex":"0x0"})
            }
            method => panic!("unexpected README write method {method}"),
        }
    });
    cli.connect(&server);
    refused(
        cli.documented("faucet --key payments", None),
        "identity_session_required",
    );
    successful(cli.documented(
        "auth set --environment beta",
        Some("readme-identity-session"),
    ));
    let grant = successful(cli.documented("faucet --key payments", None));
    assert_eq!(grant["data"]["claim"]["txHash"], "cd".repeat(32));
    assert_eq!(grant["data"]["claim"]["amount"], "1000");
    let mut hashes = Vec::new();
    for command in [
        " token create ",
        " wallet open-account ",
        " token mint ",
        " token burn ",
        " wallet send ",
        " token transfer ",
    ] {
        let args = example(command, &cli.variables);
        for flag in ["--evm-key-file", "--receipt-policy", "--fee-limit"] {
            assert!(
                args.iter().any(|arg| arg == flag),
                "README {command} lacks {flag}"
            );
        }
        let output =
            successful(cli.run(&args.iter().map(String::as_str).collect::<Vec<_>>(), None));
        assert_eq!(output["data"]["rung"], "committed");
        hashes.push(output["data"]["txHash"].as_str().unwrap().to_owned());
    }
    assert_eq!(captured.lock().unwrap().as_slice(), &[1, 4, 10, 11, 5, 5]);
    cli.variables
        .insert("TX_HASH".into(), hashes.last().unwrap().clone());
    let receipt = successful(cli.documented("wallet receipt", None));
    assert_eq!(receipt["data"]["txHash"], *hashes.last().unwrap());
    assert_eq!(receipt["data"]["rung"], "committed");
    // Required signing input is enforced by the real parser/handler before network I/O.
    let mut args = example(" wallet send ", &cli.variables);
    let position = args.iter().position(|arg| arg == "--evm-key-file").unwrap();
    args.drain(position..=position + 1);
    let before = server.requests.lock().unwrap().len();
    refused(
        cli.run(&args.iter().map(String::as_str).collect::<Vec<_>>(), None),
        "provide --evm-key-file",
    );
    assert_eq!(server.requests.lock().unwrap().len(), before);
}

#[test]
fn readme_receipt_policy_and_watch() {
    let mut cli = Cli::new();
    let corpus: Value = serde_json::from_str(include_str!(
        "../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    ))
    .unwrap();
    assert_eq!(
        corpus["provenance"]["generator"],
        "rpc/sequencer_receipt_vectors_test.go"
    );
    let case = &corpus["cases"][0];
    let mut receipt = case["receiptJson"].clone();
    // Preserve the Go-produced signed receipt byte for byte; only the peer's committed envelope is added.
    receipt["rung"] = json!("committed");
    receipt["blockNumber"] = json!("0x3e8");
    receipt["blockHash"] = json!(format!("0x{}", "ef".repeat(32)));
    let valid = json!({"protocol_version":2,"network_id":125,
        "sequencer_id":case["pinnedKey"]["keyId"].as_str().unwrap().strip_prefix("0x").unwrap(),
        "sequencer_key":case["pinnedKey"]["publicKey"].as_str().unwrap().strip_prefix("0x").unwrap(),
        "first_batch":case["pinnedKey"]["firstHeight"],"last_batch":case["pinnedKey"]["lastHeight"],
        "checkpoint_context_digest":null});
    let tamper = Arc::new(AtomicBool::new(false));
    let changed = tamper.clone();
    let server = Server::start(
        move |request, _| match request["method"].as_str().unwrap() {
            "pax_sequencerReceipt" => {
                assert_eq!(request["params"][0], receipt["txHash"]);
                let mut result = receipt.clone();
                if changed.load(Ordering::Acquire) {
                    result["gasUsed"] = json!("0x1");
                }
                result
            }
            "eth_getTransactionReceipt" => {
                json!({"transactionHash":receipt["txHash"],"blockNumber":"0x3e8","blockHash":receipt["blockHash"],"status":"0x1","transactionIndex":receipt["index"]})
            }
            method => panic!("unexpected receipt method {method}"),
        },
    );
    cli.connect(&server);
    cli.variables.insert(
        "TX_HASH".into(),
        case["receiptJson"]["txHash"].as_str().unwrap().into(),
    );
    std::fs::write(&cli.variables["RECEIPT_POLICY"], valid.to_string()).unwrap();
    let output = successful(cli.documented("wallet receipt", None));
    assert_eq!(output["data"]["receipt"], case["signedReceipt"]);
    assert_eq!(output["data"]["rung"], "committed");
    tamper.store(true, Ordering::Release);
    refused(
        cli.documented("wallet receipt", None),
        "served execution differs from the signed receipt",
    );
    tamper.store(false, Ordering::Release);
    for (field, value) in [
        ("protocol_version", json!(3)),
        ("network_id", json!(402)),
        ("first_batch", json!(0)),
        ("last_batch", json!(999)),
        ("checkpoint_context_digest", json!("11".repeat(32))),
        ("unknown_field", json!(true)),
    ] {
        let mut invalid = valid.clone();
        invalid[field] = value;
        std::fs::write(&cli.variables["RECEIPT_POLICY"], invalid.to_string()).unwrap();
        let before = server.requests.lock().unwrap().len();
        refused(
            cli.documented("wallet receipt", None),
            if field == "unknown_field" {
                "unknown field"
            } else {
                "EVM receipt policy requires"
            },
        );
        assert_eq!(server.requests.lock().unwrap().len(), before);
    }
    std::fs::write(&cli.variables["RECEIPT_POLICY"], valid.to_string()).unwrap();
    let mut finalised = example("wallet receipt", &cli.variables);
    *finalised.last_mut().unwrap() = "finalised".into();
    refused(
        cli.run(
            &finalised.iter().map(String::as_str).collect::<Vec<_>>(),
            None,
        ),
        "MissingFinalityTrust",
    );
    watch(&mut cli);
}

fn watch(cli: &mut Cli) {
    use tungstenite::handshake::server::{Request, Response};
    use tungstenite::Message;
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    listener.set_nonblocking(true).unwrap();
    cli.variables.insert(
        "CLI_RPC_URL".into(),
        format!("http://{}/rpc", listener.local_addr().unwrap()),
    );
    let address = cli.variables["WATCH_ADDRESS"].clone();
    let expected = address.clone();
    let server = thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(15);
        let stream = loop {
            match listener.accept() {
                Ok((stream, _)) => break stream,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(Instant::now() < deadline, "README watch did not connect");
                    thread::sleep(Duration::from_millis(2));
                }
                Err(error) => panic!("{error}"),
            }
        };
        stream
            .set_read_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        stream
            .set_write_timeout(Some(Duration::from_secs(5)))
            .unwrap();
        let mut socket =
            tungstenite::accept_hdr(stream, |request: &Request, response: Response| {
                assert_eq!(request.uri().path(), "/rpc/ws");
                assert_eq!(
                    request.headers()["Authorization"],
                    format!("LayerX-Key beta:lxp_live_{}", "a".repeat(64))
                );
                Ok(response)
            })
            .unwrap();
        let Message::Text(text) = socket.read().unwrap() else {
            panic!("expected subscription JSON")
        };
        let request: Value = serde_json::from_str(&text).unwrap();
        assert_eq!(request["method"], "eth_subscribe");
        assert_eq!(
            request["params"],
            json!(["logs",{"address":address,"topics":[]}])
        );
        socket
            .send(Message::Text(
                json!({"jsonrpc":"2.0","id":request["id"],"result":"readme-subscription"})
                    .to_string()
                    .into(),
            ))
            .unwrap();
        socket.send(Message::Text(json!({"jsonrpc":"2.0","method":"eth_subscription","params":{"subscription":"readme-subscription","result":{"address":address,"topics":[],"data":"0x","transactionHash":format!("0x{}","cd".repeat(32))}}}).to_string().into())).unwrap();
    });
    let result = successful(cli.documented("wallet watch account", None));
    server.join().unwrap();
    assert_eq!(result["data"]["verified"], false);
    assert_eq!(result["data"]["notification"]["address"], expected);
    for topic in ["receipts", "checkpoints"] {
        let args = example("wallet watch account", &cli.variables)
            .into_iter()
            .map(|arg| if arg == "account" { topic.into() } else { arg })
            .collect::<Vec<String>>();
        refused(
            cli.run(&args.iter().map(String::as_str).collect::<Vec<_>>(), None),
            "wallet watch requires account",
        );
    }
}
