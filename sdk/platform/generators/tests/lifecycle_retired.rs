use std::collections::BTreeSet;
use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

const RECEIPT_SOURCE: &str = "sdk/platform/generators/receipt.kvx";
const INPUTS: [&str; 4] = [
    RECEIPT_SOURCE,
    "services/agent/schema/agent-api/programs.kvx",
    "programs/sdk/rust/src/abi_policy.rs",
    "programs/crates/layerx-programs-runtime/src/terminal.rs",
];
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
const RETIRED_PATHS: [&str; 5] = [
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
            "layerx-lifecycle-retired-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).unwrap_or_else(|error| panic!("create {}: {error}", path.display()));
        let root = Self(path);
        let source = repository_root();
        for relative in INPUTS {
            let destination = root.0.join(relative);
            fs::create_dir_all(destination.parent().unwrap_or_else(|| panic!("no parent")))
                .unwrap_or_else(|error| panic!("create parent for {relative}: {error}"));
            fs::copy(source.join(relative), destination)
                .unwrap_or_else(|error| panic!("copy {relative}: {error}"));
        }
        root
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

fn generate_lifecycle(root: &Path, mode: &str) -> Output {
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

fn files(root: &Path, directory: &Path) -> BTreeSet<PathBuf> {
    let mut result = BTreeSet::new();
    for entry in fs::read_dir(directory)
        .unwrap_or_else(|error| panic!("read {}: {error}", directory.display()))
    {
        let path = entry
            .unwrap_or_else(|error| panic!("read entry: {error}"))
            .path();
        if path.is_dir() {
            result.extend(files(root, &path));
        } else {
            result.insert(
                path.strip_prefix(root)
                    .unwrap_or_else(|error| panic!("relative path: {error}"))
                    .to_owned(),
            );
        }
    }
    result
}

fn receipt_outputs(root: &Path) -> Vec<Vec<u8>> {
    RECEIPT_OUTPUTS
        .iter()
        .map(|relative| {
            fs::read(root.join(relative)).unwrap_or_else(|error| panic!("read {relative}: {error}"))
        })
        .collect()
}

#[test]
fn program_contracts_write_no_lifecycle_routes() {
    let root = Repository::create();
    let expected = INPUTS
        .iter()
        .chain(RECEIPT_OUTPUTS.iter())
        .map(|relative| PathBuf::from(*relative))
        .collect::<BTreeSet<_>>();
    for mode in ["--write-program-contracts", "--check-program-contracts"] {
        assert_success(&generate_lifecycle(&root.0, mode));
        assert_eq!(files(&root.0, &root.0), expected, "{mode}");
        for relative in RECEIPT_OUTPUTS {
            let rendered = fs::read_to_string(root.0.join(relative))
                .unwrap_or_else(|error| panic!("read {relative}: {error}"));
            assert!(!rendered.is_empty(), "{relative}");
            assert!(!rendered.contains("/v1/programs"), "{relative}");
            assert!(!rendered.contains("ProgramLifecycleRoutes"), "{relative}");
        }
        for relative in RETIRED_PATHS {
            assert!(!root.0.join(relative).exists(), "{mode} wrote {relative}");
            assert!(
                !repository_root().join(relative).exists(),
                "retired file still shipped: {relative}"
            );
        }
    }

    let relative = RECEIPT_OUTPUTS[4];
    fs::write(root.0.join(relative), "package layerx\n")
        .unwrap_or_else(|error| panic!("edit {relative}: {error}"));
    let output = generate_lifecycle(&root.0, "--check-program-contracts");
    assert_eq!(output.status.code(), Some(1));
    let error = String::from_utf8_lossy(&output.stderr);
    assert!(
        error.contains(relative) && error.contains("stale or hand-edited"),
        "{error}"
    );
}

#[test]
fn receipt_source_rejects_lifecycle_sections() {
    let root = Repository::create();
    assert_success(&generate_lifecycle(&root.0, "--write-program-contracts"));
    let original = fs::read_to_string(root.0.join(RECEIPT_SOURCE))
        .unwrap_or_else(|error| panic!("read receipt source: {error}"));
    let outputs = receipt_outputs(&root.0);
    let original_files = files(&root.0, &root.0);
    for section in [
        "program_lifecycle",
        "program_lifecycle.deploy",
        "program_lifecycle.upgrade",
        "program_lifecycle.wind_down",
        "program_lifecycle.future",
        " program_lifecycle.deploy ",
    ] {
        for entries in [
            "",
            "operation = \"program.deploy\"\nordinal = 1\npath = \"/v1/programs/deploy\"\n",
        ] {
            fs::write(
                root.0.join(RECEIPT_SOURCE),
                format!("{original}\n[{section}]\n{entries}"),
            )
            .unwrap_or_else(|error| panic!("edit receipt source: {error}"));
            for mode in ["--write-program-contracts", "--check-program-contracts"] {
                let output = generate_lifecycle(&root.0, mode);
                assert_eq!(output.status.code(), Some(1), "{mode}: [{section}]");
                let error = String::from_utf8_lossy(&output.stderr);
                assert!(
                    error.contains(RECEIPT_SOURCE)
                        && error.contains(&format!(
                            "retired section [{}] is forbidden",
                            section.trim()
                        )),
                    "{mode}: {error}"
                );
                assert_eq!(receipt_outputs(&root.0), outputs, "{mode}: [{section}]");
                assert_eq!(
                    files(&root.0, &root.0),
                    original_files,
                    "{mode}: [{section}]"
                );
            }
        }
    }
    fs::write(root.0.join(RECEIPT_SOURCE), original)
        .unwrap_or_else(|error| panic!("restore receipt source: {error}"));
    assert_success(&generate_lifecycle(&root.0, "--check-program-contracts"));
}
