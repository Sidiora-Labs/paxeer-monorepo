//! The inbound and outbound loops driven by recorded JSON-RPC exchanges
//! (`tests/fixtures`), a real signer socket and a real journal on disk.

mod support;

use std::path::{Path, PathBuf};

use layerx_bridge_relayer::abi::{encode_bridge_in, encode_release, LAYERX_BRIDGE_PRECOMPILE};
use layerx_bridge_relayer::attestation::{
    to_attestor_signature, uint256_from_u64, InboundAttestation, OutboundAttestation,
};
use layerx_bridge_relayer::attestation_api::{deposit_answer, release_answer};
use layerx_bridge_relayer::cosign::CosignDirectory;
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{
    inbound_key, outbound_key, Completion, Entry, Journal, SubmissionStatus,
};
use layerx_bridge_relayer::relayer::{
    ChainLink, ChainSettings, GasPolicy, PaxeerLink, PaxeerSettings, Relayer, RelayerError,
    RelayerParts, StepReport,
};
use layerx_bridge_relayer::rpc::RpcFault;
use layerx_bridge_relayer::signer::{
    Attestor, Submitter, ATTEST_INBOUND_DOMAIN, ATTEST_OUTBOUND_DOMAIN,
    ETHEREUM_TRANSACTION_DOMAIN, PAXEER_TRANSACTION_DOMAIN,
};
use layerx_crypto::evm_transaction::Eip1559Call;
use sha3::{Digest as _, Keccak256};
use support::{key, secret, work_directory, Recording, SignerServer};

const ATTESTOR_1: u8 = 0xa1;
const ATTESTOR_2: u8 = 0xa2;
const PAXEER_FEES_1: u8 = 0xb1;
const FEES_2: u8 = 0xb2;

const PAXEER: PaxeerSettings = PaxeerSettings {
    chain_id: 229,
    finality_depth: 2,
    start_block: 25,
    max_block_range: 10,
    gas: GasPolicy {
        gas_limit: 1_000_000,
        max_fee_per_gas: 100_000_000_000,
        max_priority_fee_per_gas: 2_000_000_000,
    },
};

const CHAIN: ChainSettings = ChainSettings {
    chain_id: 1,
    vault: [0x11; 20],
    finality_depth: 12,
    start_block: 95,
    max_block_range: 10,
    gas: GasPolicy {
        gas_limit: 400_000,
        max_fee_per_gas: 200_000_000_000,
        max_priority_fee_per_gas: 3_000_000_000,
    },
};

fn signer(name: &str) -> SignerServer {
    SignerServer::start(
        name,
        vec![
            (
                "attestor-1",
                key(ATTESTOR_1),
                vec![ATTEST_INBOUND_DOMAIN, ATTEST_OUTBOUND_DOMAIN],
            ),
            (
                "attestor-2",
                key(ATTESTOR_2),
                vec![ATTEST_INBOUND_DOMAIN, ATTEST_OUTBOUND_DOMAIN],
            ),
            (
                "paxeer-fees-1",
                key(PAXEER_FEES_1),
                vec![PAXEER_TRANSACTION_DOMAIN],
            ),
            (
                "paxeer-fees-2",
                key(FEES_2),
                vec![PAXEER_TRANSACTION_DOMAIN],
            ),
            (
                "ethereum-fees",
                key(FEES_2),
                vec![ETHEREUM_TRANSACTION_DOMAIN],
            ),
        ],
    )
}

struct Instance<'a> {
    attestor: (&'a str, u8),
    paxeer_fees: (&'a str, u8),
    journal: PathBuf,
    cosign: Option<PathBuf>,
}

fn start(recording: &Recording, signer: &SignerServer, instance: &Instance<'_>) -> Relayer {
    let parts = RelayerParts {
        attestor: Attestor::new(signer.remote(instance.attestor.0, &key(instance.attestor.1)))
            .unwrap_or_else(|error| panic!("attestor: {error:?}")),
        paxeer: PaxeerLink {
            receipt_keys: support::receipt_keys(),
            settings: PAXEER,
            rpc: recording.endpoint("paxeer"),
            submitter: Submitter::paxeer(
                signer.remote(instance.paxeer_fees.0, &key(instance.paxeer_fees.1)),
            )
            .unwrap_or_else(|error| panic!("paxeer submitter: {error:?}")),
        },
        chains: vec![ChainLink {
            settings: CHAIN,
            rpc: recording.endpoint("ethereum-1"),
            submitter: Submitter::ethereum(signer.remote("ethereum-fees", &key(FEES_2)))
                .unwrap_or_else(|error| panic!("ethereum submitter: {error:?}")),
        }],
        journal: Journal::open(&instance.journal)
            .unwrap_or_else(|error| panic!("journal: {error}")),
        cosign: instance.cosign.clone().map(CosignDirectory::new),
        max_submissions: 3,
    };
    Relayer::new(parts).unwrap_or_else(|error| panic!("relayer startup: {error}"))
}

fn padded(address: u8) -> [u8; 32] {
    let mut word = [0_u8; 32];
    word[12..].fill(address);
    word
}

fn deposit(tx: u8, log_index: u64, recipient: [u8; 32], amount: u64) -> InboundAttestation {
    InboundAttestation {
        chain_id: 1,
        vault: [0x11; 20],
        tx_hash: [tx; 32],
        log_index,
        recipient,
        asset: [0x44; 20],
        amount: uint256_from_u64(amount),
    }
}

/// The attestor signature the verifiers expect, computed in-process from the
/// test key; the relayer obtains its own through the signer socket.
fn attest(attestor: u8, digest: &[u8; 32]) -> [u8; 65] {
    let (signature, recovery) = key(attestor)
        .sign_prehash_recoverable(digest)
        .unwrap_or_else(|error| panic!("attest: {error}"));
    let mut recoverable = [0_u8; 65];
    recoverable[..64].copy_from_slice(&signature.to_bytes());
    recoverable[64] = recovery.to_byte();
    to_attestor_signature(recoverable).unwrap_or_else(|error| panic!("attest: {error}"))
}

struct Fees {
    chain_id: u64,
    nonce: u64,
    priority: u128,
    max_fee: u128,
    gas_limit: u64,
}

fn transaction(fees: &Fees, to: [u8; 20], data: Vec<u8>, payer: u8) -> Vec<u8> {
    Eip1559Call {
        chain_id: fees.chain_id,
        nonce: fees.nonce,
        max_priority_fee_per_gas: fees.priority,
        max_fee_per_gas: fees.max_fee,
        gas_limit: fees.gas_limit,
        to,
        data,
    }
    .sign(&secret(payer))
    .unwrap_or_else(|error| panic!("reference transaction: {error}"))
}

/// Paxeer fees in the recordings: priority 1 gwei, base 2 gwei, so
/// `max_fee = 2 * 2 + 1` gwei.
fn paxeer_fees(nonce: u64) -> Fees {
    Fees {
        chain_id: PAXEER.chain_id,
        nonce,
        priority: 1_000_000_000,
        max_fee: 5_000_000_000,
        gas_limit: PAXEER.gas.gas_limit,
    }
}

fn bridge_in(nonce: u64, attestation: &InboundAttestation, signers: &[u8], payer: u8) -> Vec<u8> {
    let digest = attestation.digest();
    let signatures: Vec<[u8; 65]> = signers
        .iter()
        .map(|signer| attest(*signer, &digest))
        .collect();
    transaction(
        &paxeer_fees(nonce),
        LAYERX_BRIDGE_PRECOMPILE,
        encode_bridge_in(attestation, &signatures),
        payer,
    )
}

fn hash(raw: &[u8]) -> [u8; 32] {
    Keccak256::digest(raw).into()
}

fn completion(relayer: &Relayer, item: &str) -> Option<Completion> {
    relayer
        .journal()
        .state()
        .items
        .get(item)
        .and_then(|item| item.completion)
}

fn handle_requests(signer: &SignerServer, handle: &str) -> usize {
    signer
        .requests()
        .iter()
        .filter(|(name, _, _)| name == handle)
        .count()
}

fn assert_all_matched(recording: &Recording) {
    assert_eq!(recording.unmatched(), Vec::<String>::new());
}

fn journal_path(directory: &Path, name: &str) -> PathBuf {
    directory.join(format!("{name}.jsonl"))
}

#[test]
fn the_attestation_view_holds_every_item_after_a_tick() {
    let signer = signer("attestation-view");
    let directory = work_directory("attestation-view");
    let mut document: serde_json::Value =
        serde_json::from_str(include_str!("fixtures/inbound_threshold_one.json"))
            .unwrap_or_else(|error| panic!("inbound fixture: {error}"));
    let sequencer: serde_json::Value =
        serde_json::from_str(include_str!("fixtures/inbound_sequencer_receipt.json"))
            .unwrap_or_else(|error| panic!("sequencer fixture: {error}"));
    let restart = &mut document["endpoints"]["paxeer"]["restart"];
    restart[0] = sequencer["endpoints"]["paxeer"]["committed"][0].clone();
    let pending_hash = restart[1]["params"].clone();
    restart[1] = sequencer["endpoints"]["paxeer"]["unknown"][0].clone();
    restart[1]["params"] = pending_hash;
    let fixture = directory.join("attestation-recording.json");
    std::fs::write(
        &fixture,
        serde_json::to_vec(&document).unwrap_or_else(|error| panic!("combined recording: {error}")),
    )
    .unwrap_or_else(|error| panic!("write recording: {error}"));
    let recording = Recording::load(
        fixture
            .to_str()
            .unwrap_or_else(|| panic!("recording path is not UTF-8")),
    );
    let instance = Instance {
        attestor: ("attestor-1", ATTESTOR_1),
        paxeer_fees: ("paxeer-fees-1", PAXEER_FEES_1),
        journal: journal_path(&directory, "inbound"),
        cosign: None,
    };
    recording.set_phase("scan");
    let mut relayer = start(&recording, &signer, &instance);
    let report = relayer
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("scan: {error}"));
    assert_eq!(report.observed, 3);
    assert_eq!(report.submitted, 2);
    assert_eq!(report.refused, 1);

    recording.set_phase("restart");
    let tick = relayer.tick();
    assert_eq!(tick.len(), 2);
    assert_eq!(
        tick[0].1,
        Ok(StepReport {
            completed: 1,
            waiting: 1,
            ..StepReport::default()
        })
    );
    assert_eq!(tick[1].1, Ok(StepReport::default()));
    let view = relayer.attestation_view();
    assert_eq!(view.items, relayer.journal().state().items);
    assert_eq!(view.items.len(), 3);
    assert_eq!(view.thresholds.get(&CHAIN.chain_id), Some(&1));
    assert_eq!(view.signatures.len(), 3);
    for attestation in [
        deposit(0xaa, 3, padded(0x77), 1_000_000_000_000_000_000),
        deposit(0xcc, 7, padded(0x88), 5),
    ] {
        let digest = attestation.digest();
        assert_eq!(view.signatures[&digest], vec![attest(ATTESTOR_1, &digest)]);
        let answer = deposit_answer(
            &view,
            attestation.chain_id,
            &attestation.tx_hash,
            attestation.log_index,
        )
        .unwrap_or_else(|| panic!("missing deposit"));
        assert_eq!(answer["status"], "ready");
    }
    let refused = &view.items[&inbound_key(1, &[0xdd; 32], 0)];
    assert!(refused.refusal.is_some());
    assert_eq!(refused.signature, None);
    let refused_digest = refused
        .observation
        .inbound_attestation()
        .unwrap_or_else(|| panic!("missing refused deposit"))
        .digest();
    assert!(view.signatures[&refused_digest].is_empty());
    assert_all_matched(&recording);

    let recording = Recording::load("outbound_release.json");
    let instance = Instance {
        journal: journal_path(&directory, "outbound"),
        ..instance
    };
    let mut journal =
        Journal::open(&instance.journal).unwrap_or_else(|error| panic!("journal: {error}"));
    journal
        .append(&Entry::Cursor {
            stream: "in:1".to_owned(),
            next_block: 0x101,
        })
        .unwrap_or_else(|error| panic!("inbound cursor: {error}"));
    drop(journal);
    recording.set_phase("release");
    let mut relayer = start(&recording, &signer, &instance);
    assert_eq!(
        relayer
            .outbound_step()
            .unwrap_or_else(|error| panic!("release: {error}"))
            .submitted,
        1
    );
    recording.set_phase("released");
    let tick = relayer.tick();
    assert_eq!(tick.len(), 2);
    assert_eq!(tick[0].1, Ok(StepReport::default()));
    assert_eq!(
        tick[1].1,
        Ok(StepReport {
            completed: 1,
            ..StepReport::default()
        })
    );
    let view = relayer.attestation_view();
    assert_eq!(view.items, relayer.journal().state().items);
    assert_eq!(view.items.len(), 1);
    let item = &view.items[&outbound_key(1, &[0x22; 32], 7)];
    assert!(matches!(item.completion, Some(Completion::Included { .. })));
    let digest = item
        .observation
        .outbound_attestation()
        .unwrap_or_else(|| panic!("missing release"))
        .digest();
    assert_eq!(view.signatures[&digest], vec![attest(ATTESTOR_1, &digest)]);
    assert_eq!(view.thresholds.get(&CHAIN.chain_id), Some(&1));
    assert_eq!(
        release_answer(&view, 1, &[0x22; 32], 7)
            .unwrap_or_else(|| panic!("missing release answer"))["status"],
        "ready"
    );
    assert_all_matched(&recording);
}

#[test]
fn the_attestation_view_is_a_snapshot() {
    let recording = Recording::load("cosign_threshold_two.json");
    let signer = signer("attestation-snapshot");
    let directory = work_directory("attestation-snapshot");
    let cosign = directory.join("cosign");
    let one = Instance {
        attestor: ("attestor-1", ATTESTOR_1),
        paxeer_fees: ("paxeer-fees-1", PAXEER_FEES_1),
        journal: journal_path(&directory, "one"),
        cosign: Some(cosign.clone()),
    };
    let two = Instance {
        attestor: ("attestor-2", ATTESTOR_2),
        paxeer_fees: ("paxeer-fees-2", FEES_2),
        journal: journal_path(&directory, "two"),
        cosign: Some(cosign),
    };
    recording.set_phase("pending");
    let mut first = start(&recording, &signer, &one);
    let empty = first.attestation_view();
    let mut second = start(&recording, &signer, &two);
    let attestation = deposit(0xaa, 3, padded(0x77), 1_000_000_000_000_000_000);
    let digest = attestation.digest();
    let item = inbound_key(1, &[0xaa; 32], 3);
    assert_eq!(
        first
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("first: {error}"))
            .waiting,
        1
    );
    let before = first.attestation_view();
    assert_eq!(before.items, first.journal().state().items);
    assert_eq!(
        before.signatures[&digest],
        vec![attest(ATTESTOR_1, &digest)]
    );
    assert_eq!(before.thresholds.get(&CHAIN.chain_id), Some(&2));
    assert_eq!(before.items[&item].completion, None);
    assert_eq!(
        deposit_answer(&before, 1, &[0xaa; 32], 3).unwrap_or_else(|| panic!("missing deposit"))
            ["status"],
        "attesting"
    );
    let saved_items = before.items.clone();
    let saved_signatures = before.signatures.clone();
    let saved_thresholds = before.thresholds.clone();
    assert_eq!(
        second
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("second: {error}"))
            .submitted,
        1
    );
    recording.set_phase("bridged");
    let tick = first.tick();
    assert_eq!(tick.len(), 2);
    assert_eq!(
        tick[0].1,
        Ok(StepReport {
            completed: 1,
            ..StepReport::default()
        })
    );
    assert_eq!(tick[1].1, Ok(StepReport::default()));
    let after = first.attestation_view();
    assert_eq!(after.items, first.journal().state().items);
    assert_eq!(
        after.items[&item].completion,
        Some(Completion::AlreadyBridged)
    );
    assert_eq!(
        after.signatures[&digest],
        vec![attest(ATTESTOR_2, &digest), attest(ATTESTOR_1, &digest)]
    );
    assert_eq!(after.thresholds.get(&CHAIN.chain_id), Some(&2));
    assert_eq!(
        deposit_answer(&after, 1, &[0xaa; 32], 3).unwrap_or_else(|| panic!("missing deposit"))
            ["status"],
        "ready"
    );
    assert_eq!(before.items, saved_items);
    assert_eq!(before.signatures, saved_signatures);
    assert_eq!(before.thresholds, saved_thresholds);
    assert!(empty.items.is_empty());
    assert!(empty.signatures.is_empty());
    assert!(empty.thresholds.is_empty());
    assert_eq!(recording.sent().len(), 1);
    assert_all_matched(&recording);
}

/// Restart: the first transaction is final, the second is unknown to the node
/// and is rebroadcast byte for byte without being signed again. Returns every
/// transaction sent so far.
fn restart_and_rebroadcast_the_unknown_deposit(
    recording: &Recording,
    signer: &SignerServer,
    instance: &Instance<'_>,
) -> Vec<(String, Vec<u8>)> {
    recording.set_phase("restart");
    let mut relayer = start(recording, signer, instance);
    let report = relayer
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("restart: {error}"));
    assert_eq!(
        report,
        StepReport {
            observed: 0,
            submitted: 0,
            completed: 1,
            waiting: 1,
            refused: 0,
        }
    );
    assert_all_matched(recording);
    let sent = recording.sent();
    assert_eq!(sent.len(), 3);
    assert_eq!(sent[2], sent[1]);
    assert_eq!(recording.count("ethereum-1", "eth_getLogs"), 1);
    drop(relayer);
    sent
}

fn sequencer_submission(
    name: &str,
) -> (Recording, SignerServer, Instance<'static>, Relayer, String) {
    let recording = Recording::load("inbound_sequencer_receipt.json");
    let signer = signer(name);
    let instance = Instance {
        attestor: ("attestor-1", ATTESTOR_1),
        paxeer_fees: ("paxeer-fees-1", PAXEER_FEES_1),
        journal: journal_path(&work_directory(name), "relayer"),
        cosign: None,
    };
    recording.set_phase("scan");
    let mut relayer = start(&recording, &signer, &instance);
    let report = relayer
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("scan: {error}"));
    assert_eq!(
        report,
        StepReport {
            observed: 1,
            submitted: 1,
            completed: 0,
            waiting: 0,
            refused: 0
        }
    );
    let raw = bridge_in(
        5,
        &deposit(0xaa, 3, padded(0x77), 1_000_000_000_000_000_000),
        &[ATTESTOR_1],
        PAXEER_FEES_1,
    );
    assert_eq!(recording.sent(), vec![("paxeer".to_owned(), raw.clone())]);
    let document: serde_json::Value =
        serde_json::from_str(include_str!("fixtures/inbound_sequencer_receipt.json"))
            .unwrap_or_else(|error| panic!("receipt fixture: {error}"));
    assert_eq!(hex::prefixed(&hash(&raw)), document["transactionHash"]);
    assert_all_matched(&recording);
    (
        recording,
        signer,
        instance,
        relayer,
        inbound_key(1, &[0xaa; 32], 3),
    )
}

#[test]
fn an_inbound_bridge_in_is_receipted_then_committed_from_the_sequencer_receipt() {
    let (recording, signer, instance, mut relayer, item) = sequencer_submission("sequencer-rungs");
    let initial = support::journal_entries(&instance.journal);
    for phase in ["known", "unknown", "submitted"] {
        recording.set_phase(phase);
        let report = relayer
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("{phase}: {error}"));
        assert_eq!(report.waiting, 1);
        assert_eq!(report.completed, 0);
        assert_eq!(support::journal_entries(&instance.journal), initial);
        assert_eq!(
            relayer.journal().state().items[&item].submissions[0].status,
            SubmissionStatus::Pending
        );
    }
    let sent = recording.sent();
    assert_eq!(sent.len(), 2);
    assert_eq!(sent[0], sent[1]);
    assert_eq!(handle_requests(&signer, "paxeer-fees-1"), 1);
    recording.set_phase("receipted");
    let report = relayer
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("receipted: {error}"));
    assert_eq!(report.waiting, 1);
    assert_eq!(report.completed, 0);
    let receipted = SubmissionStatus::Receipted {
        sequence: 2,
        height: 1000,
    };
    assert_eq!(
        relayer.journal().state().items[&item].submissions[0].status,
        receipted
    );
    assert_eq!(completion(&relayer, &item), None);
    let entries = support::journal_entries(&instance.journal);
    assert_eq!(entries.len(), initial.len() + 1);
    assert_eq!(
        entries.last(),
        Some(&Entry::Receipted {
            item: item.clone(),
            tx_hash: hash(&sent[0].1),
            sequence: 2,
            height: 1000
        })
    );
    drop(relayer);
    let mut relayer = start(&recording, &signer, &instance);
    assert_eq!(
        relayer.journal().state().items[&item].submissions[0].status,
        receipted
    );
    assert_eq!(
        relayer
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("repeat receipt: {error}"))
            .waiting,
        1
    );
    assert_eq!(support::journal_entries(&instance.journal), entries);
    recording.set_phase("committed");
    let report = relayer
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("committed: {error}"));
    assert_eq!(report.completed, 1);
    assert_eq!(report.waiting, 0);
    assert_eq!(
        completion(&relayer, &item),
        Some(Completion::Included {
            tx_hash: hash(&sent[0].1),
            block_number: 1003
        })
    );
    assert_eq!(
        relayer.journal().state().items[&item].submissions[0].status,
        SubmissionStatus::Committed {
            sequence: 2,
            height: 1000
        }
    );
    let replay = Journal::open(&instance.journal).unwrap_or_else(|error| panic!("replay: {error}"));
    assert_eq!(replay.state(), relayer.journal().state());
    assert_eq!(recording.count("paxeer", "eth_getTransactionReceipt"), 0);
    assert_eq!(recording.count("paxeer", "eth_blockNumber"), 0);
    assert_eq!(recording.count("paxeer", "pax_sequencerReceipt"), 6);
    assert_eq!(recording.sent(), sent);
    assert_all_matched(&recording);

    let (recording, _signer, instance, mut relayer, item) =
        sequencer_submission("sequencer-direct-commit");
    recording.set_phase("committed");
    assert_eq!(
        relayer
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("direct commit: {error}"))
            .completed,
        1
    );
    assert_eq!(
        relayer.journal().state().items[&item].submissions[0].status,
        SubmissionStatus::Committed {
            sequence: 2,
            height: 1000
        }
    );
    let entries = support::journal_entries(&instance.journal);
    assert!(matches!(
        entries[entries.len() - 2],
        Entry::Receipted {
            sequence: 2,
            height: 1000,
            ..
        }
    ));
    assert!(matches!(
        entries.last(),
        Some(Entry::Completed {
            completion: Completion::Included {
                block_number: 1003,
                ..
            },
            ..
        })
    ));
    assert_all_matched(&recording);
}

#[test]
fn a_receipt_with_status_zero_reverts_the_paxeer_submission() {
    for (phase, name) in [
        ("reverted", "sequencer-reverted"),
        ("consumed", "sequencer-consumed"),
    ] {
        let (recording, _signer, instance, mut relayer, item) = sequencer_submission(name);
        let before = support::journal_entries(&instance.journal);
        let calls_before = recording.count("paxeer", "eth_call");
        recording.set_phase(phase);
        let report = relayer
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("{phase}: {error}"));
        let entries = support::journal_entries(&instance.journal);
        assert_eq!(entries.len(), before.len() + 2);
        assert!(matches!(
            entries[before.len()],
            Entry::Receipted {
                sequence: 5,
                height: 1002,
                ..
            }
        ));
        let tx_hash = hash(&recording.sent()[0].1);
        if phase == "reverted" {
            assert_eq!(report.waiting, 1);
            assert_eq!(report.completed, 0);
            assert_eq!(
                entries.last(),
                Some(&Entry::Reverted {
                    item: item.clone(),
                    tx_hash
                })
            );
            assert_eq!(completion(&relayer, &item), None);
            assert_eq!(
                relayer.journal().state().items[&item].submissions[0].status,
                SubmissionStatus::Reverted
            );
            assert!(relayer.journal().state().items[&item].pending().is_none());
            assert!(relayer.journal().state().items[&item].is_open());
        } else {
            assert_eq!(report.completed, 1);
            assert_eq!(report.waiting, 0);
            assert_eq!(
                completion(&relayer, &item),
                Some(Completion::AlreadyBridged)
            );
            assert_eq!(
                entries.last(),
                Some(&Entry::Completed {
                    item: item.clone(),
                    completion: Completion::AlreadyBridged
                })
            );
            assert!(!entries
                .iter()
                .any(|entry| matches!(entry, Entry::Reverted { .. })));
        }
        assert_eq!(recording.count("paxeer", "eth_call"), calls_before + 1);
        assert_eq!(recording.sent().len(), 1);
        let replay =
            Journal::open(&instance.journal).unwrap_or_else(|error| panic!("replay: {error}"));
        assert_eq!(replay.state(), relayer.journal().state());
        assert_all_matched(&recording);
    }
}

#[test]
fn an_unverifiable_sequencer_receipt_leaves_the_submission_pending() {
    let (recording, signer, instance, mut relayer, item) =
        sequencer_submission("sequencer-invalid");
    recording.set_phase("invalid");
    let before =
        std::fs::read(&instance.journal).unwrap_or_else(|error| panic!("journal: {error}"));
    let state = relayer.journal().state().clone();
    assert!(matches!(
        relayer.inbound_step(0),
        Err(RelayerError::Rpc(RpcFault::Malformed))
    ));
    assert_eq!(relayer.journal().state(), &state);
    assert_eq!(
        std::fs::read(&instance.journal).unwrap_or_else(|error| panic!("journal: {error}")),
        before
    );
    assert_eq!(completion(&relayer, &item), None);
    assert_eq!(
        relayer.journal().state().items[&item].submissions[0].status,
        SubmissionStatus::Pending
    );
    assert_eq!(recording.sent().len(), 1);
    assert_eq!(recording.count("paxeer", "pax_sequencerReceipt"), 1);
    assert_eq!(recording.count("paxeer", "eth_getTransactionReceipt"), 0);
    assert_eq!(handle_requests(&signer, "paxeer-fees-1"), 1);
    drop(relayer);
    let mut relayer = start(&recording, &signer, &instance);
    assert_eq!(relayer.journal().state(), &state);
    recording.set_phase("receipted");
    assert_eq!(
        relayer
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("valid retry: {error}"))
            .waiting,
        1
    );
    assert_eq!(
        relayer.journal().state().items[&item].submissions[0].status,
        SubmissionStatus::Receipted {
            sequence: 2,
            height: 1000
        }
    );
    assert_all_matched(&recording);
}

#[test]
fn inbound_deposits_are_submitted_exactly_once_across_a_restart() {
    let recording = Recording::load("inbound_threshold_one.json");
    let signer = signer("inbound");
    let directory = work_directory("inbound");
    let instance = Instance {
        attestor: ("attestor-1", ATTESTOR_1),
        paxeer_fees: ("paxeer-fees-1", PAXEER_FEES_1),
        journal: journal_path(&directory, "relayer"),
        cosign: None,
    };
    let first = deposit(0xaa, 3, padded(0x77), 1_000_000_000_000_000_000);
    let second = deposit(0xcc, 7, padded(0x88), 5);
    let first_key = inbound_key(1, &[0xaa; 32], 3);
    let second_key = inbound_key(1, &[0xcc; 32], 7);
    let invalid_key = inbound_key(1, &[0xdd; 32], 0);

    recording.set_phase("scan");
    let mut relayer = start(&recording, &signer, &instance);
    let report = relayer
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("scan: {error}"));
    assert_eq!(
        report,
        StepReport {
            observed: 3,
            submitted: 2,
            completed: 0,
            waiting: 0,
            refused: 1,
        }
    );
    assert_all_matched(&recording);
    let sent = recording.sent();
    assert_eq!(sent.len(), 2);
    // The node still reports pending count 5 for the second call; the
    // journal's pending nonce 5 moves the second transaction to nonce 6.
    assert_eq!(
        sent[0].1,
        bridge_in(5, &first, &[ATTESTOR_1], PAXEER_FEES_1)
    );
    assert_eq!(
        sent[1].1,
        bridge_in(6, &second, &[ATTESTOR_1], PAXEER_FEES_1)
    );
    let refusal = relayer
        .journal()
        .state()
        .items
        .get(&invalid_key)
        .and_then(|item| item.refusal.clone());
    assert!(
        refusal.is_some(),
        "the 32-byte non-address recipient is refused"
    );
    assert_eq!(handle_requests(&signer, "attestor-1"), 2);
    assert_eq!(handle_requests(&signer, "paxeer-fees-1"), 2);
    drop(relayer);

    let sent = restart_and_rebroadcast_the_unknown_deposit(&recording, &signer, &instance);

    recording.set_phase("included");
    let mut relayer = start(&recording, &signer, &instance);
    let report = relayer
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("included: {error}"));
    assert_eq!(report.completed, 1);
    assert_all_matched(&recording);
    assert_eq!(
        completion(&relayer, &first_key),
        Some(Completion::Included {
            tx_hash: hash(&sent[0].1),
            block_number: 0x10,
        })
    );
    assert_eq!(
        completion(&relayer, &second_key),
        Some(Completion::Included {
            tx_hash: hash(&sent[1].1),
            block_number: 0x11,
        })
    );
    assert_eq!(recording.sent().len(), 3);
    assert_eq!(handle_requests(&signer, "attestor-1"), 2);
    assert_eq!(handle_requests(&signer, "paxeer-fees-1"), 2);

    let replayed =
        Journal::open(&instance.journal).unwrap_or_else(|error| panic!("replay: {error}"));
    assert_eq!(replayed.state(), relayer.journal().state());
}

#[test]
fn two_attestors_cosign_one_bridge_in_and_the_other_completes_as_already_bridged() {
    let recording = Recording::load("cosign_threshold_two.json");
    let signer = signer("cosign");
    let directory = work_directory("cosign");
    let cosign = directory.join("cosign");
    let one = Instance {
        attestor: ("attestor-1", ATTESTOR_1),
        paxeer_fees: ("paxeer-fees-1", PAXEER_FEES_1),
        journal: journal_path(&directory, "one"),
        cosign: Some(cosign.clone()),
    };
    let two = Instance {
        attestor: ("attestor-2", ATTESTOR_2),
        paxeer_fees: ("paxeer-fees-2", FEES_2),
        journal: journal_path(&directory, "two"),
        cosign: Some(cosign),
    };
    let item = inbound_key(1, &[0xaa; 32], 3);
    let attestation = deposit(0xaa, 3, padded(0x77), 1_000_000_000_000_000_000);

    recording.set_phase("pending");
    let mut first = start(&recording, &signer, &one);
    let mut second = start(&recording, &signer, &two);
    let report = first
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("first: {error}"));
    assert_eq!(report.observed, 1);
    assert_eq!(
        report.waiting, 1,
        "one of two signatures cannot be submitted"
    );
    assert!(recording.sent().is_empty());
    let report = second
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("second: {error}"));
    assert_eq!(report.submitted, 1);
    assert_all_matched(&recording);
    let sent = recording.sent();
    assert_eq!(sent.len(), 1);
    // Both signatures in one call, ascending by signer address (0x612b… < 0xd243…).
    assert_eq!(
        sent[0].1,
        bridge_in(0, &attestation, &[ATTESTOR_2, ATTESTOR_1], FEES_2)
    );

    recording.set_phase("bridged");
    let report = first
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("first again: {error}"));
    assert_eq!(report.completed, 1);
    let report = second
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("second again: {error}"));
    assert_eq!(report.completed, 1);
    assert_all_matched(&recording);
    assert_eq!(recording.sent().len(), 1);
    assert_eq!(completion(&first, &item), Some(Completion::AlreadyBridged));
    assert_eq!(
        completion(&second, &item),
        Some(Completion::Included {
            tx_hash: hash(&sent[0].1),
            block_number: 0x10,
        })
    );
}

#[test]
fn a_second_relayers_duplicate_bridge_in_reverts_and_is_never_retried() {
    let recording = Recording::load("duplicate_submissions.json");
    let signer = signer("duplicate");
    let directory = work_directory("duplicate");
    let one = Instance {
        attestor: ("attestor-1", ATTESTOR_1),
        paxeer_fees: ("paxeer-fees-1", PAXEER_FEES_1),
        journal: journal_path(&directory, "one"),
        cosign: None,
    };
    let two = Instance {
        attestor: ("attestor-2", ATTESTOR_2),
        paxeer_fees: ("paxeer-fees-2", FEES_2),
        journal: journal_path(&directory, "two"),
        cosign: None,
    };
    let item = inbound_key(1, &[0xaa; 32], 3);
    let attestation = deposit(0xaa, 3, padded(0x77), 1_000_000_000_000_000_000);

    recording.set_phase("race");
    let mut first = start(&recording, &signer, &one);
    let mut second = start(&recording, &signer, &two);
    for relayer in [&mut first, &mut second] {
        let report = relayer
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("race: {error}"));
        assert_eq!(report.submitted, 1);
    }
    assert_all_matched(&recording);
    let sent = recording.sent();
    assert_eq!(sent.len(), 2);
    assert_eq!(
        sent[0].1,
        bridge_in(0, &attestation, &[ATTESTOR_1], PAXEER_FEES_1)
    );
    assert_eq!(sent[1].1, bridge_in(0, &attestation, &[ATTESTOR_2], FEES_2));

    recording.set_phase("mined");
    for relayer in [&mut first, &mut second] {
        let report = relayer
            .inbound_step(0)
            .unwrap_or_else(|error| panic!("mined: {error}"));
        assert_eq!(report.completed, 1);
    }
    let report = second
        .inbound_step(0)
        .unwrap_or_else(|error| panic!("after: {error}"));
    assert_eq!(report, StepReport::default());
    assert_all_matched(&recording);
    assert_eq!(recording.sent().len(), 2);
    assert_eq!(
        completion(&first, &item),
        Some(Completion::Included {
            tx_hash: hash(&sent[0].1),
            block_number: 0x10,
        })
    );
    assert_eq!(completion(&second, &item), Some(Completion::AlreadyBridged));
    let submissions = second
        .journal()
        .state()
        .items
        .get(&item)
        .map(|item| item.submissions.len());
    assert_eq!(submissions, Some(1));
}

#[test]
fn a_final_paxeer_burn_is_released_from_the_vault_with_the_vector_digest() {
    let recording = Recording::load("outbound_release.json");
    let signer = signer("outbound");
    let directory = work_directory("outbound");
    let instance = Instance {
        attestor: ("attestor-1", ATTESTOR_1),
        paxeer_fees: ("paxeer-fees-1", PAXEER_FEES_1),
        journal: journal_path(&directory, "relayer"),
        cosign: None,
    };
    let attestation = OutboundAttestation {
        chain_id: 1,
        vault: [0x11; 20],
        paxeer_tx_hash: [0x22; 32],
        paxeer_nonce: 7,
        recipient: [0x33; 20],
        asset: [0x44; 20],
        amount: uint256_from_u64(1_000_000_000_000_000_000),
    };
    assert_eq!(
        hex::prefixed(&attestation.digest()),
        "0xbd35888e4b158986238ce7abe73957702e2f6e78fe6157197878ebd13edf5b37"
    );
    let item = outbound_key(1, &[0x22; 32], 7);

    recording.set_phase("release");
    let mut relayer = start(&recording, &signer, &instance);
    let report = relayer
        .outbound_step()
        .unwrap_or_else(|error| panic!("release: {error}"));
    assert_eq!(report.observed, 1);
    assert_eq!(report.submitted, 1);
    assert_all_matched(&recording);
    let sent = recording.sent();
    assert_eq!(sent.len(), 1);
    assert_eq!(sent[0].0, "ethereum-1");
    let signature = attest(ATTESTOR_1, &attestation.digest());
    // Ethereum fees in the recording: priority 2 gwei (under the 3 gwei cap),
    // base 10 gwei, so `max_fee = 2 * 10 + 2` gwei.
    let expected = transaction(
        &Fees {
            chain_id: 1,
            nonce: 9,
            priority: 2_000_000_000,
            max_fee: 22_000_000_000,
            gas_limit: CHAIN.gas.gas_limit,
        },
        CHAIN.vault,
        encode_release(&attestation, &[signature]),
        FEES_2,
    );
    assert_eq!(sent[0].1, expected);
    let domains: Vec<Vec<u8>> = signer
        .requests()
        .into_iter()
        .map(|(_, domain, _)| domain)
        .collect();
    assert_eq!(
        domains,
        vec![
            ATTEST_OUTBOUND_DOMAIN.to_vec(),
            ETHEREUM_TRANSACTION_DOMAIN.to_vec()
        ]
    );

    recording.set_phase("released");
    let report = relayer
        .outbound_step()
        .unwrap_or_else(|error| panic!("released: {error}"));
    assert_eq!(report.completed, 1);
    assert_all_matched(&recording);
    assert_eq!(
        completion(&relayer, &item),
        Some(Completion::Included {
            tx_hash: hash(&sent[0].1),
            block_number: 0x100,
        })
    );
}
