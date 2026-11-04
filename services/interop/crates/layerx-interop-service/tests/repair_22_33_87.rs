include!("support/settlement_fixture.rs");

#[derive(Debug, Eq, PartialEq)]
struct HttpReplayResponse {
    status: u16,
    headers: BTreeMap<String, String>,
    body: Vec<u8>,
}

fn replay_exchange(
    config: Config,
    method: &str,
    path: &str,
    body: &[u8],
    authorization: &str,
    trace: &str,
) -> TestResult<(Config, HttpReplayResponse)> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let worker = thread::spawn(move || -> TestResult<Config> {
        let (mut stream, _) = listener.accept()?;
        stream.set_read_timeout(Some(TIMEOUT))?;
        stream.set_write_timeout(Some(TIMEOUT))?;
        let request = http::read_request(&mut stream, 512 * 1024)?;
        http::write_response(&mut stream, &server::route(&config, &request))?;
        Ok(config)
    });
    let mut stream = TcpStream::connect_timeout(&address, TIMEOUT)?;
    stream.set_read_timeout(Some(TIMEOUT))?;
    stream.set_write_timeout(Some(TIMEOUT))?;
    write!(stream, "{method} {path} HTTP/1.1\r\nHost: localhost\r\nContent-Type: application/json\r\nAuthorization: {authorization}\r\nX-Trace-Id: {trace}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())?;
    stream.write_all(body)?;
    stream.flush()?;
    let status = line(&mut stream)?;
    let status = text(&status)
        .split_whitespace()
        .nth(1)
        .ok_or("HTTP status")?
        .parse()?;
    let mut headers = BTreeMap::new();
    loop {
        let header = line(&mut stream)?;
        if header.is_empty() {
            break;
        }
        let (name, value) = text(&header).split_once(':').ok_or("HTTP header")?;
        assert!(headers
            .insert(name.to_ascii_lowercase(), value.trim().to_owned())
            .is_none());
    }
    let length = headers
        .get("content-length")
        .ok_or("HTTP content length")?
        .parse::<usize>()?;
    assert!(length <= 512 * 1024);
    let mut body = vec![0; length];
    stream.read_exact(&mut body)?;
    Ok((
        worker.join().map_err(|_| "HTTP listener panicked")??,
        HttpReplayResponse {
            status,
            headers,
            body,
        },
    ))
}

#[test]
fn replayed_http_status_reopens_400_and_409() -> TestResult {
    let _serial = CONFIGURATION.lock().unwrap();
    let case = payment_case()?;
    let dir = Directory::new()?;
    let identity = configure(&dir.0, &case)?;
    let node = Node::start(&case)?;
    let redis = Redis::start(&dir.0, &identity, "127.0.0.1:0".parse()?)?;
    let mut config = gateway(&dir.0, &redis, &node)?;
    let issued = IssuedKey::generate()?;
    let salt = "http-replay-salt";
    let record = KeyRecord {
        key_id: issued.id().into(),
        principal_digest: "33".repeat(32),
        salt: salt.into(),
        secret_digest: gateway_digest(&[
            b"gateway-key-v1",
            salt.as_bytes(),
            issued.secret().as_bytes(),
        ]),
        signer_public_key: hex(SigningKey::from_bytes(&[7; 32]).verifying_key().as_bytes()),
        scopes: "interop:*".into(),
        quota_requests: 100,
        quota_window_seconds: 60,
        epoch: 1,
        disabled: false,
    };
    config
        .store
        .issue_key(&record, "HTTP replay test issuance")?;
    let authorization = format!("LayerX-Key {}:{}", issued.id(), issued.secret());
    let original_trace = layerx_interop_gateway::trace::TraceId::mint([0x87; 16]);
    let replay_trace = layerx_interop_gateway::trace::TraceId::mint([0x88; 16]);
    let mut completions = Vec::new();
    for path in ["/v1/http/ap2/mandates/verify", "/v1/http/ap2/execute"] {
        let body = serde_json::to_vec(&json!({}))?;
        let (next, response) = replay_exchange(
            config,
            "POST",
            path,
            &body,
            &authorization,
            original_trace.as_str(),
        )?;
        config = next;
        assert_eq!(response.status, 400);
        let value: Value = serde_json::from_slice(&response.body)?;
        assert_eq!(value["error"]["code"], "invalid_ap2_request");
        assert_eq!(value["trace"], original_trace.as_str());
        assert_eq!(response.headers["content-type"], "application/json");
        assert!(!response.headers.contains_key("retry-after"));
        completions.push((path, body, response));
    }

    let payment = visa_payment(&case)?;
    let (next, paid) = replay_exchange(
        config,
        "POST",
        PAYMENT_PATH,
        &payment,
        &authorization,
        original_trace.as_str(),
    )?;
    config = next;
    assert_eq!(paid.status, 200, "{}", text(&paid.body));
    let value: Value = serde_json::from_slice(&paid.body)?;
    assert_eq!(value["result"]["state"], "receipt-verified");
    let mut reused_credential = payment.clone();
    reused_credential.push(b'\n');
    let (next, refused) = replay_exchange(
        config,
        "POST",
        PAYMENT_PATH,
        &reused_credential,
        &authorization,
        original_trace.as_str(),
    )?;
    config = next;
    assert_eq!(refused.status, 409, "{}", text(&refused.body));
    let value: Value = serde_json::from_slice(&refused.body)?;
    assert_eq!(value["error"]["code"], "visa_tap_replayed");
    assert!(!refused.headers.contains_key("retry-after"));
    completions.push((PAYMENT_PATH, reused_credential, refused));
    completions.push((PAYMENT_PATH, payment, paid));

    for (path, body, original) in &completions {
        let (next, replay) = replay_exchange(
            config,
            "POST",
            path,
            body,
            &authorization,
            replay_trace.as_str(),
        )?;
        config = next;
        assert_eq!(&replay, original);
    }
    let mut stored = Vec::new();
    for (_, _, response) in &completions {
        let value: Value = serde_json::from_slice(&response.body)?;
        let operation = value["operation"].as_str().ok_or("operation scope")?;
        let record = config.store.operation(operation)?.ok_or("completion")?;
        assert_eq!(
            record.state,
            if response.status == 200 {
                "completed"
            } else {
                "refused"
            }
        );
        stored.push(record);
    }
    let address = redis.listener.address;
    drop(config);
    drop(redis);
    let redis = Redis::start(&dir.0, &identity, address)?;
    config = gateway(&dir.0, &redis, &node)?;
    for ((path, body, original), record) in completions.iter().zip(&stored) {
        assert_eq!(config.store.operation(&record.scope)?, Some(record.clone()));
        let (next, replay) = replay_exchange(
            config,
            "POST",
            path,
            body,
            &authorization,
            replay_trace.as_str(),
        )?;
        config = next;
        assert_eq!(&replay, original);
        let (next, polled) = replay_exchange(
            config,
            "GET",
            &format!("/v1/operations/{}", record.scope),
            b"",
            &authorization,
            replay_trace.as_str(),
        )?;
        config = next;
        assert_eq!(&polled, original);
    }
    let state = node.state.lock().unwrap();
    assert_eq!(state.attempts.len(), 1);
    assert_eq!(state.accepted.len(), 1);
    Ok(())
}
