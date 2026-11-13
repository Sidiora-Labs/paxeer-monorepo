#[allow(dead_code)]
#[path = "support/repair_22_33_221.rs"]
mod repair;

use layerx_programs::{
    chain_proof::decode::decode_commitment_proof, ProgramInterface, ProtocolEvidenceError,
    ReadFreshness,
};

#[test]
fn chain_deployment_binds_code_hash_at_head() {
    repair::deployment_interface_fixture();
    for version in [1, 2] {
        let descriptor = repair::descriptor(version);
        // Presence and absence each need their own authenticated state.
        for published in [None, Some(&descriptor)] {
            let (fixture, proof) = repair::state_fixture(version, published);
            let head = fixture
                .verifier
                .verify_head(&fixture.header, &fixture.commit)
                .unwrap_or_else(|error| panic!("{error:?}"));
            let evidence = proof
                .verify(&head)
                .unwrap_or_else(|error| panic!("{error:?}"));
            assert_eq!(evidence.program(), proof.program_state.program);
            assert_eq!(
                evidence.version(),
                u32::try_from(version).unwrap_or_else(|error| panic!("{error:?}"))
            );
            assert_eq!(evidence.upgrade_policy(), repair::policy());
            assert_eq!(evidence.abi_version(), 1);
            assert_eq!(evidence.code_hash(), repair::hash(&proof.code));
            assert_eq!(evidence.module(), proof.code);
            assert_eq!(evidence.state_height(), head.height() - 1);
            assert_eq!(evidence.state_root(), head.app_hash());
            assert_eq!(
                evidence.freshness(),
                ReadFreshness {
                    observed_sequence: u64::from_le_bytes((head.height()).to_le_bytes()),
                    observed_at: u64::from_le_bytes((head.time_ms()).to_le_bytes()),
                }
            );
            assert_eq!(evidence.interface_present(), published.is_some());
            assert_eq!(evidence.interface(), published);
            let (other, _) = repair::state_fixture(3 - version, published);
            let other_head = other
                .verifier
                .verify_head(&other.header, &other.commit)
                .unwrap_or_else(|error| panic!("{error:?}"));
            assert_ne!(other_head.app_hash(), head.app_hash());
            assert_eq!(
                proof.verify(&other_head),
                Err(ProtocolEvidenceError::StateProof)
            );
            let mut broken = proof.clone();
            broken.program_state.ops[1].data.pop();
            assert_eq!(broken.verify(&head), Err(ProtocolEvidenceError::StateProof));
        }
    }
}

#[test]
fn chain_deployment_refuses_substituted_code() {
    let descriptor = repair::descriptor(1);
    let (fixture, proof) = repair::state_fixture(1, Some(&descriptor));
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert!(proof.verify(&head).is_ok());
    for code in [repair::node_producer()["wasm.2"].clone(), Vec::new()] {
        let mut substituted = proof.clone();
        assert_ne!(repair::hash(&code), repair::hash(&proof.code));
        substituted.code = code;
        assert_eq!(
            substituted.verify(&head),
            Err(ProtocolEvidenceError::DeploymentMismatch)
        );
    }
    let mut changed = proof.clone();
    let last = changed.code.len() - 1;
    changed.code[last] ^= 1;
    assert_eq!(
        changed.verify(&head),
        Err(ProtocolEvidenceError::DeploymentMismatch)
    );
    let mut malformed = descriptor.canonical_encoding().to_vec();
    malformed.push(0);
    for bytes in [
        repair::descriptor(2).canonical_encoding().to_vec(),
        ProgramInterface::bind(&proof.code, 2, descriptor.entries().to_vec())
            .unwrap_or_else(|error| panic!("{error:?}"))
            .canonical_encoding()
            .to_vec(),
        malformed,
        Vec::new(),
    ] {
        let mut substituted = proof.clone();
        let mut witness = decode_commitment_proof(
            proof
                .interface
                .as_deref()
                .unwrap_or_else(|| panic!("expected value")),
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        // Preserve the original proof: supplying a descriptor alone cannot
        // change the record the node authenticated.
        let mut value = witness
            .value
            .take()
            .unwrap_or_else(|| panic!("expected value"));
        value.truncate(72);
        value.extend_from_slice(&bytes);
        witness.value = Some(value);
        substituted.interface = Some(witness.encode().unwrap_or_else(|error| panic!("{error:?}")));
        assert_eq!(
            substituted.verify(&head),
            Err(ProtocolEvidenceError::StateProof)
        );
    }
    let mut other_program = proof;
    other_program.program_state.program =
        layerx_programs::ProgramId::new([33; 32]).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        other_program.verify(&head),
        Err(ProtocolEvidenceError::ProgramRecord)
    );
}
