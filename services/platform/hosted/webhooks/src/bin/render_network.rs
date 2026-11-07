use layerx_platform_webhooks::deployment::bind_network;
use std::{
    env, fs,
    io::{self, Write},
    process::ExitCode,
};

fn render() -> Result<(), String> {
    let args: Vec<_> = env::args_os().skip(1).collect();
    if args.len() != 4 || args[0] != "--node-manifest" || args[2] != "--webhooks-manifest" {
        return Err("usage: layerx-webhooks-render-network --node-manifest NODE.yaml --webhooks-manifest DEVELOPER.yaml".into());
    }
    let node = fs::read_to_string(&args[1]).map_err(|error| error.to_string())?;
    let webhooks = fs::read_to_string(&args[3]).map_err(|error| error.to_string())?;
    let rendered = bind_network(&node, &webhooks)?;
    io::stdout()
        .lock()
        .write_all(rendered.as_bytes())
        .map_err(|error| error.to_string())
}

fn main() -> ExitCode {
    match render() {
        Ok(()) => ExitCode::SUCCESS,
        Err(error) => {
            eprintln!("{error}");
            ExitCode::FAILURE
        }
    }
}
