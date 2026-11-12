use std::path::Path;
use std::process::Command;

const RETIRED_INPUTS: [&str; 7] = [
    "LAYERX_INTEROP_SEQUENCER_PUBLIC_KEY_FILE",
    "LAYERX_INTEROP_SEQUENCER_ID_FILE",
    "LAYERX_INTEROP_SEQUENCER_FIRST_BATCH_FILE",
    "LAYERX_INTEROP_SEQUENCER_LAST_BATCH_FILE",
    "LAYERX_INTEROP_HOSTED_GATEWAY_URL",
    "LAYERX_INTEROP_RECEIPT_AUTHORITY_URL",
    "LAYERX_INTEROP_RECEIPT_AUTHORITY_TOKEN_FILE",
];

/// Checks that the listener uses the active receipt pin and sequencer settings.
///
/// # Panics
/// Panics when an active setting is absent or a retired setting is present.
pub fn assert_inputs(dir: &Path, names: impl Iterator<Item = String>) {
    for file in [
        "sequencer-public-key",
        "sequencer-id",
        "sequencer-first-batch",
        "sequencer-last-batch",
        "authority-token",
    ] {
        assert!(!dir.join(file).exists(), "retired fixture input {file}");
    }
    let names: Vec<_> = names.collect();
    for name in RETIRED_INPUTS {
        assert!(
            !names.iter().any(|key| key == name),
            "retired setting {name}"
        );
    }
    for name in [
        "LAYERX_INTEROP_SEQUENCER_URL",
        "LAYERX_INTEROP_RECEIPT_KEYS_FILE",
    ] {
        assert!(
            names.iter().any(|key| key == name),
            "missing active setting {name}"
        );
    }
    assert!(dir.join("receipt-keys.json").is_file());
}

/// Checks the listener command's configured inputs.
///
/// # Panics
/// Panics when an active setting is absent or a retired setting is present.
pub fn assert_command(command: &Command, dir: &Path) {
    assert_inputs(
        dir,
        command
            .get_envs()
            .filter_map(|(key, value)| value.map(|_| key.to_string_lossy().into_owned())),
    );
}
