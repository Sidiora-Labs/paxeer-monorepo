use super::*;
use std::fs::{self, File, OpenOptions};
use std::io::{Read, Write};
use std::os::unix::fs::{MetadataExt, OpenOptionsExt, PermissionsExt};
use std::path::{Path, PathBuf};
use zeroize::Zeroize;

const MAX_STATE: u64 = 8 * 1024 * 1024;

#[derive(Clone, Serialize, Deserialize)]
pub(super) struct Installation {
    #[serde(with = "principal")]
    pub principal: PrincipalId,
    #[serde(with = "did")]
    pub did: Did,
    pub seed: [u8; 32],
    pub grantor: [u8; 32],
    pub revocation_sequence: u64,
    pub not_before: u64,
    pub expires_at: u64,
    pub core_sequence: u64,
    #[serde(with = "activities")]
    pub activities: Vec<ActivityType>,
    pub attempted: bool,
    pub installed: Option<SessionLease>,
}
impl Installation {
    pub fn issue(&self) -> Result<IssuedSessionKey, SessionKeyError> {
        Ok(issue_session_key(&SessionKeyRequest {
            fee_budget: None,
            purpose: layerx_crypto::session::SessionPurpose::Activity,
            grantor: self.grantor,
            session_public_key: LocalSigner::new(self.seed).public_key(),
            not_before: self.not_before,
            expires_at: Some(self.expires_at),
            permitted_activity_types: self.activities.clone(),
            revocation_sequence: Some(self.revocation_sequence),
        })?)
    }
}
impl Drop for Installation {
    fn drop(&mut self) {
        self.seed.zeroize();
    }
}

#[derive(Clone, Serialize, Deserialize)]
pub(super) struct Renewal {
    pub previous: SessionLease,
    pub replacement: Option<SessionLease>,
    pub requested_at: u64,
    pub reason: RevocationReason,
}
#[derive(Clone, Serialize, Deserialize)]
pub(super) struct Stop {
    pub requested_at: u64,
    pub reason: RevocationReason,
}
#[derive(Clone, Serialize, Deserialize)]
pub(super) struct Announcement {
    pub journey: RotationJourney,
    pub payload_hash: [u8; 32],
    pub attempted: bool,
}

#[derive(Serialize, Deserialize)]
pub(super) struct State {
    version: u8,
    policy: SessionPolicy,
    agents: Vec<ManagedAgent>,
    rotations: Vec<RotationJourney>,
    history: Vec<RotationJourney>,
    installations: Vec<Installation>,
    renewals: Vec<Renewal>,
    stops: Vec<ScopedStop>,
    announcements: Vec<Announcement>,
}
#[derive(Serialize, Deserialize)]
struct ScopedStop {
    #[serde(with = "principal")]
    principal: PrincipalId,
    #[serde(with = "did")]
    did: Did,
    stop: Stop,
}
impl State {
    pub fn capture<C: AgentSessionContract, E: SessionEntropySource>(
        owner: &SessionKeyProvisioner<C, E>,
    ) -> Self {
        Self {
            version: 1,
            policy: owner.policy.clone(),
            agents: owner.agents.values().cloned().collect(),
            rotations: owner.rotations.values().cloned().collect(),
            history: owner.rotation_history.clone(),
            installations: owner.installations.values().cloned().collect(),
            renewals: owner.renewals.values().cloned().collect(),
            stops: owner
                .stops
                .iter()
                .map(|((principal, did), stop)| ScopedStop {
                    principal: principal.clone(),
                    did: did.clone(),
                    stop: stop.clone(),
                })
                .collect(),
            announcements: owner.announcements.values().cloned().collect(),
        }
    }
    pub fn restore<C: AgentSessionContract, E: SessionEntropySource>(
        self,
        owner: &mut SessionKeyProvisioner<C, E>,
    ) -> Result<(), SessionKeyError> {
        if self.version != 1 || self.policy != owner.policy {
            return Err(SessionKeyError::InvalidPolicy);
        }
        for agent in self.agents {
            validate_lease(&agent.current)?;
            for retired in &agent.retired {
                validate_lease(retired)?;
            }
            let key = (
                agent.current.principal.clone(),
                agent.current.agent_did.clone(),
            );
            if agent
                .retired
                .iter()
                .any(|lease| lease.principal != key.0 || lease.agent_did != key.1)
                || owner.agents.insert(key, agent).is_some()
            {
                return Err(SessionKeyError::InvalidEvidence);
            }
        }
        for journey in self.rotations {
            validate_rotation(&journey)?;
            if matches!(
                journey.state,
                RotationJourneyState::Effective { .. } | RotationJourneyState::Lapsed
            ) || owner
                .rotations
                .insert((journey.principal.clone(), journey.did.clone()), journey)
                .is_some()
            {
                return Err(SessionKeyError::InvalidEvidence);
            }
        }
        for journey in &self.history {
            validate_rotation(journey)?;
            if !matches!(
                journey.state,
                RotationJourneyState::Effective { .. } | RotationJourneyState::Lapsed
            ) {
                return Err(SessionKeyError::InvalidEvidence);
            }
        }
        owner.rotation_history = self.history;
        for installation in self.installations {
            installation.issue()?;
            if let Some(lease) = &installation.installed {
                validate_lease(lease)?;
            }
            if owner
                .installations
                .insert(
                    (installation.principal.clone(), installation.did.clone()),
                    installation,
                )
                .is_some()
            {
                return Err(SessionKeyError::InvalidEvidence);
            }
        }
        for renewal in self.renewals {
            validate_lease(&renewal.previous)?;
            let key = (
                renewal.previous.principal.clone(),
                renewal.previous.agent_did.clone(),
            );
            if !owner.agents.contains_key(&key) || owner.renewals.insert(key, renewal).is_some() {
                return Err(SessionKeyError::InvalidEvidence);
            }
        }
        for stop in self.stops {
            let key = (stop.principal, stop.did);
            if !owner.agents.contains_key(&key) || owner.stops.insert(key, stop.stop).is_some() {
                return Err(SessionKeyError::InvalidEvidence);
            }
        }
        for announcement in self.announcements {
            let key = (
                announcement.journey.principal.clone(),
                announcement.journey.did.clone(),
            );
            if owner.rotations.contains_key(&key)
                || owner.announcements.insert(key, announcement).is_some()
            {
                return Err(SessionKeyError::InvalidEvidence);
            }
        }
        Ok(())
    }
}
fn validate_lease(lease: &SessionLease) -> Result<(), SessionKeyError> {
    if lease.grant_id == [0; 32]
        || lease.daemon_session_id == [0; 32]
        || lease.session_public_key == [0; 32]
        || lease.provision_receipt_digest == [0; 32]
        || lease.issued_at >= lease.expires_at
        || lease.permitted_activity_types.is_empty()
    {
        return Err(SessionKeyError::InvalidEvidence);
    }
    Ok(())
}
fn validate_rotation(journey: &RotationJourney) -> Result<(), SessionKeyError> {
    if journey.tx_hash == [0; 32]
        || journey.announcement_receipt_digest == [0; 32]
        || journey.current_public_key == journey.pending_public_key
        || journey.announced_at >= journey.effective_at
        || journey.effective_at >= journey.lapse_at
        || journey.effective_sequence == 0
    {
        return Err(SessionKeyError::InvalidEvidence);
    }
    Ok(())
}

pub(super) struct StateFile {
    root: PathBuf,
    _lock: File,
}
impl StateFile {
    pub fn open(root: &Path) -> Result<(Self, Option<State>), SessionKeyError> {
        let io = |error| SessionKeyError::Custody(CustodyError::Io(error));
        if !root.exists() {
            fs::create_dir_all(root).map_err(io)?;
            fs::set_permissions(root, fs::Permissions::from_mode(0o700)).map_err(io)?;
        }
        let metadata = fs::symlink_metadata(root).map_err(io)?;
        if !metadata.is_dir()
            || metadata.uid() != rustix::process::geteuid().as_raw()
            || metadata.mode() & 0o077 != 0
        {
            return Err(SessionKeyError::InvalidEvidence);
        }
        let lock = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .mode(0o600)
            .custom_flags(rustix::fs::OFlags::NOFOLLOW.bits() as i32)
            .open(root.join("lock"))
            .map_err(io)?;
        secure(&lock)?;
        rustix::fs::flock(&lock, rustix::fs::FlockOperation::NonBlockingLockExclusive)
            .map_err(|_| SessionKeyError::InvalidEvidence)?;
        // rename is the journal commit point; an unrenamed staging inode has no authority.
        let staging = root.join("state.tmp");
        match fs::symlink_metadata(&staging) {
            Ok(_) => {
                let file = protected(&staging)?;
                secure(&file)?;
                fs::remove_file(&staging).map_err(io)?;
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => (),
            Err(error) => return Err(io(error)),
        }
        let state = match fs::symlink_metadata(root.join("state")) {
            Ok(_) => {
                let mut file = protected(&root.join("state"))?;
                let mut bytes = Zeroizing::new(Vec::new());
                Read::by_ref(&mut file)
                    .take(MAX_STATE + 1)
                    .read_to_end(&mut bytes)
                    .map_err(io)?;
                if bytes.len() as u64 > MAX_STATE {
                    return Err(SessionKeyError::InvalidEvidence);
                }
                Some(serde_json::from_slice(&bytes).map_err(|_| SessionKeyError::InvalidEvidence)?)
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => None,
            Err(error) => return Err(io(error)),
        };
        Ok((
            Self {
                root: root.to_owned(),
                _lock: lock,
            },
            state,
        ))
    }
    pub fn save(&self, state: &State) -> Result<(), SessionKeyError> {
        let io = |error| SessionKeyError::Custody(CustodyError::Io(error));
        let bytes = Zeroizing::new(
            serde_json::to_vec(state).map_err(|_| SessionKeyError::InvalidEvidence)?,
        );
        if bytes.len() as u64 > MAX_STATE {
            return Err(SessionKeyError::InvalidEvidence);
        }
        let staging = self.root.join("state.tmp");
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&staging)
            .map_err(io)?;
        file.write_all(&bytes)
            .and_then(|()| file.sync_all())
            .map_err(io)?;
        fs::rename(staging, self.root.join("state")).map_err(io)?;
        File::open(&self.root)
            .and_then(|file| file.sync_all())
            .map_err(io)
    }
}
fn secure(file: &File) -> Result<(), SessionKeyError> {
    let metadata = file
        .metadata()
        .map_err(|error| SessionKeyError::Custody(CustodyError::Io(error)))?;
    if !metadata.is_file()
        || metadata.uid() != rustix::process::geteuid().as_raw()
        || metadata.mode() & 0o077 != 0
        || metadata.nlink() != 1
        || metadata.len() > MAX_STATE
    {
        return Err(SessionKeyError::InvalidEvidence);
    }
    Ok(())
}
fn protected(path: &Path) -> Result<File, SessionKeyError> {
    let descriptor = rustix::fs::open(
        path,
        rustix::fs::OFlags::RDONLY
            | rustix::fs::OFlags::NOFOLLOW
            | rustix::fs::OFlags::NONBLOCK
            | rustix::fs::OFlags::CLOEXEC,
        rustix::fs::Mode::empty(),
    )
    .map_err(|_| SessionKeyError::InvalidEvidence)?;
    let file = File::from(descriptor);
    secure(&file)?;
    Ok(file)
}

pub(super) mod principal {
    use super::*;
    pub fn serialize<S: serde::Serializer>(
        value: &PrincipalId,
        serializer: S,
    ) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(value.as_str())
    }
    pub fn deserialize<'de, D: serde::Deserializer<'de>>(
        deserializer: D,
    ) -> Result<PrincipalId, D::Error> {
        PrincipalId::new(String::deserialize(deserializer)?).map_err(serde::de::Error::custom)
    }
}
pub(super) mod did {
    use super::*;
    pub fn serialize<S: serde::Serializer>(value: &Did, serializer: S) -> Result<S::Ok, S::Error> {
        value.as_bytes().serialize(serializer)
    }
    pub fn deserialize<'de, D: serde::Deserializer<'de>>(deserializer: D) -> Result<Did, D::Error> {
        Did::new(&Vec::<u8>::deserialize(deserializer)?)
            .map_err(|error| serde::de::Error::custom(format!("{error:?}")))
    }
}
pub(super) mod key_id {
    use super::*;
    pub fn serialize<S: serde::Serializer>(
        value: &KeyId,
        serializer: S,
    ) -> Result<S::Ok, S::Error> {
        serializer.serialize_str(value.as_str())
    }
    pub fn deserialize<'de, D: serde::Deserializer<'de>>(
        deserializer: D,
    ) -> Result<KeyId, D::Error> {
        KeyId::new(String::deserialize(deserializer)?).map_err(serde::de::Error::custom)
    }
}
pub(super) mod activities {
    use super::*;
    pub fn serialize<S: serde::Serializer>(
        value: &[ActivityType],
        serializer: S,
    ) -> Result<S::Ok, S::Error> {
        value
            .iter()
            .map(|activity| activity.value())
            .collect::<Vec<_>>()
            .serialize(serializer)
    }
    pub fn deserialize<'de, D: serde::Deserializer<'de>>(
        deserializer: D,
    ) -> Result<Vec<ActivityType>, D::Error> {
        Vec::<u32>::deserialize(deserializer)?
            .into_iter()
            .map(|value| {
                ActivityType::from_u32(value)
                    .map_err(|error| serde::de::Error::custom(format!("{error:?}")))
            })
            .collect()
    }
}
