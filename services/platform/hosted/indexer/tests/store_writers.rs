//! Separate indexer connections serialize writes to one durable file.

use std::sync::{Arc, Barrier};
use std::thread;
use std::time::{SystemTime, UNIX_EPOCH};

use layerx_indexer::store::Store;
use rusqlite::Connection;

#[test]
fn two_connections_record_every_write() {
    const ROUNDS: usize = 100;
    let nonce = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_else(|error| panic!("clock: {error:?}"))
        .as_nanos();
    let path = std::env::temp_dir().join(format!(
        "layerx-indexer-writers-{}-{nonce}.sqlite",
        std::process::id()
    ));
    let first =
        Store::open(&path).unwrap_or_else(|error| panic!("first indexer connection: {error:?}"));
    let second =
        Store::open(&path).unwrap_or_else(|error| panic!("second indexer connection: {error:?}"));
    let barrier = Arc::new(Barrier::new(2));
    let workers: Vec<_> =
        [first, second]
            .into_iter()
            .enumerate()
            .map(|(worker, store)| {
                let barrier = Arc::clone(&barrier);
                thread::spawn(move || {
                    for round in 0..ROUNDS {
                        barrier.wait();
                        store
                            .record_source_head(
                                &format!("writer-{worker}-round-{round}"),
                                Some(u64::try_from(round).unwrap_or_else(|error| {
                                    panic!("source head fits u64: {error:?}")
                                })),
                            )
                            .unwrap_or_else(|error| panic!("durable observation: {error:?}"));
                    }
                })
            })
            .collect();
    for worker in workers {
        worker
            .join()
            .unwrap_or_else(|error| panic!("writer thread: {error:?}"));
    }

    let connection =
        Connection::open(&path).unwrap_or_else(|error| panic!("read observations: {error:?}"));
    let count: i64 = connection
        .query_row("SELECT COUNT(*) FROM source_observations", [], |row| {
            row.get(0)
        })
        .unwrap_or_else(|error| panic!("observation count: {error:?}"));
    assert_eq!(
        count,
        i64::try_from(2 * ROUNDS)
            .unwrap_or_else(|error| panic!("observation count fits i64: {error:?}"))
    );
    for worker in 0..2 {
        for round in 0..ROUNDS {
            let head: i64 = connection
                .query_row(
                    "SELECT source_head FROM source_observations WHERE chain = ?1",
                    [format!("writer-{worker}-round-{round}")],
                    |row| row.get(0),
                )
                .unwrap_or_else(|error| panic!("recorded observation: {error:?}"));
            assert_eq!(
                head,
                i64::try_from(round)
                    .unwrap_or_else(|error| panic!("source head fits i64: {error:?}"))
            );
        }
    }
    drop(connection);
    std::fs::remove_file(&path).unwrap_or_else(|error| panic!("remove database: {error:?}"));
}
