use std::fs;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicUsize, Ordering};

use layerx_human_boundary_check::{human_retired_surface_gate, Violation};

static NEXT: AtomicUsize = AtomicUsize::new(0);

struct Fixture(PathBuf);

impl Fixture {
    fn new() -> Self {
        let id = NEXT.fetch_add(1, Ordering::Relaxed);
        let path = std::env::temp_dir().join(format!(
            "layerx-human-retired-surface-{}-{id}",
            std::process::id()
        ));
        fs::create_dir_all(path.join("sample/src"))
            .unwrap_or_else(|error| panic!("create fixture: {error}"));
        let fixture = Self(path);
        fixture.write(
            "sample/Cargo.toml",
            "[package]\nname = \"sample\"\nversion = \"0.1.0\"\n",
        );
        fixture.write("sample/src/lib.rs", "pub struct Value(u64);\n");
        fixture
    }

    fn path(&self) -> &Path {
        &self.0
    }

    fn write(&self, relative: &str, body: &str) {
        let path = self.0.join(relative);
        if let Some(parent) = path.parent() {
            fs::create_dir_all(parent).unwrap_or_else(|error| panic!("create parent: {error}"));
        }
        fs::write(path, body).unwrap_or_else(|error| panic!("write fixture file: {error}"));
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

fn assert_rule(fixture: &Fixture, relative: &str, rule: &'static str) {
    let violations = human_retired_surface_gate(fixture.path())
        .err()
        .unwrap_or_else(|| panic!("{rule} for {relative} must be caught"));
    assert!(
        violations.contains(&Violation {
            path: fixture.path().join(relative),
            rule,
        }),
        "missing {rule} for {relative}: {violations:?}"
    );
}

#[test]
fn retired_surface_gate_flags_each_retired_rule_in_a_fixture() {
    for name in ["layerx-human-movement-provider", "layerx-paxeer-client"] {
        for manifest in [
            format!("[dependencies]\n{name} = {{ path = \"../{name}\" }}\n"),
            format!("[dependencies]\n{name}.workspace = true\n"),
            format!("[dependencies]\n\"{name}\" = \"1\"\n"),
            format!("[workspace]\nmembers = [\"crates/{name}\"]\n"),
            format!("[workspace]\nmembers = [\n    \"{name}\", # retired\n]\n"),
        ] {
            let fixture = Fixture::new();
            fixture.write("Cargo.toml", &manifest);
            assert_rule(&fixture, "Cargo.toml", "retired-crate");
        }
    }

    for import in [
        "layerx_client::lni",
        "layerx_proof::receipt::AuthorizedBatch",
        "layerx_proof::checkpoint::SettlementDomain",
        "layerx_wire::activity",
    ] {
        let fixture = Fixture::new();
        fixture.write("sample/src/lib.rs", &format!("use {import};\n"));
        assert_rule(&fixture, "sample/src/lib.rs", "retired-import");
    }

    for module in [
        "journeys/deposit.rs",
        "journeys/withdraw.rs",
        "journeys/exit.rs",
        "binding/mod.rs",
        "server/movement_provider.rs",
        "server/production_recipient.rs",
        "custody/settlement_recipient.rs",
    ] {
        let fixture = Fixture::new();
        let relative = format!("sample/src/{module}");
        fixture.write(&relative, "");
        assert_rule(&fixture, &relative, "retired-module");
    }

    for prefix in [
        "LAYERX_HUMAN_MOVEMENT_",
        "LAYERX_HUMAN_PAXEER_",
        "LAYERX_HUMAN_EXIT_",
        "LAYERX_HUMAN_RECIPIENT_",
    ] {
        let fixture = Fixture::new();
        fixture.write(
            "sample/src/lib.rs",
            &format!("const ENV: &str = \"{prefix}ENDPOINT\";\n"),
        );
        assert_rule(&fixture, "sample/src/lib.rs", "retired-env");
    }

    let fixture = Fixture::new();
    fixture.write(
        "Cargo.toml",
        "[workspace]\nmembers = [\"layerx-paxeer-client\"]\n",
    );
    fixture.write(
        "sample/src/journeys/deposit.rs",
        "use layerx_client::lni;\nconst ENV: &str = \"LAYERX_HUMAN_MOVEMENT_URL\";\n",
    );
    let violations = human_retired_surface_gate(fixture.path())
        .err()
        .unwrap_or_else(|| panic!("all retired rules must be caught together"));
    assert_eq!(violations.len(), 4, "{violations:?}");
    for rule in [
        "retired-crate",
        "retired-import",
        "retired-module",
        "retired-env",
    ] {
        assert!(violations.iter().any(|violation| violation.rule == rule));
    }
}

#[test]
fn retired_surface_gate_passes_a_clean_fixture() {
    let fixture = Fixture::new();
    assert_eq!(human_retired_surface_gate(fixture.path()), Ok(()));

    fixture.write(
        "Cargo.toml",
        "[workspace]\nmembers = [\"sample\"]\nexclude = [\"layerx-paxeer-client\"]\n\
         [workspace.dependencies]\nlayerx-paxeer-client-extra = \"1\"\n\
         # layerx-human-movement-provider = \"1\"\n",
    );
    fixture.write(
        "sample/src/other_journeys/deposit.rs",
        "pub struct Deposit;\n",
    );
    fixture.write(
        "target/old/src/journeys/deposit.rs",
        "use layerx_client::lni;\n",
    );
    assert_eq!(human_retired_surface_gate(fixture.path()), Ok(()));
}
