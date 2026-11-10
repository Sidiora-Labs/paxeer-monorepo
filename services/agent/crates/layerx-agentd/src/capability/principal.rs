use crate::prepare::{verify_call_binding, EvmPrepared};
use crate::read::committed::CommittedReads;
use layerx_crypto::disclosure::call::ledger_selector;

use super::EffectsError;

/// Ledger ownership resolved from the EVM signer at one authenticated committed height.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct VerifiedLedgerPrincipal {
    pub(crate) sender: [u8; 20],
    pub(crate) identity_key: [u8; 32],
    pub(crate) issuer: [u8; 32],
    pub(crate) fee_account: [u8; 32],
    pub(crate) signing_hash: [u8; 32],
    pub(crate) height: u64,
    pub(crate) state_root: [u8; 32],
}

impl VerifiedLedgerPrincipal {
    /// # Errors
    /// Returns an error if the disclosure does not describe a supported native operation or its
    /// effects cannot be resolved without overflow.
    pub fn resolve(
        prepared: &EvmPrepared,
        reads: &CommittedReads<'_>,
    ) -> Result<Self, EffectsError> {
        verify_call_binding(prepared).map_err(|_| EffectsError::InvalidNativeDisclosure)?;
        let checkpoint = reads.checkpoint();
        if prepared.call.chain_id != 125
            || checkpoint.header.chain_id != "125"
            || prepared.observed_head_height > checkpoint.height
            || checkpoint.height == 0
            || checkpoint.app_hash == [0; 32]
        {
            return Err(EffectsError::InvalidNativeDisclosure);
        }
        let mut address = [0; 32];
        address[12..].copy_from_slice(&prepared.from);
        let unified = lookup(
            reads,
            layerx_client::sequencer::ADDR_PRECOMPILE,
            "getUnifiedAccount(address)",
            &address,
        )?;
        // The dynamic Pax address follows the four-word head.
        if unified.len() < 160
            || unified.len() % 32 != 0
            || unified[..32] != address
            || unified[32..56] != [0; 24]
            || u64::from_be_bytes(
                unified[56..64]
                    .try_into()
                    .map_err(|_| EffectsError::InvalidNativeDisclosure)?,
            ) != 128
        {
            return Err(EffectsError::InvalidNativeDisclosure);
        }
        let length = word(&unified, 4)?;
        if length[..24] != [0; 24] {
            return Err(EffectsError::InvalidNativeDisclosure);
        }
        let length = usize::try_from(u64::from_be_bytes(
            length[24..]
                .try_into()
                .map_err(|_| EffectsError::InvalidNativeDisclosure)?,
        ))
        .map_err(|_| EffectsError::InvalidNativeDisclosure)?;
        let padded = length
            .checked_add(31)
            .map(|n| n & !31)
            .ok_or(EffectsError::Overflow)?;
        if unified.len()
            != 160_usize
                .checked_add(padded)
                .ok_or(EffectsError::Overflow)?
            || unified[160 + length..].iter().any(|byte| *byte != 0)
        {
            return Err(EffectsError::InvalidNativeDisclosure);
        }
        let identity_key = word(&unified, 2)?;
        let fee_account = word(&unified, 3)?;
        if prepared.from == [0; 20] || identity_key == [0; 32] || fee_account == [0; 32] {
            return Err(EffectsError::InvalidNativeDisclosure);
        }
        let issuer = lookup_word(reads, "accountOwner(bytes32)", &fee_account)?;
        if issuer == [0; 32] {
            return Err(EffectsError::InvalidNativeDisclosure);
        }
        Ok(Self {
            sender: prepared.from,
            identity_key,
            issuer,
            fee_account,
            signing_hash: prepared.signing_hash,
            height: checkpoint.height,
            state_root: checkpoint.app_hash,
        })
    }

    #[must_use]
    pub const fn issuer(&self) -> [u8; 32] {
        self.issuer
    }
    #[must_use]
    pub const fn fee_account(&self) -> [u8; 32] {
        self.fee_account
    }
    #[must_use]
    pub const fn sender(&self) -> [u8; 20] {
        self.sender
    }

    pub(crate) fn ledger_account(
        &self,
        reads: &CommittedReads<'_>,
        asset: [u8; 32],
    ) -> Result<[u8; 32], EffectsError> {
        if reads.checkpoint().height != self.height
            || reads.checkpoint().app_hash != self.state_root
            || asset == [0; 32]
        {
            return Err(EffectsError::InvalidNativeDisclosure);
        }
        // The ledger accountId ABI accepts the bound DID public key, not a DID hash.
        let account = lookup_word(
            reads,
            "accountId(bytes32,bytes32)",
            &[self.identity_key, asset].concat(),
        )?;
        if account == [0; 32]
            || lookup_word(reads, "accountOwner(bytes32)", &account)? != self.issuer
        {
            return Err(EffectsError::InvalidNativeDisclosure);
        }
        Ok(account)
    }
}

fn lookup(
    reads: &CommittedReads<'_>,
    target: [u8; 20],
    signature: &str,
    args: &[u8],
) -> Result<Vec<u8>, EffectsError> {
    let mut data = ledger_selector(signature).to_vec();
    data.extend_from_slice(args);
    reads
        .capabilities(target, &data)
        .map(|call| call.canonical_bytes().to_vec())
        .map_err(|_| EffectsError::InvalidNativeDisclosure)
}
fn lookup_word(
    reads: &CommittedReads<'_>,
    signature: &str,
    args: &[u8],
) -> Result<[u8; 32], EffectsError> {
    let bytes = lookup(
        reads,
        crate::prepare::PrecompileTarget::Ledger.address(),
        signature,
        args,
    )?;
    if bytes.len() != 32 {
        return Err(EffectsError::InvalidNativeDisclosure);
    }
    word(&bytes, 0)
}
fn word(bytes: &[u8], index: usize) -> Result<[u8; 32], EffectsError> {
    bytes
        .get(index * 32..(index + 1) * 32)
        .and_then(|bytes| bytes.try_into().ok())
        .ok_or(EffectsError::InvalidNativeDisclosure)
}
