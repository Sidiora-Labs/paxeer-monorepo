use std::path::Path;
use std::process::Command;

#[test]
fn generated_catalog_loads_through_the_service_reader() {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let directory =
        tempfile::tempdir().unwrap_or_else(|error| panic!("temporary directory: {error:?}"));
    let script = r#"
import sys
import unittest
sys.path.insert(0, sys.argv[1])
import test_provision
suite = unittest.defaultTestLoader.loadTestsFromNames([
    'RegistrationInputTests.test_generated_catalog_uses_real_keys_and_production_registry_modules',
    'RegistrationInputTests.test_generated_catalog_refuses_invalid_selectors',
], test_provision)
if not unittest.TextTestRunner().run(suite).wasSuccessful():
    raise SystemExit(1)
test_provision.generated_catalog(sys.argv[2], sys.argv[3])
"#;
    let output = Command::new("python3")
        .args(["-c", script])
        .arg(root.join("services/platform/hosted/human"))
        .arg(directory.path())
        .arg(env!("CARGO_BIN_EXE_layerx-human-identity-provider"))
        .env(
            "LAYERX_HUMAN_IDENTITY_PROVIDER_BIN",
            env!("CARGO_BIN_EXE_layerx-human-identity-provider"),
        )
        .output()
        .unwrap_or_else(|error| panic!("catalog generator: {error:?}"));
    assert!(
        output.status.success(),
        "{}",
        String::from_utf8_lossy(&output.stderr)
    );
    let bytes = std::fs::read(directory.path().join("purpose-catalog.json"))
        .unwrap_or_else(|error| panic!("generated catalog: {error:?}"));
    let catalog = layerx_human_service::agents::PurposePresetCatalog::from_json(&bytes)
        .unwrap_or_else(|error| panic!("catalog load: {error:?}"));
    assert_eq!(catalog.version(), "layerx-beta-v1");
    assert_eq!(
        catalog
            .call_selectors("beta-protocol")
            .unwrap_or_else(|error| panic!("catalog selectors: {error:?}")),
        vec![[0xa9, 0x05, 0x9c, 0xbb], [0x09, 0x5e, 0xa7, 0xb3]]
    );
}
