mod support;

use layerx_crypto::disclosure::call::{bind_call, ConstructMethod};
use layerx_crypto::disclosure::DisclosureError;
use layerx_crypto::send::{ledger_payment_call, CallOptions, DisclosedCall};
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
        let mut call = ledger_payment_call(&payload, &options)
            .expect("valid payment")
            .call;
        call.data.truncate(68);
        call.data[52..68].copy_from_slice(&(malformed.len() as u128).to_be_bytes());
        call.data.extend_from_slice(malformed);
        call.data.resize(68 + malformed.len().div_ceil(32) * 32, 0);
        assert!(DisclosedCall::from_call(call, options.from).is_err());
        assert!(matches!(
            ledger_payment_call(malformed, &options),
            Err(DisclosureError::MalformedPayload)
        ));
    }
}

#[test]
fn send_envelope_encoder_keeps_asset_ordinal_five() -> Result<(), Box<dyn std::error::Error>> {
    use ed25519_dalek::{Signer as _, SigningKey};
    use layerx_crypto::evm_transaction::{assemble, decode_signed, recover_sender};
    use layerx_crypto::send::{send_context_hash, SendCondition, SendDebit};
    use layerx_crypto::signer::{sign_disclosed_call, Signer};
    use layerx_crypto::SignatureMessage;
    use layerx_types::payload::ModuleId;
    use layerx_wire::hash::Domain;

    let key = SigningKey::from_bytes(&[5; 32]);
    let account = layerx_crypto::account_derivation::derive_from_seed(&[5; 64], 0)?;
    let signer = layerx_crypto::signer::LocalSigner::from_derived_account(&account);
    let mut options = options();
    options.from = signer.evm_address().expect("fixture EVM key");
    let debit = SendDebit {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount: 123,
        source_sequence: 9,
        idempotency_key: [4; 32],
        expires_at: 100,
        context_hash: send_context_hash(&[1; 32], &[2; 32], &[3; 32], 123, &[4; 32]),
        conditions: vec![SendCondition {
            kind: 1,
            timestamp: 10,
        }],
        authorization_kind: 1,
        network_id: 17,
        protocol_version: 3,
    };
    let authorization = debit.authorization_message()?;
    let message = SignatureMessage::new(
        Domain::SignaturePreimage,
        debit.protocol_version,
        debit.network_id,
        &authorization,
    )
    .map_err(|error| format!("{error:?}"))?;
    let payload = debit.encode_signed(
        key.verifying_key().to_bytes(),
        key.sign(&message.digest()).to_bytes(),
    )?;
    let disclosed = ledger_payment_call(&payload, &options)?;
    let signature = support::complete(sign_disclosed_call(
        &signer,
        &disclosed.call,
        &disclosed.disclosure,
        options.from,
    ))?;
    let canonical = assemble(&disclosed.call, &signature);
    let (decoded, signature) = decode_signed(&canonical)?;
    let sender = recover_sender(&decoded, &signature)?;
    assert_eq!(sender, options.from);
    let disclosure = bind_call(&decoded, sender)?;
    assert_eq!(disclosure.activity_type.module(), ModuleId::Asset);
    assert_eq!(disclosure.activity_type.ordinal(), 5);
    assert_eq!(
        disclosure.construct_method()?,
        Some(ConstructMethod::LedgerSend { payload })
    );
    assert_eq!(decoded.nonce, options.nonce);
    assert_eq!(disclosure.payload_sequence()?, Some(9));
    Ok(())
}
