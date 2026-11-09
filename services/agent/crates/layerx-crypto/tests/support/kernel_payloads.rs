#![allow(dead_code)]

use super::assertions;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_crypto::send::{SendCondition, SendDebit};
use layerx_crypto::SignatureMessage;
use layerx_wire::encode::Encoder;
use layerx_wire::hash::Domain;
use sha2::{Digest as _, Sha256};

use super::{EXPIRES_AT, IDEMPOTENCY_KEY, NETWORK_ID, NOT_BEFORE, PROTOCOL_VERSION, SEQUENCE};

pub fn send_debit(amount: u128) -> SendDebit {
    SendDebit {
        from: [0x11; 32],
        to: [0x22; 32],
        asset: [0x33; 32],
        amount,
        source_sequence: SEQUENCE,
        idempotency_key: IDEMPOTENCY_KEY,
        expires_at: EXPIRES_AT,
        context_hash: [0x44; 32],
        conditions: vec![
            SendCondition {
                kind: 1,
                timestamp: NOT_BEFORE,
            },
            SendCondition {
                kind: 2,
                timestamp: EXPIRES_AT,
            },
        ],
        authorization_kind: 1,
        network_id: NETWORK_ID,
        protocol_version: PROTOCOL_VERSION,
    }
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn send(amount: u128) -> (Vec<u8>, [u8; 32]) {
    let key = SigningKey::from_bytes(&[0x53; 32]);
    let public_key = key.verifying_key().to_bytes();
    let debit = send_debit(amount);
    let canonical =
        assertions::require(debit.authorization_message(), "kernel debit authorization");
    let message = assertions::require(
        SignatureMessage::new(
            Domain::SignaturePreimage,
            PROTOCOL_VERSION,
            NETWORK_ID,
            &canonical,
        ),
        "kernel debit scope",
    );
    let signature = key.sign(&message.digest()).to_bytes();
    (
        assertions::require(
            debit.encode_signed(public_key, signature),
            "kernel send payload",
        ),
        public_key,
    )
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn recorded_send() -> Vec<u8> {
    let text = include_str!("../fixtures/payments/native-1-5.hex").trim();
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            assertions::require(
                u8::from_str_radix(
                    assertions::require(std::str::from_utf8(pair), "fixture UTF-8"),
                    16,
                ),
                "fixture hex",
            )
        })
        .collect()
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn bridge_withdraw(amount: u128) -> Vec<u8> {
    let mut payload = Encoder::new(512);
    assert!(payload.u16(0x4802).is_ok());
    assert!(payload.u16(7).is_ok());
    assert!(payload.fixed(&[0x41; 32]).is_ok());
    assert!(payload.fixed(&[0x11; 32]).is_ok());
    assert!(payload.fixed(&[0x22; 32]).is_ok());
    assert!(payload.fixed(&[0x44; 20]).is_ok());
    assert!(payload.fixed(&[0x33; 32]).is_ok());
    assert!(payload.u128(amount).is_ok());
    assert!(payload.fixed(&IDEMPOTENCY_KEY).is_ok());
    payload.finish()
}
pub fn bridge_credit(amount: u128) -> Vec<u8> {
    let mut payload = vec![0; 368];
    payload[..5].copy_from_slice(b"LXDC3");
    payload[5..37].fill(0x42);
    payload[37..41].copy_from_slice(&NETWORK_ID.to_be_bytes());
    payload[41..43].copy_from_slice(&PROTOCOL_VERSION.to_be_bytes());
    payload[43..75].fill(0x41);
    payload[75..107].fill(0x33);
    payload[107..139].fill(0x22);
    payload[171..191].fill(0x11);
    payload[191..207].copy_from_slice(&amount.to_be_bytes());
    payload[359..363].copy_from_slice(&2_u32.to_be_bytes());
    payload[363..368].copy_from_slice(b"LXLB1");
    let bundle_hash = Sha256::digest(&payload[363..]);
    payload[327..359].copy_from_slice(&bundle_hash);
    payload
}
