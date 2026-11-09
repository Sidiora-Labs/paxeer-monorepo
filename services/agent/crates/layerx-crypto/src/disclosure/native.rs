use super::{DisclosureError, Encoder};

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedNativeIdentity {
    pub did_id: [u8; 32],
    pub primary_key: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedRecoveryPolicy {
    pub did_id: [u8; 32],
    pub recovery_root: [u8; 32],
    pub threshold: u16,
    pub delay_bounds: Option<(u64, u64)>,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedNativeBudgetCreate {
    pub encoding_version: u16,
    pub budget_id: [u8; 32],
    pub budget_account: [u8; 32],
    pub asset: [u8; 32],
    pub purpose: [u8; 32],
    pub per_period_limit: u128,
    pub carry_cap: u128,
    pub initial_amount: u128,
    pub period_length_ms: u64,
    pub period_start_ms: u64,
    pub expiry_ms: u64,
    pub revocation_sequence: u64,
    pub rollover: u8,
    pub source_account: [u8; 32],
    pub source_sequence: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum DisclosedNativeOperation {
    ProgramDeploy(Box<super::DisclosedProgramDeploy>),
    ProgramUpgrade(Box<super::DisclosedProgramUpgrade>),
    ProgramCall(Box<super::DisclosedProgramCall>),
    ProgramWindDown(Box<super::DisclosedProgramWindDown>),
    LegacyProgramCall(Box<layerx_types::intent::ProgramCall>),
    IdentityRegistration(DisclosedNativeIdentity),
    RecoveryPolicy(DisclosedRecoveryPolicy),
    BudgetCreate(Box<DisclosedNativeBudgetCreate>),
    BudgetFund(Box<DisclosedNativeBudgetFund>),
    BudgetDefund(Box<DisclosedNativeBudgetDefund>),
    BudgetRevoke(Box<DisclosedNativeBudgetRevoke>),
}

/// Verified budget record context a fund, defund or revoke disclosure is bound to.
///
/// The caller supplies values it verified from proven module and account state.
/// The disclosure checks them against what the canonical bytes and the actor
/// derive, and every field is bound into the transport bytes and audit digest.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct BudgetStateContext {
    pub budget_id: [u8; 32],
    pub owner: [u8; 32],
    pub budget_account: [u8; 32],
    pub asset: [u8; 32],
    pub source_account: [u8; 32],
    pub native_source: bool,
    pub revocation_sequence: u64,
    pub balance: u128,
    pub state_digest: [u8; 32],
    pub observed_head_sequence: u64,
    pub purpose_hash: [u8; 32],
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedNativeBudgetFund {
    pub encoding_version: u16,
    pub budget_id: [u8; 32],
    pub amount: u128,
    pub source_sequence: Option<u64>,
    pub context: BudgetStateContext,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedNativeBudgetDefund {
    pub budget_id: [u8; 32],
    pub amount: u128,
    pub context: BudgetStateContext,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct DisclosedNativeBudgetRevoke {
    pub budget_id: [u8; 32],
    pub revocation_sequence: u64,
    pub context: BudgetStateContext,
}

impl BudgetStateContext {
    #[must_use]
    pub fn purpose_matches(&self, supplied: &[u8; 32]) -> bool {
        crate::ct::eq_fixed(&self.purpose_hash, supplied)
    }
}

fn encode_context(
    encoder: &mut Encoder,
    context: &BudgetStateContext,
) -> Result<(), DisclosureError> {
    encoder.fixed(&context.budget_id)?;
    encoder.fixed(&context.owner)?;
    encoder.fixed(&context.budget_account)?;
    encoder.fixed(&context.asset)?;
    encoder.fixed(&context.source_account)?;
    encoder.u8(u8::from(context.native_source))?;
    encoder.u64(context.revocation_sequence)?;
    encoder.u128(context.balance)?;
    encoder.fixed(&context.state_digest)?;
    encoder.u64(context.observed_head_sequence)?;
    encoder.fixed(&context.purpose_hash)?;
    Ok(())
}

impl DisclosedNativeOperation {
    /// # Errors
    /// Refuses disclosure fields that exceed the canonical encoding limits.
    pub fn encode(&self) -> Result<Vec<u8>, DisclosureError> {
        let mut encoder = Encoder::new(2048);
        match self {
            Self::ProgramDeploy(value) => value.encode_audit(&mut encoder)?,
            Self::ProgramUpgrade(value) => value.encode_audit(&mut encoder)?,
            Self::ProgramCall(value) => value.encode_audit(&mut encoder)?,
            Self::ProgramWindDown(value) => value.encode_audit(&mut encoder)?,
            Self::LegacyProgramCall(value) => super::program::encode_legacy(value, &mut encoder)?,
            Self::IdentityRegistration(identity) => {
                encoder.u8(1)?;
                encoder.fixed(&identity.did_id)?;
                encoder.fixed(&identity.primary_key)?;
            }
            Self::RecoveryPolicy(policy) => {
                encoder.u8(3)?;
                encoder.fixed(&policy.did_id)?;
                encoder.fixed(&policy.recovery_root)?;
                encoder.u16(policy.threshold)?;
                if let Some((minimum, maximum)) = policy.delay_bounds {
                    encoder.u8(1)?;
                    encoder.u64(minimum)?;
                    encoder.u64(maximum)?;
                } else {
                    encoder.u8(0)?;
                }
            }
            Self::BudgetCreate(budget) => {
                encoder.u8(4)?;
                encoder.u16(budget.encoding_version)?;
                encoder.fixed(&budget.budget_id)?;
                encoder.fixed(&budget.budget_account)?;
                encoder.fixed(&budget.asset)?;
                encoder.fixed(&budget.purpose)?;
                encoder.u128(budget.per_period_limit)?;
                encoder.u128(budget.carry_cap)?;
                encoder.u128(budget.initial_amount)?;
                encoder.u64(budget.period_length_ms)?;
                encoder.u64(budget.period_start_ms)?;
                encoder.u64(budget.expiry_ms)?;
                encoder.u64(budget.revocation_sequence)?;
                encoder.u8(budget.rollover)?;
                encoder.fixed(&budget.source_account)?;
                encoder.u64(budget.source_sequence)?;
            }
            Self::BudgetFund(fund) => {
                encoder.u8(6)?;
                encoder.u16(fund.encoding_version)?;
                encoder.fixed(&fund.budget_id)?;
                encoder.u128(fund.amount)?;
                match fund.source_sequence {
                    Some(sequence) => {
                        encoder.u8(1)?;
                        encoder.u64(sequence)?;
                    }
                    None => encoder.u8(0)?,
                }
                encode_context(&mut encoder, &fund.context)?;
            }
            Self::BudgetDefund(defund) => {
                encoder.u8(7)?;
                encoder.fixed(&defund.budget_id)?;
                encoder.u128(defund.amount)?;
                encode_context(&mut encoder, &defund.context)?;
            }
            Self::BudgetRevoke(revoke) => {
                encoder.u8(8)?;
                encoder.fixed(&revoke.budget_id)?;
                encoder.u64(revoke.revocation_sequence)?;
                encode_context(&mut encoder, &revoke.context)?;
            }
        }
        Ok(encoder.finish())
    }
}

pub(crate) fn budget_create_terms(
    identifiers: [&[u8; 32]; 3],
    limits: [u128; 3],
    timing: [u64; 3],
    rollover: u8,
) -> Result<(), DisclosureError> {
    let [budget_id, budget_account, asset] = identifiers;
    let [per_period_limit, carry_cap, initial_amount] = limits;
    let [period_length_ms, period_start_ms, expiry_ms] = timing;
    if *budget_id == [0; 32]
        || *budget_account == [0; 32]
        || *asset == [0; 32]
        || initial_amount == 0
        || per_period_limit == 0
        || period_length_ms == 0
        || expiry_ms <= period_start_ms
        || !matches!(rollover, 1 | 2)
        || (rollover == 1 && carry_cap != 0)
    {
        return Err(DisclosureError::MalformedPayload);
    }
    Ok(())
}
