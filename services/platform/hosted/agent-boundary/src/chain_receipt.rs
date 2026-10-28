use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::verify_sequencer_signature;
use rustls::pki_types::{pem::PemObject, CertificateDer, ServerName};
use rustls::{ClientConfig, ClientConnection, RootCertStore, StreamOwned};
use serde::Deserialize;
use serde_json::{json, Value};
use std::fs::File;
use std::io::{Read, Write};
use std::net::{SocketAddr, TcpStream, ToSocketAddrs};
use std::path::Path;
use std::sync::{mpsc, Arc};
use std::time::{Duration, Instant};

const MAX_KEYS_BYTES: u64 = 256 * 1024;
const MAX_RESPONSE_BYTES: u64 = 4 * 1024 * 1024;
const TIMEOUT: Duration = Duration::from_secs(15);

pub(super) struct ChainReceipt {
    pub(super) transaction_hash: [u8; 32],
    pub(super) bytes: Vec<u8>,
}

pub(super) struct ChainReceipts {
    host: String,
    authority: String,
    target: String,
    addresses: Vec<SocketAddr>,
    tls: Option<Arc<ClientConfig>>,
    keys: Vec<SequencerAuthorization>,
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct ReceiptKeyEntry {
    #[serde(rename = "keyId", alias = "key_id")]
    key_id: String,
    #[serde(rename = "publicKey", alias = "public_key")]
    public_key: String,
    #[serde(rename = "firstHeight", alias = "first_height")]
    first_height: u64,
    #[serde(rename = "lastHeight", alias = "last_height")]
    last_height: u64,
}

fn base64_key(text: &str) -> Result<[u8; 32], String> {
    if text.len() != 44 || !text.ends_with('=') {
        return Err("receipt key must be padded base64 of 32 bytes".to_owned());
    }
    let mut bytes = Vec::with_capacity(32);
    let mut accumulator = 0_u32;
    let mut bits = 0;
    for byte in text.bytes().take(43) {
        let digit = match byte {
            b'A'..=b'Z' => byte - b'A',
            b'a'..=b'z' => byte - b'a' + 26,
            b'0'..=b'9' => byte - b'0' + 52,
            b'+' => 62,
            b'/' => 63,
            _ => return Err("invalid receipt key base64".to_owned()),
        };
        accumulator = (accumulator << 6) | u32::from(digit);
        bits += 6;
        if bits >= 8 {
            bits -= 8;
            bytes.push((accumulator >> bits) as u8);
            accumulator &= (1 << bits) - 1;
        }
    }
    if accumulator != 0 {
        return Err("noncanonical receipt key base64".to_owned());
    }
    bytes
        .try_into()
        .map_err(|_| "invalid receipt key length".to_owned())
}

impl ChainReceipts {
    pub(super) fn from_config(url: &str, keys_path: &Path) -> Result<Self, String> {
        let mut document = Vec::new();
        File::open(keys_path)
            .map_err(|e| e.to_string())?
            .take(MAX_KEYS_BYTES + 1)
            .read_to_end(&mut document)
            .map_err(|e| e.to_string())?;
        if document.len() as u64 > MAX_KEYS_BYTES {
            return Err("receipt key list exceeds its bound".to_owned());
        }
        let entries: Vec<ReceiptKeyEntry> =
            serde_json::from_slice(&document).map_err(|e| e.to_string())?;
        if entries.is_empty() {
            return Err("receipt key list must hold at least one key".to_owned());
        }
        let mut keys: Vec<SequencerAuthorization> = Vec::with_capacity(entries.len());
        for entry in entries {
            let id = base64_key(&entry.key_id)?;
            let public_key = base64_key(&entry.public_key)?;
            if entry.first_height > entry.last_height
                || keys.iter().any(|key| key.sequencer_id() == id)
            {
                return Err("duplicate receipt key ID or invalid height range".to_owned());
            }
            keys.push(SequencerAuthorization::new(
                id,
                public_key,
                entry.first_height,
                entry.last_height,
            ));
        }
        if url.len() > 8192
            || !url.is_ascii()
            || url.bytes().any(|b| b.is_ascii_control() || b == b' ')
            || url.contains(['#', '\\'])
        {
            return Err("chain RPC URL is invalid".to_owned());
        }
        let (https, rest) = if let Some(rest) = url.strip_prefix("https://") {
            (true, rest)
        } else if let Some(rest) = url.strip_prefix("http://") {
            (false, rest)
        } else {
            return Err("chain RPC URL must use http or https".to_owned());
        };
        let end = rest.find(['/', '?']).unwrap_or(rest.len());
        let authority = &rest[..end];
        if authority.is_empty() || authority.contains('@') {
            return Err("chain RPC URL host is invalid".to_owned());
        }
        let (host, port) = if let Some(ipv6) = authority.strip_prefix('[') {
            let (host, suffix) = ipv6.split_once(']').ok_or("invalid IPv6 host")?;
            host.parse::<std::net::Ipv6Addr>()
                .map_err(|_| "invalid IPv6 host")?;
            (host, suffix)
        } else {
            let end = authority.find(':').unwrap_or(authority.len());
            let host = &authority[..end];
            if host.is_empty()
                || !host
                    .bytes()
                    .all(|b| b.is_ascii_alphanumeric() || matches!(b, b'.' | b'-'))
            {
                return Err("chain RPC URL host is invalid".to_owned());
            }
            (host, &authority[end..])
        };
        let port = if port.is_empty() {
            if https {
                443
            } else {
                80
            }
        } else {
            port.strip_prefix(':')
                .ok_or("invalid chain RPC port")?
                .parse::<u16>()
                .map_err(|_| "invalid chain RPC port")?
        };
        if port == 0 {
            return Err("invalid chain RPC port".to_owned());
        }
        let target = match &rest[end..] {
            "" => "/".to_owned(),
            query if query.starts_with('?') => format!("/{query}"),
            path => path.to_owned(),
        };
        let (sender, receiver) = mpsc::sync_channel(1);
        let owned_host = host.to_owned();
        std::thread::spawn(move || {
            let result = (owned_host.as_str(), port)
                .to_socket_addrs()
                .map(|addresses| addresses.take(8).collect::<Vec<_>>());
            let _ = sender.send(result);
        });
        let addresses = receiver
            .recv_timeout(super::CONNECT_TIMEOUT)
            .map_err(|_| "chain RPC DNS deadline exceeded")?
            .map_err(|e| e.to_string())?;
        if addresses.is_empty() {
            return Err("chain RPC host has no addresses".to_owned());
        }
        let tls = if https {
            let pem =
                std::fs::read("/etc/ssl/certs/ca-certificates.crt").map_err(|e| e.to_string())?;
            let mut roots = RootCertStore::empty();
            for certificate in CertificateDer::pem_slice_iter(&pem) {
                roots
                    .add(certificate.map_err(|e| e.to_string())?)
                    .map_err(|e| e.to_string())?;
            }
            if roots.is_empty() {
                return Err("chain RPC TLS roots are empty".to_owned());
            }
            Some(Arc::new(
                ClientConfig::builder()
                    .with_root_certificates(roots)
                    .with_no_client_auth(),
            ))
        } else {
            None
        };
        Ok(Self {
            host: host.to_owned(),
            authority: authority.to_owned(),
            target,
            addresses,
            tls,
            keys,
        })
    }

    pub(super) fn lookup(&self, transaction: [u8; 32]) -> Result<Option<Vec<u8>>, String> {
        self.lookup_receipt(transaction)
            .map(|receipt| receipt.map(|receipt| receipt.bytes))
    }

    pub(super) fn lookup_receipt(
        &self,
        transaction: [u8; 32],
    ) -> Result<Option<ChainReceipt>, String> {
        let deadline = Instant::now() + TIMEOUT;
        let connect_deadline = Instant::now() + super::CONNECT_TIMEOUT;
        let mut connected = None;
        for address in &self.addresses {
            let Some(remaining) = connect_deadline.checked_duration_since(Instant::now()) else {
                break;
            };
            if let Ok(stream) = TcpStream::connect_timeout(address, remaining) {
                connected = Some(stream);
                break;
            }
        }
        let stream = DeadlineStream {
            stream: connected.ok_or("chain RPC connection failed")?,
            deadline,
        };
        let mut stream: Box<dyn ReadWrite> = if let Some(config) = &self.tls {
            let name = ServerName::try_from(self.host.clone()).map_err(|e| e.to_string())?;
            let connection =
                ClientConnection::new(Arc::clone(config), name).map_err(|e| e.to_string())?;
            Box::new(StreamOwned::new(connection, stream))
        } else {
            Box::new(stream)
        };
        let request = json!({"jsonrpc": "2.0", "id": 1, "method": "pax_sequencerReceipt", "params": [format!("0x{}", super::hex(&transaction))]}).to_string();
        write!(stream, "POST {} HTTP/1.0\r\nHost: {}\r\nContent-Type: application/json\r\nAccept: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", self.target, self.authority, request.len(), request).map_err(|e| e.to_string())?;
        stream.flush().map_err(|e| e.to_string())?;
        let mut bytes = Vec::new();
        stream
            .take(MAX_RESPONSE_BYTES + 1)
            .read_to_end(&mut bytes)
            .map_err(|e| e.to_string())?;
        if bytes.len() as u64 > MAX_RESPONSE_BYTES {
            return Err("chain RPC response exceeds its bound".to_owned());
        }
        let end = bytes
            .windows(4)
            .position(|part| part == b"\r\n\r\n")
            .ok_or("chain RPC headers missing")?;
        if end > 16 * 1024 {
            return Err("chain RPC headers exceed their bound".to_owned());
        }
        let mut headers = std::str::from_utf8(&bytes[..end])
            .map_err(|e| e.to_string())?
            .split("\r\n");
        let status = headers.next().ok_or("chain RPC status missing")?;
        let mut parts = status.split_whitespace();
        if !matches!(parts.next(), Some("HTTP/1.0" | "HTTP/1.1")) || parts.next() != Some("200") {
            return Err("chain RPC HTTP failure".to_owned());
        }
        let body = &bytes[end + 4..];
        let mut seen = std::collections::BTreeSet::new();
        for header in headers {
            let (name, value) = header.split_once(':').ok_or("chain RPC header invalid")?;
            let name = name.trim().to_ascii_lowercase();
            if !seen.insert(name.clone()) || name == "transfer-encoding" {
                return Err("chain RPC framing invalid".to_owned());
            }
            if name == "content-length" && value.trim().parse::<usize>().ok() != Some(body.len()) {
                return Err("chain RPC response truncated".to_owned());
            }
        }
        let response: Value = serde_json::from_slice(body).map_err(|e| e.to_string())?;
        let object = response.as_object().ok_or("chain RPC envelope invalid")?;
        if object.len() != 3 || response["jsonrpc"] != "2.0" || response["id"].as_u64() != Some(1) {
            return Err("chain RPC envelope invalid".to_owned());
        }
        let result = object.get("result").ok_or("chain RPC error")?;
        if result.is_null() {
            return Ok(None);
        }
        let hash = |field: &str| -> Result<[u8; 32], String> {
            result[field]
                .as_str()
                .and_then(|s| s.strip_prefix("0x"))
                .and_then(super::parse_hex32)
                .ok_or_else(|| format!("chain RPC {field} invalid"))
        };
        if hash("txHash")? != transaction {
            return Err("chain RPC transaction mismatch".to_owned());
        }
        if result["rung"] == "submitted" {
            return Ok(None);
        }
        if !matches!(result["rung"].as_str(), Some("receipted" | "committed")) {
            return Err("chain RPC rung invalid".to_owned());
        }
        let bytes = super::decode_hex(
            result["receipt"]
                .as_str()
                .and_then(|s| s.strip_prefix("0x"))
                .ok_or("chain RPC receipt missing")?,
            MAX_RESPONSE_BYTES as usize / 2,
        )?;
        let decoded = verify_sequencer_signature(&bytes, self.keys.as_slice())
            .map_err(|e| format!("{e:?}"))?;
        let protocol = decoded
            .protocol()
            .ok_or("chain RPC receipt shape invalid")?;
        let key_id = hash("keyId")?;
        let key = self
            .keys
            .iter()
            .find(|key| key.sequencer_id() == key_id)
            .ok_or("chain RPC key is not pinned")?;
        if protocol.activity_id() != transaction
            || protocol.context_hash() != key_id
            || hash("signingKey")? != key.public_key()
        {
            return Err("chain RPC signed identity mismatch".to_owned());
        }
        Ok(Some(ChainReceipt {
            transaction_hash: transaction,
            bytes,
        }))
    }
}

trait ReadWrite: Read + Write {}
impl<T: Read + Write> ReadWrite for T {}

struct DeadlineStream {
    stream: TcpStream,
    deadline: Instant,
}

impl DeadlineStream {
    fn remaining(&self) -> std::io::Result<Duration> {
        self.deadline
            .checked_duration_since(Instant::now())
            .filter(|d| !d.is_zero())
            .ok_or_else(|| {
                std::io::Error::new(std::io::ErrorKind::TimedOut, "chain RPC deadline exceeded")
            })
    }
}

impl Read for DeadlineStream {
    fn read(&mut self, bytes: &mut [u8]) -> std::io::Result<usize> {
        self.stream.set_read_timeout(Some(self.remaining()?))?;
        self.stream.read(bytes)
    }
}

impl Write for DeadlineStream {
    fn write(&mut self, bytes: &[u8]) -> std::io::Result<usize> {
        self.stream.set_write_timeout(Some(self.remaining()?))?;
        self.stream.write(bytes)
    }
    fn flush(&mut self) -> std::io::Result<()> {
        self.stream.flush()
    }
}
