use crate::occupancy::{OccupancyAuthority, OccupancyLedger};
use crate::{PrincipalId, ProgramId, Storage, StorageNamespace};
use std::collections::{BTreeMap, BTreeSet};

const NON_CANONICAL: i32 = -3;
const UNAUTHORIZED_DEBIT: i32 = -207;
const LENGTH_LIMIT: i32 = -5;
const FATAL_INVARIANT: i32 = -1001;

pub(crate) struct CallOccupancySettlement<'a> {
    pub(crate) batch: u64,
    pub(crate) authority: OccupancyAuthority,
    pub(crate) schedule: crate::FeeSchedule,
    pub(crate) initial_sizes: &'a BTreeMap<StorageNamespace, u64>,
    pub(crate) program_owners: &'a BTreeMap<ProgramId, PrincipalId>,
    pub(crate) storage: &'a Storage,
    pub(crate) ledger: &'a mut OccupancyLedger,
}

fn occupancy_responsibilities(
    eligible: Vec<(StorageNamespace, u64)>,
    authority: OccupancyAuthority,
    schedule: crate::FeeSchedule,
) -> Result<Vec<crate::occupancy::OccupancyResponsibility>, i32> {
    let mut responsibilities = Vec::new();
    let price = schedule.occupancy_byte_batch_price();
    if price == 0 {
        return Err(NON_CANONICAL);
    }
    let minimum_total = eligible
        .iter()
        .try_fold(0u128, |total, (_, bytes)| {
            total.checked_add(u128::from(*bytes).checked_mul(u128::from(price))?)
        })
        .ok_or(-500)?;
    if minimum_total > authority.fee_ceiling() {
        return Err(UNAUTHORIZED_DEBIT);
    }
    let share_count = u128::try_from(eligible.len()).map_err(|_| LENGTH_LIMIT)?;
    let surplus = authority
        .fee_ceiling()
        .checked_sub(minimum_total)
        .ok_or(-500)?;
    let equal_share = if share_count == 0 {
        0
    } else {
        surplus / share_count
    };
    let mut remainder = if share_count == 0 {
        0
    } else {
        surplus % share_count
    };
    for (namespace, bytes) in eligible {
        let minimum = u128::from(bytes)
            .checked_mul(u128::from(price))
            .ok_or(-500)?;
        let charge_ceiling = minimum
            .checked_add(equal_share)
            .and_then(|value| value.checked_add(u128::from(remainder != 0)))
            .ok_or(-500)?;
        if remainder != 0 {
            remainder = remainder.checked_sub(1).ok_or(-500)?;
        }
        let maximum_bytes = u64::try_from(charge_ceiling / u128::from(price))
            .map_err(|_| LENGTH_LIMIT)?
            .max(bytes);
        let responsibility = authority
            .authorize(namespace, maximum_bytes, charge_ceiling)
            .map_err(|_| UNAUTHORIZED_DEBIT)?;
        responsibilities.push(responsibility);
    }
    Ok(responsibilities)
}

pub(crate) fn settle_call_occupancy(
    request: CallOccupancySettlement<'_>,
) -> Result<crate::OccupancySettlement, i32> {
    let CallOccupancySettlement {
        batch,
        authority,
        schedule,
        initial_sizes,
        program_owners,
        storage,
        ledger,
    } = request;
    let sizes: BTreeMap<_, _> = storage
        .namespace_sizes()
        .map_err(|_| NON_CANONICAL)?
        .into_iter()
        .collect();
    let changed: BTreeSet<_> = initial_sizes
        .keys()
        .chain(sizes.keys())
        .copied()
        .filter(|namespace| {
            initial_sizes.get(namespace) != sizes.get(namespace)
                || (ledger.requires_migration(*namespace) && storage.was_accessed(*namespace))
        })
        .collect();
    let mut eligible = Vec::new();
    for namespace in changed {
        let final_bytes = sizes.get(&namespace).copied().unwrap_or(0);
        if final_bytes == 0 {
            continue;
        }
        if ledger.requires_migration(namespace) && namespace.program() != authority.root_program() {
            continue;
        }
        let authorized_payer = match namespace.principal_scope() {
            Some(principal) => principal,
            None => *program_owners
                .get(&namespace.program())
                .ok_or(NON_CANONICAL)?,
        };
        if let Some((payer, maximum_bytes)) = ledger.responsibility_limits(namespace) {
            if payer == authorized_payer
                && final_bytes <= maximum_bytes
                && !ledger.requires_migration(namespace)
            {
                continue;
            }
        }
        if authorized_payer != authority.payer() {
            return Err(UNAUTHORIZED_DEBIT);
        }
        match ledger.responsibility_limits(namespace) {
            Some((payer, _))
                if payer != authority.payer() && !ledger.requires_migration(namespace) =>
            {
                return Err(UNAUTHORIZED_DEBIT);
            }
            Some((_, maximum_bytes))
                if final_bytes <= maximum_bytes && !ledger.requires_migration(namespace) => {}
            _ => eligible.push((namespace, final_bytes)),
        }
    }
    let responsibilities = occupancy_responsibilities(eligible, authority, schedule)?;
    let prepared = ledger
        .prepare_batch(batch, storage, responsibilities, schedule)
        .map_err(|_| NON_CANONICAL)?;
    let settlement = prepared.settlement().clone();
    let committed = ledger
        .commit_after_debits(prepared, storage)
        .map_err(|_| NON_CANONICAL)?;
    if committed != settlement {
        return Err(FATAL_INVARIANT);
    }
    Ok(settlement)
}

#[cfg(test)]
mod occupancy_cases {
    use super::*;
    use crate::{
        ActivityBudgetBinding, DeclaredBudget, Executor, FeeSchedule, FeeScheduleParameters,
    };

    struct CallOccupancy {
        program: ProgramId,
        owner: PrincipalId,
        other: PrincipalId,
        storage: Storage,
        ledger: OccupancyLedger,
        schedule: FeeSchedule,
    }

    impl CallOccupancy {
        fn new() -> Self {
            Self {
                program: ProgramId::new([1; 32]).unwrap_or_else(|error| panic!("{error}")),
                owner: PrincipalId::new([2; 32]).unwrap_or_else(|error| panic!("{error}")),
                other: PrincipalId::new([3; 32]).unwrap_or_else(|error| panic!("{error}")),
                storage: Storage::new(),
                ledger: OccupancyLedger::activated_after(10),
                schedule: FeeSchedule::new_complete(FeeScheduleParameters {
                    version: 1,
                    fee_units_per_cpu_fuel: 1,
                    fee_units_per_memory_byte: 1,
                    fee_units_per_storage_read_byte: 1,
                    fee_units_per_storage_write_byte: 1,
                    fee_units_per_output_value: 1,
                    fee_units_per_output_byte: 1,
                    fee_units_per_occupancy_byte_batch: 1,
                }),
            }
        }

        fn namespace(&self) -> StorageNamespace {
            StorageNamespace::shared(self.program)
        }

        fn write(&mut self, key: &[u8], value: &[u8]) {
            let mut transaction = self.storage.transaction(self.namespace());
            transaction
                .write(key, value)
                .unwrap_or_else(|error| panic!("{error}"));
            transaction.commit();
        }

        fn settle(
            &mut self,
            payer: PrincipalId,
            initial_sizes: &BTreeMap<StorageNamespace, u64>,
        ) -> Result<crate::OccupancySettlement, i32> {
            let declared = DeclaredBudget::new(100_000, 65_536, 1024, 1024, 1, 1024, 0)
                .unwrap_or_else(|error| panic!("{error}"));
            let executor = Executor::new_versioned(
                crate::ResourceBudget::declared(),
                self.schedule,
                crate::RUNTIME_VERSION,
                5,
            );
            let admitted = executor
                .admit_activity_budget(
                    declared,
                    crate::budget::PayerCoverage::new(
                        payer,
                        ActivityBudgetBinding::new(payer.bytes())
                            .unwrap_or_else(|error| panic!("{error}")),
                        1_000_000,
                    ),
                )
                .unwrap_or_else(|error| panic!("{error}"));
            let authority = OccupancyAuthority::from_admitted(
                &admitted,
                admitted.maximum_fee_units() + 16,
                self.schedule,
                self.program,
            )
            .unwrap_or_else(|error| panic!("{error}"));
            settle_call_occupancy(CallOccupancySettlement {
                batch: 11,
                authority,
                schedule: self.schedule,
                initial_sizes,
                program_owners: &BTreeMap::from([(self.program, self.owner)]),
                storage: &self.storage,
                ledger: &mut self.ledger,
            })
        }

        fn open_responsibility(&mut self) -> crate::OccupancySettlement {
            self.write(b"a", b"v");
            self.settle(self.owner, &BTreeMap::new())
                .unwrap_or_else(|code| panic!("{code}"))
        }

        fn sizes(&self) -> BTreeMap<StorageNamespace, u64> {
            self.storage
                .namespace_sizes()
                .unwrap_or_else(|error| panic!("{error}"))
                .into_iter()
                .collect()
        }
    }

    #[test]
    fn occupancy_call_by_another_payer_within_owner_responsibility_commits() {
        let mut call = CallOccupancy::new();
        let opened = call.open_responsibility();
        let initial_sizes = call.sizes();
        let namespace = call.namespace();
        let limits = call.ledger.responsibility_limits(namespace);
        assert_eq!(limits, Some((call.owner, 16)));
        call.write(b"b", &[7; 13]);
        assert_eq!(call.sizes()[&namespace], 16);
        let ledger_before = call.ledger.clone();
        let storage_before = call.storage.clone();
        let settlement = call
            .settle(call.other, &initial_sizes)
            .unwrap_or_else(|code| panic!("{code}"));
        assert_eq!(call.ledger.responsibility_limits(namespace), limits);
        assert_eq!(settlement.charges().len(), 1);
        let charge = settlement.charges()[0];
        assert_eq!(charge.payer(), call.owner);
        assert_eq!(
            charge.activity_binding(),
            opened.charges()[0].activity_binding()
        );
        assert_eq!(charge.recorded_bytes(), 2);
        assert_eq!(charge.final_bytes(), 16);
        assert!(charge.paid());
        assert_eq!(call.storage, storage_before);
        let committed = call.ledger.clone();
        call.ledger = ledger_before;
        assert_eq!(call.settle(call.owner, &initial_sizes), Ok(settlement));
        assert_eq!(call.ledger, committed);
    }

    #[test]
    fn occupancy_call_by_another_payer_past_owner_byte_limit_is_unauthorized() {
        let mut call = CallOccupancy::new();
        call.open_responsibility();
        let initial_sizes = call.sizes();
        call.write(b"b", &[7; 14]);
        assert_eq!(call.sizes()[&call.namespace()], 17);
        let ledger_before = call.ledger.clone();
        let storage_before = call.storage.clone();
        assert_eq!(
            call.settle(call.other, &initial_sizes),
            Err(UNAUTHORIZED_DEBIT)
        );
        assert_eq!(call.ledger, ledger_before);
        assert_eq!(call.storage, storage_before);
    }

    #[test]
    fn occupancy_call_by_another_payer_without_responsibility_is_unauthorized() {
        let mut call = CallOccupancy::new();
        call.write(b"a", b"v");
        let ledger_before = call.ledger.clone();
        assert_eq!(
            call.settle(call.other, &BTreeMap::new()),
            Err(UNAUTHORIZED_DEBIT)
        );
        assert_eq!(call.ledger, ledger_before);
    }

    #[test]
    fn occupancy_call_by_another_payer_cannot_use_a_migration_position() {
        let mut call = CallOccupancy::new();
        call.write(b"a", b"vvv");
        call.ledger
            .import_activation_positions(
                &call.storage,
                &BTreeMap::from([(call.program, call.owner)]),
            )
            .unwrap_or_else(|error| panic!("{error}"));
        let initial_sizes = call.sizes();
        call.write(b"a", b"v");
        let ledger_before = call.ledger.clone();
        assert_eq!(
            call.settle(call.other, &initial_sizes),
            Err(UNAUTHORIZED_DEBIT)
        );
        assert_eq!(call.ledger, ledger_before);
    }

    #[test]
    fn occupancy_call_by_another_payer_requires_the_authorized_standing_payer() {
        let mut call = CallOccupancy::new();
        call.open_responsibility();
        let initial_sizes = call.sizes();
        call.owner = PrincipalId::new([4; 32]).unwrap_or_else(|error| panic!("{error}"));
        call.write(b"b", b"v");
        let ledger_before = call.ledger.clone();
        assert_eq!(
            call.settle(call.other, &initial_sizes),
            Err(UNAUTHORIZED_DEBIT)
        );
        assert_eq!(call.ledger, ledger_before);
    }
}
