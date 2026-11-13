use crate::attestor::call::bind_call;
use crate::config::Config;
use crate::store::Store;
use crate::wire::{self, Error, Operation, Request, Result};
use rustls::{ServerConnection, StreamOwned};
use sha2::{Digest, Sha256};
use std::io::{IoSlice, Read, Write};
use std::net::{TcpListener, TcpStream};
use std::sync::atomic::{AtomicUsize, Ordering};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::{Duration, Instant};
use zeroize::Zeroizing;

struct Permit(Arc<AtomicUsize>);
impl Drop for Permit {
    fn drop(&mut self) {
        self.0.fetch_sub(1, Ordering::AcqRel);
    }
}
pub(crate) fn run(
    config: Config,
    clock: &Arc<dyn layerx_types::clock::Clock>,
) -> std::result::Result<(), String> {
    let store = Arc::new(Mutex::new(Store::open(&config)?));
    let listener = TcpListener::bind(config.listen).map_err(|_| "KMS listener unavailable")?;
    let config = Arc::new(config);
    let active = Arc::new(AtomicUsize::new(0));
    for stream in listener.incoming() {
        let Ok(stream) = stream else {
            return Err("KMS accept failed".into());
        };
        if let Err(error) = spawn_connection(
            stream,
            Arc::clone(&config),
            Arc::clone(&store),
            Arc::clone(clock),
            Arc::clone(&active),
        ) {
            if error != Error::Capacity {
                return Err("KMS worker unavailable".into());
            }
        }
    }
    Err("KMS listener stopped".into())
}
pub(crate) fn spawn_connection(
    tcp: TcpStream,
    config: Arc<Config>,
    store: Arc<Mutex<Store>>,
    clock: Arc<dyn layerx_types::clock::Clock>,
    active: Arc<AtomicUsize>,
) -> Result<thread::JoinHandle<()>> {
    let accepted = Instant::now();
    active
        .fetch_update(Ordering::AcqRel, Ordering::Acquire, |count| {
            (count < 16).then_some(count + 1)
        })
        .map_err(|_| Error::Capacity)?;
    let permit = Permit(active);
    thread::Builder::new()
        .name("human-kms".into())
        .spawn(move || {
            let _ = connection_from_accept(tcp, &config, &store, clock.as_ref(), accepted);
            drop(permit);
        })
        .map_err(|_| Error::Unavailable)
}
struct DeadlineTcp<'a> {
    tcp: TcpStream,
    clock: &'a dyn layerx_types::clock::Clock,
    deadline: layerx_types::clock::Deadline,
    transport_expires: Instant,
}
impl Drop for DeadlineTcp<'_> {
    fn drop(&mut self) {
        // Terminate incomplete TLS records as well as completed requests before
        // returning the worker's permit to the listener.
        let _ = self.tcp.shutdown(std::net::Shutdown::Both);
    }
}
impl DeadlineTcp<'_> {
    fn transport_budget(&self) -> std::io::Result<Duration> {
        self.transport_expires
            .checked_duration_since(Instant::now())
            .filter(|remaining| !remaining.is_zero())
            .ok_or_else(|| {
                std::io::Error::new(std::io::ErrorKind::TimedOut, "KMS connection deadline")
            })
    }
    fn budget(&mut self) -> std::io::Result<Duration> {
        // The trusted deadline can expire earlier. Clock RPC latency and worker
        // scheduling must never extend the transport lifetime measured at accept.
        let budget = self.transport_budget()?;
        let remaining = self
            .deadline
            .remaining_bounded(self.clock, budget)
            .map_err(std::io::Error::other)?;
        if remaining.is_zero() {
            return Err(std::io::Error::new(
                std::io::ErrorKind::TimedOut,
                "KMS connection deadline",
            ));
        }
        Ok(remaining.min(self.transport_budget()?))
    }
}
impl Read for DeadlineTcp<'_> {
    fn read(&mut self, bytes: &mut [u8]) -> std::io::Result<usize> {
        let remaining = self.budget()?;
        self.tcp.set_read_timeout(Some(remaining))?;
        self.tcp.read(bytes)
    }
}
impl Write for DeadlineTcp<'_> {
    fn write(&mut self, bytes: &[u8]) -> std::io::Result<usize> {
        let remaining = self.budget()?;
        self.tcp.set_write_timeout(Some(remaining))?;
        self.tcp.write(bytes)
    }
    fn write_vectored(&mut self, buffers: &[IoSlice<'_>]) -> std::io::Result<usize> {
        let remaining = self.budget()?;
        self.tcp.set_write_timeout(Some(remaining))?;
        self.tcp.write_vectored(buffers)
    }
    fn flush(&mut self) -> std::io::Result<()> {
        // TcpStream has no userspace write buffer. TLS writes already check
        // the absolute deadline; flushing must not make another clock RPC.
        self.tcp.flush()
    }
}
fn connection_from_accept(
    tcp: TcpStream,
    config: &Config,
    store: &Mutex<Store>,
    clock: &dyn layerx_types::clock::Clock,
    accepted: Instant,
) -> Result<()> {
    let transport_expires = accepted
        .checked_add(config.deadline)
        .ok_or(Error::Unavailable)?;
    let budget = transport_expires
        .checked_duration_since(Instant::now())
        .filter(|budget| !budget.is_zero())
        .ok_or(Error::Unavailable)?;
    let deadline =
        layerx_types::clock::Deadline::start(clock, budget).map_err(|_| Error::Unavailable)?;
    connection_until(tcp, config, store, clock, deadline, transport_expires)
}
fn connection_until(
    tcp: TcpStream,
    config: &Config,
    store: &Mutex<Store>,
    clock: &dyn layerx_types::clock::Clock,
    deadline: layerx_types::clock::Deadline,
    transport_expires: Instant,
) -> Result<()> {
    tcp.set_nodelay(true).map_err(|_| Error::Unavailable)?;
    let mut tcp = DeadlineTcp {
        tcp,
        clock,
        deadline,
        transport_expires,
    };
    let mut connection =
        ServerConnection::new(Arc::clone(&config.tls)).map_err(|_| Error::Unavailable)?;
    while connection.is_handshaking() {
        connection
            .complete_io(&mut tcp)
            .map_err(|_| Error::Refused)?;
    }
    let leaf = connection
        .peer_certificates()
        .and_then(|chain| chain.first())
        .ok_or(Error::Refused)?;
    let observed: [u8; 32] = Sha256::digest(leaf.as_ref()).into();
    let service = layerx_crypto::ct::eq_fixed(&observed, &config.client_pin);
    let executor = config
        .attestor_evm_client_certificate
        .as_ref()
        .is_some_and(|certificate| {
            let pin: [u8; 32] = Sha256::digest(certificate.as_ref()).into();
            layerx_crypto::ct::eq_fixed(&observed, &pin)
        });
    if !service && !executor {
        return Err(Error::Refused);
    }
    let mut tls = StreamOwned::new(connection, tcp);
    let frame = read_request(&mut tls)?;
    let request = match Request::decode(&frame) {
        Ok(request) => request,
        Err(error) => {
            // An authenticated caller must receive an explicit Sign migration
            // refusal, including when the retired layout cannot be decoded.
            let Some(header) = frame.get(..7).filter(|header| {
                &header[..4] == b"LXKP" && header[6] == wire::signing_contract::OPERATION
            }) else {
                return Err(error);
            };
            return write_response(
                &mut tls,
                &wire::response(
                    u16::from_be_bytes([header[4], header[5]]),
                    header[6],
                    Err(error),
                ),
            );
        }
    };
    let answer = (|| {
        let mut guard = loop {
            tls.sock.budget().map_err(|_| Error::Unavailable)?;
            match store.try_lock() {
                Ok(guard) => break guard,
                Err(std::sync::TryLockError::Poisoned(_)) => return Err(Error::Unavailable),
                Err(std::sync::TryLockError::WouldBlock) => {
                    let remaining = tls.sock.budget().map_err(|_| Error::Unavailable)?;
                    thread::sleep(remaining.min(std::time::Duration::from_millis(2)));
                }
            }
        };
        let now = clock
            .sample(tls.sock.budget().map_err(|_| Error::Unavailable)?)
            .map_err(|_| Error::Unavailable)?
            .unix_seconds();
        let digest = if request.kind == Operation::ExecutorProbe {
            if !executor {
                return Err(Error::Refused);
            }
            None
        } else {
            if !service && !matches!(request.operation, 6 | 8..=10 | 12) {
                return Err(Error::Refused);
            }
            validate_sign(&request, config, now)?
        };
        guard.dispatch(&request, digest, clock, &mut tls.sock.deadline)
    })();
    write_response(
        &mut tls,
        &wire::response(request.version, request.operation, answer),
    )
}
fn read_request(reader: &mut impl Read) -> Result<Zeroizing<Vec<u8>>> {
    let mut prefix = [0_u8; 4];
    reader.read_exact(&mut prefix).map_err(|_| Error::Refused)?;
    let length = usize::try_from(u32::from_be_bytes(prefix)).map_err(|_| Error::Refused)?;
    if length == 0 || length > wire::MAX_FRAME {
        return Err(Error::Refused);
    }
    let mut payload = Zeroizing::new(vec![0_u8; length]);
    reader
        .read_exact(&mut payload)
        .map_err(|_| Error::Refused)?;
    Ok(payload)
}
fn write_response(writer: &mut impl Write, payload: &[u8]) -> Result<()> {
    if payload.len() > wire::MAX_FRAME {
        return Err(Error::Unavailable);
    }
    let length = u32::try_from(payload.len()).map_err(|_| Error::Unavailable)?;
    writer
        .write_all(&length.to_be_bytes())
        .and_then(|()| writer.write_all(payload))
        .and_then(|()| writer.flush())
        .map_err(|_| Error::Unavailable)
}
fn validate_sign(request: &Request<'_>, config: &Config, now: u64) -> Result<Option<[u8; 32]>> {
    let network = u32::try_from(config.chain.chain_id).map_err(|_| Error::Refused)?;
    if request.operation == 13 {
        return if request.network == network && request.class == 1 {
            Ok(None)
        } else {
            Err(Error::Refused)
        };
    }
    if request.operation == 11 {
        let value = serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
        return crate::send::digest(&value, request.binding, network, 3, now).map(Some);
    }
    if request.kind != Operation::Sign {
        return Ok(None);
    }
    sign(request, config, network)
}

fn sign(request: &Request<'_>, config: &Config, network: u32) -> Result<Option<[u8; 32]>> {
    let call = crate::attestor::decode_unsigned_evm_transaction(request.canonical)
        .map_err(|_| Error::Refused)?;
    if call.chain_id != config.chain.chain_id || request.network != network {
        return Err(Error::Refused);
    }
    let actor_length = request.disclosure.get(5..9).ok_or(Error::Refused)?;
    if actor_length != 20_u32.to_be_bytes() {
        return Err(Error::Refused);
    }
    let sender = request
        .disclosure
        .get(9..29)
        .ok_or(Error::Refused)?
        .try_into()
        .map_err(|_| Error::Refused)?;
    if sender != request.from {
        return Err(Error::Refused);
    }
    let disclosure = bind_call(&call, sender).map_err(|_| Error::Refused)?;
    disclosure
        .validate_for_signing()
        .map_err(|_| Error::Refused)?;
    if disclosure.disclosure_bytes().map_err(|_| Error::Refused)? != request.disclosure {
        return Err(Error::Refused);
    }
    let digest: [u8; 32] = sha3::Keccak256::digest(request.canonical).into();
    if !layerx_crypto::ct::eq_fixed(&digest, &call.signing_hash()) {
        return Err(Error::Refused);
    }
    if !layerx_crypto::ct::eq_fixed(&digest, &request.digest) {
        return Err(Error::Integrity);
    }
    Ok(Some(digest))
}
