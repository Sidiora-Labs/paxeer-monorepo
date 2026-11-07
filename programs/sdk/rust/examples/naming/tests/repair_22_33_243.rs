//! Task 22.33.243, finding 011: `reclaim_a_preserves_reverse_b_after_restart`
//! and `transfer_a_preserves_reverse_b_after_restart` prove closure using the
//! production handlers, SDK request/record/label codecs and an in-memory host.
//! `cleanup_removes_only_the_affected_reverse_name` covers matching and absent
//! entries; `malformed_reverse_refuses_without_committing` covers corrupt state.
//! Baseline UNRUN: the test target did not exist on the base, and the bounded
//! implementation-first gate order forbids a preliminary verification run.
//! Wiring request (domain 13): refresh the naming WASM fixture at
//! programs/crates/layerx-programs-registry/tests/fixtures/naming/naming.wasm
//! and qualify it through the native chain host; generated artifacts are outside scope.

use std::collections::BTreeMap;

#[path = "../src/lib.rs"]
mod naming;

use layerx_program_sdk::{
    naming::{
        decode_label, occupancy_expiry, Name, Record, Request, DID_KEY_PREFIX, REFERENCE_ASSET,
        REFERENCE_OCCUPANCY_SEED, REFERENCE_PERIOD_BATCHES, REFERENCE_PERIOD_PRICE,
    },
    payments::PreparedProgramAccount,
    storage::{shared::SharedStorageKey, StorageValue},
    Amount, AssetId, CallInput, CallResult, Field, Principal, ProgramDeposit, ProgramError,
    ProgramId, Reason,
};
use naming::host::Host;

type Result<T> = std::result::Result<T, ProgramError>;

fn principal(byte: u8) -> Principal {
    Principal::new([byte; 32]).unwrap_or_else(|error| panic!("{error}"))
}

fn program() -> ProgramId {
    ProgramId::new([0x55; 32]).unwrap_or_else(|error| panic!("{error}"))
}

fn label(bytes: &[u8]) -> Name<'_> {
    Name::new(bytes).unwrap_or_else(|error| panic!("{error}"))
}

fn expiry(height: u64, periods: u32) -> u64 {
    occupancy_expiry(height, periods).unwrap_or_else(|error| panic!("{error}"))
}

fn malformed() -> ProgramError {
    ProgramError::value(Field::CallInput, Reason::Malformed)
}

#[derive(Clone, Debug, Eq, PartialEq)]
struct Store {
    shared: BTreeMap<Vec<u8>, Vec<u8>>,
    balances: BTreeMap<[u8; 32], Amount>,
    occupancy: Amount,
}

impl Store {
    fn new() -> Self {
        Self {
            shared: BTreeMap::new(),
            balances: [principal(1), principal(2)]
                .map(|did| (did.bytes(), Amount::from_u128(100_000)))
                .into_iter()
                .collect(),
            occupancy: Amount::from_u128(0),
        }
    }
}

struct NativeHost {
    committed: Store,
    caller: Principal,
    height: u64,
    parent: Option<ProgramId>,
    response: Option<Vec<u8>>,
}

impl NativeHost {
    fn open(committed: Store) -> Self {
        Self {
            committed,
            caller: principal(1),
            height: 1,
            parent: None,
            response: None,
        }
    }

    fn stop(self) -> Store {
        self.committed
    }

    fn invoke(&mut self, caller: Principal, height: u64, request: Request<'_>) -> Result<Vec<u8>> {
        let encoded = request.encode()?;
        let input = CallInput::new(encoded.as_slice())?;
        self.caller = caller;
        self.height = height;
        self.response = None;
        let before = self.committed.clone();
        let result = match request.method() {
            "register" => naming::register_impl(self, input.bytes()),
            "transfer" => naming::transfer_impl(self, input.bytes()),
            "renew" => naming::renew_impl(self, input.bytes()),
            "resolve" => naming::resolve_impl(self, input.bytes()),
            "reverse_resolve" => naming::reverse_resolve_impl(self, input.bytes()),
            method => panic!("unexpected SDK method {method}"),
        };
        if let Err(error) = result {
            self.committed = before;
            assert!(self.response.is_none());
            return Err(error);
        }
        assert_eq!(result, Ok(CallResult::OK));
        let response = self.response.take().ok_or_else(malformed)?;
        assert_eq!(response.get(..2), Some([1, 0x20].as_slice()));
        let length = u32::from_be_bytes(
            response[2..6]
                .try_into()
                .unwrap_or_else(|error| panic!("{error}")),
        );
        assert_eq!(usize::try_from(length), Ok(response.len() - 6));
        Ok(response[6..].to_vec())
    }

    fn register(&mut self, did: Principal, height: u64, name: Name<'_>, periods: u32) {
        assert_eq!(
            self.invoke(did, height, Request::Register { name, did, periods }),
            Ok(Vec::new())
        );
    }

    fn assert_record(&mut self, name: Name<'_>, did: Principal, expiry: u64, height: u64) {
        let bytes = self
            .invoke(principal(1), height, Request::Resolve { name })
            .unwrap_or_else(|error| panic!("resolve: {error}"));
        assert_eq!(Record::decode(&bytes), Ok(Record { did, expiry }));
    }

    fn assert_reverse(&mut self, did: Principal, name: Name<'_>, height: u64) {
        let bytes = self
            .invoke(principal(1), height, Request::ReverseResolve { did })
            .unwrap_or_else(|error| panic!("reverse resolve: {error}"));
        assert_eq!(decode_label(&bytes), Ok(name));
    }
}

impl Host for NativeHost {
    fn immediate_caller(&mut self) -> Result<Option<ProgramId>> {
        Ok(self.parent)
    }

    fn invoking_principal(&mut self) -> Result<Principal> {
        Ok(self.caller)
    }

    fn executing_program(&mut self) -> Result<ProgramId> {
        Ok(program())
    }

    fn batch_height(&mut self) -> Result<u64> {
        Ok(self.height)
    }

    fn read(&mut self, key: SharedStorageKey<'_>, output: &mut [u8]) -> Result<Option<usize>> {
        let Some(value) = self.committed.shared.get(key.bytes()) else {
            return Ok(None);
        };
        let destination = output.get_mut(..value.len()).ok_or_else(malformed)?;
        destination.copy_from_slice(value);
        Ok(Some(value.len()))
    }

    fn write(&mut self, key: SharedStorageKey<'_>, value: StorageValue<'_>) -> Result<()> {
        self.committed
            .shared
            .insert(key.bytes().to_vec(), value.bytes().to_vec());
        Ok(())
    }

    fn delete(&mut self, key: SharedStorageKey<'_>) -> Result<()> {
        self.committed.shared.remove(key.bytes());
        Ok(())
    }

    fn fund_program_account(&mut self, deposit: ProgramDeposit<'_>) -> Result<()> {
        let account = PreparedProgramAccount::new(
            program(),
            REFERENCE_OCCUPANCY_SEED,
            AssetId::new(REFERENCE_ASSET)?,
        )?;
        assert_eq!(deposit.seed(), account.seed());
        assert_eq!(deposit.destination(), account.account());
        assert_eq!(deposit.asset(), account.asset());
        let balance = self
            .committed
            .balances
            .get_mut(&self.caller.bytes())
            .ok_or_else(malformed)?;
        *balance = balance.checked_sub(deposit.amount())?;
        self.committed.occupancy = self.committed.occupancy.checked_add(deposit.amount())?;
        Ok(())
    }

    fn publish_response(&mut self, result: CallResult, bytes: &[u8]) -> Result<()> {
        assert_eq!(result, CallResult::OK);
        assert!(self.response.replace(bytes.to_vec()).is_none());
        Ok(())
    }
}

#[test]
fn reclaim_a_preserves_reverse_b_after_restart() {
    let mut host = NativeHost::open(Store::new());
    let (owner, recipient) = (principal(1), principal(2));
    let (a, b) = (label(b"alpha"), label(b"beta"));
    host.register(owner, 1, a, 1);
    host.register(owner, 2, b, 2);
    host.assert_reverse(owner, b, 2);
    let reclaimed_at = 1 + REFERENCE_PERIOD_BATCHES;
    host.register(recipient, reclaimed_at, a, 1);
    host.assert_reverse(owner, b, reclaimed_at);
    let committed = host.stop();
    let mut reopened = NativeHost::open(committed);
    reopened.assert_record(a, recipient, expiry(reclaimed_at, 1), reclaimed_at);
    reopened.assert_record(b, owner, expiry(2, 2), reclaimed_at);
    reopened.assert_reverse(owner, b, reclaimed_at);
    reopened.assert_reverse(recipient, a, reclaimed_at);
    assert_eq!(
        reopened.committed.occupancy,
        Amount::from_u128(4 * REFERENCE_PERIOD_PRICE)
    );
}

#[test]
fn transfer_a_preserves_reverse_b_after_restart() {
    let mut host = NativeHost::open(Store::new());
    let (owner, recipient) = (principal(1), principal(2));
    let (a, b) = (label(b"alpha"), label(b"beta"));
    host.register(owner, 1, a, 1);
    host.register(owner, 2, b, 2);
    host.assert_reverse(owner, b, 2);
    assert_eq!(
        host.invoke(
            owner,
            3,
            Request::Transfer {
                name: a,
                did: recipient
            }
        ),
        Ok(Vec::new())
    );
    host.assert_reverse(owner, b, 3);
    let committed = host.stop();
    let mut reopened = NativeHost::open(committed);
    reopened.assert_record(a, recipient, expiry(1, 1), 3);
    reopened.assert_record(b, owner, expiry(2, 2), 3);
    reopened.assert_reverse(owner, b, 3);
    reopened.assert_reverse(recipient, a, 3);
    assert_eq!(
        reopened.committed.occupancy,
        Amount::from_u128(3 * REFERENCE_PERIOD_PRICE)
    );
}

#[test]
fn cleanup_removes_only_the_affected_reverse_name() {
    for reclaim in [false, true] {
        let mut host = NativeHost::open(Store::new());
        let (owner, recipient) = (principal(1), principal(2));
        let (a, b) = (label(b"alpha"), label(b"beta"));
        host.register(owner, 1, a, 1);
        let height = if reclaim {
            1 + REFERENCE_PERIOD_BATCHES
        } else {
            4
        };
        if reclaim {
            host.register(recipient, height, a, 1);
        } else {
            assert_eq!(
                host.invoke(
                    owner,
                    height,
                    Request::Transfer {
                        name: a,
                        did: recipient
                    }
                ),
                Ok(Vec::new())
            );
        }
        let key = [DID_KEY_PREFIX, &owner.bytes()].concat();
        assert!(!host.committed.shared.contains_key(&key));
        assert!(host
            .invoke(owner, height, Request::ReverseResolve { did: owner })
            .is_err());
        let mut reopened = NativeHost::open(host.stop());
        reopened.assert_reverse(recipient, a, height);
        assert!(!reopened.committed.shared.contains_key(&key));

        let mut host = NativeHost::open(Store::new());
        host.register(owner, 1, a, 1);
        host.register(owner, 2, b, 2);
        assert_eq!(
            host.invoke(
                owner,
                3,
                Request::Transfer {
                    name: b,
                    did: recipient
                }
            ),
            Ok(Vec::new())
        );
        assert!(!host.committed.shared.contains_key(&key));
        if reclaim {
            host.register(recipient, height, a, 1);
        } else {
            assert_eq!(
                host.invoke(
                    owner,
                    height,
                    Request::Transfer {
                        name: a,
                        did: recipient
                    }
                ),
                Ok(Vec::new())
            );
        }
        let mut reopened = NativeHost::open(host.stop());
        reopened.assert_reverse(recipient, a, height);
        assert!(!reopened.committed.shared.contains_key(&key));
    }
}

#[test]
fn malformed_reverse_refuses_without_committing() {
    for reclaim in [false, true] {
        let mut host = NativeHost::open(Store::new());
        let (owner, recipient) = (principal(1), principal(2));
        let a = label(b"alpha");
        host.register(owner, 1, a, 1);
        host.committed
            .shared
            .insert([DID_KEY_PREFIX, &owner.bytes()].concat(), vec![3, b'x']);
        let before = host.committed.clone();
        let request = if reclaim {
            Request::Register {
                name: a,
                did: recipient,
                periods: 1,
            }
        } else {
            Request::Transfer {
                name: a,
                did: recipient,
            }
        };
        let height = if reclaim {
            1 + REFERENCE_PERIOD_BATCHES
        } else {
            4
        };
        assert!(host
            .invoke(if reclaim { recipient } else { owner }, height, request)
            .is_err());
        assert_eq!(host.committed, before);
    }
}
