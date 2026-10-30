use std::io::{Read as _, Write as _};
use std::net::TcpListener;
use std::os::unix::fs::OpenOptionsExt as _;
use std::process::{Command, Output, Stdio};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread;
use std::time::{Duration, Instant};

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::account_derivation::{derive_from_seed, DerivedAccount};
use layerx_crypto::disclosure::call::bind_call;
use layerx_crypto::evm_transaction::{
    decode_signed, recover_sender, transaction_hash, Eip1559Call,
};
use layerx_proof::merkle::{build_proof, Proof};
use layerx_proof::state::{decode_account_value, NestedAccountProof};
use layerx_wire::encode::Encoder;
use layerx_wire::hash::{account_id_for_protocol, batch_header_digest, receipt_digest};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const ACCOUNT_PROTOCOL_VERSION: u16 = 3;
const NETWORK_ID: u32 = 402;
const BATCH_NUMBER: u64 = 7;
const RECEIPT_SEQUENCE: u64 = 10;
const ASSET: [u8; 32] = [0x22; 32];
const DESTINATION: [u8; 32] = [0x33; 32];
const KEY_ID: [u8; 32] = [0xa1; 32];

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

fn append_length(bytes: &mut Vec<u8>, value: &[u8]) {
    bytes.extend_from_slice(
        &u32::try_from(value.len())
            .unwrap_or_else(|_| panic!("wire length"))
            .to_be_bytes(),
    );
    bytes.extend_from_slice(value);
}

fn append_proof(bytes: &mut Vec<u8>, proof: &Proof) {
    bytes.extend_from_slice(&proof.leaf_index().to_be_bytes());
    bytes.extend_from_slice(&proof.leaf_count().to_be_bytes());
    bytes.push(u8::try_from(proof.siblings().len()).unwrap_or_else(|_| panic!("proof depth")));
    for sibling in proof.siblings() {
        bytes.extend_from_slice(sibling);
    }
}

/// Encodes exactly the account evidence a node exports with a latest-root read.
fn exported_evidence(proof: &NestedAccountProof, sequencer_id: [u8; 32]) -> Vec<u8> {
    let mut bytes = 1_u16.to_be_bytes().to_vec();
    bytes.extend_from_slice(&[2, 1]);
    for root in [
        proof.account_id,
        proof.account_root,
        proof.universal_root,
        proof.resulting_state_root,
    ] {
        bytes.extend_from_slice(&root);
    }
    for path in [
        &proof.account_proof,
        &proof.account_tree_proof,
        &proof.universal_root_proof,
    ] {
        append_proof(&mut bytes, path);
    }
    append_length(&mut bytes, &proof.receipt_bytes);
    append_proof(&mut bytes, &proof.receipt_proof);
    bytes.extend_from_slice(&1_u16.to_be_bytes());
    bytes.extend_from_slice(&sequencer_id);
    bytes.extend_from_slice(&sequencer_id);
    bytes.extend_from_slice(&BATCH_NUMBER.to_be_bytes());
    bytes.extend_from_slice(&BATCH_NUMBER.to_be_bytes());
    append_length(&mut bytes, &proof.header_bytes);
    bytes.extend_from_slice(&proof.header_signature);
    bytes.push(0);
    bytes
}

fn account_result(did: &str) -> Value {
    let name = format!("agent:{did}:asset:{}", hex(&ASSET));
    let parsed = layerx_types::account::AccountId::parse(&name).expect("account name");
    let id = account_id_for_protocol(&parsed, 3).expect("account id");
    let mut canonical = u16::try_from(name.len())
        .expect("name length")
        .to_be_bytes()
        .to_vec();
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
    let account = decode_account_value(id, &canonical).expect("canonical source account");
    assert_eq!(account.next_sequence, 19);
    let proof = nested_fixture(id, &canonical, &sequencer());
    let result = json!({
        "account_id":hex(&id), "name":name, "asset_id":hex(&ASSET),
        "balance":"1000", "next_sequence":"19", "frozen":false,
        "canonical_value":hex(&canonical),
        "proof_material":hex(&exported_evidence(&proof, sequencer().verifying_key().to_bytes())),
        "observed_head_sequence":"10", "batch_number":BATCH_NUMBER.to_string(),
        "verification":"state_proven"
    });
    layerx_sdk::rpc_verification::VerifiedRpcAccount::from_rpc_result(
        &result,
        id,
        &layerx_sdk::rpc_verification::AccountPolicy {
            protocol_version: 3,
            network_id: NETWORK_ID,
            sequencer_key: sequencer().verifying_key().to_bytes(),
        },
    )
    .expect("real account proof");
    result
}

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

struct WalletFixture {
    directory: tempfile::TempDir,
    account: DerivedAccount,
}
impl WalletFixture {
    fn new() -> Self {
        let directory = tempfile::tempdir().expect("wallet directory");
        let account = derive_from_seed(&[0x42; 64], 0).expect("wallet derivation");
        let fixture = Self { directory, account };
        let mut child = fixture
            .command()
            .args(["wallet", "import", "test", "--did", &fixture.account.did()])
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .expect("wallet import");
        child
            .stdin
            .take()
            .expect("stdin")
            .write_all(hex(fixture.account.layerx_seed()).as_bytes())
            .expect("seed input");
        let output = child.wait_with_output().expect("import result");
        assert!(
            output.status.success(),
            "{}",
            String::from_utf8_lossy(&output.stderr)
        );
        let mut export = std::fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(fixture.directory.path().join("evm.json"))
            .expect("key export");
        write!(
            export,
            "{}",
            json!({
                "index":0, "evm_address":fixture.account.evm_address_text(),
                "evm_private_key":hex(fixture.account.evm_secret().expect("private key")),
                "layerx_seed":hex(fixture.account.layerx_seed()), "did":fixture.account.did()
            })
        )
        .expect("export data");
        std::fs::write(
            fixture.directory.path().join("policy.json"),
            json!({
                "protocol_version":2, "network_id":125, "sequencer_id":hex(&KEY_ID),
                "sequencer_key":hex(&sequencer().verifying_key().to_bytes()),
                "first_batch":1, "last_batch":100,
            })
            .to_string(),
        )
        .expect("receipt policy");
        fixture
    }
    fn command(&self) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx"));
        command
            .env("LAYERX_CONFIG", self.directory.path().join("config.json"))
            .env("LAYERX_CREDENTIAL_STORE", "file")
            .env(
                "LAYERX_CREDENTIAL_PASSPHRASE",
                "wallet evm writes test passphrase",
            )
            .args(["--json"]);
        command
    }
    fn run(&self, args: &[String], reverted: bool, tampered: bool) -> (Output, Vec<Value>) {
        let listener = TcpListener::bind("127.0.0.1:0").expect("RPC listener");
        let endpoint = format!("http://{}/rpc", listener.local_addr().expect("RPC address"));
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let done = Arc::new(AtomicBool::new(false));
        let finished = Arc::clone(&done);
        let did = self.account.did().to_owned();
        let server = thread::spawn(move || {
            let account = account_result(&did);
            let mut requests = Vec::new();
            let mut submitted = None;
            let mut polls = 0;
            let deadline = Instant::now() + Duration::from_secs(15);
            loop {
                let mut socket = match listener.accept() {
                    Ok((socket, _)) => socket,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        if finished.load(Ordering::SeqCst) {
                            break;
                        }
                        assert!(Instant::now() < deadline, "CLI RPC deadline exceeded");
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("RPC accept: {error}"),
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
                    socket.read_exact(&mut byte).expect("HTTP header");
                    header.push(byte[0]);
                }
                let header = std::str::from_utf8(&header).expect("HTTP text");
                assert!(header.starts_with("POST /rpc HTTP/1.1\r\n"));
                let length = header
                    .lines()
                    .find_map(|line| {
                        let (name, value) = line.split_once(':')?;
                        name.eq_ignore_ascii_case("content-length")
                            .then(|| value.trim().parse::<usize>().expect("length"))
                    })
                    .expect("content length");
                assert!(length <= 1_048_576);
                let mut body = vec![0; length];
                socket.read_exact(&mut body).expect("HTTP body");
                let request: Value = serde_json::from_slice(&body).expect("RPC JSON");
                let result = match request["method"].as_str().expect("method") {
                    "eth_getTransactionCount" => json!("0x7"),
                    "eth_gasPrice" => json!("0x2"),
                    "eth_estimateGas" => json!("0x5208"),
                    "px_getBalances" => {
                        assert_eq!(request["params"], json!([did]));
                        json!({"did":did,"verification":"state_proven","accounts":[account]})
                    }
                    "px_getAccount" => {
                        assert_eq!(request["params"], json!([account["account_id"]]));
                        account.clone()
                    }
                    "eth_sendRawTransaction" => {
                        assert!(submitted.is_none(), "duplicate submission");
                        let raw = unhex(request["params"][0].as_str().expect("raw transaction"));
                        let hash = format!("0x{}", hex(&transaction_hash(&raw)));
                        submitted = Some(raw);
                        json!(hash)
                    }
                    "pax_sequencerReceipt" => {
                        let raw = submitted.as_ref().expect("submitted transaction");
                        let hash = format!("0x{}", hex(&transaction_hash(raw)));
                        assert_eq!(request["params"], json!([hash]));
                        polls += 1;
                        if polls == 1 {
                            json!({"txHash":hash,"nonce":"0x7","rung":"submitted"})
                        } else {
                            let mut receipt = transaction_receipt(raw, reverted);
                            if tampered {
                                receipt["returnData"] =
                                    json!(format!("0x{}", hex(&error_data("LXP_FORGED"))));
                            }
                            receipt
                        }
                    }
                    method => panic!("unexpected RPC method {method}"),
                };
                let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
                write!(socket,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",body.len(),body).expect("HTTP response");
                requests.push(request);
            }
            requests
        });
        let output = self
            .command()
            .args(["--rpc", &endpoint])
            .args(args)
            .args(["--fee-limit", "200000", "--timeout-seconds", "5"])
            .arg("--evm-key-file")
            .arg(self.directory.path().join("evm.json"))
            .arg("--receipt-policy")
            .arg(self.directory.path().join("policy.json"))
            .output()
            .expect("wallet command");
        done.store(true, Ordering::SeqCst);
        let requests = server.join().expect("recording RPC server");
        (output, requests)
    }
}

fn transfer_args(group: &str, command: &str) -> Vec<String> {
    vec![
        group.into(),
        command.into(),
        "--asset".into(),
        hex(&ASSET),
        "--to".into(),
        hex(&DESTINATION),
        "--amount".into(),
        "25".into(),
    ]
}
fn assert_submission(fixture: &WalletFixture, requests: &[Value]) -> Eip1559Call {
    let nonce: Vec<_> = requests
        .iter()
        .filter(|r| r["method"] == "eth_getTransactionCount")
        .collect();
    assert_eq!(nonce.len(), 1);
    assert_eq!(
        nonce[0]["params"],
        json!([fixture.account.evm_address_text(), "pending"])
    );
    let sent: Vec<_> = requests
        .iter()
        .filter(|r| r["method"] == "eth_sendRawTransaction")
        .collect();
    assert_eq!(sent.len(), 1);
    let raw = unhex(sent[0]["params"][0].as_str().expect("raw tx"));
    assert_eq!(raw[0], 2);
    let (call, signature) = decode_signed(&raw).expect("EIP-1559 decode");
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(
        recover_sender(&call, &signature).expect("sender"),
        fixture.account.evm_address()
    );
    assert_eq!(
        call.to,
        unhex("000000000000000000000000000000000000101a").as_slice()
    );
    assert_eq!(call.gas_limit, 21_000);
    assert_eq!(call.max_fee_per_gas, 2);
    assert_eq!(call.max_priority_fee_per_gas, 0);
    let estimates: Vec<_> = requests
        .iter()
        .filter(|r| r["method"] == "eth_estimateGas")
        .collect();
    assert_eq!(estimates.len(), 1);
    assert_eq!(estimates[0]["params"][1], "pending");
    let tx = &estimates[0]["params"][0];
    assert_eq!(tx["from"], fixture.account.evm_address_text());
    assert_eq!(tx["chainId"], "0x7d");
    assert_eq!(tx["nonce"], "0x7");
    assert_eq!(tx["gas"], "0x186a0");
    assert_eq!(tx["data"], format!("0x{}", hex(&call.data)));
    assert!(requests
        .iter()
        .all(|r| !r["method"].as_str().expect("method").starts_with("lx_")));
    assert_eq!(
        requests
            .iter()
            .filter(|r| r["method"] == "pax_sequencerReceipt")
            .count(),
        2
    );
    call
}
fn assert_receipted(output: &Output, requests: &[Value]) {
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let value: Value = serde_json::from_slice(&output.stdout).expect("CLI JSON output");
    assert_eq!(value["ok"], true);
    assert_eq!(value["data"]["rung"], "receipted");
    let sent = requests
        .iter()
        .find(|r| r["method"] == "eth_sendRawTransaction")
        .expect("submitted");
    let hash = transaction_hash(&unhex(sent["params"][0].as_str().expect("raw tx")));
    assert_eq!(value["data"]["txHash"], format!("0x{}", hex(&hash)));
}

#[test]
fn wallet_send_submits_raw_evm_transaction() {
    let fixture = WalletFixture::new();
    let (output, requests) = fixture.run(&transfer_args("wallet", "send"), false, false);
    assert_receipted(&output, &requests);
    let call = assert_submission(&fixture, &requests);
    let disclosure = bind_call(&call, fixture.account.evm_address()).expect("send disclosure");
    assert_eq!(disclosure.asset, ASSET);
    assert_eq!(&call.data[184..192], &19_u64.to_be_bytes());
    assert_eq!(disclosure.activity_type.ordinal(), 5);
    assert_eq!(disclosure.amounts[0].value, 25);
    let source = account_result(&fixture.account.did());
    assert_eq!(
        disclosure.counterparties[0].account,
        unhex(source["account_id"].as_str().expect("source")).as_slice()
    );
    assert_eq!(disclosure.counterparties[1].account, DESTINATION);
    let args = vec![
        "wallet".into(),
        "open-account".into(),
        "--asset".into(),
        hex(&ASSET),
    ];
    let (output, requests) = fixture.run(&args, false, false);
    assert_receipted(&output, &requests);
    let call = assert_submission(&fixture, &requests);
    assert_eq!(
        bind_call(&call, fixture.account.evm_address())
            .expect("open account disclosure")
            .payment,
        Some(layerx_crypto::payments::Payment::OpenAccount { asset: ASSET })
    );
}

#[test]
fn token_writes_never_call_lx_methods() {
    let fixture = WalletFixture::new();
    let commands = [
        vec![
            "token".into(),
            "create".into(),
            "--symbol".into(),
            "TST".into(),
            "--name".into(),
            "Test Token".into(),
            "--decimals".into(),
            "6".into(),
            "--salt".into(),
            hex(&[0x44; 32]),
        ],
        vec![
            "token".into(),
            "mint".into(),
            "--asset".into(),
            hex(&ASSET),
            "--to".into(),
            hex(&DESTINATION),
            "--amount".into(),
            "25".into(),
        ],
        vec![
            "token".into(),
            "burn".into(),
            "--asset".into(),
            hex(&ASSET),
            "--amount".into(),
            "25".into(),
        ],
        transfer_args("token", "transfer"),
    ];
    for (args, ordinal) in commands.into_iter().zip([1, 10, 11, 5]) {
        let (output, requests) = fixture.run(&args, false, false);
        assert_receipted(&output, &requests);
        let call = assert_submission(&fixture, &requests);
        let disclosure = bind_call(&call, fixture.account.evm_address()).expect("token disclosure");
        assert_eq!(disclosure.activity_type.ordinal(), ordinal);
        assert!(!requests.is_empty());
        assert!(
            requests
                .iter()
                .all(|r| !r["method"].as_str().expect("method").starts_with("lx_")),
            "{requests:?}"
        );
    }
}

#[test]
fn wallet_send_reports_receipted_revert() {
    let fixture = WalletFixture::new();
    let (output, requests) = fixture.run(&transfer_args("wallet", "send"), true, false);
    assert!(!output.status.success());
    assert!(output.stdout.is_empty());
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(stderr.contains("transaction_reverted"), "{stderr}");
    assert!(stderr.contains("LXP_INSUFFICIENT_BALANCE"), "{stderr}");
    assert!(stderr.contains("receipted"), "{stderr}");
    assert_submission(&fixture, &requests);
    let sent = requests
        .iter()
        .find(|r| r["method"] == "eth_sendRawTransaction")
        .expect("submitted");
    assert!(stderr.contains(&hex(&transaction_hash(&unhex(
        sent["params"][0].as_str().expect("raw tx")
    )))));
    let (output, _) = fixture.run(&transfer_args("wallet", "send"), true, true);
    assert!(!output.status.success());
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(stderr.contains("receipt_verification_failed"), "{stderr}");
    assert!(!stderr.contains("LXP_FORGED"), "{stderr}");
    assert!(!stderr.contains("transaction_reverted"), "{stderr}");
}
