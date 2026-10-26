//! Sequencer subscriptions.

use std::net::{TcpStream, ToSocketAddrs};
use std::sync::Arc;
use std::time::Duration;

use serde_json::{json, Map, Value};
use tungstenite::{
    client::IntoClientRequest, protocol::WebSocketConfig, stream::MaybeTlsStream, Connector,
    HandshakeError, Message, WebSocket,
};

use crate::sequencer::{RpcRefusal, SequencerError};

const TIMEOUT: Duration = Duration::from_secs(30);
const MAX_MESSAGE_BYTES: usize = 16 * 1_048_576;
const MAX_PENDING_EVENTS: usize = 16;

#[derive(Clone, Debug, PartialEq, Eq)]
pub enum SubscriptionTopic {
    Logs {
        address: Option<[u8; 20]>,
        topics: Vec<[u8; 32]>,
    },
    NewHeads,
}

pub struct NodeSubscription {
    socket: WebSocket<MaybeTlsStream<TcpStream>>,
    id: String,
}

pub fn subscribe(
    url: &str,
    trust_anchors: &[&[u8]],
    topic: SubscriptionTopic,
) -> Result<NodeSubscription, SequencerError> {
    let request = url
        .into_client_request()
        .map_err(|_| SequencerError::Endpoint)?;
    let uri = request.uri();
    let host = uri.host().ok_or(SequencerError::Endpoint)?;
    let secure = match uri.scheme_str() {
        Some("wss") if !trust_anchors.is_empty() => true,
        Some("ws") if matches!(host, "127.0.0.1" | "[::1]" | "localhost") => false,
        _ => return Err(SequencerError::Endpoint),
    };
    let mut roots = rustls::RootCertStore::empty();
    for anchor in trust_anchors {
        roots
            .add(rustls::pki_types::CertificateDer::from(anchor.to_vec()))
            .map_err(|_| SequencerError::Endpoint)?;
    }
    let connector = if secure {
        let config = rustls::ClientConfig::builder_with_provider(Arc::new(
            rustls::crypto::ring::default_provider(),
        ))
        .with_safe_default_protocol_versions()
        .map_err(|_| SequencerError::Tls)?
        .with_root_certificates(roots)
        .with_no_client_auth();
        Connector::Rustls(Arc::new(config))
    } else {
        Connector::Plain
    };
    let params = match topic {
        SubscriptionTopic::Logs { address, topics } => {
            if topics.len() > 4 {
                return Err(SequencerError::Malformed);
            }
            let mut filter = Map::new();
            if let Some(address) = address {
                filter.insert("address".to_owned(), Value::String(hex(&address)));
            }
            filter.insert(
                "topics".to_owned(),
                Value::Array(
                    topics
                        .iter()
                        .map(|topic| Value::String(hex(topic)))
                        .collect(),
                ),
            );
            json!(["logs", filter])
        }
        SubscriptionTopic::NewHeads => json!(["newHeads"]),
    };
    let port = uri.port_u16().unwrap_or(if secure { 443 } else { 80 });
    let host = host.trim_start_matches('[').trim_end_matches(']');
    let addresses = (host, port)
        .to_socket_addrs()
        .map_err(|_| SequencerError::Unavailable)?;
    let mut connected = None;
    for address in addresses {
        if let Ok(stream) = TcpStream::connect_timeout(&address, TIMEOUT) {
            connected = Some(stream);
            break;
        }
    }
    let stream = connected.ok_or(SequencerError::Unavailable)?;
    stream
        .set_read_timeout(Some(TIMEOUT))
        .map_err(|_| SequencerError::Unavailable)?;
    stream
        .set_write_timeout(Some(TIMEOUT))
        .map_err(|_| SequencerError::Unavailable)?;
    let config = WebSocketConfig::default()
        .max_message_size(Some(MAX_MESSAGE_BYTES))
        .max_frame_size(Some(MAX_MESSAGE_BYTES));
    let (mut socket, _) =
        tungstenite::client_tls_with_config(request, stream, Some(config), Some(connector))
            .map_err(|error| match error {
                HandshakeError::Failure(error) => socket_error(error),
                HandshakeError::Interrupted(_) => SequencerError::Unavailable,
            })?;
    socket
        .send(Message::Text(
            json!({"jsonrpc":"2.0", "id":1, "method":"eth_subscribe", "params":params})
                .to_string()
                .into(),
        ))
        .map_err(socket_error)?;
    let value = receive(&mut socket)?;
    let id = response(&value, 1)?
        .as_str()
        .filter(|id| !id.is_empty())
        .ok_or(SequencerError::Malformed)?
        .to_owned();
    Ok(NodeSubscription { socket, id })
}

impl NodeSubscription {
    pub fn next(&mut self) -> Result<Value, SequencerError> {
        let value = receive(&mut self.socket)?;
        notification(&value, &self.id).cloned()
    }

    pub fn close(&mut self) -> Result<(), SequencerError> {
        self.socket
            .send(Message::Text(
                json!({"jsonrpc":"2.0", "id":2, "method":"eth_unsubscribe", "params":[self.id]})
                    .to_string()
                    .into(),
            ))
            .map_err(socket_error)?;
        for _ in 0..=MAX_PENDING_EVENTS {
            let value = receive(&mut self.socket)?;
            if value.get("method").and_then(Value::as_str) == Some("eth_subscription") {
                notification(&value, &self.id)?;
                continue;
            }
            if response(&value, 2)? != &Value::Bool(true) {
                return Err(SequencerError::Malformed);
            }
            return self.socket.close(None).map_err(socket_error);
        }
        Err(SequencerError::Unavailable)
    }
}

fn notification<'a>(value: &'a Value, id: &str) -> Result<&'a Value, SequencerError> {
    if value.get("jsonrpc").and_then(Value::as_str) != Some("2.0")
        || value.get("method").and_then(Value::as_str) != Some("eth_subscription")
        || value.get("id").is_some()
        || value.get("result").is_some()
        || value.get("error").is_some()
    {
        return Err(SequencerError::Malformed);
    }
    let params = value.get("params").ok_or(SequencerError::Malformed)?;
    if params.get("subscription").and_then(Value::as_str) != Some(id) {
        return Err(SequencerError::Malformed);
    }
    params.get("result").ok_or(SequencerError::Malformed)
}

fn response(value: &Value, id: u64) -> Result<&Value, SequencerError> {
    if value.get("jsonrpc").and_then(Value::as_str) != Some("2.0")
        || value.get("id").and_then(Value::as_u64) != Some(id)
        || value.get("method").is_some()
    {
        return Err(SequencerError::Malformed);
    }
    match (value.get("result"), value.get("error")) {
        (Some(result), None) => Ok(result),
        (None, Some(error)) => Err(SequencerError::Rpc(RpcRefusal {
            code: error
                .get("code")
                .and_then(Value::as_i64)
                .ok_or(SequencerError::Malformed)?,
            message: error
                .get("message")
                .and_then(Value::as_str)
                .ok_or(SequencerError::Malformed)?
                .to_owned(),
            data: error.get("data").cloned(),
        })),
        _ => Err(SequencerError::Malformed),
    }
}

fn receive(socket: &mut WebSocket<MaybeTlsStream<TcpStream>>) -> Result<Value, SequencerError> {
    loop {
        match socket.read().map_err(socket_error)? {
            Message::Text(text) => {
                return serde_json::from_str(&text).map_err(|_| SequencerError::Malformed);
            }
            Message::Ping(_) | Message::Pong(_) => socket.flush().map_err(socket_error)?,
            Message::Close(_) => return Err(SequencerError::Unavailable),
            _ => return Err(SequencerError::Malformed),
        }
    }
}

fn socket_error(error: tungstenite::Error) -> SequencerError {
    match error {
        tungstenite::Error::Tls(_) => SequencerError::Tls,
        tungstenite::Error::Http(response) => {
            SequencerError::HttpStatus(response.status().as_u16())
        }
        tungstenite::Error::Protocol(_)
        | tungstenite::Error::Utf8(_)
        | tungstenite::Error::Capacity(_) => SequencerError::Malformed,
        _ => SequencerError::Unavailable,
    }
}

fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut text = String::with_capacity(2 + bytes.len() * 2);
    text.push_str("0x");
    for byte in bytes {
        text.push(char::from(DIGITS[usize::from(byte >> 4)]));
        text.push(char::from(DIGITS[usize::from(byte & 15)]));
    }
    text
}
