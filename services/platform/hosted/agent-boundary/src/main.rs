mod artifacts;
mod chain_receipt;
mod deployment;
#[cfg(test)]
mod lifecycle_tests;

use crate::chain_receipt::ChainReceipts;
use layerx_proof::receipt::verify_sequencer_signature;
use layerx_types::result::{ResultCode, Retriability};
use layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION as PROTOCOL_VERSION;
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::server::WebPkiClientVerifier;
use rustls::{RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::PathBuf;
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::Arc;
use std::thread;
use std::time::Duration;
use subtle::ConstantTimeEq;
use zeroize::{Zeroize, Zeroizing};

const SERVICE: &str = "agent-boundary";
const MAX_ACTIVITY_BYTES: usize = 1_048_576;
const MAX_REQUEST_BYTES: usize = MAX_ACTIVITY_BYTES + 16 * 1024;
const CONNECT_TIMEOUT: Duration = Duration::from_secs(3);
const IO_TIMEOUT: Duration = Duration::from_secs(15);
const MAX_CONNECTIONS: usize = 128;
static ACTIVE_CONNECTIONS: AtomicUsize = AtomicUsize::new(0);

struct Config {
    listen: SocketAddr,
    tls: Arc<ServerConfig>,
    gateway_token: Zeroizing<String>,
    registry_token: Zeroizing<String>,
    webhook_token: Zeroizing<String>,
    chain_receipts: ChainReceipts,
    network_name: String,
}

#[derive(Clone, Copy, Eq, PartialEq)]
enum Plane {
    Gateway,
    Registry,
    Webhook,
}

#[derive(Clone, Copy, Eq, PartialEq)]
enum Route {
    Activities,
    ProgramCall,
    ProgramDeploy,
    ProgramUpgrade,
    ProgramWindDown,
}

impl Route {
    const fn name(self) -> &'static str {
        match self {
            Self::Activities => "activities",
            Self::ProgramCall => "programs_call",
            Self::ProgramDeploy => "programs_deploy",
            Self::ProgramUpgrade => "programs_upgrade",
            Self::ProgramWindDown => "programs_wind_down",
        }
    }

    const fn success_state(self) -> &'static str {
        match self {
            Self::Activities => "completed",
            Self::ProgramCall
            | Self::ProgramDeploy
            | Self::ProgramUpgrade
            | Self::ProgramWindDown => "executed",
        }
    }

    const fn program_ordinal(self) -> Option<u16> {
        match self {
            Self::Activities => None,
            Self::ProgramDeploy => Some(1),
            Self::ProgramUpgrade => Some(2),
            Self::ProgramCall => Some(3),
            Self::ProgramWindDown => Some(7),
        }
    }
}

#[derive(Clone, Serialize, Deserialize, PartialEq, Eq)]
struct Refusal {
    status: u16,
    code: String,
    retry_after: Option<u64>,
}

#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct JournalRecord {
    idempotency_key: String,
    route: String,
    request_digest: String,
    activity_id: String,
    program_id: Option<String>,
    signed_activity: String,
    state: String,
    attempts: u32,
    refusal: Option<Refusal>,
    receipt: Option<String>,
    result_code: Option<i32>,
    #[serde(default)]
    program_execution: Option<artifacts::StoredExecution>,
    #[serde(default)]
    lifecycle_sequencer_key: Option<String>,
    #[serde(default)]
    receipt_sequencer_key: Option<String>,
}

struct Request {
    method: String,
    path: String,
    headers: BTreeMap<String, String>,
    body: Vec<u8>,
}

impl Drop for Request {
    fn drop(&mut self) {
        for value in self.headers.values_mut() {
            value.zeroize();
        }
        self.body.zeroize();
    }
}

struct Response {
    status: u16,
    body: String,
    retry_after: Option<u64>,
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 15)]));
    }
    text
}

fn decode_hex(text: &str, maximum: usize) -> Result<Vec<u8>, String> {
    if !text.len().is_multiple_of(2) || text.len() / 2 > maximum {
        return Err("hex text has an invalid length".to_owned());
    }
    let mut bytes = Vec::with_capacity(text.len() / 2);
    let digits = text.as_bytes();
    for pair in digits.chunks(2) {
        let text = std::str::from_utf8(pair).map_err(|_| "hex text is not ASCII".to_owned())?;
        bytes.push(u8::from_str_radix(text, 16).map_err(|_| "hex digit is invalid".to_owned())?);
    }
    Ok(bytes)
}

fn is_hex32(text: &str) -> bool {
    text.len() == 64 && text.bytes().all(|byte| byte.is_ascii_hexdigit())
}

fn parse_hex32(text: &str) -> Option<[u8; 32]> {
    if !is_hex32(text) {
        return None;
    }
    let bytes = decode_hex(text, 32).ok()?;
    bytes.try_into().ok()
}

fn sha256_hex(bytes: &[u8]) -> String {
    format!("{:x}", Sha256::digest(bytes))
}

fn valid_identifier(value: &str, max: usize) -> bool {
    !value.is_empty()
        && value.len() <= max
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.' | b':'))
}

fn snake_case(name: &str) -> String {
    let mut text = String::with_capacity(name.len() + 8);
    for (index, character) in name.chars().enumerate() {
        if character.is_ascii_uppercase() {
            if index != 0 {
                text.push('_');
            }
            text.push(character.to_ascii_lowercase());
        } else {
            text.push(character);
        }
    }
    text
}

fn read_secret(path_variable: &str) -> Result<Zeroizing<String>, String> {
    let path = env::var(path_variable).map_err(|_| format!("{path_variable} is required"))?;
    let mut value = fs::read_to_string(path).map_err(|error| error.to_string())?;
    while matches!(value.as_bytes().last(), Some(b'\n' | b'\r')) {
        value.pop();
    }
    if value.is_empty() || value.len() > 4096 {
        value.zeroize();
        return Err(format!("{path_variable} does not contain a bounded secret"));
    }
    Ok(Zeroizing::new(value))
}

fn server_tls_config() -> Result<Arc<ServerConfig>, String> {
    rustls::crypto::ring::default_provider()
        .install_default()
        .map_err(|_| "failed to install TLS crypto provider".to_owned())?;
    let certificate_path = env::var("LAYERX_AGENT_BOUNDARY_TLS_CERT_DER")
        .map_err(|_| "LAYERX_AGENT_BOUNDARY_TLS_CERT_DER is required")?;
    let key_path = env::var("LAYERX_AGENT_BOUNDARY_TLS_KEY_DER")
        .map_err(|_| "LAYERX_AGENT_BOUNDARY_TLS_KEY_DER is required")?;
    let certificate = CertificateDer::from(fs::read(certificate_path).map_err(|e| e.to_string())?);
    let key = PrivateKeyDer::from(PrivatePkcs8KeyDer::from(
        fs::read(key_path).map_err(|e| e.to_string())?,
    ));
    let builder = ServerConfig::builder();
    let config = match env::var("LAYERX_AGENT_BOUNDARY_CLIENT_CA_DER") {
        Ok(client_ca_path) => {
            let client_ca =
                CertificateDer::from(fs::read(client_ca_path).map_err(|e| e.to_string())?);
            let mut roots = RootCertStore::empty();
            roots
                .add(client_ca)
                .map_err(|_| "client CA certificate is invalid".to_owned())?;
            let verifier = WebPkiClientVerifier::builder(roots.into())
                .allow_unauthenticated()
                .build()
                .map_err(|error| error.to_string())?;
            builder
                .with_client_cert_verifier(verifier)
                .with_single_cert(vec![certificate], key)
        }
        Err(_) => builder
            .with_no_client_auth()
            .with_single_cert(vec![certificate], key),
    }
    .map_err(|error| error.to_string())?;
    Ok(Arc::new(config))
}

fn config() -> Result<Config, String> {
    let listen = env::var("LAYERX_AGENT_BOUNDARY_LISTEN")
        .unwrap_or_else(|_| "0.0.0.0:9446".to_owned())
        .parse::<SocketAddr>()
        .map_err(|_| "LAYERX_AGENT_BOUNDARY_LISTEN must be a socket address".to_owned())?;
    let gateway_token = read_secret("LAYERX_AGENT_BOUNDARY_GATEWAY_TOKEN_FILE")?;
    let registry_token = read_secret("LAYERX_AGENT_BOUNDARY_REGISTRY_TOKEN_FILE")?;
    if gateway_token
        .as_bytes()
        .ct_eq(registry_token.as_bytes())
        .unwrap_u8()
        == 1
    {
        return Err("gateway and registry bearer tokens must be distinct".to_owned());
    }
    let webhook_token = read_secret("LAYERX_AGENT_BOUNDARY_WEBHOOK_TOKEN_FILE")?;
    if [gateway_token.as_str(), registry_token.as_str()]
        .iter()
        .any(|token| token.as_bytes().ct_eq(webhook_token.as_bytes()).unwrap_u8() == 1)
    {
        return Err("webhook bearer token must be distinct from gateway and registry".to_owned());
    }
    let network_name = env::var("LAYERX_AGENT_BOUNDARY_NETWORK_ID")
        .map_err(|_| "LAYERX_AGENT_BOUNDARY_NETWORK_ID is required".to_owned())?;
    if !valid_identifier(&network_name, 64) {
        return Err("LAYERX_AGENT_BOUNDARY_NETWORK_ID is not a canonical identifier".to_owned());
    }
    Ok(Config {
        listen,
        tls: server_tls_config()?,
        gateway_token,
        registry_token,
        webhook_token,
        chain_receipts: ChainReceipts::from_config(
            &env::var("LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL")
                .map_err(|_| "LAYERX_AGENT_BOUNDARY_CHAIN_RPC_URL is required")?,
            &PathBuf::from(
                env::var("LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE")
                    .map_err(|_| "LAYERX_AGENT_BOUNDARY_RECEIPT_KEYS_FILE is required")?,
            ),
        )?,
        network_name,
    })
}

fn result_refusal(result: ResultCode) -> Refusal {
    let code = result.known().map_or_else(
        || format!("protocol_result_{}", result.raw().unsigned_abs()),
        |known| snake_case(&format!("{known:?}")),
    );
    match result.retriability() {
        Retriability::Retriable => Refusal {
            status: 409,
            code,
            retry_after: Some(5),
        },
        Retriability::Terminal => Refusal {
            status: 422,
            code,
            retry_after: None,
        },
    }
}

fn terminal(status: u16, code: &str) -> Refusal {
    Refusal {
        status,
        code: code.to_owned(),
        retry_after: None,
    }
}

struct ReceiptMetadata {
    result_code: i32,
    state_version: u64,
    state_root: [u8; 32],
}

fn verified_receipt_metadata(record: &JournalRecord) -> Result<ReceiptMetadata, String> {
    let key = record
        .receipt_sequencer_key
        .as_deref()
        .or(record.lifecycle_sequencer_key.as_deref())
        .or_else(|| {
            record
                .program_execution
                .as_ref()
                .map(|execution| execution.sequencer_public_key.as_str())
        })
        .and_then(parse_hex32)
        .ok_or_else(|| "missing receipt sequencer key".to_owned())?;
    let receipt = artifacts::canonical_hex(
        record.receipt.as_deref().unwrap_or_default(),
        MAX_ACTIVITY_BYTES,
    )?;
    let verified =
        verify_sequencer_signature(&receipt, key).map_err(|error| format!("{error:?}"))?;
    let protocol = verified
        .protocol()
        .ok_or_else(|| "missing protocol receipt".to_owned())?;
    if hex(&protocol.activity_id()) != record.activity_id
        || Some(protocol.result_code()) != record.result_code
        || protocol.protocol_version() != PROTOCOL_VERSION
    {
        return Err("receipt journal binding mismatch".to_owned());
    }
    Ok(ReceiptMetadata {
        result_code: protocol.result_code(),
        state_version: protocol.global_sequence(),
        state_root: protocol.resulting_state_root(),
    })
}

fn outcome_response(record: &JournalRecord) -> Response {
    let metadata = match verified_receipt_metadata(record) {
        Ok(metadata) => metadata,
        Err(detail) => {
            eprintln!("{SERVICE}: {detail}");
            return refusal(503, "receipt_invalid", Some(5));
        }
    };
    let receipt = record.receipt.clone().unwrap_or_default();
    let (terminal_payload, call_graph) =
        record
            .program_execution
            .as_ref()
            .map_or(("", ""), |execution| {
                (
                    execution.terminal_payload.as_str(),
                    execution.call_graph.as_str(),
                )
            });
    let route = match record.route.as_str() {
        "programs_call" => Route::ProgramCall,
        "programs_deploy" => Route::ProgramDeploy,
        "programs_upgrade" => Route::ProgramUpgrade,
        "programs_wind_down" => Route::ProgramWindDown,
        _ => Route::Activities,
    };
    let state = if record.result_code == Some(0) {
        route.success_state()
    } else {
        "refused"
    };
    ok(serde_json::json!({
        "result": {
            "state": state,
            "activity_id": record.activity_id,
            "receipt": receipt,
            "terminal_payload": terminal_payload,
            "call_graph": call_graph,
            "result_code": metadata.result_code,
            "state_version": metadata.state_version.to_string(),
            "state_root": hex(&metadata.state_root),
        }
    })
    .to_string())
}

fn refusal_response(stored: &Refusal) -> Response {
    refusal(stored.status, &stored.code, stored.retry_after)
}

fn unknown_response(activity: &str) -> Response {
    Response {
        status: 202,
        body: format!(
            "{{\"state\":\"unknown\",\"activity_id\":\"{activity}\",\"retry\":\"after\",\"retry_after_seconds\":2}}"
        ),
        retry_after: Some(2),
    }
}

struct ProgramReadFreshness {
    minimum_sequence: u64,
    expected_state_root: Option<[u8; 32]>,
}

fn program_read_freshness(request: &Request) -> Result<ProgramReadFreshness, Response> {
    let minimum_sequence =
        request
            .headers
            .get("layerx-minimum-sequence")
            .map_or(Ok(0), |value| {
                if value.is_empty()
                    || (value.len() > 1 && value.starts_with('0'))
                    || !value.bytes().all(|byte| byte.is_ascii_digit())
                {
                    return Err(refusal(400, "invalid_minimum_sequence", None));
                }
                value
                    .parse::<u64>()
                    .map_err(|_| refusal(400, "invalid_minimum_sequence", None))
            })?;
    let expected_state_root = request
        .headers
        .get("layerx-expected-state-root")
        .map(|value| {
            if value.bytes().any(|byte| byte.is_ascii_uppercase()) {
                return Err(refusal(400, "invalid_expected_state_root", None));
            }
            parse_hex32(value)
                .filter(|root| *root != [0; 32])
                .ok_or_else(|| refusal(400, "invalid_expected_state_root", None))
        })
        .transpose()?;
    Ok(ProgramReadFreshness {
        minimum_sequence,
        expected_state_root,
    })
}

fn receipt_route(config: &Config, transaction_text: &str, plane: Plane) -> Response {
    let Some(transaction) = parse_hex32(
        transaction_text
            .strip_prefix("0x")
            .unwrap_or(transaction_text),
    ) else {
        return refusal(400, "invalid_activity_id", None);
    };
    match config.chain_receipts.lookup(transaction) {
        Ok(Some(receipt)) => {
            let body =
                serde_json::json!({"activity_id": hex(&transaction), "receipt": hex(&receipt)});
            if plane == Plane::Gateway {
                ok(serde_json::json!({"result": body}).to_string())
            } else {
                ok(body.to_string())
            }
        }
        Ok(None) => refusal(404, "receipt_not_found", None),
        Err(_) => refusal(503, "node_unavailable", Some(5)),
    }
}

fn readiness(config: &Config) -> Response {
    let probe = Sha256::digest(b"LayerX-AgentBoundary-Readiness/v1").into();
    if config.chain_receipts.lookup(probe).is_err() {
        return refusal(503, "node_unavailable", Some(5));
    }
    ok(serde_json::json!({
        "ready": true,
        "network_id": config.network_name,
        "chain_receipts": true,
    })
    .to_string())
}

fn authenticate(config: &Config, request: &Request) -> Result<Plane, Response> {
    let Some(token) = request
        .headers
        .get("authorization")
        .and_then(|value| value.strip_prefix("Bearer "))
    else {
        return Err(refusal(401, "identity_required", None));
    };
    if token.is_empty() || token.len() > 4096 {
        return Err(refusal(401, "identity_required", None));
    }
    if token
        .as_bytes()
        .ct_eq(config.gateway_token.as_bytes())
        .unwrap_u8()
        == 1
    {
        return Ok(Plane::Gateway);
    }
    if token
        .as_bytes()
        .ct_eq(config.registry_token.as_bytes())
        .unwrap_u8()
        == 1
    {
        return Ok(Plane::Registry);
    }
    if token
        .as_bytes()
        .ct_eq(config.webhook_token.as_bytes())
        .unwrap_u8()
        == 1
    {
        return Ok(Plane::Webhook);
    }
    Err(refusal(401, "identity_required", None))
}

fn route(config: &Config, request: &Request) -> Response {
    let (path, query) = request
        .path
        .split_once('?')
        .map_or((request.path.as_str(), None), |(path, query)| {
            (path, Some(query))
        });
    if request.method == "GET" && path == "/livez" && query.is_none() {
        return ok(format!("{{\"status\":\"live\",\"service\":\"{SERVICE}\"}}"));
    }
    if request.method == "GET" && path == "/readyz" && query.is_none() {
        return readiness(config);
    }
    let plane = match authenticate(config, request) {
        Ok(plane) => plane,
        Err(response) => return response,
    };
    if let Some(response) = deployment::route(config, request, path, query, plane) {
        return response;
    }
    if let Some(activity) = path.strip_prefix("/internal/v1/receipts/") {
        if !matches!(plane, Plane::Registry | Plane::Webhook) {
            return refusal(403, "entitlement_denied", None);
        }
        if request.method != "GET" || query.is_some() {
            return refusal(404, "not_found", None);
        }
        return receipt_route(config, activity, plane);
    }
    if !path.starts_with("/v1/") || query.is_some() {
        return refusal(404, "not_found", None);
    }
    let Some(activity) = path
        .strip_prefix("/v1/receipts/")
        .filter(|activity| !activity.contains('/'))
    else {
        return refusal(404, "not_found", None);
    };
    if plane != Plane::Gateway {
        return refusal(403, "entitlement_denied", None);
    }
    match request.method.as_str() {
        "GET" => receipt_route(config, activity, plane),
        _ => refusal(404, "not_found", None),
    }
}

fn ok(body: String) -> Response {
    Response {
        status: 200,
        body,
        retry_after: None,
    }
}

fn refusal(status: u16, code: &str, retry_after: Option<u64>) -> Response {
    let retry = if retry_after.is_some() {
        "after"
    } else {
        "never"
    };
    let body = retry_after.map_or_else(
        || serde_json::json!({ "error": { "code": code, "retry": retry } }),
        |seconds| serde_json::json!({ "error": { "code": code, "retry": retry, "retry_after_seconds": seconds } }),
    );
    Response {
        status,
        body: body.to_string(),
        retry_after,
    }
}

fn read_http_message(stream: &mut impl Read, maximum: usize) -> Result<Request, String> {
    let mut bytes = Vec::with_capacity(2048);
    let mut chunk = [0_u8; 8192];
    let header_end = loop {
        let count = stream.read(&mut chunk).map_err(|error| error.to_string())?;
        if count == 0 || bytes.len().saturating_add(count) > maximum {
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
    let first = lines
        .next()
        .ok_or_else(|| "HTTP start line is missing".to_owned())?
        .to_owned();
    let mut headers = BTreeMap::new();
    headers.insert(String::new(), first);
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
    if header_end.saturating_add(content_length) > maximum {
        return Err("HTTP body exceeds its bound".to_owned());
    }
    while bytes.len() < header_end + content_length {
        let count = stream.read(&mut chunk).map_err(|error| error.to_string())?;
        if count == 0 || bytes.len().saturating_add(count) > maximum {
            return Err("HTTP body is truncated or exceeds its bound".to_owned());
        }
        bytes.extend_from_slice(&chunk[..count]);
    }
    Ok(Request {
        method: String::new(),
        path: String::new(),
        headers,
        body: bytes[header_end..header_end + content_length].to_vec(),
    })
}

fn parse_client_request(stream: &mut impl Read) -> Result<Request, String> {
    let mut request = read_http_message(stream, MAX_REQUEST_BYTES)?;
    let start = request
        .headers
        .remove("")
        .ok_or_else(|| "request line is missing".to_owned())?;
    let mut parts = start.split_whitespace();
    parts
        .next()
        .ok_or_else(|| "request method is missing".to_owned())?
        .clone_into(&mut request.method);
    parts
        .next()
        .ok_or_else(|| "request target is missing".to_owned())?
        .clone_into(&mut request.path);
    if parts.next() != Some("HTTP/1.1")
        || parts.next().is_some()
        || !request.path.starts_with('/')
        || request.path.contains(['#', '\\', ' '])
    {
        return Err("request line is invalid".to_owned());
    }
    if !request.headers.contains_key("host") {
        return Err("HTTP/1.1 Host header is required".to_owned());
    }
    Ok(request)
}

fn write_response(stream: &mut impl Write, response: &Response) -> Result<(), String> {
    let reason = match response.status {
        200 => "OK",
        202 => "Accepted",
        400 => "Bad Request",
        401 => "Unauthorized",
        403 => "Forbidden",
        404 => "Not Found",
        409 => "Conflict",
        422 => "Unprocessable Content",
        429 => "Too Many Requests",
        502 => "Bad Gateway",
        _ => "Service Unavailable",
    };
    let retry = response.retry_after.map_or(String::new(), |seconds| {
        format!("Retry-After: {seconds}\r\n")
    });
    write!(
        stream,
        "HTTP/1.1 {} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nCache-Control: no-store\r\n{retry}Connection: close\r\n\r\n{}",
        response.status,
        response.body.len(),
        response.body
    )
    .map_err(|error| error.to_string())
}

fn handle_connection(config: &Arc<Config>, tcp: TcpStream) -> Result<(), String> {
    tcp.set_read_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    tcp.set_write_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    let connection = ServerConnection::new(Arc::clone(&config.tls)).map_err(|e| e.to_string())?;
    let mut stream = StreamOwned::new(connection, tcp);
    let response = parse_client_request(&mut stream).map_or_else(
        |_| refusal(400, "invalid_request", None),
        |request| route(config, &request),
    );
    write_response(&mut stream, &response)?;
    stream.flush().map_err(|error| error.to_string())
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
    let listener = TcpListener::bind(config.listen).map_err(|error| error.to_string())?;
    let config = Arc::new(config);
    eprintln!("layerx-agent-boundary listening with TLS");
    for connection in listener.incoming() {
        match connection {
            Ok(stream) => {
                let Some(permit) = ConnectionPermit::acquire() else {
                    continue;
                };
                let shared = Arc::clone(&config);
                thread::spawn(move || {
                    let _permit = permit;
                    if let Err(error) = handle_connection(&shared, stream) {
                        eprintln!("layerx-agent-boundary connection failed: {error}");
                    }
                });
            }
            Err(error) => eprintln!("layerx-agent-boundary accept failed: {error}"),
        }
    }
    Ok(())
}

fn main() {
    if let Err(error) = config().and_then(serve) {
        eprintln!("layerx-agent-boundary: {error}");
        std::process::exit(2);
    }
}
