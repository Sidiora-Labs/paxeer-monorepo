use super::Config;
use layerx_platform_authority::{parse_replica_evidence, BatchEvidence, EvidenceRefusal};
use layerx_proof::inclusion::SequencerAuthorization;
use std::time::Duration;

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
                && layerx_wire::hash::batch_header_digest(canonical)
                    .is_ok_and(|digest| digest == self.header_digest)
        })
    }
}

pub(super) fn snapshot(
    config: &Config,
    claimed_key: [u8; 32],
) -> Result<Vec<SequencerAuthorization>, ()> {
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

pub(super) fn current(config: &Config) -> Result<Vec<SequencerAuthorization>, ()> {
    let keys = config.chain_receipts.pinned_keys();
    if config.protocol_network_id == 0 || keys.is_empty() {
        return Err(());
    }
    for (index, key) in keys.iter().enumerate() {
        if key.sequencer_id() == [0; 32]
            || key.sequencer_id()
                != layerx_wire::handover::sequencer_id(&key.public_key()).map_err(|_| ())?
            || key.first_batch_number() > key.last_batch_number()
            || keys[..index]
                .iter()
                .any(|prior| prior.sequencer_id() == key.sequencer_id())
        {
            return Err(());
        }
    }
    Ok(keys.to_vec())
}

pub(super) fn replica(
    config: &Config,
    receipt: &[u8],
    document: &[u8],
) -> Result<(BatchEvidence, SequencerAuthorization), EvidenceRefusal> {
    let keys = current(config).map_err(|()| EvidenceRefusal::SequencerKey)?;
    let decoded =
        layerx_wire::receipt::decode(receipt).map_err(|_| EvidenceRefusal::ReceiptDecode)?;
    let receipt = decoded.protocol().ok_or(EvidenceRefusal::ReceiptShape)?;
    let authorization = keys
        .iter()
        .find(|key| key.sequencer_id() == receipt.context_hash())
        .copied()
        .ok_or(EvidenceRefusal::SequencerKey)?;
    let evidence = parse_replica_evidence(document, config.replica_id, authorization.public_key())?;
    let header = layerx_wire::receipt::decode_batch_header(&evidence.header)
        .map_err(|_| EvidenceRefusal::SequencerKey)?;
    if header.network_id() != config.protocol_network_id {
        return Err(EvidenceRefusal::SequencerKey);
    }
    layerx_proof::inclusion::verify_header(
        &evidence.header,
        &evidence.header_signature,
        &authorization,
    )
    .map_err(|_| EvidenceRefusal::SequencerKey)?;
    Ok((evidence, authorization))
}

pub(super) fn authorization(
    config: &Config,
    canonical: &[u8],
    signature: &[u8; 64],
) -> Result<SequencerAuthorization, ()> {
    let keys = current(config)?;
    let header = layerx_wire::receipt::decode_batch_header(canonical).map_err(|_| ())?;
    if header.network_id() != config.protocol_network_id {
        return Err(());
    }
    let key = keys
        .iter()
        .find(|key| {
            key.sequencer_id() == header.sequencer_id()
                && (key.first_batch_number()..=key.last_batch_number())
                    .contains(&header.batch_number())
        })
        .copied()
        .ok_or(())?;
    layerx_proof::inclusion::verify_header(canonical, signature, &key).map_err(|_| ())?;
    Ok(key)
}

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
