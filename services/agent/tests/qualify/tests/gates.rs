use std::process::Command;

#[test]
fn wire_gate_is_refused_as_retired() {
    for arguments in [
        vec!["wire"],
        vec!["wire", "repository", "c-reference", "harness"],
    ] {
        let output = Command::new(env!("CARGO_BIN_EXE_agent-qualification-gates"))
            .args(&arguments)
            .output()
            .expect("qualification binary must start");
        assert!(
            !output.status.success(),
            "retired wire gate must fail for {arguments:?}"
        );
        let stderr = String::from_utf8(output.stderr).expect("stderr must be UTF-8");
        assert!(
            stderr.contains("wire gate is retired"),
            "expected retirement refusal for {arguments:?}, got: {stderr}"
        );
    }
}
