use std::io::{BufRead, BufReader, Read, Write};
use std::net::{IpAddr, TcpStream, ToSocketAddrs};
use std::time::Duration;

use openssl::ssl::{SslConnector, SslMethod, SslVerifyMode};
use openssl::x509::{store::X509StoreBuilder, X509};
use serde_json::{json, Value};

const MAXIMUM_RESPONSE_BYTES: usize = 4 * 1024 * 1024;
const MAXIMUM_HEADER_BYTES: usize = 32 * 1024;

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PaxeerEndpoint {
    pub url: String,
    pub request_timeout: Duration,
    pub trust_anchor_der: Option<Vec<u8>>,
    pub expected_chain_id: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum TransportFault {
    UnsupportedUrl,
    InsecureTransport,
    InvalidTrustAnchor,
    Authentication { detail: String },
    Connect { detail: String },
    Transport { detail: String },
    Http { status: u16 },
    ResponseTooLarge,
    AmbiguousFraming,
    MalformedResponse,
    Rpc { code: i64, message: String },
    ChainMismatch { expected: u64, actual: u64 },
    InconsistentObservation,
    UnexpectedValue { detail: String },
}

impl std::fmt::Display for TransportFault {
    fn fmt(&self, formatter: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(formatter, "{self:?}")
    }
}

impl std::error::Error for TransportFault {}

impl PaxeerEndpoint {
    pub fn new(
        url: impl Into<String>,
        request_timeout: Duration,
        trust_anchor_der: Option<Vec<u8>>,
        expected_chain_id: u64,
        local_emulator: bool,
    ) -> Result<Self, TransportFault> {
        let endpoint = Self {
            url: url.into(),
            request_timeout,
            trust_anchor_der,
            expected_chain_id,
        };
        let target = parse_url(&endpoint.url)?;
        if local_emulator && (target.tls || endpoint.trust_anchor_der.is_some())
            || !local_emulator && !target.tls
        {
            return Err(TransportFault::InsecureTransport);
        }
        endpoint.validate()?;
        Ok(endpoint)
    }

    fn validate(&self) -> Result<Target, TransportFault> {
        let target = parse_url(&self.url)?;
        if target.tls {
            trust_anchor(self.trust_anchor_der.as_deref())?;
        } else if self.trust_anchor_der.is_some() || !loopback(&target.host) {
            return Err(TransportFault::InsecureTransport);
        }
        if self.request_timeout.is_zero() || self.expected_chain_id == 0 {
            return Err(TransportFault::UnexpectedValue {
                detail: "timeout and expected chain identifier must be positive".to_owned(),
            });
        }
        Ok(target)
    }
}

pub struct PaxeerTransport;

impl PaxeerTransport {
    pub fn call(
        endpoint: &PaxeerEndpoint,
        method: &str,
        params: &Value,
    ) -> Result<Value, TransportFault> {
        let target = endpoint.validate()?;
        let chain = call_target(endpoint, &target, "eth_chainId", &json!([]))?;
        let actual = chain
            .as_str()
            .and_then(|text| text.strip_prefix("0x"))
            .filter(|digits| {
                !digits.is_empty()
                    && digits.bytes().all(|byte| byte.is_ascii_hexdigit())
                    && (digits.len() == 1 || !digits.starts_with('0'))
            })
            .and_then(|digits| u64::from_str_radix(digits, 16).ok())
            .ok_or_else(|| TransportFault::UnexpectedValue {
                detail: "chain identifier is not a u64 hex quantity".to_owned(),
            })?;
        if actual != endpoint.expected_chain_id {
            return Err(TransportFault::ChainMismatch {
                expected: endpoint.expected_chain_id,
                actual,
            });
        }
        if method == "eth_chainId" {
            return Ok(chain);
        }
        call_target(endpoint, &target, method, params)
    }
}

struct Target {
    tls: bool,
    host: String,
    authority: String,
    port: u16,
    path: String,
}

fn parse_url(url: &str) -> Result<Target, TransportFault> {
    let (tls, rest, default_port) = if let Some(rest) = url.strip_prefix("https://") {
        (true, rest, 443)
    } else if let Some(rest) = url.strip_prefix("http://") {
        (false, rest, 80)
    } else {
        return Err(TransportFault::UnsupportedUrl);
    };
    if rest.is_empty()
        || rest
            .bytes()
            .any(|byte| byte.is_ascii_control() || byte.is_ascii_whitespace())
        || rest.contains(['#', '@', '?', '\\'])
    {
        return Err(TransportFault::UnsupportedUrl);
    }
    let (authority, path) = rest
        .find('/')
        .map_or((rest, "/"), |index| rest.split_at(index));
    let (host, port_text) = if let Some(ipv6) = authority.strip_prefix('[') {
        let (host, suffix) = ipv6.split_once(']').ok_or(TransportFault::UnsupportedUrl)?;
        if host.parse::<std::net::Ipv6Addr>().is_err() {
            return Err(TransportFault::UnsupportedUrl);
        }
        let port = if suffix.is_empty() {
            None
        } else {
            Some(
                suffix
                    .strip_prefix(':')
                    .ok_or(TransportFault::UnsupportedUrl)?,
            )
        };
        (host, port)
    } else {
        if authority.contains(['[', ']']) || authority.matches(':').count() > 1 {
            return Err(TransportFault::UnsupportedUrl);
        }
        authority
            .split_once(':')
            .map_or((authority, None), |(host, port)| (host, Some(port)))
    };
    if host.is_empty()
        || (!host.contains(':')
            && !host
                .bytes()
                .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'.' | b'-')))
    {
        return Err(TransportFault::UnsupportedUrl);
    }
    let port = match port_text {
        Some(text) if !text.is_empty() && text.bytes().all(|byte| byte.is_ascii_digit()) => text
            .parse::<u16>()
            .map_err(|_| TransportFault::UnsupportedUrl)?,
        Some(_) => return Err(TransportFault::UnsupportedUrl),
        None => default_port,
    };
    if port == 0 {
        return Err(TransportFault::UnsupportedUrl);
    }
    Ok(Target {
        tls,
        host: host.to_owned(),
        authority: authority.to_owned(),
        port,
        path: path.to_owned(),
    })
}

fn loopback(host: &str) -> bool {
    host.eq_ignore_ascii_case("localhost")
        || host
            .parse::<IpAddr>()
            .is_ok_and(|address| address.is_loopback())
}

fn trust_anchor(der: Option<&[u8]>) -> Result<X509, TransportFault> {
    let der = der
        .filter(|bytes| !bytes.is_empty())
        .ok_or(TransportFault::InvalidTrustAnchor)?;
    let cert = X509::from_der(der).map_err(|_| TransportFault::InvalidTrustAnchor)?;
    if cert
        .to_der()
        .map_err(|_| TransportFault::InvalidTrustAnchor)?
        != der
    {
        return Err(TransportFault::InvalidTrustAnchor);
    }
    Ok(cert)
}

fn transport(error: std::io::Error) -> TransportFault {
    TransportFault::Transport {
        detail: error.to_string(),
    }
}

fn call_target(
    endpoint: &PaxeerEndpoint,
    target: &Target,
    method: &str,
    params: &Value,
) -> Result<Value, TransportFault> {
    let addresses = (target.host.as_str(), target.port)
        .to_socket_addrs()
        .map_err(|error| TransportFault::Connect {
            detail: error.to_string(),
        })?
        .collect::<Vec<_>>();
    if !target.tls && addresses.iter().any(|address| !address.ip().is_loopback()) {
        return Err(TransportFault::InsecureTransport);
    }
    let mut last_error = "no resolved addresses".to_owned();
    let mut connected = None;
    for address in addresses {
        match TcpStream::connect_timeout(&address, endpoint.request_timeout) {
            Ok(stream) => {
                connected = Some(stream);
                break;
            }
            Err(error) => last_error = error.to_string(),
        }
    }
    let mut tcp = connected.ok_or(TransportFault::Connect { detail: last_error })?;
    tcp.set_read_timeout(Some(endpoint.request_timeout))
        .map_err(transport)?;
    tcp.set_write_timeout(Some(endpoint.request_timeout))
        .map_err(transport)?;
    let body = json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).to_string();
    let response = if target.tls {
        let authentication = |error: openssl::error::ErrorStack| TransportFault::Authentication {
            detail: error.to_string(),
        };
        let mut roots = X509StoreBuilder::new().map_err(authentication)?;
        roots
            .add_cert(trust_anchor(endpoint.trust_anchor_der.as_deref())?)
            .map_err(authentication)?;
        let mut builder = SslConnector::builder(SslMethod::tls_client()).map_err(authentication)?;
        // Replace the connector's default store, so system roots cannot authenticate this endpoint.
        builder.set_cert_store(roots.build());
        builder.set_verify(SslVerifyMode::PEER);
        let mut tls = builder
            .build()
            .connect(&target.host, tcp)
            .map_err(|error| TransportFault::Authentication {
                detail: error.to_string(),
            })?;
        exchange(&mut tls, target, &body)?
    } else {
        exchange(&mut tcp, target, &body)?
    };
    let value: Value =
        serde_json::from_slice(&response).map_err(|_| TransportFault::MalformedResponse)?;
    if value.get("jsonrpc").and_then(Value::as_str) != Some("2.0")
        || value.get("id") != Some(&json!(1))
    {
        return Err(TransportFault::MalformedResponse);
    }
    if let Some(error) = value.get("error").filter(|error| !error.is_null()) {
        if value.get("result").is_some() {
            return Err(TransportFault::MalformedResponse);
        }
        return Err(TransportFault::Rpc {
            code: error
                .get("code")
                .and_then(Value::as_i64)
                .ok_or(TransportFault::MalformedResponse)?,
            message: error
                .get("message")
                .and_then(Value::as_str)
                .ok_or(TransportFault::MalformedResponse)?
                .to_owned(),
        });
    }
    value
        .get("result")
        .cloned()
        .ok_or(TransportFault::MalformedResponse)
}

fn line<R: BufRead>(reader: &mut R, budget: &mut usize) -> Result<Vec<u8>, TransportFault> {
    let mut bytes = Vec::new();
    let limit = (*budget).min(MAXIMUM_HEADER_BYTES);
    let count = reader
        .take((limit + 1) as u64)
        .read_until(b'\n', &mut bytes)
        .map_err(transport)?;
    if count > limit {
        return Err(TransportFault::ResponseTooLarge);
    }
    *budget -= count;
    if !bytes.ends_with(b"\r\n") {
        return Err(TransportFault::MalformedResponse);
    }
    bytes.truncate(bytes.len() - 2);
    Ok(bytes)
}

fn exchange<S: Read + Write>(
    stream: &mut S,
    target: &Target,
    body: &str,
) -> Result<Vec<u8>, TransportFault> {
    write!(stream, "POST {} HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nAccept: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", target.path, target.authority, body.len(), body).map_err(transport)?;
    stream.flush().map_err(transport)?;
    let mut reader = BufReader::new(stream);
    let mut budget = MAXIMUM_RESPONSE_BYTES;
    let status_line = line(&mut reader, &mut budget)?;
    let status_line =
        std::str::from_utf8(&status_line).map_err(|_| TransportFault::MalformedResponse)?;
    let mut parts = status_line.splitn(3, ' ');
    if parts.next() != Some("HTTP/1.1") {
        return Err(TransportFault::MalformedResponse);
    }
    let status: u16 = parts
        .next()
        .filter(|code| code.len() == 3 && code.bytes().all(|byte| byte.is_ascii_digit()))
        .and_then(|code| code.parse().ok())
        .filter(|code| (100..=599).contains(code))
        .ok_or(TransportFault::MalformedResponse)?;
    if parts.next().is_none() {
        return Err(TransportFault::MalformedResponse);
    }
    let mut content_length = None;
    let mut chunked = false;
    loop {
        let bytes = line(&mut reader, &mut budget)?;
        if MAXIMUM_RESPONSE_BYTES - budget > MAXIMUM_HEADER_BYTES {
            return Err(TransportFault::ResponseTooLarge);
        }
        if bytes.is_empty() {
            break;
        }
        let text = std::str::from_utf8(&bytes).map_err(|_| TransportFault::MalformedResponse)?;
        let (name, value) = text
            .split_once(':')
            .ok_or(TransportFault::MalformedResponse)?;
        let value = value.trim();
        if name.eq_ignore_ascii_case("content-length") {
            if content_length.is_some() || chunked {
                return Err(TransportFault::AmbiguousFraming);
            }
            if value.is_empty() || !value.bytes().all(|byte| byte.is_ascii_digit()) {
                return Err(TransportFault::MalformedResponse);
            }
            content_length = Some(
                value
                    .parse::<usize>()
                    .map_err(|_| TransportFault::ResponseTooLarge)?,
            );
        } else if name.eq_ignore_ascii_case("transfer-encoding") {
            if chunked || content_length.is_some() || !value.eq_ignore_ascii_case("chunked") {
                return Err(TransportFault::AmbiguousFraming);
            }
            chunked = true;
        }
    }
    let mut body = Vec::new();
    if let Some(length) = content_length {
        read_body(&mut reader, &mut budget, &mut body, length)?;
    } else if chunked {
        loop {
            let bytes = line(&mut reader, &mut budget)?;
            if bytes.is_empty() || !bytes.iter().all(u8::is_ascii_hexdigit) {
                return Err(TransportFault::MalformedResponse);
            }
            let text =
                std::str::from_utf8(&bytes).map_err(|_| TransportFault::MalformedResponse)?;
            let length =
                usize::from_str_radix(text, 16).map_err(|_| TransportFault::ResponseTooLarge)?;
            if length == 0 {
                if !line(&mut reader, &mut budget)?.is_empty() {
                    return Err(TransportFault::AmbiguousFraming);
                }
                break;
            }
            read_body(&mut reader, &mut budget, &mut body, length)?;
            if !line(&mut reader, &mut budget)?.is_empty() {
                return Err(TransportFault::MalformedResponse);
            }
        }
    } else {
        return Err(TransportFault::AmbiguousFraming);
    }
    if status != 200 {
        return Err(TransportFault::Http { status });
    }
    Ok(body)
}

fn read_body<R: Read>(
    reader: &mut R,
    budget: &mut usize,
    body: &mut Vec<u8>,
    length: usize,
) -> Result<(), TransportFault> {
    if length > *budget {
        return Err(TransportFault::ResponseTooLarge);
    }
    *budget -= length;
    let start = body.len();
    body.resize(start + length, 0);
    reader.read_exact(&mut body[start..]).map_err(transport)
}
