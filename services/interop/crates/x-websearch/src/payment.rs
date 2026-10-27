//! Pay per request over 402LXP. Paid routes offer SID, PAX, USDC and USDL
//! as metered draws and exact payments. The receiver signs metered draws
//! as legacy EIP-155 transactions to the ledger precompile, journalling
//! their raw bytes and hash before broadcast. The payment store binds each
//! payment, idempotency key and receipt to one request.

use std::collections::BTreeMap;
use std::fs;
use std::io::{self, Read as _, Write as _};
use std::net::{TcpStream, ToSocketAddrs as _};
use std::os::unix::fs::{DirBuilderExt as _, OpenOptionsExt as _};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, PoisonError};
use std::time::{Duration, SystemTime, UNIX_EPOCH};

use base64::engine::general_purpose::STANDARD;
use base64::Engine as _;
use k256::ecdsa::SigningKey;
use layerx_crypto::payments::{Grant, Payment};
use layerx_interop_gateway::adapter::ConformanceSuite;
use layerx_interop_gateway::evidence::{
    canonical_receipt_digest, SequencerEvidence, SettlementRung,
};
use layerx_interop_gateway::principal::PrincipalId;
use layerx_interop_gateway::trace::TraceId;
use layerx_interop_gateway::GatewayCore;
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::merkle::leaf_hash;
use layerx_proof::receipt::{verify_sequencer_signature, ReceiptKeys};
use layerx_proof::sequencer_logs::{verify_logs, SequencerLog};
use layerx_types::payload::ModuleId;
pub use layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION as PROTOCOL_VERSION;
use layerx_x402::model::{
    account_identifiers, AtomicAmount, PaymentRequired, PaymentRequirements, ResourceInfo,
    X402_VERSION,
};
use layerx_x402::seller::{
    ExecutedPayment, LayerXPaymentRequest, PaymentPlane, PlanePaymentOutcome, Seller, SellerOutcome,
};
use layerx_x402::x402_adapter_descriptor;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};
use zeroize::Zeroizing;

use crate::attest::{sign_digest, signer_address};
use crate::submit::{SignedTransaction, SubmitError};
use crate::watch::{hex0x, keccak, parse_quantity};

use crate::assets::{AcceptedAsset, AcceptedAssets, AssetRefusal};
use crate::config::{payer_did_valid, AssetSymbol, Config};
use crate::server::{Request, Response, Route, RouteError, RouteTable};
use crate::settlement::{SettlementError, SettlementLog, SettlementReceipt, SettlementRpc};

pub const PAYMENT_REQUIRED: &str = "PAYMENT-REQUIRED";
pub const PAYMENT_SIGNATURE: &str = "PAYMENT-SIGNATURE";
pub const PAYMENT_RESPONSE: &str = "PAYMENT-RESPONSE";
/// Names the buyer's DID so metered offers can be bound to its accounts.
pub const PAYER_DID: &str = "LAYERX-PAYER-DID";
pub const COMMITMENT: &str = "executed";
pub const METERED: &str = "metered";
pub const EXACT: &str = "exact";
pub const OFFER_TIMEOUT_SECONDS: u32 = 60;
pub const DRAW_VALIDITY_MS: u64 = 60_000;
pub const GRANT_BYTES: usize = 346;
pub const LEDGER_PRECOMPILE: [u8; 20] = [
    0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0x10, 0x1a,
];
pub const DRAW_SIGNATURE: &str = "draw(bytes32,bytes32,uint128,bytes32,uint64,bytes32,bytes32)";

const PURPOSE_DOMAIN: &[u8] = b"LayerX/x-websearch/v1/purpose\0";
const PRINCIPAL_DOMAIN: &[u8] = b"LayerX/x-websearch/v1/principal\0";
const TRACE_DOMAIN: &[u8] = b"LayerX/x-websearch/v1/trace\0";
const ANONYMOUS_PRINCIPAL: &str = "anonymous-buyer";
const RPC_TIMEOUT: Duration = Duration::from_secs(10);
const PENDING_CODE: i64 = -32001;
const RPC_RESPONSE_LIMIT: usize = 4_194_304;
const RECEIPT_HEX_LIMIT: usize = 2_097_152;
const MAX_HOST_BYTES: usize = 255;

/// Milliseconds since the Unix epoch, as the gate reads its clock.
pub type Clock = fn() -> u64;

#[must_use]
pub fn system_clock() -> u64 {
    SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .map_or(0, |elapsed| {
            u64::try_from(elapsed.as_millis()).unwrap_or(u64::MAX)
        })
}

/// Lowercase hexadecimal.
#[must_use]
pub fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(bytes.len() * 2);
    for byte in bytes {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    text
}

/// Decodes lowercase hexadecimal only.
#[must_use]
pub fn unhex(text: &str) -> Option<Vec<u8>> {
    const fn nibble(byte: u8) -> Option<u8> {
        match byte {
            b'0'..=b'9' => Some(byte - b'0'),
            b'a'..=b'f' => Some(byte - b'a' + 10),
            _ => None,
        }
    }
    if !text.len().is_multiple_of(2) {
        return None;
    }
    text.as_bytes()
        .chunks_exact(2)
        .map(|pair| Some((nibble(pair[0])? << 4) | nibble(pair[1])?))
        .collect()
}

fn unhex32(text: &str) -> Option<[u8; 32]> {
    unhex(text)?.try_into().ok()
}

fn sha256(parts: &[&[u8]]) -> [u8; 32] {
    let mut hash = Sha256::new();
    for part in parts {
        hash.update(part);
    }
    hash.finalize().into()
}

/// The account `did` pays or is paid in `asset`: the main account
/// `agent:<did>:main` for PAX, the kernel's native coin, and the per-asset
/// account for SID, USDC and USDL.
#[must_use]
pub fn wallet_account(did: &str, asset: &AcceptedAsset) -> String {
    if asset.symbol == AssetSymbol::Pax {
        format!("agent:{did}:main")
    } else {
        format!("agent:{did}:asset:{}", hex(&asset.asset_id))
    }
}

/// The account id of a wallet reference.
#[must_use]
pub fn account_id(account: &str) -> Option<[u8; 32]> {
    account_identifiers(account).ok().map(|ids| ids[1])
}

/// The purpose every grant for this receiver's paid routes carries.
#[must_use]
pub fn purpose_hash(receiver_address: &[u8; 20]) -> [u8; 32] {
    sha256(&[PURPOSE_DOMAIN, receiver_address])
}

/// Decodes the ledger's `getPayerGrant` return words, verifying the grant's id and signature.
#[must_use]
pub fn decode_grant(bytes: &[u8], actor: &str) -> Option<Grant> {
    let payload = payer_grant_payload(bytes)?;
    match Payment::decode(ModuleId::Asset, 7, payload, actor.as_bytes()).ok()? {
        Payment::IssueGrant(grant) => Some(grant),
        _ => None,
    }
}

fn payer_grant_payload(bytes: &[u8]) -> Option<&[u8]> {
    const HEAD_BYTES: usize = 7 * 32;
    let head = bytes.get(..HEAD_BYTES)?;
    let word = |index: usize, width: usize| -> Option<u128> {
        let bytes = head.get(index * 32..(index + 1) * 32)?;
        if bytes[..32 - width].iter().any(|byte| *byte != 0) {
            return None;
        }
        Some(u128::from_be_bytes(bytes[16..].try_into().ok()?))
    };
    if word(0, 8)? != HEAD_BYTES as u128 || word(5, 1)? > 1 || word(6, 1)? > 1 {
        return None;
    }
    word(1, 16)?;
    word(2, 16)?;
    word(3, 8)?;
    word(4, 8)?;
    let length_word = bytes.get(HEAD_BYTES..HEAD_BYTES + 32)?;
    if length_word[..24].iter().any(|byte| *byte != 0) {
        return None;
    }
    let length = usize::try_from(u64::from_be_bytes(length_word[24..].try_into().ok()?)).ok()?;
    if length > GRANT_BYTES {
        return None;
    }
    let payload_start = HEAD_BYTES + 32;
    let payload_end = payload_start + length;
    let padded_end = payload_start + length.div_ceil(32) * 32;
    if bytes.len() != padded_end || bytes[payload_end..].iter().any(|byte| *byte != 0) {
        return None;
    }
    bytes.get(payload_start..payload_end)
}

pub fn read_payer_grant(
    rpc: &SettlementRpc,
    grant_id: &[u8; 32],
) -> Result<Option<Grant>, SettlementError> {
    let mut calldata = Vec::with_capacity(36);
    calldata.extend_from_slice(&sha3::Keccak256::digest(b"getPayerGrant(bytes32)")[..4]);
    calldata.extend_from_slice(grant_id);
    let answer = match rpc.eth_call(LEDGER_PRECOMPILE, &calldata) {
        Err(SettlementError::Rpc { code: 3, message })
            if message == "LXP_NO_PAYER_GRANT"
                || message == "execution reverted: LXP_NO_PAYER_GRANT" =>
        {
            return Ok(None);
        }
        result => result?,
    };
    let payload = payer_grant_payload(&answer).ok_or(SettlementError::Malformed)?;
    if payload.is_empty() || (payload.len() == GRANT_BYTES && payload.iter().all(|byte| *byte == 0))
    {
        return Ok(None);
    }
    let grant = decode_grant(&answer, "").ok_or(SettlementError::Malformed)?;
    if grant.id != *grant_id {
        return Err(SettlementError::Malformed);
    }
    Ok(Some(grant))
}

pub fn read_receiver_did(
    rpc: &SettlementRpc,
    address: &[u8; 20],
) -> Result<[u8; 32], SettlementError> {
    let mut precompile = [0; 20];
    precompile[18..].copy_from_slice(&[0x10, 0x04]);
    let data = [&keccak(b"getLayerXDid(address)")[..4], &[0; 12], address].concat();
    let answer = rpc.eth_call(precompile, &data)?;
    if answer.len() != 192 {
        return Err(SettlementError::Malformed);
    }
    let key: [u8; 32] = answer[..32]
        .try_into()
        .map_err(|_| SettlementError::Malformed)?;
    let did = format!("did:layerx:{}", hex(&key));
    if key == [0; 32]
        || answer[32..63].iter().any(|byte| *byte != 0)
        || answer[63] != 64
        || answer[64..95].iter().any(|byte| *byte != 0)
        || usize::from(answer[95]) != did.len()
        || answer[96..96 + did.len()] != *did.as_bytes()
        || answer[96 + did.len()..].iter().any(|byte| *byte != 0)
    {
        return Err(SettlementError::Malformed);
    }
    Ok(key)
}

pub fn read_receiver_sequence(
    rpc: &SettlementRpc,
    did_key: &[u8; 32],
    asset: &[u8; 32],
) -> Result<u64, SettlementError> {
    let data = [
        &keccak(b"nextSequence(bytes32,bytes32)")[..4],
        did_key,
        asset,
    ]
    .concat();
    let answer = rpc.eth_call(LEDGER_PRECOMPILE, &data)?;
    if answer.len() != 32 || answer[..24].iter().any(|byte| *byte != 0) {
        return Err(SettlementError::Malformed);
    }
    Ok(u64::from_be_bytes(
        answer[24..]
            .try_into()
            .map_err(|_| SettlementError::Malformed)?,
    ))
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct LedgerTransaction {
    pub chain_id: u64,
    pub nonce: u64,
    pub gas_price: u128,
    pub gas_limit: u64,
    pub data: Vec<u8>,
}

fn rlp_bytes(bytes: &[u8], list: bool) -> Vec<u8> {
    if !list && bytes.len() == 1 && bytes[0] < 128 {
        return bytes.to_vec();
    }
    let base = if list { 192 } else { 128 };
    let mut out = Vec::new();
    if bytes.len() < 56 {
        out.push(base + bytes.len() as u8);
    } else {
        let length = bytes.len().to_be_bytes();
        let first = length
            .iter()
            .position(|byte| *byte != 0)
            .unwrap_or(length.len());
        out.push(base + 55 + (length.len() - first) as u8);
        out.extend_from_slice(&length[first..]);
    }
    out.extend_from_slice(bytes);
    out
}

fn rlp_integer(bytes: &[u8]) -> Vec<u8> {
    let first = bytes
        .iter()
        .position(|byte| *byte != 0)
        .unwrap_or(bytes.len());
    rlp_bytes(&bytes[first..], false)
}

impl LedgerTransaction {
    pub fn sign(&self, receiver: &SigningKey) -> Result<SignedTransaction, SubmitError> {
        if self.chain_id == 0 || self.gas_limit == 0 || self.gas_price == 0 {
            return Err(SubmitError::Encode);
        }
        let fields = [
            rlp_integer(&self.nonce.to_be_bytes()),
            rlp_integer(&self.gas_price.to_be_bytes()),
            rlp_integer(&self.gas_limit.to_be_bytes()),
            rlp_bytes(&LEDGER_PRECOMPILE, false),
            rlp_integer(&[]),
            rlp_bytes(&self.data, false),
        ]
        .concat();
        let unsigned = [
            fields.clone(),
            rlp_integer(&self.chain_id.to_be_bytes()),
            vec![128, 128],
        ]
        .concat();
        let signature = sign_digest(receiver, &keccak(&rlp_bytes(&unsigned, true)))
            .map_err(|_| SubmitError::Sign)?;
        let v = u128::from(self.chain_id) * 2 + 35 + u128::from(signature[64] - 27);
        let payload = [
            fields,
            rlp_integer(&v.to_be_bytes()),
            rlp_integer(&signature[..32]),
            rlp_integer(&signature[32..64]),
        ]
        .concat();
        let raw = rlp_bytes(&payload, true);
        Ok(SignedTransaction {
            hash: keccak(&raw),
            raw,
        })
    }
}

#[must_use]
pub fn draw_calldata(
    grant_id: &[u8; 32],
    recipient: &[u8; 32],
    amount: u128,
    idempotency_key: &[u8; 32],
    receiver_sequence: u64,
    purpose: &[u8; 32],
    trace: &[u8; 32],
) -> Vec<u8> {
    let mut calldata = Vec::with_capacity(4 + 7 * 32);
    calldata.extend_from_slice(&sha3::Keccak256::digest(DRAW_SIGNATURE.as_bytes())[..4]);
    calldata.extend_from_slice(grant_id);
    calldata.extend_from_slice(recipient);
    calldata.extend_from_slice(&[0; 16]);
    calldata.extend_from_slice(&amount.to_be_bytes());
    calldata.extend_from_slice(idempotency_key);
    calldata.extend_from_slice(&[0; 24]);
    calldata.extend_from_slice(&receiver_sequence.to_be_bytes());
    calldata.extend_from_slice(purpose);
    calldata.extend_from_slice(trace);
    calldata
}

/// One JSON-RPC answer from the gateway.
#[derive(Clone, Debug, PartialEq)]
pub enum RpcAnswer {
    Result(Value),
    Error { code: i64, data: Value },
}

impl RpcAnswer {
    fn pending(&self) -> bool {
        match self {
            Self::Error { code, data } => {
                *code == PENDING_CODE
                    && data.get("state").and_then(Value::as_str) == Some("pending")
            }
            Self::Result(value) => value.get("state").and_then(Value::as_str) == Some("pending"),
        }
    }
}

/// A JSON-RPC client for the configured gateway endpoint: TLS for https and
/// plain HTTP for the loopback endpoints the configuration allows.
#[derive(Clone, Debug)]
pub struct GatewayRpc {
    tls: bool,
    host: String,
    port: u16,
    authority: String,
    path: String,
    authorization_file: Option<PathBuf>,
}

impl GatewayRpc {
    /// # Errors
    /// Refuses an endpoint that is not an http or https URL with a host.
    pub fn new(endpoint: &str) -> Result<Self, GateError> {
        let (tls, rest) = if let Some(rest) = endpoint.strip_prefix("https://") {
            (true, rest)
        } else if let Some(rest) = endpoint.strip_prefix("http://") {
            (false, rest)
        } else {
            return Err(GateError::Endpoint);
        };
        if endpoint.chars().any(char::is_whitespace) || endpoint.contains(['@', '#', '?']) {
            return Err(GateError::Endpoint);
        }
        let (authority, path) = rest
            .split_once('/')
            .map_or((rest, "/".to_owned()), |(authority, path)| {
                (authority, format!("/{path}"))
            });
        let default_port = if tls { 443 } else { 80 };
        let (host, port) = if let Some(bracketed) = authority.strip_prefix('[') {
            let (host, tail) = bracketed.split_once(']').ok_or(GateError::Endpoint)?;
            let port = match tail.strip_prefix(':') {
                Some(port) => port.parse().map_err(|_| GateError::Endpoint)?,
                None if tail.is_empty() => default_port,
                None => return Err(GateError::Endpoint),
            };
            (host, port)
        } else if let Some((host, port)) = authority.rsplit_once(':') {
            (host, port.parse().map_err(|_| GateError::Endpoint)?)
        } else {
            (authority, default_port)
        };
        if host.is_empty() {
            return Err(GateError::Endpoint);
        }
        Ok(Self {
            tls,
            host: host.to_owned(),
            port,
            authority: authority.to_owned(),
            path,
            authorization_file: None,
        })
    }

    pub fn with_authorization_file(mut self, path: &Path) -> Result<Self, GateError> {
        if !self.tls
            && self
                .host
                .parse::<std::net::IpAddr>()
                .is_ok_and(|ip| !ip.is_loopback())
        {
            return Err(GateError::Authorization);
        }
        if !self.tls && self.host.parse::<std::net::IpAddr>().is_err() && self.host != "localhost" {
            return Err(GateError::Authorization);
        }
        let _ = gateway_authorization(path)?;
        self.authorization_file = Some(path.to_owned());
        Ok(self)
    }

    /// Calls one method. `None` means the answer is unknown: the gateway was
    /// unreachable or its response was not a well-formed JSON-RPC answer.
    #[must_use]
    pub fn call(&self, method: &str, params: &Value) -> Option<RpcAnswer> {
        let body =
            json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).to_string();
        let authorization = match &self.authorization_file {
            Some(path) => Some(gateway_authorization(path).ok()?),
            None => None,
        };
        let header = Zeroizing::new(authorization.as_ref().map_or_else(String::new, |value| {
            format!("Authorization: {}\r\n", value.as_str())
        }));
        let authorization_header = header.as_str();
        let request = Zeroizing::new(format!(
            "POST {} HTTP/1.1\r\nHost: {}\r\n{authorization_header}Content-Type: application/json\r\nAccept: application/json\r\nConnection: close\r\nContent-Length: {}\r\n\r\n{body}",
            self.path,
            self.authority,
            body.len()
        ));
        let address = (self.host.as_str(), self.port)
            .to_socket_addrs()
            .ok()?
            .next()?;
        if self.authorization_file.is_some() && !self.tls && !address.ip().is_loopback() {
            return None;
        }
        let stream = TcpStream::connect_timeout(&address, RPC_TIMEOUT).ok()?;
        stream.set_read_timeout(Some(RPC_TIMEOUT)).ok()?;
        stream.set_write_timeout(Some(RPC_TIMEOUT)).ok()?;
        let raw = if self.tls {
            let connector = native_tls::TlsConnector::new().ok()?;
            let mut stream = connector.connect(&self.host, stream).ok()?;
            exchange(&mut stream, request.as_bytes())?
        } else {
            let mut stream = stream;
            exchange(&mut stream, request.as_bytes())?
        };
        decode_answer(&decode_http(&raw)?)
    }
}

fn gateway_authorization(path: &Path) -> Result<Zeroizing<String>, GateError> {
    use std::os::unix::fs::MetadataExt as _;
    if !path.is_absolute()
        || path.components().any(|part| {
            part.as_os_str()
                .to_str()
                .is_some_and(|part| part == ".env" || part.starts_with(".env."))
        })
    {
        return Err(GateError::Authorization);
    }
    if fs::canonicalize(path).map_err(|_| GateError::Authorization)? != path {
        return Err(GateError::Authorization);
    }
    let file = fs::OpenOptions::new()
        .read(true)
        .custom_flags(0x20000 | 0x800)
        .open(path)
        .map_err(|_| GateError::Authorization)?;
    let metadata = file.metadata().map_err(|_| GateError::Authorization)?;
    let owner = fs::metadata("/proc/self")
        .map_err(|_| GateError::Authorization)?
        .uid();
    if !metadata.is_file()
        || metadata.nlink() != 1
        || metadata.uid() != owner
        || metadata.mode() & 0o077 != 0
        || metadata.len() > 256
    {
        return Err(GateError::Authorization);
    }
    let mut bytes = Zeroizing::new(Vec::new());
    file.take(257)
        .read_to_end(&mut bytes)
        .map_err(|_| GateError::Authorization)?;
    if bytes.len() > 256 {
        return Err(GateError::Authorization);
    }
    let text = std::str::from_utf8(&bytes).map_err(|_| GateError::Authorization)?;
    let text = text
        .strip_suffix("\r\n")
        .or_else(|| text.strip_suffix('\n'))
        .unwrap_or(text);
    let (id, secret) = text
        .strip_prefix("LayerX-Key ")
        .and_then(|value| value.split_once(':'))
        .ok_or(GateError::Authorization)?;
    if id.is_empty()
        || id.len() > 64
        || !id
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'-' | b'_'))
        || !secret.strip_prefix("lxp_live_").is_some_and(|value| {
            value.len() == 64
                && value
                    .bytes()
                    .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
        })
    {
        return Err(GateError::Authorization);
    }
    Ok(Zeroizing::new(text.to_owned()))
}

fn exchange(stream: &mut (impl io::Read + io::Write), request: &[u8]) -> Option<Vec<u8>> {
    stream.write_all(request).ok()?;
    stream.flush().ok()?;
    let mut raw = Vec::new();
    let limit = u64::try_from(RPC_RESPONSE_LIMIT).ok()?.saturating_add(1);
    stream.take(limit).read_to_end(&mut raw).ok()?;
    (raw.len() <= RPC_RESPONSE_LIMIT).then_some(raw)
}

fn decode_http(raw: &[u8]) -> Option<Value> {
    let split = raw.windows(4).position(|window| window == b"\r\n\r\n")?;
    let head = std::str::from_utf8(&raw[..split]).ok()?;
    let mut lines = head.split("\r\n");
    let status = lines.next()?;
    if !status.starts_with("HTTP/1.") || status.split_whitespace().nth(1) != Some("200") {
        return None;
    }
    let mut length = None;
    let mut chunked = false;
    for line in lines {
        let (name, value) = line.split_once(':')?;
        if name.eq_ignore_ascii_case("transfer-encoding") {
            if chunked || !value.trim().eq_ignore_ascii_case("chunked") {
                return None;
            }
            chunked = true;
        }
        if name.eq_ignore_ascii_case("content-length") {
            if length.is_some() {
                return None;
            }
            length = Some(value.trim().parse::<usize>().ok()?);
        }
    }
    let body = &raw[split + 4..];
    if chunked {
        if length.is_some() {
            return None;
        }
        return serde_json::from_slice(&dechunk(body)?).ok();
    }
    if length.is_some_and(|length| length != body.len()) {
        return None;
    }
    serde_json::from_slice(body).ok()
}

/// Joins the chunks of a chunked transfer encoding body, refusing a body
/// that is cut short, carries anything after its last chunk, or names a
/// chunk size that is not hexadecimal.
fn dechunk(mut body: &[u8]) -> Option<Vec<u8>> {
    let mut joined = Vec::new();
    loop {
        let line_end = body.windows(2).position(|window| window == b"\r\n")?;
        let size_line = std::str::from_utf8(&body[..line_end]).ok()?;
        let size_text = size_line.split(';').next()?.trim();
        let size = usize::from_str_radix(size_text, 16).ok()?;
        body = &body[line_end + 2..];
        if size == 0 {
            loop {
                let trailer_end = body.windows(2).position(|window| window == b"\r\n")?;
                let trailer = &body[..trailer_end];
                body = &body[trailer_end + 2..];
                if trailer.is_empty() {
                    break;
                }
            }
            return body.is_empty().then_some(joined);
        }
        joined.extend_from_slice(body.get(..size)?);
        body = body.get(size..)?.strip_prefix(b"\r\n")?;
    }
}

fn decode_answer(value: &Value) -> Option<RpcAnswer> {
    if value.get("jsonrpc") != Some(&json!("2.0"))
        || value.get("id") != Some(&json!(1))
        || value.get("result").is_some() == value.get("error").is_some()
    {
        return None;
    }
    if let Some(error) = value.get("error") {
        return Some(RpcAnswer::Error {
            code: error.get("code")?.as_i64()?,
            data: error.get("data").cloned().unwrap_or(Value::Null),
        });
    }
    value.get("result").cloned().map(RpcAnswer::Result)
}

/// What the data directory keeps for one payment, keyed by the seller's
/// idempotency key.
#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct PaymentRecord {
    pub principal: String,
    pub request_digest: String,
    pub resource: String,
    pub offer: Value,
    pub payer: Option<String>,
    pub receive_key: Option<String>,
    pub tx_hash: Option<String>,
    pub raw_transaction: Option<String>,
    pub attempted: bool,
    pub receipt: Option<String>,
    pub receipt_digest: Option<String>,
    pub released: bool,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct LegacyPaymentRecord {
    principal: String,
    request_digest: String,
    resource: String,
    offer: Value,
    payer: Option<String>,
    receive_key: Option<String>,
    activity: Option<String>,
    activity_id: Option<String>,
    attempted: bool,
    receipt: Option<String>,
    receipt_digest: Option<String>,
    released: bool,
}

impl LegacyPaymentRecord {
    fn into_current(self) -> io::Result<PaymentRecord> {
        if self.attempted && !self.released {
            return Err(io::Error::new(
                io::ErrorKind::InvalidData,
                "unreleased kernel activity",
            ));
        }
        let _ = (self.activity, self.activity_id);
        Ok(PaymentRecord {
            principal: self.principal,
            request_digest: self.request_digest,
            resource: self.resource,
            offer: self.offer,
            payer: self.payer,
            receive_key: self.receive_key,
            tx_hash: None,
            raw_transaction: None,
            attempted: self.attempted,
            receipt: self.receipt,
            receipt_digest: self.receipt_digest,
            released: self.released,
        })
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PaymentExactLog {
    pub payer: [u8; 32],
    pub payee: [u8; 32],
    pub reference: [u8; 32],
    pub asset: [u8; 32],
    pub amount: u128,
}

pub fn decode_payment_exact(log: &SettlementLog) -> Option<PaymentExactLog> {
    if log.address != LEDGER_PRECOMPILE
        || log.topics.len() != 4
        || log.topics[0] != keccak(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")
        || log.data.len() != 64
        || log.data[32..48] != [0; 16]
    {
        return None;
    }
    Some(PaymentExactLog {
        payer: log.topics[2],
        payee: log.topics[3],
        reference: log.topics[1],
        asset: log.data[..32].try_into().ok()?,
        amount: u128::from_be_bytes(log.data[48..64].try_into().ok()?),
    })
}

/// File-backed payment associations under `<data_dir>/payments`.
#[derive(Clone, Debug)]
pub struct PaymentStore {
    root: PathBuf,
}

const REQUESTS: &str = "requests";
const RECEIVE_KEYS: &str = "receive-keys";
const RECEIPTS: &str = "receipts";
const DELIVERIES: &str = "deliveries";

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum DeliveryState {
    Pending,
    Computing,
    Failed,
    Ready,
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct DeliveryResponse {
    status: u16,
    content_type: String,
    headers: Vec<(String, String)>,
    body: Vec<u8>,
    body_digest: String,
    canonical_digest: Option<String>,
    canonical_content: Option<Vec<u8>>,
}

impl DeliveryResponse {
    fn from_response(response: Response, canonical_content: Option<Vec<u8>>) -> Self {
        Self {
            canonical_digest: canonical_content
                .as_ref()
                .map(|bytes| hex(&crate::canonical::content_digest(bytes))),
            canonical_content,
            body_digest: hex(&sha256(&[&response.body])),
            status: response.status,
            content_type: response.content_type,
            headers: response.headers,
            body: response.body,
        }
    }

    fn response(&self) -> io::Result<Response> {
        if self.body_digest != hex(&sha256(&[&self.body])) || !(100..=599).contains(&self.status) {
            return Err(io::ErrorKind::InvalidData.into());
        }
        let digests: Vec<_> = self
            .headers
            .iter()
            .filter(|(name, _)| name.eq_ignore_ascii_case("Content-Digest"))
            .collect();
        if digests.len() != 1
            || digests[0].1 != format!("sha-256=:{}:", STANDARD.encode(sha256(&[&self.body])))
            || self.content_type != "application/json"
        {
            return Err(io::ErrorKind::InvalidData.into());
        }
        match (&self.canonical_content, &self.canonical_digest) {
            (Some(bytes), Some(digest)) => {
                if *digest != hex(&crate::canonical::content_digest(bytes)) {
                    return Err(io::ErrorKind::InvalidData.into());
                }
                validate_canonical_response(&self.body, bytes)?;
            }
            (None, None) if self.status >= 400 => {}
            _ => return Err(io::ErrorKind::InvalidData.into()),
        }
        Ok(Response {
            status: self.status,
            content_type: self.content_type.clone(),
            headers: self.headers.clone(),
            body: self.body.clone(),
        })
    }
}

fn validate_canonical_response(body: &[u8], bytes: &[u8]) -> io::Result<()> {
    let content = crate::canonical::CanonicalContent::parse(bytes)
        .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
    let value: Value =
        serde_json::from_slice(body).map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
    if value.get("digest").and_then(Value::as_str)
        != Some(hex(&crate::canonical::content_digest(bytes)).as_str())
        || value.get("media_type").and_then(Value::as_str) != Some(content.media_type.as_str())
    {
        return Err(io::ErrorKind::InvalidData.into());
    }
    let matches = match content.kind {
        crate::canonical::ContentKind::Search => {
            value
                .get("query")
                .and_then(Value::as_str)
                .map(str::as_bytes)
                == Some(content.payload.as_slice())
                && serde_json::from_str::<Value>(&content.text).ok().as_ref()
                    == value.get("results")
        }
        crate::canonical::ContentKind::Fetch => {
            value.get("url").and_then(Value::as_str).map(str::as_bytes)
                == Some(content.payload.as_slice())
                && value.get("text").and_then(Value::as_str) == Some(content.text.as_str())
                && value.get("length").and_then(Value::as_u64) == Some(content.text.len() as u64)
        }
        crate::canonical::ContentKind::Api => false,
    };
    if matches {
        Ok(())
    } else {
        Err(io::ErrorKind::InvalidData.into())
    }
}

#[derive(Clone, Debug, Eq, PartialEq, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct DeliveryRecord {
    version: u8,
    principal: String,
    request_digest: String,
    resource: String,
    receipt_digest: String,
    state: DeliveryState,
    response: Option<DeliveryResponse>,
    #[serde(default)]
    failures: u32,
    #[serde(default)]
    retry_after_ms: u64,
}

impl DeliveryRecord {
    fn pending(payment: &PaymentRecord, receipt_digest: &[u8; 32]) -> Self {
        Self {
            version: 1,
            principal: payment.principal.clone(),
            request_digest: payment.request_digest.clone(),
            resource: payment.resource.clone(),
            receipt_digest: hex(receipt_digest),
            state: DeliveryState::Pending,
            response: None,
            failures: 0,
            retry_after_ms: 0,
        }
    }

    fn validate(&self, payment: &PaymentRecord) -> io::Result<()> {
        if self.version != 1
            || self.principal != payment.principal
            || self.request_digest != payment.request_digest
            || self.resource != payment.resource
            || payment.receipt.is_none()
            || payment.receipt_digest.as_deref() != Some(self.receipt_digest.as_str())
            || unhex32(&self.receipt_digest).is_none()
        {
            return Err(io::ErrorKind::InvalidData.into());
        }
        if let Some(response) = &self.response {
            let headers: Vec<_> = response
                .headers
                .iter()
                .filter(|(name, _)| name.eq_ignore_ascii_case(PAYMENT_RESPONSE))
                .collect();
            if headers.len() != 1 {
                return Err(io::ErrorKind::InvalidData.into());
            }
            let settlement: layerx_x402::model::SettlementResponse = serde_json::from_slice(
                &STANDARD
                    .decode(&headers[0].1)
                    .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?,
            )
            .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
            let legacy = payment.tx_hash.is_none()
                && payment.released
                && settlement.transaction == format!("lxp:{}", self.receipt_digest)
                && settlement
                    .extensions
                    .get("layerx")
                    .and_then(|value| value.get("verificationLevel"))
                    .and_then(Value::as_str)
                    == Some("sequencer-signed");
            let mut wire_settlement = settlement.clone();
            if legacy {
                wire_settlement.transaction = format!("0x{}", self.receipt_digest);
            }
            wire_settlement
                .validate_wire()
                .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
            let binding = settlement
                .extensions
                .get("x-websearch")
                .ok_or(io::ErrorKind::InvalidData)?;
            let evidence = settlement
                .extensions
                .get("layerx")
                .ok_or(io::ErrorKind::InvalidData)?;
            let receipt = evidence
                .get("receipt")
                .and_then(Value::as_str)
                .and_then(|receipt| STANDARD.decode(receipt).ok())
                .ok_or(io::ErrorKind::InvalidData)?;
            if !settlement.success
                || (!legacy
                    && payment
                        .tx_hash
                        .as_deref()
                        .and_then(unhex32)
                        .is_none_or(|hash| settlement.transaction != hex0x(&hash)))
                || settlement.payer.as_deref() != payment.payer.as_deref()
                || payment.offer.get("network").and_then(Value::as_str)
                    != Some(settlement.network.as_str())
                || settlement
                    .amount
                    .map(|amount| amount.value().to_string())
                    .as_deref()
                    != payment.offer.get("amount").and_then(Value::as_str)
                || evidence.get("receiptDigest").and_then(Value::as_str)
                    != Some(self.receipt_digest.as_str())
                || evidence.get("verificationLevel").and_then(Value::as_str)
                    != Some(if legacy {
                        "sequencer-signed"
                    } else {
                        "receipted"
                    })
                || payment.receipt.as_deref() != Some(hex(&receipt).as_str())
                || binding.get("resource").and_then(Value::as_str) != Some(self.resource.as_str())
                || binding.get("receiptDigest").and_then(Value::as_str)
                    != Some(self.receipt_digest.as_str())
                || binding.get("responseDigest").and_then(Value::as_str)
                    != Some(response.body_digest.as_str())
                || binding.get("contentDigest").and_then(Value::as_str)
                    != response.canonical_digest.as_deref()
                || binding.get("delivery").and_then(Value::as_str)
                    != Some(if self.state == DeliveryState::Ready {
                        "ready"
                    } else {
                        "failed"
                    })
            {
                return Err(io::ErrorKind::InvalidData.into());
            }
            if let Some(bytes) = &response.canonical_content {
                let content = crate::canonical::CanonicalContent::parse(bytes)
                    .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
                let prefix = match content.kind {
                    crate::canonical::ContentKind::Search => "GET /search?q=",
                    crate::canonical::ContentKind::Fetch => "GET /fetch?url=",
                    crate::canonical::ContentKind::Api => {
                        return Err(io::ErrorKind::InvalidData.into())
                    }
                };
                let payload = std::str::from_utf8(&content.payload)
                    .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
                if self.resource != format!("{prefix}{}", encode_query_value(payload)) {
                    return Err(io::ErrorKind::InvalidData.into());
                }
            }
        }
        match (&self.state, &self.response) {
            (DeliveryState::Pending | DeliveryState::Computing, None) => Ok(()),
            (DeliveryState::Failed, Some(response)) if response.status >= 400 => {
                response.response().map(|_| ())
            }
            (DeliveryState::Ready, Some(response)) if (200..300).contains(&response.status) => {
                response.response().map(|_| ())
            }
            _ => Err(io::ErrorKind::InvalidData.into()),
        }
    }
}

impl PaymentStore {
    /// # Errors
    /// Returns the I/O error that prevented creating the store directories.
    pub fn open(data_dir: &Path) -> io::Result<Self> {
        let root = data_dir.join("payments");
        for index in [REQUESTS, RECEIVE_KEYS, RECEIPTS, DELIVERIES] {
            fs::DirBuilder::new()
                .recursive(true)
                .mode(0o700)
                .create(root.join(index))?;
        }
        fs::File::open(&root)?.sync_all()?;
        fs::File::open(data_dir)?.sync_all()?;
        for entry in fs::read_dir(root.join(REQUESTS))? {
            let entry = entry?;
            if entry
                .file_name()
                .to_str()
                .is_some_and(|name| name.ends_with(".staging"))
            {
                continue;
            }
            let bytes = fs::read(entry.path())?;
            if serde_json::from_slice::<PaymentRecord>(&bytes).is_err() {
                let legacy: LegacyPaymentRecord =
                    serde_json::from_slice(&bytes).map_err(|error| {
                        io::Error::new(
                            io::ErrorKind::InvalidData,
                            format!("{}: {error}", entry.path().display()),
                        )
                    })?;
                legacy.into_current().map_err(|error| {
                    io::Error::new(error.kind(), format!("{}: {error}", entry.path().display()))
                })?;
            }
        }
        Ok(Self { root })
    }

    fn path(&self, index: &str, name: &str) -> PathBuf {
        self.root.join(index).join(name)
    }

    /// # Errors
    /// Returns an I/O error, or `InvalidData` for a record that does not parse.
    pub fn load(&self, key: &[u8; 32]) -> io::Result<Option<PaymentRecord>> {
        match fs::read(self.path(REQUESTS, &hex(key))) {
            Ok(bytes) => match serde_json::from_slice(&bytes) {
                Ok(record) => Ok(Some(record)),
                Err(_) => serde_json::from_slice::<LegacyPaymentRecord>(&bytes)
                    .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?
                    .into_current()
                    .map(Some),
            },
            Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(None),
            Err(error) => Err(error),
        }
    }

    fn save(&self, key: &[u8; 32], record: &PaymentRecord) -> io::Result<()> {
        self.save_json(REQUESTS, key, record)
    }

    fn save_json(&self, index: &str, key: &[u8; 32], record: &impl Serialize) -> io::Result<()> {
        let bytes =
            serde_json::to_vec(record).map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
        let path = self.path(index, &hex(key));
        let staging = self.path(index, &format!("{}.staging", hex(key)));
        let mut file = fs::OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .mode(0o600)
            .open(&staging)?;
        file.write_all(&bytes)?;
        file.sync_all()?;
        fs::rename(&staging, &path)?;
        fs::File::open(self.root.join(index))?.sync_all()
    }

    fn delivery(&self, key: &[u8; 32]) -> io::Result<Option<DeliveryRecord>> {
        match fs::read(self.path(DELIVERIES, &hex(key))) {
            Ok(bytes) => serde_json::from_slice(&bytes)
                .map(Some)
                .map_err(|_| io::Error::from(io::ErrorKind::InvalidData)),
            Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(None),
            Err(error) => Err(error),
        }
    }

    /// Binds `name` in `index` to `owner` once. Returns whether `owner` holds it.
    fn claim(&self, index: &str, name: &str, owner: &[u8; 32]) -> io::Result<bool> {
        let path = self.path(index, name);
        let staging = self.path(index, &format!("{name}.{}.staging", hex(owner)));
        match fs::remove_file(&staging) {
            Ok(()) => {}
            Err(error) if error.kind() == io::ErrorKind::NotFound => {}
            Err(error) => return Err(error),
        }
        let mut file = fs::OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&staging)?;
        file.write_all(hex(owner).as_bytes())?;
        file.sync_all()?;
        let claimed = match fs::hard_link(&staging, &path) {
            Ok(()) => {
                fs::File::open(self.root.join(index))?.sync_all()?;
                true
            }
            Err(error) if error.kind() == io::ErrorKind::AlreadyExists => {
                fs::read(&path)? == hex(owner).as_bytes()
            }
            Err(error) => return Err(error),
        };
        fs::remove_file(staging)?;
        Ok(claimed)
    }

    fn holder(&self, index: &str, name: &str) -> io::Result<Option<Vec<u8>>> {
        match fs::read(self.path(index, name)) {
            Ok(bytes) => Ok(Some(bytes)),
            Err(error) if error.kind() == io::ErrorKind::NotFound => Ok(None),
            Err(error) => Err(error),
        }
    }

    /// Marks the payment under `key` released by `receipt_digest`. Returns
    /// `false`, releasing nothing, when the receipt already released a
    /// request or this payment was already released.
    ///
    /// # Errors
    /// Returns the I/O error that prevented recording the release.
    pub fn release(&self, key: &[u8; 32], receipt_digest: &[u8; 32]) -> io::Result<bool> {
        let Some(mut record) = self.load(key)? else {
            return Ok(false);
        };
        if record.released
            || record.receipt_digest.as_deref() != Some(hex(receipt_digest).as_str())
            || !self.claim(RECEIPTS, &hex(receipt_digest), key)?
        {
            return Ok(false);
        }
        record.released = true;
        self.save(key, &record)?;
        Ok(true)
    }
}

#[derive(Debug)]
pub enum GateError {
    Assets(AssetRefusal),
    NetworkId,
    Endpoint,
    Authorization,
    Adapter,
    Store(io::Error),
    Payer,
    Conformance,
}

impl std::fmt::Display for GateError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Assets(refusal) => write!(f, "payment assets refused: {refusal}"),
            Self::NetworkId => f.write_str(
                "kernel_network_id must be resolved to a nonzero value matching configuration",
            ),
            Self::Endpoint => f.write_str("the gateway endpoint is not an http or https URL"),
            Self::Authorization => f.write_str("gateway authorization file or transport refused"),
            Self::Adapter => f.write_str("the 402LXP adapter cannot be registered"),
            Self::Store(error) => write!(f, "the payment store cannot be opened: {error}"),
            Self::Payer => f.write_str("payment.payer_did does not derive a payer account"),
            Self::Conformance => {
                f.write_str("payment.conformance_suite does not hold the pinned conformance suite")
            }
        }
    }
}

impl std::error::Error for GateError {}

struct Receiver {
    key: SigningKey,
    address: [u8; 20],
    did_key: [u8; 32],
    purpose: [u8; 32],
    accounts: [[u8; 32]; 4],
}

/// The payment hook every paid route calls.
pub struct PaymentGate {
    assets: AcceptedAssets,
    receiver: Receiver,
    network: String,
    chain_id: u64,
    receipt_keys: Vec<SequencerAuthorization>,
    payer: Option<String>,
    draw_fee_limit: u128,
    rpc: SettlementRpc,
    store: PaymentStore,
    clock: Clock,
    gateway: Mutex<GatewayCore>,
    delivery_lock: Mutex<()>,
    payment_lock: Mutex<()>,
    _lease: fs::File,
}

impl PaymentGate {
    /// `conformance` is the caller's pinned conformance suite for the x402
    /// adapter the seller settles through.
    ///
    /// # Errors
    /// Refuses the configured assets, an unusable gateway endpoint, a
    /// payment store that cannot be opened, an adapter that cannot be
    /// registered, a configured payer that derives no account, and a
    /// configured conformance suite directory whose recorded exchanges are
    /// not the ones `conformance` pins.
    pub fn new(
        config: &Config,
        receiver: &SigningKey,
        conformance: ConformanceSuite,
        clock: Clock,
    ) -> Result<Self, GateError> {
        Self::new_with_network_id(
            config,
            config.kernel_network_id.ok_or(GateError::NetworkId)?,
            receiver,
            conformance,
            clock,
        )
    }

    pub fn new_with_network_id(
        config: &Config,
        network_id: u32,
        receiver: &SigningKey,
        conformance: ConformanceSuite,
        clock: Clock,
    ) -> Result<Self, GateError> {
        if network_id == 0
            || config
                .kernel_network_id
                .is_some_and(|configured| configured != network_id)
        {
            return Err(GateError::NetworkId);
        }
        let assets = AcceptedAssets::new(&config.assets).map_err(GateError::Assets)?;
        let payment = &config.payment;
        if let Some(payer) = payment.payer_did.as_deref() {
            if !payer_did_valid(payer)
                || assets
                    .all()
                    .iter()
                    .any(|asset| account_id(&wallet_account(payer, asset)).is_none())
            {
                return Err(GateError::Payer);
            }
        }
        if let Some(directory) = payment.conformance_suite.as_deref() {
            if recorded_suite(directory).ok()
                != Some((conformance.suite_digest(), conformance.vector_count()))
            {
                return Err(GateError::Conformance);
            }
        }
        let rpc = SettlementRpc::new(&config.evm.endpoint, config.evm.chain_id)
            .map_err(|_| GateError::Endpoint)?;
        let store = PaymentStore::open(&config.data_dir).map_err(GateError::Store)?;
        let lease = fs::OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .mode(0o600)
            .custom_flags(0x20000)
            .open(store.root.join("writer.lock"))
            .map_err(GateError::Store)?;
        lease
            .try_lock()
            .map_err(|_| GateError::Store(io::Error::other("payment journal already open")))?;
        let address = signer_address(receiver);
        let owner = read_receiver_did(&rpc, &address).map_err(|_| GateError::Adapter)?;
        let mut accounts = [[0; 32]; 4];
        for (slot, asset) in accounts.iter_mut().zip(assets.all()) {
            let data = [
                &keccak(b"accountId(bytes32,bytes32)")[..4],
                &owner,
                &asset.asset_id,
            ]
            .concat();
            *slot = rpc
                .eth_call(LEDGER_PRECOMPILE, &data)
                .map_err(|_| GateError::Adapter)?
                .try_into()
                .map_err(|_| GateError::Adapter)?;
            if *slot == [0; 32] {
                return Err(GateError::Adapter);
            }
        }
        let trace = TraceId::mint([0; 16]);
        let mut gateway = GatewayCore::new();
        let descriptor = x402_adapter_descriptor(conformance).map_err(|_| GateError::Adapter)?;
        gateway
            .register_adapter(descriptor, &trace, clock() / 1_000)
            .map_err(|_| GateError::Adapter)?;
        Ok(Self {
            assets,
            receiver: Receiver {
                key: receiver.clone(),
                purpose: purpose_hash(&address),
                address,
                did_key: owner,
                accounts,
            },
            network: format!("eip155:{}", config.evm.chain_id),
            chain_id: config.evm.chain_id,
            receipt_keys: config.gateway.receipt_keys.clone(),
            payer: payment.payer_did.clone(),
            draw_fee_limit: payment.draw_fee_limit,
            rpc,
            store,
            clock,
            gateway: Mutex::new(gateway),
            delivery_lock: Mutex::new(()),
            payment_lock: Mutex::new(()),
            _lease: lease,
        })
    }

    #[must_use]
    pub const fn assets(&self) -> &AcceptedAssets {
        &self.assets
    }

    #[must_use]
    pub const fn receiver_address(&self) -> [u8; 20] {
        self.receiver.address
    }

    #[must_use]
    pub const fn purpose(&self) -> [u8; 32] {
        self.receiver.purpose
    }

    /// The configured payer every draw is taken from, if any.
    #[must_use]
    pub fn payer(&self) -> Option<&str> {
        self.payer.as_deref()
    }

    /// The fee limit each draw is signed with and may be charged.
    #[must_use]
    pub const fn draw_fee_limit(&self) -> u128 {
        self.draw_fee_limit
    }

    /// The account id the receiver is paid into for `asset`.
    #[must_use]
    pub fn payee(&self, asset: &AcceptedAsset) -> [u8; 32] {
        let index = self
            .assets
            .all()
            .iter()
            .position(|candidate| candidate.asset_id == asset.asset_id)
            .unwrap_or_default();
        self.receiver.accounts[index]
    }

    #[must_use]
    pub const fn store(&self) -> &PaymentStore {
        &self.store
    }

    fn offer(
        &self,
        asset: &AcceptedAsset,
        scheme: &str,
        payer: Option<&str>,
    ) -> PaymentRequirements {
        let mut layerx = json!({
            "commitment": COMMITMENT,
            "account": format!("agent:did:layerx:{}:asset:{}", hex(&self.receiver.did_key), asset.id_hex()),
            "currency": asset.symbol.code(),
        });
        if let (Some(payer), Some(terms)) = (payer, layerx.as_object_mut()) {
            let payer = account_id(&wallet_account(payer, asset)).map(|id| hex(&id));
            terms.insert("payer".to_owned(), json!(payer));
            terms.insert("purposeHash".to_owned(), json!(hex(&self.receiver.purpose)));
        }
        PaymentRequirements {
            scheme: scheme.to_owned(),
            network: self.network.clone(),
            amount: AtomicAmount::from_u128(asset.price),
            asset: asset.id_hex(),
            pay_to: hex(&self.payee(asset)),
            max_timeout_seconds: OFFER_TIMEOUT_SECONDS,
            extra: Some(json!({ "layerx": layerx })),
        }
    }

    /// The offers for one request: per asset a metered draw bound to the
    /// named payer's grant, then the exact alternative.
    ///
    /// # Errors
    /// Answers 400 for a request without a usable `Host` header.
    pub fn payment_required(
        &self,
        request: &Request,
        payer: Option<&str>,
    ) -> Result<PaymentRequired, Response> {
        let host = request
            .header("Host")
            .filter(|host| {
                !host.is_empty()
                    && host.len() <= MAX_HOST_BYTES
                    && host
                        .bytes()
                        .all(|byte| byte.is_ascii_alphanumeric() || b".-:[]".contains(&byte))
            })
            .ok_or_else(|| Response::error(400, "missing_host"))?;
        let target = canonical_target(request)?;
        let mut accepts = Vec::with_capacity(8);
        for asset in self.assets.all() {
            if payer.is_some() {
                accepts.push(self.offer(asset, METERED, payer));
            }
            accepts.push(self.offer(asset, EXACT, None));
        }
        Ok(PaymentRequired {
            x402_version: X402_VERSION,
            error: None,
            resource: ResourceInfo {
                url: format!("http://{host}{target}"),
                description: Some(describe(request.route).to_owned()),
                mime_type: Some("application/json".to_owned()),
                service_name: Some("x-websearch".to_owned()),
                tags: Vec::new(),
                icon_url: None,
            },
            accepts,
            extensions: BTreeMap::new(),
        })
    }

    fn challenge(seller: &Seller, error: &str) -> Response {
        match seller.payment_required() {
            Ok(signal) => {
                let body =
                    serde_json::to_vec(&json!({"error": error, "paymentRequired": signal.body}))
                        .unwrap_or_default();
                Response::json(signal.status, body).with_header(PAYMENT_REQUIRED, &signal.header)
            }
            Err(_) => Response::error(503, "payment_unavailable"),
        }
    }

    /// Settles the request's `PAYMENT-SIGNATURE` and calls `release` only
    /// after a verified receipt that has released no other request. Without
    /// a payment it answers 402 with `PAYMENT-REQUIRED`.
    pub fn settle(&self, request: &Request, release: &dyn Fn(&Request) -> Response) -> Response {
        self.settle_request(request, release, false)
    }

    fn settle_request(
        &self,
        request: &Request,
        release: &dyn Fn(&Request) -> Response,
        resumable: bool,
    ) -> Response {
        let _delivery = self
            .delivery_lock
            .lock()
            .unwrap_or_else(PoisonError::into_inner);
        let resource = match resource_binding(request) {
            Ok(resource) => resource,
            Err(response) => return response,
        };
        let payer = match payer_did(request, self.payer.as_deref()) {
            Ok(payer) => payer,
            Err(response) => return response,
        };
        let required = match self.payment_required(request, payer) {
            Ok(required) => required,
            Err(response) => return response,
        };
        let Ok(seller) = Seller::new(required) else {
            return Response::error(503, "payment_unavailable");
        };
        let seller = seller.with_receipt_keys(self.receipt_keys.clone());
        let Some(header) = request.header(PAYMENT_SIGNATURE) else {
            return Self::challenge(&seller, "payment_required");
        };
        let principal = principal(payer);
        let trace = TraceId::mint(trace_entropy(header));
        let now_ms = (self.clock)();
        let mut plane = Plane {
            gate: self,
            resource: &resource,
            payer,
            now_ms,
            key: None,
            refusal: None,
        };
        let outcome = {
            let mut gateway = self.gateway.lock().unwrap_or_else(PoisonError::into_inner);
            seller.settle(
                &mut gateway,
                &principal,
                header,
                &mut plane,
                &trace,
                now_ms / 1_000,
            )
        };
        if plane.refusal == Some("legacy_replay") {
            if let Some(key) = plane.key {
                let replay = (|| -> io::Result<Option<Response>> {
                    let payment = self.store.load(&key)?.ok_or(io::ErrorKind::InvalidData)?;
                    if payment.tx_hash.is_none() && payment.released {
                        if let Some(delivery) = self.store.delivery(&key)? {
                            delivery.validate(&payment)?;
                            return delivery
                                .response
                                .as_ref()
                                .map(DeliveryResponse::response)
                                .transpose();
                        }
                    }
                    Ok(None)
                })();
                match replay {
                    Ok(Some(response)) => return response,
                    Err(_) => return Response::error(503, "payment_store_unavailable"),
                    Ok(None) => {}
                }
            }
            plane.refusal = Some("receipt_consumed");
        }
        match outcome {
            Err(_) if plane.refusal == Some("payment_pending") => {
                Response::error(503, "payment_pending").with_header("Retry-After", "1")
            }
            Err(_) => Self::challenge(&seller, plane.refusal.unwrap_or("payment_invalid")),
            Ok(SellerOutcome::Pending { .. }) => {
                Response::error(503, "payment_pending").with_header("Retry-After", "1")
            }
            Ok(SellerOutcome::Refused { header, response }) => {
                let reason = response.error_reason.unwrap_or_default();
                Self::challenge(&seller, &reason).with_header(PAYMENT_RESPONSE, &header)
            }
            Ok(SellerOutcome::Settled {
                header,
                receipt_digest,
                ..
            }) => {
                let Some(key) = plane.key else {
                    return Self::challenge(&seller, "payment_invalid");
                };
                if resumable {
                    self.deliver(&key, &receipt_digest, &header, request, release)
                        .unwrap_or_else(|_| Response::error(503, "payment_store_unavailable"))
                } else {
                    match self.store.release(&key, &receipt_digest) {
                        Ok(true) => release(request).with_header(PAYMENT_RESPONSE, &header),
                        Ok(false) => Self::challenge(&seller, "receipt_consumed"),
                        Err(_) => Response::error(503, "payment_store_unavailable"),
                    }
                }
            }
        }
    }

    fn deliver(
        &self,
        key: &[u8; 32],
        receipt_digest: &[u8; 32],
        payment_header: &str,
        request: &Request,
        handler: &dyn Fn(&Request) -> Response,
    ) -> io::Result<Response> {
        let payment = self.store.load(key)?.ok_or(io::ErrorKind::InvalidData)?;
        if payment.receipt.is_none()
            || payment.receipt_digest.as_deref() != Some(hex(receipt_digest).as_str())
            || !self.store.claim(RECEIPTS, &hex(receipt_digest), key)?
        {
            return Err(io::ErrorKind::InvalidData.into());
        }
        let mut delivery = match self.store.delivery(key)? {
            Some(delivery) => delivery,
            None => {
                if payment.released {
                    return Ok(Response::error(409, "receipt_consumed"));
                }
                let delivery = DeliveryRecord::pending(&payment, receipt_digest);
                self.store.save_json(DELIVERIES, key, &delivery)?;
                delivery
            }
        };
        delivery.validate(&payment)?;
        if delivery.state == DeliveryState::Ready {
            return delivery
                .response
                .as_ref()
                .ok_or(io::ErrorKind::InvalidData)?
                .response();
        }
        if delivery.state == DeliveryState::Failed && (self.clock)() < delivery.retry_after_ms {
            let remaining = delivery
                .retry_after_ms
                .saturating_sub((self.clock)())
                .div_ceil(1_000);
            let mut response = delivery
                .response
                .as_ref()
                .ok_or(io::ErrorKind::InvalidData)?
                .response()?;
            response
                .headers
                .retain(|(name, _)| !name.eq_ignore_ascii_case("Retry-After"));
            return Ok(response.with_header("Retry-After", &remaining.max(1).to_string()));
        }
        delivery.state = DeliveryState::Computing;
        delivery.response = None;
        delivery.retry_after_ms = 0;
        self.store.save_json(DELIVERIES, key, &delivery)?;
        let mut response =
            std::panic::catch_unwind(std::panic::AssertUnwindSafe(|| handler(request)))
                .unwrap_or_else(|_| Response::error(500, "delivery_failed"));
        if !(200..300).contains(&response.status) && response.status < 400 {
            response = Response::error(502, "delivery_failed");
        }
        let canonical_content = if (200..300).contains(&response.status) {
            match self.canonical_result(request, &response) {
                Ok(bytes) => Some(bytes),
                Err(_) => {
                    response = Response::error(500, "delivery_result_invalid");
                    None
                }
            }
        } else {
            None
        };
        let mut settlement: layerx_x402::model::SettlementResponse = serde_json::from_slice(
            &STANDARD
                .decode(payment_header)
                .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?,
        )
        .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
        settlement.extensions.insert("x-websearch".to_owned(), json!({
            "resource": payment.resource,
            "receiptDigest": hex(receipt_digest),
            "delivery": if canonical_content.is_some() { "ready" } else { "failed" },
            "contentDigest": canonical_content.as_ref().map(|bytes| hex(&crate::canonical::content_digest(bytes))),
            "responseDigest": hex(&sha256(&[&response.body])),
        }));
        settlement
            .validate_wire()
            .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
        let payment_header = STANDARD.encode(
            serde_json::to_vec(&settlement)
                .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?,
        );
        response.headers.retain(|(name, _)| {
            !name.eq_ignore_ascii_case(PAYMENT_RESPONSE)
                && !name.eq_ignore_ascii_case("Content-Digest")
        });
        let digest = STANDARD.encode(sha256(&[&response.body]));
        response = response
            .with_header(PAYMENT_RESPONSE, &payment_header)
            .with_header("Content-Digest", &format!("sha-256=:{digest}:"));
        delivery.state = if (200..300).contains(&response.status) {
            delivery.failures = 0;
            delivery.retry_after_ms = 0;
            DeliveryState::Ready
        } else {
            delivery.failures = delivery.failures.saturating_add(1);
            let delay = (1_u64 << delivery.failures.saturating_sub(1).min(6)).min(60);
            delivery.retry_after_ms = (self.clock)().saturating_add(delay * 1_000);
            response
                .headers
                .retain(|(name, _)| !name.eq_ignore_ascii_case("Retry-After"));
            response = response.with_header("Retry-After", &delay.to_string());
            DeliveryState::Failed
        };
        delivery.response = Some(DeliveryResponse::from_response(response, canonical_content));
        delivery.validate(&payment)?;
        self.store.save_json(DELIVERIES, key, &delivery)?;
        delivery
            .response
            .as_ref()
            .ok_or(io::ErrorKind::InvalidData)?
            .response()
    }

    fn canonical_result(&self, request: &Request, response: &Response) -> io::Result<Vec<u8>> {
        let value: Value = serde_json::from_slice(&response.body)
            .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
        let digest = value
            .get("digest")
            .and_then(Value::as_str)
            .and_then(unhex32)
            .ok_or(io::ErrorKind::InvalidData)?;
        let path = self
            .store
            .root
            .parent()
            .ok_or(io::ErrorKind::InvalidData)?
            .join("content")
            .join(hex(&digest));
        let mut bytes = Vec::new();
        fs::File::open(path)?
            .take((crate::content::MAX_CONTENT_BYTES as u64) + 1)
            .read_to_end(&mut bytes)?;
        if bytes.len() > crate::content::MAX_CONTENT_BYTES
            || crate::canonical::content_digest(&bytes) != digest
        {
            return Err(io::ErrorKind::InvalidData.into());
        }
        validate_canonical_response(&response.body, &bytes)?;
        let content = crate::canonical::CanonicalContent::parse(&bytes)
            .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
        let parameter = match request.route {
            Route::Search => "q",
            Route::Fetch => "url",
            _ => return Err(io::ErrorKind::InvalidData.into()),
        };
        if request
            .query_param(parameter)
            .ok()
            .flatten()
            .as_deref()
            .map(str::as_bytes)
            != Some(content.payload.as_slice())
        {
            return Err(io::ErrorKind::InvalidData.into());
        }
        Ok(bytes)
    }

    pub fn recover_pending_deliveries(&self) -> io::Result<()> {
        let _delivery = self
            .delivery_lock
            .lock()
            .unwrap_or_else(PoisonError::into_inner);
        for entry in fs::read_dir(self.store.root.join(REQUESTS))? {
            let entry = entry?;
            let name = entry.file_name();
            let name = name.to_str().ok_or(io::ErrorKind::InvalidData)?;
            if name.ends_with(".staging") {
                continue;
            }
            let key = unhex32(name).ok_or(io::ErrorKind::InvalidData)?;
            let payment = self.store.load(&key)?.ok_or(io::ErrorKind::InvalidData)?;
            if unhex32(&payment.request_digest).is_none() || payment.resource.is_empty() {
                return Err(io::ErrorKind::InvalidData.into());
            }
            let Some(receipt) = payment.receipt.as_deref() else {
                if payment.receipt_digest.is_some() || self.store.delivery(&key)?.is_some() {
                    return Err(io::ErrorKind::InvalidData.into());
                }
                continue;
            };
            let bytes = unhex(receipt).ok_or(io::ErrorKind::InvalidData)?;
            let asset = payment
                .offer
                .get("asset")
                .and_then(Value::as_str)
                .and_then(unhex32)
                .and_then(|id| self.assets.by_id(&id))
                .ok_or(io::ErrorKind::InvalidData)?;
            if payment.tx_hash.is_none() && payment.released {
                let verified = verify_sequencer_signature(
                    &bytes,
                    ReceiptKeys::HeightRanges(&self.receipt_keys),
                )
                .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
                let facts = verified.protocol().ok_or(io::ErrorKind::InvalidData)?;
                let digest =
                    leaf_hash(&bytes).map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
                if facts.result_code() != 0
                    || facts.asset() != asset.asset_id
                    || payment.offer.get("amount").and_then(Value::as_str)
                        != Some(facts.amount().to_string().as_str())
                    || payment.offer.get("payTo").and_then(Value::as_str)
                        != Some(hex(&facts.to()).as_str())
                    || payment.payer.as_deref() != Some(hex(&facts.from()).as_str())
                    || payment.receipt_digest.as_deref() != Some(hex(&digest).as_str())
                    || !self.store.claim(RECEIPTS, &hex(&digest), &key)?
                {
                    return Err(io::ErrorKind::InvalidData.into());
                }
                if let Some(delivery) = self.store.delivery(&key)? {
                    delivery.validate(&payment)?;
                }
                continue;
            }
            let metered = payment.receive_key.is_some();
            let scheme = if metered { METERED } else { EXACT };
            if payment.offer.get("scheme").and_then(Value::as_str) != Some(scheme)
                || payment.offer.get("network").and_then(Value::as_str)
                    != Some(self.network.as_str())
                || payment.offer.get("amount").and_then(Value::as_str)
                    != Some(asset.price.to_string().as_str())
                || payment.offer.get("payTo").and_then(Value::as_str)
                    != Some(hex(&self.payee(asset)).as_str())
            {
                return Err(io::ErrorKind::InvalidData.into());
            }
            let expected = Expected {
                operation: if metered { 6 } else { 5 },
                fee_limit: metered.then_some(self.draw_fee_limit),
                activity_id: Some(
                    payment
                        .tx_hash
                        .as_deref()
                        .and_then(unhex32)
                        .ok_or(io::ErrorKind::InvalidData)?,
                ),
                from: Some(
                    payment
                        .payer
                        .as_deref()
                        .and_then(unhex32)
                        .ok_or(io::ErrorKind::InvalidData)?,
                ),
                to: self.payee(asset),
                asset: asset.asset_id,
                amount: asset.price,
                grant: metered
                    .then(|| recorded_grant(&payment))
                    .transpose()
                    .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?,
                receive_key: payment.receive_key.as_deref().and_then(unhex32),
            };
            let plane = Plane {
                gate: self,
                resource: &payment.resource,
                payer: self.payer.as_deref(),
                now_ms: (self.clock)(),
                key: None,
                refusal: None,
            };
            let answer = self
                .rpc
                .sequencer_receipt(&expected.activity_id.ok_or(io::ErrorKind::InvalidData)?)
                .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?
                .ok_or(io::ErrorKind::InvalidData)?;
            if answer.receipt.as_deref() != Some(bytes.as_slice()) {
                return Err(io::ErrorKind::InvalidData.into());
            }
            let digest = plane
                .verify_receipt(&answer, &expected)
                .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
            if payment.receipt_digest.as_deref() != Some(hex(&digest).as_str())
                || !self.store.claim(RECEIPTS, &hex(&digest), &key)?
            {
                return Err(io::ErrorKind::InvalidData.into());
            }
            let mut delivery = match self.store.delivery(&key)? {
                Some(delivery) => delivery,
                None if payment.released => continue,
                None => DeliveryRecord::pending(&payment, &digest),
            };
            delivery.validate(&payment)?;
            if delivery.state == DeliveryState::Computing {
                delivery.state = DeliveryState::Pending;
            }
            self.store.save_json(DELIVERIES, &key, &delivery)?;
        }
        for entry in fs::read_dir(self.store.root.join(DELIVERIES))? {
            let entry = entry?;
            let name = entry.file_name();
            let name = name.to_str().ok_or(io::ErrorKind::InvalidData)?;
            if name.ends_with(".staging") {
                continue;
            }
            let key = unhex32(name).ok_or(io::ErrorKind::InvalidData)?;
            let payment = self.store.load(&key)?.ok_or(io::ErrorKind::InvalidData)?;
            self.store
                .delivery(&key)?
                .ok_or(io::ErrorKind::InvalidData)?
                .validate(&payment)?;
        }
        Ok(())
    }

    pub fn install_validated(
        gate: &Arc<Self>,
        routes: &mut RouteTable,
        route: Route,
        validate: impl Fn(&Request) -> Result<(), Response> + Send + Sync + 'static,
        handler: impl Fn(&Request) -> Response + Send + Sync + 'static,
    ) -> Result<(), RouteError> {
        let gate = Arc::clone(gate);
        routes.set(route, move |request: &Request| {
            if let Err(response) = validate(request) {
                return response;
            }
            if request.route.is_paid() {
                gate.settle_request(request, &handler, true)
            } else {
                handler(request)
            }
        })
    }

    /// Wraps a paid route's handler so it runs only after settlement.
    pub fn paid(
        gate: &Arc<Self>,
        handler: impl Fn(&Request) -> Response + Send + Sync + 'static,
    ) -> impl Fn(&Request) -> Response + Send + Sync + 'static {
        let gate = Arc::clone(gate);
        move |request: &Request| {
            if request.route.is_paid() {
                gate.settle(request, &handler)
            } else {
                handler(request)
            }
        }
    }

    /// Installs a paid route's handler behind the payment hook.
    ///
    /// # Errors
    /// Refuses the built-in health route and a route that already has a handler.
    pub fn install(
        gate: &Arc<Self>,
        routes: &mut RouteTable,
        route: Route,
        handler: impl Fn(&Request) -> Response + Send + Sync + 'static,
    ) -> Result<(), RouteError> {
        routes.set(route, Self::paid(gate, handler))
    }
}

const fn describe(route: Route) -> &'static str {
    match route {
        Route::Health => "Sidecar health",
        Route::Search => "Web search results",
        Route::Fetch => "A fetched page and its digest",
        Route::Content => "Stored content by digest",
    }
}

/// The payer a request's draws are taken from: the configured payer when
/// there is one, which a `LAYERX-PAYER-DID` header may repeat but not
/// contradict, and otherwise the DID the header names.
fn payer_did<'a>(
    request: &'a Request,
    configured: Option<&'a str>,
) -> Result<Option<&'a str>, Response> {
    let named = match request.header(PAYER_DID) {
        None => None,
        Some(did) if payer_did_valid(did) => Some(did),
        Some(_) => return Err(Response::error(400, "malformed_payer")),
    };
    match (configured, named) {
        (Some(configured), Some(named)) if configured != named => {
            Err(Response::error(400, "payer_mismatch"))
        }
        (Some(configured), _) => Ok(Some(configured)),
        (None, named) => Ok(named),
    }
}

/// The digest and rule count of the recorded gateway exchanges in
/// `directory`: SHA-256 over every file's bytes in file name order, and the
/// number of rules under `/endpoints/gateway/*` across them.
fn recorded_suite(directory: &Path) -> io::Result<([u8; 32], u64)> {
    let mut names = fs::read_dir(directory)?
        .map(|entry| entry.map(|entry| entry.path()))
        .collect::<io::Result<Vec<_>>>()?;
    names.sort();
    let mut digest = Sha256::new();
    let mut rules = 0_u64;
    for name in names {
        let bytes = fs::read(name)?;
        digest.update(&bytes);
        let recording: Value = serde_json::from_slice(&bytes)
            .map_err(|_| io::Error::from(io::ErrorKind::InvalidData))?;
        if let Some(list) = recording
            .pointer("/endpoints/gateway/*")
            .and_then(Value::as_array)
        {
            rules = u64::try_from(list.len())
                .ok()
                .and_then(|count| rules.checked_add(count))
                .ok_or_else(|| io::Error::from(io::ErrorKind::InvalidData))?;
        }
    }
    Ok((digest.finalize().into(), rules))
}

fn principal(payer: Option<&str>) -> PrincipalId {
    let name = payer.map_or_else(
        || ANONYMOUS_PRINCIPAL.to_owned(),
        |did| {
            format!(
                "payer-{}",
                hex(&sha256(&[PRINCIPAL_DOMAIN, did.as_bytes()]))
            )
        },
    );
    PrincipalId::new(name).unwrap_or_else(|_| {
        PrincipalId::new(ANONYMOUS_PRINCIPAL).unwrap_or_else(|_| unreachable!("fixed principal"))
    })
}

fn trace_entropy(header: &str) -> [u8; 16] {
    let digest = sha256(&[TRACE_DOMAIN, header.as_bytes()]);
    let mut entropy = [0; 16];
    entropy.copy_from_slice(&digest[..16]);
    entropy
}

fn canonical_target(request: &Request) -> Result<String, Response> {
    let (path, parameter) = match request.route {
        Route::Search => ("/search", Some("q")),
        Route::Fetch => ("/fetch", Some("url")),
        Route::Content => {
            let digest = request
                .digest
                .ok_or_else(|| Response::error(400, "malformed_digest"))?;
            return Ok(format!("/content/{}", hex(&digest)));
        }
        Route::Health => ("/health", None),
    };
    let Some(parameter) = parameter else {
        return Ok(path.to_owned());
    };
    let value = request
        .query_param(parameter)
        .map_err(|_| Response::error(400, "malformed_query"))?
        .filter(|value| !value.is_empty())
        .ok_or_else(|| Response::error(400, "missing_query"))?;
    Ok(format!("{path}?{parameter}={}", encode_query_value(&value)))
}

fn encode_query_value(value: &str) -> String {
    let mut encoded = String::new();
    const DIGITS: &[u8; 16] = b"0123456789ABCDEF";
    for byte in value.bytes() {
        if byte.is_ascii_alphanumeric() || b"-._~".contains(&byte) {
            encoded.push(char::from(byte));
        } else {
            encoded.push('%');
            encoded.push(char::from(DIGITS[usize::from(byte >> 4)]));
            encoded.push(char::from(DIGITS[usize::from(byte & 15)]));
        }
    }
    encoded
}

fn resource_binding(request: &Request) -> Result<String, Response> {
    Ok(format!("{} {}", request.method, canonical_target(request)?))
}

type Settled = Result<PlanePaymentOutcome, &'static str>;

/// The plane the seller settles through: it signs, submits and recovers the
/// draw, confirms an exact receipt, and verifies every receipt against the
/// configured sequencer trust.
struct Plane<'a> {
    gate: &'a PaymentGate,
    resource: &'a str,
    payer: Option<&'a str>,
    now_ms: u64,
    key: Option<[u8; 32]>,
    refusal: Option<&'static str>,
}

impl PaymentPlane for Plane<'_> {
    fn execute(
        &mut self,
        request: LayerXPaymentRequest,
        _trace: &TraceId,
    ) -> Result<PlanePaymentOutcome, layerx_x402::model::X402Error> {
        self.key = Some(request.idempotency_key);
        match self.run(&request) {
            Ok(outcome) => Ok(outcome),
            Err(reason) => {
                self.refusal = Some(reason);
                match reason {
                    "request_mismatch" => Err(layerx_x402::model::X402Error::RequirementsMismatch),
                    "payment_pending" | "legacy_replay" => {
                        Err(layerx_x402::model::X402Error::EvidenceMissing)
                    }
                    _ => Ok(PlanePaymentOutcome::Refused { reason }),
                }
            }
        }
    }
}

fn offer_record(request: &LayerXPaymentRequest) -> Value {
    json!({
        "scheme": request.scheme,
        "network": request.network,
        "asset": request.asset,
        "amount": request.amount.value().to_string(),
        "payTo": request.pay_to,
    })
}

struct Expected {
    operation: u8,
    fee_limit: Option<u128>,
    activity_id: Option<[u8; 32]>,
    from: Option<[u8; 32]>,
    to: [u8; 32],
    asset: [u8; 32],
    amount: u128,
    grant: Option<[u8; 32]>,
    receive_key: Option<[u8; 32]>,
}

impl<'Plane> Plane<'Plane> {
    fn pending(record: &PaymentRecord) -> Settled {
        let transaction_hash = record
            .tx_hash
            .as_deref()
            .and_then(unhex32)
            .ok_or("payment_pending")?;
        Ok(PlanePaymentOutcome::Pending { transaction_hash })
    }

    fn run(&self, request: &LayerXPaymentRequest) -> Settled {
        let asset = unhex32(&request.asset)
            .and_then(|id| self.gate.assets.by_id(&id).copied())
            .ok_or("unsupported_asset")?;
        let pay_to = unhex32(&request.pay_to).ok_or("payee_mismatch")?;
        if pay_to != self.gate.payee(&asset) {
            return Err("payee_mismatch");
        }
        if request.amount.value() != asset.price {
            return Err("amount_mismatch");
        }
        let _payment = self
            .gate
            .payment_lock
            .lock()
            .unwrap_or_else(PoisonError::into_inner);
        let key = request.idempotency_key;
        let store = &self.gate.store;
        let Ok(existing) = store.load(&key) else {
            return Err("payment_pending");
        };
        if let Some(record) = existing {
            if record.principal != request.principal.as_str()
                || record.request_digest != hex(&request.request_digest)
                || record.resource != self.resource
                || record.offer != offer_record(request)
            {
                return Err("request_mismatch");
            }
            if record.tx_hash.is_none() && record.released {
                return Err("legacy_replay");
            }
            return match request.scheme.as_str() {
                METERED => self.recover(&key, record, &asset, pay_to),
                _ => self.exact(request, &asset, pay_to, Some(record)),
            };
        }
        match request.scheme.as_str() {
            METERED => self.draw(request, &asset, pay_to),
            EXACT => self.exact(request, &asset, pay_to, None),
            _ => Err("unsupported_scheme"),
        }
    }

    fn fresh_record(&self, request: &LayerXPaymentRequest) -> PaymentRecord {
        PaymentRecord {
            principal: request.principal.as_str().to_owned(),
            request_digest: hex(&request.request_digest),
            resource: self.resource.to_owned(),
            offer: offer_record(request),
            payer: None,
            receive_key: None,
            tx_hash: None,
            raw_transaction: None,
            attempted: false,
            receipt: None,
            receipt_digest: None,
            released: false,
        }
    }

    fn draw(
        &self,
        request: &LayerXPaymentRequest,
        asset: &AcceptedAsset,
        pay_to: [u8; 32],
    ) -> Settled {
        let payer = self.payer.ok_or("payer_required")?;
        let (grant_id, receive_key) = metered_payload(&request.scheme_payload)?;
        let grant = read_payer_grant(&self.gate.rpc, &grant_id)
            .map_err(|error| match error {
                SettlementError::Malformed => "invalid_grant",
                _ => "payment_pending",
            })?
            .ok_or("invalid_grant")?;
        let payer_account = account_id(&wallet_account(payer, asset)).ok_or("payer_mismatch")?;
        check_grant(
            &grant,
            &payer_account,
            &pay_to,
            asset,
            &self.gate.receiver.purpose,
        )?;
        if self.now_ms / 1_000 >= grant.expiration {
            return Err("grant_expired");
        }
        let key = request.idempotency_key;
        match self
            .gate
            .store
            .claim(RECEIVE_KEYS, &hex(&receive_key), &key)
        {
            Ok(true) => {}
            Ok(false) => return Err("idempotency_conflict"),
            Err(_) => return Err("payment_pending"),
        }
        let rpc = &self.gate.rpc;
        let did_key =
            read_receiver_did(rpc, &self.gate.receiver.address).map_err(|_| "payment_pending")?;
        if did_key != self.gate.receiver.did_key {
            return Err("payment_pending");
        }
        let receiver_sequence = read_receiver_sequence(rpc, &did_key, &asset.asset_id)
            .map_err(|_| "payment_pending")?;
        let nonce = rpc
            .pending_nonce(self.gate.receiver.address)
            .map_err(|_| "payment_pending")?;
        let data = draw_calldata(
            &grant_id,
            &pay_to,
            asset.price,
            &receive_key,
            receiver_sequence,
            &self.gate.receiver.purpose,
            &request.request_digest,
        );
        let gas_price = rpc
            .call("eth_gasPrice", &json!([]))
            .ok()
            .and_then(|value| value.as_str().and_then(parse_quantity))
            .filter(|price| *price > 0)
            .ok_or("payment_pending")?;
        let gas_limit = rpc
            .call(
                "eth_estimateGas",
                &json!([{
                    "from": hex0x(&self.gate.receiver.address), "to": hex0x(&LEDGER_PRECOMPILE),
                    "data": hex0x(&data), "value": "0x0",
                }]),
            )
            .ok()
            .and_then(|value| value.as_str().and_then(parse_quantity))
            .and_then(|gas| u64::try_from(gas).ok())
            .filter(|gas| *gas > 0)
            .ok_or("payment_pending")?;
        if gas_price
            .checked_mul(u128::from(gas_limit))
            .is_none_or(|fee| fee > self.gate.draw_fee_limit)
        {
            return Err("draw_fee_limit_exceeded");
        }
        let signed = LedgerTransaction {
            chain_id: self.gate.chain_id,
            nonce,
            gas_price,
            gas_limit,
            data,
        }
        .sign(&self.gate.receiver.key)
        .map_err(|_| "draw_refused")?;
        let record = PaymentRecord {
            payer: Some(hex(&payer_account)),
            receive_key: Some(hex(&receive_key)),
            raw_transaction: Some(hex(&signed.raw)),
            tx_hash: Some(hex(&signed.hash)),
            ..self.fresh_record(request)
        };
        if self.gate.store.save(&key, &record).is_err() {
            return Self::pending(&record);
        }
        self.submit(&key, record, asset, pay_to)
    }

    fn submit(
        &self,
        key: &[u8; 32],
        mut record: PaymentRecord,
        _asset: &AcceptedAsset,
        _pay_to: [u8; 32],
    ) -> Settled {
        let raw = record
            .raw_transaction
            .as_deref()
            .and_then(unhex)
            .ok_or("request_mismatch")?;
        let expected = record
            .tx_hash
            .as_deref()
            .and_then(unhex32)
            .ok_or("request_mismatch")?;
        if keccak(&raw) != expected {
            return Err("request_mismatch");
        }
        record.attempted = true;
        if self.gate.store.save(key, &record).is_err() {
            return Self::pending(&record);
        }
        match self.gate.rpc.send_raw_transaction(&raw) {
            Ok(hash) if hash == expected => Self::pending(&record),
            Ok(_) => Err("receipt_mismatch"),
            Err(SettlementError::Transport) => Self::pending(&record),
            Err(_) => Err("gateway_refused"),
        }
    }

    fn recover(
        &self,
        key: &[u8; 32],
        record: PaymentRecord,
        asset: &AcceptedAsset,
        pay_to: [u8; 32],
    ) -> Settled {
        let hash = record
            .tx_hash
            .as_deref()
            .and_then(unhex32)
            .ok_or("request_mismatch")?;
        let answer = self
            .gate
            .rpc
            .sequencer_receipt(&hash)
            .map_err(|error| match error {
                SettlementError::Malformed => "receipt_mismatch",
                _ => "payment_pending",
            })?;
        match answer {
            None => self.submit(key, record, asset, pay_to),
            Some(answer) => self.conclude(key, record, answer, asset, pay_to),
        }
    }

    fn conclude(
        &self,
        key: &[u8; 32],
        record: PaymentRecord,
        answer: SettlementReceipt,
        asset: &AcceptedAsset,
        pay_to: [u8; 32],
    ) -> Settled {
        if answer.rung == SettlementRung::Submitted {
            return Self::pending(&record);
        }
        self.finish(key, record, answer, asset, pay_to)
    }

    fn finish(
        &self,
        key: &[u8; 32],
        mut record: PaymentRecord,
        answer: SettlementReceipt,
        asset: &AcceptedAsset,
        pay_to: [u8; 32],
    ) -> Settled {
        let metered = record.receive_key.is_some();
        let expected = Expected {
            operation: if metered { 6 } else { 5 },
            fee_limit: metered.then_some(self.gate.draw_fee_limit),
            activity_id: record.tx_hash.as_deref().and_then(unhex32),
            from: record.payer.as_deref().and_then(unhex32),
            to: pay_to,
            asset: asset.asset_id,
            amount: asset.price,
            grant: metered.then(|| recorded_grant(&record)).transpose()?,
            receive_key: record.receive_key.as_deref().and_then(unhex32),
        };
        let digest = self.verify_receipt(&answer, &expected)?;
        let bytes = answer.receipt.ok_or("receipt_unverified")?;
        if record
            .receipt
            .as_deref()
            .is_some_and(|saved| saved != hex(&bytes))
        {
            return Err("receipt_mismatch");
        }
        match self.gate.store.claim(RECEIPTS, &hex(&digest), key) {
            Ok(true) => {}
            Ok(false) => return Err("receipt_consumed"),
            Err(_) => return Self::pending(&record),
        }
        if record.receipt.is_none() {
            record.receipt = Some(hex(&bytes));
            record.receipt_digest = Some(hex(&digest));
            if self.gate.store.save(key, &record).is_err() {
                return Self::pending(&record);
            }
        } else if record.receipt_digest.as_deref() != Some(hex(&digest).as_str()) {
            return Err("receipt_mismatch");
        }
        Ok(PlanePaymentOutcome::Executed(ExecutedPayment {
            evidence: SequencerEvidence {
                transaction_hash: answer.tx_hash,
                signed_receipt: bytes,
                logs: answer
                    .logs
                    .into_iter()
                    .map(|log| SequencerLog::new(log.address, log.topics, log.data, log.log_index))
                    .collect(),
                rung: answer.rung,
            },
        }))
    }

    fn verify_receipt(
        &self,
        answer: &SettlementReceipt,
        expected: &Expected,
    ) -> Result<[u8; 32], &'static str> {
        let bytes = answer
            .receipt
            .as_deref()
            .filter(|bytes| bytes.len() <= RECEIPT_HEX_LIMIT / 2)
            .ok_or("receipt_unverified")?;
        let receipt =
            verify_sequencer_signature(bytes, ReceiptKeys::HeightRanges(&self.gate.receipt_keys))
                .map_err(|_| "receipt_unverified")?;
        let facts = receipt.protocol().ok_or("receipt_unverified")?;
        let batch = facts.batch_id();
        let height = u64::from_be_bytes(batch[24..].try_into().map_err(|_| "receipt_unverified")?);
        if expected
            .activity_id
            .is_some_and(|hash| hash != answer.tx_hash)
            || facts.activity_id() != answer.tx_hash
            || answer.seq != Some(facts.global_sequence())
            || answer.height != Some(height)
            || answer.key_id != Some(facts.context_hash())
            || answer.status != Some(u64::from(facts.result_code() == 0))
        {
            return Err("receipt_mismatch");
        }
        let logs = answer
            .logs
            .iter()
            .map(|log| {
                SequencerLog::new(
                    log.address,
                    log.topics.clone(),
                    log.data.clone(),
                    log.log_index,
                )
            })
            .collect();
        let verified = verify_logs(&receipt, logs).map_err(|_| "receipt_mismatch")?;
        if facts.result_code() != 0 {
            return Err("payment_failed");
        }
        if expected
            .fee_limit
            .is_some_and(|limit| facts.fee_charged() > limit)
        {
            return Err("draw_fee_limit_exceeded");
        }
        let event = if expected.operation == 6 {
            keccak(b"PaymentDrawn(bytes32,bytes32,bytes32,bytes32,uint128,bytes32,bytes32)")
        } else {
            keccak(b"PaymentExact(bytes32,bytes32,bytes32,bytes32,uint128)")
        };
        let matches = verified
            .logs()
            .iter()
            .filter(|log| {
                let topics = log.topics();
                let data = log.data();
                log.address() == LEDGER_PRECOMPILE
                    && topics.len() == 4
                    && topics[0] == event
                    && if expected.operation == 6 {
                        expected.grant == Some(topics[1])
                            && expected.from == Some(topics[2])
                            && topics[3] == expected.to
                            && data.len() == 128
                            && expected
                                .receive_key
                                .as_ref()
                                .is_some_and(|key| data[64..96] == key[..])
                    } else {
                        expected.from.is_none_or(|from| topics[2] == from)
                            && topics[3] == expected.to
                            && data.len() == 64
                    }
                    && data[..32] == expected.asset
                    && data[32..48] == [0; 16]
                    && data[48..64] == expected.amount.to_be_bytes()
            })
            .count();
        if matches != 1 {
            return Err("receipt_mismatch");
        }
        canonical_receipt_digest(bytes).map_err(|_| "receipt_unverified")
    }

    fn exact(
        &self,
        request: &LayerXPaymentRequest,
        asset: &AcceptedAsset,
        pay_to: [u8; 32],
        existing: Option<PaymentRecord>,
    ) -> Settled {
        let (hash, claimed) = exact_payload(&request.scheme_payload)?;
        let key = request.idempotency_key;
        if existing
            .as_ref()
            .is_some_and(|record| record.tx_hash.as_deref() != Some(hex(&hash).as_str()))
        {
            return Err("request_mismatch");
        }
        let answer = self
            .gate
            .rpc
            .sequencer_receipt(&hash)
            .map_err(|error| match error {
                SettlementError::Malformed => "receipt_mismatch",
                _ => "payment_pending",
            })?;
        let Some(answer) = answer.filter(|answer| answer.rung != SettlementRung::Submitted) else {
            return Ok(PlanePaymentOutcome::Pending {
                transaction_hash: hash,
            });
        };
        let expected = Expected {
            operation: 5,
            fee_limit: None,
            activity_id: Some(hash),
            from: self
                .payer
                .map(|payer| account_id(&wallet_account(payer, asset)).ok_or("payer_mismatch"))
                .transpose()?,
            to: pay_to,
            asset: asset.asset_id,
            amount: asset.price,
            grant: None,
            receive_key: None,
        };
        let digest = self.verify_receipt(&answer, &expected)?;
        if digest != claimed {
            return Err("receipt_digest_mismatch");
        }
        match self.gate.store.holder(RECEIPTS, &hex(&digest)) {
            Ok(Some(owner)) if owner != hex(&key).as_bytes() => return Err("receipt_consumed"),
            Err(_) => return Err("payment_pending"),
            _ => {}
        }
        let event = answer
            .logs
            .iter()
            .filter_map(decode_payment_exact)
            .find(|event| {
                event.payee == pay_to
                    && event.asset == asset.asset_id
                    && event.amount == asset.price
            })
            .ok_or("receipt_mismatch")?;
        if event.reference != key {
            return Err("receipt_mismatch");
        }
        let record = existing.unwrap_or_else(|| PaymentRecord {
            payer: Some(hex(&event.payer)),
            tx_hash: Some(hex(&hash)),
            attempted: true,
            ..self.fresh_record(request)
        });
        if self.gate.store.save(&key, &record).is_err() {
            return Self::pending(&record);
        }
        self.conclude(&key, record, answer, asset, pay_to)
    }
}

fn recorded_grant(record: &PaymentRecord) -> Result<[u8; 32], &'static str> {
    fn item(bytes: &[u8]) -> Option<(bool, &[u8], &[u8])> {
        let first = *bytes.first()?;
        let (list, offset, length) = match first {
            0..=127 => (false, 0, 1),
            128..=183 => (false, 1, usize::from(first - 128)),
            192..=247 => (true, 1, usize::from(first - 192)),
            _ => {
                let list = first >= 248;
                let width = usize::from(first - if list { 247 } else { 183 });
                let length = bytes.get(1..1 + width)?.iter().try_fold(0usize, |n, b| {
                    n.checked_mul(256)?.checked_add(usize::from(*b))
                })?;
                (list, 1 + width, length)
            }
        };
        let end = offset.checked_add(length)?;
        Some((list, bytes.get(offset..end)?, bytes.get(end..)?))
    }
    let raw = record
        .raw_transaction
        .as_deref()
        .and_then(unhex)
        .ok_or("receipt_mismatch")?;
    if record.tx_hash.as_deref().and_then(unhex32) != Some(keccak(&raw)) {
        return Err("receipt_mismatch");
    }
    let (list, mut fields, tail) = item(&raw).ok_or("receipt_mismatch")?;
    if !list || !tail.is_empty() {
        return Err("receipt_mismatch");
    }
    for index in 0..6 {
        let (list, field, rest) = item(fields).ok_or("receipt_mismatch")?;
        if list {
            return Err("receipt_mismatch");
        }
        if index == 3 && field != LEDGER_PRECOMPILE {
            return Err("receipt_mismatch");
        }
        if index == 5 {
            if field.len() != 228 || field[..4] != keccak(DRAW_SIGNATURE.as_bytes())[..4] {
                return Err("receipt_mismatch");
            }
            return field[4..36].try_into().map_err(|_| "receipt_mismatch");
        }
        fields = rest;
    }
    Err("receipt_mismatch")
}

fn check_grant(
    grant: &Grant,
    payer: &[u8; 32],
    pay_to: &[u8; 32],
    asset: &AcceptedAsset,
    purpose: &[u8; 32],
) -> Result<(), &'static str> {
    if grant.from != *payer {
        return Err("grant_payer_mismatch");
    }
    if grant.recipient != *pay_to {
        return Err("grant_recipient_mismatch");
    }
    if grant.asset != asset.asset_id {
        return Err("grant_asset_mismatch");
    }
    if grant.purpose_hash != *purpose {
        return Err("grant_purpose_mismatch");
    }
    if grant.recurring || grant.window_length != 0 || grant.has_reference {
        return Err("grant_not_metered");
    }
    if grant.per_draw_maximum < asset.price {
        return Err("grant_limit");
    }
    Ok(())
}

/// The metered payload: `grantId` and `idempotencyKey` as 32-byte lowercase hex.
fn metered_payload(payload: &Value) -> Result<([u8; 32], [u8; 32]), &'static str> {
    let fields = payload.as_object().ok_or("invalid_payment_payload")?;
    if fields.len() != 2 {
        return Err("invalid_payment_payload");
    }
    let grant = fields
        .get("grantId")
        .and_then(Value::as_str)
        .and_then(unhex32)
        .ok_or("invalid_payment_payload")?;
    let key = fields
        .get("idempotencyKey")
        .and_then(Value::as_str)
        .and_then(unhex32)
        .ok_or("invalid_payment_payload")?;
    Ok((grant, key))
}

/// The exact payload binds a payExact transaction to its canonical receipt digest.
fn exact_payload(payload: &Value) -> Result<([u8; 32], [u8; 32]), &'static str> {
    let fields = payload.as_object().ok_or("invalid_payment_payload")?;
    let known = [
        "txHash",
        "receiptDigest",
        "verificationLevel",
        "idempotencyKey",
    ];
    if fields.keys().any(|name| !known.contains(&name.as_str()))
        || fields.get("verificationLevel").and_then(Value::as_str) != Some("receipted")
        || fields
            .get("idempotencyKey")
            .is_some_and(|key| key.as_str().and_then(unhex32).is_none())
    {
        return Err("invalid_payment_payload");
    }
    let hash = fields
        .get("txHash")
        .and_then(Value::as_str)
        .and_then(|text| unhex32(text.strip_prefix("0x").unwrap_or(text)))
        .ok_or("invalid_payment_payload")?;
    let digest = fields
        .get("receiptDigest")
        .and_then(Value::as_str)
        .and_then(unhex32)
        .ok_or("invalid_payment_payload")?;
    Ok((hash, digest))
}

#[cfg(test)]
mod chunked_tests {
    use std::io::{Read as _, Write as _};
    use std::net::TcpListener;
    use std::thread;

    use serde_json::json;

    use super::{decode_http, GatewayRpc, RpcAnswer};

    const HEAD: &str = "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nConnection: close\r\nTransfer-Encoding: chunked\r\n\r\n";

    #[test]
    fn a_chunked_answer_is_joined_across_its_chunks_and_trailers() {
        let raw = format!(
            "{HEAD}10;ext=1\r\n{{\"jsonrpc\":\"2.0\"\r\nC\r\n,\"id\":1,\"res\r\nB\r\nult\":\"0x1\"}}\r\n0\r\nExpires: never\r\n\r\n"
        );
        let value = decode_http(raw.as_bytes()).expect("chunked answer decodes");
        assert_eq!(value, json!({"jsonrpc": "2.0", "id": 1, "result": "0x1"}));
    }

    #[test]
    fn a_chunked_answer_cut_short_or_followed_by_more_bytes_is_refused() {
        let cut = format!("{HEAD}20\r\n{{\"jsonrpc\":\"2.0\"}}\r\n");
        assert!(decode_http(cut.as_bytes()).is_none());
        let trailing = format!("{HEAD}2\r\n{{}}\r\n0\r\n\r\nx");
        assert!(decode_http(trailing.as_bytes()).is_none());
        let both = format!(
            "HTTP/1.1 200 OK\r\nContent-Length: 2\r\nTransfer-Encoding: chunked\r\n\r\n2\r\n{{}}\r\n0\r\n\r\n"
        );
        assert!(decode_http(both.as_bytes()).is_none());
        let gzip = format!("HTTP/1.1 200 OK\r\nTransfer-Encoding: gzip\r\n\r\n{{}}");
        assert!(decode_http(gzip.as_bytes()).is_none());
    }

    #[test]
    fn the_client_reads_a_chunked_answer_from_a_loopback_endpoint() {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback listener");
        let address = listener.local_addr().expect("listener address");
        let body =
            json!({"jsonrpc": "2.0", "id": 1, "result": "0x".to_owned() + &"00".repeat(2048)})
                .to_string();
        let server = thread::spawn(move || {
            let (mut stream, _) = listener.accept().expect("one connection");
            let mut request = [0u8; 4096];
            let read = stream.read(&mut request).expect("request bytes");
            assert!(std::str::from_utf8(&request[..read])
                .expect("utf-8 request")
                .starts_with("POST / HTTP/1.1\r\n"));
            let (first, second) = body.split_at(1000);
            let answer = format!(
                "{HEAD}{:x}\r\n{first}\r\n{:x}\r\n{second}\r\n0\r\n\r\n",
                first.len(),
                second.len()
            );
            stream.write_all(answer.as_bytes()).expect("answer written");
        });
        let rpc = GatewayRpc::new(&format!("http://{address}")).expect("loopback endpoint");
        let answer = rpc.call("eth_call", &json!([]));
        server.join().expect("server thread");
        match answer {
            Some(RpcAnswer::Result(value)) => {
                assert_eq!(value, json!("0x".to_owned() + &"00".repeat(2048)));
            }
            other => panic!("chunked answer not read: {other:?}"),
        }
    }
}

#[cfg(test)]
mod offer_tests {
    use std::error::Error;
    use std::net::SocketAddr;

    use base64::engine::general_purpose::STANDARD;
    use base64::Engine as _;
    use k256::ecdsa::SigningKey;
    use layerx_interop_gateway::adapter::{AdapterId, ConformanceSuite};
    use layerx_x402::seller::Seller;
    use serde_json::{json, Value};

    use super::{PaymentGate, PAYER_DID, PAYMENT_REQUIRED};
    use crate::config::Config;
    use crate::server::{Request, Response, Route};

    const fn fixed_clock() -> u64 {
        1_700_000_000_000
    }

    #[test]
    fn offers_carry_the_configured_eip155_chain_and_an_unpaid_fetch_answers_402(
    ) -> Result<(), Box<dyn Error>> {
        let data =
            std::env::temp_dir().join(format!("x-websearch-payment-offers-{}", std::process::id()));
        let _ = std::fs::remove_dir_all(&data);
        let mut config: Value =
            serde_json::from_str(include_str!("../tests/fixtures/config/valid.json"))?;
        config["data_dir"] = json!(data);
        config["evm"]["chain_id"] = json!(125);
        let listener = std::net::TcpListener::bind("127.0.0.1:0")?;
        config["evm"]["endpoint"] = json!(format!("http://{}", listener.local_addr()?));
        let recording: Value =
            serde_json::from_str(include_str!("../tests/fixtures/gateway/metered.json"))?;
        let ledger = std::thread::spawn(move || {
            use std::io::{Read as _, Write as _};
            let rules = recording["endpoints"]["gateway"]["*"]
                .as_array()
                .expect("rules");
            for _ in 0..8 {
                let (mut stream, _) = listener.accept().expect("ledger connection");
                let mut raw = Vec::new();
                let request = loop {
                    let mut byte = [0];
                    stream.read_exact(&mut byte).expect("request byte");
                    raw.push(byte[0]);
                    if let Some(end) = raw.windows(4).position(|part| part == b"\r\n\r\n") {
                        let head = std::str::from_utf8(&raw[..end]).expect("HTTP head");
                        let length: usize = head
                            .lines()
                            .find_map(|line| line.strip_prefix("Content-Length: "))
                            .expect("length")
                            .parse()
                            .expect("length number");
                        let mut body = vec![0; length];
                        stream.read_exact(&mut body).expect("RPC body");
                        break serde_json::from_slice::<Value>(&body).expect("RPC request");
                    }
                };
                let result = if request["method"] == "eth_chainId" {
                    json!("0x7d")
                } else {
                    assert_eq!(request["method"], "eth_call");
                    rules
                        .iter()
                        .find(|rule| {
                            rule["method"] == request["method"]
                                && rule["params"] == request["params"]
                        })
                        .expect("recorded accountId call")["result"]
                        .clone()
                };
                let body = json!({"jsonrpc":"2.0", "id":1, "result":result}).to_string();
                write!(
                    stream,
                    "HTTP/1.1 200 OK\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{body}",
                    body.len()
                )
                .expect("ledger response");
            }
        });
        let config = Config::parse(&config.to_string())?;
        let conformance = ConformanceSuite::new(AdapterId::new("x402-v2")?, 1, [7; 32])?;
        let gate = PaymentGate::new(
            &config,
            &SigningKey::from_slice(
                &super::unhex("0000000000000000000000000000000000000000000000000000000000000003")
                    .ok_or("receiver secret")?,
            )?,
            conformance,
            fixed_clock,
        )?;
        ledger.join().map_err(|_| "ledger recorder failed")?;
        let payer = format!("did:layerx:{}", super::hex(&[12; 32]));
        let request = Request {
            method: "GET".to_owned(),
            route: Route::Fetch,
            path: "/fetch".to_owned(),
            query: Some("url=https%3A%2F%2Fpaxeer.app%2F".to_owned()),
            headers: vec![
                ("Host".to_owned(), "127.0.0.1:8480".to_owned()),
                (PAYER_DID.to_owned(), payer.clone()),
            ],
            digest: None,
            peer: SocketAddr::from(([127, 0, 0, 1], 40_000)),
        };

        let required = gate
            .payment_required(&request, Some(&payer))
            .map_err(|response| format!("offers refused with {}", response.status))?;
        assert_eq!(required.accepts.len(), 8);
        assert!(required
            .accepts
            .iter()
            .all(|offer| offer.network == "eip155:125"));
        let expected = serde_json::to_value(&required)?;
        Seller::new(required)?;

        let response = gate.settle(&request, &|_| Response::error(500, "released"));
        assert_eq!(response.status, 402);
        let header = response
            .headers
            .iter()
            .find(|(name, _)| name.eq_ignore_ascii_case(PAYMENT_REQUIRED))
            .map(|(_, value)| value)
            .ok_or("402 without PAYMENT-REQUIRED")?;
        let signalled: Value = serde_json::from_slice(&STANDARD.decode(header)?)?;
        assert_eq!(signalled, expected);
        let body: Value = serde_json::from_slice(&response.body)?;
        assert_eq!(body["error"], json!("payment_required"));
        assert_eq!(body["paymentRequired"], expected);
        drop(gate);
        std::fs::remove_dir_all(&data)?;
        Ok(())
    }
}
