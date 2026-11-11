#[path = "suites/approval.rs"]
mod approval_fixture;
#[path = "support/send_authorization.rs"]
mod send_authorization;

include!("suites/approval_ops.rs");
