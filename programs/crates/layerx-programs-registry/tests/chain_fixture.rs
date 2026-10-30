mod support;

use ed25519_dalek::{Signature, Verifier, VerifyingKey};
use layerx_programs::{
    chain_evidence::programs_state_key,
    chain_proof::{
        decode::decode_commitment_proof,
        header::ChainHeader,
        ics23::ExistenceProof,
        proto::{ProtoReader, ProtoValue},
        state::{verify_absent, verify_state},
        vote::{vote_sign_bytes, BlockIdFlag, CommitSig},
        ChainProofError,
    },
};
use support::chain::{ChainFixture, STATE_HEIGHT, STORE_NAME};

#[test]
fn chain_fixture_proofs_verify_through_chain_proof_verifier() -> Result<(), ChainProofError> {
    let seeds = [[1; 32], [2; 32], [3; 32], [4; 32]];
    let pairs = [b"b", b"d", b"h", b"j", b"n"]
        .into_iter()
        .rev()
        .map(|suffix| {
            (
                programs_state_key(suffix),
                [b"value:".as_slice(), suffix].concat(),
            )
        })
        .collect::<Vec<_>>();
    let fixture = ChainFixture::new("paxeer-fixture-test", &seeds, pairs.clone())?;
    let head = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)?;
    assert_eq!(head.chain_id(), "paxeer-fixture-test");
    assert_eq!(head.height(), STATE_HEIGHT + 1);
    assert_eq!(head.app_hash().as_slice(), fixture.header.app_hash);
    assert_eq!(fixture.commit.signatures.len(), seeds.len());
    for (validator, signature) in fixture
        .verifier
        .validators
        .validators()
        .iter()
        .zip(&fixture.commit.signatures)
    {
        assert_eq!(signature.flag, BlockIdFlag::Commit);
        assert_eq!(signature.validator_address, validator.address);
        let bytes = vote_sign_bytes(
            head.chain_id(),
            fixture.commit.height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            signature.timestamp,
        )?;
        let key = VerifyingKey::from_bytes(&validator.public_key)
            .map_err(|_| ChainProofError::Mismatch)?;
        let sig =
            Signature::from_slice(&signature.signature).map_err(|_| ChainProofError::Mismatch)?;
        assert!(key.verify(&bytes, &sig).is_ok());
    }
    for (key, value) in &pairs {
        let ops = fixture.prove(key)?;
        assert_eq!(ops.len(), 2);
        assert_eq!(ops[0].op_type, "ics23:iavl");
        assert_eq!(ops[1].op_type, "ics23:simple");
        assert_eq!(
            verify_state(&head, STORE_NAME, key, value, &ops),
            Ok(STATE_HEIGHT)
        );
        assert!(verify_state(&head, STORE_NAME, key, b"changed", &ops).is_err());
        assert!(verify_state(&head, "bank", key, value, &ops).is_err());
        assert!(verify_absent(&head, STORE_NAME, key, &ops).is_err());
        assert_eq!(fixture.prove_absent(key), Err(ChainProofError::Mismatch));
    }
    for suffix in [b"".as_slice(), b"a", b"c", b"e", b"i", b"k", b"o", &[0xff]] {
        let key = programs_state_key(suffix);
        let ops = fixture.prove_absent(&key)?;
        assert_eq!(ops.len(), 2);
        assert_eq!(
            verify_absent(&head, STORE_NAME, &key, &ops),
            Ok(STATE_HEIGHT)
        );
        assert!(verify_absent(&head, "bank", &key, &ops).is_err());
        assert_eq!(fixture.prove(&key), Err(ChainProofError::Absent));
        let encoded = fixture.encode_key(&key)?;
        let decoded = decode_commitment_proof(&encoded)?;
        assert_eq!(decoded.key, key);
        assert_eq!(decoded.value, None);
        assert_eq!(decoded.proof_ops, ops);
        assert_eq!(
            verify_absent(&head, &decoded.store_name, &decoded.key, &decoded.proof_ops),
            Ok(STATE_HEIGHT)
        );
    }

    let encoded = fixture.encode()?;
    assert_eq!(&encoded[..5], b"LXCP\x01");
    let decoded = decode_commitment_proof(&encoded)?;
    assert_eq!(decoded.encode()?, encoded);
    assert_eq!(decoded.store_name, STORE_NAME);
    assert_eq!(decoded.key, programs_state_key(b"b"));
    assert_eq!(decoded.value.as_deref(), Some(b"value:b".as_slice()));
    let header = ChainHeader::decode(&decoded.signed_header)?;
    assert_eq!(header, fixture.header);
    assert_eq!(
        fixture.verifier.verify_head(&header, &fixture.commit)?,
        head
    );
    assert_eq!(
        verify_state(
            &head,
            &decoded.store_name,
            &decoded.key,
            decoded.value.as_deref().ok_or(ChainProofError::Absent)?,
            &decoded.proof_ops
        ),
        Ok(STATE_HEIGHT)
    );

    let mut commit_reader = ProtoReader::new(&decoded.commit);
    let mut signatures = Vec::new();
    while let Some(field) = commit_reader.next_field()? {
        match (field.number, field.value) {
            (1, ProtoValue::Varint(height)) => assert_eq!(height, head.height().unsigned_abs()),
            (3, ProtoValue::Bytes(_)) => {}
            (4, ProtoValue::Bytes(bytes)) => signatures.push(CommitSig::decode(bytes)?),
            _ => return Err(ChainProofError::NonCanonical),
        }
    }
    commit_reader.finish()?;
    assert_eq!(signatures, fixture.commit.signatures);
    let mut validator_reader = ProtoReader::new(&decoded.validator_set);
    let mut validator_count = 0;
    while let Some(field) = validator_reader.next_field()? {
        match (field.number, field.value) {
            (1, ProtoValue::Bytes(_)) => validator_count += 1,
            (2, ProtoValue::Bytes(_)) => {}
            (3, ProtoValue::Varint(power)) => assert_eq!(power, seeds.len() as u64),
            _ => return Err(ChainProofError::NonCanonical),
        }
    }
    validator_reader.finish()?;
    assert_eq!(validator_count, seeds.len());
    let mut store_reader = ProtoReader::new(&decoded.proof_ops[1].data);
    let store_field = store_reader.next_field()?.ok_or(ChainProofError::Absent)?;
    assert_eq!(store_field.number, 1);
    let ProtoValue::Bytes(store_bytes) = store_field.value else {
        return Err(ChainProofError::NonCanonical);
    };
    let store = ExistenceProof::decode(store_bytes)?;
    assert_eq!(store.path.len(), 2);
    store_reader.finish()?;

    let mut forged_commit = fixture.commit.clone();
    forged_commit.signatures[0].signature[0] ^= 1;
    assert!(fixture
        .verifier
        .verify_head(&fixture.header, &forged_commit)
        .is_err());
    let mut changed_header = fixture.header.clone();
    changed_header.app_hash[0] ^= 1;
    assert!(fixture
        .verifier
        .verify_head(&changed_header, &fixture.commit)
        .is_err());
    let mut changed_pairs = pairs.clone();
    changed_pairs[0].1.push(1);
    let other = ChainFixture::new("paxeer-fixture-test", &seeds, changed_pairs)?;
    let other_head = other.verifier.verify_head(&other.header, &other.commit)?;
    assert_ne!(other_head.app_hash(), head.app_hash());
    assert!(verify_state(
        &other_head,
        STORE_NAME,
        &decoded.key,
        decoded.value.as_deref().ok_or(ChainProofError::Absent)?,
        &decoded.proof_ops
    )
    .is_err());

    for count in [1, 2, 3, 6, 8] {
        let entries = (1..=count)
            .map(|index| (vec![index * 2], vec![index]))
            .collect::<Vec<_>>();
        let small = ChainFixture::new("paxeer-fixture-shapes", &seeds[..1], entries.clone())?;
        let head = small.verifier.verify_head(&small.header, &small.commit)?;
        for (key, value) in &entries {
            assert_eq!(
                verify_state(&head, STORE_NAME, key, value, &small.prove(key)?),
                Ok(STATE_HEIGHT)
            );
        }
        for key in (1..=count * 2 + 1).step_by(2) {
            assert_eq!(
                verify_absent(&head, STORE_NAME, &[key], &small.prove_absent(&[key])?),
                Ok(STATE_HEIGHT)
            );
        }
    }
    assert!(ChainFixture::new("empty", &seeds, Vec::new()).is_err());
    assert!(ChainFixture::new("no-validators", &[], pairs.clone()).is_err());
    assert!(ChainFixture::new("duplicate-validators", &[[1; 32]; 2], pairs.clone()).is_err());
    assert!(ChainFixture::new(
        "duplicate-key",
        &seeds,
        vec![pairs[0].clone(), pairs[0].clone()]
    )
    .is_err());
    Ok(())
}

#[test]
fn support_fixture_verifies_through_public_verifier() {
    use layerx_programs::{
        DeploymentProof, ProgramLifecycle, ProtocolEvidenceError, ReadFreshness, UpgradePolicy,
    };
    use support::{
        code_hash, deploy_fixture, program, verifier_for_fixture, AUTHORITY, NOW, WASM_V1, WASM_V2,
    };

    for policy in [
        UpgradePolicy::Authority(AUTHORITY),
        UpgradePolicy::Immutable,
    ] {
        let fixture = deploy_fixture(WASM_V1, policy, 4, NOW + 123);
        let verifier = verifier_for_fixture(&fixture, 1_000);
        let evidence = verifier
            .verify_deployment(&fixture.proof, NOW + 123)
            .unwrap_or_else(|error| panic!("verify shared deployment fixture: {error}"));
        assert_eq!(evidence.program(), program());
        assert_eq!(evidence.code_hash(), code_hash(WASM_V1));
        assert_eq!(evidence.module(), WASM_V1);
        assert_eq!(evidence.version(), 1);
        assert_eq!(evidence.abi_version(), 1);
        assert_eq!(evidence.record().upgrade_policy, policy);
        assert_eq!(evidence.lifecycle(), ProgramLifecycle::Active);
        assert_eq!(
            evidence.interface(),
            Some(&support::fixture_interface(WASM_V1))
        );
        assert_eq!(
            evidence.state_root().as_slice(),
            fixture.chain.header.app_hash
        );
        assert_eq!(evidence.programs_root(), evidence.state_root());
        assert_eq!(
            evidence.receipt_digest(),
            code_hash(&fixture.proof.state.transaction)
        );
        assert_eq!(
            evidence.freshness(),
            ReadFreshness {
                observed_sequence: 5,
                observed_at: NOW + 123,
            }
        );
        let head = fixture
            .chain
            .verifier
            .verify_head(&fixture.chain.header, &fixture.chain.commit)
            .unwrap_or_else(|error| panic!("verify fixture header: {error:?}"));
        let record = fixture
            .proof
            .state
            .state
            .verify(&head)
            .unwrap_or_else(|error| panic!("verify fixture record: {error}"));
        assert_eq!(record.state_height(), 4);
        assert_eq!(fixture.block_height, 4);
        assert_eq!(
            verify_state(
                &head,
                STORE_NAME,
                &programs_state_key(&fixture.interface_witness.key),
                &fixture.interface_witness.value,
                &fixture.interface_witness.proof,
            ),
            Ok(4),
        );
        let encoded = fixture
            .proof
            .canonical_encoding()
            .unwrap_or_else(|error| panic!("encode fixture proof: {error}"));
        let decoded = DeploymentProof::decode(&encoded)
            .unwrap_or_else(|error| panic!("decode fixture proof: {error}"));
        assert_eq!(decoded, fixture.proof);
        assert_eq!(
            verifier.verify_deployment(&decoded, NOW + 123),
            Ok(evidence)
        );
        assert_eq!(
            verifier.verify_deployment(&fixture.proof, NOW + 1_124),
            Err(ProtocolEvidenceError::Stale),
        );
        let mut changed = fixture.proof.clone();
        changed.deployment.code = WASM_V2.to_vec();
        assert_eq!(
            verifier.verify_deployment(&changed, NOW + 123),
            Err(ProtocolEvidenceError::DeploymentMismatch)
        );
        let mut changed = fixture.proof.clone();
        changed.state.transaction[0] ^= 1;
        assert!(verifier.verify_deployment(&changed, NOW + 123).is_err());
        let mut changed = fixture.proof.clone();
        changed.state.state.ops[0].data[0] ^= 1;
        assert!(verifier.verify_deployment(&changed, NOW + 123).is_err());
        let mut changed = fixture.proof.clone();
        changed.state.commitment.commit[0] ^= 1;
        assert!(verifier.verify_deployment(&changed, NOW + 123).is_err());

        let deprecated = support::deprecated_state(&fixture, NOW + 124);
        let current = verifier
            .verify_current_program(&deprecated, program(), NOW + 124)
            .unwrap_or_else(|error| panic!("verify deprecated fixture: {error}"));
        assert_eq!(current.lifecycle(), ProgramLifecycle::Deprecated);
        assert_eq!(current.freshness().observed_sequence, 6);
        assert_eq!(current.code_hash(), code_hash(WASM_V1));
        assert_eq!(current.policy(), policy);
    }

    let upgrade = support::upgrade_fixture(WASM_V1, WASM_V2, 71, NOW);
    let verifier = verifier_for_fixture(&upgrade, 1_000);
    let evidence = verifier
        .verify_deployment(&upgrade.proof, NOW)
        .unwrap_or_else(|error| panic!("verify upgrade fixture: {error}"));
    assert_eq!(evidence.version(), 2);
    assert_eq!(evidence.code_hash(), code_hash(WASM_V2));
    assert_eq!(evidence.module(), WASM_V2);
    assert_eq!(evidence.freshness().observed_sequence, 72);
    let deprecated = support::deprecated_state(&upgrade, NOW + 1);
    let head = verifier
        .verify_current_program(&deprecated, program(), NOW + 1)
        .unwrap_or_else(|error| panic!("verify deprecated upgrade: {error}"));
    assert_eq!(head.lifecycle(), ProgramLifecycle::Deprecated);
    assert_eq!(head.version(), 2);
    assert_eq!(head.code_hash(), code_hash(WASM_V2));

    let call = support::programs_call_fixture(b"program call payload", 5, NOW);
    let verifier = verifier_for_fixture(&call, 1_000);
    let head = verifier
        .verify_current_program(&call.proof.state, program(), NOW)
        .unwrap_or_else(|error| panic!("verify call fixture: {error}"));
    assert_eq!(head.freshness().observed_sequence, 6);
    assert_eq!(head.receipt_digest(), code_hash(b"program call payload"));
    assert_eq!(call.proof.state.transaction, b"program call payload");
    let wrong_abi = support::wrong_abi_fixture(4, NOW);
    assert!(verifier.verify_deployment(&wrong_abi.proof, NOW).is_err());
}
