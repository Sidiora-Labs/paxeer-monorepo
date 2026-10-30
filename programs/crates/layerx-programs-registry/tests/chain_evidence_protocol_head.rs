#[path = "support/chain.rs"]
mod chain;

use chain::{ChainFixture, STORE_NAME};
use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_evidence::protocol_head::ChainProtocolHeadProof;
use layerx_programs::{chain_evidence, chain_proof, ProtocolEvidenceError};

const CHAIN_ID: &str = "paxeer-protocol-head";
const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const KERNEL_KEY: &[u8] = b"programs:protocol-head";
const VALUE: &[u8] = b"committed module state";

fn sign_height(
    fixture: &mut ChainFixture,
    height: i64,
) -> Result<(), chain_proof::ChainProofError> {
    fixture.header.height = height;
    fixture.commit.height = height;
    fixture.commit.block_id.hash = fixture.header.hash()?.to_vec();
    for signature in &mut fixture.commit.signatures {
        let key = SEEDS
            .iter()
            .map(SigningKey::from_bytes)
            .find(|key| {
                chain_proof::validators::TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .ok_or(chain_proof::ChainProofError::Mismatch)?;
        let bytes = chain_proof::vote::vote_sign_bytes(
            &fixture.header.chain_id,
            height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            signature.timestamp,
        )?;
        signature.signature = key.sign(&bytes).to_bytes().to_vec();
    }
    Ok(())
}

#[test]
fn chain_protocol_head_binds_app_hash_and_height() -> Result<(), chain_proof::ChainProofError> {
    let key = chain_evidence::programs_state_key(KERNEL_KEY);
    let mut fixture = ChainFixture::new(CHAIN_ID, &SEEDS, vec![(key.clone(), VALUE.to_vec())])?;
    sign_height(&mut fixture, 12)?;
    let next = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)?;
    let ops = fixture.prove(&key)?;
    let proof = ChainProtocolHeadProof {
        ops: &ops,
        kernel_key: KERNEL_KEY,
        value: VALUE,
        store_name: STORE_NAME,
        verifier: &fixture.verifier,
        header: &fixture.header,
        commit: &fixture.commit,
    };
    let head = proof
        .verify(&next)
        .unwrap_or_else(|error| panic!("protocol head proof: {error}"));
    assert_eq!(head.state_root(), next.app_hash());
    assert_eq!(head.state_root().as_slice(), fixture.header.app_hash);
    assert_eq!(head.height(), 11);
    assert_eq!(head.height(), next.height() - 1);
    assert_eq!(head.header_height(), 12);
    assert_eq!(head.header_hash(), fixture.header.hash()?);
    assert_eq!(head.header_hash().as_slice(), fixture.commit.block_id.hash);

    assert_eq!(
        ChainProtocolHeadProof {
            value: b"changed",
            ..proof
        }
        .verify(&next),
        Err(ProtocolEvidenceError::StateProof)
    );
    assert_eq!(
        ChainProtocolHeadProof {
            kernel_key: b"different",
            ..proof
        }
        .verify(&next),
        Err(ProtocolEvidenceError::StateProof)
    );
    assert_eq!(
        ChainProtocolHeadProof {
            store_name: "bank",
            ..proof
        }
        .verify(&next),
        Err(ProtocolEvidenceError::StateProof)
    );
    assert_eq!(
        ChainProtocolHeadProof { ops: &[], ..proof }.verify(&next),
        Err(ProtocolEvidenceError::StateProof)
    );
    let mut corrupt_ops = ops.clone();
    corrupt_ops[0].data.push(0);
    assert_eq!(
        ChainProtocolHeadProof {
            ops: &corrupt_ops,
            ..proof
        }
        .verify(&next),
        Err(ProtocolEvidenceError::StateProof)
    );
    let mut forged_commit = fixture.commit.clone();
    forged_commit.signatures[0].signature[0] ^= 1;
    assert_eq!(
        ChainProtocolHeadProof {
            commit: &forged_commit,
            ..proof
        }
        .verify(&next),
        Err(ProtocolEvidenceError::StateRoot)
    );
    let mut forged_header = fixture.header.clone();
    forged_header.consensus_hash[0] ^= 1;
    assert_eq!(
        ChainProtocolHeadProof {
            header: &forged_header,
            ..proof
        }
        .verify(&next),
        Err(ProtocolEvidenceError::StateRoot)
    );

    let foreign = ChainFixture::new("other-chain", &SEEDS, vec![(key.clone(), VALUE.to_vec())])?;
    let foreign_head = foreign
        .verifier
        .verify_head(&foreign.header, &foreign.commit)?;
    assert_eq!(
        proof.verify(&foreign_head),
        Err(ProtocolEvidenceError::ProtocolDomain)
    );
    assert_eq!(
        ChainProtocolHeadProof {
            verifier: &foreign.verifier,
            ..proof
        }
        .verify(&next),
        Err(ProtocolEvidenceError::ProtocolDomain)
    );

    let changed = ChainFixture::new(CHAIN_ID, &SEEDS, vec![(key.clone(), b"new state".to_vec())])?;
    let changed_head = changed
        .verifier
        .verify_head(&changed.header, &changed.commit)?;
    assert_ne!(changed_head.app_hash(), next.app_hash());
    assert_eq!(
        proof.verify(&changed_head),
        Err(ProtocolEvidenceError::StateRoot)
    );
    assert_eq!(
        ChainProtocolHeadProof {
            header: &changed.header,
            commit: &changed.commit,
            ..proof
        }
        .verify(&changed_head),
        Err(ProtocolEvidenceError::StateProof)
    );
    let changed_ops = changed.prove(&key)?;
    let changed_proof = ChainProtocolHeadProof {
        ops: &changed_ops,
        value: b"new state",
        verifier: &changed.verifier,
        header: &changed.header,
        commit: &changed.commit,
        ..proof
    };
    let changed_protocol_head = changed_proof
        .verify(&changed_head)
        .unwrap_or_else(|error| panic!("changed protocol head: {error}"));
    assert_eq!(changed_protocol_head.state_root(), changed_head.app_hash());
    assert_eq!(changed_protocol_head.height(), changed_head.height() - 1);
    assert_eq!(changed_protocol_head.header_hash(), changed.header.hash()?);

    sign_height(&mut fixture, 13)?;
    let later = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)?;
    let later_head = ChainProtocolHeadProof {
        ops: &ops,
        kernel_key: KERNEL_KEY,
        value: VALUE,
        store_name: STORE_NAME,
        verifier: &fixture.verifier,
        header: &fixture.header,
        commit: &fixture.commit,
    }
    .verify(&later)
    .unwrap_or_else(|error| panic!("later protocol head: {error}"));
    assert_eq!(later_head.state_root(), head.state_root());
    assert_eq!(later_head.height(), 12);
    assert_eq!(later_head.header_height(), 13);
    assert_eq!(later_head.header_hash(), fixture.header.hash()?);
    assert_ne!(later_head.header_hash(), head.header_hash());
    Ok(())
}
