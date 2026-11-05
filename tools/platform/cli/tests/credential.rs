//! Secrets live in the credential store, never in plaintext configuration.
//!
//! These tests exercise the real key and token management commands and assert
//! that private key seeds and API tokens are accepted by the credential store
//! yet never appear in the on-disk configuration file or on any output stream.

mod common;

use common::{envelope, error_envelope, string_field, Cli};
use ed25519_dalek::SigningKey;
use serde_json::Value;
use std::fmt::Write as _;

const SEED_HEX: &str = "1111111111111111111111111111111111111111111111111111111111111111";
const TOKEN: &str = "layerx-testnet-token-abcdef0123456789";

fn hex(bytes: &[u8]) -> String {
    bytes.iter().fold(String::new(), |mut output, byte| {
        assert!(write!(output, "{byte:02x}").is_ok());
        output
    })
}

fn expected_public_key() -> String {
    let mut seed = [0_u8; 32];
    seed.fill(0x11);
    let signing = SigningKey::from_bytes(&seed);
    hex(&signing.verifying_key().to_bytes())
}

#[test]
fn imported_seed_is_never_written_to_configuration() {
    if !common::credential_environment() {
        return;
    }
    let cli = Cli::new();
    let output = cli.run_with_stdin(&["--json", "key", "import", "alpha"], SEED_HEX);
    assert!(
        output.status.success(),
        "import should succeed: {}",
        String::from_utf8_lossy(&output.stderr)
    );

    let value = envelope(&output);
    assert_eq!(string_field(&value, "/kind"), "key.imported");
    let public_key = expected_public_key();
    assert_eq!(string_field(&value, "/data/public_key"), public_key);
    assert_eq!(
        string_field(&value, "/data/did"),
        format!("did:layerx:{public_key}")
    );

    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(
        !stdout.contains(SEED_HEX),
        "the private seed must never be echoed to standard output"
    );

    let config = cli.config_contents();
    assert!(
        !config.contains(SEED_HEX),
        "the private seed must never be written to plaintext configuration"
    );
    let parsed: Value = match serde_json::from_str(&config) {
        Ok(value) => value,
        Err(error) => panic!("configuration should be JSON: {error}; config={config}"),
    };
    let Some(metadata) = parsed.pointer("/keys/alpha") else {
        panic!("configuration should record key metadata: {config}");
    };
    let Some(object) = metadata.as_object() else {
        panic!("key metadata should be an object: {metadata}");
    };
    let mut fields: Vec<&String> = object.keys().collect();
    fields.sort();
    assert_eq!(
        fields,
        vec!["did", "public_key"],
        "configuration must carry only public key metadata"
    );
}

#[cfg(unix)]
#[test]
fn configuration_file_is_owner_only() {
    if !common::credential_environment() {
        return;
    }
    use std::os::unix::fs::PermissionsExt as _;

    let cli = Cli::new();
    let output = cli.run(&["--json", "key", "create", "alpha"]);
    assert!(output.status.success());

    let metadata = match std::fs::metadata(cli.config_path()) {
        Ok(metadata) => metadata,
        Err(error) => panic!("configuration should exist after key creation: {error}"),
    };
    assert_eq!(
        metadata.permissions().mode() & 0o777,
        0o600,
        "configuration must be readable only by its owner"
    );
}

#[test]
fn created_key_reports_its_secret_storage_and_hides_the_secret() {
    if !common::credential_environment() {
        return;
    }
    let cli = Cli::new();
    let created = cli.run(&["--json", "key", "create", "alpha"]);
    assert!(created.status.success());
    let public_key = string_field(&envelope(&created), "/data/public_key").to_owned();

    let shown = cli.run(&["--json", "key", "show", "alpha"]);
    assert!(shown.status.success());
    let value = envelope(&shown);
    assert_eq!(
        string_field(&value, "/data/secret_storage"),
        "operating-system-credential-store"
    );
    assert_eq!(string_field(&value, "/data/public_key"), public_key);
    let rendered = String::from_utf8_lossy(&shown.stdout);
    assert!(
        !rendered.contains("seed") && !rendered.contains("private"),
        "key metadata must not surface secret material"
    );
}

#[test]
fn api_token_is_never_written_to_configuration() {
    if !common::credential_environment() {
        return;
    }
    let cli = Cli::new();
    // Materialise a configuration file first so the assertion inspects a real file.
    assert!(cli
        .run(&["--json", "key", "create", "alpha"])
        .status
        .success());

    let output = cli.run_with_stdin(&["--json", "auth", "set", "--environment", "beta"], TOKEN);
    assert!(
        output.status.success(),
        "auth set should succeed: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    let value = envelope(&output);
    assert_eq!(string_field(&value, "/kind"), "auth.saved");
    assert_eq!(
        string_field(&value, "/data/secret_storage"),
        "operating-system-credential-store"
    );

    assert!(
        !String::from_utf8_lossy(&output.stdout).contains(TOKEN),
        "the API token must never be echoed to standard output"
    );
    assert!(
        !cli.config_contents().contains(TOKEN),
        "the API token must never be written to plaintext configuration"
    );
}

#[test]
fn duplicate_key_names_are_refused_with_a_machine_readable_error() {
    if !common::credential_environment() {
        return;
    }
    let cli = Cli::new();
    assert!(cli
        .run(&["--json", "key", "create", "alpha"])
        .status
        .success());

    let output = cli.run(&["--json", "key", "create", "alpha"]);
    assert!(!output.status.success());
    let value = error_envelope(&output);
    assert_eq!(value.pointer("/ok").and_then(Value::as_bool), Some(false));
    assert_eq!(
        value.pointer("/error/code").and_then(Value::as_str),
        Some("command_failed")
    );
}

#[test]
fn missing_dbus_prints_the_named_skip_reason() {
    let directory = tempfile::tempdir().expect("empty executable search directory");
    for test in [
        "imported_seed_is_never_written_to_configuration",
        #[cfg(unix)]
        "configuration_file_is_owner_only",
        "created_key_reports_its_secret_storage_and_hides_the_secret",
        "api_token_is_never_written_to_configuration",
        "duplicate_key_names_are_refused_with_a_machine_readable_error",
    ] {
        let output = std::process::Command::new(std::env::current_exe().expect("test executable"))
            .args(["--exact", test, "--nocapture"])
            .env("PATH", directory.path())
            .env_remove("LAYERX_CLI_TESTS_REQUIRE_DBUS")
            .output()
            .expect("credential test subprocess starts");
        assert!(output.status.success(), "{test}: {output:?}");
        assert!(
            String::from_utf8_lossy(&output.stderr)
                .contains("skipped: dbus-daemon is not installed"),
            "{test}: {output:?}"
        );
        assert!(String::from_utf8_lossy(&output.stdout).contains("1 passed"));
    }
}

#[test]
fn required_dbus_preserves_the_missing_daemon_panic() {
    let directory = tempfile::tempdir().expect("empty executable search directory");
    for required in ["", "1"] {
        let output = std::process::Command::new(std::env::current_exe().expect("test executable"))
            .args([
                "--exact",
                "imported_seed_is_never_written_to_configuration",
                "--nocapture",
            ])
            .env("PATH", directory.path())
            .env("LAYERX_CLI_TESTS_REQUIRE_DBUS", required)
            .output()
            .expect("credential test subprocess starts");
        assert_eq!(output.status.code(), Some(101), "{output:?}");
        let stderr = String::from_utf8_lossy(&output.stderr);
        assert!(
            stderr.contains("dbus-daemon must be installed for real credential tests"),
            "{stderr}"
        );
        assert!(!stderr.contains("skipped: dbus-daemon is not installed"));
        assert!(String::from_utf8_lossy(&output.stdout).contains("1 failed"));
    }
}

#[path = "common/credential_environment.rs"]
mod profile_credentials;

fn scratch(label: &str) -> std::path::PathBuf {
    let mut suffix = [0; 16];
    getrandom::fill(&mut suffix).expect("isolated credential profile entropy");
    std::env::temp_dir().join(format!("layerx-{label}-{}", hex(&suffix)))
}

struct ProfileEnvironment(Vec<(&'static str, Option<std::ffi::OsString>)>);
impl Drop for ProfileEnvironment {
    fn drop(&mut self) {
        for (name, value) in &self.0 {
            match value {
                Some(value) => std::env::set_var(name, value),
                None => std::env::remove_var(name),
            }
        }
    }
}

#[test]
fn same_alias_profiles_preserve_os_seeds_and_failed_publication() {
    use layerx_platform_cli::{config::Configuration, credential};
    let service = profile_credentials::CredentialEnvironment::new();
    let _restore = ProfileEnvironment(
        [
            "DBUS_SESSION_BUS_ADDRESS",
            "LAYERX_CONFIG",
            "LAYERX_CREDENTIAL_STORE",
        ]
        .into_iter()
        .map(|name| (name, std::env::var_os(name)))
        .collect(),
    );
    let probe = service.command("dbus-send");
    let address = probe
        .get_envs()
        .find(|(name, _)| *name == "DBUS_SESSION_BUS_ADDRESS")
        .and_then(|(_, value)| value)
        .expect("owned bus address")
        .to_owned();
    std::env::set_var("DBUS_SESSION_BUS_ADDRESS", address);
    std::env::set_var("LAYERX_CREDENTIAL_STORE", "os");
    assert!(keyring::Entry::store_status().is_ok());
    let fixture = keyring_core::Entry::new("dev.layerx.cli", "key:same").expect("fixture entry");
    fixture
        .set_password(&"22".repeat(32))
        .expect("fixture value");
    let a = service.root().join("profile-a.json");
    let b = service.root().join("profile-b.json");
    std::env::set_var("LAYERX_CONFIG", &a);
    let mut ca = Configuration::default();
    let ma = credential::create_key(&mut ca, "same", None).expect("first profile");
    let seed_a = credential::key_seed("same").expect("first OS seed");
    std::env::set_var("LAYERX_CONFIG", &b);
    let mut cb = Configuration::default();
    let mb = credential::create_key(&mut cb, "same", None).expect("second profile");
    let seed_b = credential::key_seed("same").expect("second OS seed");
    assert_eq!(
        credential::store_label(),
        "operating-system-credential-store"
    );
    assert_ne!(ma.public_key, mb.public_key);
    assert_ne!(*seed_a, *seed_b);
    assert!(credential::create_key(&mut Configuration::default(), "same", None).is_err());
    assert_eq!(
        *credential::key_seed("same").expect("unchanged orphan"),
        *seed_b
    );
    let failed = service.root().join("metadata-save-fails.json");
    std::fs::create_dir(&failed).expect("make publication fail");
    std::env::set_var("LAYERX_CONFIG", &failed);
    let mut bad = Configuration::default();
    assert!(credential::create_key(&mut bad, "same", None).is_err());
    assert!(bad.keys.is_empty());
    let rolled_back = keyring_core::Entry::new(
        "dev.layerx.cli",
        &credential::key_entry_identity(&failed, "same").expect("failed profile identity"),
    )
    .expect("failed profile entry");
    assert!(matches!(
        rolled_back.get_password(),
        Err(keyring_core::Error::NoEntry)
    ));
    for (path, metadata, seed) in [(&a, &ma, &seed_a), (&b, &mb, &seed_b)] {
        std::env::set_var("LAYERX_CONFIG", path);
        assert_eq!(
            Configuration::load().expect("reopen profile").keys["same"].public_key,
            metadata.public_key
        );
        assert_eq!(
            *credential::key_seed("same").expect("preserved OS seed"),
            **seed
        );
    }
    std::env::set_var("LAYERX_CONFIG", &a);
    credential::delete_key(&mut ca, "same").expect("delete first profile");
    assert!(credential::key_seed("same").is_err());
    std::env::set_var("LAYERX_CONFIG", &b);
    assert_eq!(
        Configuration::load()
            .expect("second profile survives deletion")
            .keys["same"]
            .public_key,
        mb.public_key
    );
    assert_eq!(
        *credential::key_seed("same").expect("second seed survives deletion"),
        *seed_b
    );
    assert_eq!(
        fixture.get_password().expect("legacy fixture survives"),
        "22".repeat(32)
    );
    credential::delete_key(&mut cb, "same").expect("delete second profile");
    assert_eq!(
        fixture.get_password().expect("fixture unchanged"),
        "22".repeat(32)
    );
    fixture.delete_credential().expect("remove fixture");
}
