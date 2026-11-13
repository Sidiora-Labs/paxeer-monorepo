//! Integration coverage for JSON migration, order book settlement, and shared storage.
use layerx_programs_runtime::{
    AuthorizationContext, AuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, Executor, Lifecycle, PrincipalId, ProgramId, Storage, StorageNamespace,
    V2ActivityOutcome, V2AuthorizedExecutionRecord, ValidatedModule, WasmEngine, WasmValue,
};
use programs_porting_cosmwasm::{
    json::{MAX_JSON_BYTES, MAX_TEXT_BYTES},
    qualify::{execute_donations, validated_module},
    shared_orderbook::{
        BoundedOrderBook, OrderBook, OrderBookTerms, MATCH_ORDER, MAX_ORDER_LOTS, PLACE_ORDER,
        READ_ORDER,
    },
    AbsentReceipts, DonationPort, DonationTerms, FieldSchema, FieldValue, Invocation,
    MigrationCell, PortRefusal, RecordSchema, StateBinding, ValueType,
};
use std::{
    fmt::Debug,
    fs,
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

fn ok<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("production path refused: {error:?}"))
}

fn actor(byte: u8) -> PrincipalId {
    ok(PrincipalId::new([byte; 32]))
}

fn program(byte: u8) -> ProgramId {
    ok(ProgramId::new([byte; 32]))
}

const SNAPSHOT_BOUND: usize = 1_048_576;
static NEXT_FILE: AtomicU64 = AtomicU64::new(0);

struct DurableState(PathBuf);

impl DurableState {
    fn stop(storage: Storage) -> Self {
        let path = std::env::temp_dir().join(format!(
            "cosmwasm-repair-22-33-235-{}-{}",
            std::process::id(),
            NEXT_FILE.fetch_add(1, Ordering::Relaxed),
        ));
        let bytes = ok(storage.replay_state_bytes(SNAPSHOT_BOUND));
        let file = Self(path);
        ok(fs::write(&file.0, bytes));
        drop(storage);
        file
    }

    fn reopen(&self) -> Storage {
        ok(Storage::decode_untrusted_replay_state(
            &ok(fs::read(&self.0)),
            SNAPSHOT_BOUND,
        ))
    }
}

impl Drop for DurableState {
    fn drop(&mut self) {
        ok(fs::remove_file(&self.0));
    }
}

fn read(storage: &mut Storage, namespace: StorageNamespace, key: &[u8]) -> Option<Vec<u8>> {
    ok(storage.transaction(namespace).read(key))
}

fn write(storage: &mut Storage, namespace: StorageNamespace, key: &[u8], value: &[u8]) {
    let mut transaction = storage.transaction(namespace);
    ok(transaction.write(key, value));
    let _ = transaction.commit();
}

fn text_schema() -> RecordSchema {
    ok(RecordSchema::new(
        "ExportedRecord",
        vec![
            FieldSchema {
                name: "owner".to_owned(),
                kind: ValueType::Text,
            },
            FieldSchema {
                name: "count".to_owned(),
                kind: ValueType::U64,
            },
        ],
    ))
}

#[test]
fn json_surrogate_pairs_transcode_export_and_reopen() {
    let schema = text_schema();
    for (escaped, text) in [
        (r"\ud83d\ude80", "🚀"),
        (r"\uD800\uDC00", "\u{10000}"),
        (r"\udbff\udfff", "\u{10ffff}"),
        (r"A\ud83D\ude80\u00e9", "A🚀é"),
    ] {
        let values = vec![FieldValue::Text(text.to_owned()), FieldValue::U64(7)];
        let exported = ok(schema.encode_json(&values));
        let escaped = format!(r#"{{"owner":"{escaped}","count":7}}"#);
        let canonical = ok(schema.transcode(&exported));
        assert_eq!(ok(schema.transcode(&escaped)), canonical);
        let mut storage = Storage::new();
        let namespace = StorageNamespace::shared(program(9));
        write(&mut storage, namespace, b"export", &canonical);
        let stopped = DurableState::stop(storage);
        let mut reopened = stopped.reopen();
        let stored = read(&mut reopened, namespace, b"export")
            .unwrap_or_else(|| panic!("lost exported record"));
        assert_eq!(ok(schema.decode(&stored)), values);
        assert_eq!(
            ok(schema.encode_json(&ok(schema.decode(&stored)))),
            exported
        );
    }
}

#[test]
fn json_surrogate_pairs_refuse_invalid_and_enforce_bounds() {
    let schema = text_schema();
    for invalid in [
        r"\ud800",
        r"\udfff",
        r"\udc00\ud800",
        r"\ud800\ud800",
        r"\ud800\u0041",
        r"\ud800x\udc00",
        r"\ud800\uDC0",
        r"\ud800\uZZZZ",
        r"\ud800\n",
        r"\ud800\ uDC00",
    ] {
        let document = format!(r#"{{"owner":"{invalid}","count":7}}"#);
        assert_eq!(
            schema.transcode(&document),
            Err(PortRefusal::InvalidJson),
            "{document}"
        );
    }
    let at_bound = "🚀".repeat(MAX_TEXT_BYTES / 4);
    let values = vec![FieldValue::Text(at_bound.clone()), FieldValue::U64(7)];
    let escaped = format!(
        r#"{{"owner":"{}","count":7}}"#,
        r"\ud83d\ude80".repeat(MAX_TEXT_BYTES / 4)
    );
    assert_eq!(ok(schema.transcode(&escaped)), ok(schema.encode(&values)));
    assert_eq!(
        ok(schema.transcode(&ok(schema.encode_json(&values)))),
        ok(schema.encode(&values))
    );
    for oversized in [
        format!(r#"{{"owner":"{at_bound}a","count":7}}"#),
        format!(
            r#"{{"owner":"{}a","count":7}}"#,
            r"\ud83d\ude80".repeat(MAX_TEXT_BYTES / 4)
        ),
        format!(r#"{{"owner":"{}","count":7}}"#, " ".repeat(MAX_JSON_BYTES)),
    ] {
        assert_eq!(schema.transcode(&oversized), Err(PortRefusal::InvalidJson));
    }
    assert_eq!(
        schema.encode(&[FieldValue::Text(format!("{at_bound}a")), FieldValue::U64(7)]),
        Err(PortRefusal::SchemaMismatch)
    );
}

fn migration_cell(holder: PrincipalId) -> MigrationCell {
    MigrationCell {
        cosmwasm_key: ok(programs_porting_cosmwasm::map_key(
            "donations",
            &holder.bytes(),
        )),
        layerx_key: ok(DonationPort::storage_key()),
        principal: holder.bytes(),
    }
}

#[test]
fn cosmwasm_partial_import_final_cell_is_atomic_across_reopen() {
    let owner = program(10);
    let first = actor(1);
    let last = actor(2);
    let first_cell = migration_cell(first);
    let last_cell = migration_cell(last);
    let valid_plan = vec![
        (first_cell.clone(), r#"{"count":5}"#.to_owned()),
        (last_cell.clone(), r#"{"count":8}"#.to_owned()),
    ];
    for invalid in 0..4 {
        let mut storage = Storage::new();
        write(
            &mut storage,
            StorageNamespace::principal(owner, first),
            &first_cell.layerx_key,
            &9_u64.to_le_bytes(),
        );
        write(
            &mut storage,
            StorageNamespace::shared(owner),
            b"unrelated",
            b"retain",
        );
        let before = storage.clone();
        let mut plan = valid_plan.clone();
        match invalid {
            0 => plan[1].1 = r#"{"count":"bad"}"#.to_owned(),
            1 => plan[1].0.principal = [0; 32],
            2 => {
                plan[1].0.layerx_key =
                    vec![1; layerx_programs_runtime::storage::MAX_STORAGE_KEY_BYTES + 1];
            }
            3 => plan[1].0.layerx_key.clear(),
            _ => unreachable!(),
        }
        assert!(programs_porting_cosmwasm::import_state(&mut storage, owner, &plan).is_err());
        assert_eq!(
            storage, before,
            "invalid final cell {invalid} leaked a prefix"
        );
        let stopped = DurableState::stop(storage);
        let mut reopened = stopped.reopen();
        assert_eq!(reopened, before);
        assert_eq!(
            ok(programs_porting_cosmwasm::import_state(
                &mut reopened,
                owner,
                &valid_plan
            )),
            2
        );
        assert_eq!(
            ok(programs_porting_cosmwasm::import_state(
                &mut reopened,
                owner,
                &valid_plan
            )),
            0
        );
        let stopped_again = DurableState::stop(reopened);
        let mut reopened = stopped_again.reopen();
        let port = ok(DonationPort::new(DonationTerms {
            asset: [3; 32],
            beneficiary: [4; 32],
            price: 1,
            cap: 100,
        }));
        let module = ok(validated_module(&port));
        for (principal, expected) in [(first, 5), (last, 8)] {
            let invocation = Invocation {
                module: &module,
                program: owner,
                principal,
                receipts: &AbsentReceipts,
            };
            let record = ok(execute_donations(&invocation, &mut reopened));
            assert_eq!(record.execution.outputs, [WasmValue::I32(expected)]);
        }
        assert_eq!(
            read(&mut reopened, StorageNamespace::shared(owner), b"unrelated"),
            Some(b"retain".to_vec())
        );
    }
}

fn terms(quote_lots: u64) -> OrderBookTerms {
    OrderBookTerms {
        maker: actor(1),
        taker: actor(2),
        base_lots: 10,
        quote_lots,
        price: 7,
    }
}

fn book_module(book: &BoundedOrderBook) -> ValidatedModule {
    ok(ok(WasmEngine::declared()).validate_v2(&ok(book.code())))
}

fn invoke(
    module: &ValidatedModule,
    storage: &mut Storage,
    principals: (ProgramId, PrincipalId),
    entrypoint: &str,
    calldata: &[u8],
    capabilities: CapabilitySet,
    response_capacity: usize,
) -> V2AuthorizedExecutionRecord {
    ok(Executor::declared().execute_authorized_v2(
        storage,
        AuthorizedExecutionRequest {
            module,
            program: principals.0,
            authorization: AuthorizationContext::new(principals.1, capabilities),
            receipts: &AbsentReceipts,
            entrypoint,
            calldata,
            composition: CompositionContext::isolated(),
            response_capacity,
        },
    ))
}

fn response(record: &V2AuthorizedExecutionRecord) -> &[u8] {
    assert!(
        matches!(record.outcome(), V2ActivityOutcome::Success { .. }),
        "{:?}",
        record.outcome()
    );
    let response = record
        .response()
        .unwrap_or_else(|| panic!("missing success response"));
    assert_eq!(response.code, 0);
    &response.bytes
}

fn balance(storage: &mut Storage, owner: ProgramId, trader: PrincipalId) -> (u64, u64) {
    let bytes = read(
        storage,
        StorageNamespace::shared(owner),
        &ok(BoundedOrderBook::settlement_key(trader)),
    )
    .unwrap_or_else(|| panic!("missing settlement balance"));
    assert_eq!(bytes.len(), 16);
    (
        u64::from_le_bytes(ok(bytes[0..8].try_into())),
        u64::from_le_bytes(ok(bytes[8..16].try_into())),
    )
}

fn position(storage: &mut Storage, owner: ProgramId, trader: PrincipalId) -> (u64, u64) {
    let bytes = read(
        storage,
        StorageNamespace::principal(owner, trader),
        &ok(OrderBook::position_key()),
    )
    .unwrap_or_else(|| panic!("missing principal position"));
    assert_eq!(bytes.len(), 16);
    (
        u64::from_le_bytes(ok(bytes[0..8].try_into())),
        u64::from_le_bytes(ok(bytes[8..16].try_into())),
    )
}

fn query(
    book: &BoundedOrderBook,
    module: &ValidatedModule,
    storage: &mut Storage,
    owner: ProgramId,
    trader: PrincipalId,
) -> u64 {
    let record = invoke(
        module,
        storage,
        (owner, trader),
        READ_ORDER,
        &[],
        ok(CapabilitySet::new([Capability::SharedStorageRead])),
        33,
    );
    ok(book.decode_order(response(&record))).quantity
}

#[test]
fn orderbook_two_traders_settle_and_reopen() {
    let terms = terms(100);
    let book = ok(BoundedOrderBook::new(terms));
    let owner = program(20);
    let deployment = ok(book.deployment(owner));
    assert_eq!(
        deployment.abi_version,
        layerx_programs_runtime::ABI_V2_VERSION
    );
    let mut lifecycle = ok(Lifecycle::declared());
    let _ = ok(lifecycle.deploy(deployment));
    let module = book_module(&book);
    let mut storage = Storage::new();
    ok(book.initialize(&mut storage, owner));
    let record = invoke(
        &module,
        &mut storage,
        (owner, terms.maker),
        PLACE_ORDER,
        &ok(book.quantity_message(10)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    assert_eq!(ok(book.decode_order(response(&record))).quantity, 10);
    assert_eq!(query(&book, &module, &mut storage, owner, terms.taker), 10);
    let record = invoke(
        &module,
        &mut storage,
        (owner, terms.taker),
        MATCH_ORDER,
        &ok(book.quantity_message(3)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    assert_eq!(ok(book.decode_order(response(&record))).quantity, 7);
    assert_eq!(balance(&mut storage, owner, terms.maker), (0, 21));
    assert_eq!(balance(&mut storage, owner, terms.taker), (3, 79));
    assert_eq!(position(&mut storage, owner, terms.maker), (0, 10));
    assert_eq!(position(&mut storage, owner, terms.taker), (3, 0));
    assert!(record.execution().usage().storage_write_bytes > 0);
    assert!(record
        .effects()
        .unwrap_or_else(|| panic!("missing effects"))
        .transfers
        .is_empty());
    let stopped = DurableState::stop(storage);
    drop(module);
    drop(lifecycle);
    let fresh_module = book_module(&book);
    let mut reopened = stopped.reopen();
    assert_eq!(
        query(&book, &fresh_module, &mut reopened, owner, actor(3)),
        7
    );
    assert_eq!(balance(&mut reopened, owner, terms.maker), (0, 21));
    assert_eq!(balance(&mut reopened, owner, terms.taker), (3, 79));
    let record = invoke(
        &fresh_module,
        &mut reopened,
        (owner, terms.taker),
        MATCH_ORDER,
        &ok(book.quantity_message(7)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    assert_eq!(ok(book.decode_order(response(&record))).quantity, 0);
    assert_eq!(balance(&mut reopened, owner, terms.maker), (0, 70));
    assert_eq!(balance(&mut reopened, owner, terms.taker), (10, 30));
    assert_eq!(position(&mut reopened, owner, terms.taker), (10, 0));
    let stopped = DurableState::stop(reopened);
    let mut reopened = stopped.reopen();
    assert_eq!(
        query(&book, &fresh_module, &mut reopened, owner, terms.maker),
        0
    );
    assert_eq!(
        balance(&mut reopened, owner, terms.maker).1 + balance(&mut reopened, owner, terms.taker).1,
        100
    );
}

fn assert_refused(record: &V2AuthorizedExecutionRecord) {
    assert!(
        matches!(record.outcome(), V2ActivityOutcome::Failure(_)),
        "{:?}",
        record.outcome()
    );
    assert!(record.effects().is_none());
    assert!(record.response().is_none());
}

fn refusal_cases(
    book: &BoundedOrderBook,
    terms: OrderBookTerms,
) -> [(PrincipalId, &'static str, Vec<u8>, CapabilitySet, usize); 9] {
    [
        (
            terms.maker,
            MATCH_ORDER,
            ok(book.quantity_message(1)),
            ok(BoundedOrderBook::capabilities()),
            33,
        ),
        (
            actor(3),
            MATCH_ORDER,
            ok(book.quantity_message(1)),
            ok(BoundedOrderBook::capabilities()),
            33,
        ),
        (
            terms.maker,
            PLACE_ORDER,
            ok(book.quantity_message(1)),
            ok(BoundedOrderBook::capabilities()),
            33,
        ),
        (
            terms.taker,
            MATCH_ORDER,
            0_u64.to_le_bytes().to_vec(),
            ok(BoundedOrderBook::capabilities()),
            33,
        ),
        (
            terms.taker,
            MATCH_ORDER,
            11_u64.to_le_bytes().to_vec(),
            ok(BoundedOrderBook::capabilities()),
            33,
        ),
        (
            terms.taker,
            MATCH_ORDER,
            vec![1; 7],
            ok(BoundedOrderBook::capabilities()),
            33,
        ),
        // Shared writes succeed in the candidate; the final principal write
        // fails. The runtime must discard the entire multi-cell settlement.
        (
            terms.taker,
            MATCH_ORDER,
            ok(book.quantity_message(1)),
            ok(CapabilitySet::new([
                Capability::StorageRead,
                Capability::SharedStorageRead,
                Capability::SharedStorageWrite,
            ])),
            33,
        ),
        // Every write is staged before the response host refuses this capacity.
        (
            terms.taker,
            MATCH_ORDER,
            ok(book.quantity_message(1)),
            ok(BoundedOrderBook::capabilities()),
            32,
        ),
        (
            terms.taker,
            MATCH_ORDER,
            ok(book.quantity_message(1)),
            ok(CapabilitySet::new([
                Capability::StorageRead,
                Capability::StorageWrite,
                Capability::SharedStorageRead,
            ])),
            33,
        ),
    ]
}

#[test]
fn orderbook_refusals_preserve_all_settlement_cells() {
    let terms = terms(100);
    let book = ok(BoundedOrderBook::new(terms));
    let owner = program(21);
    let module = book_module(&book);
    let mut storage = Storage::new();
    ok(book.initialize(&mut storage, owner));
    // Reinitialization refuses without erasing allocations or registrations.
    let initialized = storage.clone();
    assert_eq!(
        book.initialize(&mut storage, owner),
        Err(PortRefusal::OutOfRange)
    );
    assert_eq!(storage, initialized);
    let record = invoke(
        &module,
        &mut storage,
        (owner, terms.maker),
        PLACE_ORDER,
        &ok(book.quantity_message(10)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    response(&record);
    let before = storage.clone();
    for (trader, entrypoint, data, grants, capacity) in refusal_cases(&book, terms) {
        let record = invoke(
            &module,
            &mut storage,
            (owner, trader),
            entrypoint,
            &data,
            grants,
            capacity,
        );
        assert_refused(&record);
        assert_eq!(storage, before);
    }
    let stopped = DurableState::stop(storage);
    let mut reopened = stopped.reopen();
    assert_eq!(reopened, before);
    let record = invoke(
        &module,
        &mut reopened,
        (owner, terms.taker),
        MATCH_ORDER,
        &ok(book.quantity_message(2)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    assert_eq!(ok(book.decode_order(response(&record))).quantity, 8);
    // An initialized final principal destination makes bootstrap fail atomically.
    let mut storage = Storage::new();
    write(
        &mut storage,
        StorageNamespace::principal(owner, terms.taker),
        &ok(OrderBook::position_key()),
        &[4; 16],
    );
    let before = storage.clone();
    assert_eq!(
        book.initialize(&mut storage, owner),
        Err(PortRefusal::OutOfRange)
    );
    assert_eq!(storage, before);
}

fn invalid_orderbook_terms(initial: OrderBookTerms) -> [OrderBookTerms; 6] {
    [
        OrderBookTerms {
            maker: initial.taker,
            ..initial
        },
        OrderBookTerms {
            base_lots: 0,
            ..initial
        },
        OrderBookTerms {
            base_lots: MAX_ORDER_LOTS + 1,
            ..initial
        },
        OrderBookTerms {
            price: u64::MAX,
            ..initial
        },
        OrderBookTerms {
            price: 0,
            ..initial
        },
        OrderBookTerms {
            quote_lots: u64::MAX,
            ..initial
        },
    ]
}

#[test]
fn orderbook_bounds_and_corrupt_state_refuse() {
    let initial = terms(20);
    for invalid in invalid_orderbook_terms(initial) {
        assert_eq!(BoundedOrderBook::new(invalid), Err(PortRefusal::OutOfRange));
    }
    let book = ok(BoundedOrderBook::new(initial));
    assert_eq!(book.quantity_message(0), Err(PortRefusal::OutOfRange));
    assert_eq!(book.quantity_message(11), Err(PortRefusal::OutOfRange));
    let module = book_module(&book);
    let owner = program(22);
    let mut storage = Storage::new();
    ok(book.initialize(&mut storage, owner));
    let record = invoke(
        &module,
        &mut storage,
        (owner, initial.maker),
        PLACE_ORDER,
        &ok(book.quantity_message(10)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    response(&record);
    let before = storage.clone();
    let record = invoke(
        &module,
        &mut storage,
        (owner, initial.taker),
        MATCH_ORDER,
        &ok(book.quantity_message(3)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    assert_refused(&record); // 21 quote due exceeds the conserved balance of 20.
    assert_eq!(storage, before);
    let order_key = ok(OrderBook::order_key(1));
    let original = read(&mut storage, StorageNamespace::shared(owner), &order_key)
        .unwrap_or_else(|| panic!("missing order"));
    for corruption in 0..4 {
        let mut bytes = original.clone();
        match corruption {
            0 => {
                bytes.pop();
            }
            1 => bytes[17] = 1, // unsupported high price limb
            2 => bytes[25..33].copy_from_slice(&u64::MAX.to_le_bytes()),
            3 => bytes[8] = 1, // this bounded market accepts only sell orders
            _ => unreachable!(),
        }
        let mut corrupted = before.clone();
        write(
            &mut corrupted,
            StorageNamespace::shared(owner),
            &order_key,
            &bytes,
        );
        let snapshot = corrupted.clone();
        let record = invoke(
            &module,
            &mut corrupted,
            (owner, initial.taker),
            MATCH_ORDER,
            &ok(book.quantity_message(1)),
            ok(BoundedOrderBook::capabilities()),
            33,
        );
        assert_refused(&record);
        assert_eq!(corrupted, snapshot);
        assert_eq!(book.decode_order(&bytes), Err(PortRefusal::SchemaMismatch));
    }
    // In-bound values that violate conservation must also refuse unchanged.
    let key = ok(BoundedOrderBook::settlement_key(initial.taker));
    write(
        &mut storage,
        StorageNamespace::shared(owner),
        &key,
        &[1_u64.to_le_bytes(), 20_u64.to_le_bytes()].concat(),
    );
    let corrupted = storage.clone();
    let record = invoke(
        &module,
        &mut storage,
        (owner, initial.taker),
        MATCH_ORDER,
        &ok(book.quantity_message(1)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    assert_refused(&record);
    assert_eq!(storage, corrupted);
}

#[test]
fn storage_shared_docs_global_item_and_map_use_scoped_hosts() {
    let terms = terms(100);
    let book = ok(BoundedOrderBook::new(terms));
    let module = book_module(&book);
    let owner = program(23);
    let other = program(24);
    let mut storage = Storage::new();
    ok(book.initialize(&mut storage, owner));
    ok(book.initialize(&mut storage, other));
    let global_item = ok(StateBinding::Shared.layerx_key("order_count", &[]));
    let shared_map = ok(programs_porting_cosmwasm::map_key(
        "orders",
        &1_u64.to_le_bytes(),
    ));
    assert_eq!(global_item, ok(OrderBook::order_count_key()));
    assert_eq!(shared_map, ok(OrderBook::order_key(1)));
    assert_eq!(
        ok(StateBinding::Item.layerx_key("order_count", &[])),
        global_item
    );
    for trader in [terms.maker, terms.taker] {
        // The same Item key can exist independently in principal scope.
        write(
            &mut storage,
            StorageNamespace::principal(owner, trader),
            &global_item,
            &99_u64.to_le_bytes(),
        );
    }
    let record = invoke(
        &module,
        &mut storage,
        (owner, terms.maker),
        PLACE_ORDER,
        &ok(book.quantity_message(5)),
        ok(BoundedOrderBook::capabilities()),
        33,
    );
    response(&record);
    for trader in [terms.maker, terms.taker] {
        assert_eq!(query(&book, &module, &mut storage, owner, trader), 5);
        assert_eq!(
            read(
                &mut storage,
                StorageNamespace::principal(owner, trader),
                &global_item
            ),
            Some(99_u64.to_le_bytes().to_vec())
        );
        assert_eq!(
            read(
                &mut storage,
                StorageNamespace::principal(owner, trader),
                &shared_map
            ),
            None
        );
    }
    assert_eq!(
        read(&mut storage, StorageNamespace::shared(owner), &global_item),
        Some(1_u64.to_le_bytes().to_vec())
    );
    assert_eq!(query(&book, &module, &mut storage, other, terms.taker), 0);
    assert_eq!(
        read(&mut storage, StorageNamespace::shared(other), &global_item),
        Some(0_u64.to_le_bytes().to_vec())
    );
    let before = storage.clone();
    let record = invoke(
        &module,
        &mut storage,
        (owner, terms.taker),
        READ_ORDER,
        &[],
        ok(CapabilitySet::new([Capability::StorageRead])),
        33,
    );
    assert_refused(&record); // principal authority never implies shared authority
    assert_eq!(storage, before);
}
