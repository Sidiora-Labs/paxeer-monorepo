use super::{unreadable, Entry, JournalError, Key, State};
use crate::quote::Word;
use layerx_platform_internal::seal::hmac_sha256;
use serde::{Deserialize, Serialize};
use sha3::{Digest as _, Keccak256};
use std::fs::{File, OpenOptions};
use std::io::{BufRead as _, BufReader, Read as _, Seek as _, SeekFrom, Write as _};
use std::os::unix::fs::{MetadataExt as _, OpenOptionsExt as _};
use std::path::{Path, PathBuf};
use zeroize::Zeroizing;

pub(super) const ROTATE_RECORDS: usize = 128;
pub(super) const MAX_WORKING_RECORDS: usize = 4096;
const MAX_SNAPSHOT_BYTES: usize = 32 * 1024 * 1024;
const MAX_INDEX_KEYS: u64 = 100_000_000;
const KEY_BYTES: u64 = 52;

pub(super) mod map {
    use serde::{Deserialize, Deserializer, Serialize, Serializer};
    use std::collections::BTreeMap;
    pub fn serialize<K: Serialize, V: Serialize, S: Serializer>(
        map: &BTreeMap<K, V>,
        serializer: S,
    ) -> Result<S::Ok, S::Error> {
        map.iter().collect::<Vec<_>>().serialize(serializer)
    }
    pub fn deserialize<
        'de,
        K: Deserialize<'de> + Ord,
        V: Deserialize<'de>,
        D: Deserializer<'de>,
    >(
        deserializer: D,
    ) -> Result<BTreeMap<K, V>, D::Error> {
        let entries = Vec::<(K, V)>::deserialize(deserializer)?;
        let length = entries.len();
        let result: BTreeMap<_, _> = entries.into_iter().collect();
        if length != result.len() {
            return Err(serde::de::Error::custom("duplicate snapshot key"));
        }
        Ok(result)
    }
}

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct Snapshot {
    version: u32,
    state: State,
    index: String,
    index_count: u64,
    index_digest: Word,
}
#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct Authenticated {
    sequence: u64,
    previous: Word,
    payload: String,
    tag: Word,
}

fn protected(path: &Path, create: bool, append: bool) -> Result<File, JournalError> {
    let mut options = OpenOptions::new();
    options
        .read(true)
        .write(true)
        .create(create)
        .append(append)
        .mode(0o600);
    #[cfg(target_os = "linux")]
    options.custom_flags(0x20000);
    let file = options.open(path).map_err(|_| JournalError::Io)?;
    let metadata = file.metadata().map_err(|_| JournalError::Io)?;
    let owner = std::fs::metadata("/proc/self")
        .map_err(|_| JournalError::Io)?
        .uid();
    if !metadata.is_file()
        || metadata.mode() & 0o077 != 0
        || metadata.nlink() != 1
        || metadata.uid() != owner
    {
        return Err(JournalError::Corrupt);
    }
    Ok(file)
}
fn sync_parent(path: &Path) -> Result<(), JournalError> {
    File::open(
        path.parent()
            .filter(|p| !p.as_os_str().is_empty())
            .unwrap_or(Path::new(".")),
    )
    .and_then(|file| file.sync_all())
    .map_err(|_| JournalError::Io)
}
fn side(path: &Path, suffix: &str) -> PathBuf {
    let mut name = path.as_os_str().to_os_string();
    name.push(suffix);
    PathBuf::from(name)
}
fn fresh(path: &Path) -> Result<File, JournalError> {
    let mut options = OpenOptions::new();
    options.read(true).write(true).create_new(true).mode(0o600);
    options.open(path).map_err(|_| JournalError::Io)
}
fn random() -> Result<String, JournalError> {
    layerx_platform_internal::secret::random_hex(16)
        .map(|value| value.to_string())
        .map_err(|_| JournalError::Io)
}
fn key_bytes(key: Key) -> [u8; 52] {
    let mut bytes = [0; 52];
    bytes[..20].copy_from_slice(&key.sponsor);
    bytes[20..].copy_from_slice(&key.quote_nonce);
    bytes
}
fn read_key(file: &mut File) -> Result<Option<[u8; 52]>, JournalError> {
    let mut bytes = [0; 52];
    let count = file.read(&mut bytes[..1]).map_err(|_| JournalError::Io)?;
    if count == 0 {
        return Ok(None);
    }
    file.read_exact(&mut bytes[1..])
        .map_err(|_| JournalError::Corrupt)?;
    Ok(Some(bytes))
}

pub(super) struct Storage {
    path: PathBuf,
    file: File,
    _lock: File,
    key: Zeroizing<Word>,
    sequence: u64,
    head: Word,
    index: Option<(PathBuf, File, u64)>,
    pub(super) tail_records: usize,
}
impl Storage {
    pub(super) fn open(path: &Path) -> Result<(Self, State), JournalError> {
        let absolute = std::path::absolute(path).map_err(|_| JournalError::Io)?;
        let path = absolute.as_path();
        let lock = protected(&side(path, ".lock"), true, false)?;
        lock.try_lock().map_err(|_| JournalError::Locked)?;
        let file = protected(path, true, true)?;
        file.try_lock().map_err(|_| JournalError::Locked)?;
        let key = Self::load_or_create_key(path, &file)?;
        let mut storage = Self {
            path: path.to_owned(),
            file,
            _lock: lock,
            key,
            sequence: 0,
            head: [0; 32],
            index: None,
            tail_records: 0,
        };
        let mut state = State::default();
        let mut reader = BufReader::new(storage.file.try_clone().map_err(|_| JournalError::Io)?);
        reader
            .seek(SeekFrom::Start(0))
            .map_err(|_| JournalError::Io)?;
        let mut first = true;
        let mut authenticated = false;
        let mut records = 0;
        let mut last_previous_seal = None;
        loop {
            let mut line = Vec::new();
            let count = reader
                .by_ref()
                .take((MAX_SNAPSHOT_BYTES + 1) as u64)
                .read_until(b'\n', &mut line)
                .map_err(|_| JournalError::Io)?;
            if count == 0 {
                break;
            }
            if count > MAX_SNAPSHOT_BYTES || line.last() != Some(&b'\n') {
                return Err(JournalError::Corrupt);
            }
            let value: serde_json::Value =
                serde_json::from_slice(&line).map_err(|_| unreadable(&line))?;
            if value["tag"].is_null() {
                if authenticated || line.len() > 4_194_304 {
                    return Err(JournalError::Corrupt);
                }
                let entry = serde_json::from_value(value).map_err(|_| unreadable(&line))?;
                state.apply(&entry)?;
                storage.tail_records += 1;
            } else {
                let record: Authenticated =
                    serde_json::from_value(value).map_err(|_| JournalError::Corrupt)?;
                last_previous_seal = Some(storage.head_seal());
                let payload = record.payload.as_bytes();
                let tag = storage.tag(record.sequence, &record.previous, payload);
                if !constant_eq(&tag, &record.tag) || record.previous != storage.head {
                    return Err(JournalError::Corrupt);
                }
                if first && record.sequence == 0 {
                    state = storage.apply_snapshot(path, &record.payload)?;
                } else {
                    if record.sequence
                        != storage
                            .sequence
                            .checked_add(1)
                            .ok_or(JournalError::Corrupt)?
                    {
                        return Err(JournalError::Corrupt);
                    }
                    let entry =
                        serde_json::from_str(&record.payload).map_err(|_| JournalError::Corrupt)?;
                    state.apply(&entry)?;
                    storage.tail_records += 1;
                }
                storage.sequence = record.sequence;
                storage.head = record.tag;
                authenticated = true;
            }
            first = false;
            records += 1;
            if state.working_records() > MAX_WORKING_RECORDS
                || (authenticated && records > ROTATE_RECORDS + 2)
            {
                return Err(JournalError::Corrupt);
            }
        }
        storage.verify_head(path, authenticated, last_previous_seal)?;
        storage.file.sync_all().map_err(|_| JournalError::Io)?;
        sync_parent(path)?;
        storage.remove_orphans()?;
        Ok((storage, state))
    }
    fn load_or_create_key(path: &Path, file: &File) -> Result<Zeroizing<Word>, JournalError> {
        let key_path = side(path, ".key");
        let key = if key_path.exists() {
            let mut key_file = protected(&key_path, false, false)?;
            if key_file.metadata().map_err(|_| JournalError::Io)?.len() != 32 {
                return Err(JournalError::Corrupt);
            }
            let mut key = Zeroizing::new([0; 32]);
            key_file
                .read_exact(key.as_mut())
                .map_err(|_| JournalError::Io)?;
            key
        } else {
            // Legacy journals have plain entries. An authenticated journal never recreates its key.
            let mut reader = BufReader::new(file);
            let mut first = Vec::new();
            reader
                .by_ref()
                .take((MAX_SNAPSHOT_BYTES + 1) as u64)
                .read_until(b'\n', &mut first)
                .map_err(|_| JournalError::Io)?;
            if serde_json::from_slice::<serde_json::Value>(&first)
                .ok()
                .is_some_and(|v| !v["tag"].is_null())
            {
                return Err(JournalError::Corrupt);
            }
            let random =
                layerx_platform_internal::secret::random_hex(32).map_err(|_| JournalError::Io)?;
            let decoded = crate::rpc::bytes(&format!("0x{}", random.as_str()))
                .map_err(|_| JournalError::Io)?;
            let key: Zeroizing<Word> =
                Zeroizing::new(decoded.try_into().map_err(|_| JournalError::Io)?);
            let mut key_file = fresh(&key_path)?;
            key_file
                .write_all(key.as_ref())
                .and_then(|()| key_file.sync_all())
                .map_err(|_| JournalError::Io)?;
            sync_parent(path)?;
            key
        };
        Ok(key)
    }

    fn apply_snapshot(&mut self, path: &Path, payload: &str) -> Result<State, JournalError> {
        let snapshot: Snapshot =
            serde_json::from_str(payload).map_err(|_| JournalError::Corrupt)?;
        if snapshot.version != 1
            || snapshot.index.len() != 32
            || !snapshot.index.bytes().all(|b| b.is_ascii_hexdigit())
            || snapshot.index_count > MAX_INDEX_KEYS
        {
            return Err(JournalError::Corrupt);
        }
        let index_path = side(path, &format!(".replay.{}", snapshot.index));
        let mut index = protected(&index_path, false, false)?;
        if index.metadata().map_err(|_| JournalError::Io)?.len() != snapshot.index_count * KEY_BYTES
        {
            return Err(JournalError::Corrupt);
        }
        let mut hash = Keccak256::new();
        let mut previous = None;
        while let Some(key) = read_key(&mut index)? {
            if previous.is_some_and(|prior| prior >= key) {
                return Err(JournalError::Corrupt);
            }
            hash.update(key);
            previous = Some(key);
        }
        let digest: Word = hash.finalize().into();
        if digest != snapshot.index_digest {
            return Err(JournalError::Corrupt);
        }
        self.index = Some((index_path, index, snapshot.index_count));
        Ok(snapshot.state)
    }

    fn verify_head(
        &mut self,
        path: &Path,
        authenticated: bool,
        last_previous_seal: Option<Word>,
    ) -> Result<(), JournalError> {
        let head_path = side(path, ".head");
        if head_path.exists() {
            let mut file = protected(&head_path, false, false)?;
            let length = file.metadata().map_err(|_| JournalError::Io)?.len();
            if length != 32 && length != 64 {
                return Err(JournalError::Corrupt);
            }
            let mut heads = vec![0; usize::try_from(length).map_err(|_| JournalError::Corrupt)?];
            file.read_exact(&mut heads).map_err(|_| JournalError::Io)?;
            let current = self.head_seal();
            if !heads.chunks_exact(32).any(|head| {
                head == current || last_previous_seal.is_some_and(|previous| head == previous)
            }) {
                return Err(JournalError::Corrupt);
            }
            self.save_head()?;
        } else if authenticated {
            return Err(JournalError::Corrupt);
        } else {
            self.save_head()?;
        }
        Ok(())
    }

    fn remove_orphans(&self) -> Result<(), JournalError> {
        let parent = self
            .path
            .parent()
            .filter(|path| !path.as_os_str().is_empty())
            .unwrap_or(Path::new("."));
        let name = self
            .path
            .file_name()
            .and_then(|name| name.to_str())
            .ok_or(JournalError::Io)?;
        for entry in std::fs::read_dir(parent).map_err(|_| JournalError::Io)? {
            let entry = entry.map_err(|_| JournalError::Io)?;
            let filename = entry.file_name();
            let Some(filename) = filename.to_str() else {
                continue;
            };
            let ours = [".replay.", ".snapshot.", ".head."].iter().any(|suffix| {
                filename
                    .strip_prefix(&format!("{name}{suffix}"))
                    .is_some_and(|generation| {
                        generation.len() == 32
                            && generation.bytes().all(|byte| byte.is_ascii_hexdigit())
                    })
            });
            if ours
                && self
                    .index
                    .as_ref()
                    .is_none_or(|(path, _, _)| *path != entry.path())
            {
                // Never follow an orphan path supplied as a symlink or hard link.
                let _file = protected(&entry.path(), false, false)?;
                std::fs::remove_file(entry.path()).map_err(|_| JournalError::Io)?;
            }
        }
        sync_parent(&self.path)
    }
    fn head_seal(&self) -> Word {
        hmac_sha256(
            &self.key,
            &[
                b"gas-journal-head-v1",
                &self.sequence.to_be_bytes(),
                &self.head,
            ],
        )
    }
    fn save_head(&self) -> Result<(), JournalError> {
        self.write_head(&self.head_seal())
    }
    fn write_head(&self, seals: &[u8]) -> Result<(), JournalError> {
        let temporary = side(&self.path, &format!(".head.{}", random()?));
        let mut file = fresh(&temporary)?;
        file.write_all(seals)
            .and_then(|()| file.sync_all())
            .map_err(|_| JournalError::Io)?;
        std::fs::rename(temporary, side(&self.path, ".head")).map_err(|_| JournalError::Io)?;
        sync_parent(&self.path)
    }
    fn tag(&self, sequence: u64, previous: &Word, payload: &[u8]) -> Word {
        hmac_sha256(
            &self.key,
            &[
                b"gas-journal-v1",
                &sequence.to_be_bytes(),
                previous,
                payload,
            ],
        )
    }
    fn encoded(
        &self,
        sequence: u64,
        previous: Word,
        payload: String,
    ) -> Result<(Vec<u8>, Word), JournalError> {
        let bytes = payload.as_bytes();
        let tag = self.tag(sequence, &previous, bytes);
        let mut line = serde_json::to_vec(&Authenticated {
            sequence,
            previous,
            payload,
            tag,
        })
        .map_err(|_| JournalError::Corrupt)?;
        line.push(b'\n');
        if line.len() > MAX_SNAPSHOT_BYTES {
            return Err(JournalError::Corrupt);
        }
        Ok((line, tag))
    }
    pub(super) fn append(&mut self, entry: &Entry) -> Result<(), JournalError> {
        let payload = serde_json::to_string(entry).map_err(|_| JournalError::Corrupt)?;
        let sequence = self.sequence.checked_add(1).ok_or(JournalError::Corrupt)?;
        let (line, tag) = self.encoded(sequence, self.head, payload)?;
        if line.len() > 4_194_304 {
            return Err(JournalError::Corrupt);
        }
        self.file
            .write_all(&line)
            .and_then(|()| self.file.sync_all())
            .map_err(|_| JournalError::Io)?;
        self.sequence = sequence;
        self.head = tag;
        self.tail_records += 1;
        self.save_head()?;
        Ok(())
    }
    pub(super) fn contains(&self, key: Key) -> Result<bool, JournalError> {
        use std::os::unix::fs::FileExt as _;
        let Some((_, file, count)) = &self.index else {
            return Ok(false);
        };
        let wanted = key_bytes(key);
        let (mut low, mut high) = (0, *count);
        while low < high {
            let middle = low + (high - low) / 2;
            let mut bytes = [0; 52];
            file.read_exact_at(&mut bytes, middle * KEY_BYTES)
                .map_err(|_| JournalError::Io)?;
            match bytes.cmp(&wanted) {
                std::cmp::Ordering::Equal => return Ok(true),
                std::cmp::Ordering::Less => low = middle + 1,
                std::cmp::Ordering::Greater => high = middle,
            }
        }
        Ok(false)
    }
    pub(super) fn rotate(&mut self, state: &State, retired: &[Key]) -> Result<(), JournalError> {
        let generation = random()?;
        let index_path = side(&self.path, &format!(".replay.{generation}"));
        let mut index = fresh(&index_path)?;
        let mut keys: Vec<_> = retired.iter().copied().map(key_bytes).collect();
        keys.sort_unstable();
        keys.dedup();
        let mut keys = keys.into_iter().peekable();
        let mut old = self
            .index
            .as_ref()
            .map(|(_, file, _)| file.try_clone())
            .transpose()
            .map_err(|_| JournalError::Io)?;
        if let Some(file) = &mut old {
            file.seek(SeekFrom::Start(0))
                .map_err(|_| JournalError::Io)?;
        }
        let mut saved = match &mut old {
            Some(file) => read_key(file)?,
            None => None,
        };
        let mut hash = Keccak256::new();
        let mut count = 0;
        while saved.is_some() || keys.peek().is_some() {
            let key = match (saved, keys.peek().copied()) {
                (Some(a), Some(b)) if a >= b => {
                    keys.next();
                    if a == b {
                        saved = match &mut old {
                            Some(file) => read_key(file)?,
                            None => None,
                        };
                    }
                    b
                }
                (Some(a), _) => {
                    saved = match &mut old {
                        Some(file) => read_key(file)?,
                        None => None,
                    };
                    a
                }
                (None, Some(b)) => {
                    keys.next();
                    b
                }
                _ => return Err(JournalError::Corrupt),
            };
            count += 1;
            if count > MAX_INDEX_KEYS {
                return Err(JournalError::Conflict);
            }
            index.write_all(&key).map_err(|_| JournalError::Io)?;
            hash.update(key);
        }
        index.sync_all().map_err(|_| JournalError::Io)?;
        let snapshot = Snapshot {
            version: 1,
            state: state.clone(),
            index: generation.clone(),
            index_count: count,
            index_digest: hash.finalize().into(),
        };
        let (line, tag) = self.encoded(
            0,
            [0; 32],
            serde_json::to_string(&snapshot).map_err(|_| JournalError::Corrupt)?,
        )?;
        let temporary = side(&self.path, &format!(".snapshot.{generation}"));
        let mut file = fresh(&temporary)?;
        file.try_lock().map_err(|_| JournalError::Locked)?;
        file.write_all(&line)
            .and_then(|()| file.sync_all())
            .map_err(|_| JournalError::Io)?;
        sync_parent(&self.path)?;
        // Either complete generation is recoverable if rotation stops between
        // publishing the snapshot and acknowledging its new chain head.
        let next_seal = hmac_sha256(
            &self.key,
            &[b"gas-journal-head-v1", &0u64.to_be_bytes(), &tag],
        );
        self.write_head(&[self.head_seal(), next_seal].concat())?;
        std::fs::rename(&temporary, &self.path).map_err(|_| JournalError::Io)?;
        sync_parent(&self.path)?;
        self.file = file;
        self.sequence = 0;
        self.head = tag;
        self.tail_records = 0;
        self.save_head()?;
        if let Some((previous, _, _)) = self.index.replace((index_path, index, count)) {
            std::fs::remove_file(previous).map_err(|_| JournalError::Io)?;
            sync_parent(&self.path)?;
        }
        Ok(())
    }
}
fn constant_eq(a: &Word, b: &Word) -> bool {
    a.iter()
        .zip(b)
        .fold(0u8, |difference, (a, b)| difference | (a ^ b))
        == 0
}
