pub mod proto;
pub mod merkle {}
pub mod ics23 {}
pub mod ics23_absent {}
pub mod multistore {}
pub mod header {}
pub mod vote {}
pub mod validators {}
pub mod ed25519 {}
pub mod decode {}
pub mod head_verify {}
pub mod state {}
pub mod tx {}
pub mod results {}
pub mod trust {}
pub mod trust_store {}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum ChainProofError {
    Truncated,
    NonCanonical,
    UnknownField,
    Mismatch,
    InsufficientPower,
    Expired,
    Absent,
}
