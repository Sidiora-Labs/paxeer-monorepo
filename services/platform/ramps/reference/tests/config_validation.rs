use std::fs::{self, OpenOptions};
use std::io::Write as _;
use std::os::unix::fs::OpenOptionsExt as _;
use std::path::PathBuf;
use std::process::Command;
use std::time::{SystemTime, UNIX_EPOCH};

use serde_json::{json, Value};

struct ConfigFile(PathBuf);

impl ConfigFile {
    fn new(label: &str) -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("clock")
            .as_nanos();
        Self(std::env::temp_dir().join(format!(
            "layerx-ramp-config-{label}-{}-{nanos}.json",
            std::process::id()
        )))
    }

    fn refusal(&self, config: &Value) -> String {
        let mut file = OpenOptions::new()
            .create(true)
            .truncate(true)
            .write(true)
            .mode(0o600)
            .open(&self.0)
            .expect("open protected config");
        file.write_all(&serde_json::to_vec(config).expect("encode config"))
            .expect("write protected config");
        drop(file);
        let output = Command::new(env!("CARGO_BIN_EXE_layerx-reference-ramp"))
            .arg(&self.0)
            .output()
            .expect("run reference ramp");
        let stderr = String::from_utf8(output.stderr).expect("startup diagnostic");
        assert!(!output.status.success(), "ramp accepted config: {stderr}");
        assert!(stderr.contains("refused startup"), "{stderr}");
        stderr
    }
}

impl Drop for ConfigFile {
    fn drop(&mut self) {
        let _ = fs::remove_file(&self.0);
    }
}

fn config() -> Value {
    json!({
        "listen": "127.0.0.1:0",
        "listener": "validation-boundary",
        "journal_path": "unused-journal.jsonl",
        "worker_id": "config-validation",
        "lease_seconds": 60,
        "reconcile_seconds": 5,
        "operator": {
            "principal_id": "ramp-operator",
            "account": "agent:did:layerx:ramp-operator:main",
            "signer_key_handle": "kms://ramp/layerx"
        },
        "quotes": [{
            "quote_id": "config-quote",
            "direction": "on_ramp",
            "layerx_asset": ([1_u8; 32]),
            "layerx_amount": 1_000_000,
            "external_currency": "EUR",
            "external_amount_minor": 100,
            "rate_numerator": 1_000_000,
            "rate_denominator": 100,
            "fee_minor": 2,
            "maximum_slippage_bps": 25,
            "context": ([8_u8; 32]),
            "provider_token": "provider-product",
            "payout_token": "beneficiary-token",
            "expires_at": 4_000_000_000_u64
        }],
        "client_tls": {
            "ca_pem": "unused-ca.pem",
            "identity_pkcs12": "unused-identity.p12",
            "identity_password_file": "unused-password",
            "timeout_seconds": 8
        },
        "identity": {
            "endpoint": "https://identity.example.test",
            "service_token_file": "unused-identity-token",
            "audience": "layerx-ramp"
        },
        "compliance": {
            "endpoint": "https://compliance.example.test",
            "service_token_file": "unused-compliance-token",
            "public_key": "22".repeat(32)
        },
        "provider": {
            "endpoint": "https://provider.example.test",
            "credential_file": "unused-provider-token",
            "settlement_path": "/layerx-ramp-v1/settlements",
            "status_path": "/layerx-ramp-v1/settlements"
        },
        "layerx": {
            "rpc_endpoint": "https://rpc.example.test:8545",
            "rpc_trust_anchor_der": "unused-anchor.der",
            "chain_id": 125,
            "operator_address": format!("0x{}", "12".repeat(20)),
            "receipt_keys": [{
                "keyId": "01".repeat(32),
                "publicKey": "d75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a",
                "firstHeight": 1,
                "lastHeight": 10
            }],
            "max_fee_per_gas": 1000,
            "max_priority_fee_per_gas": 2,
            "gas_limit_cap": 100000,
            "signer_endpoint": "https://signer.example.test",
            "signer_token_file": "unused-signer-token",
            "actor_did": "did:layerx:ramp-operator"
        },
        "paxeer": {
            "custody_endpoint": "https://custody.example.test",
            "custody_credential_file": "unused-custody-token",
            "broadcast_path": "/layerx-paxeer-v1/rebalances",
            "status_path": "/layerx-paxeer-v1/rebalances",
            "operator_account": "agent:did:layerx:ramp-operator:main",
            "wallet_address": "operator-wallet",
            "vault_id": "operator-vault",
            "signer_key_handle": "kms://ramp/paxeer",
            "rpc_endpoints": ["https://rpc-a.example.test:8545", "https://rpc-b.example.test:8545"],
            "rpc_trust_anchor_der": "unused-paxeer-anchor.der",
            "rpc_chain_id": 229,
            "rpc_minimum_agreement": 2,
            "required_confirmations": 12,
            "poll_cadence_seconds": 5,
            "delayed_after_polls": 12
        },
        "provider_callback_public_key": "44".repeat(32),
        "operator_control_token_file": "unused-operator-token"
    })
}

#[test]
fn reference_ramp_requires_paxeer_rpc_fields() {
    let file = ConfigFile::new("required-fields");
    let baseline = config();
    let stderr = file.refusal(&baseline);
    assert!(stderr.contains("listener must be tls or plain"), "{stderr}");
    for field in [
        "rpc_endpoints",
        "rpc_trust_anchor_der",
        "rpc_chain_id",
        "rpc_minimum_agreement",
        "required_confirmations",
        "delayed_after_polls",
    ] {
        let mut document = baseline.clone();
        document["paxeer"]
            .as_object_mut()
            .expect("Paxeer config")
            .remove(field);
        let stderr = file.refusal(&document);
        assert!(
            stderr.contains(&format!("missing field `{field}`")),
            "{stderr}"
        );
    }
}

#[test]
fn reference_ramp_refuses_operator_address_mismatch() {
    let file = ConfigFile::new("operator-address");
    let baseline = config();
    for address in [
        format!("0x{}", "12".repeat(20)),
        "12".repeat(20),
        format!("0x{}", "aB".repeat(20)),
    ] {
        let mut document = baseline.clone();
        document["layerx"]["operator_address"] = json!(address);
        let stderr = file.refusal(&document);
        assert!(stderr.contains("listener must be tls or plain"), "{stderr}");
    }
    for address in [
        String::new(),
        "0x".to_owned(),
        "0x1234".to_owned(),
        format!("0x{}", "12".repeat(19)),
        format!("0x{}", "12".repeat(21)),
        format!("0x{}1", "12".repeat(19)),
        format!("0x{}gg", "12".repeat(19)),
        format!("0x{}é", "12".repeat(19)),
        format!(" 0x{}", "12".repeat(20)),
        format!("0x{} ", "12".repeat(20)),
    ] {
        let mut document = baseline.clone();
        document["layerx"]["operator_address"] = json!(address);
        let stderr = file.refusal(&document);
        assert!(stderr.contains("operator_address"), "{stderr}");
        assert!(stderr.contains("20-byte hex address"), "{stderr}");
    }
    let mut document = baseline.clone();
    document["layerx"]["actor_did"] = json!("did:layerx:another-operator");
    let stderr = file.refusal(&document);
    assert!(
        stderr.contains("operator account binding rejected"),
        "{stderr}"
    );

    let mut document = baseline.clone();
    document["paxeer"]["operator_account"] = json!("agent:did:layerx:another-operator:main");
    let stderr = file.refusal(&document);
    assert!(
        stderr.contains("operator account binding rejected"),
        "{stderr}"
    );

    let mut document = baseline.clone();
    document["operator"]["account"] = json!("agent:did:layerx:another-operator:main");
    document["paxeer"]["operator_account"] = document["operator"]["account"].clone();
    let stderr = file.refusal(&document);
    assert!(
        stderr.contains("operator account binding rejected"),
        "{stderr}"
    );

    let mut document = baseline.clone();
    document["layerx"]["chain_id"] = json!(0);
    let stderr = file.refusal(&document);
    assert!(stderr.contains("chain_id: must be nonzero"), "{stderr}");

    let mut document = baseline.clone();
    document["layerx"]["receipt_keys"][0]["firstHeight"] = json!(11);
    let stderr = file.refusal(&document);
    assert!(
        stderr.contains("firstHeight exceeds lastHeight"),
        "{stderr}"
    );

    let mut document = baseline.clone();
    document["layerx"]["receipt_keys"][0]["firstHeight"] = json!(10);
    let stderr = file.refusal(&document);
    assert!(stderr.contains("listener must be tls or plain"), "{stderr}");

    let mut document = baseline.clone();
    let mut invalid_key = document["layerx"]["receipt_keys"][0].clone();
    invalid_key["firstHeight"] = json!(11);
    document["layerx"]["receipt_keys"]
        .as_array_mut()
        .expect("receipt keys")
        .push(invalid_key);
    let stderr = file.refusal(&document);
    assert!(
        stderr.contains("firstHeight exceeds lastHeight"),
        "{stderr}"
    );

    let mut document = baseline;
    document["layerx"]["receipt_keys"] = json!([]);
    let stderr = file.refusal(&document);
    assert!(
        stderr.contains("receipt_keys: must not be empty"),
        "{stderr}"
    );
}
