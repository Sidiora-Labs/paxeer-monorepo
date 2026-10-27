// Publication and retrieval of committed block archives against in-memory
// Ethereum and Solana test networks, including stall and reorg scenarios.

use std::cell::RefCell;
use std::collections::BTreeMap;
use std::rc::Rc;

use layerx_mirror::committed::{CommittedBlock, CommittedHeader};
use layerx_mirror::{
    Archive, ArchiveCommitment, ArchiveData, ChainFailure, ChainPosition, CommittedCoordinate,
    CommittedFreshness, ConfigError, EthereumArchiveClient, EthereumArchiveWrite, EthereumConfig,
    EthereumObservation, EthereumSubmission, GenericPublisher, MirrorDegradation, MirrorState,
    NodeHead, PublicationId, RetrievalState, SolanaArchiveClient, SolanaArchiveWrite, SolanaConfig,
    SolanaObservation, SolanaSubmission,
};

const NETWORK_ID: u32 = 77;
const REQUIRED_CONFIRMATIONS: u64 = 3;
const REQUIRED_ROOTED_SLOTS: u64 = 32;

// -------------------------------------------------------------------------
// Archive fixtures built through the node boundary.
// -------------------------------------------------------------------------

fn ethereum_config() -> EthereumConfig {
    EthereumConfig {
        chain_id: 1,
        archive_contract: [0xAB; 20],
        required_confirmations: REQUIRED_CONFIRMATIONS,
    }
}

fn solana_config() -> SolanaConfig {
    SolanaConfig {
        genesis_hash: [0xCD; 32],
        archive_program: [0x01; 32],
        archive_account: [0x02; 32],
        required_rooted_slots: REQUIRED_ROOTED_SLOTS,
    }
}

fn build_archive(height: u64, node_head: NodeHead) -> Archive {
    let fixture: serde_json::Value =
        serde_json::from_str(include_str!("fixtures/committed_blocks.json"))
            .unwrap_or_else(|error| panic!("recorded committed block: {error}"));
    let mut header = CommittedHeader::from_block_json(&fixture["committedBlock"])
        .unwrap_or_else(|error| panic!("committed header: {error:?}"));
    header.height = height;
    let receipts = fixture["receipts"]
        .as_array()
        .unwrap_or_else(|| panic!("recorded receipts"))
        .iter()
        .map(|receipt| {
            let digits = receipt["receipt"]
                .as_str()
                .and_then(|text| text.strip_prefix("0x"))
                .unwrap_or_else(|| panic!("receipt hex"));
            (0..digits.len())
                .step_by(2)
                .map(|index| {
                    u8::from_str_radix(&digits[index..index + 2], 16)
                        .unwrap_or_else(|error| panic!("receipt hex: {error}"))
                })
                .collect()
        })
        .collect();
    Archive::from_committed(&CommittedBlock { header, receipts }, NETWORK_ID, node_head)
        .unwrap_or_else(|error| panic!("archive assembly failed: {error:?}"))
}

fn sealed_head(batch_number: u64) -> NodeHead {
    NodeHead {
        latest_sealed_batch: batch_number,
        latest_committed: None,
    }
}

// -------------------------------------------------------------------------
// In-memory Ethereum test network.
// -------------------------------------------------------------------------

#[derive(Clone)]
enum EthObserve {
    Pending,
    Canonical {
        block_number: u64,
        block_hash: [u8; 32],
        confirmations: u64,
    },
    Reorged {
        former_block_number: u64,
        former_block_hash: [u8; 32],
    },
    Rejected,
    Fail(ChainFailure),
}

#[derive(Clone)]
struct EthAppend {
    chain_id: u64,
    archive_contract: [u8; 20],
    commitment: ArchiveCommitment,
    network_id: u32,
    batch_number: u64,
    checkpoint: Option<CommittedCoordinate>,
    archive: Vec<u8>,
}

struct EthState {
    stored: BTreeMap<ArchiveCommitment, Vec<u8>>,
    appends: Vec<EthAppend>,
    append_failure: Option<ChainFailure>,
    observe: EthObserve,
    retrieve_failure: Option<ChainFailure>,
    withhold: bool,
    tamper: bool,
}

#[derive(Clone)]
struct Ethereum {
    state: Rc<RefCell<EthState>>,
}

impl Ethereum {
    fn new() -> Self {
        Self {
            state: Rc::new(RefCell::new(EthState {
                stored: BTreeMap::new(),
                appends: Vec::new(),
                append_failure: None,
                observe: EthObserve::Pending,
                retrieve_failure: None,
                withhold: false,
                tamper: false,
            })),
        }
    }

    fn set_observe(&self, observe: EthObserve) {
        self.state.borrow_mut().observe = observe;
    }

    fn confirmed(&self, confirmations: u64) {
        self.set_observe(EthObserve::Canonical {
            block_number: 4_200,
            block_hash: [0x11; 32],
            confirmations,
        });
    }

    fn fail_append(&self, failure: ChainFailure) {
        self.state.borrow_mut().append_failure = Some(failure);
    }

    fn fail_retrieve(&self, failure: ChainFailure) {
        self.state.borrow_mut().retrieve_failure = Some(failure);
    }

    fn withhold(&self) {
        self.state.borrow_mut().withhold = true;
    }

    fn append_count(&self) -> usize {
        self.state.borrow().appends.len()
    }

    fn last_append(&self) -> EthAppend {
        self.state
            .borrow()
            .appends
            .last()
            .cloned()
            .unwrap_or_else(|| panic!("no ethereum append recorded"))
    }
}

impl EthereumArchiveClient for Ethereum {
    fn append(
        &mut self,
        request: EthereumArchiveWrite<'_>,
    ) -> Result<EthereumSubmission, ChainFailure> {
        let mut state = self.state.borrow_mut();
        if let Some(failure) = state.append_failure.clone() {
            return Err(failure);
        }
        state.appends.push(EthAppend {
            chain_id: request.chain_id,
            archive_contract: request.archive_contract,
            commitment: request.commitment,
            network_id: request.network_id,
            batch_number: request.batch_number,
            checkpoint: request.checkpoint,
            archive: request.archive.to_vec(),
        });
        state
            .stored
            .insert(request.commitment, request.archive.to_vec());
        Ok(EthereumSubmission {
            transaction_hash: *request.commitment.as_bytes(),
        })
    }

    fn observe(
        &mut self,
        _transaction_hash: [u8; 32],
    ) -> Result<EthereumObservation, ChainFailure> {
        match self.state.borrow().observe.clone() {
            EthObserve::Pending => Ok(EthereumObservation::Pending),
            EthObserve::Canonical {
                block_number,
                block_hash,
                confirmations,
            } => Ok(EthereumObservation::Canonical {
                block_number,
                block_hash,
                confirmations,
            }),
            EthObserve::Reorged {
                former_block_number,
                former_block_hash,
            } => Ok(EthereumObservation::Reorged {
                former_block_number,
                former_block_hash,
            }),
            EthObserve::Rejected => Ok(EthereumObservation::Rejected),
            EthObserve::Fail(failure) => Err(failure),
        }
    }

    fn retrieve(
        &mut self,
        _archive_contract: [u8; 20],
        commitment: ArchiveCommitment,
    ) -> Result<Option<Vec<u8>>, ChainFailure> {
        let state = self.state.borrow();
        if let Some(failure) = state.retrieve_failure.clone() {
            return Err(failure);
        }
        if state.withhold {
            return Ok(None);
        }
        match state.stored.get(&commitment) {
            None => Ok(None),
            Some(bytes) => {
                let mut out = bytes.clone();
                if state.tamper {
                    if let Some(first) = out.first_mut() {
                        *first ^= 0x01;
                    }
                }
                Ok(Some(out))
            }
        }
    }
}

// -------------------------------------------------------------------------
// In-memory Solana test network.
// -------------------------------------------------------------------------

#[derive(Clone)]
enum SolObserve {
    Pending,
    Canonical {
        slot: u64,
        blockhash: [u8; 32],
        rooted_slots: u64,
    },
    Reorged {
        former_slot: u64,
        former_blockhash: [u8; 32],
    },
    Rejected,
}

#[derive(Clone)]
struct SolAppend {
    genesis_hash: [u8; 32],
    archive_program: [u8; 32],
    archive_account: [u8; 32],
    commitment: ArchiveCommitment,
    network_id: u32,
    batch_number: u64,
    checkpoint: Option<CommittedCoordinate>,
    archive: Vec<u8>,
}

struct SolState {
    stored: BTreeMap<ArchiveCommitment, Vec<u8>>,
    appends: Vec<SolAppend>,
    append_failure: Option<ChainFailure>,
    observe: SolObserve,
    retrieve_failure: Option<ChainFailure>,
    withhold: bool,
    tamper: bool,
}

#[derive(Clone)]
struct Solana {
    state: Rc<RefCell<SolState>>,
}

impl Solana {
    fn new() -> Self {
        Self {
            state: Rc::new(RefCell::new(SolState {
                stored: BTreeMap::new(),
                appends: Vec::new(),
                append_failure: None,
                observe: SolObserve::Pending,
                retrieve_failure: None,
                withhold: false,
                tamper: false,
            })),
        }
    }

    fn set_observe(&self, observe: SolObserve) {
        self.state.borrow_mut().observe = observe;
    }

    fn confirmed(&self, rooted_slots: u64) {
        self.set_observe(SolObserve::Canonical {
            slot: 9_000,
            blockhash: [0x22; 32],
            rooted_slots,
        });
    }

    fn tamper(&self) {
        self.state.borrow_mut().tamper = true;
    }

    fn append_count(&self) -> usize {
        self.state.borrow().appends.len()
    }

    fn last_append(&self) -> SolAppend {
        self.state
            .borrow()
            .appends
            .last()
            .cloned()
            .unwrap_or_else(|| panic!("no solana append recorded"))
    }
}

fn solana_signature(commitment: ArchiveCommitment) -> [u8; 64] {
    let mut signature = [0_u8; 64];
    signature[..32].copy_from_slice(commitment.as_bytes());
    signature[32..].copy_from_slice(commitment.as_bytes());
    signature
}

impl SolanaArchiveClient for Solana {
    fn append(
        &mut self,
        request: SolanaArchiveWrite<'_>,
    ) -> Result<SolanaSubmission, ChainFailure> {
        let mut state = self.state.borrow_mut();
        if let Some(failure) = state.append_failure.clone() {
            return Err(failure);
        }
        state.appends.push(SolAppend {
            genesis_hash: request.genesis_hash,
            archive_program: request.archive_program,
            archive_account: request.archive_account,
            commitment: request.commitment,
            network_id: request.network_id,
            batch_number: request.batch_number,
            checkpoint: request.checkpoint,
            archive: request.archive.to_vec(),
        });
        state
            .stored
            .insert(request.commitment, request.archive.to_vec());
        Ok(SolanaSubmission {
            signature: solana_signature(request.commitment),
        })
    }

    fn observe(&mut self, _signature: [u8; 64]) -> Result<SolanaObservation, ChainFailure> {
        match self.state.borrow().observe.clone() {
            SolObserve::Pending => Ok(SolanaObservation::Pending),
            SolObserve::Canonical {
                slot,
                blockhash,
                rooted_slots,
            } => Ok(SolanaObservation::Canonical {
                slot,
                blockhash,
                rooted_slots,
            }),
            SolObserve::Reorged {
                former_slot,
                former_blockhash,
            } => Ok(SolanaObservation::Reorged {
                former_slot,
                former_blockhash,
            }),
            SolObserve::Rejected => Ok(SolanaObservation::Rejected),
        }
    }

    fn retrieve(
        &mut self,
        _archive_program: [u8; 32],
        _archive_account: [u8; 32],
        commitment: ArchiveCommitment,
    ) -> Result<Option<Vec<u8>>, ChainFailure> {
        let state = self.state.borrow();
        if let Some(failure) = state.retrieve_failure.clone() {
            return Err(failure);
        }
        if state.withhold {
            return Ok(None);
        }
        match state.stored.get(&commitment) {
            None => Ok(None),
            Some(bytes) => {
                let mut out = bytes.clone();
                if state.tamper {
                    if let Some(first) = out.first_mut() {
                        *first ^= 0x01;
                    }
                }
                Ok(Some(out))
            }
        }
    }
}

fn new_publisher(ethereum: &Ethereum, solana: &Solana) -> GenericPublisher<Ethereum, Solana> {
    GenericPublisher::new(
        ethereum_config(),
        ethereum.clone(),
        solana_config(),
        solana.clone(),
    )
    .unwrap_or_else(|error| panic!("publisher construction failed: {error:?}"))
}

pub(crate) fn assert_committed_publication_on_both_test_networks() {
    let archive = build_archive(4, sealed_head(4));
    assert_eq!(&archive.bytes()[8..10], &4_u16.to_be_bytes());
    let expected = Some(CommittedCoordinate {
        height: 4,
        block_hash: archive.data().committed.block_hash,
    });
    assert_eq!(archive.data().committed_coordinate(), expected);
    assert!(!archive.data().committed.receipts.is_empty());
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut publisher = new_publisher(&ethereum, &solana);
    let report = publisher.publish(&archive);
    for state in [&report.ethereum, &report.solana] {
        let MirrorState::Confirmed {
            freshness,
            commitment,
            ..
        } = state
        else {
            panic!("publication did not confirm: {state:?}");
        };
        assert_eq!(*commitment, archive.commitment());
        assert_eq!(freshness.latest_committed_mirrored, expected);
    }
    assert_eq!(publisher.ethereum_cursor().latest_committed, expected);
    assert_eq!(publisher.solana_cursor().latest_committed, expected);
    let retrieved = publisher.retrieve(archive.commitment());
    for state in [retrieved.ethereum, retrieved.solana] {
        let RetrievalState::Retrieved(data) = state else {
            panic!("archive was not retrieved: {state:?}");
        };
        assert_eq!(*data, *archive.data());
        assert_eq!(data.committed_coordinate(), expected);
    }
}

// -------------------------------------------------------------------------
// Tests.
// -------------------------------------------------------------------------

#[test]
fn confirms_and_retrieves_the_same_archive_on_both_test_networks() {
    let archive = build_archive(1, sealed_head(1));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut publisher = new_publisher(&ethereum, &solana);

    let report = publisher.publish(&archive);
    assert_eq!(report.commitment, archive.commitment());

    let MirrorState::Confirmed {
        commitment: eth_commitment,
        publication: eth_publication,
        position: eth_position,
        freshness: eth_freshness,
    } = report.ethereum
    else {
        panic!("ethereum did not confirm: {:?}", report.ethereum);
    };
    assert_eq!(eth_commitment, archive.commitment());
    assert_eq!(
        eth_publication,
        PublicationId::Ethereum(*archive.commitment().as_bytes())
    );
    assert_eq!(
        eth_position,
        ChainPosition::Ethereum {
            block_number: 4_200,
            block_hash: [0x11; 32],
            confirmations: REQUIRED_CONFIRMATIONS,
        }
    );
    assert_eq!(eth_freshness.latest_batch_mirrored, Some(1));
    assert_eq!(eth_freshness.batch_lag, 0);
    assert_eq!(eth_freshness.node_latest_sealed_batch, 1);

    let MirrorState::Confirmed {
        publication: sol_publication,
        position: sol_position,
        freshness: sol_freshness,
        ..
    } = report.solana
    else {
        panic!("solana did not confirm: {:?}", report.solana);
    };
    assert_eq!(
        sol_publication,
        PublicationId::Solana(solana_signature(archive.commitment()))
    );
    assert_eq!(
        sol_position,
        ChainPosition::Solana {
            slot: 9_000,
            blockhash: [0x22; 32],
            rooted_slots: REQUIRED_ROOTED_SLOTS,
        }
    );
    assert_eq!(sol_freshness.latest_batch_mirrored, Some(1));
    assert_eq!(sol_freshness.batch_lag, 0);

    assert_eq!(publisher.ethereum_cursor().latest_batch, Some(1));
    assert_eq!(publisher.solana_cursor().latest_batch, Some(1));

    let retrieval = publisher.retrieve(archive.commitment());
    let RetrievalState::Retrieved(eth_archive) = retrieval.ethereum else {
        panic!("ethereum retrieval failed: {:?}", retrieval.ethereum);
    };
    assert_eq!(*eth_archive, *archive.data());
    let RetrievalState::Retrieved(sol_archive) = retrieval.solana else {
        panic!("solana retrieval failed: {:?}", retrieval.solana);
    };
    assert_eq!(*sol_archive, *archive.data());
}

#[test]
fn publishes_pure_archives_without_custody_semantics() {
    let archive = build_archive(2, sealed_head(2));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut publisher = new_publisher(&ethereum, &solana);
    let _ = publisher.publish(&archive);

    let eth_append = ethereum.last_append();
    assert_eq!(eth_append.chain_id, ethereum_config().chain_id);
    assert_eq!(
        eth_append.archive_contract,
        ethereum_config().archive_contract
    );
    assert_eq!(eth_append.commitment, archive.commitment());
    assert_eq!(eth_append.network_id, NETWORK_ID);
    assert_eq!(eth_append.batch_number, 2);
    assert_eq!(eth_append.checkpoint, archive.data().committed_coordinate());
    assert_eq!(eth_append.archive, archive.bytes());

    let sol_append = solana.last_append();
    assert_eq!(sol_append.genesis_hash, solana_config().genesis_hash);
    assert_eq!(sol_append.archive_program, solana_config().archive_program);
    assert_eq!(sol_append.archive_account, solana_config().archive_account);
    assert_eq!(sol_append.commitment, archive.commitment());
    assert_eq!(sol_append.network_id, NETWORK_ID);
    assert_eq!(sol_append.batch_number, 2);
    assert_eq!(sol_append.checkpoint, archive.data().committed_coordinate());
    assert_eq!(sol_append.archive, archive.bytes());

    let decoded = ArchiveData::decode(&eth_append.archive)
        .unwrap_or_else(|error| panic!("published archive did not decode: {error:?}"));
    assert_eq!(&decoded, archive.data());
    assert_eq!(decoded.network_id, NETWORK_ID);
    assert_eq!(decoded.batch_number, 2);
    assert_eq!(decoded.committed, archive.data().committed);
}

#[test]
fn a_stalled_mirror_reports_lag_while_the_other_confirms() {
    // Batch 5 confirms on both chains.
    let first = build_archive(5, sealed_head(5));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut publisher = new_publisher(&ethereum, &solana);
    let confirmed = publisher.publish(&first);
    assert!(matches!(confirmed.ethereum, MirrorState::Confirmed { .. }));
    assert!(matches!(confirmed.solana, MirrorState::Confirmed { .. }));

    // Batch 6 is sealed at the node; ethereum stalls in the mempool while
    // solana confirms. Neither outcome suppresses the other.
    let second = build_archive(6, sealed_head(6));
    ethereum.set_observe(EthObserve::Pending);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let report = publisher.publish(&second);

    let MirrorState::Pending {
        freshness: eth_freshness,
        ..
    } = report.ethereum
    else {
        panic!("ethereum should be pending: {:?}", report.ethereum);
    };
    // The stalled mirror states its lag honestly rather than presenting the
    // newer batch as mirrored.
    assert_eq!(eth_freshness.latest_batch_mirrored, Some(5));
    assert_eq!(eth_freshness.node_latest_sealed_batch, 6);
    assert_eq!(eth_freshness.batch_lag, 1);

    let MirrorState::Confirmed {
        freshness: sol_freshness,
        ..
    } = report.solana
    else {
        panic!("solana should confirm: {:?}", report.solana);
    };
    assert_eq!(sol_freshness.latest_batch_mirrored, Some(6));
    assert_eq!(sol_freshness.batch_lag, 0);

    assert_eq!(publisher.ethereum_cursor().latest_batch, Some(5));
    assert_eq!(publisher.solana_cursor().latest_batch, Some(6));
}

#[test]
fn a_pending_publication_is_idempotent_and_later_confirms() {
    let archive = build_archive(3, sealed_head(3));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.set_observe(EthObserve::Pending);
    solana.set_observe(SolObserve::Pending);
    let mut publisher = new_publisher(&ethereum, &solana);

    let first = publisher.publish(&archive);
    assert!(matches!(first.ethereum, MirrorState::Pending { .. }));
    let second = publisher.publish(&archive);
    assert!(matches!(second.ethereum, MirrorState::Pending { .. }));
    // A re-check of the same archive never re-appends the transaction.
    assert_eq!(ethereum.append_count(), 1);
    assert_eq!(solana.append_count(), 1);

    // Once the transaction reaches finality the same archive confirms without
    // a duplicate write.
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let third = publisher.publish(&archive);
    assert!(matches!(third.ethereum, MirrorState::Confirmed { .. }));
    assert!(matches!(third.solana, MirrorState::Confirmed { .. }));
    assert_eq!(ethereum.append_count(), 1);
    assert_eq!(solana.append_count(), 1);
}

#[test]
fn below_finality_confirmations_stay_pending() {
    let archive = build_archive(4, sealed_head(4));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS - 1);
    solana.confirmed(REQUIRED_ROOTED_SLOTS - 1);
    let mut publisher = new_publisher(&ethereum, &solana);
    let report = publisher.publish(&archive);
    assert!(matches!(report.ethereum, MirrorState::Pending { .. }));
    assert!(matches!(report.solana, MirrorState::Pending { .. }));
    // Nothing is confirmed yet, so no batch is mirrored.
    assert_eq!(publisher.ethereum_cursor().latest_batch, None);
    assert_eq!(publisher.solana_cursor().latest_batch, None);
}

#[test]
fn a_reorg_is_observable_and_the_cursor_retreats() {
    let archive = build_archive(4, sealed_head(4));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut publisher = new_publisher(&ethereum, &solana);
    let confirmed = publisher.publish(&archive);
    assert!(matches!(confirmed.ethereum, MirrorState::Confirmed { .. }));
    assert_eq!(publisher.ethereum_cursor().latest_batch, Some(4));

    // The chain that carried the archive reorgs it out; re-checking surfaces
    // the reorg and retreats the freshness cursor rather than hiding it.
    ethereum.set_observe(EthObserve::Reorged {
        former_block_number: 4_200,
        former_block_hash: [0x11; 32],
    });
    solana.set_observe(SolObserve::Reorged {
        former_slot: 9_000,
        former_blockhash: [0x22; 32],
    });
    let report = publisher.publish(&archive);

    let MirrorState::Reorged {
        former_position: eth_former,
        freshness: eth_freshness,
        ..
    } = report.ethereum
    else {
        panic!("ethereum should report a reorg: {:?}", report.ethereum);
    };
    assert_eq!(
        eth_former,
        ChainPosition::Ethereum {
            block_number: 4_200,
            block_hash: [0x11; 32],
            confirmations: 0,
        }
    );
    assert_eq!(eth_freshness.latest_batch_mirrored, None);
    assert_eq!(eth_freshness.batch_lag, 4);

    let MirrorState::Reorged {
        former_position: sol_former,
        ..
    } = report.solana
    else {
        panic!("solana should report a reorg: {:?}", report.solana);
    };
    assert_eq!(
        sol_former,
        ChainPosition::Solana {
            slot: 9_000,
            blockhash: [0x22; 32],
            rooted_slots: 0,
        }
    );

    assert_eq!(publisher.ethereum_cursor().latest_batch, None);
    assert_eq!(publisher.solana_cursor().latest_batch, None);
}

#[test]
fn a_rejected_transaction_degrades_typed() {
    let archive = build_archive(3, sealed_head(3));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.set_observe(EthObserve::Rejected);
    solana.set_observe(SolObserve::Rejected);
    let mut publisher = new_publisher(&ethereum, &solana);
    let report = publisher.publish(&archive);
    assert!(matches!(
        report.ethereum,
        MirrorState::Degraded {
            degradation: MirrorDegradation::TransactionRejected,
            ..
        }
    ));
    assert!(matches!(
        report.solana,
        MirrorState::Degraded {
            degradation: MirrorDegradation::TransactionRejected,
            ..
        }
    ));
}

#[test]
fn mirror_chain_unavailability_is_a_typed_degradation_not_a_block() {
    let archive = build_archive(6, sealed_head(6));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    // Ethereum RPC is unreachable; Solana is healthy.
    ethereum.fail_append(ChainFailure::Unavailable(
        "ethereum rpc unreachable".to_owned(),
    ));
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut publisher = new_publisher(&ethereum, &solana);

    // Publication returns a report - LayerX operation is never blocked by a
    // mirror chain being down.
    let report = publisher.publish(&archive);
    let MirrorState::Degraded {
        degradation: eth_degradation,
        freshness: eth_freshness,
        ..
    } = report.ethereum
    else {
        panic!("ethereum should degrade: {:?}", report.ethereum);
    };
    assert!(matches!(
        eth_degradation,
        MirrorDegradation::Chain(ChainFailure::Unavailable(_))
    ));
    assert_eq!(eth_freshness.latest_batch_mirrored, None);
    assert_eq!(eth_freshness.batch_lag, 6);
    // The healthy mirror is unaffected by the other chain's outage.
    assert!(matches!(report.solana, MirrorState::Confirmed { .. }));
    assert_eq!(publisher.solana_cursor().latest_batch, Some(6));

    // An observe-time outage degrades the same way without stopping progress.
    let observe_only = Ethereum::new();
    observe_only.set_observe(EthObserve::Fail(ChainFailure::RateLimited(
        "ethereum rate limited".to_owned(),
    )));
    let healthy_solana = Solana::new();
    healthy_solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut second = new_publisher(&observe_only, &healthy_solana);
    let report = second.publish(&archive);
    assert!(matches!(
        report.ethereum,
        MirrorState::Degraded {
            degradation: MirrorDegradation::Chain(ChainFailure::RateLimited(_)),
            ..
        }
    ));
    assert!(matches!(report.solana, MirrorState::Confirmed { .. }));
}

#[test]
fn confirmation_requires_the_archive_to_be_retrievable_intact() {
    // A canonical, finalised transaction whose stored bytes cannot be read
    // back must not be reported as confirmed.
    let archive = build_archive(2, sealed_head(2));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    ethereum.withhold();
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    solana.tamper();
    let mut publisher = new_publisher(&ethereum, &solana);
    let report = publisher.publish(&archive);
    assert!(matches!(
        report.ethereum,
        MirrorState::Degraded {
            degradation: MirrorDegradation::ArchiveNotRetrievable,
            ..
        }
    ));
    assert!(matches!(
        report.solana,
        MirrorState::Degraded {
            degradation: MirrorDegradation::RetrievedCommitmentMismatch,
            ..
        }
    ));
    assert_eq!(publisher.ethereum_cursor().latest_batch, None);
    assert_eq!(publisher.solana_cursor().latest_batch, None);
}

#[test]
fn retrieval_reports_missing_tampered_and_unavailable_mirrors() {
    let archive = build_archive(6, sealed_head(6));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut publisher = new_publisher(&ethereum, &solana);
    let confirmed = publisher.publish(&archive);
    assert!(matches!(confirmed.ethereum, MirrorState::Confirmed { .. }));

    // Ethereum can no longer serve the archive; Solana serves corrupted bytes.
    ethereum.withhold();
    solana.tamper();
    let retrieval = publisher.retrieve(archive.commitment());
    assert_eq!(retrieval.ethereum, RetrievalState::Missing);
    assert_eq!(
        retrieval.solana,
        RetrievalState::Degraded(MirrorDegradation::RetrievedCommitmentMismatch)
    );
    // A retrieval that no longer validates retreats the freshness cursor.
    assert_eq!(publisher.ethereum_cursor().latest_batch, None);
    assert_eq!(publisher.solana_cursor().latest_batch, None);

    // A transport failure on retrieval is surfaced as a typed chain failure.
    let offline = Ethereum::new();
    offline.confirmed(REQUIRED_CONFIRMATIONS);
    let healthy_solana = Solana::new();
    healthy_solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut second = new_publisher(&offline, &healthy_solana);
    let _ = second.publish(&archive);
    offline.fail_retrieve(ChainFailure::Unavailable(
        "ethereum archive node down".to_owned(),
    ));
    let retrieval = second.retrieve(archive.commitment());
    assert!(matches!(
        retrieval.ethereum,
        RetrievalState::Degraded(MirrorDegradation::Chain(ChainFailure::Unavailable(_)))
    ));
    assert!(matches!(retrieval.solana, RetrievalState::Retrieved(_)));
}

#[test]
fn freshness_states_committed_lag_honestly() {
    // The node has no committed block yet, so the mirror reports its absence.
    let no_committed = build_archive(5, sealed_head(5));
    let ethereum = Ethereum::new();
    let solana = Solana::new();
    ethereum.confirmed(REQUIRED_CONFIRMATIONS);
    solana.confirmed(REQUIRED_ROOTED_SLOTS);
    let mut publisher = new_publisher(&ethereum, &solana);
    let report = publisher.publish(&no_committed);
    assert_eq!(
        report.ethereum.freshness().committed,
        CommittedFreshness::NodeHasNoCommittedBlock
    );
    assert_eq!(
        report.ethereum.freshness().latest_committed_mirrored,
        no_committed.data().committed_coordinate()
    );

    // The node has committed a block but its archive has not been mirrored yet:
    // the mirror reports the target it must still reach.
    let coordinate = CommittedCoordinate {
        height: 5,
        block_hash: [0x7C; 32],
    };
    let awaiting = build_archive(
        5,
        NodeHead {
            latest_sealed_batch: 5,
            latest_committed: Some(coordinate),
        },
    );
    let fresh_eth = Ethereum::new();
    let fresh_sol = Solana::new();
    let mut second = new_publisher(&fresh_eth, &fresh_sol);
    let report = second.publish(&awaiting);
    assert_eq!(
        report.ethereum.freshness().committed,
        CommittedFreshness::NotYetMirrored { target_height: 5 }
    );
    assert_eq!(
        report.ethereum.freshness().node_latest_committed,
        Some(coordinate)
    );
    assert_eq!(report.ethereum.freshness().latest_committed_mirrored, None);
}

#[test]
fn invalid_chain_configuration_is_rejected_before_any_write() {
    let zero_ethereum = EthereumConfig {
        chain_id: 0,
        archive_contract: [0; 20],
        required_confirmations: 0,
    };
    assert_eq!(
        GenericPublisher::new(
            zero_ethereum,
            Ethereum::new(),
            solana_config(),
            Solana::new()
        )
        .err(),
        Some(ConfigError::Ethereum)
    );

    let zero_solana = SolanaConfig {
        genesis_hash: [0; 32],
        archive_program: [0; 32],
        archive_account: [0; 32],
        required_rooted_slots: 0,
    };
    assert_eq!(
        GenericPublisher::new(
            ethereum_config(),
            Ethereum::new(),
            zero_solana,
            Solana::new()
        )
        .err(),
        Some(ConfigError::Solana)
    );
}
