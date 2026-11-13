mod support;

use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use layerx_crypto::disclosure::call::{
    bind_call, reencode, validate_call, validate_canonical, ConstructMethod,
};
use layerx_crypto::disclosure::{AmountRole, CounterpartyRole};
use layerx_crypto::ed25519;
use layerx_crypto::evm_transaction::{assemble, decode_signed, decode_unsigned, recover_sender};
use layerx_crypto::send::ledger_payment_call;
use layerx_crypto::signer::{sign_disclosed_call, LocalSigner, SignError, Signer};
use layerx_crypto::VerifyError;

struct NoopWake;

impl Wake for NoopWake {
    fn wake(self: Arc<Self>) {}
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(NoopWake));
    let mut context = Context::from_waker(&waker);
    match future.as_mut().poll(&mut context) {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local signer unexpectedly blocked"),
    }
}

type Outcome = Result<(), Box<dyn std::error::Error>>;

#[test]
fn shared_fixtures_produce_real_evm_ingress() -> Outcome {
    let signer = support::signer();
    let from = signer.evm_address().ok_or("missing EVM key")?;
    for amount in [1, 25, 9_000_000, u128::MAX] {
        for nonce in [support::SEQUENCE, 19] {
            let produced = support::send_call(amount, nonce);
            let canonical = support::canonical_send_sequences(amount, nonce);
            assert_eq!(canonical.first(), Some(&2));
            assert_eq!(canonical, produced.call.encode_unsigned());
            assert_eq!(decode_unsigned(&canonical)?, produced.call);
            assert!(layerx_wire::historical::activity::decode_unsigned(
                &canonical,
                &support::registry()
            )
            .is_err());
            let signature = ready(sign_disclosed_call(
                &signer,
                &produced.call,
                &produced.disclosure,
                from,
            ))?;
            let raw = assemble(&produced.call, &signature);
            let (received, received_signature) = decode_signed(&raw)?;
            assert_eq!(received, produced.call);
            assert_eq!(recover_sender(&received, &received_signature)?, from);
            let consumed = bind_call(&received, from)?;
            assert_eq!(consumed, produced.disclosure);
            assert_eq!(consumed.actor, from);
            assert_eq!(received.chain_id, 125);
            assert_eq!(received.nonce, nonce);
            assert_eq!(consumed.envelope_sequence(), nonce);
            assert_eq!(consumed.payload_sequence()?, Some(7));
            assert_eq!(consumed.amounts.len(), 1);
            assert_eq!(consumed.amounts[0].role, AmountRole::Transfer);
            assert_eq!(consumed.amounts[0].value, amount);
            assert_eq!(consumed.asset, [0x33; 32]);
            assert_eq!(consumed.counterparties.len(), 2);
            assert_eq!(consumed.counterparties[0].role, CounterpartyRole::Payer);
            assert_eq!(consumed.counterparties[0].account, [0x11; 32]);
            assert_eq!(consumed.counterparties[1].role, CounterpartyRole::Recipient);
            assert_eq!(consumed.counterparties[1].account, [0x22; 32]);
            assert_eq!(consumed.fee_limit, 3_000_000);
            assert_eq!(consumed.idempotency_key, support::IDEMPOTENCY_KEY);
            assert_eq!(consumed.expiry.payload_expires_at, support::EXPIRES_AT);
            let (kernel, authority) = support::kernel_payloads::send(amount);
            assert_eq!(
                consumed.construct_method()?,
                Some(ConstructMethod::LedgerSend {
                    payload: kernel.clone()
                })
            );
            assert_eq!(
                support::canonical_send_with_payload(&kernel, &authority, nonce),
                canonical
            );
            assert_eq!(reencode(&consumed)?, canonical);
            validate_call(&received, &consumed, from)?;
            validate_canonical(&consumed, &canonical)?;
            assert_eq!(consumed.signing_digest()?, received.signing_hash());
        }
        assert_eq!(
            support::canonical_send(amount),
            support::canonical_send_sequences(amount, 7)
        );
    }
    Ok(())
}

#[test]
fn shared_fixtures_refuse_substituted_call_and_disclosure() -> Outcome {
    let signer = support::signer();
    let from = signer.evm_address().ok_or("missing EVM key")?;
    let produced = support::send_call(25, 19);
    for field in 0..8 {
        let mut changed = produced.call.clone();
        match field {
            0 => changed.chain_id += 1,
            1 => changed.nonce += 1,
            2 => changed.max_priority_fee_per_gas += 1,
            3 => changed.max_fee_per_gas += 1,
            4 => changed.gas_limit += 1,
            5 => changed.to[0] ^= 1,
            6 => changed.data[68 + 100] ^= 1,
            _ => changed.data.push(0),
        }
        assert!(
            ready(sign_disclosed_call(
                &signer,
                &changed,
                &produced.disclosure,
                from
            ))
            .is_err(),
            "field {field}"
        );
        assert!(validate_canonical(&produced.disclosure, &changed.encode_unsigned()).is_err());
    }
    for field in 0..4 {
        let mut changed = produced.disclosure.clone();
        match field {
            0 => changed.amounts[0].value = 1,
            1 => changed.counterparties[1].account[0] ^= 1,
            2 => changed.idempotency_key[0] ^= 1,
            _ => changed.fee_limit += 1,
        }
        assert!(ready(sign_disclosed_call(&signer, &produced.call, &changed, from)).is_err());
        assert!(reencode(&changed).is_err());
    }
    let mut other_sender = from;
    other_sender[0] ^= 1;
    assert!(ready(sign_disclosed_call(
        &signer,
        &produced.call,
        &produced.disclosure,
        other_sender
    ))
    .is_err());
    let other_disclosure = bind_call(&produced.call, other_sender)?;
    assert_eq!(
        ready(sign_disclosed_call(
            &signer,
            &produced.call,
            &other_disclosure,
            other_sender
        )),
        Err(SignError::DisclosureMismatch("signer"))
    );
    let did_only = LocalSigner::new([0x53; 32]);
    assert_eq!(
        ready(sign_disclosed_call(
            &did_only,
            &produced.call,
            &produced.disclosure,
            from
        )),
        Err(SignError::EvmKeyUnavailable)
    );
    Ok(())
}

#[test]
fn shared_fixtures_refuse_invalid_kernel_signatures_and_abi() {
    let options = support::call_options(19);
    let (payload, _) = support::kernel_payloads::send(25);
    for length in 0..payload.len() {
        assert!(
            ledger_payment_call(&payload[..length], &options).is_err(),
            "truncated at {length}"
        );
    }
    for offset in [100, 116, payload.len() - 102, payload.len() - 1] {
        let mut corrupted = payload.clone();
        corrupted[offset] ^= 1;
        assert!(
            ledger_payment_call(&corrupted, &options).is_err(),
            "kernel byte {offset}"
        );
    }
    let mut trailing = payload;
    trailing.push(0);
    assert!(ledger_payment_call(&trailing, &options).is_err());
    let produced = support::send_call(25, 19);
    for offset in [0, 4, 35, 36, 67, produced.call.data.len() - 1] {
        let mut changed = produced.call.clone();
        changed.data[offset] ^= 1;
        assert!(
            bind_call(&changed, options.from).is_err(),
            "ABI byte {offset}"
        );
    }
}

#[test]
fn shared_fixtures_preserve_kernel_payloads_separately() -> Outcome {
    let recorded = support::kernel_payloads::recorded_send();
    let produced = ledger_payment_call(&recorded, &support::call_options(7))?;
    let canonical = produced.call.encode_unsigned();
    let received = decode_unsigned(&canonical)?;
    assert_eq!(
        bind_call(&received, support::call_options(7).from)?.construct_method()?,
        Some(ConstructMethod::LedgerSend {
            payload: recorded.clone()
        })
    );
    assert_eq!(&recorded[..4], &[0x53, 1, 0, 10]);
    assert!(decode_unsigned(&recorded).is_err());
    let withdraw = support::kernel_payloads::bridge_withdraw(25);
    let mut expected = vec![0x48, 2, 0, 7];
    for bytes in [
        &[0x41; 32][..],
        &[0x11; 32],
        &[0x22; 32],
        &[0x44; 20],
        &[0x33; 32],
        &25_u128.to_be_bytes(),
        &support::IDEMPOTENCY_KEY,
    ] {
        expected.extend_from_slice(bytes);
    }
    assert_eq!(withdraw, expected);
    let credit = support::kernel_payloads::bridge_credit(25);
    assert_eq!(credit.len(), 368);
    assert_eq!(&credit[..5], b"LXDC3");
    assert_eq!(&credit[37..41], &17_u32.to_be_bytes());
    assert_eq!(&credit[191..207], &25_u128.to_be_bytes());
    assert_eq!(&credit[363..], b"LXLB1");
    for (payload, registry) in [
        (withdraw, support::bridge_withdraw_registry()),
        (credit, support::bridge_registry()),
    ] {
        assert!(decode_unsigned(&payload).is_err());
        assert!(layerx_wire::historical::activity::decode_unsigned(&payload, &registry).is_err());
    }
    Ok(())
}

#[test]
fn shared_fixtures_bind_consent_to_chain_address_and_nonce() -> Outcome {
    let address = [
        0x10, 0x21, 0x32, 0x43, 0x54, 0x65, 0x76, 0x87, 0x98, 0xa9, 0xba, 0xcb, 0xdc, 0xed, 0xfe,
        0x0f, 0x1e, 0x2d, 0x3c, 0x4b,
    ];
    let signed = support::scoped_binding(125, address, 2);
    signed.verify().map_err(|error| format!("{error:?}"))?;
    let message = signed.binding().message();
    assert_eq!(message.len(), 77);
    assert_eq!(&message[..17], b"LX:PAXEER-BIND:v1");
    assert_eq!(&message[17..41], &[0; 24]);
    assert_eq!(&message[41..49], &125_u64.to_be_bytes());
    assert_eq!(&message[49..69], &address);
    assert_eq!(&message[69..], &2_u64.to_be_bytes());
    ed25519::verify_message(&signed.public_key(), &signed.signature(), &message)
        .map_err(|error| format!("{error:?}"))?;
    for offset in 0..message.len() {
        let mut changed = message;
        changed[offset] ^= 1;
        assert_eq!(
            ed25519::verify_message(&signed.public_key(), &signed.signature(), &changed),
            Err(VerifyError::BadSignature),
            "bind byte {offset}"
        );
    }
    let mut other_address = address;
    other_address[0] ^= 1;
    for (chain, address, nonce) in [
        (126, address, 2),
        (125, other_address, 2),
        (125, address, 3),
    ] {
        let other = support::scoped_binding(chain, address, nonce);
        other.verify().map_err(|error| format!("{error:?}"))?;
        assert_ne!(other.binding().message(), message);
        assert_eq!(
            other
                .binding()
                .verify(&signed.public_key(), &signed.signature()),
            Err(VerifyError::BadSignature)
        );
    }
    assert!(decode_unsigned(&message).is_err());
    Ok(())
}
