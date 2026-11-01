use std::path::{Path, PathBuf};
use std::process::Command;

fn repository() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .expect("qualification crate must be inside the repository")
        .to_owned()
}

#[test]
fn boundary_gate_requires_a_node_rpc_url() {
    let repository = repository();
    let missing_harness =
        repository.join("services/agent/tests/qualify/tests/boundary_gate.rs.missing");
    assert!(!missing_harness.exists());
    for node in [
        "",
        env!("CARGO_BIN_EXE_agent-qualification-gates"),
        "/usr/bin/true",
        "./layerxd",
        "node.example:8545",
        "file:///tmp/layerxd",
        "unix:///run/layerxd.sock",
        "ws://localhost:8545",
        "ftp://node.example",
        "http://",
        "https:///rpc",
        "http://:8545",
        "https://node example/rpc",
        "http://localhost:invalid",
        "http://localhost:65536",
        "http://[::1/rpc",
        "http://[invalid]/rpc",
        "http://256.0.0.1/rpc",
        "https://node.example/%zz",
        "https://node.example\\rpc",
    ] {
        let output = Command::new(env!("CARGO_BIN_EXE_agent-qualification-gates"))
            .arg("boundary")
            .arg(&repository)
            .arg(node)
            .arg(&missing_harness)
            .output()
            .expect("run the actual qualification CLI");
        assert!(!output.status.success(), "accepted invalid NODE {node:?}");
        let stderr = String::from_utf8(output.stderr).expect("UTF-8 diagnostics");
        assert!(
            stderr.contains("boundary NODE must be a valid http or https URL"),
            "invalid NODE {node:?} was not rejected as a URL: {stderr}"
        );
    }
    for node in [
        "http://localhost:8545",
        "http://127.0.0.1:8545/rpc",
        "http://[::1]:8545/rpc",
        "http://sequencer.railway.internal:8545",
        "http://node.example/rpc",
        "https://node.example/rpc?network=1",
        "HTTPS://node.example/rpc",
    ] {
        let output = Command::new(env!("CARGO_BIN_EXE_agent-qualification-gates"))
            .arg("boundary")
            .arg(&repository)
            .arg(node)
            .arg(&missing_harness)
            .output()
            .expect("run the actual qualification CLI");
        assert!(!output.status.success(), "missing harness must fail");
        let stderr = String::from_utf8(output.stderr).expect("UTF-8 diagnostics");
        assert!(
            stderr.contains("boundary qualification requires a built harness executable"),
            "valid NODE {node:?} did not reach harness validation: {stderr}"
        );
    }
    #[cfg(unix)]
    {
        use std::ffi::OsStr;
        use std::os::unix::ffi::OsStrExt;
        let output = Command::new(env!("CARGO_BIN_EXE_agent-qualification-gates"))
            .arg("boundary")
            .arg(&repository)
            .arg(OsStr::from_bytes(b"https://node.example/\xff"))
            .arg(&missing_harness)
            .output()
            .expect("run the actual qualification CLI");
        assert!(!output.status.success());
        assert!(String::from_utf8(output.stderr)
            .expect("UTF-8 diagnostics")
            .contains("boundary NODE must be a valid http or https URL"));
    }
}

#[test]
fn boundary_harness_tree_has_no_lni_node() {
    let boundary_tree = repository().join("services/agent/tests/boundary");
    for name in [
        "layerxd_lni.c",
        "preparation_state.c",
        "production-lni-cases.kvx",
        "production-lni.env.example",
    ] {
        let result = std::fs::symlink_metadata(boundary_tree.join("node").join(name));
        assert!(
            matches!(result, Err(ref error) if error.kind() == std::io::ErrorKind::NotFound),
            "retired LNI node artifact is not absent: {name}: {result:?}"
        );
    }
    for entry in std::fs::read_dir(boundary_tree).expect("read boundary harness tree") {
        let entry = entry.expect("read boundary harness entry");
        if entry.file_name() != "node" {
            continue;
        }
        for entry in std::fs::read_dir(entry.path()).expect("read boundary node tree") {
            let entry = entry.expect("read boundary node entry");
            let name = entry.file_name();
            let name = name.to_str().expect("UTF-8 boundary node filename");
            assert!(
                name != "layerxd_lni.c"
                    && name != "preparation_state.c"
                    && !name.starts_with("production-lni"),
                "retired LNI node artifact remains: {name}"
            );
        }
    }
}
