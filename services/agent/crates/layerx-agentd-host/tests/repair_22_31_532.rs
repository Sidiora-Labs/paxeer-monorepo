use layerx_agentd_host::payer;

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, Disclosure, IdempotencyRef, PreparationRef, Prepared, SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::approval::{ApprovalOutcome, ApprovalService, DecisionKey, DecisionRequest};
use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    reserve_and_hold_at, ApprovalContext, ApprovalRegistry, ApprovalSequence, ApproverId,
};
use layerx_agentd::prepare::evm::{self, AbiArgument, PrecompileCall, PrecompileTarget};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{Store, TenantId};
use layerx_client::sequencer::{ExecutedReceipt, ReceiptKeyEntry, SequencerLog};
use layerx_crypto::evm_transaction::{
    assemble, decode_signed, decode_unsigned, recover_sender, transaction_hash,
};
use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_human_kms::attestor::{AttestorNodeConfig, AttestorSigner, AttestorSignerConfig};
use layerx_mcp::tools::web::{GrantTerms, WebPayer, WebPayerError};
use layerx_types::ids::Did;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::receipt_digest;
use layerx_wire::limits::{MAX_MESSAGE_BYTES, MAX_SEQUENCER_EFFECTS, PROTOCOL_VERSION};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use sha3::Keccak256;
use std::io::{Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::PermissionsExt as _;
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

type Result<T = ()> = std::result::Result<T, Box<dyn std::error::Error>>;
const KEY_ID: [u8; 32] = [0x33; 32];
const HEIGHT: u64 = 100;
const PAYER: [u8; 32] = [0x41; 32];
const ASSET: [u8; 32] = [0x43; 32];
const CA: &[u8] =
    include_bytes!("../../../../human/crates/layerx-human-kms/src/fixtures/repair_22_33_25/ca.der");
const CERT: &[u8] = include_bytes!(
    "../../../../human/crates/layerx-human-kms/src/fixtures/repair_22_33_25/peer.der"
);
const TLS_KEY: &[u8] = include_bytes!(
    "../../../../human/crates/layerx-human-kms/src/fixtures/repair_22_33_25/peer.pk8"
);

fn checked<T, E: std::fmt::Debug>(result: std::result::Result<T, E>) -> Result<T> {
    result.map_err(|error| format!("{error:?}").into())
}
fn hex(bytes: &[u8]) -> String {
    bytes
        .iter()
        .flat_map(|byte| {
            let digits = b"0123456789abcdef";
            [
                char::from(digits[usize::from(byte >> 4)]),
                char::from(digits[usize::from(byte & 0xf)]),
            ]
        })
        .collect()
}
fn prefixed(bytes: &[u8]) -> String {
    format!("0x{}", hex(bytes))
}
fn unhex(text: &str) -> Vec<u8> {
    text.strip_prefix("0x")
        .unwrap_or(text)
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let Ok(expected_value) = u8::from_str_radix(
                {
                    let Ok(expected_value) = std::str::from_utf8(pair) else {
                        panic!("hex")
                    };
                    expected_value
                },
                16,
            ) else {
                panic!("hex byte")
            };
            expected_value
        })
        .collect()
}
struct Directory(PathBuf);
impl Directory {
    fn new() -> Result<Self> {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-31-532-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&path)?;
        std::fs::set_permissions(&path, std::fs::Permissions::from_mode(0o700))?;
        Ok(Self(path))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}
fn evm_key() -> k256::ecdsa::SigningKey {
    {
        let Ok(expected_value) = k256::ecdsa::SigningKey::from_bytes((&[9; 32]).into()) else {
            panic!("test key")
        };
        expected_value
    }
}
fn sender() -> [u8; 20] {
    let point = evm_key().verifying_key().to_encoded_point(false);
    {
        let Ok(expected_value) = Keccak256::digest(&point.as_bytes()[1..])[12..].try_into() else {
            panic!("address")
        };
        expected_value
    }
}

fn wire<T>(result: std::result::Result<T, layerx_wire::WireError>) -> Result<T> {
    result.map_err(|error| format!("{error:?}").into())
}

fn sha256(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest(bytes).into()
}

#[must_use]
pub fn published_keys() -> Vec<ReceiptKeyEntry> {
    vec![ReceiptKeyEntry {
        key_id: KEY_ID,
        public_key: LocalSigner::new([7; 32]).public_key(),
        first_height: HEIGHT,
        last_height: HEIGHT,
    }]
}

fn sign_digest(seed: [u8; 32], digest: &[u8; 32]) -> Result<[u8; 64]> {
    let mut pkcs8 = vec![
        0x30, 0x2e, 0x02, 0x01, 0x00, 0x30, 0x05, 0x06, 0x03, 0x2b, 0x65, 0x70, 0x04, 0x22, 0x04,
        0x20,
    ];
    pkcs8.extend_from_slice(&seed);
    let key = rustls::crypto::ring::sign::any_supported_type(
        &rustls::pki_types::PrivateKeyDer::Pkcs8(pkcs8.into()),
    )?;
    let signer = key
        .choose_scheme(&[rustls::SignatureScheme::ED25519])
        .ok_or("Ed25519 scheme")?;
    Ok(signer
        .sign(digest)?
        .try_into()
        .map_err(|_| "signature width")?)
}

fn encode_execution(execution: &ExecutedReceipt, signature: Option<[u8; 64]>) -> Result<Vec<u8>> {
    let module = 12;
    let mut summary = Vec::new();
    summary.extend_from_slice(&execution.gas_used.to_be_bytes());
    summary.extend_from_slice(&execution.cumulative_gas_used.to_be_bytes());
    summary.extend_from_slice(&execution.effective_gas_price.to_be_bytes());
    summary.extend_from_slice(&u32::try_from(execution.index)?.to_be_bytes());
    summary.extend_from_slice(&execution.contract_address);
    summary.extend_from_slice(&execution.sponsor);
    summary.extend_from_slice(&sha256(execution.vm_error.as_bytes()));
    summary.extend_from_slice(&u32::try_from(execution.return_data.len())?.to_be_bytes());
    summary.extend_from_slice(&u32::try_from(execution.logs.len())?.to_be_bytes());
    summary.push(u8::try_from(execution.fee_denom.len())?);
    summary.extend_from_slice(execution.fee_denom.as_bytes());
    let mut effects = vec![(1, summary)];
    for log in &execution.logs {
        let mut body = log.address.to_vec();
        body.push(u8::try_from(log.topics.len())?);
        for topic in &log.topics {
            body.extend_from_slice(topic);
        }
        body.extend_from_slice(&sha256(&log.data));
        effects.push((3, body));
    }
    let previous = [0x55; 32];
    let mut sequence = b"PAXEER/v1/sequence\0".to_vec();
    sequence.extend_from_slice(&previous);
    sequence.extend_from_slice(&execution.seq.to_be_bytes());
    sequence.extend_from_slice(&execution.tx_hash);
    let mut height = [0; 32];
    height[24..].copy_from_slice(&execution.height.to_be_bytes());
    let mut from = [0; 32];
    from[12..].copy_from_slice(&execution.from);
    let mut to = [0; 32];
    to[12..].copy_from_slice(&execution.to);
    let mut encoder = Encoder::new(MAX_MESSAGE_BYTES);
    wire(encoder.structure_header_version(0x5201, PROTOCOL_VERSION))?;
    wire(encoder.u16(PROTOCOL_VERSION))?;
    wire(encoder.bytes(&execution.tx_hash, 32))?;
    wire(encoder.u64(execution.seq))?;
    wire(encoder.bytes(&previous, 32))?;
    wire(encoder.bytes(&sha256(&sequence), 32))?;
    wire(encoder.bytes(&sha256(&execution.return_data), 32))?;
    wire(encoder.i32(0))?;
    wire(encoder.sequence_length(effects.len(), MAX_SEQUENCER_EFFECTS))?;
    for (ordinal, (kind, body)) in effects.iter().enumerate() {
        wire(encoder.u16(module))?;
        wire(encoder.u16(u16::try_from(ordinal)?))?;
        wire(encoder.u16(0))?;
        wire(encoder.tag(*kind, 3))?;
        wire(encoder.u8(0))?;
        wire(encoder.bytes(&[0; 32], 32))?;
        wire(encoder.bytes(body, 256))?;
    }
    wire(encoder.u128(1))?;
    wire(encoder.bytes(&height, 32))?;
    wire(encoder.u16(module))?;
    wire(encoder.u32(1))?;
    wire(encoder.u32(0))?;
    wire(encoder.u8(2))?;
    wire(encoder.bytes(&sha256(execution.payment_denom.as_bytes()), 32))?;
    wire(encoder.u128(0))?;
    wire(encoder.bytes(&from, 32))?;
    wire(encoder.u128(0))?;
    wire(encoder.u128(0))?;
    wire(encoder.u64(execution.nonce))?;
    wire(encoder.bytes(&to, 32))?;
    wire(encoder.u128(0))?;
    wire(encoder.u128(0))?;
    wire(encoder.bytes(&[0; 32], 32))?;
    wire(encoder.bytes(&[0; 32], 32))?;
    wire(encoder.bytes(&execution.key_id, 32))?;
    wire(encoder.u64(execution.timestamp))?;
    wire(encoder.u8(u8::from(signature.is_some())))?;
    if let Some(signature) = signature {
        wire(encoder.bytes(&signature, 64))?;
    }
    Ok(encoder.finish())
}

/// Signs an execution receipt for a ledger call.
///
/// # Errors
///
/// Returns an error if calldata conversion, receipt encoding or signing fails.
///
/// # Panics
///
/// Panics if calldata does not contain the selector and the selected call words.
pub fn signed_execution(
    sender: [u8; 20],
    tx_hash: [u8; 32],
    nonce: u64,
    data: &[u8],
    seed: [u8; 32],
) -> Result<ExecutedReceipt> {
    let word = |offset: usize| -> Result<[u8; 32]> { Ok(data[offset..offset + 32].try_into()?) };
    let (topics, log_data) = match &data[..4] {
        [0x78, 0xea, 0x11, 0xdb] => (
            vec![
                transaction_hash(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)"),
                word(164)?,
                word(4)?,
                word(36)?,
            ],
            [word(68)?, word(100)?].concat(),
        ),
        [0x2e, 0x89, 0x49, 0x0f] => (
            vec![
                transaction_hash(
                    b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)",
                ),
                word(4)?,
                PAYER,
                word(36)?,
            ],
            [ASSET, word(68)?, word(100)?, word(196)?].concat(),
        ),
        [0xd0, 0xd5, 0x34, 0x80] => (
            vec![transaction_hash(b"PayerGrantIssued(bytes32)"), word(68)?],
            vec![],
        ),
        _ => return Err("unexpected ledger call".into()),
    };
    let mut fee = [0; 32];
    fee[31] = 1;
    let mut execution = ExecutedReceipt {
        tx_hash,
        nonce,
        seq: 42,
        height: HEIGHT,
        index: 0,
        timestamp: 0x123,
        success: true,
        return_data: vec![],
        logs: vec![SequencerLog {
            address: PrecompileTarget::Ledger.address(),
            topics,
            data: log_data,
            log_index: 0,
        }],
        gas_used: 0x0001_0000,
        cumulative_gas_used: 0x0001_0000,
        effective_gas_price: 1_000_000_000,
        fee_denom: "upax".to_owned(),
        payment_denom: "upax".to_owned(),
        fee_amount: fee,
        sponsor: [0; 20],
        from: sender,
        to: PrecompileTarget::Ledger.address(),
        contract_address: [0; 20],
        vm_error: String::new(),
        key_id: KEY_ID,
        receipt: vec![],
        signing_key: LocalSigner::new(seed).public_key(),
    };
    let unsigned = encode_execution(&execution, None)?;
    let signature = sign_digest(seed, &wire(receipt_digest(&unsigned))?)?;
    execution.receipt = encode_execution(&execution, Some(signature))?;
    Ok(execution)
}

struct Attestors {
    root: Directory,
    nodes: Vec<AttestorNodeConfig>,
    stop: Arc<AtomicBool>,
    threads: Vec<JoinHandle<()>>,
    signed: Arc<AtomicU64>,
}
impl Attestors {
    fn start() -> Result<Self> {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let root = Directory::new()?;
        for (name, bytes) in [("ca.der", CA), ("peer.der", CERT), ("peer.pk8", TLS_KEY)] {
            let path = root.0.join(name);
            std::fs::write(&path, bytes)?;
            std::fs::set_permissions(path, std::fs::Permissions::from_mode(0o600))?;
        }
        let mut roots = rustls::RootCertStore::empty();
        roots.add(rustls::pki_types::CertificateDer::from(CA.to_vec()))?;
        let verifier = rustls::server::WebPkiClientVerifier::builder(Arc::new(roots)).build()?;
        let tls = Arc::new(
            rustls::ServerConfig::builder()
                .with_client_cert_verifier(verifier)
                .with_single_cert(
                    vec![rustls::pki_types::CertificateDer::from(CERT.to_vec())],
                    rustls::pki_types::PrivateKeyDer::try_from(TLS_KEY.to_vec())?,
                )?,
        );
        let stop = Arc::new(AtomicBool::new(false));
        let signed = Arc::new(AtomicU64::new(0));
        let mut nodes = Vec::new();
        let mut threads = Vec::new();
        for id in ["node-1", "node-2", "node-3"] {
            let listener = TcpListener::bind("127.0.0.1:0")?;
            listener.set_nonblocking(true)?;
            nodes.push(AttestorNodeConfig {
                id: id.into(),
                address: listener.local_addr()?,
            });
            let tls = Arc::clone(&tls);
            let stop = Arc::clone(&stop);
            let signed = Arc::clone(&signed);
            threads.push(thread::spawn(move || {
                serve_attestor(id, &listener, &tls, &stop, &signed);
            }));
        }
        Ok(Self {
            root,
            nodes,
            stop,
            threads,
            signed,
        })
    }
    fn signer(&self) -> Result<Arc<AttestorSigner>> {
        let mut signer = checked(AttestorSigner::from_config(&AttestorSignerConfig {
            nodes: self.nodes.clone(),
            trust_anchors: vec![self.root.0.join("ca.der")],
            client_certificate_chain: vec![self.root.0.join("peer.der")],
            client_private_key_pkcs8: self.root.0.join("peer.pk8"),
            key_id: "owner".into(),
            public_key: LocalSigner::new([7; 32]).public_key(),
            evm_key_id: "evm-owner".into(),
            evm_address: sender(),
            signers: self.nodes.iter().map(|node| node.id.clone()).collect(),
            network_id: 125,
            timeout_seconds: 5,
        }))?;
        checked(signer.set_evm_assertion("held-web-owner"))?;
        Ok(Arc::new(signer))
    }
}
impl Drop for Attestors {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        for thread in self.threads.drain(..) {
            let Ok(()) = thread.join() else {
                panic!("attestor thread")
            };
        }
    }
}
fn unsigned_attestor_call(wire: &[u8]) -> layerx_crypto::evm_transaction::Eip1559Call {
    assert_eq!(wire[0], 2);
    let width = usize::from(wire[1].saturating_sub(0xf7));
    assert!((1..=8).contains(&width));
    let payload = &wire[2 + width..];
    assert!(payload.ends_with(&[0x80; 3]));
    let payload = &payload[..payload.len() - 3];
    let length = payload.len().to_be_bytes();
    let start = {
        let Some(expected_value) = length.iter().position(|byte| *byte != 0) else {
            panic!("RLP length")
        };
        expected_value
    };
    let mut unsigned = vec![
        2,
        0xf7 + {
            let Ok(expected_value) = u8::try_from(length.len() - start) else {
                panic!("RLP width")
            };
            expected_value
        },
    ];
    unsigned.extend_from_slice(&length[start..]);
    unsigned.extend_from_slice(payload);
    {
        let Ok(expected_value) = decode_unsigned(&unsigned) else {
            panic!("actual attestor transaction fields")
        };
        expected_value
    }
}
fn http_request(stream: &mut (impl Read + Write)) -> (String, Vec<u8>) {
    let mut header = Vec::new();
    while !header.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        let Ok(()) = stream.read_exact(&mut byte) else {
            panic!("HTTP header")
        };
        header.extend(byte);
        assert!(header.len() < 65_536);
    }
    let header = {
        let Ok(expected_value) = String::from_utf8(header) else {
            panic!("HTTP text")
        };
        expected_value
    };
    let length = {
        let Some(expected_value) = header.lines().find_map(|line| {
            line.to_ascii_lowercase()
                .strip_prefix("content-length:")
                .map(|value| {
                    let Ok(expected_value) = value.trim().parse::<usize>() else {
                        panic!("content length")
                    };
                    expected_value
                })
        }) else {
            panic!("body length")
        };
        expected_value
    };
    assert!(length < 1_048_576);
    let mut body = vec![0; length];
    let Ok(()) = stream.read_exact(&mut body) else {
        panic!("HTTP body")
    };
    (header, body)
}
fn http_response(stream: &mut impl Write, answer: &Value) {
    let answer = answer.to_string();
    let Ok(()) = write!(stream,"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{answer}",answer.len()) else { panic!("HTTP response") };
    let Ok(()) = stream.flush() else {
        panic!("HTTP flush")
    };
}

#[derive(Default)]
struct NodeState {
    raw: Vec<Vec<u8>>,
    methods: Vec<String>,
    wrong_hash: bool,
    changed: bool,
}
struct Node {
    endpoint: String,
    state: Arc<Mutex<NodeState>>,
    stop: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}
impl Node {
    fn start() -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        listener.set_nonblocking(true)?;
        let endpoint = format!("http://{}/", listener.local_addr()?);
        let stop = Arc::new(AtomicBool::new(false));
        let state = Arc::new(Mutex::new(NodeState::default()));
        let thread = {
            let stop = Arc::clone(&stop);
            let state = Arc::clone(&state);
            thread::spawn(move || {
                while !stop.load(Ordering::Acquire) {
                    let mut tcp = match listener.accept() {
                        Ok((tcp, _)) => tcp,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(1));
                            continue;
                        }
                        Err(error) => panic!("node accept: {error}"),
                    };
                    let Ok(()) = tcp.set_read_timeout(Some(Duration::from_secs(5))) else {
                        panic!("node timeout")
                    };
                    let (_, body) = http_request(&mut tcp);
                    let request: Value = {
                        let Ok(expected_value) = serde_json::from_slice(&body) else {
                            panic!("RPC request")
                        };
                        expected_value
                    };
                    let method = {
                        let Some(expected_value) = request["method"].as_str() else {
                            panic!("method")
                        };
                        expected_value
                    };
                    let mut state = {
                        let Ok(expected_value) = state.lock() else {
                            panic!("node state")
                        };
                        expected_value
                    };
                    state.methods.push(method.to_owned());
                    let result = node_result(&request, &mut state);
                    http_response(
                        &mut tcp,
                        &json!({"jsonrpc":"2.0","id":request["id"],"result":result}),
                    );
                }
            })
        };
        Ok(Self {
            endpoint,
            state,
            stop,
            thread: Some(thread),
        })
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(thread) = self.thread.take() {
            let Ok(()) = thread.join() else {
                panic!("node thread")
            };
        }
    }
}
fn draw_call() -> PrecompileCall {
    GrantTerms {
        payer_did: format!(
            "did:layerx:{}",
            hex(&LocalSigner::new([7; 32]).public_key())
        ),
        payer_account: PAYER,
        recipient: [0x42; 32],
        receiver_did_key: [0x63; 32],
        asset: ASSET,
        expires_at_ms: 1_900_000_000_000,
        amount: 654_321,
        purpose_hash: [0x62; 32],
        idempotency_key: [0x52; 32],
    }
    .prepare_draw([0x61; 32], 19)
}
fn payer(signer: Arc<AttestorSigner>, endpoint: &str) -> Result<payer::ProductionWebPayer> {
    checked(payer::ProductionWebPayer::new(
        format!("did:layerx:{}", hex(&signer.public_key())),
        signer,
        endpoint,
        &[],
        published_keys(),
        Box::new(|| Ok(())),
        Box::new(|_| Err(WebPayerError::Refused)),
    ))
}
fn limiter() -> Arc<BudgetLimiter> {
    Arc::new({
        let Ok(expected_value) = checked(BudgetLimiter::new(vec![LimitConfig {
            id: LimitId([1; 16]),
            name: "web".into(),
            scope: LimitScope::Tenant([2; 32]),
            ceiling: 1_000_000,
            consumed: 0,
        }])) else {
            panic!("limiter")
        };
        expected_value
    })
}
fn held_prepared(transaction: &evm::Prepared) -> Prepared {
    let bytes = transaction.call.encode_unsigned();
    let digest = Sha256::digest(&bytes).into();
    Prepared {
        preparation_ref: {
            let Ok(expected_value) = PreparationRef::new("held-web") else {
                panic!("reference")
            };
            expected_value
        },
        unsigned_canonical_bytes: {
            let Ok(expected_value) = CanonicalBytes::new(bytes) else {
                panic!("canonical bytes")
            };
            expected_value
        },
        signing_preimage: {
            let Ok(expected_value) = SigningPreimage::new(transaction.signing_hash.to_vec()) else {
                panic!("preimage")
            };
            expected_value
        },
        disclosure: Disclosure {
            canonical_digest: digest,
            activity_type: ActivityType(8),
            actor: {
                let Ok(expected_value) = AgentDid::new(hex(&transaction.from)) else {
                    panic!("actor")
                };
                expected_value
            },
            authority: {
                let Ok(expected_value) = AuthorityRef::new("x402:held") else {
                    panic!("authority")
                };
                expected_value
            },
            counterparties: ExplicitSet::deny_all(),
            amounts: ExplicitSet::deny_all(),
            asset: {
                let Ok(expected_value) = Asset::new(hex(&ASSET)) else {
                    panic!("asset")
                };
                expected_value
            },
            fee_limit: Amount(100_000_000_000_000),
            expiry: TimestampSeconds(1_900_000_000),
            idempotency_key: {
                let Ok(expected_value) = IdempotencyRef::new(hex(&[0x52; 32])) else {
                    panic!("key")
                };
                expected_value
            },
        },
        expiry: TimestampSeconds(1_900_000_000),
    }
}

#[test]
fn approval_resources_share_the_store_and_recover_a_recorded_decision() -> Result {
    let attestors = Attestors::start()?;
    let signer = attestors.signer()?;
    let node = Node::start()?;
    let root = Directory::new()?;
    let store = Arc::new(Mutex::new(Store::open(&root.0)?));
    let limiter = limiter();
    let mut payer = payer(signer, &node.endpoint)?
        .with_approval_resources(
            Arc::clone(&store),
            Arc::clone(&limiter),
            vec![LimitId([1; 16])],
        )
        .into_web_payer();
    let resources = payer.approval_resources().ok_or("approval resources")?;
    let cloned = payer.approval_resources().ok_or("cloned resources")?;
    assert!(Arc::ptr_eq(&resources.store, &store));
    assert!(Arc::ptr_eq(&resources.limiter, &limiter));
    assert!(Arc::ptr_eq(&resources.expiry, &cloned.expiry));
    assert_eq!(resources.limits, vec![LimitId([1; 16])]);
    let transaction = checked(payer.prepare_transaction(&draw_call()))?;
    let prepared = held_prepared(&transaction);
    let tenant = TenantId::new("web-approval")?;
    let id = [0x52; 32];
    let registry = ApprovalRegistry::with_store(Arc::clone(&resources.store));
    let context = ApprovalContext {
        tenant: tenant.clone(),
        agent: checked(Did::new(hex(&sender()).as_bytes()))?,
        actor_address: sender(),
        session: SessionId([3; 32]),
        capability: CapabilityId([4; 32]),
        policy_version: "v1".into(),
        request_id: id,
    };
    checked(reserve_and_hold_at(
        &registry,
        &resources.limiter,
        context,
        prepared,
        &ReservationRequest {
            id,
            amount: 654_321,
            expiry_sequence: 110,
            current_sequence: 100,
            applicable_limits: resources.limits.clone(),
        },
        ApprovalSequence(110),
        TimestampSeconds(1_800_000_000),
    ))?;
    assert_eq!(resources.limiter.held_reservations(), Ok(1));
    let key = checked(DecisionKey::new("reject-web"))?;
    let decision = checked(
        ApprovalService::new(&registry, &resources.limiter, &resources.expiry).reject(
            DecisionRequest {
                tenant: &tenant,
                approval_id: id,
                idempotency_key: &key,
                approver: checked(ApproverId::new("human:web"))?,
                current_sequence: 101,
                current_time: TimestampSeconds(1_800_000_001),
            },
        ),
    )?;
    assert_eq!(decision.outcome, ApprovalOutcome::Rejected);
    assert_eq!(resources.limiter.held_reservations(), Ok(0));
    let before_restart =
        checked(
            resources
                .expiry
                .recover(&tenant, id, 110, 101, &resources.limiter),
        )?;
    assert_eq!(before_restart.outcome, ApprovalOutcome::Rejected);
    drop(registry);
    drop(cloned);
    drop(resources);
    drop(payer);
    drop(store);
    let reopened = Arc::new(Mutex::new(Store::open(&root.0)?));
    let payer = payer_fn_after_restart(&attestors, &node.endpoint, Arc::clone(&reopened), limiter)?;
    let resources = payer.approval_resources().ok_or("reopened resources")?;
    assert!(Arc::ptr_eq(&resources.store, &reopened));
    let recovered = checked(
        resources
            .expiry
            .recover(&tenant, id, 110, 101, &resources.limiter),
    )?;
    assert_eq!(recovered, before_restart);
    assert!(checked(node.state.lock())?.raw.is_empty());
    assert_eq!(attestors.signed.load(Ordering::Acquire), 0);
    Ok(())
}
fn payer_fn_after_restart(
    attestors: &Attestors,
    endpoint: &str,
    store: Arc<Mutex<Store>>,
    limiter: Arc<BudgetLimiter>,
) -> Result<payer::WebPayerBoundary> {
    Ok(payer(attestors.signer()?, endpoint)?
        .with_approval_resources(store, limiter, vec![LimitId([1; 16])])
        .into_web_payer())
}

#[test]
fn prepare_transaction_binds_the_signer_chain_and_input_call() -> Result {
    let attestors = Attestors::start()?;
    let signer = attestors.signer()?;
    let node = Node::start()?;
    let root = Directory::new()?;
    let store = Arc::new(Mutex::new(Store::open(&root.0)?));
    let mut payer = payer(Arc::clone(&signer), &node.endpoint)?
        .with_approval_resources(store, limiter(), vec![LimitId([1; 16])])
        .into_web_payer();
    let input = draw_call();
    let prepared = checked(payer.prepare_transaction(&input))?;
    assert_eq!(prepared.from, signer.evm_address());
    assert_eq!(prepared.call.chain_id, 125);
    assert_eq!(prepared.call.nonce, 7);
    assert_eq!(prepared.call.to, input.target.address());
    assert_eq!(prepared.call.data, checked(input.encode())?);
    assert_eq!(prepared.call.max_fee_per_gas, 1_000_000_000);
    assert_eq!(prepared.call.max_priority_fee_per_gas, 0);
    assert_eq!(prepared.call.gas_limit, 100_000);
    assert_eq!(prepared.observed_head_height, 100);
    assert_eq!(
        decode_unsigned(&prepared.call.encode_unsigned())?,
        prepared.call
    );
    checked(evm::verify_call_binding(&prepared))?;
    assert_eq!(attestors.signed.load(Ordering::Acquire), 0);
    assert!(checked(node.state.lock())?.raw.is_empty());
    Ok(())
}

#[test]
fn submit_transaction_signs_only_the_prepared_bytes_and_rejects_a_changed_hash() -> Result {
    let attestors = Attestors::start()?;
    let signer = attestors.signer()?;
    let node = Node::start()?;
    let root = Directory::new()?;
    let store = Arc::new(Mutex::new(Store::open(&root.0)?));
    let mut payer = payer(Arc::clone(&signer), &node.endpoint)?
        .with_approval_resources(Arc::clone(&store), limiter(), vec![LimitId([1; 16])])
        .into_web_payer();
    let transaction = checked(payer.prepare_transaction(&draw_call()))?;
    let prepared = held_prepared(&transaction);
    drop(payer);
    let mut payer = payer_fn_after_restart(&attestors, &node.endpoint, store, limiter())?;
    let mut corrupt = prepared.clone();
    corrupt.signing_preimage = checked(SigningPreimage::new(vec![0; 32]))?;
    let requests_before = checked(node.state.lock())?.methods.len();
    assert_eq!(
        payer.submit_transaction(&corrupt),
        Err(WebPayerError::Refused)
    );
    assert_eq!(checked(node.state.lock())?.methods.len(), requests_before);
    assert_eq!(attestors.signed.load(Ordering::Acquire), 0);
    checked(node.state.lock())?.changed = true;
    let payment = checked(payer.submit_transaction(&prepared))?;
    {
        let state = checked(node.state.lock())?;
        assert_eq!(state.raw.len(), 1);
        let raw = &state.raw[0];
        let (decoded, signature) = decode_signed(raw)?;
        assert_eq!(decoded, transaction.call);
        assert_eq!(
            decoded.encode_unsigned(),
            prepared.unsigned_canonical_bytes.as_bytes()
        );
        assert_eq!(
            decoded.signing_hash().as_slice(),
            prepared.signing_preimage.as_bytes()
        );
        assert_eq!(recover_sender(&decoded, &signature)?, signer.evm_address());
        assert_eq!(assemble(&transaction.call, &signature), *raw);
        assert_eq!(payment.tx_hash, transaction_hash(raw));
        assert!(!payment.receipt.is_empty());
        assert_eq!(payment.logs.len(), 1);
        assert_eq!(
            &state.methods[requests_before..],
            ["eth_sendRawTransaction", "pax_sequencerReceipt"]
        );
    }
    assert_eq!(attestors.signed.load(Ordering::Acquire), 3);
    checked(node.state.lock())?.wrong_hash = true;
    assert_eq!(
        payer.submit_transaction(&prepared),
        Err(WebPayerError::Refused)
    );
    let state = checked(node.state.lock())?;
    assert_eq!(state.raw.len(), 2);
    assert_eq!(
        state.methods.last().map(String::as_str),
        Some("eth_sendRawTransaction")
    );
    Ok(())
}

#[test]
fn payer_without_resources_keeps_the_unavailable_trait_defaults() -> Result {
    let attestors = Attestors::start()?;
    let node = Node::start()?;
    let root = Directory::new()?;
    let mut with_resources = payer(attestors.signer()?, &node.endpoint)?
        .with_approval_resources(
            Arc::new(Mutex::new(Store::open(&root.0)?)),
            limiter(),
            vec![LimitId([1; 16])],
        )
        .into_web_payer();
    let transaction = checked(with_resources.prepare_transaction(&draw_call()))?;
    let mut payer = payer(attestors.signer()?, &node.endpoint)?.into_web_payer();
    assert!(payer.approval_resources().is_none());
    let requests_before = checked(node.state.lock())?.methods.len();
    assert_eq!(
        payer.prepare_transaction(&draw_call()),
        Err(WebPayerError::Unavailable)
    );
    assert_eq!(
        payer.submit_transaction(&held_prepared(&transaction)),
        Err(WebPayerError::Unavailable)
    );
    assert_eq!(checked(node.state.lock())?.methods.len(), requests_before);
    assert_eq!(attestors.signed.load(Ordering::Acquire), 0);
    Ok(())
}

#[test]
fn boundary_view_returns_node_bytes_and_refuses_when_owner_check_refuses() -> Result {
    let attestors = Attestors::start()?;
    let signer = attestors.signer()?;
    let node = Node::start()?;
    let allowed = Arc::new(AtomicBool::new(true));
    let owner_allowed = Arc::clone(&allowed);
    let mut payer = checked(payer::ProductionWebPayer::new(
        format!("did:layerx:{}", hex(&signer.public_key())),
        Arc::clone(&signer),
        &node.endpoint,
        &[],
        published_keys(),
        Box::new(move || {
            if owner_allowed.load(Ordering::Acquire) {
                Ok(())
            } else {
                Err(WebPayerError::Refused)
            }
        }),
        Box::new(|_| Err(WebPayerError::Refused)),
    ))?
    .into_web_payer();
    let mut owner = [0; 32];
    owner[12..].copy_from_slice(&signer.evm_address());
    let call = PrecompileCall {
        target: PrecompileTarget::Ledger,
        selector: transaction_hash(b"accountId(bytes32,bytes32)")[..4].try_into()?,
        arguments: vec![AbiArgument::Word(owner), AbiArgument::Word(ASSET)],
    };
    assert_eq!(checked(payer.view(&call))?, PAYER.to_vec());
    assert_eq!(checked(node.state.lock())?.methods, ["eth_call"]);

    allowed.store(false, Ordering::Release);
    assert_eq!(payer.view(&call), Err(WebPayerError::Refused));
    let state = checked(node.state.lock())?;
    assert_eq!(state.methods, ["eth_call"]);
    assert!(state.raw.is_empty());
    assert_eq!(attestors.signed.load(Ordering::Acquire), 0);
    Ok(())
}

fn node_receipt(request: &Value, state: &NodeState) -> Value {
    let raw = {
        let Some(expected_value) = state.raw.last() else {
            panic!("submitted transaction")
        };
        expected_value
    };
    let (call, _) = {
        let Ok(expected_value) = decode_signed(raw) else {
            panic!("signed fields")
        };
        expected_value
    };
    let hash = transaction_hash(raw);
    assert_eq!(request["params"], json!([prefixed(&hash)]));
    let execution = {
        let Ok(expected_value) = signed_execution(sender(), hash, call.nonce, &call.data, [7; 32])
        else {
            panic!("signed receipt")
        };
        expected_value
    };
    json!({"rung":"receipted","txHash":prefixed(&hash),"nonce":format!("{:#x}",call.nonce),
                                "seq":"0x2a","height":"0x64","index":"0x0","timestamp":"0x123","status":"0x1",
                                "returnData":"0x","logs":execution.logs.iter().map(|log|json!({"address":prefixed(&log.address),
                                    "topics":log.topics.iter().map(|topic|prefixed(topic)).collect::<Vec<_>>(),"data":prefixed(&log.data),"logIndex":"0x0"})).collect::<Vec<_>>(),
                                "gasUsed":"0x10000","cumulativeGasUsed":"0x10000","effectiveGasPrice":"0x3b9aca00",
                                "feeDenom":"upax","paymentDenom":"upax","feeAmount":"0x1","sponsor":prefixed(&[0;20]),
                                "from":prefixed(&sender()),"to":prefixed(&call.to),"contractAddress":prefixed(&[0;20]),"vmError":"",
                                "keyId":prefixed(&execution.key_id),"receipt":prefixed(&execution.receipt),"signingKey":prefixed(&execution.signing_key)})
}

fn node_result(request: &Value, state: &mut NodeState) -> Value {
    let method = {
        let Some(expected_value) = request["method"].as_str() else {
            panic!("method")
        };
        expected_value
    };
    match method {
        "eth_call" => {
            let mut owner = [0; 32];
            owner[12..].copy_from_slice(&sender());
            let data = [&[0x3f, 0x05, 0x0c, 0xbc][..], &owner, &ASSET].concat();
            assert_eq!(
                request["params"],
                json!([{
                                    "to": prefixed(&PrecompileTarget::Ledger.address()),
                                    "data": prefixed(&data),
                                }, "pending"])
            );
            json!(prefixed(&PAYER))
        }
        "eth_getBlockByNumber" => {
            assert!(
                request["params"] == json!(["latest", false])
                    || request["params"] == json!(["0x64", true])
            );
            json!({"number":"0x64","hash":prefixed(&[0x11;32]),"parentHash":prefixed(&[0x12;32]),
                                "timestamp":"0x123","transactionsRoot":prefixed(&[0x13;32]),"receiptsRoot":prefixed(&[0x14;32]),
                                "transactions":[],"baseFeePerGas":if state.changed {"0x77359400"}else{"0x1dcd6500"}})
        }
        "eth_chainId" => json!("0x7d"),
        "eth_getTransactionCount" => {
            assert_eq!(request["params"], json!([prefixed(&sender()), "pending"]));
            json!(if state.changed { "0x63" } else { "0x7" })
        }
        "eth_estimateGas" => {
            assert_eq!(request["params"][0]["from"], prefixed(&sender()));
            assert_eq!(
                request["params"][0]["to"],
                prefixed(&PrecompileTarget::Ledger.address())
            );
            assert_eq!(
                request["params"][0]["data"],
                prefixed(&{
                    let Ok(expected_value) = draw_call().encode() else {
                        panic!("call")
                    };
                    expected_value
                })
            );
            assert_eq!(request["params"][1], "pending");
            json!("0x186a0")
        }
        "eth_sendRawTransaction" => {
            let raw = unhex({
                let Some(expected_value) = request["params"][0].as_str() else {
                    panic!("raw transaction")
                };
                expected_value
            });
            let (call, signature) = {
                let Ok(expected_value) = decode_signed(&raw) else {
                    panic!("signed fields")
                };
                expected_value
            };
            assert_eq!(
                {
                    let Ok(expected_value) = recover_sender(&call, &signature) else {
                        panic!("recover sender")
                    };
                    expected_value
                },
                sender()
            );
            assert_eq!(call.data, {
                let Ok(expected_value) = draw_call().encode() else {
                    panic!("call")
                };
                expected_value
            });
            let hash = if state.wrong_hash {
                [0xee; 32]
            } else {
                transaction_hash(&raw)
            };
            state.raw.push(raw);
            json!(prefixed(&hash))
        }
        "pax_sequencerReceipt" => node_receipt(request, state),
        _ => panic!("unexpected RPC method: {method}"),
    }
}

fn serve_attestor(
    id: &str,
    listener: &TcpListener,
    tls: &Arc<rustls::ServerConfig>,
    stop: &AtomicBool,
    signed: &AtomicU64,
) {
    let mut sequence = 0_u64;
    while !stop.load(Ordering::Acquire) {
        let tcp = match listener.accept() {
            Ok((tcp, _)) => tcp,
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                thread::sleep(Duration::from_millis(1));
                continue;
            }
            Err(error) => panic!("attestor accept: {error}"),
        };
        let Ok(()) = tcp.set_read_timeout(Some(Duration::from_secs(5))) else {
            panic!("read timeout")
        };
        let Ok(()) = tcp.set_write_timeout(Some(Duration::from_secs(5))) else {
            panic!("write timeout")
        };
        let mut stream = rustls::StreamOwned::new(
            {
                let Ok(expected_value) = rustls::ServerConnection::new(Arc::clone(tls)) else {
                    panic!("TLS")
                };
                expected_value
            },
            tcp,
        );
        let (header, body) = http_request(&mut stream);
        assert!(header.starts_with("POST /v1/sign HTTP/1.1"));
        assert!(header
            .to_ascii_lowercase()
            .contains("authorization: bearer held-web-owner"));
        let request: Value = {
            let Ok(expected_value) = serde_json::from_slice(&body) else {
                panic!("attestor request")
            };
            expected_value
        };
        assert_eq!(request["kind"], "evm_tx");
        assert_eq!(request["key_id"], "evm-owner");
        assert_eq!(request["signers"], json!(["node-1", "node-2", "node-3"]));
        let wire = unhex({
            let Some(expected_value) = request["transaction"].as_str() else {
                panic!("transaction")
            };
            expected_value
        });
        let call = unsigned_attestor_call(&wire);
        let digest = call.signing_hash();
        let (signature, recovery) = {
            let Ok(expected_value) = evm_key().sign_prehash_recoverable(&digest) else {
                panic!("signature")
            };
            expected_value
        };
        let mut bytes = signature.to_bytes().to_vec();
        bytes.push(recovery.to_byte());
        sequence += 1;
        signed.fetch_add(1, Ordering::AcqRel);
        let answer = json!({"node_id":id,"key_id":"evm-owner","kind":"evm_tx","signed_bytes":hex(&digest),
                        "signature":hex(&bytes),"recovery_id":recovery.to_byte(),"audit_sequence":sequence});
        http_response(&mut stream, &answer);
    }
}
