use std::env;
use std::path::PathBuf;
use std::process::ExitCode;
use std::sync::Arc;
use std::time::Duration;

use layerx_human_service::server::{
    ComponentServerConfig, HumanComponentServer, ProductionComponents, ProductionComponentsConfig,
};

fn main() -> ExitCode {
    match run() {
        Ok(()) => ExitCode::SUCCESS,
        Err(message) => {
            eprintln!("layerx-human-components refused startup: {message}");
            ExitCode::FAILURE
        }
    }
}

fn run() -> Result<(), String> {
    let allowed_uid = required_number::<u32>("LAYERX_HUMAN_COMPONENT_ALLOWED_UID")?;
    if rustix::process::getuid().as_raw() != allowed_uid {
        return Err("the configured component UID does not own this process".to_owned());
    }
    let configuration = ComponentServerConfig {
        socket_path: PathBuf::from(required("LAYERX_HUMAN_COMPONENT_SOCKET")?),
        allowed_uid,
        worker_count: required_number("LAYERX_HUMAN_COMPONENT_WORKERS")?,
        queue_capacity: required_number("LAYERX_HUMAN_COMPONENT_QUEUE_CAPACITY")?,
        limits: layerx_human_service::server::default_component_limits(),
    };
    let maintenance_interval = Duration::from_secs(required_number(
        "LAYERX_HUMAN_MAINTENANCE_INTERVAL_SECONDS",
    )?);
    let maintenance_maximum_items = required_number("LAYERX_HUMAN_MAINTENANCE_MAXIMUM_ITEMS")?;
    if maintenance_interval.is_zero() || maintenance_maximum_items == 0 {
        return Err("the component maintenance policy is invalid".to_owned());
    }
    let clock = layerx_client::runtime_clock::RuntimeClock::from_environment()
        .map_err(|error| error.to_string())?;
    let backend = Arc::new(ProductionComponents::open(
        ProductionComponentsConfig::from_environment()?,
        clock.clone(),
    )?);
    let server = HumanComponentServer::new_maintained(
        backend,
        maintenance_interval,
        maintenance_maximum_items,
        clock,
    )
    .map_err(|_| "the component maintenance policy is invalid".to_owned())?
    .bind(configuration)
    .map_err(|_| "the privileged component listener cannot bind".to_owned())?;
    server
        .run()
        .map_err(|_| "the privileged component listener failed".to_owned())
}

fn required(name: &str) -> Result<String, String> {
    env::var(name)
        .ok()
        .filter(|value| !value.is_empty())
        .ok_or_else(|| format!("{name} is required"))
}

fn required_number<T>(name: &str) -> Result<T, String>
where
    T: std::str::FromStr,
{
    required(name)?
        .parse::<T>()
        .map_err(|_| format!("{name} is invalid"))
}
