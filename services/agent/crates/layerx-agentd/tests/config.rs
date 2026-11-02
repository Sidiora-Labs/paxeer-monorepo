use std::collections::BTreeMap;
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};

use layerx_agentd::config::{
    load, validate, ConfigError, ReceiptKeySource, RejectionReason, SECURITY_RELEVANT_SETTINGS,
};
use layerx_types::verify::VerificationLevel;

static NEXT_FILE: AtomicU64 = AtomicU64::new(1);

fn path(label: &str) -> PathBuf {
    let sequence = NEXT_FILE.fetch_add(1, Ordering::Relaxed);
    std::env::temp_dir().join(format!(
        "layerx-agentd-config-{label}-{}-{sequence}.kv",
        std::process::id()
    ))
}

fn values() -> BTreeMap<String, String> {
    BTreeMap::from([
        ("network_id".to_owned(), "42".to_owned()),
        (
            "node_endpoint".to_owned(),
            "https://node.example/rpc".to_owned(),
        ),
        ("expected_chain_id".to_owned(), "125".to_owned()),
        ("tenants".to_owned(), "tenant-a,tenant-b".to_owned()),
        (
            "policy_sources".to_owned(),
            "tenant-a:/etc/layerx/policy-a.kvx,tenant-b:/etc/layerx/policy-b.kvx".to_owned(),
        ),
        (
            "signer_configurations".to_owned(),
            "tenant-a:/etc/layerx/signer-a.kvx,tenant-b:/etc/layerx/signer-b.kvx".to_owned(),
        ),
        (
            "verification_defaults".to_owned(),
            "tenant-a:state-proven,tenant-b:checkpoint-finalised".to_owned(),
        ),
        (
            "receipt_key_source".to_owned(),
            "/etc/layerx/receipt-keys.json".to_owned(),
        ),
    ])
}

fn source(values: &BTreeMap<String, String>) -> String {
    let mut rendered = String::new();
    for (key, value) in values {
        rendered.push_str(key);
        rendered.push('=');
        rendered.push_str(value);
        rendered.push('\n');
    }
    rendered
}

fn write_protected(path: &PathBuf, contents: impl AsRef<[u8]>) {
    fs::write(path, contents).unwrap_or_else(|error| panic!("write: {error}"));
    fs::set_permissions(path, fs::Permissions::from_mode(0o600))
        .unwrap_or_else(|error| panic!("permissions: {error}"));
}

#[test]
fn explicit_environment_precedence_produces_one_fully_typed_configuration() {
    let path = path("precedence");
    write_protected(&path, source(&values()));
    let environment = BTreeMap::from([
        (
            "LAYERX_NODE_ENDPOINT".to_owned(),
            "https://override.example/rpc".to_owned(),
        ),
        ("LAYERX_EXPECTED_CHAIN_ID".to_owned(), "126".to_owned()),
        (
            "LAYERX_RECEIPT_KEY_SOURCE".to_owned(),
            "px_getReceiptKeys".to_owned(),
        ),
        ("PATH".to_owned(), "/usr/bin".to_owned()),
    ]);
    let loaded = load(&path, &environment).unwrap_or_else(|error| panic!("load: {error}"));
    assert_eq!(loaded.network_id, 42);
    assert_eq!(loaded.node_endpoint, "https://override.example/rpc");
    assert_eq!(loaded.expected_chain_id, 126);
    assert_eq!(loaded.tenants.len(), 2);
    assert!(loaded
        .verification_defaults
        .values()
        .any(|level| *level == VerificationLevel::STATE_PROVEN));
    assert_eq!(loaded.policy_sources.len(), loaded.tenants.len());
    assert_eq!(loaded.signer_configurations.len(), loaded.tenants.len());
    assert_eq!(loaded.receipt_key_source, ReceiptKeySource::NodeRpc);
    let from_file =
        load(&path, &BTreeMap::new()).unwrap_or_else(|error| panic!("load file: {error}"));
    assert_eq!(from_file.node_endpoint, "https://node.example/rpc");
    assert_eq!(from_file.expected_chain_id, 125);
    assert_eq!(
        from_file.receipt_key_source,
        ReceiptKeySource::PinnedFile(PathBuf::from("/etc/layerx/receipt-keys.json"))
    );
    let _ = fs::remove_file(path);
}

#[test]
fn every_security_relevant_setting_is_required_without_a_default() {
    assert_eq!(SECURITY_RELEVANT_SETTINGS.len(), 8);
    for setting in SECURITY_RELEVANT_SETTINGS {
        let mut incomplete = values();
        incomplete.remove(setting.file_key);
        assert_eq!(
            validate(&incomplete),
            Err(ConfigError {
                setting: setting.file_key.to_owned(),
                reason: RejectionReason::Missing,
            })
        );
    }
}

#[test]
fn duplicate_file_setting_and_unknown_environment_setting_are_refused() {
    let path = path("ambiguous");
    let mut duplicated = source(&values());
    duplicated.push_str("network_id=43\n");
    write_protected(&path, duplicated);
    assert_eq!(
        load(&path, &BTreeMap::new()),
        Err(ConfigError {
            setting: "network_id".to_owned(),
            reason: RejectionReason::Duplicate,
        })
    );
    write_protected(&path, source(&values()));
    assert_eq!(
        load(
            &path,
            &BTreeMap::from([("LAYERX_NETWORK".to_owned(), "42".to_owned())])
        ),
        Err(ConfigError {
            setting: "LAYERX_NETWORK".to_owned(),
            reason: RejectionReason::Unknown,
        })
    );
    let _ = fs::remove_file(path);
}

#[test]
fn unsafe_paths_chain_ids_maps_and_blank_overrides_name_the_setting() {
    let cases = [
        ("network_id", "0", RejectionReason::InvalidInteger),
        ("node_endpoint", "relative.sock", RejectionReason::Invalid),
        ("expected_chain_id", "0", RejectionReason::InvalidInteger),
        (
            "policy_sources",
            "tenant-a:/etc/layerx/a.kvx",
            RejectionReason::IncompleteTenantMap,
        ),
        (
            "signer_configurations",
            "tenant-a:/etc/layerx/a.kvx,tenant-c:/etc/layerx/c.kvx",
            RejectionReason::IncompleteTenantMap,
        ),
        (
            "verification_defaults",
            "tenant-a:unverified,tenant-b:state-proven",
            RejectionReason::InvalidVerificationLevel,
        ),
        (
            "receipt_key_source",
            "relative.csv",
            RejectionReason::InvalidPath,
        ),
    ];
    for (setting, value, reason) in cases {
        let mut invalid = values();
        invalid.insert(setting.to_owned(), value.to_owned());
        assert_eq!(
            validate(&invalid),
            Err(ConfigError {
                setting: setting.to_owned(),
                reason,
            })
        );
    }

    let path = path("blank-override");
    write_protected(&path, source(&values()));
    assert_eq!(
        load(
            &path,
            &BTreeMap::from([("LAYERX_NETWORK_ID".to_owned(), String::new())])
        ),
        Err(ConfigError {
            setting: "network_id".to_owned(),
            reason: RejectionReason::Empty,
        })
    );
    let _ = fs::remove_file(path);
}
