//! Proves: Journal + CosignDirectory + AttestationView::{deposit_answer, release_answer} -> attestation_api::serve over HTTP -> fetchDepositAttestation + fetchReleaseAttestation in apps/bridge/src/lib/bridge/attestation-api.ts.
//!
//! Repairs: that TypeScript module's readBytes32 accepted non-hex characters;
//! attestation_api.rs responses lacked Cache-Control: no-store; transport.rs
//! lacked the documented PaxeerEndpoint::call instance entry point.
//! The real journal is closed and replayed before the consumer runs. The real
//! TypeScript client uses native fetch, with no replacement of HTTP or decoding.
//! Node 22.15+ and the bridge application's installed dependencies are required.
//! Only negative decoding cases mutate a bundle emitted by the real producer.
//! The separate Paxeer JSON-RPC surface is exercised against the real HTTP
//! server: POST is refused, both directly and by rpc.rs::PaxeerRpc::call (which
//! currently maps HTTP faults to RpcFault::Unavailable). This asserts that
//! existing boundary; the attestation server is not a Paxeer JSON-RPC node.

use std::collections::BTreeMap;
use std::fs;
use std::io::{BufRead as _, BufReader, Read as _, Write as _};
use std::net::{SocketAddr, TcpListener};
use std::os::unix::fs::PermissionsExt as _;
use std::path::PathBuf;
use std::process::{Child, Command, Stdio};

#[path = "support/signer_binary.rs"]
mod signer_binary;

use layerx_bridge_relayer::signer::{Attestor, ATTEST_INBOUND_DOMAIN, ATTEST_OUTBOUND_DOMAIN};
use layerx_mirror::signer::{
    RemoteChainSigner, RemoteSignerConfig, SignerEndpoint, SigningAlgorithm,
};
use std::sync::OnceLock;
use std::sync::{Arc, RwLock};
use std::time::Duration;

use k256::ecdsa::SigningKey;
use layerx_bridge_relayer::attestation::{
    ethereum_address, InboundAttestation, OutboundAttestation,
};
use layerx_bridge_relayer::attestation_api::{
    deposit_answer, release_answer, serve, AttestationView,
};
use layerx_bridge_relayer::config::RelayerConfig;
use layerx_bridge_relayer::cosign::CosignDirectory;
use layerx_bridge_relayer::hex;
use layerx_bridge_relayer::journal::{Entry, Journal, Observation, Position};
use layerx_bridge_relayer::rpc::{JsonRpc, PaxeerRpc, RpcFault};
use layerx_bridge_relayer::transport::{PaxeerEndpoint, TransportFault};
use serde_json::{json, Value};

const ORIGIN: &str = "https://bridge.example:8443";
const EXAMPLE: &str = include_str!(concat!(
    env!("CARGO_MANIFEST_DIR"),
    "/../../../../services/bridge/deploy/relayer-solana.example.json"
));

struct Directory(PathBuf);

impl Directory {
    fn new() -> Self {
        let mut random = [0; 16];
        openssl::rand::rand_bytes(&mut random).expect("random directory suffix");
        let path = std::env::temp_dir().join(format!("proof-bridge-{}", hex::encode(&random)));
        fs::create_dir(&path).expect("exclusive proof directory");
        Self(path)
    }

    fn config(&self, enabled: bool) -> RelayerConfig {
        let mut config: Value = serde_json::from_str(EXAMPLE).expect("real deployment example");
        config["journal_path"] = json!(self.0.join("journal.jsonl"));
        config["cosign_directory"] = json!(self.0.join("cosign"));
        config["attestation_allowed_origins"] = json!([ORIGIN]);
        if enabled {
            config["attestation_listen"] = json!("127.0.0.1:0");
        } else {
            config
                .as_object_mut()
                .expect("config object")
                .remove("attestation_listen");
        }
        let path = self.0.join("relayer.json");
        fs::write(&path, serde_json::to_vec(&config).expect("encode config"))
            .expect("write config");
        RelayerConfig::load(&path).expect("production configuration loader")
    }
}

impl Drop for Directory {
    fn drop(&mut self) {
        fs::remove_dir_all(&self.0).expect("remove proof directory");
    }
}

fn server(config: &RelayerConfig, view: Arc<RwLock<AttestationView>>) -> Option<SocketAddr> {
    let address = config.attestation_listen?;
    let listener = TcpListener::bind(address).expect("bind real attestation server");
    let address = listener.local_addr().expect("assigned address");
    let origins = config.attestation_allowed_origins.clone();
    std::thread::spawn(move || serve(listener, view, origins).expect("real HTTP server"));
    Some(address)
}

// This is a Node host for the application's own TypeScript module, not a port
// of its parser. The resolve hook only supplies .ts for its extensionless local
// imports; Node strips types and loads the actual viem and Solana dependencies.
const CONSUMER: &str = r#"
import assert from 'node:assert/strict';
import { createInterface } from 'node:readline';
import { registerHooks } from 'node:module';
import { extname } from 'node:path';
import { pathToFileURL } from 'node:url';
registerHooks({ resolve(specifier, context, nextResolve) {
  const applicationImport = context.parentURL?.startsWith(
    pathToFileURL(process.env.BRIDGE_PROOF_SOURCE + '/').href,
  );
  if (applicationImport) {
    if (specifier === 'viem' || specifier.startsWith('@noble/curves/')) {
      context = { ...context, parentURL: pathToFileURL(process.env.BRIDGE_PROOF_DEPENDENCIES).href };
    }
    if ((specifier.startsWith('./') || specifier.startsWith('../')) && !extname(specifier)) {
      specifier += '.ts';
    }
  }
  return nextResolve(specifier, context);
} });
const lines = createInterface({ input: process.stdin })[Symbol.asyncIterator]();
let input = JSON.parse((await lines.next()).value);
const api = await import(pathToFileURL(process.env.BRIDGE_PROOF_CLIENT).href);
const deposit = {
  chainId: BigInt(input.chain), txHash: input.hash, logIndex: BigInt(input.index),
};
const release = {
  chainId: BigInt(input.chain), paxeerTxHash: input.hash, paxeerNonce: BigInt(input.index),
};
const depositPath = api.depositAttestationPath(deposit);
const releasePath = api.releaseAttestationPath(release);
assert.equal(depositPath, `/v1/deposit/${input.chain}/${input.hash}/${input.index}`);
assert.equal(releasePath, `/v1/release/${input.chain}/${input.hash}/${input.index}`);
function bundle(actual, expected, inbound) {
  assert.ok(actual);
  for (const [key, value] of Object.entries(expected)) {
    const got = actual[inbound && key === 'recipient' ? 'recipient32' : key];
    if (typeof got === 'string' && got.startsWith('0x')) {
      assert.equal(got.toLowerCase(), value, key);
    } else {
      assert.deepEqual(got, value, key);
    }
  }
  assert.equal(api.attestationPollInterval(actual, 1500),
    expected.status === 'ready' ? false : 1500);
}
for (let phase = 0; phase < 2; phase++) {
  for (let read = 0; read < 2; read++) {
    const actualDeposit = await api.fetchDepositAttestation(deposit);
    const actualRelease = await api.fetchReleaseAttestation(release);
    if (input.deposit === null) {
      assert.equal(actualDeposit, null);
      assert.equal(actualRelease, null);
      assert.equal(api.attestationPollInterval(actualDeposit, 1500), 1500);
    } else {
      bundle(actualDeposit, input.deposit, true);
      bundle(actualRelease, input.release, false);
    }
  }
  if (phase === 0 && input.next) {
    process.stdout.write('refresh\n');
    assert.equal((await lines.next()).value, 'continue');
    input = { ...input, ...input.next };
  } else {
    break;
  }
}
assert.equal(await api.fetchDepositAttestation({ ...deposit, logIndex: deposit.logIndex - 1n }), null);
assert.equal(await api.fetchReleaseAttestation({ ...release, paxeerNonce: release.paxeerNonce - 1n }), null);
for (const path of [depositPath, releasePath]) {
  const response = await fetch(process.env.NEXT_PUBLIC_BRIDGE_API_URL + path, {
    headers: { Origin: input.origin }, cache: 'no-store', signal: AbortSignal.timeout(5000),
  });
  assert.equal(response.status, input.deposit === null ? 404 : 200);
  assert.equal(response.headers.get('access-control-allow-origin'), input.origin);
  assert.equal(response.headers.get('cache-control'), 'no-store');
  assert.equal(response.headers.get('vary'), 'Origin');
  const raw = await response.json();
  if (input.deposit !== null) {
    const inbound = path === depositPath;
    const parse = inbound ? api.parseDepositAttestation : api.parseReleaseAttestation;
    const request = inbound ? deposit : release;
    assert.throws(() => parse(request, { ...raw, chainId: '0' }), api.AttestationApiError);
    for (const digest of ['0x' + 'g'.repeat(64), '0x' + '0'.repeat(63), '0x' + '0'.repeat(65)]) {
      assert.throws(() => parse(request, { ...raw, digest }), api.AttestationApiError);
    }
    if (inbound) {
      assert.throws(() => parse(request, { ...raw, recipient: '0x' + 'z'.repeat(64) }), api.AttestationApiError);
    }
    assert.throws(() => parse(request, { ...raw, signatures: ['0x00'] }), api.AttestationApiError);
  }
  const denied = await fetch(process.env.NEXT_PUBLIC_BRIDGE_API_URL + path, {
    headers: { Origin: input.origin + '.attacker' }, signal: AbortSignal.timeout(5000),
  });
  assert.equal(denied.headers.get('access-control-allow-origin'), null);
  await denied.arrayBuffer();
  const preflight = await fetch(process.env.NEXT_PUBLIC_BRIDGE_API_URL + path, {
    method: 'OPTIONS', headers: { Origin: input.origin, 'Access-Control-Request-Method': 'GET' },
    signal: AbortSignal.timeout(5000),
  });
  assert.equal(preflight.status, 204);
  assert.equal(preflight.headers.get('access-control-allow-origin'), input.origin);
  assert.equal(preflight.headers.get('access-control-allow-methods'), 'GET');
  const refused = await fetch(process.env.NEXT_PUBLIC_BRIDGE_API_URL + path, {
    method: 'OPTIONS', headers: { Origin: input.origin + '.attacker' }, signal: AbortSignal.timeout(5000),
  });
  assert.equal(refused.status, 403);
  await refused.arrayBuffer();
}
const malformed = await fetch(process.env.NEXT_PUBLIC_BRIDGE_API_URL + '/v1/deposit/no-chain/0x00/1', {
  signal: AbortSignal.timeout(5000),
});
assert.equal(malformed.status, 400);
await malformed.arrayBuffer();
process.stdout.write('consumer assertions passed\n');
process.stdin.destroy();
"#;

fn consumer_available() -> bool {
    match Command::new("npm").arg("--version").output() {
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            eprintln!("SKIP bridge TypeScript consumer: missing prerequisite npm");
            return false;
        }
        Err(error) => panic!("npm prerequisite check: {error}"),
        Ok(output) => assert!(
            output.status.success(),
            "npm prerequisite check: {}",
            String::from_utf8_lossy(&output.stderr)
        ),
    }
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .unwrap()
        .to_path_buf();
    let output = Command::new("node")
        .current_dir(root.join("apps/bridge"))
        .args([
            "--input-type=module",
            "--eval",
            "import { registerHooks } from 'node:module';",
        ])
        .output();
    match output {
        Err(error) if error.kind() == std::io::ErrorKind::NotFound => {
            eprintln!("SKIP bridge TypeScript consumer: missing prerequisite node (22.15+)");
            false
        }
        Err(error) => panic!("Node prerequisite check: {error}"),
        Ok(output) if !output.status.success() => {
            let stderr = String::from_utf8_lossy(&output.stderr);
            if stderr.contains("does not provide an export named 'registerHooks'") {
                eprintln!(
                    "SKIP bridge TypeScript consumer: missing prerequisite Node 22.15+: {stderr}"
                );
                false
            } else {
                panic!("TypeScript prerequisite check failed: {stderr}")
            }
        }
        Ok(_) => true,
    }
}

fn consumer_dependencies() -> &'static PathBuf {
    static DEPENDENCIES: OnceLock<PathBuf> = OnceLock::new();
    DEPENDENCIES.get_or_init(|| {
        let directory = std::env::temp_dir().join(format!(
            "layerx-bridge-consumer-dependencies-{}",
            std::process::id()
        ));
        fs::create_dir_all(&directory).expect("consumer dependency directory");
        let manifest: Value = serde_json::from_str(include_str!(concat!(
            env!("CARGO_MANIFEST_DIR"),
            "/../../../../apps/bridge/package.json"
        )))
        .expect("TypeScript consumer package manifest");
        let dependencies = json!({
            "viem": manifest["dependencies"]["viem"],
            "@noble/curves": manifest["devDependencies"]["@noble/curves"],
        });
        let package = directory.join("package.json");
        fs::write(
            &package,
            json!({"private": true, "dependencies": dependencies}).to_string(),
        )
        .expect("consumer dependency manifest");
        let output = Command::new("npm")
            .current_dir(&directory)
            .args([
                "install",
                "--ignore-scripts",
                "--no-audit",
                "--no-fund",
                "--no-package-lock",
            ])
            .output()
            .expect("install the real TypeScript consumer dependencies");
        assert!(
            output.status.success(),
            "TypeScript consumer dependency install failed with {}: {}\n{}",
            output.status,
            String::from_utf8_lossy(&output.stdout),
            String::from_utf8_lossy(&output.stderr)
        );
        package
    })
}

struct ProofSigner(Child);
impl Drop for ProofSigner {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

fn proof_signer(directory: &Directory) -> ProofSigner {
    let mut keys = Vec::new();
    for seed in 1..=3 {
        let path = directory.0.join(format!("attestor-{seed}.key"));
        fs::write(&path, hex::encode(&[seed; 32])).unwrap();
        fs::set_permissions(&path, fs::Permissions::from_mode(0o400)).unwrap();
        keys.push(json!({"handle": format!("attestor-{seed}"), "algorithm": "secp256k1", "key_file": path,
            "domains": [std::str::from_utf8(ATTEST_INBOUND_DOMAIN).unwrap(), std::str::from_utf8(ATTEST_OUTBOUND_DOMAIN).unwrap()]}));
    }
    fs::set_permissions(&directory.0, fs::Permissions::from_mode(0o700)).unwrap();
    let manifest = directory.0.join("keys.json");
    fs::write(&manifest, json!({"keys": keys}).to_string()).unwrap();
    let mut signer = ProofSigner(
        Command::new(signer_binary::signer_binary())
            .arg("bridge")
            .arg("--socket")
            .arg(directory.0.join("signer.sock"))
            .arg("--keys")
            .arg(manifest)
            .spawn()
            .expect("real bridge signer"),
    );
    let deadline = std::time::Instant::now() + Duration::from_secs(10);
    while !directory.0.join("signer.sock").exists() {
        assert!(
            signer.0.try_wait().unwrap().is_none(),
            "real signer exited during startup"
        );
        assert!(
            std::time::Instant::now() < deadline,
            "real signer startup timed out"
        );
        std::thread::sleep(Duration::from_millis(10));
    }
    signer
}

fn consume(
    address: SocketAddr,
    chain: u64,
    hash: &[u8; 32],
    index: u64,
    view: &AttestationView,
    transition: Option<(Arc<RwLock<AttestationView>>, &AttestationView)>,
) {
    let root = PathBuf::from(env!("CARGO_MANIFEST_DIR"))
        .ancestors()
        .nth(4)
        .expect("repository root")
        .to_path_buf();
    let mut child = Command::new("node")
        .args([
            "--experimental-strip-types",
            "--input-type=module",
            "--eval",
            CONSUMER,
        ])
        .env(
            "BRIDGE_PROOF_CLIENT",
            root.join("apps/bridge/src/lib/bridge/attestation-api.ts"),
        )
        .env(
            "BRIDGE_PROOF_SOURCE",
            root.join("apps/bridge/src/lib/bridge"),
        )
        .env("BRIDGE_PROOF_DEPENDENCIES", consumer_dependencies())
        .env("NEXT_PUBLIC_BRIDGE_API_URL", format!("http://{address}"))
        .stdin(Stdio::piped())
        .stdout(Stdio::piped())
        .stderr(Stdio::piped())
        .spawn()
        .expect("Node host for real TypeScript consumer");
    let mut input = json!({
        "chain": chain.to_string(), "hash": hex::prefixed(hash), "index": index.to_string(),
        "origin": ORIGIN,
        "deposit": deposit_answer(view, chain, hash, index),
        "release": release_answer(view, chain, hash, index),
    });
    if let Some((_, next)) = &transition {
        input["next"] = json!({
            "deposit": deposit_answer(next, chain, hash, index),
            "release": release_answer(next, chain, hash, index),
        });
    }
    let mut stdin = child.stdin.take().expect("consumer input");
    writeln!(stdin, "{input}").expect("send producer expectations");
    let mut stdout = BufReader::new(child.stdout.take().expect("consumer output"));
    if let Some((shared, next)) = transition {
        let mut signal = String::new();
        stdout.read_line(&mut signal).expect("read refresh signal");
        if signal != "refresh\n" {
            drop(stdin);
            let output = child.wait_with_output().expect("failed consumer output");
            panic!(
                "consumer must finish both initial HTTP reads; signal {signal:?}, status {}: {}",
                output.status,
                String::from_utf8_lossy(&output.stderr)
            );
        }
        *shared.write().expect("refresh live producer view") = next.clone();
        writeln!(stdin, "continue").expect("resume same consumer instance");
    }
    drop(stdin);
    let mut observed = String::new();
    stdout
        .read_to_string(&mut observed)
        .expect("read consumer assertions");
    let output = child.wait_with_output().expect("wait for real consumer");
    assert!(
        output.status.success(),
        "TypeScript consumer failed: {}\n{}",
        observed,
        String::from_utf8_lossy(&output.stderr)
    );
    assert_eq!(observed, "consumer assertions passed\n");
}

#[test]
fn unobserved() {
    if !consumer_available() {
        return;
    }
    let directory = Directory::new();
    let view = Arc::new(RwLock::new(AttestationView::default()));
    let config = directory.config(false);
    assert_eq!(config.attestation_listen, None);
    assert_eq!(server(&config, Arc::clone(&view)), None);
    let config = directory.config(true);
    let address = server(&config, Arc::clone(&view)).expect("enabled server");
    consume(
        address,
        u64::MAX,
        &[0xef; 32],
        u64::MAX - 1,
        &view.read().expect("view"),
        None,
    );
}

#[test]
fn runs() {
    if !consumer_available() {
        return;
    }
    let directory = Directory::new();
    let _signer = proof_signer(&directory);
    let mut config = directory.config(true);
    for invalid in [
        "*",
        "https://bridge.example/path",
        "https://bridge.example\r\nInjected: yes",
    ] {
        config.attestation_allowed_origins = vec![invalid.to_owned()];
        assert!(config
            .validate()
            .expect_err("invalid origin")
            .to_string()
            .contains("attestation_allowed_origins"));
    }
    config.attestation_allowed_origins = vec![ORIGIN.to_owned()];
    config.validate().expect("exact origin config");
    let inbound = InboundAttestation {
        chain_id: u64::MAX,
        vault: [0xcd; 20],
        tx_hash: [0xef; 32],
        log_index: u64::MAX - 1,
        recipient: [0xab; 32],
        asset: [0xbc; 20],
        amount: [0xff; 32],
    };
    let outbound = OutboundAttestation {
        chain_id: inbound.chain_id,
        vault: inbound.vault,
        paxeer_tx_hash: inbound.tx_hash,
        paxeer_nonce: inbound.log_index,
        recipient: [0xab; 20],
        asset: inbound.asset,
        amount: inbound.amount,
    };
    let position = Position {
        block_number: 100,
        block_hash: [0x12; 32],
    };
    let mut journal = Journal::open(&config.journal_path).expect("real journal");
    let cosign = CosignDirectory::new(config.cosign_directory.clone().expect("cosign directory"));
    let mut signatures = BTreeMap::new();
    for (observation, digest) in [
        (Observation::inbound(&inbound, position), inbound.digest()),
        (
            Observation::outbound(&outbound, position),
            outbound.digest(),
        ),
    ] {
        let item = observation.key();
        journal
            .append(&Entry::Observed {
                item: item.clone(),
                observation,
            })
            .expect("persist event");
        for seed in 1..=3 {
            let key = SigningKey::from_bytes((&[seed; 32]).into()).expect("real signing key");
            let attestor = Attestor::new(
                RemoteChainSigner::new(RemoteSignerConfig {
                    endpoint: SignerEndpoint::Uds {
                        socket: directory.0.join("signer.sock"),
                    },
                    algorithm: SigningAlgorithm::Secp256k1Recoverable,
                    key_handle: format!("attestor-{seed}"),
                    public_key: key
                        .verifying_key()
                        .to_encoded_point(true)
                        .as_bytes()
                        .to_vec(),
                    timeout: Duration::from_secs(5),
                })
                .expect("real signer client"),
            )
            .expect("real attestor");
            let signature = if matches!(observation, Observation::Inbound { .. }) {
                attestor.sign_inbound(&inbound)
            } else {
                attestor.sign_outbound(&outbound)
            }
            .expect("real bridge signer share");
            cosign
                .publish(&digest, &ethereum_address(key.verifying_key()), &signature)
                .expect("persist cosign");
            if seed == 1 {
                journal
                    .append(&Entry::Signed {
                        item: item.clone(),
                        signature,
                    })
                    .expect("persist signature");
            }
        }
        signatures.insert(digest, cosign.collect(&digest));
    }
    let persisted = journal.state().items.clone();
    drop(journal);
    drop(cosign);
    let reopened = Journal::open(&config.journal_path).expect("stop and reopen producer journal");
    assert_eq!(reopened.state().items, persisted);
    let cosign = CosignDirectory::new(config.cosign_directory.clone().expect("cosign directory"));
    for (digest, expected) in &signatures {
        assert_eq!(
            &cosign.collect(digest),
            expected,
            "cosign persists across reopen"
        );
        assert_eq!(expected.len(), 3);
    }
    let snapshot = AttestationView {
        items: reopened.state().items.clone(),
        signatures,
        thresholds: BTreeMap::from([(inbound.chain_id, 3)]),
    };
    let deposit = deposit_answer(
        &snapshot,
        inbound.chain_id,
        &inbound.tx_hash,
        inbound.log_index,
    )
    .expect("deposit");
    let release = release_answer(
        &snapshot,
        outbound.chain_id,
        &outbound.paxeer_tx_hash,
        outbound.paxeer_nonce,
    )
    .expect("release");
    for answer in [&deposit, &release] {
        assert_eq!(answer["status"], "ready");
        assert_eq!(
            answer["amount"],
            "115792089237316195423570985008687907853269984665640564039457584007913129639935"
        );
        assert_eq!(
            answer["signatures"]
                .as_array()
                .expect("signature array")
                .len(),
            3
        );
    }
    assert_eq!(deposit["digest"], hex::prefixed(&inbound.digest()));
    assert_eq!(release["digest"], hex::prefixed(&outbound.digest()));
    let shared = Arc::new(RwLock::new(snapshot.clone()));
    let address = server(&config, Arc::clone(&shared)).expect("configured socket");

    // Keep the same URL and identifiers, but withdraw peer signatures. Both
    // consecutive reads must now report attesting, rather than cached ready.
    let mut pending = snapshot.clone();
    pending.signatures.clear();
    assert_eq!(
        deposit_answer(
            &pending,
            inbound.chain_id,
            &inbound.tx_hash,
            inbound.log_index
        )
        .expect("deposit")["status"],
        "attesting"
    );
    consume(
        address,
        inbound.chain_id,
        &inbound.tx_hash,
        inbound.log_index,
        &snapshot,
        Some((Arc::clone(&shared), &pending)),
    );

    config.paxeer.endpoints[0].url = format!("http://{address}");
    config.paxeer.endpoints[0].trust_anchor_der = None;
    config.paxeer.endpoints[0].local_emulator = true;
    let endpoints = config
        .paxeer_endpoints()
        .expect("real endpoint config consumer");
    assert_eq!(endpoints[0].expected_chain_id, config.paxeer.chain_id);
    for _ in 0..2 {
        assert_eq!(
            endpoints[0].call("eth_blockNumber", &json!([])),
            Err(TransportFault::Http { status: 405 })
        );
    }
    let rpc = PaxeerRpc::new(endpoints).expect("real RPC consumer");
    assert_eq!(
        rpc.call("eth_blockNumber", json!([])),
        Err(RpcFault::Unavailable)
    );
    assert_eq!(
        PaxeerEndpoint::new(
            "http://rpc.example/rpc",
            Duration::from_secs(2),
            None,
            125,
            true
        ),
        Err(TransportFault::InsecureTransport)
    );
    config.paxeer.endpoints[0].local_emulator = false;
    assert!(
        config.paxeer_endpoints().is_err(),
        "plaintext must be explicitly local"
    );
}
