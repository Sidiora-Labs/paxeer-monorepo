//! Proves: `ReceiptKeyList` / `paxeer::relay` / `chain_reads::dispatch` / `rpc_faucet::dispatch` -> layerx-gateway / JSON-RPC -> `SequencerEndpoint` and `PaymentRpc` and gatewayReceiptKeys.
//!
//! Repairs `SequencerEndpoint::receipt_keys`: the producer returns an object
//! containing `keys` with numeric ranges, not a bare array of EVM quantities.
//! Repairs `paxeer::relay`: malformed receipt selectors are invalid params even
//! when its node backend is absent.
//!
//! Current behavior outside touches: `src/chain_reads.rs::{px_getCommit`,
//! `px_getValidators,px_getSequenceBatch`} returns `consensus_unavailable` without
//! a consensus backend. `src/rpc.rs::dispatch` gates `px_requestFunds` on Identity
//! before parsing params, so malformed input also returns `kernel_unavailable`.
//! These branches must refuse explicitly; they must never invent chain or
//! funding evidence. The shared repair proof relays committed node transcripts,
//! verifies their signed receipt and Comet evidence with the Rust consumers,
//! and reopens the gateway on its persisted receipt-key configuration.
//! Producer build wiring and cross-language expansion are recorded in
//! `tests/repair_22_33_1335.kvx`.

#![allow(non_snake_case)]
use layerx_client::sequencer::{decode_receipt_keys, SequencerEndpoint, SequencerError};
use rustls::pki_types::{CertificateDer, PrivateKeyDer, PrivatePkcs8KeyDer};
use rustls::ServerConfig;
use serde_json::{json, Value};
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::sync::Arc;
use std::thread;
use std::time::{Duration, SystemTime, UNIX_EPOCH};

#[path = "support/repair_22_33_1335.rs"]
pub mod repair_proof;

#[test]
fn signed_node_receipt_and_committed_evidence_recover() {
    repair_proof::proof_success_and_recovery();
}

fn command(program: &str, arguments: &[&str]) {
    let output = Command::new(program).args(arguments).output().unwrap_or_else(|error| panic!("{program} must run: {error}"));
    assert!(output.status.success(), "{program} failed: {}", String::from_utf8_lossy(&output.stderr));
}

fn path(value: &Path) -> &str {
    value.to_str().unwrap_or_else(|| panic!("test path must be UTF-8"))
}

fn free_port() -> u16 {
    TcpListener::bind("127.0.0.1:0").and_then(|listener| listener.local_addr()).unwrap_or_else(|error| panic!("test port must be allocated: {error}")).port()
}

/// A real TLS Redis for the gateway's durable store, and the localhost
/// certificate every TLS peer of the test presents.
struct Fixture {
    redis: Child,
    directory: PathBuf,
    redis_port: u16,
}

impl Fixture {
    fn start() -> Self {
        let _ = rustls::crypto::ring::default_provider().install_default();
        let directory = std::env::temp_dir().join(format!("layerx-chain-reads-{}-{}", std::process::id(), SystemTime::now().duration_since(UNIX_EPOCH).unwrap_or_default().as_nanos()));
        fs::create_dir(&directory).unwrap_or_else(|error| panic!("test directory must be created: {error}"));
        let pem = directory.join("server.pem");
        let der = directory.join("server.der");
        let key = directory.join("server.key");
        let key_der = directory.join("server-key.der");
        command(
            "openssl",
            &["req", "-x509", "-newkey", "rsa:2048", "-nodes", "-keyout", path(&key), "-out", path(&pem), "-days", "1", "-subj", "/CN=localhost", "-addext", "subjectAltName=DNS:localhost"],
        );
        command("openssl", &["x509", "-in", path(&pem), "-outform", "DER", "-out", path(&der)]);
        command("openssl", &["pkcs8", "-topk8", "-nocrypt", "-in", path(&key), "-outform", "DER", "-out", path(&key_der)]);
        let redis_port = free_port();
        fs::write(directory.join("users.acl"), "user default off\nuser gateway on >gateway-secret ~* &* +@all\n").unwrap_or_else(|error| panic!("test Redis ACL must be written: {error}"));
        let config = directory.join("redis.conf");
        fs::write(
            &config,
            format!(
                "bind 127.0.0.1\nport 0\ntls-port {redis_port}\ntls-cert-file {}\ntls-key-file {}\ntls-ca-cert-file {}\ntls-auth-clients no\naclfile {}\nappendonly yes\nappendfsync always\ndir {}\nprotected-mode yes\n",
                path(&pem),
                path(&key),
                path(&pem),
                path(&directory.join("users.acl")),
                path(&directory),
            ),
        )
        .unwrap_or_else(|error| panic!("test Redis config must be written: {error}"));
        let redis = Command::new("redis-server").arg(&config).stdout(Stdio::null()).stderr(Stdio::null()).spawn().unwrap_or_else(|error| panic!("real Redis server must start: {error}"));
        let certificate = CertificateDer::from(fs::read(&der).unwrap_or_else(|error| panic!("certificate must be read: {error}")));
        let private_key = PrivateKeyDer::from(PrivatePkcs8KeyDer::from(fs::read(&key_der).unwrap_or_else(|error| panic!("key must be read: {error}"))));
        let _tls = Arc::new(ServerConfig::builder().with_no_client_auth().with_single_cert(vec![certificate], private_key).unwrap_or_else(|error| panic!("listener TLS must build: {error}")));
        let fixture = Self { redis, directory, redis_port };
        for _ in 0..100 {
            if TcpStream::connect(("127.0.0.1", redis_port)).is_ok() {
                return fixture;
            }
            thread::sleep(Duration::from_millis(20));
        }
        panic!("real Redis server did not become reachable")
    }

    fn secret(&self, name: &str, value: &str) -> String {
        let file = self.directory.join(name);
        fs::write(&file, value).unwrap_or_else(|error| panic!("{name} must be written: {error}"));
        path(&file).to_owned()
    }

    fn environment(&self, listen: u16) -> Vec<(String, String)> {
        let ca = path(&self.directory.join("server.der")).to_owned();
        vec![
            ("LAYERX_GATEWAY_LISTEN".to_owned(), format!("127.0.0.1:{listen}")),
            ("LAYERX_GATEWAY_LISTENER".to_owned(), "plain".to_owned()),
            ("LAYERX_GATEWAY_OUTBOUND_CA_DER".to_owned(), ca),
            ("LAYERX_GATEWAY_NETWORK_ID".to_owned(), "chain-reads".to_owned()),
            ("LAYERX_GATEWAY_LXP_WIRE_VERSION".to_owned(), layerx_wire::limits::STATE_COMMITMENT_PROTOCOL_VERSION.to_string()),
            ("LAYERX_GATEWAY_PROTOCOL_NETWORK_ID".to_owned(), "7".to_owned()),
            ("LAYERX_GATEWAY_REDIS_URL".to_owned(), format!("rediss://localhost:{}", self.redis_port)),
            ("LAYERX_GATEWAY_REDIS_USERNAME_FILE".to_owned(), self.secret("redis-username", "gateway")),
            ("LAYERX_GATEWAY_REDIS_PASSWORD_FILE".to_owned(), self.secret("redis-password", "gateway-secret")),
        ]
    }
}

impl Drop for Fixture {
    fn drop(&mut self) {
        let _ = self.redis.kill();
        let _ = self.redis.wait();
        let _ = fs::remove_dir_all(&self.directory);
    }
}

fn gateway_command(environment: &[(String, String)]) -> Command {
    let mut command = Command::new(env!("CARGO_BIN_EXE_layerx-gateway"));
    command.env_clear().env("PATH", std::env::var_os("PATH").unwrap_or_default()).envs(environment.iter().map(|(name, value)| (name, value)));
    command
}

struct Gateway {
    child: Child,
    port: u16,
}

impl Drop for Gateway {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

impl Gateway {
    fn start(fixture: &Fixture, keys: Option<&str>) -> Self {
        Self::start_on(fixture, keys, free_port())
    }

    fn start_on(fixture: &Fixture, keys: Option<&str>, port: u16) -> Self {
        let mut environment = fixture.environment(port);
        if let Some(keys) = keys {
            environment.push(("LAYERX_GATEWAY_RECEIPT_KEYS_FILE".into(), fixture.secret("receipt-keys.json", keys)));
        }
        let child = gateway_command(&environment).stdout(Stdio::null()).stderr(Stdio::inherit()).spawn().unwrap_or_else(|error| panic!("gateway must start: {error}"));
        let mut gateway = Self { child, port };
        for _ in 0..200 {
            if let Ok(Some(status)) = gateway.child.try_wait() {
                panic!("gateway refused start-up: {status}");
            }
            if gateway.exchange("GET", "/livez", b"").is_ok_and(|(status, _)| status == 200) {
                return gateway;
            }
            thread::sleep(Duration::from_millis(50));
        }
        panic!("gateway did not become live")
    }

    fn exchange(&self, method: &str, path: &str, body: &[u8]) -> Result<(u16, Vec<u8>), String> {
        let mut stream = TcpStream::connect(("127.0.0.1", self.port)).map_err(|error| error.to_string())?;
        stream.set_read_timeout(Some(Duration::from_secs(30))).map_err(|error| error.to_string())?;
        let mut request = format!("{method} {path} HTTP/1.1\r\nHost: localhost\r\nAccept: application/json\r\nContent-Type: application/json\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len()).into_bytes();
        request.extend_from_slice(body);
        stream.write_all(&request).map_err(|error| error.to_string())?;
        let mut answer = Vec::new();
        stream.read_to_end(&mut answer).map_err(|error| error.to_string())?;
        let header_end = answer.windows(4).position(|window| window == b"\r\n\r\n").ok_or_else(|| "answer has no header terminator".to_owned())?;
        let head = String::from_utf8_lossy(&answer[..header_end]).into_owned();
        let status = head.split_whitespace().nth(1).and_then(|value| value.parse::<u16>().ok()).ok_or_else(|| format!("answer status is invalid: {head}"))?;
        Ok((status, answer[header_end + 4..].to_vec()))
    }

    fn rpc(&self, method: &str, params: &Value) -> Value {
        let call = json!({"jsonrpc": "2.0", "id": "chain-read", "method": method, "params": params});
        assert_eq!(self.exchange("POST", "/rpc", call.to_string().as_bytes()).unwrap_or_else(|error| panic!("{}: {error:?}", "retired RPC refusal")).0, 404);
        let (status, body) = self.exchange("POST", "/", call.to_string().as_bytes()).unwrap_or_else(|error| panic!("RPC exchange: {error}"));
        assert_eq!(status, 200, "{}", String::from_utf8_lossy(&body));
        let answer: Value = serde_json::from_slice(&body).unwrap_or_else(|error| panic!("RPC response: {error}"));
        assert_eq!(answer["jsonrpc"], "2.0");
        assert_eq!(answer["id"], "chain-read");
        answer
    }
}

fn key_document(seed: u8) -> String {
    let signer = ed25519_dalek::SigningKey::from_bytes(&[seed; 32]);
    json!([{
        "key_id": layerx_platform_internal::base64::encode(&[seed; 32]),
        "public_key": layerx_platform_internal::base64::encode(&signer.verifying_key().to_bytes()),
        "first_height": 1, "last_height": 999
    }])
    .to_string()
}

fn consumer(gateway: &Gateway) -> SequencerEndpoint {
    SequencerEndpoint::new(&format!("http://127.0.0.1:{}/", gateway.port), &[]).unwrap_or_else(|error| panic!("{}: {error:?}", "real gateway endpoint"))
}

fn refusal(answer: &Value, code: i64, reason: &str) {
    assert_eq!(answer["error"]["code"], code, "{answer}");
    assert_eq!(answer["error"]["data"]["code"], reason, "{answer}");
    assert!(answer.get("result").is_none(), "{answer}");
}

#[test]
fn px_getReceiptKeys() {
    let fixture = Fixture::start();
    let gateway = Gateway::start(&fixture, Some(&key_document(7)));
    let endpoint = consumer(&gateway);
    for _ in 0..2 {
        let keys = endpoint.receipt_keys().unwrap_or_else(|error| panic!("{}: {error:?}", "gateway document must decode"));
        assert_eq!(keys.len(), 1);
        assert_eq!(keys[0].key_id, [7; 32]);
        assert_eq!(keys[0].public_key, ed25519_dalek::SigningKey::from_bytes(&[7; 32]).verifying_key().to_bytes());
        assert_eq!((keys[0].first_height, keys[0].last_height), (1, 999));
    }
    let document = gateway.rpc("px_getReceiptKeys", &json!([]))["result"].clone();
    let mut malformed = Vec::new();
    let mut duplicate = document.clone();
    duplicate["keys"].as_array_mut().unwrap_or_else(|| panic!("{}", "producer keys")).push(document["keys"][0].clone());
    malformed.push(duplicate);
    for (field, value) in [
        ("firstHeight", json!(0)),
        ("firstHeight", json!(1000)),
        ("firstHeight", json!("0x1")),
        ("lastHeight", json!(-1)),
        ("keyId", json!(format!("0x{}", "00".repeat(32)))),
        ("publicKey", json!("0x00")),
        ("extra", json!(true)),
    ] {
        let mut changed = document.clone();
        changed["keys"][0][field] = value;
        malformed.push(changed);
    }
    malformed.push(json!({"keys": []}));
    malformed.push(document["keys"].clone());
    for value in &malformed {
        assert_eq!(decode_receipt_keys(value), Err(SequencerError::Malformed), "{value}");
    }
    assert_eq!(gateway.rpc("px_getReceiptKeys", &json!(["unexpected"]))["error"]["code"], -32602);
    sdk_consumers(&gateway, &fixture.directory, &document);

    // Reopen the same durable Redis and producer key file with a rotated key.
    // Neither the endpoint client nor a second request can serve stale keys.
    let port = gateway.port;
    drop(gateway);
    let gateway = Gateway::start_on(&fixture, Some(&key_document(8)), port);
    for _ in 0..2 {
        assert_eq!(endpoint.receipt_keys().unwrap_or_else(|error| panic!("{}: {error:?}", "rotated producer"))[0].key_id, [8; 32]);
    }
    drop(gateway);
    let gateway = Gateway::start(&fixture, None);
    assert!(matches!(consumer(&gateway).receipt_keys(), Err(SequencerError::Rpc(error))
        if error.code == -32003 && error.data == Some(json!({"code": "receipt_keys_unavailable"}))));
}

#[test]
fn pax_sequencerReceipt() {
    let fixture = Fixture::start();
    let gateway = Gateway::start(&fixture, None);
    for _ in 0..2 {
        assert!(matches!(consumer(&gateway).sequencer_receipt(&[9; 32]),
            Err(SequencerError::Rpc(error)) if error.code == -32001
            && error.data == Some(json!({"code":"paxeer_rpc_not_configured"}))));
    }
    for params in [json!([]), json!(["0x01"]), json!([null]), json!({}), json!([format!("0x{}", "zz".repeat(32))]), json!(["0x01", "0x02"])] {
        assert_eq!(gateway.rpc("pax_sequencerReceipt", &params)["error"]["code"], -32602);
    }
}

#[test]
fn px_getCommit() {
    let fixture = Fixture::start();
    let gateway = Gateway::start(&fixture, None);
    for _ in 0..2 {
        assert!(matches!(consumer(&gateway).commit_material(5), Err(SequencerError::Rpc(error))
            if error.code == -32003 && error.data == Some(json!({"code":"consensus_unavailable"}))));
    }
    assert_eq!(gateway.rpc("px_getCommit", &json!(["0"]))["error"]["code"], -32602);
}

#[test]
fn px_getValidators() {
    let fixture = Fixture::start();
    let gateway = Gateway::start(&fixture, None);
    for _ in 0..2 {
        assert!(matches!(consumer(&gateway).validator_set(5), Err(SequencerError::Rpc(error))
            if error.code == -32003 && error.data == Some(json!({"code":"consensus_unavailable"}))));
    }
    assert_eq!(gateway.rpc("px_getValidators", &json!([5]))["error"]["code"], -32602);
}

#[test]
fn px_getSequenceBatch() {
    let fixture = Fixture::start();
    let gateway = Gateway::start(&fixture, None);
    for _ in 0..2 {
        refusal(&gateway.rpc("px_getSequenceBatch", &json!(["5"])), -32003, "consensus_unavailable");
    }
    assert_eq!(gateway.rpc("px_getSequenceBatch", &json!(["0x5"]))["error"]["code"], -32602);
}

#[test]
fn px_requestFunds() {
    let fixture = Fixture::start();
    let gateway = Gateway::start(&fixture, None);
    for params in [json!([format!("did:layerx:{}", "07".repeat(32)), "07".repeat(32)]), json!([])] {
        for _ in 0..2 {
            let answer = gateway.rpc("px_requestFunds", &params.clone());
            refusal(&answer, -32010, "kernel_unavailable");
            assert_eq!(answer["error"]["data"]["backend"], "identity");
            assert_eq!(answer["error"]["data"]["reason"], "not_configured");
        }
    }
}

fn sdk_consumers(gateway: &Gateway, directory: &Path, document: &Value) {
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("../../../..");
    let captured = directory.join("producer.json");
    fs::write(&captured, document.to_string()).unwrap_or_else(|error| panic!("{}: {error:?}", "retain real producer result"));
    let url = format!("http://127.0.0.1:{}/", gateway.port);
    let python = Command::new("python3")
        .env("PYTHONPATH", root.join("services/agent/sdk/python"))
        .args([
            "-c",
            r"
import json, sys
from layerx_sdk.x402_rpc import PaymentRpc, PaymentRpcError, decode_receipt_keys
rpc = PaymentRpc(sys.argv[1])
for _ in range(2):
    keys = rpc.receipt_keys()
    assert len(keys) == 1 and keys[0].key_id == bytes([7])*32
    assert keys[0].first_height == 1 and keys[0].last_height == 999
raw = json.load(open(sys.argv[2]))
assert decode_receipt_keys(raw) == keys
raw['keys'][0]['firstHeight'] = True
try: decode_receipt_keys(raw)
except ValueError: pass
else: raise AssertionError('malformed producer range accepted')
for method, params, code in [
    ('pax_sequencerReceipt', ['0x'+'09'*32], -32001),
    ('px_getCommit', ['5'], -32003), ('px_getValidators', ['5'], -32003),
    ('px_getSequenceBatch', ['5'], -32003), ('px_requestFunds', [], -32010)]:
    for _ in range(2):
        try: rpc.call(method, params)
        except PaymentRpcError as error: assert error.code == code
        else: raise AssertionError(method + ' manufactured evidence')
",
            &url,
            path(&captured),
        ])
        .output()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real Python SDK must execute"));
    assert!(python.status.success(), "Python consumer: {}", String::from_utf8_lossy(&python.stderr));
    let typescript = root.join("services/agent/sdk/typescript/src/ai_market.ts").canonicalize().unwrap_or_else(|error| panic!("{}: {error:?}", "SDK source"));
    let node = Command::new("node")
        .args([
            "--experimental-transform-types",
            "--input-type=module",
            "-e",
            r"
import {registerHooks} from 'node:module';
import {existsSync, readFileSync} from 'node:fs';
import {pathToFileURL, fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
registerHooks({resolve(specifier, context, next) {
  if (specifier.startsWith('.') && specifier.endsWith('.js') && context.parentURL?.endsWith('.ts')) {
    const candidate = new URL(specifier.slice(0,-3)+'.ts', context.parentURL);
    if (existsSync(fileURLToPath(candidate))) return next(candidate.href, context);
  }
  return next(specifier, context);
}});
const {gatewayReceiptKeys, decodeGatewayReceiptKeys} = await import(pathToFileURL(process.argv[1]).href);
let id = 0;
const operations = {async request({method, params}) {
  const sent = ++id;
  const reply = await fetch(process.argv[2], {method:'POST', headers:{'Content-Type':'application/json'},
    body: JSON.stringify({jsonrpc:'2.0', id:sent, method, params})});
  assert.equal(reply.status, 200);
  const body = await reply.json();
  assert.equal(body.id, sent);
  if (body.error) throw body.error;
  return body.result;
}};
for (let i=0; i<2; i++) {
  const keys = await gatewayReceiptKeys(operations);
  assert.equal(keys.length, 1);
  assert.deepEqual(keys[0].sequencerId, new Uint8Array(32).fill(7));
  assert.equal(keys[0].firstBatchNumber, 1n);
  assert.equal(keys[0].lastBatchNumber, 999n);
}
const raw = JSON.parse(readFileSync(process.argv[3], 'utf8'));
assert.equal(decodeGatewayReceiptKeys(raw).length, 1);
raw.keys[0].firstHeight = Number.MAX_SAFE_INTEGER+1;
assert.throws(() => decodeGatewayReceiptKeys(raw));
",
            path(&typescript),
            &url,
            path(&captured),
        ])
        .output()
        .unwrap_or_else(|error| panic!("{}: {error:?}", "real TypeScript SDK must execute"));
    assert!(node.status.success(), "TypeScript consumer: {}", String::from_utf8_lossy(&node.stderr));
}
