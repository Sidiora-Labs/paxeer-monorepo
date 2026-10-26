use layerx_agentd::budget::{
    consumption_updates, create_daemon_limit, daemon_limit_id, daemon_limits, divergence_alert,
    load_daemon_limits, release, reserve, BudgetLimiter, CoreTimestampMs, DaemonLimitError,
    DaemonLimitRecord, LimitRefusal, LocalAccounting, ProtocolBudgetState, ReconcileError,
    ReleaseKind, ReservationRequest,
};
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_crypto::evm_transaction::Eip1559Call;

fn must<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("{error:?}"))
}

fn budget_result() -> Vec<u8> {
    let mut bytes = vec![0; 22 * 32];
    // ABI output from ILayerXPay.getBudget: dynamic tuple with two delegates.
    for (index, value) in [
        (0, 32_u128),
        (5, 1_000),
        (6, 900),
        (7, 100),
        (8, 350),
        (9, 100),
        (10, 60_000),
        (11, 1_000),
        (12, 90_000),
        (13, 7),
        (14, 2),
        (17, 18 * 32),
        (19, 2),
    ] {
        bytes[index * 32 + 16..(index + 1) * 32].copy_from_slice(&value.to_be_bytes());
    }
    for (index, value) in [
        (1, 0x11),
        (2, 0x22),
        (3, 0x33),
        (4, 0x44),
        (18, 0x55),
        (20, 0x66),
        (21, 0x77),
    ] {
        bytes[index * 32..(index + 1) * 32].fill(value);
    }
    bytes
}

#[test]
fn budget_state_is_decoded_from_get_budget() {
    let abi: serde_json::Value = must(serde_json::from_str(include_str!(
        "../../../../../precompiles/layerxpay/abi.json"
    )));
    let get_budget = abi
        .as_array()
        .unwrap_or_else(|| panic!("ABI array"))
        .iter()
        .find(|entry| entry["name"] == "getBudget")
        .unwrap_or_else(|| panic!("getBudget ABI"));
    assert_eq!(
        get_budget["outputs"][0]["components"]
            .as_array()
            .unwrap_or_else(|| panic!("Budget tuple"))
            .len(),
        18
    );
    let result = budget_result();
    let state = must(ProtocolBudgetState::decode_get_budget([9; 32], 42, &result));
    assert_eq!(state.committed_height(), 42);
    let record = state.record();
    assert_eq!(record.budget_id, [9; 32]);
    assert_eq!(record.owner, [0x11; 32]);
    assert_eq!(record.budget_account, [0x22; 32]);
    assert_eq!(record.asset_id, [0x33; 32]);
    assert_eq!(record.purpose_hash, [0x44; 32]);
    assert_eq!(record.per_period_limit, 1_000);
    assert_eq!(record.configured_period_limit, 900);
    assert_eq!(record.carry_cap, 100);
    assert_eq!(record.spent_this_period, 350);
    assert_eq!(record.carried, 100);
    assert_eq!(record.period_length, 60_000);
    assert_eq!(record.period_start, 1_000);
    assert_eq!(record.expiry, 90_000);
    assert_eq!(record.revocation_sequence, 7);
    assert_eq!(record.rollover_policy, 2);
    assert!(!record.closed);
    assert!(!record.revoked);
    assert_eq!(record.delegates, vec![[0x66; 32], [0x77; 32]]);
    assert_eq!(record.source_account, Some([0x55; 32]));
    let mut local = LocalAccounting {
        consumed: 400,
        window_start_sequence: 0,
        last_receipt: None,
    };
    let reconciled = must(state.reconcile(&mut local, &[]));
    assert_eq!(local.consumed, 350);
    assert_eq!(local.window_start_sequence, 1_000);
    assert_eq!(reconciled.remaining(), 650);
    let alert = divergence_alert(&reconciled, 1_000)
        .unwrap_or_else(|| panic!("local overspend must flag divergence"));
    assert_eq!(alert.enforced_consumed, 400);
    assert_eq!(alert.audit.observed_head_sequence, 42);
    assert!(!alert.health.ready_for_writes);

    // Narrow ABI types, offsets, booleans, delegate ordering and length are strict.
    for (offset, value) in [
        (0, 1),
        (5 * 32, 1),
        (10 * 32, 1),
        (14 * 32, 1),
        (15 * 32 + 31, 2),
        (16 * 32 + 31, 2),
        (17 * 32 + 31, 0),
        (19 * 32 + 31, 17),
        (21 * 32, 0),
    ] {
        let mut malformed = result.clone();
        malformed[offset] = value;
        assert_eq!(
            ProtocolBudgetState::decode_get_budget([9; 32], 42, &malformed),
            Err(ReconcileError::ProtocolStateSchemaUnavailable)
        );
    }
    for length in [0, 31, 32, 19 * 32, result.len() - 1] {
        assert!(ProtocolBudgetState::decode_get_budget([9; 32], 42, &result[..length]).is_err());
    }
    let mut trailing = result.clone();
    trailing.push(0);
    assert!(ProtocolBudgetState::decode_get_budget([9; 32], 42, &trailing).is_err());
    let mut revoked = result.clone();
    revoked[16 * 32 + 31] = 1;
    let revoked = must(ProtocolBudgetState::decode_get_budget(
        [9; 32], 42, &revoked,
    ));
    assert_eq!(must(revoked.reconcile(&mut local, &[])).remaining(), 0);
}

struct Directory(std::path::PathBuf);
impl Directory {
    fn new() -> Self {
        let suffix =
            must(std::time::SystemTime::now().duration_since(std::time::UNIX_EPOCH)).as_nanos();
        Self(std::env::temp_dir().join(format!("layerxpay-budget-{}-{suffix}", std::process::id())))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

#[test]
fn pending_fee_counts_against_the_daemon_limit() {
    let directory = Directory::new();
    let mut store = must(Store::open(&directory.0));
    let tenant = must(TenantId::new("layerxpay-budget"));
    let limiter = must(BudgetLimiter::new(Vec::new()));
    let record = must(create_daemon_limit(
        &mut store,
        &limiter,
        DaemonLimitRecord {
            tenant: tenant.clone(),
            budget_id: [1; 32],
            limit_id: daemon_limit_id([1; 32]),
            agent_digest: [2; 32],
            asset: [3; 32],
            ceiling: 1_000,
            consumed: 0,
            expiry_ms: 50_000,
            revoked: false,
            mutation_key: [4; 32],
            body_digest: [5; 32],
            revoke_key: [0; 32],
        },
        CoreTimestampMs(1),
    ));
    let transaction = Eip1559Call {
        chain_id: 125,
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 3,
        gas_limit: 100,
        to: [6; 20],
        data: vec![],
    };
    let request = ReservationRequest {
        id: [7; 32],
        amount: 500,
        expiry_sequence: 100,
        current_sequence: 1,
        applicable_limits: vec![record.limit_id],
    };
    let hold = must(record.reserve_pending_transaction(&limiter, &request, &transaction));
    assert_eq!(hold.amount, 800);
    assert_eq!(limiter.held_exposure(record.limit_id), Ok(800));
    assert_eq!(limiter.consumed(record.limit_id), Ok(0));
    let second = ReservationRequest {
        id: [8; 32],
        amount: 201,
        ..request.clone()
    };
    assert!(matches!(
        reserve(&limiter, &second),
        Err(LimitRefusal::Exceeded { held: 800, .. })
    ));
    assert_eq!(limiter.held_exposure(record.limit_id), Ok(800));
    assert!(matches!(
        record.reserve_pending_transaction(&limiter, &second, &transaction),
        Err(DaemonLimitError::Limit(LimitRefusal::Exceeded {
            held: 800,
            requested: 501,
            ..
        }))
    ));
    assert_eq!(limiter.held_exposure(record.limit_id), Ok(800));
    let updates = must(consumption_updates(&store, &tenant, &hold.durable));
    assert_eq!(updates.len(), 1);
    assert_eq!(&updates[0].1[..5], b"LXDL\x01");
    assert_eq!(must(daemon_limits(&store, &tenant))[0].consumed, 0);
    must(store.update_local_batch(updates));
    assert_eq!(
        release(&limiter, request.id, ReleaseKind::Executed, 2),
        Ok(true)
    );
    assert_eq!(limiter.consumed(record.limit_id), Ok(800));
    assert_eq!(must(daemon_limits(&store, &tenant))[0].consumed, 800);

    let mut overflow = transaction.clone();
    overflow.max_fee_per_gas = u128::MAX;
    assert!(matches!(
        record.reserve_pending_transaction(&limiter, &second, &overflow),
        Err(DaemonLimitError::Arithmetic)
    ));
    assert_eq!(limiter.held_reservations(), Ok(0));
    let mut tampered = hold.durable.clone();
    tampered[0].amount = 1;
    assert!(matches!(
        consumption_updates(&store, &tenant, &tampered),
        Err(DaemonLimitError::Corrupt)
    ));
    drop(store);
    drop(limiter);
    let store = must(Store::open(&directory.0));
    let restarted = must(BudgetLimiter::new(Vec::new()));
    must(load_daemon_limits(&store, &restarted));
    assert_eq!(restarted.consumed(record.limit_id), Ok(800));
    assert!(matches!(
        reserve(&restarted, &second),
        Err(LimitRefusal::Exceeded { consumed: 800, .. })
    ));
    let key = must(TenantKey::new(
        tenant,
        ObjectKind::Configuration,
        [b"budget/daemon/limit/".as_slice(), &[1; 32]].concat(),
    ));
    assert_eq!(
        &store
            .get(&key)
            .unwrap_or_else(|| panic!("durable record"))
            .bytes()[..5],
        b"LXDL\x01"
    );
}
