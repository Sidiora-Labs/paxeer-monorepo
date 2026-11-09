//! Sequencer subscriptions.

use std::io::{self, Read, Write};
use std::net::{TcpStream, ToSocketAddrs};
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::time::{Duration, Instant};

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
    socket: WebSocket<MaybeTlsStream<DeadlineTcp>>,
    id: String,
}

/// One total response budget, including control traffic. Cancellation is checked
/// between I/O calls and at most every 50ms while waiting on an idle socket.
/// Expiry and cancellation return `SequencerError::Unavailable`.
#[derive(Clone, Debug)]
pub struct WaitOptions {
    pub timeout: Duration,
    pub cancellation: Option<Arc<AtomicBool>>,
}

impl Default for WaitOptions {
    fn default() -> Self {
        Self {
            timeout: TIMEOUT,
            cancellation: None,
        }
    }
}

#[derive(Clone)]
struct OperationDeadline {
    end: Instant,
    options: WaitOptions,
}

impl OperationDeadline {
    fn new(options: &WaitOptions) -> Result<Self, SequencerError> {
        let end = Instant::now()
            .checked_add(options.timeout)
            .ok_or(SequencerError::Malformed)?;
        let deadline = Self {
            end,
            options: options.clone(),
        };
        deadline.remaining()?;
        Ok(deadline)
    }

    fn remaining(&self) -> Result<Duration, SequencerError> {
        if self
            .options
            .cancellation
            .as_ref()
            .is_some_and(|flag| flag.load(Ordering::Acquire))
        {
            return Err(SequencerError::Unavailable);
        }
        self.end
            .checked_duration_since(Instant::now())
            .filter(|left| !left.is_zero())
            .ok_or(SequencerError::Unavailable)
    }

    fn socket(
        &self,
        socket: &mut WebSocket<MaybeTlsStream<DeadlineTcp>>,
    ) -> Result<(), SequencerError> {
        self.remaining()?;
        let stream = match socket.get_mut() {
            MaybeTlsStream::Plain(stream) => stream,
            MaybeTlsStream::Rustls(stream) => &mut stream.sock,
            _ => return Err(SequencerError::Tls),
        };
        stream.deadline = self.clone();
        Ok(())
    }
}

// Enforce the absolute deadline at every underlying I/O, including partial
// writes inside tungstenite flush and reads inside the TLS/HTTP handshake.
struct DeadlineTcp {
    stream: TcpStream,
    deadline: OperationDeadline,
}
impl DeadlineTcp {
    fn prepare(&self) -> io::Result<()> {
        let mut left = self
            .deadline
            .remaining()
            .map_err(|_| io::Error::from(io::ErrorKind::TimedOut))?;
        if self.deadline.options.cancellation.is_some() {
            left = left.min(Duration::from_millis(50));
        }
        self.stream.set_read_timeout(Some(left))?;
        self.stream.set_write_timeout(Some(left))
    }

    fn poll_result<T>(result: io::Result<T>) -> io::Result<T> {
        result.map_err(|error| match error.kind() {
            io::ErrorKind::TimedOut => io::Error::from(io::ErrorKind::WouldBlock),
            _ => error,
        })
    }
}
impl Read for DeadlineTcp {
    fn read(&mut self, bytes: &mut [u8]) -> io::Result<usize> {
        self.prepare()?;
        Self::poll_result(self.stream.read(bytes))
    }
}
impl Write for DeadlineTcp {
    fn write(&mut self, bytes: &[u8]) -> io::Result<usize> {
        self.prepare()?;
        Self::poll_result(self.stream.write(bytes))
    }
    fn flush(&mut self) -> io::Result<()> {
        self.prepare()?;
        Self::poll_result(self.stream.flush())
    }
}

pub fn subscribe(
    url: &str,
    trust_anchors: &[&[u8]],
    topic: SubscriptionTopic,
) -> Result<NodeSubscription, SequencerError> {
    subscribe_with_options(url, trust_anchors, topic, &WaitOptions::default())
}

pub fn subscribe_with_options(
    url: &str,
    trust_anchors: &[&[u8]],
    topic: SubscriptionTopic,
    options: &WaitOptions,
) -> Result<NodeSubscription, SequencerError> {
    let deadline = OperationDeadline::new(options)?;
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
        if let Ok(stream) = TcpStream::connect_timeout(&address, deadline.remaining()?) {
            connected = Some(stream);
            break;
        }
    }
    let stream = connected.ok_or(SequencerError::Unavailable)?;
    let stream = DeadlineTcp {
        stream,
        deadline: deadline.clone(),
    };
    let config = WebSocketConfig::default()
        .max_message_size(Some(MAX_MESSAGE_BYTES))
        .max_frame_size(Some(MAX_MESSAGE_BYTES));
    let mut handshake =
        tungstenite::client_tls_with_config(request, stream, Some(config), Some(connector));
    let (mut socket, _) = loop {
        deadline.remaining()?;
        match handshake {
            Ok(established) => break established,
            Err(HandshakeError::Interrupted(interrupted)) => {
                handshake = interrupted.handshake();
            }
            Err(HandshakeError::Failure(error)) => return Err(socket_error(error)),
        }
    };
    deadline.socket(&mut socket)?;
    socket
        .send(Message::Text(
            json!({"jsonrpc":"2.0", "id":1, "method":"eth_subscribe", "params":params})
                .to_string()
                .into(),
        ))
        .map_err(socket_error)?;
    let value = receive(&mut socket, &deadline)?;
    let id = response(&value, 1)?
        .as_str()
        .filter(|id| !id.is_empty())
        .ok_or(SequencerError::Malformed)?
        .to_owned();
    Ok(NodeSubscription { socket, id })
}

impl NodeSubscription {
    pub fn next(&mut self) -> Result<Value, SequencerError> {
        self.next_with_options(&WaitOptions::default())
    }

    pub fn next_with_options(&mut self, options: &WaitOptions) -> Result<Value, SequencerError> {
        let deadline = OperationDeadline::new(options)?;
        let value = receive(&mut self.socket, &deadline)?;
        notification(&value, &self.id).cloned()
    }

    pub fn close(&mut self) -> Result<(), SequencerError> {
        self.close_with_options(&WaitOptions::default())
    }

    pub fn close_with_options(&mut self, options: &WaitOptions) -> Result<(), SequencerError> {
        let deadline = OperationDeadline::new(options)?;
        deadline.socket(&mut self.socket)?;
        self.socket
            .send(Message::Text(
                json!({"jsonrpc":"2.0", "id":2, "method":"eth_unsubscribe", "params":[self.id]})
                    .to_string()
                    .into(),
            ))
            .map_err(socket_error)?;
        for _ in 0..=MAX_PENDING_EVENTS {
            let value = receive(&mut self.socket, &deadline)?;
            if value.get("method").and_then(Value::as_str) == Some("eth_subscription") {
                notification(&value, &self.id)?;
                continue;
            }
            if response(&value, 2)? != &Value::Bool(true) {
                return Err(SequencerError::Malformed);
            }
            deadline.socket(&mut self.socket)?;
            self.socket.close(None).map_err(socket_error)?;
            return flush(&mut self.socket, &deadline);
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

fn timed_out(error: &tungstenite::Error) -> bool {
    matches!(error, tungstenite::Error::Io(error) if matches!(error.kind(),
        std::io::ErrorKind::TimedOut | std::io::ErrorKind::WouldBlock))
}

fn flush(
    socket: &mut WebSocket<MaybeTlsStream<DeadlineTcp>>,
    deadline: &OperationDeadline,
) -> Result<(), SequencerError> {
    loop {
        deadline.socket(socket)?;
        match socket.flush() {
            Ok(()) => {
                deadline.remaining()?;
                return Ok(());
            }
            Err(error) if timed_out(&error) => {
                deadline.remaining()?;
            }
            Err(error) => return Err(socket_error(error)),
        }
    }
}

fn receive(
    socket: &mut WebSocket<MaybeTlsStream<DeadlineTcp>>,
    deadline: &OperationDeadline,
) -> Result<Value, SequencerError> {
    loop {
        deadline.socket(socket)?;
        let message = match socket.read() {
            Ok(message) => message,
            Err(error) if timed_out(&error) => {
                deadline.remaining()?;
                continue;
            }
            Err(error) => return Err(socket_error(error)),
        };
        deadline.remaining()?;
        match message {
            Message::Text(text) => {
                return serde_json::from_str(&text).map_err(|_| SequencerError::Malformed)
            }
            Message::Ping(_) | Message::Pong(_) => flush(socket, deadline)?,
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
