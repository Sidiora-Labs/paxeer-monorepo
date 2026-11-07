use sha2::{Digest, Sha256};
use smart_contract_verifier::{
    DetailedVersion, EvmCompilersPool, Fetcher, ListFetcher, SolcCompiler,
};
use std::{
    collections::HashMap,
    fs,
    os::unix::fs::{symlink, MetadataExt, PermissionsExt},
    path::{Path, PathBuf},
    sync::{Arc, Mutex},
    time::{Duration, Instant},
};
use tokio::{
    io::{AsyncReadExt, AsyncWriteExt},
    net::TcpListener,
    sync::Semaphore,
    task::{JoinHandle, JoinSet},
};

// Recorded distribution payloads are opaque bytes. The catalogue binds each version to its
// SHA256; recovery and download never require executing a fixture or an external toolchain.
const VERSION_A: &str = "v0.8.15+commit.e14f2714";
const VERSION_B: &str = "v0.8.16+commit.07a7930e";
const BINARY_A: &[u8] = b"recorded compiler distribution A, version 0.8.15+commit.e14f2714";
const BINARY_B: &[u8] = b"recorded compiler distribution B, version 0.8.16+commit.07a7930e";
const SENTINEL: &[u8] = b"unrelated state must never be truncated, chmodded or replaced";

type Pool = EvmCompilersPool<SolcCompiler>;

#[derive(Clone)]
struct Reply {
    status: u16,
    body: Vec<u8>,
    chunked: bool,
    header_delay: Duration,
    body_delay: Duration,
    chunk_delay: Duration,
}

impl Reply {
    fn bytes(body: impl Into<Vec<u8>>) -> Self {
        Self {
            status: 200,
            body: body.into(),
            chunked: false,
            header_delay: Duration::ZERO,
            body_delay: Duration::ZERO,
            chunk_delay: Duration::ZERO,
        }
    }
}

struct Distribution {
    url: String,
    replies: Arc<Mutex<HashMap<String, Reply>>>,
    requests: Arc<Mutex<HashMap<String, usize>>>,
    server: JoinHandle<()>,
}

impl Drop for Distribution {
    fn drop(&mut self) {
        // Dropping the listener's JoinSet also stops every stalled response task.
        self.server.abort();
    }
}

async fn respond(
    mut stream: tokio::net::TcpStream,
    replies: Arc<Mutex<HashMap<String, Reply>>>,
    requests: Arc<Mutex<HashMap<String, usize>>>,
) -> std::io::Result<()> {
    let mut request = Vec::new();
    let mut buffer = [0; 1024];
    while !request.windows(4).any(|bytes| bytes == b"\r\n\r\n") {
        let count = stream.read(&mut buffer).await?;
        if count == 0 || request.len() + count > 8192 {
            return Err(std::io::Error::new(
                std::io::ErrorKind::InvalidData,
                "invalid fixture request",
            ));
        }
        request.extend_from_slice(&buffer[..count]);
    }
    let path = std::str::from_utf8(&request)
        .unwrap()
        .split_whitespace()
        .nth(1)
        .unwrap();
    *requests.lock().unwrap().entry(path.to_owned()).or_default() += 1;
    let reply = replies
        .lock()
        .unwrap()
        .get(path)
        .cloned()
        .unwrap_or_else(|| {
            let mut reply = Reply::bytes(b"missing fixture".as_slice());
            reply.status = 404;
            reply
        });
    tokio::time::sleep(reply.header_delay).await;
    let framing = if reply.chunked {
        "Transfer-Encoding: chunked\r\n".to_owned()
    } else {
        format!("Content-Length: {}\r\n", reply.body.len())
    };
    let headers = format!(
        "HTTP/1.1 {} Fixture\r\n{}Content-Type: application/json\r\nConnection: close\r\n\r\n",
        reply.status, framing
    );
    stream.write_all(headers.as_bytes()).await?;
    if reply.chunked {
        for (index, chunk) in reply.body.chunks(16).enumerate() {
            if index == 1 {
                tokio::time::sleep(reply.body_delay).await;
            }
            if index > 0 {
                tokio::time::sleep(reply.chunk_delay).await;
            }
            stream
                .write_all(format!("{:x}\r\n", chunk.len()).as_bytes())
                .await?;
            stream.write_all(chunk).await?;
            stream.write_all(b"\r\n").await?;
        }
        stream.write_all(b"0\r\n\r\n").await?;
    } else {
        let split = reply.body.len() / 2;
        stream.write_all(&reply.body[..split]).await?;
        tokio::time::sleep(reply.body_delay).await;
        stream.write_all(&reply.body[split..]).await?;
    }
    stream.shutdown().await
}

fn catalogue(a: &[u8], b: &[u8]) -> Vec<u8> {
    serde_json::to_vec(&serde_json::json!({"builds": [
        {"path": "a", "longVersion": VERSION_A, "sha256": format!("0x{}", hex::encode(Sha256::digest(a)))},
        {"path": "b", "longVersion": VERSION_B, "sha256": format!("0x{}", hex::encode(Sha256::digest(b)))}
    ]})).unwrap()
}

impl Distribution {
    async fn start() -> Self {
        let listener = TcpListener::bind("127.0.0.1:0").await.unwrap();
        let url = format!("http://{}/list.json", listener.local_addr().unwrap());
        let replies = Arc::new(Mutex::new(HashMap::from([
            (
                "/list.json".to_owned(),
                Reply::bytes(catalogue(BINARY_A, BINARY_B)),
            ),
            ("/a".to_owned(), Reply::bytes(BINARY_A)),
            ("/b".to_owned(), Reply::bytes(BINARY_B)),
        ])));
        let requests = Arc::new(Mutex::new(HashMap::new()));
        let server = {
            let replies = replies.clone();
            let requests = requests.clone();
            tokio::spawn(async move {
                let mut connections = JoinSet::new();
                loop {
                    tokio::select! {
                        accepted = listener.accept() => {
                            let (stream, _) = accepted.unwrap();
                            connections.spawn(respond(stream, replies.clone(), requests.clone()));
                        }
                        Some(result) = connections.join_next(), if !connections.is_empty() => {
                            // A rejected response may encounter a closed socket. Panics still fail.
                            result.unwrap().ok();
                        }
                    }
                }
            })
        };
        Self {
            url,
            replies,
            requests,
            server,
        }
    }

    fn set(&self, path: &str, reply: Reply) {
        self.replies.lock().unwrap().insert(path.to_owned(), reply);
    }

    fn count(&self, path: &str) -> usize {
        *self.requests.lock().unwrap().get(path).unwrap_or(&0)
    }

    async fn fetcher(&self, root: &Path) -> ListFetcher<DetailedVersion> {
        ListFetcher::new(self.url.parse().unwrap(), root.to_path_buf(), None, None)
            .await
            .unwrap()
    }

    async fn pool(&self, root: &Path) -> Pool {
        Pool::new(
            Arc::new(self.fetcher(root).await),
            Arc::new(Semaphore::new(1)),
        )
    }
}

fn version_a() -> DetailedVersion {
    VERSION_A.parse().unwrap()
}

fn installed(root: &Path) -> PathBuf {
    root.join(version_a().to_string()).join("solc")
}

async fn install_then_stop(distribution: &Distribution, root: &Path) -> PathBuf {
    let pool = distribution.pool(root).await;
    let path = pool.fetch_compiler(&version_a()).await.unwrap();
    assert_eq!(fs::read(&path).unwrap(), BINARY_A);
    drop(pool);
    path
}

async fn reopen(distribution: &Distribution, root: &Path) -> Pool {
    let pool = distribution.pool(root).await;
    pool.load_from_dir(&root.to_path_buf()).await;
    pool
}

#[tokio::test]
async fn restart_valid_cache_is_authenticated_without_redownload() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let path = install_then_stop(&distribution, root.path()).await;
    let inode = fs::metadata(&path).unwrap().ino();
    let mut unavailable = Reply::bytes(b"distribution unavailable".as_slice());
    unavailable.status = 503;
    distribution.set("/a", unavailable);
    let pool = reopen(&distribution, root.path()).await;
    let recovered = pool.fetch_compiler(&version_a()).await.unwrap();
    assert_eq!(recovered, path);
    assert_eq!(fs::metadata(&recovered).unwrap().ino(), inode);
    assert_eq!(fs::read(recovered).unwrap(), BINARY_A);
    assert_eq!(distribution.count("/a"), 1);
    assert_eq!(distribution.count("/list.json"), 2);
}

#[tokio::test]
async fn restart_truncated_executable_is_redownloaded() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let path = install_then_stop(&distribution, root.path()).await;
    fs::write(&path, &BINARY_A[..8]).unwrap();
    let pool = reopen(&distribution, root.path()).await;
    let recovered = pool.fetch_compiler(&version_a()).await.unwrap();
    assert_eq!(recovered, path);
    assert_eq!(fs::read(recovered).unwrap(), BINARY_A);
    assert_eq!(distribution.count("/a"), 2);
}

#[tokio::test]
async fn restart_wrong_version_and_local_checksum_are_not_trusted() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let pool = distribution.pool(root.path()).await;
    let path = pool.fetch_compiler(&version_a()).await.unwrap();
    let version_b = VERSION_B.parse().unwrap();
    let other = pool.fetch_compiler(&version_b).await.unwrap();
    drop(pool);
    fs::copy(&other, &path).unwrap();
    fs::write(
        path.with_extension("sha256"),
        hex::encode(Sha256::digest(BINARY_B)),
    )
    .unwrap();
    let pool = reopen(&distribution, root.path()).await;
    assert_eq!(
        fs::read(pool.fetch_compiler(&version_a()).await.unwrap()).unwrap(),
        BINARY_A
    );
    assert_eq!(
        fs::read(pool.fetch_compiler(&version_b).await.unwrap()).unwrap(),
        BINARY_B
    );
    assert_eq!(distribution.count("/a"), 2);
    assert_eq!(distribution.count("/b"), 1);
}

#[tokio::test]
async fn restart_symlink_even_with_matching_bytes_is_replaced_without_touching_target() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let outside = tempfile::tempdir().unwrap();
    let sentinel = outside.path().join("compiler");
    fs::write(&sentinel, BINARY_A).unwrap();
    fs::set_permissions(&sentinel, fs::Permissions::from_mode(0o700)).unwrap();
    let path = install_then_stop(&distribution, root.path()).await;
    fs::remove_file(&path).unwrap();
    symlink(&sentinel, &path).unwrap();
    let pool = reopen(&distribution, root.path()).await;
    let recovered = pool.fetch_compiler(&version_a()).await.unwrap();
    assert!(fs::symlink_metadata(&recovered).unwrap().is_file());
    assert_eq!(fs::read(&recovered).unwrap(), BINARY_A);
    assert_eq!(fs::read(&sentinel).unwrap(), BINARY_A);
    assert_eq!(distribution.count("/a"), 2);
}

#[tokio::test]
async fn restart_peer_writable_executable_is_replaced_with_private_bytes() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let path = install_then_stop(&distribution, root.path()).await;
    fs::set_permissions(&path, fs::Permissions::from_mode(0o777)).unwrap();
    let pool = reopen(&distribution, root.path()).await;
    let recovered = pool.fetch_compiler(&version_a()).await.unwrap();
    assert_eq!(fs::read(&recovered).unwrap(), BINARY_A);
    assert_eq!(
        fs::metadata(&recovered).unwrap().permissions().mode() & 0o777,
        0o700
    );
    assert_eq!(distribution.count("/a"), 2);
}

#[tokio::test]
async fn restart_socket_entry_is_never_opened_as_a_compiler() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let path = install_then_stop(&distribution, root.path()).await;
    fs::remove_file(&path).unwrap();
    let _socket = std::os::unix::net::UnixListener::bind(&path).unwrap();
    let pool = reopen(&distribution, root.path()).await;
    let recovered = tokio::time::timeout(Duration::from_secs(3), pool.fetch_compiler(&version_a()))
        .await
        .unwrap()
        .unwrap();
    assert!(fs::symlink_metadata(&recovered).unwrap().is_file());
    assert_eq!(fs::read(recovered).unwrap(), BINARY_A);
    assert_eq!(distribution.count("/a"), 2);
}

#[tokio::test]
async fn install_uses_service_owned_private_directories_and_executable() {
    let distribution = Distribution::start().await;
    let parent = tempfile::tempdir().unwrap();
    let root = parent.path().join("new-cache");
    let fetcher = distribution.fetcher(&root).await;
    let file = fetcher.fetch(&version_a()).await.unwrap();
    for path in [root.as_path(), file.parent().unwrap(), file.as_path()] {
        let metadata = fs::symlink_metadata(path).unwrap();
        assert_eq!(metadata.permissions().mode() & 0o777, 0o700);
        assert_eq!(metadata.uid(), fs::metadata(parent.path()).unwrap().uid());
    }
    assert_eq!(fs::metadata(&file).unwrap().nlink(), 1);
    assert_eq!(fs::read(&file).unwrap(), BINARY_A);
    assert_eq!(fs::read_dir(file.parent().unwrap()).unwrap().count(), 1);
}

#[tokio::test]
async fn install_rejects_peer_writable_and_symlink_directories() {
    let distribution = Distribution::start().await;
    let parent = tempfile::tempdir().unwrap();
    let outside = tempfile::tempdir().unwrap();
    let sentinel = outside.path().join("solc");
    fs::write(&sentinel, SENTINEL).unwrap();
    let sentinel_mode = fs::metadata(&sentinel).unwrap().permissions().mode();
    for version_directory in [false, true] {
        for symbolic in [false, true] {
            let case = tempfile::tempdir_in(parent.path()).unwrap();
            let root = case.path().join("cache");
            let unsafe_dir = if version_directory {
                fs::create_dir(&root).unwrap();
                fs::set_permissions(&root, fs::Permissions::from_mode(0o700)).unwrap();
                root.join(version_a().to_string())
            } else {
                root.clone()
            };
            if symbolic {
                symlink(outside.path(), &unsafe_dir).unwrap();
            } else {
                fs::create_dir(&unsafe_dir).unwrap();
                fs::set_permissions(&unsafe_dir, fs::Permissions::from_mode(0o777)).unwrap();
            }
            let fetcher = distribution.fetcher(&root).await;
            assert!(fetcher.fetch(&version_a()).await.is_err());
            assert_eq!(fs::read(&sentinel).unwrap(), SENTINEL);
            assert_eq!(
                fs::metadata(&sentinel).unwrap().permissions().mode(),
                sentinel_mode
            );
            if symbolic {
                assert!(fs::symlink_metadata(&unsafe_dir)
                    .unwrap()
                    .file_type()
                    .is_symlink());
            } else {
                assert_eq!(
                    fs::metadata(&unsafe_dir).unwrap().permissions().mode() & 0o777,
                    0o777
                );
                assert_eq!(fs::read_dir(&unsafe_dir).unwrap().count(), 0);
            }
        }
    }
}

#[tokio::test]
async fn install_ignores_predictable_temporary_symlink_during_download() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let outside = tempfile::tempdir().unwrap();
    let sentinel = outside.path().join("state");
    fs::write(&sentinel, SENTINEL).unwrap();
    let original_mode = fs::metadata(&sentinel).unwrap().permissions().mode();
    let file = installed(root.path());
    fs::create_dir(file.parent().unwrap()).unwrap();
    fs::set_permissions(file.parent().unwrap(), fs::Permissions::from_mode(0o700)).unwrap();
    let mut delayed = Reply::bytes(BINARY_A);
    delayed.body_delay = Duration::from_millis(150);
    distribution.set("/a", delayed);
    let fetcher = Arc::new(distribution.fetcher(root.path()).await);
    let download = {
        let fetcher = fetcher.clone();
        tokio::spawn(async move { fetcher.fetch(&version_a()).await })
    };
    tokio::time::timeout(Duration::from_secs(3), async {
        while distribution.count("/a") == 0 {
            tokio::task::yield_now().await;
        }
    })
    .await
    .unwrap();
    let predictable = file.with_extension("tmp");
    symlink(&sentinel, &predictable).unwrap();
    symlink(&sentinel, &file).unwrap();
    assert_eq!(download.await.unwrap().unwrap(), file);
    assert_eq!(fs::read(&file).unwrap(), BINARY_A);
    assert!(fs::symlink_metadata(&file).unwrap().is_file());
    assert!(fs::symlink_metadata(&predictable)
        .unwrap()
        .file_type()
        .is_symlink());
    assert_eq!(fs::read(&sentinel).unwrap(), SENTINEL);
    assert_eq!(
        fs::metadata(&sentinel).unwrap().permissions().mode(),
        original_mode
    );
    assert_eq!(fs::read_dir(file.parent().unwrap()).unwrap().count(), 2);
}

#[tokio::test]
async fn concurrent_installs_do_not_share_temporary_files() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let fetcher = distribution.fetcher(root.path()).await;
    let version = version_a();
    let installs = (0..8).map(|_| fetcher.fetch(&version));
    for result in futures::future::join_all(installs).await {
        assert_eq!(result.unwrap(), installed(root.path()));
    }
    assert_eq!(fs::read(installed(root.path())).unwrap(), BINARY_A);
    assert_eq!(
        fs::read_dir(installed(root.path()).parent().unwrap())
            .unwrap()
            .count(),
        1
    );
}

#[tokio::test]
async fn catalogue_rejects_http_errors_including_redirect_without_location() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    for status in [302, 404, 503] {
        let mut reply = Reply::bytes(catalogue(BINARY_A, BINARY_B));
        reply.status = status;
        distribution.set("/list.json", reply);
        let error = ListFetcher::<DetailedVersion>::new(
            distribution.url.parse().unwrap(),
            root.path().to_path_buf(),
            None,
            None,
        )
        .await
        .err()
        .expect("HTTP errors must fail discovery");
        assert!(
            format!("{error:#}").contains(&format!("status {status}")),
            "{error:#}"
        );
    }
    assert_eq!(distribution.count("/list.json"), 3);
}

#[tokio::test]
async fn binary_rejects_typed_http_errors_before_installation() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let fetcher = distribution.fetcher(root.path()).await;
    for status in [302, 404, 503] {
        let mut reply = Reply::bytes(BINARY_A);
        reply.status = status;
        distribution.set("/a", reply);
        let error = fetcher.fetch(&version_a()).await.unwrap_err();
        assert_eq!(error.http_status(), Some(status));
        assert!(!installed(root.path()).exists());
        assert_eq!(fs::read_dir(root.path()).unwrap().count(), 0);
    }
}

#[tokio::test]
async fn catalogue_bounds_content_length_and_chunked_bodies() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    for chunked in [false, true] {
        let mut reply = Reply::bytes(catalogue(BINARY_A, BINARY_B));
        reply.chunked = chunked;
        distribution.set("/list.json", reply);
        let error = ListFetcher::<DetailedVersion>::builder(
            distribution.url.parse().unwrap(),
            root.path().to_path_buf(),
        )
        .max_list_bytes(64)
        .build()
        .await
        .err()
        .expect("oversized catalogue must fail");
        assert!(
            format!("{error:#}").contains("exceeds 64 bytes"),
            "{error:#}"
        );
        assert_eq!(distribution.count("/a"), 0);
    }
}

#[tokio::test]
async fn binary_bounds_content_length_and_chunked_bodies_before_installation() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    for chunked in [false, true] {
        let mut reply = Reply::bytes(BINARY_A);
        reply.chunked = chunked;
        distribution.set("/a", reply);
        let fetcher = ListFetcher::<DetailedVersion>::builder(
            distribution.url.parse().unwrap(),
            root.path().to_path_buf(),
        )
        .max_binary_bytes(32)
        .build()
        .await
        .unwrap();
        let error = fetcher.fetch(&version_a()).await.unwrap_err();
        assert!(error.is_body_too_large(), "{error}");
        assert!(!installed(root.path()).exists());
        assert_eq!(fs::read_dir(root.path()).unwrap().count(), 0);
    }
}

#[tokio::test]
async fn exact_body_limits_allow_valid_chunked_catalogue_and_binary() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let list = catalogue(BINARY_A, BINARY_B);
    let mut reply = Reply::bytes(list.clone());
    reply.chunked = true;
    distribution.set("/list.json", reply);
    let mut reply = Reply::bytes(BINARY_A);
    reply.chunked = true;
    distribution.set("/a", reply);
    let fetcher = ListFetcher::<DetailedVersion>::builder(
        distribution.url.parse().unwrap(),
        root.path().to_path_buf(),
    )
    .max_list_bytes(list.len())
    .max_binary_bytes(BINARY_A.len())
    .build()
    .await
    .unwrap();
    assert_eq!(
        fs::read(fetcher.fetch(&version_a()).await.unwrap()).unwrap(),
        BINARY_A
    );
}

#[tokio::test]
async fn bounded_binary_stream_keeps_checksum_validation_and_leaves_no_residue() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let fetcher = distribution.fetcher(root.path()).await;
    let mut reply = Reply::bytes(BINARY_B);
    reply.chunked = true;
    distribution.set("/a", reply);
    let error = fetcher.fetch(&version_a()).await.unwrap_err();
    assert!(
        error
            .to_string()
            .contains("hashsum of fetched file mismatch"),
        "{error}"
    );
    assert_eq!(fs::read_dir(root.path()).unwrap().count(), 0);
}

fn stalled_reply(headers: bool) -> Reply {
    let mut reply = Reply::bytes(catalogue(BINARY_A, BINARY_B));
    if headers {
        reply.header_delay = Duration::from_secs(2);
    } else {
        reply.chunked = true;
        reply.body_delay = Duration::from_secs(2);
    }
    reply
}

#[tokio::test]
async fn catalogue_deadline_covers_headers_and_streamed_body() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    for headers in [true, false] {
        distribution.set("/list.json", stalled_reply(headers));
        let start = Instant::now();
        let error = ListFetcher::<DetailedVersion>::builder(
            distribution.url.parse().unwrap(),
            root.path().to_path_buf(),
        )
        .connect_timeout(Duration::from_millis(100))
        .request_timeout(Duration::from_millis(150))
        .build()
        .await
        .err()
        .expect("stalled discovery must time out");
        assert!(format!("{error:#}").contains("timed out"), "{error:#}");
        assert!(start.elapsed() < Duration::from_secs(2));
    }
}

#[tokio::test]
async fn binary_typed_deadline_covers_headers_and_streamed_body() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let fetcher = ListFetcher::<DetailedVersion>::builder(
        distribution.url.parse().unwrap(),
        root.path().to_path_buf(),
    )
    .connect_timeout(Duration::from_millis(100))
    .request_timeout(Duration::from_millis(150))
    .build()
    .await
    .unwrap();
    for headers in [true, false] {
        let mut reply = stalled_reply(headers);
        reply.body = BINARY_A.to_vec();
        distribution.set("/a", reply);
        let start = Instant::now();
        let error = fetcher.fetch(&version_a()).await.unwrap_err();
        assert!(error.is_timeout(), "{error}");
        assert!(start.elapsed() < Duration::from_secs(2));
        assert!(!installed(root.path()).exists());
        assert_eq!(fs::read_dir(root.path()).unwrap().count(), 0);
    }
}

#[tokio::test]
async fn total_deadline_is_not_reset_by_progress_on_chunked_responses() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let fetcher = ListFetcher::<DetailedVersion>::builder(
        distribution.url.parse().unwrap(),
        root.path().to_path_buf(),
    )
    .request_timeout(Duration::from_millis(150))
    .build()
    .await
    .unwrap();
    let mut reply = Reply::bytes(BINARY_A);
    reply.chunked = true;
    reply.chunk_delay = Duration::from_millis(80);
    distribution.set("/a", reply.clone());
    assert!(fetcher.fetch(&version_a()).await.unwrap_err().is_timeout());
    reply.body = catalogue(BINARY_A, BINARY_B);
    distribution.set("/list.json", reply);
    let error = ListFetcher::<DetailedVersion>::builder(
        distribution.url.parse().unwrap(),
        root.path().to_path_buf(),
    )
    .request_timeout(Duration::from_millis(150))
    .build()
    .await
    .err()
    .expect("progress must not reset the discovery deadline");
    assert!(format!("{error:#}").contains("timed out"), "{error:#}");
    assert_eq!(fs::read_dir(root.path()).unwrap().count(), 0);
}

#[tokio::test]
async fn rejected_recovery_never_becomes_a_hit_when_replacement_fails() {
    let distribution = Distribution::start().await;
    let root = tempfile::tempdir().unwrap();
    let file = install_then_stop(&distribution, root.path()).await;
    fs::write(&file, b"truncated").unwrap();
    let mut unavailable = Reply::bytes(BINARY_A);
    unavailable.status = 503;
    distribution.set("/a", unavailable);
    let pool = reopen(&distribution, root.path()).await;
    assert!(pool.fetch_compiler(&version_a()).await.is_err());
    assert!(pool.fetch_compiler(&version_a()).await.is_err());
    assert_eq!(fs::read(&file).unwrap(), b"truncated");
    distribution.set("/a", Reply::bytes(BINARY_A));
    assert_eq!(
        fs::read(pool.fetch_compiler(&version_a()).await.unwrap()).unwrap(),
        BINARY_A
    );
    assert_eq!(distribution.count("/a"), 4);
}
