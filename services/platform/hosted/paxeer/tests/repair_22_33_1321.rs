#![allow(non_snake_case)]

use paxeer_native::{
    account_identity, BootstrapRequest, ImmutableInputs, NativeEvidence, TenantAccount,
};
use serde_json::{json, Value};
use std::fs;
use std::os::unix::fs::{symlink, PermissionsExt};
use std::path::{Path, PathBuf};
use std::sync::atomic::{AtomicU64, Ordering};

#[path = "support/native_peer.rs"]
mod native_peer;
use native_peer::{transcript, NativePeer};

static NEXT: AtomicU64 = AtomicU64::new(0);
struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        let path = std::env::temp_dir().join(format!(
            "paxeer-repair-1321-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::SeqCst)
        ));
        fs::create_dir(&path).unwrap();
        fs::set_permissions(&path, fs::Permissions::from_mode(0o700)).unwrap();
        Self(path)
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).unwrap();
    }
}

fn write_private(path: &Path, bytes: &[u8]) {
    fs::write(path, bytes).unwrap();
    fs::set_permissions(path, fs::Permissions::from_mode(0o600)).unwrap();
}

fn node_fixture() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../node/tests/fixtures/repair_22_31_355")
}
fn node_configuration() -> Value {
    serde_json::from_slice(&fs::read(node_fixture().join("native_configuration.json")).unwrap())
        .unwrap()
}

fn request(root: &Path) -> BootstrapRequest {
    let source = root.join("inputs");
    fs::create_dir(&source).unwrap();
    for name in ["genesis.json", "config.toml", "app.toml"] {
        fs::copy(node_fixture().join(name), source.join(name)).unwrap();
    }
    let fixture = node_configuration();
    write_private(
        &source.join("priv_validator_key.json"),
        &serde_json::to_vec(&fixture["validator_key"]).unwrap(),
    );
    // The guard treats certificate bytes as immutable; the native bootstrap owns X.509 validation.
    fs::copy(
        node_fixture().join("tls-certificate.pem.txt"),
        source.join("rpc-ca.der"),
    )
    .unwrap();
    write_private(
        &root.join("receipt.key"),
        fixture["receipt_seed"].as_str().unwrap().as_bytes(),
    );
    write_private(
        &root.join("funding.key"),
        fixture["funding_scalar"].as_str().unwrap().as_bytes(),
    );
    let pairs = [
        ("data-dir", root.join("data").display().to_string()),
        ("native-inputs", source.display().to_string()),
        ("chain-id", "hyperpax_125-1".into()),
        ("network-id", "402".into()),
        ("evm-chain-id", "125".into()),
        ("asset", fixture["asset"].as_str().unwrap().into()),
        ("gateway-endpoint", "https://localhost:9443".into()),
        (
            "receipt-key-file",
            root.join("receipt.key").display().to_string(),
        ),
        ("treasury-signer-socket", "/keys/treasury.sock".into()),
        (
            "funding-sender-key-file",
            root.join("funding.key").display().to_string(),
        ),
    ];
    let arguments = pairs
        .into_iter()
        .flat_map(|(key, value)| [format!("--{key}"), value])
        .collect::<Vec<_>>();
    BootstrapRequest::parse(&arguments).unwrap()
}

fn publish_generation(request: &BootstrapRequest) {
    // Committed generation peer: exact native bootstrap copy and publication layout.
    fs::create_dir_all(request.data.join("config")).unwrap();
    for name in [
        "genesis.json",
        "config.toml",
        "app.toml",
        "priv_validator_key.json",
    ] {
        write_private(
            &request.data.join("config").join(name),
            &fs::read(request.source.join(name)).unwrap(),
        );
    }
    fs::copy(
        request.source.join("rpc-ca.der"),
        request.data.join("rpc-ca.der"),
    )
    .unwrap();
    for name in ["node.env", "core.env"] {
        fs::copy(
            Path::new(env!("CARGO_MANIFEST_DIR"))
                .join("tests/fixtures/repair_1321")
                .join(name),
            request.data.join(name),
        )
        .unwrap();
    }
    let mut binding = serde_json::Map::new();
    use sha2::{Digest, Sha256};
    for name in [
        "config/genesis.json",
        "config/config.toml",
        "config/app.toml",
        "node.env",
        "core.env",
        "rpc-ca.der",
    ] {
        binding.insert(
            name.into(),
            Value::String(paxeer_native::hex(&Sha256::digest(
                fs::read(request.data.join(name)).unwrap(),
            ))),
        );
    }
    write_private(
        &request.data.join(".native-prepared.json"),
        &serde_json::to_vec(&binding).unwrap(),
    );
}

#[test]
fn TestRepairM_1321_InitializationRestartDrift() {
    let root = Directory::new();
    let request = request(&root.0);
    let inputs = ImmutableInputs::capture(&request.source, &request.options).unwrap();
    let mut guard = request.open().unwrap();
    assert!(!guard.initialized());
    assert!(request
        .open()
        .err()
        .unwrap()
        .to_string()
        .contains("already owned"));
    publish_generation(&request);
    guard.complete(&inputs).unwrap();
    drop(guard);
    let guard = request.open().unwrap();
    assert!(guard.initialized());
    drop(guard);
    let marker = root.0.join(".data.native-initialization/initialized.json");
    let original_marker = fs::read(&marker).unwrap();
    let genesis = request.source.join("genesis.json");
    let original = fs::read(&genesis).unwrap();
    let value: Value = serde_json::from_slice(&original).unwrap();
    fs::write(&genesis, serde_json::to_vec_pretty(&value).unwrap()).unwrap();
    assert!(
        request.open().unwrap().initialized(),
        "JSON whitespace is not drift"
    );
    for (pointer, changed, diagnostic) in [
        (
            "/app_state/gov/deposit_params/min_deposit/0/denom",
            json!("uother"),
            "denom",
        ),
        (
            "/app_state/sequencer/leaders/0",
            json!("AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="),
            "leaders",
        ),
        (
            "/app_state/gov/voting_params/voting_period",
            json!("432001s"),
            "voting_period",
        ),
    ] {
        let mut changed_value = value.clone();
        *changed_value.pointer_mut(pointer).unwrap() = changed;
        fs::write(&genesis, serde_json::to_vec(&changed_value).unwrap()).unwrap();
        let error = request.open().err().unwrap().to_string();
        assert!(
            error.contains("immutable native input drift") && error.contains(diagnostic),
            "{error}"
        );
        assert_eq!(
            fs::read(&marker).unwrap(),
            original_marker,
            "refusal cannot overwrite the binding"
        );
    }
    fs::write(&genesis, original).unwrap();
    assert!(request.open().unwrap().initialized());
    let output = request.data.join("config/genesis.json");
    let original = fs::read(&output).unwrap();
    fs::write(&output, b"{}").unwrap();
    assert!(request
        .open()
        .err()
        .unwrap()
        .to_string()
        .contains("generation has changed"));
    fs::write(output, original).unwrap();
    assert!(request.open().unwrap().initialized());
    let marker_text = String::from_utf8(original_marker).unwrap();
    assert!(!marker_text.contains(node_configuration()["receipt_seed"].as_str().unwrap()));
    assert!(!marker_text.contains(
        node_configuration()["validator_key"]["priv_key"]["value"]
            .as_str()
            .unwrap()
    ));
}

#[test]
fn TestRepairM_1321_InitializationPublicationRefusals() {
    let root = Directory::new();
    let request = request(&root.0);
    let mut guard = request.open().unwrap();
    publish_generation(&request);
    let source = request.source.join("app.toml");
    let original = fs::read(&source).unwrap();
    fs::write(&source, b"changed immutable application configuration").unwrap();
    let changed = ImmutableInputs::capture(&request.source, &request.options).unwrap();
    assert!(guard
        .complete(&changed)
        .unwrap_err()
        .to_string()
        .contains("app.toml"));
    assert!(!root
        .0
        .join(".data.native-initialization/initialized.json")
        .exists());
    fs::write(&source, original).unwrap();
    let inputs = ImmutableInputs::capture(&request.source, &request.options).unwrap();
    fs::write(request.data.join("config/genesis.json"), b"{}").unwrap();
    assert!(guard
        .complete(&inputs)
        .unwrap_err()
        .to_string()
        .contains("output differs"));
    drop(guard);
    assert!(request
        .open()
        .err()
        .unwrap()
        .to_string()
        .contains("unbound native generation"));
}

fn account_config(root: &Path, peer: &NativePeer, tenant: &str) -> PathBuf {
    write_private(
        &root.join("account.key"),
        node_configuration()["funding_scalar"]
            .as_str()
            .unwrap()
            .as_bytes(),
    );
    let config = root.join(format!("{tenant}.json"));
    fs::write(&config, serde_json::to_vec(&json!({"schema":"paxeer/native-tenant-account/1", "tenant":tenant,
        "key_file":root.join("account.key"), "chain_rpc":peer.url, "consensus_rpc":peer.url, "minimum_balance":"0x3e8"})).unwrap()).unwrap();
    config
}

#[test]
fn TestRepairM_1321_PreparationKeyArgv() {
    let mut one = [0; 32];
    one[31] = 1;
    let identity = account_identity(&one).unwrap();
    assert_eq!(
        identity.address,
        "0x7E5F4552091A69125d5DfCb7b8C2659029395Bdf"
    );
    assert_eq!(
        identity.public_key,
        "0x0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798"
    );
    one[31] = 2;
    assert_eq!(
        account_identity(&one).unwrap().address,
        "0x2B5AD5c4795c026514f8317c7a215E218DcCD6cF"
    );
    assert!(account_identity(&[0; 32]).is_err());
    assert!(account_identity(&[255; 32]).is_err());
    let root = Directory::new();
    let peer = NativePeer::start(transcript());
    let config = account_config(&root.0, &peer, "tenant-a");
    let mut output = Vec::new();
    paxeer_native::run_read_command(
        &["account".into(), config.display().to_string()],
        &mut output,
    )
    .unwrap();
    let funded: Value = serde_json::from_slice(&output).unwrap();
    assert_eq!(
        funded["identity"]["address"],
        node_configuration()["funding"]["sender"]
    );
    assert_eq!(funded["balance"], "0x1388");
    assert_eq!(funded["evidence"]["height"], 23869863);
    let secret = node_configuration()["funding_scalar"]
        .as_str()
        .unwrap()
        .to_owned();
    assert!(!String::from_utf8(output).unwrap().contains(&secret));
    assert!(!serde_json::to_string(&*peer.requests.lock().unwrap())
        .unwrap()
        .contains(&secret));
    let bootstrap = request(&root.0);
    let command = bootstrap.command(Path::new("/maintained/node/bootstrap.sh"));
    let argv: Vec<_> = command
        .get_args()
        .map(|arg| arg.to_str().unwrap())
        .collect();
    assert_eq!(
        argv,
        bootstrap
            .arguments
            .iter()
            .map(String::as_str)
            .collect::<Vec<_>>()
    );
    assert!(!argv
        .iter()
        .any(|argument| argument.contains(&secret) || *argument == "--private-key"));
    assert!(argv.contains(&root.0.join("funding.key").to_str().unwrap()));
    fs::set_permissions(
        root.0.join("account.key"),
        fs::Permissions::from_mode(0o644),
    )
    .unwrap();
    assert!(paxeer_native::run_read_command(
        &["account".into(), config.display().to_string()],
        &mut Vec::new()
    )
    .unwrap_err()
    .to_string()
    .contains("unsafe native input"));
    fs::remove_file(root.0.join("account.key")).unwrap();
    symlink(root.0.join("funding.key"), root.0.join("account.key")).unwrap();
    assert!(paxeer_native::run_read_command(
        &["account".into(), config.display().to_string()],
        &mut Vec::new()
    )
    .is_err());
}

#[test]
fn TestRepairM_1321_RelayRetirement13() {
    assert!(!Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("prepare-beta.py")
        .exists());
    let root = Directory::new();
    let peer = NativePeer::start(transcript());
    for tenant in ["tenant-a", "tenant-b"] {
        let config = account_config(&root.0, &peer, tenant);
        let mut output = Vec::new();
        paxeer_native::run_read_command(
            &["account".into(), config.display().to_string()],
            &mut output,
        )
        .unwrap();
        let value: Value = serde_json::from_slice(&output).unwrap();
        assert_eq!(value["tenant"], tenant);
        assert_eq!(value["evidence"]["chain_id"], 125);
        for retired in [
            "guarantors",
            "guarantor_set",
            "checkpoint",
            "settlement_contract",
            "genesis_request",
        ] {
            assert!(value.get(retired).is_none());
        }
        let mut invalid: Value = serde_json::from_slice(&fs::read(&config).unwrap()).unwrap();
        invalid["guarantor_set"] = json!([]);
        assert!(serde_json::from_value::<TenantAccount>(invalid).is_err());
    }
    let mut unfunded = transcript();
    unfunded["rpc_exchanges"]
        .as_array_mut()
        .unwrap()
        .iter_mut()
        .find(|v| v["method"] == "eth_getBalance")
        .unwrap()["result"] = json!("0x0");
    let peer = NativePeer::start(unfunded);
    let config = account_config(&root.0, &peer, "unfunded");
    let mut output = Vec::new();
    assert!(paxeer_native::run_read_command(
        &["account".into(), config.display().to_string()],
        &mut output
    )
    .unwrap_err()
    .to_string()
    .contains("committed-chain funding"));
    assert!(output.is_empty());
    for phase in [
        "prepare-beta",
        "guarantors",
        "genesis",
        "checkpoint",
        "write",
        "settlement",
    ] {
        assert!(paxeer_native::run_read_command(
            &[phase.into(), root.0.display().to_string()],
            &mut Vec::new()
        )
        .is_err());
    }
    assert!(!root.0.join("guarantors.json").exists());
    assert!(!root.0.join("genesis.request").exists());
}

#[test]
fn TestRepairM_1321_RelayRetirement14() {
    assert!(!Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("settlement-domain.py")
        .exists());
    let peer = NativePeer::start(transcript());
    let mut output = Vec::new();
    paxeer_native::run_read_command(
        &["evidence".into(), peer.url.clone(), peer.url.clone()],
        &mut output,
    )
    .unwrap();
    let evidence: NativeEvidence = serde_json::from_slice(&output).unwrap();
    assert_eq!(evidence.chain_id, 125);
    assert_eq!(evidence.height, 23869863);
    assert_eq!(evidence.consensus_chain_id, "hyperpax_125-1");
    assert_eq!(
        evidence.validators[0]["address"],
        node_configuration()["validator_key"]["address"]
    );
    let requests = peer.requests.lock().unwrap();
    assert_eq!(
        requests
            .iter()
            .map(|v| v["method"].as_str().unwrap())
            .collect::<Vec<_>>(),
        [
            "eth_chainId",
            "status",
            "commit",
            "eth_getBlockByNumber",
            "validators"
        ]
    );
    let validators = requests
        .iter()
        .find(|v| v["method"] == "validators")
        .unwrap();
    assert_eq!(
        validators["params"],
        json!({"height":"23869863", "page":"1", "per_page":"100"})
    );
    assert!(!String::from_utf8(output).unwrap().contains("guarantor"));
}

#[test]
fn TestRepairM_1321_CommittedEvidenceRefusals() {
    for (method, pointer, changed) in [
        ("eth_chainId", "", json!("0x1")),
        ("status", "/sync_info/catching_up", json!(true)),
        ("commit", "/canonical", json!(false)),
        ("commit", "/signed_header/commit/height", json!("23869864")),
        ("eth_getBlockByNumber", "/number", json!("0x16c39a8")),
        ("validators", "/block_height", json!("23869864")),
        ("validators", "/total", json!("0")),
        ("validators", "/validators/0/voting_power", json!("0")),
    ] {
        let mut fixture = transcript();
        let reply = &mut fixture["rpc_exchanges"]
            .as_array_mut()
            .unwrap()
            .iter_mut()
            .find(|v| v["method"] == method)
            .unwrap()["result"];
        if pointer.is_empty() {
            *reply = changed;
        } else {
            *reply.pointer_mut(pointer).unwrap() = changed;
        }
        let peer = NativePeer::start(fixture);
        assert!(
            NativeEvidence::fetch(&peer.url, &peer.url).is_err(),
            "{method}{pointer} must refuse"
        );
    }
}
