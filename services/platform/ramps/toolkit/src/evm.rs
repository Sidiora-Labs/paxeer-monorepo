use layerx_client::sequencer::SequencerLog;
use sha3::{Digest as _, Keccak256};

/// Native ledger destination; payment logs from any other emitter are refused.
pub const LEDGER_ADDRESS: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];

/// Maintained ordinary-principal payment authority.
pub const PAY_ADDRESS: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1b,
];

#[must_use]
pub fn encode_budget_spend(budget: [u8; 32], recipient: [u8; 32], amount: u128) -> Vec<u8> {
    encode_call(
        "budgetSpend(bytes32,bytes32,uint128)",
        &[budget, recipient, uint_word(amount.to_be_bytes())],
    )
}

#[must_use]
pub fn encode_get_budget(budget: [u8; 32]) -> Vec<u8> {
    encode_call("getBudget(bytes32)", &[budget])
}

/// Exact native `BudgetSpent` event; the budget state supplies owner and asset.
#[must_use]
pub fn decode_budget_spent(log: &SequencerLog) -> Option<([u8; 32], [u8; 32], u128)> {
    let [topic, budget] = log.topics.as_slice() else {
        return None;
    };
    if log.address != PAY_ADDRESS
        || *topic != <[u8; 32]>::from(Keccak256::digest(b"BudgetSpent(bytes32,bytes32,uint128)"))
        || log.data.len() != 64
        || log.data[32..48].iter().any(|byte| *byte != 0)
    {
        return None;
    }
    Some((
        *budget,
        log.data[..32].try_into().ok()?,
        u128::from_be_bytes(log.data[48..64].try_into().ok()?),
    ))
}

const PAY_EXACT_SIGNATURE: &str = "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)";
const DRAW_SIGNATURE: &str = "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)";
const PAYMENT_EXACT_SIGNATURE: &str = "PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)";
const PAYMENT_DRAWN_SIGNATURE: &str =
    "PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)";

/// Owned payment fields decoded from the native ledger event ABI.
/// Decoding establishes the event shape, not receipt authenticity.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum LedgerPayment {
    Exact {
        from: [u8; 32],
        to: [u8; 32],
        reference: [u8; 32],
        asset: [u8; 32],
        amount: u128,
    },
    Drawn {
        from: [u8; 32],
        to: [u8; 32],
        grant: [u8; 32],
        asset: [u8; 32],
        amount: u128,
        idempotency_key: [u8; 32],
        context: [u8; 32],
    },
}

/// Encodes the six native `payExact` arguments in ABI order.
#[must_use]
pub fn encode_pay_exact(
    from: [u8; 32],
    to: [u8; 32],
    asset: [u8; 32],
    amount: u128,
    expires_at: u64,
    reference: [u8; 32],
) -> Vec<u8> {
    encode_call(
        PAY_EXACT_SIGNATURE,
        &[
            from,
            to,
            asset,
            uint_word(amount.to_be_bytes()),
            uint_word(expires_at.to_be_bytes()),
            reference,
        ],
    )
}

/// Historical codec only: the native draw execution route is retired.
/// Encodes the seven legacy `draw` arguments in ABI order. The asset and
/// payer are determined by the grant; the second argument is the recipient.
#[must_use]
pub fn encode_draw(
    grant_id: [u8; 32],
    recipient: [u8; 32],
    amount: u128,
    idempotency_key: [u8; 32],
    sequence: u64,
    purpose_hash: [u8; 32],
    context_hash: [u8; 32],
) -> Vec<u8> {
    encode_call(
        DRAW_SIGNATURE,
        &[
            grant_id,
            recipient,
            uint_word(amount.to_be_bytes()),
            idempotency_key,
            uint_word(sequence.to_be_bytes()),
            purpose_hash,
            context_hash,
        ],
    )
}

fn encode_call(signature: &str, words: &[[u8; 32]]) -> Vec<u8> {
    let mut data = Vec::with_capacity(4 + words.len() * 32);
    data.extend_from_slice(&Keccak256::digest(signature.as_bytes())[..4]);
    for word in words {
        data.extend_from_slice(word);
    }
    data
}

fn uint_word<const N: usize>(bytes: [u8; N]) -> [u8; 32] {
    let mut word = [0; 32];
    word[32 - N..].copy_from_slice(&bytes);
    word
}

/// Decodes one canonical `PaymentExact` or `PaymentDrawn` log at 0x101a.
/// Refuses an unknown emitter or signature, extra or missing topics/data,
/// and amounts that exceed uint128. Every call reads the supplied log anew.
#[must_use]
pub fn decode_ledger_payment(log: &SequencerLog) -> Option<LedgerPayment> {
    if log.address != LEDGER_ADDRESS {
        return None;
    }
    let [topic, first, second, third] = log.topics.as_slice() else {
        return None;
    };
    let exact_topic: [u8; 32] = Keccak256::digest(PAYMENT_EXACT_SIGNATURE.as_bytes()).into();
    let drawn_topic: [u8; 32] = Keccak256::digest(PAYMENT_DRAWN_SIGNATURE.as_bytes()).into();
    let exact = if *topic == exact_topic && log.data.len() == 64 {
        true
    } else if *topic == drawn_topic && log.data.len() == 128 {
        false
    } else {
        return None;
    };
    if log.data[32..48].iter().any(|byte| *byte != 0) {
        return None;
    }
    let asset = log.data[..32].try_into().ok()?;
    let amount = u128::from_be_bytes(log.data[48..64].try_into().ok()?);
    if exact {
        Some(LedgerPayment::Exact {
            from: *second,
            to: *third,
            reference: *first,
            asset,
            amount,
        })
    } else {
        Some(LedgerPayment::Drawn {
            grant: *first,
            from: *second,
            to: *third,
            asset,
            amount,
            idempotency_key: log.data[64..96].try_into().ok()?,
            context: log.data[96..128].try_into().ok()?,
        })
    }
}

/// Validates the maintained getBudget ABI against the bound customer order.
/// Native execution requires the customer's own ordinary-principal authority.
/// # Errors
/// Returns an error when the budget ABI or its bound order values are invalid.
pub fn verify_customer_budget(
    order: &crate::RampOrder,
    bytes: &[u8],
    live: bool,
) -> Result<(), crate::RampError> {
    use crate::RampError;
    let auth = order.customer_payment_authorization()?;
    let word = |index: usize| -> Result<[u8; 32], RampError> {
        bytes
            .get(index * 32..(index + 1) * 32)
            .ok_or(RampError::ReceiptMismatch)?
            .try_into()
            .map_err(|_| RampError::ReceiptMismatch)
    };
    let uint = |index: usize| -> Result<u64, RampError> {
        let value = word(index)?;
        if value[..24] != [0; 24] {
            return Err(RampError::ReceiptMismatch);
        }
        Ok(u64::from_be_bytes(
            value[24..]
                .try_into()
                .map_err(|_| RampError::ReceiptMismatch)?,
        ))
    };
    // One dynamic tuple: 18 fixed words followed by its bounded delegates.
    if uint(0)? != 32 || uint(17)? != 18 * 32 {
        return Err(RampError::ReceiptMismatch);
    }
    let count = usize::try_from(uint(19)?).map_err(|_| RampError::ReceiptMismatch)?;
    if count > 16
        || bytes.len() != (20 + count) * 32
        || word(1)? != crate::ledger_account(&order.customer.account)?
        || word(3)? != order.quote.layerx_asset
        || word(4)? != auth.purpose_hash
    {
        return Err(RampError::ReceiptMismatch);
    }
    if live
        && (uint(15)? != 0
            || uint(16)? != 0
            || uint(12)?
                != order
                    .quote
                    .expires_at
                    .checked_mul(1_000)
                    .ok_or(RampError::InvalidOrder)?)
    {
        return Err(RampError::ReceiptMismatch);
    }
    Ok(())
}
