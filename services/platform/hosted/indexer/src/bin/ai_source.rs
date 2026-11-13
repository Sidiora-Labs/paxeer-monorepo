use std::net::TcpListener;

use layerx_indexer::ai_source::{serve, SourceConfig};
use layerx_indexer::IndexError;

fn main() {
    if let Err(error) = run() {
        eprintln!("layerx-ai-source: {error}");
        std::process::exit(2);
    }
}

fn run() -> Result<(), IndexError> {
    let config = SourceConfig::from_env()?;
    let listener =
        TcpListener::bind(config.listen).map_err(|error| IndexError::Source(error.to_string()))?;
    serve(&listener, &config)?;
    Ok(())
}
