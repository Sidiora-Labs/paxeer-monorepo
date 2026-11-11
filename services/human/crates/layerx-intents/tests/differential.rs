use layerx_crypto::session::{issue_session_key, SessionKeyRequest};
use layerx_intents::{
    compile, golden, BudgetCreate, BudgetDefund, BudgetFund, DidRegistration, DisclosureCheck,
    DisclosureCheckError, DisclosureField, Intent, IntentKind, IntentVersion, KeyRotation,
    LxpReceive, LxpSend, PayerGrantRegistration, RecoveryRegistration, SessionGrant, SessionRevoke,
};
use layerx_types::account::AccountId;
use layerx_types::activity::TimestampBound;
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, Did, IdempotencyKey};
use layerx_types::intent::{
    ApprovalThreshold, AuthorityGrantId, AuthorizationSignature, BudgetId, ContextHash,
    GrantSchedule, NetworkId, PayerGrantId, PeriodLength, ProtocolVersion, PublicKey, PurposeHash,
    RecoveryRoot, RolloverPolicy, SendAuthorization, SendAuthorizationKind, Sequence,
    SessionRevocationReason, TimestampSeconds,
};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};
use proptest::prelude::*;

struct Fixture {
    name: &'static str,
    source: &'static str,
    intent: Intent,
}

fn activity(module: ModuleId, ordinal: u16) -> ActivityType {
    ActivityType::new(module, ordinal)
        .unwrap_or_else(|error| panic!("activity {module:?}/{ordinal}: {error:?}"))
}

fn registry() -> ModuleRegistry {
    let registrations = [
        ModuleRegistration::new(
            ModuleId::Governance,
            &[
                activity(ModuleId::Governance, 1),
                activity(ModuleId::Governance, 2),
                activity(ModuleId::Governance, 3),
                activity(ModuleId::Governance, 5),
                activity(ModuleId::Governance, 6),
            ],
        )
        .unwrap_or_else(|error| panic!("governance: {error:?}")),
        ModuleRegistration::new(
            ModuleId::Asset,
            &[activity(ModuleId::Asset, 5), activity(ModuleId::Asset, 6)],
        )
        .unwrap_or_else(|error| panic!("asset: {error:?}")),
        ModuleRegistration::new(
            ModuleId::Budget,
            &[
                activity(ModuleId::Budget, 1),
                activity(ModuleId::Budget, 2),
                activity(ModuleId::Budget, 4),
                activity(ModuleId::Budget, 8),
            ],
        )
        .unwrap_or_else(|error| panic!("budget: {error:?}")),
    ];
    ModuleRegistry::new(&registrations).unwrap_or_else(|error| panic!("registry: {error:?}"))
}

fn did() -> Did {
    Did::new(b"did:layerx:golden").unwrap_or_else(|error| panic!("did construction: {error:?}"))
}

fn account(value: &str) -> AccountId {
    AccountId::parse(value).unwrap_or_else(|error| panic!("account {value}: {error:?}"))
}

fn owner() -> AccountId {
    account("agent:did:layerx:golden:main")
}

fn recipient() -> AccountId {
    account("agent:did:layerx:recipient:main")
}

fn budget() -> AccountId {
    account("agent:did:layerx:golden:budget:primary")
}

fn send(amount: u128, idempotency: [u8; 32]) -> Intent {
    Intent::v1(IntentKind::LxpSend(
        LxpSend::new(
            owner(),
            recipient(),
            AssetId::new([2; 32]),
            Amount::from_u128(amount),
            Sequence::from_u64(7),
            IdempotencyKey::new(idempotency),
            TimestampSeconds::from_u64(1_010),
            ContextHash::new([5; 32]),
            SendAuthorization::new(
                SendAuthorizationKind::Owner,
                PublicKey::new([6; 32]),
                AuthorizationSignature::new([7; 64]),
            ),
            NetworkId::new(77).unwrap_or_else(|error| panic!("network: {error:?}")),
            ProtocolVersion::new(1).unwrap_or_else(|error| panic!("protocol: {error:?}")),
        )
        .unwrap_or_else(|error| panic!("send: {error:?}")),
    ))
}

#[allow(clippy::too_many_lines)]
fn fixtures() -> Vec<Fixture> {
    let asset = || AssetId::new([2; 32]);
    let idempotency = || IdempotencyKey::new([3; 32]);
    let amount = || Amount::from_u128(25);
    let issued_session = issue_session_key(&SessionKeyRequest {
        fee_budget: None,
        purpose: layerx_crypto::session::SessionPurpose::Activity,
        grantor: [1; 32],
        session_public_key: [2; 32],
        not_before: 10,
        expires_at: Some(1_000),
        permitted_activity_types: (1..=6)
            .map(|ordinal| activity(ModuleId::Governance, ordinal))
            .collect(),
        revocation_sequence: Some(3),
    })
    .unwrap_or_else(|error| panic!("session grant: {error:?}"));
    vec![
        Fixture {
            name: "did_registration",
            source: "v1 did_registration did=did:layerx:golden primary_key=01x32",
            intent: Intent::v1(IntentKind::DidRegistration(
                DidRegistration::new(did(), PublicKey::new([1; 32]))
                    .unwrap_or_else(|error| panic!("registration: {error:?}")),
            )),
        },
        Fixture {
            name: "key_rotation",
            source: "v1 key_rotation did=did:layerx:golden pending_key=02x32 not_before=10 not_after=20 effective_sequence=3",
            intent: Intent::v1(IntentKind::KeyRotation(
                KeyRotation::new(
                    did(),
                    PublicKey::new([2; 32]),
                    TimestampBound::new(10, 20).unwrap_or_else(|error| panic!("window: {error:?}")),
                    Sequence::from_u64(3),
                )
                .unwrap_or_else(|error| panic!("rotation: {error:?}")),
            )),
        },
        Fixture {
            name: "recovery_registration",
            source: "v1 recovery_registration did=did:layerx:golden recovery_root=03x32 threshold=2",
            intent: Intent::v1(IntentKind::RecoveryRegistration(
                RecoveryRegistration::new(
                    did(),
                    RecoveryRoot::new([3; 32]),
                    ApprovalThreshold::new(2)
                        .unwrap_or_else(|error| panic!("threshold: {error:?}")),
                )
                .unwrap_or_else(|error| panic!("recovery: {error:?}")),
            )),
        },
        Fixture {
            name: "session_grant",
            source: "v1 session_grant canonical_authority_grant=0x2001 grantor=01x32 session_public_key=02x32 module_mask=128 ordinal_min=1 ordinal_max=6 not_before=10 expires=1000 revocation_sequence=3",
            intent: Intent::v1(IntentKind::SessionGrant(
                SessionGrant::new(issued_session.registration_payload, 1000, [13; 32])
                    .unwrap_or_else(|error| panic!("session intent: {error:?}")),
            )),
        },
        Fixture {
            name: "session_revoke",
            source: "v1 session_revoke grant_id=03x32 reason=paused effective_sequence=9",
            intent: Intent::v1(IntentKind::SessionRevoke(
                SessionRevoke::new(
                    AuthorityGrantId::new([3; 32]),
                    SessionRevocationReason::Paused,
                    Sequence::from_u64(9),
                )
                .unwrap_or_else(|error| panic!("session revoke: {error:?}")),
            )),
        },
        Fixture {
            name: "lxp_send",
            source: "v1 lxp_send from=agent:did:layerx:golden:main to=agent:did:layerx:recipient:main asset=02x32 amount=25 sequence=7 idempotency=03x32 expires=1010 context=05x32 authorization=owner public_key=06x32 signature=07x64 network=77 protocol=1",
            intent: send(25, [3; 32]),
        },
        Fixture {
            name: "lxp_receive",
            source: "v1 lxp_receive from=agent:did:layerx:golden:main to=agent:did:layerx:recipient:main asset=02x32 amount=25 grant=06x32 sequence=4 idempotency=03x32 context=07x32",
            intent: Intent::v1(IntentKind::LxpReceive(
                LxpReceive::new(
                    owner(),
                    recipient(),
                    asset(),
                    amount(),
                    PayerGrantId::new([6; 32]),
                    Sequence::from_u64(4),
                    idempotency(),
                    ContextHash::new([7; 32]),
                )
                .unwrap_or_else(|error| panic!("receive: {error:?}")),
            )),
        },
        Fixture {
            name: "payer_grant_registration",
            source: "v1 payer_grant_registration grant=08x32 from=agent:did:layerx:golden:main recipient=agent:did:layerx:recipient:main asset=02x32 per_draw=25 allowance=100 schedule=recurring:60 expiration=2000 purpose=09x32 public_key=0ax32",
            intent: Intent::v1(IntentKind::PayerGrantRegistration(
                PayerGrantRegistration::new(
                    PayerGrantId::new([8; 32]),
                    owner(),
                    recipient(),
                    asset(),
                    amount(),
                    Amount::from_u128(100),
                    GrantSchedule::Recurring(
                        PeriodLength::new(60).unwrap_or_else(|error| panic!("period: {error:?}")),
                    ),
                    TimestampSeconds::from_u64(2_000),
                    PurposeHash::new([9; 32]),
                    PublicKey::new([10; 32]),
                )
                .unwrap_or_else(|error| panic!("payer grant: {error:?}")),
            )),
        },
        Fixture {
            name: "budget_create",
            source: "v1 budget_create budget=0bx32 owner=agent:did:layerx:golden:main account=agent:did:layerx:golden:budget:primary asset=02x32 limit=100 period=86400 rollover=capped carry=50 purpose=0cx32 expiry=3000",
            intent: Intent::v1(IntentKind::BudgetCreate(
                BudgetCreate::new(
                    BudgetId::new([11; 32]),
                    owner(),
                    budget(),
                    asset(),
                    Amount::from_u128(100),
                    PeriodLength::new(86_400).unwrap_or_else(|error| panic!("period: {error:?}")),
                    RolloverPolicy::Capped,
                    Amount::from_u128(50),
                    PurposeHash::new([12; 32]),
                    TimestampSeconds::from_u64(3_000),
                )
                .unwrap_or_else(|error| panic!("budget create: {error:?}")),
            )),
        },
        Fixture {
            name: "budget_fund",
            source: "v1 budget_fund budget=0bx32 owner=agent:did:layerx:golden:main account=agent:did:layerx:golden:budget:primary asset=02x32 amount=25 idempotency=03x32",
            intent: Intent::v1(IntentKind::BudgetFund(
                BudgetFund::new(
                    BudgetId::new([11; 32]),
                    owner(),
                    budget(),
                    asset(),
                    amount(),
                    idempotency(),
                )
                .unwrap_or_else(|error| panic!("budget fund: {error:?}")),
            )),
        },
        Fixture {
            name: "budget_defund",
            source: "v1 budget_defund budget=0bx32 account=agent:did:layerx:golden:budget:primary owner=agent:did:layerx:golden:main asset=02x32 amount=25 revocation_sequence=8 idempotency=03x32",
            intent: Intent::v1(IntentKind::BudgetDefund(
                BudgetDefund::new(
                    BudgetId::new([11; 32]),
                    budget(),
                    owner(),
                    asset(),
                    amount(),
                    Sequence::from_u64(8),
                    idempotency(),
                )
                .unwrap_or_else(|error| panic!("budget defund: {error:?}")),
            )),
        },
    ]
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut encoded = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        encoded.push(char::from(DIGITS[usize::from(byte >> 4)]));
        encoded.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    encoded
}

pub(crate) fn assert_golden_vectors(source: &str, version: u16) {
    use std::collections::{BTreeMap, BTreeSet};

    let mut sections = source.split("\n[vector.");
    let metadata: BTreeMap<_, _> = sections
        .next()
        .unwrap_or_else(|| panic!("vector metadata"))
        .lines()
        .filter(|line| !line.is_empty() && !line.starts_with('#'))
        .map(|line| {
            line.split_once(" = ")
                .unwrap_or_else(|| panic!("metadata assignment"))
        })
        .collect();
    assert_eq!(metadata.len(), 2);
    assert_eq!(metadata["intent_version"], version.to_string());
    let expected_count: usize = metadata["vector_count"]
        .parse()
        .unwrap_or_else(|error| panic!("vector count: {error:?}"));
    let fixtures = fixtures();
    assert_eq!(expected_count, 11);
    assert_eq!(expected_count, fixtures.len());
    let registry = registry();
    let mut names = BTreeSet::new();
    for section in sections {
        let (name, body) = section
            .split_once("]\n")
            .unwrap_or_else(|| panic!("vector section header"));
        assert!(names.insert(name), "duplicate vector {name}");
        assert!(!matches!(
            name,
            "bridge_deposit_credit" | "bridge_withdraw_request" | "evm_payout_binding"
        ));
        let mut fields = BTreeMap::new();
        for line in body.lines().filter(|line| !line.is_empty()) {
            let (key, value) = line
                .split_once(" = ")
                .unwrap_or_else(|| panic!("vector assignment"));
            assert!(fields.insert(key, value).is_none(), "duplicate field {key}");
        }
        assert_eq!(fields.len(), 4, "{name} fields");
        let intent_source = fields["intent"]
            .strip_prefix('"')
            .and_then(|value| value.strip_suffix('"'))
            .unwrap_or_else(|| panic!("quoted intent"));
        let fixture = fixtures
            .iter()
            .find(|fixture| fixture.name == name && fixture.source == intent_source)
            .unwrap_or_else(|| panic!("unknown intent source {intent_source}"));
        assert_ne!(fixture.intent.module(), ModuleId::Bridge);
        let compiled = compile(&fixture.intent, &registry)
            .unwrap_or_else(|error| panic!("v{version} {name}: {error:?}"));
        assert_ne!(compiled.activity_type().module(), ModuleId::Bridge);
        assert_ne!(compiled.activity_type(), activity(ModuleId::Asset, 9));
        assert_eq!(
            fields["activity_type"],
            compiled.activity_type().value().to_string(),
            "v{version} {name} activity"
        );
        assert_eq!(
            fields["payload_hex"],
            format!("\"{}\"", hex(compiled.payload().as_bytes())),
            "v{version} {name} payload"
        );
        assert_eq!(
            fields["payload_hash_hex"],
            format!("\"{}\"", hex(&compiled.payload_hash())),
            "v{version} {name} hash"
        );
        DisclosureCheck::verify(&fixture.intent, &compiled)
            .unwrap_or_else(|error| panic!("v{version} {name} disclosure: {error:?}"));
    }
    assert_eq!(names.len(), expected_count);
}

fn render_vectors() -> String {
    let registry = registry();
    let fixtures = fixtures();
    let mut rendered = format!(
        "# Immutable golden vectors for IntentVersion::V2.\nintent_version = {}\nvector_count = {}\n",
        IntentVersion::V2.value(),
        fixtures.len()
    );
    for fixture in fixtures {
        let compiled = compile(&fixture.intent, &registry)
            .unwrap_or_else(|error| panic!("compile {}: {error:?}", fixture.name));
        write!(
            rendered,
            "\n[vector.{}]\nintent = \"{}\"\nactivity_type = {}\npayload_hex = \"{}\"\npayload_hash_hex = \"{}\"\n",
            fixture.name,
            fixture.source,
            compiled.activity_type().value(),
            hex(compiled.payload().as_bytes()),
            hex(&compiled.payload_hash()),
        )
        .unwrap_or_else(|error| panic!("render {}: {error}", fixture.name));
    }
    rendered
}

#[test]
fn budget_vectors_hold_the_kernel_canonical_payloads() {
    let registry = registry();
    let fixtures = fixtures();
    for name in ["budget_create", "budget_fund", "budget_defund"] {
        let fixture = fixtures
            .iter()
            .find(|fixture| fixture.name == name)
            .unwrap_or_else(|| panic!("missing fixture {name}"));
        let compiled = compile(&fixture.intent, &registry)
            .unwrap_or_else(|error| panic!("compile {name}: {error:?}"));
        let payload_hex = format!("\"{}\"", hex(compiled.payload().as_bytes()));
        let payload_hash_hex = format!("\"{}\"", hex(&compiled.payload_hash()));
        for (version, source) in [(1, golden::V1_SOURCE), (2, golden::V2_SOURCE)] {
            let header = format!("\n[vector.{name}]\n");
            let (_, remainder) = source
                .split_once(header.as_str())
                .unwrap_or_else(|| panic!("missing v{version} vector {name}"));
            let vector = remainder
                .split("\n[")
                .next()
                .unwrap_or_else(|| panic!("vector section must exist"));
            assert_eq!(
                vector
                    .lines()
                    .find_map(|line| line.strip_prefix("payload_hex = ")),
                Some(payload_hex.as_str()),
                "v{version} {name} payload",
            );
            assert_eq!(
                vector
                    .lines()
                    .find_map(|line| line.strip_prefix("payload_hash_hex = ")),
                Some(payload_hash_hex.as_str()),
                "v{version} {name} payload hash",
            );
        }
        if name == "budget_defund" {
            assert_eq!(compiled.payload().as_bytes().len(), 50);
            assert_eq!(&compiled.payload().as_bytes()[..2], &[0, 1]);
            assert_eq!(compiled.activity_type().value(), 196_616);
        }
    }
}

#[test]
fn v2_golden_vectors_are_byte_locked_to_the_intent_version() {
    assert_eq!(golden::V2_VERSION, IntentVersion::V2.value());
    let rendered = render_vectors();
    if std::env::var_os("LAYERX_PRINT_INTENT_VECTORS").is_some() {
        println!("{rendered}");
    } else {
        assert_eq!(rendered, golden::V2_SOURCE);
    }
}

#[test]
fn every_vector_decodes_matches_and_reencodes_byte_identically() {
    assert_golden_vectors(golden::V2_SOURCE, golden::V2_VERSION);
    let registry = registry();
    for fixture in fixtures() {
        let compiled = compile(&fixture.intent, &registry)
            .unwrap_or_else(|error| panic!("compile {}: {error:?}", fixture.name));
        let check = DisclosureCheck::verify(&fixture.intent, &compiled)
            .unwrap_or_else(|error| panic!("disclosure {}: {error:?}", fixture.name));
        assert_eq!(check.activity_type(), compiled.activity_type());
        assert_eq!(check.canonical_payload(), compiled.payload().as_bytes());
        assert_eq!(check.payload_hash(), compiled.payload_hash());
    }
}

#[test]
fn mismatch_aborts_on_the_exact_originating_intent_field() {
    let registry = registry();
    let originating = send(25, [3; 32]);
    let compiled = compile(&originating, &registry)
        .unwrap_or_else(|error| panic!("originating compile: {error:?}"));
    let altered = send(26, [3; 32]);
    assert_eq!(
        DisclosureCheck::verify(&altered, &compiled),
        Err(DisclosureCheckError::FieldMismatch(DisclosureField::Amount))
    );
}

proptest! {
    #[test]
    fn generated_sends_round_trip_without_semantic_drift(
        amount in 1_u128..=u128::MAX,
        idempotency in any::<[u8; 32]>(),
    ) {
        let intent = send(amount, idempotency);
        let compiled = compile(&intent, &registry())
            .map_err(|error| TestCaseError::fail(format!("compile: {error:?}")))?;
        let checked = DisclosureCheck::verify(&intent, &compiled)
            .map_err(|error| TestCaseError::fail(format!("disclosure: {error:?}")))?;
        prop_assert_eq!(checked.canonical_payload(), compiled.payload().as_bytes());
        prop_assert_eq!(checked.payload_hash(), compiled.payload_hash());
    }
}
use std::fmt::Write as _;
