use std::path::PathBuf;
use std::sync::{Arc, Mutex};

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::budget::{
    reserve, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    hold, hold_reserved, reserve_and_hold, ApprovalContext, ApprovalError, ApprovalRegistry,
    ApprovalReservationError, ApprovalState,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::{ObjectKind, Store, TenantId, TenantKey};
use layerx_types::ids::Did;
use layerx_types::payload::ModuleRegistry;
use sha2::{Digest as _, Sha256};

const ACTOR: &str = "0404040404040404040404040404040404040404";

struct Root(PathBuf);

impl Root {
    fn new(label: &str) -> Self {
        let root = Self(
            std::env::temp_dir().join(format!("repair-22-31-216-{label}-{}", std::process::id())),
        );
        std::fs::create_dir(&root.0).unwrap_or_else(|error| panic!("create store root: {error}"));
        root
    }

    fn open(&self) -> Arc<Mutex<Store>> {
        Arc::new(Mutex::new(
            Store::open(&self.0).unwrap_or_else(|error| panic!("open store: {error}")),
        ))
    }
}

impl Drop for Root {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}

fn tenant() -> TenantId {
    TenantId::new("approval-replay-bounds").unwrap_or_else(|error| panic!("tenant: {error}"))
}

fn context(id: u8) -> ApprovalContext {
    ApprovalContext {
        tenant: tenant(),
        agent: Did::new(ACTOR.as_bytes()).unwrap_or_else(|error| panic!("actor: {error:?}")),
        actor_address: [4; 20],
        session: SessionId([2; 32]),
        capability: CapabilityId([3; 32]),
        policy_version: "replay-bounds".to_owned(),
        request_id: [id; 32],
    }
}

fn prepared(counterparties: usize, amounts: usize) -> Prepared {
    let call = layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: 1,
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [3; 20],
        data: [
            layerx_crypto::disclosure::call::ledger_selector("transfer(address,uint256)").to_vec(),
            [vec![0; 12], vec![5; 20]].concat(),
            [vec![0; 31], vec![7]].concat(),
        ]
        .concat(),
    };
    let canonical = call.encode_unsigned();
    let counterparty = |index| {
        AgentDid::new(format!("did:layerx:counterparty-{index}"))
            .unwrap_or_else(|error| panic!("counterparty: {error:?}"))
    };
    Prepared {
        preparation_ref: PreparationRef::new("replay-bounds")
            .unwrap_or_else(|error| panic!("preparation: {error:?}")),
        signing_preimage: SigningPreimage::new(call.signing_hash().to_vec())
            .unwrap_or_else(|error| panic!("preimage: {error:?}")),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(&canonical).into(),
            activity_type: ActivityType(7),
            actor: AgentDid::new(ACTOR).unwrap_or_else(|error| panic!("actor: {error:?}")),
            authority: AuthorityRef::new("session-key")
                .unwrap_or_else(|error| panic!("authority: {error:?}")),
            counterparties: ExplicitSet::allow((0..counterparties).map(counterparty).collect()),
            amounts: ExplicitSet::allow(
                (0..amounts)
                    .map(|index| DisclosedAmount {
                        counterparty: counterparty(index),
                        amount: Amount(7),
                    })
                    .collect(),
            ),
            asset: Asset::new("LXP").unwrap_or_else(|error| panic!("asset: {error:?}")),
            fee_limit: Amount(2),
            expiry: TimestampSeconds(500),
            idempotency_key: IdempotencyRef::new("replay-bounds")
                .unwrap_or_else(|error| panic!("idempotency: {error:?}")),
        },
        unsigned_canonical_bytes: CanonicalBytes::new(canonical)
            .unwrap_or_else(|error| panic!("canonical: {error:?}")),
        expiry: TimestampSeconds(500),
    }
}

fn limiter() -> BudgetLimiter {
    BudgetLimiter::new(
        (1..=65)
            .map(|id| LimitConfig {
                id: LimitId([id; 16]),
                name: format!("limit-{id}"),
                scope: LimitScope::Tenant([1; 32]),
                ceiling: 100,
                consumed: 3,
            })
            .collect(),
    )
    .unwrap_or_else(|error| panic!("limiter: {error:?}"))
}

fn request(id: u8, limits: u8) -> ReservationRequest {
    ReservationRequest {
        id: [id; 32],
        amount: 7,
        expiry_sequence: 100,
        current_sequence: 10,
        applicable_limits: (1..=limits).map(|id| LimitId([id; 16])).collect(),
    }
}

fn hold_key(id: u8) -> TenantKey {
    TenantKey::new(
        tenant(),
        ObjectKind::PreparedActivity,
        [b"approval-hold-v1:".as_slice(), &[id; 32]].concat(),
    )
    .unwrap_or_else(|error| panic!("hold key: {error}"))
}

#[test]
fn approval_replay_bounds_64_items_survive_reopen() {
    let root = Root::new("64");
    let original = prepared(64, 64);
    let ticket = {
        let store = root.open();
        let registry = ApprovalRegistry::with_store(Arc::clone(&store));
        let budgets = limiter();
        let ticket = reserve_and_hold(
            &registry,
            &budgets,
            context(1),
            original.clone(),
            &request(1, 64),
            100,
        )
        .unwrap_or_else(|error| panic!("publish: {error:?}"));
        assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
        assert_eq!(budgets.held_reservations(), Ok(64));
        assert_eq!(registry.ticket([1; 32]), Ok(Some(ticket.clone())));
        let guard = store.lock().unwrap_or_else(|_| panic!("store lock"));
        let bytes = guard
            .get(&hold_key(1))
            .unwrap_or_else(|| panic!("durable hold"));
        assert!(bytes.bytes().len() < 2 * 1024 * 1024);
        ticket
    };
    let registry = ApprovalRegistry::with_store(root.open());
    let budgets = limiter();
    assert_eq!(budgets.held_reservations(), Ok(0));
    assert_eq!(registry.replay_tenant(&tenant(), &budgets), Ok(1));
    assert_eq!(registry.ticket([1; 32]), Ok(Some(ticket)));
    let modules = ModuleRegistry::new(&[]).unwrap_or_else(|error| panic!("modules: {error:?}"));
    assert_eq!(registry.validate_registry(&tenant(), &modules), Ok(()));
    assert_eq!(budgets.held_reservations(), Ok(64));
    for id in 1..=65 {
        assert_eq!(
            budgets.held_exposure(LimitId([id; 16])),
            Ok(if id <= 64 { 7 } else { 0 })
        );
        assert_eq!(budgets.consumed(LimitId([id; 16])), Ok(3));
    }
}

#[test]
fn approval_replay_bounds_65_items_refuse_without_reservation_leaks() {
    for (label, limits, counterparties, amounts) in [
        ("reservations", 65, 1, 1),
        ("counterparties", 64, 65, 1),
        ("amounts", 64, 1, 65),
    ] {
        let root = Root::new(label);
        let retained = {
            let store = root.open();
            let registry = ApprovalRegistry::with_store(Arc::clone(&store));
            let budgets = limiter();
            let retained = reserve_and_hold(
                &registry,
                &budgets,
                context(1),
                prepared(1, 1),
                &request(1, 1),
                100,
            )
            .unwrap_or_else(|error| panic!("retained hold: {error:?}"));
            let before = store
                .lock()
                .unwrap_or_else(|_| panic!("store lock"))
                .get(&hold_key(1))
                .unwrap_or_else(|| panic!("retained record"))
                .bytes()
                .to_vec();
            assert_eq!(
                reserve_and_hold(
                    &registry,
                    &budgets,
                    context(2),
                    prepared(counterparties, amounts),
                    &request(2, limits),
                    100
                ),
                Err(ApprovalReservationError::Approval(
                    ApprovalError::TooManyItems
                )),
                "{label}"
            );
            assert_eq!(registry.ticket([2; 32]), Ok(None));
            assert_eq!(registry.ticket([1; 32]), Ok(Some(retained.clone())));
            assert_eq!(budgets.has_reservation([2; 32]), Ok(false));
            assert_eq!(budgets.held_reservations(), Ok(1));
            for id in 1..=65 {
                assert_eq!(
                    budgets.held_exposure(LimitId([id; 16])),
                    Ok(if id == 1 { 7 } else { 0 })
                );
                assert_eq!(budgets.consumed(LimitId([id; 16])), Ok(3));
            }
            let guard = store.lock().unwrap_or_else(|_| panic!("store lock"));
            assert!(guard.get(&hold_key(2)).is_none());
            assert_eq!(
                guard
                    .list_object_ids(&tenant(), ObjectKind::PreparedActivity)
                    .len(),
                1
            );
            assert_eq!(
                guard
                    .get(&hold_key(1))
                    .unwrap_or_else(|| panic!("retained record"))
                    .bytes(),
                before
            );
            retained
        };
        let registry = ApprovalRegistry::with_store(root.open());
        let budgets = limiter();
        assert_eq!(registry.replay_tenant(&tenant(), &budgets), Ok(1));
        assert_eq!(registry.ticket([1; 32]), Ok(Some(retained)));
        assert_eq!(registry.ticket([2; 32]), Ok(None));
        assert_eq!(budgets.has_reservation([2; 32]), Ok(false));
        assert_eq!(budgets.held_reservations(), Ok(1));
    }
}

#[test]
fn approval_replay_bounds_borrowed_admission_rejects_65_items() {
    let root = Root::new("borrowed");
    let store = root.open();
    let registry = ApprovalRegistry::with_store(Arc::clone(&store));
    let budgets = limiter();
    let reservation =
        reserve(&budgets, &request(1, 65)).unwrap_or_else(|error| panic!("reserve: {error:?}"));
    assert_eq!(
        hold_reserved(&registry, context(1), prepared(1, 1), 10, 100, &reservation),
        Err(ApprovalError::TooManyItems)
    );
    assert_eq!(registry.hold_ids(), Ok(Vec::new()));
    assert!(store
        .lock()
        .unwrap_or_else(|_| panic!("store lock"))
        .get(&hold_key(1))
        .is_none());
    for (counterparties, amounts) in [(65, 1), (1, 65)] {
        assert_eq!(
            hold(
                &ApprovalRegistry::default(),
                context(2),
                prepared(counterparties, amounts),
                10,
                100
            ),
            Err(ApprovalError::TooManyItems)
        );
    }
    assert_eq!(
        layerx_agentd::budget::release(
            &budgets,
            [1; 32],
            layerx_agentd::budget::ReleaseKind::Failed,
            10
        ),
        Ok(true)
    );
    assert_eq!(budgets.held_reservations(), Ok(0));
}

fn skip_bytes(bytes: &[u8], position: &mut usize) {
    let length = u32::from_be_bytes(
        bytes[*position..*position + 4]
            .try_into()
            .unwrap_or_else(|_| panic!("length")),
    ) as usize;
    *position += 4 + length;
}

fn count_offsets(bytes: &[u8]) -> [usize; 3] {
    assert_eq!(&bytes[..8], b"LXAPHLD4");
    let mut position = 8;
    skip_bytes(bytes, &mut position);
    skip_bytes(bytes, &mut position);
    position += 32 + 32;
    skip_bytes(bytes, &mut position);
    position += 32 + 20 + 8 + 8;
    let reservations = position;
    assert_eq!(&bytes[position..position + 4], &64_u32.to_be_bytes());
    position += 4 + 64 * (32 + 16 + 1 + 32 + 16 + 16 + 8 + 32);
    for _ in 0..3 {
        skip_bytes(bytes, &mut position);
    }
    position += 32 + 2;
    skip_bytes(bytes, &mut position);
    skip_bytes(bytes, &mut position);
    let counterparties = position;
    assert_eq!(&bytes[position..position + 4], &64_u32.to_be_bytes());
    position += 4;
    for _ in 0..64 {
        skip_bytes(bytes, &mut position);
    }
    let amounts = position;
    assert_eq!(&bytes[position..position + 4], &64_u32.to_be_bytes());
    [reservations, counterparties, amounts]
}

#[test]
fn approval_replay_bounds_decoder_still_refuses_oversized_records() {
    for field in 0..3 {
        let root = Root::new(&format!("corrupt-{field}"));
        {
            let store = root.open();
            let registry = ApprovalRegistry::with_store(Arc::clone(&store));
            reserve_and_hold(
                &registry,
                &limiter(),
                context(1),
                prepared(64, 64),
                &request(1, 64),
                100,
            )
            .unwrap_or_else(|error| panic!("publish: {error:?}"));
            let mut guard = store.lock().unwrap_or_else(|_| panic!("store lock"));
            let mut bytes = guard
                .get(&hold_key(1))
                .unwrap_or_else(|| panic!("hold record"))
                .bytes()
                .to_vec();
            let offset = count_offsets(&bytes)[field];
            bytes[offset..offset + 4].copy_from_slice(&65_u32.to_be_bytes());
            guard
                .update_local_batch(vec![(hold_key(1), bytes)])
                .unwrap_or_else(|error| panic!("persist corrupt count: {error}"));
        }
        let registry = ApprovalRegistry::with_store(root.open());
        let budgets = limiter();
        assert_eq!(
            registry.replay_tenant(&tenant(), &budgets),
            Err(ApprovalError::CorruptRecord)
        );
        assert_eq!(registry.hold_ids(), Ok(Vec::new()));
        assert_eq!(budgets.held_reservations(), Ok(0));
    }
}
