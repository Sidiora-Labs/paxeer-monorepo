use std::fmt::Debug;

use layerx_crypto::disclosure::call::{ledger_selector, ConstructMethod};
use layerx_crypto::disclosure::{AmountRole, CounterpartyRole, DisclosureError};
use layerx_crypto::evm_transaction::Eip1559Call;
use layerx_crypto::payments::{bind_call, Disclosure};
use layerx_intents::{
    compile, BudgetCreate, BudgetDefund, BudgetFund, CompileField, CompiledIntent, DisclosureCheck,
    Intent, IntentKind,
};
use layerx_types::account::AccountId;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, IdempotencyKey};
use layerx_types::intent::{
    BudgetId, PeriodLength, PurposeHash, RolloverPolicy, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::hash;

const BUDGET_ID: [u8; 32] = [0x12; 32];
const ASSET: [u8; 32] = [0x34; 32];
const PURPOSE: [u8; 32] = [0x56; 32];
const AMOUNT: u128 = 0x0102_0304_0506_0708_090a_0b0c_0d0e_0f10;
const CARRY: u128 = 0x0102_0304_0506_0708;
const FROM: [u8; 20] = [0x42; 20];
const ACTOR: &str = "did:layerx:budget-payloads";

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("budget payload boundary: {error:?}"))
}

fn account(name: &str) -> AccountId {
    checked(AccountId::parse(name))
}

fn owner() -> AccountId {
    account(&format!("agent:{ACTOR}:main"))
}

fn budget_account() -> AccountId {
    account(&format!("agent:{ACTOR}:budget:{}", "12".repeat(32)))
}

fn account_id(value: &AccountId) -> [u8; 32] {
    checked(hash::account_id_for_protocol(value, 3))
}

fn registry() -> ModuleRegistry {
    let activities: Vec<_> = [1, 2, 8]
        .into_iter()
        .map(|ordinal| checked(ActivityType::new(ModuleId::Budget, ordinal)))
        .collect();
    checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Budget,
        &activities,
    ))]))
}

fn create(rollover: RolloverPolicy, carry: u128, period: u64, expiry: u64) -> Intent {
    Intent::v1(IntentKind::BudgetCreate(checked(BudgetCreate::new(
        BudgetId::new(BUDGET_ID),
        owner(),
        budget_account(),
        AssetId::new(ASSET),
        Amount::from_u128(AMOUNT),
        checked(PeriodLength::new(period)),
        rollover,
        Amount::from_u128(carry),
        PurposeHash::new(PURPOSE),
        TimestampSeconds::from_u64(expiry),
    ))))
}

fn fund(budget_id: [u8; 32], amount: u128) -> Intent {
    Intent::v1(IntentKind::BudgetFund(checked(BudgetFund::new(
        BudgetId::new(budget_id),
        owner(),
        budget_account(),
        AssetId::new(ASSET),
        Amount::from_u128(amount),
        IdempotencyKey::new([0x78; 32]),
    ))))
}

fn defund(amount: u128) -> Intent {
    Intent::v1(IntentKind::BudgetDefund(checked(BudgetDefund::new(
        BudgetId::new(BUDGET_ID),
        budget_account(),
        owner(),
        AssetId::new(ASSET),
        Amount::from_u128(amount),
        Sequence::from_u64(19),
        IdempotencyKey::new([0x9a; 32]),
    ))))
}

fn budget_call(compiled: &CompiledIntent) -> Eip1559Call {
    let payload = compiled.payload().as_bytes();
    assert_eq!(&payload[..2], &1_u16.to_be_bytes());
    let (signature, ranges): (&str, &[(usize, usize)]) = match compiled.activity_type().value() {
        0x0003_0001 => (
            "budgetCreate(bytes32,bytes32,bytes32,bytes32,uint128,uint128,uint128,uint64,uint64,uint64,uint64,uint8)",
            &[
                (2, 34), (34, 66), (66, 98), (98, 130),
                (130, 146), (146, 162), (162, 178),
                (178, 186), (186, 194), (194, 202), (202, 210), (210, 211),
            ],
        ),
        0x0003_0002 => ("budgetFund(bytes32,uint128)", &[(2, 34), (34, 50)]),
        0x0003_0008 => ("budgetDefund(bytes32,uint128)", &[(2, 34), (34, 50)]),
        activity => panic!("unexpected budget activity: {activity:#x}"),
    };
    assert_eq!(
        payload.len(),
        ranges.last().unwrap_or_else(|| panic!("budget fields")).1
    );
    let mut data = ledger_selector(signature).to_vec();
    for &(start, end) in ranges {
        let mut word = [0; 32];
        word[32 - (end - start)..].copy_from_slice(&payload[start..end]);
        data.extend_from_slice(&word);
    }
    let mut to = [0; 20];
    to[18..].copy_from_slice(&[0x10, 0x1b]);
    Eip1559Call {
        chain_id: 125,
        nonce: 23,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 30,
        gas_limit: 100_000,
        to,
        data,
    }
}

fn assert_compiled(
    intent: &Intent,
    registry: &ModuleRegistry,
    ordinal: u16,
    expected: &[u8],
) -> CompiledIntent {
    let compiled = checked(compile(intent, registry));
    assert_eq!(
        compiled.activity_type().value(),
        0x0003_0000 | u32::from(ordinal)
    );
    assert_eq!(compiled.payload().as_bytes(), expected);
    assert_eq!(
        compiled.payload_hash(),
        checked(hash::payload_hash_for(compiled.payload()))
    );
    checked(DisclosureCheck::verify(intent, &compiled));
    compiled
}

fn assert_disclosure(disclosure: &Disclosure, compiled: &CompiledIntent, call: &Eip1559Call) {
    assert_eq!(disclosure.activity_type, compiled.activity_type());
    assert_eq!(disclosure.canonical_payload(), call.data);
    assert_eq!(checked(disclosure.signing_digest()), call.signing_hash());
    assert_eq!(disclosure.actor, FROM);
    assert_eq!(disclosure.authority, FROM);
    assert_eq!(disclosure.envelope_sequence(), call.nonce);
    assert_eq!(
        disclosure.fee_limit,
        u128::from(call.gas_limit) * call.max_fee_per_gas
    );
}

fn assert_mutation_disclosure(disclosure: &Disclosure, amount: u128) {
    assert_eq!(disclosure.amounts.len(), 1);
    assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
    assert_eq!(disclosure.amounts[0].value, amount);
    assert_eq!(disclosure.counterparties.len(), 1);
    assert_eq!(disclosure.counterparties[0].role, CounterpartyRole::Payer);
    assert_eq!(disclosure.counterparties[0].account, BUDGET_ID);
    assert_eq!(disclosure.asset, [0; 32]);
    assert_eq!(disclosure.idempotency_key, [0; 32]);
}

#[test]
fn budget_fund_compiles_to_canonical_call_payload() {
    let registry = registry();
    for amount in [1, AMOUNT, u128::MAX] {
        let intent = fund(BUDGET_ID, amount);
        let mut expected = vec![0, 1];
        expected.extend_from_slice(&BUDGET_ID);
        expected.extend_from_slice(&amount.to_be_bytes());
        assert_eq!(expected.len(), 50);
        let compiled = assert_compiled(&intent, &registry, 2, &expected);
        let call = budget_call(&compiled);
        let disclosure = checked(bind_call(&call, FROM));
        assert_disclosure(&disclosure, &compiled, &call);
        assert_mutation_disclosure(&disclosure, amount);
        assert_eq!(
            checked(disclosure.construct_method()),
            Some(ConstructMethod::PayBudgetFund {
                budget: BUDGET_ID,
                amount
            })
        );
        assert!(DisclosureCheck::verify(&fund(BUDGET_ID, amount ^ 2), &compiled).is_err());
        assert!(DisclosureCheck::verify(&fund([0x13; 32], amount), &compiled).is_err());
        let mut zero = call.clone();
        zero.data[36..68].fill(0);
        assert_eq!(
            bind_call(&zero, FROM),
            Err(DisclosureError::MalformedPayload)
        );
        let mut trailing = call;
        trailing.data.push(0);
        assert_eq!(
            bind_call(&trailing, FROM),
            Err(DisclosureError::MalformedPayload)
        );
    }
}

fn assert_malformed_budget_create_call(call: &Eip1559Call) {
    for (rule, index, value) in [
        ("zero budget id", 0, 0_u128),
        ("zero budget account", 1, 0),
        ("zero asset", 2, 0),
        ("zero per-period limit", 4, 0),
        ("zero initial amount", 6, 0),
        ("zero period length", 7, 0),
        ("expiry equals period start", 9, 0),
        ("expiry before period start", 8, 1_900_000_000_001),
        ("rollover zero", 11, 0),
        ("rollover three", 11, 3),
    ] {
        let mut invalid = call.clone();
        let word = &mut invalid.data[4 + index * 32..4 + (index + 1) * 32];
        word.fill(0);
        word[16..].copy_from_slice(&value.to_be_bytes());
        assert_eq!(
            bind_call(&invalid, FROM),
            Err(DisclosureError::MalformedPayload),
            "{rule}"
        );
    }
    let mut invalid_carry = call.clone();
    for (index, width) in [
        (4, 16),
        (5, 16),
        (6, 16),
        (7, 8),
        (8, 8),
        (9, 8),
        (10, 8),
        (11, 1),
    ] {
        let mut noncanonical = invalid_carry.clone();
        noncanonical.data[4 + index * 32 + 31 - width] = 1;
        assert_eq!(
            bind_call(&noncanonical, FROM),
            Err(DisclosureError::MalformedPayload),
            "noncanonical integer at argument {index}"
        );
    }
    invalid_carry.data[4 + 5 * 32..4 + 6 * 32].fill(0);
    invalid_carry.data[4 + 6 * 32 - 1] = 1;
    invalid_carry.data[4 + 11 * 32..4 + 12 * 32].fill(0);
    invalid_carry.data[4 + 12 * 32 - 1] = 1;
    assert_eq!(
        bind_call(&invalid_carry, FROM),
        Err(DisclosureError::MalformedPayload)
    );
}

#[test]
fn budget_create_compiles_to_canonical_call_payload() {
    let registry = registry();
    for (rollover, carry, tag) in [
        (RolloverPolicy::None, 0, 1),
        (RolloverPolicy::Capped, CARRY, 2),
    ] {
        let intent = create(rollover, carry, 86_400, 1_900_000_000);
        let mut expected = vec![0, 1];
        expected.extend_from_slice(&BUDGET_ID);
        expected.extend_from_slice(&account_id(&budget_account()));
        expected.extend_from_slice(&ASSET);
        expected.extend_from_slice(&PURPOSE);
        expected.extend_from_slice(&AMOUNT.to_be_bytes());
        expected.extend_from_slice(&carry.to_be_bytes());
        expected.extend_from_slice(&AMOUNT.to_be_bytes());
        expected.extend_from_slice(&86_400_000_u64.to_be_bytes());
        expected.extend_from_slice(&0_u64.to_be_bytes());
        expected.extend_from_slice(&1_900_000_000_000_u64.to_be_bytes());
        expected.extend_from_slice(&0_u64.to_be_bytes());
        expected.push(tag);
        assert_eq!(expected.len(), 211);
        let compiled = assert_compiled(&intent, &registry, 1, &expected);
        let call = budget_call(&compiled);
        let disclosure = checked(bind_call(&call, FROM));
        assert_disclosure(&disclosure, &compiled, &call);
        assert_eq!(
            checked(disclosure.construct_method()),
            Some(ConstructMethod::PayBudgetCreate { payload: expected })
        );
        assert_eq!(disclosure.amounts.len(), 2);
        assert_eq!(disclosure.amounts[0].role, AmountRole::Transfer);
        assert_eq!(disclosure.amounts[0].value, AMOUNT);
        assert_eq!(disclosure.amounts[1].role, AmountRole::SpendingLimit);
        assert_eq!(disclosure.amounts[1].value, AMOUNT);
        assert_eq!(disclosure.counterparties.len(), 1);
        assert_eq!(
            disclosure.counterparties[0].role,
            CounterpartyRole::Recipient
        );
        assert_eq!(
            disclosure.counterparties[0].account,
            account_id(&budget_account())
        );
        assert_eq!(disclosure.expiry.payload_expires_at, 1_900_000_000_000);
        assert_eq!(disclosure.asset, ASSET);
        assert_eq!(disclosure.idempotency_key, BUDGET_ID);
        assert_malformed_budget_create_call(&call);
        assert!(DisclosureCheck::verify(
            &create(rollover, carry, 86_401, 1_900_000_000),
            &compiled
        )
        .is_err());
        assert!(DisclosureCheck::verify(
            &create(rollover, carry, 86_400, 1_900_000_001),
            &compiled
        )
        .is_err());
    }
}

#[test]
fn budget_defund_compiles_to_canonical_call_payload() {
    let registry = registry();
    for amount in [1, AMOUNT, u128::MAX] {
        let intent = defund(amount);
        let mut expected = vec![0, 1];
        expected.extend_from_slice(&BUDGET_ID);
        expected.extend_from_slice(&amount.to_be_bytes());
        assert_eq!(expected.len(), 50);
        let compiled = assert_compiled(&intent, &registry, 8, &expected);
        let call = budget_call(&compiled);
        let disclosure = checked(bind_call(&call, FROM));
        assert_disclosure(&disclosure, &compiled, &call);
        assert_mutation_disclosure(&disclosure, amount);
        assert_eq!(
            checked(disclosure.construct_method()),
            Some(ConstructMethod::PayBudgetDefund {
                budget: BUDGET_ID,
                amount
            })
        );
        assert!(DisclosureCheck::verify(&defund(amount ^ 2), &compiled).is_err());
        let mut zero = call.clone();
        zero.data[36..68].fill(0);
        assert_eq!(
            bind_call(&zero, FROM),
            Err(DisclosureError::MalformedPayload)
        );
        let mut trailing = call;
        trailing.data.push(0);
        assert_eq!(
            bind_call(&trailing, FROM),
            Err(DisclosureError::MalformedPayload)
        );
    }
}

#[test]
fn budget_create_rejects_millisecond_overflow() {
    let registry = registry();
    for (period, expiry, field) in [
        (u64::MAX, 1_900_000_000, CompileField::Period),
        (86_400, u64::MAX, CompileField::Expiration),
    ] {
        let intent = create(RolloverPolicy::None, 0, period, expiry);
        let error = compile(&intent, &registry)
            .err()
            .unwrap_or_else(|| panic!("overflow must refuse compilation"));
        assert_eq!(error.field, field);
    }
    let maximum_seconds = u64::MAX / 1_000;
    let intent = create(RolloverPolicy::None, 0, maximum_seconds, maximum_seconds);
    let compiled = checked(compile(&intent, &registry));
    checked(DisclosureCheck::verify(&intent, &compiled));
    let call = budget_call(&compiled);
    let disclosure = checked(bind_call(&call, FROM));
    assert_disclosure(&disclosure, &compiled, &call);
    let payload = compiled.payload().as_bytes();
    assert_eq!(&payload[178..186], &(maximum_seconds * 1_000).to_be_bytes());
    assert_eq!(&payload[194..202], &(maximum_seconds * 1_000).to_be_bytes());
}
