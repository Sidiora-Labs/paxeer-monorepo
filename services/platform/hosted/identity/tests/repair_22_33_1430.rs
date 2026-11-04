use layerx_platform_identity::mailer::Smtp;
use layerx_platform_identity::seal::StoreKey;
use layerx_platform_identity::signup::{
    self, Desk, Limits, Refused, SignupRequest, MAX_TRACKED_KEYS,
};
use layerx_platform_identity::store::{HourlyRefusal, Principal, Store, SUBJECT_ALREADY_EXISTS};
use layerx_platform_identity::verification;
use serde_json::{json, Value};
use std::fs;
use std::io::{BufRead, BufReader, Write};
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::thread;
use std::time::Duration;

const HOUR: u64 = 3600;
const DAY: u64 = 86_400;
const NOW: u64 = 10 * DAY + 12 * HOUR;
const SIGNER: &str = "abababababababababababababababababababababababababababababababab";

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = Path::new(env!("CARGO_TARGET_TMPDIR")).join(format!(
            "repair-22-33-1430-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir_all(&path).unwrap();
        Self(path)
    }
    fn open(&self) -> Store {
        Store::open(&self.0.join("state")).unwrap()
    }
    fn outbox(&self) -> PathBuf {
        self.0.join("outbox")
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}
fn key() -> StoreKey {
    StoreKey::derive(b"repair-22-33-1430-durable-store-key")
}
fn desk() -> Desk {
    Desk::new(Limits::default(), Vec::new())
}
fn token(index: usize) -> String {
    format!("vfy_{index:064x}")
}
fn request<'a>(email: &'a str, ip: &'a str, sub: &'a str) -> SignupRequest<'a> {
    SignupRequest {
        email,
        client_ip: ip,
        tenant: "beta",
        sub,
        signer_public_key: SIGNER,
    }
}
fn start(directory: &Directory, store: &mut Store, sub: &str, index: usize) {
    verification::start(
        &mut desk(),
        store,
        &key(),
        &directory.outbox(),
        &request(&format!("user{index}@example.com"), "ip", sub),
        &token(index),
        NOW,
    )
    .unwrap();
}

// A committed loopback SMTP fixture: the production mailer drives the complete protocol.
fn relay() -> (Smtp, thread::JoinHandle<Vec<String>>) {
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let port = listener.local_addr().unwrap().port();
    let handle = thread::spawn(move || {
        let (stream, _) = listener.accept().unwrap();
        stream
            .set_read_timeout(Some(Duration::from_secs(10)))
            .unwrap();
        let mut writer = stream.try_clone().unwrap();
        let mut reader = BufReader::new(stream);
        writer.write_all(b"220 fixture ESMTP\r\n").unwrap();
        let mut transcript = Vec::new();
        let mut data = false;
        loop {
            let mut line = String::new();
            assert_ne!(reader.read_line(&mut line).unwrap(), 0);
            let line = line.trim_end_matches("\r\n").to_owned();
            let reply: &[u8] = if data {
                if line == "." {
                    data = false;
                    b"250 queued\r\n"
                } else {
                    b""
                }
            } else if line.starts_with("EHLO ") {
                b"250 fixture\r\n"
            } else if line == "DATA" {
                data = true;
                b"354 send\r\n"
            } else if line == "QUIT" {
                b"221 bye\r\n"
            } else {
                b"250 ok\r\n"
            };
            writer.write_all(reply).unwrap();
            let quit = line == "QUIT";
            transcript.push(line);
            if quit {
                break;
            }
        }
        transcript
    });
    (
        Smtp::parse(&format!("smtp://127.0.0.1:{port}"), "identity@example.com").unwrap(),
        handle,
    )
}

#[test]
fn identity_outbox_durability_recovers_before_and_after_rename() {
    // Both interruptions use the production producer, durable record, publisher and SMTP consumer.
    for after_rename in [false, true] {
        let directory = Directory::new();
        let mut store = directory.open();
        start(&directory, &mut store, "outbox-user", 1);
        let digest = signup::sha256_hex(token(1).as_bytes());
        let (_, delivery) = store.signup_deliveries().pop().unwrap();
        fs::remove_file(directory.outbox().join(format!("{digest}.json"))).unwrap();
        let staged = verification::stage(&directory.outbox(), &digest, &delivery, &key()).unwrap();
        if after_rename {
            staged.publish().unwrap();
            // The namespace entry is what can disappear after an interrupted publication.
            fs::remove_file(directory.outbox().join(format!("{digest}.json"))).unwrap();
        } else {
            drop(staged);
            assert!(directory.outbox().join(format!("{digest}.tmp")).exists());
        }
        drop(store);
        let mut store = directory.open();
        verification::reconcile(&mut store, &key(), &directory.outbox(), NOW + 1).unwrap();
        assert!(store.pending_signup(&digest).is_some());
        assert!(directory.outbox().join(format!("{digest}.json")).exists());
        let (smtp, transcript) = relay();
        assert_eq!(
            smtp.drain_confirmed(&directory.outbox(), |digest| store
                .delivery_accepted(digest))
                .unwrap(),
            1
        );
        let transcript = transcript.join().unwrap();
        assert!(transcript
            .iter()
            .any(|line| line == "RCPT TO:<user1@example.com>"));
        let received = transcript
            .iter()
            .find_map(|line| line.strip_prefix("Your verification code is "))
            .unwrap();
        assert_eq!(received, token(1));
        let response =
            verification::complete(&mut store, &key(), received, "outbox-user", HOUR, NOW + 2)
                .unwrap();
        assert_eq!(response["sub"], "outbox-user");
        assert!(store.signup_deliveries().is_empty());
        drop(store);
        let mut store = directory.open();
        verification::reconcile(&mut store, &key(), &directory.outbox(), NOW + 3).unwrap();
        assert_eq!(fs::read_dir(directory.outbox()).unwrap().count(), 0);
        let snapshot = fs::read_to_string(directory.0.join("state/snapshot.json")).unwrap();
        assert!(!snapshot.contains(&token(1)));
        assert!(!snapshot.contains(response["token"].as_str().unwrap()));
    }
}

#[test]
fn identity_outbox_failed_publication_is_retried_after_reopen() {
    let directory = Directory::new();
    fs::write(directory.outbox(), b"publication unavailable").unwrap();
    let mut store = directory.open();
    assert_eq!(
        verification::start(
            &mut desk(),
            &mut store,
            &key(),
            &directory.outbox(),
            &request("retry@example.com", "ip", "retry"),
            &token(2),
            NOW
        ),
        Err(Refused::Delivery)
    );
    let digest = signup::sha256_hex(token(2).as_bytes());
    assert!(store.pending_signup(&digest).is_some());
    drop(store);
    fs::remove_file(directory.outbox()).unwrap();
    let mut store = directory.open();
    verification::reconcile(&mut store, &key(), &directory.outbox(), NOW + 1).unwrap();
    let document: Value = serde_json::from_slice(
        &fs::read(directory.outbox().join(format!("{digest}.json"))).unwrap(),
    )
    .unwrap();
    assert_eq!(document["token"], token(2));
    assert_eq!(document["email"], "retry@example.com");
}

fn assert_session_result(store: &Store, response: &Value) {
    let id = response["session_id"].as_str().unwrap();
    let session = store.session(id).unwrap();
    assert_eq!(session.principal, response["sub"].as_str().unwrap());
    let bearer = response["token"].as_str().unwrap();
    let (session_id, secret) = bearer
        .strip_prefix("ses_")
        .unwrap()
        .split_once('.')
        .unwrap();
    assert_eq!(session_id, id);
    assert_eq!(session.token_digest, signup::sha256_hex(secret.as_bytes()));
    let csrf = response["csrf_token"].as_str().unwrap();
    assert_eq!(session.csrf_digest, signup::sha256_hex(csrf.as_bytes()));
    assert_eq!(
        key().open(&session.csrf_sealed).unwrap().as_slice(),
        csrf.as_bytes()
    );
}

#[test]
fn identity_verify_session_retry_is_atomic_and_recovers_lost_response() {
    let directory = Directory::new();
    let mut store = directory.open();
    start(&directory, &mut store, "session-user", 3);
    let digest = signup::sha256_hex(token(3).as_bytes());
    // An unavailable journal cannot consume verification or create half a session transaction.
    fs::rename(directory.0.join("state"), directory.0.join("disconnected")).unwrap();
    assert_eq!(
        verification::complete(&mut store, &key(), &token(3), "session-user", HOUR, NOW + 1),
        Err(Refused::Store)
    );
    assert!(store.pending_signup(&digest).is_some());
    assert!(store.principal("beta", "session-user").is_none());
    drop(store);
    fs::rename(directory.0.join("disconnected"), directory.0.join("state")).unwrap();
    let mut store = directory.open();
    let before = fs::read(directory.0.join("state/snapshot.json")).unwrap();
    let response =
        verification::complete(&mut store, &key(), &token(3), "session-user", HOUR, NOW + 2)
            .unwrap();
    assert_session_result(&store, &response);
    let journal = fs::read(directory.0.join("state/journal.log")).unwrap();
    assert_eq!(journal.iter().filter(|byte| **byte == b'\n').count(), 1);
    drop(store);
    // A storage crash in the middle of this one record exposes neither verification nor session.
    fs::write(
        directory.0.join("state/journal.log"),
        &journal[..journal.len() / 2],
    )
    .unwrap();
    let store = directory.open();
    assert!(store.pending_signup(&digest).is_some());
    assert!(store.principal("beta", "session-user").is_none());
    assert!(store
        .session(response["session_id"].as_str().unwrap())
        .is_none());
    drop(store);
    // Replay the fully durable transaction with its original snapshot: response was never received.
    fs::write(directory.0.join("state/snapshot.json"), before).unwrap();
    fs::write(directory.0.join("state/journal.log"), journal).unwrap();
    let mut store = directory.open();
    for _ in 0..8 {
        let retried =
            verification::complete(&mut store, &key(), &token(3), "session-user", HOUR, NOW + 3)
                .unwrap();
        assert_eq!(retried, response);
        assert_session_result(&store, &retried);
    }
    assert_eq!(store.signup_state_sizes().5, 1);
    assert!(fs::read(directory.0.join("state/journal.log"))
        .unwrap()
        .is_empty());
    assert_eq!(
        verification::complete(&mut store, &key(), &token(3), "other-user", HOUR, NOW + 3),
        Err(Refused::TokenUnknown)
    );
    drop(store);
    let mut store = directory.open();
    assert_eq!(
        verification::complete(&mut store, &key(), &token(3), "session-user", HOUR, NOW + 4)
            .unwrap(),
        response
    );
    store
        .revoke_session(response["session_id"].as_str().unwrap(), NOW + 5)
        .unwrap();
    assert_eq!(
        verification::complete(&mut store, &key(), &token(3), "session-user", HOUR, NOW + 6),
        Err(Refused::TokenExpired)
    );
}

#[test]
fn identity_hourly_key_bound_refuses_distinct_ip_flood_and_capacity() {
    let directory = Directory::new();
    let mut store = directory.open();
    let limits = Limits {
        per_ip_per_hour: 2,
        per_email_per_hour: 1,
        daily_cap: u32::MAX,
        token_ttl_seconds: DAY,
    };
    let mut desk = Desk::new(limits, Vec::new());
    desk.start(
        &mut store,
        &request("limited@example.com", "first", "pending"),
        &token(4),
        NOW,
    )
    .unwrap();
    for index in 0..MAX_TRACKED_KEYS + 17 {
        assert_eq!(
            desk.start(
                &mut store,
                &request("limited@example.com", &format!("ip-{index}"), "pending"),
                &token(index + 10),
                NOW + 1
            ),
            Err(Refused::EmailRateLimited(HOUR - 1))
        );
    }
    assert_eq!(store.signup_state_sizes().2, 1);
    assert_eq!(store.signup_state_sizes().3, 1);
    drop(store);
    let mut store = directory.open();
    let mut desk = Desk::new(limits, Vec::new());
    assert_eq!(
        desk.start(
            &mut store,
            &request("limited@example.com", "fresh", "pending"),
            &token(5),
            NOW + 2
        ),
        Err(Refused::EmailRateLimited(HOUR - 2))
    );
    desk.start(
        &mut store,
        &request("limited@example.com", "fresh", "pending"),
        &token(6),
        NOW + HOUR,
    )
    .unwrap();
    assert_eq!(
        desk.start(
            &mut store,
            &request("limited@example.com", "another", "pending"),
            &token(7),
            NOW + HOUR
        ),
        Err(Refused::EmailRateLimited(HOUR))
    );
    // Fill the authoritative limiter using its production admission path, then prove strict overload.
    for index in 1..MAX_TRACKED_KEYS {
        assert_eq!(
            store
                .admit_signup_hour(
                    &format!("capacity-{index}"),
                    "shared",
                    NOW + HOUR,
                    1,
                    u32::MAX
                )
                .unwrap(),
            Ok(())
        );
    }
    assert_eq!(store.signup_state_sizes().2, MAX_TRACKED_KEYS);
    assert_eq!(
        store
            .admit_signup_hour("overflow", "shared", NOW + HOUR, 1, u32::MAX)
            .unwrap(),
        Err(HourlyRefusal::Capacity)
    );
    drop(store);
    let mut store = directory.open();
    assert_eq!(store.signup_state_sizes().2, MAX_TRACKED_KEYS);
    assert_eq!(
        store
            .admit_signup_hour("overflow", "shared", NOW + HOUR + 1, 1, u32::MAX)
            .unwrap(),
        Err(HourlyRefusal::Capacity)
    );
    assert_eq!(
        store
            .admit_signup_hour("overflow", "shared", NOW + 2 * HOUR, 1, u32::MAX)
            .unwrap(),
        Ok(())
    );
    assert_eq!(store.signup_state_sizes().2, 1);
}

#[test]
fn identity_rate_window_restart_preserves_ip_and_email_refusals_until_expiry() {
    let directory = Directory::new();
    let limits = Limits {
        per_ip_per_hour: 1,
        per_email_per_hour: 1,
        daily_cap: 100,
        token_ttl_seconds: DAY,
    };
    let mut store = directory.open();
    let mut desk = Desk::new(limits, Vec::new());
    desk.start(
        &mut store,
        &request("one@example.com", "same-ip", "one"),
        &token(8),
        NOW + 10,
    )
    .unwrap();
    for reopen in [false, true] {
        if reopen {
            drop(store);
            store = directory.open();
            desk = Desk::new(limits, Vec::new());
        }
        assert_eq!(
            desk.start(
                &mut store,
                &request("two@example.com", "same-ip", "two"),
                &token(9),
                NOW + 20
            ),
            Err(Refused::IpRateLimited(HOUR - 20))
        );
        assert_eq!(
            desk.start(
                &mut store,
                &request("one@example.com", "other-ip", "other"),
                &token(10),
                NOW + 20
            ),
            Err(Refused::EmailRateLimited(HOUR - 20))
        );
    }
    assert_eq!(
        desk.start(
            &mut store,
            &request("one@example.com", "same-ip", "one"),
            &token(11),
            NOW + HOUR - 1
        ),
        Err(Refused::IpRateLimited(1))
    );
    desk.start(
        &mut store,
        &request("one@example.com", "same-ip", "one"),
        &token(12),
        NOW + HOUR,
    )
    .unwrap();
    assert_eq!(store.signups_on_day(NOW / DAY), 2);
}

#[test]
fn identity_expired_signup_growth_reclaims_live_state_and_preserves_bindings() {
    let directory = Directory::new();
    let mut store = directory.open();
    start(&directory, &mut store, "verified", 13);
    verification::complete(&mut store, &key(), &token(13), "verified", HOUR, NOW + 1).unwrap();
    let verified = store.principal("beta", "verified").unwrap().clone();
    let limits = Limits {
        per_ip_per_hour: 100,
        per_email_per_hour: 100,
        daily_cap: 100,
        token_ttl_seconds: 10,
    };
    let mut desk = Desk::new(limits, Vec::new());
    for day in 1..48 {
        let now = NOW + day * DAY;
        for index in 0..8 {
            let n = (day * 10 + index) as usize + 100;
            verification::start(
                &mut desk,
                &mut store,
                &key(),
                &directory.outbox(),
                &request(
                    &format!("abandoned{n}@example.com"),
                    "abandoned-ip",
                    &format!("abandoned{n}"),
                ),
                &token(n),
                now,
            )
            .unwrap();
        }
        assert_eq!(store.signup_state_sizes().0, 8);
        assert_eq!(store.signup_state_sizes().1, 1);
        assert_eq!(
            verification::complete(
                &mut store,
                &key(),
                &token((day * 10) as usize + 100),
                &format!("abandoned{}", day * 10 + 100),
                HOUR,
                now + 10
            ),
            Err(Refused::TokenExpired)
        );
        verification::reconcile(&mut store, &key(), &directory.outbox(), now + 10).unwrap();
        assert_eq!(store.signup_state_sizes().0, 0);
        assert_eq!(store.signup_state_sizes().4, 0);
        assert_eq!(store.signup_state_sizes().5, 0);
        assert_eq!(
            fs::metadata(directory.0.join("state/journal.log"))
                .unwrap()
                .len(),
            0
        );
        drop(store);
        store = directory.open();
        assert_eq!(store.signup_state_sizes().0, 0);
        assert_eq!(store.signup_state_sizes().1, 1);
        assert_eq!(store.principal("beta", "verified"), Some(&verified));
        assert!(store.email_in_use(&signup::sha256_hex(b"user13@example.com")));
        assert_eq!(
            verification::complete(
                &mut store,
                &key(),
                &token((day * 10) as usize + 100),
                &format!("abandoned{}", day * 10 + 100),
                HOUR,
                now + 11
            ),
            Err(Refused::TokenUnknown)
        );
    }
    assert!(fs::read_to_string(directory.0.join("state/audit.log"))
        .unwrap()
        .contains("signup_verified"));
}

#[test]
fn identity_signup_subject_replacement_is_refused_and_explicit_rotation_survives() {
    let directory = Directory::new();
    let mut store = directory.open();
    start(&directory, &mut store, "existing", 14);
    // A second pending signup races verification of the first, with a different email and signer.
    let other_key = "cd".repeat(32);
    let mut other = request("attacker@example.com", "other-ip", "existing");
    other.signer_public_key = &other_key;
    desk()
        .start(
            &mut store,
            &other,
            &signup::sha256_hex(token(15).as_bytes()),
            NOW,
        )
        .unwrap();
    verification::complete(&mut store, &key(), &token(14), "existing", HOUR, NOW + 1).unwrap();
    let original = store.principal("beta", "existing").unwrap().clone();
    let second_digest = signup::sha256_hex(token(15).as_bytes());
    assert_eq!(
        verification::complete(&mut store, &key(), &token(15), "existing", HOUR, NOW + 2),
        Err(Refused::SubjectExists)
    );
    assert_eq!(
        store.verify_signup(&second_digest, NOW + 2),
        Err(SUBJECT_ALREADY_EXISTS.to_owned())
    );
    assert_eq!(
        desk().start(
            &mut store,
            &other,
            &signup::sha256_hex(token(16).as_bytes()),
            NOW + 2
        ),
        Err(Refused::SubjectExists)
    );
    assert_eq!(store.principal("beta", "existing"), Some(&original));
    assert!(!store.email_in_use(&signup::sha256_hex(b"attacker@example.com")));
    drop(store);
    let mut store = directory.open();
    assert_eq!(
        verification::complete(&mut store, &key(), &token(15), "existing", HOUR, NOW + 3),
        Err(Refused::SubjectExists)
    );
    // Existing authenticated principal management remains the explicit rotation operation.
    let rotated = Principal {
        allowed_signer_public_keys: vec![other_key],
        account: Some("agent:existing:main".to_owned()),
        audiences: vec!["ramp".to_owned()],
        ..original
    };
    store.put_principal(rotated.clone()).unwrap();
    drop(store);
    let store = directory.open();
    assert_eq!(store.principal("beta", "existing"), Some(&rotated));
    assert_eq!(
        store.verified_email("existing"),
        Some(signup::sha256_hex(b"user14@example.com").as_str())
    );
}

mod http_fixture {
    include!("../src/main.rs");

    pub struct Identity(Shared);
    impl Identity {
        pub fn open(root: &std::path::Path) -> Self {
            let tls = ServerConfig::builder_with_provider(Arc::new(
                rustls::crypto::ring::default_provider(),
            ))
            .with_safe_default_protocol_versions()
            .unwrap()
            .with_no_client_auth()
            .with_cert_resolver(Arc::new(rustls::server::ResolvesServerCertUsingSni::new()));
            Self(Shared {
                config: Config {
                    listen: "127.0.0.1:0".parse().unwrap(),
                    health: "127.0.0.1:0".parse().unwrap(),
                    tls: Arc::new(tls),
                    state_dir: root.join("state"),
                    store_key: super::key(),
                    service_tokens: vec![
                        ServiceToken {
                            service: Service::Provisioning,
                            token: Zeroizing::new("fixture-provisioning-token".to_owned()),
                        },
                        ServiceToken {
                            service: Service::Gateway,
                            token: Zeroizing::new("fixture-gateway-token".to_owned()),
                        },
                    ],
                    default_ttl_seconds: 3600,
                    signup_limits: Limits::default(),
                    disposable_domains: Vec::new(),
                    outbox_dir: root.join("outbox"),
                    smtp: None,
                },
                store: Mutex::new(Store::open(&root.join("state")).unwrap()),
                desk: Mutex::new(Desk::new(Limits::default(), Vec::new())),
            })
        }

        pub fn post(
            &self,
            path: &str,
            credential: &str,
            body: serde_json::Value,
        ) -> (u16, serde_json::Value) {
            let body = body.to_string();
            let wire = format!("POST {path} HTTP/1.1\r\nHost: identity\r\nAuthorization: Bearer {credential}\r\nContent-Type: application/json\r\nContent-Length: {}\r\n\r\n{body}", body.len());
            let request = parse_client_request(&mut wire.as_bytes()).unwrap();
            let response = route(&self.0, &request);
            let mut encoded = Vec::new();
            write_response(&mut encoded, &response).unwrap();
            let encoded = String::from_utf8(encoded).unwrap();
            let (headers, body) = encoded.split_once("\r\n\r\n").unwrap();
            let status = headers.split_whitespace().nth(1).unwrap().parse().unwrap();
            (status, serde_json::from_str(body).unwrap())
        }
    }
}

#[test]
fn identity_http_signup_retry_and_authenticated_rotation_use_the_real_codec_and_routes() {
    let directory = Directory::new();
    let identity = http_fixture::Identity::open(&directory.0);
    let signup = json!({"email":"http@example.com", "client_ip":"127.0.0.1", "tenant":"beta", "sub":"http-user", "signer_public_key":SIGNER});
    assert_eq!(
        identity
            .post("/v1/signup", "fixture-provisioning-token", signup.clone())
            .0,
        200
    );
    let queued = fs::read_dir(directory.outbox())
        .unwrap()
        .next()
        .unwrap()
        .unwrap()
        .path();
    let message: Value = serde_json::from_slice(&fs::read(queued).unwrap()).unwrap();
    let verify = json!({"sub":"http-user", "token":message["token"]});
    let (status, response) = identity.post(
        "/v1/signup/verify",
        "fixture-provisioning-token",
        verify.clone(),
    );
    assert_eq!(status, 200);
    drop(identity);
    let identity = http_fixture::Identity::open(&directory.0);
    assert_eq!(
        identity.post("/v1/signup/verify", "fixture-provisioning-token", verify),
        (200, response.clone())
    );
    let introspect = json!({"token":response["token"]});
    let (status, active) = identity.post(
        "/v1/introspect",
        "fixture-gateway-token",
        introspect.clone(),
    );
    assert_eq!(status, 200);
    assert_eq!(active["active"], true);
    assert_eq!(active["allowed_signer_public_keys"], json!([SIGNER]));
    let rotated_key = "cd".repeat(32);
    let mut replacement = signup;
    replacement["email"] = json!("replacement@example.com");
    replacement["signer_public_key"] = json!(rotated_key);
    let (status, refused) = identity.post("/v1/signup", "fixture-provisioning-token", replacement);
    assert_eq!(status, 409);
    assert_eq!(refused["error"]["code"], "subject_already_exists");
    let rotation = json!({"tenant":"beta", "sub":"http-user", "allowed_signer_public_keys":[rotated_key], "account":"agent:http-user:main", "audiences":["ramp"]});
    assert_eq!(
        identity
            .post("/v1/principals", "invalid-token", rotation.clone())
            .0,
        401
    );
    assert_eq!(
        identity
            .post("/v1/principals", "fixture-gateway-token", rotation.clone())
            .0,
        403
    );
    assert_eq!(
        identity
            .post("/v1/principals", "fixture-provisioning-token", rotation)
            .0,
        200
    );
    drop(identity);
    let identity = http_fixture::Identity::open(&directory.0);
    let (status, active) = identity.post("/v1/introspect", "fixture-gateway-token", introspect);
    assert_eq!(status, 200);
    assert_eq!(active["active"], true);
    assert_eq!(active["allowed_signer_public_keys"], json!([rotated_key]));
}
