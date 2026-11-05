mod cases;
mod gaps;
mod production;

use std::io::{self, Read};
use std::path::Path;
use std::process::ExitCode;

const REPOSITORY: &str = "REPOSITORY";

fn check_report(mode: &str) -> Result<(), String> {
    let mut report = String::new();
    io::stdin()
        .take(65_537)
        .read_to_string(&mut report)
        .map_err(|error| format!("boundary result is not UTF-8: {error}"))?;
    if report.len() > 65_536 {
        return Err("boundary result exceeds the report bound".to_owned());
    }
    let lines: Vec<_> = report.lines().collect();
    let marker = match mode {
        "default" | "--production-required" => {
            if lines.len() != 3 {
                return Err(
                    "boundary result must contain the suite, pending reads and admission"
                        .to_owned(),
                );
            }
            let height = lines[0]
                .strip_prefix("agent boundary conformance suite passed: node JSON-RPC endpoint, committed block ")
                .and_then(|line| line.strip_suffix(", absent receipt, malformed submit"));
            let chain_id = lines[1]
                .strip_prefix("chain_id=")
                .and_then(|line| line.strip_suffix(" pending reads exercised: eth_getBalance eth_getTransactionCount eth_estimateGas eth_call"));
            for quantity in [height, chain_id] {
                let valid = quantity.is_some_and(|text| {
                    text.parse::<u64>()
                        .is_ok_and(|value| value.to_string() == text)
                });
                if !valid {
                    return Err(
                        "boundary result has a malformed suite or pending-read result".to_owned(),
                    );
                }
            }
            "RPC_CASE signed-transfer-admission-and-receipt"
        }
        "--production-recovery" => "RPC_CASE retained-transaction-receipt-recovery",
        "--production-duplicate" => "RPC_CASE identical-raw-transaction-preserves-hash-and-receipt",
        "--production-peer-refusal" => "RPC_CASE wrong-chain-id-refusal",
        _ => return Err("unknown boundary result mode".to_owned()),
    };
    let expected_lines = if matches!(mode, "default" | "--production-required") {
        3
    } else {
        1
    };
    if lines.len() != expected_lines || lines.last().copied() != Some(marker) {
        return Err(format!("boundary result omitted or malformed {marker}"));
    }
    Ok(())
}

fn run_default() -> Result<(), String> {
    let report = production::run_if_configured()?
        .ok_or_else(|| "LAYERX_QUALIFY_NODE_RPC_URL is required".to_owned())?;
    println!("{report}");
    Ok(())
}

fn run() -> Result<(), String> {
    production::refuse_retired_environment()?;
    let arguments: Vec<_> = std::env::args_os().skip(1).collect();
    if arguments
        .first()
        .is_some_and(|argument| argument == "--check-report")
    {
        let mode = arguments
            .get(1)
            .and_then(|value| value.to_str())
            .ok_or_else(|| "--check-report requires a result mode".to_owned())?;
        if arguments.len() != 2 {
            return Err("unexpected boundary-result argument".to_owned());
        }
        return check_report(mode);
    }
    if arguments.len() == 2 && !arguments[0].to_string_lossy().starts_with("--") {
        let node = arguments[0]
            .to_str()
            .ok_or_else(|| "NODE must be a UTF-8 node RPC URL".to_owned())?;
        let repository = Path::new(&arguments[1]);
        if !repository
            .join("services/agent/tests/boundary/Cargo.toml")
            .is_file()
        {
            return Err(format!(
                "{REPOSITORY} must contain the agent boundary crate"
            ));
        }
        std::env::set_var("LAYERX_QUALIFY_NODE_RPC_URL", node);
        return run_default();
    }
    if arguments.len() > 1 {
        return Err("unexpected boundary-suite argument".to_owned());
    }
    match arguments.first().and_then(|value| value.to_str()) {
        Some("--production-required") => production::run_required(),
        Some("--production-recovery") => production::run_recovery(),
        Some("--production-duplicate") => production::run_duplicate(),
        Some("--production-peer-refusal") => production::run_peer_refusal(),
        None if arguments.is_empty() => run_default(),
        _ => Err("expected NODE REPOSITORY, --production-required, --production-recovery, --production-duplicate or --production-peer-refusal".to_owned()),
    }
}

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("node RPC boundary conformance refused: {error}");
            ExitCode::FAILURE
        }
    }
}
