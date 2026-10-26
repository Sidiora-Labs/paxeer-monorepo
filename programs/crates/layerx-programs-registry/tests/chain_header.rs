use layerx_programs::chain_proof::{proto, ChainProofError};
use sha2::{Digest, Sha256};

#[path = "../src/chain_proof/header.rs"]
mod header;

use header::{ChainHeader, HeaderBlockId, HeaderTime, HeaderVersion};

fn consensus_header() -> ChainHeader {
    let digest = |bytes: &[u8]| Sha256::digest(bytes).to_vec();
    ChainHeader {
        version: HeaderVersion { block: 1, app: 2 },
        chain_id: "chainId".to_owned(),
        height: 3,
        time: HeaderTime {
            seconds: 1_570_983_284,
            nanos: 0,
        },
        last_block_id: HeaderBlockId {
            hash: vec![0; 32],
            part_set_total: 6,
            part_set_hash: vec![0; 32],
        },
        last_commit_hash: digest(b"last_commit_hash"),
        data_hash: digest(b"data_hash"),
        validators_hash: digest(b"validators_hash"),
        next_validators_hash: digest(b"next_validators_hash"),
        consensus_hash: digest(b"consensus_hash"),
        app_hash: digest(b"app_hash"),
        last_results_hash: digest(b"last_results_hash"),
        evidence_hash: digest(b"evidence_hash"),
        proposer_address: digest(b"proposer_address")[..20].to_vec(),
    }
}

fn varint(mut value: u64) -> Vec<u8> {
    let mut bytes = Vec::new();
    loop {
        let low = (value & 127) as u8;
        value >>= 7;
        bytes.push(low | if value == 0 { 0 } else { 128 });
        if value == 0 {
            return bytes;
        }
    }
}

fn bytes_field(number: u8, bytes: &[u8]) -> Vec<u8> {
    let mut encoded = vec![(number << 3) | 2];
    encoded.extend(varint(bytes.len() as u64));
    encoded.extend_from_slice(bytes);
    encoded
}

fn scalar_field(number: u8, value: u64) -> Vec<u8> {
    if value == 0 {
        return Vec::new();
    }
    let mut encoded = vec![number << 3];
    encoded.extend(varint(value));
    encoded
}

fn encoded_fields(header: &ChainHeader) -> [Vec<u8>; 14] {
    let version = [
        scalar_field(1, header.version.block),
        scalar_field(2, header.version.app),
    ]
    .concat();
    let time = [
        scalar_field(1, u64::from_le_bytes(header.time.seconds.to_le_bytes())),
        scalar_field(2, header.time.nanos as u64),
    ]
    .concat();
    let part_set = [
        scalar_field(1, u64::from(header.last_block_id.part_set_total)),
        bytes_field(2, &header.last_block_id.part_set_hash),
    ]
    .concat();
    let block_id = [
        bytes_field(1, &header.last_block_id.hash),
        bytes_field(2, &part_set),
    ]
    .concat();
    [
        version,
        bytes_field(1, header.chain_id.as_bytes()),
        scalar_field(1, header.height as u64),
        time,
        block_id,
        bytes_field(1, &header.last_commit_hash),
        bytes_field(1, &header.data_hash),
        bytes_field(1, &header.validators_hash),
        bytes_field(1, &header.next_validators_hash),
        bytes_field(1, &header.consensus_hash),
        bytes_field(1, &header.app_hash),
        bytes_field(1, &header.last_results_hash),
        bytes_field(1, &header.evidence_hash),
        bytes_field(1, &header.proposer_address),
    ]
}

fn wire_fields(header: &ChainHeader) -> [Vec<u8>; 14] {
    let fields = encoded_fields(header);
    [
        bytes_field(1, &fields[0]),
        bytes_field(2, header.chain_id.as_bytes()),
        scalar_field(3, header.height as u64),
        bytes_field(4, &fields[3]),
        bytes_field(5, &fields[4]),
        bytes_field(6, &header.last_commit_hash),
        bytes_field(7, &header.data_hash),
        bytes_field(8, &header.validators_hash),
        bytes_field(9, &header.next_validators_hash),
        bytes_field(10, &header.consensus_hash),
        bytes_field(11, &header.app_hash),
        bytes_field(12, &header.last_results_hash),
        bytes_field(13, &header.evidence_hash),
        bytes_field(14, &header.proposer_address),
    ]
}

fn independent_merkle(fields: &[Vec<u8>]) -> [u8; 32] {
    let mut nodes: Vec<[u8; 32]> = fields
        .iter()
        .map(|field| {
            let mut leaf = vec![0];
            leaf.extend_from_slice(field);
            Sha256::digest(leaf).into()
        })
        .collect();
    while nodes.len() > 1 {
        nodes = nodes
            .chunks(2)
            .map(|pair| {
                if pair.len() == 1 {
                    return pair[0];
                }
                let mut branch = vec![1];
                branch.extend_from_slice(&pair[0]);
                branch.extend_from_slice(&pair[1]);
                Sha256::digest(branch).into()
            })
            .collect();
    }
    nodes[0]
}

#[test]
fn chain_header_hash_matches_field_merkle() -> Result<(), ChainProofError> {
    let header = consensus_header();
    let fields = encoded_fields(&header);
    let expected = [
        0xf7, 0x40, 0x12, 0x1f, 0x55, 0x3b, 0x54, 0x18, 0xc3, 0xef, 0xbd, 0x34, 0x3c, 0x2d, 0xbf,
        0xe9, 0xe0, 0x07, 0xbb, 0x67, 0xb0, 0xd0, 0x20, 0xa0, 0x74, 0x13, 0x74, 0xba, 0xb6, 0x52,
        0x42, 0xa4,
    ];
    assert_eq!(header.hash()?, expected);
    assert_eq!(header.hash()?, independent_merkle(&fields));
    let decoded = ChainHeader::decode(&wire_fields(&header).concat())?;
    assert_eq!(decoded, header);
    assert_eq!(decoded.hash()?, expected);

    for index in 0..14 {
        let mut changed = header.clone();
        match index {
            0 => changed.version.block += 1,
            1 => changed.chain_id.push('x'),
            2 => changed.height += 1,
            3 => changed.time.nanos = 1,
            4 => changed.last_block_id.hash[0] ^= 1,
            5 => changed.last_commit_hash[0] ^= 1,
            6 => changed.data_hash[0] ^= 1,
            7 => changed.validators_hash[0] ^= 1,
            8 => changed.next_validators_hash[0] ^= 1,
            9 => changed.consensus_hash[0] ^= 1,
            10 => changed.app_hash[0] ^= 1,
            11 => changed.last_results_hash[0] ^= 1,
            12 => changed.evidence_hash[0] ^= 1,
            13 => changed.proposer_address[0] ^= 1,
            _ => unreachable!(),
        }
        let hash = changed.hash()?;
        assert_ne!(hash, expected, "field {index}");
        assert_eq!(
            hash,
            independent_merkle(&encoded_fields(&changed)),
            "field {index}"
        );
        assert_eq!(
            ChainHeader::decode(&wire_fields(&changed).concat())?,
            changed
        );
    }
    let mut reordered = fields.clone();
    reordered.swap(5, 6);
    assert_ne!(independent_merkle(&reordered), header.hash()?);

    for (seconds, nanos) in [
        (0, 0),
        (0, 999_999_999),
        (-1, 1),
        (-62_135_596_800, 0),
        (253_402_300_799, 999_999_999),
        (1_570_983_284, 123_456_789),
    ] {
        let mut changed = header.clone();
        changed.time = HeaderTime { seconds, nanos };
        changed.height = 300;
        changed.version.app = 0;
        assert_eq!(
            changed.hash()?,
            independent_merkle(&encoded_fields(&changed))
        );
        assert_eq!(
            ChainHeader::decode(&wire_fields(&changed).concat())?,
            changed
        );
    }
    Ok(())
}

#[test]
fn chain_header_refuses_malformed_fields() -> Result<(), ChainProofError> {
    let header = consensus_header();
    assert_eq!(ChainHeader::decode(&wire_fields(&header).concat())?, header);
    for (index, value) in [
        (1, bytes_field(2, b"")),
        (2, vec![0x18, 0]),
        (2, scalar_field(3, u64::MAX)),
        (10, bytes_field(11, &[0; 31])),
    ] {
        let mut fields = wire_fields(&header);
        fields[index] = value;
        assert_eq!(
            ChainHeader::decode(&fields.concat()),
            Err(ChainProofError::NonCanonical)
        );
    }
    for number in 6..=13 {
        for length in [0, 31, 33] {
            let mut fields = wire_fields(&header);
            fields[usize::from(number - 1)] = bytes_field(number, &vec![0; length]);
            assert_eq!(
                ChainHeader::decode(&fields.concat()),
                Err(ChainProofError::NonCanonical)
            );
        }
    }
    for index in 0..14 {
        let mut fields = wire_fields(&header);
        fields[index].clear();
        assert_eq!(
            ChainHeader::decode(&fields.concat()),
            Err(ChainProofError::NonCanonical)
        );
        let mut fields = wire_fields(&header);
        let duplicate = fields[index].clone();
        fields[index].extend(duplicate);
        assert_eq!(
            ChainHeader::decode(&fields.concat()),
            Err(ChainProofError::NonCanonical)
        );
    }
    for index in [0, 3, 4] {
        let mut fields = wire_fields(&header);
        let tag = ((index as u8 + 1) << 3) | 2;
        fields[index] = vec![tag, 2, 0x08, 0x80];
        assert_eq!(
            ChainHeader::decode(&fields.concat()),
            Err(ChainProofError::Truncated)
        );
    }
    let mut unknown = wire_fields(&header).concat();
    unknown.extend([0x78, 1]);
    assert_eq!(
        ChainHeader::decode(&unknown),
        Err(ChainProofError::UnknownField)
    );
    let mut noncanonical = wire_fields(&header);
    noncanonical[2] = vec![0x18, 0x83, 0];
    assert_eq!(
        ChainHeader::decode(&noncanonical.concat()),
        Err(ChainProofError::NonCanonical)
    );
    for (index, value) in [
        (1, bytes_field(2, &[0xff])),
        (2, vec![0x1a, 1, 3]),
        (13, bytes_field(14, &[0; 19])),
    ] {
        let mut fields = wire_fields(&header);
        fields[index] = value;
        assert_eq!(
            ChainHeader::decode(&fields.concat()),
            Err(ChainProofError::NonCanonical)
        );
    }
    let mut reordered = wire_fields(&header);
    reordered.swap(5, 6);
    assert_eq!(
        ChainHeader::decode(&reordered.concat()),
        Err(ChainProofError::NonCanonical)
    );
    for (seconds, nanos) in [
        (-62_135_596_801, 0),
        (253_402_300_800, 0),
        (0, -1),
        (0, 1_000_000_000),
    ] {
        let mut invalid = header.clone();
        invalid.time = HeaderTime { seconds, nanos };
        assert_eq!(invalid.hash(), Err(ChainProofError::NonCanonical));
        assert_eq!(
            ChainHeader::decode(&wire_fields(&invalid).concat()),
            Err(ChainProofError::NonCanonical)
        );
    }
    for part_set in [false, true] {
        let mut invalid = header.clone();
        if part_set {
            invalid.last_block_id.part_set_hash.pop();
        } else {
            invalid.last_block_id.hash.pop();
        }
        assert_eq!(invalid.hash(), Err(ChainProofError::NonCanonical));
        assert_eq!(
            ChainHeader::decode(&wire_fields(&invalid).concat()),
            Err(ChainProofError::NonCanonical)
        );
    }
    let mut invalid = header.clone();
    invalid.chain_id.clear();
    assert_eq!(invalid.hash(), Err(ChainProofError::NonCanonical));
    invalid = header.clone();
    invalid.height = 0;
    assert_eq!(invalid.hash(), Err(ChainProofError::NonCanonical));
    invalid = header.clone();
    invalid.app_hash.pop();
    assert_eq!(invalid.hash(), Err(ChainProofError::NonCanonical));
    Ok(())
}
