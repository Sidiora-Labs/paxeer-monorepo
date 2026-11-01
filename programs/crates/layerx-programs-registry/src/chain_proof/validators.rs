use std::collections::BTreeSet;

use sha2::{Digest, Sha256};

use super::ChainProofError;

pub const MAX_TOTAL_VOTING_POWER: u64 = (i64::MAX / 8) as u64;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct TrustedValidator {
    pub address: [u8; 20],
    pub public_key: [u8; 32],
    pub voting_power: u64,
}

impl TrustedValidator {
    #[must_use]
    pub fn new(public_key: [u8; 32], voting_power: u64) -> Self {
        let mut address = [0; 20];
        address.copy_from_slice(&Sha256::digest(public_key)[..20]);
        Self {
            address,
            public_key,
            voting_power,
        }
    }

    fn simple_validator_bytes(&self) -> Vec<u8> {
        let mut bytes = Vec::with_capacity(46);
        bytes.extend_from_slice(&[0x0a, 0x22, 0x0a, 0x20]);
        bytes.extend_from_slice(&self.public_key);
        if self.voting_power != 0 {
            bytes.push(0x10);
            let mut power = self.voting_power;
            while power >= 0x80 {
                bytes.push((power as u8 & 0x7f) | 0x80);
                power >>= 7;
            }
            bytes.push(power as u8);
        }
        bytes
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ValidatorSet {
    validators: Vec<TrustedValidator>,
    total_power: u64,
}

impl ValidatorSet {
    pub fn new(validators: Vec<TrustedValidator>) -> Result<Self, ChainProofError> {
        let mut addresses = BTreeSet::new();
        let mut total_power = 0_u64;
        for validator in &validators {
            if validator.address != TrustedValidator::new(validator.public_key, 0).address
                || !addresses.insert(validator.address)
            {
                return Err(ChainProofError::NonCanonical);
            }
            total_power = total_power
                .checked_add(validator.voting_power)
                .filter(|power| *power <= MAX_TOTAL_VOTING_POWER)
                .ok_or(ChainProofError::NonCanonical)?;
        }
        Ok(Self {
            validators,
            total_power,
        })
    }

    #[must_use]
    pub fn validators(&self) -> &[TrustedValidator] {
        &self.validators
    }

    #[must_use]
    pub const fn total_power(&self) -> u64 {
        self.total_power
    }

    #[must_use]
    pub fn hash(&self) -> [u8; 32] {
        simple_merkle_root(&self.validators)
    }
}

fn simple_merkle_root(validators: &[TrustedValidator]) -> [u8; 32] {
    let mut hash = Sha256::new();
    match validators {
        [] => {}
        [validator] => {
            hash.update([0]);
            hash.update(validator.simple_validator_bytes());
        }
        _ => {
            let split = 1_usize << (usize::BITS - (validators.len() - 1).leading_zeros() - 1);
            hash.update([1]);
            hash.update(simple_merkle_root(&validators[..split]));
            hash.update(simple_merkle_root(&validators[split..]));
        }
    }
    hash.finalize().into()
}

#[must_use]
pub fn has_two_thirds(signed_power: u64, total_power: u64) -> bool {
    u128::from(signed_power) * 3 > u128::from(total_power) * 2
}
