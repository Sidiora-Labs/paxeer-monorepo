// Compile whole production modules, without extracting declarations or launching
// a second compiler. The client budget codec is dependency-free; its API
// consumer requires the same concrete ProtocolBudgetRecord as the real crate.
extern crate self as layerx_client;

#[path = "../../../layerx-client/src/budget.rs"]
pub mod budget;
#[path = "../../../layerx-agent-api/src/generated.rs"]
pub mod generated;
pub use generated::{Amount, BudgetLimit, Sequence, TimestampSeconds};
#[path = "../../../layerx-agent-api/src/error.rs"]
pub mod error;
#[path = "../../../layerx-agent-api/src/identity.rs"]
pub mod identity;
#[path = "../../../layerx-agent-api/src/write.rs"]
pub mod write_contract;
pub mod verify {
    pub use layerx_types::rung::Rung;
}

#[path = "../../../layerx-agent-api/tests/support/independent/mod.rs"]
mod independent;
#[path = "proof_cases.rs"]
mod proof_cases;
#[path = "recorded_commit.rs"]
mod recorded_commit;
