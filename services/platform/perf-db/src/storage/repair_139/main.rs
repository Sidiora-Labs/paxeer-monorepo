use std::fs::{self, OpenOptions};
use std::io::{Read, Write};
use std::path::Path;

use perfdb::relational::alerts::Alert;
use perfdb::relational::api_keys::ApiKey;
use perfdb::relational::competitions::{Competition, CompetitionEntry};
use perfdb::storage::relational::DurableRelationalStore;
use perfdb::storage::snapshot::{self, CompleteStateSnapshot, SnapshotReferralTier};
use perfdb::storage::wal::{self, RelationalMutation, SyncPolicy, WalConfig, WalEntry, WalWriter};
use perfdb::{PerfDb, PerfDbConfig};

fn config(dir: &Path) -> WalConfig {
    WalConfig {
        dir: dir.into(),
        max_segment_size: 1024 * 1024,
        sync_policy: SyncPolicy::EveryWrite,
    }
}

fn append_bytes(path: &Path, bytes: &[u8]) {
    let mut file = OpenOptions::new().append(true).open(path).unwrap();
    file.write_all(bytes).unwrap();
    file.sync_all().unwrap();
}

fn checkpoints(dir: &Path) -> Vec<u64> {
    let mut blocks = Vec::new();
    wal::replay(dir, |entry| {
        match entry {
            WalEntry::Checkpoint { block_number } => blocks.push(block_number),
            other => panic!("unexpected WAL record {other:?}"),
        }
        Ok(())
    })
    .unwrap();
    blocks
}

fn wal_case(root: &Path) {
    let producer = root.join("producer");
    let mut writer = WalWriter::open(config(&producer)).unwrap();
    writer
        .append(&WalEntry::Checkpoint { block_number: 1 })
        .unwrap();
    let prefix_len = writer.segment_bytes() as usize;
    writer
        .append(&WalEntry::Checkpoint { block_number: 2 })
        .unwrap();
    drop(writer);
    let produced = fs::read(producer.join("wal_00000000.bin")).unwrap();
    let prefix = &produced[..prefix_len];
    let frame = &produced[prefix_len..];
    assert_eq!(
        u32::from_le_bytes(frame[..4].try_into().unwrap()) as usize + 8,
        frame.len()
    );
    assert_eq!(
        u32::from_le_bytes(frame[4..8].try_into().unwrap()),
        crc32fast::hash(&frame[8..])
    );
    let mut tails: Vec<Vec<u8>> = (1..frame.len()).map(|n| frame[..n].to_vec()).collect();
    let mut bad_crc = frame.to_vec();
    bad_crc[4] ^= 1;
    tails.push(bad_crc);
    let mut invalid_payload = frame.to_vec();
    invalid_payload[8..12].copy_from_slice(&u32::MAX.to_le_bytes());
    let checksum = crc32fast::hash(&invalid_payload[8..]);
    invalid_payload[4..8].copy_from_slice(&checksum.to_le_bytes());
    tails.push(invalid_payload);
    let mut invalid_length = frame.to_vec();
    invalid_length[..4].copy_from_slice(&u32::MAX.to_le_bytes());
    tails.push(invalid_length);
    let mut trailing_payload = frame.to_vec();
    trailing_payload.push(0);
    trailing_payload[..4].copy_from_slice(&((frame.len() - 7) as u32).to_le_bytes());
    let checksum = crc32fast::hash(&trailing_payload[8..]);
    trailing_payload[4..8].copy_from_slice(&checksum.to_le_bytes());
    tails.push(trailing_payload);
    for (i, tail) in tails.iter().enumerate() {
        let dir = root.join(format!("torn-{i}"));
        fs::create_dir(&dir).unwrap();
        let path = dir.join("wal_00000000.bin");
        fs::write(&path, prefix).unwrap();
        append_bytes(&path, tail);
        assert_eq!(checkpoints(&dir), [1]);
        let mut recovered = WalWriter::open(config(&dir)).unwrap();
        assert_eq!(recovered.segment_bytes(), prefix.len() as u64);
        assert_eq!(fs::metadata(&path).unwrap().len(), prefix.len() as u64);
        recovered
            .append(&WalEntry::Checkpoint { block_number: 99 })
            .unwrap();
        drop(recovered);
        let reopened = WalWriter::open(config(&dir)).unwrap();
        assert_eq!(
            checkpoints(&dir),
            [1, 99],
            "lost acknowledged write after damaged tail {i}"
        );
        drop(reopened);
    }
    for segmented in [false, true] {
        let dir = root.join(format!("interior-{segmented}"));
        fs::create_dir(&dir).unwrap();
        let path = dir.join("wal_00000000.bin");
        let mut bytes = produced.clone();
        bytes[prefix_len + 4] ^= 1;
        if segmented {
            fs::write(dir.join("wal_00000001.bin"), frame).unwrap();
        } else {
            bytes.extend_from_slice(frame);
        }
        fs::write(&path, &bytes).unwrap();
        assert!(WalWriter::open(config(&dir)).is_err());
        assert!(wal::replay(&dir, |_| Ok(())).is_err());
        assert_eq!(
            fs::read(&path).unwrap(),
            bytes,
            "interior corruption must not be truncated"
        );
    }
    let gap = root.join("missing-segment");
    fs::create_dir(&gap).unwrap();
    fs::write(gap.join("wal_00000000.bin"), prefix).unwrap();
    fs::write(gap.join("wal_00000002.bin"), frame).unwrap();
    assert!(WalWriter::open(config(&gap)).is_err());
}

fn snapshot_case(root: &Path) {
    let db_dir = root.join("engine");
    let cfg = PerfDbConfig::new(&db_dir);
    let db = PerfDb::open(cfg.clone()).unwrap();
    db.save_checkpoint(10, "old-image".into(), 120);
    let path = db.snapshot().unwrap();
    let old_bytes = fs::read(&path).unwrap();
    let mut old_handle = fs::File::open(&path).unwrap();
    db.save_checkpoint(10, "replacement".into(), 121);
    db.snapshot().unwrap();
    let mut old_inode = Vec::new();
    old_handle.read_to_end(&mut old_inode).unwrap();
    assert_eq!(
        old_inode, old_bytes,
        "publishing must replace the inode without truncating it"
    );
    assert_eq!(
        snapshot::load_snapshot(&cfg.snapshots_dir(), 10)
            .unwrap()
            .unwrap()
            .checkpoint
            .last_block_hash,
        "replacement"
    );
    let published = fs::read(&path).unwrap();
    fs::write(
        cfg.snapshots_dir().join(".snapshot-interrupted.tmp"),
        &published[..published.len() / 2],
    )
    .unwrap();
    drop(db);
    let reopened = PerfDb::open(cfg.clone()).unwrap();
    assert_eq!(
        reopened.checkpoint.snapshot().last_block_hash,
        "replacement"
    );
    drop(reopened);
    let mut writer = WalWriter::open(config(&cfg.wal.dir)).unwrap();
    writer
        .append(&WalEntry::Checkpoint { block_number: 20 })
        .unwrap();
    drop(writer);
    let bad_path = cfg.snapshots_dir().join("snapshot_000000000020.bin");
    let mut corrupt = published.clone();
    let index = corrupt.len() - 1;
    corrupt[index] ^= 1;
    fs::write(&bad_path, &corrupt).unwrap();
    let recovered = PerfDb::open(cfg.clone()).unwrap();
    assert_eq!(
        recovered.checkpoint.last_block(),
        20,
        "fallback must replay the verified WAL"
    );
    assert_eq!(
        recovered.checkpoint.snapshot().last_block_hash,
        "replacement"
    );
    recovered.shutdown().unwrap();
    drop(recovered);
    let second = PerfDb::open(cfg.clone()).unwrap();
    assert_eq!(second.checkpoint.last_block(), 20);
    drop(second);

    let unsafe_dir = root.join("uncovered");
    let unsafe_cfg = PerfDbConfig::new(&unsafe_dir);
    let db = PerfDb::open(unsafe_cfg.clone()).unwrap();
    db.save_checkpoint(10, "safe".into(), 1);
    db.snapshot().unwrap();
    drop(db);
    fs::write(
        unsafe_cfg.snapshots_dir().join("snapshot_000000000020.bin"),
        &corrupt,
    )
    .unwrap();
    assert!(
        PerfDb::open(unsafe_cfg).is_err(),
        "cannot silently roll back without WAL coverage"
    );

    let state = snapshot::load_snapshot(&cfg.snapshots_dir(), 10)
        .unwrap()
        .unwrap();
    let legacy = root.join("legacy");
    fs::create_dir(&legacy).unwrap();
    fs::write(
        legacy.join("snapshot_000000000010.bin"),
        bincode::serialize(&state).unwrap(),
    )
    .unwrap();
    assert_eq!(
        snapshot::load_latest_snapshot(&legacy)
            .unwrap()
            .unwrap()
            .checkpoint
            .last_block,
        10
    );
    let concurrent = root.join("concurrent");
    snapshot::write_snapshot(&concurrent, &state).unwrap();
    std::thread::scope(|scope| {
        for n in 0..2 {
            let dir = concurrent.clone();
            let mut state = state.clone();
            scope.spawn(move || {
                for _ in 0..20 {
                    state.checkpoint.last_block_hash = format!("writer-{n}");
                    snapshot::write_snapshot(&dir, &state).unwrap();
                }
            });
        }
        for _ in 0..40 {
            let hash = snapshot::load_snapshot(&concurrent, 10)
                .unwrap()
                .unwrap()
                .checkpoint
                .last_block_hash;
            assert!(["replacement", "writer-0", "writer-1"].contains(&hash.as_str()));
        }
    });
    assert_eq!(
        fs::read_dir(&concurrent).unwrap().count(),
        1,
        "successful writes must leave no temporary files"
    );
}

fn alert(address: u8) -> Alert {
    Alert {
        id: 0,
        address: [address; 20],
        alert_type: "price_above".into(),
        market_id: Some(0),
        threshold: Some("67000".into()),
        enabled: true,
        triggered_at: None,
        created_at: 100,
    }
}

fn tier(name: &str, min: u32, discount: u32) -> SnapshotReferralTier {
    SnapshotReferralTier {
        tier: name.into(),
        min_referrals: min,
        min_volume: 0,
        reward_bps: 20,
        fee_discount_bps: discount,
    }
}

fn relational_case(root: &Path) {
    let dir = root.join("relational");
    let registry = ["basic", "pro"];
    let mut db = DurableRelationalStore::open(&dir, &registry).unwrap();
    assert_eq!(db.insert_alert(alert(1)).unwrap(), 1);
    let second_id = db.insert_alert(alert(2)).unwrap();
    assert_eq!(second_id, 2);
    db.mutate(RelationalMutation::DeleteAlert(1)).unwrap();
    let mut second = db.alert(second_id).unwrap();
    second.enabled = false;
    second.triggered_at = Some(999);
    db.mutate(RelationalMutation::PutAlert(second)).unwrap();
    let key = ApiKey {
        id: "operator".into(),
        address: [1; 20],
        key_hash: "old-hash".into(),
        label: Some("trading".into()),
        rate_limit_rpm: 120,
        last_used_at: Some(800),
        expires_at: Some(10000),
        created_at: 1,
    };
    db.mutate(RelationalMutation::PutApiKey(key.clone()))
        .unwrap();
    let mut updated = key.clone();
    updated.address = [2; 20];
    updated.key_hash = "new-hash".into();
    db.mutate(RelationalMutation::PutApiKey(updated)).unwrap();
    assert!(db.api_key_by_hash("old-hash").is_none());
    assert!(db.api_keys_by_user(&[1; 20]).is_empty());
    db.mutate(RelationalMutation::PutCompetition(Competition {
        id: "c1".into(),
        name: "PnL".into(),
        description: Some("weekly".into()),
        start_at: 1,
        end_at: 1000,
        metric: "pnl".into(),
        prize_pool: Some("2000".into()),
        active: true,
        created_at: 1,
    }))
    .unwrap();
    for (address, score) in [(2, 150), (3, 250)] {
        db.mutate(RelationalMutation::PutCompetitionEntry(CompetitionEntry {
            competition_id: "c1".into(),
            address: [address; 20],
            score,
            rank: None,
            joined_at: 2,
        }))
        .unwrap();
    }
    db.mutate(RelationalMutation::SetReferralTiers(vec![
        tier("basic", 0, 1),
        tier("pro", 5, 25),
    ]))
    .unwrap();
    let sequence = db.snapshot().sequence;
    assert!(db
        .mutate(RelationalMutation::SetReferralTiers(vec![tier(
            "unknown", 0, 1
        )]))
        .is_err());
    assert_eq!(
        db.snapshot().sequence,
        sequence,
        "rejected input must not enter the WAL"
    );
    db.shutdown().unwrap();
    drop(db);
    let mut db = DurableRelationalStore::open(&dir, &registry).unwrap();
    assert!(db.alert(1).is_none());
    assert_eq!(db.alerts_by_user(&[2; 20]).len(), 1);
    let alert2 = db.alert(2).unwrap();
    assert!(!alert2.enabled);
    assert_eq!(alert2.triggered_at, Some(999));
    assert!(db.api_key_by_hash("old-hash").is_none());
    let recovered_key = db.api_key_by_hash("new-hash").unwrap();
    assert_eq!(recovered_key.label.as_deref(), Some("trading"));
    assert_eq!(recovered_key.last_used_at, Some(800));
    assert_eq!(recovered_key.expires_at, Some(10000));
    assert_eq!(db.api_keys_by_user(&[2; 20]).len(), 1);
    assert!(db.api_keys_by_user(&[1; 20]).is_empty());
    assert_eq!(
        db.competition("c1").unwrap().prize_pool.as_deref(),
        Some("2000")
    );
    assert_eq!(db.competition_entries("c1")[0].score, 250);
    assert_eq!(db.user_competitions(&[2; 20]), ["c1"]);
    assert_eq!(db.referral_tier(0, 0).tier, "basic");
    assert_eq!(db.referral_tier(5, 0).fee_discount_bps, 25);
    assert_eq!(db.insert_alert(alert(3)).unwrap(), 3);
    db.mutate(RelationalMutation::DeleteAlert(3)).unwrap();
    db.shutdown().unwrap();
    drop(db);
    let mut db = DurableRelationalStore::open(&dir, &registry).unwrap();
    assert_eq!(
        db.insert_alert(alert(4)).unwrap(),
        4,
        "deleted alert IDs must not be reused after restart"
    );
    db.mutate(RelationalMutation::DeleteApiKey("operator".into()))
        .unwrap();
    db.mutate(RelationalMutation::SetReferralTiers(vec![
        tier("basic", 0, 2),
        tier("pro", 5, 35),
    ]))
    .unwrap();
    drop(db); // All accepted mutations fsync before returning, even without shutdown.
    let mut db = DurableRelationalStore::open(&dir, &registry).unwrap();
    assert!(db.api_key("operator").is_none());
    assert!(db.api_key_by_hash("new-hash").is_none());
    assert!(db.api_keys_by_user(&[2; 20]).is_empty());
    assert_eq!(db.referral_tier(5, 0).fee_discount_bps, 35);
    assert!(db.alert(4).is_some());
    let last_snapshot = db.shutdown().unwrap();
    let relational = db.snapshot();
    drop(db);
    let mut bytes = fs::read(&last_snapshot).unwrap();
    let index = bytes.len() - 1;
    bytes[index] ^= 1;
    fs::write(&last_snapshot, bytes).unwrap();
    let recovered = DurableRelationalStore::open(&dir, &registry).unwrap();
    assert!(recovered.alert(4).is_some());
    assert_eq!(recovered.referral_tier(5, 0).fee_discount_bps, 35);
    assert!(recovered.api_key("operator").is_none());
    drop(recovered);
    let cfg = PerfDbConfig::new(root.join("complete"));
    let engine = PerfDb::open(cfg.clone()).unwrap();
    engine.save_checkpoint(42, "complete".into(), 100);
    engine.snapshot().unwrap();
    let state = snapshot::load_latest_snapshot(&cfg.snapshots_dir())
        .unwrap()
        .unwrap();
    drop(engine);
    snapshot::write_complete_snapshot(
        &cfg.snapshots_dir(),
        &CompleteStateSnapshot {
            version: 1,
            state,
            relational: Some(relational),
        },
    )
    .unwrap();
    let image = snapshot::load_latest_complete_snapshot(&cfg.snapshots_dir())
        .unwrap()
        .unwrap();
    let stores = image.relational.unwrap().restore(&registry).unwrap();
    assert!(stores.alerts.get(4).is_some());
    assert_eq!(stores.alerts.by_user(&[4; 20]).len(), 1);
    assert_eq!(stores.competitions.user_competitions(&[3; 20]), ["c1"]);
    assert_eq!(stores.referrals.compute_tier(5, 0).fee_discount_bps, 35);
}

fn main() {
    let args: Vec<_> = std::env::args().collect();
    let root = Path::new(&args[2]);
    match args[1].as_str() {
        "wal" => wal_case(root),
        "snapshot" => snapshot_case(root),
        "relational" => relational_case(root),
        other => panic!("unknown scenario {other}"),
    }
    println!("repair139:{}:ok", args[1]);
}
