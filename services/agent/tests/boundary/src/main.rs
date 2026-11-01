mod cases;
mod gaps;
mod production;

use std::process::ExitCode;

fn run() -> Result<(), String> {
    production::refuse_retired_environment()?;
    let mut arguments = std::env::args_os();
    let _program = arguments.next();
    let mode = arguments.next();
    if arguments.next().is_some() {
        return Err("unexpected boundary-suite argument".to_owned());
    }
    match mode.as_deref().and_then(|value| value.to_str()) {
        Some("--production-required") => production::run_required(),
        Some("--production-recovery") => production::run_recovery(),
        Some("--production-duplicate") => production::run_duplicate(),
        Some("--production-peer-refusal") => production::run_peer_refusal(),
        None if mode.is_none() => {
            let report = production::run_if_configured()?
                .ok_or_else(|| "LAYERX_QUALIFY_NODE_RPC_URL is required".to_owned())?;
            println!("{report}");
            Ok(())
        }
        _ => Err("expected --production-required, --production-recovery, --production-duplicate or --production-peer-refusal".to_owned()),
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
