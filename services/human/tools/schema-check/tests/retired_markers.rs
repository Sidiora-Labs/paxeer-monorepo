use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};

use layerx_human_schema_check::{
    baseline_manifest, human_api_schema, SchemaReport, Violation, BASELINE_FILE,
};

static NEXT: AtomicUsize = AtomicUsize::new(0);

fn copy_tree(from: &Path, to: &Path) {
    fs::create_dir_all(to).unwrap_or_else(|error| panic!("create {}: {error}", to.display()));
    for entry in
        fs::read_dir(from).unwrap_or_else(|error| panic!("read {}: {error}", from.display()))
    {
        let entry = entry.unwrap_or_else(|error| panic!("read entry: {error}"));
        let source = entry.path();
        let target = to.join(entry.file_name());
        if source.is_dir() {
            copy_tree(&source, &target);
        } else {
            fs::copy(&source, &target)
                .unwrap_or_else(|error| panic!("copy {}: {error}", source.display()));
        }
    }
}

struct Fixture(PathBuf);

impl Fixture {
    fn new() -> Self {
        let id = NEXT.fetch_add(1, Ordering::Relaxed);
        let path = std::env::temp_dir().join(format!(
            "layerx-human-retired-markers-{}-{id}",
            std::process::id()
        ));
        fs::create_dir(&path).unwrap_or_else(|error| panic!("create {}: {error}", path.display()));
        copy_tree(
            &Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/human-api"),
            &path,
        );
        Self(path)
    }

    fn read(&self, relative: &str) -> String {
        fs::read_to_string(self.0.join(relative))
            .unwrap_or_else(|error| panic!("read {relative}: {error}"))
    }

    fn write(&self, relative: &str, body: &str) {
        fs::write(self.0.join(relative), body)
            .unwrap_or_else(|error| panic!("write {relative}: {error}"));
    }

    fn rewrite(&self, relative: &str, from: &str, to: &str) {
        let body = self.read(relative);
        assert_eq!(
            body.matches(from).count(),
            1,
            "rewrite target in {relative}"
        );
        self.write(relative, &body.replacen(from, to, 1));
    }

    fn report(&self) -> SchemaReport {
        human_api_schema(&self.0)
            .unwrap_or_else(|violations| panic!("expected pass: {violations:#?}"))
    }

    fn violations(&self) -> Vec<Violation> {
        match human_api_schema(&self.0) {
            Ok(report) => panic!("expected violations, got {report:?}"),
            Err(violations) => violations,
        }
    }

    fn freeze_baseline(&self) {
        let manifest = baseline_manifest(&self.0)
            .unwrap_or_else(|violations| panic!("baseline manifest: {violations:#?}"));
        self.write(BASELINE_FILE, &manifest);
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn operation_marked_retired_is_accepted_and_reported() {
    let fixture = Fixture::new();
    let original = fixture.report();
    assert!(original.retired_operations.is_empty());
    fixture.rewrite(
        "movement.kvx",
        "[operation.deposit.start]\n",
        "[operation.deposit.start]\nretired = \"true\"\n",
    );
    let report = fixture.report();
    assert_eq!(report.retired_operations, vec!["deposit.start"]);
    assert_eq!(report.operations, original.operations);
    assert_eq!(report.golden_vectors, original.golden_vectors);
    assert_eq!(report.declarations, original.declarations + 1);
    assert_eq!(
        report.additions_beyond_baseline,
        original.additions_beyond_baseline + 1
    );

    for invalid in ["\"false\"", "true", "false", "\"TRUE\"", "\"\"", "[]"] {
        let marker = format!("retired = {invalid}\n");
        fixture.rewrite("movement.kvx", "retired = \"true\"\n", &marker);
        assert!(
            fixture.violations().iter().any(|entry| {
                entry.rule == "invalid-retired-flag"
                    && entry.path == fixture.0.join("movement.kvx")
                    && entry.detail.contains("operation.deposit.start")
            }),
            "invalid retired flag accepted: {invalid}"
        );
        fixture.rewrite("movement.kvx", &marker, "retired = \"true\"\n");
    }

    fs::remove_file(fixture.0.join("golden/deposit.start.response.json"))
        .unwrap_or_else(|error| panic!("remove golden: {error}"));
    assert!(fixture.violations().iter().any(|entry| {
        entry.rule == "missing-golden-vector"
            && entry.path == fixture.0.join("golden/deposit.start.response.json")
    }));
}

#[test]
fn retired_variant_outside_variants_is_rejected() {
    let fixture = Fixture::new();
    let original = fixture.report();
    assert!(original.retired_variants.is_empty());
    fixture.rewrite(
        "errors.kvx",
        "[type.ErrorCode]\n",
        "[type.ErrorCode]\nretired_variants = [\"never-declared\"]\n",
    );
    assert!(fixture.violations().iter().any(|entry| {
        entry.rule == "retired-variant-not-declared"
            && entry.path == fixture.0.join("errors.kvx")
            && entry.detail.contains("ErrorCode")
            && entry.detail.contains("never-declared")
    }));

    fixture.rewrite(
        "errors.kvx",
        "retired_variants = [\"never-declared\"]\n",
        "retired_variants = [\"unauthenticated\"]\n",
    );
    let report = fixture.report();
    assert_eq!(
        report.retired_variants.get("ErrorCode"),
        Some(&vec!["unauthenticated".to_owned()])
    );
    assert_eq!(report.golden_vectors, original.golden_vectors);
    assert_eq!(
        report.additions_beyond_baseline,
        original.additions_beyond_baseline + 1
    );

    for invalid in ["\"unauthenticated\"", "true", "[true]"] {
        let marker = format!("retired_variants = {invalid}\n");
        fixture.rewrite(
            "errors.kvx",
            "retired_variants = [\"unauthenticated\"]\n",
            &marker,
        );
        assert!(fixture
            .violations()
            .iter()
            .any(|entry| entry.rule == "invalid-retired-variants"));
        fixture.rewrite(
            "errors.kvx",
            &marker,
            "retired_variants = [\"unauthenticated\"]\n",
        );
    }

    fixture.freeze_baseline();
    fixture.rewrite(
        "errors.kvx",
        "variants = [\"unauthenticated\",",
        "variants = [",
    );
    assert!(fixture.violations().iter().any(|entry| {
        entry.rule == "breaking-declaration-change"
            && entry.path == fixture.0.join(BASELINE_FILE)
            && entry
                .detail
                .starts_with("errors.kvx:type.ErrorCode.variants:")
    }));
}

#[test]
fn removing_a_retired_operation_is_still_a_baseline_break() {
    let fixture = Fixture::new();
    fixture.rewrite(
        "movement.kvx",
        "[operation.deposit.start]\n",
        "[operation.deposit.start]\nretired = \"true\"\n",
    );
    assert_eq!(fixture.report().retired_operations, vec!["deposit.start"]);
    fixture.freeze_baseline();
    assert_eq!(fixture.report().additions_beyond_baseline, 0);

    let body = fixture.read("movement.kvx");
    let start = body
        .find("[operation.deposit.start]\n")
        .unwrap_or_else(|| panic!("missing deposit.start"));
    let end = start
        + body[start..]
            .find("\n[operation.deposit.confirm]")
            .unwrap_or_else(|| panic!("missing deposit.confirm"));
    fixture.write(
        "movement.kvx",
        &format!("{}{}", &body[..start], &body[end..]),
    );
    let violations = fixture.violations();
    for key in ["method", "path", "request", "response", "retired"] {
        let declaration = format!("movement.kvx:operation.deposit.start.{key} was removed");
        assert!(
            violations.iter().any(|entry| {
                entry.rule == "breaking-declaration-removal"
                    && entry.path == fixture.0.join(BASELINE_FILE)
                    && entry.detail.starts_with(&declaration)
            }),
            "missing baseline removal for {key}: {violations:#?}"
        );
    }
}
