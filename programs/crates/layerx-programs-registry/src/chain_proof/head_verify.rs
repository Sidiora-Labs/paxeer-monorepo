use std::collections::BTreeSet;

use super::{
    ed25519::verify_zip215,
    header::ChainHeader,
    validators::{has_two_thirds, ValidatorSet},
    vote::{vote_sign_bytes, BlockIdFlag, Timestamp},
    ChainProofError, Commit, VerifiedChainHead,
};

pub struct ChainProofVerifier {
    pub chain_id: String,
    pub validators: ValidatorSet,
}

impl ChainProofVerifier {
    pub fn verify_head(
        &self,
        header: &ChainHeader,
        commit: &Commit,
    ) -> Result<VerifiedChainHead, ChainProofError> {
        if header.chain_id != self.chain_id
            || header.validators_hash != self.validators.hash()
            || commit.height != header.height
            || commit.block_id.hash != header.hash()?
            || commit.round < 0
            || commit.block_id.part_set_header.total == 0
            || commit.block_id.part_set_header.hash.len() != 32
        {
            return Err(ChainProofError::Mismatch);
        }

        let mut seen = BTreeSet::new();
        let mut signed_power = 0_u64;
        for signature in &commit.signatures {
            if signature.flag == BlockIdFlag::Absent {
                if !signature.validator_address.is_empty()
                    || !signature.signature.is_empty()
                    || signature.timestamp != Timestamp::ZERO
                {
                    return Err(ChainProofError::Mismatch);
                }
                continue;
            }
            let validator = self
                .validators
                .validators()
                .iter()
                .find(|validator| validator.address.as_slice() == signature.validator_address)
                .ok_or(ChainProofError::Mismatch)?;
            if !seen.insert(validator.address) {
                return Err(ChainProofError::Mismatch);
            }
            let bytes = vote_sign_bytes(
                &self.chain_id,
                commit.height,
                commit.round,
                (signature.flag == BlockIdFlag::Commit).then_some(&commit.block_id),
                signature.timestamp,
            )?;
            let signature_bytes: &[u8; 64] = signature
                .signature
                .as_slice()
                .try_into()
                .map_err(|_| ChainProofError::Mismatch)?;
            if !verify_zip215(&validator.public_key, &bytes, signature_bytes) {
                return Err(ChainProofError::Mismatch);
            }
            if signature.flag == BlockIdFlag::Commit {
                signed_power = signed_power
                    .checked_add(validator.voting_power)
                    .ok_or(ChainProofError::Mismatch)?;
            }
        }
        if !has_two_thirds(signed_power, self.validators.total_power()) {
            return Err(ChainProofError::InsufficientPower);
        }

        let hash =
            |bytes: &[u8]| <[u8; 32]>::try_from(bytes).map_err(|_| ChainProofError::NonCanonical);
        Ok(VerifiedChainHead {
            chain_id: self.chain_id.clone(),
            height: header.height,
            time_ms: header
                .time
                .seconds
                .checked_mul(1_000)
                .and_then(|ms| ms.checked_add(i64::from(header.time.nanos / 1_000_000)))
                .ok_or(ChainProofError::NonCanonical)?,
            app_hash: hash(&header.app_hash)?,
            data_hash: hash(&header.data_hash)?,
            last_results_hash: hash(&header.last_results_hash)?,
            next_validators_hash: hash(&header.next_validators_hash)?,
        })
    }
}
