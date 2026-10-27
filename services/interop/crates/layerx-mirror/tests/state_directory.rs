use std::collections::BTreeMap;
use std::fmt::Debug;
use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_mirror::committed::CommittedCoordinate;
use layerx_mirror::store::{
    ArchiveSpool, FinalityPosition, MirrorChain, PublicationJournal, PublicationPhase,
    PublicationRecord, PublicationStage, StoreError, TransactionIdentity,
};
use layerx_mirror::{archive_commitment, ArchiveCommitment, CheckpointCoordinate, NodeHead};
use sha2::{Digest, Sha256};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

struct StateDirectory(PathBuf);

impl StateDirectory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "layerx-state-directory-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        succeed(fs::create_dir(&path));
        Self(path)
    }
}

impl Drop for StateDirectory {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn succeed<T, E: Debug>(result: Result<T, E>) -> T {
    result.unwrap_or_else(|error| panic!("state directory operation: {error:?}"))
}

fn checkpoint(value: CommittedCoordinate) -> CheckpointCoordinate {
    CheckpointCoordinate {
        batch_number: value.height,
        checkpoint_id: value.block_hash,
    }
}

fn snapshot(directory: &Path) -> BTreeMap<PathBuf, Vec<u8>> {
    succeed(fs::read_dir(directory))
        .map(|entry| {
            let path = succeed(entry).path();
            let bytes = succeed(fs::read(&path));
            (path, bytes)
        })
        .collect()
}

#[test]
fn a_v3_journal_round_trips_a_committed_cursor() {
    let directory = StateDirectory::new();
    let committed = CommittedCoordinate {
        height: 41,
        block_hash: [0x64; 32],
    };
    let node_head = NodeHead {
        latest_sealed_batch: 43,
        latest_finalised_checkpoint: Some(checkpoint(committed)),
    };
    let spool_path = directory.0.join("spool");
    let archives = [
        b"first committed archive".as_slice(),
        b"second committed archive".as_slice(),
    ];
    {
        let spool = succeed(ArchiveSpool::open(&spool_path));
        for (index, bytes) in archives.iter().enumerate() {
            let head = if index == 0 {
                node_head
            } else {
                NodeHead {
                    latest_sealed_batch: 44,
                    latest_finalised_checkpoint: None,
                }
            };
            succeed(spool.put(archive_commitment(bytes), bytes, head));
        }
    }
    let spool = succeed(ArchiveSpool::open(&spool_path));
    for (index, bytes) in archives.iter().enumerate() {
        let stored = succeed(spool.get(archive_commitment(bytes)));
        assert_eq!(stored.bytes, *bytes);
        assert_eq!(
            stored.node_head.latest_finalised_checkpoint,
            if index == 0 {
                Some(checkpoint(committed))
            } else {
                None
            }
        );
        assert_eq!(stored.node_head.latest_sealed_batch, 43 + index as u64);
    }
    for entry in succeed(fs::read_dir(&spool_path)) {
        let path = succeed(entry).path();
        if path.extension().and_then(|value| value.to_str()) != Some("archive") {
            continue;
        }
        let bytes = succeed(fs::read(path));
        assert_eq!(&bytes[..8], b"LXMSPL03");
        if bytes[16] == 1 {
            assert_eq!(&bytes[17..25], &committed.height.to_be_bytes());
            assert_eq!(&bytes[25..57], &committed.block_hash);
        } else {
            assert_eq!(bytes[16], 0);
        }
    }

    for (chain, lane) in [
        (MirrorChain::Ethereum, "ethereum"),
        (MirrorChain::Solana, "solana"),
    ] {
        let lane_path = directory.0.join(lane);
        let mut expected_records = Vec::new();
        {
            let mut journal = succeed(PublicationJournal::open(&lane_path, chain, 7));
            assert_eq!(journal.cursor().latest_batch, None);
            assert_eq!(journal.cursor().latest_checkpoint, None);
            for (index, bytes) in archives.iter().enumerate() {
                let mut record = PublicationRecord {
                    chain,
                    batch_number: 7 + index as u64,
                    commitment: archive_commitment(bytes),
                    checkpoint: if index == 0 {
                        Some(checkpoint(committed))
                    } else {
                        None
                    },
                    stage: PublicationStage::Finalize,
                    phase: PublicationPhase::Prepared,
                    stage_payload_digest: [0x33; 32],
                    signed_payload: Vec::new(),
                    transaction: TransactionIdentity::None,
                    position: FinalityPosition::None,
                };
                succeed(journal.append(record.clone()));
                for phase in [
                    PublicationPhase::Signed,
                    PublicationPhase::Pending,
                    PublicationPhase::Finalized,
                    PublicationPhase::RetrievedVerified,
                ] {
                    record.phase = phase;
                    record.signed_payload = vec![1, 2, 3];
                    (record.transaction, record.position) = match chain {
                        MirrorChain::Ethereum => (
                            TransactionIdentity::Ethereum([0x55; 32]),
                            FinalityPosition::Ethereum {
                                block_number: 90,
                                block_hash: [0x66; 32],
                            },
                        ),
                        MirrorChain::Solana => (
                            TransactionIdentity::Solana([0x55; 64]),
                            FinalityPosition::Solana {
                                slot: 90,
                                blockhash: [0x66; 32],
                            },
                        ),
                    };
                    succeed(journal.append(record.clone()));
                }
                expected_records.push(record);
            }
            assert_eq!(journal.cursor().latest_batch, Some(8));
            assert_eq!(
                journal.cursor().latest_checkpoint,
                Some(checkpoint(committed))
            );
        }
        let before = snapshot(&lane_path);
        let journal = succeed(PublicationJournal::open(&lane_path, chain, 7));
        assert_eq!(journal.cursor().latest_batch, Some(8));
        let restored = journal
            .cursor()
            .latest_checkpoint
            .map(|value| CommittedCoordinate {
                height: value.batch_number,
                block_hash: value.checkpoint_id,
            });
        assert_eq!(restored, Some(committed));
        for record in expected_records {
            assert_eq!(
                journal.record(record.commitment, record.stage),
                Some(&record)
            );
        }
        assert_eq!(snapshot(&lane_path), before);
        let head = succeed(fs::read(lane_path.join(format!("{lane}.head"))));
        assert_eq!(&head[..8], b"LXMHED03");
        let journal_bytes = succeed(fs::read(lane_path.join(format!("{lane}.journal"))));
        let mut offset = 0;
        let mut count = 0;
        while offset < journal_bytes.len() {
            let length =
                u32::from_be_bytes(succeed(journal_bytes[offset + 8..offset + 12].try_into()))
                    as usize;
            let body = &journal_bytes[offset + 12..offset + 12 + length];
            assert_eq!(&body[..8], b"LXMJNL03");
            assert_eq!(&body[8..10], &3_u16.to_be_bytes());
            if count < 5 {
                assert_eq!(body[51], 1);
                assert_eq!(&body[52..60], &committed.height.to_be_bytes());
                assert_eq!(&body[60..92], &committed.block_hash);
            } else {
                assert_eq!(body[51], 0);
            }
            offset += 12 + length + 32;
            count += 1;
        }
        assert_eq!(count, 10);
    }
}

fn framed_record(magic: &[u8; 8], version: u16) -> (Vec<u8>, Vec<u8>) {
    let mut body = Vec::new();
    body.extend_from_slice(magic);
    body.extend_from_slice(&version.to_be_bytes());
    body.push(1);
    body.extend_from_slice(&7_u64.to_be_bytes());
    body.extend_from_slice(ArchiveCommitment::from_bytes([0x17; 32]).as_bytes());
    body.push(1);
    body.extend_from_slice(&41_u64.to_be_bytes());
    body.extend_from_slice(&[0x64; 32]);
    body.extend_from_slice(&[3, 1]);
    body.extend_from_slice(&[0x33; 32]);
    body.extend_from_slice(&0_u32.to_be_bytes());
    body.extend_from_slice(&[0, 0]);
    let length = succeed(u32::try_from(body.len()));
    let mut hasher = Sha256::new();
    hasher.update(b"LayerX/mirror/journal-record/v2\0");
    hasher.update([0; 32]);
    hasher.update(1_u64.to_be_bytes());
    hasher.update(length.to_be_bytes());
    hasher.update(&body);
    let digest = hasher.finalize();
    let mut journal = Vec::new();
    journal.extend_from_slice(&1_u64.to_be_bytes());
    journal.extend_from_slice(&length.to_be_bytes());
    journal.extend_from_slice(&body);
    journal.extend_from_slice(&digest);
    let mut head = Vec::new();
    head.extend_from_slice(b"LXMHED03");
    head.extend_from_slice(&1_u64.to_be_bytes());
    head.extend_from_slice(&succeed(u64::try_from(journal.len())).to_be_bytes());
    head.extend_from_slice(&digest);
    (journal, head)
}

#[test]
fn a_v2_state_directory_is_refused_at_open() {
    for (magic, version) in [(b"LXMJNL02", 2), (b"LXMJNL02", 3), (b"LXMJNL03", 2)] {
        for sealed in [false, true] {
            let directory = StateDirectory::new();
            let (journal, head) = framed_record(magic, version);
            succeed(fs::write(directory.0.join("ethereum.journal"), journal));
            succeed(fs::write(directory.0.join("ethereum.lock"), []));
            if sealed {
                succeed(fs::write(directory.0.join("ethereum.head"), head));
            }
            let before = snapshot(&directory.0);
            assert!(matches!(
                PublicationJournal::open(&directory.0, MirrorChain::Ethereum, 7),
                Err(StoreError::Corrupt)
            ));
            assert_eq!(snapshot(&directory.0), before);
        }
    }
    let directory = StateDirectory::new();
    let (journal, mut head) = framed_record(b"LXMJNL03", 3);
    head[..8].copy_from_slice(b"LXMHED02");
    succeed(fs::write(directory.0.join("ethereum.journal"), journal));
    succeed(fs::write(directory.0.join("ethereum.head"), head));
    succeed(fs::write(directory.0.join("ethereum.lock"), []));
    let before = snapshot(&directory.0);
    assert!(matches!(
        PublicationJournal::open(&directory.0, MirrorChain::Ethereum, 7),
        Err(StoreError::Corrupt)
    ));
    assert_eq!(snapshot(&directory.0), before);

    let directory = StateDirectory::new();
    let archive = b"version two spooled archive";
    let commitment = archive_commitment(archive);
    let name: String = commitment
        .as_bytes()
        .iter()
        .map(|byte| format!("{byte:02x}"))
        .collect();
    let mut bytes = Vec::new();
    bytes.extend_from_slice(b"LXMSPL02");
    bytes.extend_from_slice(&43_u64.to_be_bytes());
    bytes.push(1);
    bytes.extend_from_slice(&41_u64.to_be_bytes());
    bytes.extend_from_slice(&[0x64; 32]);
    bytes.extend_from_slice(&succeed(u32::try_from(archive.len())).to_be_bytes());
    bytes.extend_from_slice(archive);
    let digest = Sha256::digest(&bytes);
    bytes.extend_from_slice(&digest);
    succeed(fs::write(
        directory.0.join(format!("{name}.archive")),
        bytes,
    ));
    succeed(fs::write(directory.0.join(".spool.lock"), []));
    let before = snapshot(&directory.0);
    assert!(matches!(
        ArchiveSpool::open(&directory.0),
        Err(StoreError::Corrupt)
    ));
    assert_eq!(snapshot(&directory.0), before);
}
