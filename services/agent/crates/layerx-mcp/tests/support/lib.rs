//! Shared MCP integration fixtures, also checked as the `support_lints` test target.

pub mod approval_fixture;
#[path = "seller_config.rs"]
pub mod config;
pub mod native_alias;
pub mod node_recording;
#[path = "assert_option.rs"]
pub mod option_assertion;
pub mod payment_binding;
#[path = "assert_refusal.rs"]
pub mod refusal_assertion;
#[path = "assert_result.rs"]
pub mod result_assertion;
pub mod seller_exchange;
#[path = "seller_server.rs"]
pub mod seller_server;
pub mod session_transport;
#[path = "clock.rs"]
pub mod test_clock;
pub mod web_hold;

/// Locates the owning MCP manifest from either fixture compilation target.
///
/// # Panics
/// Panics if the fixture source is compiled outside the MCP crate tree.
#[must_use]
pub fn manifest_dir() -> &'static std::path::Path {
    match std::path::Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .find(|path| path.join("src/repair_22_39_224.kvx").is_file())
    {
        Some(path) => path,
        None => panic!("owning MCP manifest directory"),
    }
}
