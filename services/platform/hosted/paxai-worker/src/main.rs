//! PAXAI worker service: serves the signed status probe and authenticates every signed job
//! request against the worker's delegate-signed metadata binding.
//!
//! Admission needs a current finalized authority view per request. Until the native
//! finalized-authority reader is wired in, authenticated job requests are refused with
//! `StaleAuthority` and nothing is queued or executed (fail closed).
use ed25519_dalek::SigningKey;
use layerx_paxai_worker::{
    auth::{
        decode_service, sign_status, verify_request, verify_signed_metadata, MetadataContext,
        Route, ServiceError, StatusChallenge, WorkerBinding, STATUS_PATH,
    },
    discovery::spki_sha256,
};
use layerx_programs_ai_market::{
    queries::parse_hex32,
    types::{
        ChainDomain, MarketId, Presence, PrincipalId, ProgramId, PublicKey32, RequestId, WorkerId,
    },
    MAX_ENVELOPE_BYTES,
};
use rustls::{
    pki_types::{pem::PemObject, CertificateDer, PrivateKeyDer},
    ServerConfig, ServerConnection, StreamOwned,
};
use serde::Deserialize;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::PathBuf;
use std::process::ExitCode;
use std::sync::{
    atomic::{AtomicUsize, Ordering},
    Arc,
};
use std::time::{Duration, Instant};

const MAX_CONNECTIONS: usize = 128;
static ACTIVE_CONNECTIONS: AtomicUsize = AtomicUsize::new(0);
const REQUEST_DEADLINE: Duration = Duration::from_secs(30);
const MAX_REQUEST_HEAD_BYTES: usize = 8_192;

struct ConnectionPermit;

impl ConnectionPermit {
    fn acquire() -> Option<Self> {
        ACTIVE_CONNECTIONS
            .fetch_update(Ordering::AcqRel, Ordering::Acquire, |active| {
                (active < MAX_CONNECTIONS).then_some(active + 1)
            })
            .ok()
            .map(|_| Self)
    }
}

impl Drop for ConnectionPermit {
    fn drop(&mut self) {
        ACTIVE_CONNECTIONS.fetch_sub(1, Ordering::AcqRel);
    }
}

struct DeadlineSocket {
    socket: TcpStream,
    deadline: Instant,
}

impl DeadlineSocket {
    fn remaining(&self) -> std::io::Result<Duration> {
        self.deadline
            .checked_duration_since(Instant::now())
            .filter(|remaining| !remaining.is_zero())
            .ok_or_else(|| std::io::Error::from(std::io::ErrorKind::TimedOut))
    }
}

impl Read for DeadlineSocket {
    fn read(&mut self, buf: &mut [u8]) -> std::io::Result<usize> {
        self.socket.set_read_timeout(Some(self.remaining()?))?;
        self.socket.read(buf)
    }
}

impl Write for DeadlineSocket {
    fn write(&mut self, buf: &[u8]) -> std::io::Result<usize> {
        self.socket.set_write_timeout(Some(self.remaining()?))?;
        self.socket.write(buf)
    }

    fn flush(&mut self) -> std::io::Result<()> {
        self.socket.set_write_timeout(Some(self.remaining()?))?;
        self.socket.flush()
    }
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct Config {
    listen: String,
    certificate_chain: PathBuf,
    private_key: PathBuf,
    delegate_seed: PathBuf,
    signed_metadata: PathBuf,
    chain: String,
    program: String,
    market: String,
    worker: String,
    owner: String,
}

/// One startup refusal; it never carries key material or file contents.
struct Startup(&'static str);

struct Service {
    binding: WorkerBinding,
    key: SigningKey,
}

struct Response {
    status: u16,
    body: Vec<u8>,
}

fn hex32(text: &str) -> Result<[u8; 32], Startup> {
    parse_hex32(text.trim()).map_err(|_| Startup("identifier is not 64 lowercase hex digits"))
}

fn load(path: &str) -> Result<(Service, Arc<ServerConfig>, String), Startup> {
    let text = std::fs::read_to_string(path).map_err(|_| Startup("configuration unreadable"))?;
    let config: Config =
        serde_json::from_str(&text).map_err(|_| Startup("configuration is not valid"))?;
    let seed = std::fs::read_to_string(&config.delegate_seed)
        .map_err(|_| Startup("delegate seed unreadable"))?;
    let key = SigningKey::from_bytes(&hex32(&seed)?);
    let context = MetadataContext {
        chain: ChainDomain::new(hex32(&config.chain)?).map_err(|_| Startup("zero chain"))?,
        program: ProgramId::new(hex32(&config.program)?).map_err(|_| Startup("zero program"))?,
        market: MarketId::new(hex32(&config.market)?).map_err(|_| Startup("zero market"))?,
        worker: WorkerId::new(hex32(&config.worker)?).map_err(|_| Startup("zero worker"))?,
        owner: PrincipalId::new(hex32(&config.owner)?).map_err(|_| Startup("zero owner"))?,
        delegate: PublicKey32(key.verifying_key().to_bytes()),
    };
    let signed = std::fs::read(&config.signed_metadata)
        .map_err(|_| Startup("signed metadata unreadable"))?;
    let signed = verify_signed_metadata(&signed, &context)
        .map_err(|_| Startup("signed metadata refused for this worker and delegate"))?;
    let chain = CertificateDer::pem_file_iter(&config.certificate_chain)
        .map_err(|_| Startup("certificate chain unreadable"))?
        .collect::<Result<Vec<_>, _>>()
        .map_err(|_| Startup("certificate chain is not valid PEM"))?;
    let leaf = chain.first().ok_or(Startup("certificate chain is empty"))?;
    let pin = spki_sha256(leaf).map_err(|_| Startup("leaf certificate unparsable"))?;
    if !signed
        .manifest
        .endpoints
        .iter()
        .any(|e| e.spki_sha256 == pin)
    {
        return Err(Startup(
            "leaf certificate is not pinned by the signed metadata",
        ));
    }
    let private = PrivateKeyDer::from_pem_file(&config.private_key)
        .map_err(|_| Startup("private key unreadable"))?;
    let tls =
        ServerConfig::builder_with_provider(Arc::new(rustls::crypto::ring::default_provider()))
            .with_protocol_versions(&[&rustls::version::TLS13])
            .map_err(|_| Startup("TLS 1.3 unavailable"))?
            .with_no_client_auth()
            .with_single_cert(chain, private)
            .map_err(|_| Startup("certificate and private key do not match"))?;
    let service = Service {
        binding: signed.binding(),
        key,
    };
    Ok((service, Arc::new(tls), config.listen))
}

fn refusal(error: ServiceError, request: Presence<RequestId>) -> Response {
    let status = match error {
        ServiceError::NotFound => 404,
        ServiceError::InputTooLarge => 413,
        ServiceError::StaleAuthority => 503,
        _ => 400,
    };
    Response {
        status,
        body: error.body(request).to_vec(),
    }
}

impl Service {
    fn dispatch(&self, method: &str, path: &str, body: &[u8]) -> Response {
        if method != "POST" {
            return refusal(ServiceError::NotFound, Presence::Absent);
        }
        if path == STATUS_PATH {
            return self.status(body);
        }
        match Route::from_path(path) {
            Some(route) => self.job(route, body),
            None => refusal(ServiceError::NotFound, Presence::Absent),
        }
    }

    fn status(&self, body: &[u8]) -> Response {
        let signed = StatusChallenge::decode(body)
            .and_then(|challenge| sign_status(&self.binding, &challenge, &self.key));
        match signed {
            Ok(body) => Response { status: 200, body },
            Err(error) => refusal(error, Presence::Absent),
        }
    }

    fn job(&self, route: Route, body: &[u8]) -> Response {
        let envelope = match decode_service(body) {
            Ok(envelope) => envelope,
            Err(error) => return refusal(error, Presence::Absent),
        };
        let request = Presence::Present(envelope.context.request);
        if let Err(error) = verify_request(&envelope, route, &self.binding) {
            return refusal(error, request);
        }
        refusal(ServiceError::StaleAuthority, request)
    }
}

/// Reads one `POST` request with a `Content-Length` body of at most one envelope.
fn read_request(stream: &mut impl Read) -> Result<(String, String, Vec<u8>), Response> {
    let malformed = || refusal(ServiceError::NonCanonical, Presence::Absent);
    let mut head = Vec::new();
    let mut chunk = [0u8; 1024];
    let head_end = loop {
        if head.len() >= MAX_REQUEST_HEAD_BYTES {
            return Err(malformed());
        }
        let limit = chunk.len().min(MAX_REQUEST_HEAD_BYTES - head.len());
        let count = stream.read(&mut chunk[..limit]).map_err(|_| malformed())?;
        if count == 0 {
            return Err(malformed());
        }
        head.extend_from_slice(&chunk[..count]);
        if let Some(end) = head.windows(4).position(|bytes| bytes == b"\r\n\r\n") {
            break end + 4;
        }
    };
    let text = std::str::from_utf8(&head[..head_end]).map_err(|_| malformed())?;
    let mut lines = text.split("\r\n");
    let mut start = lines.next().unwrap_or_default().split(' ');
    let (Some(method), Some(path), Some("HTTP/1.1")) = (start.next(), start.next(), start.next())
    else {
        return Err(malformed());
    };
    let mut length = 0usize;
    for line in lines {
        if let Some((name, value)) = line.split_once(':') {
            if name.eq_ignore_ascii_case("content-length") {
                length = value.trim().parse().map_err(|_| malformed())?;
            }
        }
    }
    if length > MAX_ENVELOPE_BYTES {
        return Err(refusal(ServiceError::InputTooLarge, Presence::Absent));
    }
    let mut body = vec![0; length];
    let buffered = &head[head_end..];
    let copied = buffered.len().min(length);
    body[..copied].copy_from_slice(&buffered[..copied]);
    stream
        .read_exact(&mut body[copied..])
        .map_err(|_| malformed())?;
    Ok((method.to_owned(), path.to_owned(), body))
}

fn reason(status: u16) -> &'static str {
    match status {
        200 => "OK",
        404 => "Not Found",
        413 => "Content Too Large",
        503 => "Service Unavailable",
        _ => "Bad Request",
    }
}

fn serve(
    service: &Service,
    tls: Arc<ServerConfig>,
    socket: TcpStream,
    deadline: Instant,
) -> std::io::Result<()> {
    let connection = ServerConnection::new(tls).map_err(std::io::Error::other)?;
    let mut stream = StreamOwned::new(connection, DeadlineSocket { socket, deadline });
    let response = match read_request(&mut stream) {
        Ok((method, path, body)) => service.dispatch(&method, &path, &body),
        Err(response) => response,
    };
    let head = format!(
        "HTTP/1.1 {} {}\r\nContent-Type: application/octet-stream\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        response.status,
        reason(response.status),
        response.body.len()
    );
    stream.write_all(head.as_bytes())?;
    stream.write_all(&response.body)?;
    stream.flush()?;
    stream.conn.send_close_notify();
    stream.conn.complete_io(&mut stream.sock)?;
    Ok(())
}

fn main() -> ExitCode {
    let mut args = std::env::args().skip(1);
    let (Some("--config"), Some(path), None) = (args.next().as_deref(), args.next(), args.next())
    else {
        eprintln!("usage: layerx-paxai-worker --config <file>");
        return ExitCode::from(2);
    };
    let (service, tls, listen) = match load(&path) {
        Ok(loaded) => loaded,
        Err(Startup(reason)) => {
            eprintln!("refusing to start: {reason}");
            return ExitCode::FAILURE;
        }
    };
    let listener = match TcpListener::bind(&listen) {
        Ok(listener) => listener,
        Err(error) => {
            eprintln!("refusing to start: cannot listen: {error}");
            return ExitCode::FAILURE;
        }
    };
    match listener.local_addr() {
        Ok(addr) => println!("listening {addr}"),
        Err(error) => {
            eprintln!("refusing to start: {error}");
            return ExitCode::FAILURE;
        }
    }
    let service = Arc::new(service);
    for socket in listener.incoming() {
        match socket {
            Ok(socket) => {
                let deadline = Instant::now() + REQUEST_DEADLINE;
                let Some(permit) = ConnectionPermit::acquire() else {
                    drop(socket);
                    continue;
                };
                let service = Arc::clone(&service);
                let tls = Arc::clone(&tls);
                std::thread::spawn(move || {
                    let _permit = permit;
                    if let Err(error) = serve(&service, tls, socket, deadline) {
                        eprintln!("connection closed: {}", error.kind());
                    }
                });
            }
            Err(error) => eprintln!("accept failed: {}", error.kind()),
        }
    }
    ExitCode::SUCCESS
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn request_limits_permit_refuses_beyond_cap() {
        let mut permits = Vec::new();
        for _ in 0..MAX_CONNECTIONS {
            permits.push(ConnectionPermit::acquire().expect("permit below connection cap"));
        }
        assert!(ConnectionPermit::acquire().is_none());
        drop(permits.pop().expect("held permit"));
        let replacement = ConnectionPermit::acquire().expect("released connection slot");
        assert!(ConnectionPermit::acquire().is_none());
        drop(replacement);
        drop(permits);
        assert_eq!(ACTIVE_CONNECTIONS.load(Ordering::Acquire), 0);
    }

    #[test]
    fn request_limits_slow_head_is_refused_at_deadline() {
        let listener = TcpListener::bind("127.0.0.1:0").expect("bind loopback listener");
        let mut client = TcpStream::connect(listener.local_addr().expect("listener address"))
            .expect("connect loopback client");
        let (socket, _) = listener.accept().expect("accept loopback client");
        client.write_all(b"P").expect("write partial request head");
        let started = Instant::now();
        let mut stream = DeadlineSocket {
            socket,
            deadline: started + Duration::from_millis(200),
        };
        let response = match read_request(&mut stream) {
            Err(response) => response,
            Ok(_) => panic!("incomplete request head accepted"),
        };
        assert!(started.elapsed() < Duration::from_secs(2));
        let expected = refusal(ServiceError::NonCanonical, Presence::Absent);
        assert_eq!(response.status, expected.status);
        assert_eq!(response.body, expected.body);
    }
}
