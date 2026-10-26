use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};

use layerx_human_api_gen::{write_client, Generated};

static NEXT: AtomicUsize = AtomicUsize::new(0);

fn copy_tree(from: &Path, to: &Path) {
    fs::create_dir_all(to).unwrap_or_else(|error| panic!("create {}: {error}", to.display()));
    let entries =
        fs::read_dir(from).unwrap_or_else(|error| panic!("read {}: {error}", from.display()));
    for entry in entries {
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
    fn from_real_schema() -> Self {
        let id = NEXT.fetch_add(1, Ordering::Relaxed);
        let root = std::env::temp_dir().join(format!(
            "layerx-human-api-gen-retired-{}-{id}",
            std::process::id()
        ));
        fs::create_dir(&root).unwrap_or_else(|error| panic!("create {}: {error}", root.display()));
        let source = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../schema/human-api");
        copy_tree(&source, &root.join("schema"));
        Self(root)
    }

    fn rewrite(&self, relative: &str, from: &str, to: &str) {
        let path = self.0.join("schema").join(relative);
        let body = fs::read_to_string(&path)
            .unwrap_or_else(|error| panic!("read {}: {error}", path.display()));
        assert_eq!(body.matches(from).count(), 1, "rewrite target: {from}");
        fs::write(&path, body.replacen(from, to, 1))
            .unwrap_or_else(|error| panic!("write {}: {error}", path.display()));
    }

    fn generate(&self) -> Generated {
        let generated = write_client(&self.0.join("schema"), &self.0.join("out"))
            .unwrap_or_else(|violations| panic!("expected generation: {violations:#?}"));
        for (name, body) in &generated.files {
            let path = self.0.join("out").join(name);
            let written = fs::read_to_string(&path)
                .unwrap_or_else(|error| panic!("read {}: {error}", path.display()));
            assert_eq!(&written, body);
        }
        generated
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn retired_operation_has_no_generated_client_method() {
    let fixture = Fixture::from_real_schema();
    let baseline = fixture.generate();
    assert_eq!(baseline.retired(), 0);
    let original = &baseline.files["index.ts"];
    assert!(original.contains("  depositStart("));
    assert!(original.contains("\"/v1/deposits\""));
    assert!(baseline.files["conformance.ts"].contains("run.client.depositStart("));

    fixture.rewrite(
        "movement.kvx",
        "[operation.deposit.start]",
        "[operation.deposit.start]\nretired = \"true\"",
    );
    let generated = fixture.generate();
    assert_eq!(generated.retired(), 1);
    assert_eq!(generated.retired, 1);
    assert_eq!(
        generated.operations + generated.retired(),
        baseline.operations
    );
    assert_eq!(generated.types, baseline.types);
    for body in generated.files.values() {
        assert!(!body.contains("depositStart"));
        assert!(!body.contains("\"deposit.start\""));
        assert!(!body.contains("\"/v1/deposits\""));
    }
    let index = &generated.files["index.ts"];
    assert!(index.contains("  depositConfirm("));
    assert!(index.contains("\"/v1/deposits/{journey_id}/confirm\""));
    assert!(generated.files["conformance.ts"].contains("run.client.depositConfirm("));
}

#[test]
fn retired_variant_stays_in_the_generated_union() {
    let fixture = Fixture::from_real_schema();
    let baseline = fixture.generate();
    fixture.rewrite(
        "journeys.kvx",
        "[type.JourneyKind]",
        "[type.JourneyKind]\nretired_variants = [\"deposit\"]",
    );
    let generated = fixture.generate();
    let index = &generated.files["index.ts"];
    let union = index
        .lines()
        .find_map(|line| line.strip_prefix("export type JourneyKind = "))
        .unwrap_or_else(|| panic!("JourneyKind union is missing"));
    assert!(union
        .trim_end_matches(';')
        .split(" | ")
        .any(|variant| variant == "\"deposit\""));
    let variants = index
        .lines()
        .find(|line| {
            line.starts_with("export const journeyKindVariants: readonly JourneyKind[] = [")
        })
        .unwrap_or_else(|| panic!("JourneyKind decoder variants are missing"));
    assert!(variants.contains("\"deposit\""));
    assert!(index.contains("for (const variant of journeyKindVariants)"));
    assert_eq!(generated.files, baseline.files);
    assert_eq!(generated.operations, baseline.operations);
    assert_eq!(generated.types, baseline.types);
    assert_eq!(generated.retired(), 0);
}
