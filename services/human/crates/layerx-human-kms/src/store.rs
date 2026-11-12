use crate::config::{protected, Config};
use crate::wire::{blob, Error, Operation, Request, Result};
use ring::aead::{Aad, LessSafeKey, Nonce, UnboundKey, AES_256_GCM};
use ring::signature::{Ed25519KeyPair, KeyPair};
use serde::{Deserialize, Serialize};
use sha2::{Digest, Sha256};
use std::collections::BTreeMap;
use std::fs::{self, File, OpenOptions};
use std::io::Write;
use std::os::unix::fs::{MetadataExt, OpenOptionsExt, PermissionsExt};
use std::path::PathBuf;
use zeroize::{Zeroize, Zeroizing};

const MAX_STATE: usize = 8 * 1024 * 1024;
const MAX_RECORD: usize = 2 * 1024 * 1024;
#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct Record {
    binding: [u8; 32],
    class: u8,
    handle: [u8; 32],
    public: [u8; 32],
    seed: Option<[u8; 32]>,
    previous: Option<[u8; 32]>,
    generation: u64,
    #[serde(default)]
    wallet: Option<crate::evm::Wallet>,
    #[serde(default)]
    send_actions: BTreeMap<String, (crate::send::SendAuthorization, Vec<u8>)>,
    #[serde(default, skip_serializing_if = "Option::is_none")]
    recipient_identity: Option<[u8; 32]>,
    #[serde(default)]
    send_reservations: BTreeMap<String, [u8; 32]>,
    #[serde(default)]
    self_custodied: bool,
    #[serde(default)]
    export: Option<PendingExport>,
}
impl Drop for Record {
    fn drop(&mut self) {
        self.seed.zeroize();
    }
}
#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct PendingExport {
    envelope: Option<crate::export::ExportEnvelope>,
    ceremony: crate::export::ExportCeremony,
    proof_hash: [u8; 32],
}
#[derive(Default, Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct Nonces {
    next: u64,
    requests: BTreeMap<u64, [u8; 32]>,
}
fn reserve_nonce(
    nonces: &mut BTreeMap<String, Nonces>,
    address: [u8; 20],
    chain: u64,
    nonce: u64,
    digest: [u8; 32],
) -> Result<()> {
    let account = nonces
        .entry(format!("{chain}:{}", hex(&address)))
        .or_default();
    if let Some(previous) = account.requests.get(&nonce) {
        return if *previous == digest {
            Ok(())
        } else {
            Err(Error::Conflict)
        };
    }
    if nonce < account.next || account.requests.len() >= 4096 {
        return Err(Error::Conflict);
    }
    account.next = nonce.checked_add(1).ok_or(Error::Refused)?;
    account.requests.insert(nonce, digest);
    Ok(())
}
#[derive(Serialize, Deserialize)]
#[serde(deny_unknown_fields)]
struct State {
    version: u8,
    provider: String,
    network: u32,
    records: BTreeMap<String, Record>,
    #[serde(default)]
    nonces: BTreeMap<String, Nonces>,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct AttestorBindings {
    bindings: Vec<AttestorBinding>,
}
#[derive(Deserialize)]
#[serde(deny_unknown_fields)]
struct AttestorBinding {
    binding: [u8; 32],
    tenant: String,
    principal: String,
    config: crate::attestor::AttestorSignerConfig,
    assertion_file: PathBuf,
}
struct EvmAttestor {
    signer: crate::attestor::AttestorSigner,
    key_id: String,
    address: [u8; 20],
    tenant: String,
    principal: String,
    assertion_file: PathBuf,
    assertion_subject: String,
}
impl EvmAttestor {
    fn load(chain_id: u64) -> std::result::Result<BTreeMap<String, Self>, String> {
        let Some(path) = std::env::var_os("LAYERX_HUMAN_KMS_ATTESTOR_CONFIG_FILE") else {
            return Ok(BTreeMap::new());
        };
        Self::load_file(chain_id, std::path::Path::new(&path))
    }
    fn load_file(
        chain_id: u64,
        path: &std::path::Path,
    ) -> std::result::Result<BTreeMap<String, Self>, String> {
        let bytes = protected(path, MAX_STATE, true)?;
        let bindings: AttestorBindings = serde_json::from_slice(&bytes)
            .map_err(|_| "principal-bound attestor configuration required")?;
        if bindings.bindings.len() > 4096 {
            return Err("attestor binding count refused".into());
        }
        let mut result = BTreeMap::new();
        let mut addresses = std::collections::BTreeSet::new();
        let mut keys = std::collections::BTreeSet::new();
        for binding in bindings.bindings {
            let config = binding.config;
            if binding.binding == [0; 32]
                || binding.tenant.is_empty()
                || binding.principal.is_empty()
                || binding.tenant.len() > 128
                || binding.principal.len() > 128
                || binding.tenant.contains('\0')
                || binding.principal.contains('\0')
                || u64::from(config.network_id) != chain_id
                || config.evm_address == [0; 20]
                || !addresses.insert(config.evm_address)
                || !keys.insert(config.evm_key_id.clone())
            {
                return Err("attestor custody isolation refused".into());
            }
            let assertion = protected(&binding.assertion_file, 65_536, true)?;
            let assertion =
                std::str::from_utf8(&assertion).map_err(|_| "EVM owner assertion invalid")?;
            let mut signer = crate::attestor::AttestorSigner::from_config(&config)
                .map_err(|error| error.to_string())?;
            signer
                .set_evm_assertion(assertion.trim())
                .map_err(|error| error.to_string())?;
            let assertion_subject = crate::attestor::assertion_claims(assertion.trim())
                .map_err(|_| "EVM owner assertion claims invalid")?
                .subject;
            let attestor = Self {
                signer,
                key_id: config.evm_key_id,
                address: config.evm_address,
                tenant: binding.tenant,
                principal: binding.principal,
                assertion_file: binding.assertion_file,
                assertion_subject,
            };
            if result.insert(hex(&binding.binding), attestor).is_some() {
                return Err("duplicate attestor binding".into());
            }
        }
        Ok(result)
    }
    fn refresh(&mut self, now: u64) -> Result<()> {
        let bytes =
            protected(&self.assertion_file, 65_536, true).map_err(|_| Error::Unavailable)?;
        let assertion = std::str::from_utf8(&bytes)
            .map_err(|_| Error::Integrity)?
            .trim();
        let claims =
            crate::attestor::assertion_claims(assertion).map_err(|error| attestor_error(&error))?;
        if claims.subject != self.assertion_subject {
            return Err(Error::Integrity);
        }
        claims.check(now).map_err(|error| attestor_error(&error))?;
        self.signer
            .set_evm_assertion(assertion)
            .map_err(|error| attestor_error(&error))
    }
    fn validate_wallet(&self, wallet: &crate::evm::Wallet) -> Result<()> {
        if wallet.evm_key_id != self.key_id
            || wallet.address != self.address
            || wallet.tenant != self.tenant
            || wallet.principal != self.principal
            || (!wallet.assertion_subject.is_empty()
                && wallet.assertion_subject != self.assertion_subject)
        {
            return Err(Error::Integrity);
        }
        Ok(())
    }
}
pub(crate) struct Store {
    root: PathBuf,
    state: State,
    key: LessSafeKey,
    aad: Vec<u8>,
    healthy: bool,
    evm_attestors: BTreeMap<String, EvmAttestor>,
    durable: Zeroizing<Vec<u8>>,
    _lock: File,
}
pub(crate) fn hex(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    bytes
        .iter()
        .flat_map(|b| {
            [
                char::from(DIGITS[usize::from(b >> 4)]),
                char::from(DIGITS[usize::from(b & 15)]),
            ]
        })
        .collect()
}
fn signing_key(seed: &[u8; 32]) -> Result<Ed25519KeyPair> {
    Ed25519KeyPair::from_seed_unchecked(seed).map_err(|_| Error::Integrity)
}
fn public(seed: &[u8; 32]) -> Result<[u8; 32]> {
    signing_key(seed)?
        .public_key()
        .as_ref()
        .try_into()
        .map_err(|_| Error::Integrity)
}
fn new_seed() -> Result<Zeroizing<[u8; 32]>> {
    let mut seed = Zeroizing::new([0; 32]);
    getrandom::fill(&mut *seed).map_err(|_| Error::Unavailable)?;
    Ok(seed)
}
fn description(record: &Record) -> Result<Vec<u8>> {
    let mut bytes = Vec::new();
    blob(&mut bytes, &record.handle)?;
    bytes.extend(record.public);
    bytes.extend(record.binding);
    bytes.push(0);
    Ok(bytes)
}

fn read_state(
    config: &Config,
    key: &LessSafeKey,
    aad: &[u8],
    network: u32,
) -> std::result::Result<State, String> {
    let path = config.state.join("state.aead");
    if path.try_exists().map_err(|_| "state lookup failed")? {
        let mut encrypted = protected(&path, MAX_STATE + 28, true)?;
        if encrypted.len() < 28 {
            return Err("state truncated".into());
        }
        let nonce: [u8; 12] = encrypted[..12]
            .try_into()
            .map_err(|_| "state nonce invalid")?;
        let plain = key
            .open_in_place(
                Nonce::assume_unique_for_key(nonce),
                Aad::from(aad),
                &mut encrypted[12..],
            )
            .map_err(|_| "state authentication failed")?;
        serde_json::from_slice::<State>(plain).map_err(|_| "state encoding invalid".into())
    } else {
        Ok(State {
            version: 1,
            provider: config.provider.clone(),
            network,
            records: BTreeMap::new(),
            nonces: BTreeMap::new(),
        })
    }
}

fn remove_recovery_file(root: &std::path::Path) -> std::result::Result<(), String> {
    let temp = root.join("state.next");
    if temp
        .try_exists()
        .map_err(|_| "state recovery lookup failed")?
    {
        let metadata = fs::symlink_metadata(&temp).map_err(|_| "state recovery metadata failed")?;
        if !metadata.is_file()
            || metadata.file_type().is_symlink()
            || metadata.uid() != rustix::process::geteuid().as_raw()
            || metadata.mode() & 0o077 != 0
        {
            return Err("state recovery file refused".into());
        }
        fs::remove_file(temp).map_err(|_| "state recovery failed")?;
    }
    Ok(())
}

impl Store {
    pub fn open(config: &Config) -> std::result::Result<Self, String> {
        Self::open_with_attestors(config, None)
    }
    pub(crate) fn open_with_attestors(
        config: &Config,
        attestors: Option<&std::path::Path>,
    ) -> std::result::Result<Self, String> {
        if !config.state.is_absolute() {
            return Err("state path must be absolute".into());
        }
        fs::create_dir_all(&config.state).map_err(|_| "state directory unavailable")?;
        let metadata =
            fs::symlink_metadata(&config.state).map_err(|_| "state metadata unavailable")?;
        if !metadata.is_dir()
            || metadata.file_type().is_symlink()
            || metadata.uid() != rustix::process::geteuid().as_raw()
        {
            return Err("state ownership refused".into());
        }
        fs::set_permissions(&config.state, fs::Permissions::from_mode(0o700))
            .map_err(|_| "state permissions unavailable")?;
        let lock_fd = rustix::fs::open(
            config.state.join("owner.lock"),
            rustix::fs::OFlags::CREATE
                | rustix::fs::OFlags::RDWR
                | rustix::fs::OFlags::NOFOLLOW
                | rustix::fs::OFlags::CLOEXEC,
            rustix::fs::Mode::from_bits_truncate(0o600),
        )
        .map_err(|_| "state lock unavailable")?;
        let lock = File::from(lock_fd);
        let metadata = lock
            .metadata()
            .map_err(|_| "state lock metadata unavailable")?;
        if !metadata.is_file()
            || metadata.uid() != rustix::process::geteuid().as_raw()
            || metadata.mode() & 0o077 != 0
        {
            return Err("state lock ownership or permissions refused".into());
        }
        lock.try_lock().map_err(|_| "state already owned")?;
        let key = LessSafeKey::new(
            UnboundKey::new(&AES_256_GCM, &config.seal).map_err(|_| "seal key invalid")?,
        );
        let mut aad = b"LXKP/state/v1\0".to_vec();
        aad.extend(config.provider.as_bytes());
        let network = u32::try_from(config.chain.chain_id).map_err(|_| "state chain id invalid")?;
        aad.extend(network.to_be_bytes());
        let mut state = read_state(config, &key, &aad, network)?;
        validate(&state, config)?;
        let evm_attestors = match attestors {
            Some(path) => EvmAttestor::load_file(config.chain.chain_id, path)?,
            None => EvmAttestor::load(config.chain.chain_id)?,
        };
        for (binding, record) in &state.records {
            if let Some(wallet) = &record.wallet {
                evm_attestors
                    .get(binding)
                    .ok_or("retained wallet attestor binding missing")?
                    .validate_wallet(wallet)
                    .map_err(|_| "retained wallet attestor identity changed")?;
                for (chain, next) in &wallet.next_nonce {
                    let account = state
                        .nonces
                        .entry(format!("{chain}:{}", hex(&wallet.address)))
                        .or_default();
                    account.next = account.next.max(*next);
                }
            }
        }
        remove_recovery_file(&config.state)?;
        let mut store = Self {
            root: config.state.clone(),
            state,
            key,
            aad,
            healthy: true,
            evm_attestors,
            durable: Zeroizing::new(Vec::new()),
            _lock: lock,
        };
        store
            .persist()
            .map_err(|_| "state durability unavailable")?;
        Ok(store)
    }
    fn capacity(&self, binding: Option<[u8; 32]>, extra: usize) -> Result<()> {
        let size =
            Zeroizing::new(serde_json::to_vec(&self.state).map_err(|_| Error::Integrity)?).len();
        if size.saturating_add(extra) > MAX_STATE {
            return Err(Error::Capacity);
        }
        for (key, record) in &self.state.records {
            let size =
                Zeroizing::new(serde_json::to_vec(record).map_err(|_| Error::Integrity)?).len();
            let reserve = if binding.is_some_and(|binding| key == &hex(&binding)) {
                extra
            } else {
                0
            };
            if size.saturating_add(reserve) > MAX_RECORD {
                return Err(Error::Capacity);
            }
        }
        Ok(())
    }
    fn persist(&mut self) -> Result<()> {
        self.capacity(None, 0)?;
        let result = self.write_state();
        if result.is_err() {
            self.healthy = false;
        } else {
            self.durable =
                Zeroizing::new(serde_json::to_vec(&self.state).map_err(|_| Error::Integrity)?);
        }
        result
    }
    fn archive_path(&self, binding: [u8; 32], action: [u8; 32]) -> PathBuf {
        self.root
            .join(format!("action-{}-{}.aead", hex(&binding), hex(&action)))
    }
    fn archive(&self, binding: [u8; 32], action: &crate::evm_types::EvmAction) -> Result<()> {
        let mut plain = Zeroizing::new(serde_json::to_vec(action).map_err(|_| Error::Integrity)?);
        if plain.len() > MAX_RECORD {
            return Err(Error::Capacity);
        }
        let mut nonce = [0; 12];
        getrandom::fill(&mut nonce).map_err(|_| Error::Unavailable)?;
        let mut aad = self.aad.clone();
        aad.extend(binding);
        aad.extend(action.authorization.action_key);
        self.key
            .seal_in_place_append_tag(
                Nonce::assume_unique_for_key(nonce),
                Aad::from(&aad),
                &mut *plain,
            )
            .map_err(|_| Error::Unavailable)?;
        let path = self.archive_path(binding, action.authorization.action_key);
        let temp = path.with_extension("next");
        let mut file = OpenOptions::new()
            .write(true)
            .create(true)
            .truncate(true)
            .custom_flags(i32::from_ne_bytes(
                rustix::fs::OFlags::NOFOLLOW.bits().to_ne_bytes(),
            ))
            .mode(0o600)
            .open(&temp)
            .map_err(|_| Error::Unavailable)?;
        file.write_all(&nonce)
            .and_then(|()| file.write_all(&plain))
            .and_then(|()| file.sync_all())
            .map_err(|_| Error::Unavailable)?;
        fs::rename(temp, path).map_err(|_| Error::Unavailable)?;
        File::open(&self.root)
            .and_then(|file| file.sync_all())
            .map_err(|_| Error::Unavailable)
    }
    fn archived(
        &self,
        binding: [u8; 32],
        key: [u8; 32],
    ) -> Result<Option<crate::evm_types::EvmAction>> {
        let wallet = self
            .state
            .records
            .get(&hex(&binding))
            .and_then(|record| record.wallet.as_ref())
            .ok_or(Error::NotFound)?;
        let Some(expected) = wallet.archived.get(&hex(&key)) else {
            return Ok(None);
        };
        let mut encrypted = protected(&self.archive_path(binding, key), MAX_RECORD + 28, true)
            .map_err(|_| Error::Unavailable)?;
        if encrypted.len() < 28 {
            return Err(Error::Integrity);
        }
        let nonce = encrypted[..12].try_into().map_err(|_| Error::Integrity)?;
        let mut aad = self.aad.clone();
        aad.extend(binding);
        aad.extend(key);
        let plain = self
            .key
            .open_in_place(
                Nonce::assume_unique_for_key(nonce),
                Aad::from(&aad),
                &mut encrypted[12..],
            )
            .map_err(|_| Error::Integrity)?;
        let action: crate::evm_types::EvmAction =
            serde_json::from_slice(plain).map_err(|_| Error::Integrity)?;
        if !action.acknowledged
            || action.authorization.action_key != key
            || action.authorization.binding_digest != binding
            || authorization_hash(&action.authorization)? != *expected
        {
            return Err(Error::Integrity);
        }
        Ok(Some(action))
    }
    fn write_state(&self) -> Result<()> {
        let mut plain =
            Zeroizing::new(serde_json::to_vec(&self.state).map_err(|_| Error::Integrity)?);
        if plain.len() > MAX_STATE {
            return Err(Error::Capacity);
        }
        let mut nonce = [0; 12];
        getrandom::fill(&mut nonce).map_err(|_| Error::Unavailable)?;
        self.key
            .seal_in_place_append_tag(
                Nonce::assume_unique_for_key(nonce),
                Aad::from(&self.aad),
                &mut *plain,
            )
            .map_err(|_| Error::Unavailable)?;
        let temp = self.root.join("state.next");
        let mut file = OpenOptions::new()
            .write(true)
            .create_new(true)
            .mode(0o600)
            .open(&temp)
            .map_err(|_| Error::Unavailable)?;
        file.write_all(&nonce)
            .and_then(|()| file.write_all(&plain))
            .and_then(|()| file.sync_all())
            .map_err(|_| Error::Unavailable)?;
        fs::rename(temp, self.root.join("state.aead")).map_err(|_| Error::Unavailable)?;
        File::open(&self.root)
            .and_then(|f| f.sync_all())
            .map_err(|_| Error::Unavailable)
    }
    pub fn dispatch(
        &mut self,
        request: &Request<'_>,
        signing_digest: Option<[u8; 32]>,
        clock: &dyn layerx_types::clock::Clock,
        deadline: &mut layerx_types::clock::Deadline,
    ) -> Result<Vec<u8>> {
        let now = fresh(clock)?;
        let result = self.dispatch_inner(request, signing_digest, now, clock, deadline);
        if result.is_err() && self.healthy && !self.durable.is_empty() {
            self.state = serde_json::from_slice(&self.durable).map_err(|_| Error::Integrity)?;
        }
        result
    }
    fn dispatch_inner(
        &mut self,
        request: &Request<'_>,
        signing_digest: Option<[u8; 32]>,
        now: u64,
        clock: &dyn layerx_types::clock::Clock,
        deadline: &mut layerx_types::clock::Deadline,
    ) -> Result<Vec<u8>> {
        if !self.healthy {
            return Err(Error::Unavailable);
        }
        if request.provider != self.state.provider
            || (request.kind != Operation::Probe && request.network != self.state.network)
        {
            return Err(Error::Refused);
        }
        match request.kind {
            Operation::Probe => {
                self.persist()?;
                return Ok(Vec::new());
            }
            Operation::ExecutorProbe => {
                let mut answer = request.challenge.to_vec();
                answer.extend(self.state.network.to_be_bytes());
                return Ok(answer);
            }
            Operation::Create => return self.create(request),
            Operation::Rotate => return Err(Error::Refused),
            Operation::AuthorizeSend => {
                return self.authorize_send(
                    request,
                    signing_digest.ok_or(Error::Refused)?,
                    now,
                    clock,
                    deadline,
                )
            }
            Operation::EvmWallet | Operation::Evm => return self.evm(request, clock, deadline),
            Operation::ExportPrimary => return self.export_primary(request),
            Operation::AcknowledgeExport => return self.acknowledge_export(request),
            Operation::Sign => {
                return self.sign_evm(
                    request,
                    signing_digest.ok_or(Error::Refused)?,
                    clock,
                    deadline,
                )
            }
            Operation::Describe | Operation::RotateIfCurrent | Operation::Destroy => {}
        }
        let key = hex(&request.binding);
        let record = self.state.records.get_mut(&key).ok_or(Error::NotFound)?;
        if record.class != request.class || request.reference != record.handle {
            return Err(Error::Integrity);
        }
        if request.kind == Operation::Destroy && record.seed.is_none() {
            return Ok(Vec::new());
        }
        if record.self_custodied && request.kind != Operation::Describe {
            return Err(Error::SelfCustodied);
        }
        if record.seed.is_none() && !record.self_custodied {
            return Err(Error::NotFound);
        }
        match request.kind {
            Operation::Describe => description(record),
            Operation::RotateIfCurrent => {
                self.rotate(&key, request.expected.ok_or(Error::Refused)?)
            }
            Operation::Destroy => {
                record.seed.zeroize();
                record.seed = None;
                record.wallet = None;
                record.send_actions.clear();
                self.persist()?;
                Ok(Vec::new())
            }
            _ => Err(Error::Refused),
        }
    }
    fn sign_evm(
        &mut self,
        request: &Request<'_>,
        digest: [u8; 32],
        clock: &dyn layerx_types::clock::Clock,
        deadline: &mut layerx_types::clock::Deadline,
    ) -> Result<Vec<u8>> {
        let record = self
            .state
            .records
            .get(&hex(&request.binding))
            .ok_or(Error::NotFound)?;
        if record.class != request.class || request.reference != record.handle {
            return Err(Error::Integrity);
        }
        if record.self_custodied {
            return Err(Error::SelfCustodied);
        }
        record.seed.as_ref().ok_or(Error::NotFound)?;
        let attestor = self
            .evm_attestors
            .get(&hex(&request.binding))
            .ok_or(Error::Unavailable)?;
        let wallet = record.wallet.as_ref().ok_or(Error::NotFound)?;
        attestor.validate_wallet(wallet)?;
        if request.from != wallet.address {
            return Err(Error::Integrity);
        }
        let call = crate::attestor::decode_unsigned_evm_transaction(request.canonical)
            .map_err(|_| Error::Refused)?;
        if call.chain_id != u64::from(self.state.network) || digest != call.signing_hash() {
            return Err(Error::Integrity);
        }
        self.capacity(
            Some(request.binding),
            request
                .canonical
                .len()
                .saturating_mul(4)
                .saturating_add(2048),
        )?;
        self.evm_attestors
            .get_mut(&hex(&request.binding))
            .ok_or(Error::Unavailable)?
            .refresh(fresh(clock)?)?;
        reserve_nonce(
            &mut self.state.nonces,
            request.from,
            call.chain_id,
            call.nonce,
            digest,
        )?;
        self.persist()?;
        self.evm_attestors
            .get(&hex(&request.binding))
            .ok_or(Error::Unavailable)?
            .signer
            .sign_evm_transaction_bounded(
                request.canonical,
                call.chain_id,
                Some(deadline.remaining(clock).map_err(|_| Error::Unavailable)?),
            )
            .map(|mut signature| {
                // LXKP Sign carries r || s || v; the attestor returns parity 0/1.
                signature[64] += 27;
                signature.to_vec()
            })
            .map_err(|error| attestor_error(&error))
    }
    fn export_primary(&mut self, request: &Request<'_>) -> Result<Vec<u8>> {
        let ceremony: crate::export::ExportCeremony =
            serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
        ceremony.validate().map_err(|_| Error::Refused)?;
        let record = self
            .state
            .records
            .get_mut(&hex(&request.binding))
            .ok_or(Error::NotFound)?;
        if record.class != request.class || request.reference != record.handle {
            return Err(Error::Integrity);
        }
        if record.class != 1 {
            return Err(Error::Refused);
        }
        if let Some(pending) = &record.export {
            if pending.ceremony != ceremony {
                return Err(Error::Conflict);
            }
            return serde_json::to_vec(pending.envelope.as_ref().ok_or(Error::SelfCustodied)?)
                .map_err(|_| Error::Integrity);
        }
        if record.self_custodied {
            return Err(Error::SelfCustodied);
        }
        let (envelope, proof_hash) = crate::export::seal(
            &self.state.provider,
            request.binding,
            request.reference,
            ceremony.clone(),
            record.seed.as_ref().ok_or(Error::NotFound)?,
        )
        .map_err(|_| Error::Refused)?;
        let response = serde_json::to_vec(&envelope).map_err(|_| Error::Integrity)?;
        record.export = Some(PendingExport {
            envelope: Some(envelope),
            ceremony,
            proof_hash,
        });
        record.self_custodied = true;
        record.seed.zeroize();
        record.seed = None;
        record.wallet = None;
        record.send_actions.clear();
        self.persist()?;
        Ok(response)
    }
    fn acknowledge_export(&mut self, request: &Request<'_>) -> Result<Vec<u8>> {
        let ack: crate::export::ExportAcknowledgement =
            serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
        let record = self
            .state
            .records
            .get_mut(&hex(&request.binding))
            .ok_or(Error::NotFound)?;
        if record.class != request.class || request.reference != record.handle {
            return Err(Error::Integrity);
        }
        let pending = record.export.as_mut().ok_or(Error::NotFound)?;
        let hash: [u8; 32] = Sha256::digest(ack.proof).into();
        if pending.ceremony != ack.ceremony
            || !layerx_crypto::ct::eq_fixed(&pending.proof_hash, &hash)
        {
            return Err(Error::Refused);
        }
        pending.envelope = None;
        self.persist()?;
        Ok(Vec::new())
    }
    fn authorize_send(
        &mut self,
        request: &Request<'_>,
        digest: [u8; 32],
        now: u64,
        clock: &dyn layerx_types::clock::Clock,
        deadline: &mut layerx_types::clock::Deadline,
    ) -> Result<Vec<u8>> {
        let authorization: crate::send::SendAuthorization =
            serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
        let reservation: [u8; 32] =
            Sha256::digest(serde_json::to_vec(&authorization).map_err(|_| Error::Integrity)?)
                .into();
        if let Some(transaction) = self.previous_send(request, &authorization, reservation, now)? {
            return Ok(transaction);
        }
        self.capacity(
            Some(request.binding),
            request.evm.len().saturating_mul(3).saturating_add(4096),
        )?;
        self.evm_attestors
            .get_mut(&hex(&request.binding))
            .ok_or(Error::Unavailable)?
            .refresh(fresh(clock)?)?;
        let record = self
            .state
            .records
            .get_mut(&hex(&request.binding))
            .ok_or(Error::NotFound)?;
        if record.class != request.class || request.reference != record.handle {
            return Err(Error::Refused);
        }
        if record.self_custodied {
            return Err(Error::SelfCustodied);
        }
        record.seed.as_ref().ok_or(Error::NotFound)?;
        let authorization: crate::send::SendAuthorization =
            serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
        if crate::send::digest(&authorization, request.binding, self.state.network, 3, now)?
            != digest
        {
            return Err(Error::Integrity);
        }
        let key = hex(&authorization.authorization.action_key);
        if let Some((previous, transaction)) = record.send_actions.get(&key) {
            return if previous == &authorization {
                Ok(transaction.clone())
            } else {
                Err(Error::Conflict)
            };
        }
        if record.send_actions.len() >= 4096 {
            return Err(Error::Unavailable);
        }
        let attestor = self
            .evm_attestors
            .get(&hex(&request.binding))
            .ok_or(Error::Unavailable)?;
        let wallet = record.wallet.as_mut().ok_or(Error::NotFound)?;
        attestor.validate_wallet(wallet)?;
        validate_authorization(
            &authorization.authorization,
            wallet,
            request.binding,
            self.state.network,
        )?;
        let mut candidate = wallet.clone();
        candidate.authorize(authorization.authorization.clone(), fresh(clock)?)?;
        let transaction = &authorization.authorization.transaction;
        reserve_nonce(
            &mut self.state.nonces,
            wallet.address,
            transaction.chain_id,
            transaction.nonce,
            crate::evm::transaction_digest(transaction),
        )?;
        *wallet = candidate;
        record.send_reservations.insert(key.clone(), reservation);
        // Commit authorization and the address-wide nonce before the remote signer can act.
        self.persist()?;
        let record = self
            .state
            .records
            .get_mut(&hex(&request.binding))
            .ok_or(Error::NotFound)?;
        let wallet = record.wallet.as_mut().ok_or(Error::NotFound)?;
        let attestor = self
            .evm_attestors
            .get(&hex(&request.binding))
            .ok_or(Error::Unavailable)?;
        let action = wallet.sign(
            &authorization.authorization.action_key,
            clock,
            u64::from(self.state.network),
            &attestor.signer,
            deadline,
        )?;
        // The wallet action contains the exact signed bytes; the reservation also binds the disclosure.
        self.persist()?;
        Ok(action.raw_transaction)
    }

    fn previous_send(
        &self,
        request: &Request<'_>,
        authorization: &crate::send::SendAuthorization,
        reservation: [u8; 32],
        now: u64,
    ) -> Result<Option<Vec<u8>>> {
        let key = hex(&authorization.authorization.action_key);
        if let Some(record) = self.state.records.get(&hex(&request.binding)) {
            if record.class != request.class || request.reference != record.handle {
                return Err(Error::Refused);
            }
            if record
                .send_reservations
                .get(&key)
                .is_some_and(|previous| *previous != reservation)
            {
                return Err(Error::Conflict);
            }
            if let Some(action) = record
                .wallet
                .as_ref()
                .and_then(|wallet| wallet.actions.get(&key))
            {
                if !action.raw_transaction.is_empty() {
                    if action.authorization != authorization.authorization {
                        return Err(Error::Conflict);
                    }
                    crate::send::digest(
                        authorization,
                        request.binding,
                        self.state.network,
                        3,
                        now,
                    )?;
                    return Ok(Some(action.raw_transaction.clone()));
                }
            }
            if let Some(action) =
                self.archived(request.binding, authorization.authorization.action_key)?
            {
                if action.authorization != authorization.authorization {
                    return Err(Error::Conflict);
                }
                crate::send::digest(authorization, request.binding, self.state.network, 3, now)?;
                return Ok(Some(action.raw_transaction));
            }
        }
        Ok(None)
    }
    fn evm(
        &mut self,
        request: &Request<'_>,
        clock: &dyn layerx_types::clock::Clock,
        deadline: &mut layerx_types::clock::Deadline,
    ) -> Result<Vec<u8>> {
        let existing = self
            .state
            .records
            .get(&hex(&request.binding))
            .ok_or(Error::NotFound)?;
        if existing.class != request.class || request.reference != existing.handle {
            return Err(Error::Refused);
        }
        if existing.self_custodied {
            return Err(Error::SelfCustodied);
        }
        if let Some(response) = self.archived_evm(request, clock)? {
            return Ok(response);
        }
        if request.operation == 8 {
            let key: [u8; 32] = serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
            let action = existing
                .wallet
                .as_ref()
                .and_then(|wallet| wallet.actions.get(&hex(&key)))
                .ok_or(Error::NotFound)?;
            let extra = action
                .authorization
                .transaction
                .calldata
                .len()
                .saturating_add(256)
                .saturating_mul(4);
            self.capacity(Some(request.binding), extra)?;
            self.evm_attestors
                .get_mut(&hex(&request.binding))
                .ok_or(Error::Unavailable)?
                .refresh(fresh(clock)?)?;
        }
        let record = self
            .state
            .records
            .get_mut(&hex(&request.binding))
            .ok_or(Error::NotFound)?;
        if record.class != request.class || request.reference != record.handle {
            return Err(Error::Refused);
        }
        if record.self_custodied {
            return Err(Error::SelfCustodied);
        }
        if record.seed.is_none() {
            return Err(Error::NotFound);
        }
        if request.operation == 6 {
            if record.wallet.is_none() {
                let attestor = self
                    .evm_attestors
                    .get(&hex(&request.binding))
                    .ok_or(Error::Unavailable)?;
                let mut wallet = crate::evm::Wallet::create(&attestor.key_id, attestor.address)?;
                wallet.tenant.clone_from(&attestor.tenant);
                wallet.principal.clone_from(&attestor.principal);
                wallet
                    .assertion_subject
                    .clone_from(&attestor.assertion_subject);
                record.wallet = Some(wallet);
            }
            let address = record.wallet.as_ref().ok_or(Error::Integrity)?.address;
            self.persist()?;
            return Ok(address.to_vec());
        }
        let action = self.execute_evm_action(request, clock, deadline)?;
        if request.operation == 9 {
            self.archive(request.binding, &action)?;
            let record = self
                .state
                .records
                .get_mut(&hex(&request.binding))
                .ok_or(Error::NotFound)?;
            let wallet = record.wallet.as_mut().ok_or(Error::NotFound)?;
            let key = hex(&action.authorization.action_key);
            wallet
                .archived
                .insert(key.clone(), authorization_hash(&action.authorization)?);
            wallet.actions.remove(&key);
            record.send_actions.remove(&key);
        }
        let response = serde_json::to_vec(&action).map_err(|_| Error::Integrity)?;
        self.persist()?;
        Ok(response)
    }

    fn archived_evm(
        &self,
        request: &Request<'_>,
        clock: &dyn layerx_types::clock::Clock,
    ) -> Result<Option<Vec<u8>>> {
        use crate::evm_types::{EvmAcknowledgement, EvmPlanAuthorization};
        if !matches!(request.operation, 7..=10 | 12) {
            return Ok(None);
        }
        let key = match request.operation {
            7 => {
                serde_json::from_slice::<EvmPlanAuthorization>(request.evm)
                    .map_err(|_| Error::Refused)?
                    .action_key
            }
            9 => {
                serde_json::from_slice::<EvmAcknowledgement>(request.evm)
                    .map_err(|_| Error::Refused)?
                    .action_key
            }
            12 => {
                serde_json::from_slice::<crate::evm_types::EvmExternalSignature>(request.evm)
                    .map_err(|_| Error::Refused)?
                    .action_key
            }
            _ => serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?,
        };
        let Some(action) = self.archived(request.binding, key)? else {
            return Ok(None);
        };
        if request.operation == 7
            && serde_json::from_slice::<EvmPlanAuthorization>(request.evm)
                .map_err(|_| Error::Refused)?
                != action.authorization
        {
            return Err(Error::Conflict);
        }
        if request.operation == 9
            && serde_json::from_slice::<EvmAcknowledgement>(request.evm)
                .map_err(|_| Error::Refused)?
                .transaction_hash
                != action.transaction_hash.ok_or(Error::Integrity)?
        {
            return Err(Error::Integrity);
        }
        if request.operation == 12 {
            let imported: crate::evm_types::EvmExternalSignature =
                serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
            let mut wallet = self
                .state
                .records
                .get(&hex(&request.binding))
                .and_then(|record| record.wallet.as_ref())
                .ok_or(Error::NotFound)?
                .clone();
            wallet.actions.insert(hex(&key), action.clone());
            wallet.accept_external_with_clock(&key, &imported.signature, clock)?;
        }
        serde_json::to_vec(&action)
            .map(Some)
            .map_err(|_| Error::Integrity)
    }

    fn execute_evm_action(
        &mut self,
        request: &Request<'_>,
        clock: &dyn layerx_types::clock::Clock,
        deadline: &mut layerx_types::clock::Deadline,
    ) -> Result<crate::evm_types::EvmAction> {
        use crate::evm_types::{EvmAcknowledgement, EvmPlanAuthorization};
        let record = self
            .state
            .records
            .get_mut(&hex(&request.binding))
            .ok_or(Error::NotFound)?;
        let wallet = record.wallet.as_mut().ok_or(Error::NotFound)?;
        self.evm_attestors
            .get(&hex(&request.binding))
            .ok_or(Error::Unavailable)?
            .validate_wallet(wallet)?;
        match request.operation {
            7 => {
                let authorization: EvmPlanAuthorization =
                    serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
                validate_authorization(
                    &authorization,
                    wallet,
                    request.binding,
                    self.state.network,
                )?;
                let digest = crate::evm::transaction_digest(&authorization.transaction);
                let mut candidate = wallet.clone();
                let action = candidate.authorize(authorization, fresh(clock)?)?;
                let transaction = &action.authorization.transaction;
                reserve_nonce(
                    &mut self.state.nonces,
                    wallet.address,
                    transaction.chain_id,
                    transaction.nonce,
                    digest,
                )?;
                *wallet = candidate;
                Ok(action)
            }
            8 => {
                let key: [u8; 32] =
                    serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
                let attestor = self
                    .evm_attestors
                    .get(&hex(&request.binding))
                    .ok_or(Error::Unavailable)?;
                attestor.validate_wallet(wallet)?;
                wallet.sign(
                    &key,
                    clock,
                    u64::from(self.state.network),
                    &attestor.signer,
                    deadline,
                )
            }
            9 => {
                let ack: EvmAcknowledgement =
                    serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
                let action = wallet
                    .actions
                    .get_mut(&hex(&ack.action_key))
                    .ok_or(Error::NotFound)?;
                if action.transaction_hash != Some(ack.transaction_hash) {
                    return Err(Error::Integrity);
                }
                action.acknowledged = true;
                Ok(action.clone())
            }
            10 => {
                let key: [u8; 32] =
                    serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
                wallet
                    .actions
                    .get(&hex(&key))
                    .ok_or(Error::NotFound)
                    .cloned()
            }
            12 => {
                let request: crate::evm_types::EvmExternalSignature =
                    serde_json::from_slice(request.evm).map_err(|_| Error::Refused)?;
                wallet.accept_external_with_clock(&request.action_key, &request.signature, clock)
            }
            _ => Err(Error::Refused),
        }
    }
    fn create(&mut self, request: &Request<'_>) -> Result<Vec<u8>> {
        let key = hex(&request.binding);
        if let Some(record) = self.state.records.get(&key) {
            if record.class != request.class {
                return Err(Error::Integrity);
            }
            if record.seed.is_none() || record.generation != 0 {
                return Err(Error::Conflict);
            }
            return description(record);
        }
        if self.state.records.len() >= 4096 {
            return Err(Error::Unavailable);
        }
        let seed = new_seed()?;
        let public = public(&seed)?;
        let mut handle = [0; 32];
        getrandom::fill(&mut handle).map_err(|_| Error::Unavailable)?;
        if handle == [0; 32] || self.state.records.values().any(|r| r.handle == handle) {
            return Err(Error::Unavailable);
        }
        let record = Record {
            binding: request.binding,
            class: request.class,
            handle,
            public,
            seed: Some(*seed),
            previous: None,
            generation: 0,
            wallet: None,
            send_actions: BTreeMap::new(),
            recipient_identity: None,
            send_reservations: BTreeMap::new(),
            self_custodied: false,
            export: None,
        };
        let response = description(&record)?;
        self.state.records.insert(key, record);
        self.persist()?;
        Ok(response)
    }
    fn rotate(&mut self, key: &str, expected: [u8; 32]) -> Result<Vec<u8>> {
        let record = self.state.records.get_mut(key).ok_or(Error::NotFound)?;
        if record.previous == Some(expected) {
            return description(record);
        }
        if record.public != expected {
            return Err(Error::Conflict);
        }
        let seed = new_seed()?;
        let public = public(&seed)?;
        if public == record.public {
            return Err(Error::Unavailable);
        }
        let generation = record.generation.checked_add(1).ok_or(Error::Unavailable)?;
        record.previous = Some(record.public);
        record.public = public;
        record.seed.zeroize();
        record.seed = Some(*seed);
        record.generation = generation;
        let response = description(record)?;
        self.persist()?;
        Ok(response)
    }
}
fn validate(state: &State, config: &Config) -> std::result::Result<(), String> {
    if state.version != 1
        || state.provider != config.provider
        || u64::from(state.network) != config.chain.chain_id
        || state.records.len() > 4096
    {
        return Err("state policy mismatch".into());
    }
    for (account, nonces) in &state.nonces {
        let Some((chain, address)) = account.split_once(':') else {
            return Err("nonce account invalid".into());
        };
        if chain != config.chain.chain_id.to_string()
            || address.len() != 40
            || !address
                .bytes()
                .all(|byte| byte.is_ascii_digit() || (b'a'..=b'f').contains(&byte))
            || nonces.requests.len() > 4096
            || nonces
                .requests
                .iter()
                .any(|(nonce, digest)| *nonce >= nonces.next || *digest == [0; 32])
        {
            return Err("nonce state invariant failed".into());
        }
    }
    let mut handles = std::collections::BTreeSet::new();
    for (key, record) in &state.records {
        if key != &hex(&record.binding)
            || record.binding == [0; 32]
            || !matches!(record.class, 1 | 2)
            || record.handle == [0; 32]
            || record.public == [0; 32]
            || !handles.insert(record.handle)
            || (record.generation == 0) != record.previous.is_none()
            || record.previous == Some(record.public)
            || record.recipient_identity == Some([0; 32])
            || (record.self_custodied
                && (record.class != 1
                    || record.seed.is_some()
                    || record.wallet.is_some()
                    || !record.send_actions.is_empty()))
        {
            return Err("state invariant failed".into());
        }
        if let Some(pending) = &record.export {
            if !record.self_custodied
                || pending.ceremony.validate().is_err()
                || pending.proof_hash == [0; 32]
                || pending.envelope.as_ref().is_some_and(|envelope| {
                    envelope.ceremony != pending.ceremony || envelope.ciphertext.len() != 80
                })
            {
                return Err("export state invariant failed".into());
            }
        }
        if let Some(wallet) = &record.wallet {
            if record.seed.is_none() {
                return Err("destroyed wallet invariant failed".into());
            }
            wallet
                .validate(record.binding)
                .map_err(|_| "wallet state invariant failed")?;
        }
        if let Some(seed) = &record.seed {
            if public(seed).map_err(|_| "state key invalid")? != record.public {
                return Err("state key binding failed".into());
            }
        }
    }
    Ok(())
}

fn attestor_error(error: &crate::attestor::AttestorError) -> Error {
    match error {
        crate::attestor::AttestorError::Refused { code, .. } if code == "assertion.expired" => {
            Error::Expired
        }
        crate::attestor::AttestorError::Refused { .. } => Error::Refused,
        crate::attestor::AttestorError::SignatureInvalid { .. }
        | crate::attestor::AttestorError::MalformedResponse { .. } => Error::Integrity,
        _ => Error::Unavailable,
    }
}
fn validate_authorization(
    authorization: &crate::evm_types::EvmPlanAuthorization,
    wallet: &crate::evm::Wallet,
    binding: [u8; 32],
    network: u32,
) -> Result<()> {
    if authorization.binding_digest != binding
        || authorization.tenant != wallet.tenant
        || authorization.principal != wallet.principal
        || authorization.transaction.chain_id != u64::from(network)
    {
        return Err(Error::Refused);
    }
    Ok(())
}

fn fresh(clock: &dyn layerx_types::clock::Clock) -> Result<u64> {
    clock
        .sample(std::time::Duration::from_secs(1))
        .map(layerx_types::clock::ClockReading::unix_seconds)
        .map_err(|_| Error::Unavailable)
}
fn authorization_hash(authorization: &crate::evm_types::EvmPlanAuthorization) -> Result<[u8; 32]> {
    Ok(Sha256::digest(serde_json::to_vec(authorization).map_err(|_| Error::Integrity)?).into())
}
