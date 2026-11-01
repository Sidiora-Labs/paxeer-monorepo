use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::sync::atomic::{AtomicUsize, Ordering};

static NEXT: AtomicUsize = AtomicUsize::new(0);

struct Fixture(PathBuf);

impl Fixture {
    fn new(crate_name: &str) -> Self {
        let id = NEXT.fetch_add(1, Ordering::Relaxed);
        let path =
            std::env::temp_dir().join(format!("layerx-boundary-lni-{}-{id}", std::process::id()));
        let fixture = Self(path);
        let crate_root = fixture.path().join("crates").join(crate_name);
        fs::create_dir_all(crate_root.join("src")).expect("create fixture");
        fs::write(
            fixture.path().join("unsafe-allowlist.toml"),
            "version = 1\nexceptions = []\n",
        )
        .expect("write unsafe allowlist");
        fs::write(
            fixture.path().join("nondeterminism-allowlist.toml"),
            "version = 1\nexceptions = []\n",
        )
        .expect("write nondeterminism allowlist");
        fs::write(
            crate_root.join("Cargo.toml"),
            format!("[package]\nname = \"{crate_name}\"\nversion = \"0.1.0\"\n"),
        )
        .expect("write manifest");
        fs::write(crate_root.join("src/lib.rs"), "pub struct Value(u64);\n").expect("write source");
        fixture
    }

    fn path(&self) -> &Path {
        &self.0
    }

    fn run(&self) -> Output {
        Command::new(env!("CARGO_BIN_EXE_layerx-boundary-check"))
            .arg(self.path())
            .output()
            .expect("run boundary check")
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

#[test]
fn any_lni_import_fails_the_purity_check() {
    let sources = [
        "use layerx_client::lni;\n",
        "use layerx_client::lni as retired;\n",
        "use layerx_client::lni::*;\n",
        "use layerx_client::lni::transport::Uds;\n",
        "use layerx_client::lni::transport::Tls;\n",
        "use layerx_client::lni::abi::Handle;\n",
        "use layerx_client::lni::transport;\n",
        "use layerx_client::lni::abi;\n",
        "use layerx_client::lni::activity::Envelope;\n",
    ];
    for crate_name in ["sample", "layerx-client"] {
        let fixture = Fixture::new(crate_name);
        fs::write(
            fixture.path().join("stable-abi-allowlist.toml"),
            format!("version = 1\npaths = [\n\"crates/{crate_name}/src/lib.rs\",\n]\n"),
        )
        .expect("write retired allowlist");
        let source_path = fixture
            .path()
            .join("crates")
            .join(crate_name)
            .join("src/lib.rs");
        let clean_output = fixture.run();
        assert!(
            clean_output.status.success(),
            "clean {crate_name}: {}",
            String::from_utf8_lossy(&clean_output.stderr)
        );
        for source in sources {
            fs::write(&source_path, source).expect("write LNI import");
            let output = fixture.run();
            let stderr = String::from_utf8_lossy(&output.stderr);
            assert_eq!(
                output.status.code(),
                Some(1),
                "{crate_name}: {source}: {stderr}"
            );
            assert_eq!(
                stderr.trim(),
                format!("direct-core-connection: {}", source_path.display()),
                "{crate_name}: {source}"
            );
        }
    }
}

#[test]
fn purity_check_runs_without_a_stable_abi_allowlist() {
    let fixture = Fixture::new("sample");
    assert!(!fixture.path().join("stable-abi-allowlist.toml").exists());
    fs::write(
        fixture.path().join("crates/sample/Cargo.toml"),
        "[package]\nname = \"sample\"\nversion = \"0.1.0\"\n\n[dependencies]\nlayerx-wire = { path = \"../layerx-wire\" }\n",
    )
    .expect("write permitted wire dependency");
    fs::write(
        fixture.path().join("crates/sample/src/lib.rs"),
        "use layerx_wire::budget;\npub struct Value(u64);\n",
    )
    .expect("write permitted wire import");
    let output = fixture.run();
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(output.stderr.is_empty());
    assert_eq!(
        String::from_utf8_lossy(&output.stdout).trim(),
        "agent boundary purity passed"
    );
}
