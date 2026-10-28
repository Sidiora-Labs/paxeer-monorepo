use super::{
    protected, Config, CORRELATION, LNI_FRAME_BYTES, MAX_LNI_CONNECTIONS, PROTOCOL_VERSION,
};
use layerx_client::evidence::{
    checkpoint as read_checkpoint, CheckpointSelector, EvidenceContext, VerifiedCheckpoint,
};
use layerx_client::handover::SequencerHistory;
use layerx_client::lni::schema::Version;
use layerx_client::lni::transport::{FrameTransport, Limits};
use layerx_paxeer_verifier::PaxeerCheckpointVerifier;
use layerx_platform_authority::{parse_replica_evidence, BatchEvidence, EvidenceRefusal};
use layerx_proof::inclusion::SequencerAuthorization;
use std::env;
use std::path::Path;
use std::sync::atomic::Ordering;
use std::sync::Mutex;
use std::time::Instant;

pub(super) struct Trust {
    history: Mutex<SequencerHistory>,
}

pub(super) fn correlation(count: u64) -> Result<u64, ()> {
    CORRELATION
        .fetch_update(Ordering::AcqRel, Ordering::Acquire, |value| {
            value
                .checked_add(count)
                .filter(|_| value != 0 && count != 0)
        })
        .map_err(|_| ())
}

pub(super) fn limits(deadline: Instant) -> Result<Limits, ()> {
    let remaining = deadline.checked_duration_since(Instant::now()).ok_or(())?;
    if remaining.is_zero() {
        return Err(());
    }
    Ok(Limits {
        maximum_frame_bytes: LNI_FRAME_BYTES,
        maximum_connections: MAX_LNI_CONNECTIONS,
        maximum_streams: 1,
        maximum_queued_bytes: LNI_FRAME_BYTES,
        deadline: remaining,
    })
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
        PaxeerCheckpointVerifier::new(policy).map_err(|_| ())?;
        Ok(Self {
            history: Mutex::new(history),
        })
    }
}

pub(super) fn snapshot(
    config: &Config,
    _target: u64,
    claimed_key: [u8; 32],
    _deadline: Instant,
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

pub(super) fn checkpoint(
    config: &Config,
    transport: &mut dyn FrameTransport,
    batch: u64,
    version: Version,
    history: Option<&SequencerHistory>,
) -> Result<VerifiedCheckpoint, ()> {
    let key = match history {
        Some(history) => history
            .authorization_for_batch(batch)
            .map_err(|_| ())?
            .public_key(),
        None => config.sequencer_public_key,
    };
    let checkpoint = read_checkpoint(
        transport,
        CheckpointSelector::Batch(batch),
        EvidenceContext {
            interface_version: version,
            correlation_id: correlation(1)?,
            expected_protocol_version: PROTOCOL_VERSION,
            expected_network_id: config.protocol_network_id,
            handshake_sequencer_key: key,
        },
    )
    .map_err(|_| ())?;
    if let Some(history) = history {
        let header =
            layerx_client::batch::lookup_untrusted(transport, version, batch, correlation(1)?)
                .map_err(|_| ())?;
        history
            .verify_header(header.canonical_bytes(), header.signature())
            .map_err(|_| ())?;
        if header.canonical_bytes() != checkpoint.canonical_header() {
            return Err(());
        }
    }
    Ok(checkpoint)
}

#[cfg(test)]
#[path = "../tests/support/handover_trust.rs"]
mod tests;
