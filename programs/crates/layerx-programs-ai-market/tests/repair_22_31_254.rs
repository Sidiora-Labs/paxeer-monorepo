//! Task 22.31.254 / audit 22.30.123, finding 001 (config-drift).
//! Proofs: finding_001_packaging_uses_chain_125_and_refuses_foreign_settings;
//! finding_001_request_state_event_and_header_share_chain_125;
//! finding_001_foreign_requests_and_state_refuse_without_writes.
//! Wiring request (domain 13): Makefile PAXAI_CHAIN_DOMAIN must use the chain-125
//! golden 62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6.
//! Wiring request (domain 13): tests/ci/test_paxai_chain_domain.py must derive its
//! golden from chain 125 and retain the packaging override checks.
//! Wiring request (domain 13): regenerate build/paxai-host-boundary guest/manifest
//! through the real deployment build after the Makefile domain is wired to 125.
//! Baseline: base 7fc896febbb27d599c34aa9b6e0a81d4a7905759 already defaults the
//! crate to chain 125; Makefile/CI still contain chain 229. No baseline tests ran:
//! the ordered gate rules require complete implementation before the only verify.
//! Qualification revision, command, exit and private log are recorded by hf-done.

use layerx_programs_ai_market::{
    codec::{self, Envelope, ResultStatus},
    deployment_chain_domain, deployment_chain_domain_from_setting,
    dispatch::{self, Buffers, Operation, Routed},
    errors::WRONG_DOMAIN,
    merged_chain_domain,
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries,
    registry::derive_rewards_account,
    registry_ops::{CallContext, PolicySection},
    state::{self, Section},
    AssetId, Authentication, ChainDomain, CodecResult, Digest32, Presence, PrincipalId, ProgramId,
    RequestId, RubricDigest, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES,
    MAX_STATE_BYTES, MERGED_CHAIN_DOMAIN, MERGED_CHAIN_ID,
};
use sha2::{Digest, Sha256};

const PAXEER_DOMAIN: &str = "62b6b14734fa769c3e0fbc94de495a3d038be49a2524b1c071d75d0a4eed71f6";
const RETIRED_DOMAIN: &str = "b69143b1aa0c360367c299a2a47bbbfe3546ecbdc0a78c640770d49d977f038d";

fn context(chain: ChainDomain) -> CodecResult<CallContext> {
    Ok(CallContext {
        chain,
        program: ProgramId::new([11; 32])?,
        principal: PrincipalId::new([12; 32])?,
        height: 100,
    })
}

fn request(ctx: &CallContext, operation: Operation, payload: &[u8]) -> CodecResult<Vec<u8>> {
    let envelope = Envelope {
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
    let n = codec::encode_envelope(&envelope, &mut bytes)?;
    bytes.truncate(n);
    Ok(bytes)
}

fn create_payload(ctx: &CallContext) -> CodecResult<Vec<u8>> {
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
    Ok(payload)
}

struct CallOutput {
    routed: Routed,
    next: Vec<u8>,
    event: Vec<u8>,
    result: Vec<u8>,
}

fn route(ctx: &CallContext, bytes: &[u8], current: Option<&[u8]>) -> CodecResult<CallOutput> {
    let mut next = vec![0xa5; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0xa5; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let routed = dispatch::route(
        ctx,
        bytes,
        current,
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    )?;
    let result_len = match routed {
        Routed::Applied {
            state_len,
            event_len,
            result_len,
            ..
        } => {
            next.truncate(state_len);
            event.truncate(event_len);
            result_len
        }
        Routed::Unchanged { result_len } | Routed::Refused { result_len } => {
            assert!(next.iter().all(|b| *b == 0xa5));
            assert!(event.iter().all(|b| *b == 0xa5));
            result_len
        }
    };
    result.truncate(result_len);
    Ok(CallOutput {
        routed,
        next,
        event,
        result,
    })
}

fn create(ctx: &CallContext) -> CodecResult<CallOutput> {
    let bytes = request(ctx, dispatch::CREATE, &create_payload(ctx)?)?;
    let output = route(ctx, &bytes, None)?;
    assert!(matches!(
        output.routed,
        Routed::Applied {
            operation: dispatch::CREATE,
            revision: 1,
            ..
        }
    ));
    let result = codec::decode_result(&output.result)?;
    assert_eq!(result.status, ResultStatus::Ok);
    assert_eq!(result.error, None);
    assert_eq!(
        result.request,
        Presence::Present(codec::decode_envelope(&bytes)?.request_digest()?)
    );
    Ok(output)
}

#[test]
fn finding_001_packaging_uses_chain_125_and_refuses_foreign_settings() -> CodecResult<()> {
    assert_eq!(MERGED_CHAIN_ID, 125);
    let mut hash = Sha256::new();
    hash.update(b"PAXAI/chain-domain/v1");
    hash.update(125u64.to_be_bytes());
    let golden: [u8; 32] = hash.finalize().into();
    assert_eq!(golden, queries::parse_hex32(PAXEER_DOMAIN)?);
    assert_eq!(MERGED_CHAIN_DOMAIN, golden);
    assert_eq!(merged_chain_domain(125), golden);
    assert_eq!(deployment_chain_domain()?.bytes(), golden);
    assert_eq!(deployment_chain_domain_from_setting(None)?.bytes(), golden);
    assert_eq!(
        deployment_chain_domain_from_setting(Some(PAXEER_DOMAIN))?.bytes(),
        golden
    );
    assert_eq!(
        merged_chain_domain(229),
        queries::parse_hex32(RETIRED_DOMAIN)?
    );
    for invalid in [
        RETIRED_DOMAIN.to_owned(),
        "00".repeat(32),
        "aa".repeat(32),
        String::new(),
        PAXEER_DOMAIN[..63].to_owned(),
        format!("{PAXEER_DOMAIN}0"),
        PAXEER_DOMAIN.to_uppercase(),
        format!(" {PAXEER_DOMAIN}"),
        format!("{PAXEER_DOMAIN}\n"),
    ] {
        assert_eq!(
            deployment_chain_domain_from_setting(Some(&invalid)),
            Err(WRONG_DOMAIN)
        );
    }
    Ok(())
}

#[test]
fn finding_001_request_state_event_and_header_share_chain_125() -> CodecResult<()> {
    let ctx = context(deployment_chain_domain()?)?;
    let local = create(&ctx)?;
    let market = codec::derive_market(ctx.chain, ctx.program)?;
    let mut hash = Sha256::new();
    hash.update(b"PAXAI/market/v1\0");
    hash.update(queries::parse_hex32(PAXEER_DOMAIN)?);
    hash.update(ctx.program.as_bytes());
    let expected_market: [u8; 32] = hash.finalize().into();
    assert_eq!(market.bytes(), expected_market);
    let shared = state::decode_shared_state(&local.next)?;
    let policy = PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
    assert_eq!(policy.header.deployment_chain_domain, ctx.chain);
    assert_eq!(policy.header.program_id, ctx.program);
    assert_eq!(policy.header.market_id, market);
    let (_, event, _) = codec::decode_event_frame(b"PAXAI/v1/CREATE", &local.event)?;
    assert_eq!(event.market, market);
    assert_eq!(
        event.request,
        match codec::decode_result(&local.result)?.request {
            Presence::Present(request) => request,
            Presence::Absent => panic!("CREATE result must bind its request"),
        }
    );

    let bytes = request(&ctx, dispatch::READ_HEADER, &[])?;
    let decoded = codec::decode_envelope(&bytes)?;
    assert_eq!(
        decoded.envelope.chain.bytes(),
        queries::parse_hex32(PAXEER_DOMAIN)?
    );
    assert_eq!(decoded.envelope.market, market);
    let mut hash = Sha256::new();
    hash.update(b"PAXAI/request/v1\0");
    hash.update(decoded.unsigned_bytes());
    let expected_request: [u8; 32] = hash.finalize().into();
    assert_eq!(decoded.request_digest()?.bytes(), expected_request);
    let stored = local.next.clone();
    let read = route(&ctx, &bytes, Some(&stored))?;
    assert!(matches!(read.routed, Routed::Unchanged { .. }));
    let result = codec::decode_result(&read.result)?;
    assert_eq!(result.status, ResultStatus::Ok);
    assert_eq!(result.error, None);
    assert_eq!(result.request, Presence::Present(decoded.request_digest()?));
    let header = codec::decode_read_header(result.payload)?;
    assert_eq!(header.chain, ctx.chain);
    assert_eq!(header.program, ctx.program);
    assert_eq!(header.market, market);
    assert_eq!(header.digest, codec::state_digest(&stored)?);
    assert_eq!(header.revision, 1);
    assert_eq!(
        usize::try_from(header.total_bytes).expect("state length fits usize"),
        stored.len()
    );
    assert_eq!(
        header,
        queries::read_header(&stored, ctx.chain, ctx.program)?
    );
    assert_eq!(stored, local.next);
    Ok(())
}

#[test]
fn finding_001_foreign_requests_and_state_refuse_without_writes() -> CodecResult<()> {
    let ctx = context(deployment_chain_domain()?)?;
    let local = create(&ctx)?.next;
    let local_request = request(&ctx, dispatch::READ_HEADER, &[])?;
    let local_digest = codec::decode_envelope(&local_request)?.request_digest()?;
    for chain_id in [229, 1, 230] {
        let foreign_ctx = context(ChainDomain::new(merged_chain_domain(chain_id))?)?;
        let foreign_state = create(&foreign_ctx)?.next;
        let foreign_request = request(&foreign_ctx, dispatch::READ_HEADER, &[])?;
        let foreign_digest = codec::decode_envelope(&foreign_request)?.request_digest()?;
        assert_ne!(foreign_digest, local_digest);
        let mut domain_only = codec::decode_envelope(&local_request)?.envelope;
        domain_only.chain = foreign_ctx.chain;
        let mut domain_only_request = vec![0; MAX_ENVELOPE_BYTES];
        let n = codec::encode_envelope(&domain_only, &mut domain_only_request)?;
        domain_only_request.truncate(n);
        assert_ne!(
            codec::decode_envelope(&domain_only_request)?.request_digest()?,
            local_digest
        );
        assert_ne!(
            codec::derive_market(foreign_ctx.chain, ctx.program)?,
            codec::derive_market(ctx.chain, ctx.program)?
        );
        assert_ne!(
            codec::state_digest(&foreign_state)?,
            codec::state_digest(&local)?
        );
        assert_eq!(
            queries::read_header(&foreign_state, ctx.chain, ctx.program),
            Err(WRONG_DOMAIN)
        );
        assert_eq!(
            queries::read_header(&local, foreign_ctx.chain, ctx.program),
            Err(WRONG_DOMAIN)
        );
        let foreign_create = request(
            &foreign_ctx,
            dispatch::CREATE,
            &create_payload(&foreign_ctx)?,
        )?;
        for (bytes, current) in [
            (domain_only_request.as_slice(), Some(local.as_slice())),
            (foreign_request.as_slice(), Some(local.as_slice())),
            (local_request.as_slice(), Some(foreign_state.as_slice())),
            (foreign_request.as_slice(), Some(foreign_state.as_slice())),
            (foreign_create.as_slice(), None),
        ] {
            let original = current.map(<[u8]>::to_vec);
            let output = route(&ctx, bytes, current)?;
            assert!(matches!(output.routed, Routed::Refused { .. }));
            let result = codec::decode_result(&output.result)?;
            assert_eq!(result.status, ResultStatus::Error);
            assert_eq!(result.error, Some(WRONG_DOMAIN));
            assert_eq!(
                result.request,
                Presence::Present(codec::decode_envelope(bytes)?.request_digest()?)
            );
            assert_eq!(current, original.as_deref());
        }
    }
    Ok(())
}
