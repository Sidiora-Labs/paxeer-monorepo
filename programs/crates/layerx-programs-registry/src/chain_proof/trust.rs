use std::collections::BTreeSet;

use super::{
    ed25519::verify_zip215,
    header::ChainHeader,
    validators::ValidatorSet,
    vote::{vote_sign_bytes, BlockIdFlag},
    ChainProofError, ChainProofVerifier, Commit, VerifiedChainHead,
};

const TRUST_PERIOD_MS: i128 = 1_209_600_000;

impl ChainProofVerifier {
    pub fn advance(
        &self,
        trusted: &VerifiedChainHead,
        header: &ChainHeader,
        commit: &Commit,
        next_validators: ValidatorSet,
    ) -> Result<ChainProofVerifier, ChainProofError> {
        if trusted.chain_id() != self.chain_id || header.chain_id != self.chain_id {
            return Err(ChainProofError::Mismatch);
        }
        let time_ms =
            i128::from(header.time.seconds) * 1_000 + i128::from(header.time.nanos / 1_000_000);
        let elapsed_ms = time_ms - i128::from(trusted.time_ms());
        if header.height <= trusted.height() || !(1..=TRUST_PERIOD_MS).contains(&elapsed_ms) {
            return Err(ChainProofError::Expired);
        }

        if header.height.checked_sub(trusted.height()) == Some(1) {
            if header.validators_hash != trusted.next_validators_hash() {
                return Err(ChainProofError::Mismatch);
            }
        } else {
            let mut seen = BTreeSet::new();
            let mut signed_power = 0_u64;
            for signature in &commit.signatures {
                if signature.flag != BlockIdFlag::Commit {
                    continue;
                }
                let Some(validator) =
                    self.validators.validators().iter().find(|validator| {
                        validator.address.as_slice() == signature.validator_address
                    })
                else {
                    continue;
                };
                if !seen.insert(validator.address) {
                    return Err(ChainProofError::Mismatch);
                }
                let bytes = vote_sign_bytes(
                    &self.chain_id,
                    commit.height,
                    commit.round,
                    Some(&commit.block_id),
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
                signed_power = signed_power
                    .checked_add(validator.voting_power)
                    .ok_or(ChainProofError::Mismatch)?;
            }
            if u128::from(signed_power) * 3 <= u128::from(self.validators.total_power()) {
                return Err(ChainProofError::InsufficientPower);
            }
        }

        let verifier = ChainProofVerifier {
            chain_id: self.chain_id.clone(),
            validators: next_validators,
        };
        verifier.verify_head(header, commit)?;
        Ok(verifier)
    }
}
