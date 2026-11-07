use std::future::Future;
use std::pin::pin;
use std::task::{Context, Poll, Waker};

use k256::ecdsa::SigningKey;
use layerx_agentd::prepare::{evm, verify_call_binding};
use layerx_agentd::sign::{
    attach_external_signature, external, verify_before_submit, SigningError, SigningPreparation,
};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::local::LocalSigner;
use layerx_crypto::secp256k1::evm_address;
use layerx_crypto::send::{ledger_payment_call, CallOptions, SendDebit};
use layerx_types::payload::ModuleRegistry;

pub(crate) const OWNER: [u8; 32] = [0xa5; 32];
const FOREIGN: [u8; 32] = [0x5a; 32];

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    match future
        .as_mut()
        .poll(&mut Context::from_waker(Waker::noop()))
    {
        Poll::Ready(value) => value,
        Poll::Pending => panic!("local debit authorization unexpectedly blocked"),
    }
}

pub(crate) fn registry() -> ModuleRegistry {
    must(ModuleRegistry::new(&[]))
}

fn address(seed: &[u8; 32]) -> [u8; 20] {
    let key = must(SigningKey::from_slice(seed));
    must(evm_address(
        key.verifying_key().to_encoded_point(false).as_bytes(),
    ))
}

pub(crate) fn owner_bound() -> SigningPreparation {
    // The inner debit has its native authorization; the outer transaction uses
    // secp256k1 and the real call-bound disclosure, without an activity envelope.
    let payload = must(ready(
        SendDebit {
            from: [0x11; 32],
            to: [0x22; 32],
            asset: [0x33; 32],
            amount: 25,
            source_sequence: 5,
            idempotency_key: [4; 32],
            expires_at: 1_010,
            context_hash: [0x55; 32],
            conditions: Vec::new(),
            authorization_kind: 1,
            network_id: 125,
            protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
        }
        .sign(&LocalSigner::new([0x66; 32])),
    ));
    let actor_address = address(&OWNER);
    let produced = must(ledger_payment_call(
        &payload,
        &CallOptions {
            from: actor_address,
            chain_id: 125,
            nonce: 5,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 7,
            gas_limit: 100_000,
        },
    ));
    let transaction = must(evm::prepare_bound(
        evm::EvmPreparationState {
            chain_id: 125,
            pending_nonce: 5,
            base_fee_per_gas: 3,
            head_height: 88,
        },
        evm::PreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 12,
            maximum_payload_bytes: 1_024,
        },
        evm::PrepareRequest {
            from: actor_address,
            call: evm::PrecompileCall::ledger_transfer(payload),
            expected_nonce: Some(5),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: Some(7),
            declared_payload_limit: 1_024,
        },
        100_000,
    ));
    assert_eq!(transaction.call, produced.call);
    assert_eq!(transaction.disclosure, produced.disclosure);
    assert_eq!(transaction.disclosure.actor, actor_address);
    assert_eq!(transaction.disclosure.authority, actor_address);
    assert_eq!(transaction.disclosure.amounts[0].value, 25);
    assert_eq!(transaction.disclosure.idempotency_key, [4; 32]);
    must(verify_call_binding(&transaction));
    SigningPreparation {
        transaction,
        actor_address,
    }
}

pub(crate) fn signed_by(prepared: &SigningPreparation, seed: &[u8; 32]) -> Vec<u8> {
    let package = must(external(prepared));
    assert_eq!(
        package.canonical_bytes,
        prepared.transaction.call.encode_unsigned()
    );
    assert_eq!(
        package.signing_preimage,
        prepared.transaction.call.signing_hash()
    );
    assert_eq!(
        package.disclosure_digest.0,
        prepared.transaction.disclosure_digest
    );
    must(prepared.transaction.call.sign(seed))
}

#[test]
fn a_foreign_key_signature_over_the_owner_preimage_is_refused() {
    let prepared = owner_bound();
    let foreign_address = address(&FOREIGN);
    assert_ne!(prepared.actor_address, foreign_address);
    let signed_bytes = signed_by(&prepared, &FOREIGN);
    let (call, signature) = must(decode_signed(&signed_bytes));
    assert_eq!(call, prepared.transaction.call);
    assert_eq!(must(recover_sender(&call, &signature)), foreign_address);
    assert_eq!(
        attach_external_signature(&prepared, *signature.as_bytes()),
        Err(SigningError::SignerMismatch)
    );
    for signer_address in [foreign_address, prepared.actor_address] {
        assert_eq!(
            verify_before_submit(&signed_bytes, &prepared, &signer_address, &registry()),
            Err(SigningError::SignerMismatch)
        );
    }
}

#[test]
fn the_bound_owner_key_still_verifies() {
    let prepared = owner_bound();
    let signed_bytes = signed_by(&prepared, &OWNER);
    let (call, signature) = must(decode_signed(&signed_bytes));
    assert_eq!(call, prepared.transaction.call);
    assert_eq!(
        must(recover_sender(&call, &signature)),
        prepared.actor_address
    );
    let attached = must(attach_external_signature(&prepared, *signature.as_bytes()));
    assert_eq!(attached, signed_bytes);
    let verified = must(verify_before_submit(
        &attached,
        &prepared,
        &prepared.actor_address,
        &registry(),
    ));
    assert_eq!(verified.exact_bytes(), signed_bytes);
    assert_eq!(verified.signed_byte_length(), signed_bytes.len());
    assert_eq!(verified.activity_id(), transaction_hash(&signed_bytes));
    assert_ne!(verified.activity_id(), [0; 32]);
    assert_eq!(verified.idempotency_key(), [4; 32]);
    assert_eq!(verified.recovered_sender(), prepared.actor_address);
    assert_eq!(verified.clone().into_exact_bytes(), signed_bytes);
}

#[test]
fn a_claimed_foreign_sender_cannot_replace_the_bound_owner() {
    let prepared = owner_bound();
    let signed_bytes = signed_by(&prepared, &OWNER);
    assert_eq!(
        verify_before_submit(&signed_bytes, &prepared, &address(&FOREIGN), &registry()),
        Err(SigningError::SignerMismatch)
    );
    let mut changed_actor = prepared.clone();
    changed_actor.actor_address = address(&FOREIGN);
    assert_eq!(
        verify_before_submit(
            &signed_bytes,
            &changed_actor,
            &changed_actor.actor_address,
            &registry()
        ),
        Err(SigningError::SignerMismatch)
    );
    let mut changed_disclosure = prepared.clone();
    changed_disclosure.transaction.disclosure.authority = address(&FOREIGN).to_vec();
    assert!(verify_before_submit(
        &signed_bytes,
        &changed_disclosure,
        &prepared.actor_address,
        &registry()
    )
    .is_err());
    let (_, signature) = must(decode_signed(&signed_bytes));
    assert!(attach_external_signature(&changed_disclosure, *signature.as_bytes()).is_err());
}
