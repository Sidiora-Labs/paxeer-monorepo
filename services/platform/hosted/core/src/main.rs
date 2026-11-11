mod chain_fund;
mod funding_native;
mod public_reads;

use layerx_client::px::PxEndpoint;
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::disclosure::{AmountRole, CounterpartyRole};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender, transaction_hash};
use layerx_crypto::payments::{bind_call, ledger_payment_call, CallOptions, SendDebit};
use layerx_platform_core::{
    asset_registry, did_for_public_key, fixed_hex, hex_decode, hex_encode, main_account,
    SocketSigner, TreasurySigner as _,
};
use layerx_proof::receipt::{verify_outcome, AuthorizedBatch};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::server::WebPkiClientVerifier;
use rustls::{RootCertStore, ServerConfig, ServerConnection, StreamOwned};
use serde::Deserialize;
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::env;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::os::unix::fs::{DirBuilderExt, MetadataExt, OpenOptionsExt};
use std::os::unix::net::UnixStream;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};
use subtle::ConstantTimeEq;
use zeroize::{Zeroize, Zeroizing};

const MAX_REQUEST_BYTES: usize = 4 * 1024 * 1024;
const MAX_RECEIPT_BYTES: usize = 1_212_416;
const IO_TIMEOUT: Duration = layerx_platform_internal::http::IO_TIMEOUT;
const RESET_TIMEOUT: Duration = Duration::from_secs(180);
const MAX_CONNECTIONS: usize = 128;
const WIRE_VERSION: &str = "3";
const MAX_JOURNAL_BYTES: u64 = 16 * 1024 * 1024;
const MAX_REQUESTS_PER_CONNECTION: usize = 128;

static ACTIVE_CONNECTIONS: AtomicUsize = AtomicUsize::new(0);
static TRACE: AtomicU64 = AtomicU64::new(1);

fn pay_timing(stage: &str, started: Instant) {
    if std::env::var_os("LAYERX_PAY_TIMING").is_some() {
        eprintln!(
            "pay_timing stage={stage} duration_us={}",
            started.elapsed().as_micros()
        );
    }
}

#[derive(Clone, Copy, PartialEq, Eq)]
enum Plane {
    Core,
    Admin,
}

struct Config {
    listen: SocketAddr,
    admin_listen: SocketAddr,
    tls: Arc<ServerConfig>,
    admin_tls: Arc<ServerConfig>,
    sequencer: SequencerEndpoint,
    network: PxEndpoint,
    network_id: u32,
    _node: NodeEndpoint,
    _node_token: Zeroizing<String>,
    receipt_events_token: Option<Zeroizing<String>>,
    _wallet_caps_token: Option<Zeroizing<String>>,
    _replica: NodeEndpoint,
    _replica_token: Zeroizing<String>,
    admin_token: Zeroizing<String>,
    treasury: Arc<dyn layerx_platform_core::TreasurySigner>,
    treasury_did: String,
    treasury_asset: [u8; 32],
    funding: chain_fund::FundingSlot,
    sequencer_id: [u8; 32],
    supervisor_socket: PathBuf,
    state_dir: PathBuf,
    _fee_limit: u128,
    receipt_deadline: Duration,
    admin_lock: Mutex<()>,
    journal_lock: Mutex<()>,
}

#[derive(Clone)]
struct NodeEndpoint {
    host: String,
    port: u16,
}

struct Request {
    method: String,
    path: String,
    query: Option<String>,
    headers: BTreeMap<String, String>,
    body: Vec<u8>,
}

struct Response {
    status: u16,
    body: String,
    retry_after: Option<u64>,
    authoritative_replay: bool,
}

#[derive(Clone, Deserialize, serde::Serialize)]
#[serde(deny_unknown_fields)]
struct FundingCommand {
    funding_id: String,
    did: String,
    public_key: String,
    amount: u64,
}

#[derive(Deserialize, serde::Serialize)]
struct JournalEntry {
    request_digest: String,
    status: u16,
    body: String,
    retry_after: Option<u64>,
}

#[derive(Clone, Deserialize, serde::Serialize)]
#[serde(deny_unknown_fields)]
struct PreparedFunding {
    canonical: Vec<u8>,
    transaction_hash: [u8; 32],
    sender: [u8; 20],
}

#[derive(Deserialize, serde::Serialize)]
#[serde(deny_unknown_fields)]
struct FundingIntent {
    version: u8,
    key: String,
    request_digest: String,
    request_body: Vec<u8>,
    command: FundingCommand,
    network_id: u32,
    asset: [u8; 32],
    sequencer_id: [u8; 32],
    sequencer_key: [u8; 32],
    signed: PreparedFunding,
}

#[derive(Deserialize, serde::Serialize)]
#[serde(deny_unknown_fields)]
struct FundingArchive {
    version: u8,
    intent_digest: String,
    receipt: Vec<u8>,
}

const MAX_FUNDING_RECORDS: usize = 4096;
const MAX_FUNDING_RECORD_BYTES: u64 = MAX_JOURNAL_BYTES;
const MAX_FUNDING_INVENTORY_BYTES: u64 = 64 * 1024 * 1024;

// These are native facts obtained by authenticating sequencer receipt bytes,
// independent of the TypeScript conformance ReceiptFacts fixture input. Portable
// middleware evidence must still cross its receipt decoder and verifier.
struct ReceiptFacts {
    activity_id: [u8; 32],
    batch_id: [u8; 32],
    result_code: i32,
    state_root: [u8; 32],
    canonical: Vec<u8>,
}

fn read_input(variable: &str, path: &str) -> Result<Vec<u8>, String> {
    fs::read(path).map_err(|error| format!("{variable} ({path}): {error}"))
}

fn read_secret(path_variable: &str) -> Result<Zeroizing<String>, String> {
    let path = env::var(path_variable).map_err(|_| format!("{path_variable} is required"))?;
    let mut value =
        fs::read_to_string(&path).map_err(|error| format!("{path_variable} ({path}): {error}"))?;
    while matches!(value.as_bytes().last(), Some(b'\n' | b'\r')) {
        value.pop();
    }
    if value.is_empty() || value.len() > 4096 {
        value.zeroize();
        return Err(format!("{path_variable} does not contain a bounded secret"));
    }
    Ok(Zeroizing::new(value))
}

fn required(name: &str) -> Result<String, String> {
    env::var(name).map_err(|_| format!("{name} is required"))
}

fn parse_listen(name: &str, default: &str) -> Result<SocketAddr, String> {
    env::var(name)
        .unwrap_or_else(|_| default.to_owned())
        .parse::<SocketAddr>()
        .map_err(|_| format!("{name} must be a socket address"))
}

fn parse_u64(name: &str, default: u64) -> Result<u64, String> {
    env::var(name).map_or(Ok(default), |value| {
        value
            .parse::<u64>()
            .map_err(|_| format!("{name} must be an integer"))
    })
}

fn install_provider() -> Result<(), String> {
    if rustls::crypto::CryptoProvider::get_default().is_some() {
        return Ok(());
    }
    rustls::crypto::ring::default_provider()
        .install_default()
        .map_err(|_| "failed to install TLS crypto provider".to_owned())
}

fn server_tls_config(
    certificate_variable: &str,
    key_variable: &str,
    client_ca: Option<&[u8]>,
) -> Result<Arc<ServerConfig>, String> {
    install_provider()?;
    let certificate_path = required(certificate_variable)?;
    let key_path = required(key_variable)?;
    let certificate = CertificateDer::from(read_input(certificate_variable, &certificate_path)?);
    let key = PrivateKeyDer::from(PrivatePkcs8KeyDer::from(read_input(
        key_variable,
        &key_path,
    )?));
    let builder = ServerConfig::builder();
    let config = match client_ca {
        Some(ca) => {
            let mut roots = RootCertStore::empty();
            roots
                .add(CertificateDer::from(ca.to_vec()))
                .map_err(|error| error.to_string())?;
            let verifier = WebPkiClientVerifier::builder(Arc::new(roots))
                .allow_unauthenticated()
                .build()
                .map_err(|error| error.to_string())?;
            builder.with_client_cert_verifier(verifier)
        }
        None => builder.with_no_client_auth(),
    }
    .with_single_cert(vec![certificate], key)
    .map_err(|error| error.to_string())?;
    Ok(Arc::new(config))
}

fn parse_node_url(value: &str) -> Result<NodeEndpoint, String> {
    let rest = value
        .strip_prefix("http://")
        .ok_or_else(|| "LAYERX_CORE_NODE_URL must use plaintext http on loopback".to_owned())?;
    let authority = rest.trim_end_matches('/');
    if authority.contains(['/', '?', '#', '@', '\\']) {
        return Err("LAYERX_CORE_NODE_URL must not carry a path".to_owned());
    }
    let (host, port) = authority
        .rsplit_once(':')
        .ok_or_else(|| "LAYERX_CORE_NODE_URL must carry a port".to_owned())?;
    let port = port
        .parse::<u16>()
        .map_err(|_| "LAYERX_CORE_NODE_URL port is invalid".to_owned())?;
    if host != "127.0.0.1" && host != "localhost" {
        return Err("LAYERX_CORE_NODE_URL must address the loopback listener".to_owned());
    }
    Ok(NodeEndpoint {
        host: host.to_owned(),
        port,
    })
}

fn prepare_state_dir() -> Result<PathBuf, String> {
    let state_dir = PathBuf::from(required("LAYERX_CORE_STATE_DIR")?);
    let mut journal = fs::DirBuilder::new();
    journal.recursive(true).mode(0o700);
    journal
        .create(state_dir.join("journal"))
        .map_err(|error| format!("LAYERX_CORE_STATE_DIR is unusable: {error}"))?;
    for directory in ["funding-intents", "funding-receipts"] {
        journal
            .create(state_dir.join(directory))
            .map_err(|error| format!("LAYERX_CORE_STATE_DIR is unusable: {error}"))?;
    }
    funding_directory(&state_dir)?;
    fs::File::open(&state_dir)
        .and_then(|directory| directory.sync_all())
        .map_err(|error| format!("LAYERX_CORE_STATE_DIR is not durable: {error}"))?;
    Ok(state_dir)
}

fn config() -> Result<Config, String> {
    let client_ca = match env::var("LAYERX_CORE_CLIENT_CA_DER") {
        Ok(path) => Some(read_input("LAYERX_CORE_CLIENT_CA_DER", &path)?),
        Err(_) => None,
    };
    let network_id = required("LAYERX_CORE_NETWORK_ID")?
        .parse::<u32>()
        .map_err(|_| "LAYERX_CORE_NETWORK_ID must be a 32-bit integer".to_owned())?;
    if network_id == 0 {
        return Err("LAYERX_CORE_NETWORK_ID must be non-zero".to_owned());
    }
    let treasury =
        SocketSigner::connect(Path::new(&required("LAYERX_CORE_TREASURY_SIGNER_SOCKET")?))
            .map_err(|error| format!("LAYERX_CORE_TREASURY_SIGNER_SOCKET: {error}"))?;
    let treasury_asset = fixed_hex::<32>(
        "LAYERX_CORE_TREASURY_ASSET",
        &required("LAYERX_CORE_TREASURY_ASSET")?,
    )?;
    if treasury_asset == [0; 32] {
        return Err("LAYERX_CORE_TREASURY_ASSET must be non-zero".to_owned());
    }
    let sequencer_id = fixed_hex::<32>(
        "LAYERX_CORE_SEQUENCER_ID",
        &required("LAYERX_CORE_SEQUENCER_ID")?,
    )?;
    let state_dir = prepare_state_dir()?;
    let sequencer_url = required("LAYERX_CORE_SEQUENCER_ENDPOINT")?;
    let trust_anchor_path = required("LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER")?;
    let mut trust_anchor = Vec::new();
    fs::File::open(&trust_anchor_path)
        .and_then(|file| file.take(1_048_577).read_to_end(&mut trust_anchor))
        .map_err(|error| format!("LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER: {error}"))?;
    if trust_anchor.is_empty() || trust_anchor.len() > 1_048_576 {
        return Err("invalid LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER".to_owned());
    }
    RootCertStore::empty()
        .add(CertificateDer::from(trust_anchor.clone()))
        .map_err(|error| format!("invalid LAYERX_CORE_SEQUENCER_TRUST_ANCHOR_DER: {error}"))?;
    let sequencer = SequencerEndpoint::new(&sequencer_url, &[&trust_anchor])
        .map_err(|error| format!("LayerX sequencer endpoint rejected: {error:?}"))?;
    let network = PxEndpoint::new(&sequencer_url, &[&trust_anchor])
        .map_err(|error| format!("LayerX sequencer network endpoint rejected: {error:?}"))?;
    Ok(Config {
        listen: parse_listen("LAYERX_CORE_LISTEN", "0.0.0.0:9443")?,
        admin_listen: parse_listen("LAYERX_CORE_ADMIN_LISTEN", "0.0.0.0:9444")?,
        tls: server_tls_config(
            "LAYERX_CORE_TLS_CERT_DER",
            "LAYERX_CORE_TLS_KEY_DER",
            client_ca.as_deref(),
        )?,
        admin_tls: server_tls_config(
            "LAYERX_CORE_ADMIN_TLS_CERT_DER",
            "LAYERX_CORE_ADMIN_TLS_KEY_DER",
            None,
        )?,
        sequencer,
        network,
        network_id,
        _node: NodeEndpoint {
            host: String::new(),
            port: 0,
        },
        _node_token: Zeroizing::new(String::new()),
        _wallet_caps_token: env::var_os("LAYERX_CORE_WALLET_CAPS_TOKEN_FILE")
            .map(|_| read_secret("LAYERX_CORE_WALLET_CAPS_TOKEN_FILE"))
            .transpose()?,
        receipt_events_token: env::var_os("LAYERX_CORE_RECEIPT_EVENTS_TOKEN_FILE")
            .map(|_| read_secret("LAYERX_CORE_RECEIPT_EVENTS_TOKEN_FILE"))
            .transpose()?,
        _replica: NodeEndpoint {
            host: String::new(),
            port: 0,
        },
        _replica_token: Zeroizing::new(String::new()),
        admin_token: read_secret("LAYERX_CORE_ADMIN_TOKEN_FILE")?,
        treasury_did: did_for_public_key(&treasury.public_key()),
        treasury: Arc::new(treasury),
        treasury_asset,
        funding: chain_fund::FundingSlot::from_env(),
        sequencer_id,
        supervisor_socket: PathBuf::from(required("LAYERX_CORE_SUPERVISOR_SOCKET")?),
        state_dir,
        _fee_limit: u128::from(parse_u64("LAYERX_CORE_FEE_LIMIT", 1_000)?),
        receipt_deadline: Duration::from_millis(parse_u64(
            "LAYERX_CORE_RECEIPT_DEADLINE_MS",
            15_000,
        )?),
        admin_lock: Mutex::new(()),
        journal_lock: Mutex::new(()),
    })
}

fn read_http_message(stream: &mut impl Read, maximum: usize) -> Result<Request, String> {
    let mut bytes = Vec::with_capacity(2048);
    let mut chunk = [0_u8; 4096];
    let header_end = loop {
        let count = stream.read(&mut chunk).map_err(|error| error.to_string())?;
        if count == 0 || bytes.len().saturating_add(count) > maximum {
            return Err("HTTP message is empty or exceeds its bound".to_owned());
        }
        bytes.extend_from_slice(&chunk[..count]);
        if let Some(position) = bytes.windows(4).position(|window| window == b"\r\n\r\n") {
            break position + 4;
        }
        if bytes.len() > 16 * 1024 {
            return Err("HTTP headers exceed their bound".to_owned());
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
        query: None,
        headers,
        body: bytes[header_end..header_end + content_length].to_vec(),
    })
}

fn valid_query(query: &str) -> bool {
    !query.is_empty()
        && query.len() <= 512
        && query.bytes().all(|byte| {
            byte.is_ascii_alphanumeric() || matches!(byte, b'=' | b'&' | b'-' | b'_' | b'.')
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
    let target = parts
        .next()
        .ok_or_else(|| "request target is missing".to_owned())?
        .to_owned();
    if parts.next() != Some("HTTP/1.1") || parts.next().is_some() {
        return Err("request line is invalid".to_owned());
    }
    match target.split_once('?') {
        Some((path, query)) => {
            if !valid_query(query) {
                return Err("request query is invalid".to_owned());
            }
            path.clone_into(&mut request.path);
            request.query = Some(query.to_owned());
        }
        None => request.path = target,
    }
    if !request.path.starts_with('/') || request.path.contains(['#', '\\', ' ']) {
        return Err("request path is invalid".to_owned());
    }
    if !request.headers.contains_key("host") {
        return Err("HTTP/1.1 Host header is required".to_owned());
    }
    Ok(request)
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
        authoritative_replay: false,
    }
}

fn json_response(status: u16, value: &serde_json::Value) -> Response {
    Response {
        status,
        body: value.to_string(),
        retry_after: None,
        authoritative_replay: false,
    }
}

fn next_trace() -> String {
    format!("core-{}", TRACE.fetch_add(1, Ordering::AcqRel))
}

fn success(result: &serde_json::Value) -> Response {
    json_response(
        200,
        &serde_json::json!({ "ok": true, "result": result, "trace": next_trace() }),
    )
}

fn write_response(
    stream: &mut impl Write,
    response: &Response,
    keep_alive: bool,
) -> Result<(), String> {
    let reason = match response.status {
        200 => "OK",
        202 => "Accepted",
        400 => "Bad Request",
        401 => "Unauthorized",
        404 => "Not Found",
        405 => "Method Not Allowed",
        409 => "Conflict",
        422 => "Unprocessable Entity",
        429 => "Too Many Requests",
        _ => "Service Unavailable",
    };
    let retry = response.retry_after.map_or(String::new(), |seconds| {
        format!("Retry-After: {seconds}\r\n")
    });
    let connection = if keep_alive { "keep-alive" } else { "close" };
    write!(
        stream,
        "HTTP/1.1 {} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nCache-Control: no-store\r\n{retry}Connection: {connection}\r\n\r\n{}",
        response.status,
        response.body.len(),
        response.body
    )
    .map_err(|error| error.to_string())
}

fn receipt_facts(bytes: &[u8], sequencer_key: [u8; 32]) -> Result<ReceiptFacts, String> {
    let decoded = layerx_wire::receipt::decode(bytes)
        .map_err(|error| format!("receipt does not decode: {error:?}"))?;
    let protocol = decoded
        .protocol()
        .ok_or_else(|| "receipt is not a protocol receipt".to_owned())?;
    let authorised = AuthorizedBatch::new(
        protocol.batch_id(),
        protocol.asset(),
        protocol.previous_state_root(),
        protocol.resulting_state_root(),
        sequencer_key,
    );
    let verified = verify_outcome(bytes, &authorised)
        .map_err(|error| format!("receipt verification failed: {error:?}"))?;
    let receipt = verified
        .receipt()
        .protocol()
        .ok_or_else(|| "verified receipt is not a protocol receipt".to_owned())?;
    Ok(ReceiptFacts {
        activity_id: receipt.activity_id(),
        batch_id: receipt.batch_id(),
        result_code: receipt.result_code(),
        state_root: receipt.resulting_state_root(),
        canonical: verified.canonical_bytes().to_vec(),
    })
}

fn await_authenticated_receipt(
    config: &Config,
    tx_hash: [u8; 32],
    deadline: Duration,
) -> Result<layerx_client::rpc::receipt::Lookup, String> {
    use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
    let started = Instant::now();
    let keys = config
        .sequencer
        .receipt_keys()
        .map_err(|error| format!("receipt keys unavailable: {error:?}"))?;
    loop {
        let outcome = lookup_authenticated(
            &config.sequencer,
            tx_hash,
            LookupContext {
                receipt_keys: &keys,
            },
        )
        .map_err(|error| format!("receipt lookup failed: {error:?}"))?;
        if let Lookup::Verified(receipt) = &outcome {
            if !keys.iter().any(|key| {
                key.key_id == receipt.execution().key_id
                    && key.public_key == receipt.execution().signing_key
                    && (key.first_height..=key.last_height).contains(&receipt.height())
            }) {
                return Err("receipt signing key differs".to_owned());
            }
            return Ok(outcome);
        }
        let Some(remaining) = deadline
            .checked_sub(started.elapsed())
            .filter(|remaining| !remaining.is_zero())
        else {
            return Ok(outcome);
        };
        thread::sleep(remaining.min(Duration::from_millis(250)));
    }
}

enum ReceiptRead {
    Absent,
    Submitted { tx_hash: [u8; 32], nonce: u64 },
    Verified(ReceiptFacts),
}

impl ReceiptRead {
    fn into_verified(self) -> Option<ReceiptFacts> {
        match self {
            Self::Verified(facts) => Some(facts),
            Self::Absent | Self::Submitted { .. } => None,
        }
    }
}

fn await_receipt(
    config: &Config,
    tx_hash: [u8; 32],
    deadline: Duration,
) -> Result<ReceiptRead, String> {
    use layerx_client::rpc::receipt::Lookup;
    let receipt = match await_authenticated_receipt(config, tx_hash, deadline)? {
        Lookup::Absent => return Ok(ReceiptRead::Absent),
        Lookup::Submitted { tx_hash, nonce } => {
            return Ok(ReceiptRead::Submitted { tx_hash, nonce });
        }
        Lookup::Verified(receipt) => receipt,
    };
    let landing = receipt.landing().ok_or("native receipt has not landed")?;
    let (batch, state_root) =
        layerx_platform_core::native_committed_state(&config.sequencer, landing.block_number)?;
    validate_receipt_landing(&receipt, &batch)?;
    Ok(ReceiptRead::Verified(ReceiptFacts {
        activity_id: receipt.activity_id(),
        result_code: receipt.result_code().raw(),
        canonical: receipt.canonical_bytes().to_vec(),
        batch_id: batch.hash,
        state_root,
    }))
}

fn validate_receipt_landing(
    receipt: &layerx_client::rpc::receipt::AuthenticatedReceipt,
    block: &layerx_client::sequencer::SequencerBlock,
) -> Result<(), String> {
    let landing = receipt.landing().ok_or("native receipt has not landed")?;
    let index =
        usize::try_from(landing.transaction_index).map_err(|_| "native receipt index overflow")?;
    if block.transactions.get(index).is_none_or(|transaction| {
        layerx_crypto::evm_transaction::transaction_hash(transaction) != receipt.activity_id()
    }) {
        return Err("native receipt transaction is absent from its landing block".into());
    }
    if block.number != landing.block_number
        || block.number < receipt.height()
        || landing.status != receipt.execution().success
        || receipt
            .covering_block()
            .is_some_and(|(height, hash)| height != block.number || hash != block.hash)
    {
        return Err("native receipt landing block differs".into());
    }
    Ok(())
}

fn receipt_route(config: &Config, activity_hex: &str) -> Response {
    let Ok(activity_id) = fixed_hex::<32>("activity id", activity_hex) else {
        return refusal(400, "invalid_argument", None);
    };
    if activity_hex.bytes().any(|byte| byte.is_ascii_uppercase()) {
        return refusal(400, "invalid_argument", None);
    }
    match await_receipt(config, activity_id, Duration::ZERO) {
        Ok(ReceiptRead::Verified(facts)) => success(&serde_json::json!({
            "activity_id": activity_hex,
            "receipt": hex_encode(&facts.canonical),
            "block_hash": hex_encode(&facts.batch_id),
            "state_root": hex_encode(&facts.state_root),
        })),
        Ok(ReceiptRead::Absent) => refusal(404, "not_found", None),
        Ok(ReceiptRead::Submitted { tx_hash, nonce }) => json_response(
            202,
            &serde_json::json!({"result": {
                "state": "pending", "rung": "submitted",
                "activity_id": hex_encode(&tx_hash), "nonce": nonce.to_string()
            }}),
        ),
        Err(error) => {
            eprintln!("layerx-core-boundary: {error}");
            refusal(503, "receipt_unavailable", Some(5))
        }
    }
}

fn readiness(config: &Config) -> Response {
    let mut response = (|| {
        let (Ok(chain_id), Ok(height)) =
            (config.sequencer.chain_id(), config.sequencer.block_number())
        else {
            return refusal(503, "node_unavailable", Some(5));
        };
        let Ok((block, app_hash)) =
            layerx_platform_core::native_committed_state(&config.sequencer, height)
        else {
            return refusal(503, "committed_chain_unavailable", Some(5));
        };
        let Ok(_guard) = config.journal_lock.lock() else {
            return refusal(503, "journal_unavailable", Some(5));
        };
        if journal_probe(&config.state_dir.join("journal/ready")).is_err() {
            return refusal(503, "journal_unavailable", Some(5));
        }
        json_response(
            200,
            &serde_json::json!({
                "ready": true, "network_id": format!("eip155:{chain_id}"),
                "paxeer": {"chain_id": chain_id, "latest_block": block.number},
                "latest_committed_height": block.number, "block_hash": hex_encode(&block.hash),
                "app_hash": hex_encode(&app_hash), "wire_version": WIRE_VERSION,
                "synchronous_receipts": true, "state_snapshot": true,
            }),
        )
    })();
    let funding = config
        .funding
        .get()
        .and_then(chain_fund::ChainFunding::readiness)
        .unwrap_or_else(|reason| serde_json::json!({"ready": false, "reason": reason}));
    let mut document: serde_json::Value = serde_json::from_str(&response.body)
        .unwrap_or_else(|error| panic!("{}: {error:?}", "readiness response is JSON"));
    document["funding"] = funding;
    response.body = document.to_string();
    response
}

fn native_state_route(config: &Config) -> Response {
    let Ok(height) = config.sequencer.block_number() else {
        return refusal(503, "node_unavailable", Some(5));
    };
    match layerx_platform_core::native_committed_state(&config.sequencer, height) {
        Ok((block, root)) => success(&serde_json::json!({
            "height": block.number.to_string(), "block_hash": hex_encode(&block.hash),
            "state_root": hex_encode(&root), "commitment": "native_app_hash"
        })),
        Err(_) => refusal(503, "committed_chain_unavailable", Some(5)),
    }
}

fn sequencer_route(config: &Config) -> Response {
    match config.network.network() {
        Ok(head) => success(&serde_json::json!({
            "network_id": head.network_id,
            "paxeer": {
                "chain_id": head.paxeer.chain_id,
                "latest_block": head.paxeer.latest_block,
            },
            "layerx": head.layerx,
            "latest_committed_height": head.latest_committed_height,
        })),
        Err(error) => {
            eprintln!("layerx-core-boundary: {error:?}");
            refusal(503, "node_unavailable", Some(5))
        }
    }
}

fn is_hex64(value: &str) -> bool {
    value.len() == 64 && value.bytes().all(|byte| byte.is_ascii_hexdigit())
}

fn retired_contract(path: &str) -> bool {
    let segments: Vec<&str> = path.trim_start_matches('/').split('/').collect();
    match segments.as_slice() {
        ["v1", "protocol", "account-state", "head"]
        | ["v1", "programs", "account-state", "changes"] => true,
        ["v1", "receipts" | "programs", id, "account-state"]
        | ["v1", "batches", id, "receipt-authority"] => is_hex64(id),
        _ => false,
    }
}

const PROGRAM_EVENTS_PREFIX: &str = "/v1/programs/events/";
const PROGRAM_EVENT_MAX_TOPIC_BYTES: usize = 64;
const PROGRAM_EVENTS_MAX_PAGE: u64 = 256;

fn canonical_u64(value: &str) -> Option<u64> {
    if value.is_empty()
        || (value.len() > 1 && value.starts_with('0'))
        || !value.bytes().all(|byte| byte.is_ascii_digit())
    {
        return None;
    }
    value.parse().ok()
}

fn program_events_target(path: &str) -> bool {
    let Some(suffix) = path.strip_prefix(PROGRAM_EVENTS_PREFIX) else {
        return false;
    };
    let segments: Vec<&str> = suffix.split('/').collect();
    let [topic, from_sequence, limit] = segments.as_slice() else {
        return false;
    };
    !topic.is_empty()
        && topic.len() % 2 == 0
        && topic.len() <= 2 * PROGRAM_EVENT_MAX_TOPIC_BYTES
        && topic
            .bytes()
            .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
        && canonical_u64(from_sequence).is_some()
        && canonical_u64(limit).is_some_and(|limit| (1..=PROGRAM_EVENTS_MAX_PAGE).contains(&limit))
}

fn unavailable_capability(path: &str) -> bool {
    path == "/v1/accounts"
        || path == "/v1/programs/registry"
        || path.starts_with("/v1/programs/registry/")
        || path.starts_with("/v1/programs/activities/")
        || path.starts_with("/v1/programs/receipts/by-idempotency/")
}

fn gateway_readiness(config: &Config, request: &Request) -> Response {
    let Some(token) = &config.receipt_events_token else {
        return refusal(503, "gateway_readiness_not_configured", None);
    };
    let supplied = request
        .headers
        .get("authorization")
        .and_then(|value| value.strip_prefix("Bearer "))
        .unwrap_or("");
    if !bool::from(supplied.as_bytes().ct_eq(token.as_bytes())) {
        return refusal(401, "unauthorized", None);
    }
    if request.method != "GET" {
        return refusal(405, "method_not_allowed", None);
    }
    if request.query.is_some() || !request.body.is_empty() {
        return refusal(400, "invalid_request", None);
    }
    readiness(config)
}

fn core_route(config: &Config, request: &Request) -> Response {
    if request.path == "/internal/readyz" {
        return gateway_readiness(config, request);
    }
    public_reads::route(config, request).unwrap_or_else(|| protocol_route(config, request))
}

fn protocol_route(config: &Config, request: &Request) -> Response {
    let method = request.method.as_str();
    let path = request.path.as_str();
    if let Some(key) = path.strip_prefix("/v1/programs/receipts/by-idempotency/") {
        return if method != "GET" {
            refusal(405, "method_not_allowed", None)
        } else if request.query.is_some() {
            refusal(400, "invalid_request", None)
        } else {
            let _ = key;
            refusal(410, "retired_http_contract", None)
        };
    }
    if unavailable_capability(path) {
        return refusal(503, "capability_unavailable", Some(3600));
    }
    if path.starts_with(PROGRAM_EVENTS_PREFIX) {
        return if !program_events_target(path) {
            refusal(400, "invalid_argument", None)
        } else if method != "GET" {
            refusal(405, "method_not_allowed", None)
        } else if request.query.is_some() {
            refusal(400, "invalid_request", None)
        } else {
            refusal(410, "retired_http_contract", None)
        };
    }
    if retired_contract(path) {
        return if method == "GET" {
            if path == "/v1/protocol/account-state/head" {
                native_state_route(config)
            } else {
                refusal(410, "retired_http_contract", None)
            }
        } else {
            refusal(405, "method_not_allowed", None)
        };
    }
    if request.query.is_some() {
        return refusal(400, "invalid_request", None);
    }
    match (method, path) {
        ("GET", "/livez") => json_response(200, &serde_json::json!({ "live": true })),
        ("GET", "/readyz") => readiness(config),
        ("GET", "/v1/sequencer") => sequencer_route(config),
        ("GET", "/v1/state") => native_state_route(config),
        ("GET", other) if other.starts_with("/v1/receipts/") => {
            receipt_route(config, &other["/v1/receipts/".len()..])
        }
        (_, "/livez" | "/readyz" | "/v1/sequencer" | "/v1/state") => {
            refusal(405, "method_not_allowed", None)
        }
        _ => refusal(404, "not_found", None),
    }
}

fn valid_key(value: &str) -> bool {
    !value.is_empty()
        && value.len() <= 128
        && value
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_' | b'.' | b':'))
}

fn journal_path(config: &Config, scope: &str, key: &str) -> PathBuf {
    let mut digest = Sha256::new();
    digest.update(scope.as_bytes());
    digest.update([0]);
    digest.update(key.as_bytes());
    config
        .state_dir
        .join("journal")
        .join(format!("{}.json", hex_encode(&digest.finalize())))
}

fn request_digest(request: &Request) -> String {
    let mut digest = Sha256::new();
    digest.update(request.method.as_bytes());
    digest.update([0]);
    digest.update(request.path.as_bytes());
    digest.update([0]);
    digest.update(&request.body);
    hex_encode(&digest.finalize())
}

fn journal_read(path: &Path) -> Result<Option<JournalEntry>, String> {
    match fs::read(path) {
        Ok(bytes) => {
            if bytes.len() as u64 > MAX_JOURNAL_BYTES {
                return Err("journal entry exceeds its bound".to_owned());
            }
            if let Ok(entry) = serde_json::from_slice::<JournalEntry>(&bytes) {
                return Ok(Some(entry));
            }
            let mut latest = None;
            for record in bytes.split_inclusive(|byte| *byte == b'\n') {
                if !record.ends_with(b"\n") {
                    break;
                }
                let record = &record[..record.len() - 1];
                if record.is_empty() {
                    return Err("journal entry is corrupt: empty record".to_owned());
                }
                latest = Some(
                    serde_json::from_slice::<JournalEntry>(record)
                        .map_err(|error| format!("journal entry is corrupt: {error}"))?,
                );
            }
            latest
                .map(Some)
                .ok_or_else(|| "journal entry is corrupt: no complete record".to_owned())
        }
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => Ok(None),
        Err(error) => Err(error.to_string()),
    }
}

fn journal_write(path: &Path, entry: &JournalEntry) -> Result<(), String> {
    let mut bytes = serde_json::to_vec(entry).map_err(|error| error.to_string())?;
    bytes.push(b'\n');
    let mut file = fs::OpenOptions::new()
        .read(true)
        .create(true)
        .append(true)
        .mode(0o600)
        .open(path)
        .map_err(|error| error.to_string())?;
    let mut prior = Vec::new();
    file.read_to_end(&mut prior)
        .map_err(|error| error.to_string())?;
    if prior.len() as u64 > MAX_JOURNAL_BYTES {
        return Err("journal entry exceeds its bound".to_owned());
    }
    if !prior.is_empty() && !prior.ends_with(b"\n") {
        if serde_json::from_slice::<JournalEntry>(&prior).is_ok() {
            bytes.insert(0, b'\n');
        } else {
            let committed = prior
                .iter()
                .rposition(|byte| *byte == b'\n')
                .map_or(0, |position| position + 1);
            if committed == 0 {
                return Err("journal entry is corrupt: no complete record".to_owned());
            }
            file.set_len(committed as u64)
                .map_err(|error| error.to_string())?;
            prior.truncate(committed);
        }
    }
    let prior_len = prior.len() as u64;
    if prior_len.saturating_add(bytes.len() as u64) > MAX_JOURNAL_BYTES {
        return Err("journal entry exceeds its bound".to_owned());
    }
    file.write_all(&bytes).map_err(|error| error.to_string())?;
    file.sync_data().map_err(|error| error.to_string())?;
    if prior_len == 0 {
        let directory = path
            .parent()
            .ok_or_else(|| "journal path has no parent".to_owned())?;
        fs::File::open(directory)
            .and_then(|handle| handle.sync_all())
            .map_err(|error| error.to_string())?;
    }
    Ok(())
}

fn journal_probe(path: &Path) -> Result<(), String> {
    let mut file = fs::OpenOptions::new()
        .write(true)
        .create(true)
        .truncate(true)
        .mode(0o600)
        .open(path)
        .map_err(|error| error.to_string())?;
    file.write_all(b"ready\n")
        .map_err(|error| error.to_string())?;
    file.sync_data().map_err(|error| error.to_string())
}

fn stateful(
    config: &Config,
    scope: &str,
    request: &Request,
    execute: impl FnOnce() -> Response,
) -> Response {
    let total_started = Instant::now();
    let timed = scope == "activities";
    let Some(key) = request.headers.get("idempotency-key") else {
        return if scope == "activities" {
            execute()
        } else {
            refusal(400, "idempotency_key_required", None)
        };
    };
    if !valid_key(key) {
        return refusal(400, "invalid_idempotency_key", None);
    }
    let lock_started = Instant::now();
    let Ok(_guard) = config.journal_lock.lock() else {
        return refusal(503, "journal_unavailable", Some(5));
    };
    if timed {
        pay_timing("core.journal.lock", lock_started);
    }
    let path = journal_path(config, scope, key);
    let digest = request_digest(request);
    let read_started = Instant::now();
    let prior = journal_read(&path);
    if timed {
        pay_timing("core.journal.read", read_started);
    }
    match prior {
        Ok(Some(entry)) if entry.request_digest == digest => {
            if matches!(scope, "fund" | "reset")
                && (entry.status == 202
                    || entry.status == 409
                    || entry.status >= 500
                    || entry.retry_after.is_some())
            {
                if journal_write(&path, &entry).is_err() {
                    return refusal(503, "journal_unavailable", Some(5));
                }
                let response = execute();
                if response.authoritative_replay {
                    return response;
                }
                if journal_write(
                    &path,
                    &JournalEntry {
                        request_digest: digest,
                        status: response.status,
                        body: response.body.clone(),
                        retry_after: response.retry_after,
                    },
                )
                .is_err()
                {
                    return refusal(503, "journal_unavailable", Some(5));
                }
                return response;
            }
            return Response {
                status: entry.status,
                body: entry.body,
                retry_after: entry.retry_after,
                authoritative_replay: false,
            };
        }
        Ok(Some(_)) => return refusal(409, "idempotency_conflict", None),
        Ok(None) => {}
        Err(error) => {
            eprintln!("layerx-core-boundary: journal: {error}");
            return refusal(503, "journal_unavailable", Some(5));
        }
    }
    stateful_execute_new(&path, digest, scope, timed, total_started, execute)
}

fn stateful_execute_new(
    path: &Path,
    digest: String,
    scope: &str,
    timed: bool,
    total_started: Instant,
    execute: impl FnOnce() -> Response,
) -> Response {
    let pending = if scope == "activities" {
        json_response(
            202,
            &serde_json::json!({"ok": true, "result": {"state": "pending"}}),
        )
    } else {
        refusal(409, "outcome_unknown", Some(5))
    };
    let pending_started = Instant::now();
    if journal_write(
        path,
        &JournalEntry {
            request_digest: digest.clone(),
            status: pending.status,
            body: pending.body,
            retry_after: pending.retry_after,
        },
    )
    .is_err()
    {
        return refusal(503, "journal_unavailable", Some(5));
    }
    if timed {
        pay_timing("core.journal.pending_commit", pending_started);
    }
    let execute_started = Instant::now();
    let response = execute();
    if timed {
        pay_timing("core.journal.execute", execute_started);
    }
    if response.authoritative_replay {
        return response;
    }
    let final_started = Instant::now();
    if let Err(error) = journal_write(
        path,
        &JournalEntry {
            request_digest: digest,
            status: response.status,
            body: response.body.clone(),
            retry_after: response.retry_after,
        },
    ) {
        eprintln!("layerx-core-boundary: journal: {error}");
        return refusal(503, "journal_unavailable", Some(5));
    }
    if timed {
        pay_timing("core.journal.final_commit", final_started);
        pay_timing("core.journal.total", total_started);
    }
    response
}

fn admin_authorized(config: &Config, request: &Request) -> bool {
    request
        .headers
        .get("authorization")
        .and_then(|value| value.strip_prefix("Bearer "))
        .is_some_and(|token| {
            token.len() == config.admin_token.len()
                && token.as_bytes().ct_eq(config.admin_token.as_bytes()).into()
        })
}

fn send_idempotency(key: &str) -> [u8; 32] {
    let mut digest = Sha256::new();
    digest.update(b"layerx-core-fund\0");
    digest.update(key.as_bytes());
    digest.finalize().into()
}

#[cfg(test)]
fn send_refusal(error: layerx_platform_core::SendError) -> Response {
    match error {
        layerx_platform_core::SendError::Signer(reason) => {
            eprintln!("layerx-core-boundary: treasury signer: {reason}");
            refusal(503, "treasury_signer_unavailable", Some(5))
        }
        layerx_platform_core::SendError::Invalid(reason) => {
            eprintln!("layerx-core-boundary: send construction: {reason}");
            refusal(422, "send_unbuildable", None)
        }
    }
}

fn fund(config: &Config, request: &Request, key: &str) -> Response {
    let Ok(command) = serde_json::from_slice::<FundingCommand>(&request.body) else {
        return refusal(400, "invalid_argument", None);
    };
    if !valid_key(&command.funding_id)
        || !command.did.starts_with("did:")
        || command.did.len() > 512
        || !is_hex64(&command.public_key)
        || command.did != format!("did:layerx:{}", command.public_key.to_ascii_lowercase())
        || command.amount == 0
        || command.did == config.treasury_did
        || main_account(&command.did).is_err()
    {
        return refusal(400, "invalid_argument", None);
    }
    match fund_send(config, request, &command, key) {
        Ok(response) | Err(response) => response,
    }
}

fn prepare_funding(
    config: &Config,
    request: &Request,
    command: &FundingCommand,
    key: &str,
) -> Result<chain_fund::NativeTransfer, Response> {
    funding_native::prepare(config, request, command, key)
}

fn fund_send(
    config: &Config,
    request: &Request,
    command: &FundingCommand,
    key: &str,
) -> Result<Response, Response> {
    if let Some(bytes) = funding_read(&funding_record_path(config, "funding-intents", key))
        .map_err(|_| refusal(503, "journal_unavailable", Some(5)))?
    {
        if !funding_native::is_native(&bytes) {
            // Version one is an authenticated native ledger SEND, not a
            // value transfer. Replay it through its original chain interface.
            let intent: FundingIntent = serde_json::from_slice(&bytes)
                .map_err(|_| refusal(503, "journal_unavailable", Some(5)))?;
            validate_funding_intent(config, &intent)
                .map_err(|_| refusal(503, "journal_unavailable", Some(5)))?;
            if intent.request_digest != request_digest(request) {
                return Err(refusal(409, "idempotency_conflict", None));
            }
            config
                .sequencer
                .send_raw_transaction(&intent.signed.canonical)
                .map_err(|_| refusal(503, "treasury_unavailable", Some(5)))?;
            let facts = await_receipt(
                config,
                intent.signed.transaction_hash,
                config.receipt_deadline,
            )
            .map_err(|_| refusal(503, "receipt_unavailable", Some(5)))?
            .into_verified()
            .ok_or_else(|| refusal(503, "receipt_unavailable", Some(5)))?;
            return intent_funding_response(config, &intent, &facts, true);
        }
    }
    let transfer = prepare_funding(config, request, command, key)?;
    let funding = config
        .funding
        .get()
        .map_err(|_| refusal(503, "treasury_unavailable", Some(5)))?;
    if transfer.sender != funding.sender_address() || transfer.chain_id != funding.chain_id {
        return Err(refusal(409, "funding_sender_conflict", None));
    }
    funding.submit(&transfer.signed)?;
    let receipt = funding.confirmation(&transfer)?;
    if let Some(receipt) = receipt {
        funding_native::archive(config, key, receipt)?;
        return funding_native::archived_response(config, request, key)?
            .ok_or_else(|| refusal(503, "journal_unavailable", Some(5)));
    }
    Ok(json_response(
        202,
        &serde_json::json!({
            "funding_id": command.funding_id, "state": "pending",
            "transaction_id": transfer.signed.transaction_hash,
        }),
    ))
}

fn funding_record_path(config: &Config, directory: &str, key: &str) -> PathBuf {
    let canonical = journal_path(config, "fund-canonical", key);
    config.state_dir.join(directory).join(
        canonical
            .file_name()
            .unwrap_or_else(|| panic!("{}", "journal filename")),
    )
}

fn funding_directory(path: &Path) -> Result<(), String> {
    let metadata = fs::symlink_metadata(path).map_err(|error| error.to_string())?;
    if !metadata.is_dir() || metadata.mode() & 0o077 != 0 || metadata.mode() & 0o700 != 0o700 {
        return Err("funding storage is not a private usable directory".to_owned());
    }
    Ok(())
}

fn funding_read(path: &Path) -> Result<Option<Vec<u8>>, String> {
    let parent = path.parent().ok_or("funding storage parent missing")?;
    funding_directory(parent)?;
    let mut file = match fs::OpenOptions::new()
        .read(true)
        .custom_flags(0x20800)
        .open(path)
    {
        Ok(file) => file,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => return Ok(None),
        Err(error) => return Err(error.to_string()),
    };
    let metadata = file.metadata().map_err(|error| error.to_string())?;
    if !metadata.is_file()
        || metadata.mode() & 0o077 != 0
        || metadata.mode() & 0o400 == 0
        || metadata.len() > MAX_FUNDING_RECORD_BYTES
        || metadata.nlink() != 1
        || metadata.uid()
            != fs::symlink_metadata(parent)
                .map_err(|error| error.to_string())?
                .uid()
    {
        return Err("funding record is invalid or exceeds its bound".to_owned());
    }
    let mut bytes = Vec::new();
    (&mut file)
        .take(MAX_FUNDING_RECORD_BYTES + 1)
        .read_to_end(&mut bytes)
        .map_err(|error| error.to_string())?;
    if bytes.len() as u64 > MAX_FUNDING_RECORD_BYTES {
        return Err("funding record exceeds its bound".to_owned());
    }
    Ok(Some(bytes))
}

fn funding_store(path: &Path, bytes: &[u8]) -> Result<(), String> {
    if bytes.len() as u64 > MAX_FUNDING_RECORD_BYTES {
        return Err("funding record exceeds its bound".to_owned());
    }
    let parent = path.parent().ok_or("funding storage parent missing")?;
    funding_directory(parent)?;
    if let Some(existing) = funding_read(path)? {
        if existing != bytes {
            return Err("immutable funding record differs".to_owned());
        }
        fs::File::open(path)
            .and_then(|file| file.sync_all())
            .map_err(|error| error.to_string())?;
        return fs::File::open(parent)
            .and_then(|directory| directory.sync_all())
            .map_err(|error| error.to_string());
    }
    let inventory = funding_inventory(parent)?;
    let total = inventory
        .iter()
        .try_fold(bytes.len() as u64, |total, entry| {
            fs::metadata(entry).map(|metadata| total.saturating_add(metadata.len()))
        })
        .map_err(|error| error.to_string())?;
    if inventory.len() >= MAX_FUNDING_RECORDS || total > MAX_FUNDING_INVENTORY_BYTES {
        return Err("funding inventory exceeds bound".to_owned());
    }
    let temporary = parent.join(format!(
        ".pending-{}-{}",
        std::process::id(),
        TRACE.fetch_add(1, Ordering::Relaxed)
    ));
    let result = (|| {
        let mut file = fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temporary)
            .map_err(|error| error.to_string())?;
        file.write_all(bytes).map_err(|error| error.to_string())?;
        file.sync_all().map_err(|error| error.to_string())?;
        fs::hard_link(&temporary, path).map_err(|error| error.to_string())?;
        fs::remove_file(&temporary).map_err(|error| error.to_string())?;
        fs::File::open(parent)
            .and_then(|directory| directory.sync_all())
            .map_err(|error| error.to_string())
    })();
    if result.is_err() {
        let _ = fs::remove_file(&temporary);
    }
    result
}

fn validate_funding_request(config: &Config, intent: &FundingIntent) -> Result<(), String> {
    if intent.version != 1
        || !valid_key(&intent.key)
        || intent.network_id != config.network_id
        || intent.asset != config.treasury_asset
        || intent.sequencer_id != config.sequencer_id
    {
        return Err("funding intent identity differs".to_owned());
    }
    let command: FundingCommand =
        serde_json::from_slice(&intent.request_body).map_err(|error| error.to_string())?;
    if !valid_key(&command.funding_id)
        || command.amount == 0
        || !is_hex64(&command.public_key)
        || command.did != format!("did:layerx:{}", command.public_key.to_ascii_lowercase())
        || command.did == config.treasury_did
    {
        return Err("funding command differs".to_owned());
    }
    if serde_json::to_vec(&command).map_err(|error| error.to_string())?
        != serde_json::to_vec(&intent.command).map_err(|error| error.to_string())?
        || intent.request_digest
            != request_digest(&Request {
                method: "POST".to_owned(),
                path: "/admin/v1/testnet/fund".to_owned(),
                query: None,
                headers: BTreeMap::new(),
                body: intent.request_body.clone(),
            })
    {
        return Err("funding request binding differs".to_owned());
    }
    Ok(())
}

fn validate_funding_debit(
    config: &Config,
    intent: &FundingIntent,
    payload: &[u8],
) -> Result<Vec<u8>, String> {
    let source = main_account(&config.treasury_did)?;
    let destination = main_account(&intent.command.did)?;
    let idempotency_key = send_idempotency(&intent.key);
    let debit = SendDebit {
        from: source,
        to: destination,
        asset: intent.asset,
        amount: u128::from(intent.command.amount),
        source_sequence: u64::from_be_bytes(
            payload[116..124]
                .try_into()
                .map_err(|_| "invalid debit sequence")?,
        ),
        idempotency_key,
        expires_at: u64::from_be_bytes(
            payload[156..164]
                .try_into()
                .map_err(|_| "invalid debit expiry")?,
        ),
        context_hash: layerx_platform_core::send_context_hash(
            &source,
            &destination,
            &intent.asset,
            u128::from(intent.command.amount),
            &idempotency_key,
        ),
        conditions: Vec::new(),
        authorization_kind: layerx_types::intent::SendAuthorizationKind::Owner as u8,
        network_id: config.network_id,
        protocol_version: layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
    };
    let payload_signature = payload[262..326]
        .try_into()
        .map_err(|_| "invalid debit signature")?;
    let expected_payload = debit
        .encode_signed(config.treasury.public_key(), payload_signature)
        .map_err(|error| format!("{error:?}"))?;
    if payload != expected_payload.as_slice() {
        return Err("canonical funding debit differs".to_owned());
    }
    Ok(expected_payload)
}

fn validate_funding_intent(config: &Config, intent: &FundingIntent) -> Result<(), String> {
    validate_funding_request(config, intent)?;
    let (call, signature) =
        decode_signed(&intent.signed.canonical).map_err(|error| error.to_string())?;
    let sender = recover_sender(&call, &signature).map_err(|error| error.to_string())?;
    let treasury = config
        .treasury
        .transaction_signer()
        .evm_address()
        .ok_or("treasury address unavailable")?;
    let mut ledger = [0; 20];
    ledger[18..].copy_from_slice(&0x101a_u16.to_be_bytes());
    if call.chain_id != 125
        || call.to != ledger
        || sender != treasury
        || sender != intent.signed.sender
        || transaction_hash(&intent.signed.canonical) != intent.signed.transaction_hash
    {
        return Err("canonical funding identity differs".to_owned());
    }
    let disclosure = bind_call(&call, sender).map_err(|error| format!("{error:?}"))?;
    let Some(layerx_crypto::disclosure::call::ConstructMethod::LedgerSend { payload }) = disclosure
        .construct_method()
        .map_err(|error| format!("{error:?}"))?
    else {
        return Err("canonical funding is not a ledger send".to_owned());
    };
    if payload.len() != 364 {
        return Err("canonical funding debit differs".to_owned());
    }
    let expected_payload = validate_funding_debit(config, intent, &payload)?;
    let expected = ledger_payment_call(
        &expected_payload,
        &CallOptions {
            from: sender,
            chain_id: 125,
            nonce: call.nonce,
            max_priority_fee_per_gas: call.max_priority_fee_per_gas,
            max_fee_per_gas: call.max_fee_per_gas,
            gas_limit: call.gas_limit,
        },
    )
    .map_err(|error| format!("{error:?}"))?;
    if call != expected.call || disclosure != expected.disclosure {
        return Err("canonical funding call differs".to_owned());
    }
    if disclosure.asset != config.treasury_asset
        || disclosure.counterparties.len() != 2
        || disclosure.amounts.len() != 1
        || !disclosure.counterparties.iter().any(|party| {
            party.role == CounterpartyRole::Payer
                && main_account(&config.treasury_did) == Ok(party.account)
        })
        || !disclosure.counterparties.iter().any(|party| {
            party.role == CounterpartyRole::Recipient
                && main_account(&intent.command.did) == Ok(party.account)
        })
        || !disclosure.amounts.iter().any(|amount| {
            amount.role == AmountRole::Transfer && amount.value == u128::from(intent.command.amount)
        })
    {
        return Err("canonical funding semantics differ".to_owned());
    }
    Ok(())
}

fn persist_funding_intent(config: &Config, intent: &FundingIntent) -> Result<(), String> {
    validate_funding_intent(config, intent)?;
    let path = funding_record_path(config, "funding-intents", &intent.key);
    if let Some(bytes) = funding_read(&path)? {
        let existing: FundingIntent =
            serde_json::from_slice(&bytes).map_err(|error| error.to_string())?;
        validate_funding_intent(config, &existing)?;
        if existing.request_digest != intent.request_digest
            || existing.signed.canonical != intent.signed.canonical
            || existing.signed.transaction_hash != intent.signed.transaction_hash
            || existing.signed.sender != intent.signed.sender
        {
            return Err("durable funding intent differs".to_owned());
        }
        return Ok(());
    }
    funding_store(
        &path,
        &serde_json::to_vec(intent).map_err(|error| error.to_string())?,
    )
}

fn import_funding_intent(config: &Config, request: &Request) -> Response {
    let unavailable = || refusal(503, "journal_unavailable", Some(5));
    if request.method != "POST" {
        return refusal(405, "method_not_allowed", None);
    }
    if request.headers.get("content-type").map(String::as_str) != Some("application/json") {
        return refusal(400, "content_type_required", None);
    }
    let Ok(intent) = serde_json::from_slice::<FundingIntent>(&request.body) else {
        return refusal(400, "invalid_funding_intent", None);
    };
    if request.headers.get("idempotency-key") != Some(&intent.key)
        || validate_funding_intent(config, &intent).is_err()
    {
        return refusal(400, "invalid_funding_intent", None);
    }
    let stage_path = journal_path(config, "fund-canonical", &intent.key);
    let Ok(command_bytes) = serde_json::to_vec(&intent.command) else {
        return unavailable();
    };
    let command_digest = hex_encode(&Sha256::digest(&command_bytes));
    let Ok(stage) = journal_read(&stage_path) else {
        return unavailable();
    };
    if let Some(stage) = &stage {
        let Ok(signed) = serde_json::from_str::<PreparedFunding>(&stage.body) else {
            return refusal(409, "idempotency_conflict", None);
        };
        if stage.request_digest != command_digest
            || signed.canonical != intent.signed.canonical
            || signed.transaction_hash != intent.signed.transaction_hash
            || signed.sender != intent.signed.sender
        {
            return refusal(409, "idempotency_conflict", None);
        }
    }
    if persist_funding_intent(config, &intent).is_err() {
        return unavailable();
    }
    if stage.is_none() {
        let Ok(body) = serde_json::to_string(&intent.signed) else {
            return unavailable();
        };
        if journal_write(
            &stage_path,
            &JournalEntry {
                request_digest: command_digest,
                status: 202,
                body,
                retry_after: Some(5),
            },
        )
        .is_err()
        {
            return unavailable();
        }
    }
    success(&serde_json::json!({
        "state": "prepared",
        "transaction_hash": hex_encode(&intent.signed.transaction_hash),
        "sender": hex_encode(&intent.signed.sender)
    }))
}

fn funding_intent_route(config: &Config, request: &Request) -> Response {
    let unavailable = || refusal(503, "journal_unavailable", Some(5));
    let Ok(_guard) = config.admin_lock.lock() else {
        return unavailable();
    };
    if request.path == "/admin/v1/testnet/funding-intents" {
        if request.method == "POST" && funding_native::is_native(&request.body) {
            if request.headers.get("content-type").map(String::as_str) != Some("application/json") {
                return refusal(400, "content_type_required", None);
            }
            return funding_native::route(config, request);
        }
    } else if let Some(key) = request
        .path
        .strip_prefix("/admin/v1/testnet/funding-intents/")
    {
        if !valid_key(key) || !request.body.is_empty() {
            return refusal(400, "invalid_funding_intent", None);
        }
        match funding_read(&funding_record_path(config, "funding-intents", key)) {
            Ok(Some(bytes)) if funding_native::is_native(&bytes) => {
                return funding_native::route(config, request)
            }
            Err(_) => return unavailable(),
            _ => {}
        }
    }
    if request.path == "/admin/v1/testnet/funding-intents" {
        return import_funding_intent(config, request);
    }
    if request.method != "GET" {
        return refusal(405, "method_not_allowed", None);
    }
    let Some(key) = request
        .path
        .strip_prefix("/admin/v1/testnet/funding-intents/")
    else {
        return refusal(404, "not_found", None);
    };
    if !valid_key(key) || !request.body.is_empty() {
        return refusal(400, "invalid_funding_intent", None);
    }
    let bytes = match funding_read(&funding_record_path(config, "funding-intents", key)) {
        Ok(Some(bytes)) => bytes,
        Ok(None) => return refusal(404, "not_found", None),
        Err(_) => return unavailable(),
    };
    let Ok(intent) = serde_json::from_slice::<FundingIntent>(&bytes) else {
        return unavailable();
    };
    if intent.key != key || validate_funding_intent(config, &intent).is_err() {
        return unavailable();
    }
    success(&serde_json::json!({
        "state": "prepared",
        "transaction_hash": hex_encode(&intent.signed.transaction_hash),
        "sender": hex_encode(&intent.signed.sender),
        "canonical_hex": hex_encode(&intent.signed.canonical)
    }))
}

fn funding_archive_facts(
    config: &Config,
    intent: &FundingIntent,
    intent_bytes: &[u8],
    archive: &FundingArchive,
) -> Result<ReceiptFacts, String> {
    validate_funding_intent(config, intent)?;
    if archive.version != 1
        || archive.intent_digest != hex_encode(&Sha256::digest(intent_bytes))
        || archive.receipt.is_empty()
        || archive.receipt.len() > MAX_RECEIPT_BYTES
    {
        return Err("funding archive binding differs".to_owned());
    }
    funding_intent_receipt_facts(config, intent, &archive.receipt)
}

fn funding_intent_receipt_facts(
    config: &Config,
    intent: &FundingIntent,
    bytes: &[u8],
) -> Result<ReceiptFacts, String> {
    validate_funding_intent(config, intent)?;
    let facts = receipt_facts(bytes, intent.sequencer_key)?;
    let decoded =
        layerx_wire::receipt::decode(&facts.canonical).map_err(|error| format!("{error:?}"))?;
    let receipt = decoded
        .protocol()
        .ok_or("funding receipt is not protocol receipt")?;
    if facts.activity_id != intent.signed.transaction_hash
        || receipt.asset() != intent.asset
        || receipt.amount() != u128::from(intent.command.amount)
        || receipt.from() != main_account(&config.treasury_did)?
        || receipt.to() != main_account(&intent.command.did)?
        || receipt.protocol_version() != 3
        || receipt.module_id() != 1
        || u16::from(receipt.operation()) != asset_registry()?.1.ordinal()
    {
        return Err("funding receipt semantics differ".to_owned());
    }
    Ok(facts)
}

fn intent_funding_response(
    config: &Config,
    intent: &FundingIntent,
    facts: &ReceiptFacts,
    submitted: bool,
) -> Result<Response, Response> {
    let verified =
        funding_intent_receipt_facts(config, intent, &facts.canonical).map_err(|_| {
            let mut response = refusal(503, "receipt_unavailable", Some(5));
            response.authoritative_replay = submitted;
            response
        })?;
    match funding_receipt_response(&intent.command, &verified) {
        Ok(mut response) => {
            response.authoritative_replay = true;
            Ok(response)
        }
        Err(mut response) => {
            response.authoritative_replay = true;
            Err(response)
        }
    }
}

fn archived_funding_response(
    config: &Config,
    request: &Request,
    key: &str,
) -> Result<Option<Response>, Response> {
    let unavailable = || refusal(503, "journal_unavailable", Some(5));
    let intent_bytes = funding_read(&funding_record_path(config, "funding-intents", key))
        .map_err(|_| unavailable())?;
    let archive_bytes = funding_read(&funding_record_path(config, "funding-receipts", key))
        .map_err(|_| unavailable())?;
    let Some(intent_bytes) = intent_bytes else {
        return if archive_bytes.is_some() {
            Err(unavailable())
        } else {
            Ok(None)
        };
    };
    if funding_native::is_native(&intent_bytes) {
        return funding_native::archived_response(config, request, key);
    }
    let intent: FundingIntent = serde_json::from_slice(&intent_bytes).map_err(|_| unavailable())?;
    validate_funding_intent(config, &intent).map_err(|_| unavailable())?;
    if intent.key != key || intent.request_digest != request_digest(request) {
        return Err(refusal(409, "idempotency_conflict", None));
    }
    let Some(archive_bytes) = archive_bytes else {
        let stage = journal_read(&journal_path(config, "fund-canonical", key))
            .map_err(|_| unavailable())?
            .ok_or_else(unavailable)?;
        return if stage.status == 200 {
            Err(unavailable())
        } else {
            Ok(None)
        };
    };
    let archive: FundingArchive =
        serde_json::from_slice(&archive_bytes).map_err(|_| unavailable())?;
    let facts = funding_archive_facts(config, &intent, &intent_bytes, &archive)
        .map_err(|_| unavailable())?;
    let response = match funding_receipt_response(&intent.command, &facts) {
        Ok(response) | Err(response) => response,
    };
    let _guard = config.journal_lock.lock().map_err(|_| unavailable())?;
    let response_path = journal_path(config, "fund", key);
    if let Some(prior) = journal_read(&response_path).map_err(|_| unavailable())? {
        if prior.request_digest != intent.request_digest {
            return Err(refusal(409, "idempotency_conflict", None));
        }
        if prior.status == 202
            || prior.status == 409
            || prior.status >= 500
            || prior.retry_after.is_some()
        {
            journal_write(&response_path, &prior).map_err(|_| unavailable())?;
        } else if prior.status != response.status
            || prior.body != response.body
            || prior.retry_after != response.retry_after
        {
            return Err(unavailable());
        }
    }
    Ok(Some(response))
}

fn funding_inventory(directory: &Path) -> Result<Vec<PathBuf>, String> {
    funding_directory(directory)?;
    let mut paths = Vec::new();
    let mut total = 0u64;
    for (index, entry) in fs::read_dir(directory)
        .map_err(|error| error.to_string())?
        .enumerate()
    {
        if index >= MAX_FUNDING_RECORDS {
            return Err("funding inventory exceeds bound".to_owned());
        }
        let path = entry.map_err(|error| error.to_string())?.path();
        if path
            .file_name()
            .and_then(|name| name.to_str())
            .is_some_and(|name| name.starts_with(".pending-"))
        {
            continue;
        }
        let metadata = fs::symlink_metadata(&path).map_err(|error| error.to_string())?;
        if !metadata.is_file() || metadata.len() > MAX_FUNDING_RECORD_BYTES {
            return Err("invalid funding inventory record".to_owned());
        }
        total = total
            .checked_add(metadata.len())
            .ok_or("funding inventory overflow")?;
        paths.push(path);
        if paths.len() > MAX_FUNDING_RECORDS || total > MAX_FUNDING_INVENTORY_BYTES {
            return Err("funding inventory exceeds bound".to_owned());
        }
    }
    paths.sort();
    Ok(paths)
}

fn retain_funding_receipts(config: &Config) -> Result<(), Response> {
    let unavailable = || refusal(503, "journal_unavailable", Some(5));
    let intent_paths =
        funding_inventory(&config.state_dir.join("funding-intents")).map_err(|_| unavailable())?;
    for path in
        funding_inventory(&config.state_dir.join("funding-receipts")).map_err(|_| unavailable())?
    {
        let counterpart = config
            .state_dir
            .join("funding-intents")
            .join(path.file_name().ok_or_else(unavailable)?);
        if !intent_paths.contains(&counterpart) {
            return Err(unavailable());
        }
    }
    for path in funding_inventory(&config.state_dir.join("journal")).map_err(|_| unavailable())? {
        if path.extension().and_then(|extension| extension.to_str()) != Some("json") {
            continue;
        }
        let entry = journal_read(&path)
            .map_err(|_| unavailable())?
            .ok_or_else(unavailable)?;
        if serde_json::from_str::<PreparedFunding>(&entry.body).is_ok()
            || serde_json::from_str::<chain_fund::SignedFunding>(&entry.body).is_ok()
            || serde_json::from_str::<chain_fund::NativeTransfer>(&entry.body).is_ok()
        {
            let counterpart = config
                .state_dir
                .join("funding-intents")
                .join(path.file_name().ok_or_else(unavailable)?);
            if !intent_paths.contains(&counterpart) {
                return Err(unavailable());
            }
        }
    }
    let started = Instant::now();
    for path in intent_paths {
        let bytes = funding_read(&path)
            .map_err(|_| unavailable())?
            .ok_or_else(unavailable)?;
        if funding_native::is_native(&bytes) {
            funding_native::retain(config, &bytes)?;
            continue;
        }
        let intent: FundingIntent = serde_json::from_slice(&bytes).map_err(|_| unavailable())?;
        validate_funding_intent(config, &intent).map_err(|_| unavailable())?;
        if path != funding_record_path(config, "funding-intents", &intent.key) {
            return Err(unavailable());
        }
        let archive_path = funding_record_path(config, "funding-receipts", &intent.key);
        let archive = if let Some(bytes) = funding_read(&archive_path).map_err(|_| unavailable())? {
            serde_json::from_slice::<FundingArchive>(&bytes).map_err(|_| unavailable())?
        } else {
            let remaining = config
                .receipt_deadline
                .checked_sub(started.elapsed())
                .ok_or_else(|| refusal(503, "receipt_unavailable", Some(5)))?;
            let lookup =
                await_authenticated_receipt(config, intent.signed.transaction_hash, remaining)
                    .map_err(|_| refusal(503, "receipt_unavailable", Some(5)))?;
            let layerx_client::rpc::receipt::Lookup::Verified(receipt) = lookup else {
                return Err(refusal(503, "receipt_unavailable", Some(5)));
            };
            if receipt.execution().signing_key != intent.sequencer_key {
                return Err(refusal(503, "receipt_unavailable", Some(5)));
            }
            let receipt = receipt.canonical_bytes().to_vec();
            FundingArchive {
                version: 1,
                intent_digest: hex_encode(&Sha256::digest(&bytes)),
                receipt,
            }
        };
        funding_archive_facts(config, &intent, &bytes, &archive).map_err(|_| unavailable())?;
        funding_store(
            &archive_path,
            &serde_json::to_vec(&archive).map_err(|_| unavailable())?,
        )
        .map_err(|_| unavailable())?;
        let stage_path = journal_path(config, "fund-canonical", &intent.key);
        let mut stage = journal_read(&stage_path)
            .map_err(|_| unavailable())?
            .ok_or_else(unavailable)?;
        let signed: PreparedFunding =
            serde_json::from_str(&stage.body).map_err(|_| unavailable())?;
        if signed.canonical != intent.signed.canonical
            || signed.transaction_hash != intent.signed.transaction_hash
            || signed.sender != intent.signed.sender
            || stage.request_digest
                != hex_encode(&Sha256::digest(
                    serde_json::to_vec(&intent.command).map_err(|_| unavailable())?,
                ))
        {
            return Err(unavailable());
        }
        if stage.status != 200 {
            stage.status = 200;
            journal_write(&stage_path, &stage).map_err(|_| unavailable())?;
        }
    }
    Ok(())
}

fn funding_receipt_response(
    command: &FundingCommand,
    facts: &ReceiptFacts,
) -> Result<Response, Response> {
    if facts.result_code != 0 {
        return Err(refusal(422, "send_refused", None));
    }
    Ok(json_response(
        200,
        &serde_json::json!({
            "funding_id": command.funding_id,
            "state": "funded",
            "transaction_id": hex_encode(&facts.activity_id),
        }),
    ))
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SupervisorReply {
    state: Option<String>,
    reset_id: Option<String>,
    generation: Option<u64>,
    error: Option<SupervisorError>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct SupervisorError {
    code: String,
    retry: String,
    retry_after_seconds: Option<u64>,
}

#[derive(Clone, Copy, serde::Serialize)]
struct SupervisorRequest<'a> {
    version: u8,
    operation: &'a str,
    reset_id: &'a str,
    network: u32,
    request_digest: &'a str,
}

fn reset_identity(config: &Config, request: &Request, key: &str) -> (String, String) {
    let mut identity = Sha256::new();
    identity.update(b"layerx-core-reset/v1\0");
    identity.update(config.network_id.to_be_bytes());
    identity.update((key.len() as u64).to_be_bytes());
    identity.update(key.as_bytes());
    identity.update(request_digest(request).as_bytes());
    let identity = identity.finalize();
    let reset_id = hex_encode(&identity[..16]);
    let canonical = format!(
        "{{\"network\":{},\"operation\":\"reset\",\"reset_id\":\"{}\",\"version\":1}}",
        config.network_id, reset_id
    );
    (reset_id, hex_encode(&Sha256::digest(canonical.as_bytes())))
}

fn supervisor_exchange(
    config: &Config,
    request: &SupervisorRequest<'_>,
    attempted: &mut bool,
) -> Result<SupervisorReply, String> {
    let mut stream =
        UnixStream::connect(&config.supervisor_socket).map_err(|error| error.to_string())?;
    stream
        .set_read_timeout(Some(RESET_TIMEOUT))
        .map_err(|error| error.to_string())?;
    stream
        .set_write_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    let mut bytes = serde_json::to_vec(request).map_err(|error| error.to_string())?;
    bytes.push(b'\n');
    *attempted = request.operation == "reset";
    stream
        .write_all(&bytes)
        .map_err(|error| error.to_string())?;
    stream.flush().map_err(|error| error.to_string())?;
    let mut line = String::new();
    BufReader::new(stream)
        .take(4097)
        .read_line(&mut line)
        .map_err(|error| error.to_string())?;
    if line.len() > 4096 || !line.ends_with('\n') {
        return Err("supervisor response framing differs".to_owned());
    }
    serde_json::from_str(line.trim_end_matches(['\r', '\n'])).map_err(|error| error.to_string())
}

fn reset_pending(reset_id: &str) -> Response {
    let mut response = json_response(
        202,
        &serde_json::json!({
            "state": "pending", "reset_id": reset_id,
        }),
    );
    response.retry_after = Some(30);
    response.authoritative_replay = true;
    response
}

fn reset_authoritative_reply(reply: &SupervisorReply, reset_id: &str) -> Option<Response> {
    if reply.error.is_some()
        || reply.reset_id.as_deref() != Some(reset_id)
        || !reply
            .generation
            .is_some_and(|value| value > 0 && i64::try_from(value).is_ok())
    {
        return None;
    }
    match reply.state.as_deref()? {
        "reset" => {
            let mut response = json_response(
                200,
                &serde_json::json!({
                    "state": "reset", "reset_id": reset_id,
                }),
            );
            response.authoritative_replay = true;
            Some(response)
        }
        "admitted" | "stopping" | "bootstrapping" | "prepared" | "activating" | "ambiguous" => {
            Some(reset_pending(reset_id))
        }
        _ => None,
    }
}

fn reset_pre_admission_refusal(reply: &SupervisorReply) -> Option<Response> {
    if reply.state.is_some() || reply.reset_id.is_some() || reply.generation.is_some() {
        return None;
    }
    let error = reply.error.as_ref()?;
    if error.retry != "never" || error.retry_after_seconds != Some(0) {
        return None;
    }
    match error.code.as_str() {
        "wrong_network"
        | "digest_mismatch"
        | "unsupported_version"
        | "unauthorized_peer"
        | "identity_conflict"
        | "reset_in_progress"
        | "state_capacity"
        | "generation_exhausted"
        | "invalid_request"
        | "invalid_operation"
        | "invalid_reset_id"
        | "invalid_digest" => Some(refusal(503, &error.code, None)),
        _ => None,
    }
}

fn reset(config: &Config, request: &Request, key: &str) -> Response {
    let (reset_id, digest) = reset_identity(config, request, key);
    let status_request = SupervisorRequest {
        version: 1,
        operation: "status",
        reset_id: &reset_id,
        network: config.network_id,
        request_digest: &digest,
    };
    let path = journal_path(config, "reset", key);
    let previous_attempt = match journal_read(&path) {
        Ok(Some(entry)) => entry.status == 202,
        Ok(None) | Err(_) => return refusal(503, "journal_unavailable", Some(5)),
    };
    if !previous_attempt {
        if let Err(response) = retain_funding_receipts(config) {
            return response;
        }
    }
    let mut attempted = false;
    match supervisor_exchange(config, &status_request, &mut attempted) {
        Ok(reply) => {
            if let Some(response) = reset_authoritative_reply(&reply, &reset_id) {
                return response;
            }
            if let Some(response) = reset_pre_admission_refusal(&reply) {
                return response;
            }
            if !reply.error.as_ref().is_some_and(|error| {
                error.code == "unknown_reset"
                    && error.retry == "never"
                    && error.retry_after_seconds == Some(0)
            }) {
                return if previous_attempt {
                    reset_pending(&reset_id)
                } else {
                    refusal(503, "reset_failed", Some(30))
                };
            }
        }
        Err(error) => {
            eprintln!("layerx-core-boundary: supervisor status: {error}");
            return if previous_attempt {
                reset_pending(&reset_id)
            } else {
                refusal(503, "supervisor_unavailable", Some(30))
            };
        }
    }
    if previous_attempt {
        if let Err(response) = retain_funding_receipts(config) {
            return response;
        }
    }
    let pending = reset_pending(&reset_id);
    if journal_write(
        &path,
        &JournalEntry {
            request_digest: request_digest(request),
            status: pending.status,
            body: pending.body.clone(),
            retry_after: pending.retry_after,
        },
    )
    .is_err()
    {
        return refusal(503, "journal_unavailable", Some(5));
    }
    let admission = SupervisorRequest {
        operation: "reset",
        ..status_request
    };
    match supervisor_exchange(config, &admission, &mut attempted) {
        Ok(reply) => {
            if let Some(response) = reset_authoritative_reply(&reply, &reset_id) {
                return response;
            }
            if let Some(response) = reset_pre_admission_refusal(&reply) {
                return response;
            }
        }
        Err(error) => {
            eprintln!("layerx-core-boundary: supervisor admission: {error}");
            if !attempted {
                return refusal(503, "supervisor_unavailable", Some(30));
            }
        }
    }
    if let Ok(reply) = supervisor_exchange(config, &status_request, &mut attempted) {
        if let Some(response) = reset_authoritative_reply(&reply, &reset_id) {
            return response;
        }
    }
    pending
}

fn admin_result(mut response: Response) -> Response {
    let storage_failure = serde_json::from_str::<serde_json::Value>(&response.body)
        .ok()
        .is_some_and(|body| body["error"]["code"] == "journal_unavailable");
    if response.status >= 500 && !storage_failure {
        response.status = 422;
    }
    response
}

fn admin_route(config: &Config, request: &Request) -> Response {
    match (request.method.as_str(), request.path.as_str()) {
        ("GET", "/livez") => return json_response(200, &serde_json::json!({ "live": true })),
        ("GET", "/readyz") => return readiness(config),
        _ => {}
    }
    if !admin_authorized(config, request) {
        return refusal(401, "unauthorized", None);
    }
    if request.query.is_some() {
        return refusal(400, "invalid_request", None);
    }
    if request.path == "/admin/v1/testnet/funding-intents"
        || request
            .path
            .starts_with("/admin/v1/testnet/funding-intents/")
    {
        return funding_intent_route(config, request);
    }
    if request.method != "POST" {
        return match request.path.as_str() {
            "/admin/v1/testnet/fund" | "/admin/v1/testnet/reset" => {
                refusal(405, "method_not_allowed", None)
            }
            _ => refusal(404, "not_found", None),
        };
    }
    if request.headers.get("content-type").map(String::as_str) != Some("application/json") {
        return refusal(400, "content_type_required", None);
    }
    let Some(key) = request.headers.get("idempotency-key").cloned() else {
        return refusal(400, "idempotency_key_required", None);
    };
    if !valid_key(&key) {
        return refusal(400, "invalid_idempotency_key", None);
    }
    let Ok(_guard) = config.admin_lock.lock() else {
        return refusal(503, "admin_unavailable", Some(5));
    };
    match request.path.as_str() {
        "/admin/v1/testnet/fund" => {
            match archived_funding_response(config, request, &key) {
                Ok(Some(response)) | Err(response) => return response,
                Ok(None) => {}
            }
            stateful(config, "fund", request, || fund(config, request, &key))
        }
        "/admin/v1/testnet/reset" => {
            if request.body != b"{}" {
                return refusal(400, "invalid_argument", None);
            }
            stateful(config, "reset", request, || {
                admin_result(reset(config, request, &key))
            })
        }
        _ => refusal(404, "not_found", None),
    }
}

fn handle_connection(config: &Arc<Config>, plane: Plane, tcp: TcpStream) -> Result<(), String> {
    tcp.set_nodelay(true).map_err(|error| error.to_string())?;
    tcp.set_read_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    tcp.set_write_timeout(Some(IO_TIMEOUT))
        .map_err(|error| error.to_string())?;
    let tls = match plane {
        Plane::Core => &config.tls,
        Plane::Admin => &config.admin_tls,
    };
    let connection = ServerConnection::new(Arc::clone(tls)).map_err(|error| error.to_string())?;
    let mut stream = StreamOwned::new(connection, tcp);
    for request_number in 0..MAX_REQUESTS_PER_CONNECTION {
        let request = match parse_client_request(&mut stream) {
            Ok(request) => request,
            Err(_) if request_number == 0 => {
                write_response(&mut stream, &refusal(400, "invalid_request", None), false)?;
                break;
            }
            Err(_) => break,
        };
        let keep_alive = request_number + 1 < MAX_REQUESTS_PER_CONNECTION
            && request
                .headers
                .get("connection")
                .is_none_or(|value| !value.eq_ignore_ascii_case("close"));
        let response = match plane {
            Plane::Core => core_route(config, &request),
            Plane::Admin => admin_route(config, &request),
        };
        write_response(&mut stream, &response, keep_alive)?;
        stream.flush().map_err(|error| error.to_string())?;
        if !keep_alive {
            break;
        }
    }
    stream.conn.send_close_notify();
    let _ = stream.flush();
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

fn serve(config: &Arc<Config>, plane: Plane, listener: &TcpListener) {
    for connection in listener.incoming() {
        match connection {
            Ok(stream) => {
                let Some(permit) = ConnectionPermit::acquire() else {
                    continue;
                };
                let shared = Arc::clone(config);
                thread::spawn(move || {
                    let _permit = permit;
                    if let Err(error) = handle_connection(&shared, plane, stream) {
                        eprintln!("layerx-core-boundary connection failed: {error}");
                    }
                });
            }
            Err(error) => eprintln!("layerx-core-boundary accept failed: {error}"),
        }
    }
}

fn platform_core(config: Config) -> Result<(), String> {
    let listener = TcpListener::bind(config.listen).map_err(|error| error.to_string())?;
    let admin = TcpListener::bind(config.admin_listen).map_err(|error| error.to_string())?;
    let config = Arc::new(config);
    eprintln!("layerx-core-boundary listening with TLS on the core and admin planes");
    let admin_config = Arc::clone(&config);
    let admin_thread = thread::spawn(move || serve(&admin_config, Plane::Admin, &admin));
    serve(&config, Plane::Core, &listener);
    admin_thread
        .join()
        .map_err(|_| "admin listener thread panicked".to_owned())
}

pub fn main() {
    if let Err(error) = config().and_then(platform_core) {
        eprintln!("layerx-core-boundary: {error}");
        std::process::exit(2);
    }
}

#[cfg(test)]
mod journal_tests {
    use super::{journal_read, journal_write, JournalEntry, TRACE};
    use std::fs;
    use std::io::Write;
    use std::sync::atomic::Ordering;

    fn entry(status: u16, body: &str) -> JournalEntry {
        JournalEntry {
            request_digest: "request".to_owned(),
            status,
            body: body.to_owned(),
            retry_after: None,
        }
    }

    #[test]
    fn append_journal_recovers_last_complete_transition_and_legacy_record() {
        let path = std::env::temp_dir().join(format!(
            "layerx-core-journal-{}-{}.json",
            std::process::id(),
            TRACE.fetch_add(1, Ordering::AcqRel)
        ));
        journal_write(&path, &entry(202, "pending"))
            .unwrap_or_else(|error| panic!("pending journal: {error}"));
        journal_write(&path, &entry(200, "complete"))
            .unwrap_or_else(|error| panic!("complete journal: {error}"));
        let mut file = fs::OpenOptions::new()
            .append(true)
            .open(&path)
            .unwrap_or_else(|error| panic!("open journal: {error}"));
        file.write_all(b"{\"request_digest\":")
            .unwrap_or_else(|error| panic!("partial journal: {error}"));
        file.sync_data()
            .unwrap_or_else(|error| panic!("sync partial journal: {error}"));
        let recovered = journal_read(&path)
            .unwrap_or_else(|error| panic!("read journal: {error}"))
            .unwrap_or_else(|| panic!("journal is missing"));
        assert_eq!(recovered.status, 200);
        assert_eq!(recovered.body, "complete");
        journal_write(&path, &entry(200, "reconciled"))
            .unwrap_or_else(|error| panic!("repair torn tail: {error}"));
        let reconciled = journal_read(&path)
            .unwrap_or_else(|error| panic!("read repaired journal: {error}"))
            .unwrap_or_else(|| panic!("repaired journal is missing"));
        assert_eq!(reconciled.body, "reconciled");

        let legacy = serde_json::to_vec(&entry(202, "legacy"))
            .unwrap_or_else(|error| panic!("legacy journal: {error}"));
        fs::write(&path, legacy).unwrap_or_else(|error| panic!("write legacy journal: {error}"));
        journal_write(&path, &entry(200, "migrated"))
            .unwrap_or_else(|error| panic!("migrate journal: {error}"));
        let migrated = journal_read(&path)
            .unwrap_or_else(|error| panic!("read migrated journal: {error}"))
            .unwrap_or_else(|| panic!("migrated journal is missing"));
        assert_eq!(migrated.status, 200);
        assert_eq!(migrated.body, "migrated");
        fs::remove_file(&path).unwrap_or_else(|error| panic!("remove journal: {error}"));
    }
}

#[cfg(test)]
mod fund_refusal_tests {
    use super::{admin_result, refusal, send_refusal, Response};
    use layerx_platform_core::SendError;

    fn code(response: &Response) -> String {
        let document: serde_json::Value = serde_json::from_str(&response.body)
            .unwrap_or_else(|error| panic!("refusal body {}: {error}", response.body));
        document["error"]["code"]
            .as_str()
            .unwrap_or_else(|| panic!("refusal body {}", response.body))
            .to_owned()
    }

    #[test]
    fn a_treasury_signer_outage_is_a_retryable_refusal_the_admin_plane_delivers_as_422() {
        let refused = send_refusal(SendError::Signer(
            "treasury signer socket /run/layerx/node/treasury-signer.sock is not available"
                .to_owned(),
        ));
        assert_eq!(refused.status, 503);
        assert_eq!(refused.retry_after, Some(5));
        assert_eq!(code(&refused), "treasury_signer_unavailable");
        let delivered = admin_result(refused);
        assert_eq!(
            delivered.status, 422,
            "every admin 5xx is delivered as 422: {}",
            delivered.body
        );
        assert_eq!(delivered.retry_after, Some(5));
        assert_eq!(code(&delivered), "treasury_signer_unavailable");
    }

    #[test]
    fn an_unbuildable_send_is_a_terminal_refusal_on_both_planes() {
        let refused = send_refusal(SendError::Invalid(
            "amount exceeds the fee limit".to_owned(),
        ));
        assert_eq!(refused.status, 422);
        assert_eq!(refused.retry_after, None);
        assert_eq!(code(&refused), "send_unbuildable");
        let delivered = admin_result(refused);
        assert_eq!(delivered.status, 422);
        assert_eq!(delivered.retry_after, None);
        assert_eq!(code(&delivered), "send_unbuildable");
    }
    #[test]
    fn admin_storage_failure_retains_503_while_dependency_refusal_is_422() {
        let storage = admin_result(refusal(503, "journal_unavailable", Some(5)));
        assert_eq!(storage.status, 503);
        assert_eq!(storage.retry_after, Some(5));
        let dependency = admin_result(refusal(503, "node_unavailable", Some(5)));
        assert_eq!(dependency.status, 422);
    }
}
