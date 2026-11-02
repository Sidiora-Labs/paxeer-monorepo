use crate::{chain_proof::VerifiedChainHead, ProtocolEvidenceError, ReadFreshness};

pub fn chain_freshness(
    head: &VerifiedChainHead,
    now_ms: u64,
    max_age_ms: u64,
) -> Result<ReadFreshness, ProtocolEvidenceError> {
    let time_ms = u64::try_from(head.time_ms()).map_err(|_| ProtocolEvidenceError::Stale)?;
    if time_ms.saturating_sub(now_ms) > 1_000 || now_ms.saturating_sub(time_ms) > max_age_ms {
        return Err(ProtocolEvidenceError::Stale);
    }
    Ok(ReadFreshness {
        observed_sequence: u64::try_from(head.height())
            .map_err(|_| ProtocolEvidenceError::Encoding)?,
        observed_at: time_ms,
    })
}
