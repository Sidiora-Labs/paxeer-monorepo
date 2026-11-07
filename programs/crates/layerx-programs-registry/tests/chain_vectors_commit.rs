use std::error::Error;

use layerx_programs::chain_proof::{
    header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp},
    ChainProofVerifier, Commit,
};
use layerx_types::json::{self, JsonValue};
use sha2::{Digest, Sha256};

const LIGHT_COMMIT_VECTOR: &[u8] = include_bytes!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../consensus/types/testdata/light_commit_v1.json"
));
const LIGHT_COMMIT_SHA256: &str =
    "51e81b13834957bd953e10ad900a1f81cb1c9283d7e6bd7af26b267ef46887b2";

fn hex_field(value: &JsonValue, path: &str) -> Result<Vec<u8>, Box<dyn Error>> {
    Ok(json::decode_hex(&format!("0x{}", value.str_at(path)?))?)
}

fn vector_timestamp(text: &str) -> Result<Timestamp, Box<dyn Error>> {
    let fraction = text
        .strip_prefix("2023-11-14T22:13:20.")
        .and_then(|value| value.strip_suffix('Z'))
        .ok_or("unexpected timestamp in the pinned light commit vector")?;
    assert_eq!(fraction.len(), 9);
    Ok(Timestamp {
        seconds: 1_700_000_000,
        nanos: fraction.parse()?,
    })
}

#[test]
fn chain_commit_matches_consensus_vector() -> Result<(), Box<dyn Error>> {
    assert_eq!(
        Sha256::digest(LIGHT_COMMIT_VECTOR).as_slice(),
        json::decode_hex(&format!("0x{LIGHT_COMMIT_SHA256}"))?,
        "consensus light commit vector changed; review its provenance before updating the pin"
    );
    let vector = json::parse(std::str::from_utf8(LIGHT_COMMIT_VECTOR)?)?;
    assert_eq!(vector.u64_at("version")?, 1);
    assert_eq!(vector.str_at("bytes_encoding")?, "hex");

    let recorded_header = vector.path("header")?;
    let time = vector_timestamp(recorded_header.str_at("time")?)?;
    let header = ChainHeader {
        version: HeaderVersion {
            block: recorded_header.str_at("version.block")?.parse()?,
            app: recorded_header.str_at("version.app")?.parse()?,
        },
        chain_id: recorded_header.str_at("chain_id")?.to_owned(),
        height: recorded_header.str_at("height")?.parse()?,
        time: HeaderTime {
            seconds: time.seconds,
            nanos: time.nanos,
        },
        last_block_id: HeaderBlockId {
            hash: hex_field(recorded_header, "last_block_id.hash")?,
            part_set_total: recorded_header
                .u64_at("last_block_id.parts.total")?
                .try_into()?,
            part_set_hash: hex_field(recorded_header, "last_block_id.parts.hash")?,
        },
        last_commit_hash: hex_field(recorded_header, "last_commit_hash")?,
        data_hash: hex_field(recorded_header, "data_hash")?,
        validators_hash: hex_field(recorded_header, "validators_hash")?,
        next_validators_hash: hex_field(recorded_header, "next_validators_hash")?,
        consensus_hash: hex_field(recorded_header, "consensus_hash")?,
        app_hash: hex_field(recorded_header, "app_hash")?,
        last_results_hash: hex_field(recorded_header, "last_results_hash")?,
        evidence_hash: hex_field(recorded_header, "evidence_hash")?,
        proposer_address: hex_field(recorded_header, "proposer_address")?,
    };
    assert_eq!(
        ChainHeader::decode(&hex_field(&vector, "header_protobuf")?)
            .map_err(|error| format!("decode consensus header: {error:?}"))?,
        header
    );
    assert_eq!(
        header
            .hash()
            .map_err(|error| format!("hash consensus header: {error:?}"))?
            .as_slice(),
        hex_field(&vector, "header_hash")?
    );

    let mut validators = Vec::new();
    for recorded in vector.array_at("validators")? {
        let public_key = hex_field(recorded, "public_key")?;
        let validator = TrustedValidator::new(
            public_key.as_slice().try_into()?,
            recorded.u64_at("voting_power")?,
        );
        assert_eq!(
            validator.address.as_slice(),
            hex_field(recorded, "address")?
        );
        validators.push(validator);
    }
    assert_eq!(validators.len(), 4);
    let validators = ValidatorSet::new(validators)
        .map_err(|error| format!("construct consensus validator set: {error:?}"))?;
    assert_eq!(
        validators.hash().as_slice(),
        hex_field(&vector, "validator_set_hash")?
    );
    assert_eq!(validators.hash().as_slice(), header.validators_hash);
    assert_eq!(validators.hash().as_slice(), header.next_validators_hash);

    let recorded_commit = vector.path("commit")?;
    let mut commit = Commit {
        height: recorded_commit.u64_at("height")?.try_into()?,
        round: recorded_commit.u64_at("round")?.try_into()?,
        block_id: BlockId {
            hash: hex_field(recorded_commit, "block_id.hash")?,
            part_set_header: PartSetHeader {
                total: recorded_commit.u64_at("block_id.parts.total")?.try_into()?,
                hash: hex_field(recorded_commit, "block_id.parts.hash")?,
            },
        },
        signatures: Vec::new(),
    };
    for (index, recorded) in recorded_commit.array_at("signatures")?.iter().enumerate() {
        assert_eq!(usize::try_from(recorded.u64_at("validator_index")?)?, index);
        let signature = CommitSig {
            flag: BlockIdFlag::try_from(recorded.u64_at("block_id_flag")?)
                .map_err(|error| format!("decode vote flag: {error:?}"))?,
            validator_address: hex_field(recorded, "validator_address")?,
            timestamp: vector_timestamp(recorded.str_at("timestamp")?)?,
            signature: hex_field(recorded, "signature")?,
        };
        assert_eq!(signature.flag, BlockIdFlag::Commit);
        assert_eq!(
            signature.validator_address,
            validators.validators()[index].address
        );
        assert_eq!(
            vote_sign_bytes(
                &header.chain_id,
                commit.height,
                commit.round,
                Some(&commit.block_id),
                signature.timestamp,
            )
            .map_err(|error| format!("encode consensus vote {index}: {error:?}"))?,
            hex_field(recorded, "vote_sign_bytes")?,
            "consensus vote {index} sign bytes"
        );
        commit.signatures.push(signature);
    }
    assert_eq!(commit.signatures.len(), validators.validators().len());
    assert!(vector.bool_at("verify_commit_light")?);
    let verifier = ChainProofVerifier {
        chain_id: header.chain_id.clone(),
        validators,
    };
    let head = verifier
        .verify_head(&header, &commit)
        .map_err(|error| format!("Rust rejected the consensus-accepted commit: {error:?}"))?;
    assert_eq!(head.chain_id(), header.chain_id);
    assert_eq!(head.height(), header.height);
    assert_eq!(
        head.time_ms(),
        time.seconds * 1_000 + i64::from(time.nanos / 1_000_000)
    );
    assert_eq!(head.app_hash().as_slice(), header.app_hash);
    assert_eq!(head.data_hash().as_slice(), header.data_hash);
    assert_eq!(
        head.last_results_hash().as_slice(),
        header.last_results_hash
    );
    assert_eq!(
        head.next_validators_hash().as_slice(),
        header.next_validators_hash
    );
    Ok(())
}
