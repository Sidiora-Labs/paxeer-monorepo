use layerx_programs_ai_market::{
    codec::{self, Envelope},
    dispatch::{self, Buffers, Routed},
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    registry::derive_rewards_account,
    registry_ops::CallContext,
    rewards::{FundRequest, FUNDING_POLICY_VERSION},
    types::*,
    CodecResult, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
    MERGED_CHAIN_DOMAIN,
};

pub const HEIGHT: u64 = 1002;
pub const AMOUNT: u128 = 100;
pub const PROGRAM: [u8; 32] = [11; 32];
pub const REFUND: [u8; 32] = [14; 32];

pub struct Fixture {
    pub create: Vec<u8>,
    pub fund: Vec<u8>,
    pub current: Vec<u8>,
    pub next: Vec<u8>,
    pub result: Vec<u8>,
    pub event: Vec<u8>,
    pub rewards: [u8; 32],
}

pub fn fixture(owner: [u8; 32], asset: [u8; 32]) -> CodecResult<Fixture> {
    let ctx = CallContext {
        chain: ChainDomain::new(MERGED_CHAIN_DOMAIN)?,
        program: ProgramId::new(PROGRAM)?,
        principal: PrincipalId::new(owner)?,
        height: HEIGHT,
    };
    let rewards = derive_rewards_account(ctx.program, AssetId::new(asset)?)?;
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
    let mut create_payload = owner.to_vec();
    create_payload.extend_from_slice(&asset);
    create_payload.extend_from_slice(rewards.as_bytes());
    create_payload.extend_from_slice(&REFUND);
    create_payload.push(0);
    create_payload.extend_from_slice(&policy_bytes);
    create_payload.extend_from_slice(&[16; 32]);
    let mut fund_payload = vec![0; 57];
    let n = FundRequest {
        amount: AMOUNT,
        refund_recipient: AccountId::new(REFUND)?,
        policy_version: FUNDING_POLICY_VERSION,
        consent: true,
    }
    .encode(&mut fund_payload)?;
    fund_payload.truncate(n);
    let envelope = |operation, sequence, payload: &[u8]| -> CodecResult<Vec<u8>> {
        let mut encoded = vec![0; MAX_ENVELOPE_BYTES];
        let n = codec::encode_envelope(
            &Envelope {
                operation,
                chain: ctx.chain,
                program: ctx.program,
                market: codec::derive_market(ctx.chain, ctx.program)?,
                actor: ctx.principal,
                epoch: 0,
                config: 1,
                roster: Presence::Absent,
                sequence,
                expiry: HEIGHT + 1000,
                request: RequestId::new(
                    [u8::try_from(sequence)
                        .map_err(|_| layerx_programs_ai_market::errors::ARITHMETIC)?;
                        32],
                )?,
                payload,
                authentication: Authentication::Native,
            },
            &mut encoded,
        )?;
        encoded.truncate(n);
        Ok(encoded)
    };
    let create = envelope(dispatch::CREATE, 1, &create_payload)?;
    let fund = envelope(dispatch::FUND, 2, &fund_payload)?;
    let mut current = vec![0; MAX_STATE_BYTES];
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let Routed::Applied { state_len, .. } = dispatch::route(
        &ctx,
        &create,
        None,
        Buffers {
            next: &mut current,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?
    else {
        return Err(layerx_programs_ai_market::errors::NON_CANONICAL);
    };
    current.truncate(state_len);
    let Routed::Applied {
        state_len,
        event_len,
        result_len,
        ..
    } = dispatch::route(
        &ctx,
        &fund,
        Some(&current),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?
    else {
        return Err(layerx_programs_ai_market::errors::NON_CANONICAL);
    };
    next.truncate(state_len);
    event.truncate(event_len);
    result.truncate(result_len);
    Ok(Fixture {
        create,
        fund,
        current,
        next,
        event,
        result,
        rewards: rewards.bytes(),
    })
}
