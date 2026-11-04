//! Policy history requires complete native result ranges, not an index's end marker.
use crate::protocol_evidence::{
    admit_bytes, history_entries_digest, AuthenticatedCoreTime, AuthenticatedCumulativeUse,
    AuthenticatedTimeWindowUse, CumulativeUseWindow, EvidenceAuthority, HistoryActivityEvidence,
    HistoryWindowBatch, PolicyHistorySource, VerifiedHistoryCoverage,
};
use crate::read::{HistoryLimits, HistoryReadError, ServedCheckpoint};
use layerx_client::rpc::receipt::{lookup_authenticated, Lookup, LookupContext};
use layerx_types::ids::Did;
use std::collections::BTreeSet;

#[derive(Clone, Copy)]
pub enum PolicyHistoryWindow {
    Time { head: u64, seconds: u64 },
    Sequence { core: u64, width: u64 },
}
pub struct PolicyUsageHistory {
    pub(crate) entries: Vec<HistoryActivityEvidence>,
    pub(crate) checkpoint: ServedCheckpoint,
    pub(crate) observed: AuthenticatedCoreTime,
    pub(crate) first_covered: u64,
    pub(crate) coverage: VerifiedHistoryCoverage,
}
impl PolicyUsageHistory {
    pub fn cumulative(
        &self,
        actor: &Did,
        first: u64,
        last: u64,
    ) -> Result<AuthenticatedCumulativeUse, crate::protocol_evidence::CumulativeUseError> {
        CumulativeUseWindow { first, last }.authenticate_history(
            actor,
            &self.checkpoint,
            &self.entries,
            &self.coverage,
        )
    }
    pub fn time_window(
        &self,
        actor: &Did,
        seconds: u64,
    ) -> Result<AuthenticatedTimeWindowUse, crate::protocol_evidence::TimeWindowUseError> {
        HistoryWindowBatch::from_committed_history(self.entries.clone(), self.observed.clone())
            .authenticate_history(actor, seconds, self.first_covered, &self.coverage)
    }
}

/// Both transports are explicit. The native Comet endpoint owns block_results and commit.
pub fn read_policy_history_from_sources(
    config: &crate::node_rpc::NodeRpcConfig,
    result_url: &str,
    actor: &Did,
    trust: &crate::export::ExportTrustSource,
    window: PolicyHistoryWindow,
) -> Result<PolicyUsageHistory, HistoryReadError> {
    let bad = || HistoryReadError::ItemUnproven;
    let source = PolicyHistorySource::new(config, result_url)?;
    let did = std::str::from_utf8(actor.as_bytes()).map_err(|_| bad())?;
    let public =
        layerx_programs::hex::decode_digest(did.strip_prefix("did:layerx:").ok_or_else(bad)?)
            .map_err(|_| bad())?;
    let address = source
        .endpoint
        .bound_address(&public)
        .map_err(HistoryReadError::Sequencer)?
        .ok_or_else(bad)?;
    let mut account = [0; 32];
    account[12..].copy_from_slice(&address);
    let limits = HistoryLimits {
        maximum_items: 256,
        maximum_response_bytes: 16 * 1024 * 1024,
    };
    let mut page = source.page(
        &format!("0x{}", layerx_programs::hex::encode(&address)),
        None,
        limits,
    )?;
    let height = source
        .endpoint
        .block_number()
        .map_err(HistoryReadError::Sequencer)?;
    if page.height != height {
        return Err(bad());
    }
    let checkpoint = ServedCheckpoint::acquire_at_committed_block(
        &source.endpoint,
        height,
        None,
        &config.chain_id.to_string(),
    )
    .map_err(|_| bad())?;
    if checkpoint.validators != trust.validator_set() {
        return Err(bad());
    }
    let key = trust.receipt_keys().first().ok_or_else(bad)?;
    let observed = EvidenceAuthority::pinned_to_handshake(
        layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION,
        u32::try_from(config.chain_id).map_err(|_| bad())?,
        key.public_key,
    )
    .map_err(|_| bad())?
    .pin_committed_validators(config.chain_id.to_string(), trust.validator_set().to_vec())
    .map_err(|_| bad())?
    .authenticate_committed_time(&checkpoint)
    .map_err(|_| bad())?;
    let lower_time = match window {
        PolicyHistoryWindow::Time { head, seconds }
            if head == height && seconds > 0 && seconds.checked_mul(1000).is_some() =>
        {
            Some(observed.observed_core_ms().saturating_sub(seconds * 1000))
        }
        PolicyHistoryWindow::Sequence { core, width }
            if core > 0 && core <= height && width > 0 && width <= 4096 =>
        {
            None
        }
        _ => return Err(HistoryReadError::InvalidRange),
    };
    let (first, last) = match window {
        PolicyHistoryWindow::Sequence { core, width } => {
            (core.saturating_sub(width - 1).max(1), core)
        }
        PolicyHistoryWindow::Time { .. } => {
            let mut block = checkpoint.clone();
            let lower = lower_time.ok_or_else(bad)?;
            while block.height > 1 && checkpoint_timestamp_ms(&block)? > lower {
                if height - block.height >= 4095 {
                    return Err(HistoryReadError::Oversized);
                }
                block = source.committed_at(&checkpoint, block.height - 1)?;
            }
            (block.height, height)
        }
    };
    if height - first > 4096 {
        return Err(HistoryReadError::Oversized);
    }
    let mut bytes = 0;
    let mut entries = Vec::new();
    let mut actual = BTreeSet::new();
    let mut previous_cursor = None;
    let mut page_count = 0;
    loop {
        page_count += 1;
        if page_count > 4096 || page.height != height {
            return Err(bad());
        }
        admit_bytes(&mut bytes, page.response_bytes)?;
        let mut boundary = false;
        for item in &page.items {
            if item.position.block_height < first {
                boundary = true;
                break;
            }
            if item.position.block_height > last {
                continue;
            }
            if item.kind != layerx_client::rpc::history::HistoryKind::Receipt {
                continue;
            }
            let receipt = source.prove(
                &item.canonical,
                item.position,
                &checkpoint,
                trust.receipt_keys(),
                &mut bytes,
            )?;
            let decoded = layerx_wire::receipt::decode(&receipt.canonical).map_err(|_| bad())?;
            let protocol = decoded.protocol().ok_or_else(bad)?;
            if protocol.from() != account || !actual.insert(protocol.activity_id()) {
                return Err(bad());
            }
            if lower_time.is_none_or(|lower| receipt.timestamp > lower) {
                entries.push(
                    HistoryActivityEvidence::from_committed_history(actor, account, receipt)
                        .map_err(|_| bad())?,
                );
            }
            if actual.len() > 4096 {
                return Err(HistoryReadError::Oversized);
            }
        }
        if boundary || page.next_cursor.is_none() {
            break;
        }
        if page.items.is_empty() || page.next_cursor == previous_cursor {
            return Err(bad());
        }
        previous_cursor = page.next_cursor;
        page = source.page(
            &format!("0x{}", layerx_programs::hex::encode(&address)),
            page.next_cursor,
            HistoryLimits {
                maximum_response_bytes: 16 * 1024 * 1024 - bytes,
                ..limits
            },
        )?;
    }
    // Every native result leaf is enumerated, including non-EVM and failed results.
    // Authentication of the entire root proves the absence of omitted outcome leaves.
    let mut expected = BTreeSet::new();
    let mut items = 0usize;
    for block_height in first..=last {
        let block = source.committed_at(&checkpoint, block_height)?;
        let next = source.successor(&checkpoint, &block)?;
        let results = source
            .results
            .all_results(block_height)
            .map_err(|_| bad())?;
        items = items
            .checked_add(results.len())
            .filter(|n| *n <= 4096)
            .ok_or(HistoryReadError::Oversized)?;
        for result in &results {
            admit_bytes(&mut bytes, result.len())?;
        }
        if next.header.last_results_hash
            != Some(layerx_programs::chain_proof::merkle::hash_from_byte_slices(
                &results.iter().map(Vec::as_slice).collect::<Vec<_>>(),
            ))
        {
            return Err(bad());
        }
        for (index, result) in results.iter().enumerate() {
            let Some(hash) =
                crate::read::abci_result::transaction_hash(result).map_err(|_| bad())?
            else {
                continue;
            };
            let Lookup::Verified(receipt) = lookup_authenticated(
                &source.endpoint,
                hash,
                LookupContext {
                    receipt_keys: trust.receipt_keys(),
                },
            )
            .map_err(|_| bad())?
            else {
                return Err(bad());
            };
            crate::read::abci_result::verify_native_receipt(result, &receipt).map_err(|_| bad())?;
            if receipt.height() != block_height
                || receipt.execution().index != index as u64
                || checkpoint_timestamp_ms(&block).ok() != Some(receipt.execution().timestamp)
            {
                return Err(bad());
            }
            let protocol = receipt.receipt().protocol().ok_or_else(bad)?;
            if protocol.from() == account && !expected.insert(hash) {
                return Err(bad());
            }
        }
    }
    if expected != actual {
        return Err(bad());
    }
    let coverage = VerifiedHistoryCoverage {
        actor: actor.clone(),
        checkpoint: checkpoint.block_hash,
        first,
        last,
        entries_digest: history_entries_digest(&entries),
        time_seconds: match window {
            PolicyHistoryWindow::Time { seconds, .. } => Some(seconds),
            _ => None,
        },
    };
    Ok(PolicyUsageHistory {
        entries,
        checkpoint,
        observed,
        first_covered: first,
        coverage,
    })
}

pub(crate) fn checkpoint_timestamp_ms(block: &ServedCheckpoint) -> Result<u64, HistoryReadError> {
    u64::try_from(block.header.time.seconds)
        .ok()
        .and_then(|s| s.checked_mul(1000))
        .and_then(|ms| ms.checked_add(u64::from(block.header.time.nanos / 1_000_000)))
        .ok_or(HistoryReadError::ItemUnproven)
}
