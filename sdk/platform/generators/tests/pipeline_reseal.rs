use sha2::{Digest, Sha256};
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};

const LOCK_PATH: &str = "sdk/platform/pipeline.kvx";
const VECTORS: [&str; 5] = [
    "vector-v1-receipted-positive.json",
    "vector-v2-receipted-revert.json",
    "vector-v3-receipt-refusals.json",
    "vector-v4-committed.json",
    "vector-v5-portable.json",
];

struct ResealedLock(PathBuf);

impl ResealedLock {
    fn create(source: &[u8]) -> Self {
        let directory =
            std::env::temp_dir().join(format!("layerx-pipeline-reseal-{}", std::process::id()));
        fs::create_dir(&directory)
            .unwrap_or_else(|error| panic!("create {}: {error}", directory.display()));
        let lock = Self(directory.join("pipeline.kvx"));
        fs::write(&lock.0, source)
            .unwrap_or_else(|error| panic!("seed {}: {error}", lock.0.display()));
        lock
    }
}

impl Drop for ResealedLock {
    fn drop(&mut self) {
        let directory = self
            .0
            .parent()
            .unwrap_or_else(|| panic!("lock has no parent"));
        fs::remove_dir_all(directory)
            .unwrap_or_else(|error| panic!("remove {}: {error}", directory.display()));
    }
}

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("resolve repository root: {error}"))
}

fn write_lock(root: &Path, lock: &Path) -> Output {
    Command::new(env!("CARGO_BIN_EXE_layerx-platform-sdkgen"))
        .arg("--write")
        .arg(root)
        .arg(lock)
        .output()
        .unwrap_or_else(|error| panic!("run generator --write: {error}"))
}

fn check(root: &Path) -> Output {
    Command::new(env!("CARGO_BIN_EXE_layerx-platform-sdkgen"))
        .arg("--check")
        .arg(root)
        .output()
        .unwrap_or_else(|error| panic!("run generator --check: {error}"))
}

fn assert_success(output: &Output) {
    assert!(
        output.status.success(),
        "generator exited {}\nstdout: {}\nstderr: {}",
        output.status,
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
}

fn retired_names() -> Vec<String> {
    let mut names = Vec::new();
    for kind in ["positive", "refusals"] {
        for version in 1..=3 {
            names.push(format!("receipt-{kind}-v{version}.json"));
        }
    }
    for case in [
        "positive-v1",
        "positive-v2",
        "positive-v3",
        "refusals-v2",
        "executed-v3",
        "executed-v4",
        "principal-v4",
        "mutated-leg-v4",
    ] {
        names.push(format!("receipt-programs-{case}.json"));
    }
    names.extend(
        [
            "generate_receipt_fixture.c",
            "generate_program_receipt_fixture.c",
            "generate_program_receipt_refusals.py",
            "generate_executed_program_fixture.py",
            "generate_executed_program_v4_fixture.py",
            "terminal-v4.test.py",
            "terminal_v4_test.go",
            "TerminalV4Test.java",
        ]
        .map(str::to_owned),
    );
    names
}

fn scan_conformance(directory: &Path, retired: &[String], violations: &mut Vec<String>) {
    for entry in fs::read_dir(directory)
        .unwrap_or_else(|error| panic!("read {}: {error}", directory.display()))
    {
        let entry = entry.unwrap_or_else(|error| panic!("read directory entry: {error}"));
        let path = entry.path();
        let kind = entry
            .file_type()
            .unwrap_or_else(|error| panic!("stat {}: {error}", path.display()));
        for name in retired {
            if entry.file_name() == name.as_str() {
                violations.push(format!("retired file still exists: {}", path.display()));
            }
        }
        if kind.is_dir() {
            scan_conformance(&path, retired, violations);
        } else {
            assert!(
                kind.is_file(),
                "unsupported conformance entry: {}",
                path.display()
            );
            let name = entry.file_name();
            if name.to_str().is_some_and(|name| {
                name.starts_with("kernel-fixtures-retired-") && name.ends_with(".test.py")
            }) {
                continue;
            }
            let source =
                fs::read(&path).unwrap_or_else(|error| panic!("read {}: {error}", path.display()));
            for name in retired {
                if source
                    .windows(name.len())
                    .any(|bytes| bytes == name.as_bytes())
                {
                    violations.push(format!("{} names {name}", path.display()));
                }
            }
        }
    }
}

#[test]
fn pipeline_lock_matches_repository() {
    let root = repository_root();
    assert_success(&check(&root));
    let committed = fs::read(root.join(LOCK_PATH))
        .unwrap_or_else(|error| panic!("read pipeline lock: {error}"));
    let resealed = ResealedLock::create(&committed);
    assert_success(&write_lock(&root, &resealed.0));
    let regenerated = fs::read(&resealed.0)
        .unwrap_or_else(|error| panic!("read resealed pipeline lock: {error}"));
    assert_eq!(regenerated, committed, "--write changed the pipeline lock");
}

#[test]
fn pipeline_lock_lists_no_kernel_receipt_fixture() {
    let root = repository_root();
    let recording =
        fs::read(root.join("daemon/committed-fixture/vectors/vector-v4-committed.json"))
            .unwrap_or_else(|error| panic!("read committed recording: {error}"));
    assert_eq!(
        format!("{:x}", Sha256::digest(&recording)),
        "5013cd232ee94ece8d25fca4c802ca60c892f3a5f637a70962d9909dc71c4e7b",
        "committed recording changed"
    );
    let conformance =
        fs::read_to_string(root.join("sdk/platform/conformance/fixtures/vector-v4-committed.json"))
            .unwrap_or_else(|error| panic!("read committed conformance: {error}"));
    assert!(conformance.contains(
        "\"sha256\": \"5013cd232ee94ece8d25fca4c802ca60c892f3a5f637a70962d9909dc71c4e7b\""
    ));
    let source = fs::read_to_string(root.join(LOCK_PATH))
        .unwrap_or_else(|error| panic!("read pipeline lock: {error}"));
    let lock = layerx_platform_kvx::parse(&source)
        .unwrap_or_else(|error| panic!("parse pipeline lock: {error}"));
    let mut violations = Vec::new();
    for prefix in [
        "receipt-positive-",
        "receipt-refusals-",
        "receipt-programs-",
    ] {
        assert!(!source.contains(prefix), "pipeline lock lists {prefix}");
    }
    for vector in VECTORS {
        let relative = format!("fixtures/{vector}");
        assert_eq!(
            lock.get("handwritten.platform-conformance", &relative),
            Some("\"handwritten\""),
            "pipeline lock must declare {relative}"
        );
        assert!(root
            .join("sdk/platform/conformance")
            .join(relative)
            .is_file());
    }
    let retired = retired_names();
    for name in &retired {
        if source.contains(name) {
            violations.push(format!("pipeline lock names {name}"));
        }
    }
    scan_conformance(
        &root.join("sdk/platform/conformance"),
        &retired,
        &mut violations,
    );
    violations.sort();
    assert!(
        violations.is_empty(),
        "retired kernel fixture references remain:\n{}",
        violations.join("\n")
    );
}
