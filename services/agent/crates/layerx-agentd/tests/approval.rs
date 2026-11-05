use std::sync::Arc;
use std::thread;

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{
    decide_at, expire_at, hold_at, hold_reserved_timed, ApprovalChoice, ApprovalContext,
    ApprovalError, ApprovalRegistry, ApprovalSequence, ApprovalState, ApproverId,
};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::TenantId;
use layerx_types::ids::Did;
use sha2::{Digest as _, Sha256};

fn digest(bytes: &[u8]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(bytes);
    hasher.finalize().into()
}

fn prepared(label: &[u8]) -> Prepared {
    let id = label.last().copied().unwrap_or(1);
    let call = layerx_crypto::evm_transaction::Eip1559Call {
        chain_id: 125,
        nonce: u64::from(id),
        max_priority_fee_per_gas: 1,
        max_fee_per_gas: 2,
        gas_limit: 100_000,
        to: [3; 20],
        data: [
            layerx_crypto::disclosure::call::ledger_selector("transfer(address,uint256)").to_vec(),
            [vec![0; 12], vec![5; 20]].concat(),
            [vec![0; 31], vec![50]].concat(),
        ]
        .concat(),
    };
    let bytes = call.encode_unsigned();
    let actor = AgentDid::new("0404040404040404040404040404040404040404")
        .unwrap_or_else(|error| panic!("actor: {error:?}"));
    Prepared {
        preparation_ref: PreparationRef::new("prepared-1")
            .unwrap_or_else(|error| panic!("preparation: {error:?}")),
        unsigned_canonical_bytes: CanonicalBytes::new(bytes.to_vec())
            .unwrap_or_else(|error| panic!("canonical: {error:?}")),
        signing_preimage: SigningPreimage::new(call.signing_hash().to_vec())
            .unwrap_or_else(|error| panic!("preimage: {error:?}")),
        disclosure: Disclosure {
            canonical_digest: digest(&bytes),
            activity_type: ActivityType(7),
            actor: actor.clone(),
            authority: AuthorityRef::new("session-key-1")
                .unwrap_or_else(|error| panic!("authority: {error:?}")),
            counterparties: ExplicitSet::allow(vec![AgentDid::new("did:layerx:merchant")
                .unwrap_or_else(|error| panic!("counterparty: {error:?}"))]),
            amounts: ExplicitSet::allow(vec![DisclosedAmount {
                counterparty: AgentDid::new("did:layerx:merchant")
                    .unwrap_or_else(|error| panic!("counterparty: {error:?}")),
                amount: Amount(50),
            }]),
            asset: Asset::new("LXP").unwrap_or_else(|error| panic!("asset: {error:?}")),
            fee_limit: Amount(2),
            expiry: TimestampSeconds(500),
            idempotency_key: IdempotencyRef::new("idempotency-1")
                .unwrap_or_else(|error| panic!("idempotency: {error:?}")),
        },
        expiry: TimestampSeconds(500),
    }
}

fn context(id: u8) -> ApprovalContext {
    ApprovalContext {
        actor_address: [4; 20],
        tenant: TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}")),
        agent: Did::new(b"0404040404040404040404040404040404040404")
            .unwrap_or_else(|error| panic!("agent: {error:?}")),
        session: SessionId([2; 32]),
        capability: CapabilityId([3; 32]),
        policy_version: "policy-v2".to_owned(),
        request_id: [id; 32],
    }
}

fn approver(value: &str) -> ApproverId {
    ApproverId::new(value).unwrap_or_else(|error| panic!("approver: {error:?}"))
}

#[test]
fn expiry_is_deterministic_and_never_approves() {
    let registry = ApprovalRegistry::default();
    let ticket = hold_at(
        &registry,
        context(1),
        prepared(b"canonical-1"),
        ApprovalSequence(10),
        ApprovalSequence(20),
        TimestampSeconds(10),
    )
    .unwrap_or_else(|error| panic!("hold: {error:?}"));
    assert_eq!(ticket.state, ApprovalState::AwaitingApproval);
    assert_eq!(
        expire_at(&registry, ApprovalSequence(19), TimestampSeconds(11))
            .map(|entries| entries.len()),
        Ok(0)
    );
    let expired = expire_at(&registry, ApprovalSequence(20), TimestampSeconds(11))
        .unwrap_or_else(|error| panic!("expire: {error:?}"));
    assert_eq!(expired.len(), 1);
    assert_eq!(expired[0].decision, ApprovalState::Expired);
    assert_eq!(expired[0].verification_level.wire_rank(), 0);
    assert_eq!(
        decide_at(
            &registry,
            ticket.hold_id,
            approver("operator-a"),
            ApprovalChoice::Approve,
            ticket.disclosure_digest,
            ApprovalSequence(20),
            TimestampSeconds(11)
        ),
        Err(ApprovalError::AlreadyDecided(ApprovalState::Expired))
    );
}

#[test]
fn approval_is_bound_to_the_disclosure_from_prepared_bytes() {
    let registry = ApprovalRegistry::default();
    let ticket = hold_at(
        &registry,
        context(2),
        prepared(b"canonical-2"),
        ApprovalSequence(10),
        ApprovalSequence(20),
        TimestampSeconds(10),
    )
    .unwrap_or_else(|error| panic!("hold: {error:?}"));
    assert_eq!(ticket.disclosure.activity_type, ActivityType(7));
    assert_eq!(ticket.disclosure.amounts.values()[0].amount, Amount(50));
    assert_eq!(
        decide_at(
            &registry,
            ticket.hold_id,
            approver("operator-a"),
            ApprovalChoice::Approve,
            digest(b"altered-request"),
            ApprovalSequence(11),
            TimestampSeconds(11)
        ),
        Err(ApprovalError::DisclosureChanged)
    );
    let audit = decide_at(
        &registry,
        ticket.hold_id,
        approver("operator-a"),
        ApprovalChoice::Approve,
        ticket.disclosure_digest,
        ApprovalSequence(11),
        TimestampSeconds(11),
    )
    .unwrap_or_else(|error| panic!("approve: {error:?}"));
    assert_eq!(audit.decision, ApprovalState::Approved);
    assert_eq!(
        audit.approver.as_ref().map(ApproverId::as_str),
        Some("operator-a")
    );
    assert_eq!(
        audit.disclosure_digest,
        prepared(b"canonical-2").disclosure.canonical_digest
    );
    assert_eq!(audit.idempotency_key, "idempotency-1");
}

#[test]
fn concurrent_approvals_have_exactly_one_winner() {
    let registry = Arc::new(ApprovalRegistry::default());
    let ticket = hold_at(
        &registry,
        context(3),
        prepared(b"canonical-3"),
        ApprovalSequence(10),
        ApprovalSequence(30),
        TimestampSeconds(10),
    )
    .unwrap_or_else(|error| panic!("hold: {error:?}"));
    let disclosure_digest = ticket.disclosure_digest;
    let mut workers = Vec::new();
    for index in 0..24 {
        let registry = Arc::clone(&registry);
        workers.push(thread::spawn(move || {
            decide_at(
                &registry,
                [3; 32],
                approver(&format!("operator-{index}")),
                ApprovalChoice::Approve,
                disclosure_digest,
                ApprovalSequence(11),
                TimestampSeconds(11),
            )
        }));
    }
    let accepted = workers
        .into_iter()
        .map(thread::JoinHandle::join)
        .filter(|result| result.as_ref().is_ok_and(Result::is_ok))
        .count();
    assert_eq!(accepted, 1);
    let audit = registry
        .audit_entry([3; 32])
        .unwrap_or_else(|error| panic!("audit: {error:?}"))
        .unwrap_or_else(|| panic!("audit entry missing"));
    assert_eq!(audit.decision, ApprovalState::Approved);
}

#[test]
fn unbound_disclosure_digest_is_refused_before_hold() {
    let registry = ApprovalRegistry::default();
    let mut altered = prepared(b"canonical-4");
    altered.disclosure.canonical_digest = [0; 32];
    assert_eq!(
        hold_at(
            &registry,
            context(4),
            altered,
            ApprovalSequence(10),
            ApprovalSequence(20),
            TimestampSeconds(10)
        ),
        Err(ApprovalError::InvalidDisclosureDigest)
    );
}

#[test]
fn replay_refuses_a_hold_whose_call_bytes_or_sender_changed() {
    use std::sync::Mutex;

    use layerx_agentd::budget::{
        reserve, BudgetLimiter, LimitConfig, LimitId, LimitScope, ReservationRequest,
    };
    use layerx_agentd::store::{ObjectKind, Store, TenantKey};
    use layerx_crypto::evm_transaction::decode_unsigned;
    use layerx_types::payload::ModuleRegistry;

    fn limiter() -> BudgetLimiter {
        BudgetLimiter::new(vec![LimitConfig {
            id: LimitId([9; 16]),
            name: "call-replay".to_owned(),
            scope: LimitScope::Tenant([9; 32]),
            ceiling: 1_000,
            consumed: 0,
        }])
        .unwrap_or_else(|error| panic!("limiter: {error:?}"))
    }

    for alteration in ["unchanged", "call", "sender", "previous", "previous-timed"] {
        let root = std::env::temp_dir().join(format!(
            "layerx-approval-call-replay-{alteration}-{}",
            std::process::id()
        ));
        let _ = std::fs::remove_dir_all(&root);
        let ctx = context(9);
        let tenant = ctx.tenant.clone();
        let original = prepared(b"canonical-9");
        let unsigned = original.unsigned_canonical_bytes.as_bytes();
        let call = decode_unsigned(unsigned).unwrap_or_else(|error| panic!("call: {error:?}"));
        assert_eq!(original.signing_preimage.as_bytes(), call.signing_hash());
        assert_eq!(call.encode_unsigned(), unsigned);
        let key = TenantKey::new(
            tenant.clone(),
            ObjectKind::PreparedActivity,
            [b"approval-hold-v1:".as_slice(), &[9; 32]].concat(),
        )
        .unwrap_or_else(|error| panic!("key: {error}"));
        {
            let store = Arc::new(Mutex::new(
                Store::open(&root).unwrap_or_else(|error| panic!("store: {error}")),
            ));
            let registry = ApprovalRegistry::with_store(Arc::clone(&store));
            let limiter = limiter();
            let reservation = reserve(
                &limiter,
                &ReservationRequest {
                    id: [9; 32],
                    amount: 50,
                    expiry_sequence: 100,
                    current_sequence: 10,
                    applicable_limits: vec![LimitId([9; 16])],
                },
            )
            .unwrap_or_else(|error| panic!("reserve: {error:?}"));
            hold_reserved_timed(
                &registry,
                ctx,
                original.clone(),
                ApprovalSequence(10),
                ApprovalSequence(100),
                &reservation,
                TimestampSeconds(10),
            )
            .unwrap_or_else(|error| panic!("hold: {error:?}"));
            let mut store = store.lock().unwrap_or_else(|_| panic!("store lock"));
            let mut record = store
                .get(&key)
                .unwrap_or_else(|| panic!("persisted hold"))
                .bytes()
                .to_vec();
            match alteration {
                "call" => {
                    let offsets = record
                        .windows(unsigned.len())
                        .enumerate()
                        .filter_map(|(index, value)| (value == unsigned).then_some(index))
                        .collect::<Vec<_>>();
                    assert_eq!(offsets.len(), 1);
                    let start = offsets[0];
                    let data_offsets = unsigned
                        .windows(call.data.len())
                        .enumerate()
                        .filter_map(|(index, value)| (value == call.data).then_some(index))
                        .collect::<Vec<_>>();
                    assert_eq!(data_offsets.len(), 1);
                    record[start + data_offsets[0] + call.data.len() - 1] ^= 1;
                    let changed = &record[start..start + unsigned.len()];
                    assert!(decode_unsigned(changed).is_ok());
                    let changed_digest = digest(changed);
                    let digest_offsets = record
                        .windows(32)
                        .enumerate()
                        .filter_map(|(index, value)| {
                            (value == original.disclosure.canonical_digest).then_some(index)
                        })
                        .collect::<Vec<_>>();
                    assert_eq!(digest_offsets.len(), 1);
                    record[digest_offsets[0]..digest_offsets[0] + 32]
                        .copy_from_slice(&changed_digest);
                }
                "sender" => {
                    let offsets = record
                        .windows(20)
                        .enumerate()
                        .filter_map(|(index, value)| (value == [4; 20]).then_some(index))
                        .collect::<Vec<_>>();
                    assert_eq!(offsets.len(), 1);
                    record[offsets[0]..offsets[0] + 20].copy_from_slice(&[6; 20]);
                }
                "previous" => record[..8].copy_from_slice(b"LXAPHLD2"),
                "previous-timed" => record[..8].copy_from_slice(b"LXAPHLD3"),
                "unchanged" => {}
                _ => unreachable!(),
            }
            store
                .update_local_batch(vec![(key, record)])
                .unwrap_or_else(|error| panic!("persist changed record: {error}"));
        }
        {
            let store = Arc::new(Mutex::new(
                Store::open(&root).unwrap_or_else(|error| panic!("restart store: {error}")),
            ));
            let restarted = ApprovalRegistry::with_store(store);
            let replay = restarted.replay_tenant(&tenant, &limiter());
            if matches!(alteration, "previous" | "previous-timed") {
                assert_eq!(replay, Err(ApprovalError::CorruptRecord));
            } else {
                assert_eq!(replay, Ok(1));
                let modules =
                    ModuleRegistry::new(&[]).unwrap_or_else(|error| panic!("registry: {error:?}"));
                let result = restarted.validate_registry(&tenant, &modules);
                if alteration == "unchanged" {
                    assert_eq!(result, Ok(()));
                    assert_eq!(
                        restarted
                            .ticket([9; 32])
                            .unwrap_or_else(|error| panic!("ticket: {error:?}"))
                            .unwrap_or_else(|| panic!("held ticket"))
                            .disclosure,
                        original.disclosure
                    );
                } else {
                    assert_eq!(result, Err(ApprovalError::CorruptRecord));
                }
            }
        }
        std::fs::remove_dir_all(root).unwrap_or_else(|error| panic!("remove fixture: {error}"));
    }
}
