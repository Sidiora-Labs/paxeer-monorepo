//! Repair 22.33.1548: f50 is proven by `proof_cases`; f49 by `rung_cases`.
//! The adjacent `repair_22_33_1548.kvx` records coverage, baseline limitations,
//! and out-of-path wiring. Exact revision and private gate evidence are supplied
//! by hf-build/hf-done rather than a self-referential source revision.
include!("support/current_consumers.rs");

#[path = "support/rung_cases.rs"]
mod rung_cases;
