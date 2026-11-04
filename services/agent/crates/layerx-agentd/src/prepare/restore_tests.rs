//! Shared integration fixtures emitted by the production EIP-1559 preparer.

use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
use layerx_agentd::prepare::{
    prepare_bound, AbiArgument, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest,
    EvmPrepared, PrecompileCall, PrecompileTarget,
};
use layerx_agentd::session::{SessionId, SessionRef};
use layerx_agentd::store::{Store, TenantId};
use layerx_crypto::disclosure::call::ledger_selector;

static NEXT: AtomicU64 = AtomicU64::new(0);
pub const OWNER: [u8; 20] = [
    0x7e, 0x5f, 0x45, 0x52, 0x09, 0x1a, 0x69, 0x12, 0x5d, 0x5d, 0xfc, 0xb7, 0xb8, 0xc2, 0x65, 0x90,
    0x29, 0x39, 0x5b, 0xdf,
];
pub const GENERATION: u64 = 7;
pub const EXPIRY: u64 = 1_000;

pub struct Directory(pub PathBuf);

impl Directory {
    pub fn new() -> Self {
        Self(std::env::temp_dir().join(format!(
            "repair-22-33-1543-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        )))
    }

    pub fn open(&self) -> Store {
        Store::open(&self.0).expect("open real tenant store")
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        if self.0.exists() {
            std::fs::remove_dir_all(&self.0).expect("remove integration store");
        }
    }
}

pub fn session() -> SessionRef {
    SessionRef::new(
        TenantId::new("call-owner").expect("tenant"),
        SessionId([9; 32]),
    )
}

pub fn defaults() -> EvmPreparationDefaults {
    EvmPreparationDefaults {
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 20,
        maximum_payload_bytes: 4_096,
    }
}

pub fn state() -> EvmPreparationState {
    EvmPreparationState {
        chain_id: 125,
        pending_nonce: 7,
        base_fee_per_gas: 2,
        head_height: 80,
    }
}

fn word(value: u128) -> [u8; 32] {
    let mut word = [0; 32];
    word[16..].copy_from_slice(&value.to_be_bytes());
    word
}

pub fn request(identity: u8) -> EvmPrepareRequest {
    EvmPrepareRequest {
        from: OWNER,
        call: PrecompileCall {
            target: PrecompileTarget::Ledger,
            selector: ledger_selector("payExact(bytes32,bytes32,bytes32,uint128,uint64,bytes32)"),
            arguments: vec![
                AbiArgument::Word([1; 32]),
                AbiArgument::Word([2; 32]),
                AbiArgument::Word([3; 32]),
                AbiArgument::Word(word(25)),
                AbiArgument::Word(word(u128::from(EXPIRY))),
                AbiArgument::Word([identity; 32]),
            ],
        },
        expected_nonce: Some(7),
        max_priority_fee_per_gas: None,
        max_fee_per_gas: None,
        declared_payload_limit: 4_096,
    }
}

pub fn prepared(identity: u8) -> EvmPrepared {
    prepare_bound(state(), defaults(), request(identity), 100_000)
        .expect("production call preparation")
}

pub fn signed(prepared: &EvmPrepared) -> Vec<u8> {
    let mut secret = [0; 32];
    secret[31] = 1;
    prepared
        .call
        .sign(&secret)
        .expect("real EIP-1559 signature")
}

pub fn limiter() -> BudgetLimiter {
    BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([1; 16]),
        name: "tenant-call-budget".into(),
        scope: LimitScope::Tenant([1; 32]),
        ceiling: 10_000,
        consumed: 0,
    }])
    .expect("real budget limiter")
}
