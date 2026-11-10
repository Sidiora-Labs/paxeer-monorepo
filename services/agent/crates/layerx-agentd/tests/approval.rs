#[path = "support/send_authorization.rs"]
mod send_authorization;

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

pub mod fixture {
    use super::*;
    use layerx_agentd::policy::approval::{hold_at, ApprovalSequence, ApprovalTicket};
    use layerx_agentd::prepare::{
        prepare_bound, EvmPreparationDefaults, EvmPreparationState, EvmPrepareRequest,
        PrecompileCall,
    };
    use layerx_wire::encode::Encoder;

    #[must_use = "use the constructed fixture value"]
    /// # Panics
    /// Panics if the ledger payload cannot be encoded, prepared, or converted into the API fixture types.
    pub fn ledger_prepared(id: u8, expiry: u64) -> Prepared {
        let transaction = assert_ok(
            prepare_bound(
                EvmPreparationState {
                    chain_id: 125,
                    pending_nonce: u64::from(id),
                    base_fee_per_gas: 1,
                    head_height: 10,
                },
                EvmPreparationDefaults {
                    max_priority_fee_per_gas: 1,
                    max_fee_per_gas: 3,
                    maximum_payload_bytes: 4096,
                },
                EvmPrepareRequest {
                    from: [4; 20],
                    call: PrecompileCall::ledger_transfer(send_authorization::sign(
                        ledger_payload(id, expiry),
                    )),
                    expected_nonce: Some(u64::from(id)),
                    max_priority_fee_per_gas: None,
                    max_fee_per_gas: None,
                    declared_payload_limit: 4096,
                },
                100_000,
            ),
            "unexpected error",
        );
        let source = &transaction.disclosure;
        let canonical = transaction.call.encode_unsigned();
        let recipient = assert_some(
            source
                .counterparties
                .iter()
                .find(|party| party.role == layerx_crypto::disclosure::CounterpartyRole::Recipient),
            "expected Some value",
        );
        let recipient = assert_ok(
            AgentDid::new(hex::encode(recipient.account)),
            "unexpected error",
        );
        let disclosure = Disclosure {
            canonical_digest: digest(&canonical),
            activity_type: ActivityType(source.activity_type.ordinal()),
            actor: assert_ok(
                AgentDid::new(hex::encode(&source.actor)),
                "unexpected error",
            ),
            authority: assert_ok(
                AuthorityRef::new(hex::encode(&source.authority)),
                "unexpected error",
            ),
            counterparties: ExplicitSet::allow(
                source
                    .counterparties
                    .iter()
                    .map(|party| {
                        assert_ok(
                            AgentDid::new(hex::encode(party.account)),
                            "unexpected error",
                        )
                    })
                    .collect(),
            ),
            amounts: ExplicitSet::allow(
                source
                    .amounts
                    .iter()
                    .map(|amount| DisclosedAmount {
                        counterparty: recipient.clone(),
                        amount: Amount(amount.value),
                    })
                    .collect(),
            ),
            asset: assert_ok(Asset::new(hex::encode(source.asset)), "unexpected error"),
            fee_limit: Amount(source.fee_limit),
            expiry: TimestampSeconds(source.expiry.payload_expires_at),
            idempotency_key: assert_ok(
                IdempotencyRef::new(hex::encode(source.idempotency_key)),
                "unexpected error",
            ),
        };
        Prepared {
            preparation_ref: assert_ok(
                PreparationRef::new(format!("prepared-{id}")),
                "unexpected error",
            ),
            unsigned_canonical_bytes: assert_ok(CanonicalBytes::new(canonical), "unexpected error"),
            signing_preimage: assert_ok(
                SigningPreimage::new(transaction.signing_hash.to_vec()),
                "unexpected error",
            ),
            expiry: disclosure.expiry,
            disclosure,
        }
    }

    fn ledger_payload(id: u8, expiry: u64) -> Vec<u8> {
        let mut payload = Encoder::new(512);
        assert_ok(payload.u16(0x5301), "unexpected error");
        assert_ok(payload.u16(10), "unexpected error");
        assert_ok(payload.fixed(&[0x11; 32]), "unexpected error");
        assert_ok(payload.fixed(&[0x22; 32]), "unexpected error");
        assert_ok(payload.fixed(&[0x33; 32]), "unexpected error");
        assert_ok(payload.u128(50), "unexpected error");
        assert_ok(payload.u64(u64::from(id)), "unexpected error");
        assert_ok(payload.fixed(&[id; 32]), "unexpected error");
        assert_ok(payload.u64(expiry), "unexpected error");
        assert_ok(payload.fixed(&[0x55; 32]), "unexpected error");
        assert_ok(payload.u8(0), "unexpected error");
        assert_ok(payload.u8(1), "unexpected error");
        assert_ok(payload.fixed(&[0x11; 32]), "unexpected error");
        assert_ok(payload.fixed(&[0x66; 32]), "unexpected error");
        assert_ok(payload.fixed(&[0x77; 64]), "unexpected error");
        assert_ok(payload.fixed(&[0x55; 32]), "unexpected error");
        assert_ok(payload.u32(125), "unexpected error");
        assert_ok(
            payload.u16(layerx_wire::limits::PROTOCOL_VERSION),
            "unexpected error",
        );
        payload.finish()
    }

    /// # Errors
    /// Returns an error if the payload or approval state fails the production validation checks.
    pub fn hold(
        registry: &ApprovalRegistry,
        context: ApprovalContext,
        prepared: Prepared,
        sequence: u64,
        expiry: u64,
    ) -> Result<ApprovalTicket, ApprovalError> {
        hold_at(
            registry,
            context,
            prepared,
            ApprovalSequence(sequence),
            ApprovalSequence(expiry),
            TimestampSeconds(10),
        )
    }

    #[track_caller]
    fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
        match result {
            Ok(value) => value,
            Err(error) => panic!("{message}: {error:?}"),
        }
    }

    #[track_caller]
    fn assert_some<T>(value: Option<T>, message: &str) -> T {
        match value {
            Some(value) => value,
            None => panic!("{message}"),
        }
    }
}

fn prepared(label: &[u8]) -> Prepared {
    fixture::ledger_prepared(label.last().copied().unwrap_or(1), 500)
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
    assert_eq!(ticket.disclosure.activity_type, ActivityType(5));
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
    assert_eq!(audit.idempotency_key, hex::encode([b'2'; 32]));
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

    use layerx_agentd::budget::{reserve, LimitId, ReservationRequest};
    use layerx_agentd::store::{ObjectKind, Store, TenantKey};
    use layerx_crypto::evm_transaction::decode_unsigned;
    use layerx_types::payload::ModuleRegistry;

    for alteration in ["unchanged", "call", "sender", "previous", "previous-timed"] {
        let root = std::env::temp_dir().join(format!(
            "layerx-approval-call-replay-{alteration}-{}-{}",
            std::process::id(),
            module_path!()
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
            let limiter = replay_limiter();
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
            alter_replay_record(alteration, &mut record, unsigned, &call, &original);
            store
                .update_local_batch(vec![(key, record)])
                .unwrap_or_else(|error| panic!("persist changed record: {error}"));
        }
        {
            let store = Arc::new(Mutex::new(
                Store::open(&root).unwrap_or_else(|error| panic!("restart store: {error}")),
            ));
            let restarted = ApprovalRegistry::with_store(store);
            let replay = restarted.replay_tenant(&tenant, &replay_limiter());
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

#[test]
fn ledger_preparation_projects_real_call_and_refuses_semantic_changes() {
    use layerx_crypto::disclosure::call::{bind_call, validate_call, validate_canonical};
    let prepared = fixture::ledger_prepared(21, 500);
    let call = assert_ok(
        layerx_crypto::evm_transaction::decode_unsigned(
            prepared.unsigned_canonical_bytes.as_bytes(),
        ),
        "unexpected error",
    );
    let source = assert_ok(bind_call(&call, [4; 20]), "unexpected error");
    assert_eq!(
        call.to,
        layerx_agentd::prepare::evm::PrecompileTarget::Ledger.address()
    );
    assert_eq!(
        prepared.disclosure.asset.as_str(),
        hex::encode(source.asset)
    );
    assert_eq!(prepared.disclosure.fee_limit, Amount(source.fee_limit));
    assert_eq!(
        prepared.disclosure.expiry,
        TimestampSeconds(source.expiry.payload_expires_at)
    );
    assert_eq!(
        prepared.disclosure.counterparties.values().len(),
        source.counterparties.len()
    );
    assert_eq!(
        prepared.disclosure.amounts.values()[0].amount,
        Amount(source.amounts[0].value)
    );
    assert_eq!(
        prepared.signing_preimage.as_bytes(),
        assert_ok(source.signing_digest(), "unexpected error")
    );
    assert_eq!(
        validate_canonical(&source, prepared.unsigned_canonical_bytes.as_bytes()),
        Ok(())
    );
    let mut changed_call = call.clone();
    changed_call.nonce += 1;
    assert!(validate_call(&changed_call, &source, [4; 20]).is_err());
    assert!(validate_canonical(&source, &changed_call.encode_unsigned()).is_err());
    let mut changed_disclosure = source.clone();
    changed_disclosure.amounts[0].value += 1;
    assert!(validate_call(&call, &changed_disclosure, [4; 20]).is_err());
    assert!(validate_canonical(
        &changed_disclosure,
        prepared.unsigned_canonical_bytes.as_bytes()
    )
    .is_err());
}

#[track_caller]
fn assert_ok<T, E: std::fmt::Debug>(result: Result<T, E>, message: &str) -> T {
    match result {
        Ok(value) => value,
        Err(error) => panic!("{message}: {error:?}"),
    }
}

fn alter_replay_record(
    alteration: &str,
    record: &mut [u8],
    unsigned: &[u8],
    call: &layerx_crypto::evm_transaction::Eip1559Call,
    original: &Prepared,
) {
    use layerx_crypto::evm_transaction::decode_unsigned;
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
            record[digest_offsets[0]..digest_offsets[0] + 32].copy_from_slice(&changed_digest);
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
}

fn replay_limiter() -> layerx_agentd::budget::BudgetLimiter {
    use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
    BudgetLimiter::new(vec![LimitConfig {
        id: LimitId([9; 16]),
        name: "call-replay".to_owned(),
        scope: LimitScope::Tenant([9; 32]),
        ceiling: 1_000,
        consumed: 0,
    }])
    .unwrap_or_else(|error| panic!("limiter: {error:?}"))
}
