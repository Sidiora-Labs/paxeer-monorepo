// Reuse the committed backend session transcripts and all verified-session assertions.
include!("repair_22_31_271.rs");

use blockscout_auth::{get_user_info_from_metadata, AccountClient};
use std::{future::Future, time::Instant};

#[derive(Clone, Copy, Debug)]
enum AccountCall {
    TokenGet,
    TokenPost,
    MetadataGet,
    MetadataPost,
    UserInfo,
}

impl AccountCall {
    fn is_post(self) -> bool {
        matches!(self, Self::TokenPost | Self::MetadataPost)
    }

    async fn send(self, client: &AccountClient, host: &Url) -> Result<(), Error> {
        let metadata = account_metadata(self.is_post());
        match self {
            Self::TokenGet | Self::TokenPost => client
                .auth_from_tokens(
                    "session",
                    self.is_post().then_some("csrf"),
                    host,
                    Some("key"),
                )
                .await
                .map(|_| ()),
            Self::MetadataGet | Self::MetadataPost => client
                .auth_from_metadata(&metadata, !self.is_post(), host, Some("key"))
                .await
                .map(|_| ()),
            Self::UserInfo => client
                .get_user_info_from_metadata(&metadata, host, Some("key"))
                .await
                .map(|_| ()),
        }
    }
}

fn account_metadata(post: bool) -> MetadataMap {
    let mut headers = HeaderMap::new();
    headers.insert("cookie", "_explorer_key=session".parse().unwrap());
    if post {
        headers.insert("x-csrf-token", "csrf".parse().unwrap());
    }
    MetadataMap::from_headers(headers)
}

struct StalledAccount {
    host: Url,
    request: Arc<Mutex<Option<AccountRequest>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl StalledAccount {
    fn start(stall_body: bool) -> Self {
        let listener = TcpListener::bind((std::net::Ipv4Addr::LOCALHOST, 0)).unwrap();
        let host = Url::parse(&format!("http://{}", listener.local_addr().unwrap())).unwrap();
        listener.set_nonblocking(true).unwrap();
        let request = Arc::new(Mutex::new(None));
        let observed = Arc::clone(&request);
        let stop = Arc::new(AtomicBool::new(false));
        let worker_stop = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let mut stream = loop {
                if worker_stop.load(Ordering::Acquire) {
                    return;
                }
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("accept stalled account connection: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(20)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(2)))
                .unwrap();
            *observed.lock().unwrap() = Some(AccountRequest::read(&mut stream));
            if stall_body {
                stream.write_all(
                    b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 4096\r\n\r\n{",
                ).unwrap();
                stream.flush().unwrap();
            }
            // Keep the response incomplete until the production transport cancels it.
            let mut byte = [0];
            match stream.read(&mut byte) {
                Ok(0) => (),
                Err(error)
                    if matches!(
                        error.kind(),
                        std::io::ErrorKind::ConnectionReset | std::io::ErrorKind::ConnectionAborted
                    ) =>
                {
                    ()
                }
                result => panic!("deadline did not close the stalled connection: {result:?}"),
            }
        });
        Self {
            host,
            request,
            stop,
            worker: Some(worker),
        }
    }

    async fn finish(mut self) {
        self.stop.store(true, Ordering::Release);
        let worker = self.worker.take().unwrap();
        // Keep driving the HTTP connection task while checking peer-observed cancellation.
        tokio::task::spawn_blocking(move || worker.join())
            .await
            .expect("join stalled account worker")
            .expect("stalled account server failed");
    }

    fn assert_request(&self, call: AccountCall) {
        let guard = self.request.lock().unwrap();
        let request = guard
            .as_ref()
            .expect("production client must reach the account peer");
        assert_eq!(request.method, if call.is_post() { "POST" } else { "GET" });
        let path = if matches!(call, AccountCall::UserInfo) {
            "/api/account/v2/user/info"
        } else {
            "/api/account/v2/authenticate"
        };
        assert_eq!(request.target, format!("{path}?api_key=key"));
        assert_eq!(request.headers["cookie"], "_explorer_key=session");
        assert_eq!(request.headers["api_key"], "key");
        if call.is_post() {
            assert_eq!(request.headers["x-csrf-token"], "csrf");
            assert_eq!(
                serde_json::from_slice::<Value>(&request.body).unwrap(),
                serde_json::json!({"api_key": "key"})
            );
        } else {
            assert!(request.body.is_empty());
            assert!(!request.headers.contains_key("x-csrf-token"));
        }
    }
}

impl Drop for StalledAccount {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !thread::panicking() {
                result.expect("stalled account server failed");
            }
        }
    }
}

async fn assert_upstream_timeout<T: std::fmt::Debug>(
    future: impl Future<Output = Result<T, Error>>,
    upper_bound: Duration,
) {
    let start = Instant::now();
    let error = tokio::time::timeout(upper_bound, future)
        .await
        .expect("account call exceeded its bounded completion window")
        .expect_err("incomplete HTTP responses must fail");
    assert!(
        matches!(error, Error::BlockscoutApi(ref message) if message.starts_with("request timed out:")),
        "{error:?}"
    );
    assert!(start.elapsed() < upper_bound);
}

async fn exercise_stall(stall_body: bool) {
    let client = AccountClient::new(Duration::from_millis(60), Duration::from_millis(180)).unwrap();
    for call in [
        AccountCall::TokenGet,
        AccountCall::TokenPost,
        AccountCall::MetadataGet,
        AccountCall::MetadataPost,
        AccountCall::UserInfo,
    ] {
        let server = StalledAccount::start(stall_body);
        assert_upstream_timeout(call.send(&client, &server.host), Duration::from_secs(1)).await;
        server.assert_request(call);
        server.finish().await;
    }
    // A timed-out request must not poison the injected transport for later sessions.
    let backend = backend();
    let recovered = client
        .auth_from_tokens(
            &backend.verified_email.jwt,
            None,
            &Url::parse(&backend.base_url).unwrap(),
            backend.api_key.as_deref(),
        )
        .await
        .unwrap();
    assert_eq!(recovered.id, backend.verified_email.id);
    assert_eq!(
        recovered.require_watchlist().unwrap(),
        backend.verified_email.watchlist_id.unwrap()
    );
}

#[tokio::test]
async fn auth_http_stalled_headers_are_bounded() {
    exercise_stall(false).await;
}

#[tokio::test]
async fn auth_http_stalled_body_is_bounded() {
    exercise_stall(true).await;
}

#[tokio::test]
async fn auth_http_existing_entry_points_are_bounded() {
    let token = StalledAccount::start(false);
    let metadata_server = StalledAccount::start(true);
    let info = StalledAccount::start(true);
    let metadata = account_metadata(true);
    let bound = Duration::from_secs(12);
    tokio::join!(
        assert_upstream_timeout(
            auth_from_tokens("session", None, &token.host, Some("key")),
            bound
        ),
        assert_upstream_timeout(
            auth_from_metadata(&metadata, false, &metadata_server.host, Some("key")),
            bound
        ),
        assert_upstream_timeout(
            get_user_info_from_metadata(&metadata, &info.host, Some("key")),
            bound
        ),
    );
    token.assert_request(AccountCall::TokenGet);
    metadata_server.assert_request(AccountCall::MetadataPost);
    info.assert_request(AccountCall::UserInfo);
    token.finish().await;
    metadata_server.finish().await;
    info.finish().await;
}

#[test]
fn auth_http_requires_finite_nonzero_deadlines() {
    assert!(matches!(
        AccountClient::new(Duration::ZERO, Duration::from_secs(1)),
        Err(Error::BlockscoutApi(_))
    ));
    assert!(matches!(
        AccountClient::new(Duration::from_secs(1), Duration::ZERO),
        Err(Error::BlockscoutApi(_))
    ));
}

#[tokio::test]
async fn auth_fixtures_session_variants_through_injected_client() {
    let backend = backend();
    let host = Url::parse(&backend.base_url).unwrap();
    let client = AccountClient::new(Duration::from_secs(1), Duration::from_secs(2)).unwrap();
    for session in [
        &backend.wallet,
        &backend.unverified_email,
        &backend.verified_email,
    ] {
        for post in [false, true] {
            let consumer = client
                .clone()
                .auth_from_tokens(
                    &session.jwt,
                    post.then_some(session.csrf_token.as_str()),
                    &host,
                    backend.api_key.as_deref(),
                )
                .await
                .unwrap();
            assert_eq!(consumer.id, session.id);
            assert_eq!(consumer.uid, session.uid);
            assert_eq!(consumer.email, session.email);
            assert_eq!(consumer.watchlist_id, session.watchlist_id);
            assert_eq!(
                serde_json::to_value(consumer.email_verified).unwrap(),
                session.response["email_verified"]
            );
            for (value, field) in [
                (&consumer.avatar, "avatar"),
                (&consumer.name, "name"),
                (&consumer.nickname, "nickname"),
            ] {
                assert_eq!(
                    serde_json::to_value(value).unwrap(),
                    session.response[field]
                );
            }
            match consumer.session_profile() {
                SessionProfile::Wallet => {
                    assert!(consumer.email.is_none());
                    assert!(consumer.email_verified);
                }
                SessionProfile::UnverifiedEmail(email) => {
                    assert_eq!(Some(email), session.email.as_deref());
                    assert!(!consumer.email_verified);
                    assert!(matches!(
                        consumer.require_watchlist(),
                        Err(Error::Forbidden(_))
                    ));
                }
                SessionProfile::VerifiedEmail(email) => {
                    assert_eq!(email, "john@blockscout.com");
                    assert!(consumer.email_verified);
                    assert_eq!(consumer.require_watchlist().unwrap(), 203);
                    assert_eq!(consumer.name.as_deref(), Some("John Snow"));
                    assert_eq!(consumer.nickname.as_deref(), Some("johnnny"));
                    assert_eq!(
                        consumer.avatar.as_deref(),
                        Some("https://avatars.githubusercontent.com/u/666666=4")
                    );
                }
            }
        }
    }
    assert_eq!(backend.server.requests.lock().unwrap().len(), 6);
}
