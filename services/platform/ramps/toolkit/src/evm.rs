use layerx_client::sequencer::SequencerLog;
use sha3::{Digest as _, Keccak256};

pub const LEDGER_ADDRESS: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];

const PAY_EXACT_SIGNATURE: &str = "payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)";
const DRAW_SIGNATURE: &str = "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)";
const PAYMENT_EXACT_SIGNATURE: &str = "PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)";
const PAYMENT_DRAWN_SIGNATURE: &str =
    "PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)";

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

#[must_use]
pub fn encode_draw(
    from: [u8; 32],
    asset: [u8; 32],
    amount: u128,
    grant: [u8; 32],
    receiver_sequence: u64,
    idempotency_key: [u8; 32],
    context: [u8; 32],
) -> Vec<u8> {
    encode_call(
        DRAW_SIGNATURE,
        &[
            from,
            asset,
            uint_word(amount.to_be_bytes()),
            grant,
            uint_word(receiver_sequence.to_be_bytes()),
            idempotency_key,
            context,
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
