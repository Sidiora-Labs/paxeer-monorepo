use ed25519_dalek::Signer as _;
use layerx_agentd::budget::{LimitConfig, LimitId, LimitScope};
use layerx_agentd::human::{HumanNativeSendOwnerPreviewRequestV1, HumanPrepare};
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
use std::os::unix::net::UnixListener;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

static FIXTURE_LOCK: Mutex<()> = Mutex::new(());
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
        let stdout = child
            .stdout
            .take()
            .unwrap_or_else(|| panic!("anvil stdout"));
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
    fn rpc(&self, method: &str, params: &Value) -> Value {
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

fn binding_read(request: &Value, binding: &SignedBinding) -> Value {
    assert_eq!(request["params"][1], "pending");
    let call = &request["params"][0];
    let data = call
        .get("data")
        .or_else(|| call.get("input"))
        .and_then(Value::as_str)
        .unwrap_or_else(|| panic!("ABI call data"));
    let data = must(hex::decode(data.trim_start_matches("0x")));
    let mut word = [0; 32];
    match data[..4]
        .try_into()
        .unwrap_or_else(|error| panic!("selector: {error:?}"))
    {
        layerx_client::account_binding::SELECTOR_LAYERX_BIND_NONCE => {
            assert_eq!(
                data,
                layerx_client::account_binding::bind_nonce_call(&binding.binding().evm_address())
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
}

fn abi_string(value: &[u8]) -> Vec<u8> {
    let mut result = vec![0; 64 + value.len().div_ceil(32) * 32];
    result[31] = 32;
    result[56..64].copy_from_slice(&(value.len() as u64).to_be_bytes());
    result[64..64 + value.len()].copy_from_slice(value);
    result
}

fn chain_response(
    request: &Value,
    binding: &SignedBinding,
    writes: &AtomicUsize,
    keys: &Value,
    checkpoint_reads: &[Value],
    upstream: &str,
) -> Value {
    let method = text(request, "method");
    if method == "eth_sendRawTransaction" || method == "eth_sendTransaction" {
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
            .unwrap_or_else(|| panic!("checkpoint RPC recorded at the fixture head"));
        json!({"jsonrpc":"2.0", "id":request["id"], "result":recorded["result"]})
    } else if method == "eth_call"
        && request["params"][0]["to"]
            == format!(
                "0x{}",
                hex::encode(layerx_client::account_binding::ADDR_PRECOMPILE)
            )
    {
        binding_read(request, binding)
    } else if method == "eth_call"
        && request["params"][0]["to"] == "0x0000000000000000000000000000000000001018"
    {
        let call = &request["params"][0];
        let data = call
            .get("data")
            .or_else(|| call.get("input"))
            .and_then(Value::as_str)
            .unwrap_or_else(|| panic!("ABI call data"));
        let data = must(hex::decode(data.trim_start_matches("0x")));
        assert!(
            data.starts_with(
                &layerx_crypto::evm_transaction::transaction_hash(b"getFeeDenom(address)")[..4]
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
            .unwrap_or_else(|| panic!("ABI call data"));
        let data = must(hex::decode(data.trim_start_matches("0x")));
        let answer = if data
            .starts_with(&layerx_crypto::evm_transaction::transaction_hash(b"symbol(string)")[..4])
        {
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
        rpc(upstream, request)
    } else {
        panic!("unexpected chain read: {method}: {request}");
    }
}

fn chain_peer(node: &Node, binding: &SignedBinding, writes: Arc<AtomicUsize>) -> (String, Peer) {
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
        "../../../layerx-agentd/tests/support/human_budget_checkpoint.json"
    )));
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let url = format!("http://{}/", must(listener.local_addr()));
    must(listener.set_nonblocking(true));
    let upstream = node.url.clone();
    let binding = *binding;
    let peer = Peer::spawn(move |stop| {
        while !stop.load(Ordering::Acquire) {
            let Some(mut stream) = incoming(&listener) else {
                continue;
            };
            let (_, body) = http_request(&mut stream);
            let request: Value = must(serde_json::from_slice(&body));
            let result = chain_response(
                &request,
                &binding,
                &writes,
                &keys,
                &checkpoint_reads,
                &upstream,
            );
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
        let subject = peer
            .subject
            .as_ref()
            .unwrap_or_else(|| panic!("owner subject"));
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
                .unwrap_or_else(|| panic!("authority route"))
                .split('?')
                .next()
                .unwrap_or_else(|| panic!("route"));
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
                    "maximum_age_seconds":315_360_000,
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
        .unwrap_or_else(|error| panic!("wall time: {error:?}"));
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
                            .unwrap_or_else(|| panic!("fixture clock time")),
                        monotonic_nanoseconds: must(start.elapsed().as_nanos().try_into()),
                    };
                    must(stream.write_all(&clock_protocol::response(request.counter, Ok(reading))));
                }
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(2));
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

struct Fixture {
    owner: SharedAgentOwner<RemoteHumanAuthority>,
    peer: HumanPeer,
    credential: session::SessionCredential,
    account: layerx_crypto::account_derivation::DerivedAccount,
    endpoint: SequencerEndpoint,
    writes: Arc<AtomicUsize>,
    peers: Vec<Peer>,
    node: Node,
    root: PathBuf,
    previous_rpc: Option<std::ffi::OsString>,
    previous_chain: Option<std::ffi::OsString>,
    previous_comet: Option<std::ffi::OsString>,
}

type FixturePeers = BTreeMap<u32, (String, String)>;
struct FixtureIdentity {
    account: layerx_crypto::account_derivation::DerivedAccount,
    peer: HumanPeer,
    peers: FixturePeers,
    authority: RemoteHumanAuthority,
    authority_worker: Peer,
    store: Store,
    credential: session::SessionCredential,
}
fn fixture_identity(root: &Path) -> FixtureIdentity {
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
    FixtureIdentity {
        account,
        peer,
        peers,
        authority,
        authority_worker,
        store,
        credential,
    }
}

fn fixture_key_registry(root: &Path) -> SessionKeyRegistry {
    let key_root = root.join("keys");
    must(fs::create_dir(&key_root));
    must(fs::set_permissions(
        &key_root,
        fs::Permissions::from_mode(0o700),
    ));
    must(SessionKeyRegistry::open(
        key_root,
        random::<32>().to_vec(),
        125,
        must(fs::metadata(root)).uid(),
    ))
}

fn mark_owner_ready(owner: &mut UnifiedAgentOwner<RemoteHumanAuthority>, head_sequence: u64) {
    layerx_agentd::degraded::enter(
        &mut owner.degraded,
        layerx_agentd::degraded::Observation::Ready {
            reference: layerx_agentd::degraded::Reference {
                head_sequence,
                checkpoint: [0; 32],
            },
            maximum_rung: layerx_types::rung::Rung::Submitted,
        },
    );
}

impl Fixture {
    fn new() -> Self {
        let root = directory("native-send-preview");
        let FixtureIdentity {
            account,
            peer,
            peers,
            authority,
            authority_worker,
            store,
            credential,
        } = fixture_identity(&root);
        let public_key = account.layerx_public_key();
        let retained = Arc::new(Mutex::new(store));
        let (clock, clock_worker) = clock_peer(&root);
        let node = Node::start();
        node.rpc(
            "anvil_setBalance",
            &json!([
                format!("0x{}", hex::encode(account.evm_address())),
                "0x3635c9adc5dea00000"
            ]),
        );
        node.rpc(
            "anvil_setNonce",
            &json!([format!("0x{}", hex::encode(account.evm_address())), "0x9"]),
        );
        node.rpc("anvil_mine", &json!([format!("0x{HEAD:x}")]));
        let binding = Binding::new(125, account.evm_address(), 3).sign(account.layerx_seed());
        let writes = Arc::new(AtomicUsize::new(0));
        let (url, chain_worker) = chain_peer(&node, &binding, Arc::clone(&writes));
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
        let registry = fixture_key_registry(&root);
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
        mark_owner_ready(&mut owner, node_head);
        let owner = SharedAgentOwner::new(owner);
        let previous_rpc = std::env::var_os("LAYERX_AGENT_NODE_RPC_URL");
        let previous_chain = std::env::var_os("LAYERX_AGENT_NODE_CHAIN_ID");
        let previous_comet = std::env::var_os("LAYERX_AGENT_NODE_COMET_RPC_URL");
        std::env::set_var("LAYERX_AGENT_NODE_RPC_URL", &url);
        std::env::set_var("LAYERX_AGENT_NODE_CHAIN_ID", "125");
        Self {
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

    fn local_grant(
        &self,
        now: u64,
        timestamp: u64,
        asset: [u8; 32],
        to: [u8; 32],
        activity: ActivityType,
    ) -> layerx_agent_api::identity::NativeLocalGrantConsentV1 {
        use layerx_agentd::capability::binding::SignedNativeLocalGrantV1;
        use layerx_agentd::capability::timed::{
            NativeSpendSourceV1, NativeTimedCapabilityV1, TimedCapability,
        };
        use layerx_agentd::session::NativeSessionScopeV1;
        let signing = ed25519_dalek::SigningKey::from_bytes(self.account.layerx_seed());
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
                    expiry_seconds: timestamp + 30,
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
        layerx_agent_api::identity::NativeLocalGrantConsentV1 {
            capability: must(grant.capability.encode()),
            session_scope: must(grant.session.encode()),
            expires_at_ms: grant.expires_at_ms,
            owner_public_key: grant.owner_public_key,
            signature: grant.signature,
        }
    }

    fn request_with_payload_expiry(
        &self,
    ) -> (
        HumanPrepare,
        layerx_agent_api::identity::NativeLocalGrantConsentV1,
        u64,
    ) {
        use layerx_crypto::send::{send_context_hash, SendDebit};
        let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
        let from = must(layerx_wire::hash::account_id_for_protocol(
            &must(AccountId::for_asset(&self.account.did(), asset, asset)),
            3,
        ));
        let to = [9; 32];
        let idempotency_key = [10; 32];
        let header =
            must(self.endpoint.block_by_number(HEAD)).unwrap_or_else(|| panic!("mined head"));
        let now = header
            .timestamp
            .checked_mul(1000)
            .unwrap_or_else(|| panic!("fixture head timestamp in milliseconds"));
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
        let consent = self.local_grant(now, header.timestamp, asset, to, activity);
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
}

impl Drop for Fixture {
    fn drop(&mut self) {
        assert_eq!(self.writes.load(Ordering::SeqCst), 0);
        assert_eq!(
            self.node.rpc(
                "eth_getTransactionCount",
                &json!([
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

fn read_bytes<'a>(input: &mut &'a [u8]) -> &'a [u8] {
    let (length, rest) = input.split_at(4);
    let length = u32::from_be_bytes(must(length.try_into())) as usize;
    let (value, rest) = rest.split_at(length);
    *input = rest;
    value
}

use layerx_agent_api::identity::NativeSendPurposeV2;

type CommittedNativeCalls = BTreeMap<(String, Vec<u8>), Vec<u8>>;

fn abi_word(value: u128) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[16..].copy_from_slice(&value.to_be_bytes());
    bytes
}

fn committed_budget(
    owner: [u8; 32],
    asset: [u8; 32],
) -> (layerx_agentd::budget::ProtocolBudgetRecord, Vec<u8>) {
    let word = abi_word;
    // Use the production budget codec and the record used by the Human budget
    // checkpoint fixture, with its owner and asset bound to this fixture.
    let vectors: Value = must(serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../tests/vectors/native-budget-records.json"
    ))));
    let record = must(layerx_agentd::budget::ProtocolBudgetRecord::decode(&must(
        hex::decode(text(&vectors, "v1").trim_start_matches("0x")),
    )));
    let mut budget = vec![[0; 32]; 20 + record.delegates.len()];
    budget[0] = word(32);
    budget[1] = owner;
    budget[2] = record.budget_account;
    budget[3] = asset;
    budget[4] = record.purpose_hash;
    for (index, value) in [
        (5, record.per_period_limit),
        (6, record.configured_period_limit),
        (7, record.carry_cap),
        (8, record.spent_this_period),
        (9, record.carried),
        (10, u128::from(record.period_length)),
        (11, u128::from(record.period_start)),
        (12, u128::from(record.expiry)),
        (13, u128::from(record.revocation_sequence)),
        (14, u128::from(record.rollover_policy)),
        (15, u128::from(record.closed)),
        (16, u128::from(record.revoked)),
        (17, 576),
        (19, record.delegates.len() as u128),
    ] {
        budget[index] = word(value);
    }
    budget[18] = record.source_account.unwrap_or([0; 32]);
    budget[20..].copy_from_slice(&record.delegates);
    let budget = budget.concat();
    let decoded = must(
        layerx_agentd::budget::ProtocolBudgetState::decode_get_budget(
            record.budget_id,
            HEAD,
            &budget,
        ),
    );
    assert_eq!(decoded.record().owner, owner);
    assert_eq!(decoded.record().asset_id, asset);
    assert_eq!(decoded.record().remaining(), record.remaining());

    (record, budget)
}

fn ledger_metadata(asset: [u8; 32]) -> Vec<u8> {
    let word = abi_word;
    // getAsset returns the twelve-word ledger metadata tuple, including three
    // dynamic fields. The fee denomination is the same upax asset as the send.
    let mut metadata = vec![[0; 32]; 12];
    metadata[0] = asset;
    metadata[1] = word(384);
    metadata[2] = word(448);
    metadata[3] = word(6);
    metadata[9] = word(512);
    let mut metadata = metadata.concat();
    for value in [b"PAX".as_slice(), b"Paxeer", b""] {
        metadata.extend_from_slice(&word(value.len() as u128));
        metadata.extend_from_slice(value);
        metadata.resize(metadata.len().div_ceil(32) * 32, 0);
    }

    metadata
}

fn committed_native_send_calls(binding: &SignedBinding) -> CommittedNativeCalls {
    let word = abi_word;
    let public = binding.public_key();
    let did = must(Did::new(
        format!("did:layerx:{}", hex::encode(public)).as_bytes(),
    ));
    let owner = must(layerx_wire::hash::did_id_for_protocol(&did, 3));
    let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
    let account = must(AccountId::for_asset(
        must(std::str::from_utf8(did.as_bytes())),
        asset,
        asset,
    ));
    let main = must(layerx_wire::hash::account_id_for_protocol(&account, 3));
    let mut address = [0; 32];
    address[12..].copy_from_slice(&binding.binding().evm_address());
    let mut did_binding = [public, word(64), word(did.as_bytes().len() as u128)].concat();
    did_binding.extend_from_slice(did.as_bytes());
    did_binding.resize(did_binding.len().div_ceil(32) * 32, 0);

    let metadata = ledger_metadata(asset);
    let (record, budget) = committed_budget(owner, asset);

    let mut calls = BTreeMap::new();
    let mut add = |target: [u8; 20], signature: &[u8], args: &[u8], answer: Vec<u8>| {
        let selector: [u8; 4] = layerx_crypto::evm_transaction::transaction_hash(signature)[..4]
            .try_into()
            .unwrap_or_else(|error| panic!("four-byte selector: {error:?}"));
        let calldata = [selector.as_slice(), args].concat();
        assert!(calls
            .insert((format!("0x{}", hex::encode(target)), calldata), answer)
            .is_none());
    };
    let layerx = layerx_client::account_binding::ADDR_PRECOMPILE;
    let mut ledger = [0; 20];
    ledger[18..].copy_from_slice(&[0x10, 0x1a]);
    let mut budget_precompile = ledger;
    budget_precompile[19] = 0x1b;
    add(
        layerx,
        b"getEvmAddrByLayerX(bytes32)",
        &public,
        address.to_vec(),
    );
    add(
        layerx,
        b"getUnifiedAccount(address)",
        &address,
        [address, word(128), public, main, word(0)].concat(),
    );
    add(layerx, b"getLayerXDid(address)", &address, did_binding);
    add(ledger, b"getAsset(bytes32)", &asset, metadata);
    add(
        ledger,
        b"accountId(bytes32,bytes32)",
        &[owner, asset].concat(),
        main.to_vec(),
    );
    add(ledger, b"accountOwner(bytes32)", &main, owner.to_vec());
    add(
        ledger,
        b"nextSequence(bytes32,bytes32)",
        &[main, asset].concat(),
        word(7).to_vec(),
    );
    add(
        budget_precompile,
        b"getBudget(bytes32)",
        &record.budget_id,
        budget,
    );
    add(
        budget_precompile,
        b"budgetRemaining(bytes32)",
        &record.budget_id,
        word(record.remaining()).to_vec(),
    );
    // balanceOf has two legitimate accounts, selected by the full calldata.
    // The ledger account balance covers both the send and its declared fee.
    add(
        ledger,
        b"balanceOf(bytes32,bytes32)",
        &[main, asset].concat(),
        word(2_000_000_000_000).to_vec(),
    );
    add(
        ledger,
        b"balanceOf(bytes32,bytes32)",
        &[record.budget_account, asset].concat(),
        word(record.remaining()).to_vec(),
    );
    calls
}

// The recorded checkpoint has no captured predecessor or result ranges. Build
// the surrounding empty-block history with its real fixture validator, using
// the production header hash and precommit encoder. Both production verifiers
// authenticate every generated commit before the peer can serve it.
fn history_validator(
    verifier: &layerx_programs::chain_proof::ChainProofVerifier,
    key: &ed25519_dalek::SigningKey,
) -> layerx_agentd::read::CommitValidator {
    let public_key = key.verifying_key().to_bytes();
    assert_eq!(verifier.validators.validators().len(), 1);
    assert_eq!(verifier.validators.validators()[0].public_key, public_key);
    layerx_agentd::read::CommitValidator {
        address: layerx_agentd::read::CommitValidator::address_for_key(&public_key),
        public_key,
        voting_power: verifier.validators.validators()[0].voting_power,
    }
}

fn signed_history_commit(
    recorded: &Value,
    height: u64,
    timestamp: &str,
    parent: [u8; 32],
    empty_results: [u8; 32],
    block_hash: [u8; 32],
    signature: &[u8; 64],
) -> Value {
    let mut signed = recorded["signedHeader"].clone();
    signed["header"]["height"] = json!(height.to_string());
    signed["header"]["time"] = json!(timestamp);
    signed["header"]["last_block_id"]["hash"] = json!(hex::encode(parent));
    signed["header"]["last_results_hash"] = json!(hex::encode(empty_results));
    signed["commit"]["height"] = json!(height.to_string());
    signed["commit"]["block_id"]["hash"] = json!(hex::encode(block_hash));
    signed["commit"]["signatures"][0]["timestamp"] = json!(timestamp);
    signed["commit"]["signatures"][0]["signature"] =
        json!(openssl::base64::encode_block(signature));
    signed
}

fn history_header(
    h: &Value,
    validator: layerx_agentd::read::CommitValidator,
    height: u64,
    seconds: i64,
    parent: [u8; 32],
    app_hash: [u8; 32],
) -> layerx_agentd::read::CheckpointHeader {
    use layerx_agentd::read::{BlockId, CheckpointHeader, CommitTimestamp};
    let empty_results = layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[]);
    let digest = |name: &str| -> [u8; 32] {
        must(hex::decode(text(h, name)))
            .try_into()
            .unwrap_or_else(|error| panic!("recorded header digest: {error:?}"))
    };
    CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".to_owned(),
        height,
        time: CommitTimestamp { seconds, nanos: 0 },
        last_block_id: Some(BlockId {
            hash: parent,
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some(digest("last_commit_hash")),
        data_hash: Some(digest("data_hash")),
        validators_hash: layerx_agentd::read::validator_set_hash(&[validator]),
        next_validators_hash: layerx_agentd::read::validator_set_hash(&[validator]),
        consensus_hash: digest("consensus_hash"),
        app_hash,
        last_results_hash: Some(empty_results),
        evidence_hash: Some(digest("evidence_hash")),
        proposer_address: validator.address,
    }
}

fn native_send_history_commits(
    recorded: &Value,
    validators: &Value,
    upstream: &str,
) -> BTreeMap<u64, Value> {
    use layerx_agentd::read::{BlockCommit, BlockId, CommitVote};
    let verifier = layerx_programs::chain_proof::ChainProofVerifier {
        chain_id: "125".to_owned(),
        validators: must(layerx_client::rpc::commit::decode_validators(validators)),
    };
    let original = must(layerx_client::rpc::commit::verify_commit_material(
        &recorded["signedHeader"],
        &verifier,
    ));
    let key = ed25519_dalek::SigningKey::from_bytes(&[1; 32]);
    let validator = history_validator(&verifier, &key);
    let h = &recorded["signedHeader"]["header"];
    let empty_results = layerx_programs::chain_proof::merkle::hash_from_byte_slices(&[]);
    let head_seconds = original.time_ms() / 1000;
    assert_eq!(h["time"], "2026-09-14T05:59:04Z");
    let mut parent: [u8; 32] = must(hex::decode(text(&h["last_block_id"], "hash")))
        .try_into()
        .unwrap_or_else(|error| panic!("recorded predecessor: {error:?}"));
    let mut commits = BTreeMap::new();
    for (height, seconds, timestamp) in [
        (HEAD - 1, head_seconds - 11, "2026-09-14T05:58:53Z"),
        (HEAD, head_seconds, "2026-09-14T05:59:04Z"),
        (HEAD + 1, head_seconds + 1, "2026-09-14T05:59:05Z"),
    ] {
        // The two blocks whose complete native result sets are served below
        // contain no transactions on the real loopback EVM node either.
        if height <= HEAD {
            let block = rpc(
                upstream,
                &json!({"jsonrpc":"2.0", "id":1,
                "method":"eth_getBlockByNumber", "params":[format!("0x{height:x}"), false]}),
            );
            assert!(block.get("error").is_none(), "{block}");
            assert_eq!(block["result"]["transactions"], json!([]));
        }
        let header = history_header(h, validator, height, seconds, parent, original.app_hash());
        let mut commit = BlockCommit {
            height,
            round: 1,
            block_id: BlockId {
                hash: header.block_hash(),
                part_set_total: 1,
                part_set_hash: [9; 32],
            },
            votes: Vec::new(),
        };
        let signature = key
            .sign(&commit.vote_sign_bytes("125", header.time))
            .to_bytes();
        commit.votes.push(CommitVote {
            validator_address: validator.address,
            voting_power: validator.voting_power,
            timestamp: header.time,
            signature,
        });
        let checkpoint = must(layerx_agentd::read::checkpoint(
            height,
            "125",
            &header,
            &commit,
            &[validator],
        ));
        let signed = signed_history_commit(
            recorded,
            height,
            timestamp,
            parent,
            empty_results,
            checkpoint.block_hash,
            &signature,
        );
        let authenticated = must(layerx_client::rpc::commit::verify_commit_material(
            &signed, &verifier,
        ));
        assert_eq!(authenticated.header_hash(), checkpoint.block_hash);
        assert_eq!(authenticated.app_hash(), original.app_hash());
        assert_eq!(authenticated.time_ms(), seconds * 1000);
        parent = checkpoint.block_hash;
        assert!(commits.insert(height, signed).is_none());
    }
    commits
}

struct ReceiptPeerState {
    keys: Value,
    commit: Value,
    validators: Value,
    history_account: String,
    history_commits: BTreeMap<u64, Value>,
    committed_calls: CommittedNativeCalls,
    timestamp: u64,
    upstream: String,
    header_hash: [u8; 32],
    app_hash: [u8; 32],
}
impl ReceiptPeerState {
    fn response(&self, request: &Value) -> Value {
        let method = text(request, "method");
        if method == "px_getReceiptKeys" {
            json!({"jsonrpc": "2.0", "id": request["id"], "result": self.keys})
        } else if method == "px_getCommit" || method == "px_getValidators" {
            assert_eq!(request["params"], json!([HEAD.to_string()]));
            let result = if method == "px_getCommit" {
                &self.commit
            } else {
                &self.validators
            };
            json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
        } else if method == "px_getHistory" {
            assert_eq!(request["params"][0], self.history_account);
            assert!(request["params"][1].is_null() || request["params"][1].is_string());
            assert_eq!(request["params"][2], 100);
            json!({"jsonrpc":"2.0", "id":request["id"], "result":{
                "account":self.history_account, "items":[], "next_cursor":null,
                "indexedHeight":HEAD, "chainHeight":HEAD,
                "freshness":{"version":1, "database":"ok", "status":"ready",
                    "sources":[{"source":"comet", "source_head":HEAD,
                        "indexed_cursor":HEAD, "finalized_cursor":HEAD,
                        "last_success_at":self.timestamp, "freshness_secs":0,
                        "last_error_at":null, "reconciled":true,
                        "consecutive_failures":0, "state":"ready"}]}
            }})
        } else if method == "commit" || method == "block_results" {
            let height = must(text(&request["params"], "height").parse::<u64>());
            let signed = self
                .history_commits
                .get(&height)
                .unwrap_or_else(|| panic!("covered policy history block"));
            let result = if method == "commit" {
                json!({"signed_header":signed})
            } else {
                assert!((HEAD - 1..=HEAD).contains(&height));
                json!({"height":height.to_string(), "txs_results":[]})
            };
            eprintln!("repair_22_31_560 chain peer: authenticated history method={method} height={height}");
            json!({"jsonrpc":"2.0", "id":request["id"], "result":result})
        } else if method == "eth_call" && request["params"][1] == format!("0x{HEAD:x}") {
            self.committed_call(request)
        } else {
            if method == "eth_getProof" || method == "abci_query" {
                eprintln!("repair_22_31_560 chain peer: committed proof request={request}");
            }
            let mut response = rpc(&self.upstream, request);
            if response.get("error").is_some() {
                eprintln!("repair_22_31_560 chain peer: request={request} response={response}");
            }
            if method == "eth_getBlockByNumber"
                && (request["params"][0] == format!("0x{HEAD:x}")
                    || request["params"][0] == "latest")
            {
                assert!(response.get("error").is_none(), "{response}");
                let block = &mut response["result"];
                assert_eq!(block["number"], format!("0x{HEAD:x}"));
                block["hash"] = json!(format!("0x{}", hex::encode(self.header_hash)));
                block["stateRoot"] = json!(format!("0x{}", hex::encode(self.app_hash)));
                block["timestamp"] = json!(format!("0x{:x}", self.timestamp));
            }
            response
        }
    }
    fn committed_call(&self, request: &Value) -> Value {
        let call = &request["params"][0];
        let data = call
            .get("data")
            .or_else(|| call.get("input"))
            .and_then(Value::as_str)
            .unwrap_or_else(|| panic!("committed call data"));
        let data = must(hex::decode(data.trim_start_matches("0x")));
        assert!(data.len() >= 4, "committed call selector: {request}");
        let target = text(call, "to").to_owned();
        if let Some(answer) = self.committed_calls.get(&(target.clone(), data.clone())) {
            eprintln!(
                "repair_22_31_560 chain peer: served committed call target={target} selector=0x{}",
                hex::encode(&data[..4])
            );
            json!({"jsonrpc": "2.0", "id": request["id"],
                        "result": format!("0x{}", hex::encode(answer))})
        } else {
            assert!(
                !self
                    .committed_calls
                    .keys()
                    .any(|(address, expected)| address == &target && expected[..4] == data[..4]),
                "committed call arguments: {request}"
            );
            let response = rpc(&self.upstream, request);
            if response.get("error").is_some() {
                eprintln!("repair_22_31_560 chain peer: request={request} response={response}");
            }
            response
        }
    }
}

fn fixture_with_receipt_keys() -> Fixture {
    let mut fixture = Fixture::new();
    let upstream = must(NodeRpcConfig::from_environment()).url;
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    must(listener.set_nonblocking(true));
    let url = format!("http://{}/", must(listener.local_addr()));
    let receipt_key = ed25519_dalek::SigningKey::from_bytes(fixture.account.layerx_seed());
    let key_id: [u8; 32] = Sha256::digest(receipt_key.verifying_key().as_bytes()).into();
    let keys = json!({"keys": [{
        "keyId": format!("0x{}", hex::encode(key_id)),
        "publicKey": format!("0x{}", hex::encode(receipt_key.verifying_key().as_bytes())),
        "firstHeight": 1,
        "lastHeight": HEAD + 1000
    }]});
    let decoded = must(layerx_client::sequencer::decode_receipt_keys(&keys));
    assert_eq!(decoded.len(), 1);
    assert_eq!(decoded[0].key_id, key_id);
    assert_eq!(
        decoded[0].public_key,
        receipt_key.verifying_key().to_bytes()
    );
    assert!((decoded[0].first_height..=decoded[0].last_height).contains(&HEAD));
    let checkpoint_reads: Vec<Value> = must(serde_json::from_str(include_str!(
        "../../../layerx-agentd/tests/support/human_budget_checkpoint.json"
    )));
    let checkpoint_result = |method: &str| {
        checkpoint_reads
            .iter()
            .find(|recorded| {
                recorded["method"] == method && recorded["params"] == json!([HEAD.to_string()])
            })
            .unwrap_or_else(|| panic!("checkpoint RPC recorded at the fixture head"))["result"]
            .clone()
    };
    let commit = checkpoint_result("px_getCommit");
    let validators = checkpoint_result("px_getValidators");
    let verifier = layerx_programs::chain_proof::ChainProofVerifier {
        chain_id: "125".to_owned(),
        validators: must(layerx_client::rpc::commit::decode_validators(&validators)),
    };
    let history_commits = native_send_history_commits(&commit, &validators, &upstream);
    let commit = json!({"height":HEAD.to_string(), "signedHeader":history_commits[&HEAD]});
    let head = must(layerx_client::rpc::commit::verify_commit_material(
        &commit["signedHeader"],
        &verifier,
    ));
    assert_eq!(head.chain_id(), "125");
    assert_eq!(must(u64::try_from(head.height())), HEAD);
    let timestamp = must(u64::try_from(head.time_ms())) / 1000;
    let binding =
        Binding::new(125, fixture.account.evm_address(), 3).sign(fixture.account.layerx_seed());
    must(binding.verify());
    let committed_calls = committed_native_send_calls(&binding);
    let history_account = format!("0x{}", hex::encode(fixture.account.evm_address()));
    let state = ReceiptPeerState {
        keys,
        commit,
        validators,
        history_account,
        history_commits,
        committed_calls,
        timestamp,
        upstream,
        header_hash: head.header_hash(),
        app_hash: head.app_hash(),
    };
    let peer = Peer::spawn(move |stop| {
        let _receipt_key = receipt_key;
        while !stop.load(Ordering::Acquire) {
            let Some(mut stream) = incoming(&listener) else {
                continue;
            };
            let (_, body) = http_request(&mut stream);
            let request: Value = must(serde_json::from_slice(&body));
            let response = state.response(&request);
            http_response(&mut stream, &response);
        }
    });
    fixture.peers.push(peer);
    std::env::set_var("LAYERX_AGENT_NODE_RPC_URL", &url);
    std::env::set_var("LAYERX_AGENT_NODE_COMET_RPC_URL", &url);
    fixture.endpoint = must(SequencerEndpoint::new(&url, &[]));
    reopen_daemon(&mut fixture);
    fixture
}

fn restart_policy(fixture: &Fixture, restart: &str) -> (TenantId, PathBuf) {
    let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
    let account = must(layerx_wire::hash::account_id_for_protocol(
        &must(AccountId::for_asset(&fixture.account.did(), asset, asset)),
        3,
    ));
    let commitment = [11_u8; 32];
    let destination = [9_u8; 32];
    let policy = json!({
        "schema": "layerx.native-effect-policy.v1", "version": "native-send-test-v1",
        "network_id": 125, "step_limit": 100,
        "rules": [
            {"id": "operation", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
             "scope": {"kind": "Operation"}, "required_approval": false, "purpose_commitment": commitment},
            {"id": "transfer", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
             "scope": {"kind": "Transfer", "from": account, "to": destination,
                       "asset": asset, "maximum_amount": "10"},
             "required_approval": false, "purpose_commitment": commitment},
            {"id": "fee", "effect": "Permit", "activity": {"module": 1, "ordinal": 5},
             "scope": {"kind": "Fee", "source_account": account, "asset": asset,
                       "maximum_amount": "1000000000000"},
             "required_approval": false, "purpose_commitment": commitment}
        ]
    });
    let policy_tenant = must(TenantId::new(fixture.peer.transport_tenant().to_owned()));
    let policy_path = fixture
        .root
        .join(format!("native-effect-policy-{restart}.json"));
    must(fs::write(
        &policy_path,
        must(serde_json::to_vec(&json!({
            "version": "layerx.native-effect-policy-source.v1",
            "tenant": policy_tenant.as_str(), "policy": policy.to_string(),
        }))),
    ));
    must(fs::set_permissions(
        &policy_path,
        fs::Permissions::from_mode(0o600),
    ));
    (policy_tenant, policy_path)
}

fn restart_budget(
    fixture: &Fixture,
    restart: &str,
    operations: &mut ProductionHumanOperations<RemoteHumanAuthority>,
) -> LimitConfig {
    let asset = layerx_crypto::evm_transaction::transaction_hash(b"upax");
    let account = must(layerx_wire::hash::account_id_for_protocol(
        &must(AccountId::for_asset(&fixture.account.did(), asset, asset)),
        3,
    ));
    let limit = LimitConfig {
        id: LimitId([12; 16]),
        name: "preview".to_owned(),
        scope: LimitScope::Tenant(
            Sha256::digest(fixture.credential.tenant().as_str().as_bytes()).into(),
        ),
        ceiling: u128::MAX,
        consumed: 0,
    };
    let stable_id = layerx_agentd::budget::enrolment_limit_id(
        fixture.credential.tenant(),
        &limit.scope,
        &limit.id,
    );
    let denominations_path = fixture
        .root
        .join(format!("native-send-denominations-{restart}.json"));
    must(fs::write(
        &denominations_path,
        must(serde_json::to_vec(&json!({
            "version": 1, "tenant": fixture.credential.tenant().as_str(),
            "limits": [{"stable_id": hex::encode(stable_id),
                "asset": hex::encode(asset), "source": hex::encode(account)}]
        }))),
    ));
    must(fs::set_permissions(
        &denominations_path,
        fs::Permissions::from_mode(0o600),
    ));
    must(
        operations.attach_program_budget_denominations(&BTreeMap::from([(
            fixture.credential.tenant().clone(),
            denominations_path,
        )])),
    );
    limit
}

#[allow(
    deprecated,
    reason = "Retains the recorded checkpoint verification floor in the restart compatibility fixture"
)]
fn reopen_daemon(fixture: &mut Fixture) {
    let retained = must(fixture.owner.lock()).session_control.store();
    {
        let mut store = must(retained.lock());
        let replacement = must(Store::open(fixture.root.join("closed-store")));
        drop(std::mem::replace(&mut *store, replacement));
        *store = must(Store::open(fixture.root.join("store")));
    }
    let public_key = fixture.account.layerx_public_key();
    let activity = must(ActivityType::new(ModuleId::Asset, 5));
    let (authority, authority_worker) = authority_peer(
        fixture.peer.clone(),
        CoreIdentity {
            canonical_bytes: public_key.to_vec(),
            head_sequence: HEAD,
            revocation_sequence: 1,
            verification_level: layerx_types::verify::VerificationLevel::CHECKPOINT_FINALISED,
            frozen: false,
            authorities: vec![ProtocolAuthority::PrimaryKey(public_key)],
        },
        activity,
    );
    let restart = hex::encode(random::<4>());
    let clock_root = fixture.root.join(format!("restart-clock-{restart}"));
    must(fs::create_dir(&clock_root));
    must(fs::set_permissions(
        &clock_root,
        fs::Permissions::from_mode(0o700),
    ));
    let config = must(NodeRpcConfig::from_environment());
    let endpoint = must(SequencerEndpoint::new(&config.url, &[]));
    let checkpoint = must(
        layerx_agentd::read::ServedCheckpoint::acquire_at_committed_block(
            &endpoint, HEAD, None, "125",
        ),
    );
    let core_time_ms = must(u64::try_from(checkpoint.header.time.seconds))
        .checked_mul(1000)
        .unwrap_or_else(|| panic!("checkpoint time in milliseconds"));
    let (clock, clock_worker) = clock_peer_at(&clock_root, core_time_ms);
    let trust = must(layerx_agentd::export::ExportTrustSource::new(
        must(endpoint.receipt_keys()),
        checkpoint.validators,
    ));
    let (policy_tenant, policy_path) = restart_policy(fixture, &restart);
    let peers = BTreeMap::from([(
        7,
        ("bootstrap-owner".to_owned(), "bootstrap-service".to_owned()),
    )]);
    let mut operations = must(ProductionHumanOperations::new(
        authority,
        must(SequencerEndpoint::new(&config.url, &[])),
        Box::new(must(NodeRpc::connect(&config))),
        Arc::clone(&retained),
        &peers,
        1_048_576,
        30_000,
        clock,
    ));
    must(operations.attach_native_effect_policies(&BTreeMap::from([(policy_tenant, policy_path)])));
    let limit = restart_budget(fixture, &restart, &mut operations);
    operations.install_export_trust(trust, Duration::from_secs(3));
    must(operations.authorize_subject(&fixture.peer));
    operations.recover_tenants(std::slice::from_ref(&fixture.peer), u128::MAX);
    assert!(matches!(
        operations.write_admission(&fixture.peer.tenant),
        Some(Ok(()))
    ));
    let keys = fixture.root.join(format!("restart-keys-{restart}"));
    must(fs::create_dir(&keys));
    must(fs::set_permissions(
        &keys,
        fs::Permissions::from_mode(0o700),
    ));
    let registry = must(SessionKeyRegistry::open(
        keys,
        random::<32>().to_vec(),
        125,
        must(fs::metadata(&fixture.root)).uid(),
    ));
    let mut owner = must(UnifiedAgentOwner::new(
        operations,
        retained,
        &peers,
        vec![limit],
        registry,
    ));
    mark_owner_ready(&mut owner, HEAD);
    fixture.owner = must(SharedAgentOwner::new(owner).with_idempotency(
        fixture.root.join("rpc-idempotency"),
        1000,
        1000,
    ));
    fixture.peers.extend([authority_worker, clock_worker]);
}

fn with_fixture_lock(work: impl FnOnce() + std::panic::UnwindSafe) {
    let guard = must(FIXTURE_LOCK.lock());
    let outcome = std::panic::catch_unwind(work);
    drop(guard);
    if let Err(panic) = outcome {
        std::panic::resume_unwind(panic);
    }
}
