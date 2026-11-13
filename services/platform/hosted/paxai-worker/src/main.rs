//! PAXAI worker service: serves the signed status probe and authenticates every signed job
//! request against the worker's delegate-signed metadata binding.
//!
//! Signed jobs are admitted against the current finalized delegate and F08 membership.
//! Durable receipts precede sponsored task acceptance; execution waits for that acceptance
//! to finalize. The maintenance loop reconciles fenced leases and submits result commitments
//! and membership heartbeats through the programs precompile.
use ed25519_dalek::SigningKey;
use layerx_client::px::{PxNetworkHead, PxPaxeerHead};
use layerx_paxai_worker::{
    auth::{acknowledgment_digest, verify_signed_metadata, MetadataContext, ServiceError},
    discovery::spki_sha256,
    discovery::{AuthorityEvidence, FinalizedAuthority, IdentityEvidence},
    jobs::{JobService, Outcome},
    membership::{MembershipObserver, Subject},
    metrics::Metrics,
    runner::ProcessRunner,
    store::{JobKey, JobState, JobStore},
};
use layerx_programs_ai_market::{
    dispatch::{self, Operation},
    queries::{parse_hex32, FinalityEvidence, ReadProof, StateCapture},
    registry_ops::PolicySection,
    state::{decode_shared_state, ActorSlot, Section},
    tasks::{TaskSet, TaskStatus},
    types::{ChainDomain, Digest32, MarketId, PrincipalId, ProgramId, PublicKey32, WorkerId},
    MAX_STATE_BYTES,
};
use rustls::{
    pki_types::{pem::PemObject, CertificateDer, PrivateKeyDer},
    ServerConfig, ServerConnection, StreamOwned,
};
use serde::{Deserialize, Serialize};
use serde_json::json;
use std::fs::{self, File};
use std::io::Write;
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::process::{Command, ExitCode};
use std::sync::{Arc, Mutex};
use std::time::{Duration, Instant};

mod worker_core;
pub(crate) use worker_core::{
    claim_admitted, hex, now_ms, read_request, signed_operation, task_binding, ConnectionPermit,
    DeadlineSocket, PendingCall, Response, Runtime, Service,
};

const REQUEST_DEADLINE: Duration = Duration::from_secs(30);

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
    #[serde(rename = "chain_config")]
    chain_file: PathBuf,
    capabilities: Vec<u8>,
    access_declaration: Vec<u8>,
    response_capacity: u32,
    budget: [u64; 7],
    fee_limit: u128,
}

/// One startup refusal; it never carries key material or file contents.
struct Startup(&'static str);

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

impl Service {
    pub(crate) fn cleanup_retention(
        &self,
        state: &[u8],
        now: u64,
        height: u64,
    ) -> Result<usize, ServiceError> {
        let shared = decode_shared_state(state)?;
        let policy = PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
        let mut settled = Vec::new();
        for task in TaskSet::decode(policy.task_region)?.bindings() {
            let task = task?;
            if matches!(
                task.status,
                TaskStatus::ResultCommitted | TaskStatus::Cancelled
            ) {
                settled.push(task.task);
            }
        }
        self.jobs.purge_settled(now, height, &settled)
    }

    pub(crate) fn maintain_fences(&self, now: u64) -> Result<(), ServiceError> {
        // Migrate legacy vector files once; the job record is the durable per-key index.
        let path = self.state_dir.join("expired-attempts.json");
        match fs::read(&path) {
            Ok(bytes) => {
                let expired: Vec<([u8; 32], u64)> =
                    serde_json::from_slice(&bytes).map_err(|_| ServiceError::NonCanonical)?;
                for (digest, fence) in expired {
                    let key = JobKey::from_digest(Digest32::new(digest)?);
                    match self.jobs.store().job(key) {
                        Ok(_) => self.jobs.fence_expired(key, fence, now)?,
                        Err(ServiceError::NotFound) => {}
                        Err(error) => return Err(error),
                    }
                }
                fs::remove_file(&path).map_err(|_| ServiceError::ExecutionUnavailable)?;
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(_) => return Err(ServiceError::ExecutionUnavailable),
        }
        let legacy = self.state_dir.join("chain-jobs.json");
        match fs::remove_file(legacy) {
            Ok(()) => {}
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(_) => return Err(ServiceError::ExecutionUnavailable),
        }
        for (key, state) in self.jobs.store().active()? {
            if state.state == JobState::Running && state.lease_until_ms <= now {
                self.jobs.fence_expired(key, state.fence, now)?;
                self.jobs
                    .runner()
                    .cancel(key, state.fence)
                    .map_err(|_| ServiceError::ExecutionUnavailable)?;
            }
        }
        Ok(())
    }
}

/// Process adapters receive JSON on stdin and answer JSON on stdout. The authority reader
/// supplies native verified, same-height chunk proofs and identity facts from `chain_config`;
/// the sponsored sender signs and submits the exact ABI call, preserving its idempotency key.
fn adapter(
    program: &std::path::Path,
    request: &serde_json::Value,
) -> Result<Vec<u8>, ServiceError> {
    adapter_call(&mut Command::new(program), request, Duration::from_secs(10))
}

pub(crate) fn adapter_call(
    command: &mut Command,
    request: &serde_json::Value,
    timeout: Duration,
) -> Result<Vec<u8>, ServiceError> {
    let input = serde_json::to_vec(&request).map_err(|_| ServiceError::NonCanonical)?;
    let (status, bytes) =
        layerx_paxai_worker::process::invoke(command, &input, MAX_STATE_BYTES * 8, timeout)
            .map_err(|_| ServiceError::ExecutionUnavailable)?;
    if !status.success() {
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
            &json!({
                "chain_config": config.chain_file, "finality": "finalized",
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
            head: PxNetworkHead {
                network_id: "paxeer".to_owned(),
                paxeer: PxPaxeerHead {
                    chain_id: 125,
                    latest_block: observation.sealed_height,
                },
                layerx: None,
                latest_committed_height: observation.sealed_height,
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
        runtime.observe(&evidence, &observation.signed_metadata, identities)?;
        let delegate = runtime
            .membership
            .current()
            .ok_or(ServiceError::StaleAuthority)?
            .authority
            .worker_binding()
            .delegate;
        let seed = fs::read_to_string(&config.delegate_seed)
            .map_err(|_| ServiceError::ExecutionUnavailable)?;
        let key = SigningKey::from_bytes(&hex32(&seed).map_err(|_| ServiceError::NonCanonical)?);
        if PublicKey32(key.verifying_key().to_bytes()) != delegate {
            return Err(ServiceError::WrongGeneration);
        }
        service.jobs.store().retain_signing_key(&key)
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

fn send_pending(
    service: &Service,
    config: &Config,
    pending: &PendingCall,
) -> Result<(), ServiceError> {
    #[derive(Deserialize)]
    #[serde(deny_unknown_fields)]
    struct Submission {
        transaction_hash: [u8; 32],
    }
    let response = adapter(
        &config.sponsored_sender,
        &json!({
            "chain_config": config.chain_file,
            "idempotency_key": hex(&pending.request),
            "calls": [{"to": "0x000000000000000000000000000000000000101d", "value": "0x0",
                "data": format!("0x{}", hex(&programs_call(config, service.binding.program, &pending.envelope)))}],
        }),
    )?;
    let submitted: Submission =
        serde_json::from_slice(&response).map_err(|_| ServiceError::NonCanonical)?;
    if submitted.transaction_hash == [0; 32] {
        return Err(ServiceError::ExecutionUnavailable);
    }
    Ok(())
}

fn resume_pending(
    service: &Service,
    config: &Config,
    path: &std::path::Path,
    last_request: Option<[u8; 32]>,
    height: u64,
) -> Result<bool, ServiceError> {
    match fs::read(path) {
        Ok(bytes) => {
            let pending: PendingCall =
                serde_json::from_slice(&bytes).map_err(|_| ServiceError::NonCanonical)?;
            if last_request == Some(pending.request) || pending.expiry <= height {
                fs::remove_file(path).map_err(|_| ServiceError::ExecutionUnavailable)?;
                Ok(false)
            } else {
                send_pending(service, config, &pending)?;
                Ok(true)
            }
        }
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(false),
        Err(_) => Err(ServiceError::ExecutionUnavailable),
    }
}

fn heartbeat_operation(
    service: &Service,
    runtime: &Runtime,
    authority: &FinalizedAuthority,
) -> Result<Option<(Operation, Vec<u8>)>, ServiceError> {
    let Some(meta) = runtime
        .membership
        .current()
        .and_then(|membership| membership.meta)
    else {
        return Ok(None);
    };
    let epoch = authority
        .current_epoch()
        .ok_or(ServiceError::AdmissionNotEffective)?;
    if meta.revoked() || meta.last_heartbeat_epoch == Some(epoch) {
        return Ok(None);
    }
    let mut payload = vec![1]; // F08 Worker participant tag.
    payload.extend_from_slice(service.binding.worker.as_bytes());
    for value in [
        meta.membership_generation,
        authority.record().generation,
        epoch,
    ] {
        payload.extend_from_slice(&value.to_be_bytes());
    }
    Ok(Some((dispatch::Heartbeat, payload)))
}

fn maintenance_tick(service: &Service, config: &Config) -> Result<(), ServiceError> {
    // Reconciliation and cancellation continue even if the chain reader is unavailable.
    let now = now_ms()?;
    service.maintain_fences(now)?;
    service.jobs.reconcile(now)?;
    refresh_authority(service, config)?;
    let mut runtime = service
        .runtime
        .lock()
        .map_err(|_| ServiceError::ExecutionUnavailable)?;
    runtime.fresh();
    let authority = *runtime.membership.admitted_work()?;
    let current = runtime
        .current
        .as_ref()
        .ok_or(ServiceError::StaleAuthority)?;
    service.jobs.expire(&authority, now)?;
    service.cleanup_retention(&current.state, now, authority.height())?;
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
    if resume_pending(
        service,
        config,
        &pending_path,
        replay.last.map(|last| last.request_id.bytes()),
        authority.height(),
    )? {
        return Ok(());
    }
    let mut operation = None;
    for key in service.jobs.store().tracked()? {
        let Some(job) = service.jobs.store().find(key)? else {
            continue;
        };
        let task = task_binding(&current.state, job.record.admission.task)?;
        match (job.state.state, task.status) {
            (JobState::Accepted, TaskStatus::Admitted) => {
                runtime.membership.admitted_work()?;
                let mut payload = shared.revision.to_be_bytes().to_vec();
                payload.extend_from_slice(task.task.as_bytes());
                payload.extend_from_slice(
                    acknowledgment_digest(&job.record.acknowledgment)?.as_bytes(),
                );
                operation = Some((dispatch::ACCEPT_TASK, payload));
                break;
            }
            (JobState::Accepted, TaskStatus::Accepted) => {
                let lease = claim_admitted(service, &runtime.membership, key, &task, now)?;
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
                if job.state.late {
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
        operation = heartbeat_operation(service, &runtime, &authority)?;
    }
    if let Some((operation, payload)) = operation {
        let pending = signed_operation(service, &authority, operation, sequence, &payload)?;
        atomic_json(&pending_path, &pending)?;
        drop(runtime);
        send_pending(service, config, &pending)?;
    }
    Ok(())
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

pub(crate) fn serve(
    service: &Service,
    tls: Arc<ServerConfig>,
    socket: TcpStream,
    deadline: Instant,
) -> std::io::Result<()> {
    let connection = ServerConnection::new(tls).map_err(std::io::Error::other)?;
    let mut stream = StreamOwned::new(connection, DeadlineSocket { socket, deadline });
    let response: Response = match read_request(&mut stream) {
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
