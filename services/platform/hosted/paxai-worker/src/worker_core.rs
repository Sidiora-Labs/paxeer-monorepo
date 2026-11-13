use ed25519_dalek::{Signer, SigningKey};
use layerx_paxai_worker::{
    auth::{
        decode_service, sign_status, verify_request, verify_signed_metadata, AuthenticatedRequest,
        EvaluationAccess, JobReference, Route, ServiceError, StatusChallenge, WorkerBinding,
        STATUS_PATH,
    },
    discovery::{AuthorityEvidence, IdentityEvidence, VerifiedMetadata, READINESS_TTL_MS},
    jobs::{AdmissionView, Input, JobService},
    membership::MembershipObserver,
    metrics::Metrics,
    store::JobKey,
};
use layerx_programs_ai_market::{
    codec::{self, Envelope},
    dispatch::Operation,
    registry_ops::PolicySection,
    state::{decode_shared_state, Section},
    tasks::{TaskBinding, TaskSet},
    types::{Authentication, MarketId, Presence, PrincipalId, RequestId, Signature64, TaskId},
    MAX_ENVELOPE_BYTES,
};
use serde::{Deserialize, Serialize};
use std::fs::{self, File};
use std::io::{Read, Write};
use std::net::TcpStream;
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Mutex;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

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
    pub(crate) fn invalidate(&mut self) {
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

    pub(crate) fn fresh(&mut self) {
        if self
            .refreshed
            .is_none_or(|at| at.elapsed() >= Duration::from_millis(READINESS_TTL_MS))
        {
            self.invalidate();
        }
    }
}

pub(crate) struct Response {
    pub(crate) status: u16,
    pub(crate) body: Vec<u8>,
    pub(crate) content_type: &'static str,
}

pub(crate) fn refusal(error: ServiceError, request: Presence<RequestId>) -> Response {
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
        let signed = (|| {
            let challenge = StatusChallenge::decode(body)?;
            let mut runtime = self
                .runtime
                .lock()
                .map_err(|_| ServiceError::ExecutionUnavailable)?;
            runtime.fresh();
            let authority = &runtime
                .membership
                .current()
                .ok_or(ServiceError::StaleAuthority)?
                .authority;
            let binding = authority.worker_binding();
            let key = self.jobs.signing_key(binding.delegate)?;
            sign_status(&binding, &challenge, &key)
        })();
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
        let answer = (|| {
            let mut runtime = self
                .runtime
                .lock()
                .map_err(|_| ServiceError::ExecutionUnavailable)?;
            runtime.fresh();
            let authority = *runtime.membership.admitted_work()?;
            let binding = if route == Route::Submit {
                let key = JobKey::derive(
                    envelope.context.market,
                    envelope.context.actor,
                    envelope.context.request,
                )?;
                match self.jobs.store().find(key)? {
                    Some(saved) => saved.record.admission.worker_binding(),
                    None => runtime.membership.new_work()?.worker_binding(),
                }
            } else {
                authority.worker_binding()
            };
            let authenticated = verify_request(&envelope, route, &binding)?;
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
                    if let Some(receipt) = self.jobs.replay_submit(body, &authority, actor)? {
                        return Ok(receipt.acknowledgment);
                    }
                    let task = task_binding(&current.state, submit.task)?;
                    let name = hex(submit.input_commitment.as_bytes());
                    let manifest = read_bounded(
                        &self.input_dir.join(format!("{name}.manifest")),
                        layerx_programs_ai_market::evidence::MAX_MANIFEST_BYTES,
                    )?;
                    let payload = read_bounded(
                        &self.input_dir.join(format!("{name}.payload")),
                        layerx_paxai_worker::store::MAX_ENCRYPTED_BYTES,
                    )?;
                    let access_path = self.input_dir.join(format!(
                        "{}.access",
                        hex(submit.evaluation_access.as_bytes())
                    ));
                    match fs::metadata(&access_path) {
                        Ok(_) => {
                            let access = read_bounded(&access_path, EvaluationAccess::MAX_BYTES)?;
                            self.jobs
                                .store()
                                .retain_evaluation_access(submit.evaluation_access, &access)?;
                        }
                        // No access proof means no evaluator release; customer access is independent.
                        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
                        Err(_) => return Err(ServiceError::ExecutionUnavailable),
                    }
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
                AuthenticatedRequest::Query(reference) => {
                    let granted =
                        self.query_grants(envelope.context.market, actor.principal, &reference)?;
                    let answer = self
                        .jobs
                        .query(body, &authority, actor, &granted, now_ms()?)?;
                    Ok(answer.result.unwrap_or(answer.acknowledgment))
                }
                AuthenticatedRequest::Cancel(_) => {
                    let cancelled = self
                        .jobs
                        .cancel_record(body, &authority, actor, now_ms()?)?;
                    Ok(cancelled.acknowledgment)
                }
            }
        })();
        job_response(answer, request)
    }

    fn query_grants(
        &self,
        market: MarketId,
        actor: PrincipalId,
        reference: &JobReference,
    ) -> Result<Vec<PrincipalId>, ServiceError> {
        let mut granted = Vec::new();
        for job in self.jobs.store().by_request(market, reference.original)? {
            let admission = job.record.admission;
            if actor != admission.context.actor
                && (
                    reference.task,
                    reference.generation,
                    reference.key_version,
                    reference.metadata_revision,
                ) == (
                    admission.task,
                    admission.generation,
                    admission.key_version,
                    admission.metadata_revision,
                )
            {
                let request = job.record.request()?;
                let access = self
                    .jobs
                    .store()
                    .evaluation_access(request.evaluation_access)?;
                granted.extend_from_slice(access.principals());
            }
        }
        Ok(granted)
    }
}

fn job_response(answer: Result<Vec<u8>, ServiceError>, request: Presence<RequestId>) -> Response {
    match answer {
        Ok(body) => Response {
            status: 200,
            body,
            content_type: "application/octet-stream",
        },
        Err(error) => refusal(error, request),
    }
}

pub(crate) fn now_ms() -> Result<u64, ServiceError> {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .ok()
        .and_then(|time| u64::try_from(time.as_millis()).ok())
        .ok_or(ServiceError::ExecutionUnavailable)
}

pub(crate) fn hex(bytes: &[u8]) -> String {
    use std::fmt::Write as _;
    bytes.iter().fold(String::new(), |mut out, byte| {
        let _ = write!(out, "{byte:02x}");
        out
    })
}

pub(crate) fn read_bounded(path: &std::path::Path, limit: usize) -> Result<Vec<u8>, ServiceError> {
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

pub(crate) fn task_binding(state: &[u8], task: TaskId) -> Result<TaskBinding, ServiceError> {
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
    let key = service.jobs.signing_key(binding.delegate)?;
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
        signature: Signature64(key.sign(digest.as_bytes()).to_bytes()),
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

pub(crate) fn claim_admitted(
    service: &Service,
    membership: &MembershipObserver,
    key: JobKey,
    task: &TaskBinding,
    now: u64,
) -> Result<layerx_paxai_worker::jobs::Lease, ServiceError> {
    let authority = membership.admitted_work()?;
    service.jobs.claim(key, task, authority, now)
}

const MAX_CONNECTIONS: usize = 128;
static ACTIVE_CONNECTIONS: AtomicUsize = AtomicUsize::new(0);
const MAX_REQUEST_HEAD_BYTES: usize = 8_192;

pub(crate) struct ConnectionPermit;

impl ConnectionPermit {
    pub(crate) fn acquire() -> Option<Self> {
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

pub(crate) struct DeadlineSocket {
    pub(crate) socket: TcpStream,
    pub(crate) deadline: Instant,
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

/// Reads one request with a `Content-Length` body of at most one envelope.
pub(crate) fn read_request(stream: &mut impl Read) -> Result<(String, String, Vec<u8>), Response> {
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
    if start.next().is_some()
        || method.is_empty()
        || path.is_empty()
        || !path.bytes().all(|b| (33..=126).contains(&b))
    {
        return Err(malformed());
    }
    let length = layerx_paxai_worker::http::content_length(lines).ok_or_else(malformed)?;
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

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn request_limits_permit_refuses_beyond_cap() {
        let mut permits = Vec::new();
        for _ in 0..MAX_CONNECTIONS {
            permits.push(
                ConnectionPermit::acquire()
                    .unwrap_or_else(|| panic!("permit below connection cap")),
            );
        }
        assert!(ConnectionPermit::acquire().is_none());
        drop(permits.pop().unwrap_or_else(|| panic!("held permit")));
        let replacement =
            ConnectionPermit::acquire().unwrap_or_else(|| panic!("released connection slot"));
        assert!(ConnectionPermit::acquire().is_none());
        drop(replacement);
        drop(permits);
        assert_eq!(ACTIVE_CONNECTIONS.load(Ordering::Acquire), 0);
    }

    #[test]
    fn request_limits_slow_head_is_refused_at_deadline() {
        let listener = std::net::TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("{error:?}: {}", "bind loopback listener"));
        let mut client = TcpStream::connect(
            listener
                .local_addr()
                .unwrap_or_else(|error| panic!("{error:?}: {}", "listener address")),
        )
        .unwrap_or_else(|error| panic!("{error:?}: {}", "connect loopback client"));
        let (socket, _) = listener
            .accept()
            .unwrap_or_else(|error| panic!("{error:?}: {}", "accept loopback client"));
        client
            .write_all(b"P")
            .unwrap_or_else(|error| panic!("{error:?}: {}", "write partial request head"));
        let started = Instant::now();
        let mut stream = DeadlineSocket {
            socket,
            deadline: started + Duration::from_millis(200),
        };
        let Err(response) = read_request(&mut stream) else {
            panic!("incomplete request head accepted");
        };
        assert!(started.elapsed() < Duration::from_secs(2));
        let expected = refusal(ServiceError::NonCanonical, Presence::Absent);
        assert_eq!(response.status, expected.status);
        assert_eq!(response.body, expected.body);
    }
}
