//! Audit 22.30.122 regression paths: real dispatch producers, canonical capture and
//! the projection codec used by the hosted indexer on observation and recovery.
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::{
    admission::{AdmissionTable, EvaluatorConsent, Participant},
    aggregation,
    aggregation_codec::AggregationPhase,
    codec::{self, CommitScorePayload, Envelope, ReportBody, RevealScorePayload, ScoreVector},
    dispatch::{self, Buffers, Operation, Routed, SequencePolicy},
    epoch,
    errors::{ApplicationError, WRONG_DOMAIN, WRONG_ROSTER},
    evaluators::authority,
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{
        self, Availability, CaptureFacts, FinalityEvidence, ParticipantKind, ParticipantRow,
        ReadProof, RewardField, ScoreField, ScoreStatus, SnapshotBinding, StateCapture,
        BINDING_MAX_BYTES,
    },
    registry::derive_rewards_account,
    registry_ops::{CallContext, PolicySection},
    reputation_transition::split_joint,
    rewards::{self, EpochStatus, REWARD_STATE_BYTES},
    state::{self, Section},
    tasks::{self, SetBinding, TaskSet},
    types::{
        AssetId, Authentication, ChainDomain, Digest32, EvaluatorBinding, EvaluatorId,
        EvidenceRoot, FrozenBinding, MetadataDigest, Presence, PrincipalId, ProgramId, PublicKey32,
        RequestId, RubricDigest, Salt32, Signature64, Version, WorkerId,
    },
    workers::{self, WorkerTable},
    MAX_CHUNK_BYTES, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
};
use std::{
    collections::BTreeMap,
    fmt::Debug,
    fs,
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

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
        let e = ok(codec::decode_envelope(encoded));
        let mut next = vec![0xa5; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; MAX_EVENT_BYTES];
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
            Routed::Unchanged { result_len } => result_len,
            Routed::Refused { result_len } => {
                assert!(next.iter().all(|v| *v == 0xa5));
                let decoded = ok(codec::decode_result(&result[..result_len]));
                return Err(decoded
                    .error
                    .unwrap_or_else(|| panic!("refusal without code")));
            }
        };
        let decoded = ok(codec::decode_result(&result[..result_len]));
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
    fn rotate(&mut self, n: u8, at: u64) {
        let r = self.worker_record(n);
        let k = key(n + 64);
        let metadata = ok(MetadataDigest::new([0x45; 32]));
        let consent = ok(workers::consent_digest(
            &self.policy().header,
            r.worker,
            r.owner,
            public(&k),
            r.generation + 1,
            r.key_version + 1,
            metadata,
            at + 120,
        ));
        let mut payload = r.worker.bytes().to_vec();
        payload.extend_from_slice(&r.generation.to_be_bytes());
        payload.extend_from_slice(&r.key_version.to_be_bytes());
        payload.extend_from_slice(&public(&k).0);
        payload.extend_from_slice(metadata.as_bytes());
        payload.extend_from_slice(&(at + 120).to_be_bytes());
        payload.extend_from_slice(&k.sign(consent.as_bytes()).to_bytes());
        self.call(n, dispatch::RotateDelegate, &payload, at);
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
    fn settle(&mut self) {
        let at = ORIGIN + self.binding().epoch * 128 + 96;
        self.call(KEEPER, dispatch::BeginAggregation, &[], at);
        loop {
            let p = ok(aggregation::progress(&self.bytes));
            if p.cursor == p.worker_count {
                break;
            }
            let Presence::Present(input) = p.input else {
                panic!("missing sealed input")
            };
            let mut payload = input.bytes().to_vec();
            payload.extend_from_slice(&p.cursor.to_be_bytes());
            self.call(KEEPER, dispatch::ProcessAggregation, &payload, at);
        }
        let Presence::Present(input) = ok(aggregation::progress(&self.bytes)).input else {
            panic!("missing input")
        };
        self.call(KEEPER, dispatch::FinalizeAggregation, input.as_bytes(), at);
        assert_eq!(
            ok(aggregation::progress(&self.bytes)).phase,
            AggregationPhase::Terminal
        );
    }
    fn capture(&mut self) -> Captured {
        let proof = ReadProof {
            chain: Self::context(KEEPER, 1).chain,
            program: Self::context(KEEPER, 1).program,
            native_state_root: ok(codec::domain_hash("PAXAI/test-root/v1", &self.bytes)),
            observed_sequence: ok(state::decode_shared_state(&self.bytes)).revision,
            execution_height: ORIGIN + self.binding().epoch * 128 + 96,
            block_hash: digest(0xbb),
        };
        let mut buffer = vec![0; MAX_STATE_BYTES];
        let mut capture = StateCapture::new(&mut buffer);
        let mut pin = (0u64, [0; 32]);
        for offset in (0..self.bytes.len()).step_by(MAX_CHUNK_BYTES) {
            let mut payload = pin.0.to_be_bytes().to_vec();
            payload.extend_from_slice(&pin.1);
            payload.extend_from_slice(&(offset as u32).to_be_bytes());
            payload.extend_from_slice(&(MAX_CHUNK_BYTES as u16).to_be_bytes());
            let chunk = self.call(
                KEEPER,
                dispatch::READ_STATE_CHUNK,
                &payload,
                proof.execution_height,
            );
            let decoded = ok(codec::decode_chunk_response(&chunk));
            pin = (decoded.revision, decoded.digest.bytes());
            ok(capture.accept(&proof, &chunk));
        }
        let (state, facts) = ok(capture.finish());
        let finality = FinalityEvidence {
            native_state_root: proof.native_state_root,
            block_height: proof.execution_height,
            block_hash: proof.block_hash,
            app_hash: proof.native_state_root,
            store_proof_digest: digest(0xdd),
            rank: 0,
        };
        let binding = ok(queries::bind_snapshot(state, &facts, &finality, 0));
        Captured {
            state: state.to_vec(),
            facts,
            binding,
        }
    }
}

struct Captured {
    state: Vec<u8>,
    facts: CaptureFacts,
    binding: SnapshotBinding,
}
fn blank() -> ParticipantRow {
    ParticipantRow {
        kind: ParticipantKind::Worker,
        id: [1; 32],
        owner: principal(1),
        generation: 1,
        identity_state: 0,
        frozen_member: false,
        frozen_generation: Presence::Absent,
        eligibility: 0,
        metadata: Presence::Absent,
        metadata_revision: 0,
        score: ok(ScoreField::absent(
            Presence::Absent,
            ScoreStatus::NotProduced,
        )),
        reward: ok(RewardField::unavailable(Availability::NotYetProduced)),
        history_status: Availability::NotYetProduced,
        history: Presence::Absent,
    }
}
impl Captured {
    fn projection(&self) -> ([Availability; 11], Vec<ParticipantRow>) {
        ok(self.binding.verify_capture(&self.state, &self.facts));
        let mut rows = vec![blank(); 40];
        let (features, len) = ok(queries::project_capture(
            &self.state,
            &self.binding,
            &mut rows,
        ));
        rows.truncate(len);
        (features, rows)
    }
    fn wire(&self) -> Vec<u8> {
        let mut bytes = vec![0; BINDING_MAX_BYTES];
        let n = ok(self.binding.encode(&mut bytes));
        bytes.truncate(n);
        bytes
    }
}
static SERIAL: AtomicU64 = AtomicU64::new(0);
struct Saved(PathBuf);
impl Saved {
    fn new() -> Self {
        let dir = std::env::temp_dir().join(format!(
            "paxai-22-31-253-{}-{}",
            std::process::id(),
            SERIAL.fetch_add(1, Ordering::Relaxed)
        ));
        ok(fs::create_dir(&dir));
        Self(dir)
    }
    fn save(&self, capture: &Captured) {
        ok(fs::write(self.0.join("state"), &capture.state));
        ok(fs::write(self.0.join("binding"), capture.wire()));
        // Save precisely the projected canonical rows and availability that indexer observe commits.
        let (features, rows) = capture.projection();
        ok(fs::write(
            self.0.join("availability"),
            features.map(|v| v as u8),
        ));
        ok(fs::write(self.0.join("rows"), row_bytes(&rows)));
    }
    fn reopen(&self, facts: CaptureFacts) -> Captured {
        let state = ok(fs::read(self.0.join("state")));
        let binding = ok(SnapshotBinding::decode(&ok(fs::read(
            self.0.join("binding"),
        ))));
        let capture = Captured {
            state,
            facts,
            binding,
        };
        let (features, rows) = capture.projection();
        assert_eq!(
            ok(fs::read(self.0.join("availability"))),
            features.map(|v| v as u8)
        );
        assert_eq!(ok(fs::read(self.0.join("rows"))), row_bytes(&rows));
        capture
    }
}
impl Drop for Saved {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}
fn row_bytes(rows: &[ParticipantRow]) -> Vec<u8> {
    let mut bytes = vec![0; queries::PAGE_MAX_BYTES];
    let mut writer = codec::Writer::new(&mut bytes);
    for row in rows {
        ok(row.encode(&mut writer));
    }
    let len = writer.len();
    bytes.truncate(len);
    bytes
}
fn reopen_world(w: World) -> World {
    let saved = Saved::new();
    ok(fs::write(saved.0.join("world"), &w.bytes));
    let sequences = w.sequences.clone();
    let request = w.request;
    drop(w);
    World {
        bytes: ok(fs::read(saved.0.join("world"))),
        sequences,
        request,
    }
}
fn replace_section(bytes: &[u8], section: Section, payload: &[u8]) -> Vec<u8> {
    let state = ok(state::decode_shared_state(bytes));
    let next = ok(state.replace_section(section, payload));
    let mut bytes = vec![0; MAX_STATE_BYTES];
    let mut control = vec![0; Section::Control.payload_cap()];
    let len = ok(state::encode_shared_state(&next, &mut bytes, &mut control));
    bytes.truncate(len);
    bytes
}

#[test]
fn header_omits_opened_epoch() {
    let mut w = World::first_open();
    let expected = w.binding();
    let header = w.header(258);
    assert_eq!(header.epoch, Presence::Present(expected.epoch));
    assert_eq!(header.roster, Presence::Present(expected.roster));
    let capture = w.capture();
    let saved = Saved::new();
    saved.save(&capture);
    let facts = capture.facts;
    assert_eq!(
        ok(SnapshotBinding::decode(&capture.wire())),
        capture.binding
    );
    drop(capture);
    drop(w);
    let recovered = saved.reopen(facts);
    assert_eq!(recovered.binding.epoch, Presence::Present(expected.epoch));
    assert_eq!(recovered.binding.roster, Presence::Present(expected.roster));
    // A canonical F08 table naming another epoch must disagree with F03/F06.
    let shared = ok(state::decode_shared_state(&recovered.state));
    let mut joint = shared.feature_sections[4].to_vec();
    let table_len = ok(split_joint(&joint)).1.len();
    let offset = joint.len() - table_len;
    joint[offset + 3..offset + 11].copy_from_slice(&(expected.epoch + 1).to_be_bytes());
    let bad = replace_section(&recovered.state, Section::ReputationAdmission, &joint);
    assert_eq!(
        queries::read_header(&bad, expected.chain, expected.program),
        Err(WRONG_ROSTER)
    );
    let mut identity = shared.feature_sections[1].to_vec();
    let (worker_bytes, evaluator_bytes) = ok(authority::split_identity_section(&identity));
    let snapshot_offset =
        worker_bytes.len() + 1 + usize::from(evaluator_bytes[0]) * authority::RECORD_BYTES;
    identity[snapshot_offset + 1..snapshot_offset + 9]
        .copy_from_slice(&(expected.epoch + 1).to_be_bytes());
    let bad = replace_section(&recovered.state, Section::IdentityRoster, &identity);
    assert_eq!(
        queries::read_header(&bad, expected.chain, expected.program),
        Err(WRONG_ROSTER)
    );
    let mut settlement = shared.feature_sections[3].to_vec();
    let offset = settlement
        .windows(32)
        .position(|v| v == expected.roster.as_bytes())
        .unwrap_or_else(|| panic!("missing F06 roster"));
    settlement[offset] ^= 1;
    let bad = replace_section(&recovered.state, Section::SettlementClaims, &settlement);
    assert_eq!(
        queries::read_header(&bad, expected.chain, expected.program),
        Err(WRONG_ROSTER)
    );
}

#[test]
fn availability_hides_enabled_features() {
    let mut expected = [Availability::Available; 11];
    expected[10] = Availability::NotEnabled;
    let mut w = World::first_open();
    let before = ok(queries::feature_availability(&w.bytes));
    assert_eq!(before[5], Availability::Available);
    assert_eq!(before[6], Availability::Available);
    assert_eq!(before[3], Availability::NotYetProduced);
    assert_eq!(before[4], Availability::NotYetProduced);
    w.score(800_000);
    w.settle();
    let capture = w.capture();
    assert_eq!(capture.projection().0, expected);
    let saved = Saved::new();
    saved.save(&capture);
    let facts = capture.facts;
    drop(capture);
    drop(w);
    assert_eq!(saved.reopen(facts).projection().0, expected);
    let recovered = saved.reopen(facts);
    let shared = ok(state::decode_shared_state(&recovered.state));
    let mut joint = shared.feature_sections[4].to_vec();
    let table_len = ok(split_joint(&joint)).1.len();
    let offset = joint.len() - table_len;
    joint[offset..offset + 2].copy_from_slice(&2u16.to_be_bytes());
    let bad = replace_section(&recovered.state, Section::ReputationAdmission, &joint);
    assert_eq!(
        ok(queries::feature_availability(&bad))[7],
        Availability::UnsupportedVersion
    );
    let bad = replace_section(&recovered.state, Section::CurrentReports, &[0xff]);
    assert_eq!(
        ok(queries::feature_availability(&bad))[3],
        Availability::ContentUnavailable
    );
}

#[test]
fn participant_projection_drops_market_state() {
    let mut w = World::first_open();
    let opened = w.capture();
    let (_, rows) = opened.projection();
    assert_eq!(rows.len(), 4);
    assert!(rows.iter().all(|r| r.frozen_member));
    assert_eq!(
        rows.iter()
            .filter(|r| r.kind == ParticipantKind::Evaluator)
            .count(),
        3
    );
    assert_eq!(rows[0].frozen_generation, Presence::Present(1));
    w.score(800_000);
    w.settle();
    let captured = w.capture();
    let (_, rows) = captured.projection();
    let worker = &rows[0];
    assert_eq!(worker.score.ppm(), Presence::Present(800_000));
    assert_eq!(worker.score.epoch(), Presence::Present(1));
    assert_eq!(worker.reward.status(), Availability::Available);
    assert_eq!(worker.reward.earned(), Presence::Present(100));
    assert_eq!(worker.reward.claimed(), Presence::Present(0));
    assert_eq!(worker.history_status, Availability::Available);
    assert_ne!(worker.history, Presence::Absent);
    let roster = ok(epoch::captured_roster(&ok(state::decode_shared_state(
        &captured.state,
    ))))
    .unwrap_or_else(|| panic!("missing roster"));
    let mut wire = vec![0; codec::ROSTER_MAX_BYTES];
    let len = ok(codec::encode_roster(&roster.view(), &mut wire));
    wire.truncate(len);
    let mut projected = vec![blank(); 40];
    assert_eq!(
        ok(queries::participant_rows(
            &captured.state,
            Some(&ok(codec::decode_roster(&wire))),
            &mut projected
        )),
        rows.len()
    );
    assert_eq!(&projected[..rows.len()], rows.as_slice());
    // Same-market supplied roster with different immutable metadata is refused.
    let mut workers = roster.workers().to_vec();
    workers[0].metadata = ok(MetadataDigest::new([0x46; 32]));
    let external = codec::Roster {
        workers: &workers,
        ..roster.view()
    };
    let len = ok(codec::encode_roster(&external, &mut wire));
    assert_eq!(
        queries::participant_rows(
            &captured.state,
            Some(&ok(codec::decode_roster(&wire[..len]))),
            &mut projected
        ),
        Err(WRONG_ROSTER)
    );
    let saved = Saved::new();
    saved.save(&captured);
    let facts = captured.facts;
    drop(captured);
    drop(w);
    assert_eq!(saved.reopen(facts).projection().1, rows);
}

#[test]
fn reputation_region_never_created() {
    let initial = World::create();
    let shared = ok(state::decode_shared_state(&initial.bytes));
    let region = ok(split_joint(shared.feature_sections[4]))
        .0
        .unwrap_or_else(|| panic!("CREATE omitted RP07"));
    assert_eq!(region.state.market, initial.market());
    assert_eq!(region.state.records().count(), 0);
    drop(initial);
    let mut w = World::first_open();
    w.score(800_000);
    w.settle();
    w.publish(WORKER, 360);
    w = reopen_world(w);
    w.open(2);
    w.score(800_000);
    w.settle();
    let captured = w.capture();
    let shared = ok(state::decode_shared_state(&captured.state));
    let history = ok(split_joint(shared.feature_sections[4]))
        .0
        .unwrap_or_else(|| panic!("missing RP07"));
    assert_eq!(history.state.latest_completed(), Presence::Present(2));
    assert!(history.state.lookup_epoch(1).is_ok());
    assert!(history.state.lookup_epoch(2).is_ok());
    let standing = history
        .state
        .records()
        .next()
        .unwrap_or_else(|| panic!("missing standing"));
    assert_eq!(standing.qualifying_count, 2);
    assert_eq!(standing.quality.get(), 187_500);
    let saved = Saved::new();
    saved.save(&captured);
    let facts = captured.facts;
    drop(captured);
    drop(w);
    let recovered = saved.reopen(facts);
    let shared = ok(state::decode_shared_state(&recovered.state));
    assert_eq!(ok(split_joint(shared.feature_sections[4])).0, Some(history));

    // A real zero-score terminal legitimately releases all F06 recipient slots.
    // Its immutable opening and produced zero entitlement must still be recoverable.
    let mut zero = World::first_open();
    let binding = zero.binding();
    zero.score(0);
    zero.settle();
    zero = reopen_world(zero);
    assert_eq!(zero.binding(), binding);
    let capture = zero.capture();
    let (_, rows) = capture.projection();
    assert_eq!(rows[0].score.ppm(), Presence::Present(0));
    assert_eq!(rows[0].reward.earned(), Presence::Present(0));
    assert_eq!(rows[0].history_status, Availability::Available);
    let saved = Saved::new();
    saved.save(&capture);
    let facts = capture.facts;
    drop(capture);
    assert_eq!(saved.reopen(facts).projection().1, rows);
    zero.publish(WORKER, 360);
    zero.open(2);
    assert_eq!(zero.binding().epoch, 2);
}

#[test]
fn aggregation_reconstructs_mutable_roster() {
    let mut w = World::first_open();
    let original = w.binding();
    let snapshot = ok(epoch::captured_roster(&ok(state::decode_shared_state(
        &w.bytes,
    ))))
    .unwrap_or_else(|| panic!("missing roster"))
    .workers()
    .to_vec();
    w.publish(WORKER, 270);
    w.rotate(WORKER, 280);
    assert_eq!(w.worker_record(WORKER).generation, 2);
    w = reopen_world(w);
    assert_eq!(w.binding(), original);
    assert_eq!(
        ok(epoch::captured_roster(&ok(state::decode_shared_state(
            &w.bytes
        ))))
        .unwrap_or_else(|| panic!("missing roster"))
        .workers(),
        snapshot
    );
    w.score(800_000);
    w.call(KEEPER, dispatch::BeginAggregation, &[], 352);
    let sealed = ok(aggregation::progress(&w.bytes));
    assert_eq!(sealed.cursor, 0);
    w = reopen_world(w);
    let Presence::Present(input) = sealed.input else {
        panic!("missing input")
    };
    let mut payload = input.bytes().to_vec();
    payload.extend_from_slice(&0u16.to_be_bytes());
    w.call(KEEPER, dispatch::ProcessAggregation, &payload, 353);
    assert_eq!(ok(aggregation::progress(&w.bytes)).cursor, 1);
    w = reopen_world(w);
    w.call(KEEPER, dispatch::FinalizeAggregation, input.as_bytes(), 354);
    let shared = ok(state::decode_shared_state(&w.bytes));
    let row = ok(ok(rewards::decode_reward_state(
        &shared.feature_sections[3][..REWARD_STATE_BYTES],
    ))
    .row(1));
    assert_eq!(row.status, EpochStatus::Terminal);
    assert_eq!(row.roster, original.roster);
    assert_eq!(row.entries()[0].entitlement, 100);
    assert_eq!(
        ok(aggregation::captured_aggregation(&w.bytes))
            .unwrap_or_else(|| panic!("missing output"))
            .output(0)
            .map(|v| v.generation().get()),
        Ok(1)
    );
}

fn admit_task(
    w: &mut World,
    worker: u8,
    nonce: u8,
    at: u64,
    metadata: MetadataDigest,
) -> Result<Vec<u8>, ApplicationError> {
    let binding = w.binding();
    let mut payload = binding.epoch.to_be_bytes().to_vec();
    payload.extend_from_slice(&binding.config.get().to_be_bytes());
    payload.extend_from_slice(ok(w.policy().current.digest()).as_bytes());
    payload.extend_from_slice(binding.roster.as_bytes());
    payload.extend_from_slice(principal(KEEPER).as_bytes());
    payload.extend_from_slice(w.worker(worker).as_bytes());
    payload.extend_from_slice(metadata.as_bytes());
    payload.extend_from_slice(&[nonce; 32]);
    payload.extend_from_slice(digest(0x42).as_bytes());
    payload.extend_from_slice(&(at + 8).to_be_bytes());
    w.route(KEEPER, dispatch::ADMIT_TASK, &payload, at)
}
#[test]
fn task_admission_uses_next_epoch_attributes() {
    let mut w = World::first_open();
    w.enroll(WORKER + 1, 260);
    w.publish(WORKER + 1, 280);
    w.publish(WORKER, 281);
    w.score(800_000);
    w.settle();
    w.open(2);
    let original = w.binding();
    let other_metadata = w.worker_record(WORKER + 1).metadata;
    let own_metadata = w.worker_record(WORKER).metadata;
    w.publish(WORKER, 395);
    w.rotate(WORKER, 400);
    assert_eq!(w.worker_record(WORKER).effective_epoch, 3);
    ok(admit_task(&mut w, WORKER + 1, 1, 401, other_metadata));
    w = reopen_world(w);
    assert_eq!(w.binding(), original);
    ok(admit_task(&mut w, WORKER + 1, 2, 402, other_metadata));
    ok(admit_task(&mut w, WORKER, 3, 403, own_metadata));
    let next_metadata = w.worker_record(WORKER).metadata;
    assert_eq!(
        admit_task(&mut w, WORKER, 4, 404, next_metadata),
        Err(WRONG_ROSTER)
    );
    // Revocation is checked live even though the immutable roster is unchanged.
    let r = w.worker_record(WORKER + 1);
    let mut revoke = r.worker.bytes().to_vec();
    revoke.extend_from_slice(&r.generation.to_be_bytes());
    revoke.push(1);
    revoke.extend_from_slice(&1u64.to_be_bytes());
    w.call(WORKER + 1, dispatch::RevokeDelegate, &revoke, 405);
    assert_eq!(
        admit_task(&mut w, WORKER + 1, 5, 406, other_metadata),
        Err(layerx_programs_ai_market::errors::F02_DELEGATE_REVOKED)
    );
    assert_eq!(w.binding(), original);
}

#[test]
fn build_default_targets_chain_229() {
    assert_eq!(layerx_programs_ai_market::MERGED_CHAIN_ID, 125);
    let expected = ok(queries::parse_hex32(
        "62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6",
    ));
    assert_eq!(
        layerx_programs_ai_market::merged_chain_domain(125),
        expected
    );
    assert_eq!(layerx_programs_ai_market::MERGED_CHAIN_DOMAIN, expected);
    match option_env!("PAXAI_CHAIN_DOMAIN") {
        None => assert_eq!(
            ok(layerx_programs_ai_market::deployment_chain_domain()).bytes(),
            expected
        ),
        Some(value)
            if value == "62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6" =>
        {
            assert_eq!(
                ok(layerx_programs_ai_market::deployment_chain_domain()).bytes(),
                expected
            )
        }
        Some(_) => assert_eq!(
            layerx_programs_ai_market::deployment_chain_domain(),
            Err(WRONG_DOMAIN)
        ),
    }
    let mut w = World::first_open();
    let local = w.encoded_call(KEEPER, dispatch::READ_HEADER, &[], 258);
    let decoded = ok(codec::decode_envelope(&local));
    let mut foreign = decoded.envelope;
    foreign.chain = ok(ChainDomain::new(
        layerx_programs_ai_market::merged_chain_domain(229),
    ));
    let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
    let n = ok(codec::encode_envelope(&foreign, &mut bytes));
    bytes.truncate(n);
    assert_eq!(w.route_encoded(KEEPER, 258, &bytes), Err(WRONG_DOMAIN));
    assert_ne!(
        ok(codec::derive_market(foreign.chain, foreign.program)),
        w.market()
    );
}
