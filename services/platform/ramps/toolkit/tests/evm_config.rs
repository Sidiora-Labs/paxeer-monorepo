use std::fs;
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::process::Command;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

use layerx_ramp_toolkit::clients::{LayerxConfig, MutualTlsClient, MutualTlsFiles, SecretFile};
use serde_json::{json, Value};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

struct ClientFiles(PathBuf);

impl ClientFiles {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "ramp-evm-config-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).expect("create client credential directory");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
            .expect("protect client credential directory");
        let files = Self(path);
        files.openssl(&[
            "req",
            "-x509",
            "-newkey",
            "rsa:2048",
            "-nodes",
            "-days",
            "1",
            "-subj",
            "/CN=ramp-config-test",
            "-keyout",
            "key.pem",
            "-out",
            "ca.pem",
        ]);
        files.openssl(&["x509", "-in", "ca.pem", "-outform", "DER", "-out", "ca.der"]);
        files.openssl(&[
            "pkcs12",
            "-export",
            "-inkey",
            "key.pem",
            "-in",
            "ca.pem",
            "-out",
            "identity.p12",
            "-passout",
            "pass:configuration-test",
        ]);
        fs::set_permissions(
            files.0.join("identity.p12"),
            fs::Permissions::from_mode(0o600),
        )
        .expect("protect client identity");
        for (name, value) in [
            ("password", b"configuration-test".as_slice()),
            ("token", b"configuration-test-token".as_slice()),
        ] {
            use std::io::Write as _;
            let mut file = fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(files.0.join(name))
                .expect("create private credential");
            file.write_all(value).expect("write private credential");
        }
        files
    }

    fn openssl(&self, args: &[&str]) {
        let output = Command::new("openssl")
            .args(args)
            .current_dir(&self.0)
            .output()
            .expect("run openssl to create real TLS credentials");
        assert!(
            output.status.success(),
            "openssl failed: {:?}",
            output.status
        );
    }

    fn http(&self) -> MutualTlsClient {
        MutualTlsClient::new(
            &MutualTlsFiles {
                ca_pem: self.0.join("ca.pem"),
                identity_pkcs12: SecretFile::new(self.0.join("identity.p12"))
                    .expect("private TLS identity"),
                identity_password: SecretFile::new(self.0.join("password"))
                    .expect("private identity password"),
            },
            Duration::from_secs(1),
        )
        .expect("construct mutual TLS client")
    }

    fn document(&self) -> Value {
        json!({
            "rpc_endpoint": "https://sequencer.example.test",
            "rpc_trust_anchor_der": self.0.join("ca.der"),
            "chain_id": 125,
            "operator_address": format!("0x{}", "12".repeat(20)),
            "receipt_keys": [{
                "keyId": format!("0x{}", "34".repeat(32)),
                "publicKey": "0xd75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
                "firstHeight": 10,
                "lastHeight": 20
            }],
            "max_fee_per_gas": 100,
            "max_priority_fee_per_gas": 2,
            "gas_limit_cap": 100000,
            "signer_endpoint": "https://signer.example.test",
            "signer_token_file": self.0.join("token"),
            "actor_did": "did:layerx:ramp"
        })
    }

    fn config(&self) -> LayerxConfig {
        serde_json::from_value(self.document()).expect("deserialize sequencer config")
    }
}

impl Drop for ClientFiles {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove client credentials");
    }
}

#[test]
fn layerx_config_refuses_zero_chain_id() {
    let files = ClientFiles::new();
    let mut config = files.config();
    assert_eq!(config.chain_id, 125);
    let client = config
        .build(files.http())
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(client.chain_id, 125);
    assert_eq!(client.operator_address, [0x12; 20]);
    assert_eq!(client.max_fee_per_gas, 100);
    assert_eq!(client.max_priority_fee_per_gas, 2);
    assert_eq!(client.gas_limit_cap, 100000);
    assert_eq!(client.actor_did, "did:layerx:ramp");
    assert_eq!(client.signer_token, "configuration-test-token");
    for bytes in [19, 21] {
        config.operator_address = format!("0x{}", "12".repeat(bytes));
        let error = config
            .build(files.http())
            .err()
            .expect("operator address must contain exactly 20 bytes");
        assert!(error.contains("operator_address"), "{error}");
    }
    config.operator_address = format!("0x{}", "12".repeat(20));
    config.chain_id = 0;
    let error = config
        .build(files.http())
        .err()
        .expect("zero chain id must fail");
    assert!(error.contains("chain_id"), "unexpected error: {error}");
    config.chain_id = 125;
    config.max_priority_fee_per_gas = config.max_fee_per_gas + 1;
    assert!(config
        .build(files.http())
        .err()
        .expect("inverted fees must fail")
        .contains("max_priority_fee_per_gas"));
    config.max_priority_fee_per_gas = config.max_fee_per_gas;
    assert!(config.build(files.http()).is_ok());
    config.gas_limit_cap = 0;
    assert!(config
        .build(files.http())
        .err()
        .expect("zero gas cap must fail")
        .contains("gas_limit_cap"));
}

#[test]
fn layerx_config_refuses_inverted_receipt_key_range() {
    let files = ClientFiles::new();
    let mut config = files.config();
    let client = config
        .build(files.http())
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(client.receipt_keys.len(), 1);
    assert_eq!(client.receipt_keys[0].key_id, [0x34; 32]);
    assert_eq!(client.receipt_keys[0].first_height, 10);
    assert_eq!(client.receipt_keys[0].last_height, 20);
    config.receipt_keys[0].first_height = 21;
    let error = config
        .build(files.http())
        .err()
        .expect("inverted heights must fail");
    assert!(
        error.contains("receipt_keys")
            && error.contains("firstHeight")
            && error.contains("lastHeight"),
        "{error}"
    );
    config.receipt_keys[0].first_height = 20;
    assert!(
        config.build(files.http()).is_ok(),
        "a single-height range is valid"
    );
    config.receipt_keys.clear();
    assert!(config
        .build(files.http())
        .err()
        .expect("empty receipt pins must fail")
        .contains("receipt_keys"));
}

#[test]
fn layerx_config_rejects_retired_gateway_field() {
    let files = ClientFiles::new();
    let mut document = files.document();
    assert!(serde_json::from_value::<LayerxConfig>(document.clone()).is_ok());
    document["gateway_endpoint"] = json!("https://gateway.example.test");
    let error = serde_json::from_value::<LayerxConfig>(document)
        .expect_err("retired gateway_endpoint must be rejected");
    assert!(
        error
            .to_string()
            .contains("unknown field `gateway_endpoint`"),
        "{error}"
    );
    let mut document = files.document();
    document["network_id"] = json!(125);
    let error = serde_json::from_value::<LayerxConfig>(document)
        .expect_err("retired network_id configuration must be rejected");
    assert!(
        error.to_string().contains("unknown field `network_id`"),
        "{error}"
    );
}
