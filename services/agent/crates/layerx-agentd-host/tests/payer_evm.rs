#[path = "../../../../human/crates/layerx-human-kms/tests/support/attestor_cluster.rs"]
mod attestor_cluster;
#[path = "../src/payer.rs"]
mod payer;

use attestor_cluster::{checked, Cluster, Result, SIGNERS};
use layerx_agentd::prepare::evm::PrecompileCall;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_human_kms::attestor::{AttestorNodeConfig, AttestorSigner, AttestorSignerConfig};
use layerx_mcp::tools::web::{ExactTerms, GrantTerms, WebPayerError};
use serde_json::{json, Value};
use sha3::{Digest, Keccak256};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt as _;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{Arc, Mutex, OnceLock};
use std::thread::{self, JoinHandle};
use std::time::Duration;

const LEDGER_ADDRESS: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];
const PAY_ADDRESS: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1b,
];

const RECEIPT: &[u8] = include_bytes!("../../../../../tests/fixtures/receipt-supply-v2.bin");

struct SigningFixture {
    _cluster: Cluster,
    signer: Arc<AttestorSigner>,
}

impl SigningFixture {
    fn start() -> Result<Self> {
        let mut cluster = Cluster::start_with_policies(
            &[],
            &json!({
                "version": 1,
                "defaults": {
                    "chain_id": 125,
                    "kinds": ["lx_activity", "evm_tx"],
                    "caps": {"native": {"per_transaction": "1000000000000000000", "daily": "10000000000000000000"}},
                    "rate_per_minute": 1000,
                    "selectors": {
                        "0x000000000000000000000000000000000000101a": ["0x78ea11db", "0x2e89490f"],
                        "0x000000000000000000000000000000000000101b": ["0xa863162b"]
                    }
                }
            }),
            &json!({"version": 1, "defaults": {"modules": {}}}),
        )?;
        let owner = "web-payer-owner";
        let public_key = cluster.generate_key(
            "web-owner",
            "ed25519",
            owner,
            "0x1111111111111111111111111111111111111111",
        )?;
        let evm_public = cluster.generate_key("web-evm", "secp256k1", owner, "")?;
        let point =
            k256::ecdsa::VerifyingKey::from_sec1_bytes(&evm_public)?.to_encoded_point(false);
        let evm_address = Keccak256::digest(&point.as_bytes()[1..])[12..].try_into()?;
        cluster.pki.certificate_der("node-ca")?;
        cluster.pki.certificate_der("gateway")?;
        cluster.pki.key_der("gateway")?;
        std::fs::set_permissions(
            cluster.pki.path("gateway.pk8"),
            std::fs::Permissions::from_mode(0o600),
        )?;
        let mut signer = checked(AttestorSigner::from_config(&AttestorSignerConfig {
            nodes: cluster
                .nodes
                .iter()
                .map(|(id, address)| AttestorNodeConfig {
                    id: id.clone(),
                    address: *address,
                })
                .collect(),
            trust_anchors: vec![cluster.pki.path("node-ca.der")],
            client_certificate_chain: vec![cluster.pki.path("gateway.der")],
            client_private_key_pkcs8: cluster.pki.path("gateway.pk8"),
            key_id: "web-owner".to_owned(),
            public_key: public_key.try_into().map_err(|_| "owner key width")?,
            evm_key_id: "web-evm".to_owned(),
            evm_address,
            signers: SIGNERS.iter().map(|id| (*id).to_owned()).collect(),
            network_id: 125,
            timeout_seconds: 60,
        }))?;
        checked(signer.set_evm_assertion(&cluster.tokens.mint(owner)?))?;
        Ok(Self {
            _cluster: cluster,
            signer: Arc::new(signer),
        })
    }
}

fn with_signer(run: impl FnOnce(Arc<AttestorSigner>) -> Result<()>) -> Result<()> {
    static FIXTURE: OnceLock<Mutex<Option<SigningFixture>>> = OnceLock::new();
    let mut fixture = FIXTURE
        .get_or_init(|| Mutex::new(None))
        .lock()
        .map_err(|_| "signer lock poisoned")?;
    if fixture.is_none() {
        *fixture = Some(SigningFixture::start()?);
    }
    let fixture = fixture.as_ref().ok_or("missing signer")?;
    run(Arc::clone(&fixture.signer)).map_err(|error| {
        format!(
            "payer integration failed: {error}; attestors: {}",
            fixture._cluster.logs()
        )
        .into()
    })
}

fn hex(bytes: &[u8]) -> String {
    format!("0x{}", attestor_cluster::hex(bytes))
}
fn unhex(value: &Value) -> Vec<u8> {
    let text = value
        .as_str()
        .unwrap_or_else(|| panic!("hex string: {value}"));
    let text = text
        .strip_prefix("0x")
        .unwrap_or_else(|| panic!("hex prefix: {text}"));
    (0..text.len())
        .step_by(2)
        .map(|i| {
            u8::from_str_radix(&text[i..i + 2], 16).unwrap_or_else(|error| panic!("hex: {error}"))
        })
        .collect()
}

struct RpcServer {
    endpoint: String,
    stop: Arc<AtomicBool>,
    raw: Arc<Mutex<Vec<Vec<u8>>>>,
    requests: Arc<Mutex<Vec<String>>>,
    thread: Option<JoinHandle<()>>,
}

impl RpcServer {
    fn start(sender: [u8; 20]) -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let endpoint = format!("http://{}/", listener.local_addr()?);
        let stop = Arc::new(AtomicBool::new(false));
        let raw = Arc::new(Mutex::new(Vec::<Vec<u8>>::new()));
        let requests = Arc::new(Mutex::new(Vec::new()));
        let thread = {
            let stop = Arc::clone(&stop);
            let raw = Arc::clone(&raw);
            let requests = Arc::clone(&requests);
            thread::spawn(move || {
                let mut polls = 0;
                let mut estimate = Vec::new();
                let mut destination = [0; 20];
                while !stop.load(Ordering::Acquire) {
                    let (stream, _) = match listener.accept() {
                        Ok(value) => value,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(2));
                            continue;
                        }
                        Err(error) => panic!("RPC accept: {error}"),
                    };
                    stream
                        .set_read_timeout(Some(Duration::from_secs(5)))
                        .unwrap_or_else(|error| panic!("RPC timeout: {error}"));
                    let mut reader = BufReader::new(stream);
                    let mut length = None;
                    loop {
                        let mut line = String::new();
                        assert!(
                            reader
                                .read_line(&mut line)
                                .unwrap_or_else(|error| panic!("HTTP header: {error}"))
                                > 0
                        );
                        if line == "\r\n" {
                            break;
                        }
                        if let Some(value) =
                            line.to_ascii_lowercase().strip_prefix("content-length:")
                        {
                            length = Some(
                                value
                                    .trim()
                                    .parse::<usize>()
                                    .unwrap_or_else(|error| panic!("HTTP length: {error}")),
                            );
                        }
                    }
                    let mut body =
                        vec![0; length.unwrap_or_else(|| panic!("HTTP length required"))];
                    reader
                        .read_exact(&mut body)
                        .unwrap_or_else(|error| panic!("HTTP body: {error}"));
                    let request: Value = serde_json::from_slice(&body)
                        .unwrap_or_else(|error| panic!("RPC request: {error}"));
                    let method = request["method"]
                        .as_str()
                        .unwrap_or_else(|| panic!("RPC method"));
                    requests
                        .lock()
                        .unwrap_or_else(|_| panic!("requests lock"))
                        .push(method.to_owned());
                    let result = match method {
                        "eth_chainId" => json!("0x7d"),
                        "eth_getTransactionCount" => {
                            assert_eq!(request["params"], json!([hex(&sender), "pending"]));
                            json!(format!(
                                "{:#x}",
                                raw.lock().unwrap_or_else(|_| panic!("raw lock")).len() + 7
                            ))
                        }
                        "eth_estimateGas" => {
                            assert_eq!(request["params"][0]["from"], hex(&sender));
                            estimate = unhex(&request["params"][0]["data"]);
                            destination = match &estimate[..4] {
                                [0x78, 0xea, 0x11, 0xdb] | [0x2e, 0x89, 0x49, 0x0f] => {
                                    LEDGER_ADDRESS
                                }
                                [0xa8, 0x63, 0x16, 0x2b] => PAY_ADDRESS,
                                selector => panic!("unexpected payment selector {selector:?}"),
                            };
                            assert_eq!(request["params"][0]["to"], hex(&destination));
                            assert_eq!(request["params"][1], "pending");
                            json!("0x186a0")
                        }
                        "eth_gasPrice" => json!("0x3b9aca00"),
                        "eth_sendRawTransaction" => {
                            let transaction = unhex(&request["params"][0]);
                            let (call, signature) = decode_signed(&transaction)
                                .unwrap_or_else(|error| panic!("signed transaction: {error}"));
                            assert_eq!(
                                recover_sender(&call, &signature)
                                    .unwrap_or_else(|error| panic!("signature: {error}")),
                                sender
                            );
                            assert_eq!(call.data, estimate);
                            assert_eq!(call.to, destination);
                            let hash = transaction_hash(&transaction);
                            raw.lock()
                                .unwrap_or_else(|_| panic!("raw lock"))
                                .push(transaction);
                            polls = 0;
                            json!(hex(&hash))
                        }
                        "pax_sequencerReceipt" => {
                            let raw = raw.lock().unwrap_or_else(|_| panic!("raw lock"));
                            let transaction = raw
                                .last()
                                .unwrap_or_else(|| panic!("submission before receipt"));
                            let hash = transaction_hash(transaction);
                            assert_eq!(request["params"], json!([hex(&hash)]));
                            polls += 1;
                            let nonce = raw.len() + 6;
                            if polls == 1 {
                                json!({"rung":"submitted", "txHash":hex(&hash), "nonce":format!("{nonce:#x}")})
                            } else {
                                json!({
                                    "rung":"receipted", "txHash":hex(&hash), "nonce":format!("{nonce:#x}"),
                                    "seq":"0x2a", "height":"0x64", "index":"0x0", "timestamp":"0x123", "status":"0x1",
                                    "returnData":"0x", "logs":[{"address":hex(&destination), "topics":[hex(&[3;32])], "data":"0x010203", "logIndex":"0x0"}],
                                    "gasUsed":"0x10000", "cumulativeGasUsed":"0x10000", "effectiveGasPrice":"0x3b9aca00",
                                    "feeDenom":"upax", "feeAmount":"0x1", "sponsor":hex(&[0;20]), "from":hex(&sender),
                                    "to":hex(&destination), "contractAddress":hex(&[0;20]), "vmError":"",
                                    "keyId":hex(&[1;32]), "receipt":hex(RECEIPT), "signingKey":hex(&[2;32]),
                                })
                            }
                        }
                        _ => panic!("unexpected method {method}"),
                    };
                    let answer =
                        json!({"jsonrpc":"2.0", "id":request["id"], "result":result}).to_string();
                    write!(reader.get_mut(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}", answer.len()).unwrap_or_else(|error| panic!("RPC response: {error}"));
                }
            })
        };
        Ok(Self {
            endpoint,
            stop,
            raw,
            requests,
            thread: Some(thread),
        })
    }

    fn submitted(&self) -> Vec<Vec<u8>> {
        self.raw
            .lock()
            .unwrap_or_else(|_| panic!("raw lock"))
            .clone()
    }
    fn finish(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take() {
            thread
                .join()
                .unwrap_or_else(|error| std::panic::resume_unwind(error));
        }
    }
}
impl Drop for RpcServer {
    fn drop(&mut self) {
        self.finish();
    }
}

fn payer(signer: Arc<AttestorSigner>, endpoint: &str) -> Result<payer::ProductionWebPayer> {
    let did = format!("did:layerx:{}", attestor_cluster::hex(&signer.public_key()));
    checked(payer::ProductionWebPayer::new(
        did,
        signer,
        endpoint,
        &[],
        Box::new(|| Ok(())),
        Box::new(|_| Err(WebPayerError::Refused)),
    ))
}

fn assert_call(
    raw: &[u8],
    signer: &AttestorSigner,
    prepared: &PrecompileCall,
    nonce: u64,
    destination: [u8; 20],
) -> Result<Vec<u8>> {
    assert_eq!(raw[0], 2);
    let (call, signature) = decode_signed(raw)?;
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.to, prepared.target.address());
    assert_eq!(call.to, destination);
    assert_eq!(call.nonce, nonce);
    assert_eq!(call.max_priority_fee_per_gas, 0);
    assert_eq!(call.max_fee_per_gas, 1_000_000_000);
    assert_eq!(call.gas_limit, 100_000);
    assert_eq!(recover_sender(&call, &signature)?, signer.evm_address());
    assert_eq!(call.data, checked(prepared.encode())?);
    Ok(call.data)
}

#[test]
fn web_pay_exact_submits_a_signed_layerxpay_call() -> Result<()> {
    with_signer(|signer| {
        let mut rpc = RpcServer::start(signer.evm_address())?;
        let mut payer = payer(Arc::clone(&signer), &rpc.endpoint)?;
        let terms = ExactTerms {
            payer_did: format!("did:layerx:{}", attestor_cluster::hex(&signer.public_key())),
            payer_account: signer.evm_address(),
            recipient: [0x31; 20],
            recipient_account: hex(&[0x31; 20]),
            asset: [0x41; 20],
            current_height: 100,
            amount: 123_456,
            idempotency_key: [0x51; 32],
        };
        let mut wrong = terms.clone();
        wrong.payer_account[0] ^= 1;
        assert_eq!(
            payer.pay(&wrong),
            Err(payer::PayerError::Boundary(WebPayerError::Refused))
        );
        assert!(rpc.submitted().is_empty());
        let payment = checked(payer.pay(&terms))?;
        rpc.finish();
        let submitted = rpc.submitted();
        assert_eq!(submitted.len(), 1);
        let data = assert_call(
            &submitted[0],
            &signer,
            &terms.prepare_pay_exact(),
            7,
            LEDGER_ADDRESS,
        )?;
        assert_eq!(data.len(), 4 + 6 * 32);
        assert_eq!(
            &data[..4],
            &Keccak256::digest(b"payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)")[..4]
        );
        assert_eq!(&data[16..36], &terms.payer_account);
        assert_eq!(&data[48..68], &terms.recipient);
        assert_eq!(&data[80..100], &terms.asset);
        assert_eq!(&data[116..132], &terms.amount.to_be_bytes());
        assert_eq!(&data[156..164], &terms.current_height.to_be_bytes());
        assert_eq!(&data[164..196], &terms.idempotency_key);
        assert_eq!(payment.tx_hash, transaction_hash(&submitted[0]));
        assert_eq!(payment.receipt, RECEIPT);
        assert_eq!(
            payment.logs,
            vec![layerx_proof::sequencer_logs::SequencerLog::new(
                LEDGER_ADDRESS,
                vec![[3; 32]],
                vec![1, 2, 3],
                0
            )]
        );
        let methods = rpc.requests.lock().map_err(|_| "requests lock")?;
        assert_eq!(
            *methods,
            [
                "eth_chainId",
                "eth_getTransactionCount",
                "eth_estimateGas",
                "eth_gasPrice",
                "eth_sendRawTransaction",
                "pax_sequencerReceipt",
                "pax_sequencerReceipt"
            ]
        );
        Ok(())
    })
}

#[test]
fn web_grant_submits_a_signed_layerxpay_call() -> Result<()> {
    with_signer(|signer| {
        let mut rpc = RpcServer::start(signer.evm_address())?;
        let mut payer = payer(Arc::clone(&signer), &rpc.endpoint)?;
        let terms = GrantTerms {
            payer_did: format!("did:layerx:{}", attestor_cluster::hex(&signer.public_key())),
            payer_account: signer.evm_address(),
            recipient: [0x32; 20],
            asset: [0x42; 20],
            current_height: 100,
            amount: 654_321,
            purpose_hash: [0x62; 32],
            idempotency_key: [0x52; 32],
        };
        let raw = checked(payer.grant(&terms))?;
        let draw = checked(payer.draw(&terms, terms.idempotency_key))?;
        let drawn = checked(payer.submit(&draw))?;
        rpc.finish();
        let submitted = rpc.submitted();
        assert_eq!(submitted.len(), 2);
        assert_eq!(raw, submitted[0]);
        let data = assert_call(&raw, &signer, &terms.prepare_budget(), 7, PAY_ADDRESS)?;
        assert_eq!(data.len(), 4 + 12 * 32);
        assert_eq!(
            &data[..4],
            &Keccak256::digest(b"budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)")[..4]
        );
        assert_eq!(&data[4..36], &terms.idempotency_key);
        assert_eq!(&data[48..68], &terms.payer_account);
        assert_eq!(&data[80..100], &terms.asset);
        assert_eq!(&data[100..132], &terms.purpose_hash);
        assert_eq!(&data[148..164], &terms.amount.to_be_bytes());
        assert_eq!(&data[284..292], &terms.current_height.to_be_bytes());
        assert_eq!(&data[316..324], &(terms.current_height + 1).to_be_bytes());
        let draw_data = assert_call(
            &submitted[1],
            &signer,
            &terms.prepare_draw(terms.idempotency_key),
            8,
            LEDGER_ADDRESS,
        )?;
        assert_eq!(draw_data.len(), 4 + 7 * 32);
        assert_eq!(
            &draw_data[..4],
            &Keccak256::digest(b"draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)")
                [..4]
        );
        assert_eq!(&draw_data[48..68], &terms.recipient);
        assert_eq!(&draw_data[84..100], &terms.amount.to_be_bytes());
        assert_eq!(&draw_data[100..132], &terms.idempotency_key);
        assert_eq!(&draw_data[156..164], &terms.current_height.to_be_bytes());
        assert_eq!(&draw_data[164..196], &terms.purpose_hash);
        assert_eq!(drawn.tx_hash, transaction_hash(&submitted[1]));
        assert_eq!(drawn.receipt, RECEIPT);
        Ok(())
    })
}
