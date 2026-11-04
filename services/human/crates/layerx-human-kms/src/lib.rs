#![forbid(unsafe_code)]
pub mod attestor;
pub use attestor::call;
mod config;
mod evm;
mod evm_types;
pub mod export;
mod send;
mod server;
mod store;
mod wire;
pub use wire::signing_contract;

/// Runs the protected Human LXKP provider using its explicit environment configuration.
///
/// # Errors
/// Refuses invalid policy, TLS identity, protected storage or listener failure.
pub fn run_from_environment(
    clock: &std::sync::Arc<dyn layerx_types::clock::Clock>,
) -> Result<(), String> {
    rustls::crypto::ring::default_provider()
        .install_default()
        .map_err(|_| "TLS cryptographic provider already configured".to_owned())?;
    server::run(config::Config::load()?, clock)
}
