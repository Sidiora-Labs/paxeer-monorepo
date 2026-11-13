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

const MERGED_CHAIN_ID: u64 = 125;
const MERGED_CHAIN_DOMAIN: [u8; 32] = [
    0x62, 0xb6, 0xb1, 0x47, 0x34, 0xfa, 0x76, 0x9c, 0x3e, 0x0f, 0xbc, 0x94, 0xde, 0x49, 0x5a, 0x3d,
    0x03, 0x8b, 0xe4, 0x9a, 0x25, 0x24, 0xb1, 0xc0, 0x71, 0xd7, 0x5d, 0x0a, 0x4e, 0xed, 0x71, 0xf6,
];

#[test]
fn chain_domain_is_derived_from_the_merged_chain_id() -> CodecResult<()> {
    assert_eq!(layerx_programs_ai_market::MERGED_CHAIN_ID, MERGED_CHAIN_ID);
    assert_eq!(
        layerx_programs_ai_market::MERGED_CHAIN_DOMAIN,
        MERGED_CHAIN_DOMAIN
    );
    assert_eq!(merged_chain_domain(MERGED_CHAIN_ID), MERGED_CHAIN_DOMAIN);
    for chain_id in [0, 1, MERGED_CHAIN_ID, 256, 0x0102_0304_0506_0708, u64::MAX] {
        let mut preimage = b"PAXAI/chain-domain/v1".to_vec();
        preimage.extend_from_slice(&chain_id.to_be_bytes());
        let expected: [u8; 32] = Sha256::digest(&preimage).into();
        assert_eq!(merged_chain_domain(chain_id), expected);
    }
    assert_ne!(merged_chain_domain(229), MERGED_CHAIN_DOMAIN);
    assert_ne!(merged_chain_domain(230), MERGED_CHAIN_DOMAIN);

    let golden_hex: String = MERGED_CHAIN_DOMAIN
        .iter()
        .fold(String::new(), |mut text, b| {
            use core::fmt::Write as _;
            write!(text, "{b:02x}").unwrap_or_else(|error| panic!("hex formatting: {error}"));
            text
        });
    let makefile_default = include_str!("../../../../Makefile")
        .lines()
        .find_map(|line| line.strip_prefix("PAXAI_CHAIN_DOMAIN ?= "));
    assert_eq!(makefile_default, Some(golden_hex.as_str()));
    let ci_golden = include_str!("../../../../tests/ci/test_paxai_chain_domain.py")
        .lines()
        .find_map(|line| line.strip_prefix("MERGED_CHAIN_DOMAIN = \""))
        .and_then(|value| value.strip_suffix('"'));
    assert_eq!(ci_golden, Some(golden_hex.as_str()));
    assert_eq!(
        deployment_chain_domain()?.bytes(),
        merged_chain_domain(MERGED_CHAIN_ID)
    );
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
