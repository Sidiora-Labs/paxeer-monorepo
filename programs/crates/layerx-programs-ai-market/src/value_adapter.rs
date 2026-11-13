//! Value boundary between the F06 reward ledger and the registered
//! Program-derived rewards account. The ledger moves no funds; this adapter
//! turns one committed `RewardEffect` into at most one native 402LXP action
//! under an exact grant ceiling.
use crate::types::GuestIndex as _;
use layerx_program_sdk::{
    payments::PaymentGrant, Bytes, Capability, HostRefusal, ProgramAccountPayment, ProgramDeposit,
    ProgramError,
};

use crate::{
    errors::{
        ApplicationError, CodecResult, ACCOUNT_BINDING, ARITHMETIC, F06_INVALID_AMOUNT,
        F06_LEDGER_INVARIANT_VIOLATION, F06_REFUND_RECIPIENT_MISMATCH, F06_WRONG_ASSET,
        HOST_CAPABILITY, HOST_TRANSFER,
    },
    rewards::{RewardEffect, RewardLedger},
    types::{AccountId, Amount, AssetId, Digest32, ProgramId},
    REWARDS_ACCOUNT_SEED,
};

/// Runtime capability tag of one `BalanceView` sight grant.
pub const BALANCE_VIEW_TAG: u8 = 10;
/// `tag || account32 || asset32 || receipt_digest32`.
pub const BALANCE_SIGHT_BYTES: usize = 97;
/// `program32 || "LXPA1" || asset32 || u32 seed length || seed`.
pub const REGISTRATION_PAYLOAD_BYTES: usize = 32 + 5 + 32 + 4 + REWARDS_ACCOUNT_SEED.len();

/// The market's rewards account: the current Program, the fixed rewards seed
/// and the immutable reward asset, derived exactly as the runtime derives it.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RewardsAccount {
    program: ProgramId,
    account: AccountId,
    asset: AssetId,
}

impl RewardsAccount {
    /// # Errors
    /// `ACCOUNT_BINDING` when the SDK refuses the program, asset or derived account.
    pub fn derive(program: ProgramId, asset: AssetId) -> CodecResult<Self> {
        let account =
            crate::registry::derive_rewards_account(program, asset).map_err(|_| ACCOUNT_BINDING)?;
        Ok(Self {
            program,
            account,
            asset,
        })
    }

    /// Binds the ledger's immutable account and asset to this Program.
    ///
    /// # Errors
    /// `ACCOUNT_BINDING` when the ledger account is not this Program's derived rewards account.
    pub fn for_ledger(program: ProgramId, ledger: &RewardLedger) -> CodecResult<Self> {
        let bound = Self::derive(program, ledger.asset)?;
        bound.check(ledger)?;
        Ok(bound)
    }

    /// # Errors
    /// `F06_WRONG_ASSET` for another asset, `ACCOUNT_BINDING` for another account.
    pub fn check(&self, ledger: &RewardLedger) -> CodecResult<()> {
        if ledger.asset != self.asset {
            return Err(F06_WRONG_ASSET);
        }
        if ledger.account != self.account {
            return Err(ACCOUNT_BINDING);
        }
        Ok(())
    }

    #[must_use]
    pub const fn program(&self) -> ProgramId {
        self.program
    }

    #[must_use]
    pub const fn account(&self) -> AccountId {
        self.account
    }

    #[must_use]
    pub const fn asset(&self) -> AssetId {
        self.asset
    }

    /// Native `ProgramAccount` registration payload; submitting it requires the
    /// deployment's registration authority.
    ///
    /// # Errors
    /// `ACCOUNT_BINDING` when the SDK cannot encode the payload.
    pub fn registration_payload(&self) -> CodecResult<Bytes<201>> {
        crate::registry::prepare_rewards_account(self.program, self.asset)
            .map_err(|_| ACCOUNT_BINDING)?
            .registration_payload()
            .map_err(|_| ACCOUNT_BINDING)
    }

    /// `Transfer402` grant the funding principal signs: this asset, this
    /// account, ceiling exactly equal to the deposit.
    ///
    /// # Errors
    /// `F06_INVALID_AMOUNT` for a zero amount.
    pub fn funding_grant(&self, amount: Amount) -> CodecResult<Capability> {
        Capability::transfer(self.sdk_asset()?, self.sdk_account()?, sdk_amount(amount)?)
            .map_err(|_| F06_INVALID_AMOUNT)
    }

    /// `ProgramSpend` grant for one payout: this Program, seed, source and
    /// asset, the fixed recipient, ceiling exactly equal to the payout.
    ///
    /// # Errors
    /// `F06_INVALID_AMOUNT` for a zero amount, `ACCOUNT_BINDING` for a payout to the rewards account itself.
    pub fn spend_grant(
        &self,
        recipient: AccountId,
        amount: Amount,
    ) -> CodecResult<PaymentGrant<'static>> {
        let payment = self.payment(recipient, amount)?;
        Ok(PaymentGrant::ProgramSpend {
            account: crate::registry::prepare_rewards_account(self.program, self.asset)
                .map_err(|_| ACCOUNT_BINDING)?,
            to: payment.to(),
            maximum: payment.amount(),
        })
    }

    /// `BalanceView` sight grant of this account and asset at one finalized receipt.
    #[must_use]
    pub fn balance_sight(&self, receipt: Digest32) -> [u8; BALANCE_SIGHT_BYTES] {
        let mut out = [0; BALANCE_SIGHT_BYTES];
        *out.guest_index_mut(0) = BALANCE_VIEW_TAG;
        out.guest_index_mut(1..33)
            .guest_copy_from_slice(self.account.as_bytes());
        out.guest_index_mut(33..65)
            .guest_copy_from_slice(self.asset.as_bytes());
        out.guest_index_mut(65..)
            .guest_copy_from_slice(receipt.as_bytes());
        out
    }

    /// # Errors
    /// `F06_INVALID_AMOUNT` for a zero amount.
    pub fn deposit(&self, amount: Amount) -> CodecResult<ProgramDeposit<'static>> {
        ProgramDeposit::new(
            Self::seed()?,
            self.sdk_account()?,
            self.sdk_asset()?,
            sdk_amount(amount)?,
        )
        .map_err(|_| F06_INVALID_AMOUNT)
    }

    /// # Errors
    /// `F06_INVALID_AMOUNT` for a zero amount, `ACCOUNT_BINDING` for a payout to the rewards account itself.
    pub fn payment(
        &self,
        recipient: AccountId,
        amount: Amount,
    ) -> CodecResult<ProgramAccountPayment<'static>> {
        if recipient == self.account {
            return Err(ACCOUNT_BINDING);
        }
        let to =
            layerx_program_sdk::AccountId::new(recipient.bytes()).map_err(|_| ACCOUNT_BINDING)?;
        ProgramAccountPayment::new(
            Self::seed()?,
            self.sdk_account()?,
            self.sdk_asset()?,
            to,
            sdk_amount(amount)?,
        )
        .map_err(|_| F06_INVALID_AMOUNT)
    }
    fn seed() -> CodecResult<layerx_program_sdk::ProgramAccountSeed<'static>> {
        layerx_program_sdk::ProgramAccountSeed::new(REWARDS_ACCOUNT_SEED)
            .map_err(|_| ACCOUNT_BINDING)
    }

    fn sdk_account(&self) -> CodecResult<layerx_program_sdk::AccountId> {
        layerx_program_sdk::AccountId::new(self.account.bytes()).map_err(|_| ACCOUNT_BINDING)
    }

    fn sdk_asset(&self) -> CodecResult<layerx_program_sdk::AssetId> {
        layerx_program_sdk::AssetId::new(self.asset.bytes()).map_err(|_| ACCOUNT_BINDING)
    }
}

fn sdk_amount(amount: Amount) -> CodecResult<layerx_program_sdk::Amount> {
    if amount == 0 {
        Err(F06_INVALID_AMOUNT)
    } else {
        Ok(layerx_program_sdk::Amount::from_u128(amount))
    }
}

/// The one native action a committed reward transition stages.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum ValueAction {
    None,
    Fund {
        amount: Amount,
    },
    Pay {
        recipient: AccountId,
        amount: Amount,
    },
}

impl ValueAction {
    /// # Errors
    /// `F06_INVALID_AMOUNT` for a zero deposit or payout.
    pub fn from_effect(effect: RewardEffect) -> CodecResult<Self> {
        match effect {
            RewardEffect::Deposit { amount, .. } => {
                sdk_amount(amount)?;
                Ok(Self::Fund { amount })
            }
            RewardEffect::Payout { recipient, amount } => {
                sdk_amount(amount)?;
                Ok(Self::Pay { recipient, amount })
            }
            RewardEffect::NoTransfer
            | RewardEffect::Released(_)
            | RewardEffect::Pruned(_)
            | RewardEffect::AlreadyApplied(_)
            | RewardEffect::ReplayedFund(_)
            | RewardEffect::RepeatedRefund(_) => Ok(Self::None),
        }
    }
}

/// Checks that one committed transition and its effect agree before anything
/// is staged: same bound account and asset, both ledgers conserved, and the
/// physical counters D, P and X move by exactly the staged amount.
///
/// # Errors
/// `F06_WRONG_ASSET` or `ACCOUNT_BINDING` for a foreign ledger; `F06_REFUND_RECIPIENT_MISMATCH` when a refund leaves for another recipient; `ARITHMETIC` on overflow; `F06_LEDGER_INVARIANT_VIOLATION` when the counters disagree with the action; any ledger validation error.
pub fn plan(
    bound: &RewardsAccount,
    prev: &RewardLedger,
    next: &RewardLedger,
    effect: RewardEffect,
) -> CodecResult<ValueAction> {
    bound.check(prev)?;
    bound.check(next)?;
    if prev.refund_recipient != next.refund_recipient {
        return Err(F06_REFUND_RECIPIENT_MISMATCH);
    }
    prev.validate()?;
    next.validate()?;
    let action = ValueAction::from_effect(effect)?;
    let (deposited, claimed, refunded) = match action {
        ValueAction::None => (0, 0, 0),
        ValueAction::Fund { amount } => (amount, 0, 0),
        ValueAction::Pay { recipient, amount } => {
            bound.payment(recipient, amount)?;
            if next.tracked_refunds == prev.tracked_refunds {
                (0, amount, 0)
            } else if recipient == prev.refund_recipient {
                (0, 0, amount)
            } else {
                return Err(F06_REFUND_RECIPIENT_MISMATCH);
            }
        }
    };
    let moved = |before: Amount, delta: Amount, after: Amount| {
        before
            .checked_add(delta)
            .ok_or(ARITHMETIC)
            .map(|expected| expected == after)
    };
    if moved(prev.tracked_deposits, deposited, next.tracked_deposits)?
        && moved(prev.total_claimed, claimed, next.total_claimed)?
        && moved(prev.tracked_refunds, refunded, next.tracked_refunds)?
    {
        Ok(action)
    } else {
        Err(F06_LEDGER_INVARIANT_VIOLATION)
    }
}

/// State audit law: the observed balance plus the staged deposit, or minus the
/// staged payout, must cover `F + R + C` of the next ledger. No operation
/// enforces this law. Returns the unsolicited surplus `U` for an audit.
///
/// # Errors
/// `ARITHMETIC` on overflow; `F06_LEDGER_INVARIANT_VIOLATION` when the physical balance cannot cover the ledger.
pub fn cover(next: &RewardLedger, observed: Amount, action: ValueAction) -> CodecResult<Amount> {
    let post = match action {
        ValueAction::None => Some(observed),
        ValueAction::Fund { amount } => Some(observed.checked_add(amount).ok_or(ARITHMETIC)?),
        ValueAction::Pay { amount, .. } => observed.checked_sub(amount),
    }
    .ok_or(F06_LEDGER_INVARIANT_VIOLATION)?;
    let required = next
        .free
        .checked_add(next.reserved)
        .and_then(|sum| sum.checked_add(next.liability))
        .ok_or(ARITHMETIC)?;
    post.checked_sub(required)
        .ok_or(F06_LEDGER_INVARIANT_VIOLATION)
}

/// `TransferAuthorityRefused` is `HOST_CAPABILITY`; every other host refusal
/// is `TransferFailed`; a refused construction is `F06_INVALID_AMOUNT`.
#[must_use]
pub const fn transfer_refusal(error: ProgramError) -> ApplicationError {
    match error {
        ProgramError::Host(HostRefusal::Denied) => HOST_CAPABILITY,
        ProgramError::Host(_) => HOST_TRANSFER,
        ProgramError::Value(_) => F06_INVALID_AMOUNT,
    }
}

/// Applies one committed reward transition's value effect inside the same
/// Program activity: plan, then stage at most one funding or Program-account
/// payment under its spending grant.
///
/// # Errors
/// Every `plan` refusal and `transfer_refusal` of the staged transfer.
#[cfg(target_arch = "wasm32")]
pub fn settle(
    bound: &RewardsAccount,
    prev: &RewardLedger,
    next: &RewardLedger,
    effect: RewardEffect,
) -> CodecResult<()> {
    let action = plan(bound, prev, next, effect)?;
    match action {
        ValueAction::None => {}
        ValueAction::Fund { amount } => {
            layerx_program_sdk::transfer::fund_program_account(bound.deposit(amount)?)
                .map_err(transfer_refusal)?;
        }
        ValueAction::Pay { recipient, amount } => {
            layerx_program_sdk::transfer::pay_from_program_account(
                bound.payment(recipient, amount)?,
            )
            .map_err(transfer_refusal)?;
        }
    }
    Ok(())
}
