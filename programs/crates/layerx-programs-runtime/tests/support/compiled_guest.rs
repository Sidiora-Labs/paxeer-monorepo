use std::{path::Path, process::Command};

pub fn load_or_build(variable: &str, package: &str, artifact: &str) -> Vec<u8> {
    let path = if let Some(path) = std::env::var_os(variable) {
        path.into()
    } else {
        let programs = Path::new(env!("CARGO_MANIFEST_DIR")).join("../..");
        let target = programs.join("target/runtime-test-guests");
        eprintln!("{variable} is unset; compiling the real {package} guest");
        let status = Command::new(env!("CARGO"))
            .current_dir(&programs)
            .env("CARGO_TARGET_DIR", &target)
            .args([
                "build",
                "--locked",
                "--release",
                "--target",
                "wasm32-unknown-unknown",
                "-p",
                package,
            ])
            .status()
            .unwrap_or_else(|error| panic!("start {package} guest build: {error}"));
        assert!(status.success(), "{package} guest build: {status}");
        target.join("wasm32-unknown-unknown/release").join(artifact)
    };
    std::fs::read(&path)
        .unwrap_or_else(|error| panic!("read {package} guest {}: {error}", path.display()))
}
