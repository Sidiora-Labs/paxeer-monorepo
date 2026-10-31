#[path = "../../../../services/platform/hosted/gateway/tests/local/required.rs"]
mod required;

use required::Required;
use std::{
    env, fs,
    path::{Path, PathBuf},
};

pub(crate) fn pay2_build_inputs(manifest: &Path) -> [PathBuf; 3] {
    let repository = manifest
        .join("../../../..")
        .canonicalize()
        .required("repository");
    let local = repository.join("services/platform/hosted/gateway/tests/local");
    let pay2_source = manifest.join("pay2e_test.rs");
    [
        local.join("required.rs"),
        local.join("verifier.rs"),
        pay2_source,
    ]
}

fn main() {
    let manifest = PathBuf::from(env::var_os("CARGO_MANIFEST_DIR").required("manifest"));
    for source in pay2_build_inputs(&manifest) {
        println!("cargo:rerun-if-changed={}", source.display());
        fs::read_to_string(source).required("PAY2 qualification build input");
    }
}
