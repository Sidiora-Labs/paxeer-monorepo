//! AI market client: proof-bound finalized views, operation planning over the existing
//! `NativeProgramCall`, owner review and signing, and durable exact-byte submission with
//! receipt and block-commit resolution.

use std::cell::Cell;
use std::fs::{self, File, OpenOptions};
use std::io::{self, Write as _};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_crypto::disclosure::call::{decode_program_call, encode_program_call};
use layerx_crypto::disclosure::{DisclosedNativeOperation, Disclosure, DisclosureError};
use layerx_crypto::evm_transaction::{self, Eip1559Call, EvmTransactionError};
use layerx_crypto::payments::bind_call;
use layerx_crypto::signer::{sign_disclosed_call, SignError, Signer};
use layerx_crypto::VerifyError;
use layerx_programs_ai_market::codec::{
    decode_envelope, decode_roster, encode_envelope, Envelope, RosterView,
};
use layerx_programs_ai_market::dispatch::{self, CallBoundary, Operation};
use layerx_programs_ai_market::errors::{
    ApplicationError, F06_CONTRIBUTION_CONSENT_REQUIRED, F06_FUNDING_POLICY_MISMATCH,
    F06_INVALID_AMOUNT, F06_NOTHING_TO_CLAIM, F06_REFUND_RECIPIENT_MISMATCH,
    F06_UNKNOWN_WORKER_ENTITLEMENT, F06_WRONG_CLAIM_RECIPIENT, NON_CANONICAL, NOT_FOUND,
    UNAUTHORIZED, WRONG_CONFIG, WRONG_MARKET, WRONG_ROSTER,
};
use layerx_programs_ai_market::policy::{TaskPolicyV1, TASK_POLICY_BYTES};
use layerx_programs_ai_market::queries::{
    bind_snapshot, participant_rows, read_header, Availability, CaptureFacts, FinalityEvidence,
    ParticipantKind, ParticipantRow, QueryError, ReadProof, RewardField, ScoreField, ScoreStatus,
    SnapshotBinding, StateCapture, FINALIZED_RANK,
};
use layerx_programs_ai_market::registry::{market_clock, MarketClock, MarketHeader};
use layerx_programs_ai_market::registry_ops::PolicySection;
use layerx_programs_ai_market::rewards::{
    ClaimRequest, FundRequest, RefundRequest, FUNDING_POLICY_VERSION,
};
use layerx_programs_ai_market::state::{decode_shared_state, Section};
use layerx_programs_ai_market::types::{
    AccountId, AssetId, Authentication, ChainDomain, Digest32, MarketId, PolicyDigest, Presence,
    PrincipalId, ProgramId, RequestDigest, RequestId, RosterDigest, Version, WorkerId,
    WorkerRosterEntry,
};
use layerx_programs_ai_market::{MAX_ENVELOPE_BYTES, MAX_EVALUATORS, MAX_STATE_BYTES, MAX_WORKERS};
use layerx_types::activity::ActivityBuildError;
use layerx_types::ids::{Did, LengthError};
use layerx_types::intent;
use layerx_types::payload::{ModuleRegistry, PayloadError};
use layerx_types::program_call::{InvalidNativeCall, NativeProgramCall, Resources};
use layerx_types::result::{KnownResult, Retriability};

use layerx_wire::WireError;
use sha2::{Digest as _, Sha256};

use layerx_types::rung::Rung;

use crate::rpc::receipt::{
    lookup_authenticated, lookup_authenticated_with_commit, AuthenticatedLookup,
    AuthenticatedLookupContext, AuthenticatedReceipt, ReceiptError,
};
use crate::rpc::receipt_commit::ReceiptCommitEvidence;
use crate::rpc::submit::{submit_signed, Submission, SubmitError};
use crate::sequencer::{SequencerEndpoint, SequencerError};
use layerx_programs_registry::chain_proof::{
    decode::ProofOp, state::verify_state, VerifiedChainHead,
};

pub mod inference_market {
    pub use layerx_programs_ai_market as program;
    pub use program::canary::{ChallengeLapPayload, ResolveChallengePayload};
    pub use program::laps::{LapReceipt, ReceiptBatch, SettleLapsRequest};
    pub use program::placement::BookView;
    pub use program::policy::RoomProfile;
    pub use program::queries::{DealView, PeerView, ReceiptBatchView, RoomView};
    pub use program::rooms::{
        LayerSplit, OpenRoomRequest, PostAskRequest, PostBidRequest, RecordDealRequest,
    };
    pub use program::types::{Digest32, PrincipalId, RequestId};

    use super::{
        AiClientError, FinalizedSnapshot, NativeTerms, OperationRequest, PreparedOperation,
    };
    use layerx_types::payload::ModuleRegistry;
    use program::dispatch::{self, Operation};

    #[derive(Clone, Debug, Eq, PartialEq)]
    pub struct MarketOperation {
        operation: Operation,
        payload: Vec<u8>,
    }

    pub struct OperationContext<'a> {
        pub actor: PrincipalId,
        pub roster: Option<&'a [u8]>,
        pub sequence: u64,
        pub expiry: u64,
        pub request: RequestId,
        pub required_rank: u8,
    }

    impl MarketOperation {
        fn new(operation: Operation, payload: Vec<u8>) -> Result<Self, AiClientError> {
            operation.validate_payload_length(payload.len())?;
            Ok(Self { operation, payload })
        }

        pub const fn operation(&self) -> Operation {
            self.operation
        }

        pub fn payload(&self) -> &[u8] {
            &self.payload
        }

        pub fn request<'a>(&'a self, context: OperationContext<'a>) -> OperationRequest<'a> {
            OperationRequest {
                operation: self.operation,
                payload: &self.payload,
                actor: context.actor,
                roster: context.roster,
                sequence: context.sequence,
                expiry: context.expiry,
                request: context.request,
                required_rank: context.required_rank,
            }
        }

        pub fn prepare(
            &self,
            snapshot: &FinalizedSnapshot,
            registry: &ModuleRegistry,
            authority_height: u64,
            context: OperationContext<'_>,
            terms: &NativeTerms<'_>,
        ) -> Result<PreparedOperation, AiClientError> {
            snapshot.prepare(registry, authority_height, &self.request(context), terms)
        }
    }

    fn encode(
        operation: Operation,
        write: impl FnOnce(&mut [u8]) -> program::errors::CodecResult<usize>,
    ) -> Result<MarketOperation, AiClientError> {
        let mut payload = vec![0; operation.metadata().payload_max];
        let length = write(&mut payload)?;
        payload.truncate(length);
        MarketOperation::new(operation, payload)
    }

    pub fn open_room(request: &OpenRoomRequest) -> Result<MarketOperation, AiClientError> {
        encode(dispatch::OpenRoom, |out| request.encode(out))
    }

    pub fn post_ask(request: &PostAskRequest) -> Result<MarketOperation, AiClientError> {
        encode(dispatch::PostAsk, |out| request.encode(out))
    }

    pub fn post_bid(request: &PostBidRequest) -> Result<MarketOperation, AiClientError> {
        encode(dispatch::PostBid, |out| request.encode(out))
    }

    pub fn record_deal(request: &RecordDealRequest) -> Result<MarketOperation, AiClientError> {
        encode(dispatch::RecordDeal, |out| request.encode(out))
    }

    pub fn settle_laps(request: &SettleLapsRequest<'_>) -> Result<MarketOperation, AiClientError> {
        encode(dispatch::SettleLaps, |out| request.encode(out))
    }

    pub fn challenge_lap(request: &ChallengeLapPayload) -> Result<MarketOperation, AiClientError> {
        MarketOperation::new(dispatch::ChallengeLap, request.encode()?.to_vec())
    }

    pub fn resolve_challenge(
        request: &ResolveChallengePayload,
    ) -> Result<MarketOperation, AiClientError> {
        MarketOperation::new(dispatch::ResolveChallenge, request.encode()?.to_vec())
    }

    pub fn split_payout(payload: [u8; 80]) -> Result<MarketOperation, AiClientError> {
        MarketOperation::new(dispatch::SplitPayout, payload.to_vec())
    }

    pub fn lock_bond(payload: [u8; 80]) -> Result<MarketOperation, AiClientError> {
        MarketOperation::new(dispatch::LockBond, payload.to_vec())
    }

    pub fn slash_bond(payload: [u8; 104]) -> Result<MarketOperation, AiClientError> {
        MarketOperation::new(dispatch::SlashBond, payload.to_vec())
    }

    pub fn refund_bond(payload: [u8; 64]) -> Result<MarketOperation, AiClientError> {
        MarketOperation::new(dispatch::RefundBond, payload.to_vec())
    }

    pub fn close_room(room: Digest32) -> Result<MarketOperation, AiClientError> {
        MarketOperation::new(dispatch::CloseRoom, room.bytes().to_vec())
    }

    pub fn decode_room_view(bytes: &[u8]) -> Result<RoomView, AiClientError> {
        Ok(RoomView::decode(bytes)?)
    }

    pub fn decode_deal_view(bytes: &[u8]) -> Result<DealView, AiClientError> {
        Ok(DealView::decode(bytes)?)
    }

    pub fn decode_peer_view(bytes: &[u8]) -> Result<PeerView, AiClientError> {
        Ok(PeerView::decode(bytes)?)
    }

    pub fn decode_receipt_batch_view(bytes: &[u8]) -> Result<ReceiptBatchView, AiClientError> {
        Ok(ReceiptBatchView::decode(bytes)?)
    }

    pub fn decode_book_view(bytes: &[u8]) -> Result<BookView<'_>, AiClientError> {
        Ok(BookView::decode(bytes)?)
    }
}

/// A host program namespace authenticated under a verified Comet app hash.
/// The constructor verifies the real `progstor` head, manifest and AI state blob.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct VerifiedProgramState {
    block_height: u64,
    block_hash: [u8; 32],
    app_hash: [u8; 32],
    chain: ChainDomain,
    program: ProgramId,
    native_root: [u8; 32],
    digest: [u8; 32],
    revision: u64,
    total_bytes: u32,
    proof_digest: [u8; 32],
}

impl VerifiedProgramState {
    /// Verify an SDK/IAVL namespace head and its content-addressed manifest.
    /// `expected_chain_id` comes from the caller's pinned chain verifier.
    pub fn verify(
        head: &VerifiedChainHead,
        expected_chain_id: &str,
        program: ProgramId,
        namespace_head: &[u8],
        manifest: &[u8],
        state: &[u8],
        ops: &[ProofOp],
    ) -> Result<Self, AiClientError> {
        let invalid = || AiClientError::FinalityMismatch;
        if head.chain_id() != expected_chain_id
            || expected_chain_id.is_empty()
            || namespace_head.len() != 38
            || namespace_head[..2] != [0, 1]
            || manifest.len() > MAX_STATE_BYTES
            || state.len() > MAX_STATE_BYTES
        {
            return Err(invalid());
        }
        let mut key = vec![1, 0, 9];
        key.extend_from_slice(b"progstor");
        key.extend_from_slice(program.as_bytes());
        key.push(1);
        let state_height =
            verify_state(head, "lxphost", &key, namespace_head, ops).map_err(|_| invalid())?;
        let count = u32::from_be_bytes(namespace_head[2..6].try_into().map_err(|_| invalid())?);
        if count == 0
            || count > 65_536
            || Sha256::digest(manifest).as_slice() != &namespace_head[6..]
            || manifest.len() < 6
            || manifest[..2] != [0, 1]
            || manifest[2..6] != namespace_head[2..6]
        {
            return Err(invalid());
        }
        let mut remaining = &manifest[6..];
        let mut previous: &[u8] = &[];
        let mut found = false;
        for _ in 0..count {
            let length_bytes = remaining.get(..2).ok_or_else(invalid)?;
            let length = usize::from(u16::from_be_bytes(
                length_bytes.try_into().map_err(|_| invalid())?,
            ));
            remaining = &remaining[2..];
            let key = remaining.get(..length).ok_or_else(invalid)?;
            if length == 0 || length > 1024 || key <= previous {
                return Err(invalid());
            }
            previous = key;
            remaining = &remaining[length..];
            let digest = remaining.get(..32).ok_or_else(invalid)?;
            let length_bytes = remaining.get(32..36).ok_or_else(invalid)?;
            let value_length = u32::from_be_bytes(length_bytes.try_into().map_err(|_| invalid())?);
            if key == layerx_programs_ai_market::SHARED_STATE_KEY {
                if usize::try_from(value_length).ok() != Some(state.len())
                    || Sha256::digest(state).as_slice() != digest
                {
                    return Err(invalid());
                }
                found = true;
            }
            remaining = &remaining[36..];
        }
        if !remaining.is_empty() || !found {
            return Err(invalid());
        }
        let chain = ChainDomain::new(layerx_programs_ai_market::MERGED_CHAIN_DOMAIN)?;
        let header = read_header(state, chain, program)?;
        let outer = crate::storage_witness::existence(&ops[1].data).map_err(|_| invalid())?;
        let native_root = outer.value.try_into().map_err(|_| invalid())?;
        let mut hash = Sha256::new();
        for op in ops {
            hash.update((op.data.len() as u64).to_be_bytes());
            hash.update(&op.data);
        }
        // Comet header H authenticates application state H-1.
        let block_height = u64::try_from(state_height + 1).map_err(|_| invalid())?;
        Ok(Self {
            block_height,
            block_hash: head.header_hash(),
            app_hash: head.app_hash(),
            chain,
            program,
            native_root,
            digest: header.digest.bytes(),
            revision: header.revision,
            total_bytes: header.total_bytes,
            proof_digest: hash.finalize().into(),
        })
    }
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
struct ReceiptCoverageFacts {
    activity_id: [u8; 32],
    sequence: u64,
    execution_height: u64,
    resulting_root: [u8; 32],
    receipt_digest: [u8; 32],
    block_height: u64,
    block_hash: [u8; 32],
}

/// F02 live-authority freshness bound: a snapshot lagging verified authority by more refuses.
pub const AUTHORITY_FRESHNESS_HEIGHTS: u64 = 8;
/// Settlement-anchored rank; displayed only when actually achieved, never by default.
pub const SETTLEMENT_RANK: u8 = 5;
/// Guest ABI of the AI market program call.
pub const GUEST_ABI: u16 = 5;
/// Exported guest entrypoint receiving the canonical PAXAI envelope as calldata.
pub const ENTRYPOINT: &[u8] = b"layerx_call";
/// Protocol version of native program-call activities.
pub const NATIVE_CALL_PROTOCOL_VERSION: u16 = 3;
/// Programs module activity ordinal of a native program call.
pub const PROGRAM_CALL_ORDINAL: u16 = 3;

const RECORD_MAGIC: &[u8; 8] = b"PAXAIOP3";
const LEGACY_RECORD_MAGIC: &[u8; 8] = b"PAXAIOP2";
const RECORD_EXTENSION: &str = "op";
const HEX: &[u8; 16] = b"0123456789abcdef";

/// Typed client refusal; every variant keeps the exact refusal of the boundary it came from.
#[derive(Clone, Debug, Eq, PartialEq)]
pub enum AiClientError {
    Query(QueryError),
    StaleAuthority { lag: u64, limit: u64 },
    NativeCall(InvalidNativeCall),
    Activity(ActivityBuildError),
    Actor(LengthError),
    Payload(PayloadError),
    Wire(WireError),
    Transaction(EvmTransactionError),
    Disclosure(DisclosureError),
    NotNativeProgramCall,
    NotMutation,
    TransactionRequired,
    ReviewMismatch(ReviewField),
    UnauthorizedKey,
    Sign(SignError),
    Signature(VerifyError),
    Submit(SubmitError),
    Receipt(ReceiptError),
    InvalidTransition { from: OperationState },
    ReceiptMismatch,
    FinalityMismatch,
    HistoryUnavailable(EpochStatus),
    Journal(io::ErrorKind),
    CorruptRecord,
    StaleRecord,
}

macro_rules! from_refusal {
    ($($source:ty => $variant:ident),+ $(,)?) => {
        $(impl From<$source> for AiClientError {
            fn from(error: $source) -> Self {
                Self::$variant(error)
            }
        })+
    };
}

from_refusal!(
    QueryError => Query,
    InvalidNativeCall => NativeCall,
    ActivityBuildError => Activity,
    LengthError => Actor,
    PayloadError => Payload,
    WireError => Wire,
    EvmTransactionError => Transaction,
    DisclosureError => Disclosure,
    SignError => Sign,
    VerifyError => Signature,
    SubmitError => Submit,
    ReceiptError => Receipt,
);

impl From<ApplicationError> for AiClientError {
    fn from(error: ApplicationError) -> Self {
        Self::Query(QueryError::Application(error))
    }
}

impl From<io::Error> for AiClientError {
    fn from(error: io::Error) -> Self {
        Self::Journal(error.kind())
    }
}

/// Block facts plus optional opaque verified evidence. `new` retains assertions only.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct CommitFinality {
    block_height: u64,
    block_hash: Digest32,
    app_hash: Digest32,
    store_proof_digest: Digest32,
    program: Option<VerifiedProgramState>,
    coverage: Option<ReceiptCoverageFacts>,
}

impl CommitFinality {
    /// # Errors
    /// `FinalityUnavailable` for a zero block or app hash; digest validation refusals.
    pub fn new(
        block_height: u64,
        block_hash: [u8; 32],
        app_hash: [u8; 32],
        store_proof_digest: [u8; 32],
    ) -> Result<Self, AiClientError> {
        if block_hash == [0; 32] || app_hash == [0; 32] {
            return Err(QueryError::FinalityUnavailable.into());
        }
        Ok(Self {
            block_height,
            block_hash: Digest32::new(block_hash)?,
            app_hash: Digest32::new(app_hash)?,
            store_proof_digest: Digest32::new(store_proof_digest)?,
            program: None,
            coverage: None,
        })
    }

    /// Construct finality only after the node commit and namespace proofs verify.
    pub fn from_verified_program(
        head: &VerifiedChainHead,
        witness: &VerifiedProgramState,
    ) -> Result<Self, AiClientError> {
        if head.header_hash() != witness.block_hash
            || head.app_hash() != witness.app_hash
            || u64::try_from(head.height()).ok() != Some(witness.block_height)
        {
            return Err(AiClientError::FinalityMismatch);
        }
        let mut finality = Self::new(
            witness.block_height,
            witness.block_hash,
            witness.app_hash,
            witness.proof_digest,
        )?;
        finality.program = Some(*witness);
        Ok(finality)
    }

    /// Requires the authenticated receipt path to have verified signed batch inclusion,
    /// sequence coverage and the receipt's resulting root under this exact commit.
    pub fn from_verified_receipt(receipt: &AuthenticatedReceipt) -> Result<Self, AiClientError> {
        let head = receipt
            .committed_header()
            .ok_or(AiClientError::FinalityMismatch)?;
        if receipt.rung() != Rung::Committed {
            return Err(AiClientError::FinalityMismatch);
        }
        let protocol = receipt
            .receipt()
            .protocol()
            .ok_or(AiClientError::ReceiptMismatch)?;
        let receipt_digest = Sha256::digest(receipt.canonical_bytes()).into();
        let block_height =
            u64::try_from(head.height()).map_err(|_| AiClientError::FinalityMismatch)?;
        let mut finality = Self::new(
            block_height,
            head.header_hash(),
            head.app_hash(),
            receipt_digest,
        )?;
        finality.coverage = Some(ReceiptCoverageFacts {
            activity_id: receipt.activity_id(),
            sequence: receipt.global_sequence(),
            execution_height: receipt.height(),
            resulting_root: protocol.resulting_state_root(),
            receipt_digest,
            block_height,
            block_hash: head.header_hash(),
        });
        Ok(finality)
    }

    #[must_use]
    pub const fn block_height(&self) -> u64 {
        self.block_height
    }

    #[must_use]
    pub const fn block_hash(&self) -> Digest32 {
        self.block_hash
    }

    #[must_use]
    pub const fn app_hash(&self) -> Digest32 {
        self.app_hash
    }

    #[must_use]
    pub const fn store_proof_digest(&self) -> Digest32 {
        self.store_proof_digest
    }
}

/// Age of a snapshot against independently verified live authority.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum Freshness {
    Current,
    Stale { lag: u64 },
    Unknown,
}

impl Freshness {
    #[must_use]
    pub const fn code(self) -> u8 {
        match self {
            Self::Current => 1,
            Self::Stale { .. } => 2,
            Self::Unknown => 3,
        }
    }

    const fn of(execution_height: u64, authority_height: Option<u64>) -> Self {
        match authority_height {
            None => Self::Unknown,
            Some(authority) => {
                let lag = authority.saturating_sub(execution_height);
                if lag > AUTHORITY_FRESHNESS_HEIGHTS {
                    Self::Stale { lag }
                } else {
                    Self::Current
                }
            }
        }
    }
}

/// A complete same-root `ProgramRead` capture; evidence-verified but not yet finalized.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ObservedSnapshot {
    state: Vec<u8>,
    facts: CaptureFacts,
}

impl ObservedSnapshot {
    /// Reassembles every chunk of one root in offset order and checks the market header.
    ///
    /// # Errors
    /// Capture refusals (`SnapshotConflict`, `IntegrityFailure`, `CAPACITY`) and header refusals.
    pub fn capture(chunks: &[(ReadProof, Vec<u8>)]) -> Result<Self, AiClientError> {
        let mut buffer = vec![0; MAX_STATE_BYTES];
        let mut capture = StateCapture::new(&mut buffer);
        for (proof, body) in chunks {
            capture.accept(proof, body)?;
        }
        let (bytes, facts) = capture.finish()?;
        let state = bytes.to_vec();
        read_header(&state, facts.proof.chain, facts.proof.program)?;
        Ok(Self { state, facts })
    }

    #[must_use]
    pub const fn facts(&self) -> &CaptureFacts {
        &self.facts
    }

    /// Promotes the capture with a block commit covering its observed height.
    ///
    /// # Errors
    /// `FinalityUnavailable` without evidence, `BindingMismatch` for a commit below the
    /// observed height, and binding refusals.
    pub fn finalize(
        self,
        finality: Option<&CommitFinality>,
        publication_time_ms: u64,
    ) -> Result<FinalizedSnapshot, AiClientError> {
        let finality = finality.ok_or(QueryError::FinalityUnavailable)?;
        let verified = finality.program.ok_or(QueryError::FinalityUnavailable)?;
        if finality.block_height.checked_sub(1) != Some(self.facts.proof.execution_height)
            || verified.chain != self.facts.proof.chain
            || verified.program != self.facts.proof.program
            || verified.native_root != self.facts.proof.native_state_root.bytes()
            || verified.digest != self.facts.digest.bytes()
            || verified.revision != self.facts.revision
            || verified.total_bytes != self.facts.total_bytes
            || verified.block_hash != self.facts.proof.block_hash.bytes()
        {
            return Err(QueryError::BindingMismatch.into());
        }
        let evidence = FinalityEvidence {
            native_state_root: self.facts.proof.native_state_root,
            block_height: finality.block_height,
            block_hash: finality.block_hash,
            app_hash: finality.app_hash,
            store_proof_digest: finality.store_proof_digest,
            rank: FINALIZED_RANK,
        };
        let binding = bind_snapshot(&self.state, &self.facts, &evidence, publication_time_ms)?;
        binding.require_finalized()?;
        let snapshot_id = binding.snapshot_id()?;
        let header = market_header(&self.state)?;
        Ok(FinalizedSnapshot {
            state: self.state,
            header,
            binding,
            snapshot_id,
            commit: *finality,
        })
    }
}

fn market_header(state: &[u8]) -> Result<MarketHeader, AiClientError> {
    let shared = decode_shared_state(state)?;
    Ok(PolicySection::decode(shared.section(Section::PolicyLifecycle)?)?.header)
}

/// Decodes a canonical v2 binding, including its block-commit evidence.
/// The encoded length follows field presence, up to `BINDING_MAX_BYTES`.
///
/// # Errors
/// `BAD_VERSION` for a schema version other than 1; `NON_CANONICAL` for a length
/// above `BINDING_MAX_BYTES`, malformed bytes, invalid identities, revision zero
/// or a rank above 4; `BindingMismatch` when the commit predates execution.
pub fn decode_snapshot_binding(bytes: &[u8]) -> Result<SnapshotBinding, AiClientError> {
    let binding = SnapshotBinding::decode(bytes)?;
    if binding.block_height < binding.execution_height {
        return Err(QueryError::BindingMismatch.into());
    }
    Ok(binding)
}

/// Public inspection of a finalized snapshot; staleness is labelled, never hidden.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Inspection {
    pub binding: SnapshotBinding,
    pub snapshot_id: Digest32,
    pub freshness: Freshness,
    pub clock: MarketClock,
}

/// A rank-4 snapshot bound to its exact root, state and block commit.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct FinalizedSnapshot {
    state: Vec<u8>,
    header: MarketHeader,
    binding: SnapshotBinding,
    snapshot_id: Digest32,
    commit: CommitFinality,
}

/// Exact common request of one F01..F09 mutation.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct OperationRequest<'a> {
    pub operation: Operation,
    pub payload: &'a [u8],
    pub actor: PrincipalId,
    pub roster: Option<&'a [u8]>,
    pub sequence: u64,
    pub expiry: u64,
    pub request: RequestId,
    pub required_rank: u8,
}

/// Programs call and EIP-1559 terms selected by the caller.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct NativeTerms<'a> {
    pub network_id: u32,
    pub actor_did: &'a [u8],
    pub owner_public_key: [u8; 32],
    pub account_sequence: u64,
    pub idempotency_key: [u8; 32],
    pub not_before: u64,
    pub not_after: u64,
    pub fee_limit: u128,
    pub capabilities: &'a [u8],
    pub access_declaration: &'a [u8],
    pub response_capacity: u32,
    pub resources: Resources,
    pub gas_limit: u64,
    pub max_fee_per_gas: u128,
    pub max_priority_fee_per_gas: u128,
}

/// Previewed semantic effect; advisory, re-checked by the program at execution.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum OperationEffect {
    Fund {
        amount: u128,
        asset: AssetId,
        account: AccountId,
        refund_recipient: AccountId,
    },
    Claim {
        worker: WorkerId,
        recipient: AccountId,
        amount: u128,
        asset: AssetId,
    },
    RefundFree {
        expected_refunded: u128,
        amount: u128,
        recipient: AccountId,
        asset: AssetId,
    },
    Other {
        selector: u16,
    },
}

impl FinalizedSnapshot {
    #[must_use]
    pub const fn commit(&self) -> &CommitFinality {
        &self.commit
    }

    #[must_use]
    pub const fn binding(&self) -> &SnapshotBinding {
        &self.binding
    }

    #[must_use]
    pub const fn snapshot_id(&self) -> Digest32 {
        self.snapshot_id
    }

    #[must_use]
    pub const fn header(&self) -> &MarketHeader {
        &self.header
    }

    #[must_use]
    pub fn state_bytes(&self) -> &[u8] {
        &self.state
    }

    /// Exact snapshot with its freshness label and the epoch clock at its execution height.
    ///
    /// # Errors
    /// `ARITHMETIC` when the execution height precedes the market origin.
    pub fn inspect(&self, authority_height: Option<u64>) -> Result<Inspection, AiClientError> {
        Ok(Inspection {
            binding: self.binding,
            snapshot_id: self.snapshot_id,
            freshness: Freshness::of(self.binding.execution_height, authority_height),
            clock: market_clock(self.header.origin_height, self.binding.execution_height)?,
        })
    }

    /// Refuses routing, signing or release decisions on stale authority.
    ///
    /// # Errors
    /// `StaleAuthority` when verified authority leads the snapshot by more than the bound.
    pub fn require_current(&self, authority_height: u64) -> Result<(), AiClientError> {
        match Freshness::of(self.binding.execution_height, Some(authority_height)) {
            Freshness::Stale { lag } => Err(AiClientError::StaleAuthority {
                lag,
                limit: AUTHORITY_FRESHNESS_HEIGHTS,
            }),
            Freshness::Current | Freshness::Unknown => Ok(()),
        }
    }

    /// One participant row by stable identifier; a reused seat never inherits another id.
    ///
    /// # Errors
    /// `NOT_FOUND` for an unknown identifier, `WRONG_MARKET`/`WRONG_CONFIG` for a foreign
    /// roster, and row derivation refusals.
    pub fn participant(
        &self,
        kind: ParticipantKind,
        id: [u8; 32],
        roster: Option<&[u8]>,
    ) -> Result<ParticipantRow, AiClientError> {
        let view = roster.map(decode_roster).transpose()?;
        if let Some(view) = &view {
            self.check_roster(view)?;
        }
        let filler = ParticipantRow {
            kind,
            id,
            owner: self.header.owner_principal,
            generation: 0,
            identity_state: 0,
            frozen_member: false,
            frozen_generation: Presence::Absent,
            eligibility: 0,
            metadata: Presence::Absent,
            metadata_revision: 0,
            score: ScoreField::absent(Presence::Absent, ScoreStatus::NotProduced)?,
            reward: RewardField::unavailable(Availability::NotEnabled)?,
            history_status: Availability::NotYetProduced,
            history: Presence::Absent,
        };
        let mut rows = vec![filler; MAX_WORKERS + MAX_EVALUATORS];
        let count = participant_rows(&self.state, view.as_ref(), &mut rows)?;
        rows.truncate(count);
        rows.into_iter()
            .find(|row| row.kind == kind && row.id == id)
            .ok_or_else(|| NOT_FOUND.into())
    }

    fn check_roster(&self, view: &RosterView<'_>) -> Result<(), AiClientError> {
        if view.market != self.binding.market {
            return Err(WRONG_MARKET.into());
        }
        if view.config != self.binding.config {
            return Err(WRONG_CONFIG.into());
        }
        Ok(())
    }

    /// Plans one mutation over this finalized context: the canonical PAXAI envelope becomes
    /// the calldata of a `NativeProgramCall` inside an unsigned EIP-1559 transaction.
    ///
    /// # Errors
    /// `StaleAuthority`, `FinalityUnavailable` for an unmet rank, `NotMutation` for a read
    /// selector, envelope/roster refusals, typed preview refusals and native encoding refusals.
    pub fn prepare(
        &self,
        _registry: &ModuleRegistry,
        authority_height: u64,
        request: &OperationRequest<'_>,
        terms: &NativeTerms<'_>,
    ) -> Result<PreparedOperation, AiClientError> {
        self.require_current(authority_height)?;
        if !(FINALIZED_RANK..=SETTLEMENT_RANK).contains(&request.required_rank) {
            return Err(NON_CANONICAL.into());
        }
        if request.required_rank > self.binding.rank {
            return Err(QueryError::FinalityUnavailable.into());
        }
        if request.operation.metadata().boundary != CallBoundary::Mutation {
            return Err(AiClientError::NotMutation);
        }
        let roster = request.roster.map(decode_roster).transpose()?;
        let (epoch, roster_digest) = match &roster {
            Some(view) => {
                self.check_roster(view)?;
                (view.epoch, Presence::Present(view.digest()?))
            }
            None => (0, Presence::Absent),
        };
        let envelope = Envelope {
            operation: request.operation,
            chain: self.binding.chain,
            program: self.binding.program,
            market: self.binding.market,
            actor: request.actor,
            epoch,
            config: self.binding.config.get(),
            roster: roster_digest,
            sequence: request.sequence,
            expiry: request.expiry,
            request: request.request,
            payload: request.payload,
            authentication: Authentication::Native,
        };
        let mut calldata = vec![0; MAX_ENVELOPE_BYTES];
        let length = encode_envelope(&envelope, &mut calldata)?;
        calldata.truncate(length);
        let intent = decode_envelope(&calldata)?.request_digest()?;
        let effect = preview(&self.header, &envelope, roster.as_ref())?;
        Did::new(terms.actor_did)?;
        if terms.network_id != 125
            || terms.not_before != 0
            || terms.not_after != request.expiry
            || terms.idempotency_key != request.request.bytes()
            || terms.gas_limit == 0
            || terms.max_priority_fee_per_gas > terms.max_fee_per_gas
            || u128::from(terms.gas_limit)
                .checked_mul(terms.max_fee_per_gas)
                .is_none()
        {
            return Err(AiClientError::ReviewMismatch(ReviewField::Transaction));
        }
        if terms.owner_public_key == [0; 32] {
            return Err(AiClientError::UnauthorizedKey);
        }
        let native = NativeProgramCall {
            program_id: intent::ProgramId::new(self.binding.program.bytes()),
            guest_abi: GUEST_ABI,
            entrypoint: ENTRYPOINT,
            calldata: &calldata,
            capabilities: terms.capabilities,
            access_declaration: terms.access_declaration,
            response_capacity: terms.response_capacity,
            resources: terms.resources,
        };
        let native_bytes = native.encode()?;
        let mut target = [0; 20];
        target[18..].copy_from_slice(&[0x10, 0x1d]);
        let call = Eip1559Call {
            chain_id: 125,
            nonce: terms.account_sequence,
            gas_limit: terms.gas_limit,
            max_fee_per_gas: terms.max_fee_per_gas,
            max_priority_fee_per_gas: terms.max_priority_fee_per_gas,
            to: target,
            data: encode_program_call(&native, terms.fee_limit),
        };
        let canonical = call.encode_unsigned();
        Ok(PreparedOperation {
            call,
            canonical,
            native: native_bytes,
            fee_limit: terms.fee_limit,
            owner_public_key: terms.owner_public_key,
            binding: self.binding,
            snapshot_id: self.snapshot_id,
            header: self.header,
            intent,
            effect,
        })
    }
}

/// Semantic effect decoded from the exact payload, without any precondition.
fn effect_of(
    header: &MarketHeader,
    envelope: &Envelope<'_>,
) -> Result<OperationEffect, AiClientError> {
    let asset = header.funding_asset;
    Ok(match envelope.operation {
        dispatch::FUND => {
            let fund = FundRequest::decode(envelope.payload)?;
            OperationEffect::Fund {
                amount: fund.amount,
                asset,
                account: header.rewards_account,
                refund_recipient: fund.refund_recipient,
            }
        }
        dispatch::CLAIM => {
            let claim = ClaimRequest::decode(envelope.payload)?;
            OperationEffect::Claim {
                worker: claim.worker,
                recipient: claim.recipient,
                amount: claim.amount,
                asset,
            }
        }
        dispatch::REFUND_FREE => {
            let refund = RefundRequest::decode(envelope.payload)?;
            OperationEffect::RefundFree {
                expected_refunded: refund.expected_refunded,
                amount: refund.amount,
                recipient: refund.recipient,
                asset,
            }
        }
        other => OperationEffect::Other {
            selector: other.selector(),
        },
    })
}

/// Advisory preview in the program's own refusal order for the checks it can see.
fn preview(
    header: &MarketHeader,
    envelope: &Envelope<'_>,
    roster: Option<&RosterView<'_>>,
) -> Result<OperationEffect, AiClientError> {
    let effect = effect_of(header, envelope)?;
    match effect {
        OperationEffect::Fund {
            amount,
            refund_recipient,
            ..
        } => {
            let fund = FundRequest::decode(envelope.payload)?;
            if envelope.actor != header.owner_principal
                && header.treasury_principal != Presence::Present(envelope.actor)
            {
                return Err(UNAUTHORIZED.into());
            }
            if fund.policy_version != FUNDING_POLICY_VERSION {
                return Err(F06_FUNDING_POLICY_MISMATCH.into());
            }
            if !fund.consent {
                return Err(F06_CONTRIBUTION_CONSENT_REQUIRED.into());
            }
            if refund_recipient != header.refund_recipient_account {
                return Err(F06_REFUND_RECIPIENT_MISMATCH.into());
            }
            if amount == 0 {
                return Err(F06_INVALID_AMOUNT.into());
            }
        }
        OperationEffect::Claim {
            worker,
            recipient,
            amount,
            ..
        } => {
            let entry = roster_worker(roster.ok_or(WRONG_ROSTER)?, worker)?
                .ok_or(F06_UNKNOWN_WORKER_ENTITLEMENT)?;
            if recipient != entry.recipient {
                return Err(F06_WRONG_CLAIM_RECIPIENT.into());
            }
            if amount == 0 {
                return Err(F06_NOTHING_TO_CLAIM.into());
            }
        }
        OperationEffect::RefundFree {
            amount, recipient, ..
        } => {
            if recipient != header.refund_recipient_account {
                return Err(F06_REFUND_RECIPIENT_MISMATCH.into());
            }
            if amount == 0 {
                return Err(F06_INVALID_AMOUNT.into());
            }
        }
        OperationEffect::Other { .. } => {}
    }
    Ok(effect)
}

fn roster_worker(
    view: &RosterView<'_>,
    worker: WorkerId,
) -> Result<Option<WorkerRosterEntry>, AiClientError> {
    for index in 0..view.worker_count() {
        let entry = view.worker(index)?;
        if entry.worker == worker {
            return Ok(Some(entry));
        }
    }
    Ok(None)
}

/// Planned operation: exact unsigned type-2 transaction and its snapshot binding.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct PreparedOperation {
    call: Eip1559Call,
    canonical: Vec<u8>,
    native: Vec<u8>,
    fee_limit: u128,
    owner_public_key: [u8; 32],
    binding: SnapshotBinding,
    snapshot_id: Digest32,
    header: MarketHeader,
    intent: RequestDigest,
    effect: OperationEffect,
}

/// The review field whose approved value differs from the bytes about to be signed.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ReviewField {
    Action,
    Chain,
    Program,
    Market,
    Actor,
    Epoch,
    Config,
    Roster,
    Policy,
    Snapshot,
    Amount,
    Asset,
    Payee,
    Worker,
    Capabilities,
    AccessDeclaration,
    ResponseCapacity,
    Resources,
    FeeLimit,
    Validity,
    Expiry,
    IdempotencyKey,
    Authority,
    Commitment,
    Transaction,
}

/// Every term the owner approves, derived from the disclosure of the exact canonical bytes.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct ApprovalTerms {
    pub action: u16,
    pub chain: ChainDomain,
    pub program: ProgramId,
    pub market: MarketId,
    pub actor: PrincipalId,
    pub epoch: u64,
    pub config: u64,
    pub roster: Presence<RosterDigest>,
    pub policy: PolicyDigest,
    pub snapshot: Digest32,
    pub effect: OperationEffect,
    pub capabilities: Vec<u8>,
    pub access_declaration: Vec<u8>,
    pub response_capacity: u32,
    pub resources: Resources,
    pub fee_limit: u128,
    pub not_before: u64,
    pub not_after: u64,
    pub expiry: u64,
    pub idempotency_key: [u8; 32],
    pub authority: Vec<u8>,
    pub commitment: [u8; 32],
    pub transaction: Option<(Eip1559Call, [u8; 20])>,
}

impl PreparedOperation {
    #[must_use]
    pub fn canonical_bytes(&self) -> &[u8] {
        &self.canonical
    }

    #[must_use]
    pub const fn binding(&self) -> &SnapshotBinding {
        &self.binding
    }

    #[must_use]
    pub const fn snapshot_id(&self) -> Digest32 {
        self.snapshot_id
    }

    #[must_use]
    pub const fn intent(&self) -> RequestDigest {
        self.intent
    }

    #[must_use]
    pub const fn effect(&self) -> OperationEffect {
        self.effect
    }

    #[must_use]
    pub fn payload_digest(&self) -> [u8; 32] {
        Sha256::digest(&self.native).into()
    }

    #[must_use]
    pub const fn expected_revision(&self) -> u64 {
        self.binding.revision
    }

    /// Binds the exact programs-precompile transaction and its sender before owner approval.
    ///
    /// # Errors
    /// Refuses a transaction differing from the plan or unsupported by the call disclosure binder.
    pub fn review_call(
        self,
        registry: &ModuleRegistry,
        call: Eip1559Call,
        from: [u8; 20],
    ) -> Result<Review, AiClientError> {
        if call != self.call {
            return Err(AiClientError::ReviewMismatch(ReviewField::Transaction));
        }
        self.review(registry, from)
    }

    /// Reviews the stored transaction for the EVM sender that will sign it.
    ///
    /// # Errors
    /// Disclosure, program identity, domain, intent or effect mismatches.
    pub fn review(
        self,
        _registry: &ModuleRegistry,
        from: [u8; 20],
    ) -> Result<Review, AiClientError> {
        let native = NativeProgramCall::decode(&self.native)?;
        if evm_transaction::decode_unsigned(&self.canonical)? != self.call
            || self.call.data != encode_program_call(&native, self.fee_limit)
        {
            return Err(AiClientError::ReviewMismatch(ReviewField::Transaction));
        }
        let (decoded, fee_limit) = decode_program_call(&self.call.data)?;
        if decoded.native() != native || fee_limit != self.fee_limit {
            return Err(AiClientError::ReviewMismatch(ReviewField::Transaction));
        }
        let disclosure = bind_call(&self.call, from)?;
        let Some(DisclosedNativeOperation::ProgramCall(call)) = &disclosure.native_operation else {
            return Err(AiClientError::NotNativeProgramCall);
        };
        if call.native() != native
            || call.guest_abi != GUEST_ABI
            || call.entrypoint != ENTRYPOINT
            || call.program_id.bytes() != self.binding.program.bytes()
        {
            return Err(AiClientError::NotNativeProgramCall);
        }
        let validated = decode_envelope(&call.calldata)?;
        let envelope = validated.envelope;
        envelope.check_domain(
            self.binding.chain,
            self.binding.program,
            self.binding.market,
        )?;
        if validated.request_digest()? != self.intent
            || effect_of(&self.header, &envelope)? != self.effect
            || disclosure.fee_limit != u128::from(self.call.gas_limit) * self.call.max_fee_per_gas
            || disclosure.authority != from
            || disclosure.idempotency_key != envelope.request.bytes()
            || disclosure.expiry.not_before != 0
            || disclosure.expiry.not_after != envelope.expiry
            || disclosure.expiry.payload_expires_at != envelope.expiry
        {
            return Err(QueryError::IntegrityFailure.into());
        }
        let terms = ApprovalTerms {
            action: envelope.operation.selector(),
            chain: envelope.chain,
            program: envelope.program,
            market: envelope.market,
            actor: envelope.actor,
            epoch: envelope.epoch,
            config: envelope.config,
            roster: envelope.roster,
            policy: self.binding.policy,
            snapshot: self.snapshot_id,
            effect: self.effect,
            capabilities: call.capabilities.clone(),
            access_declaration: call.access_declaration.clone(),
            response_capacity: call.response_capacity,
            resources: call.resources,
            fee_limit: self.fee_limit,
            not_before: disclosure.expiry.not_before,
            not_after: disclosure.expiry.not_after,
            expiry: envelope.expiry,
            idempotency_key: disclosure.idempotency_key,
            authority: disclosure.authority.clone(),
            commitment: disclosure.audit_digest()?,
            transaction: Some((self.call.clone(), from)),
        };
        Ok(Review {
            prepared: self,
            disclosure,
            terms,
        })
    }
}

/// The disclosure-derived terms presented to the owner for one prepared operation.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Review {
    prepared: PreparedOperation,
    disclosure: Disclosure,
    terms: ApprovalTerms,
}

/// Owner approval of exactly the reviewed terms by the prepared operation's owner key.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct Approval {
    review: Review,
    key: [u8; 32],
}

impl Review {
    #[must_use]
    pub const fn terms(&self) -> &ApprovalTerms {
        &self.terms
    }

    #[must_use]
    pub const fn prepared(&self) -> &PreparedOperation {
        &self.prepared
    }

    /// Accepts the owner's approved terms only when every term equals the disclosed bytes and
    /// the approving key is the prepared operation's owner key.
    ///
    /// # Errors
    /// `ReviewMismatch` naming the first differing term, then `UnauthorizedKey`.
    pub fn approve(
        self,
        approved: &ApprovalTerms,
        key: [u8; 32],
    ) -> Result<Approval, AiClientError> {
        if let Some(field) = first_difference(&self.terms, approved) {
            return Err(AiClientError::ReviewMismatch(field));
        }
        if self.prepared.owner_public_key != key {
            return Err(AiClientError::UnauthorizedKey);
        }
        let (call, from) = self
            .terms
            .transaction
            .as_ref()
            .ok_or(AiClientError::TransactionRequired)?;
        if call != &self.prepared.call || bind_call(call, *from)? != self.disclosure {
            return Err(AiClientError::ReviewMismatch(ReviewField::Transaction));
        }
        Ok(Approval { review: self, key })
    }
}

fn first_difference(actual: &ApprovalTerms, approved: &ApprovalTerms) -> Option<ReviewField> {
    let binding = [
        (actual.action == approved.action, ReviewField::Action),
        (actual.chain == approved.chain, ReviewField::Chain),
        (actual.program == approved.program, ReviewField::Program),
        (actual.market == approved.market, ReviewField::Market),
        (actual.actor == approved.actor, ReviewField::Actor),
        (actual.epoch == approved.epoch, ReviewField::Epoch),
        (actual.config == approved.config, ReviewField::Config),
        (actual.roster == approved.roster, ReviewField::Roster),
        (actual.policy == approved.policy, ReviewField::Policy),
        (actual.snapshot == approved.snapshot, ReviewField::Snapshot),
    ];
    let native = [
        (
            actual.capabilities == approved.capabilities,
            ReviewField::Capabilities,
        ),
        (
            actual.access_declaration == approved.access_declaration,
            ReviewField::AccessDeclaration,
        ),
        (
            actual.response_capacity == approved.response_capacity,
            ReviewField::ResponseCapacity,
        ),
        (
            actual.resources == approved.resources,
            ReviewField::Resources,
        ),
        (
            actual.fee_limit == approved.fee_limit,
            ReviewField::FeeLimit,
        ),
        (
            actual.not_before == approved.not_before && actual.not_after == approved.not_after,
            ReviewField::Validity,
        ),
        (actual.expiry == approved.expiry, ReviewField::Expiry),
        (
            actual.idempotency_key == approved.idempotency_key,
            ReviewField::IdempotencyKey,
        ),
        (
            actual.authority == approved.authority,
            ReviewField::Authority,
        ),
        (
            actual.commitment == approved.commitment,
            ReviewField::Commitment,
        ),
        (
            actual.transaction == approved.transaction,
            ReviewField::Transaction,
        ),
    ];
    let unequal = |checks: &[(bool, ReviewField)]| {
        checks
            .iter()
            .find(|(equal, _)| !equal)
            .map(|(_, field)| *field)
    };
    unequal(&binding)
        .or_else(|| effect_difference(&actual.effect, &approved.effect))
        .or_else(|| unequal(&native))
}

fn effect_difference(actual: &OperationEffect, approved: &OperationEffect) -> Option<ReviewField> {
    let checks: Vec<(bool, ReviewField)> = match (actual, approved) {
        (
            OperationEffect::Fund {
                amount,
                asset,
                account,
                refund_recipient,
            },
            OperationEffect::Fund {
                amount: approved_amount,
                asset: approved_asset,
                account: approved_account,
                refund_recipient: approved_refund,
            },
        ) => vec![
            (amount == approved_amount, ReviewField::Amount),
            (asset == approved_asset, ReviewField::Asset),
            (
                account == approved_account && refund_recipient == approved_refund,
                ReviewField::Payee,
            ),
        ],
        (
            OperationEffect::Claim {
                worker,
                recipient,
                amount,
                asset,
            },
            OperationEffect::Claim {
                worker: approved_worker,
                recipient: approved_recipient,
                amount: approved_amount,
                asset: approved_asset,
            },
        ) => vec![
            (amount == approved_amount, ReviewField::Amount),
            (asset == approved_asset, ReviewField::Asset),
            (recipient == approved_recipient, ReviewField::Payee),
            (worker == approved_worker, ReviewField::Worker),
        ],
        (
            OperationEffect::RefundFree {
                expected_refunded,
                amount,
                recipient,
                asset,
            },
            OperationEffect::RefundFree {
                expected_refunded: approved_refunded,
                amount: approved_amount,
                recipient: approved_recipient,
                asset: approved_asset,
            },
        ) => vec![
            (
                amount == approved_amount && expected_refunded == approved_refunded,
                ReviewField::Amount,
            ),
            (asset == approved_asset, ReviewField::Asset),
            (recipient == approved_recipient, ReviewField::Payee),
        ],
        (
            OperationEffect::Other { selector },
            OperationEffect::Other {
                selector: approved_selector,
            },
        ) => {
            vec![(selector == approved_selector, ReviewField::Action)]
        }
        _ => vec![(false, ReviewField::Action)],
    };
    checks
        .into_iter()
        .find(|(equal, _)| !equal)
        .map(|(_, field)| field)
}

impl Approval {
    #[must_use]
    pub const fn review(&self) -> &Review {
        &self.review
    }

    /// Signs the approved EIP-1559 call and verifies its recovered sender.
    ///
    /// # Errors
    /// `TransactionRequired` without a call review, freshness, key, disclosure or signature refusals.
    pub async fn sign(
        self,
        signer: &dyn Signer,
        _registry: &ModuleRegistry,
        authority_height: u64,
    ) -> Result<OperationRecord, AiClientError> {
        let prepared = &self.review.prepared;
        let lag = authority_height.saturating_sub(prepared.binding.execution_height);
        if lag > AUTHORITY_FRESHNESS_HEIGHTS {
            return Err(AiClientError::StaleAuthority {
                lag,
                limit: AUTHORITY_FRESHNESS_HEIGHTS,
            });
        }
        if signer.public_key() != self.key {
            return Err(AiClientError::UnauthorizedKey);
        }
        let (call, from) = self
            .review
            .terms
            .transaction
            .as_ref()
            .ok_or(AiClientError::TransactionRequired)?;
        let disclosure = &self.review.disclosure;
        if signer.evm_address() != Some(*from) {
            return Err(AiClientError::UnauthorizedKey);
        }
        let signature = sign_disclosed_call(signer, call, disclosure, *from).await?;
        if evm_transaction::recover_sender(call, &signature)? != *from {
            return Err(AiClientError::UnauthorizedKey);
        }
        OperationRecord::from_signed_transaction(
            evm_transaction::assemble(call, &signature),
            prepared.intent.bytes(),
            self.review.terms.not_after,
        )
    }
}

/// SDK lifecycle of one native operation; distinct from the AI domain job status.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum OperationState {
    Prepared = 1,
    Reviewed = 2,
    Signed = 3,
    Submitting = 4,
    Pending = 5,
    Executed = 6,
    Finalized = 7,
    Failed = 8,
    Unknown = 9,
}

impl OperationState {
    #[must_use]
    pub const fn code(self) -> u8 {
        self as u8
    }

    /// # Errors
    /// `CorruptRecord` for an undefined code.
    pub const fn from_code(code: u8) -> Result<Self, AiClientError> {
        Ok(match code {
            1 => Self::Prepared,
            2 => Self::Reviewed,
            3 => Self::Signed,
            4 => Self::Submitting,
            5 => Self::Pending,
            6 => Self::Executed,
            7 => Self::Finalized,
            8 => Self::Failed,
            9 => Self::Unknown,
            _ => return Err(AiClientError::CorruptRecord),
        })
    }
}

/// AI domain job status; a finalized native operation can still have a pending job.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum DomainStatus {
    Pending = 1,
    Completed = 2,
}

impl DomainStatus {
    #[must_use]
    pub const fn code(self) -> u8 {
        self as u8
    }

    /// # Errors
    /// `CorruptRecord` for an undefined code.
    pub const fn from_code(code: u8) -> Result<Self, AiClientError> {
        match code {
            1 => Ok(Self::Pending),
            2 => Ok(Self::Completed),
            _ => Err(AiClientError::CorruptRecord),
        }
    }
}

struct SignedIdentity {
    activity_id: [u8; 32],
    idempotency_key: [u8; 32],
    protocol_version: u16,
    network_id: u32,
    not_after: u64,
}

fn transaction_identity(
    raw: &[u8],
    not_after: u64,
) -> Result<(SignedIdentity, [u8; 32]), AiClientError> {
    let (call, signature) = evm_transaction::decode_signed(raw)?;
    let sender = evm_transaction::recover_sender(&call, &signature)?;
    let mut key = [0; 32];
    key[12..].copy_from_slice(&sender);
    let tx_hash = evm_transaction::transaction_hash(raw);
    Ok((
        SignedIdentity {
            activity_id: tx_hash,
            idempotency_key: tx_hash,
            protocol_version: 2,
            network_id: u32::try_from(call.chain_id).map_err(|_| AiClientError::CorruptRecord)?,
            not_after,
        },
        key,
    ))
}

/// Durable record of one signed operation; the signed bytes never change after signing.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OperationRecord {
    generation: Cell<[u8; 32]>,
    state: OperationState,
    domain: DomainStatus,
    protocol_version: u16,
    network_id: u32,
    activity_id: [u8; 32],
    idempotency_key: [u8; 32],
    intent: [u8; 32],
    signer_public_key: [u8; 32],
    not_after: u64,
    attempt: u32,
    result_code: Option<i32>,
    global_sequence: Option<u64>,
    execution_height: Option<u64>,
    receipt_root: Option<[u8; 32]>,
    receipt_digest: Option<[u8; 32]>,
    coverage: Option<ReceiptCoverageFacts>,
    checkpoint: Option<[u8; 32]>,
    rung: Option<Rung>,
    signed_bytes: Vec<u8>,
}

impl OperationRecord {
    /// Retains a canonical signed EIP-1559 transaction for exact-byte delivery.
    /// `not_after` is the operation's expiry approved by the caller.
    ///
    /// # Errors
    /// Transaction decoding or signature refusals, or an unsupported chain id.
    pub fn from_signed_transaction(
        signed_bytes: Vec<u8>,
        intent: [u8; 32],
        not_after: u64,
    ) -> Result<Self, AiClientError> {
        let (identity, key) = transaction_identity(&signed_bytes, not_after)?;
        let record = Self {
            generation: Cell::new([0; 32]),
            state: OperationState::Signed,
            domain: DomainStatus::Pending,
            protocol_version: identity.protocol_version,
            network_id: identity.network_id,
            activity_id: identity.activity_id,
            idempotency_key: identity.idempotency_key,
            intent,
            signer_public_key: key,
            not_after,
            attempt: 0,
            result_code: None,
            global_sequence: None,
            execution_height: None,
            receipt_root: None,
            receipt_digest: None,
            coverage: None,
            checkpoint: None,
            rung: None,
            signed_bytes,
        };
        record
            .generation
            .set(Sha256::digest(record.encode()?).into());
        Ok(record)
    }

    #[must_use]
    pub const fn rung(&self) -> Option<Rung> {
        self.rung
    }

    #[must_use]
    pub const fn state(&self) -> OperationState {
        self.state
    }

    #[must_use]
    pub const fn domain(&self) -> DomainStatus {
        self.domain
    }

    #[must_use]
    pub const fn protocol_version(&self) -> u16 {
        self.protocol_version
    }

    #[must_use]
    pub const fn network_id(&self) -> u32 {
        self.network_id
    }

    #[must_use]
    pub const fn activity_id(&self) -> [u8; 32] {
        self.activity_id
    }

    #[must_use]
    pub const fn idempotency_key(&self) -> [u8; 32] {
        self.idempotency_key
    }

    #[must_use]
    pub const fn intent(&self) -> [u8; 32] {
        self.intent
    }

    #[must_use]
    pub const fn signer_public_key(&self) -> [u8; 32] {
        self.signer_public_key
    }

    #[must_use]
    pub const fn not_after(&self) -> u64 {
        self.not_after
    }

    #[must_use]
    pub const fn attempt(&self) -> u32 {
        self.attempt
    }

    #[must_use]
    pub const fn result_code(&self) -> Option<i32> {
        self.result_code
    }

    #[must_use]
    pub const fn global_sequence(&self) -> Option<u64> {
        self.global_sequence
    }

    #[must_use]
    pub const fn execution_height(&self) -> Option<u64> {
        self.execution_height
    }

    #[must_use]
    pub const fn receipt_root(&self) -> Option<[u8; 32]> {
        self.receipt_root
    }

    #[must_use]
    pub const fn checkpoint(&self) -> Option<[u8; 32]> {
        self.checkpoint
    }

    #[must_use]
    pub fn signed_bytes(&self) -> &[u8] {
        &self.signed_bytes
    }

    /// Strict binary encoding of the durable record.
    ///
    /// # Errors
    /// `CorruptRecord` when the signed bytes exceed the length field.
    pub fn encode(&self) -> Result<Vec<u8>, AiClientError> {
        let length =
            u32::try_from(self.signed_bytes.len()).map_err(|_| AiClientError::CorruptRecord)?;
        let mut out = Vec::with_capacity(256 + self.signed_bytes.len());
        out.extend_from_slice(RECORD_MAGIC);
        out.push(self.state.code());
        out.push(self.domain.code());
        out.extend_from_slice(&self.protocol_version.to_be_bytes());
        out.extend_from_slice(&self.network_id.to_be_bytes());
        out.extend_from_slice(&self.activity_id);
        out.extend_from_slice(&self.idempotency_key);
        out.extend_from_slice(&self.intent);
        out.extend_from_slice(&self.signer_public_key);
        out.extend_from_slice(&self.not_after.to_be_bytes());
        out.extend_from_slice(&self.attempt.to_be_bytes());
        put_optional(&mut out, self.result_code.map(i32::to_be_bytes));
        put_optional(&mut out, self.global_sequence.map(u64::to_be_bytes));
        put_optional(&mut out, self.checkpoint);
        put_optional(&mut out, self.rung.map(|rung| [rung.wire_rank()]));
        put_optional(&mut out, self.execution_height.map(u64::to_be_bytes));
        put_optional(&mut out, self.receipt_root);
        put_optional(&mut out, self.receipt_digest);
        match self.coverage {
            None => out.push(0),
            Some(coverage) => {
                out.push(1);
                out.extend_from_slice(&coverage.activity_id);
                out.extend_from_slice(&coverage.sequence.to_be_bytes());
                out.extend_from_slice(&coverage.execution_height.to_be_bytes());
                out.extend_from_slice(&coverage.resulting_root);
                out.extend_from_slice(&coverage.receipt_digest);
                out.extend_from_slice(&coverage.block_height.to_be_bytes());
                out.extend_from_slice(&coverage.block_hash);
            }
        }
        out.extend_from_slice(&length.to_be_bytes());
        out.extend_from_slice(&self.signed_bytes);
        Ok(out)
    }

    /// Decodes a durable record and re-derives its identity from the signed bytes.
    ///
    /// # Errors
    /// `CorruptRecord` for malformed, trailing, inconsistent or pre-signing records, and
    /// wire or signature refusals of the retained bytes.
    pub fn decode(bytes: &[u8], _registry: &ModuleRegistry) -> Result<Self, AiClientError> {
        Self::decode_bytes(bytes)
    }

    fn decode_bytes(bytes: &[u8]) -> Result<Self, AiClientError> {
        let mut input = Cursor(bytes);
        let magic = input.take::<8>()?;
        let legacy = &magic == LEGACY_RECORD_MAGIC;
        if &magic != RECORD_MAGIC && !legacy {
            return Err(AiClientError::CorruptRecord);
        }
        let state = OperationState::from_code(input.take::<1>()?[0])?;
        let domain = DomainStatus::from_code(input.take::<1>()?[0])?;
        let mut record = Self {
            generation: Cell::new(Sha256::digest(bytes).into()),
            state,
            domain,
            protocol_version: u16::from_be_bytes(input.take()?),
            network_id: u32::from_be_bytes(input.take()?),
            activity_id: input.take()?,
            idempotency_key: input.take()?,
            intent: input.take()?,
            signer_public_key: input.take()?,
            not_after: u64::from_be_bytes(input.take()?),
            attempt: u32::from_be_bytes(input.take()?),
            result_code: input.optional()?.map(i32::from_be_bytes),
            global_sequence: input.optional()?.map(u64::from_be_bytes),
            checkpoint: input.optional()?,
            rung: match input.optional()? {
                None => None,
                Some([0]) => Some(Rung::Submitted),
                Some([1]) => Some(Rung::Receipted),
                Some([2]) => Some(Rung::Committed),
                _ => return Err(AiClientError::CorruptRecord),
            },
            execution_height: if legacy {
                None
            } else {
                input.optional()?.map(u64::from_be_bytes)
            },
            receipt_root: if legacy { None } else { input.optional()? },
            receipt_digest: if legacy { None } else { input.optional()? },
            coverage: if legacy {
                None
            } else {
                match input.take::<1>()? {
                    [0] => None,
                    [1] => Some(ReceiptCoverageFacts {
                        activity_id: input.take()?,
                        sequence: u64::from_be_bytes(input.take()?),
                        execution_height: u64::from_be_bytes(input.take()?),
                        resulting_root: input.take()?,
                        receipt_digest: input.take()?,
                        block_height: u64::from_be_bytes(input.take()?),
                        block_hash: input.take()?,
                    }),
                    _ => return Err(AiClientError::CorruptRecord),
                }
            },
            signed_bytes: input.rest()?.to_vec(),
        };
        // Old records have no authenticated height/root/coverage: retain the execution,
        // but revoke the old unsupported finality claim rather than manufacturing evidence.
        if legacy && record.state == OperationState::Finalized {
            record.state = OperationState::Executed;
            record.checkpoint = None;
            record.rung = Some(Rung::Receipted);
        }
        let (identity, key) = transaction_identity(&record.signed_bytes, record.not_after)?;
        if key != record.signer_public_key {
            return Err(AiClientError::CorruptRecord);
        }
        if identity.activity_id != record.activity_id
            || identity.idempotency_key != record.idempotency_key
            || identity.protocol_version != record.protocol_version
            || identity.network_id != record.network_id
            || identity.not_after != record.not_after
            || !record.consistent()
        {
            return Err(AiClientError::CorruptRecord);
        }
        Ok(record)
    }

    fn consistent(&self) -> bool {
        if self.execution_height.is_some() != self.receipt_root.is_some()
            || self.execution_height.is_some() != self.receipt_digest.is_some()
            || (self.execution_height.is_some() && self.global_sequence.is_none())
        {
            return false;
        }
        if let Some(coverage) = self.coverage {
            if !self.covers(&coverage) {
                return false;
            }
        }
        let pristine = self.execution_height.is_none()
            && self.coverage.is_none()
            && self.result_code.is_none()
            && self.global_sequence.is_none()
            && self.checkpoint.is_none();
        let executed = matches!(self.result_code, Some(0)) && self.global_sequence.is_some();
        if self.global_sequence.is_some()
            != matches!(self.rung, Some(Rung::Receipted | Rung::Committed))
        {
            return false;
        }
        match self.state {
            OperationState::Prepared | OperationState::Reviewed => false,
            OperationState::Signed => pristine && self.attempt == 0 && self.rung.is_none(),
            OperationState::Pending => {
                pristine && self.attempt > 0 && matches!(self.rung, Some(Rung::Submitted))
            }
            OperationState::Submitting | OperationState::Unknown => pristine && self.attempt > 0,
            OperationState::Executed => executed && self.checkpoint.is_none() && self.attempt > 0,
            OperationState::Finalized => {
                executed
                    && self.attempt > 0
                    && self
                        .coverage
                        .is_some_and(|coverage| self.checkpoint == Some(coverage.block_hash))
                    && self.rung == Some(Rung::Committed)
            }
            OperationState::Failed => {
                self.checkpoint.is_none()
                    && match self.result_code {
                        Some(code) => code != 0,
                        None => self.global_sequence.is_none(),
                    }
            }
        }
    }
    fn covers(&self, coverage: &ReceiptCoverageFacts) -> bool {
        coverage.activity_id == self.activity_id
            && Some(coverage.sequence) == self.global_sequence
            && Some(coverage.execution_height) == self.execution_height
            && Some(coverage.resulting_root) == self.receipt_root
            && Some(coverage.receipt_digest) == self.receipt_digest
            && coverage.block_height >= coverage.execution_height
    }
}

fn put_optional<const N: usize>(out: &mut Vec<u8>, value: Option<[u8; N]>) {
    match value {
        Some(bytes) => {
            out.push(1);
            out.extend_from_slice(&bytes);
        }
        None => out.push(0),
    }
}

struct Cursor<'a>(&'a [u8]);

impl Cursor<'_> {
    fn take<const N: usize>(&mut self) -> Result<[u8; N], AiClientError> {
        let (head, tail) = self
            .0
            .split_first_chunk::<N>()
            .ok_or(AiClientError::CorruptRecord)?;
        self.0 = tail;
        Ok(*head)
    }

    fn optional<const N: usize>(&mut self) -> Result<Option<[u8; N]>, AiClientError> {
        match self.take::<1>()? {
            [0] => Ok(None),
            [1] => Ok(Some(self.take()?)),
            _ => Err(AiClientError::CorruptRecord),
        }
    }

    fn rest(&mut self) -> Result<&[u8], AiClientError> {
        let length = usize::try_from(u32::from_be_bytes(self.take()?))
            .map_err(|_| AiClientError::CorruptRecord)?;
        if self.0.len() != length {
            return Err(AiClientError::CorruptRecord);
        }
        Ok(self.0)
    }
}

fn hex(bytes: &[u8; 32]) -> String {
    bytes
        .iter()
        .flat_map(|byte| [HEX[usize::from(byte >> 4)], HEX[usize::from(byte & 0x0f)]])
        .map(char::from)
        .collect()
}

/// Durable journal: every lifecycle change is synced before the next side effect.
#[derive(Clone, Debug, Eq, PartialEq)]
pub struct OperationJournal {
    directory: PathBuf,
}

impl OperationJournal {
    /// # Errors
    /// `Journal` when the directory cannot be created.
    pub fn open(directory: &Path) -> Result<Self, AiClientError> {
        fs::create_dir_all(directory)?;
        Ok(Self {
            directory: directory.to_path_buf(),
        })
    }

    #[must_use]
    pub fn record_path(&self, activity_id: &[u8; 32]) -> PathBuf {
        self.directory
            .join(format!("{}.{RECORD_EXTENSION}", hex(activity_id)))
    }

    fn acquire(&self, activity_id: &[u8; 32]) -> Result<File, AiClientError> {
        // The lock inode is permanent: unlinking it would allow a second owner.
        let file = OpenOptions::new()
            .read(true)
            .write(true)
            .create(true)
            .truncate(false)
            .open(self.record_path(activity_id).with_extension("lock"))?;
        file.try_lock().map_err(|error| match error {
            std::fs::TryLockError::WouldBlock => AiClientError::Journal(io::ErrorKind::WouldBlock),
            std::fs::TryLockError::Error(error) => error.into(),
        })?;
        Ok(file)
    }

    fn current(&self, record: &OperationRecord) -> Result<Option<OperationRecord>, AiClientError> {
        let bytes = match fs::read(self.record_path(&record.activity_id)) {
            Ok(bytes) => bytes,
            Err(error) if error.kind() == io::ErrorKind::NotFound => return Ok(None),
            Err(error) => return Err(error.into()),
        };
        if <[u8; 32]>::from(Sha256::digest(&bytes)) != record.generation.get() {
            return Err(AiClientError::StaleRecord);
        }
        let previous = OperationRecord::decode_bytes(&bytes)?;
        if !same_operation(&previous, record) {
            return Err(AiClientError::CorruptRecord);
        }
        Ok(Some(previous))
    }

    /// Generation-checked atomic replacement under exclusive process-safe ownership.
    /// A record cloned before another transition cannot overwrite the newer state.
    pub fn persist(&self, record: &OperationRecord) -> Result<(), AiClientError> {
        let _owner = self.acquire(&record.activity_id)?;
        self.persist_owned(record)
    }

    fn persist_owned(&self, record: &OperationRecord) -> Result<(), AiClientError> {
        if !record.consistent() {
            return Err(AiClientError::CorruptRecord);
        }
        if let Some(previous) = self.current(record)? {
            if !monotonic(&previous, record) {
                return Err(AiClientError::InvalidTransition {
                    from: previous.state,
                });
            }
        }
        let bytes = record.encode()?;
        let path = self.record_path(&record.activity_id);
        static NEXT_PARTIAL: AtomicU64 = AtomicU64::new(0);
        let (partial, mut file) = loop {
            let partial = path.with_extension(format!(
                "{}.{}.partial",
                std::process::id(),
                NEXT_PARTIAL.fetch_add(1, Ordering::Relaxed)
            ));
            match OpenOptions::new()
                .write(true)
                .create_new(true)
                .open(&partial)
            {
                Ok(file) => break (partial, file),
                Err(error) if error.kind() == io::ErrorKind::AlreadyExists => continue,
                Err(error) => return Err(error.into()),
            }
        };
        let result = (|| -> io::Result<()> {
            file.write_all(&bytes)?;
            file.sync_all()?;
            drop(file);
            fs::rename(&partial, &path)?;
            File::open(&self.directory)?.sync_all()
        })();
        if result.is_err() {
            let _ = fs::remove_file(&partial);
        }
        result?;
        record.generation.set(Sha256::digest(&bytes).into());
        Ok(())
    }

    /// Loads one record; an interrupted `Submitting` record becomes `Unknown` durably.
    ///
    /// # Errors
    /// `Journal`, `CorruptRecord` and the refusals of [`OperationRecord::decode`].
    pub fn load(
        &self,
        activity_id: &[u8; 32],
        registry: &ModuleRegistry,
    ) -> Result<OperationRecord, AiClientError> {
        let _owner = self.acquire(activity_id)?;
        let bytes = fs::read(self.record_path(activity_id))?;
        let migrated = bytes.starts_with(LEGACY_RECORD_MAGIC);
        let mut record = OperationRecord::decode(&bytes, registry)?;
        if record.activity_id != *activity_id {
            return Err(AiClientError::CorruptRecord);
        }
        if record.state == OperationState::Submitting {
            record.state = OperationState::Unknown;
            self.persist_owned(&record)?;
        } else if migrated {
            self.persist_owned(&record)?;
        }
        Ok(record)
    }

    /// Persists a freshly signed record before any transmission.
    ///
    /// # Errors
    /// `InvalidTransition` unless the record is `Signed`; journal failures.
    pub fn record_signed(&self, record: &OperationRecord) -> Result<(), AiClientError> {
        require(record, &[OperationState::Signed])?;
        let _owner = self.acquire(&record.activity_id)?;
        match fs::read(self.record_path(&record.activity_id)) {
            Ok(bytes) => {
                let current = OperationRecord::decode_bytes(&bytes)?;
                if !same_operation(&current, record) {
                    return Err(AiClientError::CorruptRecord);
                }
                // Registration is idempotent, even after execution. Keep the caller's
                // old generation so a later mutation must reload the current record.
                if current.state == OperationState::Signed {
                    record.generation.set(current.generation.get());
                }
                Ok(())
            }
            Err(error) if error.kind() == io::ErrorKind::NotFound => self.persist_owned(record),
            Err(error) => Err(error.into()),
        }
    }

    /// First transmission of the exact signed bytes. `Submitting` is durable before sending;
    /// acknowledgment means pending, and lost or indeterminate delivery means unknown.
    ///
    /// # Errors
    /// `InvalidTransition` unless `Signed`; transaction or journal refusals.
    pub fn submit(
        &self,
        record: &mut OperationRecord,
        endpoint: &SequencerEndpoint,
    ) -> Result<OperationState, AiClientError> {
        require(record, &[OperationState::Signed])?;
        self.deliver(record, endpoint)
    }

    /// Separately authorised resend of the same signed bytes from `Unknown`; never re-signs.
    ///
    /// # Errors
    /// `InvalidTransition` unless `Unknown`; transaction or journal refusals.
    pub fn resend_exact(
        &self,
        record: &mut OperationRecord,
        endpoint: &SequencerEndpoint,
    ) -> Result<OperationState, AiClientError> {
        require(record, &[OperationState::Unknown])?;
        self.deliver(record, endpoint)
    }

    fn deliver(
        &self,
        record: &mut OperationRecord,
        endpoint: &SequencerEndpoint,
    ) -> Result<OperationState, AiClientError> {
        let _owner = self.acquire(&record.activity_id)?;
        self.current(record)?.ok_or(AiClientError::StaleRecord)?;
        let (identity, key) = transaction_identity(&record.signed_bytes, record.not_after)?;
        if identity.activity_id != record.activity_id
            || identity.idempotency_key != record.idempotency_key
            || identity.protocol_version != record.protocol_version
            || identity.network_id != record.network_id
            || key != record.signer_public_key
        {
            return Err(AiClientError::CorruptRecord);
        }
        record.attempt = record
            .attempt
            .checked_add(1)
            .ok_or(AiClientError::CorruptRecord)?;
        record.state = OperationState::Submitting;
        self.persist_owned(record)?;
        record.state = match submit_signed(endpoint, &record.signed_bytes) {
            Ok(Submission::Submitted { tx_hash, rung }) if tx_hash == record.activity_id => {
                record.rung = Some(rung);
                OperationState::Pending
            }
            Ok(Submission::Submitted { .. }) => {
                record.state = OperationState::Unknown;
                self.persist_owned(record)?;
                return Err(AiClientError::ReceiptMismatch);
            }
            Err(SubmitError::Refused { result, .. })
                if record.attempt == 1
                    && result.retriability() == Retriability::Terminal
                    && result.known() != Some(KnownResult::IdempotentReplay) =>
            {
                record.result_code = Some(result.raw());
                OperationState::Failed
            }
            Ok(Submission::Unknown(_)) | Err(SubmitError::Refused { .. }) => {
                OperationState::Unknown
            }
            Err(error @ SubmitError::Sequencer(SequencerError::Endpoint | SequencerError::Tls)) => {
                record.state = OperationState::Unknown;
                self.persist_owned(record)?;
                return Err(error.into());
            }
            Err(error @ SubmitError::Sequencer(_)) => {
                record.state = OperationState::Unknown;
                self.persist_owned(record)?;
                return Err(error.into());
            }
        };
        self.persist_owned(record)?;
        Ok(record.state)
    }

    /// Resolves `Pending` or `Unknown` from an authenticated sequencer receipt.
    /// Absence or a submitted-only answer leaves the state unchanged.
    ///
    /// # Errors
    /// `InvalidTransition`, receipt lookup refusals, or `ReceiptMismatch` for another call.
    pub fn resolve(
        &self,
        record: &mut OperationRecord,
        endpoint: &SequencerEndpoint,
        context: AuthenticatedLookupContext<'_>,
    ) -> Result<OperationState, AiClientError> {
        let _owner = self.acquire(&record.activity_id)?;
        self.current(record)?.ok_or(AiClientError::StaleRecord)?;
        require(record, &[OperationState::Pending, OperationState::Unknown])?;
        let receipt = match lookup_authenticated(endpoint, record.activity_id, context)? {
            AuthenticatedLookup::Absent | AuthenticatedLookup::Submitted { .. } => {
                return Ok(record.state);
            }
            AuthenticatedLookup::Verified(receipt) => receipt,
        };
        self.apply_receipt(record, receipt)
    }

    /// Resolve with real node batch inclusion evidence; retained coverage survives reopen.
    pub fn resolve_with_commit(
        &self,
        record: &mut OperationRecord,
        endpoint: &SequencerEndpoint,
        context: AuthenticatedLookupContext<'_>,
        evidence: &ReceiptCommitEvidence<'_>,
    ) -> Result<OperationState, AiClientError> {
        let _owner = self.acquire(&record.activity_id)?;
        self.current(record)?.ok_or(AiClientError::StaleRecord)?;
        require(record, &[OperationState::Pending, OperationState::Unknown])?;
        match lookup_authenticated_with_commit(endpoint, record.activity_id, context, evidence)? {
            AuthenticatedLookup::Verified(receipt) => self.apply_receipt(record, receipt),
            _ => Ok(record.state),
        }
    }

    fn apply_receipt(
        &self,
        record: &mut OperationRecord,
        receipt: AuthenticatedReceipt,
    ) -> Result<OperationState, AiClientError> {
        let (call, signature) = evm_transaction::decode_signed(&record.signed_bytes)?;
        let sender = evm_transaction::recover_sender(&call, &signature)?;
        if receipt.activity_id() != record.activity_id
            || receipt.execution().to != call.to
            || receipt.execution().from != sender
            || receipt.execution().nonce != call.nonce
        {
            return Err(AiClientError::ReceiptMismatch);
        }
        let code = receipt.result_code().raw();
        record.result_code = Some(code);
        record.global_sequence = Some(receipt.global_sequence());
        record.execution_height = Some(receipt.height());
        record.receipt_root = Some(
            receipt
                .receipt()
                .protocol()
                .ok_or(AiClientError::ReceiptMismatch)?
                .resulting_state_root(),
        );
        record.receipt_digest = Some(Sha256::digest(receipt.canonical_bytes()).into());
        record.coverage = if receipt.rung() == Rung::Committed {
            CommitFinality::from_verified_receipt(&receipt)?.coverage
        } else {
            None
        };
        record.rung = Some(receipt.rung());
        record.state = if code == 0 {
            OperationState::Executed
        } else {
            OperationState::Failed
        };
        self.persist_owned(record)?;
        Ok(record.state)
    }

    /// Promotes a successful execution with a block commit covering its receipt height.
    ///
    /// # Errors
    /// `InvalidTransition` unless `Executed`; `FinalityMismatch` when the receipt height
    /// exceeds the committed height or is unavailable.
    pub fn finalize(
        &self,
        record: &mut OperationRecord,
        finality: &CommitFinality,
    ) -> Result<OperationState, AiClientError> {
        let _owner = self.acquire(&record.activity_id)?;
        self.current(record)?.ok_or(AiClientError::StaleRecord)?;
        require(record, &[OperationState::Executed])?;
        let coverage = finality.coverage.ok_or(AiClientError::FinalityMismatch)?;
        if !record.covers(&coverage)
            || coverage.block_height != finality.block_height
            || coverage.block_hash != finality.block_hash.bytes()
        {
            return Err(AiClientError::FinalityMismatch);
        }
        record.coverage = Some(coverage);
        record.checkpoint = Some(finality.block_hash.bytes());
        record.rung = Some(Rung::Committed);
        record.state = OperationState::Finalized;
        self.persist_owned(record)?;
        Ok(record.state)
    }

    /// A signed but never-sent operation fails once its validity has passed; submitted
    /// operations never fail through elapsed time.
    ///
    /// # Errors
    /// `InvalidTransition` unless `Signed`; journal failures.
    pub fn expire(
        &self,
        record: &mut OperationRecord,
        now: u64,
    ) -> Result<OperationState, AiClientError> {
        let _owner = self.acquire(&record.activity_id)?;
        self.current(record)?.ok_or(AiClientError::StaleRecord)?;
        require(record, &[OperationState::Signed])?;
        if now > record.not_after {
            record.state = OperationState::Failed;
            self.persist_owned(record)?;
        }
        Ok(record.state)
    }
}

fn same_operation(left: &OperationRecord, right: &OperationRecord) -> bool {
    left.signed_bytes == right.signed_bytes
        && left.intent == right.intent
        && left.not_after == right.not_after
        && left.activity_id == right.activity_id
}

fn monotonic(previous: &OperationRecord, next: &OperationRecord) -> bool {
    use OperationState::{Executed, Failed, Finalized, Pending, Signed, Submitting, Unknown};
    if previous.state == next.state {
        return previous.encode().ok() == next.encode().ok();
    }
    let allowed = matches!(
        (previous.state, next.state),
        (Signed, Submitting | Failed)
            | (Submitting, Pending | Unknown | Failed)
            | (Pending | Unknown, Executed | Failed)
            | (Unknown, Submitting)
            | (Executed, Finalized)
    );
    let attempt = if next.state == Submitting {
        previous.attempt.checked_add(1) == Some(next.attempt)
    } else {
        previous.attempt == next.attempt
    };
    allowed && attempt
}

fn require(record: &OperationRecord, allowed: &[OperationState]) -> Result<(), AiClientError> {
    if allowed.contains(&record.state) {
        Ok(())
    } else {
        Err(AiClientError::InvalidTransition { from: record.state })
    }
}

/// Retention status of one requested epoch.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
#[repr(u8)]
pub enum EpochStatus {
    Retained = 1,
    NeverOpened = 2,
    RetainedTerminal = 3,
    ArchiveRequired = 4,
    ArchiveUnavailable = 5,
    UnsupportedVersion = 6,
}

impl EpochStatus {
    #[must_use]
    pub const fn code(self) -> u8 {
        self as u8
    }
}

/// The immutable earlier binding a historical epoch refers to explicitly.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct HistoricalSource {
    pub snapshot: Digest32,
    pub config: Version,
    pub policy: PolicyDigest,
    pub roster: Presence<RosterDigest>,
}

/// One epoch header: a status plus its source, absent exactly when nothing was retained.
#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct EpochHistoryEntry {
    epoch: u64,
    status: EpochStatus,
    source: Presence<HistoricalSource>,
}

impl EpochHistoryEntry {
    /// # Errors
    /// `NON_CANONICAL` when presence of the source contradicts the status.
    pub fn new(
        epoch: u64,
        status: EpochStatus,
        source: Presence<HistoricalSource>,
    ) -> Result<Self, AiClientError> {
        let sourceless = matches!(
            status,
            EpochStatus::NeverOpened | EpochStatus::UnsupportedVersion
        );
        if sourceless != matches!(source, Presence::Absent) {
            return Err(NON_CANONICAL.into());
        }
        Ok(Self {
            epoch,
            status,
            source,
        })
    }

    #[must_use]
    pub const fn epoch(&self) -> u64 {
        self.epoch
    }

    #[must_use]
    pub const fn status(&self) -> EpochStatus {
        self.status
    }

    #[must_use]
    pub const fn source(&self) -> Presence<HistoricalSource> {
        self.source
    }

    /// The epoch's own policy from retained or archived content, verified against the
    /// historical digest; the current policy is never substituted.
    ///
    /// # Errors
    /// `HistoryUnavailable` for a status without retrievable content and `IntegrityFailure`
    /// for content that is not exactly the recorded policy.
    pub fn policy(&self, content: &[u8]) -> Result<TaskPolicyV1, AiClientError> {
        let Presence::Present(source) = self.source else {
            return Err(AiClientError::HistoryUnavailable(self.status));
        };
        if self.status == EpochStatus::ArchiveUnavailable {
            return Err(AiClientError::HistoryUnavailable(self.status));
        }
        if content.len() != TASK_POLICY_BYTES {
            return Err(QueryError::IntegrityFailure.into());
        }
        let policy = TaskPolicyV1::decode(content).map_err(|_| QueryError::IntegrityFailure)?;
        let mut encoded = vec![0; TASK_POLICY_BYTES];
        let length = policy
            .encode(&mut encoded)
            .map_err(|_| QueryError::IntegrityFailure)?;
        if encoded.get(..length) != Some(content)
            || policy.digest().map_err(|_| QueryError::IntegrityFailure)? != source.policy
        {
            return Err(QueryError::IntegrityFailure.into());
        }
        Ok(policy)
    }
}
