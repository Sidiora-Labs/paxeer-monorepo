use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};

use layerx_human_schema_check::{baseline_manifest, human_api_schema, Violation, BASELINE_FILE};

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
            "layerx-human-compatibility-rows-{}-{id}",
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

    fn set_matrix(&self, body: &str) {
        self.write("compatibility.kvx", body);
        let manifest = baseline_manifest(&self.0)
            .unwrap_or_else(|violations| panic!("baseline manifest: {violations:#?}"));
        self.write(BASELINE_FILE, &manifest);
    }

    fn assert_passes(&self) {
        human_api_schema(&self.0)
            .unwrap_or_else(|violations| panic!("expected pass: {violations:#?}"));
    }

    fn violations(&self) -> Vec<Violation> {
        human_api_schema(&self.0).unwrap_err()
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn missing_compatibility_row_for_a_minor_is_rejected() {
    let fixture = Fixture::new();
    fixture.assert_passes();
    assert!(fixture
        .read("v1.kvx")
        .lines()
        .any(|line| line == "minor = 9"));
    let matrix = fixture.read("compatibility.kvx");
    let (through_seven, _) = matrix
        .split_once("[compatibility.row.9]")
        .unwrap_or_else(|| panic!("missing row for schema 1.8"));
    fixture.set_matrix(through_seven);
    let violations = fixture.violations();
    let missing: Vec<_> = violations
        .iter()
        .filter(|entry| entry.rule == "compatibility-row-missing")
        .collect();
    assert_eq!(missing.len(), 1, "{violations:#?}");
    assert_eq!(missing[0].path, fixture.0.join("compatibility.kvx"));
    assert_eq!(
        missing[0].detail,
        "no matrix row records schema version 1.8"
    );
    assert!(violations
        .iter()
        .all(|entry| entry.rule != "compatibility-row-order"));

    let (_, last_row) = matrix
        .split_once("[compatibility.row.10]")
        .unwrap_or_else(|| panic!("missing row for schema 1.9"));
    fixture.set_matrix(&format!("{through_seven}[compatibility.row.10]{last_row}"));
    let violations = fixture.violations();
    assert_eq!(violations.len(), 1, "{violations:#?}");
    assert_eq!(violations[0].rule, "compatibility-row-missing");
    assert_eq!(
        violations[0].detail,
        "no matrix row records schema version 1.8"
    );

    let (header, _) = matrix
        .split_once("[compatibility.row.1]")
        .unwrap_or_else(|| panic!("missing first row"));
    fixture.set_matrix(header);
    assert!(fixture.violations().iter().any(|entry| {
        entry.rule == "compatibility-row-missing"
            && entry.detail == "no matrix row records schema version 1.0"
    }));

    fixture.set_matrix(&matrix);
    fixture.assert_passes();
}

#[test]
fn out_of_order_compatibility_rows_are_rejected() {
    let fixture = Fixture::new();
    fixture.assert_passes();
    let matrix = fixture.read("compatibility.kvx");
    let (header, rest) = matrix
        .split_once("[compatibility.row.1]")
        .unwrap_or_else(|| panic!("missing first row"));
    let (first, rest) = rest
        .split_once("[compatibility.row.2]")
        .unwrap_or_else(|| panic!("missing second row"));
    let (second, remaining) = rest
        .split_once("[compatibility.row.3]")
        .unwrap_or_else(|| panic!("missing third row"));
    fixture.set_matrix(&format!(
        "{header}[compatibility.row.2]{second}[compatibility.row.1]{first}[compatibility.row.3]{remaining}"
    ));
    let violations = fixture.violations();
    assert_eq!(violations.len(), 1, "{violations:#?}");
    assert_eq!(violations[0].rule, "compatibility-row-order");
    assert_eq!(violations[0].path, fixture.0.join("compatibility.kvx"));
    assert!(violations[0].detail.contains("compatibility.row.1"));

    for invalid in ["1.0", "2.1", "1.01", "1.-1", "1.1.0", "invalid"] {
        fixture.set_matrix(&matrix.replacen(
            "schema = \"1.1\"",
            &format!("schema = \"{invalid}\""),
            1,
        ));
        let violations = fixture.violations();
        assert!(
            violations.iter().any(|entry| {
                entry.rule == "compatibility-row-order"
                    && entry.path == fixture.0.join("compatibility.kvx")
                    && entry.detail.contains("compatibility.row.2")
            }),
            "invalid version accepted: {invalid}: {violations:#?}"
        );
    }

    fixture.set_matrix(&matrix);
    fixture.assert_passes();
}
