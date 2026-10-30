use std::io::{Read as _, Write as _};
use std::net::{TcpStream, ToSocketAddrs as _};
use std::time::{Duration, Instant};

use layerx_types::ids::Did;
use serde_json::{json, Value};

use crate::server::{InvocationOutcome, Server, ServerError, FAUCET_REQUEST};

const DEADLINE: Duration = Duration::from_secs(30);
const MAX_RESPONSE_BYTES: usize = 262_144;

#[derive(Debug)]
pub enum FaucetError {
    Server(ServerError),
    InvalidRequest,
    Endpoint,
    Transport(std::io::Error),
    Http(u16),
    InvalidResponse,
    Rpc {
        code: i64,
        message: String,
        data: Option<Value>,
    },
}

pub struct FaucetSession<'a> {
    pub did: &'a Did,
    pub signer_key: &'a [u8; 32],
}

pub fn execute(
    server: &mut Server,
    core_sequence: u64,
    node: &str,
    session: &FaucetSession<'_>,
) -> Result<Value, FaucetError> {
    if session.did != server.binding().agent() {
        return Err(FaucetError::InvalidRequest);
    }
    let did =
        std::str::from_utf8(session.did.as_bytes()).map_err(|_| FaucetError::InvalidRequest)?;
    let signer_key = encode_key(session.signer_key);
    let arguments = serde_json::to_vec(&json!({
        "did": did,
        "public_key": signer_key,
    }))
    .map_err(|_| FaucetError::InvalidRequest)?;
    server
        .execute_committed(core_sequence, FAUCET_REQUEST.name, arguments, |_| {
            let result = request_funds(node, did, session.signer_key);
            let outcome = invocation_outcome(&result);
            (result.map(|tx_hash| json!({"tx_hash": tx_hash})), outcome)
        })
        .map_err(FaucetError::Server)?
}

pub fn invocation_outcome(result: &Result<String, FaucetError>) -> InvocationOutcome {
    match result {
        Ok(_) => InvocationOutcome::Completed,
        Err(FaucetError::Transport(_) | FaucetError::InvalidResponse) => InvocationOutcome::Unknown,
        Err(_) => InvocationOutcome::Refused,
    }
}

pub fn request_funds(node: &str, did: &str, signer_key: &[u8; 32]) -> Result<String, FaucetError> {
    let method_and_identifier = did.strip_prefix("did:").and_then(|did| did.split_once(':'));
    if Did::new(did.as_bytes()).is_err()
        || !method_and_identifier
            .is_some_and(|(method, identifier)| !method.is_empty() && !identifier.is_empty())
        || did.len() > 512
        || !did
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || b"-._:".contains(&byte))
        || signer_key == &[0; 32]
    {
        return Err(FaucetError::InvalidRequest);
    }
    let body = serde_json::to_vec(&json!({
        "jsonrpc": "2.0",
        "id": 1,
        "method": "px_requestFunds",
        "params": [did, encode_key(signer_key)],
    }))
    .map_err(|_| FaucetError::InvalidRequest)?;
    let bytes = post(node, &body)?;
    let response: Value =
        serde_json::from_slice(&bytes).map_err(|_| FaucetError::InvalidResponse)?;
    let object = response.as_object().ok_or(FaucetError::InvalidResponse)?;
    if object.len() != 3 || response["jsonrpc"] != "2.0" || response["id"].as_u64() != Some(1) {
        return Err(FaucetError::InvalidResponse);
    }
    if let Some(error) = object.get("error") {
        let error = error.as_object().ok_or(FaucetError::InvalidResponse)?;
        if error
            .keys()
            .any(|key| !matches!(key.as_str(), "code" | "message" | "data"))
        {
            return Err(FaucetError::InvalidResponse);
        }
        return Err(FaucetError::Rpc {
            code: error
                .get("code")
                .and_then(Value::as_i64)
                .ok_or(FaucetError::InvalidResponse)?,
            message: error
                .get("message")
                .and_then(Value::as_str)
                .ok_or(FaucetError::InvalidResponse)?
                .to_owned(),
            data: error.get("data").cloned(),
        });
    }
    let claim = response["result"]
        .as_object()
        .ok_or(FaucetError::InvalidResponse)?;
    let funded = claim.get("funded").and_then(Value::as_bool) == Some(true)
        && claim
            .get("funding_id")
            .and_then(Value::as_str)
            .is_some_and(|id| {
                !id.is_empty() && id.len() <= 128 && id.bytes().all(|byte| byte.is_ascii_hexdigit())
            })
        && claim
            .get("amount")
            .and_then(Value::as_str)
            .is_some_and(|amount| {
                !amount.is_empty()
                    && amount.len() <= 39
                    && amount.bytes().all(|byte| byte.is_ascii_digit())
                    && amount.bytes().any(|byte| byte != b'0')
            })
        && claim
            .get("network")
            .and_then(Value::as_str)
            .is_some_and(|network| !network.is_empty() && network.len() <= 64);
    if !funded {
        return Err(FaucetError::InvalidResponse);
    }
    let hash = claim
        .get("transaction_id")
        .and_then(Value::as_str)
        .filter(|id| !id.is_empty() && id.len() <= 128)
        .ok_or(FaucetError::InvalidResponse)?;
    Ok(hash.to_owned())
}

fn encode_key(key: &[u8; 32]) -> String {
    key.iter().map(|byte| format!("{byte:02x}")).collect()
}

fn post(node: &str, body: &[u8]) -> Result<Vec<u8>, FaucetError> {
    let rest = node.strip_prefix("http://").ok_or(FaucetError::Endpoint)?;
    if rest.is_empty()
        || rest
            .bytes()
            .any(|byte| byte.is_ascii_control() || byte.is_ascii_whitespace())
        || rest.contains(['@', '?', '#'])
    {
        return Err(FaucetError::Endpoint);
    }
    let (authority, path) = rest
        .find('/')
        .map_or((rest, "/"), |index| rest.split_at(index));
    let (host, port) = if let Some(rest) = authority.strip_prefix('[') {
        let (host, tail) = rest.split_once(']').ok_or(FaucetError::Endpoint)?;
        let port = if tail.is_empty() {
            80
        } else {
            tail.strip_prefix(':')
                .and_then(|port| port.parse::<u16>().ok())
                .ok_or(FaucetError::Endpoint)?
        };
        (host, port)
    } else {
        match authority.split_once(':') {
            Some((host, port)) => (host, port.parse().map_err(|_| FaucetError::Endpoint)?),
            None => (authority, 80),
        }
    };
    if host.is_empty() || port == 0 {
        return Err(FaucetError::Endpoint);
    }
    let expires = Instant::now() + DEADLINE;
    let addresses = (host, port)
        .to_socket_addrs()
        .map_err(FaucetError::Transport)?;
    let mut stream = None;
    let mut last_error = None;
    for address in addresses {
        match TcpStream::connect_timeout(&address, remaining(expires)?) {
            Ok(connected) => {
                stream = Some(connected);
                break;
            }
            Err(error) => last_error = Some(error),
        }
    }
    let mut stream = stream.ok_or_else(|| {
        FaucetError::Transport(last_error.unwrap_or_else(|| {
            std::io::Error::new(std::io::ErrorKind::AddrNotAvailable, "no node address")
        }))
    })?;
    stream
        .set_write_timeout(Some(remaining(expires)?))
        .map_err(FaucetError::Transport)?;
    write!(
        stream,
        "POST {path} HTTP/1.1\r\nHost: {authority}\r\nContent-Type: application/json\r\nAccept: application/json\r\nConnection: close\r\nContent-Length: {}\r\n\r\n",
        body.len(),
    )
    .and_then(|()| stream.write_all(body))
    .map_err(FaucetError::Transport)?;
    let mut response = Vec::new();
    loop {
        stream
            .set_read_timeout(Some(remaining(expires)?))
            .map_err(FaucetError::Transport)?;
        let mut buffer = [0; 4096];
        let count = stream.read(&mut buffer).map_err(FaucetError::Transport)?;
        if count == 0 {
            break;
        }
        if response.len() + count > MAX_RESPONSE_BYTES {
            return Err(FaucetError::InvalidResponse);
        }
        response.extend_from_slice(&buffer[..count]);
    }
    decode_http(&response)
}

fn remaining(expires: Instant) -> Result<Duration, FaucetError> {
    expires
        .checked_duration_since(Instant::now())
        .filter(|duration| !duration.is_zero())
        .ok_or_else(|| {
            FaucetError::Transport(std::io::Error::new(
                std::io::ErrorKind::TimedOut,
                "node RPC deadline",
            ))
        })
}

fn decode_http(response: &[u8]) -> Result<Vec<u8>, FaucetError> {
    let split = response
        .windows(4)
        .position(|bytes| bytes == b"\r\n\r\n")
        .ok_or(FaucetError::InvalidResponse)?;
    let header =
        std::str::from_utf8(&response[..split]).map_err(|_| FaucetError::InvalidResponse)?;
    let mut lines = header.split("\r\n");
    let mut status = lines
        .next()
        .ok_or(FaucetError::InvalidResponse)?
        .splitn(3, ' ');
    if !matches!(status.next(), Some("HTTP/1.0" | "HTTP/1.1")) {
        return Err(FaucetError::InvalidResponse);
    }
    let status: u16 = status
        .next()
        .and_then(|status| status.parse().ok())
        .ok_or(FaucetError::InvalidResponse)?;
    if status != 200 {
        return Err(FaucetError::Http(status));
    }
    let mut length = None;
    let mut chunked = false;
    for line in lines {
        let (name, value) = line.split_once(':').ok_or(FaucetError::InvalidResponse)?;
        if name.eq_ignore_ascii_case("content-length") {
            if length.is_some() {
                return Err(FaucetError::InvalidResponse);
            }
            length = Some(
                value
                    .trim()
                    .parse::<usize>()
                    .map_err(|_| FaucetError::InvalidResponse)?,
            );
        }
        if name.eq_ignore_ascii_case("transfer-encoding") {
            if chunked || !value.trim().eq_ignore_ascii_case("chunked") {
                return Err(FaucetError::InvalidResponse);
            }
            chunked = true;
        }
    }
    let mut body = &response[split + 4..];
    if chunked {
        if length.is_some() {
            return Err(FaucetError::InvalidResponse);
        }
        let mut decoded = Vec::new();
        loop {
            let end = body
                .windows(2)
                .position(|bytes| bytes == b"\r\n")
                .ok_or(FaucetError::InvalidResponse)?;
            let size =
                std::str::from_utf8(&body[..end]).map_err(|_| FaucetError::InvalidResponse)?;
            let size = usize::from_str_radix(
                size.split(';').next().ok_or(FaucetError::InvalidResponse)?,
                16,
            )
            .map_err(|_| FaucetError::InvalidResponse)?;
            body = &body[end + 2..];
            if size == 0 {
                if body != b"\r\n" {
                    return Err(FaucetError::InvalidResponse);
                }
                return Ok(decoded);
            }
            if size > body.len().saturating_sub(2) || &body[size..size + 2] != b"\r\n" {
                return Err(FaucetError::InvalidResponse);
            }
            decoded.extend_from_slice(&body[..size]);
            body = &body[size + 2..];
        }
    }
    if length.is_some_and(|length| length != body.len()) {
        return Err(FaucetError::InvalidResponse);
    }
    Ok(body.to_vec())
}
