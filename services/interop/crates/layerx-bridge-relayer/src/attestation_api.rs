use std::collections::BTreeMap;
use std::fmt::Write as _;
use std::io::{self, Read as _, Write as _};
use std::net::{Shutdown, TcpListener, TcpStream};
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

use serde_json::{json, Value};

use crate::attestation::recover_signer;
use crate::hex;
use crate::journal::{inbound_key, outbound_key, Item};

#[derive(Clone, Debug, Default)]
pub struct AttestationView {
    pub items: BTreeMap<String, Item>,
    pub signatures: BTreeMap<[u8; 32], Vec<[u8; 65]>>,
    pub thresholds: BTreeMap<u64, usize>,
}

const MAX_REQUEST_BYTES: usize = 8192;
const REQUEST_TIMEOUT: Duration = Duration::from_secs(2);

pub fn serve(
    listener: TcpListener,
    view: Arc<RwLock<AttestationView>>,
    origins: Vec<String>,
) -> io::Result<()> {
    for connection in listener.incoming() {
        let mut stream = connection?;
        let result = serve_connection(&mut stream, &view, &origins);
        let _ = stream.shutdown(Shutdown::Both);
        if let Err(error) = result {
            eprintln!("layerx-bridge-relayer: attestation connection: {error}");
        }
    }
    Ok(())
}

fn serve_connection(
    stream: &mut TcpStream,
    view: &RwLock<AttestationView>,
    origins: &[String],
) -> io::Result<()> {
    stream.set_write_timeout(Some(REQUEST_TIMEOUT))?;
    let deadline = Instant::now() + REQUEST_TIMEOUT;
    let mut request = Vec::with_capacity(MAX_REQUEST_BYTES);
    let header_end = loop {
        if let Some(end) = request.windows(4).position(|bytes| bytes == b"\r\n\r\n") {
            break end;
        }
        if request.len() == MAX_REQUEST_BYTES {
            return respond(stream, 413, json!({"error": "request too large"}));
        }
        let Some(remaining) = deadline.checked_duration_since(Instant::now()) else {
            return respond(stream, 408, json!({"error": "request timeout"}));
        };
        stream.set_read_timeout(Some(remaining))?;
        let mut bytes = [0_u8; 1024];
        let capacity = bytes.len().min(MAX_REQUEST_BYTES - request.len());
        match stream.read(&mut bytes[..capacity]) {
            Ok(0) => return respond(stream, 400, json!({"error": "incomplete request"})),
            Ok(count) => request.extend_from_slice(&bytes[..count]),
            Err(error) if error.kind() == io::ErrorKind::Interrupted => {}
            Err(error)
                if matches!(
                    error.kind(),
                    io::ErrorKind::TimedOut | io::ErrorKind::WouldBlock
                ) =>
            {
                return respond(stream, 408, json!({"error": "request timeout"}));
            }
            Err(error) => return Err(error),
        }
    };
    let Ok(headers) = std::str::from_utf8(&request[..header_end]) else {
        return respond(stream, 400, json!({"error": "invalid request"}));
    };
    let mut lines = headers.split("\r\n");
    let parts: Vec<_> = lines.next().unwrap_or_default().split(' ').collect();
    let [method, path, version] = parts.as_slice() else {
        return respond(stream, 400, json!({"error": "invalid request line"}));
    };
    if !matches!(*version, "HTTP/1.0" | "HTTP/1.1") {
        return respond(stream, 400, json!({"error": "invalid request headers"}));
    }
    let mut origin = None;
    for line in lines {
        let Some((name, value)) = line.split_once(':') else {
            return respond(stream, 400, json!({"error": "invalid request headers"}));
        };
        if name.eq_ignore_ascii_case("Origin") {
            if origin.is_some() {
                return respond(stream, 400, json!({"error": "duplicate origin"}));
            }
            origin = Some(value.trim_matches([' ', '\t']));
        }
    }
    let allowed_origin =
        origin.filter(|origin| origins.iter().any(|allowed| allowed.as_str() == *origin));
    let respond = |stream: &mut TcpStream, status, body| {
        respond_with_origin(stream, status, body, allowed_origin)
    };
    let preflight = *method == "OPTIONS" && origin.is_some();
    if preflight && allowed_origin.is_none() {
        return respond(stream, 403, json!({"error": "origin not allowed"}));
    }
    if *method != "GET" && !preflight {
        return respond(stream, 405, json!({"error": "method not allowed"}));
    }
    let segments: Vec<_> = path.split('/').collect();
    let ["", "v1", route @ ("deposit" | "release"), chain, hash, index] = segments.as_slice()
    else {
        return respond(stream, 404, json!({"error": "not found"}));
    };
    let (Some(chain), Some(index)) = (decimal_u64(chain), decimal_u64(index)) else {
        return respond(stream, 400, json!({"error": "invalid chain id or index"}));
    };
    if hash.len() != 66
        || !hash.starts_with("0x")
        || !hash.as_bytes()[2..]
            .iter()
            .all(|byte| matches!(byte, b'0'..=b'9' | b'a'..=b'f'))
    {
        return respond(stream, 400, json!({"error": "invalid transaction hash"}));
    }
    let hash =
        hex::fixed(hash).map_err(|error| io::Error::new(io::ErrorKind::InvalidData, error))?;
    if preflight {
        return respond(stream, 204, Value::Null);
    }
    let answer = {
        let Ok(view) = view.read() else {
            return respond(
                stream,
                500,
                json!({"error": "attestation view unavailable"}),
            );
        };
        if *route == "deposit" {
            deposit_answer(&view, chain, &hash, index)
        } else {
            release_answer(&view, chain, &hash, index)
        }
    };
    match answer {
        Some(answer) => respond(stream, 200, answer),
        None => respond(stream, 404, json!({"error": "not observed"})),
    }
}

fn decimal_u64(text: &str) -> Option<u64> {
    if text.is_empty() || !text.bytes().all(|byte| byte.is_ascii_digit()) {
        return None;
    }
    text.parse().ok()
}

fn respond(stream: &mut TcpStream, status: u16, body: Value) -> io::Result<()> {
    respond_with_origin(stream, status, body, None)
}

fn respond_with_origin(
    stream: &mut TcpStream,
    status: u16,
    body: Value,
    origin: Option<&str>,
) -> io::Result<()> {
    let reason = match status {
        200 => "OK",
        204 => "No Content",
        400 => "Bad Request",
        403 => "Forbidden",
        404 => "Not Found",
        405 => "Method Not Allowed",
        408 => "Request Timeout",
        413 => "Content Too Large",
        _ => "Internal Server Error",
    };
    let body = if status == 204 {
        String::new()
    } else {
        body.to_string()
    };
    let mut cors = String::from("Vary: Origin\r\n");
    if let Some(origin) = origin {
        write!(cors, "Access-Control-Allow-Origin: {origin}\r\n").map_err(io::Error::other)?;
        if status == 204 {
            cors.push_str(
                "Access-Control-Allow-Methods: GET\r\nAccess-Control-Allow-Headers: accept\r\n",
            );
        }
    }
    if status == 204 {
        return write!(
            stream,
            "HTTP/1.1 {status} {reason}\r\nConnection: close\r\n{cors}\r\n"
        );
    }
    write!(
        stream,
        "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n{}{cors}\r\n{body}",
        body.len(),
        if status == 405 { "Allow: GET, OPTIONS\r\n" } else { "" },
    )
}

#[must_use]
pub fn deposit_answer(
    view: &AttestationView,
    chain_id: u64,
    tx_hash: &[u8; 32],
    log_index: u64,
) -> Option<Value> {
    let item = view.items.get(&inbound_key(chain_id, tx_hash, log_index))?;
    let attestation = item.observation.inbound_attestation()?;
    let digest = attestation.digest();
    let signatures = signatures(view, item, &digest);
    let status = status(view, item, chain_id, signatures.len());
    Some(json!({
        "chainId": attestation.chain_id.to_string(),
        "vault": hex::prefixed(&attestation.vault),
        "txHash": hex::prefixed(&attestation.tx_hash),
        "logIndex": attestation.log_index.to_string(),
        "recipient": hex::prefixed(&attestation.recipient),
        "asset": hex::prefixed(&attestation.asset),
        "amount": decimal(&attestation.amount),
        "digest": hex::prefixed(&digest),
        "signatures": signatures,
        "status": status,
    }))
}

#[must_use]
pub fn release_answer(
    view: &AttestationView,
    chain_id: u64,
    paxeer_tx_hash: &[u8; 32],
    paxeer_nonce: u64,
) -> Option<Value> {
    let item = view
        .items
        .get(&outbound_key(chain_id, paxeer_tx_hash, paxeer_nonce))?;
    let attestation = item.observation.outbound_attestation()?;
    let digest = attestation.digest();
    let signatures = signatures(view, item, &digest);
    let status = status(view, item, chain_id, signatures.len());
    Some(json!({
        "chainId": attestation.chain_id.to_string(),
        "vault": hex::prefixed(&attestation.vault),
        "paxeerTxHash": hex::prefixed(&attestation.paxeer_tx_hash),
        "paxeerNonce": attestation.paxeer_nonce.to_string(),
        "recipient": hex::prefixed(&attestation.recipient),
        "asset": hex::prefixed(&attestation.asset),
        "amount": decimal(&attestation.amount),
        "digest": hex::prefixed(&digest),
        "signatures": signatures,
        "status": status,
    }))
}

fn signatures(view: &AttestationView, item: &Item, digest: &[u8; 32]) -> Vec<String> {
    let mut by_signer = BTreeMap::new();
    for signature in item
        .signature
        .iter()
        .chain(view.signatures.get(digest).into_iter().flatten())
    {
        if let Ok(signer) = recover_signer(digest, signature) {
            by_signer.entry(signer).or_insert(*signature);
        }
    }
    by_signer
        .values()
        .map(|signature| hex::prefixed(signature))
        .collect()
}

fn status(view: &AttestationView, item: &Item, chain_id: u64, signatures: usize) -> &'static str {
    if item.signature.is_none() {
        "observed"
    } else if view
        .thresholds
        .get(&chain_id)
        .is_some_and(|threshold| *threshold > 0 && signatures >= *threshold)
    {
        "ready"
    } else {
        "attesting"
    }
}

fn decimal(bytes: &[u8; 32]) -> String {
    let mut digits = vec![0_u16];
    for byte in bytes {
        let mut carry = u16::from(*byte);
        for digit in &mut digits {
            carry += *digit * 256;
            *digit = carry % 10;
            carry /= 10;
        }
        while carry != 0 {
            digits.push(carry % 10);
            carry /= 10;
        }
    }
    digits
        .iter()
        .rev()
        .map(|digit| char::from(b"0123456789"[usize::from(*digit)]))
        .collect()
}
