//! Audit 22.30.84 repairs through the worker binary, pinned TLS and native finalized snapshots.
#![allow(dead_code, unused_imports)]

#[path = "../src/main.rs"]
mod worker;

include!("support/native_workload.rs");

fn worker_service(directory: &Scratch) -> Checked<worker::Service> {
    let binding = admitting()?.authority.worker_binding();
    let membership = layerx_paxai_worker::membership::MembershipObserver::open(
        &directory.0,
        layerx_paxai_worker::membership::Subject {
            chain: binding.chain,
            program: binding.program,
            market: binding.market,
            worker: binding.worker,
        },
    )
    .map_err(|_| Failure::Unexpected("membership store unavailable"))?;
    let input_dir = directory.0.join("inputs");
    fs::create_dir_all(&input_dir)?;
    Ok(worker::Service {
        binding,
        key: delegate(),
        metrics: Default::default(),
        jobs: service(
            &directory.store(),
            PathBuf::from(env!("CARGO_BIN_EXE_layerx-paxai-runner")),
        )?,
        runtime: std::sync::Mutex::new(worker::Runtime {
            membership,
            current: None,
            refreshed: None,
        }),
        state_dir: directory.0.clone(),
        input_dir,
    })
}

fn feed(service: &worker::Service, market: &Market, height: u64) -> Checked {
    let view = view(&market.world.bytes, &market.roster, height)?;
    let finality = FinalityEvidence {
        native_state_root: view.facts.proof.native_state_root,
        block_height: height,
        block_hash: view.facts.proof.block_hash,
        app_hash: Digest32::new([0xC1; 32])?,
        store_proof_digest: Digest32::new([0xD3; 32])?,
        rank: 4,
    };
    let evidence = AuthorityEvidence {
        state: &view.bytes,
        facts: &view.facts,
        finality: &finality,
        head: PxNetworkHead {
            network_id: "paxeer".to_owned(),
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
    service.runtime.lock().expect("runtime lock").observe(
        &evidence,
        &market.signed,
        vec![customer_at(height)?],
    )?;
    Ok(())
}

fn stage_input(service: &worker::Service, order: &Order) -> Checked {
    let name = order
        .submit
        .request
        .input_commitment
        .bytes()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect::<String>();
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

use layerx_paxai_worker::auth::{
    EvaluationAccess, StatusChallenge, CANCEL_PATH, QUERY_PATH, STATUS_PATH, SUBMIT_PATH,
};
use layerx_paxai_worker::discovery::{spki_sha256, EndpointClient, NetworkProfile, TransportError};
use std::io::{BufRead, BufReader};
use std::net::{IpAddr, Ipv4Addr, TcpListener};
use std::os::unix::fs::PermissionsExt;
use std::process::{Child, Command, Stdio};
use std::sync::Arc;
use std::time::{Instant, SystemTime, UNIX_EPOCH};

fn millis() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .expect("wall clock")
        .as_millis()
        .try_into()
        .expect("milliseconds")
}

struct TlsWorker {
    directory: Scratch,
    config: PathBuf,
    endpoint: Endpoint,
    client: EndpointClient,
    process: Option<Child>,
}

impl Drop for TlsWorker {
    fn drop(&mut self) {
        self.stop();
    }
}

impl TlsWorker {
    fn new(name: &str) -> Checked<(Self, Admitting)> {
        let directory = Scratch::new(name)?;
        let ca_key = rcgen::KeyPair::generate().expect("CA key");
        let mut ca_params =
            rcgen::CertificateParams::new(Vec::<String>::new()).expect("CA parameters");
        ca_params.is_ca = rcgen::IsCa::Ca(rcgen::BasicConstraints::Unconstrained);
        let ca = ca_params.self_signed(&ca_key).expect("CA certificate");
        let issuer = rcgen::Issuer::from_params(&ca_params, &ca_key);
        let leaf_key = rcgen::KeyPair::generate().expect("TLS key");
        let leaf = rcgen::CertificateParams::new(vec!["localhost".into()])
            .expect("leaf parameters")
            .signed_by(&leaf_key, &issuer)
            .expect("TLS certificate");
        let mut roots = rustls::RootCertStore::empty();
        roots.add(ca.der().clone()).expect("trusted root");
        let port = TcpListener::bind("127.0.0.1:0")?.local_addr()?.port();
        let endpoint = endpoint(
            &format!("https://localhost:{port}/paxai/v1"),
            spki_sha256(leaf.der()).expect("SPKI"),
        );
        let market = opened(vec![endpoint.clone()])?;
        let authority = authority_at(&market, HEIGHT)?;
        let metadata = verified(&authority, &market.signed)?;
        let submit = submit(&authority, &capability(0x31))?;
        let fixture = Admitting {
            market,
            authority,
            metadata,
            submit,
        };
        let file = |name: &str| directory.0.join(name);
        fs::write(file("chain.pem"), format!("{}{}", leaf.pem(), ca.pem()))?;
        fs::write(file("key.pem"), leaf_key.serialize_pem())?;
        fs::write(
            file("delegate.hex"),
            DELEGATE_SEED
                .iter()
                .map(|n| format!("{n:02x}"))
                .collect::<String>(),
        )?;
        fs::write(file("metadata.bin"), &fixture.market.signed)?;
        fs::create_dir_all(file("inputs"))?;
        // The external authority adapter reads a captured native snapshot. It performs no
        // service logic: the binary must decode all chunks and bind finalized authority.
        let reader = file("authority-reader");
        fs::write(
            &reader,
            format!(
                "#!/bin/sh\ncat >/dev/null\nexec cat '{}'\n",
                file("observation.json").display()
            ),
        )?;
        fs::set_permissions(&reader, fs::Permissions::from_mode(0o700))?;
        let text = |bytes: &[u8]| bytes.iter().map(|n| format!("{n:02x}")).collect::<String>();
        let (market, owner, worker) = identities()?;
        let config = file("worker.json");
        let value = serde_json::json!({
            "listen": format!("127.0.0.1:{port}"),
            "certificate_chain": file("chain.pem"), "private_key": file("key.pem"),
            "delegate_seed": file("delegate.hex"), "signed_metadata": file("metadata.bin"),
            "chain": text(&CHAIN), "program": text(&PROGRAM), "market": text(market.as_bytes()),
            "worker": text(worker.as_bytes()), "owner": text(owner.as_bytes()),
            "state_dir": file("state"), "input_dir": file("inputs"),
            "runner": env!("CARGO_BIN_EXE_layerx-paxai-runner"),
            "authority_reader": reader, "sponsored_sender": file("sender-unavailable"),
            "chain_config": file("chain.json"), "capabilities": [1], "access_declaration": [1],
            "response_capacity": 16384, "budget": [1, 1, 1, 1, 1, 1, 1], "fee_limit": 1
        });
        fs::write(
            &config,
            serde_json::to_vec(&value).expect("configuration JSON"),
        )?;
        let harness = Self {
            directory,
            config,
            endpoint,
            client: EndpointClient {
                profile: NetworkProfile::Private {
                    allowlist: vec![IpAddr::V4(Ipv4Addr::LOCALHOST)],
                },
                roots: Arc::new(roots),
                timeout: Duration::from_secs(3),
            },
            process: None,
        };
        harness.observe(&fixture.market, HEIGHT)?;
        Ok((harness, fixture))
    }

    fn jobs(&self) -> PathBuf {
        self.directory.0.join("state/jobs")
    }

    fn observe(&self, market: &Market, height: u64) -> Checked {
        let mut out = vec![0; CHUNK_RESPONSE_MAX];
        let n = read_state_chunk(&market.world.bytes, &chunk_payload(0, None, 0), &mut out)?;
        let first = codec::decode_chunk_response(&out[..n])?;
        let (revision, digest, total) = (first.revision, first.digest, first.total_bytes);
        let mut chunks = vec![out[..n].to_vec()];
        for offset in (8192..total).step_by(8192) {
            let n = read_state_chunk(
                &market.world.bytes,
                &chunk_payload(revision, Some(digest), offset),
                &mut out,
            )?;
            chunks.push(out[..n].to_vec());
        }
        let identity = |n: u8, key: PublicKey32| {
            serde_json::json!({
                "principal": principal(n).expect("principal").bytes(), "primary_key": key.0,
                "frozen": false, "execution_height": height,
            })
        };
        let observation = serde_json::json!({
            "chunks": chunks, "native_state_root": ROOT, "observed_sequence": 77,
            "execution_height": height, "block_height": height, "block_hash": ([0xBB; 32]),
            "app_hash": ([0xC1; 32]), "store_proof_digest": ([0xD3; 32]), "sealed_height": height,
            "owner": identity(1, PublicKey32([0x0E; 32])),
            "identities": [identity(9, public(&customer())), identity(8, public(&SigningKey::from_bytes(&[0xC8; 32]))),
                identity(2, public(&SigningKey::from_bytes(&[2; 32]))), identity(10, public(&SigningKey::from_bytes(&[10; 32])))],
            "roster": market.roster, "signed_metadata": market.signed,
        });
        let path = self.directory.0.join("observation.json");
        let pending = path.with_extension("pending");
        fs::write(
            &pending,
            serde_json::to_vec(&observation).expect("snapshot JSON"),
        )?;
        fs::rename(pending, path)?;
        Ok(())
    }

    fn start(&mut self, authority: &FinalizedAuthority) -> Checked {
        assert!(self.process.is_none());
        let log = fs::File::create(self.directory.0.join("worker.stderr"))?;
        let mut process = Command::new(env!("CARGO_BIN_EXE_layerx-paxai-worker"))
            .arg("--config")
            .arg(&self.config)
            .stdout(Stdio::piped())
            .stderr(log)
            .spawn()?;
        let mut first = String::new();
        BufReader::new(process.stdout.take().expect("worker stdout")).read_line(&mut first)?;
        self.process = Some(process);
        assert!(
            first.starts_with("listening "),
            "worker startup: {first}; {}",
            fs::read_to_string(self.directory.0.join("worker.stderr"))?
        );
        self.wait_current(authority);
        Ok(())
    }

    fn stop(&mut self) {
        if let Some(mut process) = self.process.take() {
            process.kill().expect("stop worker process");
            process.wait().expect("reap worker process");
        }
    }

    fn challenge(authority: &FinalizedAuthority) -> StatusChallenge {
        StatusChallenge {
            epoch: authority.current_epoch().expect("epoch"),
            config: authority.config(),
            roster: authority.roster(),
            expiry: TASK_EXPIRY,
            challenge: RequestId::new([0x5C; 32]).expect("challenge"),
        }
    }

    fn wait_current(&self, authority: &FinalizedAuthority) {
        let challenge = Self::challenge(authority);
        let until = Instant::now() + Duration::from_secs(10);
        loop {
            let response = self.post(STATUS_PATH, &challenge.encode());
            if let Ok(bytes) = &response {
                if layerx_paxai_worker::auth::verify_status(
                    bytes,
                    &authority.worker_binding(),
                    &challenge,
                )
                .is_ok()
                {
                    return;
                }
            }
            assert!(
                Instant::now() < until,
                "current signed status unavailable: {response:?}; {}",
                fs::read_to_string(self.directory.0.join("worker.stderr"))
                    .expect("worker diagnostics")
            );
            thread::sleep(Duration::from_millis(50));
        }
    }

    fn post(&self, path: &str, body: &[u8]) -> Result<Vec<u8>, TransportError> {
        self.client.post(&self.endpoint, path, body, |_, _| {
            Ok(vec![IpAddr::V4(Ipv4Addr::LOCALHOST)])
        })
    }

    fn stage(&self, order: &Order) -> Checked {
        let name = order
            .submit
            .request
            .input_commitment
            .bytes()
            .iter()
            .map(|n| format!("{n:02x}"))
            .collect::<String>();
        fs::write(
            self.directory.0.join(format!("inputs/{name}.manifest")),
            &order.manifest,
        )?;
        fs::write(
            self.directory.0.join(format!("inputs/{name}.payload")),
            &order.payload,
        )?;
        Ok(())
    }

    fn stage_access(&self, access: &EvaluationAccess) -> Checked {
        let name = access
            .commitment()?
            .bytes()
            .iter()
            .map(|n| format!("{n:02x}"))
            .collect::<String>();
        fs::write(
            self.directory.0.join(format!("inputs/{name}.access")),
            access.encode()?,
        )?;
        Ok(())
    }
}

fn apply_chain(market: &mut Market, signed: &[u8], actor: PrincipalId, at: u64) -> Checked {
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; layerx_programs_ai_market::MAX_RESULT_BYTES];
    let context = CallContext {
        chain: ChainDomain::new(CHAIN)?,
        program: ProgramId::new(PROGRAM)?,
        principal: actor,
        height: at,
    };
    let routed = dispatch::route(
        &context,
        signed,
        Some(&market.world.bytes),
        dispatch::Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let dispatch::Routed::Applied { state_len, .. } = routed else {
        let result_len = match routed {
            dispatch::Routed::Refused { result_len }
            | dispatch::Routed::Unchanged { result_len } => result_len,
            _ => unreachable!(),
        };
        panic!(
            "native transition refused: {:?}",
            codec::decode_result(&result[..result_len])
        );
    };
    next.truncate(state_len);
    market.world.bytes = next;
    Ok(())
}

fn accept_chain(
    market: &mut Market,
    order: &Order,
    ack: &[u8],
    sequence: u64,
) -> Checked<TaskBinding> {
    let authority = authority_at(market, HEIGHT + 2)?;
    let mut payload = market.world.revision()?.to_be_bytes().to_vec();
    payload.extend_from_slice(order.submit.request.task.as_bytes());
    payload.extend_from_slice(acknowledgment_digest(ack)?.as_bytes());
    let call = Call {
        operation: dispatch::ACCEPT_TASK,
        actor: principal(1)?,
        epoch: 1,
        config: authority.config(),
        roster: authority.roster(),
        sequence,
        request: 0xE0 + u8::try_from(sequence).expect("sequence"),
        payload: &payload,
    };
    apply_chain(market, &call.encode()?, principal(1)?, HEIGHT + 2)?;
    order.accepted(ack)
}

fn drain(market: &mut Market) -> Checked {
    let (_, owner, worker) = identities()?;
    market.world.edit(|parts, header| {
        parts
            .admission
            .request_exit(
                &admission_context(header, owner, HEIGHT),
                Participant::Worker(worker),
                1,
                2,
                layerx_programs_ai_market::admission::ExitReason::Voluntary,
            )
            .map(|_| ())
    })?;
    Ok(())
}

fn publish(market: &mut Market, manifest: &Manifest, key: &SigningKey, sequence: u64) -> Checked {
    let mut context = metadata_context()?;
    context.delegate = public(key);
    let bytes = manifest.encode()?;
    let mut publication = publication(&bytes, manifest.revision - 1)?;
    publication.sequence = sequence;
    publication.request = RequestId::new([0x70 + u8::try_from(sequence).expect("sequence"); 32])?;
    let signed = sign_metadata(&context, &publication, key)?;
    apply_chain(market, &signed, principal(1)?, HEIGHT + 1)?;
    market.signed = signed;
    Ok(())
}

fn order_for(fixture: &Admitting, request: u8, actor: u8, key: &SigningKey) -> Checked<Order> {
    let mut order = order(fixture, request, 1, 512)?;
    let task = codec::derive_task(identities()?.0, 1, principal(actor)?, [request; 32])?;
    order.manifest = object_manifest(
        ArtifactKind::Input,
        principal(actor)?,
        task,
        fixture.authority.policy(),
        &order.payload,
    )?;
    order.submit.context.actor = principal(actor)?;
    order.submit.request.task = task;
    order.submit.request.input_commitment = root(&order.manifest)?;
    order.submit.task.task = task;
    order.submit.task.requester = principal(actor)?;
    order.submit.task.input = order.submit.request.input_commitment;
    order.signed = order.submit.signed(key)?;
    Ok(order)
}

fn admit_order(fixture: &mut Admitting, order: &Order, nonce: u8) -> Checked {
    let authority = fixture.authority;
    let mut payload = 1u64.to_be_bytes().to_vec();
    payload.extend_from_slice(&authority.config().to_be_bytes());
    payload.extend_from_slice(authority.policy().as_bytes());
    let Presence::Present(roster) = authority.roster() else {
        panic!("frozen roster");
    };
    payload.extend_from_slice(roster.as_bytes());
    payload.extend_from_slice(order.submit.context.actor.as_bytes());
    payload.extend_from_slice(authority.record().worker.as_bytes());
    payload.extend_from_slice(fixture.metadata.digest().as_bytes());
    payload.extend_from_slice(&[nonce; 32]);
    payload.extend_from_slice(order.submit.request.input_commitment.as_bytes());
    payload.extend_from_slice(&TASK_EXPIRY.to_be_bytes());
    let call = Call {
        operation: dispatch::ADMIT_TASK,
        actor: order.submit.context.actor,
        epoch: 1,
        config: authority.config(),
        roster: authority.roster(),
        sequence: 0,
        request: nonce,
        payload: &payload,
    };
    apply_chain(&mut fixture.market, &call.encode()?, call.actor, HEIGHT)
}

#[test]
fn worker_drain_preserves_tls_redelivery_query_cancel_and_admitted_execution_after_restart(
) -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("drain")?;
    let first = order(&fixture, 0x81, 1, 512)?;
    let other = order(&fixture, 0x82, 2, 512)?;
    let new = order(&fixture, 0x83, 3, 512)?;
    for (order, nonce) in [(&first, 0x81), (&other, 0x82), (&new, 0x83)] {
        admit_order(&mut fixture, order, nonce)?;
        transport.stage(order)?;
    }
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let ack = transport
        .post(SUBMIT_PATH, &first.signed)
        .expect("first admission through TLS");
    let other_ack = transport
        .post(SUBMIT_PATH, &other.signed)
        .expect("second admission through TLS");
    transport.stop();
    drain(&mut fixture.market)?;
    let drained = authority_at(&fixture.market, HEIGHT + 1)?;
    assert_eq!(
        drained.admission_gate(),
        Err(ServiceError::AdmissionNotEffective)
    );
    transport.observe(&fixture.market, HEIGHT + 1)?;
    fs::remove_dir_all(transport.directory.0.join("inputs"))?;
    transport.start(&drained)?;
    assert_eq!(
        transport
            .post(SUBMIT_PATH, &first.signed)
            .expect("saved acknowledgment during drain"),
        ack
    );
    let query = first.reference(Route::Query, principal(9)?, 10, &customer())?;
    assert_eq!(
        transport
            .post(QUERY_PATH, &query)
            .expect("customer query during drain"),
        ack
    );
    assert_eq!(
        transport.post(SUBMIT_PATH, &new.signed),
        Err(TransportError::Service(ServiceError::AdmissionNotEffective))
    );
    let cancel = first.reference(Route::Cancel, principal(9)?, 11, &customer())?;
    assert_eq!(
        transport
            .post(CANCEL_PATH, &cancel)
            .expect("cancel during drain"),
        ack
    );
    let result = transport
        .post(
            QUERY_PATH,
            &first.reference(Route::Query, principal(9)?, 12, &customer())?,
        )
        .expect("cancel result");
    assert_eq!(
        decode_result(&result, public(&delegate()))?.0.outcome,
        Outcome::CancelledBeforeStart
    );
    transport.stop();
    let accepted = accept_chain(&mut fixture.market, &other, &other_ack, 1)?;
    // Drive the same admission/claim boundary used by maintenance on the reopened durable job.
    let service = worker_service(&transport.directory)?;
    let service = worker::Service {
        jobs: service_jobs(&transport, delegate())?,
        ..service
    };
    feed(&service, &fixture.market, HEIGHT + 2)?;
    let runtime = service.runtime.lock().expect("runtime");
    assert_eq!(
        runtime.membership.new_work(),
        Err(ServiceError::AdmissionNotEffective)
    );
    let job = service.jobs.store().job(other.key()?)?;
    let lease = worker::claim_admitted(
        &service,
        &runtime.membership,
        other.key()?,
        &accepted,
        job.record.accepted_ms + 1,
    )?;
    assert_eq!(
        service.jobs.store().job(lease.key)?.state.state,
        JobState::Running
    );
    Ok(())
}

fn service_jobs(transport: &TlsWorker, key: SigningKey) -> Checked<JobService> {
    Ok(JobService::new(
        JobStore::open(&transport.jobs())?,
        ProcessRunner::new(
            PathBuf::from(env!("CARGO_BIN_EXE_layerx-paxai-runner")),
            Vec::new(),
            Duration::from_secs(5),
        ),
        key,
    ))
}

#[test]
fn worker_current_metadata_refresh_accepts_tls_submit_and_signs_current_status() -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("metadata")?;
    let old = order(&fixture, 0x84, 1, 512)?;
    admit_order(&mut fixture, &old, 0x84)?;
    // F01 tasks are admitted against the epoch's frozen metadata before its refresh.
    let next_task = order(&fixture, 0x85, 2, 512)?;
    admit_order(&mut fixture, &next_task, 0x85)?;
    transport.stage(&old)?;
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let old_ack = transport
        .post(SUBMIT_PATH, &old.signed)
        .expect("revision one admission");
    let replacement = manifest(2, vec![capability(0x41)], vec![transport.endpoint.clone()])?;
    publish(&mut fixture.market, &replacement, &delegate(), 1)?;
    fixture.authority = authority_at(&fixture.market, HEIGHT + 1)?;
    fixture.metadata = verified(&fixture.authority, &fixture.market.signed)?;
    fixture.submit = submit(&fixture.authority, &capability(0x41))?;
    let new = order(&fixture, 0x85, 2, 512)?;
    assert_eq!(new.submit.request.task, next_task.submit.request.task);
    assert_eq!(
        new.submit.request.input_commitment,
        next_task.submit.request.input_commitment
    );
    transport.stage(&new)?;
    transport.observe(&fixture.market, HEIGHT + 1)?;
    transport.wait_current(&fixture.authority);
    let ack = transport
        .post(SUBMIT_PATH, &new.signed)
        .expect("current revision admission");
    let (current, _) = decode_acknowledgment(&ack, public(&delegate()))?;
    assert_eq!(current.metadata, fixture.metadata.digest());
    assert_eq!(current.model, Digest32::new([0x41; 32])?);
    assert_eq!(
        transport
            .post(SUBMIT_PATH, &old.signed)
            .expect("old revision replay"),
        old_ack
    );
    let (saved, _) = decode_acknowledgment(&old_ack, public(&delegate()))?;
    assert_ne!(saved.metadata, current.metadata);
    let stale = new.submit.with(|s| {
        s.context.request = RequestId::new([0x86; 32]).expect("request");
        s.request.metadata_revision = 1;
    });
    assert_eq!(
        transport.post(SUBMIT_PATH, &stale.signed(&customer())?),
        Err(TransportError::Service(ServiceError::WrongRevision))
    );
    Ok(())
}

#[test]
fn worker_verified_evaluator_access_reaches_tls_result_consumer_after_restart() -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("access")?;
    let mut order = order(&fixture, 0x87, 1, 512)?;
    let access = EvaluationAccess::new(vec![principal(2)?])?;
    order.submit.request.evaluation_access = access.commitment()?;
    order.signed = order.submit.signed(&customer())?;
    admit_order(&mut fixture, &order, 0x87)?;
    transport.stage(&order)?;
    transport.stage_access(&access)?;
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let ack = transport
        .post(SUBMIT_PATH, &order.signed)
        .expect("grant-bound admission");
    let evaluator = SigningKey::from_bytes(&[2; 32]);
    let outsider = SigningKey::from_bytes(&[10; 32]);
    assert_eq!(
        transport
            .post(
                QUERY_PATH,
                &order.reference(Route::Query, principal(2)?, 1, &evaluator)?
            )
            .expect("granted query"),
        ack
    );
    assert_eq!(
        transport.post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(10)?, 1, &outsider)?
        ),
        Err(TransportError::Service(ServiceError::AccessDenied))
    );
    assert_eq!(
        transport.post(
            CANCEL_PATH,
            &order.reference(Route::Cancel, principal(2)?, 2, &evaluator)?
        ),
        Err(TransportError::Service(ServiceError::AccessDenied))
    );
    transport
        .post(
            CANCEL_PATH,
            &order.reference(Route::Cancel, principal(9)?, 2, &customer())?,
        )
        .expect("customer cancellation");
    let customer_result = transport
        .post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(9)?, 3, &customer())?,
        )
        .expect("customer result");
    assert_eq!(
        decode_result(&customer_result, public(&delegate()))?
            .0
            .outcome,
        Outcome::CancelledBeforeStart
    );
    transport.stop();
    fs::remove_dir_all(transport.directory.0.join("inputs"))?;
    transport.start(&fixture.authority)?;
    assert_eq!(
        transport
            .post(
                QUERY_PATH,
                &order.reference(Route::Query, principal(2)?, 3, &evaluator)?
            )
            .expect("recovered evaluator grant"),
        customer_result
    );
    assert_eq!(
        transport.post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(10)?, 2, &outsider)?
        ),
        Err(TransportError::Service(ServiceError::AccessDenied))
    );
    // The grant commitment is checked on every release, including after storage damage.
    let name = access
        .commitment()?
        .bytes()
        .iter()
        .map(|n| format!("{n:02x}"))
        .collect::<String>();
    let forged = EvaluationAccess::new(vec![principal(10)?])?;
    fs::write(
        transport.jobs().join(format!("evaluation-access/{name}")),
        forged.encode()?,
    )?;
    assert_eq!(
        transport.post(
            QUERY_PATH,
            &order.reference(Route::Query, principal(10)?, 3, &outsider)?
        ),
        Err(TransportError::Service(
            ServiceError::MetadataIntegrityFailure
        ))
    );
    Ok(())
}

#[test]
fn worker_cancel_returns_exact_customer_acknowledgment_for_shared_request_id() -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("cancel")?;
    let first = order_for(&fixture, 0x89, 9, &customer())?;
    let second_key = SigningKey::from_bytes(&[0xC8; 32]);
    let second = order_for(&fixture, 0x89, 8, &second_key)?;
    assert_eq!(first.submit.context.request, second.submit.context.request);
    assert_ne!(first.key()?, second.key()?);
    admit_order(&mut fixture, &first, 0x89)?;
    admit_order(&mut fixture, &second, 0x89)?;
    transport.stage(&first)?;
    transport.stage(&second)?;
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let first_ack = transport
        .post(SUBMIT_PATH, &first.signed)
        .expect("customer one admission");
    let second_ack = transport
        .post(SUBMIT_PATH, &second.signed)
        .expect("customer two admission");
    assert_ne!(first_ack, second_ack);
    let cancel = second.reference(Route::Cancel, principal(8)?, 2, &second_key)?;
    assert_eq!(
        transport
            .post(CANCEL_PATH, &cancel)
            .expect("customer two cancellation"),
        second_ack
    );
    let store = JobStore::open(&transport.jobs())?;
    assert_eq!(store.job(first.key()?)?.state.state, JobState::Accepted);
    assert_eq!(
        store.job(second.key()?)?.state.state,
        JobState::Ended(Outcome::CancelledBeforeStart)
    );
    assert_eq!(
        transport
            .post(
                CANCEL_PATH,
                &first.reference(Route::Cancel, principal(9)?, 2, &customer())?
            )
            .expect("customer one cancellation"),
        first_ack
    );
    Ok(())
}

#[test]
fn worker_delegate_value_rotation_recovers_old_job_completion_under_original_identity() -> Checked {
    let (mut transport, mut fixture) = TlsWorker::new("rotation")?;
    let old = order(&fixture, 0x8A, 1, 512)?;
    let expiring = order(&fixture, 0x8B, 2, 512)?;
    admit_order(&mut fixture, &old, 0x8A)?;
    admit_order(&mut fixture, &expiring, 0x8B)?;
    transport.stage(&old)?;
    transport.stage(&expiring)?;
    transport.observe(&fixture.market, HEIGHT)?;
    transport.start(&fixture.authority)?;
    let ack = transport
        .post(SUBMIT_PATH, &old.signed)
        .expect("old admission");
    transport
        .post(SUBMIT_PATH, &expiring.signed)
        .expect("second old admission");
    let rotated = SigningKey::from_bytes(&[0xD2; 32]);
    let mut replacement = manifest(1, vec![capability(0x31)], vec![transport.endpoint.clone()])?;
    replacement.generation = 2;
    replacement.key_version = 2;
    let (_, digest) = Manifest::decode(&replacement.encode()?)?;
    let record = *fixture.authority.record();
    let header = fixture.market.world.header()?;
    let consent = layerx_programs_ai_market::workers::consent_digest(
        &header,
        record.worker,
        record.owner,
        public(&rotated),
        2,
        2,
        digest,
        EXPIRY,
    )?;
    let mut payload = record.worker.as_bytes().to_vec();
    payload.extend_from_slice(&1u64.to_be_bytes());
    payload.extend_from_slice(&1u64.to_be_bytes());
    payload.extend_from_slice(&public(&rotated).0);
    payload.extend_from_slice(digest.as_bytes());
    payload.extend_from_slice(&EXPIRY.to_be_bytes());
    payload.extend_from_slice(&rotated.sign(consent.as_bytes()).to_bytes());
    let call = Call {
        operation: dispatch::RotateDelegate,
        actor: record.owner,
        epoch: 1,
        config: fixture.authority.config(),
        roster: fixture.authority.roster(),
        sequence: 1,
        request: 0xE7,
        payload: &payload,
    };
    apply_chain(
        &mut fixture.market,
        &call.encode()?,
        record.owner,
        HEIGHT + 1,
    )?;
    let mut context = metadata_context()?;
    context.delegate = public(&rotated);
    let bytes = replacement.encode()?;
    fixture.market.signed = sign_metadata(&context, &publication(&bytes, 0)?, &rotated)?;
    fixture.authority = authority_at(&fixture.market, HEIGHT + 1)?;
    assert_ne!(
        public(&delegate()),
        fixture.authority.worker_binding().delegate
    );
    fs::write(
        transport.directory.0.join("delegate.hex"),
        [0xD2u8; 32]
            .iter()
            .map(|n| format!("{n:02x}"))
            .collect::<String>(),
    )?;
    fs::write(
        transport.directory.0.join("metadata.bin"),
        &fixture.market.signed,
    )?;
    transport.observe(&fixture.market, HEIGHT + 1)?;
    transport.wait_current(&fixture.authority);
    // Restart the actual worker with a different configured secret, not just new versions.
    transport.stop();
    fs::remove_dir_all(transport.directory.0.join("inputs"))?;
    transport.start(&fixture.authority)?;
    assert_eq!(
        transport
            .post(SUBMIT_PATH, &old.signed)
            .expect("old-generation replay"),
        ack
    );
    assert_eq!(
        transport
            .post(
                CANCEL_PATH,
                &old.reference(Route::Cancel, principal(9)?, 10, &customer())?
            )
            .expect("old-generation completion after restart"),
        ack
    );
    let saved = transport
        .post(
            QUERY_PATH,
            &old.reference(Route::Query, principal(9)?, 11, &customer())?,
        )
        .expect("old result retrieval");
    let (result, _) = decode_result(&saved, public(&delegate()))?;
    assert_eq!(
        (result.generation, result.key_version, result.outcome),
        (1, 1, Outcome::CancelledBeforeStart)
    );
    assert_eq!(
        decode_result(&saved, public(&rotated)),
        Err(ServiceError::BadSignature)
    );
    transport.stop();
    let reopened = service_jobs(&transport, rotated)?;
    let job = reopened.store().job(expiring.key()?)?;
    assert_eq!(
        reopened.expire(
            &fixture.authority,
            job.record.accepted_ms + u64::from(job.record.admission.deadline_ms)
        )?,
        1
    );
    let result = reopened
        .store()
        .job(expiring.key()?)?
        .result
        .expect("old admission settled");
    assert_eq!(
        decode_result(&result, public(&delegate()))?.0.outcome,
        Outcome::ExpiredBeforeStart
    );
    let private = fs::metadata(transport.jobs().join("delegates"))?;
    assert_eq!(private.permissions().mode() & 0o077, 0);
    for entry in fs::read_dir(transport.jobs().join("delegates"))? {
        assert_eq!(entry?.metadata()?.permissions().mode() & 0o077, 0);
    }
    Ok(())
}
