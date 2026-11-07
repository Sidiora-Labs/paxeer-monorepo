use ed25519_dalek::Signer as _;
use layerx_agentd::budget::{LimitConfig, LimitId, LimitScope};
use layerx_agentd::human::{
    serve_one, AcceptedTransport, HumanNativeSendOwnerPreviewRequestV1,
    HumanNativeSendPreviewRequestV1, HumanPrepare, NATIVE_SEND_PREVIEW_V1,
};
use layerx_agentd::human::{HumanOperations, HumanPeer, HumanSubject};
use layerx_agentd::human_runtime::{
    HumanAuthorityBoundary, ProductionHumanOperations, RemoteHumanAuthority, SharedAgentOwner,
    UnifiedAgentOwner,
};
use layerx_agentd::identity::{
    self, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::session::{self, OpenRequest, SessionId, SessionRegistry};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::store::{Store, TenantId};
use layerx_client::paxeer_binding::{Binding, SignedBinding};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_types::account::AccountId;
use layerx_types::clock::ClockReading;
use layerx_types::clock_protocol;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId};
use native_tls::{Identity, TlsAcceptor};
use openssl::asn1::Asn1Time;
use openssl::bn::{BigNum, MsbOption};
use openssl::hash::MessageDigest;
use openssl::pkey::{PKey, Private};
use openssl::rsa::Rsa;
use openssl::x509::extension::{
    BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName,
};
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::os::unix::fs::{MetadataExt as _, PermissionsExt as _};
use std::os::unix::net::{UnixListener, UnixStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

pub(crate) static FIXTURE_LOCK: Mutex<()> = Mutex::new(());
const HEAD: u64 = 64;

#[track_caller]
fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("bootstrap peer: {error:?}"),
    }
}

fn random<const N: usize>() -> [u8; N] {
    let mut value = [0; N];
    must(getrandom::fill(&mut value));
    value
}

fn directory(label: &str) -> PathBuf {
    let path =
        std::env::temp_dir().join(format!("bootstrap-{label}-{}", hex::encode(random::<16>())));
    must(fs::create_dir(&path));
    must(fs::set_permissions(
        &path,
        fs::Permissions::from_mode(0o700),
    ));
    path
}

fn text<'a>(value: &'a Value, field: &str) -> &'a str {
    value[field]
        .as_str()
        .unwrap_or_else(|| panic!("missing {field}: {value}"))
}

struct Peer {
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Peer {
    fn spawn(work: impl FnOnce(Arc<AtomicBool>) + Send + 'static) -> Self {
        let stop = Arc::new(AtomicBool::new(false));
        let signal = Arc::clone(&stop);
        Self {
            stop,
            worker: Some(thread::spawn(move || work(signal))),
        }
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            if let Err(failure) = worker.join() {
                let message = failure
                    .downcast_ref::<String>()
                    .map(String::as_str)
                    .or_else(|| failure.downcast_ref::<&str>().copied())
                    .unwrap_or("non-string panic payload");
                eprintln!("bootstrap peer failed: {message}");
                if !thread::panicking() {
                    std::panic::resume_unwind(failure);
                }
            }
        }
    }
}

fn incoming(listener: &TcpListener) -> Option<TcpStream> {
    match listener.accept() {
        Ok((stream, _)) => {
            must(stream.set_read_timeout(Some(Duration::from_secs(3))));
            must(stream.set_write_timeout(Some(Duration::from_secs(3))));
            Some(stream)
        }
        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
            thread::sleep(Duration::from_millis(2));
            None
        }
        Err(error) => panic!("peer accept: {error}"),
    }
}

fn http_request(stream: &mut impl Read) -> (String, Vec<u8>) {
    let mut headers = Vec::new();
    while !headers.ends_with(b"\r\n\r\n") {
        assert!(headers.len() < 16_384);
        let mut byte = [0];
        must(stream.read_exact(&mut byte));
        headers.push(byte[0]);
    }
    let headers = must(String::from_utf8(headers));
    let length = headers
        .lines()
        .filter_map(|line| line.split_once(':'))
        .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
        .map_or(0, |(_, value)| must(value.trim().parse::<usize>()));
    assert!(length <= 1_048_576);
    let mut body = vec![0; length];
    must(stream.read_exact(&mut body));
    (headers, body)
}

fn http_response(stream: &mut impl Write, body: &Value) {
    let body = must(serde_json::to_vec(body));
    must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()));
    must(stream.write_all(&body));
    must(stream.flush());
}

struct Node {
    child: Child,
    url: String,
}
impl Node {
    fn start() -> Self {
        let executable = std::env::var_os("LAYERX_TEST_ANVIL").unwrap_or_else(|| "anvil".into());
        let mut child = must(
            Command::new(executable)
                .args([
                    "--host",
                    "127.0.0.1",
                    "--port",
                    "0",
                    "--chain-id",
                    "125",
                    "--no-mining",
                    "--accounts",
                    "0",
                ])
                .stdout(Stdio::piped())
                .stderr(Stdio::inherit())
                .spawn(),
        );
        let stdout = child.stdout.take().expect("anvil stdout");
        let (sender, receiver) = std::sync::mpsc::sync_channel(1);
        thread::spawn(move || {
            for line in BufReader::new(stdout).lines() {
                let line = must(line);
                if let Some(address) = line.strip_prefix("Listening on ") {
                    let _ = sender.try_send(format!("http://{address}/"));
                }
            }
        });
        let mut node = Self {
            child,
            url: String::new(),
        };
        node.url = must(receiver.recv_timeout(Duration::from_secs(10)));
        node
    }
    fn rpc(&self, method: &str, params: Value) -> Value {
        let value = rpc(
            &self.url,
            &json!({"jsonrpc":"2.0", "id":1, "method":method, "params":params}),
        );
        assert!(value.get("error").is_none(), "{value}");
        value["result"].clone()
    }
}
impl Drop for Node {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

fn rpc(url: &str, request: &Value) -> Value {
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .timeout_global(Some(Duration::from_secs(3)))
        .build()
        .into();
    must(
        must(agent.post(url).send_json(request))
            .body_mut()
            .read_json(),
    )
}

fn chain_peer(node: &Node, binding: SignedBinding, writes: Arc<AtomicUsize>) -> (String, Peer) {
    fn abi_string(value: &[u8]) -> Vec<u8> {
        let mut result = vec![0; 64 + value.len().div_ceil(32) * 32];
        result[31] = 32;
        result[56..64].copy_from_slice(&(value.len() as u64).to_be_bytes());
        result[64..64 + value.len()].copy_from_slice(value);
        result
    }

    must(binding.verify());
    let key_id: [u8; 32] = Sha256::digest(binding.public_key()).into();
    assert_ne!(key_id, [0; 32]);
    let keys = json!({"keys": [{
        "keyId": format!("0x{}", hex::encode(key_id)),
        "publicKey": format!("0x{}", hex::encode(binding.public_key())),
        "firstHeight": 1,
        "lastHeight": HEAD + 1000
    }]});
    let decoded = must(layerx_client::sequencer::decode_receipt_keys(&keys));
    assert_eq!(decoded.len(), 1);
    assert_eq!(decoded[0].key_id, key_id);
    assert_eq!(decoded[0].public_key, binding.public_key());
    assert!((decoded[0].first_height..=decoded[0].last_height).contains(&HEAD));
    let checkpoint_reads: Vec<Value> = must(serde_json::from_str(include_str!(
        "support/human_budget_checkpoint.json"
    )));
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let upstream = node.url.clone();
    let peer = Peer::spawn(move |stop| {
        while !stop.load(Ordering::Acquire) {
            let Some(mut stream) = incoming(&listener) else {
                continue;
            };
            let (_, body) = http_request(&mut stream);
            let request: Value = must(serde_json::from_slice(&body));
            let method = text(&request, "method");
            let result = if method == "eth_sendRawTransaction" || method == "eth_sendTransaction" {
                writes.fetch_add(1, Ordering::SeqCst);
                panic!("prepare attempted to broadcast: {method}");
            } else if method == "px_getReceiptKeys" {
                assert_eq!(request["params"], json!([]));
                json!({"jsonrpc":"2.0", "id":request["id"], "result":keys})
            } else if method == "px_getCommit" || method == "px_getValidators" {
                assert_eq!(request["params"], json!([HEAD.to_string()]));
                let recorded = checkpoint_reads
                    .iter()
                    .find(|read| read["method"] == method && read["params"] == request["params"])
                    .expect("checkpoint RPC recorded at the fixture head");
                json!({"jsonrpc":"2.0", "id":request["id"], "result":recorded["result"]})
            } else if method == "eth_call"
                && request["params"][0]["to"]
                    == format!(
                        "0x{}",
                        hex::encode(layerx_client::account_binding::ADDR_PRECOMPILE)
                    )
            {
                assert_eq!(request["params"][1], "pending");
                let call = &request["params"][0];
                let data = call
                    .get("data")
                    .or_else(|| call.get("input"))
                    .and_then(Value::as_str)
                    .expect("ABI call data");
                let data = must(hex::decode(data.trim_start_matches("0x")));
                let mut word = [0; 32];
                match data[..4].try_into().expect("selector") {
                    layerx_client::account_binding::SELECTOR_LAYERX_BIND_NONCE => {
                        assert_eq!(
                            data,
                            layerx_client::account_binding::bind_nonce_call(
                                &binding.binding().evm_address()
                            )
                        );
                        word[24..].copy_from_slice(&binding.binding().nonce().to_be_bytes());
                    }
                    layerx_client::sequencer::GET_EVM_ADDR_BY_LAYERX => {
                        assert_eq!(data[4..], binding.public_key());
                        word[12..].copy_from_slice(&binding.binding().evm_address());
                    }
                    other => panic!("unsupported binding read: {other:?}"),
                }
                json!({"jsonrpc":"2.0", "id":request["id"], "result":format!("0x{}", hex::encode(word))})
            } else if method == "eth_call"
                && request["params"][0]["to"] == "0x0000000000000000000000000000000000001018"
            {
                let call = &request["params"][0];
                let data = call
                    .get("data")
                    .or_else(|| call.get("input"))
                    .and_then(Value::as_str)
                    .expect("ABI call data");
                let data = must(hex::decode(data.trim_start_matches("0x")));
                assert!(
                    data.starts_with(
                        &layerx_crypto::evm_transaction::transaction_hash(b"getFeeDenom(address)")
                            [..4]
                    ),
                    "unsupported fee read"
                );
                json!({"jsonrpc":"2.0", "id":request["id"], "result":format!("0x{}", hex::encode(abi_string(b"upax")))})
            } else if method == "eth_call"
                && request["params"][0]["to"] == "0x0000000000000000000000000000000000001001"
            {
                let call = &request["params"][0];
                let data = call
                    .get("data")
                    .or_else(|| call.get("input"))
                    .and_then(Value::as_str)
                    .expect("ABI call data");
                let data = must(hex::decode(data.trim_start_matches("0x")));
                let answer = if data.starts_with(
                    &layerx_crypto::evm_transaction::transaction_hash(b"symbol(string)")[..4],
                ) {
                    abi_string(b"PAX")
                } else if data.starts_with(
                    &layerx_crypto::evm_transaction::transaction_hash(b"decimals(string)")[..4],
                ) {
                    let mut word = vec![0; 32];
                    word[31] = 6;
                    word
                } else {
                    panic!("unsupported fee read");
                };
                json!({"jsonrpc":"2.0", "id":request["id"], "result":format!("0x{}", hex::encode(answer))})
            } else if matches!(
                method,
                "eth_chainId"
                    | "eth_blockNumber"
                    | "eth_getBlockByNumber"
                    | "eth_getTransactionCount"
                    | "eth_getBalance"
                    | "eth_estimateGas"
            ) {
                rpc(&upstream, &request)
            } else {
                panic!("unexpected chain read: {method}: {request}");
            };
            http_response(&mut stream, &result);
        }
    });
    (url, peer)
}

fn certificate(issuer: Option<(&X509, &PKey<Private>)>) -> (X509, PKey<Private>) {
    let key = must(PKey::from_rsa(must(Rsa::generate(2048))));
    let mut name = must(X509NameBuilder::new());
    must(name.append_entry_by_text("CN", "localhost"));
    let name = name.build();
    let mut cert = must(X509::builder());
    must(cert.set_version(2));
    let mut serial = must(BigNum::new());
    must(serial.rand(128, MsbOption::MAYBE_ZERO, false));
    must(cert.set_serial_number(must(serial.to_asn1_integer()).as_ref()));
    must(cert.set_subject_name(&name));
    must(cert.set_issuer_name(issuer.map_or(&name, |(ca, _)| ca.subject_name())));
    must(cert.set_pubkey(&key));
    must(cert.set_not_before(must(Asn1Time::days_from_now(0)).as_ref()));
    must(cert.set_not_after(must(Asn1Time::days_from_now(1)).as_ref()));
    if let Some((ca, signing_key)) = issuer {
        must(cert.append_extension(must(BasicConstraints::new().critical().build())));
        must(
            cert.append_extension(must(
                KeyUsage::new()
                    .digital_signature()
                    .key_encipherment()
                    .build(),
            )),
        );
        must(cert.append_extension(must(ExtendedKeyUsage::new().server_auth().build())));
        let san = must(
            SubjectAlternativeName::new()
                .dns("localhost")
                .build(&cert.x509v3_context(Some(ca), None)),
        );
        must(cert.append_extension(san));
        must(cert.sign(signing_key, MessageDigest::sha256()));
    } else {
        must(cert.append_extension(must(BasicConstraints::new().critical().ca().build())));
        must(
            cert.append_extension(must(
                KeyUsage::new()
                    .critical()
                    .key_cert_sign()
                    .crl_sign()
                    .build(),
            )),
        );
        must(cert.sign(&key, MessageDigest::sha256()));
    }
    (cert.build(), key)
}

struct AuthorityIdentity<'a> {
    authority: &'a mut RemoteHumanAuthority,
    peer: &'a HumanPeer,
}
impl IdentityResolver for AuthorityIdentity<'_> {
    fn resolve(&mut self, did: &Did) -> Result<Option<CoreIdentity>, IdentityError> {
        self.authority.core_identity(self.peer, did).map(Some)
    }
}

fn authority_peer(
    peer: HumanPeer,
    identity: CoreIdentity,
    activity: ActivityType,
) -> (RemoteHumanAuthority, Peer) {
    authority_peer_with_identity(peer, Arc::new(Mutex::new(identity)), activity)
}

fn authority_peer_with_identity(
    peer: HumanPeer,
    identity: Arc<Mutex<CoreIdentity>>,
    activity: ActivityType,
) -> (RemoteHumanAuthority, Peer) {
    let (ca, ca_key) = certificate(None);
    let (cert, key) = certificate(Some((&ca, &ca_key)));
    let acceptor = must(TlsAcceptor::new(must(Identity::from_pkcs8(
        &must(cert.to_pem()),
        &must(key.private_key_to_pem_pkcs8()),
    ))));
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let port = must(listener.local_addr()).port();
    must(listener.set_nonblocking(true));
    let bearer = hex::encode(random::<32>());
    let authority = must(RemoteHumanAuthority::connect(
        &format!("https://localhost:{port}"),
        bearer.clone(),
        Duration::from_secs(3),
        1_048_576,
        &must(ca.to_der()),
    ));
    let worker = Peer::spawn(move |stop| {
        let subject = peer.subject.as_ref().expect("owner subject");
        let account = must(AccountId::parse(&subject.account));
        let account_id = must(layerx_wire::hash::account_id_for_protocol(&account, 3));
        while !stop.load(Ordering::Acquire) {
            let Some(stream) = incoming(&listener) else {
                continue;
            };
            let mut stream = must(acceptor.accept(stream));
            let (headers, body) = http_request(&mut stream);
            assert!(body.is_empty());
            assert!(headers
                .lines()
                .any(|line| line.eq_ignore_ascii_case(&format!("authorization: Bearer {bearer}"))));
            let path = headers
                .split_whitespace()
                .nth(1)
                .expect("authority route")
                .split('?')
                .next()
                .expect("route");
            let identity = must(identity.lock());
            let value = match path {
                "/v1/agent/registry" => {
                    json!({"modules":[{"module_id":activity.module() as u16, "activity_types":[activity.value()]}]})
                }
                "/v1/agent/subject-context" => {
                    json!({"subject_principal":peer.principal, "tenant":subject.transport_tenant, "agent_tenant":peer.tenant,
                    "owner_did":subject.owner, "account_id":hex::encode(account_id), "asset_id":hex::encode(subject.asset),
                    "verification_level":layerx_types::verify::VerificationLevel::COMMITTED.wire_label(), "observed_head_sequence":identity.head_sequence, "checkpoint_digest":hex::encode(random::<32>())})
                }
                "/v1/agent/balance-context" => {
                    json!({"account_id":hex::encode(account_id), "asset_id":hex::encode(subject.asset),
                    "currency":"USD", "observed_at":"loopback committed block", "age_seconds":0,
                    "maximum_age_seconds":315360000,
                    "sequencer_id":hex::encode(Sha256::digest(&identity.canonical_bytes)),
                    "sequencer_public_key":hex::encode(&identity.canonical_bytes),
                    "first_batch_number":1, "last_batch_number":HEAD + 1000})
                }
                "/v1/agent/identity" => {
                    json!({"canonical_core_bytes":hex::encode(&identity.canonical_bytes), "head_sequence":identity.head_sequence,
                    "revocation_sequence":identity.revocation_sequence, "frozen":identity.frozen, "verification_level": match identity.verification_level { layerx_types::verify::VerificationLevel::COMMITTED => layerx_types::verify::VerificationLevel::COMMITTED.wire_label(), layerx_types::verify::VerificationLevel::STATE_PROVEN => layerx_types::verify::VerificationLevel::STATE_PROVEN.wire_label(), _ => panic!("identity floor case") },
                    "authorities":identity.authorities.iter().map(|authority| match authority { ProtocolAuthority::PrimaryKey(id) => json!({"kind":"primary_key","id":hex::encode(id)}), _ => panic!("owner authority") }).collect::<Vec<_>>()})
                }
                other => panic!("unexpected authority read: {other}"),
            };
            http_response(&mut stream, &value);
        }
    });
    (authority, worker)
}

fn clock_peer(root: &Path) -> (Arc<RuntimeClock>, Peer) {
    let now = must(SystemTime::now().duration_since(UNIX_EPOCH))
        .as_millis()
        .try_into()
        .expect("wall time");
    clock_peer_at(root, now)
}

fn clock_peer_at(root: &Path, unix_milliseconds: u64) -> (Arc<RuntimeClock>, Peer) {
    let path = root.join("clock.sock");
    let listener = must(UnixListener::bind(&path));
    must(fs::set_permissions(
        &path,
        fs::Permissions::from_mode(0o600),
    ));
    must(listener.set_nonblocking(true));
    let generation = random::<16>();
    let worker = Peer::spawn(move |stop| {
        let start = Instant::now();
        while !stop.load(Ordering::Acquire) {
            match listener.accept() {
                Ok((mut stream, _)) => {
                    must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                    let mut bytes = [0; clock_protocol::REQUEST_BYTES];
                    must(stream.read_exact(&mut bytes));
                    let request = must(clock_protocol::Request::decode(&bytes));
                    thread::sleep(Duration::from_nanos(request.wait_nanoseconds));
                    let reading = ClockReading {
                        generation,
                        unix_milliseconds: unix_milliseconds
                            .checked_add(must(start.elapsed().as_millis().try_into()))
                            .expect("fixture clock time"),
                        monotonic_nanoseconds: must(start.elapsed().as_nanos().try_into()),
                    };
                    must(stream.write_all(&clock_protocol::response(request.counter, Ok(reading))));
                }
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(2))
                }
                Err(error) => panic!("clock accept: {error}"),
            }
        }
    });
    let clock = must(RuntimeClock::connect(ClockBinding {
        socket: path,
        process: std::process::id(),
        user: must(fs::metadata(root)).uid(),
    }));
    (clock, worker)
}

pub(crate) struct Fixture {
    pub(crate) identity: Arc<Mutex<CoreIdentity>>,
    pub(crate) owner: SharedAgentOwner<RemoteHumanAuthority>,
    pub(crate) peer: HumanPeer,
    pub(crate) credential: session::SessionCredential,
    pub(crate) account: layerx_crypto::account_derivation::DerivedAccount,
    endpoint: SequencerEndpoint,
    writes: Arc<AtomicUsize>,
    peers: Vec<Peer>,
    node: Node,
    root: PathBuf,
    previous_rpc: Option<std::ffi::OsString>,
    previous_chain: Option<std::ffi::OsString>,
    previous_comet: Option<std::ffi::OsString>,
}

impl Fixture {
    pub(crate) fn new() -> Self {
        let root = directory("native-send-preview");
        let account = must(derive_from_seed(&random::<64>(), 0));
        let public_key = account.layerx_public_key();
        let did = must(Did::new(account.did().as_bytes()));
        let activity = must(ActivityType::new(ModuleId::Asset, 5));
        let principal = hex::encode(public_key);
        let tenant = must(TenantId::new(must(
            layerx_identity_binding::subject_namespace("bootstrap-service", &principal),
        )));
        let peer = HumanPeer {
            uid: 7,
            principal,
            tenant: tenant.as_str().to_owned(),
            subject: Some(HumanSubject {
                transport_tenant: "bootstrap-service".to_owned(),
                transport_principal: "bootstrap-owner".to_owned(),
                owner: account.did(),
                account: format!("agent:{}:main", account.did()),
                asset: layerx_crypto::evm_transaction::transaction_hash(b"upax"),
                registration: None,
            }),
        };
        let peers = BTreeMap::from([(
            7,
            ("bootstrap-owner".to_owned(), "bootstrap-service".to_owned()),
        )]);
        let identity = CoreIdentity {
            canonical_bytes: public_key.to_vec(),
            head_sequence: HEAD,
            revocation_sequence: 1,
            verification_level: layerx_types::verify::VerificationLevel::COMMITTED,
            frozen: false,
            authorities: vec![ProtocolAuthority::PrimaryKey(public_key)],
        };
        let identity_source = Arc::new(Mutex::new(identity));
        let (mut authority, authority_worker) =
            authority_peer_with_identity(peer.clone(), Arc::clone(&identity_source), activity);
        let resolved = must(authority.core_identity(&peer, &did));
        assert_eq!(resolved.canonical_bytes, public_key);
        assert_eq!(resolved.head_sequence, HEAD);
        assert_eq!(resolved.revocation_sequence, 1);
        assert_eq!(
            resolved.verification_level,
            layerx_types::verify::VerificationLevel::COMMITTED
        );
        assert!(!resolved.frozen);
        assert_eq!(
            resolved.authorities,
            vec![ProtocolAuthority::PrimaryKey(public_key)]
        );
        let mut store = must(Store::open(root.join("store")));
        let identity = must(identity::register(
            &mut store,
            tenant.clone(),
            did.clone(),
            &mut AuthorityIdentity {
                authority: &mut authority,
                peer: &peer,
            },
        ));
        let credential = must(session::open(
            &mut store,
            &mut SessionRegistry::default(),
            &identity,
            OpenRequest {
                session_id: SessionId([8; 32]),
                token_id: random(),
                tenant: tenant.clone(),
                agent: did,
                authority: ProtocolAuthority::PrimaryKey(public_key),
                permitted_activity_types: BTreeSet::from([5]),
                scopes: BTreeSet::from(["prepare".to_owned()]),
                expiry_sequence: 1000,
                expiry_seconds: Some(4_000_000_000),
                opening_client: "sdk".to_owned(),
                policy_version: "v1".to_owned(),
            },
            HEAD,
        ))
        .credential();
        let retained = Arc::new(Mutex::new(store));
        let (clock, clock_worker) = clock_peer(&root);
        let node = Node::start();
        node.rpc(
            "anvil_setBalance",
            json!([
                format!("0x{}", hex::encode(account.evm_address())),
                "0x3635c9adc5dea00000"
            ]),
        );
        node.rpc(
            "anvil_setNonce",
            json!([format!("0x{}", hex::encode(account.evm_address())), "0x9"]),
        );
        node.rpc("anvil_mine", json!([format!("0x{HEAD:x}")]));
        let binding = Binding::new(125, account.evm_address(), 3).sign(account.layerx_seed());
        let writes = Arc::new(AtomicUsize::new(0));
        let (url, chain_worker) = chain_peer(&node, binding, Arc::clone(&writes));
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let chain = must(NodeRpc::connect(&NodeRpcConfig {
            url: url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(3),
        }));
        let node_head = must(endpoint.block_number());
        assert_eq!(node_head, HEAD);
        assert_eq!(
            must(chain.bound_address(&public_key)),
            Some(account.evm_address()),
            "the pending addr-precompile binding must resolve the consent key"
        );
        let mut operations = must(ProductionHumanOperations::new(
            authority,
            must(SequencerEndpoint::new(&url, &[])),
            Box::new(chain),
            Arc::clone(&retained),
            &peers,
            1_048_576,
            30_000,
            clock,
        ));
        must(operations.authorize_subject(&peer));
        operations.recover_tenants(std::slice::from_ref(&peer), u128::MAX);
        assert!(matches!(
            operations.write_admission(&peer.tenant),
            Some(Ok(()))
        ));
        let key_root = root.join("keys");
        must(fs::create_dir(&key_root));
        must(fs::set_permissions(
            &key_root,
            fs::Permissions::from_mode(0o700),
        ));
        let registry = must(SessionKeyRegistry::open(
            key_root,
            random::<32>().to_vec(),
            125,
            must(fs::metadata(&root)).uid(),
        ));
        let mut owner = must(UnifiedAgentOwner::new(
            operations,
            Arc::clone(&retained),
            &peers,
            vec![LimitConfig {
                id: LimitId(random()),
                name: "preview".to_owned(),
                scope: LimitScope::Tenant(random()),
                ceiling: u128::MAX,
                consumed: 0,
            }],
            registry,
        ));
        layerx_agentd::degraded::enter(
            &mut owner.degraded,
            layerx_agentd::degraded::Observation::Ready {
                reference: layerx_agentd::degraded::Reference {
                    head_sequence: node_head,
                    checkpoint: [0; 32],
                },
                maximum_rung: layerx_types::rung::Rung::Submitted,
            },
        );
        let owner = SharedAgentOwner::new(owner);
        let previous_rpc = std::env::var_os("LAYERX_AGENT_NODE_RPC_URL");
        let previous_chain = std::env::var_os("LAYERX_AGENT_NODE_CHAIN_ID");
        let previous_comet = std::env::var_os("LAYERX_AGENT_NODE_COMET_RPC_URL");
        std::env::set_var("LAYERX_AGENT_NODE_RPC_URL", &url);
        std::env::set_var("LAYERX_AGENT_NODE_CHAIN_ID", "125");
        Self {
            identity: identity_source,
            owner,
            peer,
            credential,
            account,
            endpoint,
            writes,
            peers: vec![authority_worker, clock_worker, chain_worker],
            node,
            root,
            previous_rpc,
            previous_chain,
            previous_comet,
        }
    }

    pub(crate) fn request(
        &self,
    ) -> (
        HumanPrepare,
        layerx_agent_api::identity::NativeLocalGrantConsentV1,
    ) {
        let (prepare, consent, _) = self.request_with_payload_expiry();
        (prepare, consent)
    }

    fn request_with_payload_expiry(
        &self,
    ) -> (
        HumanPrepare,
        layerx_agent_api::identity::NativeLocalGrantConsentV1,
        u64,
    ) {
        use layerx_agentd::capability::binding::SignedNativeLocalGrantV1;
        use layerx_agentd::capability::timed::{
            NativeSpendSourceV1, NativeTimedCapabilityV1, TimedCapability,
        };
        use layerx_agentd::session::NativeSessionScopeV1;
        use layerx_crypto::send::{send_context_hash, SendDebit};
        let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
        let from = must(layerx_wire::hash::account_id_for_protocol(
            &must(AccountId::for_asset(&self.account.did(), asset, asset)),
            3,
        ));
        let to = [9; 32];
        let idempotency_key = [10; 32];
        let header = must(self.endpoint.block_by_number(HEAD)).expect("mined head");
        let now = header
            .timestamp
            .checked_mul(1000)
            .expect("fixture head timestamp in milliseconds");
        let debit = SendDebit {
            from,
            to,
            asset,
            amount: 10,
            source_sequence: 7,
            idempotency_key,
            expires_at: now + 20_000,
            context_hash: send_context_hash(&from, &to, &asset, 10, &idempotency_key),
            conditions: vec![],
            authorization_kind: 1,
            network_id: 125,
            protocol_version: 3,
        };
        let signing = ed25519_dalek::SigningKey::from_bytes(self.account.layerx_seed());
        let canonical = must(debit.authorization_message());
        let message = must(layerx_crypto::SignatureMessage::new(
            layerx_wire::hash::Domain::SignaturePreimage,
            3,
            125,
            &canonical,
        ));
        let payload = must(debit.encode_signed(
            self.account.layerx_public_key(),
            signing.sign(&message.digest()).to_bytes(),
        ));
        let activity = must(ActivityType::new(ModuleId::Asset, 5));
        let registry = must(layerx_types::payload::ModuleRegistry::new(&[must(
            layerx_types::payload::ModuleRegistration::new(ModuleId::Asset, &[activity]),
        )]));
        let payload_hash = must(layerx_wire::hash::payload_hash_for(&must(
            layerx_types::payload::Payload::new(&registry, activity, &payload),
        )));
        let tenant = must(TenantId::new(self.peer.tenant.clone()));
        let native_activity = layerx_agent_api::identity::NativeActivity::from(activity);
        let mut grant = SignedNativeLocalGrantV1 {
            capability: NativeTimedCapabilityV1 {
                record: TimedCapability {
                    id: [8; 32],
                    parent: None,
                    tenant: tenant.clone(),
                    agent: self.account.did(),
                    authority: ProtocolAuthority::PrimaryKey(self.account.layerx_public_key()),
                    activity_types: BTreeSet::new(),
                    counterparties: BTreeSet::from([to]),
                    assets: BTreeSet::from([asset]),
                    amount_ceilings: BTreeMap::from([(asset, 2_000_000_000_000)]),
                    rate_ceilings: BTreeMap::from([(10, 100)]),
                    purposes: BTreeSet::new(),
                    expiry_seconds: header.timestamp + 30,
                    grant_not_after_ms: now + 30_000,
                    created_at_ms: now,
                    created_at_sequence: HEAD,
                    revoked: None,
                },
                activities: BTreeSet::from([native_activity]),
                purpose_commitments: BTreeSet::from([[11; 32]]),
                spend_ceilings: BTreeMap::from([(
                    (NativeSpendSourceV1::Principal, asset),
                    2_000_000_000_000,
                )]),
            },
            session: NativeSessionScopeV1 {
                tenant,
                agent: must(Did::new(self.account.did().as_bytes())),
                session_id: self.credential.session_id(),
                generation: self.credential.generation(),
                permitted_activities: BTreeSet::from([native_activity]),
            },
            expires_at_ms: now + 30_000,
            owner_public_key: self.account.layerx_public_key(),
            signature: [0; 64],
        };
        grant.signature = signing.sign(&must(grant.signing_digest())).to_bytes();
        let consent = layerx_agent_api::identity::NativeLocalGrantConsentV1 {
            capability: must(grant.capability.encode()),
            session_scope: must(grant.session.encode()),
            expires_at_ms: grant.expires_at_ms,
            owner_public_key: grant.owner_public_key,
            signature: grant.signature,
        };
        (
            HumanPrepare {
                activity_type: activity.value(),
                actor: self.account.did(),
                authority: hex::encode(self.account.layerx_public_key()),
                nonce: must(self.endpoint.pending_nonce(&self.account.evm_address())),
                not_before_height: now,
                not_after_height: now + 30_000,
                idempotency_key: hex::encode(idempotency_key),
                fee_limit: 1_000_000_000_000,
                payload,
                payload_hash,
                capability_id: Some([8; 32]),
            },
            consent,
            debit.expires_at,
        )
    }

    fn post_preview(&mut self, request_id: u64, commitment: [u8; 32]) -> Vec<u8> {
        let (prepare, grant) = self.request();
        let bytes = |out: &mut Vec<u8>, value: &[u8]| {
            out.extend_from_slice(&must(u32::try_from(value.len())).to_be_bytes());
            out.extend_from_slice(value);
        };
        let mut request = b"LXHAGT01".to_vec();
        request.push(NATIVE_SEND_PREVIEW_V1);
        bytes(&mut request, self.credential.tenant().as_str().as_bytes());
        request.extend_from_slice(&self.credential.session_id().0);
        request.extend_from_slice(&self.credential.token_id());
        request.extend_from_slice(&self.credential.generation().to_be_bytes());
        request.extend_from_slice(&request_id.to_be_bytes());
        request.extend_from_slice(&prepare.activity_type.to_be_bytes());
        bytes(&mut request, prepare.actor.as_bytes());
        bytes(&mut request, prepare.authority.as_bytes());
        request.extend_from_slice(&prepare.nonce.to_be_bytes());
        request.extend_from_slice(&prepare.not_before_height.to_be_bytes());
        request.extend_from_slice(&prepare.not_after_height.to_be_bytes());
        bytes(&mut request, prepare.idempotency_key.as_bytes());
        request.extend_from_slice(&prepare.fee_limit.to_be_bytes());
        bytes(&mut request, &prepare.payload);
        request.extend_from_slice(&prepare.payload_hash);
        request.extend_from_slice(&prepare.capability_id.expect("send capability"));
        request.extend_from_slice(&self.account.layerx_public_key());
        request.extend_from_slice(&prepare.not_after_height.to_be_bytes());
        request.extend_from_slice(&commitment);
        bytes(&mut request, &grant.capability);
        bytes(&mut request, &grant.session_scope);
        request.extend_from_slice(&grant.expires_at_ms.to_be_bytes());
        request.extend_from_slice(&grant.owner_public_key);
        request.extend_from_slice(&grant.signature);

        let deadline = Duration::from_secs(10);
        let maximum = 1_048_576;
        let (mut client, server) = must(UnixStream::pair());
        must(client.set_read_timeout(Some(deadline)));
        must(client.set_write_timeout(Some(deadline)));
        let mut transport = must(AcceptedTransport::new(server, maximum, deadline));
        must(client.write_all(&must(u32::try_from(request.len())).to_be_bytes()));
        must(client.write_all(&request));
        must(serve_one(&mut transport, &self.peer, &mut self.owner));
        let mut prefix = [0; 4];
        must(client.read_exact(&mut prefix));
        let length = u32::from_be_bytes(prefix) as usize;
        assert!(length <= maximum);
        let mut response = vec![0; length];
        must(client.read_exact(&mut response));
        response
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        assert_eq!(self.writes.load(Ordering::SeqCst), 0);
        assert_eq!(
            self.node.rpc(
                "eth_getTransactionCount",
                json!([
                    format!("0x{}", hex::encode(self.account.evm_address())),
                    "pending",
                ])
            ),
            "0x9"
        );
        self.peers.clear();
        for (name, previous) in [
            ("LAYERX_AGENT_NODE_RPC_URL", &self.previous_rpc),
            ("LAYERX_AGENT_NODE_CHAIN_ID", &self.previous_chain),
            ("LAYERX_AGENT_NODE_COMET_RPC_URL", &self.previous_comet),
        ] {
            match previous {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
        must(fs::remove_dir_all(&self.root));
    }
}

fn field(bytes: &mut Vec<u8>, value: &[u8]) {
    bytes.extend_from_slice(&must(u32::try_from(value.len())).to_be_bytes());
    bytes.extend_from_slice(value);
}

fn read_bytes<'a>(input: &mut &'a [u8]) -> &'a [u8] {
    let (length, rest) = input.split_at(4);
    let length = u32::from_be_bytes(must(length.try_into())) as usize;
    let (value, rest) = rest.split_at(length);
    *input = rest;
    value
}

#[test]
fn native_send_preview_is_an_unsigned_ledger_transfer_call() {
    let _guard = must(FIXTURE_LOCK.lock());
    let mut fixture = Fixture::new();
    let (prepare, local_grant) = fixture.request();
    let payload = prepare.payload.clone();
    let expires = prepare.not_after_height;
    let response = must(fixture.owner.native_send_owner_preview_v1(
        &fixture.peer,
        HumanNativeSendOwnerPreviewRequestV1 {
            credential: fixture.credential.clone(),
            request_id: 1,
            prepare,
            owner_public_key: fixture.account.layerx_public_key(),
            purpose_expires_at_ms: expires,
            commitment: [11; 32],
            economic_action: [8; 32],
            local_grant,
        },
    ));
    let mut input = response.bytes();
    assert_eq!(&input[..2], &1_u16.to_be_bytes());
    input = &input[2..];
    let canonical_bytes = read_bytes(&mut input);
    let signing_preimage = read_bytes(&mut input);
    let purpose = must(
        layerx_agent_api::identity::NativeSendPurposeV2::from_canonical_bytes(read_bytes(
            &mut input,
        )),
    );
    let call = must(layerx_crypto::evm_transaction::decode_unsigned(
        canonical_bytes,
    ));
    assert_eq!(call.chain_id, 125);
    assert_eq!(purpose.chain_id, 125);
    assert_eq!(purpose.transaction_type, 2);
    assert_eq!(purpose.evm_sender, fixture.account.evm_address());
    assert_eq!(
        purpose.consent_public_key,
        fixture.account.layerx_public_key()
    );
    assert_ne!(
        call.nonce, 7,
        "EVM nonce is independent of the ledger source sequence"
    );
    assert_eq!(
        call.nonce,
        must(
            fixture
                .endpoint
                .pending_nonce(&fixture.account.evm_address())
        )
    );
    let ledger = layerx_agentd::prepare::evm::PrecompileCall::ledger_transfer(payload);
    assert_eq!(call.to, ledger.target.address());
    assert_eq!(call.data, must(ledger.encode()));
    assert_eq!(signing_preimage, call.signing_hash());
    assert_eq!(canonical_bytes, call.encode_unsigned());
    assert_eq!(
        purpose.canonical_digest,
        <[u8; 32]>::from(Sha256::digest(canonical_bytes))
    );
    assert_eq!(purpose.preparation_id, purpose.canonical_digest);
    assert_eq!(u64::from_be_bytes(must(input[..8].try_into())), HEAD);
    assert_eq!(input.len(), 8 + 8 + 32 + 8);
    assert!(call.gas_limit > 0);
    assert!(call.max_fee_per_gas <= 1_000_000_000_000);
    assert_eq!(call.max_priority_fee_per_gas, 0);
    assert!(layerx_crypto::evm_transaction::decode_signed(canonical_bytes).is_err());
    for length in 0..canonical_bytes.len() {
        assert!(
            layerx_crypto::evm_transaction::decode_unsigned(&canonical_bytes[..length]).is_err()
        );
    }
    let mut trailing = canonical_bytes.to_vec();
    trailing.push(0);
    assert!(layerx_crypto::evm_transaction::decode_unsigned(&trailing).is_err());
    let mut zero_chain = call.clone();
    zero_chain.chain_id = 0;
    assert!(
        layerx_crypto::evm_transaction::decode_unsigned(&zero_chain.encode_unsigned()).is_err()
    );
    let (prepare, local_grant) = fixture.request();
    let preview = must(fixture.owner.native_send_preview_v1(
        &fixture.peer,
        HumanNativeSendPreviewRequestV1 {
            credential: fixture.credential.clone(),
            request_id: 2,
            prepare,
            owner_public_key: fixture.account.layerx_public_key(),
            purpose_expires_at_ms: expires,
            commitment: [11; 32],
            local_grant,
        },
    ));
    let mut input = preview.bytes();
    let unsigned = read_bytes(&mut input);
    assert_eq!(
        must(layerx_crypto::evm_transaction::decode_unsigned(unsigned)),
        call
    );
    assert_eq!(read_bytes(&mut input), call.signing_hash());
    let purpose = must(
        layerx_agent_api::identity::NativePreparationPurposeV1::from_canonical_bytes(read_bytes(
            &mut input,
        )),
    );
    assert_eq!(
        purpose.canonical_digest,
        <[u8; 32]>::from(Sha256::digest(unsigned))
    );
    let (prepare, local_grant) = fixture.request();
    assert!(matches!(
        fixture.owner.native_send_preview_v1(
            &fixture.peer,
            HumanNativeSendPreviewRequestV1 {
                credential: fixture.credential.clone(),
                request_id: 3,
                purpose_expires_at_ms: prepare.not_after_height,
                prepare,
                owner_public_key: fixture.account.layerx_public_key(),
                commitment: [12; 32],
                local_grant,
            },
        ),
        Err(layerx_agentd::human::HumanOperationError::Refused)
    ));

    let response = fixture.post_preview(4, [11; 32]);
    assert_eq!(&response[..9], b"LXHAGT01\x00");
    let mut input = &response[9..];
    let canonical = read_bytes(&mut input);
    let frame_call = must(layerx_crypto::evm_transaction::decode_unsigned(canonical));
    assert_eq!(frame_call, call);
    assert_eq!(read_bytes(&mut input), frame_call.signing_hash());
    let purpose = must(
        layerx_agent_api::identity::NativePreparationPurposeV1::from_canonical_bytes(read_bytes(
            &mut input,
        )),
    );
    assert_eq!(
        purpose.canonical_digest,
        <[u8; 32]>::from(Sha256::digest(canonical))
    );
    assert_eq!(purpose.commitment, [11; 32]);
    assert_eq!(u64::from_be_bytes(must(input[..8].try_into())), HEAD);
    assert_eq!(input.len(), 8 + 8 + 32 + 8);
    assert_eq!(&input[16..48], &fixture.account.layerx_public_key());
    let response = fixture.post_preview(5, [12; 32]);
    let mut expected_refusal = b"LXHAGT01\x01".to_vec();
    // SessionControl refuses the unmatched purpose commitment outside managed_agent.
    field(&mut expected_refusal, b"operation.refused");
    assert_eq!(response, expected_refusal);
}
