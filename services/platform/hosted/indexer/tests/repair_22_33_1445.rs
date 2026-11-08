//! Task 22.33.1445 repair record, audit 22.30.82.
//! names-unbounded-reads: names_history_refuses_over_limit,
//! names_portfolios_refuse_over_limit, names_union_enforces_combined_limit.
//! reverse-forward-binding: primary_names_require_valid_forward_binding.
//! transport-total-deadline: framed_responses_finish_while_peer_stays_open,
//! trickling_http_responses_obey_total_deadline, tls_handshake_obeys_total_deadline,
//! request_writes_obey_total_deadline, malformed_frames_are_refused_before_peer_close.
//! Baseline: no prior gate ledger or repair tests existed; no baseline was run because
//! the explicit qualification order requires complete implementation before either gate.
//! Attempt 1: qualification stopped before tests because the workspace member manifest
//! services/platform/hosted/paxeer/Cargo.toml was absent. Attempt 2 retains all nine
//! tests and the three repairs after task 22.31.465 restored that manifest.
//! Attempt 3: eight cases passed; request_writes_obey_total_deadline failed at
//! 1.68 s for plaintext. Attempt 4 bounds every plaintext/TLS write and rechecks
//! the shared deadline between partial writes without changing any assertion.
//! Wiring request: services/explorer/backend/apps/explorer/lib/explorer/microservice_interfaces/bens.ex
//! (explorer domain) must surface HTTP 422 names_result_limit_exceeded and maximum_results=500.
//! Wiring request: apps/wallet/src/lib/pns.ts (wallet domain) must surface the same explicit
//! history/portfolio refusal instead of displaying it as an empty or complete result.

use std::io::{Read, Write};
use std::net::TcpListener;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{mpsc, Arc};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

use layerx_indexer::abi::{keccak, AbiRegistry};
use layerx_indexer::api;
use layerx_indexer::codec::hex0x;
use layerx_indexer::names::{self, NamesConfig};
use layerx_indexer::store::{NameEventRow, NameOrder, Store, Unit, MAXIMUM_NAME_RESULTS};
use layerx_indexer::transport::{split_response, Endpoint, Security};
use layerx_indexer::IndexError;
use rustls::pki_types::{CertificateDer, PrivatePkcs8KeyDer};
use rustls::{ServerConfig, ServerConnection, StreamOwned};
use serde_json::{json, Value};

type Checked<T> = Result<T, Box<dyn std::error::Error + Send + Sync>>;
const OWNER: [u8; 20] = [0xab; 20];
const OTHER: [u8; 20] = [0xcd; 20];
const FUTURE: u64 = 4_102_444_800;
const CERTIFICATE: &[u8] =
    include_bytes!("../../internal/tests/fixtures/repair_22_33_1452_tls/configured-server.der");
const PRIVATE_KEY: &[u8] =
    include_bytes!("../../internal/tests/fixtures/repair_22_33_1452_tls/configured-server-key.der");
const TRUST_ANCHOR: &[u8] =
    include_bytes!("../../internal/tests/fixtures/repair_22_33_1452_tls/configured-ca.der");

fn word(number: u64) -> [u8; 32] {
    let mut value = [0; 32];
    value[24..].copy_from_slice(&number.to_be_bytes());
    value
}

fn address_word(address: [u8; 20]) -> [u8; 32] {
    let mut value = [0; 32];
    value[12..].copy_from_slice(&address);
    value
}

fn string_tail(text: &str) -> Vec<u8> {
    let mut bytes = word(text.len() as u64).to_vec();
    bytes.extend_from_slice(text.as_bytes());
    bytes.resize(bytes.len().div_ceil(32) * 32, 0);
    bytes
}

struct Producer {
    registry: AbiRegistry,
    config: NamesConfig,
}

impl Producer {
    fn new() -> Checked<Self> {
        let config = NamesConfig {
            registry: [1; 20],
            registrar: [2; 20],
            controller: [3; 20],
            resolver: [4; 20],
            reverse: [5; 20],
        };
        let mut registry = AbiRegistry::default();
        for abi in names::names_abi(&config)? {
            registry.insert(abi);
        }
        Ok(Self { registry, config })
    }

    fn decode(
        &self,
        emitter: [u8; 20],
        topics: &[[u8; 32]],
        data: &[u8],
        sender: [u8; 20],
    ) -> Checked<NameEventRow> {
        let (_, event) = self
            .registry
            .decode_log(emitter, topics, data)?
            .ok_or("ABI event missing")?;
        Ok(names::decode(&event, &hex0x(&sender), 1_800_000_000).ok_or("name event missing")?)
    }

    fn register(&self, label: &str, owner: [u8; 20], expiry: u64) -> Checked<NameEventRow> {
        let mut data = [word(128), word(1), word(expiry), word(0)].concat();
        data.extend(string_tail(label));
        self.decode(
            self.config.controller,
            &[
                keccak(b"NameRegistered(string,bytes32,address,uint256,uint256,bytes32)"),
                keccak(label.as_bytes()),
                address_word(owner),
            ],
            &data,
            owner,
        )
    }

    fn target(&self, label: &str, target: [u8; 20]) -> Checked<NameEventRow> {
        self.decode(
            self.config.resolver,
            &[
                keccak(b"AddrChanged(bytes32,address)"),
                names::child_node(names::pax_node(), keccak(label.as_bytes())),
            ],
            &address_word(target),
            OWNER,
        )
    }

    fn reverse(&self, sender: [u8; 20], name: &str) -> Checked<NameEventRow> {
        let mut data = word(32).to_vec();
        data.extend(string_tail(name));
        self.decode(
            self.config.resolver,
            &[
                keccak(b"NameChanged(bytes32,string)"),
                names::reverse_node(&hex0x(&sender)).ok_or("reverse node missing")?,
            ],
            &data,
            sender,
        )
    }
}

fn commit(store: &Store, height: u64, mut name_events: Vec<NameEventRow>) -> Checked<()> {
    for (index, event) in name_events.iter_mut().enumerate() {
        event.height = height;
        event.log_index = index as u64;
        event.tx_hash = format!("0x{height:064x}");
    }
    store.commit(
        &Unit {
            chain: "paxeer".to_owned(),
            position: height,
            hash: format!("block-{height}"),
            parent: if height == 1 {
                String::new()
            } else {
                format!("block-{}", height - 1)
            },
            link: format!("block-{height}"),
            boundary: height,
            transfers: Vec::new(),
            events: Vec::new(),
            name_events,
            holders: Vec::new(),
            assets: Vec::new(),
            accounts: Vec::new(),
        },
        10,
    )?;
    Ok(())
}

fn api_endpoint(store: Arc<Store>) -> Checked<Endpoint> {
    let listener = api::bind("127.0.0.1:0".parse()?, false)?;
    let endpoint = Endpoint::parse(
        &format!("http://{}", listener.local_addr()?),
        Security::Plaintext {
            allow_remote: false,
        },
        Duration::from_secs(3),
    )?;
    thread::spawn(move || api::serve(&listener, &store, None));
    Ok(endpoint)
}

fn answer(endpoint: &Endpoint, target: &str) -> Checked<(u16, Value)> {
    let (status, body) = endpoint.get(target)?;
    Ok((status, serde_json::from_slice(&body)?))
}

fn assert_limit(endpoint: &Endpoint, target: &str) -> Checked<()> {
    let (status, body) = answer(endpoint, target)?;
    assert_eq!(status, 422, "{target}: {body}");
    assert_eq!(
        body,
        json!({"error": {"code":"names_result_limit_exceeded", "retry":"never", "maximum_results":500}})
    );
    Ok(())
}

#[test]
fn names_history_refuses_over_limit() -> Checked<()> {
    assert_eq!(MAXIMUM_NAME_RESULTS, 500);
    let producer = Producer::new()?;
    let store = Arc::new(Store::open_in_memory()?);
    let mut events = vec![producer.register("history", OWNER, FUTURE)?];
    for _ in 1..MAXIMUM_NAME_RESULTS {
        events.push(producer.target("history", OWNER)?);
    }
    commit(&store, 1, events)?;
    let endpoint = api_endpoint(Arc::clone(&store))?;
    for order in ["ASC", "DESC"] {
        let (status, body) = answer(
            &endpoint,
            &format!("/api/v1/domains/history.pax/events?order={order}"),
        )?;
        assert_eq!(status, 200);
        let items = body["items"].as_array().ok_or("history items missing")?;
        assert_eq!(items.len(), 500);
        assert_eq!(
            items[if order == "ASC" { 0 } else { 499 }]["action"],
            "register"
        );
        assert!(body["next_page_params"].is_null());
    }
    commit(&store, 2, vec![producer.target("history", OWNER)?])?;
    for order in [NameOrder::Asc, NameOrder::Desc] {
        assert!(matches!(
            store.name_events(
                &hex0x(&names::child_node(names::pax_node(), keccak(b"history"))),
                order
            ),
            Err(IndexError::NameResultLimit)
        ));
    }
    for order in ["ASC", "DESC"] {
        assert_limit(
            &endpoint,
            &format!("/api/v1/domains/history/events?order={order}"),
        )?;
    }
    assert_eq!(answer(&endpoint, "/api/v1/domains/history")?.0, 200);
    assert!(store
        .name_events("nonexistent", NameOrder::Desc)?
        .is_empty());
    Ok(())
}

#[test]
fn names_portfolios_refuse_over_limit() -> Checked<()> {
    let producer = Producer::new()?;
    let store = Arc::new(Store::open_in_memory()?);
    let mut events = Vec::new();
    for index in 0..500 {
        let label = format!("portfolio-{index:04}");
        events.push(producer.register(&label, OWNER, FUTURE)?);
        events.push(producer.target(&label, OWNER)?);
    }
    commit(&store, 1, events)?;
    let endpoint = api_endpoint(Arc::clone(&store))?;
    let address = hex0x(&OWNER);
    for flags in [
        "owned_by=true",
        "resolved_to=true",
        "owned_by=true&resolved_to=true",
    ] {
        let (status, body) = answer(
            &endpoint,
            &format!(
                "/api/v1/addresses:lookup?address={address}&{flags}&only_active=true&order=ASC"
            ),
        )?;
        assert_eq!(status, 200);
        let items = body["items"].as_array().ok_or("portfolio items missing")?;
        assert_eq!(items.len(), 500);
        assert!(items
            .windows(2)
            .all(|pair| pair[0]["id"].as_str() < pair[1]["id"].as_str()));
    }
    let (status, body) = answer(&endpoint, &format!("/api/v1/addresses/{address}"))?;
    assert_eq!(status, 200);
    assert_eq!(body["resolved_domains_count"], 500);
    assert!(body["domain"].is_null());
    commit(
        &store,
        2,
        vec![
            producer.register("overflow", OWNER, FUTURE)?,
            producer.target("overflow", OWNER)?,
        ],
    )?;
    for active in [false, true] {
        for order in [NameOrder::Asc, NameOrder::Desc] {
            assert!(matches!(
                store.names_owned_by(&address, active, order),
                Err(IndexError::NameResultLimit)
            ));
            assert!(matches!(
                store.names_resolving_to(&address, active, order),
                Err(IndexError::NameResultLimit)
            ));
        }
        for flags in [
            "owned_by=true",
            "resolved_to=true",
            "owned_by=true&resolved_to=true",
        ] {
            assert_limit(
                &endpoint,
                &format!("/api/v1/addresses:lookup?address={address}&{flags}&only_active={active}"),
            )?;
        }
    }
    assert_limit(&endpoint, &format!("/api/v1/addresses/{address}"))?;
    let (status, body) = answer(
        &endpoint,
        &format!(
            "/api/v1/addresses:lookup?address={}&owned_by=true",
            hex0x(&OTHER)
        ),
    )?;
    assert_eq!(status, 200);
    assert_eq!(body["items"], json!([]));
    Ok(())
}

#[test]
fn names_union_enforces_combined_limit() -> Checked<()> {
    let producer = Producer::new()?;
    let store = Arc::new(Store::open_in_memory()?);
    let mut events = Vec::new();
    for index in 0..501 {
        let label = format!("union-{index:04}");
        events.push(producer.register(&label, if index < 250 { OWNER } else { OTHER }, FUTURE)?);
        events.push(producer.target(&label, if index < 250 { OTHER } else { OWNER })?);
    }
    commit(&store, 1, events)?;
    let endpoint = api_endpoint(Arc::clone(&store))?;
    let address = hex0x(&OWNER);
    assert_eq!(
        store.names_owned_by(&address, true, NameOrder::Asc)?.len(),
        250
    );
    assert_eq!(
        store
            .names_resolving_to(&address, true, NameOrder::Asc)?
            .len(),
        251
    );
    assert_limit(
        &endpoint,
        &format!("/api/v1/addresses:lookup?address={address}&owned_by=true&resolved_to=true"),
    )?;
    Ok(())
}

#[test]
fn primary_names_require_valid_forward_binding() -> Checked<()> {
    let producer = Producer::new()?;
    for (label, target, expiry, reverse, valid) in [
        ("valid", Some(OWNER), FUTURE, "VALID.PAX", true),
        ("foreign", Some(OTHER), FUTURE, "foreign.pax", false),
        ("expired", Some(OWNER), 1, "expired.pax", false),
        ("zero-expiry", Some(OWNER), 0, "zero-expiry.pax", false),
        ("unset", None, FUTURE, "unset.pax", false),
        ("missing", Some(OWNER), FUTURE, "absent.pax", false),
    ] {
        let store = Arc::new(Store::open_in_memory()?);
        let mut events = vec![producer.register(label, OTHER, expiry)?];
        if let Some(target) = target {
            events.push(producer.target(label, target)?);
        }
        events.push(producer.reverse(OWNER, reverse)?);
        commit(&store, 1, events)?;
        let endpoint = api_endpoint(Arc::clone(&store))?;
        let address = hex0x(&OWNER).to_ascii_uppercase();
        let record = store.name_for_address(&address)?;
        assert_eq!(record.is_some(), valid, "{label}");
        let (status, body) = answer(&endpoint, &format!("/api/v1/addresses/{address}"))?;
        assert_eq!(status, 200, "{label}: {body}");
        assert_eq!(!body["domain"].is_null(), valid, "{label}: {body}");
        if valid {
            assert_eq!(body["domain"]["name"], "valid.pax");
            assert_eq!(body["domain"]["resolved_address"]["hash"], hex0x(&OWNER));
            commit(&store, 2, vec![producer.target(label, OTHER)?])?;
            assert!(store.name_for_address(&address)?.is_none());
            let (status, body) = answer(&endpoint, &format!("/api/v1/addresses/{address}"))?;
            assert_eq!(status, 200);
            assert!(body["domain"].is_null(), "changed forward target: {body}");
        }
    }
    Ok(())
}

trait Wire: Read + Write {}
impl<T: Read + Write> Wire for T {}

fn peer(
    tls: bool,
    timeout: Duration,
    serve: impl FnOnce(&mut dyn Wire) -> Checked<()> + Send + 'static,
) -> Checked<(Endpoint, JoinHandle<Checked<()>>)> {
    let _ = rustls::crypto::ring::default_provider().install_default();
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let port = listener.local_addr()?.port();
    let endpoint = Endpoint::parse(
        &format!(
            "{}://{}:{port}",
            if tls { "https" } else { "http" },
            if tls { "localhost" } else { "127.0.0.1" }
        ),
        if tls {
            Security::PinnedTls(TRUST_ANCHOR.to_vec())
        } else {
            Security::Plaintext {
                allow_remote: false,
            }
        },
        timeout,
    )?;
    let config = Arc::new(
        ServerConfig::builder()
            .with_no_client_auth()
            .with_single_cert(
                vec![CertificateDer::from(CERTIFICATE.to_vec())],
                PrivatePkcs8KeyDer::from(PRIVATE_KEY.to_vec()).into(),
            )?,
    );
    let worker = thread::spawn(move || -> Checked<()> {
        let (mut socket, _) = listener.accept()?;
        socket.set_read_timeout(Some(Duration::from_secs(3)))?;
        socket.set_write_timeout(Some(Duration::from_secs(3)))?;
        if tls {
            let mut stream = StreamOwned::new(ServerConnection::new(config)?, socket);
            serve(&mut stream)
        } else {
            serve(&mut socket)
        }
    });
    Ok((endpoint, worker))
}

fn join(worker: JoinHandle<Checked<()>>) -> Checked<()> {
    worker.join().map_err(|_| "loopback peer panicked")?
}

fn request_head(stream: &mut dyn Wire) -> Checked<()> {
    let mut head = Vec::new();
    while !head.ends_with(b"\r\n\r\n") {
        let mut byte = [0];
        stream.read_exact(&mut byte)?;
        head.push(byte[0]);
        assert!(
            head.len() <= 16 * 1024,
            "request headers exceeded their bound"
        );
    }
    Ok(())
}

#[test]
fn framed_responses_finish_while_peer_stays_open() -> Checked<()> {
    for tls in [false, true] {
        for chunked in [false, true] {
            let (release, held) = mpsc::sync_channel(1);
            let (endpoint, worker) = peer(tls, Duration::from_secs(1), move |stream| {
                request_head(stream)?;
                let fragments: &[&[u8]] = if chunked {
                    &[
                        b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r",
                        b"\n\r\n1\r\n{\r\n",
                        b"1\r\n}\r\n0\r",
                        b"\n\r\n",
                    ]
                } else {
                    &[
                        b"HTTP/1.1 200 OK\r\nContent-Length:",
                        b" 2\r\n\r",
                        b"\n{",
                        b"}",
                    ]
                };
                for fragment in fragments {
                    stream.write_all(fragment)?;
                    stream.flush()?;
                }
                held.recv_timeout(Duration::from_secs(3))?;
                Ok(())
            })?;
            let started = Instant::now();
            let result = endpoint.get_json("/framed");
            let elapsed = started.elapsed();
            release.send(())?;
            join(worker)?;
            assert_eq!(result?, json!({}), "tls={tls}, chunked={chunked}");
            assert!(
                elapsed < Duration::from_millis(900),
                "complete response waited {elapsed:?}"
            );
        }
    }
    Ok(())
}

#[test]
fn trickling_http_responses_obey_total_deadline() -> Checked<()> {
    for tls in [false, true] {
        for prefix in [
            &b"HTTP/1.1 200 OK\r\nX-Trickle: "[..],
            &b"HTTP/1.1 200 OK\r\nContent-Length: 64\r\n\r\n"[..],
            &b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n40\r\n"[..],
        ] {
            let sent = Arc::new(AtomicUsize::new(0));
            let count = Arc::clone(&sent);
            let (release, held) = mpsc::sync_channel(1);
            let (endpoint, worker) = peer(tls, Duration::from_millis(250), move |stream| {
                request_head(stream)?;
                stream.write_all(prefix)?;
                stream.flush()?;
                for _ in 0..40 {
                    if held.recv_timeout(Duration::from_millis(40)).is_ok() {
                        break;
                    }
                    match stream.write_all(b"x").and_then(|()| stream.flush()) {
                        Ok(()) => {
                            count.fetch_add(1, Ordering::SeqCst);
                        }
                        Err(error)
                            if matches!(
                                error.kind(),
                                std::io::ErrorKind::BrokenPipe
                                    | std::io::ErrorKind::ConnectionReset
                                    | std::io::ErrorKind::ConnectionAborted
                            ) =>
                        {
                            break;
                        }
                        Err(error) => return Err(error.into()),
                    }
                }
                Ok(())
            })?;
            let started = Instant::now();
            let result = endpoint.get("/trickle");
            let elapsed = started.elapsed();
            let _ = release.send(());
            join(worker)?;
            assert!(
                matches!(result, Err(IndexError::Source(_))),
                "tls={tls}: {result:?}"
            );
            assert!(
                elapsed >= Duration::from_millis(150) && elapsed < Duration::from_millis(700),
                "tls={tls}: {elapsed:?}"
            );
            assert!(
                sent.load(Ordering::SeqCst) >= 2,
                "peer did not trickle before timeout"
            );
        }
    }
    Ok(())
}

#[test]
fn tls_handshake_obeys_total_deadline() -> Checked<()> {
    let listener = TcpListener::bind("127.0.0.1:0")?;
    let endpoint = Endpoint::parse(
        &format!("https://localhost:{}", listener.local_addr()?.port()),
        Security::PinnedTls(TRUST_ANCHOR.to_vec()),
        Duration::from_millis(250),
    )?;
    let (release, held) = mpsc::sync_channel(1);
    let sent = Arc::new(AtomicUsize::new(0));
    let count = Arc::clone(&sent);
    let worker = thread::spawn(move || -> Checked<()> {
        let (mut socket, _) = listener.accept()?;
        // A recorded TLS handshake record header followed by a deliberately incomplete payload.
        socket.write_all(&[0x16, 0x03, 0x03, 0x00, 0x80])?;
        for _ in 0..40 {
            if held.recv_timeout(Duration::from_millis(40)).is_ok() {
                break;
            }
            match socket.write_all(&[0]) {
                Ok(()) => {
                    count.fetch_add(1, Ordering::SeqCst);
                }
                Err(error)
                    if matches!(
                        error.kind(),
                        std::io::ErrorKind::BrokenPipe | std::io::ErrorKind::ConnectionReset
                    ) =>
                {
                    break;
                }
                Err(error) => return Err(error.into()),
            }
        }
        Ok(())
    });
    let started = Instant::now();
    let result = endpoint.get("/handshake");
    let elapsed = started.elapsed();
    let _ = release.send(());
    join(worker)?;
    assert!(matches!(result, Err(IndexError::Source(_))), "{result:?}");
    assert!(
        elapsed >= Duration::from_millis(150) && elapsed < Duration::from_millis(700),
        "{elapsed:?}"
    );
    assert!(sent.load(Ordering::SeqCst) >= 2);
    Ok(())
}

#[test]
fn request_writes_obey_total_deadline() -> Checked<()> {
    for tls in [false, true] {
        let (release, held) = mpsc::sync_channel(1);
        let received = Arc::new(AtomicBool::new(false));
        let observed = Arc::clone(&received);
        let (endpoint, worker) = peer(tls, Duration::from_millis(350), move |stream| {
            request_head(stream)?;
            observed.store(true, Ordering::SeqCst);
            held.recv_timeout(Duration::from_secs(3))?;
            Ok(())
        })?;
        let params = json!(["x".repeat(16 * 1024 * 1024)]);
        let started = Instant::now();
        let result = endpoint.rpc("write_backpressure", &params);
        let elapsed = started.elapsed();
        release.send(())?;
        join(worker)?;
        assert!(
            received.load(Ordering::SeqCst),
            "peer never received request headers"
        );
        assert!(
            matches!(result, Err(IndexError::Source(_))),
            "tls={tls}: {result:?}"
        );
        assert!(
            elapsed >= Duration::from_millis(250) && elapsed < Duration::from_secs(1),
            "tls={tls}: {elapsed:?}"
        );
    }
    Ok(())
}

#[test]
fn malformed_frames_are_refused_before_peer_close() -> Checked<()> {
    let mut oversized_header = b"HTTP/1.1 200 OK\r\nX: ".to_vec();
    oversized_header.extend(vec![b'x'; 32 * 1024]);
    let mut oversized_chunk_line =
        b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n".to_vec();
    oversized_chunk_line.extend(vec![b'f'; 32 * 1024 + 1]);
    for response in [
        b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nTransfer-Encoding: chunked\r\n\r\n{}".to_vec(),
        b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nContent-Length: 2\r\n\r\n{}".to_vec(),
        b"HTTP/1.1 200 OK\r\nContent-Length: 67108865\r\n\r\n".to_vec(),
        b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n4000001\r\n".to_vec(),
        b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n1;ext=x\r\nx\r\n0\r\n\r\n".to_vec(),
        b"\r\n".to_vec(),
        oversized_header,
        oversized_chunk_line,
    ] {
        assert!(split_response(&response).is_err());
        let (release, held) = mpsc::sync_channel(1);
        let (endpoint, worker) = peer(false, Duration::from_secs(1), move |stream| {
            request_head(stream)?;
            stream.write_all(&response)?;
            stream.flush()?;
            held.recv_timeout(Duration::from_secs(3))?;
            Ok(())
        })?;
        let started = Instant::now();
        let result = endpoint.get("/invalid");
        let elapsed = started.elapsed();
        release.send(())?;
        join(worker)?;
        assert!(matches!(result, Err(IndexError::Source(_))), "{result:?}");
        assert!(
            elapsed < Duration::from_millis(750),
            "framing refusal waited {elapsed:?}"
        );
    }
    for response in [
        &b"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\n{}"[..],
        &b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n2\r\n{"[..],
        &b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}extra"[..],
    ] {
        assert!(
            split_response(response).is_err(),
            "truncated or surplus response accepted"
        );
    }
    Ok(())
}
