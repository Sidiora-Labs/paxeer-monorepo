#[path = "support/authority_receipt_transcripts.rs"]
mod repair_support;
mod support;

include!("suites/grant_revocation_on_chain.rs");
