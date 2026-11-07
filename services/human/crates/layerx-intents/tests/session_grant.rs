use std::fmt::Debug;

use layerx_crypto::authority_grant::NativeFeeBudget;
use layerx_crypto::session::{issue_session_key, SessionKeyRequest, SessionPurpose};
use layerx_intents::{compile, CompileField, DisclosureCheck, Intent, IntentKind, SessionGrant};
use layerx_types::payload::{ActivityType, ModuleId, ModuleRegistration, ModuleRegistry};

fn checked<T, E: Debug>(value: Result<T, E>) -> T {
    value.unwrap_or_else(|error| panic!("session grant: {error:?}"))
}

fn registry() -> ModuleRegistry {
    let kind = checked(ActivityType::new(ModuleId::Governance, 5));
    checked(ModuleRegistry::new(&[checked(ModuleRegistration::new(
        ModuleId::Governance,
        &[kind],
    ))]))
}

fn fee_budget() -> NativeFeeBudget {
    NativeFeeBudget {
        asset: [3; 32],
        maximum_per_activity: 4,
        maximum_total: 12,
        period_length: 10,
        maximum_per_period: 8,
        period_start: 10,
    }
}

fn session_grant(purpose: SessionPurpose, fee_budget: Option<NativeFeeBudget>) -> SessionGrant {
    let permitted_activity_types = match purpose {
        SessionPurpose::Activity => vec![checked(ActivityType::new(ModuleId::Governance, 5))],
        SessionPurpose::Authentication => Vec::new(),
    };
    let issued = checked(issue_session_key(&SessionKeyRequest {
        grantor: [1; 32],
        session_public_key: [2; 32],
        not_before: 10,
        expires_at: Some(1_000),
        permitted_activity_types,
        revocation_sequence: Some(3),
        fee_budget,
        purpose,
    }));
    checked(SessionGrant::new(
        issued.registration_payload,
        1_000,
        [13; 32],
    ))
}

#[test]
fn session_grant_compiles_and_discloses_under_v3() {
    let registry = registry();
    let paid = session_grant(SessionPurpose::Activity, Some(fee_budget()));
    for grant in [
        session_grant(SessionPurpose::Activity, None),
        paid.clone(),
        session_grant(SessionPurpose::Authentication, None),
        checked(paid.replacing([4; 32], [5; 32])),
    ] {
        let intent = Intent::v3(IntentKind::SessionGrant(grant));
        let compiled = checked(compile(&intent, &registry));
        assert_eq!(
            compiled.activity_type(),
            checked(ActivityType::new(ModuleId::Governance, 5))
        );
        assert_eq!(
            checked(DisclosureCheck::verify(&intent, &compiled)).canonical_payload(),
            compiled.payload().as_bytes()
        );
    }
}

#[test]
fn replacing_session_grant_is_refused_before_v3() {
    let registry = registry();
    let grant = checked(
        session_grant(SessionPurpose::Activity, Some(fee_budget())).replacing([4; 32], [5; 32]),
    );
    for intent in [
        Intent::v1(IntentKind::SessionGrant(grant.clone())),
        Intent::v2(IntentKind::SessionGrant(grant)),
    ] {
        let Err(error) = compile(&intent, &registry) else {
            panic!("replacing session grant compiled before v3");
        };
        assert_eq!(error.field, CompileField::Version);
    }
}
