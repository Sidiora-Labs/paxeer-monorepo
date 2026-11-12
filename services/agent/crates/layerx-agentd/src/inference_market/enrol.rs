use std::fs::{self, File};
use std::io::Read;
use std::path::Path;
use std::time::{Duration, Instant};

use layerx_client::ai_market::market_program::{
    codec::{self, Reader},
    dispatch,
    types::{Authentication, MetadataDigest},
    workers::{decode_manifest, CapabilityFields, ManifestSummary, ManifestV2, MAX_MANIFEST_BYTES},
};
use layerx_client::ai_market::{
    market_envelope, named_market_operation, submit_market_operation, AiClientError,
    OperationJournal, OperationRecord, OperationState,
};
use layerx_client::rpc::receipt::AuthenticatedReceipt;
use layerx_client::sequencer::{ReceiptKeyEntry, SequencerEndpoint};
use layerx_crypto::evm_transaction::{decode_signed, recover_sender};

use crate::finality::{wait_for_market_acceptance, FinalityError};

#[derive(Debug)]
pub enum EnrolError {
    Client(AiClientError),
    Finality(FinalityError),
    Measurement(&'static str),
    Binding(&'static str),
    Deadline,
    ExecutionRefused,
    Document(&'static str),
}

impl From<AiClientError> for EnrolError {
    fn from(error: AiClientError) -> Self {
        Self::Client(error)
    }
}
impl From<layerx_client::ai_market::market_program::errors::ApplicationError> for EnrolError {
    fn from(error: layerx_client::ai_market::market_program::errors::ApplicationError) -> Self {
        Self::Client(error.into())
    }
}

impl std::fmt::Display for EnrolError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Client(AiClientError::ProgramRefused {
                operation,
                name,
                code,
            }) => {
                write!(f, "{operation}: {name} ({code:#06x})")
            }
            Self::Client(AiClientError::MissingOperation(name)) => {
                write!(f, "market program does not implement {name}")
            }
            Self::Measurement(reason) | Self::Binding(reason) | Self::Document(reason) => {
                f.write_str(reason)
            }
            _ => write!(f, "enrolment refused: {self:?}"),
        }
    }
}
impl std::error::Error for EnrolError {}

pub struct MeasurementConfig<'a> {
    pub manifest_v1: &'a [u8],
    pub region_code: [u8; 2],
    pub attestation_family: u8,
    pub bandwidth_probe: &'a str,
}

fn read_number(path: &Path) -> Result<u64, EnrolError> {
    fs::read_to_string(path)
        .map_err(|_| EnrolError::Measurement("memory measurement is unreadable"))?
        .trim()
        .parse()
        .map_err(|_| EnrolError::Measurement("memory measurement is malformed"))
}

fn accelerator_memory_mib() -> Result<u32, EnrolError> {
    let mut bytes = 0_u64;
    let mut devices = std::collections::BTreeSet::new();
    let entries = match fs::read_dir("/sys/class/drm") {
        Ok(entries) => entries,
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            let devices = fs::read_dir("/sys/bus/pci/devices")
                .map_err(|_| EnrolError::Measurement("accelerator inventory is unavailable"))?;
            for device in devices {
                let device =
                    device.map_err(|_| EnrolError::Measurement("PCI inventory is unreadable"))?;
                let class = fs::read_to_string(device.path().join("class"))
                    .map_err(|_| EnrolError::Measurement("PCI device class is unreadable"))?;
                if class.trim().starts_with("0x03") || class.trim().starts_with("0x12") {
                    return Err(EnrolError::Measurement(
                        "accelerator does not expose measurable memory",
                    ));
                }
            }
            return Ok(0);
        }
        Err(_) => {
            return Err(EnrolError::Measurement(
                "accelerator inventory is unavailable",
            ))
        }
    };
    for entry in entries {
        let entry =
            entry.map_err(|_| EnrolError::Measurement("accelerator inventory is unreadable"))?;
        let name = entry.file_name();
        let name = name.to_string_lossy();
        if !name
            .strip_prefix("card")
            .is_some_and(|n| !n.is_empty() && n.bytes().all(|b| b.is_ascii_digit()))
        {
            continue;
        }
        let device = fs::canonicalize(entry.path().join("device"))
            .map_err(|_| EnrolError::Measurement("accelerator device is unreadable"))?;
        if !devices.insert(device.clone()) {
            continue;
        }
        let memory = device.join("mem_info_vram_total");
        if !memory.exists() {
            return Err(EnrolError::Measurement(
                "accelerator does not expose measurable memory",
            ));
        }
        bytes = bytes
            .checked_add(read_number(&memory)?)
            .ok_or(EnrolError::Measurement("accelerator memory overflow"))?;
    }
    u32::try_from(bytes / 1_048_576)
        .map_err(|_| EnrolError::Measurement("accelerator memory exceeds manifest bound"))
}

fn system_memory_mib() -> Result<u32, EnrolError> {
    let meminfo = fs::read_to_string("/proc/meminfo")
        .map_err(|_| EnrolError::Measurement("system memory is unreadable"))?;
    let mut fields = meminfo
        .lines()
        .find_map(|line| line.strip_prefix("MemTotal:"))
        .ok_or(EnrolError::Measurement("system memory is absent"))?
        .split_whitespace();
    let kib: u64 = fields
        .next()
        .ok_or(EnrolError::Measurement("system memory is absent"))?
        .parse()
        .map_err(|_| EnrolError::Measurement("system memory is malformed"))?;
    if fields.next() != Some("kB") || fields.next().is_some() {
        return Err(EnrolError::Measurement(
            "system memory has unexpected units",
        ));
    }
    let mut bytes = kib
        .checked_mul(1024)
        .ok_or(EnrolError::Measurement("system memory overflow"))?;
    for path in [
        "/sys/fs/cgroup/memory.max",
        "/sys/fs/cgroup/memory/memory.limit_in_bytes",
    ] {
        match fs::read_to_string(path) {
            Ok(limit) => {
                if limit.trim() != "max" {
                    let limit = limit.trim().parse::<u64>().map_err(|_| {
                        EnrolError::Measurement("process memory limit is malformed")
                    })?;
                    bytes = bytes.min(limit);
                }
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => {}
            Err(_) => {
                return Err(EnrolError::Measurement(
                    "process memory limit is unreadable",
                ))
            }
        }
    }
    u32::try_from(bytes / 1_048_576)
        .map_err(|_| EnrolError::Measurement("system memory exceeds manifest bound"))
}

fn bandwidth_class(endpoint: &str) -> Result<u8, EnrolError> {
    if !(endpoint.starts_with("https://") || endpoint.starts_with("http://127.0.0.1:")) {
        return Err(EnrolError::Measurement(
            "bandwidth probe requires HTTPS or loopback",
        ));
    }
    let agent: ureq::Agent = ureq::Agent::config_builder()
        .timeout_global(Some(Duration::from_secs(2)))
        .max_redirects(0)
        .build()
        .into();
    let start = Instant::now();
    let mut response = agent
        .get(endpoint)
        .call()
        .map_err(|_| EnrolError::Measurement("bandwidth probe failed"))?;
    if response.status().as_u16() != 200 {
        return Err(EnrolError::Measurement("bandwidth probe refused"));
    }
    let length = response
        .headers()
        .get("content-length")
        .and_then(|value| value.to_str().ok())
        .and_then(|value| value.parse::<usize>().ok())
        .filter(|length| (65_536..=1_048_576).contains(length))
        .ok_or(EnrolError::Measurement(
            "bandwidth probe length is absent or outside its bound",
        ))?;
    let mut bytes = vec![0; length];
    response
        .body_mut()
        .with_config()
        .limit(1_048_576)
        .reader()
        .read_exact(&mut bytes)
        .map_err(|_| EnrolError::Measurement("bandwidth probe body is incomplete"))?;
    let elapsed = start.elapsed().as_nanos().max(1);
    let bits_per_second = (bytes.len() as u128) * 8 * 1_000_000_000 / elapsed;
    Ok(match bits_per_second {
        0..=9_999_999 => 1,
        10_000_000..=99_999_999 => 2,
        100_000_000..=999_999_999 => 3,
        _ => 4,
    })
}

/// Measures host resources and encodes the bound version-two manifest.
/// # Errors
/// Returns an error if the template or region is invalid, resource measurement or
/// the bounded bandwidth probe fails, or capability validation or encoding fails.
pub fn measured_manifest(config: &MeasurementConfig<'_>) -> Result<Vec<u8>, EnrolError> {
    if decode_manifest(config.manifest_v1)?.version != 1
        || !config.region_code.iter().all(u8::is_ascii_alphabetic)
    {
        return Err(EnrolError::Binding(
            "host manifest template or region is invalid",
        ));
    }
    let capabilities = CapabilityFields {
        accelerator_memory_mib: accelerator_memory_mib()?,
        system_memory_mib: system_memory_mib()?,
        bandwidth_class: bandwidth_class(config.bandwidth_probe)?,
        region_code: config.region_code,
        attestation_family: config.attestation_family,
        runtime_kind: 2,
    };
    capabilities.validate()?;
    let mut bytes = vec![0; MAX_MANIFEST_BYTES];
    let len = codec::encode_manifest_v2(
        &ManifestV2 {
            manifest_v1: config.manifest_v1,
            capabilities,
        },
        &mut bytes,
    )?;
    bytes.truncate(len);
    Ok(bytes)
}

pub struct SignedManifest {
    pub manifest: Vec<u8>,
    pub proposal: OperationRecord,
    pub acceptance: OperationRecord,
    pub publication: OperationRecord,
}

/// Peer-signed `LockBond` and `PostBid` records carry the admission table's current
/// epoch and `roster::roster_digest` for the state each operation executes on,
/// after identity acceptance. The captured epoch roster is not this binding.
pub struct PriceQuote {
    pub lock_bond: OperationRecord,
    pub post_bid: OperationRecord,
}

impl PriceQuote {
    /// Checks that the market program serves both quote operations.
    /// # Errors
    /// Returns an error if either required operation is absent from the program interface.
    pub fn require_program_interface() -> Result<(), EnrolError> {
        named_market_operation("LockBond")?;
        named_market_operation("PostBid")?;
        Ok(())
    }
}

#[derive(Debug)]
pub struct EnrolOutcome {
    pub manifest_digest: MetadataDigest,
    pub acceptance: AuthenticatedReceipt,
    pub bond_lock: AuthenticatedReceipt,
    pub bid: AuthenticatedReceipt,
}

pub struct EnrolmentRelay<'a> {
    pub endpoint: &'a SequencerEndpoint,
    pub journal: &'a OperationJournal,
    pub receipt_keys: &'a [ReceiptKeyEntry],
    pub timeout: Duration,
}

fn sender(record: &OperationRecord) -> Result<[u8; 20], EnrolError> {
    let (call, signature) = decode_signed(record.signed_bytes()).map_err(AiClientError::from)?;
    recover_sender(&call, &signature)
        .map_err(AiClientError::from)
        .map_err(EnrolError::from)
}

impl SignedManifest {
    /// Verifies the manifest bindings and signed enrolment operation sequence.
    /// # Errors
    /// Returns an error for malformed envelopes, inconsistent identities or manifests,
    /// invalid authority or delegate signatures, or invalid proposal validity.
    pub fn validate(&self) -> Result<(), EnrolError> {
        let summary = decode_manifest(&self.manifest)?;
        if summary.version != 2
            || summary.generation != 1
            || summary.key_version != 1
            || summary.revision != 2
        {
            return Err(EnrolError::Binding(
                "inference enrolment requires manifest v2",
            ));
        }
        let mut context = None;
        let mut proposal_delegate = None;
        let mut proposal_expiry = None;
        for (record, operation) in [
            (&self.proposal, dispatch::EnrollWorker),
            (&self.acceptance, dispatch::AcceptEnrollment),
            (&self.publication, dispatch::PublishMetadata),
        ] {
            let bytes = market_envelope(record)?;
            let env = codec::decode_envelope(&bytes)?.envelope;
            if env.operation != operation || env.market != summary.market {
                return Err(EnrolError::Binding("enrolment operation or market differs"));
            }
            let binding = (env.chain, env.program, env.market, env.config);
            if context.is_some_and(|expected| expected != binding) {
                return Err(EnrolError::Binding("enrolment context differs"));
            }
            context = Some(binding);
            if operation != dispatch::EnrollWorker && env.actor != summary.owner {
                return Err(EnrolError::Binding("peer operation has another owner"));
            }
            if operation == dispatch::EnrollWorker {
                let mut reader = Reader::new(env.payload);
                let owner = reader.principal_id()?;
                let nonce = reader.fixed()?;
                let delegate: [u8; 32] = reader.fixed()?;
                proposal_delegate = Some(delegate);
                let digest = reader.metadata_digest()?;
                let expiry = reader.u64()?;
                proposal_expiry = Some(expiry);
                reader.finish()?;
                if env.authentication != Authentication::Native {
                    return Err(EnrolError::Binding(
                        "proposal requires native owner authority",
                    ));
                }
                if owner != summary.owner
                    || codec::derive_worker(summary.market, owner, nonce)? != summary.worker
                    || digest != summary.digest
                    || expiry <= summary.valid_from
                {
                    return Err(EnrolError::Binding("proposal does not bind the manifest"));
                }
            } else if operation == dispatch::AcceptEnrollment {
                validate_manifest_acceptance(&env, &summary, proposal_delegate, proposal_expiry)?;
            } else {
                validate_manifest_publication(
                    &env,
                    &summary,
                    &self.manifest,
                    proposal_delegate,
                    &bytes,
                )?;
            }
        }
        Ok(())
    }
}

fn validate_manifest_acceptance(
    env: &codec::Envelope<'_>,
    summary: &ManifestSummary,
    proposal_delegate: Option<[u8; 32]>,
    proposal_expiry: Option<u64>,
) -> Result<(), EnrolError> {
    let mut reader = Reader::new(env.payload);
    if reader.worker_id()? != summary.worker
        || reader.u64()? != summary.generation
        || reader.u64()? != summary.key_version
        || reader.metadata_digest()? != summary.digest
    {
        return Err(EnrolError::Binding("acceptance does not bind the manifest"));
    }
    let signature: [u8; 64] = reader.fixed()?;
    reader.finish()?;
    if env.authentication != Authentication::Native {
        return Err(EnrolError::Binding(
            "acceptance requires native peer authority",
        ));
    }
    let delegate = proposal_delegate.ok_or(EnrolError::Binding("proposal delegate is absent"))?;
    let mut consent = Vec::new();
    for value in [
        env.chain.as_bytes(),
        env.program.as_bytes(),
        env.market.as_bytes(),
        summary.worker.as_bytes(),
        summary.owner.as_bytes(),
        &delegate,
    ] {
        consent.extend_from_slice(value);
    }
    consent.extend_from_slice(&summary.generation.to_be_bytes());
    consent.extend_from_slice(&summary.key_version.to_be_bytes());
    consent.extend_from_slice(summary.digest.as_bytes());
    consent.extend_from_slice(
        &proposal_expiry
            .ok_or(EnrolError::Binding("proposal expiry is absent"))?
            .to_be_bytes(),
    );
    let digest = codec::domain_hash("PAXAI/worker-delegate-consent/v1", &consent)?;
    layerx_crypto::ed25519::verify_message(&delegate, &signature, digest.as_bytes())
        .map_err(|_| EnrolError::Binding("peer acceptance consent is invalid"))?;
    Ok(())
}

fn validate_manifest_publication(
    env: &codec::Envelope<'_>,
    summary: &ManifestSummary,
    manifest: &[u8],
    proposal_delegate: Option<[u8; 32]>,
    bytes: &[u8],
) -> Result<(), EnrolError> {
    let mut reader = Reader::new(env.payload);
    if reader.worker_id()? != summary.worker
        || reader.u64()?.checked_add(1) != Some(summary.revision)
        || reader.u64()? != summary.revision
        || reader.metadata_digest()? != summary.digest
        || reader.u64()? != summary.valid_from
        || reader.u64()? != summary.expiry
        || reader.bytes(MAX_MANIFEST_BYTES)? != manifest
    {
        return Err(EnrolError::Binding(
            "publication does not bind the manifest",
        ));
    }
    reader.finish()?;
    let Authentication::Delegate { key, signature } = env.authentication else {
        return Err(EnrolError::Binding(
            "peer publication requires its own delegate signature",
        ));
    };
    if Some(key.0) != proposal_delegate {
        return Err(EnrolError::Binding(
            "publication delegate differs from proposal",
        ));
    }
    layerx_crypto::ed25519::verify_message(
        &key.0,
        &signature.0,
        codec::decode_envelope(bytes)?.request_digest()?.as_bytes(),
    )
    .map_err(|_| EnrolError::Binding("peer publication signature is invalid"))?;
    Ok(())
}

impl EnrolmentRelay<'_> {
    /// Submits the exact signed operation and waits for its authenticated receipt.
    /// # Errors
    /// Returns an error if submission, execution, receipt authentication or the bounded wait fails.
    pub fn submit_and_accept(
        &self,
        record: &mut OperationRecord,
    ) -> Result<AuthenticatedReceipt, EnrolError> {
        let state = submit_market_operation(self.journal, record, self.endpoint)?;
        if !matches!(
            state,
            OperationState::Pending | OperationState::Unknown | OperationState::Executed
        ) {
            return Err(EnrolError::ExecutionRefused);
        }
        wait_for_market_acceptance(
            self.journal,
            record,
            self.endpoint,
            self.receipt_keys,
            self.timeout,
        )
    }

    /// Validates and submits the proposal, peer acceptance and manifest publication.
    /// # Errors
    /// Returns an error if validation or any operation's submission and acceptance fails.
    pub fn enrol_identity(
        &self,
        manifest: &mut SignedManifest,
    ) -> Result<AuthenticatedReceipt, EnrolError> {
        manifest.validate()?;
        self.submit_and_accept(&mut manifest.proposal)?;
        let acceptance = self.submit_and_accept(&mut manifest.acceptance)?;
        self.submit_and_accept(&mut manifest.publication)?;
        Ok(acceptance)
    }

    /// Enrols the host after comparing the signed manifest with measured capabilities.
    /// # Errors
    /// Returns an error if measurement fails, capabilities differ, or peer enrolment fails.
    pub fn enrol_host(
        &self,
        config: &MeasurementConfig<'_>,
        manifest: &mut SignedManifest,
        quote: &mut PriceQuote,
    ) -> Result<EnrolOutcome, EnrolError> {
        if measured_manifest(config)? != manifest.manifest {
            return Err(EnrolError::Binding(
                "signed host manifest differs from measured capability",
            ));
        }
        self.enrol_peer(manifest, quote)
    }

    /// Verifies the peer's market quote and submits identity, bond and bid operations.
    /// # Errors
    /// Returns an error if signatures, quote bindings or manifest checks fail, a required
    /// operation is unavailable, or any submission or acceptance fails.
    pub fn enrol_peer(
        &self,
        manifest: &mut SignedManifest,
        quote: &mut PriceQuote,
    ) -> Result<EnrolOutcome, EnrolError> {
        let manifest_digest = validate_peer(manifest, quote)?;
        let acceptance = self.enrol_identity(manifest)?;
        let bond_lock = self.submit_and_accept(&mut quote.lock_bond)?;
        let bid = self.submit_and_accept(&mut quote.post_bid)?;
        Ok(EnrolOutcome {
            manifest_digest,
            acceptance,
            bond_lock,
            bid,
        })
    }
}

fn validate_peer(
    manifest: &SignedManifest,
    quote: &PriceQuote,
) -> Result<MetadataDigest, EnrolError> {
    manifest.validate()?;
    PriceQuote::require_program_interface()?;
    let peer = sender(&manifest.acceptance)?;
    let reference = market_envelope(&manifest.acceptance)?;
    let reference = codec::decode_envelope(&reference)?.envelope;
    for (record, name) in [(&quote.lock_bond, "LockBond"), (&quote.post_bid, "PostBid")] {
        let bytes = market_envelope(record)?;
        let env = codec::decode_envelope(&bytes)?.envelope;
        if env.operation != named_market_operation(name)?
            || env.chain != reference.chain
            || env.program != reference.program
            || env.market != reference.market
            || env.actor != reference.actor
            || env.config != reference.config
            || sender(record)? != peer
        {
            return Err(EnrolError::Binding(
                "quote is not signed by the enrolled peer for this market",
            ));
        }
    }
    let manifest_digest = decode_manifest(&manifest.manifest)?.digest;
    let lock = market_envelope(&quote.lock_bond)?;
    let bid = market_envelope(&quote.post_bid)?;
    let mut lock = Reader::new(codec::decode_envelope(&lock)?.envelope.payload);
    let mut bid = Reader::new(codec::decode_envelope(&bid)?.envelope.payload);
    let room = lock.digest32()?;
    let bid_id = lock.digest32()?;
    let amount = lock.u128()?;
    lock.finish()?;
    if bid.digest32()? != room
        || bid.digest32()? != bid_id
        || bid.digest32()?.bytes() != manifest_digest.bytes()
        || bid.u128()? == 0
        || bid.u128()? != amount
        || bid.u16()? == 0
        || amount == 0
    {
        return Err(EnrolError::Binding(
            "bond and bid do not bind the manifest and quote",
        ));
    }
    bid.finish()?;
    Ok(manifest_digest)
}

/// Reads one enrolment JSON document within the fixed file-size bound.
/// # Errors
/// Returns an error if the file is unreadable, exceeds the size bound or contains invalid JSON.
pub fn read_enrolment_file(path: &Path) -> Result<serde_json::Value, EnrolError> {
    let mut bytes = Vec::new();
    File::open(path)
        .map_err(|_| EnrolError::Binding("enrolment file is unreadable"))?
        .take((MAX_ENROLMENT_BYTES + 1) as u64)
        .read_to_end(&mut bytes)
        .map_err(|_| EnrolError::Binding("enrolment file is unreadable"))?;
    if bytes.len() > MAX_ENROLMENT_BYTES {
        return Err(EnrolError::Binding("enrolment file exceeds its bound"));
    }
    serde_json::from_slice(&bytes).map_err(|_| EnrolError::Binding("enrolment file is malformed"))
}

pub const MAX_ENROLMENT_BYTES: usize = 262_144;

pub struct EnrolmentContext {
    pub chain_id: u64,
    pub program: [u8; 32],
    pub model_digest: [u8; 32],
    pub runtime_digest: [u8; 32],
}

pub struct EnrolmentDocument {
    pub legacy: Vec<u8>,
    pub manifest: SignedManifest,
    pub quote: PriceQuote,
}

impl EnrolmentDocument {
    /// Parses the host and peer document, binding every record before submission.
    /// # Errors
    /// Refuses missing or malformed members, expired records, or a different chain,
    /// program, model, runtime, or manifest template.
    pub fn parse(
        document: &serde_json::Value,
        context: &EnrolmentContext,
        height: u64,
    ) -> Result<Self, EnrolError> {
        let expiry = document["expiry"]
            .as_u64()
            .ok_or(EnrolError::Document("missing_record"))?;
        if expiry <= height {
            return Err(EnrolError::Document("expired_document"));
        }
        let bytes = |name: &str| -> Result<Vec<u8>, EnrolError> {
            let text = document[name]
                .as_str()
                .ok_or(EnrolError::Document("missing_record"))?;
            layerx_programs::hex::decode(text.strip_prefix("0x").unwrap_or(text))
                .map_err(|_| EnrolError::Document("malformed_document"))
        };
        let record = |name: &str| -> Result<OperationRecord, EnrolError> {
            let record = OperationRecord::from_signed_transaction(bytes(name)?, [0; 32], expiry)?;
            let raw = market_envelope(&record)?;
            let envelope = codec::decode_envelope(&raw)?;
            if envelope.envelope.chain.bytes()
                != layerx_client::ai_market::market_program::merged_chain_domain(context.chain_id)
            {
                return Err(EnrolError::Document("wrong_chain"));
            }
            if envelope.envelope.program.bytes() != context.program {
                return Err(EnrolError::Document("wrong_program"));
            }
            OperationRecord::from_signed_transaction(
                record.signed_bytes().to_vec(),
                envelope.request_digest()?.bytes(),
                expiry,
            )
            .map_err(EnrolError::from)
        };
        let legacy = bytes("manifest_v1")?;
        let manifest = bytes("manifest_v2")?;
        let legacy_summary = decode_manifest(&legacy)?;
        let summary = decode_manifest(&manifest)?;
        if legacy_summary.version != 1
            || summary.version != 2
            || manifest.get(2..legacy.len()) != legacy.get(2..)
            || legacy.get(138..170) != Some(context.runtime_digest.as_slice())
            || legacy.get(174..206) != Some(context.model_digest.as_slice())
        {
            return Err(EnrolError::Document("manifest_mismatch"));
        }
        if summary.expiry <= height {
            return Err(EnrolError::Document("expired_document"));
        }
        let manifest = SignedManifest {
            manifest,
            proposal: record("enroll")?,
            acceptance: record("accept")?,
            publication: record("publish")?,
        };
        let quote = PriceQuote {
            lock_bond: record("lock_bond")?,
            post_bid: record("post_bid")?,
        };
        validate_peer(&manifest, &quote)?;
        let raw = market_envelope(&manifest.proposal)?;
        let mut proposal = Reader::new(codec::decode_envelope(&raw)?.envelope.payload);
        proposal.take(128)?;
        if proposal.u64()? <= height {
            return Err(EnrolError::Document("expired_document"));
        }
        proposal.finish()?;
        Ok(Self {
            legacy,
            manifest,
            quote,
        })
    }
}

/// The host's existing enrolment relay resources; no peer signing key is held.
pub struct EnrolmentService {
    pub endpoint: SequencerEndpoint,
    pub journal: OperationJournal,
    pub receipt_keys: Vec<ReceiptKeyEntry>,
    pub timeout: Duration,
    pub context: EnrolmentContext,
}

impl EnrolmentService {
    /// Relays all five owner-signed records and returns authenticated operation IDs.
    /// # Errors
    /// Returns a document refusal before submission or the relay's execution error.
    pub fn enrol(
        &self,
        document: &serde_json::Value,
        height: u64,
    ) -> Result<serde_json::Value, EnrolError> {
        let mut document = EnrolmentDocument::parse(document, &self.context, height).map_err(
            |error| match error {
                EnrolError::Document(_) => error,
                _ => EnrolError::Document("invalid_enrolment"),
            },
        )?;
        let outcome = EnrolmentRelay {
            endpoint: &self.endpoint,
            journal: &self.journal,
            receipt_keys: &self.receipt_keys,
            timeout: self.timeout,
        }
        .enrol_peer(&mut document.manifest, &mut document.quote)?;
        Ok(serde_json::json!({
            "enrolled": true,
            "manifest_digest": layerx_programs::hex::encode(outcome.manifest_digest.as_bytes()),
            "acceptance": layerx_programs::hex::encode(&outcome.acceptance.activity_id()),
            "bond_lock": layerx_programs::hex::encode(&outcome.bond_lock.activity_id()),
            "bid": layerx_programs::hex::encode(&outcome.bid.activity_id()),
        }))
    }
}
