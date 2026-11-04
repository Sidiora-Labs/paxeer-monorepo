use layerx_programs_ai_market::codec::{self, derive_market};
use layerx_programs_ai_market::dispatch;
use layerx_programs_ai_market::evidence::ArtifactContext;
use layerx_programs_ai_market::policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES};
use layerx_programs_ai_market::queries::{
    bind_snapshot, read_state_chunk, FinalityEvidence, ReadProof, SnapshotBinding, StateCapture,
};
use layerx_programs_ai_market::registry::{derive_rewards_account, F01_SECTION_CAP};
use layerx_programs_ai_market::registry_ops::{self, CallContext, Outcome, PolicySection};
use layerx_programs_ai_market::state::{self, Section};
use layerx_programs_ai_market::tasks::{TaskBinding, TaskStatus};
use layerx_programs_ai_market::types::*;
use layerx_programs_ai_market::{
    MAX_CHUNK_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
};
use serde_json::{json, Value};

pub const TASK: [u8; 32] = [0x31; 32];

pub fn ctx() -> ArtifactContext {
    let chain = ChainDomain::new([0x11; 32]).unwrap();
    let program = ProgramId::new([0x22; 32]).unwrap();
    ArtifactContext {
        chain,
        program,
        market: derive_market(chain, program).unwrap(),
        policy: PolicyDigest::new([0x44; 32]).unwrap(),
    }
}
fn encode_shared(shared: &state::SharedState<'_>) -> Vec<u8> {
    let mut out = vec![0; shared.encoded_len().unwrap()];
    let mut scratch = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(shared, &mut out, &mut scratch).unwrap();
    out.truncate(n);
    out
}
/// Real F01 CREATE through `registry_ops::apply`, then a task region holding
/// one canonical CANCELLED record for TASK.
fn cancelled_task_state() -> Vec<u8> {
    let (chain, program) = (ctx().chain, ctx().program);
    let digest = |b: u8| Digest32::new([b; 32]).unwrap();
    let policy = TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: digest(1),
            dataset_artifact: [2; 32],
            benchmark_suite: digest(3),
            rubric: RubricDigest::new([4; 32]).unwrap(),
            task_schema: digest(5),
            result_schema: digest(6),
            service_terms: digest(7),
        },
        100,
        1,
    )
    .unwrap();
    let mut encoded_policy = vec![0; TASK_POLICY_BYTES];
    policy.encode(&mut encoded_policy).unwrap();
    let rewards = derive_rewards_account(program, AssetId::new([13; 32]).unwrap()).unwrap();
    let mut payload = [12; 32].to_vec();
    payload.extend_from_slice(&[13; 32]);
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&[14; 32]);
    payload.push(0);
    payload.extend_from_slice(&encoded_policy);
    payload.extend_from_slice(&[16; 32]);
    let owner = PrincipalId::new([12; 32]).unwrap();
    let envelope = codec::Envelope {
        operation: dispatch::CREATE,
        chain,
        program,
        market: ctx().market,
        actor: owner,
        epoch: 0,
        config: 1,
        roster: Presence::Absent,
        sequence: 1,
        expiry: 1_000_000,
        request: RequestId::new([1; 32]).unwrap(),
        payload: &payload,
        authentication: Authentication::Native,
    };
    let mut encoded = vec![0; 16_384];
    let n = codec::encode_envelope(&envelope, &mut encoded).unwrap();
    let validated = codec::decode_envelope(&encoded[..n]).unwrap();
    let call = CallContext {
        chain,
        program,
        principal: owner,
        height: 1000,
    };
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let Outcome::Applied { state: created, .. } =
        registry_ops::apply(&call, None, &validated, &mut section, &mut event).unwrap()
    else {
        panic!("fresh create retried");
    };
    let binding = TaskBinding {
        task: TaskId::new(TASK).unwrap(),
        requester: PrincipalId::new([0x61; 32]).unwrap(),
        worker: WorkerId::new([0x62; 32]).unwrap(),
        input: digest(0x67),
        deadline: 5_000,
        status: TaskStatus::Cancelled,
        acknowledgement: None,
        result: None,
        admission: digest(0x65),
    };
    let mut region = 1u16.to_be_bytes().to_vec();
    region.push(0);
    region.extend_from_slice(&binding.encode().unwrap());
    let mut policy_section =
        PolicySection::decode(created.section(Section::PolicyLifecycle).unwrap()).unwrap();
    policy_section.task_region = &region;
    let mut bytes = vec![0; Section::PolicyLifecycle.payload_cap()];
    let n = policy_section.encode(&mut bytes).unwrap();
    encode_shared(
        &created
            .replace_section(Section::PolicyLifecycle, &bytes[..n])
            .unwrap(),
    )
}
/// Capture the committed fixture through the real ai-market chunk producer and codec.
pub fn observation(request: &str, root: &str, rank: u8) -> (Value, SnapshotBinding) {
    let state = cancelled_task_state();
    let digest = |b| Digest32::new([b; 32]).unwrap();
    let proof = ReadProof {
        chain: ctx().chain,
        program: ctx().program,
        native_state_root: digest(0x5a),
        observed_sequence: 77,
        execution_height: 1010,
        block_hash: digest(0xcc),
    };
    let mut buffer = vec![0; MAX_STATE_BYTES];
    let mut capture = StateCapture::new(&mut buffer);
    let mut out = vec![0; MAX_RESULT_BYTES];
    let mut revision = 0u64;
    let mut pinned = [0; 32];
    for offset in (0..state.len()).step_by(MAX_CHUNK_BYTES) {
        let mut payload = revision.to_be_bytes().to_vec();
        payload.extend_from_slice(&pinned);
        payload.extend_from_slice(&(offset as u32).to_be_bytes());
        payload.extend_from_slice(&(MAX_CHUNK_BYTES as u16).to_be_bytes());
        let n = read_state_chunk(&state, &payload, &mut out).unwrap();
        let chunk = codec::decode_chunk_response(&out[..n]).unwrap();
        revision = chunk.revision;
        pinned = chunk.digest.bytes();
        capture.accept(&proof, &out[..n]).unwrap();
    }
    let (captured, facts) = capture.finish().unwrap();
    assert_eq!(captured, state);
    let finality = FinalityEvidence {
        native_state_root: proof.native_state_root,
        block_height: proof.execution_height,
        block_hash: proof.block_hash,
        app_hash: proof.native_state_root,
        store_proof_digest: digest(0xdd),
        rank,
    };
    let head = bind_snapshot(captured, &facts, &finality, 1_700_000_000_000).unwrap();
    let mut wire = vec![0; layerx_programs_ai_market::queries::BINDING_MAX_BYTES];
    let n = head.encode(&mut wire).unwrap();
    let head = SnapshotBinding::decode(&wire[..n]).unwrap();
    head.verify_capture(captured, &facts).unwrap();
    let body = json!({
        "request_id": request, "root": root, "state": hex::encode(captured),
        "native_state_root": hex::encode(head.native_state_root.bytes()),
        "observed_sequence": head.observed_sequence, "execution_height": head.execution_height,
        "batch_id": hex::encode(head.batch_id.bytes()), "commit": hex::encode(head.block_hash.bytes()),
        "block_height": head.block_height, "app_hash": hex::encode(head.app_hash.bytes()),
        "store_proof_digest": hex::encode(head.store_proof_digest.bytes()), "rank": head.rank,
        "publication_time_ms": head.publication_time_ms,
    });
    (body, head)
}
