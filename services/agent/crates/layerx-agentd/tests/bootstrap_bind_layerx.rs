use layerx_agentd::agent_rpc::{handle_rpc, AgentRpcResponse};
use layerx_agentd::budget::{LimitConfig, LimitId, LimitScope};
use layerx_agentd::capability::{Capability, CapabilityDimensions, CapabilityId, RateCeiling};
use layerx_agentd::human::{HumanOperations, HumanPeer, HumanSubject};
use layerx_agentd::human_runtime::{
    HumanAuthorityBoundary, ProductionHumanOperations, RemoteHumanAuthority, SharedAgentOwner,
    UnifiedAgentOwner,
};
use layerx_agentd::identity::{
    self, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
use layerx_agentd::prepare::evm::{PrecompileTarget, ProductionEvmPreparationBoundary};
use layerx_agentd::session::{self, OpenRequest, SessionId, SessionRecord, SessionRegistry};
use layerx_agentd::session_keys::SessionKeyRegistry;
use layerx_agentd::store::{ObjectKind, Store, TenantId};
use layerx_client::client::{ClientConfig, ReconnectPolicy};
use layerx_client::lni::handshake::{encode_node_info, HandshakeConfig, NodeInfo, NodeRole};
use layerx_client::lni::schema::{encode_envelope, Envelope, Version};
use layerx_client::lni::transport::Limits;
use layerx_client::paxeer_binding::{Binding, SignedBinding};
use layerx_client::runtime_clock::{ClockBinding, RuntimeClock};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_client::Client;
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::session::{
    decode_session_key, issue_session_key, SessionKeyRequest, SessionPurpose,
};
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
const HEAD: u64 = 50;

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("bootstrap peer: {error:?}"))
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

fn number(value: &Value, field: &str) -> u64 {
    must(text(value, field).parse())
}
fn bytes<const N: usize>(value: &str) -> [u8; N] {
    must(must(hex::decode(value)).try_into())
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
            must(worker.join());
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
    must(binding.verify());
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
            } else {
                rpc(&upstream, &request)
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
            let value = match path {
                "/v1/agent/registry" => {
                    json!({"modules":[{"module_id":activity.module() as u16, "activity_types":[activity.value()]}]})
                }
                "/v1/agent/subject-context" => {
                    json!({"subject_principal":peer.principal, "tenant":subject.transport_tenant, "agent_tenant":peer.tenant,
                    "owner_did":subject.owner, "account_id":hex::encode(account_id), "asset_id":hex::encode(subject.asset),
                    "verification_level":"checkpoint_finalised", "observed_head_sequence":identity.head_sequence, "checkpoint_digest":hex::encode(random::<32>())})
                }
                "/v1/agent/identity" => {
                    json!({"canonical_core_bytes":hex::encode(&identity.canonical_bytes), "head_sequence":identity.head_sequence,
                    "revocation_sequence":identity.revocation_sequence, "frozen":identity.frozen, "verification_level":"checkpoint_finalised",
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
                        unix_milliseconds: must(SystemTime::now().duration_since(UNIX_EPOCH))
                            .as_millis()
                            .try_into()
                            .expect("wall time"),
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

fn node_peer(root: &Path) -> (Client, Peer) {
    let path = root.join("node.sock");
    let listener = must(UnixListener::bind(&path));
    must(listener.set_nonblocking(true));
    let info = NodeInfo {
        interface_version: Version::V1_8,
        protocol_version: 3,
        network_id: 125,
        role: NodeRole::Sequencer,
        chain_head_sequence: HEAD,
        latest_sealed_batch: 1,
        latest_finalised_checkpoint: random(),
        authorised_sequencer_key: ed25519_dalek::SigningKey::from_bytes(&random())
            .verifying_key()
            .to_bytes(),
        advertised_capabilities: vec![],
    };
    let worker = Peer::spawn(move |stop| {
        while !stop.load(Ordering::Acquire) {
            match listener.accept() {
                Ok((mut stream, _)) => {
                    must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                    let mut prefix = [0; 4];
                    must(stream.read_exact(&mut prefix));
                    let mut request = vec![0; u32::from_be_bytes(prefix) as usize];
                    must(stream.read_exact(&mut request));
                    let payload = must(encode_node_info(&info));
                    let response = must(encode_envelope(Envelope {
                        version: Version::V1_8,
                        message_tag: 2,
                        correlation_id: 0,
                        canonical_payload: &payload,
                        proof_material: &[],
                    }));
                    must(stream.write_all(&must(u32::try_from(response.len())).to_be_bytes()));
                    must(stream.write_all(&response));
                }
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    thread::sleep(Duration::from_millis(2))
                }
                Err(error) => panic!("LNI accept: {error}"),
            }
        }
    });
    let deadline = Duration::from_secs(3);
    let node = must(Client::connect(ClientConfig {
        endpoint: path,
        handshake: HandshakeConfig {
            built_interface_version: Version::V1_8,
            expected_protocol_version: 3,
            expected_network_id: 125,
        },
        limits: Limits {
            maximum_frame_bytes: 1_212_416,
            maximum_connections: 1,
            maximum_streams: 1,
            maximum_queued_bytes: 1_212_416,
            deadline,
        },
        reconnect: ReconnectPolicy {
            maximum_attempts: 1,
            base_delay: Duration::from_millis(1),
            maximum_delay: Duration::from_millis(1),
            jitter_percent: 0,
        },
    }));
    (node, worker)
}

struct OwnerFixture {
    owner: SharedAgentOwner<RemoteHumanAuthority>,
    retained: Arc<Mutex<Store>>,
    document: Value,
    endpoint: SequencerEndpoint,
    boundary: ProductionEvmPreparationBoundary,
    writes: Arc<AtomicUsize>,
    peers: Vec<Peer>,
    node: Node,
    root: PathBuf,
    previous_rpc: Option<std::ffi::OsString>,
    previous_chain: Option<std::ffi::OsString>,
}

impl OwnerFixture {
    fn new() -> Self {
        let root = directory("owner");
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
            principal: principal.clone(),
            tenant: tenant.as_str().to_owned(),
            subject: Some(HumanSubject {
                transport_tenant: "bootstrap-service".to_owned(),
                transport_principal: "bootstrap-owner".to_owned(),
                owner: account.did(),
                account: format!("agent:{}:main", account.did()),
                asset: random(),
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
            verification_level: layerx_types::verify::VerificationLevel::CHECKPOINT_FINALISED,
            frozen: false,
            authorities: vec![ProtocolAuthority::PrimaryKey(public_key)],
        };
        let (mut authority, authority_worker) = authority_peer(peer.clone(), identity, activity);
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
        let capability = must(Capability::new(
            CapabilityId(random()),
            tenant.clone(),
            CapabilityDimensions {
                activity_types: BTreeSet::from([5]),
                counterparties: BTreeSet::from([random()]),
                assets: BTreeSet::from([random()]),
                amount_ceiling: 1000,
                rate_ceiling: RateCeiling {
                    maximum_uses: 100,
                    window_sequences: 10,
                },
                purposes: BTreeSet::from(["bootstrap".to_owned()]),
                expiry_sequence: 1000,
            },
        ));
        must(capability.persist(&mut store));
        let record = SessionRecord {
            request: OpenRequest {
                session_id: SessionId(random()),
                token_id: random(),
                tenant: tenant.clone(),
                agent: did.clone(),
                authority: ProtocolAuthority::PrimaryKey(public_key),
                permitted_activity_types: BTreeSet::from([5]),
                scopes: BTreeSet::from(["prepare".to_owned()]),
                expiry_sequence: 1000,
                expiry_seconds: Some(u64::MAX),
                opening_client: "sdk".to_owned(),
                policy_version: "v1".to_owned(),
            },
            open: true,
            sequence: 0,
            budget_reserved: 0,
            subscription_cursor: 0,
            generation: 1,
            retired_token_ids: BTreeSet::new(),
        };
        let credential = must(session::open(
            &mut store,
            &mut SessionRegistry::default(),
            &identity,
            record.request.clone(),
            HEAD,
        ))
        .credential();
        let retained = Arc::new(Mutex::new(store));
        let (clock, clock_worker) = clock_peer(&root);
        let (client, node_worker) = node_peer(&root);
        let node_head = client.head();
        let node = Node::start();
        let chain = must(NodeRpc::connect(&NodeRpcConfig {
            url: node.url.clone(),
            chain_id: 125,
            deadline: Duration::from_secs(3),
        }));
        let mut operations = must(ProductionHumanOperations::new(
            authority,
            client,
            Box::new(chain),
            Arc::clone(&retained),
            &peers,
            1_048_576,
            30,
            clock,
        ));
        must(operations.authorize_subject(&peer));
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
        let seed = must(registry.prepare_seed(b"bootstrap", random()));
        let mut address = [0; 32];
        address[12..].copy_from_slice(&seed.evm_address());
        let issued = must(issue_session_key(&SessionKeyRequest {
            grantor: must(layerx_wire::hash::did_id_for_protocol(&did, 3)),
            session_public_key: address,
            not_before: HEAD,
            expires_at: Some(1000),
            permitted_activity_types: vec![activity],
            revocation_sequence: Some(1),
            fee_budget: None,
            purpose: SessionPurpose::Activity,
        }));
        must(registry.provision(issued.grant_id, seed.secret(), &issued));
        let limits = vec![LimitConfig {
            id: LimitId(random()),
            name: "bootstrap".to_owned(),
            scope: LimitScope::Tenant(random()),
            ceiling: 1000,
            consumed: 0,
        }];
        let mut owner = must(UnifiedAgentOwner::new(
            operations,
            Arc::clone(&retained),
            &peers,
            limits,
            registry,
        ));
        layerx_agentd::degraded::enter(
            &mut owner.degraded,
            layerx_agentd::degraded::Observation::Ready {
                reference: layerx_agentd::degraded::Reference {
                    head_sequence: node_head.chain_sequence,
                    checkpoint: node_head.finalised_checkpoint,
                },
                maximum_rung: layerx_types::rung::Rung::Submitted,
            },
        );
        let owner = must(SharedAgentOwner::new(owner).with_idempotency(
            root.join("idempotency"),
            1000,
            1000,
        ));
        let from = account.evm_address();
        node.rpc(
            "anvil_setBalance",
            json!([format!("0x{}", hex::encode(from)), "0x3635c9adc5dea00000"]),
        );
        node.rpc(
            "anvil_setNonce",
            json!([format!("0x{}", hex::encode(from)), "0x7"]),
        );
        let binding = Binding::new(125, from, 3).sign(account.layerx_seed());
        let writes = Arc::new(AtomicUsize::new(0));
        let (url, chain_worker) = chain_peer(&node, binding, Arc::clone(&writes));
        let endpoint = must(SequencerEndpoint::new(&url, &[]));
        let boundary = must(ProductionEvmPreparationBoundary::new(&url, &[]));
        let previous_rpc = std::env::var_os("LAYERX_AGENT_NODE_RPC_URL");
        let previous_chain = std::env::var_os("LAYERX_AGENT_NODE_CHAIN_ID");
        std::env::set_var("LAYERX_AGENT_NODE_RPC_URL", &url);
        std::env::set_var("LAYERX_AGENT_NODE_CHAIN_ID", "125");
        let context = json!({"tenant":tenant.as_str(), "agent_did":account.did(), "authority_ref":hex::encode(public_key), "permitted_activity_types":["5"], "expiry":u64::MAX.to_string(), "client":"sdk", "policy_version":"v1"});
        let document = json!({"tenant":tenant.as_str(), "credential": {"tenant":credential.tenant().as_str(), "session_id":hex::encode(credential.session_id().0), "token_id":hex::encode(credential.token_id()), "generation":credential.generation().to_string()},
            "agent_register": {"tenant":tenant.as_str(), "agent_did":account.did(), "authority_ref":hex::encode(public_key), "client":"sdk", "policy_version":"v1", "owner_address":hex::encode(from), "binding_consent":hex::encode(binding.signature()), "fee_limit":"1000000000000"},
            "session_open": {"context":context, "grant_id":hex::encode(issued.grant_id), "registration_payload":hex::encode(issued.registration_payload), "expiry_sequence":"1000", "purpose":hex::encode(random::<32>()), "fee_limit":"1000000000000"}});
        Self {
            owner,
            retained,
            document,
            endpoint,
            boundary,
            writes,
            peers: vec![authority_worker, clock_worker, node_worker, chain_worker],
            node,
            root,
            previous_rpc,
            previous_chain,
        }
    }
    fn envelope(&self, operation: &str, request: Value) -> Value {
        let mut key = [0; 32];
        getrandom::fill(&mut key).expect("fresh preparation request key");
        json!({"version": 1, "request_id": "1", "operation": operation, "request": request,
            "credential": self.document["credential"], "idempotency_key": hex::encode(key)})
    }

    fn call(&self, envelope: &Value) -> AgentRpcResponse {
        handle_rpc(
            &self.owner,
            &serde_json::to_vec(envelope).expect("actual RPC envelope"),
        )
    }

    fn outbox(&self) -> Vec<Vec<u8>> {
        self.retained
            .lock()
            .expect("retained store")
            .list_object_ids(
                &TenantId::new(text(&self.document, "tenant")).expect("tenant"),
                ObjectKind::Outbox,
            )
    }

    fn assert_prepared(
        &self,
        response: AgentRpcResponse,
        from: [u8; 20],
        to: [u8; 20],
        selector: [u8; 4],
        fee_limit: u128,
    ) -> Eip1559Call {
        assert_eq!(
            response.status,
            200,
            "{}",
            String::from_utf8_lossy(&response.body)
        );
        let response: Value =
            serde_json::from_slice(&response.body).expect("AuthorityResponse<Prepared>");
        let authority = &response["value"]["authority"];
        assert_eq!(authority["tenant"], self.document["credential"]["tenant"]);
        assert_eq!(
            authority["agent_did"],
            self.document["agent_register"]["agent_did"]
        );
        let prepared = &response["value"]["value"];
        assert_eq!(
            prepared.as_object().expect("unsigned preparation").len(),
            10
        );
        assert_eq!(prepared["value"], "0");
        let call = Eip1559Call {
            chain_id: number(prepared, "chain_id"),
            nonce: number(prepared, "nonce"),
            max_priority_fee_per_gas: text(prepared, "max_priority_fee_per_gas")
                .parse()
                .expect("priority fee"),
            max_fee_per_gas: text(prepared, "max_fee_per_gas")
                .parse()
                .expect("maximum fee"),
            gas_limit: number(prepared, "gas_limit"),
            to: bytes(text(prepared, "to")),
            data: hex::decode(text(prepared, "data")).expect("unsigned calldata"),
        };
        let state = self
            .boundary
            .preparation_state(&from)
            .expect("real node preparation state");
        assert_eq!(call.to, to);
        assert_eq!(call.data[..4], selector);
        assert_eq!(
            call.chain_id,
            self.endpoint.chain_id().expect("node chain id")
        );
        assert_eq!(call.nonce, state.pending_nonce);
        assert_eq!(
            call.max_fee_per_gas,
            state
                .base_fee_per_gas
                .checked_mul(2)
                .expect("base fee bound")
        );
        assert_eq!(call.max_priority_fee_per_gas, 0);
        assert!(call.max_fee_per_gas <= fee_limit);
        assert!(call.gas_limit > 0);
        assert_eq!(
            bytes::<32>(text(prepared, "signing_hash")),
            call.signing_hash()
        );
        assert!(number(prepared, "observed_head_height") <= state.head_height);
        assert!(prepared.get("signed_transaction").is_none());
        assert!(prepared.get("tx_hash").is_none());
        call
    }
}

impl Drop for OwnerFixture {
    fn drop(&mut self) {
        assert_eq!(
            self.writes.load(Ordering::SeqCst),
            0,
            "preparation broadcasts nothing"
        );
        assert_eq!(
            self.node.rpc(
                "eth_getTransactionCount",
                json!([
                    format!(
                        "0x{}",
                        text(&self.document["agent_register"], "owner_address")
                    ),
                    "pending"
                ])
            ),
            "0x7"
        );
        self.peers.clear();
        for (name, previous) in [
            ("LAYERX_AGENT_NODE_RPC_URL", &self.previous_rpc),
            ("LAYERX_AGENT_NODE_CHAIN_ID", &self.previous_chain),
        ] {
            match previous {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
        must(fs::remove_dir_all(&self.root));
    }
}

fn assert_refused(response: AgentRpcResponse, status: u16, reason: &str) {
    assert_eq!(
        response.status,
        status,
        "{}",
        String::from_utf8_lossy(&response.body)
    );
    let value: Value = serde_json::from_slice(&response.body).expect("typed refusal");
    assert_eq!(value["reason"], reason);
}

#[test]
fn agent_register_prepares_bind_layerx() {
    let _serial = FIXTURE_LOCK.lock().expect("real fixture lock");
    let fixture = OwnerFixture::new();
    let request = fixture.document["agent_register"].clone();
    let from = bytes(text(&request, "owner_address"));
    let fee_limit = text(&request, "fee_limit").parse().expect("fee limit");
    let before = fixture.outbox();
    let envelope = fixture.envelope("agent.register", request.clone());
    let mut missing = envelope.clone();
    missing["credential"] = Value::Null;
    assert_refused(fixture.call(&missing), 400, "envelope.credential");
    let call = fixture.assert_prepared(
        fixture.call(&envelope),
        from,
        layerx_client::account_binding::ADDR_PRECOMPILE,
        [0xdd, 0x9a, 0xa6, 0x28],
        fee_limit,
    );
    assert_eq!(
        &call.data[4..36],
        &bytes::<32>(
            text(&request, "agent_did")
                .strip_prefix("did:layerx:")
                .expect("owner DID")
        )
    );
    assert_eq!(
        &call.data[100..],
        &hex::decode(text(&request, "binding_consent")).expect("owner binding consent")
    );
    assert_eq!(call.data.len(), 164);
    let mut invalid = request.clone();
    let mut consent = hex::decode(text(&invalid, "binding_consent")).expect("consent");
    consent[0] ^= 1;
    invalid["binding_consent"] = json!(hex::encode(consent));
    assert_refused(
        fixture.call(&fixture.envelope("agent.register", invalid)),
        403,
        "binding.invalid_consent",
    );
    let mut too_expensive = request;
    let base_fee = fixture
        .boundary
        .preparation_state(&from)
        .expect("node base fee")
        .base_fee_per_gas;
    assert!(
        base_fee > 0,
        "fee refusal fixture must have a positive node base fee"
    );
    too_expensive["fee_limit"] = json!((base_fee * 2 - 1).to_string());
    assert_refused(
        fixture.call(&fixture.envelope("agent.register", too_expensive)),
        403,
        "preparation.fee_limit",
    );
    assert_eq!(
        fixture.outbox(),
        before,
        "preparation queues and broadcasts nothing"
    );
}

#[test]
fn session_open_prepares_grant_session_for_the_session_address() {
    let _serial = FIXTURE_LOCK.lock().expect("real fixture lock");
    let fixture = OwnerFixture::new();
    let request = fixture.document["session_open"].clone();
    let issued = decode_session_key(
        &hex::decode(text(&request, "registration_payload")).expect("real grant bytes"),
    )
    .expect("real canonical session grant");
    assert_eq!(issued.grant_id, bytes(text(&request, "grant_id")));
    let public_key = bytes(
        text(&request["context"], "agent_did")
            .strip_prefix("did:layerx:")
            .expect("owner DID"),
    );
    let from = fixture
        .endpoint
        .bound_address(&public_key)
        .expect("real owner address lookup")
        .expect("bound owner");
    let address = fixture
        .owner
        .lock()
        .expect("shared owner")
        .session_keys
        .load(issued.grant_id, issued.clone())
        .expect("real provisioned session key")
        .evm_address();
    let before = fixture.outbox();
    let envelope = fixture.envelope("session.open", request.clone());
    let mut missing = envelope.clone();
    missing["credential"] = Value::Null;
    assert_refused(fixture.call(&missing), 400, "envelope.credential");
    let call = fixture.assert_prepared(
        fixture.call(&envelope),
        from,
        PrecompileTarget::Auth.address(),
        [0x11, 0x07, 0xb4, 0x9e],
        text(&request, "fee_limit").parse().expect("fee limit"),
    );
    let actual = decode_session_key(&call.data[132..132 + issued.registration_payload.len()])
        .expect("grantSession canonical grant");
    assert_eq!(actual, issued);
    assert_eq!(actual.session_public_key[12..], address);
    assert_eq!(&call.data[68..100], &bytes::<32>(text(&request, "purpose")));
    assert_eq!(
        u64::from_be_bytes(call.data[60..68].try_into().expect("grant expiry")),
        number(&request, "expiry_sequence")
    );
    let mut too_expensive = request.clone();
    let base_fee = fixture
        .boundary
        .preparation_state(&from)
        .expect("node base fee")
        .base_fee_per_gas;
    assert!(
        base_fee > 0,
        "fee refusal fixture must have a positive node base fee"
    );
    too_expensive["fee_limit"] = json!((base_fee * 2 - 1).to_string());
    assert_refused(
        fixture.call(&fixture.envelope("session.open", too_expensive)),
        403,
        "preparation.fee_limit",
    );
    fixture
        .owner
        .lock()
        .expect("shared owner")
        .session_keys
        .revoke(issued.grant_id)
        .expect("revoke only the isolated fixture key");
    assert_refused(
        fixture.call(&fixture.envelope("session.open", request)),
        403,
        "session.key_unavailable",
    );
    assert_eq!(
        fixture.outbox(),
        before,
        "preparation queues and broadcasts nothing"
    );
}
