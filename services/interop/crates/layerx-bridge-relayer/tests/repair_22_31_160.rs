//! Task 22.31.160: real journal/cosign -> production snapshot -> HTTP -> vault;
//! published configuration and Rust consumers -> production loaders/codecs/RPC.
use std::collections::BTreeMap;
use std::fs;
use std::io::{BufRead, BufReader, Read, Write};
use std::net::{SocketAddr, TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, ChildStdin, ChildStdout, Command, Stdio};
use std::sync::atomic::{AtomicBool, AtomicU64, Ordering};
use std::sync::{Arc, Mutex, MutexGuard, OnceLock, RwLock};
use std::thread::{self, JoinHandle};
use std::time::Duration;

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::abi::{self, decode_deposit_log};
use layerx_bridge_relayer::attestation::{
    assemble_signatures, ethereum_address, recover_signer, to_attestor_signature, uint256_from_u64,
    InboundAttestation, OutboundAttestation,
};
use layerx_bridge_relayer::attestation_api::{
    serve, AttestationView, AuthenticatedPolicy, PolicyDestination,
};
use layerx_bridge_relayer::config::RelayerConfig;
use layerx_bridge_relayer::cosign::CosignDirectory;
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{Entry, Journal, Observation, Position};
use layerx_bridge_relayer::receipt::{ReceiptError, SequencerReceipt};
use layerx_bridge_relayer::rpc::{JsonRpc, PaxeerRpc, RpcFault};
use layerx_bridge_relayer::transport::{PaxeerEndpoint, PaxeerTransport, TransportFault};
use layerx_proof::inclusion::SequencerAuthorization;
use layerx_proof::receipt::{ReceiptCheck, VerificationFailure};
use layerx_proof::sequencer_logs::{LogField, SequencerLogRefusal};
use openssl::asn1::Asn1Time;
use openssl::hash::MessageDigest;
use openssl::pkey::{PKey, Private};
use openssl::rsa::Rsa;
use openssl::ssl::{SslAcceptor, SslMethod};
use openssl::x509::extension::{BasicConstraints, SubjectAlternativeName};
use openssl::x509::{X509NameBuilder, X509};
use serde_json::{json, Value};

#[path = "support/evm_dependencies.rs"]
mod evm_dependencies;

const TIMEOUT: Duration = Duration::from_secs(5);
const CONTRACT: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../spec/layerx-merge/contracts/interop-bridge-relayer.md"
));
const TRANSPORT_CONTRACT: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../spec/layerx-merge/contracts/interop.md"
));

fn root() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..")
}

struct Directory(PathBuf);
impl Directory {
    fn new() -> Self {
        static NEXT: AtomicU64 = AtomicU64::new(0);
        let path = std::env::temp_dir().join(format!(
            "repair-22-31-160-{}-{}",
            std::process::id(),
            NEXT.fetch_add(1, Ordering::Relaxed)
        ));
        fs::create_dir(&path).or_panic("expected Ok value");
        Self(path)
    }
    fn journal(&self) -> Journal {
        Journal::open(&self.0.join("journal.jsonl")).or_panic("expected Ok value")
    }
    fn cosign(&self) -> CosignDirectory {
        CosignDirectory::new(self.0.join("cosign"))
    }
}
impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).or_panic("expected Ok value");
    }
}

struct VaultNode {
    child: Child,
    input: ChildStdin,
    output: BufReader<ChildStdout>,
    url: String,
}
impl VaultNode {
    fn new() -> Self {
        let mut child = Command::new("node")
            .arg(Path::new(env!("CARGO_MANIFEST_DIR")).join("src/repair_22_31_160_vault.cjs"))
            .arg(root())
            .env("NODE_PATH", evm_dependencies::node_modules())
            .stdin(Stdio::piped())
            .stdout(Stdio::piped())
            .stderr(Stdio::inherit())
            .spawn()
            .unwrap_or_else(|error| {
                panic!(
                    "{}: {error:?}",
                    "Node and the provisioned real EVM dependencies are required"
                )
            });
        let input = child.stdin.take().or_panic("expected Some value");
        let output = BufReader::new(child.stdout.take().or_panic("expected Some value"));
        let mut node = Self {
            child,
            input,
            output,
            url: String::new(),
        };
        node.url = String::from(node.read()["url"].as_str().or_panic("actual node URL"));
        node
    }
    fn read(&mut self) -> Value {
        let mut line = String::new();
        assert_ne!(
            self.output
                .read_line(&mut line)
                .or_panic("expected Ok value"),
            0,
            "real EVM host exited"
        );
        let answer: Value = serde_json::from_str(&line).or_panic("EVM host JSON");
        assert!(answer.get("error").is_none(), "{answer}");
        answer
    }
    fn command(&mut self, value: &Value) -> Value {
        writeln!(self.input, "{value}").or_panic("expected Ok value");
        self.input.flush().or_panic("expected Ok value");
        self.read()
    }
    fn endpoint(&self) -> PaxeerEndpoint {
        PaxeerEndpoint::new(&self.url, TIMEOUT, None, 125, true).or_panic("expected Ok value")
    }
    fn rpc(&self) -> PaxeerRpc {
        published_rpc(vec![self.endpoint()]).or_panic("expected Ok value")
    }
    fn deploy(&mut self, keys: &[u8], threshold: usize) -> (InboundAttestation, Position) {
        let deployed = self.command(&json!({"op": "deploy", "keys": keys, "threshold": threshold}));
        let vault = hex::fixed(deployed["address"].as_str().or_panic("expected Some value"))
            .or_panic("expected Ok value");
        let event = decode_deposit_log(&deployed["log"], &vault).or_panic("expected Ok value");
        (
            event.attestation(125, vault),
            Position {
                block_number: event.position.block_number,
                block_hash: event.position.block_hash,
            },
        )
    }
}
impl Drop for VaultNode {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}
fn node() -> MutexGuard<'static, VaultNode> {
    static NODE: OnceLock<Mutex<VaultNode>> = OnceLock::new();
    let mut node = NODE
        .get_or_init(|| Mutex::new(VaultNode::new()))
        .lock()
        .unwrap_or_else(std::sync::PoisonError::into_inner);
    if node
        .child
        .try_wait()
        .or_panic("EVM host child status")
        .is_some()
    {
        *node = VaultNode::new();
    }
    node
}

fn sign(digest: &[u8; 32], key: u8) -> ([u8; 20], [u8; 65]) {
    let key = SigningKey::from_bytes((&[key; 32]).into()).or_panic("expected Ok value");
    let (signature, recovery) = key
        .sign_prehash_recoverable(digest)
        .or_panic("expected Ok value");
    let mut recoverable = [0; 65];
    recoverable[..64].copy_from_slice(&signature.to_bytes());
    recoverable[64] = recovery.to_byte();
    (
        ethereum_address(key.verifying_key()),
        to_attestor_signature(recoverable).or_panic("expected Ok value"),
    )
}
fn observe_signed(journal: &mut Journal, observation: Observation, key: u8) -> [u8; 65] {
    let digest = observation
        .inbound_attestation()
        .map(|a| a.digest())
        .or_else(|| observation.outbound_attestation().map(|a| a.digest()))
        .or_panic("expected Some value");
    let item = observation.key();
    journal
        .append(&Entry::Observed {
            item: item.clone(),
            observation,
        })
        .or_panic("expected Ok value");
    let signature = sign(&digest, key).1;
    journal
        .append(&Entry::Signed { item, signature })
        .or_panic("expected Ok value");
    signature
}
fn deposit_path(attestation: &InboundAttestation) -> String {
    format!(
        "/v1/deposit/{}/{}/{}",
        attestation.chain_id,
        hex::prefixed(&attestation.tx_hash),
        attestation.log_index
    )
}
fn release_path(attestation: &OutboundAttestation) -> String {
    format!(
        "/v1/release/{}/{}/{}",
        attestation.chain_id,
        hex::prefixed(&attestation.paxeer_tx_hash),
        attestation.paxeer_nonce
    )
}
fn server(view: AttestationView) -> (SocketAddr, Arc<RwLock<AttestationView>>) {
    let listener = TcpListener::bind("127.0.0.1:0").or_panic("expected Ok value");
    let address = listener.local_addr().or_panic("expected Ok value");
    let shared = Arc::new(RwLock::new(view));
    let serving = Arc::clone(&shared);
    thread::spawn(move || {
        serve(&listener, &serving, &[]).or_panic("expected Ok value");
    });
    (address, shared)
}
fn get(address: SocketAddr, path: &str) -> Value {
    let mut stream = TcpStream::connect_timeout(&address, TIMEOUT).or_panic("expected Ok value");
    stream
        .set_read_timeout(Some(TIMEOUT))
        .or_panic("expected Ok value");
    write!(
        stream,
        "GET {path} HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n"
    )
    .or_panic("expected Ok value");
    let mut response = String::new();
    stream
        .read_to_string(&mut response)
        .or_panic("expected Ok value");
    let (headers, body) = response
        .split_once("\r\n\r\n")
        .or_panic("expected Some value");
    assert!(headers.starts_with("HTTP/1.1 200 OK"), "{response}");
    assert!(headers.contains("Cache-Control: no-store"));
    serde_json::from_str(body).or_panic("expected Ok value")
}
fn inbound_destination(a: &InboundAttestation) -> PolicyDestination {
    PolicyDestination::Inbound {
        chain_id: 125,
        bridge: abi::LAYERX_BRIDGE_PRECOMPILE,
        source_chain_id: a.chain_id,
        source_vault: a.vault,
    }
}
fn call_bytes(rpc: &PaxeerRpc, vault: &[u8; 20], selector: &[u8]) -> Vec<u8> {
    let value = rpc
        .call(
            "eth_call",
            json!([{
        "to": hex::prefixed(vault), "data": hex::prefixed(selector),
    }, "latest"]),
        )
        .or_panic("expected Ok value");
    hex::decode(value.as_str().or_panic("expected Some value")).or_panic("expected Ok value")
}
fn live_policy(
    rpc: &PaxeerRpc,
    vault: &[u8; 20],
    destination: PolicyDestination,
) -> AuthenticatedPolicy {
    let threshold = abi::decode_threshold(&call_bytes(rpc, vault, &abi::THRESHOLD_SELECTOR))
        .or_panic("expected Ok value");
    let attestors = abi::decode_address_list(&call_bytes(rpc, vault, &abi::ATTESTORS_SELECTOR))
        .or_panic("expected Ok value");
    AuthenticatedPolicy {
        destination,
        threshold: usize::try_from(threshold).or_panic("expected Ok value"),
        attestors,
    }
}
fn insert(
    policies: &mut BTreeMap<PolicyDestination, AuthenticatedPolicy>,
    policy: AuthenticatedPolicy,
) {
    policies.insert(policy.destination, policy);
}
fn outbound(deposit: &InboundAttestation, nonce: u64) -> OutboundAttestation {
    OutboundAttestation {
        chain_id: deposit.chain_id,
        vault: deposit.vault,
        paxeer_tx_hash: deposit.tx_hash,
        paxeer_nonce: nonce,
        recipient: [0x3a; 20],
        asset: deposit.asset,
        amount: uint256_from_u64(17),
    }
}

#[test]
fn startup_policy_reopens_journal_and_serves_attesting_until_threshold_three() {
    let mut node = node();
    let (deposit, position) = node.deploy(&[1, 2, 3], 3);
    let directory = Directory::new();
    let mut journal = directory.journal();
    observe_signed(&mut journal, Observation::inbound(&deposit, position), 1);
    drop(journal);
    let journal = directory.journal();
    assert!(journal
        .state()
        .items
        .values()
        .all(|item| item.attestation_policy.is_none()));
    let mut policies = BTreeMap::new();
    let (address, shared) = server(AttestationView::from_journal(&journal, None, &policies));
    let path = deposit_path(&deposit);
    let unknown = get(address, &path);
    assert_eq!(unknown["status"], "attesting");
    assert_eq!(unknown["signatures"], json!([]));
    let policy = live_policy(&node.rpc(), &deposit.vault, inbound_destination(&deposit));
    assert_eq!(policy.threshold, 3);
    insert(&mut policies, policy);
    *shared.write().or_panic("expected Ok value") =
        AttestationView::from_journal(&journal, None, &policies);
    let authenticated = get(address, &path);
    assert_eq!(authenticated["status"], "attesting");
    assert_eq!(
        authenticated["signatures"]
            .as_array()
            .or_panic("expected Some value")
            .len(),
        1
    );
    let cosign = directory.cosign();
    for key in [2, 3] {
        let (signer, signature) = sign(&deposit.digest(), key);
        cosign
            .publish(&deposit.digest(), &signer, &signature)
            .or_panic("expected Ok value");
    }
    *shared.write().or_panic("expected Ok value") =
        AttestationView::from_journal(&journal, Some(&cosign), &policies);
    assert_eq!(get(address, &path)["status"], "ready");
    // A withdrawn/unavailable destination policy must not retain a ready result.
    policies.clear();
    *shared.write().or_panic("expected Ok value") =
        AttestationView::from_journal(&journal, Some(&cosign), &policies);
    assert_eq!(get(address, &path)["status"], "attesting");
    assert_eq!(get(address, &path)["signatures"], json!([]));
}

#[test]
fn directional_policy_keeps_thresholds_in_both_orders_and_after_restart() {
    let mut node = node();
    let (destination_policy, _) = node.deploy(&[1, 2, 3], 3);
    let (deposit, position) = node.deploy(&[1], 1);
    let release = outbound(&deposit, 9);
    let inbound = live_policy(
        &node.rpc(),
        &destination_policy.vault,
        inbound_destination(&deposit),
    );
    let outbound = live_policy(
        &node.rpc(),
        &deposit.vault,
        PolicyDestination::EvmRelease {
            chain_id: deposit.chain_id,
            vault: deposit.vault,
        },
    );
    assert_eq!(inbound.threshold, 3);
    assert_eq!(outbound.threshold, 1);
    let directory = Directory::new();
    let mut journal = directory.journal();
    observe_signed(&mut journal, Observation::inbound(&deposit, position), 1);
    observe_signed(&mut journal, Observation::outbound(&release, position), 1);
    let (address, shared) = server(AttestationView::default());
    for reverse in [false, true] {
        let mut policies = BTreeMap::new();
        let order = if reverse {
            [&outbound, &inbound]
        } else {
            [&inbound, &outbound]
        };
        for policy in order {
            insert(&mut policies, policy.clone());
        }
        *shared.write().or_panic("expected Ok value") =
            AttestationView::from_journal(&journal, None, &policies);
        assert_eq!(get(address, &deposit_path(&deposit))["status"], "attesting");
        assert_eq!(get(address, &release_path(&release))["status"], "ready");
        assert!(
            shared
                .read()
                .or_panic("expected Ok value")
                .thresholds
                .is_empty(),
            "different directions have no shared threshold"
        );
        drop(journal);
        journal = directory.journal();
        *shared.write().or_panic("expected Ok value") =
            AttestationView::from_journal(&journal, None, &BTreeMap::new());
        assert_eq!(get(address, &deposit_path(&deposit))["status"], "attesting");
        assert_eq!(get(address, &release_path(&release))["status"], "attesting");
        *shared.write().or_panic("expected Ok value") =
            AttestationView::from_journal(&journal, None, &policies);
        assert_eq!(get(address, &deposit_path(&deposit))["status"], "attesting");
        assert_eq!(get(address, &release_path(&release))["status"], "ready");
    }
    // The same chain with another vault must not inherit this vault's policy.
    let mut mismatched = outbound.clone();
    mismatched.destination = PolicyDestination::EvmRelease {
        chain_id: 125,
        vault: [0xdd; 20],
    };
    let policies = BTreeMap::from([(mismatched.destination, mismatched)]);
    *shared.write().or_panic("expected Ok value") =
        AttestationView::from_journal(&journal, None, &policies);
    assert_eq!(get(address, &release_path(&release))["status"], "attesting");
}

#[test]
fn bundle_membership_rotation_filters_local_and_cosign_for_actual_vault() {
    let mut node = node();
    let (deposit, position) = node.deploy(&[1, 2, 3], 2);
    let release = outbound(&deposit, 21);
    let directory = Directory::new();
    let mut journal = directory.journal();
    let cosign = directory.cosign();
    let old_deposit = observe_signed(&mut journal, Observation::inbound(&deposit, position), 1);
    let old_release = observe_signed(&mut journal, Observation::outbound(&release, position), 1);
    for digest in [deposit.digest(), release.digest()] {
        for key in [1, 2, 3] {
            let (signer, signature) = sign(&digest, key);
            cosign
                .publish(&digest, &signer, &signature)
                .or_panic("expected Ok value");
        }
    }
    node.command(&json!({"op": "rotate", "vault": hex::prefixed(&deposit.vault), "keys": [3, 4], "threshold": 2}));
    let outbound_key = PolicyDestination::EvmRelease {
        chain_id: 125,
        vault: deposit.vault,
    };
    let release_policy = live_policy(&node.rpc(), &deposit.vault, outbound_key);
    let mut deposit_policy = release_policy.clone();
    deposit_policy.destination = inbound_destination(&deposit);
    let policies = BTreeMap::from([
        (release_policy.destination, release_policy.clone()),
        (deposit_policy.destination, deposit_policy),
    ]);
    drop(journal);
    let journal = directory.journal();
    let (address, shared) = server(AttestationView::from_journal(
        &journal,
        Some(&cosign),
        &policies,
    ));
    for (path, old) in [
        (deposit_path(&deposit), old_deposit),
        (release_path(&release), old_release),
    ] {
        let bundle = get(address, &path);
        assert_eq!(
            bundle["status"], "attesting",
            "removed local/cosigner cannot meet quorum"
        );
        assert_eq!(
            bundle["signatures"]
                .as_array()
                .or_panic("expected Some value")
                .len(),
            1
        );
        assert!(!bundle["signatures"]
            .as_array()
            .or_panic("expected Some value")
            .contains(&json!(hex::prefixed(&old))));
    }
    for digest in [deposit.digest(), release.digest()] {
        let (signer, signature) = sign(&digest, 4);
        cosign
            .publish(&digest, &signer, &signature)
            .or_panic("expected Ok value");
    }
    *shared.write().or_panic("expected Ok value") =
        AttestationView::from_journal(&journal, Some(&cosign), &policies);
    for (path, digest) in [
        (deposit_path(&deposit), deposit.digest()),
        (release_path(&release), release.digest()),
    ] {
        let bundle = get(address, &path);
        assert_eq!(bundle["status"], "ready");
        let signatures: Vec<[u8; 65]> = bundle["signatures"]
            .as_array()
            .or_panic("expected Some value")
            .iter()
            .map(|value| {
                hex::fixed(value.as_str().or_panic("expected Some value"))
                    .or_panic("expected Ok value")
            })
            .collect();
        assert_eq!(signatures.len(), 2);
        let candidates = [1, 2, 3, 4].map(|key| sign(&digest, key).1);
        assert_eq!(
            signatures,
            assemble_signatures(&digest, candidates, &release_policy.attestors, 2)
                .or_panic("expected Ok value")
        );
        let signers: Vec<_> = signatures
            .iter()
            .map(|sig| recover_signer(&digest, sig).or_panic("expected Ok value"))
            .collect();
        assert!(signers.windows(2).all(|pair| pair[0] < pair[1]));
        assert!(signers
            .iter()
            .all(|signer| release_policy.attestors.contains(signer)));
    }
    assert_release_uses_destination_vault(&mut node, address, &release);
}

fn assert_release_uses_destination_vault(
    node: &mut VaultNode,
    address: SocketAddr,
    release: &OutboundAttestation,
) {
    let url = format!("http://{address}{}", release_path(release));
    let mut poisoned: Vec<_> = [1, 3, 4]
        .into_iter()
        .map(|key| sign(&release.digest(), key))
        .collect();
    poisoned.sort_by_key(|(signer, _)| *signer);
    let poisoned: Vec<_> = poisoned.iter().map(|(_, sig)| hex::prefixed(sig)).collect();
    let refused = node.command(
        &json!({"op": "verify", "vault": hex::prefixed(&release.vault),
        "url": url, "signatures": poisoned}),
    );
    assert_eq!(refused["accepted"], false);
    assert_eq!(refused["refusal"], "UnknownSigner");
    let accepted =
        node.command(&json!({"op": "release", "vault": hex::prefixed(&release.vault), "url": url}));
    assert_eq!(
        accepted["accepted"], true,
        "HTTP bundle must execute in the actual destination vault"
    );
}

fn published_config() -> Value {
    let record = CONTRACT
        .split("## Task 22.31.160 repair record")
        .nth(1)
        .or_panic("expected Some value");
    let example = record
        .split("```json\n")
        .nth(1)
        .or_panic("expected Some value")
        .split("\n```")
        .next()
        .or_panic("expected Some value");
    serde_json::from_str(example).or_panic("expected Ok value")
}
fn load_config(
    directory: &Directory,
    value: &Value,
) -> Result<RelayerConfig, layerx_bridge_relayer::relayer::RelayerError> {
    let path = directory.0.join("relayer.json");
    fs::write(
        &path,
        serde_json::to_vec(value).or_panic("expected Ok value"),
    )
    .or_panic("expected Ok value");
    RelayerConfig::load(&path)
}
#[test]
fn receipt_key_contract_example_loads_strictly() {
    assert!(
        CONTRACT.contains("receipt_keys: [{sequencer_id, public_key, first_height, last_height}]")
    );
    assert!(!CONTRACT.contains("receipt_keys: [{key_id,"));
    let directory = Directory::new();
    let value = published_config();
    let config = load_config(&directory, &value).or_panic("expected Ok value");
    let keys = config.receipt_keys().or_panic("expected Ok value");
    assert_eq!(keys.len(), 1);
    assert_eq!(keys[0].sequencer_id(), [0x6b; 32]);
    assert_eq!(keys[0].first_batch_number(), 1000);
    assert_eq!(keys[0].last_batch_number(), 1999);
    let vectors: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))
    .or_panic("expected Ok value");
    let authentic = vectors["cases"]
        .as_array()
        .or_panic("expected Some value")
        .iter()
        .find(|case| case["id"] == "V1-erc20-approve")
        .or_panic("expected Some value");
    SequencerReceipt::decode(&authentic["receiptJson"])
        .or_panic("expected Ok value")
        .verify(&keys)
        .or_panic("expected Ok value");
    let mut alias = value.clone();
    let key = alias["paxeer"]["receipt_keys"][0]
        .as_object_mut()
        .or_panic("expected Some value");
    let identity = key.remove("sequencer_id").or_panic("expected Some value");
    key.insert("key_id".to_owned(), identity);
    assert!(load_config(&directory, &alias)
        .err()
        .or_panic("expected Err value")
        .to_string()
        .contains("unknown field"));
    let mut extra = value.clone();
    extra["paxeer"]["receipt_keys"][0]["ignored"] = json!(true);
    assert!(load_config(&directory, &extra)
        .err()
        .or_panic("expected Err value")
        .to_string()
        .contains("unknown field"));
    let mut inverted = value.clone();
    inverted["paxeer"]["receipt_keys"][0]["first_height"] = json!(2000);
    assert!(load_config(&directory, &inverted).is_err());
    let mut overlap = value;
    let duplicate = overlap["paxeer"]["receipt_keys"][0].clone();
    overlap["paxeer"]["receipt_keys"]
        .as_array_mut()
        .or_panic("expected Some value")
        .push(duplicate);
    assert!(load_config(&directory, &overlap).is_err());
}

// This consumer has exactly the corrected published parameter and result types.
fn published_rpc(endpoints: Vec<PaxeerEndpoint>) -> Result<PaxeerRpc, RpcFault> {
    PaxeerRpc::new(endpoints)
}
fn published_transport(url: String) -> Result<Value, TransportFault> {
    let endpoint = PaxeerEndpoint::new(url, Duration::from_secs(5), None, 125, true)?;
    PaxeerTransport::call(&endpoint, "eth_blockNumber", &json!([]))
}

fn certificate() -> (PKey<Private>, X509) {
    let key = PKey::from_rsa(Rsa::generate(2048).or_panic("expected Ok value"))
        .or_panic("expected Ok value");
    let mut name = X509NameBuilder::new().or_panic("expected Ok value");
    name.append_entry_by_text("CN", "localhost")
        .or_panic("expected Ok value");
    let name = name.build();
    let mut cert = X509::builder().or_panic("expected Ok value");
    cert.set_version(2).or_panic("expected Ok value");
    let serial = openssl::bn::BigNum::from_u32(1)
        .or_panic("expected Ok value")
        .to_asn1_integer()
        .or_panic("expected Ok value");
    cert.set_serial_number(&serial)
        .or_panic("expected Ok value");
    cert.set_subject_name(&name).or_panic("expected Ok value");
    cert.set_issuer_name(&name).or_panic("expected Ok value");
    cert.set_pubkey(&key).or_panic("expected Ok value");
    cert.set_not_before(&Asn1Time::days_from_now(0).or_panic("expected Ok value"))
        .or_panic("expected Ok value");
    cert.set_not_after(&Asn1Time::days_from_now(1).or_panic("expected Ok value"))
        .or_panic("expected Ok value");
    cert.append_extension(
        BasicConstraints::new()
            .critical()
            .ca()
            .build()
            .or_panic("expected Ok value"),
    )
    .or_panic("expected Ok value");
    let alternative = SubjectAlternativeName::new()
        .dns("localhost")
        .build(&cert.x509v3_context(None, None))
        .or_panic("expected Ok value");
    cert.append_extension(alternative)
        .or_panic("expected Ok value");
    cert.sign(&key, MessageDigest::sha256())
        .or_panic("expected Ok value");
    (key, cert.build())
}

// Real pinned TLS termination forwarding unchanged HTTP bytes to the real EVM.
struct TlsProxy {
    endpoint: PaxeerEndpoint,
    stop: Arc<AtomicBool>,
    worker: Option<JoinHandle<()>>,
}
impl TlsProxy {
    fn new(upstream: &str) -> Self {
        let upstream: SocketAddr = upstream
            .strip_prefix("http://")
            .or_panic("expected Some value")
            .parse()
            .or_panic("expected Ok value");
        let (key, certificate) = certificate();
        let mut acceptor =
            SslAcceptor::mozilla_intermediate(SslMethod::tls()).or_panic("expected Ok value");
        acceptor.set_private_key(&key).or_panic("expected Ok value");
        acceptor
            .set_certificate(&certificate)
            .or_panic("expected Ok value");
        acceptor.check_private_key().or_panic("expected Ok value");
        let acceptor = acceptor.build();
        let listener = TcpListener::bind("127.0.0.1:0").or_panic("expected Ok value");
        listener.set_nonblocking(true).or_panic("expected Ok value");
        let address = listener.local_addr().or_panic("expected Ok value");
        let endpoint = PaxeerEndpoint::new(
            format!("https://localhost:{}", address.port()),
            TIMEOUT,
            Some(certificate.to_der().or_panic("expected Ok value")),
            125,
            false,
        )
        .or_panic("expected Ok value");
        let stop = Arc::new(AtomicBool::new(false));
        let stopped = Arc::clone(&stop);
        let worker = thread::spawn(move || {
            while !stopped.load(Ordering::Acquire) {
                match listener.accept() {
                    Ok((stream, _)) => {
                        stream
                            .set_read_timeout(Some(TIMEOUT))
                            .or_panic("expected Ok value");
                        stream
                            .set_write_timeout(Some(TIMEOUT))
                            .or_panic("expected Ok value");
                        let Ok(mut tls) = acceptor.accept(stream) else {
                            continue;
                        };
                        let mut request = Vec::new();
                        let mut byte = [0; 1];
                        while !request.ends_with(b"\r\n\r\n") {
                            tls.read_exact(&mut byte).or_panic("expected Ok value");
                            request.push(byte[0]);
                            assert!(request.len() < 32768);
                        }
                        let length: usize = std::str::from_utf8(&request)
                            .or_panic("expected Ok value")
                            .lines()
                            .find_map(|line| line.strip_prefix("Content-Length: "))
                            .or_panic("expected Some value")
                            .parse()
                            .or_panic("expected Ok value");
                        let head = request.len();
                        request.resize(head + length, 0);
                        tls.read_exact(&mut request[head..])
                            .or_panic("expected Ok value");
                        let mut node = TcpStream::connect_timeout(&upstream, TIMEOUT)
                            .or_panic("expected Ok value");
                        node.set_read_timeout(Some(TIMEOUT))
                            .or_panic("expected Ok value");
                        node.write_all(&request).or_panic("expected Ok value");
                        let mut response = Vec::new();
                        node.read_to_end(&mut response)
                            .or_panic("expected Ok value");
                        tls.write_all(&response).or_panic("expected Ok value");
                        let _ = tls.shutdown();
                    }
                    Err(error) if error.kind() == std::io::ErrorKind::WouldBlock => {
                        thread::sleep(Duration::from_millis(2));
                    }
                    Err(error) => panic!("TLS listener: {error}"),
                }
            }
        });
        Self {
            endpoint,
            stop,
            worker: Some(worker),
        }
    }
}
impl Drop for TlsProxy {
    fn drop(&mut self) {
        self.stop.store(true, Ordering::Release);
        self.worker
            .take()
            .or_panic("expected Some value")
            .join()
            .or_panic("expected Ok value");
    }
}

#[test]
fn rpc_constructor_contract_consumes_paxeer_endpoints() {
    assert!(CONTRACT.contains(
        "pub fn new(endpoints: Vec&lt;PaxeerEndpoint&gt;) -&gt; Result&lt;Self, RpcFault&gt;"
    ));
    assert!(CONTRACT.contains(
        "fn published_rpc(endpoints: Vec<PaxeerEndpoint>) -> Result<PaxeerRpc, RpcFault>"
    ));
    let node = node();
    let proxy = TlsProxy::new(&node.url);
    let rpc = published_rpc(vec![proxy.endpoint.clone()]).or_panic("expected Ok value");
    assert_eq!(rpc.call("eth_chainId", json!([])), Ok(json!("0x7d")));
    assert!(rpc
        .call("eth_blockNumber", json!([]))
        .or_panic("expected Ok value")
        .as_str()
        .or_panic("expected Some value")
        .starts_with("0x"));
    assert!(matches!(
        published_rpc(Vec::new()),
        Err(RpcFault::Configuration)
    ));
    let mut mismatch = proxy.endpoint.clone();
    mismatch.expected_chain_id = 126;
    assert!(matches!(
        published_rpc(vec![proxy.endpoint.clone(), mismatch.clone()]),
        Err(RpcFault::Configuration)
    ));
    assert_eq!(
        published_rpc(vec![mismatch])
            .or_panic("expected Ok value")
            .call("eth_blockNumber", json!([])),
        Err(RpcFault::Unavailable)
    );
    let mut untrusted = proxy.endpoint.clone();
    untrusted.trust_anchor_der = Some(certificate().1.to_der().or_panic("expected Ok value"));
    assert!(matches!(
        PaxeerTransport::call(&untrusted, "eth_blockNumber", &json!([])),
        Err(TransportFault::Authentication { .. })
    ));
}

// Exhaustive public consumer: no invented timeout/malformed variants or erasure.
fn transport_case(error: &TransportFault) -> &'static str {
    match error {
        TransportFault::UnsupportedUrl => "UnsupportedUrl",
        TransportFault::InsecureTransport => "InsecureTransport",
        TransportFault::InvalidTrustAnchor => "InvalidTrustAnchor",
        TransportFault::Authentication { .. } => "Authentication",
        TransportFault::Connect { .. } => "Connect",
        TransportFault::Transport { .. } => "Transport",
        TransportFault::Http { .. } => "Http",
        TransportFault::ResponseTooLarge => "ResponseTooLarge",
        TransportFault::AmbiguousFraming => "AmbiguousFraming",
        TransportFault::MalformedResponse => "MalformedResponse",
        TransportFault::Rpc { .. } => "Rpc",
        TransportFault::ChainMismatch { .. } => "ChainMismatch",
        TransportFault::InconsistentObservation => "InconsistentObservation",
        TransportFault::UnexpectedValue { .. } => "UnexpectedValue",
    }
}
#[test]
fn paxeer_transport_contract_constructs_calls_and_matches_faults() {
    assert!(TRANSPORT_CONTRACT
        .contains("trust_anchor_der: Option&lt;Vec&lt;u8&gt;&gt;, expected_chain_id: u64"));
    assert!(TRANSPORT_CONTRACT.contains(
        "pub fn call(endpoint: &amp;PaxeerEndpoint, method: &amp;str, params: &amp;Value)"
    ));
    let node = node();
    let endpoint = node.endpoint();
    let PaxeerEndpoint {
        url,
        request_timeout,
        trust_anchor_der,
        expected_chain_id,
    } = endpoint.clone();
    assert_eq!(request_timeout, TIMEOUT);
    assert_eq!(trust_anchor_der, None);
    assert_eq!(expected_chain_id, 125);
    assert!(published_transport(url)
        .or_panic("expected Ok value")
        .as_str()
        .or_panic("expected Some value")
        .starts_with("0x"));
    assert_eq!(
        PaxeerTransport::call(&endpoint, "eth_chainId", &json!([])),
        Ok(json!("0x7d"))
    );
    let mut wrong = endpoint.clone();
    wrong.expected_chain_id = 126;
    let mismatch = PaxeerTransport::call(&wrong, "eth_blockNumber", &json!([]))
        .err()
        .or_panic("expected Err value");
    assert_eq!(
        mismatch,
        TransportFault::ChainMismatch {
            expected: 126,
            actual: 125
        }
    );
    assert_eq!(transport_case(&mismatch), "ChainMismatch");
    let refused = PaxeerTransport::call(&endpoint, "repair_22_31_160_missing_method", &json!([]))
        .err()
        .or_panic("expected Err value");
    assert!(
        matches!(&refused, TransportFault::Rpc { code, message } if *code < 0 && !message.is_empty())
    );
    assert_eq!(transport_case(&refused), "Rpc");
    let insecure = PaxeerEndpoint::new("http://192.0.2.1", TIMEOUT, None, 125, true)
        .err()
        .or_panic("expected Err value");
    assert_eq!(insecure, TransportFault::InsecureTransport);
    let invalid_anchor = PaxeerEndpoint::new(
        "https://localhost",
        TIMEOUT,
        Some(vec![1, 2, 3]),
        125,
        false,
    )
    .err()
    .or_panic("expected Err value");
    assert_eq!(invalid_anchor, TransportFault::InvalidTrustAnchor);
    for error in [&mismatch, &refused, &insecure, &invalid_anchor] {
        assert!(TRANSPORT_CONTRACT.contains(transport_case(error)));
    }
}

fn published_decode(value: &Value) -> Result<SequencerReceipt, ReceiptError> {
    SequencerReceipt::decode(value)
}
fn published_verify(
    receipt: &SequencerReceipt,
    keys: &[SequencerAuthorization],
) -> Result<(), ReceiptError> {
    receipt.verify(keys)
}
#[test]
fn receipt_error_contract_preserves_real_refusal_categories() {
    assert!(TRANSPORT_CONTRACT.contains("Result&lt;SequencerReceipt, ReceiptError&gt;"));
    assert!(TRANSPORT_CONTRACT.contains("Result&lt;(), ReceiptError&gt;"));
    assert!(TRANSPORT_CONTRACT.contains("`Verification(VerificationFailure)`"));
    assert!(TRANSPORT_CONTRACT.contains("`Mismatch(&'static str)`"));
    assert!(TRANSPORT_CONTRACT.contains("`Logs(SequencerLogRefusal)`"));
    assert_eq!(published_decode(&Value::Null), Err(ReceiptError::Malformed));
    let vectors: Value = serde_json::from_str(include_str!(concat!(
        env!("CARGO_MANIFEST_DIR"),
        "/../../../../sdk/platform/conformance/fixtures/sequencer-receipt-v1.json"
    )))
    .or_panic("expected Ok value");
    let mut categories = [0; 4];
    for case in vectors["cases"].as_array().or_panic("expected Some value") {
        let pin = &case["pinnedKey"];
        let key = SequencerAuthorization::new(
            hex::fixed(pin["keyId"].as_str().or_panic("expected Some value"))
                .or_panic("expected Ok value"),
            hex::fixed(pin["publicKey"].as_str().or_panic("expected Some value"))
                .or_panic("expected Ok value"),
            pin["firstHeight"].as_u64().or_panic("expected Some value"),
            pin["lastHeight"].as_u64().or_panic("expected Some value"),
        );
        let answer = published_decode(&case["receiptJson"]).or_panic("expected Ok value");
        let result = published_verify(&answer, &[key]);
        match case["expected"]["refusedAt"].as_str() {
            Some("R2") => {
                assert!(matches!(
                    result,
                    Err(ReceiptError::Verification(VerificationFailure {
                        check: ReceiptCheck::Decode | ReceiptCheck::CanonicalEncoding
                    }))
                ));
                categories[1] += 1;
            }
            Some("R3") => {
                assert_eq!(
                    result,
                    Err(ReceiptError::Verification(VerificationFailure {
                        check: ReceiptCheck::SequencerSignature
                    }))
                );
                categories[1] += 1;
            }
            Some("R5") => {
                assert_eq!(
                    result,
                    Err(ReceiptError::Logs(SequencerLogRefusal::Log {
                        index: 0,
                        field: LogField::Data
                    }))
                );
                categories[3] += 1;
            }
            None | Some("R4" | "R6") => {
                result.or_panic("expected Ok value");
                categories[0] += 1;
                let mut mismatch = case["receiptJson"].clone();
                mismatch["paymentDenom"] = json!("changed-denomination");
                assert_eq!(
                    published_verify(
                        &published_decode(&mismatch).or_panic("expected Ok value"),
                        &[key]
                    ),
                    Err(ReceiptError::Mismatch("paymentDenom"))
                );
                categories[2] += 1;
            }
            other => panic!("unclassified signed producer vector: {other:?}"),
        }
    }
    assert!(
        categories.iter().all(|count| *count > 0),
        "each real refusal category must execute: {categories:?}"
    );
}

trait OrPanic {
    type Output;

    fn or_panic(self, context: &str) -> Self::Output;
}

impl<T, E: std::fmt::Debug> OrPanic for std::result::Result<T, E> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|error| panic!("{context}: {error:?}"))
    }
}

impl<T> OrPanic for std::option::Option<T> {
    type Output = T;

    fn or_panic(self, context: &str) -> T {
        self.unwrap_or_else(|| panic!("{context}"))
    }
}
