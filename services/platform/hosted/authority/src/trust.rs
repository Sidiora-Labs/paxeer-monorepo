use super::{protected, Config, PROTOCOL_VERSION};
use layerx_client::handover::SequencerHistory;
use layerx_platform_authority::{parse_replica_evidence, BatchEvidence, EvidenceRefusal};
use layerx_proof::inclusion::SequencerAuthorization;
use std::env;
use std::path::Path;
use std::sync::Mutex;
use std::time::Duration;

pub(super) struct Trust {
    history: Mutex<SequencerHistory>,
}

pub(crate) struct CommittedCheckpoint {
    pub batch_number: u64,
    pub checkpoint_id: [u8; 32],
    pub header_digest: [u8; 32],
    pub last_sequence: u64,
    pub previous_state_root: [u8; 32],
    pub state_root: [u8; 32],
    pub timestamp_ms: u64,
    pub finalized_height: u64,
    pub checkpoint_call: Vec<u8>,
}

impl CommittedCheckpoint {
    pub(crate) fn binds_header(&self, canonical: &[u8]) -> bool {
        layerx_wire::receipt::decode_batch_header(canonical).is_ok_and(|header| {
            header.batch_number() == self.batch_number
                && header.last_sequence() == self.last_sequence
                && header.previous_state_root() == self.previous_state_root
                && header.resulting_state_root() == self.state_root
        })
    }
}

impl Trust {
    pub(super) fn load(
        network: u32,
        sequencer_id: [u8; 32],
        initial_key: [u8; 32],
    ) -> Result<Option<Self>, String> {
        let genesis = env::var_os("LAYERX_AUTHORITY_GENESIS_TRUST");
        let finality = env::var_os("LAYERX_AUTHORITY_HANDOVER_FINALITY");
        match (genesis, finality) {
            (None, None) => Ok(None),
            (Some(genesis), Some(finality)) => Self::from_paths(
                Path::new(&genesis),
                Path::new(&finality),
                network,
                sequencer_id,
                initial_key,
            )
            .map(Some)
            .map_err(|()| "invalid protected Authority genesis or finality policy".to_owned()),
            _ => Err(
                "Authority genesis trust and handover finality must both be configured".to_owned(),
            ),
        }
    }

    fn from_paths(
        genesis: &Path,
        finality: &Path,
        network: u32,
        sequencer_id: [u8; 32],
        initial_key: [u8; 32],
    ) -> Result<Self, ()> {
        let artifact = protected::read(
            genesis,
            u64::try_from(layerx_wire::handover::GENESIS_TRUST_MAX_BYTES).map_err(|_| ())?,
        )?;
        let policy_bytes = protected::read(finality, 1_048_576)?;
        let policy =
            layerx_client::handover::decode_finality_policy(&policy_bytes).map_err(|_| ())?;
        if policy.protocol_version != PROTOCOL_VERSION
            || policy.network_id != network
            || layerx_wire::handover::sequencer_id(&initial_key).map_err(|_| ())? != sequencer_id
        {
            return Err(());
        }
        let history = SequencerHistory::from_genesis_artifact(
            &artifact,
            network,
            policy.canonical_genesis_root,
            initial_key,
        )
        .map_err(|_| ())?;
        Ok(Self {
            history: Mutex::new(history),
        })
    }
}

pub(super) fn snapshot(
    config: &Config,
    claimed_key: [u8; 32],
) -> Result<Option<SequencerHistory>, ()> {
    if !config
        .chain_receipts
        .pinned_keys()
        .iter()
        .any(|key| key.public_key() == claimed_key)
    {
        return Err(());
    }
    current(config)
}

pub(super) fn current(config: &Config) -> Result<Option<SequencerHistory>, ()> {
    SequencerHistory::from_pinned_keys(
        config.protocol_network_id,
        config.chain_receipts.pinned_keys(),
    )
    .map(Some)
    .map_err(|_| ())
}

pub(super) fn replica(
    config: &Config,
    receipt: &[u8],
    document: &[u8],
) -> Result<(BatchEvidence, SequencerAuthorization), EvidenceRefusal> {
    let history = current(config).map_err(|()| EvidenceRefusal::SequencerKey)?;
    let decoded =
        layerx_wire::receipt::decode(receipt).map_err(|_| EvidenceRefusal::ReceiptDecode)?;
    let receipt = decoded.protocol().ok_or(EvidenceRefusal::ReceiptShape)?;
    let authorization = config
        .chain_receipts
        .pinned_keys()
        .iter()
        .find(|key| key.sequencer_id() == receipt.context_hash())
        .copied()
        .ok_or(EvidenceRefusal::SequencerKey)?;
    let evidence = parse_replica_evidence(document, config.replica_id, authorization.public_key())?;
    if let Some(history) = &history {
        history
            .verify_header(&evidence.header, &evidence.header_signature)
            .map_err(|_| EvidenceRefusal::SequencerKey)?;
    }
    Ok((evidence, authorization))
}

pub(super) fn authorization(
    config: &Config,
    canonical: &[u8],
    signature: &[u8; 64],
) -> Result<SequencerAuthorization, ()> {
    let Some(history) = current(config)? else {
        return Ok(config.authorization);
    };
    let header = history
        .verify_header(canonical, signature)
        .map_err(|_| ())?;
    config
        .chain_receipts
        .pinned_keys()
        .iter()
        .find(|key| key.sequencer_id() == header.header().sequencer_id())
        .copied()
        .ok_or(())
}

#[cfg(test)]
#[path = "../tests/support/handover_trust.rs"]
mod tests;

pub(super) fn chain_endpoint(
    url: &str,
    timeout: Duration,
) -> layerx_paxeer_verifier::EndpointConfig {
    layerx_paxeer_verifier::EndpointConfig {
        url: url.to_owned(),
        request_timeout: timeout,
        transport: layerx_paxeer_verifier::EndpointTransport::LocalEmulator,
        expected_chain_id: 125,
    }
}
