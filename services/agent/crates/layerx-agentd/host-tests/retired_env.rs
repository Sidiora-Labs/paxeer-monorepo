use std::error::Error;
use std::ffi::OsString;
use std::os::unix::ffi::OsStringExt as _;
use std::process::Command;

#[test]
fn retired_trust_variables_are_refused_by_name() -> Result<(), Box<dyn Error>> {
    let names = [
        "LAYERX_AGENT_GENESIS_TRUST",
        "LAYERX_AGENT_HANDOVER_FINALITY",
        "LAYERX_AGENT_SEQUENCER_TRUST_HISTORY",
        "LAYERX_AGENT_HUMAN_EXPORT_SETTLEMENT_DOMAIN",
        "LAYERX_AGENT_MCP_WEB_SEQUENCER_PUBLIC_KEY",
    ];
    for mode in [None, Some("full"), Some("human-owner")] {
        let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-agentd"));
        command.env_clear();
        if let Some(mode) = mode {
            command.env("LAYERX_AGENT_MODE", mode);
        }
        let output = command.output()?;
        assert_eq!(output.status.code(), Some(2));
        let stderr = String::from_utf8(output.stderr)?;
        assert!(
            stderr.contains("LAYERX_AGENT_PROGRAM_LISTEN is required"),
            "{stderr}"
        );
        for name in names {
            for value in [
                OsString::from("private-retired-configuration-value"),
                OsString::new(),
                OsString::from_vec(vec![0xff]),
            ] {
                let output = command.env(name, &value).output()?;
                command.env_remove(name);
                assert_eq!(output.status.code(), Some(2), "{name}: {mode:?}");
                let stderr = String::from_utf8(output.stderr)?;
                assert!(stderr.contains(name), "{name}: {mode:?}: {stderr}");
                assert!(stderr.contains("is retired"), "{stderr}");
                assert!(
                    !stderr.contains("private-retired-configuration-value"),
                    "{stderr}"
                );
                assert!(output.stdout.is_empty());
            }
        }
    }
    let output = Command::new(env!("CARGO_BIN_EXE_layerx-agentd"))
        .env_clear()
        .arg("--mcp-evidence")
        .env(names[0], "private-retired-configuration-value")
        .output()?;
    assert_eq!(output.status.code(), Some(2));
    let stderr = String::from_utf8(output.stderr)?;
    assert!(stderr.contains(names[0]), "{stderr}");
    assert!(stderr.contains("is retired"), "{stderr}");
    Ok(())
}
