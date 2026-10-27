use std::collections::BTreeSet;
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};
use std::time::Duration;

use layerx_bridge_relayer::config::RelayerConfig;
use layerx_bridge_relayer::relayer::RelayerError;
use layerx_bridge_relayer::solana::release::{find_program_address, VAULT_SEED};
use layerx_bridge_relayer::solana::rpc::Commitment;
use layerx_bridge_relayer::solana::{base58_encode, handle, SOLANA_CHAIN_ID};
use layerx_bridge_relayer::transport::PaxeerEndpoint;
use layerx_proof::inclusion::SequencerAuthorization;
use openssl::asn1::Asn1Time;
use openssl::hash::MessageDigest;
use openssl::pkey::PKey;
use openssl::rsa::Rsa;
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};

const EXAMPLE_PATH: &str = concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../services/bridge/deploy/relayer-solana.example.json"
);
const EXAMPLE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../services/bridge/deploy/relayer-solana.example.json"
));
static NEXT_FILE: AtomicU64 = AtomicU64::new(0);

struct ConfigFile {
    directory: PathBuf,
}

impl ConfigFile {
    fn new(value: &Value) -> Self {
        let directory = std::env::temp_dir().join(format!(
            "layerx-operator-config-{}-{}",
            std::process::id(),
            NEXT_FILE.fetch_add(1, Ordering::Relaxed)
        ));
        std::fs::create_dir(&directory).expect("create exclusive config case directory");
        let file = Self { directory };
        std::fs::write(
            file.directory.join("relayer.json"),
            serde_json::to_vec(value).expect("encode public config case"),
        )
        .expect("write public config case");
        file
    }

    fn load(&self) -> Result<RelayerConfig, RelayerError> {
        RelayerConfig::load(&self.directory.join("relayer.json"))
    }
}

impl Drop for ConfigFile {
    fn drop(&mut self) {
        let _ = std::fs::remove_dir_all(&self.directory);
    }
}

fn example() -> Value {
    serde_json::from_str(EXAMPLE).expect("documented example JSON")
}

fn load(value: &Value) -> RelayerConfig {
    ConfigFile::new(value)
        .load()
        .expect("production loader accepts public config case")
}

fn refusal(value: &Value) -> String {
    match ConfigFile::new(value).load() {
        Err(RelayerError::Configuration(detail)) => detail,
        result => panic!("expected configuration refusal, got {result:?}"),
    }
}

fn replace(value: &mut Value, pointer: &str, replacement: Value) {
    *value.pointer_mut(pointer).expect("existing config field") = replacement;
}

fn receipt_key(sequencer: u8, key: u8, first: u64, last: u64) -> Value {
    json!({
        "sequencer_id": format!("0x{}", format!("{sequencer:02x}").repeat(32)),
        "public_key": format!("0x{}", format!("{key:02x}").repeat(32)),
        "first_height": first,
        "last_height": last,
    })
}

#[test]
fn paxeer_endpoints_are_built_without_the_verifier_crate() {
    let key = PKey::from_rsa(Rsa::generate(2048).expect("generate certificate key"))
        .expect("certificate public key");
    let mut name = X509NameBuilder::new().expect("certificate name builder");
    name.append_entry_by_text("CN", "paxeer.example.invalid")
        .expect("certificate name");
    let name = name.build();
    let mut certificate = X509::builder().expect("certificate builder");
    certificate.set_version(2).expect("X509 version");
    certificate.set_subject_name(&name).expect("subject name");
    certificate.set_issuer_name(&name).expect("issuer name");
    certificate.set_pubkey(&key).expect("certificate key");
    certificate
        .set_not_before(&Asn1Time::days_from_now(0).expect("start time"))
        .expect("valid from");
    certificate
        .set_not_after(&Asn1Time::days_from_now(1).expect("end time"))
        .expect("valid until");
    certificate
        .sign(&key, MessageDigest::sha256())
        .expect("sign certificate");
    let der = certificate.build().to_der().expect("DER certificate");

    let mut value = example();
    value["paxeer"]["receipt_keys"] = json!([receipt_key(1, 2, 0, u64::MAX)]);
    let file = ConfigFile::new(&value);
    let anchor = file.directory.join("paxeer-ca.der");
    std::fs::write(&anchor, &der).expect("write configured trust anchor");
    value["paxeer"]["endpoints"] = json!([
        {"url": "https://paxeer.example.invalid/rpc", "trust_anchor_der": anchor, "request_timeout_ms": 1234},
        {"url": "http://127.0.0.1:8545", "local_emulator": true, "request_timeout_ms": 5678}
    ]);
    std::fs::write(
        file.directory.join("relayer.json"),
        serde_json::to_vec(&value).expect("config JSON"),
    )
    .expect("write endpoint config");
    let mut config = file.load().expect("load endpoint config");
    let endpoints: Vec<PaxeerEndpoint> = config.paxeer_endpoints().expect("build native endpoints");
    assert_eq!(endpoints.len(), 2);
    assert_eq!(endpoints[0].url, "https://paxeer.example.invalid/rpc");
    assert_eq!(endpoints[0].expected_chain_id, 125);
    assert_eq!(endpoints[0].request_timeout, Duration::from_millis(1234));
    assert_eq!(
        endpoints[0].trust_anchor_der.as_deref(),
        Some(der.as_slice())
    );
    assert_eq!(endpoints[1].url, "http://127.0.0.1:8545");
    assert_eq!(endpoints[1].expected_chain_id, 125);
    assert_eq!(endpoints[1].request_timeout, Duration::from_millis(5678));
    assert_eq!(endpoints[1].trust_anchor_der, None);

    config.paxeer.endpoints[1].local_emulator = false;
    assert!(matches!(
        config.paxeer_endpoints(),
        Err(RelayerError::Configuration(_))
    ));
    config.paxeer.endpoints[1].local_emulator = true;
    config.paxeer.endpoints[0].local_emulator = true;
    assert!(config.paxeer_endpoints().is_err());
    config.paxeer.endpoints[0].local_emulator = false;
    std::fs::write(&anchor, b"invalid DER").expect("write invalid anchor");
    assert!(config.paxeer_endpoints().is_err());
    std::fs::remove_file(&anchor).expect("remove trust anchor");
    assert!(config.paxeer_endpoints().is_err());
}

#[test]
fn receipt_keys_are_loaded_as_height_windows() {
    let mut value = example();
    value["paxeer"]["receipt_keys"] = json!([
        receipt_key(0x11, 0x22, 0, 100),
        receipt_key(0x11, 0x33, 101, 101),
        receipt_key(0x44, 0x55, 102, u64::MAX),
    ]);
    let config = load(&value);
    assert_eq!(config.paxeer.receipt_keys.len(), 3);
    assert_eq!(config.paxeer.receipt_keys[0].sequencer_id, [0x11; 32]);
    assert_eq!(config.paxeer.receipt_keys[1].public_key, [0x33; 32]);
    assert_eq!(config.paxeer.receipt_keys[2].first_height, 102);
    assert_eq!(config.paxeer.receipt_keys[2].last_height, u64::MAX);
    assert_eq!(
        config.receipt_keys().expect("typed receipt keys"),
        vec![
            SequencerAuthorization::new([0x11; 32], [0x22; 32], 0, 100),
            SequencerAuthorization::new([0x11; 32], [0x33; 32], 101, 101),
            SequencerAuthorization::new([0x44; 32], [0x55; 32], 102, u64::MAX),
        ]
    );

    for (keys, expected) in [
        (json!([]), "paxeer needs at least one receipt key"),
        (
            json!([receipt_key(1, 2, 9, 8)]),
            "receipt key first_height must not exceed last_height",
        ),
        (
            json!([receipt_key(1, 2, 0, 100), receipt_key(1, 3, 100, 200)]),
            "receipt key windows must be ordered and non-overlapping",
        ),
        (
            json!([receipt_key(1, 2, 0, 100), receipt_key(3, 4, 50, 60)]),
            "receipt key windows must be ordered and non-overlapping",
        ),
        (
            json!([receipt_key(1, 2, 100, 200), receipt_key(3, 4, 0, 99)]),
            "receipt key windows must be ordered and non-overlapping",
        ),
        (
            json!([
                receipt_key(1, 2, u64::MAX, u64::MAX),
                receipt_key(3, 4, u64::MAX, u64::MAX)
            ]),
            "receipt key windows must be ordered and non-overlapping",
        ),
    ] {
        let mut invalid = value.clone();
        invalid["paxeer"]["receipt_keys"] = keys;
        assert_eq!(refusal(&invalid), expected);
        let unchecked: RelayerConfig =
            serde_json::from_value(invalid).expect("well-typed invalid windows");
        assert!(unchecked.validate().is_err());
        assert!(unchecked.receipt_keys().is_err());
    }
    for field in ["sequencer_id", "public_key"] {
        for malformed in ["0x01", "11", "0xzz", ""] {
            let mut invalid = value.clone();
            invalid["paxeer"]["receipt_keys"][0][field] = json!(malformed);
            assert!(refusal(&invalid).contains("hexadecimal"));
        }
    }
    for field in ["sequencer_id", "public_key", "first_height", "last_height"] {
        let mut invalid = value.clone();
        invalid["paxeer"]["receipt_keys"][0]
            .as_object_mut()
            .expect("receipt key object")
            .remove(field);
        assert!(refusal(&invalid).starts_with(&format!("missing field `{field}`")));
    }
    let mut missing = value.clone();
    missing["paxeer"]
        .as_object_mut()
        .expect("Paxeer object")
        .remove("receipt_keys");
    assert!(refusal(&missing).starts_with("missing field `receipt_keys`"));
    let mut unknown = value.clone();
    unknown["paxeer"]["receipt_keys"][0]["unexpected_operator_field"] = json!(true);
    assert!(refusal(&unknown).starts_with("unknown field `unexpected_operator_field`"));
}

#[test]
fn documented_example_loads_through_production_config() {
    let config = RelayerConfig::load(Path::new(EXAMPLE_PATH))
        .expect("unmodified documented example passes production validation");
    assert_eq!(config, load(&example()));
    let solana = config.solana.as_ref().expect("Solana entry");
    let settings = solana.settings().expect("typed observer settings");
    assert_eq!(settings.chain_id, SOLANA_CHAIN_ID);
    assert_eq!(settings.commitment, Commitment::Finalized);
    assert_eq!(settings.finality_depth, 32);
    assert_eq!(settings.start_slot, 1);
    assert_eq!(settings.max_slot_range, 100);
    assert_eq!(solana.mints().expect("typed release mints"), vec![[1; 32]]);
    let (authority, bump) = find_program_address(&[VAULT_SEED], &settings.program_id)
        .expect("public custody program PDA vector");
    assert_eq!(bump, 255);
    assert_eq!(
        base58_encode(&authority),
        "GxxA9Cs9v5pAGVsaCe2jjDrtmieeBijcY4S5HHTY8Vq6"
    );
    assert_eq!(settings.vault, handle(&authority));
    assert_ne!(settings.vault, handle(&settings.program_id));
    let mut handles = BTreeSet::new();
    for key in std::iter::once(&config.attestor)
        .chain(std::iter::once(&config.paxeer.submitter))
        .chain(config.chains.iter().map(|chain| &chain.submitter))
    {
        assert!(key.handle.starts_with("EXAMPLE_ONLY/"));
        assert!(handles.insert(key.handle.as_str()));
        k256::PublicKey::from_sec1_bytes(&key.public_key).expect("full public secp256k1 vector");
    }
    assert!(handles.insert(solana.fee_payer.handle.as_str()));
    assert!(solana.fee_payer.handle.starts_with("EXAMPLE_ONLY/"));
    let fee_public: [u8; 32] = solana
        .fee_payer
        .public_key
        .as_slice()
        .try_into()
        .expect("ed25519 width");
    ed25519_dalek::VerifyingKey::from_bytes(&fee_public).expect("public ed25519 vector");
    for rpc in std::iter::once(&solana.rpc).chain(config.chains.iter().map(|chain| &chain.rpc)) {
        assert_eq!(rpc.endpoints.len(), 3);
        assert_eq!(rpc.quorum, 2);
        let mut backends = BTreeSet::new();
        let mut origins = BTreeSet::new();
        for endpoint in &rpc.endpoints {
            assert!(endpoint.url.starts_with("https://"));
            assert!(endpoint.url.ends_with(".example.invalid/rpc"));
            assert!(origins.insert(endpoint.url.as_str()));
            assert!(endpoint.independent_backend.starts_with("EXAMPLE_ONLY-"));
            assert!(backends.insert(endpoint.independent_backend.as_str()));
            assert!(endpoint.ca_certificate_der.starts_with("/EXAMPLE_ONLY"));
            assert!(endpoint.bearer_token_file.starts_with("/EXAMPLE_ONLY"));
        }
    }
}

#[test]
fn solana_omission_and_empty_mints_preserve_observation_only_settings() {
    let configured = load(&example());
    let expected = configured.solana.as_ref().expect("Solana entry");
    for omitted in [false, true] {
        let mut value = example();
        if omitted {
            value["solana"]
                .as_object_mut()
                .expect("Solana object")
                .remove("release_mints");
        } else {
            value["solana"]["release_mints"] = json!([]);
        }
        let config = load(&value);
        let observation = config.solana.expect("observer remains configured");
        assert!(observation.release_mints.is_empty());
        assert!(observation.mints().expect("empty mint set").is_empty());
        assert_eq!(
            observation.settings().expect("observer settings"),
            expected.settings().expect("original settings")
        );
        assert_eq!(observation.fee_payer, expected.fee_payer);
    }
    let mut value = example();
    value
        .as_object_mut()
        .expect("config object")
        .remove("solana");
    let evm_only = load(&value);
    assert!(evm_only.solana.is_none());
    assert_eq!(evm_only.chains, configured.chains);
    assert_eq!(evm_only.paxeer, configured.paxeer);
}

#[test]
fn accepted_commitments_preserve_slot_policy() {
    for (text, commitment) in [
        ("confirmed", Commitment::Confirmed),
        ("finalized", Commitment::Finalized),
    ] {
        let mut value = example();
        value["solana"]["commitment"] = json!(text);
        value["solana"]["finality_depth"] = json!(1);
        value["solana"]["start_slot"] = json!(0);
        value["solana"]["max_slot_range"] = json!(1);
        let settings = load(&value)
            .solana
            .expect("Solana entry")
            .settings()
            .expect("settings");
        assert_eq!(settings.commitment, commitment);
        assert_eq!(settings.finality_depth, 1);
        assert_eq!(settings.start_slot, 0);
        assert_eq!(settings.max_slot_range, 1);
    }
}

#[test]
fn unknown_fields_are_refused_at_every_config_boundary() {
    for pointer in [
        "",
        "/signer",
        "/signer/endpoint",
        "/attestor",
        "/paxeer",
        "/paxeer/endpoints/0",
        "/paxeer/submitter",
        "/paxeer/gas",
        "/chains/0",
        "/chains/0/rpc",
        "/chains/0/rpc/endpoints/0",
        "/chains/0/submitter",
        "/chains/0/gas",
        "/solana",
        "/solana/rpc",
        "/solana/rpc/endpoints/0",
        "/solana/fee_payer",
    ] {
        let mut value = example();
        value
            .pointer_mut(pointer)
            .expect("schema object")
            .as_object_mut()
            .expect("object")
            .insert("unexpected_operator_field".to_owned(), json!(true));
        let detail = refusal(&value);
        assert!(
            detail.starts_with("unknown field `unexpected_operator_field`"),
            "{pointer}: {detail}"
        );
    }
}

#[test]
fn reserved_chain_identity_is_not_an_evm_destination() {
    let expected = format!(
        "solana must use the reserved chain id {SOLANA_CHAIN_ID} and no ethereum chain may"
    );
    for chain_id in [0, 1, 125, SOLANA_CHAIN_ID - 1, SOLANA_CHAIN_ID + 1] {
        let mut value = example();
        value["solana"]["chain_id"] = json!(chain_id);
        assert_eq!(refusal(&value), expected);
    }
    let mut value = example();
    value["chains"][0]["chain_id"] = json!(SOLANA_CHAIN_ID);
    assert_eq!(refusal(&value), expected);
}

#[test]
fn invalid_commitments_are_refused_during_load() {
    for commitment in ["processed", "Confirmed", "FINALIZED", "finalised", ""] {
        let mut value = example();
        value["solana"]["commitment"] = json!(commitment);
        let detail = refusal(&value);
        assert!(
            detail.starts_with(&format!("unknown variant `{commitment}`")),
            "{detail}"
        );
        assert!(
            detail.contains("expected `confirmed` or `finalized`"),
            "{detail}"
        );
    }
    for commitment in [Value::Null, json!(1), json!([])] {
        let mut value = example();
        value["solana"]["commitment"] = commitment;
        assert!(refusal(&value).starts_with("invalid type:"));
    }
}

#[test]
fn attestor_handle_cannot_pay_any_chain_fees() {
    for pointer in [
        "/solana/fee_payer/handle",
        "/paxeer/submitter/handle",
        "/chains/0/submitter/handle",
    ] {
        let mut value = example();
        let attestor = value["attestor"]["handle"].clone();
        replace(&mut value, pointer, attestor);
        assert_eq!(
            refusal(&value),
            "the attestor key must not pay transaction fees"
        );
    }
    let mut value = example();
    value["solana"]
        .as_object_mut()
        .expect("Solana object")
        .remove("release_mints");
    value["solana"]["fee_payer"]["handle"] = value["attestor"]["handle"].clone();
    assert_eq!(
        refusal(&value),
        "the attestor key must not pay transaction fees"
    );
}

#[test]
fn solana_identity_and_bounds_are_required() {
    for (pointer, replacement, expected) in [
        (
            "/solana/vault",
            json!("0x0000000000000000000000000000000000000000"),
            "solana needs a vault, a finality depth and a slot range",
        ),
        (
            "/solana/finality_depth",
            json!(0),
            "solana needs a vault, a finality depth and a slot range",
        ),
        (
            "/solana/max_slot_range",
            json!(0),
            "solana needs a vault, a finality depth and a slot range",
        ),
        (
            "/solana/program_id",
            json!("11111111111111111111111111111111"),
            "solana program id must not be the zero key",
        ),
        (
            "/solana/program_id",
            json!("not-a-base58-key"),
            "solana program id is not a base58 32-byte key",
        ),
        (
            "/solana/program_id",
            json!("1"),
            "solana program id is not a base58 32-byte key",
        ),
        (
            "/solana/fee_payer/handle",
            json!(""),
            "the solana fee payer needs a handle and a 32-byte public key",
        ),
        (
            "/solana/fee_payer/public_key",
            json!("0x01"),
            "the solana fee payer needs a handle and a 32-byte public key",
        ),
    ] {
        let mut value = example();
        replace(&mut value, pointer, replacement);
        assert_eq!(refusal(&value), expected, "{pointer}");
    }
    let mut value = example();
    value["solana"]["fee_payer"]["public_key"] = value["attestor"]["public_key"].clone();
    assert_eq!(
        refusal(&value),
        "the solana fee payer needs a handle and a 32-byte public key"
    );
    let mut value = example();
    value["solana"]["vault"] = json!("0x01");
    assert!(refusal(&value).contains("hex"));
}

#[test]
fn release_mints_are_distinct_nonzero_full_keys() {
    let mint = example()["solana"]["release_mints"][0].clone();
    for mints in [
        json!([mint.clone(), mint]),
        json!(["11111111111111111111111111111111"]),
    ] {
        let mut value = example();
        value["solana"]["release_mints"] = mints;
        assert_eq!(
            refusal(&value),
            "solana release mints must be distinct non-zero keys"
        );
    }
    for mint in ["", "1", "not-a-base58-key"] {
        let mut value = example();
        value["solana"]["release_mints"] = json!([mint]);
        assert_eq!(
            refusal(&value),
            "a solana release mint is not a base58 32-byte key"
        );
    }
}

#[test]
fn required_solana_fields_cannot_be_omitted() {
    for field in [
        "chain_id",
        "vault",
        "program_id",
        "commitment",
        "finality_depth",
        "start_slot",
        "max_slot_range",
        "rpc",
        "fee_payer",
    ] {
        let mut value = example();
        value["solana"]
            .as_object_mut()
            .expect("Solana object")
            .remove(field);
        let detail = refusal(&value);
        assert!(
            detail.starts_with(&format!("missing field `{field}`")),
            "{detail}"
        );
    }
}
