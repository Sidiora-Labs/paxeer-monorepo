use blockscout_auth::{auth_from_metadata, auth_from_tokens, AuthSuccess, Error, SessionProfile};
use reqwest::{header::HeaderMap, Client, StatusCode};
use serde::Deserialize;
use serde_json::Value;
use std::{
    collections::BTreeMap,
    io::{BufRead, BufReader, Read, Write},
    net::{TcpListener, TcpStream},
    sync::{
        atomic::{AtomicBool, Ordering},
        Arc, Mutex,
    },
    thread::{self, JoinHandle},
    time::Duration,
};
use tonic::metadata::MetadataMap;
use url::Url;

// Session transcripts follow Account.Identity.session_info/2 and Account.Session:
// wallet email is null; an unverified email omits both name and watchlist_id.
// Tokens are inert fixture identifiers, used only by the loopback account peer.
const SESSION_TRANSCRIPTS: &str = r#"{
    "api_key": "account-session-fixture-key",
    "wallet": {
        "jwt": "wallet-session", "csrf_token": "wallet-csrf",
        "id": 101, "uid": "0x0000000000000000000000000000000000000101",
        "email": null, "watchlist_id": 201,
        "response": {
            "id": 101, "uid": "0x0000000000000000000000000000000000000101",
            "email": null, "email_verified": true, "watchlist_id": 201,
            "name": null, "nickname": null, "avatar": null,
            "address_hash": "0x0000000000000000000000000000000000000101"
        }
    },
    "unverified_email": {
        "jwt": "unverified-email-session", "csrf_token": "unverified-email-csrf",
        "id": 102, "uid": "auth0|unverified-email",
        "email": "unverified@example.test", "watchlist_id": null,
        "response": {
            "id": 102, "uid": "auth0|unverified-email",
            "email": "unverified@example.test", "email_verified": false,
            "nickname": null, "avatar": null, "address_hash": null
        }
    },
    "verified_email": {
        "jwt": "verified-email-session", "csrf_token": "verified-email-csrf",
        "id": 103, "uid": "github|666666",
        "email": "john@blockscout.com", "watchlist_id": 203,
        "response": {
            "id": 103, "uid": "github|666666",
            "email": "john@blockscout.com", "email_verified": true,
            "watchlist_id": 203, "name": "John Snow", "nickname": "johnnny",
            "avatar": "https://avatars.githubusercontent.com/u/666666=4",
            "address_hash": null
        }
    }
}"#;

#[derive(Deserialize)]
struct SessionTranscripts {
    api_key: String,
    wallet: Session,
    unverified_email: Session,
    verified_email: Session,
}

struct Backend {
    base_url: String,
    api_key: Option<String>,
    wallet: Session,
    unverified_email: Session,
    verified_email: Session,
    server: AccountServer,
}

#[derive(Clone, Deserialize)]
struct Session {
    jwt: String,
    csrf_token: String,
    id: i64,
    uid: String,
    email: Option<String>,
    watchlist_id: Option<i64>,
    response: Value,
}

struct AccountRequest {
    method: String,
    target: String,
    headers: BTreeMap<String, String>,
    body: Vec<u8>,
}

impl AccountRequest {
    fn read(stream: &mut TcpStream) -> Self {
        let mut reader = BufReader::new(stream);
        let mut line = String::new();
        reader.read_line(&mut line).expect("read HTTP request line");
        let mut parts = line.split_whitespace();
        let method = parts.next().expect("HTTP method").to_owned();
        let target = parts.next().expect("HTTP request target").to_owned();
        assert_eq!(parts.next(), Some("HTTP/1.1"));
        let mut headers = BTreeMap::new();
        loop {
            line.clear();
            assert!(reader.read_line(&mut line).expect("read HTTP header") > 0);
            if line == "\r\n" {
                break;
            }
            let (name, value) = line.split_once(':').expect("HTTP header separator");
            headers.insert(name.to_ascii_lowercase(), value.trim().to_owned());
        }
        let length = headers
            .get("content-length")
            .map(|value| value.parse::<usize>().expect("HTTP content length"))
            .unwrap_or(0);
        assert!(length <= 64 * 1024);
        let mut body = vec![0; length];
        reader
            .read_exact(&mut body)
            .expect("read HTTP request body");
        Self {
            method,
            target,
            headers,
            body,
        }
    }

    fn response(&self, api_key: &str, sessions: &[Session]) -> (StatusCode, Value) {
        let endpoint = Url::parse(&format!("http://localhost{}", self.target)).unwrap();
        assert_eq!(endpoint.path(), "/api/account/v2/authenticate");
        assert!(matches!(self.method.as_str(), "GET" | "POST"));
        let query_key = endpoint.query_pairs().find(|(key, _)| key == "api_key");
        if query_key.as_ref().map(|(_, value)| value.as_ref()) != Some(api_key)
            || self.headers.get("api_key").map(String::as_str) != Some(api_key)
        {
            return (
                StatusCode::FORBIDDEN,
                serde_json::json!({"message": "invalid API key"}),
            );
        }
        let session = sessions.iter().find(|session| {
            self.headers.get("cookie") == Some(&format!("_explorer_key={}", session.jwt))
        });
        let Some(session) = session else {
            return (
                StatusCode::UNAUTHORIZED,
                serde_json::json!({"message": "unauthenticated session"}),
            );
        };
        if self.method == "POST" {
            let body: Value = serde_json::from_slice(&self.body).expect("POST JSON body");
            assert_eq!(body, serde_json::json!({"api_key": api_key}));
            if self.headers.get("x-csrf-token") != Some(&session.csrf_token) {
                return (
                    StatusCode::FORBIDDEN,
                    serde_json::json!({"message": "invalid CSRF token"}),
                );
            }
        } else {
            assert!(self.body.is_empty());
            assert!(!self.headers.contains_key("x-csrf-token"));
        }
        (StatusCode::OK, session.response.clone())
    }
}

struct AccountServer {
    base_url: String,
    stop: Arc<AtomicBool>,
    requests: Arc<Mutex<Vec<AccountRequest>>>,
    worker: Option<JoinHandle<()>>,
}

impl AccountServer {
    fn start(api_key: String, sessions: Vec<Session>) -> Self {
        let listener = TcpListener::bind((std::net::Ipv4Addr::LOCALHOST, 0))
            .expect("bind loopback account backend");
        let base_url = format!("http://{}", listener.local_addr().unwrap());
        listener.set_nonblocking(true).unwrap();
        let stop = Arc::new(AtomicBool::new(false));
        let requests = Arc::new(Mutex::new(Vec::new()));
        let worker_stop = Arc::clone(&stop);
        let worker_requests = Arc::clone(&requests);
        let worker = thread::spawn(move || {
            while !worker_stop.load(Ordering::Acquire) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("accept account request: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let request = AccountRequest::read(&mut stream);
                let (status, response) = request.response(&api_key, &sessions);
                let body = serde_json::to_vec(&response).unwrap();
                worker_requests.lock().unwrap().push(request);
                write!(stream,
                    "HTTP/1.1 {} {}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    status.as_u16(), status.canonical_reason().unwrap(), body.len()
                ).unwrap();
                stream
                    .write_all(&body)
                    .expect("write account session response");
            }
        });
        Self {
            base_url,
            stop,
            requests,
            worker: Some(worker),
        }
    }
}

impl Drop for AccountServer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let result = self.worker.take().unwrap().join();
        if !thread::panicking() {
            result.expect("loopback account backend failed");
        }
    }
}

fn backend() -> Backend {
    let sessions: SessionTranscripts =
        serde_json::from_str(SESSION_TRANSCRIPTS).expect("invalid committed session transcripts");
    let server = AccountServer::start(
        sessions.api_key.clone(),
        vec![
            sessions.wallet.clone(),
            sessions.unverified_email.clone(),
            sessions.verified_email.clone(),
        ],
    );
    Backend {
        base_url: server.base_url.clone(),
        api_key: Some(sessions.api_key),
        wallet: sessions.wallet,
        unverified_email: sessions.unverified_email,
        verified_email: sessions.verified_email,
        server,
    }
}

async fn round_trip(backend: &Backend, session: &Session, post: bool) -> AuthSuccess {
    let request_start = backend.server.requests.lock().unwrap().len();
    let host = Url::parse(&backend.base_url).expect("invalid backend base_url");
    let mut endpoint = host.join("/api/account/v2/authenticate").unwrap();
    if let Some(key) = backend.api_key.as_deref() {
        endpoint.query_pairs_mut().append_pair("api_key", key);
    }
    let client = Client::builder()
        .timeout(Duration::from_secs(15))
        .redirect(reqwest::redirect::Policy::none())
        .build()
        .unwrap();
    let mut request = if post {
        client
            .post(endpoint)
            .header("x-csrf-token", &session.csrf_token)
            .json(&serde_json::json!({"api_key": backend.api_key}))
    } else {
        client.get(endpoint)
    }
    .header("cookie", format!("_explorer_key={}", session.jwt));
    if let Some(key) = backend.api_key.as_deref() {
        request = request.header("api_key", key);
    }
    let response = request.send().await.expect("real backend request failed");
    assert_eq!(response.status(), StatusCode::OK);
    let producer: Value = response.json().await.expect("backend did not return JSON");
    assert_eq!(producer["id"], session.id);
    assert_eq!(producer["uid"], session.uid);
    assert_eq!(
        producer["email"],
        serde_json::to_value(&session.email).unwrap()
    );
    assert_eq!(
        producer.get("watchlist_id").and_then(Value::as_i64),
        session.watchlist_id
    );
    if session.watchlist_id.is_none() {
        assert!(producer.get("watchlist_id").is_none());
    }

    let csrf = post.then_some(session.csrf_token.as_str());
    let decoded = auth_from_tokens(&session.jwt, csrf, &host, backend.api_key.as_deref())
        .await
        .expect("backend-created session must authenticate through the production client");
    let mut headers = HeaderMap::new();
    headers.insert(
        "cookie",
        format!("_explorer_key={}", session.jwt).parse().unwrap(),
    );
    if post {
        headers.insert("x-csrf-token", session.csrf_token.parse().unwrap());
    }
    let from_metadata = auth_from_metadata(
        &MetadataMap::from_headers(headers),
        !post,
        &host,
        backend.api_key.as_deref(),
    )
    .await
    .expect("backend-created session must authenticate through the metadata interface");
    for consumer in [&decoded, &from_metadata] {
        assert_eq!(consumer.id, session.id);
        assert_eq!(consumer.uid, session.uid);
        assert_eq!(consumer.email, session.email);
        assert_eq!(consumer.watchlist_id, session.watchlist_id);
        assert_eq!(producer["email_verified"], consumer.email_verified);
        assert_eq!(
            producer["avatar"],
            serde_json::to_value(&consumer.avatar).unwrap()
        );
        assert_eq!(
            producer["name"],
            serde_json::to_value(&consumer.name).unwrap()
        );
        assert_eq!(
            producer["nickname"],
            serde_json::to_value(&consumer.nickname).unwrap()
        );
        assert_eq!(consumer.session_profile(), decoded.session_profile());
        match session.watchlist_id {
            Some(id) => assert_eq!(consumer.require_watchlist().unwrap(), id),
            None => assert!(matches!(
                consumer.require_watchlist(),
                Err(Error::Forbidden(_))
            )),
        }
    }

    let requests = backend.server.requests.lock().unwrap();
    let observed = &requests[request_start..];
    assert_eq!(
        observed.len(),
        3,
        "direct, token and metadata requests must reach the account peer"
    );
    for request in observed {
        assert_eq!(request.method, if post { "POST" } else { "GET" });
        assert_eq!(
            request.headers["cookie"],
            format!("_explorer_key={}", session.jwt)
        );
        if post {
            assert_eq!(request.headers["x-csrf-token"], session.csrf_token);
        }
    }
    drop(requests);

    // Identity, verification and field types remain mandatory in the decoder.
    for field in ["id", "uid", "email", "email_verified"] {
        let mut invalid = producer.clone();
        invalid.as_object_mut().unwrap().remove(field);
        assert!(serde_json::from_value::<AuthSuccess>(invalid).is_err());
    }
    for field in ["email", "watchlist_id", "email_verified"] {
        let mut invalid = producer.clone();
        invalid[field] = serde_json::json!({"invalid": true});
        assert!(serde_json::from_value::<AuthSuccess>(invalid).is_err());
    }
    let mut invalid = producer;
    invalid["watchlist_id"] = Value::Null;
    assert!(serde_json::from_value::<AuthSuccess>(invalid).is_err());
    decoded
}

#[tokio::test]
async fn auth_session_optionality_wallet_get_and_post() {
    let backend = backend();
    assert!(backend.wallet.email.is_none());
    for post in [false, true] {
        let session = round_trip(&backend, &backend.wallet, post).await;
        assert_eq!(session.session_profile(), SessionProfile::Wallet);
        // Identity defaults this flag to true for wallet authentication without an email.
        assert!(session.email_verified);
    }
}

#[tokio::test]
async fn auth_session_optionality_unverified_email_get_and_post() {
    let backend = backend();
    let email = backend
        .unverified_email
        .email
        .as_deref()
        .expect("email required");
    assert!(backend.unverified_email.watchlist_id.is_none());
    for post in [false, true] {
        let session = round_trip(&backend, &backend.unverified_email, post).await;
        assert_eq!(
            session.session_profile(),
            SessionProfile::UnverifiedEmail(email)
        );
        assert!(!session.email_verified);
        assert!(matches!(
            session.require_watchlist(),
            Err(Error::Forbidden(_))
        ));
    }
}

#[tokio::test]
async fn auth_session_optionality_verified_email_get_and_post() {
    let backend = backend();
    let email = backend
        .verified_email
        .email
        .as_deref()
        .expect("email required");
    let watchlist_id = backend
        .verified_email
        .watchlist_id
        .expect("watchlist required");
    for post in [false, true] {
        let session = round_trip(&backend, &backend.verified_email, post).await;
        assert_eq!(
            session.session_profile(),
            SessionProfile::VerifiedEmail(email)
        );
        assert!(session.email_verified);
        assert_eq!(session.require_watchlist().unwrap(), watchlist_id);
    }
}

#[tokio::test]
async fn auth_session_optionality_rejects_invalid_credentials() {
    let backend = backend();
    let host = Url::parse(&backend.base_url).unwrap();
    let session = &backend.verified_email;
    for csrf in [None, Some(session.csrf_token.as_str())] {
        assert!(matches!(
            auth_from_tokens("unknown-session", csrf, &host, backend.api_key.as_deref()).await,
            Err(Error::Unauthorized(message)) if message == "unauthenticated session"
        ));
        assert!(matches!(
            auth_from_tokens(&session.jwt, csrf, &host, Some("wrong-api-key")).await,
            Err(Error::Forbidden(message)) if message == "invalid API key"
        ));
    }
    assert!(matches!(
        auth_from_tokens(&session.jwt, Some("wrong-csrf"), &host, backend.api_key.as_deref()).await,
        Err(Error::Forbidden(message)) if message == "invalid CSRF token"
    ));
    let mut headers = HeaderMap::new();
    headers.insert(
        "cookie",
        format!("_explorer_key={}", session.jwt).parse().unwrap(),
    );
    assert!(matches!(
        auth_from_metadata(
            &MetadataMap::from_headers(headers),
            false,
            &host,
            backend.api_key.as_deref()
        )
        .await,
        Err(Error::InvalidCsrfToken(_))
    ));
    assert!(matches!(
        auth_from_metadata(&MetadataMap::new(), true, &host, backend.api_key.as_deref()).await,
        Err(Error::InvalidJwt(_))
    ));
    assert_eq!(backend.server.requests.lock().unwrap().len(), 5);
}
