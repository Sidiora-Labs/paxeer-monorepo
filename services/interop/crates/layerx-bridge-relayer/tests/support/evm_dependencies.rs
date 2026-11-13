use std::path::PathBuf;
use std::process::Command;
use std::sync::OnceLock;

pub fn node_modules() -> &'static PathBuf {
    static MODULES: OnceLock<PathBuf> = OnceLock::new();
    MODULES.get_or_init(|| {
        let mut random = [0; 16];
        openssl::rand::rand_bytes(&mut random)
            .unwrap_or_else(|error| panic!("EVM dependency directory identifier: {error:?}"));
        let directory = std::env::temp_dir().join(format!(
            "layerx-bridge-evm-dependencies-{}",
            layerx_bridge_relayer::hex::encode(&random)
        ));
        std::fs::create_dir(&directory)
            .unwrap_or_else(|error| panic!("exclusive EVM dependency directory: {error:?}"));
        for (name, content) in [
            ("package.json", include_str!("evm/package.json")),
            ("package-lock.json", include_str!("evm/package-lock.json")),
        ] {
            std::fs::write(directory.join(name), content)
                .unwrap_or_else(|error| panic!("pinned EVM dependency manifest: {error:?}"));
        }
        let output = Command::new("npm")
            .current_dir(&directory)
            .args(["ci", "--ignore-scripts", "--no-audit", "--no-fund"])
            .output()
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "npm is required to provision the real EVM test dependencies"
                )
            });
        assert!(
            output.status.success(),
            "EVM dependency install failed with {}: {}\n{}",
            output.status,
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        directory.join("node_modules")
    })
}
