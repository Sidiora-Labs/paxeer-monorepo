// Repair record: release-relative-git-fetch is covered by both tests below.
// Baseline verification was not run: the bounded gate order forbids a pre-change run.
// Wiring requests: none; the existing workflow's relative --fetch is supported in place.

use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};

use serde_json::{json, Value};
use sha2::{Digest, Sha256};

const VERSION: &str = "0.1.0";
const ARTIFACT: &str = "LayerXSDK-0.1.0.tar";

fn successful(command: &mut Command) -> Output {
    let output = command
        .output()
        .unwrap_or_else(|error| panic!("{command:?}: {error}"));
    assert!(
        output.status.success(),
        "{command:?}: {}\nstdout: {}\nstderr: {}",
        output.status,
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    output
}

fn digest(bytes: &[u8]) -> String {
    format!("sha256:{:x}", Sha256::digest(bytes))
}

fn image_record(root: &Path, archive: &[u8], revision: &str) -> Value {
    let blobs = root.join("image/blobs/sha256");
    fs::create_dir_all(&blobs).unwrap_or_else(|error| panic!("create image blobs: {error}"));
    let store = |bytes: &[u8]| {
        let hash = digest(bytes);
        fs::write(blobs.join(hash.trim_start_matches("sha256:")), bytes)
            .unwrap_or_else(|error| panic!("write image blob: {error}"));
        hash
    };
    let layer_digest = store(archive);
    let config = serde_json::to_vec(&json!({
        "architecture": "amd64", "os": "linux", "config": {},
        "rootfs": {"type": "layers", "diff_ids": [layer_digest]}
    }))
    .unwrap_or_else(|error| panic!("encode image config: {error}"));
    let config_digest = store(&config);
    let image = serde_json::to_vec(&json!({
        "schemaVersion": 2,
        "mediaType": "application/vnd.oci.image.manifest.v1+json",
        "config": {"mediaType": "application/vnd.oci.image.config.v1+json", "digest": config_digest, "size": config.len()},
        "layers": [{"mediaType": "application/vnd.oci.image.layer.v1.tar", "digest": layer_digest, "size": archive.len()}]
    })).unwrap_or_else(|error| panic!("encode image manifest: {error}"));
    let image_digest = store(&image);
    let sbom = serde_json::to_vec(&json!({
        "spdxVersion": "SPDX-2.3", "SPDXID": "SPDXRef-DOCUMENT",
        "name": "LayerXSDK", "documentNamespace": format!("https://example.invalid/spdx/{revision}")
    }))
    .unwrap_or_else(|error| panic!("encode SBOM: {error}"));
    let sbom_digest = store(&sbom);
    let attestation = serde_json::to_vec(&json!({
        "_type": "https://in-toto.io/Statement/v1",
        "subject": [{"name": "layerx-sdk", "digest": {"sha256": image_digest.trim_start_matches("sha256:")}}],
        "predicateType": "https://slsa.dev/provenance/v1",
        "predicate": {"buildDefinition": {"buildType": "git-archive", "externalParameters": {"revision": revision}}}
    })).unwrap_or_else(|error| panic!("encode provenance: {error}"));
    let attestation_digest = store(&attestation);
    json!({
        "name": "layerx-sdk", "version": VERSION,
        "registry": "ghcr", "distribution": "ghcr.io/sidiora-labs",
        "repository": "ghcr.io/sidiora-labs/layerx-sdk", "tag": format!("sdk-v{VERSION}"),
        "reference": format!("ghcr.io/sidiora-labs/layerx-sdk@{image_digest}"),
        "digest": image_digest, "platform": "linux/amd64",
        "signature": format!("oci-referrer:{image_digest}"),
        "sbom": format!("oci-referrer:{sbom_digest}"), "sbom_digest": sbom_digest,
        "attestation": format!("oci-referrer:{attestation_digest}"), "attestation_digest": attestation_digest,
        "source_revision": revision, "rollback_version": null, "published": true, "pull_check": true
    })
}

struct Producer {
    root: PathBuf,
    caller: PathBuf,
    manifest_path: PathBuf,
    manifest: Value,
    archive: Vec<u8>,
    revision: String,
}

impl Producer {
    fn new(case: &str) -> Self {
        let root = std::path::absolute(Path::new(env!("CARGO_TARGET_TMPDIR")))
            .unwrap_or_else(|error| panic!("resolve test directory: {error}"))
            .join(format!("repair-22-31-201-{}-{case}", std::process::id()));
        fs::create_dir(&root).unwrap_or_else(|error| panic!("create {}: {error}", root.display()));
        let repository = root.join("tagged producer");
        let caller = root.join("caller workspace");
        fs::create_dir(&repository).unwrap_or_else(|error| panic!("create producer: {error}"));
        fs::create_dir(&caller).unwrap_or_else(|error| panic!("create caller: {error}"));
        successful(
            Command::new("git")
                .arg("init")
                .arg("--quiet")
                .arg(&repository),
        );
        fs::write(repository.join("Package.swift"), b"// swift-tools-version:5.9\nimport PackageDescription\nlet package = Package(name: \"LayerXSDK\")\n")
            .unwrap_or_else(|error| panic!("write package: {error}"));
        successful(
            Command::new("git")
                .arg("-C")
                .arg(&repository)
                .args(["add", "Package.swift"]),
        );
        Self::commit(&repository, "Publish Swift package");
        successful(Command::new("git").arg("-C").arg(&repository).args([
            "tag",
            "--no-sign",
            VERSION,
        ]));
        let revision = String::from_utf8(
            successful(
                Command::new("git")
                    .arg("-C")
                    .arg(&repository)
                    .args(["rev-parse", VERSION]),
            )
            .stdout,
        )
        .unwrap_or_else(|error| panic!("decode revision: {error}"))
        .trim()
        .to_owned();
        let archive = successful(Command::new("git").arg("-C").arg(&repository).args([
            "archive",
            "--format=tar",
            VERSION,
        ]))
        .stdout;
        // Leave HEAD ahead of the published tag so the consumer must clone the tag.
        fs::write(repository.join("Package.swift"), b"unreleased changes\n")
            .unwrap_or_else(|error| panic!("write unreleased package: {error}"));
        successful(
            Command::new("git")
                .arg("-C")
                .arg(&repository)
                .args(["add", "Package.swift"]),
        );
        Self::commit(&repository, "Prepare next release");

        let archive_digest = digest(&archive);
        let manifest = json!({
            "schema": "layerx/artifact-manifest/1",
            "version": VERSION,
            "tag": format!("sdk-v{VERSION}"),
            "source_revision": revision,
            "source_digest": archive_digest,
            "rollback_version": null,
            "artifacts": [{
                "name": "LayerXSDK", "version": VERSION,
                "registry": "swiftpm", "distribution": "git", "target": "git",
                "artifact": ARTIFACT, "retained": ARTIFACT,
                "digest": archive_digest, "digest_of": "built-bytes",
                "signature": "tag-signature.txt", "sbom": "LayerXSDK.sbom.spdx.json",
                "attestation": "provenance.sigstore.json",
                "location": format!("git+file://{}#{VERSION}", repository.display()),
                "source_revision": revision, "rollback_version": null,
                "published": true, "install_check": true
            }],
            "images": [image_record(&root, &archive, &revision)]
        });
        let manifest_path = caller.join("artifact-manifest.json");
        let producer = Self {
            root,
            caller,
            manifest_path,
            manifest,
            archive,
            revision,
        };
        producer.write_manifest(&producer.manifest);
        producer
    }

    fn commit(repository: &Path, message: &str) {
        successful(Command::new("git").arg("-C").arg(repository).args([
            "-c",
            "user.name=Release integration test",
            "-c",
            "user.email=release-test@example.invalid",
            "-c",
            "commit.gpgsign=false",
            "commit",
            "--quiet",
            "-m",
            message,
        ]));
    }

    fn write_manifest(&self, manifest: &Value) {
        fs::write(
            &self.manifest_path,
            serde_json::to_vec_pretty(manifest)
                .unwrap_or_else(|error| panic!("encode manifest: {error}")),
        )
        .unwrap_or_else(|error| panic!("write manifest: {error}"));
    }

    fn verify(&self, directory: &Path) -> Command {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-platform-release"));
        command
            .current_dir(&self.caller)
            .arg("verify")
            .arg("--manifest")
            .arg(&self.manifest_path)
            .arg("--fetch")
            .arg(directory)
            .arg("--source-revision")
            .arg(&self.revision);
        command
    }
}

impl Drop for Producer {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root)
            .unwrap_or_else(|error| panic!("remove {}: {error}", self.root.display()));
    }
}

fn check_fetch(case: &str, absolute: bool) {
    let producer = Producer::new(case);
    let relative = Path::new("build/release/verification");
    let selected = producer.caller.join(relative);
    let directory = if absolute {
        selected.as_path()
    } else {
        relative
    };
    assert!(!selected.exists());
    let output = successful(&mut producer.verify(directory));
    let stdout = String::from_utf8(output.stdout)
        .unwrap_or_else(|error| panic!("decode verification: {error}"));
    let expected_digest = producer.manifest["artifacts"][0]["digest"]
        .as_str()
        .unwrap_or_else(|| panic!("manifest has no artifact digest"));
    assert!(
        stdout.contains(&format!(
            "verified LayerXSDK@{VERSION} from swiftpm {expected_digest}"
        )),
        "{stdout}"
    );
    let artifact = selected.join("swiftpm").join(ARTIFACT);
    let emitted =
        fs::read(&artifact).unwrap_or_else(|error| panic!("read {}: {error}", artifact.display()));
    assert_eq!(emitted, producer.archive);
    assert_eq!(digest(&emitted), expected_digest);
    assert!(!artifact.with_extension("tar.checkout").exists());

    // Fetch afresh with a conflicting manifest digest: retrieval must still succeed,
    // and verification must reject the real producer's bytes in the selected directory.
    let mut conflicting = producer.manifest.clone();
    let conflicting_digest = digest(b"unreleased changes\n");
    assert_ne!(conflicting_digest, expected_digest);
    conflicting["artifacts"][0]["digest"] = json!(conflicting_digest);
    producer.write_manifest(&conflicting);
    fs::remove_file(&artifact).unwrap_or_else(|error| panic!("remove archive: {error}"));
    let output = producer
        .verify(directory)
        .output()
        .unwrap_or_else(|error| panic!("verify conflicting manifest: {error}"));
    assert_eq!(output.status.code(), Some(1));
    let stderr = String::from_utf8_lossy(&output.stderr);
    assert!(stderr.contains("LayerXSDK@0.1.0 from swiftpm"), "{stderr}");
    assert!(
        stderr.contains(&format!(
            "hash to {expected_digest}, but the manifest lists {conflicting_digest}"
        )),
        "{stderr}"
    );
    assert_eq!(
        fs::read(&artifact).unwrap_or_else(|error| panic!("read refetched archive: {error}")),
        producer.archive
    );
    assert!(!artifact.with_extension("tar.checkout").exists());
}

#[test]
fn release_relative_git_fetch_uses_the_caller_directory_and_checks_digest() {
    check_fetch("relative", false);
}

#[test]
fn release_absolute_git_fetch_uses_the_caller_directory_and_checks_digest() {
    check_fetch("absolute", true);
}
