use ed25519_dalek::{Signer, SigningKey};
use layerx_programs::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    merkle::{hash_from_byte_slices, SimpleProof},
    results::{verify_tx_result, TxResult},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofError, ChainProofVerifier, Commit, VerifiedChainHead,
};
use layerx_types::json::{decode_hex, parse, JsonValue};
use sha2::{Digest, Sha256};
use std::error::Error;

const RESULTS_VECTOR: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../consensus/types/testdata/results_hash_v1.json"
));
const RESULTS_VECTOR_SHA256: &str =
    "1aec2a6c2c28d5fa6386c79ca55ca3dcf6fdf31d4fcf7a62f80096eb583b0cce";

fn hex(text: &str) -> Result<Vec<u8>, Box<dyn Error>> {
    Ok(decode_hex(&format!("0x{text}"))?)
}

fn hash(text: &str) -> Result<[u8; 32], Box<dyn Error>> {
    Ok(hex(text)?.as_slice().try_into()?)
}

fn recorded_proof(record: &JsonValue) -> Result<SimpleProof, Box<dyn Error>> {
    let mut aunts = Vec::new();
    for index in 0..record.array_at("proof.aunts")?.len() {
        aunts.push(hash(record.str_at(&format!("proof.aunts.{index}"))?)?);
    }
    Ok(SimpleProof {
        total: record.u64_at("proof.total")?.try_into()?,
        index: record.u64_at("proof.index")?.try_into()?,
        leaf_hash: hash(record.str_at("proof.leaf_hash")?)?,
        aunts,
    })
}

fn verified_next_head(results_hash: [u8; 32]) -> Result<VerifiedChainHead, ChainProofError> {
    let key = SigningKey::from_bytes(&[1; 32]);
    let validator = TrustedValidator::new(key.verifying_key().to_bytes(), 1);
    let address = validator.address;
    let validators = ValidatorSet::new(vec![validator])?;
    let header = ChainHeader {
        version: HeaderVersion { block: 11, app: 1 },
        chain_id: "paxeer-results-vector-test".to_owned(),
        height: 38,
        time: HeaderTime {
            seconds: 1_700_000_000,
            nanos: 0,
        },
        last_block_id: HeaderBlockId {
            hash: vec![1; 32],
            part_set_total: 1,
            part_set_hash: vec![2; 32],
        },
        last_commit_hash: vec![3; 32],
        data_hash: vec![4; 32],
        validators_hash: validators.hash().to_vec(),
        next_validators_hash: validators.hash().to_vec(),
        consensus_hash: vec![6; 32],
        app_hash: vec![7; 32],
        last_results_hash: results_hash.to_vec(),
        evidence_hash: vec![9; 32],
        proposer_address: address.to_vec(),
    };
    let mut commit = Commit {
        height: header.height,
        round: 0,
        block_id: BlockId {
            hash: header.hash()?.to_vec(),
            part_set_header: PartSetHeader {
                total: 1,
                hash: vec![10; 32],
            },
        },
        signatures: Vec::new(),
    };
    let timestamp = Timestamp {
        seconds: header.time.seconds,
        nanos: header.time.nanos,
    };
    let sign_bytes = vote_sign_bytes(
        &header.chain_id,
        commit.height,
        commit.round,
        Some(&commit.block_id),
        timestamp,
    )?;
    commit.signatures.push(CommitSig {
        flag: BlockIdFlag::Commit,
        validator_address: address.to_vec(),
        timestamp,
        signature: key.sign(&sign_bytes).to_bytes().to_vec(),
    });
    ChainProofVerifier {
        chain_id: header.chain_id.clone(),
        validators,
    }
    .verify_head(&header, &commit)
}

#[test]
fn chain_results_match_consensus_vector() -> Result<(), Box<dyn Error>> {
    let digest: [u8; 32] = Sha256::digest(RESULTS_VECTOR.as_bytes()).into();
    assert_eq!(digest, hash(RESULTS_VECTOR_SHA256)?);

    let vector = parse(RESULTS_VECTOR)?;
    assert_eq!(vector.u64_at("version")?, 1);
    assert_eq!(vector.str_at("bytes_encoding")?, "hex");
    let records = vector.array_at("results")?;
    assert_eq!(records.len(), 3);
    let results_hash = hash(vector.str_at("abci_results_hash")?)?;
    let next = verified_next_head(results_hash)
        .map_err(|error| format!("verify signed vector header: {error:?}"))?;
    assert_eq!(next.last_results_hash(), results_hash);

    let mut encoded_results = Vec::new();
    for (index, record) in records.iter().enumerate() {
        let result = TxResult {
            code: record.u64_at("code")?.try_into()?,
            data: hex(record.str_at("data")?)?,
            gas_wanted: record.u64_at("gas_wanted")?.try_into()?,
            gas_used: record.u64_at("gas_used")?.try_into()?,
        };
        let proof = recorded_proof(record)?;
        assert_eq!(proof.total, i64::try_from(records.len())?);
        assert_eq!(proof.index, i64::try_from(index)?);
        let protobuf = hex(record.str_at("protobuf")?)?;
        assert_eq!(proof.verify(&results_hash, &protobuf), Ok(()));
        assert_eq!(
            verify_tx_result(&next, &result, &proof),
            Ok(()),
            "consensus result {index} must verify against the recorded results hash"
        );
        encoded_results.push(protobuf);
    }
    let leaves: Vec<&[u8]> = encoded_results.iter().map(Vec::as_slice).collect();
    assert_eq!(hash_from_byte_slices(&leaves), results_hash);
    Ok(())
}
