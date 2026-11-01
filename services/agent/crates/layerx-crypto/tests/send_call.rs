use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::send::{
    encode_send_envelope, ledger_payment_call, CallOptions, EnvelopeOptions,
};
use sha3::{Digest as _, Keccak256};

fn payload() -> Vec<u8> {
    include_str!("fixtures/payments/native-1-5.hex")
        .trim()
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            u8::from_str_radix(std::str::from_utf8(pair).expect("fixture UTF-8"), 16)
                .expect("fixture hex")
        })
        .collect()
}

fn options() -> CallOptions {
    CallOptions {
        from: [0xa1; 20],
        chain_id: 125,
        nonce: 7,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    }
}

#[test]
fn ledger_payment_call_wraps_the_send_payload_in_send_bytes(
) -> Result<(), Box<dyn std::error::Error>> {
    let payload = payload();
    let options = options();
    let disclosed = ledger_payment_call(&payload, &options)?;
    let call = &disclosed.call;
    let mut target = [0; 20];
    target[18] = 0x10;
    target[19] = 0x1a;
    assert_eq!(call.to, target);
    assert_eq!(call.chain_id, 125);
    assert_eq!(call.nonce, 7);
    assert_eq!(call.max_priority_fee_per_gas, 2);
    assert_eq!(call.max_fee_per_gas, 30);
    assert_eq!(call.gas_limit, 100_000);
    assert_eq!(&call.data[..4], &Keccak256::digest(b"send(bytes)")[..4]);
    assert_eq!(&call.data[4..35], &[0; 31]);
    assert_eq!(call.data[35], 32);
    assert_eq!(&call.data[36..52], &[0; 16]);
    let length = u128::from_be_bytes(call.data[52..68].try_into()?);
    assert_eq!(length, payload.len() as u128);
    assert_eq!(&call.data[68..68 + payload.len()], payload.as_slice());
    assert_eq!(call.data.len(), 68 + payload.len().div_ceil(32) * 32);
    assert!(call.data[68 + payload.len()..]
        .iter()
        .all(|byte| *byte == 0));
    assert_eq!(disclosed.disclosure, bind_call(call, options.from)?);
    assert_eq!(disclosed.disclosure.actor, options.from);
    assert_eq!(disclosed.disclosure.signing_digest()?, call.signing_hash());
    assert_eq!(
        disclosed.disclosure.construct_method()?,
        Some(ConstructMethod::LedgerSend { payload })
    );
    Ok(())
}

#[test]
fn ledger_payment_call_refuses_a_malformed_payload() {
    let payload = payload();
    let options = options();
    let envelope_options = EnvelopeOptions {
        actor: "did:layerx:alice",
        public_key: [0; 32],
        protocol_version: 3,
        network_id: 17,
        identity_sequence: 7,
        idempotency_key: [4; 32],
        fee_limit: 20,
        not_before: 10,
        not_after: 100,
    };
    let mut trailing = payload.clone();
    trailing.push(0);
    let mut invalid_signature = payload.clone();
    invalid_signature[4] ^= 1;
    for malformed in [
        &payload[..payload.len() - 1],
        &[],
        trailing.as_slice(),
        invalid_signature.as_slice(),
    ] {
        assert!(encode_send_envelope(malformed, &envelope_options).is_err());
        assert!(matches!(
            ledger_payment_call(malformed, &options),
            Err(DisclosureError::MalformedPayload)
        ));
    }
}
