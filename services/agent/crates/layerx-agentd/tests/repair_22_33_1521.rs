//! Repair record 22.33.1521 / audit 22.30.92; verified revision is in the gate ledger.
//! history_range: history_range_seeks_pages_and_stops_without_splitting_positions;
//! history_range_enforces_bytes_and_validates_discarded_rows.
//! grant_fixture: grant_fixture_binds_native_authority_identity_scope_and_revocation.
//! Baseline verify was not run: the explicit gate order permits verification only after implementation/build.
//! Wiring request (domain 9): precompiles/layerxauth/views_test.go must publish a committed
//! keeper-created getGrant/capabilityScope transcript; this test reuses the committed native authority vector.

use ed25519_dalek::{Signer as _, SigningKey};
use layerx_agentd::read::committed::CommittedReadError;
use layerx_agentd::read::{
    checkpoint, validator_set_hash, BlockCommit, BlockId, CheckpointHeader, CommitTimestamp,
    CommitValidator, CommitVote, CommittedReads, ServedCheckpoint,
};
use layerx_agentd::read::{Cursor, HistoryLimits, HistoryPage, HistoryReadError};
use layerx_client::sequencer::SequencerEndpoint;
use layerx_crypto::authority_grant::AuthorityGrant;
use serde_json::{json, Value};
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::TcpListener;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;

struct Loopback {
    endpoint: SequencerEndpoint,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<usize>>,
}

impl Loopback {
    fn new(mut reply: impl FnMut(Value) -> Value + Send + 'static) -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").expect("loopback listener");
        listener
            .set_nonblocking(true)
            .expect("nonblocking listener");
        let endpoint =
            SequencerEndpoint::new(&format!("http://{}", listener.local_addr().unwrap()), &[])
                .unwrap();
        let stop = Arc::new(AtomicBool::new(false));
        let stopping = stop.clone();
        let worker = thread::spawn(move || {
            let mut count = 0;
            while !stopping.load(Ordering::SeqCst) {
                let (stream, _) = match listener.accept() {
                    Ok(stream) => stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap();
                let mut reader = BufReader::new(stream);
                let mut length = None;
                loop {
                    let mut line = String::new();
                    assert_ne!(reader.read_line(&mut line).unwrap(), 0);
                    if line == "\r\n" {
                        break;
                    }
                    if let Some((name, value)) = line.split_once(':') {
                        if name.eq_ignore_ascii_case("content-length") {
                            length = Some(value.trim().parse::<usize>().unwrap());
                        }
                    }
                }
                let mut body = vec![0; length.expect("HTTP content length")];
                reader.read_exact(&mut body).unwrap();
                let request: Value = serde_json::from_slice(&body).unwrap();
                let id = request["id"].clone();
                let body = json!({"jsonrpc":"2.0", "id":id, "result":reply(request)}).to_string();
                write!(reader.into_inner(), "HTTP/1.1 200 OK\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n{}", body.len(), body).unwrap();
                count += 1;
            }
            count
        });
        Self {
            endpoint,
            stop,
            worker: Some(worker),
        }
    }

    fn finish(mut self) -> usize {
        self.stop.store(true, Ordering::SeqCst);
        self.worker.take().unwrap().join().expect("loopback server")
    }
}

impl Drop for Loopback {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        if let Some(worker) = self.worker.take() {
            let result = worker.join();
            if !thread::panicking() {
                result.expect("loopback server");
            }
        }
    }
}

fn position(height: u64, tx_index: u32, log_index: u32) -> Cursor {
    Cursor {
        block_height: height,
        tx_index,
        log_index,
    }
}

fn row(height: u64, name: &str) -> Value {
    json!({"blockHeight":height, "txIndex":1, "logIndex":0, "name":name})
}

fn row_position(row: &Value) -> Cursor {
    position(
        row["blockHeight"].as_u64().unwrap(),
        row["txIndex"].as_u64().unwrap() as u32,
        row["logIndex"].as_u64().unwrap() as u32,
    )
}

fn freshness() -> Value {
    let transcript: Value = serde_json::from_str(include_str!(
        "../../../../../apps/web/e2e/fixtures/px-get-unified-history.json"
    ))
    .unwrap();
    transcript["result"]["freshness"].clone()
}

fn history_server(rows: Vec<Value>) -> Loopback {
    Loopback::new(move |request| {
        assert_eq!(request["method"], "px_getHistory");
        assert_eq!(request["params"][0], "account");
        let after = request["params"][1]
            .as_str()
            .map(|text| Cursor::from_hex(text).unwrap());
        let limit = request["params"][2].as_u64().unwrap() as usize;
        let remaining: Vec<_> = rows
            .iter()
            .filter(|row| after.is_none_or(|after| row_position(row) > after))
            .collect();
        let mut end = limit.min(remaining.len());
        while end > 0
            && end < remaining.len()
            && row_position(remaining[end]) == row_position(remaining[end - 1])
        {
            end += 1;
        }
        let next = (end < remaining.len()).then(|| row_position(remaining[end - 1]).to_hex());
        json!({"account":"account", "items":remaining[..end], "next_cursor":next,
            "indexedHeight":20, "chainHeight":22, "freshness":freshness()})
    })
}

fn limits(items: u16, bytes: usize) -> HistoryLimits {
    HistoryLimits {
        maximum_items: items,
        maximum_response_bytes: bytes,
    }
}

#[test]
fn history_range_seeks_pages_and_stops_without_splitting_positions() {
    let server = history_server(vec![
        row(0, "old-zero"),
        row(2, "old"),
        row(10, "first-a"),
        row(10, "first-b"),
        row(11, "last"),
        row(15, "new"),
    ]);
    let mut cursor = None;
    let mut names = Vec::new();
    for page_number in 0..3 {
        let page =
            HistoryPage::read_account(&server.endpoint, "account", 10, 11, cursor, limits(1, 4096))
                .unwrap();
        assert_eq!(page.height, 20);
        assert!(page.response_bytes <= 4096);
        if page_number == 0 {
            assert_eq!(
                page.items.len(),
                2,
                "whole first position must survive the row limit"
            );
            assert_eq!(page.next_cursor, Some(position(10, 1, 0)));
        }
        if page_number == 1 {
            assert_eq!(page.next_cursor, Some(position(11, 1, 0)));
        }
        for item in page.items {
            assert!((10..=11).contains(&item.position.block_height));
            let row: Value = serde_json::from_slice(&item.canonical).unwrap();
            names.push(row["name"].as_str().unwrap().to_owned());
        }
        cursor = page.next_cursor;
        if page_number == 2 {
            assert_eq!(cursor, None);
        }
    }
    assert_eq!(names, ["first-a", "first-b", "last"]);
    assert_eq!(server.finish(), 3);

    let server = history_server(vec![
        row(0, "zero"),
        row(2, "old"),
        row(10, "selected"),
        row(15, "new"),
    ]);
    let page =
        HistoryPage::read_account(&server.endpoint, "account", 10, 11, None, limits(4, 4096))
            .unwrap();
    assert_eq!(page.items.len(), 1);
    assert_eq!(page.next_cursor, None);
    assert_eq!(server.finish(), 1);

    let server = history_server(vec![row(0, "zero"), row(2, "old"), row(15, "new")]);
    let page =
        HistoryPage::read_account(&server.endpoint, "account", 10, 11, None, limits(4, 4096))
            .unwrap();
    assert!(page.items.is_empty());
    assert_eq!(page.next_cursor, None);
    let zero = HistoryPage::read_account(&server.endpoint, "account", 0, 0, None, limits(4, 4096))
        .unwrap();
    assert_eq!(zero.items.len(), 1);
    assert_eq!(zero.items[0].position.block_height, 0);
    assert_eq!(zero.next_cursor, None);
    assert_eq!(server.finish(), 2);
}

#[test]
fn history_range_enforces_bytes_and_validates_discarded_rows() {
    let server = history_server(vec![row(10, "selected"), row(15, &"large".repeat(2000))]);
    assert_eq!(
        HistoryPage::read_account(&server.endpoint, "account", 10, 11, None, limits(4, 4096)),
        Err(HistoryReadError::Oversized)
    );
    assert_eq!(server.finish(), 1);

    let server = Loopback::new(|_| {
        json!({"account":"account", "items":[row(15,"new"),row(10,"out-of-order")],
        "next_cursor":null, "indexedHeight":20,"chainHeight":22,"freshness":freshness()})
    });
    assert_eq!(
        HistoryPage::read_account(&server.endpoint, "account", 10, 11, None, limits(4, 4096)),
        Err(HistoryReadError::Cursor)
    );
    assert_eq!(server.finish(), 1);

    let server = Loopback::new(|_| {
        json!({"account":"account", "items":[row(10,"selected")],
        "next_cursor":position(9,1,0).to_hex(),"indexedHeight":20,"chainHeight":22,"freshness":freshness()})
    });
    assert_eq!(
        HistoryPage::read_account(&server.endpoint, "account", 10, 11, None, limits(4, 4096)),
        Err(HistoryReadError::Cursor)
    );
    assert_eq!(server.finish(), 1);
}

fn word(n: u128) -> [u8; 32] {
    let mut bytes = [0; 32];
    bytes[16..].copy_from_slice(&n.to_be_bytes());
    bytes
}

fn grant_result(bytes: &[u8], revoked: bool, revoked_at: u64) -> Vec<u8> {
    let mut result = [
        word(96),
        word(u128::from(revoked)),
        word(u128::from(revoked_at)),
        word(bytes.len() as u128),
    ]
    .concat();
    result.extend(bytes);
    result.resize(result.len().div_ceil(32) * 32, 0);
    result
}

fn scope_result(grant: AuthorityGrant) -> Vec<u8> {
    let s = grant.scope;
    [
        word(s.module_mask.into()),
        word(s.ordinal_min.into()),
        word(s.ordinal_max.into()),
        s.asset,
        word(s.maximum_per_activity),
        word(s.maximum_total),
        word(0),
        word(s.period_length.into()),
        word(s.maximum_per_period),
        word(0),
        word(s.period_start.into()),
        s.purpose,
    ]
    .concat()
}

fn block(checkpoint: &ServedCheckpoint) -> Value {
    let h = |bytes: &[u8]| format!("0x{}", hex::encode(bytes));
    json!({"number":format!("{:#x}",checkpoint.height),"hash":h(&checkpoint.block_hash),
        "parentHash":h(&[1;32]),"timestamp":format!("{:#x}",checkpoint.header.time.seconds),
        "transactionsRoot":h(&[4;32]),"receiptsRoot":h(&[7;32]),"stateRoot":h(&checkpoint.app_hash),"transactions":[]})
}

fn signed_checkpoint(height: u64, seconds: i64, nanos: u32) -> ServedCheckpoint {
    let keys: Vec<_> = [1, 2, 3]
        .map(|seed| SigningKey::from_bytes(&[seed; 32]))
        .into();
    let validators: Vec<_> = keys
        .iter()
        .zip([4, 3, 2])
        .map(|(key, voting_power)| {
            let public_key = key.verifying_key().to_bytes();
            CommitValidator {
                address: CommitValidator::address_for_key(&public_key),
                public_key,
                voting_power,
            }
        })
        .collect();
    let header = CheckpointHeader {
        version_block: 11,
        version_app: 1,
        chain_id: "125".into(),
        height,
        time: CommitTimestamp { seconds, nanos },
        last_block_id: Some(BlockId {
            hash: [1; 32],
            part_set_total: 1,
            part_set_hash: [2; 32],
        }),
        last_commit_hash: Some([3; 32]),
        data_hash: Some([4; 32]),
        validators_hash: validator_set_hash(&validators),
        next_validators_hash: validator_set_hash(&validators),
        consensus_hash: [5; 32],
        app_hash: [6; 32],
        last_results_hash: Some([7; 32]),
        evidence_hash: Some([8; 32]),
        proposer_address: validators[0].address,
    };
    let mut commit = BlockCommit {
        height,
        round: 1,
        block_id: BlockId {
            hash: header.block_hash(),
            part_set_total: 1,
            part_set_hash: [9; 32],
        },
        votes: Vec::new(),
    };
    for (key, validator) in keys.iter().zip(&validators).take(2) {
        let timestamp = CommitTimestamp { seconds, nanos };
        commit.votes.push(CommitVote {
            validator_address: validator.address,
            voting_power: validator.voting_power,
            timestamp,
            signature: key
                .sign(&commit.vote_sign_bytes(&header.chain_id, timestamp))
                .to_bytes(),
        });
    }
    checkpoint(height, "125", &header, &commit, &validators).expect("signed block commit")
}

#[test]
fn grant_fixture_binds_native_authority_identity_scope_and_revocation() {
    let canonical = include_bytes!(
        "../../../../../tests/fixtures/authority/native-fee-grants/capability/grant.bin"
    );
    let id = *include_bytes!(
        "../../../../../tests/fixtures/authority/native-fee-grants/capability/grant.id"
    );
    let native = AuthorityGrant::decode(canonical).expect("native authority producer vector");
    assert_eq!(native.encode().unwrap(), canonical);
    for mutation in 0..8 {
        let checkpoint = signed_checkpoint(8, 1_700_000_000, 123_000_000);
        let mut bytes = canonical.to_vec();
        let mut scope = scope_result(native);
        let (revoked, revoked_at) = match mutation {
            1 => {
                bytes[9] ^= 1;
                (false, 0)
            }
            2 => {
                bytes[45] ^= 1;
                (false, 0)
            }
            3 => {
                bytes[318] = 1;
                bytes[326] = 7;
                (true, 7)
            }
            4 => {
                scope[4 * 32 + 31] ^= 1;
                (false, 0)
            }
            5 => (true, 7),
            6 => {
                bytes = vec![1, 2, 3];
                (false, 0)
            }
            _ => (false, 0),
        };
        let grant = grant_result(&bytes, revoked, revoked_at);
        let served_checkpoint = checkpoint.clone();
        let server = Loopback::new(move |request| match request["method"].as_str().unwrap() {
            "eth_call" => {
                assert_eq!(request["params"][1], "0x8");
                assert_eq!(
                    request["params"][0]["to"],
                    "0x000000000000000000000000000000000000101c"
                );
                let data = hex::decode(
                    request["params"][0]["data"]
                        .as_str()
                        .unwrap()
                        .strip_prefix("0x")
                        .unwrap(),
                )
                .unwrap();
                assert_eq!(&data[4..], id);
                let get = layerx_crypto::evm_transaction::transaction_hash(b"getGrant(bytes32)");
                let cap =
                    layerx_crypto::evm_transaction::transaction_hash(b"capabilityScope(bytes32)");
                let answer = if data[..4] == get[..4] {
                    &grant
                } else {
                    assert_eq!(data[..4], cap[..4]);
                    &scope
                };
                json!(format!("0x{}", hex::encode(answer)))
            }
            "eth_getBlockByNumber" => {
                assert_eq!(request["params"][0], "0x8");
                block(&served_checkpoint)
            }
            method => panic!("unexpected method {method}"),
        });
        let reads = CommittedReads::at_checkpoint(&server.endpoint, &checkpoint).unwrap();
        let owner = if mutation == 7 {
            [9; 32]
        } else {
            native.grantor
        };
        let result = reads.grant(id, owner);
        if mutation == 0 {
            let result = result.expect("native authority grant admitted");
            assert_eq!(result.encoding, canonical);
            assert_eq!(result.id, id);
            assert_eq!(
                result.maximum_per_activity,
                native.scope.maximum_per_activity
            );
            assert_eq!(result.purpose, native.scope.purpose);
            assert!(!result.revoked);
        } else {
            assert!(
                matches!(result, Err(CommittedReadError::Selector)),
                "mutation {mutation}: {result:?}"
            );
        }
        assert_eq!(
            server.finish(),
            8,
            "two views must each be repeated and checkpoint-checked"
        );
    }
}
