// Repair record 22.33.1437: release-git-fetch-timeout is proven by the tests below.
// Baseline: not run, because the explicit bounded gate order forbids pre-change verification.
// Wiring: none. The real Git subprocess requirement takes precedence over the generic
// no-external-binary clause; the only tools used are the production Git/POSIX shell.
// The runner's gate evidence binds these tests to the verified candidate revision.

use std::fs;
use std::io::Read as _;
use std::net::TcpListener;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::{Duration, Instant};

use layerx_platform_release::fetch::{fetch, fetch_with_git_limits, GitFetchLimits};
use layerx_platform_release::ArtifactManifestEntry;
use sha2::{Digest, Sha256};

static SEQUENCE: AtomicU64 = AtomicU64::new(0);

struct Fixture(PathBuf);

impl Fixture {
    fn new() -> Self {
        let root = PathBuf::from(env!("CARGO_TARGET_TMPDIR")).join(format!(
            "repair-22-33-1437-{}-{}",
            std::process::id(),
            SEQUENCE.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap();
        Self(root)
    }

    fn producer(&self, bytes: &[u8]) -> PathBuf {
        let repository = self.0.join("producer");
        git(Command::new("git")
            .args(["init", "--quiet"])
            .arg(&repository));
        fs::write(repository.join("Package.swift"), bytes).unwrap();
        git(Command::new("git")
            .arg("-C")
            .arg(&repository)
            .args(["add", "Package.swift"]));
        git(Command::new("git").arg("-C").arg(&repository).args([
            "-c",
            "user.name=Release fixture",
            "-c",
            "user.email=release@example.invalid",
            "-c",
            "commit.gpgsign=false",
            "-c",
            "core.hooksPath=/dev/null",
            "commit",
            "--quiet",
            "-m",
            "Publish package",
        ]));
        git(Command::new("git")
            .arg("-C")
            .arg(&repository)
            .args(["tag", "--no-sign", "v1"]));
        repository
    }

    fn destination(&self) -> PathBuf {
        self.0.join("consumer/package.tar")
    }

    fn clean(&self) {
        let consumer = self.0.join("consumer");
        for entry in fs::read_dir(&consumer).unwrap() {
            let name = entry.unwrap().file_name();
            assert_eq!(
                name, "package.tar",
                "incomplete Git checkout or archive survived"
            );
        }
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

fn git(command: &mut Command) -> Output {
    let output = command.output().unwrap();
    assert!(
        output.status.success(),
        "{command:?}: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    output
}

fn entry(location: String) -> ArtifactManifestEntry {
    ArtifactManifestEntry {
        name: "LayerXSDK".into(),
        version: "1".into(),
        registry: "swiftpm".into(),
        distribution: "git".into(),
        target: "git".into(),
        artifact: "package.tar".into(),
        retained: "package.tar".into(),
        digest: String::new(),
        digest_of: "built-bytes".into(),
        signature: "tag-signature".into(),
        sbom: "sbom".into(),
        attestation: "provenance".into(),
        location,
        source_revision: String::new(),
        rollback_version: None,
        published: true,
        install_check: true,
    }
}

fn local(repository: &Path, tag: &str) -> ArtifactManifestEntry {
    entry(format!("git+file://{}#{tag}", repository.display()))
}

fn limits() -> GitFetchLimits {
    GitFetchLimits {
        timeout: Duration::from_secs(10),
        ..GitFetchLimits::default()
    }
}

#[test]
fn release_git_fetch_preserves_the_real_tagged_archive_and_cleans_checkout() {
    let fixture = Fixture::new();
    let repository = fixture.producer(b"// published Swift package\n");
    let expected =
        git(Command::new("git")
            .arg("-C")
            .arg(&repository)
            .args(["archive", "--format=tar", "v1"]))
        .stdout;
    // HEAD differs from the published tag, so fetching HEAD cannot satisfy this proof.
    fs::write(repository.join("Package.swift"), b"unreleased\n").unwrap();
    git(Command::new("git")
        .arg("-C")
        .arg(&repository)
        .args(["add", "Package.swift"]));
    git(Command::new("git").arg("-C").arg(&repository).args([
        "-c",
        "user.name=Release fixture",
        "-c",
        "user.email=release@example.invalid",
        "-c",
        "commit.gpgsign=false",
        "-c",
        "core.hooksPath=/dev/null",
        "commit",
        "--quiet",
        "-m",
        "Next release",
    ]));
    let destination = fixture.destination();
    fetch(&local(&repository, "v1"), &destination).unwrap();
    let actual = fs::read(&destination).unwrap();
    assert_eq!(actual, expected);
    assert_eq!(Sha256::digest(&actual), Sha256::digest(&expected));
    fixture.clean();
    fs::remove_file(destination).unwrap();
    // The exact archive boundary is accepted, not truncated or rejected.
    fetch_with_git_limits(
        &local(&repository, "v1"),
        &fixture.destination(),
        GitFetchLimits {
            archive_bytes: expected.len() as u64,
            ..limits()
        },
    )
    .unwrap();
    assert_eq!(fs::read(fixture.destination()).unwrap(), expected);
    fixture.clean();
}

#[test]
fn release_git_fetch_deadline_kills_transport_reaps_clone_and_cleans_checkout() {
    let fixture = Fixture::new();
    let listener = TcpListener::bind("127.0.0.1:0").unwrap();
    let address = listener.local_addr().unwrap();
    listener.set_nonblocking(true).unwrap();
    let server = std::thread::spawn(move || {
        let deadline = Instant::now() + Duration::from_secs(5);
        let mut socket = loop {
            match listener.accept() {
                Ok((socket, _)) => break socket,
                Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                    assert!(
                        Instant::now() < deadline,
                        "Git never connected to its real HTTP peer"
                    );
                    std::thread::sleep(Duration::from_millis(2));
                }
                Err(error) => panic!("accept Git transport: {error}"),
            }
        };
        socket
            .set_read_timeout(Some(Duration::from_secs(3)))
            .unwrap();
        let mut request = Vec::new();
        let mut byte = [0];
        while !request.ends_with(b"\r\n\r\n") {
            assert_eq!(socket.read(&mut byte).unwrap(), 1);
            request.push(byte[0]);
        }
        assert!(String::from_utf8_lossy(&request).contains("info/refs?service=git-upload-pack"));
        // A real server withholding its response must lose the helper's connection on timeout.
        assert_eq!(
            socket.read(&mut byte).unwrap(),
            0,
            "Git transport survived cancellation"
        );
    });
    let started = Instant::now();
    let error = fetch_with_git_limits(
        &entry(format!("git+http://{address}/repository#v1")),
        &fixture.destination(),
        GitFetchLimits {
            timeout: Duration::from_millis(500),
            ..limits()
        },
    )
    .unwrap_err();
    assert!(error.contains("total deadline exceeded"), "{error}");
    assert!(started.elapsed() < Duration::from_secs(3));
    server.join().unwrap();
    assert!(!fixture.destination().exists());
    fixture.clean();
}

#[test]
fn release_git_fetch_refuses_unbounded_subprocess_diagnostics() {
    let fixture = Fixture::new();
    let repository = fixture.producer(b"package\n");
    let error = fetch_with_git_limits(
        &local(&repository, &"missing".repeat(600)),
        &fixture.destination(),
        GitFetchLimits {
            output_bytes: 512,
            ..limits()
        },
    )
    .unwrap_err();
    assert!(
        error.contains("subprocess output exceeds 512 bytes"),
        "{error}"
    );
    assert!(
        error.len() < 1024,
        "diagnostics escaped their memory/error bound"
    );
    assert!(!fixture.destination().exists());
    fixture.clean();
}

#[test]
fn release_git_fetch_refuses_archive_overflow_without_publishing_partial_bytes() {
    let fixture = Fixture::new();
    let repository = fixture.producer(&vec![b'x'; 64 * 1024]);
    fs::create_dir_all(fixture.destination().parent().unwrap()).unwrap();
    fs::write(fixture.destination(), b"previous verified artifact").unwrap();
    let error = fetch_with_git_limits(
        &local(&repository, "v1"),
        &fixture.destination(),
        GitFetchLimits {
            archive_bytes: 1024,
            ..limits()
        },
    )
    .unwrap_err();
    assert!(error.contains("archive exceeds 1024 bytes"), "{error}");
    assert_eq!(
        fs::read(fixture.destination()).unwrap(),
        b"previous verified artifact"
    );
    fixture.clean();
}

#[test]
fn release_git_fetch_refuses_checkout_disk_overflow_and_removes_incomplete_clone() {
    let fixture = Fixture::new();
    // SHA-256 blocks produce deterministic incompressible source bytes in a real pack.
    let bytes = (0_u32..16_384)
        .flat_map(|index| Sha256::digest(index.to_le_bytes()).to_vec())
        .collect::<Vec<_>>();
    let repository = fixture.producer(&bytes);
    let error = fetch_with_git_limits(
        &local(&repository, "v1"),
        &fixture.destination(),
        GitFetchLimits {
            checkout_bytes: 32 * 1024,
            ..limits()
        },
    )
    .unwrap_err();
    assert!(
        error.contains("checkout exceeds disk budget")
            || error.contains("File size limit")
            || error.contains("signal: 25"),
        "{error}"
    );
    assert!(!fixture.destination().exists());
    fixture.clean();
}

#[test]
fn release_git_fetch_failed_clone_cleans_up_and_can_be_retried() {
    let fixture = Fixture::new();
    let repository = fixture.producer(b"package\n");
    let error = fetch_with_git_limits(
        &local(&repository, "missing"),
        &fixture.destination(),
        limits(),
    )
    .unwrap_err();
    assert!(error.contains("git exited with"), "{error}");
    assert!(!fixture.destination().exists());
    fixture.clean();
    fetch_with_git_limits(&local(&repository, "v1"), &fixture.destination(), limits()).unwrap();
    assert!(!fs::read(fixture.destination()).unwrap().is_empty());
    fixture.clean();
}
