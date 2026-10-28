//! Published daemon compatibility matrix and release-time validation.

pub const DAEMON_VERSION: &str = env!("CARGO_PKG_VERSION");
pub const CONTRACT_VERSION: u16 = 2;
pub const SDK_VERSION: &str = "0.1.0";
pub const PUBLISHED_MATRIX: &str = include_str!("../../../COMPATIBILITY.md");

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub struct Compatibility {
    pub daemon_version: &'static str,
    pub node_interface_major: u16,
    pub minimum_node_interface_minor: u16,
    pub maximum_node_interface_minor: u16,
    pub contract_version: u16,
    pub sdk_version: &'static str,
}

const MATRIX: [Compatibility; 1] = [Compatibility {
    daemon_version: DAEMON_VERSION,
    node_interface_major: 1,
    minimum_node_interface_minor: 0,
    maximum_node_interface_minor: u16::MAX,
    contract_version: CONTRACT_VERSION,
    sdk_version: SDK_VERSION,
}];

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum CompatibilityError {
    UnsupportedDaemon,
    UnsupportedNodeInterface,
    UnsupportedContract,
    UnsupportedSdk,
    PublishedMatrixDrift,
}

#[must_use]
pub const fn matrix() -> &'static [Compatibility] {
    &MATRIX
}

/// Validates a daemon, node `web3_clientVersion`, contract, and SDK version tuple.
///
/// # Errors
///
/// Names the incompatible dimension when no published row supports the tuple.
pub fn verify(
    daemon_version: &str,
    node_client_version: &str,
    contract_version: u16,
    sdk_version: &str,
) -> Result<Compatibility, CompatibilityError> {
    let daemon_rows: Vec<_> = MATRIX
        .into_iter()
        .filter(|row| row.daemon_version == daemon_version)
        .collect();
    if daemon_rows.is_empty() {
        return Err(CompatibilityError::UnsupportedDaemon);
    }
    let node_major =
        client_major(node_client_version).ok_or(CompatibilityError::UnsupportedNodeInterface)?;
    let node_rows: Vec<_> = daemon_rows
        .into_iter()
        .filter(|row| row.node_interface_major == node_major)
        .collect();
    if node_rows.is_empty() {
        return Err(CompatibilityError::UnsupportedNodeInterface);
    }
    let contract_rows: Vec<_> = node_rows
        .into_iter()
        .filter(|row| row.contract_version == contract_version)
        .collect();
    if contract_rows.is_empty() {
        return Err(CompatibilityError::UnsupportedContract);
    }
    contract_rows
        .into_iter()
        .find(|row| row.sdk_version == sdk_version)
        .ok_or(CompatibilityError::UnsupportedSdk)
}

/// Verifies that the checked-in human-readable matrix contains the executable row.
///
/// # Errors
///
/// Returns drift when any executable version dimension is absent from the publication.
pub fn verify_published() -> Result<(), CompatibilityError> {
    let row = MATRIX[0];
    let expected = format!(
        "| {} | {}.{}+ | {} | {} |",
        row.daemon_version,
        row.node_interface_major,
        row.minimum_node_interface_minor,
        row.contract_version,
        row.sdk_version
    );
    if PUBLISHED_MATRIX.lines().any(|line| line.trim() == expected) {
        Ok(())
    } else {
        Err(CompatibilityError::PublishedMatrixDrift)
    }
}

fn client_major(client_version: &str) -> Option<u16> {
    let mut parts = client_version.split('/');
    let product = parts.next()?;
    if product.is_empty() || product.bytes().any(|byte| byte.is_ascii_whitespace()) {
        return None;
    }
    let version = parts.next()?;
    let version = version.strip_prefix('v').unwrap_or(version);
    let mut release = version.split('.');
    let major = release.next()?;
    let minor = release.next()?;
    let patch = release.next()?;
    let patch_number = patch.split(['-', '+']).next()?;
    if major.is_empty()
        || minor.is_empty()
        || patch_number.is_empty()
        || !major.bytes().all(|byte| byte.is_ascii_digit())
        || !minor.bytes().all(|byte| byte.is_ascii_digit())
        || !patch_number.bytes().all(|byte| byte.is_ascii_digit())
        || release.next().is_some()
    {
        return None;
    }
    major.parse().ok()
}
