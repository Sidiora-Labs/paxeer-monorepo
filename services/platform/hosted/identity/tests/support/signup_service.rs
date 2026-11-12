#![allow(dead_code)]

include!("../../src/main.rs");

use rustls::pki_types::ServerName;
use rustls::{ClientConfig, ClientConnection};
use serde_json::{json, Value};
use std::io::{BufRead, BufReader};
use std::sync::atomic::{AtomicBool, AtomicU64};
use std::thread::JoinHandle;

const SIGNUP_SUB: &str = "did:key:signup-alice";
const OTHER_SUB: &str = "did:key:signup-other";
const SIGNUP_EMAIL: &str = "alice@example.com";
const SIGNUP_SIGNER: &str = "1f2e3d4c5b6a79880123456789abcdef1f2e3d4c5b6a79880123456789abcdef";
const CA: &[u8] = include_bytes!("../fixtures/signup-tls/ca.der");
const CERT: &[u8] = include_bytes!("../fixtures/signup-tls/server.der");
const KEY: &[u8] = include_bytes!("../fixtures/signup-tls/server-key.der");

struct SignupFixture {
    root: PathBuf,
    ttl: u64,
}

impl SignupFixture {
    fn new(ttl: u64) -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let root = Path::new(env!("CARGO_TARGET_TMPDIR")).join(format!(
            "identity-signup-service-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap_or_else(|error| panic!("fixture: {error}"));
        Self { root, ttl }
    }

    fn state(&self) -> PathBuf {
        self.root.join("state")
    }

    fn outbox(&self) -> PathBuf {
        self.state().join("signup-outbox")
    }

    fn open(&self) -> RunningIdentity {
        let provider = Arc::new(rustls::crypto::ring::default_provider());
        let tls = ServerConfig::builder_with_provider(provider)
            .with_safe_default_protocol_versions()
            .unwrap_or_else(|error| panic!("TLS versions: {error}"))
            .with_no_client_auth()
            .with_single_cert(
                vec![CertificateDer::from(CERT.to_vec())],
                PrivateKeyDer::from(PrivatePkcs8KeyDer::from(KEY.to_vec())),
            )
            .unwrap_or_else(|error| panic!("TLS fixture: {error}"));
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("identity listener: {error}"));
        let address = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error}"));
        let limits = Limits {
            token_ttl_seconds: self.ttl,
            ..Limits::default()
        };
        let store = Store::open(&self.state()).unwrap_or_else(|error| panic!("store: {error}"));
        store
            .probe_writable()
            .unwrap_or_else(|error| panic!("store readiness: {error}"));
        let shared = Arc::new(Shared {
            config: Config {
                listen: address,
                health: "127.0.0.1:0"
                    .parse()
                    .unwrap_or_else(|error| panic!("{error}")),
                tls: Arc::new(tls),
                state_dir: self.state(),
                store_key: StoreKey::derive(b"signup-integration-only-store-key"),
                service_tokens: Service::ALL
                    .into_iter()
                    .map(|service| ServiceToken {
                        service,
                        token: Zeroizing::new(caller_token(service)),
                    })
                    .collect(),
                default_ttl_seconds: 3600,
                signup_limits: limits,
                disposable_domains: Vec::new(),
                outbox_dir: self.outbox(),
                smtp: None,
            },
            store: Mutex::new(store),
            desk: Mutex::new(Desk::new(limits, Vec::new())),
        });
        let stopped = Arc::new(AtomicBool::new(false));
        let worker_shared = Arc::clone(&shared);
        let worker_stopped = Arc::clone(&stopped);
        let worker = thread::spawn(move || {
            for connection in listener.incoming() {
                let connection = connection.unwrap_or_else(|error| panic!("accept: {error}"));
                if worker_stopped.load(Ordering::Acquire) {
                    break;
                }
                handle_connection(&worker_shared, connection)
                    .unwrap_or_else(|error| panic!("identity connection: {error}"));
            }
        });
        RunningIdentity {
            shared,
            address,
            stopped,
            worker: Some(worker),
        }
    }

    fn queued(&self) -> Vec<PathBuf> {
        let mut paths: Vec<_> = fs::read_dir(self.outbox())
            .unwrap_or_else(|error| panic!("outbox: {error}"))
            .map(|entry| entry.unwrap_or_else(|error| panic!("{error}")).path())
            .collect();
        paths.sort();
        paths
    }

    fn deliver(&self, acknowledge: bool) -> String {
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("SMTP listener: {error}"));
        let address = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("{error}"));
        let smtp = Smtp::parse(&format!("smtp://{address}"), "identity@example.com")
            .unwrap_or_else(|error| panic!("SMTP config: {error}"));
        thread::scope(|scope| {
            let receiver = scope.spawn(move || smtp_transcript(&listener, acknowledge));
            let delivered = smtp.drain(&self.outbox());
            if acknowledge {
                assert_eq!(delivered, Ok(1));
            } else {
                assert!(
                    delivered.is_err(),
                    "a lost SMTP acknowledgement must retain the outbox"
                );
            }
            receiver
                .join()
                .unwrap_or_else(|error| std::panic::resume_unwind(error))
        })
    }
}

impl Drop for SignupFixture {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root).unwrap_or_else(|error| panic!("cleanup: {error}"));
    }
}

struct RunningIdentity {
    shared: Arc<Shared>,
    address: SocketAddr,
    stopped: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Drop for RunningIdentity {
    fn drop(&mut self) {
        self.stopped.store(true, Ordering::Release);
        TcpStream::connect(self.address).unwrap_or_else(|error| panic!("stop listener: {error}"));
        if let Some(worker) = self.worker.take() {
            worker
                .join()
                .unwrap_or_else(|error| std::panic::resume_unwind(error));
        }
    }
}

struct SignupReply {
    status: u16,
    headers: BTreeMap<String, String>,
    body: Value,
}

fn caller_token(service: Service) -> String {
    format!("signup-integration-{}-0123456789abcdef", service.name())
}

impl RunningIdentity {
    fn request(&self, path: &str, service: Option<Service>, body: &Value) -> SignupReply {
        let mut roots = RootCertStore::empty();
        roots
            .add(CertificateDer::from(CA.to_vec()))
            .unwrap_or_else(|error| panic!("client trust: {error}"));
        let client =
            ClientConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
                .with_safe_default_protocol_versions()
                .unwrap_or_else(|error| panic!("client TLS versions: {error}"))
                .with_root_certificates(roots)
                .with_no_client_auth();
        let connection = ClientConnection::new(
            Arc::new(client),
            ServerName::try_from("localhost")
                .unwrap_or_else(|error| panic!("server name: {error}")),
        )
        .unwrap_or_else(|error| panic!("client connection: {error}"));
        let tcp =
            TcpStream::connect(self.address).unwrap_or_else(|error| panic!("connect: {error}"));
        tcp.set_read_timeout(Some(IO_TIMEOUT))
            .unwrap_or_else(|error| panic!("{error}"));
        tcp.set_write_timeout(Some(IO_TIMEOUT))
            .unwrap_or_else(|error| panic!("{error}"));
        let mut stream = StreamOwned::new(connection, tcp);
        let authorization = service.map_or_else(String::new, |service| {
            format!("Authorization: Bearer {}\r\n", caller_token(service))
        });
        let body = body.to_string();
        write!(stream, "POST {path} HTTP/1.1\r\nHost: localhost\r\n{authorization}Content-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}", body.len())
            .unwrap_or_else(|error| panic!("request write: {error}"));
        stream
            .flush()
            .unwrap_or_else(|error| panic!("request flush: {error}"));
        let response = read_http_message(&mut stream, MAX_REQUEST_BYTES)
            .unwrap_or_else(|error| panic!("response: {error}"));
        let status = response.headers[""]
            .split_whitespace()
            .nth(1)
            .unwrap_or_else(|| panic!("status missing"))
            .parse()
            .unwrap_or_else(|error| panic!("status: {error}"));
        assert_eq!(response.headers["content-type"], "application/json");
        assert_eq!(response.headers["cache-control"], "no-store");
        SignupReply {
            status,
            headers: response.headers.clone(),
            body: serde_json::from_slice(&response.body)
                .unwrap_or_else(|error| panic!("response JSON: {error}")),
        }
    }

    fn provision(&self, path: &str, body: &Value) -> SignupReply {
        self.request(path, Some(Service::Provisioning), body)
    }

    fn verify(&self, token: &str, sub: &str) -> SignupReply {
        self.provision("/v1/signup/verify", &json!({"token":token,"sub":sub}))
    }

    fn grant(&self, sub: &str) -> SignupReply {
        self.provision("/v1/starter-credits", &json!({"sub":sub}))
    }

    fn settle(&self, sub: &str, grant: &str) -> SignupReply {
        self.provision(
            "/v1/starter-credits/settle",
            &json!({"sub":sub,"grant_id":grant}),
        )
    }
}

fn signup_body() -> Value {
    json!({
        "email":" Alice@Example.COM ", "client_ip":"127.0.0.1", "tenant":"beta",
        "sub":SIGNUP_SUB, "signer_public_key":SIGNUP_SIGNER,
    })
}

fn assert_refused(reply: &SignupReply, status: u16, code: &str) {
    assert_eq!(reply.status, status, "{}", reply.body);
    assert_eq!(reply.body["error"]["code"], code);
    assert_eq!(reply.body["error"]["retry"], "never");
}

fn text(value: &Value, field: &str) -> String {
    value[field]
        .as_str()
        .unwrap_or_else(|| panic!("missing {field}: {value}"))
        .to_owned()
}

fn smtp_line(session: &mut BufReader<TcpStream>) -> String {
    let mut line = String::new();
    assert!(
        session
            .read_line(&mut line)
            .unwrap_or_else(|error| panic!("SMTP read: {error}"))
            > 0
    );
    line
}

fn smtp_reply(session: &mut BufReader<TcpStream>, reply: &str) {
    session
        .get_mut()
        .write_all(reply.as_bytes())
        .unwrap_or_else(|error| panic!("SMTP write: {error}"));
    session
        .get_mut()
        .flush()
        .unwrap_or_else(|error| panic!("SMTP flush: {error}"));
}

fn smtp_transcript(listener: &TcpListener, acknowledge: bool) -> String {
    listener
        .set_nonblocking(true)
        .unwrap_or_else(|error| panic!("SMTP listener mode: {error}"));
    let deadline = std::time::Instant::now() + IO_TIMEOUT;
    let tcp = loop {
        match listener.accept() {
            Ok((tcp, _)) => break tcp,
            Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                assert!(
                    std::time::Instant::now() < deadline,
                    "production mailer never connected to SMTP"
                );
                thread::sleep(Duration::from_millis(10));
            }
            Err(error) => panic!("SMTP accept: {error}"),
        }
    };
    tcp.set_read_timeout(Some(IO_TIMEOUT))
        .unwrap_or_else(|error| panic!("{error}"));
    tcp.set_write_timeout(Some(IO_TIMEOUT))
        .unwrap_or_else(|error| panic!("{error}"));
    let mut session = BufReader::new(tcp);
    smtp_reply(&mut session, "220 integration relay\r\n");
    assert_eq!(smtp_line(&mut session), "EHLO layerx-identity\r\n");
    smtp_reply(&mut session, "250-localhost\r\n250 PIPELINING\r\n");
    assert_eq!(
        smtp_line(&mut session),
        "MAIL FROM:<identity@example.com>\r\n"
    );
    smtp_reply(&mut session, "250 sender accepted\r\n");
    assert_eq!(
        smtp_line(&mut session),
        format!("RCPT TO:<{SIGNUP_EMAIL}>\r\n")
    );
    smtp_reply(&mut session, "250 recipient accepted\r\n");
    assert_eq!(smtp_line(&mut session), "DATA\r\n");
    smtp_reply(&mut session, "354 send message\r\n");
    let mut message = String::new();
    loop {
        let line = smtp_line(&mut session);
        if line == ".\r\n" {
            break;
        }
        message.push_str(&line);
    }
    assert!(message.contains("Subject: Verify your Paxeer X Network account\r\n"));
    assert!(message.contains(&format!("To: <{SIGNUP_EMAIL}>\r\n")));
    if acknowledge {
        smtp_reply(&mut session, "250 queued\r\n");
        assert_eq!(smtp_line(&mut session), "QUIT\r\n");
        smtp_reply(&mut session, "221 goodbye\r\n");
    }
    message
}

fn delivered_token(message: &str) -> String {
    let token = message
        .lines()
        .find_map(|line| line.strip_prefix("Your verification code is "))
        .unwrap_or_else(|| panic!("verification code absent from SMTP message"));
    assert!(token
        .strip_prefix("vfy_")
        .is_some_and(|secret| valid_hex(secret, SIGNUP_TOKEN_BYTES)));
    token.to_owned()
}

fn assert_session(server: &RunningIdentity, session: &Value) {
    assert_eq!(session["tenant"], "beta");
    assert_eq!(session["sub"], SIGNUP_SUB);
    let token = text(session, "token");
    assert!(parse_session_token(&token).is_some());
    let reply = server.request(
        "/v1/introspect",
        Some(Service::Gateway),
        &json!({"token":token}),
    );
    assert_eq!(reply.status, 200, "{}", reply.body);
    assert_eq!(
        reply.body,
        json!({"active":true,"sub":SIGNUP_SUB,"allowed_signer_public_keys":[SIGNUP_SIGNER]})
    );
    let dashboard = server.request(
        "/v1/sessions/introspect",
        Some(Service::Dashboard),
        &json!({"token":token}),
    );
    assert_eq!(dashboard.status, 200, "{}", dashboard.body);
    assert_eq!(
        dashboard.body,
        json!({"active":true,"sub":SIGNUP_SUB,"csrf_token":session["csrf_token"]})
    );
}

struct SignupOutcome {
    token: String,
    verified: SignupReply,
    duplicate: Value,
}
struct CreditOutcome {
    grant: SignupReply,
    grant_id: String,
    settled: SignupReply,
    settled_at: u64,
}

fn begin_signup(fixture: &SignupFixture) -> SignupOutcome {
    let server = fixture.open();
    for path in [
        "/v1/signup",
        "/v1/signup/verify",
        "/v1/starter-credits",
        "/v1/starter-credits/settle",
    ] {
        assert_refused(
            &server.request(path, None, &signup_body()),
            401,
            "service_token_required",
        );
        for service in Service::ALL
            .into_iter()
            .filter(|service| *service != Service::Provisioning)
        {
            assert_refused(
                &server.request(path, Some(service), &signup_body()),
                403,
                "service_not_permitted",
            );
        }
    }
    let pending = server.provision("/v1/signup", &signup_body());
    assert_eq!(pending.status, 200, "{}", pending.body);
    assert_eq!(pending.body["pending"], true);
    assert_eq!(pending.body["sub"], SIGNUP_SUB);
    assert_eq!(pending.body["tenant"], "beta");
    assert!(pending.body.get("token").is_none());
    assert_eq!(fixture.queued().len(), 1);
    assert_refused(&server.grant(SIGNUP_SUB), 403, "verification_required");
    assert_refused(
        &server.provision("/v1/sessions", &json!({"sub":SIGNUP_SUB})),
        404,
        "principal_not_found",
    );
    drop(server);

    let server = fixture.open();
    let message = fixture.deliver(true);
    let token = delivered_token(&message);
    assert!(message.contains(&format!(
        "It expires at unix time {}.",
        pending.body["expires_at"]
    )));
    assert!(fixture.queued().is_empty());
    {
        let store = server
            .shared
            .store
            .lock()
            .unwrap_or_else(|error| panic!("{error}"));
        let signup = store
            .pending_signup(&sha256_hex(token.as_bytes()))
            .unwrap_or_else(|| panic!("durable pending signup"));
        assert_eq!(signup.sub, SIGNUP_SUB);
        assert_eq!(signup.email_digest, sha256_hex(SIGNUP_EMAIL.as_bytes()));
        assert!(store.principal("beta", SIGNUP_SUB).is_none());
    }
    drop(server);

    let server = fixture.open();
    assert_refused(
        &server.verify(&token, OTHER_SUB),
        404,
        "verification_not_found",
    );
    let verified = server.verify(&token, SIGNUP_SUB);
    assert_eq!(verified.status, 200, "{}", verified.body);
    assert_session(&server, &verified.body);
    assert_refused(
        &server.verify(&token, SIGNUP_SUB),
        404,
        "verification_not_found",
    );
    let mut duplicate = signup_body();
    duplicate["sub"] = json!(OTHER_SUB);
    assert_refused(
        &server.provision("/v1/signup", &duplicate),
        409,
        "email_registered",
    );
    assert!(fixture.queued().is_empty());
    drop(server);

    SignupOutcome {
        token,
        verified,
        duplicate,
    }
}

fn assert_recovered(fixture: &SignupFixture, signup: &SignupOutcome, credit: &CreditOutcome) {
    let token = &signup.token;
    let verified = &signup.verified;
    let duplicate = &signup.duplicate;
    let grant_id = &credit.grant_id;
    let settled = &credit.settled;
    let settled_at = credit.settled_at;
    let server = fixture.open();
    let repeated_settle = server.settle(SIGNUP_SUB, grant_id);
    assert_eq!(repeated_settle.status, 200);
    assert_eq!(repeated_settle.body, settled.body);
    assert_refused(
        &server.grant(SIGNUP_SUB),
        409,
        "starter_credit_already_claimed",
    );
    assert_refused(
        &server.provision("/v1/signup", duplicate),
        409,
        "email_registered",
    );
    assert_refused(
        &server.verify(token, SIGNUP_SUB),
        404,
        "verification_not_found",
    );
    assert_session(&server, &verified.body);
    let store = server
        .shared
        .store
        .lock()
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(store
        .pending_signup(&sha256_hex(token.as_bytes()))
        .is_none());
    assert_eq!(
        store.verified_email(SIGNUP_SUB),
        Some(sha256_hex(SIGNUP_EMAIL.as_bytes()).as_str())
    );
    assert!(store.principal("beta", OTHER_SUB).is_none());
    assert!(store.starter_credit(OTHER_SUB).is_none());
    let credit = store
        .starter_credit(SIGNUP_SUB)
        .unwrap_or_else(|| panic!("recovered credit"));
    assert_eq!(&credit.grant_id, grant_id);
    assert_eq!(credit.amount, "1000000");
    assert_eq!(credit.settled_at, Some(settled_at));
    let session = store
        .session(&text(&verified.body, "session_id"))
        .unwrap_or_else(|| panic!("recovered session"));
    assert_eq!(session.principal, SIGNUP_SUB);
    assert_eq!(session.tenant, "beta");
    drop(store);
    let audit = fs::read_to_string(fixture.state().join("audit.log"))
        .unwrap_or_else(|error| panic!("audit: {error}"));
    for event in [
        "signup_requested",
        "signup_verified",
        "starter_credit_granted",
        "starter_credit_settled",
    ] {
        assert!(audit.contains(event));
    }
    for secret in [
        token.clone(),
        text(&verified.body, "token"),
        SIGNUP_EMAIL.to_owned(),
    ] {
        assert!(
            !audit.contains(&secret),
            "audit must not disclose signup secrets"
        );
    }
}

pub(super) fn signup_delivery_session_and_credit_survive_restarts() {
    let fixture = SignupFixture::new(86_400);
    let signup = begin_signup(&fixture);
    let verified = &signup.verified;
    let server = fixture.open();
    assert_session(&server, &verified.body);
    let grant = server.grant(SIGNUP_SUB);
    assert_eq!(grant.status, 200, "{}", grant.body);
    assert_eq!(grant.body["sub"], SIGNUP_SUB);
    assert_eq!(grant.body["amount"], "1000000");
    assert_eq!(grant.body["asset"], "PAX");
    assert_eq!(grant.body["settled_at"], Value::Null);
    let grant_id = text(&grant.body, "grant_id");
    assert!(valid_hex(&grant_id, GRANT_ID_BYTES));
    let retry = server.grant(SIGNUP_SUB);
    assert_eq!(retry.status, 200);
    assert_eq!(retry.body, grant.body);
    assert_refused(
        &server.settle(OTHER_SUB, &grant_id),
        403,
        "verification_required",
    );
    let wrong_grant = if grant_id == "0".repeat(32) {
        "1".repeat(32)
    } else {
        "0".repeat(32)
    };
    assert_refused(
        &server.settle(SIGNUP_SUB, &wrong_grant),
        409,
        "starter_credit_grant_mismatch",
    );
    drop(server);

    let server = fixture.open();
    let recovered_grant = server.grant(SIGNUP_SUB);
    assert_eq!(recovered_grant.status, 200);
    assert_eq!(recovered_grant.body, grant.body);
    let settled = server.settle(SIGNUP_SUB, &grant_id);
    assert_eq!(settled.status, 200, "{}", settled.body);
    let settled_at = settled.body["settled_at"]
        .as_u64()
        .unwrap_or_else(|| panic!("settlement timestamp"));
    assert!(
        settled_at
            >= grant.body["granted_at"]
                .as_u64()
                .unwrap_or_else(|| panic!("grant timestamp"))
    );
    assert_eq!(settled.body["grant_id"], grant_id);
    assert_eq!(settled.body["amount"], grant.body["amount"]);
    drop(server);

    let credit = CreditOutcome {
        grant,
        grant_id,
        settled,
        settled_at,
    };
    assert_recovered(&fixture, &signup, &credit);
}

pub(super) fn expired_delivered_token_is_refused_after_restart() {
    let fixture = SignupFixture::new(1);
    let server = fixture.open();
    let pending = server.provision("/v1/signup", &signup_body());
    assert_eq!(pending.status, 200, "{}", pending.body);
    let token = delivered_token(&fixture.deliver(true));
    let expiry = pending.body["expires_at"]
        .as_u64()
        .unwrap_or_else(|| panic!("expiry"));
    drop(server);
    let deadline = std::time::Instant::now() + Duration::from_secs(3);
    while unix_seconds().unwrap_or_else(|error| panic!("{error}")) < expiry {
        assert!(
            std::time::Instant::now() < deadline,
            "verification TTL did not elapse"
        );
        thread::sleep(Duration::from_millis(10));
    }
    let server = fixture.open();
    assert_refused(
        &server.verify(&token, SIGNUP_SUB),
        410,
        "verification_expired",
    );
    assert_refused(&server.grant(SIGNUP_SUB), 403, "verification_required");
    assert_refused(
        &server.provision("/v1/sessions", &json!({"sub":SIGNUP_SUB})),
        404,
        "principal_not_found",
    );
    drop(server);
    let server = fixture.open();
    assert_refused(
        &server.verify(&token, SIGNUP_SUB),
        410,
        "verification_expired",
    );
    let store = server
        .shared
        .store
        .lock()
        .unwrap_or_else(|error| panic!("{error}"));
    assert!(store
        .pending_signup(&sha256_hex(token.as_bytes()))
        .is_some());
    assert!(store.verified_email(SIGNUP_SUB).is_none());
    assert!(store.principal("beta", SIGNUP_SUB).is_none());
    assert!(store.starter_credit(SIGNUP_SUB).is_none());
}

pub(super) fn interrupted_smtp_delivery_retries_the_durable_token() {
    let fixture = SignupFixture::new(86_400);
    let server = fixture.open();
    let pending = server.provision("/v1/signup", &signup_body());
    assert_eq!(pending.status, 200, "{}", pending.body);
    let queued = fixture.queued();
    assert_eq!(queued.len(), 1);
    let bytes = fs::read(&queued[0]).unwrap_or_else(|error| panic!("queued bytes: {error}"));
    let interrupted = fixture.deliver(false);
    assert_eq!(fixture.queued(), queued);
    assert_eq!(
        fs::read(&queued[0]).unwrap_or_else(|error| panic!("{error}")),
        bytes
    );
    assert_refused(&server.grant(SIGNUP_SUB), 403, "verification_required");
    drop(server);

    let server = fixture.open();
    let delivered = fixture.deliver(true);
    assert_eq!(
        delivered, interrupted,
        "delivery retry must reuse the producer's token and expiry"
    );
    assert!(fixture.queued().is_empty());
    let token = delivered_token(&delivered);
    let verified = server.verify(&token, SIGNUP_SUB);
    assert_eq!(verified.status, 200, "{}", verified.body);
    drop(server);
    let server = fixture.open();
    assert_session(&server, &verified.body);
    assert_refused(
        &server.verify(&token, SIGNUP_SUB),
        404,
        "verification_not_found",
    );
    let smtp = Smtp::parse("smtp://127.0.0.1:1", "identity@example.com")
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(
        smtp.drain(&fixture.outbox()),
        Ok(0),
        "accepted mail is never redelivered"
    );
}

pub(super) fn interrupted_outbox_enqueue_never_verifies_the_subject() {
    let fixture = SignupFixture::new(86_400);
    let server = fixture.open();
    fs::write(
        fixture.outbox(),
        b"delivery interrupted by an unavailable directory",
    )
    .unwrap_or_else(|error| panic!("outbox obstacle: {error}"));
    let failed = server.provision("/v1/signup", &signup_body());
    assert_eq!(failed.status, 503, "{}", failed.body);
    assert_eq!(
        failed.body["error"]["code"],
        "verification_delivery_unavailable"
    );
    assert_eq!(failed.body["error"]["retry"], "after");
    assert_eq!(failed.headers["retry-after"], "30");
    assert!(failed.body.get("token").is_none());
    drop(server);

    let server = fixture.open();
    assert_refused(&server.grant(SIGNUP_SUB), 403, "verification_required");
    assert_refused(
        &server.provision("/v1/sessions", &json!({"sub":SIGNUP_SUB})),
        404,
        "principal_not_found",
    );
    {
        let store = server
            .shared
            .store
            .lock()
            .unwrap_or_else(|error| panic!("{error}"));
        assert!(store.verified_email(SIGNUP_SUB).is_none());
        assert!(store.principal("beta", SIGNUP_SUB).is_none());
        assert_eq!(
            store.signups_on_day(unix_seconds().unwrap_or_else(|error| panic!("{error}")) / 86_400),
            1
        );
    }
    fs::remove_file(fixture.outbox()).unwrap_or_else(|error| panic!("restore delivery: {error}"));
    let retry = server.provision("/v1/signup", &signup_body());
    assert_eq!(retry.status, 200, "{}", retry.body);
    drop(server);
    let server = fixture.open();
    let token = delivered_token(&fixture.deliver(true));
    let verified = server.verify(&token, SIGNUP_SUB);
    assert_eq!(verified.status, 200, "{}", verified.body);
    assert_session(&server, &verified.body);
    let grant = server.grant(SIGNUP_SUB);
    assert_eq!(grant.status, 200, "{}", grant.body);
    drop(server);
    let server = fixture.open();
    let recovered_grant = server.grant(SIGNUP_SUB);
    assert_eq!(recovered_grant.status, 200);
    assert_eq!(recovered_grant.body, grant.body);
    assert_refused(
        &server.verify(&token, SIGNUP_SUB),
        404,
        "verification_not_found",
    );
}
