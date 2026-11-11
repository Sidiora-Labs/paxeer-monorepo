use super::*;
use std::fs::{self, OpenOptions};
use std::os::unix::fs::OpenOptionsExt as _;
use std::path::{Path, PathBuf};

use layerx_client::ai_market::market_program as market;
use layerx_client::ai_market::{
    inference_market::{self as client, BookView},
    market_envelope, submit_market_operation, AiClientError, FinalizedSnapshot, OperationJournal,
    OperationRecord, OperationState,
};
use layerx_client::sequencer::SequencerEndpoint;
use market::{
    admission::{AdmissionTable, Participant},
    rooms::{LayerSplit, RecordDealRequest, RoomBook, RoomRecord, MAX_DEAL_PEERS},
    state::{self, Section},
    types::{Digest32, RequestId, RosterDigest},
    workers::{WorkerState, WorkerTable},
};

#[derive(Debug)]
pub enum DealError {
    Client(AiClientError),
    Signal(SignalError),
    InsufficientCapacity,
    Invalid(&'static str),
    Io(io::Error),
}
impl From<AiClientError> for DealError {
    fn from(error: AiClientError) -> Self {
        Self::Client(error)
    }
}
impl From<market::errors::ApplicationError> for DealError {
    fn from(error: market::errors::ApplicationError) -> Self {
        Self::Client(error.into())
    }
}
impl From<io::Error> for DealError {
    fn from(error: io::Error) -> Self {
        Self::Io(error)
    }
}
impl std::fmt::Display for DealError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Client(error) => write!(f, "deal negotiation refused: Client({error:?})"),
            Self::Signal(error) => write!(f, "deal negotiation refused: Signal({error:?})"),
            Self::Io(error) => write!(f, "deal negotiation refused: Io({error:?})"),
            Self::Invalid(error) => write!(f, "deal negotiation refused: Invalid({error:?})"),
            Self::InsufficientCapacity => {
                write!(f, "deal negotiation refused: InsufficientCapacity")
            }
        }
    }
}
impl std::error::Error for DealError {}

#[derive(Clone, Debug)]
pub struct PlanPeer {
    pub peer_key: [u8; 32],
    pub owner: [u8; 32],
    pub bid_id: [u8; 32],
    pub manifest_digest: [u8; 32],
    pub accelerator_memory_mib: u32,
    pub layer_capacity: u16,
    pub minimum_price: u128,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct PlannedLayers {
    pub peer_key: [u8; 32],
    pub owner: [u8; 32],
    pub bid_id: [u8; 32],
    pub manifest_digest: [u8; 32],
    pub first_layer: u16,
    pub end_layer: u16,
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct LayerPlan {
    pub embedding_host: [u8; 32],
    pub head_host: [u8; 32],
    pub layer_count: u16,
    pub layers: Vec<PlannedLayers>,
}
impl LayerPlan {
    pub fn validate(&self) -> Result<(), DealError> {
        if self.embedding_host == [0; 32]
            || self.embedding_host != self.head_host
            || self.layer_count == 0
            || self.layers.is_empty()
            || self.layers.len() > MAX_DEAL_PEERS
        {
            return Err(DealError::Invalid("invalid host or layer count"));
        }
        let mut end = 0;
        let mut peers = BTreeSet::new();
        let mut owners = BTreeSet::new();
        let mut bids = BTreeSet::new();
        for split in &self.layers {
            if split.first_layer != end
                || split.end_layer <= end
                || split.end_layer > self.layer_count
                || split.owner == self.embedding_host
                || split.owner == [0; 32]
                || split.peer_key == [0; 32]
                || split.bid_id == [0; 32]
                || split.manifest_digest == [0; 32]
                || !peers.insert(split.peer_key)
                || !owners.insert(split.owner)
                || !bids.insert(split.bid_id)
            {
                return Err(DealError::Invalid(
                    "layer plan is not a unique contiguous partition",
                ));
            }
            end = split.end_layer;
        }
        if end != self.layer_count {
            return Err(DealError::Invalid("layer plan is incomplete"));
        }
        Ok(())
    }
}

pub fn plan_layers(
    host: [u8; 32],
    layer_count: u16,
    layer_memory_mib: u32,
    peers: &[PlanPeer],
) -> Result<LayerPlan, DealError> {
    if host == [0; 32] || layer_count == 0 || layer_memory_mib == 0 || peers.len() > 64 {
        return Err(DealError::Invalid("invalid model geometry"));
    }
    let mut peers = peers.to_vec();
    peers.sort_by(|a, b| {
        b.accelerator_memory_mib
            .cmp(&a.accelerator_memory_mib)
            .then_with(|| a.peer_key.cmp(&b.peer_key))
            .then_with(|| a.bid_id.cmp(&b.bid_id))
    });
    let mut plan = LayerPlan {
        embedding_host: host,
        head_host: host,
        layer_count,
        layers: Vec::new(),
    };
    let mut end = 0_u16;
    let mut used = BTreeSet::new();
    for peer in peers {
        if peer.owner == host || !used.insert(peer.owner) {
            continue;
        }
        let capacity = u16::try_from(
            u32::from(peer.layer_capacity)
                .min(peer.accelerator_memory_mib / layer_memory_mib)
                .min(u32::from(layer_count - end)),
        )
        .map_err(|_| DealError::Invalid("invalid model geometry"))?;
        if capacity == 0 {
            continue;
        }
        plan.layers.push(PlannedLayers {
            peer_key: peer.peer_key,
            owner: peer.owner,
            bid_id: peer.bid_id,
            manifest_digest: peer.manifest_digest,
            first_layer: end,
            end_layer: end + capacity,
        });
        end += capacity;
        if end == layer_count {
            break;
        }
        if plan.layers.len() == MAX_DEAL_PEERS {
            break;
        }
    }
    if end != layer_count {
        return Err(DealError::InsufficientCapacity);
    }
    plan.validate()?;
    Ok(plan)
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct DealFrame {
    pub chain: [u8; 32],
    pub program: [u8; 32],
    pub market: [u8; 32],
    pub room_id: [u8; 32],
    pub model_digest: [u8; 32],
    pub runtime_digest: [u8; 32],
    pub deal_id: [u8; 32],
    pub ask_id: [u8; 32],
    pub roster_digest: [u8; 32],
    pub epoch: u64,
    pub config: u64,
    pub round: u64,
    pub price: String,
    pub plan: LayerPlan,
}
impl DealFrame {
    #[cfg(test)]
    pub fn verify_recorded(
        &self,
        endpoint: &SequencerEndpoint,
        snapshot: &FinalizedSnapshot,
    ) -> Result<(), DealError> {
        if !DealEngine::record_matches(snapshot, self)? {
            return Err(DealError::Invalid("deal has not reached finality"));
        }
        client::verify_recorded_deal(endpoint, &self.request()?)?;
        Ok(())
    }

    pub fn request(&self) -> Result<RecordDealRequest, DealError> {
        self.plan.validate()?;
        let price = self
            .price
            .parse::<u128>()
            .map_err(|_| DealError::Invalid("invalid deal price"))?;
        if price == 0 || price.to_string() != self.price {
            return Err(DealError::Invalid("noncanonical deal price"));
        }
        let mut layers = [None; MAX_DEAL_PEERS];
        for (out, split) in layers.iter_mut().zip(&self.plan.layers) {
            *out = Some(LayerSplit {
                bid_id: Digest32::new(split.bid_id)?,
                first_layer: split.first_layer,
                end_layer: split.end_layer,
            });
        }
        Ok(RecordDealRequest {
            room_id: Digest32::new(self.room_id)?,
            deal_id: Digest32::new(self.deal_id)?,
            ask_id: Digest32::new(self.ask_id)?,
            agreed_price_per_thousand_tokens: price,
            roster_digest: RosterDigest::new(self.roster_digest)?,
            layers,
        })
    }

    pub fn digest(&self) -> Result<[u8; 32], DealError> {
        let request = self.request()?;
        let mut hash = Sha256::new();
        hash.update(b"PAXAI/inference-deal/v1\0");
        for value in [
            self.chain,
            self.program,
            self.market,
            self.room_id,
            self.model_digest,
            self.runtime_digest,
            self.deal_id,
            self.ask_id,
            self.roster_digest,
        ] {
            hash.update(value);
        }
        for value in [self.epoch, self.config, self.round] {
            hash.update(value.to_be_bytes());
        }
        hash.update(request.agreed_price_per_thousand_tokens.to_be_bytes());
        hash.update(self.plan.embedding_host);
        hash.update(self.plan.head_host);
        hash.update(self.plan.layer_count.to_be_bytes());
        let layer_count = u16::try_from(self.plan.layers.len())
            .map_err(|_| DealError::Invalid("invalid host or layer count"))?;
        hash.update(layer_count.to_be_bytes());
        for split in &self.plan.layers {
            for value in [
                split.peer_key,
                split.owner,
                split.bid_id,
                split.manifest_digest,
            ] {
                hash.update(value);
            }
            hash.update(split.first_layer.to_be_bytes());
            hash.update(split.end_layer.to_be_bytes());
        }
        Ok(hash.finalize().into())
    }

    #[cfg(test)]
    pub fn acknowledgement(
        &self,
        peer_key: &[u8; 32],
        signature: [u8; 64],
    ) -> Result<DealAcknowledgement, DealError> {
        if !self
            .plan
            .layers
            .iter()
            .any(|split| &split.peer_key == peer_key)
        {
            return Err(DealError::Invalid("peer is not assigned to the deal"));
        }
        layerx_crypto::ed25519::verify_message(
            peer_key,
            &signature,
            &self.acknowledgement_bytes()?,
        )
        .map_err(|_| DealError::Signal(SignalError::BadSignature))?;
        Ok(DealAcknowledgement::DealAck {
            deal_id: self.deal_id,
            digest: self.digest()?,
            signature: signature.to_vec(),
        })
    }

    pub fn acknowledgement_bytes(&self) -> Result<Vec<u8>, DealError> {
        let mut bytes = b"PAXAI/inference-deal-ack/v1\0".to_vec();
        bytes.extend_from_slice(&self.digest()?);
        Ok(bytes)
    }
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
pub enum DealAcknowledgement {
    DealAck {
        deal_id: [u8; 32],
        digest: [u8; 32],
        signature: Vec<u8>,
    },
}

#[derive(Clone, Debug, Deserialize, Serialize)]
#[serde(tag = "type", rename_all = "snake_case", deny_unknown_fields)]
#[allow(
    clippy::large_enum_variant,
    reason = "large_enum_variant: tests/inference_deals.rs constructs DealServerFrame::Deal with an unboxed DealFrame"
)]
pub enum DealServerFrame {
    Deal { deal: DealFrame },
    DealLive { deal_id: [u8; 32] },
}

#[derive(Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct Negotiation {
    frame: DealFrame,
    acknowledgements: BTreeMap<String, Vec<u8>>,
    deadline_ms: u64,
    submitted: Option<[u8; 32]>,
}
impl Negotiation {
    fn ready(&self) -> bool {
        self.frame
            .plan
            .layers
            .iter()
            .all(|split| self.acknowledgements.contains_key(&hex(&split.peer_key)))
    }
    fn check_signature(&self, key: &[u8; 32], signature: &[u8]) -> Result<(), DealError> {
        if !self
            .frame
            .plan
            .layers
            .iter()
            .any(|split| &split.peer_key == key)
        {
            return Err(DealError::Invalid(
                "acknowledgement is not from a participant",
            ));
        }
        let signature: [u8; 64] = signature
            .try_into()
            .map_err(|_| DealError::Invalid("invalid acknowledgement signature length"))?;
        layerx_crypto::ed25519::verify_message(
            key,
            &signature,
            &self.frame.acknowledgement_bytes()?,
        )
        .map_err(|_| DealError::Signal(SignalError::BadSignature))
    }
}

#[derive(Clone, Debug, Deserialize, Eq, PartialEq, Serialize)]
#[serde(deny_unknown_fields)]
pub struct DealConfig {
    pub room_id: [u8; 32],
    pub model_digest: [u8; 32],
    pub runtime_digest: [u8; 32],
    pub program: [u8; 32],
    pub layer_count: u16,
    pub layer_memory_mib: u32,
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct Session {
    config: DealConfig,
    round: u64,
    current: Option<Negotiation>,
    excluded: BTreeSet<[u8; 32]>,
    live: Vec<Negotiation>,
}
struct PeerManifest {
    connection: u64,
    bytes: Vec<u8>,
}

pub struct DealEngine {
    session: Session,
    path: PathBuf,
    peers: BTreeMap<[u8; 32], PeerManifest>,
    snapshot: Option<FinalizedSnapshot>,
}
impl DealEngine {
    pub fn open(path: &Path, config: DealConfig) -> Result<Self, DealError> {
        if !path.is_absolute()
            || config.layer_count == 0
            || config.layer_memory_mib == 0
            || config.room_id == [0; 32]
            || config.program == [0; 32]
            || config.model_digest == [0; 32]
            || config.runtime_digest == [0; 32]
        {
            return Err(DealError::Invalid("invalid negotiation configuration"));
        }
        let session = match fs::symlink_metadata(path) {
            Ok(_) => {
                let bytes = layerx_agentd::config::read_protected_source(path, 1_048_576)
                    .map_err(|_| DealError::Invalid("invalid deal session journal"))?;
                let session: Session = serde_json::from_slice(&bytes)
                    .map_err(|_| DealError::Invalid("corrupt deal session"))?;
                if session.config != config || session.live.len() > market::rooms::DEAL_CAPACITY {
                    return Err(DealError::Invalid("deal session belongs to another room"));
                }
                for negotiation in session.current.iter().chain(&session.live) {
                    negotiation.frame.digest()?;
                    if negotiation.frame.room_id != config.room_id
                        || negotiation.frame.program != config.program
                        || negotiation.frame.model_digest != config.model_digest
                        || negotiation.frame.runtime_digest != config.runtime_digest
                        || negotiation.frame.plan.layer_count != config.layer_count
                        || negotiation.frame.round > session.round
                    {
                        return Err(DealError::Invalid("deal session binding differs"));
                    }
                    for (key, signature) in &negotiation.acknowledgements {
                        let key: [u8; 32] = layerx_programs::hex::decode(key)
                            .map_err(|_| DealError::Invalid("invalid acknowledgement key"))?
                            .try_into()
                            .map_err(|_| DealError::Invalid("invalid acknowledgement key"))?;
                        negotiation.check_signature(&key, signature)?;
                    }
                    if negotiation.submitted.is_some() && !negotiation.ready() {
                        return Err(DealError::Invalid(
                            "submitted deal is missing acknowledgements",
                        ));
                    }
                }
                session
            }
            Err(error) if error.kind() == io::ErrorKind::NotFound => Session {
                config,
                round: 0,
                current: None,
                excluded: BTreeSet::new(),
                live: Vec::new(),
            },
            Err(error) => return Err(error.into()),
        };
        let engine = Self {
            session,
            path: path.to_owned(),
            peers: BTreeMap::new(),
            snapshot: None,
        };
        engine.persist()?;
        Ok(engine)
    }

    fn persist(&self) -> Result<(), DealError> {
        let bytes = serde_json::to_vec(&self.session)
            .map_err(|_| DealError::Invalid("deal session encoding failed"))?;
        if bytes.len() > 1_048_576 {
            return Err(DealError::Invalid("deal session exceeded its bound"));
        }
        let mut nonce = [0; 16];
        getrandom::fill(&mut nonce)
            .map_err(|_| DealError::Invalid("deal journal entropy is unavailable"))?;
        let temporary = self.path.with_extension(format!("{}.tmp", hex(&nonce)));
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
                    .ok_or(DealError::Invalid("deal journal has no parent"))?,
            )?
            .sync_all()?;
            Ok(())
        })();
        if result.is_err() {
            let _ = fs::remove_file(&temporary);
        }
        result
    }

    fn admitted_peer(
        &self,
        snapshot: &FinalizedSnapshot,
        key: &[u8; 32],
        bytes: &[u8],
    ) -> Result<(), DealError> {
        let summary = workers::decode_manifest(bytes)?;
        let shared = state::decode_shared_state(snapshot.state_bytes())?;
        let admission = AdmissionTable::decode(
            market::reputation_transition::split_joint(
                shared.section(Section::ReputationAdmission)?,
            )?
            .1,
        )?;
        let epoch = admission
            .current_epoch()
            .ok_or(DealError::Invalid("admission epoch is unavailable"))?;
        let workers = WorkerTable::decode(
            market::evaluators::authority::split_identity_section(
                shared.section(Section::IdentityRoster)?,
            )?
            .0,
        )?;
        let worker = workers
            .get(summary.worker)
            .ok_or(DealError::Invalid("peer is not enrolled"))?;
        let member = admission
            .get(Participant::Worker(summary.worker))
            .ok_or(DealError::Invalid("peer is not admitted"))?;
        let height = snapshot.binding().execution_height;
        if summary.version != 2
            || summary.market != snapshot.binding().market
            || summary.capabilities.is_none()
            || !matches!(worker.state, WorkerState::Enrolled | WorkerState::Available)
            || worker.delegate.0 != *key
            || worker.owner != summary.owner
            || worker.generation != summary.generation
            || worker.key_version != summary.key_version
            || worker.metadata != summary.digest
            || worker.metadata_revision != summary.revision
            || worker.valid_from != summary.valid_from
            || worker.expiry != summary.expiry
            || worker.effective_epoch > epoch
            || height < summary.valid_from
            || height >= summary.expiry
            || !member.admitted()
            || member.admitted_epoch.is_none()
            || member.revoked()
            || member.draining()
            || member.owner != summary.owner
            || member.delegate_generation != summary.key_version
            || bytes.get(138..170) != Some(self.session.config.runtime_digest.as_slice())
            || bytes.get(174..206) != Some(self.session.config.model_digest.as_slice())
        {
            return Err(DealError::Invalid(
                "peer manifest or admission binding differs",
            ));
        }
        Ok(())
    }

    pub fn admit(
        &mut self,
        connection: u64,
        hello: &HelloFrame,
        challenge: &[u8; 32],
    ) -> Result<(), DealError> {
        hello
            .verify(challenge, &self.session.config.room_id)
            .map_err(DealError::Signal)?;
        if self.session.excluded.contains(&hello.peer_key)
            || self.peers.contains_key(&hello.peer_key)
            || self.peers.len() == MAX_CONNECTIONS
        {
            return Err(DealError::Invalid(
                "peer is duplicate, dropped or over capacity",
            ));
        }
        let snapshot = self
            .snapshot
            .as_ref()
            .ok_or(DealError::Invalid("committed book is unavailable"))?;
        self.admitted_peer(snapshot, &hello.peer_key, &hello.manifest)?;
        self.peers.insert(
            hello.peer_key,
            PeerManifest {
                connection,
                bytes: hello.manifest.clone(),
            },
        );
        Ok(())
    }

    fn disconnect(&mut self, key: &[u8; 32], connection: u64) {
        if self
            .peers
            .get(key)
            .is_some_and(|peer| peer.connection == connection)
        {
            self.peers.remove(key);
        }
    }

    pub fn current(&self) -> Option<&DealFrame> {
        self.session.current.as_ref().map(|n| &n.frame)
    }
    pub fn ready_to_record(&self) -> Option<&DealFrame> {
        self.session
            .current
            .as_ref()
            .filter(|n| n.ready() && n.submitted.is_none())
            .map(|n| &n.frame)
    }
    pub fn live(&self, deal: &[u8; 32]) -> Result<bool, DealError> {
        let Some(snapshot) = &self.snapshot else {
            return Ok(false);
        };
        let Some(negotiation) = self.session.live.iter().find(|n| n.frame.deal_id == *deal) else {
            return Ok(false);
        };
        Ok(negotiation.ready() && Self::record_matches(snapshot, &negotiation.frame)?)
    }
    pub fn excluded(&self, peer: &[u8; 32]) -> bool {
        self.session.excluded.contains(peer)
    }

    pub fn acknowledge(
        &mut self,
        key: &[u8; 32],
        acknowledgement: &DealAcknowledgement,
        now_ms: u64,
    ) -> Result<(), DealError> {
        let current = self
            .session
            .current
            .as_mut()
            .ok_or(DealError::Invalid("no deal is being negotiated"))?;
        let DealAcknowledgement::DealAck {
            deal_id,
            digest,
            signature,
        } = acknowledgement;
        if current.submitted.is_some()
            || now_ms >= current.deadline_ms
            || *deal_id != current.frame.deal_id
            || *digest != current.frame.digest()?
        {
            return Err(DealError::Invalid("acknowledgement is stale or late"));
        }
        current.check_signature(key, signature)?;
        current.acknowledgements.insert(hex(key), signature.clone());
        self.persist()
    }

    fn record_matches(snapshot: &FinalizedSnapshot, frame: &DealFrame) -> Result<bool, DealError> {
        if snapshot.binding().chain.bytes() != frame.chain
            || snapshot.binding().program.bytes() != frame.program
            || snapshot.binding().market.bytes() != frame.market
        {
            return Err(DealError::Invalid("finalized deal is from another market"));
        }
        let shared = state::decode_shared_state(snapshot.state_bytes())?;
        let book = RoomBook::decode(shared.room_market()?.1)?;
        let Some(record) = book.deal(Digest32::new(frame.deal_id)?)? else {
            return Ok(false);
        };
        let request = frame.request()?;
        if record.room_id != request.room_id
            || record.ask_id != request.ask_id
            || record.layers != request.layers
            || record.roster_digest != request.roster_digest
            || record.agreed_price_per_thousand_tokens != request.agreed_price_per_thousand_tokens
        {
            return Err(DealError::Invalid(
                "finalized deal differs from acknowledged terms",
            ));
        }
        Ok(true)
    }

    pub fn poll(
        &mut self,
        snapshot: FinalizedSnapshot,
        now_ms: u64,
    ) -> Result<Vec<[u8; 32]>, DealError> {
        let config = &self.session.config;
        if snapshot.binding().program.bytes() != config.program
            || snapshot.header().deployment_chain_domain.bytes() != market::merged_chain_domain(125)
            || self.snapshot.as_ref().is_some_and(|previous| {
                snapshot.binding().execution_height < previous.binding().execution_height
            })
        {
            return Err(DealError::Invalid("book source or height differs"));
        }
        let shared = state::decode_shared_state(snapshot.state_bytes())?;
        let book = RoomBook::decode(shared.room_market()?.1)?;
        let room = book
            .room(Digest32::new(config.room_id)?)?
            .ok_or(DealError::Invalid("room is unavailable"))?;
        if room.model_digest.bytes() != config.model_digest
            || room.runtime_digest.bytes() != config.runtime_digest
        {
            return Err(DealError::Invalid("room digests differ"));
        }
        room.profile.validate()?;
        let admission = AdmissionTable::decode(
            market::reputation_transition::split_joint(
                shared.section(Section::ReputationAdmission)?,
            )?
            .1,
        )?;
        let epoch = admission
            .current_epoch()
            .ok_or(DealError::Invalid("admission epoch is unavailable"))?;
        let roster = market::roster::roster_digest(&admission, epoch)?.bytes();
        let dropped = Vec::new();
        if let Some(current) = &self.session.current {
            if Self::record_matches(&snapshot, &current.frame)? {
                if current.submitted.is_none() || !current.ready() {
                    return Err(DealError::Invalid("unacknowledged deal appeared on chain"));
                }
                let current = self
                    .session
                    .current
                    .take()
                    .ok_or(DealError::Invalid("missing current deal"))?;
                self.session.live.push(current);
            } else if current.submitted.is_some() {
                self.snapshot = Some(snapshot);
                return Ok(dropped);
            }
        }
        let view_bytes = client::book_view(&snapshot, room.model_digest)?;
        let view = BookView::decode(&view_bytes)?;
        let dropped = self.refresh_negotiation(&snapshot, &book, epoch, roster, now_ms)?;
        self.negotiate(&snapshot, &book, &room, &view, (epoch, roster), now_ms)?;
        self.snapshot = Some(snapshot);
        self.persist()?;
        Ok(dropped)
    }

    fn refresh_negotiation(
        &mut self,
        snapshot: &FinalizedSnapshot,
        book: &RoomBook<'_>,
        epoch: u64,
        roster: [u8; 32],
        now_ms: u64,
    ) -> Result<Vec<[u8; 32]>, DealError> {
        let mut dropped = Vec::new();
        if let Some(current) = &self.session.current {
            let valid = current.frame.roster_digest == roster
                && current.frame.epoch == epoch
                && current.frame.config == snapshot.binding().config.get()
                && book
                    .ask(Digest32::new(current.frame.ask_id)?)?
                    .is_some_and(|ask| ask.state == market::rooms::AskState::Open)
                && current.frame.plan.layers.iter().all(|split| {
                    self.peers.get(&split.peer_key).is_some_and(|peer| {
                        self.admitted_peer(snapshot, &split.peer_key, &peer.bytes)
                            .is_ok()
                    })
                });
            if now_ms >= current.deadline_ms && !current.ready() {
                for split in &current.frame.plan.layers {
                    if !current.acknowledgements.contains_key(&hex(&split.peer_key)) {
                        dropped.push(split.peer_key);
                        self.session.excluded.insert(split.peer_key);
                        self.peers.remove(&split.peer_key);
                    }
                }
                self.session.current = None;
            } else if !valid {
                self.session.current = None;
            }
        }
        Ok(dropped)
    }

    fn plan_candidates(
        &self,
        snapshot: &FinalizedSnapshot,
        book: &RoomBook<'_>,
        room: &RoomRecord,
        view: &BookView<'_>,
        ask_index: usize,
    ) -> Result<Vec<PlanPeer>, DealError> {
        let mut candidates = Vec::new();
        for (_, bid_index) in view
            .crossing_pairs()
            .filter(|(index, _)| *index == ask_index)
        {
            let placement = view.bid(bid_index)?;
            let bid = book
                .bid(placement.order_id)?
                .ok_or(DealError::Invalid("book bid is unavailable"))?;
            for (key, peer) in &self.peers {
                if self.session.excluded.contains(key)
                    || self.admitted_peer(snapshot, key, &peer.bytes).is_err()
                {
                    continue;
                }
                let manifest = workers::decode_manifest(&peer.bytes)?;
                if manifest.owner != bid.peer_key
                    || bid.peer_key == room.host_key
                    || bid.capability_digest.bytes() != manifest.digest.bytes()
                    || bid.bond_amount < room.profile.minimum_bond
                {
                    continue;
                }
                let capabilities = manifest
                    .capabilities
                    .ok_or(DealError::Invalid("peer has no accelerator manifest"))?;
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

    fn negotiate(
        &mut self,
        snapshot: &FinalizedSnapshot,
        book: &RoomBook<'_>,
        room: &RoomRecord,
        view: &BookView<'_>,
        (epoch, roster): (u64, [u8; 32]),
        now_ms: u64,
    ) -> Result<(), DealError> {
        if self.session.current.is_none() && room.state == market::rooms::RoomState::Open {
            let config = &self.session.config;
            let mut considered = BTreeSet::new();
            for (ask_index, _) in view.crossing_pairs() {
                let ask = view.ask(ask_index)?;
                if ask.room_id != room.room_id || !considered.insert(ask.order_id) {
                    continue;
                }
                let candidates = self.plan_candidates(snapshot, book, room, view, ask_index)?;
                let plan = match plan_layers(
                    room.host_key.bytes(),
                    config.layer_count,
                    config.layer_memory_mib,
                    &candidates,
                ) {
                    Ok(plan) => plan,
                    Err(DealError::InsufficientCapacity) => continue,
                    Err(error) => return Err(error),
                };
                let price = plan
                    .layers
                    .iter()
                    .map(|split| {
                        candidates
                            .iter()
                            .find(|peer| peer.bid_id == split.bid_id)
                            .map(|peer| peer.minimum_price)
                            .ok_or(DealError::Invalid("planned bid is unavailable"))
                    })
                    .collect::<Result<Vec<_>, _>>()?
                    .into_iter()
                    .max()
                    .ok_or(DealError::Invalid("plan has no price"))?;
                self.session.round = self
                    .session
                    .round
                    .checked_add(1)
                    .ok_or(DealError::Invalid("negotiation round overflow"))?;
                let mut identity = Sha256::new();
                identity.update(b"PAXAI/inference-deal-id/v1\0");
                identity.update(snapshot.binding().market.as_bytes());
                identity.update(room.room_id.as_bytes());
                identity.update(ask.order_id.as_bytes());
                identity.update(self.session.round.to_be_bytes());
                for split in &plan.layers {
                    identity.update(split.bid_id);
                }
                let frame = DealFrame {
                    chain: snapshot.binding().chain.bytes(),
                    program: config.program,
                    market: snapshot.binding().market.bytes(),
                    room_id: config.room_id,
                    model_digest: config.model_digest,
                    runtime_digest: config.runtime_digest,
                    deal_id: identity.finalize().into(),
                    ask_id: ask.order_id.bytes(),
                    roster_digest: roster,
                    epoch,
                    config: snapshot.binding().config.get(),
                    round: self.session.round,
                    price: price.to_string(),
                    plan,
                };
                frame.digest()?;
                let deadline_ms = now_ms
                    .checked_add(room.profile.acknowledgement_window_ms)
                    .ok_or(DealError::Invalid("acknowledgement deadline overflow"))?;
                self.session.current = Some(Negotiation {
                    frame,
                    acknowledgements: BTreeMap::new(),
                    deadline_ms,
                    submitted: None,
                });
                break;
            }
        }
        Ok(())
    }

    pub fn envelope(&self, expiry: u64) -> Result<Vec<u8>, DealError> {
        let frame = self
            .ready_to_record()
            .ok_or(DealError::Invalid("deal is not fully acknowledged"))?;
        let snapshot = self
            .snapshot
            .as_ref()
            .ok_or(DealError::Invalid("committed book is unavailable"))?;
        Ok(client::deal_envelope(
            snapshot,
            &frame.request()?,
            RequestId::new(frame.deal_id)?,
            expiry,
        )?)
    }
}

pub fn record_deal(
    engine: &mut DealEngine,
    journal: &OperationJournal,
    endpoint: &SequencerEndpoint,
    record: &mut OperationRecord,
) -> Result<OperationState, DealError> {
    let expected = engine.envelope(record.not_after())?;
    let actual = market_envelope(record)?;
    if actual != expected {
        return Err(DealError::Invalid(
            "signed RecordDeal differs from acknowledged terms",
        ));
    }
    let (call, signature) = layerx_crypto::evm_transaction::decode_signed(record.signed_bytes())
        .map_err(AiClientError::from)?;
    let sender = layerx_crypto::evm_transaction::recover_sender(&call, &signature)
        .map_err(AiClientError::from)?;
    let mut principal = [0; 32];
    principal[12..].copy_from_slice(&sender);
    if engine
        .current()
        .is_none_or(|frame| frame.plan.embedding_host != principal)
    {
        return Err(DealError::Invalid("RecordDeal is not signed by the host"));
    }
    journal.record_signed(record)?;
    engine
        .session
        .current
        .as_mut()
        .ok_or(DealError::Invalid("no current deal"))?
        .submitted = Some(record.activity_id());
    engine.persist()?;
    let state = submit_market_operation(journal, record, endpoint)?;
    if !matches!(
        state,
        OperationState::Pending
            | OperationState::Unknown
            | OperationState::Executed
            | OperationState::Finalized
    ) {
        return Err(DealError::Invalid("RecordDeal was refused"));
    }
    Ok(state)
}

pub fn resume_submission(
    engine: &DealEngine,
    journal: &OperationJournal,
    endpoint: &SequencerEndpoint,
) -> Result<(), DealError> {
    let Some(activity_id) = engine.session.current.as_ref().and_then(|n| n.submitted) else {
        return Ok(());
    };
    let registry = layerx_types::payload::ModuleRegistry::new(&[]).map_err(AiClientError::from)?;
    let mut record = journal.load(&activity_id, &registry)?;
    let current = engine
        .session
        .current
        .as_ref()
        .ok_or(DealError::Invalid("submitted deal is unavailable"))?;
    let bytes = market_envelope(&record)?;
    let envelope = market::codec::decode_envelope(&bytes)?.envelope;
    let operation = client::record_deal(&current.frame.request()?)?;
    if envelope.operation != market::dispatch::RecordDeal
        || envelope.payload != operation.payload()
        || envelope.request.bytes() != current.frame.deal_id
        || envelope.actor.bytes() != current.frame.plan.embedding_host
        || envelope.chain.bytes() != current.frame.chain
        || envelope.program.bytes() != current.frame.program
        || envelope.market.bytes() != current.frame.market
        || envelope.epoch != current.frame.epoch
        || envelope.config != current.frame.config
        || envelope.roster
            != market::types::Presence::Present(RosterDigest::new(current.frame.roster_digest)?)
    {
        return Err(DealError::Invalid(
            "retained submission differs from acknowledged terms",
        ));
    }
    if matches!(record.state(), OperationState::Signed) {
        let height = engine
            .snapshot
            .as_ref()
            .ok_or(DealError::Invalid("committed book is unavailable"))?
            .binding()
            .execution_height;
        if height > record.not_after() {
            return Err(DealError::Invalid("RecordDeal expired before delivery"));
        }
        submit_market_operation(journal, &mut record, endpoint)?;
    }
    Ok(())
}

pub struct DealSignalListener {
    signal: SignalListener,
    engine: Arc<Mutex<DealEngine>>,
    announced: Option<[u8; 32]>,
    live_announced: BTreeSet<[u8; 32]>,
}
impl DealSignalListener {
    pub fn check(&self) -> Result<(), String> {
        self.signal.check()
    }

    pub fn tick(&mut self, snapshot: FinalizedSnapshot, now_ms: u64) -> Result<(), DealError> {
        let (dropped, current, live) = {
            let mut engine = self
                .engine
                .lock()
                .map_err(|_| DealError::Invalid("deal engine unavailable"))?;
            let dropped = engine.poll(snapshot, now_ms)?;
            (
                dropped,
                engine.current().cloned(),
                engine
                    .session
                    .live
                    .iter()
                    .map(|n| n.frame.clone())
                    .collect::<Vec<_>>(),
            )
        };
        {
            let connections = self
                .signal
                .shared
                .lock()
                .map_err(|_| DealError::Invalid("signal connections unavailable"))?;
            for key in dropped {
                if let Some(peer) = connections.peers.get(&key) {
                    if let Some(socket) = connections.sockets.get(&peer.connection) {
                        let _ = socket.shutdown(Shutdown::Both);
                    }
                }
            }
        }
        if let Some(frame) = current {
            let digest = frame.digest()?;
            if self.announced != Some(digest) {
                for split in &frame.plan.layers {
                    self.send_to(
                        &split.peer_key,
                        &DealServerFrame::Deal {
                            deal: frame.clone(),
                        },
                    )?;
                }
                self.announced = Some(digest);
            }
        } else {
            self.announced = None;
        }
        for frame in live {
            if !self.live_announced.contains(&frame.deal_id) {
                for split in &frame.plan.layers {
                    self.send_to(
                        &split.peer_key,
                        &DealServerFrame::DealLive {
                            deal_id: frame.deal_id,
                        },
                    )?;
                }
                self.live_announced.insert(frame.deal_id);
            }
        }
        Ok(())
    }
    fn send_to(&self, key: &[u8; 32], frame: &DealServerFrame) -> Result<(), DealError> {
        let writer = {
            let connections = self
                .signal
                .shared
                .lock()
                .map_err(|_| DealError::Invalid("signal connections unavailable"))?;
            connections
                .peers
                .get(key)
                .map(|peer| Arc::clone(&peer.writer))
        };
        if let Some(writer) = writer {
            send_deal(&writer, frame).map_err(DealError::Signal)?;
        }
        Ok(())
    }
}

pub fn send_deal(
    writer: &Arc<Mutex<TcpStream>>,
    frame: &DealServerFrame,
) -> Result<(), SignalError> {
    let bytes = serde_json::to_vec(frame).map_err(|_| SignalError::InvalidFrame)?;
    if bytes.len() > MAX_FRAME_BYTES {
        return Err(SignalError::FrameTooLarge);
    }
    let mut writer = writer.lock().map_err(|_| SignalError::Backpressure)?;
    write_frame(&mut writer, 1, &bytes).map_err(|_| SignalError::Backpressure)
}

struct AdmittedConnection<'a> {
    engine: &'a Arc<Mutex<DealEngine>>,
    key: [u8; 32],
    connection: u64,
}
impl Drop for AdmittedConnection<'_> {
    fn drop(&mut self) {
        if let Ok(mut engine) = self.engine.lock() {
            engine.disconnect(&self.key, self.connection);
        }
    }
}

#[derive(Deserialize)]
#[serde(untagged)]
pub enum DealClientFrame {
    Acknowledgement(DealAcknowledgement),
    Signal(ClientFrame),
}

#[cfg(test)]
pub fn receive_deal(
    stream: &mut TcpStream,
    writer: &Arc<Mutex<TcpStream>>,
    fragments: &mut Option<Vec<u8>>,
) -> Result<DealClientFrame, SignalError> {
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
