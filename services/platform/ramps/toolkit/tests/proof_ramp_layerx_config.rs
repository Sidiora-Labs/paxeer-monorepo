//! `LayerX` configuration serializes and decodes receipt key pins, fees, and trust anchors.
//! Invalid certificate inputs are rejected before client construction.

include!("support/required.rs");

use std::fs;
use std::io::Write as _;
use std::os::unix::fs::{OpenOptionsExt as _, PermissionsExt as _};
use std::path::PathBuf;
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

use ed25519_dalek::SigningKey;
use layerx_ramp_toolkit::clients::{
    parse_hex32, LayerxClient, LayerxConfig, MutualTlsClient, MutualTlsFiles, ReceiptKeyConfig,
    SecretFile,
};
use serde::{Deserialize, Serialize};
use serde_json::{json, Value};

static NEXT_DIRECTORY: AtomicU64 = AtomicU64::new(0);

// Only the shared key is in this envelope; its value is the production type.
#[derive(Debug, Deserialize, Serialize)]
#[serde(deny_unknown_fields)]
struct RampConfig {
    layerx: LayerxConfig,
}

pub(crate) struct ClientFiles(pub(crate) PathBuf);

impl ClientFiles {
    pub(crate) fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "proof-ramp-layerx-config-{}-{}",
            std::process::id(),
            NEXT_DIRECTORY.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).required("create private credential directory");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700))
            .required("protect credential directory");
        let files = Self(path);
        // Committed test-only identity; no subprocess or installed TLS tool is needed.
        fs::write(
            files.0.join("ca.pem"),
            include_bytes!("fixtures/repair_22_33_1486/ca.pem"),
        )
        .required("value");
        fs::write(
            files.0.join("ca-0.der"),
            include_bytes!("fixtures/repair_22_33_1486/ca.der"),
        )
        .required("value");
        fs::copy(files.0.join("ca-0.der"), files.0.join("ca-1.der")).required("value");
        fs::write(
            files.0.join("identity.p12"),
            include_bytes!("fixtures/repair_22_33_1486/identity.p12"),
        )
        .required("value");
        fs::set_permissions(
            files.0.join("identity.p12"),
            fs::Permissions::from_mode(0o600),
        )
        .required("protect real TLS identity");
        for (name, value) in [
            ("password", "ramp-config-proof"),
            ("token-0", "proof-credential-0"),
            ("token-1", "proof-credential-1"),
        ] {
            let mut file = fs::OpenOptions::new()
                .write(true)
                .create_new(true)
                .mode(0o600)
                .open(files.0.join(name))
                .required("create private credential");
            file.write_all(value.as_bytes())
                .required("write credential");
        }
        files
    }

    pub(crate) fn http(&self) -> MutualTlsClient {
        MutualTlsClient::new(
            &MutualTlsFiles {
                ca_pem: self.0.join("ca.pem"),
                identity_pkcs12: SecretFile::new(self.0.join("identity.p12"))
                    .required("private identity"),
                identity_password: SecretFile::new(self.0.join("password"))
                    .required("private password"),
            },
            Duration::from_secs(1),
        )
        .required("real mutual TLS client")
    }

    pub(crate) fn producer(&self, round: u8) -> LayerxConfig {
        let signing_key = SigningKey::from_bytes(&[round + 1; 32]);
        LayerxConfig {
            rpc_endpoint: format!("https://sequencer-{round}.example.test/rpc"),
            rpc_trust_anchor_der: self.0.join(format!("ca-{round}.der")),
            chain_id: 125 + u64::from(round),
            operator_address: format!("0x{}", format!("{:02x}", 0x12 + round).repeat(20)),
            receipt_keys: vec![ReceiptKeyConfig {
                key_id: format!("0x{}", format!("{:02x}", 0x34 + round).repeat(32)),
                public_key: format!(
                    "0x{}",
                    layerx_ramp_toolkit::clients::hex(signing_key.verifying_key().as_bytes())
                ),
                first_height: 10 + u64::from(round),
                last_height: 20 + u64::from(round),
            }],
            max_fee_per_gas: u128::from(u64::MAX) + 100 + u128::from(round),
            max_priority_fee_per_gas: u128::from(u64::MAX) + 2 + u128::from(round),
            gas_limit_cap: 100_000 + u64::from(round),
            signer_endpoint: format!("https://signer-{round}.example.test"),
            signer_token_file: self.0.join(format!("token-{round}")),
            actor_did: format!("did:layerx:ramp-{round}"),
        }
    }

    fn transport(producer: &LayerxConfig) -> Vec<u8> {
        serde_json::to_vec(&RampConfig {
            layerx: producer.clone(),
        })
        .required("production types emit the shared layerx schema")
    }

    fn decode(bytes: &[u8]) -> LayerxConfig {
        serde_json::from_slice::<RampConfig>(bytes)
            .required("consumer decodes producer's layerx key")
            .layerx
    }

    fn observe_twice(&self) -> LayerxConfig {
        for round in 0..=1 {
            let producer = self.producer(round);
            let bytes = Self::transport(&producer);
            let decoded = Self::decode(&bytes);
            assert_eq!(decoded.rpc_endpoint, producer.rpc_endpoint);
            assert_eq!(decoded.rpc_trust_anchor_der, producer.rpc_trust_anchor_der);
            let client = decoded
                .build(self.http())
                .unwrap_or_else(|error| panic!("{error}"));
            assert_eq!(client.chain_id, producer.chain_id);
            assert_eq!(client.gas_limit_cap, producer.gas_limit_cap);
            assert_eq!(client.max_fee_per_gas, producer.max_fee_per_gas);
            assert_eq!(
                client.max_priority_fee_per_gas,
                producer.max_priority_fee_per_gas
            );
            assert_eq!(client.operator_address, [0x12 + round; 20]);
            assert_eq!(client.receipt_keys.len(), producer.receipt_keys.len());
            for (observed, produced) in client.receipt_keys.iter().zip(&producer.receipt_keys) {
                assert_eq!(
                    observed.key_id,
                    parse_hex32(&produced.key_id).required("producer id")
                );
                assert_eq!(
                    observed.public_key,
                    parse_hex32(&produced.public_key).required("producer key")
                );
                assert_eq!(observed.first_height, produced.first_height);
                assert_eq!(observed.last_height, produced.last_height);
            }
            // Debug exposes the constructed endpoint's URL without making an RPC call.
            assert!(format!("{:?}", client.sequencer)
                .contains(&format!("url: {:?}", producer.rpc_endpoint)));
            assert_eq!(client.actor_did, producer.actor_did);
            assert_eq!(client.signer_token, format!("proof-credential-{round}"));
        }
        self.producer(1)
    }

    fn refusal(&self, producer: &LayerxConfig) -> std::string::String {
        Self::decode(&Self::transport(producer))
            .build(self.http())
            .err()
            .required("real config consumer must refuse invalid producer state")
    }
}

impl Drop for ClientFiles {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).required("remove private proof credentials");
    }
}

#[test]
fn u64() {
    let files = ClientFiles::new();
    let mut producer = files.observe_twice();
    producer.chain_id = 0;
    assert_eq!(
        files.refusal(&producer),
        "invalid LayerX chain_id: must be nonzero"
    );
    producer.chain_id = 126;
    producer.gas_limit_cap = 0;
    assert_eq!(
        files.refusal(&producer),
        "invalid LayerX gas_limit_cap: must be nonzero"
    );
}

#[test]
fn u128() {
    let files = ClientFiles::new();
    let mut producer = files.observe_twice();
    producer.max_fee_per_gas = u128::MAX;
    producer.max_priority_fee_per_gas = u128::MAX;
    let client: LayerxClient = ClientFiles::decode(&ClientFiles::transport(&producer))
        .build(files.http())
        .unwrap_or_else(|error| panic!("{error}"));
    assert_eq!(client.max_fee_per_gas, u128::MAX);
    assert_eq!(client.max_priority_fee_per_gas, u128::MAX);
    producer.max_fee_per_gas -= 1;
    assert_eq!(
        files.refusal(&producer),
        "invalid LayerX max_priority_fee_per_gas: exceeds max_fee_per_gas"
    );
}

#[test]
fn string_field() {
    let files = ClientFiles::new();
    let mut producer = files.observe_twice();
    producer.operator_address = "0x12".to_owned();
    assert_eq!(
        files.refusal(&producer),
        "invalid LayerX operator_address: expected 20 bytes"
    );
    producer.operator_address = files.producer(1).operator_address;
    producer.rpc_endpoint = "http://untrusted.example.test".to_owned();
    assert_eq!(
        files.refusal(&producer),
        "LayerX rpc_endpoint or rpc_trust_anchor_der rejected"
    );
}

#[test]
fn vector_field() {
    let files = ClientFiles::new();
    let mut producer = files.observe_twice();
    producer.receipt_keys[0].first_height = producer.receipt_keys[0].last_height + 1;
    assert_eq!(
        files.refusal(&producer),
        "invalid LayerX receipt_keys: firstHeight exceeds lastHeight"
    );
    producer.receipt_keys.clear();
    assert_eq!(
        files.refusal(&producer),
        "invalid LayerX receipt_keys: must not be empty"
    );
}

#[test]
fn optional_field() {
    let files = ClientFiles::new();
    let mut producer = files.observe_twice();
    // Reuse exactly the already-consumed bytes after corrupting their referenced
    // certificate. A cached config/client would incorrectly keep succeeding.
    let bytes = ClientFiles::transport(&producer);
    fs::write(&producer.rpc_trust_anchor_der, b"malformed DER")
        .required("corrupt producer's anchor file");
    let error = ClientFiles::decode(&bytes)
        .build(files.http())
        .err()
        .required("consumer must re-read and refuse malformed trust anchor");
    assert_eq!(error, "invalid LayerX rpc_trust_anchor_der");

    // Malformed transport cases start from real producer output. Small fees
    // allow Value mutations without converting the full-width fees to floats.
    producer.max_fee_per_gas = 100;
    producer.max_priority_fee_per_gas = 2;
    let document: Value = serde_json::from_slice(&ClientFiles::transport(&producer))
        .required("producer JSON for malformed-input cases");
    for malformed in [Value::Null, json!(42)] {
        let mut changed = document.clone();
        changed["layerx"]["rpc_trust_anchor_der"] = malformed;
        let bytes = serde_json::to_vec(&changed).required("malformed producer JSON");
        assert!(serde_json::from_slice::<RampConfig>(&bytes).is_err());
    }
    let mut missing = document.clone();
    missing["layerx"]
        .as_object_mut()
        .required("layerx object")
        .remove("rpc_trust_anchor_der");
    let error = serde_json::from_slice::<RampConfig>(
        &serde_json::to_vec(&missing).required("missing-anchor document"),
    )
    .err()
    .required("missing anchor must fail at consumer decoding");
    assert!(error
        .to_string()
        .contains("missing field `rpc_trust_anchor_der`"));
    let mut retired = document;
    retired["layerx"]["gateway_endpoint"] = json!("https://retired.example.test");
    let error = serde_json::from_slice::<RampConfig>(
        &serde_json::to_vec(&retired).required("retired-field document"),
    )
    .err()
    .required("retired fields must fail at consumer decoding");
    assert!(error
        .to_string()
        .contains("unknown field `gateway_endpoint`"));
    assert!(
        serde_json::from_slice::<RampConfig>(&bytes[..bytes.len() - 1]).is_err(),
        "truncated producer output must be refused by the decoder"
    );
}
