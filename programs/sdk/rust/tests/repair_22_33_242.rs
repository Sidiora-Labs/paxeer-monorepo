//! Repair record 22.33.242, audit 22.30.129 finding 010.
//! Proving tests: finding_010_exact_limit_generated_call_survives_runtime_transport
//! and finding_010_one_byte_over_and_selector_overflow_refused_at_construction.
//! Baseline verification was not run: the ordered gate contract permits only
//! implementation, hf-build, then hf-done; no baseline failure is claimed.
//! The previous attempt's build failed on unresolved registry and runtime imports.
//! Their workspace-path dev-dependencies are now declared in the SDK manifest;
//! no out-of-path wiring remains.

#[path = "support/bytes_call_boundary.rs"]
mod bytes_call_boundary;
