use std::collections::BTreeMap;
use std::fs;
use std::os::unix::fs::PermissionsExt;
use std::path::PathBuf;

use layerx_agentd::config::{load, validate, ConfigError, ReceiptKeySource, RejectionReason};

fn values() -> BTreeMap<String, String> {
    BTreeMap::from([
        ("network_id".to_owned(), "42".to_owned()),
        (
            "node_endpoint".to_owned(),
            "https://node.example/rpc".to_owned(),
        ),
        ("expected_chain_id".to_owned(), "125".to_owned()),
        ("tenants".to_owned(), "tenant-a".to_owned()),
        (
            "policy_sources".to_owned(),
            "tenant-a:/etc/layerx/policy.kvx".to_owned(),
        ),
        (
            "signer_configurations".to_owned(),
            "tenant-a:/etc/layerx/signer.kvx".to_owned(),
        ),
        (
            "verification_defaults".to_owned(),
            "tenant-a:state-proven".to_owned(),
        ),
        (
            "receipt_key_source".to_owned(),
            "px_getReceiptKeys".to_owned(),
        ),
    ])
}

struct ConfigFile(PathBuf);

impl ConfigFile {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "layerx-agentd-config-node-rpc-{}.kv",
            std::process::id()
        ));
        Self(path)
    }

    fn write(&self, values: &BTreeMap<String, String>) {
        let source: String = values
            .iter()
            .map(|(key, value)| format!("{key}={value}\n"))
            .collect();
        fs::write(&self.0, source).unwrap_or_else(|error| panic!("write: {error}"));
        fs::set_permissions(&self.0, fs::Permissions::from_mode(0o600))
            .unwrap_or_else(|error| panic!("permissions: {error}"));
    }
}

impl Drop for ConfigFile {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.0);
    }
}

#[test]
fn node_endpoint_must_be_a_json_rpc_url() {
    let mut configuration = values();
    for endpoint in [
        "/run/layerx/layerxd.sock",
        "relative.sock",
        "file:///run/layerx/layerxd.sock",
        "unix:///run/layerx/layerxd.sock",
        "ftp://node.example/rpc",
        "ws://node.example/rpc",
        "https://",
        "https:///rpc",
        "https://user:password@node.example/rpc",
        "https://node.example/rpc#fragment",
        "https://node.example/rpc\n",
    ] {
        configuration.insert("node_endpoint".to_owned(), endpoint.to_owned());
        assert_eq!(
            validate(&configuration),
            Err(ConfigError {
                setting: "node_endpoint".to_owned(),
                reason: RejectionReason::Invalid,
            }),
            "endpoint: {endpoint:?}"
        );
    }
    for endpoint in [
        "https://node.example/rpc",
        "http://127.0.0.1:8545",
        "https://[::1]:8545/rpc?key=value",
    ] {
        configuration.insert("node_endpoint".to_owned(), endpoint.to_owned());
        let parsed = validate(&configuration).unwrap_or_else(|error| panic!("validate: {error}"));
        assert_eq!(parsed.node_endpoint, endpoint);
        assert_eq!(parsed.expected_chain_id, 125);
        assert_eq!(parsed.receipt_key_source, ReceiptKeySource::NodeRpc);
    }
    configuration.insert(
        "receipt_key_source".to_owned(),
        "/etc/layerx/receipt-keys.json".to_owned(),
    );
    assert_eq!(
        validate(&configuration)
            .unwrap_or_else(|error| panic!("validate: {error}"))
            .receipt_key_source,
        ReceiptKeySource::PinnedFile(PathBuf::from("/etc/layerx/receipt-keys.json"))
    );
    for chain_id in ["0", "-1", "abc", "18446744073709551616"] {
        configuration.insert("expected_chain_id".to_owned(), chain_id.to_owned());
        assert_eq!(
            validate(&configuration),
            Err(ConfigError {
                setting: "expected_chain_id".to_owned(),
                reason: RejectionReason::InvalidInteger,
            })
        );
    }
}

#[test]
fn expected_protocol_version_is_retired() {
    let file = ConfigFile::new();
    let mut configuration = values();
    for value in ["2", "", "not-a-version"] {
        configuration.insert("expected_protocol_version".to_owned(), value.to_owned());
        let expected = Err(ConfigError {
            setting: "expected_protocol_version".to_owned(),
            reason: RejectionReason::Retired,
        });
        assert_eq!(validate(&configuration), expected);
        file.write(&configuration);
        assert_eq!(load(&file.0, &BTreeMap::new()), expected);
        assert_eq!(
            load(
                &file.0,
                &BTreeMap::from([("LAYERX_EXPECTED_CHAIN_ID".to_owned(), "126".to_owned())])
            ),
            expected
        );
    }
    configuration.remove("expected_chain_id");
    assert_eq!(
        validate(&configuration),
        Err(ConfigError {
            setting: "expected_protocol_version".to_owned(),
            reason: RejectionReason::Retired,
        })
    );

    file.write(&values());
    for value in ["2", "", "not-a-version"] {
        assert_eq!(
            load(
                &file.0,
                &BTreeMap::from([
                    (
                        "LAYERX_EXPECTED_PROTOCOL_VERSION".to_owned(),
                        value.to_owned()
                    ),
                    ("LAYERX_EXPECTED_CHAIN_ID".to_owned(), "126".to_owned()),
                ])
            ),
            Err(ConfigError {
                setting: "LAYERX_EXPECTED_PROTOCOL_VERSION".to_owned(),
                reason: RejectionReason::Retired,
            })
        );
    }
}
