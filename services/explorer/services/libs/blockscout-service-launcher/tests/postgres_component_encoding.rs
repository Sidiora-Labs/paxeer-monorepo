use blockscout_service_launcher::database::{
    connect_postgres, ensure_database_exists_if_configured, ConnectionTrait, DatabaseBackend,
    DatabaseConnectOptionsSettings, DatabaseConnectSettings, DatabaseKvConnection,
    DatabaseSettings,
};
use std::{
    collections::BTreeMap,
    io::{Read, Write},
    net::{Ipv4Addr, TcpListener, TcpStream},
    thread,
    time::{Duration, Instant},
};

struct PostgresPeer {
    port: u16,
    worker: thread::JoinHandle<()>,
}

fn read_body(stream: &mut TcpStream) -> Vec<u8> {
    let mut length = [0; 4];
    stream.read_exact(&mut length).unwrap();
    let length = u32::from_be_bytes(length) as usize;
    assert!((4..=8192).contains(&length), "bounded PostgreSQL frame");
    let mut body = vec![0; length - 4];
    stream.read_exact(&mut body).unwrap();
    body
}

fn send_message(stream: &mut TcpStream, tag: u8, body: &[u8]) {
    stream.write_all(&[tag]).unwrap();
    stream
        .write_all(&((body.len() + 4) as u32).to_be_bytes())
        .unwrap();
    stream.write_all(body).unwrap();
    stream.flush().unwrap();
}

impl PostgresPeer {
    fn start(
        user: &str,
        password: &str,
        database: &str,
        application: &str,
        expected_sql: Option<&str>,
    ) -> Self {
        let listener = TcpListener::bind((Ipv4Addr::LOCALHOST, 0)).unwrap();
        let port = listener.local_addr().unwrap().port();
        listener.set_nonblocking(true).unwrap();
        let user = user.to_owned();
        let password = password.to_owned();
        let database = database.to_owned();
        let application = application.to_owned();
        let expected_sql = expected_sql.map(str::to_owned);
        let worker = thread::spawn(move || {
            let deadline = Instant::now() + Duration::from_secs(5);
            let mut stream = loop {
                match listener.accept() {
                    Ok((stream, _)) => break stream,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        assert!(Instant::now() < deadline, "driver did not connect to peer");
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("accept PostgreSQL connection: {error}"),
                }
            };
            stream
                .set_read_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            stream
                .set_write_timeout(Some(Duration::from_secs(5)))
                .unwrap();
            let startup = read_body(&mut stream);
            assert_eq!(&startup[..4], &196608_u32.to_be_bytes());
            let fields: Vec<_> = startup[4..]
                .split(|byte| *byte == 0)
                .map(|field| std::str::from_utf8(field).unwrap())
                .collect();
            assert_eq!(&fields[fields.len() - 2..], &["", ""]);
            let parameters: BTreeMap<_, _> = fields[..fields.len() - 2]
                .chunks_exact(2)
                .map(|pair| (pair[0], pair[1]))
                .collect();
            assert_eq!(parameters.get("user"), Some(&user.as_str()));
            assert_eq!(parameters.get("database"), Some(&database.as_str()));
            assert_eq!(
                parameters.get("application_name"),
                Some(&application.as_str())
            );

            // Committed PostgreSQL v3 cleartext-authentication transcript, on loopback only.
            send_message(&mut stream, b'R', &3_u32.to_be_bytes());
            let mut tag = [0];
            stream.read_exact(&mut tag).unwrap();
            assert_eq!(tag[0], b'p');
            let supplied_password = read_body(&mut stream);
            assert_eq!(supplied_password, [password.as_bytes(), &[0]].concat());
            send_message(&mut stream, b'R', &0_u32.to_be_bytes());
            send_message(&mut stream, b'S', b"server_version\x0014.0\0");
            send_message(&mut stream, b'S', b"client_encoding\0UTF8\0");
            send_message(&mut stream, b'K', &[0, 0, 0, 1, 0, 0, 0, 2]);
            send_message(&mut stream, b'Z', b"I");

            let mut pings = 0;
            let mut executions = 0;
            loop {
                match stream.read_exact(&mut tag) {
                    Ok(()) => (),
                    Err(error) if error.kind() == std::io::ErrorKind::UnexpectedEof => break,
                    Err(error) => panic!("read PostgreSQL frontend message: {error}"),
                }
                let body = read_body(&mut stream);
                match tag[0] {
                    b'S' => {
                        assert!(body.is_empty());
                        pings += 1;
                        send_message(&mut stream, b'Z', b"I");
                    }
                    b'P' => {
                        let fields: Vec<_> = body.split(|byte| *byte == 0).collect();
                        assert_eq!(
                            std::str::from_utf8(fields[1]).unwrap(),
                            expected_sql
                                .as_deref()
                                .expect("only the configured CREATE may execute")
                        );
                        send_message(&mut stream, b'1', &[]);
                    }
                    b'D' => {
                        assert_eq!(body[0], b'S');
                        send_message(&mut stream, b't', &[0, 0]);
                        send_message(&mut stream, b'n', &[]);
                    }
                    b'B' => send_message(&mut stream, b'2', &[]),
                    b'E' => {
                        assert!(expected_sql.is_some());
                        executions += 1;
                        send_message(&mut stream, b'C', b"CREATE DATABASE\0");
                    }
                    b'C' => send_message(&mut stream, b'3', &[]),
                    b'X' => {
                        assert!(body.is_empty());
                        break;
                    }
                    other => panic!("unexpected PostgreSQL frontend message: {other}"),
                }
            }
            assert!(
                pings > 0,
                "the connected production driver must synchronize with the peer"
            );
            assert_eq!(executions, usize::from(expected_sql.is_some()));
        });
        Self { port, worker }
    }
}

async fn assert_postgres_components(user: &str, password: &str, database: &str) {
    let application = "repair #/?%+&= session";
    let peer = PostgresPeer::start(user, password, database, application, None);
    let settings = DatabaseSettings {
        connect: DatabaseConnectSettings::Kv(DatabaseKvConnection {
            host: Ipv4Addr::LOCALHOST.to_string(),
            port: peer.port,
            user: user.into(),
            password: password.into(),
            dbname: Some(database.into()),
            options: Some(
                "sslmode=disable&application_name=repair%20%23%2F%3F%25%2B%26%3D%20session".into(),
            ),
        }),
        connect_options: DatabaseConnectOptionsSettings {
            max_connections: Some(1),
            min_connections: Some(1),
            connect_timeout: Some(Duration::from_secs(3)),
            acquire_timeout: Some(Duration::from_secs(3)),
            ..Default::default()
        },
        create_database: false,
        run_migrations: false,
    };
    let result = connect_postgres(&settings).await;
    match result {
        Ok(db) => {
            assert_eq!(db.get_database_backend(), DatabaseBackend::Postgres);
            db.ping()
                .await
                .expect("ping through the real SeaORM/SQLx connection");
            db.close().await.unwrap();
            tokio::task::spawn_blocking(move || peer.worker.join())
                .await
                .unwrap()
                .expect("PostgreSQL peer validated startup identity and password");
        }
        Err(error) => {
            let peer_result = tokio::task::spawn_blocking(move || peer.worker.join())
                .await
                .unwrap();
            panic!("production connection failed: {error}; peer: {peer_result:?}");
        }
    }
}

#[actix_web::test]
async fn postgres_component_encoding_reserved_credentials_and_database() {
    assert_postgres_components("user#/?@:[]", "password#/?@:[]", "/database#/?@:[]").await;
}

#[actix_web::test]
async fn postgres_component_encoding_literal_escapes_and_unicode() {
    assert_postgres_components("user%2F+&=é", "password%23+&=雪", "database%3F+&=雪").await;
}

#[actix_web::test]
async fn postgres_component_encoding_plain_settings_preserve_query_options() {
    assert_postgres_components("launcher", "password", "explorer").await;
}

#[test]
fn postgres_component_encoding_url_settings_remain_exact() {
    let configured = "postgresql://user:p%23ss@localhost/explorer?sslmode=require";
    assert_eq!(
        DatabaseConnectSettings::Url(configured.into()).url(),
        configured
    );
}

#[actix_web::test]
async fn postgres_component_encoding_create_database_decodes_name_and_preserves_query() {
    let peer = PostgresPeer::start(
        "user#/?%",
        "password#/?%",
        "postgres",
        "create #/?%+&= session",
        Some("CREATE DATABASE \"/database#/?%2F+&=雪\"\"quoted\"\"\""),
    );
    let settings = DatabaseSettings {
        connect: DatabaseConnectSettings::Kv(DatabaseKvConnection {
            host: Ipv4Addr::LOCALHOST.to_string(),
            port: peer.port,
            user: "user#/?%".into(),
            password: "password#/?%".into(),
            dbname: Some("/database#/?%2F+&=雪\"quoted\"".into()),
            options: Some(
                "sslmode=disable&application_name=create%20%23%2F%3F%25%2B%26%3D%20session".into(),
            ),
        }),
        connect_options: DatabaseConnectOptionsSettings {
            max_connections: Some(1),
            min_connections: Some(1),
            connect_timeout: Some(Duration::from_secs(3)),
            acquire_timeout: Some(Duration::from_secs(3)),
            ..Default::default()
        },
        create_database: true,
        run_migrations: false,
    };
    let result = ensure_database_exists_if_configured(&settings).await;
    let peer_result = tokio::task::spawn_blocking(move || peer.worker.join())
        .await
        .unwrap();
    result.expect("create exact database identity through production SeaORM/SQLx");
    peer_result
        .expect("PostgreSQL peer validated maintenance database, query and CREATE identifier");
}
