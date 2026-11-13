//! Companion runtime proof for the documented registry path. The task's declared
//! qualification gate is the Python documentation test, not a Cargo invocation.

#[path = "../crates/layerx-programs-registry/tests/support/chain.rs"]
mod chain;

use chain::{ChainFixture, STATE_HEIGHT};
use layerx_programs::{
    chain_evidence::{commitment_trust_step, programs_state_key, HOST_STORE_NAME},
    chain_proof::{decode::decode_commitment_proof, state::verify_absent, ChainProofError},
    verify_state,
};

const README: &str = include_str!("../README.md");

#[test]
fn repair_22_33_238_registry_read_bridge() -> Result<(), ChainProofError> {
    let key = programs_state_key(b"documented-read");
    let value = b"committed-program-state";
    let fixture = ChainFixture::new(
        "paxeer-documentation-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![(key.clone(), value.to_vec())],
    )?;
    let wire = fixture.encode()?;
    let proof = decode_commitment_proof(&wire)?;
    assert_eq!(proof.encode()?, wire);
    let (header, commit, validators) = commitment_trust_step(&proof)?;
    assert_eq!(validators, fixture.verifier.validators);
    let head = fixture.verifier.verify_head(&header, &commit)?;
    assert_eq!(head.height(), STATE_HEIGHT + 1);
    assert_eq!(HOST_STORE_NAME, "lxphost");
    assert_eq!(proof.store_name, HOST_STORE_NAME);
    let proven_value = proof.value.as_deref().ok_or(ChainProofError::Absent)?;
    assert_eq!(proven_value, value);
    assert_eq!(
        verify_state(&head, HOST_STORE_NAME, &key, proven_value, &proof.proof_ops),
        Ok(STATE_HEIGHT)
    );
    assert!(verify_state(&head, "bank", &key, proven_value, &proof.proof_ops).is_err());
    assert!(verify_state(&head, HOST_STORE_NAME, &key, b"forged", &proof.proof_ops).is_err());
    let mut forged_header = header;
    forged_header.app_hash[0] ^= 1;
    assert!(fixture
        .verifier
        .verify_head(&forged_header, &commit)
        .is_err());

    for fact in [
        "The read bridge is **`layerx-programs-registry`**",
        "`ics23:iavl`",
        "`ics23:simple`",
        "`lxphost`",
        "header at `H + 1`",
        "`verify_state` returns `H`",
    ] {
        assert!(README.contains(fact), "missing documentation: {fact}");
    }
    Ok(())
}

#[test]
fn repair_22_33_238_registry_absent_read() -> Result<(), ChainProofError> {
    let fixture = ChainFixture::new(
        "paxeer-documentation-test",
        &[[1; 32], [2; 32], [3; 32], [4; 32]],
        vec![(programs_state_key(b"present"), b"value".to_vec())],
    )?;
    let missing = programs_state_key(b"missing");
    let proof = decode_commitment_proof(&fixture.encode_key(&missing)?)?;
    assert_eq!(proof.value, None);
    let (header, commit, validators) = commitment_trust_step(&proof)?;
    assert_eq!(validators, fixture.verifier.validators);
    let head = fixture.verifier.verify_head(&header, &commit)?;
    assert_eq!(
        verify_absent(&head, HOST_STORE_NAME, &missing, &proof.proof_ops),
        Ok(STATE_HEIGHT)
    );
    assert!(verify_absent(&head, "bank", &missing, &proof.proof_ops).is_err());
    assert!(verify_state(&head, HOST_STORE_NAME, &missing, b"value", &proof.proof_ops).is_err());
    assert!(README.contains("`verify_absent` authenticates non-membership"));
    Ok(())
}
