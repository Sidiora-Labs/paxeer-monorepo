use std::io::{self, Read as _, Write as _};
use std::net::{TcpStream, ToSocketAddrs as _};
use std::time::Duration;

use serde_json::{json, Value};

pub use layerx_interop_gateway::evidence::SettlementRung;

use crate::watch::{hex0x, parse_quantity, unhex0x};

const RPC_TIMEOUT: Duration = Duration::from_secs(10);
const RPC_RESPONSE_LIMIT: usize = 4_194_304;

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum SettlementError {
    Transport,
    Rpc { code: i64, message: String },
    Malformed,
    ChainMismatch { expected: u64, actual: u64 },
}

impl std::fmt::Display for SettlementError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Transport => f.write_str("settlement RPC transport failed"),
            Self::Rpc { code, message } => write!(f, "settlement RPC {code}: {message}"),
            Self::Malformed => f.write_str("malformed settlement RPC answer or endpoint"),
            Self::ChainMismatch { expected, actual } => {
                write!(
                    f,
                    "settlement chain mismatch: expected {expected}, got {actual}"
                )
            }
        }
    }
}

impl std::error::Error for SettlementError {}

#[derive(Clone, Debug)]
pub struct SettlementRpc {
    endpoint: String,
    chain_id: u64,
    tls: bool,
    host: String,
    port: u16,
    authority: String,
    path: String,
}

impl SettlementRpc {
    pub fn new(endpoint: &str, chain_id: u64) -> Result<Self, SettlementError> {
        let (tls, rest) = if let Some(rest) = endpoint.strip_prefix("https://") {
            (true, rest)
        } else if let Some(rest) = endpoint.strip_prefix("http://") {
            (false, rest)
        } else {
            return Err(SettlementError::Malformed);
        };
        if endpoint.chars().any(char::is_whitespace) || endpoint.contains(['@', '#', '?']) {
            return Err(SettlementError::Malformed);
        }
        let (authority, path) = rest
            .split_once('/')
            .map_or((rest, "/".to_owned()), |(authority, path)| {
                (authority, format!("/{path}"))
            });
        let default_port = if tls { 443 } else { 80 };
        let (host, port) = if let Some(bracketed) = authority.strip_prefix('[') {
            let (host, tail) = bracketed
                .split_once(']')
                .ok_or(SettlementError::Malformed)?;
            let port = match tail.strip_prefix(':') {
                Some(port) => port.parse().map_err(|_| SettlementError::Malformed)?,
                None if tail.is_empty() => default_port,
                None => return Err(SettlementError::Malformed),
            };
            (host, port)
        } else if let Some((host, port)) = authority.rsplit_once(':') {
            (host, port.parse().map_err(|_| SettlementError::Malformed)?)
        } else {
            (authority, default_port)
        };
        if host.is_empty() {
            return Err(SettlementError::Malformed);
        }
        Ok(Self {
            endpoint: endpoint.to_owned(),
            chain_id,
            tls,
            host: host.to_owned(),
            port,
            authority: authority.to_owned(),
            path,
        })
    }

    pub fn call(&self, method: &str, params: &Value) -> Result<Value, SettlementError> {
        let actual = self.chain_id()?;
        if actual != self.chain_id {
            return Err(SettlementError::ChainMismatch {
                expected: self.chain_id,
                actual,
            });
        }
        self.request(method, params)
    }

    pub fn chain_id(&self) -> Result<u64, SettlementError> {
        quantity(&self.request("eth_chainId", &json!([]))?)
    }

    pub fn pending_nonce(&self, address: [u8; 20]) -> Result<u64, SettlementError> {
        quantity(&self.call(
            "eth_getTransactionCount",
            &json!([hex0x(&address), "pending"]),
        )?)
    }

    pub fn eth_call(&self, to: [u8; 20], data: &[u8]) -> Result<Vec<u8>, SettlementError> {
        bytes(&self.call(
            "eth_call",
            &json!([{"to": hex0x(&to), "data": hex0x(data)}, "pending"]),
        )?)
    }

    pub fn send_raw_transaction(&self, transaction: &[u8]) -> Result<[u8; 32], SettlementError> {
        fixed(&self.call("eth_sendRawTransaction", &json!([hex0x(transaction)]))?)
    }

    pub fn sequencer_receipt(
        &self,
        hash: &[u8; 32],
    ) -> Result<Option<SettlementReceipt>, SettlementError> {
        let value = self.call("pax_sequencerReceipt", &json!([hex0x(hash)]))?;
        if value.is_null() {
            return Ok(None);
        }
        let receipt = decode(&value)?;
        if &receipt.tx_hash != hash {
            return Err(SettlementError::Malformed);
        }
        Ok(Some(receipt))
    }

    fn request(&self, method: &str, params: &Value) -> Result<Value, SettlementError> {
        let body =
            json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}).to_string();
        let request = format!(
            "POST {} HTTP/1.1\r\nHost: {}\r\nContent-Type: application/json\r\nAccept: application/json\r\nConnection: close\r\nContent-Length: {}\r\n\r\n{body}",
            self.path, self.authority, body.len()
        );
        let address = (self.host.as_str(), self.port)
            .to_socket_addrs()
            .map_err(|_| SettlementError::Transport)?
            .next()
            .ok_or(SettlementError::Transport)?;
        let stream = TcpStream::connect_timeout(&address, RPC_TIMEOUT)
            .map_err(|_| SettlementError::Transport)?;
        stream
            .set_read_timeout(Some(RPC_TIMEOUT))
            .map_err(|_| SettlementError::Transport)?;
        stream
            .set_write_timeout(Some(RPC_TIMEOUT))
            .map_err(|_| SettlementError::Transport)?;
        let raw = if self.tls {
            let connector =
                native_tls::TlsConnector::new().map_err(|_| SettlementError::Transport)?;
            let mut stream = connector
                .connect(&self.host, stream)
                .map_err(|_| SettlementError::Transport)?;
            exchange(&mut stream, request.as_bytes())
        } else {
            let mut stream = stream;
            exchange(&mut stream, request.as_bytes())
        }
        .ok_or(SettlementError::Transport)?;
        let value = decode_http(&raw).ok_or(SettlementError::Malformed)?;
        if value.get("jsonrpc") != Some(&json!("2.0"))
            || value.get("id") != Some(&json!(1))
            || value.get("result").is_some() == value.get("error").is_some()
        {
            return Err(SettlementError::Malformed);
        }
        if let Some(error) = value.get("error") {
            return Err(SettlementError::Rpc {
                code: error
                    .get("code")
                    .and_then(Value::as_i64)
                    .ok_or(SettlementError::Malformed)?,
                message: error
                    .get("message")
                    .and_then(Value::as_str)
                    .ok_or(SettlementError::Malformed)?
                    .to_owned(),
            });
        }
        value
            .get("result")
            .cloned()
            .ok_or(SettlementError::Malformed)
    }

    #[must_use]
    pub fn endpoint(&self) -> &str {
        &self.endpoint
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SettlementLog {
    pub address: [u8; 20],
    pub topics: Vec<[u8; 32]>,
    pub data: Vec<u8>,
    pub log_index: u64,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct SettlementReceipt {
    pub rung: SettlementRung,
    pub tx_hash: [u8; 32],
    pub nonce: u64,
    pub seq: Option<u64>,
    pub height: Option<u64>,
    pub status: Option<u64>,
    pub logs: Vec<SettlementLog>,
    pub key_id: Option<[u8; 32]>,
    pub receipt: Option<Vec<u8>>,
}

pub fn decode(value: &Value) -> Result<SettlementReceipt, SettlementError> {
    let object = value.as_object().ok_or(SettlementError::Malformed)?;
    let rung = match field(value, "rung")?.as_str() {
        Some("submitted") => SettlementRung::Submitted,
        Some("receipted") => SettlementRung::Receipted,
        Some("committed") => SettlementRung::Committed {
            block_number: quantity(field(value, "blockNumber")?)?,
            block_hash: fixed(field(value, "blockHash")?)?,
        },
        _ => return Err(SettlementError::Malformed),
    };
    let mut answer = SettlementReceipt {
        rung,
        tx_hash: fixed(field(value, "txHash")?)?,
        nonce: quantity(field(value, "nonce")?)?,
        seq: None,
        height: None,
        status: None,
        logs: Vec::new(),
        key_id: None,
        receipt: None,
    };
    if rung == SettlementRung::Submitted {
        const RECEIPT_FIELDS: &[&str] = &[
            "seq",
            "height",
            "index",
            "timestamp",
            "status",
            "returnData",
            "logs",
            "gasUsed",
            "cumulativeGasUsed",
            "effectiveGasPrice",
            "feeDenom",
            "feeAmount",
            "sponsor",
            "from",
            "to",
            "contractAddress",
            "vmError",
            "keyId",
            "receipt",
            "blockNumber",
            "blockHash",
        ];
        if RECEIPT_FIELDS.iter().any(|name| object.contains_key(*name)) {
            return Err(SettlementError::Malformed);
        }
        return Ok(answer);
    }
    if rung == SettlementRung::Receipted
        && (object.contains_key("blockNumber") || object.contains_key("blockHash"))
    {
        return Err(SettlementError::Malformed);
    }
    answer.seq = Some(quantity(field(value, "seq")?)?);
    let height = quantity(field(value, "height")?)?;
    answer.height = Some(height);
    let status = quantity(field(value, "status")?)?;
    if status > 1 {
        return Err(SettlementError::Malformed);
    }
    answer.status = Some(status);
    answer.key_id = Some(fixed(field(value, "keyId")?)?);
    let receipt = bytes(field(value, "receipt")?)?;
    if receipt.is_empty() {
        return Err(SettlementError::Malformed);
    }
    answer.receipt = Some(receipt);
    answer.logs = field(value, "logs")?
        .as_array()
        .ok_or(SettlementError::Malformed)?
        .iter()
        .map(|log| {
            let topics = field(log, "topics")?
                .as_array()
                .ok_or(SettlementError::Malformed)?;
            if topics.len() > 4 {
                return Err(SettlementError::Malformed);
            }
            Ok(SettlementLog {
                address: fixed(field(log, "address")?)?,
                topics: topics.iter().map(fixed).collect::<Result<_, _>>()?,
                data: bytes(field(log, "data")?)?,
                log_index: quantity(field(log, "logIndex")?)?,
            })
        })
        .collect::<Result<_, _>>()?;
    if let SettlementRung::Committed { block_number, .. } = rung {
        if block_number < height {
            return Err(SettlementError::Malformed);
        }
    }
    Ok(answer)
}

fn field<'a>(value: &'a Value, name: &str) -> Result<&'a Value, SettlementError> {
    value.get(name).ok_or(SettlementError::Malformed)
}

fn quantity(value: &Value) -> Result<u64, SettlementError> {
    value
        .as_str()
        .and_then(parse_quantity)
        .and_then(|value| u64::try_from(value).ok())
        .ok_or(SettlementError::Malformed)
}

fn bytes(value: &Value) -> Result<Vec<u8>, SettlementError> {
    value
        .as_str()
        .and_then(unhex0x)
        .ok_or(SettlementError::Malformed)
}

fn fixed<const N: usize>(value: &Value) -> Result<[u8; N], SettlementError> {
    bytes(value)?
        .try_into()
        .map_err(|_| SettlementError::Malformed)
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
