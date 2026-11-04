use std::error::Error;

use layerx_programs::chain_proof::{
    decode::{decode_commitment_proof, CommitmentProof},
    header::ChainHeader,
    proto::{ProtoReader, ProtoValue},
    validators::{TrustedValidator, ValidatorSet},
    vote::{vote_sign_bytes, BlockId, CommitSig, PartSetHeader},
    ChainProofError, ChainProofVerifier, Commit,
};
use layerx_types::json::{self, JsonValue};
use sha2::{Digest, Sha256};

const VECTORS: &str = include_str!("../src/chain_proof/fixtures/bootstrap_headers.json");

fn hex(value: &JsonValue, path: &str) -> Result<Vec<u8>, Box<dyn Error>> {
    Ok(json::decode_hex(&format!("0x{}", value.str_at(path)?))?)
}

fn transport(record: &JsonValue) -> Result<CommitmentProof, Box<dyn Error>> {
    let proof = CommitmentProof {
        signed_header: hex(record, "protobuf")?,
        commit: hex(record, "commit_protobuf")?,
        validator_set: Vec::new(),
        store_name: "layerxprograms".into(),
        key: b"header".to_vec(),
        value: None,
        proof_ops: Vec::new(),
    };
    let decoded = decode_commitment_proof(&proof.encode()?)?;
    assert_eq!(decoded, proof);
    Ok(decoded)
}

// Decode the recorded Go peer's commit framing; header and signature decoding,
// hashing, vote encoding and cryptographic verification use the production crate.
fn recorded_commit(bytes: &[u8]) -> Result<Commit, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut height = None;
    let mut round = 0;
    let mut block_id = None;
    let mut signatures = Vec::new();
    while let Some(field) = reader.next_field()? {
        match (field.number, field.value) {
            (1, ProtoValue::Varint(value)) if height.is_none() => {
                height = Some(i64::try_from(value).map_err(|_| ChainProofError::NonCanonical)?);
            }
            (2, ProtoValue::Varint(value)) => {
                round = i32::try_from(value).map_err(|_| ChainProofError::NonCanonical)?;
            }
            (3, ProtoValue::Bytes(value)) if block_id.is_none() => {
                let mut id = BlockId::default();
                let mut reader = ProtoReader::new(value);
                while let Some(field) = reader.next_field()? {
                    match (field.number, field.value) {
                        (1, ProtoValue::Bytes(value)) => id.hash = value.to_vec(),
                        (2, ProtoValue::Bytes(value)) => {
                            let mut parts = PartSetHeader::default();
                            let mut reader = ProtoReader::new(value);
                            while let Some(field) = reader.next_field()? {
                                match (field.number, field.value) {
                                    (1, ProtoValue::Varint(value)) => {
                                        parts.total = u32::try_from(value)
                                            .map_err(|_| ChainProofError::NonCanonical)?
                                    }
                                    (2, ProtoValue::Bytes(value)) => parts.hash = value.to_vec(),
                                    _ => return Err(ChainProofError::NonCanonical),
                                }
                            }
                            reader.finish()?;
                            id.part_set_header = parts;
                        }
                        _ => return Err(ChainProofError::NonCanonical),
                    }
                }
                reader.finish()?;
                block_id = Some(id);
            }
            (4, ProtoValue::Bytes(value)) => signatures.push(CommitSig::decode(value)?),
            _ => return Err(ChainProofError::NonCanonical),
        }
    }
    reader.finish()?;
    Ok(Commit {
        height: height.ok_or(ChainProofError::NonCanonical)?,
        round,
        block_id: block_id.ok_or(ChainProofError::NonCanonical)?,
        signatures,
    })
}

// bootstrap-header-encoding: exercise Go MakeBlock/Populate -> Header.ToProto
// -> committed protobuf transcript -> production LXCP codec -> header hash and
// commit verifier. No Go toolchain, network or environment is used by the gate.
#[test]
fn bootstrap_header_encoding_go_producer_transport_consumer() -> Result<(), Box<dyn Error>> {
    assert_eq!(
        Sha256::digest(VECTORS.as_bytes()).as_slice(),
        json::decode_hex("0x181c81885db34f042cef2696df7574ac4d519cc60b09cc6876455321ac9fc66f")?,
        "review the Go producer transcript before updating its pin"
    );
    let vectors = json::parse(VECTORS)?;
    assert_eq!(vectors.str_at("producer")?, "consensus/types: MakeBlock, Header.Populate, Header.ToProto, Header.Hash, VoteSignBytes, VerifyCommitLight");
    let validators = ValidatorSet::new(
        vectors
            .array_at("validators")?
            .iter()
            .map(|record| {
                Ok(TrustedValidator::new(
                    hex(record, "public_key")?.as_slice().try_into()?,
                    record.u64_at("power")?,
                ))
            })
            .collect::<Result<Vec<_>, Box<dyn Error>>>()?,
    )?;
    assert_eq!(validators.validators().len(), 4);
    let verifier = ChainProofVerifier {
        chain_id: "bootstrap-header-v1".into(),
        validators,
    };
    let records = vectors.array_at("headers")?;
    assert_eq!(records.len(), 6);
    let mut previous = None;
    for record in records {
        let name = record.str_at("name")?;
        assert!(record.bool_at("go_validate_basic")?);
        assert!(record.bool_at("go_verify_commit_light")?);
        let transported = transport(record)?;
        let encoded = transported.signed_header;
        let header = ChainHeader::decode(&encoded)?;
        let expected = hex(record, "header_hash")?;
        assert_eq!(header.hash()?.as_slice(), expected, "{name}: Go hash");
        assert_eq!(header.height, i64::try_from(record.u64_at("height")?)?);
        let commit = recorded_commit(&transported.commit)?;
        assert_eq!(commit.height, header.height);
        assert_eq!(commit.round, i32::try_from(record.u64_at("round")?)?);
        assert_eq!(commit.block_id.hash, expected);
        assert_eq!(
            commit.block_id.part_set_header.total,
            u32::try_from(record.u64_at("parts_total")?)?
        );
        assert_eq!(
            commit.block_id.part_set_header.hash,
            hex(record, "parts_hash")?
        );
        assert_eq!(commit.signatures.len(), 4);
        assert_eq!(record.array_at("signatures")?.len(), 4);
        for (signature, exported) in commit.signatures.iter().zip(record.array_at("signatures")?) {
            assert_eq!(*signature, CommitSig::decode(&hex(exported, "protobuf")?)?);
            assert_eq!(
                vote_sign_bytes(
                    &header.chain_id,
                    commit.height,
                    commit.round,
                    Some(&commit.block_id),
                    signature.timestamp
                )?,
                hex(exported, "sign_bytes")?,
                "{name}: Go vote bytes"
            );
        }
        assert_eq!(
            verifier.verify_header_commit(&header, &commit)?.as_slice(),
            expected
        );
        if name.starts_with("initial") {
            assert!(header.last_block_id.hash.is_empty());
            assert!(header.last_block_id.part_set_hash.is_empty());
            assert_eq!(header.last_block_id.part_set_total, 0);
            assert!(header.last_results_hash.is_empty());
            // These headers authenticate but supply no preceding result root.
            // The state-proof API must never silently manufacture one.
            assert_eq!(
                verifier.verify_head(&header, &commit),
                Err(ChainProofError::NonCanonical)
            );
            assert!(!fields(&encoded)?.iter().any(|(number, _)| *number == 12));
            assert_eq!(header.app_hash, [0x41; 32]);
        } else if name == "later" {
            let head = verifier.verify_head(&header, &commit)?;
            assert_eq!(head.header_hash().as_slice(), expected);
            assert_eq!(
                head.last_results_hash().as_slice(),
                header.last_results_hash
            );
            assert_eq!(head.app_hash().as_slice(), header.app_hash);
            assert_eq!(header.last_block_id.hash, previous.clone().unwrap());
        }
        if name == "initial-seven" {
            previous = Some(expected.clone());
        }
        if name == "defaults" {
            for number in [6, 7, 9, 10, 11, 12, 13] {
                assert!(!fields(&encoded)?.iter().any(|(field, _)| *field == number));
            }
            let mut explicit = fields(&encoded)?;
            for number in [6, 7, 9, 10, 11, 12, 13] {
                explicit.push((number, bytes_field(number, &[])));
            }
            explicit.sort_by_key(|(number, _)| *number);
            let explicit_header = ChainHeader::decode(&join(&explicit))?;
            assert_eq!(explicit_header, header);
            assert_eq!(explicit_header.hash()?, header.hash()?);
            // Both predecessor hash fields can also be explicitly empty.
            let parts = bytes_field(2, &[]);
            let block_id = [bytes_field(1, &[]), bytes_field(2, &parts)].concat();
            let explicit = replace(&encoded, 5, Some(bytes_field(5, &block_id)))?;
            assert_eq!(ChainHeader::decode(&explicit)?, header);
        }
        if name != "later" {
            assert_eq!(
                verifier.verify_head(&header, &commit),
                Err(ChainProofError::NonCanonical)
            );
        }
        if name == "app-short" {
            assert_eq!(header.app_hash, [1, 2, 3]);
        }
        if name == "app-long" {
            assert_eq!(header.app_hash, [0x42; 65]);
        }

        let mut changed = header.clone();
        changed.height += 1;
        assert!(
            verifier.verify_header_commit(&changed, &commit).is_err(),
            "{name}: header mutation"
        );
        let mut bad_signature = commit.clone();
        bad_signature.signatures[0].signature[0] ^= 1;
        assert!(
            verifier
                .verify_header_commit(&header, &bad_signature)
                .is_err(),
            "{name}: signature mutation"
        );
        let mut insufficient = commit.clone();
        insufficient.signatures.truncate(2);
        assert_eq!(
            verifier.verify_header_commit(&header, &insufficient),
            Err(ChainProofError::InsufficientPower)
        );
    }
    Ok(())
}

fn varint(mut value: u64) -> Vec<u8> {
    let mut out = Vec::new();
    while value >= 128 {
        out.push((value as u8 & 127) | 128);
        value >>= 7;
    }
    out.push(value as u8);
    out
}

fn bytes_field(number: u32, value: &[u8]) -> Vec<u8> {
    [
        varint(u64::from(number) << 3 | 2),
        varint(value.len() as u64),
        value.to_vec(),
    ]
    .concat()
}

fn fields(bytes: &[u8]) -> Result<Vec<(u32, Vec<u8>)>, ChainProofError> {
    let mut reader = ProtoReader::new(bytes);
    let mut result = Vec::new();
    let mut start = 0;
    while let Some(field) = reader.next_field()? {
        result.push((field.number, bytes[start..reader.cursor()].to_vec()));
        start = reader.cursor();
    }
    reader.finish()?;
    Ok(result)
}

fn join(fields: &[(u32, Vec<u8>)]) -> Vec<u8> {
    fields
        .iter()
        .flat_map(|(_, bytes)| bytes.iter().copied())
        .collect()
}

fn replace(
    bytes: &[u8],
    number: u32,
    replacement: Option<Vec<u8>>,
) -> Result<Vec<u8>, ChainProofError> {
    let mut fields = fields(bytes)?;
    fields.retain(|(field, _)| *field != number);
    if let Some(value) = replacement {
        fields.push((number, value));
    }
    fields.sort_by_key(|(number, _)| *number);
    Ok(join(&fields))
}

#[test]
fn bootstrap_header_encoding_keeps_protocol_refusals() -> Result<(), Box<dyn Error>> {
    let vectors = json::parse(VECTORS)?;
    let record = &vectors.array_at("headers")?[2];
    assert_eq!(record.str_at("name")?, "later");
    let original = hex(record, "protobuf")?;
    for number in [6, 7, 8, 9, 10, 12, 13, 14] {
        for length in [1, 19, 20, 31, 33] {
            if number == 14 && length == 20 {
                continue;
            }
            assert_eq!(
                ChainHeader::decode(&replace(
                    &original,
                    number,
                    Some(bytes_field(number, &vec![0; length]))
                )?),
                Err(ChainProofError::NonCanonical),
                "field {number} length {length}"
            );
        }
    }
    for number in [8, 14] {
        for replacement in [None, Some(bytes_field(number, &[]))] {
            assert_eq!(
                ChainHeader::decode(&replace(&original, number, replacement)?),
                Err(ChainProofError::NonCanonical)
            );
        }
    }
    // Byte defaults must not make wrong wire types, duplicate fields,
    // unknown fields, non-minimal varints or reordering acceptable.
    for number in 6..=14 {
        let scalar = [varint(u64::from(number) << 3), vec![0]].concat();
        assert_eq!(
            ChainHeader::decode(&replace(&original, number, Some(scalar))?),
            Err(ChainProofError::NonCanonical)
        );
    }
    for index in 0..fields(&original)?.len() {
        let mut duplicate = fields(&original)?;
        duplicate.insert(index, duplicate[index].clone());
        assert_eq!(
            ChainHeader::decode(&join(&duplicate)),
            Err(ChainProofError::NonCanonical)
        );
    }
    let mut reordered = fields(&original)?;
    reordered.swap(5, 6);
    assert_eq!(
        ChainHeader::decode(&join(&reordered)),
        Err(ChainProofError::NonCanonical)
    );
    assert_eq!(
        ChainHeader::decode(&[original.clone(), bytes_field(15, &[])].concat()),
        Err(ChainProofError::UnknownField)
    );
    assert_eq!(
        ChainHeader::decode(&replace(&original, 3, Some(vec![0x18, 0x88, 0]))?),
        Err(ChainProofError::NonCanonical)
    );
    for number in [1, 4, 5] {
        assert_eq!(
            ChainHeader::decode(&replace(&original, number, None)?),
            Err(ChainProofError::NonCanonical)
        );
    }
    for total in [1_602, u64::from(u32::MAX), u64::from(u32::MAX) + 1] {
        let part_set = [vec![0x08], varint(total), bytes_field(2, &[0x22; 32])].concat();
        let block_id = [bytes_field(1, &[0x11; 32]), bytes_field(2, &part_set)].concat();
        assert_eq!(
            ChainHeader::decode(&replace(&original, 5, Some(bytes_field(5, &block_id)))?),
            Err(ChainProofError::NonCanonical)
        );
    }
    for (hash_length, parts_length) in [(31, 32), (33, 32), (32, 31), (32, 33)] {
        let part_set = [vec![0x08, 1], bytes_field(2, &vec![0x22; parts_length])].concat();
        let block_id = [
            bytes_field(1, &vec![0x11; hash_length]),
            bytes_field(2, &part_set),
        ]
        .concat();
        assert_eq!(
            ChainHeader::decode(&replace(&original, 5, Some(bytes_field(5, &block_id)))?),
            Err(ChainProofError::NonCanonical)
        );
    }
    let initial = hex(&vectors.array_at("headers")?[0], "protobuf")?;
    let mut reader = ProtoReader::new(&initial);
    while let Some(field) = reader.next_field()? {
        if field.number == 5 {
            assert_eq!(field.value, ProtoValue::Bytes(&[0x12, 0]));
        }
    }
    Ok(())
}
