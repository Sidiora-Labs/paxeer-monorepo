use migration::from_sql;
use sea_orm_migration::{
    SchemaManager,
    sea_orm::{ConnectOptions, ConnectionTrait, Database, DbBackend, Statement},
};
use std::{
    io::{Read, Write},
    net::{TcpListener, TcpStream},
    thread,
    time::Duration,
};

const MULTIBYTE_MIGRATION: &str = "-- Unicode comment é 中 🦀 is filtered by from_sql.
CREATE TEMP TABLE repair_unicode (id integer PRIMARY KEY, value text);
INSERT INTO repair_unicode VALUES (1, 'é');
INSERT INTO repair_unicode VALUES (2, '中');
INSERT INTO repair_unicode VALUES (3, '🦀');
INSERT INTO repair_unicode VALUES (4, 'é café 中文 🦀');
INSERT INTO repair_unicode VALUES (5, 'ASCII tail');
DO $_$
BEGIN
    INSERT INTO repair_unicode VALUES (6, 'body é 中 🦀');
END
$_$;";

#[async_std::test]
async fn migration_unicode_offset_from_sql_preserves_multibyte_statements() {
    let listener = TcpListener::bind(("127.0.0.1", 0)).expect("bind transcript listener");
    let address = listener.local_addr().unwrap();
    let server = thread::spawn(move || replay_postgres_transaction(listener));
    let mut options = ConnectOptions::new(format!(
        "postgres://postgres@{address}/postgres?sslmode=disable"
    ));
    options
        .max_connections(1)
        .min_connections(1)
        .connect_timeout(Duration::from_secs(5))
        .acquire_timeout(Duration::from_secs(5));
    let db = Database::connect(options)
        .await
        .expect("connect real PostgreSQL driver");
    let manager = SchemaManager::new(&db);

    // With character ordinals this first delimiter slices inside the four-byte literal.
    from_sql(&manager, "SELECT '🦀';")
        .await
        .expect("preserve a UTF-8 boundary before the first semicolon");
    from_sql(&manager, MULTIBYTE_MIGRATION)
        .await
        .expect("preserve every statement and commit the Unicode migration");

    let rows = db
        .query_all(Statement::from_string(
            DbBackend::Postgres,
            "SELECT id, value FROM repair_unicode ORDER BY id",
        ))
        .await
        .expect("read the recorded committed PostgreSQL rows");
    let actual: Vec<(i32, String)> = rows
        .iter()
        .map(|row| {
            (
                row.try_get("", "id").unwrap(),
                row.try_get("", "value").unwrap(),
            )
        })
        .collect();
    assert_eq!(
        actual,
        vec![
            (1, "é".to_owned()),
            (2, "中".to_owned()),
            (3, "🦀".to_owned()),
            (4, "é café 中文 🦀".to_owned()),
            (5, "ASCII tail".to_owned()),
            (6, "body é 中 🦀".to_owned()),
        ]
    );
    db.close().await.expect("close the PostgreSQL driver");
    assert_eq!(
        server.join().expect("replay every recorded exchange"),
        TRANSCRIPT.len()
    );
}

fn decode_hex(value: &str) -> Vec<u8> {
    assert_eq!(value.len() % 2, 0);
    value
        .as_bytes()
        .chunks_exact(2)
        .map(|pair| u8::from_str_radix(std::str::from_utf8(pair).unwrap(), 16).unwrap())
        .collect()
}

fn read_batch(stream: &mut TcpStream) -> Vec<u8> {
    let mut batch = Vec::new();
    loop {
        let mut header = [0; 5];
        stream
            .read_exact(&mut header)
            .expect("read PostgreSQL frame header");
        let length = u32::from_be_bytes(header[1..].try_into().unwrap()) as usize;
        assert!(
            (4..=65536).contains(&length),
            "invalid PostgreSQL frame length"
        );
        let mut body = vec![0; length - 4];
        stream
            .read_exact(&mut body)
            .expect("read PostgreSQL frame body");
        batch.extend_from_slice(&header);
        batch.extend_from_slice(&body);
        if matches!(header[0], b'Q' | b'S' | b'X') {
            return batch;
        }
    }
}

fn replay_postgres_transaction(listener: TcpListener) -> usize {
    let (mut stream, _) = listener.accept().expect("accept PostgreSQL client");
    stream
        .set_read_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    stream
        .set_write_timeout(Some(Duration::from_secs(5)))
        .unwrap();
    let mut length = [0; 4];
    stream.read_exact(&mut length).expect("read startup length");
    let length = u32::from_be_bytes(length) as usize;
    assert!((8..=4096).contains(&length));
    let mut startup = vec![0; length - 4];
    stream
        .read_exact(&mut startup)
        .expect("read startup parameters");
    assert_eq!(&startup[..4], &196608_u32.to_be_bytes());
    for parameter in [
        b"user\0postgres\0".as_slice(),
        b"database\0postgres\0",
        b"client_encoding\0UTF8\0",
    ] {
        assert!(
            startup
                .windows(parameter.len())
                .any(|value| value == parameter)
        );
    }
    stream.write_all(&decode_hex(STARTUP_RESPONSE)).unwrap();

    let mut consumed = 0;
    loop {
        let request = read_batch(&mut stream);
        if request == b"S\0\0\0\x04" {
            // SQLx pool health checks outside the transactions use the recorded idle response.
            assert!(consumed == 0 || TRANSCRIPT[consumed - 1].2.ends_with(PING_RESPONSE));
            stream.write_all(&decode_hex(PING_RESPONSE)).unwrap();
        } else if request == b"X\0\0\0\x04" {
            assert_eq!(
                consumed,
                TRANSCRIPT.len(),
                "all statements and commits must execute"
            );
            return consumed;
        } else {
            let (label, expected, response) = TRANSCRIPT
                .get(consumed)
                .expect("client sent an unrecorded exchange");
            assert_eq!(
                request,
                decode_hex(expected),
                "PostgreSQL exchange {consumed}: {label}"
            );
            stream.write_all(&decode_hex(response)).unwrap();
            consumed += 1;
        }
    }
}

// Recorded from PostgreSQL 18.6 using protocol 3.0, UTF8, UTC and binary result formats.
// Each entry contains the exact frontend batch and the backend bytes through ReadyForQuery.
// Only the connection-specific BackendKeyData frame is omitted from startup.
const STARTUP_RESPONSE: &str = "5200000008000000005300000017696e5f686f745f7374616e646279006f6666005300000019696e74656765725f6461746574696d6573006f6e00530000001154696d655a6f6e650055544300530000001b496e74657276616c5374796c6500706f7374677265730053000000207365617263685f7061746800222475736572222c207075626c696300530000001469735f737570657275736572006f6e0053000000166170706c69636174696f6e5f6e616d650000530000002664656661756c745f7472616e73616374696f6e5f726561645f6f6e6c79006f666600530000001a736372616d5f697465726174696f6e730034303936005300000017446174655374796c650049534f2c204d44590053000000237374616e646172645f636f6e666f726d696e675f737472696e6773006f6e00530000002373657373696f6e5f617574686f72697a6174696f6e00706f737467726573005300000019636c69656e745f656e636f64696e6700555446380053000000377365727665725f76657273696f6e0031382e3620285562756e74752031382e362d307562756e7475302e32362e30342e31290053000000197365727665725f656e636f64696e670055544638005a0000000549";
const PING_RESPONSE: &str = "5a0000000549";
const TRANSCRIPT: &[(&str, &str, &str)] = &[
    (
        "BEGIN",
        "510000000a424547494e00",
        "430000000a424547494e005a0000000554",
    ),
    (
        "prepare SELECT '🦀';",
        "500000001e73716c785f735f310053454c4543542027f09fa680273b000000440000000e5373716c785f735f31005300000004",
        "310000000474000000060000540000002100013f636f6c756d6e3f0000000000000000000019ffffffffffff00005a0000000554",
    ),
    (
        "execute SELECT '🦀';",
        "42000000180073716c785f735f31000001000100000001000145000000090000000000430000000650005300000004",
        "3200000004440000000e000100000004f09fa680430000000d53454c45435420310033000000045a0000000554",
    ),
    (
        "COMMIT",
        "510000000b434f4d4d495400",
        "430000000b434f4d4d4954005a0000000549",
    ),
    (
        "BEGIN",
        "510000000a424547494e00",
        "430000000a424547494e005a0000000554",
    ),
    (
        "prepare CREATE TEMP TABLE repair_unicode (id integer PRIMARY KEY, value text);",
        "500000005673716c785f735f32004352454154452054454d50205441424c45207265706169725f756e69636f64652028696420696e7465676572205052494d415259204b45592c2076616c75652074657874293b000000440000000e5373716c785f735f32005300000004",
        "3100000004740000000600006e000000045a0000000554",
    ),
    (
        "execute CREATE TEMP TABLE repair_unicode (id integer PRIMARY KEY, value text);",
        "42000000180073716c785f735f32000001000100000001000145000000090000000000430000000650005300000004",
        "32000000044300000011435245415445205441424c450033000000045a0000000554",
    ),
    (
        "prepare \nINSERT INTO repair_unicode VALUES (1, 'é');",
        "500000003d73716c785f735f33000a494e5345525420494e544f207265706169725f756e69636f64652056414c5545532028312c2027c3a927293b000000440000000e5373716c785f735f33005300000004",
        "3100000004740000000600006e000000045a0000000554",
    ),
    (
        "execute \nINSERT INTO repair_unicode VALUES (1, 'é');",
        "42000000180073716c785f735f33000001000100000001000145000000090000000000430000000650005300000004",
        "3200000004430000000f494e53455254203020310033000000045a0000000554",
    ),
    (
        "prepare \nINSERT INTO repair_unicode VALUES (2, '中');",
        "500000003e73716c785f735f34000a494e5345525420494e544f207265706169725f756e69636f64652056414c5545532028322c2027e4b8ad27293b000000440000000e5373716c785f735f34005300000004",
        "3100000004740000000600006e000000045a0000000554",
    ),
    (
        "execute \nINSERT INTO repair_unicode VALUES (2, '中');",
        "42000000180073716c785f735f34000001000100000001000145000000090000000000430000000650005300000004",
        "3200000004430000000f494e53455254203020310033000000045a0000000554",
    ),
    (
        "prepare \nINSERT INTO repair_unicode VALUES (3, '🦀');",
        "500000003f73716c785f735f35000a494e5345525420494e544f207265706169725f756e69636f64652056414c5545532028332c2027f09fa68027293b000000440000000e5373716c785f735f35005300000004",
        "3100000004740000000600006e000000045a0000000554",
    ),
    (
        "execute \nINSERT INTO repair_unicode VALUES (3, '🦀');",
        "42000000180073716c785f735f35000001000100000001000145000000090000000000430000000650005300000004",
        "3200000004430000000f494e53455254203020310033000000045a0000000554",
    ),
    (
        "prepare \nINSERT INTO repair_unicode VALUES (4, 'é café 中文 🦀');",
        "500000005073716c785f735f36000a494e5345525420494e544f207265706169725f756e69636f64652056414c5545532028342c202765cc8120636166c3a920e4b8ade6968720f09fa68027293b000000440000000e5373716c785f735f36005300000004",
        "3100000004740000000600006e000000045a0000000554",
    ),
    (
        "execute \nINSERT INTO repair_unicode VALUES (4, 'é café 中文 🦀');",
        "42000000180073716c785f735f36000001000100000001000145000000090000000000430000000650005300000004",
        "3200000004430000000f494e53455254203020310033000000045a0000000554",
    ),
    (
        "prepare \nINSERT INTO repair_unicode VALUES (5, 'ASCII tail');",
        "500000004573716c785f735f37000a494e5345525420494e544f207265706169725f756e69636f64652056414c5545532028352c20274153434949207461696c27293b000000440000000e5373716c785f735f37005300000004",
        "3100000004740000000600006e000000045a0000000554",
    ),
    (
        "execute \nINSERT INTO repair_unicode VALUES (5, 'ASCII tail');",
        "42000000180073716c785f735f37000001000100000001000145000000090000000000430000000650005300000004",
        "3200000004430000000f494e53455254203020310033000000045a0000000554",
    ),
    (
        "prepare \nDO $_$\nBEGIN\n    INSERT INTO repair_unicode VALUES (6, 'body é 中 🦀');\nEND\n$_$;",
        "500000006573716c785f735f38000a444f20245f240a424547494e0a20202020494e5345525420494e544f207265706169725f756e69636f64652056414c5545532028362c2027626f647920c3a920e4b8ad20f09fa68027293b0a454e440a245f243b000000440000000e5373716c785f735f38005300000004",
        "3100000004740000000600006e000000045a0000000554",
    ),
    (
        "execute \nDO $_$\nBEGIN\n    INSERT INTO repair_unicode VALUES (6, 'body é 中 🦀');\nEND\n$_$;",
        "42000000180073716c785f735f38000001000100000001000145000000090000000000430000000650005300000004",
        "32000000044300000007444f0033000000045a0000000554",
    ),
    (
        "COMMIT",
        "510000000b434f4d4d495400",
        "430000000b434f4d4d4954005a0000000549",
    ),
    (
        "prepare SELECT id, value FROM repair_unicode ORDER BY id",
        "500000004073716c785f735f390053454c4543542069642c2076616c75652046524f4d207265706169725f756e69636f6465204f52444552204259206964000000440000000e5373716c785f735f39005300000004",
        "31000000047400000006000054000000330002696400000040020001000000170004ffffffff000076616c75650000004002000200000019ffffffffffff00005a0000000549",
    ),
    (
        "execute SELECT id, value FROM repair_unicode ORDER BY id",
        "42000000180073716c785f735f39000001000100000001000145000000090000000000430000000650005300000004",
        "320000000444000000140002000000040000000100000002c3a944000000150002000000040000000200000003e4b8ad44000000160002000000040000000300000004f09fa6804400000027000200000004000000040000001565cc8120636166c3a920e4b8ade6968720f09fa680440000001c000200000004000000050000000a4153434949207461696c44000000220002000000040000000600000010626f647920c3a920e4b8ad20f09fa680430000000d53454c45435420360033000000045a0000000549",
    ),
];
