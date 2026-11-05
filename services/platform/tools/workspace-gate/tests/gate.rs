#[allow(dead_code)]
#[path = "../src/main.rs"]
mod gate;

use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

use gate::workspace_gate;

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(1);

fn directory(label: &str) -> PathBuf {
    let sequence = NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-platform-gate-{label}-{}-{sequence}",
        std::process::id()
    ))
}

fn place(root: &Path, relative: &str) {
    let path = root.join(relative);
    let parent = path
        .parent()
        .unwrap_or_else(|| panic!("no parent for {relative}"));
    fs::create_dir_all(parent).unwrap_or_else(|error| panic!("create {relative}: {error}"));
    fs::copy(repository().join(relative), &path)
        .unwrap_or_else(|error| panic!("copy committed {relative}: {error}"));
}

fn repository() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../..")
}

fn manifest() -> String {
    let path = repository().join(gate::MANIFEST_PATH);
    fs::read_to_string(&path).unwrap_or_else(|error| panic!("read {}: {error}", path.display()))
}

fn declared_directories(source: &str) -> Vec<String> {
    let document = layerx_platform_kvx::parse(source)
        .unwrap_or_else(|error| panic!("parse committed manifest: {error}"));
    document
        .sections()
        .into_iter()
        .filter(|section| section.starts_with("dir."))
        .map(|section| declaration(source, section, "path"))
        .collect()
}

fn declaration(source: &str, section: &str, key: &str) -> String {
    let document = layerx_platform_kvx::parse(source)
        .unwrap_or_else(|error| panic!("parse manifest: {error}"));
    let value = document
        .required(section, key)
        .unwrap_or_else(|error| panic!("read declaration: {error}"));
    layerx_platform_kvx::unquote(value)
        .unwrap_or_else(|error| panic!("read {section}.{key}: {error}"))
}

fn repo_fixture(label: &str) -> PathBuf {
    let root = directory(label);
    let source = manifest();
    let declared = declared_directories(&source);
    assert!(
        !declared.is_empty(),
        "committed manifest declares no workspace directory"
    );
    for relative in &declared {
        fs::create_dir_all(root.join(relative))
            .unwrap_or_else(|error| panic!("create {relative}: {error}"));
    }
    for key in ["manifest", "lockfile", "audit", "lint", "policy"] {
        place(&root, &declaration(&source, "ecosystem.rust", key));
    }
    root
}

fn cleanup(root: &Path) {
    fs::remove_dir_all(root).unwrap_or_else(|error| panic!("cleanup: {error}"));
}

fn expect_refusal(root: &Path, source: &str, needle: &str) {
    let error = workspace_gate(root, source)
        .err()
        .unwrap_or_else(|| panic!("gate passed but expected refusal about {needle}"));
    assert!(
        error.contains(needle),
        "expected refusal mentioning {needle}, got: {error}"
    );
}

#[test]
fn committed_manifest_passes_on_the_real_repository() {
    let root = repository();
    gate::default_workspace_gate(&root)
        .unwrap_or_else(|error| panic!("default gate does not describe the repository: {error}"));
    workspace_gate(&root, &manifest()).unwrap_or_else(|error| {
        panic!("committed manifest does not describe the repository: {error}")
    });
}

#[test]
fn committed_manifest_passes_on_a_complete_tree() {
    let root = repo_fixture("complete");
    workspace_gate(&root, &manifest()).unwrap_or_else(|error| panic!("gate failed: {error}"));
    cleanup(&root);
}

#[test]
fn missing_directory_is_refused() {
    let root = repo_fixture("missing-dir");
    let path = declaration(&manifest(), "dir.emulator", "path");
    fs::remove_dir_all(root.join(&path)).unwrap_or_else(|error| panic!("remove: {error}"));
    expect_refusal(&root, &manifest(), &path);
    cleanup(&root);
}

#[test]
fn missing_policy_file_is_refused() {
    let root = repo_fixture("missing-policy");
    let path = declaration(&manifest(), "ecosystem.rust", "audit");
    fs::remove_file(root.join(&path)).unwrap_or_else(|error| panic!("remove: {error}"));
    expect_refusal(&root, &manifest(), &path);
    cleanup(&root);
}

#[test]
fn missing_lockfile_is_refused() {
    let root = repo_fixture("missing-lockfile");
    let path = declaration(&manifest(), "ecosystem.rust", "lockfile");
    fs::remove_file(root.join(&path)).unwrap_or_else(|error| panic!("remove: {error}"));
    expect_refusal(&root, &manifest(), &path);
    cleanup(&root);
}

#[test]
fn undeclared_mandated_directory_is_refused() {
    let root = repo_fixture("undeclared");
    let path = declaration(&manifest(), "dir.emulator", "path");
    let start_marker = format!("[dir.emulator]\npath = \"{path}\"\n");
    assert!(manifest().contains(&start_marker));
    let source = manifest().replace(&start_marker, "");
    expect_refusal(
        &root,
        &source,
        "mandated workspace directory undeclared: emulator",
    );
    cleanup(&root);
}

#[test]
fn undeclared_rust_ecosystem_is_refused() {
    let root = repo_fixture("no-rust");
    let source = manifest().replace("[ecosystem.rust]", "[ecosystem.zig]");
    expect_refusal(&root, &source, "rust ecosystem must be declared");
    cleanup(&root);
}

#[test]
fn unknown_section_is_refused() {
    let root = repo_fixture("unknown-section");
    let source = format!("{}\n[surprise]\nkey = \"value\"\n", manifest());
    expect_refusal(&root, &source, "unknown section surprise");
    cleanup(&root);
}

#[test]
fn unknown_declaration_is_refused() {
    let root = repo_fixture("unknown-declaration");
    let source = manifest().replace(
        "[ecosystem.rust]\nmanifest",
        "[ecosystem.rust]\nwarn_only = \"true\"\nmanifest",
    );
    expect_refusal(
        &root,
        &source,
        "unknown declaration ecosystem.rust.warn_only",
    );
    cleanup(&root);
}

#[test]
fn directory_escaping_the_workspace_root_is_refused() {
    let root = repo_fixture("escape");
    let path = declaration(&manifest(), "dir.middleware", "path");
    let declaration = format!("path = \"{path}\"");
    assert!(manifest().contains(&declaration));
    let source = manifest().replace(&declaration, "path = \"services/human/crates\"");
    expect_refusal(&root, &source, "escapes the workspace root");
    cleanup(&root);
}
