use chain_proof::{
    proto::{ProtoField, ProtoReader, ProtoValue},
    ChainProofError,
};
use layerx_programs::chain_proof;

#[path = "../src/chain_proof/vote.rs"]
mod vote;

use vote::{
    vote_sign_bytes, BlockId, BlockIdFlag, CommitSig, PartSetHeader, Timestamp,
    SIGNED_MSG_TYPE_PRECOMMIT,
};

#[test]
fn chain_vote_sign_bytes_are_length_prefixed_canonical_vote() -> Result<(), ChainProofError> {
    let block_id = BlockId {
        hash: (0..32).collect(),
        part_set_header: PartSetHeader {
            total: 100,
            hash: (32..64).collect(),
        },
    };
    let timestamp = Timestamp {
        seconds: 1,
        nanos: 2,
    };
    let bytes = vote_sign_bytes("chain", 300, 7, Some(&block_id), timestamp)?;
    let expected = [
        0x6b, 0x08, 0x02, 0x11, 0x2c, 0x01, 0, 0, 0, 0, 0, 0, 0x19, 0x07, 0, 0, 0, 0, 0, 0, 0,
        0x22, 0x48, 0x0a, 0x20, 0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18,
        19, 20, 21, 22, 23, 24, 25, 26, 27, 28, 29, 30, 31, 0x12, 0x24, 0x08, 0x64, 0x12, 0x20, 32,
        33, 34, 35, 36, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48, 49, 50, 51, 52, 53, 54, 55,
        56, 57, 58, 59, 60, 61, 62, 63, 0x2a, 0x04, 0x08, 0x01, 0x10, 0x02, 0x32, 0x05, b'c', b'h',
        b'a', b'i', b'n',
    ];
    assert_eq!(bytes, expected);
    assert_eq!(usize::from(bytes[0]), bytes.len() - 1);
    let mut reader = ProtoReader::new(&bytes[1..]);
    assert_eq!(
        reader.next_field()?,
        Some(ProtoField {
            number: 1,
            value: ProtoValue::Varint(SIGNED_MSG_TYPE_PRECOMMIT),
        })
    );
    assert_eq!(
        reader.next_field()?,
        Some(ProtoField {
            number: 2,
            value: ProtoValue::Fixed64(300),
        })
    );
    assert_eq!(
        reader.next_field()?,
        Some(ProtoField {
            number: 3,
            value: ProtoValue::Fixed64(7),
        })
    );
    let Some(ProtoField {
        number: 4,
        value: ProtoValue::Bytes(block),
    }) = reader.next_field()?
    else {
        panic!("canonical block ID missing");
    };
    let mut block_reader = ProtoReader::new(block);
    assert_eq!(
        block_reader.next_field()?,
        Some(ProtoField {
            number: 1,
            value: ProtoValue::Bytes(&block_id.hash),
        })
    );
    let Some(ProtoField {
        number: 2,
        value: ProtoValue::Bytes(parts),
    }) = block_reader.next_field()?
    else {
        panic!("canonical part set header missing");
    };
    let mut parts_reader = ProtoReader::new(parts);
    assert_eq!(
        parts_reader.next_field()?,
        Some(ProtoField {
            number: 1,
            value: ProtoValue::Varint(100),
        })
    );
    assert_eq!(
        parts_reader.next_field()?,
        Some(ProtoField {
            number: 2,
            value: ProtoValue::Bytes(&block_id.part_set_header.hash),
        })
    );
    assert_eq!(parts_reader.next_field()?, None);
    parts_reader.finish()?;
    assert_eq!(block_reader.next_field()?, None);
    block_reader.finish()?;
    assert_eq!(
        reader.next_field()?,
        Some(ProtoField {
            number: 5,
            value: ProtoValue::Bytes(&[0x08, 1, 0x10, 2]),
        })
    );
    assert_eq!(
        reader.next_field()?,
        Some(ProtoField {
            number: 6,
            value: ProtoValue::Bytes(b"chain"),
        })
    );
    assert_eq!(reader.next_field()?, None);
    reader.finish()?;

    let long_chain = "c".repeat(128);
    let long = vote_sign_bytes(&long_chain, 300, 7, Some(&block_id), timestamp)?;
    assert_eq!(&long[..2], &[0xe7, 0x01]);
    assert_eq!(long.len() - 2, 231);
    assert_eq!(&long[102..105], &[0x32, 0x80, 0x01]);
    assert_eq!(&long[105..], long_chain.as_bytes());

    let signed = vote_sign_bytes(
        "",
        -1,
        -2,
        None,
        Timestamp {
            seconds: -1,
            nanos: 0,
        },
    )?;
    assert_eq!(&signed[4..12], &(-1_i64).to_le_bytes());
    assert_eq!(&signed[13..21], &(-2_i64).to_le_bytes());
    assert_eq!(
        &signed[21..],
        &[0x2a, 11, 0x08, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 1]
    );

    for flag in [BlockIdFlag::Commit, BlockIdFlag::Nil] {
        let mut encoded = vec![0x08, flag as u8, 0x12, 20];
        encoded.extend_from_slice(&[0x11; 20]);
        encoded.extend_from_slice(&[0x1a, 4, 0x08, 1, 0x10, 2, 0x22, 64]);
        encoded.extend_from_slice(&[0x22; 64]);
        assert_eq!(
            CommitSig::decode(&encoded)?,
            CommitSig {
                flag,
                validator_address: vec![0x11; 20],
                timestamp,
                signature: vec![0x22; 64],
            }
        );
        encoded.pop();
        assert_eq!(CommitSig::decode(&encoded), Err(ChainProofError::Truncated));
    }
    for invalid in [
        &[0x08, 0][..],
        &[0x08, 4],
        &[0x08, 2],
        &[0x08, 3],
        &[0x08, 1, 0x08, 1],
        &[0x08, 1, 0x12, 1, 0],
        &[0x08, 1, 0x1a, 0],
    ] {
        assert_eq!(
            CommitSig::decode(invalid),
            Err(ChainProofError::NonCanonical)
        );
    }
    assert_eq!(
        CommitSig::decode(&[0x08, 1, 0x28, 1]),
        Err(ChainProofError::UnknownField)
    );
    for timestamp in [
        Timestamp {
            seconds: -62_135_596_801,
            nanos: 0,
        },
        Timestamp {
            seconds: 253_402_300_800,
            nanos: 0,
        },
        Timestamp {
            seconds: 0,
            nanos: -1,
        },
        Timestamp {
            seconds: 0,
            nanos: 1_000_000_000,
        },
    ] {
        assert_eq!(
            vote_sign_bytes("chain", 1, 0, None, timestamp),
            Err(ChainProofError::NonCanonical)
        );
    }
    Ok(())
}

#[test]
fn chain_vote_nil_vote_omits_block_id() -> Result<(), ChainProofError> {
    let timestamp = Timestamp {
        seconds: 1,
        nanos: 0,
    };
    let bytes = vote_sign_bytes("chain", 300, 0, None, timestamp)?;
    assert_eq!(
        bytes,
        [
            0x16, 0x08, 2, 0x11, 0x2c, 1, 0, 0, 0, 0, 0, 0, 0x2a, 2, 0x08, 1, 0x32, 5, b'c', b'h',
            b'a', b'i', b'n',
        ]
    );
    assert_eq!(
        vote_sign_bytes("chain", 300, 0, Some(&BlockId::default()), timestamp)?,
        bytes
    );
    let mut reader = ProtoReader::new(&bytes[1..]);
    let mut fields = Vec::new();
    while let Some(field) = reader.next_field()? {
        assert_ne!(field.number, 4);
        fields.push(field.number);
    }
    reader.finish()?;
    assert_eq!(fields, [1, 2, 5, 6]);
    assert_eq!(
        vote_sign_bytes("", 0, 0, None, Timestamp::default())?,
        [4, 0x08, 2, 0x2a, 0]
    );

    let absent = CommitSig::decode(&[0x08, 1])?;
    assert_eq!(absent.flag, BlockIdFlag::Absent);
    assert!(absent.validator_address.is_empty());
    assert_eq!(absent.timestamp, Timestamp::ZERO);
    assert!(absent.signature.is_empty());
    let zero_time = [
        0x08, 0x80, 0x92, 0xb8, 0xc3, 0x98, 0xfe, 0xff, 0xff, 0xff, 1,
    ];
    let mut encoded = vec![0x08, 1, 0x1a, 11];
    encoded.extend_from_slice(&zero_time);
    assert_eq!(CommitSig::decode(&encoded)?, absent);
    Ok(())
}
