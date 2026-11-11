#![allow(dead_code)]

pub mod assertions;
pub mod kernel_payloads;
pub mod signing;

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

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn signer() -> LocalSigner {
    let account = assertions::require(derive_from_seed(&[0x07; 64], 0), "fixture account");
    LocalSigner::from_derived_account(&account)
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn call_options(nonce: u64) -> CallOptions {
    let account = assertions::require(derive_from_seed(&[0x07; 64], 0), "fixture account");
    CallOptions {
        from: account.evm_address(),
        chain_id: CHAIN_ID,
        nonce,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
    }
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn send_call(amount: u128, nonce: u64) -> DisclosedCall {
    let (payload, _) = kernel_payloads::send(amount);
    assertions::require(
        ledger_payment_call(&payload, &call_options(nonce)),
        "ledger send call",
    )
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

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn canonical_send_with_payload(
    payload: &[u8],
    authority: &[u8],
    transaction_nonce: u64,
) -> Vec<u8> {
    assert_eq!(authority.len(), 32, "kernel authorization key width");
    let key_start = assertions::require(
        payload.len().checked_sub(134),
        "complete kernel send payload",
    );
    assert_eq!(
        &payload[key_start..key_start + 32],
        authority,
        "kernel authorization key"
    );
    assertions::require(
        ledger_payment_call(payload, &call_options(transaction_nonce)),
        "validated ledger send payload",
    )
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
/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn canonical_native_withdraw(payload: &[u8], activity_type: u32) -> Vec<u8> {
    assert_eq!(activity_type >> 16, ModuleId::Asset as u32);
    payload.to_vec()
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
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

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
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

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
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
/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn archived_send(amount: u128) -> Vec<u8> {
    let (payload, _) = kernel_payloads::send(amount);
    archived_activity(
        &payload,
        assertions::require(ActivityType::new(ModuleId::Asset, 5), "archived send kind"),
        IDEMPOTENCY_KEY,
    )
}

fn archived_activity(payload: &[u8], kind: ActivityType, action_key: [u8; 32]) -> Vec<u8> {
    use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
    use layerx_types::amount::Amount;
    use layerx_types::ids::{Did, IdempotencyKey};
    use layerx_types::payload::Payload;
    use layerx_wire::hash::Domain;
    use sha2::{Digest as _, Sha256};
    let module = assertions::require(
        ModuleRegistration::new(kind.module(), &[kind]),
        "archive module",
    );
    let registry = assertions::require(ModuleRegistry::new(&[module]), "archive registry");
    let mut hash = Sha256::new();
    hash.update(Domain::PayloadHash.tag());
    hash.update(payload);
    let mut builder = EnvelopeBuilder::new();
    assertions::require(builder.protocol_version(PROTOCOL_VERSION), "protocol");
    assertions::require(builder.network_id(NETWORK_ID), "network");
    assertions::require(builder.activity_type(kind), "kind");
    assertions::require(
        builder.actor_did(assertions::require(Did::new(b"did:layerx:alice"), "DID")),
        "actor",
    );
    assertions::require(
        builder.authority(assertions::require(Authority::owner(&[0x55; 32]), "key")),
        "authority",
    );
    assertions::require(builder.account_sequence(SEQUENCE), "sequence");
    assertions::require(
        builder.timestamp_bound(assertions::require(
            TimestampBound::new(NOT_BEFORE, EXPIRES_AT),
            "bounds",
        )),
        "time",
    );
    assertions::require(
        builder.idempotency_key(IdempotencyKey::new(action_key)),
        "retry identity",
    );
    assertions::require(builder.fee_limit(Amount::from_u128(20)), "fee");
    assertions::require(builder.payload_hash(hash.finalize().into()), "commitment");
    assertions::require(
        builder.payload(assertions::require(
            Payload::new(&registry, kind, payload),
            "payload",
        )),
        "payload field",
    );
    assertions::require(
        layerx_wire::historical::activity::encode_unsigned_envelope(&assertions::require(
            builder.build(),
            "archive envelope",
        )),
        "archive bytes",
    )
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn archived_bridge_credit(amount: u128) -> Vec<u8> {
    use sha2::{Digest as _, Sha256};
    let payload = kernel_payloads::bridge_credit(amount);
    let mut nullifier = Sha256::new();
    nullifier.update(b"LX:DEPOSIT:NULLIFIER:v1");
    nullifier.update(&payload[43..75]);
    archived_activity(
        &payload,
        assertions::require(ActivityType::new(ModuleId::Bridge, 1), "credit"),
        nullifier.finalize().into(),
    )
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn archived_bridge_withdraw(amount: u128) -> Vec<u8> {
    archived_activity(
        &kernel_payloads::bridge_withdraw(amount),
        assertions::require(ActivityType::new(ModuleId::Bridge, 2), "withdraw"),
        IDEMPOTENCY_KEY,
    )
}

/// # Panics
/// Panics if the retained protocol fixture, local peer or test environment is invalid.
pub fn archived_native_withdraw(payload: &[u8], activity_type: u32) -> Vec<u8> {
    assert_eq!(activity_type >> 16, ModuleId::Asset as u32);
    archived_activity(
        payload,
        assertions::require(
            ActivityType::new(ModuleId::Asset, {
                let Ok(ordinal) = u16::try_from(activity_type & 0xffff) else {
                    panic!("ordinal exceeds u16");
                };
                ordinal
            }),
            "native withdraw",
        ),
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
