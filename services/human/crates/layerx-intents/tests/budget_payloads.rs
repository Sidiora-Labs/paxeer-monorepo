use std::fmt::Debug;

use layerx_crypto::disclosure::{
    bind, bind_budget_mutation, BudgetStateContext, DisclosedNativeOperation, Disclosure,
};
use layerx_intents::{
    compile, BudgetCreate, BudgetDefund, BudgetFund, CompileField, CompiledIntent, DisclosureCheck,
    Intent, IntentKind,
};
use layerx_types::account::AccountId;
use layerx_types::activity::{Authority, EnvelopeBuilder, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, Did, IdempotencyKey};
use layerx_types::intent::{
    BudgetId, PeriodLength, PurposeHash, RolloverPolicy, Sequence, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use layerx_wire::{activity::encode_unsigned_envelope, hash};

const BUDGET_ID: [u8; 32] = [0x12; 32];
const ASSET: [u8; 32] = [0x34; 32];
const PURPOSE: [u8; 32] = [0x56; 32];
const AMOUNT: u128 = 0x0102_0304_0506_0708_090a_0b0c_0d0e_0f10;
const CARRY: u128 = 0x0102_0304_0506_0708;
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

fn context() -> BudgetStateContext {
    BudgetStateContext {
        budget_id: BUDGET_ID,
        owner: account_id(&owner()),
        budget_account: account_id(&budget_account()),
        asset: ASSET,
        source_account: account_id(&owner()),
        native_source: false,
        revocation_sequence: 18,
        balance: u128::MAX,
        state_digest: [0xbc; 32],
        observed_head_sequence: 77,
        purpose_hash: PURPOSE,
    }
}

fn unsigned(compiled: &CompiledIntent) -> Vec<u8> {
    let mut builder = EnvelopeBuilder::new();
    checked(builder.protocol_version(3));
    checked(builder.network_id(17));
    checked(builder.activity_type(compiled.activity_type()));
    checked(builder.actor_did(checked(Did::new(ACTOR.as_bytes()))));
    checked(builder.authority(checked(Authority::owner(&[0x42; 32]))));
    checked(builder.account_sequence(23));
    checked(builder.timestamp_bound(checked(TimestampBound::new(1_000, 2_000))));
    checked(builder.idempotency_key(IdempotencyKey::new([0x78; 32])));
    checked(builder.fee_limit(Amount::from_u128(5)));
    checked(builder.payload_hash(compiled.payload_hash()));
    checked(builder.payload(compiled.payload().clone()));
    checked(encode_unsigned_envelope(&checked(builder.build())))
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

fn assert_disclosure(disclosure: &Disclosure, compiled: &CompiledIntent, bytes: &[u8]) {
    assert_eq!(
        disclosure.canonical_payload(),
        compiled.payload().as_bytes()
    );
    assert_eq!(checked(disclosure.reencode()), bytes);
}

#[test]
fn budget_fund_compiles_to_the_kernel_canonical_payload() {
    let registry = registry();
    for amount in [1, AMOUNT, u128::MAX] {
        let intent = fund(BUDGET_ID, amount);
        let mut expected = vec![0, 1];
        expected.extend_from_slice(&BUDGET_ID);
        expected.extend_from_slice(&amount.to_be_bytes());
        assert_eq!(expected.len(), 50);
        let compiled = assert_compiled(&intent, &registry, 2, &expected);
        let bytes = unsigned(&compiled);
        assert!(bind(&bytes, &registry).is_err());
        let disclosure = checked(bind_budget_mutation(&bytes, &registry, &context()));
        assert_disclosure(&disclosure, &compiled, &bytes);
        let Some(DisclosedNativeOperation::BudgetFund(value)) = &disclosure.native_operation else {
            panic!("expected canonical fund disclosure");
        };
        assert_eq!(value.encoding_version, 1);
        assert_eq!(value.budget_id, BUDGET_ID);
        assert_eq!(value.amount, amount);
        assert_eq!(value.source_sequence, None);
        assert_eq!(value.context, context());
        assert!(DisclosureCheck::verify(&fund(BUDGET_ID, amount ^ 2), &compiled).is_err());
        assert!(DisclosureCheck::verify(&fund([0x13; 32], amount), &compiled).is_err());
        let mut wrong = context();
        wrong.budget_id[0] ^= 1;
        assert!(bind_budget_mutation(&bytes, &registry, &wrong).is_err());
        wrong = context();
        wrong.native_source = true;
        assert!(bind_budget_mutation(&bytes, &registry, &wrong).is_err());
    }
}

#[test]
fn budget_create_compiles_to_the_kernel_canonical_payload() {
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
        let bytes = unsigned(&compiled);
        let disclosure = checked(bind(&bytes, &registry));
        assert_disclosure(&disclosure, &compiled, &bytes);
        let Some(DisclosedNativeOperation::BudgetCreate(value)) = &disclosure.native_operation
        else {
            panic!("expected canonical create disclosure");
        };
        assert_eq!(value.encoding_version, 1);
        assert_eq!(value.budget_id, BUDGET_ID);
        assert_eq!(value.budget_account, account_id(&budget_account()));
        assert_eq!(value.asset, ASSET);
        assert_eq!(value.purpose, PURPOSE);
        assert_eq!(value.per_period_limit, AMOUNT);
        assert_eq!(value.carry_cap, carry);
        assert_eq!(value.initial_amount, AMOUNT);
        assert_eq!(value.period_length_ms, 86_400_000);
        assert_eq!(value.period_start_ms, 0);
        assert_eq!(value.expiry_ms, 1_900_000_000_000);
        assert_eq!(value.revocation_sequence, 0);
        assert_eq!(value.rollover, tag);
        assert_eq!(value.source_account, account_id(&owner()));
        assert_eq!(value.source_sequence, 23);
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
fn budget_defund_compiles_to_the_kernel_canonical_payload() {
    let registry = registry();
    for amount in [1, AMOUNT, u128::MAX] {
        let intent = defund(amount);
        let mut expected = vec![0, 1];
        expected.extend_from_slice(&BUDGET_ID);
        expected.extend_from_slice(&amount.to_be_bytes());
        assert_eq!(expected.len(), 50);
        let compiled = assert_compiled(&intent, &registry, 8, &expected);
        let bytes = unsigned(&compiled);
        assert!(bind(&bytes, &registry).is_err());
        let disclosure = checked(bind_budget_mutation(&bytes, &registry, &context()));
        assert_disclosure(&disclosure, &compiled, &bytes);
        let Some(DisclosedNativeOperation::BudgetDefund(value)) = &disclosure.native_operation
        else {
            panic!("expected canonical defund disclosure");
        };
        assert_eq!(value.budget_id, BUDGET_ID);
        assert_eq!(value.amount, amount);
        assert_eq!(value.context, context());
        assert!(DisclosureCheck::verify(&defund(amount ^ 2), &compiled).is_err());
        let mut wrong = context();
        wrong.balance = amount - 1;
        assert!(bind_budget_mutation(&bytes, &registry, &wrong).is_err());
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
        let error = compile(&intent, &registry).expect_err("overflow must refuse compilation");
        assert_eq!(error.field, field);
    }
    let maximum_seconds = u64::MAX / 1_000;
    let intent = create(RolloverPolicy::None, 0, maximum_seconds, maximum_seconds);
    let compiled = checked(compile(&intent, &registry));
    checked(DisclosureCheck::verify(&intent, &compiled));
    let bytes = unsigned(&compiled);
    let disclosure = checked(bind(&bytes, &registry));
    assert_disclosure(&disclosure, &compiled, &bytes);
    let payload = compiled.payload().as_bytes();
    assert_eq!(&payload[178..186], &(maximum_seconds * 1_000).to_be_bytes());
    assert_eq!(&payload[194..202], &(maximum_seconds * 1_000).to_be_bytes());
}
