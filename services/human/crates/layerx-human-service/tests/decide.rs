use layerx_agent_api::track::SubmissionState;
use layerx_human_test_support as support;

#[allow(dead_code)]
pub(crate) mod owner {
    include!("support/approval_owner.rs");
}

#[allow(unused_imports)]
use owner::hex;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_human_service::approvals::{
    AgentApprovalState, AgentDecisionResolution, AgentDecisionStatus, ApprovalBoundary,
    ApprovalBoundaryError, DecisionError, Decisions,
};
use layerx_human_service::audit::{verify_export, ApprovalOutcome, AuditChain, AuditEvent};
use layerx_human_service::auth::{
    AccountIdentity, AuthConfig, Device, OperationDigest, Passkeys, RateLimit, SessionGrant,
};
use layerx_human_service::store::{PrincipalScope, PrincipalStore};
use layerx_human_service::trace::TraceId;
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use std::fmt::Display;

#[track_caller]
fn checked<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
    match value {
        Ok(value) => value,
        Err(error) => panic!("decision integration: {error:?}"),
    }
}

const RP_ID: &str = "paxportwallet.com";
const ORIGIN: &str = "https://paxportwallet.com";
const FLAG_UP: u8 = 1;
const FLAG_UV: u8 = 1 << 2;
const FLAG_AT: u8 = 1 << 6;

fn required<T, E: Display>(result: Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error}"))
}

fn auth_config() -> AuthConfig {
    AuthConfig {
        rp_id: RP_ID.to_owned(),
        rp_name: "LayerX".to_owned(),
        origin: ORIGIN.to_owned(),
        ceremony_ttl_secs: 300,
        assertion_ttl_secs: 30,
        session_ttl_secs: 60,
        refresh_ttl_secs: 300,
        step_up_ttl_secs: 10,
        rate_limit: RateLimit {
            attempts: 100,
            window_secs: 60,
        },
    }
}

fn decode_ceremony(value: &str) -> Value {
    let bytes = required(URL_SAFE_NO_PAD.decode(value), "decode ceremony");
    required(serde_json::from_slice(&bytes), "parse ceremony")
}

fn required_text<'a>(value: &'a Value, pointer: &str) -> &'a str {
    value
        .pointer(pointer)
        .and_then(Value::as_str)
        .unwrap_or_else(|| panic!("missing ceremony field {pointer}"))
}

fn encode_response(value: &Value) -> String {
    URL_SAFE_NO_PAD.encode(required(serde_json::to_vec(value), "serialize response"))
}

struct SoftwareAuthenticator {
    signing_key: SigningKey,
    credential_id: Vec<u8>,
    counter: u32,
    user_handle: Option<String>,
}

impl SoftwareAuthenticator {
    fn new() -> Self {
        let mut seed = [0_u8; 32];
        required(getrandom::fill(&mut seed), "authenticator entropy");
        let mut credential_id = vec![0_u8; 32];
        required(
            getrandom::fill(&mut credential_id),
            "credential identifier entropy",
        );
        Self {
            signing_key: SigningKey::from_bytes(&seed),
            credential_id,
            counter: 0,
            user_handle: None,
        }
    }

    fn register(&mut self, ceremony: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.user_handle = Some(required_text(&options, "/user/id").to_owned());
        let client_data = client_data("webauthn.create", challenge);
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "transports": ["internal"],
            "attestationObject": URL_SAFE_NO_PAD.encode(self.attestation_object()),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
        }))
    }

    fn assert(&mut self, ceremony: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.counter = self.counter.saturating_add(1);
        let authenticator_data = self.authenticator_data(self.counter, false);
        let client_data = client_data("webauthn.get", challenge);
        let client_hash = Sha256::digest(&client_data);
        let mut signed = Vec::with_capacity(authenticator_data.len() + client_hash.len());
        signed.extend_from_slice(&authenticator_data);
        signed.extend_from_slice(&client_hash);
        let signature = self.signing_key.sign(&signed).to_bytes();
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "authenticatorData": URL_SAFE_NO_PAD.encode(authenticator_data),
            "signature": URL_SAFE_NO_PAD.encode(signature),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
            "userHandle": self.user_handle,
        }))
    }

    fn attestation_object(&self) -> Vec<u8> {
        let map = CborValue::Map(vec![
            (
                CborValue::Text("fmt".to_owned()),
                CborValue::Text("none".to_owned()),
            ),
            (
                CborValue::Text("attStmt".to_owned()),
                CborValue::Map(Vec::new()),
            ),
            (
                CborValue::Text("authData".to_owned()),
                CborValue::Bytes(self.authenticator_data(0, true)),
            ),
        ]);
        let mut bytes = Vec::new();
        required(
            ciborium::ser::into_writer(&map, &mut bytes),
            "encode attestation",
        );
        bytes
    }

    fn authenticator_data(&self, counter: u32, attested: bool) -> Vec<u8> {
        let mut bytes = Vec::new();
        bytes.extend_from_slice(&Sha256::digest(RP_ID.as_bytes()));
        bytes.push(FLAG_UP | FLAG_UV | if attested { FLAG_AT } else { 0 });
        bytes.extend_from_slice(&counter.to_be_bytes());
        if attested {
            bytes.extend_from_slice(&[0_u8; 16]);
            let credential_length = u16::try_from(self.credential_id.len())
                .unwrap_or_else(|_| panic!("credential identifier too long"));
            bytes.extend_from_slice(&credential_length.to_be_bytes());
            bytes.extend_from_slice(&self.credential_id);
            bytes.extend_from_slice(&self.cose_public_key());
        }
        bytes
    }

    fn cose_public_key(&self) -> Vec<u8> {
        let public_key = self.signing_key.verifying_key().to_bytes().to_vec();
        let map = CborValue::Map(vec![
            (CborValue::Integer(1.into()), CborValue::Integer(1.into())),
            (
                CborValue::Integer(3.into()),
                CborValue::Integer((-8).into()),
            ),
            (
                CborValue::Integer((-1).into()),
                CborValue::Integer(6.into()),
            ),
            (
                CborValue::Integer((-2).into()),
                CborValue::Bytes(public_key),
            ),
        ]);
        let mut bytes = Vec::new();
        required(
            ciborium::ser::into_writer(&map, &mut bytes),
            "encode public key",
        );
        bytes
    }
}

fn client_data(kind: &str, challenge: &str) -> Vec<u8> {
    required(
        serde_json::to_vec(&json!({
            "type": kind,
            "challenge": challenge,
            "origin": ORIGIN,
            "crossOrigin": false,
        })),
        "encode client data",
    )
}

fn register_session(
    passkeys: &Passkeys,
    scope: &mut PrincipalScope<'_>,
    authenticator: &mut SoftwareAuthenticator,
    now: u64,
) -> SessionGrant {
    let identity = required(
        AccountIdentity::new("mara@example.com", "Mara"),
        "account identity",
    );
    let registration = required(
        passkeys.begin_registration(scope, &identity, "Phone passkey", now),
        "begin registration",
    );
    let response = authenticator.register(&registration.ceremony);
    required(
        passkeys.finish_registration(scope, &registration.registration_id, &response, now + 1),
        "finish registration",
    );
    open_session(
        passkeys,
        scope,
        authenticator,
        "dev_aabbccddeeff00112233445566778891",
        now + 2,
    )
}

fn open_session(
    passkeys: &Passkeys,
    scope: &mut PrincipalScope<'_>,
    authenticator: &mut SoftwareAuthenticator,
    device_id: &str,
    now: u64,
) -> SessionGrant {
    let challenge = required(passkeys.begin_assertion(scope, now), "begin assertion");
    let response = authenticator.assert(&challenge.ceremony);
    required(
        passkeys.finish_assertion(scope, &challenge.assertion_id, &response, now + 1),
        "finish assertion",
    );
    let device = required(Device::new(device_id, "Phone", "mobile"), "device");
    required(
        passkeys.open_session(scope, &challenge.assertion_id, device, now + 2),
        "open session",
    )
}

struct LostReplyContext<'a, 'scope> {
    fixture: &'a owner::Fixture,
    scope: &'a mut PrincipalScope<'scope>,
    passkeys: &'a Passkeys,
    authenticator: &'a mut SoftwareAuthenticator,
    first_session: &'a SessionGrant,
    boundary: &'a mut layerx_human_service::server::agent_runtime::AgentRuntime,
    audit: &'a mut AuditChain,
    sequence: u64,
}

fn assert_digest_bound_lost_reply(
    context: LostReplyContext<'_, '_>,
    first_digest: [u8; 32],
    second_digest: [u8; 32],
) {
    let LostReplyContext {
        fixture,
        scope,
        passkeys,
        authenticator,
        first_session,
        boundary,
        audit,
        sequence,
    } = context;
    let wrong_challenge = required(
        passkeys.begin_step_up(
            &mut *scope,
            first_session.access_token().expose(),
            first_session.csrf_token().expose(),
            OperationDigest::new(second_digest),
            110,
        ),
        "begin wrong step-up",
    );
    let wrong_assertion = authenticator.assert(&wrong_challenge.ceremony);
    let wrong_evidence = required(
        passkeys.finish_step_up(
            &mut *scope,
            &wrong_challenge.challenge_id,
            &wrong_assertion,
            111,
        ),
        "finish wrong step-up",
    );
    assert!(Decisions::approve(
        &mut *scope,
        passkeys,
        first_session.access_token().expose(),
        first_session.csrf_token().expose(),
        &wrong_evidence,
        &mut *boundary,
        [1; 32],
        "approve-first-wrong",
        sequence,
        112,
        &mut *audit,
        &TraceId::mint([1; 16])
    )
    .is_err());
    fixture.assert_reservations(2, 51, 0);
    assert!(checked(audit.entries(scope)).is_empty());

    let challenge = required(
        passkeys.begin_step_up(
            &mut *scope,
            first_session.access_token().expose(),
            first_session.csrf_token().expose(),
            OperationDigest::new(first_digest),
            113,
        ),
        "begin approval step-up",
    );
    let assertion = authenticator.assert(&challenge.ceremony);
    let evidence = required(
        passkeys.finish_step_up(&mut *scope, &challenge.challenge_id, &assertion, 114),
        "finish approval step-up",
    );
    let (mut lossy_boundary, relay) = fixture.lose_approval_reply();
    let lost = Decisions::approve(
        &mut *scope,
        passkeys,
        first_session.access_token().expose(),
        first_session.csrf_token().expose(),
        &evidence,
        &mut lossy_boundary,
        [1; 32],
        "approve-first",
        sequence,
        115,
        &mut *audit,
        &TraceId::mint([2; 16]),
    );
    assert!(matches!(
        lost,
        Err(DecisionError::Agent(ApprovalBoundaryError::Unavailable))
    ));
    checked(relay.join());
    fixture.assert_reservations(2, 51, 1);
    assert!(
        checked(audit.entries(scope)).is_empty(),
        "a lost reply cannot invent a Human success"
    );
}

struct DecisionAuditContext<'a, 'scope> {
    fixture: &'a owner::Fixture,
    scope: &'a mut PrincipalScope<'scope>,
    passkeys: &'a Passkeys,
    second_session: &'a SessionGrant,
    boundary: &'a mut layerx_human_service::server::agent_runtime::AgentRuntime,
    audit: &'a mut AuditChain,
    sequence: u64,
    reference: [u8; 32],
}

fn assert_decision_audit(
    context: DecisionAuditContext<'_, '_>,
    first_digest: [u8; 32],
    second_digest: [u8; 32],
) -> (Vec<u8>, Vec<u8>) {
    let DecisionAuditContext {
        fixture,
        scope,
        passkeys,
        second_session,
        boundary,
        audit,
        sequence,
        reference,
    } = context;
    for (key, now, trace) in [
        ("reject-first-other-device", 116, [3; 16]),
        ("reject-first-another-key", 117, [4; 16]),
    ] {
        let other = checked(Decisions::reject(
            &mut *scope,
            passkeys,
            second_session.access_token().expose(),
            second_session.csrf_token().expose(),
            &mut *boundary,
            [1; 32],
            key,
            sequence,
            now,
            &mut *audit,
            &TraceId::mint(trace),
        ));
        assert!(other.already_decided());
        assert_eq!(
            other.status(),
            AgentDecisionStatus::Approved {
                submission_ref: None
            }
        );
        assert_eq!(other.tracking(), None);
        fixture.assert_reservations(2, 51, 1);
    }
    let rejected = checked(Decisions::reject(
        &mut *scope,
        passkeys,
        second_session.access_token().expose(),
        second_session.csrf_token().expose(),
        &mut *boundary,
        [2; 32],
        "reject-second",
        sequence,
        118,
        &mut *audit,
        &TraceId::mint([5; 16]),
    ));
    assert_eq!(rejected.resolution(), AgentDecisionResolution::Applied);
    assert_eq!(rejected.status(), AgentDecisionStatus::Rejected);
    assert_eq!(rejected.nothing_moved(), Some("Nothing moved."));
    fixture.assert_reservations(1, 25, 1);
    fixture.assert_release(1, reference);
    let rejected_hold = checked(boundary.approval_get([2; 32], sequence));
    assert_eq!(rejected_hold.state, AgentApprovalState::Rejected);
    assert_eq!(rejected_hold.held_activity, fixture.prepared[&2].disclosure);
    fixture.assert_budget(&rejected_hold, 52, sequence);
    let rejected_decision = fixture.assert_rejected_decision(2, sequence);
    let entries = checked(audit.entries(scope));
    assert_eq!(entries.len(), 3);
    assert!(matches!(entries[0].event(), AuditEvent::ApprovalDecision {
        hold_digest, outcome: ApprovalOutcome::Approved, ..
    } if *hold_digest == first_digest));
    assert!(matches!(entries[1].event(), AuditEvent::ApprovalDecision {
        hold_digest, outcome: ApprovalOutcome::Approved, ..
    } if *hold_digest == first_digest));
    assert!(matches!(entries[2].event(), AuditEvent::ApprovalDecision {
        hold_digest, outcome: ApprovalOutcome::Rejected, ..
    } if *hold_digest == second_digest));
    let export = checked(audit.export(scope));
    let report = checked(verify_export(&export));
    assert_eq!(report.entries(), 3);
    assert_eq!(report.evidence_rows(), 3);
    (rejected_decision, export)
}

fn assert_reopened_approval(
    fixture: &owner::Fixture,
    boundary: &mut layerx_human_service::server::agent_runtime::AgentRuntime,
    sequence: u64,
    first_digest: [u8; 32],
) -> [u8; 32] {
    let replayed_decision =
        checked(boundary.approval_decide(true, [1; 32], first_digest, "approve-first", sequence));
    let reference = match replayed_decision.status {
        AgentDecisionStatus::Approved {
            submission_ref: Some(reference),
        } => reference,
        status => panic!("durable winner: {status:?}"),
    };
    fixture.assert_release(1, reference);
    fixture.assert_reservations(2, 51, 1);
    let recovered = checked(boundary.approval_get([1; 32], sequence));
    assert_eq!(recovered.held_activity, fixture.prepared[&1].disclosure);
    assert_eq!(
        recovered.state,
        AgentApprovalState::Approved {
            submission_ref: reference
        }
    );
    fixture.assert_budget(&recovered, 26, sequence);
    let mut wrong_digest = first_digest;
    wrong_digest[0] ^= 1;
    assert!(boundary
        .approval_decide(true, [1; 32], wrong_digest, "approve-first", sequence)
        .is_err());
    fixture.assert_foreign_refusals(1, sequence);
    fixture.assert_release(1, reference);
    fixture.assert_reservations(2, 51, 1);

    let tracking = boundary.track_released(reference);
    eprintln!(
        "decision after lost-reply reopen: track_released({}) = {tracking:?}; expected state: Queued",
        hex::encode(reference),
    );
    let tracking = checked(tracking);
    assert_eq!(tracking.state, SubmissionState::Queued);
    assert_eq!(tracking.submission_ref.as_str(), hex::encode(reference));
    reference
}

#[test]
fn digest_bound_decisions_converge_and_export_the_agent_outcome() {
    let mut fixture = owner::Fixture::new(&[1, 2]);
    let sequence = fixture.sequence;
    let first = fixture.prepared[&1].clone();
    let second = fixture.prepared[&2].clone();
    let first_digest = first.disclosure.canonical_digest;
    let second_digest = second.disclosure.canonical_digest;
    assert_ne!(first_digest, second_digest);
    fixture.assert_reservations(2, 51, 0);
    fixture.assert_foreign_refusals(1, sequence);
    let mut boundary = fixture.client();
    let hold = checked(boundary.approval_get([1; 32], sequence));
    assert_eq!(hold.held_activity, first.disclosure);
    fixture.assert_budget(&hold, 26, sequence);

    let human_root = fixture.root().join("human");
    let map = support::tenancy(&[("mara", fixture.peer.tenant.as_str())]);
    let retention = support::retention_uniform(10_000);
    let (mut store, tenancy_digest) = support::install_and_open(&human_root, &map, retention);
    let mut scope = checked(store.principal(&support::principal("mara")));
    let passkeys = required(Passkeys::new(auth_config()), "passkeys");
    let mut authenticator = SoftwareAuthenticator::new();
    let first_session = register_session(&passkeys, &mut scope, &mut authenticator, 100);
    let second_session = open_session(
        &passkeys,
        &mut scope,
        &mut authenticator,
        "dev_aabbccddeeff00112233445566778892",
        106,
    );
    let mut audit = checked(AuditChain::open(&scope));
    assert_digest_bound_lost_reply(
        LostReplyContext {
            fixture: &fixture,
            scope: &mut scope,
            passkeys: &passkeys,
            authenticator: &mut authenticator,
            first_session: &first_session,
            boundary: &mut boundary,
            audit: &mut audit,
            sequence,
        },
        first_digest,
        second_digest,
    );
    let _ = audit;
    drop(scope);
    drop(store);
    fixture.reopen();
    let mut store = checked(PrincipalStore::open(
        &human_root,
        support::retention_uniform(10_000),
        tenancy_digest,
    ));
    let mut scope = checked(store.principal(&support::principal("mara")));
    let mut audit = checked(AuditChain::open(&scope));
    let mut boundary = fixture.client();
    let reference = assert_reopened_approval(&fixture, &mut boundary, sequence, first_digest);
    let tracking = checked(boundary.track_released(reference));
    let (rejected_decision, export) = assert_decision_audit(
        DecisionAuditContext {
            fixture: &fixture,
            scope: &mut scope,
            passkeys: &passkeys,
            second_session: &second_session,
            boundary: &mut boundary,
            audit: &mut audit,
            sequence,
            reference,
        },
        first_digest,
        second_digest,
    );
    let _ = audit;
    drop(scope);
    drop(store);
    fixture.reopen();
    fixture.assert_reservations(1, 25, 1);
    fixture.assert_release(1, reference);
    assert_eq!(
        fixture.assert_rejected_decision(2, sequence),
        rejected_decision,
        "the same store must retain the exact daemon-written rejection"
    );
    let mut store = checked(PrincipalStore::open(
        &human_root,
        support::retention_uniform(10_000),
        tenancy_digest,
    ));
    let scope = checked(store.principal(&support::principal("mara")));
    let audit = checked(AuditChain::open(&scope));
    assert_eq!(checked(audit.export(&scope)), export);
    let mut boundary = fixture.client();
    let terminal = checked(boundary.approval_get([2; 32], sequence));
    assert_eq!(terminal.state, AgentApprovalState::Rejected);
    fixture.assert_budget(&terminal, 52, sequence);
    assert_eq!(checked(boundary.track_released(reference)), tracking);
}

pub mod require_value {
    include!("support/require_value.rs");
}
