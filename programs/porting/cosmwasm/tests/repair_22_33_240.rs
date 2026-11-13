use std::fmt::Debug;

use layerx_program_sdk::{
    payments::PreparedProgramAccount, AccountId, Amount, AssetId, ProgramId as SdkProgramId,
};
use layerx_programs_runtime::{
    abi::UnavailableReceiptOracle,
    storage::{
        ScanLimits, StorageError, MAX_STORAGE_KEY_BYTES, MAX_STORAGE_SCAN_CURSOR_BYTES,
        MAX_STORAGE_VALUE_BYTES,
    },
    Abi, AbiError, AuthorizationContext, AuthorizedExecutionRequest, CallFrameId, Capability,
    CapabilitySet, CompositionContext, Executor, Meter, PrincipalId, ProgramId, Storage,
    StorageNamespace, StorageSelector, TransferSource, V2ActivityOutcome, WasmEngine, ABI_VERSION,
    CALL_ENTRY_EXPORT,
};
use programs_porting_cosmwasm::{
    item_key, map_prefix,
    shared_orderbook::OrderBook,
    wasm::{Code, ModuleBuilder, I32, I64},
    PortRefusal, ProgramAccountTransferPlan, StateBinding, TranslatedValueFlow, ValueFlow,
};

const GUIDE: &str = include_str!("../MIGRATION.md");

fn checked<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("unexpected refusal: {error:?}"))
}

fn program() -> ProgramId {
    checked(ProgramId::new([1; 32]))
}

fn actor(byte: u8) -> PrincipalId {
    checked(PrincipalId::new([byte; 32]))
}

fn abi(storage: Storage, owner: ProgramId, principal: PrincipalId, grants: &[Capability]) -> Abi {
    checked(Abi::new(
        ABI_VERSION,
        owner,
        AuthorizationContext::new(principal, checked(CapabilitySet::new(grants.to_vec()))),
        storage,
        &UnavailableReceiptOracle,
    ))
}

#[test]
fn cosmwasm_storage_guide_global_item_preserves_cross_sender_state_and_grants() {
    assert!(GUIDE.contains("`Item<T>` holding a mutable global aggregate"));
    assert!(GUIDE.contains("StateBinding::Shared.layerx_key(\"order_count\", &[])"));
    assert!(GUIDE.contains("fn increment_order_count()"));
    let binding = OrderBook::order_count_binding();
    assert_eq!(binding, StateBinding::Shared);
    assert!(binding.shared());
    let key = checked(binding.layerx_key("order_count", &[]));
    assert_eq!(key, checked(item_key("order_count")));
    assert_eq!(key, checked(OrderBook::order_count_key()));
    let grants = [
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
    ];
    let mut storage = Storage::new();
    for (sender, expected) in [(actor(2), 1u64), (actor(3), 2u64)] {
        let mut invocation = abi(storage, program(), sender, &grants);
        let mut meter = Meter::declared();
        let prior =
            checked(invocation.storage_read_selected(&mut meter, StorageSelector::Shared, &key));
        let count = match prior {
            None => 0,
            Some(bytes) => u64::from_le_bytes(checked(bytes.try_into())),
        };
        let next = count
            .checked_add(1)
            .unwrap_or_else(|| panic!("counter overflow"));
        assert_eq!(next, expected);
        checked(invocation.storage_write_selected(
            &mut meter,
            StorageSelector::Shared,
            &key,
            &next.to_le_bytes(),
        ));
        storage = invocation.commit().storage;
    }
    assert_eq!(
        checked(
            storage
                .transaction(StorageNamespace::shared(program()))
                .read(&key)
        ),
        Some(2u64.to_le_bytes().to_vec())
    );
    for sender in [actor(2), actor(3)] {
        assert_eq!(
            checked(
                storage
                    .transaction(StorageNamespace::principal(program(), sender))
                    .read(&key)
            ),
            None
        );
    }
    for grants in [
        vec![Capability::StorageRead, Capability::StorageWrite],
        vec![],
    ] {
        let mut denied = abi(storage.clone(), program(), actor(2), &grants);
        assert_eq!(
            denied.storage_read_selected(&mut Meter::declared(), StorageSelector::Shared, &key),
            Err(AbiError::CapabilityDenied)
        );
        assert_eq!(
            denied.storage_write_selected(
                &mut Meter::declared(),
                StorageSelector::Shared,
                &key,
                &[0; 8]
            ),
            Err(AbiError::CapabilityDenied)
        );
        assert_eq!(denied.commit().storage, storage);
    }
    let mut read_only = abi(
        storage.clone(),
        program(),
        actor(3),
        &[Capability::SharedStorageRead],
    );
    assert_eq!(
        checked(read_only.storage_read_selected(
            &mut Meter::declared(),
            StorageSelector::Shared,
            &key
        )),
        Some(2u64.to_le_bytes().to_vec())
    );
    assert_eq!(
        read_only.storage_write_selected(
            &mut Meter::declared(),
            StorageSelector::Shared,
            &key,
            &[0; 8]
        ),
        Err(AbiError::CapabilityDenied)
    );
    assert_eq!(read_only.commit().storage, storage);
}

fn check_scan_limits() {
    for text in [
        "storage_scan_scoped",
        "1–256 bytes",
        "0–1,048,576 bytes",
        "5–67,126,228",
        "at most 591 bytes",
        "not a snapshot",
    ] {
        assert!(
            GUIDE.contains(text),
            "missing documented scan contract: {text}"
        );
    }
    assert_eq!(MAX_STORAGE_KEY_BYTES, 256);
    assert_eq!(MAX_STORAGE_VALUE_BYTES, 1_048_576);
    assert_eq!(MAX_STORAGE_SCAN_CURSOR_BYTES, 591);
    for (entries, bytes) in [(0, 512), (65, 512), (1, 4), (1, 67_126_229)] {
        assert_eq!(
            ScanLimits::new(entries, bytes),
            Err(StorageError::InvalidScanLimits)
        );
    }
    assert!(ScanLimits::new(64, 67_126_228).is_ok());
}

#[test]
fn cosmwasm_storage_guide_scans_enforce_scope_bounds_and_cursor_contract() {
    check_scan_limits();
    let mut storage = Storage::new();
    {
        let mut tx = storage.transaction(StorageNamespace::shared(program()));
        checked(tx.write(&[b'x'; 256], &vec![0; 1_048_576]));
        assert_eq!(tx.write(&[], b""), Err(StorageError::EmptyKey));
        assert_eq!(tx.write(&[b'x'; 257], b""), Err(StorageError::KeyTooLarge));
        assert_eq!(
            tx.write(b"oversized", &vec![0; 1_048_577]),
            Err(StorageError::ValueTooLarge)
        );
        assert_eq!(tx.commit(), 1);
    }
    let prefix = checked(map_prefix("orders"));
    let first_key = checked(OrderBook::order_key(1));
    let second_key = checked(OrderBook::order_key(2));
    for selector in [StorageSelector::Principal, StorageSelector::Shared] {
        let mut invocation = abi(
            storage,
            program(),
            actor(2),
            &[
                Capability::StorageRead,
                Capability::StorageWrite,
                Capability::SharedStorageRead,
                Capability::SharedStorageWrite,
            ],
        );
        let mut meter = Meter::declared();
        // Insert out of order through the actual scope-aware ABI writer.
        checked(invocation.storage_write_selected(&mut meter, selector, &second_key, b"second"));
        checked(invocation.storage_write_selected(&mut meter, selector, &first_key, b"first"));
        let limits = checked(ScanLimits::new(1, 512));
        let first =
            checked(invocation.storage_scan_selected(&mut meter, selector, &prefix, &[], limits));
        assert_eq!(first.entries().len(), 1);
        assert_eq!(first.entries()[0].key, first_key);
        assert_eq!(first.entries()[0].value, b"first");
        let cursor = first
            .cursor()
            .unwrap_or_else(|| panic!("continuation missing"))
            .to_vec();
        assert!(cursor.len() <= 591);
        assert!(first.metered_bytes() <= 512);
        assert_eq!(
            invocation.storage_scan_selected(
                &mut meter,
                selector,
                &prefix,
                &[],
                checked(ScanLimits::new(1, 5))
            ),
            Err(AbiError::Storage(StorageError::ScanCeilingExceeded))
        );
        for (changed_prefix, changed_limits) in [
            (b"different".as_slice(), limits),
            (prefix.as_slice(), checked(ScanLimits::new(2, 512))),
            (prefix.as_slice(), checked(ScanLimits::new(1, 513))),
        ] {
            assert_eq!(
                invocation.storage_scan_selected(
                    &mut meter,
                    selector,
                    changed_prefix,
                    &cursor,
                    changed_limits
                ),
                Err(AbiError::Storage(StorageError::InvalidScanCursor))
            );
        }
        assert_eq!(
            invocation.storage_scan_selected(&mut meter, selector, &[0; 257], &[], limits),
            Err(AbiError::AccessDeclaration)
        );
        let maximum_prefix =
            checked(invocation.storage_scan_selected(&mut meter, selector, &[0; 256], &[], limits));
        assert!(maximum_prefix.entries().is_empty());
        assert_eq!(maximum_prefix.cursor(), None);
        storage = invocation.commit().storage;
        storage = check_scan_continuation(storage, selector, &prefix, &cursor, limits, &second_key);
    }
}

fn check_scan_continuation(
    mut storage: Storage,
    selector: StorageSelector,
    prefix: &[u8],
    cursor: &[u8],
    limits: ScanLimits,
    second_key: &[u8],
) -> Storage {
    // A fresh invocation consumes the real store-issued opaque continuation.
    let mut next = abi(
        storage,
        program(),
        actor(2),
        &[Capability::StorageRead, Capability::SharedStorageRead],
    );
    let second = checked(next.storage_scan_selected(
        &mut Meter::declared(),
        selector,
        prefix,
        cursor,
        limits,
    ));
    assert_eq!(second.entries().len(), 1);
    assert_eq!(second.entries()[0].key, second_key);
    assert_eq!(second.entries()[0].value, b"second");
    assert_eq!(second.cursor(), None);
    storage = next.commit().storage;
    let mut other_sender = abi(
        storage.clone(),
        program(),
        actor(3),
        &[Capability::StorageRead, Capability::SharedStorageRead],
    );
    let resumed = other_sender.storage_scan_selected(
        &mut Meter::declared(),
        selector,
        prefix,
        cursor,
        limits,
    );
    if selector == StorageSelector::Shared {
        assert_eq!(checked(resumed), second);
    } else {
        assert_eq!(
            resumed,
            Err(AbiError::Storage(StorageError::InvalidScanCursor))
        );
        assert!(checked(other_sender.storage_scan_selected(
            &mut Meter::declared(),
            selector,
            prefix,
            &[],
            limits
        ))
        .entries()
        .is_empty());
    }
    let mut other_program = abi(
        storage.clone(),
        checked(ProgramId::new([9; 32])),
        actor(2),
        &[Capability::StorageRead, Capability::SharedStorageRead],
    );
    assert_eq!(
        other_program.storage_scan_selected(
            &mut Meter::declared(),
            selector,
            prefix,
            cursor,
            limits
        ),
        Err(AbiError::Storage(StorageError::InvalidScanCursor))
    );
    let wrong_scope = if selector == StorageSelector::Shared {
        StorageSelector::Principal
    } else {
        StorageSelector::Shared
    };
    assert_eq!(
        other_sender.storage_scan_selected(
            &mut Meter::declared(),
            wrong_scope,
            prefix,
            cursor,
            limits
        ),
        Err(AbiError::Storage(StorageError::InvalidScanCursor))
    );
    let wrong_grant = if selector == StorageSelector::Shared {
        Capability::StorageRead
    } else {
        Capability::SharedStorageRead
    };
    let mut denied = abi(storage.clone(), program(), actor(2), &[wrong_grant]);
    assert_eq!(
        denied.storage_scan_selected(&mut Meter::declared(), selector, prefix, &[], limits),
        Err(AbiError::CapabilityDenied)
    );
    storage
}

fn finish_guest(mut builder: ModuleBuilder, code: &Code) -> Vec<u8> {
    let reserve_type = builder.signature(&[I32], &[I32]);
    let mut reserve = Code::new();
    reserve.i32_const(4096);
    reserve.end();
    let reserve_index = builder.function(reserve_type, &[], &reserve);
    let entry_type = builder.signature(&[I32, I32], &[I32]);
    let entry_index = builder.function(entry_type, &[], code);
    builder.export_function("layerx_reserve", reserve_index);
    builder.export_function(CALL_ENTRY_EXPORT, entry_index);
    builder.export_memory("memory");
    builder.finish()
}

fn funding_guest(deposit: layerx_program_sdk::ProgramDeposit<'_>) -> Vec<u8> {
    let mut builder = ModuleBuilder::new(1);
    let signature = builder.signature(&[I64, I64, I32, I32, I32, I32, I32, I32], &[I32]);
    let fund = builder.import("layerx_v2", "fund_program_402", signature);
    builder.segment(0, deposit.seed().bytes());
    builder.segment(256, &deposit.destination().bytes());
    builder.segment(288, &deposit.asset().bytes());
    let mut code = Code::new();
    let (high, low) = deposit.amount().split();
    code.i64_const(high);
    code.i64_const(low);
    for value in [
        0,
        checked(i32::try_from(deposit.seed().bytes().len())),
        256,
        32,
        288,
        32,
    ] {
        code.i32_const(value);
    }
    code.call(fund);
    code.trap_unless_ok();
    code.i32_const(0);
    code.end();
    finish_guest(builder, &code)
}

fn withdrawal_guest(plan: &ProgramAccountTransferPlan, repetitions: usize) -> Vec<u8> {
    let mut builder = ModuleBuilder::new(1);
    let signature = builder.signature(&[I64, I64, I32, I32, I32, I32, I32, I32, I32, I32], &[I32]);
    let pay = builder.import("layerx_v2", "transfer_program_402", signature);
    builder.segment(0, plan.seed());
    builder.segment(256, &plan.source());
    builder.segment(288, &plan.asset());
    builder.segment(320, &plan.to());
    let mut code = Code::new();
    let (high, low) = Amount::from_u128(plan.amount()).split();
    for _ in 0..repetitions {
        code.i64_const(high);
        code.i64_const(low);
        for value in [
            0,
            checked(i32::try_from(plan.seed().len())),
            256,
            32,
            288,
            32,
            320,
            32,
        ] {
            code.i32_const(value);
        }
        code.call(pay);
        code.trap_unless_ok();
    }
    code.i32_const(0);
    code.end();
    finish_guest(builder, &code)
}

fn execute(
    wasm: &[u8],
    owner: ProgramId,
    principal: PrincipalId,
    capabilities: CapabilitySet,
) -> layerx_programs_runtime::CandidateAuthorizedExecutionRecord {
    execute_in(&mut Storage::new(), wasm, owner, principal, capabilities)
}

fn execute_in(
    storage: &mut Storage,
    wasm: &[u8],
    owner: ProgramId,
    principal: PrincipalId,
    capabilities: CapabilitySet,
) -> layerx_programs_runtime::CandidateAuthorizedExecutionRecord {
    checked(try_execute_in(
        storage,
        wasm,
        owner,
        principal,
        capabilities,
    ))
}

fn try_execute_in(
    storage: &mut Storage,
    wasm: &[u8],
    owner: ProgramId,
    principal: PrincipalId,
    capabilities: CapabilitySet,
) -> Result<
    layerx_programs_runtime::CandidateAuthorizedExecutionRecord,
    layerx_programs_runtime::ExecutionError,
> {
    let engine = checked(WasmEngine::declared());
    let module = checked(engine.validate_candidate_v2(wasm));
    Executor::declared().execute_authorized_candidate(
        storage,
        AuthorizedExecutionRequest {
            module: &module,
            program: owner,
            authorization: AuthorizationContext::new(principal, capabilities),
            receipts: &UnavailableReceiptOracle,
            entrypoint: CALL_ENTRY_EXPORT,
            calldata: &[],
            composition: CompositionContext::isolated(),
            response_capacity: 512,
        },
    )
}

#[test]
fn cosmwasm_storage_guide_scoped_scan_crosses_real_guest_host_boundary() {
    let key = checked(OrderBook::order_key(1));
    let prefix = checked(map_prefix("orders"));
    let mut producer = abi(
        Storage::new(),
        program(),
        actor(2),
        &[Capability::SharedStorageWrite],
    );
    checked(producer.storage_write_selected(
        &mut Meter::declared(),
        StorageSelector::Shared,
        &key,
        b"order",
    ));
    let mut storage = producer.commit().storage;
    let mut builder = ModuleBuilder::new(1);
    let scan_type = builder.signature(&[I32; 9], &[I32]);
    let scan = builder.import("layerx_v2", "storage_scan_scoped", scan_type);
    let response_type = builder.signature(&[I32; 3], &[I32]);
    let respond = builder.import("layerx_v2", "response_write", response_type);
    builder.segment(0, &prefix);
    let mut code = Code::new();
    // Keep the host-produced page length in guest memory for response transport.
    code.i32_const(768);
    for value in [
        2,
        0,
        checked(i32::try_from(prefix.len())),
        512,
        0,
        1,
        512,
        1024,
        512,
    ] {
        code.i32_const(value);
    }
    code.call(scan);
    code.memory(programs_porting_cosmwasm::wasm::I32_STORE, 0);
    code.i32_const(768);
    code.memory(programs_porting_cosmwasm::wasm::I32_LOAD, 0);
    code.i32_const(0);
    code.op(programs_porting_cosmwasm::wasm::I32_LT_S);
    code.trap_if();
    code.i32_const(0);
    code.i32_const(1024);
    code.i32_const(768);
    code.memory(programs_porting_cosmwasm::wasm::I32_LOAD, 0);
    code.call(respond);
    code.trap_unless_ok();
    code.i32_const(0);
    code.end();
    let wasm = finish_guest(builder, &code);
    let record = execute_in(
        &mut storage,
        &wasm,
        program(),
        actor(3),
        checked(CapabilitySet::new([Capability::SharedStorageRead])),
    );
    let response = record
        .response()
        .unwrap_or_else(|| panic!("missing page: {:?}", record.outcome()));
    let mut expected = 1u16.to_be_bytes().to_vec();
    expected.extend_from_slice(&checked(u16::try_from(key.len())).to_be_bytes());
    expected.extend_from_slice(&key);
    expected.extend_from_slice(&5u32.to_be_bytes());
    expected.extend_from_slice(b"order");
    expected.extend_from_slice(&[0, 0, 0]);
    assert_eq!(response.bytes, expected);
    let before = storage.clone();
    let denied = execute_in(
        &mut storage,
        &wasm,
        program(),
        actor(3),
        checked(CapabilitySet::new([Capability::StorageRead])),
    );
    assert!(matches!(denied.outcome(), V2ActivityOutcome::Failure(_)));
    assert!(denied.response().is_none());
    assert_eq!(storage, before);
}

fn verify_custody_withdrawal(vault: &PreparedProgramAccount, asset: AssetId, recipient: AccountId) {
    let flow = ValueFlow::BankSend {
        recipient: recipient.bytes(),
        amount: 5,
    };
    let translated = checked(flow.translate_with_program_account(
        asset.bytes(),
        actor(3).bytes(),
        program(),
        b"vault",
        vault.account().bytes(),
    ));
    let TranslatedValueFlow::ProgramAccount(plan) = translated else {
        panic!("withdraw must use program custody")
    };
    let spend = checked(vault.spend_grant(recipient, Amount::from_u128(5)));
    let decoded = checked(CapabilitySet::decode_v2_canonical(spend.as_slice()));
    assert_eq!(decoded, vec![plan.capability()]);
    let grants = checked(CapabilitySet::new(decoded));
    let withdrawal = execute(
        &withdrawal_guest(&plan, 1),
        program(),
        actor(3),
        grants.clone(),
    );
    let effects = withdrawal
        .effects()
        .unwrap_or_else(|| panic!("withdrawal failed: {:?}", withdrawal.outcome()));
    assert_eq!(effects.transfers.len(), 1);
    let payment = &effects.transfers[0];
    assert_eq!(payment.principal, actor(3));
    assert_eq!(payment.amount, 5);
    assert_eq!(payment.to, recipient.bytes());
    let TransferSource::Program(authority) = payment.source() else {
        panic!("withdrawal must debit the vault, not the current caller")
    };
    assert_eq!(authority.owner_program(), program());
    assert_eq!(authority.source_account(), vault.account().bytes());
    assert_eq!(authority.seed(), b"vault");
    assert_eq!(authority.asset(), asset.bytes());
    assert_eq!(authority.to(), recipient.bytes());
    assert_eq!(authority.amount(), 5);
    assert_eq!(authority.staging_frame(), CallFrameId::root());
    // A foreign program's root grant is rejected before execution.
    let mut foreign_storage = Storage::new();
    let before = foreign_storage.clone();
    assert_eq!(
        try_execute_in(
            &mut foreign_storage,
            &withdrawal_guest(&plan, 1),
            checked(ProgramId::new([9; 32])),
            actor(3),
            grants.clone(),
        ),
        Err(layerx_programs_runtime::ExecutionError::Abi(
            AbiError::InvalidCapability
        ))
    );
    assert_eq!(foreign_storage, before);
    // A second request cannot reuse the full ceiling, nor can principal-funded
    // authority stand in for the exact program grant.
    for (wasm, owner, authorized) in [
        (withdrawal_guest(&plan, 2), program(), grants.clone()),
        (
            withdrawal_guest(&plan, 1),
            program(),
            CapabilitySet::empty(),
        ),
        (
            withdrawal_guest(&plan, 1),
            program(),
            checked(CapabilitySet::new([Capability::Transfer402 {
                asset: asset.bytes(),
                to: recipient.bytes(),
                maximum_amount: 5,
            }])),
        ),
    ] {
        let denied = execute(&wasm, owner, actor(3), authorized);
        assert!(matches!(denied.outcome(), V2ActivityOutcome::Failure(_)));
        assert!(
            denied.effects().is_none(),
            "refusal must discard even an earlier staged payment"
        );
    }
    assert_eq!(
        flow.translate(asset.bytes(), actor(3).bytes()),
        Err(PortRefusal::ContractHeldBalance)
    );
    assert_eq!(
        flow.translate_with_program_account(
            asset.bytes(),
            actor(3).bytes(),
            program(),
            b"other",
            vault.account().bytes()
        ),
        Err(PortRefusal::InvalidProgramAccount)
    );
}

#[test]
fn cosmwasm_custody_guide_prepares_registration_funds_and_bounds_later_withdrawal() {
    for text in [
        "registration_payload()",
        "funding_grant(Amount::from_u128(30))",
        "spend_grant(recipient, Amount::from_u128(5))",
        "transfer::fund_program_account",
        "transfer::pay_from_program_account",
        "`MessageInfo.funds` remains unavailable",
    ] {
        assert!(
            GUIDE.contains(text),
            "missing documented custody phase: {text}"
        );
    }
    let asset = checked(AssetId::new([4; 32]));
    let recipient = checked(AccountId::new([5; 32]));
    let vault = checked(PreparedProgramAccount::new(
        checked(SdkProgramId::new(program().bytes())),
        b"vault",
        asset,
    ));
    let registration = checked(vault.registration_payload());
    // Freeze the real SDK producer's native registration wire layout.
    let mut expected_registration = program().bytes().to_vec();
    expected_registration.extend_from_slice(b"LXPA1");
    expected_registration.extend_from_slice(&asset.bytes());
    expected_registration.extend_from_slice(&5u32.to_be_bytes());
    expected_registration.extend_from_slice(b"vault");
    assert_eq!(registration.as_slice(), expected_registration);

    let deposit = checked(vault.deposit(Amount::from_u128(30)));
    let funding_grant = checked(vault.funding_grant(Amount::from_u128(30)));
    let mut encoded = [0; 128];
    let mut sdk_grants = layerx_program_sdk::CapabilitySet::<1>::empty();
    checked(sdk_grants.insert(funding_grant));
    let used = checked(sdk_grants.encode_into(&mut encoded));
    let funding_grants = checked(CapabilitySet::new(checked(
        CapabilitySet::decode_v2_canonical(&encoded[..used]),
    )));
    let funded = execute(&funding_guest(deposit), program(), actor(2), funding_grants);
    let funding_effects = funded
        .effects()
        .unwrap_or_else(|| panic!("funding failed: {:?}", funded.outcome()));
    assert_eq!(funding_effects.transfers.len(), 1);
    let funding = &funding_effects.transfers[0];
    assert_eq!(funding.amount, 30);
    assert_eq!(funding.to, vault.account().bytes());
    assert_eq!(funding.asset, asset.bytes());
    let TransferSource::ProgramFunding { principal, binding } = funding.source() else {
        panic!("deposit must debit the caller")
    };
    assert_eq!(*principal, actor(2));
    assert_eq!(binding.owner_program(), program());
    assert_eq!(binding.seed(), b"vault");
    assert_eq!(binding.destination_account(), vault.account().bytes());
    assert_eq!(binding.asset(), asset.bytes());
    let denied_funding = execute(
        &funding_guest(deposit),
        program(),
        actor(2),
        CapabilitySet::empty(),
    );
    assert!(matches!(
        denied_funding.outcome(),
        V2ActivityOutcome::Failure(_)
    ));
    assert!(denied_funding.effects().is_none());

    verify_custody_withdrawal(&vault, asset, recipient);
}
