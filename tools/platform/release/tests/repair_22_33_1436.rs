// The Go module fixture checks publication and registry download behavior.

#[allow(dead_code)]
#[path = "../src/main.rs"]
mod release;

#[allow(dead_code)]
#[path = "support/manifest_fixture.rs"]
mod fixture;

use std::fs;
use std::io::{BufRead, BufReader, Write};
use std::net::TcpListener;
use std::path::Path;
use std::sync::{
    atomic::{AtomicBool, Ordering},
    Arc, Mutex,
};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use fixture::{
    go_fixture::{GoFixture, ARCHIVE},
    Fixture, REVISION, VERSION,
};
use release::{
    parse_artifact_manifest, release_pipeline_verify, render_artifact_manifest, ArtifactSource,
};

struct Registry {
    location: String,
    requests: Arc<Mutex<Vec<String>>>,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}

impl Registry {
    fn start(canonical_location: &str, bytes: Vec<u8>, status: u16) -> Self {
        let path = canonical_location
            .strip_prefix("https://")
            .unwrap_or_else(|| panic!("canonical registry HTTPS URL"))
            .split_once('/')
            .unwrap_or_else(|| panic!("canonical module path"))
            .1;
        let path = format!("/{path}");
        let archive_path = path;
        let index_path = format!("/publications/{}/", GoFixture::load(VERSION).tag);
        let listener = TcpListener::bind("127.0.0.1:0")
            .unwrap_or_else(|error| panic!("loopback registry: {error:?}"));
        let address = listener
            .local_addr()
            .unwrap_or_else(|error| panic!("registry address: {error:?}"));
        listener
            .set_nonblocking(true)
            .unwrap_or_else(|error| panic!("nonblocking listener: {error:?}"));
        let requests = Arc::new(Mutex::new(Vec::new()));
        let stop = Arc::new(AtomicBool::new(false));
        let location = format!("simple+http://{address}{index_path}");
        let recorded = Arc::clone(&requests);
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::SeqCst) {
                let (mut stream, _) = match listener.accept() {
                    Ok(connection) => connection,
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(1));
                        continue;
                    }
                    Err(error) => panic!("accept registry request: {error}"),
                };
                stream
                    .set_read_timeout(Some(Duration::from_secs(5)))
                    .unwrap_or_else(|error| panic!("request deadline: {error:?}"));
                stream
                    .set_write_timeout(Some(Duration::from_secs(5)))
                    .unwrap_or_else(|error| panic!("response deadline: {error:?}"));
                let mut reader = BufReader::new(&mut stream);
                let mut line = String::new();
                reader
                    .read_line(&mut line)
                    .unwrap_or_else(|error| panic!("HTTP request line: {error:?}"));
                let requested = line
                    .split_whitespace()
                    .nth(1)
                    .unwrap_or_else(|| panic!("request target"))
                    .to_owned();
                loop {
                    line.clear();
                    reader
                        .read_line(&mut line)
                        .unwrap_or_else(|error| panic!("HTTP request headers: {error:?}"));
                    if line == "\r\n" || line.is_empty() {
                        break;
                    }
                }
                drop(reader);
                recorded
                    .lock()
                    .unwrap_or_else(|error| panic!("request log: {error:?}"))
                    .push(requested.clone());
                let (code, body) = if requested == index_path {
                    (
                        200,
                        format!("<a href=\"{archive_path}\">v{VERSION}.zip</a>").into_bytes(),
                    )
                } else {
                    assert_eq!(requested, archive_path);
                    (status, bytes.clone())
                };
                write!(
                    stream,
                    "HTTP/1.1 {code} Fixture\r\nContent-Length: {}\r\nConnection: close\r\n\r\n",
                    body.len()
                )
                .unwrap_or_else(|error| panic!("registry response headers: {error:?}"));
                stream
                    .write_all(&body)
                    .unwrap_or_else(|error| panic!("registry response body: {error:?}"));
            }
        });
        Self {
            location,
            requests,
            stop,
            worker: Some(worker),
        }
    }
}

impl Drop for Registry {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::SeqCst);
        self.worker
            .take()
            .unwrap_or_else(|| panic!("registry thread"))
            .join()
            .unwrap_or_else(|error| panic!("registry completed: {error:?}"));
    }
}

#[test]
fn go_fixture_matches_the_canonical_publication() {
    let go = GoFixture::load(VERSION);
    assert_eq!(
        go.module,
        "github.com/Sidiora-Labs/LayerX-Network/sdk/platform/go"
    );
    assert_eq!(go.tag, "sdk/platform/go/v0.1.0");
    assert_eq!(go.location, "https://proxy.golang.org/github.com/!sidiora-!labs/!layer!x-!network/sdk/platform/go/@v/v0.1.0.zip");
    assert_eq!(&ARCHIVE[..4], b"PK\x03\x04");
    let member = format!("{}@v{VERSION}/go.mod", go.module);
    assert!(ARCHIVE
        .windows(member.len())
        .any(|bytes| bytes == member.as_bytes()));
    let pipeline = fixture::committed_pipeline();
    let produced = Fixture::build("repair-1436-canonical", &pipeline);
    let manifest = produced.manifest(&pipeline);
    let entry = manifest
        .artifacts
        .iter()
        .find(|entry| entry.registry == "go-modules")
        .unwrap_or_else(|| panic!("emitted Go artifact"));
    assert_eq!(entry.name, go.module);
    assert_eq!(entry.location, go.location);
    assert_eq!(entry.artifact, format!("v{VERSION}.zip"));
    assert_eq!(entry.digest_of, "registry-bytes");
    assert_eq!(entry.digest, release::sha256_digest(ARCHIVE));
    assert_eq!(
        fs::read(produced.release_dir.join(&entry.retained))
            .unwrap_or_else(|error| panic!("retained Go zip: {error:?}")),
        ARCHIVE
    );
}

#[test]
fn go_archive_fetch_preserves_digest_refusals() {
    let pipeline = fixture::committed_pipeline();
    let produced = Fixture::build("repair-1436-fetch", &pipeline);
    let canonical = produced.manifest(&pipeline);
    let go = GoFixture::load(VERSION);
    let published = canonical
        .artifacts
        .iter()
        .find(|entry| entry.registry == "go-modules")
        .unwrap_or_else(|| panic!("published Go archive"));
    assert_eq!(published.location, go.location);
    let archive = fs::read(produced.release_dir.join(&published.retained))
        .unwrap_or_else(|error| panic!("archive retained by the manifest producer: {error:?}"));
    assert_eq!(archive, ARCHIVE);
    let into = produced.downloads.join("fetched");
    for (case, status) in [("matching", 200), ("tampered", 200), ("missing", 404)] {
        let mut bytes = archive.clone();
        if case == "tampered" {
            *bytes
                .last_mut()
                .unwrap_or_else(|| panic!("nonempty Go archive")) ^= 1;
        }
        let registry = Registry::start(&go.location, bytes, status);
        let mut manifest = canonical.clone();
        manifest
            .artifacts
            .retain(|entry| entry.registry == "go-modules");
        assert_eq!(manifest.artifacts.len(), 1);
        // Redirect the test peer to loopback using a scheme the real fetcher supports.
        // Canonical publication emission is asserted separately above.
        manifest.artifacts[0]
            .location
            .clone_from(&registry.location);
        let wire = render_artifact_manifest(&manifest)
            .unwrap_or_else(|error| panic!("real manifest encoder: {error:?}"));
        let received = parse_artifact_manifest(&wire)
            .unwrap_or_else(|error| panic!("real manifest decoder: {error:?}"));
        let destination = into
            .join("go-modules")
            .join(&received.artifacts[0].artifact);
        if destination.exists() {
            fs::remove_file(&destination).unwrap_or_else(|error| panic!("fresh fetch: {error:?}"));
        }
        let result = release_pipeline_verify(
            &received,
            ArtifactSource::Registries { into: &into },
            Some(REVISION),
        );
        if case == "matching" {
            let verified =
                result.unwrap_or_else(|error| panic!("matching fetched module: {error:?}"));
            assert_eq!(verified.artifacts.len(), 1);
            assert_eq!(verified.artifacts[0].name, go.module);
            assert_eq!(
                verified.artifacts[0].digest,
                release::sha256_digest(ARCHIVE)
            );
            assert_eq!(
                fs::read(&destination)
                    .unwrap_or_else(|error| panic!("downloaded archive: {error:?}")),
                ARCHIVE
            );
        } else {
            let error = result
                .err()
                .unwrap_or_else(|| panic!("registry refusal must halt release"));
            assert!(
                error.contains(&format!("{}@{VERSION} from go-modules", go.module)),
                "{error}"
            );
            if case == "tampered" {
                assert!(
                    error.contains("hash to sha256:")
                        && error.contains("but the manifest lists sha256:"),
                    "{error}"
                );
            } else {
                assert!(error.contains("answered HTTP 404, not 200"), "{error}");
                assert!(
                    !destination.exists(),
                    "failed fetch must not reuse downloaded bytes"
                );
            }
        }
        let archive_path = format!(
            "/{}",
            go.location
                .strip_prefix("https://")
                .unwrap_or_else(|| panic!("published HTTPS locator"))
                .split_once('/')
                .unwrap_or_else(|| panic!("published module path"))
                .1
        );
        assert_eq!(
            *registry
                .requests
                .lock()
                .unwrap_or_else(|error| panic!("request log: {error:?}")),
            [format!("/publications/{}/", go.tag), archive_path]
        );
    }
    assert_go_publication_prerequisites(&canonical, &into);
}

fn assert_go_publication_prerequisites(canonical: &release::ArtifactManifest, into: &Path) {
    for reason in ["unpublished", "unchecked", "source-archive"] {
        let mut manifest = canonical.clone();
        manifest
            .artifacts
            .retain(|entry| entry.registry == "go-modules");
        let entry = &mut manifest.artifacts[0];
        match reason {
            "unpublished" => entry.published = false,
            "unchecked" => entry.install_check = false,
            "source-archive" => "source-archive".clone_into(&mut entry.digest_of),
            _ => unreachable!(),
        }
        let error = release_pipeline_verify(
            &manifest,
            ArtifactSource::Registries { into },
            Some(REVISION),
        )
        .err()
        .unwrap_or_else(|| panic!("publication prerequisites must halt before any fetch"));
        let expected = match reason {
            "unpublished" => "was not published",
            "unchecked" => "install check from the registry did not pass",
            "source-archive" => "not of the bytes the registry serves",
            _ => unreachable!(),
        };
        assert!(error.contains(expected), "{error}");
    }
}
