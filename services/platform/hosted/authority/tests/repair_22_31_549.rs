include!("../src/main.rs");
use layerx_platform_authority::assertion::Must;
include!("support/committed_human_fixture.rs");

#[path = "../../runtime-clock/src/server.rs"]
mod runtime_clock_server;

use layerx_types::verify::VerificationLevel;
use std::os::unix::fs::MetadataExt as _;
use std::os::unix::net::UnixListener;

fn assert_committed_label(config: Arc<Config>, path: &str) -> Value {
    let (status, document) = tls_request(config, path, "repair-human-token-208");
    assert_eq!(status, 200, "{path}: {document}");
    let label = document["verification_level"]
        .as_str()
        .must_with("verification label");
    assert_eq!(label, VerificationLevel::COMMITTED.wire_label());
    assert_eq!(
        VerificationLevel::from_wire_label(label),
        Ok(VerificationLevel::COMMITTED)
    );
    document
}

#[test]
fn repair_22_31_549_policy_identity_emits_committed_label_through_tls() {
    for capability in [false, true] {
        let fixture = Arc::new(identity_fixture(capability));
        let node = Node::start(Arc::clone(&fixture));
        let directory = Directory::new();
        let config = config_fixture(&node, &fixture, &directory.0);
        retain_records(&config, &fixture.records);
        drop(config);
        let config = Arc::new(config_fixture(&node, &fixture, &directory.0));
        let document = assert_committed_label(
            config,
            &format!("{}&did={}", human_path("identity"), fixture.did),
        );
        assert_eq!(document["head_sequence"], fixture.sequence);
        assert!(!document["authorities"]
            .as_array()
            .must_with("native authorities")
            .is_empty());
        assert_eq!(
            document["finality"]["app_hash"],
            hex::encode(&fixture.store.app_hash)
        );
    }
}

struct SubjectBindingPeer {
    stop: Arc<AtomicBool>,
    worker: Option<thread::JoinHandle<()>>,
    clock_worker: Option<thread::JoinHandle<()>>,
    client: layerx_identity_binding::Client,
}

impl SubjectBindingPeer {
    fn start(root: &Path, did: &str) -> Self {
        let metadata = fixture_must(fs::metadata(root));
        let mut clock_server = fixture_must(runtime_clock_server::Server::bind(root));
        let clock_socket = clock_server.socket().to_owned();
        let stop = Arc::new(AtomicBool::new(false));
        let clock_stop = Arc::clone(&stop);
        let clock_worker = thread::spawn(move || {
            while !clock_stop.load(Ordering::Acquire) {
                fixture_must(clock_server.poll());
                thread::sleep(Duration::from_millis(1));
            }
        });
        let clock = fixture_must(layerx_client::runtime_clock::RuntimeClock::connect(
            layerx_client::runtime_clock::ClockBinding {
                socket: clock_socket,
                process: std::process::id(),
                user: metadata.uid(),
            },
        ));
        let socket = root.join("binding.sock");
        let listener = fixture_must(UnixListener::bind(&socket));
        fixture_must(fs::set_permissions(
            &socket,
            fs::Permissions::from_mode(0o600),
        ));
        fixture_must(listener.set_nonblocking(true));
        let client = fixture_must(layerx_identity_binding::Client::new(
            layerx_identity_binding::Config {
                socket,
                tenant: "repair-tenant".to_owned(),
                peer_uid: metadata.uid(),
                peer_gid: metadata.gid(),
                deadline: Duration::from_secs(3),
            },
            clock,
        ));
        let binding_stop = Arc::clone(&stop);
        let did = did.to_owned();
        let worker = thread::spawn(move || {
            while !binding_stop.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(peer) => peer,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("binding peer: {error}"),
                };
                fixture_must(stream.set_read_timeout(Some(Duration::from_secs(3))));
                fixture_must(stream.set_write_timeout(Some(Duration::from_secs(3))));
                let mut length = [0; 4];
                fixture_must(stream.read_exact(&mut length));
                let length = u32::from_be_bytes(length) as usize;
                assert!((6..=2048).contains(&length));
                let mut frame = vec![0; length];
                fixture_must(stream.read_exact(&mut frame));
                assert_eq!(&frame[..5], b"LXIB\x01");
                let request: Value = fixture_must(serde_json::from_slice(&frame[5..]));
                assert_eq!(
                    request,
                    document!({"operation":"principal","tenant":"repair-tenant","principal":"repair-principal"})
                );
                let mut frame = b"LXIB\x01".to_vec();
                frame.extend(fixture_must(serde_json::to_vec(&document!({"status":"bound","tenant":"repair-tenant","principal":"repair-principal","did":did}))));
                fixture_must(stream.write_all(&(u32::try_from(frame.len()).must()).to_be_bytes()));
                fixture_must(stream.write_all(&frame));
            }
        });
        Self {
            stop,
            worker: Some(worker),
            clock_worker: Some(clock_worker),
            client,
        }
    }
}

impl Drop for SubjectBindingPeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        fixture_must(self.worker.take().must_with("binding worker").join());
        fixture_must(self.clock_worker.take().must_with("clock worker").join());
    }
}

fn dynamic_identity_fixture() -> Fixture {
    let mut fixture = identity_fixture(false);
    let (owner, owner_value) = account(
        &format!("agent:{}:main", fixture.did),
        1,
        fixture.asset,
        500,
    );
    let mut rows: Vec<_> = fixture
        .store
        .queries
        .values()
        .map(|row| {
            (
                fixture_must(committed::base64(row["key"].as_str().must())),
                fixture_must(committed::base64(row["value"].as_str().must())),
            )
        })
        .collect();
    rows.push((vec![2], fixture.sequence.to_be_bytes().to_vec()));
    rows.push(([vec![6], owner.to_vec()].concat(), owner_value));
    fixture.owner = owner;
    fixture.store = NativeStore::commit(rows);
    fixture
}

#[test]
fn repair_22_31_549_dynamic_identity_and_context_emit_committed_label_through_tls() {
    let fixture = Arc::new(dynamic_identity_fixture());
    let node = Node::start(Arc::clone(&fixture));
    let directory = Directory::new();
    let binding = SubjectBindingPeer::start(&directory.0, &fixture.did);
    let mut config = config_fixture(&node, &fixture, &directory.0);
    config.fixture_subject_binding(binding.client.clone());
    retain_records(&config, &fixture.records);
    drop(config);
    let mut config = config_fixture(&node, &fixture, &directory.0);
    config.fixture_subject_binding(binding.client.clone());
    let config = Arc::new(config);
    let subject = format!(
        "&subject_principal=repair-principal&owner_did={}&owner_account=agent:{}:main&asset_id={}",
        fixture.did,
        fixture.did,
        hex::encode(&fixture.asset)
    );
    let context = assert_committed_label(
        Arc::clone(&config),
        &format!("{}{subject}", human_path("subject-context")),
    );
    let identity = assert_committed_label(
        config,
        &format!("{}{subject}&did={}", human_path("identity"), fixture.did),
    );
    assert_eq!(context["owner_did"], fixture.did);
    assert_eq!(context["account_id"], hex::encode(&fixture.owner));
    assert_eq!(
        context["agent_tenant"],
        fixture_must(layerx_identity_binding::subject_namespace(
            "repair-tenant",
            "repair-principal"
        ))
    );
    assert_eq!(context["observed_head_sequence"], fixture.sequence);
    assert_eq!(identity["head_sequence"], fixture.sequence);
    assert_eq!(context["committed_height"], identity["committed_height"]);
    assert_eq!(context["state_height"], identity["state_height"]);
    assert!(!identity["authorities"]
        .as_array()
        .must_with("native authorities")
        .is_empty());
}
