#[path = "support/chain.rs"]
mod chain;

use chain::ChainFixture;
use layerx_programs::chain_evidence::freshness::chain_freshness;
use layerx_programs::{chain_proof, ProtocolEvidenceError, ReadFreshness};

#[test]
fn chain_freshness_bounds_age_and_future_time() -> Result<(), chain_proof::ChainProofError> {
    let fixture = ChainFixture::new(
        "paxeer-freshness-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![(b"freshness".to_vec(), b"verified".to_vec())],
    )?;
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)?;
    let time_ms = 1_700_000_000_123;
    let expected = ReadFreshness {
        observed_sequence: 38,
        observed_at: time_ms,
    };
    let max_age_ms = 5_000;

    for age_ms in [0, 1, max_age_ms - 1, max_age_ms] {
        assert_eq!(
            chain_freshness(&head, time_ms + age_ms, max_age_ms),
            Ok(expected)
        );
    }
    assert_eq!(
        chain_freshness(&head, time_ms + max_age_ms + 1, max_age_ms),
        Err(ProtocolEvidenceError::Stale)
    );
    for future_ms in [1, 999, 1_000] {
        assert_eq!(
            chain_freshness(&head, time_ms - future_ms, max_age_ms),
            Ok(expected)
        );
    }
    assert_eq!(
        chain_freshness(&head, time_ms - 1_001, max_age_ms),
        Err(ProtocolEvidenceError::Stale)
    );
    assert_eq!(chain_freshness(&head, time_ms, 0), Ok(expected));
    assert_eq!(chain_freshness(&head, time_ms - 1_000, 0), Ok(expected));
    assert_eq!(
        chain_freshness(&head, time_ms + 1, 0),
        Err(ProtocolEvidenceError::Stale)
    );
    assert_eq!(
        chain_freshness(&head, 0, u64::MAX),
        Err(ProtocolEvidenceError::Stale)
    );
    assert_eq!(
        chain_freshness(&head, u64::MAX, max_age_ms),
        Err(ProtocolEvidenceError::Stale)
    );
    assert_eq!(chain_freshness(&head, u64::MAX, u64::MAX), Ok(expected));
    Ok(())
}
