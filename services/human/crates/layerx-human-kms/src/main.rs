fn main() -> std::process::ExitCode {
    match run() {
        Ok(()) => std::process::ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("Human KMS refused: {error}");
            std::process::ExitCode::FAILURE
        }
    }
}

fn run() -> Result<(), String> {
    if std::env::args_os().len() != 1 {
        return Err(
            "configuration is supplied through LAYERX_HUMAN_KMS_*; arguments refused".into(),
        );
    }
    let clock = layerx_client::runtime_clock::RuntimeClock::from_environment()
        .map_err(|error| error.to_string())?;
    let clock: std::sync::Arc<dyn layerx_types::clock::Clock> = clock;
    layerx_human_kms::run_from_environment(&clock)
}
