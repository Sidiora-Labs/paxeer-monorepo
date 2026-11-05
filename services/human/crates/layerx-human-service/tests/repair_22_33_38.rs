//! Task record 22.33.38, audit 22.30.103, acceptance 22998.1.
//! gateway-operation-deadline: gateway_operation_deadline_bounds_real_delayed_transport.
//! custody-read-bound: custody_read_bound_and_protected_descriptor.
//! custody-temp-recovery: custody_temp_recovery_before_link_rename_and_unlink.
//! sessions-state-restart: sessions_state_restart_recovers_install_suspend_and_revoke.
//! session-renewal-orphan: session_renewal_orphan_resumes_exact_replacement.
//! session-second-rotation: session_second_rotation_and_retry_after_lapse.
//! session-production-driver: session_production_driver_renews_and_recovers_through_maintenance.
//! Baseline not run: the explicit gate order forbids verification before implementation.
//! Wiring request (Human domain 10): src/server/production_components.rs must compose the durable SessionLifecycleDriver with its ComponentMaintenance owner, replacing the dormant non-durable helper; src/server/agent_runtime.rs and the agent daemon owner must implement AgentSessionContract with authoritative recover_provision/recover_rotation and idempotent exact-target suspend/revoke; production daemon scheduling remains outside this scope.
//! Wiring request (Human domain 10): src/bin/layerx-human-kms-gateway.rs must supply the authenticated ClientConfig through RemoteKmsProvider::with_deadline_transport; providers without that transport now refuse bounded dispatch.
//! The user forbids spec edits; this integration module is the finding-to-test task record.

use std::collections::{BTreeMap, BTreeSet};
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener};
use std::os::unix::fs::PermissionsExt;
use std::path::{Path, PathBuf};
use std::sync::Arc;
use std::time::{Duration, Instant};

use layerx_agentd::identity::{
    register, CoreIdentity, IdentityError, IdentityResolver, ProtocolAuthority,
};
use layerx_agentd::session::{
    close, invalidate_on_revocation, open, InvalidationReason, OpenRequest, RevocationEvent,
    SessionId, SessionRegistry,
};
use layerx_agentd::sign::ProvisionedSessionKey;
use layerx_agentd::store::{Store, TenantId};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::signer::Signer as _;
use layerx_human_service::custody::*;
use layerx_human_service::frame::{
    read_frame, write_frame, ConnectionGate, FrameTransport, Limits, MutualTlsConfig, Tls,
};
use layerx_human_service::server::ComponentMaintenance;
use layerx_human_service::store::PrincipalId;
use layerx_human_test_support::directory;
use layerx_intents::DisclosureCheck;
use layerx_types::clock::{Clock, ClockError, ClockReading};
use layerx_types::ids::Did;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_types::rung::Rung;
use layerx_types::verify::VerificationLevel;
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::{ClientConfig, RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

#[allow(dead_code)]
#[path = "../../layerx-human-kms/src/wire.rs"]
mod kms_wire;

type Result<T = ()> = std::result::Result<T, Box<dyn std::error::Error>>;
fn checked<T, E: std::fmt::Debug>(value: std::result::Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("repair 22.33.38: {error:?}"))
}
fn principal() -> PrincipalId {
    checked(PrincipalId::new("alice"))
}
fn did() -> Did {
    checked(Did::new(b"did:layerx:repair-session"))
}
fn activity() -> ActivityType {
    checked(ActivityType::new(ModuleId::Asset, 5))
}
fn registry() -> ModuleRegistry {
    checked(ModuleRegistry::new(&[
        checked(ModuleRegistration::new(ModuleId::Asset, &[activity()])),
        checked(ModuleRegistration::new(
            ModuleId::Governance,
            &[checked(ActivityType::new(ModuleId::Governance, 2))],
        )),
    ]))
}
fn policy() -> SessionPolicy {
    checked(SessionPolicy::new(
        100,
        20,
        3,
        vec!["prepare".into(), "submit".into()],
        "repair-v1",
    ))
}

struct Fixture(PathBuf);
impl Fixture {
    fn new(label: &str) -> Self {
        let root = directory(label);
        checked(fs::create_dir_all(&root));
        checked(fs::write(root.join("secret"), [0x55; 64]));
        Self(root)
    }
    fn custody(&self) -> Keystore {
        checked(Keystore::open_development(
            self.0.join("keys"),
            77,
            checked(EnvelopeKms::new("repair-kms", self.0.join("secret"))),
        ))
    }
    fn key_path(&self) -> PathBuf {
        self.0.join("keys/principals/alice/main.key")
    }
}
impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn custody_read_bound_and_protected_descriptor() -> Result {
    let fixture = Fixture::new("repair-custody-bound");
    let key = KeyId::new("main")?;
    let store = fixture.custody();
    store.create(&principal(), &key, KeyClass::HumanPrimary)?;
    let path = fixture.key_path();
    let original = fs::read(&path)?;
    let file = fs::OpenOptions::new().write(true).open(&path)?;
    file.set_len(1 << 34)?;
    assert!(matches!(
        store.describe(&principal(), &key),
        Err(CustodyError::CorruptRecord("key record size"))
    ));
    drop(file);
    fs::write(&path, &original)?;
    fs::set_permissions(&path, fs::Permissions::from_mode(0o644))?;
    assert!(matches!(
        store.describe(&principal(), &key),
        Err(CustodyError::CorruptRecord(_))
    ));
    fs::set_permissions(&path, fs::Permissions::from_mode(0o600))?;
    let alias = path.with_extension("alias");
    fs::hard_link(&path, &alias)?;
    assert!(matches!(
        store.describe(&principal(), &key),
        Err(CustodyError::CorruptRecord(_))
    ));
    fs::remove_file(&alias)?;
    fs::rename(&path, &alias)?;
    std::os::unix::fs::symlink(&alias, &path)?;
    assert!(matches!(
        store.describe(&principal(), &key),
        Err(CustodyError::CorruptRecord(_))
    ));
    fs::remove_file(&path)?;
    fs::rename(&alias, &path)?;
    assert!(store.describe(&principal(), &key).is_ok());
    Ok(())
}

#[test]
fn custody_temp_recovery_before_link_rename_and_unlink() -> Result {
    let fixture = Fixture::new("repair-custody-temp");
    let key = KeyId::new("main")?;
    let store = fixture.custody();
    let initial = store.create(&principal(), &key, KeyClass::HumanPrimary)?;
    let path = fixture.key_path();
    let temp = path.with_extension("key.tmp");
    fs::rename(&path, &temp)?;
    drop(store);
    let store = fixture.custody();
    assert_eq!(store.describe(&principal(), &key)?.public_key, initial);
    assert!(!temp.exists());
    let old = fs::read(&path)?;
    let next = store.rotate(&principal(), &key)?.public_key;
    fs::rename(&path, &temp)?;
    fs::write(&path, &old)?;
    fs::set_permissions(&path, fs::Permissions::from_mode(0o600))?;
    drop(store);
    let store = fixture.custody();
    assert_eq!(store.describe(&principal(), &key)?.public_key, next);
    assert!(!temp.exists());
    fs::hard_link(&path, &temp)?;
    drop(store);
    let store = fixture.custody();
    assert_eq!(store.describe(&principal(), &key)?.public_key, next);
    assert!(!temp.exists());
    // A verified, incomplete, uncommitted inode is the only partial write removed.
    fs::write(&temp, b"LXC")?;
    fs::set_permissions(&temp, fs::Permissions::from_mode(0o600))?;
    drop(store);
    let store = fixture.custody();
    assert!(!temp.exists());
    assert_eq!(store.describe(&principal(), &key)?.public_key, next);
    Ok(())
}

struct ElapsedClock(Instant, u64);
impl Clock for ElapsedClock {
    fn sample(&self, budget: Duration) -> std::result::Result<ClockReading, ClockError> {
        if budget.is_zero() {
            return Err(ClockError::Unavailable);
        }
        let nanos = u64::try_from(self.0.elapsed().as_nanos()).map_err(|_| ClockError::Overflow)?;
        Ok(ClockReading {
            generation: [1; 16],
            unix_milliseconds: self.1 + nanos / 1_000_000,
            monotonic_nanoseconds: nanos,
        })
    }
}
const CA: &[u8] = include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/ca.der");
const SERVER: &[u8] =
    include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/server.der");
const SERVER_KEY: &[u8] =
    include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/server.pk8");
const CLIENT: &[u8] =
    include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/client.der");
const CLIENT_KEY: &[u8] =
    include_bytes!("../../layerx-human-kms/tests/fixtures/repair_22_33_27/client.pk8");
fn roots() -> RootCertStore {
    let mut roots = RootCertStore::empty();
    checked(roots.add(CertificateDer::from(CA.to_vec())));
    roots
}
fn client_config() -> Arc<ClientConfig> {
    Arc::new(checked(
        ClientConfig::builder()
            .with_root_certificates(roots())
            .with_client_auth_cert(
                vec![CertificateDer::from(CLIENT.to_vec())],
                checked(PrivateKeyDer::try_from(CLIENT_KEY.to_vec())),
            ),
    ))
}
fn mutual_tls() -> MutualTlsConfig {
    checked(MutualTlsConfig::new(
        roots(),
        vec![CertificateDer::from(CLIENT.to_vec())],
        checked(PrivateKeyDer::try_from(CLIENT_KEY.to_vec())),
    ))
}
fn server_config() -> Arc<ServerConfig> {
    let verifier =
        checked(rustls::server::WebPkiClientVerifier::builder(Arc::new(roots())).build());
    Arc::new(checked(
        ServerConfig::builder()
            .with_client_cert_verifier(verifier)
            .with_single_cert(
                vec![CertificateDer::from(SERVER.to_vec())],
                checked(PrivateKeyDer::try_from(SERVER_KEY.to_vec())),
            ),
    ))
}
fn limits() -> Limits {
    Limits {
        maximum_connections: 8,
        maximum_streams: 8,
        maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
        maximum_queued_bytes: 8 * GATEWAY_FRAME_LIMIT,
        deadline: Duration::from_secs(2),
    }
}
fn remote(address: SocketAddr, clock: Arc<dyn Clock>) -> RemoteKmsProvider {
    checked(RemoteKmsProvider::new(
        "repair-deadline",
        address,
        "localhost",
        mutual_tls(),
        limits(),
        clock,
    ))
    .with_deadline_transport(client_config())
}

#[test]
fn gateway_operation_deadline_bounds_real_delayed_transport() -> Result {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let clock: Arc<dyn Clock> = Arc::new(ElapsedClock(Instant::now(), 1_793_779_200_000));
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let (arrival, arrivals) = std::sync::mpsc::channel();
    let backend = std::thread::spawn(move || {
        let mut workers = Vec::new();
        // The constructor probe succeeds; every relayed operation reaches a delayed TLS peer.
        for index in 0..8 {
            let (tcp, _) = checked(listener.accept());
            let config = server_config();
            let arrival = arrival.clone();
            workers.push(std::thread::spawn(move || {
                checked(tcp.set_read_timeout(Some(Duration::from_secs(2))));
                checked(tcp.set_write_timeout(Some(Duration::from_secs(2))));
                let mut stream = StreamOwned::new(checked(ServerConnection::new(config)), tcp);
                let frame = checked(read_frame(&mut stream, GATEWAY_FRAME_LIMIT));
                let request = checked(kms_wire::Request::decode(&frame));
                if index == 0 {
                    checked(write_frame(
                        &mut stream,
                        &kms_wire::response(request.version, request.operation, Ok(Vec::new())),
                        GATEWAY_FRAME_LIMIT,
                    ));
                    checked(stream.flush());
                } else {
                    checked(arrival.send(Instant::now()));
                    // Dribble a real codec response slowly: per-read relative timeouts
                    // would keep this worker alive beyond the operation budget.
                    let response = kms_wire::response(
                        request.version,
                        request.operation,
                        Err(kms_wire::Error::Unavailable),
                    );
                    let bytes = [
                        (response.len() as u32).to_be_bytes().as_slice(),
                        response.as_slice(),
                    ]
                    .concat();
                    for byte in bytes {
                        if stream
                            .write_all(&[byte])
                            .and_then(|()| stream.flush())
                            .is_err()
                        {
                            break;
                        }
                        std::thread::sleep(Duration::from_millis(35));
                    }
                }
            }));
        }
        for worker in workers {
            checked(worker.join());
        }
    });
    let gateway = Arc::new(LxkpGateway::new(
        Arc::new(remote(address, clock.clone())),
        GatewayTls::new(CA, SERVER, &Zeroizing::new(SERVER_KEY.to_vec()), CLIENT)?,
        GatewayPolicy {
            network_id: 77,
            protocol_version: 2,
            registry: registry(),
            limits: GatewayLimits {
                maximum_connections: 1,
                maximum_frame_bytes: GATEWAY_FRAME_LIMIT,
                deadline: Duration::from_millis(110),
                maximum_operations: 30,
                operation_window: Duration::from_secs(10),
            },
        },
        clock.clone(),
    )?);
    let binding = PrincipalKeyBinding::from_digest([0x44; 32], 77, KeyClass::HumanPrimary)?;
    let reference = ProviderKeyReference::new(vec![0x33; 32])?;
    for operation in 0..7 {
        let front = TcpListener::bind("127.0.0.1:0")?;
        let front_address = front.local_addr()?;
        let serving = gateway.clone();
        let worker = std::thread::spawn(move || {
            let (tcp, _) = checked(front.accept());
            checked(serving.serve_connection(tcp));
        });
        let client = remote(front_address, clock.clone());
        let start = Instant::now();
        let result = match operation {
            0 => client.probe().map(|_| ()),
            1 => client.create_key(&binding).map(|_| ()),
            2 => client.describe_key(&binding, &reference).map(|_| ()),
            3 => client.rotate_key(&binding, &reference).map(|_| ()),
            4 => client
                .rotate_key_if_current(&binding, &reference, [0x22; 32])
                .map(|_| ()),
            5 => client.destroy_key(&binding, &reference),
            _ => {
                let key = checked(k256::ecdsa::SigningKey::from_slice(&[7; 32]));
                let from = checked(layerx_crypto::secp256k1::evm_address(
                    key.verifying_key().to_encoded_point(false).as_bytes(),
                ));
                let mut to = [0; 20];
                to[18..].copy_from_slice(&[0x10, 0x1a]);
                let mut data =
                    layerx_crypto::disclosure::call::ledger_selector("openAccount(bytes32)")
                        .to_vec();
                data.extend([3; 32]);
                let call = layerx_crypto::evm_transaction::Eip1559Call {
                    chain_id: 77,
                    nonce: 0,
                    max_priority_fee_per_gas: 1,
                    max_fee_per_gas: 2,
                    gas_limit: 100000,
                    to,
                    data,
                };
                let disclosure = checked(layerx_crypto::disclosure::call::bind_call(&call, from));
                let frame = checked(client.encode_signing_request(
                    &binding,
                    &reference,
                    ProviderSignRequest::new(&call, &disclosure, from, [0x22; 32]),
                ));
                let gate = ConnectionGate::new(1);
                let mut tls = checked(Tls::connect(
                    front_address,
                    checked("localhost".to_owned().try_into()),
                    &mutual_tls(),
                    &gate,
                    limits(),
                ));
                checked(tls.send(&frame));
                let answer = checked(tls.receive());
                assert_eq!(
                    answer,
                    kms_wire::response(7, 5, Err(kms_wire::Error::Unavailable))
                );
                Err(KmsError::Unavailable)
            }
        };
        assert!(
            matches!(result, Err(KmsError::Unavailable)),
            "operation {operation}: {result:?}"
        );
        assert!(
            start.elapsed() < Duration::from_millis(270),
            "operation {operation} exceeded total gateway budget: {:?}",
            start.elapsed()
        );
        let provider_started = checked(arrivals.recv_timeout(Duration::from_secs(1)));
        assert!(
            provider_started.elapsed() < Duration::from_millis(155),
            "provider exceeded the 110ms absolute budget: {:?}",
            provider_started.elapsed()
        );
        checked(worker.join()); // A refusal also releases the sole gateway participant slot.
    }
    checked(backend.join());
    Ok(())
}

struct Entropy(u8);
impl SessionEntropySource for Entropy {
    fn next_session_entropy(&mut self) -> std::result::Result<SessionKeyEntropy, SessionKeyError> {
        self.0 = self
            .0
            .checked_add(1)
            .ok_or(SessionKeyError::EntropyUnavailable)?;
        let mut seed = [0; 32];
        getrandom::fill(&mut seed).map_err(|_| SessionKeyError::EntropyUnavailable)?;
        SessionKeyEntropy::new(seed)
    }
}
struct IdentityFixture(CoreIdentity);
impl IdentityResolver for IdentityFixture {
    fn resolve(&mut self, _: &Did) -> std::result::Result<Option<CoreIdentity>, IdentityError> {
        Ok(Some(self.0.clone()))
    }
}

/// The peer uses the actual daemon identity registration, session writer,
/// recovery reader, permission checks and revocation code. Chain observations
/// and one-shot refusal boundaries are committed in-memory fixtures.
struct AgentPeer {
    root: PathBuf,
    store: Store,
    sessions: SessionRegistry,
    tenant: TenantId,
    now: u64,
    sequence: u64,
    primary: [u8; 32],
    installs: BTreeMap<[u8; 32], ProvisionEvidence>,
    suspensions: BTreeMap<[u8; 32], SuspensionEvidence>,
    revocations: BTreeMap<[u8; 32], RevocationEvidence>,
    rotation: Option<(RotationEvidence, [u8; 32], bool)>,
    fail: Option<&'static str>,
}
impl AgentPeer {
    fn new(root: PathBuf, primary: [u8; 32]) -> Self {
        let store = checked(Store::open(&root));
        Self {
            root,
            store,
            sessions: SessionRegistry::default(),
            tenant: checked(TenantId::new("tenant-alice")),
            now: 1000,
            sequence: 10,
            primary,
            installs: BTreeMap::new(),
            suspensions: BTreeMap::new(),
            revocations: BTreeMap::new(),
            rotation: None,
            fail: None,
        }
    }
    fn reopen(mut self) -> Self {
        // Close and reopen the real daemon store and rebuild its session registry.
        let root = self.root.clone();
        let placeholder_root = self.root.join("reopen-placeholder");
        let placeholder = checked(Store::open(&placeholder_root));
        drop(std::mem::replace(&mut self.store, placeholder));
        self.store = checked(Store::open(root));
        let _ = fs::remove_dir_all(placeholder_root);
        self.sessions = SessionRegistry::default();
        checked(self.sessions.restore_tenant(&self.store, &self.tenant));
        self
    }
    fn scope(&self, p: &PrincipalId, d: &Did) -> std::result::Result<(), AgentContractError> {
        if *p == principal() && *d == did() {
            Ok(())
        } else {
            Err(AgentContractError::Refused("foreign principal"))
        }
    }
    fn interrupt(&mut self, point: &'static str) -> std::result::Result<(), AgentContractError> {
        if self.fail == Some(point) {
            self.fail = None;
            Err(AgentContractError::Unavailable)
        } else {
            Ok(())
        }
    }
    fn active(&self, grant: [u8; 32]) -> bool {
        self.sessions
            .get(&self.tenant, SessionId(grant))
            .is_some_and(|record| record.open)
    }
    fn commit_rotation(&mut self) {
        let (evidence, _, committed) = self.rotation.as_mut().expect("rotation");
        assert!(self.now >= evidence.effective_at && self.now <= evidence.lapse_at);
        *committed = true;
        self.primary = evidence.pending_public_key;
        self.sequence += 1;
    }
}
impl AgentSessionContract for AgentPeer {
    fn session_authority(
        &mut self,
        target: &SessionTarget,
    ) -> std::result::Result<SessionAuthorityObservation, AgentContractError> {
        self.scope(&target.principal, &target.agent_did)?;
        Ok(SessionAuthorityObservation {
            provision: self
                .installs
                .get(&target.grant_id)
                .cloned()
                .ok_or(AgentContractError::Unavailable)?,
            daemon_suspended: !self.active(target.grant_id),
            revocation: self.revocations.get(&target.grant_id).cloned(),
        })
    }

    fn identity_snapshot(
        &mut self,
        p: &PrincipalId,
        d: &Did,
    ) -> std::result::Result<ProtocolIdentitySnapshot, AgentContractError> {
        self.scope(p, d)?;
        Ok(ProtocolIdentitySnapshot {
            protocol_identity: [0x42; 32],
            primary_public_key: self.primary,
            revocation_sequence: 1,
            protocol_time: self.now,
            core_sequence: self.sequence,
            rung: Rung::Committed,
        })
    }
    fn recover_provision(
        &mut self,
        p: &PrincipalId,
        d: &Did,
        id: [u8; 32],
    ) -> std::result::Result<Option<ProvisionEvidence>, AgentContractError> {
        self.scope(p, d)?;
        Ok(self.installs.get(&id).cloned())
    }
    fn provision_session(
        &mut self,
        request: AgentSessionProvision,
    ) -> std::result::Result<ProvisionEvidence, AgentContractError> {
        let (p, d, issued, scopes, policy_version, secret) = request.into_parts();
        self.scope(&p, &d)?;
        self.interrupt("before-install")?;
        assert!(
            !self.installs.contains_key(&issued.grant_id),
            "a replacement was installed twice"
        );
        let _signer = ProvisionedSessionKey::new(secret.into_seed(), issued.clone())
            .map_err(|_| AgentContractError::Refused("invalid issued secret"))?;
        let mut identity = IdentityFixture(CoreIdentity {
            canonical_bytes: vec![0x42; 32],
            head_sequence: self.sequence,
            revocation_sequence: 1,
            verification_level: VerificationLevel::STATE_PROVEN,
            frozen: false,
            authorities: self
                .installs
                .keys()
                .copied()
                .chain(std::iter::once(issued.grant_id))
                .map(ProtocolAuthority::SessionKey)
                .collect(),
        });
        let identity = register(
            &mut self.store,
            self.tenant.clone(),
            d.clone(),
            &mut identity,
        )
        .map_err(|_| AgentContractError::Unavailable)?;
        let _token = open(
            &mut self.store,
            &mut self.sessions,
            &identity,
            OpenRequest {
                session_id: SessionId(issued.grant_id),
                token_id: Sha256::digest(issued.grant_id).into(),
                tenant: self.tenant.clone(),
                agent: d,
                authority: ProtocolAuthority::SessionKey(issued.grant_id),
                permitted_activity_types: issued
                    .permitted_activity_types
                    .iter()
                    .map(|activity| activity.ordinal())
                    .collect::<BTreeSet<_>>(),
                scopes: scopes.into_iter().collect(),
                expiry_sequence: issued.expires_at,
                expiry_seconds: None,
                opening_client: "human-repair".into(),
                policy_version,
            },
            self.sequence,
        )
        .map_err(|_| AgentContractError::Unavailable)?;
        let mut evidence = ProvisionEvidence {
            grant_id: issued.grant_id,
            session_public_key: issued.session_public_key,
            daemon_session_id: issued.grant_id,
            protocol_sequence: self.sequence,
            observed_at: self.now,
            rung: Rung::Committed,
            receipt_digest: [0; 32],
        };
        evidence.receipt_digest = evidence.expected_digest();
        self.installs.insert(issued.grant_id, evidence.clone());
        self.interrupt("after-install")?;
        Ok(evidence)
    }
    fn suspend_permissions(
        &mut self,
        target: &SessionTarget,
        reason: RevocationReason,
        at: u64,
    ) -> std::result::Result<SuspensionEvidence, AgentContractError> {
        self.scope(&target.principal, &target.agent_did)?;
        if let Some(evidence) = self.suspensions.get(&target.grant_id) {
            return Ok(evidence.clone());
        }
        self.interrupt("before-suspend")?;
        if self.active(target.grant_id) {
            close(
                &mut self.store,
                &mut self.sessions,
                &self.tenant,
                SessionId(target.daemon_session_id),
            )
            .map_err(|_| AgentContractError::Unavailable)?;
        }
        let mut evidence = SuspensionEvidence {
            grant_id: target.grant_id,
            daemon_session_id: target.daemon_session_id,
            reason,
            observed_at: at.max(self.now),
            receipt_digest: [0; 32],
        };
        evidence.receipt_digest = evidence.expected_digest();
        self.suspensions.insert(target.grant_id, evidence.clone());
        self.interrupt("after-suspend")?;
        Ok(evidence)
    }
    fn revoke_protocol_session(
        &mut self,
        target: &SessionTarget,
        reason: RevocationReason,
        at: u64,
    ) -> std::result::Result<RevocationEvidence, AgentContractError> {
        self.scope(&target.principal, &target.agent_did)?;
        if let Some(evidence) = self.revocations.get(&target.grant_id) {
            return Ok(evidence.clone());
        }
        self.interrupt("before-revoke")?;
        invalidate_on_revocation(
            &mut self.store,
            &mut self.sessions,
            &mut [],
            &RevocationEvent {
                did: target.agent_did.clone(),
                authority: Some(ProtocolAuthority::SessionKey(target.grant_id)),
                reason: InvalidationReason::GrantRevokedOnChain,
                observed_sequence: self.sequence + 1,
            },
        )
        .map_err(|_| AgentContractError::Unavailable)?;
        self.sequence += 1;
        let mut evidence = RevocationEvidence {
            grant_id: target.grant_id,
            reason,
            observed_sequence: self.sequence,
            observed_at: self.now.max(at) + 1,
            rung: Rung::Committed,
            receipt_digest: [0; 32],
        };
        evidence.receipt_digest = evidence.expected_digest();
        self.revocations.insert(target.grant_id, evidence.clone());
        self.interrupt("after-revoke")?;
        Ok(evidence)
    }
    fn announce_rotation(
        &mut self,
        submission: RotationSubmission,
    ) -> std::result::Result<RotationEvidence, AgentContractError> {
        self.scope(&submission.principal, &submission.did)?;
        if self
            .rotation
            .as_ref()
            .is_some_and(|(previous, _, committed)| !committed && self.now <= previous.lapse_at)
        {
            return Err(AgentContractError::Refused("rotation open"));
        }
        if checked(DisclosureCheck::verify(
            &submission.intent,
            &submission.compiled,
        )) != submission.disclosure
            || submission.current_public_key != self.primary
        {
            return Err(AgentContractError::Refused("rotation disclosure"));
        }
        let payload_hash = submission.compiled.payload_hash();
        let mut evidence = RotationEvidence {
            tx_hash: Sha256::digest(payload_hash).into(),
            payload_hash,
            pending_public_key: submission.pending_public_key,
            effective_at: submission.effective_at,
            lapse_at: submission.lapse_at,
            effective_sequence: submission.effective_sequence,
            observed_sequence: self.sequence,
            observed_at: self.now,
            rung: Rung::Committed,
            receipt_digest: [0; 32],
        };
        evidence.receipt_digest = evidence.expected_digest();
        self.rotation = Some((evidence.clone(), self.primary, false));
        self.interrupt("after-announce")?;
        Ok(evidence)
    }
    fn recover_rotation(
        &mut self,
        p: &PrincipalId,
        d: &Did,
        payload: [u8; 32],
    ) -> std::result::Result<Option<RotationEvidence>, AgentContractError> {
        self.scope(p, d)?;
        Ok(self
            .rotation
            .as_ref()
            .filter(|(evidence, _, _)| evidence.payload_hash == payload)
            .map(|(evidence, _, _)| evidence.clone()))
    }
    fn rotation_observation(
        &mut self,
        p: &PrincipalId,
        d: &Did,
    ) -> std::result::Result<RotationObservation, AgentContractError> {
        self.scope(p, d)?;
        let (evidence, old, committed) = self
            .rotation
            .as_ref()
            .ok_or(AgentContractError::Unavailable)?;
        Ok(RotationObservation {
            tx_hash: evidence.tx_hash,
            primary_public_key: self.primary,
            pending_public_key: (!committed && self.now <= evidence.lapse_at)
                .then_some(evidence.pending_public_key),
            superseded_public_key: committed.then_some(*old),
            effective_at: evidence.effective_at,
            lapse_at: evidence.lapse_at,
            effective_sequence: evidence.effective_sequence,
            observed_at: self.now,
            observed_sequence: self.sequence,
            rung: Rung::Committed,
        })
    }
}
fn owner(root: &Path, peer: AgentPeer) -> SessionKeyProvisioner<AgentPeer, Entropy> {
    checked(SessionKeyProvisioner::open(
        root,
        peer,
        Entropy(10),
        policy(),
        registry(),
    ))
}

#[test]
fn sessions_state_restart_recovers_install_suspend_and_revoke() -> Result {
    for point in [
        "before-install",
        "after-install",
        "after-suspend",
        "before-revoke",
        "after-revoke",
    ] {
        let fixture = Fixture::new(point);
        let root = fixture.0.join("lifecycle");
        let mut peer = AgentPeer::new(fixture.0.join("daemon"), [0x31; 32]);
        if point.contains("install") {
            peer.fail = Some(point);
        }
        let mut service = owner(&root, peer);
        let provision = service.provision(&principal(), &did(), vec![activity()]);
        if point.contains("install") {
            assert!(provision.is_err());
            let peer = service.into_contract().reopen();
            let service = owner(&root, peer);
            let lease = service
                .session(&principal(), &did())
                .expect("installation recovered");
            assert!(service.contract().active(lease.grant_id));
            assert_eq!(service.contract().installs.len(), 1);
        } else {
            let lease = provision?;
            service.contract_mut().fail = Some(point);
            assert!(service.pause(&principal(), &did(), 1001).is_err());
            assert!(!service.contract().active(lease.grant_id));
            let peer = service.into_contract().reopen();
            let mut service = owner(&root, peer);
            assert!(matches!(
                service
                    .session(&principal(), &did())
                    .expect("recovered")
                    .state,
                SessionLeaseState::Revoked {
                    reason: RevocationReason::Paused,
                    ..
                }
            ));
            assert_eq!(service.contract().revocations.len(), 1);
            assert!(service
                .provision(&principal(), &did(), vec![activity()])
                .is_err());
            let stranger = PrincipalId::new("mallory")?;
            assert!(service.session(&stranger, &did()).is_none());
            let resumed = service.resume(&principal(), &did())?;
            assert_ne!(resumed.grant_id, lease.grant_id);
            assert!(service.contract().active(resumed.grant_id));
            service.archive(&principal(), &did(), 1002)?;
            let peer = service.into_contract().reopen();
            let mut service = owner(&root, peer);
            assert!(matches!(
                service.resume(&principal(), &did()),
                Err(SessionKeyError::Archived)
            ));
        }
    }
    Ok(())
}

#[test]
fn session_renewal_orphan_resumes_exact_replacement() -> Result {
    for restart in [false, true] {
        for point in [
            "after-install",
            "before-suspend",
            "after-suspend",
            "before-revoke",
            "after-revoke",
        ] {
            let fixture = Fixture::new(&format!("renewal-{point}-{restart}"));
            let root = fixture.0.join("lifecycle");
            let mut service = owner(&root, AgentPeer::new(fixture.0.join("daemon"), [0x31; 32]));
            let first = service.provision(&principal(), &did(), vec![activity()])?;
            service.contract_mut().now = 1081;
            service.contract_mut().fail = Some(point);
            assert!(service.renew_expiring(1081).remove(0).is_err());
            assert_eq!(service.contract().installs.len(), 2);
            let replacement = *service
                .contract()
                .installs
                .keys()
                .find(|grant| **grant != first.grant_id)
                .expect("replacement retained");
            let service = if restart {
                let peer = service.into_contract().reopen();
                owner(&root, peer)
            } else {
                assert!(service.renew_expiring(1081).remove(0).is_ok());
                service
            };
            assert_eq!(
                service
                    .session(&principal(), &did())
                    .expect("recovered")
                    .grant_id,
                replacement
            );
            assert_eq!(service.contract().installs.len(), 2);
            assert!(!service.contract().active(first.grant_id));
            assert!(service.contract().active(replacement));
            assert_eq!(service.contract().revocations.len(), 1);
        }
    }
    Ok(())
}

#[test]
fn session_second_rotation_and_retry_after_lapse() -> Result {
    let fixture = Fixture::new("repair-rotations");
    let custody = fixture.custody();
    let keys: Vec<_> = (0..5)
        .map(|index| checked(KeyId::new(format!("key{index}"))))
        .collect();
    let publics: Vec<_> = keys
        .iter()
        .map(|key| checked(custody.create(&principal(), key, KeyClass::HumanPrimary)))
        .collect();
    let root = fixture.0.join("lifecycle");
    let mut service = owner(&root, AgentPeer::new(fixture.0.join("daemon"), publics[0]));
    for index in 0..2 {
        if index == 0 {
            service.contract_mut().fail = Some("after-announce");
            assert!(service
                .announce_rotation(
                    &custody,
                    &principal(),
                    RotationSubject::Human,
                    &did(),
                    &keys[index],
                    &keys[index + 1],
                    10,
                    100 + index as u64
                )
                .is_err());
            let peer = service.into_contract().reopen();
            service = owner(&root, peer);
        } else {
            service.announce_rotation(
                &custody,
                &principal(),
                RotationSubject::Human,
                &did(),
                &keys[index],
                &keys[index + 1],
                10,
                100 + index as u64,
            )?;
        }
        assert!(matches!(
            service.announce_rotation(
                &custody,
                &principal(),
                RotationSubject::Human,
                &did(),
                &keys[index],
                &keys[index + 1],
                10,
                120
            ),
            Err(SessionKeyError::RotationAlreadyOpen)
        ));
        service.contract_mut().now += 10;
        service.contract_mut().commit_rotation();
        let peer = service.into_contract().reopen();
        service = owner(&root, peer);
        assert!(matches!(
            service
                .rotation_history()
                .last()
                .expect("terminal history")
                .state,
            RotationJourneyState::Effective { .. }
        ));
    }
    assert_eq!(service.rotation_history().len(), 2);
    let lapsed = service.announce_rotation(
        &custody,
        &principal(),
        RotationSubject::Human,
        &did(),
        &keys[2],
        &keys[3],
        10,
        200,
    )?;
    service.contract_mut().now = lapsed.lapse_at + 1;
    let peer = service.into_contract().reopen();
    service = owner(&root, peer);
    assert!(matches!(
        service.rotation_history().last().expect("lapsed").state,
        RotationJourneyState::Lapsed
    ));
    let next = service.announce_rotation(
        &custody,
        &principal(),
        RotationSubject::Human,
        &did(),
        &keys[2],
        &keys[4],
        10,
        201,
    )?;
    assert_ne!(next.tx_hash, lapsed.tx_hash);
    assert_eq!(service.rotation_history().len(), 3);
    Ok(())
}

#[test]
fn session_production_driver_renews_and_recovers_through_maintenance() -> Result {
    let fixture = Fixture::new("repair-driver");
    let root = fixture.0.join("lifecycle");
    let mut service = owner(&root, AgentPeer::new(fixture.0.join("daemon"), [0x31; 32]));
    let first = service.provision(&principal(), &did(), vec![activity()])?;
    service.contract_mut().now = 1081;
    service.contract_mut().fail = Some("before-revoke");
    let driver = SessionLifecycleDriver::new(service)?;
    assert_eq!(checked(driver.maintain(0, 1081)), 0);
    assert!(driver.maintain(1, 1081).is_err());
    assert!(!driver.healthy());
    let peer = driver.into_provisioner()?.into_contract().reopen();
    let service = owner(&root, peer);
    assert_eq!(service.contract().installs.len(), 2);
    assert!(!service.contract().active(first.grant_id));
    let replacement = service
        .session(&principal(), &did())
        .expect("recovered replacement")
        .clone();
    let driver = SessionLifecycleDriver::new(service)?;
    assert_eq!(checked(driver.maintain(1, 1082)), 0);
    assert!(driver.healthy());
    let mut service = driver.into_provisioner()?;
    service.contract_mut().now = replacement.expires_at - 19;
    let driver = SessionLifecycleDriver::new(service)?;
    let clock: Arc<dyn Clock> = Arc::new(ElapsedClock(
        Instant::now(),
        (replacement.expires_at - 19) * 1000,
    ));
    let backend = Arc::new(checked(
        layerx_human_service::server::backend::UnixComponents::new_with_clock(
            fixture.0.join("product.sock"),
            limits(),
            clock.clone(),
        ),
    ));
    let components = Arc::new(SessionMaintainedComponents::new(backend, driver));
    let socket_root = fixture.0.join("component");
    fs::create_dir_all(&socket_root)?;
    fs::set_permissions(&socket_root, fs::Permissions::from_mode(0o700))?;
    let server = checked(
        layerx_human_service::server::HumanComponentServer::new_maintained(
            components.clone(),
            Duration::from_millis(20),
            1,
            clock,
        ),
    );
    let bound = checked(
        server.bind(layerx_human_service::server::ComponentServerConfig {
            socket_path: socket_root.join("service.sock"),
            allowed_uid: rustix::process::geteuid().as_raw(),
            worker_count: 1,
            queue_capacity: 1,
            limits: limits(),
        }),
    );
    let shutdown = bound.shutdown();
    let server = std::thread::spawn(move || checked(bound.run()));
    let until = Instant::now() + Duration::from_secs(2);
    loop {
        let current = components
            .sessions()
            .session(&principal(), &did())?
            .expect("scheduled session");
        if current.grant_id != replacement.grant_id {
            break;
        }
        assert!(
            Instant::now() < until,
            "production component scheduler did not renew authority"
        );
        std::thread::sleep(Duration::from_millis(5));
    }
    shutdown.request();
    checked(server.join());
    let components = Arc::try_unwrap(components)
        .unwrap_or_else(|_| panic!("component graph still held by server"));
    let service = components.into_sessions().into_provisioner()?;
    assert_eq!(service.contract().installs.len(), 3);
    assert!(!service.contract().active(replacement.grant_id));
    assert!(SessionLifecycleDriver::new(SessionKeyProvisioner::new(
        AgentPeer::new(fixture.0.join("ephemeral"), [0x31; 32]),
        Entropy(20),
        policy(),
        registry()
    ))
    .is_err());
    Ok(())
}
