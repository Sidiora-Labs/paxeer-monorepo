use std::collections::BTreeMap;
use std::fmt::Write as _;
use std::io::{self, Read as _, Write as _};
use std::net::{Shutdown, TcpListener, TcpStream};
use std::sync::{Arc, RwLock};
use std::time::{Duration, Instant};

use serde_json::{json, Value};

use crate::attestation::eligible_signatures;
use crate::cosign::CosignDirectory;
use crate::hex;
use crate::journal::{
    inbound_key, outbound_key, History, Item, Journal, JournalError, Observation,
};

/// Identity of the consumer whose authenticated policy authorizes a bundle.
#[derive(Clone, Copy, Debug, Eq, PartialEq, Ord, PartialOrd)]
pub enum PolicyDestination {
    Inbound {
        chain_id: u64,
        bridge: [u8; 20],
        source_chain_id: u64,
        source_vault: [u8; 20],
    },
    EvmRelease {
        chain_id: u64,
        vault: [u8; 20],
    },
    SolanaRelease {
        chain_id: u64,
        program: [u8; 32],
        vault: [u8; 20],
    },
}

impl PolicyDestination {
    fn accepts(&self, observation: &Observation) -> bool {
        match (self, observation) {
            (
                Self::Inbound {
                    source_chain_id,
                    source_vault,
                    ..
                },
                Observation::Inbound {
                    chain_id, vault, ..
                },
            ) => source_chain_id == chain_id && source_vault == vault,
            (
                Self::EvmRelease {
                    chain_id: destination_chain,
                    vault: destination_vault,
                }
                | Self::SolanaRelease {
                    chain_id: destination_chain,
                    vault: destination_vault,
                    ..
                },
                Observation::Outbound {
                    chain_id, vault, ..
                },
            ) => destination_chain == chain_id && destination_vault == vault,
            _ => false,
        }
    }
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub struct AuthenticatedPolicy {
    pub destination: PolicyDestination,
    pub threshold: usize,
    pub attestors: Vec<[u8; 20]>,
}

#[derive(Clone, Debug, Default)]
pub struct AttestationView {
    pub items: BTreeMap<String, Item>,
    pub signatures: BTreeMap<[u8; 32], Vec<[u8; 65]>>,
    /// Informational summary for chains with an unambiguous threshold.
    /// Readiness and membership use each item's authenticated policy.
    pub thresholds: BTreeMap<u64, usize>,
}

impl AttestationView {
    /// Takes a journal snapshot without treating durable signatures as durable
    /// policy. Only current authenticated destination policies can authorize it.
    #[must_use]
    pub fn from_journal(
        journal: &Journal,
        cosign: Option<&CosignDirectory>,
        policies: &BTreeMap<PolicyDestination, AuthenticatedPolicy>,
    ) -> Self {
        let mut view = Self {
            items: journal.state().items.clone(),
            ..Self::default()
        };
        let mut summaries: BTreeMap<u64, Option<usize>> = BTreeMap::new();
        for item in view.items.values_mut() {
            // Ambiguous destination identity is unknown, never last-writer-wins.
            let mut matches = policies.iter().filter(|(destination, policy)| {
                **destination == policy.destination && destination.accepts(&item.observation)
            });
            item.attestation_policy = matches.next().map(|(_, policy)| policy.clone());
            if matches.next().is_some() {
                item.attestation_policy = None;
            }
            let (Observation::Inbound { chain_id, .. } | Observation::Outbound { chain_id, .. }) =
                &item.observation;
            if let Some(policy) = &item.attestation_policy {
                summaries
                    .entry(*chain_id)
                    .and_modify(|summary| {
                        if *summary != Some(policy.threshold) {
                            *summary = None;
                        }
                    })
                    .or_insert(Some(policy.threshold));
            }
            let digest = item
                .observation
                .inbound_attestation()
                .map(|attestation| attestation.digest())
                .or_else(|| {
                    item.observation
                        .outbound_attestation()
                        .map(|attestation| attestation.digest())
                });
            if let Some(digest) = digest {
                let collected = cosign
                    .map(|cosign| cosign.collect(&digest))
                    .unwrap_or_default();
                view.signatures.insert(digest, collected);
            }
        }
        view.thresholds = summaries
            .into_iter()
            .filter_map(|(chain, threshold)| threshold.map(|threshold| (chain, threshold)))
            .collect();
        view
    }
}

/// Serving snapshots copy pending work only. Completed bundles are read by key
/// from the journal's durable index and cosigns are collected for that request.
pub struct AttestationSnapshot {
    pub pending: AttestationView,
    history: History,
    cosign: Option<CosignDirectory>,
    policies: BTreeMap<PolicyDestination, AuthenticatedPolicy>,
}

impl std::ops::Deref for AttestationSnapshot {
    type Target = AttestationView;
    fn deref(&self) -> &Self::Target {
        &self.pending
    }
}

impl AttestationSnapshot {
    pub fn from_journal(
        journal: &Journal,
        cosign: Option<&CosignDirectory>,
        policies: &BTreeMap<PolicyDestination, AuthenticatedPolicy>,
    ) -> Self {
        Self {
            pending: AttestationView::from_journal(journal, cosign, policies),
            history: journal.history(),
            cosign: cosign.cloned(),
            policies: policies.clone(),
        }
    }
}

pub trait AttestationSource {
    fn resolve(&self, key: &str) -> Result<AttestationView, JournalError>;
}

impl<S: AttestationSource + ?Sized> AttestationSource for &S {
    fn resolve(&self, key: &str) -> Result<AttestationView, JournalError> {
        S::resolve(*self, key)
    }
}

impl AttestationSource for AttestationView {
    fn resolve(&self, key: &str) -> Result<AttestationView, JournalError> {
        let mut view = AttestationView::default();
        if let Some(item) = self.items.get(key) {
            view.items.insert(key.to_owned(), item.clone());
            if let Some(digest) = item_digest(item) {
                if let Some(signatures) = self.signatures.get(&digest) {
                    view.signatures.insert(digest, signatures.clone());
                }
            }
        }
        Ok(view)
    }
}

fn item_digest(item: &Item) -> Option<[u8; 32]> {
    match item.observation {
        Observation::Inbound { .. } => item.observation.inbound_attestation().map(|a| a.digest()),
        Observation::Outbound { .. } => item.observation.outbound_attestation().map(|a| a.digest()),
    }
}

impl AttestationSource for AttestationSnapshot {
    fn resolve(&self, key: &str) -> Result<AttestationView, JournalError> {
        if self.pending.items.contains_key(key) {
            return self.pending.resolve(key);
        }
        let mut view = AttestationView::default();
        if let Some(mut item) = self.history.lookup(key)? {
            let mut policies = self.policies.iter().filter(|(destination, policy)| {
                **destination == policy.destination && destination.accepts(&item.observation)
            });
            item.attestation_policy = policies.next().map(|(_, policy)| policy.clone());
            if policies.next().is_some() {
                item.attestation_policy = None;
            }
            if let (Some(digest), Some(cosign)) = (item_digest(&item), &self.cosign) {
                view.signatures.insert(digest, cosign.collect(&digest));
            }
            view.items.insert(key.to_owned(), item);
        }
        Ok(view)
    }
}

const MAX_REQUEST_BYTES: usize = 8192;
const REQUEST_TIMEOUT: Duration = Duration::from_secs(2);

pub fn serve<S: AttestationSource>(
    listener: TcpListener,
    view: Arc<RwLock<S>>,
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

fn serve_connection<S: AttestationSource>(
    stream: &mut TcpStream,
    view: &RwLock<S>,
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
        let key = if *route == "deposit" {
            inbound_key(chain, &hash, index)
        } else {
            outbound_key(chain, &hash, index)
        };
        let resolved = match view.resolve(&key) {
            Ok(resolved) => resolved,
            Err(_) => {
                return respond(
                    stream,
                    500,
                    json!({"error": "attestation history unavailable"}),
                )
            }
        };
        if *route == "deposit" {
            deposit_from_view(&resolved, chain, &hash, index)
        } else {
            release_from_view(&resolved, chain, &hash, index)
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
            "HTTP/1.1 {status} {reason}\r\nConnection: close\r\nCache-Control: no-store\r\n{cors}\r\n"
        );
    }
    write!(
        stream,
        "HTTP/1.1 {status} {reason}\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\nCache-Control: no-store\r\n{}{cors}\r\n{body}",
        body.len(),
        if status == 405 { "Allow: GET, OPTIONS\r\n" } else { "" },
    )
}

#[must_use]
pub fn deposit_answer<S: AttestationSource + ?Sized>(
    view: &S,
    chain_id: u64,
    tx_hash: &[u8; 32],
    log_index: u64,
) -> Option<Value> {
    let resolved = view
        .resolve(&inbound_key(chain_id, tx_hash, log_index))
        .ok()?;
    deposit_from_view(&resolved, chain_id, tx_hash, log_index)
}

#[must_use]
pub fn release_answer<S: AttestationSource + ?Sized>(
    view: &S,
    chain_id: u64,
    tx_hash: &[u8; 32],
    nonce: u64,
) -> Option<Value> {
    let resolved = view.resolve(&outbound_key(chain_id, tx_hash, nonce)).ok()?;
    release_from_view(&resolved, chain_id, tx_hash, nonce)
}

#[must_use]
fn deposit_from_view(
    view: &AttestationView,
    chain_id: u64,
    tx_hash: &[u8; 32],
    log_index: u64,
) -> Option<Value> {
    let item = view.items.get(&inbound_key(chain_id, tx_hash, log_index))?;
    let attestation = item.observation.inbound_attestation()?;
    let digest = attestation.digest();
    let signatures = signatures(view, item, &digest);
    let status = status(item, signatures.len());
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
fn release_from_view(
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
    let status = status(item, signatures.len());
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
    let Some(policy) = &item.attestation_policy else {
        return Vec::new();
    };
    eligible_signatures(
        digest,
        item.signature
            .iter()
            .copied()
            .chain(view.signatures.get(digest).into_iter().flatten().copied()),
        &policy.attestors,
    )
    .iter()
    .map(|signature| hex::prefixed(signature))
    .collect()
}

fn status(item: &Item, signatures: usize) -> &'static str {
    if item.signature.is_none() {
        "observed"
    } else if item
        .attestation_policy
        .as_ref()
        .is_some_and(|policy| policy.threshold > 0 && signatures >= policy.threshold)
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
