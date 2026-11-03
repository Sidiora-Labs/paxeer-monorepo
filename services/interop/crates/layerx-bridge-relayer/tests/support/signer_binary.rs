use std::path::PathBuf;
use std::process::Command;
use std::sync::OnceLock;

pub fn signer_binary() -> PathBuf {
    static BINARY: OnceLock<PathBuf> = OnceLock::new();
    BINARY.get_or_init(resolve_signer_binary).clone()
}

fn resolve_signer_binary() -> PathBuf {
    let binary = PathBuf::from(env!("CARGO_BIN_EXE_layerx-bridge-relayer")).with_file_name(
        format!("layerx-mirror-signer{}", std::env::consts::EXE_SUFFIX),
    );
    if binary.is_file() {
        return binary;
    }
    let output = Command::new(env!("CARGO"))
        .args(["build", "--locked", "--manifest-path"])
        .arg(PathBuf::from(env!("CARGO_MANIFEST_DIR")).join("../../Cargo.toml"))
        .args(["-p", "layerx-mirror-signer", "--message-format=json"])
        .output()
        .expect("build the repository's real layerx-mirror-signer prerequisite");
    assert!(
        output.status.success(),
        "layerx-mirror-signer build failed with {}: {}\n{}",
        output.status,
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    String::from_utf8(output.stdout)
        .expect("Cargo artifact output is UTF-8")
        .lines()
        .filter_map(|line| serde_json::from_str::<serde_json::Value>(line).ok())
        .find_map(|message| {
            (message["reason"] == "compiler-artifact"
                && message["target"]["name"] == "layerx-mirror-signer")
                .then(|| message["executable"].as_str().map(PathBuf::from))
                .flatten()
        })
        .filter(|path| path.is_file())
        .expect("Cargo must report the real layerx-mirror-signer executable")
}
