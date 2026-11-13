//! Task 22.33.233, audit 22.30.128: ai-real-consumer-proof.
//!
//! Proving tests: `ai_real_consumer_recovers_fresh_guest_captures` and
//! `ai_real_consumer_refuses_unauthenticated_evidence_across_restart`.
//! CREATE and every chunk still run through `dispatch::route`. The continuation
//! ruling uses a recorded hosted-consumer transcript instead of a programs ->
//! hosted-indexer dependency. `CONSUMER_TRANSCRIPT` was emitted once by the unchanged
//! production `ProjectionStore` (observe, promote, drop, open, snapshot, `market_view`,
//! `pending_events`), including real consensus and namespace membership verification.
//! Its source revision and source hashes identify the recorder's production inputs.
//! The gate emits fresh guest bytes, verifies commits, and compares canonical
//! bindings and authenticated read facts with that consumer recording, including
//! recovered state and all five failed promotions. No indexer implementation is
//! copied or substituted here. This does not prove signed `ProgramRead` receipts.
//!
//! Baseline: attempt 1 failed to compile (E0433 for `layerx_committed_proof` and
//! `layerx_indexer`); neither test executed. No extra baseline verify is run.
//! The authoritative repair record is `repair_22_33_233.kvx`.
//! Its [wiring.platform] block records the current platform wiring request.
//! Wiring request (domain 8, Platform): services/platform/hosted/indexer owns
//! transcript regeneration and its live `ProjectionStore` restart integration;
//! programs intentionally does not depend on that hosted service.
//! Qualification revision, command, exit and private log are recorded by hf-done.

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_committed_proof::{
    commit::{self, BlockIdFlag, Commit, CommitSig},
    header::{
        validators_hash, BlockId, ConsensusVersion, Header, PartSetHeader, Timestamp, Validator,
    },
    proto::{encode_length_delimited, encode_varint, encode_varint_field},
    ProofError,
};
use layerx_programs_ai_market::{
    codec::{self, Envelope, ResultStatus},
    dispatch::{self, Buffers, Operation, Routed},
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{
        self, bind_snapshot, CaptureFacts, FinalityEvidence, QueryError, ReadProof,
        SnapshotBinding, StateCapture, BINDING_MAX_BYTES, FINALIZED_RANK,
    },
    registry::derive_rewards_account,
    registry_ops::CallContext,
    types::{
        AssetId, Authentication, ChainDomain, Digest32, Presence, PrincipalId, ProgramId,
        RequestId, RubricDigest,
    },
    MAX_CHUNK_BYTES, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
    MERGED_CHAIN_DOMAIN, SHARED_STATE_KEY,
};
use serde_json::Value;
use sha2::{Digest as _, Sha256};
use std::fmt::Debug;

pub(crate) const HEIGHT: u64 = 1010;
const OWNER: [u8; 32] = [12; 32];
pub(crate) const PROGRAM: [u8; 32] = [11; 32];
pub(crate) const CHAIN_ID: &str = "ai-market-consumer-fixture";

#[track_caller]
fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("production path refused: {error:?}"))
}
fn digest(byte: u8) -> Digest32 {
    checked(Digest32::new([byte; 32]))
}
fn context() -> CallContext {
    CallContext {
        chain: checked(ChainDomain::new(MERGED_CHAIN_DOMAIN)),
        program: checked(ProgramId::new(PROGRAM)),
        principal: checked(PrincipalId::new(OWNER)),
        height: HEIGHT,
    }
}

fn envelope(operation: Operation, payload: &[u8]) -> Vec<u8> {
    let ctx = context();
    let read = operation.metadata().boundary == dispatch::CallBoundary::ProgramRead;
    let request = Envelope {
        operation,
        chain: ctx.chain,
        program: ctx.program,
        market: checked(codec::derive_market(ctx.chain, ctx.program)),
        actor: ctx.principal,
        epoch: 0,
        config: u64::from(!read),
        roster: Presence::Absent,
        sequence: u64::from(!read),
        expiry: HEIGHT + 1000,
        request: checked(RequestId::new([1; 32])),
        payload,
        authentication: Authentication::Native,
    };
    let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
    let n = checked(codec::encode_envelope(&request, &mut bytes));
    bytes.truncate(n);
    bytes
}

/// Real guest entrypoint routing, including its application-result framing.
fn route(operation: Operation, payload: &[u8], current: Option<&[u8]>) -> (Vec<u8>, Vec<u8>) {
    let request = envelope(operation, payload);
    let mut next = vec![0xa5; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0xa5; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let routed = checked(dispatch::route(
        &context(),
        &request,
        current,
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    ));
    let n = match routed {
        Routed::Applied {
            state_len,
            result_len,
            event_len,
            ..
        } => {
            assert_eq!(operation, dispatch::CREATE);
            assert!(event_len > 0);
            next.truncate(state_len);
            result_len
        }
        Routed::Unchanged { result_len } => {
            assert_eq!(operation, dispatch::READ_STATE_CHUNK);
            assert!(next.iter().all(|byte| *byte == 0xa5));
            assert!(event.iter().all(|byte| *byte == 0xa5));
            next.clear();
            result_len
        }
        other @ Routed::Refused { .. } => panic!("guest refused request: {other:?}"),
    };
    let result = checked(codec::decode_result(&result[..n]));
    assert_eq!(result.status, ResultStatus::Ok);
    assert_eq!(result.error, None);
    assert_eq!(
        result.request,
        Presence::Present(checked(
            checked(codec::decode_envelope(&request)).request_digest()
        ))
    );
    (next, result.payload.to_vec())
}

pub(crate) fn producer_state() -> Vec<u8> {
    let policy = checked(TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: digest(1),
            dataset_artifact: [2; 32],
            benchmark_suite: digest(3),
            rubric: checked(RubricDigest::new([4; 32])),
            task_schema: digest(5),
            result_schema: digest(6),
            service_terms: digest(7),
        },
        100,
        1,
    ));
    let mut policy_bytes = vec![0; TASK_POLICY_BYTES];
    checked(policy.encode(&mut policy_bytes));
    let asset = checked(AssetId::new([13; 32]));
    let rewards = checked(derive_rewards_account(context().program, asset));
    let mut payload = OWNER.to_vec();
    payload.extend_from_slice(asset.as_bytes());
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&[14; 32]);
    payload.push(0);
    payload.extend_from_slice(&policy_bytes);
    payload.extend_from_slice(&[16; 32]);
    route(dispatch::CREATE, &payload, None).0
}

pub(crate) fn authority() -> Vec<Validator> {
    vec![Validator {
        pub_key: SigningKey::from_bytes(&[7; 32]).verifying_key().to_bytes(),
        voting_power: 10,
    }]
}

/// Committed peer fixture: native namespace manifest and ICS23 leaf operations,
/// matching `indexer/tests/repair_22_31_350.rs` and the lxphost store layout.
fn native_membership(state: &[u8], program: [u8; 32]) -> (Vec<u8>, [u8; 32]) {
    fn leaf(key: &[u8], value: &[u8], prefix: &[u8]) -> (Vec<u8>, [u8; 32]) {
        let mut operation = Vec::new();
        for (field, value) in [(1, 1), (2, 0), (3, 1), (4, 1)] {
            encode_varint_field(field, value, &mut operation);
        }
        encode_length_delimited(5, prefix, &mut operation);
        let mut proof = Vec::new();
        encode_length_delimited(1, key, &mut proof);
        encode_length_delimited(2, value, &mut proof);
        encode_length_delimited(3, &operation, &mut proof);
        let mut preimage = prefix.to_vec();
        encode_varint(checked(u64::try_from(key.len())), &mut preimage);
        preimage.extend_from_slice(key);
        encode_varint(32, &mut preimage);
        preimage.extend_from_slice(&Sha256::digest(value));
        (proof, Sha256::digest(preimage).into())
    }
    let mut manifest = vec![0, 1];
    manifest.extend_from_slice(&1u32.to_be_bytes());
    manifest.extend_from_slice(&checked(u16::try_from(SHARED_STATE_KEY.len())).to_be_bytes());
    manifest.extend_from_slice(SHARED_STATE_KEY);
    manifest.extend_from_slice(&Sha256::digest(state));
    manifest.extend_from_slice(&checked(u32::try_from(state.len())).to_be_bytes());
    let mut head = vec![0, 1];
    head.extend_from_slice(&1u32.to_be_bytes());
    head.extend_from_slice(&Sha256::digest(&manifest));
    let mut key = b"\x01\x00\x09progstor".to_vec();
    key.extend_from_slice(&program);
    key.push(1);
    let (proof, store_root) = leaf(&key, &head, &[0, 2, 2]);
    let (outer, app_hash) = leaf(b"lxphost", &store_root, &[0]);
    let mut multistore = Vec::new();
    encode_length_delimited(1, &outer, &mut multistore);
    // Byte arrays have the same JSON representation as ProgramStateProof; no
    // replacement implementation of the production membership verifier is used.
    let json = format!(
        "{{\"membership\":{{\"proof\":{proof:?},\"multistore\":{multistore:?}}},\"manifest\":{manifest:?}}}"
    );
    (json.into_bytes(), app_hash)
}

fn signed_commit(header: &Header) -> Commit {
    let key = SigningKey::from_bytes(&[7; 32]);
    let mut commit = Commit {
        height: checked(u64::try_from(header.height)),
        round: 0,
        block_id: BlockId {
            hash: header.hash().to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![0xa5; 32],
            },
        },
        signatures: Vec::new(),
    };
    let mut vote = Vec::new();
    encode_varint_field(1, 2, &mut vote);
    vote.push(0x11);
    vote.extend_from_slice(&commit.height.to_le_bytes());
    let mut parts = Vec::new();
    encode_varint_field(1, 1, &mut parts);
    encode_length_delimited(2, &commit.block_id.part_set_header.hash, &mut parts);
    let mut block = Vec::new();
    encode_length_delimited(1, &commit.block_id.hash, &mut block);
    encode_length_delimited(2, &parts, &mut block);
    encode_length_delimited(4, &block, &mut vote);
    let mut time = Vec::new();
    encode_varint_field(1, checked(u64::try_from(header.time.seconds)), &mut time);
    encode_varint_field(2, u64::from(header.time.nanos), &mut time);
    encode_length_delimited(5, &time, &mut vote);
    encode_length_delimited(6, header.chain_id.as_bytes(), &mut vote);
    let mut sign_bytes = Vec::new();
    encode_varint(checked(u64::try_from(vote.len())), &mut sign_bytes);
    sign_bytes.extend_from_slice(&vote);
    commit.signatures.push(CommitSig {
        block_id_flag: BlockIdFlag::Commit,
        validator_address: Sha256::digest(key.verifying_key().as_bytes())[..20].to_vec(),
        timestamp: header.time,
        signature: Some(key.sign(&sign_bytes).to_bytes()),
    });
    commit
}

pub(crate) struct Material {
    pub(crate) header: Header,
    pub(crate) commit: Commit,
    pub(crate) proof: Vec<u8>,
}
impl Material {
    pub(crate) fn new(state: &[u8], program: [u8; 32]) -> Self {
        let (proof, app_hash) = native_membership(state, program);
        let authority = authority();
        let header = Header {
            chain_id: CHAIN_ID.to_owned(),
            height: checked(i64::try_from(HEIGHT)),
            app_hash: app_hash.to_vec(),
            version: ConsensusVersion { block: 11, app: 1 },
            time: Timestamp {
                seconds: 1000,
                nanos: 123,
            },
            last_block_id: BlockId::default(),
            last_commit_hash: vec![1; 32],
            data_hash: vec![2; 32],
            validators_hash: validators_hash(&authority),
            next_validators_hash: validators_hash(&authority).to_vec(),
            consensus_hash: vec![3; 32],
            last_results_hash: vec![4; 32],
            evidence_hash: vec![5; 32],
            proposer_address: vec![6; 20],
        };
        Self {
            commit: signed_commit(&header),
            header,
            proof,
        }
    }
    fn verify(&self) -> Result<(), ProofError> {
        let validators = authority();
        let block_hash = layerx_committed_proof::verify_header(&self.header, &validators)?;
        commit::verify_commit(CHAIN_ID, &validators, block_hash, HEIGHT, &self.commit)?;
        Ok(())
    }
    fn finality(&self) -> FinalityEvidence {
        checked(self.verify());
        FinalityEvidence {
            native_state_root: checked(Digest32::new(checked(
                self.header.app_hash.as_slice().try_into(),
            ))),
            block_height: HEIGHT,
            block_hash: checked(Digest32::new(self.header.hash())),
            app_hash: checked(Digest32::new(checked(
                self.header.app_hash.as_slice().try_into(),
            ))),
            store_proof_digest: checked(Digest32::new(Sha256::digest(&self.proof).into())),
            rank: FINALIZED_RANK,
        }
    }
}

/// Fresh `ProgramRead` bytes bound to a verified commit; membership is checked against
/// the recorded production consumer inputs below.
pub(crate) fn capture(state: &[u8], material: &Material, sequence: u64) -> (Vec<u8>, CaptureFacts) {
    checked(material.verify());
    let header = checked(queries::read_header(
        state,
        context().chain,
        context().program,
    ));
    let proof = ReadProof {
        chain: header.chain,
        program: header.program,
        native_state_root: material.finality().native_state_root,
        observed_sequence: sequence,
        execution_height: HEIGHT,
        block_hash: checked(Digest32::new(material.header.hash())),
    };
    let mut buffer = vec![0; MAX_STATE_BYTES];
    let mut capture = StateCapture::new(&mut buffer);
    let mut pin = None;
    for offset in (0..state.len()).step_by(MAX_CHUNK_BYTES) {
        let (revision, digest) = pin.unwrap_or((0u64, [0; 32]));
        let mut payload = revision.to_be_bytes().to_vec();
        payload.extend_from_slice(&digest);
        payload.extend_from_slice(&checked(u32::try_from(offset)).to_be_bytes());
        payload.extend_from_slice(&checked(u16::try_from(MAX_CHUNK_BYTES)).to_be_bytes());
        let (_, body) = route(dispatch::READ_STATE_CHUNK, &payload, Some(state));
        let chunk = checked(codec::decode_chunk_response(&body));
        assert_eq!(chunk.digest, header.digest);
        pin = Some((chunk.revision, chunk.digest.bytes()));
        checked(capture.accept(&proof, &body));
    }
    let (captured, facts) = checked(capture.finish());
    assert_eq!(captured, state);
    assert_eq!(facts.digest, header.digest);
    assert_eq!(facts.chunks, state.len().div_ceil(MAX_CHUNK_BYTES));
    (captured.to_vec(), facts)
}

fn recorded_bytes(record: &Value, field: &str) -> Vec<u8> {
    let text = record[field]
        .as_str()
        .unwrap_or_else(|| panic!("missing transcript field {field}"));
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|offset| checked(u8::from_str_radix(&text[offset..offset + 2], 16)))
        .collect()
}

pub(crate) fn transcript() -> Value {
    let recorded: Value = checked(serde_json::from_str(CONSUMER_TRANSCRIPT));
    assert_eq!(recorded["format"], 1);
    assert_eq!(
        recorded["source_revision"],
        "116ae09395ecb90e9547e536cc7d40168f788fa9"
    );
    assert_eq!(
        recorded["consumer"],
        "services/platform/hosted/indexer/src/ai_market.rs::ProjectionStore"
    );
    assert_eq!(
        recorded_bytes(&recorded, "consumer_source_sha256").len(),
        32
    );
    assert_eq!(
        recorded_bytes(&recorded, "membership_source_sha256").len(),
        32
    );
    recorded
}

fn assert_recorded_capture(
    record: &Value,
    state: &[u8],
    facts: &CaptureFacts,
    material: &Material,
) {
    assert_eq!(
        recorded_bytes(record, "state_sha256"),
        Sha256::digest(state).as_slice()
    );
    assert_eq!(
        recorded_bytes(record, "state_digest"),
        facts.digest.as_bytes()
    );
    assert_eq!(record["sequence"], facts.proof.observed_sequence);
    assert_eq!(record["total_bytes"], facts.total_bytes);
    assert_eq!(record["chunks"], facts.chunks);
    assert_eq!(
        recorded_bytes(record, "authenticated_chain"),
        facts.proof.chain.as_bytes()
    );
    assert_eq!(
        recorded_bytes(record, "authenticated_program"),
        facts.proof.program.as_bytes()
    );
    assert_eq!(
        recorded_bytes(record, "native_root"),
        facts.proof.native_state_root.as_bytes()
    );
    assert_eq!(
        recorded_bytes(record, "block_hash"),
        facts.proof.block_hash.as_bytes()
    );
    assert_eq!(
        recorded_bytes(record, "proof_sha256"),
        Sha256::digest(&material.proof).as_slice()
    );
}

#[test]
pub(crate) fn ai_real_consumer_recovers_fresh_guest_captures() {
    let state = producer_state();
    let material = Material::new(&state, PROGRAM);
    let recorded = transcript();
    let records = recorded["successes"]
        .as_array()
        .unwrap_or_else(|| panic!("missing successes"));
    assert_eq!(records.len(), 2);
    let mut previous = None;
    for (sequence, record) in [77, 78].into_iter().zip(records) {
        let (captured, facts) = capture(&state, &material, sequence);
        assert_recorded_capture(record, &captured, &facts, &material);
        let binding = checked(bind_snapshot(
            &captured,
            &facts,
            &material.finality(),
            sequence * 100 + 1,
        ));
        let snapshot_id = checked(binding.snapshot_id());
        assert_eq!(record["observed_rank"], 0);
        assert_eq!(record["rank"], FINALIZED_RANK);
        assert_eq!(record["projection"], "FinalizedPublishable");
        assert_eq!(record["freshness"], "Current");
        assert_eq!(binding.observed_sequence, sequence);
        assert_eq!(binding.state_digest, facts.digest);
        assert_eq!(
            checked(binding.finalized_head()),
            (HEIGHT, checked(Digest32::new(material.header.hash())))
        );
        assert_eq!(
            binding.store_proof_digest.bytes(),
            <[u8; 32]>::from(Sha256::digest(&material.proof))
        );
        let mut encoded = vec![0; BINDING_MAX_BYTES];
        let n = checked(binding.encode(&mut encoded));
        encoded.truncate(n);
        // Each byte string was returned by a distinct real consumer operation.
        // The recovered values came from a new ProjectionStore after drop/open.
        for field in [
            "record_binding",
            "before_binding",
            "recovered_binding",
            "event_binding",
        ] {
            let bytes = recorded_bytes(record, field);
            assert_eq!(encoded, bytes, "consumer operation {field}");
            let decoded = checked(SnapshotBinding::decode(&bytes));
            assert_eq!(decoded, binding);
            checked(decoded.verify_capture(&captured, &facts));
            assert_eq!(
                checked(decoded.finalized_head()),
                checked(binding.finalized_head())
            );
        }
        assert_eq!(record["record_equal_after_reopen"], true);
        assert_eq!(recorded_bytes(record, "database_sha256").len(), 32);
        for field in ["snapshot_id", "recovered_snapshot_id", "event_snapshot_id"] {
            assert_eq!(recorded_bytes(record, field), snapshot_id.as_bytes());
        }
        for field in ["source_activity", "recovered_source_activity"] {
            assert_eq!(
                recorded_bytes(record, field),
                digest(checked(u8::try_from(sequence))).as_bytes()
            );
        }
        assert_eq!(record["recovered_projection"], "FinalizedPublishable");
        assert_eq!(
            record["pending_count"],
            checked(usize::try_from(sequence - 76))
        );
        if let Some(id) = previous {
            assert_ne!(snapshot_id, id);
        }
        previous = Some(snapshot_id);
    }
}

#[test]
pub(crate) fn ai_real_consumer_refuses_unauthenticated_evidence_across_restart() {
    let state = producer_state();
    let valid = Material::new(&state, PROGRAM);
    let (captured, facts) = capture(&state, &valid, 77);
    let recorded = transcript();
    let refusals = recorded["refusals"]
        .as_array()
        .unwrap_or_else(|| panic!("missing refusals"));
    assert_eq!(refusals.len(), 5);
    let saved = checked(SnapshotBinding::decode(&recorded_bytes(
        &recorded["successes"][0],
        "recovered_binding",
    )));
    checked(saved.verify_capture(&captured, &facts));
    for (case, record) in refusals.iter().enumerate() {
        let mut material = Material::new(&state, PROGRAM);
        let mut offered = facts;
        let mut read_verification = 2;
        match case {
            0 => {
                material.commit.signatures[0]
                    .signature
                    .as_mut()
                    .unwrap_or_else(|| panic!("fixture signature absent"))[0] ^= 1;
            }
            1 => material = Material::new(&state, [99; 32]),
            2 => material.proof.push(0),
            3 => offered.proof.block_hash = digest(99),
            4 => read_verification = 1,
            _ => unreachable!(),
        }
        // Compare every changed consumer input, not just its recorded error.
        assert_eq!(record["case"], case);
        assert_eq!(
            recorded_bytes(record, "proof_sha256"),
            Sha256::digest(&material.proof).as_slice()
        );
        assert_eq!(
            recorded_bytes(record, "native_root"),
            material.header.app_hash
        );
        assert_eq!(recorded_bytes(record, "block_hash"), material.header.hash());
        assert_eq!(
            recorded_bytes(record, "signature"),
            material.commit.signatures[0]
                .signature
                .unwrap_or_else(|| panic!("fixture signature absent"))
        );
        assert_eq!(
            recorded_bytes(record, "read_block_hash"),
            offered.proof.block_hash.as_bytes()
        );
        assert_eq!(record["read_verification"], read_verification);
        match case {
            0 => assert_eq!(material.verify(), Err(ProofError::InvalidCommitSignature)),
            1 => assert_eq!(
                bind_snapshot(&captured, &offered, &material.finality(), 2000),
                Err(QueryError::BindingMismatch)
            ),
            2 => assert_ne!(
                saved.store_proof_digest.bytes(),
                <[u8; 32]>::from(Sha256::digest(&material.proof))
            ),
            3 => assert_eq!(
                saved.verify_capture(&captured, &offered),
                Err(QueryError::BindingMismatch)
            ),
            4 => assert_eq!(read_verification, 1),
            _ => unreachable!(),
        }
        assert_eq!(
            record["error"],
            if case >= 3 {
                "BindingMismatch"
            } else {
                "FinalityUnavailable"
            }
        );
        let offered_binding = checked(bind_snapshot(&captured, &offered, &valid.finality(), 2000));
        let snapshot_id = checked(offered_binding.snapshot_id());
        assert_eq!(
            recorded_bytes(record, "snapshot_id"),
            snapshot_id.as_bytes()
        );
        assert_eq!(
            recorded_bytes(record, "recovered_snapshot_id"),
            snapshot_id.as_bytes()
        );
        assert_eq!(
            recorded_bytes(record, "recovered_state_digest"),
            facts.digest.as_bytes()
        );
        assert_eq!(record["recovered_rank"], 0);
        assert!(record["recovered_binding"].is_null());
        assert_eq!(record["recovered_projection_finalized"], false);
        assert_eq!(record["view_error"], "FinalityUnavailable");
        assert_eq!(record["pending_count"], 0);
    }
}

// Recorded peer fixture, produced by the real consumer at implementation time.
const CONSUMER_TRANSCRIPT: &str = r#"{
  "consumer": "services/platform/hosted/indexer/src/ai_market.rs::ProjectionStore",
  "consumer_source_sha256": "ea9d74fffbd8142a5e729dff6b500e6c905df536338bb6d4a58195b7ff78709f",
  "format": 1,
  "membership_source_sha256": "4febdba4ca0e65df633c39a729dbb4b8ed07a65a0f755a2c1a915887a451ae19",
  "refusals": [
    {
      "block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "case": 0,
      "error": "FinalityUnavailable",
      "native_root": "21514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0",
      "pending_count": 0,
      "proof_sha256": "880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668",
      "read_block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "read_verification": 2,
      "recovered_binding": null,
      "recovered_projection_finalized": false,
      "recovered_rank": 0,
      "recovered_snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "recovered_state_digest": "2362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb",
      "signature": "b91c086123245f456ec7c6e28e313d58a9db27e0c442e368a0b2804b4471f4600c1e1907fa1f9246e2715f503de85b9d1c9494e5b525e5894489da2315a8f707",
      "snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "view_error": "FinalityUnavailable"
    },
    {
      "block_hash": "9950268b2a2dc0ef92e1320c9b3adaf6105a6e699c01e840e1158c4f41b4ae66",
      "case": 1,
      "error": "FinalityUnavailable",
      "native_root": "98b86ae36e88f8c890059f956ffb692356bf9ca9e16042e783cefb72f04d0640",
      "pending_count": 0,
      "proof_sha256": "c78956eacc12198ebc6683fb0fc69d853842d950f88451c44edfb6a3b5f417d5",
      "read_block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "read_verification": 2,
      "recovered_binding": null,
      "recovered_projection_finalized": false,
      "recovered_rank": 0,
      "recovered_snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "recovered_state_digest": "2362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb",
      "signature": "9c78d98f3ba82e014e2c349edbefc548e3e46a5368f1681ee933b74a5617d5971cf76ea1f75eea1fd14fa2e910a5e176be648498a87628dd0da502246d64240f",
      "snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "view_error": "FinalityUnavailable"
    },
    {
      "block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "case": 2,
      "error": "FinalityUnavailable",
      "native_root": "21514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0",
      "pending_count": 0,
      "proof_sha256": "ecc1147496294355d40bd3ab0135e4af79c5a8a8d3d7a680049c3dcf8089a4e1",
      "read_block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "read_verification": 2,
      "recovered_binding": null,
      "recovered_projection_finalized": false,
      "recovered_rank": 0,
      "recovered_snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "recovered_state_digest": "2362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb",
      "signature": "b81c086123245f456ec7c6e28e313d58a9db27e0c442e368a0b2804b4471f4600c1e1907fa1f9246e2715f503de85b9d1c9494e5b525e5894489da2315a8f707",
      "snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "view_error": "FinalityUnavailable"
    },
    {
      "block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "case": 3,
      "error": "BindingMismatch",
      "native_root": "21514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0",
      "pending_count": 0,
      "proof_sha256": "880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668",
      "read_block_hash": "6363636363636363636363636363636363636363636363636363636363636363",
      "read_verification": 2,
      "recovered_binding": null,
      "recovered_projection_finalized": false,
      "recovered_rank": 0,
      "recovered_snapshot_id": "b0c2c5ff22426f825d523c42b273a639ad31fde9e1fbe8ec8ae7fc4058b968ab",
      "recovered_state_digest": "2362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb",
      "signature": "b81c086123245f456ec7c6e28e313d58a9db27e0c442e368a0b2804b4471f4600c1e1907fa1f9246e2715f503de85b9d1c9494e5b525e5894489da2315a8f707",
      "snapshot_id": "b0c2c5ff22426f825d523c42b273a639ad31fde9e1fbe8ec8ae7fc4058b968ab",
      "view_error": "FinalityUnavailable"
    },
    {
      "block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "case": 4,
      "error": "BindingMismatch",
      "native_root": "21514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0",
      "pending_count": 0,
      "proof_sha256": "880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668",
      "read_block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "read_verification": 1,
      "recovered_binding": null,
      "recovered_projection_finalized": false,
      "recovered_rank": 0,
      "recovered_snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "recovered_state_digest": "2362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb",
      "signature": "b81c086123245f456ec7c6e28e313d58a9db27e0c442e368a0b2804b4471f4600c1e1907fa1f9246e2715f503de85b9d1c9494e5b525e5894489da2315a8f707",
      "snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "view_error": "FinalityUnavailable"
    }
  ],
  "source_revision": "116ae09395ecb90e9547e536cc7d40168f788fa9",
  "successes": [
    {
      "authenticated_chain": "62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6",
      "authenticated_program": "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b",
      "before_binding": "000162b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f60b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b744eebed105eb139035ae582bae51c8c92208f1a7f1e1933c661c0e9df02bc2c000000000000004d00000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b000000000000000012362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb000000000000000001a67386b575a56979d7c4ec20c110df67bab48fb3821b3b458ae3adad0d423d230000000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668040000000000001e150000000000000000",
      "block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "chunks": 1,
      "database_sha256": "5dec17fa9ded20ba65943c0c19a94274f36d62f615a9d62acf52eec779a3ffaa",
      "event_binding": "000162b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f60b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b744eebed105eb139035ae582bae51c8c92208f1a7f1e1933c661c0e9df02bc2c000000000000004d00000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b000000000000000012362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb000000000000000001a67386b575a56979d7c4ec20c110df67bab48fb3821b3b458ae3adad0d423d230000000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668040000000000001e150000000000000000",
      "event_snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "freshness": "Current",
      "native_root": "21514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0",
      "observed_rank": 0,
      "pending_count": 1,
      "projection": "FinalizedPublishable",
      "proof_sha256": "880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668",
      "rank": 4,
      "record_binding": "000162b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f60b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b744eebed105eb139035ae582bae51c8c92208f1a7f1e1933c661c0e9df02bc2c000000000000004d00000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b000000000000000012362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb000000000000000001a67386b575a56979d7c4ec20c110df67bab48fb3821b3b458ae3adad0d423d230000000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668040000000000001e150000000000000000",
      "record_equal_after_reopen": true,
      "recovered_binding": "000162b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f60b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b744eebed105eb139035ae582bae51c8c92208f1a7f1e1933c661c0e9df02bc2c000000000000004d00000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b000000000000000012362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb000000000000000001a67386b575a56979d7c4ec20c110df67bab48fb3821b3b458ae3adad0d423d230000000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668040000000000001e150000000000000000",
      "recovered_projection": "FinalizedPublishable",
      "recovered_snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "recovered_source_activity": "4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d",
      "sequence": 77,
      "snapshot_id": "e79f506c76b52e843255052e4f795b49b6f6770c6503b848788293d30b925ed7",
      "source_activity": "4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d4d",
      "state_digest": "2362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb",
      "state_sha256": "2e846ebac9f0c52a2a8241ef681a039ae7f50aebadb06f0a59310916ba8eefff",
      "total_bytes": 1084
    },
    {
      "authenticated_chain": "62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6",
      "authenticated_program": "0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b",
      "before_binding": "000162b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f60b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b744eebed105eb139035ae582bae51c8c92208f1a7f1e1933c661c0e9df02bc2c000000000000004e00000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b000000000000000012362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb000000000000000001a67386b575a56979d7c4ec20c110df67bab48fb3821b3b458ae3adad0d423d230000000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668040000000000001e790000000000000000",
      "block_hash": "abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c3",
      "chunks": 1,
      "database_sha256": "258fae3eaf0adaa19b05cfd157827632fabbeb5af6c1f475e2da2cd8302f6eee",
      "event_binding": "000162b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f60b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b744eebed105eb139035ae582bae51c8c92208f1a7f1e1933c661c0e9df02bc2c000000000000004e00000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b000000000000000012362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb000000000000000001a67386b575a56979d7c4ec20c110df67bab48fb3821b3b458ae3adad0d423d230000000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668040000000000001e790000000000000000",
      "event_snapshot_id": "9cb4665739b71c8f7534098d0c4ec0aa9bc12126f559614154e3c6f0976ba282",
      "freshness": "Current",
      "native_root": "21514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0",
      "observed_rank": 0,
      "pending_count": 2,
      "projection": "FinalizedPublishable",
      "proof_sha256": "880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668",
      "rank": 4,
      "record_binding": "000162b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f60b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b744eebed105eb139035ae582bae51c8c92208f1a7f1e1933c661c0e9df02bc2c000000000000004e00000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b000000000000000012362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb000000000000000001a67386b575a56979d7c4ec20c110df67bab48fb3821b3b458ae3adad0d423d230000000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668040000000000001e790000000000000000",
      "record_equal_after_reopen": true,
      "recovered_binding": "000162b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f60b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b0b744eebed105eb139035ae582bae51c8c92208f1a7f1e1933c661c0e9df02bc2c000000000000004e00000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b000000000000000012362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb000000000000000001a67386b575a56979d7c4ec20c110df67bab48fb3821b3b458ae3adad0d423d230000000000000003f2abe1bd580ca46f0fd46c0a56031309feed1ebd2475bfe36b3d3509ce112c39c321514b4d3a58d5a6e6979fce4cb9f4cdfa80000e274ff7208c661a6c325a67b0880a61049e8d0f99f612275a77986b174b1f36da32487f0188c1ed9b7cedf668040000000000001e790000000000000000",
      "recovered_projection": "FinalizedPublishable",
      "recovered_snapshot_id": "9cb4665739b71c8f7534098d0c4ec0aa9bc12126f559614154e3c6f0976ba282",
      "recovered_source_activity": "4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e",
      "sequence": 78,
      "snapshot_id": "9cb4665739b71c8f7534098d0c4ec0aa9bc12126f559614154e3c6f0976ba282",
      "source_activity": "4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e4e",
      "state_digest": "2362af4a9e9a60e9d0c3595a01caa9aa061f1de35e100b7070079ca571b405bb",
      "state_sha256": "2e846ebac9f0c52a2a8241ef681a039ae7f50aebadb06f0a59310916ba8eefff",
      "total_bytes": 1084
    }
  ]
}"#;
