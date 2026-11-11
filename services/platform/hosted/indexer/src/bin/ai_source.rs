use std::net::TcpListener;

use layerx_indexer::ai_source::{serve, SourceConfig};

fn main() {
    if let Err(error) = run() {
        eprintln!("layerx-ai-source: {error}");
        std::process::exit(2);
    }
}

fn run() -> Result<(), Box<dyn std::error::Error>> {
    let config = SourceConfig::from_env()?;
    let listener = TcpListener::bind(config.listen)?;
    serve(&listener, &config)?;
    Ok(())
}
