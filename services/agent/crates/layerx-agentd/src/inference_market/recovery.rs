use super::*;

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct RoomPaused {
    pub room_id: [u8; 32],
    pub previous_deal_id: [u8; 32],
    pub retry_after_seconds: u64,
}
impl RoomPaused {
    /// Writes the room pause response.
    ///
    /// # Errors
    /// Returns an error if the response cannot be written or flushed.
    pub fn respond(&self, stream: &mut impl Write) -> Result<(), LapError> {
        let body = br#"{"type":"error","error":{"type":"room_paused","message":"Room is waiting for layer capacity"}}"#;
        write!(stream, "HTTP/1.1 503 Service Unavailable\r\nRetry-After: {}\r\nContent-Type: application/json\r\nCache-Control: no-store\r\nConnection: close\r\nContent-Length: {}\r\n\r\n", self.retry_after_seconds, body.len())?;
        stream.write_all(body)?;
        stream.flush()?;
        Ok(())
    }
}

#[derive(Clone, Copy, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum LossReason {
    DataChannel,
    LivenessTimeout,
    SlashMark,
}

#[derive(Clone, Copy, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct PeerLoss {
    pub deal_id: [u8; 32],
    pub peer_key: [u8; 32],
}

#[derive(Clone, Debug, Default, Deserialize, Serialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct ResumePoint {
    pub last_countersigned_lap: Option<u64>,
    pub next_lap_index: u64,
    pub cumulative_tokens: u64,
    pub generated_tokens: u64,
}

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CacheRecompute {
    pub peer_key: [u8; 32],
    pub first_layer: u16,
    pub end_layer: u16,
    pub through_lap: Option<u64>,
}

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(rename_all = "snake_case")]
pub enum RecoveryState {
    Paused,
    Negotiating,
    Live,
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RecoveryRecord {
    pub previous_deal: DealFrame,
    pub lost_peers: BTreeMap<String, LossReason>,
    pub resume: ResumePoint,
    pub stream_id: [u8; 32],
    pub replacement: Option<DealFrame>,
    pub recompute: Vec<CacheRecompute>,
    pub state: RecoveryState,
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct RecoveryJournal {
    config: DealConfig,
    records: Vec<RecoveryRecord>,
    unresponsive_since: BTreeMap<String, u64>,
}

#[derive(Deserialize)]
struct LapArchive {
    proofs: Vec<PendingLap>,
}

pub struct RecoveryEngine {
    path: PathBuf,
    journal: RecoveryJournal,
}
impl RecoveryEngine {
    /// Opens the protected recovery journal for the configured room.
    ///
    /// # Errors
    /// Returns an error for an unavailable, corrupt or differently bound journal.
    pub fn open(path: &Path, config: DealConfig) -> Result<Self, LapError> {
        if !path.is_absolute() {
            return Err(LapError::Invalid("recovery journal must be absolute"));
        }
        let journal = match fs::symlink_metadata(path) {
            Ok(_) => {
                let bytes = layerx_agentd::config::read_protected_source(path, 4 * 1024 * 1024)
                    .map_err(|_| LapError::Invalid("recovery journal unavailable"))?;
                let journal: RecoveryJournal = serde_json::from_slice(&bytes)
                    .map_err(|_| LapError::Invalid("corrupt recovery journal"))?;
                if journal.config != config || journal.records.len() > market::rooms::DEAL_CAPACITY
                {
                    return Err(LapError::Invalid("recovery journal room differs"));
                }
                let mut previous = BTreeSet::new();
                for record in &journal.records {
                    record.previous_deal.digest()?;
                    if record.previous_deal.room_id != config.room_id
                        || record.previous_deal.model_digest != config.model_digest
                        || record.previous_deal.runtime_digest != config.runtime_digest
                        || record.previous_deal.program != config.program
                        || record.previous_deal.plan.layer_count != config.layer_count
                        || !previous.insert(record.previous_deal.deal_id)
                        || record.lost_peers.is_empty()
                        || record.resume.next_lap_index
                            != match record.resume.last_countersigned_lap {
                                Some(lap) => lap
                                    .checked_add(1)
                                    .ok_or(LapError::Invalid("recovery lap overflow"))?,
                                None => 0,
                            }
                    {
                        return Err(LapError::Invalid("recovery binding differs"));
                    }
                    if let Some(frame) = &record.replacement {
                        frame.digest()?;
                        if frame.ask_id != record.previous_deal.ask_id
                            || frame.room_id != config.room_id
                            || frame.model_digest != config.model_digest
                            || frame.runtime_digest != config.runtime_digest
                            || frame.program != config.program
                            || frame.plan.layer_count != config.layer_count
                            || frame.deal_id == record.previous_deal.deal_id
                            || frame
                                .plan
                                .layers
                                .iter()
                                .any(|p| record.lost_peers.contains_key(&hex(&p.peer_key)))
                        {
                            return Err(LapError::Invalid("replacement binding differs"));
                        }
                    } else if record.state != RecoveryState::Paused {
                        return Err(LapError::Invalid("missing replacement deal"));
                    }
                }
                journal
            }
            Err(error) if error.kind() == io::ErrorKind::NotFound => RecoveryJournal {
                config,
                records: Vec::new(),
                unresponsive_since: BTreeMap::new(),
            },
            Err(error) => return Err(error.into()),
        };
        let engine = Self {
            path: path.to_owned(),
            journal,
        };
        engine.persist()?;
        Ok(engine)
    }

    fn persist(&self) -> Result<(), LapError> {
        let bytes = serde_json::to_vec(&self.journal)
            .map_err(|_| LapError::Invalid("recovery encoding"))?;
        if bytes.len() > 4 * 1024 * 1024 {
            return Err(LapError::Invalid("recovery journal exceeds bound"));
        }
        let mut nonce = [0; 16];
        getrandom::fill(&mut nonce)
            .map_err(|_| LapError::Invalid("recovery entropy unavailable"))?;
        let temporary = self.path.with_extension(format!("{}.next", hex(&nonce)));
        let result = (|| {
            let mut file = OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(&temporary)?;
            file.write_all(&bytes)?;
            file.sync_all()?;
            fs::rename(&temporary, &self.path)?;
            fs::File::open(
                self.path
                    .parent()
                    .ok_or(LapError::Invalid("recovery journal parent"))?,
            )?
            .sync_all()?;
            Ok(())
        })();
        if result.is_err() {
            let _ = fs::remove_file(&temporary);
        }
        result
    }

    #[must_use]
    pub fn records(&self) -> &[RecoveryRecord] {
        &self.journal.records
    }

    #[must_use]
    pub fn paused(&self) -> Option<RoomPaused> {
        self.journal
            .records
            .iter()
            .find(|r| r.state != RecoveryState::Live)
            .map(|record| RoomPaused {
                room_id: self.journal.config.room_id,
                previous_deal_id: record.previous_deal.deal_id,
                retry_after_seconds: 1,
            })
    }

    /// Refuses completion routes while replacement capacity or finality is pending.
    ///
    /// # Errors
    /// Returns an error if writing the pause response fails.
    pub fn refuse_paused_completion<S: Read + Write>(
        &self,
        stream: &mut super::super::serve::CompletionConnection<'_, S>,
    ) -> Result<bool, LapError> {
        if stream.is_completion() {
            if let Some(paused) = self.paused() {
                Self::drain_paused_request(stream)?;
                paused.respond(stream)?;
                return Ok(true);
            }
        }
        Ok(false)
    }

    fn drain_paused_request(stream: &mut impl Read) -> Result<(), LapError> {
        let mut headers = Vec::new();
        while !headers.ends_with(b"\r\n\r\n") {
            if headers.len() >= 16_384 {
                return Err(LapError::Invalid("completion headers exceed bound"));
            }
            let mut byte = [0];
            stream.read_exact(&mut byte)?;
            headers.push(byte[0]);
        }
        let headers = std::str::from_utf8(&headers)
            .map_err(|_| LapError::Invalid("invalid completion headers"))?;
        let mut length = None;
        for line in headers
            .split("\r\n")
            .skip(1)
            .filter(|line| !line.is_empty())
        {
            let (name, value) = line
                .split_once(':')
                .ok_or(LapError::Invalid("invalid completion header"))?;
            if name.eq_ignore_ascii_case("transfer-encoding") {
                return Err(LapError::Invalid("completion transfer encoding refused"));
            }
            if name.eq_ignore_ascii_case("content-length") {
                if length.is_some() {
                    return Err(LapError::Invalid("duplicate completion length"));
                }
                let value = value
                    .trim()
                    .parse::<usize>()
                    .map_err(|_| LapError::Invalid("invalid completion length"))?;
                if value > super::super::serve::BODY_LIMIT {
                    return Err(LapError::Invalid("completion body exceeds bound"));
                }
                length = Some(value);
            }
        }
        let length = length.unwrap_or(0) as u64;
        if io::copy(&mut stream.take(length), &mut io::sink())? != length {
            return Err(LapError::Invalid("incomplete completion body"));
        }
        Ok(())
    }

    #[must_use]
    pub fn superseded(&self, deal_id: &[u8; 32]) -> bool {
        self.journal
            .records
            .iter()
            .any(|r| r.previous_deal.deal_id == *deal_id)
    }

    fn checkpoint(&self, engine: &LapEngine) -> Result<ResumePoint, LapError> {
        let mut point = self
            .journal
            .records
            .iter()
            .find(|r| {
                r.replacement
                    .as_ref()
                    .is_some_and(|f| f.deal_id == engine.session.deal.deal_id)
            })
            .map_or_else(ResumePoint::default, |r| r.resume.clone());
        let mut proofs = engine.session.proofs.clone();
        if engine.session.next_lap > engine.session.receipts.len() as u64 {
            let prefix = engine
                .path
                .file_stem()
                .and_then(std::ffi::OsStr::to_str)
                .ok_or(LapError::Invalid("lap archive name"))?;
            for entry in fs::read_dir(
                engine
                    .path
                    .parent()
                    .ok_or(LapError::Invalid("lap archive parent"))?,
            )? {
                let entry = entry?;
                let name = entry.file_name();
                let Some(name) = name.to_str() else {
                    continue;
                };
                if name.starts_with(&format!("{prefix}.batch-"))
                    && entry
                        .path()
                        .extension()
                        .is_some_and(|extension| extension == "json")
                {
                    let bytes = layerx_agentd::config::read_protected_source(
                        &entry.path(),
                        4 * 1024 * 1024,
                    )
                    .map_err(|_| LapError::Invalid("lap archive unavailable"))?;
                    let archive: LapArchive = serde_json::from_slice(&bytes)
                        .map_err(|_| LapError::Invalid("corrupt lap archive"))?;
                    proofs.extend(archive.proofs);
                }
            }
        }
        if let Some(pending) = &engine.session.pending {
            proofs.push(pending.clone());
        }
        proofs.sort_by_key(|p| p.frame.receipt.lap_index);
        for proof in proofs {
            let receipt = &proof.frame.receipt;
            receipt.verify(engine.session.host_meter_key)?;
            if proof.frame.deal_digest != engine.session.deal.digest()?
                || receipt.deal_id != engine.session.deal.deal_id
            {
                return Err(LapError::Invalid("checkpoint deal differs"));
            }
            let fully_signed = engine
                .session
                .deal
                .plan
                .layers
                .iter()
                .all(|p| proof.signatures.contains_key(&hex(&p.peer_key)));
            for peer in &engine.session.deal.plan.layers {
                if let Some(signature) = proof.signatures.get(&hex(&peer.peer_key)) {
                    verify_countersignature(&proof.frame, &peer.peer_key, signature)?;
                }
            }
            if fully_signed
                && point
                    .last_countersigned_lap
                    .is_none_or(|n| receipt.lap_index > n)
            {
                point.last_countersigned_lap = Some(receipt.lap_index);
                point.next_lap_index = receipt
                    .lap_index
                    .checked_add(1)
                    .ok_or(LapError::Invalid("resume lap overflow"))?;
                point.cumulative_tokens = receipt.cumulative_tokens;
                // Prompt tokens occur only at index zero; cumulative generated work includes all earlier laps.
                let prompt = engine
                    .session
                    .cumulative_tokens
                    .checked_sub(engine.session.generated_tokens)
                    .ok_or(LapError::Invalid("invalid lap token counters"))?;
                point.generated_tokens = receipt
                    .cumulative_tokens
                    .checked_sub(if receipt.lap_index == 0 {
                        receipt.prompt_tokens
                    } else {
                        prompt
                    })
                    .ok_or(LapError::Invalid("invalid resume token counters"))?;
            }
        }
        Ok(point)
    }

    /// Retains the checkpoint, excludes the lost peer and negotiates a replacement.
    ///
    /// # Errors
    /// Returns an error for an unbound loss, invalid snapshot or failed persistence.
    pub fn replan_after_loss(
        &mut self,
        deals: &mut DealEngine,
        laps: &mut BTreeMap<[u8; 32], LapEngine>,
        loss: PeerLoss,
        reason: LossReason,
        snapshot: &FinalizedSnapshot,
        now_ms: u64,
    ) -> Result<(), LapError> {
        self.validate_source(deals, snapshot)?;
        let existing = self
            .journal
            .records
            .iter()
            .position(|r| r.previous_deal.deal_id == loss.deal_id);
        if existing.is_none() {
            let previous = deals
                .session
                .live
                .iter()
                .find(|n| n.frame.deal_id == loss.deal_id)
                .ok_or(LapError::Invalid("loss is not from a live deal"))?
                .frame
                .clone();
            if !previous
                .plan
                .layers
                .iter()
                .any(|p| p.peer_key == loss.peer_key)
            {
                return Err(LapError::Invalid("loss is not an assigned peer"));
            }
            let engine = laps
                .get(&loss.deal_id)
                .ok_or(LapError::Invalid("lost deal lap journal unavailable"))?;
            if self.journal.records.len() == market::rooms::DEAL_CAPACITY {
                return Err(LapError::Invalid("recovery history full"));
            }
            let resume = self.checkpoint(engine)?;
            self.journal.records.push(RecoveryRecord {
                previous_deal: previous,
                lost_peers: BTreeMap::from([(hex(&loss.peer_key), reason)]),
                resume,
                stream_id: engine.session.stream_id,
                replacement: None,
                recompute: Vec::new(),
                state: RecoveryState::Paused,
            });
        } else {
            let record =
                &mut self.journal.records[existing.ok_or(LapError::Invalid("recovery missing"))?];
            if !record
                .previous_deal
                .plan
                .layers
                .iter()
                .any(|p| p.peer_key == loss.peer_key)
            {
                return Err(LapError::Invalid("loss is not an assigned peer"));
            }
            record.lost_peers.insert(hex(&loss.peer_key), reason);
        }
        // Retain the checkpoint and loss before changing the negotiation or lap journals.
        self.persist()?;
        self.reconcile(deals, laps)?;
        self.plan_pending(deals, snapshot, now_ms)
    }

    fn reconcile(
        &self,
        deals: &mut DealEngine,
        laps: &mut BTreeMap<[u8; 32], LapEngine>,
    ) -> Result<(), LapError> {
        for record in &self.journal.records {
            for key in record.lost_peers.keys() {
                let key: [u8; 32] = layerx_programs::hex::decode(key)
                    .map_err(|_| LapError::Invalid("invalid lost peer"))?
                    .try_into()
                    .map_err(|_| LapError::Invalid("invalid lost peer width"))?;
                deals.session.excluded.insert(key);
                deals.peers.remove(&key);
            }
            deals
                .session
                .live
                .retain(|n| n.frame.deal_id != record.previous_deal.deal_id);
            if let Some(engine) = laps.get_mut(&record.previous_deal.deal_id) {
                engine.session.pending = None;
                engine.session.ending = true;
                engine.persist()?;
            }
        }
        deals.persist()?;
        Ok(())
    }

    fn validate_source(
        &self,
        deals: &DealEngine,
        snapshot: &FinalizedSnapshot,
    ) -> Result<(), LapError> {
        let config = &self.journal.config;
        if snapshot.binding().program.bytes() != config.program
            || deals.session.config != *config
            || snapshot.header().deployment_chain_domain.bytes() != market::merged_chain_domain(125)
            || deals.snapshot.as_ref().is_some_and(|previous| {
                snapshot.binding().execution_height < previous.binding().execution_height
            })
        {
            return Err(LapError::Invalid("recovery snapshot binding differs"));
        }
        Ok(())
    }

    fn replacement_candidates(
        deals: &DealEngine,
        record: &RecoveryRecord,
        snapshot: &FinalizedSnapshot,
        book: &RoomBook<'_>,
        room: &RoomRecord,
        view: &BookView<'_>,
    ) -> Result<Vec<PlanPeer>, LapError> {
        let ask = book
            .ask(Digest32::new(record.previous_deal.ask_id)?)?
            .ok_or(LapError::Invalid("recovery ask unavailable"))?;
        let mut candidates = Vec::new();
        let mut bid_ids = record
            .previous_deal
            .plan
            .layers
            .iter()
            .map(|p| Digest32::new(p.bid_id))
            .collect::<Result<Vec<_>, _>>()?;
        for index in 0..view.counts()[1] {
            let bid_id = view.bid(index)?.order_id;
            if !bid_ids.contains(&bid_id) {
                bid_ids.push(bid_id);
            }
        }
        for bid_id in bid_ids {
            let bid = book
                .bid(bid_id)?
                .ok_or(LapError::Invalid("recovery bid unavailable"))?;
            if bid.room_id != room.room_id
                || bid.bond_amount < room.profile.minimum_bond
                || bid.minimum_price_per_thousand_tokens > ask.maximum_price_per_thousand_tokens
            {
                continue;
            }
            if market::bonds::bond_from_state(snapshot.state_bytes(), bid.bid_id)?
                .is_some_and(|b| b.slashed != 0 || b.refunded)
            {
                continue;
            }
            for (key, peer) in &deals.peers {
                if deals.excluded(key) || deals.admitted_peer(snapshot, key, &peer.bytes).is_err() {
                    continue;
                }
                let manifest = workers::decode_manifest(&peer.bytes)?;
                if manifest.owner != bid.peer_key
                    || manifest.digest.bytes() != bid.capability_digest.bytes()
                {
                    continue;
                }
                let capabilities = manifest
                    .capabilities
                    .ok_or(LapError::Invalid("recovery peer has no capabilities"))?;
                candidates.push(PlanPeer {
                    peer_key: *key,
                    owner: bid.peer_key.bytes(),
                    bid_id: bid.bid_id.bytes(),
                    manifest_digest: manifest.digest.bytes(),
                    accelerator_memory_mib: capabilities.accelerator_memory_mib,
                    layer_capacity: bid.layer_capacity,
                    minimum_price: bid.minimum_price_per_thousand_tokens,
                });
            }
        }
        Ok(candidates)
    }

    fn prepare_replacement(
        record: &mut RecoveryRecord,
        deals: &mut DealEngine,
        snapshot: &FinalizedSnapshot,
        room: &RoomRecord,
        candidates: &[PlanPeer],
        epoch: u64,
        roster: [u8; 32],
    ) -> Result<Option<DealFrame>, LapError> {
        let plan = match plan_layers(
            room.host_key.bytes(),
            deals.session.config.layer_count,
            deals.session.config.layer_memory_mib,
            candidates,
        ) {
            Ok(plan) => plan,
            Err(DealError::InsufficientCapacity) => {
                record.state = RecoveryState::Paused;
                record.replacement = None;
                record.recompute.clear();
                return Ok(None);
            }
            Err(error) => return Err(error.into()),
        };
        let price = plan
            .layers
            .iter()
            .map(|p| {
                candidates
                    .iter()
                    .find(|c| c.bid_id == p.bid_id)
                    .map(|c| c.minimum_price)
                    .ok_or(LapError::Invalid("missing recovery price"))
            })
            .collect::<Result<Vec<_>, _>>()?
            .into_iter()
            .max()
            .ok_or(LapError::Invalid("empty recovery plan"))?;
        deals.session.round = deals
            .session
            .round
            .checked_add(1)
            .ok_or(LapError::Invalid("recovery round overflow"))?;
        let mut frame = record.previous_deal.clone();
        frame.plan = plan;
        frame.round = deals.session.round;
        frame.epoch = epoch;
        frame.roster_digest = roster;
        frame.config = snapshot.binding().config.get();
        frame.price = price.to_string();
        let mut identity = Sha256::new();
        identity.update(b"PAXAI/inference-recovery-deal/v1\0");
        identity.update(record.previous_deal.deal_id);
        identity.update(frame.digest()?);
        frame.deal_id = identity.finalize().into();
        frame.digest()?;
        record.recompute = frame
            .plan
            .layers
            .iter()
            .filter(|p| {
                !record.previous_deal.plan.layers.iter().any(|old| {
                    old.peer_key == p.peer_key
                        && old.first_layer == p.first_layer
                        && old.end_layer == p.end_layer
                })
            })
            .map(|p| CacheRecompute {
                peer_key: p.peer_key,
                first_layer: p.first_layer,
                end_layer: p.end_layer,
                through_lap: record.resume.last_countersigned_lap,
            })
            .collect();
        Ok(Some(frame))
    }

    fn keep_replacement(
        record: &RecoveryRecord,
        deals: &mut DealEngine,
        snapshot: &FinalizedSnapshot,
        epoch: u64,
        roster: [u8; 32],
    ) -> bool {
        if let Some(frame) = &record.replacement {
            if deals
                .session
                .current
                .as_ref()
                .is_some_and(|n| n.frame.deal_id == frame.deal_id && n.submitted.is_some())
            {
                return true;
            }
            if deals
                .session
                .live
                .iter()
                .any(|n| n.frame.deal_id == frame.deal_id)
            {
                return true;
            }
            if deals.current().is_some_and(|f| {
                f == frame
                    && f.plan.layers.iter().all(|p| {
                        deals.peers.contains_key(&p.peer_key) && !deals.excluded(&p.peer_key)
                    })
                    && f.epoch == epoch
                    && f.roster_digest == roster
                    && f.config == snapshot.binding().config.get()
            }) {
                return true;
            }
            if deals.current().is_some_and(|f| f.deal_id == frame.deal_id) {
                deals.session.current = None;
            }
        }
        false
    }

    fn plan_pending(
        &mut self,
        deals: &mut DealEngine,
        snapshot: &FinalizedSnapshot,
        now_ms: u64,
    ) -> Result<(), LapError> {
        self.validate_source(deals, snapshot)?;
        let config = &self.journal.config;
        let shared = state::decode_shared_state(snapshot.state_bytes())?;
        let book = RoomBook::decode(shared.room_market()?.1)?;
        let room = book
            .room(Digest32::new(config.room_id)?)?
            .ok_or(LapError::Invalid("recovery room unavailable"))?;
        if room.state != market::rooms::RoomState::Open
            || room.model_digest.bytes() != config.model_digest
            || room.runtime_digest.bytes() != config.runtime_digest
        {
            return Err(LapError::Invalid("recovery room binding differs"));
        }
        let view_bytes = client::book_view(snapshot, room.model_digest)?;
        let view = BookView::decode(&view_bytes)?;
        let admission = AdmissionTable::decode(
            market::reputation_transition::split_joint(
                shared.section(Section::ReputationAdmission)?,
            )?
            .1,
        )?;
        let epoch = admission
            .current_epoch()
            .ok_or(LapError::Invalid("recovery epoch unavailable"))?;
        let roster = market::roster::roster_digest(&admission, epoch)?.bytes();
        let mut prepared = None;
        for record in &mut self.journal.records {
            if record.previous_deal.chain != snapshot.binding().chain.bytes()
                || record.previous_deal.market != snapshot.binding().market.bytes()
            {
                return Err(LapError::Invalid("recovery market domain differs"));
            }
            if record.state == RecoveryState::Live {
                continue;
            }
            if Self::keep_replacement(record, deals, snapshot, epoch, roster) {
                continue;
            }
            if deals.current().is_some() {
                continue;
            }
            let ask = book
                .ask(Digest32::new(record.previous_deal.ask_id)?)?
                .ok_or(LapError::Invalid("recovery ask unavailable"))?;
            if ask.state == market::rooms::AskState::Closed {
                return Err(LapError::Invalid("recovery ask is closed"));
            }
            let candidates =
                Self::replacement_candidates(deals, record, snapshot, &book, &room, &view)?;
            let Some(frame) = Self::prepare_replacement(
                record,
                deals,
                snapshot,
                &room,
                &candidates,
                epoch,
                roster,
            )?
            else {
                continue;
            };
            let deadline_ms = now_ms
                .checked_add(room.profile.acknowledgement_window_ms)
                .ok_or(LapError::Invalid("recovery acknowledgement overflow"))?;
            record.state = RecoveryState::Negotiating;
            record.replacement = Some(frame.clone());
            // The lineage is durable before a frame can be announced or signed for the chain.
            prepared = Some((frame, deadline_ms));
            break;
        }
        self.persist()?;
        if let Some((frame, deadline_ms)) = prepared {
            deals.session.current = Some(Negotiation {
                frame,
                acknowledgements: BTreeMap::new(),
                deadline_ms,
                submitted: None,
            });
            deals.snapshot = Some(snapshot.clone());
            deals.persist()?;
        }
        Ok(())
    }

    /// Reconciles retained losses and detects disconnects, liveness expiry and slashes.
    ///
    /// # Errors
    /// Returns an error for invalid market state or unavailable recovery journals.
    pub fn refresh(
        &mut self,
        deals: &mut DealEngine,
        laps: &mut BTreeMap<[u8; 32], LapEngine>,
        snapshot: &FinalizedSnapshot,
        now_ms: u64,
    ) -> Result<(), LapError> {
        self.validate_source(deals, snapshot)?;
        self.reconcile(deals, laps)?;
        let shared = state::decode_shared_state(snapshot.state_bytes())?;
        let book = RoomBook::decode(shared.room_market()?.1)?;
        let room = book
            .room(Digest32::new(self.journal.config.room_id)?)?
            .ok_or(LapError::Invalid("recovery room missing"))?;
        let mut losses = Vec::new();
        for negotiation in &deals.session.live {
            if self.superseded(&negotiation.frame.deal_id) {
                continue;
            }
            let Some(engine) = laps.get(&negotiation.frame.deal_id) else {
                continue;
            };
            for peer in &negotiation.frame.plan.layers {
                let timer_key = format!(
                    "{}:{}",
                    hex(&negotiation.frame.deal_id),
                    hex(&peer.peer_key)
                );
                let latest = engine
                    .session
                    .pending
                    .as_ref()
                    .or_else(|| engine.session.proofs.last());
                if let Some(proof) = latest {
                    if proof.signatures.contains_key(&hex(&peer.peer_key)) {
                        self.journal.unresponsive_since.remove(&timer_key);
                    } else {
                        let issued = proof
                            .frame
                            .deadline_ms
                            .checked_sub(engine.session.countersign_window_ms)
                            .ok_or(LapError::Invalid("invalid liveness timestamp"))?;
                        self.journal
                            .unresponsive_since
                            .entry(timer_key.clone())
                            .or_insert(issued);
                    }
                }
                let reason =
                    if market::bonds::bond_from_state(
                        snapshot.state_bytes(),
                        Digest32::new(peer.bid_id)?,
                    )?
                    .is_some_and(|b| b.slashed > 0)
                    {
                        Some(LossReason::SlashMark)
                    } else if !deals.peers.contains_key(&peer.peer_key) {
                        Some(LossReason::DataChannel)
                    } else if self.journal.unresponsive_since.get(&timer_key).is_some_and(
                        |issued| {
                            issued
                                .checked_add(room.profile.liveness_timeout_ms)
                                .is_some_and(|deadline| now_ms >= deadline)
                        },
                    ) {
                        Some(LossReason::LivenessTimeout)
                    } else {
                        None
                    };
                if let Some(reason) = reason {
                    losses.push((
                        PeerLoss {
                            deal_id: negotiation.frame.deal_id,
                            peer_key: peer.peer_key,
                        },
                        reason,
                    ));
                }
            }
        }
        for (loss, reason) in losses {
            self.replan_after_loss(deals, laps, loss, reason, snapshot, now_ms)?;
        }
        self.plan_pending(deals, snapshot, now_ms)?;
        self.persist()
    }

    /// Advances recovery and delivers finalized resume and cache recomputation handoffs.
    ///
    /// # Errors
    /// Returns an error for unavailable journals, sessions or peer connections.
    pub fn tick(
        &mut self,
        listener: &mut LapSignalListener,
        snapshot: &FinalizedSnapshot,
        now_ms: u64,
    ) -> Result<(), LapError> {
        let mut deals = listener
            .listener
            .engine
            .lock()
            .map_err(|_| LapError::Invalid("recovery deals unavailable"))?;
        let mut laps = listener
            .laps
            .lock()
            .map_err(|_| LapError::Invalid("recovery laps unavailable"))?;
        self.refresh(&mut deals, &mut laps, snapshot, now_ms)?;
        let announced = deals
            .current()
            .filter(|frame| {
                self.journal.records.iter().any(|r| {
                    r.replacement
                        .as_ref()
                        .is_some_and(|f| f.deal_id == frame.deal_id)
                })
            })
            .cloned();
        let mut handoffs = Vec::new();
        let mut activated = BTreeSet::new();
        for record in &self.journal.records {
            let Some(frame) = &record.replacement else {
                continue;
            };
            if record.state == RecoveryState::Live || !deals.live(&frame.deal_id)? {
                continue;
            }
            let Some(engine) = laps.get_mut(&frame.deal_id) else {
                continue;
            };
            record.resume_lap_engine(&deals, engine)?;
            for split in &frame.plan.layers {
                handoffs.push((
                    split.peer_key,
                    RecoveryHandoff {
                        previous_deal_id: record.previous_deal.deal_id,
                        deal: frame.clone(),
                        resume: record.resume.clone(),
                        recompute: record
                            .recompute
                            .iter()
                            .filter(|r| r.peer_key == split.peer_key)
                            .cloned()
                            .collect(),
                    },
                ));
            }
            activated.insert(frame.deal_id);
        }
        drop(laps);
        drop(deals);
        if let Some(frame) = announced {
            let digest = frame.digest()?;
            if listener.listener.announced != Some(digest) {
                for split in &frame.plan.layers {
                    listener.listener.send_to(
                        &split.peer_key,
                        &DealServerFrame::Deal {
                            deal: frame.clone(),
                        },
                    )?;
                }
                listener.listener.announced = Some(digest);
            }
        }
        for (key, handoff) in handoffs {
            Self::send_handoff(listener, &key, &handoff)?;
        }
        for record in &mut self.journal.records {
            if record
                .replacement
                .as_ref()
                .is_some_and(|f| activated.contains(&f.deal_id))
            {
                record.state = RecoveryState::Live;
            }
        }
        self.persist()
    }

    fn send_handoff(
        listener: &LapSignalListener,
        key: &[u8; 32],
        handoff: &RecoveryHandoff,
    ) -> Result<(), LapError> {
        let writer = {
            let connections = listener
                .listener
                .signal
                .shared
                .lock()
                .map_err(|_| LapError::Invalid("recovery connections unavailable"))?;
            Arc::clone(
                &connections
                    .peers
                    .get(key)
                    .ok_or(LapError::Invalid("replacement peer disconnected"))?
                    .writer,
            )
        };
        let bytes =
            serde_json::to_vec(&serde_json::json!({"type":"recovery_resume", "recovery":handoff}))
                .map_err(|_| LapError::Invalid("handoff encoding"))?;
        if bytes.len() > MAX_FRAME_BYTES {
            return Err(LapError::Invalid("handoff exceeds bound"));
        }
        let mut writer = writer
            .lock()
            .map_err(|_| LapError::Invalid("recovery writer unavailable"))?;
        write_frame(&mut writer, 1, &bytes)?;
        Ok(())
    }

    /// Handles a data channel loss reported through the authenticated host route.
    ///
    /// # Errors
    /// Returns an error for an unavailable session or an invalid loss report.
    pub fn notify_loss(
        &mut self,
        listener: &LapSignalListener,
        loss: PeerLoss,
        now_ms: u64,
    ) -> Result<(), LapError> {
        let mut deals = listener
            .listener
            .engine
            .lock()
            .map_err(|_| LapError::Invalid("recovery deals unavailable"))?;
        let snapshot = deals
            .snapshot
            .clone()
            .ok_or(LapError::Invalid("recovery snapshot unavailable"))?;
        let mut laps = listener
            .laps
            .lock()
            .map_err(|_| LapError::Invalid("recovery laps unavailable"))?;
        self.replan_after_loss(
            &mut deals,
            &mut laps,
            loss,
            LossReason::DataChannel,
            &snapshot,
            now_ms,
        )
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct RecoveryHandoff {
    pub previous_deal_id: [u8; 32],
    pub deal: DealFrame,
    pub resume: ResumePoint,
    pub recompute: Vec<CacheRecompute>,
}

impl RecoveryRecord {
    /// Restores generation counters only after the replacement deal is finalized.
    ///
    /// # Errors
    /// Returns an error if finality, stream, deal or retained counters disagree.
    pub fn resume_lap_engine(
        &self,
        deals: &DealEngine,
        engine: &mut LapEngine,
    ) -> Result<(), LapError> {
        let frame = self
            .replacement
            .as_ref()
            .ok_or(LapError::Invalid("no replacement to resume"))?;
        if !deals.live(&frame.deal_id)?
            || engine.session.deal != *frame
            || engine.session.stream_id != self.stream_id
        {
            return Err(LapError::Invalid(
                "replacement is not finalized for this stream",
            ));
        }
        if engine.session.next_lap == 0
            && engine.session.receipts.is_empty()
            && engine.session.pending.is_none()
        {
            engine.session.next_lap = self.resume.next_lap_index;
            engine.session.cumulative_tokens = self.resume.cumulative_tokens;
            engine.session.generated_tokens = self.resume.generated_tokens;
            engine.persist()?;
        } else if engine.session.next_lap < self.resume.next_lap_index
            || engine.session.cumulative_tokens < self.resume.cumulative_tokens
        {
            return Err(LapError::Invalid(
                "replacement cursor precedes recovery point",
            ));
        }
        Ok(())
    }
}
