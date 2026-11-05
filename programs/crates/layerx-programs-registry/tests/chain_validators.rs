use layerx_programs::chain_proof::ChainProofError;
use sha2::{Digest, Sha256};

#[path = "../src/chain_proof/validators.rs"]
mod validators;

use validators::{has_two_thirds, TrustedValidator, ValidatorSet, MAX_TOTAL_VOTING_POWER};

const PUBLIC_KEYS: [[u8; 32]; 3] = [
    [
        0xd7, 0x5a, 0x98, 0x01, 0x82, 0xb1, 0x0a, 0xb7, 0xd5, 0x4b, 0xfe, 0xd3, 0xc9, 0x64, 0x07,
        0x3a, 0x0e, 0xe1, 0x72, 0xf3, 0xda, 0xa6, 0x23, 0x25, 0xaf, 0x02, 0x1a, 0x68, 0xf7, 0x07,
        0x51, 0x1a,
    ],
    [
        0x3d, 0x40, 0x17, 0xc3, 0xe8, 0x43, 0x89, 0x5a, 0x92, 0xb7, 0x0a, 0xa7, 0x4d, 0x1b, 0x7e,
        0xbc, 0x9c, 0x98, 0x2c, 0xcf, 0x2e, 0xc4, 0x96, 0x8c, 0xc0, 0xcd, 0x55, 0xf1, 0x2a, 0xf4,
        0x66, 0x0c,
    ],
    [
        0xfc, 0x51, 0xcd, 0x8e, 0x62, 0x18, 0xa1, 0xa3, 0x8d, 0xa4, 0x7e, 0xd0, 0x02, 0x30, 0xf0,
        0x58, 0x08, 0x16, 0xed, 0x13, 0xba, 0x33, 0x03, 0xac, 0x5d, 0xeb, 0x91, 0x15, 0x48, 0x90,
        0x80, 0x25,
    ],
];

fn leaf(bytes: &[u8]) -> [u8; 32] {
    Sha256::digest([&[0][..], bytes].concat()).into()
}

fn branch(left: [u8; 32], right: [u8; 32]) -> [u8; 32] {
    Sha256::digest([&[1][..], &left, &right].concat()).into()
}

#[test]
fn chain_validator_set_hash_matches_simple_validator_merkle() -> Result<(), ChainProofError> {
    let encodings = [
        [
            0x0a, 0x22, 0x0a, 0x20, 0xd7, 0x5a, 0x98, 0x01, 0x82, 0xb1, 0x0a, 0xb7, 0xd5, 0x4b,
            0xfe, 0xd3, 0xc9, 0x64, 0x07, 0x3a, 0x0e, 0xe1, 0x72, 0xf3, 0xda, 0xa6, 0x23, 0x25,
            0xaf, 0x02, 0x1a, 0x68, 0xf7, 0x07, 0x51, 0x1a, 0x10, 0x0a,
        ],
        [
            0x0a, 0x22, 0x0a, 0x20, 0x3d, 0x40, 0x17, 0xc3, 0xe8, 0x43, 0x89, 0x5a, 0x92, 0xb7,
            0x0a, 0xa7, 0x4d, 0x1b, 0x7e, 0xbc, 0x9c, 0x98, 0x2c, 0xcf, 0x2e, 0xc4, 0x96, 0x8c,
            0xc0, 0xcd, 0x55, 0xf1, 0x2a, 0xf4, 0x66, 0x0c, 0x10, 0x14,
        ],
        [
            0x0a, 0x22, 0x0a, 0x20, 0xfc, 0x51, 0xcd, 0x8e, 0x62, 0x18, 0xa1, 0xa3, 0x8d, 0xa4,
            0x7e, 0xd0, 0x02, 0x30, 0xf0, 0x58, 0x08, 0x16, 0xed, 0x13, 0xba, 0x33, 0x03, 0xac,
            0x5d, 0xeb, 0x91, 0x15, 0x48, 0x90, 0x80, 0x25, 0x10, 0x1e,
        ],
    ];
    let members: Vec<_> = PUBLIC_KEYS
        .into_iter()
        .zip([10, 20, 30])
        .map(|(key, power)| TrustedValidator::new(key, power))
        .collect();
    for (validator, key) in members.iter().zip(PUBLIC_KEYS) {
        assert_eq!(validator.address, Sha256::digest(key)[..20]);
        assert_eq!(validator.public_key, key);
    }
    let set = ValidatorSet::new(members.clone())?;
    assert_eq!(set.validators(), members);
    assert_eq!(set.total_power(), 60);
    let leaves = encodings.map(|bytes| leaf(&bytes));
    let expected = branch(branch(leaves[0], leaves[1]), leaves[2]);
    assert_eq!(set.hash(), expected);
    assert_eq!(
        ValidatorSet::new(vec![members[0].clone()])?.hash(),
        leaves[0]
    );
    assert_eq!(
        ValidatorSet::new(members[..2].to_vec())?.hash(),
        branch(leaves[0], leaves[1])
    );
    let mut reversed = members.clone();
    reversed.reverse();
    assert_eq!(
        ValidatorSet::new(reversed)?.hash(),
        branch(branch(leaves[2], leaves[1]), leaves[0])
    );
    assert_ne!(
        ValidatorSet::new(vec![TrustedValidator::new(PUBLIC_KEYS[0], 11)])?.hash(),
        leaves[0]
    );

    let empty = ValidatorSet::new(vec![])?;
    assert_eq!(empty.hash(), <[u8; 32]>::from(Sha256::digest([])));
    assert_eq!(empty.total_power(), 0);
    for (power, encoded_power) in [
        (0, &[][..]),
        (127, &[0x10, 0x7f]),
        (128, &[0x10, 0x80, 0x01]),
        (16384, &[0x10, 0x80, 0x80, 0x01]),
        (
            MAX_TOTAL_VOTING_POWER,
            &[0x10, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0x0f],
        ),
    ] {
        let mut bytes = encodings[0][..36].to_vec();
        bytes.extend_from_slice(encoded_power);
        let single = ValidatorSet::new(vec![TrustedValidator::new(PUBLIC_KEYS[0], power)])?;
        assert_eq!(single.hash(), leaf(&bytes));
        assert_eq!(single.total_power(), power);
    }

    let five: Vec<_> = (1..=5)
        .map(|seed| {
            let key = ed25519_dalek::SigningKey::from_bytes(&[seed; 32])
                .verifying_key()
                .to_bytes();
            TrustedValidator::new(key, 1)
        })
        .collect();
    let five_leaves: Vec<_> = five
        .iter()
        .map(|validator| {
            let mut bytes = vec![0x0a, 0x22, 0x0a, 0x20];
            bytes.extend_from_slice(&validator.public_key);
            bytes.extend_from_slice(&[0x10, 1]);
            leaf(&bytes)
        })
        .collect();
    assert_eq!(
        ValidatorSet::new(five)?.hash(),
        branch(
            branch(
                branch(five_leaves[0], five_leaves[1]),
                branch(five_leaves[2], five_leaves[3])
            ),
            five_leaves[4]
        )
    );

    assert_eq!(
        ValidatorSet::new(vec![
            members[0].clone(),
            members[1].clone(),
            members[0].clone()
        ]),
        Err(ChainProofError::NonCanonical)
    );
    let mut forged = members[0].clone();
    forged.address[0] ^= 1;
    assert_eq!(
        ValidatorSet::new(vec![forged]),
        Err(ChainProofError::NonCanonical)
    );
    assert_eq!(MAX_TOTAL_VOTING_POWER, (i64::MAX / 8) as u64);
    for power in [MAX_TOTAL_VOTING_POWER + 1, i64::MAX as u64, u64::MAX] {
        assert_eq!(
            ValidatorSet::new(vec![TrustedValidator::new(PUBLIC_KEYS[0], power)]),
            Err(ChainProofError::NonCanonical)
        );
    }
    let at_limit = vec![
        TrustedValidator::new(PUBLIC_KEYS[0], MAX_TOTAL_VOTING_POWER - 1),
        TrustedValidator::new(PUBLIC_KEYS[1], 1),
    ];
    assert_eq!(
        ValidatorSet::new(at_limit)?.total_power(),
        MAX_TOTAL_VOTING_POWER
    );
    assert_eq!(
        ValidatorSet::new(vec![
            TrustedValidator::new(PUBLIC_KEYS[0], MAX_TOTAL_VOTING_POWER),
            TrustedValidator::new(PUBLIC_KEYS[1], 1)
        ]),
        Err(ChainProofError::NonCanonical)
    );
    assert_eq!(
        ValidatorSet::new(vec![
            TrustedValidator::new(PUBLIC_KEYS[0], 1),
            TrustedValidator::new(PUBLIC_KEYS[1], u64::MAX)
        ]),
        Err(ChainProofError::NonCanonical)
    );
    Ok(())
}

#[test]
fn chain_validator_power_threshold_is_strictly_two_thirds() {
    assert!(!has_two_thirds(40, 60));
    assert!(has_two_thirds(41, 60));
    assert!(!has_two_thirds(39, 60));
    assert!(has_two_thirds(60, 60));
    assert!(!has_two_thirds(0, 0));
    assert!(!has_two_thirds(0, 1));
    assert!(has_two_thirds(1, 1));
    assert!(!has_two_thirds(1, 2));
    assert!(has_two_thirds(2, 2));
    let two_thirds = (u64::MAX / 3) * 2;
    assert!(!has_two_thirds(two_thirds, u64::MAX));
    assert!(has_two_thirds(two_thirds + 1, u64::MAX));
    assert!(has_two_thirds(u64::MAX, u64::MAX));
}
