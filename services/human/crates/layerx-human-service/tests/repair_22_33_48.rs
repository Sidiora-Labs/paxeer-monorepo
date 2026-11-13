use std::fmt::Display;
use std::path::PathBuf;

use base64::engine::general_purpose::URL_SAFE_NO_PAD;
use base64::Engine as _;
use ciborium::value::Value as CborValue;
use ed25519_dalek::{Signer as _, SigningKey};
use layerx_human_service::auth::{
    AccountIdentity, AssertionProof, AuthConfig, AuthError, Device, Passkeys,
    PreparedBrowserSession, RateLimit, ReplayableSessionRequest, SessionGrant,
};
use layerx_human_service::store::{
    AuditDisposition, EvidenceRef, PrincipalScope, PrincipalStore, StoreError, Table, TenancyDigest,
};
use layerx_human_test_support::{
    directory, install_and_open, principal, retention_uniform, row_key, tenancy,
};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

const RP_ID: &str = "paxportwallet.com";
const ORIGIN: &str = "https://paxportwallet.com";
const FLAG_UP: u8 = 1 << 0;
const FLAG_UV: u8 = 1 << 2;
const FLAG_AT: u8 = 1 << 6;

fn required<T, E: Display>(result: Result<T, E>, label: &str) -> T {
    result.unwrap_or_else(|error| panic!("{label}: {error}"))
}

fn config(attempts: u32) -> AuthConfig {
    AuthConfig {
        rp_id: RP_ID.to_owned(),
        rp_name: "LayerX".to_owned(),
        origin: ORIGIN.to_owned(),
        ceremony_ttl_secs: 300,
        assertion_ttl_secs: 30,
        session_ttl_secs: 300,
        refresh_ttl_secs: 600,
        step_up_ttl_secs: 10,
        rate_limit: RateLimit {
            attempts,
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

/// A software `WebAuthn` authenticator exercising the same Ed25519 signature,
/// authenticator-data, CBOR attestation and monotonic-counter path as a
/// hardware/platform authenticator.
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
        let client_data = client_data("webauthn.create", challenge, ORIGIN);
        encode_response(&json!({
            "id": URL_SAFE_NO_PAD.encode(&self.credential_id),
            "transports": ["internal"],
            "attestationObject": URL_SAFE_NO_PAD.encode(self.attestation_object()),
            "clientDataJSON": URL_SAFE_NO_PAD.encode(client_data),
        }))
    }

    fn assert(&mut self, ceremony: &str) -> String {
        self.assert_for_origin(ceremony, ORIGIN)
    }

    fn assert_for_origin(&mut self, ceremony: &str, origin: &str) -> String {
        let options = decode_ceremony(ceremony);
        let challenge = required_text(&options, "/challenge");
        self.counter = self.counter.saturating_add(1);
        let authenticator_data = self.authenticator_data(self.counter, false);
        let client_data = client_data("webauthn.get", challenge, origin);
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

fn client_data(kind: &str, challenge: &str, origin: &str) -> Vec<u8> {
    required(
        serde_json::to_vec(&json!({
            "type": kind,
            "challenge": challenge,
            "origin": origin,
            "crossOrigin": false,
        })),
        "encode client data",
    )
}

struct Fixture {
    root: PathBuf,
    store: Option<PrincipalStore>,
    digest: TenancyDigest,
    passkeys: Passkeys,
    authenticator: SoftwareAuthenticator,
    proof: AssertionProof,
}

impl Fixture {
    fn new() -> Self {
        let root = directory("repair-22-33-48");
        let map = tenancy(&[("alice", "tenant-a")]);
        let (mut store, digest) = install_and_open(&root, &map, retention_uniform(10_000));
        let passkeys = required(Passkeys::new(config(100)), "passkey service");
        let mut authenticator = SoftwareAuthenticator::new();
        let mut scope = required(store.principal(&principal("alice")), "scope");
        let identity = required(
            AccountIdentity::new("alice@example.com", "Alice"),
            "identity",
        );
        let registration = required(
            passkeys.begin_registration(&mut scope, &identity, "Passkey", 100),
            "begin registration",
        );
        let response = authenticator.register(&registration.ceremony);
        required(
            passkeys.finish_registration(&mut scope, &registration.registration_id, &response, 101),
            "finish registration",
        );
        let proof = Self::assert(&passkeys, &mut authenticator, &mut scope, 102);
        drop(scope);
        let mut fixture = Self {
            root,
            store: Some(store),
            digest,
            passkeys,
            authenticator,
            proof,
        };
        fixture.reopen();
        fixture
    }

    fn assert(
        passkeys: &Passkeys,
        authenticator: &mut SoftwareAuthenticator,
        scope: &mut PrincipalScope<'_>,
        now: u64,
    ) -> AssertionProof {
        let challenge = required(passkeys.begin_assertion(scope, now), "begin assertion");
        let response = authenticator.assert(&challenge.ceremony);
        required(
            passkeys.finish_assertion(scope, &challenge.assertion_id, &response, now + 1),
            "verify signed assertion",
        )
    }

    fn next_assertion(&mut self, now: u64) {
        let mut scope = required(
            self.store
                .as_mut()
                .or_fail("open store")
                .principal(&principal("alice")),
            "scope",
        );
        self.proof = Self::assert(&self.passkeys, &mut self.authenticator, &mut scope, now);
    }

    fn reopen(&mut self) {
        drop(self.store.take());
        self.store = Some(required(
            PrincipalStore::open(&self.root, retention_uniform(10_000), self.digest),
            "reopen durable store",
        ));
    }

    fn scope(&mut self) -> PrincipalScope<'_> {
        required(
            self.store
                .as_mut()
                .or_fail("open store")
                .principal(&principal("alice")),
            "scope",
        )
    }

    fn prepare(&mut self, device: Device, now: u64) -> Result<PreparedBrowserSession, AuthError> {
        let scope = required(
            self.store
                .as_mut()
                .or_fail("open store")
                .principal(&principal("alice")),
            "scope",
        );
        self.passkeys
            .prepare_open_session(&scope, &self.proof.assertion_id, device, now, [0; 32])
    }

    fn replayable(&mut self, action: u8, seed: u8, now: u64) -> PreparedBrowserSession {
        let mut scope = required(
            self.store
                .as_mut()
                .or_fail("open store")
                .principal(&principal("alice")),
            "scope",
        );
        required(
            self.passkeys.prepare_open_session_replayable(
                &mut scope,
                ReplayableSessionRequest {
                    assertion_id: &self.proof.assertion_id,
                    label: "Browser",
                    platform: "web",
                    action_key: [action; 32],
                    recovery_seed: [seed; 32],
                    now,
                },
            ),
            "prepare replayable session",
        )
    }

    fn commit(
        &mut self,
        prepared: PreparedBrowserSession,
        now: u64,
    ) -> Result<SessionGrant, AuthError> {
        Passkeys::commit_open_session(&mut self.scope(), prepared, now)
    }

    fn verified_row(&mut self) -> Value {
        let suffix = self
            .proof
            .assertion_id
            .strip_prefix("asr_")
            .or_fail("assertion identifier");
        let key = row_key(&format!("auth-verified-{suffix}"));
        let scope = self.scope();
        let row = scope.get(Table::Cache, &key).or_fail("verified assertion");
        required(
            serde_json::from_slice(row.bytes()),
            "verified assertion JSON",
        )
    }

    fn assert_counts(&mut self, sessions: usize, devices: usize, notifications: usize) {
        let scope = self.scope();
        let keys = scope.keys(Table::Cache);
        assert_eq!(
            keys.iter()
                .filter(|key| key.as_str().starts_with("auth-session-"))
                .count(),
            sessions
        );
        assert_eq!(
            keys.iter()
                .filter(|key| key.as_str().starts_with("auth-device-"))
                .count(),
            devices
        );
        assert_eq!(scope.keys(Table::Notifications).len(), notifications);
    }

    fn assert_grant(&mut self, grant: &SessionGrant, now: u64) {
        let scope = self.scope();
        let context = required(
            Passkeys::authorize_refresh(
                &scope,
                grant.refresh_token().expose(),
                grant.csrf_token().expose(),
                now,
            ),
            "consume recovered refresh and CSRF tokens",
        );
        assert_eq!(context.session_id, grant.session_id());
        assert_eq!(context.device_id, grant.session().device.device_id());
        let sessions = required(
            Passkeys::list_sessions_authorized(&scope, grant.session_id()),
            "recovered session inventory",
        );
        assert!(sessions
            .iter()
            .any(|session| session.session_id == grant.session_id()));
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        drop(self.store.take());
        required(std::fs::remove_dir_all(&self.root), "remove fixture");
    }
}

fn device() -> Device {
    required(Device::mint("Browser", "web"), "device")
}

#[test]
fn finding_7_interleaved_preparations_have_one_durable_winner() {
    for reverse in [false, true] {
        let mut fixture = Fixture::new();
        let first = required(fixture.prepare(device(), 104), "first preparation");
        fixture.reopen();
        let second = required(fixture.prepare(device(), 104), "second preparation");
        assert_ne!(first.session_id(), second.session_id());
        fixture.reopen();
        fixture.assert_counts(0, 0, 0);
        let (winner, loser) = if reverse {
            (second, first)
        } else {
            (first, second)
        };
        let grant = required(fixture.commit(winner, 105), "commit winner");
        fixture.reopen();
        let assertion = fixture.verified_row();
        assert_eq!(assertion["consumed"], true);
        assert_eq!(assertion["session_id"], grant.session_id());
        assert!(matches!(
            fixture.commit(loser, 106),
            Err(AuthError::AssertionSpent)
        ));
        fixture.reopen();
        fixture.assert_counts(1, 1, 1);
        fixture.assert_grant(&grant, 106);
        assert!(matches!(
            fixture.prepare(device(), 106),
            Err(AuthError::AssertionSpent)
        ));
    }
}

#[test]
fn finding_7_identical_replay_survives_each_durable_write_and_assertion_expiry() {
    for reverse in [false, true] {
        let mut fixture = Fixture::new();
        let first = fixture.replayable(1, 11, 104);
        let first_id = first.session_id().to_owned();
        fixture.reopen();
        fixture.assert_counts(0, 0, 0);
        let second = fixture.replayable(2, 22, 105);
        fixture.reopen();
        let (winner, loser, action, seed) = if reverse {
            (second, first, 2, 22)
        } else {
            (first, second, 1, 11)
        };
        let grant = required(fixture.commit(winner, 106), "commit winning opening");
        fixture.reopen();
        assert!(matches!(
            fixture.commit(loser, 107),
            Err(AuthError::AssertionSpent)
        ));
        fixture.reopen();
        let replay_at = fixture.proof.expires_at + 1;
        let replay = fixture.replayable(action, seed, replay_at);
        fixture.reopen();
        let replayed = required(
            fixture.commit(replay, replay_at),
            "replay original opening after expiry",
        );
        fixture.reopen();
        assert_eq!(grant.session_id(), replayed.session_id());
        assert_eq!(
            grant.access_token().expose(),
            replayed.access_token().expose()
        );
        assert_eq!(
            grant.refresh_token().expose(),
            replayed.refresh_token().expose()
        );
        assert_eq!(grant.csrf_token().expose(), replayed.csrf_token().expose());
        assert_eq!(grant.access_expires_at(), replayed.access_expires_at());
        assert_eq!(grant.refresh_expires_at(), replayed.refresh_expires_at());
        assert_eq!(grant.session(), replayed.session());
        assert_eq!(grant.session_id() == first_id, !reverse);
        fixture.assert_counts(1, 1, 1);
        fixture.assert_grant(&replayed, replay_at);
        let wrong_seed = fixture.replayable(action, seed + 1, replay_at);
        assert!(matches!(
            fixture.commit(wrong_seed, replay_at),
            Err(AuthError::AssertionSpent)
        ));
        fixture.reopen();
        fixture.assert_counts(1, 1, 1);
        fixture.assert_grant(&grant, replay_at);
    }
}

#[test]
fn finding_7_preparation_does_not_extend_assertion_expiry() {
    for replayable in [false, true] {
        for at_deadline in [false, true] {
            let mut fixture = Fixture::new();
            let prepared = if replayable {
                fixture.replayable(3, 33, 104)
            } else {
                required(fixture.prepare(device(), 104), "prepare before expiration")
            };
            fixture.reopen();
            let now = fixture.proof.expires_at + u64::from(!at_deadline);
            let result = fixture.commit(prepared, now);
            fixture.reopen();
            if at_deadline {
                let grant = required(result, "commit at inclusive expiration boundary");
                fixture.assert_counts(1, 1, 1);
                fixture.assert_grant(&grant, now);
            } else {
                assert!(matches!(result, Err(AuthError::ChallengeExpired)));
                fixture.assert_counts(0, 0, 0);
                let assertion = fixture.verified_row();
                assert_eq!(assertion["consumed"], false);
                assert!(assertion["session_id"].is_null());
            }
        }
    }
}

#[test]
fn finding_7_failed_commit_rolls_back_assertion_device_and_notification() {
    for pin_notification in [false, true] {
        let mut fixture = Fixture::new();
        let original_device = device();
        let prepared = required(
            fixture.prepare(original_device.clone(), 104),
            "prepare original",
        );
        let original = required(fixture.commit(prepared, 105), "commit original");
        fixture.reopen();
        let suffix = original_device
            .device_id()
            .strip_prefix("dev_")
            .or_fail("device identifier");
        let device_key = row_key(&format!("auth-device-{suffix}"));
        {
            let mut scope = fixture.scope();
            let (table, key) = if pin_notification {
                let notifications = scope.keys(Table::Notifications);
                assert_eq!(notifications.len(), 1);
                (Table::Notifications, notifications[0].clone())
            } else {
                (Table::Cache, device_key.clone())
            };
            let bytes = scope
                .get(table, &key)
                .or_fail("real produced row")
                .bytes()
                .to_vec();
            required(
                scope.append_audit(
                    row_key("opening-evidence"),
                    106,
                    bytes,
                    AuditDisposition::Exportable {
                        evidence: vec![EvidenceRef::new(table, key)],
                    },
                ),
                "pin real evidence row",
            );
            if pin_notification {
                assert!(required(
                    scope.remove(Table::Cache, &device_key),
                    "remove unpinned device"
                ));
            }
        }
        fixture.next_assertion(110);
        fixture.reopen();
        let pending = fixture.replayable(4, 44, 112);
        fixture.reopen();
        // The regular preparation targets the already produced device/notification.
        let refused = required(
            fixture.prepare(original_device, 112),
            "prepare pinned target",
        );
        assert!(matches!(
            fixture.commit(refused, 113),
            Err(AuthError::Store(StoreError::EvidencePinned))
        ));
        fixture.reopen();
        let assertion = fixture.verified_row();
        assert_eq!(assertion["consumed"], false);
        assert!(assertion["session_id"].is_null());
        fixture.assert_counts(1, usize::from(!pin_notification), 1);
        let grant = required(
            fixture.commit(pending, 114),
            "commit original replayable opening after rollback",
        );
        fixture.reopen();
        fixture.assert_counts(2, 1 + usize::from(!pin_notification), 2);
        fixture.assert_grant(&grant, 114);
        if !pin_notification {
            fixture.assert_grant(&original, 114);
        }
    }
}

#[test]
fn finding_7_legacy_consumption_without_a_binding_cannot_open_another_session() {
    let mut fixture = Fixture::new();
    let prepared = required(
        fixture.prepare(device(), 104),
        "prepare before legacy consumption",
    );
    let mut legacy = fixture.verified_row();
    legacy
        .as_object_mut()
        .or_fail("assertion object")
        .remove("session_id");
    legacy["consumed"] = json!(true);
    let suffix = fixture
        .proof
        .assertion_id
        .strip_prefix("asr_")
        .or_fail("assertion identifier");
    let key = row_key(&format!("auth-verified-{suffix}"));
    required(
        fixture.scope().put(
            Table::Cache,
            key,
            105,
            required(serde_json::to_vec(&legacy), "legacy encoding"),
        ),
        "store previous schema",
    );
    fixture.reopen();
    assert!(matches!(
        fixture.commit(prepared, 106),
        Err(AuthError::AssertionSpent)
    ));
    fixture.reopen();
    fixture.assert_counts(0, 0, 0);
}

#[test]
fn finding_7_mixed_preparations_cannot_bypass_the_binding() {
    for replayable_wins in [false, true] {
        let mut fixture = Fixture::new();
        let regular = required(fixture.prepare(device(), 104), "prepare regular session");
        let replayable = fixture.replayable(5, 55, 104);
        fixture.reopen();
        let (winner, loser) = if replayable_wins {
            (replayable, regular)
        } else {
            (regular, replayable)
        };
        let grant = required(fixture.commit(winner, 105), "commit mixed winner");
        fixture.reopen();
        assert!(matches!(
            fixture.commit(loser, 106),
            Err(AuthError::AssertionSpent)
        ));
        fixture.reopen();
        fixture.assert_counts(1, 1, 1);
        fixture.assert_grant(&grant, 106);
    }
}

#[test]
fn finding_7_persistence_failure_leaves_no_partial_opening() {
    let mut fixture = Fixture::new();
    let pending = fixture.replayable(6, 66, 104);
    let session_id = pending.session_id().to_owned();
    fixture.reopen();
    let temp = fixture
        .root
        .join("principals")
        .join(principal("alice").as_str())
        .join("store.bin.tmp");
    required(
        std::fs::create_dir(&temp),
        "block atomic snapshot destination",
    );
    assert!(matches!(
        fixture.commit(pending, 105),
        Err(AuthError::Store(StoreError::Io(_)))
    ));
    assert_eq!(fixture.verified_row()["consumed"], false);
    fixture.assert_counts(0, 0, 0);
    required(std::fs::remove_dir(temp), "restore snapshot destination");
    fixture.reopen();
    assert_eq!(fixture.verified_row()["consumed"], false);
    fixture.assert_counts(0, 0, 0);
    let recovered = fixture.replayable(6, 66, 106);
    assert_eq!(recovered.session_id(), session_id);
    let grant = required(
        fixture.commit(recovered, 106),
        "recover after persistence refusal",
    );
    fixture.reopen();
    fixture.assert_counts(1, 1, 1);
    fixture.assert_grant(&grant, 106);
}

use crate::require_value::RequireValue as _;

pub mod require_value {
    include!("support/require_value.rs");
}
