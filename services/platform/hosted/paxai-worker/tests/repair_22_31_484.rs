use layerx_client::px::{PxNetworkHead, PxPaxeerHead};
use layerx_paxai_worker::{
    auth::ServiceError,
    discovery::{AuthorityEvidence, FinalizedAuthority, IdentityEvidence},
    membership::{MembershipError, MembershipObserver, Standing, Subject},
};
use layerx_programs_ai_market::{
    admission::{AdmissionContext, AdmissionTable, ApprovalTerms, Participant, TABLE_MAX_BYTES},
    codec::{self, derive_market, derive_worker, Envelope},
    dispatch,
    errors::{BAD_VERSION, CAPACITY, NON_CANONICAL},
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{read_state_chunk, CaptureFacts, FinalityEvidence, ReadProof, StateCapture},
    registry::{derive_rewards_account, F01_SECTION_CAP},
    registry_ops::{self, CallContext, Outcome, PolicySection},
    reputation_transition::{encode_joint, split_joint, HistorySeal, Region},
    state::{decode_shared_state, encode_shared_state, Section, SharedState},
    types::{
        AssetId, Authentication, ChainDomain, Digest32, MetadataDigest, Presence, PrincipalId,
        ProgramId, PublicKey32, RequestId, RubricDigest,
    },
    workers::{WorkerCurrent, WorkerState, WorkerTable, WORKER_TABLE_MAX_BYTES},
    MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_STATE_BYTES,
};
use std::{fmt::Debug, fs, path::PathBuf};

type Checked<T = ()> = Result<T, String>;
const HEIGHT: u64 = 1_000;

fn checked<T>(result: Result<T, impl Debug>) -> Checked<T> {
    result.map_err(|error| format!("{error:?}"))
}

fn encode_state(state: &SharedState<'_>) -> Checked<Vec<u8>> {
    let mut out = vec![0; checked(state.encoded_len())?];
    let mut control = vec![0; Section::Control.payload_cap()];
    let len = checked(encode_shared_state(state, &mut out, &mut control))?;
    out.truncate(len);
    Ok(out)
}

struct Fixture {
    state: Vec<u8>,
    subject: Subject,
    worker: WorkerCurrent,
    table: AdmissionTable,
    table_bytes: Vec<u8>,
    region: Region,
}

impl Fixture {
    fn new() -> Checked<Self> {
        let chain = checked(ChainDomain::new([10; 32]))?;
        let program = checked(ProgramId::new([11; 32]))?;
        let owner = checked(PrincipalId::new([12; 32]))?;
        let asset = checked(AssetId::new([13; 32]))?;
        let market = checked(derive_market(chain, program))?;
        let policy = checked(TaskPolicyV1::bounded_default(
            1,
            1,
            PolicyCommitments {
                model_artifact: checked(Digest32::new([1; 32]))?,
                dataset_artifact: [2; 32],
                benchmark_suite: checked(Digest32::new([3; 32]))?,
                rubric: checked(RubricDigest::new([4; 32]))?,
                task_schema: checked(Digest32::new([5; 32]))?,
                result_schema: checked(Digest32::new([6; 32]))?,
                service_terms: checked(Digest32::new([7; 32]))?,
            },
            100,
            1,
        ))?;
        let mut policy_bytes = [0; TASK_POLICY_BYTES];
        checked(policy.encode(&mut policy_bytes))?;
        let mut payload = owner.bytes().to_vec();
        payload.extend_from_slice(asset.as_bytes());
        payload.extend_from_slice(checked(derive_rewards_account(program, asset))?.as_bytes());
        payload.extend_from_slice(&[14; 32]);
        payload.push(0);
        payload.extend_from_slice(&policy_bytes);
        payload.extend_from_slice(&[16; 32]);
        let mut envelope_bytes = vec![0; MAX_ENVELOPE_BYTES];
        let len = checked(codec::encode_envelope(
            &Envelope {
                operation: dispatch::CREATE,
                chain,
                program,
                market,
                actor: owner,
                epoch: 0,
                config: 1,
                roster: Presence::Absent,
                sequence: 1,
                expiry: u64::MAX,
                request: checked(RequestId::new([1; 32]))?,
                payload: &payload,
                authentication: Authentication::Native,
            },
            &mut envelope_bytes,
        ))?;
        let envelope = checked(codec::decode_envelope(&envelope_bytes[..len]))?;
        let mut sections = vec![0; F01_SECTION_CAP];
        let mut event = vec![0; MAX_EVENT_BYTES];
        let Outcome::Applied { state, .. } = checked(registry_ops::apply(
            &CallContext {
                chain,
                program,
                principal: owner,
                height: HEIGHT,
            },
            None,
            &envelope,
            &mut sections,
            &mut event,
        ))?
        else {
            return Err("CREATE must apply".into());
        };
        let header = checked(PolicySection::decode(checked(
            state.section(Section::PolicyLifecycle),
        )?))?
        .header;
        let (region, _) = checked(split_joint(checked(
            state.section(Section::ReputationAdmission),
        )?))?;
        let region = region.ok_or("CREATE must produce the F07 region")?;
        let worker = WorkerCurrent {
            worker: checked(derive_worker(market, owner, [17; 32]))?,
            owner,
            delegate: PublicKey32([18; 32]),
            metadata: checked(MetadataDigest::new([19; 32]))?,
            generation: 1,
            key_version: 1,
            metadata_revision: 1,
            valid_from: HEIGHT,
            expiry: HEIGHT + 64,
            revocation_sequence: 0,
            effective_epoch: 0,
            last_sequence: 0,
            last_request_id: [0; 32],
            last_request_digest: [0; 32],
            last_result_digest: [0; 32],
            state: WorkerState::Enrolled,
            slot: 0,
            last_metadata_height: HEIGHT,
        };
        let mut workers = WorkerTable::new();
        checked(workers.insert(&worker))?;
        let mut identity = vec![0; WORKER_TABLE_MAX_BYTES];
        let len = checked(workers.encode(&mut identity))?;
        identity.truncate(len);
        let mut table = AdmissionTable::new();
        checked(table.approve(
            &AdmissionContext {
                market: &header,
                invoking_principal: owner,
                height: HEIGHT,
            },
            &ApprovalTerms {
                participant: Participant::Worker(worker.worker),
                owner,
                enrollment_nonce_commitment: checked(Digest32::new([20; 32]))?,
                delegate: worker.delegate,
                delegate_generation: 1,
                identity_commitment: checked(Digest32::new([21; 32]))?,
                effective_epoch: 0,
                config_version: 1,
                request: checked(RequestId::new([22; 32]))?,
                expiry_height: HEIGHT + 64,
            },
        ))?;
        let mut table_bytes = vec![0; TABLE_MAX_BYTES];
        let len = checked(table.encode(&mut table_bytes))?;
        table_bytes.truncate(len);
        let state = checked(state.replace_section(Section::IdentityRoster, &identity))?;
        Ok(Self {
            state: encode_state(&state)?,
            subject: Subject {
                chain,
                program,
                market,
                worker: worker.worker,
            },
            worker,
            table,
            table_bytes,
            region,
        })
    }

    fn joint(&self, sealed: bool) -> Checked<Vec<u8>> {
        let mut region = self.region.clone();
        if sealed {
            region.seal = Presence::Present(HistorySeal {
                epoch: 0,
                allowed: 0,
                eligible: 3,
            });
        }
        let mut joint = vec![0; Section::ReputationAdmission.payload_cap()];
        let len = checked(encode_joint(&region, &self.table_bytes, &mut joint))?;
        joint.truncate(len);
        let (decoded_region, table) = checked(split_joint(&joint))?;
        assert_eq!(decoded_region.as_ref(), Some(&region));
        assert_eq!(checked(AdmissionTable::decode(table))?, self.table);
        Ok(joint)
    }

    fn view(&self, section: &[u8]) -> Checked<View> {
        let state = checked(decode_shared_state(&self.state))?;
        let state = checked(state.replace_section(Section::ReputationAdmission, section))?;
        let state = encode_state(&state)?;
        let proof = ReadProof {
            chain: self.subject.chain,
            program: self.subject.program,
            native_state_root: checked(Digest32::new([0xA1; 32]))?,
            observed_sequence: 77,
            execution_height: HEIGHT,
            block_hash: checked(Digest32::new([0xBB; 32]))?,
        };
        let mut capture_buffer = vec![0; MAX_STATE_BYTES];
        let mut capture = StateCapture::new(&mut capture_buffer);
        let mut response = vec![0; 8_244];
        let mut revision = 0u64;
        let mut pinned = [0; 32];
        for offset in (0..state.len()).step_by(8_192) {
            let mut payload = revision.to_be_bytes().to_vec();
            payload.extend_from_slice(&pinned);
            payload.extend_from_slice(&checked(u32::try_from(offset))?.to_be_bytes());
            payload.extend_from_slice(&8_192u16.to_be_bytes());
            let len = checked(read_state_chunk(&state, &payload, &mut response))?;
            let chunk = checked(codec::decode_chunk_response(&response[..len]))?;
            revision = chunk.revision;
            pinned = chunk.digest.bytes();
            checked(capture.accept(&proof, &response[..len]))?;
        }
        let (state, facts) = checked(capture.finish())?;
        Ok(View {
            state: state.to_vec(),
            facts,
            finality: FinalityEvidence {
                native_state_root: proof.native_state_root,
                block_height: HEIGHT,
                block_hash: proof.block_hash,
                app_hash: checked(Digest32::new([0xC1; 32]))?,
                store_proof_digest: checked(Digest32::new([0xD3; 32]))?,
                rank: 4,
            },
            owner: IdentityEvidence {
                principal: self.worker.owner,
                primary_key: self.worker.delegate,
                frozen: false,
                execution_height: HEIGHT,
            },
        })
    }
}

struct View {
    state: Vec<u8>,
    facts: CaptureFacts,
    finality: FinalityEvidence,
    owner: IdentityEvidence,
}
impl View {
    fn evidence(&self) -> AuthorityEvidence<'_> {
        AuthorityEvidence {
            state: &self.state,
            facts: &self.facts,
            finality: &self.finality,
            head: PxNetworkHead {
                network_id: "paxeer".into(),
                paxeer: PxPaxeerHead {
                    chain_id: 125,
                    latest_block: HEIGHT,
                },
                layerx: None,
                latest_committed_height: HEIGHT,
            },
            owner: self.owner,
            roster: None,
            observed_ms: 5_000,
        }
    }
}

struct Directory(PathBuf);
impl Directory {
    fn new(name: &str) -> Checked<Self> {
        let path =
            std::env::temp_dir().join(format!("repair-22-31-484-{}-{name}", std::process::id()));
        checked(fs::create_dir(&path))?;
        Ok(Self(path))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn accepts(fixture: &Fixture, section: &[u8], name: &str) -> Checked {
    let view = fixture.view(section)?;
    let evidence = view.evidence();
    let subject = fixture.subject;
    let authority = checked(FinalizedAuthority::bind(
        &evidence,
        subject.chain,
        subject.program,
        subject.market,
        subject.worker,
    ))?;
    let expected = fixture.table.get(Participant::Worker(subject.worker));
    assert!(expected.is_some());
    assert_eq!(authority.membership(), expected);
    assert_eq!(authority.current_epoch(), fixture.table.current_epoch());
    assert_eq!(authority.record(), &fixture.worker);
    let directory = Directory::new(name)?;
    let mut observer = checked(MembershipObserver::open(&directory.0, subject))?;
    let standing = Standing::Approved {
        effective_epoch: 0,
        expiry_height: HEIGHT + 64,
    };
    assert_eq!(checked(observer.observe(Some(&evidence)))?, standing);
    let current = observer.current().ok_or("missing finalized membership")?;
    assert_eq!(current.meta, expected);
    assert_eq!(current.authority, authority);
    let record = observer.record().clone();
    drop(observer);
    let mut recovered = checked(MembershipObserver::open(&directory.0, subject))?;
    assert_eq!(recovered.record(), &record);
    assert!(recovered.current().is_none());
    assert_eq!(checked(recovered.observe(Some(&evidence)))?, standing);
    assert_eq!(
        recovered
            .current()
            .ok_or("missing recovered membership")?
            .meta,
        expected
    );
    Ok(())
}

#[test]
fn joint_section_reaches_both_finalized_consumers() -> Checked {
    let fixture = Fixture::new()?;
    for sealed in [false, true] {
        let joint = fixture.joint(sealed)?;
        assert_eq!(AdmissionTable::decode(&joint), Err(BAD_VERSION));
        assert_eq!(
            ServiceError::from(BAD_VERSION),
            ServiceError::UnsupportedVersion
        );
        accepts(&fixture, &joint, if sealed { "sealed" } else { "joint" })?;
    }
    Ok(())
}

#[test]
fn legacy_table_section_reaches_both_finalized_consumers() -> Checked {
    let fixture = Fixture::new()?;
    assert_eq!(checked(split_joint(&fixture.table_bytes))?.0, None);
    accepts(&fixture, &fixture.table_bytes, "legacy")
}

#[test]
fn rejected_joint_sections_return_typed_errors_and_clear_readiness() -> Checked {
    let fixture = Fixture::new()?;
    let joint = fixture.joint(true)?;
    let (region, _) = checked(split_joint(&joint))?;
    let mut invalid_reserved = joint.clone();
    let seal_end = joint.len() - fixture.table_bytes.len();
    invalid_reserved[seal_end - 1] = 1;
    let mut excessive_count = joint.clone();
    excessive_count[6] = u8::MAX;
    let cases = [
        (joint[..7].to_vec(), NON_CANONICAL),
        (joint[..seal_end - 1].to_vec(), NON_CANONICAL),
        (invalid_reserved, NON_CANONICAL),
        (excessive_count, CAPACITY),
    ];
    assert!(region.is_some());
    for (index, (section, error)) in cases.into_iter().enumerate() {
        assert_eq!(split_joint(&section), Err(error));
        let directory = Directory::new(&format!("invalid-{index}"))?;
        let mut observer = checked(MembershipObserver::open(&directory.0, fixture.subject))?;
        let good = fixture.view(&joint)?;
        checked(observer.observe(Some(&good.evidence())))?;
        let record = observer.record().clone();
        let before = checked(fs::read(directory.0.join("membership.record")))?;
        let invalid = fixture.view(&section)?;
        let evidence = invalid.evidence();
        let subject = fixture.subject;
        assert_eq!(
            FinalizedAuthority::bind(
                &evidence,
                subject.chain,
                subject.program,
                subject.market,
                subject.worker,
            ),
            Err(ServiceError::from(error))
        );
        assert_eq!(
            observer.observe(Some(&evidence)),
            Err(MembershipError::Service(error.into()))
        );
        assert!(observer.current().is_none());
        assert_eq!(observer.record(), &record);
        assert_eq!(
            checked(fs::read(directory.0.join("membership.record")))?,
            before
        );
    }
    Ok(())
}
