use layerx_crypto::authority_grant::AuthorityGrant;
use layerx_crypto::onboarding::SponsoredRegistration;
use layerx_crypto::rotation::OwnerRotation;
use layerx_crypto::send::SendDebit;
use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
use layerx_crypto::signer::{LocalSigner, Signer};
use layerx_intents::{
    compile, BridgeDepositCredit, BridgeWithdrawRequest, BudgetCreate, BudgetDefund, BudgetFund,
    DidRegistration, DisclosureCheck, EvmPayoutBinding, Intent, IntentKind, KeyRotation,
    LxpReceive, LxpSend, NativeBudgetCreate, NativeCustodyCredit, NativeReceive,
    PayerGrantRegistration, RecoveryRegistration, SessionGrant, SessionRevoke,
};
use layerx_types::account::AccountId;
use layerx_types::activity::{Signature, TimestampBound};
use layerx_types::amount::Amount;
use layerx_types::ids::{AssetId, CheckpointId, Did, IdempotencyKey};
use layerx_types::intent::{
    ApprovalThreshold, AuthorityGrantId, AuthorizationSignature, BudgetId, ContextHash,
    DepositProofId, EvmAddress, GrantSchedule, NetworkId, PayerGrantId, PeriodLength,
    ProtocolVersion, PublicKey, PurposeHash, RecoveryRoot, RolloverPolicy, SendAuthorization,
    SendAuthorizationKind, Sequence, SessionRevocationReason, TimestampSeconds,
};
use layerx_types::payload::{
    ActivityType, ModuleId, ModuleRegistration, ModuleRegistry, PerpsPayload,
};
use layerx_wire::hash;
use std::future::Future;
use std::pin::pin;
use std::sync::Arc;
use std::task::{Context, Poll, Wake, Waker};

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

fn activity(module: ModuleId, ordinal: u16) -> ActivityType {
    ActivityType::new(module, ordinal)
        .unwrap_or_else(|error| panic!("activity {module:?}/{ordinal}: {error:?}"))
}

fn registry() -> ModuleRegistry {
    let governance = [
        activity(ModuleId::Governance, 1),
        activity(ModuleId::Governance, 2),
        activity(ModuleId::Governance, 3),
        activity(ModuleId::Governance, 4),
        activity(ModuleId::Governance, 5),
        activity(ModuleId::Governance, 6),
        activity(ModuleId::Governance, 8),
    ];
    let asset = [
        activity(ModuleId::Asset, 4),
        activity(ModuleId::Asset, 5),
        activity(ModuleId::Asset, 6),
        activity(ModuleId::Asset, 9),
    ];
    let budget = [
        activity(ModuleId::Budget, 1),
        activity(ModuleId::Budget, 2),
        activity(ModuleId::Budget, 4),
        activity(ModuleId::Budget, 7),
    ];
    let bridge = [activity(ModuleId::Bridge, 1), activity(ModuleId::Bridge, 2)];
    let perps = [activity(ModuleId::Perps, 9)];
    let registrations = [
        ModuleRegistration::new(ModuleId::Governance, &governance)
            .unwrap_or_else(|error| panic!("governance registry: {error:?}")),
        ModuleRegistration::new(ModuleId::Asset, &asset)
            .unwrap_or_else(|error| panic!("asset registry: {error:?}")),
        ModuleRegistration::new(ModuleId::Budget, &budget)
            .unwrap_or_else(|error| panic!("budget registry: {error:?}")),
        ModuleRegistration::new(ModuleId::Bridge, &bridge)
            .unwrap_or_else(|error| panic!("bridge registry: {error:?}")),
        ModuleRegistration::new(ModuleId::Perps, &perps)
            .unwrap_or_else(|error| panic!("perps registry: {error:?}")),
    ];
    ModuleRegistry::new(&registrations).unwrap_or_else(|error| panic!("module registry: {error:?}"))
}

fn did() -> Did {
    Did::new(b"did:layerx:human-compiler")
        .unwrap_or_else(|error| panic!("did construction: {error:?}"))
}

fn account(value: &str) -> AccountId {
    AccountId::parse(value).unwrap_or_else(|error| panic!("account {value}: {error:?}"))
}

fn owner() -> AccountId {
    account("agent:did:layerx:human-compiler:main")
}

fn recipient() -> AccountId {
    account("agent:did:layerx:recipient:main")
}

fn send_intent(entropy: [u8; 32]) -> Intent {
    let signer = LocalSigner::new(entropy);
    let debit = SendDebit {
        from: hash::account_id(&owner()).unwrap_or_else(|error| panic!("from: {error:?}")),
        to: hash::account_id(&recipient()).unwrap_or_else(|error| panic!("to: {error:?}")),
        asset: [2; 32],
        amount: u128::from(entropy[0]) + 1,
        source_sequence: 7,
        idempotency_key: entropy,
        expires_at: 1_010,
        context_hash: [5; 32],
        conditions: Vec::new(),
        authorization_kind: 1,
        network_id: 77,
        protocol_version: layerx_wire::limits::PROTOCOL_VERSION,
    };
    let message = debit
        .authorization_message()
        .unwrap_or_else(|error| panic!("authorization message: {error:?}"));
    let request = debit
        .signing_request(&message)
        .unwrap_or_else(|error| panic!("signing request: {error:?}"));
    let signature = run(signer.sign(request))
        .unwrap_or_else(|error| panic!("authorization signature: {error:?}"));
    let send = LxpSend::new(
        owner(),
        recipient(),
        AssetId::new([2; 32]),
        Amount::from_u128(u128::from(entropy[0]) + 1),
        Sequence::from_u64(7),
        IdempotencyKey::new(entropy),
        TimestampSeconds::from_u64(1_010),
        ContextHash::new([5; 32]),
        SendAuthorization::new(
            SendAuthorizationKind::Owner,
            PublicKey::new(signer.public_key()),
            AuthorizationSignature::new(*signature.as_bytes()),
        ),
        NetworkId::new(77).unwrap_or_else(|error| panic!("network: {error:?}")),
        ProtocolVersion::new(layerx_wire::limits::PROTOCOL_VERSION)
            .unwrap_or_else(|error| panic!("protocol: {error:?}")),
    )
    .unwrap_or_else(|error| panic!("send intent: {error:?}"));
    Intent::v1(IntentKind::LxpSend(send))
}

fn checked<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("live intent: {error:?}"))
}

fn perps_intent() -> Intent {
    Intent::v1(IntentKind::Perps(PerpsPayload::FundingTick {
        market_id: [0x11; 32],
    }))
}

#[allow(clippy::too_many_lines)]
fn live_intents() -> Vec<Intent> {
    let asset = || AssetId::new([2; 32]);
    let idempotency = || IdempotencyKey::new([3; 32]);
    let amount = || Amount::from_u128(25);
    let budget_account = || account("agent:did:layerx:human-compiler:budget:primary");
    let mut intents = vec![
        Intent::v1(IntentKind::DidRegistration(
            DidRegistration::new(did(), PublicKey::new([1; 32]))
                .unwrap_or_else(|error| panic!("registration: {error:?}")),
        )),
        Intent::v1(IntentKind::KeyRotation(
            KeyRotation::new(
                did(),
                PublicKey::new([2; 32]),
                TimestampBound::new(10, 20).unwrap_or_else(|error| panic!("window: {error:?}")),
                Sequence::from_u64(3),
            )
            .unwrap_or_else(|error| panic!("rotation: {error:?}")),
        )),
        Intent::v1(IntentKind::RecoveryRegistration(
            RecoveryRegistration::new(
                did(),
                RecoveryRoot::new([3; 32]),
                ApprovalThreshold::new(2).unwrap_or_else(|error| panic!("threshold: {error:?}")),
            )
            .unwrap_or_else(|error| panic!("recovery: {error:?}")),
        )),
        Intent::v1(IntentKind::EvmPayoutBinding(
            EvmPayoutBinding::new(
                did(),
                NetworkId::new(77).unwrap_or_else(|error| panic!("network: {error:?}")),
                EvmAddress::new([4; 20]),
                Signature::new(&[5; 65]).unwrap_or_else(|error| panic!("signature: {error:?}")),
            )
            .unwrap_or_else(|error| panic!("payout binding: {error:?}")),
        )),
        send_intent([4; 32]),
        Intent::v1(IntentKind::LxpReceive(
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
        Intent::v1(IntentKind::PayerGrantRegistration(
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
        Intent::v1(IntentKind::BudgetCreate(
            BudgetCreate::new(
                BudgetId::new([11; 32]),
                owner(),
                budget_account(),
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
        Intent::v1(IntentKind::BudgetFund(
            BudgetFund::new(
                BudgetId::new([11; 32]),
                owner(),
                budget_account(),
                asset(),
                amount(),
                idempotency(),
            )
            .unwrap_or_else(|error| panic!("budget fund: {error:?}")),
        )),
        Intent::v1(IntentKind::BudgetDefund(
            BudgetDefund::new(
                BudgetId::new([11; 32]),
                budget_account(),
                owner(),
                asset(),
                amount(),
                Sequence::from_u64(8),
                idempotency(),
            )
            .unwrap_or_else(|error| panic!("budget defund: {error:?}")),
        )),
        Intent::v1(IntentKind::BridgeDepositCredit(
            BridgeDepositCredit::new(
                DepositProofId::new([13; 32]),
                CheckpointId::new([14; 32]),
                account("system:treasury"),
                owner(),
                asset(),
                amount(),
                idempotency(),
            )
            .unwrap_or_else(|error| panic!("bridge deposit: {error:?}")),
        )),
        Intent::v2(IntentKind::BridgeWithdrawRequest(
            BridgeWithdrawRequest::new(
                CheckpointId::new([18; 32]),
                100,
                owner(),
                account("system:paxeer-withdrawals"),
                EvmAddress::new([16; 20]),
                asset(),
                amount(),
                idempotency(),
            )
            .unwrap_or_else(|error| panic!("bridge withdrawal: {error:?}")),
        )),
    ];

    let consent = checked(SponsoredRegistration::from_signed_consent(include_bytes!(
        "../../../../../tests/fixtures/authority/provider-subject/onboarding.activity"
    )));
    let issued = checked(issue_session_key(&SessionKeyRequest {
        fee_budget: None,
        purpose: SessionPurpose::Activity,
        grantor: [1; 32],
        session_public_key: [2; 32],
        not_before: 10,
        expires_at: Some(1_000),
        permitted_activity_types: (1..=6)
            .map(|ordinal| activity(ModuleId::Governance, ordinal))
            .collect(),
        revocation_sequence: Some(3),
    }));
    let credit =
        include_bytes!("../../../../../tests/fixtures/custody/paxeer-light-v1/custody.credit");
    let beneficiary: String = credit[139..171]
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    let receive = include_str!(
        "../../../../agent/crates/layerx-crypto/tests/fixtures/payments/native-1-6.hex"
    );
    let receive = receive
        .trim()
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| checked(u8::from_str_radix(checked(std::str::from_utf8(pair)), 16)))
        .collect::<Vec<_>>();
    intents.extend([
        Intent::v3(IntentKind::NativeOnboardingConsent(consent.consent.clone())),
        Intent::v3(IntentKind::NativeOnboarding(consent)),
        Intent::v3(IntentKind::NativeBudgetCreate(NativeBudgetCreate {
            budget_id: [0x31; 32],
            budget_account: account(&format!("agent:did:key:managed:budget:{}", "31".repeat(32))),
            asset: [0x42; 32],
            purpose: [0x53; 32],
            per_period_limit: 400,
            carry_cap: 0,
            initial_amount: 600,
            period_length_ms: 1000,
            period_start_ms: 2000,
            expiry_ms: 5000,
            revocation_sequence: 0,
            rollover: 1,
            source_account: account(&format!("agent:did:key:managed:asset:{}", "42".repeat(32))),
            source_sequence: 7,
        })),
        Intent::v3(IntentKind::NativeOwnerRotation(OwnerRotation::Announce {
            owner: checked(Did::new(b"did:key:managed")),
            pending_public_key: LocalSigner::new([21; 32]).public_key(),
            begin: 10,
            end: 20,
            effective_sequence: 3,
        })),
        Intent::v3(IntentKind::NativeAssetAccountOpen(asset())),
        Intent::v1(IntentKind::SessionGrant(checked(SessionGrant::new(
            issued.registration_payload,
            1000,
            [13; 32],
        )))),
        Intent::v1(IntentKind::AuthorityGrant(checked(AuthorityGrant::decode(
            include_bytes!(
            "../../../../agent/crates/layerx-crypto/tests/fixtures/authority-grant-capability.bin"
        ),
        )))),
        Intent::v1(IntentKind::SessionRevoke(checked(SessionRevoke::new(
            AuthorityGrantId::new([3; 32]),
            SessionRevocationReason::Paused,
            Sequence::from_u64(9),
        )))),
        Intent::v1(IntentKind::NativeCustodyCredit(checked(
            NativeCustodyCredit::new(
                credit,
                account("system:treasury"),
                account(&format!("agent:did:layerx:{beneficiary}:main")),
            ),
        ))),
        Intent::v1(IntentKind::NativeReceive(checked(NativeReceive::new(
            &receive,
        )))),
        perps_intent(),
    ]);
    intents
}

fn kind_index(kind: &IntentKind) -> usize {
    match kind {
        IntentKind::DidRegistration(_) => 0,
        IntentKind::NativeOnboardingConsent(_) => 1,
        IntentKind::NativeOnboarding(_) => 2,
        IntentKind::NativeBudgetCreate(_) => 3,
        IntentKind::NativeOwnerRotation(_) => 4,
        IntentKind::NativeAssetAccountOpen(_) => 5,
        IntentKind::KeyRotation(_) => 6,
        IntentKind::RecoveryRegistration(_) => 7,
        IntentKind::EvmPayoutBinding(_) => 8,
        IntentKind::SessionGrant(_) => 9,
        IntentKind::AuthorityGrant(_) => 10,
        IntentKind::SessionRevoke(_) => 11,
        IntentKind::LxpSend(_) => 12,
        IntentKind::LxpReceive(_) => 13,
        IntentKind::PayerGrantRegistration(_) => 14,
        IntentKind::BudgetCreate(_) => 15,
        IntentKind::BudgetFund(_) => 16,
        IntentKind::BudgetDefund(_) => 17,
        IntentKind::BridgeDepositCredit(_) => 18,
        IntentKind::NativeCustodyCredit(_) => 19,
        IntentKind::NativeReceive(_) => 20,
        IntentKind::BridgeWithdrawRequest(_) => 21,
        IntentKind::Perps(_) => 22,
    }
}

#[test]
fn every_remaining_intent_kind_compiles() {
    let registry = registry();
    let intents = live_intents();
    let mut covered = [false; 23];
    for intent in &intents {
        let index = kind_index(intent.kind());
        assert!(
            !covered[index],
            "duplicate intent kind: {:?}",
            intent.kind()
        );
        covered[index] = true;
        let compiled = compile(intent, &registry)
            .unwrap_or_else(|error| panic!("{:?} refused compilation: {error:?}", intent.kind()));
        assert!(matches!(
            compiled.activity_type().module(),
            ModuleId::Governance
                | ModuleId::Asset
                | ModuleId::Budget
                | ModuleId::Bridge
                | ModuleId::Perps
        ));
        assert_eq!(compiled.activity_type().module(), intent.module());
        assert_eq!(compiled.payload().activity_type().module(), intent.module());
        assert!(!compiled.payload().as_bytes().is_empty());
        checked(DisclosureCheck::verify(intent, &compiled));
    }
    assert!(
        covered.into_iter().all(|seen| seen),
        "every remaining kind must compile"
    );
}

#[test]
fn perps_intent_still_compiles_to_perps_module() {
    let intent = perps_intent();
    let expected = intent.kind().perps_payload().expect("Perps payload");
    let compiled = checked(compile(&intent, &registry()));
    assert_eq!(intent.module(), ModuleId::Perps);
    assert_eq!(
        compiled.activity_type(),
        activity(ModuleId::Perps, expected.ordinal())
    );
    assert_eq!(
        checked(PerpsPayload::from_payload(compiled.payload())),
        *expected
    );
    assert_eq!(compiled.payload().as_bytes(), checked(expected.encode()));
    checked(DisclosureCheck::verify(&intent, &compiled));
}
