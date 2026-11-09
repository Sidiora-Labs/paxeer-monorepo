#![allow(non_upper_case_globals)]
//! Fixed selector routing and the native call router over the landed feature transitions.
use crate::codec::{self, ApplicationResult, ResultStatus, ValidatedEnvelope};
use crate::errors::{
    CodecResult, NON_CANONICAL, NOT_FOUND, UNKNOWN_OPERATION, WRONG_DOMAIN, WRONG_PROGRAM,
};
use crate::evaluators::{admission as challenge, authority};
use crate::registry::F01_SECTION_CAP;
use crate::registry_ops::{self, CallContext, PolicySection};
use crate::state::{self, RetainedResult, Section, SharedState};
use crate::types::GuestIndex as _;
use crate::types::{Presence, RequestDigest};
use crate::{
    admission, aggregation, canary, closure, commit_reveal, epoch, evidence, laps, queries,
    rewards, rooms, tasks, value_adapter, workers, MAX_STATE_BYTES,
};
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum SequencePolicy {
    Role,
    ObjectLocal,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum CallBoundary {
    Mutation,
    ProgramRead,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum AuthorityClass {
    FeatureDefined,
    Host,
    AnyAccount,
    EnrolledPeer,
    HostOrAfterChallengeWindow,
    ResolvedFault,
    BondOwner,
    PublicRead,
}
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct Operation(u16);
#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub struct OperationMetadata {
    pub operation: Operation,
    pub name: &'static str,
    pub feature: u8,
    pub boundary: CallBoundary,
    pub sequence: SequencePolicy,
    pub delegate_allowed: bool,
    pub payload_min: usize,
    pub payload_max: usize,
    pub authority: AuthorityClass,
    pub sections: &'static [Section],
}
impl Operation {
    pub const OpenRoom: Self = OpenRoom;
    pub const PostAsk: Self = PostAsk;
    pub const PostBid: Self = PostBid;
    pub const RecordDeal: Self = RecordDeal;
    pub const SettleLaps: Self = SettleLaps;
    pub const ChallengeLap: Self = ChallengeLap;
    pub const ResolveChallenge: Self = ResolveChallenge;
    pub const SplitPayout: Self = SplitPayout;
    pub const LockBond: Self = LockBond;
    pub const SlashBond: Self = SlashBond;
    pub const RefundBond: Self = RefundBond;
    pub const CloseRoom: Self = CloseRoom;
    pub const RoomView: Self = RoomView;
    pub const DealView: Self = DealView;
    pub const PeerView: Self = PeerView;
    pub const ReceiptBatchView: Self = ReceiptBatchView;
    pub const BookView: Self = BookView;
    /// Decodes a selector from the frozen operation table.
    ///
    /// # Errors
    /// Returns `UNKNOWN_OPERATION` when the selector is not in the table.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn decode(selector: u16) -> CodecResult<Self> {
        metadata_for_selector(selector)
            .map(|m| m.operation)
            .ok_or(UNKNOWN_OPERATION)
    }
    #[must_use]
    pub const fn selector(self) -> u16 {
        self.0
    }
    #[must_use]
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn metadata(self) -> &'static OperationMetadata {
        if let Some(metadata) = metadata_for_selector(self.0) {
            return metadata;
        }
        #[cfg(target_arch = "wasm32")]
        core::arch::wasm32::unreachable();
        #[cfg(not(target_arch = "wasm32"))]
        unreachable!("Operation is constructed only by the frozen table")
    }

    /// Checks a payload length against the operation's bounds.
    ///
    /// # Errors
    /// Returns `NON_CANONICAL` when the length is outside the operation's bounds, or is neither 40 nor 48 for `0x0106`/`0x0107`.
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    pub fn validate_payload_length(self, length: usize) -> CodecResult<()> {
        if self == CommitScore {
            return if length == 224 || length == canary::CANARY_COMMIT_BYTES {
                Ok(())
            } else {
                Err(NON_CANONICAL)
            };
        }
        if self == RevealScore {
            if length == canary::CANARY_REVEAL_BYTES {
                return Ok(());
            }
            if length < 364 {
                return Err(NON_CANONICAL);
            }
        }
        let m = self.metadata();
        if matches!(self.0, 0x0106 | 0x0107) && length != 40 && length != 48 {
            return Err(NON_CANONICAL);
        }
        if self == Self::RecordDeal && (length < 145 || (length - 145) % 36 != 0) {
            return Err(NON_CANONICAL);
        }
        if length < m.payload_min || length > m.payload_max {
            Err(NON_CANONICAL)
        } else {
            Ok(())
        }
    }
}
pub const CREATE: Operation = Operation(0x0101);
pub const STAGE_POLICY: Operation = Operation(0x0102);
pub const CANCEL_POLICY: Operation = Operation(0x0103);
pub const SCHEDULE_ACTIVATION: Operation = Operation(0x0104);
pub const ADVANCE_ACTIVATION: Operation = Operation(0x0105);
pub const SUSPEND: Operation = Operation(0x0106);
pub const UPDATE_METADATA: Operation = Operation(0x0107);
pub const APPOINT_OPERATOR: Operation = Operation(0x0108);
pub const REVOKE_OPERATOR: Operation = Operation(0x0109);
pub const REQUEST_CLOSE: Operation = Operation(0x010A);
pub const ADVANCE_CLOSE: Operation = Operation(0x010B);
pub const ADMIT_TASK: Operation = Operation(0x010C);
pub const ACCEPT_TASK: Operation = Operation(0x010D);
pub const CANCEL_TASK: Operation = Operation(0x010E);
pub const COMMIT_TASK_RESULT: Operation = Operation(0x010F);
pub const SEAL_TASK_SET: Operation = Operation(0x0110);
pub const OPEN_EPOCH: Operation = Operation(0x0111);
pub const EnrollWorker: Operation = Operation(0x0201);
pub const PublishMetadata: Operation = Operation(0x0202);
pub const SetDraining: Operation = Operation(0x0203);
pub const UndoDrain: Operation = Operation(0x0204);
pub const RotateDelegate: Operation = Operation(0x0205);
pub const RevokeDelegate: Operation = Operation(0x0206);
pub const RetireWorker: Operation = Operation(0x0207);
pub const AcceptEnrollment: Operation = Operation(0x0209);
pub const ExpireEnrollment: Operation = Operation(0x020A);
pub const ScheduleEvaluator: Operation = Operation(0x0301);
pub const RotateEvaluatorKey: Operation = Operation(0x0302);
pub const RevokeEvaluator: Operation = Operation(0x0303);
pub const ChallengeAssessment: Operation = Operation(0x0304);
pub const CommitScore: Operation = Operation(0x0401);
pub const RevealScore: Operation = Operation(0x0402);
pub const BeginAggregation: Operation = Operation(0x0501);
pub const ProcessAggregation: Operation = Operation(0x0502);
pub const FinalizeAggregation: Operation = Operation(0x0503);
pub const FUND: Operation = Operation(0x0601);
pub const CLAIM: Operation = Operation(0x0602);
pub const EXPIRE_EPOCH_CLAIMS: Operation = Operation(0x0603);
pub const REFUND_FREE: Operation = Operation(0x0604);
pub const PRUNE_EPOCH: Operation = Operation(0x0605);
pub const ResetHistory: Operation = Operation(0x0701);
pub const SuspendHistory: Operation = Operation(0x0702);
pub const ResumeHistory: Operation = Operation(0x0703);
pub const ApproveAdmission: Operation = Operation(0x0801);
pub const RevokeAdmissionApproval: Operation = Operation(0x0802);
pub const AdmitWorker: Operation = Operation(0x0803);
/// Successful consent binds the evaluator replay slot and records this request. Subsequent
/// evidence, commit and reveal calls must use the next sequence from committed replay.
pub const AdmitEvaluator: Operation = Operation(0x0804);
pub const Heartbeat: Operation = Operation(0x0805);
pub const RequestExit: Operation = Operation(0x0806);
pub const CancelExit: Operation = Operation(0x0807);
pub const PruneInactive: Operation = Operation(0x0808);
pub const AdministrativeRemove: Operation = Operation(0x0809);
pub const SealEvidence: Operation = Operation(0x0901);
pub const READ_HEADER: Operation = Operation(0x0A01);
pub const READ_STATE_CHUNK: Operation = Operation(0x0A02);
pub const OpenRoom: Operation = Operation(0x0B01);
pub const PostAsk: Operation = Operation(0x0B02);
pub const PostBid: Operation = Operation(0x0B03);
pub const RecordDeal: Operation = Operation(0x0B04);
pub const SettleLaps: Operation = Operation(0x0B05);
pub const ChallengeLap: Operation = Operation(0x0B06);
pub const ResolveChallenge: Operation = Operation(0x0B07);
pub const SplitPayout: Operation = Operation(0x0B08);
pub const LockBond: Operation = Operation(0x0B09);
pub const SlashBond: Operation = Operation(0x0B0A);
pub const RefundBond: Operation = Operation(0x0B0B);
pub const CloseRoom: Operation = Operation(0x0B0C);
pub const RoomView: Operation = Operation(0x0B10);
pub const DealView: Operation = Operation(0x0B11);
pub const PeerView: Operation = Operation(0x0B12);
pub const ReceiptBatchView: Operation = Operation(0x0B13);
pub const BookView: Operation = Operation(0x0B14);
#[allow(non_upper_case_globals)]
pub const OPERATIONS: &[OperationMetadata] = &[
    OperationMetadata {
        operation: CREATE,
        name: "CREATE",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: STAGE_POLICY,
        name: "STAGE_POLICY",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: CANCEL_POLICY,
        name: "CANCEL_POLICY",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 16,
        payload_max: 16,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: SCHEDULE_ACTIVATION,
        name: "SCHEDULE_ACTIVATION",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 16,
        payload_max: 16,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ADVANCE_ACTIVATION,
        name: "ADVANCE_ACTIVATION",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 16,
        payload_max: 16,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: SUSPEND,
        name: "SUSPEND",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 40,
        payload_max: 48,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: UPDATE_METADATA,
        name: "UPDATE_METADATA",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 40,
        payload_max: 48,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: APPOINT_OPERATOR,
        name: "APPOINT_OPERATOR",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 49,
        payload_max: 49,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: REVOKE_OPERATOR,
        name: "REVOKE_OPERATOR",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 16,
        payload_max: 16,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: REQUEST_CLOSE,
        name: "REQUEST_CLOSE",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 40,
        payload_max: 40,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ADVANCE_CLOSE,
        name: "ADVANCE_CLOSE",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 11,
        payload_max: 11,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ADMIT_TASK,
        name: "ADMIT_TASK",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 248,
        payload_max: 248,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ACCEPT_TASK,
        name: "ACCEPT_TASK",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: true,
        payload_min: 72,
        payload_max: 72,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: CANCEL_TASK,
        name: "CANCEL_TASK",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 32,
        payload_max: 32,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: COMMIT_TASK_RESULT,
        name: "COMMIT_TASK_RESULT",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: true,
        payload_min: 72,
        payload_max: 72,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: SEAL_TASK_SET,
        name: "SEAL_TASK_SET",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 48,
        payload_max: 48,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: OPEN_EPOCH,
        name: "OPEN_EPOCH",
        feature: 1,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: EnrollWorker,
        name: "EnrollWorker",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 136,
        payload_max: 136,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: PublishMetadata,
        name: "PublishMetadata",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: true,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: SetDraining,
        name: "SetDraining",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: UndoDrain,
        name: "UndoDrain",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: RotateDelegate,
        name: "RotateDelegate",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: RevokeDelegate,
        name: "RevokeDelegate",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: RetireWorker,
        name: "RetireWorker",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: AcceptEnrollment,
        name: "AcceptEnrollment",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 144,
        payload_max: 144,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ExpireEnrollment,
        name: "ExpireEnrollment",
        feature: 2,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 72,
        payload_max: 72,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ScheduleEvaluator,
        name: "ScheduleEvaluator",
        feature: 3,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 160,
        payload_max: 160,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: RotateEvaluatorKey,
        name: "RotateEvaluatorKey",
        feature: 3,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 88,
        payload_max: 88,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: RevokeEvaluator,
        name: "RevokeEvaluator",
        feature: 3,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 74,
        payload_max: 74,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ChallengeAssessment,
        name: "ChallengeAssessment",
        feature: 3,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 97,
        payload_max: 97,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: CommitScore,
        name: "CommitScore",
        feature: 4,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: true,
        payload_min: 224,
        payload_max: canary::CANARY_COMMIT_BYTES,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: RevealScore,
        name: "RevealScore",
        feature: 4,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: true,
        payload_min: canary::CANARY_REVEAL_BYTES,
        payload_max: 1480,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: BeginAggregation,
        name: "BeginAggregation",
        feature: 5,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 0,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ProcessAggregation,
        name: "ProcessAggregation",
        feature: 5,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 34,
        payload_max: 34,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: FinalizeAggregation,
        name: "FinalizeAggregation",
        feature: 5,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 32,
        payload_max: 32,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: FUND,
        name: "FUND",
        feature: 6,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 57,
        payload_max: 57,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: CLAIM,
        name: "CLAIM",
        feature: 6,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 80,
        payload_max: 80,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: EXPIRE_EPOCH_CLAIMS,
        name: "EXPIRE_EPOCH_CLAIMS",
        feature: 6,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 0,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: REFUND_FREE,
        name: "REFUND_FREE",
        feature: 6,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 64,
        payload_max: 64,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: PRUNE_EPOCH,
        name: "PRUNE_EPOCH",
        feature: 6,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 0,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ResetHistory,
        name: "ResetHistory",
        feature: 7,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 73,
        payload_max: 73,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: SuspendHistory,
        name: "SuspendHistory",
        feature: 7,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 73,
        payload_max: 73,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ResumeHistory,
        name: "ResumeHistory",
        feature: 7,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 73,
        payload_max: 73,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: ApproveAdmission,
        name: "ApproveAdmission",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: RevokeAdmissionApproval,
        name: "RevokeAdmissionApproval",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: AdmitWorker,
        name: "AdmitWorker",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: AdmitEvaluator,
        name: "AdmitEvaluator",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: Heartbeat,
        name: "Heartbeat",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: true,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: RequestExit,
        name: "RequestExit",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: CancelExit,
        name: "CancelExit",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: PruneInactive,
        name: "PruneInactive",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: AdministrativeRemove,
        name: "AdministrativeRemove",
        feature: 8,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 15965,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: SealEvidence,
        name: "SealEvidence",
        feature: 9,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: true,
        payload_min: 131,
        payload_max: 131,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: READ_HEADER,
        name: "READ_HEADER",
        feature: 10,
        boundary: CallBoundary::ProgramRead,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 0,
        payload_max: 0,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: READ_STATE_CHUNK,
        name: "READ_STATE_CHUNK",
        feature: 10,
        boundary: CallBoundary::ProgramRead,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 46,
        payload_max: 46,
        authority: AuthorityClass::FeatureDefined,
        sections: &[],
    },
    OperationMetadata {
        operation: OpenRoom,
        name: "OpenRoom",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: crate::rooms::OPEN_ROOM_REQUEST_BYTES,
        payload_max: crate::rooms::OPEN_ROOM_REQUEST_BYTES,
        authority: AuthorityClass::Host,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: PostAsk,
        name: "PostAsk",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 128,
        payload_max: 128,
        authority: AuthorityClass::AnyAccount,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: PostBid,
        name: "PostBid",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 130,
        payload_max: 130,
        authority: AuthorityClass::EnrolledPeer,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::SettlementClaims,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: RecordDeal,
        name: "RecordDeal",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 181,
        payload_max: 1297,
        authority: AuthorityClass::Host,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: SettleLaps,
        name: "SettleLaps",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: laps::SETTLE_FIXED_BYTES + evidence::MANIFEST_FIXED_BYTES,
        payload_max: laps::SETTLE_FIXED_BYTES + evidence::MAX_MANIFEST_BYTES,
        authority: AuthorityClass::Host,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::SettlementClaims,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: ChallengeLap,
        name: "ChallengeLap",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 88,
        payload_max: 88,
        authority: AuthorityClass::AnyAccount,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::CurrentReports,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: ResolveChallenge,
        name: "ResolveChallenge",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 104,
        payload_max: 104,
        authority: AuthorityClass::AnyAccount,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::CurrentReports,
            Section::SettlementClaims,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: SplitPayout,
        name: "SplitPayout",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 80,
        payload_max: 80,
        authority: AuthorityClass::HostOrAfterChallengeWindow,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::CurrentReports,
            Section::SettlementClaims,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: LockBond,
        name: "LockBond",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 80,
        payload_max: 80,
        authority: AuthorityClass::EnrolledPeer,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::SettlementClaims,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: SlashBond,
        name: "SlashBond",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 104,
        payload_max: 104,
        authority: AuthorityClass::ResolvedFault,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::CurrentReports,
            Section::SettlementClaims,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: RefundBond,
        name: "RefundBond",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 64,
        payload_max: 64,
        authority: AuthorityClass::BondOwner,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::CurrentReports,
            Section::SettlementClaims,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: CloseRoom,
        name: "CloseRoom",
        feature: 11,
        boundary: CallBoundary::Mutation,
        sequence: SequencePolicy::Role,
        delegate_allowed: false,
        payload_min: 32,
        payload_max: 32,
        authority: AuthorityClass::Host,
        sections: &[
            Section::PolicyLifecycle,
            Section::IdentityRoster,
            Section::CurrentReports,
            Section::SettlementClaims,
            Section::Control,
        ],
    },
    OperationMetadata {
        operation: RoomView,
        name: "RoomView",
        feature: 11,
        boundary: CallBoundary::ProgramRead,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 32,
        payload_max: 32,
        authority: AuthorityClass::PublicRead,
        sections: &[Section::PolicyLifecycle, Section::IdentityRoster],
    },
    OperationMetadata {
        operation: DealView,
        name: "DealView",
        feature: 11,
        boundary: CallBoundary::ProgramRead,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 32,
        payload_max: 32,
        authority: AuthorityClass::PublicRead,
        sections: &[Section::IdentityRoster, Section::SettlementClaims],
    },
    OperationMetadata {
        operation: PeerView,
        name: "PeerView",
        feature: 11,
        boundary: CallBoundary::ProgramRead,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 32,
        payload_max: 32,
        authority: AuthorityClass::PublicRead,
        sections: &[Section::IdentityRoster, Section::SettlementClaims],
    },
    OperationMetadata {
        operation: ReceiptBatchView,
        name: "ReceiptBatchView",
        feature: 11,
        boundary: CallBoundary::ProgramRead,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 64,
        payload_max: 64,
        authority: AuthorityClass::PublicRead,
        sections: &[Section::IdentityRoster, Section::SettlementClaims],
    },
    OperationMetadata {
        operation: BookView,
        name: "BookView",
        feature: 11,
        boundary: CallBoundary::ProgramRead,
        sequence: SequencePolicy::ObjectLocal,
        delegate_allowed: false,
        payload_min: 32,
        payload_max: 32,
        authority: AuthorityClass::PublicRead,
        sections: &[Section::PolicyLifecycle, Section::SettlementClaims],
    },
];

static OPERATION_INDICES: [[u8; 32]; 12] = {
    let mut indices = [[u8::MAX; 32]; 12];
    let mut i = 0;
    while i < OPERATIONS.len() {
        let selector = OPERATIONS[i].operation.0 as usize;
        indices[selector >> 8][selector & 0xff] = i as u8;
        i += 1;
    }
    indices
};

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn metadata_for_selector(selector: u16) -> Option<&'static OperationMetadata> {
    let family = OPERATION_INDICES.get(usize::from(selector >> 8))?;
    let index = *family.get(usize::from(selector & 0xff))?;
    OPERATIONS.get(usize::from(index))
}

const CONTROL_CAP: usize = Section::Control.payload_cap();
const REBIND_SCRATCH_BYTES: usize = F01_SECTION_CAP + CONTROL_CAP + MAX_STATE_BYTES;
const RECORD_PAYLOAD_BYTES: usize = 192;

const fn max_of(values: &[usize]) -> usize {
    let mut maximum = 0;
    let mut i = 0;
    while i < values.len() {
        if values[i] > maximum {
            maximum = values[i];
        }
        i += 1;
    }
    maximum
}

const TRANSITION_SCRATCH_BYTES: usize = max_of(&[
    tasks::SCRATCH_BYTES,
    canary::SCRATCH_BYTES,
    laps::SCRATCH_BYTES,
    commit_reveal::SCRATCH_BYTES,
    evidence::SEAL_SCRATCH_BYTES,
    challenge::CHALLENGE_SCRATCH_BYTES,
    authority::SCRATCH_BYTES,
    workers::CONTROL_SCRATCH_BYTES,
    closure::SCRATCH_BYTES,
    rewards::SCRATCH_BYTES,
    F01_SECTION_CAP + CONTROL_CAP,
    REBIND_SCRATCH_BYTES,
    admission::ADMISSION_SCRATCH_BYTES,
    rooms::SCRATCH_BYTES,
    rewards::bonds::SCRATCH_BYTES,
]);
/// Router scratch: at least the largest scratch any routed transition or the F01 revision
/// rebinding needs.
pub const SCRATCH_BYTES: usize = max_of(&[
    epoch::ADVANCE_SCRATCH_BYTES,
    epoch::OPEN_SCRATCH_BYTES,
    epoch::AGGREGATE_SCRATCH_BYTES,
    epoch::HISTORY_SCRATCH_BYTES,
    TRANSITION_SCRATCH_BYTES + epoch::CARRY_SCRATCH_BYTES,
]);

/// Caller-owned outputs of one routed call.
pub struct Buffers<'b> {
    pub next: &'b mut [u8],
    pub scratch: &'b mut [u8],
    pub event: &'b mut [u8],
    pub result: &'b mut [u8],
}

#[derive(Clone, Copy, Eq, PartialEq)]
#[cfg_attr(not(target_arch = "wasm32"), derive(Debug))]
pub enum Routed {
    /// `next[..state_len]` and `event[..event_len]` must be committed together with the
    /// `Ok` result frame in `result[..result_len]`.
    Applied {
        operation: Operation,
        revision: u64,
        state_len: usize,
        event_len: usize,
        result_len: usize,
    },
    /// Nothing changes; `result[..result_len]` is an `AlreadyApplied` frame.
    Unchanged { result_len: usize },
    /// Nothing changes; `result[..result_len]` is the error frame.
    Refused { result_len: usize },
}

enum Payload {
    Empty,
    Suffix,
    // Record bytes stay in the route-owned fixed buffer across reputation carry.
    Record(usize),
}
impl Payload {
    #[cfg_attr(target_arch = "wasm32", inline(never))]
    fn record(bytes: &[u8], out: &mut [u8; RECORD_PAYLOAD_BYTES]) -> CodecResult<Self> {
        out.get_mut(..bytes.len())
            .ok_or(NON_CANONICAL)?
            .guest_copy_from_slice(bytes);
        Ok(Self::Record(bytes.len()))
    }
}

enum Step {
    Applied {
        revision: u64,
        state_len: usize,
        event_len: usize,
        payload: Payload,
    },
    Retained(RetainedResult),
    Unchanged(Payload),
    Read {
        payload_len: usize,
    },
}

/// Routes one direct native call to its landed feature transition and frames its result.
///
/// Every selector without a landed transition refuses `UNKNOWN_OPERATION`. `current` is the
/// committed `paxai/state/v1` value, absent before CREATE. A refused call writes nothing.
///
/// # Errors
/// Returns an error only when the result frame itself cannot be encoded into `result`.
#[cfg_attr(target_arch = "wasm32", inline(never))]
pub fn route(
    ctx: &CallContext,
    envelope_bytes: &[u8],
    current: Option<&[u8]>,
    buffers: Buffers<'_>,
) -> CodecResult<Routed> {
    if matches!(envelope_bytes.get(8..10), Some([0x0b, 0x10..=0x13])) {
        return route_f11_view(ctx, envelope_bytes, current, buffers);
    }
    if queries::placement::is_book_view(envelope_bytes) {
        return route_book_view(ctx, envelope_bytes, current, buffers);
    }
    if commit_reveal::is_canary_score(envelope_bytes) {
        return route_canary_score(ctx, envelope_bytes, current, buffers);
    }
    let (envelope, request) = match request_prologue(envelope_bytes) {
        Ok(request) => request,
        Err(error) => return refuse(error, Presence::Absent, 0, buffers.result),
    };
    let operation = envelope.envelope.operation;
    let Buffers {
        next,
        scratch,
        event,
        result,
    } = buffers;
    let mut record = [0; RECORD_PAYLOAD_BYTES];
    let step = if operation == FUND {
        present(current).and_then(|current| {
            reward_step(
                rewards::apply_fund(ctx, &envelope, request, current, next, scratch, event)?,
                &mut record,
            )
        })
    } else {
        transition(
            ctx,
            &envelope,
            envelope_bytes,
            current,
            next,
            scratch,
            event,
            &mut record,
        )
    };
    result_epilogue(
        operation, request, current, step, event, scratch, &record, result,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn request_prologue(input: &[u8]) -> CodecResult<(ValidatedEnvelope<'_>, RequestDigest)> {
    let envelope = codec::decode_envelope(input)?;
    let request = envelope.request_digest()?;
    Ok((envelope, request))
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn visible_revision(current: Option<&[u8]>) -> u64 {
    current
        .and_then(|bytes| state::decode_shared_state(bytes).ok())
        .map_or(0, |state| state.revision)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn result_epilogue(
    operation: Operation,
    request: RequestDigest,
    current: Option<&[u8]>,
    step: CodecResult<Step>,
    event: &[u8],
    scratch: &[u8],
    record: &[u8; RECORD_PAYLOAD_BYTES],
    result: &mut [u8],
) -> CodecResult<Routed> {
    let framed = match step {
        Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload,
        }) => applied_frame(
            operation, request, revision, event, event_len, &payload, record, result,
        )
        .map(|result_len| Routed::Applied {
            operation,
            revision,
            state_len,
            event_len,
            result_len,
        }),
        Ok(Step::Retained(retained)) => {
            let digest = retained.result_digest.bytes();
            frame(
                ApplicationResult::success(
                    ResultStatus::AlreadyApplied,
                    request,
                    retained.applied_revision,
                    &digest,
                ),
                result,
            )
            .map(|result_len| Routed::Unchanged { result_len })
        }
        Ok(Step::Unchanged(payload)) => {
            let bytes: &[u8] = match &payload {
                Payload::Empty | Payload::Suffix => &[],
                Payload::Record(n) => record.guest_index(..*n),
            };
            frame(
                ApplicationResult::success(
                    ResultStatus::AlreadyApplied,
                    request,
                    visible_revision(current),
                    bytes,
                ),
                result,
            )
            .map(|result_len| Routed::Unchanged { result_len })
        }
        Ok(Step::Read { payload_len }) => scratch
            .get(..payload_len)
            .ok_or(NON_CANONICAL)
            .and_then(|body| {
                queries::frame_read_result(request, visible_revision(current), body, result)
            })
            .map(|result_len| Routed::Unchanged { result_len }),
        Err(error) => Err(error),
    };
    finish_framed(request, current, framed, result)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn finish_framed(
    request: RequestDigest,
    current: Option<&[u8]>,
    framed: CodecResult<Routed>,
    result: &mut [u8],
) -> CodecResult<Routed> {
    match framed {
        Ok(routed) => Ok(routed),
        Err(error) => refuse(
            error,
            Presence::Present(request),
            visible_revision(current),
            result,
        ),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn route_book_view(
    ctx: &CallContext,
    input: &[u8],
    current: Option<&[u8]>,
    buffers: Buffers<'_>,
) -> CodecResult<Routed> {
    let visible = visible_revision(current);
    let request = match queries::placement::decode_book_envelope(input) {
        Ok(request) => request,
        Err(error) => return refuse(error, Presence::Absent, visible, buffers.result),
    };
    let digest = match request.request_digest() {
        Ok(digest) => digest,
        Err(error) => return refuse(error, Presence::Absent, visible, buffers.result),
    };
    let result = buffers.result;
    let answer = answer_view(
        ctx,
        &request.envelope,
        digest,
        current,
        buffers.scratch,
        result,
    );
    match answer {
        Ok(result_len) => Ok(Routed::Unchanged { result_len }),
        Err(error) => refuse(error, Presence::Present(digest), visible, result),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn route_f11_view(
    ctx: &CallContext,
    input: &[u8],
    current: Option<&[u8]>,
    buffers: Buffers<'_>,
) -> CodecResult<Routed> {
    let visible = visible_revision(current);
    let request = match queries::decode_view_envelope(input) {
        Ok(request) => request,
        Err(error) => return refuse(error, Presence::Absent, visible, buffers.result),
    };
    let digest = match request.request_digest() {
        Ok(digest) => digest,
        Err(error) => return refuse(error, Presence::Absent, visible, buffers.result),
    };
    let result = buffers.result;
    let answer = answer_view(
        ctx,
        &request.envelope,
        digest,
        current,
        buffers.scratch,
        result,
    );
    match answer {
        Ok(result_len) => Ok(Routed::Unchanged { result_len }),
        Err(error) => refuse(error, Presence::Present(digest), visible, result),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn answer_view(
    ctx: &CallContext,
    envelope: &codec::Envelope<'_>,
    digest: RequestDigest,
    current: Option<&[u8]>,
    scratch: &mut [u8],
    result: &mut [u8],
) -> CodecResult<usize> {
    codec::compare_native_principal(envelope, ctx.principal)?;
    envelope.check_domain(
        ctx.chain,
        ctx.program,
        codec::derive_market(ctx.chain, ctx.program)?,
    )?;
    envelope.check_expiry(ctx.height)?;
    let current = present(current)?;
    let shared = state::decode_shared_state(current)?;
    bound_section(ctx, &shared)?;
    let answer = match envelope.operation {
        RoomView => queries::answer_room_view,
        DealView => queries::answer_deal_view,
        PeerView => queries::answer_peer_view,
        ReceiptBatchView => queries::answer_receipt_batch_view,
        BookView => queries::answer_book_view,
        _ => return Err(UNKNOWN_OPERATION),
    };
    let len = answer(current, envelope.payload, scratch)?;
    queries::frame_read_result(digest, shared.revision, scratch.guest_index(..len), result)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn route_canary_score(
    ctx: &CallContext,
    input: &[u8],
    current: Option<&[u8]>,
    buffers: Buffers<'_>,
) -> CodecResult<Routed> {
    let visible = visible_revision(current);
    let score = match commit_reveal::decode_canary_envelope(input) {
        Ok(score) => score,
        Err(error) => return refuse(error, Presence::Absent, visible, buffers.result),
    };
    let request = match score.request_digest() {
        Ok(request) => request,
        Err(error) => return refuse(error, Presence::Absent, visible, buffers.result),
    };
    let Buffers {
        next,
        scratch,
        event,
        result,
    } = buffers;
    let applied = present(current).and_then(|current| {
        commit_reveal::apply_canary_score(ctx, &score, current, next, scratch, event)
    });
    let operation = score.envelope.operation;
    match applied {
        Ok(canary::Outcome::Applied {
            record,
            revision,
            state_len,
            event_len,
        }) => {
            let (payload, n) =
                commit_reveal::canary_score_payload(&record, operation, score.envelope.sequence)?;
            let mut record = [0; RECORD_PAYLOAD_BYTES];
            result_epilogue(
                operation,
                request,
                current,
                Ok(Step::Applied {
                    revision,
                    state_len,
                    event_len,
                    payload: Payload::record(payload.guest_index(..n), &mut record)?,
                }),
                event,
                scratch,
                &record,
                result,
            )
        }
        Ok(canary::Outcome::Retained(retained)) => {
            let result_len = frame(
                ApplicationResult::success(
                    ResultStatus::AlreadyApplied,
                    request,
                    retained.applied_revision,
                    &retained.result_digest.bytes(),
                ),
                result,
            )?;
            Ok(Routed::Unchanged { result_len })
        }
        Ok(canary::Outcome::AlreadyApplied) => Err(NON_CANONICAL),
        Err(error) => refuse(error, Presence::Present(request), visible, result),
    }
}

fn frame(value: CodecResult<ApplicationResult<'_>>, out: &mut [u8]) -> CodecResult<usize> {
    codec::encode_result(&value?, out)
}

fn refuse(
    error: crate::errors::ApplicationError,
    request: Presence<RequestDigest>,
    visible: u64,
    out: &mut [u8],
) -> CodecResult<Routed> {
    let result_len = failure_frame(error, request, visible, out)?;
    Ok(Routed::Refused { result_len })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
pub(crate) fn failure_frame(
    error: crate::errors::ApplicationError,
    request: Presence<RequestDigest>,
    visible: u64,
    out: &mut [u8],
) -> CodecResult<usize> {
    frame(ApplicationResult::failure(error, request, visible), out)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn applied_frame(
    operation: Operation,
    request: RequestDigest,
    revision: u64,
    event: &[u8],
    event_len: usize,
    payload: &Payload,
    record: &[u8; RECORD_PAYLOAD_BYTES],
    out: &mut [u8],
) -> CodecResult<usize> {
    let mut topic = [0; 64];
    let topic_len = codec::event_topic(operation, &mut topic)?;
    let body = event.get(..event_len).ok_or(NON_CANONICAL)?;
    let (named, common, suffix) = codec::decode_event_frame(topic.guest_index(..topic_len), body)?;
    let bytes = match payload {
        Payload::Empty => &[],
        Payload::Suffix => suffix,
        Payload::Record(n) => record.guest_index(..*n),
    };
    let value = ApplicationResult::success(ResultStatus::Ok, request, revision, bytes)?;
    if named != operation
        || common.revision != revision
        || common.request != request
        || common.result != value.digest
    {
        return Err(NON_CANONICAL);
    }
    codec::encode_result(&value, out)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn transition(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    if current.is_none()
        || matches!(
            envelope.envelope.operation.0,
            0x0105 | 0x0111 | 0x0401 | 0x0402 | 0x0501..=0x0503 | 0x0601 | 0x0701..=0x0703 | 0x0A01 | 0x0A02 | 0x0B01..=0x0B14
        )
    {
        return transition_inner(
            ctx,
            envelope,
            envelope_bytes,
            current,
            next,
            scratch,
            event,
            record_out,
        );
    }
    let (inner, carry) = scratch
        .split_at_mut_checked(TRANSITION_SCRATCH_BYTES)
        .ok_or(crate::errors::CAPACITY)?;
    let (mut step, written) =
        epoch::carry_reputation(present(current)?, next, carry, |current, next| {
            let step = transition_inner(
                ctx,
                envelope,
                envelope_bytes,
                Some(current),
                next,
                inner,
                event,
                record_out,
            )?;
            let written = match &step {
                Step::Applied { state_len, .. } => Some(*state_len),
                _ => None,
            };
            Ok((step, written))
        })?;
    if let Step::Applied { state_len, .. } = &mut step {
        *state_len = written.ok_or(NON_CANONICAL)?;
    }
    Ok(step)
}

// A selected function pointer keeps the guest operation handlers separate through
// packaging; direct single-use calls are folded together by the Wasm optimizer.
type TransitionHandler = fn(
    &CallContext,
    &ValidatedEnvelope<'_>,
    &[u8],
    Option<&[u8]>,
    &mut [u8],
    &mut [u8],
    &mut [u8],
    &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step>;

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn transition_inner(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    let handler: TransitionHandler = match envelope.envelope.operation.0 {
        0x0101..=0x0104 | 0x0106..=0x0109 => handle_registry,
        0x0105 => handle_advance,
        0x010A | 0x010B => handle_close,
        0x0111 => handle_open_epoch,
        0x010C..=0x0110 => handle_tasks,
        0x0201..=0x0207 | 0x0209 | 0x020A | 0x0301..=0x0303 => handle_identity,
        0x0304 | 0x0401 | 0x0402 | 0x0901 => handle_assessment,
        0x0501..=0x0503 | 0x0601..=0x0605 | 0x0701..=0x0703 => handle_settlement,
        0x0801..=0x0809 => handle_admission,
        0x0A01 | 0x0A02 => handle_read,
        0x0B01..=0x0B04 | 0x0B09..=0x0B0C => handle_rooms,
        0x0B05 => handle_laps,
        0x0B08 => handle_split_payout,
        0x0B06 | 0x0B07 => handle_canary,
        _ => return Err(UNKNOWN_OPERATION),
    };
    handler(
        ctx,
        envelope,
        envelope_bytes,
        current,
        next,
        scratch,
        event,
        record_out,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_registry(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    registry(ctx, envelope, current, next, scratch, event, record_out)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_advance(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    _record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    let a = epoch::advance_activation(ctx, envelope, present(current)?, next, scratch, event)?;
    Ok(Step::Applied {
        revision: a.revision,
        state_len: a.state_len,
        event_len: a.event_len,
        payload: Payload::Suffix,
    })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_close(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    close(
        ctx,
        envelope,
        present(current)?,
        next,
        scratch,
        event,
        record_out,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_open_epoch(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    _record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match epoch::open_epoch(ctx, envelope, present(current)?, next, scratch, event)? {
        epoch::Outcome::Opened {
            revision,
            state_len,
            event_len,
            ..
        } => Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload: Payload::Suffix,
        }),
        epoch::Outcome::AlreadyApplied { .. } => Ok(Step::Unchanged(Payload::Empty)),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_tasks(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match tasks::apply(ctx, envelope, present(current)?, next, scratch, event)? {
        tasks::Outcome::Applied {
            receipt,
            revision,
            state_len,
            event_len,
        } => Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload: Payload::record(receipt.bytes(), record_out)?,
        }),
        tasks::Outcome::AlreadyApplied { subject } => Ok(Step::Unchanged(Payload::record(
            subject.as_bytes(),
            record_out,
        )?)),
        tasks::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_identity(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    let operation = envelope.envelope.operation;
    identity(
        ctx,
        operation,
        envelope_bytes,
        present(current)?,
        next,
        scratch,
        event,
        record_out,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_assessment(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    assessment(
        ctx,
        envelope,
        present(current)?,
        next,
        scratch,
        event,
        record_out,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_settlement(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    settlement(
        ctx,
        envelope,
        present(current)?,
        next,
        scratch,
        event,
        record_out,
    )
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_admission(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    _record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match admission::apply_admission(ctx, envelope, present(current)?, next, scratch, event) {
        admission::AdmissionOutcome::Applied {
            revision,
            state_len,
            event_len,
            ..
        } => Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload: Payload::Suffix,
        }),
        admission::AdmissionOutcome::Retained(retained) => Ok(Step::Retained(retained)),
        admission::AdmissionOutcome::Refused(error) => Err(error),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_read(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    _next: &mut [u8],
    scratch: &mut [u8],
    _event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    read(ctx, envelope, present(current)?, scratch, record_out)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_rooms(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    let current = present(current)?;
    let operation = envelope.envelope.operation;
    let outcome = match operation {
        LockBond => rewards::bonds::apply_lock_bond(ctx, envelope, current, next, scratch, event)?,
        SlashBond => {
            rewards::bonds::apply_slash_bond(ctx, envelope, current, next, scratch, event)?
        }
        RefundBond => {
            rewards::bonds::apply_refund_bond(ctx, envelope, current, next, scratch, event)?
        }
        CloseRoom => {
            rewards::bonds::apply_close_room(ctx, envelope, current, next, scratch, event)?
        }
        _ => queries::placement::apply(ctx, envelope, current, next, scratch, event)?,
    };
    match outcome {
        rooms::Outcome::Applied {
            revision,
            state_len,
            event_len,
        } => Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload: Payload::Suffix,
        }),
        rooms::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
        rooms::Outcome::AlreadyApplied(subject) => Ok(Step::Unchanged(Payload::record(
            subject.as_bytes(),
            record_out,
        )?)),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_laps(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    _record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match laps::apply_settle_laps(ctx, envelope, present(current)?, next, scratch, event)? {
        laps::Outcome::Applied {
            revision,
            state_len,
            event_len,
        } => Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload: Payload::Suffix,
        }),
        laps::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_split_payout(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match rewards::apply_split_payout(ctx, envelope, present(current)?, next, scratch, event)? {
        rooms::Outcome::Applied {
            revision,
            state_len,
            event_len,
        } => Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload: Payload::Suffix,
        }),
        rooms::Outcome::AlreadyApplied(subject) => Ok(Step::Unchanged(Payload::record(
            subject.as_bytes(),
            record_out,
        )?)),
        rooms::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn handle_canary(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    _envelope_bytes: &[u8],
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    _record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    let operation = envelope.envelope.operation;
    let outcome = if operation == ChallengeLap {
        canary::apply_challenge_lap(ctx, envelope, present(current)?, next, scratch, event)?
    } else {
        canary::apply_resolve_challenge(ctx, envelope, present(current)?, next, scratch, event)?
    };
    match outcome {
        canary::Outcome::Applied {
            revision,
            state_len,
            event_len,
            ..
        } => Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload: Payload::Suffix,
        }),
        canary::Outcome::AlreadyApplied => Ok(Step::Unchanged(Payload::Empty)),
        canary::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn close(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match closure::apply(ctx, envelope, current, next, scratch, event)? {
        closure::Outcome::Applied {
            receipt,
            revision,
            state_len,
            event_len,
            effect,
            ..
        } => {
            if effect != rewards::RewardEffect::NoTransfer {
                let before = state::decode_shared_state(current)?;
                let after =
                    state::decode_shared_state(next.get(..state_len).ok_or(NON_CANONICAL)?)?;
                let before = state::settlement_rewards(&before, None)?.ledger()?;
                let after = state::settlement_rewards(&after, None)?.ledger()?;
                let bound = value_adapter::RewardsAccount::for_ledger(ctx.program, &before)?;
                #[cfg(target_arch = "wasm32")]
                value_adapter::settle(&bound, &before, &after, effect)?;
                #[cfg(not(target_arch = "wasm32"))]
                value_adapter::plan(&bound, &before, &after, effect)?;
            }
            Ok(Step::Applied {
                revision,
                state_len,
                event_len,
                payload: Payload::record(receipt.bytes(), record_out)?,
            })
        }
        closure::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
    }
}

fn reward_step(
    outcome: rewards::Outcome,
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match outcome {
        rewards::Outcome::Applied {
            response,
            revision,
            state_len,
            event_len,
            ..
        } => Ok(Step::Applied {
            revision,
            state_len,
            event_len,
            payload: Payload::record(response.as_bytes(), record_out)?,
        }),
        rewards::Outcome::AlreadyApplied { subject } => {
            Ok(Step::Unchanged(Payload::record(&subject, record_out)?))
        }
        rewards::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn settlement(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match envelope.envelope.operation.0 {
        0x0501..=0x0503 => match epoch::aggregate(ctx, envelope, current, next, scratch, event)? {
            aggregation::Outcome::Applied {
                response,
                revision,
                state_len,
                event_len,
                ..
            } => Ok(Step::Applied {
                revision,
                state_len,
                event_len,
                payload: Payload::record(response.as_bytes(), record_out)?,
            }),
            aggregation::Outcome::AlreadyApplied { response, .. } => Ok(Step::Unchanged(
                Payload::record(response.as_bytes(), record_out)?,
            )),
        },
        0x0601..=0x0605 => reward_step(
            rewards::apply(ctx, envelope, current, next, scratch, event)?,
            record_out,
        ),

        0x0701..=0x0703 => match epoch::history(ctx, envelope, current, next, scratch, event)? {
            epoch::HistoryOutcome::Applied {
                revision,
                state_len,
                event_len,
                ..
            } => Ok(Step::Applied {
                revision,
                state_len,
                event_len,
                payload: Payload::Suffix,
            }),
            epoch::HistoryOutcome::Retained(retained) => Ok(Step::Retained(retained)),
        },
        _ => Err(UNKNOWN_OPERATION),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn read(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    out: &mut [u8],
    _record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    let e = &envelope.envelope;
    codec::compare_native_principal(e, ctx.principal)?;
    e.check_domain(
        ctx.chain,
        ctx.program,
        codec::derive_market(ctx.chain, ctx.program)?,
    )?;
    e.check_expiry(ctx.height)?;
    let state = state::decode_shared_state(current)?;
    bound_section(ctx, &state)?;
    let header = queries::read_header(current, ctx.chain, ctx.program)?;
    let payload_len = match e.operation {
        READ_HEADER => codec::encode_read_header(&header, out)?,
        READ_STATE_CHUNK => queries::read_state_chunk(current, e.payload, out)?,
        _ => return Err(UNKNOWN_OPERATION),
    };
    Ok(Step::Read { payload_len })
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn identity(
    ctx: &CallContext,
    operation: Operation,
    envelope_bytes: &[u8],
    bytes: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    _record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    let state = state::decode_shared_state(bytes)?;
    let section = bound_section(ctx, &state)?;
    match operation.0 {
        0x0201..=0x0207 | 0x0209 | 0x020A => {
            let call = workers::CallContext {
                market: &section.header,
                invoking_principal: ctx.principal,
                height: ctx.height,
            };
            match workers::apply(bytes, &call, envelope_bytes, next, event, scratch)? {
                workers::Applied::Applied {
                    state_len,
                    event_len,
                } => rebind(next, state_len, event_len, scratch),
                workers::Applied::AlreadyApplied(retained) => Ok(Step::Retained(retained)),
            }
        }
        0x0301..=0x0303 => {
            let aggregate_sealed =
                authority::evaluator_region(state.section(Section::IdentityRoster)?)?
                    .snapshot()
                    .is_some()
                    && aggregation::progress(bytes)?.phase
                        != crate::aggregation_codec::AggregationPhase::Unsealed;
            let call = authority::AuthorityContext {
                market: &section.header,
                invoking_principal: ctx.principal,
                immediate_caller: Presence::Absent,
                height: ctx.height,
                approved_rubric: section.current.commitments.rubric,
                aggregate_sealed,
            };
            match authority::apply(bytes, &call, envelope_bytes, scratch, next, event)? {
                authority::Outcome::Applied {
                    state_len,
                    event_len,
                } => rebind(next, state_len, event_len, scratch),
                authority::Outcome::AlreadyApplied(retained) => Ok(Step::Retained(retained)),
                authority::Outcome::Idempotent => Ok(Step::Unchanged(Payload::Empty)),
            }
        }
        _ => Err(UNKNOWN_OPERATION),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn assessment(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: &[u8],
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    match envelope.envelope.operation.0 {
        0x0304 => match challenge::apply_challenge(ctx, envelope, current, next, scratch, event)? {
            challenge::ChallengeOutcome::Applied {
                record,
                revision,
                state_len,
                event_len,
                ..
            } => Ok(Step::Applied {
                revision,
                state_len,
                event_len,
                payload: Payload::record(&record.encode()?, record_out)?,
            }),
            challenge::ChallengeOutcome::AlreadyApplied { record } => Ok(Step::Unchanged(
                Payload::record(&record.encode()?, record_out)?,
            )),
        },
        0x0401 | 0x0402 => {
            match commit_reveal::apply(ctx, envelope, current, next, scratch, event)? {
                commit_reveal::Outcome::Committed {
                    record,
                    revision,
                    state_len,
                    event_len,
                    ..
                } => Ok(Step::Applied {
                    revision,
                    state_len,
                    event_len,
                    payload: Payload::record(&record.payload()?, record_out)?,
                }),
                commit_reveal::Outcome::Revealed {
                    receipt,
                    revision,
                    state_len,
                    event_len,
                } => Ok(Step::Applied {
                    revision,
                    state_len,
                    event_len,
                    payload: Payload::record(&receipt.payload()?, record_out)?,
                }),
                commit_reveal::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
            }
        }
        0x0901 => match evidence::apply(ctx, envelope, current, next, scratch, event)? {
            evidence::Outcome::Applied {
                seal,
                revision,
                state_len,
                event_len,
                ..
            } => Ok(Step::Applied {
                revision,
                state_len,
                event_len,
                payload: Payload::record(&seal.encode()?, record_out)?,
            }),
            evidence::Outcome::AlreadyApplied { seal } => Ok(Step::Unchanged(Payload::record(
                &seal.encode()?,
                record_out,
            )?)),
            evidence::Outcome::Retained(retained) => Ok(Step::Retained(retained)),
        },
        _ => Err(UNKNOWN_OPERATION),
    }
}

fn present(current: Option<&[u8]>) -> CodecResult<&[u8]> {
    current.ok_or(NOT_FOUND)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn registry(
    ctx: &CallContext,
    envelope: &ValidatedEnvelope<'_>,
    current: Option<&[u8]>,
    next: &mut [u8],
    scratch: &mut [u8],
    event: &mut [u8],
    record_out: &mut [u8; RECORD_PAYLOAD_BYTES],
) -> CodecResult<Step> {
    let decoded = match current {
        Some(bytes) => Some(state::decode_shared_state(bytes)?),
        None => None,
    };
    let (section_out, control) = scratch
        .get_mut(..F01_SECTION_CAP + CONTROL_CAP)
        .ok_or(NON_CANONICAL)?
        .guest_split_at_mut(F01_SECTION_CAP);
    match registry_ops::apply(ctx, decoded.as_ref(), envelope, section_out, event)? {
        registry_ops::Outcome::Applied {
            state,
            receipt,
            event_len,
        } => Ok(Step::Applied {
            revision: state.revision,
            state_len: state::encode_shared_state(&state, next, control)?,
            event_len,
            payload: Payload::record(receipt.bytes(), record_out)?,
        }),
        registry_ops::Outcome::AlreadyApplied(retained) => Ok(Step::Retained(retained)),
    }
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn bound_section<'a>(ctx: &CallContext, state: &SharedState<'a>) -> CodecResult<PolicySection<'a>> {
    let section = PolicySection::decode(
        *state
            .feature_sections
            .guest_index(Section::PolicyLifecycle.index()),
    )?;
    if section.header.state_revision != state.revision {
        return Err(NON_CANONICAL);
    }
    if section.header.deployment_chain_domain != ctx.chain {
        return Err(WRONG_DOMAIN);
    }
    if section.header.program_id != ctx.program {
        return Err(WRONG_PROGRAM);
    }
    Ok(section)
}

#[cfg_attr(target_arch = "wasm32", inline(never))]
fn rebind(
    next: &mut [u8],
    state_len: usize,
    event_len: usize,
    scratch: &mut [u8],
) -> CodecResult<Step> {
    let (section_out, rest) = scratch
        .get_mut(..REBIND_SCRATCH_BYTES)
        .ok_or(NON_CANONICAL)?
        .guest_split_at_mut(F01_SECTION_CAP);
    let (control, staged) = rest.guest_split_at_mut(CONTROL_CAP);
    let (revision, staged_len) = {
        let state = state::decode_shared_state(next.get(..state_len).ok_or(NON_CANONICAL)?)?;
        let mut section = PolicySection::decode(
            *state
                .feature_sections
                .guest_index(Section::PolicyLifecycle.index()),
        )?;
        section.header.state_revision = state.revision;
        let section_len = section.encode(section_out)?;
        let revision = state.revision;
        let rebound = state.with_section(
            Section::PolicyLifecycle,
            section_out.guest_index(..section_len),
        )?;
        (
            revision,
            state::encode_shared_state(&rebound, staged, control)?,
        )
    };
    next.get_mut(..staged_len)
        .ok_or(NON_CANONICAL)?
        .guest_copy_from_slice(staged.guest_index(..staged_len));
    Ok(Step::Applied {
        revision,
        state_len: staged_len,
        event_len,
        payload: Payload::Suffix,
    })
}
