#[allow(dead_code)]
#[path = "../src/main.rs"]
mod gate;

use std::fs;
use std::os::unix::fs::symlink;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

static NEXT_FIXTURE: AtomicU64 = AtomicU64::new(0);

fn repository() -> PathBuf {
    PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../../..")
}

fn manifest() -> String {
    fs::read_to_string(repository().join(gate::MANIFEST_PATH))
        .unwrap_or_else(|error| panic!("read production default: {error}"))
}

struct Fixture {
    root: PathBuf,
    source: String,
}

impl Fixture {
    fn new() -> Self {
        // Validate the actual checkout before constructing or mutating a fixture.
        gate::default_workspace_gate(&repository())
            .unwrap_or_else(|error| panic!("checked-in default failed: {error}"));
        let root = std::env::temp_dir().join(format!(
            "workspace-gate-repair-22-33-1414-{}-{}",
            std::process::id(),
            NEXT_FIXTURE.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&root).unwrap_or_else(|error| panic!("create fixture: {error}"));
        let source = manifest();
        let document = layerx_platform_kvx::parse(&source)
            .unwrap_or_else(|error| panic!("parse production manifest: {error}"));
        for section in document.sections() {
            if section.starts_with("dir.") {
                let path = layerx_platform_kvx::unquote(
                    document
                        .required(section, "path")
                        .unwrap_or_else(|error| panic!("{error}")),
                )
                .unwrap_or_else(|error| panic!("{error}"));
                fs::create_dir_all(root.join(path))
                    .unwrap_or_else(|error| panic!("create declared directory: {error}"));
            } else if section.starts_with("ecosystem.") {
                for (_, value) in document.section_entries(section) {
                    let path = layerx_platform_kvx::unquote(value)
                        .unwrap_or_else(|error| panic!("{error}"));
                    let destination = root.join(&path);
                    fs::create_dir_all(destination.parent().unwrap_or_else(|| panic!("no parent")))
                        .unwrap_or_else(|error| panic!("create policy parent: {error}"));
                    fs::copy(repository().join(&path), destination)
                        .unwrap_or_else(|error| panic!("copy committed policy {path}: {error}"));
                }
            }
        }
        gate::workspace_gate(&root, &source)
            .unwrap_or_else(|error| panic!("unmodified fixture failed: {error}"));
        Self { root, source }
    }

    fn refuses(&self, source: &str, expected: &str) {
        let error = gate::workspace_gate(&self.root, source)
            .expect_err("unsafe or incomplete workspace must be rejected");
        assert!(error.contains(expected), "expected {expected}, got {error}");
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.root)
            .unwrap_or_else(|error| panic!("remove fixture {}: {error}", self.root.display()));
    }
}

fn replace_path(source: &str, old: &str, new: &str) -> String {
    let old = layerx_platform_kvx::quote(old);
    assert_eq!(source.matches(&old).count(), 1, "unique declaration {old}");
    source.replace(&old, &layerx_platform_kvx::quote(new))
}

fn write_file(path: &Path) {
    fs::create_dir_all(path.parent().unwrap_or_else(|| panic!("no parent")))
        .unwrap_or_else(|error| panic!("create parent: {error}"));
    fs::write(path, "outside workspace\n")
        .unwrap_or_else(|error| panic!("write outside policy: {error}"));
}

#[test]
fn finding_040_default_validates_checked_in_split_tree_before_mutations() {
    gate::default_workspace_gate(&repository())
        .unwrap_or_else(|error| panic!("default on checked-in tree: {error}"));
    let fixture = Fixture::new();
    let document = layerx_platform_kvx::parse(&fixture.source)
        .unwrap_or_else(|error| panic!("parse default: {error}"));
    for section in document.sections() {
        if section.starts_with("dir.") {
            let relative = layerx_platform_kvx::unquote(
                document
                    .required(section, "path")
                    .unwrap_or_else(|error| panic!("{error}")),
            )
            .unwrap_or_else(|error| panic!("{error}"));
            let path = fixture.root.join(&relative);
            let held = fixture.root.join("held-directory");
            fs::rename(&path, &held).unwrap_or_else(|error| panic!("hold directory: {error}"));
            fixture.refuses(&fixture.source, &relative);
            fs::rename(&held, &path).unwrap_or_else(|error| panic!("restore directory: {error}"));
        } else if section.starts_with("ecosystem.") {
            for (_, value) in document.section_entries(section) {
                let relative =
                    layerx_platform_kvx::unquote(value).unwrap_or_else(|error| panic!("{error}"));
                let path = fixture.root.join(&relative);
                let held = fixture.root.join("held-policy");
                fs::rename(&path, &held).unwrap_or_else(|error| panic!("hold policy: {error}"));
                fixture.refuses(&fixture.source, &relative);
                fs::rename(&held, &path).unwrap_or_else(|error| panic!("restore policy: {error}"));
            }
        }
    }
    gate::workspace_gate(&fixture.root, &fixture.source)
        .unwrap_or_else(|error| panic!("restored fixture failed: {error}"));
}

#[test]
fn finding_032_rejects_directory_traversal_and_absolute_paths() {
    let fixture = Fixture::new();
    fs::create_dir_all(fixture.root.join("services/outside"))
        .unwrap_or_else(|error| panic!("create traversal target: {error}"));
    fs::create_dir_all(fixture.root.join("services/platform-other"))
        .unwrap_or_else(|error| panic!("create prefix target: {error}"));
    let absolute = fixture.root.join("services/platform/middleware");
    for path in [
        "services/platform/../outside",
        "services/platform/../nonexistent",
        "services/platform/middleware/../middleware",
        "services/platform-other",
        absolute.to_str().unwrap_or_else(|| panic!("UTF-8 fixture")),
    ] {
        let source = replace_path(&fixture.source, "services/platform/middleware", path);
        fixture.refuses(&source, "escapes the workspace root");
    }
}

#[test]
fn finding_032_rejects_directory_symlink_escapes_before_missing_leaf() {
    let fixture = Fixture::new();
    let outside = fixture.root.join("outside");
    fs::create_dir(&outside).unwrap_or_else(|error| panic!("create outside: {error}"));
    let link = fixture.root.join("services/platform/escape");
    symlink(&outside, &link).unwrap_or_else(|error| panic!("link outside: {error}"));
    for path in [
        "services/platform/escape",
        "services/platform/escape/missing",
    ] {
        let source = replace_path(&fixture.source, "services/platform/middleware", path);
        fixture.refuses(&source, "escapes the workspace root");
    }
    fs::remove_file(&link).unwrap_or_else(|error| panic!("remove link: {error}"));
    // Another declared workspace is still outside this directory's owning root.
    symlink(fixture.root.join("tools/platform"), &link)
        .unwrap_or_else(|error| panic!("link sibling workspace: {error}"));
    let source = replace_path(
        &fixture.source,
        "services/platform/middleware",
        "services/platform/escape/cli",
    );
    fixture.refuses(&source, "escapes the workspace root");
}

#[test]
fn finding_032_contains_every_ecosystem_policy_path() {
    let fixture = Fixture::new();
    let outside = fixture.root.join("services/outside-policy");
    write_file(&outside);
    fs::create_dir(fixture.root.join("outside"))
        .unwrap_or_else(|error| panic!("create outside: {error}"));
    symlink(
        fixture.root.join("outside"),
        fixture.root.join("services/platform/policy-escape"),
    )
    .unwrap_or_else(|error| panic!("link outside directory: {error}"));
    let document = layerx_platform_kvx::parse(&fixture.source)
        .unwrap_or_else(|error| panic!("parse default: {error}"));
    for (key, value) in document.section_entries("ecosystem.rust") {
        let relative =
            layerx_platform_kvx::unquote(value).unwrap_or_else(|error| panic!("{error}"));
        for path in [
            "services/platform/../outside-policy",
            "services/platform/policy-escape/missing",
            "services/outside-policy",
            outside.to_str().unwrap_or_else(|| panic!("UTF-8 fixture")),
        ] {
            let source = replace_path(&fixture.source, &relative, path);
            fixture.refuses(&source, "escapes the workspace root");
        }
        let path = fixture.root.join(&relative);
        let held = fixture.root.join(format!("held-{key}"));
        fs::rename(&path, &held).unwrap_or_else(|error| panic!("hold policy: {error}"));
        symlink(&outside, &path).unwrap_or_else(|error| panic!("link outside file: {error}"));
        fixture.refuses(&fixture.source, "escapes the workspace root");
        fs::remove_file(&path).unwrap_or_else(|error| panic!("remove policy link: {error}"));
        fs::rename(&held, &path).unwrap_or_else(|error| panic!("restore policy: {error}"));
    }
}

#[test]
fn finding_032_rejects_workspace_root_escapes() {
    let fixture = Fixture::new();
    let absolute = fixture.root.join("services/platform");
    for root in [
        "services/platform/../platform",
        absolute.to_str().unwrap_or_else(|| panic!("UTF-8 fixture")),
    ] {
        let source = replace_path(&fixture.source, "services/platform", root);
        fixture.refuses(&source, "escapes the workspace root");
    }
    let source = replace_path(&fixture.source, "sdk/platform", "sdk/../sdk/platform");
    fixture.refuses(&source, "escapes the workspace root");
    let outside = Fixture::new();
    for root in ["services/platform", "sdk/platform", "tools/platform"] {
        let path = fixture.root.join(root);
        let held = fixture.root.join("held-root");
        fs::rename(&path, &held).unwrap_or_else(|error| panic!("hold root: {error}"));
        symlink(&outside.root, &path).unwrap_or_else(|error| panic!("link outside root: {error}"));
        fixture.refuses(&fixture.source, "escapes the repository root");
        fs::remove_file(&path).unwrap_or_else(|error| panic!("remove root link: {error}"));
        fs::rename(&held, &path).unwrap_or_else(|error| panic!("restore root: {error}"));
    }
}

#[test]
fn finding_032_accepts_normalized_paths_and_contained_symlinks() {
    let fixture = Fixture::new();
    symlink(
        fixture.root.join("services/platform/middleware"),
        fixture.root.join("services/platform/inside"),
    )
    .unwrap_or_else(|error| panic!("link contained directory: {error}"));
    let source = replace_path(
        &fixture.source,
        "services/platform/middleware",
        "./services//platform/./inside",
    );
    let source = replace_path(&source, "services/platform", "./services//platform/.");
    symlink(
        fixture.root.join("services/platform/Cargo.toml"),
        fixture.root.join("services/platform/inside-policy"),
    )
    .unwrap_or_else(|error| panic!("link contained policy: {error}"));
    let source = replace_path(
        &source,
        "services/platform/Cargo.toml",
        "services/platform/inside-policy",
    );
    let alias = fixture.root.join("repo-alias");
    symlink(&fixture.root, &alias).unwrap_or_else(|error| panic!("link repository: {error}"));
    gate::workspace_gate(&alias, &source)
        .unwrap_or_else(|error| panic!("contained paths through canonical repo failed: {error}"));
}
