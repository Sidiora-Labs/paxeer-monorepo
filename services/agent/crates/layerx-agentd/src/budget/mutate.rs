//! Layerxpay budget fund and revoke identity and committed confirmation.

use super::create::{
    budget_log, committed_receipt, integer, selector, word, BudgetReceiptEvidence,
    BudgetStateEvidence, SequencerBudgetPipeline,
};
use crate::prepare::evm::PrecompileTarget;
use layerx_client::sequencer::SequencerReceipt;
use layerx_crypto::disclosure::BudgetStateContext;
use layerx_crypto::evm_transaction::{decode_signed, recover_sender};
use layerx_types::payload::ModuleRegistry;
use sha2::{Digest as _, Sha256};

use super::accounting::{ProtocolBudgetRecord, ProtocolBudgetState};
use super::create::{BudgetCreationError, CoreBudgetReceipt};
use crate::protocol_evidence::EvidenceAuthority;
use crate::sign::VerifiedSubmission;

const STATE_DIGEST_DOMAIN: &[u8] = b"layerx:budget-state-context:v1\0";

/// One decoded layerxpay fund or revoke call.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum BudgetMutation {
    Fund {
        budget_id: [u8; 32],
        amount: u128,
        encoding_version: u16,
        source_height: Option<u64>,
    },
    Revoke {
        budget_id: [u8; 32],
        revocation_height: u64,
    },
}

impl BudgetMutation {
    #[must_use]
    pub const fn budget_id(&self) -> [u8; 32] {
        match self {
            Self::Fund { budget_id, .. } | Self::Revoke { budget_id, .. } => *budget_id,
        }
    }
}

/// Builds the disclosure context from a verified, decoded core budget record.
///
/// `verified_state` must be the canonical state bytes the evidence authority verified for
/// `record`, and `balance` the verified budget-account balance. A version-1 budget funds from
/// the owner's main account, the source the native create disclosure binds for version 1.
///
/// # Errors
///
/// Returns `ContextMismatch` when the state bytes do not decode to `record`, and
/// `BudgetNotLive` for a closed record.
pub fn budget_state_context(
    record: &ProtocolBudgetRecord,
    verified_state: &[u8],
    observed_head_sequence: u64,
    balance: u128,
) -> Result<BudgetStateContext, BudgetCreationError> {
    let decoded = ProtocolBudgetRecord::decode(verified_state)
        .map_err(|_| BudgetCreationError::ContextMismatch)?;
    if &decoded != record {
        return Err(BudgetCreationError::ContextMismatch);
    }
    if record.closed {
        return Err(BudgetCreationError::BudgetNotLive);
    }
    let state_digest: [u8; 32] = Sha256::new()
        .chain_update(STATE_DIGEST_DOMAIN)
        .chain_update(verified_state)
        .finalize()
        .into();
    Ok(BudgetStateContext {
        budget_id: record.budget_id,
        owner: record.owner,
        budget_account: record.budget_account,
        asset: record.asset_id,
        source_account: record.source_account.unwrap_or(record.owner),
        native_source: record.source_account.is_some(),
        revocation_sequence: record.revocation_sequence,
        balance,
        state_digest,
        observed_head_sequence,
        purpose_hash: record.purpose_hash,
    })
}

/// Decodes a signed canonical budget fund or revoke activity through the shared
/// context-bound native disclosure.
///
/// # Errors
///
/// Returns `NotBudgetCreation` when the bytes are not a canonical signed activity, and
/// `ContextMismatch` when the payload is not a fund or revoke consistent with `context`.
pub fn budget_mutation_identity(
    canonical_activity: &[u8],
    _registry: &ModuleRegistry,
    context: &BudgetStateContext,
) -> Result<BudgetMutation, BudgetCreationError> {
    let (call, _) =
        decode_signed(canonical_activity).map_err(|_| BudgetCreationError::NotBudgetCreation)?;
    let data = &call.data;
    if call.to != PrecompileTarget::Pay.address() || word(data, 0)? != context.budget_id {
        return Err(BudgetCreationError::ContextMismatch);
    }
    let native = data.starts_with(&selector("budgetFundNative(bytes32,uint128,uint64)"));
    if native || data.starts_with(&selector("budgetFund(bytes32,uint128)")) {
        if data.len() != 4 + if native { 96 } else { 64 } || native != context.native_source {
            return Err(BudgetCreationError::ContextMismatch);
        }
        let amount = integer(data, 1, 16)?;
        let source_height = if native {
            Some(integer(data, 2, 8)? as u64)
        } else {
            None
        };
        if amount == 0
            || source_height
                .is_some_and(|height| height == 0 || height > context.observed_head_sequence)
        {
            return Err(BudgetCreationError::ContextMismatch);
        }
        return Ok(BudgetMutation::Fund {
            budget_id: context.budget_id,
            amount,
            encoding_version: if native { 2 } else { 1 },
            source_height,
        });
    }
    if data.starts_with(&selector("budgetRevoke(bytes32,uint64)")) && data.len() == 68 {
        let revocation_height = integer(data, 1, 8)? as u64;
        if revocation_height <= context.revocation_sequence {
            return Err(BudgetCreationError::StaleRevocation);
        }
        return Ok(BudgetMutation::Revoke {
            budget_id: context.budget_id,
            revocation_height,
        });
    }
    Err(BudgetCreationError::ContextMismatch)
}

/// Exact verified-submission transport and proven-state read for budget mutations.
pub trait BudgetMutationPipeline<
    R = crate::protocol_evidence::RawReceiptEvidence,
    S = crate::protocol_evidence::RawStateEvidence,
>
{
    /// Submits the byte-identical verifier-bound activity and returns raw core evidence.
    ///
    /// # Errors
    ///
    /// Returns `Submission` when the exact signed activity does not reach core and produce a
    /// receipt; the outcome is then unknown and must be reconciled under the same key.
    fn submit_verified(
        &mut self,
        submission: &VerifiedSubmission,
    ) -> Result<CoreBudgetReceipt<R>, BudgetCreationError>;

    /// Reads the proven budget module state stored under `budget_state_key(budget_id)`.
    ///
    /// # Errors
    ///
    /// Returns `CreatedBudgetUnconfirmed` when the node serves no proven state for the key.
    fn budget_state(
        &mut self,
        budget_id: [u8; 32],
    ) -> Result<ProtocolBudgetState<S>, BudgetCreationError>;

    /// Reads the state-proven balance of `account` in `asset` (the verified client balance
    /// read); its amount is the `balance` passed to `budget_state_context`.
    ///
    /// # Errors
    ///
    /// Returns `CreatedBudgetUnconfirmed` when no state-proven balance is served.
    fn budget_balance(
        &mut self,
        account: [u8; 32],
        asset: [u8; 32],
    ) -> Result<u128, BudgetCreationError>;
}

/// A budget mutation confirmed by its verified receipt and proven resulting state.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ConfirmedBudgetMutation {
    mutation: BudgetMutation,
    record: ProtocolBudgetRecord,
    activity_id: [u8; 32],
    receipt_bytes: Vec<u8>,
    observed_head_sequence: u64,
}

impl ConfirmedBudgetMutation {
    #[must_use]
    pub const fn mutation(&self) -> &BudgetMutation {
        &self.mutation
    }

    #[must_use]
    pub const fn record(&self) -> &ProtocolBudgetRecord {
        &self.record
    }

    #[must_use]
    pub const fn activity_id(&self) -> [u8; 32] {
        self.activity_id
    }

    #[must_use]
    pub fn receipt_bytes(&self) -> &[u8] {
        &self.receipt_bytes
    }

    #[must_use]
    pub const fn observed_head_sequence(&self) -> u64 {
        self.observed_head_sequence
    }
}

/// Submits one verified fund or revoke activity and confirms it from proven core state.
///
/// # Errors
///
/// Refuses an activity whose idempotency key is not the mutation key, a payload that does not
/// bind to `context`, an unverified or substituted receipt, a core rejection, and a resulting
/// state that is not the expected effect: a fund must leave the same live budget identity, a
/// revoke must leave the budget revoked at the signed counter with its limit set to the amount
/// already spent. `Submission` means the outcome is unknown and nothing may be released.
pub fn confirm_budget_mutation<R: BudgetReceiptEvidence, S>(
    pipeline: &mut dyn BudgetMutationPipeline<R, S>,
    verifier: &EvidenceAuthority,
    submission: &VerifiedSubmission,
    registry: &ModuleRegistry,
    context: &BudgetStateContext,
    mutation_key: [u8; 32],
) -> Result<ConfirmedBudgetMutation, BudgetCreationError>
where
    ProtocolBudgetState<S>: BudgetStateEvidence,
{
    if submission.idempotency_key() != mutation_key {
        return Err(BudgetCreationError::ActivityBindingMismatch);
    }
    let mutation = budget_mutation_identity(submission.exact_bytes(), registry, context)?;
    let receipt = pipeline.submit_verified(submission)?;
    let executed = committed_receipt(
        receipt.evidence.layerxpay_receipt()?,
        submission.activity_id(),
    )?;
    let (call, signature) = decode_signed(submission.exact_bytes())
        .map_err(|_| BudgetCreationError::NotBudgetCreation)?;
    if recover_sender(&call, &signature).map_err(|_| BudgetCreationError::NotBudgetCreation)?
        != executed.from
        || call.nonce != executed.nonce
    {
        return Err(BudgetCreationError::IdentityMismatch);
    }
    let event = budget_log(
        executed,
        match mutation {
            BudgetMutation::Fund { .. } => "BudgetFunded(bytes32,uint128)",
            BudgetMutation::Revoke { .. } => "BudgetRevoked(bytes32,uint128,uint64)",
        },
    )?;
    if event.topics[1] != mutation.budget_id() {
        return Err(BudgetCreationError::IdentityMismatch);
    }
    let state = pipeline.budget_state(mutation.budget_id())?;
    let (record, height) = state.verified_record(verifier)?;
    let same_identity = height == executed.height
        && height >= context.observed_head_sequence
        && record.budget_id == context.budget_id
        && record.owner == context.owner
        && record.budget_account == context.budget_account
        && record.asset_id == context.asset
        && record.purpose_hash == context.purpose_hash
        && record.source_account == context.native_source.then_some(context.source_account)
        && !record.closed;
    let balance = pipeline.budget_balance(record.budget_account, record.asset_id)?;
    let effect = match mutation {
        BudgetMutation::Fund { amount, .. } => {
            event.data.len() == 32
                && event.data[..16].iter().all(|byte| *byte == 0)
                && event.data[16..] == amount.to_be_bytes()
                && !record.revoked
                && record.revocation_sequence == context.revocation_sequence
                && context.balance.checked_add(amount) == Some(balance)
        }
        BudgetMutation::Revoke {
            revocation_height, ..
        } => {
            event.data.len() == 64
                && event.data[..16].iter().all(|byte| *byte == 0)
                && event.data[16..32] == context.balance.to_be_bytes()
                && event.data[32..56].iter().all(|byte| *byte == 0)
                && event.data[56..] == revocation_height.to_be_bytes()
                && revocation_height <= height
                && record.revoked
                && record.revocation_sequence == revocation_height
                && record.per_period_limit == record.spent_this_period
                && balance == 0
        }
    };
    if !same_identity || !effect {
        return Err(BudgetCreationError::CreatedBudgetUnconfirmed);
    }
    Ok(ConfirmedBudgetMutation {
        mutation,
        activity_id: submission.activity_id(),
        receipt_bytes: executed.receipt.clone(),
        observed_head_sequence: height,
        record,
    })
}

impl BudgetMutationPipeline<SequencerReceipt, super::accounting::CommittedBudgetEvidence>
    for SequencerBudgetPipeline
{
    fn submit_verified(
        &mut self,
        submission: &VerifiedSubmission,
    ) -> Result<CoreBudgetReceipt<SequencerReceipt>, BudgetCreationError> {
        self.submit(submission)
    }
    fn budget_state(
        &mut self,
        budget_id: [u8; 32],
    ) -> Result<ProtocolBudgetState<super::accounting::CommittedBudgetEvidence>, BudgetCreationError>
    {
        self.read(budget_id)
    }
    fn budget_balance(
        &mut self,
        account: [u8; 32],
        asset: [u8; 32],
    ) -> Result<u128, BudgetCreationError> {
        let height = self
            .committed_height
            .ok_or(BudgetCreationError::CreatedBudgetUnconfirmed)?;
        let mut data = selector("balanceOf(bytes32,bytes32)").to_vec();
        data.extend_from_slice(&account);
        data.extend_from_slice(&asset);
        let result = self
            .endpoint
            .call_at(height, &PrecompileTarget::Ledger.address(), &data)
            .map_err(|_| BudgetCreationError::CreatedBudgetUnconfirmed)?;
        if result.len() != 32 || result[..16].iter().any(|byte| *byte != 0) {
            return Err(BudgetCreationError::CreatedBudgetUnconfirmed);
        }
        Ok(u128::from_be_bytes(result[16..].try_into().map_err(
            |_| BudgetCreationError::CreatedBudgetUnconfirmed,
        )?))
    }
}
