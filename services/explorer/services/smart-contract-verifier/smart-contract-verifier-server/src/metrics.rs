use lazy_static::lazy_static;
use prometheus::{register_int_counter_vec, IntCounterVec};
use std::sync::{
    atomic::{AtomicU64, Ordering},
    OnceLock,
};

const MAX_CHAINS: usize = 32;
const UNKNOWN_CHAIN: &str = "unknown";
static CHAIN_IDS: OnceLock<Vec<u64>> = OnceLock::new();
static LAST_UNKNOWN_LOG: AtomicU64 = AtomicU64::new(0);

lazy_static! {
    pub static ref VERIFICATION: IntCounterVec = register_int_counter_vec!(
        "smart_contract_verifier_verify_contract",
        "number of contract verifications",
        &["chain_id", "language", "endpoint", "status"],
    )
    .unwrap();
}

// Immutable for the process lifetime: changing an allowlist must not retain old series.
// Operators may explicitly configure up to 32 decimal IDs; Paxeer is the default.
pub(crate) fn configure_chain_labels() -> anyhow::Result<()> {
    let configured = match std::env::var("VERIFICATION_METRIC_CHAIN_IDS") {
        Ok(value) => value,
        Err(std::env::VarError::NotPresent) => "125".to_owned(),
        Err(error) => return Err(error.into()),
    };
    anyhow::ensure!(
        configured.len() <= 1024,
        "metric chain allowlist is too long"
    );
    let mut ids = configured
        .split(',')
        .map(|id| id.trim().parse::<u64>())
        .collect::<Result<Vec<_>, _>>()?;
    ids.sort_unstable();
    ids.dedup();
    anyhow::ensure!(
        !ids.is_empty() && ids.len() <= MAX_CHAINS,
        "metric chain allowlist must contain 1..=32 IDs"
    );
    let current = CHAIN_IDS.get_or_init(|| ids.clone());
    anyhow::ensure!(
        current == &ids,
        "metric chain allowlist cannot change in a running process"
    );
    Ok(())
}

fn chain_label(chain_id: &str) -> String {
    let ids = CHAIN_IDS.get_or_init(|| vec![125]);
    let parsed = if chain_id.len() <= 32 {
        let value = chain_id.trim();
        match value
            .strip_prefix("0x")
            .or_else(|| value.strip_prefix("0X"))
        {
            Some(hex) => u64::from_str_radix(hex, 16).ok(),
            None if value.bytes().all(|b| b.is_ascii_digit()) => value.parse::<u64>().ok(),
            None => None,
        }
    } else {
        None
    };
    if let Some(id) = parsed.filter(|id| ids.binary_search(id).is_ok()) {
        return id.to_string();
    }
    // At most one sample per minute, at most 128 characters, independent of input size.
    let now = std::time::SystemTime::now()
        .duration_since(std::time::UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    let last = LAST_UNKNOWN_LOG.load(Ordering::Relaxed);
    if now.saturating_sub(last) >= 60
        && LAST_UNKNOWN_LOG
            .compare_exchange(last, now, Ordering::Relaxed, Ordering::Relaxed)
            .is_ok()
    {
        let sample: String = chain_id.chars().take(128).collect();
        tracing::info!(chain_id = ?sample, bytes = chain_id.len(), "verification chain mapped to unknown metric bucket");
    }
    UNKNOWN_CHAIN.to_owned()
}

pub fn count_verify_contract(chain_id: &str, language: &str, status: &str, method: &str) {
    VERIFICATION
        .with_label_values(&[&chain_label(chain_id), language, method, status])
        .inc();
}
