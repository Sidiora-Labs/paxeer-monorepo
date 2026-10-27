use layerx_mirror::committed::{CommittedBlock, CommittedHeader};
use layerx_mirror::source::{
    MirrorCanonicalPosition, MirrorLag, MirrorObservation, MirrorProvenance, MirrorSourceError,
    MirrorSourceId, MirrorTargetIdentity,
};
use layerx_mirror::{
    reconcile_journal, verified_cursor, Archive, ArchiveData, CommittedCoordinate,
    CommittedFreshness, MirrorCursor, MirrorFreshness, MirrorVerificationFreshness, NodeHead,
};

fn coordinate(height: u64, byte: u8) -> CommittedCoordinate {
    CommittedCoordinate {
        height,
        block_hash: [byte; 32],
    }
}

fn archive(height: u64, byte: u8) -> Archive {
    let committed = coordinate(height, byte);
    Archive::from_committed(
        &CommittedBlock {
            header: CommittedHeader {
                height,
                block_hash: committed.block_hash,
                parent_hash: [2; 32],
                app_hash: [3; 32],
                timestamp: 1_700_000_000,
                transactions: Vec::new(),
            },
            receipts: Vec::new(),
        },
        125,
        NodeHead {
            latest_sealed_batch: height,
            latest_committed: Some(committed),
        },
    )
    .unwrap_or_else(|error| panic!("committed archive: {error:?}"))
}

#[test]
fn freshness_reports_committed_lag_in_blocks() {
    let target = coordinate(20, 4);
    let node = NodeHead {
        latest_sealed_batch: 30,
        latest_committed: Some(target),
    };
    let cursor = MirrorCursor {
        latest_batch: Some(29),
        latest_committed: Some(coordinate(13, 5)),
    };
    let freshness = MirrorFreshness::new(cursor, node);
    assert_eq!(
        freshness.committed,
        CommittedFreshness::Behind { block_lag: 7 }
    );
    assert_eq!(freshness.batch_lag, 1);
    assert_eq!(freshness.node_latest_committed, Some(target));
    assert_eq!(freshness.latest_committed_mirrored, cursor.latest_committed);
    let verified = MirrorVerificationFreshness::relative(freshness);
    assert_eq!(
        verified.committed,
        Some(CommittedFreshness::Behind { block_lag: 7 })
    );
    assert_eq!(verified.latest_committed_mirrored, cursor.latest_committed);
    for (current, expected) in [
        (
            None,
            CommittedFreshness::NotYetMirrored { target_height: 20 },
        ),
        (Some(target), CommittedFreshness::Current),
        (
            Some(coordinate(20, 9)),
            CommittedFreshness::DifferentAtHeight { height: 20 },
        ),
        (
            Some(coordinate(21, 9)),
            CommittedFreshness::DifferentAtHeight { height: 21 },
        ),
    ] {
        assert_eq!(
            MirrorFreshness::new(
                MirrorCursor {
                    latest_committed: current,
                    ..cursor
                },
                node
            )
            .committed,
            expected
        );
    }
    assert_eq!(
        MirrorFreshness::new(
            cursor,
            NodeHead {
                latest_committed: None,
                ..node
            }
        )
        .committed,
        CommittedFreshness::NodeHasNoCommittedBlock,
    );
    let archives = [archive(13, 5), archive(20, 4)];
    let cursor = verified_cursor(&archives);
    assert_eq!(cursor.latest_committed, Some(target));
    assert_eq!(
        MirrorFreshness::new(cursor, node).committed,
        CommittedFreshness::Current
    );
    assert_eq!(
        reconcile_journal(cursor, &archives, |commitment| {
            assert_eq!(commitment, archives[1].commitment());
            Ok::<_, ()>(Some(archives[1].bytes().to_vec()))
        }),
        Ok(())
    );
}

#[test]
fn a_source_observation_carries_the_committed_coordinate() {
    let archive = archive(42, 7);
    assert_eq!(&archive.bytes()[8..10], &4_u16.to_be_bytes());
    let decoded =
        ArchiveData::decode(archive.bytes()).unwrap_or_else(|error| panic!("decode v4: {error:?}"));
    let expected = Some(coordinate(42, 7));
    assert_eq!(decoded.committed_coordinate(), expected);
    let source =
        MirrorSourceId::new("ethereum").unwrap_or_else(|error| panic!("source id: {error:?}"));
    let target = MirrorTargetIdentity::Ethereum {
        chain_id: 1,
        genesis_hash: [1; 32],
        contract: [2; 20],
        code_hash: [3; 32],
        publisher: [4; 20],
    };
    let position = MirrorCanonicalPosition::Ethereum {
        block_number: 500,
        block_hash: [5; 32],
        reference_head_number: 510,
        reference_head_hash: [6; 32],
    };
    let observe = |bytes: &[u8], network, height| {
        MirrorObservation::from_archive(
            source.clone(),
            target.clone(),
            bytes,
            network,
            height,
            archive.commitment(),
            position,
        )
    };
    let observation = observe(archive.bytes(), 125, 42)
        .unwrap_or_else(|error| panic!("source observation: {error:?}"));
    assert_eq!(observation.committed, expected);
    assert_eq!(observation.freshness.latest_committed, expected);
    assert_eq!(observation.freshness.committed_lag, MirrorLag::Unknown);
    assert_eq!(observation.freshness.batch_lag, MirrorLag::Unknown);
    assert_eq!(observation.provenance, MirrorProvenance::Canonical);
    assert_eq!(observation.source, source);
    assert_eq!(observation.target, target);
    assert_eq!(observation.position, position);
    let freshness = MirrorVerificationFreshness::from_source(observation.freshness);
    assert_eq!(freshness.latest_committed_mirrored, expected);
    assert_eq!(
        observe(archive.bytes(), 126, 42),
        Err(MirrorSourceError::TargetMismatch)
    );
    assert_eq!(
        observe(archive.bytes(), 125, 43),
        Err(MirrorSourceError::TargetMismatch)
    );
    let mut altered = archive.bytes().to_vec();
    altered[22] ^= 1;
    assert_eq!(observe(&altered, 125, 42), Err(MirrorSourceError::Archive));
}
