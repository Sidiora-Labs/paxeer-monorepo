#[path = "../build.rs"]
mod build_script;
#[path = "../funded_inputs.rs"]
mod funded_inputs;
#[path = "../funding.rs"]
mod funding;
#[path = "support/registered_scenario.rs"]
mod registered_scenario;
#[path = "../required.rs"]
mod required;

use std::{fs, path::Path};

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1568_registered_target_executes_signed_scenario() {
    let manifest = include_str!("../Cargo.toml");
    assert!(manifest.contains("[[test]]\nname = \"pay2e\"\npath = \"pay2e_test.rs\""));
    registered_scenario::pay2_signed_payments_survive_restart();
}

#[test]
#[allow(non_snake_case)]
fn TestRepairM_1568_build_inputs_are_not_execution_evidence() {
    let manifest = Path::new(env!("CARGO_MANIFEST_DIR"));
    let inputs = build_script::pay2_build_inputs(manifest);
    assert_eq!(
        inputs,
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
    );
    build_script::pay2_check_build_inputs(manifest).expect("registered scenario inputs exist");
    let directory = registered_scenario::Directory::new();
    for input in &inputs {
        let destination = directory
            .0
            .join(input.strip_prefix(manifest).expect("package input"));
        fs::create_dir_all(destination.parent().expect("input parent")).expect("fixture parent");
        fs::copy(input, destination).expect("copy committed scenario inputs");
    }
    build_script::pay2_check_build_inputs(&directory.0).expect("complete input inventory");
    for input in inputs {
        let destination = directory
            .0
            .join(input.strip_prefix(manifest).expect("relative input"));
        fs::remove_file(&destination).expect("remove one input");
        assert_eq!(
            build_script::pay2_check_build_inputs(&directory.0)
                .expect_err("missing input must fail")
                .kind(),
            std::io::ErrorKind::NotFound
        );
        fs::create_dir(&destination).expect("directory at input path");
        assert_eq!(
            build_script::pay2_check_build_inputs(&directory.0)
                .expect_err("directory cannot be a source input")
                .kind(),
            std::io::ErrorKind::InvalidInput
        );
        fs::remove_dir(&destination).expect("remove directory fixture");
        fs::copy(input, destination).expect("restore input");
    }
    build_script::pay2_check_build_inputs(&directory.0).expect("restored input inventory");
}
