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
