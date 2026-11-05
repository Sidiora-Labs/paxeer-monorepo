//! Proves: DidRegistration, RecoveryRegistration, KeyRotation, BudgetCreate, BudgetFund, BudgetDefund and SendDebit -> IntentKind, compile, Payload and ledger_payment_call -> CompiledIntent::verify_disclosure, disclosure::call::bind_call, DisclosureCheck::verify_agent and RouteResolver::resolve.
//!
//! The onboarding, agent creation, agent controls and custody session callers
//! use the same consumer decoder exercised here. The event producer's send
//! travels through Intent::from; the journey resolver is compiled directly
//! from its production source, including its canonical transport codec.
//! All producers are constructed in process from real domain types; sends use
//! LocalSigner and SendDebit's signing and encoding functions. No ambient
//! configuration or external service supplies any producer. The anchor has no
//! persistent state: each observation compiles anew and independently decodes
//! the payload, including a corrupt delivery between successful observations.
//!
//! Repairs services/human/crates/layerx-intents/src/vocabulary.rs:
//! KeyRotation::new accepted a zero effective sequence, although the consumer
//! in custody/sessions.rs, SessionKeyProvisioner::announce_rotation, requires
//! a nonzero sequence. The producer now refuses it before compilation.

#[path = "../../layerx-human-service/src/journeys/resolver.rs"]
mod resolver;

use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

use layerx_crypto::disclosure::{call::bind_call, AmountRole, CounterpartyRole};
use layerx_crypto::send::{ledger_payment_call, CallOptions, SendDebit};
use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_intents::{
    compile, inspect_intent, BudgetCreate, BudgetDefund, BudgetFund, CompileErrorReason,
    CompileField, DidRegistration, DisclosureCheck, DisclosureCheckError, DisclosureField, Intent,
    IntentError, IntentErrorReason, IntentField, IntentKind, IntentKindTag, KeyRotation, LxpSend,
    RecoveryRegistration, RejectReason,
};
use layerx_types::account::AccountId;
use layerx_types::activity::TimestampBound;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, Did, IdempotencyKey};
use layerx_types::intent::{
    ApprovalThreshold, AuthorizationSignature, BudgetId, ContextHash, NetworkId, PeriodLength,
    ProtocolVersion, PublicKey, PurposeHash, RecoveryRoot, RolloverPolicy, SendAuthorization,
    SendAuthorizationKind, Sequence, TimestampSeconds,
};
use layerx_types::payload::{
    ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, Payload, PayloadError,
};
use layerx_wire::{decode::Decoder, hash};
use resolver::{
    BudgetCreation, BudgetRoute, Endpoint, Mechanism, MovementTerm, Relationship, RouteRequest,
    RouteResolver, SendRoute,
};

const ASSET: [u8; 32] = [0x34; 32];
const BUDGET: [u8; 32] = [0x12; 32];
const PURPOSE: [u8; 32] = [0x56; 32];

fn required<T, E: std::fmt::Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("human intents production path unavailable: {error:?}"))
}

struct ThreadWake(std::thread::Thread);
impl Wake for ThreadWake {
    fn wake(self: Arc<Self>) {
        self.0.unpark();
    }
}

fn run<F: Future>(future: F) -> F::Output {
    let mut future = pin!(future);
    let waker = Waker::from(Arc::new(ThreadWake(std::thread::current())));
    let mut context = Context::from_waker(&waker);
    loop {
        match future.as_mut().poll(&mut context) {
            Poll::Ready(value) => return value,
            Poll::Pending => std::thread::park(),
        }
    }
}

fn did() -> Did {
    required(Did::new(b"did:layerx:proof-human"))
}
fn owner() -> AccountId {
    required(AccountId::parse("agent:did:layerx:proof-human:main"))
}
fn recipient() -> AccountId {
    required(AccountId::parse("agent:did:layerx:proof-recipient:main"))
}
fn budget_account() -> AccountId {
    required(AccountId::parse(&format!(
        "agent:did:layerx:proof-human:budget:{}",
        "12".repeat(32)
    )))
}
fn activity(module: ModuleId, ordinal: u16) -> ActivityType {
    required(ActivityType::new(module, ordinal))
}
pub(crate) fn registry() -> ModuleRegistry {
    required(ModuleRegistry::new(&[
        required(ModuleRegistration::new(
            ModuleId::Governance,
            &[
                activity(ModuleId::Governance, 1),
                activity(ModuleId::Governance, 2),
                activity(ModuleId::Governance, 3),
            ],
        )),
        required(ModuleRegistration::new(
            ModuleId::Asset,
            &[activity(ModuleId::Asset, 5)],
        )),
        required(ModuleRegistration::new(
            ModuleId::Budget,
            &[
                activity(ModuleId::Budget, 1),
                activity(ModuleId::Budget, 2),
                activity(ModuleId::Budget, 8),
            ],
        )),
    ]))
}

fn budget_create(limit: u128) -> Result<BudgetCreate, IntentError> {
    BudgetCreate::new(
        BudgetId::new(BUDGET),
        owner(),
        budget_account(),
        AssetId::new(ASSET),
        Amount::from_u128(limit),
        required(PeriodLength::new(60)),
        RolloverPolicy::None,
        Amount::ZERO,
        PurposeHash::new(PURPOSE),
        TimestampSeconds::from_u64(2_000),
    )
}

pub(crate) fn signed_send() -> (LxpSend, SendRoute, Vec<u8>) {
    signed_send_for_network(125)
}

pub(crate) fn signed_send_for_network(network_id: u32) -> (LxpSend, SendRoute, Vec<u8>) {
    let signer = LocalSigner::new([0x42; 32]);
    let debit = SendDebit {
        from: required(hash::account_id(&owner())),
        to: required(hash::account_id(&recipient())),
        asset: ASSET,
        amount: 25,
        source_sequence: 7,
        idempotency_key: [0x21; 32],
        expires_at: 1_010,
        context_hash: [0x55; 32],
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id,
        protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
    };
    let canonical = required(debit.authorization_message());
    let signature = required(run(signer.sign(required(debit.signing_request(&canonical)))));
    let signed = required(debit.encode_signed(signer.public_key(), *signature.as_bytes()));
    let route = SendRoute {
        account_sequence: Sequence::from_u64(debit.source_sequence),
        idempotency_key: IdempotencyKey::new(debit.idempotency_key),
        expires_at: TimestampSeconds::from_u64(debit.expires_at),
        context_hash: ContextHash::new(debit.context_hash),
        authorization: SendAuthorization::new(
            SendAuthorizationKind::Owner,
            PublicKey::new(signer.public_key()),
            AuthorizationSignature::new(*signature.as_bytes()),
        ),
        network_id: required(NetworkId::new(debit.network_id)),
        protocol_version: required(ProtocolVersion::new(debit.protocol_version)),
    };
    let send = required(LxpSend::new(
        owner(),
        recipient(),
        AssetId::new(ASSET),
        Amount::from_u128(debit.amount),
        route.account_sequence,
        route.idempotency_key,
        route.expires_at,
        route.context_hash,
        route.authorization,
        route.network_id,
        route.protocol_version,
    ));
    (send, route, signed)
}

fn producers() -> Vec<(&'static str, Intent, ModuleId, u16, IntentKindTag)> {
    let primary = LocalSigner::new([0x41; 32]);
    let pending = LocalSigner::new([0x43; 32]);
    vec![
        (
            "onboarding/mod.rs: DidRegistration",
            IntentKind::DidRegistration(required(DidRegistration::new(
                did(),
                PublicKey::new(primary.public_key()),
            )))
            .into(),
            ModuleId::Governance,
            1,
            IntentKindTag::DidRegistration,
        ),
        (
            "agents/create.rs: RecoveryRegistration",
            IntentKind::RecoveryRegistration(required(RecoveryRegistration::new(
                did(),
                RecoveryRoot::new([0x65; 32]),
                required(ApprovalThreshold::new(2)),
            )))
            .into(),
            ModuleId::Governance,
            3,
            IntentKindTag::RecoveryRegistration,
        ),
        (
            "custody/sessions.rs: KeyRotation",
            IntentKind::KeyRotation(required(KeyRotation::new(
                did(),
                PublicKey::new(pending.public_key()),
                required(TimestampBound::new(1_000, 1_100)),
                Sequence::from_u64(9),
            )))
            .into(),
            ModuleId::Governance,
            2,
            IntentKindTag::KeyRotation,
        ),
        (
            "agents/controls.rs: BudgetCreate",
            IntentKind::BudgetCreate(required(budget_create(100))).into(),
            ModuleId::Budget,
            1,
            IntentKindTag::BudgetCreate,
        ),
        (
            "agents/create.rs: BudgetFund",
            IntentKind::BudgetFund(required(BudgetFund::new(
                BudgetId::new(BUDGET),
                owner(),
                budget_account(),
                AssetId::new(ASSET),
                Amount::from_u128(25),
                IdempotencyKey::new([0x21; 32]),
            )))
            .into(),
            ModuleId::Budget,
            2,
            IntentKindTag::BudgetFund,
        ),
        (
            "journeys/resolver.rs: BudgetDefund",
            IntentKind::BudgetDefund(required(BudgetDefund::new(
                BudgetId::new(BUDGET),
                budget_account(),
                owner(),
                AssetId::new(ASSET),
                Amount::from_u128(25),
                Sequence::from_u64(9),
                IdempotencyKey::new([0x21; 32]),
            )))
            .into(),
            ModuleId::Budget,
            8,
            IntentKindTag::BudgetDefund,
        ),
        (
            "event_producer_tls.rs: LxpSend",
            IntentKind::LxpSend(signed_send().0).into(),
            ModuleId::Asset,
            5,
            IntentKindTag::LxpSend,
        ),
    ]
}

fn assert_observed_fields(intent: &Intent, payload: &[u8]) {
    let mut decoder = Decoder::new(payload, 1024);
    match intent.kind() {
        IntentKind::DidRegistration(_) => {
            assert_eq!(required(decoder.u16()), 0x7101);
            assert_eq!(required(decoder.u16()), 2);
            assert_eq!(required(decoder.fixed(32)), required(hash::did_id(&did())));
            assert_eq!(
                required(decoder.fixed(32)),
                LocalSigner::new([0x41; 32]).public_key()
            );
        }
        IntentKind::RecoveryRegistration(_) => {
            assert_eq!(required(decoder.u16()), 0x7103);
            assert_eq!(required(decoder.u16()), 3);
            assert_eq!(required(decoder.fixed(32)), required(hash::did_id(&did())));
            assert_eq!(required(decoder.fixed(32)), [0x65; 32]);
            assert_eq!(required(decoder.u16()), 2);
        }
        IntentKind::KeyRotation(_) => {
            assert_eq!(required(decoder.u16()), 0x7102);
            assert_eq!(required(decoder.u16()), 4);
            assert_eq!(required(decoder.fixed(32)), required(hash::did_id(&did())));
            assert_eq!(
                required(decoder.fixed(32)),
                LocalSigner::new([0x43; 32]).public_key()
            );
            assert_eq!(required(decoder.u64()), 1_000);
            assert_eq!(required(decoder.u64()), 1_100);
            assert_eq!(required(decoder.u64()), 9);
        }
        IntentKind::BudgetCreate(_) => {
            assert_eq!(required(decoder.u16()), 1);
            assert_eq!(required(decoder.fixed(32)), BUDGET);
            assert_eq!(
                required(decoder.fixed(32)),
                required(hash::account_id_for_protocol(&budget_account(), 3))
            );
            assert_eq!(required(decoder.fixed(32)), ASSET);
            assert_eq!(required(decoder.fixed(32)), PURPOSE);
            assert_eq!(required(decoder.u128()), 100);
            assert_eq!(required(decoder.u128()), 0);
            assert_eq!(required(decoder.u128()), 100);
            assert_eq!(required(decoder.u64()), 60_000);
            assert_eq!(required(decoder.u64()), 0);
            assert_eq!(required(decoder.u64()), 2_000_000);
            assert_eq!(required(decoder.u64()), 0);
            assert_eq!(required(decoder.u8()), 1);
        }
        IntentKind::BudgetFund(_) | IntentKind::BudgetDefund(_) => {
            assert_eq!(required(decoder.u16()), 1);
            assert_eq!(required(decoder.fixed(32)), BUDGET);
            assert_eq!(required(decoder.u128()), 25);
        }
        IntentKind::LxpSend(_) => {
            assert_eq!(payload, signed_send().2);
            return;
        }
        other => panic!("missing producer assertion: {other:?}"),
    }
    required(decoder.finish());
}

#[test]
fn five_human_producers_cross_the_public_items_into_the_consumer_decoder_twice() {
    for pass in 0..2 {
        for (path, intent, module, ordinal, tag) in producers() {
            let registry = registry();
            assert_eq!(intent.kind().module(), module, "{path}, observation {pass}");
            assert_eq!(intent.module(), module, "{path}");
            assert_eq!(intent.kind().tag(), Some(tag), "{path}");
            let header = tag.header_bytes();
            let inspected = required(inspect_intent(&header));
            assert_eq!(inspected.version, 1);
            assert_eq!(inspected.kind, tag);
            let compiled = required(compile(&intent, &registry));
            assert_eq!(
                compiled.activity_type(),
                activity(module, ordinal),
                "{path}"
            );
            let observed = required(compiled.verify_disclosure(&intent));
            assert_eq!(observed.activity_type(), compiled.activity_type());
            assert_eq!(
                observed.payload_hash(),
                required(hash::payload_hash_for(compiled.payload()))
            );
            assert_eq!(observed.canonical_payload(), compiled.payload().as_bytes());
            assert_observed_fields(&intent, observed.canonical_payload());

            let bytes = compiled.payload().as_bytes();
            for end in [0, 1, bytes.len() - 1] {
                let malformed = required(Payload::new(
                    &registry,
                    compiled.activity_type(),
                    &bytes[..end],
                ));
                assert!(
                    compiled
                        .verify_received_payload(&intent, &malformed)
                        .is_err(),
                    "{path} accepted truncated producer output at {end}"
                );
            }
            let mut trailing = bytes.to_vec();
            trailing.push(0);
            let malformed = required(Payload::new(&registry, compiled.activity_type(), &trailing));
            assert!(
                compiled
                    .verify_received_payload(&intent, &malformed)
                    .is_err(),
                "{path} accepted trailing producer output"
            );
            let mut changed = bytes.to_vec();
            changed[bytes.len() - 1] ^= 1;
            let malformed = required(Payload::new(&registry, compiled.activity_type(), &changed));
            assert!(
                compiled
                    .verify_received_payload(&intent, &malformed)
                    .is_err(),
                "{path} accepted changed producer output"
            );
            assert_eq!(
                required(compiled.verify_disclosure(&intent)),
                observed,
                "{path} cached a corrupt result"
            );

            let empty = required(ModuleRegistry::new(&[]));
            let rejected = compile(&intent, &empty).expect_err("unknown activity must be refused");
            assert_eq!(rejected.field, CompileField::Payload);
            assert_eq!(
                rejected.reason,
                CompileErrorReason::Payload(PayloadError::UndeclaredActivity(
                    activity(module, ordinal).value()
                ))
            );
            for end in 0..header.len() {
                let rejection =
                    inspect_intent(&header[..end]).expect_err("truncated header accepted");
                assert!(matches!(rejection.reason(), RejectReason::Malformed(_)));
                assert_eq!(rejection.input(), &header[..end]);
            }
        }
    }
}

#[test]
fn consumer_keeps_the_original_producer_commitment_when_received_bytes_change() {
    let original = Intent::from(IntentKind::BudgetCreate(required(budget_create(100))));
    let changed = Intent::from(IntentKind::BudgetCreate(required(budget_create(101))));
    let first = required(compile(&original, &registry()));
    let second = required(compile(&changed, &registry()));
    assert_eq!(
        first.verify_received_payload(&changed, second.payload()),
        Err(DisclosureCheckError::FieldMismatch(
            DisclosureField::PayloadHash
        ))
    );
    assert!(first.verify_disclosure(&changed).is_err());
    let wrong_module = required(Payload::new(
        &registry(),
        activity(ModuleId::Governance, 1),
        first.payload().as_bytes(),
    ));
    assert_eq!(
        first.verify_received_payload(&original, &wrong_module),
        Err(DisclosureCheckError::FieldMismatch(
            DisclosureField::ActivityType
        ))
    );
    required(first.verify_disclosure(&original));
}

#[test]
fn signed_send_crosses_the_real_ledger_call_and_agent_disclosure_consumer_twice() {
    let (send, _, signed) = signed_send();
    let intent = Intent::from(IntentKind::LxpSend(send));
    let compiled = required(compile(&intent, &registry()));
    assert_eq!(compiled.payload().as_bytes(), signed);
    let options = CallOptions {
        from: [0x24; 20],
        chain_id: 125,
        nonce: 11,
        max_priority_fee_per_gas: 2,
        max_fee_per_gas: 7,
        gas_limit: 100_000,
    };
    let produced = required(ledger_payment_call(compiled.payload().as_bytes(), &options));
    for _ in 0..2 {
        let observed = required(bind_call(&produced.call, options.from));
        required(DisclosureCheck::verify_agent(&intent, &observed));
        assert_eq!(observed.activity_type, compiled.activity_type());
        assert_eq!(produced.call.nonce, 11);
        assert_eq!(required(observed.payload_sequence()), Some(7));
        assert_eq!(observed.actor, produced.disclosure.actor);
        assert_eq!(observed.counterparties.len(), 2);
        assert_eq!(observed.counterparties[0].role, CounterpartyRole::Payer);
        assert_eq!(
            observed.counterparties[0].account,
            required(hash::account_id(&owner()))
        );
        assert_eq!(observed.counterparties[1].role, CounterpartyRole::Recipient);
        assert_eq!(
            observed.counterparties[1].account,
            required(hash::account_id(&recipient()))
        );
        assert_eq!(observed.amounts.len(), 1);
        assert_eq!(observed.amounts[0].role, AmountRole::Transfer);
        assert_eq!(observed.amounts[0].value, 25);
        assert_eq!(observed.asset, ASSET);
        assert_eq!(observed.fee_limit, produced.disclosure.fee_limit);
        assert_eq!(observed, produced.disclosure);
        for end in [0, 1, produced.call.data.len() - 1] {
            let mut truncated = produced.call.clone();
            truncated.data.truncate(end);
            assert!(bind_call(&truncated, options.from).is_err());
        }
        let mut trailing = produced.call.clone();
        trailing.data.push(0);
        assert!(bind_call(&trailing, options.from).is_err());
        let mut corrupt = compiled.payload().as_bytes().to_vec();
        corrupt[0] ^= 1;
        assert!(ledger_payment_call(&corrupt, &options).is_err());
        assert_eq!(required(bind_call(&produced.call, options.from)), observed);
    }
}

#[test]
fn real_producers_refuse_invalid_keys_sequences_roots_limits_and_movements() {
    assert_eq!(
        DidRegistration::new(did(), PublicKey::new([0; 32])).expect_err("zero key"),
        IntentError {
            field: IntentField::PrimaryKey,
            reason: IntentErrorReason::Zero
        }
    );
    assert_eq!(
        RecoveryRegistration::new(
            did(),
            RecoveryRoot::new([0; 32]),
            required(ApprovalThreshold::new(2))
        )
        .expect_err("zero root"),
        IntentError {
            field: IntentField::RecoveryRoot,
            reason: IntentErrorReason::Zero
        }
    );
    let rotation = KeyRotation::new(
        did(),
        PublicKey::new(LocalSigner::new([0x43; 32]).public_key()),
        required(TimestampBound::new(1_000, 1_100)),
        Sequence::from_u64(0),
    );
    assert_eq!(
        rotation.expect_err("zero effective sequence must be refused at the producer"),
        IntentError {
            field: IntentField::EffectiveSequence,
            reason: IntentErrorReason::Zero
        }
    );
    assert_eq!(
        budget_create(0).expect_err("zero limit"),
        IntentError {
            field: IntentField::Amount,
            reason: IntentErrorReason::Zero
        }
    );
    assert!(BudgetFund::new(
        BudgetId::new(BUDGET),
        owner(),
        budget_account(),
        AssetId::new(ASSET),
        Amount::ZERO,
        IdempotencyKey::new([0x21; 32])
    )
    .is_err());
    assert!(BudgetDefund::new(
        BudgetId::new([0; 32]),
        budget_account(),
        owner(),
        AssetId::new(ASSET),
        Amount::from_u128(25),
        Sequence::from_u64(9),
        IdempotencyKey::new([0x21; 32])
    )
    .is_err());
    let (_, route, _) = signed_send();
    assert!(LxpSend::new(
        owner(),
        owner(),
        AssetId::new(ASSET),
        Amount::from_u128(25),
        route.account_sequence,
        route.idempotency_key,
        route.expires_at,
        route.context_hash,
        route.authorization,
        route.network_id,
        route.protocol_version
    )
    .is_err());
}

#[test]
fn production_journey_codec_and_resolver_consume_real_requests_twice_and_refuse_corruption() {
    let (_, send, _) = signed_send();
    let budget = BudgetRoute {
        budget_id: BudgetId::new(BUDGET),
        idempotency_key: send.idempotency_key,
        revocation_sequence: Sequence::from_u64(9),
        create: Some(BudgetCreation {
            per_period_limit: Amount::from_u128(100),
            period_length: required(PeriodLength::new(60)),
            rollover: RolloverPolicy::None,
            carry_cap: Amount::ZERO,
            purpose: PurposeHash::new(PURPOSE),
            expiry: TimestampSeconds::from_u64(2_000),
        }),
    };
    let cases = [
        (
            required(Endpoint::human(owner())),
            required(Endpoint::agent(recipient())),
            Relationship::Direct(send),
            vec![(MovementTerm::Transfer, Mechanism::Send)],
        ),
        (
            required(Endpoint::human(owner())),
            required(Endpoint::agent_budget(budget_account())),
            Relationship::ManagedBudget(budget),
            vec![
                (MovementTerm::Fund, Mechanism::BudgetCreate),
                (MovementTerm::Fund, Mechanism::BudgetFund),
            ],
        ),
        (
            required(Endpoint::agent_budget(budget_account())),
            required(Endpoint::human(owner())),
            Relationship::ManagedBudget(BudgetRoute {
                create: None,
                ..budget
            }),
            vec![(MovementTerm::Return, Mechanism::BudgetDefund)],
        ),
    ];
    for (source, destination, relationship, expected) in cases {
        let produced = required(RouteRequest::from_wire_parts(
            source,
            destination,
            relationship,
            AssetId::new(ASSET),
            Amount::from_u128(25),
        ));
        let transport = produced.canonical_encode();
        for _ in 0..2 {
            let received = required(RouteRequest::canonical_decode(&transport));
            assert_eq!(received, produced);
            let consumed = required(RouteResolver::resolve(&received));
            assert!(consumed.bank_transfers().is_empty());
            assert_eq!(
                consumed
                    .legs()
                    .iter()
                    .map(|leg| (leg.term(), leg.mechanism()))
                    .collect::<Vec<_>>(),
                expected
            );
            for leg in consumed.legs() {
                let compiled = required(compile(leg.intent(), &registry()));
                required(compiled.verify_disclosure(leg.intent()));
                if matches!(leg.intent().kind(), IntentKind::LxpSend(_)) {
                    assert_eq!(compiled.payload().as_bytes(), signed_send().2);
                } else {
                    assert_observed_fields(leg.intent(), compiled.payload().as_bytes());
                }
            }
            for end in [0, 1, transport.len() - 1] {
                assert!(RouteRequest::canonical_decode(&transport[..end]).is_err());
            }
            let mut malformed = transport.clone();
            malformed.push(0);
            assert!(RouteRequest::canonical_decode(&malformed).is_err());
        }
        let mut refused = produced;
        refused.amount = Amount::ZERO;
        assert!(RouteResolver::resolve(&refused).is_err());
        assert!(RouteRequest::canonical_decode(&refused.canonical_encode()).is_err());
    }
}

#[test]
fn inspection_refuses_retired_unknown_and_versioned_producer_headers_without_losing_bytes() {
    let header = IntentKindTag::LxpSend.header_bytes();
    for _ in 0..2 {
        for (kind, expected) in [
            (4, RejectReason::RetiredShape),
            (11, RejectReason::RetiredShape),
            (12, RejectReason::RetiredShape),
            (16, RejectReason::RetiredShape),
            (u16::MAX, RejectReason::UnknownKind(u16::MAX)),
        ] {
            let mut malformed = header;
            malformed[2..].copy_from_slice(&kind.to_be_bytes());
            let rejected =
                inspect_intent(&malformed).expect_err("retired or unknown kind accepted");
            assert_eq!(rejected.reason(), expected);
            assert_eq!(rejected.input(), malformed);
            assert_eq!(rejected.input().as_ptr(), malformed.as_ptr());
        }
        let mut unknown_version = header;
        unknown_version[..2].copy_from_slice(&2_u16.to_be_bytes());
        let rejected = inspect_intent(&unknown_version).expect_err("V2 envelope accepted");
        assert_eq!(rejected.reason(), RejectReason::UnknownVersion(2));
        assert_eq!(rejected.input(), unknown_version);
        assert_eq!(
            required(inspect_intent(&header)).kind,
            IntentKindTag::LxpSend
        );
    }
}
