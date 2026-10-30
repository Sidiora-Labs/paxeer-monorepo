#[path = "../../layerx-programs-registry/tests/support/chain.rs"]
mod chain;

use chain::{ChainFixture, STATE_HEIGHT};
use layerx_programs::chain_proof::state::verify_state;
use layerx_programs_runtime::{AuthorizationContext, CapabilitySet, PrincipalId, ProgramId};
use layerx_programs_sandbox::{
    sandbox_state_key, Lease, LeaseActivity, LeaseId, LeaseLimits, LeaseRefusal, LeaseState,
    LeaseTransition, TransitionEvidence,
};
use sha2::{Digest, Sha256};

fn requested() -> Lease {
    Lease::request(
        LeaseId::new([1; 32]).unwrap_or_else(|error| panic!("lease id: {error}")),
        PrincipalId::new([2; 32]).unwrap_or_else(|error| panic!("tenant: {error}")),
        ProgramId::new([3; 32]).unwrap_or_else(|error| panic!("host: {error}")),
        [4; 32],
        [5; 32],
        100,
        LeaseLimits {
            cpu_fuel: 10,
            memory_bytes: 10,
            storage_read_bytes: 10,
            storage_write_bytes: 10,
            output_values: 10,
            output_bytes: 10,
            table_elements: 10,
            namespace_bytes: 10,
        },
        10,
        100,
    )
    .unwrap_or_else(|error| panic!("request: {error}"))
}

fn record(steps: usize) -> (Lease, LeaseTransition) {
    let lease = requested();
    let transitions = [
        (
            LeaseActivity::Request,
            LeaseState::Requested,
            LeaseState::Requested,
            10u64,
        ),
        (
            LeaseActivity::Fund,
            LeaseState::Requested,
            LeaseState::Funded,
            11,
        ),
        (
            LeaseActivity::Activate,
            LeaseState::Funded,
            LeaseState::Active,
            STATE_HEIGHT as u64,
        ),
    ];
    let mut bytes = lease
        .canonical_state_bytes()
        .unwrap_or_else(|error| panic!("encode request: {error}"));
    bytes.truncate(bytes.len() - 5);
    bytes.push(transitions[steps - 1].2 as u8);
    bytes.push(u8::try_from(steps).unwrap_or_else(|error| panic!("steps: {error}")));
    let mut latest = None;
    for (activity, from, to, height) in transitions.into_iter().take(steps) {
        let tx_hash: [u8; 32] = Sha256::digest([activity as u8, 0x13, 0x63]).into();
        let observation = if activity == LeaseActivity::Request {
            lease
                .request_binding_digest()
                .unwrap_or_else(|error| panic!("binding: {error}"))
        } else {
            [0; 32]
        };
        let transition = LeaseTransition {
            lease: lease.id(),
            tenant: lease.tenant(),
            activity,
            from,
            to,
            activity_id: tx_hash,
            usage_observation_digest: observation,
        };
        bytes.extend_from_slice(&[activity as u8, from as u8, to as u8]);
        bytes.extend_from_slice(&tx_hash);
        bytes.extend_from_slice(&observation);
        let mut receipt = Sha256::new();
        receipt.update(lease.id().bytes());
        receipt.update([activity as u8, from as u8, to as u8]);
        receipt.update(tx_hash);
        receipt.update(observation);
        receipt.update(height.to_be_bytes());
        let receipt_digest: [u8; 32] = receipt.finalize().into();
        bytes.extend_from_slice(&receipt_digest);
        bytes.extend_from_slice(&height.to_be_bytes());
        latest = Some(transition);
    }
    bytes.extend_from_slice(&[0, 0, 0]);
    (
        Lease::decode_state(&bytes).unwrap_or_else(|error| panic!("decode record: {error}")),
        latest.unwrap_or_else(|| panic!("missing transition")),
    )
}

#[test]
fn lease_transition_verifies_against_proven_lease_record() {
    let (funded, _) = record(2);
    let (active, transition) = record(3);
    assert_eq!(
        funded.history(),
        &active.history()[..funded.history().len()]
    );
    let canonical = active
        .canonical_state_bytes()
        .unwrap_or_else(|error| panic!("encode: {error}"));
    let key =
        sandbox_state_key(0, active.id().bytes()).unwrap_or_else(|error| panic!("key: {error}"));
    let usage_key = sandbox_state_key(1, active.id().bytes())
        .unwrap_or_else(|error| panic!("usage key: {error}"));
    let fixture = ChainFixture::new_in_store(
        "lxphost",
        "paxeer-lease-chain",
        &[[1; 32], [2; 32], [3; 32]],
        vec![
            (key.clone(), canonical.clone()),
            (usage_key.clone(), canonical.clone()),
        ],
    )
    .unwrap_or_else(|error| panic!("chain: {error:?}"));
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("head: {error:?}"));
    let ops = fixture
        .prove(&key)
        .unwrap_or_else(|error| panic!("proof: {error:?}"));
    let authorization = AuthorizationContext::new(active.tenant(), CapabilitySet::empty());
    let evidence = TransitionEvidence::verify_call(
        &head,
        &funded,
        transition,
        &authorization,
        &canonical,
        &ops,
    )
    .unwrap_or_else(|error| panic!("activate: {error}"));
    assert_eq!(active.state(), LeaseState::Active);
    assert_eq!(evidence.tx_hash(), transition.activity_id);
    assert_eq!(evidence.state_height(), STATE_HEIGHT as u64);
    assert_eq!(
        evidence.lease_digest(),
        active
            .state_digest()
            .unwrap_or_else(|error| panic!("digest: {error}"))
    );
    assert_eq!(
        evidence.lease_digest(),
        <[u8; 32]>::from(Sha256::digest(&canonical))
    );
    let mut advanced = funded.clone();
    advanced
        .transition(transition, evidence)
        .unwrap_or_else(|error| panic!("apply proven transition: {error}"));
    assert_eq!(advanced, active);
    assert_eq!(advanced.state_digest(), Ok(evidence.lease_digest()));

    let wrong_ops = fixture
        .prove(&usage_key)
        .unwrap_or_else(|error| panic!("wrong proof: {error:?}"));
    assert_eq!(
        TransitionEvidence::verify_call(
            &head,
            &funded,
            transition,
            &authorization,
            &canonical,
            &wrong_ops,
        ),
        Err(LeaseRefusal::InvalidCanonicalEvidence)
    );
    let mut changed = canonical.clone();
    changed[0] ^= 1;
    assert_eq!(
        TransitionEvidence::verify_call(&head, &funded, transition, &authorization, &changed, &ops,),
        Err(LeaseRefusal::InvalidCanonicalEvidence)
    );
    let mut wrong_tx = transition;
    wrong_tx.activity_id[0] ^= 1;
    assert_eq!(
        TransitionEvidence::verify_call(&head, &funded, wrong_tx, &authorization, &canonical, &ops,),
        Err(LeaseRefusal::ActivityReceiptMismatch)
    );
}

#[test]
fn lease_transition_verifies_only_against_mounted_lxphost_store() {
    let (funded, _) = record(2);
    let (active, transition) = record(3);
    assert_eq!(funded.state(), LeaseState::Funded);
    assert_eq!(active.state(), LeaseState::Active);
    let canonical = active
        .canonical_state_bytes()
        .unwrap_or_else(|error| panic!("encode: {error}"));
    let key =
        sandbox_state_key(0, active.id().bytes()).unwrap_or_else(|error| panic!("key: {error}"));
    let authorization = AuthorizationContext::new(funded.tenant(), CapabilitySet::empty());
    for store_name in ["lxphost", "layerx", "unmounted"] {
        let fixture = ChainFixture::new_in_store(
            store_name,
            "paxeer-lease-chain",
            &[[1; 32], [2; 32], [3; 32]],
            vec![(key.clone(), canonical.clone())],
        )
        .unwrap_or_else(|error| panic!("chain: {error:?}"));
        let head = fixture
            .verifier
            .verify_head(&fixture.header, &fixture.commit)
            .unwrap_or_else(|error| panic!("head: {error:?}"));
        let ops = fixture
            .prove(&key)
            .unwrap_or_else(|error| panic!("proof: {error:?}"));
        assert_eq!(
            verify_state(&head, store_name, &key, &canonical, &ops),
            Ok(STATE_HEIGHT)
        );
        let evidence = TransitionEvidence::verify_call(
            &head,
            &funded,
            transition,
            &authorization,
            &canonical,
            &ops,
        );
        if store_name == "lxphost" {
            let evidence = evidence.unwrap_or_else(|error| panic!("activate: {error}"));
            assert_eq!(evidence.state_height(), STATE_HEIGHT as u64);
            assert_eq!(evidence.tx_hash(), transition.activity_id);
            assert_eq!(
                evidence.lease_digest(),
                active
                    .state_digest()
                    .unwrap_or_else(|error| panic!("digest: {error}"))
            );
        } else {
            assert_eq!(evidence, Err(LeaseRefusal::InvalidCanonicalEvidence));
        }
    }
}

#[test]
fn lease_transition_refuses_regressed_state() {
    let (funded, _) = record(2);
    let (_, activate) = record(3);
    let canonical = funded
        .canonical_state_bytes()
        .unwrap_or_else(|error| panic!("encode: {error}"));
    let key =
        sandbox_state_key(0, funded.id().bytes()).unwrap_or_else(|error| panic!("key: {error}"));
    let fixture = ChainFixture::new_in_store(
        "lxphost",
        "paxeer-lease-chain",
        &[[1; 32]],
        vec![(key.clone(), canonical.clone())],
    )
    .unwrap_or_else(|error| panic!("chain: {error:?}"));
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("head: {error:?}"));
    let ops = fixture
        .prove(&key)
        .unwrap_or_else(|error| panic!("proof: {error:?}"));
    assert_eq!(
        verify_state(&head, "lxphost", &key, &canonical, &ops),
        Ok(STATE_HEIGHT)
    );
    assert_eq!(funded.state(), LeaseState::Funded);
    assert_eq!(
        TransitionEvidence::verify_call(
            &head,
            &funded,
            activate,
            &AuthorizationContext::new(funded.tenant(), CapabilitySet::empty()),
            &canonical,
            &ops,
        ),
        Err(LeaseRefusal::InvalidTransition)
    );
}
