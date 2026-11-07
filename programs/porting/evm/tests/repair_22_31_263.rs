use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_programs_runtime::{
    AuthorizationContext, AuthorizedExecutionRequest, Capability, CapabilitySet,
    CompositionContext, Executor, Lifecycle, PrincipalId, ProgramId, Storage, StorageNamespace,
    V2ActivityOutcome, V2AuthorizedExecutionRecord, ValidatedModule, WasmEngine, ABI_V1_VERSION,
    ABI_V2_VERSION,
};
use programs_porting_evm::shared_supply::{
    SharedSupplyPort, TokenTerms, BALANCE_OF_ENTRY_EXPORT, MAX_SUPPLY, MINT_ENTRY_EXPORT,
    TOTAL_SUPPLY_ENTRY_EXPORT,
};
use programs_porting_evm::{AbsentReceipts, Word};

const STATE_LIMIT: usize = 1_048_576;

struct NativeToken {
    module: ValidatedModule,
    program: ProgramId,
    storage: Storage,
}

impl NativeToken {
    fn deploy(program: ProgramId) -> Self {
        let deployment = SharedSupplyPort::deployment(program)
            .unwrap_or_else(|error| panic!("deployment: {error}"));
        assert_eq!(deployment.abi_version, ABI_V2_VERSION);
        let engine = WasmEngine::declared().unwrap_or_else(|error| panic!("engine: {error}"));
        let module = engine
            .validate_versioned(deployment.abi_version, &deployment.wasm)
            .unwrap_or_else(|error| panic!("module: {error}"));
        let mut lifecycle =
            Lifecycle::declared().unwrap_or_else(|error| panic!("lifecycle: {error}"));
        let receipt = lifecycle
            .deploy(deployment)
            .unwrap_or_else(|error| panic!("deploy: {error}"));
        assert_eq!(receipt.program(), program);
        assert_eq!(receipt.version(), 1);
        Self {
            module,
            program,
            storage: Storage::new(),
        }
    }

    fn invoke(
        &mut self,
        principal: PrincipalId,
        capabilities: CapabilitySet,
        entrypoint: &str,
        calldata: &[u8],
    ) -> V2AuthorizedExecutionRecord {
        Executor::declared()
            .execute_authorized_v2(
                &mut self.storage,
                AuthorizedExecutionRequest {
                    module: &self.module,
                    program: self.program,
                    authorization: AuthorizationContext::new(principal, capabilities),
                    receipts: &AbsentReceipts,
                    entrypoint,
                    calldata,
                    composition: CompositionContext::isolated(),
                    response_capacity: 32,
                },
            )
            .unwrap_or_else(|error| panic!("native execution: {error}"))
    }

    fn mint(&mut self, principal: PrincipalId, amount: u64) -> V2AuthorizedExecutionRecord {
        let port = SharedSupplyPort::new(TokenTerms {
            asset: [7; 32],
            price_per_token: 1,
        })
        .unwrap_or_else(|error| panic!("port: {error}"));
        self.invoke(
            principal,
            port.mint_capabilities(amount)
                .unwrap_or_else(|error| panic!("mint grant: {error}")),
            MINT_ENTRY_EXPORT,
            &amount.to_be_bytes(),
        )
    }

    fn assert_views(&mut self, principal: PrincipalId, supply: u64, balance: u64) {
        let supply_result = self.invoke(
            principal,
            SharedSupplyPort::supply_query_capabilities()
                .unwrap_or_else(|error| panic!("supply grant: {error}")),
            TOTAL_SUPPLY_ENTRY_EXPORT,
            &[],
        );
        assert_response(&supply_result, supply);
        let balance_result = self.invoke(
            principal,
            SharedSupplyPort::balance_query_capabilities()
                .unwrap_or_else(|error| panic!("balance grant: {error}")),
            BALANCE_OF_ENTRY_EXPORT,
            &[],
        );
        assert_response(&balance_result, balance);
    }

    fn checkpoint(self, path: &Path) {
        let bytes = self
            .storage
            .replay_state_bytes(STATE_LIMIT)
            .unwrap_or_else(|error| panic!("checkpoint: {error:?}"));
        std::fs::write(path, bytes).unwrap_or_else(|error| panic!("persist: {error}"));
    }

    fn reopen(program: ProgramId, path: &Path) -> Self {
        let mut token = Self::deploy(program);
        let bytes = std::fs::read(path).unwrap_or_else(|error| panic!("reopen: {error}"));
        token.storage = Storage::decode_untrusted_replay_state(&bytes, STATE_LIMIT)
            .unwrap_or_else(|error| panic!("recover: {error:?}"));
        assert_eq!(token.storage.replay_state_bytes(STATE_LIMIT), Ok(bytes));
        token
    }
}

struct Checkpoint(PathBuf);

impl Checkpoint {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let directory = std::env::temp_dir().join(format!(
            "repair_22_31_263-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed),
        ));
        std::fs::create_dir(&directory)
            .unwrap_or_else(|error| panic!("checkpoint directory: {error}"));
        Self(directory.join("storage"))
    }
}

impl Drop for Checkpoint {
    fn drop(&mut self) {
        if let Some(directory) = self.0.parent() {
            std::fs::remove_dir_all(directory).unwrap_or_else(|error| panic!("cleanup: {error}"));
        }
    }
}

fn program(byte: u8) -> ProgramId {
    ProgramId::new([byte; 32]).unwrap_or_else(|error| panic!("program: {error}"))
}

fn principal(byte: u8) -> PrincipalId {
    PrincipalId::new([byte; 32]).unwrap_or_else(|error| panic!("principal: {error}"))
}

fn assert_response(record: &V2AuthorizedExecutionRecord, expected: u64) {
    match record.outcome() {
        V2ActivityOutcome::Success { response, .. } => {
            assert_eq!(response.code, 0);
            assert_eq!(response.bytes.as_slice(), &Word::from_u64(expected).bytes());
        }
        outcome => panic!("expected successful token response, got {outcome:?}"),
    }
}

#[test]
fn shared_supply_namespace_two_principals_native_execution_and_reopen() {
    let owner = program(1);
    let alice = principal(2);
    let bob = principal(3);
    let checkpoint = Checkpoint::new();
    let mut token = NativeToken::deploy(owner);
    token.assert_views(alice, 0, 0);
    token.assert_views(bob, 0, 0);
    assert_response(&token.mint(alice, 10), 10);
    token.assert_views(alice, 10, 10);
    token.assert_views(bob, 10, 0);
    assert_response(&token.mint(bob, 7), 17);
    token.assert_views(alice, 17, 10);
    token.assert_views(bob, 17, 7);
    assert_eq!(
        token
            .storage
            .namespace_cell_count(StorageNamespace::shared(owner)),
        1
    );
    for actor in [alice, bob] {
        let namespace = token
            .storage
            .transaction(StorageNamespace::principal(owner, actor));
        assert_eq!(
            namespace.read(&SharedSupplyPort::total_supply_key()),
            Ok(None)
        );
        // Each holder has precisely its own balance, never a principal copy of totalSupply.
        assert_eq!(namespace.commit(), 0);
        assert_eq!(
            token
                .storage
                .namespace_cell_count(StorageNamespace::principal(owner, actor)),
            1
        );
    }
    assert_eq!(
        token
            .storage
            .transaction(StorageNamespace::shared(owner))
            .read(&SharedSupplyPort::balance_key()),
        Ok(None)
    );
    token.checkpoint(&checkpoint.0);
    let mut reopened = NativeToken::reopen(owner, &checkpoint.0);
    reopened.assert_views(alice, 17, 10);
    reopened.assert_views(bob, 17, 7);
    assert_response(&reopened.mint(bob, 3), 20);
    reopened.assert_views(alice, 20, 10);
    reopened.assert_views(bob, 20, 10);
    reopened.checkpoint(&checkpoint.0);
    let mut recovered_again = NativeToken::reopen(owner, &checkpoint.0);
    recovered_again.assert_views(alice, 20, 10);
    recovered_again.assert_views(bob, 20, 10);
    let mut other_program = NativeToken::deploy(program(4));
    other_program.storage = recovered_again.storage.clone();
    other_program.assert_views(alice, 0, 0);
    other_program.assert_views(bob, 0, 0);
}

#[test]
fn shared_supply_namespace_enforces_scope_grants_and_atomic_mint() {
    let actor = principal(12);
    let mut token = NativeToken::deploy(program(11));
    assert_response(&token.mint(actor, 9), 9);
    let snapshot = token.storage.clone();
    for (grant, entrypoint) in [
        (Capability::StorageRead, TOTAL_SUPPLY_ENTRY_EXPORT),
        (Capability::SharedStorageRead, BALANCE_OF_ENTRY_EXPORT),
    ] {
        let record = token.invoke(
            actor,
            CapabilitySet::new([grant]).unwrap_or_else(|error| panic!("grant: {error}")),
            entrypoint,
            &[],
        );
        assert!(matches!(record.outcome(), V2ActivityOutcome::Failure(_)));
        assert_eq!(token.storage, snapshot);
    }
    let required = [
        Capability::SharedStorageRead,
        Capability::SharedStorageWrite,
        Capability::StorageRead,
        Capability::StorageWrite,
    ];
    for missing in 0..required.len() {
        let grants = CapabilitySet::new(
            required
                .iter()
                .enumerate()
                .filter(|(index, _)| *index != missing)
                .map(|(_, grant)| grant.clone()),
        )
        .unwrap_or_else(|error| panic!("mint grants: {error}"));
        let record = token.invoke(actor, grants, MINT_ENTRY_EXPORT, &5u64.to_be_bytes());
        assert!(matches!(record.outcome(), V2ActivityOutcome::Failure(_)));
        assert_eq!(
            token.storage, snapshot,
            "partial mint must roll back when grant {missing} is absent"
        );
    }
    token.assert_views(actor, 9, 9);
}

#[test]
fn shared_supply_namespace_deployment_abi_and_global_bound() {
    let owner = program(21);
    let mut legacy =
        SharedSupplyPort::deployment(owner).unwrap_or_else(|error| panic!("deployment: {error}"));
    legacy.abi_version = ABI_V1_VERSION;
    let mut lifecycle = Lifecycle::declared().unwrap_or_else(|error| panic!("lifecycle: {error}"));
    assert!(
        lifecycle.deploy(legacy).is_err(),
        "scoped imports require ABI v2 deployment"
    );
    let mut token = NativeToken::deploy(owner);
    let alice = principal(22);
    let bob = principal(23);
    assert_response(&token.mint(alice, MAX_SUPPLY), MAX_SUPPLY);
    let checkpoint = Checkpoint::new();
    token.checkpoint(&checkpoint.0);
    let mut reopened = NativeToken::reopen(owner, &checkpoint.0);
    let snapshot = reopened.storage.clone();
    assert!(matches!(
        reopened.mint(bob, 1).outcome(),
        V2ActivityOutcome::Failure(_)
    ));
    assert_eq!(reopened.storage, snapshot);
    reopened.assert_views(alice, MAX_SUPPLY, MAX_SUPPLY);
    reopened.assert_views(bob, MAX_SUPPLY, 0);
}
