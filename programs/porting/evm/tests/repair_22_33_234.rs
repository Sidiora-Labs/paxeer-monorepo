use std::sync::atomic::{AtomicU64, Ordering};

use layerx_programs_runtime::{
    AbiEffects, AuthorizationContext, AuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, Executor, Lifecycle, PrincipalId, ProgramId, Storage, StorageNamespace,
    TransferSource, V2ActivityOutcome, V2AuthorizedExecutionRecord, ValidatedModule, WasmEngine,
    ABI_V2_VERSION,
};
use programs_porting_evm::shared_supply::{
    SharedSupplyPort, TokenTerms, BALANCE_OF_ENTRY_EXPORT, MAX_SUPPLY, MINT_ENTRY_EXPORT,
    TOTAL_SUPPLY_ENTRY_EXPORT,
};
use programs_porting_evm::{
    caller_indexed_import, external_call, import_state, AbsentReceipts, Address, EventAbi,
    MethodAbi, PortRefusal, Word,
};

const STATE_LIMIT: usize = 1_048_576;
const ASSET: [u8; 32] = [41; 32];
const RECIPIENT: [u8; 32] = [42; 32];
const PRICE: u64 = 17;

fn program() -> ProgramId {
    ProgramId::new([31; 32]).expect("program")
}

fn principal() -> PrincipalId {
    PrincipalId::new([32; 32]).expect("principal")
}

fn port(price: u64, asset: [u8; 32], recipient: [u8; 32]) -> SharedSupplyPort {
    SharedSupplyPort::new_with_recipient(
        TokenTerms {
            asset,
            price_per_token: price,
        },
        recipient,
    )
    .expect("validated terms")
}

fn reopen(storage: Storage) -> Storage {
    static NEXT: AtomicU64 = AtomicU64::new(0);
    let path = std::env::temp_dir().join(format!(
        "repair_22_33_234-{}-{}",
        std::process::id(),
        NEXT.fetch_add(1, Ordering::Relaxed)
    ));
    let encoded = storage
        .replay_state_bytes(STATE_LIMIT)
        .expect("encode storage");
    std::fs::write(&path, &encoded).expect("persist storage");
    drop(storage);
    let recovered = Storage::decode_untrusted_replay_state(
        &std::fs::read(&path).expect("read durable state"),
        STATE_LIMIT,
    )
    .expect("recover storage");
    std::fs::remove_file(path).expect("remove checkpoint");
    assert_eq!(
        recovered
            .replay_state_bytes(STATE_LIMIT)
            .expect("canonical state"),
        encoded
    );
    recovered
}

struct NativeToken {
    port: SharedSupplyPort,
    module: ValidatedModule,
    storage: Storage,
}

impl NativeToken {
    fn new(port: SharedSupplyPort) -> Self {
        let deployment = port.deployment_for_terms(program()).expect("deployment");
        assert_eq!(deployment.abi_version, ABI_V2_VERSION);
        assert_eq!(
            deployment.wasm,
            port.code_for_terms().expect("deterministic code")
        );
        let engine = WasmEngine::declared().expect("engine");
        let module = engine
            .validate_versioned(deployment.abi_version, &deployment.wasm)
            .expect("native module");
        let receipt = Lifecycle::declared()
            .expect("lifecycle")
            .deploy(deployment)
            .expect("deploy");
        assert_eq!(receipt.program(), program());
        Self {
            port,
            module,
            storage: Storage::new(),
        }
    }

    fn invoke(
        &mut self,
        grants: CapabilitySet,
        entry: &str,
        input: &[u8],
        capacity: usize,
    ) -> V2AuthorizedExecutionRecord {
        Executor::declared()
            .execute_authorized_v2(
                &mut self.storage,
                AuthorizedExecutionRequest {
                    module: &self.module,
                    program: program(),
                    authorization: AuthorizationContext::new(principal(), grants),
                    receipts: &AbsentReceipts,
                    entrypoint: entry,
                    calldata: input,
                    composition: CompositionContext::isolated(),
                    response_capacity: capacity,
                },
            )
            .expect("native activity")
    }

    fn mint(&mut self, amount: u64) -> V2AuthorizedExecutionRecord {
        self.invoke(
            self.port.mint_capabilities(amount).expect("mint grants"),
            MINT_ENTRY_EXPORT,
            &amount.to_be_bytes(),
            32,
        )
    }

    fn write(&mut self, shared: bool, value: &[u8]) {
        let (namespace, key) = if shared {
            (
                StorageNamespace::shared(program()),
                SharedSupplyPort::total_supply_key(),
            )
        } else {
            (
                StorageNamespace::principal(program(), principal()),
                SharedSupplyPort::balance_key(),
            )
        };
        let mut tx = self.storage.transaction(namespace);
        tx.write(&key, value).expect("seed migration state");
        assert_eq!(tx.commit(), 1);
    }

    fn views(&mut self, supply: u64, balance: u64) {
        for (grants, entry, expected) in [
            (
                SharedSupplyPort::supply_query_capabilities().expect("supply grants"),
                TOTAL_SUPPLY_ENTRY_EXPORT,
                supply,
            ),
            (
                SharedSupplyPort::balance_query_capabilities().expect("balance grants"),
                BALANCE_OF_ENTRY_EXPORT,
                balance,
            ),
        ] {
            let record = self.invoke(grants, entry, &[], 32);
            let effects = success(&record, expected);
            assert_eq!(effects, &AbiEffects::default());
        }
    }

    fn refuses(&mut self, amount: u64, grants: CapabilitySet, capacity: usize) {
        let before = self.storage.clone();
        let bytes = before.replay_state_bytes(STATE_LIMIT).expect("before");
        let record = self.invoke(grants, MINT_ENTRY_EXPORT, &amount.to_be_bytes(), capacity);
        assert!(
            matches!(record.outcome(), V2ActivityOutcome::Failure(_)),
            "{record:?}"
        );
        assert_eq!(self.storage, before, "failed mint published storage");
        assert_eq!(
            self.storage.replay_state_bytes(STATE_LIMIT).expect("after"),
            bytes
        );
    }
}

fn success(record: &V2AuthorizedExecutionRecord, supply: u64) -> &AbiEffects {
    match record.outcome() {
        V2ActivityOutcome::Success { response, effects } => {
            assert_eq!(response.code, 0);
            assert_eq!(response.bytes, Word::from_u64(supply).bytes());
            effects
        }
        outcome => panic!("expected successful mint/query: {outcome:?}"),
    }
}

fn grants(
    asset: [u8; 32],
    recipient: [u8; 32],
    maximum: Option<u128>,
    event: bool,
) -> CapabilitySet {
    let mut capabilities = vec![
        Capability::StorageRead,
        Capability::StorageWrite,
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
    ];
    if event {
        capabilities.push(Capability::EmitEvent);
    }
    if let Some(maximum_amount) = maximum {
        capabilities.push(Capability::Transfer402 {
            asset,
            to: recipient,
            maximum_amount,
        });
    }
    CapabilitySet::new(capabilities).expect("capabilities")
}

#[test]
fn evm_partial_import_invalid_final_cell_reopen_and_retry() {
    let holders = [
        (Address::new([1; 20]).expect("holder"), principal().bytes()),
        (Address::new([2; 20]).expect("holder"), [33; 32]),
    ];
    let cells = caller_indexed_import(1, &holders).expect("real import producer");
    let original = Word::from_u64(3).bytes();
    let mut storage = Storage::new();
    assert_eq!(
        import_state(&mut storage, program(), &[(cells[0], original)]),
        Ok(1)
    );
    let before = storage.clone();
    let mut plan = vec![
        (cells[0], Word::from_u64(10).bytes()),
        (cells[1], Word::from_u64(20).bytes()),
    ];
    plan[1].0.principal = [0; 32];
    assert!(import_state(&mut storage, program(), &plan).is_err());
    assert_eq!(storage, before);
    let mut recovered = reopen(storage);
    assert_eq!(recovered, before);
    assert_eq!(
        recovered
            .transaction(StorageNamespace::principal(program(), principal()))
            .read(&cells[0].layerx_key)
            .expect("old cell"),
        Some(original.to_vec())
    );
    plan[1].0 = cells[1];
    assert_eq!(import_state(&mut recovered, program(), &plan), Ok(2));
    let mut recovered = reopen(recovered);
    for (cell, value) in &plan {
        let namespace = StorageNamespace::principal(
            program(),
            PrincipalId::new(cell.principal).expect("owner"),
        );
        assert_eq!(
            recovered
                .transaction(namespace)
                .read(&cell.layerx_key)
                .expect("imported cell"),
            Some(value.to_vec())
        );
    }
    assert_eq!(
        import_state(&mut recovered, program(), &plan),
        Ok(0),
        "retry must be idempotent"
    );
}

fn hex(bytes: &[u8]) -> String {
    bytes.iter().map(|byte| format!("{byte:02x}")).collect()
}

#[test]
fn unsupported_solidity_signatures_refuse_dynamic_compound_and_malformed() {
    for signature in [
        "f(bytes)",
        "f(string)",
        "f(uint256[])",
        "f(uint256[2])",
        "f((uint256,address))",
        "f(tuple)",
        "f((uint256,address),bytes32)",
        "f(uint)",
        "f(int)",
        "f(byte)",
        "f(uint0)",
        "f(uint7)",
        "f(uint264)",
        "f(uint08)",
        "f(int257)",
        "f(bytes0)",
        "f(bytes33)",
        "f(bytes01)",
        "f(address payable)",
        "f(uint256 x)",
        "f(uint+8)",
        "f(,uint256)",
        "f(uint256,)",
        "f(uint256,,bool)",
        "(bool)",
        "1f(bool)",
        "f (bool)",
        "f(bool))",
        "f((bool)",
        "f(bool)trailing",
        "f(bool",
        "f(bool)()",
        "f( bool)",
        "f(bool )",
        "f(\n)",
        "f(é)",
        "é(bool)",
        "f(function)",
    ] {
        assert_eq!(
            MethodAbi::new(signature),
            Err(PortRefusal::InvalidSignature),
            "{signature}"
        );
        assert_eq!(
            EventAbi::new(signature),
            Err(PortRefusal::InvalidSignature),
            "{signature}"
        );
    }
    let too_long = format!("{}(uint256)", "f".repeat(256));
    assert_eq!(
        MethodAbi::new(&too_long),
        Err(PortRefusal::InvalidSignature)
    );
    for width in (8..=256).step_by(8) {
        for kind in ["int", "uint"] {
            assert_eq!(
                MethodAbi::new(&format!("f({kind}{width})"))
                    .expect("scalar")
                    .parameters(),
                1
            );
        }
    }
    for width in 1..=32 {
        assert_eq!(
            EventAbi::new(&format!("E(bytes{width})"))
                .expect("fixed bytes")
                .arguments(),
            1
        );
    }
    assert_eq!(
        MethodAbi::new("_f$(address,bool)")
            .expect("identifier")
            .parameters(),
        2
    );
    assert_eq!(
        MethodAbi::new("f()")
            .expect("no arguments")
            .calldata(&[])
            .expect("calldata")
            .len(),
        4
    );
}

#[test]
fn supported_solidity_selector_calldata_and_event_vectors() {
    let method = MethodAbi::new("transfer(address,uint256)").expect("transfer");
    assert_eq!(hex(&method.selector()), "a9059cbb");
    let address = Word::from_address(Address::new([0x11; 20]).expect("address"));
    let request =
        external_call(program(), &method, &[address, Word::from_u64(7)]).expect("external call");
    assert_eq!(request.callee, program());
    assert_eq!(request.authority, Capability::Call { program: program() });
    assert_eq!(
        hex(&request.input),
        concat!(
            "a9059cbb",
            "0000000000000000000000001111111111111111111111111111111111111111",
            "0000000000000000000000000000000000000000000000000000000000000007"
        )
    );
    assert_eq!(
        hex(&MethodAbi::new("balanceOf(address)")
            .expect("balance")
            .selector()),
        "70a08231"
    );
    assert_eq!(
        method.calldata(&[]),
        Err(PortRefusal::ArgumentCountMismatch)
    );
    let event = EventAbi::envelope_derived("Transfer(address,address,uint256)", 2).expect("event");
    assert_eq!(
        hex(&event.topic()),
        "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
    );
    assert_eq!(
        event.data(&[Word::from_u64(7)]).expect("event data"),
        Word::from_u64(7).bytes()
    );
}

#[test]
fn shared_supply_payment_pins_terms_and_native_paid_mint() {
    let configured = port(PRICE, ASSET, RECIPIENT);
    let code = configured.code_for_terms().expect("code");
    for alternate in [
        port(PRICE + 1, ASSET, RECIPIENT),
        port(PRICE, [43; 32], RECIPIENT),
        port(PRICE, ASSET, [44; 32]),
    ] {
        assert_ne!(
            code,
            alternate
                .code_for_terms()
                .expect("changed terms change code")
        );
    }
    let mut token = NativeToken::new(configured);
    token.views(0, 0);
    let record = token.mint(5);
    let effects = success(&record, 5);
    assert_eq!(effects.transfers.len(), 1);
    let payment = &effects.transfers[0];
    assert_eq!(payment.program, program());
    assert_eq!(payment.principal, principal());
    assert_eq!(payment.source, TransferSource::Principal(principal()));
    assert_eq!(payment.asset, ASSET);
    assert_eq!(payment.to, RECIPIENT);
    assert_eq!(payment.amount, 5 * u128::from(PRICE));
    assert_eq!(effects.events.len(), 1);
    let event = &effects.events[0];
    assert_eq!(event.program, program());
    assert_eq!(event.principal, principal());
    assert_eq!(event.frame, payment.frame);
    assert_eq!(
        hex(&event.topic),
        "ddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef"
    );
    assert_eq!(event.data, Word::from_u64(5).bytes());
    assert!(effects.calls.is_empty());
    token.storage = reopen(token.storage);
    token.views(5, 5);
}

#[test]
fn shared_supply_payment_insufficient_authority_and_late_failure_rollback() {
    let mut token = NativeToken::new(port(PRICE, ASSET, RECIPIENT));
    success(&token.mint(2), 2);
    let amount = 3;
    let price = u128::from(amount * PRICE);
    for grant in [
        grants(ASSET, RECIPIENT, None, true),
        grants(ASSET, RECIPIENT, Some(price - 1), true),
        grants([45; 32], RECIPIENT, Some(price), true),
        grants(ASSET, [46; 32], Some(price), true),
        grants(ASSET, RECIPIENT, Some(price), false),
    ] {
        token.refuses(amount, grant, 32);
        token.views(2, 2);
    }
    // Response publication fails after both writes, the transfer, and the event.
    token.refuses(
        amount,
        token.port.mint_capabilities(amount).expect("grants"),
        0,
    );
    token.storage = reopen(token.storage);
    token.views(2, 2);
}

#[test]
fn shared_supply_payment_rejects_invalid_deployment_terms() {
    for (asset, recipient, price) in [
        ([0; 32], RECIPIENT, PRICE),
        (ASSET, [0; 32], PRICE),
        (ASSET, RECIPIENT, 0),
        (ASSET, RECIPIENT, u64::MAX / MAX_SUPPLY + 1),
    ] {
        assert_eq!(
            SharedSupplyPort::new_with_recipient(
                TokenTerms {
                    asset,
                    price_per_token: price
                },
                recipient
            ),
            Err(PortRefusal::OutOfRange)
        );
    }
    let configured = port(u64::MAX / MAX_SUPPLY, ASSET, RECIPIENT);
    assert!(configured.mint_capabilities(0).is_err());
    assert!(configured.mint_capabilities(MAX_SUPPLY + 1).is_err());
    let mut token = NativeToken::new(configured);
    let record = token.mint(MAX_SUPPLY);
    let effects = success(&record, MAX_SUPPLY);
    assert_eq!(
        effects.transfers[0].amount,
        u128::from(u64::MAX / MAX_SUPPLY) * u128::from(MAX_SUPPLY)
    );
    token.views(MAX_SUPPLY, MAX_SUPPLY);
}

#[test]
fn shared_supply_overflow_boundaries_and_independent_balance_check() {
    let mut token = NativeToken::new(port(PRICE, ASSET, RECIPIENT));
    token.write(true, &Word::from_u64(MAX_SUPPLY - 1).bytes());
    token.write(false, &Word::from_u64(u64::MAX - 1).bytes());
    success(&token.mint(1), MAX_SUPPLY);
    token.views(MAX_SUPPLY, u64::MAX);
    token.refuses(1, token.port.mint_capabilities(1).expect("grants"), 32);
    for amount in [0, MAX_SUPPLY + 1, i64::MAX as u64, 1_u64 << 63, u64::MAX] {
        let mut token = NativeToken::new(port(PRICE, ASSET, RECIPIENT));
        token.write(true, &Word::from_u64(1).bytes());
        token.refuses(amount, grants(ASSET, RECIPIENT, Some(u128::MAX), true), 32);
        token.views(1, 0);
    }
    let mut token = NativeToken::new(port(PRICE, ASSET, RECIPIENT));
    token.write(false, &Word::from_u64(u64::MAX).bytes());
    token.refuses(1, token.port.mint_capabilities(1).expect("grants"), 32);
    token.views(0, u64::MAX);
    for supply in [MAX_SUPPLY + 1, i64::MAX as u64, u64::MAX] {
        let mut token = NativeToken::new(port(PRICE, ASSET, RECIPIENT));
        token.write(true, &Word::from_u64(supply).bytes());
        token.refuses(1, token.port.mint_capabilities(1).expect("grants"), 32);
    }
}

#[test]
fn shared_supply_overflow_malformed_stored_words_roll_back() {
    let mut malformed = vec![vec![], vec![0], vec![0; 31], vec![0; 33]];
    for offset in [0, 8, 16, 23] {
        let mut word = vec![0; 32];
        word[offset] = 1;
        malformed.push(word);
    }
    for shared in [true, false] {
        for value in &malformed {
            let mut token = NativeToken::new(port(PRICE, ASSET, RECIPIENT));
            token.write(shared, value);
            let before = token.storage.clone();
            token.refuses(1, token.port.mint_capabilities(1).expect("grants"), 32);
            let (capabilities, entry) = if shared {
                (
                    SharedSupplyPort::supply_query_capabilities().expect("grants"),
                    TOTAL_SUPPLY_ENTRY_EXPORT,
                )
            } else {
                (
                    SharedSupplyPort::balance_query_capabilities().expect("grants"),
                    BALANCE_OF_ENTRY_EXPORT,
                )
            };
            let record = token.invoke(capabilities, entry, &[], 32);
            assert!(
                matches!(record.outcome(), V2ActivityOutcome::Failure(_)),
                "malformed word: {value:?}"
            );
            assert_eq!(token.storage, before);
            assert_eq!(reopen(token.storage), before);
        }
    }
}
