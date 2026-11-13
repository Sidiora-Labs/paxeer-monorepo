use layerx_programs::state_record::{ProtocolAdapterError, ProtocolProgramStateRead};
use layerx_programs::{
    account_tree_commitment, programs_root_commitment, state_node_commitment,
    universal_root_commitment, AccountStateError, AccountStateHead, AccountStateJournal,
    CanonicalAccountLeaf, DeploymentRecord, Deprecation, DeprecationRequest, ExitRoute,
    JournalAccountStateAuthority, LifecycleReceipt, ProgramLifecycle, ProgramValueAccountBinding,
    ProvenAccountLeaf, ProvenProgramBinding, ReadFreshness, Registry, RegistryEntry, StateProof,
    VerifiedAccountSnapshot, WindDownPolicy, WindDownStateAccess,
};
use layerx_programs_runtime::{hash_bytes, HashAlgorithm, ProgramId, UpgradePolicy};
use std::collections::BTreeMap;

const VECTORS: &str = include_str!("../../../../tests/vectors/program_account_state_v2.vec");
const NOW: u64 = 1_000;
const LIMIT: u64 = 10;

fn vector(name: &str) -> &'static str {
    VECTORS
        .lines()
        .find_map(|line| line.strip_prefix(name)?.strip_prefix('='))
        .unwrap_or_else(|| panic!("missing vector {name}"))
}

fn vector_bytes(name: &str) -> Vec<u8> {
    let text = vector(name);
    assert_eq!(text.len() % 2, 0);
    (0..text.len())
        .step_by(2)
        .map(|index| {
            u8::from_str_radix(&text[index..index + 2], 16)
                .unwrap_or_else(|error| panic!("{error:?}"))
        })
        .collect()
}

fn vector_hash(name: &str) -> [u8; 32] {
    vector_bytes(name)
        .try_into()
        .unwrap_or_else(|error| panic!("{error:?}"))
}

fn singleton_proof() -> StateProof {
    StateProof {
        leaf_index: 0,
        leaf_count: 1,
        siblings: Vec::new(),
    }
}

#[derive(Clone)]
struct ReceiptStore {
    heads: BTreeMap<[u8; 32], AccountStateHead>,
    current: [u8; 32],
}

impl AccountStateJournal for ReceiptStore {
    fn account_state_head(
        &self,
        receipt_digest: [u8; 32],
    ) -> Result<AccountStateHead, AccountStateError> {
        self.heads
            .get(&receipt_digest)
            .copied()
            .ok_or(AccountStateError::UnverifiedReceipt)
    }

    fn current_account_state_head(&self) -> Result<AccountStateHead, AccountStateError> {
        self.account_state_head(self.current)
    }
}

fn complete_registry(registry: &Registry) -> Vec<RegistryEntry> {
    registry
        .program_ids()
        .into_iter()
        .map(|program| {
            registry
                .entry_for_wind_down(program)
                .unwrap_or_else(|error| panic!("{error:?}"))
                .clone()
        })
        .collect()
}

struct Fixture {
    baseline: Registry,
    expected: Registry,
    program: ProgramId,
    snapshot: VerifiedAccountSnapshot,
    binding: ProgramValueAccountBinding,
    routes: Vec<ExitRoute>,
    history: Vec<LifecycleReceipt>,
    head: AccountStateHead,
    state: ProtocolProgramStateRead,
    encoded: Vec<u8>,
}

fn fixture_binding() -> ProgramValueAccountBinding {
    let program = ProgramId::new(vector_hash("registration_program_id"))
        .unwrap_or_else(|error| panic!("{error:?}"));
    let binding = ProgramValueAccountBinding {
        record_version: 2,
        program,
        seed: vector_bytes("registration_seed"),
        account_id: vector_hash("registration_account_id"),
        asset_id: vector_hash("registration_asset_id"),
        registered_sequence: vector("registration_registered_sequence")
            .parse()
            .unwrap_or_else(|error| panic!("{error:?}")),
        registration_event_digest: vector_hash("registration_event_digest"),
    };
    assert_eq!(
        binding
            .primary_value()
            .unwrap_or_else(|error| panic!("{error:?}")),
        vector_bytes("registration_primary_value")
    );
    assert_eq!(
        binding
            .primary_commitment()
            .unwrap_or_else(|error| panic!("{error:?}")),
        vector_hash("registration_primary_commitment")
    );

    binding
}

fn baseline_registry(program: ProgramId) -> Registry {
    let module = vec![0, 97, 115, 109, 1, 0, 0, 0];
    let code_hash =
        hash_bytes(HashAlgorithm::Sha256, &module).unwrap_or_else(|error| panic!("{error:?}"));
    let other_program = ProgramId::new([2; 32]).unwrap_or_else(|error| panic!("{error:?}"));
    let mut baseline = Registry::new();
    let deployments: Vec<_> = [program, other_program]
        .into_iter()
        .map(|program| DeploymentRecord {
            program,
            version: 1,
            abi_version: 2,
            upgrade_policy: UpgradePolicy::Authority([4; 32]),
            old_code_hash: None,
            new_code_hash: code_hash,
            sequence: 1,
            observed_at: 1,
            module: module.clone(),
            migration: None,
        })
        .collect();
    baseline
        .replay_journal(&deployments)
        .unwrap_or_else(|error| panic!("{error:?}"));

    baseline
}

fn fixture_snapshot(
    binding: &ProgramValueAccountBinding,
) -> (VerifiedAccountSnapshot, AccountStateHead) {
    let account_id_hex = layerx_programs::hex::encode(&binding.account_id);
    let leaf = CanonicalAccountLeaf {
        account_id: binding.account_id,
        name: format!("module:programs:value:{account_id_hex}").into_bytes(),
        kind: 13,
        balance: 500,
        asset_id: binding.asset_id,
        has_asset: true.into(),
        next_sequence: 0,
        created_at_sequence: binding.registered_sequence,
        frozen: false.into(),
        has_open_reference: false.into(),
        authority_key: [0; 32],
        has_authority_key: false.into(),
    };
    let account_root = leaf
        .commitment()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let programs_root = binding
        .primary_commitment()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let universal_root = account_tree_commitment(account_root);
    let universal_leaf = universal_root_commitment(universal_root);
    let programs_leaf = programs_root_commitment(programs_root);
    let state_root = state_node_commitment(universal_leaf, programs_leaf);
    let snapshot = VerifiedAccountSnapshot {
        protocol_version: 2,
        receipt_digest: [9; 32],
        state_root,
        universal_root,
        programs_root,
        account_root,
        account_tree_proof: singleton_proof(),
        universal_root_proof: StateProof {
            leaf_index: 0,
            leaf_count: 2,
            siblings: vec![programs_leaf],
        },
        programs_root_proof: StateProof {
            leaf_index: 1,
            leaf_count: 2,
            siblings: vec![universal_leaf],
        },
        freshness: ReadFreshness {
            observed_sequence: 99,
            observed_at: NOW,
        },
        bindings: vec![ProvenProgramBinding {
            binding: binding.clone(),
            proof: singleton_proof(),
        }],
        accounts: vec![ProvenAccountLeaf {
            leaf,
            proof: singleton_proof(),
        }],
    };
    let head = AccountStateHead {
        receipt_digest: snapshot.receipt_digest,
        state_root,
        freshness: snapshot.freshness,
    };
    (snapshot, head)
}

impl Fixture {
    fn new() -> Self {
        let binding = fixture_binding();
        let program = binding.program;
        let baseline = baseline_registry(program);
        let (snapshot, head) = fixture_snapshot(&binding);
        let authority = JournalAccountStateAuthority::new(
            ReceiptStore {
                heads: BTreeMap::from([(head.receipt_digest, head)]),
                current: head.receipt_digest,
            },
            NOW,
            LIMIT,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        snapshot
            .verify(&authority)
            .unwrap_or_else(|error| panic!("{error:?}"));

        let routes = vec![ExitRoute {
            seed: binding.seed.clone(),
            account_id: binding.account_id,
            asset_id: binding.asset_id,
            destination: [0x80; 32],
        }];
        let mut producer = baseline.clone();
        producer
            .record_value_account(binding.clone())
            .unwrap_or_else(|error| panic!("{error:?}"));
        let receipt = Deprecation::new()
            .transition(
                &mut producer,
                &DeprecationRequest {
                    program,
                    expected: ProgramLifecycle::Active,
                    target: ProgramLifecycle::Deprecated,
                    authority: [4; 32],
                    effective_sequence: 100,
                    wind_down: WindDownPolicy {
                        exit_program: program.bytes(),
                        deadline: 500,
                        state_access: WindDownStateAccess::ReadOnly,
                    },
                    exits: routes.clone(),
                    account_snapshot: snapshot.clone(),
                },
                &authority,
            )
            .unwrap_or_else(|error| panic!("{error:?}"));
        let history = vec![receipt];
        let mut expected = baseline.clone();
        let state = ProtocolProgramStateRead::verify_snapshot(
            &mut expected,
            program,
            snapshot.clone(),
            vec![binding.clone()],
            ProgramLifecycle::Deprecated,
            routes.clone(),
            history.clone(),
            head,
            NOW,
            LIMIT,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        let encoded = state
            .canonical_encode()
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_ne!(complete_registry(&expected), complete_registry(&baseline));
        let entry = expected
            .entry_for_wind_down(program)
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(entry.value_accounts, vec![binding.clone()]);
        assert_eq!(entry.lifecycle, ProgramLifecycle::Deprecated);
        assert_eq!(entry.lifecycle_history, history);
        assert_eq!(entry.exit_routes, routes);
        Self {
            baseline,
            expected,
            program,
            snapshot,
            binding,
            routes,
            history,
            head,
            state,
            encoded,
        }
    }

    fn verify(
        &self,
        registry: &mut Registry,
        snapshot: VerifiedAccountSnapshot,
        head: AccountStateHead,
        now: u64,
        limit: u64,
    ) -> Result<ProtocolProgramStateRead, ProtocolAdapterError> {
        ProtocolProgramStateRead::verify_snapshot(
            registry,
            self.program,
            snapshot,
            vec![self.binding.clone()],
            ProgramLifecycle::Deprecated,
            self.routes.clone(),
            self.history.clone(),
            head,
            now,
            limit,
        )
    }

    fn assert_verify_refusal(
        &self,
        snapshot: VerifiedAccountSnapshot,
        head: AccountStateHead,
        now: u64,
        limit: u64,
        error: AccountStateError,
    ) {
        let mut registry = self.baseline.clone();
        let before = complete_registry(&registry);
        assert_eq!(
            self.verify(&mut registry, snapshot, head, now, limit),
            Err(ProtocolAdapterError::AccountState(error))
        );
        assert_eq!(complete_registry(&registry), before);
        assert_eq!(
            self.verify(&mut registry, self.snapshot.clone(), self.head, NOW, LIMIT),
            Ok(self.state.clone())
        );
        assert_eq!(
            complete_registry(&registry),
            complete_registry(&self.expected)
        );
    }

    fn assert_restore_refusal(
        &self,
        bytes: &[u8],
        verified: AccountStateHead,
        current: AccountStateHead,
        now: u64,
        limit: u64,
        error: ProtocolAdapterError,
    ) {
        let mut registry = self.baseline.clone();
        let before = complete_registry(&registry);
        assert_eq!(
            ProtocolProgramStateRead::restore_verified(
                bytes,
                &mut registry,
                verified,
                current,
                now,
                limit,
            ),
            Err(error)
        );
        assert_eq!(complete_registry(&registry), before);
        assert_eq!(
            ProtocolProgramStateRead::restore_verified(
                &self.encoded,
                &mut registry,
                self.head,
                self.head,
                NOW,
                LIMIT,
            ),
            Ok(self.state.clone())
        );
        assert_eq!(
            complete_registry(&registry),
            complete_registry(&self.expected)
        );
    }
}

#[test]
fn registry_failed_read_mutation_verify_staleness() {
    let fixture = Fixture::new();
    for now in [NOW + LIMIT + 1, NOW - 1] {
        fixture.assert_verify_refusal(
            fixture.snapshot.clone(),
            fixture.head,
            now,
            LIMIT,
            AccountStateError::StaleRead,
        );
    }
}

#[test]
fn registry_failed_read_mutation_verify_authority() {
    let fixture = Fixture::new();
    let mut receipt = fixture.head;
    receipt.receipt_digest[0] ^= 1;
    let mut root = fixture.head;
    root.state_root[0] ^= 1;
    let mut freshness = fixture.head;
    freshness.freshness.observed_sequence += 1;
    for head in [receipt, root, freshness] {
        fixture.assert_verify_refusal(
            fixture.snapshot.clone(),
            head,
            NOW,
            LIMIT,
            AccountStateError::UnverifiedReceipt,
        );
    }
    for (now, limit) in [(0, LIMIT), (NOW, 0)] {
        fixture.assert_verify_refusal(
            fixture.snapshot.clone(),
            fixture.head,
            now,
            limit,
            AccountStateError::InvalidFreshness,
        );
    }
}

#[test]
fn registry_failed_read_mutation_verify_account_proof() {
    let fixture = Fixture::new();
    let mut snapshot = fixture.snapshot.clone();
    snapshot.accounts[0].leaf.balance += 1;
    fixture.assert_verify_refusal(
        snapshot,
        fixture.head,
        NOW,
        LIMIT,
        AccountStateError::AccountRootMismatch,
    );
}

#[test]
fn registry_failed_read_mutation_restore_staleness() {
    let fixture = Fixture::new();
    for now in [NOW + LIMIT + 1, NOW - 1] {
        fixture.assert_restore_refusal(
            &fixture.encoded,
            fixture.head,
            fixture.head,
            now,
            LIMIT,
            ProtocolAdapterError::AccountState(AccountStateError::StaleRead),
        );
    }
    let mut current = fixture.head;
    current.freshness.observed_sequence += 1;
    fixture.assert_restore_refusal(
        &fixture.encoded,
        fixture.head,
        current,
        NOW,
        LIMIT,
        ProtocolAdapterError::AccountState(AccountStateError::StaleRead),
    );
}

#[test]
fn registry_failed_read_mutation_restore_authority() {
    let fixture = Fixture::new();
    let mut receipt = fixture.head;
    receipt.receipt_digest[0] ^= 1;
    let mut root = fixture.head;
    root.state_root[0] ^= 1;
    let mut freshness = fixture.head;
    freshness.freshness.observed_sequence += 1;
    for head in [receipt, root, freshness] {
        fixture.assert_restore_refusal(
            &fixture.encoded,
            head,
            head,
            NOW,
            LIMIT,
            ProtocolAdapterError::AccountState(AccountStateError::UnverifiedReceipt),
        );
    }
    for (now, limit) in [(0, LIMIT), (NOW, 0)] {
        fixture.assert_restore_refusal(
            &fixture.encoded,
            fixture.head,
            fixture.head,
            now,
            limit,
            ProtocolAdapterError::CorruptRecord,
        );
    }
}

#[test]
fn registry_failed_read_mutation_restore_account_proof() {
    let fixture = Fixture::new();
    let name = &fixture.snapshot.accounts[0].leaf.name;
    let positions: Vec<_> = fixture
        .encoded
        .windows(name.len())
        .enumerate()
        .filter_map(|(index, bytes)| (bytes == name).then_some(index))
        .collect();
    assert_eq!(positions.len(), 1);
    let balance_end = positions[0] + name.len() + 1 + 16;
    let mut corrupted = fixture.encoded.clone();
    corrupted[balance_end - 1] ^= 1;
    fixture.assert_restore_refusal(
        &corrupted,
        fixture.head,
        fixture.head,
        NOW,
        LIMIT,
        ProtocolAdapterError::AccountState(AccountStateError::AccountRootMismatch),
    );
}

#[test]
fn registry_failed_read_mutation_valid_reads_publish_complete_state() {
    let fixture = Fixture::new();
    let mut registry = fixture.baseline.clone();
    let verified = fixture
        .verify(
            &mut registry,
            fixture.snapshot.clone(),
            fixture.head,
            NOW + LIMIT,
            LIMIT,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        complete_registry(&registry),
        complete_registry(&fixture.expected)
    );
    assert_eq!(verified.balances().value_accounts()[0].balance, 500);
    assert_eq!(
        verified.balances().bindings(),
        std::slice::from_ref(&fixture.binding)
    );
    assert_eq!(verified.routes(), fixture.routes);
    assert_eq!(verified.history(), fixture.history);
    let encoded = verified
        .canonical_encode()
        .unwrap_or_else(|error| panic!("{error:?}"));
    drop(verified);
    drop(registry);
    let mut reopened = fixture.baseline.clone();
    let restored = ProtocolProgramStateRead::restore_verified(
        &encoded,
        &mut reopened,
        fixture.head,
        fixture.head,
        NOW + LIMIT,
        LIMIT,
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(restored, fixture.state);
    assert_eq!(
        restored
            .canonical_encode()
            .unwrap_or_else(|error| panic!("{error:?}")),
        encoded
    );
    assert_eq!(
        complete_registry(&reopened),
        complete_registry(&fixture.expected)
    );
}
