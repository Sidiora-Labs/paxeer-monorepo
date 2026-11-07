//! Library-owned support for integration tests of otherwise private code.

pub use crate::agent_rpc_wire::version_exchange;
pub use crate::managed_agent::rotation::{prepare, OwnerRotation, Prepared, Projection};
pub use crate::outbound_tls::private_ca;

#[path = "prepare/restore_tests.rs"]
pub mod preparation;
