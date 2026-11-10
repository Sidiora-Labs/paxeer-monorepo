use std::collections::{BTreeMap, BTreeSet};
use std::io::{self, Read, Write};
use std::net::{Shutdown, SocketAddr, TcpListener, TcpStream};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use layerx_agentd::boot::InferenceBootGate;
use layerx_agentd::degraded::{Mode, Reference};
use layerx_client::ai_market::market_program::workers;
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};
use sha2::{Digest as _, Sha256};

pub const MAX_CONNECTIONS: usize = 64;
pub const MAX_FRAME_BYTES: usize = 65_536;
const IO_DEADLINE: Duration = Duration::from_secs(10);

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct HelloFrame {
    pub room_id: [u8; 32],
    pub peer_key: [u8; 32],
    pub nonce: [u8; 32],
    pub signature: Vec<u8>,
    pub manifest: Vec<u8>,
}

impl HelloFrame {
    pub fn signing_bytes(room_id: &[u8; 32], nonce: &[u8; 32]) -> Vec<u8> {
        let mut bytes = b"PAXAI/inference-signal/hello/v1\0".to_vec();
        bytes.extend_from_slice(room_id);
        bytes.extend_from_slice(nonce);
        bytes
    }

    fn verify(&self, nonce: &[u8; 32], room: &[u8; 32]) -> Result<(), SignalError> {
        let signature: [u8; 64] = self
            .signature
            .as_slice()
            .try_into()
            .map_err(|_| SignalError::BadSignature)?;
        layerx_crypto::ed25519::verify_message(
            &self.peer_key,
            &signature,
            &Self::signing_bytes(&self.room_id, &self.nonce),
        )
        .map_err(|_| SignalError::BadSignature)?;
        if &self.nonce != nonce || &self.room_id != room {
            return Err(SignalError::WrongChallenge);
        }
        manifest_digest(&self.manifest)?;
        Ok(())
    }
}

pub fn manifest_digest(manifest: &[u8]) -> Result<[u8; 32], SignalError> {
    let summary = workers::decode_manifest(manifest).map_err(|_| SignalError::InvalidManifest)?;
    if summary.version != 2 {
        return Err(SignalError::InvalidManifest);
    }
    Ok(summary.digest.bytes())
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct LayerAssignment {
    pub peer_key: [u8; 32],
    pub first_layer: u32,
    pub end_layer: u32,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RoomOffer {
    pub room_id: [u8; 32],
    pub model_digest: [u8; 32],
    pub runtime_digest: [u8; 32],
    pub layer_plan: Vec<LayerAssignment>,
    pub price: String,
}

impl RoomOffer {
    pub fn digest(&self) -> Result<[u8; 32], SignalError> {
        let price = self
            .price
            .parse::<u128>()
            .map_err(|_| SignalError::InvalidOffer)?;
        if self.room_id == [0; 32]
            || self.model_digest == [0; 32]
            || self.runtime_digest == [0; 32]
            || self.layer_plan.is_empty()
            || self.layer_plan.len() > MAX_CONNECTIONS
            || self.price != price.to_string()
        {
            return Err(SignalError::InvalidOffer);
        }
        let mut hash = Sha256::new();
        hash.update(b"PAXAI/inference-room-offer/v1\0");
        hash.update(self.room_id);
        hash.update(self.model_digest);
        hash.update(self.runtime_digest);
        hash.update((self.layer_plan.len() as u16).to_be_bytes());
        let mut end = 0;
        let mut peers = BTreeSet::new();
        for assignment in &self.layer_plan {
            if assignment.first_layer != end
                || assignment.end_layer <= assignment.first_layer
                || assignment.peer_key == [0; 32]
                || !peers.insert(assignment.peer_key)
            {
                return Err(SignalError::InvalidOffer);
            }
            end = assignment.end_layer;
            hash.update(assignment.peer_key);
            hash.update(assignment.first_layer.to_be_bytes());
            hash.update(assignment.end_layer.to_be_bytes());
        }
        hash.update(price.to_be_bytes());
        Ok(hash.finalize().into())
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "snake_case")]
pub enum SignalError {
    BadSignature,
    WrongChallenge,
    InvalidManifest,
    InvalidOffer,
    ChainUnavailable,
    Unenrolled,
    ManifestMismatch,
    OfferMismatch,
    DuplicatePeer,
    UnknownPeer,
    InvalidFrame,
    FrameTooLarge,
    Degraded,
    Backpressure,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(rename_all = "lowercase")]
pub enum DescriptionKind {
    Offer,
    Answer,
    Pranswer,
    Rollback,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct SessionDescription {
    #[serde(rename = "type")]
    pub kind: DescriptionKind,
    pub sdp: String,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct IceCandidate {
    pub candidate: String,
    pub sdp_mid: Option<String>,
    pub sdp_m_line_index: Option<u16>,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
pub enum ClientFrame {
    Hello(HelloFrame),
    Description {
        to: [u8; 32],
        description: SessionDescription,
    },
    Candidate {
        to: [u8; 32],
        candidate: IceCandidate,
    },
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
pub enum ServerFrame {
    Challenge {
        nonce: [u8; 32],
    },
    Offer {
        offer: RoomOffer,
    },
    Description {
        from: [u8; 32],
        description: SessionDescription,
    },
    Candidate {
        from: [u8; 32],
        candidate: IceCandidate,
    },
    Error {
        code: SignalError,
    },
}

pub trait SignalChain: Send + Sync + 'static {
    fn reference(&self) -> Result<Reference, SignalError>;
    fn read(&self, method: &str, params: Value) -> Result<Value, SignalError>;
}

#[derive(Clone)]
pub struct SignalAuthority {
    chain: Arc<dyn SignalChain>,
    chain_id: u64,
    program_id: [u8; 32],
}

struct RpcSignalChain {
    agent: ureq::Agent,
    endpoint: String,
    bearer: String,
    gate: Arc<Mutex<InferenceBootGate>>,
}

impl SignalAuthority {
    pub fn new(
        agent: ureq::Agent,
        endpoint: String,
        bearer: String,
        chain_id: u64,
        program_id: [u8; 32],
        gate: Arc<Mutex<InferenceBootGate>>,
    ) -> Result<Self, String> {
        let uri: ureq::http::Uri = endpoint.parse().map_err(|_| "invalid signal node URL")?;
        if uri.path() != "/"
            || uri.query().is_some()
            || uri
                .authority()
                .is_none_or(|authority| authority.as_str().contains('@'))
            || endpoint.contains('#')
            || !matches!(uri.scheme_str(), Some("http" | "https"))
            || (uri.scheme_str() == Some("http")
                && !matches!(uri.host(), Some("127.0.0.1" | "localhost" | "[::1]")))
            || chain_id == 0
            || program_id == [0; 32]
        {
            return Err("invalid signal node configuration".to_owned());
        }
        Ok(Self::with_chain(
            Arc::new(RpcSignalChain {
                agent,
                endpoint,
                bearer,
                gate,
            }),
            chain_id,
            program_id,
        ))
    }

    pub fn with_chain(chain: Arc<dyn SignalChain>, chain_id: u64, program_id: [u8; 32]) -> Self {
        Self {
            chain,
            chain_id,
            program_id,
        }
    }

    fn reference(&self) -> Result<Reference, SignalError> {
        self.chain.reference()
    }

    fn check_head(&self, reference: Reference) -> Result<(), SignalError> {
        let head = self
            .chain
            .read("eth_getBlockByNumber", json!(["latest", false]))?;
        if head["number"] != format!("0x{:x}", reference.head_sequence)
            || head["hash"] != format!("0x{}", hex(&reference.checkpoint))
        {
            return Err(SignalError::ChainUnavailable);
        }
        Ok(())
    }

    fn admit(&self, hello: &HelloFrame, offer: &RoomOffer) -> Result<(), SignalError> {
        let reference = self.reference()?;
        let chain = self.chain.read("eth_chainId", json!([]))?;
        if chain != format!("0x{:x}", self.chain_id) {
            return Err(SignalError::ChainUnavailable);
        }
        self.check_head(reference)?;
        let room = self
            .chain
            .read("px_getInferenceRoom", json!([hex(&offer.room_id)]))?;
        if room["room_id"] != hex(&offer.room_id)
            || room["state"] != 0
            || room["model_digest"] != hex(&offer.model_digest)
            || room["runtime_digest"] != hex(&offer.runtime_digest)
            || room["offer_digest"] != hex(&offer.digest()?)
        {
            return Err(SignalError::OfferMismatch);
        }
        let summary =
            workers::decode_manifest(&hello.manifest).map_err(|_| SignalError::InvalidManifest)?;
        if summary.market.bytes()
            != layerx_agentd::boot::inference_market::market_id(self.chain_id, &self.program_id)
        {
            return Err(SignalError::InvalidManifest);
        }
        let peer = self.chain.read(
            "px_getInferencePeer",
            json!([hex(summary.owner.as_bytes())]),
        )?;
        if peer.is_null() || peer["enrolled"] == false {
            return Err(SignalError::Unenrolled);
        }
        if peer["peer_key"] != hex(summary.owner.as_bytes()) || peer["enrolled"] != true {
            return Err(SignalError::ChainUnavailable);
        }
        if peer["delegate"] != hex(&hello.peer_key) {
            return Err(SignalError::Unenrolled);
        }
        if peer["metadata"] != hex(&manifest_digest(&hello.manifest)?)
            || peer["worker"] != hex(summary.worker.as_bytes())
            || peer["generation"] != summary.generation.to_string()
            || peer["key_version"] != summary.key_version.to_string()
            || peer["metadata_revision"] != summary.revision.to_string()
            || peer["valid_from"] != summary.valid_from.to_string()
            || peer["expiry"] != summary.expiry.to_string()
            || reference.head_sequence < summary.valid_from
            || reference.head_sequence >= summary.expiry
        {
            return Err(SignalError::ManifestMismatch);
        }
        self.check_head(reference)?;
        if self.reference()? != reference {
            return Err(SignalError::ChainUnavailable);
        }
        Ok(())
    }
}

impl SignalChain for RpcSignalChain {
    fn reference(&self) -> Result<Reference, SignalError> {
        let status = self
            .gate
            .lock()
            .map_err(|_| SignalError::Degraded)?
            .status();
        if status.mode != Mode::Healthy {
            return Err(SignalError::Degraded);
        }
        status.reference.ok_or(SignalError::Degraded)
    }

    fn read(&self, method: &str, params: Value) -> Result<Value, SignalError> {
        let mut response = self
            .agent
            .post(&self.endpoint)
            .header("Authorization", &format!("Bearer {}", self.bearer))
            .send_json(json!({"jsonrpc": "2.0", "id": 1, "method": method, "params": params}))
            .map_err(|_| SignalError::ChainUnavailable)?;
        if response.status().as_u16() != 200 {
            return Err(SignalError::ChainUnavailable);
        }
        let bytes = response
            .body_mut()
            .with_config()
            .limit(MAX_FRAME_BYTES as u64)
            .read_to_vec()
            .map_err(|_| SignalError::ChainUnavailable)?;
        let document: Value =
            serde_json::from_slice(&bytes).map_err(|_| SignalError::ChainUnavailable)?;
        if document["jsonrpc"] != "2.0"
            || document["id"] != 1
            || document.get("error").is_some()
            || document.get("result").is_none()
        {
            return Err(SignalError::ChainUnavailable);
        }
        Ok(document["result"].clone())
    }
}

fn hex(bytes: &[u8]) -> String {
    layerx_programs::hex::encode(bytes)
}

struct Peer {
    connection: u64,
    writer: Arc<Mutex<TcpStream>>,
}

#[derive(Default)]
struct Connections {
    sockets: BTreeMap<u64, TcpStream>,
    peers: BTreeMap<[u8; 32], Peer>,
}

struct Connection {
    id: u64,
    shared: Arc<Mutex<Connections>>,
}

impl Drop for Connection {
    fn drop(&mut self) {
        if let Ok(mut connections) = self.shared.lock() {
            connections
                .peers
                .retain(|_, peer| peer.connection != self.id);
            if let Some(stream) = connections.sockets.remove(&self.id) {
                let _ = stream.shutdown(Shutdown::Both);
            }
        }
    }
}

pub struct SignalListener {
    address: SocketAddr,
    stop: Arc<AtomicBool>,
    shared: Arc<Mutex<Connections>>,
    thread: Option<JoinHandle<Result<(), String>>>,
}

impl SignalListener {
    pub fn start(
        listener: TcpListener,
        offer: RoomOffer,
        authority: SignalAuthority,
    ) -> Result<Self, String> {
        offer
            .digest()
            .map_err(|error| format!("invalid signal offer: {error:?}"))?;
        listener
            .set_nonblocking(true)
            .map_err(|error| error.to_string())?;
        let address = listener.local_addr().map_err(|error| error.to_string())?;
        let stop = Arc::new(AtomicBool::new(false));
        let shared = Arc::new(Mutex::new(Connections::default()));
        let worker_stop = Arc::clone(&stop);
        let worker_shared = Arc::clone(&shared);
        let worker = thread::Builder::new().name("inference-signal".to_owned()).spawn(move || {
            let next = AtomicU64::new(1);
            let mut workers: Vec<JoinHandle<()>> = Vec::new();
            let result = (|| {
                while !worker_stop.load(Ordering::Acquire) {
                    let mut i = 0;
                    while i < workers.len() {
                        if workers[i].is_finished() {
                            workers.swap_remove(i).join().map_err(|_| "signal connection panicked")?;
                        } else { i += 1; }
                    }
                    match listener.accept() {
                        Ok((mut stream, _)) => {
                            stream.set_read_timeout(Some(IO_DEADLINE)).map_err(|error| error.to_string())?;
                            stream.set_write_timeout(Some(IO_DEADLINE)).map_err(|error| error.to_string())?;
                            let id = next.fetch_add(1, Ordering::Relaxed);
                            let mut connections = worker_shared.lock().map_err(|_| "signal connections unavailable")?;
                            if connections.sockets.len() == MAX_CONNECTIONS {
                                drop(connections);
                                let _ = stream.write_all(b"HTTP/1.1 503 Service Unavailable\r\nConnection: close\r\nContent-Length: 0\r\n\r\n");
                                continue;
                            }
                            connections.sockets.insert(id, stream.try_clone().map_err(|error| error.to_string())?);
                            drop(connections);
                            let connection = Connection { id, shared: Arc::clone(&worker_shared) };
                            let offer = offer.clone();
                            let authority = authority.clone();
                            let worker = thread::Builder::new().name("inference-signal-peer".to_owned()).spawn(move || {
                                let _connection = connection;
                                let _ = serve_connection(&mut stream, &offer, &authority, &_connection);
                            }).map_err(|error| error.to_string())?;
                            workers.push(worker);
                        }
                        Err(error) if error.kind() == io::ErrorKind::WouldBlock => thread::sleep(Duration::from_millis(5)),
                        Err(error) => return Err(error.to_string()),
                    }
                }
                Ok(())
            })();
            if let Ok(connections) = worker_shared.lock() {
                for stream in connections.sockets.values() { let _ = stream.shutdown(Shutdown::Both); }
            }
            for worker in workers { let _ = worker.join(); }
            result
        }).map_err(|error| error.to_string())?;
        Ok(Self {
            address,
            stop,
            shared,
            thread: Some(worker),
        })
    }

    pub fn local_addr(&self) -> SocketAddr {
        self.address
    }

    pub fn check(&self) -> Result<(), String> {
        if self.thread.as_ref().is_none_or(JoinHandle::is_finished) {
            Err("inference signalling listener terminated".to_owned())
        } else {
            Ok(())
        }
    }
}

impl Drop for SignalListener {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        if let Ok(connections) = self.shared.lock() {
            for stream in connections.sockets.values() {
                let _ = stream.shutdown(Shutdown::Both);
            }
        }
        if let Some(worker) = self.thread.take() {
            let _ = worker.join();
        }
    }
}

fn serve_connection(
    stream: &mut TcpStream,
    offer: &RoomOffer,
    authority: &SignalAuthority,
    connection: &Connection,
) -> Result<(), SignalError> {
    if !upgrade(stream, authority)? {
        return Ok(());
    }
    let writer = Arc::new(Mutex::new(
        stream.try_clone().map_err(|_| SignalError::Backpressure)?,
    ));
    let mut nonce = [0; 32];
    getrandom::fill(&mut nonce).map_err(|_| SignalError::ChainUnavailable)?;
    send(&writer, &ServerFrame::Challenge { nonce })?;
    let result = (|| {
        let mut fragments = None;
        let ClientFrame::Hello(hello) = receive(stream, &writer, &mut fragments)? else {
            return Err(SignalError::InvalidFrame);
        };
        hello.verify(&nonce, &offer.room_id)?;
        authority.admit(&hello, offer)?;
        {
            let mut connections = connection
                .shared
                .lock()
                .map_err(|_| SignalError::Backpressure)?;
            if connections.peers.contains_key(&hello.peer_key) {
                return Err(SignalError::DuplicatePeer);
            }
            send(
                &writer,
                &ServerFrame::Offer {
                    offer: offer.clone(),
                },
            )?;
            connections.peers.insert(
                hello.peer_key,
                Peer {
                    connection: connection.id,
                    writer: Arc::clone(&writer),
                },
            );
        }
        loop {
            match receive(stream, &writer, &mut fragments)? {
                ClientFrame::Hello(_) => return Err(SignalError::InvalidFrame),
                ClientFrame::Description { to, description } => {
                    relay_description(&connection.shared, &hello.peer_key, &to, description)?
                }
                ClientFrame::Candidate { to, candidate } => relay(
                    &connection.shared,
                    &to,
                    &ServerFrame::Candidate {
                        from: hello.peer_key,
                        candidate,
                    },
                )?,
            }
        }
    })();
    if let Err(code) = &result {
        let _ = send(&writer, &ServerFrame::Error { code: code.clone() });
        let close = if *code == SignalError::FrameTooLarge {
            1009u16
        } else {
            1008u16
        };
        if let Ok(mut writer) = writer.lock() {
            let _ = write_frame(&mut writer, 8, &close.to_be_bytes());
        }
    }
    result
}

fn relay_description(
    shared: &Arc<Mutex<Connections>>,
    from: &[u8; 32],
    to: &[u8; 32],
    description: SessionDescription,
) -> Result<(), SignalError> {
    relay(
        shared,
        to,
        &ServerFrame::Description {
            from: *from,
            description,
        },
    )
}

fn relay(
    shared: &Arc<Mutex<Connections>>,
    to: &[u8; 32],
    frame: &ServerFrame,
) -> Result<(), SignalError> {
    let writer = {
        let connections = shared.lock().map_err(|_| SignalError::Backpressure)?;
        Arc::clone(
            &connections
                .peers
                .get(to)
                .ok_or(SignalError::UnknownPeer)?
                .writer,
        )
    };
    send(&writer, frame)
}

fn send(writer: &Arc<Mutex<TcpStream>>, frame: &ServerFrame) -> Result<(), SignalError> {
    let bytes = serde_json::to_vec(frame).map_err(|_| SignalError::InvalidFrame)?;
    if bytes.len() > MAX_FRAME_BYTES {
        return Err(SignalError::FrameTooLarge);
    }
    let mut writer = writer.lock().map_err(|_| SignalError::Backpressure)?;
    write_frame(&mut writer, 1, &bytes).map_err(|_| SignalError::Backpressure)
}

fn write_frame(stream: &mut TcpStream, opcode: u8, bytes: &[u8]) -> io::Result<()> {
    let mut header = vec![0x80 | opcode];
    match bytes.len() {
        0..=125 => header.push(bytes.len() as u8),
        126..=65535 => {
            header.push(126);
            header.extend_from_slice(&(bytes.len() as u16).to_be_bytes());
        }
        _ => {
            header.push(127);
            header.extend_from_slice(&(bytes.len() as u64).to_be_bytes());
        }
    }
    stream.write_all(&header)?;
    stream.write_all(bytes)
}

fn receive(
    stream: &mut TcpStream,
    writer: &Arc<Mutex<TcpStream>>,
    fragments: &mut Option<Vec<u8>>,
) -> Result<ClientFrame, SignalError> {
    loop {
        let mut header = [0; 2];
        stream
            .read_exact(&mut header)
            .map_err(|_| SignalError::InvalidFrame)?;
        let final_frame = header[0] & 0x80 != 0;
        let opcode = header[0] & 0x0f;
        if header[0] & 0x70 != 0 || header[1] & 0x80 == 0 {
            return Err(SignalError::InvalidFrame);
        }
        let length = match header[1] & 0x7f {
            126 => {
                let mut length = [0; 2];
                stream
                    .read_exact(&mut length)
                    .map_err(|_| SignalError::InvalidFrame)?;
                let length = u64::from(u16::from_be_bytes(length));
                if length < 126 {
                    return Err(SignalError::InvalidFrame);
                }
                length
            }
            127 => {
                let mut length = [0; 8];
                stream
                    .read_exact(&mut length)
                    .map_err(|_| SignalError::InvalidFrame)?;
                let length = u64::from_be_bytes(length);
                if length <= 65535 || length >> 63 != 0 {
                    return Err(SignalError::InvalidFrame);
                }
                length
            }
            length => u64::from(length),
        };
        if length > MAX_FRAME_BYTES as u64 {
            return Err(SignalError::FrameTooLarge);
        }
        if opcode >= 8 && (!final_frame || length > 125) {
            return Err(SignalError::InvalidFrame);
        }
        let mut mask = [0; 4];
        stream
            .read_exact(&mut mask)
            .map_err(|_| SignalError::InvalidFrame)?;
        let mut bytes = vec![0; length as usize];
        stream
            .read_exact(&mut bytes)
            .map_err(|_| SignalError::InvalidFrame)?;
        for (i, byte) in bytes.iter_mut().enumerate() {
            *byte ^= mask[i % 4];
        }
        match opcode {
            8 => return Err(SignalError::InvalidFrame),
            9 => {
                let mut writer = writer.lock().map_err(|_| SignalError::Backpressure)?;
                write_frame(&mut writer, 10, &bytes).map_err(|_| SignalError::Backpressure)?;
                continue;
            }
            10 => continue,
            1 if fragments.is_none() => {
                if final_frame {
                    return serde_json::from_slice(&bytes).map_err(|_| SignalError::InvalidFrame);
                }
                *fragments = Some(bytes);
            }
            0 => {
                let pending = fragments.as_mut().ok_or(SignalError::InvalidFrame)?;
                if pending.len() + bytes.len() > MAX_FRAME_BYTES {
                    return Err(SignalError::FrameTooLarge);
                }
                pending.extend_from_slice(&bytes);
                if final_frame {
                    let bytes = fragments.take().ok_or(SignalError::InvalidFrame)?;
                    return serde_json::from_slice(&bytes).map_err(|_| SignalError::InvalidFrame);
                }
            }
            _ => return Err(SignalError::InvalidFrame),
        }
    }
}

fn upgrade(stream: &mut TcpStream, authority: &SignalAuthority) -> Result<bool, SignalError> {
    let mut headers = Vec::new();
    while !headers.ends_with(b"\r\n\r\n") {
        if headers.len() == 16_384 {
            return Err(SignalError::InvalidFrame);
        }
        let mut byte = [0];
        stream
            .read_exact(&mut byte)
            .map_err(|_| SignalError::InvalidFrame)?;
        headers.push(byte[0]);
    }
    let request = std::str::from_utf8(&headers).map_err(|_| SignalError::InvalidFrame)?;
    if request.starts_with("GET /healthz HTTP/1.1\r\n") {
        let ready = authority.reference().is_ok();
        let status = if ready { 200 } else { 503 };
        let body = json!({"ready": ready}).to_string();
        write!(stream, "HTTP/1.1 {status} Response\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\nCache-Control: no-store\r\n\r\n{body}", body.len()).map_err(|_| SignalError::Backpressure)?;
        return Ok(false);
    }
    let mut lines = request.split("\r\n");
    if lines.next() != Some("GET / HTTP/1.1") {
        return Err(SignalError::InvalidFrame);
    }
    let mut fields = BTreeMap::new();
    for line in lines.take_while(|line| !line.is_empty()) {
        let (name, value) = line.split_once(':').ok_or(SignalError::InvalidFrame)?;
        if fields
            .insert(name.to_ascii_lowercase(), value.trim())
            .is_some()
        {
            return Err(SignalError::InvalidFrame);
        }
    }
    let key = fields
        .get("sec-websocket-key")
        .ok_or(SignalError::InvalidFrame)?;
    if fields.get("host").is_none_or(|host| host.is_empty())
        || fields.get("sec-websocket-version") != Some(&"13")
        || fields
            .get("upgrade")
            .is_none_or(|value| !value.eq_ignore_ascii_case("websocket"))
        || fields.get("connection").is_none_or(|value| {
            !value
                .split(',')
                .any(|token| token.trim().eq_ignore_ascii_case("upgrade"))
        })
        || key.len() != 24
        || !key.is_ascii()
        || !key.ends_with("==")
        || !key[..22]
            .bytes()
            .all(|byte| byte.is_ascii_alphanumeric() || matches!(byte, b'+' | b'/'))
    {
        return Err(SignalError::InvalidFrame);
    }
    let accept = websocket_accept(key);
    write!(stream, "HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\nSec-WebSocket-Accept: {accept}\r\n\r\n").map_err(|_| SignalError::Backpressure)?;
    Ok(true)
}

fn websocket_accept(key: &str) -> String {
    let mut input = format!("{key}258EAFA5-E914-47DA-95CA-C5AB0DC85B11").into_bytes();
    let bit_length = (input.len() as u64) * 8;
    input.push(0x80);
    while input.len() % 64 != 56 {
        input.push(0);
    }
    input.extend_from_slice(&bit_length.to_be_bytes());
    let mut state = [
        0x67452301u32,
        0xefcdab89,
        0x98badcfe,
        0x10325476,
        0xc3d2e1f0,
    ];
    for block in input.chunks_exact(64) {
        let mut words = [0u32; 80];
        for (i, chunk) in block.chunks_exact(4).enumerate() {
            words[i] = u32::from_be_bytes([chunk[0], chunk[1], chunk[2], chunk[3]]);
        }
        for i in 16..80 {
            words[i] = (words[i - 3] ^ words[i - 8] ^ words[i - 14] ^ words[i - 16]).rotate_left(1);
        }
        let [mut a, mut b, mut c, mut d, mut e] = state;
        for (i, word) in words.iter().enumerate() {
            let (f, k) = match i {
                0..=19 => ((b & c) | ((!b) & d), 0x5a827999u32),
                20..=39 => (b ^ c ^ d, 0x6ed9eba1),
                40..=59 => ((b & c) | (b & d) | (c & d), 0x8f1bbcdc),
                _ => (b ^ c ^ d, 0xca62c1d6),
            };
            let next = a
                .rotate_left(5)
                .wrapping_add(f)
                .wrapping_add(e)
                .wrapping_add(k)
                .wrapping_add(*word);
            e = d;
            d = c;
            c = b.rotate_left(30);
            b = a;
            a = next;
        }
        for (target, value) in state.iter_mut().zip([a, b, c, d, e]) {
            *target = target.wrapping_add(value);
        }
    }
    let digest: Vec<_> = state.iter().flat_map(|word| word.to_be_bytes()).collect();
    const BASE64: &[u8; 64] = b"ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/";
    let mut encoded = String::new();
    for chunk in digest.chunks(3) {
        let a = chunk[0];
        let b = chunk.get(1).copied().unwrap_or(0);
        let c = chunk.get(2).copied().unwrap_or(0);
        encoded.push(char::from(BASE64[usize::from(a >> 2)]));
        encoded.push(char::from(BASE64[usize::from(((a & 3) << 4) | (b >> 4))]));
        encoded.push(if chunk.len() > 1 {
            char::from(BASE64[usize::from(((b & 15) << 2) | (c >> 6))])
        } else {
            '='
        });
        encoded.push(if chunk.len() > 2 {
            char::from(BASE64[usize::from(c & 63)])
        } else {
            '='
        });
    }
    encoded
}
