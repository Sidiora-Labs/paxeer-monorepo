//! Repair record 22.31.266, audit 22.30.130:
//! occupancy-ledger-capacity: `occupancy_ledger_capacity_principal` and
//! `occupancy_ledger_capacity_shared` prove atomic admission, maximum publication,
//! persisted-ledger reopen and next-block settlement using the native producer's
//! shared Rust path and codecs.
//! occupancy-unpaid-replay: `occupancy_unpaid_replay_paid`,
//! `occupancy_unpaid_replay_unpaid` and `occupancy_unpaid_replay_mixed_payer` prove
//! authenticated aggregate debit outcomes, altered-outcome rejection and recovery.
//! `occupancy_unpaid_replay_deleted_namespace` preserves unpaid debt after deletion.
//! Baseline verification was not run: the explicit implementation-first gate order
//! and single verify budget prohibit the conflicting unchanged-tree run in step 6.
//! No out-of-path wiring is required; the native adapter is repaired in `ffi_call.rs`.
//! The initial build refused a private executor constructor; the fixture uses `Executor::new`.
//! The gate runner records the verified candidate revision and private log.

use std::collections::BTreeMap;

use layerx_programs_runtime::occupancy::OccupancyDebitAuthority;
use layerx_programs_runtime::{
    ActivityBudgetBinding, DeclaredBudget, Executor, FeeSchedule, FeeScheduleParameters,
    OccupancyDisposition, OccupancyError, OccupancyLedger, OccupancyPaymentAccount,
    OccupancyResponsibility, OccupancySettlement, PrincipalId, ProgramId, ResourceBudget, Storage,
    StorageNamespace, MAX_OCCUPANCY_EVIDENCE_BYTES, MAX_OCCUPANCY_LEDGER_BYTES,
    MAX_OCCUPANCY_POSITIONS,
};

const ASSET: [u8; 32] = [1; 32];

fn schedule() -> FeeSchedule {
    FeeSchedule::new_complete(FeeScheduleParameters {
        version: 7,
        fee_units_per_cpu_fuel: 1,
        fee_units_per_memory_byte: 1,
        fee_units_per_storage_read_byte: 1,
        fee_units_per_storage_write_byte: 1,
        fee_units_per_output_value: 1,
        fee_units_per_output_byte: 1,
        fee_units_per_occupancy_byte_batch: 2,
    })
}

fn program(index: usize) -> ProgramId {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut bytes = [0x51; 32];
    bytes[..8].copy_from_slice(&(index as u64).to_be_bytes());
    ProgramId::new(bytes).assert_success("test value")
}

fn write_ten_bytes(storage: &mut Storage, namespace: StorageNamespace) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut transaction = storage.transaction(namespace);
    transaction
        .write(b"k", &[0x61; 9])
        .assert_success("test value");
    assert_eq!(transaction.commit(), 1);
}

// A committed in-memory host account store, independent of the settlement wire.
// Accounts use the production DID/account codec; unknown or stale facts fail closed.
#[derive(Clone)]
struct CommittedAccounts {
    batch: u64,
    accounts: BTreeMap<PrincipalId, (OccupancyPaymentAccount, u128)>,
    queries: BTreeMap<PrincipalId, u128>,
}

impl CommittedAccounts {
    fn new(batch: u64, accounts: &[(OccupancyPaymentAccount, u128)]) -> Self {
        Self {
            batch,
            accounts: accounts
                .iter()
                .map(|(account, balance)| (account.payer(), (*account, *balance)))
                .collect(),
            queries: BTreeMap::new(),
        }
    }
}

impl OccupancyDebitAuthority for CommittedAccounts {
    fn can_debit(
        &mut self,
        batch: u64,
        payer: PrincipalId,
        amount: u128,
    ) -> Result<bool, OccupancyError> {
        if batch != self.batch {
            return Err(OccupancyError::UnprovenPaymentAccount);
        }
        let (account, balance) = self
            .accounts
            .get(&payer)
            .ok_or(OccupancyError::UnprovenPaymentAccount)?;
        if account.payer() != payer || account.asset_id() != ASSET {
            return Err(OccupancyError::UnprovenPaymentAccount);
        }
        assert!(self.queries.insert(payer, amount).is_none());
        Ok(*balance >= amount)
    }
}

fn inventory(
    count: usize,
    shared: bool,
) -> (
    Storage,
    BTreeMap<ProgramId, PrincipalId>,
    Vec<StorageNamespace>,
) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let payer = PrincipalId::new([7; 32]).assert_success("test value");
    let mut storage = Storage::new();
    let mut owners = BTreeMap::new();
    let namespaces = (0..count)
        .map(|index| {
            let program = program(index);
            owners.insert(program, payer);
            let namespace = if shared {
                StorageNamespace::shared(program)
            } else {
                StorageNamespace::principal(program, payer)
            };
            write_ten_bytes(&mut storage, namespace);
            namespace
        })
        .collect();
    (storage, owners, namespaces)
}

fn prove_capacity(shared: bool) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    // Derive the boundary from bytes emitted by the real producer, including
    // both headers and the actual principal/shared namespace encodings.
    let mut empty = OccupancyLedger::activated_after(9);
    let empty_settlement = empty
        .finalize_unchanged_batch(10, schedule(), &mut CommittedAccounts::new(10, &[]))
        .assert_success("test value");
    let (empty_state, empty_evidence) = empty
        .canonical_publication(&empty_settlement)
        .assert_success("test value");
    let (one_storage, one_owners, _) = inventory(1, shared);
    let mut one = OccupancyLedger::activated_after(9);
    one.import_activation_positions(&one_storage, &one_owners)
        .assert_success("test value");
    let one_settlement = one
        .finalize_unchanged_batch(10, schedule(), &mut CommittedAccounts::new(10, &[]))
        .assert_success("test value");
    let (one_state, one_evidence) = one
        .canonical_publication(&one_settlement)
        .assert_success("test value");
    let position_bytes = one_state.len() - empty_state.len();
    let charge_bytes = one_evidence.len() - empty_evidence.len();
    let maximum = MAX_OCCUPANCY_POSITIONS
        .min((MAX_OCCUPANCY_LEDGER_BYTES - empty_state.len()) / position_bytes)
        .min((MAX_OCCUPANCY_EVIDENCE_BYTES - empty_evidence.len()) / charge_bytes);
    assert_eq!(maximum, if shared { 197 } else { 180 });
    assert!(maximum < MAX_OCCUPANCY_POSITIONS);
    assert!(empty_evidence.len() + (maximum + 1) * charge_bytes > MAX_OCCUPANCY_EVIDENCE_BYTES);

    let (storage, owners, namespaces) = inventory(maximum, shared);
    let mut ledger = OccupancyLedger::activated_after(9);
    ledger
        .import_activation_positions(&storage, &owners)
        .assert_success("test value");
    let prior_bytes = ledger.canonical_state();
    assert_eq!(
        OccupancyLedger::canonical_decode(&prior_bytes).assert_success("test value"),
        ledger
    );

    // The complete oversized migration is rejected, with no admitted prefix.
    let (oversized_storage, oversized_owners, _) = inventory(maximum + 1, shared);
    let mut rejected = OccupancyLedger::activated_after(9);
    let rejected_before = rejected.canonical_state();
    assert_eq!(
        rejected.import_activation_positions(&oversized_storage, &oversized_owners),
        Err(OccupancyError::LengthLimit)
    );
    assert_eq!(rejected.canonical_state(), rejected_before);
    assert_eq!(
        ledger.import_activation_positions(&oversized_storage, &oversized_owners),
        Err(OccupancyError::LengthLimit)
    );
    assert_eq!(ledger.canonical_state(), prior_bytes);

    let settlement = ledger
        .finalize_unchanged_batch(10, schedule(), &mut CommittedAccounts::new(10, &[]))
        .assert_success("test value");
    assert_eq!(settlement.charges().len(), maximum);
    assert!(settlement
        .charges()
        .iter()
        .all(|charge| charge.disposition() == OccupancyDisposition::MigrationRequired));
    let (published_state, published_evidence) = ledger
        .canonical_publication(&settlement)
        .assert_success("test value");
    assert_eq!(
        published_state.len(),
        empty_state.len() + maximum * position_bytes
    );
    assert_eq!(
        published_evidence.len(),
        empty_evidence.len() + maximum * charge_bytes
    );
    assert!(published_state.len() <= MAX_OCCUPANCY_LEDGER_BYTES);
    assert!(published_evidence.len() <= MAX_OCCUPANCY_EVIDENCE_BYTES);
    assert_eq!(
        OccupancySettlement::canonical_decode(&published_evidence).assert_success("test value"),
        settlement
    );
    drop(ledger);

    assert_capacity_recovery(
        &prior_bytes,
        &published_state,
        &published_evidence,
        &storage,
        namespaces,
        maximum,
    );
}
fn assert_capacity_recovery(
    prior_bytes: &[u8],
    published_state: &[u8],
    published_evidence: &[u8],
    storage: &Storage,
    namespaces: Vec<StorageNamespace>,
    maximum: usize,
) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut replayed = OccupancyLedger::canonical_decode(prior_bytes).assert_success("test value");
    replayed
        .recover_finalized_evidence(
            published_evidence,
            storage,
            [],
            &mut CommittedAccounts::new(10, &[]),
        )
        .assert_success("test value");
    assert_eq!(replayed.canonical_state(), published_state);
    let mut reopened =
        OccupancyLedger::canonical_decode(published_state).assert_success("test value");
    assert_eq!(reopened.last_finalized_batch(), 10);
    for namespace in namespaces {
        assert_eq!(reopened.recorded_bytes(namespace), Some(10));
        assert_eq!(
            reopened.ensure_accessible([namespace]),
            Err(OccupancyError::FrozenNamespace { namespace })
        );
    }
    let next = reopened
        .finalize_unchanged_batch(11, schedule(), &mut CommittedAccounts::new(11, &[]))
        .assert_success("test value");
    assert_eq!(next.usage().byte_batches, maximum as u128 * 10);
    assert_eq!(next.usage().paid_fee_units, 0);
    assert!(next
        .charges()
        .iter()
        .all(|charge| { charge.start_batch() == 10 && charge.to_batch() == 11 }));
    let (next_state, next_evidence) = reopened
        .canonical_publication(&next)
        .assert_success("test value");
    assert_eq!(
        OccupancyLedger::canonical_decode(&next_state).assert_success("test value"),
        reopened
    );
    assert_eq!(
        OccupancySettlement::canonical_decode(&next_evidence).assert_success("test value"),
        next
    );
}

#[test]
fn occupancy_ledger_capacity_principal() {
    prove_capacity(false);
}

#[test]
fn occupancy_ledger_capacity_shared() {
    prove_capacity(true);
}

#[test]
fn occupancy_activation_late_invalid_record_is_atomic() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let (storage, owners, namespaces) = inventory(1, false);
    let payer = owners[&namespaces[0].program()];
    let mut ledger = OccupancyLedger::activated_after(9);
    ledger
        .import_activation_positions(&storage, &owners)
        .assert_success("test value");
    let before = ledger.canonical_state();
    let valid = StorageNamespace::principal(program(2), payer);
    let invalid = StorageNamespace::principal(program(3), payer);
    assert_eq!(
        ledger.import_activation_records([
            (valid, payer, 10),
            (
                invalid,
                PrincipalId::new([8; 32]).assert_success("test value"),
                10
            ),
        ]),
        Err(OccupancyError::AuthorityMismatch { namespace: invalid })
    );
    assert_eq!(ledger.canonical_state(), before);
    assert!(!ledger.contains_namespace(valid));
}

fn initialized(
    accounts: &[OccupancyPaymentAccount; 2],
) -> (Storage, Vec<StorageNamespace>, Vec<u8>) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let mut storage = Storage::new();
    let namespaces: Vec<_> = [accounts[0], accounts[0], accounts[1]]
        .iter()
        .enumerate()
        .map(|(index, account)| {
            let namespace = StorageNamespace::principal(program(index), account.payer());
            write_ten_bytes(&mut storage, namespace);
            namespace
        })
        .collect();
    let executor = Executor::new(ResourceBudget::declared(), schedule());
    let responsibilities: Vec<_> = namespaces
        .iter()
        .enumerate()
        .map(|(index, namespace)| {
            let admitted = executor
                .admit_activity_budget_for_qualification(
                    DeclaredBudget::new(100, 65_536, 0, 0, 1, 0, 0).assert_success("test value"),
                    namespace.principal_scope().assert_success("test value"),
                    ActivityBudgetBinding::new(
                        [u8::try_from(index).assert_success("fixture fits u8") + 1; 32],
                    )
                    .assert_success("test value"),
                    1_000_000_000,
                )
                .assert_success("test value");
            OccupancyResponsibility::from_admitted(
                &admitted,
                1_000_000_000,
                schedule(),
                namespace.program(),
                *namespace,
                10,
                40,
            )
            .assert_success("test value")
        })
        .collect();
    let mut ledger = OccupancyLedger::new();
    let mut initial_accounts = CommittedAccounts::new(1, &[]);
    ledger
        .settle_batch(
            1,
            &storage,
            responsibilities,
            schedule(),
            &mut initial_accounts,
        )
        .assert_success("test value");
    let first = ledger
        .finalize_unchanged_batch(1, schedule(), &mut initial_accounts)
        .assert_success("test value");
    assert!(initial_accounts.queries.is_empty());
    let (persisted, evidence) = ledger
        .canonical_publication(&first)
        .assert_success("test value");
    assert_eq!(
        OccupancySettlement::canonical_decode(&evidence).assert_success("test value"),
        first
    );
    drop(ledger);
    (storage, namespaces, persisted)
}

fn prove_replay(balance_a: u128, balance_b: u128) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let accounts = [
        OccupancyPaymentAccount::main(b"did:layerx:occupancy-a", ASSET)
            .assert_success("test value"),
        OccupancyPaymentAccount::main(b"did:layerx:occupancy-b", ASSET)
            .assert_success("test value"),
    ];
    let (storage, namespaces, prior_bytes) = initialized(&accounts);
    let snapshot = CommittedAccounts::new(2, &[(accounts[0], balance_a), (accounts[1], balance_b)]);
    let mut producer = OccupancyLedger::canonical_decode(&prior_bytes).assert_success("test value");
    let mut producer_accounts = snapshot.clone();
    let settlement = producer
        .finalize_unchanged_batch(2, schedule(), &mut producer_accounts)
        .assert_success("test value");
    assert_eq!(
        producer_accounts.queries,
        BTreeMap::from([(accounts[0].payer(), 40), (accounts[1].payer(), 20),])
    );
    let expected_paid = if balance_a >= 40 { 40 } else { 0 } + if balance_b >= 20 { 20 } else { 0 };
    assert_eq!(settlement.usage().paid_fee_units, expected_paid);
    assert_eq!(settlement.usage().arrears_fee_units, 60 - expected_paid);
    for charge in settlement.charges() {
        let funded = if charge.payer() == accounts[0].payer() {
            balance_a >= 40
        } else {
            balance_b >= 20
        };
        assert_eq!(charge.amount_due(), 20);
        assert_eq!(
            charge.disposition(),
            if funded {
                OccupancyDisposition::Paid
            } else {
                OccupancyDisposition::InsufficientFunds
            }
        );
        assert_eq!(charge.arrears_after(), if funded { 0 } else { 20 });
    }
    let (published_state, published_evidence) = producer
        .canonical_publication(&settlement)
        .assert_success("test value");
    assert_eq!(
        OccupancySettlement::canonical_decode(&published_evidence).assert_success("test value"),
        settlement
    );
    drop(producer);

    let mut consumer = OccupancyLedger::canonical_decode(&prior_bytes).assert_success("test value");
    assert_eq!(
        consumer
            .replay_evidence(&published_evidence, &storage, [], &mut snapshot.clone())
            .assert_success("test value"),
        settlement
    );
    consumer
        .recover_finalized_evidence(&published_evidence, &storage, [], &mut snapshot.clone())
        .assert_success("test value");
    assert_eq!(consumer.canonical_state(), published_state);
    assert_eq!(consumer.last_finalized_batch(), 2);
    for namespace in &namespaces {
        let funded = if namespace.principal_scope() == Some(accounts[0].payer()) {
            balance_a >= 40
        } else {
            balance_b >= 20
        };
        assert_eq!(
            consumer.ensure_accessible([*namespace]),
            if funded {
                Ok(())
            } else {
                Err(OccupancyError::FrozenNamespace {
                    namespace: *namespace,
                })
            }
        );
    }

    assert_occupancy_snapshot_refusals(OccupancyReplayContext {
        accounts: &accounts,
        balance_a,
        balance_b,
        prior_bytes: &prior_bytes,
        published_evidence: &published_evidence,
        storage: &storage,
        snapshot: &snapshot,
    });
    drop(consumer);
    assert_next_occupancy_recovery(
        &accounts,
        &published_state,
        &storage,
        namespaces,
        expected_paid,
    );
}
#[derive(Clone, Copy)]
struct OccupancyReplayContext<'a> {
    accounts: &'a [OccupancyPaymentAccount; 2],
    balance_a: u128,
    balance_b: u128,
    prior_bytes: &'a [u8],
    published_evidence: &'a [u8],
    storage: &'a Storage,
    snapshot: &'a CommittedAccounts,
}

fn assert_occupancy_snapshot_refusals(context: OccupancyReplayContext<'_>) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let OccupancyReplayContext {
        accounts,
        balance_a,
        balance_b,
        prior_bytes,
        published_evidence,
        storage,
        snapshot,
    } = context;
    // Another real producer emits internally valid bytes with the opposite payer
    // outcome. The original authenticated snapshot must reject those bytes.
    let alternate_snapshot = CommittedAccounts::new(
        2,
        &[
            (accounts[0], if balance_a >= 40 { 0 } else { 40 }),
            (accounts[1], balance_b),
        ],
    );
    let mut alternate_producer =
        OccupancyLedger::canonical_decode(prior_bytes).assert_success("test value");
    let alternate = alternate_producer
        .finalize_unchanged_batch(2, schedule(), &mut alternate_snapshot.clone())
        .assert_success("test value");
    let (_, alternate_evidence) = alternate_producer
        .canonical_publication(&alternate)
        .assert_success("test value");
    assert_eq!(
        OccupancySettlement::canonical_decode(&alternate_evidence).assert_success("test value"),
        alternate
    );
    let mut refused = OccupancyLedger::canonical_decode(prior_bytes).assert_success("test value");
    assert_eq!(
        refused.recover_finalized_evidence(&alternate_evidence, storage, [], &mut snapshot.clone()),
        Err(OccupancyError::MalformedEvidence)
    );
    assert_eq!(refused.canonical_state(), prior_bytes);
    assert_eq!(
        refused.replay_evidence(
            published_evidence,
            storage,
            [],
            &mut alternate_snapshot.clone()
        ),
        Err(OccupancyError::MalformedEvidence)
    );
    let mut missing_account = snapshot.clone();
    missing_account.accounts.remove(&accounts[0].payer());
    assert_eq!(
        refused.recover_finalized_evidence(published_evidence, storage, [], &mut missing_account),
        Err(OccupancyError::UnprovenPaymentAccount)
    );
    assert_eq!(refused.canonical_state(), prior_bytes);
    let mut stale_snapshot = snapshot.clone();
    stale_snapshot.batch = 1;
    assert_eq!(
        refused.replay_evidence(published_evidence, storage, [], &mut stale_snapshot),
        Err(OccupancyError::UnprovenPaymentAccount)
    );
}

fn assert_next_occupancy_recovery(
    accounts: &[OccupancyPaymentAccount; 2],
    published_state: &[u8],
    storage: &Storage,
    namespaces: Vec<StorageNamespace>,
    expected_paid: u128,
) {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    // Reopen the unpaid ledger, refill the accounts, and settle the next block.
    // The 40-unit mandate can pay 20 arrears + 20 new accrual only if deferral
    // restored its unused authority. Recovery must reproduce that exact state.
    let mut next_producer =
        OccupancyLedger::canonical_decode(published_state).assert_success("test value");
    let next_snapshot = CommittedAccounts::new(3, &[(accounts[0], 1_000), (accounts[1], 1_000)]);
    let next = next_producer
        .finalize_unchanged_batch(3, schedule(), &mut next_snapshot.clone())
        .assert_success("test value");
    assert!(next
        .charges()
        .iter()
        .all(|charge| charge.paid() && charge.arrears_after() == 0));
    assert_eq!(next.usage().paid_fee_units, 120 - expected_paid);
    assert_eq!(next.usage().arrears_fee_units, 0);
    let (next_state, next_evidence) = next_producer
        .canonical_publication(&next)
        .assert_success("test value");
    let mut next_consumer =
        OccupancyLedger::canonical_decode(published_state).assert_success("test value");
    next_consumer
        .recover_finalized_evidence(&next_evidence, storage, [], &mut next_snapshot.clone())
        .assert_success("test value");
    assert_eq!(next_consumer.canonical_state(), next_state);
    assert_eq!(next_consumer.last_finalized_batch(), 3);
    next_consumer
        .ensure_accessible(namespaces)
        .assert_success("test value");
}

#[test]
fn occupancy_unpaid_replay_paid() {
    prove_replay(40, 20);
}

#[test]
fn occupancy_unpaid_replay_unpaid() {
    prove_replay(0, 0);
}

#[test]
fn occupancy_unpaid_replay_mixed_payer() {
    // A can afford either of its namespaces individually, but not their aggregate.
    prove_replay(20, 20);
}

#[test]
fn occupancy_unpaid_replay_deleted_namespace() {
    use layerx_programs_runtime::test_support::AssertSuccess as _;

    let accounts = [
        OccupancyPaymentAccount::main(b"did:layerx:occupancy-a", ASSET)
            .assert_success("test value"),
        OccupancyPaymentAccount::main(b"did:layerx:occupancy-b", ASSET)
            .assert_success("test value"),
    ];
    let (mut storage, namespaces, prior_bytes) = initialized(&accounts);
    let deleted = namespaces[0];
    let mut transaction = storage.transaction(deleted);
    transaction.delete(b"k").assert_success("test value");
    assert_eq!(transaction.commit(), 1);
    let snapshot = CommittedAccounts::new(2, &[(accounts[0], 0), (accounts[1], 20)]);
    let mut producer = OccupancyLedger::canonical_decode(&prior_bytes).assert_success("test value");
    let settlement = producer
        .settle_batch(2, &storage, [], schedule(), &mut snapshot.clone())
        .assert_success("test value");
    let deleted_charge = settlement
        .charges()
        .iter()
        .find(|charge| charge.namespace() == deleted)
        .assert_success("test value");
    assert_eq!(deleted_charge.final_bytes(), 0);
    assert_eq!(
        deleted_charge.disposition(),
        OccupancyDisposition::InsufficientFunds
    );
    assert_eq!(deleted_charge.arrears_after(), 20);
    let (persisted, evidence) = producer
        .canonical_publication(&settlement)
        .assert_success("test value");
    drop(producer);
    let verifier = OccupancyLedger::canonical_decode(&prior_bytes).assert_success("test value");
    assert_eq!(
        verifier
            .replay_evidence(&evidence, &storage, [], &mut snapshot.clone())
            .assert_success("test value"),
        settlement
    );
    let mut reopened = OccupancyLedger::canonical_decode(&persisted).assert_success("test value");
    assert_eq!(reopened.recorded_bytes(deleted), Some(0));
    assert_eq!(
        reopened.ensure_accessible([deleted]),
        Err(OccupancyError::FrozenNamespace { namespace: deleted })
    );
    let finalized = reopened
        .finalize_unchanged_batch(2, schedule(), &mut snapshot.clone())
        .assert_success("test value");
    let (finalized_state, finalized_evidence) = reopened
        .canonical_publication(&finalized)
        .assert_success("test value");
    assert_eq!(
        OccupancySettlement::canonical_decode(&finalized_evidence).assert_success("test value"),
        finalized
    );
    drop(reopened);
    let funded = CommittedAccounts::new(3, &[(accounts[0], 1_000), (accounts[1], 1_000)]);
    let mut next = OccupancyLedger::canonical_decode(&finalized_state).assert_success("test value");
    let paid = next
        .finalize_unchanged_batch(3, schedule(), &mut funded.clone())
        .assert_success("test value");
    assert!(paid.charges().iter().all(|charge| charge.paid()));
    assert!(!next.contains_namespace(deleted));
    let (paid_state, paid_evidence) = next
        .canonical_publication(&paid)
        .assert_success("test value");
    let mut recovered =
        OccupancyLedger::canonical_decode(&finalized_state).assert_success("test value");
    recovered
        .recover_finalized_evidence(&paid_evidence, &storage, [], &mut funded.clone())
        .assert_success("test value");
    assert_eq!(recovered.canonical_state(), paid_state);
    assert!(!recovered.contains_namespace(deleted));
}
