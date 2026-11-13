#![forbid(unsafe_code)]

pub mod canonical;
#[path = "compile.rs"]
mod compiler;
mod disclosure;
pub mod golden;
pub mod keccak;
mod native_budget;
mod native_receive;
mod reject;
#[cfg(feature = "test-vectors")]
pub mod vectors;
mod vocabulary;

pub use compiler::{
    compile, CompileError, CompileErrorReason, CompileField, CompiledIntent, NativeOwnerBootstrap,
};
pub use disclosure::{DisclosureCheck, DisclosureCheckError, DisclosureField};
pub use native_budget::NativeBudgetCreate;
pub use native_receive::NativeReceive;
pub use reject::{inspect_intent, IntentHeader, IntentKindTag, RejectReason, RejectedIntent};

/// Names of the committed, scheduled fuzz surfaces shipped by this crate.
pub mod fuzz_targets {
    pub const INTENT_PARSING_AND_COMPILATION: &str = "intent";
}

pub use vocabulary::{
    BudgetCreate, BudgetDefund, BudgetFund, DidRegistration, Intent, IntentError,
    IntentErrorReason, IntentField, IntentKind, IntentVersion, KeyRotation, LxpReceive, LxpSend,
    PayerGrantRegistration, RecoveryRegistration, SessionGrant, SessionRevoke,
};

/// Stable identity of the sole human-plane payload authority.
pub const CRATE_IDENTITY: &str = "layerx-intents";
