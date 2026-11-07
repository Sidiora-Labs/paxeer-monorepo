use paxeer_native::{BootstrapRequest, Result};
use std::path::Path;
use std::process::ExitCode;

fn run(arguments: &[String]) -> Result<()> {
    if let [action, flag, script, rest @ ..] = arguments {
        if action == "bootstrap" && flag == "--bootstrap-script" {
            if !Path::new(script).is_absolute() || !Path::new(script).is_file() {
                return Err("maintained native bootstrap script is required".into());
            }
            let request = BootstrapRequest::parse(rest)?;
            let mut guard = request.open()?;
            if guard.initialized() {
                return Ok(());
            }
            let status = request.command(Path::new(script)).status()?;
            if !status.success() {
                return Err(format!("native bootstrap failed: {status}").into());
            }
            // Refuse concurrent source changes rather than sealing a different request.
            let current =
                paxeer_native::ImmutableInputs::capture(&request.source, &request.options)?;
            guard.complete(&current)?;
            return Ok(());
        }
    }
    paxeer_native::run_read_command(arguments, &mut std::io::stdout().lock())
}

fn main() -> ExitCode {
    match run(&std::env::args().skip(1).collect::<Vec<_>>()) {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("native provisioning refused: {error}");
            ExitCode::FAILURE
        }
    }
}
