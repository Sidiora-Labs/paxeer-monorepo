//! Proves: `dispatch::route` / `ReadProof` / `FinalityEvidence` -> `SnapshotBinding::encode` -> `SnapshotBinding::decode` / `verify_capture` / `finalized_head`.
//!
//! CREATE and `ProgramRead` run through the real guest dispatcher. The transport is the
//! public canonical binding consumed by layerx-client::ai_market::decode_snapshot_binding
//! and the saved-binding recovery in `hosted/indexer/src/ai_market.rs::rebind`. Retention's
//! `CommitFinality::of` and `webhooks::ai_market::certify` use the same `finalized_head` check.
//! The guest anchor has no durable state or cache: both observations below issue fresh
//! `ProgramReads` and decode fresh bytes. Host read and finality facts are explicit inputs;
//! this codec proof does not establish their cryptographic authority.
//! Audit ai-real-consumer-proof is covered by `repair_22_33_233.rs`, which compares
//! fresh guest captures with the hosted consumer's recorded promotion and recovery.
//!
//! Repairs: `queries.rs::bind_snapshot` promoted every evidence rank to 4 and failed to
//! check capture size/count; the client decoder now shares the canonical guest decoder.
//! ai-artifacts::{main.rs::state_head,retention.rs::CommitFinality::of} and
//! `tests/artifact_recovery.rs::snapshot` used removed pre-V2 fields. `Indexer::rebind`
//! reconstructed app/root evidence instead of decoding the exact saved V2 binding;
//! `webhooks::certify` now checks the same committed head as retention.
//!
//! Out-of-scope contradiction: `tests/view_binding.rs::a02_v2_finality_binds_block_commit_of_the_same_root`
//! expects `bind_snapshot` to upgrade ranks 0, 2 and even 255 to 4. That assertion is left
//! intact; the expected consumer behavior proved here is refusal of unfinalized evidence.
use layerx_programs_ai_market::{
    codec::{self, Envelope, ResultStatus},
    dispatch::{self, Buffers, Operation, Routed},
    errors::{BAD_VERSION, CAPACITY, NON_CANONICAL, UNAUTHORIZED},
    policy::{PolicyCommitments, TaskPolicyV1, TASK_POLICY_BYTES},
    queries::{
        bind_snapshot, CaptureFacts, FinalityEvidence, QueryError, ReadProof, SnapshotBinding,
        StateCapture, BINDING_MAX_BYTES, FINALIZED_RANK,
    },
    registry::derive_rewards_account,
    registry_ops::CallContext,
    state,
    types::{
        AssetId, Authentication, ChainDomain, Digest32, Presence, PrincipalId, ProgramId,
        RequestId, RosterDigest, RubricDigest,
    },
    MAX_CHUNK_BYTES, MAX_ENVELOPE_BYTES, MAX_EVENT_BYTES, MAX_RESULT_BYTES, MAX_STATE_BYTES,
};
use std::fmt::Debug;

const OWNER: [u8; 32] = [12; 32];

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("real path refused: {error:?}"))
}
fn digest(byte: u8) -> Digest32 {
    checked(Digest32::new([byte; 32]))
}
fn context() -> CallContext {
    CallContext {
        chain: checked(ChainDomain::new([10; 32])),
        program: checked(ProgramId::new([11; 32])),
        principal: checked(PrincipalId::new(OWNER)),
        height: 1010,
    }
}

fn envelope(operation: Operation, payload: &[u8], actor: [u8; 32]) -> Vec<u8> {
    let ctx = context();
    let read = operation == dispatch::READ_STATE_CHUNK;
    let e = Envelope {
        operation,
        chain: ctx.chain,
        program: ctx.program,
        market: checked(codec::derive_market(ctx.chain, ctx.program)),
        actor: checked(PrincipalId::new(actor)),
        epoch: 0,
        config: u64::from(!read),
        roster: Presence::Absent,
        sequence: u64::from(!read),
        expiry: 1_000_000,
        request: checked(RequestId::new([1; 32])),
        payload,
        authentication: Authentication::Native,
    };
    let mut bytes = vec![0; MAX_ENVELOPE_BYTES];
    let n = checked(codec::encode_envelope(&e, &mut bytes));
    bytes.truncate(n);
    bytes
}

/// Produces state by the same CREATE dispatch that the native guest entrypoint calls.
fn producer_state() -> Vec<u8> {
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
    let rewards = checked(derive_rewards_account(
        context().program,
        checked(AssetId::new([13; 32])),
    ));
    let mut payload = OWNER.to_vec();
    payload.extend_from_slice(&[13; 32]);
    payload.extend_from_slice(rewards.as_bytes());
    payload.extend_from_slice(&[14; 32]);
    payload.push(0); // absent treasury
    payload.extend_from_slice(&policy_bytes);
    payload.extend_from_slice(&[16; 32]);
    let mut next = vec![0; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let routed = checked(dispatch::route(
        &context(),
        &envelope(dispatch::CREATE, &payload, OWNER),
        None,
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    ));
    let Routed::Applied {
        state_len,
        result_len,
        event_len,
        revision,
        ..
    } = routed
    else {
        panic!("CREATE did not produce state: {routed:?}");
    };
    assert!(event_len > 0);
    assert_eq!(
        checked(codec::decode_result(&result[..result_len])).status,
        ResultStatus::Ok
    );
    next.truncate(state_len);
    assert_eq!(
        checked(state::decode_shared_state(&next)).revision,
        revision
    );
    next
}

fn read_proof() -> ReadProof {
    ReadProof {
        chain: context().chain,
        program: context().program,
        native_state_root: digest(0xa1),
        observed_sequence: 77,
        execution_height: context().height,
        block_hash: digest(0xbb),
    }
}
fn finality(rank: u8) -> FinalityEvidence {
    let proof = read_proof();
    FinalityEvidence {
        native_state_root: proof.native_state_root,
        block_height: proof.execution_height,
        block_hash: proof.block_hash,
        app_hash: proof.native_state_root,
        store_proof_digest: digest(0xdd),
        rank,
    }
}

/// Fresh guest reads, common application-result framing and actual `StateCapture` decode.
fn capture(state: &[u8]) -> (Vec<u8>, CaptureFacts) {
    let mut buffer = vec![0; MAX_STATE_BYTES];
    let mut capture = StateCapture::new(&mut buffer);
    let mut pin = None;
    for offset in (0..state.len()).step_by(MAX_CHUNK_BYTES) {
        let (revision, digest) = pin.unwrap_or((0u64, [0; 32]));
        let mut payload = revision.to_be_bytes().to_vec();
        payload.extend_from_slice(&digest);
        payload.extend_from_slice(&checked(u32::try_from(offset)).to_be_bytes());
        payload.extend_from_slice(&checked(u16::try_from(MAX_CHUNK_BYTES)).to_be_bytes());
        let (body, status) = guest_read(state, &payload, OWNER);
        assert_eq!(status, ResultStatus::Ok);
        let chunk = checked(codec::decode_chunk_response(&body));
        pin = Some((chunk.revision, chunk.digest.bytes()));
        checked(capture.accept(&read_proof(), &body));
    }
    let (bytes, facts) = checked(capture.finish());
    (bytes.to_vec(), facts)
}

fn guest_read(state: &[u8], payload: &[u8], actor: [u8; 32]) -> (Vec<u8>, ResultStatus) {
    let mut next = vec![0xa5; MAX_STATE_BYTES];
    let mut scratch = vec![0; dispatch::SCRATCH_BYTES];
    let mut event = vec![0xa5; MAX_EVENT_BYTES];
    let mut result = vec![0; MAX_RESULT_BYTES];
    let routed = checked(dispatch::route(
        &context(),
        &envelope(dispatch::READ_STATE_CHUNK, payload, actor),
        Some(state),
        Buffers {
            next: &mut next,
            scratch: &mut scratch,
            event: &mut event,
            result: &mut result,
        },
    ));
    let result_len = match routed {
        Routed::Unchanged { result_len } | Routed::Refused { result_len } => result_len,
        Routed::Applied { .. } => panic!("ProgramRead mutated state"),
    };
    assert!(next.iter().all(|byte| *byte == 0xa5));
    assert!(event.iter().all(|byte| *byte == 0xa5));
    let result = checked(codec::decode_result(&result[..result_len]));
    if actor != OWNER {
        assert_eq!(result.error, Some(UNAUTHORIZED));
    }
    (result.payload.to_vec(), result.status)
}
fn wire(binding: &SnapshotBinding) -> Vec<u8> {
    let mut bytes = vec![0; BINDING_MAX_BYTES];
    let n = checked(binding.encode(&mut bytes));
    assert!(n <= BINDING_MAX_BYTES);
    bytes.truncate(n);
    bytes
}

#[test]
fn proof_ai_market_guest_observes_fresh_producer_twice_and_refuses_wrong_actor() {
    let state = producer_state();
    let original = state.clone();
    let mut previous = None;
    for publication_time_ms in [1000, 2000] {
        let (captured, facts) = capture(&state);
        assert_eq!(captured, original);
        assert_eq!(facts.proof, read_proof());
        assert_eq!(facts.chunks, state.len().div_ceil(MAX_CHUNK_BYTES));
        let binding = checked(bind_snapshot(
            &captured,
            &facts,
            &finality(FINALIZED_RANK),
            publication_time_ms,
        ));
        // This is the shared decoder called by the client and by indexer recovery.
        let consumer = checked(SnapshotBinding::decode(&wire(&binding)));
        assert_eq!(consumer, binding);
        checked(consumer.verify_capture(&captured, &facts));
        checked(consumer.require_finalized());
        assert_eq!(
            checked(consumer.finalized_head()),
            (context().height, read_proof().block_hash)
        );
        assert_eq!(
            consumer.state_digest,
            checked(codec::state_digest(&original))
        );
        assert_eq!(
            consumer.market,
            checked(codec::derive_market(context().chain, context().program))
        );
        assert_eq!(consumer.publication_time_ms, publication_time_ms);
        if let Some((id, bytes)) = previous {
            assert_eq!(checked(consumer.snapshot_id()), id);
            assert_ne!(wire(&consumer), bytes); // second response was freshly consumed
        }
        previous = Some((checked(consumer.snapshot_id()), wire(&consumer)));
    }
    let mut payload = vec![0; 44];
    payload.extend_from_slice(&checked(u16::try_from(MAX_CHUNK_BYTES)).to_be_bytes());
    assert_eq!(
        guest_read(&state, &payload, [99; 32]).1,
        ResultStatus::Error
    );
    assert_eq!(state, original);
}

#[test]
fn proof_ai_market_guest_refuses_malformed_bytes_at_the_consumer_decoder() {
    let state = producer_state();
    let (_, facts) = capture(&state);
    let binding = checked(bind_snapshot(
        &state,
        &facts,
        &finality(FINALIZED_RANK),
        9000,
    ));
    // Cover both presence-dependent lengths, including the exact public capacity bound.
    for binding in [
        binding,
        SnapshotBinding {
            epoch: Presence::Present(1),
            roster: Presence::Present(checked(RosterDigest::new([18; 32]))),
            ..binding
        },
    ] {
        let bytes = wire(&binding);
        assert_eq!(checked(SnapshotBinding::decode(&bytes)), binding);
        if matches!(binding.epoch, Presence::Present(_)) {
            assert_eq!(bytes.len(), BINDING_MAX_BYTES);
        }
        assert_eq!(binding.encode(&mut vec![0; bytes.len() - 1]), Err(CAPACITY));
        for end in 0..bytes.len() {
            assert!(
                SnapshotBinding::decode(&bytes[..end]).is_err(),
                "truncated at {end}"
            );
        }
        let mut malformed = bytes.clone();
        malformed.push(0);
        assert_eq!(SnapshotBinding::decode(&malformed), Err(NON_CANONICAL));
        let mut malformed = bytes.clone();
        malformed[1] = 2;
        assert_eq!(SnapshotBinding::decode(&malformed), Err(BAD_VERSION));
        let mut malformed = bytes.clone();
        let last = malformed.len() - 1;
        malformed[last] = 1;
        assert_eq!(SnapshotBinding::decode(&malformed), Err(NON_CANONICAL));
        let mut malformed = bytes.clone();
        let rank_offset = malformed.len() - 17;
        malformed[rank_offset] = FINALIZED_RANK + 1;
        assert_eq!(SnapshotBinding::decode(&malformed), Err(NON_CANONICAL));
        assert_eq!(checked(SnapshotBinding::decode(&bytes)), binding);
    }
    let mut payload = vec![0; 44];
    payload.extend_from_slice(&checked(u16::try_from(MAX_CHUNK_BYTES)).to_be_bytes());
    let (body, status) = guest_read(&state, &payload, OWNER);
    assert_eq!(status, ResultStatus::Ok);
    let mut buffer = vec![0; MAX_STATE_BYTES];
    let mut consumer = StateCapture::new(&mut buffer);
    assert!(consumer
        .accept(&read_proof(), &body[..body.len() - 1])
        .is_err());
    assert_eq!(
        consumer.accept(&read_proof(), &body),
        Err(QueryError::IntegrityFailure)
    );
    assert_eq!(consumer.finish(), Err(QueryError::IntegrityFailure));
}

#[test]
fn proof_ai_market_guest_never_promotes_unfinalized_or_mismatched_evidence() {
    let state = producer_state();
    let (_, facts) = capture(&state);
    for rank in 0..FINALIZED_RANK {
        let binding = checked(bind_snapshot(&state, &facts, &finality(rank), 9000));
        let consumer = checked(SnapshotBinding::decode(&wire(&binding)));
        assert_eq!(consumer.rank, rank);
        assert_eq!(
            consumer.require_finalized(),
            Err(QueryError::FinalityUnavailable)
        );
        assert_eq!(
            consumer.finalized_head(),
            Err(QueryError::FinalityUnavailable)
        );
    }
    for rank in [FINALIZED_RANK + 1, u8::MAX] {
        assert_eq!(
            bind_snapshot(&state, &facts, &finality(rank), 9000),
            Err(QueryError::Application(NON_CANONICAL))
        );
    }
    let evidence = finality(FINALIZED_RANK);
    let binding = checked(bind_snapshot(&state, &facts, &evidence, 9000));
    {
        let malformed = FinalityEvidence {
            native_state_root: digest(98),
            ..evidence
        };
        assert_eq!(
            bind_snapshot(&state, &facts, &malformed, 9000),
            Err(QueryError::BindingMismatch)
        );
    }
    for malformed in [
        CaptureFacts {
            total_bytes: facts.total_bytes + 1,
            ..facts
        },
        CaptureFacts {
            chunks: facts.chunks + 1,
            ..facts
        },
        CaptureFacts {
            revision: facts.revision + 1,
            ..facts
        },
    ] {
        assert_eq!(
            bind_snapshot(&state, &malformed, &evidence, 9000),
            Err(QueryError::BindingMismatch)
        );
        assert_eq!(
            binding.verify_capture(&state, &malformed),
            Err(QueryError::BindingMismatch)
        );
    }
    for malformed in [
        SnapshotBinding {
            block_height: binding.block_height + 1,
            ..binding
        },
        SnapshotBinding {
            block_hash: digest(98),
            ..binding
        },
        SnapshotBinding {
            app_hash: digest(98),
            ..binding
        },
        SnapshotBinding {
            execution_height: 0,
            ..binding
        },
    ] {
        let consumer = checked(SnapshotBinding::decode(&wire(&malformed)));
        assert_eq!(consumer.finalized_head(), Err(QueryError::BindingMismatch));
    }
    let changed = SnapshotBinding {
        observed_sequence: binding.observed_sequence + 1,
        ..binding
    };
    assert_eq!(
        changed.verify_capture(&state, &facts),
        Err(QueryError::BindingMismatch)
    );
}

#[test]
fn proof_ai_market_guest_saved_binding_reopens_with_exact_v2_evidence() {
    use std::fs::{self, File};
    use std::io::Write;
    use std::sync::atomic::{AtomicU64, Ordering};
    static NEXT: AtomicU64 = AtomicU64::new(0);
    struct Saved(std::path::PathBuf);
    impl Drop for Saved {
        fn drop(&mut self) {
            checked(fs::remove_dir_all(&self.0));
        }
    }
    let saved = Saved(std::env::temp_dir().join(format!(
        "proof-ai-market-guest-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    )));
    checked(fs::create_dir(&saved.0));
    let state = producer_state();
    let (_, facts) = capture(&state);
    let binding = checked(bind_snapshot(
        &state,
        &facts,
        &finality(FINALIZED_RANK),
        1234,
    ));
    let id = checked(binding.snapshot_id());
    // Close every writer before reopening the canonical saved values. The decoder and
    // capture verifier are exactly the functions used by indexer::ai_market::rebind.
    for (name, bytes) in [("state", state.clone()), ("binding", wire(&binding))] {
        let mut file = checked(File::create(saved.0.join(name)));
        checked(file.write_all(&bytes));
        checked(file.sync_all());
    }
    for _ in 0..2 {
        let reopened_state = checked(fs::read(saved.0.join("state")));
        let (_, reopened_facts) = capture(&reopened_state);
        let consumer = checked(SnapshotBinding::decode(&checked(fs::read(
            saved.0.join("binding"),
        ))));
        checked(consumer.verify_capture(&reopened_state, &reopened_facts));
        assert_eq!(consumer, binding);
        assert_eq!(checked(consumer.snapshot_id()), id);
        assert_eq!(
            checked(consumer.finalized_head()),
            (binding.block_height, binding.block_hash)
        );
        assert_eq!(consumer.store_proof_digest, binding.store_proof_digest);
        assert_eq!(consumer.app_hash, binding.app_hash);
    }
    let mut malformed = wire(&binding);
    let end = malformed.len() - 1;
    malformed[end] = 1;
    checked(fs::write(saved.0.join("binding"), malformed));
    assert_eq!(
        SnapshotBinding::decode(&checked(fs::read(saved.0.join("binding")))),
        Err(NON_CANONICAL)
    );
    // A successful earlier recovery cannot hide the corrupted saved bytes.
    checked(fs::write(saved.0.join("binding"), wire(&binding)));
    assert_eq!(
        checked(SnapshotBinding::decode(&checked(fs::read(
            saved.0.join("binding")
        )))),
        binding
    );
}
