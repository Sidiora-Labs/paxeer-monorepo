//! Task 22.33.1375 repair record (audit 22.30.75, owner domain 8).
//! numeric-parse-zero-fallback: numeric_parse_zero_fallback_recorded_ingestion;
//! numeric_parse_supported_signed_width.
//! identifier-parse-validation: identifier_parse_validation_recorded_ingestion;
//! identifier_parse_exact_ascii_width_without_panics; identifier_parse_validation_cli.
//! Baseline verification was not run: the ordered gate instructions require
//! complete implementation before the first build or verification.
//! Attempt 1 build failed with E0308 in main.rs; verification was not reached.
//! CLI callers now propagate parser errors before querying the real store.
//! Ingestion accepts typed events; the recorded text producer below propagates
//! each parse error before encoding, delivery, or database mutation.
//! No out-of-path wiring is required.

use std::collections::HashMap;
use std::error::Error;

use perfdb::core::types::{
    parse_address, parse_fixed, parse_hash, Event, IndexedEvent, ParseFixedError,
    ParseIdentifierError,
};
use perfdb::storage::wal::{self, SyncPolicy};
use perfdb::{PerfDb, PerfDbConfig};

const USER: &str = "0x00000000000000000000000000000000000000a1";
const POSITION: &str = "0x0000000000000000000000000000000000000000000000000000000000000007";
const TOKEN: [u8; 20] = [0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 195];

fn config(path: &std::path::Path) -> PerfDbConfig {
    let mut config = PerfDbConfig::new(path);
    config.wal.sync_policy = SyncPolicy::EveryWrite;
    config
}

// The peers are the committed chain transcript and the real binary codec/store.
// Only external text fields are replaced, using the production parsers under repair.
fn ingest_recorded(
    db: &PerfDb,
    quantity: &str,
    user_text: &str,
    position_text: &str,
    block: u64,
) -> Result<(), Box<dyn Error>> {
    let quantity = parse_fixed(quantity)?;
    let user = parse_address(user_text)?;
    let position = parse_hash(position_text)?;
    let mut transcript: HashMap<String, Event> =
        serde_json::from_str(include_str!("../src/fixtures/repair_1370_chain.json"))?;
    let mut event = transcript.remove("position_opened").unwrap();
    match &mut event {
        Event::PositionOpened {
            position_id,
            user: address,
            collateral_amount,
            ..
        } => {
            *position_id = position;
            *address = user;
            *collateral_amount = quantity;
        }
        _ => panic!("recording must be a position opening"),
    }
    let indexed = IndexedEvent {
        block_number: block,
        block_timestamp: block * 12,
        tx_hash: position,
        tx_index: 0,
        log_index: 0,
        event,
    };
    let transport = bincode::serialize(&indexed)?;
    let decoded: IndexedEvent = bincode::deserialize(&transport)?;
    db.process_event(&decoded)?;
    Ok(())
}

fn assert_unchanged(db: &PerfDb, history: &[u8], checkpoint: u64, wal_count: u64) {
    assert_eq!(
        bincode::serialize(&db.events.global(None, None)).unwrap(),
        history
    );
    assert_eq!(db.checkpoint.last_block(), checkpoint);
    assert_eq!(
        wal::replay(&db.config().wal.dir, |_| Ok(())).unwrap(),
        wal_count
    );
    assert_eq!(db.positions.all().len(), 1);
}

#[test]
fn numeric_parse_supported_signed_width() {
    for (text, expected) in [
        ("0", 0),
        ("-0", 0),
        ("+0", 0),
        ("000123", 123),
        ("-500", -500),
        ("+1000000000000000000", 1_000_000_000_000_000_000),
        ("170141183460469231731687303715884105727", i128::MAX),
        ("-170141183460469231731687303715884105728", i128::MIN),
    ] {
        assert_eq!(parse_fixed(text), Ok(expected), "{text:?}");
    }
    for text in [
        "170141183460469231731687303715884105728",
        "-170141183460469231731687303715884105729",
        "115792089237316195423570985008687907853269984665640564039457584007913129639935",
        "-57896044618658097711785492504343953926634992332820282019728792003956564819968",
    ] {
        assert_eq!(
            parse_fixed(text),
            Err(ParseFixedError::OutOfRange),
            "{text}"
        );
    }
    for text in [
        "", "+", "-", " 0", "0 ", "1.0", "1e18", "0x10", "1_0", "--1", "１２", "€a", "12\0",
    ] {
        assert_eq!(
            parse_fixed(text),
            Err(ParseFixedError::InvalidDecimal),
            "{text:?}"
        );
    }
}

#[test]
fn numeric_parse_zero_fallback_recorded_ingestion() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    ingest_recorded(&db, "100000000", USER, POSITION, 1).unwrap();
    let id = parse_hash(POSITION).unwrap();
    let history = bincode::serialize(&db.events.global(None, None)).unwrap();
    let wal_count = wal::replay(&cfg.wal.dir, |_| Ok(())).unwrap();
    assert_eq!(wal_count, 1);
    for (text, expected) in [
        ("invalid", ParseFixedError::InvalidDecimal),
        ("", ParseFixedError::InvalidDecimal),
        ("1.5", ParseFixedError::InvalidDecimal),
        (
            "170141183460469231731687303715884105728",
            ParseFixedError::OutOfRange,
        ),
        (
            "-170141183460469231731687303715884105729",
            ParseFixedError::OutOfRange,
        ),
    ] {
        let error = ingest_recorded(&db, text, USER, POSITION, 2).unwrap_err();
        assert_eq!(error.downcast_ref::<ParseFixedError>(), Some(&expected));
        assert_unchanged(&db, &history, 1, wal_count);
        assert_eq!(
            db.positions.get(&id).unwrap().collateral_amount,
            100_000_000
        );
    }
    // Genuine zero is accepted and remains distinguishable from all refusals.
    let second = format!("{}08", &POSITION[..POSITION.len() - 2]);
    ingest_recorded(&db, "0", USER, &second, 2).unwrap();
    assert_eq!(
        db.positions
            .get(&parse_hash(&second).unwrap())
            .unwrap()
            .collateral_amount,
        0
    );
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.events.total_count(), 2);
    assert_eq!(
        db.positions.get(&id).unwrap().collateral_amount,
        100_000_000
    );
    assert_eq!(
        db.positions
            .get(&parse_hash(&second).unwrap())
            .unwrap()
            .collateral_amount,
        0
    );
}

#[test]
fn identifier_parse_exact_ascii_width_without_panics() {
    for width in [20, 32] {
        let parse = |text: &str| -> Result<Vec<u8>, ParseIdentifierError> {
            if width == 20 {
                parse_address(text).map(Vec::from)
            } else {
                parse_hash(text).map(Vec::from)
            }
        };
        for prefix in ["", "0x"] {
            for hex in ["00", "ff", "aB", "Cd"] {
                let text = format!("{prefix}{}", hex.repeat(width));
                let expected = match hex {
                    "00" => 0,
                    "ff" => 255,
                    "aB" => 171,
                    "Cd" => 205,
                    _ => unreachable!(),
                };
                assert_eq!(parse(&text).unwrap(), vec![expected; width]);
            }
        }
        for length in [
            0,
            1,
            width * 2 - 2,
            width * 2 - 1,
            width * 2 + 1,
            width * 2 + 2,
            width * 4,
        ] {
            for prefix in ["", "0x"] {
                let text = format!("{prefix}{}", "a".repeat(length));
                assert_eq!(
                    parse(&text),
                    Err(ParseIdentifierError::InvalidLength {
                        expected_bytes: width,
                        expected_digits: width * 2,
                        actual_digits: length,
                    })
                );
            }
        }
        for text in [
            "€a".into(),
            format!("€{}", "a".repeat(width * 2 - 3)),
            format!("{}é", "a".repeat(width * 2 - 2)),
            format!("g{}", "0".repeat(width * 2 - 1)),
            format!(" {}", "0".repeat(width * 2 - 1)),
            format!("{}\0", "0".repeat(width * 2 - 1)),
            format!("0X{}", "0".repeat(width * 2)),
            format!("0x0x{}", "0".repeat(width * 2)),
        ] {
            // Calling directly also proves malformed UTF-8 boundaries never panic.
            assert!(
                matches!(parse(&text), Err(ParseIdentifierError::InvalidHex { .. })),
                "{text:?}"
            );
        }
    }
    // Every byte value is decoded in its original position, without padding/truncation.
    for start in (0..256).step_by(16) {
        let address = std::array::from_fn::<_, 20, _>(|i| ((start + i) % 256) as u8);
        let hash = std::array::from_fn::<_, 32, _>(|i| ((start + i) % 256) as u8);
        let encode = |bytes: &[u8]| bytes.iter().map(|b| format!("{b:02x}")).collect::<String>();
        assert_eq!(parse_address(&encode(&address)), Ok(address));
        assert_eq!(parse_hash(&encode(&hash)), Ok(hash));
    }
}

#[test]
fn identifier_parse_validation_recorded_ingestion() {
    let dir = tempfile::tempdir().unwrap();
    let cfg = config(dir.path());
    let db = PerfDb::open(cfg.clone()).unwrap();
    ingest_recorded(&db, "100000000", USER, POSITION, 1).unwrap();
    let history = bincode::serialize(&db.events.global(None, None)).unwrap();
    let wal_count = wal::replay(&cfg.wal.dir, |_| Ok(())).unwrap();
    let address = parse_address(USER).unwrap();
    let id = parse_hash(POSITION).unwrap();
    for invalid in [
        "".into(),
        "0x".into(),
        "0xa1".into(),
        format!("{USER}00"),
        "€a".into(),
        format!("0x{}", "g".repeat(40)),
    ] {
        let error = ingest_recorded(&db, "100000000", &invalid, POSITION, 2).unwrap_err();
        assert!(error.downcast_ref::<ParseIdentifierError>().is_some());
        assert_unchanged(&db, &history, 1, wal_count);
    }
    for invalid in [
        "".into(),
        "0x".into(),
        "0x07".into(),
        format!("{POSITION}00"),
        "€a".into(),
        format!("0x{}", "g".repeat(64)),
    ] {
        let error = ingest_recorded(&db, "100000000", USER, &invalid, 2).unwrap_err();
        assert!(error.downcast_ref::<ParseIdentifierError>().is_some());
        assert_unchanged(&db, &history, 1, wal_count);
    }
    assert_eq!(db.positions.get(&id).unwrap().user_address, address);
    assert_eq!(db.positions.get(&id).unwrap().collateral_token, TOKEN);
    assert_eq!(db.positions.by_user(&address).len(), 1);
    assert!(db.positions.get(&[0; 32]).is_none());
    assert!(db.positions.by_user(&[0; 20]).is_empty());
    db.sync_wal().unwrap();
    drop(db);
    let db = PerfDb::open(cfg).unwrap();
    assert_eq!(db.events.total_count(), 1);
    assert_eq!(db.positions.by_user(&address)[0].position_id, id);
    assert_eq!(
        db.positions.get(&id).unwrap().collateral_amount,
        100_000_000
    );
}

#[test]
fn identifier_parse_validation_cli() {
    let dir = tempfile::tempdir().unwrap();
    let db = PerfDb::open(config(dir.path())).unwrap();
    ingest_recorded(&db, "100000000", USER, POSITION, 1).unwrap();
    db.sync_wal().unwrap();
    drop(db);

    // Cargo supplies this crate's real CLI; no toolchain or external service is invoked.
    let run = |args: &[&str]| {
        std::process::Command::new(env!("CARGO_BIN_EXE_perfdb"))
            .arg("--data-dir")
            .arg(dir.path())
            .arg("--json")
            .args(args)
            .output()
            .unwrap()
    };
    for args in [
        vec!["positions", "--id", POSITION],
        vec!["positions", "--user", USER],
    ] {
        let output = run(&args);
        assert!(output.status.success(), "{:?}", output);
        let rows: serde_json::Value = serde_json::from_slice(&output.stdout).unwrap();
        assert_eq!(rows.as_array().unwrap().len(), 1);
        assert_eq!(rows[0]["position_id"], POSITION);
        assert_eq!(rows[0]["user"], USER);
    }

    for invalid in ["", "0x", "0x01", "€a", "0xgg"] {
        for args in [
            vec!["positions", "--id", invalid],
            vec!["orders", "--id", invalid],
            vec!["positions", "--user", invalid],
            vec!["orders", "--user", invalid],
            vec!["balances", invalid],
            vec!["user", invalid],
            vec!["stats", invalid],
            vec!["daily", invalid],
            vec!["events", "--trader", invalid],
        ] {
            let output = run(&args);
            assert_eq!(output.status.code(), Some(1), "{args:?}: {output:?}");
            assert!(output.stdout.is_empty(), "{args:?}: {output:?}");
            let error = String::from_utf8(output.stderr).unwrap();
            let kind = if args.get(1) == Some(&"--id") {
                "invalid hash:"
            } else {
                "invalid address:"
            };
            assert!(
                error.starts_with(&format!("error: {kind}")),
                "{args:?}: {error}"
            );
            assert!(
                error.contains("expected ") || error.contains("non-ASCII-hex"),
                "{args:?}: {error}"
            );
        }
    }
    let db = PerfDb::open(config(dir.path())).unwrap();
    assert_eq!(db.events.total_count(), 1);
    assert_eq!(db.positions.all().len(), 1);
    assert_eq!(db.checkpoint.last_block(), 1);
}
