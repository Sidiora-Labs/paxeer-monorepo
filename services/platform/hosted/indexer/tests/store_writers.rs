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
        .expect("clock")
        .as_nanos();
    let path = std::env::temp_dir().join(format!(
        "layerx-indexer-writers-{}-{nonce}.sqlite",
        std::process::id()
    ));
    let first = Store::open(&path).expect("first indexer connection");
    let second = Store::open(&path).expect("second indexer connection");
    let barrier = Arc::new(Barrier::new(2));
    let workers: Vec<_> = [first, second]
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
                            Some(round as u64),
                        )
                        .expect("durable observation");
                }
            })
        })
        .collect();
    for worker in workers {
        worker.join().expect("writer thread");
    }

    let connection = Connection::open(&path).expect("read observations");
    let count: i64 = connection
        .query_row("SELECT COUNT(*) FROM source_observations", [], |row| {
            row.get(0)
        })
        .expect("observation count");
    assert_eq!(count, (2 * ROUNDS) as i64);
    for worker in 0..2 {
        for round in 0..ROUNDS {
            let head: i64 = connection
                .query_row(
                    "SELECT source_head FROM source_observations WHERE chain = ?1",
                    [format!("writer-{worker}-round-{round}")],
                    |row| row.get(0),
                )
                .expect("recorded observation");
            assert_eq!(head, round as i64);
        }
    }
    drop(connection);
    std::fs::remove_file(&path).expect("remove database");
}
