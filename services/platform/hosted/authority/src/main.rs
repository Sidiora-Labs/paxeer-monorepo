use layerx_platform_authority::{
    chain_receipt, committed, json, loopback_http, protected, refusal, Response,
};
#[path = "committed_head_route.rs"]
mod committed_head_route;
pub mod human;
mod trust;

use chain_receipt::{ChainReceipts, ReceiptSource};

use layerx_client::sequencer::SequencerEndpoint;
use layerx_platform_authority::{
    authorized_batch_by_activity, hex, receipt_locator, EvidenceRefusal,
};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION as PROTOCOL_VERSION;
use rustls::pki_types::{CertificateDer, PrivateKeyDer};
use rustls::server::WebPkiClientVerifier;
use rustls::{RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicBool, AtomicUsize, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, Instant};
use subtle::ConstantTimeEq;
use zeroize::{Zeroize, Zeroizing};

const MAX_REQUEST_BYTES: usize = 16 * 1024;
const MAX_REPLICA_RESPONSE_BYTES: usize = 256 * 1024;
const MAX_TLS_FILE_BYTES: u64 = 64 * 1024;
const IO_TIMEOUT: Duration = Duration::from_secs(8);
const REPLICA_TIMEOUT: Duration = Duration::from_secs(8);
const READINESS_TIMEOUT: Duration = Duration::from_secs(2);
const MAX_CONNECTIONS: usize = 128;
const MAX_REQUESTS_PER_CONNECTION: usize = 64;
const MAX_READINESS_CHECKS: usize = 16;
const ZERO_HEX32: &str = "0000000000000000000000000000000000000000000000000000000000000000";
static ACTIVE_CONNECTIONS: AtomicUsize = AtomicUsize::new(0);

const USAGE: &str = "layerx-receipt-authority serves verified authorised-batch facts over TLS.

Routes (GET only):
  /livez
  /readyz                                              {ready, network_id, protocol_network_id, wire_version}
  /v1/authorized-batches/by-activity/{activity_id}     bearer required
  /v1/authorized-batches/wait-by-activity/{activity_id}
                                                       bearer required; waits for a publication notification
  /internal/v1/activities/{activity_id}/authority      bearer required
  /v1/batches/{batch_id}/receipt-authority?receipt_digest={digest}
                                                       bearer required, relayed to the replica unchanged

Environment:
  LAYERX_AUTHORITY_LISTEN                    listen address, default 0.0.0.0:9445
  LAYERX_AUTHORITY_TLS_CERT_DER              server certificate (DER)
  LAYERX_AUTHORITY_TLS_KEY_DER               server private key (PKCS#8 DER)
  LAYERX_AUTHORITY_CLIENT_CA_DER             optional; when set a presented client certificate must chain to it
  LAYERX_AUTHORITY_TOKEN_FILES               colon-separated files, one bearer token each (gateway, registry, webhooks)
  LAYERX_AUTHORITY_REPLICA_URL               loopback http://127.0.0.1:PORT of the independent receipt-authority replica
  LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE bearer token the replica requires
  LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE  optional protected file; its distinct bearer may read only /v1/batches/{batch_id}/receipt-authority
  LAYERX_AUTHORITY_REPLICA_ID                64-hex replica identity every evidence document must carry
  LAYERX_AUTHORITY_CHAIN_RPC_URL             loopback http://host:port serving committed headers, pax_sequencerReceipt, pinned eth_call and /comet native-store proofs
  LAYERX_AUTHORITY_RECEIPT_KEYS_FILE         protected ReceiptKeyList JSON with pinned keys and height ranges
  LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID       numeric protocol network id expected in signed receipt evidence
  LAYERX_AUTHORITY_NETWORK_ID                deployment network identifier echoed in every answer
  LAYERX_AUTHORITY_WIRE_VERSION              wire version echoed in every answer, must be the built protocol version (default 3)
  LAYERX_AUTHORITY_SEQUENCER_ID              64-hex sequencer identity pinned for header verification
  LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY      64-hex sequencer public key pinned for header and receipt signatures
  LAYERX_AUTHORITY_FIRST_BATCH               first authorised batch number
  LAYERX_AUTHORITY_LAST_BATCH                last authorised batch number
";

pub struct Config {
    listen: SocketAddr,
    tls: Arc<ServerConfig>,
    tokens: Vec<Zeroizing<String>>,
    human: Option<human::Human>,
    replica_address: SocketAddr,
    replica_host: String,
    replica_token: Zeroizing<String>,
    evidence_read_token: Option<Zeroizing<Vec<u8>>>,
    replica_id: [u8; 32],
    chain_receipts: ChainReceipts,
    chain_rpc_url: String,
    protocol_network_id: u32,
    network_id: String,
    wire_version: String,
    authorization: SequencerAuthorization,
    sequencer_public_key: [u8; 32],
}

impl layerx_platform_authority::budget_state::Configuration for Config {
    fn chain_rpc_url(&self) -> &str {
        &self.chain_rpc_url
    }

    fn protocol_network_id(&self) -> u32 {
        self.protocol_network_id
    }
}

struct Request {
    method: String,
    path: String,
    query: Option<String>,
    headers: BTreeMap<String, String>,
}

enum ReplicaAnswer {
    Status(u16, Vec<u8>),
    Unavailable,
}

fn valid_identifier(value: &str, max: usize) -> bool {
    !value.is_empty()
        && value.len() <= max
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.'))
}

fn read_secret_file(path: &str, name: &str) -> Result<Zeroizing<String>, String> {
    let mut value = fs::read_to_string(path).map_err(|error| format!("{name}: {error}"))?;
    while matches!(value.as_bytes().last(), Some(b'\n' | b'\r')) {
        value.pop();
    }
    if value.is_empty()
        || value.len() > 4096
        || value.bytes().any(|byte| !(0x21..=0x7e).contains(&byte))
    {
        value.zeroize();
        return Err(format!(
            "{name} does not contain a bounded printable secret"
        ));
    }
    Ok(Zeroizing::new(value))
}

fn read_secret(variable: &str) -> Result<Zeroizing<String>, String> {
    let path = env::var(variable).map_err(|_| format!("{variable} is required"))?;
    read_secret_file(&path, variable)
}

fn read_evidence_read_token(
    tokens: &[Zeroizing<String>],
    replica_token: &Zeroizing<String>,
) -> Result<Option<Zeroizing<Vec<u8>>>, String> {
    let Some(path) = env::var_os("LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE") else {
        return Ok(None);
    };
    let token = Zeroizing::new(protected::read(&PathBuf::from(path), 4096).map_err(
        |layerx_platform_authority::AuthorityError| {
            "LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE is unavailable or unprotected".to_owned()
        },
    )?);
    if token.len() < 32 || !token.iter().all(|byte| (0x21..=0x7e).contains(byte)) {
        return Err("LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE must contain 32..4096 printable bytes without whitespace".to_owned());
    }
    let collides = tokens
        .iter()
        .chain(std::iter::once(replica_token))
        .fold(false, |found, other| {
            found | bool::from(other.as_bytes().ct_eq(token.as_slice()))
        });
    if collides {
        return Err(
            "LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE must differ from every general and replica bearer"
                .to_owned(),
        );
    }
    Ok(Some(token))
}

fn read_bounded(variable: &str) -> Result<Vec<u8>, String> {
    let path = env::var(variable).map_err(|_| format!("{variable} is required"))?;
    let metadata = fs::metadata(&path).map_err(|error| format!("{variable}: {error}"))?;
    if !metadata.is_file() || metadata.len() == 0 || metadata.len() > MAX_TLS_FILE_BYTES {
        return Err(format!("{variable} is not a bounded regular file"));
    }
    fs::read(&path).map_err(|error| format!("{variable}: {error}"))
}

fn parse_hex32(variable: &str) -> Result<[u8; 32], String> {
    let value = env::var(variable).map_err(|_| format!("{variable} is required"))?;
    hex::decode32(&value).map_err(|_| format!("{variable} must be 64 hexadecimal characters"))
}

fn parse_u64(variable: &str) -> Result<u64, String> {
    env::var(variable)
        .map_err(|_| format!("{variable} is required"))?
        .parse::<u64>()
        .map_err(|_| format!("{variable} must be an integer"))
}

fn server_tls_config() -> Result<Arc<ServerConfig>, String> {
    rustls::crypto::ring::default_provider()
        .install_default()
        .map_err(|_| "failed to install the authority TLS provider".to_owned())?;
    let certificate = CertificateDer::from(read_bounded("LAYERX_AUTHORITY_TLS_CERT_DER")?);
    let private_key = PrivateKeyDer::try_from(read_bounded("LAYERX_AUTHORITY_TLS_KEY_DER")?)
        .map_err(|_| "LAYERX_AUTHORITY_TLS_KEY_DER is not a PKCS#8 private key".to_owned())?;
    let builder = ServerConfig::builder();
    let config = if env::var_os("LAYERX_AUTHORITY_CLIENT_CA_DER").is_some() {
        let client_ca = CertificateDer::from(read_bounded("LAYERX_AUTHORITY_CLIENT_CA_DER")?);
        let mut roots = RootCertStore::empty();
        roots
            .add(client_ca)
            .map_err(|_| "LAYERX_AUTHORITY_CLIENT_CA_DER is not a CA certificate".to_owned())?;
        let verifier = WebPkiClientVerifier::builder(roots.into())
            .allow_unauthenticated()
            .build()
            .map_err(|_| "authority client certificate verifier is invalid".to_owned())?;
        builder
            .with_client_cert_verifier(verifier)
            .with_single_cert(vec![certificate], private_key)
    } else {
        builder
            .with_no_client_auth()
            .with_single_cert(vec![certificate], private_key)
    }
    .map_err(|_| "authority TLS identity is invalid".to_owned())?;
    Ok(Arc::new(config))
}

fn config() -> Result<Config, String> {
    let listen = env::var("LAYERX_AUTHORITY_LISTEN")
        .unwrap_or_else(|_| "0.0.0.0:9445".to_owned())
        .parse::<SocketAddr>()
        .map_err(|_| "LAYERX_AUTHORITY_LISTEN must be a socket address".to_owned())?;
    let tls = server_tls_config()?;
    let token_files = env::var("LAYERX_AUTHORITY_TOKEN_FILES")
        .map_err(|_| "LAYERX_AUTHORITY_TOKEN_FILES is required")?;
    let mut tokens = Vec::new();
    for path in token_files.split(':').filter(|path| !path.is_empty()) {
        tokens.push(read_secret_file(path, "LAYERX_AUTHORITY_TOKEN_FILES")?);
    }
    if tokens.is_empty() {
        return Err("LAYERX_AUTHORITY_TOKEN_FILES names no token file".to_owned());
    }
    let (replica_host, replica_address) = env::var("LAYERX_AUTHORITY_REPLICA_URL")
        .ok()
        .as_deref()
        .and_then(loopback_http)
        .ok_or_else(|| {
            "LAYERX_AUTHORITY_REPLICA_URL must be a loopback http://host:port endpoint".to_owned()
        })?;
    let replica_token = read_secret("LAYERX_AUTHORITY_REPLICA_BEARER_TOKEN_FILE")?;
    let evidence_read_token = read_evidence_read_token(&tokens, &replica_token)?;
    let mut human_peers = tokens.clone();
    if let Some(token) = &evidence_read_token {
        human_peers.push(Zeroizing::new(String::from_utf8(token.to_vec()).map_err(
            |_| "LAYERX_AUTHORITY_EVIDENCE_READ_TOKEN_FILE is not printable",
        )?));
    }
    let replica_id = parse_hex32("LAYERX_AUTHORITY_REPLICA_ID")?;
    if replica_id == [0; 32] {
        return Err("LAYERX_AUTHORITY_REPLICA_ID must not be zero".to_owned());
    }
    let chain_rpc_url = env::var("LAYERX_AUTHORITY_CHAIN_RPC_URL")
        .map_err(|_| "LAYERX_AUTHORITY_CHAIN_RPC_URL is required")?;
    let receipt_keys_file = env::var("LAYERX_AUTHORITY_RECEIPT_KEYS_FILE")
        .map_err(|_| "LAYERX_AUTHORITY_RECEIPT_KEYS_FILE is required")?;
    let chain_receipts = ChainReceipts::load(&chain_rpc_url, &PathBuf::from(receipt_keys_file))?;
    let protocol_network_id = parse_u64("LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID")?;
    let protocol_network_id = u32::try_from(protocol_network_id)
        .ok()
        .filter(|value| *value != 0)
        .ok_or_else(|| {
            "LAYERX_AUTHORITY_PROTOCOL_NETWORK_ID must be a non-zero 32-bit integer".to_owned()
        })?;
    let network_id = env::var("LAYERX_AUTHORITY_NETWORK_ID")
        .map_err(|_| "LAYERX_AUTHORITY_NETWORK_ID is required")?;
    let wire_version =
        env::var("LAYERX_AUTHORITY_WIRE_VERSION").unwrap_or_else(|_| PROTOCOL_VERSION.to_string());
    if !valid_identifier(&network_id, 64) || !valid_identifier(&wire_version, 32) {
        return Err(
            "LAYERX_AUTHORITY_NETWORK_ID or LAYERX_AUTHORITY_WIRE_VERSION is invalid".to_owned(),
        );
    }
    if wire_version.parse::<u16>().ok() != Some(PROTOCOL_VERSION) {
        return Err("LAYERX_AUTHORITY_WIRE_VERSION is not the built protocol version".to_owned());
    }
    let sequencer_id = parse_hex32("LAYERX_AUTHORITY_SEQUENCER_ID")?;
    let sequencer_public_key = parse_hex32("LAYERX_AUTHORITY_SEQUENCER_PUBLIC_KEY")?;
    if sequencer_id == [0; 32] || sequencer_public_key == [0; 32] {
        return Err("sequencer identity and public key must not be zero".to_owned());
    }
    let first_batch = parse_u64("LAYERX_AUTHORITY_FIRST_BATCH")?;
    let last_batch = parse_u64("LAYERX_AUTHORITY_LAST_BATCH")?;
    if first_batch == 0 || last_batch < first_batch {
        return Err(
            "LAYERX_AUTHORITY_FIRST_BATCH..LAYERX_AUTHORITY_LAST_BATCH is not a batch range"
                .to_owned(),
        );
    }
    Ok(Config {
        listen,
        tls,
        human: human::Human::load(&human_peers)?,
        tokens,
        replica_address,
        replica_host,
        replica_token,
        evidence_read_token,
        replica_id,
        chain_receipts,
        chain_rpc_url,
        protocol_network_id,
        network_id,
        wire_version,
        authorization: SequencerAuthorization::new(
            sequencer_id,
            sequencer_public_key,
            first_batch,
            last_batch,
        ),
        sequencer_public_key,
    })
}

fn read_http_message(stream: &mut impl Read) -> Result<Request, String> {
    let mut bytes = Vec::with_capacity(2048);
    let mut chunk = [0_u8; 2048];
    let header_end = loop {
        let count = stream.read(&mut chunk).map_err(|error| error.to_string())?;
        if count == 0 || bytes.len().saturating_add(count) > MAX_REQUEST_BYTES {
            return Err("HTTP message is empty or exceeds its bound".to_owned());
        }
        bytes.extend_from_slice(&chunk[..count]);
        if let Some(position) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            break position + 4;
        }
    };
    let source = std::str::from_utf8(&bytes[..header_end])
        .map_err(|_| "HTTP headers are not UTF-8".to_owned())?;
    let mut lines = source.split("\r\n");
    let start = lines
        .next()
        .ok_or_else(|| "HTTP start line is missing".to_owned())?
        .to_owned();
    let mut headers = BTreeMap::new();
    let mut content_length = 0_usize;
    for line in lines.filter(|line| !line.is_empty()) {
        let (name, value) = line
            .split_once(':')
            .ok_or_else(|| "HTTP header is malformed".to_owned())?;
        let name = name.trim().to_ascii_lowercase();
        let value = value.trim().to_owned();
        if headers.contains_key(&name) {
            return Err("duplicate HTTP header".to_owned());
        }
        if name == "transfer-encoding" {
            return Err("transfer-encoded messages are not accepted".to_owned());
        }
        if name == "content-length" {
            content_length = value
                .parse::<usize>()
                .map_err(|_| "content length is invalid".to_owned())?;
        }
        headers.insert(name, value);
    }
    if content_length != 0 {
        return Err("request bodies are not accepted".to_owned());
    }
    if bytes.len() != header_end {
        return Err("request carries unexpected bytes".to_owned());
    }
    let mut parts = start.split_whitespace();
    let method = parts
        .next()
        .ok_or_else(|| "request method is missing".to_owned())?
        .to_owned();
    let target = parts
        .next()
        .ok_or_else(|| "request target is missing".to_owned())?;
    if parts.next() != Some("HTTP/1.1") || parts.next().is_some() || !target.starts_with('/') {
        return Err("request line is invalid".to_owned());
    }
    if !headers.contains_key("host") {
        return Err("HTTP/1.1 Host header is required".to_owned());
    }
    let (path, query) = target
        .split_once('?')
        .map_or((target.to_owned(), None), |(path, query)| {
            (path.to_owned(), Some(query.to_owned()))
        });
    if path.contains('#')
        || query
            .as_deref()
            .is_some_and(|query| query.contains(['?', '#']))
    {
        return Err("request target is invalid".to_owned());
    }
    Ok(Request {
        method,
        path,
        query,
        headers,
    })
}

fn write_response(
    stream: &mut impl Write,
    response: &Response,
    keep_alive: bool,
) -> Result<(), String> {
    let reason = match response.status {
        200 => "OK",
        400 => "Bad Request",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not Found",
        405 => "Method Not Allowed",
        502 => "Bad Gateway",
        _ => "Service Unavailable",
    };
    let retry = response.retry_after.map_or(String::new(), |seconds| {
        format!("Retry-After: {seconds}\r\n")
    });
    let diagnostics = response
        .readiness_headers
        .map_or_else(String::new, |[replica, chain]| {
            format!(
                "X-LayerX-Authority-Replica: {replica}\r\nX-LayerX-Authority-Chain: {chain}\r\n"
            )
        });
    let connection = if keep_alive { "keep-alive" } else { "close" };
    let head = format!(
        "HTTP/1.1 {} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nCache-Control: no-store\r\n{retry}{diagnostics}Connection: {connection}\r\n\r\n",
        response.status,
        response.body.len()
    );
    stream
        .write_all(head.as_bytes())
        .and_then(|()| stream.write_all(&response.body))
        .and_then(|()| stream.flush())
        .map_err(|error| error.to_string())
}

fn authenticate(config: &Config, request: &Request) -> Result<(), Response> {
    let Some(presented) = request
        .headers
        .get("authorization")
        .and_then(|value| value.strip_prefix("Bearer "))
    else {
        return Err(refusal(401, "identity_required", None));
    };
    if presented.is_empty() || presented.len() > 4096 {
        return Err(refusal(401, "identity_required", None));
    }
    let accepted = config.tokens.iter().any(|token| {
        token.len() == presented.len() && bool::from(token.as_bytes().ct_eq(presented.as_bytes()))
    });
    if accepted {
        Ok(())
    } else {
        Err(refusal(401, "identity_required", None))
    }
}

fn evidence_reader(config: &Config, request: &Request) -> bool {
    let Some(token) = config.evidence_read_token.as_ref() else {
        return false;
    };
    request
        .headers
        .get("authorization")
        .and_then(|value| value.strip_prefix("Bearer "))
        .is_some_and(|presented| bool::from(token.as_slice().ct_eq(presented.as_bytes())))
}

fn replica_get(config: &Config, path: &str) -> ReplicaAnswer {
    match replica_exchange(config, path) {
        Ok(answer) => answer,
        Err(error) => {
            eprintln!("layerx-receipt-authority replica GET failed: {error}");
            ReplicaAnswer::Unavailable
        }
    }
}

fn replica_exchange(config: &Config, path: &str) -> Result<ReplicaAnswer, String> {
    replica_exchange_until(config, path, Instant::now() + REPLICA_TIMEOUT)
}

fn replica_exchange_until(
    config: &Config,
    path: &str,
    deadline: Instant,
) -> Result<ReplicaAnswer, String> {
    let remaining = || {
        deadline
            .checked_duration_since(Instant::now())
            .filter(|duration| !duration.is_zero())
            .ok_or_else(|| "replica deadline elapsed".to_owned())
    };
    let mut stream = TcpStream::connect_timeout(&config.replica_address, remaining()?)
        .map_err(|error| format!("connect: {error}"))?;
    stream
        .set_read_timeout(Some(remaining()?))
        .and_then(|()| {
            stream.set_write_timeout(Some(deadline.saturating_duration_since(Instant::now())))
        })
        .map_err(|error| format!("timeout: {error}"))?;
    let request = format!(
        "GET {path} HTTP/1.1\r\nHost: {}\r\nAuthorization: Bearer {}\r\nAccept: application/json\r\nConnection: close\r\n\r\n",
        config.replica_host,
        config.replica_token.as_str()
    );
    stream
        .write_all(request.as_bytes())
        .map_err(|error| format!("write: {error}"))?;
    let mut raw = Vec::new();
    let mut chunk = [0_u8; 4096];
    loop {
        stream
            .set_read_timeout(Some(remaining()?))
            .map_err(|error| format!("timeout: {error}"))?;
        let count = stream
            .read(&mut chunk)
            .map_err(|error| format!("read: {error}"))?;
        if count == 0 {
            break;
        }
        raw.extend_from_slice(&chunk[..count]);
        if raw.len() > MAX_REPLICA_RESPONSE_BYTES {
            return Err("response exceeds the replica document bound".to_owned());
        }
    }
    let head_end = raw
        .windows(4)
        .position(|window| window == b"\r\n\r\n")
        .ok_or_else(|| "response lacks a header terminator".to_owned())?;
    let head = std::str::from_utf8(&raw[..head_end])
        .map_err(|_| "response head is not UTF-8".to_owned())?;
    let mut lines = head.split("\r\n");
    let status_line = lines
        .next()
        .ok_or_else(|| "response lacks a status line".to_owned())?;
    let status = status_line
        .strip_prefix("HTTP/1.1 ")
        .or_else(|| status_line.strip_prefix("HTTP/1.0 "))
        .and_then(|rest| rest.split(' ').next())
        .and_then(|code| code.parse::<u16>().ok())
        .filter(|code| (100..=599).contains(code))
        .ok_or_else(|| "response status line is malformed".to_owned())?;
    let mut content_length = None;
    for line in lines {
        let (name, value) = line
            .split_once(':')
            .ok_or_else(|| "response header is malformed".to_owned())?;
        if name.trim().eq_ignore_ascii_case("content-length") {
            let length = value
                .trim()
                .parse::<usize>()
                .map_err(|_| "response content length is malformed".to_owned())?;
            if content_length.replace(length).is_some() {
                return Err("response repeats content length".to_owned());
            }
        }
        if name.trim().eq_ignore_ascii_case("transfer-encoding") {
            return Err("response uses a transfer encoding".to_owned());
        }
    }
    let body = raw[head_end + 4..].to_vec();
    match content_length {
        Some(length) if length == body.len() => Ok(ReplicaAnswer::Status(status, body)),
        Some(_) => Err("response body length disagrees with its header".to_owned()),
        None if body.is_empty() => Ok(ReplicaAnswer::Status(status, body)),
        None => Err("response body lacks a content length".to_owned()),
    }
}

#[derive(Clone, Copy, Debug, serde::Serialize, PartialEq, Eq)]
#[serde(rename_all = "snake_case")]
enum DependencyReason {
    Ready,
    Empty,
    Unavailable,
    Timeout,
    IdentityMismatch,
    ConnectionLimit,
}

#[derive(Clone, Copy, serde::Serialize)]
struct DependencyReadiness {
    ready: bool,
    reason: DependencyReason,
}

impl DependencyReadiness {
    fn from_result(result: Result<DependencyReason, DependencyReason>) -> Self {
        match result {
            Ok(reason) => Self {
                ready: true,
                reason,
            },
            Err(reason) => Self {
                ready: false,
                reason,
            },
        }
    }
}

fn lookup_receipt(
    config: &Config,
    activity_id: [u8; 32],
    _wait_publication: bool,
) -> ReceiptSource {
    config.chain_receipts.lookup(activity_id)
}

fn evidence_refusal(refusal_kind: &EvidenceRefusal) -> Response {
    eprintln!("layerx-receipt-authority refused replica evidence: {refusal_kind:?}");
    refusal(502, "evidence_refused", None)
}

fn await_requested_receipt(
    config: &Config,
    activity_id: [u8; 32],
    wait_publication: bool,
    deadline: Instant,
) -> ReceiptSource {
    loop {
        let source = lookup_receipt(config, activity_id, wait_publication);
        if !matches!(source, ReceiptSource::Unknown)
            || !wait_publication
            || Instant::now() >= deadline
        {
            return source;
        }
        thread::sleep(Duration::from_millis(25));
    }
}

fn await_replica_evidence(
    config: &Config,
    path: &str,
    wait: bool,
    deadline: Instant,
) -> ReplicaAnswer {
    loop {
        let answer = replica_get(config, path);
        if !matches!(answer, ReplicaAnswer::Status(404, _)) || !wait || Instant::now() >= deadline {
            return answer;
        }
        thread::sleep(Duration::from_millis(25));
    }
}

fn by_activity(config: &Config, requested: &str, wait_publication: bool) -> Response {
    let Ok(activity_id) = hex::decode32(requested) else {
        return refusal(400, "invalid_activity_id", None);
    };
    if activity_id == [0; 32] {
        return refusal(400, "invalid_activity_id", None);
    }
    let deadline = Instant::now() + IO_TIMEOUT;
    let receipt = match await_requested_receipt(config, activity_id, wait_publication, deadline) {
        ReceiptSource::Found(receipt) => receipt,
        ReceiptSource::Unknown => return refusal(404, "unknown_activity", None),
        ReceiptSource::KeyMismatch => {
            eprintln!(
                "layerx-receipt-authority: chain receipt key differs from the pinned keys ({})",
                hex::encode(&config.sequencer_public_key)
            );
            return refusal(502, "sequencer_key_mismatch", None);
        }
        ReceiptSource::Unavailable(reason) => {
            eprintln!("layerx-receipt-authority receipt source unavailable: {reason}");
            return refusal(503, "receipt_source_unavailable", Some(5));
        }
    };
    let locator = match receipt_locator(&receipt) {
        Ok(locator) => locator,
        Err(error) => return evidence_refusal(&error),
    };
    if locator.activity_id != activity_id {
        return evidence_refusal(&EvidenceRefusal::ActivityMismatch);
    }
    let path = format!(
        "/v1/batches/{}/receipt-authority?receipt_digest={}",
        hex::encode(&locator.batch_id),
        hex::encode(&locator.receipt_digest)
    );
    let document = match await_replica_evidence(config, &path, wait_publication, deadline) {
        ReplicaAnswer::Status(200, body) => body,
        ReplicaAnswer::Status(404, _) => {
            return refusal(503, "replica_evidence_unavailable", Some(1));
        }
        ReplicaAnswer::Status(status, _) => {
            eprintln!("layerx-receipt-authority replica answered HTTP {status}");
            return refusal(503, "replica_unavailable", Some(5));
        }
        ReplicaAnswer::Unavailable => return refusal(503, "replica_unavailable", Some(5)),
    };
    let (evidence, authorization) = match trust::replica(config, &receipt, &document) {
        Ok(verified) => verified,
        Err(error) => return evidence_refusal(&error),
    };
    if layerx_wire::receipt::decode_batch_header(&evidence.header).map_or(true, |header| {
        header.network_id() != config.protocol_network_id
    }) {
        return refusal(503, "receipt_network_mismatch", Some(5));
    }
    match authorized_batch_by_activity(activity_id, &receipt, &evidence, &authorization) {
        Ok(facts) => {
            if config
                .human
                .as_ref()
                .is_some_and(|human| human.retain(&receipt, &document, config).is_err())
            {
                return refusal(503, "state_persistence_unavailable", Some(5));
            }
            let mut response = serde_json::json!({
                "activity_id": requested,
                "receipt": hex::encode(&receipt),
                "batch_id": hex::encode(&facts.batch_id),
                "asset": hex::encode(&facts.asset),
                "previous_state_root": hex::encode(&facts.previous_state_root),
                "resulting_state_root": hex::encode(&facts.resulting_state_root),
                "sequencer_public_key": hex::encode(&facts.sequencer_public_key),
                "network_id": config.network_id,
                "protocol_network_id": config.protocol_network_id,
                "wire_version": config.wire_version,
            });
            if matches!(
                evidence.batch_identity,
                layerx_platform_authority::BatchIdentityEvidence::OccupancyMaintenanceV2 { .. }
                    | layerx_platform_authority::BatchIdentityEvidence::BatchMaintenanceV1 { .. }
            ) {
                let replica: serde_json::Value = match serde_json::from_slice(&document) {
                    Ok(value) => value,
                    Err(_) => return evidence_refusal(&EvidenceRefusal::ReplicaDocument),
                };
                response["batch_evidence"] = replica["batch_evidence"].clone();
            }
            json(200, &response)
        }
        Err(error) => evidence_refusal(&error),
    }
}

fn relay(config: &Config, batch_id: &str, query: Option<&str>) -> Response {
    let Some(digest) = query.and_then(|query| query.strip_prefix("receipt_digest=")) else {
        return refusal(400, "invalid_request", None);
    };
    if !hex::is_hex32(batch_id) || !hex::is_hex32(digest) {
        return refusal(400, "invalid_request", None);
    }
    let path = format!("/v1/batches/{batch_id}/receipt-authority?receipt_digest={digest}");
    match replica_get(config, &path) {
        ReplicaAnswer::Status(status @ (200 | 404), body) => Response {
            status,
            body,
            retry_after: None,
            readiness_headers: None,
        },
        ReplicaAnswer::Status(status, _) => {
            eprintln!("layerx-receipt-authority replica answered HTTP {status}");
            refusal(503, "replica_unavailable", Some(5))
        }
        ReplicaAnswer::Unavailable => refusal(503, "replica_unavailable", Some(5)),
    }
}

static ACTIVE_READINESS: AtomicUsize = AtomicUsize::new(0);

struct ReadinessPermit;
impl ReadinessPermit {
    fn acquire() -> Option<Arc<Self>> {
        ACTIVE_READINESS
            .fetch_update(Ordering::AcqRel, Ordering::Acquire, |active| {
                (active < MAX_READINESS_CHECKS).then_some(active + 1)
            })
            .ok()
            .map(|_| Arc::new(Self))
    }
}
impl Drop for ReadinessPermit {
    fn drop(&mut self) {
        ACTIVE_READINESS.fetch_sub(1, Ordering::AcqRel);
    }
}

fn replica_answers(
    config: &Config,
    deadline: Instant,
) -> Result<DependencyReason, DependencyReason> {
    let path = format!("/v1/batches/{ZERO_HEX32}/receipt-authority?receipt_digest={ZERO_HEX32}");
    match replica_exchange_until(config, &path, deadline) {
        Ok(ReplicaAnswer::Status(200, _)) => Ok(DependencyReason::Ready),
        Ok(ReplicaAnswer::Status(404, _)) => Ok(DependencyReason::Empty),
        _ if Instant::now() >= deadline => Err(DependencyReason::Timeout),
        _ => Err(DependencyReason::Unavailable),
    }
}

fn chain_answers(config: &Config, deadline: Instant) -> Result<DependencyReason, DependencyReason> {
    if Instant::now() >= deadline {
        return Err(DependencyReason::Timeout);
    }
    let result =
        SequencerEndpoint::new(&config.chain_rpc_url, &[]).and_then(|endpoint| endpoint.chain_id());
    match result {
        Ok(id) if id != trust::EXPECTED_CHAIN_ID => {
            return Err(DependencyReason::IdentityMismatch);
        }
        Err(_) if Instant::now() >= deadline => return Err(DependencyReason::Timeout),
        Err(_) => return Err(DependencyReason::Unavailable),
        Ok(_) => {}
    }
    if Instant::now() >= deadline {
        return Err(DependencyReason::Timeout);
    }
    Ok(DependencyReason::Ready)
}

#[derive(serde::Serialize)]
struct AuthorityDependencies {
    replica: DependencyReadiness,
    chain: DependencyReadiness,
}

#[derive(serde::Serialize)]
struct AuthorityReadinessResponse<'a> {
    ready: bool,
    network_id: &'a str,
    protocol_network_id: u32,
    wire_version: &'a str,
}

fn readiness(config: &Arc<Config>) -> Response {
    let deadline = Instant::now() + READINESS_TIMEOUT;
    let unavailable = DependencyReadiness::from_result(Err(DependencyReason::Timeout));
    let mut dependencies = AuthorityDependencies {
        replica: unavailable,
        chain: unavailable,
    };
    if let Some(permit) = ReadinessPermit::acquire() {
        let (send, receive) = std::sync::mpsc::channel();
        for is_chain in [false, true] {
            let config = Arc::clone(config);
            let send = send.clone();
            let permit = Arc::clone(&permit);
            thread::spawn(move || {
                let _permit = permit;
                let result = if is_chain {
                    chain_answers(&config, deadline)
                } else {
                    replica_answers(&config, deadline)
                };
                let _ = send.send((is_chain, DependencyReadiness::from_result(result)));
            });
        }
        drop(send);
        for _ in 0..2 {
            let Some(remaining) = deadline.checked_duration_since(Instant::now()) else {
                break;
            };
            let Ok((is_chain, result)) = receive.recv_timeout(remaining) else {
                break;
            };
            if is_chain {
                dependencies.chain = result;
            } else {
                dependencies.replica = result;
            }
        }
    } else {
        let limited = DependencyReadiness::from_result(Err(DependencyReason::ConnectionLimit));
        dependencies = AuthorityDependencies {
            replica: limited,
            chain: limited,
        };
    }
    let ready = dependencies.replica.ready && dependencies.chain.ready;
    let body = AuthorityReadinessResponse {
        ready,
        network_id: &config.network_id,
        protocol_network_id: config.protocol_network_id,
        wire_version: &config.wire_version,
    };
    let mut response = json(if ready { 200 } else { 503 }, &serde_json::json!(body));
    let category = |dependency: DependencyReadiness| {
        if dependency.ready {
            "ready"
        } else if dependency.reason == DependencyReason::ConnectionLimit {
            "unprobed"
        } else {
            "unavailable"
        }
    };
    response.readiness_headers =
        Some([category(dependencies.replica), category(dependencies.chain)]);
    if !ready {
        response.retry_after = Some(5);
    }
    response
}

fn route(config: &Arc<Config>, request: &Request) -> Response {
    if request.method != "GET" {
        return refusal(405, "method_not_allowed", None);
    }
    let path = request.path.as_str();
    if path == "/livez" {
        return json(200, &serde_json::json!({ "live": true }));
    }
    if path == "/readyz" {
        return readiness(config);
    }
    if path.starts_with("/v1/agent/") {
        return human::route(config, request);
    }
    if let Some(batch_id) = path
        .strip_prefix("/v1/batches/")
        .and_then(|rest| rest.strip_suffix("/receipt-authority"))
    {
        return match authenticate(config, request) {
            Ok(()) => relay(config, batch_id, request.query.as_deref()),
            Err(_) if evidence_reader(config, request) => {
                relay(config, batch_id, request.query.as_deref())
            }
            Err(response) => response,
        };
    }
    if let Some((status, document)) = committed_head_route::route(
        &request.method,
        path,
        request.query.as_deref(),
        || authenticate(config, request).is_ok(),
        config.replica_id,
        |target| match replica_get(config, target) {
            ReplicaAnswer::Status(200, body) => {
                serde_json::from_slice(&body).map_err(|error| error.to_string())
            }
            _ => Err("independent committed head is unavailable".to_owned()),
        },
    ) {
        return json(status, &document);
    }
    if request.query.is_some() {
        return refusal(404, "not_found", None);
    }
    let activity = path
        .strip_prefix("/v1/authorized-batches/by-activity/")
        .map(|activity| (activity, false))
        .or_else(|| {
            path.strip_prefix("/v1/authorized-batches/wait-by-activity/")
                .map(|activity| (activity, true))
        })
        .or_else(|| {
            path.strip_prefix("/internal/v1/activities/")
                .and_then(|rest| rest.strip_suffix("/authority"))
                .map(|activity| (activity, false))
        });
    match activity {
        Some((activity, wait_publication)) => match authenticate(config, request) {
            Ok(()) => by_activity(config, activity, wait_publication),
            Err(response) => response,
        },
        None => refusal(404, "not_found", None),
    }
}

fn handle_connection(
    config: &Arc<Config>,
    tcp: TcpStream,
    shutdown: &AtomicBool,
) -> Result<(), String> {
    tcp.set_nodelay(true).map_err(|error| error.to_string())?;
    tcp.set_read_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    tcp.set_write_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    let connection =
        ServerConnection::new(Arc::clone(&config.tls)).map_err(|error| error.to_string())?;
    let mut stream = StreamOwned::new(connection, tcp);
    for request_number in 0..MAX_REQUESTS_PER_CONNECTION {
        if shutdown.load(Ordering::Acquire) {
            break;
        }
        let request = match read_http_message(&mut stream) {
            Ok(request) => request,
            Err(_) if request_number == 0 => {
                write_response(&mut stream, &refusal(400, "invalid_request", None), false)?;
                break;
            }
            Err(_) => break,
        };
        let keep_alive = !shutdown.load(Ordering::Acquire)
            && request_number + 1 < MAX_REQUESTS_PER_CONNECTION
            && request
                .headers
                .get("connection")
                .is_none_or(|value| !value.eq_ignore_ascii_case("close"));
        write_response(&mut stream, &route(config, &request), keep_alive)?;
        if !keep_alive {
            break;
        }
    }
    stream.conn.send_close_notify();
    let _ = stream.conn.write_tls(&mut stream.sock);
    Ok(())
}

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

fn serve(config: Config) -> Result<(), String> {
    let shutdown = Arc::new(AtomicBool::new(false));
    signal_hook::flag::register(signal_hook::consts::SIGTERM, Arc::clone(&shutdown))
        .map_err(|error| error.to_string())?;
    signal_hook::flag::register(signal_hook::consts::SIGINT, Arc::clone(&shutdown))
        .map_err(|error| error.to_string())?;
    let listener = TcpListener::bind(config.listen).map_err(|error| error.to_string())?;
    listener
        .set_nonblocking(true)
        .map_err(|error| error.to_string())?;
    let config = Arc::new(config);
    let mut workers = Vec::<thread::JoinHandle<()>>::new();
    eprintln!(
        "layerx-receipt-authority listening with TLS on {}",
        config.listen
    );
    while !shutdown.load(Ordering::Acquire) {
        workers.retain(|worker| !worker.is_finished());
        match listener.accept() {
            Ok((stream, _)) => {
                let Some(permit) = ConnectionPermit::acquire() else {
                    continue;
                };
                let shared = Arc::clone(&config);
                let stopping = Arc::clone(&shutdown);
                workers.push(thread::spawn(move || {
                    let _permit = permit;
                    if let Err(error) = handle_connection(&shared, stream, &stopping) {
                        eprintln!("layerx-receipt-authority connection failed: {error}");
                    }
                }));
            }
            Err(error) => {
                if error.kind() != std::io::ErrorKind::WouldBlock
                    && error.kind() != std::io::ErrorKind::Interrupted
                {
                    eprintln!("layerx-receipt-authority accept failed: {error}");
                }
                thread::sleep(Duration::from_millis(25));
            }
        }
    }
    drop(listener);
    for worker in workers {
        worker
            .join()
            .map_err(|_| "receipt authority connection worker panicked".to_owned())?;
    }
    Ok(())
}

fn main() {
    let arguments: Vec<String> = env::args().skip(1).collect();
    if arguments
        .iter()
        .any(|argument| argument == "--help" || argument == "-h")
    {
        print!("{USAGE}");
        return;
    }
    if !arguments.is_empty() {
        eprint!("layerx-receipt-authority accepts no arguments\n\n{USAGE}");
        std::process::exit(2);
    }
    if let Err(error) = config().and_then(serve) {
        eprintln!("layerx-receipt-authority: {error}");
        std::process::exit(2);
    }
}

#[cfg(test)]
mod chain_readiness_tests {
    use super::*;
    use layerx_platform_authority::assertion::Must;
    use std::io::BufReader;
    use std::path::Path;
    use std::sync::Mutex;

    pub(super) fn must<T, E: std::fmt::Debug>(value: Result<T, E>) -> T {
        value.unwrap_or_else(|error| panic!("{error:?}"))
    }

    pub(super) fn receipt_vectors() -> serde_json::Value {
        let root = Path::new(env!("CARGO_MANIFEST_DIR"))
            .ancestors()
            .nth(4)
            .must();
        must(serde_json::from_slice(&must(fs::read(root.join(
            "sdk/platform/conformance/fixtures/sequencer-receipt-v1.json",
        )))))
    }

    pub(super) fn vector(document: &serde_json::Value, id: &str) -> serde_json::Value {
        document["cases"]
            .as_array()
            .must()
            .iter()
            .find(|case| case["id"] == id)
            .must()
            .clone()
    }

    pub(super) fn hex_bytes(value: &serde_json::Value) -> Vec<u8> {
        must(hex::decode(value.as_str().must().strip_prefix("0x").must()))
    }

    pub(super) fn base64(bytes: &[u8]) -> String {
        const ALPHABET: &[u8] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
        let mut text = String::new();
        for chunk in bytes.chunks(3) {
            let a = chunk[0];
            let b = chunk.get(1).copied().unwrap_or(0);
            let c = chunk.get(2).copied().unwrap_or(0);
            text.push(char::from(ALPHABET[usize::from(a >> 2)]));
            text.push(char::from(ALPHABET[usize::from(((a & 3) << 4) | (b >> 4))]));
            text.push(if chunk.len() > 1 {
                char::from(ALPHABET[usize::from(((b & 15) << 2) | (c >> 6))])
            } else {
                '='
            });
            text.push(if chunk.len() > 2 {
                char::from(ALPHABET[usize::from(c & 63)])
            } else {
                '='
            });
        }
        text
    }

    pub(super) struct ReceiptHarness {
        pub(super) directory: PathBuf,
        pub(super) config: Config,
        pub(super) calls: Arc<Mutex<Vec<serde_json::Value>>>,
        pub(super) stopped: Arc<AtomicBool>,
        pub(super) server: Option<thread::JoinHandle<()>>,
    }

    impl ReceiptHarness {
        pub(super) fn start(cases: &[serde_json::Value], pinned: &serde_json::Value) -> Self {
            Self::start_with_pins(cases, std::slice::from_ref(pinned))
        }

        pub(super) fn start_with_pins(
            cases: &[serde_json::Value],
            pinned: &[serde_json::Value],
        ) -> Self {
            Self::start_with_chain_id(cases, pinned, 125)
        }

        fn serve_chain(
            listener: &TcpListener,
            stop: &AtomicBool,
            seen: &Mutex<Vec<serde_json::Value>>,
            recorded: &BTreeMap<String, serde_json::Value>,
            chain_id: u64,
        ) {
            while !stop.load(Ordering::Acquire) {
                let (stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                        continue;
                    }
                    Err(error) => panic!("recorded chain listener: {error}"),
                };
                must(stream.set_read_timeout(Some(Duration::from_secs(2))));
                let mut reader = BufReader::new(stream);
                let mut line = String::new();
                must(std::io::BufRead::read_line(&mut reader, &mut line));
                let replica_probe = line.starts_with("GET /v1/batches/");
                if !replica_probe {
                    assert!(matches!(
                        line.as_str(),
                        "POST /evm HTTP/1.0\r\n" | "POST /evm HTTP/1.1\r\n"
                    ));
                }
                let mut length = None;
                loop {
                    line.clear();
                    assert_ne!(must(std::io::BufRead::read_line(&mut reader, &mut line)), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some(value) = line.to_ascii_lowercase().strip_prefix("content-length:") {
                        length = Some(must(value.trim().parse::<usize>()));
                    }
                }
                if replica_probe {
                    let mut stream = reader.into_inner();
                    must(write!(
                        stream,
                        "HTTP/1.1 404 Not Found\r\nContent-Length: 0\r\nConnection: close\r\n\r\n"
                    ));
                    continue;
                }
                let length = length.must();
                assert!(length <= 1024);
                let mut bytes = vec![0; length];
                must(reader.read_exact(&mut bytes));
                let request: serde_json::Value = must(serde_json::from_slice(&bytes));
                assert_eq!(request["jsonrpc"], "2.0");
                let result = match request["method"].as_str().must() {
                    "eth_chainId" => {
                        assert_eq!(request["params"], serde_json::json!([]));
                        serde_json::json!(format!("0x{chain_id:x}"))
                    }
                    "pax_sequencerReceipt" => {
                        assert_eq!(request["params"].as_array().must().len(), 1);
                        let hash = request["params"][0].as_str().must();
                        recorded
                            .get(hash)
                            .cloned()
                            .unwrap_or(serde_json::Value::Null)
                    }
                    method => panic!("unexpected chain method: {method}"),
                };
                let answer =
                    serde_json::json!({"jsonrpc": "2.0", "id": request["id"], "result": result})
                        .to_string();
                must(seen.lock()).push(request);
                let mut stream = reader.into_inner();
                must(write!(
                stream,
                "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}",
                answer.len(),
                answer
            ));
            }
        }

        pub(super) fn start_with_chain_id(
            cases: &[serde_json::Value],
            pinned: &[serde_json::Value],
            chain_id: u64,
        ) -> Self {
            let mut nonce = [0; 16];
            must(getrandom::fill(&mut nonce));
            let directory = std::env::temp_dir()
                .join(format!("authority-chain-receipts-{}", hex::encode(&nonce)));
            must(fs::create_dir(&directory));
            must(fs::set_permissions(
                &directory,
                <fs::Permissions as std::os::unix::fs::PermissionsExt>::from_mode(0o700),
            ));
            let keys_file = directory.join("receipt-keys.json");
            let pin = &pinned[0]["pinnedKey"];
            let keys: Vec<_> = pinned
                .iter()
                .map(|case| {
                    let pin = &case["pinnedKey"];
                    serde_json::json!({
                        "keyId": base64(&hex_bytes(&pin["keyId"])),
                        "publicKey": base64(&hex_bytes(&pin["publicKey"])),
                        "firstHeight": pin["firstHeight"], "lastHeight": pin["lastHeight"],
                    })
                })
                .collect();
            let keys = serde_json::json!(keys);
            must(fs::write(&keys_file, keys.to_string()));
            must(fs::set_permissions(
                &keys_file,
                <fs::Permissions as std::os::unix::fs::PermissionsExt>::from_mode(0o600),
            ));

            let listener = must(TcpListener::bind("127.0.0.1:0"));
            must(listener.set_nonblocking(true));
            let address = must(listener.local_addr());
            let mut recorded = BTreeMap::new();
            for case in cases {
                let mut result = case["receiptJson"].clone();
                result["signingKey"] = case["pinnedKey"]["publicKey"].clone();
                recorded.insert(result["txHash"].as_str().must().to_owned(), result);
            }
            let calls = Arc::new(Mutex::new(Vec::new()));
            let seen = Arc::clone(&calls);
            let stopped = Arc::new(AtomicBool::new(false));
            let stop = Arc::clone(&stopped);
            let server = thread::spawn(move || {
                Self::serve_chain(&listener, &stop, &seen, &recorded, chain_id);
            });

            let tls_key_file = directory.join("tls-key.der");
            let cert_file = directory.join("tls-cert.der");
            must(fs::write(
                &cert_file,
                include_bytes!("human/fixtures/server.der"),
            ));
            must(fs::write(
                &tls_key_file,
                include_bytes!("human/fixtures/server-key.der"),
            ));
            let _ = rustls::crypto::ring::default_provider().install_default();
            let tls = must(
                ServerConfig::builder()
                    .with_no_client_auth()
                    .with_single_cert(
                        vec![CertificateDer::from(must(fs::read(cert_file)))],
                        must(PrivateKeyDer::try_from(must(fs::read(tls_key_file)))),
                    ),
            );
            let public_key: [u8; 32] = hex_bytes(&pin["publicKey"]).try_into().must();
            let config = Config {
                listen: "127.0.0.1:0".parse().must(),
                tls: Arc::new(tls),
                tokens: vec![Zeroizing::new("chain-receipts-test-token".to_owned())],
                human: None,
                replica_address: address,
                replica_host: address.to_string(),
                replica_token: Zeroizing::new("replica-test-token".to_owned()),
                evidence_read_token: None,
                replica_id: [1; 32],
                chain_receipts: must(ChainReceipts::load(
                    &format!("http://{address}/evm"),
                    &keys_file,
                )),
                chain_rpc_url: format!("http://{address}/evm"),
                protocol_network_id: 125,
                network_id: "chain-receipts-test".to_owned(),
                wire_version: PROTOCOL_VERSION.to_string(),
                authorization: SequencerAuthorization::new(
                    hex_bytes(&pin["keyId"]).try_into().must(),
                    public_key,
                    pin["firstHeight"].as_u64().must(),
                    pin["lastHeight"].as_u64().must(),
                ),
                sequencer_public_key: public_key,
            };
            Self {
                directory,
                config,
                calls,
                stopped,
                server: Some(server),
            }
        }

        pub(super) fn stop_chain(&mut self) {
            self.stopped.store(true, Ordering::Release);
            if let Some(server) = self.server.take() {
                server.join().must();
            }
        }
    }

    impl Drop for ReceiptHarness {
        fn drop(&mut self) {
            self.stop_chain();
            must(fs::remove_dir_all(&self.directory));
        }
    }

    #[test]
    pub(super) fn actual_node_info_refuses_incompatible_receipt_admission() {
        let document = receipt_vectors();
        let case = vector(&document, "V1-pax-transfer");
        let harness = ReceiptHarness::start(std::slice::from_ref(&case), &case);
        assert_eq!(
            chain_answers(&harness.config, Instant::now() + READINESS_TIMEOUT),
            Ok(DependencyReason::Ready)
        );
        assert!(!must(harness.calls.lock()).is_empty());
        let incompatible = ReceiptHarness::start_with_chain_id(
            std::slice::from_ref(&case),
            std::slice::from_ref(&case),
            126,
        );
        assert_eq!(
            chain_answers(&incompatible.config, Instant::now() + READINESS_TIMEOUT),
            Err(DependencyReason::IdentityMismatch)
        );
    }
}
