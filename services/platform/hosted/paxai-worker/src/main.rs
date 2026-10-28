//! PAXAI worker service: serves the signed status probe and authenticates every signed job
//! request against the worker's delegate-signed metadata binding.
//!
//! Signed jobs are admitted against the current finalized delegate and F08 membership.
//! Durable receipts precede sponsored task acceptance; execution waits for that acceptance
//! to finalize. The maintenance loop reconciles fenced leases and submits result commitments
//! and membership heartbeats through the programs precompile.
use ed25519_dalek::{Signer, SigningKey};
use layerx_client::head::Head;
use layerx_paxai_worker::{
    auth::{
        acknowledgment_digest, decode_service, sign_status, verify_request, verify_signed_metadata,
        AuthenticatedRequest, MetadataContext, Route, ServiceError, StatusChallenge, WorkerBinding,
        STATUS_PATH,
    },
    discovery::spki_sha256,
    discovery::{AuthorityEvidence, IdentityEvidence, VerifiedMetadata, READINESS_TTL_MS},
    jobs::{AdmissionView, Input, JobService, Outcome},
    membership::{MembershipObserver, Subject},
    metrics::Metrics,
    runner::ProcessRunner,
    store::{JobKey, JobState, JobStore},
};
use layerx_programs_ai_market::{
    codec::{self, Envelope},
    dispatch::{self, Operation},
    queries::{parse_hex32, FinalityEvidence, ReadProof, StateCapture},
    registry_ops::PolicySection,
    state::{decode_shared_state, ActorSlot, Section},
    tasks::{TaskBinding, TaskSet, TaskStatus},
    types::{
        Authentication, ChainDomain, Digest32, MarketId, Presence, PrincipalId, ProgramId,
        PublicKey32, RequestId, Signature64, TaskId, WorkerId,
    },
    MAX_ENVELOPE_BYTES, MAX_STATE_BYTES,
};
use rustls::{
    pki_types::{pem::PemObject, CertificateDer, PrivateKeyDer},
    ServerConfig, ServerConnection, StreamOwned,
};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::fs::{self, File};
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::process::{Command, ExitCode, Stdio};
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc, Mutex,
};
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

const MAX_CONNECTIONS: usize = 128;
static ACTIVE_CONNECTIONS: AtomicUsize = AtomicUsize::new(0);
const REQUEST_DEADLINE: Duration = Duration::from_secs(30);
const MAX_REQUEST_HEAD_BYTES: usize = 8_192;

struct ConnectionPermit;

impl ConnectionPermit {
    fn acquire() -> Option<Self> {
        ACTIVE_CONNECTIONS
            .fetch_update(Ordering::AcqRel, Ordering::Acquire, |active| {
                (active < MAX_CONNECTIONS).then_some(active + 1)
            })
            .ok()
            .map(|_| Self)
    }
}

impl Drop for ConnectionPermit {
    fn drop(&mut self) {
        ACTIVE_CONNECTIONS.fetch_sub(1, Ordering::AcqRel);
    }
}

struct DeadlineSocket {
    socket: TcpStream,
    deadline: Instant,
}

impl DeadlineSocket {
    fn remaining(&self) -> std::io::Result<Duration> {
        self.deadline
            .checked_duration_since(Instant::now())
            .filter(|remaining| !remaining.is_zero())
            .ok_or_else(|| std::io::Error::from(std::io::ErrorKind::TimedOut))
    }
}

impl Read for DeadlineSocket {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        self.socket.set_read_timeout(Some(self.remaining()?))?;
        self.socket.read(buf)
    }
}

impl Write for DeadlineSocket {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        self.socket.set_write_timeout(Some(self.remaining()?))?;
        self.socket.write(buf)
    }

    fn flush(&mut self) -> std::io::Result<()> {
        self.socket.set_write_timeout(Some(self.remaining()?))?;
        self.socket.flush()
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct Config {
    listen: String,
    certificate_chain: PathBuf,
    private_key: PathBuf,
    delegate_seed: PathBuf,
    signed_metadata: PathBuf,
    chain: String,
    program: String,
    market: String,
    worker: String,
    owner: String,
    state_dir: PathBuf,
    input_dir: PathBuf,
    runner: PathBuf,
    authority_reader: PathBuf,
    sponsored_sender: PathBuf,
    chain_config: PathBuf,
    capabilities: Vec<u8>,
    access_declaration: Vec<u8>,
    response_capacity: u32,
    budget: [u64; 7],
    fee_limit: u128,
}

/// One startup refusal; it never carries key material or file contents.
struct Startup(&'static str);

pub(crate) struct Service {
    pub(crate) binding: WorkerBinding,
    pub(crate) key: SigningKey,
    pub(crate) metrics: Metrics,
    pub(crate) jobs: JobService,
    pub(crate) runtime: Mutex<Runtime>,
    pub(crate) state_dir: PathBuf,
    pub(crate) input_dir: PathBuf,
}

pub(crate) struct Runtime {
    pub(crate) membership: MembershipObserver,
    pub(crate) current: Option<CurrentView>,
    pub(crate) refreshed: Option<Instant>,
}

pub(crate) struct CurrentView {
    pub(crate) state: Vec<u8>,
    pub(crate) metadata: VerifiedMetadata,
    pub(crate) identities: Vec<IdentityEvidence>,
}

impl Runtime {
    fn invalidate(&mut self) {
        self.current = None;
        self.refreshed = None;
        let _ = self.membership.observe(None);
    }

    pub(crate) fn observe(
        &mut self,
        evidence: &AuthorityEvidence<'_>,
        signed_metadata: &[u8],
        identities: Vec<IdentityEvidence>,
    ) -> Result<(), ServiceError> {
        self.invalidate();
        self.membership
            .observe(Some(evidence))
            .map_err(|error| match error {
                layerx_paxai_worker::membership::MembershipError::Service(error) => error,
                _ => ServiceError::ExecutionUnavailable,
            })?;
        let authority = &self
            .membership
            .current()
            .ok_or(ServiceError::StaleAuthority)?
            .authority;
        let metadata = match verify_signed_metadata(signed_metadata, &authority.metadata_context())
            .and_then(|signed| authority.bind_metadata(signed))
        {
            Ok(metadata) => metadata,
            Err(error) => {
                self.invalidate();
                return Err(error);
            }
        };
        if identities
            .iter()
            .any(|identity| identity.execution_height != authority.height())
        {
            self.invalidate();
            return Err(ServiceError::StaleAuthority);
        }
        self.current = Some(CurrentView {
            state: evidence.state.to_vec(),
            metadata,
            identities,
        });
        self.refreshed = Some(Instant::now());
        Ok(())
    }

    fn fresh(&mut self) -> Result<(), ServiceError> {
        if !self
            .refreshed
            .is_some_and(|at| at.elapsed() < Duration::from_millis(READINESS_TTL_MS))
        {
            self.invalidate();
        }
        Ok(())
    }
}

pub(crate) struct Response {
    pub(crate) status: u16,
    pub(crate) body: Vec<u8>,
    content_type: &'static str,
}

fn hex32(text: &str) -> Result<[u8; 32], Startup> {
    parse_hex32(text.trim()).map_err(|_| Startup("identifier is not 64 lowercase hex digits"))
}

fn load(path: &str) -> Result<(Service, Arc<ServerConfig>, Config), Startup> {
    let text = std::fs::read_to_string(path).map_err(|_| Startup("configuration unreadable"))?;
    let config: Config =
        serde_json::from_str(&text).map_err(|_| Startup("configuration is not valid"))?;
    let seed = std::fs::read_to_string(&config.delegate_seed)
        .map_err(|_| Startup("delegate seed unreadable"))?;
    let key = SigningKey::from_bytes(&hex32(&seed)?);
    let context = MetadataContext {
        chain: ChainDomain::new(hex32(&config.chain)?).map_err(|_| Startup("zero chain"))?,
        program: ProgramId::new(hex32(&config.program)?).map_err(|_| Startup("zero program"))?,
        market: MarketId::new(hex32(&config.market)?).map_err(|_| Startup("zero market"))?,
        worker: WorkerId::new(hex32(&config.worker)?).map_err(|_| Startup("zero worker"))?,
        owner: PrincipalId::new(hex32(&config.owner)?).map_err(|_| Startup("zero owner"))?,
        delegate: PublicKey32(key.verifying_key().to_bytes()),
    };
    let signed = std::fs::read(&config.signed_metadata)
        .map_err(|_| Startup("signed metadata unreadable"))?;
    let signed = verify_signed_metadata(&signed, &context)
        .map_err(|_| Startup("signed metadata refused for this worker and delegate"))?;
    let chain = CertificateDer::pem_file_iter(&config.certificate_chain)
        .map_err(|_| Startup("certificate chain unreadable"))?
        .collect::<Result<Vec<_>, _>>()
        .map_err(|_| Startup("certificate chain is not valid PEM"))?;
    let leaf = chain.first().ok_or(Startup("certificate chain is empty"))?;
    let pin = spki_sha256(leaf).map_err(|_| Startup("leaf certificate unparsable"))?;
    if !signed
        .manifest
        .endpoints
        .iter()
        .any(|e| e.spki_sha256 == pin)
    {
        return Err(Startup(
            "leaf certificate is not pinned by the signed metadata",
        ));
    }
    let private = PrivateKeyDer::from_pem_file(&config.private_key)
        .map_err(|_| Startup("private key unreadable"))?;
    let tls =
        ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_protocol_versions(&[&rustls::version::TLS13])
            .map_err(|_| Startup("TLS 1.3 unavailable"))?
            .with_no_client_auth()
            .with_single_cert(chain, private)
            .map_err(|_| Startup("certificate and private key do not match"))?;
    let binding = signed.binding();
    let membership = MembershipObserver::open(
        &config.state_dir,
        Subject {
            chain: binding.chain,
            program: binding.program,
            market: binding.market,
            worker: binding.worker,
        },
    )
    .map_err(|_| Startup("membership journal unavailable"))?;
    let store = JobStore::open(&config.state_dir.join("jobs"))
        .map_err(|_| Startup("job store unavailable"))?;
    let runner = ProcessRunner::new(config.runner.clone(), Vec::new(), Duration::from_secs(5));
    let service = Service {
        binding,
        jobs: JobService::new(store, runner, key.clone()),
        key,
        metrics: Metrics::default(),
        runtime: Mutex::new(Runtime {
            membership,
            current: None,
            refreshed: None,
        }),
        state_dir: config.state_dir.clone(),
        input_dir: config.input_dir.clone(),
    };
    Ok((service, Arc::new(tls), config))
}

fn refusal(error: ServiceError, request: Presence<RequestId>) -> Response {
    let status = match error {
        ServiceError::NotFound => 404,
        ServiceError::InputTooLarge => 413,
        ServiceError::StaleAuthority => 503,
        _ => 400,
    };
    Response {
        status,
        body: error.body(request).to_vec(),
        content_type: "application/octet-stream",
    }
}

impl Service {
    pub(crate) fn dispatch(&self, method: &str, path: &str, body: &[u8]) -> Response {
        if method == "GET" && path == "/metrics" {
            return Response {
                status: 200,
                body: self.metrics.render().into_bytes(),
                content_type: "text/plain; version=0.0.4",
            };
        }
        if method != "POST" {
            return refusal(ServiceError::NotFound, Presence::Absent);
        }
        if path == STATUS_PATH {
            return self.status(body);
        }
        match Route::from_path(path) {
            Some(route) => self.job(route, body),
            None => refusal(ServiceError::NotFound, Presence::Absent),
        }
    }

    fn status(&self, body: &[u8]) -> Response {
        let signed = StatusChallenge::decode(body)
            .and_then(|challenge| sign_status(&self.binding, &challenge, &self.key));
        match signed {
            Ok(body) => Response {
                status: 200,
                body,
                content_type: "application/octet-stream",
            },
            Err(error) => refusal(error, Presence::Absent),
        }
    }

    fn job(&self, route: Route, body: &[u8]) -> Response {
        let envelope = match decode_service(body) {
            Ok(envelope) => envelope,
            Err(error) => return refusal(error, Presence::Absent),
        };
        let request = Presence::Present(envelope.context.request);
        if let Err(error) = verify_request(&envelope, route, &self.binding) {
            return refusal(error, request);
        }
        let answer = (|| {
            let mut runtime = self
                .runtime
                .lock()
                .map_err(|_| ServiceError::ExecutionUnavailable)?;
            runtime.fresh()?;
            let authority = *runtime.membership.new_work()?;
            let authenticated = verify_request(&envelope, route, &authority.worker_binding())?;
            let current = runtime
                .current
                .as_ref()
                .ok_or(ServiceError::ExecutionUnavailable)?;
            let actor = current
                .identities
                .iter()
                .find(|identity| identity.principal == envelope.context.actor)
                .ok_or(ServiceError::AccessDenied)?;
            match authenticated {
                AuthenticatedRequest::Submit(submit) => {
                    let task = task_binding(&current.state, submit.task)?;
                    let key = JobKey::derive(
                        envelope.context.market,
                        envelope.context.actor,
                        envelope.context.request,
                    )?;
                    self.track(key)?;
                    let name = hex(submit.input_commitment.as_bytes());
                    let manifest = read_bounded(
                        &self.input_dir.join(format!("{name}.manifest")),
                        layerx_programs_ai_market::evidence::MAX_MANIFEST_BYTES,
                    )?;
                    let payload = read_bounded(
                        &self.input_dir.join(format!("{name}.payload")),
                        layerx_paxai_worker::store::MAX_ENCRYPTED_BYTES,
                    )?;
                    let receipt = self.jobs.submit(
                        body,
                        Input {
                            manifest: &manifest,
                            payload: &payload,
                        },
                        &AdmissionView {
                            authority: &authority,
                            customer: actor,
                            metadata: &current.metadata,
                            task: &task,
                        },
                        now_ms()?,
                    )?;
                    Ok(receipt.acknowledgment)
                }
                AuthenticatedRequest::Query(_) => {
                    let answer = self.jobs.query(body, &authority, actor, &[], now_ms()?)?;
                    Ok(answer.result.unwrap_or(answer.acknowledgment))
                }
                AuthenticatedRequest::Cancel(reference) => {
                    self.jobs.cancel(body, &authority, actor, now_ms()?)?;
                    let answer = self
                        .jobs
                        .store()
                        .by_request(envelope.context.market, reference.original)?;
                    answer
                        .into_iter()
                        .next()
                        .map(|job| job.record.acknowledgment)
                        .ok_or(ServiceError::NotFound)
                }
            }
        })();
        match answer {
            Ok(body) => Response {
                status: 200,
                body,
                content_type: "application/octet-stream",
            },
            Err(error) => refusal(error, request),
        }
    }
}

fn now_ms() -> Result<u64, ServiceError> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .ok()
        .and_then(|time| u64::try_from(time.as_millis()).ok())
        .ok_or(ServiceError::ExecutionUnavailable)
}

fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    bytes.iter().fold(String::new(), |mut out, byte| {
        let _ = write!(out, "{byte:02x}");
        out
    })
}

fn read_bounded(path: &std::path::Path, limit: usize) -> Result<Vec<u8>, ServiceError> {
    let file = File::open(path).map_err(|_| ServiceError::ExecutionUnavailable)?;
    let mut bytes = Vec::new();
    file.take((limit + 1) as u64)
        .read_to_end(&mut bytes)
        .map_err(|_| ServiceError::ExecutionUnavailable)?;
    if bytes.len() > limit {
        return Err(ServiceError::InputTooLarge);
    }
    Ok(bytes)
}

fn atomic_json(path: &std::path::Path, value: &impl Serialize) -> Result<(), ServiceError> {
    let bytes = serde_json::to_vec(value).map_err(|_| ServiceError::NonCanonical)?;
    let temporary = path.with_extension("pending");
    let mut file = File::create(&temporary).map_err(|_| ServiceError::ExecutionUnavailable)?;
    file.write_all(&bytes)
        .and_then(|()| file.sync_all())
        .map_err(|_| ServiceError::ExecutionUnavailable)?;
    fs::rename(&temporary, path).map_err(|_| ServiceError::ExecutionUnavailable)?;
    File::open(path.parent().ok_or(ServiceError::NonCanonical)?)
        .and_then(|directory| directory.sync_all())
        .map_err(|_| ServiceError::ExecutionUnavailable)
}

fn task_binding(state: &[u8], task: TaskId) -> Result<TaskBinding, ServiceError> {
    let shared = decode_shared_state(state)?;
    let policy = PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
    for binding in TaskSet::decode(policy.task_region)?.bindings() {
        let binding = binding?;
        if binding.task == task {
            return Ok(binding);
        }
    }
    Err(ServiceError::NotFound)
}

impl Service {
    fn tracked(&self) -> Result<Vec<[u8; 32]>, ServiceError> {
        let path = self.state_dir.join("chain-jobs.json");
        match fs::read(&path) {
            Ok(bytes) => serde_json::from_slice(&bytes).map_err(|_| ServiceError::NonCanonical),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(Vec::new()),
            Err(_) => Err(ServiceError::ExecutionUnavailable),
        }
    }

    fn track(&self, key: JobKey) -> Result<(), ServiceError> {
        let mut keys = self.tracked()?;
        if !keys.contains(&key.digest().bytes()) {
            keys.push(key.digest().bytes());
            atomic_json(&self.state_dir.join("chain-jobs.json"), &keys)?;
        }
        Ok(())
    }
}

/// Process adapters receive JSON on stdin and answer JSON on stdout. The authority reader
/// supplies native verified, same-height chunk proofs and identity facts from `chain_config`;
/// the sponsored sender signs and submits the exact ABI call, preserving its idempotency key.
fn adapter(program: &std::path::Path, request: serde_json::Value) -> Result<Vec<u8>, ServiceError> {
    let input = serde_json::to_vec(&request).map_err(|_| ServiceError::NonCanonical)?;
    let mut child = Command::new(program)
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::null())
        .spawn()
        .map_err(|_| ServiceError::ExecutionUnavailable)?;
    let mut stdin = child
        .stdin
        .take()
        .ok_or(ServiceError::ExecutionUnavailable)?;
    let stdout = child
        .stdout
        .take()
        .ok_or(ServiceError::ExecutionUnavailable)?;
    let writer = std::thread::spawn(move || stdin.write_all(&input));
    let reader = std::thread::spawn(move || {
        let mut bytes = Vec::new();
        stdout
            .take((MAX_STATE_BYTES * 8 + 1) as u64)
            .read_to_end(&mut bytes)
            .map(|_| bytes)
    });
    let deadline = Instant::now() + Duration::from_secs(10);
    let success = loop {
        match child.try_wait() {
            Ok(Some(status)) => break status.success(),
            Ok(None) if Instant::now() < deadline => std::thread::sleep(Duration::from_millis(10)),
            _ => {
                let _ = child.kill();
                let _ = child.wait();
                break false;
            }
        }
    };
    let written = writer
        .join()
        .map_err(|_| ServiceError::ExecutionUnavailable)?;
    let bytes = reader
        .join()
        .map_err(|_| ServiceError::ExecutionUnavailable)?
        .map_err(|_| ServiceError::ExecutionUnavailable)?;
    if !success || written.is_err() || bytes.len() > MAX_STATE_BYTES * 8 {
        return Err(ServiceError::ExecutionUnavailable);
    }
    Ok(bytes)
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct IdentityFact {
    principal: [u8; 32],
    primary_key: [u8; 32],
    frozen: bool,
    execution_height: u64,
}
impl IdentityFact {
    fn evidence(&self) -> Result<IdentityEvidence, ServiceError> {
        Ok(IdentityEvidence {
            principal: PrincipalId::new(self.principal)?,
            primary_key: PublicKey32(self.primary_key),
            frozen: self.frozen,
            execution_height: self.execution_height,
        })
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct FinalizedObservation {
    chunks: Vec<Vec<u8>>,
    native_state_root: [u8; 32],
    observed_sequence: u64,
    execution_height: u64,
    block_height: u64,
    block_hash: [u8; 32],
    app_hash: [u8; 32],
    store_proof_digest: [u8; 32],
    sealed_height: u64,
    owner: IdentityFact,
    identities: Vec<IdentityFact>,
    roster: Option<Vec<u8>>,
    signed_metadata: Vec<u8>,
}

fn refresh_authority(service: &Service, config: &Config) -> Result<(), ServiceError> {
    let result = (|| {
        let bytes = adapter(
            &config.authority_reader,
            json!({
                "chain_config": config.chain_config, "finality": "finalized",
                "chain": hex(service.binding.chain.as_bytes()), "program": hex(service.binding.program.as_bytes()),
                "market": hex(service.binding.market.as_bytes()), "worker": hex(service.binding.worker.as_bytes()),
            }),
        )?;
        let observation: FinalizedObservation =
            serde_json::from_slice(&bytes).map_err(|_| ServiceError::NonCanonical)?;
        let proof = ReadProof {
            chain: service.binding.chain,
            program: service.binding.program,
            native_state_root: Digest32::new(observation.native_state_root)?,
            observed_sequence: observation.observed_sequence,
            execution_height: observation.execution_height,
            block_hash: Digest32::new(observation.block_hash)?,
        };
        let mut buffer = vec![0; MAX_STATE_BYTES];
        let mut capture = StateCapture::new(&mut buffer);
        for chunk in &observation.chunks {
            capture
                .accept(&proof, chunk)
                .map_err(|_| ServiceError::StaleAuthority)?;
        }
        let (state, facts) = capture.finish().map_err(|_| ServiceError::StaleAuthority)?;
        let finality = FinalityEvidence {
            native_state_root: proof.native_state_root,
            block_height: observation.block_height,
            block_hash: proof.block_hash,
            app_hash: Digest32::new(observation.app_hash)?,
            store_proof_digest: Digest32::new(observation.store_proof_digest)?,
            rank: 4,
        };
        let evidence = AuthorityEvidence {
            state,
            facts: &facts,
            finality: &finality,
            head: Head {
                chain_sequence: observation.observed_sequence,
                sealed_batch: observation.sealed_height,
                finalised_checkpoint: observation.block_hash,
            },
            owner: observation.owner.evidence()?,
            roster: observation.roster.as_deref(),
            observed_ms: now_ms()?,
        };
        let identities = observation
            .identities
            .iter()
            .map(IdentityFact::evidence)
            .collect::<Result<Vec<_>, _>>()?;
        let mut runtime = service
            .runtime
            .lock()
            .map_err(|_| ServiceError::ExecutionUnavailable)?;
        runtime.observe(&evidence, &observation.signed_metadata, identities)
    })();
    if result.is_err() {
        service
            .runtime
            .lock()
            .map_err(|_| ServiceError::ExecutionUnavailable)?
            .invalidate();
    }
    result
}

fn word(value: u128) -> [u8; 32] {
    let mut out = [0; 32];
    out[16..].copy_from_slice(&value.to_be_bytes());
    out
}

pub(crate) fn programs_call(config: &Config, program: ProgramId, envelope: &[u8]) -> Vec<u8> {
    let mut head = vec![0x54, 0xbd, 0x2b, 0x48];
    head.extend_from_slice(program.as_bytes());
    head.extend_from_slice(&word(5));
    let mut tail = Vec::new();
    for bytes in [
        b"layerx_call".as_slice(),
        envelope,
        &config.capabilities,
        &config.access_declaration,
    ] {
        head.extend_from_slice(&word((15 * 32 + tail.len()) as u128));
        tail.extend_from_slice(&word(bytes.len() as u128));
        tail.extend_from_slice(bytes);
        tail.resize(tail.len().next_multiple_of(32), 0);
    }
    head.extend_from_slice(&word(u128::from(config.response_capacity)));
    for budget in config.budget {
        head.extend_from_slice(&word(u128::from(budget)));
    }
    head.extend_from_slice(&word(config.fee_limit));
    head.extend_from_slice(&tail);
    head
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub(crate) struct PendingCall {
    pub(crate) request: [u8; 32],
    pub(crate) sequence: u64,
    pub(crate) expiry: u64,
    pub(crate) envelope: Vec<u8>,
}

pub(crate) fn signed_operation(
    service: &Service,
    authority: &layerx_paxai_worker::discovery::FinalizedAuthority,
    operation: Operation,
    sequence: u64,
    payload: &[u8],
) -> Result<PendingCall, ServiceError> {
    let binding = authority.worker_binding();
    if PublicKey32(service.key.verifying_key().to_bytes()) != binding.delegate {
        return Err(ServiceError::WrongGeneration);
    }
    let mut identity = payload.to_vec();
    identity.extend_from_slice(&sequence.to_be_bytes());
    identity.extend_from_slice(&operation.selector().to_be_bytes());
    identity.extend_from_slice(authority.snapshot().snapshot_id()?.as_bytes());
    let request = codec::domain_hash("PAXAI/worker-chain-operation/v1", &identity)?;
    let expiry = authority
        .height()
        .checked_add(64)
        .ok_or(ServiceError::Overflow)?;
    let mut envelope = Envelope {
        operation,
        chain: binding.chain,
        program: binding.program,
        market: binding.market,
        actor: binding.owner,
        epoch: authority
            .current_epoch()
            .ok_or(ServiceError::AdmissionNotEffective)?,
        config: authority.config(),
        roster: authority.roster(),
        sequence,
        expiry,
        request: RequestId::new(request.bytes())?,
        payload,
        authentication: Authentication::Delegate {
            key: binding.delegate,
            signature: Signature64([0; 64]),
        },
    };
    let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
    let n = codec::encode_envelope(&envelope, &mut encoded)?;
    let digest = codec::decode_envelope(&encoded[..n])?.request_digest()?;
    envelope.authentication = Authentication::Delegate {
        key: binding.delegate,
        signature: Signature64(service.key.sign(digest.as_bytes()).to_bytes()),
    };
    let n = codec::encode_envelope(&envelope, &mut encoded)?;
    encoded.truncate(n);
    Ok(PendingCall {
        request: request.bytes(),
        sequence,
        expiry,
        envelope: encoded,
    })
}

fn send_pending(
    service: &Service,
    config: &Config,
    pending: &PendingCall,
) -> Result<(), ServiceError> {
    let response = adapter(
        &config.sponsored_sender,
        json!({
            "chain_config": config.chain_config,
            "idempotency_key": hex(&pending.request),
            "calls": [{"to": "0x000000000000000000000000000000000000101d", "value": "0x0",
                "data": format!("0x{}", hex(&programs_call(config, service.binding.program, &pending.envelope)))}],
        }),
    )?;
    #[derive(Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Submission {
        transaction_hash: [u8; 32],
    }
    let submitted: Submission =
        serde_json::from_slice(&response).map_err(|_| ServiceError::NonCanonical)?;
    if submitted.transaction_hash == [0; 32] {
        return Err(ServiceError::ExecutionUnavailable);
    }
    Ok(())
}

fn maintenance_tick(service: &Service, config: &Config) -> Result<(), ServiceError> {
    // Reconciliation and cancellation continue even if the chain reader is unavailable.
    let now = now_ms()?;
    let expired_path = service.state_dir.join("expired-attempts.json");
    let mut expired: Vec<([u8; 32], u64)> = match fs::read(&expired_path) {
        Ok(bytes) => serde_json::from_slice(&bytes).map_err(|_| ServiceError::NonCanonical)?,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Vec::new(),
        Err(_) => return Err(ServiceError::ExecutionUnavailable),
    };
    for (key, state) in service.jobs.store().active()? {
        if state.state == JobState::Running && state.lease_until_ms <= now {
            if !expired.contains(&(key.digest().bytes(), state.fence)) {
                expired.push((key.digest().bytes(), state.fence));
                atomic_json(&expired_path, &expired)?;
            }
            service
                .jobs
                .runner()
                .cancel(key, state.fence)
                .map_err(|_| ServiceError::ExecutionUnavailable)?;
        }
    }
    service.jobs.reconcile(now)?;
    refresh_authority(service, config)?;
    let mut runtime = service
        .runtime
        .lock()
        .map_err(|_| ServiceError::ExecutionUnavailable)?;
    runtime.fresh()?;
    let authority = *runtime.membership.admitted_work()?;
    let current = runtime
        .current
        .as_ref()
        .ok_or(ServiceError::StaleAuthority)?;
    service.jobs.expire(&authority, now)?;
    let shared = decode_shared_state(&current.state)?;
    let record = authority.record();
    let replay = shared
        .control
        .replay
        .actor(ActorSlot::worker(usize::from(record.slot))?)
        .ok_or(ServiceError::AdmissionNotEffective)?;
    let sequence = replay.last.map_or(Ok(1), |last| {
        last.sequence.checked_add(1).ok_or(ServiceError::Overflow)
    })?;
    let pending_path = service.state_dir.join("program-operation.json");
    match fs::read(&pending_path) {
        Ok(bytes) => {
            let pending: PendingCall =
                serde_json::from_slice(&bytes).map_err(|_| ServiceError::NonCanonical)?;
            if replay
                .last
                .is_some_and(|last| last.request_id.bytes() == pending.request)
            {
                fs::remove_file(&pending_path).map_err(|_| ServiceError::ExecutionUnavailable)?;
            } else if pending.expiry <= authority.height() {
                fs::remove_file(&pending_path).map_err(|_| ServiceError::ExecutionUnavailable)?;
            } else {
                return send_pending(service, config, &pending);
            }
        }
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
        Err(_) => return Err(ServiceError::ExecutionUnavailable),
    }
    let mut operation = None;
    for digest in service.tracked()? {
        let key = JobKey::from_digest(Digest32::new(digest)?);
        let job = match service.jobs.store().find(key)? {
            Some(job) => job,
            None => continue,
        };
        let task = task_binding(&current.state, job.record.admission.task)?;
        match (job.state.state, task.status) {
            (JobState::Accepted, TaskStatus::Admitted) => {
                runtime.membership.new_work()?;
                let mut payload = shared.revision.to_be_bytes().to_vec();
                payload.extend_from_slice(task.task.as_bytes());
                payload.extend_from_slice(
                    acknowledgment_digest(&job.record.acknowledgment)?.as_bytes(),
                );
                operation = Some((dispatch::ACCEPT_TASK, payload));
                break;
            }
            (JobState::Accepted, TaskStatus::Accepted) => {
                runtime.membership.new_work()?;
                let lease = service.jobs.claim(key, &task, &authority, now)?;
                // Keep the timer and authority reader independent of a running model process.
                let store = JobStore::open(&service.state_dir.join("jobs"))?;
                let runner =
                    ProcessRunner::new(config.runner.clone(), Vec::new(), Duration::from_secs(5));
                let jobs = JobService::new(store, runner, service.key.clone());
                std::thread::spawn(move || {
                    if let Err(error) = jobs.run(lease, now) {
                        eprintln!("runner refused: {error}");
                    }
                });
            }
            (JobState::Ended(Outcome::Succeeded), TaskStatus::Accepted) => {
                if job.state.late || expired.contains(&(key.digest().bytes(), job.state.fence)) {
                    continue;
                }
                let Some(digest) = service.jobs.commit_record_or_retain(key, &authority)? else {
                    continue;
                };
                let mut payload = shared.revision.to_be_bytes().to_vec();
                payload.extend_from_slice(task.task.as_bytes());
                payload.extend_from_slice(digest.as_bytes());
                operation = Some((dispatch::COMMIT_TASK_RESULT, payload));
                break;
            }
            _ => {}
        }
    }
    if operation.is_none() {
        if let Some(meta) = runtime
            .membership
            .current()
            .and_then(|membership| membership.meta)
        {
            let epoch = authority
                .current_epoch()
                .ok_or(ServiceError::AdmissionNotEffective)?;
            if !meta.revoked() && meta.last_heartbeat_epoch != Some(epoch) {
                let mut payload = vec![1]; // F08 Worker participant tag.
                payload.extend_from_slice(service.binding.worker.as_bytes());
                for value in [meta.membership_generation, record.generation, epoch] {
                    payload.extend_from_slice(&value.to_be_bytes());
                }
                operation = Some((dispatch::Heartbeat, payload));
            }
        }
    }
    if let Some((operation, payload)) = operation {
        let pending = signed_operation(service, &authority, operation, sequence, &payload)?;
        atomic_json(&pending_path, &pending)?;
        drop(runtime);
        send_pending(service, config, &pending)?;
    }
    Ok(())
}

/// Reads one request with a `Content-Length` body of at most one envelope.
fn read_request(stream: &mut impl Read) -> Result<(String, String, Vec<u8>), Response> {
    let malformed = || refusal(ServiceError::NonCanonical, Presence::Absent);
    let mut head = Vec::new();
    let mut chunk = [0u8; 1024];
    let head_end = loop {
        if head.len() >= MAX_REQUEST_HEAD_BYTES {
            return Err(malformed());
        }
        let limit = chunk.len().min(MAX_REQUEST_HEAD_BYTES - head.len());
        let count = stream.read(&mut chunk[..limit]).map_err(|_| malformed())?;
        if count == 0 {
            return Err(malformed());
        }
        head.extend_from_slice(&chunk[..count]);
        if let Some(end) = head.windows(4).position(|bytes| bytes == b"\r\n\r\n") {
            break end + 4;
        }
    };
    let text = std::str::from_utf8(&head[..head_end]).map_err(|_| malformed())?;
    let mut lines = text.split("\r\n");
    let mut start = lines.next().unwrap_or_default().split(' ');
    let (Some(method), Some(path), Some("HTTP/1.1")) = (start.next(), start.next(), start.next())
    else {
        return Err(malformed());
    };
    let mut length = 0usize;
    for line in lines {
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-length") {
                length = value.trim().parse().map_err(|_| malformed())?;
            }
        }
    }
    if length > MAX_ENVELOPE_BYTES {
        return Err(refusal(ServiceError::InputTooLarge, Presence::Absent));
    }
    let mut body = vec![0; length];
    let buffered = &head[head_end..];
    let copied = buffered.len().min(length);
    body[..copied].copy_from_slice(&buffered[..copied]);
    stream
        .read_exact(&mut body[copied..])
        .map_err(|_| malformed())?;
    Ok((method.to_owned(), path.to_owned(), body))
}

fn reason(status: u16) -> &'static str {
    match status {
        200 => "OK",
        404 => "Not Found",
        413 => "Content Too Large",
        503 => "Service Unavailable",
        _ => "Bad Request",
    }
}

fn serve(
    service: &Service,
    tls: Arc<ServerConfig>,
    socket: TcpStream,
    deadline: Instant,
) -> std::io::Result<()> {
    let connection = ServerConnection::new(tls).map_err(std::io::Error::other)?;
    let mut stream = StreamOwned::new(connection, DeadlineSocket { socket, deadline });
    let response = match read_request(&mut stream) {
        Ok((method, path, body)) => service.dispatch(&method, &path, &body),
        Err(response) => response,
    };
    service.metrics.record(response.status);
    let head = format!(
        "HTTP/1.1 {} {}\r\nContent-Type: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        response.status,
        reason(response.status),
        response.content_type,
        response.body.len()
    );
    stream.write_all(head.as_bytes())?;
    stream.write_all(&response.body)?;
    stream.flush()?;
    stream.conn.send_close_notify();
    stream.conn.complete_io(&mut stream.sock)?;
    Ok(())
}

fn main() -> ExitCode {
    let mut args = std::env::args().skip(1);
    let (Some("--config"), Some(path), None) = (args.next().as_deref(), args.next(), args.next())
    else {
        eprintln!("usage: layerx-paxai-worker --config <file>");
        return ExitCode::from(2);
    };
    let (service, tls, config) = match load(&path) {
        Ok(loaded) => loaded,
        Err(Startup(reason)) => {
            eprintln!("refusing to start: {reason}");
            return ExitCode::FAILURE;
        }
    };
    let listener = match TcpListener::bind(&config.listen) {
        Ok(listener) => listener,
        Err(error) => {
            eprintln!("refusing to start: cannot listen: {error}");
            return ExitCode::FAILURE;
        }
    };
    match listener.local_addr() {
        Ok(addr) => println!("listening {addr}"),
        Err(error) => {
            eprintln!("refusing to start: {error}");
            return ExitCode::FAILURE;
        }
    }
    let service = Arc::new(service);
    let maintenance = Arc::clone(&service);
    std::thread::spawn(move || loop {
        let started = Instant::now();
        if let Err(error) = maintenance_tick(&maintenance, &config) {
            eprintln!("worker maintenance refused: {error}");
        }
        std::thread::sleep(Duration::from_secs(1).saturating_sub(started.elapsed()));
    });
    for socket in listener.incoming() {
        match socket {
            Ok(socket) => {
                let deadline = Instant::now() + REQUEST_DEADLINE;
                let Some(permit) = ConnectionPermit::acquire() else {
                    drop(socket);
                    continue;
                };
                let service = Arc::clone(&service);
                let tls = Arc::clone(&tls);
                std::thread::spawn(move || {
                    let _permit = permit;
                    if let Err(error) = serve(&service, tls, socket, deadline) {
                        eprintln!("connection closed: {}", error.kind());
                    }
                });
            }
            Err(error) => eprintln!("accept failed: {}", error.kind()),
        }
    }
    ExitCode::SUCCESS
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn request_limits_permit_refuses_beyond_cap() {
        let mut permits = Vec::new();
        for _ in 0..MAX_CONNECTIONS {
            permits.push(ConnectionPermit::acquire().expect("permit below connection cap"));
        }
        assert!(ConnectionPermit::acquire().is_none());
        drop(permits.pop().expect("held permit"));
        let replacement = ConnectionPermit::acquire().expect("released connection slot");
        assert!(ConnectionPermit::acquire().is_none());
        drop(replacement);
        drop(permits);
        assert_eq!(ACTIVE_CONNECTIONS.load(Ordering::Acquire), 0);
    }

    #[test]
    fn request_limits_slow_head_is_refused_at_deadline() {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind loopback listener");
        let mut client = TcpStream::connect(listener.local_addr().expect("listener address"))
            .expect("connect loopback client");
        let (socket, _) = listener.accept().expect("accept loopback client");
        client.write_all(b"P").expect("write partial request head");
        let started = Instant::now();
        let mut stream = DeadlineSocket {
            socket,
            deadline: started + Duration::from_millis(200),
        };
        let response = match read_request(&mut stream) {
            Err(response) => response,
            Ok(_) => panic!("incomplete request head accepted"),
        };
        assert!(started.elapsed() < Duration::from_secs(2));
        let expected = refusal(ServiceError::NonCanonical, Presence::Absent);
        assert_eq!(response.status, expected.status);
        assert_eq!(response.body, expected.body);
    }
}
