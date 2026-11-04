//! Task 22.33.236 exercises the port's producer codecs, real scoped WASM host,
//! transactional storage and canonical storage reopen path. Transfer assertions
//! concern the real kernel-bound requests; no substitute balance ledger is used.

use layerx_programs_runtime::storage::{MAX_STORAGE_KEY_BYTES, MAX_STORAGE_VALUE_BYTES};
use layerx_programs_runtime::{
    AbiEffects, AbiError, AuthorizationContext, AuthorizedExecutionRequest, Capability,
    CapabilitySet, CompositionContext, CompositionRefusal, ExecutionError, Executor, PrincipalId,
    ProgramId, Storage, StorageError, StorageNamespace, TransferRequest, TransferSource,
    V2ActivityOutcome, V2AuthorizedExecutionRecord, ValidatedModule, CALL_ENTRY_EXPORT,
};
use programs_porting_solana::shared_pool::{
    ParticipantDeposit, PoolReserve, PoolView, SharedPoolPort, POOL_QUERY_BYTES,
};
use programs_porting_solana::{
    import_accounts, per_signer_import, AbsentReceipts, AccountHolder, FieldValue, MigrationCell,
    PortRefusal, Pubkey, SeedPath,
};

const SNAPSHOT_BOUND: usize = 8 * 1024 * 1024;
const AUTHORITY: [u8; 32] = [3; 32];
const ASSET: [u8; 32] = [4; 32];
const DESTINATION: [u8; 32] = [5; 32];

fn program(byte: u8) -> ProgramId {
    ProgramId::new([byte; 32]).unwrap_or_else(|error| panic!("program: {error}"))
}

fn principal(byte: u8) -> PrincipalId {
    PrincipalId::new([byte; 32]).unwrap_or_else(|error| panic!("principal: {error}"))
}

fn pool(maximum: u64) -> SharedPoolPort {
    SharedPoolPort::new(AUTHORITY, ASSET, DESTINATION, maximum)
        .unwrap_or_else(|error| panic!("pool: {error}"))
}

fn reopen(storage: Storage) -> Storage {
    let bytes = storage
        .replay_state_bytes(SNAPSHOT_BOUND)
        .unwrap_or_else(|error| panic!("persist storage: {error:?}"));
    drop(storage);
    Storage::decode_untrusted_replay_state(&bytes, SNAPSHOT_BOUND)
        .unwrap_or_else(|error| panic!("reopen storage: {error:?}"))
}

fn read(storage: &mut Storage, namespace: StorageNamespace, key: &[u8]) -> Option<Vec<u8>> {
    storage
        .transaction(namespace)
        .read(key)
        .unwrap_or_else(|error| panic!("stored account: {error}"))
}

fn write(storage: &mut Storage, namespace: StorageNamespace, key: &[u8], value: &[u8]) {
    let mut transaction = storage.transaction(namespace);
    transaction
        .write(key, value)
        .unwrap_or_else(|error| panic!("seed account: {error}"));
    assert_eq!(transaction.commit(), 1);
}

fn migration_plan() -> Vec<(MigrationCell, Vec<u8>)> {
    let source = Pubkey::new([10; 32]).unwrap_or_else(|error| panic!("source: {error}"));
    let seeds = SeedPath::new(vec![
        b"deposit".to_vec(),
        b"participant".to_vec(),
        vec![0; 32],
    ])
    .unwrap_or_else(|error| panic!("seeds: {error}"));
    let holders: Vec<_> = [11, 12]
        .into_iter()
        .map(|byte| {
            let signer = Pubkey::new([byte; 32]).unwrap_or_else(|error| panic!("signer: {error}"));
            let path = seeds
                .with_seed(2, &signer.bytes())
                .unwrap_or_else(|error| panic!("holder seeds: {error}"));
            AccountHolder {
                signer,
                bump: 255,
                address: path.address(255, source),
                principal: [byte; 32],
            }
        })
        .collect();
    per_signer_import(&seeds, 2, source, &[2], &holders)
        .unwrap_or_else(|error| panic!("migration producer: {error}"))
        .into_iter()
        .enumerate()
        .map(|(index, cell)| {
            let data = ParticipantDeposit::schema()
                .and_then(|schema| {
                    schema.encode(&[FieldValue::U64(index as u64 + 1), FieldValue::U32(1)])
                })
                .unwrap_or_else(|error| panic!("snapshot account producer: {error}"));
            (cell, data)
        })
        .collect()
}

#[test]
fn solana_partial_import_rejects_invalid_final_cell_after_reopen() {
    let owner = program(1);
    let plan = migration_plan();
    let mut original = Storage::new();
    assert_eq!(import_accounts(&mut original, owner, &plan), Ok(2));
    let key = &plan[0].0.layerx_key;
    write(
        &mut original,
        StorageNamespace::shared(owner),
        key,
        b"shared sentinel",
    );
    write(
        &mut original,
        StorageNamespace::principal(program(2), principal(11)),
        key,
        b"neighbor sentinel",
    );
    for (invalid, expected) in [
        (0, StorageError::InvalidPrincipal),
        (1, StorageError::ValueTooLarge),
        (2, StorageError::EmptyKey),
        (3, StorageError::KeyTooLarge),
    ] {
        let mut storage = original.clone();
        let mut retry = plan.clone();
        retry[0].1 = ParticipantDeposit::schema()
            .and_then(|schema| schema.encode(&[FieldValue::U64(10), FieldValue::U32(2)]))
            .unwrap_or_else(|error| panic!("replacement account: {error}"));
        retry[1].1 = ParticipantDeposit::schema()
            .and_then(|schema| schema.encode(&[FieldValue::U64(20), FieldValue::U32(2)]))
            .unwrap_or_else(|error| panic!("replacement account: {error}"));
        let mut invalid_plan = retry.clone();
        match invalid {
            0 => invalid_plan[1].0.principal = [0; 32],
            1 => invalid_plan[1].1 = vec![1; MAX_STORAGE_VALUE_BYTES + 1],
            2 => invalid_plan[1].0.layerx_key.clear(),
            3 => invalid_plan[1].0.layerx_key = vec![1; MAX_STORAGE_KEY_BYTES + 1],
            _ => unreachable!("bounded cases"),
        }
        assert_eq!(
            import_accounts(&mut storage, owner, &invalid_plan),
            Err(PortRefusal::Storage(expected))
        );
        assert_eq!(
            storage, original,
            "invalid final cell must not commit a prefix"
        );
        storage = reopen(storage);
        assert_eq!(
            storage, original,
            "refused import survives stop and reopen unchanged"
        );
        for (cell, bytes) in &plan {
            assert_eq!(
                read(
                    &mut storage,
                    StorageNamespace::principal(
                        owner,
                        PrincipalId::new(cell.principal)
                            .unwrap_or_else(|error| panic!("principal: {error}"))
                    ),
                    &cell.layerx_key
                ),
                Some(bytes.clone())
            );
        }
        assert_eq!(import_accounts(&mut storage, owner, &retry), Ok(2));
        storage = reopen(storage);
        assert_eq!(
            import_accounts(&mut storage, owner, &retry),
            Ok(0),
            "retry is idempotent"
        );
        assert_eq!(import_accounts(&mut storage, owner, &[]), Ok(0));
        for (cell, bytes) in &retry {
            assert_eq!(
                read(
                    &mut storage,
                    StorageNamespace::principal(
                        owner,
                        PrincipalId::new(cell.principal)
                            .unwrap_or_else(|error| panic!("principal: {error}"))
                    ),
                    &cell.layerx_key
                ),
                Some(bytes.clone())
            );
        }
        assert_eq!(
            read(&mut storage, StorageNamespace::shared(owner), key),
            Some(b"shared sentinel".to_vec())
        );
        assert_eq!(
            read(
                &mut storage,
                StorageNamespace::principal(program(2), principal(11)),
                key
            ),
            Some(b"neighbor sentinel".to_vec())
        );
    }
}

fn execute(
    port: &SharedPoolPort,
    module: &ValidatedModule,
    storage: &mut Storage,
    owner: ProgramId,
    actor: PrincipalId,
    capabilities: CapabilitySet,
    calldata: &[u8],
    response_capacity: usize,
) -> Result<V2AuthorizedExecutionRecord, ExecutionError> {
    port.execute(
        storage,
        AuthorizedExecutionRequest {
            module,
            program: owner,
            authorization: AuthorizationContext::new(actor, capabilities),
            receipts: &AbsentReceipts,
            entrypoint: CALL_ENTRY_EXPORT,
            calldata,
            composition: CompositionContext::isolated(),
            response_capacity,
        },
    )
}

fn deposit_data(amount: u64) -> Vec<u8> {
    SharedPoolPort::deposit_instruction()
        .and_then(|codec| codec.data(&[FieldValue::U64(amount)]))
        .unwrap_or_else(|error| panic!("deposit producer: {error}"))
}

fn query_data() -> Vec<u8> {
    SharedPoolPort::query_instruction()
        .and_then(|codec| codec.data(&[]))
        .unwrap_or_else(|error| panic!("query producer: {error}"))
}

fn deposit(
    module: &ValidatedModule,
    port: &SharedPoolPort,
    storage: &mut Storage,
    actor: PrincipalId,
    amount: u64,
) {
    let record = execute(
        port,
        module,
        storage,
        program(1),
        actor,
        port.deposit_capabilities(amount)
            .unwrap_or_else(|error| panic!("grants: {error}")),
        &deposit_data(amount),
        0,
    )
    .unwrap_or_else(|error| panic!("deposit execution: {error}"));
    assert!(record.execution().usage().cpu_fuel > 0);
    let V2ActivityOutcome::Success { response, effects } = record.outcome() else {
        panic!("deposit must execute successfully: {:?}", record.outcome());
    };
    assert_eq!(response.code, 0);
    assert!(response.bytes.is_empty());
    assert_eq!(
        effects,
        &AbiEffects {
            transfers: vec![TransferRequest {
                program: program(1),
                principal: actor,
                frame: layerx_programs_runtime::CallFrameId::root(),
                source: TransferSource::Principal(actor),
                asset: ASSET,
                to: DESTINATION,
                amount: u128::from(amount),
            }],
            ..AbiEffects::default()
        }
    );
}

fn query(
    port: &SharedPoolPort,
    module: &ValidatedModule,
    storage: &mut Storage,
    owner: ProgramId,
    actor: PrincipalId,
) -> PoolView {
    let before = storage.clone();
    let record = execute(
        port,
        module,
        storage,
        owner,
        actor,
        SharedPoolPort::query_capabilities()
            .unwrap_or_else(|error| panic!("query grants: {error}")),
        &query_data(),
        POOL_QUERY_BYTES,
    )
    .unwrap_or_else(|error| panic!("query execution: {error}"));
    assert_eq!(*storage, before, "query has no persistent writes");
    let V2ActivityOutcome::Success { response, effects } = record.outcome() else {
        panic!("query must execute successfully: {:?}", record.outcome());
    };
    assert_eq!(response.code, 0);
    assert_eq!(effects, &AbiEffects::default());
    assert_eq!(response.bytes.len(), POOL_QUERY_BYTES);
    PoolView::decode(&response.bytes).unwrap_or_else(|error| panic!("query consumer: {error}"))
}

#[test]
fn pool_execution_proof_two_participants_reopen_and_isolation() {
    let port = pool(1000);
    let module = port
        .validated_module()
        .unwrap_or_else(|error| panic!("pool module: {error}"));
    let mut storage = Storage::new();
    let mut expected = PoolView {
        total_deposited: 0,
        participant_count: 0,
        pool_authority: AUTHORITY,
        deposit_amount: 0,
        deposit_count: 0,
    };
    assert_eq!(
        query(&port, &module, &mut storage, program(1), principal(11)),
        expected
    );
    assert_eq!(
        storage,
        Storage::new(),
        "initial read must not initialize storage"
    );
    deposit(&module, &port, &mut storage, principal(11), 40);
    deposit(&module, &port, &mut storage, principal(12), 70);
    expected.total_deposited = 110;
    expected.participant_count = 2;
    expected.deposit_amount = 40;
    expected.deposit_count = 1;
    assert_eq!(
        query(&port, &module, &mut storage, program(1), principal(11)),
        expected
    );
    expected.deposit_amount = 70;
    assert_eq!(
        query(&port, &module, &mut storage, program(1), principal(12)),
        expected
    );
    expected.deposit_amount = 0;
    expected.deposit_count = 0;
    assert_eq!(
        query(&port, &module, &mut storage, program(1), principal(13)),
        expected
    );
    assert_eq!(
        storage.namespace_cell_count(StorageNamespace::shared(program(1))),
        1
    );
    for actor in [11, 12] {
        assert_eq!(
            storage.namespace_cell_count(StorageNamespace::principal(program(1), principal(actor))),
            1
        );
        assert!(read(
            &mut storage,
            StorageNamespace::principal(program(1), principal(actor)),
            &PoolReserve::storage_key().unwrap_or_else(|error| panic!("reserve key: {error}"))
        )
        .is_none());
    }
    assert!(read(
        &mut storage,
        StorageNamespace::shared(program(1)),
        &ParticipantDeposit::storage_key().unwrap_or_else(|error| panic!("deposit key: {error}"))
    )
    .is_none());
    storage = reopen(storage);
    drop(module);
    drop(port);
    let reopened_pool = pool(1000);
    let reopened_module = reopened_pool
        .validated_module()
        .unwrap_or_else(|error| panic!("reopened module: {error}"));
    expected.deposit_amount = 40;
    expected.deposit_count = 1;
    assert_eq!(
        query(
            &reopened_pool,
            &reopened_module,
            &mut storage,
            program(1),
            principal(11)
        ),
        expected
    );
    expected.deposit_amount = 70;
    assert_eq!(
        query(
            &reopened_pool,
            &reopened_module,
            &mut storage,
            program(1),
            principal(12)
        ),
        expected
    );
    deposit(
        &reopened_module,
        &reopened_pool,
        &mut storage,
        principal(11),
        5,
    );
    storage = reopen(storage);
    expected.total_deposited = 115;
    expected.deposit_amount = 45;
    expected.deposit_count = 2;
    assert_eq!(
        query(
            &reopened_pool,
            &reopened_module,
            &mut storage,
            program(1),
            principal(11)
        ),
        expected
    );
    expected.deposit_amount = 70;
    expected.deposit_count = 1;
    assert_eq!(
        query(
            &reopened_pool,
            &reopened_module,
            &mut storage,
            program(1),
            principal(12)
        ),
        expected
    );
    assert_eq!(
        query(
            &reopened_pool,
            &reopened_module,
            &mut storage,
            program(2),
            principal(11)
        ),
        PoolView {
            total_deposited: 0,
            participant_count: 0,
            pool_authority: AUTHORITY,
            deposit_amount: 0,
            deposit_count: 0
        }
    );
}

#[test]
fn pool_execution_proof_denied_transfer_and_scope_roll_back_both_cells() {
    let port = pool(1000);
    let module = port
        .validated_module()
        .unwrap_or_else(|error| panic!("module: {error}"));
    let mut storage = Storage::new();
    deposit(&module, &port, &mut storage, principal(11), 40);
    let original = storage.clone();
    for missing in 0..7 {
        let mut grants = vec![
            Capability::StorageRead,
            Capability::StorageWrite,
            Capability::SharedStorageRead,
            Capability::SharedStorageWrite,
        ];
        let expected_error = match missing {
            0 => AbiError::CapabilityDenied,
            1 => {
                grants.push(Capability::Transfer402 {
                    asset: ASSET,
                    to: DESTINATION,
                    maximum_amount: 9,
                });
                AbiError::CapabilityEscalation
            }
            2 => {
                grants.push(Capability::Transfer402 {
                    asset: [6; 32],
                    to: DESTINATION,
                    maximum_amount: 10,
                });
                AbiError::CapabilityDenied
            }
            3 => {
                grants.push(Capability::Transfer402 {
                    asset: ASSET,
                    to: [6; 32],
                    maximum_amount: 10,
                });
                AbiError::CapabilityDenied
            }
            4..=6 => {
                grants.remove(match missing {
                    4 => 3,
                    5 => 1,
                    _ => 2,
                });
                grants.push(Capability::Transfer402 {
                    asset: ASSET,
                    to: DESTINATION,
                    maximum_amount: 10,
                });
                AbiError::CapabilityDenied
            }
            _ => unreachable!("bounded cases"),
        };
        for actor in [11, 12] {
            let result = execute(
                &port,
                &module,
                &mut storage,
                program(1),
                principal(actor),
                CapabilitySet::new(grants.clone())
                    .unwrap_or_else(|error| panic!("negative grants: {error}")),
                &deposit_data(10),
                0,
            );
            assert_eq!(
                result,
                Err(ExecutionError::Composition(CompositionRefusal::Authority(
                    expected_error.clone()
                )))
            );
            assert_eq!(
                storage, original,
                "refused transfer or scope leaves no writes"
            );
            storage = reopen(storage);
            assert_eq!(
                storage, original,
                "refusal leaves no durable partial deposit"
            );
        }
    }
    deposit(&module, &port, &mut storage, principal(12), 10);
    assert_eq!(
        query(&port, &module, &mut storage, program(1), principal(12)).participant_count,
        2
    );
}

fn assert_guest_failure(result: Result<V2AuthorizedExecutionRecord, ExecutionError>) {
    let record =
        result.unwrap_or_else(|error| panic!("expected receipt-ready guest failure: {error}"));
    let V2ActivityOutcome::Failure(failure) = record.outcome() else {
        panic!(
            "expected guest refusal without success effects: {:?}",
            record.outcome()
        );
    };
    assert_eq!(
        failure.class(),
        layerx_programs_runtime::RefusalClass::RuntimeFault
    );
}

#[test]
fn pool_execution_proof_bounds_and_malformed_calls_are_atomic() {
    assert!(SharedPoolPort::new([0; 32], ASSET, DESTINATION, 1).is_err());
    assert!(SharedPoolPort::new(AUTHORITY, [0; 32], DESTINATION, 1).is_err());
    assert!(SharedPoolPort::new(AUTHORITY, ASSET, [0; 32], 1).is_err());
    assert!(SharedPoolPort::new(AUTHORITY, ASSET, DESTINATION, 0).is_err());
    let port = pool(1000);
    assert_eq!(port.deposit_capabilities(0), Err(PortRefusal::OutOfRange));
    assert_eq!(
        port.deposit_capabilities(1001),
        Err(PortRefusal::OutOfRange)
    );
    let module = port
        .validated_module()
        .unwrap_or_else(|error| panic!("module: {error}"));
    let mut storage = Storage::new();
    deposit(&module, &port, &mut storage, principal(11), 40);
    let original = storage.clone();
    let mut trailing = deposit_data(1);
    trailing.push(0);
    let mut wrong_tag = deposit_data(1);
    wrong_tag[0] ^= 1;
    let mut long_query = query_data();
    long_query.extend_from_slice(&[0; 8]);
    for calldata in [
        deposit_data(0),
        deposit_data(1001),
        deposit_data(1)[..8].to_vec(),
        query_data()[..7].to_vec(),
        trailing,
        wrong_tag,
        long_query,
    ] {
        assert_guest_failure(execute(
            &port,
            &module,
            &mut storage,
            program(1),
            principal(11),
            port.deposit_capabilities(1000)
                .unwrap_or_else(|error| panic!("grants: {error}")),
            &calldata,
            POOL_QUERY_BYTES,
        ));
        assert_eq!(storage, original);
    }
    let query_result = execute(
        &port,
        &module,
        &mut storage,
        program(1),
        principal(11),
        SharedPoolPort::query_capabilities().unwrap_or_else(|error| panic!("grants: {error}")),
        &query_data(),
        POOL_QUERY_BYTES - 1,
    );
    assert!(
        query_result.is_err(),
        "undersized consumer response must refuse"
    );
    assert_eq!(storage, original);
    storage = reopen(storage);
    assert_eq!(
        query(&port, &module, &mut storage, program(1), principal(11)).deposit_amount,
        40
    );
}

#[test]
fn pool_execution_proof_checked_totals_counts_and_account_schema() {
    let port = pool(u64::MAX);
    let module = port
        .validated_module()
        .unwrap_or_else(|error| panic!("module: {error}"));
    let reserve_key = PoolReserve::storage_key().unwrap_or_else(|error| panic!("key: {error}"));
    let participant_key =
        ParticipantDeposit::storage_key().unwrap_or_else(|error| panic!("key: {error}"));
    for case in 0..8 {
        let mut storage = Storage::new();
        let mut reserve = PoolReserve::schema()
            .and_then(|schema| {
                schema.encode(&[
                    FieldValue::U64(if case == 0 { u64::MAX } else { 1 }),
                    FieldValue::U32(if case == 1 { u32::MAX } else { 1 }),
                    FieldValue::Pubkey(if case == 6 { [7; 32] } else { AUTHORITY }),
                ])
            })
            .unwrap_or_else(|error| panic!("reserve producer: {error}"));
        if case == 4 {
            reserve[0] ^= 1;
        }
        if case == 5 {
            reserve.push(0);
        }
        write(
            &mut storage,
            StorageNamespace::shared(program(1)),
            &reserve_key,
            &reserve,
        );
        if case != 1 {
            let mut participant = ParticipantDeposit::schema()
                .and_then(|schema| {
                    schema.encode(&[
                        FieldValue::U64(if case == 2 { u64::MAX } else { 1 }),
                        FieldValue::U32(if case == 3 { u32::MAX } else { 1 }),
                    ])
                })
                .unwrap_or_else(|error| panic!("participant producer: {error}"));
            if case == 7 {
                participant[0] ^= 1;
            }
            write(
                &mut storage,
                StorageNamespace::principal(program(1), principal(11)),
                &participant_key,
                &participant,
            );
        }
        let original = storage.clone();
        assert_guest_failure(execute(
            &port,
            &module,
            &mut storage,
            program(1),
            principal(11),
            port.deposit_capabilities(1)
                .unwrap_or_else(|error| panic!("grants: {error}")),
            &deposit_data(1),
            0,
        ));
        assert_eq!(storage, original, "case {case} must leave no partial state");
        assert_eq!(reopen(storage), original);
    }
    let mut storage = Storage::new();
    deposit(&module, &port, &mut storage, principal(11), u64::MAX);
    storage = reopen(storage);
    let view = query(&port, &module, &mut storage, program(1), principal(11));
    assert_eq!(view.total_deposited, u64::MAX);
    assert_eq!(view.deposit_amount, u64::MAX);
    assert_eq!(view.deposit_count, 1);
    let original = storage.clone();
    assert_guest_failure(execute(
        &port,
        &module,
        &mut storage,
        program(1),
        principal(12),
        port.deposit_capabilities(1)
            .unwrap_or_else(|error| panic!("grants: {error}")),
        &deposit_data(1),
        0,
    ));
    assert_eq!(storage, original);
}

#[test]
fn pool_execution_proof_query_requires_both_scopes_and_exact_codec() {
    let port = pool(1000);
    let module = port
        .validated_module()
        .unwrap_or_else(|error| panic!("module: {error}"));
    let mut storage = Storage::new();
    deposit(&module, &port, &mut storage, principal(11), 40);
    let original = storage.clone();
    for grant in [Capability::StorageRead, Capability::SharedStorageRead] {
        let result = execute(
            &port,
            &module,
            &mut storage,
            program(1),
            principal(11),
            CapabilitySet::new([grant]).unwrap_or_else(|error| panic!("grants: {error}")),
            &query_data(),
            POOL_QUERY_BYTES,
        );
        assert_eq!(
            result,
            Err(ExecutionError::Composition(CompositionRefusal::Authority(
                AbiError::CapabilityDenied
            )))
        );
        assert_eq!(storage, original);
    }
    let mut encoded =
        PoolReserve::encode_initial(AUTHORITY).unwrap_or_else(|error| panic!("schema: {error}"));
    encoded.extend(
        ParticipantDeposit::schema()
            .and_then(|schema| schema.encode(&[FieldValue::U64(0), FieldValue::U32(0)]))
            .unwrap_or_else(|error| panic!("schema: {error}")),
    );
    assert!(PoolView::decode(&encoded).is_ok());
    assert_eq!(
        PoolView::decode(&encoded[..POOL_QUERY_BYTES - 1]),
        Err(PortRefusal::AccountBounds)
    );
    encoded.push(0);
    assert_eq!(PoolView::decode(&encoded), Err(PortRefusal::AccountBounds));
    encoded.pop();
    encoded[0] ^= 1;
    assert_eq!(
        PoolView::decode(&encoded),
        Err(PortRefusal::DiscriminatorMismatch)
    );
}

#[test]
fn pool_execution_proof_raw_host_denial_discards_staged_writes() {
    let port = pool(1000);
    let module = port
        .validated_module()
        .unwrap_or_else(|error| panic!("module: {error}"));
    let mut storage = Storage::new();
    deposit(&module, &port, &mut storage, principal(11), 40);
    let original = storage.clone();
    let write_bytes = PoolReserve::storage_key()
        .unwrap_or_else(|error| panic!("key: {error}"))
        .len()
        + PoolReserve::schema()
            .unwrap_or_else(|error| panic!("schema: {error}"))
            .space()
        + ParticipantDeposit::storage_key()
            .unwrap_or_else(|error| panic!("key: {error}"))
            .len()
        + ParticipantDeposit::schema()
            .unwrap_or_else(|error| panic!("schema: {error}"))
            .space();
    // Bypass the public call preflight to prove the guest's own transfer denial
    // happens after BOTH writes were metered and staged by the real host.
    for actor in [11, 12] {
        for maximum_amount in [None, Some(9)] {
            let mut grants = vec![
                Capability::StorageRead,
                Capability::StorageWrite,
                Capability::SharedStorageRead,
                Capability::SharedStorageWrite,
            ];
            if let Some(maximum_amount) = maximum_amount {
                grants.push(Capability::Transfer402 {
                    asset: ASSET,
                    to: DESTINATION,
                    maximum_amount,
                });
            }
            let record = Executor::declared()
                .execute_authorized_v2(
                    &mut storage,
                    AuthorizedExecutionRequest {
                        module: &module,
                        program: program(1),
                        authorization: AuthorizationContext::new(
                            principal(actor),
                            CapabilitySet::new(grants)
                                .unwrap_or_else(|error| panic!("grants: {error}")),
                        ),
                        receipts: &AbsentReceipts,
                        entrypoint: CALL_ENTRY_EXPORT,
                        calldata: &deposit_data(10),
                        composition: CompositionContext::isolated(),
                        response_capacity: 0,
                    },
                )
                .unwrap_or_else(|error| panic!("raw host execution: {error}"));
            assert_eq!(
                record.execution().usage().storage_write_bytes,
                write_bytes as u64
            );
            assert_guest_failure(Ok(record));
            assert_eq!(
                storage, original,
                "late transfer refusal must discard both staged writes"
            );
            storage = reopen(storage);
            assert_eq!(storage, original);
        }
    }
    // Preflight belongs to this precise emitted program, including its bound.
    let other_port = pool(999);
    let mismatch = other_port.execute(
        &mut storage,
        AuthorizedExecutionRequest {
            module: &module,
            program: program(1),
            authorization: AuthorizationContext::new(
                principal(11),
                port.deposit_capabilities(1)
                    .unwrap_or_else(|error| panic!("grants: {error}")),
            ),
            receipts: &AbsentReceipts,
            entrypoint: CALL_ENTRY_EXPORT,
            calldata: &deposit_data(1),
            composition: CompositionContext::isolated(),
            response_capacity: 0,
        },
    );
    assert_eq!(
        mismatch,
        Err(ExecutionError::Abi(AbiError::InvalidEncoding))
    );
    assert_eq!(storage, original);
}
