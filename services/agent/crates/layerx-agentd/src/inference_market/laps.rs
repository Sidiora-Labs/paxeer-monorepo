use super::*;
use layerx_client::ai_market::inference_market::{HostMeterKey, MarketOperation};
use market::{
    evidence,
    laps::{LapReceipt, ReceiptBatch, SettleLapsRequest, LAP_BATCH_MAX},
    types::Signature64,
};

#[derive(Debug)]
pub enum LapError {
    Deal(DealError),
    Invalid(&'static str),
    Client(AiClientError),
    #[cfg(test)]
    Signal(SignalError),
    Io(io::Error),
}
impl From<AiClientError> for LapError {
    fn from(e: AiClientError) -> Self {
        Self::Client(e)
    }
}
impl From<market::errors::ApplicationError> for LapError {
    fn from(e: market::errors::ApplicationError) -> Self {
        Self::Client(e.into())
    }
}
impl From<DealError> for LapError {
    fn from(e: DealError) -> Self {
        Self::Deal(e)
    }
}
impl From<io::Error> for LapError {
    fn from(e: io::Error) -> Self {
        Self::Io(e)
    }
}
impl std::fmt::Display for LapError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Deal(error) => write!(f, "lap processing refused: Deal({error:?})"),
            Self::Invalid(error) => write!(f, "lap processing refused: Invalid({error:?})"),
            Self::Client(error) => write!(f, "lap processing refused: Client({error:?})"),
            #[cfg(test)]
            Self::Signal(error) => write!(f, "lap processing refused: Signal({error:?})"),
            Self::Io(error) => write!(f, "lap processing refused: Io({error:?})"),
        }
    }
}
impl std::error::Error for LapError {}

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct IssuedLap {
    pub deal_id: [u8; 32],
    pub lap_index: u64,
    pub prompt_tokens: u64,
    pub generated_tokens: u64,
    pub cumulative_tokens: u64,
    pub layer_range_digests: Vec<[u8; 32]>,
    pub host_signature: Vec<u8>,
    pub peer_countersignatures: u32,
}
impl IssuedLap {
    pub fn receipt<'a>(&self, ranges: &'a [Digest32]) -> Result<LapReceipt<'a>, LapError> {
        Ok(LapReceipt {
            deal_id: Digest32::new(self.deal_id)?,
            lap_index: self.lap_index,
            prompt_tokens: self.prompt_tokens,
            generated_tokens: self.generated_tokens,
            cumulative_tokens: self.cumulative_tokens,
            layer_range_digests: ranges,
            host_signature: Signature64(
                self.host_signature
                    .as_slice()
                    .try_into()
                    .map_err(|_| LapError::Invalid("signature length"))?,
            ),
            peer_countersignatures: self.peer_countersignatures,
        })
    }
    fn ranges(&self) -> Result<Vec<Digest32>, LapError> {
        self.layer_range_digests
            .iter()
            .map(|d| Digest32::new(*d).map_err(Into::into))
            .collect()
    }
    pub fn verify(&self, host: [u8; 32]) -> Result<(), LapError> {
        let ranges = self.ranges()?;
        let receipt = self.receipt(&ranges)?;
        layerx_crypto::ed25519::verify_digest(
            &host,
            &receipt.host_signature.0,
            &receipt.signing_digest()?.bytes(),
        )
        .map_err(|_| LapError::Invalid("invalid host signature"))
    }
    fn sign(&mut self, host: &HostMeterKey) -> Result<(), LapError> {
        let ranges = self.ranges()?;
        let digest = self.receipt(&ranges)?.signing_digest()?;
        self.host_signature = host.sign(&digest.bytes()).to_vec();
        Ok(())
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct LapFrame {
    pub deal_digest: [u8; 32],
    pub host_meter_key: [u8; 32],
    pub deadline_ms: u64,
    pub receipt: IssuedLap,
}
impl LapFrame {
    pub fn countersign_digest(&self) -> Result<[u8; 32], LapError> {
        self.receipt.verify(self.host_meter_key)?;
        if self.receipt.peer_countersignatures != 0 {
            return Err(LapError::Invalid("issued bitmap must be empty"));
        }
        let ranges = self.receipt.ranges()?;
        let digest = self.receipt.receipt(&ranges)?.digest()?;
        let mut hash = Sha256::new();
        hash.update(b"PAXAI/lap-countersign/v1\0");
        hash.update(self.deal_digest);
        hash.update(self.host_meter_key);
        hash.update(self.deadline_ms.to_be_bytes());
        hash.update(digest.as_bytes());
        Ok(hash.finalize().into())
    }
    #[cfg(test)]
    pub fn countersign(&self, peer: &HostMeterKey) -> Result<LapCountersignature, LapError> {
        Ok(LapCountersignature {
            deal_id: self.receipt.deal_id,
            lap_index: self.receipt.lap_index,
            digest: self.countersign_digest()?,
            signature: peer.sign(&self.countersign_digest()?).to_vec(),
        })
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct LapCountersignature {
    pub deal_id: [u8; 32],
    pub lap_index: u64,
    pub digest: [u8; 32],
    pub signature: Vec<u8>,
}

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
pub struct PeerPenalty {
    pub peer_key: [u8; 32],
    pub bid_id: [u8; 32],
    pub batch_first_lap: u64,
    pub missed_laps: Vec<u64>,
}

#[derive(Clone, Deserialize, Serialize)]
struct PendingLap {
    frame: LapFrame,
    signatures: BTreeMap<String, Vec<u8>>,
    deal_end: bool,
}
#[derive(Deserialize, Serialize)]
struct LapSession {
    deal: DealFrame,
    host_meter_key: [u8; 32],
    batch_size: usize,
    lap_size: u64,
    maximum_generated: u64,
    countersign_window_ms: u64,
    stream_id: [u8; 32],
    next_lap: u64,
    cumulative_tokens: u64,
    generated_tokens: u64,
    pending: Option<PendingLap>,
    receipts: Vec<IssuedLap>,
    proofs: Vec<PendingLap>,
    misses: BTreeMap<String, Vec<u64>>,
    penalties: Vec<PeerPenalty>,
    dropped: BTreeSet<[u8; 32]>,
    ending: bool,
    ended: bool,
    artifact_manifest: Option<Vec<u8>>,
}

pub trait LapTransport {
    fn state(&mut self) -> Result<Vec<u8>, LapError>;
    fn submit(&mut self, operation: &MarketOperation) -> Result<(), LapError>;
    fn meter(&mut self, report: &MeterReport) -> Result<bool, LapError>;
}

pub struct MeterReport {
    pub stream_id: [u8; 32],
    pub cumulative_tokens: u64,
    pub host_meter_key: [u8; 32],
    pub signature: [u8; 64],
}
impl MeterReport {
    pub fn calldata(&self) -> Result<Vec<u8>, LapError> {
        Ok(client::stream_meter_calldata(
            self.stream_id,
            self.cumulative_tokens,
            self.host_meter_key,
            self.signature,
        )?)
    }
}

pub struct LapEngine {
    session: LapSession,
    path: PathBuf,
}
impl LapEngine {
    pub fn open(
        path: &Path,
        state_bytes: &[u8],
        deal: DealFrame,
        host: [u8; 32],
        batch_size: usize,
    ) -> Result<Self, LapError> {
        if !path.is_absolute() || batch_size == 0 || batch_size > LAP_BATCH_MAX || host == [0; 32] {
            return Err(LapError::Invalid("invalid lap configuration"));
        }
        deal.request()?;
        let shared = state::decode_shared_state(state_bytes)?;
        let policy =
            market::registry_ops::PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
        if deal.chain != policy.header.deployment_chain_domain.bytes()
            || deal.program != policy.header.program_id.bytes()
            || deal.market != policy.header.market_id.bytes()
        {
            return Err(LapError::Invalid("deal domain differs"));
        }
        let book = RoomBook::decode(shared.room_market()?.1)?;
        let recorded = book
            .deal(Digest32::new(deal.deal_id)?)?
            .ok_or(LapError::Invalid("unrecorded deal"))?;
        let request = deal.request()?;
        if recorded.room_id != request.room_id
            || recorded.ask_id != request.ask_id
            || recorded.layers != request.layers
            || recorded.roster_digest != request.roster_digest
            || recorded.agreed_price_per_thousand_tokens != request.agreed_price_per_thousand_tokens
        {
            return Err(LapError::Invalid("recorded deal differs"));
        }
        let room = book
            .room(recorded.room_id)?
            .ok_or(LapError::Invalid("missing room"))?;
        if room.state != market::rooms::RoomState::Open
            || recorded.state != market::rooms::DealState::Open
            || room.host_key.bytes() != deal.plan.embedding_host
            || room.model_digest.bytes() != deal.model_digest
            || room.runtime_digest.bytes() != deal.runtime_digest
        {
            return Err(LapError::Invalid("room differs"));
        }
        let workers = WorkerTable::decode(
            market::evaluators::authority::split_identity_section(
                shared.section(Section::IdentityRoster)?,
            )?
            .0,
        )?;
        Self::validate_deal_peers(&deal, &book, &workers)?;
        let ask = book
            .ask(recorded.ask_id)?
            .ok_or(LapError::Invalid("missing ask"))?;
        let mut stream_id = [0; 32];
        stream_id[16..].copy_from_slice(&ask.stream_id.to_be_bytes());
        let session = LapSession {
            deal,
            host_meter_key: host,
            batch_size,
            lap_size: room.profile.lap_size_tokens,
            maximum_generated: room.profile.max_generated_tokens,
            countersign_window_ms: room.profile.acknowledgement_window_ms,
            stream_id,
            next_lap: 0,
            cumulative_tokens: 0,
            generated_tokens: 0,
            pending: None,
            receipts: Vec::new(),
            proofs: Vec::new(),
            misses: BTreeMap::new(),
            penalties: Vec::new(),
            dropped: BTreeSet::new(),
            ending: false,
            ended: false,
            artifact_manifest: None,
        };
        let mut engine = Self {
            session,
            path: path.to_path_buf(),
        };
        if path.exists() {
            engine.restore(host)?;
        } else if market::laps::settlement_from_state(
            state_bytes,
            Digest32::new(engine.session.deal.deal_id)?,
        )?
        .is_some()
        {
            return Err(LapError::Invalid("settled deal requires its lap journal"));
        } else {
            engine.persist()?;
        }
        Ok(engine)
    }
    fn validate_deal_peers(
        deal: &DealFrame,
        book: &RoomBook<'_>,
        workers: &WorkerTable,
    ) -> Result<(), LapError> {
        for split in &deal.plan.layers {
            let bid = book
                .bid(Digest32::new(split.bid_id)?)?
                .ok_or(LapError::Invalid("missing bid"))?;
            let worker = workers
                .iter()
                .find(|w| w.owner.bytes() == split.owner)
                .ok_or(LapError::Invalid("missing peer"))?;
            if bid.peer_key.bytes() != split.owner
                || worker.delegate.0 != split.peer_key
                || worker.metadata.bytes() != split.manifest_digest
                || bid.capability_digest.bytes() != split.manifest_digest
            {
                return Err(LapError::Invalid("peer authority differs"));
            }
        }
        Ok(())
    }
    fn restore(&mut self, host: [u8; 32]) -> Result<(), LapError> {
        let bytes = fs::read(&self.path)?;
        if bytes.len() > 4 * 1024 * 1024 {
            return Err(LapError::Invalid("lap journal too large"));
        }
        let restored: LapSession =
            serde_json::from_slice(&bytes).map_err(|_| LapError::Invalid("corrupt lap journal"))?;
        if restored.deal != self.session.deal
            || restored.host_meter_key != host
            || restored.batch_size != self.session.batch_size
            || restored.lap_size != self.session.lap_size
            || restored.maximum_generated != self.session.maximum_generated
            || restored.countersign_window_ms != self.session.countersign_window_ms
            || restored.stream_id != self.session.stream_id
        {
            return Err(LapError::Invalid("lap journal binding differs"));
        }
        if restored.receipts.len() != restored.proofs.len()
            || restored.receipts.len() > self.session.batch_size
        {
            return Err(LapError::Invalid("receipt proof count"));
        }
        for (receipt, proof) in restored.receipts.iter().zip(&restored.proofs) {
            receipt.verify(host)?;
            proof.frame.receipt.verify(host)?;
            let mut bitmap = 0;
            for (i, peer) in restored.deal.plan.layers.iter().enumerate() {
                if let Some(signature) = proof.signatures.get(&hex(&peer.peer_key)) {
                    verify_countersignature(&proof.frame, &peer.peer_key, signature)?;
                    bitmap |= 1 << i;
                }
            }
            let mut original = receipt.clone();
            original.peer_countersignatures = 0;
            original
                .host_signature
                .clone_from(&proof.frame.receipt.host_signature);
            if original != proof.frame.receipt
                || bitmap != receipt.peer_countersignatures
                || proof.frame.deal_digest != restored.deal.digest()?
            {
                return Err(LapError::Invalid("receipt proof differs"));
            }
        }
        if let Some(pending) = &restored.pending {
            pending.frame.receipt.verify(host)?;
            if pending.frame.deal_digest != restored.deal.digest()? {
                return Err(LapError::Invalid("pending deal differs"));
            }
            for (key, signature) in &pending.signatures {
                let peer = restored
                    .deal
                    .plan
                    .layers
                    .iter()
                    .find(|p| hex(&p.peer_key) == *key)
                    .ok_or(LapError::Invalid("unknown countersigner"))?;
                verify_countersignature(&pending.frame, &peer.peer_key, signature)?;
            }
        }
        self.session = restored;
        if !self.session.receipts.is_empty() {
            self.batch()?;
        }
        Ok(())
    }
    fn persist(&self) -> Result<(), LapError> {
        let parent = self
            .path
            .parent()
            .ok_or(LapError::Invalid("journal has no parent"))?;
        let mut nonce = [0; 16];
        getrandom::fill(&mut nonce)
            .map_err(|_| LapError::Invalid("journal entropy unavailable"))?;
        let temporary = self.path.with_extension(format!("{}.next", hex(&nonce)));
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temporary)?;
        let bytes =
            serde_json::to_vec(&self.session).map_err(|_| LapError::Invalid("lap encoding"))?;
        file.write_all(&bytes)?;
        file.sync_all()?;
        fs::rename(&temporary, &self.path)?;
        fs::File::open(parent)?.sync_all()?;
        Ok(())
    }
    pub fn issue_lap(
        &mut self,
        prompt_tokens: u64,
        generated_tokens: u64,
        ranges: Vec<[u8; 32]>,
        deal_end: bool,
        now_ms: u64,
        host: &HostMeterKey,
    ) -> Result<LapFrame, LapError> {
        if host.public_key() != self.session.host_meter_key
            || self.session.pending.is_some()
            || self.ready_to_settle()
            || self.session.ending
            || self.session.ended
            || ranges.len() != self.session.deal.plan.layers.len()
            || generated_tokens > self.session.lap_size
            || (!deal_end && generated_tokens != self.session.lap_size)
            || generated_tokens == 0
            || (self.session.next_lap != 0 && prompt_tokens != 0)
        {
            return Err(LapError::Invalid("invalid lap boundary or pending batch"));
        }
        let generated = self
            .session
            .generated_tokens
            .checked_add(generated_tokens)
            .ok_or(LapError::Invalid("generated overflow"))?;
        if generated > self.session.maximum_generated {
            return Err(LapError::Invalid("generation limit"));
        }
        let cumulative = self
            .session
            .cumulative_tokens
            .checked_add(prompt_tokens)
            .and_then(|v| v.checked_add(generated_tokens))
            .ok_or(LapError::Invalid("token overflow"))?;
        let mut receipt = IssuedLap {
            deal_id: self.session.deal.deal_id,
            lap_index: self.session.next_lap,
            prompt_tokens,
            generated_tokens,
            cumulative_tokens: cumulative,
            layer_range_digests: ranges,
            host_signature: vec![1; 64],
            peer_countersignatures: 0,
        };
        receipt.sign(host)?;
        let frame = LapFrame {
            deal_digest: self.session.deal.digest()?,
            host_meter_key: host.public_key(),
            deadline_ms: now_ms
                .checked_add(self.session.countersign_window_ms)
                .ok_or(LapError::Invalid("deadline overflow"))?,
            receipt,
        };
        self.session.pending = Some(PendingLap {
            frame: frame.clone(),
            signatures: BTreeMap::new(),
            deal_end,
        });
        self.persist()?;
        Ok(frame)
    }
    pub fn countersign(
        &mut self,
        peer: [u8; 32],
        ack: &LapCountersignature,
        now_ms: u64,
    ) -> Result<(), LapError> {
        if self.session.dropped.contains(&peer)
            || !self
                .session
                .deal
                .plan
                .layers
                .iter()
                .any(|p| p.peer_key == peer)
        {
            return Err(LapError::Invalid("unknown or dropped peer"));
        }
        let pending = self
            .session
            .pending
            .as_mut()
            .ok_or(LapError::Invalid("no pending lap"))?;
        if now_ms >= pending.frame.deadline_ms
            || ack.deal_id != pending.frame.receipt.deal_id
            || ack.lap_index != pending.frame.receipt.lap_index
            || ack.digest != pending.frame.countersign_digest()?
        {
            return Err(LapError::Invalid("late or mismatched countersignature"));
        }
        verify_countersignature(&pending.frame, &peer, &ack.signature)?;
        if let Some(existing) = pending.signatures.get(&hex(&peer)) {
            if existing != &ack.signature {
                return Err(LapError::Invalid("conflicting countersignature"));
            }
            return Ok(());
        }
        pending.signatures.insert(hex(&peer), ack.signature.clone());
        self.persist()
    }
    pub fn finish_lap(&mut self, now_ms: u64, host: &HostMeterKey) -> Result<bool, LapError> {
        if host.public_key() != self.session.host_meter_key {
            return Err(LapError::Invalid("wrong host key"));
        }
        let Some(pending) = &self.session.pending else {
            return Ok(false);
        };
        if pending.signatures.len() != self.session.deal.plan.layers.len()
            && now_ms < pending.frame.deadline_ms
        {
            return Ok(false);
        }
        let mut receipt = pending.frame.receipt.clone();
        let ending = pending.deal_end;
        let first = self
            .session
            .receipts
            .first()
            .map_or(receipt.lap_index, |r| r.lap_index);
        for (index, peer) in self.session.deal.plan.layers.iter().enumerate() {
            if pending.signatures.contains_key(&hex(&peer.peer_key)) {
                receipt.peer_countersignatures |= 1 << index;
            } else {
                let misses = self.session.misses.entry(hex(&peer.peer_key)).or_default();
                misses.push(receipt.lap_index);
                if misses.len() == 2 {
                    self.session.penalties.push(PeerPenalty {
                        peer_key: peer.peer_key,
                        bid_id: peer.bid_id,
                        batch_first_lap: first,
                        missed_laps: misses.clone(),
                    });
                    self.session.dropped.insert(peer.peer_key);
                }
            }
        }
        receipt.sign(host)?;
        self.session.next_lap = receipt
            .lap_index
            .checked_add(1)
            .ok_or(LapError::Invalid("lap overflow"))?;
        self.session.cumulative_tokens = receipt.cumulative_tokens;
        self.session.generated_tokens += receipt.generated_tokens;
        self.session.receipts.push(receipt);
        self.session.ending = ending;
        let completed = self
            .session
            .pending
            .take()
            .ok_or(LapError::Invalid("pending proof missing"))?;
        self.session.proofs.push(completed);
        self.persist()?;
        Ok(true)
    }
    #[cfg(test)]
    pub fn end_deal(&mut self) -> Result<(), LapError> {
        if self.session.pending.is_some() {
            return Err(LapError::Invalid("pending lap at end"));
        }
        self.session.ending = true;
        if self.session.receipts.is_empty() {
            self.session.ended = true;
        }
        self.persist()
    }
    pub fn ready_to_settle(&self) -> bool {
        !self.session.receipts.is_empty()
            && (self.session.receipts.len() == self.session.batch_size || self.session.ending)
    }
    #[cfg(test)]
    pub fn receipts(&self) -> &[IssuedLap] {
        &self.session.receipts
    }
    pub fn penalties(&self) -> &[PeerPenalty] {
        &self.session.penalties
    }
    pub fn dropped(&self, peer: &[u8; 32]) -> bool {
        self.session.dropped.contains(peer)
    }
    pub fn batch(&self) -> Result<ReceiptBatch, LapError> {
        let ranges = self
            .session
            .receipts
            .iter()
            .map(IssuedLap::ranges)
            .collect::<Result<Vec<_>, _>>()?;
        let receipts = self
            .session
            .receipts
            .iter()
            .zip(&ranges)
            .map(|(r, d)| r.receipt(d))
            .collect::<Result<Vec<_>, _>>()?;
        Ok(ReceiptBatch::from_receipts(&receipts)?)
    }
    pub fn seal_and_settle(
        &mut self,
        transport: &mut impl LapTransport,
        host: &HostMeterKey,
    ) -> Result<bool, LapError> {
        if !self.ready_to_settle() {
            return Ok(false);
        }
        let batch = self.batch()?;
        if (batch.countersign_quorum as usize) + 1 < self.session.deal.plan.layers.len() {
            return Err(LapError::Invalid("batch has no settlement quorum"));
        }
        let bytes = transport.state()?;
        let shared = state::decode_shared_state(&bytes)?;
        let policy =
            market::registry_ops::PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
        let frozen = market::epoch::captured_roster(&shared)?
            .ok_or(LapError::Invalid("no frozen roster"))?
            .binding;
        let admission = AdmissionTable::decode(market::reputation_transition::joint_table(
            shared.section(Section::ReputationAdmission)?,
        )?)?;
        let roster = market::roster::roster_digest(&admission, frozen.epoch)?;
        if roster.bytes() != self.session.deal.roster_digest
            || frozen.epoch != self.session.deal.epoch
            || frozen.config.get() != self.session.deal.config
        {
            return Err(LapError::Invalid("batch epoch or roster changed"));
        }
        self.prepare_manifest(&shared, &policy, frozen, &batch)?;
        let manifest = self
            .session
            .artifact_manifest
            .as_ref()
            .ok_or(LapError::Invalid("no manifest"))?;
        match evidence::require_sealed_artifact_chunk_root(&shared, frozen.epoch, manifest, &batch)
        {
            Ok(_) => {}
            Err(error) if error == market::errors::F11_BATCH_UNSEALED => {
                let root = evidence::manifest_root(manifest)
                    .map_err(|_| LapError::Invalid("manifest root"))?;
                let mut payload = 1_u16.to_be_bytes().to_vec();
                payload.extend_from_slice(&root.bytes());
                payload.extend_from_slice(policy.current.digest()?.as_bytes());
                payload.extend_from_slice(
                    &evidence::decode_manifest(manifest)
                        .map_err(|_| LapError::Invalid("manifest"))?
                        .subject,
                );
                payload.extend_from_slice(policy.current.commitments.rubric.as_bytes());
                payload.push(policy.current.assessment_mode);
                transport.submit(&client::seal_evidence(&payload)?)?;
                return Ok(false);
            }
            Err(error) => return Err(error.into()),
        }
        match market::laps::settlement_from_state(&bytes, batch.deal_id)? {
            Some(settled) if settled.batch == batch => {}
            Some(settled) if settled.batch.last_lap_index >= batch.first_lap_index => {
                return Err(LapError::Invalid("conflicting on-chain settlement"))
            }
            _ => {
                transport.submit(&client::settle_laps(&SettleLapsRequest {
                    batch,
                    artifact_manifest: manifest,
                })?)?;
                return Ok(false);
            }
        }
        self.archive_batch(&batch)?;
        if !self.report_meter(transport, host)? {
            return Ok(false);
        }
        self.session.receipts.clear();
        self.session.proofs.clear();
        self.session.misses.clear();
        self.session.artifact_manifest = None;
        self.session.ended = self.session.ending;
        self.persist()?;
        Ok(true)
    }
    fn prepare_manifest(
        &mut self,
        shared: &state::SharedState<'_>,
        policy: &market::registry_ops::PolicySection<'_>,
        frozen: market::types::FrozenBinding,
        batch: &ReceiptBatch,
    ) -> Result<(), LapError> {
        if self.session.artifact_manifest.is_none() {
            let admission = market::reputation_transition::joint_table(
                shared.section(Section::ReputationAdmission)?,
            )?;
            let task_view = shared.replace_section(Section::ReputationAdmission, admission)?;
            let task_set = market::tasks::sealed_task_set(&task_view, frozen.epoch)?;
            let parents = [evidence::ParentRef {
                purpose: evidence::ParentPurpose::Execution,
                root: batch.summary_digest()?.bytes(),
            }];
            let byte_length = self.session.receipts.iter().try_fold(
                0_u64,
                |len, receipt| -> Result<u64, LapError> {
                    let ranges = receipt.ranges()?;
                    let mut out = [0; 2048];
                    Ok(len + receipt.receipt(&ranges)?.encode(&mut out)? as u64)
                },
            )?;
            let manifest = evidence::ArtifactManifest {
                kind: evidence::ArtifactKind::EvaluationEvidence,
                privacy: evidence::Privacy::Public,
                context: evidence::ArtifactContext {
                    chain: frozen.chain,
                    program: frozen.program,
                    market: frozen.market,
                    policy: policy.current.digest()?,
                },
                epoch: frozen.epoch,
                publisher: market::types::PrincipalId::new(self.session.deal.plan.embedding_host)?,
                subject: task_set.bytes(),
                byte_length,
                chunk_count: 1,
                content_root: evidence::content_root(byte_length, 1, &batch.batch_digest.bytes()),
                parents: evidence::Items::Typed(&parents),
                declaration_root: [0; 32],
                reproduction_root: [0; 32],
                access_policy_root: [0; 32],
                not_after_height: u64::MAX,
            };
            let mut encoded = vec![0; evidence::MAX_MANIFEST_BYTES];
            let n = evidence::encode_manifest(&manifest, &mut encoded)
                .map_err(|_| LapError::Invalid("artifact manifest"))?;
            encoded.truncate(n);
            self.session.artifact_manifest = Some(encoded);
            self.persist()?;
        }
        Ok(())
    }
    fn archive_batch(&self, batch: &ReceiptBatch) -> Result<(), LapError> {
        let archive = self
            .path
            .with_extension(format!("batch-{}.json", batch.first_lap_index));
        let evidence_bytes = serde_json::to_vec(&serde_json::json!({
            "receipts": self.session.receipts, "proofs": self.session.proofs,
            "artifact_manifest": self.session.artifact_manifest,
        }))
        .map_err(|_| LapError::Invalid("evidence encoding"))?;
        if archive.exists() {
            if fs::read(&archive)? != evidence_bytes {
                return Err(LapError::Invalid("batch archive differs"));
            }
        } else {
            let mut file = OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(&archive)?;
            file.write_all(&evidence_bytes)?;
            file.sync_all()?;
            fs::File::open(
                archive
                    .parent()
                    .ok_or(LapError::Invalid("archive parent"))?,
            )?
            .sync_all()?;
        }
        Ok(())
    }
    pub fn report_meter(
        &self,
        transport: &mut impl LapTransport,
        host: &HostMeterKey,
    ) -> Result<bool, LapError> {
        if host.public_key() != self.session.host_meter_key {
            return Err(LapError::Invalid("wrong meter key"));
        }
        let bytes = transport.state()?;
        let batch = self.batch()?;
        let settlement = market::laps::settlement_from_state(&bytes, batch.deal_id)?
            .ok_or(LapError::Invalid("meter before settlement"))?;
        if settlement.batch != batch {
            return Err(LapError::Invalid("meter differs from settlement"));
        }
        let digest = client::meter_digest(
            self.session.stream_id,
            batch.cumulative_tokens,
            host.public_key(),
        );
        transport.meter(&MeterReport {
            stream_id: self.session.stream_id,
            cumulative_tokens: batch.cumulative_tokens,
            host_meter_key: host.public_key(),
            signature: host.sign(&digest),
        })
    }
}
fn verify_countersignature(
    frame: &LapFrame,
    peer: &[u8; 32],
    signature: &[u8],
) -> Result<(), LapError> {
    let signature = signature
        .try_into()
        .map_err(|_| LapError::Invalid("countersignature length"))?;
    layerx_crypto::ed25519::verify_digest(peer, &signature, &frame.countersign_digest()?)
        .map_err(|_| LapError::Invalid("bad countersignature"))
}

pub type LapSessions = Arc<Mutex<BTreeMap<[u8; 32], LapEngine>>>;

#[derive(Deserialize, Serialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
pub enum LapAcknowledgement {
    LapCountersign(LapCountersignature),
}
#[derive(Deserialize)]
#[serde(untagged)]
enum LapClientFrame {
    Countersign(LapAcknowledgement),
    Deal(DealClientFrame),
}
#[derive(Deserialize, Serialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
pub enum LapServerFrame {
    LapReceipt { lap: LapFrame },
}

pub struct LapSignalListener {
    listener: DealSignalListener,
    pub laps: LapSessions,
}
impl LapSignalListener {
    pub fn start(
        listener: TcpListener,
        offer: RoomOffer,
        authority: SignalAuthority,
        engine: Arc<Mutex<DealEngine>>,
        clock: fn() -> Result<u64, String>,
        laps: LapSessions,
    ) -> Result<Self, String> {
        offer
            .digest()
            .map_err(|error| format!("invalid signal offer: {error:?}"))?;
        listener
            .set_nonblocking(true)
            .map_err(|error| error.to_string())?;
        #[cfg(test)]
        let address = listener.local_addr().map_err(|error| error.to_string())?;
        let stop = Arc::new(AtomicBool::new(false));
        let shared = Arc::new(Mutex::new(Connections::default()));
        let worker_stop = Arc::clone(&stop);
        let worker_shared = Arc::clone(&shared);
        let worker_engine = Arc::clone(&engine);
        let worker_laps = Arc::clone(&laps);
        let worker = thread::Builder::new().name("inference-deals-signal".to_owned()).spawn(move || {
            let next = AtomicU64::new(1);
            let mut workers: Vec<JoinHandle<()>> = Vec::new();
            let result = (|| {
                while !worker_stop.load(Ordering::Acquire) {
                    let mut index = 0;
                    while index < workers.len() {
                        if workers[index].is_finished() { workers.swap_remove(index).join().map_err(|_| "deal signal connection panicked")?; }
                        else { index += 1; }
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
                            let engine = Arc::clone(&worker_engine);
                            let laps = Arc::clone(&worker_laps);
                            workers.push(thread::Builder::new().name("inference-deals-peer".to_owned()).spawn(move || {
                                let _ = serve_lap_connection(&mut stream, &offer, &authority, &connection, &engine, clock, &laps);
                            }).map_err(|error| error.to_string())?);
                        }
                        Err(error) if error.kind() == io::ErrorKind::WouldBlock => thread::sleep(Duration::from_millis(5)),
                        Err(error) => return Err(error.to_string()),
                    }
                }
                Ok(())
            })();
            if let Ok(connections) = worker_shared.lock() { for stream in connections.sockets.values() { let _ = stream.shutdown(Shutdown::Both); } }
            for worker in workers { let _ = worker.join(); }
            result
        }).map_err(|error| error.to_string())?;
        Ok(Self {
            laps,
            listener: DealSignalListener {
                signal: SignalListener {
                    #[cfg(test)]
                    address,
                    stop,
                    shared,
                    thread: Some(worker),
                },
                engine,
                announced: None,
                live_announced: BTreeSet::new(),
            },
        })
    }
    pub fn check(&self) -> Result<(), String> {
        self.listener.check()
    }
    #[allow(
        clippy::needless_pass_by_value,
        reason = "needless_pass_by_value: src/main.rs passes an owned FinalizedSnapshot to LapSignalListener::tick"
    )]
    pub fn tick(
        &mut self,
        snapshot: FinalizedSnapshot,
        now_ms: u64,
        directory: &Path,
        host: &HostMeterKey,
        batch_size: usize,
    ) -> Result<(), LapError> {
        self.listener.tick(snapshot.clone(), now_ms)?;
        let live = self
            .listener
            .engine
            .lock()
            .map_err(|_| LapError::Invalid("deal engine unavailable"))?
            .session
            .live
            .iter()
            .map(|n| n.frame.clone())
            .collect::<Vec<_>>();
        let mut laps = self
            .laps
            .lock()
            .map_err(|_| LapError::Invalid("laps unavailable"))?;
        for frame in live {
            if let std::collections::btree_map::Entry::Vacant(entry) = laps.entry(frame.deal_id) {
                let path = directory.join(format!("lap-{}.json", hex(&frame.deal_id)));
                let engine = LapEngine::open(
                    &path,
                    snapshot.state_bytes(),
                    frame.clone(),
                    host.public_key(),
                    batch_size,
                )?;
                entry.insert(engine);
            }
        }
        for engine in laps.values_mut() {
            engine.finish_lap(now_ms, host)?;
            if let Some(pending) = &engine.session.pending {
                if now_ms < pending.frame.deadline_ms {
                    self.broadcast(engine, &pending.frame)?;
                }
            }
            {
                let mut deals = self
                    .listener
                    .engine
                    .lock()
                    .map_err(|_| LapError::Invalid("deal engine unavailable"))?;
                let mut changed = false;
                for penalty in engine.penalties() {
                    changed |= deals.session.excluded.insert(penalty.peer_key);
                }
                if changed {
                    deals.persist()?;
                }
            }
            let connections = self
                .listener
                .signal
                .shared
                .lock()
                .map_err(|_| LapError::Invalid("connections unavailable"))?;
            for penalty in engine.penalties() {
                if let Some(peer) = connections.peers.get(&penalty.peer_key) {
                    if let Some(stream) = connections.sockets.get(&peer.connection) {
                        stream.shutdown(Shutdown::Both)?;
                    }
                }
            }
        }
        Ok(())
    }
    pub fn issue_lap(
        &self,
        input: LapInput,
        now_ms: u64,
        host: &HostMeterKey,
    ) -> Result<LapFrame, LapError> {
        let mut laps = self
            .laps
            .lock()
            .map_err(|_| LapError::Invalid("laps unavailable"))?;
        let engine = laps
            .get_mut(&input.deal_id)
            .ok_or(LapError::Invalid("unknown live deal"))?;
        if input.lap_index != engine.session.next_lap {
            return Err(LapError::Invalid("unexpected lap index"));
        }
        if let Some(pending) = &engine.session.pending {
            let receipt = &pending.frame.receipt;
            if receipt.prompt_tokens == input.prompt_tokens
                && receipt.generated_tokens == input.generated_tokens
                && receipt.layer_range_digests == input.layer_range_digests
                && pending.deal_end == input.deal_end
            {
                self.broadcast(engine, &pending.frame)?;
                return Ok(pending.frame.clone());
            }
            return Err(LapError::Invalid("conflicting output for pending lap"));
        }
        let frame = engine.issue_lap(
            input.prompt_tokens,
            input.generated_tokens,
            input.layer_range_digests,
            input.deal_end,
            now_ms,
            host,
        )?;
        self.broadcast(engine, &frame)?;
        Ok(frame)
    }
    fn broadcast(&self, engine: &LapEngine, frame: &LapFrame) -> Result<(), LapError> {
        let connections = self
            .listener
            .signal
            .shared
            .lock()
            .map_err(|_| LapError::Invalid("connections unavailable"))?;
        for split in &engine.session.deal.plan.layers {
            if engine.dropped(&split.peer_key)
                || engine
                    .session
                    .pending
                    .as_ref()
                    .is_some_and(|p| p.signatures.contains_key(&hex(&split.peer_key)))
            {
                continue;
            }
            if let Some(peer) = connections.peers.get(&split.peer_key) {
                send_lap(&peer.writer, frame)?;
            }
        }
        Ok(())
    }
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct LapInput {
    pub deal_id: [u8; 32],
    pub lap_index: u64,
    pub prompt_tokens: u64,
    pub generated_tokens: u64,
    pub layer_range_digests: Vec<[u8; 32]>,
    pub deal_end: bool,
}
fn serve_lap_connection(
    stream: &mut TcpStream,
    offer: &RoomOffer,
    authority: &SignalAuthority,
    connection: &Connection,
    engine: &Arc<Mutex<DealEngine>>,
    clock: fn() -> Result<u64, String>,
    laps: &LapSessions,
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
        let _admitted = {
            let mut connections = connection
                .shared
                .lock()
                .map_err(|_| SignalError::Backpressure)?;
            if connections.peers.contains_key(&hello.peer_key) {
                return Err(SignalError::DuplicatePeer);
            }
            engine
                .lock()
                .map_err(|_| SignalError::Backpressure)?
                .admit(connection.id, &hello, &nonce)
                .map_err(|_| SignalError::Unenrolled)?;
            let admitted = AdmittedConnection {
                engine,
                key: hello.peer_key,
                connection: connection.id,
            };
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
            admitted
        };
        loop {
            handle_lap_frame(
                receive_lap(stream, &writer, &mut fragments)?,
                connection,
                engine,
                clock,
                laps,
                &hello.peer_key,
            )?;
        }
    })();
    if let Err(code) = &result {
        let _ = send(&writer, &ServerFrame::Error { code: code.clone() });
        if let Ok(mut writer) = writer.lock() {
            let _ = write_frame(
                &mut writer,
                8,
                &(if *code == SignalError::FrameTooLarge {
                    1009_u16
                } else {
                    1008_u16
                })
                .to_be_bytes(),
            );
        }
    }
    result
}
fn handle_lap_frame(
    frame: LapClientFrame,
    connection: &Connection,
    engine: &Arc<Mutex<DealEngine>>,
    clock: fn() -> Result<u64, String>,
    laps: &LapSessions,
    peer: &[u8; 32],
) -> Result<(), SignalError> {
    match frame {
        LapClientFrame::Countersign(LapAcknowledgement::LapCountersign(ack)) => {
            let now = clock().map_err(|_| SignalError::ChainUnavailable)?;
            laps.lock()
                .map_err(|_| SignalError::Backpressure)?
                .get_mut(&ack.deal_id)
                .ok_or(SignalError::InvalidFrame)?
                .countersign(*peer, &ack, now)
                .map_err(|_| SignalError::BadSignature)?;
        }
        LapClientFrame::Deal(DealClientFrame::Acknowledgement(ack)) => {
            let now = clock().map_err(|_| SignalError::ChainUnavailable)?;
            engine
                .lock()
                .map_err(|_| SignalError::Backpressure)?
                .acknowledge(peer, &ack, now)
                .map_err(|error| match error {
                    DealError::Signal(code) => code,
                    _ => SignalError::InvalidFrame,
                })?;
        }
        LapClientFrame::Deal(DealClientFrame::Signal(ClientFrame::Description {
            to,
            description,
        })) => relay_description(&connection.shared, peer, &to, description)?,
        LapClientFrame::Deal(DealClientFrame::Signal(ClientFrame::Candidate { to, candidate })) => {
            relay(
                &connection.shared,
                &to,
                &ServerFrame::Candidate {
                    from: *peer,
                    candidate,
                },
            )?;
        }
        LapClientFrame::Deal(DealClientFrame::Signal(ClientFrame::Hello(_))) => {
            return Err(SignalError::InvalidFrame);
        }
    }
    Ok(())
}
fn receive_lap(
    stream: &mut TcpStream,
    writer: &Arc<Mutex<TcpStream>>,
    fragments: &mut Option<Vec<u8>>,
) -> Result<LapClientFrame, SignalError> {
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
        let length = usize::try_from(length).map_err(|_| SignalError::FrameTooLarge)?;
        let mut bytes = vec![0; length];
        stream
            .read_exact(&mut bytes)
            .map_err(|_| SignalError::InvalidFrame)?;
        for (index, byte) in bytes.iter_mut().enumerate() {
            *byte ^= mask[index % 4];
        }
        match opcode {
            9 => {
                let mut writer = writer.lock().map_err(|_| SignalError::Backpressure)?;
                write_frame(&mut writer, 10, &bytes).map_err(|_| SignalError::Backpressure)?;
            }
            10 => {}
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
                    return serde_json::from_slice(
                        &fragments.take().ok_or(SignalError::InvalidFrame)?,
                    )
                    .map_err(|_| SignalError::InvalidFrame);
                }
            }
            _ => return Err(SignalError::InvalidFrame),
        }
    }
}

pub fn send_lap(writer: &Arc<Mutex<TcpStream>>, frame: &LapFrame) -> Result<(), LapError> {
    let bytes = serde_json::to_vec(&LapServerFrame::LapReceipt { lap: frame.clone() })
        .map_err(|_| LapError::Invalid("lap signal encoding"))?;
    if bytes.len() > MAX_FRAME_BYTES {
        return Err(LapError::Invalid("lap frame too large"));
    }
    let mut writer = writer
        .lock()
        .map_err(|_| LapError::Invalid("peer unavailable"))?;
    write_frame(&mut writer, 1, &bytes)?;
    Ok(())
}
#[cfg(test)]
pub fn receive_countersignature(
    stream: &mut TcpStream,
    writer: &Arc<Mutex<TcpStream>>,
) -> Result<LapCountersignature, LapError> {
    match receive_lap(stream, writer, &mut None).map_err(LapError::Signal)? {
        LapClientFrame::Countersign(LapAcknowledgement::LapCountersign(ack)) => Ok(ack),
        _ => Err(LapError::Invalid("expected lap countersignature")),
    }
}
