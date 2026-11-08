//! Checked common identities, immutable bindings, and bounded records.
use crate::errors::{CodecResult, ARITHMETIC, F03_SCORE_RANGE, NON_CANONICAL};

/// Bounds-checked access to internal storage, trapping guest invariant failures
/// without constructing a formatted panic. Native callers retain Rust indexing.
#[doc(hidden)]
pub trait GuestIndex<T> {
    fn guest_index<I: core::slice::SliceIndex<[T]>>(&self, index: I) -> &I::Output;
    fn guest_index_mut<I: core::slice::SliceIndex<[T]>>(&mut self, index: I) -> &mut I::Output;
    fn guest_split_at(&self, mid: usize) -> (&[T], &[T]);
    fn guest_split_at_mut(&mut self, mid: usize) -> (&mut [T], &mut [T]);
    fn guest_copy_from_slice(&mut self, source: &[T])
    where
        T: Copy;
    fn guest_copy_within<R: core::ops::RangeBounds<usize>>(&mut self, source: R, dest: usize)
    where
        T: Copy;
    fn guest_swap(&mut self, a: usize, b: usize);
    fn guest_chunks_exact(&self, size: usize) -> core::slice::ChunksExact<'_, T>;
}

impl<T> GuestIndex<T> for [T] {
    #[inline(always)]
    fn guest_index<I: core::slice::SliceIndex<[T]>>(&self, index: I) -> &I::Output {
        #[cfg(target_arch = "wasm32")]
        {
            match self.get(index) {
                Some(value) => value,
                None => core::arch::wasm32::unreachable(),
            }
        }
        #[cfg(not(target_arch = "wasm32"))]
        &self[index]
    }

    #[inline(always)]
    fn guest_index_mut<I: core::slice::SliceIndex<[T]>>(&mut self, index: I) -> &mut I::Output {
        #[cfg(target_arch = "wasm32")]
        {
            match self.get_mut(index) {
                Some(value) => value,
                None => core::arch::wasm32::unreachable(),
            }
        }
        #[cfg(not(target_arch = "wasm32"))]
        &mut self[index]
    }

    #[inline(always)]
    fn guest_split_at(&self, mid: usize) -> (&[T], &[T]) {
        #[cfg(target_arch = "wasm32")]
        {
            match self.split_at_checked(mid) {
                Some(value) => value,
                None => core::arch::wasm32::unreachable(),
            }
        }
        #[cfg(not(target_arch = "wasm32"))]
        self.split_at(mid)
    }

    #[inline(always)]
    fn guest_split_at_mut(&mut self, mid: usize) -> (&mut [T], &mut [T]) {
        #[cfg(target_arch = "wasm32")]
        {
            match self.split_at_mut_checked(mid) {
                Some(value) => value,
                None => core::arch::wasm32::unreachable(),
            }
        }
        #[cfg(not(target_arch = "wasm32"))]
        self.split_at_mut(mid)
    }

    #[inline(always)]
    fn guest_copy_from_slice(&mut self, source: &[T])
    where
        T: Copy,
    {
        #[cfg(target_arch = "wasm32")]
        {
            if self.len() != source.len() {
                core::arch::wasm32::unreachable();
            }
        }
        self.copy_from_slice(source);
    }

    #[inline(always)]
    fn guest_copy_within<R: core::ops::RangeBounds<usize>>(&mut self, source: R, dest: usize)
    where
        T: Copy,
    {
        #[cfg(target_arch = "wasm32")]
        {
            use core::ops::Bound;
            let start = match source.start_bound() {
                Bound::Included(n) => *n,
                Bound::Excluded(n) => n
                    .checked_add(1)
                    .unwrap_or_else(|| core::arch::wasm32::unreachable()),
                Bound::Unbounded => 0,
            };
            let end = match source.end_bound() {
                Bound::Excluded(n) => *n,
                Bound::Included(n) => n
                    .checked_add(1)
                    .unwrap_or_else(|| core::arch::wasm32::unreachable()),
                Bound::Unbounded => self.len(),
            };
            let len = self.guest_index(start..end).len();
            let dest_end = dest
                .checked_add(len)
                .unwrap_or_else(|| core::arch::wasm32::unreachable());
            self.guest_index(dest..dest_end);
            if dest > start {
                for i in (0..len).rev() {
                    let value = *self.guest_index(start + i);
                    *self.guest_index_mut(dest + i) = value;
                }
            } else {
                for i in 0..len {
                    let value = *self.guest_index(start + i);
                    *self.guest_index_mut(dest + i) = value;
                }
            }
        }
        #[cfg(not(target_arch = "wasm32"))]
        self.copy_within(source, dest);
    }

    #[inline(always)]
    fn guest_swap(&mut self, a: usize, b: usize) {
        #[cfg(target_arch = "wasm32")]
        {
            if a == b {
                self.guest_index(a);
                return;
            }
            let (a, b) = if a < b { (a, b) } else { (b, a) };
            let (left, right) = self.guest_split_at_mut(b);
            core::mem::swap(left.guest_index_mut(a), right.guest_index_mut(0));
        }
        #[cfg(not(target_arch = "wasm32"))]
        self.swap(a, b);
    }

    #[inline(always)]
    fn guest_chunks_exact(&self, size: usize) -> core::slice::ChunksExact<'_, T> {
        #[cfg(target_arch = "wasm32")]
        if size == 0 {
            core::arch::wasm32::unreachable();
        }
        self.chunks_exact(size)
    }
}

/// Borrowed identity bytes whose nonzero invariant has already been checked.
/// Only this constructor can create the proof; typed identities copy directly
/// from the wire after the shared reader validates it.
#[derive(Clone, Copy)]
pub(crate) struct CheckedIdentityBytes<'a>(&'a [u8; 32]);
impl<'a> CheckedIdentityBytes<'a> {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub(crate) fn new(bytes: &'a [u8; 32]) -> CodecResult<Self> {
        if *bytes == [0; 32] {
            Err(NON_CANONICAL)
        } else {
            Ok(Self(bytes))
        }
    }
    pub(crate) fn bytes(self) -> &'a [u8; 32] {
        self.0
    }
}

/// Private initialization for a fully overwritten decoder destination. Seeded
/// identities stay checked even while the destination is being assembled.
pub(crate) trait InitialField {
    fn initial(seed: &Digest32) -> Self;
}
pub(crate) trait CheckedIdentity {
    fn seed(self) -> Digest32;
}
macro_rules! initial_scalar {
    ($($ty:ty => $value:expr),+ $(,)?) => {$(
        impl InitialField for $ty {
            fn initial(_: &Digest32) -> Self { $value }
        }
    )+};
}
initial_scalar! {
    u8 => 0, u16 => 0, u32 => 0, u64 => 0, u128 => 0, bool => false,
    Version => Version(core::num::NonZeroU64::MIN), Score => Score(0),
    PublicKey32 => PublicKey32([0; 32]), Signature64 => Signature64([0; 64]),
}
impl<const N: usize> InitialField for [u8; N] {
    fn initial(_: &Digest32) -> Self {
        [0; N]
    }
}
impl<T> InitialField for Presence<T> {
    fn initial(_: &Digest32) -> Self {
        Self::Absent
    }
}

macro_rules! id32 {
    ($($name:ident),+ $(,)?) => {$ (
        #[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd, Hash)]
        #[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
        pub struct $name([u8; 32]);
        impl InitialField for $name {
            #[inline]
            fn initial(seed: &Digest32) -> Self { Self::from_seed(*seed) }
        }
        impl CheckedIdentity for $name {
            fn seed(self) -> Digest32 { Digest32(self.bytes()) }
        }
        impl $name {
            fn from_seed(seed: Digest32) -> Self { Self(seed.bytes()) }
            #[inline]
            pub(crate) fn from_checked(bytes: CheckedIdentityBytes<'_>) -> Self { Self(*bytes.bytes()) }
            pub(crate) fn decode_destination(&mut self) -> &mut [u8; 32] { &mut self.0 }
            /// Builds the identity from its 32 bytes.
            ///
            /// # Errors
            /// Returns `NON_CANONICAL` when the bytes are all zero.
            #[cfg_attr(target_arch = "wasm32", inline(never))]
            pub fn new(bytes: [u8; 32]) -> CodecResult<Self> {
                if bytes == [0; 32] { Err(NON_CANONICAL) } else { Ok(Self(bytes)) }
            }
            pub const fn bytes(self) -> [u8; 32] { self.0 }
            pub const fn as_bytes(&self) -> &[u8; 32] { &self.0 }
        }
    )+};
}
id32!(
    PrincipalId,
    ProgramId,
    MarketId,
    WorkerId,
    EvaluatorId,
    AccountId,
    AssetId,
    RequestId,
    TaskId
);

#[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd, Hash)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Digest32([u8; 32]);
impl InitialField for Digest32 {
    fn initial(seed: &Digest32) -> Self {
        *seed
    }
}
impl CheckedIdentity for Digest32 {
    fn seed(self) -> Digest32 {
        self
    }
}
impl Digest32 {
    #[inline]
    pub(crate) fn from_checked(bytes: CheckedIdentityBytes<'_>) -> Self {
        Self(*bytes.bytes())
    }
    pub(crate) fn decode_destination(&mut self) -> &mut [u8; 32] {
        &mut self.0
    }
    /// Builds the digest from its 32 bytes.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the bytes are all zero.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn new(bytes: [u8; 32]) -> CodecResult<Self> {
        if bytes == [0; 32] {
            Err(NON_CANONICAL)
        } else {
            Ok(Self(bytes))
        }
    }
    #[must_use]
    pub const fn bytes(self) -> [u8; 32] {
        self.0
    }
    #[must_use]
    pub const fn as_bytes(&self) -> &[u8; 32] {
        &self.0
    }
}
macro_rules! digest {
    ($($name:ident),+ $(,)?) => {$ (
        #[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd)]
        #[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
        pub struct $name(Digest32);
        impl InitialField for $name {
            #[inline]
            fn initial(seed: &Digest32) -> Self { Self::from_seed(*seed) }
        }
        impl CheckedIdentity for $name {
            fn seed(self) -> Digest32 { Digest32(self.bytes()) }
        }
        impl $name {
            fn from_seed(seed: Digest32) -> Self { Self(seed) }
            #[inline]
            pub(crate) fn from_checked(bytes: CheckedIdentityBytes<'_>) -> Self { Self(Digest32(*bytes.bytes())) }
            pub(crate) fn decode_destination(&mut self) -> &mut [u8; 32] { &mut self.0.0 }
            /// Builds the typed digest from its 32 bytes.
            ///
            /// # Errors
            /// Returns `NON_CANONICAL` when the bytes are all zero.
            #[cfg_attr(target_arch = "wasm32", inline(never))]
            pub fn new(bytes: [u8; 32]) -> CodecResult<Self> {
                if bytes == [0; 32] { Err(NON_CANONICAL) } else { Ok(Self(Digest32(bytes))) }
            }
            pub const fn bytes(self) -> [u8; 32] { self.0.bytes() }
            pub const fn as_bytes(&self) -> &[u8; 32] { self.0.as_bytes() }
        }
    )+};
}
digest!(
    ChainDomain,
    RequestDigest,
    ReportDigest,
    AttestationDigest,
    CommitmentDigest,
    RosterDigest,
    ResultDigest,
    StateDigest,
    EvidenceRoot,
    MetadataDigest,
    RubricDigest,
    PolicyDigest
);

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Presence<T> {
    Absent,
    Present(T),
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct PublicKey32(pub [u8; 32]);
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Signature64(pub [u8; 64]);
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Salt32([u8; 32]);
impl Salt32 {
    /// Builds the salt from its 32 bytes.
    ///
    /// # Errors
    /// Returns `F04_SALT_INVALID` when the bytes are all zero.
    pub fn new(bytes: [u8; 32]) -> CodecResult<Self> {
        if bytes == [0; 32] {
            Err(crate::errors::F04_SALT_INVALID)
        } else {
            Ok(Self(bytes))
        }
    }
    #[must_use]
    pub const fn bytes(self) -> [u8; 32] {
        self.0
    }
}
#[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Version(core::num::NonZeroU64);
impl Version {
    /// Builds a version from its value.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the value is zero.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn new(value: u64) -> CodecResult<Self> {
        core::num::NonZeroU64::new(value)
            .map(Self)
            .ok_or(NON_CANONICAL)
    }
    #[must_use]
    pub const fn get(self) -> u64 {
        self.0.get()
    }
    /// Returns the following version.
    ///
    /// # Errors
    /// Returns `ARITHMETIC` when the value is `u64::MAX`.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn next(self) -> CodecResult<Self> {
        Self::new(self.get().checked_add(1).ok_or(ARITHMETIC)?)
    }
}
#[derive(Clone, Copy, Eq, PartialEq, Ord, PartialOrd)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Score(u32);
impl Score {
    /// Builds a score from its value.
    ///
    /// # Errors
    /// Returns `F03_SCORE_RANGE` when the value exceeds 1,000,000.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn new(value: u32) -> CodecResult<Self> {
        if value > 1_000_000 {
            Err(F03_SCORE_RANGE)
        } else {
            Ok(Self(value))
        }
    }
    #[must_use]
    pub const fn get(self) -> u32 {
        self.0
    }
}
pub type Amount = u128;

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct FrozenBinding {
    pub chain: ChainDomain,
    pub program: ProgramId,
    pub market: MarketId,
    pub epoch: u64,
    pub config: Version,
    pub roster: RosterDigest,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EvaluatorBinding {
    pub frozen: FrozenBinding,
    pub evaluator: EvaluatorId,
    pub grant: Version,
    pub key_version: Version,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct WorkerRosterEntry {
    pub worker: WorkerId,
    pub owner: PrincipalId,
    pub recipient: AccountId,
    pub generation: Version,
    pub key_version: Version,
    pub public_key: PublicKey32,
    pub metadata: MetadataDigest,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EvaluatorRosterEntry {
    pub evaluator: EvaluatorId,
    pub owner: PrincipalId,
    pub grant: Version,
    pub key_version: Version,
    pub public_key: PublicKey32,
    pub rubric: RubricDigest,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct ScoreEntry {
    pub worker: WorkerId,
    pub score: Score,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Authentication {
    Native,
    Delegate {
        key: PublicKey32,
        signature: Signature64,
    },
}

/// Replay assessment only; this does not authenticate callers or commit state.
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct RoleReplay {
    pub sequence: u64,
    pub request: Presence<RequestId>,
    pub digest: Presence<RequestDigest>,
    pub result: Presence<ResultDigest>,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum ReplayDecision {
    New,
    AlreadyApplied(ResultDigest),
}
impl RoleReplay {
    /// Classifies a role request against the recorded replay slot.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the stored slot is malformed; `SEQUENCE_CONSUMED` when the sequence is zero or already passed; `REPLAY_CONFLICT` when the current sequence is reused for a different request or digest; `ARITHMETIC` when the stored sequence cannot advance; `SEQUENCE_GAP` when the sequence skips ahead.
    pub fn assess(
        &self,
        sequence: u64,
        request: RequestId,
        digest: RequestDigest,
    ) -> CodecResult<ReplayDecision> {
        use crate::errors::{REPLAY_CONFLICT, SEQUENCE_CONSUMED, SEQUENCE_GAP};
        if self.sequence == 0 {
            if self.request != Presence::Absent
                || self.digest != Presence::Absent
                || self.result != Presence::Absent
            {
                return Err(NON_CANONICAL);
            }
        } else if !matches!(
            (self.request, self.digest, self.result),
            (
                Presence::Present(_),
                Presence::Present(_),
                Presence::Present(_)
            )
        ) {
            return Err(NON_CANONICAL);
        }
        if sequence == 0 || sequence < self.sequence {
            return Err(SEQUENCE_CONSUMED);
        }
        if sequence == self.sequence {
            if self.request == Presence::Present(request)
                && self.digest == Presence::Present(digest)
            {
                if let Presence::Present(result) = self.result {
                    return Ok(ReplayDecision::AlreadyApplied(result));
                }
            }
            return Err(REPLAY_CONFLICT);
        }
        if sequence != self.sequence.checked_add(1).ok_or(ARITHMETIC)? {
            return Err(SEQUENCE_GAP);
        }
        Ok(ReplayDecision::New)
    }
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct EpochWindows {
    pub start: u64,
    pub commit: u64,
    pub reveal: u64,
    pub settlement: u64,
    pub end: u64,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum EpochPhase {
    Before,
    Work,
    Commit,
    Reveal,
    Settlement,
    After,
}
impl EpochWindows {
    /// Computes the windows of `epoch` counted from `origin`.
    ///
    /// # Errors
    /// Returns `ARITHMETIC` when any window bound overflows `u64`.
    pub fn new(origin: u64, epoch: u64) -> CodecResult<Self> {
        let start = origin
            .checked_add(epoch.checked_mul(128).ok_or(ARITHMETIC)?)
            .ok_or(ARITHMETIC)?;
        Ok(Self {
            start,
            commit: start.checked_add(64).ok_or(ARITHMETIC)?,
            reveal: start.checked_add(80).ok_or(ARITHMETIC)?,
            settlement: start.checked_add(96).ok_or(ARITHMETIC)?,
            end: start.checked_add(128).ok_or(ARITHMETIC)?,
        })
    }
    #[must_use]
    pub const fn phase(self, height: u64) -> EpochPhase {
        if height < self.start {
            EpochPhase::Before
        } else if height < self.commit {
            EpochPhase::Work
        } else if height < self.reveal {
            EpochPhase::Commit
        } else if height < self.settlement {
            EpochPhase::Reveal
        } else if height < self.end {
            EpochPhase::Settlement
        } else {
            EpochPhase::After
        }
    }
}
