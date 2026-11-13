mod support;

#[allow(
    dead_code,
    reason = "The host binary uses signalling items that this route target does not call (decisions 3218 and 3273)"
)]
#[path = "../src/inference_market/signal.rs"]
mod signal;

use std::collections::BTreeMap;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::TcpListener;
use std::path::PathBuf;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};
use std::thread;
use std::time::Duration;

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::finality::inference_enrol::{
    EnrolmentContext, EnrolmentService, SignedManifest,
};
use layerx_client::ai_market::market_program as market;
use layerx_client::ai_market::{
    sign_market_envelope, NativeTerms, OperationJournal, OperationRecord,
};
use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::disclosure::call::decode_program_call;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_types::program_call::Resources;
use market::admission::{ApprovalTerms, EvaluatorConsent, Participant};
use market::codec::{self, Envelope, Writer};
use market::dispatch::{self, Operation};
use market::policy::{PolicyCommitments, RoomProfile, TaskPolicyV1, TASK_POLICY_BYTES};
use market::registry::{derive_rewards_account, MarketHeader};
use market::registry_ops::{CallContext, PolicySection};
use market::state::{self, ActorSlot, Control, ReplayTable, Section, SharedState};
use market::types::{
    AccountId, AssetId, Authentication, ChainDomain, Digest32, MarketId, MetadataDigest, Presence,
    PrincipalId, ProgramId, PublicKey32, RequestId, RosterDigest, RubricDigest, Signature64,
    Version, WorkerId,
};
use market::workers::{self, CapabilityFields, ManifestV2, WorkerState, WorkerTable};
use serde_json::{json, Value};

fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("{error:?}"))
}

fn ready<F: std::future::Future>(future: F) -> F::Output {
    struct ThreadWake(thread::Thread);
    impl Wake for ThreadWake {
        fn wake(self: Arc<Self>) {
            self.0.unpark();
        }
        fn wake_by_ref(self: &Arc<Self>) {
            self.0.unpark();
        }
    }
    let waker = Waker::from(Arc::new(ThreadWake(thread::current())));
    let mut context = Context::from_waker(&waker);
    let mut future = std::pin::pin!(future);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => return value,
            Poll::Pending => thread::park(),
        }
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = support::directory("inference-enrol");
        must(std::fs::create_dir_all(&path));
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

struct Fixture {
    header: MarketHeader,
    owner: LocalSigner,
    peer: LocalSigner,
    delegate: SigningKey,
    peer_principal: PrincipalId,
    legacy: Vec<u8>,
    manifest: Vec<u8>,
    state: Vec<u8>,
}

#[derive(Clone, Copy)]
struct SigningSequences {
    replay: u64,
    account: u64,
}

fn capabilities(runtime_kind: u8) -> CapabilityFields {
    CapabilityFields {
        accelerator_memory_mib: 24_576,
        system_memory_mib: 65_536,
        bandwidth_class: 3,
        region_code: *b"GB",
        attestation_family: 255,
        runtime_kind,
    }
}

fn legacy_manifest(market: MarketId, worker: WorkerId, owner: PrincipalId) -> Vec<u8> {
    let mut bytes = vec![0; workers::MAX_MANIFEST_BYTES];
    let mut w = Writer::new(&mut bytes);
    must(w.u16(1));
    must(w.put(market.as_bytes()));
    must(w.put(worker.as_bytes()));
    must(w.put(owner.as_bytes()));
    for value in [1, 1, 2, 150, 350] {
        must(w.u64(value));
    }
    must(w.put(&[13; 32]));
    must(w.u16(1));
    must(w.u8(1));
    must(w.u8(1));
    for value in 20..25 {
        must(w.put(&[value; 32]));
    }
    for value in [1024, 2048, 4096, 8192] {
        must(w.u32(value));
    }
    must(w.u8(1));
    must(w.u32(60_000));
    must(w.u16(4));
    must(w.u8(1));
    must(w.u16(1));
    must(w.u8(1));
    must(w.bytes(b"https://worker.example/paxai/v1", workers::MAX_URI_BYTES));
    must(w.put(&[30; 32]));
    must(w.u8(1));
    must(w.u16(1));
    must(w.u16(1));
    must(w.put(&[31; 32]));
    must(w.put(&[32; 32]));
    must(w.u32(0));
    let length = w.len();
    bytes.truncate(length);
    bytes
}

fn principal(address: [u8; 20]) -> PrincipalId {
    let mut bytes = [0; 32];
    bytes[12..].copy_from_slice(&address);
    must(PrincipalId::new(bytes))
}

impl Fixture {
    fn program() -> Self {
        let mut fixture = Self::new(1);
        let digest = |byte| must(Digest32::new([byte; 32]));
        let policy = must(TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: digest(20),
                dataset_artifact: [21; 32],
                benchmark_suite: digest(22),
                rubric: must(RubricDigest::new([23; 32])),
                task_schema: digest(24),
                result_schema: digest(25),
                service_terms: digest(26),
            },
            1000,
            1,
        ));
        let mut policy_bytes = vec![0; TASK_POLICY_BYTES];
        let length = must(policy.encode(&mut policy_bytes));
        policy_bytes.truncate(length);
        let mut payload = Vec::new();
        payload.extend_from_slice(fixture.header.owner_principal.as_bytes());
        payload.extend_from_slice(fixture.header.funding_asset.as_bytes());
        payload.extend_from_slice(fixture.header.rewards_account.as_bytes());
        payload.extend_from_slice(fixture.header.refund_recipient_account.as_bytes());
        payload.push(0);
        payload.extend_from_slice(&policy_bytes);
        payload.extend_from_slice(fixture.header.metadata_digest.as_bytes());
        let create = fixture.sign(
            dispatch::CREATE,
            &payload,
            fixture.header.owner_principal,
            SigningSequences {
                replay: 1,
                account: 0,
            },
            &fixture.owner,
            false,
        );
        fixture.state = must(apply_program(
            &fixture.header,
            None,
            create.signed_bytes(),
            128,
        ));
        fixture.header = must(PolicySection::decode(must(
            must(state::decode_shared_state(&fixture.state)).section(Section::PolicyLifecycle),
        )))
        .header;
        fixture.bootstrap_roster();
        let mut payload = vec![0; 256];
        let length = must(
            market::rooms::OpenRoomRequest {
                room_id: digest(60),
                model_digest: digest(20),
                runtime_digest: digest(13),
                profile: RoomProfile {
                    minimum_bond: 100,
                    canary_ratio_bps: 250,
                    lap_size_tokens: 32,
                    challenge_window_blocks: 16,
                    slash_fraction_bps: 1000,
                    host_share_bps: 1000,
                    protocol_share_bps: 1000,
                    settlement_account: must(AccountId::new([80; 32])),
                    max_generated_tokens: 4096,
                    queue_depth: market::policy::DEFAULT_QUEUE_DEPTH,
                    liveness_timeout_ms: 30_000,
                    acknowledgement_window_ms: 5_000,
                    canary_relative_tolerance_bps: 100,
                },
            }
            .encode(&mut payload),
        );
        payload.truncate(length);
        let open = fixture.sign(
            dispatch::OpenRoom,
            &payload,
            fixture.header.owner_principal,
            SigningSequences {
                replay: fixture.next_sequence(fixture.header.owner_principal),
                account: 20,
            },
            &fixture.owner,
            false,
        );
        fixture.state = must(apply_program(
            &fixture.header,
            Some(&fixture.state),
            open.signed_bytes(),
            153,
        ));
        fixture
    }

    fn next_sequence(&self, actor: PrincipalId) -> u64 {
        let state = must(state::decode_shared_state(&self.state));
        (0..43)
            .filter_map(|index| {
                state
                    .control
                    .replay
                    .actor(must(ActorSlot::from_index(index)))
            })
            .find(|entry| entry.principal == actor)
            .map_or(1, |entry| entry.last.map_or(1, |last| last.sequence + 1))
    }

    fn apply_signed(&mut self, record: &OperationRecord, height: u64) {
        self.state = apply_program(
            &self.header,
            Some(&self.state),
            record.signed_bytes(),
            height,
        )
        .unwrap_or_else(|error| {
            panic!(
                "{} at {height}: {error:?}",
                must(codec::decode_envelope(&must(
                    layerx_client::ai_market::market_envelope(record)
                )))
                .envelope
                .operation
                .metadata()
                .name
            )
        });
        self.header = must(PolicySection::decode(must(
            must(state::decode_shared_state(&self.state)).section(Section::PolicyLifecycle),
        )))
        .header;
    }

    fn call(&mut self, operation: Operation, payload: &[u8], signer: &LocalSigner, height: u64) {
        let actor = principal(
            signer
                .evm_address()
                .unwrap_or_else(|| panic!("native actor")),
        );
        let sequence = if operation.metadata().sequence == dispatch::SequencePolicy::ObjectLocal {
            0
        } else {
            self.next_sequence(actor)
        };
        let record = self.sign(
            operation,
            payload,
            actor,
            SigningSequences {
                replay: sequence,
                account: sequence,
            },
            signer,
            false,
        );
        self.apply_signed(&record, height);
    }

    fn approve(
        &mut self,
        participant: Participant,
        owner: PrincipalId,
        key: PublicKey32,
        nonce: u8,
        identity: Digest32,
    ) -> Digest32 {
        let mut terms = ApprovalTerms {
            participant,
            owner,
            enrollment_nonce_commitment: must(codec::domain_hash(
                "PAXAI/enrollment-nonce/v1",
                &[nonce; 32],
            )),
            delegate: key,
            delegate_generation: 1,
            identity_commitment: identity,
            effective_epoch: 0,
            config_version: 1,
            request: must(RequestId::new([1; 32])),
            expiry_height: 192,
        };
        let mut payload = vec![participant.role() as u8];
        for bytes in [
            participant.bytes().as_slice(),
            owner.as_bytes(),
            terms.enrollment_nonce_commitment.as_bytes(),
            &key.0,
        ] {
            payload.extend_from_slice(bytes);
        }
        payload.extend_from_slice(&1_u64.to_be_bytes());
        payload.extend_from_slice(identity.as_bytes());
        for value in [0_u64, 1, terms.expiry_height] {
            payload.extend_from_slice(&value.to_be_bytes());
        }
        let owner_signer = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
        let sequence = self.next_sequence(self.header.owner_principal);
        let record = self.sign(
            dispatch::ApproveAdmission,
            &payload,
            self.header.owner_principal,
            SigningSequences {
                replay: sequence,
                account: sequence,
            },
            &owner_signer,
            false,
        );
        let envelope = must(layerx_client::ai_market::market_envelope(&record));
        terms.request = must(codec::decode_envelope(&envelope)).envelope.request;
        let digest = must(terms.digest(&self.header));
        self.apply_signed(&record, 151);
        digest
    }

    fn bootstrap_roster(&mut self) {
        let mut bootstrap = Self::new(1);
        bootstrap.state.clone_from(&self.state);
        bootstrap.header = self.header;
        bootstrap.peer = LocalSigner::from_derived_account(&must(derive_from_seed(&[13; 32], 0)));
        bootstrap.peer_principal = principal(
            bootstrap
                .peer
                .evm_address()
                .unwrap_or_else(|| panic!("bootstrap peer")),
        );
        bootstrap.delegate = SigningKey::from_bytes(&[57; 32]);
        let worker = must(codec::derive_worker(
            bootstrap.header.market_id,
            bootstrap.peer_principal,
            [21; 32],
        ));
        bootstrap.legacy =
            legacy_manifest(bootstrap.header.market_id, worker, bootstrap.peer_principal);
        let mut bytes = vec![0; workers::MAX_MANIFEST_BYTES];
        let length = must(codec::encode_manifest_v2(
            &ManifestV2 {
                manifest_v1: &bootstrap.legacy,
                capabilities: capabilities(1),
            },
            &mut bytes,
        ));
        bytes.truncate(length);
        bootstrap.manifest = bytes;
        let plan = bootstrap.plan();
        for (record, height) in [
            (&plan.proposal, 140),
            (&plan.acceptance, 141),
            (&plan.publication, 150),
        ] {
            bootstrap.apply_signed(record, height);
        }
        let state = must(state::decode_shared_state(&bootstrap.state));
        let workers = must(WorkerTable::decode(
            must(market::evaluators::authority::split_identity_section(must(
                state.section(Section::IdentityRoster),
            )))
            .0,
        ));
        let identity = must(
            workers
                .get(worker)
                .unwrap_or_else(|| panic!("bootstrap worker"))
                .proposal_digest(&bootstrap.header),
        );
        let approval = bootstrap.approve(
            Participant::Worker(worker),
            bootstrap.peer_principal,
            PublicKey32(bootstrap.delegate.verifying_key().to_bytes()),
            21,
            identity,
        );
        let mut payload = worker.bytes().to_vec();
        for value in [1_u64, 0, 1] {
            payload.extend_from_slice(&value.to_be_bytes());
        }
        payload.extend_from_slice(approval.as_bytes());
        let peer = LocalSigner::from_derived_account(&must(derive_from_seed(&[13; 32], 0)));
        bootstrap.call(dispatch::AdmitWorker, &payload, &peer, 151);
        bootstrap.bootstrap_evaluators();
        self.state = bootstrap.state;
        self.header = bootstrap.header;
    }

    fn bootstrap_evaluators(&mut self) {
        let bootstrap = self;
        let owner = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
        for nonce in [2_u8, 3, 4] {
            let signer =
                LocalSigner::from_derived_account(&must(derive_from_seed(&[nonce; 32], 0)));
            let actor = principal(signer.evm_address().unwrap_or_else(|| panic!("evaluator")));
            let key = SigningKey::from_bytes(&[nonce; 32]);
            let public = PublicKey32(key.verifying_key().to_bytes());
            let rubric = must(PolicySection::decode(must(
                must(state::decode_shared_state(&bootstrap.state))
                    .section(Section::PolicyLifecycle),
            )))
            .current
            .commitments
            .rubric;
            let grant = must(market::evaluators::model::EvaluatorGrant::nominate(
                bootstrap.header.market_id,
                actor,
                [nonce; 32],
                market::evaluators::model::GrantTerms {
                    rubric,
                    grant_version: must(Version::new(1)),
                    key_version: must(Version::new(1)),
                    signing_key: public,
                    effective_epoch: 0,
                    expiry_epoch_exclusive: 32,
                },
            ));
            let mut payload = actor.bytes().to_vec();
            payload.extend_from_slice(&[nonce; 32]);
            payload.extend_from_slice(rubric.as_bytes());
            payload.extend_from_slice(&public.0);
            for value in [1_u64, 1, 0, 32] {
                payload.extend_from_slice(&value.to_be_bytes());
            }
            bootstrap.call(dispatch::ScheduleEvaluator, &payload, &owner, 151);
            let mut bytes = [0; 256];
            let length = must(market::evaluators::codec::encode_grant(&grant, &mut bytes));
            let identity = must(codec::domain_hash(
                "PAXAI/evaluator-grant/v1",
                &bytes[..length],
            ));
            let approval = bootstrap.approve(
                Participant::Evaluator(grant.evaluator),
                actor,
                public,
                nonce,
                identity,
            );
            let mut request = [1; 32];
            request[..2].copy_from_slice(&dispatch::AdmitEvaluator.selector().to_be_bytes());
            request[2..10].copy_from_slice(&bootstrap.next_sequence(actor).to_be_bytes());
            let consent = EvaluatorConsent {
                chain: bootstrap.header.deployment_chain_domain,
                program: bootstrap.header.program_id,
                market: bootstrap.header.market_id,
                evaluator: grant.evaluator,
                owner: actor,
                signing_key: public,
                enrollment_nonce: [nonce; 32],
                rubric,
                approval_digest: approval,
                request: must(RequestId::new(request)),
                grant_version: 1,
                key_version: 1,
                effective_epoch: 0,
                config_version: 1,
                expiry_height: 192,
            };
            let mut body = [0; 362];
            must(consent.encode(&mut body));
            let mut payload = body.to_vec();
            payload.extend_from_slice(&key.sign(must(consent.digest()).as_bytes()).to_bytes());
            bootstrap.call(dispatch::AdmitEvaluator, &payload, &signer, 151);
        }
        let mut funding = 1000_u128.to_be_bytes().to_vec();
        funding.extend_from_slice(bootstrap.header.refund_recipient_account.as_bytes());
        funding.extend_from_slice(&1_u64.to_be_bytes());
        funding.push(1);
        bootstrap.call(dispatch::FUND, &funding, &owner, 151);
        bootstrap.call(dispatch::OPEN_EPOCH, &[], &owner, 152);
    }

    fn new(runtime: u8) -> Self {
        let owner = LocalSigner::from_derived_account(&must(derive_from_seed(&[11; 32], 0)));
        let peer = LocalSigner::from_derived_account(&must(derive_from_seed(&[12; 32], 0)));
        let owner_principal = principal(
            owner
                .evm_address()
                .unwrap_or_else(|| panic!("owner address")),
        );
        let peer_principal =
            principal(peer.evm_address().unwrap_or_else(|| panic!("peer address")));
        let chain = must(ChainDomain::new(market::merged_chain_domain(125)));
        let program = must(ProgramId::new([51; 32]));
        let market = must(codec::derive_market(chain, program));
        let asset = must(AssetId::new([52; 32]));
        let header = MarketHeader {
            format_version: 1,
            market_id: market,
            deployment_chain_domain: chain,
            program_id: program,
            owner_principal,
            funding_asset: asset,
            rewards_account: must(derive_rewards_account(program, asset)),
            refund_recipient_account: must(AccountId::new([54; 32])),
            treasury_principal: Presence::Absent,
            origin_height: 0,
            lifecycle: 2,
            state_revision: 1,
            highest_config_version: 1,
            active_config_version: 1,
            activation_epoch: 0,
            activation_scheduled: false,
            closure_requested_at: 0,
            close_phase: 0,
            close_cursor: 0,
            suspension_reason_digest: [0; 32],
            metadata_digest: must(MetadataDigest::new([55; 32])),
            closing_request_digest: [0; 32],
            reserved: [0; 8],
        };
        let worker = must(codec::derive_worker(market, peer_principal, [21; 32]));
        let legacy = legacy_manifest(market, worker, peer_principal);
        let mut manifest = vec![0; workers::MAX_MANIFEST_BYTES];
        let length = must(codec::encode_manifest_v2(
            &ManifestV2 {
                manifest_v1: &legacy,
                capabilities: capabilities(runtime),
            },
            &mut manifest,
        ));
        manifest.truncate(length);
        let mut replay = ReplayTable::new();
        must(replay.bind(ActorSlot::OWNER, owner_principal, must(Version::new(1))));
        let shared = SharedState {
            revision: 1,
            feature_sections: [&[], &[], &[], &[], &[]],
            control: Control {
                replay,
                feature_bytes: &[],
            },
        };
        let mut state = vec![0; market::MAX_STATE_BYTES];
        let mut scratch = vec![0; workers::CONTROL_SCRATCH_BYTES];
        let len = must(state::encode_shared_state(
            &shared,
            &mut state,
            &mut scratch,
        ));
        state.truncate(len);
        Self {
            header,
            owner,
            peer,
            delegate: SigningKey::from_bytes(&[56; 32]),
            peer_principal,
            legacy,
            manifest,
            state,
        }
    }

    fn sign(
        &self,
        operation: Operation,
        payload: &[u8],
        actor: PrincipalId,
        sequences: SigningSequences,
        signer: &LocalSigner,
        delegate: bool,
    ) -> OperationRecord {
        let mut request = [1; 32];
        request[..2].copy_from_slice(&operation.selector().to_be_bytes());
        request[2..10].copy_from_slice(&sequences.replay.to_be_bytes());
        let state = must(state::decode_shared_state(&self.state));
        let (epoch, roster) = if matches!(operation, dispatch::LockBond | dispatch::PostBid) {
            let admission = must(market::admission::AdmissionTable::decode(
                must(market::reputation_transition::split_joint(must(
                    state.section(Section::ReputationAdmission),
                )))
                .1,
            ));
            let epoch = admission
                .current_epoch()
                .unwrap_or_else(|| panic!("open admission epoch"));
            let roster = must(market::roster::roster_digest(&admission, epoch));
            (
                epoch,
                Presence::Present(must(RosterDigest::new(roster.bytes()))),
            )
        } else if must(state.section(Section::PolicyLifecycle)).is_empty() {
            (0, Presence::Absent)
        } else {
            let roster = must(market::epoch::captured_roster(&state))
                .map_or(Presence::Absent, |roster| {
                    Presence::Present(roster.binding.roster)
                });
            (0, roster)
        };
        let mut env = Envelope {
            operation,
            chain: self.header.deployment_chain_domain,
            program: self.header.program_id,
            market: self.header.market_id,
            actor,
            epoch,
            config: 1,
            roster,
            sequence: sequences.replay,
            expiry: 1000,
            request: must(RequestId::new(request)),
            payload,
            authentication: Authentication::Native,
        };
        let mut bytes = vec![0; market::MAX_ENVELOPE_BYTES];
        if delegate {
            let key = PublicKey32(self.delegate.verifying_key().to_bytes());
            env.authentication = Authentication::Delegate {
                key,
                signature: Signature64([0; 64]),
            };
            let len = must(codec::encode_envelope(&env, &mut bytes));
            let digest = must(must(codec::decode_envelope(&bytes[..len])).request_digest());
            env.authentication = Authentication::Delegate {
                key,
                signature: Signature64(self.delegate.sign(digest.as_bytes()).to_bytes()),
            };
        }
        let len = must(codec::encode_envelope(&env, &mut bytes));
        must(ready(sign_market_envelope(
            &bytes[..len],
            &NativeTerms {
                network_id: 125,
                actor_did: b"did:layerx:inference-test",
                owner_public_key: signer.public_key(),
                account_sequence: sequences.account,
                idempotency_key: request,
                not_before: 0,
                not_after: 1000,
                fee_limit: 1_000_000,
                capabilities: &[],
                access_declaration: &[],
                response_capacity: 16_384,
                resources: Resources([10_000_000; 7]),
                gas_limit: 100_000,
                max_fee_per_gas: 10,
                max_priority_fee_per_gas: 1,
            },
            signer,
        )))
    }

    fn plan(&self) -> SignedManifest {
        let state = must(state::decode_shared_state(&self.state));
        let owner_sequence = state
            .control
            .replay
            .actor(ActorSlot::OWNER)
            .unwrap_or_else(|| panic!("owner replay authority"))
            .last
            .map_or(1, |last| last.sequence + 1);
        let summary = must(workers::decode_manifest(&self.manifest));
        let key = PublicKey32(self.delegate.verifying_key().to_bytes());
        let mut proposal = Vec::new();
        proposal.extend_from_slice(self.peer_principal.as_bytes());
        proposal.extend_from_slice(&[21; 32]);
        proposal.extend_from_slice(&key.0);
        proposal.extend_from_slice(summary.digest.as_bytes());
        proposal.extend_from_slice(&200_u64.to_be_bytes());
        let digest = must(workers::consent_digest(
            &self.header,
            summary.worker,
            summary.owner,
            key,
            1,
            1,
            summary.digest,
            200,
        ));
        let mut acceptance = Vec::new();
        acceptance.extend_from_slice(summary.worker.as_bytes());
        acceptance.extend_from_slice(&1_u64.to_be_bytes());
        acceptance.extend_from_slice(&1_u64.to_be_bytes());
        acceptance.extend_from_slice(summary.digest.as_bytes());
        acceptance.extend_from_slice(&self.delegate.sign(digest.as_bytes()).to_bytes());
        let mut publication = vec![0; workers::MAX_MANIFEST_BYTES + 100];
        let mut w = Writer::new(&mut publication);
        must(w.put(summary.worker.as_bytes()));
        must(w.u64(1));
        must(w.u64(2));
        must(w.put(summary.digest.as_bytes()));
        must(w.u64(summary.valid_from));
        must(w.u64(summary.expiry));
        must(w.bytes(&self.manifest, workers::MAX_MANIFEST_BYTES));
        let len = w.len();
        publication.truncate(len);
        SignedManifest {
            manifest: self.manifest.clone(),
            proposal: self.sign(
                dispatch::EnrollWorker,
                &proposal,
                self.header.owner_principal,
                SigningSequences {
                    replay: owner_sequence,
                    account: owner_sequence - 1,
                },
                &self.owner,
                false,
            ),
            acceptance: self.sign(
                dispatch::AcceptEnrollment,
                &acceptance,
                self.peer_principal,
                SigningSequences {
                    replay: 1,
                    account: 0,
                },
                &self.peer,
                false,
            ),
            publication: self.sign(
                dispatch::PublishMetadata,
                &publication,
                self.peer_principal,
                SigningSequences {
                    replay: 2,
                    account: 1,
                },
                &self.peer,
                true,
            ),
        }
    }
}

fn apply_program(
    header: &MarketHeader,
    current: Option<&[u8]>,
    raw: &[u8],
    height: u64,
) -> Result<Vec<u8>, market::errors::ApplicationError> {
    let (call, signature) = must(decode_signed(raw));
    let (native, _) = must(decode_program_call(&call.data));
    let mut next = vec![0; market::MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; market::MAX_EVENT_BYTES];
    let mut result = vec![0; market::MAX_RESULT_BYTES];
    match dispatch::route(
        &CallContext {
            chain: header.deployment_chain_domain,
            program: header.program_id,
            principal: principal(must(recover_sender(&call, &signature))),
            height,
        },
        &native.calldata,
        current,
        dispatch::Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )? {
        dispatch::Routed::Applied { state_len, .. } => {
            next.truncate(state_len);
            Ok(next)
        }
        dispatch::Routed::Refused { result_len } => {
            Err(codec::decode_result(&result[..result_len])?
                .error
                .unwrap_or_else(|| panic!("program refusal must name an error")))
        }
        dispatch::Routed::Unchanged { .. } => panic!("unexpected replay"),
    }
}

struct NodeState {
    bytes: Vec<u8>,
    operations: Vec<(String, [u8; 20])>,
    submitted: Vec<String>,
    manifests: Vec<Vec<u8>>,
    receipts: BTreeMap<String, Value>,
}

fn request(stream: std::net::TcpStream) -> (Value, std::net::TcpStream) {
    must(stream.set_read_timeout(Some(Duration::from_secs(5))));
    let mut reader = BufReader::new(stream);
    let mut length = None;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = Some(must(value.trim().parse::<usize>()));
        }
    }
    let length = length.unwrap_or_else(|| panic!("content length"));
    assert!(length <= 262_144);
    let mut bytes = vec![0; length];
    must(reader.read_exact(&mut bytes));
    (must(serde_json::from_slice(&bytes)), reader.into_inner())
}

fn node(
    fixture: &Fixture,
    requests: usize,
) -> (
    layerx_client::sequencer::SequencerEndpoint,
    thread::JoinHandle<NodeState>,
    layerx_client::sequencer::ReceiptKeyEntry,
) {
    resume_node(fixture, requests, BTreeMap::new())
}

fn resume_node(
    fixture: &Fixture,
    requests: usize,
    receipts: BTreeMap<String, Value>,
) -> (
    layerx_client::sequencer::SequencerEndpoint,
    thread::JoinHandle<NodeState>,
    layerx_client::sequencer::ReceiptKeyEntry,
) {
    let listener = must(TcpListener::bind("127.0.0.1:0"));
    let endpoint = must(layerx_client::sequencer::SequencerEndpoint::new(
        &format!("http://{}/", must(listener.local_addr())),
        &[],
    ));
    let header = fixture.header;
    let mut state = NodeState {
        bytes: fixture.state.clone(),
        operations: Vec::new(),
        submitted: Vec::new(),
        manifests: Vec::new(),
        receipts,
    };
    let (_, key) = support::sequencer_receipt(fixture.plan().proposal.signed_bytes(), true);
    let server = thread::spawn(move || {
        for _ in 0..requests {
            let (rpc, mut stream) = request(must(listener.accept()).0);
            let result = match rpc["method"].as_str().unwrap_or("") {
                "eth_sendRawTransaction" => apply_transaction(&header, &rpc, &mut state),
                "pax_sequencerReceipt" => Ok(state
                    .receipts
                    .get(rpc["params"][0].as_str().unwrap_or(""))
                    .cloned()
                    .unwrap_or(Value::Null)),
                "eth_getTransactionReceipt" => Ok(Value::Null),
                method => panic!("unexpected RPC {method}"),
            };
            let reply = match result {
                Ok(value) => json!({"jsonrpc":"2.0", "id":rpc["id"], "result":value}),
                Err(error) => json!({"jsonrpc":"2.0", "id":rpc["id"], "error":error}),
            }
            .to_string();
            must(write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{reply}", reply.len()));
            must(stream.flush());
        }
        state
    });
    (endpoint, server, key)
}

fn apply_transaction(
    header: &MarketHeader,
    rpc: &Value,
    state: &mut NodeState,
) -> Result<Value, Value> {
    let raw = must(hex::decode(
        rpc["params"][0]
            .as_str()
            .unwrap_or("")
            .trim_start_matches("0x"),
    ));
    state.submitted.push(hex::encode(&raw));
    let (call, signature) = must(decode_signed(&raw));
    let from = must(recover_sender(&call, &signature));
    let (native, _) = must(decode_program_call(&call.data));
    assert_eq!(native.program_id.bytes(), header.program_id.bytes());
    let env = must(codec::decode_envelope(&native.calldata)).envelope;
    state
        .operations
        .push((env.operation.metadata().name.to_owned(), from));
    let height = match env.operation {
        dispatch::EnrollWorker => 154,
        dispatch::AcceptEnrollment => 155,
        dispatch::PublishMetadata => 154 + workers::METADATA_COOLDOWN,
        dispatch::LockBond => 155 + workers::METADATA_COOLDOWN,
        dispatch::PostBid => 156 + workers::METADATA_COOLDOWN,
        operation => panic!("unexpected operation {operation:?}"),
    };
    match apply_program(header, Some(&state.bytes), &raw, height) {
        Ok(next) => {
            state.bytes = next;
            if env.operation == dispatch::PublishMetadata {
                let mut r = codec::Reader::new(env.payload);
                must(r.take(96));
                state
                    .manifests
                    .push(must(r.bytes(workers::MAX_MANIFEST_BYTES)).to_vec());
                must(r.finish());
            }
            let hash = transaction_hash(&raw);
            state.receipts.insert(
                support::rpc_hex(&hash),
                support::sequencer_receipt(&raw, true).0,
            );
            Ok(json!(support::rpc_hex(&hash)))
        }
        Err(error) => {
            let result = must(codec::ApplicationResult::failure(
                error,
                Presence::Absent,
                0,
            ));
            let mut bytes = vec![0; market::MAX_RESULT_BYTES];
            let len = must(codec::encode_result(&result, &mut bytes));
            Err(
                json!({"code": 3, "message": "program refused", "data": support::rpc_hex(&bytes[..len])}),
            )
        }
    }
}

fn vectors() -> Value {
    must(serde_json::from_str(include_str!(
        "../../../../../apps/inference-peer/test/fixtures/host-enrol-route.json"
    )))
}

struct RouteChain(std::sync::atomic::AtomicBool);
impl signal::SignalChain for RouteChain {
    fn reference(&self) -> Result<layerx_agentd::degraded::Reference, signal::SignalError> {
        if !self.0.load(std::sync::atomic::Ordering::Acquire) {
            return Err(signal::SignalError::ChainUnavailable);
        }
        Ok(layerx_agentd::degraded::Reference {
            head_sequence: vectors()["context"]["height"].as_u64().unwrap_or(0),
            checkpoint: [1; 32],
        })
    }

    fn read(&self, _: &str, _: Value) -> Result<Value, signal::SignalError> {
        Err(signal::SignalError::ChainUnavailable)
    }
}

fn offer() -> signal::RoomOffer {
    signal::RoomOffer {
        room_id: [60; 32],
        model_digest: [20; 32],
        runtime_digest: [13; 32],
        layer_plan: vec![signal::LayerAssignment {
            peer_key: [56; 32],
            first_layer: 0,
            end_layer: 16,
        }],
        price: "17".to_owned(),
    }
}

fn route(authority: signal::SignalAuthority) -> signal::SignalListener {
    must(signal::SignalListener::start(
        must(TcpListener::bind("127.0.0.1:0")),
        offer(),
        authority,
    ))
}

fn response(stream: std::net::TcpStream) -> (u16, Value) {
    let mut reader = BufReader::new(stream);
    let mut status = String::new();
    must(reader.read_line(&mut status));
    let status = must(status.split_whitespace().nth(1).unwrap_or("").parse());
    let mut length = 0;
    loop {
        let mut line = String::new();
        assert_ne!(must(reader.read_line(&mut line)), 0);
        if line == "\r\n" {
            break;
        }
        if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
            length = must(value.trim().parse::<usize>());
        }
    }
    assert!(length <= 4096);
    let mut bytes = vec![0; length];
    must(reader.read_exact(&mut bytes));
    (status, must(serde_json::from_slice(&bytes)))
}

fn post(listener: &signal::SignalListener, vector: &Value, case: &Value) -> (u16, Value) {
    let mut document = vector["request"].clone();
    if let Some(replacements) = case["replace"].as_object() {
        for (key, value) in replacements {
            document[key] = value.clone();
        }
    }
    if let Some(name) = case["remove"].as_str() {
        document
            .as_object_mut()
            .unwrap_or_else(|| panic!("request object"))
            .remove(name);
    }
    let body = case["raw"]
        .as_str()
        .map_or_else(|| document.to_string(), str::to_owned);
    let mut stream = must(std::net::TcpStream::connect(listener.local_addr()));
    must(stream.set_read_timeout(Some(Duration::from_secs(10))));
    let length = if case["omit_length"] == true {
        String::new()
    } else {
        let length = case.get("length").map_or_else(
            || body.len().to_string(),
            |value| {
                value
                    .as_str()
                    .map_or_else(|| value.to_string(), str::to_owned)
            },
        );
        format!("Content-Length: {length}\r\n")
    };
    must(write!(stream, "POST {} HTTP/1.1\r\nHost: localhost\r\n{length}Content-Type: {}\r\nConnection: close\r\n\r\n",
        vector["route"].as_str().unwrap_or(""), case["content_type"].as_str().unwrap_or("application/json")));
    // Oversized requests prove the length refusal arrives without sending a body.
    if case["length"].is_null() && case["omit_length"] != true {
        must(stream.write_all(body.as_bytes()));
    }
    response(stream)
}

#[test]
fn enrol_route_relays_every_signed_record_and_refuses_invalid_documents_before_submission() {
    let vector = vectors();
    let fixture = Fixture::program();
    assert_native_producer_document(&fixture, &vector);
    let (endpoint, server, key) = node(&fixture, 15);
    let root = Directory::new();
    let context = &vector["context"];
    let fixed = |name: &str| -> [u8; 32] {
        must(must(hex::decode(context[name].as_str().unwrap_or(""))).try_into())
    };
    let service = EnrolmentService {
        endpoint,
        journal: must(OperationJournal::open(&root.0)),
        receipt_keys: vec![key],
        timeout: Duration::from_secs(2),
        context: EnrolmentContext {
            chain_id: context["chain_id"].as_u64().unwrap_or(0),
            program: fixed("program"),
            model_digest: fixed("model_digest"),
            runtime_digest: fixed("runtime_digest"),
        },
    };
    let chain = Arc::new(RouteChain(std::sync::atomic::AtomicBool::new(true)));
    let authority = signal::SignalAuthority::with_chain(chain.clone(), 125, fixed("program"));
    let unconfigured = route(authority.clone());
    let configured = route(authority.with_enrolment(service));
    for case in vector["refusals"]
        .as_array()
        .unwrap_or_else(|| panic!("refusals"))
    {
        chain.0.store(
            case["chain_ready"] != false,
            std::sync::atomic::Ordering::Release,
        );
        let listener = if case["configured"] == false {
            &unconfigured
        } else {
            &configured
        };
        let (status, body) = post(listener, &vector, case);
        assert_eq!(
            u64::from(status),
            case["status"].as_u64().unwrap_or(0),
            "{}",
            case["name"]
        );
        assert_eq!(body, case["body"], "{}", case["name"]);
    }
    chain.0.store(true, std::sync::atomic::Ordering::Release);
    let (status, body) = post(&configured, &vector, &Value::Null);
    assert_eq!(
        u64::from(status),
        vector["accepted"]["status"].as_u64().unwrap_or(0),
        "host outcome: {body}"
    );
    assert_eq!(body, vector["accepted"]["body"]);
    let result = must(server.join());
    assert_chain_result(&vector, &result);
}

fn assert_native_producer_document(fixture: &Fixture, vector: &Value) {
    let expiry = vector["request"]["expiry"].as_u64().unwrap_or(0);
    for name in vector["records"]
        .as_array()
        .unwrap_or_else(|| panic!("records"))
    {
        let raw = must(hex::decode(
            vector["request"][name.as_str().unwrap_or("")]
                .as_str()
                .unwrap_or(""),
        ));
        let (call, signature) = must(decode_signed(&raw));
        let sender = must(recover_sender(&call, &signature));
        let signer = if Some(sender) == fixture.owner.evm_address() {
            &fixture.owner
        } else {
            &fixture.peer
        };
        let record = must(OperationRecord::from_signed_transaction(
            raw.clone(),
            [0; 32],
            expiry,
        ));
        let bytes = must(layerx_client::ai_market::market_envelope(&record));
        let env = must(codec::decode_envelope(&bytes)).envelope;
        if env.operation == dispatch::EnrollWorker {
            assert_eq!(
                env.sequence,
                fixture.next_sequence(fixture.header.owner_principal)
            );
        }
        let reproduced = must(ready(sign_market_envelope(
            &bytes,
            &NativeTerms {
                network_id: 125,
                actor_did: b"did:layerx:inference-test",
                owner_public_key: signer.public_key(),
                account_sequence: call.nonce,
                idempotency_key: env.request.bytes(),
                not_before: 0,
                not_after: expiry,
                fee_limit: 1_000_000,
                capabilities: &[],
                access_declaration: &[],
                response_capacity: 16_384,
                resources: Resources([10_000_000; 7]),
                gas_limit: call.gas_limit,
                max_fee_per_gas: call.max_fee_per_gas,
                max_priority_fee_per_gas: call.max_priority_fee_per_gas,
            },
            signer,
        )));
        assert_eq!(reproduced.signed_bytes(), raw, "native producer for {name}");
    }
}

fn assert_chain_result(vector: &Value, result: &NodeState) {
    assert_eq!(
        result
            .operations
            .iter()
            .map(|entry| &entry.0)
            .collect::<Vec<_>>(),
        vector["operations"]
            .as_array()
            .unwrap_or_else(|| panic!("operations"))
            .iter()
            .map(|name| name.as_str().unwrap_or(""))
            .collect::<Vec<_>>()
    );
    let records = vector["records"]
        .as_array()
        .unwrap_or_else(|| panic!("records"));
    let expected = records
        .iter()
        .map(|name| {
            vector["request"][name.as_str().unwrap_or("")]
                .as_str()
                .unwrap_or("")
        })
        .collect::<Vec<_>>();
    assert_eq!(
        result.submitted, expected,
        "refusals submit nothing; accepted records retain exact bytes and order"
    );
    assert_eq!(
        result.manifests,
        vec![must(hex::decode(
            vector["request"]["manifest_v2"].as_str().unwrap_or("")
        ))]
    );
    assert_eq!(result.receipts.len(), 5);
    let summary = must(workers::decode_manifest(&result.manifests[0]));
    let state = must(state::decode_shared_state(&result.bytes));
    let table = must(WorkerTable::decode(
        must(market::evaluators::authority::split_identity_section(must(
            state.section(Section::IdentityRoster),
        )))
        .0,
    ));
    let worker = table
        .get(summary.worker)
        .unwrap_or_else(|| panic!("enrolled peer"));
    assert_eq!(worker.state, WorkerState::Enrolled);
    assert_eq!(worker.metadata, summary.digest);
    let bid = must(market::rooms::RoomBook::decode(
        must(must(state::decode_shared_state(&result.bytes)).room_market()).1,
    ))
    .bid(must(Digest32::new([70; 32])))
    .unwrap_or_else(|error| panic!("{error:?}"))
    .unwrap_or_else(|| panic!("posted bid"));
    assert_eq!(bid.bond_amount, 100);
    assert_eq!(bid.layer_capacity, 16);
    assert!(must(market::bonds::bond_from_state(
        &result.bytes,
        must(Digest32::new([70; 32]))
    ))
    .is_some());
}

#[test]
fn enrol_route_uses_connection_bound_and_preserves_health_and_websocket_upgrade() {
    let chain = Arc::new(RouteChain(std::sync::atomic::AtomicBool::new(true)));
    let listener = route(signal::SignalAuthority::with_chain(chain, 125, [51; 32]));
    let mut health = must(std::net::TcpStream::connect(listener.local_addr()));
    must(health.write_all(b"GET /healthz HTTP/1.1\r\nHost: localhost\r\n\r\n"));
    assert_eq!(response(health), (200, json!({"ready": true})));
    let mut websocket = must(std::net::TcpStream::connect(listener.local_addr()));
    must(websocket.set_read_timeout(Some(Duration::from_secs(2))));
    must(websocket.write_all(b"GET / HTTP/1.1\r\nHost: localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Version: 13\r\nSec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n\r\n"));
    let mut reader = BufReader::new(websocket);
    let mut status = String::new();
    must(reader.read_line(&mut status));
    assert_eq!(status, "HTTP/1.1 101 Switching Protocols\r\n");
    drop(reader);
    let mut held = Vec::new();
    for _ in 0..signal::MAX_CONNECTIONS {
        let mut stream = must(std::net::TcpStream::connect(listener.local_addr()));
        // A partially read enrolment request owns a listener connection slot.
        must(stream.write_all(b"POST /enrol_peer HTTP/1.1\r\nHost:"));
        held.push(stream);
    }
    let mut excess = must(std::net::TcpStream::connect(listener.local_addr()));
    must(excess.set_read_timeout(Some(Duration::from_secs(2))));
    let mut bytes = [0; 128];
    let size = must(excess.read(&mut bytes));
    assert!(bytes[..size].starts_with(b"HTTP/1.1 503 Service Unavailable\r\n"));
    drop(held);
}
