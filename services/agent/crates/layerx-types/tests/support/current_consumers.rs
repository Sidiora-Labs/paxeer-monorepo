// Link production consumers with their real dependencies and shared types.
// The independent verifier below authenticates the recorded producer evidence.
pub use layerx_agent_api::TimestampSeconds;
pub use layerx_client::budget;

pub mod error {
    pub use layerx_agent_api::error::{ApiError, ErrorClass, ReasonCode, RequestId};
    pub use layerx_agent_api::verify::{ApiSuccess, RungStatus};
}

pub mod write_contract {
    pub use layerx_agent_api::prepare::CanonicalBytes;
    pub use layerx_agent_api::track::{
        EvidenceRef, ReceiptRef, SubmissionRef, SubmissionState, TrackedSubmission, WaitRequest,
        WaitResult,
    };
}

#[path = "independent/mod.rs"]
mod independent;
#[path = "proof_cases.rs"]
mod proof_cases;
#[path = "recorded_commit.rs"]
mod recorded_commit;
