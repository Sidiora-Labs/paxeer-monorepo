use std::io::Write;
use std::net::{Ipv4Addr, Ipv6Addr};
use std::path::Path;
use std::process::{Command, Stdio};

fn node_rpc_url(node: &Path) -> Result<&str, String> {
    let invalid = || "boundary NODE must be a valid http or https URL".to_owned();
    let url = node.to_str().ok_or_else(invalid)?;
    let (scheme, rest) = url.split_once("://").ok_or_else(invalid)?;
    if !scheme.eq_ignore_ascii_case("http") && !scheme.eq_ignore_ascii_case("https") {
        return Err(invalid());
    }
    if url.chars().any(|character| {
        character.is_whitespace()
            || character.is_control()
            || matches!(
                character,
                '\\' | '<' | '>' | '"' | '{' | '}' | '|' | '^' | '`'
            )
    }) {
        return Err(invalid());
    }
    for (index, byte) in url.bytes().enumerate() {
        if byte == b'%'
            && !url
                .as_bytes()
                .get(index + 1..index + 3)
                .is_some_and(|digits| digits.iter().all(u8::is_ascii_hexdigit))
        {
            return Err(invalid());
        }
    }
    let authority = rest.split(['/', '?', '#']).next().ok_or_else(invalid)?;
    let host_port = authority
        .rsplit_once('@')
        .map_or(authority, |(_, host)| host);
    let port = if let Some(address) = host_port.strip_prefix('[') {
        let (address, suffix) = address.split_once(']').ok_or_else(invalid)?;
        address.parse::<Ipv6Addr>().map_err(|_| invalid())?;
        if suffix.is_empty() {
            None
        } else {
            Some(suffix.strip_prefix(':').ok_or_else(invalid)?)
        }
    } else {
        let (host, port) = host_port
            .split_once(':')
            .map_or((host_port, None), |(host, port)| (host, Some(port)));
        let domain = host.strip_suffix('.').unwrap_or(host);
        if domain.is_empty()
            || domain.split('.').any(|label| {
                label.is_empty()
                    || label.starts_with('-')
                    || label.ends_with('-')
                    || !label
                        .chars()
                        .all(|character| character.is_alphanumeric() || character == '-')
            })
        {
            return Err(invalid());
        }
        if domain
            .chars()
            .all(|character| character.is_ascii_digit() || character == '.')
        {
            domain.parse::<Ipv4Addr>().map_err(|_| invalid())?;
        }
        port
    };
    if let Some(port) = port {
        if port.is_empty()
            || !port.bytes().all(|byte| byte.is_ascii_digit())
            || port.parse::<u16>().is_err()
        {
            return Err(invalid());
        }
    }
    Ok(url)
}

/// Runs and audits the boundary suite against a node JSON-RPC endpoint.
///
/// # Errors
///
/// Fails on an invalid URL, a missing harness or any RPC conformance gap.
pub fn agent_qualify_boundary_gate(
    repository: &Path,
    node: &Path,
    harness: &Path,
) -> Result<String, String> {
    let node = node_rpc_url(node)?;
    if !harness.is_file() {
        return Err("boundary qualification requires a built harness executable".to_owned());
    }
    let suite_source =
        std::fs::read_to_string(repository.join("services/agent/tests/boundary/cases.rs"))
            .map_err(|error| format!("could not audit boundary suite source: {error}"))?;
    for required in [
        "block_number()",
        "block_by_number(head)",
        "sequencer_receipt(&[0xff; 32])",
        "send_raw_transaction(&[])",
        "crate::gaps::verify_and_render(endpoint)",
    ] {
        if !suite_source.contains(required) {
            return Err(format!("boundary suite no longer covers {required}"));
        }
    }
    let mut reports = Vec::new();
    for (mode, required) in [
        (
            "--production-required",
            "RPC_CASE signed-transfer-admission-and-receipt",
        ),
        (
            "--production-recovery",
            "RPC_CASE retained-transaction-receipt-recovery",
        ),
        (
            "--production-duplicate",
            "RPC_CASE identical-raw-transaction-preserves-hash-and-receipt",
        ),
        (
            "--production-peer-refusal",
            "RPC_CASE wrong-chain-id-refusal",
        ),
    ] {
        let output = Command::new(harness)
            .env("LAYERX_QUALIFY_NODE_RPC_URL", node)
            .arg(mode)
            .output()
            .map_err(|error| format!("could not start boundary harness {mode}: {error}"))?;
        if !output.status.success() {
            return Err(format!(
                "boundary harness {mode} failed: status={} stderr={}",
                output.status,
                String::from_utf8_lossy(&output.stderr).trim()
            ));
        }
        let stdout = String::from_utf8(output.stdout)
            .map_err(|_| "boundary harness emitted non-UTF-8 output".to_owned())?;
        if !stdout.lines().any(|line| line == required) {
            return Err(format!("boundary result omitted {required}"));
        }
        check_boundary_report(harness, mode, stdout.as_bytes())?;
        reports.push(stdout);
    }
    Ok(format!(
        "agent_qualify_boundary_gate passed node_json_rpc=true modes=admission,recovery,duplicate,peer-refusal\n{}",
        reports.join("\n")
    ))
}

/// Uses the live harness's strict decoder so producer and consumer share one contract.
pub fn check_boundary_report(harness: &Path, mode: &str, report: &[u8]) -> Result<(), String> {
    if report.len() > 65_536 {
        return Err("boundary result exceeds the report bound".to_owned());
    }
    let mut decoder = Command::new(harness)
        .args(["--check-report", mode])
        .stdin(Stdio::piped())
        .stdout(Stdio::null())
        .stderr(Stdio::piped())
        .spawn()
        .map_err(|error| format!("could not start boundary report decoder {mode}: {error}"))?;
    let sent = decoder
        .stdin
        .take()
        .ok_or_else(|| "boundary report decoder has no input pipe".to_owned())?
        .write_all(report);
    let output = decoder
        .wait_with_output()
        .map_err(|error| format!("could not wait for boundary report decoder {mode}: {error}"))?;
    if !output.status.success() {
        return Err(format!(
            "boundary report decoder {mode} failed: status={} stderr={}",
            output.status,
            String::from_utf8_lossy(&output.stderr).trim()
        ));
    }
    sent.map_err(|error| format!("could not send boundary report {mode}: {error}"))
}
