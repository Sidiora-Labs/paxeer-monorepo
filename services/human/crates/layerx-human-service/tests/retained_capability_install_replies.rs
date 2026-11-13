use layerx_agent_api::identity::{AgentDid, AuthorityRef};
use layerx_agentd::capability::{Capability, CapabilityId};
use layerx_agentd::human::{HumanCapabilityInstall, HumanPeer};
use layerx_agentd::human_runtime::{
    has_completed_capability_install, retained_capability_producers, verify_enrolment_expiry,
    RemoteHumanAuthority,
};
use layerx_agentd::store::{key, ObjectKind, StorageClass, Store, TenantId, TenantKey};
use layerx_human_service::agents::{
    AgentCreationContract, AgentEvidence, AgentFailure, CapabilityProvision, CreationStage,
};
use layerx_human_service::custody::{CustodySigner, EnvelopeKms, Keystore, SigningLimits};
use layerx_human_service::frame::Limits;
use layerx_human_service::server::agent_creation::{CreationBounds, ProductionAgentCreation, Rung};
use layerx_human_service::server::agent_runtime::AgentRuntime;
use layerx_human_service::trace::TraceId;
use layerx_human_test_support::{self as support, StepClock};
use layerx_sdk::Client;
use layerx_types::clock::Clock;
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use sha2::{Digest as _, Sha256};
use std::fmt::Debug;
use std::os::unix::net::UnixListener;
use std::path::Path;
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;
const HEIGHT: u64 = 11;
const MAX_FRAME: usize = 8192;
const OWNER_DID: &str = "did:layerx:owner-receipt-fixture";
fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("retained capability reply: {error:?}"))
}
fn hex(value: &[u8]) -> String {
    layerx_human_service::hex::lower(value)
}
fn registry() -> ModuleRegistry {
    checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Budget,
        &[
            checked(ActivityType::new(ModuleId::Budget, 1)),
            checked(ActivityType::new(ModuleId::Budget, 2)),
        ],
    ))]))
}

fn capability() -> CapabilityProvision {
    CapabilityProvision {
        action_key: [0x71; 32],
        did: checked(Did::new(b"did:layerx:owner-receipt-fixture")),
        capability_id: [0x72; 32],
        activity_types: [checked(ActivityType::new(ModuleId::Budget, 1)).ordinal()].into(),
        counterparties: [[0x73; 32]].into(),
        assets: [[0x22; 32]].into(),
        amount_ceiling: 500,
        rate_maximum_uses: 10,
        rate_window_sequences: 100,
        purposes: ["operations".to_owned()].into(),
        expiry_sequence: 1000,
        primary_authority: [0x74; 32],
    }
}

mod capability_install_boundary {
    use super::*;
    use std::collections::BTreeMap;
    use std::io::{Read, Write};
    use std::net::{TcpListener, TcpStream};
    use std::path::Path;

    use layerx_agentd::human::{serve_one, AcceptedTransport, HumanPeer};
    use layerx_agentd::human_runtime::{ProductionHumanOperations, RemoteHumanAuthority};
    use layerx_agentd::node_rpc::{NodeRpc, NodeRpcConfig};
    use layerx_agentd::store::Store;
    use layerx_client::sequencer::SequencerEndpoint;
    use layerx_types::verify::VerificationLevel;
    use serde_json::{json, Value};

    const DEADLINE: Duration = Duration::from_secs(5);

    fn accept(listener: &TcpListener) -> TcpStream {
        let mut accept_steps = 0..5_000;
        loop {
            match listener.accept() {
                Ok((stream, _)) => {
                    checked(stream.set_read_timeout(Some(DEADLINE)));
                    checked(stream.set_write_timeout(Some(DEADLINE)));
                    return stream;
                }
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(
                        accept_steps.next().is_some(),
                        "capability authority deadline"
                    );
                    thread::sleep(Duration::from_millis(1));
                }
                Err(error) => panic!("capability authority accept: {error}"),
            }
        }
    }

    fn http_request(stream: &mut (impl Read + Write)) -> (String, Vec<u8>) {
        let mut headers = Vec::new();
        while !headers.ends_with(b"\r\n\r\n") {
            let mut byte = [0];
            checked(stream.read_exact(&mut byte));
            headers.push(byte[0]);
            assert!(headers.len() <= MAX_FRAME);
        }
        let headers = checked(String::from_utf8(headers));
        let length = headers
            .lines()
            .filter_map(|line| line.split_once(':'))
            .find(|(name, _)| name.eq_ignore_ascii_case("content-length"))
            .map_or(0, |(_, value)| checked(value.trim().parse::<usize>()));
        assert!(length <= MAX_FRAME);
        let mut body = vec![0; length];
        checked(stream.read_exact(&mut body));
        (headers, body)
    }

    fn http_response(stream: &mut impl Write, document: &Value) {
        let body = checked(serde_json::to_vec(document));
        checked(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()));
        checked(stream.write_all(&body));
        checked(stream.flush());
    }

    pub(super) fn authority(count: usize) -> (String, JoinHandle<()>) {
        let request = capability();
        let listener = checked(TcpListener::bind("127.0.0.1:0"));
        checked(listener.set_nonblocking(true));
        let endpoint = format!(
            "https://localhost:{}",
            checked(listener.local_addr()).port()
        );
        let identity = checked(native_tls::Identity::from_pkcs8(
            include_bytes!("support/approval_fixture/loopback.pem"),
            include_bytes!("support/approval_fixture/loopback-key.pem"),
        ));
        let acceptor = checked(native_tls::TlsAcceptor::new(identity));
        let worker = thread::spawn(move || {
            let committed = VerificationLevel::COMMITTED.wire_rank();
            let mut digest = Sha256::new();
            for part in [
                b"layerx-human/agent-create/agent-evidence/v1".as_slice(),
                &[5],
                &request.action_key,
                &request.capability_id,
                &HEIGHT.to_be_bytes(),
                &[committed],
            ] {
                digest.update(checked(u32::try_from(part.len())).to_be_bytes());
                digest.update(part);
            }
            let digest: [u8; 32] = digest.finalize().into();
            let mut summary = b"LXGS2".to_vec();
            summary.extend_from_slice(&request.capability_id);
            summary.extend_from_slice(&[0x70; 32]);
            summary.extend_from_slice(&request.primary_authority);
            summary.extend_from_slice(&request.action_key);
            summary.extend_from_slice(&[0x75; 32]);
            summary.extend_from_slice(&request.expiry_sequence.to_be_bytes());
            summary.extend_from_slice(&(1_u64 << (ModuleId::Budget as u16)).to_be_bytes());
            let activity = *request.activity_types.first().or_fail("activity type");
            summary.extend_from_slice(&activity.to_be_bytes());
            summary.extend_from_slice(&activity.to_be_bytes());
            summary.extend_from_slice(&1_000_000_u64.to_be_bytes());
            summary.extend_from_slice(&2_000_000_u64.to_be_bytes());
            summary.extend_from_slice(&0_u64.to_be_bytes());
            assert_eq!(summary.len(), 209);
            let identity = json!({
                "canonical_core_bytes": hex(&request.primary_authority),
                "head_sequence": HEIGHT, "revocation_sequence": 0,
                "verification_level": "committed", "frozen": false,
                "authorities": [{"kind": "primary_key", "id": hex(&request.primary_authority)}]
            });
            let scope = json!({
                "activity_types": request.activity_types,
                "counterparties": request.counterparties.iter().map(|v| hex(v)).collect::<Vec<_>>(),
                "assets": request.assets.iter().map(|v| hex(v)).collect::<Vec<_>>(),
                "amount_ceiling": request.amount_ceiling.to_string(),
                "expiry_sequence": request.expiry_sequence,
                "enforceable_dimensions": ["activity_type", "counterparty", "asset", "amount", "expiry"],
                "observed_sequence": HEIGHT, "verification": committed,
                "evidence_digest": hex(&digest), "canonical_core_bytes": hex(&summary)
            });
            for _ in 0..count {
                let mut stream = checked(acceptor.accept(accept(&listener)));
                let (headers, body) = http_request(&mut stream);
                assert!(body.is_empty());
                assert!(headers.starts_with("GET /v1/agent/"));
                assert!(headers.contains("tenant=tenant-alice&principal=alice&"));
                let document = if headers.starts_with("GET /v1/agent/identity?") {
                    &identity
                } else {
                    assert!(headers.starts_with("GET /v1/agent/capability-scope?"));
                    for (name, value) in [
                        ("authority", request.primary_authority),
                        ("action_key", request.action_key),
                        ("capability_id", request.capability_id),
                    ] {
                        assert!(headers.contains(&format!("{name}={}", hex(&value))));
                    }
                    &scope
                };
                http_response(&mut stream, document);
            }
        });
        (endpoint, worker)
    }

    pub(super) fn node(count: usize) -> (String, JoinHandle<()>) {
        let listener = checked(TcpListener::bind("127.0.0.1:0"));
        checked(listener.set_nonblocking(true));
        let endpoint = format!("http://{}", checked(listener.local_addr()));
        let worker = thread::spawn(move || {
            for _ in 0..count {
                let mut stream = accept(&listener);
                let (_, body) = http_request(&mut stream);
                let request: Value = checked(serde_json::from_slice(&body));
                assert_eq!(request["method"], "eth_chainId");
                assert_eq!(request["params"], json!([]));
                http_response(
                    &mut stream,
                    &json!({"jsonrpc":"2.0", "id":request["id"], "result":"0x4d"}),
                );
            }
        });
        (endpoint, worker)
    }

    pub(super) fn operations(
        store: Arc<Mutex<Store>>,
        authority_endpoint: &str,
        node_endpoint: &str,
        peer: &HumanPeer,
        clock: Arc<dyn Clock>,
    ) -> ProductionHumanOperations<RemoteHumanAuthority> {
        let peers = BTreeMap::from([(peer.uid, (peer.principal.clone(), peer.tenant.clone()))]);
        let authority = checked(RemoteHumanAuthority::connect(
            authority_endpoint,
            "capability-install-authority-token".to_owned(),
            DEADLINE,
            MAX_FRAME,
            include_bytes!("support/approval_fixture/loopback-ca.der"),
        ));
        checked(ProductionHumanOperations::new(
            authority,
            checked(SequencerEndpoint::new(node_endpoint, &[])),
            Box::new(checked(NodeRpc::connect(&NodeRpcConfig {
                url: node_endpoint.to_owned(),
                chain_id: 77,
                deadline: DEADLINE,
            }))),
            store,
            &peers,
            MAX_FRAME,
            30,
            clock,
        ))
    }

    pub(super) fn run(
        socket: &Path,
        store: Arc<Mutex<Store>>,
        authority_endpoint: &str,
        node_endpoint: &str,
        custody: &CustodySigner,
        requests: Vec<CapabilityProvision>,
        tenant: &str,
    ) -> Vec<Result<AgentEvidence, AgentFailure>> {
        let clock: Arc<dyn Clock> = Arc::new(StepClock::new(1_000_000_000));
        let peer = HumanPeer {
            uid: 1000,
            tenant: tenant.to_owned(),
            principal: "alice".to_owned(),
            subject: None,
        };
        let mut operations = operations(
            store,
            authority_endpoint,
            node_endpoint,
            &peer,
            Arc::clone(&clock),
        );
        let listener = checked(UnixListener::bind(socket));
        checked(listener.set_nonblocking(true));
        let count = requests.len();
        let server = thread::spawn(move || {
            for _ in 0..count {
                let mut accept_steps = 0..5_000;
                let stream = loop {
                    match listener.accept() {
                        Ok((stream, _)) => break stream,
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            assert!(accept_steps.next().is_some(), "capability install deadline");
                            thread::sleep(Duration::from_millis(1));
                        }
                        Err(error) => panic!("capability install accept: {error}"),
                    }
                };
                let mut transport = checked(AcceptedTransport::new(stream, MAX_FRAME, DEADLINE));
                checked(serve_one(&mut transport, &peer, &mut operations));
            }
        });
        let mut runtime = checked(AgentRuntime::new_with_clock(
            socket,
            Limits {
                maximum_frame_bytes: MAX_FRAME,
                maximum_connections: 1,
                maximum_streams: 1,
                maximum_queued_bytes: MAX_FRAME,
                deadline: DEADLINE,
            },
            registry(),
            clock,
        ));
        let client = checked(Client::daemon(
            "/run/layerx-agentd.sock",
            layerx_agent_api::agent_api_schema_v1().version,
        ));
        let trace = TraceId::mint([0x31; 16]);
        let mut creation = checked(ProductionAgentCreation::new(
            &mut runtime,
            &client,
            custody,
            &trace,
            checked(AgentDid::new(OWNER_DID)),
            checked(AuthorityRef::new(hex(&capability().primary_authority))),
            CreationBounds {
                timestamp_span: 30,
                fee_limit: 10,
            },
        ));
        let results = requests
            .into_iter()
            .map(|request| creation.narrow_capability(request))
            .collect();
        checked(server.join());
        results
    }
}

fn tenant() -> TenantId {
    checked(TenantId::new("tenant-alice"))
}

fn action_key(owner: TenantId) -> TenantKey {
    let mut id = b"human-capability-action-v1:".to_vec();
    id.extend_from_slice(&capability().action_key);
    checked(key(owner, ObjectKind::Idempotency, id))
}

fn install_request() -> HumanCapabilityInstall {
    let request = capability();
    HumanCapabilityInstall {
        action_key: request.action_key,
        agent: checked(String::from_utf8(request.did.as_bytes().to_vec())),
        authority_id: request.primary_authority,
        capability_id: request.capability_id,
        activity_types: request.activity_types.into_iter().collect(),
        counterparties: request.counterparties.into_iter().collect(),
        assets: request.assets.into_iter().collect(),
        amount_ceiling: request.amount_ceiling,
        rate_maximum_uses: request.rate_maximum_uses,
        rate_window_sequences: request.rate_window_sequences,
        purposes: request.purposes.into_iter().collect(),
        expiry_sequence: request.expiry_sequence,
    }
}

fn peer() -> HumanPeer {
    HumanPeer {
        uid: 1000,
        tenant: "tenant-alice".to_owned(),
        principal: "alice".to_owned(),
        subject: None,
    }
}

fn record(store: &Arc<Mutex<Store>>) -> Vec<u8> {
    checked(store.lock())
        .get(&action_key(tenant()))
        .or_fail("retained action")
        .bytes()
        .to_vec()
}

fn installed(store: &Arc<Mutex<Store>>) -> Capability {
    let guard = checked(store.lock());
    let store: &Store = &guard;
    checked(Capability::restore(
        store,
        tenant(),
        CapabilityId(capability().capability_id),
    ))
    .or_fail("installed capability")
}

fn snapshot(path: &Path) -> Vec<u8> {
    checked(std::fs::read(path.join("store.bin")))
}

fn open_fixture(path: &Path, bytes: &[u8]) -> Arc<Mutex<Store>> {
    use std::os::unix::fs::PermissionsExt as _;
    checked(std::fs::create_dir_all(path));
    checked(std::fs::set_permissions(
        path,
        std::fs::Permissions::from_mode(0o700),
    ));
    checked(std::fs::write(path.join("store.bin"), bytes));
    checked(std::fs::set_permissions(
        path.join("store.bin"),
        std::fs::Permissions::from_mode(0o600),
    ));
    Arc::new(Mutex::new(checked(Store::open(path))))
}

fn exchange(
    root: &Path,
    store: &Arc<Mutex<Store>>,
    custody: &CustodySigner,
    requests: Vec<CapabilityProvision>,
    owner: &str,
    authority_reads: usize,
) -> Vec<Result<AgentEvidence, AgentFailure>> {
    let (authority, authority_server) = capability_install_boundary::authority(authority_reads);
    let (node, node_server) = capability_install_boundary::node(1);
    let socket = root.join("capability.sock");
    let results = capability_install_boundary::run(
        &socket,
        Arc::clone(store),
        &authority,
        &node,
        custody,
        requests,
        owner,
    );
    checked(authority_server.join());
    checked(node_server.join());
    checked(std::fs::remove_file(socket));
    results
}

fn grant_discovered(store: &Arc<Mutex<Store>>) {
    assert!(checked(has_completed_capability_install(
        store,
        &tenant(),
        CapabilityId(capability().capability_id)
    )));
    let (endpoint, server) = capability_install_boundary::authority(2);
    let mut authority = checked(RemoteHumanAuthority::connect(
        &endpoint,
        "capability-install-authority-token".to_owned(),
        Duration::from_secs(5),
        MAX_FRAME,
        include_bytes!("support/approval_fixture/loopback-ca.der"),
    ));
    let expiry = checked(verify_enrolment_expiry(
        &mut authority,
        store,
        &peer(),
        &capability().did,
        CapabilityId(capability().capability_id),
    ));
    assert_eq!(expiry.not_after_ms(), 2_000_000);
    checked(server.join());
}

// These retained snapshots match this exact request (capability()), including its
// authority, tenant-alice action binding, and sequence 11. The archived install
// bodies from each producer run below and prove every retained byte, including
// the capability and the full historical evidence digest. Their store codec was
// schema 3; schema 4 changes only the header for these two object kinds.
const PRE_RUNG_PRODUCER: &str = "f0d685372a0262a696e085204fc5e048db4d73c7";
const POST_RUNG_PRODUCER: &str = "2c7ff104282ab5e1e67afeb2576086a482d8a366";
const PRE_RUNG_STORE: &[u8] = include_bytes!("fixtures/issue_1758/pre_rung_store.bin");
const POST_RUNG_STORE: &[u8] = include_bytes!("fixtures/issue_1758/post_rung_store.bin");

fn assert_historical_producer(root: &Path, pre_rung: bool, producer: &str, fixture: &[u8]) {
    // Reproduce the old producer, rather than synthesizing a successful reply.
    let path = root.join(format!("producer-{pre_rung}"));
    let store = Arc::new(Mutex::new(checked(Store::open(&path))));
    let (authority, authority_server) = capability_install_boundary::authority(2);
    let (node, node_server) = capability_install_boundary::node(1);
    let mut operations = capability_install_boundary::operations(
        Arc::clone(&store),
        &authority,
        &node,
        &peer(),
        Arc::new(StepClock::new(1_000_000_000)),
    );
    let original = checked(retained_capability_producers::install(
        &mut operations,
        &peer(),
        &install_request(),
        pre_rung,
    ));
    assert_eq!(original.bytes()[40], if pre_rung { 6 } else { 2 });
    let mut historical = snapshot(&path);
    assert_eq!(&historical[4..8], &4_u32.to_be_bytes());
    historical[4..8].copy_from_slice(&3_u32.to_be_bytes());
    assert_eq!(
        historical, fixture,
        "retained bytes from producer {producer}"
    );
    checked(authority_server.join());
    checked(node_server.join());
    drop(operations);
    drop(store);
}

fn assert_replay_schema(
    root: &Path,
    custody: &CustodySigner,
    request: &CapabilityProvision,
    expected: &AgentEvidence,
    pre_rung: bool,
    schema: u32,
    fixture: &[u8],
) {
    let path = root.join(format!("replay-{pre_rung}-{schema}"));
    let mut bytes = fixture.to_vec();
    bytes[4..8].copy_from_slice(&schema.to_be_bytes());
    let store = open_fixture(&path, &bytes);
    assert_eq!(&snapshot(&path)[4..8], &4_u32.to_be_bytes());
    let before_capability = installed(&store);
    let before_record = record(&store);
    let before = snapshot(&path);
    grant_discovered(&store);
    assert_eq!(
        snapshot(&path),
        before,
        "lookup must not rewrite legacy records"
    );
    let first = checked(
        exchange(
            root,
            &store,
            custody,
            vec![request.clone()],
            "tenant-alice",
            0,
        )
        .remove(0),
    );
    assert_eq!(&first, expected);
    assert!(first.creation_complete());
    let converted = record(&store);
    assert_eq!(converted[0], 2);
    assert_eq!(&converted[1..34], &before_record[1..34]);
    assert_eq!(converted.len(), 107);
    assert_eq!(installed(&store), before_capability);
    assert_eq!(
        checked(store.lock()).list_object_ids(&tenant(), ObjectKind::Capability),
        vec![request.capability_id.to_vec()]
    );
    grant_discovered(&store);
    let converted_snapshot = snapshot(&path);
    assert_eq!(Arc::strong_count(&store), 1);
    drop(store);

    let store = Arc::new(Mutex::new(checked(Store::open(&path))));
    let mut changed = request.clone();
    changed.amount_ceiling -= 1;
    let mut changed_did = request.clone();
    changed_did.did = checked(Did::new(b"did:layerx:another-owner"));
    let mut changed_authority = request.clone();
    changed_authority.primary_authority[0] ^= 1;
    let results = exchange(
        root,
        &store,
        custody,
        vec![request.clone(), changed, changed_did, changed_authority],
        "tenant-alice",
        0,
    );
    assert_eq!(checked(results[0].clone()), first);
    for result in &results[1..] {
        assert_eq!(
            *result,
            Err(AgentFailure::Refused(
                "agent refused or corrupted creation operation"
            ))
        );
    }
    assert_eq!(record(&store), converted);
    assert_eq!(
        snapshot(&path),
        converted_snapshot,
        "restart replay must be byte-identical"
    );
    assert_eq!(installed(&store), before_capability);
    grant_discovered(&store);
    assert!(!checked(has_completed_capability_install(
        &store,
        &checked(TenantId::new("tenant-bob")),
        CapabilityId(request.capability_id)
    )));
    // The same action in a different tenant is independently pending. It
    // must not replay Alice's completed result or modify either capability.
    let mut pending = before_record[..34].to_vec();
    pending[33] = 0;
    checked(
        checked(store.lock()).put_local(action_key(checked(TenantId::new("tenant-bob"))), pending),
    );
    let before_cross_tenant = snapshot(&path);
    assert_eq!(
        exchange(
            root,
            &store,
            custody,
            vec![request.clone()],
            "tenant-bob",
            0
        )
        .remove(0),
        Err(AgentFailure::Unavailable)
    );
    assert_eq!(snapshot(&path), before_cross_tenant);
    assert_eq!(installed(&store), before_capability);
    drop(store);
}

fn assert_fresh_replay(
    root: &Path,
    custody: &CustodySigner,
    request: &CapabilityProvision,
    expected: &AgentEvidence,
) -> Vec<u8> {
    // The current producer must emit the same canonical evidence on first use,
    // and retain exactly those bytes through a process/store restart.
    let path = root.join("fresh");
    let store = Arc::new(Mutex::new(checked(Store::open(&path))));
    assert_eq!(
        checked(
            exchange(
                root,
                &store,
                custody,
                vec![request.clone()],
                "tenant-alice",
                2
            )
            .remove(0)
        ),
        *expected
    );
    let fresh = record(&store);
    assert_eq!(fresh[0], 2);
    let fresh_snapshot = snapshot(&path);
    let fresh_capability = installed(&store);
    drop(store);
    let store = Arc::new(Mutex::new(checked(Store::open(&path))));
    let replays = exchange(
        root,
        &store,
        custody,
        vec![request.clone(), request.clone()],
        "tenant-alice",
        0,
    );
    assert_eq!(replays, vec![Ok(expected.clone()), Ok(expected.clone())]);
    assert_eq!(snapshot(&path), fresh_snapshot);
    assert_eq!(installed(&store), fresh_capability);
    grant_discovered(&store);
    drop(store);

    fresh
}

fn assert_read_only_conversion(
    root: &Path,
    custody: &CustodySigner,
    request: &CapabilityProvision,
) {
    // A replay is not returned until the converted record is durable.
    let path = root.join("read-only-conversion");
    let store = open_fixture(&path, PRE_RUNG_STORE);
    let before = snapshot(&path);
    drop(store);
    let store = Arc::new(Mutex::new(checked(Store::open_read_only(&path))));
    assert_eq!(
        exchange(
            root,
            &store,
            custody,
            vec![request.clone()],
            "tenant-alice",
            0
        )
        .remove(0),
        Err(AgentFailure::Unavailable)
    );
    assert_eq!(snapshot(&path), before);
    assert_eq!(record(&store)[0], 1);
    drop(store);
}

fn invalid_record_cases(original: &[u8], fresh: &[u8]) -> Vec<(&'static str, Vec<u8>, bool)> {
    let mut cases: Vec<(&str, Vec<u8>, bool)> = Vec::new();
    for source_record in [original, fresh] {
        for length in [0, 1, 33, 34, 66, 74, 75, 106] {
            cases.push(("truncated", source_record[..length].to_vec(), false));
        }
        let mut extra = source_record.to_vec();
        extra.push(0);
        cases.push(("trailing payload", extra, false));
        for version in [0, 3, 255] {
            let mut invalid = source_record.to_vec();
            invalid[0] = version;
            cases.push(("unknown version", invalid, false));
        }
        for rung in [0, 1, 3, 4, 5, 7, 255] {
            let mut invalid = source_record.to_vec();
            invalid[74] = rung;
            cases.push(("invalid rung", invalid, false));
        }
        for offset in [34, 66, 75, 106] {
            let mut invalid = source_record.to_vec();
            invalid[offset] ^= 1;
            cases.push(("inconsistent digest", invalid, false));
        }
        let mut invalid = source_record.to_vec();
        invalid[66..74].fill(0);
        cases.push(("zero sequence", invalid, false));
        let mut invalid = source_record.to_vec();
        invalid[33] = 2;
        cases.push(("invalid state", invalid, false));
        let mut pending = source_record[..34].to_vec();
        pending[33] = 0;
        cases.push(("pending", pending.clone(), true));
        pending.push(0);
        cases.push(("pending with payload", pending, false));
    }
    let mut wrong_contract = original.to_vec();
    wrong_contract[74] = if original[74] == 6 { 2 } else { 6 };
    cases.push(("rung/digest contract mismatch", wrong_contract, false));
    let mut wrong_contract = fresh.to_vec();
    wrong_contract[74] = 6;
    cases.push(("legacy label in new version", wrong_contract, false));
    cases
}

fn assert_binding_refusals(
    root: &Path,
    custody: &CustodySigner,
    request: &CapabilityProvision,
    store: &Arc<Mutex<Store>>,
    path: &Path,
    original: Vec<u8>,
    capability_before: &Capability,
) {
    // Request binding is checked before conversion, including the retained
    // producer records themselves, so a failed retry cannot repair its record.
    checked(checked(store.lock()).put_local(action_key(tenant()), original.clone()));
    let before = snapshot(path);
    let mut changed = request.clone();
    changed.amount_ceiling -= 1;
    assert_eq!(
        exchange(root, store, custody, vec![changed], "tenant-alice", 0).remove(0),
        Err(AgentFailure::Refused(
            "agent refused or corrupted creation operation"
        ))
    );
    assert_eq!(snapshot(path), before);
    let mut wrong_binding = original.clone();
    wrong_binding[1] ^= 1;
    checked(checked(store.lock()).put_local(action_key(tenant()), wrong_binding));
    let before = snapshot(path);
    assert_eq!(
        exchange(
            root,
            store,
            custody,
            vec![request.clone()],
            "tenant-alice",
            0
        )
        .remove(0),
        Err(AgentFailure::Refused(
            "agent refused or corrupted creation operation"
        ))
    );
    assert_eq!(snapshot(path), before);
    checked(checked(store.lock()).put_core_cache(action_key(tenant()), original));
    let before = snapshot(path);
    assert_eq!(
        checked(store.lock())
            .get(&action_key(tenant()))
            .or_fail("action")
            .class(),
        StorageClass::CoreProducedCache
    );
    assert_eq!(
        exchange(
            root,
            store,
            custody,
            vec![request.clone()],
            "tenant-alice",
            0
        )
        .remove(0),
        Err(AgentFailure::Refused(
            "agent refused or corrupted creation operation"
        ))
    );
    assert_eq!(snapshot(path), before);
    assert_eq!(&installed(store), capability_before);
    assert!(has_completed_capability_install(
        store,
        &tenant(),
        CapabilityId(request.capability_id)
    )
    .is_err());
}

fn assert_invalid_records(
    root: &Path,
    custody: &CustodySigner,
    request: &CapabilityProvision,
    fresh: &[u8],
    source: &str,
    bytes: &[u8],
) {
    let path = root.join(format!("invalid-{source}"));
    let store = open_fixture(&path, bytes);
    let original = record(&store);
    let capability_before = installed(&store);
    let cases = invalid_record_cases(&original, fresh);
    for (name, invalid, pending) in cases {
        checked(checked(store.lock()).put_local(action_key(tenant()), invalid.clone()));
        let before = snapshot(&path);
        let result = exchange(
            root,
            &store,
            custody,
            vec![request.clone()],
            "tenant-alice",
            0,
        )
        .remove(0);
        assert_eq!(
            result,
            if pending {
                Err(AgentFailure::Unavailable)
            } else {
                Err(AgentFailure::Refused(
                    "agent refused or corrupted creation operation",
                ))
            },
            "{source}: {name}"
        );
        assert_eq!(snapshot(&path), before, "{source}: {name} must not rewrite");
        assert_eq!(record(&store), invalid);
        assert_eq!(installed(&store), capability_before);
        let discovery = has_completed_capability_install(
            &store,
            &tenant(),
            CapabilityId(request.capability_id),
        );
        if pending {
            assert!(!checked(discovery));
        } else {
            assert!(discovery.is_err(), "{name}");
        }
    }
    assert_binding_refusals(
        root,
        custody,
        request,
        &store,
        &path,
        original,
        &capability_before,
    );
    drop(store);
}

#[test]
fn retained_capability_install_replays_survive_upgrade() {
    let root = support::directory("retained-capability-install");
    checked(std::fs::create_dir_all(&root));
    let (principals, _) = support::install_and_open(
        &root.join("principals"),
        &support::tenancy(&[("alice", "tenant-alice")]),
        support::retention_uniform(100),
    );
    let secret = root.join("kms-root");
    checked(std::fs::write(&secret, [0x51; 64]));
    let custody = CustodySigner::new_shared(
        checked(Keystore::open_development(
            root.join("custody"),
            77,
            checked(EnvelopeKms::new("kms://creation-primary", secret)),
        )),
        Arc::new(Mutex::new(principals)),
        registry(),
        checked(SigningLimits::new(10, 60)),
    );
    let request = capability();
    let mut expected = AgentEvidence {
        action_key: request.action_key,
        object_id: request.capability_id,
        observed_sequence: HEIGHT,
        rung: Rung::Committed,
        receipt_digest: [0; 32],
    };
    expected.receipt_digest = expected.expected_digest(CreationStage::CapabilityNarrowing);

    for (pre_rung, producer, fixture) in [
        (true, PRE_RUNG_PRODUCER, PRE_RUNG_STORE),
        (false, POST_RUNG_PRODUCER, POST_RUNG_STORE),
    ] {
        assert_historical_producer(&root, pre_rung, producer, fixture);
        for schema in [3_u32, 4_u32] {
            assert_replay_schema(
                &root, &custody, &request, &expected, pre_rung, schema, fixture,
            );
        }
    }

    let fresh = assert_fresh_replay(&root, &custody, &request, &expected);
    assert_read_only_conversion(&root, &custody, &request);
    for (source, bytes) in [("pre", PRE_RUNG_STORE), ("post", POST_RUNG_STORE)] {
        assert_invalid_records(&root, &custody, &request, &fresh, source, bytes);
    }
    drop(custody);
    checked(std::fs::remove_dir_all(root));
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
