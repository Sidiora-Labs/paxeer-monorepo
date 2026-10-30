// Kernel anchors: wiring.J51.6 (modules/lxphost/host.go:679) and
// wiring.J51.7 (include/layerx/programs.h:504).
// wiring.J51.5 binds the sweep limit to src/modules/programs/sandbox_expiry.c:14.
#![cfg(feature = "host-ffi")]

#[cfg(test)]
use layerx_programs_sandbox::MAX_SWEEP_LEASES_PER_BATCH;

#[cfg(not(test))]
unsafe extern "C" {
    fn layerx_programs_event_log_encode(
        call_token: u64,
        index: u32,
        out: *mut u8,
        out_len: u32,
    ) -> i32;
    fn layerx_programs_sandbox_admit_host(
        token: u64,
        observed_batch: u64,
        maximum_fee_hi: u64,
        maximum_fee_lo: u64,
    ) -> i32;
}

#[test]
fn host_ffi_exports_link_and_refuse_unknown_tokens() {
    assert_eq!(MAX_SWEEP_LEASES_PER_BATCH, 128);

    // Compile this file as a native probe against the actual staticlib and C
    // kernel. Cargo's Rust-only test link does not supply the host callbacks.
    let crate_dir = std::path::Path::new(env!("CARGO_MANIFEST_DIR"));
    let root = crate_dir.join("../../..");
    let target = std::path::Path::new(env!("CARGO_TARGET_TMPDIR")).join("host-ffi-link");
    std::fs::create_dir_all(&target).unwrap_or_else(|error| panic!("probe directory: {error}"));
    let kernel = target.join("kernel");
    let kernel_library = kernel.join("liblayerx.a");
    let mut build_kernel = std::process::Command::new("make");
    build_kernel
        .current_dir(&root)
        .arg("-j2")
        .arg(format!("BUILD_DIR={}", kernel.display()))
        .arg(&kernel_library);
    run_probe_command(&mut build_kernel);

    let executable = std::env::current_exe()
        .unwrap_or_else(|error| panic!("integration test executable: {error}"));
    let deps = executable
        .parent()
        .unwrap_or_else(|| panic!("integration test has no dependency directory"));
    let libraries: Vec<_> = std::fs::read_dir(deps)
        .unwrap_or_else(|error| panic!("staticlib directory: {error}"))
        .map(|entry| {
            entry
                .unwrap_or_else(|error| panic!("staticlib entry: {error}"))
                .path()
        })
        .filter(|path| {
            path.file_name()
                .and_then(|name| name.to_str())
                .is_some_and(|name| {
                    name.starts_with("liblayerx_programs_sandbox-") && name.ends_with(".a")
                })
        })
        .collect();
    assert_eq!(
        libraries.len(),
        1,
        "expected one sandbox staticlib: {libraries:?}"
    );

    let probe = target.join("host-ffi-probe");
    let mut compile_probe = std::process::Command::new("rustc");
    compile_probe
        .arg("--edition=2021")
        .args(["--cfg", "feature=\"host-ffi\""])
        .arg(crate_dir.join("tests/host_ffi_link.rs"))
        .arg("-o")
        .arg(&probe)
        .arg("-C")
        .arg(format!("link-arg={}", libraries[0].display()))
        .arg("-C")
        .arg(format!("link-arg={}", kernel_library.display()))
        .args([
            "-l", "crypto", "-l", "ssl", "-l", "pthread", "-l", "dl", "-l", "m",
        ]);
    run_probe_command(&mut compile_probe);
    run_probe_command(&mut std::process::Command::new(&probe));
}

#[cfg(test)]
fn run_probe_command(command: &mut std::process::Command) {
    let output = command
        .output()
        .unwrap_or_else(|error| panic!("{command:?}: {error}"));
    assert!(
        output.status.success(),
        "{command:?}: {}\n{}\n{}",
        output.status,
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
}

#[cfg(not(test))]
fn main() {
    for token in [0, u64::MAX] {
        let mut output = [0xa5; 192];
        // The output pointer names a writable buffer of the declared length;
        // neither token belongs to an active kernel call.
        let encoded =
            unsafe { layerx_programs_event_log_encode(token, 0, output.as_mut_ptr(), 192) };
        assert!(encoded < 0, "event encoder accepted unknown token {token}");
        assert_eq!(output, [0xa5; 192]);

        // Admission receives only scalar arguments and must refuse the token
        // before attempting to read any sandbox state.
        let admitted = unsafe { layerx_programs_sandbox_admit_host(token, 0, 0, 0) };
        assert!(admitted < 0, "sandbox admitted unknown token {token}");
    }
}
