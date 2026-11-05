//! Loopback peers for the maintained node-facing Rustls transport and chain codec.
//! No relay process, environment configuration, generated certificates or RPC double.

use layerx_types::chain::BlockReference;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer, ServerName};
use rustls::{
    ClientConfig, ClientConnection, RootCertStore, ServerConfig, ServerConnection, StreamOwned,
};
use std::io::{Read, Write};
use std::net::{Shutdown, SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::{mpsc, Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::{Duration, Instant};

const CA: &[u8] = include_bytes!("../fixtures/repair_22_33_1324/ca.der");
const CERT: &[u8] = include_bytes!("../fixtures/repair_22_33_1324/server.der");
const KEY: &[u8] = include_bytes!("../fixtures/repair_22_33_1324/server-key.der");
const REQUEST: &[u8] =
    b"GET /chain-reference HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n";
const TIMEOUT: Duration = Duration::from_secs(5);
const WORKERS: usize = 8;

pub fn client_config(ca: Option<&[u8]>) -> Result<Arc<ClientConfig>, rustls::Error> {
    let mut roots = RootCertStore::empty();
    if let Some(ca) = ca {
        roots.add(CertificateDer::from(ca.to_vec()))?;
    }
    Ok(Arc::new(
        ClientConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_safe_default_protocol_versions()
            .expect("TLS versions")
            .with_root_certificates(roots)
            .with_no_client_auth(),
    ))
}

pub fn connect(
    address: SocketAddr,
    name: &str,
    config: Arc<ClientConfig>,
) -> StreamOwned<ClientConnection, TcpStream> {
    let connection = ClientConnection::new(
        config,
        ServerName::try_from(name.to_owned()).expect("server name"),
    )
    .expect("TLS client");
    let socket = TcpStream::connect_timeout(&address, TIMEOUT).expect("connect reported address");
    socket.set_read_timeout(Some(TIMEOUT)).expect("read bound");
    socket
        .set_write_timeout(Some(TIMEOUT))
        .expect("write bound");
    StreamOwned::new(connection, socket)
}

pub fn fetch(address: SocketAddr, name: &str, trusted: bool) -> Result<BlockReference, String> {
    fetch_with_config(
        address,
        name,
        client_config(if trusted { Some(CA) } else { None }).expect("TLS roots"),
    )
}

pub fn fetch_with_config(
    address: SocketAddr,
    name: &str,
    config: Arc<ClientConfig>,
) -> Result<BlockReference, String> {
    let mut stream = connect(address, name, config);
    stream
        .write_all(REQUEST)
        .map_err(|error| error.to_string())?;
    stream.flush().map_err(|error| error.to_string())?;
    let mut response = Vec::new();
    // Rustls refuses EOF without close_notify. Read through the TLS close, not
    // merely Content-Length, so truncated TLS shutdown cannot masquerade as success.
    stream
        .read_to_end(&mut response)
        .map_err(|error| error.to_string())?;
    let end = response
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .ok_or("missing headers")?
        + 4;
    let headers = std::str::from_utf8(&response[..end]).map_err(|error| error.to_string())?;
    assert!(headers.starts_with("HTTP/1.1 200 OK\r\n"));
    assert!(headers.contains("Content-Type: application/octet-stream\r\n"));
    assert!(headers.contains("Connection: close\r\n"));
    let size: usize = headers
        .lines()
        .find_map(|line| line.strip_prefix("Content-Length: "))
        .ok_or("missing length")?
        .parse()
        .map_err(|error: std::num::ParseIntError| error.to_string())?;
    assert_eq!(response.len() - end, size, "exact response body");
    let result = BlockReference::decode(&response[end..])
        .map_err(|error| format!("chain codec: {error:?}"))?;
    stream.conn.send_close_notify();
    stream.flush().map_err(|error| error.to_string())?;
    stream
        .sock
        .shutdown(Shutdown::Both)
        .map_err(|error| error.to_string())?;
    Ok(result)
}

pub struct EvidencePeer {
    pub address: SocketAddr,
    pub report: String,
    pub active: Arc<AtomicUsize>,
    pub maximum: Arc<AtomicUsize>,
    stop: Arc<AtomicBool>,
    done: mpsc::Receiver<Vec<Result<(), String>>>,
    worker: Option<JoinHandle<()>>,
    tls: Arc<ServerConfig>,
}

impl EvidencePeer {
    pub fn start(reference: BlockReference) -> Self {
        Self::with_identity(reference, CERT, KEY)
    }

    pub fn ca_as_server(reference: BlockReference) -> Self {
        Self::with_identity(
            reference,
            CA,
            include_bytes!("../fixtures/repair_22_33_1324/ca-key.der"),
        )
    }

    fn with_identity(reference: BlockReference, cert: &[u8], key: &[u8]) -> Self {
        let tls = Arc::new(
            ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
                .with_safe_default_protocol_versions()
                .expect("TLS versions")
                .with_no_client_auth()
                .with_single_cert(
                    vec![CertificateDer::from(cert.to_vec())],
                    PrivateKeyDer::from(PrivatePkcs8KeyDer::from(key.to_vec())),
                )
                .expect("committed server identity"),
        );
        let listener = TcpListener::bind(("127.0.0.1", 0)).expect("loopback listener");
        let address = listener.local_addr().expect("bound address");
        listener.set_nonblocking(true).expect("bounded accept loop");
        let report = format!("{address}\n");
        let active = Arc::new(AtomicUsize::new(0));
        let maximum = Arc::new(AtomicUsize::new(0));
        let stop = Arc::new(AtomicBool::new(false));
        let (finished, done) = mpsc::channel();
        let worker = {
            let tls = Arc::clone(&tls);
            let stop = Arc::clone(&stop);
            let active = Arc::clone(&active);
            let maximum = Arc::clone(&maximum);
            thread::spawn(move || {
                let (sender, receiver) = mpsc::sync_channel::<TcpStream>(128);
                let receiver = Arc::new(Mutex::new(receiver));
                let workers: Vec<_> = (0..WORKERS)
                    .map(|_| {
                        let receiver = Arc::clone(&receiver);
                        let tls = Arc::clone(&tls);
                        let active = Arc::clone(&active);
                        let maximum = Arc::clone(&maximum);
                        thread::spawn(move || {
                            let mut results = Vec::new();
                            loop {
                                let socket = match receiver.lock().expect("peer queue").recv() {
                                    Ok(socket) => socket,
                                    Err(_) => return results,
                                };
                                let count = active.fetch_add(1, Ordering::SeqCst) + 1;
                                maximum.fetch_max(count, Ordering::SeqCst);
                                results.push(exchange(socket, &tls, reference));
                                active.fetch_sub(1, Ordering::SeqCst);
                            }
                        })
                    })
                    .collect();
                while !stop.load(Ordering::SeqCst) {
                    match listener.accept() {
                        Ok((socket, _)) => sender.send(socket).expect("peer workers remain live"),
                        Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                            thread::sleep(Duration::from_millis(1))
                        }
                        Err(error) => panic!("accept peer: {error}"),
                    }
                }
                drop(sender);
                let results = workers
                    .into_iter()
                    .flat_map(|worker| worker.join().expect("TLS worker"))
                    .collect();
                drop(listener);
                finished.send(results).expect("peer result receiver");
            })
        };
        Self {
            address,
            report,
            active,
            maximum,
            stop,
            done,
            worker: Some(worker),
            tls,
        }
    }

    pub fn finish(mut self) -> Vec<Result<(), String>> {
        self.stop.store(true, Ordering::SeqCst);
        let results = self
            .done
            .recv_timeout(TIMEOUT + Duration::from_secs(2))
            .expect("TLS workers must terminate within their socket bound");
        self.worker
            .take()
            .expect("peer owner")
            .join()
            .expect("listener owner");
        assert_eq!(
            self.active.load(Ordering::SeqCst),
            0,
            "all connections released"
        );
        assert_eq!(
            Arc::strong_count(&self.tls),
            1,
            "all Rustls server configurations released"
        );
        TcpListener::bind(self.address).expect("listener port must be reusable after shutdown");
        results
    }
}

impl Drop for EvidencePeer {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            worker.join().expect("stop loopback peer");
        }
    }
}

fn exchange(
    socket: TcpStream,
    config: &Arc<ServerConfig>,
    reference: BlockReference,
) -> Result<(), String> {
    socket
        .set_read_timeout(Some(TIMEOUT))
        .map_err(|error| error.to_string())?;
    socket
        .set_write_timeout(Some(TIMEOUT))
        .map_err(|error| error.to_string())?;
    let connection =
        ServerConnection::new(Arc::clone(config)).map_err(|error| error.to_string())?;
    let mut stream = StreamOwned::new(connection, socket);
    let mut request = vec![0; REQUEST.len()];
    stream
        .read_exact(&mut request)
        .map_err(|error| error.to_string())?;
    assert_eq!(request, REQUEST);
    // The real current producer emits the payload; the peer only frames it.
    let body = reference.encode();
    write!(stream, "HTTP/1.1 200 OK\r\nContent-Type: application/octet-stream\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).map_err(|error| error.to_string())?;
    stream.write_all(&body).map_err(|error| error.to_string())?;
    stream.conn.send_close_notify();
    stream.flush().map_err(|error| error.to_string())?;
    let start = Instant::now();
    stream
        .sock
        .set_read_timeout(Some(Duration::from_secs(1)))
        .map_err(|error| error.to_string())?;
    let mut trailing = [0; 4096];
    assert_eq!(
        stream
            .read(&mut trailing)
            .map_err(|error| error.to_string())?,
        0,
        "TLS peer sends no trailing application bytes"
    );
    assert!(
        start.elapsed() < Duration::from_secs(1),
        "TLS shutdown must finish within one second"
    );
    Ok(())
}

pub const TRUST: &str = include_str!("../fixtures/repair_22_33_1324/chain-trust-v1.json");
pub const ARCHIVE: &str =
    include_str!("../../../../../../contracts/config/checkpoint-settlement.json");

pub fn reference_from_recording() -> BlockReference {
    let record: serde_json::Value =
        serde_json::from_str(include_str!("../fixtures/repair_22_33_1324/commit.json"))
            .expect("recorded node interface");
    let signed = &record["result"]["signed_header"];
    let header = &signed["header"];
    let trust =
        layerx_types::chain::ValidatorSetTrust::parse(TRUST).expect("independent chain pin");
    let height = header["height"]
        .as_str()
        .expect("header height")
        .parse::<u64>()
        .expect("height number");
    assert_eq!(header["chain_id"], trust.comet_chain_id());
    assert_eq!(height, trust.height());
    assert_eq!(signed["commit"]["height"], header["height"]);
    fn hash(value: &serde_json::Value) -> [u8; 32] {
        let text = value.as_str().expect("recorded hash");
        assert_eq!(text.len(), 64);
        let mut hash = [0; 32];
        for (index, byte) in hash.iter_mut().enumerate() {
            *byte = u8::from_str_radix(&text[index * 2..index * 2 + 2], 16).expect("hash byte");
        }
        hash
    }
    BlockReference::new(
        height,
        height,
        hash(&signed["commit"]["block_id"]["hash"]),
        hash(&header["validators_hash"]),
        hash(&header["app_hash"]),
    )
    .expect("production chain-reference producer")
}

pub fn qualify_listener() {
    let reference = reference_from_recording();
    let peer = EvidencePeer::start(reference);
    assert_eq!(peer.report.lines().count(), 1);
    assert!(peer.report.ends_with('\n'));
    let reported: SocketAddr = peer
        .report
        .trim_end()
        .parse()
        .expect("reported socket address");
    assert_eq!(reported, peer.address);
    assert!(reported.ip().is_loopback());
    assert_ne!(reported.port(), 0);
    assert!(
        TcpListener::bind(reported).is_err(),
        "the reported listener retains its port"
    );
    assert_eq!(
        fetch(reported, "localhost", true).expect("trusted TLS and exact evidence"),
        reference
    );
    let results = peer.finish();
    assert_eq!(results.len(), 1);
    assert!(results.into_iter().all(|result| result.is_ok()));
}

pub fn qualify_tls_identity() {
    let peer = EvidencePeer::start(reference_from_recording());
    for host in ["wrong-host.invalid", "127.0.0.1"] {
        assert!(
            fetch(peer.address, host, true).is_err(),
            "wrong server identity must be rejected"
        );
    }
    assert!(
        fetch(peer.address, "localhost", false).is_err(),
        "empty root store must not trust the server"
    );
    let unrelated = client_config(Some(include_bytes!(
        "../fixtures/repair_22_33_1324/unrelated-ca.der"
    )))
    .expect("unrelated CA");
    assert!(
        fetch_with_config(peer.address, "localhost", unrelated).is_err(),
        "unrelated CA must not trust the server"
    );
    assert!(
        client_config(Some(&[])).is_err(),
        "empty DER root must be refused"
    );
    assert!(
        std::fs::read(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/tests/fixtures/repair_22_33_1324/missing-ca.der"
        ))
        .is_err(),
        "missing CA is an explicit configuration failure"
    );
    fetch(peer.address, "localhost", true).expect("trusted connection after refused handshakes");
    let results = peer.finish();
    assert_eq!(results.len(), 5);
    assert_eq!(results.iter().filter(|result| result.is_err()).count(), 4);
    assert_eq!(results.iter().filter(|result| result.is_ok()).count(), 1);
    let invalid_identity = EvidencePeer::ca_as_server(reference_from_recording());
    assert!(
        fetch(invalid_identity.address, "localhost", true).is_err(),
        "a trusted CA must not be used as an end-entity server"
    );
    let results = invalid_identity.finish();
    assert_eq!(results.len(), 1);
    assert!(results[0].is_err());
}
