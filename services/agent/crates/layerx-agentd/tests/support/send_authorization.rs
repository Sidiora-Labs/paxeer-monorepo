#![allow(dead_code)]

use std::future::Future;
use std::pin::pin;
use std::task::{Context, Poll, Waker};

use layerx_agentd::prepare::{
    prepare_bound, verify_call_binding, EvmPreparationDefaults, EvmPreparationState,
    EvmPrepareRequest, PrecompileCall,
};
use layerx_agentd::sign::SigningPreparation;
use layerx_crypto::local::LocalSigner;
use layerx_crypto::send::{ledger_payment_call, CallOptions, SendCondition, SendDebit};
use layerx_wire::decode::Decoder;

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn ready<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    match future
        .as_mut()
        .poll(&mut Context::from_waker(Waker::noop()))
    {
        Poll::Ready(result) => result,
        Poll::Pending => panic!("local signing unexpectedly blocked"),
    }
}

pub fn debit(id: u8) -> SendDebit {
    SendDebit {
        from: [0x11; 32],
        to: [0x22; 32],
        asset: [0x33; 32],
        amount: 25,
        source_sequence: 5,
        idempotency_key: [id; 32],
        expires_at: 1_010,
        context_hash: [0x55; 32],
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: 17,
        protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
    }
}

pub fn signed_debit(debit: &SendDebit) -> Vec<u8> {
    must(ready(debit.sign(&LocalSigner::new([0x66; 32]))))
}

pub fn preparation(id: u8) -> SigningPreparation {
    let debit = debit(id);
    let payload = signed_debit(&debit);
    let key = must(k256::ecdsa::SigningKey::from_slice(&[0xa5; 32]));
    let actor = must(layerx_crypto::secp256k1::evm_address(
        key.verifying_key().to_encoded_point(false).as_bytes(),
    ));
    let produced = must(ledger_payment_call(
        &payload,
        &CallOptions {
            from: actor,
            chain_id: 17,
            nonce: 5,
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 3,
            gas_limit: 100_000,
        },
    ));
    let transaction = must(prepare_bound(
        EvmPreparationState {
            chain_id: 17,
            pending_nonce: 5,
            base_fee_per_gas: 1,
            head_height: 88,
        },
        EvmPreparationDefaults {
            max_priority_fee_per_gas: 1,
            max_fee_per_gas: 7,
            maximum_payload_bytes: 1_024,
        },
        EvmPrepareRequest {
            from: actor,
            call: PrecompileCall::ledger_transfer(payload),
            expected_nonce: Some(5),
            max_priority_fee_per_gas: None,
            max_fee_per_gas: None,
            declared_payload_limit: 1_024,
        },
        100_000,
    ));
    assert_eq!(transaction.call, produced.call);
    assert_eq!(transaction.disclosure, produced.disclosure);
    must(verify_call_binding(&transaction));
    SigningPreparation {
        transaction,
        actor_address: actor,
    }
}

// Compatibility entry for out-of-scope fixture callers. Decode every field,
// then ask the production typed producer to authorize it; never patch signatures.
pub fn sign(payload: Vec<u8>) -> Vec<u8> {
    let payload = payload.into_boxed_slice();
    let mut decoder = Decoder::new(&payload, 1_024);
    assert_eq!(must(decoder.u16()), 0x5301);
    assert_eq!(must(decoder.u16()), 10);
    let from = must(must(decoder.fixed(32)).try_into());
    let to = must(must(decoder.fixed(32)).try_into());
    let asset = must(must(decoder.fixed(32)).try_into());
    let amount = must(decoder.u128());
    let source_sequence = must(decoder.u64());
    let idempotency_key = must(must(decoder.fixed(32)).try_into());
    let expires_at = must(decoder.u64());
    let context_hash = must(must(decoder.fixed(32)).try_into());
    let count = must(decoder.u8());
    assert!(count <= 8);
    let conditions = (0..count)
        .map(|_| SendCondition {
            kind: must(decoder.u8()),
            timestamp: must(decoder.u64()),
        })
        .collect();
    let authorization_kind = must(decoder.u8());
    assert_eq!(must(decoder.fixed(32)), from);
    let _supplied_key = must(decoder.fixed(32));
    let _supplied_signature = must(decoder.fixed(64));
    assert_eq!(must(decoder.fixed(32)), context_hash);
    let network_id = must(decoder.u32());
    let protocol_version = must(decoder.u16());
    must(decoder.finish());
    signed_debit(&SendDebit {
        from,
        to,
        asset,
        amount,
        source_sequence,
        idempotency_key,
        expires_at,
        context_hash,
        conditions,
        authorization_kind,
        network_id,
        protocol_version,
    })
}
