//! Audit 22.30.122 medium repairs through the production router and persisted codecs.
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::{
    admission::{AdmissionTable, EvaluatorConsent, Participant, RemovalReason},
    aggregation,
    aggregation_codec::AggregationPhase,
    codec::{self, CommitScorePayload, Envelope, ReportBody, RevealScorePayload, ScoreVector},
    dispatch::{self, Buffers, Operation, Routed, SequencePolicy},
    epoch,
    errors::{ApplicationError, CAPACITY, F08_QUORUM_UNAVAILABLE, NON_CANONICAL, WRONG_EPOCH},
    evaluators::{authority, model::GrantStatus},
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::derive_rewards_account,
    registry_ops::{CallContext, PolicySection},
    reputation_transition::split_joint,
    roster,
    state::{self, Section},
    tasks::{self, SetBinding, TaskSet},
    types::{
        AssetId, Authentication, ChainDomain, Digest32, EvaluatorBinding, EvaluatorId,
        EvidenceRoot, FrozenBinding, Presence, PrincipalId, ProgramId, PublicKey32, RequestId,
        RubricDigest, Salt32, Signature64, Version, WorkerId,
    },
    workers::{self, WorkerTable},
    MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_PAYLOAD_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
};
use std::{
    collections::BTreeMap,
    fmt::Debug,
    fs,
    io::Write,
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

// Execute the repaired fixture and the established guest capture/transport/reopen proof
// inside this task's single integration gate, without a toolchain subprocess or artifact.
#[path = "proof_ai_market_guest.rs"]
mod proof_ai_market_guest;
#[path = "view_binding.rs"]
mod view_binding;

#[track_caller]
fn ok<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|e| panic!("real path refused: {e:?}"))
}
fn digest(n: u8) -> Digest32 {
    ok(Digest32::new([n; 32]))
}
fn principal(n: u8) -> PrincipalId {
    ok(PrincipalId::new([n; 32]))
}
fn key(n: u8) -> SigningKey {
    SigningKey::from_bytes(&[n; 32])
}
fn public(k: &SigningKey) -> PublicKey32 {
    PublicKey32(k.verifying_key().to_bytes())
}
const OWNER: u8 = 12;
const WORKER: u8 = 32;
const ORIGIN: u64 = 128;
const KEEPER: u8 = 127;

struct World {
    bytes: Vec<u8>,
    sequences: BTreeMap<u8, u64>,
    request: u64,
    event: Vec<u8>,
    status: Option<codec::ResultStatus>,
}
impl World {
    fn context(actor: u8, height: u64) -> CallContext {
        CallContext {
            chain: ok(ChainDomain::new(
                layerx_programs_ai_market::MERGED_CHAIN_DOMAIN,
            )),
            program: ok(ProgramId::new([11; 32])),
            principal: principal(actor),
            height,
        }
    }
    fn policy(&self) -> PolicySection<'_> {
        ok(PolicySection::decode(
            ok(state::decode_shared_state(&self.bytes)).feature_sections[0],
        ))
    }
    fn market(&self) -> layerx_programs_ai_market::MarketId {
        ok(codec::derive_market(
            Self::context(OWNER, ORIGIN).chain,
            Self::context(OWNER, ORIGIN).program,
        ))
    }
    fn worker(&self, n: u8) -> WorkerId {
        ok(codec::derive_worker(self.market(), principal(n), [n; 32]))
    }
    fn evaluator(&self, n: u8) -> EvaluatorId {
        ok(codec::derive_evaluator(
            self.market(),
            principal(n),
            [n; 32],
        ))
    }
    fn worker_record(&self, n: u8) -> workers::WorkerCurrent {
        let state = ok(state::decode_shared_state(&self.bytes));
        let table = ok(WorkerTable::decode(
            ok(authority::split_identity_section(state.feature_sections[1])).0,
        ));
        table
            .get(self.worker(n))
            .unwrap_or_else(|| panic!("missing produced worker"))
    }
    fn binding(&self) -> FrozenBinding {
        ok(epoch::captured_roster(&ok(state::decode_shared_state(
            &self.bytes,
        ))))
        .unwrap_or_else(|| panic!("missing opening"))
        .binding
    }
    fn header(&mut self, height: u64) -> codec::ReadHeader {
        ok(codec::decode_read_header(&self.call(
            KEEPER,
            dispatch::READ_HEADER,
            &[],
            height,
        )))
    }
    fn encoded_call(&mut self, actor: u8, op: Operation, payload: &[u8], height: u64) -> Vec<u8> {
        self.request += 1;
        let mut request = [0x71; 32];
        request[..8].copy_from_slice(&self.request.to_be_bytes());
        let read = op.metadata().boundary == dispatch::CallBoundary::ProgramRead;
        let (epoch, roster) = if read {
            (0, Presence::Absent)
        } else if op == dispatch::OPEN_EPOCH {
            let mut next = vec![0; MAX_STATE_BYTES];
            let mut scratch = vec![0; epoch::OPEN_SCRATCH_BYTES];
            let frozen = ok(epoch::preview_open(
                &self.bytes,
                height,
                &mut next,
                &mut scratch,
            ));
            (frozen.epoch, Presence::Present(frozen.roster))
        } else if self.bytes.is_empty() {
            (0, Presence::Absent)
        } else {
            let state = ok(state::decode_shared_state(&self.bytes));
            match ok(epoch::captured_roster(&state)) {
                Some(roster) => (
                    roster.binding.epoch,
                    Presence::Present(roster.binding.roster),
                ),
                None => (0, Presence::Absent),
            }
        };
        let sequence = if op.metadata().sequence == SequencePolicy::Role {
            self.sequences.get(&actor).copied().unwrap_or(0) + 1
        } else {
            0
        };
        let ctx = Self::context(actor, height);
        let e = Envelope {
            operation: op,
            chain: ctx.chain,
            program: ctx.program,
            market: self.market(),
            actor: principal(actor),
            epoch,
            config: if read { 0 } else { 1 },
            roster,
            sequence,
            expiry: height + 1000,
            request: ok(RequestId::new(request)),
            payload,
            authentication: Authentication::Native,
        };
        let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
        let n = ok(codec::encode_envelope(&e, &mut bytes));
        bytes.truncate(n);
        bytes
    }
    fn route(
        &mut self,
        actor: u8,
        op: Operation,
        payload: &[u8],
        height: u64,
    ) -> Result<Vec<u8>, ApplicationError> {
        let encoded = self.encoded_call(actor, op, payload, height);
        self.route_encoded(actor, height, &encoded)
    }
    fn route_encoded(
        &mut self,
        actor: u8,
        height: u64,
        encoded: &[u8],
    ) -> Result<Vec<u8>, ApplicationError> {
        self.event.clear();
        self.status = None;
        let e = ok(codec::decode_envelope(encoded));
        let mut next = vec![0xa5; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0xa5; MAX_EVENT_BYTES];
        let mut result = vec![0; MAX_RESULT_BYTES];
        let routed = ok(dispatch::route(
            &Self::context(actor, height),
            encoded,
            if self.bytes.is_empty() {
                None
            } else {
                Some(&self.bytes)
            },
            Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        ));
        let result_len = match routed {
            Routed::Applied {
                state_len,
                result_len,
                revision,
                event_len,
                ..
            } => {
                assert!(event_len > 0);
                self.event = event[..event_len].to_vec();
                let mut topic = [0; 64];
                let len = ok(codec::event_topic(e.envelope.operation, &mut topic));
                let (operation, common, _) =
                    ok(codec::decode_event_frame(&topic[..len], &self.event));
                assert_eq!(operation, e.envelope.operation);
                assert_eq!(common.revision, revision);
                assert_eq!(common.request, ok(e.request_digest()));
                assert_eq!(
                    ok(state::decode_shared_state(&next[..state_len])).revision,
                    revision
                );
                self.bytes = next[..state_len].to_vec();
                if e.envelope.sequence != 0 {
                    self.sequences.insert(actor, e.envelope.sequence);
                }
                result_len
            }
            Routed::Unchanged { result_len } => {
                assert!(next.iter().all(|v| *v == 0xa5));
                assert!(event.iter().all(|v| *v == 0xa5));
                result_len
            }
            Routed::Refused { result_len } => {
                assert!(event.iter().all(|v| *v == 0xa5));
                assert!(next.iter().all(|v| *v == 0xa5));
                let decoded = ok(codec::decode_result(&result[..result_len]));
                return Err(decoded
                    .error
                    .unwrap_or_else(|| panic!("refusal without code")));
            }
        };
        let decoded = ok(codec::decode_result(&result[..result_len]));
        self.status = Some(decoded.status);
        assert_eq!(decoded.error, None);
        Ok(decoded.payload.to_vec())
    }
    fn call(&mut self, actor: u8, op: Operation, payload: &[u8], height: u64) -> Vec<u8> {
        self.route(actor, op, payload, height)
            .unwrap_or_else(|e| panic!("{} at {height}: {e:?}", op.metadata().name))
    }
    fn create() -> Self {
        let mut w = Self {
            bytes: Vec::new(),
            sequences: BTreeMap::new(),
            request: 0,
            event: Vec::new(),
            status: None,
        };
        let policy = ok(TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: digest(1),
                dataset_artifact: [2; 32],
                benchmark_suite: digest(3),
                rubric: ok(RubricDigest::new([4; 32])),
                task_schema: digest(5),
                result_schema: digest(6),
                service_terms: digest(7),
            },
            100,
            1,
        ));
        let mut policy_bytes = vec![0; TASK_POLICY_BYTES];
        ok(policy.encode(&mut policy_bytes));
        let mut payload = principal(OWNER).bytes().to_vec();
        payload.extend_from_slice(&[13; 32]);
        payload.extend_from_slice(
            ok(derive_rewards_account(
                Self::context(OWNER, ORIGIN).program,
                ok(AssetId::new([13; 32])),
            ))
            .as_bytes(),
        );
        payload.extend_from_slice(&[14; 32]);
        payload.push(0);
        payload.extend_from_slice(&policy_bytes);
        payload.extend_from_slice(&[16; 32]);
        w.call(OWNER, dispatch::CREATE, &payload, ORIGIN);
        w
    }
    fn approval(
        &mut self,
        participant: Participant,
        owner: u8,
        delegate: PublicKey32,
        identity: Digest32,
        at: u64,
    ) -> (u64, Digest32) {
        let state = ok(state::decode_shared_state(&self.bytes));
        let (_, table) = ok(split_joint(state.feature_sections[4]));
        let table = ok(AdmissionTable::decode(table));
        let effective = if table.current_epoch().is_none() {
            0
        } else {
            (at - ORIGIN) / 128 + 1
        };
        let mut payload = vec![match participant {
            Participant::Worker(_) => 1,
            Participant::Evaluator(_) => 2,
        }];
        payload.extend_from_slice(&participant.bytes());
        payload.extend_from_slice(principal(owner).as_bytes());
        payload.extend_from_slice(digest(owner).as_bytes());
        payload.extend_from_slice(&delegate.0);
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(identity.as_bytes());
        payload.extend_from_slice(&effective.to_be_bytes());
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(&(ORIGIN + effective * 128 + 64).max(at + 1).to_be_bytes());
        self.call(OWNER, dispatch::ApproveAdmission, &payload, at);
        let state = ok(state::decode_shared_state(&self.bytes));
        let table = ok(AdmissionTable::decode(
            ok(split_joint(state.feature_sections[4])).1,
        ));
        let approval = table
            .get(participant)
            .and_then(|m| m.approval)
            .unwrap_or_else(|| panic!("missing produced approval"));
        (effective, approval.digest)
    }
    fn enroll(&mut self, n: u8, at: u64) {
        let k = key(n);
        let mut payload = principal(n).bytes().to_vec();
        payload.extend_from_slice(&[n; 32]);
        payload.extend_from_slice(&public(&k).0);
        payload.extend_from_slice(&[0x44; 32]);
        payload.extend_from_slice(&(at + 120).to_be_bytes());
        self.call(OWNER, dispatch::EnrollWorker, &payload, at);
        let record = self.worker_record(n);
        let consent = ok(workers::consent_digest(
            &self.policy().header,
            record.worker,
            record.owner,
            record.delegate,
            1,
            1,
            record.metadata,
            record.expiry,
        ));
        let mut payload = record.worker.bytes().to_vec();
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(record.metadata.as_bytes());
        payload.extend_from_slice(&k.sign(consent.as_bytes()).to_bytes());
        self.call(n, dispatch::AcceptEnrollment, &payload, at + 1);
        let record = self.worker_record(n);
        let (effective, approval) = self.approval(
            Participant::Worker(record.worker),
            n,
            record.delegate,
            ok(record.proposal_digest(&self.policy().header)),
            at + 2,
        );
        let mut payload = record.worker.bytes().to_vec();
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(&effective.to_be_bytes());
        payload.extend_from_slice(&1u64.to_be_bytes());
        payload.extend_from_slice(approval.as_bytes());
        self.call(n, dispatch::AdmitWorker, &payload, at + 3);
    }
    fn nominate(&mut self, n: u8, at: u64) {
        let effective = if ok(epoch::captured_roster(&ok(state::decode_shared_state(
            &self.bytes,
        ))))
        .is_none()
        {
            0
        } else {
            (at - ORIGIN) / 128 + 1
        };
        let mut payload = principal(n).bytes().to_vec();
        payload.extend_from_slice(&[n; 32]);
        payload.extend_from_slice(&[4; 32]);
        payload.extend_from_slice(&public(&key(n)).0);
        for value in [1u64, 1, effective, effective + 32] {
            payload.extend_from_slice(&value.to_be_bytes());
        }
        self.call(OWNER, dispatch::ScheduleEvaluator, &payload, at);
        let state = ok(state::decode_shared_state(&self.bytes));
        let region = ok(authority::evaluator_region(state.feature_sections[1]));
        let grant = region
            .get(self.evaluator(n))
            .unwrap_or_else(|| panic!("missing nomination"))
            .grant;
        let mut bytes = vec![0; layerx_programs_ai_market::evaluators::model::GRANT_BYTES];
        let len = ok(layerx_programs_ai_market::evaluators::codec::encode_grant(
            &grant, &mut bytes,
        ));
        let identity = ok(codec::domain_hash(
            "PAXAI/evaluator-grant/v1",
            &bytes[..len],
        ));
        let (_, approval) = self.approval(
            Participant::Evaluator(grant.evaluator),
            n,
            grant.signing_key,
            identity,
            at + 1,
        );
        // The signed consent carries the same request ID as its real envelope.
        let consent = EvaluatorConsent {
            chain: Self::context(n, at).chain,
            program: Self::context(n, at).program,
            market: self.market(),
            evaluator: grant.evaluator,
            owner: principal(n),
            signing_key: grant.signing_key,
            enrollment_nonce: [n; 32],
            rubric: grant.rubric,
            approval_digest: approval,
            request: ok(RequestId::new({
                let mut id = [0x71; 32];
                id[..8].copy_from_slice(&(self.request + 1).to_be_bytes());
                id
            })),
            grant_version: 1,
            key_version: 1,
            effective_epoch: effective,
            config_version: 1,
            expiry_height: (ORIGIN + effective * 128 + 64).max(at + 3),
        };
        let mut bytes = [0; layerx_programs_ai_market::evaluators::model::CONSENT_BYTES];
        ok(consent.encode(&mut bytes));
        let mut payload = bytes.to_vec();
        payload.extend_from_slice(&key(n).sign(ok(consent.digest()).as_bytes()).to_bytes());
        self.call(n, dispatch::AdmitEvaluator, &payload, at + 2);
    }
    fn publish(&mut self, n: u8, at: u64) {
        let r = self.worker_record(n);
        let mut manifest = 1u16.to_be_bytes().to_vec();
        manifest.extend_from_slice(self.market().as_bytes());
        manifest.extend_from_slice(r.worker.as_bytes());
        manifest.extend_from_slice(r.owner.as_bytes());
        for value in [
            r.generation,
            r.key_version,
            r.metadata_revision + 1,
            at,
            at + 256,
        ] {
            manifest.extend_from_slice(&value.to_be_bytes());
        }
        manifest.extend_from_slice(&[60; 32]);
        manifest.extend_from_slice(&1u16.to_be_bytes());
        manifest.extend_from_slice(&[1, 1]);
        for byte in 61..66 {
            manifest.extend_from_slice(&[byte; 32]);
        }
        for _ in 0..4 {
            manifest.extend_from_slice(&1024u32.to_be_bytes());
        }
        manifest.push(1);
        manifest.extend_from_slice(&60_000u32.to_be_bytes());
        manifest.extend_from_slice(&4u16.to_be_bytes());
        manifest.push(1);
        manifest.extend_from_slice(&1u16.to_be_bytes());
        manifest.push(1);
        let uri = b"https://worker.example/paxai/v1";
        manifest.extend_from_slice(&(uri.len() as u32).to_be_bytes());
        manifest.extend_from_slice(uri);
        manifest.extend_from_slice(&[70; 32]);
        manifest.push(1);
        manifest.extend_from_slice(&1u16.to_be_bytes());
        manifest.extend_from_slice(&1u16.to_be_bytes());
        manifest.extend_from_slice(&[71; 32]);
        manifest.extend_from_slice(&[72; 32]);
        manifest.extend_from_slice(&0u32.to_be_bytes());
        let m = ok(workers::decode_manifest(&manifest));
        let mut payload = r.worker.bytes().to_vec();
        payload.extend_from_slice(&r.metadata_revision.to_be_bytes());
        payload.extend_from_slice(&m.revision.to_be_bytes());
        payload.extend_from_slice(m.digest.as_bytes());
        payload.extend_from_slice(&m.valid_from.to_be_bytes());
        payload.extend_from_slice(&m.expiry.to_be_bytes());
        payload.extend_from_slice(&(manifest.len() as u32).to_be_bytes());
        payload.extend_from_slice(&manifest);
        let mut encoded = self.encoded_call(n, dispatch::PublishMetadata, &payload, at);
        let signed = ok(codec::decode_envelope(&encoded));
        let mut env = signed.envelope;
        env.authentication = Authentication::Delegate {
            key: r.delegate,
            signature: Signature64([0; 64]),
        };
        let k = if r.key_version == 1 {
            key(n)
        } else {
            key(n + 64)
        };
        let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
        let len = ok(codec::encode_envelope(&env, &mut bytes));
        let request = ok(ok(codec::decode_envelope(&bytes[..len])).request_digest());
        env.authentication = Authentication::Delegate {
            key: r.delegate,
            signature: Signature64(k.sign(request.as_bytes()).to_bytes()),
        };
        let len = ok(codec::encode_envelope(&env, &mut bytes));
        bytes.truncate(len);
        encoded = bytes;
        ok(self.route_encoded(n, at, &encoded));
    }
    fn open(&mut self, epoch: u64) {
        self.call(KEEPER, dispatch::OPEN_EPOCH, &[], ORIGIN + epoch * 128);
        assert_eq!(self.binding().epoch, epoch);
    }
    fn first_open() -> Self {
        let mut w = Self::create();
        assert_eq!(w.header(ORIGIN).epoch, Presence::Absent);
        let mut schedule = 1u64.to_be_bytes().to_vec();
        schedule.extend_from_slice(&1u64.to_be_bytes());
        w.call(OWNER, dispatch::SCHEDULE_ACTIVATION, &schedule, ORIGIN);
        w.enroll(WORKER, 130);
        for n in 2..=4 {
            w.nominate(n, 128 + 4 * u64::from(n));
        }
        let mut fund = 5000u128.to_be_bytes().to_vec();
        fund.extend_from_slice(&[14; 32]);
        fund.extend_from_slice(&1u64.to_be_bytes());
        fund.push(1);
        w.call(OWNER, dispatch::FUND, &fund, 147);
        w.publish(WORKER, 150);
        w.open(1);
        let mut activate = ok(state::decode_shared_state(&w.bytes))
            .revision
            .to_be_bytes()
            .to_vec();
        activate.extend_from_slice(&1u64.to_be_bytes());
        w.call(KEEPER, dispatch::ADVANCE_ACTIVATION, &activate, 257);
        w
    }
    fn score(&mut self, ppm: u32) {
        let binding = self.binding();
        let start = ORIGIN + binding.epoch * 128;
        let policy = self.policy().current.digest();
        let policy = ok(policy);
        let set_binding = SetBinding {
            market: binding.market,
            epoch: binding.epoch,
            config: binding.config,
            policy,
            roster: binding.roster,
        };
        let task_region = self.policy().task_region.to_vec();
        let set = ok(tasks::task_set_digest(
            &set_binding,
            &ok(TaskSet::decode(&task_region)),
        ));
        let mut payload = binding.epoch.to_be_bytes().to_vec();
        payload.extend_from_slice(&binding.config.get().to_be_bytes());
        payload.extend_from_slice(set.as_bytes());
        self.call(KEEPER, dispatch::SEAL_TASK_SET, &payload, start + 64);
        let mut frozen_workers = ok(epoch::captured_roster(&ok(state::decode_shared_state(
            &self.bytes,
        ))))
        .unwrap_or_else(|| panic!("missing opening"))
        .workers()
        .to_vec();
        frozen_workers.sort_by_key(|w| w.worker);
        let mut scores = Vec::new();
        for w in frozen_workers {
            scores.extend_from_slice(w.worker.as_bytes());
            scores.extend_from_slice(&ppm.to_be_bytes());
        }
        let mut signed = Vec::new();
        for n in 2..=4 {
            let evaluator_binding = EvaluatorBinding {
                frozen: binding,
                evaluator: self.evaluator(n),
                grant: ok(Version::new(1)),
                key_version: ok(Version::new(1)),
            };
            let evidence = ok(EvidenceRoot::new([0xe0 + n; 32]));
            let mut seal = 1u16.to_be_bytes().to_vec();
            seal.extend_from_slice(evidence.as_bytes());
            seal.extend_from_slice(policy.as_bytes());
            seal.extend_from_slice(set.as_bytes());
            seal.extend_from_slice(&[4; 32]);
            seal.push(1);
            self.call(n, dispatch::SealEvidence, &seal, start + 64);
            let body = ReportBody {
                binding: evaluator_binding,
                evidence,
                scores: ScoreVector::Encoded(&scores),
            };
            let report = ok(codec::report_digest(&body));
            let commitment = ok(codec::commitment_digest(
                &evaluator_binding,
                report,
                ok(Salt32::new([0x5a; 32])),
            ));
            let mut commit =
                vec![0; layerx_programs_ai_market::commit_reveal::commitment::COMMIT_SCORE_BYTES];
            ok(codec::encode_commit_score(
                &CommitScorePayload {
                    binding: evaluator_binding,
                    commitment,
                },
                &mut commit,
            ));
            self.call(n, dispatch::CommitScore, &commit, start + 64);
            let signature = Signature64(
                key(n)
                    .sign(ok(codec::attestation_digest(report)).as_bytes())
                    .to_bytes(),
            );
            let mut reveal =
                vec![0; layerx_programs_ai_market::commit_reveal::commitment::REVEAL_MAX_BYTES];
            let len = ok(codec::encode_reveal_score(
                &RevealScorePayload {
                    report: body,
                    signature,
                    salt: ok(Salt32::new([0x5a; 32])),
                },
                &mut reveal,
            ));
            reveal.truncate(len);
            signed.push((n, reveal));
        }
        for (n, reveal) in signed {
            self.call(n, dispatch::RevealScore, &reveal, start + 80);
        }
    }
}

static SERIAL: AtomicU64 = AtomicU64::new(0);
struct Saved(PathBuf);
impl Saved {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "paxai-22-33-217-{}-{}",
            std::process::id(),
            SERIAL.fetch_add(1, Ordering::Relaxed)
        ));
        ok(fs::create_dir(&path));
        Self(path)
    }
    fn write(&self, name: &str, bytes: &[u8]) {
        let mut file = ok(fs::File::create(self.0.join(name)));
        ok(file.write_all(bytes));
        ok(file.sync_all());
    }
    fn read(&self, name: &str) -> Vec<u8> {
        ok(fs::read(self.0.join(name)))
    }
}
impl Drop for Saved {
    fn drop(&mut self) {
        ok(fs::remove_dir_all(&self.0));
    }
}
fn reopen(w: World) -> World {
    let saved = Saved::new();
    saved.write("state", &w.bytes);
    saved.write("event", &w.event);
    let sequences = w.sequences.clone();
    let request = w.request;
    drop(w);
    let bytes = saved.read("state");
    ok(state::decode_shared_state(&bytes));
    World {
        bytes,
        sequences,
        request,
        event: saved.read("event"),
        status: None,
    }
}
fn admission(w: &World) -> AdmissionTable {
    let state = ok(state::decode_shared_state(&w.bytes));
    ok(AdmissionTable::decode(
        ok(split_joint(
            state.feature_sections[Section::ReputationAdmission.index()],
        ))
        .1,
    ))
}
fn revoke_payload(w: &World, n: u8) -> Vec<u8> {
    let mut payload = w.evaluator(n).bytes().to_vec();
    payload.extend_from_slice(&1u64.to_be_bytes());
    payload.extend_from_slice(&(authority::RevocationReason::CompromisedKey as u16).to_be_bytes());
    payload.extend_from_slice(digest(0xe1).as_bytes());
    payload
}
fn assert_revocation(w: &World, n: u8, height: u64, excludes: bool) {
    let shared = ok(state::decode_shared_state(&w.bytes));
    let region = ok(authority::evaluator_region(
        shared.feature_sections[Section::IdentityRoster.index()],
    ));
    let record = region
        .get(w.evaluator(n))
        .unwrap_or_else(|| panic!("missing evaluator"));
    assert_eq!(record.grant.status, GrantStatus::Revoked);
    assert_eq!(
        record.revocation,
        Some(authority::Revocation {
            reason: authority::RevocationReason::CompromisedKey,
            evidence: digest(0xe1),
            height,
            excludes_frozen_epoch: excludes,
        })
    );
    let (operation, common, suffix) = ok(codec::decode_event_frame(
        b"PAXAI/v1/RevokeEvaluator",
        &w.event,
    ));
    assert_eq!(operation, dispatch::RevokeEvaluator);
    assert_eq!(common.market, w.market());
    assert_eq!(common.revision, shared.revision);
    let mut expected = revoke_payload(w, n);
    expected.push(u8::from(excludes));
    assert_eq!(suffix, expected);
    assert_eq!(common.result, ok(codec::result_digest(&expected)));
    assert_eq!(record.last.result, common.result);
    assert_eq!(record.last.digest, common.request);
}
fn settlement(w: &World) -> Vec<u8> {
    ok(state::decode_shared_state(&w.bytes)).feature_sections[Section::SettlementClaims.index()]
        .to_vec()
}

#[test]
fn authority_route_forgets_aggregate_seal() {
    // Before opening, no F05 epoch exists and no frozen evaluator can be excluded.
    let mut preopen = World::create();
    preopen.nominate(2, 136);
    assert_eq!(aggregation::progress(&preopen.bytes), Err(WRONG_EPOCH));
    let payload = revoke_payload(&preopen, 2);
    let result = preopen.call(OWNER, dispatch::RevokeEvaluator, &payload, 140);
    assert_eq!(result.last(), Some(&0));
    assert_revocation(&preopen, 2, 140, false);
    preopen = reopen(preopen);
    assert_revocation(&preopen, 2, 140, false);

    let mut unsealed = World::first_open();
    assert_eq!(
        ok(aggregation::progress(&unsealed.bytes)).phase,
        AggregationPhase::Unsealed
    );
    let before = settlement(&unsealed);
    let payload = revoke_payload(&unsealed, 2);
    let result = unsealed.call(OWNER, dispatch::RevokeEvaluator, &payload, 258);
    assert_eq!(result.last(), Some(&1));
    assert_revocation(&unsealed, 2, 258, true);
    assert_eq!(settlement(&unsealed), before);
    unsealed = reopen(unsealed);
    assert_revocation(&unsealed, 2, 258, true);
    assert_eq!(
        ok(aggregation::progress(&unsealed.bytes)).phase,
        AggregationPhase::Unsealed
    );

    let mut sealed = World::first_open();
    sealed.score(800_000);
    sealed.call(KEEPER, dispatch::BeginAggregation, &[], 352);
    let progress = ok(aggregation::progress(&sealed.bytes));
    assert_eq!(progress.phase, AggregationPhase::Processing);
    let Presence::Present(input) = progress.input else {
        panic!("no sealed input")
    };
    let captured =
        ok(aggregation::captured_aggregation(&sealed.bytes)).unwrap_or_else(|| panic!("no record"));
    let reports = (0..3).map(|i| ok(captured.report(i))).collect::<Vec<_>>();
    assert!(captured.report(3).is_err());
    for (n, phase) in [
        (2, AggregationPhase::Processing),
        (3, AggregationPhase::Processing),
        (4, AggregationPhase::Terminal),
    ] {
        assert_eq!(ok(aggregation::progress(&sealed.bytes)).phase, phase);
        let before = settlement(&sealed);
        let payload = revoke_payload(&sealed, n);
        let encoded = sealed.encoded_call(OWNER, dispatch::RevokeEvaluator, &payload, 352);
        let result = ok(sealed.route_encoded(OWNER, 352, &encoded));
        assert_eq!(result.last(), Some(&0));
        assert_revocation(&sealed, n, 352, false);
        assert_eq!(settlement(&sealed), before);
        sealed = reopen(sealed);
        assert_revocation(&sealed, n, 352, false);
        assert_eq!(settlement(&sealed), before);
        assert_eq!(
            ok(aggregation::progress(&sealed.bytes)).input,
            Presence::Present(input)
        );
        let recovered = ok(aggregation::captured_aggregation(&sealed.bytes))
            .unwrap_or_else(|| panic!("lost seal"));
        for (i, report) in reports.iter().enumerate() {
            assert_eq!(ok(recovered.report(i)), *report);
        }
        let state = sealed.bytes.clone();
        ok(sealed.route_encoded(OWNER, 352, &encoded));
        assert_eq!(sealed.status, Some(codec::ResultStatus::AlreadyApplied));
        assert_eq!(sealed.bytes, state);
        assert!(sealed.event.is_empty());
        if n == 2 {
            let mut process = input.bytes().to_vec();
            process.extend_from_slice(&progress.cursor.to_be_bytes());
            sealed.call(KEEPER, dispatch::ProcessAggregation, &process, 352);
            let output = ok(aggregation::captured_aggregation(&sealed.bytes))
                .unwrap_or_else(|| panic!("lost outputs"));
            assert_eq!(ok(output.output(0)).support(), 3);
            assert_eq!(ok(output.output(0)).score().get(), 800_000);
        } else if n == 3 {
            sealed.call(KEEPER, dispatch::FinalizeAggregation, input.as_bytes(), 352);
        }
    }
}

fn admit_payload(w: &World, nonce: u8) -> Vec<u8> {
    let binding = w.binding();
    let mut payload = binding.epoch.to_be_bytes().to_vec();
    payload.extend_from_slice(&binding.config.get().to_be_bytes());
    payload.extend_from_slice(ok(w.policy().current.digest()).as_bytes());
    payload.extend_from_slice(binding.roster.as_bytes());
    payload.extend_from_slice(principal(KEEPER).as_bytes());
    payload.extend_from_slice(w.worker(WORKER).as_bytes());
    payload.extend_from_slice(w.worker_record(WORKER).metadata.as_bytes());
    payload.extend_from_slice(&[nonce; 32]);
    payload.extend_from_slice(digest(0x42).as_bytes());
    payload.extend_from_slice(&280u64.to_be_bytes());
    payload
}

#[test]
fn quorum_gate_has_no_production_caller() {
    let mut w = World::first_open();
    assert_eq!(ok(roster::health(&admission(&w))).eligible_evaluators, 3);
    let original = admit_payload(&w, 1);
    let retry = w.encoded_call(KEEPER, dispatch::ADMIT_TASK, &original, 258);
    ok(w.route_encoded(KEEPER, 258, &retry));
    assert_eq!(w.status, Some(codec::ResultStatus::Ok));
    let task = ok(codec::derive_task(
        w.market(),
        w.binding().epoch,
        principal(KEEPER),
        [1; 32],
    ));
    assert_eq!(ok(TaskSet::decode(w.policy().task_region)).len(), 1);
    let new = admit_payload(&w, 2);
    let new_request = w.encoded_call(KEEPER, dispatch::ADMIT_TASK, &new, 260);
    for n in 2..=4 {
        let at = 259 + u64::from(n - 2) * 2;
        let mut removal = vec![2];
        removal.extend_from_slice(w.evaluator(n).as_bytes());
        removal.extend_from_slice(&1u64.to_be_bytes());
        removal.push(if n == 2 {
            RemovalReason::Terms
        } else {
            RemovalReason::Security
        } as u8);
        w.call(OWNER, dispatch::AdministrativeRemove, &removal, at);
        assert_eq!(
            ok(roster::health(&admission(&w))).eligible_evaluators,
            4 - n
        );
        let before = w.bytes.clone();
        assert_eq!(
            w.route_encoded(KEEPER, at + 1, &new_request),
            Err(F08_QUORUM_UNAVAILABLE)
        );
        assert_eq!(w.bytes, before);
        assert!(w.event.is_empty());
        // Exact transport replay and the same object under a fresh envelope remain retries.
        assert_eq!(ok(w.route_encoded(KEEPER, at + 1, &retry)), task.bytes());
        assert_eq!(w.status, Some(codec::ResultStatus::AlreadyApplied));
        assert_eq!(
            ok(w.route(KEEPER, dispatch::ADMIT_TASK, &original, at + 1)),
            task.bytes()
        );
        assert_eq!(w.status, Some(codec::ResultStatus::AlreadyApplied));
        assert_eq!(w.bytes, before);
        assert!(w.event.is_empty());
        w = reopen(w);
        assert_eq!(w.bytes, before);
        assert_eq!(
            w.route_encoded(KEEPER, at + 1, &new_request),
            Err(F08_QUORUM_UNAVAILABLE)
        );
        assert_eq!(w.bytes, before);
        assert!(w.event.is_empty());
        ok(w.route_encoded(KEEPER, at + 1, &retry));
        assert_eq!(w.status, Some(codec::ResultStatus::AlreadyApplied));
        assert_eq!(w.bytes, before);
        assert_eq!(ok(TaskSet::decode(w.policy().task_region)).len(), 1);
    }
}

#[test]
fn guest_payload_bound_and_routed_execution_are_retained() {
    // Actual deployment profile/artifact qualification belongs to the wiring requests.
    // This native gate retains the guest codec limits and executes its production router.
    assert_eq!(MAX_ENVELOPE_BYTES, 16_384);
    assert_eq!(MAX_PAYLOAD_BYTES, 15_965);
    let mut w = World::first_open();
    let original = w.bytes.clone();
    let payload = admit_payload(&w, 9);
    let encoded = w.encoded_call(KEEPER, dispatch::ADMIT_TASK, &payload, 258);
    let decoded = ok(codec::decode_envelope(&encoded));
    let max = vec![0x42; MAX_PAYLOAD_BYTES];
    let mut envelope = decoded.envelope;
    envelope.operation = dispatch::CREATE;
    envelope.epoch = 0;
    envelope.roster = Presence::Absent;
    envelope.sequence = 1;
    envelope.payload = &max;
    let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
    let len = ok(codec::encode_envelope(&envelope, &mut bytes));
    assert_eq!(len, codec::ENVELOPE_PREFIX_BYTES + MAX_PAYLOAD_BYTES + 1);
    assert_eq!(len, 16_204);
    assert_eq!(
        ok(codec::decode_envelope(&bytes[..len])).envelope.payload,
        max
    );
    let over = vec![0x42; MAX_PAYLOAD_BYTES + 1];
    envelope.payload = &over;
    assert_eq!(
        codec::encode_envelope(&envelope, &mut bytes),
        Err(NON_CANONICAL)
    );
    bytes.resize(MAX_ENVELOPE_BYTES + 1, 0);
    let mut next = vec![0xa5; MAX_STATE_BYTES];
    let mut event = vec![0xa5; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let Routed::Refused { result_len } = ok(dispatch::route(
        &World::context(KEEPER, 258),
        &bytes,
        Some(&w.bytes),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )) else {
        panic!("oversize envelope accepted")
    };
    assert_eq!(
        ok(codec::decode_result(&result[..result_len])).error,
        Some(CAPACITY)
    );
    assert!(next.iter().all(|v| *v == 0xa5));
    assert!(event.iter().all(|v| *v == 0xa5));
    assert_eq!(w.bytes, original);
    ok(w.route_encoded(KEEPER, 258, &encoded));
    assert_eq!(w.status, Some(codec::ResultStatus::Ok));
    assert_eq!(ok(TaskSet::decode(w.policy().task_region)).len(), 1);
    assert_ne!(w.bytes, original);
}
