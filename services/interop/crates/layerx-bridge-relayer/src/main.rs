//! `layerx-bridge-relayer --config PATH`

use std::net::TcpListener;
use std::path::PathBuf;
use std::process::ExitCode;
use std::sync::{Arc, RwLock};
use std::time::Duration;

use layerx_bridge_relayer::attestation_api::serve;
use layerx_bridge_relayer::config::RelayerConfig;
use layerx_bridge_relayer::relayer::{Relayer, RelayerError, StepReport, OUTBOUND_STREAM};

fn config_path() -> Option<PathBuf> {
    let mut arguments = std::env::args_os().skip(1);
    let flag = arguments.next()?;
    let path = arguments.next()?;
    (flag == "--config" && arguments.next().is_none()).then(|| PathBuf::from(path))
}

fn main() -> ExitCode {
    let Some(path) = config_path() else {
        eprintln!("usage: layerx-bridge-relayer --config PATH");
        return ExitCode::from(2);
    };
    let config = match RelayerConfig::load(&path) {
        Ok(config) => config,
        Err(error) => {
            eprintln!("layerx-bridge-relayer: {error}");
            return ExitCode::from(2);
        }
    };
    let mut relayer = match config.build().and_then(Relayer::new) {
        Ok(relayer) => relayer,
        Err(error) => {
            eprintln!("layerx-bridge-relayer: startup refused: {error}");
            return ExitCode::FAILURE;
        }
    };
    let view = Arc::new(RwLock::new(relayer.attestation_snapshot()));
    if let Some(address) = config.attestation_listen {
        let listener = match TcpListener::bind(address) {
            Ok(listener) => listener,
            Err(error) => {
                eprintln!("layerx-bridge-relayer: attestation_listen {address}: {error}");
                return ExitCode::FAILURE;
            }
        };
        for result in relayer.refresh_attestation_policies() {
            if let Err(error) = result {
                eprintln!("layerx-bridge-relayer: attestation policy unavailable: {error}");
            }
        }
        if let Ok(mut shared) = view.write() {
            *shared = relayer.attestation_snapshot();
        } else {
            eprintln!("layerx-bridge-relayer: attestation view lock poisoned");
            return ExitCode::FAILURE;
        }
        let shared = Arc::clone(&view);
        let origins = config.attestation_allowed_origins.clone();
        std::thread::spawn(move || {
            if let Err(error) = serve(&listener, &shared, &origins) {
                eprintln!("layerx-bridge-relayer: attestation server: {error}");
                std::process::exit(1);
            }
        });
    }
    let interval = Duration::from_millis(config.poll_interval_ms);
    loop {
        let mut fatal = false;
        for (stream, result) in relayer.tick() {
            fatal |= matches!(&result, Err(RelayerError::Journal(_)));
            match result {
                Ok(report) if report == StepReport::default() => {}
                Ok(report) => eprintln!("layerx-bridge-relayer: {stream}: {report:?}"),
                Err(error) => eprintln!("layerx-bridge-relayer: {stream}: {error}"),
            }
        }
        if config.attestation_listen.is_some() {
            for result in relayer.refresh_attestation_policies() {
                if let Err(error) = result {
                    eprintln!("layerx-bridge-relayer: attestation policy unavailable: {error}");
                }
            }
            let snapshot = relayer.attestation_snapshot();
            if let Ok(mut shared) = view.write() {
                *shared = snapshot;
            } else {
                eprintln!("layerx-bridge-relayer: attestation view lock poisoned");
                return ExitCode::FAILURE;
            }
        }
        if !relayer.failures().is_empty() {
            eprintln!(
                "layerx-bridge-relayer: {OUTBOUND_STREAM}: failed={}",
                relayer.failures().len()
            );
        }
        for failure in relayer.failures() {
            eprintln!(
                "layerx-bridge-relayer: {OUTBOUND_STREAM}: item {} to chain {}: {}",
                failure.item, failure.chain_id, failure.error
            );
        }
        if fatal {
            return ExitCode::FAILURE;
        }
        std::thread::sleep(interval);
    }
}
