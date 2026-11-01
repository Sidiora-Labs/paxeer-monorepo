#[path = "support/chain.rs"]
mod chain;
#[path = "../src/chain_evidence/verify_head.rs"]
mod verify_head;

use ed25519_dalek::{Signer, SigningKey};
use layerx_programs as protocol_evidence;
use layerx_programs::chain_evidence::protocol_head::ChainProtocolHeadProof;
use layerx_programs::{
    chain_evidence, chain_proof, ProgramHeadKind, ProtocolEvidenceError, ReadFreshness,
    ReceiptEvidenceDigest,
};
use sha2::{Digest, Sha256};

use chain::{ChainFixture, STORE_NAME};
use chain_proof::{
    validators::TrustedValidator,
    vote::{vote_sign_bytes, Timestamp},
};
use verify_head::{chain_head_at, historical_protocol_head_at, protocol_head_at, ChainTrustStep};

const CHAIN_ID: &str = "paxeer-verifier-head";
const SEEDS: [[u8; 32]; 4] = [[1; 32], [2; 32], [3; 32], [4; 32]];
const ROTATED_SEEDS: [[u8; 32]; 4] = [[5; 32], [6; 32], [7; 32], [8; 32]];
const KERNEL_KEY: &[u8] = b"programs:protocol-head";
const VALUE: &[u8] = b"committed module state";
const MAX_AGE_MS: u64 = 5_000;

fn fixture(seeds: &[[u8; 32]]) -> ChainFixture {
    ChainFixture::new(
        CHAIN_ID,
        seeds,
        vec![(
            chain_evidence::programs_state_key(KERNEL_KEY),
            VALUE.to_vec(),
        )],
    )
    .expect("signed header and real state membership proof")
}

fn sign(fixture: &mut ChainFixture, seeds: &[[u8; 32]]) {
    fixture.commit.height = fixture.header.height;
    fixture.commit.block_id.hash = fixture.header.hash().expect("header hash").to_vec();
    for signature in &mut fixture.commit.signatures {
        let key = seeds
            .iter()
            .map(SigningKey::from_bytes)
            .find(|key| {
                TrustedValidator::new(key.verifying_key().to_bytes(), 1)
                    .address
                    .as_slice()
                    == signature.validator_address
            })
            .expect("validator signing key");
        signature.timestamp = Timestamp {
            seconds: fixture.header.time.seconds,
            nanos: fixture.header.time.nanos,
        };
        let bytes = vote_sign_bytes(
            &fixture.header.chain_id,
            fixture.commit.height,
            fixture.commit.round,
            Some(&fixture.commit.block_id),
            signature.timestamp,
        )
        .expect("canonical vote bytes");
        signature.signature = key.sign(&bytes).to_bytes().to_vec();
    }
}

fn proof<'a>(
    fixture: &'a ChainFixture,
    ops: &'a [chain_proof::decode::ProofOp],
) -> ChainProtocolHeadProof<'a> {
    ChainProtocolHeadProof {
        ops,
        kernel_key: KERNEL_KEY,
        value: VALUE,
        store_name: STORE_NAME,
        verifier: &fixture.verifier,
        header: &fixture.header,
        commit: &fixture.commit,
    }
}

fn evidence_digest(fixture: &ChainFixture) -> [u8; 32] {
    let mut bytes = fixture.header.hash().expect("header hash").to_vec();
    bytes.extend_from_slice(&fixture.header.app_hash);
    Sha256::digest(bytes).into()
}

#[test]
fn chain_protocol_and_chain_heads_verify() {
    let mut fixture = fixture(&SEEDS);
    fixture.header.height = 12;
    sign(&mut fixture, &SEEDS);
    let verified = fixture
        .verifier
        .verify_head(&fixture.header, &fixture.commit)
        .expect("independently verified chain header");
    let now_ms = u64::try_from(verified.time_ms()).expect("positive chain time");
    let ops = fixture
        .prove(&chain_evidence::programs_state_key(KERNEL_KEY))
        .expect("state proof");
    let proof = proof(&fixture, &ops);
    let protocol =
        protocol_head_at(&fixture.verifier, &proof, now_ms, MAX_AGE_MS).expect("protocol head");
    let digest = evidence_digest(&fixture);
    assert_eq!(protocol.state_root(), verified.app_hash());
    assert_eq!(protocol.state_root().as_slice(), fixture.header.app_hash);
    assert_eq!(protocol.header_hash(), verified.header_hash());
    assert_eq!(protocol.header_height(), 12);
    assert_eq!(protocol.receipt_digest(), digest);
    assert_eq!(protocol.head_kind(), ProgramHeadKind::Ordinary);
    assert_eq!(
        protocol.receipt_evidence_digest(),
        ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(digest)
    );
    assert_eq!(protocol.freshness().observed_sequence, 12);
    assert_eq!(protocol.freshness().observed_at, now_ms);

    for kind in [ProgramHeadKind::Ordinary, ProgramHeadKind::Maintenance] {
        let head = chain_head_at(
            &fixture.verifier,
            kind,
            &fixture.header,
            &fixture.commit,
            now_ms,
            MAX_AGE_MS,
        )
        .expect("chain head");
        assert_eq!(head.state_root(), verified.app_hash());
        assert_eq!(head.header_hash(), verified.header_hash());
        assert_eq!(head.header_height(), 12);
        assert_eq!(head.receipt_digest(), digest);
        assert_eq!(head.receipt_evidence_digest().bytes(), digest);
        assert_eq!(head.head_kind(), kind);
        assert_eq!(head.receipt_evidence_digest().head_kind(), kind);
        assert_eq!(head.freshness(), protocol.freshness());
        assert_eq!(
            head.receipt_evidence_digest(),
            match kind {
                ProgramHeadKind::Ordinary => ReceiptEvidenceDigest::OrdinaryUnsignedReceipt(digest),
                ProgramHeadKind::Maintenance =>
                    ReceiptEvidenceDigest::MaintenanceReceiptSha256(digest),
            }
        );
        assert_eq!(
            chain_head_at(
                &fixture.verifier,
                kind,
                &fixture.header,
                &fixture.commit,
                now_ms + MAX_AGE_MS + 1,
                MAX_AGE_MS,
            ),
            Err(ProtocolEvidenceError::Stale)
        );
    }
    assert_eq!(
        protocol_head_at(&fixture.verifier, &proof, now_ms + MAX_AGE_MS, MAX_AGE_MS),
        Ok(protocol.clone())
    );
    for time in [now_ms + MAX_AGE_MS + 1, now_ms - 1_001] {
        assert_eq!(
            protocol_head_at(&fixture.verifier, &proof, time, MAX_AGE_MS),
            Err(ProtocolEvidenceError::Stale)
        );
    }
    assert_eq!(
        protocol_head_at(
            &fixture.verifier,
            &ChainProtocolHeadProof {
                value: b"changed",
                ..proof
            },
            now_ms,
            MAX_AGE_MS,
        ),
        Err(ProtocolEvidenceError::StateProof)
    );
    let mut forged = fixture.commit.clone();
    forged.signatures[0].signature[0] ^= 1;
    assert_eq!(
        protocol_head_at(
            &fixture.verifier,
            &ChainProtocolHeadProof {
                commit: &forged,
                ..proof
            },
            now_ms,
            MAX_AGE_MS,
        ),
        Err(ProtocolEvidenceError::StateRoot)
    );
    assert_eq!(
        chain_head_at(
            &fixture.verifier,
            ProgramHeadKind::Ordinary,
            &fixture.header,
            &forged,
            now_ms,
            MAX_AGE_MS,
        ),
        Err(ProtocolEvidenceError::StateRoot)
    );
    let foreign = fixture_with_chain_id("foreign-chain");
    assert_eq!(
        protocol_head_at(&foreign.verifier, &proof, now_ms, MAX_AGE_MS),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );
    let mut later = fixture;
    later.header.height += 1;
    sign(&mut later, &SEEDS);
    let later_head = protocol_head_at(
        &later.verifier,
        &self::proof(&later, &ops),
        now_ms,
        MAX_AGE_MS,
    )
    .expect("later signed head with the same app hash");
    assert_eq!(later_head.state_root(), protocol.state_root());
    assert_ne!(later_head.receipt_digest(), protocol.receipt_digest());
}

fn fixture_with_chain_id(chain_id: &str) -> ChainFixture {
    ChainFixture::new(
        chain_id,
        &SEEDS,
        vec![(
            chain_evidence::programs_state_key(KERNEL_KEY),
            VALUE.to_vec(),
        )],
    )
    .expect("foreign signed head")
}

#[test]
fn chain_historical_protocol_head_requires_trust_chain() {
    let mut anchor = fixture(&SEEDS);
    anchor.header.height = 12;
    let mut rotated = fixture(&ROTATED_SEEDS);
    rotated.header.height = 13;
    rotated.header.time.seconds += 1;
    anchor.header.next_validators_hash = rotated.verifier.validators.hash().to_vec();
    sign(&mut anchor, &SEEDS);
    sign(&mut rotated, &ROTATED_SEEDS);
    let mut historical = fixture(&ROTATED_SEEDS);
    historical.header.height = 14;
    historical.header.time.seconds += 2;
    sign(&mut historical, &ROTATED_SEEDS);
    let ops = historical
        .prove(&chain_evidence::programs_state_key(KERNEL_KEY))
        .expect("historical state proof");
    let proof = proof(&historical, &ops);
    let history = [ChainTrustStep {
        header: &rotated.header,
        commit: &rotated.commit,
        validators: &rotated.verifier.validators,
    }];
    let head = historical_protocol_head_at(
        &anchor.verifier,
        &anchor.header,
        &anchor.commit,
        &history,
        &proof,
    )
    .expect("historical head reached through authenticated validator rotation");
    assert_eq!(head.state_root().as_slice(), historical.header.app_hash);
    assert_eq!(head.header_hash(), historical.header.hash().expect("hash"));
    assert_eq!(head.header_height(), 14);
    assert_eq!(head.receipt_digest(), evidence_digest(&historical));
    assert_eq!(head.freshness().observed_sequence, 14);
    let historical_time = u64::try_from(historical.header.time.seconds).expect("time") * 1_000
        + u64::try_from(historical.header.time.nanos / 1_000_000).expect("nanos");
    assert_eq!(head.freshness().observed_at, historical_time);
    assert_eq!(
        protocol_head_at(
            &historical.verifier,
            &proof,
            historical_time + MAX_AGE_MS + 1,
            MAX_AGE_MS
        ),
        Err(ProtocolEvidenceError::Stale)
    );
    assert_eq!(
        historical_protocol_head_at(
            &anchor.verifier,
            &anchor.header,
            &anchor.commit,
            &[],
            &proof
        ),
        Err(ProtocolEvidenceError::InvalidTrustAnchor),
        "the rotated set has no overlap, so its intermediate transition is required"
    );

    let mut unrelated = fixture(&[[9; 32], [10; 32], [11; 32], [12; 32]]);
    unrelated.header.height = 15;
    unrelated.header.time.seconds += 3;
    sign(&mut unrelated, &[[9; 32], [10; 32], [11; 32], [12; 32]]);
    unrelated
        .verifier
        .verify_head(&unrelated.header, &unrelated.commit)
        .expect("unrelated head is valid under its own set");
    let unrelated_proof = self::proof(&unrelated, &ops);
    assert_eq!(
        historical_protocol_head_at(
            &anchor.verifier,
            &anchor.header,
            &anchor.commit,
            &history,
            &unrelated_proof
        ),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );
    let mut forged = rotated.commit.clone();
    forged.signatures[0].signature[0] ^= 1;
    let forged_history = [ChainTrustStep {
        header: &rotated.header,
        commit: &forged,
        validators: &rotated.verifier.validators,
    }];
    assert_eq!(
        historical_protocol_head_at(
            &anchor.verifier,
            &anchor.header,
            &anchor.commit,
            &forged_history,
            &proof
        ),
        Err(ProtocolEvidenceError::InvalidTrustAnchor)
    );
    assert_eq!(
        historical_protocol_head_at(
            &anchor.verifier,
            &anchor.header,
            &anchor.commit,
            &history,
            &ChainProtocolHeadProof {
                value: b"changed",
                ..proof
            },
        ),
        Err(ProtocolEvidenceError::StateProof)
    );
    let anchor_proof = self::proof(&anchor, &ops);
    let anchored = historical_protocol_head_at(
        &anchor.verifier,
        &anchor.header,
        &anchor.commit,
        &[],
        &anchor_proof,
    )
    .expect("exact historical trust anchor");
    assert_eq!(anchored.header_height(), 12);
    assert_eq!(anchored.receipt_digest(), evidence_digest(&anchor));
}
