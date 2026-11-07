//! Qualification adapter to the deployed loader; no copied configuration parser.
#[allow(dead_code)]
#[path = "../../../../../services/interop/crates/x-websearch/src/config.rs"]
mod config;

fn main() -> std::process::ExitCode {
    let paths: Vec<_> = std::env::args_os().skip(1).collect();
    if paths.is_empty() {
        eprintln!("usage: bringup-xweb-config-loader CONFIG...");
        return std::process::ExitCode::FAILURE;
    }
    for path in paths {
        match config::load(std::path::Path::new(&path)) {
            Ok(config) => {
                let keys: Vec<_> = config
                    .gateway
                    .receipt_keys
                    .iter()
                    .map(|key| {
                        serde_json::json!({
                            "key_id": key.sequencer_id(),
                            "public_key": key.public_key(),
                            "first_height": key.first_batch_number(),
                            "last_height": key.last_batch_number(),
                        })
                    })
                    .collect();
                println!("{}", serde_json::json!({"receipt_keys": keys}));
            }
            Err(error) => {
                eprintln!("{error}");
                return std::process::ExitCode::FAILURE;
            }
        }
    }
    std::process::ExitCode::SUCCESS
}
