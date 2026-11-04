use std::{
    env, fs, io,
    path::{Path, PathBuf},
};

pub(crate) fn pay2_build_inputs(manifest: &Path) -> [PathBuf; 7] {
    [
        "Cargo.toml",
        "required.rs",
        "pay2e_test.rs",
        "funded_inputs.rs",
        "funding.rs",
        "sequence.rs",
        "tests/support/registered_scenario.rs",
    ]
    .map(|input| manifest.join(input))
}

pub(crate) fn pay2_check_build_inputs(manifest: &Path) -> io::Result<()> {
    for input in pay2_build_inputs(manifest) {
        if !fs::metadata(&input)?.is_file() {
            return Err(io::Error::new(
                io::ErrorKind::InvalidInput,
                format!("PAY2 build input is not a file: {}", input.display()),
            ));
        }
    }
    Ok(())
}

fn main() {
    let manifest = PathBuf::from(env::var_os("CARGO_MANIFEST_DIR").expect("manifest"));
    for source in pay2_build_inputs(&manifest) {
        println!("cargo:rerun-if-changed={}", source.display());
    }
    pay2_check_build_inputs(&manifest).expect("PAY2 registered scenario inputs must exist");
}
