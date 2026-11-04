use std::collections::HashSet;
use std::sync::{Arc, Barrier};

use perfdb::core::types::{Event, IndexedEvent, User};
use perfdb::error::PerfDbError;
use perfdb::relational::api_keys::{ApiKey, ApiKeyError, ApiKeyStore};
use perfdb::relational::checkpoint::CheckpointStore;
use perfdb::relational::referrals::{ReferralEngine, ReferralTier, ReferralTierError};
use perfdb::relational::users::{UserStore, UserStoreError};
use perfdb::storage::relational::DurableRelationalStore;
use perfdb::storage::snapshot::{self, SnapshotReferralTier};
use perfdb::storage::wal::{RelationalMutation, SyncPolicy};
use perfdb::{PerfDb, PerfDbConfig};

fn key(id: &str, owner: u8, hash: &str) -> ApiKey {
    ApiKey {
        id: id.into(),
        address: [owner; 20],
        key_hash: hash.into(),
        label: Some(format!("owner-{owner}")),
        rate_limit_rpm: u32::from(owner) * 60,
        last_used_at: None,
        expires_at: None,
        created_at: 100,
    }
}

fn user(owner: u8, code: Option<&str>) -> User {
    User {
        address: [owner; 20],
        vault_address: None,
        referral_code: code.map(str::to_owned),
        referred_by: None,
        tier: "bronze".into(),
        fee_discount_bps: 0,
        created_at: 100,
        first_trade_at: None,
        last_active_at: None,
    }
}

#[test]
fn api_key_replacement_revokes_old_secret_and_owner_aliases() {
    let store = ApiKeyStore::new();
    store.insert(key("rotated", 1, "old-secret-hash")).unwrap();
    store
        .insert(key("unrelated", 1, "other-secret-hash"))
        .unwrap();
    store.insert(key("rotated", 2, "new-secret-hash")).unwrap();
    store.insert(key("rotated", 2, "new-secret-hash")).unwrap();

    assert!(store.get_by_hash("old-secret-hash").is_none());
    let current = store.get_by_hash("new-secret-hash").unwrap();
    assert_eq!(current.address, [2; 20]);
    assert_eq!(current.rate_limit_rpm, 120);
    assert_eq!(current.label.as_deref(), Some("owner-2"));
    assert_eq!(
        store
            .by_user(&[1; 20])
            .iter()
            .map(|k| k.id.as_str())
            .collect::<Vec<_>>(),
        vec!["unrelated"]
    );
    assert_eq!(store.by_user(&[2; 20]).len(), 1);
    assert!(store.remove_by_user_and_id(&[1; 20], "rotated").is_none());
    assert!(store.touch_last_used("rotated", 200));
    assert_eq!(
        store.get_by_hash("new-secret-hash").unwrap().last_used_at,
        Some(200)
    );

    // The revoked hash can be explicitly assigned again without a later removal
    // of the rotated identity deleting its new owner's mapping.
    store.insert(key("fresh", 3, "old-secret-hash")).unwrap();
    assert_eq!(
        store
            .remove_by_user_and_id(&[2; 20], "rotated")
            .unwrap()
            .address,
        [2; 20]
    );
    assert!(store.get("rotated").is_none());
    assert!(store.get_by_hash("new-secret-hash").is_none());
    assert!(store.by_user(&[2; 20]).is_empty());
    assert_eq!(store.get_by_hash("old-secret-hash").unwrap().id, "fresh");
    assert_eq!(store.count(), 2);
}

#[test]
fn api_key_duplicate_hash_rejection_preserves_both_identities() {
    let store = ApiKeyStore::new();
    store.insert(key("first", 1, "hash-1")).unwrap();
    store.insert(key("second", 2, "hash-2")).unwrap();
    assert_eq!(
        store.insert(key("third", 3, "hash-1")),
        Err(ApiKeyError::DuplicateHash)
    );
    assert!(store.get("third").is_none());
    assert!(store.by_user(&[3; 20]).is_empty());
    let before = bincode::serialize(&store.get("second").unwrap()).unwrap();
    assert_eq!(
        store.insert(key("second", 3, "hash-1")),
        Err(ApiKeyError::DuplicateHash)
    );
    assert_eq!(
        bincode::serialize(&store.get("second").unwrap()).unwrap(),
        before
    );
    assert_eq!(store.get_by_hash("hash-1").unwrap().id, "first");
    assert_eq!(store.get_by_hash("hash-2").unwrap().id, "second");
    assert_eq!(store.by_user(&[1; 20]).len(), 1);
    assert_eq!(store.by_user(&[2; 20]).len(), 1);
    assert_eq!(store.count(), 2);
}

#[test]
fn api_key_concurrent_hash_claims_and_rotation_have_no_aliases() {
    let store = Arc::new(ApiKeyStore::new());
    let barrier = Arc::new(Barrier::new(16));
    let wins = std::thread::scope(|scope| {
        let handles: Vec<_> = (1..=16u8)
            .map(|owner| {
                let store = Arc::clone(&store);
                let barrier = Arc::clone(&barrier);
                scope.spawn(move || {
                    barrier.wait();
                    store.insert(key(&format!("key-{owner}"), owner, "shared-hash"))
                })
            })
            .collect();
        handles
            .into_iter()
            .map(|h| h.join().unwrap())
            .filter(Result::is_ok)
            .count()
    });
    assert_eq!(wins, 1);
    assert_eq!(store.count(), 1);
    let winner = store.get_by_hash("shared-hash").unwrap();
    assert_eq!(store.by_user(&winner.address).len(), 1);
    store.remove(&winner.id).unwrap();
    store.insert(key("rotating", 1, "hash-1")).unwrap();
    let barrier = Barrier::new(3);
    std::thread::scope(|scope| {
        scope.spawn(|| {
            barrier.wait();
            for i in 0..8_000 {
                let owner = 1 + (i % 2) as u8;
                store
                    .insert(key("rotating", owner, &format!("hash-{owner}")))
                    .unwrap();
            }
        });
        scope.spawn(|| {
            barrier.wait();
            for _ in 0..8_000 {
                for owner in [1, 2] {
                    let hash = format!("hash-{owner}");
                    if let Some(found) = store.get_by_hash(&hash) {
                        assert_eq!(found.key_hash, hash);
                        assert_eq!(found.address, [owner; 20]);
                    }
                    let keys = store.by_user(&[owner; 20]);
                    assert!(keys.iter().all(|k| k.address == [owner; 20]));
                    assert_eq!(
                        keys.iter().map(|k| &k.id).collect::<HashSet<_>>().len(),
                        keys.len()
                    );
                }
            }
        });
        scope.spawn(|| {
            barrier.wait();
            for _ in 0..4_000 {
                if let Some(removed) = store.remove_by_user_and_id(&[1; 20], "rotating") {
                    assert_eq!(removed.address, [1; 20]);
                    store.insert(key("rotating", 1, "hash-1")).unwrap();
                }
            }
        });
    });
    assert_eq!(store.count(), 1);
}

#[test]
fn user_referral_replacement_clears_only_the_current_owners_code() {
    let store = UserStore::new();
    store.upsert(user(1, Some("OLD"))).unwrap();
    store.upsert(user(1, Some("NEW"))).unwrap();
    assert!(store.by_referral_code("OLD").is_none());
    assert_eq!(store.by_referral_code("NEW").unwrap().address, [1; 20]);
    store.upsert(user(2, Some("OLD"))).unwrap();
    assert!(store.update(&[1; 20], |u| u.referral_code = Some("NEXT".into())));
    assert!(store.by_referral_code("NEW").is_none());
    assert_eq!(store.by_referral_code("OLD").unwrap().address, [2; 20]);
    assert!(store.set_referral_code(&[1; 20], String::new()));
    assert!(store.by_referral_code("NEXT").is_none());
    assert!(store.by_referral_code("").is_none());
    store.upsert(user(1, None)).unwrap();
    store.remove(&[1; 20]).unwrap();
    assert_eq!(store.by_referral_code("OLD").unwrap().address, [2; 20]);
    store.remove(&[2; 20]).unwrap();
    assert!(store.by_referral_code("OLD").is_none());
    assert_eq!(store.count(), 0);
}

#[test]
fn user_referral_collisions_reject_complete_updates() {
    let store = UserStore::new();
    store.upsert(user(1, Some("TAKEN"))).unwrap();
    store.upsert(user(2, Some("KEPT"))).unwrap();
    let before = bincode::serialize(&store.get(&[2; 20]).unwrap()).unwrap();
    assert_eq!(
        store.upsert(user(3, Some("TAKEN"))),
        Err(UserStoreError::DuplicateReferralCode)
    );
    assert!(!store.contains(&[3; 20]));
    assert_eq!(
        store.upsert(user(2, Some("TAKEN"))),
        Err(UserStoreError::DuplicateReferralCode)
    );
    assert!(!store.set_referral_code(&[2; 20], "TAKEN".into()));
    assert!(!store.update(&[2; 20], |u| {
        u.referral_code = Some("TAKEN".into());
        u.tier = "gold".into();
        u.fee_discount_bps = 123;
    }));
    assert_eq!(
        bincode::serialize(&store.get(&[2; 20]).unwrap()).unwrap(),
        before
    );
    assert!(!store.update(&[2; 20], |u| u.address = [1; 20]));
    assert_eq!(
        bincode::serialize(&store.get(&[2; 20]).unwrap()).unwrap(),
        before
    );
    assert_eq!(store.by_referral_code("TAKEN").unwrap().address, [1; 20]);
    assert_eq!(store.by_referral_code("KEPT").unwrap().address, [2; 20]);
    store.remove(&[2; 20]).unwrap();
    assert_eq!(store.by_referral_code("TAKEN").unwrap().address, [1; 20]);
    assert_eq!(store.count(), 1);
}

#[test]
fn user_referral_concurrent_claims_and_reads_keep_unique_current_owners() {
    let store = Arc::new(UserStore::new());
    let barrier = Arc::new(Barrier::new(16));
    let wins = std::thread::scope(|scope| {
        let handles: Vec<_> = (1..=16u8)
            .map(|owner| {
                let store = Arc::clone(&store);
                let barrier = Arc::clone(&barrier);
                scope.spawn(move || {
                    barrier.wait();
                    store.upsert(user(owner, Some("SHARED")))
                })
            })
            .collect();
        handles
            .into_iter()
            .map(|h| h.join().unwrap())
            .filter(Result::is_ok)
            .count()
    });
    assert_eq!(wins, 1);
    assert_eq!(store.count(), 1);
    let owner = store.by_referral_code("SHARED").unwrap().address;
    let barrier = Barrier::new(2);
    std::thread::scope(|scope| {
        scope.spawn(|| {
            barrier.wait();
            for i in 0..8_000 {
                let code = if i % 2 == 0 { "FIRST" } else { "SECOND" };
                assert!(store.set_referral_code(&owner, code.into()));
            }
        });
        scope.spawn(|| {
            barrier.wait();
            for _ in 0..8_000 {
                for code in ["SHARED", "FIRST", "SECOND"] {
                    if let Some(found) = store.by_referral_code(code) {
                        assert_eq!(found.address, owner);
                        assert_eq!(found.referral_code.as_deref(), Some(code));
                    }
                }
            }
        });
    });
    assert!(store.by_referral_code("SHARED").is_none());
    assert!(store.by_referral_code("FIRST").is_none());
    assert_eq!(store.by_referral_code("SECOND").unwrap().address, owner);
}

fn tier(name: &'static str, count: u32, volume: i128, reward: u32, discount: u32) -> ReferralTier {
    ReferralTier {
        tier: name.into(),
        min_referrals: count,
        min_volume: volume,
        reward_bps: reward,
        fee_discount_bps: discount,
    }
}

fn tier_table(engine: &ReferralEngine) -> Vec<(String, u32, i128, u32, u32)> {
    engine
        .tiers()
        .iter()
        .map(|t| {
            (
                t.tier.clone(),
                t.min_referrals,
                t.min_volume,
                t.reward_bps,
                t.fee_discount_bps,
            )
        })
        .collect()
}

#[test]
fn referral_tier_invalid_configuration_returns_typed_errors_and_preserves_table() {
    use ReferralTierError::*;
    let base = tier("base", 0, 0, 5, 0);
    let invalid = vec![
        (vec![], Empty),
        (vec![tier("base", 1, 0, 5, 0)], MissingBase),
        (vec![tier("base", 0, 1, 5, 0)], MissingBase),
        (vec![tier("base", 0, -1, 5, 0)], MissingBase),
        (
            vec![base.clone(), tier("bad", 1, -1, 5, 0)],
            UnorderedThresholds { index: 1 },
        ),
        (
            vec![base.clone(), tier("same", 0, 0, 5, 0)],
            UnorderedThresholds { index: 1 },
        ),
        (
            vec![
                base.clone(),
                tier("high", 10, 100, 10, 5),
                tier("bad", 9, 200, 15, 10),
            ],
            UnorderedThresholds { index: 2 },
        ),
        (
            vec![
                base.clone(),
                tier("high", 10, 100, 10, 5),
                tier("bad", 20, 99, 15, 10),
            ],
            UnorderedThresholds { index: 2 },
        ),
        (
            vec![tier("base", 0, 0, 10_001, 0)],
            InvalidBasisPoints { index: 0 },
        ),
        (
            vec![tier("base", 0, 0, 5, 10_001)],
            InvalidBasisPoints { index: 0 },
        ),
        (
            vec![base.clone(), tier("bad", 1, 100, u32::MAX, 0)],
            InvalidBasisPoints { index: 1 },
        ),
        (
            vec![base, tier("bad", 1, 100, 5, u32::MAX)],
            InvalidBasisPoints { index: 1 },
        ),
    ];
    let mut engine = ReferralEngine::new();
    let before = tier_table(&engine);
    for (table, error) in invalid {
        assert_eq!(
            ReferralEngine::with_tiers(table.clone()).err(),
            Some(error.clone())
        );
        assert_eq!(engine.set_tiers(table), Err(error));
        assert_eq!(tier_table(&engine), before);
        assert_eq!(engine.compute_tier(0, 0).tier, "bronze");
        assert_eq!(engine.fee_discount_bps(0, 0), 0);
        assert_eq!(engine.reward_bps(0, 0), 5);
        assert_eq!(engine.compute_tier(u32::MAX, i128::MAX).tier, "platinum");
    }
}

#[test]
fn referral_tier_valid_configuration_drives_thresholds_and_basis_point_limits() {
    let tiers = vec![
        tier("base", 0, 0, 0, 0),
        tier("volume", 0, 100, 500, 250),
        tier("count", 5, 100, 10_000, 10_000),
    ];
    let mut engine = ReferralEngine::with_tiers(tiers.clone()).unwrap();
    for (count, volume, name, reward, discount) in [
        (0, 0, "base", 0, 0),
        (0, 99, "base", 0, 0),
        (0, 100, "volume", 500, 250),
        (4, i128::MAX, "volume", 500, 250),
        (5, 99, "base", 0, 0),
        (5, 100, "count", 10_000, 10_000),
        (u32::MAX, i128::MAX, "count", 10_000, 10_000),
    ] {
        assert_eq!(engine.compute_tier(count, volume).tier, name);
        assert_eq!(engine.reward_bps(count, volume), reward);
        assert_eq!(engine.fee_discount_bps(count, volume), discount);
    }
    engine
        .set_tiers(vec![tier("new-base", 0, 0, 7, 3)])
        .unwrap();
    assert_eq!(engine.tier_count(), 1);
    assert_eq!(engine.compute_tier(u32::MAX, i128::MAX).tier, "new-base");
    let last_valid = tier_table(&engine);
    assert_eq!(engine.set_tiers(vec![]), Err(ReferralTierError::Empty));
    assert_eq!(tier_table(&engine), last_valid);
    engine.set_tiers(tiers).unwrap();
    assert_eq!(engine.fee_discount_bps(5, 100), 10_000);
}

fn assert_checkpoint_tuple(block: u64, hash: &str, timestamp: u64) {
    assert_eq!(hash, format!("canonical-{block:016x}"));
    assert_eq!(timestamp, block * 12);
}

#[test]
fn checkpoint_concurrent_saves_publish_one_complete_tuple() {
    let cp = CheckpointStore::new(1, "canonical-0000000000000001".into(), 12);
    let barrier = Barrier::new(8);
    std::thread::scope(|scope| {
        for writer in 0..4u64 {
            let cp = &cp;
            let barrier = &barrier;
            scope.spawn(move || {
                barrier.wait();
                for offset in 0..10_000 {
                    let block = 2 + writer * 10_000 + offset;
                    cp.save(block, format!("canonical-{block:016x}"), block * 12);
                }
            });
        }
        for _ in 0..4 {
            let cp = &cp;
            let barrier = &barrier;
            scope.spawn(move || {
                barrier.wait();
                for _ in 0..10_000 {
                    let observed = cp.snapshot();
                    assert_checkpoint_tuple(
                        observed.last_block,
                        &observed.last_block_hash,
                        observed.last_block_timestamp,
                    );
                }
            });
        }
    });
    let observed = cp.snapshot();
    assert_checkpoint_tuple(
        observed.last_block,
        &observed.last_block_hash,
        observed.last_block_timestamp,
    );
}

fn deposit(sequence: u64) -> IndexedEvent {
    let mut tx_hash = [0; 32];
    tx_hash[..8].copy_from_slice(&sequence.to_le_bytes());
    IndexedEvent {
        block_number: 1,
        block_timestamp: 12,
        tx_hash,
        tx_index: sequence as u32,
        log_index: 0,
        event: Event::CollateralDeposited {
            user: [1; 20],
            token: [2; 20],
            amount: 7,
        },
    }
}

#[test]
fn checkpoint_engine_snapshot_event_boundary_and_reopen_recover_complete_state() {
    let dir = tempfile::tempdir().unwrap();
    let mut config = PerfDbConfig::new(dir.path());
    config.wal.sync_policy = SyncPolicy::EveryWrite;
    config.wal.max_segment_size = 1024;
    let db = PerfDb::open(config.clone()).unwrap();
    db.save_checkpoint(1, "canonical-0000000000000001".into(), 12);
    let barrier = Barrier::new(3);
    std::thread::scope(|scope| {
        scope.spawn(|| {
            barrier.wait();
            for sequence in 1..=48 {
                db.process_event(&deposit(sequence)).unwrap();
            }
        });
        scope.spawn(|| {
            barrier.wait();
            for block in 2..=49 {
                db.save_checkpoint(block, format!("canonical-{block:016x}"), block * 12);
            }
        });
        scope.spawn(|| {
            barrier.wait();
            for _ in 0..16 {
                db.snapshot().unwrap();
                let (state, recovery) =
                    snapshot::load_latest_engine_snapshot(&config.snapshots_dir())
                        .unwrap()
                        .unwrap();
                let cp = state.checkpoint;
                assert_checkpoint_tuple(
                    cp.last_block,
                    &cp.last_block_hash,
                    cp.last_block_timestamp,
                );
                assert_eq!(state.snapshot_timestamp, cp.last_block_timestamp);
                assert!(
                    recovery
                        .events
                        .iter()
                        .all(|e| matches!(e.event, Event::CollateralDeposited { amount: 7, .. }))
                );
                let balance: i128 = state.balances.iter().map(|b| b.amount).sum();
                assert_eq!(balance, recovery.events.len() as i128 * 7);
            }
        });
    });
    db.snapshot().unwrap();
    let expected = db.checkpoint.snapshot();
    assert_eq!(expected.last_block, 49);
    db.process_event(&deposit(49)).unwrap();
    db.sync_wal().unwrap();
    drop(db);

    let db = PerfDb::open(config.clone()).unwrap();
    let recovered = db.checkpoint.snapshot();
    assert_eq!(recovered.last_block, expected.last_block);
    assert_eq!(recovered.last_block_hash, expected.last_block_hash);
    assert_eq!(
        recovered.last_block_timestamp,
        expected.last_block_timestamp
    );
    assert_eq!(db.balances.get(&[1; 20], &[2; 20]).unwrap().amount, 49 * 7);
    assert_eq!(db.events.total_count(), 49);
    db.process_event(&deposit(49)).unwrap();
    assert_eq!(db.events.total_count(), 49);
    assert_eq!(db.balances.get(&[1; 20], &[2; 20]).unwrap().amount, 49 * 7);
    db.snapshot().unwrap();
    drop(db);
    let db = PerfDb::open(config).unwrap();
    let recovered = db.checkpoint.snapshot();
    assert_checkpoint_tuple(
        recovered.last_block,
        &recovered.last_block_hash,
        recovered.last_block_timestamp,
    );
    assert_eq!(db.events.total_count(), 49);
    assert_eq!(db.balances.get(&[1; 20], &[2; 20]).unwrap().amount, 49 * 7);
}

#[test]
fn referral_tier_durable_boundary_rejects_invalid_tables_before_wal_and_recovers_valid_table() {
    let dir = tempfile::tempdir().unwrap();
    let names = ["base", "volume", "count"];
    let valid = vec![
        tier("base", 0, 0, 0, 0),
        tier("volume", 0, 100, 500, 250),
        tier("count", 5, 100, 10_000, 10_000),
    ];
    let mut store = DurableRelationalStore::open(dir.path(), &names).unwrap();
    store
        .mutate(RelationalMutation::SetReferralTiers(
            valid.iter().map(SnapshotReferralTier::from).collect(),
        ))
        .unwrap();
    let before = bincode::serialize(&store.snapshot()).unwrap();
    for invalid in [
        vec![],
        vec![tier("base", 1, 0, 0, 0)],
        vec![tier("base", 0, 0, 10_001, 0)],
        vec![tier("base", 0, 0, 0, 10_001)],
        vec![tier("base", 0, 0, 0, 0), tier("volume", 0, 0, 1, 1)],
        vec![tier("base", 0, 0, 0, 0), tier("volume", 1, -1, 1, 1)],
    ] {
        assert!(
            store
                .mutate(RelationalMutation::SetReferralTiers(
                    invalid.iter().map(SnapshotReferralTier::from).collect(),
                ))
                .is_err()
        );
        assert_eq!(bincode::serialize(&store.snapshot()).unwrap(), before);
        assert_eq!(store.referral_tier(5, 100).tier, "count");
    }
    // Reopen from the WAL alone: no rejected table may have been appended.
    drop(store);
    let mut store = DurableRelationalStore::open(dir.path(), &names).unwrap();
    assert_eq!(bincode::serialize(&store.snapshot()).unwrap(), before);
    assert_eq!(store.referral_tier(0, 0).tier, "base");
    assert_eq!(store.referral_tier(0, 100).tier, "volume");
    assert_eq!(store.referral_tier(5, 100).fee_discount_bps, 10_000);
    store.shutdown().unwrap();
    drop(store);
    // The snapshot restoration call site also consumes the validated Result.
    let store = DurableRelationalStore::open(dir.path(), &names).unwrap();
    assert_eq!(bincode::serialize(&store.snapshot()).unwrap(), before);
    assert_eq!(store.referral_tier(5, 100).reward_bps, 10_000);

    let mut invalid_snapshot = store.snapshot();
    invalid_snapshot.referral_tiers[0].min_referrals = 1;
    assert!(matches!(
        invalid_snapshot.restore(&names),
        Err(PerfDbError::Corruption(_))
    ));
}

#[test]
fn api_key_durable_rotation_and_duplicate_rejection_survive_wal_and_snapshot_recovery() {
    let dir = tempfile::tempdir().unwrap();
    let mut store = DurableRelationalStore::open(dir.path(), &[]).unwrap();
    store
        .mutate(RelationalMutation::PutApiKey(key("rotated", 1, "old")))
        .unwrap();
    store
        .mutate(RelationalMutation::PutApiKey(key("rotated", 2, "new")))
        .unwrap();
    let before = bincode::serialize(&store.snapshot()).unwrap();
    assert!(matches!(
        store.mutate(RelationalMutation::PutApiKey(key("intruder", 3, "new"))),
        Err(PerfDbError::InvalidArgument(_))
    ));
    assert_eq!(bincode::serialize(&store.snapshot()).unwrap(), before);
    drop(store);
    let mut store = DurableRelationalStore::open(dir.path(), &[]).unwrap();
    assert!(store.api_key_by_hash("old").is_none());
    assert!(store.api_keys_by_user(&[1; 20]).is_empty());
    assert_eq!(store.api_key_by_hash("new").unwrap().address, [2; 20]);
    assert_eq!(store.api_keys_by_user(&[2; 20]).len(), 1);
    assert_eq!(bincode::serialize(&store.snapshot()).unwrap(), before);
    store.shutdown().unwrap();
    drop(store);
    let store = DurableRelationalStore::open(dir.path(), &[]).unwrap();
    assert!(store.api_key_by_hash("old").is_none());
    assert!(store.api_keys_by_user(&[1; 20]).is_empty());
    assert_eq!(store.api_key_by_hash("new").unwrap().address, [2; 20]);
    assert!(store.api_key("intruder").is_none());
    assert_eq!(store.api_keys_by_user(&[2; 20]).len(), 1);
}

#[test]
fn user_referral_current_owners_survive_engine_snapshot_recovery() {
    let dir = tempfile::tempdir().unwrap();
    let config = PerfDbConfig::new(dir.path());
    let db = PerfDb::open(config.clone()).unwrap();
    db.users.upsert(user(1, Some("OLD"))).unwrap();
    db.users.upsert(user(1, Some("NEW"))).unwrap();
    db.users.upsert(user(2, Some("OLD"))).unwrap();
    assert_eq!(
        db.users.upsert(user(2, Some("NEW"))),
        Err(UserStoreError::DuplicateReferralCode)
    );
    db.snapshot().unwrap();
    drop(db);
    let db = PerfDb::open(config).unwrap();
    assert_eq!(db.users.count(), 2);
    assert_eq!(db.users.by_referral_code("OLD").unwrap().address, [2; 20]);
    assert_eq!(db.users.by_referral_code("NEW").unwrap().address, [1; 20]);
    assert!(db.users.set_referral_code(&[1; 20], "NEXT".into()));
    assert!(db.users.by_referral_code("NEW").is_none());
    assert_eq!(db.users.by_referral_code("OLD").unwrap().address, [2; 20]);
}
