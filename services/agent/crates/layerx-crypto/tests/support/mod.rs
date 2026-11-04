#![allow(dead_code)]

pub mod kernel_payloads;

// Compile the maintained producer directly: this task cannot change the dev-dependency manifest.
#[path = "../../../layerx-client/src/paxeer_binding.rs"]
pub mod paxeer_binding;

use layerx_crypto::account_derivation::derive_from_seed;
use layerx_crypto::send::{ledger_payment_call, CallOptions, DisclosedCall};
use layerx_crypto::signer::LocalSigner;
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

pub const CHAIN_ID: u64 = 125;
pub const NETWORK_ID: u32 = 17;
pub const PROTOCOL_VERSION: u16 = layerx_wire::limits::PROTOCOL_VERSION;
pub const SEQUENCE: u64 = 7;
pub const NOT_BEFORE: u64 = 10;
pub const EXPIRES_AT: u64 = 100;
pub const IDEMPOTENCY_KEY: [u8; 32] = [0x71; 32];

pub fn signer() -> LocalSigner {
    let account = derive_from_seed(&[0x07; 64], 0).expect("fixture account");
    LocalSigner::from_derived_account(&account)
}

pub fn call_options(nonce: u64) -> CallOptions {
    let account = derive_from_seed(&[0x07; 64], 0).expect("fixture account");
    CallOptions {
        from: account.evm_address(),
        chain_id: CHAIN_ID,
        nonce,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    }
}

pub fn send_call(amount: u128, nonce: u64) -> DisclosedCall {
    let (payload, _) = kernel_payloads::send(amount);
    ledger_payment_call(&payload, &call_options(nonce)).expect("ledger send call")
}

pub fn scoped_binding(
    chain_id: u64,
    address: [u8; 20],
    nonce: u64,
) -> paxeer_binding::SignedBinding {
    paxeer_binding::Binding::new(chain_id, address, nonce).sign(&[0x61; 32])
}

pub fn canonical_send(amount: u128) -> Vec<u8> {
    canonical_send_sequences(amount, SEQUENCE)
}

pub fn canonical_send_sequences(amount: u128, transaction_nonce: u64) -> Vec<u8> {
    send_call(amount, transaction_nonce).call.encode_unsigned()
}

pub fn canonical_send_with_payload(
    payload: &[u8],
    authority: &[u8],
    transaction_nonce: u64,
) -> Vec<u8> {
    assert_eq!(authority.len(), 32, "kernel authorization key width");
    let key_start = payload
        .len()
        .checked_sub(134)
        .expect("complete kernel send payload");
    assert_eq!(
        &payload[key_start..key_start + 32],
        authority,
        "kernel authorization key"
    );
    ledger_payment_call(payload, &call_options(transaction_nonce))
        .expect("validated ledger send payload")
        .call
        .encode_unsigned()
}

#[deprecated(
    note = "raw kernel payload only; migrate the consumer away from bridge activity ingress"
)]
pub fn canonical_bridge_withdraw(amount: u128) -> Vec<u8> {
    kernel_payloads::bridge_withdraw(amount)
}

#[deprecated(
    note = "raw kernel payload only; migrate the consumer away from bridge activity ingress"
)]
pub fn canonical_bridge_credit(amount: u128) -> Vec<u8> {
    kernel_payloads::bridge_credit(amount)
}

#[deprecated(note = "raw kernel payload only; native withdrawal activity ingress is retired")]
pub fn canonical_native_withdraw(payload: &[u8], activity_type: u32) -> Vec<u8> {
    assert_eq!(activity_type >> 16, ModuleId::Asset as u32);
    payload.to_vec()
}

pub fn registry() -> ModuleRegistry {
    let Ok(send) = ActivityType::new(ModuleId::Asset, 5) else {
        panic!("asset send activity type rejected");
    };
    let Ok(asset) = ModuleRegistration::new(ModuleId::Asset, &[send]) else {
        panic!("asset registration rejected");
    };
    let Ok(registry) = ModuleRegistry::new(&[asset]) else {
        panic!("module registry rejected");
    };
    registry
}

pub fn bridge_registry() -> ModuleRegistry {
    let Ok(credit) = ActivityType::new(ModuleId::Bridge, 1) else {
        panic!("bridge credit activity type rejected");
    };
    let Ok(bridge) = ModuleRegistration::new(ModuleId::Bridge, &[credit]) else {
        panic!("bridge registration rejected");
    };
    let Ok(registry) = ModuleRegistry::new(&[bridge]) else {
        panic!("module registry rejected");
    };
    registry
}

pub fn bridge_withdraw_registry() -> ModuleRegistry {
    let Ok(withdraw) = ActivityType::new(ModuleId::Bridge, 2) else {
        panic!("bridge withdrawal activity type rejected");
    };
    let Ok(bridge) = ModuleRegistration::new(ModuleId::Bridge, &[withdraw]) else {
        panic!("bridge registration rejected");
    };
    let Ok(registry) = ModuleRegistry::new(&[bridge]) else {
        panic!("module registry rejected");
    };
    registry
}

// Archive fixtures have explicit names so the current transaction fixtures
// remain unchanged for consumers migrated by other tasks.
pub fn archived_send(amount: u128) -> Vec<u8> {
    let (payload, key) = kernel_payloads::send(amount);
    layerx_crypto::send::encode_send_envelope(
        &payload,
        &layerx_crypto::send::EnvelopeOptions {
            actor: "did:layerx:alice",
            public_key: key,
            protocol_version: PROTOCOL_VERSION,
            network_id: NETWORK_ID,
            identity_sequence: SEQUENCE,
            idempotency_key: IDEMPOTENCY_KEY,
            fee_limit: 20,
            not_before: NOT_BEFORE,
            not_after: EXPIRES_AT,
        },
    )
    .expect("legacy producer compatibility")
    .canonical
}

fn archived_activity(payload: &[u8], kind: ActivityType, action_key: [u8; 32]) -> Vec<u8> {
    use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::{Did, IdempotencyKey};
    use layerx_types::payload::Payload;
    use layerx_wire::hash::Domain;
    use sha2::{Digest as _, Sha256};
    let module = ModuleRegistration::new(kind.module(), &[kind]).expect("archive module");
    let registry = ModuleRegistry::new(&[module]).expect("archive registry");
    let mut hash = Sha256::new();
    hash.update(Domain::PayloadHash.tag());
    hash.update(payload);
    let mut builder = EnvelopeBuilder::new();
    builder
        .protocol_version(PROTOCOL_VERSION)
        .expect("protocol");
    builder.network_id(NETWORK_ID).expect("network");
    builder.activity_type(kind).expect("kind");
    builder
        .actor_did(Did::new(b"did:layerx:alice").expect("DID"))
        .expect("actor");
    builder
        .authority(Authority::owner(&[0x55; 32]).expect("key"))
        .expect("authority");
    builder.account_sequence(SEQUENCE).expect("sequence");
    builder
        .timestamp_bound(TimestampBound::new(NOT_BEFORE, EXPIRES_AT).expect("bounds"))
        .expect("time");
    builder
        .idempotency_key(IdempotencyKey::new(action_key))
        .expect("retry identity");
    builder.fee_limit(Amount::from_u128(20)).expect("fee");
    builder
        .payload_hash(hash.finalize().into())
        .expect("commitment");
    builder
        .payload(Payload::new(&registry, kind, payload).expect("payload"))
        .expect("payload field");
    layerx_wire::activity::encode_unsigned_envelope(&builder.build().expect("archive envelope"))
        .expect("archive bytes")
}

pub fn archived_bridge_credit(amount: u128) -> Vec<u8> {
    use sha2::{Digest as _, Sha256};
    let payload = kernel_payloads::bridge_credit(amount);
    let mut nullifier = Sha256::new();
    nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
    nullifier.update(&payload[43..75]);
    archived_activity(
        &payload,
        ActivityType::new(ModuleId::Bridge, 1).expect("credit"),
        nullifier.finalize().into(),
    )
}

pub fn archived_bridge_withdraw(amount: u128) -> Vec<u8> {
    archived_activity(
        &kernel_payloads::bridge_withdraw(amount),
        ActivityType::new(ModuleId::Bridge, 2).expect("withdraw"),
        IDEMPOTENCY_KEY,
    )
}

pub fn archived_native_withdraw(payload: &[u8], activity_type: u32) -> Vec<u8> {
    assert_eq!(activity_type >> 16, ModuleId::Asset as u32);
    archived_activity(
        payload,
        ActivityType::new(ModuleId::Asset, activity_type as u16).expect("native withdraw"),
        IDEMPOTENCY_KEY,
    )
}

pub fn debit(amount: u128) -> layerx_crypto::send::SendDebit {
    kernel_payloads::send_debit(amount)
}

pub fn evm_signer() -> LocalSigner {
    signer()
}

pub fn complete<F: std::future::Future>(future: F) -> F::Output {
    use std::sync::Arc;
    use std::task::{Context, Poll, Wake, Waker};
    struct ThreadWake(std::thread::Thread);
    impl Wake for ThreadWake {
        fn wake(self: Arc<Self>) {
            self.0.unpark();
        }
    }
    let mut future = std::pin::pin!(future);
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(result) => return result,
            Poll::Pending => std::thread::park(),
        }
    }
}
