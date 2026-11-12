use super::Config;
use layerx_platform_authority::{parse_replica_evidence, BatchEvidence, EvidenceRefusal};
use layerx_proof::inclusion::SequencerAuthorization;
use sha2::Digest as _;

pub(crate) struct CommittedCheckpoint {
    pub checkpoint_id: [u8; 32],
    header_digest: [u8; 32],
    pub finality: crate::committed::CommittedHeader,
}

impl CommittedCheckpoint {
    pub(crate) fn new(
        canonical: &[u8],
        finality: crate::committed::CommittedHeader,
    ) -> Result<Self, ()> {
        Ok(Self {
            checkpoint_id: finality.block_hash,
            header_digest: layerx_wire::hash::batch_header_digest(canonical).map_err(|_| ())?,
            finality,
        })
    }

    pub(crate) fn binds_header(&self, canonical: &[u8]) -> bool {
        layerx_wire::hash::batch_header_digest(canonical)
            .is_ok_and(|digest| digest == self.header_digest)
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
            || key.sequencer_id() != <[u8; 32]>::from(sha2::Sha256::digest(key.public_key()))
            || layerx_wire::handover::sequencer_id(&key.public_key()).is_err()
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
    // Native receipt context is state-projection material, not a key identifier.
    // Select a pinned candidate from the replica envelope, then authenticate its
    // identity and range with the signed batch header before using it.
    let envelope: serde_json::Value =
        serde_json::from_slice(document).map_err(|_| EvidenceRefusal::ReplicaDocument)?;
    let public_key = envelope["sequencer_public_key"]
        .as_str()
        .and_then(|text| layerx_platform_authority::hex::decode32(text).ok())
        .ok_or(EvidenceRefusal::SequencerKey)?;
    snapshot(config, public_key).map_err(|()| EvidenceRefusal::SequencerKey)?;
    let authorization = keys
        .iter()
        .find(|key| key.public_key() == public_key)
        .copied()
        .ok_or(EvidenceRefusal::SequencerKey)?;
    let evidence = parse_replica_evidence(document, config.replica_id, authorization.public_key())?;
    let header = layerx_wire::receipt::decode_batch_header(&evidence.header)
        .map_err(|_| EvidenceRefusal::SequencerKey)?;
    if header.network_id() != config.protocol_network_id
        || (!(1..=11).contains(&receipt.module_id())
            && receipt.context_hash() != authorization.sequencer_id())
    {
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

pub(crate) use layerx_platform_authority::committed::EXPECTED_CHAIN_ID;
