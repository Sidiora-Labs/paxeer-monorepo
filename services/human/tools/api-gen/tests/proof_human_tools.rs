//! Proves: human_api_ts_client / write_client -> layerx-human-api-gen subprocess -> human-gen-api / platform-human-api-generate / human_api_drift_gate.
//!
//! Repairs: services/human/tools/api-gen/src/lib.rs::parse_kvx now refuses
//! duplicate sections, and services/human/tools/api-gen/src/main.rs::main
//! rejects excess CLI arguments before writing. Makefile and
//! services/platform/Makefile.inc now share the real generation recipe and expose
//! a focused drift consumer. Every subprocess uses Cargo's built binary; no
//! cargo command, schema parser, generator, or drift gate is replaced.
//! There is no persistent service state: each invocation starts a new process,
//! and corrupting its output between invocations proves generation runs again.

use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::sync::atomic::{AtomicUsize, Ordering};

use layerx_human_api_gen::{
    human_api_drift_gate, human_api_ts_client, GeneratedClient, GENERATED_FILES,
};

static NEXT: AtomicUsize = AtomicUsize::new(0);

fn repository() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("locate repository: {error}"))
}

struct Proof {
    root: PathBuf,
    schema: PathBuf,
    output: PathBuf,
}

impl Proof {
    fn new() -> Self {
        let root = std::env::temp_dir().join(format!(
            "human tools proof {} {}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap_or_else(|error| panic!("create proof directory: {error}"));
        let schema = root.join("schema");
        fs::create_dir(&schema).unwrap_or_else(|error| panic!("create schema directory: {error}"));
        let source = repository().join("services/human/schema/human-api");
        for entry in fs::read_dir(source).unwrap_or_else(|error| panic!("read schema: {error}")) {
            let entry = entry.unwrap_or_else(|error| panic!("read schema entry: {error}"));
            if entry.path().extension().is_some_and(|value| value == "kvx") {
                fs::copy(entry.path(), schema.join(entry.file_name()))
                    .unwrap_or_else(|error| panic!("copy real schema: {error}"));
            }
        }
        let output = root.join("generated client");
        Self {
            root,
            schema,
            output,
        }
    }

    fn expected(&self) -> GeneratedClient {
        human_api_ts_client(&self.schema)
            .unwrap_or_else(|violations| panic!("produce real client: {violations:#?}"))
    }

    fn make(&self, target: &str) -> Output {
        // Override only the executable location: the recipe, arguments, process
        // boundary, file writes and consuming drift gate are all production code.
        let binary = env!("CARGO_BIN_EXE_layerx-human-api-gen")
            .replace('\'', "'\\''")
            .replace('$', "$$");
        Command::new("make")
            .current_dir(repository())
            .env_remove("MAKEFLAGS")
            .env_remove("MFLAGS")
            .args(["--no-print-directory", "-f", "Makefile", target])
            .arg(format!("HUMAN_API_GEN='{binary}'"))
            .arg(format!("HUMAN_API_SCHEMA_DIR={}", self.schema.display()))
            .arg(format!("HUMAN_API_OUT_DIR={}", self.output.display()))
            .output()
            .unwrap_or_else(|error| panic!("run real make consumer: {error}"))
    }

    fn cli(&self, checking: bool, extra: bool) -> Output {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-human-api-gen"));
        if checking {
            command.arg("--check");
        }
        command.arg(&self.schema).arg(&self.output);
        if extra {
            command.arg("unexpected-argument");
        }
        command
            .output()
            .unwrap_or_else(|error| panic!("run real CLI: {error}"))
    }

    fn assert_files(&self, expected: &GeneratedClient) {
        assert_eq!(expected.files.len(), GENERATED_FILES.len());
        let mut actual = fs::read_dir(&self.output)
            .unwrap_or_else(|error| panic!("read generated directory: {error}"))
            .map(|entry| {
                entry
                    .unwrap_or_else(|error| panic!("read output entry: {error}"))
                    .file_name()
            })
            .collect::<Vec<_>>();
        actual.sort();
        assert_eq!(
            actual,
            GENERATED_FILES
                .iter()
                .map(|name| std::ffi::OsString::from(*name))
                .collect::<Vec<_>>()
        );
        for (name, body) in &expected.files {
            let observed = fs::read_to_string(self.output.join(name))
                .unwrap_or_else(|error| panic!("read generated {name}: {error}"));
            assert_eq!(&observed, body, "consumer must observe producer's {name}");
        }
    }
}

impl Drop for Proof {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.root);
    }
}

fn assert_success(output: &Output) {
    assert!(
        output.status.success(),
        "status={}\nstdout={}\nstderr={}",
        output.status,
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
}

fn assert_refusal(output: &Output, diagnostic: &str) {
    assert!(!output.status.success(), "invalid input was accepted");
    assert!(
        String::from_utf8_lossy(&output.stderr).contains(diagnostic),
        "expected {diagnostic}: {}",
        String::from_utf8_lossy(&output.stderr)
    );
}

#[test]
fn proof_human_tools_make_consumers_regenerate_and_check_twice() {
    let proof = Proof::new();
    let expected = proof.expected();
    assert!(expected.operations > 0);
    assert!(expected.types > 0);
    assert!(expected.retired() > 0);

    for target in ["human-gen-api", "platform-human-api-generate"] {
        for _ in 0..2 {
            let produced = proof.make(target);
            assert_success(&produced);
            assert!(String::from_utf8_lossy(&produced.stdout).contains(&format!(
                "generated {} file(s) covering {} operations and {} types, skipping {} retired operations",
                expected.files.len(), expected.operations, expected.types, expected.retired()
            )));
            proof.assert_files(&expected);
            assert_eq!(
                human_api_drift_gate(&proof.schema, &proof.output),
                Ok(proof.expected())
            );
            let consumed = proof.make("human-check-api");
            assert_success(&consumed);
            assert!(
                String::from_utf8_lossy(&consumed.stdout).contains("human-api client is fresh:")
            );

            // Damage a real producer result at the consumer boundary. Both the
            // library and Make/CLI consumer must refuse it without rewriting it.
            let path = proof.output.join("index.ts");
            let mut malformed =
                fs::read(&path).unwrap_or_else(|error| panic!("read output: {error}"));
            malformed.extend_from_slice(b"\nexport const broken = ;\n");
            fs::write(&path, &malformed).unwrap_or_else(|error| panic!("damage output: {error}"));
            let violations = human_api_drift_gate(&proof.schema, &proof.output).unwrap_err();
            assert!(violations
                .iter()
                .any(|entry| entry.path == path && entry.rule == "stale-or-hand-edited-output"));
            assert_refusal(
                &proof.make("human-check-api"),
                "stale-or-hand-edited-output",
            );
            assert_eq!(
                fs::read(&path).unwrap_or_else(|error| panic!("read refused output: {error}")),
                malformed
            );
            // The next generation must execute and repair this corruption.
        }
    }
}

#[test]
fn proof_human_tools_malformed_schema_refuses_without_publishing() {
    let proof = Proof::new();
    let expected = proof.expected();
    assert_success(&proof.cli(false, false));
    let path = proof.schema.join("v1.kvx");
    let mut body = fs::read_to_string(&path).unwrap_or_else(|error| panic!("read schema: {error}"));
    body.push_str("\n[schema]\n");
    fs::write(&path, body).unwrap_or_else(|error| panic!("damage schema: {error}"));

    let violations = human_api_ts_client(&proof.schema).unwrap_err();
    assert!(violations.iter().any(|entry| entry.path == path
        && entry.rule == "invalid-kvx"
        && entry.detail.contains("duplicate section schema")));
    for checking in [false, true] {
        let refused = proof.cli(checking, false);
        assert_eq!(refused.status.code(), Some(1));
        assert_refusal(&refused, "duplicate section schema");
        proof.assert_files(&expected);
    }
    for target in [
        "human-gen-api",
        "platform-human-api-generate",
        "human-check-api",
    ] {
        assert_refusal(&proof.make(target), "invalid-kvx");
        proof.assert_files(&expected);
    }
}

#[test]
fn proof_human_tools_excess_arguments_refuse_before_writing() {
    let proof = Proof::new();
    for checking in [false, true] {
        let refused = proof.cli(checking, true);
        assert_eq!(refused.status.code(), Some(1));
        assert_refusal(&refused, "usage: layerx-human-api-gen");
        assert!(
            !proof.output.exists(),
            "argument refusal must precede generation"
        );
    }
}
