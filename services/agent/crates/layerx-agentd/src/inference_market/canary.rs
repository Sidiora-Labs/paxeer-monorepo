use layerx_client::ai_market::{
    inference_market::{self as client, HostMeterKey, MarketOperation},
    market_program as market, AiClientError,
};
use market::{
    canary::{self, CanaryLogits, CanaryReveal, CanaryRoomProfile, CanaryTask, ChallengeStatus},
    codec::Reader,
    policy::{RoomProfile, ROOM_PROFILE_BYTES},
    registry_ops::PolicySection,
    rooms::RoomBook,
    state::{decode_shared_state, Section},
    types::{Digest32, PrincipalId, Signature64},
};
use serde::{Deserialize, Serialize};
use sha2::{Digest as _, Sha256};
use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::os::unix::fs::OpenOptionsExt as _;
use std::path::{Path, PathBuf};
use std::sync::Arc;

#[derive(Debug)]
pub enum CanaryError {
    Invalid(&'static str),
    Client(AiClientError),
    Io(std::io::Error),
}
impl From<AiClientError> for CanaryError {
    fn from(value: AiClientError) -> Self {
        Self::Client(value)
    }
}
impl From<market::errors::ApplicationError> for CanaryError {
    fn from(value: market::errors::ApplicationError) -> Self {
        Self::Client(value.into())
    }
}
impl From<std::io::Error> for CanaryError {
    fn from(value: std::io::Error) -> Self {
        Self::Io(value)
    }
}
impl std::fmt::Display for CanaryError {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        match self {
            Self::Invalid(error) => write!(f, "canary processing refused: Invalid({error:?})"),
            Self::Client(error) => write!(f, "canary processing refused: Client({error:?})"),
            Self::Io(error) => write!(f, "canary processing refused: Io({error:?})"),
        }
    }
}
impl std::error::Error for CanaryError {}

#[derive(Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct CanaryExample {
    pub prompt_tokens: Vec<u32>,
    pub expected_logits: CanaryLogits,
}
#[derive(Clone, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
pub struct CanaryTable {
    pub model_digest: [u8; 32],
    pub entries: Vec<CanaryExample>,
}
impl CanaryTable {
    fn validate(&self) -> Result<(), CanaryError> {
        if self.model_digest == [0; 32]
            || self.entries.is_empty()
            || self.entries.len() > 256
            || self
                .entries
                .iter()
                .any(|entry| entry.prompt_tokens.is_empty() || entry.prompt_tokens.len() > 4096)
        {
            return Err(CanaryError::Invalid("invalid model canary table"));
        }
        Ok(())
    }
}

// Each selected lap carries one probe. Rejection sampling keeps both the lap
// selection (room ratio / 10000) and the position uniform, without modulo bias.
fn sample(key: &[u8; 32], domain: &[u8], bound: u64) -> u64 {
    let ceiling = u64::MAX - u64::MAX % bound;
    for counter in 0_u64.. {
        let mut hash = Sha256::new();
        hash.update(b"PAXAI/canary-position/v1\0");
        hash.update(key);
        hash.update(domain);
        hash.update(counter.to_be_bytes());
        let digest = hash.finalize();
        let mut bytes = [0; 8];
        bytes.copy_from_slice(&digest[..8]);
        let value = u64::from_be_bytes(bytes);
        if value < ceiling {
            return value % bound;
        }
    }
    unreachable!("sampling counter exhausted")
}
pub fn canary_positions(
    key: &[u8; 32],
    tokens: u64,
    ratio_bps: u16,
) -> Result<Vec<u64>, CanaryError> {
    if tokens == 0 || ratio_bps > 10_000 {
        return Err(CanaryError::Invalid("invalid canary sampling bounds"));
    }
    Ok(if sample(key, b"lap", 10_000) < u64::from(ratio_bps) {
        vec![sample(key, b"position", tokens)]
    } else {
        Vec::new()
    })
}

#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
pub struct CanaryLapRequest {
    pub deal_id: [u8; 32],
    pub lap_index: u64,
    pub generated_tokens: u64,
}

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CanaryPlan {
    pub deal_id: [u8; 32],
    pub lap_index: u64,
    pub generated_tokens: u64,
    pub position: Option<u64>,
    pub prompt_tokens: Vec<u32>,
    pub peer_key: Option<[u8; 32]>,
    pub answer_context: Vec<u8>,
}
impl CanaryPlan {
    #[cfg(test)]
    pub fn inject(&self, lap: &mut Vec<Vec<u32>>) -> Result<(), CanaryError> {
        if lap.len() as u64 != self.generated_tokens {
            return Err(CanaryError::Invalid("canary lap length differs"));
        }
        if let Some(position) = self.position {
            let position = usize::try_from(position)
                .map_err(|_| CanaryError::Invalid("canary position overflow"))?;
            if position >= lap.len() || self.prompt_tokens.is_empty() {
                return Err(CanaryError::Invalid("invalid canary position"));
            }
            lap.insert(position, self.prompt_tokens.clone());
        }
        Ok(())
    }
}

#[derive(Clone, Debug, Deserialize, Serialize, Eq, PartialEq)]
#[serde(deny_unknown_fields)]
pub struct CanaryAnswer {
    pub deal_id: [u8; 32],
    pub lap_index: u64,
    pub logits: CanaryLogits,
    pub signature: Vec<u8>,
}
impl CanaryAnswer {
    #[cfg(test)]
    pub fn sign(
        plan: &CanaryPlan,
        logits: CanaryLogits,
        peer: &HostMeterKey,
    ) -> Result<Self, CanaryError> {
        if plan.peer_key != Some(peer.public_key()) {
            return Err(CanaryError::Invalid("wrong canary peer"));
        }
        Ok(Self {
            deal_id: plan.deal_id,
            lap_index: plan.lap_index,
            logits,
            signature: peer
                .sign(client::canary_answer_attestation(&plan.answer_context, &logits)?.as_bytes())
                .to_vec(),
        })
    }
}
#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct PendingCanary {
    plan: CanaryPlan,
    model_digest: [u8; 32],
    profile: Vec<u8>,
    expected: CanaryLogits,
    task: Option<Vec<u8>>,
    answer: Option<CanaryAnswer>,
    issued: bool,
    audited: bool,
    completed: bool,
}

pub struct CanaryView {
    pub state: Vec<u8>,
    pub height: u64,
}
pub trait CanaryTransport {
    fn view(&mut self) -> Result<CanaryView, CanaryError>;
    fn submit(&mut self, operation: &MarketOperation) -> Result<(), CanaryError>;
}

pub struct CanaryEngine {
    directory: PathBuf,
    host: Arc<HostMeterKey>,
    table: CanaryTable,
}
pub struct CanaryRuntime {
    pub engine: Arc<CanaryEngine>,
    pub state: Vec<u8>,
}
pub type CanarySessions = Arc<std::sync::Mutex<Option<CanaryRuntime>>>;

pub const DEFAULT_CANARY_ANSWER_TIMEOUT_MS: u64 = 5000;
pub fn default_canary_answer_timeout_ms() -> u64 {
    DEFAULT_CANARY_ANSWER_TIMEOUT_MS
}

impl CanaryEngine {
    pub fn open(
        directory: &Path,
        host: Arc<HostMeterKey>,
        table: CanaryTable,
    ) -> Result<Self, CanaryError> {
        table.validate()?;
        if !directory.is_absolute() || !directory.is_dir() {
            return Err(CanaryError::Invalid("canary journal directory unavailable"));
        }
        Ok(Self {
            directory: directory.to_path_buf(),
            host,
            table,
        })
    }
    fn path(&self, deal: [u8; 32], lap: u64) -> PathBuf {
        let deal = layerx_programs::hex::encode(&deal);
        self.directory.join(format!("canary-{deal}-{lap}.json"))
    }
    fn load(&self, deal: [u8; 32], lap: u64) -> Result<PendingCanary, CanaryError> {
        let bytes = fs::read(self.path(deal, lap))?;
        if bytes.len() > 131_072 {
            return Err(CanaryError::Invalid("oversized canary journal"));
        }
        let pending: PendingCanary = serde_json::from_slice(&bytes)
            .map_err(|_| CanaryError::Invalid("corrupt canary journal"))?;
        if pending.plan.deal_id != deal
            || pending.plan.lap_index != lap
            || pending.model_digest != self.table.model_digest
        {
            return Err(CanaryError::Invalid("canary journal binding differs"));
        }
        if pending.task.is_some() != pending.answer.is_some() {
            return Err(CanaryError::Invalid("incomplete canary attestation"));
        }
        if let (Some(task), Some(answer)) = (&pending.task, &pending.answer) {
            let task = CanaryTask::decode(task)?;
            let binding = Reader::new(&pending.plan.answer_context).frozen_binding()?;
            if task.deal.bytes() != deal
                || task.lap != lap
                || task.expected_logits_digest != canary::logits_digest(&pending.expected)?
                || task.answer_digest != canary::logits_digest(&answer.logits)?
                || task.answer_attestation(&binding)?
                    != client::canary_answer_attestation(
                        &pending.plan.answer_context,
                        &answer.logits,
                    )?
                || task.profile.room != RoomProfile::decode(&pending.profile)?
                || task.peer_signature.0.as_slice() != answer.signature
            {
                return Err(CanaryError::Invalid("canary journal evidence differs"));
            }
            verify_answer(&pending.plan, answer)?;
        }
        Ok(pending)
    }
    fn persist(&self, pending: &PendingCanary) -> Result<(), CanaryError> {
        let path = self.path(pending.plan.deal_id, pending.plan.lap_index);
        let mut nonce = [0; 16];
        getrandom::fill(&mut nonce)
            .map_err(|_| CanaryError::Invalid("canary journal entropy unavailable"))?;
        let temporary =
            path.with_extension(format!("{}.next", layerx_programs::hex::encode(&nonce)));
        let bytes = serde_json::to_vec(pending)
            .map_err(|_| CanaryError::Invalid("canary journal encoding"))?;
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temporary)?;
        file.write_all(&bytes)?;
        file.sync_all()?;
        fs::rename(&temporary, path)?;
        fs::File::open(&self.directory)?.sync_all()?;
        Ok(())
    }
    pub fn prepare(
        &self,
        bytes: &[u8],
        deal: [u8; 32],
        lap: u64,
        tokens: u64,
    ) -> Result<CanaryPlan, CanaryError> {
        let shared = decode_shared_state(bytes)?;
        let policy = PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
        let book = RoomBook::decode(shared.room_market()?.1)?;
        let recorded = book
            .deal(Digest32::new(deal)?)?
            .ok_or(CanaryError::Invalid("unknown deal"))?;
        let room = book
            .room(recorded.room_id)?
            .ok_or(CanaryError::Invalid("unknown room"))?;
        if room.model_digest.bytes() != self.table.model_digest
            || room.host_key != policy.header.owner_principal
            || recorded.state != market::rooms::DealState::Open
            || room.state != market::rooms::RoomState::Open
            || tokens == 0
            || tokens > room.profile.lap_size_tokens
        {
            return Err(CanaryError::Invalid("canary model, host or lap differs"));
        }
        let frozen = market::epoch::captured_roster(&shared)?
            .ok_or(CanaryError::Invalid("missing canary roster"))?;
        let key = zeroize::Zeroizing::new(client::canary_sampling_key(&self.host, deal, lap));
        let positions = canary_positions(&key, tokens, room.profile.canary_ratio_bps)?;
        let mut plan = CanaryPlan {
            deal_id: deal,
            lap_index: lap,
            generated_tokens: tokens,
            position: positions.first().copied(),
            prompt_tokens: Vec::new(),
            peer_key: None,
            answer_context: Vec::new(),
        };
        let mut expected = [0; 8];
        if let Some(position) = plan.position {
            let layers = recorded.layers.iter().flatten().collect::<Vec<_>>();
            if layers.is_empty() {
                return Err(CanaryError::Invalid("canary deal has no peers"));
            }
            let peer_index = usize::try_from(sample(&key, b"peer", layers.len() as u64))
                .map_err(|_| CanaryError::Invalid("canary peer index overflow"))?;
            let layer = layers[peer_index];
            let bid = book
                .bid(layer.bid_id)?
                .ok_or(CanaryError::Invalid("missing canary bid"))?;
            let peer = frozen
                .workers()
                .iter()
                .find(|w| w.owner == bid.peer_key)
                .ok_or(CanaryError::Invalid("canary peer not in frozen roster"))?;
            let example_index =
                usize::try_from(sample(&key, b"example", self.table.entries.len() as u64))
                    .map_err(|_| CanaryError::Invalid("canary example index overflow"))?;
            let example = &self.table.entries[example_index];
            plan.prompt_tokens.clone_from(&example.prompt_tokens);
            plan.peer_key = Some(peer.public_key.0);
            let mut prompt = self.table.model_digest.to_vec();
            prompt.extend_from_slice(&position.to_be_bytes());
            for token in &plan.prompt_tokens {
                prompt.extend_from_slice(&token.to_be_bytes());
            }
            let digest = market::codec::domain_hash("PAXAI/canary-prompt/v1", &prompt)?;
            plan.answer_context = client::canary_answer_context(
                &frozen.binding,
                recorded.deal_id,
                lap,
                peer.worker,
                digest,
            );
            expected = example.expected_logits;
        }
        let mut profile = vec![0; ROOM_PROFILE_BYTES];
        room.profile.encode(&mut profile)?;
        if self.path(deal, lap).exists() {
            let pending = self.load(deal, lap)?;
            if pending.plan != plan || pending.expected != expected || pending.profile != profile {
                return Err(CanaryError::Invalid("prepared canary changed"));
            }
        } else {
            self.persist(&PendingCanary {
                plan: plan.clone(),
                model_digest: self.table.model_digest,
                profile,
                expected,
                task: None,
                answer: None,
                issued: false,
                audited: false,
                completed: false,
            })?;
        }
        Ok(plan)
    }
    pub fn pending_request(
        &self,
        deal: [u8; 32],
        lap: u64,
    ) -> Result<Option<CanaryPlan>, CanaryError> {
        let pending = match self.load(deal, lap) {
            Ok(pending) => pending,
            Err(CanaryError::Io(error)) if error.kind() == std::io::ErrorKind::NotFound => {
                return Ok(None);
            }
            Err(error) => return Err(error),
        };
        Ok((pending.plan.position.is_some()
            && pending.plan.peer_key.is_some()
            && pending.answer.is_none())
        .then_some(pending.plan))
    }

    pub fn accept_peer_answer(
        &self,
        peer: [u8; 32],
        answer: CanaryAnswer,
    ) -> Result<(), CanaryError> {
        let pending = match self.load(answer.deal_id, answer.lap_index) {
            Err(CanaryError::Io(error)) if error.kind() == std::io::ErrorKind::NotFound => {
                return Err(CanaryError::Invalid("unknown canary plan"));
            }
            other => other?,
        };
        if pending.plan.position.is_none() || pending.plan.peer_key != Some(peer) {
            return Err(CanaryError::Invalid("wrong canary peer"));
        }
        if pending.answer.is_some() {
            return Err(CanaryError::Invalid("canary answer already accepted"));
        }
        self.accept_answer(answer)
    }

    pub fn accept_answer(&self, answer: CanaryAnswer) -> Result<(), CanaryError> {
        let mut pending = self.load(answer.deal_id, answer.lap_index)?;
        verify_answer(&pending.plan, &answer)?;
        if let Some(previous) = &pending.answer {
            if previous != &answer {
                return Err(CanaryError::Invalid("conflicting canary answer"));
            }
            return Ok(());
        }
        if pending.issued {
            return Err(CanaryError::Invalid("late canary attestation"));
        }
        let mut context = Reader::new(&pending.plan.answer_context);
        context.frozen_binding()?;
        let task = CanaryTask {
            deal: context.digest32()?,
            lap: context.u64()?,
            peer: context.worker_id()?,
            prompt_digest: context.digest32()?,
            expected_logits_digest: canary::logits_digest(&pending.expected)?,
            answer_digest: canary::logits_digest(&answer.logits)?,
            profile: CanaryRoomProfile {
                room: RoomProfile::decode(&pending.profile)?,
            },
            peer_signature: Signature64(
                answer
                    .signature
                    .as_slice()
                    .try_into()
                    .map_err(|_| CanaryError::Invalid("canary signature length"))?,
            ),
        };
        context.finish()?;
        pending.task = Some(task.encode()?.to_vec());
        pending.answer = Some(answer);
        self.persist(&pending)
    }
    pub fn require_committed(
        &self,
        bytes: &[u8],
        deal: [u8; 32],
        lap: u64,
        tokens: u64,
    ) -> Result<(), CanaryError> {
        self.prepare(bytes, deal, lap, tokens)?;
        let pending = self.load(deal, lap)?;
        if pending.plan.position.is_none() {
            return Ok(());
        }
        let task = CanaryTask::decode(
            pending
                .task
                .as_deref()
                .ok_or(CanaryError::Invalid("missing canary attestation"))?,
        )?;
        let (binding, host) = binding_and_host(bytes)?;
        let id = task.commitment(&binding, host)?;
        let record = canary::commitment_from_state(bytes, id)?;
        if record.task != task {
            return Err(CanaryError::Invalid("finalized canary differs"));
        }
        Ok(())
    }
    pub fn audit_answer(
        &self,
        bytes: &[u8],
        deal: [u8; 32],
        lap: u64,
    ) -> Result<bool, CanaryError> {
        let mut pending = self.load(deal, lap)?;
        self.require_committed(bytes, deal, lap, pending.plan.generated_tokens)?;
        let hit = match &pending.answer {
            Some(answer) => canary::relative_tolerance_hit(
                &pending.expected,
                &answer.logits,
                RoomProfile::decode(&pending.profile)?.canary_relative_tolerance_bps,
            )?,
            None if pending.plan.position.is_none() => true,
            None => return Err(CanaryError::Invalid("missing canary answer")),
        };
        pending.issued = true;
        pending.audited = true;
        self.persist(&pending)?;
        Ok(hit)
    }
    pub fn advance(
        &self,
        transport: &mut impl CanaryTransport,
        deal: [u8; 32],
        lap: u64,
    ) -> Result<bool, CanaryError> {
        let mut pending = self.load(deal, lap)?;
        if pending.completed {
            return Ok(true);
        }
        if pending.plan.position.is_none() {
            pending.completed = true;
            self.persist(&pending)?;
            return Ok(true);
        }
        let Some(encoded) = &pending.task else {
            return Ok(false);
        };
        let task = CanaryTask::decode(encoded)?;
        let view = transport.view()?;
        let (binding, host) = binding_and_host(&view.state)?;
        let original = Reader::new(&pending.plan.answer_context).frozen_binding()?;
        if original != binding {
            return Err(CanaryError::Invalid("canary epoch changed"));
        }
        let commitment = task.commitment(&binding, host)?;
        let shared = decode_shared_state(&view.state)?;
        let region = canary::CanaryRegion::decode(shared.control.feature_bytes)?;
        let Some(record) = region.get(commitment)? else {
            let policy = PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
            market::commit_reveal::canary_window(
                policy.header.origin_height,
                binding.epoch,
                false,
            )?
            .check(view.height)?;
            transport.submit(&client::commit_canary(&task)?)?;
            return Ok(false);
        };
        if record.task != task {
            return Err(CanaryError::Invalid("committed canary changed"));
        }
        if !pending.issued || !pending.audited {
            return Ok(false);
        }
        let answer = pending
            .answer
            .as_ref()
            .ok_or(CanaryError::Invalid("missing canary answer"))?;
        if canary::relative_tolerance_hit(
            &pending.expected,
            &answer.logits,
            task.profile.room.canary_relative_tolerance_bps,
        )? {
            pending.completed = true;
            self.persist(&pending)?;
            return Ok(true);
        }
        let complete =
            Self::challenge_and_reveal(transport, &view, &record, pending.expected, answer.logits)?;
        if complete {
            pending.completed = true;
            self.persist(&pending)?;
        }
        Ok(complete)
    }
    pub fn challenge_and_reveal(
        transport: &mut impl CanaryTransport,
        view: &CanaryView,
        record: &canary::CanaryCommitment,
        expected: CanaryLogits,
        answer: CanaryLogits,
    ) -> Result<bool, CanaryError> {
        let task = &record.task;
        let shared = decode_shared_state(&view.state)?;
        let policy = PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
        match record.status {
            ChallengeStatus::Miss => return Ok(true),
            ChallengeStatus::Hit => {
                return Err(CanaryError::Invalid(
                    "challenge resolution contradicts audit",
                ))
            }
            ChallengeStatus::Unchallenged => {
                canary::challenge_window(policy.header.origin_height, record)?
                    .check(view.height)?;
                transport.submit(&client::challenge_lap(&canary::ChallengeLapPayload {
                    deal: task.deal,
                    lap: task.lap,
                    commitment: record.id,
                    deposit: task.profile.room.minimum_bond,
                })?)?;
            }
            ChallengeStatus::Open if record.revealed_height.is_none() => {
                let window = market::commit_reveal::canary_window(
                    policy.header.origin_height,
                    record.epoch,
                    true,
                )?;
                if view.height < window.start {
                    return Ok(false);
                }
                window.check(view.height)?;
                transport.submit(&client::reveal_canary(&CanaryReveal {
                    commitment: record.id,
                    prompt_digest: task.prompt_digest,
                    logits: expected,
                })?)?;
            }
            ChallengeStatus::Open => {
                transport.submit(&client::resolve_challenge(
                    &canary::ResolveChallengePayload {
                        deal: task.deal,
                        lap: task.lap,
                        commitment: record.id,
                        answer,
                    },
                )?)?;
            }
        }
        Ok(false)
    }
    pub fn tick(&self, transport: &mut impl CanaryTransport) -> Result<bool, CanaryError> {
        for file in fs::read_dir(&self.directory)? {
            let file = file?;
            let name = file.file_name();
            let Some(name) = name
                .to_str()
                .and_then(|name| name.strip_prefix("canary-"))
                .and_then(|name| name.strip_suffix(".json"))
            else {
                continue;
            };
            let Some((deal, lap)) = name.split_once('-') else {
                return Err(CanaryError::Invalid("invalid canary journal name"));
            };
            let deal = layerx_programs::hex::decode_digest(deal)
                .map_err(|_| CanaryError::Invalid("invalid canary deal"))?;
            let lap = lap
                .parse()
                .map_err(|_| CanaryError::Invalid("invalid canary lap"))?;
            if !self.advance(transport, deal, lap)? {
                return Ok(false);
            }
        }
        Ok(true)
    }
}
fn verify_answer(plan: &CanaryPlan, answer: &CanaryAnswer) -> Result<(), CanaryError> {
    if answer.deal_id != plan.deal_id
        || answer.lap_index != plan.lap_index
        || plan.position.is_none()
    {
        return Err(CanaryError::Invalid("canary answer binding differs"));
    }
    let key = plan
        .peer_key
        .ok_or(CanaryError::Invalid("missing canary peer"))?;
    let signature = answer
        .signature
        .as_slice()
        .try_into()
        .map_err(|_| CanaryError::Invalid("canary signature length"))?;
    layerx_crypto::ed25519::verify_digest(
        &key,
        &signature,
        client::canary_answer_attestation(&plan.answer_context, &answer.logits)?.as_bytes(),
    )
    .map_err(|_| CanaryError::Invalid("invalid canary answer signature"))
}
fn binding_and_host(
    bytes: &[u8],
) -> Result<(market::types::FrozenBinding, PrincipalId), CanaryError> {
    let shared = decode_shared_state(bytes)?;
    let binding = market::epoch::captured_roster(&shared)?
        .ok_or(CanaryError::Invalid("missing canary roster"))?
        .binding;
    let policy = PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?;
    Ok((binding, policy.header.owner_principal))
}
