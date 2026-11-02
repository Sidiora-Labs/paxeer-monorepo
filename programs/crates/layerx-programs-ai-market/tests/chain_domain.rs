use layerx_programs_ai_market::{
    codec::{self, Envelope},
    deployment_chain_domain,
    dispatch::{self, Buffers, Routed},
    errors::WRONG_DOMAIN,
    merged_chain_domain,
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::{derive_rewards_account, F01_SECTION_CAP},
    registry_ops::{self, CallContext, Outcome},
    state::{self, Section},
    AssetId, Authentication, ChainDomain, CodecResult, Digest32, Presence, PrincipalId, ProgramId,
    RequestId, RubricDigest, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES,
    MAX_STATE_BYTES,
};
use sha2::{Digest, Sha256};

const MERGED_CHAIN_ID: u64 = 229;
const MERGED_CHAIN_DOMAIN: [u8; 32] = [
    0xb6, 0x91, 0x43, 0xb1, 0xaa, 0x0c, 0x36, 0x03, 0x67, 0xc2, 0x99, 0xa2, 0xa4, 0x7b, 0xbb, 0xfe,
    0x35, 0x46, 0xec, 0xbd, 0xc0, 0xa7, 0x8c, 0x64, 0x07, 0x70, 0xd4, 0x9d, 0x97, 0x7f, 0x03, 0x8d,
];

#[test]
fn chain_domain_is_derived_from_the_merged_chain_id() -> CodecResult<()> {
    assert_eq!(merged_chain_domain(MERGED_CHAIN_ID), MERGED_CHAIN_DOMAIN);
    for chain_id in [0, 1, MERGED_CHAIN_ID, 256, 0x0102_0304_0506_0708, u64::MAX] {
        let mut preimage = b"PAXAI/chain-domain/v1".to_vec();
        preimage.extend_from_slice(&chain_id.to_be_bytes());
        let expected: [u8; 32] = Sha256::digest(&preimage).into();
        assert_eq!(merged_chain_domain(chain_id), expected);
    }
    assert_ne!(merged_chain_domain(230), MERGED_CHAIN_DOMAIN);

    let golden_hex: String = MERGED_CHAIN_DOMAIN
        .iter()
        .map(|b| format!("{b:02x}"))
        .collect();
    let makefile_default = include_str!("../../../../Makefile")
        .lines()
        .find_map(|line| line.strip_prefix("PAXAI_CHAIN_DOMAIN ?= "));
    assert_eq!(makefile_default, Some(golden_hex.as_str()));
    let ci_golden = include_str!("../../../../tests/ci/test_paxai_chain_domain.py")
        .lines()
        .find_map(|line| line.strip_prefix("MERGED_CHAIN_DOMAIN = \""))
        .and_then(|value| value.strip_suffix('"'));
    assert_eq!(ci_golden, Some(golden_hex.as_str()));
    assert_eq!(option_env!("PAXAI_CHAIN_DOMAIN"), Some(golden_hex.as_str()));
    assert_eq!(deployment_chain_domain()?.bytes(), MERGED_CHAIN_DOMAIN);
    Ok(())
}

fn envelope(
    ctx: &CallContext,
    operation: dispatch::Operation,
    payload: &[u8],
) -> CodecResult<Vec<u8>> {
    let value = Envelope {
        operation,
        chain: ctx.chain,
        program: ctx.program,
        market: codec::derive_market(ctx.chain, ctx.program)?,
        actor: ctx.principal,
        epoch: 0,
        config: match operation.metadata().boundary {
            dispatch::CallBoundary::Mutation => 1,
            dispatch::CallBoundary::ProgramRead => 0,
        },
        roster: Presence::Absent,
        sequence: match operation.metadata().sequence {
            dispatch::SequencePolicy::Role => 1,
            dispatch::SequencePolicy::ObjectLocal => 0,
        },
        expiry: 1000,
        request: RequestId::new([1; 32])?,
        payload,
        authentication: Authentication::Native,
    };
    let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
    let n = codec::encode_envelope(&value, &mut bytes)?;
    bytes.truncate(n);
    Ok(bytes)
}

fn created_state(ctx: &CallContext) -> CodecResult<Vec<u8>> {
    let policy = TaskPolicyV1::bounded_default(
        1,
        1,
        PolicyCommitments {
            model_artifact: Digest32::new([1; 32])?,
            dataset_artifact: [2; 32],
            benchmark_suite: Digest32::new([3; 32])?,
            rubric: RubricDigest::new([4; 32])?,
            task_schema: Digest32::new([5; 32])?,
            result_schema: Digest32::new([6; 32])?,
            service_terms: Digest32::new([7; 32])?,
        },
        100,
        1,
    )?;
    let mut policy_bytes = vec![0; TASK_POLICY_BYTES];
    policy.encode(&mut policy_bytes)?;
    let asset = AssetId::new([13; 32])?;
    let rewards = derive_rewards_account(ctx.program, asset)?;
    let mut payload = ctx.principal.bytes().to_vec();
    payload.extend_from_slice(asset.as_bytes());
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&[14; 32]);
    payload.push(0);
    payload.extend_from_slice(&policy_bytes);
    payload.extend_from_slice(&[16; 32]);
    let bytes = envelope(ctx, dispatch::CREATE, &payload)?;
    let validated = codec::decode_envelope(&bytes)?;
    let mut section = vec![0; F01_SECTION_CAP];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let Outcome::Applied { state, .. } =
        registry_ops::apply(ctx, None, &validated, &mut section, &mut event)?
    else {
        panic!("fresh CREATE must apply");
    };
    let mut bytes = vec![0; state.encoded_len()?];
    let mut control = vec![0; Section::Control.payload_cap()];
    let n = state::encode_shared_state(&state, &mut bytes, &mut control)?;
    assert_eq!(n, bytes.len());
    Ok(bytes)
}

#[test]
fn deployment_refuses_state_of_another_chain_domain() -> CodecResult<()> {
    let ctx = CallContext {
        chain: deployment_chain_domain()?,
        program: ProgramId::new([11; 32])?,
        principal: PrincipalId::new([12; 32])?,
        height: 100,
    };
    assert_eq!(ctx.chain.bytes(), MERGED_CHAIN_DOMAIN);
    let foreign_ctx = CallContext {
        chain: ChainDomain::new(merged_chain_domain(230))?,
        ..ctx
    };
    let local = created_state(&ctx)?;
    let foreign = created_state(&foreign_ctx)?;
    let request = envelope(&ctx, dispatch::READ_HEADER, &[])?;

    for (current, expected_error) in [(&local, None), (&foreign, Some(WRONG_DOMAIN))] {
        let original = current.clone();
        let mut next = vec![0xa5; MAX_STATE_BYTES];
        let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
        let mut event = vec![0xa5; MAX_EVENT_BYTES];
        let mut result = vec![0; MAX_RESULT_BYTES];
        let routed = dispatch::route(
            &ctx,
            &request,
            Some(current),
            Buffers {
                next: &mut next,
                scratch: &mut scratch,
                event: &mut event,
                result: &mut result,
            },
        )?;
        let n = match (routed, expected_error) {
            (Routed::Unchanged { result_len }, None)
            | (Routed::Refused { result_len }, Some(WRONG_DOMAIN)) => result_len,
            other => panic!("unexpected deployment outcome: {other:?}"),
        };
        assert_eq!(codec::decode_result(&result[..n])?.error, expected_error);
        assert_eq!(current, &original);
        assert!(next.iter().all(|b| *b == 0xa5));
        assert!(event.iter().all(|b| *b == 0xa5));
    }
    Ok(())
}
