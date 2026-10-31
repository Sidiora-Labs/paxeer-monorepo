use std::error::Error as StdError;
use std::fs::{self, DirBuilder};
use std::io::{Read, Write};
use std::net::TcpListener;
use std::os::unix::fs::{DirBuilderExt, MetadataExt, PermissionsExt};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::Arc;
use std::thread;
use std::time::Duration;

use layerx_human_service::custody::RemoteKmsProvider;
use layerx_human_service::server::movement_provider::{
    MovementProviderCodec, MovementProviderConfig, MovementProviderRequest as Request,
    MovementProviderResponse as Response, NativeMovementCodec, PlanningContext, PlanningRequest,
    UnixMovementProvider,
};
use layerx_human_service::store::{AgentTenantId, PrincipalId};
use layerx_human_service::trace::TraceId;
use layerx_paxeer_client::{
    raw_call, DepositProofConfig, EndpointConfig, EndpointTransport, FinalityTracker,
    TrackerConfig, TransactionHash,
};
use layerx_types::intent::EvmAddress;
use sha2::{Digest, Sha256};

use crate::config::{hex_string, Config, MAX_FRAME};
use crate::journal::{read_private, Journal};
use crate::listener::{Listener, ListenerConfig};
use crate::service::EvidenceService;
use crate::Error;

#[allow(dead_code)]
#[path = "../tests/kms/mod.rs"]
mod kms;

type Result<T = ()> = std::result::Result<T, Box<dyn StdError>>;
fn checked<T, E: std::fmt::Debug>(value: std::result::Result<T, E>) -> Result<T> {
    value.map_err(|error| format!("{error:?}").into())
}

pub(crate) struct Directory(PathBuf);
impl Directory {
    pub(crate) fn new() -> Result<Self> {
        let mut nonce = [0; 8];
        checked(getrandom::fill(&mut nonce))?;
        let root = std::env::temp_dir().join("hm5");
        fs::create_dir_all(&root)?;
        let path = root.join(format!(
            "test-{}-{}",
            std::process::id(),
            hex_string(&nonce)
        ));
        DirBuilder::new().mode(0o700).create(&path)?;
        Ok(Self(path))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn random_seed() -> Result<[u8; 32]> {
    let mut bytes = [0; 32];
    checked(getrandom::fill(&mut bytes))?;
    Ok(bytes)
}
fn listener_config(dir: &Directory) -> ListenerConfig {
    ListenerConfig {
        socket: dir.0.join("movement.sock"),
        allowed_uid: rustix::process::geteuid().as_raw(),
        allowed_gid: rustix::process::getegid().as_raw(),
        maximum_frame_bytes: MAX_FRAME,
        deadline: Duration::from_millis(100),
        protocol: 2,
    }
}
fn unreachable_endpoint() -> Result<EndpointConfig> {
    let closed = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = EndpointConfig {
        url: format!("http://{}", closed.local_addr()?),
        request_timeout: Duration::from_millis(50),
        transport: EndpointTransport::LocalEmulator,
        expected_chain_id: 31337,
    };
    drop(closed);
    Ok(endpoint)
}
pub(crate) fn config(dir: &Directory) -> Result<Config> {
    config_for(dir, unreachable_endpoint()?, None)
}
fn config_for(
    dir: &Directory,
    endpoint: EndpointConfig,
    executor: Option<Arc<RemoteKmsProvider>>,
) -> Result<Config> {
    let evidence_root = dir.0.join("evidence");
    DirBuilder::new().mode(0o700).create(&evidence_root)?;
    Ok(Config {
        listener: listener_config(dir),
        state_root: dir.0.join("state"),
        evidence_root,
        custody_profile: None,
        tracker: TrackerConfig {
            endpoints: vec![endpoint.clone()],
            minimum_endpoint_agreement: 1,
            required_confirmations: 2,
            poll_cadence: Duration::from_secs(1),
            delayed_after_polls: 2,
        },
        proof: DepositProofConfig {
            endpoints: vec![endpoint],
            minimum_endpoint_agreement: 1,
            required_confirmations: 2,
            paxeer_chain_id: 31337,
            paxeer_checkpoint_authority: ed25519_dalek::SigningKey::from_bytes(&random_seed()?)
                .verifying_key()
                .to_bytes(),
            custody_reference: [9; 32],
            layerx_network_id: 77,
            layerx_protocol_version: 2,
        },
        checkpoint_registry: EvmAddress::new([12; 20]),
        executor,
        checkpoint_interval_seconds: 10,
        paxeer_block_seconds: 1,
        reminder_interval_seconds: 60,
    })
}
fn client(config: &Config) -> Result<UnixMovementProvider> {
    checked(UnixMovementProvider::new(
        MovementProviderConfig {
            socket: config.listener.socket.clone(),
            peer_uid: rustix::process::geteuid().as_raw(),
            peer_gid: rustix::process::getegid().as_raw(),
            maximum_frame_bytes: MAX_FRAME,
            deadline: Duration::from_secs(2),
        },
        Arc::new(NativeMovementCodec::new()),
    ))
}
fn plan(operation: &str) -> Result<PlanningRequest> {
    let money = serde_json::json!({"amount":"10", "currency":"LXP"});
    let body = match operation {
        "move.quote" => {
            serde_json::json!({"source":"account", "destination":"managed-agent", "money":money})
        }
        _ => return Err("unsupported test operation".into()),
    };
    checked(PlanningRequest::from_wire_parts(PlanningRequest {
        principal: checked(PrincipalId::new("alice"))?,
        tenant: checked(AgentTenantId::new("tenant-alice"))?,
        context: untrusted_context()?,
        operation: operation.to_owned(),
        idempotency_key: [3; 32],
        canonical_body: serde_json::to_vec(&body)?,
        trace: checked(TraceId::parse("trc_0123456789abcdef0123456789abcdef"))?,
        now: 100,
    }))
}

fn untrusted_context() -> Result<PlanningContext> {
    use layerx_agent_api::identity::{AgentDid, AuthorityRef};
    use layerx_human_service::custody::KeyId;
    use layerx_types::{account::AccountId, amount::Amount, ids::AssetId};
    Ok(PlanningContext {
        request_anchor: [18; 32],
        account: checked(AccountId::parse("agent:did:layerx:alice:main"))?,
        reserve: checked(AccountId::parse("system:treasury"))?,
        withdrawals_account: checked(AccountId::parse("system:paxeer-withdrawals"))?,
        route: None,
        amount: Amount::from_u128(10),
        asset: AssetId::new([1; 32]),
        currency: "LXP".to_owned(),
        actor: checked(AgentDid::new("did:layerx:alice"))?,
        authority: checked(AuthorityRef::new("primary"))?,
        custody_key: checked(KeyId::new("human-primary"))?,
        custody_provider_reference: vec![1],
        custody_binding_digest: [2; 32],
        wallet: EvmAddress::new([3; 20]),
        network: checked(layerx_types::intent::NetworkId::new(77))?,
        protocol_version: 2,
        paxeer_chain_id: 31337,
        account_sequence: 1,
        budget_grant: None,
        fee_limit: 10,
        evm_gas_limit: 100_000,
        evm_max_fee_per_gas: 100,
        evm_max_priority_fee_per_gas: 1,
        not_before: 100,
        not_after: 200,
        binding_receipt_digest: [4; 32],
        identity_authority_evidence: vec![0],
        balance_evidence: vec![0],
    })
}

#[test]
fn real_client_refuses_unavailable_plans_and_reads_actual_endpoint_failure() -> Result {
    let dir = Directory::new()?;
    let config = config(&dir)?;
    let journal = Journal::open(&config.state_root, 2)?;
    let mut service = EvidenceService::new(&config, journal)?;
    let listener = Listener::bind(listener_config(&dir))?;
    let mut client = client(&config)?;
    let server = thread::spawn(move || {
        for _ in 0..2 {
            listener.serve_next(&mut service)?;
        }
        Ok::<_, Error>(())
    });
    assert!(!client.ready());
    assert!(client.move_plan(plan("move.quote")?).is_err());
    server.join().map_err(|_| "server panicked")??;
    let journal = Journal::open(&config.state_root, 2)?;
    let bytes = checked(NativeMovementCodec::new().encode_request(&Request::Readiness))?;
    let key = hex_string(&Sha256::digest(bytes));
    let response = journal
        .record(&key)
        .ok_or("missing readiness record")?
        .response
        .as_ref()
        .ok_or("missing response")?;
    assert_eq!(
        checked(NativeMovementCodec::new().decode_response(response))?,
        Response::Unavailable
    );
    Ok(())
}

#[test]
fn wrong_uid_and_oversized_frames_are_refused_before_payload_allocation() -> Result {
    for wrong_uid in [true, false] {
        let dir = Directory::new()?;
        let config = config(&dir)?;
        let mut service = EvidenceService::new(&config, Journal::open(&config.state_root, 2)?)?;
        let mut policy = listener_config(&dir);
        if wrong_uid {
            policy.allowed_uid ^= 1;
        }
        let socket = policy.socket.clone();
        let listener = Listener::bind(policy)?;
        let server = thread::spawn(move || listener.serve_next(&mut service));
        let mut stream = UnixStream::connect(socket)?;
        stream.set_read_timeout(Some(Duration::from_secs(2)))?;
        let mut header = layerx_human_service::server::movement_provider::MOVEMENT_PROTOCOL_VERSION
            .to_be_bytes()
            .to_vec();
        header.extend(if wrong_uid { 2_u64 } else { u64::MAX }.to_be_bytes());
        let _ = stream.write_all(&header);
        let _ = stream.write_all(&[2, 14]);
        let mut reply = [0; 1];
        assert!(!matches!(stream.read(&mut reply), Ok(1)));
        server.join().map_err(|_| "server panicked")??;
    }
    Ok(())
}

#[test]
fn partial_header_deadline_releases_the_listener() -> Result {
    let dir = Directory::new()?;
    let config = config(&dir)?;
    let mut service = EvidenceService::new(&config, Journal::open(&config.state_root, 2)?)?;
    let listener = Listener::bind(listener_config(&dir))?;
    let client = client(&config)?;
    let server = thread::spawn(move || {
        listener.serve_next(&mut service)?;
        listener.serve_next(&mut service)
    });
    let mut stalled = UnixStream::connect(&config.listener.socket)?;
    stalled.write_all(&[0])?;
    stalled.set_read_timeout(Some(Duration::from_secs(2)))?;
    let mut byte = [0];
    assert!(!matches!(stalled.read(&mut byte), Ok(1)));
    assert!(!client.ready());
    server.join().map_err(|_| "server panicked")??;
    Ok(())
}

#[test]
fn listener_does_not_replace_live_socket_or_follow_symlinks() -> Result {
    let dir = Directory::new()?;
    let listener = Listener::bind(listener_config(&dir))?;
    assert!(matches!(
        Listener::bind(listener_config(&dir)),
        Err(Error::Conflict)
    ));
    assert_eq!(
        fs::metadata(dir.0.join("movement.sock"))?.mode() & 0o777,
        0o660
    );
    drop(listener);
    let target = dir.0.join("unrelated");
    fs::write(&target, b"preserve")?;
    std::os::unix::fs::symlink(&target, dir.0.join("movement.sock"))?;
    assert!(Listener::bind(listener_config(&dir)).is_err());
    assert_eq!(fs::read(target)?, b"preserve");
    Ok(())
}

#[test]
fn journal_replays_pending_actions_and_refuses_conflicts_corruption_and_second_writer() -> Result {
    let dir = Directory::new()?;
    let root = dir.0.join("state");
    let key = "a".repeat(64);
    let codec = NativeMovementCodec::new();
    let request = checked(codec.encode_request(&Request::Readiness))?;
    let other = checked(codec.encode_request(&Request::PlanMove(plan("move.quote")?)))?;
    let mut journal = Journal::open(&root, 2)?;
    journal.begin(&key, &request)?;
    assert!(matches!(Journal::open(&root, 2), Err(Error::Conflict)));
    assert!(matches!(journal.begin(&key, &other), Err(Error::Conflict)));
    drop(journal);
    let mut journal = Journal::open(&root, 2)?;
    assert_eq!(journal.record(&key).ok_or("record")?.request, request);
    assert!(journal.record(&key).ok_or("record")?.response.is_none());
    journal.complete(
        &key,
        &checked(codec.encode_response(&Response::Unavailable))?,
    )?;
    drop(journal);
    assert!(Journal::open(&root, 3).is_err());
    let journal = Journal::open(&root, 2)?;
    assert!(journal.record(&key).ok_or("record")?.response.is_some());
    assert_eq!(
        fs::metadata(root.join("journal.bin"))?.mode() & 0o777,
        0o600
    );
    drop(journal);
    let mut bytes = fs::read(root.join("journal.bin"))?;
    let last = bytes.last_mut().ok_or("journal empty")?;
    *last ^= 1;
    fs::write(root.join("journal.bin"), bytes)?;
    assert!(Journal::open(&root, 2).is_err());
    Ok(())
}

#[test]
fn private_files_refuse_symlinks_hardlinks_and_public_permissions() -> Result {
    let dir = Directory::new()?;
    let path = dir.0.join("proof.bin");
    fs::write(&path, b"not proof material")?;
    fs::set_permissions(&path, fs::Permissions::from_mode(0o600))?;
    assert!(read_private(&path, 1).is_err());
    let link = dir.0.join("link.bin");
    std::os::unix::fs::symlink(&path, &link)?;
    assert!(read_private(&link, MAX_FRAME).is_err());
    fs::remove_file(&link)?;
    fs::hard_link(&path, &link)?;
    assert!(read_private(&path, MAX_FRAME).is_err());
    fs::remove_file(link)?;
    fs::set_permissions(&path, fs::Permissions::from_mode(0o644))?;
    assert!(read_private(&path, MAX_FRAME).is_err());
    Ok(())
}

#[test]
fn published_evidence_is_private_complete_and_never_overwrites_an_existing_path() -> Result {
    let dir = Directory::new()?;
    let codec = NativeMovementCodec::new();
    let original = checked(codec.encode_response(&Response::Unavailable))?;
    let changed = checked(codec.encode_response(&Response::ContractViolation))?;
    let path = dir.0.join("evidence.bin");
    assert!(crate::journal::publish_private(&path, &original)?);
    assert_eq!(read_private(&path, MAX_FRAME)?, original);
    let metadata = fs::symlink_metadata(&path)?;
    assert_eq!(metadata.mode() & 0o777, 0o600);
    assert_eq!(metadata.nlink(), 1);
    assert!(!crate::journal::publish_private(&path, &changed)?);
    assert_eq!(read_private(&path, MAX_FRAME)?, original);
    assert!(crate::journal::publish_private(&dir.0.join("empty.bin"), &[]).is_err());
    assert!(
        crate::journal::publish_private(&dir.0.join("large.bin"), &vec![0; MAX_FRAME + 1]).is_err()
    );
    let linked = dir.0.join("linked.bin");
    std::os::unix::fs::symlink(&path, &linked)?;
    assert!(!crate::journal::publish_private(&linked, &changed)?);
    assert!(read_private(&linked, MAX_FRAME).is_err());
    assert_eq!(read_private(&path, MAX_FRAME)?, original);
    assert!(fs::read_dir(&dir.0)?.all(|entry| entry.is_ok_and(|entry| {
        !entry
            .file_name()
            .to_string_lossy()
            .starts_with("proof-pending-")
    })));
    Ok(())
}

#[test]
fn tracker_configuration_refuses_insecure_production_and_duplicate_endpoints() -> Result {
    let dir = Directory::new()?;
    let mut config = config(&dir)?;
    config
        .tracker
        .endpoints
        .push(config.tracker.endpoints[0].clone());
    config.tracker.minimum_endpoint_agreement = 2;
    assert!(FinalityTracker::new(config.tracker.clone(), TransactionHash::new([1; 32])).is_err());
    config.tracker.endpoints.truncate(1);
    config.tracker.endpoints[0].url = "http://paxeer.example.invalid".to_owned();
    assert!(FinalityTracker::new(config.tracker, TransactionHash::new([1; 32])).is_err());
    Ok(())
}

#[test]
fn stale_owned_socket_recovers_but_unmanaged_live_socket_is_preserved() -> Result {
    let dir = Directory::new()?;
    let socket = dir.0.join("movement.sock");
    let live = std::os::unix::net::UnixListener::bind(&socket)?;
    assert!(Listener::bind(listener_config(&dir)).is_err());
    assert!(socket.exists());
    drop(live);
    let recovered = Listener::bind(listener_config(&dir))?;
    assert!(socket.exists());
    drop(recovered);
    assert!(!socket.exists());
    Ok(())
}

#[test]
fn drip_fed_payload_cannot_extend_absolute_connection_deadline() -> Result {
    let dir = Directory::new()?;
    let config = config(&dir)?;
    let mut service = EvidenceService::new(&config, Journal::open(&config.state_root, 2)?)?;
    let listener = Listener::bind(listener_config(&dir))?;
    let server = thread::spawn(move || listener.serve_next(&mut service));
    let mut stream = UnixStream::connect(&config.listener.socket)?;
    stream.write_all(&1_u16.to_be_bytes())?;
    stream.write_all(&(MAX_FRAME as u64).to_be_bytes())?;
    let mut closed = false;
    for _ in 0..40 {
        if stream.write_all(&[1]).is_err() {
            closed = true;
            break;
        }
        thread::sleep(Duration::from_millis(20));
    }
    drop(stream);
    server.join().map_err(|_| "server panicked")??;
    assert!(
        closed,
        "payload trickle extended connection beyond its absolute deadline"
    );
    Ok(())
}

fn probe_settings(config: &Config) -> crate::probe::Settings {
    crate::probe::Settings {
        socket: config.listener.socket.clone(),
        deadline: Duration::from_secs(2),
        maximum_frame_bytes: MAX_FRAME,
        protocol: config.listener.protocol,
    }
}

#[test]
fn probe_reaches_the_real_listener_and_reports_the_provider_answer() -> Result {
    let dir = Directory::new()?;
    let config = config(&dir)?;
    let settings = probe_settings(&config);
    assert!(crate::probe::probe(&settings).is_err());
    let mut service = EvidenceService::new(&config, Journal::open(&config.state_root, 2)?)?;
    let listener = Listener::bind(listener_config(&dir))?;
    let server = thread::spawn(move || listener.serve_next(&mut service));
    assert_eq!(
        checked(crate::probe::probe(&settings))?,
        crate::probe::Outcome::NotReady
    );
    server.join().map_err(|_| "server panicked")??;
    assert!(crate::probe::probe(&settings).is_err());
    Ok(())
}

#[test]
fn probe_accepts_only_the_provider_ready_answer() -> Result {
    let codec = NativeMovementCodec::new();
    let ready = checked(codec.encode_response(&Response::Ready))?;
    assert_eq!(
        checked(crate::probe::interpret(2, &ready))?,
        crate::probe::Outcome::Ready
    );
    let unavailable = checked(codec.encode_response(&Response::Unavailable))?;
    assert_eq!(
        checked(crate::probe::interpret(2, &unavailable))?,
        crate::probe::Outcome::NotReady
    );
    assert!(crate::probe::interpret(2, b"not a movement frame").is_err());
    Ok(())
}

fn anvil_binary() -> PathBuf {
    let foundry = PathBuf::from("/root/.foundry/bin/anvil");
    if foundry.exists() {
        foundry
    } else {
        PathBuf::from("anvil")
    }
}

/// A real local Paxeer-compatible JSON-RPC origin, started for the readiness
/// tests and stopped again so readiness can be observed on both sides of the
/// origin going away.
struct Anvil {
    child: Child,
    endpoint: EndpointConfig,
}

impl Anvil {
    fn launch() -> Result<Self> {
        for _ in 0..8 {
            let reserved = TcpListener::bind("127.0.0.1:0")?;
            let port = reserved.local_addr()?.port();
            drop(reserved);
            let child = Command::new(anvil_binary())
                .arg("--port")
                .arg(port.to_string())
                .arg("--silent")
                .stdin(Stdio::null())
                .stdout(Stdio::null())
                .stderr(Stdio::null())
                .spawn()?;
            let mut anvil = Self {
                child,
                endpoint: EndpointConfig {
                    url: format!("http://127.0.0.1:{port}"),
                    request_timeout: Duration::from_secs(2),
                    transport: EndpointTransport::LocalEmulator,
                    expected_chain_id: 31337,
                },
            };
            if anvil.answers() {
                return Ok(anvil);
            }
            anvil.halt();
        }
        Err("no local paxeer emulator became reachable".into())
    }

    fn answers(&self) -> bool {
        for _ in 0..100 {
            if raw_call(&self.endpoint, "eth_chainId", &[]).is_ok() {
                return true;
            }
            thread::sleep(Duration::from_millis(50));
        }
        false
    }

    fn halt(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
        for _ in 0..100 {
            if raw_call(&self.endpoint, "eth_chainId", &[]).is_err() {
                return;
            }
            thread::sleep(Duration::from_millis(50));
        }
    }
}

impl Drop for Anvil {
    fn drop(&mut self) {
        self.halt();
    }
}

/// Starts a real Human KMS process and builds the same `RemoteKmsProvider`
/// the movement mode builds from its environment, presenting the restricted
/// executor identity that KMS pins.
fn execution_authority(dir: &Directory) -> Result<(kms::Kms, Arc<RemoteKmsProvider>)> {
    let exe = std::env::current_exe()?;
    let profile = exe
        .parent()
        .and_then(Path::parent)
        .ok_or("the test executable has no profile directory")?;
    let kms = kms::Kms::start(
        &dir.0.join("kms"),
        &kms::beside(&profile.join("layerx-human-movement-provider"))?,
    )?;
    let executor = Arc::new(kms.remote("executor", kms::PROVIDER, "localhost", "ca.der")?);
    Ok((kms, executor))
}

fn readiness_listener(dir: &Directory) -> ListenerConfig {
    let mut policy = listener_config(dir);
    policy.deadline = Duration::from_secs(10);
    policy
}

#[test]
fn readiness_is_ready_while_the_real_paxeer_origin_and_execution_authority_answer() -> Result {
    let dir = Directory::new()?;
    let paxeer = Anvil::launch()?;
    let (_kms, executor) = execution_authority(&dir)?;
    let mut config = config_for(&dir, paxeer.endpoint.clone(), Some(executor))?;
    config.listener.deadline = Duration::from_secs(5);
    let mut service = EvidenceService::new(&config, Journal::open(&config.state_root, 2)?)?;
    let listener = Listener::bind(readiness_listener(&dir))?;
    let client = client(&config)?;
    let server = thread::spawn(move || listener.serve_next(&mut service));
    assert!(client.ready());
    server.join().map_err(|_| "server panicked")??;
    let journal = Journal::open(&config.state_root, 2)?;
    let key = hex_string(&Sha256::digest(checked(
        NativeMovementCodec::new().encode_request(&Request::Readiness),
    )?));
    let recorded = journal
        .record(&key)
        .ok_or("missing readiness record")?
        .response
        .as_ref()
        .ok_or("missing readiness response")?;
    assert_eq!(
        checked(NativeMovementCodec::new().decode_response(recorded))?,
        Response::Ready
    );
    Ok(())
}

#[test]
fn readiness_stops_being_ready_once_the_paxeer_origin_is_down() -> Result {
    let dir = Directory::new()?;
    let mut paxeer = Anvil::launch()?;
    let (_kms, executor) = execution_authority(&dir)?;
    let mut config = config_for(&dir, paxeer.endpoint.clone(), Some(executor))?;
    config.listener.deadline = Duration::from_secs(5);
    let mut service = EvidenceService::new(&config, Journal::open(&config.state_root, 2)?)?;
    let listener = Listener::bind(readiness_listener(&dir))?;
    let client = client(&config)?;
    let server = thread::spawn(move || {
        listener.serve_next(&mut service)?;
        listener.serve_next(&mut service)
    });
    assert!(client.ready());
    paxeer.halt();
    assert!(!client.ready());
    server.join().map_err(|_| "server panicked")??;
    Ok(())
}
