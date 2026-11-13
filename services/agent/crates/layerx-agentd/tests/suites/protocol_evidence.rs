use layerx_agentd::boot::GateError;
use layerx_agentd::protocol_evidence::{
    CumulativeUseError, CumulativeUseWindow, EvidenceAuthority, ReceiptEvidenceError,
    StateEvidenceError, VerifierPolicyError,
};
use layerx_types::ids::Did;
use layerx_wire::limits::{LEGACY_PROTOCOL_VERSION, PROTOCOL_VERSION};

use crate::support::{StateHeaderIdentity, TestAuthorityPolicy, TestAuthorityRecord};

#[derive(Clone, Copy)]
struct PolicyIdentity {
    signing_seed: [u8; 32],
    protocol_version: u16,
    network_id: u32,
    epoch: u64,
    first_batch: u64,
    last_batch: u64,
    revoked_at_batch: Option<u64>,
    handshake_signing_seed: [u8; 32],
    handshake_batch: u64,
}

fn verifier(identity: PolicyIdentity) -> EvidenceAuthority {
    let mut records = vec![TestAuthorityRecord {
        signing_seed: identity.signing_seed,
        epoch: identity.epoch,
        first_batch: identity.first_batch,
        last_batch: identity.last_batch,
        revoked_at_batch: identity.revoked_at_batch,
    }];
    if identity.handshake_signing_seed != identity.signing_seed {
        records.push(TestAuthorityRecord {
            signing_seed: identity.handshake_signing_seed,
            epoch: identity.epoch,
            first_batch: identity.handshake_batch,
            last_batch: identity.handshake_batch,
            revoked_at_batch: None,
        });
    }
    crate::support::evidence_authority(TestAuthorityPolicy {
        protocol_version: identity.protocol_version,
        network_id: identity.network_id,
        records: &records,
        handshake_signing_seed: identity.handshake_signing_seed,
        handshake_batch: identity.handshake_batch,
    })
}

fn state(identity: StateHeaderIdentity) -> layerx_agentd::protocol_evidence::RawStateEvidence {
    crate::support::raw_state_leaf_with(b"canonical-state-value".to_vec(), 50, identity)
}

#[test]
fn cumulative_use_refuses_invalid_or_incomplete_windows_before_receipt_use() {
    let authority = crate::support::evidence_verifier();
    let actor = Did::new(b"did:layerx:cumulative")
        .unwrap_or_else(|error| panic!("cumulative DID: {error:?}"));
    assert!(matches!(
        authority.authenticate_cumulative_use(
            &actor,
            CumulativeUseWindow { first: 2, last: 1 },
            &[]
        ),
        Err(CumulativeUseError::InvalidWindow)
    ));
    assert!(matches!(
        authority.authenticate_cumulative_use(
            &actor,
            CumulativeUseWindow { first: 1, last: 1 },
            &[]
        ),
        Err(CumulativeUseError::IncompleteWindow)
    ));
}

#[test]
#[allow(
    deprecated,
    reason = "Compatibility coverage retains the archived checkpoint-finalised level."
)]
fn trusted_policy_issues_state_evidence_only_for_its_exact_identity() {
    let authority = crate::support::current_node::authority(8, 100, 199);
    let (protocol, checkpoint) = crate::support::current_node::committed_budget([9; 32], 25);
    let verified = lint_success(
        authority.verify_state(&protocol.evidence),
        "committed state evidence",
    );
    assert_eq!(
        verified.level(),
        layerx_types::verify::VerificationLevel::CHECKPOINT_FINALISED
    );
    assert_eq!(verified.observed_head_sequence(), 150);
    let authority = lint_success(
        authority.pin_committed_validators("125".to_owned(), checkpoint.validators.clone()),
        "pinned consensus validators",
    );
    let time = lint_success(
        authority.authenticate_committed_time(&checkpoint),
        "authenticated committed time",
    );
    assert_eq!(time.observed_core_ms(), 1_700_000_000_123);
    let mut altered = checkpoint.clone();
    altered.commit.votes[0].signature[0] ^= 1;
    assert!(authority.authenticate_committed_time(&altered).is_err());
}

#[test]
fn canonical_genesis_batch_zero_is_inside_a_bounded_trusted_range() {
    // The old batch-zero authority does not exist in the node's height-based
    // key contract: firstHeight must be positive. Preserve the zero boundary
    // coverage as an exact parser refusal, and prove the first valid window.
    let (endpoint, server) = crate::support::serve(vec![(
        "px_getReceiptKeys",
        serde_json::json!([]),
        Some(crate::support::current_node::keys(8, 0, 0)),
    )]);
    assert_eq!(
        endpoint.receipt_keys(),
        Err(layerx_client::sequencer::SequencerError::Malformed)
    );
    lint_success(server.join(), "genesis node server");
    let _authority = crate::support::current_node::authority(8, 1, 1);
}

#[test]
fn revocation_handover_selects_only_the_key_active_at_each_batch() {
    use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext, ReceiptError};
    let raw = crate::support::signed_transaction(2);
    let hash = layerx_crypto::evm_transaction::transaction_hash(&raw);
    for (first, last, seed, signing_seed, accepted) in [
        (100, 150, 8, 8, true),
        (150, 199, 9, 9, true),
        (100, 149, 8, 8, false),
        (151, 199, 8, 8, false),
        (150, 199, 9, 8, false),
    ] {
        let (view, _) = crate::support::sequencer_receipt_with_seed(&raw, true, signing_seed);
        let mut exchanges = vec![
            (
                "px_getReceiptKeys",
                serde_json::json!([]),
                Some(crate::support::current_node::keys(seed, first, last)),
            ),
            (
                "pax_sequencerReceipt",
                serde_json::json!([crate::support::rpc_hex(&hash)]),
                Some(view.clone()),
            ),
        ];
        if accepted {
            exchanges.push((
                "eth_getTransactionReceipt",
                serde_json::json!([crate::support::rpc_hex(&hash)]),
                Some(serde_json::Value::Null),
            ));
        }
        let (endpoint, server) = crate::support::serve(exchanges);
        let keys = lint_success(endpoint.receipt_keys(), "node key response");
        let observed = lookup_authenticated(
            &endpoint,
            hash,
            LookupContext {
                receipt_keys: &keys,
            },
        );
        if accepted {
            assert!(matches!(observed, Ok(Lookup::Verified(_))));
        } else {
            assert!(matches!(observed, Err(ReceiptError::Verification(_))));
        }
        lint_success(server.join(), "rotation node server");
    }
    let checkpoint =
        crate::support::current_node::signed_checkpoint(150, 1_700_000_000, 123_000_000);
    for seed in [8, 9] {
        let authority = lint_success(
            crate::support::current_node::authority(seed, 100, 199)
                .pin_committed_validators("125".to_owned(), checkpoint.validators.clone()),
            "consensus authority",
        );
        assert_eq!(
            lint_success(
                authority.authenticate_committed_time(&checkpoint),
                "committed chain independent of receipt rotation"
            )
            .through_sequence(),
            150
        );
    }
}

#[test]
fn a_caller_cannot_substitute_a_policy_after_the_daemon_handshake() {
    assert_eq!(
        crate::support::try_evidence_authority(TestAuthorityPolicy {
            protocol_version: PROTOCOL_VERSION,
            network_id: 42,
            records: &[TestAuthorityRecord {
                signing_seed: [0x4a; 32],
                epoch: 2,
                first_batch: 7,
                last_batch: 7,
                revoked_at_batch: None,
            }],
            handshake_signing_seed: [0x5a; 32],
            handshake_batch: 7,
        }),
        Err(GateError::Evidence(VerifierPolicyError::HandshakeKey))
    );
}

#[test]
fn an_attacker_signed_header_cannot_supply_its_own_trust_key() {
    let attacker = state(StateHeaderIdentity {
        signing_seed: [0x5a; 32],
        protocol_version: PROTOCOL_VERSION,
        network_id: 42,
        epoch: 2,
        batch_number: 7,
    });
    assert_eq!(
        verifier(PolicyIdentity {
            signing_seed: [0x4a; 32],
            protocol_version: PROTOCOL_VERSION,
            network_id: 42,
            epoch: 2,
            first_batch: 7,
            last_batch: 7,
            revoked_at_batch: None,
            handshake_signing_seed: [0x4a; 32],
            handshake_batch: 7,
        })
        .verify_state(&attacker),
        Err(StateEvidenceError::Policy(
            VerifierPolicyError::UnknownSequencer
        ))
    );
}

#[test]
fn cross_network_and_cross_protocol_headers_are_refused_before_signature_use() {
    let cross_network = state(StateHeaderIdentity {
        signing_seed: [0x4a; 32],
        protocol_version: PROTOCOL_VERSION,
        network_id: 43,
        epoch: 2,
        batch_number: 7,
    });
    assert_eq!(
        verifier(PolicyIdentity {
            signing_seed: [0x4a; 32],
            protocol_version: PROTOCOL_VERSION,
            network_id: 42,
            epoch: 2,
            first_batch: 7,
            last_batch: 7,
            revoked_at_batch: None,
            handshake_signing_seed: [0x4a; 32],
            handshake_batch: 7,
        })
        .verify_state(&cross_network),
        Err(StateEvidenceError::Policy(VerifierPolicyError::Network))
    );

    let cross_protocol = state(StateHeaderIdentity {
        signing_seed: [0x4a; 32],
        protocol_version: LEGACY_PROTOCOL_VERSION,
        network_id: 42,
        epoch: 2,
        batch_number: 7,
    });
    assert_eq!(
        verifier(PolicyIdentity {
            signing_seed: [0x4a; 32],
            protocol_version: PROTOCOL_VERSION,
            network_id: 42,
            epoch: 2,
            first_batch: 7,
            last_batch: 7,
            revoked_at_batch: None,
            handshake_signing_seed: [0x4a; 32],
            handshake_batch: 7,
        })
        .verify_state(&cross_protocol),
        Err(StateEvidenceError::Policy(
            VerifierPolicyError::ProtocolVersion
        ))
    );
}

#[test]
fn a_header_from_an_unconfigured_epoch_cannot_reuse_an_authorised_key() {
    let wrong_epoch = state(StateHeaderIdentity {
        signing_seed: [0x4a; 32],
        protocol_version: PROTOCOL_VERSION,
        network_id: 42,
        epoch: 3,
        batch_number: 7,
    });
    assert_eq!(
        verifier(PolicyIdentity {
            signing_seed: [0x4a; 32],
            protocol_version: PROTOCOL_VERSION,
            network_id: 42,
            epoch: 2,
            first_batch: 7,
            last_batch: 7,
            revoked_at_batch: None,
            handshake_signing_seed: [0x4a; 32],
            handshake_batch: 7,
        })
        .verify_state(&wrong_epoch),
        Err(StateEvidenceError::Policy(VerifierPolicyError::Epoch))
    );
}

#[test]
fn revoked_and_out_of_range_keys_cannot_issue_evidence() {
    let raw = state(StateHeaderIdentity {
        signing_seed: [0x4a; 32],
        protocol_version: PROTOCOL_VERSION,
        network_id: 42,
        epoch: 2,
        batch_number: 7,
    });
    assert_eq!(
        verifier(PolicyIdentity {
            signing_seed: [0x4a; 32],
            protocol_version: PROTOCOL_VERSION,
            network_id: 42,
            epoch: 2,
            first_batch: 7,
            last_batch: 7,
            revoked_at_batch: Some(7),
            handshake_signing_seed: [0x3a; 32],
            handshake_batch: 7,
        })
        .verify_state(&raw),
        Err(StateEvidenceError::Policy(VerifierPolicyError::Revoked))
    );
    assert_eq!(
        verifier(PolicyIdentity {
            signing_seed: [0x4a; 32],
            protocol_version: PROTOCOL_VERSION,
            network_id: 42,
            epoch: 2,
            first_batch: 8,
            last_batch: 9,
            revoked_at_batch: None,
            handshake_signing_seed: [0x3a; 32],
            handshake_batch: 7,
        })
        .verify_state(&raw),
        Err(StateEvidenceError::Policy(VerifierPolicyError::BatchRange))
    );
}

#[test]
fn receipt_execution_id_must_bind_the_signed_headers_batch_number() {
    let raw = crate::support::raw_receipt_for_execution_batch([0x71; 32], 0, 25, 9, 8);
    assert_eq!(
        crate::support::evidence_verifier().verify_receipt(&raw),
        Err(ReceiptEvidenceError::BatchIdentity)
    );
}

#[track_caller]
fn lint_success<T, E: std::fmt::Debug>(result: Result<T, E>, context: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{context}: {error:?}"),
    }
}
