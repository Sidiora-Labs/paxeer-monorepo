use std::fs;
use std::path::{Path, PathBuf};
use std::process::{Command, Output};
use std::time::{SystemTime, UNIX_EPOCH};

fn repository_root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../..")
        .canonicalize()
        .unwrap_or_else(|error| panic!("locate repository: {error}"))
}

fn run_policy(root: &Path) -> Output {
    Command::new("sh")
        .arg(root.join("services/human/tools/dependency-policy.sh"))
        .current_dir(root)
        .output()
        .unwrap_or_else(|error| panic!("run real dependency policy: {error}"))
}

fn banned(output: &Output) -> bool {
    String::from_utf8_lossy(&output.stderr)
        .contains("human dependency policy: forbidden boundary or global-initialisation crate")
}

struct Workspace(PathBuf);

impl Workspace {
    fn new() -> Self {
        let nonce = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .unwrap_or_else(|error| panic!("read fixture creation time: {error}"))
            .as_nanos();
        let root = std::env::temp_dir().join(format!(
            "human-dependency-policy-{}-{nonce}",
            std::process::id()
        ));
        fs::create_dir(&root).unwrap_or_else(|error| panic!("create workspace: {error}"));
        let workspace = Self(root);
        workspace.write(
            "services/human/Cargo.toml",
            "[workspace]\nresolver = \"2\"\nmembers = [\"crates/policy-probe\", \"crates/layerx-paxeer-client\"]\n",
        );
        workspace.write(
            "services/human/crates/policy-probe/Cargo.toml",
            "[package]\nname = \"policy-probe\"\nversion = \"0.1.0\"\nedition = \"2021\"\n\n[dependencies]\nlayerx-paxeer-client = { path = \"../layerx-paxeer-client\" }\n",
        );
        workspace.write("services/human/crates/policy-probe/src/lib.rs", "");
        workspace.write(
            "services/human/crates/layerx-paxeer-client/Cargo.toml",
            "[package]\nname = \"layerx-paxeer-client\"\nversion = \"0.1.0\"\nedition = \"2021\"\n",
        );
        workspace.write("services/human/crates/layerx-paxeer-client/src/lib.rs", "");
        fs::create_dir_all(workspace.0.join("services/human/tools"))
            .unwrap_or_else(|error| panic!("create tools directory: {error}"));
        fs::copy(
            repository_root().join("services/human/tools/dependency-policy.sh"),
            workspace
                .0
                .join("services/human/tools/dependency-policy.sh"),
        )
        .unwrap_or_else(|error| panic!("copy real policy script: {error}"));

        let output = Command::new("cargo")
            .args(["generate-lockfile", "--offline", "--manifest-path"])
            .arg(workspace.0.join("services/human/Cargo.toml"))
            .current_dir(&workspace.0)
            .output()
            .unwrap_or_else(|error| panic!("generate workspace lockfile: {error}"));
        assert!(
            output.status.success(),
            "generate real lockfile: {output:?}"
        );
        workspace
    }

    fn write(&self, relative: &str, body: &str) {
        let path = self.0.join(relative);
        let parent = path
            .parent()
            .unwrap_or_else(|| panic!("fixture path must have a parent"));
        fs::create_dir_all(parent)
            .unwrap_or_else(|error| panic!("create fixture directory: {error}"));
        fs::write(path, body).unwrap_or_else(|error| panic!("write fixture: {error}"));
    }
}

impl Drop for Workspace {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0)
            .unwrap_or_else(|error| panic!("remove temporary workspace: {error}"));
    }
}

#[test]
fn dependency_policy_script_passes_on_the_merged_workspace() {
    let output = run_policy(&repository_root());
    assert!(
        output.status.success(),
        "merged human workspace must pass the dependency policy: {output:?}"
    );
    assert!(
        String::from_utf8_lossy(&output.stdout)
            .contains("human dependency and unsafe policies passed"),
        "policy must finish all checks: {output:?}"
    );
}

#[test]
fn dependency_policy_script_bans_the_paxeer_client_crate() {
    let workspace = Workspace::new();
    let output = run_policy(&workspace.0);
    assert_eq!(
        output.status.code(),
        Some(1),
        "workspace containing layerx-paxeer-client must be rejected: {output:?}"
    );
    assert!(
        banned(&output),
        "policy must reject the banned package rather than fail metadata or license checks: {output:?}"
    );
}
