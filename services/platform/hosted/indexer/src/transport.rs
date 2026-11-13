//! Bounded blocking HTTP/1.1 client for the indexer's three upstreams: the
//! relay/archive sync API, Paxeer EVM JSON-RPC and `CometBFT` JSON-RPC.
//!
//! The framing rules require exactly one of `Content-Length` or chunked
//! encoding, bounded headers and bodies, and completion at the framed body
//! boundary. HTTPS is authenticated by an explicitly configured trust anchor;
//! plaintext is admitted for loopback
//! endpoints, or for in-cluster endpoints only when the operator opts in.

use std::io::{self, BufRead, BufReader, Read, Write};
use std::net::{IpAddr, SocketAddr, TcpStream, ToSocketAddrs as _};
use std::sync::atomic::{AtomicU64, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant};

use rustls::pki_types::{CertificateDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use serde::Serialize;
use serde_json::Value;

use crate::IndexError;

const MAXIMUM_HEADER_BYTES: usize = 32 * 1024;
const MAXIMUM_BODY_BYTES: usize = 64 * 1024 * 1024;
const MAXIMUM_CHUNKS: usize = 1 << 20;
const MAXIMUM_WRITE_BYTES: usize = 16 * 1024;

#[derive(Serialize)]
struct RpcRequest<'a> {
    jsonrpc: &'static str,
    id: u64,
    method: &'a str,
    params: &'a Value,
}

/// How one endpoint is authenticated.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Security {
    /// HTTPS authenticated by exactly this DER trust anchor.
    PinnedTls(Vec<u8>),
    /// Plaintext HTTP; admitted for loopback hosts, and for any host only
    /// when `allow_remote` is set by the operator.
    Plaintext { allow_remote: bool },
}

/// One upstream origin plus base path.
#[derive(Clone, Debug)]
pub struct Endpoint {
    https: bool,
    host: String,
    port: u16,
    base: String,
    security: Security,
    timeout: Duration,
    tls: Option<Arc<ClientConfig>>,
}

static REQUEST_ID: AtomicU64 = AtomicU64::new(1);

impl Endpoint {
    /// Parses `http(s)://host[:port][/base]` and binds its security policy.
    ///
    /// # Errors
    /// Refuses credentials, queries, fragments, a scheme that does not match
    /// the security policy, and remote plaintext without opt-in.
    pub fn parse(url: &str, security: Security, timeout: Duration) -> Result<Self, IndexError> {
        let (https, rest, default_port) = if let Some(rest) = url.strip_prefix("https://") {
            (true, rest, 443)
        } else if let Some(rest) = url.strip_prefix("http://") {
            (false, rest, 80)
        } else {
            return Err(IndexError::Config(format!("{url} is not http(s)")));
        };
        if rest.is_empty()
            || rest.contains(['#', '@', '?'])
            || rest
                .bytes()
                .any(|byte| byte.is_ascii_whitespace() || byte.is_ascii_control())
        {
            return Err(IndexError::Config(format!("{url} is not a bare origin")));
        }
        let (authority, base) = match rest.find('/') {
            Some(index) => rest.split_at(index),
            None => (rest, ""),
        };
        let (host, port) = match authority.rsplit_once(':') {
            Some((host, port)) if !host.contains(':') => (
                host,
                port.parse::<u16>()
                    .ok()
                    .filter(|port| *port != 0)
                    .ok_or_else(|| IndexError::Config(format!("{url} has an invalid port")))?,
            ),
            Some(_) => return Err(IndexError::Config(format!("{url} host is ambiguous"))),
            None => (authority, default_port),
        };
        if host.is_empty() {
            return Err(IndexError::Config(format!("{url} has no host")));
        }
        let tls = match (&security, https) {
            (Security::PinnedTls(anchor), true) => {
                let _ = rustls::crypto::ring::default_provider().install_default();
                let mut roots = RootCertStore::empty();
                roots
                    .add(CertificateDer::from(anchor.clone()))
                    .map_err(|error| IndexError::Config(format!("trust anchor: {error}")))?;
                Some(Arc::new(
                    ClientConfig::builder()
                        .with_root_certificates(roots)
                        .with_no_client_auth(),
                ))
            }
            (Security::Plaintext { allow_remote }, false) => {
                if !allow_remote && !loopback(host) {
                    return Err(IndexError::Config(format!(
                        "{url} is remote plaintext; configure a trust anchor or opt in"
                    )));
                }
                None
            }
            _ => {
                return Err(IndexError::Config(format!(
                    "{url} scheme does not match its security policy"
                )))
            }
        };
        Ok(Self {
            https,
            host: host.to_ascii_lowercase(),
            port,
            base: base.trim_end_matches('/').to_owned(),
            security,
            timeout,
            tls,
        })
    }

    /// The configured security policy.
    #[must_use]
    pub const fn security(&self) -> &Security {
        &self.security
    }

    /// GETs `path` (relative to the base path) and returns status and body.
    ///
    /// # Errors
    /// Returns [`IndexError::Source`] on transport or framing failure.
    pub fn get(&self, path: &str) -> Result<(u16, Vec<u8>), IndexError> {
        self.exchange("GET", path, &[])
    }

    /// GETs `path` and parses a 200 JSON body.
    ///
    /// # Errors
    /// Refuses a non-200 status or a non-JSON body.
    pub fn get_json(&self, path: &str) -> Result<Value, IndexError> {
        let (status, body) = self.get(path)?;
        if status != 200 {
            return Err(IndexError::Source(format!("GET {path} answered {status}")));
        }
        serde_json::from_slice(&body)
            .map_err(|error| IndexError::Decode(format!("GET {path}: {error}")))
    }

    /// Issues one JSON-RPC 2.0 call and returns its `result`.
    ///
    /// # Errors
    /// Refuses transport failures, non-200 answers, an answer bound to a
    /// different request, and returns the upstream's own error.
    pub fn rpc(&self, method: &str, params: &Value) -> Result<Value, IndexError> {
        self.rpc_answer(method, params)?
            .map_err(|error| IndexError::Source(format!("{method} refused: {error}")))
    }

    /// Issues one JSON-RPC 2.0 call and returns either its `result` or the
    /// upstream's own `error` object, so a caller can recognise a specific
    /// refusal.
    ///
    /// # Errors
    /// Refuses transport failures, non-200 answers, an answer bound to a
    /// different request, and an answer with neither result nor error.
    pub fn rpc_answer(
        &self,
        method: &str,
        params: &Value,
    ) -> Result<Result<Value, Value>, IndexError> {
        let id = REQUEST_ID.fetch_add(1, Ordering::Relaxed);
        let body = serde_json::to_vec(&RpcRequest {
            jsonrpc: "2.0",
            id,
            method,
            params,
        })
        .map_err(|error| IndexError::Source(format!("{method} request: {error}")))?;
        let (status, answer) = self.exchange("POST", "", &body)?;
        if status != 200 {
            return Err(IndexError::Source(format!(
                "{method} answered HTTP {status}"
            )));
        }
        let value: Value = serde_json::from_slice(&answer)
            .map_err(|error| IndexError::Decode(format!("{method}: {error}")))?;
        if value.get("id").and_then(Value::as_u64) != Some(id) {
            return Err(IndexError::Source(format!(
                "{method} answer is for another request"
            )));
        }
        if let Some(error) = value.get("error").filter(|error| !error.is_null()) {
            return Ok(Err(error.clone()));
        }
        value
            .get("result")
            .cloned()
            .map(Ok)
            .ok_or_else(|| IndexError::Decode(format!("{method} answer has no result")))
    }

    fn exchange(
        &self,
        method: &str,
        path: &str,
        body: &[u8],
    ) -> Result<(u16, Vec<u8>), IndexError> {
        if path
            .bytes()
            .any(|byte| byte.is_ascii_whitespace() || byte.is_ascii_control())
        {
            return Err(IndexError::Source("request path is invalid".to_owned()));
        }
        let target = if self.base.is_empty() && path.is_empty() {
            "/".to_owned()
        } else {
            format!("{}{path}", self.base)
        };
        let loopback_only = matches!(
            self.security,
            Security::Plaintext {
                allow_remote: false
            }
        );
        let deadline = Deadline::new(self.timeout).map_err(source_io)?;
        let tcp = DeadlineTcp {
            tcp: connect(&self.host, self.port, deadline, loopback_only)?,
            deadline,
        };
        let host = if (self.https && self.port == 443) || (!self.https && self.port == 80) {
            self.host.clone()
        } else {
            format!("{}:{}", self.host, self.port)
        };
        let head = format!(
            "{method} {target} HTTP/1.1\r\nHost: {host}\r\nAccept: application/json\r\nContent-Type: application/json\r\nUser-Agent: layerx-indexer/1\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
            body.len()
        );
        if let Some(tls) = &self.tls {
            let name = ServerName::try_from(self.host.clone())
                .map_err(|_| IndexError::Config("TLS server name is invalid".to_owned()))?;
            let connection = ClientConnection::new(Arc::clone(tls), name)
                .map_err(|error| IndexError::Source(error.to_string()))?;
            let mut stream = StreamOwned::new(connection, tcp);
            round_trip(&mut stream, head.as_bytes(), body, deadline)
        } else {
            let mut stream = tcp;
            round_trip(&mut stream, head.as_bytes(), body, deadline)
        }
    }
}

fn loopback(host: &str) -> bool {
    host.eq_ignore_ascii_case("localhost")
        || host
            .trim_start_matches('[')
            .trim_end_matches(']')
            .parse::<std::net::IpAddr>()
            .is_ok_and(|address| address.is_loopback())
}

#[derive(Clone, Copy)]
struct Deadline(Instant);

impl Deadline {
    fn new(timeout: Duration) -> io::Result<Self> {
        Instant::now()
            .checked_add(timeout)
            .map(Self)
            .ok_or_else(|| {
                io::Error::new(io::ErrorKind::InvalidInput, "request timeout is too large")
            })
    }

    fn remaining(self) -> io::Result<Duration> {
        self.0
            .checked_duration_since(Instant::now())
            .filter(|remaining| !remaining.is_zero())
            .ok_or_else(deadline_error)
    }
}

fn deadline_error() -> io::Error {
    io::Error::new(io::ErrorKind::TimedOut, "request deadline exceeded")
}

#[allow(
    clippy::needless_pass_by_value,
    reason = "Own the I/O error to match Result::map_err callbacks without changing the error adapter signature."
)]
fn source_io(error: io::Error) -> IndexError {
    IndexError::Source(error.to_string())
}

struct DeadlineTcp {
    tcp: TcpStream,
    deadline: Deadline,
}

impl Read for DeadlineTcp {
    fn read(&mut self, output: &mut [u8]) -> io::Result<usize> {
        self.tcp
            .set_read_timeout(Some(self.deadline.remaining()?))?;
        let result = self.tcp.read(output);
        self.deadline.remaining()?;
        result
    }
}

impl Write for DeadlineTcp {
    fn write(&mut self, input: &[u8]) -> io::Result<usize> {
        self.tcp
            .set_write_timeout(Some(self.deadline.remaining()?))?;
        let result = self
            .tcp
            .write(&input[..input.len().min(MAXIMUM_WRITE_BYTES)]);
        self.deadline.remaining()?;
        result
    }

    fn flush(&mut self) -> io::Result<()> {
        self.tcp
            .set_write_timeout(Some(self.deadline.remaining()?))?;
        let result = self.tcp.flush();
        self.deadline.remaining()?;
        result
    }
}

fn connect(
    host: &str,
    port: u16,
    deadline: Deadline,
    loopback_only: bool,
) -> Result<TcpStream, IndexError> {
    let addresses = if let Ok(ip) = host.parse::<IpAddr>() {
        vec![SocketAddr::new(ip, port)]
    } else {
        let host = host.to_owned();
        let (send, receive) = std::sync::mpsc::sync_channel(1);
        std::thread::Builder::new()
            .name("indexer-resolve".to_owned())
            .spawn(move || {
                let result = (host.as_str(), port)
                    .to_socket_addrs()
                    .map(std::iter::Iterator::collect::<Vec<_>>);
                let _ = send.send(result);
            })
            .map_err(source_io)?;
        receive
            .recv_timeout(deadline.remaining().map_err(source_io)?)
            .map_err(|error| match error {
                std::sync::mpsc::RecvTimeoutError::Timeout => source_io(deadline_error()),
                std::sync::mpsc::RecvTimeoutError::Disconnected => {
                    IndexError::Source("resolver stopped".to_owned())
                }
            })?
            .map_err(source_io)?
    };
    deadline.remaining().map_err(source_io)?;
    if loopback_only && addresses.iter().any(|address| !address.ip().is_loopback()) {
        return Err(IndexError::Source(format!(
            "{host} does not resolve to loopback"
        )));
    }
    let mut last = None;
    for address in addresses {
        match TcpStream::connect_timeout(&address, deadline.remaining().map_err(source_io)?) {
            Ok(stream) => {
                deadline.remaining().map_err(source_io)?;
                return Ok(stream);
            }
            Err(error) => last = Some(error),
        }
    }
    deadline.remaining().map_err(source_io)?;
    Err(IndexError::Source(last.map_or_else(
        || format!("{host} resolved to no address"),
        |error| format!("{host}: {error}"),
    )))
}

fn round_trip<S: Read + Write>(
    stream: &mut S,
    head: &[u8],
    body: &[u8],
    deadline: Deadline,
) -> Result<(u16, Vec<u8>), IndexError> {
    for bytes in [head, body] {
        for chunk in bytes.chunks(MAXIMUM_WRITE_BYTES) {
            let mut remaining = chunk;
            while !remaining.is_empty() {
                deadline.remaining().map_err(source_io)?;
                let written = match stream.write(remaining) {
                    Err(error) if error.kind() == io::ErrorKind::Interrupted => continue,
                    result => result.map_err(source_io)?,
                };
                deadline.remaining().map_err(source_io)?;
                if written == 0 {
                    return Err(source_io(io::Error::new(
                        io::ErrorKind::WriteZero,
                        "request write made no progress",
                    )));
                }
                remaining = &remaining[written..];
            }
        }
    }
    deadline.remaining().map_err(source_io)?;
    stream.flush().map_err(source_io)?;
    deadline.remaining().map_err(source_io)?;
    let mut reader = BufReader::new(stream);
    let response = read_response(&mut reader, Some(deadline))?;
    if !reader.buffer().is_empty() {
        return Err(IndexError::Source("ambiguous HTTP framing".to_owned()));
    }
    deadline.remaining().map_err(source_io)?;
    Ok(response)
}

fn read_line(
    reader: &mut impl BufRead,
    maximum: usize,
    deadline: Option<Deadline>,
) -> Result<Vec<u8>, IndexError> {
    let mut line = Vec::new();
    loop {
        if let Some(deadline) = deadline {
            deadline.remaining().map_err(source_io)?;
        }
        let available = reader.fill_buf().map_err(source_io)?;
        if available.is_empty() {
            return Err(IndexError::Source("truncated HTTP response".to_owned()));
        }
        let take = available
            .iter()
            .position(|byte| *byte == b'\n')
            .map_or(available.len(), |index| index + 1);
        if take > maximum.saturating_sub(line.len()) {
            return Err(IndexError::Source("HTTP line exceeds its bound".to_owned()));
        }
        line.extend_from_slice(&available[..take]);
        reader.consume(take);
        if line.ends_with(b"\n") {
            if !line.ends_with(b"\r\n") {
                return Err(IndexError::Source("malformed HTTP response".to_owned()));
            }
            return Ok(line);
        }
    }
}

fn read_response(
    reader: &mut impl BufRead,
    deadline: Option<Deadline>,
) -> Result<(u16, Vec<u8>), IndexError> {
    let mut headers = Vec::new();
    loop {
        let line = read_line(
            reader,
            (MAXIMUM_HEADER_BYTES + 4).saturating_sub(headers.len()),
            deadline,
        )?;
        let end = line == b"\r\n";
        headers.extend_from_slice(&line);
        if end {
            if headers.len() < 4 {
                return Err(IndexError::Source("malformed HTTP response".to_owned()));
            }
            break;
        }
    }
    let (status, framing) = response_head(&headers[..headers.len() - 4])?;
    let body = match framing {
        Framing::Length(length) => {
            if headers.len() + length > MAXIMUM_BODY_BYTES + MAXIMUM_HEADER_BYTES {
                return Err(IndexError::Source("response exceeds its bound".to_owned()));
            }
            let mut body = vec![0; length];
            reader.read_exact(&mut body).map_err(source_io)?;
            body
        }
        Framing::Chunked => {
            let mut output = Vec::new();
            let mut wire_bytes = headers.len();
            for _ in 0..MAXIMUM_CHUNKS {
                let line = read_line(reader, MAXIMUM_HEADER_BYTES, deadline)?;
                wire_bytes += line.len();
                if wire_bytes > MAXIMUM_BODY_BYTES + MAXIMUM_HEADER_BYTES {
                    return Err(IndexError::Source("response exceeds its bound".to_owned()));
                }
                let size = chunk_size(&line[..line.len() - 2])?;
                if size == 0 {
                    let end = read_line(reader, 2, deadline)?;
                    if end != b"\r\n" || wire_bytes + 2 > MAXIMUM_BODY_BYTES + MAXIMUM_HEADER_BYTES
                    {
                        return Err(IndexError::Source("malformed chunked body".to_owned()));
                    }
                    return Ok((status, output));
                }
                if size > MAXIMUM_BODY_BYTES.saturating_sub(output.len())
                    || size.saturating_add(2)
                        > (MAXIMUM_BODY_BYTES + MAXIMUM_HEADER_BYTES).saturating_sub(wire_bytes)
                {
                    return Err(IndexError::Source(
                        "chunked body exceeds its bound".to_owned(),
                    ));
                }
                wire_bytes += size + 2;
                let start = output.len();
                output.resize(start + size, 0);
                reader.read_exact(&mut output[start..]).map_err(source_io)?;
                let mut end = [0; 2];
                reader.read_exact(&mut end).map_err(source_io)?;
                if end != *b"\r\n" {
                    return Err(IndexError::Source("malformed chunked body".to_owned()));
                }
            }
            return Err(IndexError::Source("too many chunks".to_owned()));
        }
    };
    Ok((status, body))
}

enum Framing {
    Length(usize),
    Chunked,
}

fn response_head(headers: &[u8]) -> Result<(u16, Framing), IndexError> {
    let malformed = || IndexError::Source("malformed HTTP response".to_owned());
    let ambiguous = || IndexError::Source("ambiguous HTTP framing".to_owned());
    if headers.len() > MAXIMUM_HEADER_BYTES {
        return Err(IndexError::Source(
            "HTTP headers exceed their bound".to_owned(),
        ));
    }
    let headers = std::str::from_utf8(headers).map_err(|_| malformed())?;
    let mut lines = headers.split("\r\n");
    let mut status_parts = lines.next().ok_or_else(malformed)?.split_ascii_whitespace();
    if status_parts.next() != Some("HTTP/1.1") {
        return Err(malformed());
    }
    let status: u16 = status_parts
        .next()
        .and_then(|code| code.parse().ok())
        .ok_or_else(malformed)?;
    let mut content_length = None;
    let mut chunked = None;
    for line in lines {
        let (name, value) = line.split_once(':').ok_or_else(malformed)?;
        let value = value.trim();
        if name.eq_ignore_ascii_case("content-length") {
            if content_length.is_some() {
                return Err(ambiguous());
            }
            content_length = Some(value.parse::<usize>().map_err(|_| malformed())?);
        } else if name.eq_ignore_ascii_case("transfer-encoding") {
            if chunked.is_some() {
                return Err(ambiguous());
            }
            chunked = Some(value.eq_ignore_ascii_case("chunked"));
        }
    }
    match (content_length, chunked) {
        (Some(length), None) if length <= MAXIMUM_BODY_BYTES => {
            Ok((status, Framing::Length(length)))
        }
        (None, Some(true)) => Ok((status, Framing::Chunked)),
        _ => Err(ambiguous()),
    }
}

fn chunk_size(text: &[u8]) -> Result<usize, IndexError> {
    let malformed = || IndexError::Source("malformed chunked body".to_owned());
    if text.is_empty() || !text.iter().all(u8::is_ascii_hexdigit) {
        return Err(malformed());
    }
    usize::from_str_radix(std::str::from_utf8(text).map_err(|_| malformed())?, 16)
        .map_err(|_| malformed())
}

/// Splits one complete HTTP/1.1 response into status and exact body.
///
/// # Errors
/// Refuses ambiguous framing, truncation and oversized parts.
pub fn split_response(response: &[u8]) -> Result<(u16, Vec<u8>), IndexError> {
    let mut reader = BufReader::new(response);
    let result = read_response(&mut reader, None)?;
    if !reader.fill_buf().map_err(source_io)?.is_empty() {
        return Err(IndexError::Source("ambiguous HTTP framing".to_owned()));
    }
    Ok(result)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn endpoints_refuse_remote_plaintext_without_opt_in() {
        let timeout = Duration::from_secs(1);
        assert!(Endpoint::parse(
            "http://127.0.0.1:8545",
            Security::Plaintext {
                allow_remote: false
            },
            timeout
        )
        .is_ok());
        assert!(Endpoint::parse(
            "http://paxeer-node:8545",
            Security::Plaintext {
                allow_remote: false
            },
            timeout
        )
        .is_err());
        assert!(Endpoint::parse(
            "http://paxeer-node:8545",
            Security::Plaintext { allow_remote: true },
            timeout
        )
        .is_ok());
        assert!(Endpoint::parse(
            "https://paxeer-node",
            Security::Plaintext { allow_remote: true },
            timeout
        )
        .is_err());
        assert!(Endpoint::parse(
            "http://user@127.0.0.1",
            Security::Plaintext {
                allow_remote: false
            },
            timeout
        )
        .is_err());
    }

    #[test]
    fn framing_is_exact() {
        let framed = b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\n\r\n{}";
        assert_eq!(
            split_response(framed).unwrap_or((0, Vec::new())),
            (200, b"{}".to_vec())
        );
        let chunked = b"HTTP/1.1 200 OK\r\nTransfer-Encoding: chunked\r\n\r\n2\r\n{}\r\n0\r\n\r\n";
        assert_eq!(
            split_response(chunked).unwrap_or((0, Vec::new())),
            (200, b"{}".to_vec())
        );
        let both = b"HTTP/1.1 200 OK\r\nContent-Length: 2\r\nTransfer-Encoding: chunked\r\n\r\n{}";
        assert!(split_response(both).is_err());
        let short = b"HTTP/1.1 200 OK\r\nContent-Length: 3\r\n\r\n{}";
        assert!(split_response(short).is_err());
    }
}
