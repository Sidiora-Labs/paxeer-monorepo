use std::io::{Read as _, Write as _};
use std::net::{TcpListener, TcpStream};
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
use layerx_wire::hash::{account_id_for_protocol, receipt_digest};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const BATCH_NUMBER: u64 = 7;
const ASSET: [u8; 32] = [0x22; 32];
const DESTINATION: [u8; 32] = [0x33; 32];
const KEY_ID: [u8; 32] = [0xa1; 32];

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::new(), |mut output, byte| {
        std::fmt::Write::write_fmt(&mut output, format_args!("{byte:02x}"))
            .unwrap_or_else(|error| panic!("{error:?}"));
        output
    })
}
fn unhex(text: &str) -> Vec<u8> {
    let text = text.strip_prefix("0x").unwrap_or(text);
    assert_eq!(text.len() % 2, 0);
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(
                std::str::from_utf8(pair).unwrap_or_else(|error| panic!("{}: {error:?}", "hex")),
                16,
            )
            .unwrap_or_else(|error| panic!("{}: {error:?}", "hex byte"))
        })
        .collect()
}
fn sequencer() -> SigningKey {
    SigningKey::from_bytes(&[0x51; 32])
}

fn account_result(did: &str) -> Value {
    use layerx_platform_cli::wallet_gateway as producer;
    let owner = producer::key(
        did.strip_prefix("did:layerx:")
            .unwrap_or_else(|| panic!("{}", "DID prefix")),
    )
    .unwrap_or_else(|error| panic!("{}: {error:?}", "owner key"));
    let mut native = [0; 32];
    native[0] = 1;
    let name = layerx_wire::account::account_name_for_asset(did, ASSET, native)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "account name"));
    let id = account_id_for_protocol(&name, 3)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "account id"));
    producer::ledger_account(did, &hex(&ASSET), BATCH_NUMBER, &mut |data| {
        let word = |value: u128| {
            let mut result = vec![0; 16];
            result.extend_from_slice(&value.to_be_bytes());
            result
        };
        match &data[..4] {
            [0x3f, 0x05, 0x0c, 0xbc] => {
                assert_eq!(&data[4..36], &owner);
                assert_eq!(&data[36..], &ASSET);
                Ok(id.to_vec())
            }
            [0x37, 0x95, 0x5a, 0xb2] => {
                assert_eq!(&data[4..], &id);
                Ok(owner.to_vec())
            }
            [0xce, 0x2f, 0x92, 0x1f] => {
                assert_eq!(&data[4..36], &id);
                assert_eq!(&data[36..], &ASSET);
                Ok(word(19))
            }
            [0xb1, 0xff, 0xc0, 0x3e] => {
                assert_eq!(&data[4..36], &id);
                assert_eq!(&data[36..], &ASSET);
                Ok(word(1000))
            }
            selector => panic!("unexpected native account selector: {selector:?}"),
        }
    })
    .unwrap_or_else(|error| panic!("{}: {error:?}", "production gateway account producer"))
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
    let (call, signature) = decode_signed(raw)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "signed EVM transaction"));
    let from_address = recover_sender(&call, &signature)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "secp256k1 recovery"));
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
    body.extend_from_slice(
        &u32::try_from(return_data.len())
            .unwrap_or_else(|error| panic!("{error:?}"))
            .to_be_bytes(),
    );
    body.extend_from_slice(&0_u32.to_be_bytes());
    body.push(4);
    body.extend_from_slice(b"uhpx");
    let mut batch_id = [0; 32];
    batch_id[24..].copy_from_slice(&7_u64.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&from_address);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&call.to);
    let fields = receipt_wire::ReceiptFields {
        hash: &hash,
        return_data: &return_data,
        reverted,
        body: &body,
        batch_id: &batch_id,
        from: &from,
        to: &to,
        nonce: call.nonce,
        key_id: &KEY_ID,
    };
    let unsigned = receipt_wire::encode(&fields, None);
    let signature = sequencer()
        .sign(
            &receipt_digest(&unsigned)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "receipt digest")),
        )
        .to_bytes();
    let canonical = receipt_wire::encode(&fields, Some(signature));
    assert_eq!(
        layerx_wire::receipt::encode(
            &layerx_wire::receipt::decode(&canonical)
                .unwrap_or_else(|error| panic!("{}: {error:?}", "receipt decode"))
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "receipt encode")),
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
        let directory =
            tempfile::tempdir().unwrap_or_else(|error| panic!("{}: {error:?}", "wallet directory"));
        let account = derive_from_seed(&[0x42; 64], 0)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "wallet derivation"));
        let fixture = Self { directory, account };
        let mut child = fixture
            .command()
            .args(["wallet", "import", "test", "--did", &fixture.account.did()])
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::piped())
            .spawn()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "wallet import"));
        child
            .stdin
            .take()
            .unwrap_or_else(|| panic!("{}", "stdin"))
            .write_all(hex(fixture.account.layerx_seed()).as_bytes())
            .unwrap_or_else(|error| panic!("{}: {error:?}", "seed input"));
        let output = child
            .wait_with_output()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "import result"));
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
            .unwrap_or_else(|error| panic!("{}: {error:?}", "key export"));
        write!(
            export,
            "{}",
            json!({
                "index":0, "evm_address":fixture.account.evm_address_text(),
                "evm_private_key":hex(fixture.account.evm_secret().unwrap_or_else(|| panic!("{}", "private key"))),
                "layerx_seed":hex(fixture.account.layerx_seed()), "did":fixture.account.did()
            })
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "export data"));
        std::fs::write(
            fixture.directory.path().join("policy.json"),
            json!({
                "protocol_version":2, "network_id":125, "sequencer_id":hex(&KEY_ID),
                "sequencer_key":hex(&sequencer().verifying_key().to_bytes()),
                "first_batch":1, "last_batch":100,
            })
            .to_string(),
        )
        .unwrap_or_else(|error| panic!("{}: {error:?}", "receipt policy"));
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
        self.run_with_account_change(args, reverted, tampered, |_, _| {})
    }
    fn run_with_account_change(
        &self,
        args: &[String],
        reverted: bool,
        tampered: bool,
        change: impl Fn(&str, &mut Value) + Send + 'static,
    ) -> (Output, Vec<Value>) {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("{}: {error:?}", "RPC listener"));
        let endpoint = format!(
            "http://{}",
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("{}: {error:?}", "RPC address"))
        );
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "nonblocking listener"));
        let done = Arc::new(AtomicBool::new(false));
        let finished = Arc::clone(&done);
        let did = self.account.did().clone();
        let server = thread::spawn(move || {
            serve_wallet_rpc(&listener, &finished, &did, reverted, tampered, change)
        });
        let output = self
            .command()
            .args(["--rpc", &endpoint])
            .args(args)
            .args(["--gateway-rpc", &endpoint])
            .args(["--fee-limit", "200000", "--timeout-seconds", "5"])
            .arg("--evm-key-file")
            .arg(self.directory.path().join("evm.json"))
            .arg("--receipt-policy")
            .arg(self.directory.path().join("policy.json"))
            .output()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "wallet command"));
        done.store(true, Ordering::SeqCst);
        let requests = server
            .join()
            .unwrap_or_else(|error| panic!("{}: {error:?}", "recording RPC server"));
        (output, requests)
    }
}

fn serve_wallet_rpc(
    listener: &TcpListener,
    finished: &AtomicBool,
    did: &str,
    reverted: bool,
    tampered: bool,
    change: impl Fn(&str, &mut Value),
) -> Vec<Value> {
    let account = account_result(did);
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
        let request = read_wallet_request(&mut socket);
        let method = request["method"]
            .as_str()
            .unwrap_or_else(|| panic!("{}", "method"));
        let mut result = match method {
            "eth_getTransactionCount" => json!("0x7"),
            "eth_gasPrice" => json!("0x2"),
            "eth_estimateGas" => json!("0x5208"),
            "px_getAccount" => {
                assert_eq!(request["params"], json!([did, hex(&ASSET)]));
                account.clone()
            }
            "eth_sendRawTransaction" => {
                assert!(submitted.is_none(), "duplicate submission");
                let raw = unhex(
                    request["params"][0]
                        .as_str()
                        .unwrap_or_else(|| panic!("{}", "raw transaction")),
                );
                let hash = format!("0x{}", hex(&transaction_hash(&raw)));
                submitted = Some(raw);
                json!(hash)
            }
            "pax_sequencerReceipt" => {
                let raw = submitted
                    .as_ref()
                    .unwrap_or_else(|| panic!("{}", "submitted transaction"));
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
        change(method, &mut result);
        let body = json!({"jsonrpc":"2.0","id":request["id"],"result":result}).to_string();
        write!(socket,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",body.len(),body).unwrap_or_else(|error| panic!("{}: {error:?}", "HTTP response"));
        requests.push(request);
    }
    requests
}

fn read_wallet_request(socket: &mut TcpStream) -> Value {
    socket
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "read timeout"));
    socket
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap_or_else(|error| panic!("{}: {error:?}", "write timeout"));
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        assert!(header.len() < 8192);
        let mut byte = [0];
        socket
            .read_exact(&mut byte)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "HTTP header"));
        header.push(byte[0]);
    }
    let header =
        std::str::from_utf8(&header).unwrap_or_else(|error| panic!("{}: {error:?}", "HTTP text"));
    let length = header
        .lines()
        .find_map(|line| {
            let (name, value) = line.split_once(':')?;
            name.eq_ignore_ascii_case("content-length").then(|| {
                value
                    .trim()
                    .parse::<usize>()
                    .unwrap_or_else(|error| panic!("{}: {error:?}", "length"))
            })
        })
        .unwrap_or_else(|| panic!("{}", "content length"));
    assert!(length <= 1_048_576);
    let mut body = vec![0; length];
    socket
        .read_exact(&mut body)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "HTTP body"));
    let request: Value =
        serde_json::from_slice(&body).unwrap_or_else(|error| panic!("{}: {error:?}", "RPC JSON"));
    assert!(header.starts_with("POST / HTTP/1.1\r\n"));
    request
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
        json!([
            format!("0x{}", hex(&fixture.account.evm_address())),
            "pending"
        ])
    );
    let sent: Vec<_> = requests
        .iter()
        .filter(|r| r["method"] == "eth_sendRawTransaction")
        .collect();
    assert_eq!(sent.len(), 1);
    let raw = unhex(
        sent[0]["params"][0]
            .as_str()
            .unwrap_or_else(|| panic!("{}", "raw tx")),
    );
    assert_eq!(raw[0], 2);
    let (call, signature) =
        decode_signed(&raw).unwrap_or_else(|error| panic!("{}: {error:?}", "EIP-1559 decode"));
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(
        recover_sender(&call, &signature).unwrap_or_else(|error| panic!("{}: {error:?}", "sender")),
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
    assert_eq!(
        tx["from"],
        format!("0x{}", hex(&fixture.account.evm_address()))
    );
    assert_eq!(tx["chainId"], "0x7d");
    assert_eq!(tx["nonce"], "0x7");
    assert_eq!(tx["gas"], "0x186a0");
    assert_eq!(tx["data"], format!("0x{}", hex(&call.data)));
    assert!(requests.iter().all(|r| !r["method"]
        .as_str()
        .unwrap_or_else(|| panic!("{}", "method"))
        .starts_with("lx_")));
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
    let value: Value = serde_json::from_slice(&output.stdout)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "CLI JSON output"));
    assert_eq!(value["ok"], true);
    assert_eq!(value["data"]["rung"], "receipted");
    let sent = requests
        .iter()
        .find(|r| r["method"] == "eth_sendRawTransaction")
        .unwrap_or_else(|| panic!("{}", "submitted"));
    let hash = transaction_hash(&unhex(
        sent["params"][0]
            .as_str()
            .unwrap_or_else(|| panic!("{}", "raw tx")),
    ));
    assert_eq!(value["data"]["txHash"], format!("0x{}", hex(&hash)));
}

#[test]
fn wallet_send_submits_raw_evm_transaction() {
    let fixture = WalletFixture::new();
    let (output, requests) = fixture.run(&transfer_args("wallet", "send"), false, false);
    assert_receipted(&output, &requests);
    let call = assert_submission(&fixture, &requests);
    let disclosure = bind_call(&call, fixture.account.evm_address())
        .unwrap_or_else(|error| panic!("{}: {error:?}", "send disclosure"));
    assert_eq!(disclosure.asset, ASSET);
    assert_eq!(&call.data[184..192], &19_u64.to_be_bytes());
    assert_eq!(disclosure.activity_type.ordinal(), 5);
    assert_eq!(disclosure.amounts[0].value, 25);
    let source = account_result(&fixture.account.did());
    assert_eq!(source["asset_id"], hex(&disclosure.asset));
    assert_eq!(
        requests
            .iter()
            .filter(|request| request["method"] == "px_getBalances")
            .map(|request| request["params"].clone())
            .collect::<Vec<_>>(),
        Vec::<Value>::new()
    );
    assert_eq!(
        requests
            .iter()
            .filter(|request| request["method"] == "px_getAccount")
            .map(|request| request["params"].clone())
            .collect::<Vec<_>>(),
        vec![json!([fixture.account.did(), hex(&ASSET)])]
    );
    assert_eq!(
        disclosure.counterparties[0].account,
        unhex(
            source["account_id"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "source"))
        )
        .as_slice()
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
            .unwrap_or_else(|error| panic!("{}: {error:?}", "open account disclosure"))
            .payment,
        Some(layerx_crypto::payments::Payment::OpenAccount { asset: ASSET })
    );
}

#[test]
fn wallet_send_refuses_mismatched_account_reads_before_submission() {
    let fixture = WalletFixture::new();
    for (method, field, value, error) in [
        (
            "px_getAccount",
            "account_id",
            json!(hex(&DESTINATION)),
            "InvalidResponse",
        ),
        (
            "px_getAccount",
            "asset_id",
            json!(hex(&[0x44; 32])),
            "InvalidResponse",
        ),
        (
            "px_getAccount",
            "owner",
            json!(hex(&DESTINATION)),
            "InvalidResponse",
        ),
        (
            "px_getAccount",
            "next_sequence",
            json!("019"),
            "InvalidResponse",
        ),
    ] {
        let (output, requests) = fixture.run_with_account_change(
            &transfer_args("wallet", "send"),
            false,
            false,
            move |received_method, result| {
                if received_method == method {
                    result[field] = value.clone();
                }
            },
        );
        assert!(!output.status.success());
        assert!(output.stdout.is_empty());
        let stderr = String::from_utf8_lossy(&output.stderr);
        let refusal: Value = serde_json::from_slice(&output.stderr)
            .unwrap_or_else(|error| panic!("{}: {error:?}", "public refusal envelope"));
        assert_eq!(refusal["ok"], false);
        assert_eq!(refusal["error"]["detail"], error, "{stderr}");
        assert!(requests.iter().any(|request| request["method"] == method));
        assert!(requests
            .iter()
            .all(|request| request["method"] != "eth_sendRawTransaction"));
    }
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
        let disclosure = bind_call(&call, fixture.account.evm_address())
            .unwrap_or_else(|error| panic!("{}: {error:?}", "token disclosure"));
        assert_eq!(disclosure.activity_type.ordinal(), ordinal);
        assert!(!requests.is_empty());
        assert!(
            requests.iter().all(|r| !r["method"]
                .as_str()
                .unwrap_or_else(|| panic!("{}", "method"))
                .starts_with("lx_")),
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
        .unwrap_or_else(|| panic!("{}", "submitted"));
    assert!(stderr.contains(&hex(&transaction_hash(&unhex(
        sent["params"][0]
            .as_str()
            .unwrap_or_else(|| panic!("{}", "raw tx"))
    )))));
    let (output, _) = fixture.run(&transfer_args("wallet", "send"), true, true);
    assert!(!output.status.success());
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(stderr.contains("receipt_verification_failed"), "{stderr}");
    assert!(!stderr.contains("LXP_FORGED"), "{stderr}");
    assert!(!stderr.contains("transaction_reverted"), "{stderr}");
}

#[path = "common/receipt_wire.rs"]
mod receipt_wire;
