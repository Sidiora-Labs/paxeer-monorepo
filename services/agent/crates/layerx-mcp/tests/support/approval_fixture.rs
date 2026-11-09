use crate::result_assertion;

use crate::test_clock;

use layerx_types::clock::Clock as _;

use layerx_agent_api::identity::{ActivityType, AgentDid, Asset, AuthorityRef, ExplicitSet};
use layerx_agent_api::prepare::{
    CanonicalBytes, DisclosedAmount, Disclosure, IdempotencyRef, PreparationRef, Prepared,
    SigningPreimage,
};
use layerx_agent_api::{Amount, TimestampSeconds};
use layerx_agentd::capability::CapabilityId;
use layerx_agentd::policy::approval::{ApprovalContext, ApprovalRegistry};
use layerx_agentd::session::SessionId;
use layerx_agentd::store::TenantId;
use layerx_types::ids::Did;
use sha2::{Digest, Sha256};

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn prepared(amount: u128, request_id: u8) -> Prepared {
    use layerx_agentd::prepare::evm::{self, AbiArgument, PrecompileCall, PrecompileTarget};
    use layerx_crypto::disclosure::call::ledger_selector;
    let mut amount_word = [0; 32];
    amount_word[16..].copy_from_slice(&amount.to_be_bytes());
    let transaction = result_assertion::required(
        evm::prepare_bound(
            evm::EvmPreparationState {
                chain_id: 125,
                pending_nonce: u64::from(request_id),
                base_fee_per_gas: 1,
                head_height: 50,
            },
            evm::PreparationDefaults {
                max_priority_fee_per_gas: 1,
                max_fee_per_gas: 5,
                maximum_payload_bytes: 65_536,
            },
            evm::PrepareRequest {
                from: [4; 20],
                call: PrecompileCall {
                    target: PrecompileTarget::Ledger,
                    selector: ledger_selector("mint(bytes32,bytes32,uint128)"),
                    arguments: vec![
                        AbiArgument::Word([3; 32]),
                        AbiArgument::Word([5; 32]),
                        AbiArgument::Word(amount_word),
                    ],
                },
                expected_nonce: Some(u64::from(request_id)),
                max_priority_fee_per_gas: None,
                max_fee_per_gas: None,
                declared_payload_limit: 65_536,
            },
            100_000,
        ),
        "real daemon preparation",
    );
    result_assertion::required(
        evm::verify_call_binding(&transaction),
        "producer call binding",
    );
    let source = &transaction.disclosure;
    let canonical = transaction.call.encode_unsigned();
    let actor = result_assertion::required(AgentDid::new(hex(&source.actor)), "sender-bound actor");
    let parties: Vec<_> = source
        .counterparties
        .iter()
        .map(|p| result_assertion::required(AgentDid::new(hex(&p.account)), "party"))
        .collect();
    Prepared {
        preparation_ref: result_assertion::required(
            PreparationRef::new(format!("preparation-{request_id}")),
            "preparation",
        ),
        unsigned_canonical_bytes: result_assertion::required(
            CanonicalBytes::new(canonical.clone()),
            "unsigned call",
        ),
        signing_preimage: result_assertion::required(
            SigningPreimage::new(transaction.signing_hash.to_vec()),
            "signing hash",
        ),
        disclosure: Disclosure {
            canonical_digest: Sha256::digest(&canonical).into(),
            activity_type: ActivityType(source.activity_type.ordinal()),
            actor,
            authority: result_assertion::required(
                AuthorityRef::new(hex(&source.authority)),
                "authority",
            ),
            counterparties: ExplicitSet::allow(parties.clone()),
            amounts: ExplicitSet::allow(
                source
                    .amounts
                    .iter()
                    .map(|a| DisclosedAmount {
                        counterparty: parties[0].clone(),
                        amount: Amount(a.value),
                    })
                    .collect(),
            ),
            asset: result_assertion::required(Asset::new(hex(&source.asset)), "asset"),
            fee_limit: Amount(source.fee_limit),
            expiry: TimestampSeconds(120),
            idempotency_key: result_assertion::required(
                IdempotencyRef::new(format!("idempotency-{request_id}")),
                "idempotency",
            ),
        },
        expiry: TimestampSeconds(120),
    }
}

fn hex(bytes: &[u8]) -> String {
    layerx_programs::hex::encode(bytes)
}

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn context(request_id: u8) -> ApprovalContext {
    ApprovalContext {
        actor_address: [4; 20],
        tenant: TenantId::new("tenant-a").unwrap_or_else(|error| panic!("tenant: {error}")),
        agent: Did::new(hex(&[4; 20]).as_bytes()).unwrap_or_else(|error| panic!("DID: {error:?}")),
        session: SessionId([7; 32]),
        capability: CapabilityId([9; 32]),
        policy_version: "policy-v1".to_owned(),
        request_id: [request_id; 32],
    }
}

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
pub fn assert_major_two_approval_facts(
    facts: &serde_json::Value,
    approval_id: &str,
    tx_hash: &str,
    rung: &str,
) {
    assert_eq!(facts["approval_id"], approval_id);
    assert_eq!(facts["tx_hash"], tx_hash);
    assert_eq!(facts["rung"], rung);
    assert!(facts.get("activity_id").is_none());
    assert!(facts.get("resulting_activity_id").is_none());
    assert!(facts.get("verification_level").is_none());
    assert!(facts.get("level").is_none());
}

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
#[must_use]
pub fn limiter() -> layerx_agentd::budget::BudgetLimiter {
    use layerx_agentd::budget::{BudgetLimiter, LimitConfig, LimitId, LimitScope};
    result_assertion::required(
        BudgetLimiter::new(vec![LimitConfig {
            id: LimitId([1; 16]),
            name: "approval".into(),
            scope: LimitScope::Tenant([1; 32]),
            ceiling: 1000,
            consumed: 0,
        }]),
        "reservation limiter",
    )
}

/// # Panics
/// Panics if the producer fixture, protocol assertion or required local resource is invalid.
pub fn reserved_hold(
    registry: &ApprovalRegistry,
    limiter: &layerx_agentd::budget::BudgetLimiter,
    request_id: u8,
) -> layerx_agentd::policy::approval::ApprovalTicket {
    use layerx_agentd::budget::{LimitId, ReservationRequest};
    result_assertion::required(
        layerx_agentd::policy::approval::reserve_and_hold_at(
            registry,
            limiter,
            context(request_id),
            prepared(101, request_id),
            &ReservationRequest {
                id: [request_id; 32],
                amount: 101,
                expiry_sequence: 120,
                current_sequence: 50,
                applicable_limits: vec![LimitId([1; 16])],
            },
            layerx_agentd::policy::approval::ApprovalSequence(120),
            layerx_agent_api::TimestampSeconds(
                result_assertion::required(
                    test_clock::TestClock::new(50).sample(std::time::Duration::from_secs(1)),
                    "clock",
                )
                .unix_seconds(),
            ),
        ),
        "durable reserved call hold",
    )
}
