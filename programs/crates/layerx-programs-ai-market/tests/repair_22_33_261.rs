//! Task 22.33.261 / audit 22.30.130 / acceptance 22998.1.
//! reward-finality-proof-origin -> reward_finality_proof_origin: committed routed reads,
//! production ICS23 IAVL/multistore verification, authenticated header and SnapshotBinding;
//! modified value, key, block and app-hash witnesses refuse before reward authorization.
//! reward-native-reopen -> reward_native_reopen: exact signed Claim and RefundFree requests
//! survive termination of the writer process and reopening production Storage, preserving
//! retained results, ledger balances, payout plans, events and reward rows without replanning
//! a second transfer. Native bank execution remains the wiring request below.
//! task-active-admission-fixture -> task_active_admission_fixture: routed lifecycle,
//! participant-specific enrollment, signed consent, approval and admission across epochs;
//! two-worker policy is staged before the final opening, and foreign approval is refused.
//! Wiring request (domain 13): tests/programs/test_ai_host_boundary.c must persist/reopen
//! its actual native store and expose signed Claim/Refund replies, recipient bank balances,
//! events and rows after lost replies; no native executable or guest artifact is required here.
//! Baseline aa5e0820b3652890f3812a220f4b91f4f8a45387: this target did not exist; no baseline
//! gate ran because the explicit order requires complete implementation before qualification.
//! The exact candidate revision, command, exit and private evidence log are recorded by the gate ledger.
//! Qualification blocker at c348ddaa3798a9a2a8e46e74427c2ed53b5d635f:
//! command = cargo check --locked --manifest-path /work/paxeer-monorepo/services/platform/Cargo.toml --all-targets -p layerx-ai-evaluator -p layerx-indexer -p layerx-paxai-worker -p layerx-platform-agent-boundary -p layerx-platform-authority -p layerx-platform-core -p layerx-platform-dashboard -p layerx-platform-gateway -p layerx-platform-registry -p layerx-platform-webhooks -p layerx-ramp-toolkit -p layerx-reference-ramp -p layerx-runtime-clock
//! exit = 101; log = /out/logs/dependents-1.log; hf-done exit = 7 before verify_cmd.
//! Observed: platform-core public_reads.rs uses unavailable Uint256::to_be_bytes,
//! PxAssetEntry.layerx/paxeer, incompatible asset-id types and a non-AsRef<str> asset key.
//! Assumption: these out-of-scope dependency errors require the platform domain; no focused tests ran.
//! Attempt 4 baseline: one of six cases passed; the continued fixture now proves and
//! respects F02's eight-height metadata cooldown before capturing the next epoch roster.
//! Attempt 7: direct rewards::apply read the joint F07 prefix as the F08 schema version
//! (BAD_VERSION); payouts now use dispatch's production epoch::carry_reputation adapter.
//! The commit child asserts the original refusal and compares the adapted state and event
//! with real dispatch before persisting either payout; recovery assertions remain intact.

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs_ai_market::{
    admission::{AdmissionTable, EvaluatorConsent, Participant},
    aggregation,
    aggregation_codec::AggregationPhase,
    codec::{self, CommitScorePayload, Envelope, ReportBody, RevealScorePayload, ScoreVector},
    dispatch::{self, Buffers, Operation, Routed, SequencePolicy},
    epoch,
    errors::ApplicationError,
    evaluators::authority,
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{self, FinalityEvidence, ReadProof, SnapshotBinding, StateCapture},
    registry::derive_rewards_account,
    registry_ops::{CallContext, PolicySection},
    reputation_transition::split_joint,
    rewards,
    state::{self, Section},
    tasks::{self, SetBinding, TaskSet},
    types::{
        AssetId, Authentication, ChainDomain, Digest32, EvaluatorBinding, EvaluatorId,
        EvidenceRoot, FrozenBinding, Presence, PrincipalId, ProgramId, PublicKey32, RequestId,
        RubricDigest, Salt32, Signature64, Version, WorkerId,
    },
    workers::{self, WorkerTable},
    MAX_CHUNK_BYTES, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
};
use std::{
    collections::BTreeMap,
    fs,
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

macro_rules! ok {
    ($value:expr) => {
        $value.unwrap_or_else(|e| panic!("{} refused: {e:?}", stringify!($value)))
    };
}
fn digest(n: u8) -> Digest32 {
    ok!(Digest32::new([n; 32]))
}
fn principal(n: u8) -> PrincipalId {
    let mut bytes = [0x50; 32];
    bytes[0] = n;
    ok!(PrincipalId::new(if n == OWNER { [12; 32] } else { bytes }))
}
fn key(n: u8) -> SigningKey {
    SigningKey::from_bytes(&[if n == 1 || n == 5 { 0x90 + n } else { n }; 32])
}
fn public(k: &SigningKey) -> PublicKey32 {
    PublicKey32(k.verifying_key().to_bytes())
}
const OWNER: u8 = 12;
const WORKER: u8 = 1;
const ORIGIN: u64 = 64;
const KEEPER: u8 = 127;

struct World {
    bytes: Vec<u8>,
    sequences: BTreeMap<u8, u64>,
    request: u64,
    last_event: Vec<u8>,
    last_result: Vec<u8>,
    last_frozen: Option<epoch::Frozen>,
}
impl World {
    fn context(actor: u8, height: u64) -> CallContext {
        CallContext {
            chain: ok!(ChainDomain::new([10; 32])),
            program: ok!(ProgramId::new([11; 32])),
            principal: principal(actor),
            height,
        }
    }
    fn policy(&self) -> PolicySection<'_> {
        ok!(PolicySection::decode(
            ok!(state::decode_shared_state(&self.bytes)).feature_sections[0],
        ))
    }
    fn market(&self) -> layerx_programs_ai_market::MarketId {
        ok!(codec::derive_market(
            Self::context(OWNER, ORIGIN).chain,
            Self::context(OWNER, ORIGIN).program,
        ))
    }
    fn worker(&self, n: u8) -> WorkerId {
        ok!(codec::derive_worker(self.market(), principal(n), [n; 32]))
    }
    fn evaluator(&self, n: u8) -> EvaluatorId {
        ok!(codec::derive_evaluator(
            self.market(),
            principal(n),
            [n; 32],
        ))
    }
    fn worker_record(&self, n: u8) -> workers::WorkerCurrent {
        let state = ok!(state::decode_shared_state(&self.bytes));
        let table = ok!(WorkerTable::decode(
            ok!(authority::split_identity_section(state.feature_sections[1])).0,
        ));
        table
            .get(self.worker(n))
            .unwrap_or_else(|| panic!("missing produced worker"))
    }
    fn binding(&self) -> FrozenBinding {
        ok!(epoch::captured_roster(&ok!(state::decode_shared_state(
            &self.bytes,
        ))))
        .unwrap_or_else(|| panic!("missing opening"))
        .binding
    }
    fn header(&mut self, height: u64) -> codec::ReadHeader {
        ok!(codec::decode_read_header(&self.call(
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
            let frozen = ok!(epoch::preview_open(
                &self.bytes,
                height,
                &mut next,
                &mut scratch,
            ));
            (frozen.epoch, Presence::Present(frozen.roster))
        } else if self.bytes.is_empty() {
            (0, Presence::Absent)
        } else {
            let state = ok!(state::decode_shared_state(&self.bytes));
            match ok!(epoch::captured_roster(&state)) {
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
            config: if read {
                0
            } else if self.bytes.is_empty() {
                1
            } else if op == dispatch::OPEN_EPOCH {
                self.last_frozen
                    .unwrap_or_else(|| panic!("opening requires its produced preview"))
                    .config
                    .get()
            } else {
                self.policy().header.active_config_version
            },
            roster,
            sequence,
            expiry: height + 1000,
            request: ok!(RequestId::new(request)),
            payload,
            authentication: Authentication::Native,
        };
        let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
        let n = ok!(codec::encode_envelope(&e, &mut bytes));
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
        let e = ok!(codec::decode_envelope(encoded));
        let mut next = vec![0xa5; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let mut result = vec![0; MAX_RESULT_BYTES];
        self.last_event.clear();
        self.last_result.clear();
        let routed = ok!(dispatch::route(
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
                    ok!(state::decode_shared_state(&next[..state_len])).revision,
                    revision
                );
                self.last_event = event[..event_len].to_vec();
                self.bytes = next[..state_len].to_vec();
                if e.envelope.sequence != 0 {
                    self.sequences.insert(actor, e.envelope.sequence);
                }
                result_len
            }
            Routed::Unchanged { result_len } => result_len,
            Routed::Refused { result_len } => {
                assert!(next.iter().all(|v| *v == 0xa5));
                let decoded = ok!(codec::decode_result(&result[..result_len]));
                return Err(decoded
                    .error
                    .unwrap_or_else(|| panic!("refusal without code")));
            }
        };
        self.last_result = result[..result_len].to_vec();
        let decoded = ok!(codec::decode_result(&result[..result_len]));
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
            last_event: Vec::new(),
            last_result: Vec::new(),
            last_frozen: None,
        };
        let mut policy = ok!(TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: digest(1),
                dataset_artifact: [2; 32],
                benchmark_suite: digest(3),
                rubric: ok!(RubricDigest::new([4; 32])),
                task_schema: digest(5),
                result_schema: digest(6),
                service_terms: digest(7),
            },
            100,
            1,
        ));
        policy.task_timeout_heights = 8;
        let mut policy_bytes = vec![0; TASK_POLICY_BYTES];
        ok!(policy.encode(&mut policy_bytes));
        let mut payload = principal(OWNER).bytes().to_vec();
        payload.extend_from_slice(&[13; 32]);
        payload.extend_from_slice(
            ok!(derive_rewards_account(
                Self::context(OWNER, ORIGIN).program,
                ok!(AssetId::new([13; 32])),
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
        let state = ok!(state::decode_shared_state(&self.bytes));
        let (_, table) = ok!(split_joint(state.feature_sections[4]));
        let table = ok!(AdmissionTable::decode(table));
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
        payload.extend_from_slice(&self.policy().header.active_config_version.to_be_bytes());
        payload.extend_from_slice(&(ORIGIN + effective * 128 + 64).max(at + 1).to_be_bytes());
        self.call(OWNER, dispatch::ApproveAdmission, &payload, at);
        let state = ok!(state::decode_shared_state(&self.bytes));
        let table = ok!(AdmissionTable::decode(
            ok!(split_joint(state.feature_sections[4])).1,
        ));
        let approval = table
            .get(participant)
            .and_then(|m| m.approval)
            .unwrap_or_else(|| panic!("missing produced approval"));
        (effective, approval.digest)
    }
    fn enroll_identity(&mut self, n: u8, at: u64) {
        let k = key(n);
        let mut payload = principal(n).bytes().to_vec();
        payload.extend_from_slice(&[n; 32]);
        payload.extend_from_slice(&public(&k).0);
        payload.extend_from_slice(&[0x44; 32]);
        payload.extend_from_slice(&(at + 120).to_be_bytes());
        self.call(OWNER, dispatch::EnrollWorker, &payload, at);
        let record = self.worker_record(n);
        let consent = ok!(workers::consent_digest(
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
        if n != WORKER {
            let mut foreign = payload.clone();
            let signature_at = foreign.len() - 64;
            foreign[signature_at..]
                .copy_from_slice(&key(WORKER).sign(consent.as_bytes()).to_bytes());
            let before = self.bytes.clone();
            assert_eq!(
                self.route(n, dispatch::AcceptEnrollment, &foreign, at + 1),
                Err(layerx_programs_ai_market::errors::BAD_SIGNATURE)
            );
            assert_eq!(self.bytes, before);
        }
        self.call(n, dispatch::AcceptEnrollment, &payload, at + 1);
    }
    fn enroll(&mut self, n: u8, at: u64) {
        self.enroll_identity(n, at);
        let record = self.worker_record(n);
        let (effective, approval) = self.approval(
            Participant::Worker(record.worker),
            n,
            record.delegate,
            ok!(record.proposal_digest(&self.policy().header)),
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
        let effective = if ok!(epoch::captured_roster(&ok!(state::decode_shared_state(
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
        let state = ok!(state::decode_shared_state(&self.bytes));
        let region = ok!(authority::evaluator_region(state.feature_sections[1]));
        let grant = region
            .get(self.evaluator(n))
            .unwrap_or_else(|| panic!("missing nomination"))
            .grant;
        let mut bytes = vec![0; layerx_programs_ai_market::evaluators::model::GRANT_BYTES];
        let len = ok!(layerx_programs_ai_market::evaluators::codec::encode_grant(
            &grant, &mut bytes,
        ));
        let identity = ok!(codec::domain_hash(
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
            request: ok!(RequestId::new({
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
        ok!(consent.encode(&mut bytes));
        let mut payload = bytes.to_vec();
        payload.extend_from_slice(&key(n).sign(ok!(consent.digest()).as_bytes()).to_bytes());
        self.call(n, dispatch::AdmitEvaluator, &payload, at + 2);
    }
    fn try_publish(&mut self, n: u8, at: u64) -> Result<Vec<u8>, ApplicationError> {
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
        let m = ok!(workers::decode_manifest(&manifest));
        let mut payload = r.worker.bytes().to_vec();
        payload.extend_from_slice(&r.metadata_revision.to_be_bytes());
        payload.extend_from_slice(&m.revision.to_be_bytes());
        payload.extend_from_slice(m.digest.as_bytes());
        payload.extend_from_slice(&m.valid_from.to_be_bytes());
        payload.extend_from_slice(&m.expiry.to_be_bytes());
        payload.extend_from_slice(&(manifest.len() as u32).to_be_bytes());
        payload.extend_from_slice(&manifest);
        let mut encoded = self.encoded_call(n, dispatch::PublishMetadata, &payload, at);
        let signed = ok!(codec::decode_envelope(&encoded));
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
        let len = ok!(codec::encode_envelope(&env, &mut bytes));
        let request = ok!(ok!(codec::decode_envelope(&bytes[..len])).request_digest());
        env.authentication = Authentication::Delegate {
            key: r.delegate,
            signature: Signature64(k.sign(request.as_bytes()).to_bytes()),
        };
        let len = ok!(codec::encode_envelope(&env, &mut bytes));
        bytes.truncate(len);
        encoded = bytes;
        self.route_encoded(n, at, &encoded)
    }
    fn publish(&mut self, n: u8, at: u64) {
        ok!(self.try_publish(n, at));
    }
    fn open(&mut self, epoch: u64) {
        let mut next = vec![0; MAX_STATE_BYTES];
        let mut scratch = vec![0; epoch::OPEN_SCRATCH_BYTES];
        self.last_frozen = Some(ok!(epoch::preview_open(
            &self.bytes,
            ORIGIN + epoch * 128,
            &mut next,
            &mut scratch,
        )));
        self.call(KEEPER, dispatch::OPEN_EPOCH, &[], ORIGIN + epoch * 128);
        assert_eq!(self.binding().epoch, epoch);
    }
    fn first_open() -> Self {
        let mut w = Self::create();
        let mut schedule = 1u64.to_be_bytes().to_vec();
        schedule.extend_from_slice(&1u64.to_be_bytes());
        w.call(OWNER, dispatch::SCHEDULE_ACTIVATION, &schedule, 65);
        w.enroll(WORKER, 66);
        for n in 2..=4 {
            w.nominate(n, 64 + 4 * u64::from(n));
        }
        w.publish(WORKER, 84);
        let mut fund = 500u128.to_be_bytes().to_vec();
        fund.extend_from_slice(&[14; 32]);
        fund.extend_from_slice(&1u64.to_be_bytes());
        fund.push(1);
        w.call(OWNER, dispatch::FUND, &fund, 90);
        w.open(1);
        let mut activate = ok!(state::decode_shared_state(&w.bytes))
            .revision
            .to_be_bytes()
            .to_vec();
        activate.extend_from_slice(&1u64.to_be_bytes());
        w.call(KEEPER, dispatch::ADVANCE_ACTIVATION, &activate, 193);
        assert_eq!(
            w.policy().header.lifecycle,
            layerx_programs_ai_market::registry_ops::ACTIVE
        );
        w
    }
    fn score(&mut self, ppm: u32) {
        let binding = self.binding();
        let start = ORIGIN + binding.epoch * 128;
        let policy = self.policy().current.digest();
        let policy = ok!(policy);
        let set_binding = SetBinding {
            market: binding.market,
            epoch: binding.epoch,
            config: binding.config,
            policy,
            roster: binding.roster,
        };
        let task_region = self.policy().task_region.to_vec();
        let set = ok!(tasks::task_set_digest(
            &set_binding,
            &ok!(TaskSet::decode(&task_region)),
        ));
        let mut payload = binding.epoch.to_be_bytes().to_vec();
        payload.extend_from_slice(&binding.config.get().to_be_bytes());
        payload.extend_from_slice(set.as_bytes());
        self.call(KEEPER, dispatch::SEAL_TASK_SET, &payload, start + 64);
        let mut frozen_workers = ok!(epoch::captured_roster(&ok!(state::decode_shared_state(
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
                grant: ok!(Version::new(1)),
                key_version: ok!(Version::new(1)),
            };
            let evidence = ok!(EvidenceRoot::new([0xe0 + n; 32]));
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
            let report = ok!(codec::report_digest(&body));
            let commitment = ok!(codec::commitment_digest(
                &evaluator_binding,
                report,
                ok!(Salt32::new([0x5a; 32])),
            ));
            let mut commit =
                vec![0; layerx_programs_ai_market::commit_reveal::commitment::COMMIT_SCORE_BYTES];
            ok!(codec::encode_commit_score(
                &CommitScorePayload {
                    binding: evaluator_binding,
                    commitment,
                },
                &mut commit,
            ));
            self.call(n, dispatch::CommitScore, &commit, start + 64);
            let signature = Signature64(
                key(n)
                    .sign(ok!(codec::attestation_digest(report)).as_bytes())
                    .to_bytes(),
            );
            let mut reveal =
                vec![0; layerx_programs_ai_market::commit_reveal::commitment::REVEAL_MAX_BYTES];
            let len = ok!(codec::encode_reveal_score(
                &RevealScorePayload {
                    report: body,
                    signature,
                    salt: ok!(Salt32::new([0x5a; 32])),
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
            let p = ok!(aggregation::progress(&self.bytes));
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
        let Presence::Present(input) = ok!(aggregation::progress(&self.bytes)).input else {
            panic!("missing input")
        };
        self.call(KEEPER, dispatch::FinalizeAggregation, input.as_bytes(), at);
        assert_eq!(
            ok!(aggregation::progress(&self.bytes)).phase,
            AggregationPhase::Terminal
        );
    }
}

pub(crate) struct ExecutionFixture {
    pub bytes: Vec<u8>,
    pub owner_sequence: u64,
    pub frozen: epoch::Frozen,
    pub workers: Vec<layerx_programs_ai_market::WorkerRosterEntry>,
}

fn execution_world() -> World {
    let mut w = World::first_open();
    w.enroll(5, 194);
    let before = w.bytes.clone();
    assert_eq!(
        w.try_publish(5, 198),
        Err(layerx_programs_ai_market::errors::F02_RATE_LIMITED)
    );
    assert_eq!(w.bytes, before);
    w.publish(1, 199);
    w.publish(5, 194 + workers::METADATA_COOLDOWN);
    let mut policy = w.policy().current;
    policy.config_version = 2;
    policy.minimum_worker_count = 2;
    let mut payload = ok!(state::decode_shared_state(&w.bytes))
        .revision
        .to_be_bytes()
        .to_vec();
    let mut encoded = vec![0; TASK_POLICY_BYTES];
    ok!(policy.encode(&mut encoded));
    payload.extend_from_slice(&encoded);
    payload.extend_from_slice(&2u64.to_be_bytes());
    w.call(OWNER, dispatch::STAGE_POLICY, &payload, 210);
    w.score(1_000_000);
    w.settle();
    w.open(2);
    assert_eq!(w.policy().current.minimum_worker_count, 2);
    assert_eq!(w.policy().current.minimum_evaluator_count, 3);
    assert_eq!(w.policy().current.task_timeout_heights, 8);
    w
}

pub(crate) fn execution_height_fixture() -> ExecutionFixture {
    let w = execution_world();
    let frozen = w
        .last_frozen
        .unwrap_or_else(|| panic!("missing final opening"));
    let captured = ok!(epoch::captured_roster(&ok!(state::decode_shared_state(
        &w.bytes,
    ))))
    .unwrap_or_else(|| panic!("missing produced roster"));
    let workers = [1, 5]
        .into_iter()
        .map(|n| {
            *captured
                .workers()
                .iter()
                .find(|r| r.worker == w.worker(n))
                .unwrap_or_else(|| panic!("worker missing from produced roster"))
        })
        .collect();
    ExecutionFixture {
        bytes: w.bytes,
        owner_sequence: w.sequences[&OWNER] + 1,
        frozen,
        workers,
    }
}

#[test]
pub(crate) fn task_active_admission_fixture() {
    let mut w = execution_world();
    let state = ok!(state::decode_shared_state(&w.bytes));
    let admission = ok!(AdmissionTable::decode(
        ok!(split_joint(state.feature_sections[4])).1,
    ));
    let first = admission
        .get(Participant::Worker(w.worker(1)))
        .unwrap_or_else(|| panic!("first admission"));
    let second = admission
        .get(Participant::Worker(w.worker(5)))
        .unwrap_or_else(|| panic!("second admission"));
    assert_ne!(first.owner, second.owner);
    assert_ne!(first.participant, second.participant);
    assert_eq!(first.admission_height, Some(69));
    assert_eq!(second.admission_height, Some(197));
    assert!(first.admitted() && second.admitted());
    assert_eq!(
        (
            w.worker_record(1).last_sequence,
            w.worker_record(5).last_sequence
        ),
        (4, 3)
    );
    assert_eq!(
        w.policy().header.lifecycle,
        layerx_programs_ai_market::registry_ops::ACTIVE
    );
    assert_eq!(w.binding().epoch, 2);

    // A fresh worker cannot consume another participant's signed approval.
    w.enroll_identity(6, 322);
    w.enroll_identity(7, 326);
    let record = w.worker_record(6);
    let (effective, foreign_approval) = w.approval(
        Participant::Worker(record.worker),
        6,
        record.delegate,
        ok!(record.proposal_digest(&w.policy().header)),
        328,
    );
    let other = w.worker_record(7);
    let (_, own_approval) = w.approval(
        Participant::Worker(other.worker),
        7,
        other.delegate,
        ok!(other.proposal_digest(&w.policy().header)),
        329,
    );
    assert_ne!(foreign_approval, own_approval);
    let mut payload = other.worker.bytes().to_vec();
    for value in [1, effective, 2] {
        payload.extend_from_slice(&value.to_be_bytes());
    }
    payload.extend_from_slice(foreign_approval.as_bytes());
    let before = w.bytes.clone();
    assert_eq!(
        w.route(7, dispatch::AdmitWorker, &payload, 330),
        Err(layerx_programs_ai_market::errors::F08_BAD_CONSENT)
    );
    assert_eq!(w.bytes, before);
    let digest_at = payload.len() - 32;
    payload[digest_at..].copy_from_slice(own_approval.as_bytes());
    w.call(7, dispatch::AdmitWorker, &payload, 330);

    let mut w = execution_world();
    let worker = w.worker_record(1);
    let mut payload = 2u64.to_be_bytes().to_vec();
    payload.extend_from_slice(&2u64.to_be_bytes());
    payload.extend_from_slice(ok!(w.policy().current.digest()).as_bytes());
    payload.extend_from_slice(w.binding().roster.as_bytes());
    payload.extend_from_slice(principal(0x21).as_bytes());
    payload.extend_from_slice(worker.worker.as_bytes());
    payload.extend_from_slice(worker.metadata.as_bytes());
    payload.extend_from_slice(&[1; 32]);
    payload.extend_from_slice(&[0xa1; 32]);
    payload.extend_from_slice(&332u64.to_be_bytes());
    w.call(0x21, dispatch::ADMIT_TASK, &payload, 324);
    let task = ok!(codec::derive_task(w.market(), 2, principal(0x21), [1; 32]));
    let mut step = ok!(state::decode_shared_state(&w.bytes))
        .revision
        .to_be_bytes()
        .to_vec();
    step.extend_from_slice(task.as_bytes());
    step.extend_from_slice(&[0xa5; 32]);
    w.call(1, dispatch::ACCEPT_TASK, &step, 325);
    let mut step = ok!(state::decode_shared_state(&w.bytes))
        .revision
        .to_be_bytes()
        .to_vec();
    step.extend_from_slice(task.as_bytes());
    step.extend_from_slice(&[0xe5; 32]);
    w.call(1, dispatch::COMMIT_TASK_RESULT, &step, 329);
    let set = ok!(TaskSet::decode(w.policy().task_region));
    let record = ok!(set.position(task))
        .1
        .unwrap_or_else(|| panic!("produced task missing"));
    assert_eq!(record.deadline, 332);
    assert_eq!(record.status, tasks::TaskStatus::ResultCommitted);
}

// Compile the production verifier modules directly: the task does not authorize a
// manifest change. This test-local error carrier leaves every decoding and hash check intact.
mod chain_proof {
    #[derive(Clone, Copy, Debug, Eq, PartialEq)]
    pub enum ChainProofError {
        Truncated,
        NonCanonical,
        UnknownField,
        Mismatch,
    }
    pub mod proto {
        include!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../layerx-programs-registry/src/chain_proof/proto.rs"
        ));
    }
    pub mod ics23 {
        include!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../layerx-programs-registry/src/chain_proof/ics23.rs"
        ));
    }
    pub mod multistore {
        include!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../layerx-programs-registry/src/chain_proof/multistore.rs"
        ));
    }
    pub mod header {
        include!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../layerx-programs-registry/src/chain_proof/header.rs"
        ));
    }
}

use chain_proof::{ics23, multistore};
use layerx_programs_ai_market::{
    value_adapter::{self, RewardsAccount, ValueAction},
    AccountId,
};
use layerx_programs_runtime::storage::{Storage, StorageNamespace};
use sha2::{Digest, Sha256};

fn namespace() -> StorageNamespace {
    StorageNamespace::shared(ok!(layerx_programs_runtime::storage::ProgramId::new(
        [11; 32],
    )))
}
fn store_value(store: &mut Storage, key: &[u8]) -> Vec<u8> {
    ok!(store.transaction(namespace()).read(key))
        .unwrap_or_else(|| panic!("missing committed store value"))
}
fn commit_market(store: &mut Storage, state: &[u8]) {
    let mut tx = store.transaction(namespace());
    ok!(tx.write(layerx_programs_ai_market::SHARED_STATE_KEY, state));
    assert_eq!(tx.commit(), 1);
}
fn varint(mut n: usize) -> Vec<u8> {
    let mut out = Vec::new();
    while n >= 128 {
        out.push((n as u8 & 127) | 128);
        n >>= 7;
    }
    out.push(n as u8);
    out
}
fn protobuf_bytes(field: u8, value: &[u8]) -> Vec<u8> {
    let mut out = vec![field * 8 + 2];
    out.extend(varint(value.len()));
    out.extend_from_slice(value);
    out
}
fn leaf_proof(key: &[u8], value: &[u8], prefix: &[u8]) -> (Vec<u8>, [u8; 32]) {
    let mut hash = Sha256::new();
    hash.update(prefix);
    hash.update(varint(key.len()));
    hash.update(key);
    hash.update([32]);
    hash.update(Sha256::digest(value));
    let root = hash.finalize().into();
    let mut leaf = vec![8, 1, 24, 1, 32, 1];
    leaf.extend(protobuf_bytes(5, prefix));
    let mut proof = protobuf_bytes(1, key);
    proof.extend(protobuf_bytes(2, value));
    proof.extend(protobuf_bytes(3, &leaf));
    (proof, root)
}

struct CommittedRead {
    key: Vec<u8>,
    value: Vec<u8>,
    existence: Vec<u8>,
    store_root: [u8; 32],
    store_proof: Vec<u8>,
    header: chain_proof::header::ChainHeader,
    block: [u8; 32],
    signature: ed25519_dalek::Signature,
}
impl CommittedRead {
    fn produce(store: &mut Storage, height: u64) -> Self {
        let value = store_value(store, layerx_programs_ai_market::SHARED_STATE_KEY);
        let mut key = namespace().canonical_bytes();
        key.extend_from_slice(layerx_programs_ai_market::SHARED_STATE_KEY);
        let (existence, store_root) = leaf_proof(&key, &value, &[0, 2, 2]);
        let (simple, app_hash) = leaf_proof(b"lxphost", &store_root, &[0]);
        let store_proof = protobuf_bytes(1, &simple);
        let header = chain_proof::header::ChainHeader {
            version: chain_proof::header::HeaderVersion { block: 11, app: 1 },
            chain_id: "paxeer-programs-committed-fixture".to_owned(),
            height: i64::try_from(height).unwrap_or_else(|e| panic!("height: {e}")),
            time: chain_proof::header::HeaderTime {
                seconds: 1_700_000_000,
                nanos: 0,
            },
            last_block_id: chain_proof::header::HeaderBlockId {
                hash: vec![1; 32],
                part_set_total: 1,
                part_set_hash: vec![2; 32],
            },
            last_commit_hash: vec![3; 32],
            data_hash: vec![4; 32],
            validators_hash: Sha256::digest(key_for_commit().verifying_key().to_bytes()).to_vec(),
            next_validators_hash: Sha256::digest(key_for_commit().verifying_key().to_bytes())
                .to_vec(),
            consensus_hash: vec![5; 32],
            app_hash: app_hash.to_vec(),
            last_results_hash: vec![6; 32],
            evidence_hash: vec![7; 32],
            proposer_address: vec![8; 20],
        };
        let block = ok!(header.hash());
        let signature = key_for_commit().sign(&block);
        Self {
            key,
            value,
            existence,
            store_root,
            store_proof,
            header,
            block,
            signature,
        }
    }
    fn verify(&self, expected_key: &[u8], expected_value: &[u8], expected_block: [u8; 32]) -> bool {
        let Ok(block) = self.header.hash() else {
            return false;
        };
        block == expected_block
            && self.block == expected_block
            && key_for_commit()
                .verifying_key()
                .verify_strict(&block, &self.signature)
                .is_ok()
            && ics23::ExistenceProof::decode(&self.existence)
                .and_then(|proof| {
                    ics23::verify_existence(
                        &proof,
                        &ics23::ProofSpec::iavl(),
                        &self.store_root,
                        expected_key,
                        expected_value,
                    )
                })
                .is_ok()
            && <[u8; 32]>::try_from(self.header.app_hash.as_slice()).is_ok_and(|app| {
                multistore::verify_store_root("lxphost", self.store_root, &self.store_proof, app)
                    .is_ok()
            })
    }
    fn capture(&self, w: &mut World, trusted_block: [u8; 32]) -> SnapshotBinding {
        assert!(
            self.verify(&self.key, &w.bytes, trusted_block),
            "authenticate before binding or authorizing rewards"
        );
        let proof = ReadProof {
            chain: World::context(KEEPER, 0).chain,
            program: World::context(KEEPER, 0).program,
            native_state_root: ok!(Digest32::new(self.store_root)),
            observed_sequence: ok!(state::decode_shared_state(&self.value)).revision,
            execution_height: u64::try_from(self.header.height)
                .unwrap_or_else(|e| panic!("height: {e}")),
            block_hash: ok!(Digest32::new(self.block)),
        };
        let mut output = vec![0; MAX_STATE_BYTES];
        let mut capture = StateCapture::new(&mut output);
        let mut offset = 0;
        let mut pin = (0, Presence::Absent);
        while offset < self.value.len() {
            let mut payload = [0; 46];
            let len = ok!(codec::encode_chunk_request(
                &codec::ChunkRequest {
                    revision: pin.0,
                    digest: pin.1,
                    offset: offset as u32,
                    requested: MAX_CHUNK_BYTES as u16,
                },
                &mut payload,
            ));
            let body = w.call(
                KEEPER,
                dispatch::READ_STATE_CHUNK,
                &payload[..len],
                proof.execution_height,
            );
            let response = ok!(codec::decode_chunk_response(&body));
            assert_eq!(
                response.bytes,
                &self.value[offset..offset + response.bytes.len()]
            );
            ok!(capture.accept(&proof, &body));
            pin = (response.revision, Presence::Present(response.digest));
            offset += response.bytes.len();
        }
        let (bytes, facts) = ok!(capture.finish());
        let mut proof_bytes = self.existence.clone();
        proof_bytes.extend_from_slice(&self.store_proof);
        let binding = ok!(queries::bind_snapshot(
            bytes,
            &facts,
            &FinalityEvidence {
                native_state_root: proof.native_state_root,
                block_height: proof.execution_height,
                block_hash: proof.block_hash,
                app_hash: ok!(Digest32::new(
                    self.header
                        .app_hash
                        .as_slice()
                        .try_into()
                        .unwrap_or_else(|_| panic!("app hash length")),
                )),
                store_proof_digest: ok!(Digest32::new(Sha256::digest(&proof_bytes).into())),
                rank: queries::FINALIZED_RANK,
            },
            1000,
        ));
        ok!(binding.verify_capture(bytes, &facts));
        ok!(binding.require_finalized());
        binding
    }
}
fn key_for_commit() -> SigningKey {
    SigningKey::from_bytes(&[0xc1; 32])
}
fn settled() -> World {
    let mut w = World::first_open();
    w.score(1_000_000);
    w.settle();
    w
}
fn ledger(bytes: &[u8]) -> rewards::RewardLedger {
    let shared = ok!(state::decode_shared_state(bytes));
    ok!(ok!(rewards::decode_reward_state(
        shared.feature_sections[Section::SettlementClaims.index()]
            .get(..rewards::REWARD_STATE_BYTES)
            .unwrap_or_else(|| panic!("reward head missing")),
    ))
    .ledger())
}
fn claim_payload(w: &World) -> Vec<u8> {
    let record = w.worker_record(WORKER);
    let mut payload = record.worker.bytes().to_vec();
    payload.extend_from_slice(record.owner.as_bytes());
    payload.extend_from_slice(&100u128.to_be_bytes());
    payload
}

#[test]
pub(crate) fn reward_finality_proof_origin() {
    let mut w = settled();
    let mut store = Storage::default();
    commit_market(&mut store, &w.bytes);
    let mut read = CommittedRead::produce(&mut store, 289);
    let expected_block = read.block;
    let mut value = read.value.clone();
    let last = value.len() - 1;
    value[last] ^= 1;
    assert!(!read.verify(&read.key, &value, expected_block));
    let mut key = read.key.clone();
    key[0] ^= 1;
    assert!(!read.verify(&key, &read.value, expected_block));
    let mut block = expected_block;
    block[0] ^= 1;
    assert!(!read.verify(&read.key, &read.value, block));
    read.header.app_hash[0] ^= 1;
    assert!(!read.verify(&read.key, &read.value, expected_block));
    read.header.app_hash[0] ^= 1;
    let mut wrong_app: [u8; 32] = read
        .header
        .app_hash
        .as_slice()
        .try_into()
        .unwrap_or_else(|_| panic!("app hash"));
    wrong_app[0] ^= 1;
    assert!(multistore::verify_store_root(
        "lxphost",
        read.store_root,
        &read.store_proof,
        wrong_app
    )
    .is_err());
    read.header.height += 1;
    assert!(!read.verify(&read.key, &read.value, expected_block));
    read.header.height -= 1;
    let mut corrupted_store = read.store_proof.clone();
    let at = corrupted_store.len() - 1;
    corrupted_store[at] ^= 1;
    assert!(multistore::verify_store_root(
        "lxphost",
        read.store_root,
        &corrupted_store,
        read.header
            .app_hash
            .as_slice()
            .try_into()
            .unwrap_or_else(|_| panic!("app hash"))
    )
    .is_err());
    assert!(read.verify(&read.key, &read.value, expected_block));
    let binding = read.capture(&mut w, expected_block);
    assert_eq!(binding.state_digest, ok!(codec::state_digest(&w.bytes)));
    assert_eq!(binding.block_hash.bytes(), expected_block);
    assert_eq!(binding.native_state_root.bytes(), read.store_root);
    assert_eq!(binding.block_height, 289);
    assert_eq!(
        binding.app_hash.bytes().as_slice(),
        read.header.app_hash.as_slice()
    );
    let before = ledger(&w.bytes);
    let payload = claim_payload(&w);
    w.call(
        KEEPER,
        dispatch::CLAIM,
        &payload,
        binding.execution_height + 1,
    );
    assert_eq!(ledger(&w.bytes).total_claimed, before.total_claimed + 100);
}

const RECOVERY_BYTES: usize = 1 << 20;

fn signed_request(w: &mut World, op: Operation, payload: &[u8], at: u64) -> (Vec<u8>, [u8; 64]) {
    let encoded = w.encoded_call(KEEPER, op, payload, at);
    let signature = key(KEEPER).sign(&encoded).to_bytes();
    (encoded, signature)
}
fn authenticate_request(encoded: &[u8], signature: &[u8]) {
    let signature = ed25519_dalek::Signature::from_slice(signature)
        .unwrap_or_else(|e| panic!("signature encoding: {e}"));
    ok!(key(KEEPER)
        .verifying_key()
        .verify_strict(encoded, &signature));
    assert_eq!(
        ok!(codec::decode_envelope(encoded)).envelope.actor,
        principal(KEEPER)
    );
}
fn commit_payout(
    w: &mut World,
    store: &mut Storage,
    label: &[u8],
    encoded: &[u8],
    signature: &[u8],
    at: u64,
) {
    authenticate_request(encoded, signature);
    let envelope = ok!(codec::decode_envelope(encoded));
    let before = ledger(&w.bytes);
    let held = before.tracked_deposits - before.total_claimed - before.tracked_refunds;
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; rewards::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let shared = ok!(state::decode_shared_state(&w.bytes));
    let (region, admission) = ok!(split_joint(
        shared.feature_sections[Section::ReputationAdmission.index()]
    ));
    assert!(
        region.is_some(),
        "settlement must retain its produced F07 region"
    );
    ok!(AdmissionTable::decode(admission));
    assert_eq!(
        rewards::apply(
            &World::context(KEEPER, at),
            &envelope,
            &w.bytes,
            &mut next,
            &mut scratch,
            &mut event,
        )
        .err(),
        Some(layerx_programs_ai_market::errors::BAD_VERSION),
        "the F06 decoder requires the F08 table, not the joint F07/F08 prefix"
    );
    let mut carry = vec![0; epoch::CARRY_SCRATCH_BYTES];
    let (outcome, carried_len) = ok!(epoch::carry_reputation(
        &w.bytes,
        &mut next,
        &mut carry,
        |current, next| {
            let outcome = ok!(rewards::apply(
                &World::context(KEEPER, at),
                &envelope,
                current,
                next,
                &mut scratch,
                &mut event,
            ));
            let state_len = match &outcome {
                rewards::Outcome::Applied { state_len, .. } => Some(*state_len),
                _ => None,
            };
            Ok((outcome, state_len))
        },
    ));
    let rewards::Outcome::Applied {
        effect,
        after,
        event_len,
        response,
        ..
    } = outcome
    else {
        panic!("payout must apply once");
    };
    let state_len = carried_len.unwrap_or_else(|| panic!("payout must produce a carried state"));
    let account = ok!(RewardsAccount::for_ledger(
        World::context(KEEPER, at).program,
        &before,
    ));
    let action = ok!(value_adapter::plan(&account, &before, &after, effect));
    let ValueAction::Pay { recipient, amount } = action else {
        panic!("outward payout plan required");
    };
    assert_eq!(amount, if label == b"claim" { 100 } else { 400 });
    assert_eq!(
        recipient,
        if label == b"claim" {
            ok!(AccountId::new(principal(WORKER).bytes()))
        } else {
            ok!(AccountId::new([14; 32]))
        }
    );
    assert_eq!(ok!(value_adapter::cover(&after, held, action)), 0);
    let body = ok!(w.route_encoded(KEEPER, at, encoded));
    assert_eq!(body, response.as_bytes());
    assert_eq!(w.bytes, next[..state_len]);
    assert_eq!(w.last_event, event[..event_len]);
    assert!(!w.last_event.is_empty());
    assert_eq!(ledger(&w.bytes), after);
    let mut plan = recipient.bytes().to_vec();
    plan.extend_from_slice(&amount.to_be_bytes());
    let mut tx = store.transaction(namespace());
    ok!(tx.write(layerx_programs_ai_market::SHARED_STATE_KEY, &w.bytes));
    for (suffix, bytes) in [
        (b"-request".as_slice(), encoded),
        (b"-signature", signature),
        (b"-result", w.last_result.as_slice()),
        (b"-event", w.last_event.as_slice()),
        (b"-plan", plan.as_slice()),
    ] {
        let mut key = label.to_vec();
        key.extend_from_slice(suffix);
        ok!(tx.write(&key, bytes));
    }
    assert_eq!(tx.commit(), 6);
    // The application reply is discarded; recovery reads only the committed store.
    drop(body);
}
fn write_recovery_store(path: &std::path::Path) {
    let mut w = settled();
    let mut store = Storage::default();
    commit_market(&mut store, &w.bytes);
    let payload = claim_payload(&w);
    let (claim, signature) = signed_request(&mut w, dispatch::CLAIM, &payload, 290);
    commit_payout(&mut w, &mut store, b"claim", &claim, &signature, 290);
    let mut close = ok!(state::decode_shared_state(&w.bytes))
        .revision
        .to_be_bytes()
        .to_vec();
    close.extend_from_slice(&[0xcc; 32]);
    w.call(OWNER, dispatch::REQUEST_CLOSE, &close, 291);
    let mut refund = 0u128.to_be_bytes().to_vec();
    refund.extend_from_slice(&400u128.to_be_bytes());
    refund.extend_from_slice(&[14; 32]);
    let (refund, signature) = signed_request(&mut w, dispatch::REFUND_FREE, &refund, 292);
    commit_payout(&mut w, &mut store, b"refund", &refund, &signature, 292);
    assert_eq!(
        (
            ledger(&w.bytes).total_claimed,
            ledger(&w.bytes).tracked_refunds
        ),
        (100, 400)
    );
    assert_eq!(
        (
            ledger(&w.bytes).free,
            ledger(&w.bytes).reserved,
            ledger(&w.bytes).liability
        ),
        (0, 0, 0)
    );
    let encoded = ok!(store.replay_state_bytes(RECOVERY_BYTES));
    let mut file = ok!(std::fs::File::create(path));
    use std::io::Write as _;
    ok!(file.write_all(&encoded));
    ok!(file.sync_all());
}
fn reopen_recovery_store(path: &std::path::Path) {
    let encoded = ok!(fs::read(path));
    let mut store = ok!(Storage::decode_untrusted_replay_state(
        &encoded,
        RECOVERY_BYTES,
    ));
    let committed = store_value(&mut store, layerx_programs_ai_market::SHARED_STATE_KEY);
    let mut w = World {
        bytes: committed.clone(),
        sequences: BTreeMap::new(),
        request: 0,
        last_event: Vec::new(),
        last_result: Vec::new(),
        last_frozen: None,
    };
    let settled = ledger(&committed);
    assert_eq!(
        (
            settled.tracked_deposits,
            settled.total_claimed,
            settled.tracked_refunds
        ),
        (500, 100, 400)
    );
    let shared = ok!(state::decode_shared_state(&committed));
    let rewards = ok!(rewards::decode_reward_state(
        shared.feature_sections[Section::SettlementClaims.index()]
            .get(..rewards::REWARD_STATE_BYTES)
            .unwrap_or_else(|| panic!("reward head missing")),
    ));
    let row = ok!(rewards.row(1));
    assert_eq!(row.paid_sum, 100);
    assert_eq!(row.entries().len(), 1);
    assert_eq!(row.entries()[0].disposition, rewards::Disposition::Claimed);
    let Presence::Present(allocation) = row.allocation else {
        panic!("allocation missing");
    };
    let entitlement = ok!(rewards::entitlement_id(allocation, w.worker(WORKER)));
    for (label, at) in [(b"claim".as_slice(), 293), (b"refund".as_slice(), 294)] {
        let get = |store: &mut Storage, suffix: &[u8]| {
            let mut key = label.to_vec();
            key.extend_from_slice(suffix);
            store_value(store, &key)
        };
        let request = get(&mut store, b"-request");
        let signature = get(&mut store, b"-signature");
        let original_result = get(&mut store, b"-result");
        let original_event = get(&mut store, b"-event");
        let original_plan = get(&mut store, b"-plan");
        authenticate_request(&request, &signature);
        let mut corrupted = request.clone();
        let last = corrupted.len() - 1;
        corrupted[last] ^= 1;
        assert!(key(KEEPER)
            .verifying_key()
            .verify_strict(
                &corrupted,
                &ed25519_dalek::Signature::from_slice(&signature)
                    .unwrap_or_else(|e| panic!("signature: {e}"))
            )
            .is_err());
        let body = ok!(w.route_encoded(KEEPER, at, &request));
        assert_eq!(
            ok!(codec::decode_result(&w.last_result)).status,
            codec::ResultStatus::AlreadyApplied
        );
        let original = ok!(codec::decode_result(&original_result));
        let expected = if label == b"claim" {
            entitlement.bytes()
        } else {
            original.digest.bytes()
        };
        assert_eq!(body, expected);
        assert_eq!(w.bytes, committed);
        assert!(w.last_event.is_empty());
        assert_eq!(ledger(&w.bytes), settled);
        let bound = ok!(RewardsAccount::for_ledger(
            World::context(KEEPER, at).program,
            &settled,
        ));
        let replayed = rewards::RewardEffect::AlreadyApplied(ok!(Digest32::new(expected)));
        assert_eq!(
            ok!(value_adapter::plan(&bound, &settled, &settled, replayed)),
            ValueAction::None
        );
        assert_eq!(ok!(value_adapter::cover(&settled, 0, ValueAction::None)), 0);
        assert_eq!(get(&mut store, b"-result"), original_result);
        assert_eq!(get(&mut store, b"-event"), original_event);
        assert_eq!(get(&mut store, b"-plan"), original_plan);
    }
    assert_eq!(ok!(store.replay_state_bytes(RECOVERY_BYTES)), encoded);
    assert_eq!(ok!(fs::read(path)), encoded);
}

#[test]
pub(crate) fn reward_native_reopen() {
    if let Some(path) = std::env::var_os("PAXAI_REPAIR_261_STORE") {
        let path = PathBuf::from(path);
        match std::env::var("PAXAI_REPAIR_261_PHASE").as_deref() {
            Ok("commit") => write_recovery_store(&path),
            Ok("replay") => reopen_recovery_store(&path),
            _ => panic!("invalid internal recovery phase"),
        }
        return;
    }
    static NEXT: AtomicU64 = AtomicU64::new(0);
    let dir = std::env::temp_dir().join(format!(
        "paxai-repair-261-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    ));
    ok!(fs::create_dir(&dir));
    let path = dir.join("store");
    let mut test_name = module_path!().split("::").skip(1).collect::<Vec<_>>();
    test_name.push("reward_native_reopen");
    let test_name = test_name.join("::");
    for phase in ["commit", "replay"] {
        let output = ok!(std::process::Command::new(ok!(std::env::current_exe()))
            .args(["--exact", &test_name, "--nocapture"])
            .env("PAXAI_REPAIR_261_STORE", &path)
            .env("PAXAI_REPAIR_261_PHASE", phase)
            .output());
        assert!(
            output.status.success(),
            "{phase} process failed: {}\n{}",
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        assert!(
            String::from_utf8_lossy(&output.stdout).contains("1 passed"),
            "child test must execute"
        );
    }
    reopen_recovery_store(&path);
    ok!(fs::remove_dir_all(dir));
}

fn admit_boundary_task(w: &mut World, deadline: u64) -> Result<Vec<u8>, ApplicationError> {
    let worker = w.worker_record(WORKER);
    let binding = w.binding();
    let mut payload = binding.epoch.to_be_bytes().to_vec();
    payload.extend_from_slice(&binding.config.get().to_be_bytes());
    payload.extend_from_slice(ok!(w.policy().current.digest()).as_bytes());
    payload.extend_from_slice(binding.roster.as_bytes());
    payload.extend_from_slice(principal(0x21).as_bytes());
    payload.extend_from_slice(worker.worker.as_bytes());
    payload.extend_from_slice(worker.metadata.as_bytes());
    payload.extend_from_slice(&[1; 32]);
    payload.extend_from_slice(&[0xa1; 32]);
    payload.extend_from_slice(&deadline.to_be_bytes());
    w.route(0x21, dispatch::ADMIT_TASK, &payload, 324)
}
fn boundary_task(w: &World) -> layerx_programs_ai_market::TaskId {
    ok!(codec::derive_task(w.market(), 2, principal(0x21), [1; 32]))
}
fn task_step(w: &mut World, operation: Operation, at: u64) -> Result<Vec<u8>, ApplicationError> {
    let mut payload = ok!(state::decode_shared_state(&w.bytes))
        .revision
        .to_be_bytes()
        .to_vec();
    payload.extend_from_slice(boundary_task(w).as_bytes());
    payload.extend_from_slice(
        &[if operation == dispatch::ACCEPT_TASK {
            0xa5
        } else {
            0xe5
        }; 32],
    );
    let encoded = w.encoded_call(WORKER, operation, &payload, at);
    let mut envelope = ok!(codec::decode_envelope(&encoded)).envelope;
    let mut out = vec![0; MAX_ENVELOPE_BYTES];
    envelope.authentication = Authentication::Delegate {
        key: public(&key(WORKER)),
        signature: Signature64([0; 64]),
    };
    let length = ok!(codec::encode_envelope(&envelope, &mut out));
    let digest = ok!(ok!(codec::decode_envelope(&out[..length])).request_digest());
    envelope.authentication = Authentication::Delegate {
        key: public(&key(WORKER)),
        signature: Signature64(key(WORKER).sign(digest.as_bytes()).to_bytes()),
    };
    let length = ok!(codec::encode_envelope(&envelope, &mut out));
    w.route_encoded(WORKER, at, &out[..length])
}
fn task_record(w: &World) -> tasks::TaskBinding {
    let section = w.policy();
    let set = ok!(TaskSet::decode(section.task_region));
    ok!(set.position(boundary_task(w)))
        .1
        .unwrap_or_else(|| panic!("task missing"))
}

#[test]
fn task_execution_height_timeout_commits() {
    for height in [329, 331] {
        let mut w = execution_world();
        let before = w.bytes.clone();
        assert_eq!(
            admit_boundary_task(&mut w, 333),
            Err(layerx_programs_ai_market::errors::F01_TASK_EXPIRED)
        );
        assert_eq!(w.bytes, before);
        let rewards = ledger(&w.bytes);
        ok!(admit_boundary_task(&mut w, 332));
        ok!(task_step(&mut w, dispatch::ACCEPT_TASK, 325));
        ok!(task_step(&mut w, dispatch::COMMIT_TASK_RESULT, height));
        let record = task_record(&w);
        assert_eq!(
            (
                record.status,
                record.deadline,
                record.acknowledgement,
                record.result
            ),
            (
                tasks::TaskStatus::ResultCommitted,
                332,
                Some(digest(0xa5)),
                Some(digest(0xe5))
            )
        );
        assert_eq!(ledger(&w.bytes), rewards);
        let task = boundary_task(&w);
        assert_eq!(
            w.route(0x21, dispatch::CANCEL_TASK, task.as_bytes(), 334),
            Err(layerx_programs_ai_market::errors::F01_TASK_ALREADY_ACCEPTED)
        );
    }
}
#[test]
fn task_execution_height_timeout_refuses_and_cancels() {
    for cancel_height in [332, 334, 352] {
        let mut w = execution_world();
        ok!(admit_boundary_task(&mut w, 332));
        ok!(task_step(&mut w, dispatch::ACCEPT_TASK, 325));
        let before = w.bytes.clone();
        let rewards = ledger(&before);
        for height in [332, 334] {
            assert_eq!(
                task_step(&mut w, dispatch::COMMIT_TASK_RESULT, height),
                Err(layerx_programs_ai_market::errors::F01_TASK_EXPIRED)
            );
            assert_eq!(w.bytes, before);
        }
        let task = boundary_task(&w);
        assert_eq!(
            w.route(0x21, dispatch::CANCEL_TASK, task.as_bytes(), 331),
            Err(layerx_programs_ai_market::errors::F01_TASK_ALREADY_ACCEPTED)
        );
        assert_eq!(
            w.route(0x22, dispatch::CANCEL_TASK, task.as_bytes(), cancel_height),
            Err(layerx_programs_ai_market::errors::UNAUTHORIZED)
        );
        ok!(w.route(0x21, dispatch::CANCEL_TASK, task.as_bytes(), cancel_height));
        let record = task_record(&w);
        assert_eq!(
            (record.status, record.acknowledgement, record.result),
            (tasks::TaskStatus::Cancelled, None, None)
        );
        assert_eq!(ledger(&w.bytes), rewards);
        let shared = ok!(state::decode_shared_state(&w.bytes));
        let rewards = ok!(rewards::decode_reward_state(
            shared.feature_sections[Section::SettlementClaims.index()]
                .get(..rewards::REWARD_STATE_BYTES)
                .unwrap_or_else(|| panic!("reward head missing")),
        ));
        let row = ok!(rewards.row(2));
        assert_eq!(ok!(row.unclaimed_sum()), 0);
        assert_eq!(
            row.check_claim(
                &rewards.dictionary(),
                w.worker(WORKER),
                ok!(AccountId::new(principal(WORKER).bytes())),
                1,
                cancel_height
            ),
            Err(layerx_programs_ai_market::errors::F06_CLAIM_NOT_READY)
        );
        let after = w.bytes.clone();
        ok!(w.route(0x21, dispatch::CANCEL_TASK, task.as_bytes(), cancel_height));
        assert_eq!(
            ok!(codec::decode_result(&w.last_result)).status,
            codec::ResultStatus::AlreadyApplied
        );
        assert_eq!(w.bytes, after);
    }
}
#[test]
fn task_execution_height_unaccepted_deadline_guards() {
    let mut w = execution_world();
    ok!(admit_boundary_task(&mut w, 332));
    let task = boundary_task(&w);
    let before = w.bytes.clone();
    for height in [332, 352] {
        assert_eq!(
            w.route(0x21, dispatch::CANCEL_TASK, task.as_bytes(), height),
            Err(layerx_programs_ai_market::errors::F01_TASK_EXPIRED)
        );
        assert_eq!(w.bytes, before);
    }
    assert_eq!(
        w.route(0x21, dispatch::CANCEL_TASK, task.as_bytes(), 63),
        Err(layerx_programs_ai_market::errors::ARITHMETIC)
    );
    assert_eq!(w.bytes, before);
    ok!(w.route(0x21, dispatch::CANCEL_TASK, task.as_bytes(), 331));
    assert_eq!(task_record(&w).status, tasks::TaskStatus::Cancelled);
}
