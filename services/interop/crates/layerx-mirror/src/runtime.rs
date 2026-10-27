//! Deployable independent-worker mirror publisher runtime.

use std::collections::BTreeMap;
use std::fs;
use std::io::{Read, Write};
use std::net::{IpAddr, SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::sync::{Arc, Mutex};
use std::thread;
use std::time::Duration;

use serde::{Deserialize, Serialize};

use crate::committed::{
    CommittedBlock, CommittedBlockSource, CommittedCoordinate, CommittedRefusal,
};
use crate::ethereum::{
    EthereumArchiveClient, EthereumError, EthereumProductionConfig, EthereumProgress,
};
use crate::rpc::{RpcCluster, RpcQuorumConfig};
use crate::signer::{RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SigningAlgorithm};
use crate::solana::{SolanaArchiveClient, SolanaError, SolanaProductionConfig, SolanaProgress};
use crate::store::{ArchiveSpool, PublicationPhase};
use crate::{Archive, ArchiveCommitment, ChainProgress, ProgressGate};

const MAX_CONFIG_BYTES: usize = 1024 * 1024;
const MAX_STATUS_REQUEST_BYTES: usize = 4096;

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct RuntimeConfig {
    pub state_directory: PathBuf,
    pub first_height: u64,
    pub poll_interval_ms: u64,
    /// Largest number of committed blocks a chain may trail while reporting ready.
    pub freshness_budget_blocks: u64,
    pub status_listen: SocketAddr,
    pub paxeer: crate::committed::CommittedSourceConfig,
    pub ethereum: EthereumFileConfig,
    /// Absent when the deployment mirrors to the EVM chain only. A present
    /// section is validated exactly as a required one.
    #[serde(default)]
    pub solana: Option<SolanaFileConfig>,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct EthereumFileConfig {
    pub rpc: RpcQuorumConfig,
    pub chain_id: u64,
    pub genesis_hash_hex: String,
    pub archive_contract_hex: String,
    pub archive_code_hash_hex: String,
    pub required_confirmations: u64,
    pub maximum_reorg_depth: u64,
    pub chunk_bytes: usize,
    pub transaction_gas_limit: u64,
    pub maximum_fee_per_gas: u128,
    pub maximum_priority_fee_per_gas: u128,
    pub signer: SignerFileConfig,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SolanaFileConfig {
    pub rpc: RpcQuorumConfig,
    pub genesis_hash_base58: String,
    pub archive_program_base58: String,
    pub upgradeable_loader_base58: String,
    pub program_data_account_base58: String,
    pub program_code_hash_hex: String,
    pub required_rooted_slots: u64,
    pub maximum_ancestry: u64,
    pub chunk_bytes: usize,
    pub signer: SignerFileConfig,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(deny_unknown_fields)]
pub struct SignerFileConfig {
    pub key_handle: String,
    pub public_key: String,
    pub timeout_ms: u64,
    pub transport: SignerTransportFileConfig,
}

#[derive(Clone, Debug, Deserialize)]
#[serde(tag = "kind", rename_all = "snake_case", deny_unknown_fields)]
pub enum SignerTransportFileConfig {
    Uds {
        socket: PathBuf,
    },
    MutualTls {
        endpoint: SocketAddr,
        server_name: String,
        trust_anchor: PathBuf,
        client_certificate: PathBuf,
        client_private_key: PathBuf,
    },
}

#[derive(Clone, Debug, Default, Serialize)]
struct ComponentStatus {
    ready: bool,
    latest_batch_mirrored: Option<u64>,
    latest_batch_observed: Option<u64>,
    latest_checkpoint_batch_mirrored: Option<u64>,
    latest_checkpoint_id_mirrored: Option<String>,
    latest_batch_verified: Option<u64>,
    latest_checkpoint_batch_verified: Option<u64>,
    checkpoint_batch_lag: Option<u64>,
    freshness_budget_batches: u64,
    freshness: Option<&'static str>,
    checkpoint_within_budget: bool,
    phase: Option<&'static str>,
    error_class: Option<&'static str>,
    reorgs_observed: u64,
}

#[derive(Clone, Debug, Default, Serialize)]
struct RuntimeStatus {
    source_chain_id: u64,
    native_network_id: u32,
    native_protocol_version: u16,
    service_version: String,
    node: ComponentStatus,
    ethereum: ComponentStatus,
    solana: Option<ComponentStatus>,
    committed_ready: bool,
    latest_committed_height: Option<u64>,
    latest_committed_block_hash: Option<String>,
    committed_refusal: Option<&'static str>,
    checkpoint_freshness_budget_batches: u64,
}

#[derive(Clone, Copy, Debug, Eq, PartialEq)]
pub enum RuntimeError {
    Arguments,
    Configuration,
    State,
    Status,
}

#[derive(Debug)]
enum SpoolRecoveryError {
    Store,
    Archive,
    BatchBeforeFirst,
    DuplicateBatch,
    BatchGap,
    SequenceExhausted,
}

/// Loads one bounded JSON configuration and starts node acquisition, two
/// independent durable chain workers and a redacted loopback status endpoint.
///
/// # Errors
/// Returns an error for invalid configuration, inaccessible durable state or status listener failure.
pub fn run(config_path: &Path) -> Result<(), RuntimeError> {
    let metadata = fs::metadata(config_path).map_err(|_| RuntimeError::Configuration)?;
    if metadata.len() == 0 || metadata.len() > MAX_CONFIG_BYTES as u64 {
        return Err(RuntimeError::Configuration);
    }
    let bytes = fs::read(config_path).map_err(|_| RuntimeError::Configuration)?;
    let config: RuntimeConfig =
        serde_json::from_slice(&bytes).map_err(|_| RuntimeError::Configuration)?;
    validate_runtime(&config)?;
    let spool = ArchiveSpool::open(config.state_directory.join("archives"))
        .map_err(|_| RuntimeError::State)?;
    let next_batch =
        recover_next_batch(&spool, config.first_height).map_err(|_| RuntimeError::State)?;
    let status = Arc::new(Mutex::new(RuntimeStatus {
        source_chain_id: config.paxeer.chain_id,
        service_version: env!("CARGO_PKG_VERSION").to_owned(),
        solana: config.solana.as_ref().map(|_| ComponentStatus::default()),
        checkpoint_freshness_budget_batches: config.freshness_budget_blocks,
        ..RuntimeStatus::default()
    }));
    let poll = Duration::from_millis(config.poll_interval_ms);

    spawn_status(config.status_listen, Arc::clone(&status))?;
    spawn_committed(
        config.clone(),
        spool.clone(),
        Arc::clone(&status),
        poll,
        next_batch,
    );
    spawn_ethereum(config.clone(), spool.clone(), Arc::clone(&status), poll);
    if let Some(solana) = config.solana.clone() {
        spawn_solana(config, solana, spool, status, poll);
    }
    loop {
        thread::park();
    }
}

fn spawn_committed(
    config: RuntimeConfig,
    spool: ArchiveSpool,
    status: Arc<Mutex<RuntimeStatus>>,
    poll: Duration,
    mut next_height: u64,
) {
    thread::spawn(move || {
        let Ok(network_id) = u32::try_from(config.paxeer.chain_id) else {
            update_status(&status, |value| {
                record_committed(value, Err(CommittedRefusal::ChainMismatch));
            });
            return;
        };
        let mut previous =
            match recover_committed_coordinate(&spool, config.first_height, next_height) {
                Ok(coordinate) => coordinate,
                Err(_) => {
                    update_status(&status, |value| {
                        value.node.error_class = Some("archive_spool");
                    });
                    return;
                }
            };
        let source = loop {
            match CommittedBlockSource::open(&config.paxeer) {
                Ok(source) => break source,
                Err(error) => {
                    update_status(&status, |value| {
                        record_committed(value, Err(error));
                    });
                    thread::sleep(poll);
                }
            }
        };
        loop {
            let acquired = match previous {
                Some(coordinate) => source.acquire_next(coordinate),
                None => source.acquire(next_height),
            };
            match acquired {
                Ok(block) => {
                    update_status(&status, |value| {
                        value.node.latest_batch_observed = Some(block.header.height);
                    });
                    match spool_committed(
                        &block,
                        network_id,
                        &spool,
                        &status,
                        next_height,
                        previous,
                    ) {
                        Ok(coordinate) => {
                            previous = Some(coordinate);
                            let Some(incremented) = coordinate.height.checked_add(1) else {
                                update_status(&status, |value| {
                                    value.node.ready = false;
                                    value.node.error_class = Some("block_sequence_exhausted");
                                });
                                return;
                            };
                            next_height = incremented;
                        }
                        Err(error) => update_status(&status, |value| {
                            value.node.ready = false;
                            value.node.error_class = Some(error);
                        }),
                    }
                }
                Err(error) => update_status(&status, |value| {
                    record_committed(value, Err(error));
                }),
            }
            thread::sleep(poll);
        }
    });
}

fn recover_committed_coordinate(
    spool: &ArchiveSpool,
    first_height: u64,
    next_height: u64,
) -> Result<Option<CommittedCoordinate>, SpoolRecoveryError> {
    if next_height == first_height {
        return Ok(None);
    }
    let archive = ordered_archives(spool)
        .pop()
        .ok_or(SpoolRecoveryError::Archive)?
        .map_err(|()| SpoolRecoveryError::Archive)?;
    if archive.data().batch_number.checked_add(1) != Some(next_height) {
        return Err(SpoolRecoveryError::BatchGap);
    }
    let block = archive
        .data()
        .committed
        .as_ref()
        .ok_or(SpoolRecoveryError::Archive)?;
    Ok(Some(CommittedCoordinate {
        height: archive.data().batch_number,
        block_hash: block.block_hash,
    }))
}

fn spool_committed(
    block: &CommittedBlock,
    network_id: u32,
    spool: &ArchiveSpool,
    status: &Arc<Mutex<RuntimeStatus>>,
    next_height: u64,
    previous: Option<CommittedCoordinate>,
) -> Result<CommittedCoordinate, &'static str> {
    if block.header.height != next_height {
        return Err("committed_not_committed");
    }
    if let Some(previous) = previous {
        if previous.height.checked_add(1) != Some(next_height) {
            return Err("committed_not_committed");
        }
        if block.header.parent_hash != previous.block_hash {
            return Err("committed_parent_mismatch");
        }
    }
    let coordinate = CommittedCoordinate {
        height: block.header.height,
        block_hash: block.header.block_hash,
    };
    let head = crate::NodeHead::new(coordinate.height, Some(coordinate));
    let archive =
        Archive::from_committed(block, network_id, head).map_err(|_| "archive_evidence")?;
    spool
        .put(archive.commitment(), archive.bytes(), head)
        .map_err(|_| "archive_spool")?;
    update_status(status, |value| {
        value.node.ready = true;
        value.node.latest_batch_observed = Some(coordinate.height);
        value.node.latest_batch_mirrored = Some(coordinate.height);
        value.node.error_class = None;
        value.node.phase = Some("spooled_committed_block");
        record_committed(value, Ok(coordinate));
    });
    Ok(coordinate)
}

fn spawn_ethereum(
    config: RuntimeConfig,
    spool: ArchiveSpool,
    status: Arc<Mutex<RuntimeStatus>>,
    poll: Duration,
) {
    thread::spawn(move || {
        let mut client = loop {
            match ethereum_client(&config) {
                Ok(value) => break value,
                Err(error) => {
                    update_status(&status, |value| {
                        value.ethereum.error_class = Some(ethereum_error_class(&error));
                    });
                    thread::sleep(poll);
                }
            }
        };
        let mut gate = ProgressGate::new();
        loop {
            let ordered = ordered_archives(&spool);
            let mut publication_error = None;
            for archive in &ordered {
                match archive {
                    Ok(archive) => match client.advance(archive) {
                        Ok(progress) => update_ethereum(&status, progress),
                        Err(error) => {
                            let error_class = ethereum_error_class(&error);
                            publication_error.get_or_insert(error_class);
                            update_status(&status, |value| {
                                value.ethereum.ready = false;
                                value.ethereum.error_class = Some(error_class);
                            });
                        }
                    },
                    Err(()) => {
                        publication_error.get_or_insert("archive_spool");
                        update_status(&status, |value| {
                            value.ethereum.ready = false;
                            value.ethereum.error_class = Some("archive_spool");
                        });
                    }
                }
            }
            let archives = ordered.into_iter().flatten().collect::<Vec<_>>();
            let progress = gate.acknowledge(client.cursor(), &archives, |commitment| {
                client.retrieve_finalized(commitment)
            });
            apply_ethereum_progress(&status, progress, publication_error);
            thread::sleep(poll);
        }
    });
}

fn spawn_solana(
    config: RuntimeConfig,
    solana: SolanaFileConfig,
    spool: ArchiveSpool,
    status: Arc<Mutex<RuntimeStatus>>,
    poll: Duration,
) {
    thread::spawn(move || {
        let mut client = loop {
            match solana_client(&config, &solana) {
                Ok(value) => break value,
                Err(error) => {
                    update_solana_status(&status, |value| {
                        value.error_class = Some(solana_error_class(&error));
                    });
                    thread::sleep(poll);
                }
            }
        };
        let mut gate = ProgressGate::new();
        loop {
            let ordered = ordered_archives(&spool);
            let mut publication_error = None;
            for archive in &ordered {
                match archive {
                    Ok(archive) => match client.advance(archive) {
                        Ok(progress) => update_solana(&status, progress),
                        Err(error) => {
                            let error_class = solana_error_class(&error);
                            publication_error.get_or_insert(error_class);
                            update_solana_status(&status, |value| {
                                value.ready = false;
                                value.error_class = Some(error_class);
                            });
                        }
                    },
                    Err(()) => {
                        publication_error.get_or_insert("archive_spool");
                        update_solana_status(&status, |value| {
                            value.ready = false;
                            value.error_class = Some("archive_spool");
                        });
                    }
                }
            }
            let archives = ordered.into_iter().flatten().collect::<Vec<_>>();
            let progress = gate.acknowledge(client.cursor(), &archives, |commitment| {
                client.retrieve_finalized(commitment)
            });
            apply_solana_progress(&status, progress, publication_error);
            thread::sleep(poll);
        }
    });
}

fn ordered_archives(spool: &ArchiveSpool) -> Vec<Result<Archive, ()>> {
    let Ok(commitments) = spool.commitments() else {
        return vec![Err(())];
    };
    let mut archives = commitments
        .into_iter()
        .map(|commitment| {
            let stored = spool.get(commitment).map_err(|_| ())?;
            Archive::from_spool(stored.bytes, stored.node_head).map_err(|_| ())
        })
        .collect::<Vec<_>>();
    archives.sort_by_key(|value| {
        value
            .as_ref()
            .map_or(u64::MAX, |archive| archive.data().batch_number)
    });
    archives
}

fn recover_next_batch(spool: &ArchiveSpool, first: u64) -> Result<u64, SpoolRecoveryError> {
    recover_spool(
        first,
        || spool.commitments().map_err(|_| SpoolRecoveryError::Store),
        |commitment| {
            let stored = spool
                .get(commitment)
                .map_err(|_| SpoolRecoveryError::Store)?;
            let archive = Archive::from_spool(stored.bytes, stored.node_head)
                .map_err(|_| SpoolRecoveryError::Archive)?;
            Ok(archive.data().batch_number)
        },
    )
}

fn recover_spool(
    first: u64,
    inventory: impl FnOnce() -> Result<Vec<ArchiveCommitment>, SpoolRecoveryError>,
    load_batch: impl FnMut(ArchiveCommitment) -> Result<u64, SpoolRecoveryError>,
) -> Result<u64, SpoolRecoveryError> {
    recover_batch_sequence(first, inventory()?, load_batch)
}

fn recover_batch_sequence(
    first: u64,
    commitments: impl IntoIterator<Item = ArchiveCommitment>,
    mut load_batch: impl FnMut(ArchiveCommitment) -> Result<u64, SpoolRecoveryError>,
) -> Result<u64, SpoolRecoveryError> {
    let mut batches = BTreeMap::new();
    for commitment in commitments {
        let batch = load_batch(commitment)?;
        if batch < first {
            return Err(SpoolRecoveryError::BatchBeforeFirst);
        }
        if batches.insert(batch, commitment).is_some() {
            return Err(SpoolRecoveryError::DuplicateBatch);
        }
    }
    let mut expected = first;
    for batch in batches.keys().copied() {
        if batch != expected {
            return Err(SpoolRecoveryError::BatchGap);
        }
        expected = expected
            .checked_add(1)
            .ok_or(SpoolRecoveryError::SequenceExhausted)?;
    }
    Ok(expected)
}

fn ethereum_client(config: &RuntimeConfig) -> Result<EthereumArchiveClient, EthereumError> {
    let signer = RemoteChainSigner::new(signer_config(
        &config.ethereum.signer,
        SigningAlgorithm::Secp256k1Recoverable,
    )?)?;
    EthereumArchiveClient::open(ethereum_production_config(config)?, signer)
}

fn solana_client(
    config: &RuntimeConfig,
    solana: &SolanaFileConfig,
) -> Result<SolanaArchiveClient, SolanaError> {
    let signer = RemoteChainSigner::new(signer_config(&solana.signer, SigningAlgorithm::Ed25519)?)?;
    SolanaArchiveClient::open(solana_production_config(config, solana)?, signer)
}

fn ethereum_production_config(
    config: &RuntimeConfig,
) -> Result<EthereumProductionConfig, EthereumError> {
    Ok(EthereumProductionConfig {
        rpc: config.ethereum.rpc.clone(),
        chain_id: config.ethereum.chain_id,
        genesis_hash: fixed_hex(&config.ethereum.genesis_hash_hex)
            .map_err(|_| EthereumError::Configuration)?,
        archive_contract: fixed_hex(&config.ethereum.archive_contract_hex)
            .map_err(|_| EthereumError::Configuration)?,
        archive_code_hash: fixed_hex(&config.ethereum.archive_code_hash_hex)
            .map_err(|_| EthereumError::Configuration)?,
        first_batch_number: config.first_height,
        required_confirmations: config.ethereum.required_confirmations,
        maximum_reorg_depth: config.ethereum.maximum_reorg_depth,
        chunk_bytes: config.ethereum.chunk_bytes,
        transaction_gas_limit: config.ethereum.transaction_gas_limit,
        maximum_fee_per_gas: config.ethereum.maximum_fee_per_gas,
        maximum_priority_fee_per_gas: config.ethereum.maximum_priority_fee_per_gas,
        journal_directory: config.state_directory.join("ethereum"),
    })
}

fn solana_production_config(
    config: &RuntimeConfig,
    solana: &SolanaFileConfig,
) -> Result<SolanaProductionConfig, SolanaError> {
    Ok(SolanaProductionConfig {
        rpc: solana.rpc.clone(),
        genesis_hash: fixed_base58(&solana.genesis_hash_base58)?,
        archive_program: fixed_base58(&solana.archive_program_base58)?,
        upgradeable_loader: fixed_base58(&solana.upgradeable_loader_base58)?,
        program_data_account: fixed_base58(&solana.program_data_account_base58)?,
        program_code_hash: fixed_hex(&solana.program_code_hash_hex)
            .map_err(|_| SolanaError::Configuration)?,
        first_batch_number: config.first_height,
        required_rooted_slots: solana.required_rooted_slots,
        maximum_ancestry: solana.maximum_ancestry,
        chunk_bytes: solana.chunk_bytes,
        journal_directory: config.state_directory.join("solana"),
    })
}

fn signer_config(
    config: &SignerFileConfig,
    algorithm: SigningAlgorithm,
) -> Result<RemoteSignerConfig, SignerErrorShim> {
    let public_key = match algorithm {
        SigningAlgorithm::Secp256k1Recoverable => decode_hex(&config.public_key)?,
        SigningAlgorithm::Ed25519 => base58_decode(&config.public_key, 32)?,
    };
    let endpoint = match &config.transport {
        SignerTransportFileConfig::Uds { socket } => {
            if !socket.is_absolute() {
                return Err(SignerErrorShim);
            }
            SignerEndpoint::Uds {
                socket: socket.clone(),
            }
        }
        SignerTransportFileConfig::MutualTls {
            endpoint,
            server_name,
            trust_anchor,
            client_certificate,
            client_private_key,
        } => {
            if !trust_anchor.is_absolute()
                || !client_certificate.is_absolute()
                || !client_private_key.is_absolute()
            {
                return Err(SignerErrorShim);
            }
            SignerEndpoint::MutualTls {
                endpoint: *endpoint,
                server_name: server_name.clone(),
                trust_anchor: trust_anchor.clone(),
                client_certificate: client_certificate.clone(),
                client_private_key: client_private_key.clone(),
            }
        }
    };
    Ok(RemoteSignerConfig {
        endpoint,
        algorithm,
        key_handle: config.key_handle.clone(),
        public_key,
        timeout: Duration::from_millis(config.timeout_ms),
    })
}

#[derive(Clone, Copy, Debug)]
struct SignerErrorShim;

impl From<SignerErrorShim> for EthereumError {
    fn from(_: SignerErrorShim) -> Self {
        Self::Configuration
    }
}

impl From<SignerErrorShim> for SolanaError {
    fn from(_: SignerErrorShim) -> Self {
        Self::Configuration
    }
}

fn spawn_status(
    address: SocketAddr,
    status: Arc<Mutex<RuntimeStatus>>,
) -> Result<(), RuntimeError> {
    if !address.ip().is_loopback() {
        return Err(RuntimeError::Configuration);
    }
    let listener = TcpListener::bind(address).map_err(|_| RuntimeError::Status)?;
    listener
        .set_nonblocking(false)
        .map_err(|_| RuntimeError::Status)?;
    thread::spawn(move || {
        for stream in listener.incoming().flatten() {
            let _ = serve_status(stream, &status);
        }
    });
    Ok(())
}

fn serve_status(
    mut stream: TcpStream,
    status: &Arc<Mutex<RuntimeStatus>>,
) -> Result<(), RuntimeError> {
    stream
        .set_read_timeout(Some(Duration::from_secs(2)))
        .and_then(|()| stream.set_write_timeout(Some(Duration::from_secs(2))))
        .map_err(|_| RuntimeError::Status)?;
    let mut request = [0_u8; MAX_STATUS_REQUEST_BYTES];
    let read = stream
        .read(&mut request)
        .map_err(|_| RuntimeError::Status)?;
    let request = std::str::from_utf8(&request[..read]).map_err(|_| RuntimeError::Status)?;
    let first = request.lines().next().ok_or(RuntimeError::Status)?;
    let snapshot = status.lock().map_err(|_| RuntimeError::Status)?.clone();
    let ready = snapshot.node.ready
        && snapshot.ethereum.ready
        && snapshot.ethereum.checkpoint_within_budget
        && snapshot
            .solana
            .as_ref()
            .is_none_or(|solana| solana.ready && solana.checkpoint_within_budget)
        && snapshot.committed_ready;
    let (code, reason, body) = match first {
        "GET /status HTTP/1.1" | "GET /status HTTP/1.0" => (
            200,
            "OK",
            serde_json::to_vec(&snapshot).map_err(|_| RuntimeError::Status)?,
        ),
        "GET /readyz HTTP/1.1" | "GET /readyz HTTP/1.0" if ready => {
            (200, "OK", b"{\"ready\":true}".to_vec())
        }
        "GET /readyz HTTP/1.1" | "GET /readyz HTTP/1.0" => {
            (503, "Service Unavailable", b"{\"ready\":false}".to_vec())
        }
        _ => (404, "Not Found", b"{\"error\":\"not_found\"}".to_vec()),
    };
    write!(
        stream,
        "HTTP/1.1 {code} {reason}\r\nContent-Type: application/json\r\nCache-Control: no-store\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
        body.len()
    )
    .and_then(|()| stream.write_all(&body))
    .and_then(|()| stream.flush())
    .map_err(|_| RuntimeError::Status)
}

/// A verified committed coordinate restores readiness; a refusal latches it off.
fn record_committed(
    status: &mut RuntimeStatus,
    committed: Result<CommittedCoordinate, CommittedRefusal>,
) {
    match committed {
        Ok(coordinate) => {
            status.node.ready = true;
            status.node.error_class = None;
            status.committed_ready = true;
            status.committed_refusal = None;
            status.latest_committed_height = Some(coordinate.height);
            status.latest_committed_block_hash = Some(hex_bytes(&coordinate.block_hash));
        }
        Err(refusal) => {
            status.node.ready = false;
            status.node.error_class = Some(refusal.code());
            status.committed_ready = false;
            status.committed_refusal = Some(refusal.code());
        }
    }
    let progress = VerifiedProgress::of(status);
    progress.apply(&mut status.ethereum);
    if let Some(solana) = status.solana.as_mut() {
        progress.apply(solana);
    }
}

/// Latest verified committed coordinates every chain is measured against.
#[derive(Clone, Copy)]
struct VerifiedProgress {
    batch: Option<u64>,
    checkpoint_batch: Option<u64>,
    budget: u64,
}

impl VerifiedProgress {
    fn of(status: &RuntimeStatus) -> Self {
        Self {
            batch: status.node.latest_batch_mirrored,
            checkpoint_batch: status.latest_committed_height,
            budget: status.checkpoint_freshness_budget_batches,
        }
    }

    fn apply(self, status: &mut ComponentStatus) {
        status.latest_batch_verified = self.batch;
        status.latest_checkpoint_batch_verified = self.checkpoint_batch;
        status.freshness_budget_batches = self.budget;
        status.checkpoint_batch_lag = self.checkpoint_batch.map(|target| {
            target.saturating_sub(status.latest_checkpoint_batch_mirrored.unwrap_or(0))
        });
        status.checkpoint_within_budget = status
            .checkpoint_batch_lag
            .is_none_or(|lag| lag <= self.budget);
        status.freshness = Some(match status.checkpoint_batch_lag {
            None => "no_verified_committed_block",
            Some(0) => "current",
            Some(lag) if lag <= self.budget => "within_budget",
            Some(_) => "beyond_budget",
        });
    }
}

fn update_ethereum(status: &Arc<Mutex<RuntimeStatus>>, progress: EthereumProgress) {
    update_status(status, |value| {
        if progress.phase == PublicationPhase::Reorged {
            value.ethereum.reorgs_observed = value.ethereum.reorgs_observed.saturating_add(1);
        }
        value.ethereum.ready = value.ethereum.ready
            && !matches!(
                progress.phase,
                PublicationPhase::PermanentRefusal | PublicationPhase::Reorged
            );
        VerifiedProgress::of(value).apply(&mut value.ethereum);
        value.ethereum.phase = Some(phase_name(progress.phase));
        value.ethereum.error_class = None;
    });
}

fn update_solana(status: &Arc<Mutex<RuntimeStatus>>, progress: SolanaProgress) {
    let Some(verified) = status.lock().ok().map(|value| VerifiedProgress::of(&value)) else {
        return;
    };
    update_solana_status(status, |value| {
        if progress.phase == PublicationPhase::Reorged {
            value.reorgs_observed = value.reorgs_observed.saturating_add(1);
        }
        value.ready = value.ready
            && !matches!(
                progress.phase,
                PublicationPhase::PermanentRefusal | PublicationPhase::Reorged
            );
        verified.apply(value);
        value.phase = Some(phase_name(progress.phase));
        value.error_class = None;
    });
}

/// Reports only gate-acknowledged mirrored coordinates. A reconcile refusal
/// holds the lane unready under its typed code.
fn apply_ethereum_progress(
    status: &Arc<Mutex<RuntimeStatus>>,
    progress: ChainProgress,
    publication_error: Option<&'static str>,
) {
    update_status(status, |value| {
        let verified = VerifiedProgress::of(value);
        apply_progress(&mut value.ethereum, verified, progress, publication_error);
    });
}

fn apply_solana_progress(
    status: &Arc<Mutex<RuntimeStatus>>,
    progress: ChainProgress,
    publication_error: Option<&'static str>,
) {
    update_status(status, |value| {
        let verified = VerifiedProgress::of(value);
        if let Some(solana) = value.solana.as_mut() {
            apply_progress(solana, verified, progress, publication_error);
        }
    });
}

fn apply_progress(
    status: &mut ComponentStatus,
    verified: VerifiedProgress,
    progress: ChainProgress,
    publication_error: Option<&'static str>,
) {
    status.latest_batch_mirrored = progress.mirrored.latest_batch;
    set_committed_status(status, progress.mirrored.latest_committed);
    verified.apply(status);
    if let Some(error_class) = progress
        .refusal
        .map(|refusal| refusal.code())
        .or(publication_error)
    {
        status.ready = false;
        status.error_class = Some(error_class);
    } else {
        status.ready = progress.mirrored.latest_committed.is_some()
            && !matches!(status.phase, Some("permanent_refusal" | "reorged"));
        status.error_class = None;
    }
}

/// Updates the Solana component of a runtime that has one. The Solana worker is
/// spawned only for a configured section, so the slot it reports into is always
/// present while that worker runs.
fn update_solana_status(
    status: &Arc<Mutex<RuntimeStatus>>,
    update: impl FnOnce(&mut ComponentStatus),
) {
    update_status(status, |value| {
        if let Some(solana) = value.solana.as_mut() {
            update(solana);
        }
    });
}

fn update_status(status: &Arc<Mutex<RuntimeStatus>>, update: impl FnOnce(&mut RuntimeStatus)) {
    if let Ok(mut status) = status.lock() {
        update(&mut status);
    }
}

fn set_committed_status(
    status: &mut ComponentStatus,
    committed: Option<crate::CommittedCoordinate>,
) {
    status.latest_checkpoint_batch_mirrored = committed.map(|value| value.height);
    status.latest_checkpoint_id_mirrored = committed.map(|value| hex_bytes(&value.block_hash));
}

fn hex_bytes(bytes: &[u8]) -> String {
    const DIGITS: &[u8; 16] = b"0123456789abcdef";
    let mut output = String::with_capacity(bytes.len().saturating_mul(2));
    for byte in bytes {
        output.push(char::from(DIGITS[usize::from(byte >> 4)]));
        output.push(char::from(DIGITS[usize::from(byte & 0x0f)]));
    }
    output
}

const fn phase_name(value: PublicationPhase) -> &'static str {
    match value {
        PublicationPhase::Prepared => "prepared",
        PublicationPhase::PreBroadcastFailure => "pre_broadcast_failure",
        PublicationPhase::Signed => "signed",
        PublicationPhase::BroadcastUnknown => "broadcast_unknown",
        PublicationPhase::Pending => "pending",
        PublicationPhase::Finalized => "finalized",
        PublicationPhase::RetrievedVerified => "retrieved_verified",
        PublicationPhase::PermanentRefusal => "permanent_refusal",
        PublicationPhase::Reorged => "reorged",
        PublicationPhase::BroadcastExpired => "broadcast_expired",
    }
}

fn ethereum_error_class(error: &EthereumError) -> &'static str {
    match error {
        EthereumError::Rpc(_) => "rpc",
        EthereumError::Signer(_) => "signer",
        EthereumError::Store(_) => "store",
        EthereumError::ChainIdentity | EthereumError::ContractIdentity => "target_identity",
        EthereumError::Retrieval => "retrieval",
        EthereumError::Reorg => "reorg",
        _ => "publication",
    }
}

fn solana_error_class(error: &SolanaError) -> &'static str {
    match error {
        SolanaError::Rpc(_) => "rpc",
        SolanaError::Signer(_) => "signer",
        SolanaError::Store(_) => "store",
        SolanaError::ClusterIdentity
        | SolanaError::ProgramIdentity
        | SolanaError::ProgramMutable => "target_identity",
        SolanaError::Retrieval => "retrieval",
        SolanaError::Reorg => "reorg",
        _ => "publication",
    }
}

fn validate_runtime(config: &RuntimeConfig) -> Result<(), RuntimeError> {
    let paxeer = &config.paxeer;
    if paxeer.chain_id == 0
        || u32::try_from(paxeer.chain_id).is_err()
        || fixed_hex::<32>(&paxeer.genesis_hash_hex).map_err(|_| RuntimeError::Configuration)?
            == [0; 32]
        || paxeer.receipt_keys.is_empty()
    {
        return Err(RuntimeError::Configuration);
    }
    for key in &paxeer.receipt_keys {
        if key.first_height > key.last_height
            || fixed_hex::<32>(&key.sequencer_id_hex).map_err(|_| RuntimeError::Configuration)?
                == [0; 32]
            || fixed_hex::<32>(&key.public_key_hex).map_err(|_| RuntimeError::Configuration)?
                == [0; 32]
        {
            return Err(RuntimeError::Configuration);
        }
    }
    RpcCluster::new(&paxeer.rpc).map_err(|_| RuntimeError::Configuration)?;
    if !config.state_directory.is_absolute()
        || config.first_height == 0
        || !(100..=60_000).contains(&config.poll_interval_ms)
        || !(1..=1_000_000).contains(&config.freshness_budget_blocks)
        || !matches!(config.status_listen.ip(), IpAddr::V4(_) | IpAddr::V6(_))
        || !config.status_listen.ip().is_loopback()
        || config.status_listen.port() == 0
    {
        return Err(RuntimeError::Configuration);
    }
    let ethereum = ethereum_production_config(config).map_err(|_| RuntimeError::Configuration)?;
    crate::ethereum::validate_config(&ethereum).map_err(|_| RuntimeError::Configuration)?;
    RpcCluster::new(&ethereum.rpc).map_err(|_| RuntimeError::Configuration)?;
    let ethereum_signer = signer_config(
        &config.ethereum.signer,
        SigningAlgorithm::Secp256k1Recoverable,
    )
    .map_err(|_| RuntimeError::Configuration)?;
    RemoteChainSigner::new(ethereum_signer).map_err(|_| RuntimeError::Configuration)?;

    if let Some(section) = &config.solana {
        let solana =
            solana_production_config(config, section).map_err(|_| RuntimeError::Configuration)?;
        crate::solana::validate_config(&solana).map_err(|_| RuntimeError::Configuration)?;
        RpcCluster::new(&solana.rpc).map_err(|_| RuntimeError::Configuration)?;
        let solana_signer = signer_config(&section.signer, SigningAlgorithm::Ed25519)
            .map_err(|_| RuntimeError::Configuration)?;
        RemoteChainSigner::new(solana_signer).map_err(|_| RuntimeError::Configuration)?;
    }
    Ok(())
}

fn fixed_hex<const N: usize>(value: &str) -> Result<[u8; N], SignerErrorShim> {
    decode_hex(value)?.try_into().map_err(|_| SignerErrorShim)
}

fn decode_hex(value: &str) -> Result<Vec<u8>, SignerErrorShim> {
    let digits = value.strip_prefix("0x").unwrap_or(value);
    if digits.is_empty() || !digits.len().is_multiple_of(2) {
        return Err(SignerErrorShim);
    }
    digits
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| {
            let high = hex_digit(pair[0]).ok_or(SignerErrorShim)?;
            let low = hex_digit(pair[1]).ok_or(SignerErrorShim)?;
            Ok(high << 4 | low)
        })
        .collect()
}

fn hex_digit(value: u8) -> Option<u8> {
    match value {
        b'0'..=b'9' => Some(value - b'0'),
        b'a'..=b'f' => Some(value - b'a' + 10),
        b'A'..=b'F' => Some(value - b'A' + 10),
        _ => None,
    }
}

fn fixed_base58(value: &str) -> Result<[u8; 32], SolanaError> {
    base58_decode(value, 32)?
        .try_into()
        .map_err(|_| SolanaError::Configuration)
}

fn base58_decode(value: &str, maximum: usize) -> Result<Vec<u8>, SignerErrorShim> {
    if value.is_empty() || value.len() > maximum.saturating_mul(2) {
        return Err(SignerErrorShim);
    }
    let alphabet = b"123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz";
    let mut bytes = vec![0_u8];
    for character in value.bytes() {
        let digit = alphabet
            .iter()
            .position(|candidate| *candidate == character)
            .ok_or(SignerErrorShim)?;
        let mut carry = u32::try_from(digit).map_err(|_| SignerErrorShim)?;
        for byte in bytes.iter_mut().rev() {
            let next = u32::from(*byte).saturating_mul(58).saturating_add(carry);
            *byte = u8::try_from(next & 0xff).map_err(|_| SignerErrorShim)?;
            carry = next >> 8;
        }
        while carry > 0 {
            bytes.insert(0, u8::try_from(carry & 0xff).map_err(|_| SignerErrorShim)?);
            carry >>= 8;
        }
        if bytes.len() > maximum {
            return Err(SignerErrorShim);
        }
    }
    let leading = value.bytes().take_while(|byte| *byte == b'1').count();
    let first_nonzero = bytes
        .iter()
        .position(|byte| *byte != 0)
        .unwrap_or(bytes.len());
    let mut decoded = vec![0; leading];
    decoded.extend_from_slice(&bytes[first_nonzero..]);
    if decoded.len() > maximum {
        return Err(SignerErrorShim);
    }
    Ok(decoded)
}

#[cfg(test)]
mod tests {
    use std::path::PathBuf;
    use std::sync::atomic::{AtomicU64, Ordering};
    use std::sync::{Arc, Mutex};

    use super::{
        apply_ethereum_progress, apply_solana_progress, hex_bytes, record_committed,
        recover_batch_sequence, recover_next_batch, recover_spool, ComponentStatus, RuntimeConfig,
        RuntimeStatus, SpoolRecoveryError,
    };
    use crate::committed::{CommittedCoordinate, CommittedRefusal};
    use crate::store::{ArchiveSpool, StoreError};
    use crate::{archive_commitment, ArchiveCommitment, ChainProgress, NodeHead};

    static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

    struct ScratchDirectory(PathBuf);

    impl ScratchDirectory {
        fn new(label: &str) -> Self {
            let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
            let path = std::env::temp_dir().join(format!(
                "layerx-mirror-runtime-{label}-{}-{sequence}",
                std::process::id()
            ));
            std::fs::create_dir(&path)
                .unwrap_or_else(|error| panic!("create scratch directory: {error}"));
            Self(path)
        }

        fn path(&self) -> &std::path::Path {
            &self.0
        }
    }

    impl Drop for ScratchDirectory {
        fn drop(&mut self) {
            if self.0.is_dir() {
                let _ = std::fs::remove_dir_all(&self.0);
            } else {
                let _ = std::fs::remove_file(&self.0);
            }
        }
    }

    fn commitment(byte: u8) -> ArchiveCommitment {
        ArchiveCommitment::from_bytes([byte; 32])
    }

    const ETHEREUM_SECTION: &str = r#"
        "ethereum": {
          "rpc": {
            "endpoints": [
              {"url":"https://ethereum-a.invalid/rpc","ca_certificate_der":"/run/secrets/a.der","bearer_token_file":"/run/secrets/a.token","independent_backend":"ethereum-a"},
              {"url":"https://ethereum-b.invalid/rpc","ca_certificate_der":"/run/secrets/b.der","bearer_token_file":"/run/secrets/b.token","independent_backend":"ethereum-b"}
            ],
            "quorum": 2,
            "connect_timeout_ms": 3000,
            "request_timeout_ms": 15000,
            "maximum_response_bytes": 16777216
          },
          "chain_id": 125,
          "genesis_hash_hex": "0000000000000000000000000000000000000000000000000000000000000001",
          "archive_contract_hex": "0000000000000000000000000000000000000001",
          "archive_code_hash_hex": "0000000000000000000000000000000000000000000000000000000000000001",
          "required_confirmations": 12,
          "maximum_reorg_depth": 256,
          "chunk_bytes": 24576,
          "transaction_gas_limit": 30000000,
          "maximum_fee_per_gas": 100000000000,
          "maximum_priority_fee_per_gas": 5000000000,
          "signer": {
            "key_handle": "mirror/ethereum/beta",
            "public_key": "0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798",
            "timeout_ms": 5000,
            "transport": {"kind":"uds","socket":"/run/mirror-signer/signer.sock"}
          }
        }"#;

    const SOLANA_SECTION: &str = r#"
        "solana": {
          "rpc": {
            "endpoints": [
              {"url":"https://solana-a.invalid/rpc","ca_certificate_der":"/run/secrets/a.der","bearer_token_file":"/run/secrets/a.token","independent_backend":"solana-a"},
              {"url":"https://solana-b.invalid/rpc","ca_certificate_der":"/run/secrets/b.der","bearer_token_file":"/run/secrets/b.token","independent_backend":"solana-b"}
            ],
            "quorum": 2,
            "connect_timeout_ms": 3000,
            "request_timeout_ms": 15000,
            "maximum_response_bytes": 16777216
          },
          "genesis_hash_base58": "11111111111111111111111111111112",
          "archive_program_base58": "11111111111111111111111111111113",
          "upgradeable_loader_base58": "BPFLoaderUpgradeab1e11111111111111111111111",
          "program_data_account_base58": "11111111111111111111111111111114",
          "program_code_hash_hex": "0000000000000000000000000000000000000000000000000000000000000001",
          "required_rooted_slots": 32,
          "maximum_ancestry": 4096,
          "chunk_bytes": 640,
          "signer": {
            "key_handle": "mirror/solana/beta",
            "public_key": "11111111111111111111111111111115",
            "timeout_ms": 5000,
            "transport": {"kind":"uds","socket":"/run/mirror-signer/signer.sock"}
          }
        }"#;

    /// The rendered publisher configuration, with the chain sections the caller
    /// names. This is the shape `services/interop/deploy/mirror/render-config.py` writes.
    fn rendered_configuration(sections: &[&str]) -> String {
        format!(
            r#"{{
              "state_directory": "/var/lib/layerx-mirror",
              "first_height": 1,
              "poll_interval_ms": 5000,
              "freshness_budget_blocks": 64,
              "status_listen": "127.0.0.1:9091",
              "paxeer": {{
                "rpc": {{
                  "endpoints": [
                    {{"url":"https://paxeer-a.invalid/rpc","ca_certificate_der":"/run/secrets/a.der","bearer_token_file":"/run/secrets/a.token","independent_backend":"paxeer-a"}},
                    {{"url":"https://paxeer-b.invalid/rpc","ca_certificate_der":"/run/secrets/b.der","bearer_token_file":"/run/secrets/b.token","independent_backend":"paxeer-b"}},
                    {{"url":"https://paxeer-c.invalid/rpc","ca_certificate_der":"/run/secrets/c.der","bearer_token_file":"/run/secrets/c.token","independent_backend":"paxeer-c"}}
                  ],
                  "quorum": 2,
                  "connect_timeout_ms": 3000,
                  "request_timeout_ms": 15000,
                  "maximum_response_bytes": 16777216
                }},
                "chain_id": 125,
                "genesis_hash_hex": "1111111111111111111111111111111111111111111111111111111111111111",
                "receipt_keys": [{{
                  "sequencer_id_hex": "6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b6b",
                  "public_key_hex": "8146640f02493af4fbc54fe33388e75dc2c937ae0b7727cc2b2afb1b75199a3e",
                  "first_height": 1000,
                  "last_height": 1999
                }}]
              }},
              {}
            }}"#,
            sections.join(",\n")
        )
    }

    fn configure_rpc_files(config: &mut RuntimeConfig, directory: &std::path::Path) {
        use openssl::asn1::Asn1Time;
        use openssl::hash::MessageDigest;
        use openssl::pkey::PKey;
        use openssl::rsa::Rsa;
        use openssl::x509::{X509NameBuilder, X509};
        use std::os::unix::fs::PermissionsExt;

        let key = PKey::from_rsa(Rsa::generate(2048).unwrap_or_else(|e| panic!("RSA: {e}")))
            .unwrap_or_else(|e| panic!("key: {e}"));
        let mut name = X509NameBuilder::new().unwrap_or_else(|e| panic!("name: {e}"));
        name.append_entry_by_text("CN", "Runtime configuration test")
            .unwrap_or_else(|e| panic!("name: {e}"));
        let name = name.build();
        let mut certificate = X509::builder().unwrap_or_else(|e| panic!("certificate: {e}"));
        certificate
            .set_version(2)
            .unwrap_or_else(|e| panic!("version: {e}"));
        let serial = openssl::bn::BigNum::from_u32(1)
            .and_then(|n| n.to_asn1_integer())
            .unwrap_or_else(|e| panic!("serial: {e}"));
        certificate
            .set_serial_number(&serial)
            .unwrap_or_else(|e| panic!("serial: {e}"));
        certificate
            .set_subject_name(&name)
            .unwrap_or_else(|e| panic!("subject: {e}"));
        certificate
            .set_issuer_name(&name)
            .unwrap_or_else(|e| panic!("issuer: {e}"));
        certificate
            .set_pubkey(&key)
            .unwrap_or_else(|e| panic!("public key: {e}"));
        let before = Asn1Time::days_from_now(0).unwrap_or_else(|e| panic!("time: {e}"));
        let after = Asn1Time::days_from_now(1).unwrap_or_else(|e| panic!("time: {e}"));
        certificate
            .set_not_before(&before)
            .unwrap_or_else(|e| panic!("time: {e}"));
        certificate
            .set_not_after(&after)
            .unwrap_or_else(|e| panic!("time: {e}"));
        certificate
            .sign(&key, MessageDigest::sha256())
            .unwrap_or_else(|e| panic!("sign: {e}"));
        let certificate_path = directory.join("ca.der");
        std::fs::write(
            &certificate_path,
            certificate
                .build()
                .to_der()
                .unwrap_or_else(|e| panic!("DER: {e}")),
        )
        .unwrap_or_else(|e| panic!("certificate file: {e}"));
        let token_path = directory.join("bearer");
        std::fs::write(&token_path, b"runtime-configuration-test")
            .unwrap_or_else(|e| panic!("token file: {e}"));
        std::fs::set_permissions(&token_path, std::fs::Permissions::from_mode(0o600))
            .unwrap_or_else(|e| panic!("token permissions: {e}"));
        for rpc in [&mut config.paxeer.rpc, &mut config.ethereum.rpc] {
            for endpoint in &mut rpc.endpoints {
                endpoint.ca_certificate_der = certificate_path.clone();
                endpoint.bearer_token_file = token_path.clone();
            }
        }
    }

    #[test]
    fn a_paxeer_section_builds_the_committed_source_configuration() {
        let rendered = rendered_configuration(&[ETHEREUM_SECTION]);
        let mut config: RuntimeConfig = serde_json::from_str(&rendered)
            .unwrap_or_else(|error| panic!("parse committed configuration: {error}"));
        let document: serde_json::Value = serde_json::from_str(&rendered)
            .unwrap_or_else(|error| panic!("configuration JSON: {error}"));
        let expected: crate::committed::CommittedSourceConfig =
            serde_json::from_value(document["paxeer"].clone())
                .unwrap_or_else(|error| panic!("committed source section: {error}"));
        assert_eq!(config.paxeer, expected);
        assert_eq!(config.first_height, 1);
        assert_eq!(config.freshness_budget_blocks, 64);
        assert_eq!(config.paxeer.chain_id, 125);
        assert_eq!(config.paxeer.receipt_keys.len(), 1);
        assert_eq!(config.paxeer.receipt_keys[0].first_height, 1000);
        assert_eq!(config.paxeer.receipt_keys[0].last_height, 1999);

        let scratch = ScratchDirectory::new("paxeer-configuration");
        configure_rpc_files(&mut config, scratch.path());
        assert_eq!(super::validate_runtime(&config), Ok(()));
        let mut invalid = config.clone();
        invalid.paxeer.chain_id = 0;
        assert_eq!(
            super::validate_runtime(&invalid),
            Err(super::RuntimeError::Configuration)
        );
        invalid = config.clone();
        invalid.paxeer.genesis_hash_hex = "not-a-block-hash".to_owned();
        assert_eq!(
            super::validate_runtime(&invalid),
            Err(super::RuntimeError::Configuration)
        );
        invalid = config.clone();
        invalid.paxeer.receipt_keys.clear();
        assert_eq!(
            super::validate_runtime(&invalid),
            Err(super::RuntimeError::Configuration)
        );
        invalid = config.clone();
        invalid.paxeer.receipt_keys[0].last_height = 999;
        assert_eq!(
            super::validate_runtime(&invalid),
            Err(super::RuntimeError::Configuration)
        );
        for (current, retired) in [
            ("paxeer", "node"),
            ("first_height", "first_batch_number"),
            (
                "freshness_budget_blocks",
                "checkpoint_freshness_budget_batches",
            ),
        ] {
            let retired = rendered.replace(&format!("\"{current}\":"), &format!("\"{retired}\":"));
            assert!(serde_json::from_str::<RuntimeConfig>(&retired).is_err());
        }
    }

    #[test]
    fn the_runtime_spools_committed_archives_in_height_order() {
        use crate::committed::{CommittedBlock, CommittedCoordinate, CommittedHeader};

        let fixture: serde_json::Value =
            serde_json::from_str(include_str!("../tests/fixtures/committed_blocks.json"))
                .unwrap_or_else(|e| panic!("recorded committed block: {e}"));
        let first = CommittedBlock {
            header: CommittedHeader::from_block_json(&fixture["committedBlock"])
                .unwrap_or_else(|e| panic!("recorded header: {e:?}")),
            receipts: fixture["receipts"]
                .as_array()
                .unwrap_or_else(|| panic!("recorded receipts"))
                .iter()
                .map(|receipt| {
                    super::decode_hex(
                        receipt["receipt"]
                            .as_str()
                            .unwrap_or_else(|| panic!("receipt bytes")),
                    )
                    .unwrap_or_else(|_| panic!("decode recorded receipt"))
                })
                .collect(),
        };
        let second = CommittedBlock {
            header: CommittedHeader {
                height: first.header.height + 1,
                block_hash: [0x66; 32],
                parent_hash: first.header.block_hash,
                app_hash: first.header.app_hash,
                timestamp: first.header.timestamp + 1,
                transactions: Vec::new(),
            },
            receipts: Vec::new(),
        };
        let scratch = ScratchDirectory::new("committed-height-order");
        let directory = scratch.path().join("archives");
        let spool = ArchiveSpool::open(&directory).unwrap_or_else(|e| panic!("open spool: {e:?}"));
        let status = Arc::new(Mutex::new(RuntimeStatus {
            checkpoint_freshness_budget_batches: 64,
            ..RuntimeStatus::default()
        }));
        let first_height = first.header.height;
        assert_eq!(
            recover_next_batch(&spool, first_height)
                .unwrap_or_else(|e| panic!("empty recovery: {e:?}")),
            first_height
        );
        assert_eq!(
            super::recover_committed_coordinate(&spool, first_height, first_height)
                .unwrap_or_else(|e| panic!("empty coordinate: {e:?}")),
            None
        );
        assert_eq!(
            super::spool_committed(&second, 125, &spool, &status, first_height, None),
            Err("committed_not_committed")
        );
        assert!(spool
            .commitments()
            .unwrap_or_else(|e| panic!("inventory: {e:?}"))
            .is_empty());
        let previous = super::spool_committed(&first, 125, &spool, &status, first_height, None)
            .unwrap_or_else(|e| panic!("spool first block: {e}"));
        assert_eq!(
            previous,
            CommittedCoordinate {
                height: first_height,
                block_hash: first.header.block_hash
            }
        );
        let next_height = recover_next_batch(&spool, first_height)
            .unwrap_or_else(|e| panic!("recover next height: {e:?}"));
        assert_eq!(next_height, second.header.height);
        drop(spool);

        let spool =
            ArchiveSpool::open(&directory).unwrap_or_else(|e| panic!("reopen spool: {e:?}"));
        let recovered = super::recover_committed_coordinate(&spool, first_height, next_height)
            .unwrap_or_else(|e| panic!("recover committed coordinate: {e:?}"));
        assert_eq!(recovered, Some(previous));
        let mut wrong_parent = second.clone();
        wrong_parent.header.parent_hash = [0x77; 32];
        assert_eq!(
            super::spool_committed(&wrong_parent, 125, &spool, &status, next_height, recovered),
            Err("committed_parent_mismatch")
        );
        super::spool_committed(&second, 125, &spool, &status, next_height, recovered)
            .unwrap_or_else(|e| panic!("spool next block after restart: {e}"));
        let archives = super::ordered_archives(&spool)
            .into_iter()
            .map(|archive| archive.unwrap_or_else(|()| panic!("load committed archive")))
            .collect::<Vec<_>>();
        assert_eq!(archives.len(), 2);
        for (archive, block) in archives.iter().zip([&first, &second]) {
            assert_eq!(archive.data().batch_number, block.header.height);
            assert_eq!(archive.data().network_id, 125);
            assert_eq!(&archive.bytes()[8..10], &4_u16.to_be_bytes());
            let committed = archive
                .data()
                .committed
                .as_ref()
                .unwrap_or_else(|| panic!("committed archive data"));
            assert_eq!(committed.block_hash, block.header.block_hash);
            assert_eq!(committed.receipts, block.receipts);
            assert_eq!(
                archive.node_head(),
                NodeHead::new(
                    block.header.height,
                    Some(crate::CommittedCoordinate {
                        height: block.header.height,
                        block_hash: block.header.block_hash,
                    })
                )
            );
        }
        assert_eq!(
            recover_next_batch(&spool, first_height)
                .unwrap_or_else(|e| panic!("recover completed prefix: {e:?}")),
            second.header.height + 1
        );
        let value = status
            .lock()
            .unwrap_or_else(|e| panic!("runtime status: {e}"));
        assert!(value.node.ready);
        assert!(value.committed_ready);
        assert_eq!(value.node.latest_batch_mirrored, Some(second.header.height));
        assert_eq!(value.latest_committed_height, Some(second.header.height));
        assert_eq!(
            value.latest_committed_block_hash,
            Some(hex_bytes(&second.header.block_hash))
        );
        assert_eq!(
            value.ethereum.latest_batch_verified,
            Some(second.header.height)
        );
    }

    #[test]
    fn a_configuration_without_a_solana_section_mirrors_to_the_evm_chain_only() {
        let both: RuntimeConfig =
            serde_json::from_str(&rendered_configuration(&[ETHEREUM_SECTION, SOLANA_SECTION]))
                .unwrap_or_else(|error| panic!("parse the dual-target configuration: {error}"));
        assert!(both.solana.is_some());

        let ethereum_only: RuntimeConfig =
            serde_json::from_str(&rendered_configuration(&[ETHEREUM_SECTION]))
                .unwrap_or_else(|error| panic!("parse the Ethereum-only configuration: {error}"));

        assert!(
            ethereum_only.solana.is_none(),
            "a configuration with no solana section mirrors to the EVM chain only"
        );
        assert_eq!(ethereum_only.ethereum.chain_id, both.ethereum.chain_id);
    }

    #[test]
    fn a_present_solana_section_is_still_complete_or_refused() {
        let incomplete = SOLANA_SECTION.replace(
            r#""program_data_account_base58": "11111111111111111111111111111114","#,
            "",
        );
        assert_ne!(incomplete, SOLANA_SECTION);

        assert!(
            serde_json::from_str::<RuntimeConfig>(&rendered_configuration(&[
                ETHEREUM_SECTION,
                &incomplete
            ]))
            .is_err(),
            "a solana section that is present must still carry every field"
        );
    }

    #[test]
    fn startup_refuses_unreadable_commitment_inventory() {
        let result = recover_spool(
            1,
            || Err(SpoolRecoveryError::Store),
            |_| unreachable!("failed inventory cannot load an object"),
        );

        assert!(matches!(result, Err(SpoolRecoveryError::Store)));
    }

    #[test]
    fn startup_refuses_a_second_spool_owner() {
        let scratch = ScratchDirectory::new("exclusive-owner");
        let directory = scratch.path().join("archives");
        let owner = ArchiveSpool::open(&directory)
            .unwrap_or_else(|error| panic!("open archive spool owner: {error:?}"));

        assert!(matches!(
            ArchiveSpool::open(&directory),
            Err(StoreError::Conflict)
        ));

        drop(owner);
        ArchiveSpool::open(&directory)
            .unwrap_or_else(|error| panic!("reopen released archive spool: {error:?}"));
    }

    #[test]
    fn startup_refuses_object_missing_after_inventory() {
        let scratch = ScratchDirectory::new("missing-object");
        let directory = scratch.path().join("archives");
        let spool = ArchiveSpool::open(&directory)
            .unwrap_or_else(|error| panic!("open archive spool: {error:?}"));
        let bytes = b"inventory object removed before recovery";
        let missing = archive_commitment(bytes);
        spool
            .put(missing, bytes, NodeHead::new(1, None))
            .unwrap_or_else(|error| panic!("spool missing archive fixture: {error:?}"));
        let commitments = spool
            .commitments()
            .unwrap_or_else(|error| panic!("list archive fixture: {error:?}"));
        let name = hex_bytes(missing.as_bytes());
        std::fs::remove_file(directory.join(format!("{name}.archive")))
            .unwrap_or_else(|error| panic!("remove inventoried archive: {error}"));
        let result = recover_batch_sequence(1, commitments, |value| {
            spool
                .get(value)
                .map(|_| 1)
                .map_err(|_| SpoolRecoveryError::Store)
        });

        assert!(matches!(result, Err(SpoolRecoveryError::Store)));
    }

    #[test]
    fn startup_refuses_corrupt_archive_bytes() {
        let scratch = ScratchDirectory::new("corrupt-archive");
        let spool = ArchiveSpool::open(scratch.path().join("archives"))
            .unwrap_or_else(|error| panic!("open archive spool: {error:?}"));
        let bytes = b"not a canonical LayerX mirror archive";
        spool
            .put(archive_commitment(bytes), bytes, NodeHead::new(1, None))
            .unwrap_or_else(|error| panic!("spool corrupt archive fixture: {error:?}"));

        assert!(matches!(
            recover_next_batch(&spool, 1),
            Err(SpoolRecoveryError::Archive)
        ));
    }

    #[test]
    fn startup_refuses_duplicate_batch_metadata() {
        let result = recover_batch_sequence(7, [commitment(1), commitment(2)], |_| Ok(7));

        assert!(matches!(result, Err(SpoolRecoveryError::DuplicateBatch)));
    }

    #[test]
    fn startup_refuses_gapped_batch_metadata() {
        let first = commitment(1);
        let third = commitment(3);
        let result = recover_batch_sequence(10, [first, third], |value| {
            Ok(if value == first { 10 } else { 12 })
        });

        assert!(matches!(result, Err(SpoolRecoveryError::BatchGap)));
    }

    #[test]
    fn clean_restart_resumes_after_the_contiguous_spool_prefix() {
        let first = commitment(1);
        let second = commitment(2);
        let result = recover_batch_sequence(41, [second, first], |value| {
            Ok(if value == first { 41 } else { 42 })
        });

        assert_eq!(
            result.unwrap_or_else(|error| panic!("recover clean spool: {error:?}")),
            43
        );
        assert_eq!(
            recover_batch_sequence(41, [], |_| unreachable!(
                "empty inventory has no loader calls"
            ))
            .unwrap_or_else(|error| panic!("recover empty spool: {error:?}")),
            41
        );
    }

    #[test]
    fn worker_degradation_survives_progress_application_per_lane() {
        let status = Arc::new(Mutex::new(RuntimeStatus {
            solana: Some(ComponentStatus::default()),
            ..RuntimeStatus::default()
        }));
        apply_ethereum_progress(&status, ChainProgress::default(), Some("rpc"));
        apply_solana_progress(&status, ChainProgress::default(), Some("archive_spool"));
        {
            let value = status
                .lock()
                .unwrap_or_else(|error| panic!("runtime status: {error}"));
            assert!(!value.ethereum.ready);
            assert_eq!(value.ethereum.error_class, Some("rpc"));
            assert_eq!(value.ethereum.latest_batch_mirrored, None);
            let solana = value
                .solana
                .as_ref()
                .unwrap_or_else(|| panic!("configured Solana lane"));
            assert!(!solana.ready);
            assert_eq!(solana.error_class, Some("archive_spool"));
            assert_eq!(solana.latest_checkpoint_batch_mirrored, None);
        }

        apply_ethereum_progress(&status, ChainProgress::default(), None);
        let value = status
            .lock()
            .unwrap_or_else(|error| panic!("runtime status: {error}"));
        assert!(!value.ethereum.ready);
        assert_eq!(value.ethereum.error_class, None);
        let solana = value
            .solana
            .as_ref()
            .unwrap_or_else(|| panic!("configured Solana lane"));
        assert!(!solana.ready);
        assert_eq!(solana.error_class, Some("archive_spool"));
    }

    #[test]
    fn checkpoint_refusal_latches_until_verified_evidence() {
        let mut status = RuntimeStatus {
            solana: Some(ComponentStatus::default()),
            checkpoint_freshness_budget_batches: 4,
            ..RuntimeStatus::default()
        };
        assert!(!status.committed_ready);
        for (refusal, code) in [
            (CommittedRefusal::Unavailable, "committed_unavailable"),
            (CommittedRefusal::ChainMismatch, "committed_chain_mismatch"),
            (
                CommittedRefusal::GenesisMismatch,
                "committed_genesis_mismatch",
            ),
            (CommittedRefusal::Divergence, "committed_divergence"),
            (CommittedRefusal::NotCommitted, "committed_not_committed"),
            (
                CommittedRefusal::ReceiptSignature,
                "committed_receipt_signature",
            ),
            (
                CommittedRefusal::ParentMismatch,
                "committed_parent_mismatch",
            ),
            (
                CommittedRefusal::HeightRegressed,
                "committed_height_regressed",
            ),
            (CommittedRefusal::Malformed, "committed_malformed"),
        ] {
            assert_eq!(refusal.code(), code);
            record_committed(&mut status, Err(refusal));
            assert!(!status.committed_ready);
            assert!(!status.node.ready);
            assert_eq!(status.committed_refusal, Some(code));
            assert_eq!(status.node.error_class, Some(code));
            assert_eq!(status.latest_committed_height, None);
            assert_eq!(status.latest_committed_block_hash, None);
            assert!(status.ethereum.checkpoint_within_budget);
            assert_eq!(
                status.ethereum.freshness,
                Some("no_verified_committed_block")
            );
            assert_eq!(status.ethereum.freshness_budget_batches, 4);

            let shared = Arc::new(Mutex::new(status));
            apply_ethereum_progress(&shared, ChainProgress::default(), None);
            apply_solana_progress(&shared, ChainProgress::default(), None);
            status = shared
                .lock()
                .unwrap_or_else(|error| panic!("runtime status: {error}"))
                .clone();
            assert!(!status.committed_ready);
            assert!(!status.node.ready);
            assert_eq!(status.committed_refusal, Some(code));
        }
        record_committed(
            &mut status,
            Ok(CommittedCoordinate {
                height: 9,
                block_hash: [9; 32],
            }),
        );
        assert!(status.committed_ready);
        assert!(status.node.ready);
        assert_eq!(status.committed_refusal, None);
        assert_eq!(status.node.error_class, None);
        assert_eq!(status.latest_committed_height, Some(9));
        assert_eq!(
            status.latest_committed_block_hash,
            Some(hex_bytes(&[9; 32]))
        );
        assert_eq!(status.ethereum.latest_checkpoint_batch_verified, Some(9));
        assert_eq!(status.ethereum.checkpoint_batch_lag, Some(9));
        assert!(!status.ethereum.checkpoint_within_budget);
        let solana = status
            .solana
            .as_ref()
            .unwrap_or_else(|| panic!("configured Solana lane"));
        assert_eq!(solana.latest_checkpoint_batch_verified, Some(9));
        assert_eq!(solana.checkpoint_batch_lag, Some(9));
        assert!(!solana.checkpoint_within_budget);
    }

    #[test]
    fn runtime_status_reports_the_committed_source_refusal_code() {
        let mut status = RuntimeStatus::default();
        record_committed(&mut status, Err(CommittedRefusal::ParentMismatch));
        let refused = serde_json::to_value(&status)
            .unwrap_or_else(|error| panic!("serialize refused status: {error}"));
        assert_eq!(refused["committed_refusal"], "committed_parent_mismatch");
        assert_eq!(refused["committed_ready"], false);
        assert_eq!(refused["node"]["ready"], false);
        assert_eq!(refused["node"]["error_class"], "committed_parent_mismatch");
        assert!(refused.get("checkpoint_refusal").is_none());
        assert!(refused["latest_committed_height"].is_null());
        assert!(refused["latest_committed_block_hash"].is_null());

        let coordinate = CommittedCoordinate {
            height: 9,
            block_hash: [0xab; 32],
        };
        record_committed(&mut status, Ok(coordinate));
        let verified = serde_json::to_value(&status)
            .unwrap_or_else(|error| panic!("serialize verified status: {error}"));
        assert_eq!(verified["committed_ready"], true);
        assert_eq!(verified["node"]["ready"], true);
        assert!(verified["committed_refusal"].is_null());
        assert!(verified["node"]["error_class"].is_null());
        assert_eq!(verified["latest_committed_height"], 9);
        assert_eq!(verified["latest_committed_block_hash"], "ab".repeat(32));
        assert_eq!(verified["ethereum"]["latest_checkpoint_batch_verified"], 9);
        assert!(verified.get("checkpoint_refusal").is_none());

        let shared = Arc::new(Mutex::new(status));
        super::update_status(&shared, |value| {
            record_committed(value, Err(CommittedRefusal::Unavailable));
        });
        apply_ethereum_progress(&shared, ChainProgress::default(), None);
        let latched = shared
            .lock()
            .unwrap_or_else(|error| panic!("runtime status: {error}"));
        assert!(!latched.committed_ready);
        assert!(!latched.node.ready);
        assert_eq!(latched.committed_refusal, Some("committed_unavailable"));
        assert_eq!(latched.latest_committed_height, Some(9));
        assert_eq!(latched.latest_committed_block_hash, Some("ab".repeat(32)));
    }
}
