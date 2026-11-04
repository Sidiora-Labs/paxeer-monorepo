//! Content-addressed mirror of published program source.
//!
//! A verification request names a source location and a source digest. The
//! mirror is the operator-maintained store the service rebuilds from: it
//! returns an archive only when the archive hashes to exactly the requested
//! digest and was published under exactly the requested location.

use std::fmt::{self, Display};
use std::fs;
use std::io::Write;
use std::path::PathBuf;

use layerx_programs::{hex, BuildPlan, BuildRefusal, PublishedSource, SourceArchive};
use sha2::{Digest as _, Sha256};

use serde::{Deserialize, Serialize};

const ARCHIVE_SUFFIX: &str = "archive";
const PLAN_SUFFIX: &str = "plan";
const URI_SUFFIX: &str = "uri";
const MAX_URI: usize = 512;
const BINDING_SUFFIX: &str = "binding";

#[derive(Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct SourceBinding {
    version: u32,
    archive: String,
    plan: String,
    uri: String,
}

/// Published source, its declared build recipe and the location it was
/// published under.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct MirroredSource {
    pub source: PublishedSource,
    pub plan: BuildPlan,
}

/// Typed refusal produced when the mirror cannot supply verifiable source.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum MirrorRefusal {
    NotMirrored,
    UriMismatch { published: String },
    DigestMismatch { mirrored: [u8; 32] },
    Unreadable { reason: String },
    Plan(BuildRefusal),
}

impl Display for MirrorRefusal {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        match self {
            Self::NotMirrored => {
                formatter.write_str("no published source is mirrored for that digest")
            }
            Self::UriMismatch { published } => {
                write!(formatter, "mirrored source was published at {published}")
            }
            Self::DigestMismatch { mirrored } => write!(
                formatter,
                "mirrored archive hashes to {}",
                hex::encode(mirrored)
            ),
            Self::Unreadable { reason } => write!(formatter, "source mirror unreadable: {reason}"),
            Self::Plan(refusal) => write!(formatter, "published build plan refused: {refusal}"),
        }
    }
}

impl std::error::Error for MirrorRefusal {}

/// Operator-maintained store of published program source.
#[derive(Clone, Debug)]
pub struct SourceMirror {
    root: PathBuf,
    interrupt_before: Option<&'static str>,
}

impl SourceMirror {
    /// Opens, creating the mirror directory when it is absent.
    ///
    /// # Errors
    ///
    /// Returns the filesystem error that prevented the directory from opening.
    pub fn open(root: PathBuf) -> Result<Self, String> {
        fs::create_dir_all(&root)
            .map_err(|error| format!("could not open the source mirror: {error}"))?;
        Ok(Self {
            root,
            interrupt_before: None,
        })
    }

    /// Stops before a persistence boundary, retaining the same on-disk state
    /// as an interrupted publisher. Used to exercise restart recovery.
    #[must_use]
    pub const fn interrupt_before(mut self, step: &'static str) -> Self {
        self.interrupt_before = Some(step);
        self
    }

    /// Publishes one canonical archive with its build recipe and returns the
    /// source digest verification requests must name.
    ///
    /// # Errors
    ///
    /// Refuses malformed locations and returns the filesystem error that
    /// prevented durable persistence.
    pub fn publish(
        &self,
        uri: &str,
        plan: &BuildPlan,
        archive: &SourceArchive,
    ) -> Result<[u8; 32], String> {
        if uri.is_empty()
            || uri.len() > MAX_URI
            || uri
                .bytes()
                .any(|byte| byte.is_ascii_control() || byte.is_ascii_whitespace())
        {
            return Err("published source location is malformed".to_owned());
        }
        let plan_text = plan.encode();
        BuildPlan::parse(&plan_text).map_err(|error| error.to_string())?;
        let bytes = archive.encode();
        let digest: [u8; 32] = Sha256::digest(&bytes).into();
        let lock = fs::OpenOptions::new()
            .create(true)
            .truncate(false)
            .write(true)
            .open(self.path(&digest, "lock"))
            .map_err(|error| format!("could not open the source mirror lock: {error}"))?;
        lock.lock()
            .map_err(|error| format!("could not lock the source mirror: {error}"))?;
        let path = self.path(&digest, BINDING_SUFFIX);
        let exists = path.try_exists().map_err(|error| error.to_string())?;
        let mut legacy = false;
        for suffix in [ARCHIVE_SUFFIX, PLAN_SUFFIX, URI_SUFFIX] {
            legacy |= self
                .path(&digest, suffix)
                .try_exists()
                .map_err(|error| error.to_string())?;
        }
        if exists || legacy {
            let existing = self
                .fetch_committed(uri, digest)
                .map_err(|error| format!("source binding conflicts or is unreadable: {error}"))?;
            if existing.plan != *plan || existing.source.canonical_archive != bytes {
                return Err(
                    "source digest is already bound to another archive or build plan".to_owned(),
                );
            }
            if exists {
                fs::File::open(&self.root)
                    .and_then(|file| file.sync_all())
                    .map_err(|error| format!("could not sync the source mirror: {error}"))?;
                return Ok(digest);
            }
        }
        let binding = serde_json::to_vec(&SourceBinding {
            version: 1,
            archive: hex::encode(&bytes),
            plan: plan_text,
            uri: uri.to_owned(),
        })
        .map_err(|error| error.to_string())?;
        let temporary = self.path(&digest, "binding.tmp");
        self.reach("create_temporary")?;
        let mut file = fs::OpenOptions::new()
            .create(true)
            .truncate(true)
            .write(true)
            .open(&temporary)
            .map_err(|error| error.to_string())?;
        self.reach("write_binding")?;
        file.write_all(&binding)
            .map_err(|error| error.to_string())?;
        self.reach("sync_temporary")?;
        file.sync_all().map_err(|error| error.to_string())?;
        self.reach("commit")?;
        fs::rename(&temporary, &path).map_err(|error| error.to_string())?;
        self.reach("sync_directory")?;
        fs::File::open(&self.root)
            .and_then(|file| file.sync_all())
            .map_err(|error| format!("could not sync the source mirror: {error}"))?;
        Ok(digest)
    }

    /// Returns mirrored source only when it matches the requested location and
    /// digest exactly.
    ///
    /// # Errors
    ///
    /// Refuses absent, unreadable, relocated and altered source, and refuses
    /// published recipes that are not admissible build plans.
    pub fn fetch(&self, uri: &str, digest: [u8; 32]) -> Result<MirroredSource, MirrorRefusal> {
        let lock = match fs::File::open(self.path(&digest, "lock")) {
            Ok(lock) => Some(lock),
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => None,
            Err(error) => {
                return Err(MirrorRefusal::Unreadable {
                    reason: error.to_string(),
                })
            }
        };
        if let Some(lock) = &lock {
            lock.lock_shared()
                .map_err(|error| MirrorRefusal::Unreadable {
                    reason: error.to_string(),
                })?;
        }
        let source = self.fetch_committed(uri, digest)?;
        // A publisher that died after rename may not have synced the directory.
        // Do so before this binding can be used in a durable verification.
        fs::File::open(&self.root)
            .and_then(|file| file.sync_all())
            .map_err(|error| MirrorRefusal::Unreadable {
                reason: error.to_string(),
            })?;
        Ok(source)
    }

    fn fetch_committed(
        &self,
        uri: &str,
        digest: [u8; 32],
    ) -> Result<MirroredSource, MirrorRefusal> {
        let binding = match fs::read(self.path(&digest, BINDING_SUFFIX)) {
            Ok(bytes) => {
                let binding: SourceBinding =
                    serde_json::from_slice(&bytes).map_err(|error| MirrorRefusal::Unreadable {
                        reason: error.to_string(),
                    })?;
                if binding.version != 1 {
                    return Err(MirrorRefusal::Unreadable {
                        reason: "unsupported source binding version".to_owned(),
                    });
                }
                Some(binding)
            }
            Err(error) if error.kind() == std::io::ErrorKind::NotFound => None,
            Err(error) => {
                return Err(MirrorRefusal::Unreadable {
                    reason: error.to_string(),
                })
            }
        };
        let (canonical_archive, published, text) = if let Some(binding) = binding {
            let archive =
                hex::decode(&binding.archive).map_err(|error| MirrorRefusal::Unreadable {
                    reason: error.to_string(),
                })?;
            (archive, binding.uri, binding.plan)
        } else {
            let archive = match fs::read(self.path(&digest, ARCHIVE_SUFFIX)) {
                Ok(bytes) => bytes,
                Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
                    return Err(MirrorRefusal::NotMirrored)
                }
                Err(error) => {
                    return Err(MirrorRefusal::Unreadable {
                        reason: error.to_string(),
                    })
                }
            };
            let uri = fs::read_to_string(self.path(&digest, URI_SUFFIX)).map_err(|error| {
                MirrorRefusal::Unreadable {
                    reason: error.to_string(),
                }
            })?;
            let plan = fs::read_to_string(self.path(&digest, PLAN_SUFFIX)).map_err(|error| {
                MirrorRefusal::Unreadable {
                    reason: error.to_string(),
                }
            })?;
            (archive, uri.trim().to_owned(), plan)
        };
        let mirrored: [u8; 32] = Sha256::digest(&canonical_archive).into();
        if mirrored != digest {
            return Err(MirrorRefusal::DigestMismatch { mirrored });
        }
        if published != uri {
            return Err(MirrorRefusal::UriMismatch { published });
        }
        let plan = BuildPlan::parse(&text).map_err(MirrorRefusal::Plan)?;
        Ok(MirroredSource {
            source: PublishedSource {
                uri: published,
                canonical_archive,
            },
            plan,
        })
    }

    fn path(&self, digest: &[u8; 32], suffix: &str) -> PathBuf {
        self.root.join(format!("{}.{suffix}", hex::encode(digest)))
    }

    fn reach(&self, step: &str) -> Result<(), String> {
        if self.interrupt_before == Some(step) {
            return Err(format!(
                "source mirror publication interrupted before {step}"
            ));
        }
        Ok(())
    }
}
