use std::collections::BTreeMap;
use std::io::{self, Write as _};
use std::net::{SocketAddr, ToSocketAddrs as _};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex, PoisonError};
use std::time::{Duration, Instant};

use k256::ecdsa::SigningKey;
use serde_json::{json, Value};

use crate::attest::{
    network_word, recover_signer, sign_digest, signer_address, stored_response, Answer,
    AttestError, Attestation, AttestorSet, Discard, Level, Ready, MAX_RECORD_BYTES,
    MAX_RESPONSE_BYTES, ORIGIN_PROGRAM, SIGNATURE_LENGTH,
};
use crate::content::{ContentStore, PEER_HEADER};
use crate::fetch::{Fetcher, HttpClient, Url};
use crate::index::WebIndex;
use crate::kernel::KernelError;
use crate::payment::{hex, unhex};
use crate::search;
use crate::server::{Response, RouteError, RouteTable};
use crate::watch::{hex0x, keccak, unhex0x};

pub use crate::server::PROGRAM_ATTESTATION_PATH;

/// The observation header ahead of the stored response.
pub const OBSERVATION_HEADER_BYTES: usize = 146;

/// The most attestor signatures one observation carries.
pub const MAX_OBSERVATION_SIGNATURES: usize = 16;

const PEER_CONNECT_TIMEOUT: Duration = Duration::from_secs(3);
const PEER_TOTAL_TIMEOUT: Duration = Duration::from_secs(10);
const DISCARD_LOG: &str = "program-discarded.jsonl";

/// One committed program web request.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramRequest {
    pub program_id: [u8; 32],
    pub request_id: u64,
    pub kind: u8,
    pub payload: Vec<u8>,
    pub sequence: u64,
}

impl ProgramRequest {
    /// The origin-2 attestation of this request's answer.
    #[must_use]
    pub fn attestation(
        &self,
        network_id: u32,
        content_digest: [u8; 32],
        response: &[u8],
        full_length: u32,
    ) -> Attestation {
        Attestation {
            origin: ORIGIN_PROGRAM,
            network_id: network_word(u64::from(network_id)),
            requester: self.program_id,
            request_id: self.request_id,
            kind: self.kind,
            payload_hash: keccak(&self.payload),
            content_digest,
            response_hash: keccak(response),
            full_length,
        }
    }
}

pub(crate) fn fixed<const N: usize>(value: Option<&Value>) -> Option<[u8; N]> {
    unhex(value?.as_str()?)?.try_into().ok()
}

/// Fetches or searches a program request's payload independently and signs
/// the origin-2 digest with the web-attestor key.
pub struct ProgramAttestor {
    key: SigningKey,
    signer: [u8; 20],
    pub(crate) network_id: u32,
    fetcher: Arc<Fetcher>,
    index: Arc<WebIndex>,
    store: Arc<ContentStore>,
}

impl ProgramAttestor {
    #[must_use]
    pub fn new(
        key: SigningKey,
        network_id: u32,
        fetcher: Arc<Fetcher>,
        index: Arc<WebIndex>,
        store: Arc<ContentStore>,
    ) -> Self {
        Self {
            signer: signer_address(&key),
            key,
            network_id,
            fetcher,
            index,
            store,
        }
    }

    /// The address this attestor signs as.
    #[must_use]
    pub const fn signer(&self) -> [u8; 20] {
        self.signer
    }

    fn content(&self, request: &ProgramRequest) -> Result<([u8; 32], String), AttestError> {
        let payload = std::str::from_utf8(&request.payload).map_err(|_| AttestError::Payload)?;
        let (canonical, text) = match request.kind {
            1 => {
                let page = self.fetcher.fetch(payload).map_err(AttestError::Fetch)?;
                (page.canonical, page.text)
            }
            2 => {
                let results: Vec<search::SearchResult> = search::search(&self.index, payload)
                    .map_err(|_| AttestError::Search)?
                    .into_iter()
                    .map(|scored| scored.result)
                    .collect();
                let canonical = search::search_canonical_bytes(payload, &results)
                    .map_err(|_| AttestError::Search)?;
                let text = search::search_text(&results).map_err(|_| AttestError::Search)?;
                (canonical, text)
            }
            kind => return Err(AttestError::UnknownKind(kind)),
        };
        let digest = self.store.put(&canonical).map_err(|_| AttestError::Store)?;
        Ok((digest, text))
    }

    /// Answers one program request: the content, the stored response and the
    /// signature over the origin-2 digest. A program request carries no
    /// callback gas and no timeout height.
    ///
    /// # Errors
    /// Returns why the request could not be answered.
    pub fn attest(&self, request: &ProgramRequest) -> Result<Answer, AttestError> {
        let (content_digest, text) = self.content(request)?;
        let (response, full_length) = stored_response(&text)?;
        let attestation =
            request.attestation(self.network_id, content_digest, &response, full_length);
        let digest = attestation.digest();
        let signature = sign_digest(&self.key, &digest).map_err(AttestError::Sign)?;
        Ok(Answer {
            attestation,
            level: Level::Majority,
            response,
            callback_gas: 0,
            timeout_height: u64::MAX,
            digest,
            signer: self.signer,
            signature,
        })
    }
}

/// The observation payload the kernel's web intake admits: the 146-byte
/// header, the stored response, the signature count and the signatures in
/// ascending signer order.
///
/// # Errors
/// Refuses a response longer than a fulfilment stores, a full length shorter
/// than the response and a signature count outside one to
/// [`MAX_OBSERVATION_SIGNATURES`].
pub fn observation_bytes(
    network_id: u32,
    request: &ProgramRequest,
    ready: &Ready,
) -> Result<Vec<u8>, KernelError> {
    let response_length = u32::try_from(ready.response.len()).map_err(|_| KernelError::Encode)?;
    let count = ready.signatures.len();
    if network_id == 0
        || ready.request_id != request.request_id
        || ready.response.len() > MAX_RESPONSE_BYTES
        || ready.full_length < response_length
        || count == 0
        || count > MAX_OBSERVATION_SIGNATURES
    {
        return Err(KernelError::Encode);
    }
    let digest = request
        .attestation(
            network_id,
            ready.content_digest,
            &ready.response,
            ready.full_length,
        )
        .digest();
    if request.program_id == [0; 32]
        || !matches!(request.kind, 1 | 2)
        || ready.digest != digest
        || ready.signers.len() != count
        || ready.signers.windows(2).any(|pair| pair[0] >= pair[1])
        || ready
            .signers
            .iter()
            .zip(&ready.signatures)
            .any(|(signer, signature)| recover_signer(&digest, signature).ok() != Some(*signer))
    {
        return Err(KernelError::Encode);
    }
    let mut out = Vec::with_capacity(
        OBSERVATION_HEADER_BYTES + ready.response.len() + 1 + count * SIGNATURE_LENGTH,
    );
    out.push(ORIGIN_PROGRAM);
    out.extend_from_slice(&[0; 28]);
    out.extend_from_slice(&network_id.to_be_bytes());
    out.extend_from_slice(&request.program_id);
    out.extend_from_slice(&request.request_id.to_be_bytes());
    out.push(request.kind);
    out.extend_from_slice(&keccak(&request.payload));
    out.extend_from_slice(&ready.content_digest);
    out.extend_from_slice(&ready.full_length.to_be_bytes());
    out.extend_from_slice(&response_length.to_be_bytes());
    out.extend_from_slice(&ready.response);
    out.push(u8::try_from(count).map_err(|_| KernelError::Encode)?);
    for signature in &ready.signatures {
        out.extend_from_slice(signature);
    }
    Ok(out)
}

pub(crate) fn observation_signers(
    bytes: &[u8],
    digest: &[u8; 32],
) -> Result<(Vec<[u8; 20]>, Vec<[u8; SIGNATURE_LENGTH]>), KernelError> {
    if bytes.len() <= OBSERVATION_HEADER_BYTES {
        return Err(KernelError::Journal);
    }
    let length = u32::from_be_bytes(
        bytes[142..146]
            .try_into()
            .map_err(|_| KernelError::Journal)?,
    );
    let offset = OBSERVATION_HEADER_BYTES
        .checked_add(length as usize)
        .ok_or(KernelError::Journal)?;
    let count = usize::from(*bytes.get(offset).ok_or(KernelError::Journal)?);
    if count == 0
        || count > MAX_OBSERVATION_SIGNATURES
        || bytes.len() != offset + 1 + count * SIGNATURE_LENGTH
    {
        return Err(KernelError::Journal);
    }
    let mut signers = Vec::new();
    let mut signatures = Vec::new();
    for bytes in bytes[offset + 1..].chunks_exact(SIGNATURE_LENGTH) {
        let signature = bytes.try_into().map_err(|_| KernelError::Journal)?;
        signers.push(recover_signer(digest, &signature).map_err(|_| KernelError::Journal)?);
        signatures.push(signature);
    }
    Ok((signers, signatures))
}

/// The key every program answer is held under: the program id and the
/// request id together, so two programs' request ids never share a slot.
pub type ProgramKey = ([u8; 32], u64);

/// One peer signature for a program request that was discarded, as recorded.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ProgramDiscarded {
    pub peer: String,
    pub program_id: [u8; 32],
    pub request_id: u64,
    pub reason: Discard,
    pub claimed_digest: Option<[u8; 32]>,
    pub claimed_signer: Option<[u8; 20]>,
}

impl ProgramDiscarded {
    fn line(&self) -> String {
        json!({
            "peer": self.peer,
            "program_id": hex0x(&self.program_id),
            "request_id": self.request_id,
            "reason": self.reason.code(),
            "claimed_digest": self.claimed_digest.map(|digest| hex0x(&digest)),
            "claimed_signer": self.claimed_signer.map(|signer| hex0x(&signer)),
        })
        .to_string()
    }
}

/// The record the program signature-exchange route serves for an answer:
/// the fields of [`Answer::record`] with the program id beside the request
/// id.
#[must_use]
pub fn program_record(answer: &Answer) -> Value {
    let mut record = answer.record();
    if let Some(object) = record.as_object_mut() {
        object.insert(
            "program_id".to_owned(),
            Value::String(hex0x(&answer.attestation.requester)),
        );
    }
    record
}

struct ProgramRecord {
    program_id: [u8; 32],
    request_id: u64,
    digest: [u8; 32],
    signer: [u8; 20],
    signature: [u8; SIGNATURE_LENGTH],
}

pub(crate) fn fixed0x<const N: usize>(value: Option<&Value>) -> Option<[u8; N]> {
    unhex0x(value?.as_str()?)?.try_into().ok()
}

fn parse_program_record(record: &Value) -> Option<ProgramRecord> {
    let object = record.as_object()?;
    let known = [
        "program_id",
        "request_id",
        "digest",
        "content_digest",
        "response_hash",
        "full_length",
        "signer",
        "signature",
    ];
    if object.keys().any(|key| !known.contains(&key.as_str())) {
        return None;
    }
    fixed0x::<32>(object.get("content_digest"))?;
    fixed0x::<32>(object.get("response_hash"))?;
    u32::try_from(object.get("full_length")?.as_u64()?).ok()?;
    Some(ProgramRecord {
        program_id: fixed0x(object.get("program_id"))?,
        request_id: object.get("request_id")?.as_u64()?,
        digest: fixed0x(object.get("digest"))?,
        signer: fixed0x(object.get("signer"))?,
        signature: fixed0x(object.get("signature"))?,
    })
}

pub(crate) struct Held {
    pub(crate) answer: Answer,
    pub(crate) signatures: BTreeMap<[u8; 20], [u8; SIGNATURE_LENGTH]>,
}

pub(crate) fn held_value(held: &Held) -> Value {
    json!({
        "response": hex(&held.answer.response),
        "content_digest": hex(&held.answer.attestation.content_digest),
        "full_length": held.answer.attestation.full_length,
        "signer": hex(&held.answer.signer),
        "signature": hex(&held.answer.signature),
        "signatures": held.signatures.iter().map(|(signer, signature)|
            json!({"signer": hex(signer), "signature": hex(signature)})).collect::<Vec<_>>(),
    })
}

/// Exchanges attestor signatures for program requests with the configured
/// peer sidecars, holding every answer under its [`ProgramKey`].
///
/// Each sidecar serves its own signed record for a program request at
/// `GET /program-attestations/<program id>/<request id>`. A record is taken
/// only when it names the same program and request, its signature recovers
/// over this sidecar's own digest to the signer it claims, and that signer is
/// a registered attestor. Every refusal is recorded, never accepted.
pub struct ProgramExchange {
    peers: Vec<(String, Url)>,
    client: HttpClient,
    answers: Mutex<BTreeMap<ProgramKey, Held>>,
    discarded: Mutex<Vec<ProgramDiscarded>>,
    log_path: PathBuf,
}

impl ProgramExchange {
    /// Opens the exchange with its discard log under `state_dir`.
    ///
    /// # Errors
    /// Refuses a peer that is not an http or https URL with no query and
    /// returns the error creating the directory.
    pub fn open(state_dir: &Path, peers: &[String]) -> io::Result<Self> {
        let peers = peers
            .iter()
            .map(|peer| {
                Url::parse(peer)
                    .ok()
                    .filter(|url| !url.target.contains('?'))
                    .map(|url| (peer.clone(), url))
                    .ok_or_else(|| io::Error::new(io::ErrorKind::InvalidInput, "invalid peer url"))
            })
            .collect::<io::Result<Vec<_>>>()?;
        std::fs::create_dir_all(state_dir)?;
        let client = HttpClient::new(PEER_CONNECT_TIMEOUT)
            .map_err(|error| io::Error::other(error.code()))?;
        Ok(Self {
            peers,
            client,
            answers: Mutex::new(BTreeMap::new()),
            discarded: Mutex::new(Vec::new()),
            log_path: state_dir.join(DISCARD_LOG),
        })
    }

    pub(crate) fn snapshot(&self, key: ProgramKey) -> Option<Value> {
        self.answers().get(&key).map(held_value)
    }

    pub(crate) fn restore(
        &self,
        request: &ProgramRequest,
        network_id: u32,
        value: &Value,
    ) -> Result<(), KernelError> {
        let response = value
            .get("response")
            .and_then(Value::as_str)
            .and_then(unhex)
            .ok_or(KernelError::Journal)?;
        let full_length = value
            .get("full_length")
            .and_then(Value::as_u64)
            .and_then(|n| u32::try_from(n).ok())
            .ok_or(KernelError::Journal)?;
        if response.len() > MAX_RESPONSE_BYTES || u64::from(full_length) < response.len() as u64 {
            return Err(KernelError::Journal);
        }
        let attestation = request.attestation(
            network_id,
            fixed(value.get("content_digest")).ok_or(KernelError::Journal)?,
            &response,
            full_length,
        );
        let digest = attestation.digest();
        let signer = fixed(value.get("signer")).ok_or(KernelError::Journal)?;
        let signature = fixed(value.get("signature")).ok_or(KernelError::Journal)?;
        if recover_signer(&digest, &signature).ok() != Some(signer) {
            return Err(KernelError::Journal);
        }
        let mut signatures = BTreeMap::new();
        for row in value
            .get("signatures")
            .and_then(Value::as_array)
            .ok_or(KernelError::Journal)?
        {
            let signer = fixed(row.get("signer")).ok_or(KernelError::Journal)?;
            let signature = fixed(row.get("signature")).ok_or(KernelError::Journal)?;
            if recover_signer(&digest, &signature).ok() != Some(signer)
                || signatures.insert(signer, signature).is_some()
            {
                return Err(KernelError::Journal);
            }
        }
        if signatures.get(&signer) != Some(&signature)
            || signatures.len() - 1 > MAX_OBSERVATION_SIGNATURES
        {
            return Err(KernelError::Journal);
        }
        self.answers().insert(
            (request.program_id, request.request_id),
            Held {
                answer: Answer {
                    attestation,
                    level: Level::Majority,
                    response,
                    callback_gas: 0,
                    timeout_height: u64::MAX,
                    digest,
                    signer,
                    signature,
                },
                signatures,
            },
        );
        Ok(())
    }

    /// The file every discarded signature is appended to.
    #[must_use]
    pub fn log_path(&self) -> &Path {
        &self.log_path
    }

    pub(crate) fn answers(&self) -> std::sync::MutexGuard<'_, BTreeMap<ProgramKey, Held>> {
        self.answers.lock().unwrap_or_else(PoisonError::into_inner)
    }

    /// Keeps this sidecar's own answer and its own signature under the
    /// answer's program and request. An answer already held there is kept
    /// as it is.
    pub fn record(&self, answer: Answer) {
        self.answers()
            .entry((answer.attestation.requester, answer.request_id()))
            .or_insert_with(|| Held {
                signatures: BTreeMap::from([(answer.signer, answer.signature)]),
                answer,
            });
    }

    /// This sidecar's answer to a program's request.
    #[must_use]
    pub fn answer(&self, key: ProgramKey) -> Option<Answer> {
        self.answers().get(&key).map(|held| held.answer.clone())
    }

    /// The program requests this sidecar holds an answer for, ascending.
    #[must_use]
    pub fn pending(&self) -> Vec<ProgramKey> {
        self.answers().keys().copied().collect()
    }

    /// Drops the answer and signatures for a program's request.
    pub fn forget(&self, key: ProgramKey) {
        self.answers().remove(&key);
    }

    /// Every signature discarded so far, in the order it was discarded.
    #[must_use]
    pub fn discarded(&self) -> Vec<ProgramDiscarded> {
        self.discarded
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .clone()
    }

    fn discard(&self, entry: ProgramDiscarded) {
        let line = entry.line();
        eprintln!("x-websearch discarded a peer program signature: {line}");
        let appended = std::fs::OpenOptions::new()
            .create(true)
            .append(true)
            .open(&self.log_path)
            .and_then(|mut file| {
                file.write_all(line.as_bytes())?;
                file.write_all(b"\n")?;
                file.sync_all()
            });
        if let Err(error) = appended {
            eprintln!("x-websearch could not append to the discard log: {error}");
        }
        self.discarded
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .push(entry);
    }

    /// The `GET /program-attestations/<program id>/<request id>` resource:
    /// this sidecar's own signed record, or 404.
    #[must_use]
    pub fn handle(&self, key: ProgramKey) -> Response {
        match self.answer(key) {
            Some(answer) => Response::json(200, program_record(&answer).to_string().into_bytes()),
            None => Response::error(404, "attestation_not_found"),
        }
    }

    /// Checks one peer record for a program request against this sidecar's
    /// own answer and the registered set, and keeps its signature. Every
    /// refusal is recorded.
    ///
    /// # Errors
    /// Returns why the signature was discarded. A request this sidecar holds
    /// no answer for is not an error and takes nothing.
    pub fn accept(
        &self,
        peer: &str,
        key: ProgramKey,
        record: &Value,
        set: &AttestorSet,
    ) -> Result<Option<[u8; 20]>, Discard> {
        let Some(local) = self.answer(key) else {
            return Ok(None);
        };
        let parsed = parse_program_record(record);
        let verdict = match &parsed {
            None => Err(Discard::Malformed),
            Some(record) if (record.program_id, record.request_id) != key => {
                Err(Discard::WrongRequest)
            }
            Some(record) if record.digest != local.digest => Err(Discard::DifferentDigest),
            Some(record) => match recover_signer(&local.digest, &record.signature) {
                Ok(signer) if signer != record.signer => Err(Discard::BadSignature),
                Err(_) => Err(Discard::BadSignature),
                Ok(signer) if !set.contains(&signer) => Err(Discard::UnknownSigner),
                Ok(signer) => Ok(signer),
            },
        };
        match verdict {
            Ok(signer) => {
                if let (Some(held), Some(record)) = (self.answers().get_mut(&key), parsed) {
                    let own = held.answer.signer;
                    held.signatures
                        .retain(|kept, _| *kept == own || set.contains(kept));
                    held.signatures.insert(signer, record.signature);
                }
                Ok(Some(signer))
            }
            Err(reason) => {
                self.discard(ProgramDiscarded {
                    peer: peer.to_owned(),
                    program_id: key.0,
                    request_id: key.1,
                    reason,
                    claimed_digest: parsed.as_ref().map(|record| record.digest),
                    claimed_signer: parsed.as_ref().map(|record| record.signer),
                });
                Err(reason)
            }
        }
    }

    fn ask(&self, peer: &Url, key: ProgramKey) -> Option<Value> {
        let url = Url {
            target: format!(
                "{}{PROGRAM_ATTESTATION_PATH}{}/{}",
                peer.target.trim_end_matches('/'),
                hex(&key.0),
                key.1
            ),
            ..peer.clone()
        };
        let address: SocketAddr = (url.bare_host(), url.port).to_socket_addrs().ok()?.next()?;
        let response = self
            .client
            .get(
                &url,
                address,
                Instant::now() + PEER_TOTAL_TIMEOUT,
                MAX_RECORD_BYTES,
                &[("Accept", "application/json"), (PEER_HEADER, "1")],
            )
            .ok()?;
        (response.status == 200)
            .then(|| serde_json::from_slice(&response.body).ok())
            .flatten()
    }

    /// Asks every peer for its record of a program request and keeps each
    /// signature that checks out. An unreachable peer or one with no record
    /// is skipped. Returns the number of signatures held for the request.
    #[must_use]
    pub fn collect(&self, key: ProgramKey, set: &AttestorSet) -> usize {
        if self.answer(key).is_none() {
            return 0;
        }
        for (name, peer) in &self.peers {
            let Some(record) = self.ask(peer, key) else {
                continue;
            };
            let _ = self.accept(name, key, &record, set);
        }
        self.answers()
            .get(&key)
            .map_or(0, |held| held.signatures.len())
    }

    /// The signatures for the observation once at least the threshold of
    /// registered signers agree with this sidecar's answer, ascending by
    /// signer.
    #[must_use]
    pub fn ready(&self, key: ProgramKey, set: &AttestorSet) -> Option<Ready> {
        let answers = self.answers();
        let held = answers.get(&key)?;
        Ready::from_signatures(&held.answer, set, &held.signatures)
    }
}

/// Registers the program signature-exchange route
/// `GET /program-attestations/<program id>/<request id>`.
///
/// # Errors
/// Refuses a table that already has a program exchange handler.
pub fn register(
    routes: &mut RouteTable,
    exchange: &Arc<ProgramExchange>,
) -> Result<(), RouteError> {
    let exchange = Arc::clone(exchange);
    routes.set_program_attestations(move |program_id, request_id| {
        exchange.handle((program_id, request_id))
    })
}
