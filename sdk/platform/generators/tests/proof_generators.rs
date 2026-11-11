//! Exercises the receipt contract generator and the production contract checker.
//! The writer reads the receipt schema, Programs schema, and canonical ABI policy.
//! Each check launches a new process and reads its exit status and diagnostics.
//! The focused mode shares `check_program_contracts` with the full suite.

use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

const RECEIPT: &str = "sdk/platform/generators/receipt.kvx";
const INPUTS: [&str; 3] = [
    RECEIPT,
    "services/agent/schema/agent-api/programs.kvx",
    "programs/sdk/rust/src/abi_policy.rs",
];
const OUTPUTS: [&str; 8] = [
    "services/agent/crates/layerx-sdk/src/receipt_generated.rs",
    "services/agent/sdk/typescript/src/generated/receipt.ts",
    "services/agent/sdk/python/layerx_sdk/generated/receipt.py",
    "services/agent/sdk/python/layerx_sdk/generated/receipt.pyi",
    "sdk/platform/go/receipt_generated.go",
    "sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/GeneratedReceiptContract.java",
    "sdk/platform/swift/Sources/LayerXSDK/Generated/ReceiptContract.swift",
    "sdk/platform/dotnet/Generated/ReceiptContract.cs",
];
const RETIRED: [&str; 5] = [
    "sdk/platform/generators/generate_lifecycle.py",
    "sdk/platform/go/program_lifecycle_generated.go",
    "sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/ProgramLifecycleRoutes.java",
    "sdk/platform/swift/Sources/LayerXSDK/Generated/ProgramLifecycleRoutes.swift",
    "sdk/platform/dotnet/Generated/ProgramLifecycleRoutes.cs",
];

struct Repository(PathBuf);

impl Repository {
    fn create() -> Self {
        let path = std::env::temp_dir().join(format!(
            "layerx generators proof {} {}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap_or_else(|error| panic!("create {}: {error}", path.display()));
        let root = Self(path);
        for relative in INPUTS {
            let destination = root.0.join(relative);
            let parent = destination
                .parent()
                .unwrap_or_else(|| panic!("no parent for {relative}"));
            fs::create_dir_all(parent)
                .unwrap_or_else(|error| panic!("create {relative} parent: {error}"));
            fs::copy(repository_root().join(relative), destination)
                .unwrap_or_else(|error| panic!("copy real input {relative}: {error}"));
        }
        root
    }

    fn generator(&self, mode: &str) -> Output {
        Command::new(env!("CARGO_BIN_EXE_layerx-platform-sdkgen"))
            .current_dir(&self.0)
            .arg(mode)
            .arg(&self.0)
            .arg(self.0.join("optional lock.kvx"))
            .output()
            .unwrap_or_else(|error| panic!("run {mode}: {error}"))
    }

    fn consumer(&self) -> Output {
        Command::new("bash")
            .arg(repository_root().join("tools/paxeer-x/gates/104.38.6.sh"))
            .arg("--check-program-contracts")
            .arg(&self.0)
            .arg(env!("CARGO_BIN_EXE_layerx-platform-sdkgen"))
            .output()
            .unwrap_or_else(|error| panic!("run receipt parity consumer: {error}"))
    }

    fn write(&self) {
        assert_success(&self.generator("--write-program-contracts"));
        assert!(!self.0.join("optional lock.kvx").exists());
    }

    fn observe_twice(&self) {
        let before = self.snapshot();
        for _ in 0..2 {
            let output = self.consumer();
            assert_success(&output);
            let stdout = String::from_utf8(output.stdout)
                .unwrap_or_else(|error| panic!("decode consumer stdout: {error}"));
            assert!(
                stdout.contains(env!("CARGO_BIN_EXE_layerx-platform-sdkgen")),
                "consumer must report the real binary it invoked: {stdout}"
            );
            assert!(stdout.contains(&format!("--check-program-contracts {}", self.0.display())));
            assert_eq!(
                stdout
                    .matches("== seven-language receipt contract drift:")
                    .count(),
                1
            );
            assert!(
                stdout.ends_with("PAXEER_X_GATE tests=1 skipped=0\n"),
                "{stdout}"
            );
            assert_eq!(
                self.snapshot(),
                before,
                "checking must not repair the producer's output"
            );
        }
    }

    fn snapshot(&self) -> Vec<Vec<u8>> {
        OUTPUTS.iter().map(|relative| self.read(relative)).collect()
    }

    fn read(&self, relative: &str) -> Vec<u8> {
        fs::read(self.0.join(relative)).unwrap_or_else(|error| panic!("read {relative}: {error}"))
    }

    fn put(&self, relative: &str, bytes: impl AsRef<[u8]>) {
        fs::write(self.0.join(relative), bytes)
            .unwrap_or_else(|error| panic!("write {relative}: {error}"));
    }
}

impl Drop for Repository {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0)
            .unwrap_or_else(|error| panic!("remove {}: {error}", self.0.display()));
    }
}

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../..")
}

fn assert_success(output: &Output) {
    assert!(
        output.status.success(),
        "exit {:?}\nstdout: {}\nstderr: {}",
        output.status.code(),
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
}

fn assert_consumer_refusal(output: &Output, reason: &str) {
    assert_eq!(output.status.code(), Some(78), "{output:?}");
    let error = String::from_utf8_lossy(&output.stderr);
    assert!(error.contains(reason), "missing {reason:?}: {error}");
    assert!(
        error.contains("SDK receipt parity qualification refused:")
            && error.contains("seven-language receipt contract drift failed with exit 1"),
        "{error}"
    );
    assert!(!String::from_utf8_lossy(&output.stdout).contains("PAXEER_X_GATE tests="));
}

#[test]
fn proof_generators_write_crosses_real_gate_repeatedly_and_refuses_corrupt_output() {
    let root = Repository::create();
    for relative in OUTPUTS {
        assert!(
            !root.0.join(relative).exists(),
            "outputs must come from the writer"
        );
    }
    root.write();
    let original = root.snapshot();
    for (relative, bytes) in OUTPUTS.iter().zip(&original) {
        let text = std::str::from_utf8(bytes)
            .unwrap_or_else(|error| panic!("decode generated {relative}: {error}"));
        assert!(text.contains("Code generated from sdk/platform/generators/receipt.kvx"));
        assert!(
            !text.contains("/v1/programs"),
            "retired route in {relative}"
        );
        assert!(
            !text.contains("ProgramOutcome"),
            "retired receipt in {relative}"
        );
    }
    for relative in RETIRED {
        assert!(!root.0.join(relative).exists(), "writer revived {relative}");
        assert!(
            !repository_root().join(relative).exists(),
            "retired producer still shipped: {relative}"
        );
    }
    root.observe_twice();

    // These are corruptions of bytes emitted by the real writer, not fixtures
    // substituting for it. Each reader must reject malformed UTF-8 on decoding.
    for (relative, bytes) in OUTPUTS.iter().zip(&original) {
        let mut malformed = bytes.clone();
        malformed.push(0xff);
        root.put(relative, &malformed);
        let refused = root.consumer();
        assert_consumer_refusal(&refused, relative);
        assert!(String::from_utf8_lossy(&refused.stderr).contains("valid UTF-8"));
        assert_eq!(
            root.read(relative),
            malformed,
            "check rewrote invalid output"
        );
        root.write();
        assert_eq!(
            root.snapshot(),
            original,
            "writer did not restore its own output"
        );
        root.observe_twice();
    }

    // Both commands read the current configuration, even after earlier success.
    let source = String::from_utf8(root.read(RECEIPT))
        .unwrap_or_else(|error| panic!("decode receipt source: {error}"));
    let changed = source.replace("LXP/v1/receipt", "LXP/proof/receipt");
    assert_ne!(changed, source);
    root.put(RECEIPT, changed);
    assert_consumer_refusal(&root.consumer(), "stale or hand-edited");
    assert_eq!(root.snapshot(), original);
    root.write();
    let regenerated = root.snapshot();
    for ((relative, before), after) in OUTPUTS.iter().zip(&original).zip(&regenerated) {
        if Path::new(relative)
            .extension()
            .is_some_and(|ext| ext.eq_ignore_ascii_case("pyi"))
        {
            assert_eq!(
                before, after,
                "stub declarations do not carry domain values"
            );
        } else {
            assert_ne!(before, after, "{relative} ignored the new receipt domain");
            assert!(std::str::from_utf8(after)
                .unwrap_or_else(|error| panic!("decode {relative}: {error}"))
                .contains("LXP/proof/receipt"));
        }
    }
    root.observe_twice();
}

#[test]
fn proof_generators_refuse_malformed_shared_configuration_without_writing() {
    let root = Repository::create();
    root.write();
    let original = root.snapshot();
    let source = String::from_utf8(root.read(RECEIPT))
        .unwrap_or_else(|error| panic!("decode receipt source: {error}"));
    let invalid_sources = [
        (
            source.replace("[\"2\", \"3\"]", "[\"2\", \"4\"]"),
            "receipt.receipt_versions must be [2, 3]",
        ),
        (
            source.replace("LXP/v1/receipt", ""),
            "receipt domains must be non-empty",
        ),
        (
            source.replace("\"signature\"", "\"decode\""),
            "receipt.failure_checks must be non-empty and unique",
        ),
        (
            format!("{source}\n[program_lifecycle.deploy]\noperation = \"deploy\"\n"),
            "retired section [program_lifecycle.deploy] is forbidden",
        ),
    ];
    for (malformed, reason) in invalid_sources {
        assert_ne!(malformed, source);
        root.put(RECEIPT, malformed);
        let writer = root.generator("--write-program-contracts");
        assert_eq!(writer.status.code(), Some(1), "{writer:?}");
        assert!(
            String::from_utf8_lossy(&writer.stderr).contains(reason),
            "{writer:?}"
        );
        assert_consumer_refusal(&root.consumer(), reason);
        assert_eq!(
            root.snapshot(),
            original,
            "malformed source changed outputs"
        );
    }
    root.put(RECEIPT, source);
    root.observe_twice();
}

#[test]
fn proof_generators_refuse_excess_arguments_before_reading_or_writing() {
    let root = Repository::create();
    root.write();
    let original = root.snapshot();
    for mode in ["--write-program-contracts", "--check-program-contracts"] {
        for repository in [&root.0, &root.0.join("absent repository")] {
            let output = Command::new(env!("CARGO_BIN_EXE_layerx-platform-sdkgen"))
                .arg(mode)
                .arg(repository)
                .arg(root.0.join("optional lock.kvx"))
                .arg("unexpected argument")
                .output()
                .unwrap_or_else(|error| panic!("run {mode}: {error}"));
            assert_eq!(output.status.code(), Some(1), "{output:?}");
            assert_eq!(
                String::from_utf8_lossy(&output.stderr),
                format!(
                "platform-sdkgen: usage: layerx-platform-sdkgen {mode} [repo-root] [lock-path]\n"
            )
            );
            assert_eq!(root.snapshot(), original);
            assert!(!root.0.join("optional lock.kvx").exists());
        }
        // Omitted optional arguments resolve against the actual working directory.
        let output = Command::new(env!("CARGO_BIN_EXE_layerx-platform-sdkgen"))
            .current_dir(&root.0)
            .arg(mode)
            .output()
            .unwrap_or_else(|error| panic!("run {mode} with defaults: {error}"));
        assert_success(&output);
        assert_eq!(root.snapshot(), original);
    }
    root.observe_twice();
}
