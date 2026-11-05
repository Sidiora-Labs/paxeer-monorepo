#[path = "payments.rs"]
mod payments;

use layerx_crypto::disclosure::call::{bind_call, reencode, ConstructMethod};
use layerx_crypto::evm_transaction::{assemble, decode_signed, recover_sender};
use layerx_crypto::payments::{
    ledger_payment_call, send_context_hash, DisclosedCall, SendCondition, SendDebit,
};
use layerx_crypto::signer::{sign_disclosed_call, SignError};

#[test]
fn payment_tests_old_signing_path_recovers_sender_and_refuses_changed_calls(
) -> Result<(), Box<dyn std::error::Error>> {
    let signer = payments::signer()?;
    let options = payments::options(&signer);
    let debit = SendDebit {
        from: [1; 32],
        to: [2; 32],
        asset: [3; 32],
        amount: 123,
        source_sequence: 23,
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
    let payload = payments::complete(debit.sign(&signer))?;
    let produced = ledger_payment_call(&payload, &options)?;
    let consumed = DisclosedCall::from_call(produced.call.clone(), options.from)?;
    let reviewed = bind_call(&consumed.call, options.from)?;
    assert_eq!(reviewed, produced.disclosure);
    assert_eq!(reviewed, consumed.disclosure);
    assert_eq!(reviewed.actor, options.from);
    assert_eq!(reviewed.asset, debit.asset);
    assert_eq!(reviewed.counterparties.len(), 2);
    assert_eq!(reviewed.counterparties[0].account, debit.from);
    assert_eq!(reviewed.counterparties[1].account, debit.to);
    assert_eq!(reviewed.amounts[0].value, debit.amount);
    assert_eq!(reviewed.expiry.payload_expires_at, debit.expires_at);
    assert_eq!(reviewed.idempotency_key, debit.idempotency_key);
    assert_eq!(reviewed.envelope_sequence(), options.nonce);
    assert_eq!(reviewed.payload_sequence()?, Some(debit.source_sequence));
    assert_eq!(
        reviewed.fee_limit,
        u128::from(options.gas_limit) * options.max_fee_per_gas
    );
    assert_eq!(reviewed.signing_digest()?, consumed.call.signing_hash());
    assert_eq!(reencode(&reviewed)?, consumed.call.encode_unsigned());
    assert_eq!(
        reviewed.construct_method()?,
        Some(ConstructMethod::LedgerSend {
            payload: payload.clone()
        })
    );
    assert_eq!(consumed.call.chain_id, 125);
    let mut target = [0; 20];
    target[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
    assert_eq!(consumed.call.to, target);
    assert_eq!(
        consumed.call.max_priority_fee_per_gas,
        options.max_priority_fee_per_gas
    );
    assert_eq!(consumed.call.max_fee_per_gas, options.max_fee_per_gas);
    assert_eq!(consumed.call.gas_limit, options.gas_limit);

    let signature = payments::complete(sign_disclosed_call(
        &signer,
        &consumed.call,
        &reviewed,
        options.from,
    ))?;
    let raw = assemble(&consumed.call, &signature);
    let (decoded, decoded_signature) = decode_signed(&raw)?;
    assert_eq!(decoded, produced.call);
    assert_eq!(recover_sender(&decoded, &decoded_signature)?, options.from);
    assert_eq!(assemble(&decoded, &decoded_signature), raw);

    for field in 0..7 {
        let mut changed = consumed.call.clone();
        match field {
            0 => changed.chain_id += 1,
            1 => changed.nonce += 1,
            2 => changed.max_priority_fee_per_gas += 1,
            3 => changed.max_fee_per_gas += 1,
            4 => changed.gas_limit += 1,
            5 => changed.to[19] ^= 1,
            6 => {
                let mut different = debit.clone();
                different.amount += 1;
                different.context_hash = send_context_hash(
                    &different.from,
                    &different.to,
                    &different.asset,
                    different.amount,
                    &different.idempotency_key,
                );
                let different_payload = payments::complete(different.sign(&signer))?;
                changed = ledger_payment_call(&different_payload, &options)?.call;
                assert!(bind_call(&changed, options.from).is_ok());
            }
            _ => unreachable!(),
        }
        assert_ne!(changed.signing_hash(), consumed.call.signing_hash());
        let expected = if field == 5 {
            SignError::InvalidDisclosure
        } else {
            SignError::DisclosureMismatch("call")
        };
        assert_eq!(
            payments::complete(sign_disclosed_call(
                &signer,
                &changed,
                &reviewed,
                options.from
            )),
            Err(expected),
            "changed field {field}"
        );
    }
    let mut other_sender = options.from;
    other_sender[0] ^= 1;
    assert_eq!(
        payments::complete(sign_disclosed_call(
            &signer,
            &consumed.call,
            &reviewed,
            other_sender
        )),
        Err(SignError::DisclosureMismatch("call"))
    );
    let other_signer = layerx_crypto::signer::LocalSigner::from_derived_account(
        &layerx_crypto::account_derivation::derive_from_seed(&[8; 64], 0)?,
    );
    assert_eq!(
        payments::complete(sign_disclosed_call(
            &other_signer,
            &consumed.call,
            &reviewed,
            options.from
        )),
        Err(SignError::DisclosureMismatch("signer"))
    );
    Ok(())
}

#[test]
fn payment_tests_old_signing_path_refuses_every_malformed_send_fixture(
) -> Result<(), Box<dyn std::error::Error>> {
    let signer = payments::signer()?;
    let options = payments::options(&signer);
    let prepared = payments::send_fixture(&options)?;
    let Some(ConstructMethod::LedgerSend { payload }) = prepared.disclosure.construct_method()?
    else {
        return Err("send fixture decoded to another construct".into());
    };
    for length in 0..payload.len() {
        assert!(
            ledger_payment_call(&payload[..length], &options).is_err(),
            "length {length}"
        );
    }
    let mut trailing = payload.clone();
    trailing.push(0);
    assert!(ledger_payment_call(&trailing, &options).is_err());
    for offset in 0..payload.len() {
        let mut changed = payload.clone();
        changed[offset] ^= 1;
        assert!(
            ledger_payment_call(&changed, &options).is_err(),
            "offset {offset}"
        );
    }
    let signature = payments::complete(sign_disclosed_call(
        &signer,
        &prepared.call,
        &prepared.disclosure,
        options.from,
    ))?;
    let mut raw = assemble(&prepared.call, &signature);
    raw.push(0);
    assert!(decode_signed(&raw).is_err());
    Ok(())
}
