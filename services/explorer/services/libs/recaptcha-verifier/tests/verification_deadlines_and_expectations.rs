use recaptcha_verifier::{Error, RecaptchaClient, VerifyMode, VerifyRequest};
use std::{
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    sync::mpsc::{self, Receiver, Sender},
    thread::{self, JoinHandle},
    time::{Duration, Instant},
};

const V2: &str = r#"{"success":true,"hostname":"example.com"}"#;
const V3: &str = r#"{"success":true,"hostname":"example.com","score":0.9,"action":"login"}"#;

enum Response {
    Json(&'static str),
    HoldHeaders,
    HoldBody,
}

struct Peer {
    url: String,
    stop: Sender<()>,
    worker: Option<JoinHandle<Option<Vec<u8>>>>,
}

impl Peer {
    fn start(response: Response) -> Self {
        let listener = TcpListener::bind(("127.0.0.1", 0)).unwrap();
        let url = format!(
            "http://{}/recaptcha/api/siteverify",
            listener.local_addr().unwrap()
        );
        listener.set_nonblocking(true).unwrap();
        let (stop, stopped) = mpsc::channel();
        let worker = thread::spawn(move || {
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(e) if e.kind() == std::io::ErrorKind::WouldBlock => {
                        if stopped.recv_timeout(Duration::from_millis(5)).is_ok() {
                            return None;
                        }
                    }
                    Err(e) => panic!("loopback accept failed: {e}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(3)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(3)))
                .unwrap();
            let request = read_request(&mut stream);
            match response {
                Response::Json(body) => {
                    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap();
                }
                Response::HoldHeaders => hold_open(&stopped),
                Response::HoldBody => {
                    stream.write_all(b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 100\r\n\r\n{\"success\":").unwrap();
                    stream.flush().unwrap();
                    hold_open(&stopped);
                }
            }
            Some(request)
        });
        Self {
            url,
            stop,
            worker: Some(worker),
        }
    }

    fn client(&self) -> RecaptchaClient {
        RecaptchaClient::new("test secret&key").with_verify_url(&self.url)
    }

    fn finish(mut self) {
        self.stop.send(()).ok();
        let request = self
            .worker
            .take()
            .unwrap()
            .join()
            .unwrap()
            .expect("provider received no request");
        let request = String::from_utf8(request).unwrap();
        let (headers, body) = request.split_once("\r\n\r\n").unwrap();
        assert!(headers.starts_with("POST /recaptcha/api/siteverify HTTP/1.1\r\n"));
        assert!(
            headers
                .to_ascii_lowercase()
                .contains("content-type: application/x-www-form-urlencoded")
        );
        assert_eq!(body, "secret=test+secret%26key&response=token+%26value");
    }
}

impl Drop for Peer {
    fn drop(&mut self) {
        self.stop.send(()).ok();
        if let Some(worker) = self.worker.take() {
            // Preserve the original assertion panic while still releasing the socket.
            let _ = worker.join();
        }
    }
}

fn hold_open(stopped: &Receiver<()>) {
    let _ = stopped.recv_timeout(Duration::from_secs(15));
}

fn read_request(stream: &mut TcpStream) -> Vec<u8> {
    let mut request = Vec::new();
    let mut buf = [0; 1024];
    loop {
        let n = stream.read(&mut buf).unwrap();
        assert!(n > 0, "client closed before completing its form");
        request.extend_from_slice(&buf[..n]);
        assert!(request.len() <= 16 * 1024);
        if let Some(end) = request.windows(4).position(|part| part == b"\r\n\r\n") {
            let headers = std::str::from_utf8(&request[..end]).unwrap();
            let length: usize = headers
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse().unwrap())
                })
                .expect("form must have content-length");
            if request.len() >= end + 4 + length {
                assert_eq!(request.len(), end + 4 + length);
                return request;
            }
        }
    }
}

fn token() -> VerifyRequest<'static> {
    VerifyRequest::new("token &value")
}

async fn check_response(
    body: &'static str,
    request: VerifyRequest<'_>,
) -> Result<recaptcha_verifier::VerifyResponse, Error> {
    let peer = Peer::start(Response::Json(body));
    let result = peer.client().verify(request).await;
    peer.finish();
    result
}

#[tokio::test]
async fn recaptcha_http_default_deadline_covers_open_body() {
    let peer = Peer::start(Response::HoldBody);
    let start = Instant::now();
    let result = tokio::time::timeout(Duration::from_secs(12), peer.client().verify(token())).await;
    let elapsed = start.elapsed();
    peer.finish();
    let error = result
        .expect("default client exceeded its total deadline")
        .unwrap_err();
    assert!(
        matches!(error, Error::HttpRequest(ref cause) if cause.is_timeout()),
        "{error:?}"
    );
    assert!(elapsed >= Duration::from_secs(9));
    assert!(elapsed < Duration::from_secs(12));
}

#[tokio::test]
async fn recaptcha_http_override_propagates_header_and_body_timeouts() {
    for response in [Response::HoldHeaders, Response::HoldBody] {
        let peer = Peer::start(response);
        let client = RecaptchaClient::with_http_client(
            "test secret&key",
            reqwest::Client::builder()
                .no_proxy()
                .connect_timeout(Duration::from_millis(100))
                .timeout(Duration::from_millis(250))
                .build()
                .unwrap(),
        )
        .with_verify_url(&peer.url);
        let start = Instant::now();
        let result = tokio::time::timeout(Duration::from_secs(2), client.verify(token())).await;
        let elapsed = start.elapsed();
        peer.finish();
        let error = result
            .expect("configured deadline was not honored")
            .unwrap_err();
        assert!(
            matches!(error, Error::HttpRequest(ref cause) if cause.is_timeout()),
            "{error:?}"
        );
        assert!(elapsed >= Duration::from_millis(200));
        assert!(elapsed < Duration::from_secs(2));
    }
}

#[tokio::test]
async fn recaptcha_expected_hostname_rejects_missing_and_null() {
    for body in [r#"{"success":true}"#, r#"{"success":true,"hostname":null}"#] {
        let error = check_response(body, token().with_expected_hostname("example.com"))
            .await
            .unwrap_err();
        assert!(matches!(error, Error::MissingHostname), "{error:?}");
    }
    let error = check_response(
        r#"{"success":true,"score":0.9,"action":"login"}"#,
        VerifyRequest::new_v3("token &value").with_expected_hostname("example.com"),
    )
    .await
    .unwrap_err();
    assert!(matches!(error, Error::MissingHostname), "{error:?}");
}

#[tokio::test]
async fn recaptcha_expected_action_rejects_missing_and_null() {
    for body in [
        r#"{"success":true,"hostname":"example.com","score":0.9}"#,
        r#"{"success":true,"hostname":"example.com","score":0.9,"action":null}"#,
    ] {
        for request in [
            token().with_expected_action("login"),
            token().with_min_score(0.7),
            VerifyRequest::new_v3("token &value"),
        ] {
            let error = check_response(body, request).await.unwrap_err();
            assert!(matches!(error, Error::MissingAction), "{error:?}");
        }
    }
}

#[tokio::test]
async fn recaptcha_v3_requirement_rejects_v2_and_null_score() {
    for body in [
        V2,
        r#"{"success":true,"hostname":"example.com","action":"login"}"#,
        r#"{"success":true,"hostname":"example.com","score":null,"action":"login"}"#,
    ] {
        for request in [
            token().with_expected_action("login"),
            token().with_min_score(0.7),
            VerifyRequest::new_v3("token &value"),
        ] {
            let error = check_response(body, request).await.unwrap_err();
            assert!(matches!(error, Error::MissingScore), "{error:?}");
        }
    }
}

#[tokio::test]
async fn recaptcha_public_expectation_fields_require_v3() {
    let mut action_request = token();
    action_request.expected_action = Some("login");
    let mut score_request = token();
    score_request.min_score = Some(0.7);
    let mut mode_request = token();
    mode_request.mode = VerifyMode::V3;
    for request in [action_request, score_request, mode_request] {
        let error = check_response(V2, request).await.unwrap_err();
        assert!(matches!(error, Error::MissingScore), "{error:?}");
    }
}

#[tokio::test]
async fn recaptcha_v2_and_v3_accept_satisfied_expectations() {
    let v2 = check_response(V2, token().with_expected_hostname("example.com"))
        .await
        .unwrap();
    assert!(v2.success);
    assert!(!v2.is_v3());
    let request = VerifyRequest::new_v3("token &value")
        .with_expected_hostname("example.com")
        .with_expected_action("login")
        .with_min_score(0.7);
    let v3 = check_response(V3, request).await.unwrap();
    assert!(v3.success);
    assert!(v3.is_v3());
    assert_eq!(v3.score(), Some(0.9));
    assert_eq!(v3.action.as_deref(), Some("login"));
}

#[tokio::test]
async fn recaptcha_retains_value_and_provider_refusals() {
    let error = check_response(V2, token().with_expected_hostname("other.example"))
        .await
        .unwrap_err();
    assert!(
        matches!(error, Error::HostnameMismatch { expected, actual } if expected == "other.example" && actual == "example.com")
    );
    let error = check_response(V3, token().with_expected_action("signup"))
        .await
        .unwrap_err();
    assert!(
        matches!(error, Error::ActionMismatch { expected, actual } if expected == "signup" && actual == "login")
    );
    let error = check_response(V3, token().with_min_score(0.95))
        .await
        .unwrap_err();
    assert!(
        matches!(error, Error::ScoreTooLow { score, threshold } if score == 0.9 && threshold == 0.95)
    );
    let error = check_response(
        r#"{"success":true,"hostname":"example.com","score":0.3,"action":"login"}"#,
        VerifyRequest::new_v3("token &value"),
    )
    .await
    .unwrap_err();
    assert!(
        matches!(error, Error::ScoreTooLow { score, threshold } if score == 0.3 && threshold == 0.5)
    );
    let error = check_response(
        r#"{"success":false,"error-codes":["invalid-input-response","timeout-or-duplicate"]}"#,
        VerifyRequest::new_v3("token &value").with_expected_hostname("example.com"),
    )
    .await
    .unwrap_err();
    assert!(
        matches!(error, Error::VerificationFailed(codes) if codes == ["invalid-input-response", "timeout-or-duplicate"])
    );
}
