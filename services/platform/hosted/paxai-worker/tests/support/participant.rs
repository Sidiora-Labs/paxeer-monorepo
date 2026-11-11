// Same-store service/chain regression for audit worker-live-path-test-gap.
#[path = "../../src/worker_core.rs"]
mod worker;
include!("native_workload.rs");

use layerx_paxai_worker::auth::{CANCEL_PATH, QUERY_PATH, SUBMIT_PATH};
use layerx_paxai_worker::discovery::{spki_sha256, EndpointClient, NetworkProfile};
use layerx_paxai_worker::jobs::Lease;
use layerx_paxai_worker::membership::{MembershipObserver, Subject};
use rustls::{RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{IpAddr, Ipv4Addr, TcpListener, TcpStream};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread::JoinHandle;

struct Listener {
    service: Arc<worker::Service>,
    endpoint: Endpoint,
    client: EndpointClient,
    stopped: Arc<AtomicBool>,
    thread: Option<JoinHandle<()>>,
}
impl Drop for Listener {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        self.thread
            .take()
            .unwrap_or_else(|| panic!("listener thread"))
            .join()
            .unwrap_or_else(|error| panic!("{error:?}: {}", "listener stopped"));
    }
}
impl Listener {
    fn start(
        service: worker::Service,
        socket: TcpListener,
        tls: Arc<ServerConfig>,
        endpoint: Endpoint,
        roots: Arc<RootCertStore>,
    ) -> Self {
        socket
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("{error:?}: {}", "nonblocking accept"));
        let service = Arc::new(service);
        let consumer = Arc::clone(&service);
        let stopped = Arc::new(AtomicBool::new(false));
        let stop = Arc::clone(&stopped);
        let thread = std::thread::spawn(move || {
            while !stop.load(Ordering::Acquire) {
                let (socket, _) = match socket.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == io::ErrorKind::WouldBlock => {
                        std::thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                socket
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "read timeout"));
                socket
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "write timeout"));
                let connection = ServerConnection::new(Arc::clone(&tls))
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "TLS server"));
                let mut stream = StreamOwned::new(connection, socket);
                let mut reader = BufReader::new(&mut stream);
                let mut first = String::new();
                reader
                    .read_line(&mut first)
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "request line"));
                let fields: Vec<_> = first.split_whitespace().collect();
                assert_eq!(fields.len(), 3);
                assert_eq!(fields[2], "HTTP/1.1");
                let mut length = None;
                loop {
                    let mut line = String::new();
                    reader
                        .read_line(&mut line)
                        .unwrap_or_else(|error| panic!("{error:?}: {}", "header line"));
                    if line == "\r\n" {
                        break;
                    }
                    let (name, value) = line.split_once(':').unwrap_or_else(|| panic!("header"));
                    if name.eq_ignore_ascii_case("content-length") {
                        assert!(length.is_none());
                        length = Some(
                            value
                                .trim()
                                .parse::<usize>()
                                .unwrap_or_else(|error| panic!("{error:?}: {}", "body size")),
                        );
                    }
                }
                let mut body = vec![0; length.unwrap_or_else(|| panic!("content length"))];
                reader
                    .read_exact(&mut body)
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "binary service envelope"));
                drop(reader);
                // The production handler authenticates and decodes these exact TLS bytes,
                // reads staged inputs, tracks the job and writes this service's JobStore.
                let response = consumer.dispatch(fields[0], fields[1], &body);
                write!(stream, "HTTP/1.1 {} OK\r\nContent-Type: {}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", response.status, response.content_type, response.body.len()).unwrap_or_else(|error| panic!("{error:?}: {}", "response header"));
                stream
                    .write_all(&response.body)
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "binary response"));
                stream
                    .flush()
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "response flush"));
                stream.conn.send_close_notify();
                let _ = stream.conn.complete_io(&mut stream.sock);
            }
        });
        Self {
            service,
            endpoint,
            client: EndpointClient {
                profile: NetworkProfile::Private {
                    allowlist: vec![IpAddr::V4(Ipv4Addr::LOCALHOST)],
                },
                roots,
                timeout: Duration::from_secs(3),
            },
            stopped,
            thread: Some(thread),
        }
    }
    fn post(&self, path: &str, body: &[u8]) -> Vec<u8> {
        self.client
            .post(&self.endpoint, path, body, |_, _| {
                Ok(vec![IpAddr::V4(Ipv4Addr::LOCALHOST)])
            })
            .unwrap_or_else(|error| panic!("{error:?}: {}", "signed job through pinned TLS"))
    }
}

struct Certificates {
    tls: Arc<ServerConfig>,
    roots: Arc<RootCertStore>,
    endpoint: Endpoint,
}
fn certificates(socket: &TcpListener) -> Certificates {
    let ca_key =
        rcgen::KeyPair::generate().unwrap_or_else(|error| panic!("{error:?}: {}", "CA key"));
    let mut params = rcgen::CertificateParams::new(Vec::<String>::new())
        .unwrap_or_else(|error| panic!("{error:?}: {}", "CA parameters"));
    params.is_ca = rcgen::IsCa::Ca(rcgen::BasicConstraints::Unconstrained);
    let ca = params
        .self_signed(&ca_key)
        .unwrap_or_else(|error| panic!("{error:?}: {}", "CA certificate"));
    let issuer = rcgen::Issuer::from_params(&params, &ca_key);
    let key =
        rcgen::KeyPair::generate().unwrap_or_else(|error| panic!("{error:?}: {}", "server key"));
    let leaf = rcgen::CertificateParams::new(vec!["localhost".into()])
        .unwrap_or_else(|error| panic!("{error:?}: {}", "server parameters"))
        .signed_by(&key, &issuer)
        .unwrap_or_else(|error| panic!("{error:?}: {}", "server certificate"));
    let mut roots = RootCertStore::empty();
    roots
        .add(ca.der().clone())
        .unwrap_or_else(|error| panic!("{error:?}: {}", "root trust"));
    let tls = ServerConfig::builder()
        .with_no_client_auth()
        .with_single_cert(
            vec![leaf.der().clone()],
            rustls::pki_types::PrivateKeyDer::Pkcs8(rustls::pki_types::PrivatePkcs8KeyDer::from(
                key.serialize_der(),
            )),
        )
        .unwrap_or_else(|error| panic!("{error:?}: {}", "server TLS configuration"));
    Certificates {
        tls: Arc::new(tls),
        roots: Arc::new(roots),
        endpoint: endpoint(
            &format!(
                "https://localhost:{}/paxai/v1",
                socket
                    .local_addr()
                    .unwrap_or_else(|error| panic!("{error:?}: {}", "bound socket"))
                    .port()
            ),
            spki_sha256(leaf.der()).unwrap_or_else(|error| panic!("{error:?}: {}", "leaf pin")),
        ),
    }
}

fn open_service(directory: &Scratch, fixture: &Admitting) -> Checked<worker::Service> {
    let binding = fixture.authority.worker_binding();
    let membership = MembershipObserver::open(
        &directory.0,
        Subject {
            chain: binding.chain,
            program: binding.program,
            market: binding.market,
            worker: binding.worker,
        },
    )
    .map_err(|_| Failure::Unexpected("membership store"))?;
    let input_dir = directory.0.join("inputs");
    fs::create_dir_all(&input_dir)?;
    let service = worker::Service {
        binding,
        key: delegate(),
        metrics: layerx_paxai_worker::metrics::Metrics::default(),
        jobs: JobService::new(
            JobStore::open(&directory.store())?,
            ProcessRunner::new(std::env::current_exe()?, Vec::new(), Duration::from_secs(3)),
            delegate(),
        ),
        runtime: Mutex::new(worker::Runtime {
            membership,
            current: None,
            refreshed: None,
        }),
        state_dir: directory.0.clone(),
        input_dir,
    };
    assert_eq!(service.key.verifying_key(), delegate().verifying_key());
    assert_eq!(service.state_dir, directory.0);
    Ok(service)
}

// Committed in-memory authority peer: chunk production and capture, finality binding,
// membership decoding and metadata authentication all use the production types/codecs.
fn observe(service: &worker::Service, market: &Market, height: u64) -> Checked {
    let captured = view(&market.world.bytes, &market.roster, height)?;
    let finality = FinalityEvidence {
        native_state_root: captured.facts.proof.native_state_root,
        block_height: height,
        block_hash: captured.facts.proof.block_hash,
        app_hash: Digest32::new([0xC1; 32])?,
        store_proof_digest: Digest32::new([0xD3; 32])?,
        rank: 4,
    };
    let evidence = AuthorityEvidence {
        state: &captured.bytes,
        facts: &captured.facts,
        finality: &finality,
        head: PxNetworkHead {
            network_id: "paxeer".into(),
            paxeer: PxPaxeerHead {
                chain_id: 125,
                latest_block: height,
            },
            layerx: None,
            latest_committed_height: height,
        },
        owner: IdentityEvidence {
            principal: identities()?.1,
            primary_key: PublicKey32([0x0E; 32]),
            frozen: false,
            execution_height: height,
        },
        roster: Some(&market.roster),
        observed_ms: NOW,
    };
    service
        .runtime
        .lock()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "runtime"))
        .observe(&evidence, &market.signed, vec![customer_at(height)?])?;
    Ok(())
}
fn stage(service: &worker::Service, order: &Order) -> Checked {
    let name = hex_bytes(&order.submit.request.input_commitment.bytes());
    fs::write(
        service.input_dir.join(format!("{name}.manifest")),
        &order.manifest,
    )?;
    fs::write(
        service.input_dir.join(format!("{name}.payload")),
        &order.payload,
    )?;
    Ok(())
}

// Real producer -> loopback framed transport -> production native dispatcher. The peer
// applies the envelope emitted by signed_operation; it never constructs a replacement call.
fn sponsored(
    service: &worker::Service,
    market: &mut Market,
    operation: Operation,
    sequence: u64,
    task: TaskId,
    digest: Digest32,
    height: u64,
) -> Checked {
    let authority = authority_at(market, height)?;
    let mut payload = market.world.revision()?.to_be_bytes().to_vec();
    payload.extend_from_slice(task.as_bytes());
    payload.extend_from_slice(digest.as_bytes());
    let pending = worker::signed_operation(service, &authority, operation, sequence, &payload)?;
    let socket = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
    let address = socket.local_addr()?;
    let emitted = pending.envelope.clone();
    let sender = std::thread::spawn(move || {
        let mut stream = TcpStream::connect(address)
            .unwrap_or_else(|error| panic!("{error:?}: {}", "sponsored relay"));
        stream
            .write_all(
                &u32::try_from(emitted.len())
                    .unwrap_or_else(|error| panic!("{error:?}: envelope length"))
                    .to_be_bytes(),
            )
            .unwrap_or_else(|error| panic!("{error:?}: {}", "envelope length"));
        stream
            .write_all(&emitted)
            .unwrap_or_else(|error| panic!("{error:?}: {}", "signed envelope transport"));
    });
    let (mut stream, _) = socket.accept()?;
    let mut length = [0; 4];
    stream.read_exact(&mut length)?;
    let mut bytes = vec![0; u32::from_be_bytes(length) as usize];
    stream.read_exact(&mut bytes)?;
    sender
        .join()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "sender stopped"));
    assert_eq!(bytes, pending.envelope);
    let decoded = codec::decode_envelope(&bytes)?;
    assert_eq!(decoded.envelope.operation, operation);
    assert_eq!(decoded.envelope.actor, service.binding.owner);
    assert_eq!(decoded.envelope.request.bytes(), pending.request);
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; layerx_programs_ai_market::MAX_RESULT_BYTES];
    let routed = dispatch::route(
        &CallContext {
            chain: service.binding.chain,
            program: service.binding.program,
            principal: service.binding.owner,
            height,
        },
        &bytes,
        Some(&market.world.bytes),
        dispatch::Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let dispatch::Routed::Applied { state_len, .. } = routed else {
        panic!("sponsored native operation refused: {routed:?}");
    };
    next.truncate(state_len);
    market.world.bytes = next;
    Ok(())
}
fn task(market: &Market, id: TaskId) -> Checked<TaskBinding> {
    let state = decode_shared_state(&market.world.bytes)?;
    let policy = PolicySection::decode(state.section(Section::PolicyLifecycle)?)?;
    for binding in layerx_programs_ai_market::tasks::TaskSet::decode(policy.task_region)?.bindings()
    {
        let binding = binding?;
        if binding.task == id {
            return Ok(binding);
        }
    }
    Err(Failure::Unexpected("chain task missing"))
}

pub fn participant_tls_same_store_chain_commit_and_restart() {
    same_store_chain()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "same-store native chain proof"));
}
fn accept_order(
    server: &Listener,
    fixture: &mut Admitting,
    order: &Order,
) -> Checked<(Vec<u8>, TaskId, Lease, u64)> {
    let ack = server.post(SUBMIT_PATH, &order.signed);
    let admission = decode_acknowledgment(&ack, public(&delegate()))?.0;
    assert_eq!(admission.task, order.submit.request.task);
    assert_eq!(
        server
            .service
            .jobs
            .store()
            .job(order.key()?)?
            .record
            .acknowledgment,
        ack
    );
    assert_eq!(server.post(SUBMIT_PATH, &order.signed), ack);
    let query = order.reference(Route::Query, principal(9)?, 2, &customer())?;
    assert_eq!(server.post(QUERY_PATH, &query), ack);
    let now = server
        .service
        .jobs
        .store()
        .job(order.key()?)?
        .record
        .accepted_ms;
    // Local admission cannot claim work until this exact acknowledgment is finalized.
    assert_eq!(
        server.service.jobs.claim(
            order.key()?,
            &task(&fixture.market, admission.task)?,
            &fixture.authority,
            now
        ),
        Err(ServiceError::AdmissionNotEffective)
    );
    sponsored(
        &server.service,
        &mut fixture.market,
        dispatch::ACCEPT_TASK,
        1,
        admission.task,
        acknowledgment_digest(&ack)?,
        HEIGHT + 1,
    )?;
    let accepted = task(&fixture.market, admission.task)?;
    assert_eq!(
        (accepted.status, accepted.acknowledgement),
        (TaskStatus::Accepted, Some(acknowledgment_digest(&ack)?))
    );
    observe(&server.service, &fixture.market, HEIGHT + 1)?;
    let lease = {
        let runtime = server
            .service
            .runtime
            .lock()
            .unwrap_or_else(|error| panic!("{error:?}: {}", "runtime"));
        worker::claim_admitted(
            &server.service,
            &runtime.membership,
            order.key()?,
            &accepted,
            now + 1,
        )?
    };
    Ok((ack, admission.task, lease, now))
}

fn completed_report(
    order: &Order,
    fixture: &Admitting,
    lease: &Lease,
    now: u64,
) -> Checked<(Vec<u8>, Vec<u8>)> {
    let (output, output_manifest) = order.output(fixture, 32)?;
    // The runner peer supplies a recorded completion through the real runner codec. No
    // model binary, environment variable or subprocess toolchain is part of this gate.
    let report = Report {
        job: lease.key,
        fence: lease.fence,
        progress: Progress::Finished(Completion {
            succeeded: true,
            error_code: 0,
            start_ns: 1_000_000,
            finish_ns: 3_000_000,
            started_at_ms: now + 1,
            finished_at_ms: now + 3,
            segments: vec![
                Segment {
                    role: Role::Input,
                    unit_kind: TOKENS,
                    count: 16,
                },
                Segment {
                    role: Role::Output,
                    unit_kind: TOKENS,
                    count: 8,
                },
            ],
            output: output.clone(),
            output_manifest,
        }),
    }
    .encode()?;
    Ok((output, report))
}

fn same_store_chain() -> Checked {
    let directory = Scratch::new("participant-chain")?;
    let socket = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
    let restart_socket = socket.try_clone()?;
    let pki = certificates(&socket);
    let market = opened(vec![pki.endpoint.clone()])
        .unwrap_or_else(|error| panic!("{error:?}: {}", "create and open native market"));
    let authority = authority_at(&market, HEIGHT)?;
    let metadata = verified(&authority, &market.signed)?;
    let submit = submit(&authority, &capability(0x31))?;
    let mut fixture = Admitting {
        market,
        authority,
        metadata,
        submit,
    };
    let order = order(&fixture, 0x91, 1, 512)?;
    admit_chain_task(&mut fixture, &order, 0x91)
        .unwrap_or_else(|error| panic!("{error:?}: {}", "admit chain completion task"));
    let service = open_service(&directory, &fixture)?;
    observe(&service, &fixture.market, HEIGHT)?;
    stage(&service, &order)?;
    let server = Listener::start(
        service,
        socket,
        Arc::clone(&pki.tls),
        pki.endpoint.clone(),
        Arc::clone(&pki.roots),
    );
    let (ack, task_id, lease, now) = accept_order(&server, &mut fixture, &order)?;
    let (output, report) = completed_report(&order, &fixture, &lease, now)?;
    assert_eq!(
        server.service.jobs.complete(lease, &report, now + 3)?,
        JobState::Ended(Outcome::Succeeded)
    );
    let signed = server.post(
        QUERY_PATH,
        &order.reference(Route::Query, principal(9)?, 3, &customer())?,
    );
    let result = decode_result(&signed, public(&delegate()))?.0;
    assert_eq!(result.outcome, Outcome::Succeeded);
    assert_eq!(result.evidence, Some(evidence_digest(&report)?));
    assert_eq!(result.task, task_id);
    assert_eq!(
        (result.output_bytes, result.input_units, result.output_units),
        (32, 16, 8)
    );
    let digest = server
        .service
        .jobs
        .commit_record(order.key()?, &authority_at(&fixture.market, HEIGHT + 2)?)?;
    assert_eq!(digest, result_manifest_digest(&signed)?);
    sponsored(
        &server.service,
        &mut fixture.market,
        dispatch::COMMIT_TASK_RESULT,
        2,
        task_id,
        digest,
        HEIGHT + 2,
    )?;
    let committed = task(&fixture.market, task_id)?;
    assert_eq!(
        (committed.status, committed.result),
        (TaskStatus::ResultCommitted, Some(digest))
    );
    drop(server); // joins the listener and drops every Service/JobStore handle.
    fs::remove_dir_all(directory.0.join("inputs"))?;
    let socket = restart_socket;
    let reopened = open_service(&directory, &fixture)?;
    observe(&reopened, &fixture.market, HEIGHT + 3)?;
    let server = Listener::start(reopened, socket, pki.tls, pki.endpoint, pki.roots);
    assert_eq!(
        server.post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(9)?, 4, &customer())?
        ),
        signed
    );
    assert_eq!(server.post(SUBMIT_PATH, &order.signed), ack);
    assert_eq!(
        server.service.jobs.store().output(order.key()?)?,
        Some(output)
    );
    assert_eq!(
        server.service.jobs.store().job(order.key()?)?.state.fence,
        lease.fence
    );
    assert_eq!(
        server.service.jobs.store().payload(order.key()?)?,
        order.payload
    );
    assert_eq!(server.service.jobs.runner().invocations(), 0);
    Ok(())
}

pub fn participant_tls_cancel_survives_store_reopen() {
    same_store_cancel()
        .unwrap_or_else(|error| panic!("{error:?}: {}", "same-store cancellation proof"));
}
fn same_store_cancel() -> Checked {
    let directory = Scratch::new("participant-cancel")?;
    let socket = TcpListener::bind((Ipv4Addr::LOCALHOST, 0))?;
    let restart_socket = socket.try_clone()?;
    let pki = certificates(&socket);
    let market = opened(vec![pki.endpoint.clone()])
        .unwrap_or_else(|error| panic!("{error:?}: {}", "create and open native market"));
    let authority = authority_at(&market, HEIGHT)?;
    let metadata = verified(&authority, &market.signed)?;
    let submit = submit(&authority, &capability(0x31))?;
    let mut fixture = Admitting {
        market,
        authority,
        metadata,
        submit,
    };
    let order = order(&fixture, 0x92, 1, 512)?;
    admit_chain_task(&mut fixture, &order, 0x92)
        .unwrap_or_else(|error| panic!("{error:?}: {}", "admit chain cancellation task"));
    let service = open_service(&directory, &fixture)?;
    stage(&service, &order)?;
    observe(&service, &fixture.market, HEIGHT)?;
    let server = Listener::start(
        service,
        socket,
        Arc::clone(&pki.tls),
        pki.endpoint.clone(),
        Arc::clone(&pki.roots),
    );
    let ack = server.post(SUBMIT_PATH, &order.signed);
    let cancelled = server.post(
        CANCEL_PATH,
        &order.reference(Route::Cancel, principal(9)?, 2, &customer())?,
    );
    assert_eq!(cancelled, ack);
    let signed = server.post(
        QUERY_PATH,
        &order.reference(Route::Query, principal(9)?, 3, &customer())?,
    );
    let result = decode_result(&signed, public(&delegate()))?.0;
    assert_eq!(result.outcome, Outcome::CancelledBeforeStart);
    assert_eq!(result.task, order.submit.request.task);
    assert_eq!(result.output_bytes, 0);
    drop(server);
    fs::remove_dir_all(directory.0.join("inputs"))?;
    let socket = restart_socket;
    let service = open_service(&directory, &fixture)?;
    observe(&service, &fixture.market, HEIGHT + 1)?;
    let server = Listener::start(service, socket, pki.tls, pki.endpoint, pki.roots);
    assert_eq!(
        server.post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(9)?, 4, &customer())?
        ),
        signed
    );
    assert_eq!(
        server.post(
            CANCEL_PATH,
            &order.reference(Route::Cancel, principal(9)?, 5, &customer())?
        ),
        ack
    );
    assert_eq!(
        server.service.jobs.store().job(order.key()?)?.state.state,
        JobState::Ended(Outcome::CancelledBeforeStart)
    );
    assert_eq!(server.service.jobs.runner().invocations(), 0);
    Ok(())
}
