#[allow(dead_code)]
#[path = "support/chain.rs"]
mod chain;

use chain::ChainFixture;
use layerx_programs::{
    chain_evidence::programs_state_key,
    chain_proof::{decode::decode_commitment_proof, VerifiedChainHead},
    program_account_registration_commitment, AccountStateError, CanonicalAccountLeaf,
    ChainAccountStateAuthority, ChainProgramAccountProof, DeploymentRecord, Deprecation,
    DeprecationRefusal, ExitRoute, LifecycleTransition, ProgramId, ProgramLifecycle,
    ProgramValueAccountBinding, Registry, RegistryError, UpgradePolicy, WindDownPolicy,
    WindDownStateAccess,
};
use layerx_programs_runtime::{
    derive_program_account, hash_bytes, AuthorizationContext, AuthorizedExecutionRequest,
    Capability, CapabilitySet, CompositionContext, Deploy, Executor, HashAlgorithm, Lifecycle,
    PrincipalId, ProgramVersion, Storage, StorageNamespace, UnavailableReceiptOracle, Upgrade,
    WasmEngine,
};
use std::{
    fs,
    path::PathBuf,
    sync::atomic::{AtomicU64, Ordering},
};

const AUTHORITY: [u8; 32] = [0x51; 32];
const VECTOR: &str = include_str!("../../../../tests/vectors/program_account_state_v2.vec");
const SWAP: &[u8] = include_bytes!("../../../fixtures/pay5/swap-cpmm.wasm");

fn vector(name: &str) -> Vec<u8> {
    layerx_programs::hex::decode(
        VECTOR
            .lines()
            .find_map(|line| line.strip_prefix(name)?.strip_prefix('='))
            .unwrap_or_else(|| panic!("expected value")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"))
}
fn program() -> ProgramId {
    ProgramId::new([1; 32]).unwrap_or_else(|error| panic!("{error:?}"))
}
fn version(abi_version: u16) -> ProgramVersion {
    let wasm = layerx_programs_runtime::test_support::add_module();
    ProgramVersion {
        code_hash: hash_bytes(HashAlgorithm::Sha256, &wasm)
            .unwrap_or_else(|error| panic!("{error:?}")),
        wasm,
        abi_version,
    }
}
fn deployment(
    abi: u16,
    policy: UpgradePolicy,
) -> (
    Lifecycle,
    layerx_programs_runtime::DeploymentReceipt,
    ProgramVersion,
) {
    let version = version(abi);
    let mut runtime = Lifecycle::declared().unwrap_or_else(|error| panic!("{error:?}"));
    let receipt = runtime
        .deploy(Deploy {
            program: program(),
            code_hash: version.code_hash,
            wasm: version.wasm.clone(),
            abi_version: abi,
            upgrade_policy: policy,
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
    (runtime, receipt, version)
}
fn entries(registry: &Registry) -> Vec<layerx_programs::RegistryEntry> {
    registry
        .program_ids()
        .into_iter()
        .map(|id| {
            registry
                .entry_for_wind_down(id)
                .unwrap_or_else(|error| panic!("{error:?}"))
                .clone()
        })
        .collect()
}

#[test]
fn rejected_deployment_creates_entry_refusals_preserve_enumeration_and_policy() {
    let policy = UpgradePolicy::Authority(AUTHORITY);
    let (mut runtime, first, valid) = deployment(2, policy);
    let mut upgraded = valid.clone();
    upgraded.wasm.extend_from_slice(&[0, 2, 1, b'x']);
    upgraded.code_hash = hash_bytes(HashAlgorithm::Sha256, &upgraded.wasm)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let second = runtime
        .upgrade(Upgrade {
            program: program(),
            authority: AUTHORITY,
            code_hash: upgraded.code_hash,
            wasm: upgraded.wasm.clone(),
            abi_version: 2,
            migration: None,
        })
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(second.version(), 2);
    for case in 0..6 {
        let mut registry = Registry::new();
        let before = entries(&registry);
        let mut malformed = valid.clone();
        let result = match case {
            0 => registry.record_deployment(&second, &upgraded, policy, [8; 32]),
            1 => {
                malformed.abi_version = 0;
                registry.record_deployment(&first, &malformed, policy, [8; 32])
            }
            2 => {
                malformed.abi_version = u16::MAX;
                registry.record_deployment(&first, &malformed, policy, [8; 32])
            }
            3 => registry.record_deployment(
                &first,
                &valid,
                UpgradePolicy::Authority([0; 32]),
                [8; 32],
            ),
            4 => registry.record_deployment(&first, &valid, policy, [0; 32]),
            _ => {
                malformed.code_hash[0] ^= 1;
                registry.record_deployment(&first, &malformed, policy, [8; 32])
            }
        };
        match case {
            0 => assert_eq!(result, Err(RegistryError::VersionHistoryMismatch)),
            1 | 2 => assert!(matches!(result, Err(RegistryError::AbiVersion(_)))),
            3 => assert_eq!(result, Err(RegistryError::InvalidUpgradeAuthority)),
            _ => assert_eq!(result, Err(RegistryError::DeploymentMismatch)),
        }
        assert_eq!(entries(&registry), before, "case {case}");
        assert_eq!(registry.program_ids(), Vec::new());
        assert_eq!(
            registry.entry_for_wind_down(program()),
            Err(RegistryError::UnknownProgram)
        );
        // A refused first record must not pin its authority into a future entry.
        registry
            .record_deployment(&first, &valid, UpgradePolicy::Immutable, [9; 32])
            .unwrap_or_else(|error| panic!("{error:?}"));
        assert_eq!(registry.program_ids(), vec![program()]);
        assert_eq!(
            registry
                .entry_for_wind_down(program())
                .unwrap_or_else(|error| panic!("{error:?}"))
                .upgrade_policy,
            UpgradePolicy::Immutable
        );
        assert_eq!(registry.latest_version(program()), Ok(1));
        let admitted = entries(&registry);
        assert!(registry
            .record_deployment(&first, &valid, policy, [8; 32])
            .is_err());
        assert_eq!(entries(&registry), admitted);
    }
}

fn fixture_bindings() -> Vec<ProgramValueAccountBinding> {
    // These first two primary records are committed ABI-two producer vectors.
    let mut bindings = ["registration_primary_value", "empty_primary_value"]
        .into_iter()
        .map(|key| {
            ProgramValueAccountBinding::decode_primary_value(&vector(key))
                .unwrap_or_else(|error| panic!("{error:?}"))
        })
        .collect::<Vec<_>>();
    assert_eq!(
        bindings[0].primary_key(),
        vector("registration_primary_key")
    );
    assert_eq!(bindings[1].primary_key(), vector("empty_primary_key"));
    let seed = vector("order_a_seed");
    let mut third = ProgramValueAccountBinding {
        record_version: 2,
        program: program(),
        account_id: derive_program_account(program(), &seed)
            .unwrap_or_else(|error| panic!("{error:?}"))
            .bytes(),
        seed,
        asset_id: [0x33; 32],
        registered_sequence: 7,
        registration_event_digest: [0; 32],
    };
    third.registration_event_digest = program_account_registration_commitment(&third);
    bindings.push(third);
    bindings
}

struct FixtureRows {
    rows: Vec<(Vec<u8>, Vec<u8>)>,
    account_keys: Vec<Vec<u8>>,
    owner_key: Vec<u8>,
    profile_key: Vec<u8>,
}

fn fixture_rows(
    bindings: &[ProgramValueAccountBinding],
    frozen: bool,
    profile: bool,
    delta: u128,
    wrong_asset: bool,
) -> FixtureRows {
    let mut rows = Vec::new();
    let mut account_keys = Vec::new();
    for (index, binding) in bindings.iter().enumerate() {
        rows.push((
            programs_state_key(&binding.primary_key()),
            binding
                .primary_value()
                .unwrap_or_else(|error| panic!("{error:?}")),
        ));
        let leaf = CanonicalAccountLeaf {
            account_id: binding.account_id,
            name: format!(
                "module:programs:value:{}",
                layerx_programs::hex::encode(&binding.account_id)
            )
            .into_bytes(),
            kind: 13,
            balance: (1_u128 << 96) + index as u128 + delta,
            asset_id: if wrong_asset && index == 0 {
                [0x99; 32]
            } else {
                binding.asset_id
            },
            has_asset: true.into(),
            next_sequence: 9,
            created_at_sequence: 7,
            frozen: (frozen && index == 0).into(),
            has_open_reference: false.into(),
            authority_key: [0; 32],
            has_authority_key: false.into(),
        };
        let mut key = vec![6];
        key.extend_from_slice(&binding.account_id);
        rows.push((
            key.clone(),
            leaf.host_record_encoding()
                .unwrap_or_else(|error| panic!("{error:?}")),
        ));
        account_keys.push(key);
        // An old module mirror is not a source of the live balance.
        let mut mirror = leaf.clone();
        mirror.balance = 0;
        rows.push((
            programs_state_key(&leaf.primary_key()),
            mirror
                .primary_value()
                .unwrap_or_else(|error| panic!("{error:?}")),
        ));
    }
    rows.push((vec![2], 100_u64.to_be_bytes().to_vec()));
    let mut owner_key = b"program-owner\0".to_vec();
    owner_key.extend_from_slice(&program().bytes());
    let mut profile_key = b"program-account-profile\0".to_vec();
    profile_key.extend_from_slice(&program().bytes());
    if profile {
        let mut owner = vec![1];
        owner.extend_from_slice(&AUTHORITY);
        let mut value = vec![2];
        value.extend_from_slice(&AUTHORITY);
        rows.push((programs_state_key(&owner_key), owner));
        rows.push((programs_state_key(&profile_key), value));
    }
    FixtureRows {
        rows,
        account_keys,
        owner_key,
        profile_key,
    }
}

struct Fixture {
    chain: ChainFixture,
    proof: ChainProgramAccountProof,
    head: VerifiedChainHead,
    record: DeploymentRecord,
}
impl Fixture {
    fn new(frozen: bool, profile: bool, delta: u128, wrong_asset: bool) -> Self {
        let mut bindings = fixture_bindings();
        let FixtureRows {
            rows,
            account_keys,
            owner_key,
            profile_key,
        } = fixture_rows(&bindings, frozen, profile, delta, wrong_asset);
        let chain = ChainFixture::new(
            "registry-chain-account-repair",
            &[[1; 32], [2; 32], [3; 32], [4; 32]],
            rows,
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        let head = chain
            .verifier
            .verify_head(&chain.header, &chain.commit)
            .unwrap_or_else(|error| panic!("{error:?}"));
        let response = |key: &[u8]| {
            decode_commitment_proof(
                &chain
                    .encode_key(key)
                    .unwrap_or_else(|error| panic!("{error:?}")),
            )
            .unwrap_or_else(|error| panic!("{error:?}"))
        };
        let (lower, upper) = ChainProgramAccountProof::binding_range(program());
        bindings.sort_by_key(layerx_programs::ProgramValueAccountBinding::primary_key);
        let proof = ChainProgramAccountProof {
            program: program(),
            lower_bound: response(&lower),
            upper_bound: response(&upper),
            bindings: bindings
                .iter()
                .map(|b| response(&programs_state_key(&b.primary_key())))
                .collect(),
            accounts: account_keys.iter().map(|key| response(key)).collect(),
            sequence: response(&[2]),
            profile: profile.then(|| response(&programs_state_key(&profile_key))),
            owner: profile.then(|| response(&programs_state_key(&owner_key))),
        };
        let (_, receipt, version) = deployment(
            if profile { 4 } else { 2 },
            UpgradePolicy::Authority(AUTHORITY),
        );
        let record = DeploymentRecord::from_deployment(
            &receipt,
            &version,
            UpgradePolicy::Authority(AUTHORITY),
            1,
            u64::from_le_bytes((head.time_ms()).to_le_bytes()),
        )
        .unwrap_or_else(|error| panic!("{error:?}"));
        Self {
            chain,
            proof,
            head,
            record,
        }
    }
    fn registry(&self) -> Registry {
        let mut registry = Registry::new();
        registry
            .replay_journal(std::slice::from_ref(&self.record))
            .unwrap_or_else(|error| panic!("{error:?}"));
        registry
    }
    fn now(&self) -> u64 {
        u64::from_le_bytes((self.head.time_ms()).to_le_bytes()) + 1
    }
}

fn verify_account_range_refusals(
    fixture: &Fixture,
    transported: &ChainProgramAccountProof,
    encoded: &[u8],
) -> Fixture {
    for mask in 0_u8..7 {
        let mut missing = transported.clone();
        missing.bindings = missing
            .bindings
            .into_iter()
            .enumerate()
            .filter(|(i, _)| mask & (1 << i) != 0)
            .map(|(_, p)| p)
            .collect();
        missing.accounts.retain(|p| {
            missing.bindings.iter().any(|b| {
                ProgramValueAccountBinding::decode_primary_value(
                    b.value.as_ref().unwrap_or_else(|| panic!("expected value")),
                )
                .unwrap_or_else(|error| panic!("{error:?}"))
                .account_id
                .as_slice()
                    == &p.key[1..]
            })
        });
        assert!(
            missing.verify(&fixture.head).is_err(),
            "omission mask {mask}"
        );
    }
    let mut reordered = transported.clone();
    reordered.bindings.reverse();
    assert!(reordered.verify(&fixture.head).is_err());
    let mut duplicate = transported.clone();
    duplicate.bindings[1] = duplicate.bindings[0].clone();
    assert!(duplicate.verify(&fixture.head).is_err());
    let mut duplicate = transported.clone();
    duplicate.accounts[1] = duplicate.accounts[0].clone();
    assert!(matches!(
        duplicate.verify(&fixture.head),
        Err(AccountStateError::DuplicateAccount { .. })
    ));
    let mut incomplete = transported.clone();
    incomplete.accounts.pop();
    assert!(incomplete.verify(&fixture.head).is_err());
    let mut boundary = transported.clone();
    boundary.lower_bound = boundary.upper_bound.clone();
    assert!(boundary.verify(&fixture.head).is_err());
    let mut boundary = transported.clone();
    boundary.upper_bound = boundary.lower_bound.clone();
    assert!(boundary.verify(&fixture.head).is_err());
    let other = Fixture::new(false, true, 1, false);
    let mut crossed = transported.clone();
    crossed.accounts[0] = other.proof.accounts[0].clone();
    assert!(crossed.verify(&fixture.head).is_err());
    let mut forged = transported.clone();
    forged.accounts[0]
        .value
        .as_mut()
        .unwrap_or_else(|| panic!("expected value"))[120] ^= 1;
    assert!(forged.verify(&fixture.head).is_err());
    let mut forged = transported.clone();
    forged.sequence = transported.accounts[0].clone();
    assert!(forged.verify(&fixture.head).is_err());
    let wrong_asset = Fixture::new(false, true, 0, true);
    assert!(matches!(
        wrong_asset.proof.verify(&wrong_asset.head),
        Err(AccountStateError::AccountMismatch { .. })
    ));
    let mut missing_profile = transported.clone();
    missing_profile.profile = None;
    assert!(missing_profile.verify(&fixture.head).is_err());
    for bytes in [&encoded[..encoded.len() - 1], &[encoded, &[0]].concat()[..]] {
        assert!(ChainProgramAccountProof::decode(bytes).is_err());
    }
    other
}

#[test]
fn chain_account_consumer_wiring_complete_range_refuses_every_omission() {
    let fixture = Fixture::new(false, true, 0, false);
    let encoded = fixture
        .proof
        .canonical_encoding()
        .unwrap_or_else(|error| panic!("{error:?}"));
    let transported =
        ChainProgramAccountProof::decode(&encoded).unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(transported, fixture.proof);
    let snapshot = transported
        .verify(&fixture.head)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(snapshot.bindings().len(), 3);
    assert_eq!(snapshot.accounts().len(), 3);
    assert_eq!(snapshot.chain_head().height() - 1, chain::STATE_HEIGHT);
    assert_eq!(snapshot.freshness().observed_sequence, 100);
    let other = verify_account_range_refusals(&fixture, &transported, &encoded);
    let registry = fixture.registry();
    let authority = ChainAccountStateAuthority::new(&fixture.head, fixture.now(), 1000)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let read = registry
        .read_value_accounts_chain(program(), &snapshot, &authority)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(read.value_accounts(), snapshot.accounts());
    assert!(read
        .value_accounts()
        .iter()
        .all(|a| a.balance >= 1_u128 << 96));
    assert_eq!(read.state_root(), fixture.head.app_hash());
    assert_eq!(read.receipt_digest(), fixture.head.header_hash());
    assert_eq!(read.freshness(), snapshot.freshness());
    let other_authority = ChainAccountStateAuthority::new(&other.head, other.now(), 1000)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        registry.read_value_accounts_chain(program(), &snapshot, &other_authority),
        Err(AccountStateError::UnverifiedReceipt)
    );
    assert!(matches!(
        ChainAccountStateAuthority::new(&fixture.head, fixture.now() + 1000, 1000),
        Err(AccountStateError::StaleRead)
    ));
    assert!(matches!(
        ChainAccountStateAuthority::new(&fixture.head, fixture.now() - 1002, 1000),
        Err(AccountStateError::StaleRead)
    ));
    let mut no_profile = transported;
    no_profile.profile = None;
    no_profile.owner = None;
    let legacy_view = no_profile
        .verify(&fixture.head)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        registry.read_value_accounts_chain(program(), &legacy_view, &authority),
        Err(AccountStateError::LegacyProtocol)
    );
}

#[test]
fn chain_account_consumer_wiring_empty_range_is_proven_and_program_bound() {
    let fixture = Fixture::new(false, false, 0, false);
    let other = ProgramId::new([0xff; 32]).unwrap_or_else(|error| panic!("{error:?}"));
    let (lower, upper) = ChainProgramAccountProof::binding_range(other);
    assert!(lower < upper);
    let response = |key: &[u8]| {
        decode_commitment_proof(
            &fixture
                .chain
                .encode_key(key)
                .unwrap_or_else(|error| panic!("{error:?}")),
        )
        .unwrap_or_else(|error| panic!("{error:?}"))
    };
    let proof = ChainProgramAccountProof {
        program: other,
        lower_bound: response(&lower),
        upper_bound: response(&upper),
        bindings: vec![],
        accounts: vec![],
        sequence: fixture.proof.sequence.clone(),
        profile: None,
        owner: None,
    };
    let empty = proof
        .verify(&fixture.head)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(empty.bindings(), &[]);
    assert_eq!(empty.accounts(), &[]);
    let mut forged_empty = fixture.proof.clone();
    forged_empty.bindings.clear();
    forged_empty.accounts.clear();
    assert!(forged_empty.verify(&fixture.head).is_err());
    let authority = ChainAccountStateAuthority::new(&fixture.head, fixture.now(), 1000)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        fixture
            .registry()
            .read_value_accounts_chain(program(), &empty, &authority),
        Err(AccountStateError::InvalidBinding)
    );
}

struct Store(PathBuf);
impl Store {
    fn open() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "registry-repair-220-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap_or_else(|error| panic!("{error:?}"));
        Self(path)
    }
}
impl Drop for Store {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap_or_else(|error| panic!("{error:?}"));
    }
}

fn routes(snapshot: &layerx_programs::VerifiedChainProgramAccounts) -> Vec<ExitRoute> {
    snapshot
        .bindings()
        .iter()
        .map(|b| ExitRoute {
            seed: b.seed.clone(),
            account_id: b.account_id,
            asset_id: b.asset_id,
            destination: [0x61; 32],
        })
        .collect()
}
fn transition() -> LifecycleTransition {
    LifecycleTransition {
        program: program(),
        expected: ProgramLifecycle::Active,
        target: ProgramLifecycle::Deprecated,
        authority: AUTHORITY,
        effective_sequence: 101,
        wind_down: WindDownPolicy {
            exit_program: program().bytes(),
            deadline: 200,
            state_access: WindDownStateAccess::ReadOnly,
        },
        live_value_accounts: 3,
    }
}

struct RecoveryExpectations {
    before_read: layerx_programs::VerifiedProgramBalanceRead,
    after_read: layerx_programs::VerifiedProgramBalanceRead,
    view: layerx_programs::WindDownView,
    authorizations: Vec<layerx_programs::AuthorizedExit>,
    expected_entries: Vec<layerx_programs::RegistryEntry>,
    journal: Vec<(LifecycleTransition, Vec<ExitRoute>)>,
}

fn verify_profile_recovery(fixture: &Fixture, store: &Store, expected: RecoveryExpectations) {
    let RecoveryExpectations {
        before_read,
        after_read,
        view,
        authorizations,
        expected_entries,
        journal,
    } = expected;
    // New components consume only reopened production codec bytes and the durable log.
    let proof = ChainProgramAccountProof::decode(
        &fs::read(store.0.join("accounts")).unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let head = fixture
        .chain
        .verifier
        .verify_head(&fixture.chain.header, &fixture.chain.commit)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let restored = proof
        .verify(&head)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let record = DeploymentRecord::decode(
        &fs::read(store.0.join("deployment")).unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let mut registry = Registry::new();
    registry
        .replay_journal(&[record])
        .unwrap_or_else(|error| panic!("{error:?}"));
    let authority = ChainAccountStateAuthority::new(&head, fixture.now(), 1000)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(
        registry
            .read_value_accounts_chain(program(), &restored, &authority)
            .unwrap_or_else(|error| panic!("{error:?}")),
        before_read
    );
    let mut deprecation = Deprecation::new();
    for (request, exits) in &journal {
        deprecation
            .transition_chain(&mut registry, *request, exits, &restored, &authority)
            .unwrap_or_else(|error| panic!("{error:?}"));
    }
    assert_eq!(entries(&registry), expected_entries);
    assert_eq!(
        deprecation
            .read_chain(&registry, program(), &restored, &authority)
            .unwrap_or_else(|error| panic!("{error:?}")),
        view
    );
    assert_eq!(
        registry
            .read_value_accounts_chain(program(), &restored, &authority)
            .unwrap_or_else(|error| panic!("{error:?}")),
        after_read
    );
    for expected in authorizations {
        assert_eq!(
            deprecation
                .authorize_exit_chain(
                    &registry,
                    program(),
                    expected.account.account_id,
                    &restored,
                    &authority
                )
                .unwrap_or_else(|error| panic!("{error:?}")),
            expected
        );
    }
}

fn produce_profile_recovery(fixture: &Fixture, store: &Store) -> RecoveryExpectations {
    fs::write(
        store.0.join("accounts"),
        fixture
            .proof
            .canonical_encoding()
            .unwrap_or_else(|error| panic!("{error:?}")),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    fs::write(
        store.0.join("deployment"),
        fixture.record.canonical_encoding(),
    )
    .unwrap_or_else(|error| panic!("{error:?}"));
    let snapshot = fixture
        .proof
        .verify(&fixture.head)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let authority = ChainAccountStateAuthority::new(&fixture.head, fixture.now(), 1000)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut registry = fixture.registry();
    let before_read = registry
        .read_value_accounts_chain(program(), &snapshot, &authority)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let exits = routes(&snapshot);
    let request = transition();
    // The committed in-memory journal is a peer retained across consumer restart.
    let journal = vec![(request, exits.clone())];
    let mut deprecation = Deprecation::new();
    let entry_before = entries(&registry);
    assert!(matches!(
        deprecation.transition_chain(&mut registry, request, &exits[..2], &snapshot, &authority),
        Err(DeprecationRefusal::MissingExit { .. })
    ));
    assert_eq!(entries(&registry), entry_before);
    let mut stale = request;
    stale.effective_sequence -= 1;
    assert_eq!(
        deprecation.transition_chain(&mut registry, stale, &exits, &snapshot, &authority),
        Err(DeprecationRefusal::SnapshotSequenceMismatch)
    );
    assert_eq!(entries(&registry), entry_before);
    let receipt = deprecation
        .transition_chain(&mut registry, request, &exits, &snapshot, &authority)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(receipt.live_value_accounts, 3);
    let after_read = registry
        .read_value_accounts_chain(program(), &snapshot, &authority)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let view = deprecation
        .read_chain(&registry, program(), &snapshot, &authority)
        .unwrap_or_else(|error| panic!("{error:?}"));
    assert_eq!(view.value_accounts, before_read.value_accounts());
    assert_eq!(view.value_accounts, after_read.value_accounts());
    assert_eq!(
        view.reachable_value_by_asset()
            .unwrap_or_else(|| panic!("expected value"))
            .len(),
        2
    );
    let authorizations: Vec<_> = snapshot
        .accounts()
        .iter()
        .map(|account| {
            deprecation
                .authorize_exit_chain(
                    &registry,
                    program(),
                    account.account_id,
                    &snapshot,
                    &authority,
                )
                .unwrap_or_else(|error| panic!("{error:?}"))
        })
        .collect();
    for exit in &authorizations {
        assert_eq!(exit.protocol_activity.activity_type, 0x0009_0007);
        assert_eq!(&exit.protocol_activity.payload[..32], &program().bytes());
        assert_eq!(exit.protocol_activity.payload[32], 4);
        assert_eq!(
            &exit.protocol_activity.payload[33..],
            &exit.account.account_id
        );
        assert_eq!(exit.account.state_root, fixture.head.app_hash());
    }
    let expected_entries = entries(&registry);
    drop(deprecation);
    drop(registry);
    drop(snapshot);
    RecoveryExpectations {
        before_read,
        after_read,
        view,
        authorizations,
        expected_entries,
        journal,
    }
}

#[test]
fn chain_account_consumer_wiring_balances_and_wind_down_recover_the_same_view() {
    for profile in [false, true] {
        let fixture = Fixture::new(false, profile, 0, false);
        let store = Store::open();
        let expected = produce_profile_recovery(&fixture, &store);
        verify_profile_recovery(&fixture, &store, expected);
    }
    let fixture = Fixture::new(true, true, 0, false);
    let snapshot = fixture
        .proof
        .verify(&fixture.head)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let authority = ChainAccountStateAuthority::new(&fixture.head, fixture.now(), 1000)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut registry = fixture.registry();
    let before = entries(&registry);
    assert!(matches!(
        Deprecation::new().transition_chain(
            &mut registry,
            transition(),
            &routes(&snapshot),
            &snapshot,
            &authority
        ),
        Err(DeprecationRefusal::ValueWouldBeStranded { .. })
    ));
    assert_eq!(entries(&registry), before);
}

#[test]
fn pool_refusal_mutates_state_boundary_preserves_entire_pool_and_allows_retry() {
    use layerx_programs::swap::{Pool, PoolRefusal, SHARE_CEILING};
    let mut pool = Pool::default();
    assert_eq!(pool.add_liquidity(1, 1, u128::MAX), Ok((1, u128::MAX)));
    let before = pool;
    assert_eq!(
        pool.add_liquidity(1, 1, u128::MAX),
        Err(PoolRefusal::Overflow)
    );
    assert_eq!(pool, before);
    assert_eq!(pool.remove_liquidity(1, 1, u128::MAX), Ok((1, u128::MAX)));
    assert_eq!(pool, Pool::default());
    for (shares, a, b, expected) in [
        (0, 1, 1, PoolRefusal::Amount),
        (1, 0, 1, PoolRefusal::Slippage),
        (1, 1, 0, PoolRefusal::Amount),
        (
            SHARE_CEILING + 1,
            u128::MAX,
            u128::MAX,
            PoolRefusal::Ceiling,
        ),
    ] {
        let before = pool;
        assert_eq!(pool.add_liquidity(shares, a, b), Err(expected));
        assert_eq!(pool, before);
    }
    assert_eq!(pool.add_liquidity(2, 2, 4), Ok((2, 4)));
    let before = pool;
    assert_eq!(pool.remove_liquidity(1, 2, 2), Err(PoolRefusal::Slippage));
    assert_eq!(pool, before);
}

#[test]
fn pool_refusal_mutates_state_real_guest_transaction_rolls_back_share_debit() {
    let pool = ProgramId::new([0x55; 32]).unwrap_or_else(|error| panic!("{error:?}"));
    let actor = PrincipalId::new([0x71; 32]).unwrap_or_else(|error| panic!("{error:?}"));
    let holder = derive_program_account(pool, &actor.bytes())
        .unwrap_or_else(|error| panic!("{error:?}"))
        .bytes();
    let treasury = derive_program_account(pool, b"lx.ref.swap.treasury")
        .unwrap_or_else(|error| panic!("{error:?}"))
        .bytes();
    let mut holder_key = b"lx.ref.swap.share:".to_vec();
    holder_key.extend_from_slice(&holder);
    let mut storage = Storage::new();
    let mut tx = storage.transaction(StorageNamespace::shared(pool));
    for (key, amount) in [
        (b"lx.ref.swap.reserve.a".as_slice(), 10_u128),
        (b"lx.ref.swap.reserve.b", 20),
        (b"lx.ref.swap.shares", 2),
        (holder_key.as_slice(), 2),
    ] {
        tx.write(key, &amount.to_be_bytes())
            .unwrap_or_else(|error| panic!("{error:?}"));
    }
    assert_eq!(tx.commit(), 4);
    let before = storage.clone();
    let engine = WasmEngine::declared().unwrap_or_else(|error| panic!("{error:?}"));
    let module = engine
        .validate_candidate_v2(SWAP)
        .unwrap_or_else(|error| panic!("{error:?}"));
    let mut payload = treasury.to_vec();
    payload.extend_from_slice(&1_u128.to_be_bytes());
    payload.extend_from_slice(&0_u128.to_be_bytes());
    payload.extend_from_slice(&0_u128.to_be_bytes());
    let mut calldata = b"LXS\x02\x01\x20".to_vec();
    calldata.extend_from_slice(&80_u32.to_be_bytes());
    calldata.extend_from_slice(&payload);
    let result = Executor::declared().execute_authorized_versioned_for_qualification(
        &mut storage,
        AuthorizedExecutionRequest {
            module: &module,
            program: pool,
            authorization: AuthorizationContext::new(
                actor,
                CapabilitySet::new([
                    Capability::SharedStorageRead,
                    Capability::SharedStorageWrite,
                ])
                .unwrap_or_else(|error| panic!("{error:?}")),
            ),
            receipts: &UnavailableReceiptOracle,
            entrypoint: "remove_liquidity",
            calldata: &calldata,
            composition: CompositionContext::isolated(),
            response_capacity: 128,
        },
    );
    let record = result.unwrap_or_else(|error| panic!("{error:?}"));
    assert!(record.failure().is_some());
    assert!(record.effects().is_none());
    assert!(
        record.execution().usage().storage_write_bytes > 0,
        "the real share debit must precede the refused transfer"
    );
    assert_eq!(storage, before);
}
