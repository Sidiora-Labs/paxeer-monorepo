use std::path::Path;

#[test]
fn movement_provider_directory_holds_no_rust_source() -> std::io::Result<()> {
    let provider =
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../../crates/layerx-human-movement-provider");
    for relative in [
        "src/execution.rs",
        "src/service.rs",
        "src/listener.rs",
        "src/planning.rs",
        "src/probe.rs",
        "src/producer.rs",
        "src/tests.rs",
        "tests/startup.rs",
        "tests/probe.rs",
        "tests/kms/mod.rs",
    ] {
        let source = provider.join(relative);
        assert!(
            !source.try_exists()?,
            "movement provider Rust source must be retired: {}",
            source.display()
        );
    }
    Ok(())
}
