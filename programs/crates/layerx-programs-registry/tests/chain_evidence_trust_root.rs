#[allow(dead_code)]
#[path = "support/chain.rs"]
mod chain;

#[path = "support/assertions.rs"]
mod assertions;
use assertions::TestResultExt;

use chain::ChainFixture;
use layerx_programs::{
    chain_evidence::programs_state_key, chain_proof::validators::TrustedValidator,
    ProtocolDeploymentVerifier, ProtocolEvidenceError,
};

#[test]
fn chain_trust_root_builds_verifier() {
    let chain_id = "paxeer-trust-root-test";
    let fixture = ChainFixture::new(
        chain_id,
        &[[1; 32], [2; 32], [3; 32]],
        vec![(programs_state_key(b"program"), b"state".to_vec())],
    )
    .or_panic("real signed chain fixture");
    let validators = fixture.verifier.validators.validators().to_vec();
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| {
            panic!(
                "{}: {error:?}",
                "fixture commit verifies with the trusted validators"
            )
        });
    assert_eq!(head.chain_id(), chain_id);

    for max_age_ms in [1, 1_000, u64::MAX] {
        let verifier: ProtocolDeploymentVerifier =
            ProtocolDeploymentVerifier::from_chain_trust(chain_id, validators.clone(), max_age_ms)
                .unwrap_or_else(|error| {
                    panic!(
                        "{}: {error:?}",
                        "nonempty chain trust and positive maximum age"
                    )
                });
        assert_eq!(verifier.staleness_limit_ms(), max_age_ms);
    }

    for (trusted, max_age_ms) in [
        (Vec::new(), 1_000),
        (validators.clone(), 0),
        (Vec::new(), 0),
        (vec![validators[0].clone(), validators[0].clone()], 1_000),
        (
            vec![TrustedValidator::new(validators[0].public_key, 0)],
            1_000,
        ),
    ] {
        assert!(matches!(
            ProtocolDeploymentVerifier::from_chain_trust(chain_id, trusted, max_age_ms),
            Err(ProtocolEvidenceError::InvalidTrustAnchor)
        ));
    }
}
