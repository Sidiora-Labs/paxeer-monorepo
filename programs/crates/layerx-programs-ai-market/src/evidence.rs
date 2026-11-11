//! F09 offchain artifact manifest, publisher envelope, declaration, reproduction,
//! chunk-proof and evidence-manifest codecs with domain-separated SHA256 roots.
//! The codecs are pure: no publication, grant admission, availability or finality is
//! established by any of their values.
//!
//! `SealEvidence` ([`apply`]) is the only F09 protocol mutation. Its bounded seals live in the
//! F09 seal region at the start of the shared control feature bytes ([`SealRegion`]); any later
//! feature bytes belong to other control producers and are carried unchanged. Readings chosen
//! where producers are silent:
//! - The payload names no evaluator. The envelope actor is the evaluator owner; a native call
//!   selects the owner's only frozen evaluator, a delegate call the frozen evaluator whose key
//!   it presents.
//! - The evaluator application sequence is the replay slot `ActorSlot::evaluator(i)` bound to
//!   the evaluator owner; no producer binds those slots yet.
//! - Seals of an earlier epoch read as absent; the next seal of a later opened epoch replaces
//!   them, so no opening has to clear the region (an opening requires its predecessor
//!   terminal).
//! - `EvidenceSealed` carries the 185-byte seal record as its suffix; its result digest is the
//!   result digest of that record.
//! - A mode, task policy or rubric other than the frozen policy's, or a task-set digest other
//!   than the sealed one, refuses `EVIDENCE_BINDING`.
#[cfg(target_arch = "wasm32")]
use crate::codec::Sha256State as Sha256;
use crate::types::GuestIndex as _;
use crate::{
    admission::{AdmissionTable, Participant},
    codec::{self as wire, EventCommon, ReportBody, ValidatedEnvelope},
    commit_reveal::{COMMIT_CLOSES, COMMIT_OPENS},
    dispatch,
    errors::{
        CodecResult, ARITHMETIC, BAD_VERSION, CAPACITY, CONFLICT, EVIDENCE_BINDING, EXPIRED,
        F03_GRANT_VERSION_CONFLICT, F03_KEY_VERSION_CONFLICT, F03_NO_GRANT,
        F09_EVIDENCE_SEAL_CONFLICT, KEY_MISMATCH, NON_CANONICAL, NOT_FOUND, REVOKED, UNAUTHORIZED,
        UNKNOWN_OPERATION, WRONG_EPOCH, WRONG_PHASE,
    },
    evaluators::{
        authority::{evaluator_region, FrozenEvaluator},
        model::{GrantStatus, RegisteredEvidence},
    },
    registry::check_f01_capacity,
    registry_ops::{CallContext, PolicySection},
    rewards::RewardState,
    state::{
        decode_shared_state, encode_shared_state, ActorSlot, HeightWindow, ReplayDecision,
        ReplayRequest, RetainedResult, Section, SharedState,
    },
    tasks,
    types::{Authentication, EvaluatorId, FrozenBinding, Presence, ResultDigest},
    MAX_EVALUATORS,
};
use crate::{
    codec::{domain_hash, Reader, Writer},
    commit_reveal::commitment::{decode_binding, encode_binding, BINDING_BYTES},
    errors::{offchain_error, ApplicationError, OffchainSpace},
    evaluators::{codec::verify_digest, model::VerificationError},
    types::{
        ChainDomain, Digest32, EvaluatorBinding, EvidenceRoot, MarketId, PolicyDigest, PrincipalId,
        ProgramId, PublicKey32, RubricDigest, Score, Signature64, TaskId, Version, WorkerId,
    },
    MAX_TASKS, MAX_WORKERS,
};
#[cfg(not(target_arch = "wasm32"))]
use sha2::{Digest, Sha256};

pub const MAGIC: &[u8; 8] = b"PAXAIF09";
pub const VERSION: u16 = 1;
pub const SIGNATURE_SCHEME_ED25519: u16 = 1;
pub const CHUNK_BYTES: u32 = 262_144;
pub const MAX_CHUNKS: u32 = 131_072;
pub const MAX_OBJECT_BYTES: u64 = 34_359_738_368;
pub const MAX_PARENTS: usize = 16;
pub const MAX_MANIFEST_BYTES: usize = 4_096;
pub const MAX_ENVELOPE_BYTES: usize = 8_192;
pub const MAX_RECORD_BYTES: usize = 16_384;
pub const MAX_DOCUMENTS: usize = 16;
pub const MAX_LABEL_BYTES: usize = 256;
pub const MAX_PROOF_SIBLINGS: usize = 17;
pub const MAX_EVIDENCE_BYTES: usize = 65_536;
pub const MANIFEST_FIXED_BYTES: usize = 386;
pub const PARENT_BYTES: usize = 33;
pub const ENVELOPE_FIXED_BYTES: usize = 110;
pub const DECLARATION_FIXED_BYTES: usize = 97;
pub const DOCUMENT_FIXED_BYTES: usize = 37;
pub const REPRODUCTION_BYTES: usize = 379;
pub const PROOF_FIXED_BYTES: usize = 43;
pub const EVIDENCE_FIXED_BYTES: usize = 373;
pub const WORKER_GROUP_FIXED_BYTES: usize = 144;
pub const TASK_ENTRY_BYTES: usize = 161;

pub const MANIFEST_DOMAIN: &[u8] = b"PAXAI/artifact-manifest/v1\0";
pub const PUBLISHER_DOMAIN: &[u8] = b"PAXAI/artifact-publisher/v1\0";
pub const DECLARATION_DOMAIN: &[u8] = b"PAXAI/artifact-declaration/v1\0";
pub const REPRODUCTION_DOMAIN: &[u8] = b"PAXAI/artifact-reproduction/v1\0";
pub const CHUNK_DOMAIN: &[u8] = b"PAXAI/artifact-chunk/v1\0";
pub const EMPTY_DOMAIN: &[u8] = b"PAXAI/artifact-empty/v1\0";
pub const NODE_DOMAIN: &[u8] = b"PAXAI/artifact-node/v1\0";
pub const CONTENT_DOMAIN: &[u8] = b"PAXAI/artifact-content/v1\0";
pub const EVIDENCE_DOMAIN: &str = "PAXAI/evidence/v1";

/// Typed failures of the independent offchain artifact error space; the
/// discriminant is the 1-based code of `errors::ARTIFACT_SERVICE_ERRORS`.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
#[repr(u16)]
pub enum ArtifactError {
    Malformed = 1,
    UnsupportedVersion,
    UnsupportedKind,
    InvalidContext,
    Unauthorized,
    AuthorityRevoked,
    PurposeDenied,
    Expired,
    QuotaExceeded,
    CapacityUnavailable,
    MissingChunk,
    LengthMismatch,
    RootMismatch,
    SignatureInvalid,
    IdempotencyConflict,
    IntegrityConflict,
    Tombstoned,
    ContentUnavailable,
    UnsafeLocator,
    UnknownDelivery,
    StorageFailure,
}
impl ArtifactError {
    #[must_use]
    pub const fn code(self) -> u16 {
        self as u16
    }
    #[must_use]
    pub fn name(self) -> &'static str {
        offchain_error(OffchainSpace::ArtifactService, self.code()).unwrap_or("Malformed")
    }
}
/// Common reader/writer/identity refusals (noncanonical, bound, overflow,
/// zero identity) are malformed artifact bytes.
impl From<ApplicationError> for ArtifactError {
    fn from(_: ApplicationError) -> Self {
        Self::Malformed
    }
}
pub type ArtifactResult<T> = Result<T, ArtifactError>;

/// Host verification failures stay distinct from typed signature refusal.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum VerificationFailure {
    Artifact(ArtifactError),
    Host(VerificationError),
}
impl From<ArtifactError> for VerificationFailure {
    fn from(error: ArtifactError) -> Self {
        Self::Artifact(error)
    }
}

macro_rules! u8_enum {
    ($name:ident, $unknown:expr, { $($variant:ident = $value:literal),+ $(,)? }) => {
        #[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd)]
        #[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
        #[repr(u8)]
        pub enum $name { $($variant = $value),+ }
        impl $name {
            /// Decodes the wire discriminant.
            ///
            /// # Errors
            #[doc = concat!("Returns `", stringify!($unknown), "` when `value` is not a defined discriminant.")]
            #[cfg_attr(target_arch = "wasm32", inline(never))]
            pub fn decode(value: u8) -> ArtifactResult<Self> {
                const VALUES: &[(u8, $name)] = &[$(($value, $name::$variant)),+];
                VALUES.iter().find(|(code, _)| *code == value)
                    .map(|(_, variant)| *variant).ok_or($unknown)
            }
        }
    };
}
u8_enum!(ArtifactKind, ArtifactError::UnsupportedKind, {
    Model = 1, Dataset = 2, Benchmark = 3, Input = 4, Result = 5, ExecutionEvidence = 6,
    Reproduction = 7, Declaration = 8, EvaluationEvidence = 9,
});
u8_enum!(Privacy, ArtifactError::Malformed, { Public = 0, Encrypted = 1 });
u8_enum!(ParentPurpose, ArtifactError::Malformed, {
    ModelShard = 1, Input = 2, Result = 3, Model = 4, Dataset = 5, Benchmark = 6,
    Execution = 7, Reproduction = 8, ScoreEvidence = 9, Declaration = 10,
});
u8_enum!(RightsStatus, ArtifactError::Malformed, { Undeclared = 0, Declared = 1, Documented = 2 });
u8_enum!(DocumentRole, ArtifactError::Malformed, {
    License = 1, Consent = 2, Acquisition = 3, Restriction = 4,
});
u8_enum!(ReproductionStatus, ArtifactError::Malformed, {
    NotAttempted = 0, Reproduced = 1, Diverged = 2, Unavailable = 3,
});
u8_enum!(AssessmentMode, ArtifactError::Malformed, { Objective = 1, Subjective = 2 });
u8_enum!(TerminalStatus, ArtifactError::Malformed, {
    Success = 1, Refused = 2, Timeout = 3, Unknown = 4, Cancelled = 5,
});

/// Bounded list in the crate's typed-or-encoded view style. Encoded views are
/// produced only by strict decoders after full validation.
pub trait Item<'a>: Copy {
    /// Reads one item from `r`.
    ///
    /// # Errors
    /// Returns `Malformed` when the bytes are truncated or not a canonical item.
    fn read(r: &mut Reader<'a>) -> ArtifactResult<Self>;
    /// Writes this item to `w`.
    ///
    /// # Errors
    /// Returns `Malformed` when `w` lacks space or a length does not fit its field.
    fn write(&self, w: &mut Writer<'_>) -> ArtifactResult<()>;
}
#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Items<'a, T> {
    Typed(&'a [T]),
    Encoded { count: usize, bytes: &'a [u8] },
}
impl<'a, T: Item<'a>> Items<'a, T> {
    #[must_use]
    pub fn len(&self) -> usize {
        match self {
            Self::Typed(items) => items.len(),
            Self::Encoded { count, .. } => *count,
        }
    }
    #[must_use]
    pub fn is_empty(&self) -> bool {
        self.len() == 0
    }
    #[must_use]
    pub fn iter(&self) -> ItemIter<'a, T> {
        let bytes: &'a [u8] = match self {
            Self::Typed(_) => &[],
            Self::Encoded { bytes, .. } => bytes,
        };
        ItemIter {
            items: *self,
            index: 0,
            reader: Reader::new(bytes),
        }
    }
}
impl<'a, T: Item<'a>> IntoIterator for &Items<'a, T> {
    type Item = ArtifactResult<T>;
    type IntoIter = ItemIter<'a, T>;
    fn into_iter(self) -> Self::IntoIter {
        self.iter()
    }
}
pub struct ItemIter<'a, T> {
    items: Items<'a, T>,
    index: usize,
    reader: Reader<'a>,
}
impl<'a, T: Item<'a>> Iterator for ItemIter<'a, T> {
    type Item = ArtifactResult<T>;
    fn next(&mut self) -> Option<Self::Item> {
        let item = match self.items {
            Items::Typed(items) => items.get(self.index).copied().map(Ok),
            Items::Encoded { count, .. } if self.index < count => Some(T::read(&mut self.reader)),
            Items::Encoded { .. } => None,
        };
        self.index += 1;
        item
    }
}
impl<'a> Item<'a> for [u8; 32] {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'a>) -> ArtifactResult<Self> {
        Ok(r.fixed()?)
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write(&self, w: &mut Writer<'_>) -> ArtifactResult<()> {
        Ok(w.put(self)?)
    }
}

fn h(domain: &[u8], parts: &[&[u8]]) -> [u8; 32] {
    let mut hasher = Sha256::new();
    hasher.update(domain);
    for part in parts {
        hasher.update(part);
    }
    hasher.finalize().into()
}
fn nonzero(root: &[u8; 32]) -> ArtifactResult<()> {
    if *root == [0; 32] {
        Err(ArtifactError::Malformed)
    } else {
        Ok(())
    }
}
fn version(r: &mut Reader<'_>) -> ArtifactResult<()> {
    if r.u16()? == VERSION {
        Ok(())
    } else {
        Err(ArtifactError::UnsupportedVersion)
    }
}
fn length_u32(length: usize) -> ArtifactResult<[u8; 4]> {
    Ok(u32::try_from(length)
        .map_err(|_| ArtifactError::Malformed)?
        .to_be_bytes())
}

/// Common context C = `chain_domain32||program32||market32||policy32`.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ArtifactContext {
    pub chain: ChainDomain,
    pub program: ProgramId,
    pub market: MarketId,
    pub policy: PolicyDigest,
}
impl ArtifactContext {
    #[must_use]
    pub fn bytes(&self) -> [u8; 128] {
        let mut out = [0; 128];
        out.guest_index_mut(..32)
            .guest_copy_from_slice(self.chain.as_bytes());
        out.guest_index_mut(32..64)
            .guest_copy_from_slice(self.program.as_bytes());
        out.guest_index_mut(64..96)
            .guest_copy_from_slice(self.market.as_bytes());
        out.guest_index_mut(96..)
            .guest_copy_from_slice(self.policy.as_bytes());
        out
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'_>) -> ArtifactResult<Self> {
        Ok(
            crate::codec::read_record!(r, Self {  }; chain = r.chain_domain()?;
                program: ProgramId,
                market: MarketId,
                policy: PolicyDigest
            ),
        )
    }
}

/// Unsigned-manifest root, distinct from the F09 `EvidenceRoot`.
#[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ArtifactManifestRoot(Digest32);
impl ArtifactManifestRoot {
    /// Root from nonzero bytes.
    ///
    /// # Errors
    /// Returns `Malformed` when `bytes` are all zero.
    pub fn new(bytes: [u8; 32]) -> ArtifactResult<Self> {
        Ok(Self(Digest32::new(bytes)?))
    }
    #[must_use]
    pub const fn bytes(self) -> [u8; 32] {
        self.0.bytes()
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ParentRef {
    pub purpose: ParentPurpose,
    pub root: [u8; 32],
}
impl<'a> Item<'a> for ParentRef {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'a>) -> ArtifactResult<Self> {
        Ok(Self {
            purpose: ParentPurpose::decode(r.u8()?)?,
            root: r.fixed()?,
        })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write(&self, w: &mut Writer<'_>) -> ArtifactResult<()> {
        w.u8(self.purpose as u8)?;
        Ok(w.put(&self.root)?)
    }
}

#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ArtifactManifest<'a> {
    pub kind: ArtifactKind,
    pub privacy: Privacy,
    pub context: ArtifactContext,
    pub epoch: u64,
    pub publisher: PrincipalId,
    pub subject: [u8; 32],
    pub byte_length: u64,
    pub chunk_count: u32,
    pub content_root: [u8; 32],
    pub parents: Items<'a, ParentRef>,
    pub declaration_root: [u8; 32],
    pub reproduction_root: [u8; 32],
    pub access_policy_root: [u8; 32],
    pub not_after_height: u64,
}
impl ArtifactManifest<'_> {
    const fn epoch_independent(&self) -> bool {
        matches!(
            self.kind,
            ArtifactKind::Model | ArtifactKind::Dataset | ArtifactKind::Benchmark
        )
    }
    const fn task_bound(&self) -> bool {
        matches!(
            self.kind,
            ArtifactKind::Input | ArtifactKind::Result | ArtifactKind::ExecutionEvidence
        )
    }
    /// Encoded byte length of the manifest.
    ///
    /// # Errors
    /// Returns `Malformed` when the length overflows.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encoded_len(&self) -> ArtifactResult<usize> {
        self.parents
            .len()
            .checked_mul(PARENT_BYTES)
            .and_then(|n| n.checked_add(MANIFEST_FIXED_BYTES))
            .ok_or(ArtifactError::Malformed)
    }
    /// Checks the manifest's bounds, roots, parent order and context rules.
    ///
    /// # Errors
    /// Returns `LengthMismatch` when `chunk_count` disagrees with `byte_length`; `RootMismatch`
    /// when an empty object's content root is not the empty-tree root; `InvalidContext` when a
    /// policy-scoped kind has a nonzero epoch or a subject other than the policy; `Malformed` for
    /// every other bound, zero-root or ordering violation.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate(&self) -> ArtifactResult<()> {
        if self.byte_length > MAX_OBJECT_BYTES || self.chunk_count > MAX_CHUNKS {
            return Err(ArtifactError::Malformed);
        }
        if chunk_count(self.byte_length)? != self.chunk_count {
            return Err(ArtifactError::LengthMismatch);
        }
        nonzero(&self.content_root)?;
        if self.byte_length == 0 {
            if self.privacy == Privacy::Encrypted {
                return Err(ArtifactError::Malformed);
            }
            if self.content_root != content_root(0, 0, &empty_tree_root()) {
                return Err(ArtifactError::RootMismatch);
            }
        }
        if self.privacy == Privacy::Encrypted {
            nonzero(&self.access_policy_root)?;
        }
        if self.parents.len() > MAX_PARENTS {
            return Err(ArtifactError::Malformed);
        }
        let mut previous: Option<(u8, [u8; 32])> = None;
        for parent in &self.parents {
            let parent = parent?;
            if !matches!(
                parent.purpose,
                ParentPurpose::Reproduction | ParentPurpose::Declaration
            ) {
                nonzero(&parent.root)?;
            }
            let key = (parent.purpose as u8, parent.root);
            if previous.is_some_and(|p| p >= key) {
                return Err(ArtifactError::Malformed);
            }
            previous = Some(key);
        }
        nonzero(&self.subject)?;
        if self.epoch_independent()
            && (self.epoch != 0 || self.subject != self.context.policy.bytes())
        {
            return Err(ArtifactError::InvalidContext);
        }
        Ok(())
    }
}

/// Encodes a validated manifest into `out`, returning the bytes written.
///
/// # Errors
/// Propagates `ArtifactManifest::validate` refusals; returns `Malformed` when the encoding exceeds
/// `MAX_MANIFEST_BYTES` or `out` is too short.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_manifest(value: &ArtifactManifest<'_>, out: &mut [u8]) -> ArtifactResult<usize> {
    value.validate()?;
    let size = value.encoded_len()?;
    if size > MAX_MANIFEST_BYTES || out.len() < size {
        return Err(ArtifactError::Malformed);
    }
    let mut w = Writer::new(out);
    crate::codec::write_fields!(w;
        put: MAGIC;
        u16: VERSION;
        u8: value.kind as u8;
        u8: value.privacy as u8;
        u32: 0_u32;
        put: &value.context.bytes();
        u64: value.epoch;
        id32: value.publisher.as_bytes();
        put: &value.subject;
        u64: value.byte_length;
        u32: CHUNK_BYTES;
        u32: value.chunk_count;
        put: &value.content_root;
    )?;
    w.u16(u16::try_from(value.parents.len()).map_err(|_| ArtifactError::Malformed)?)?;
    for parent in &value.parents {
        parent?.write(&mut w)?;
    }
    crate::codec::write_fields!(w;
        put: &value.declaration_root;
        put: &value.reproduction_root;
        put: &value.access_policy_root;
        u64: value.not_after_height;
    )?;
    w.put(&[0; 16])?;
    Ok(w.len())
}

/// Strictly decodes and validates a manifest.
///
/// # Errors
/// Returns `UnsupportedVersion` for another version; `UnsupportedKind` for an unknown kind;
/// `Malformed` for size, magic, chunk-size, parent-count, reserved, zero-identity, truncation or
/// trailing-byte faults; then propagates `ArtifactManifest::validate` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_manifest(input: &[u8]) -> ArtifactResult<ArtifactManifest<'_>> {
    if input.len() > MAX_MANIFEST_BYTES || input.len() < MANIFEST_FIXED_BYTES {
        return Err(ArtifactError::Malformed);
    }
    let mut r = Reader::new(input);
    if r.fixed::<8>()? != *MAGIC {
        return Err(ArtifactError::Malformed);
    }
    version(&mut r)?;
    let kind = ArtifactKind::decode(r.u8()?)?;
    let privacy = Privacy::decode(r.u8()?)?;
    r.reserved(4)?;
    let context = ArtifactContext::read(&mut r)?;
    let epoch = r.u64()?;
    let (publisher, subject, byte_length) = crate::codec::read_values!(&mut r;
        publisher = r.principal_id()?; subject: Fixed32, byte_length: U64,
    );
    if r.u32()? != CHUNK_BYTES {
        return Err(ArtifactError::Malformed);
    }
    let chunk_count = r.u32()?;
    let content_root = r.fixed()?;
    let count = usize::from(r.u16()?);
    if count > MAX_PARENTS {
        return Err(ArtifactError::Malformed);
    }
    let parents = Items::Encoded {
        count,
        bytes: r.take(count * PARENT_BYTES)?,
    };
    let (declaration_root, reproduction_root, access_policy_root, not_after_height) = crate::codec::read_values!(&mut r; seed = publisher;
        declaration_root: Fixed32, reproduction_root: Fixed32,
        access_policy_root: Fixed32, not_after_height: U64,
    );
    let manifest = ArtifactManifest {
        kind,
        privacy,
        context,
        epoch,
        publisher,
        subject,
        byte_length,
        chunk_count,
        content_root,
        parents,
        declaration_root,
        reproduction_root,
        access_policy_root,
        not_after_height,
    };
    r.reserved(16)?;
    r.finish()?;
    manifest.validate()?;
    Ok(manifest)
}

/// H(D(manifest)||C||u32(len)||encoded) over strictly decoded canonical bytes.
///
/// # Errors
/// Propagates `decode_manifest` refusals.
pub fn manifest_root(encoded: &[u8]) -> ArtifactResult<ArtifactManifestRoot> {
    let manifest = decode_manifest(encoded)?;
    ArtifactManifestRoot::new(h(
        MANIFEST_DOMAIN,
        &[
            &manifest.context.bytes(),
            &length_u32(encoded.len())?,
            encoded,
        ],
    ))
}

/// Supplied subject expectation; a comparison input, never host authority.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum SubjectContext {
    Policy,
    Task { epoch: u64, task: TaskId },
}

/// Compares a manifest's context, epoch and subject with the expectation.
///
/// # Errors
/// Returns `InvalidContext` when the context differs or the epoch/subject does not match `subject`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn check_manifest_context(
    manifest: &ArtifactManifest<'_>,
    expected: &ArtifactContext,
    subject: SubjectContext,
) -> ArtifactResult<()> {
    if manifest.context != *expected {
        return Err(ArtifactError::InvalidContext);
    }
    let matches = match subject {
        SubjectContext::Policy => {
            !manifest.task_bound()
                && manifest.epoch == 0
                && manifest.subject == expected.policy.bytes()
        }
        SubjectContext::Task { epoch, task } => {
            !manifest.epoch_independent()
                && manifest.epoch == epoch
                && manifest.subject == task.bytes()
        }
    };
    if matches {
        Ok(())
    } else {
        Err(ArtifactError::InvalidContext)
    }
}

#[must_use]
pub fn publisher_digest(
    context: &ArtifactContext,
    root: ArtifactManifestRoot,
    generation: Version,
) -> [u8; 32] {
    h(
        PUBLISHER_DOMAIN,
        &[
            &context.bytes(),
            &root.bytes(),
            &generation.get().to_be_bytes(),
        ],
    )
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct PublisherEnvelope<'a> {
    pub manifest: &'a [u8],
    pub generation: Version,
    pub key: PublicKey32,
    pub signature: Signature64,
}
/// Signature-verified attribution; not tenant admission or grant authority.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct VerifiedPublisher {
    pub root: ArtifactManifestRoot,
    pub publisher: PrincipalId,
    pub generation: Version,
    pub key: PublicKey32,
}

/// Encodes a publisher envelope into `out`, returning the bytes written.
///
/// # Errors
/// Propagates `decode_manifest` refusals for the embedded manifest; returns `Malformed` when the
/// key is zero, the envelope exceeds `MAX_ENVELOPE_BYTES` or `out` is too short.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_envelope(value: &PublisherEnvelope<'_>, out: &mut [u8]) -> ArtifactResult<usize> {
    decode_manifest(value.manifest)?;
    if value.key.0 == [0; 32] {
        return Err(ArtifactError::Malformed);
    }
    let size = ENVELOPE_FIXED_BYTES + value.manifest.len();
    if size > MAX_ENVELOPE_BYTES || out.len() < size {
        return Err(ArtifactError::Malformed);
    }
    let mut w = Writer::new(out);
    w.put(&length_u32(value.manifest.len())?)?;
    crate::codec::write_fields!(w;
        put: value.manifest;
        u16: SIGNATURE_SCHEME_ED25519;
        u64: value.generation.get();
        put: &value.key.0;
        put: &value.signature.0;
    )?;
    Ok(w.len())
}

/// Strictly decodes a publisher envelope.
///
/// # Errors
/// Returns `Malformed` for an oversized envelope or manifest, a non-Ed25519 scheme, a zero
/// generation or key, truncation or trailing bytes; propagates `decode_manifest` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_envelope(input: &[u8]) -> ArtifactResult<PublisherEnvelope<'_>> {
    if input.len() > MAX_ENVELOPE_BYTES {
        return Err(ArtifactError::Malformed);
    }
    let mut r = Reader::new(input);
    let length = usize::try_from(r.u32()?).map_err(|_| ArtifactError::Malformed)?;
    if length > MAX_MANIFEST_BYTES {
        return Err(ArtifactError::Malformed);
    }
    let manifest = r.take(length)?;
    decode_manifest(manifest)?;
    if r.u16()? != SIGNATURE_SCHEME_ED25519 {
        return Err(ArtifactError::Malformed);
    }
    let generation = r.version()?;
    let key = PublicKey32(r.fixed()?);
    if key.0 == [0; 32] {
        return Err(ArtifactError::Malformed);
    }
    let signature = Signature64(r.fixed()?);
    r.finish()?;
    Ok(PublisherEnvelope {
        manifest,
        generation,
        key,
        signature,
    })
}

/// Ed25519 over the 32-byte publisher digest only; the manifest root ignores
/// the signature, so distinct valid signatures share one root.
///
/// # Errors
/// Returns `VerificationFailure::Artifact` with the `decode_manifest` refusals or
/// `SignatureInvalid` when the signature does not verify; `VerificationFailure::Host` when the host
/// verifier fails (wasm32 only).
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn verify_publisher(
    envelope: &PublisherEnvelope<'_>,
) -> Result<VerifiedPublisher, VerificationFailure> {
    let manifest = decode_manifest(envelope.manifest)?;
    let root = manifest_root(envelope.manifest)?;
    let digest = publisher_digest(&manifest.context, root, envelope.generation);
    verify_digest(envelope.key, envelope.signature, digest).map_err(|error| match error {
        VerificationError::Application(_) => {
            VerificationFailure::Artifact(ArtifactError::SignatureInvalid)
        }
        #[cfg(target_arch = "wasm32")]
        host => VerificationFailure::Host(host),
    })?;
    Ok(VerifiedPublisher {
        root,
        publisher: manifest.publisher,
        generation: envelope.generation,
        key: envelope.key,
    })
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RightsDocument<'a> {
    pub role: DocumentRole,
    pub root: [u8; 32],
    pub label: &'a str,
}
impl<'a> Item<'a> for RightsDocument<'a> {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'a>) -> ArtifactResult<Self> {
        let role = DocumentRole::decode(r.u8()?)?;
        let root = r.fixed()?;
        let length = usize::try_from(r.u32()?).map_err(|_| ArtifactError::Malformed)?;
        if length > MAX_LABEL_BYTES {
            return Err(ArtifactError::Malformed);
        }
        let label = core::str::from_utf8(r.take(length)?).map_err(|_| ArtifactError::Malformed)?;
        Ok(Self { role, root, label })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write(&self, w: &mut Writer<'_>) -> ArtifactResult<()> {
        w.u8(self.role as u8)?;
        w.put(&self.root)?;
        w.put(&length_u32(self.label.len())?)?;
        Ok(w.put(self.label.as_bytes())?)
    }
}

/// A rights claim and evidence reference; DOCUMENTED is never legal clearance.
#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Declaration<'a> {
    pub publisher: PrincipalId,
    pub rights: RightsStatus,
    pub purpose_mask: u16,
    pub restriction_mask: u16,
    pub valid_until_height: u64,
    pub documents: Items<'a, RightsDocument<'a>>,
    pub review_reference_root: [u8; 32],
}
impl Declaration<'_> {
    /// Checks masks, document bounds and strict document order.
    ///
    /// # Errors
    /// Returns `Malformed` for undefined mask bits, too many documents, `Documented` without
    /// documents, a zero document root, an oversized label or unordered/duplicate documents.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate(&self) -> ArtifactResult<()> {
        if self.purpose_mask & !0x000f != 0 || self.restriction_mask & !0x001f != 0 {
            return Err(ArtifactError::Malformed);
        }
        if self.documents.len() > MAX_DOCUMENTS {
            return Err(ArtifactError::Malformed);
        }
        if self.rights == RightsStatus::Documented && self.documents.is_empty() {
            return Err(ArtifactError::Malformed);
        }
        let mut previous: Option<RightsDocument<'_>> = None;
        for document in &self.documents {
            let document = document?;
            nonzero(&document.root)?;
            if document.label.len() > MAX_LABEL_BYTES {
                return Err(ArtifactError::Malformed);
            }
            let key = (document.role, document.root, document.label.as_bytes());
            if previous.is_some_and(|p| (p.role, p.root, p.label.as_bytes()) >= key) {
                return Err(ArtifactError::Malformed);
            }
            previous = Some(document);
        }
        Ok(())
    }
    /// Encoded byte length of the declaration.
    ///
    /// # Errors
    /// Returns `Malformed` when a document fails to decode or the length overflows.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encoded_len(&self) -> ArtifactResult<usize> {
        let mut size = DECLARATION_FIXED_BYTES;
        for document in &self.documents {
            size = size
                .checked_add(DOCUMENT_FIXED_BYTES + document?.label.len())
                .ok_or(ArtifactError::Malformed)?;
        }
        Ok(size)
    }
}
/// A missing declaration stays UNDECLARED; nothing is inferred.
#[must_use]
pub fn rights_status(declaration: Option<&Declaration<'_>>) -> RightsStatus {
    declaration.map_or(RightsStatus::Undeclared, |d| d.rights)
}

/// Encodes a validated declaration into `out`, returning the bytes written.
///
/// # Errors
/// Propagates `Declaration::validate` and `Declaration::encoded_len` refusals; returns `Malformed`
/// when the encoding exceeds `MAX_RECORD_BYTES` or `out` is too short.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_declaration(value: &Declaration<'_>, out: &mut [u8]) -> ArtifactResult<usize> {
    value.validate()?;
    let size = value.encoded_len()?;
    if size > MAX_RECORD_BYTES || out.len() < size {
        return Err(ArtifactError::Malformed);
    }
    let mut w = Writer::new(out);
    crate::codec::write_fields!(w;
        u16: VERSION;
        id32: value.publisher.as_bytes();
        u8: value.rights as u8;
        u16: value.purpose_mask;
        u16: value.restriction_mask;
        u64: value.valid_until_height;
    )?;
    w.u16(u16::try_from(value.documents.len()).map_err(|_| ArtifactError::Malformed)?)?;
    for document in &value.documents {
        document?.write(&mut w)?;
    }
    w.put(&value.review_reference_root)?;
    w.put(&[0; 16])?;
    Ok(w.len())
}

/// Strictly decodes and validates a declaration.
///
/// # Errors
/// Returns `UnsupportedVersion` for another version; `Malformed` for size, zero-publisher,
/// unknown-enum, document-count, reserved, truncation or trailing-byte faults; then propagates
/// `Declaration::validate` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_declaration(input: &[u8]) -> ArtifactResult<Declaration<'_>> {
    if input.len() > MAX_RECORD_BYTES || input.len() < DECLARATION_FIXED_BYTES {
        return Err(ArtifactError::Malformed);
    }
    let mut r = Reader::new(input);
    version(&mut r)?;
    let publisher = r.principal_id()?;
    let rights = RightsStatus::decode(r.u8()?)?;
    let purpose_mask = r.u16()?;
    let restriction_mask = r.u16()?;
    let valid_until_height = r.u64()?;
    let count = usize::from(r.u16()?);
    if count > MAX_DOCUMENTS {
        return Err(ArtifactError::Malformed);
    }
    let start = r.offset();
    for _ in 0..count {
        RightsDocument::read(&mut r)?;
    }
    let documents = Items::Encoded {
        count,
        bytes: input.guest_index(start..r.offset()),
    };
    let declaration = Declaration {
        publisher,
        rights,
        purpose_mask,
        restriction_mask,
        valid_until_height,
        documents,
        review_reference_root: r.fixed()?,
    };
    r.reserved(16)?;
    r.finish()?;
    declaration.validate()?;
    Ok(declaration)
}

/// Context-bound root of canonical declaration bytes.
///
/// # Errors
/// Propagates `decode_declaration` refusals.
pub fn declaration_root(context: &ArtifactContext, encoded: &[u8]) -> ArtifactResult<Digest32> {
    decode_declaration(encoded)?;
    Ok(Digest32::new(h(
        DECLARATION_DOMAIN,
        &[&context.bytes(), &length_u32(encoded.len())?, encoded],
    ))?)
}

/// Matching roots mean equal bytes, never independent re-execution.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Reproduction {
    pub task: TaskId,
    pub request_root: [u8; 32],
    pub model_root: [u8; 32],
    pub input_root: [u8; 32],
    pub result_root: [u8; 32],
    pub evaluator_program_record_root: [u8; 32],
    pub environment_root: [u8; 32],
    pub method_root: [u8; 32],
    pub seed_root: [u8; 32],
    pub metric_schema_root: [u8; 32],
    pub observed_units: u64,
    pub status: ReproductionStatus,
    pub repeated_result_root: [u8; 32],
}
impl Reproduction {
    /// Checks that the request, model, input and result roots are nonzero.
    ///
    /// # Errors
    /// Returns `Malformed` when any of those roots is zero.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate(&self) -> ArtifactResult<()> {
        nonzero(&self.request_root)?;
        nonzero(&self.model_root)?;
        nonzero(&self.input_root)?;
        nonzero(&self.result_root)
    }
}

/// Encodes a validated reproduction record.
///
/// # Errors
/// Propagates `Reproduction::validate` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_reproduction(value: &Reproduction) -> ArtifactResult<[u8; REPRODUCTION_BYTES]> {
    value.validate()?;
    let mut out = [0; REPRODUCTION_BYTES];
    let mut w = Writer::new(&mut out);
    w.u16(VERSION)?;
    w.put(value.task.as_bytes())?;
    for root in [
        &value.request_root,
        &value.model_root,
        &value.input_root,
        &value.result_root,
        &value.evaluator_program_record_root,
        &value.environment_root,
        &value.method_root,
        &value.seed_root,
        &value.metric_schema_root,
    ] {
        w.put(root)?;
    }
    crate::codec::write_fields!(w;
        u64: value.observed_units;
        u8: value.status as u8;
        put: &value.repeated_result_root;
    )?;
    w.put(&[0; 16])?;
    Ok(out)
}

/// Strictly decodes and validates a reproduction record.
///
/// # Errors
/// Returns `UnsupportedVersion` for another version; `Malformed` for a wrong length, zero task,
/// unknown status or nonzero reserved bytes; then propagates `Reproduction::validate` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_reproduction(input: &[u8]) -> ArtifactResult<Reproduction> {
    if input.len() != REPRODUCTION_BYTES {
        return Err(ArtifactError::Malformed);
    }
    let mut r = Reader::new(input);
    version(&mut r)?;
    let (
        task,
        request_root,
        model_root,
        input_root,
        result_root,
        evaluator_program_record_root,
        environment_root,
        method_root,
        seed_root,
        metric_schema_root,
        observed_units,
    ) = crate::codec::read_values!(&mut r;
        task = r.task_id()?;
        request_root: Fixed32, model_root: Fixed32, input_root: Fixed32, result_root: Fixed32,
        evaluator_program_record_root: Fixed32, environment_root: Fixed32, method_root: Fixed32,
        seed_root: Fixed32, metric_schema_root: Fixed32, observed_units: U64,
    );
    let value = Reproduction {
        task,
        request_root,
        model_root,
        input_root,
        result_root,
        evaluator_program_record_root,
        environment_root,
        method_root,
        seed_root,
        metric_schema_root,
        observed_units,
        status: ReproductionStatus::decode(r.u8()?)?,
        repeated_result_root: r.fixed()?,
    };
    r.reserved(16)?;
    r.finish()?;
    value.validate()?;
    Ok(value)
}

/// Context-bound root of canonical reproduction bytes.
///
/// # Errors
/// Propagates `decode_reproduction` refusals.
pub fn reproduction_root(context: &ArtifactContext, encoded: &[u8]) -> ArtifactResult<Digest32> {
    decode_reproduction(encoded)?;
    Ok(Digest32::new(h(
        REPRODUCTION_DOMAIN,
        &[&context.bytes(), &length_u32(encoded.len())?, encoded],
    ))?)
}

/// n = `ceil(byte_length / 262144)`, refused above 32 GiB / 131072 chunks.
///
/// # Errors
/// Returns `Malformed` when `byte_length` exceeds `MAX_OBJECT_BYTES`.
pub fn chunk_count(byte_length: u64) -> ArtifactResult<u32> {
    if byte_length > MAX_OBJECT_BYTES {
        return Err(ArtifactError::Malformed);
    }
    u32::try_from(byte_length.div_ceil(u64::from(CHUNK_BYTES)))
        .map_err(|_| ArtifactError::Malformed)
}
/// Exact length of chunk `index` of an object of `byte_length` in `count` chunks.
///
/// # Errors
/// Returns `Malformed` when `index` is not below `count` or the chunk offset is out of range;
/// `LengthMismatch` when the final chunk would be empty or longer than `CHUNK_BYTES`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn expected_chunk_length(byte_length: u64, count: u32, index: u32) -> ArtifactResult<u32> {
    if index >= count {
        return Err(ArtifactError::Malformed);
    }
    if index + 1 < count {
        return Ok(CHUNK_BYTES);
    }
    let before = u64::from(index)
        .checked_mul(u64::from(CHUNK_BYTES))
        .ok_or(ArtifactError::Malformed)?;
    let last = byte_length
        .checked_sub(before)
        .ok_or(ArtifactError::Malformed)?;
    if last == 0 || last > u64::from(CHUNK_BYTES) {
        return Err(ArtifactError::LengthMismatch);
    }
    u32::try_from(last).map_err(|_| ArtifactError::Malformed)
}
/// Domain-separated leaf hash of chunk `index`.
///
/// # Errors
/// Returns `Malformed` when `chunk` is longer than `CHUNK_BYTES`.
pub fn chunk_leaf(index: u32, chunk: &[u8]) -> ArtifactResult<[u8; 32]> {
    if chunk.len() > CHUNK_BYTES as usize {
        return Err(ArtifactError::Malformed);
    }
    Ok(h(
        CHUNK_DOMAIN,
        &[&index.to_be_bytes(), &length_u32(chunk.len())?, chunk],
    ))
}
#[must_use]
pub fn empty_tree_root() -> [u8; 32] {
    h(EMPTY_DOMAIN, &[])
}
#[must_use]
pub fn node_root(left: &[u8; 32], right: &[u8; 32]) -> [u8; 32] {
    h(NODE_DOMAIN, &[left, right])
}
#[must_use]
pub fn content_root(byte_length: u64, count: u32, tree_root: &[u8; 32]) -> [u8; 32] {
    h(
        CONTENT_DOMAIN,
        &[&byte_length.to_be_bytes(), &count.to_be_bytes(), tree_root],
    )
}
/// Reduces leaves in place; an unpaired final node is paired with itself.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn tree_root(leaves: &mut [[u8; 32]]) -> [u8; 32] {
    let mut width = leaves.len();
    if width == 0 {
        return empty_tree_root();
    }
    while width > 1 {
        let next = width.div_ceil(2);
        for i in 0..next {
            let left = *leaves.guest_index(2 * i);
            let right = if 2 * i + 1 < width {
                *leaves.guest_index(2 * i + 1)
            } else {
                left
            };
            *leaves.guest_index_mut(i) = node_root(&left, &right);
        }
        width = next;
    }
    *leaves.guest_index(0)
}
/// Number of proof siblings implied by successive ceil(n/2) levels.
#[must_use]
pub fn proof_sibling_count(count: u32) -> usize {
    let mut width = count;
    let mut siblings = 0;
    while width > 1 {
        width = width.div_ceil(2);
        siblings += 1;
    }
    siblings
}
/// Content root of in-memory object bytes; scratch must hold every leaf.
///
/// # Errors
/// Returns `Malformed` when `object` exceeds `MAX_OBJECT_BYTES`; `CapacityUnavailable` when
/// `scratch` holds fewer leaves than chunks.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn object_content_root(object: &[u8], scratch: &mut [[u8; 32]]) -> ArtifactResult<[u8; 32]> {
    let byte_length = u64::try_from(object.len()).map_err(|_| ArtifactError::Malformed)?;
    let count = chunk_count(byte_length)?;
    let leaves = scratch
        .get_mut(..count as usize)
        .ok_or(ArtifactError::CapacityUnavailable)?;
    for (index, chunk) in object.chunks(CHUNK_BYTES as usize).enumerate() {
        *leaves.guest_index_mut(index) = chunk_leaf(
            u32::try_from(index).map_err(|_| ArtifactError::Malformed)?,
            chunk,
        )?;
    }
    Ok(content_root(byte_length, count, &tree_root(leaves)))
}

/// Complete-retrieval verifier: every index exactly once (identical duplicates
/// add no coverage), exact lengths and the recomputed manifest content root.
pub struct ContentAssembler<'s> {
    byte_length: u64,
    count: u32,
    expected: [u8; 32],
    leaves: &'s mut [[u8; 32]],
}
impl<'s> ContentAssembler<'s> {
    /// Assembler for a validated manifest, using `scratch` for one leaf per chunk.
    ///
    /// # Errors
    /// Propagates `ArtifactManifest::validate` refusals; returns `CapacityUnavailable` when
    /// `scratch` holds fewer leaves than chunks.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn new(
        manifest: &ArtifactManifest<'_>,
        scratch: &'s mut [[u8; 32]],
    ) -> ArtifactResult<Self> {
        manifest.validate()?;
        let leaves = scratch
            .get_mut(..manifest.chunk_count as usize)
            .ok_or(ArtifactError::CapacityUnavailable)?;
        leaves.fill([0; 32]);
        Ok(Self {
            byte_length: manifest.byte_length,
            count: manifest.chunk_count,
            expected: manifest.content_root,
            leaves,
        })
    }
    /// Returns true only when the chunk adds new coverage.
    ///
    /// # Errors
    /// Propagates `expected_chunk_length` refusals; returns `LengthMismatch` when the chunk length
    /// differs, `Malformed` when it exceeds `CHUNK_BYTES`, `IntegrityConflict` when a different
    /// chunk was already delivered at `index`.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn deliver(&mut self, index: u32, chunk: &[u8]) -> ArtifactResult<bool> {
        let expected = expected_chunk_length(self.byte_length, self.count, index)?;
        if chunk.len() != expected as usize {
            return Err(ArtifactError::LengthMismatch);
        }
        let leaf = chunk_leaf(index, chunk)?;
        let slot = self.leaves.guest_index_mut(index as usize);
        if *slot == [0; 32] {
            *slot = leaf;
            Ok(true)
        } else if *slot == leaf {
            Ok(false)
        } else {
            Err(ArtifactError::IntegrityConflict)
        }
    }
    /// Checks full coverage and the recomputed content root.
    ///
    /// # Errors
    /// Returns `MissingChunk` when any index is undelivered; `RootMismatch` when the recomputed
    /// content root differs from the manifest.
    pub fn finish(self) -> ArtifactResult<()> {
        if self.leaves.contains(&[0; 32]) {
            return Err(ArtifactError::MissingChunk);
        }
        let tree = tree_root(self.leaves);
        if content_root(self.byte_length, self.count, &tree) == self.expected {
            Ok(())
        } else {
            Err(ArtifactError::RootMismatch)
        }
    }
}

/// Membership/integrity of one chunk only; never complete availability.
#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ChunkProof<'a> {
    pub manifest_root: ArtifactManifestRoot,
    pub index: u32,
    pub chunk: &'a [u8],
    pub siblings: Items<'a, [u8; 32]>,
}

/// Encodes a chunk proof into `out`, returning the bytes written.
///
/// # Errors
/// Returns `Malformed` when the chunk or sibling count exceeds its bound or `out` is too short.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_chunk_proof(value: &ChunkProof<'_>, out: &mut [u8]) -> ArtifactResult<usize> {
    if value.chunk.len() > CHUNK_BYTES as usize || value.siblings.len() > MAX_PROOF_SIBLINGS {
        return Err(ArtifactError::Malformed);
    }
    let size = PROOF_FIXED_BYTES + value.chunk.len() + 32 * value.siblings.len();
    if out.len() < size {
        return Err(ArtifactError::Malformed);
    }
    let mut w = Writer::new(out);
    crate::codec::write_fields!(w;
        u16: VERSION;
        put: &value.manifest_root.bytes();
        u32: value.index;
    )?;
    w.put(&length_u32(value.chunk.len())?)?;
    w.put(value.chunk)?;
    w.u8(u8::try_from(value.siblings.len()).map_err(|_| ArtifactError::Malformed)?)?;
    for sibling in &value.siblings {
        sibling?.write(&mut w)?;
    }
    Ok(w.len())
}

/// Strictly decodes a chunk proof.
///
/// # Errors
/// Returns `UnsupportedVersion` for another version; `Malformed` for a zero manifest root, an
/// oversized chunk or sibling count, truncation or trailing bytes.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_chunk_proof(input: &[u8]) -> ArtifactResult<ChunkProof<'_>> {
    let mut r = Reader::new(input);
    version(&mut r)?;
    let manifest_root = ArtifactManifestRoot::new(r.fixed()?)?;
    let index = r.u32()?;
    let length = r.u32()?;
    if length > CHUNK_BYTES {
        return Err(ArtifactError::Malformed);
    }
    let chunk = r.take(length as usize)?;
    let count = usize::from(r.u8()?);
    if count > MAX_PROOF_SIBLINGS {
        return Err(ArtifactError::Malformed);
    }
    let siblings = Items::Encoded {
        count,
        bytes: r.take(count * 32)?,
    };
    r.finish()?;
    Ok(ChunkProof {
        manifest_root,
        index,
        chunk,
        siblings,
    })
}

/// Verifies against the strictly decoded manifest whose root the proof names.
///
/// # Errors
/// Propagates `decode_manifest` and `expected_chunk_length` refusals; returns `RootMismatch` when
/// the manifest root or recomputed content root differs, `LengthMismatch` when the chunk length is
/// wrong, `Malformed` for a wrong sibling count or a self-paired sibling that is not the node.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn verify_chunk_proof(proof: &ChunkProof<'_>, manifest_bytes: &[u8]) -> ArtifactResult<()> {
    let manifest = decode_manifest(manifest_bytes)?;
    if manifest_root(manifest_bytes)? != proof.manifest_root {
        return Err(ArtifactError::RootMismatch);
    }
    let count = manifest.chunk_count;
    let expected = expected_chunk_length(manifest.byte_length, count, proof.index)?;
    if proof.chunk.len() != expected as usize {
        return Err(ArtifactError::LengthMismatch);
    }
    if proof.siblings.len() != proof_sibling_count(count) {
        return Err(ArtifactError::Malformed);
    }
    let mut node = chunk_leaf(proof.index, proof.chunk)?;
    let mut index = proof.index;
    let mut width = count;
    for sibling in &proof.siblings {
        let sibling = sibling?;
        if index + 1 == width && width % 2 == 1 {
            if sibling != node {
                return Err(ArtifactError::Malformed);
            }
            node = node_root(&node, &node);
        } else if index.is_multiple_of(2) {
            node = node_root(&node, &sibling);
        } else {
            node = node_root(&sibling, &node);
        }
        index /= 2;
        width = width.div_ceil(2);
    }
    if content_root(manifest.byte_length, count, &node) == manifest.content_root {
        Ok(())
    } else {
        Err(ArtifactError::RootMismatch)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EvidenceTask {
    pub task: TaskId,
    pub request_root: [u8; 32],
    pub result_root: [u8; 32],
    pub execution_root: [u8; 32],
    pub status: TerminalStatus,
    pub reproduction_root: [u8; 32],
}
impl<'a> Item<'a> for EvidenceTask {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'a>) -> ArtifactResult<Self> {
        let (task, request_root, result_root, execution_root) = crate::codec::read_values!(r;
            task = r.task_id()?; request_root: Fixed32, result_root: Fixed32, execution_root: Fixed32,
        );
        Ok(Self {
            task,
            request_root,
            result_root,
            execution_root,
            status: TerminalStatus::decode(r.u8()?)?,
            reproduction_root: r.fixed()?,
        })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write(&self, w: &mut Writer<'_>) -> ArtifactResult<()> {
        crate::codec::write_fields!(w;
            id32: self.task.as_bytes();
            put: &self.request_root;
            put: &self.result_root;
            put: &self.execution_root;
            u8: self.status as u8;
        )?;
        Ok(w.put(&self.reproduction_root)?)
    }
}

#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct WorkerGroup<'a> {
    pub worker: WorkerId,
    pub generation: Version,
    pub model_root: [u8; 32],
    pub deployment_root: [u8; 32],
    pub score: Score,
    pub reason_code: u16,
    pub reason_artifact_root: [u8; 32],
    pub tasks: Items<'a, EvidenceTask>,
}
impl<'a> Item<'a> for WorkerGroup<'a> {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn read(r: &mut Reader<'a>) -> ArtifactResult<Self> {
        let (
            worker,
            generation,
            model_root,
            deployment_root,
            score,
            reason_code,
            reason_artifact_root,
        ) = crate::codec::read_values!(r; worker = r.worker_id()?;
            generation: Version, model_root: Fixed32, deployment_root: Fixed32,
            score: Score, reason_code: U16, reason_artifact_root: Fixed32,
        );
        let count = usize::from(r.u16()?);
        if count > MAX_TASKS {
            return Err(ArtifactError::Malformed);
        }
        let tasks = Items::Encoded {
            count,
            bytes: r.take(count * TASK_ENTRY_BYTES)?,
        };
        for task in &tasks {
            task?;
        }
        Ok(Self {
            worker,
            generation,
            model_root,
            deployment_root,
            score,
            reason_code,
            reason_artifact_root,
            tasks,
        })
    }
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write(&self, w: &mut Writer<'_>) -> ArtifactResult<()> {
        crate::codec::write_fields!(w;
            id32: self.worker.as_bytes();
            u64: self.generation.get();
            put: &self.model_root;
            put: &self.deployment_root;
            u32: self.score.get();
            u16: self.reason_code;
            put: &self.reason_artifact_root;
        )?;
        w.u16(u16::try_from(self.tasks.len()).map_err(|_| ArtifactError::Malformed)?)?;
        for task in &self.tasks {
            task?.write(w)?;
        }
        Ok(())
    }
}

/// Explicit frozen-policy inputs for evidence admission of optional roots.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EvidencePolicy {
    pub mode: AssessmentMode,
    pub rubric: RubricDigest,
    pub dataset_absence_admitted: bool,
    pub benchmark_absence_admitted: bool,
    pub missing_result_admitted: bool,
}

#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EvidenceManifest<'a> {
    pub binding: EvaluatorBinding,
    pub task_policy: PolicyDigest,
    pub task_set: Digest32,
    pub rubric: RubricDigest,
    pub dataset_root: [u8; 32],
    pub benchmark_root: [u8; 32],
    pub mode: AssessmentMode,
    pub groups: Items<'a, WorkerGroup<'a>>,
}
impl EvidenceManifest<'_> {
    /// Checks the manifest against the frozen evidence policy.
    ///
    /// # Errors
    /// Returns `InvalidContext` when the mode or rubric differs from `policy`; `Malformed` for an
    /// unadmitted absent dataset/benchmark root, an out-of-range group count, unordered workers,
    /// zero model/deployment/request roots, empty, unordered or duplicate tasks, or an unadmitted
    /// missing result.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate(&self, policy: &EvidencePolicy) -> ArtifactResult<()> {
        if self.mode != policy.mode || self.rubric != policy.rubric {
            return Err(ArtifactError::InvalidContext);
        }
        if (self.dataset_root == [0; 32] && !policy.dataset_absence_admitted)
            || (self.benchmark_root == [0; 32] && !policy.benchmark_absence_admitted)
        {
            return Err(ArtifactError::Malformed);
        }
        if self.groups.is_empty() || self.groups.len() > MAX_WORKERS {
            return Err(ArtifactError::Malformed);
        }
        let mut seen = [[0u8; 32]; MAX_TASKS];
        let mut total = 0usize;
        let mut previous_worker: Option<WorkerId> = None;
        for group in &self.groups {
            let group = group?;
            if previous_worker.is_some_and(|w| w >= group.worker) {
                return Err(ArtifactError::Malformed);
            }
            previous_worker = Some(group.worker);
            nonzero(&group.model_root)?;
            nonzero(&group.deployment_root)?;
            if group.tasks.is_empty() {
                return Err(ArtifactError::Malformed);
            }
            let mut previous_task: Option<TaskId> = None;
            for task in &group.tasks {
                let task = task?;
                if previous_task.is_some_and(|t| t >= task.task) {
                    return Err(ArtifactError::Malformed);
                }
                previous_task = Some(task.task);
                if total >= MAX_TASKS || seen.guest_index(..total).contains(&task.task.bytes()) {
                    return Err(ArtifactError::Malformed);
                }
                *seen.guest_index_mut(total) = task.task.bytes();
                total += 1;
                nonzero(&task.request_root)?;
                let missing = task.result_root == [0; 32] || task.execution_root == [0; 32];
                if missing
                    && (task.status == TerminalStatus::Success || !policy.missing_result_admitted)
                {
                    return Err(ArtifactError::Malformed);
                }
            }
        }
        Ok(())
    }
    /// Encoded byte length of the evidence manifest.
    ///
    /// # Errors
    /// Returns `Malformed` when a group fails to decode or the length overflows.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encoded_len(&self) -> ArtifactResult<usize> {
        let mut size = EVIDENCE_FIXED_BYTES;
        for group in &self.groups {
            size = group?
                .tasks
                .len()
                .checked_mul(TASK_ENTRY_BYTES)
                .and_then(|n| n.checked_add(WORKER_GROUP_FIXED_BYTES))
                .and_then(|n| n.checked_add(size))
                .ok_or(ArtifactError::Malformed)?;
        }
        Ok(size)
    }
}

/// Encodes a validated evidence manifest into `out`, returning the bytes written.
///
/// # Errors
/// Propagates `EvidenceManifest::validate` and `EvidenceManifest::encoded_len` refusals; returns
/// `Malformed` when the encoding exceeds `MAX_EVIDENCE_BYTES`, `out` is too short or the binding
/// fails to encode.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn encode_evidence_manifest(
    value: &EvidenceManifest<'_>,
    policy: &EvidencePolicy,
    out: &mut [u8],
) -> ArtifactResult<usize> {
    value.validate(policy)?;
    let size = value.encoded_len()?;
    if size > MAX_EVIDENCE_BYTES || out.len() < size {
        return Err(ArtifactError::Malformed);
    }
    let mut w = Writer::new(out);
    w.u16(VERSION)?;
    w.put(&encode_binding(&value.binding)?)?;
    crate::codec::write_fields!(w;
        id32: value.task_policy.as_bytes();
        id32: value.task_set.as_bytes();
        id32: value.rubric.as_bytes();
        put: &value.dataset_root;
        put: &value.benchmark_root;
        u8: value.mode as u8;
    )?;
    w.u16(u16::try_from(value.groups.len()).map_err(|_| ArtifactError::Malformed)?)?;
    for group in &value.groups {
        group?.write(&mut w)?;
    }
    w.put(&[0; 16])?;
    Ok(w.len())
}

/// Strictly decodes and validates an evidence manifest.
///
/// # Errors
/// Returns `UnsupportedVersion` for another version; `Malformed` for size, binding, zero-identity,
/// unknown-mode, group or task count, reserved, truncation or trailing-byte faults; then propagates
/// `EvidenceManifest::validate` refusals.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn decode_evidence_manifest<'a>(
    input: &'a [u8],
    policy: &EvidencePolicy,
) -> ArtifactResult<EvidenceManifest<'a>> {
    if input.len() > MAX_EVIDENCE_BYTES || input.len() < EVIDENCE_FIXED_BYTES {
        return Err(ArtifactError::Malformed);
    }
    let mut r = Reader::new(input);
    version(&mut r)?;
    let binding = decode_binding(r.take(BINDING_BYTES)?)?;
    let (task_policy, task_set, rubric, dataset_root, benchmark_root) = crate::codec::read_values!(&mut r; task_policy = r.policy_digest()?;
        task_set: Digest32, rubric: RubricDigest, dataset_root: Fixed32, benchmark_root: Fixed32,
    );
    let mode = AssessmentMode::decode(r.u8()?)?;
    let count = usize::from(r.u16()?);
    if count == 0 || count > MAX_WORKERS {
        return Err(ArtifactError::Malformed);
    }
    let start = r.offset();
    let mut total = 0usize;
    for _ in 0..count {
        total = total
            .checked_add(WorkerGroup::read(&mut r)?.tasks.len())
            .ok_or(ArtifactError::Malformed)?;
        if total > MAX_TASKS {
            return Err(ArtifactError::Malformed);
        }
    }
    let groups = Items::Encoded {
        count,
        bytes: input.guest_index(start..r.offset()),
    };
    r.reserved(16)?;
    r.finish()?;
    let manifest = EvidenceManifest {
        binding,
        task_policy,
        task_set,
        rubric,
        dataset_root,
        benchmark_root,
        mode,
        groups,
    };
    manifest.validate(policy)?;
    Ok(manifest)
}

/// H(PAXAI/evidence/v1, canonical `EvidenceManifestV1` bytes): no artifact
/// wrapping, context prefix or extra length prefix.
///
/// # Errors
/// Propagates `decode_evidence_manifest` refusals.
pub fn evidence_root(encoded: &[u8], policy: &EvidencePolicy) -> ArtifactResult<EvidenceRoot> {
    decode_evidence_manifest(encoded, policy)?;
    Ok(EvidenceRoot::new(
        domain_hash(EVIDENCE_DOMAIN, encoded)?.bytes(),
    )?)
}

/// Canonical `SealEvidence` payload length.
pub const SEAL_PAYLOAD_BYTES: usize = 131;
/// One compact seal record before enclosing framing.
pub const SEAL_RECORD_BYTES: usize = 185;
const SEAL_REGION_HEADER_BYTES: usize = 9;
/// The F09 seal region at its bound: epoch, count and eight seals.
pub const SEAL_REGION_MAX_BYTES: usize =
    SEAL_REGION_HEADER_BYTES + MAX_EVALUATORS * SEAL_RECORD_BYTES;
const POLICY_CAP: usize = Section::PolicyLifecycle.payload_cap();
const CONTROL_CAP: usize = Section::Control.payload_cap();
/// Caller scratch for [`apply`]: the next F01 section, the next control feature bytes and the
/// control encoding.
pub const SEAL_SCRATCH_BYTES: usize = POLICY_CAP + 2 * CONTROL_CAP;

/// One evaluator's committed evidence claim for the opened epoch. A seal asserts an
/// authorized claim, never verified AI quality or availability.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EvidenceSeal {
    pub evaluator: EvaluatorId,
    pub grant: Version,
    pub key_version: Version,
    pub root: EvidenceRoot,
    pub task_policy: PolicyDigest,
    pub task_set: Digest32,
    pub rubric: RubricDigest,
    pub mode: AssessmentMode,
    pub height: u64,
}
impl EvidenceSeal {
    /// The canonical 185-byte record.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the record does not fill exactly 185 bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn encode(&self) -> CodecResult<[u8; SEAL_RECORD_BYTES]> {
        let mut out = [0; SEAL_RECORD_BYTES];
        let mut w = Writer::new(&mut out);
        crate::codec::write_fields!(w;
            id32: self.evaluator.as_bytes();
            u64: self.grant.get();
            u64: self.key_version.get();
            id32: self.root.as_bytes();
            id32: self.task_policy.as_bytes();
            id32: self.task_set.as_bytes();
            id32: self.rubric.as_bytes();
            u8: self.mode as u8;
            u64: self.height;
        )?;
        if w.len() != SEAL_RECORD_BYTES {
            return Err(NON_CANONICAL);
        }
        Ok(out)
    }
    /// Strictly decodes one record.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` for a wrong length, zero identity, version or root, or an
    /// unknown mode.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(input: &[u8]) -> CodecResult<Self> {
        let mut r = Reader::new(input);
        let (evaluator, grant, key_version, root, task_policy, task_set, rubric) = crate::codec::read_values!(&mut r; evaluator = r.evaluator_id()?;
            grant: Version, key_version: Version, root: EvidenceRoot,
            task_policy: PolicyDigest, task_set: Digest32, rubric: RubricDigest,
        );
        let seal = Self {
            evaluator,
            grant,
            key_version,
            root,
            task_policy,
            task_set,
            rubric,
            mode: AssessmentMode::decode(r.u8()?).map_err(|_| NON_CANONICAL)?,
            height: r.u64()?,
        };
        r.finish()?;
        Ok(seal)
    }
    /// Equal claims: every field except the sealing height.
    #[must_use]
    pub fn same_claim(&self, other: &Self) -> bool {
        Self {
            height: other.height,
            ..*self
        } == *other
    }
    /// The compact registration F03/F04 report admission compares against.
    #[must_use]
    pub const fn registered(&self, frozen: FrozenBinding) -> RegisteredEvidence {
        RegisteredEvidence {
            binding: EvaluatorBinding {
                frozen,
                evaluator: self.evaluator,
                grant: self.grant,
                key_version: self.key_version,
            },
            root: self.root,
            rubric: self.rubric,
        }
    }
}

/// The F09 seal region at the start of the control feature bytes: `epoch:u64 || count:u8 ||
/// count seal records` in strictly ascending evaluator order, then the bytes of other control
/// producers. Empty feature bytes hold no seal; an empty region exists only before other bytes.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct SealRegion<'a> {
    pub epoch: u64,
    records: &'a [u8],
    laps: &'a [u8],
    rest: &'a [u8],
}
impl<'a> SealRegion<'a> {
    /// Strictly decodes the region prefix of `feature_bytes`.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` for a truncated, oversized, unordered or duplicate region, an
    /// invalid record, or an empty region with nothing after it.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(feature_bytes: &'a [u8]) -> CodecResult<Self> {
        if feature_bytes.is_empty() {
            return Ok(Self {
                epoch: 0,
                records: &[],
                laps: &[],
                rest: &[],
            });
        }
        let mut r = Reader::new(feature_bytes);
        let epoch = r.u64()?;
        let count = usize::from(r.u8()?);
        if count > MAX_EVALUATORS {
            return Err(NON_CANONICAL);
        }
        let records = r.take(count * SEAL_RECORD_BYTES)?;
        let rest = feature_bytes.get(r.offset()..).ok_or(NON_CANONICAL)?;
        let lap_len = crate::laps::region_length(rest)?;
        let (laps, rest) = rest.guest_split_at(lap_len);
        if count == 0 && rest.is_empty() && laps.is_empty() {
            return Err(NON_CANONICAL);
        }
        let mut previous = None;
        for record in records
            .as_chunks::<{ SEAL_RECORD_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
        {
            let evaluator = EvidenceSeal::decode(record)?.evaluator;
            if previous.is_some_and(|p| p >= evaluator) {
                return Err(NON_CANONICAL);
            }
            previous = Some(evaluator);
        }
        Ok(Self {
            epoch,
            records,
            laps,
            rest,
        })
    }
    pub(crate) const fn lap_settlements(&self) -> &'a [u8] {
        self.laps
    }
    pub(crate) fn write_lap_prefix(&self, out: &mut [u8]) -> CodecResult<usize> {
        let mut w = Writer::new(out);
        crate::codec::write_fields!(w;
            u64: self.epoch;
            u8: (self.records.len() / SEAL_RECORD_BYTES) as u8;
            put: self.records;
        )?;
        Ok(w.len())
    }
    const fn of(&self, epoch: u64) -> &'a [u8] {
        if self.epoch == epoch {
            self.records
        } else {
            &[]
        }
    }
    /// Seals of `epoch`; seals of any other epoch read as absent.
    pub fn seals(&self, epoch: u64) -> impl Iterator<Item = CodecResult<EvidenceSeal>> + 'a {
        self.of(epoch)
            .as_chunks::<{ SEAL_RECORD_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
            .map(EvidenceSeal::decode)
    }
    /// The seal of `evaluator` in `epoch`.
    ///
    /// # Errors
    /// Propagates record decoding refusals.
    pub fn get(&self, epoch: u64, evaluator: EvaluatorId) -> CodecResult<Option<EvidenceSeal>> {
        for seal in self.seals(epoch) {
            let seal = seal?;
            if seal.evaluator == evaluator {
                return Ok(Some(seal));
            }
        }
        Ok(None)
    }
    /// Bytes of other control producers after the region.
    #[must_use]
    pub const fn rest(&self) -> &'a [u8] {
        self.rest
    }
    /// Writes the region of `epoch` with `seal` inserted, then the unchanged trailing bytes.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn write_with(&self, epoch: u64, seal: &EvidenceSeal, out: &mut [u8]) -> CodecResult<usize> {
        let current = self.of(epoch);
        let count = current.len() / SEAL_RECORD_BYTES;
        if count >= MAX_EVALUATORS {
            return Err(CAPACITY);
        }
        let mut at = current.len();
        for (index, record) in current
            .as_chunks::<{ SEAL_RECORD_BYTES }>()
            .0
            .iter()
            .map(|chunk| chunk.as_slice())
            .enumerate()
        {
            if EvidenceSeal::decode(record)?.evaluator > seal.evaluator {
                at = index * SEAL_RECORD_BYTES;
                break;
            }
        }
        let mut w = Writer::new(out);
        w.u64(epoch)?;
        w.u8(u8::try_from(count + 1).map_err(|_| ARITHMETIC)?)?;
        w.put(current.guest_index(..at))?;
        w.put(&seal.encode()?)?;
        crate::codec::write_fields!(w;
            put: &(*(current).guest_index(at..));
            put: self.laps;
            put: self.rest;
        )?;
        Ok(w.len())
    }
}

/// Committed F01 section whose header revision equals the shared revision.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn committed<'a>(state: &SharedState<'a>) -> CodecResult<PolicySection<'a>> {
    let section = PolicySection::decode(
        *state
            .feature_sections
            .guest_index(Section::PolicyLifecycle.index()),
    )?;
    if section.header.state_revision != state.revision {
        return Err(NON_CANONICAL);
    }
    Ok(section)
}

/// The frozen context of the opened epoch: F08 epoch presence, F01 config and F06 roster, with
/// the validated reward state recording that roster, reusing `known` when it is that state.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn frozen_binding<'a>(
    state: &SharedState<'a>,
    section: &PolicySection<'_>,
    _known: Option<RewardState<'a>>,
) -> CodecResult<FrozenBinding> {
    let admission = AdmissionTable::decode(crate::reputation_transition::framed_admission_table(
        *state
            .feature_sections
            .guest_index(Section::ReputationAdmission.index()),
    )?)?;
    let epoch = admission.current_epoch().ok_or(WRONG_EPOCH)?;
    let row = crate::rewards::committed_row(state.section(Section::SettlementClaims)?, epoch)
        .map_err(|e| if e == NOT_FOUND { WRONG_EPOCH } else { e })?
        .row;
    let header = &section.header;
    let frozen = FrozenBinding {
        chain: header.deployment_chain_domain,
        program: header.program_id,
        market: header.market_id,
        epoch,
        config: Version::new(header.active_config_version)?,
        roster: row.roster,
    };
    Ok(frozen)
}

/// The sealed evidence registration of `evaluator` in the opened `epoch`, read from committed
/// state exactly as F03/F04 report admission must supply `ReportContext::evidence`. An absent
/// seal refuses there (`F03_EVIDENCE_NOT_SEALED`); a seal never authorizes a report alone.
///
/// # Errors
/// Returns `WRONG_EPOCH` when `epoch` is not the opened epoch; `NON_CANONICAL` for an
/// inconsistent state or seal region.
pub fn sealed_evidence(
    state: &SharedState<'_>,
    epoch: u64,
    evaluator: EvaluatorId,
) -> CodecResult<Presence<RegisteredEvidence>> {
    sealed_evidence_with(state, epoch, evaluator, None)
}

/// [`sealed_evidence`] reusing the reward state `known` the caller already validated.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn sealed_evidence_with<'a>(
    state: &SharedState<'a>,
    epoch: u64,
    evaluator: EvaluatorId,
    known: Option<RewardState<'a>>,
) -> CodecResult<Presence<RegisteredEvidence>> {
    let frozen = frozen_binding(state, &committed(state)?, known)?;
    if frozen.epoch != epoch {
        return Err(WRONG_EPOCH);
    }
    sealed_evidence_for_binding(state, frozen, evaluator)
}

/// Reads the canonical seal using the caller's validated opened-epoch binding.
pub(crate) fn sealed_evidence_for_binding(
    state: &SharedState<'_>,
    frozen: FrozenBinding,
    evaluator: EvaluatorId,
) -> CodecResult<Presence<RegisteredEvidence>> {
    let epoch = frozen.epoch;
    let region = SealRegion::decode(state.control.feature_bytes)?;
    if region.epoch > epoch && !region.records.is_empty() {
        return Err(NON_CANONICAL);
    }
    Ok(match region.get(epoch, evaluator)? {
        Some(seal) => Presence::Present(seal.registered(frozen)),
        None => Presence::Absent,
    })
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Outcome {
    /// One revision increment and one role sequence were composed into `next` and the
    /// `EvidenceSealed` event into `event`.
    Applied {
        seal: EvidenceSeal,
        revision: u64,
        result: ResultDigest,
        state_len: usize,
        event_len: usize,
    },
    /// A new request repeating the stored claim; nothing was written.
    AlreadyApplied { seal: EvidenceSeal },
    /// Exact retry of a request already applied under the evaluator sequence.
    Retained(RetainedResult),
}

struct SealPayload {
    root: EvidenceRoot,
    task_policy: PolicyDigest,
    task_set: Digest32,
    rubric: RubricDigest,
    mode: AssessmentMode,
}
impl SealPayload {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn parse(payload: &[u8]) -> CodecResult<Self> {
        if payload.len() != SEAL_PAYLOAD_BYTES {
            return Err(NON_CANONICAL);
        }
        let mut r = Reader::new(payload);
        if r.u16()? != VERSION {
            return Err(BAD_VERSION);
        }
        let value = Self {
            root: r.evidence_root()?,
            task_policy: r.policy_digest()?,
            task_set: r.digest32()?,
            rubric: r.rubric_digest()?,
            mode: AssessmentMode::decode(r.u8()?).map_err(|_| NON_CANONICAL)?,
        };
        r.finish()?;
        Ok(value)
    }
}

struct Opened<'a> {
    state: SharedState<'a>,
    section: PolicySection<'a>,
    frozen: FrozenBinding,
    region: SealRegion<'a>,
}
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn opened(current: &[u8]) -> CodecResult<Opened<'_>> {
    let state = decode_shared_state(current)?;
    let section = committed(&state)?;
    let frozen = frozen_binding(&state, &section, None)?;
    let region = SealRegion::decode(state.control.feature_bytes)?;
    if region.epoch > frozen.epoch && !region.records.is_empty() {
        return Err(NON_CANONICAL);
    }
    Ok(Opened {
        state,
        section,
        frozen,
        region,
    })
}

struct Call<'c> {
    ctx: &'c CallContext,
    envelope: &'c ValidatedEnvelope<'c>,
    opened: &'c Opened<'c>,
}
impl Call<'_> {
    /// Domain, expiry, market and the exact frozen epoch, config and roster.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn check_binding(&self) -> CodecResult<()> {
        wire::check_frozen_envelope(&self.envelope.envelope, self.ctx, &self.opened.frozen)
    }
    /// The frozen evaluator the envelope authenticates as, natively by its owner or by its
    /// frozen delegate key.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn authenticate(&self) -> CodecResult<FrozenEvaluator> {
        let e = &self.envelope.envelope;
        let identity = *self
            .opened
            .state
            .feature_sections
            .guest_index(Section::IdentityRoster.index());
        let region = evaluator_region(identity)?;
        let snapshot = region.snapshot().ok_or(F03_NO_GRANT)?;
        if snapshot.epoch != self.opened.frozen.epoch {
            return Err(NON_CANONICAL);
        }
        let mut owned = snapshot.entries().filter(|f| f.entry.owner == e.actor);
        let frozen = match e.authentication {
            Authentication::Native => {
                wire::compare_native_principal(e, self.ctx.principal)?;
                let first = owned.next().copied().ok_or(UNAUTHORIZED)?;
                if owned.next().is_some() {
                    return Err(UNAUTHORIZED);
                }
                first
            }
            Authentication::Delegate { key, signature } => {
                let frozen = owned
                    .find(|f| f.entry.public_key == key)
                    .copied()
                    .ok_or(KEY_MISMATCH)?;
                let digest = Digest32::new(self.envelope.request_digest()?.bytes())?;
                verify_digest(key, signature, digest.bytes()).map_err(|error| match error {
                    VerificationError::Application(a) => a,
                    #[cfg(target_arch = "wasm32")]
                    VerificationError::Host(_) => crate::errors::HOST_CAPABILITY,
                })?;
                frozen
            }
        };
        check_frozen_evaluator(
            &region,
            &frozen,
            self.opened.frozen.epoch,
            *self
                .opened
                .state
                .feature_sections
                .guest_index(Section::ReputationAdmission.index()),
        )?;
        Ok(frozen)
    }
    /// Role replay request under the evaluator slot bound to the evaluator owner.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn request(&self, owner: PrincipalId) -> CodecResult<ReplayRequest> {
        let replay = &self.opened.state.control.replay;
        for index in 0..MAX_EVALUATORS {
            let slot = ActorSlot::evaluator(index)?;
            if let Some(actor) = replay.actor(slot).filter(|a| a.principal == owner) {
                return ReplayRequest::from_envelope(slot, actor.authority_version, self.envelope);
            }
        }
        Err(NOT_FOUND)
    }
    /// The claim of `payload` against the frozen policy and the explicit F01 task-set seal.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn claim(&self, frozen: &FrozenEvaluator, payload: &SealPayload) -> CodecResult<EvidenceSeal> {
        let policy = &self.opened.section.current;
        let rubric = policy.commitments.rubric;
        if payload.mode as u8 != policy.assessment_mode
            || payload.task_policy != policy.digest()?
            || payload.rubric != rubric
            || frozen.entry.rubric != rubric
        {
            return Err(EVIDENCE_BINDING);
        }
        let sealed =
            tasks::sealed_task_set_with(&self.opened.state, self.opened.frozen.epoch, None)?;
        if payload.task_set != sealed {
            return Err(EVIDENCE_BINDING);
        }
        Ok(EvidenceSeal {
            evaluator: frozen.entry.evaluator,
            grant: frozen.entry.grant,
            key_version: frozen.entry.key_version,
            root: payload.root,
            task_policy: payload.task_policy,
            task_set: payload.task_set,
            rubric: payload.rubric,
            mode: payload.mode,
            height: self.ctx.height,
        })
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn check_frozen_evaluator(
    region: &crate::evaluators::authority::EvaluatorRegion,
    frozen: &FrozenEvaluator,
    epoch: u64,
    membership: &[u8],
) -> CodecResult<()> {
    check_frozen_grant(region, frozen, epoch)?;
    let membership = crate::reputation_transition::joint_table(membership)?;
    let admission = AdmissionTable::decode(membership)?;
    check_frozen_membership(frozen, &admission)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn check_frozen_grant(
    region: &crate::evaluators::authority::EvaluatorRegion,
    frozen: &FrozenEvaluator,
    epoch: u64,
) -> CodecResult<()> {
    let evaluator = frozen.entry.evaluator;
    let live = region.get(evaluator).ok_or(NON_CANONICAL)?;
    if live.grant.status == GrantStatus::Revoked || region.excluded(evaluator) {
        return Err(REVOKED);
    }
    if live.grant.status == GrantStatus::Expired || epoch >= frozen.expiry_epoch_exclusive {
        return Err(EXPIRED);
    }
    if live.grant.status != GrantStatus::Active {
        return Err(F03_NO_GRANT);
    }
    if live.grant.grant_version != frozen.entry.grant {
        return Err(F03_GRANT_VERSION_CONFLICT);
    }
    if live.grant.key_version != frozen.entry.key_version {
        return Err(F03_KEY_VERSION_CONFLICT);
    }
    if live.grant.signing_key != frozen.entry.public_key {
        return Err(KEY_MISMATCH);
    }
    Ok(())
}

fn check_frozen_membership(
    frozen: &FrozenEvaluator,
    admission: &AdmissionTable,
) -> CodecResult<()> {
    let evaluator = frozen.entry.evaluator;
    match admission.get(Participant::Evaluator(evaluator)) {
        Some(meta) if meta.revoked() => Err(REVOKED),
        Some(meta) if meta.admitted() && meta.owner == frozen.entry.owner => Ok(()),
        _ => Err(UNAUTHORIZED),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn check_frozen_evaluator_in_table(
    region: &crate::evaluators::authority::EvaluatorRegion,
    frozen: &FrozenEvaluator,
    epoch: u64,
    admission: &AdmissionTable,
) -> CodecResult<()> {
    check_frozen_grant(region, frozen, epoch)?;
    check_frozen_membership(frozen, admission)
}

/// Commits `seal` at the next revision and evaluator sequence and emits `EvidenceSealed`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn commit(
    call: &Call<'_>,
    seal: &EvidenceSeal,
    request: &ReplayRequest,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    let opened = call.opened;
    let (policy, rest) = scratch.split_at_mut_checked(POLICY_CAP).ok_or(CAPACITY)?;
    let (features, control) = rest.split_at_mut_checked(CONTROL_CAP).ok_or(CAPACITY)?;
    let record = seal.encode()?;
    let result = wire::result_digest(&record)?;
    let revision = opened.state.revision.checked_add(1).ok_or(ARITHMETIC)?;
    let mut section = opened.section;
    section.header.state_revision = revision;
    let policy_len = section.encode(policy)?;
    let features_len = opened
        .region
        .write_with(opened.frozen.epoch, seal, features)?;
    let policy = policy.get(..policy_len).ok_or(CAPACITY)?;
    let mut candidate = opened
        .state
        .replace_section(Section::PolicyLifecycle, policy)?;
    candidate.control.feature_bytes = features.get(..features_len).ok_or(CAPACITY)?;
    if candidate.record_success(request, call.ctx.height, result)? != ReplayDecision::Apply
        || candidate.revision != revision
    {
        return Err(NON_CANONICAL);
    }
    check_f01_capacity(policy_len, candidate.encoded_len()?)?;
    let state_len = encode_shared_state(&candidate, next, control)?;
    let frozen = &opened.frozen;
    let event_len = wire::encode_event_frame(
        dispatch::SealEvidence,
        &EventCommon {
            market: frozen.market,
            epoch: frozen.epoch,
            config: frozen.config,
            revision,
            request: call.envelope.request_digest()?,
            result,
        },
        &record,
        event,
    )?;
    Ok(Outcome::Applied {
        seal: *seal,
        revision,
        result,
        state_len,
        event_len,
    })
}

/// Applies one `SealEvidence` (0x0901) request to the committed `current` state, writing the
/// whole next state into `next` (at least `MAX_STATE_BYTES`) and its `EvidenceSealed` event
/// into `event`; `scratch` holds at least [`SEAL_SCRATCH_BYTES`]. Dispatch arm:
/// `dispatch::SealEvidence => evidence::apply(&ctx, &envelope, current, next, scratch, event)`.
/// It must be admitted before the evaluator's `CommitScore`.
///
/// # Errors
/// `UNKNOWN_OPERATION`; `NON_CANONICAL` for a malformed payload, zero root or digest, or an
/// inconsistent committed state; `BAD_VERSION`; envelope principal, domain and expiry
/// refusals; `WRONG_MARKET`; `WRONG_EPOCH` (also with no opened epoch), `WRONG_CONFIG` and
/// `WRONG_ROSTER` for a binding other than the opened epoch's; `F03_NO_GRANT`,
/// `UNAUTHORIZED`, `KEY_MISMATCH`, `BAD_SIGNATURE`, `REVOKED`, `EXPIRED`,
/// `F03_GRANT_VERSION_CONFLICT` and `F03_KEY_VERSION_CONFLICT` for evaluator authority;
/// `NOT_FOUND` without a bound evaluator slot; common replay refusals; `WRONG_PHASE` outside
/// `[T+64, T+80)`; `EVIDENCE_BINDING` for a mode, task policy, rubric or task set other than
/// the frozen ones; `F09_EVIDENCE_TASK_SET_UNSEALED` before the explicit F01 task-set seal;
/// `F09_EVIDENCE_SEAL_CONFLICT` for a different claim of an already sealed evaluator;
/// `CAPACITY` and `F01_CAPACITY_UNAVAILABLE`. On any error `current` is unchanged and the
/// outputs must be discarded.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn apply(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
) -> CodecResult<Outcome> {
    if envelope.envelope.operation != dispatch::SealEvidence {
        return Err(UNKNOWN_OPERATION);
    }
    let opened = opened(current)?;
    let payload = SealPayload::parse(envelope.envelope.payload)?;
    let call = Call {
        ctx,
        envelope,
        opened: &opened,
    };
    call.check_binding()?;
    let frozen = call.authenticate()?;
    let request = call.request(frozen.entry.owner)?;
    let replay = &opened.state.control.replay;
    if let ReplayDecision::AlreadyApplied(retained) = replay.check(&request, ctx.height)? {
        return Ok(Outcome::Retained(retained));
    }
    let origin = opened.section.header.origin_height;
    HeightWindow::epoch(origin, opened.frozen.epoch, COMMIT_OPENS, COMMIT_CLOSES)?
        .check(ctx.height)?;
    let seal = call.claim(&frozen, &payload)?;
    if let Some(stored) = opened.region.get(opened.frozen.epoch, seal.evaluator)? {
        return if stored.same_claim(&seal) {
            Ok(Outcome::AlreadyApplied { seal: stored })
        } else {
            Err(F09_EVIDENCE_SEAL_CONFLICT)
        };
    }
    commit(&call, &seal, &request, next, scratch, event)
}

/// R10 transition of one owning record's root field.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum RootBind {
    /// The owning operation stores this root with its record.
    Bound(Digest32),
    /// The record already holds exactly this root; nothing changes.
    AlreadyBound(Digest32),
}

/// R10 immutable root binding inside an owning F01/F02/F03 operation, after that operation's
/// own authority and phase checks. `stored` is the record's current root and `frozen` whether
/// the record has frozen. The request carries only the root, never a locator or object bytes.
///
/// # Errors
/// Returns `NON_CANONICAL` for a zero mandatory root; `CONFLICT` when a different root is
/// already bound (a later upload never overwrites it); `WRONG_PHASE` when an unbound record
/// has already frozen.
pub fn bind_root(stored: Option<Digest32>, frozen: bool, root: [u8; 32]) -> CodecResult<RootBind> {
    let root = Digest32::new(root)?;
    match stored {
        Some(bound) if bound == root => Ok(RootBind::AlreadyBound(root)),
        Some(_) => Err(CONFLICT),
        None if frozen => Err(WRONG_PHASE),
        None => Ok(RootBind::Bound(root)),
    }
}

/// Offchain admission of the manifest behind a root an owning operation binds: exact context,
/// subject and kind, and the exact manifest root.
///
/// # Errors
/// Propagates `decode_manifest` refusals; returns `InvalidContext` for another network,
/// program, market, policy, epoch or subject; `UnsupportedKind` for another field kind;
/// `RootMismatch` when the manifest root differs from `root`.
pub fn check_root_binding(
    manifest: &[u8],
    context: &ArtifactContext,
    subject: SubjectContext,
    kind: ArtifactKind,
    root: [u8; 32],
) -> ArtifactResult<ArtifactManifestRoot> {
    let decoded = decode_manifest(manifest)?;
    check_manifest_context(&decoded, context, subject)?;
    if decoded.kind != kind {
        return Err(ArtifactError::UnsupportedKind);
    }
    let bound = manifest_root(manifest)?;
    if bound.bytes() != root {
        return Err(ArtifactError::RootMismatch);
    }
    Ok(bound)
}

/// One verifier conclusion; never collapsed into a single verified flag.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Conclusion {
    Holds,
    Fails(ArtifactError),
    /// The supplied proof cannot establish this property.
    Unproven,
}
impl Conclusion {
    fn of(result: ArtifactResult<()>) -> Self {
        result.map_or_else(Self::Fails, |()| Self::Holds)
    }
}

/// R19 conclusion vector of one minimal evidence proof.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Conclusions {
    pub binding: Conclusion,
    pub signatures: Conclusion,
    pub integrity: Conclusion,
    pub availability: Conclusion,
    pub rights: Conclusion,
    pub reproduction: Conclusion,
    pub quality: Conclusion,
}

/// One F02 task/result record of a minimal proof: the task's worker binding and its roots.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ProvenTask {
    pub task: TaskId,
    pub worker: WorkerId,
    pub generation: Version,
    pub model_root: [u8; 32],
    pub request_root: [u8; 32],
    pub result_root: [u8; 32],
}

/// R19 minimal proof. `seal` and `binding` come from the authenticated finality reference;
/// `report` is the admitted F03 report; `manifest` the exact evidence bytes; `tasks` the F02
/// task/result records; `publishers` the publisher envelopes the tasks require.
#[derive(Clone, Copy)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EvidenceProof<'a> {
    pub seal: EvidenceSeal,
    pub binding: EvaluatorBinding,
    pub policy: EvidencePolicy,
    pub report: &'a ReportBody<'a>,
    pub manifest: &'a [u8],
    pub tasks: &'a [ProvenTask],
    pub publishers: &'a [&'a [u8]],
}

fn proof_integrity(proof: &EvidenceProof<'_>) -> ArtifactResult<()> {
    let root = evidence_root(proof.manifest, &proof.policy)?;
    if root == proof.seal.root && root == proof.report.evidence {
        Ok(())
    } else {
        Err(ArtifactError::RootMismatch)
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn proof_context(proof: &EvidenceProof<'_>, manifest: &EvidenceManifest<'_>) -> ArtifactResult<()> {
    let seal = &proof.seal;
    let claimed = EvaluatorBinding {
        frozen: proof.binding.frozen,
        evaluator: seal.evaluator,
        grant: seal.grant,
        key_version: seal.key_version,
    };
    if manifest.binding != proof.binding
        || proof.report.binding != proof.binding
        || claimed != proof.binding
        || manifest.task_policy != seal.task_policy
        || manifest.task_set != seal.task_set
        || manifest.rubric != seal.rubric
        || manifest.mode != seal.mode
    {
        return Err(ArtifactError::InvalidContext);
    }
    Ok(())
}

/// Report score pairs equal the manifest worker groups and every manifest task matches its F02
/// record exactly; the F02 records name no other task.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn proof_records(proof: &EvidenceProof<'_>, manifest: &EvidenceManifest<'_>) -> ArtifactResult<()> {
    let mut scores = proof.report.scores.entries();
    let mut tasks = 0usize;
    for group in &manifest.groups {
        let group = group?;
        let score = scores.next().ok_or(ArtifactError::InvalidContext)??;
        if score.worker != group.worker || score.score != group.score {
            return Err(ArtifactError::InvalidContext);
        }
        for entry in &group.tasks {
            let entry = entry?;
            let record = proof
                .tasks
                .iter()
                .find(|t| t.task == entry.task)
                .ok_or(ArtifactError::InvalidContext)?;
            if record.worker != group.worker
                || record.generation != group.generation
                || record.model_root != group.model_root
                || record.request_root != entry.request_root
                || record.result_root != entry.result_root
            {
                return Err(ArtifactError::InvalidContext);
            }
            tasks += 1;
        }
    }
    if scores.next().is_some() || tasks != proof.tasks.len() {
        return Err(ArtifactError::InvalidContext);
    }
    Ok(())
}

fn proof_binding(proof: &EvidenceProof<'_>) -> ArtifactResult<()> {
    let manifest = decode_evidence_manifest(proof.manifest, &proof.policy)?;
    proof_context(proof, &manifest)?;
    proof_records(proof, &manifest)
}

/// Every publisher envelope verifies and attributes a root one proven task binds.
#[cfg_attr(target_arch = "wasm32", inline(never))]
fn proof_signatures(proof: &EvidenceProof<'_>) -> Conclusion {
    if proof.publishers.is_empty() {
        return Conclusion::Unproven;
    }
    Conclusion::of(proof.publishers.iter().try_for_each(|bytes| {
        let verified =
            verify_publisher(&decode_envelope(bytes)?).map_err(|failure| match failure {
                VerificationFailure::Artifact(error) => error,
                VerificationFailure::Host(_) => ArtifactError::SignatureInvalid,
            })?;
        let root = verified.root.bytes();
        if proof
            .tasks
            .iter()
            .any(|t| [t.model_root, t.request_root, t.result_root].contains(&root))
        {
            Ok(())
        } else {
            Err(ArtifactError::InvalidContext)
        }
    }))
}

/// Verifies a minimal evidence proof into its R19 conclusion vector. Availability, rights,
/// reproduction and quality stay `Unproven`: no root, seal or chunk proof establishes complete
/// availability, benchmark coverage or AI quality; an accepted seal is an authorized claim.
#[must_use]
pub fn verify_evidence_proof(proof: &EvidenceProof<'_>) -> Conclusions {
    Conclusions {
        binding: Conclusion::of(proof_binding(proof)),
        signatures: proof_signatures(proof),
        integrity: Conclusion::of(proof_integrity(proof)),
        availability: Conclusion::Unproven,
        rights: Conclusion::Unproven,
        reproduction: Conclusion::Unproven,
        quality: Conclusion::Unproven,
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn require_sealed_artifact_chunk_root(
    state: &SharedState<'_>,
    epoch: u64,
    manifest_bytes: &[u8],
    batch: &crate::laps::ReceiptBatch,
) -> CodecResult<Digest32> {
    let admission =
        crate::reputation_transition::joint_table(state.section(Section::ReputationAdmission)?)?;
    let view = state.replace_section(Section::ReputationAdmission, admission)?;
    let state = &view;
    let unsealed = crate::errors::F11_BATCH_UNSEALED;
    let manifest = decode_manifest(manifest_bytes).map_err(|_| unsealed)?;
    let section = committed(state)?;
    let frozen = frozen_binding(state, &section, None)?;
    if frozen.epoch != epoch {
        return Err(WRONG_EPOCH);
    }
    let task_set = tasks::sealed_task_set_with(state, epoch, None)?;
    let chunk_root = wire::sha256([
        CONTENT_DOMAIN,
        &manifest.byte_length.to_be_bytes(),
        &manifest.chunk_count.to_be_bytes(),
        batch.batch_digest.as_bytes(),
    ])?;
    let policy_digest = section.current.digest()?;
    if manifest.kind != ArtifactKind::EvaluationEvidence
        || manifest.epoch != epoch
        || manifest.context.chain != frozen.chain
        || manifest.context.program != frozen.program
        || manifest.context.market != frozen.market
        || manifest.context.policy != policy_digest
        || manifest.subject != task_set.bytes()
        || manifest.byte_length == 0
        || manifest.content_root != chunk_root
    {
        return Err(unsealed);
    }
    let summary = batch.summary_digest()?;
    let mut summary_bound = false;
    for parent in &manifest.parents {
        let parent = parent.map_err(|_| unsealed)?;
        if parent.purpose == ParentPurpose::Execution && parent.root == summary.bytes() {
            summary_bound = true;
        }
    }
    if !summary_bound {
        return Err(unsealed);
    }
    let root = ArtifactManifestRoot::new(wire::sha256([
        MANIFEST_DOMAIN,
        &manifest.context.bytes(),
        &length_u32(manifest_bytes.len()).map_err(|_| unsealed)?,
        manifest_bytes,
    ])?)
    .map_err(|_| unsealed)?;
    for seal in SealRegion::decode(state.control.feature_bytes)?.seals(epoch) {
        let seal = seal?;
        if seal.root.bytes() == root.bytes()
            && seal.task_set == task_set
            && seal.task_policy == manifest.context.policy
            && seal.rubric == section.current.commitments.rubric
            && seal.mode as u8 == section.current.assessment_mode
        {
            return Ok(task_set);
        }
    }
    Err(unsealed)
}
