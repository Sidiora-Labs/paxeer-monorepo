use std::collections::BTreeMap;
use std::error::Error;
use std::io::{Read, Write};
use std::net::{IpAddr, SocketAddr, TcpListener, TcpStream, UdpSocket};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use k256::ecdsa::SigningKey;
use native_tls::{Certificate, Identity, TlsAcceptor};
use openssl::asn1::Asn1Time;
use openssl::bn::{BigNum, MsbOption};
use openssl::ec::{EcGroup, EcKey};
use openssl::hash::MessageDigest;
use openssl::nid::Nid;
use openssl::pkey::PKey;
use openssl::x509::extension::{
    BasicConstraints, ExtendedKeyUsage, KeyUsage, SubjectAlternativeName,
};
use openssl::x509::{X509Builder, X509NameBuilder};
use serde_json::{json, Value};
use x_websearch::api::{self, ApiClient, ApiError, ApiHeader, ApiPayload, Credential, Envelope};
use x_websearch::config::{FetchLimits, Refusal};
use x_websearch::content::ContentStore;
use x_websearch::fetch::{FetchError, Fetcher, HttpClient, Url};
use x_websearch::index::WebIndex;
use x_websearch::payment::{system_clock, PaymentGate};
use x_websearch::resolve::Resolver;
use x_websearch::search::{self, MAX_RESULTS};
use x_websearch::watch::{
    decode_program_requested, decode_requested, hex0x, program_requested_topic, requested_topic,
    EvmRpc, RequestWatcher, WatchOwnership,
};
use x_websearch::{Config, Limits, Response, RouteTable, Server};

type Checked<T = ()> = Result<T, Box<dyn Error>>;
const CHAIN: u64 = 125;
const CREDENTIAL: &str = "loopback-framing-credential";

struct Scratch(PathBuf);
impl Scratch {
    fn new(label: &str) -> Checked<Self> {
        let path =
            std::env::temp_dir().join(format!("repair-22-33-118-{}-{label}", std::process::id()));
        std::fs::create_dir(&path)?;
        Ok(Self(path))
    }
}
impl Drop for Scratch {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.0);
    }
}
fn key(number: u8) -> Checked<SigningKey> {
    let mut scalar = [0; 32];
    scalar[31] = number;
    Ok(SigningKey::from_slice(&scalar)?)
}
fn limits(milliseconds: u64, allow_loopback: bool) -> FetchLimits {
    FetchLimits {
        connect_timeout_ms: milliseconds.min(1000),
        total_timeout_ms: milliseconds,
        max_body_bytes: 2_097_152,
        max_redirects: 3,
        allow_loopback,
    }
}

fn certificate() -> Checked<(Identity, Certificate)> {
    let group = EcGroup::from_curve_name(Nid::X9_62_PRIME256V1)?;
    let key = PKey::from_ec_key(EcKey::generate(&group)?)?;
    let mut name = X509NameBuilder::new()?;
    name.append_entry_by_text("CN", "127.0.0.1")?;
    let name = name.build();
    let mut builder = X509Builder::new()?;
    builder.set_version(2)?;
    let mut serial = BigNum::new()?;
    serial.rand(64, MsbOption::MAYBE_ZERO, false)?;
    builder.set_serial_number(serial.to_asn1_integer()?.as_ref())?;
    builder.set_subject_name(&name)?;
    builder.set_issuer_name(&name)?;
    builder.set_pubkey(&key)?;
    builder.set_not_before(Asn1Time::days_from_now(0)?.as_ref())?;
    builder.set_not_after(Asn1Time::days_from_now(1)?.as_ref())?;
    builder.append_extension(BasicConstraints::new().critical().ca().build()?)?;
    builder.append_extension(
        KeyUsage::new()
            .critical()
            .digital_signature()
            .key_cert_sign()
            .build()?,
    )?;
    builder.append_extension(ExtendedKeyUsage::new().server_auth().build()?)?;
    builder.append_extension(
        SubjectAlternativeName::new()
            .ip("127.0.0.1")
            .build(&builder.x509v3_context(None, None))?,
    )?;
    builder.sign(&key, MessageDigest::sha256())?;
    let certificate = builder.build();
    let pem = certificate.to_pem()?;
    Ok((
        Identity::from_pkcs8(&pem, &key.private_key_to_pem_pkcs8()?)?,
        Certificate::from_pem(&pem)?,
    ))
}

fn read_request(stream: &mut impl Read) -> Checked<(String, Vec<u8>)> {
    let mut bytes = Vec::new();
    let mut chunk = [0; 4096];
    loop {
        if let Some(end) = bytes.windows(4).position(|part| part == b"\r\n\r\n") {
            let head = String::from_utf8(bytes[..end].to_vec())?;
            let length = head
                .lines()
                .find_map(|line| {
                    let (name, value) = line.split_once(':')?;
                    name.eq_ignore_ascii_case("content-length")
                        .then(|| value.trim().parse::<usize>().ok())?
                })
                .unwrap_or(0);
            if bytes.len() >= end + 4 + length {
                return Ok((head, bytes[end + 4..end + 4 + length].to_vec()));
            }
        }
        if bytes.len() > 65_536 {
            return Err("oversized test request".into());
        }
        let count = stream.read(&mut chunk)?;
        if count == 0 {
            return Err("early test request EOF".into());
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
}

struct HttpTranscript {
    address: SocketAddr,
    root: Option<Certificate>,
    calls: Arc<Mutex<Vec<String>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl HttpTranscript {
    fn start(replies: BTreeMap<String, Vec<u8>>, tls: bool, delay: Duration) -> Checked<Self> {
        let (acceptor, root) = if tls {
            let (identity, root) = certificate()?;
            (Some(TlsAcceptor::new(identity)?), Some(root))
        } else {
            (None, None)
        };
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let calls = Arc::new(Mutex::new(Vec::new()));
        let served = Arc::clone(&calls);
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            for incoming in listener.incoming() {
                if stopping.load(Ordering::Acquire) {
                    break;
                }
                let Ok(stream) = incoming else {
                    break;
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                stream
                    .set_write_timeout(Some(Duration::from_secs(3)))
                    .unwrap();
                if let Some(acceptor) = &acceptor {
                    if let Ok(mut stream) = acceptor.accept(stream) {
                        Self::respond(&mut stream, &replies, &served, true, delay);
                        let _ = stream.shutdown();
                    }
                } else {
                    Self::respond(&mut { stream }, &replies, &served, false, delay);
                }
            }
        });
        Ok(Self {
            address,
            root,
            calls,
            stop,
            worker: Some(worker),
        })
    }
    fn respond(
        stream: &mut impl ReadWrite,
        replies: &BTreeMap<String, Vec<u8>>,
        calls: &Mutex<Vec<String>>,
        credentials: bool,
        delay: Duration,
    ) {
        let Ok((head, _)) = read_request(stream) else {
            return;
        };
        let target = head
            .split_whitespace()
            .nth(1)
            .unwrap_or_default()
            .to_owned();
        let authenticated = head
            .lines()
            .any(|line| line == format!("X-Api-Key: {CREDENTIAL}"));
        calls.lock().unwrap().push(head);
        let response = if credentials && !authenticated {
            b"HTTP/1.1 401 Unauthorized\r\nContent-Length: 0\r\n\r\n".as_slice()
        } else if let Some(reply) = replies.get(&target) {
            reply.as_slice()
        } else {
            b"HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\n\r\n".as_slice()
        };
        thread::sleep(delay);
        // Adversarial transcripts are intentionally cut off by the client's
        // framing refusal. A broken pipe here is an expected peer close.
        let _ = stream.write_all(response);
        let _ = stream.flush();
    }
    fn url(&self, target: &str) -> String {
        format!(
            "{}://{}{target}",
            if self.root.is_some() { "https" } else { "http" },
            self.address
        )
    }
}
trait ReadWrite: Read + Write {}
impl<T: Read + Write> ReadWrite for T {}
impl Drop for HttpTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            worker.join().unwrap();
        }
    }
}

fn chunked(count: usize, extension: usize) -> Vec<u8> {
    let mut wire =
        b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nTransfer-Encoding: chunked\r\n\r\n"
            .to_vec();
    let line = format!("1;{}\r\na\r\n", "x".repeat(extension));
    for _ in 0..count {
        wire.extend_from_slice(line.as_bytes());
    }
    wire.extend_from_slice(b"0\r\n\r\n");
    wire
}
fn framing_transcripts() -> BTreeMap<String, Vec<u8>> {
    let mut replies = BTreeMap::new();
    replies.insert("/tiny".into(), chunked(20_000, 0));
    replies.insert("/chunks".into(), chunked(60_000, 0));
    replies.insert("/extensions".into(), chunked(400, 1000));
    replies.insert("/completed-line".into(), chunked(1, 1023));
    let mut trailers = chunked(1, 0);
    trailers.truncate(trailers.len() - 2);
    for _ in 0..100 {
        trailers.extend_from_slice(format!("X-Trailer: {}\r\n", "x".repeat(1000)).as_bytes());
    }
    trailers.extend_from_slice(b"\r\n");
    replies.insert("/trailers".into(), trailers);
    let mut line = chunked(1, 0);
    line.truncate(line.len() - 2);
    line.extend_from_slice(format!("X: {}\r\n\r\n", "x".repeat(65_534)).as_bytes());
    replies.insert("/completed-trailer".into(), line);
    let head = format!(
        "HTTP/1.1 200 OK\r\nX: {}\r\nContent-Length: 0\r\n\r\n",
        "x".repeat(65_498)
    );
    assert_eq!(head.find("\r\n\r\n"), Some(65_537));
    replies.insert("/completed-head".into(), head.into_bytes());
    let informational = format!(
        "HTTP/1.1 103 Early Hints\r\nX: {}\r\n\r\n",
        "x".repeat(1000)
    );
    let mut heads = informational.repeat(70).into_bytes();
    heads.extend_from_slice(b"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n");
    replies.insert("/informational".into(), heads);
    replies.insert(
        "/body-limit".into(),
        b"HTTP/1.1 200 OK\r\nContent-Length: 2097153\r\n\r\n".to_vec(),
    );
    replies.insert("/json".into(), b"HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: 11\r\n\r\n{\"ok\":true}".to_vec());
    replies
}
const MALFORMED: [&str; 7] = [
    "/chunks",
    "/extensions",
    "/completed-line",
    "/trailers",
    "/completed-trailer",
    "/completed-head",
    "/informational",
];

#[test]
fn fetch_framing_memory_is_bounded_on_the_real_http_client() -> Checked {
    let peer = HttpTranscript::start(framing_transcripts(), false, Duration::ZERO)?;
    let client = HttpClient::new(Duration::from_secs(1))?;
    let get = |target: &str| -> Result<_, FetchError> {
        client.get(
            &Url::parse(&peer.url(target))?,
            peer.address,
            Instant::now() + Duration::from_secs(2),
            2_097_152,
            &[],
        )
    };
    let tiny = get("/tiny")?;
    assert_eq!(tiny.body, vec![b'a'; 20_000]);
    assert!(
        tiny.wire_buffer_peak <= 32_768,
        "consumed tiny chunks retained: {}",
        tiny.wire_buffer_peak
    );
    for target in MALFORMED {
        let started = Instant::now();
        assert_eq!(get(target), Err(FetchError::MalformedResponse), "{target}");
        assert!(started.elapsed() < Duration::from_secs(2), "{target}");
    }
    assert_eq!(get("/body-limit"), Err(FetchError::BodyTooLarge));
    Ok(())
}

fn payload(url: String) -> ApiPayload {
    ApiPayload {
        method: api::METHOD_GET,
        level: api::LEVEL_MAJORITY,
        attestor: [0; 20],
        url,
        headers: Vec::new(),
        body: Vec::new(),
        pointers: Vec::new(),
        envelopes: Vec::new(),
    }
}

#[test]
fn api_framing_memory_is_bounded_over_tls_with_credentials() -> Checked {
    let peer = HttpTranscript::start(framing_transcripts(), true, Duration::ZERO)?;
    let fetcher = Arc::new(Fetcher::new(limits(2000, true))?);
    let client = ApiClient::new(fetcher, &[peer.root.as_ref().ok_or("TLS root")?.clone()])?;
    let plaintext = Credential::encode(&[ApiHeader::new("X-Api-Key", CREDENTIAL)])?;
    let credential = Credential::decode(&plaintext, &[])?;
    let tiny = client.call(&payload(peer.url("/tiny")), Some(&credential))?;
    assert_eq!(tiny.body, vec![b'a'; 20_000]);
    assert!(
        tiny.wire_buffer_peak <= 32_768,
        "TLS wire retained {} bytes",
        tiny.wire_buffer_peak
    );
    for target in MALFORMED {
        assert_eq!(
            client.call(&payload(peer.url(target)), Some(&credential)),
            Err(ApiError::Fetch(FetchError::MalformedResponse)),
            "{target}"
        );
    }
    assert_eq!(
        client.call(&payload(peer.url("/body-limit")), Some(&credential)),
        Err(ApiError::Fetch(FetchError::BodyTooLarge))
    );
    assert_eq!(
        client.call(&payload(peer.url("/json")), None),
        Err(ApiError::Status(401))
    );

    // Exercise the credential producer, payload codec, TLS transport and
    // attested-answer consumer together, and refuse tampered envelopes.
    let signer = key(1)?;
    let ephemeral = key(2)?;
    let recipient =
        k256::PublicKey::from_sec1_bytes(signer.verifying_key().to_encoded_point(true).as_bytes())?;
    let mut request = payload(peer.url("/json"));
    request.pointers = vec!["/ok".into()];
    let envelope = api::seal_envelope_with(
        &recipient,
        &ephemeral,
        [7; 12],
        &request.origin()?,
        &plaintext,
    )?;
    request.envelopes.push(Envelope::parse(&envelope)?);
    let raw = request.encode()?;
    let decoded = ApiPayload::decode(&raw)?;
    let answer = api::answer(&client, &signer, &raw, &decoded)?;
    assert_eq!(answer.answer, b"[true]");
    let before = peer.calls.lock().unwrap().len();
    let mut tampered = decoded.clone();
    tampered.envelopes[0].ciphertext[0] ^= 1;
    assert!(matches!(
        api::answer(&client, &signer, &raw, &tampered),
        Err(ApiError::Envelope(_))
    ));
    assert_eq!(
        api::answer(&client, &key(3)?, &raw, &decoded),
        Err(ApiError::NoEnvelope)
    );
    assert_eq!(peer.calls.lock().unwrap().len(), before);
    Ok(())
}

struct DnsTranscript {
    address: SocketAddr,
    questions: Arc<Mutex<Vec<u16>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl DnsTranscript {
    fn start(addresses: Vec<IpAddr>) -> Checked<Self> {
        let socket = UdpSocket::bind("127.0.0.1:0")?;
        let address = socket.local_addr()?;
        let questions = Arc::new(Mutex::new(Vec::new()));
        let recorded = Arc::clone(&questions);
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            let mut bytes = [0; 2048];
            loop {
                let (count, peer) = socket.recv_from(&mut bytes).unwrap();
                if stopping.load(Ordering::Acquire) {
                    break;
                }
                assert!(count >= 16);
                let question = &bytes[..count];
                let kind = u16::from_be_bytes([question[count - 4], question[count - 3]]);
                recorded.lock().unwrap().push(kind);
                let matching: Vec<_> = addresses
                    .iter()
                    .filter(|ip| matches!((kind, ip), (1, IpAddr::V4(_)) | (28, IpAddr::V6(_))))
                    .collect();
                let mut response = question.to_vec();
                response[2] = 0x81;
                response[3] = 0x80;
                response[6..8]
                    .copy_from_slice(&u16::try_from(matching.len()).unwrap().to_be_bytes());
                for ip in matching {
                    response.extend_from_slice(&[0xc0, 0x0c]);
                    response.extend_from_slice(&kind.to_be_bytes());
                    response.extend_from_slice(&[0, 1, 0, 0, 0, 30]);
                    match ip {
                        IpAddr::V4(ip) => {
                            response.extend_from_slice(&[0, 4]);
                            response.extend_from_slice(&ip.octets());
                        }
                        IpAddr::V6(ip) => {
                            response.extend_from_slice(&[0, 16]);
                            response.extend_from_slice(&ip.octets());
                        }
                    }
                }
                socket.send_to(&response, peer).unwrap();
            }
        });
        Ok(Self {
            address,
            questions,
            stop,
            worker: Some(worker),
        })
    }
    fn resolver(&self) -> Checked<Resolver> {
        Ok(Resolver::from_nameservers(&[self.address])?)
    }
}
impl Drop for DnsTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Ok(socket) = UdpSocket::bind("127.0.0.1:0") {
            let _ = socket.send_to(&[0], self.address);
        }
        if let Some(worker) = self.worker.take() {
            worker.join().unwrap();
        }
    }
}

#[test]
fn fetch_dns_deadline_cancels_slow_resolution_for_fetch_api_and_robots() -> Checked {
    // A real DNS peer that keeps the datagrams pending, without detached
    // resolver threads, environment changes or a substituted resolver.
    let dns = UdpSocket::bind("127.0.0.1:0")?;
    let server = dns.local_addr()?;
    dns.set_read_timeout(Some(Duration::from_secs(1)))?;
    let fetcher = Arc::new(Fetcher::with_resolver(
        limits(100, true),
        Resolver::from_nameservers(&[server])?,
    )?);
    let client = ApiClient::new(Arc::clone(&fetcher), &[])?;
    for _ in 0..4 {
        let start = Instant::now();
        assert_eq!(fetcher.fetch("http://slow.test/"), Err(FetchError::Timeout));
        assert!(start.elapsed() < Duration::from_millis(500));
        let mut question = [0; 512];
        let (count, _) = dns.recv_from(&mut question)?;
        assert!(question[..count].windows(4).any(|part| part == b"slow"));
    }
    let start = Instant::now();
    assert_eq!(
        client.call(&payload("https://slow.test/".into()), None),
        Err(ApiError::Fetch(FetchError::Timeout))
    );
    assert!(start.elapsed() < Duration::from_millis(500));
    let mut question = [0; 512];
    dns.recv_from(&mut question)?;

    let mut replies = BTreeMap::new();
    replies.insert(
        "/robots.txt".into(),
        b"HTTP/1.1 302 Found\r\nLocation: http://slow.test/robots.txt\r\nContent-Length: 0\r\n\r\n"
            .to_vec(),
    );
    let peer = HttpTranscript::start(replies, false, Duration::from_millis(70))?;
    let start = Instant::now();
    assert_eq!(fetcher.fetch(&peer.url("/page")), Err(FetchError::Timeout));
    assert!(
        start.elapsed() < Duration::from_millis(500),
        "robots redirect reset the total deadline"
    );
    dns.recv_from(&mut question)?;
    Ok(())
}

#[test]
fn fetch_dns_preserves_all_address_ssrf_checks_and_pins_the_transport() -> Checked {
    for addresses in [
        vec!["93.184.216.34".parse()?, "10.0.0.1".parse()?],
        vec!["93.184.216.34".parse()?, "::1".parse()?],
    ] {
        let dns = DnsTranscript::start(addresses)?;
        let fetcher = Fetcher::with_resolver(limits(1000, false), dns.resolver()?)?;
        assert_eq!(
            fetcher.destination(&Url::parse("http://mixed.test/")?),
            Err(FetchError::ForbiddenDestination)
        );
        assert_eq!(*dns.questions.lock().unwrap(), vec![1, 28]);
    }
    let dns = DnsTranscript::start(vec!["127.0.0.1".parse()?])?;
    let mut replies = BTreeMap::new();
    replies.insert(
        "/page".into(),
        b"HTTP/1.1 200 OK\r\nContent-Type: text/plain\r\nContent-Length: 6\r\n\r\npinned".to_vec(),
    );
    let peer = HttpTranscript::start(replies, false, Duration::ZERO)?;
    let fetcher = Fetcher::with_resolver(limits(1000, true), dns.resolver()?)?;
    let page = fetcher.fetch(&format!("http://pinned.test:{}/page", peer.address.port()))?;
    assert_eq!(page.text, "pinned");
    assert_eq!(page.body, b"pinned");
    assert_eq!(*dns.questions.lock().unwrap(), vec![1, 28]);
    let calls = peer.calls.lock().unwrap();
    assert_eq!(calls.len(), 2);
    assert!(calls
        .iter()
        .all(|call| call.contains(&format!("Host: pinned.test:{}", peer.address.port()))));
    Ok(())
}

#[test]
fn receipt_key_registry_generated_configuration_uses_strict_nested_snake_case() -> Checked {
    // Fill values in the committed operator template without translating a
    // field name or moving receipt_keys out of gateway.
    let valid: Value = serde_json::from_str(include_str!("fixtures/config/valid.json"))?;
    let mut generated = include_str!("../config.example.json").to_owned();
    for (placeholder, value) in [
        ("SEED_URL", valid["seeds"][0].as_str().ok_or("seed")?),
        (
            "GATEWAY_ENDPOINT",
            valid["gateway"]["endpoint"].as_str().ok_or("gateway")?,
        ),
        (
            "EVM_ENDPOINT",
            valid["evm"]["endpoint"].as_str().ok_or("evm")?,
        ),
        (
            "RECEIPT_KEY_ID",
            valid["gateway"]["receipt_keys"][0]["key_id"]
                .as_str()
                .ok_or("key id")?,
        ),
        (
            "RECEIPT_PUBLIC_KEY",
            valid["gateway"]["receipt_keys"][0]["public_key"]
                .as_str()
                .ok_or("key")?,
        ),
        (
            "PAYER_ADDRESS",
            "0x3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c3c",
        ),
    ] {
        generated = generated.replace(&format!("${{X_WEBSEARCH_{placeholder}}}"), value);
    }
    for symbol in ["SID", "PAX", "USDC", "USDL"] {
        for (suffix, field) in [("ASSET_ID", "asset_id"), ("PRICE", "price")] {
            generated = generated.replace(
                &format!("${{X_WEBSEARCH_{symbol}_{suffix}}}"),
                valid["assets"][symbol][field].as_str().ok_or("asset")?,
            );
        }
    }
    assert!(!generated.contains("${"));
    let parsed = Config::parse(&generated)?;
    let window = &parsed.gateway.receipt_keys[0];
    assert_eq!(
        hex0x(&window.sequencer_id()),
        format!(
            "0x{}",
            valid["gateway"]["receipt_keys"][0]["key_id"]
                .as_str()
                .ok_or("key id")?
        )
    );
    assert_eq!(window.first_batch_number(), 0);
    assert_eq!(window.last_batch_number(), 1_000_000);
    let template: Value = serde_json::from_str(&generated)?;
    for (snake, camel) in [
        ("key_id", "keyId"),
        ("public_key", "publicKey"),
        ("first_height", "firstHeight"),
        ("last_height", "lastHeight"),
    ] {
        let mut bad = template.clone();
        let entry = bad["gateway"]["receipt_keys"][0]
            .as_object_mut()
            .ok_or("receipt entry")?;
        let value = entry.remove(snake).ok_or("snake key")?;
        entry.insert(camel.to_owned(), value);
        assert!(
            Config::parse(&bad.to_string()).is_err(),
            "accepted registry alias {camel}"
        );
    }
    for legacy in ["sequencer_id", "sequencer_public_key"] {
        let mut bad = template.clone();
        bad["gateway"][legacy] = json!("12".repeat(32));
        let error = Config::parse(&bad.to_string()).expect_err("retired key must be unknown");
        assert_eq!(error.refusal, Refusal::Unknown);
        assert_eq!(error.field, format!("gateway.{legacy}"));
    }
    let mut empty = template.clone();
    empty["gateway"]["receipt_keys"] = json!([]);
    assert!(Config::parse(&empty.to_string()).is_err());
    let mut overlap = template.clone();
    let entry = overlap["gateway"]["receipt_keys"][0].clone();
    overlap["gateway"]["receipt_keys"]
        .as_array_mut()
        .ok_or("receipt list")?
        .push(entry);
    assert!(Config::parse(&overlap.to_string()).is_err());
    let mut weak = template.clone();
    weak["gateway"]["receipt_keys"][0]["public_key"] = json!("00".repeat(32));
    assert!(Config::parse(&weak.to_string()).is_err());
    let mut reversed = template;
    reversed["gateway"]["receipt_keys"][0]["first_height"] = json!(1_000_001);
    assert!(Config::parse(&reversed.to_string()).is_err());
    Ok(())
}

struct RpcTranscript {
    address: SocketAddr,
    calls: Arc<Mutex<Vec<Value>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl RpcTranscript {
    fn start(handler: impl Fn(&Value) -> Result<Value, Value> + Send + 'static) -> Checked<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?;
        let calls = Arc::new(Mutex::new(Vec::new()));
        let recorded = Arc::clone(&calls);
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            for incoming in listener.incoming() {
                if stopping.load(Ordering::Acquire) {
                    break;
                }
                let Ok(mut stream) = incoming else {
                    break;
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(2)))
                    .unwrap();
                let (_, body) = read_request(&mut stream).unwrap();
                let request: Value = serde_json::from_slice(&body).unwrap();
                recorded.lock().unwrap().push(request.clone());
                let response = match handler(&request) {
                    Ok(result) => json!({"jsonrpc":"2.0", "id":request["id"], "result":result}),
                    Err(error) => json!({"jsonrpc":"2.0", "id":request["id"], "error":error}),
                }
                .to_string();
                write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{response}", response.len()).unwrap();
            }
        });
        Ok(Self {
            address,
            calls,
            stop,
            worker: Some(worker),
        })
    }
    fn url(&self) -> String {
        format!("http://{}/rpc", self.address)
    }
    fn rpc(&self) -> Checked<EvmRpc> {
        Ok(EvmRpc::new(&self.url())?)
    }
}
impl Drop for RpcTranscript {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        let _ = TcpStream::connect(self.address);
        if let Some(worker) = self.worker.take() {
            worker.join().unwrap();
        }
    }
}
fn method_not_found() -> Value {
    json!({"code":-32601,"message":"method not found in committed transcript"})
}
fn settlement_peer(receipts: Arc<AtomicBool>, chain: Arc<AtomicU64>) -> Checked<RpcTranscript> {
    let recording: Value = serde_json::from_str(include_str!("fixtures/gateway/metered.json"))?;
    let exchanges = recording["endpoints"]["gateway"]["*"]
        .as_array()
        .ok_or("ledger exchanges")?
        .clone();
    RpcTranscript::start(move |request| match request["method"].as_str() {
        Some("eth_chainId") => Ok(json!(format!("0x{:x}", chain.load(Ordering::Acquire)))),
        Some("pax_sequencerReceipt") if receipts.load(Ordering::Acquire) => Ok(Value::Null),
        Some("eth_call") => exchanges
            .iter()
            .find(|exchange| {
                exchange["method"] == request["method"] && exchange["params"] == request["params"]
            })
            .map(|exchange| exchange["result"].clone())
            .ok_or_else(method_not_found),
        _ => Err(method_not_found()),
    })
}

#[test]
fn settlement_authority_is_shared_with_readiness_and_paid_route_eligibility() -> Checked {
    let scratch = Scratch::new("settlement")?;
    let receipt_support = Arc::new(AtomicBool::new(true));
    let chain = Arc::new(AtomicU64::new(CHAIN));
    let settlement = settlement_peer(Arc::clone(&receipt_support), Arc::clone(&chain))?;
    let watcher = settlement_peer(
        Arc::new(AtomicBool::new(false)),
        Arc::new(AtomicU64::new(CHAIN)),
    )?;
    let mut config: Value = serde_json::from_str(include_str!("fixtures/config/valid.json"))?;
    config["data_dir"] = json!(scratch.0);
    config["gateway"]["endpoint"] = json!(settlement.url());
    config["evm"]["endpoint"] = json!(watcher.url());
    config["evm"]["chain_id"] = json!(CHAIN);
    let config = Config::parse(&config.to_string())?;
    let gate = Arc::new(PaymentGate::new(
        &config,
        &key(3)?,
        x_websearch::conformance_suite()?,
        system_clock,
    )?);
    assert_eq!(gate.settlement_authority().endpoint(), settlement.url());
    assert!(gate.settlement_usable());
    let index = Arc::new(WebIndex::open(&scratch.0)?);
    index.put("https://paxeer.app/", "readiness", "readiness")?;
    index.commit()?;
    let store = Arc::new(ContentStore::open(&scratch.0, &[])?);
    let mut routes = RouteTable::new();
    search::register(&mut routes, &gate, &index, &store)?;
    let readiness = Arc::clone(&gate);
    routes.set_serving_eligibility(move || readiness.settlement_usable())?;
    let ready = Arc::clone(&gate);
    routes.set_readiness(move |_| {
        if ready.settlement_usable() {
            Response::json(200, b"{}".to_vec())
        } else {
            Response::error(503, "settlement_unavailable")
        }
    })?;
    let server = Server::bind("127.0.0.1:0".parse()?, Limits::default(), routes)?.spawn()?;
    let client = HttpClient::new(Duration::from_secs(1))?;
    let get = |path: &str| -> Checked<_> {
        Ok(client.get(
            &Url::parse(&format!("http://{}{path}", server.local_addr()))?,
            server.local_addr(),
            Instant::now() + Duration::from_secs(2),
            65_536,
            &[],
        )?)
    };
    assert_eq!(get("/readyz")?.status, 200);
    let eligible = get("/search?q=readiness")?;
    assert_eq!(eligible.status, 402);
    assert!(eligible.header("PAYMENT-REQUIRED").is_some());
    assert!(
        watcher.calls.lock().unwrap().is_empty(),
        "payment used the watcher authority"
    );
    receipt_support.store(false, Ordering::Release);
    assert_eq!(get("/readyz")?.status, 503);
    assert_eq!(get("/search?q=readiness")?.status, 503);
    receipt_support.store(true, Ordering::Release);
    chain.store(CHAIN + 1, Ordering::Release);
    assert_eq!(
        get("/search?q=readiness")?.status,
        503,
        "wrong-chain receipt authority admitted payment"
    );
    chain.store(CHAIN, Ordering::Release);
    assert_eq!(get("/search?q=readiness")?.status, 402);
    assert!(settlement
        .calls
        .lock()
        .unwrap()
        .iter()
        .any(|call| call["method"] == "eth_call"));
    assert!(settlement
        .calls
        .lock()
        .unwrap()
        .iter()
        .any(|call| call["method"] == "pax_sequencerReceipt"));
    server.shutdown()?;
    Ok(())
}

fn mixed_logs(rounds: u64) -> Checked<Vec<Value>> {
    let recording: Value = serde_json::from_str(include_str!("fixtures/evm/watch.json"))?;
    let original = recording["endpoints"]["evm"]["*"][1]["result"][0].clone();
    let mut result = Vec::new();
    for round in 0..rounds {
        let mut web = original.clone();
        let mut id = [0; 32];
        id[24..].copy_from_slice(&(round + 1).to_be_bytes());
        web["topics"][1] = json!(hex0x(&id));
        web["blockNumber"] = json!(format!("0x{:x}", 100 + round));
        let mut identity = [0x31; 32];
        identity[24..].copy_from_slice(&(100 + round).to_be_bytes());
        web["blockHash"] = json!(hex0x(&identity));
        web["transactionHash"] = json!(hex0x(&identity));
        web["logIndex"] = json!("0x0");
        // The existing committed program-watch fixture uses these ABI
        // words for kind, payload offset, sequence, length and URL bytes.
        let mut program = web.clone();
        program["topics"] = json!([
            hex0x(&program_requested_topic()),
            hex0x(&[0xa1; 32]),
            hex0x(&id)
        ]);
        program["logIndex"] = json!("0x1");
        program["data"] = json!(concat!(
            "0x",
            "0000000000000000000000000000000000000000000000000000000000000001",
            "0000000000000000000000000000000000000000000000000000000000000060",
            "0000000000000000000000000000000000000000000000000000000000000029",
            "0000000000000000000000000000000000000000000000000000000000000013",
            "68747470733a2f2f7061786565722e6170702f00000000000000000000000000"
        ));
        decode_requested(&web)?;
        decode_program_requested(&program)?;
        result.extend([web, program]);
    }
    Ok(result)
}
fn owned_watcher(
    peer: &RpcTranscript,
    directory: &std::path::Path,
    ownership: WatchOwnership,
) -> Checked<RequestWatcher> {
    Ok(RequestWatcher::open(peer.rpc()?, 0, directory, Some(100))?
        .with_chain_id(CHAIN)?
        .with_ownership(ownership)?)
}
fn journal_empty(directory: &std::path::Path) -> Checked {
    let value: Value = serde_json::from_slice(&std::fs::read(directory.join("pending.json"))?)?;
    assert_eq!(value["entries"], json!([]));
    assert_eq!(value["program_entries"], json!([]));
    Ok(())
}

#[test]
fn watcher_ownership_retires_mixed_traffic_and_recovers_without_orphans() -> Checked {
    let scratch = Scratch::new("watchers")?;
    let logs = mixed_logs(32)?;
    let head = Arc::new(AtomicU64::new(100));
    let current = Arc::clone(&head);
    let peer = RpcTranscript::start(move |request| match request["method"].as_str() {
        Some("eth_chainId") => Ok(json!(format!("0x{CHAIN:x}"))),
        Some("eth_blockNumber") => Ok(json!(format!("0x{:x}", current.load(Ordering::Acquire)))),
        Some("eth_getLogs") => {
            let filter = &request["params"][0];
            assert_eq!(
                filter["topics"],
                json!([[hex0x(&requested_topic()), hex0x(&program_requested_topic())]])
            );
            let number = |value: &Value| {
                u64::from_str_radix(value.as_str().unwrap().trim_start_matches("0x"), 16).unwrap()
            };
            let (from, to) = (number(&filter["fromBlock"]), number(&filter["toBlock"]));
            Ok(Value::Array(
                logs.iter()
                    .filter(|log| (from..=to).contains(&number(&log["blockNumber"])))
                    .cloned()
                    .collect(),
            ))
        }
        _ => Err(method_not_found()),
    })?;
    let ordinary_dir = scratch.0.join("watch");
    let program_dir = scratch.0.join("program-watch");
    // Reopen legacy queues containing both kinds, then adopt the exact
    // ownership assigned by main.rs without discarding owned work.
    for directory in [&ordinary_dir, &program_dir] {
        let mut legacy =
            RequestWatcher::open(peer.rpc()?, 0, directory, Some(100))?.with_chain_id(CHAIN)?;
        assert_eq!(legacy.poll()?.len(), 1);
        assert_eq!(legacy.program_work().len(), 1);
    }
    let mut ordinary = owned_watcher(&peer, &ordinary_dir, WatchOwnership::Ordinary)?;
    let mut program = owned_watcher(&peer, &program_dir, WatchOwnership::Program)?;
    assert_eq!(ordinary.work().len(), 1);
    assert!(ordinary.program_work().is_empty());
    assert!(program.work().is_empty());
    assert_eq!(program.program_work().len(), 1);
    for round in 0..32_u64 {
        head.store(100 + round, Ordering::Release);
        if round > 0 {
            assert_eq!(ordinary.poll()?.len(), 1);
            assert!(program.poll()?.is_empty());
        }
        assert_eq!(ordinary.work().len(), 1);
        assert!(ordinary.program_work().is_empty());
        assert!(program.work().is_empty());
        assert_eq!(program.program_work().len(), 1);
        if round == 15 {
            let saved_web = ordinary.work();
            let saved_program = program.program_work();
            drop(ordinary);
            drop(program);
            ordinary = owned_watcher(&peer, &ordinary_dir, WatchOwnership::Ordinary)?;
            program = owned_watcher(&peer, &program_dir, WatchOwnership::Program)?;
            assert_eq!(ordinary.work(), saved_web);
            assert_eq!(program.program_work(), saved_program);
            assert!(ordinary.program_work().is_empty());
            assert!(program.work().is_empty());
        }
        ordinary.retire(round + 1)?;
        program.retire_program([0xa1; 32], round + 1)?;
        journal_empty(&ordinary_dir)?;
        journal_empty(&program_dir)?;
    }
    drop(ordinary);
    drop(program);
    let mut ordinary = owned_watcher(&peer, &ordinary_dir, WatchOwnership::Ordinary)?;
    let mut program = owned_watcher(&peer, &program_dir, WatchOwnership::Program)?;
    assert!(ordinary.poll()?.is_empty());
    assert!(program.poll()?.is_empty());
    assert!(ordinary.work().is_empty());
    assert!(ordinary.program_work().is_empty());
    assert!(program.work().is_empty());
    assert!(program.program_work().is_empty());
    journal_empty(&ordinary_dir)?;
    journal_empty(&program_dir)?;
    assert_eq!(
        peer.calls
            .lock()
            .unwrap()
            .iter()
            .filter(|call| call["method"] == "eth_getLogs")
            .count(),
        64
    );
    Ok(())
}

#[test]
fn search_tie_expansion_keeps_ten_candidates_and_loads_only_winning_bodies() -> Checked {
    let scratch = Scratch::new("ties")?;
    let index = WebIndex::open(&scratch.0)?;
    let body = format!("tied {}", "common ".repeat(2048));
    for number in (0..4096).rev() {
        index.put(
            &format!("https://paxeer.app/{number:05}"),
            "tied title",
            &body,
        )?;
        if number % 1024 == 0 {
            index.commit()?;
        }
    }
    index.commit()?;
    let expected: Vec<_> = (0..MAX_RESULTS)
        .map(|number| format!("https://paxeer.app/{number:05}"))
        .collect();
    let (results, stats) = search::search_with_stats(&index, "tied")?;
    assert_eq!(
        results
            .iter()
            .map(|result| result.result.url.clone())
            .collect::<Vec<_>>(),
        expected
    );
    assert!(results
        .windows(2)
        .all(|pair| pair[0].score == pair[1].score));
    assert_eq!(stats.matches_considered, 4096);
    assert_eq!(stats.retained_candidates_peak, MAX_RESULTS);
    assert_eq!(stats.stored_bodies_loaded, MAX_RESULTS);
    let canonical = search::search_canonical_bytes(
        "tied",
        &results
            .iter()
            .map(|result| result.result.clone())
            .collect::<Vec<_>>(),
    )?;
    let store = ContentStore::open(&scratch.0, &[])?;
    let digest = store.put(&canonical)?;
    drop(index);
    let reopened = WebIndex::open(&scratch.0)?;
    let (recovered, stats) = search::search_with_stats(&reopened, "tied")?;
    assert_eq!(recovered, results);
    assert_eq!(stats.retained_candidates_peak, MAX_RESULTS);
    assert_eq!(stats.stored_bodies_loaded, MAX_RESULTS);
    assert_eq!(store.get(&digest)?, Some(canonical));
    Ok(())
}

#[test]
fn fetch_dns_tcp_fallback_obeys_the_same_deadline() -> Checked {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let address = listener.local_addr()?;
    let udp = UdpSocket::bind(address)?;
    let worker = thread::spawn(move || -> Result<(), String> {
        udp.set_read_timeout(Some(Duration::from_secs(2)))
            .map_err(|error| error.to_string())?;
        let mut query = [0; 512];
        let (count, client) = udp
            .recv_from(&mut query)
            .map_err(|error| error.to_string())?;
        query[2] = 0x83; // truncated recursive response: retry over TCP
        query[3] = 0x80;
        udp.send_to(&query[..count], client)
            .map_err(|error| error.to_string())?;
        let (mut stream, _) = listener.accept().map_err(|error| error.to_string())?;
        stream
            .set_read_timeout(Some(Duration::from_secs(2)))
            .map_err(|error| error.to_string())?;
        let mut size = [0; 2];
        stream
            .read_exact(&mut size)
            .map_err(|error| error.to_string())?;
        let mut query = vec![0; usize::from(u16::from_be_bytes(size))];
        stream
            .read_exact(&mut query)
            .map_err(|error| error.to_string())?;
        // Keep a response length pending until the production resolver
        // cancels its TCP read and closes the socket at the caller deadline.
        let mut byte = [0; 1];
        let count = stream.read(&mut byte).map_err(|error| error.to_string())?;
        if count != 0 {
            return Err("resolver sent unexpected trailing data".into());
        }
        Ok(())
    });
    let fetcher =
        Fetcher::with_resolver(limits(100, true), Resolver::from_nameservers(&[address])?)?;
    let started = Instant::now();
    assert_eq!(
        fetcher.fetch("http://truncated.test/"),
        Err(FetchError::Timeout)
    );
    assert!(started.elapsed() < Duration::from_millis(500));
    worker
        .join()
        .map_err(|_| "TCP DNS peer panicked")?
        .map_err(|error| -> Box<dyn Error> { error.into() })?;
    Ok(())
}
