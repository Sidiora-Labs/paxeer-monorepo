use std::collections::{BTreeMap, BTreeSet};
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
use crate::payment::{hex, unhex};
use crate::search;
use crate::server::{Response, RouteError, RouteTable};
use crate::submit::{Outcome, SubmitError, Submitter};
use crate::watch::{hex0x, keccak, unhex0x, EvmError, RequestWatcher};

pub use crate::server::PROGRAM_ATTESTATION_PATH;

/// The observation header ahead of the stored response.
pub const OBSERVATION_HEADER_BYTES: usize = 146;

/// The most attestor signatures one observation carries.
pub const MAX_OBSERVATION_SIGNATURES: usize = 16;

const PEER_CONNECT_TIMEOUT: Duration = Duration::from_secs(3);
const PEER_TOTAL_TIMEOUT: Duration = Duration::from_secs(10);
const DISCARD_LOG: &str = "program-discarded.jsonl";

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum KernelError {
    Authorization,
    /// The gateway endpoint is not an http or https URL with a host.
    Endpoint,
    /// No well-formed answer arrived.
    Unavailable,
    /// The gateway answered with a JSON-RPC error.
    Rejected {
        code: i64,
    },
    /// The answer does not have the shape the method defines.
    Malformed,
    /// A request record is not a canonical program web request.
    Record,
    /// The watcher's cursor could not be read or written.
    Cursor,
    /// The observation or its activity could not be encoded.
    Encode,
    /// The relay journal could not be read, is not this version or could
    /// not be written.
    Journal,
}

impl std::fmt::Display for KernelError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Authorization => f.write_str("kernel gateway authorization refused"),
            Self::Endpoint => f.write_str("gateway endpoint refused"),
            Self::Unavailable => f.write_str("gateway unavailable"),
            Self::Rejected { code } => write!(f, "gateway rejected the call with {code}"),
            Self::Malformed => f.write_str("gateway answer malformed"),
            Self::Record => f.write_str("program web request record malformed"),
            Self::Cursor => f.write_str("kernel watch cursor unreadable or unwritable"),
            Self::Encode => f.write_str("observation activity could not be encoded"),
            Self::Journal => f.write_str("kernel relay journal unreadable or unwritable"),
        }
    }
}

impl std::error::Error for KernelError {}

/// Decodes the record a program emits: the request id big-endian, the kind,
/// the payload length big-endian and the payload.
///
/// # Errors
/// Refuses an unknown kind, an empty payload and a length that does not
/// match the record.
pub fn decode_request_record(
    program_id: [u8; 32],
    sequence: u64,
    data: &[u8],
) -> Result<ProgramRequest, KernelError> {
    const HEADER_BYTES: usize = 13;
    if data.len() < HEADER_BYTES {
        return Err(KernelError::Record);
    }
    let (header, payload) = data.split_at(HEADER_BYTES);
    let mut id = [0; 8];
    id.copy_from_slice(&header[..8]);
    let mut length = [0; 4];
    length.copy_from_slice(&header[9..13]);
    let length = usize::try_from(u32::from_be_bytes(length)).map_err(|_| KernelError::Record)?;
    let kind = header[8];
    if !matches!(kind, 1 | 2) || length == 0 || payload.len() != length {
        return Err(KernelError::Record);
    }
    Ok(ProgramRequest {
        program_id,
        request_id: u64::from_be_bytes(id),
        kind,
        payload: payload.to_vec(),
        sequence,
    })
}

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

pub type ObservationSigners = (Vec<[u8; 20]>, Vec<[u8; SIGNATURE_LENGTH]>);

///
/// # Errors
/// Returns an error when the requested operation fails validation or storage.
pub fn observation_signers(
    bytes: &[u8],
    digest: &[u8; 32],
) -> Result<ObservationSigners, KernelError> {
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
    content_digest: [u8; 32],
    response_hash: [u8; 32],
    full_length: u32,
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
    Some(ProgramRecord {
        program_id: fixed0x(object.get("program_id"))?,
        request_id: object.get("request_id")?.as_u64()?,
        digest: fixed0x(object.get("digest"))?,
        content_digest: fixed0x(object.get("content_digest"))?,
        response_hash: fixed0x(object.get("response_hash"))?,
        full_length: u32::try_from(object.get("full_length")?.as_u64()?).ok()?,
        signer: fixed0x(object.get("signer"))?,
        signature: fixed0x(object.get("signature"))?,
    })
}

#[derive(Clone)]
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
    retired: Mutex<BTreeSet<ProgramKey>>,
    _lease: std::fs::File,
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
        let lease = std::fs::OpenOptions::new()
            .create(true)
            .truncate(false)
            .read(true)
            .write(true)
            .open(state_dir.join("answers.lock"))?;
        lease
            .try_lock()
            .map_err(|_| io::Error::other("program exchange already open"))?;
        let mut answers = BTreeMap::new();
        let mut retired = BTreeSet::new();
        let state = match std::fs::read(state_dir.join("answers.json")) {
            Ok(bytes) => Some(serde_json::from_slice::<Value>(&bytes).map_err(io::Error::other)?),
            Err(e) if e.kind() == io::ErrorKind::NotFound => None,
            Err(e) => return Err(e),
        };
        if let Some(state) = state {
            if state["version"] != 1 {
                return Err(io::ErrorKind::InvalidData.into());
            }
            for row in state["retired"]
                .as_array()
                .ok_or(io::ErrorKind::InvalidData)?
            {
                let key: ProgramKey =
                    serde_json::from_value(row.clone()).map_err(io::Error::other)?;
                if !retired.insert(key) {
                    return Err(io::ErrorKind::InvalidData.into());
                }
            }
            for row in state["answers"]
                .as_array()
                .ok_or(io::ErrorKind::InvalidData)?
            {
                let held = decode_held(row).ok_or(io::ErrorKind::InvalidData)?;
                let key = (held.answer.attestation.requester, held.answer.request_id());
                if row["retired"].as_bool().ok_or(io::ErrorKind::InvalidData)? {
                    retired.insert(key);
                }
                if answers.insert(key, held).is_some() {
                    return Err(io::ErrorKind::InvalidData.into());
                }
            }
        }
        Ok(Self {
            retired: Mutex::new(retired),
            _lease: lease,
            peers,
            client,
            answers: Mutex::new(answers),
            discarded: Mutex::new(Vec::new()),
            log_path: state_dir.join(DISCARD_LOG),
        })
    }

    fn persist(
        &self,
        answers: &BTreeMap<ProgramKey, Held>,
        retired: &BTreeSet<ProgramKey>,
    ) -> io::Result<()> {
        let rows: Vec<Value> = answers
            .iter()
            .map(|(key, held)| {
                json!({
                    "attestation": held.answer.attestation, "held": held_value(held),
                    "retired": retired.contains(key),
                })
            })
            .collect();
        let path = self.log_path.with_file_name("answers.json");
        let temporary = path.with_extension("tmp");
        std::fs::write(
            &temporary,
            json!({"version": 1, "answers": rows, "retired": retired}).to_string(),
        )?;
        std::fs::File::open(&temporary)?.sync_all()?;
        std::fs::rename(&temporary, &path)?;
        std::fs::File::open(path.parent().ok_or(io::ErrorKind::InvalidInput)?)?.sync_all()
    }

    /// Pins persisted program answers to the chain and local signing identity.
    /// # Errors
    /// Returns an error when program state or chain interaction cannot be validated.
    pub fn bind_chain(&self, chain: u64, network: u32, signer: [u8; 20]) -> io::Result<()> {
        let path = self.log_path.with_file_name("identity.json");
        let identity = json!({"chain": chain, "network": network, "signer": hex(&signer)});
        match std::fs::read(&path) {
            Ok(bytes) => {
                if serde_json::from_slice::<Value>(&bytes).map_err(io::Error::other)? != identity {
                    return Err(io::ErrorKind::InvalidData.into());
                }
            }
            Err(e) if e.kind() == io::ErrorKind::NotFound => {
                let tmp = path.with_extension("tmp");
                std::fs::write(&tmp, identity.to_string())?;
                std::fs::File::open(&tmp)?.sync_all()?;
                std::fs::rename(tmp, &path)?;
                std::fs::File::open(path.parent().ok_or(io::ErrorKind::InvalidInput)?)?
                    .sync_all()?;
            }
            Err(e) => return Err(e),
        }
        if self.answers().values().any(|held| {
            held.answer.signer != signer
                || held.answer.attestation.network_id
                    != crate::attest::network_word(u64::from(network))
        }) {
            return Err(io::ErrorKind::InvalidData.into());
        }
        Ok(())
    }

    fn check_request(
        &self,
        request: &ProgramRequest,
        network: u32,
        signer: [u8; 20],
    ) -> Result<(), AttestError> {
        let key = (request.program_id, request.request_id);
        let answers = self.answers();
        if self
            .retired
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .contains(&key)
        {
            return Err(AttestError::Store);
        }
        if let Some(held) = answers.get(&key) {
            if held.answer.signer != signer
                || held.answer.attestation
                    != request.attestation(
                        network,
                        held.answer.attestation.content_digest,
                        &held.answer.response,
                        held.answer.attestation.full_length,
                    )
            {
                return Err(AttestError::Store);
            }
        }
        Ok(())
    }

    pub fn snapshot(&self, key: ProgramKey) -> Option<Value> {
        self.answers().get(&key).map(held_value)
    }

    /// # Errors
    /// Returns an error when the saved answer does not match its request or signatures.
    pub fn restore(
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
            || signatures.len() > MAX_OBSERVATION_SIGNATURES
        {
            return Err(KernelError::Journal);
        }
        let answer = Held {
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
        };
        self.record(answer.answer.clone())
            .map_err(|_| KernelError::Journal)?;
        let mut answers = self.answers();
        answers.insert((request.program_id, request.request_id), answer);
        self.persist(
            &answers,
            &self.retired.lock().unwrap_or_else(PoisonError::into_inner),
        )
        .map_err(|_| KernelError::Journal)?;
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
    /// # Errors
    /// Returns an error when program state or chain interaction cannot be validated.
    pub fn record(&self, answer: Answer) -> Result<(), KernelError> {
        let key = (answer.attestation.requester, answer.request_id());
        if answer.attestation.origin != ORIGIN_PROGRAM
            || answer.attestation.response_hash != keccak(&answer.response)
            || answer.digest != answer.attestation.digest()
            || recover_signer(&answer.digest, &answer.signature).ok() != Some(answer.signer)
        {
            return Err(KernelError::Journal);
        }
        let mut answers = self.answers();
        let retired = self.retired.lock().unwrap_or_else(PoisonError::into_inner);
        if retired.contains(&key) {
            return Err(KernelError::Journal);
        }
        if let Some(held) = answers.get(&key) {
            return if held.answer.digest == answer.digest && held.answer.signer == answer.signer {
                Ok(())
            } else {
                Err(KernelError::Journal)
            };
        }
        let mut next = answers.clone();
        next.insert(
            key,
            Held {
                signatures: BTreeMap::from([(answer.signer, answer.signature)]),
                answer,
            },
        );
        self.persist(&next, &retired)
            .map_err(|_| KernelError::Journal)?;
        *answers = next;
        Ok(())
    }

    /// This sidecar's answer to a program's request.
    #[must_use]
    pub fn answer(&self, key: ProgramKey) -> Option<Answer> {
        let answers = self.answers();
        if self
            .retired
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .contains(&key)
        {
            return None;
        }
        answers.get(&key).map(|held| held.answer.clone())
    }

    /// The program requests this sidecar holds an answer for, ascending.
    #[must_use]
    pub fn pending(&self) -> Vec<ProgramKey> {
        let answers = self.answers();
        let retired = self.retired.lock().unwrap_or_else(PoisonError::into_inner);
        answers
            .keys()
            .filter(|key| !retired.contains(key))
            .copied()
            .collect()
    }

    /// Drops the answer and signatures for a program's request.
    /// # Errors
    /// Returns an error when program state or chain interaction cannot be validated.
    pub fn forget(&self, key: ProgramKey) -> Result<(), KernelError> {
        let answers = self.answers();
        let mut retired = self.retired.lock().unwrap_or_else(PoisonError::into_inner);
        let mut next = retired.clone();
        next.insert(key);
        self.persist(&answers, &next)
            .map_err(|_| KernelError::Journal)?;
        *retired = next;
        Ok(())
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
            Some(record)
                if record.digest != local.digest
                    || record.content_digest != local.attestation.content_digest
                    || record.response_hash != local.attestation.response_hash
                    || record.full_length != local.attestation.full_length =>
            {
                Err(Discard::DifferentDigest)
            }
            Some(record) => match recover_signer(&local.digest, &record.signature) {
                Ok(signer) if signer != record.signer => Err(Discard::BadSignature),
                Err(_) => Err(Discard::BadSignature),
                Ok(signer) if !set.contains(&signer) => Err(Discard::UnknownSigner),
                Ok(signer) => Ok(signer),
            },
        };
        match verdict {
            Ok(signer) => {
                let mut answers = self.answers();
                let mut next = answers.clone();
                if let (Some(held), Some(record)) = (next.get_mut(&key), parsed) {
                    let own = held.answer.signer;
                    held.signatures
                        .retain(|kept, _| *kept == own || set.contains(kept));
                    held.signatures.insert(signer, record.signature);
                }
                self.persist(
                    &next,
                    &self.retired.lock().unwrap_or_else(PoisonError::into_inner),
                )
                .map_err(|_| Discard::Persistence)?;
                *answers = next;
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
        if self
            .retired
            .lock()
            .unwrap_or_else(PoisonError::into_inner)
            .contains(&key)
        {
            return None;
        }
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
        match exchange.answer((program_id, request_id)) {
            Some(answer) => Response::json(200, program_record(&answer).to_string().into_bytes()),
            None => Response::error(404, "not_found"),
        }
    })
}

#[derive(Clone, Debug, Eq, PartialEq, serde::Deserialize, serde::Serialize)]
pub struct ProgramRefusal {
    pub program_id: [u8; 32],
    pub request_id: u64,
    pub code: String,
}

#[derive(Clone, Debug, Eq, PartialEq)]
pub enum Step {
    Submitted {
        program_id: [u8; 32],
        request_id: u64,
        outcome: Outcome,
    },
    Discarded {
        program_id: [u8; 32],
        request_id: u64,
        code: String,
    },
}

pub struct ProgramRelay {
    pub watcher: RequestWatcher,
    pub attestor: ProgramAttestor,
    pub exchange: Arc<ProgramExchange>,
    pub set: AttestorSet,
    pub submitter: Submitter,
}

impl ProgramRelay {
    /// # Errors
    /// Returns an error when program state or chain interaction cannot be validated.
    pub fn discarded(&self) -> Result<Vec<ProgramRefusal>, EvmError> {
        let path = self
            .exchange
            .log_path()
            .with_file_name("program-requests-discarded.jsonl");
        let contents = match std::fs::read_to_string(path) {
            Ok(contents) => contents,
            Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(Vec::new()),
            Err(_) => return Err(EvmError::Journal),
        };
        contents
            .lines()
            .map(|line| serde_json::from_str(line).map_err(|_| EvmError::Journal))
            .collect()
    }

    /// Independently answers a watched request and exchanges signatures.
    /// `None` means the registered quorum has not yet agreed. This is also
    /// the preparation boundary used by [`Self::step`] before submission.
    ///
    /// # Errors
    /// Returns the attestor's refusal of the request.
    pub fn prepare_program(&self, request: &ProgramRequest) -> Result<Option<Ready>, AttestError> {
        let key = (request.program_id, request.request_id);
        self.exchange
            .check_request(request, self.attestor.network_id, self.attestor.signer())?;
        if self.exchange.answer(key).is_none() {
            self.exchange
                .record(self.attestor.attest(request)?)
                .map_err(|_| AttestError::Store)?;
        }
        let _ = self.exchange.collect(key, &self.set);
        Ok(self.exchange.ready(key, &self.set))
    }

    /// # Errors
    /// Returns an error when program state or chain interaction cannot be validated.
    pub fn step(&mut self) -> Result<Vec<Step>, EvmError> {
        let discarded = self.discarded()?;
        self.watcher.poll()?;
        let mut steps = Vec::new();
        for request in self.watcher.program_work() {
            let key = (request.program_id, request.request_id);
            if let Some(refusal) = discarded
                .iter()
                .find(|entry| (entry.program_id, entry.request_id) == key)
            {
                self.watcher.retire_program(key.0, key.1)?;
                self.exchange.forget(key).map_err(|_| EvmError::Journal)?;
                steps.push(Step::Discarded {
                    program_id: key.0,
                    request_id: key.1,
                    code: refusal.code.clone(),
                });
                continue;
            }
            let resumed = self.submitter.resume_program(key).map_err(submit_error)?;
            let outcome = if let Some(outcome) = resumed {
                outcome
            } else {
                let ready = match self.prepare_program(&request) {
                    Ok(Some(ready)) => ready,
                    Ok(None) => continue,
                    Err(AttestError::Store) => return Err(EvmError::Journal),
                    Err(reason) => {
                        let code = match &reason {
                            AttestError::Fetch(error) => error.code().to_owned(),
                            _ => reason.to_string(),
                        };
                        let record = serde_json::to_string(&ProgramRefusal {
                            program_id: key.0,
                            request_id: key.1,
                            code: code.clone(),
                        })
                        .map_err(|_| EvmError::Journal)?;
                        let mut file = std::fs::OpenOptions::new()
                            .create(true)
                            .append(true)
                            .open(
                                self.exchange
                                    .log_path()
                                    .with_file_name("program-requests-discarded.jsonl"),
                            )
                            .map_err(|_| EvmError::Journal)?;
                        writeln!(file, "{record}").map_err(|_| EvmError::Journal)?;
                        file.sync_all().map_err(|_| EvmError::Journal)?;
                        self.watcher.retire_program(key.0, key.1)?;
                        self.exchange.forget(key).map_err(|_| EvmError::Journal)?;
                        steps.push(Step::Discarded {
                            program_id: key.0,
                            request_id: key.1,
                            code,
                        });
                        continue;
                    }
                };
                let observation = observation_bytes(self.attestor.network_id, &request, &ready)
                    .map_err(|_| EvmError::Malformed)?;
                self.submitter
                    .submit_program(key, &observation, ready.signatures.len())
                    .map_err(submit_error)?
            };
            if outcome.completed() {
                self.watcher.retire_program(key.0, key.1)?;
                self.exchange.forget(key).map_err(|_| EvmError::Journal)?;
            }
            steps.push(Step::Submitted {
                program_id: key.0,
                request_id: key.1,
                outcome,
            });
        }
        Ok(steps)
    }
}

fn submit_error(error: SubmitError) -> EvmError {
    match error {
        SubmitError::Evm(error) => error,
        SubmitError::Journal => EvmError::Journal,
        _ => EvmError::Malformed,
    }
}

fn decode_held(row: &Value) -> Option<Held> {
    let attestation: Attestation = serde_json::from_value(row["attestation"].clone()).ok()?;
    let value = &row["held"];
    let response = unhex(value["response"].as_str()?)?;
    let signer = fixed(value.get("signer"))?;
    let signature = fixed(value.get("signature"))?;
    let digest = attestation.digest();
    if attestation.origin != ORIGIN_PROGRAM
        || attestation.requester == [0; 32]
        || !matches!(attestation.kind, 1 | 2)
        || attestation.network_id[..28] != [0; 28]
        || attestation.network_id[28..] == [0; 4]
        || response.len() > MAX_RESPONSE_BYTES
        || u32::try_from(response.len()).map_or(true, |length| attestation.full_length < length)
        || fixed(value.get("content_digest")) != Some(attestation.content_digest)
        || value["full_length"].as_u64() != Some(u64::from(attestation.full_length))
        || attestation.response_hash != keccak(&response)
        || recover_signer(&digest, &signature).ok() != Some(signer)
    {
        return None;
    }
    let mut signatures = BTreeMap::new();
    for row in value["signatures"].as_array()? {
        let signer = fixed(row.get("signer"))?;
        let signature = fixed(row.get("signature"))?;
        if recover_signer(&digest, &signature).ok() != Some(signer)
            || signatures.insert(signer, signature).is_some()
        {
            return None;
        }
    }
    if signatures.len() > MAX_OBSERVATION_SIGNATURES || signatures.get(&signer) != Some(&signature)
    {
        return None;
    }
    Some(Held {
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
    })
}
