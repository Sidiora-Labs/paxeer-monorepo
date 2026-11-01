use std::path::Path;

#[test]
fn native_reads_tests_name_no_admission_daemon() {
    for (path, source) in [
        (
            "native_reads_daemon.rs",
            include_str!("native_reads_daemon.rs"),
        ),
        ("native_routes_rpc.rs", include_str!("native_routes_rpc.rs")),
    ] {
        assert!(
            !source.contains("program-admission.sh"),
            "{path} must not drive the retired program admission script"
        );
    }

    let availability_daemon =
        Path::new(env!("CARGO_MANIFEST_DIR")).join("../layerx-client/tests/availability_daemon.rs");
    assert!(
        !availability_daemon
            .try_exists()
            .expect("inspect the retired client availability daemon test path"),
        "the retired client availability daemon test must remain deleted"
    );
}
