//! Task 22.33.382: cache-cancellation-poisons-inflight is covered by
//! cancelled_producer_shares_persists_and_expires, cancelled_waiters_leave_worker_owned,
//! cancelled_failure_allows_current_retry, and panicked_worker_releases_ownership.
//! Baseline verification was not run: the ordered gate contract requires complete
//! implementation before qualification and forbids invoking verification by hand.
//! Inherited attempt 8459b545c2ffc8c57148f17ab02bc5f6332300ec failed all four tests:
//! the loopback peer rejected SETEX, emitted by RedisStore::set for expiring values.
//! Wiring request: spec/layerx-merge/spec.kvx (fm-web-explorer-bridge) records this
//! finding-to-test mapping and the runner's final revision/evidence; spec is out of scope.

use recache::{
    handler::{CacheHandler, CacheRequestError},
    stores::{AsyncCacheStore, redis::RedisStore},
};
use std::{
    collections::HashMap,
    future::Future,
    io::ErrorKind,
    sync::{
        Arc,
        atomic::{AtomicUsize, Ordering},
    },
    time::{Duration, Instant},
};
use tokio::{
    net::{TcpListener, TcpStream},
    sync::{Mutex, oneshot},
    task::{JoinHandle, JoinSet},
    time::{sleep, timeout},
};

const TTL: Duration = Duration::from_secs(1);
const TTL_SCRIPT: &str = r#"return {redis.call("GET",KEYS[1]),redis.call("PTTL",KEYS[1])}"#;
type Handler = CacheHandler<RedisStore, String, usize>;
type Entries = HashMap<Vec<u8>, (Vec<u8>, Option<Instant>)>;

// A loopback RESP peer with an in-memory expiring store. Requests, serialization,
// script loading, and response decoding all pass through the production RedisStore.
struct RedisFixture {
    task: JoinHandle<()>,
    store: Arc<RedisStore>,
}

impl RedisFixture {
    async fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let address = listener.local_addr().unwrap();
        let entries = Arc::new(Mutex::new(Entries::new()));
        let task = tokio::spawn(async move {
            let mut connections = JoinSet::new();
            loop {
                tokio::select! {
                    accepted = listener.accept() => {
                        let (stream, _) = accepted.unwrap();
                        connections.spawn(serve_connection(stream, entries.clone()));
                    }
                    result = connections.join_next(), if !connections.is_empty() => {
                        result.unwrap().unwrap();
                    }
                }
            }
        });
        let store = Arc::new(
            RedisStore::builder()
                .connection_string(format!("redis://{address}/"))
                .prefix("repair_22_33_382")
                .build()
                .await
                .unwrap(),
        );
        Self { task, store }
    }

    fn handler(&self) -> Handler {
        CacheHandler::builder(self.store.clone())
            .default_ttl(TTL)
            .build()
    }

    async fn read(&self, key: &str) -> Option<usize> {
        self.store.get(&key.to_owned()).await.unwrap()
    }
}

impl Drop for RedisFixture {
    fn drop(&mut self) {
        self.task.abort();
    }
}

async fn serve_connection(stream: TcpStream, entries: Arc<Mutex<Entries>>) {
    let mut script_loaded = false;
    loop {
        let Some(line) = read_line(&stream).await else {
            return;
        };
        assert!(line.starts_with('*'));
        let count: usize = line[1..].trim().parse().unwrap();
        let mut args = Vec::with_capacity(count);
        for _ in 0..count {
            let line = read_line(&stream).await.unwrap();
            assert!(line.starts_with('$'));
            let length: usize = line[1..].trim().parse().unwrap();
            let mut bytes = vec![0; length + 2];
            assert!(read_exact(&stream, &mut bytes).await);
            assert_eq!(&bytes[length..], b"\r\n");
            bytes.truncate(length);
            args.push(bytes);
        }
        let mut entries = entries.lock().await;
        entries.retain(|_, (_, expiry)| expiry.is_none_or(|time| time > Instant::now()));
        let response = match args[0].as_slice() {
            b"CLIENT" => {
                assert_eq!(args[1], b"SETINFO");
                b"+OK\r\n".to_vec()
            }
            b"SCRIPT" => {
                assert_eq!(args[1], b"LOAD");
                assert_eq!(args[2], TTL_SCRIPT.as_bytes());
                script_loaded = true;
                bulk(redis::Script::new(TTL_SCRIPT).get_hash().as_bytes())
            }
            b"EVALSHA" => {
                assert_eq!(
                    args[1],
                    redis::Script::new(TTL_SCRIPT).get_hash().as_bytes()
                );
                assert_eq!(args[2], b"1");
                if !script_loaded {
                    b"-NOSCRIPT No matching script\r\n".to_vec()
                } else {
                    let (value, ttl) = match entries.get(&args[3]) {
                        Some((value, expiry)) => (
                            bulk(value),
                            expiry.map_or(-1, |time| {
                                time.saturating_duration_since(Instant::now()).as_millis() as i64
                            }),
                        ),
                        None => (b"$-1\r\n".to_vec(), -2),
                    };
                    [
                        b"*2\r\n".as_slice(),
                        &value,
                        format!(":{ttl}\r\n").as_bytes(),
                    ]
                    .concat()
                }
            }
            b"GET" => entries
                .get(&args[1])
                .map_or_else(|| b"$-1\r\n".to_vec(), |(value, _)| bulk(value)),
            b"SETEX" => {
                assert_eq!(args.len(), 4);
                let seconds = std::str::from_utf8(&args[2]).unwrap().parse().unwrap();
                assert!(seconds > 0);
                let expiry = Some(Instant::now() + Duration::from_secs(seconds));
                entries.insert(args[1].clone(), (args[3].clone(), expiry));
                b"+OK\r\n".to_vec()
            }
            b"SET" => {
                let expiry = if args.len() == 5 {
                    assert_eq!(args[3], b"EX");
                    let seconds = std::str::from_utf8(&args[4]).unwrap().parse().unwrap();
                    assert!(seconds > 0);
                    Some(Instant::now() + Duration::from_secs(seconds))
                } else {
                    assert_eq!(args.len(), 3);
                    None
                };
                entries.insert(args[1].clone(), (args[2].clone(), expiry));
                b"+OK\r\n".to_vec()
            }
            command => panic!("unexpected Redis command: {command:?}"),
        };
        drop(entries);
        write_all(&stream, &response).await;
    }
}

async fn read_exact(stream: &TcpStream, mut buffer: &mut [u8]) -> bool {
    let mut received = false;
    while !buffer.is_empty() {
        stream.readable().await.unwrap();
        match stream.try_read(buffer) {
            Ok(0) => {
                assert!(!received, "connection closed inside a request");
                return false;
            }
            Ok(count) => {
                received = true;
                buffer = &mut buffer[count..];
            }
            Err(error) if error.kind() == ErrorKind::WouldBlock => continue,
            Err(error) => panic!("Redis fixture read failed: {error}"),
        }
    }
    true
}

async fn read_line(stream: &TcpStream) -> Option<String> {
    let mut line = Vec::new();
    loop {
        let mut byte = [0];
        if !read_exact(stream, &mut byte).await {
            assert!(line.is_empty(), "connection closed inside a RESP line");
            return None;
        }
        line.push(byte[0]);
        if byte[0] == b'\n' {
            return Some(String::from_utf8(line).unwrap());
        }
    }
}

async fn write_all(stream: &TcpStream, mut buffer: &[u8]) {
    while !buffer.is_empty() {
        stream.writable().await.unwrap();
        match stream.try_write(buffer) {
            Ok(count) => {
                assert!(count > 0, "connection closed inside a response");
                buffer = &buffer[count..];
            }
            Err(error) if error.kind() == ErrorKind::WouldBlock => continue,
            Err(error) => panic!("Redis fixture write failed: {error}"),
        }
    }
}

fn bulk(value: &[u8]) -> Vec<u8> {
    [format!("${}\r\n", value.len()).as_bytes(), value, b"\r\n"].concat()
}

async fn bounded<T>(future: impl Future<Output = T>) -> T {
    timeout(Duration::from_secs(5), future)
        .await
        .expect("cache lifecycle did not finish")
}

async fn current(handler: &Handler, key: &str, value: usize) -> usize {
    bounded(
        handler
            .default_request()
            .key(key)
            .execute(|| async move { Ok::<_, &'static str>(value) }),
    )
    .await
    .unwrap()
}

async fn assert_expiry_and_retry(fixture: &RedisFixture, handler: &Handler, key: &str) {
    let (value, ttl): (usize, _) = fixture
        .store
        .get_with_ttl(&key.to_owned())
        .await
        .unwrap()
        .expect("worker must persist the shared computation");
    assert_eq!(value, 1);
    assert!(ttl.is_some_and(|ttl| ttl <= TTL));
    sleep(TTL + Duration::from_millis(100)).await;
    assert_eq!(fixture.read(key).await, None);
    assert_eq!(current(handler, key, 2).await, 2);
    assert_eq!(fixture.read(key).await, Some(2));
}

#[tokio::test]
async fn cancelled_producer_shares_persists_and_expires() {
    bounded(async {
        let fixture = RedisFixture::start().await;
        let shared = Arc::new(AtomicUsize::new(0));
        let shared_events = shared.clone();
        let handler = CacheHandler::builder(fixture.store.clone())
            .default_ttl(TTL)
            .on_inflight_computed(Arc::new(move |_, _| {
                shared_events.fetch_add(1, Ordering::SeqCst);
            }))
            .build();
        let (started_tx, started_rx) = oneshot::channel();
        let (release_tx, release_rx) = oneshot::channel();
        let first_handler = handler.clone();
        let first = tokio::spawn(async move {
            first_handler
                .default_request()
                .key("shared")
                .execute(|| async move {
                    started_tx.send(()).unwrap();
                    release_rx.await.unwrap();
                    Ok::<_, &'static str>(1)
                })
                .await
        });
        started_rx.await.unwrap();
        first.abort();
        assert!(first.await.unwrap_err().is_cancelled());
        let second_handler = handler.clone();
        let mut second = tokio::spawn(async move {
            second_handler
                .default_request()
                .key("shared")
                .execute(|| async {
                    panic!("second caller must join the active computation");
                    #[allow(unreachable_code)]
                    Ok::<usize, &'static str>(99)
                })
                .await
        });
        assert!(
            timeout(Duration::from_millis(50), &mut second)
                .await
                .is_err()
        );
        release_tx.send(()).unwrap();
        assert_eq!(second.await.unwrap().unwrap(), 1);
        assert_eq!(shared.load(Ordering::SeqCst), 1);
        assert_expiry_and_retry(&fixture, &handler, "shared").await;
    })
    .await;
}

#[tokio::test]
async fn cancelled_waiters_leave_worker_owned() {
    bounded(async {
        let fixture = RedisFixture::start().await;
        let handler = fixture.handler();
        let (started_tx, started_rx) = oneshot::channel();
        let (release_tx, release_rx) = oneshot::channel();
        let first_handler = handler.clone();
        let first = tokio::spawn(async move {
            first_handler
                .default_request()
                .key("unattended")
                .execute(|| async move {
                    started_tx.send(()).unwrap();
                    release_rx.await.unwrap();
                    Ok::<_, &'static str>(1)
                })
                .await
        });
        started_rx.await.unwrap();
        first.abort();
        assert!(first.await.unwrap_err().is_cancelled());
        release_tx.send(()).unwrap();
        while fixture.read("unattended").await != Some(1) {
            sleep(Duration::from_millis(5)).await;
        }
        assert_expiry_and_retry(&fixture, &handler, "unattended").await;
    })
    .await;
}

#[tokio::test]
async fn cancelled_failure_allows_current_retry() {
    bounded(async {
        let fixture = RedisFixture::start().await;
        let handler = fixture.handler();
        let (started_tx, started_rx) = oneshot::channel();
        let (release_tx, release_rx) = oneshot::channel();
        let first_handler = handler.clone();
        let first = tokio::spawn(async move {
            first_handler.default_request().key("failure").execute(|| async move {
                started_tx.send(()).unwrap();
                release_rx.await.unwrap();
                Err::<usize, _>("obsolete computation")
            }).await
        });
        started_rx.await.unwrap();
        first.abort();
        assert!(first.await.unwrap_err().is_cancelled());
        let second_handler = handler.clone();
        let mut second = tokio::spawn(async move {
            second_handler.default_request().key("failure").execute(|| async {
                Ok::<_, &'static str>(99)
            }).await
        });
        assert!(timeout(Duration::from_millis(50), &mut second).await.is_err());
        release_tx.send(()).unwrap();
        assert!(matches!(second.await.unwrap(), Err(CacheRequestError::ComputeError(error)) if error == "obsolete computation"));
        assert_eq!(fixture.read("failure").await, None);
        assert_eq!(current(&handler, "failure", 2).await, 2);
        assert_eq!(fixture.read("failure").await, Some(2));
    }).await;
}

#[tokio::test]
async fn panicked_worker_releases_ownership() {
    bounded(async {
        let fixture = RedisFixture::start().await;
        let handler = fixture.handler();
        let result = handler.default_request().key("panic").execute(|| async {
            panic!("computation panicked");
            #[allow(unreachable_code)]
            Ok::<usize, &'static str>(1)
        }).await;
        assert!(matches!(result, Err(CacheRequestError::ComputeError(error)) if error == "cache computation task cancelled or panicked"));
        assert_eq!(fixture.read("panic").await, None);
        assert_eq!(current(&handler, "panic", 2).await, 2);
        assert_eq!(fixture.read("panic").await, Some(2));
    }).await;
}
