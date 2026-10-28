#[allow(dead_code)]
#[path = "../src/main.rs"]
mod pipeline;

use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

const RECEIPT_OUTPUTS: [&str; 8] = [
    "services/agent/crates/layerx-sdk/src/receipt_generated.rs",
    "services/agent/sdk/typescript/src/generated/receipt.ts",
    "services/agent/sdk/python/layerx_sdk/generated/receipt.py",
    "services/agent/sdk/python/layerx_sdk/generated/receipt.pyi",
    "sdk/platform/go/receipt_generated.go",
    "sdk/platform/jvm/src/main/java/com/sidiora/layerx/sdk/verify/GeneratedReceiptContract.java",
    "sdk/platform/swift/Sources/LayerXSDK/Generated/ReceiptContract.swift",
    "sdk/platform/dotnet/Generated/ReceiptContract.cs",
];

const CHECKS: [&str; 12] = [
    "decode",
    "canonical-encoding",
    "receipt-version",
    "activity-id",
    "key-id",
    "key-height",
    "raw-transaction",
    "fields",
    "return-data",
    "logs",
    "sequence-hash",
    "signature",
];

struct Repository(PathBuf);

impl Repository {
    fn create(path: PathBuf) -> std::io::Result<Self> {
        fs::create_dir(&path)?;
        Ok(Self(path))
    }
}

impl Drop for Repository {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0)
            .unwrap_or_else(|error| panic!("remove {}: {error}", self.0.display()));
    }
}

fn copy_path(source: &Path, destination: &Path) {
    if source.is_dir() {
        fs::create_dir_all(destination)
            .unwrap_or_else(|error| panic!("create {}: {error}", destination.display()));
        for entry in fs::read_dir(source)
            .unwrap_or_else(|error| panic!("list {}: {error}", source.display()))
        {
            let entry = entry.unwrap_or_else(|error| panic!("read entry: {error}"));
            copy_path(&entry.path(), &destination.join(entry.file_name()));
        }
    } else {
        fs::create_dir_all(
            destination
                .parent()
                .unwrap_or_else(|| panic!("missing parent")),
        )
        .unwrap_or_else(|error| panic!("create parent: {error}"));
        fs::copy(source, destination)
            .unwrap_or_else(|error| panic!("copy {}: {error}", source.display()));
    }
}

fn read(root: &Path, relative: &str) -> String {
    fs::read_to_string(root.join(relative))
        .unwrap_or_else(|error| panic!("read {relative}: {error}"))
}

fn generator(root: &Path, mode: &str) -> Output {
    Command::new(env!("CARGO_BIN_EXE_layerx-platform-sdkgen"))
        .arg(mode)
        .arg(root)
        .output()
        .unwrap_or_else(|error| panic!("run {mode}: {error}"))
}

fn assert_success(output: &Output) {
    assert!(
        output.status.success(),
        "generator failed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

fn repository() -> Repository {
    let root = Repository::create(std::env::temp_dir().join(format!(
        "layerx-receipt-contract-{}-{}",
        std::process::id(),
        NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
    )))
    .unwrap_or_else(|error| panic!("create fixture: {error}"));
    let initialized = Command::new("git")
        .arg("-C")
        .arg(&root.0)
        .args(["init", "-q"])
        .output()
        .unwrap_or_else(|error| panic!("git init: {error}"));
    assert_success(&initialized);

    let source = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../..");
    let lock = "sdk/platform/pipeline.kvx";
    let pipeline = pipeline::parse_lock(&read(&source, lock))
        .unwrap_or_else(|error| panic!("parse repository lock: {error}"));
    for state in &pipeline.sources {
        copy_path(&source.join(&state.root), &root.0.join(&state.root));
    }
    for state in &pipeline.generators {
        let inputs: &[&str] = match state.name.as_str() {
            "platform-sdkgen" => &[
                "Cargo.toml",
                "generate_jvm.py",
                "generate_lifecycle.py",
                "generate_portable.py",
                "receipt.kvx",
                "src",
            ],
            "agent-sdk-gen" => &["Cargo.lock", "Cargo.toml", "src", "templates"],
            "human-api-gen" => &["Cargo.lock", "Cargo.toml", "src"],
            name => panic!("unknown generator {name}"),
        };
        for input in inputs {
            let relative = Path::new(&state.root).join(input);
            copy_path(&source.join(&relative), &root.0.join(&relative));
        }
    }
    for output in &pipeline.outputs {
        for (file, _) in &output.files {
            let relative = Path::new(&output.root).join(file);
            copy_path(&source.join(&relative), &root.0.join(&relative));
        }
    }
    for relative in [
        lock,
        "programs/sdk/rust/src/abi_policy.rs",
        "programs/crates/layerx-programs-runtime/src/terminal.rs",
    ] {
        copy_path(&source.join(relative), &root.0.join(relative));
    }
    assert_success(&generator(&root.0, "--write-program-contracts"));
    root
}

#[test]
fn repository_cleanup_requires_directory_ownership() {
    let path = std::env::temp_dir().join(format!(
        "layerx-receipt-contract-cleanup-{}-{}",
        std::process::id(),
        NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
    ));
    fs::create_dir(&path).unwrap_or_else(|error| panic!("create isolated root: {error}"));
    let root = Repository(path);
    let existing = root.0.join("existing");
    fs::create_dir(&existing).unwrap_or_else(|error| panic!("create existing fixture: {error}"));
    let sentinel = existing.join("sentinel");
    fs::write(&sentinel, b"keep this fixture")
        .unwrap_or_else(|error| panic!("write sentinel: {error}"));

    let error = Repository::create(existing.clone())
        .err()
        .unwrap_or_else(|| panic!("existing fixture must not be acquired"));
    assert_eq!(error.kind(), std::io::ErrorKind::AlreadyExists);
    assert!(existing.is_dir());
    assert_eq!(
        fs::read(&sentinel).unwrap_or_else(|error| panic!("read sentinel: {error}")),
        b"keep this fixture"
    );

    let created = root.0.join("created");
    let owned = Repository::create(created.clone())
        .unwrap_or_else(|error| panic!("create owned fixture: {error}"));
    fs::write(created.join("sentinel"), b"remove this fixture")
        .unwrap_or_else(|error| panic!("write owned sentinel: {error}"));
    drop(owned);
    assert!(!created.exists());
    assert!(sentinel.is_file());
}

#[test]
fn receipt_contract_lists_sequencer_checks() {
    let root = repository();
    for relative in RECEIPT_OUTPUTS {
        let rendered = read(&root.0, relative);
        let checks = rendered
            .lines()
            .filter(|line| {
                let trimmed = line.trim_start();
                match Path::new(relative)
                    .extension()
                    .and_then(|value| value.to_str())
                {
                    Some("rs") => trimmed.starts_with("Self::") && line.contains(" => \""),
                    Some("ts" | "py") => line.starts_with(' ') && line.contains(" = \""),
                    Some("pyi") => line.starts_with(' ') && trimmed.ends_with(": str"),
                    Some("go") => {
                        trimmed.starts_with("ReceiptCheck") && line.contains("ReceiptCheck = ")
                    }
                    Some("java") => line.contains("(\""),
                    Some("swift") => trimmed.starts_with("case ") && line.contains(" = \""),
                    Some("cs") => trimmed.starts_with("ReceiptCheck.") && line.contains(" => \""),
                    extension => panic!("unexpected receipt contract extension: {extension:?}"),
                }
            })
            .map(|line| {
                if relative.ends_with(".pyi") {
                    line.trim()
                        .strip_suffix(": str")
                        .unwrap_or_else(|| panic!("missing check declaration: {line}"))
                        .to_ascii_lowercase()
                        .replace('_', "-")
                } else {
                    line.split('"')
                        .nth(1)
                        .unwrap_or_else(|| panic!("missing check: {line}"))
                        .to_owned()
                }
            })
            .collect::<Vec<_>>();
        assert_eq!(checks, CHECKS, "{relative}");
    }

    for (relative, declarations) in [
        (
            RECEIPT_OUTPUTS[4],
            [
                "const ReceiptVersionV2 uint16 = 2",
                "const ReceiptVersionV3 uint16 = 3",
                "const ReceiptDomain = \"LXP/v1/receipt\"",
                "const SequenceDomain = \"PAXEER/v1/sequence\"",
            ],
        ),
        (
            RECEIPT_OUTPUTS[5],
            [
                "public static final int RECEIPT_VERSION_V2 = 2;",
                "public static final int RECEIPT_VERSION_V3 = 3;",
                "public static final String RECEIPT_DOMAIN = \"LXP/v1/receipt\";",
                "public static final String SEQUENCE_DOMAIN = \"PAXEER/v1/sequence\";",
            ],
        ),
        (
            RECEIPT_OUTPUTS[6],
            [
                "public let receiptVersionV2: UInt16 = 2",
                "public let receiptVersionV3: UInt16 = 3",
                "public let receiptDomain = \"LXP/v1/receipt\"",
                "public let sequenceDomain = \"PAXEER/v1/sequence\"",
            ],
        ),
        (
            RECEIPT_OUTPUTS[7],
            [
                "public const ushort ReceiptVersionV2 = 2;",
                "public const ushort ReceiptVersionV3 = 3;",
                "public const string ReceiptDomain = \"LXP/v1/receipt\";",
                "public const string SequenceDomain = \"PAXEER/v1/sequence\";",
            ],
        ),
    ] {
        let rendered = read(&root.0, relative);
        for declaration in declarations {
            assert!(
                rendered.lines().any(|line| line.trim() == declaration),
                "{relative} missing {declaration}"
            );
        }
    }
    assert_success(&generator(&root.0, "--check-program-contracts"));
}

#[test]
fn receipt_contract_drops_programs_module() {
    let root = repository();
    for relative in RECEIPT_OUTPUTS {
        let rendered = read(&root.0, relative);
        let normalized = rendered.to_ascii_lowercase().replace('_', "");
        for retired in [
            "programsmoduleid",
            "programoutcome",
            "program-outcome",
            "requirednonzerochecks",
            "50524731",
            "50524732",
            "50524733",
            "50524734",
            "1347569457",
            "1347569458",
            "1347569459",
            "1347569460",
        ] {
            assert!(
                !normalized.contains(retired),
                "{relative} retains {retired}"
            );
        }
    }
}

#[test]
fn receipt_contract_check_detects_drift() {
    let root = repository();
    assert_success(&generator(&root.0, "--check-program-contracts"));
    for relative in RECEIPT_OUTPUTS {
        let original = read(&root.0, relative);
        let changed = if relative.ends_with(".pyi") {
            original.replacen("DECODE: str", "EDITED_DECODE: str", 1)
        } else {
            original.replacen("\"decode\"", "\"edited-decode\"", 1)
        };
        assert_ne!(changed, original, "{relative} has no decode check");
        fs::write(root.0.join(relative), changed)
            .unwrap_or_else(|error| panic!("edit {relative}: {error}"));
        let output = generator(&root.0, "--check-program-contracts");
        assert_eq!(output.status.code(), Some(1));
        let error = String::from_utf8_lossy(&output.stderr);
        assert!(
            error.contains(relative) && error.contains("stale or hand-edited"),
            "{error}"
        );
        fs::write(root.0.join(relative), original)
            .unwrap_or_else(|error| panic!("restore {relative}: {error}"));
    }
    assert_success(&generator(&root.0, "--check-program-contracts"));

    let source = "sdk/platform/generators/receipt.kvx";
    fs::write(
        root.0.join(source),
        read(&root.0, source).replace("\"signature\"", "\"changed-signature\""),
    )
    .unwrap_or_else(|error| panic!("edit receipt source: {error}"));
    let output = generator(&root.0, "--check-program-contracts");
    assert_eq!(output.status.code(), Some(1));
    let error = String::from_utf8_lossy(&output.stderr);
    assert!(
        error.contains(RECEIPT_OUTPUTS[0]) && error.contains("stale or hand-edited"),
        "{error}"
    );
}
