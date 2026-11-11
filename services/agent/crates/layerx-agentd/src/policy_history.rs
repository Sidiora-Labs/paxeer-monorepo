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
    /// # Errors
    /// Returns an error if the cumulative usage evidence cannot be authenticated against its
    /// committed source.
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
    /// # Errors
    /// Returns an error if committed time or usage evidence fails observation, binding or window
    /// validation.
    pub fn time_window(
        &self,
        actor: &Did,
        seconds: u64,
    ) -> Result<AuthenticatedTimeWindowUse, crate::protocol_evidence::TimeWindowUseError> {
        HistoryWindowBatch::from_committed_history(self.entries.clone(), self.observed.clone())
            .authenticate_history(actor, seconds, self.first_covered, &self.coverage)
    }
}

/// Both transports are explicit. The native Comet endpoint owns `block_results` and commit.
/// # Errors
/// Returns an error if the history range, cursor or page is invalid or oversized, the node
/// read fails, or an item lacks verified evidence.
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
    let page = source.page(
        &format!("0x{}", layerx_programs::hex::encode(&address)),
        None,
        limits,
    )?;
    let height = source
        .endpoint
        .block_number()
        .map_err(HistoryReadError::Sequencer)?;
    crate::reject_if(page.height != height, bad)?;
    let checkpoint = ServedCheckpoint::acquire_at_committed_block(
        &source.endpoint,
        height,
        None,
        &config.chain_id.to_string(),
    )
    .map_err(|_| bad())?;
    crate::reject_if(checkpoint.validators != trust.validator_set(), bad)?;
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
    let (first, last, lower_time) = policy_history_range(
        &source,
        &checkpoint,
        window,
        observed.observed_core_ms(),
        height,
    )?;
    let scan = PolicyHistoryScan {
        source: &source,
        checkpoint: &checkpoint,
        trust,
        actor,
        account,
        address,
        first,
        last,
        height,
        lower_time,
        limits,
    };
    let (entries, actual, mut bytes) = scan.read_pages(page)?;
    let expected = scan.expected_activities(&mut bytes)?;
    crate::reject_if(expected != actual, bad)?;
    let coverage = VerifiedHistoryCoverage {
        actor: actor.clone(),
        checkpoint: checkpoint.block_hash,
        first,
        last,
        entries_digest: history_entries_digest(&entries),
        time_seconds: match window {
            PolicyHistoryWindow::Time { seconds, .. } => Some(seconds),
            PolicyHistoryWindow::Sequence { .. } => None,
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

type PolicyHistoryPageEvidence = (Vec<HistoryActivityEvidence>, BTreeSet<[u8; 32]>, usize);
#[derive(Clone, Copy)]
struct PolicyHistoryScan<'a> {
    source: &'a PolicyHistorySource,
    checkpoint: &'a ServedCheckpoint,
    trust: &'a crate::export::ExportTrustSource,
    actor: &'a Did,
    account: [u8; 32],
    address: [u8; 20],
    first: u64,
    last: u64,
    height: u64,
    lower_time: Option<u64>,
    limits: HistoryLimits,
}

impl PolicyHistoryScan<'_> {
    fn read_pages(
        &self,
        mut page: crate::read::HistoryPage,
    ) -> Result<PolicyHistoryPageEvidence, HistoryReadError> {
        let bad = || HistoryReadError::ItemUnproven;
        let Self {
            source,
            checkpoint,
            trust,
            actor,
            account,
            address,
            first,
            last,
            height,
            lower_time,
            limits,
        } = *self;
        let mut bytes = 0;
        let mut entries = Vec::new();
        let mut actual = BTreeSet::new();
        let mut previous_cursor = None;
        let mut page_count = 0;
        loop {
            page_count += 1;
            crate::reject_if(page_count > 4096 || page.height != height, bad)?;
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
                    checkpoint,
                    trust.receipt_keys(),
                    &mut bytes,
                )?;
                let decoded =
                    layerx_wire::receipt::decode(&receipt.canonical).map_err(|_| bad())?;
                let protocol = decoded.protocol().ok_or_else(bad)?;
                crate::reject_if(
                    protocol.from() != account || !actual.insert(protocol.activity_id()),
                    bad,
                )?;
                if lower_time.is_none_or(|lower| receipt.timestamp > lower) {
                    entries.push(
                        HistoryActivityEvidence::from_committed_history(actor, account, receipt)
                            .map_err(|_| bad())?,
                    );
                }
                crate::reject_if(actual.len() > 4096, || HistoryReadError::Oversized)?;
            }
            if boundary || page.next_cursor.is_none() {
                break;
            }
            crate::reject_if(
                page.items.is_empty() || page.next_cursor == previous_cursor,
                bad,
            )?;
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

        Ok((entries, actual, bytes))
    }
    fn expected_activities(
        &self,
        bytes: &mut usize,
    ) -> Result<BTreeSet<[u8; 32]>, HistoryReadError> {
        let bad = || HistoryReadError::ItemUnproven;
        let Self {
            source,
            checkpoint,
            trust,
            account,
            first,
            last,
            ..
        } = *self;
        // Every native result leaf is enumerated, including non-EVM and failed results.
        // Authentication of the entire root proves the absence of omitted outcome leaves.
        let mut expected = BTreeSet::new();
        let mut items = 0usize;
        for block_height in first..=last {
            let block = source.committed_at(checkpoint, block_height)?;
            let next = source.successor(checkpoint, &block)?;
            let results = source
                .results
                .all_results(block_height)
                .map_err(|_| bad())?;
            items = items
                .checked_add(results.len())
                .filter(|n| *n <= 4096)
                .ok_or(HistoryReadError::Oversized)?;
            for result in &results {
                admit_bytes(bytes, result.len())?;
            }
            crate::reject_if(
                next.header.last_results_hash
                    != Some(layerx_programs::chain_proof::merkle::hash_from_byte_slices(
                        &results.iter().map(Vec::as_slice).collect::<Vec<_>>(),
                    )),
                bad,
            )?;
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
                crate::read::abci_result::verify_native_receipt(result, &receipt)
                    .map_err(|_| bad())?;
                crate::reject_if(
                    receipt.height() != block_height
                        || receipt.execution().index != index as u64
                        || checkpoint_timestamp_ms(&block).ok()
                            != Some(receipt.execution().timestamp),
                    bad,
                )?;
                let protocol = receipt.receipt().protocol().ok_or_else(bad)?;
                crate::reject_if(protocol.from() == account && !expected.insert(hash), || {
                    bad()
                })?;
            }
        }

        Ok(expected)
    }
}

fn policy_history_range(
    source: &PolicyHistorySource,
    checkpoint: &ServedCheckpoint,
    window: PolicyHistoryWindow,
    observed_at: u64,
    height: u64,
) -> Result<(u64, u64, Option<u64>), HistoryReadError> {
    let bad = || HistoryReadError::ItemUnproven;
    let lower_time = match window {
        PolicyHistoryWindow::Time { head, seconds }
            if head == height && seconds > 0 && seconds.checked_mul(1000).is_some() =>
        {
            Some(observed_at.saturating_sub(seconds * 1000))
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
                crate::reject_if(height - block.height >= 4095, || {
                    HistoryReadError::Oversized
                })?;
                block = source.committed_at(checkpoint, block.height - 1)?;
            }
            (block.height, height)
        }
    };
    crate::reject_if(height - first > 4096, || HistoryReadError::Oversized)?;

    Ok((first, last, lower_time))
}

pub(crate) fn checkpoint_timestamp_ms(block: &ServedCheckpoint) -> Result<u64, HistoryReadError> {
    u64::try_from(block.header.time.seconds)
        .ok()
        .and_then(|s| s.checked_mul(1000))
        .and_then(|ms| ms.checked_add(u64::from(block.header.time.nanos / 1_000_000)))
        .ok_or(HistoryReadError::ItemUnproven)
}
