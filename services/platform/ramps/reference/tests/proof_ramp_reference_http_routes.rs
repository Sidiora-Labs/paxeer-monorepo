//! Proves: State/route/perform_work/rebalance -> serve/exchange over TLS HTTP -> ramp_request/decode_ramp_response and check-live.sh ramp.
//!
//! Repairs main.rs::verified_recovery: fresh, empty journals need no external
//! settlement recovery; failed RPC discovery used to halt their local readiness.
//! Repairs main.rs::rebalance error mapping: custody transport failures from
//! MutualTlsClient must report paxeer_unavailable, not provider_unavailable.
//! Repairs main.rs::decode_rebalance: tagged-enum buffering rejected even valid
//! u128 amounts; strict per-action decoding preserves the complete numeric range.
//! Repairs prepare-production.py decoding and next.config.mjs readiness caching:
//! HTTP 200 alone is insufficient and readiness must be fetched without caching.
//! No external settlement service is replaced. Valid POSTs prove the current
//! fail-closed behavior when an order is unknown or custody is unavailable; a
//! failed broadcast retains the real Journal::plan_paxeer intent across restart.
//! In services/platform/ramps/toolkit/src/clients.rs::verify_recovery_settlement,
//! retained journals require external chain evidence even for a planned transfer;
//! the expected restart result is 503 until that evidence can be verified.
//! The shared /readyz spelling in swagger_spot.json, next.config.mjs,
//! docker/kernel/init.sh and human-service/server/http.rs does not make those
//! independent services ramp consumers. Their distinct contracts stay explicit.
//! services/platform/ramps/reference/tests/work_routes.rs::work_route_refuses_native_payload_field
//! retains an expectation of 503 for a fresh journal; this proof instead asserts
//! 404/order_not_found for its unknown identifier after the bootstrap repair.

use std::fs;
use std::io::Read as _;
use std::net::{SocketAddr, TcpListener};
use std::os::unix::fs::PermissionsExt as _;
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

use serde_json::{json, Value};

struct Workspace(PathBuf);

impl Workspace {
    fn new() -> Self {
        let nanos = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .expect("clock")
            .as_nanos();
        let workspace = Self(
            std::env::temp_dir().join(format!("layerx-ramp-work-{}-{nanos}", std::process::id())),
        );
        fs::create_dir(&workspace.0).expect("create workspace");
        fs::set_permissions(&workspace.0, fs::Permissions::from_mode(0o700))
            .expect("private workspace");
        for args in [
            vec![
                "req",
                "-x509",
                "-newkey",
                "rsa:2048",
                "-nodes",
                "-days",
                "2",
                "-subj",
                "/CN=localhost",
                "-addext",
                "subjectAltName=DNS:localhost,IP:127.0.0.1",
                "-keyout",
                "key.pem",
                "-out",
                "cert.pem",
            ],
            vec![
                "pkcs12",
                "-export",
                "-inkey",
                "key.pem",
                "-in",
                "cert.pem",
                "-passout",
                "pass:ramp-test-password",
                "-out",
                "identity.p12",
            ],
            vec![
                "x509",
                "-in",
                "cert.pem",
                "-outform",
                "DER",
                "-out",
                "anchor.der",
            ],
        ] {
            let output = Command::new("openssl")
                .args(args)
                .current_dir(&workspace.0)
                .output()
                .expect("generate client TLS material");
            assert!(
                output.status.success(),
                "{}",
                String::from_utf8_lossy(&output.stderr)
            );
        }
        fs::set_permissions(
            workspace.path("identity.p12"),
            fs::Permissions::from_mode(0o600),
        )
        .expect("private TLS identity");
        workspace.private_file("password", b"ramp-test-password\n");
        workspace.private_file("service-token", b"work-route-service-token\n");
        workspace.private_file("operator-token", b"work-route-operator-token\n");
        workspace
    }

    fn path(&self, name: &str) -> PathBuf {
        self.0.join(name)
    }

    fn private_file(&self, name: &str, bytes: &[u8]) -> PathBuf {
        let path = self.path(name);
        fs::write(&path, bytes).expect("write protected file");
        fs::set_permissions(&path, fs::Permissions::from_mode(0o600)).expect("protect file");
        path
    }

    fn config(&self, address: SocketAddr) -> PathBuf {
        let p = |name: &str| self.path(name);
        let did = "did:layerx:ramp-work-test";
        let account = format!("agent:{did}:main");
        let config = json!({
            "listen": address.to_string(),
            "listener": "tls",
            "server_identity_pkcs12": p("identity.p12"),
            "server_identity_password_file": p("password"),
            "journal_path": p("journal.jsonl"),
            "worker_id": "ramp-work-worker",
            "lease_seconds": 60,
            "reconcile_seconds": 5,
            "operator": {
                "principal_id": "ramp-work-operator",
                "account": account,
                "signer_key_handle": "kms://ramp-work/layerx"
            },
            "quotes": [{
                "quote_id": "work-quote",
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
                "ca_pem": p("cert.pem"),
                "identity_pkcs12": p("identity.p12"),
                "identity_password_file": p("password"),
                "timeout_seconds": 8
            },
            "identity": {
                "endpoint": "https://identity.example.test",
                "service_token_file": p("service-token"),
                "audience": "layerx-ramp"
            },
            "compliance": {
                "endpoint": "https://compliance.example.test",
                "service_token_file": p("service-token"),
                "public_key": "22".repeat(32)
            },
            "provider": {
                "endpoint": "https://provider.example.test",
                "credential_file": p("service-token"),
                "settlement_path": "/layerx-ramp-v1/settlements",
                "status_path": "/layerx-ramp-v1/settlements"
            },
            "layerx": {
                "rpc_endpoint": "https://127.0.0.1:1",
                "rpc_trust_anchor_der": p("anchor.der"),
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
                "signer_token_file": p("service-token"),
                "actor_did": did
            },
            "paxeer": {
                "custody_endpoint": "https://localhost:1",
                "custody_credential_file": p("service-token"),
                "broadcast_path": "/layerx-paxeer-v1/rebalances",
                "status_path": "/layerx-paxeer-v1/rebalances",
                "operator_account": account,
                "wallet_address": "operator-wallet",
                "vault_id": "operator-vault",
                "signer_key_handle": "kms://ramp-work/paxeer",
                "poll_cadence_seconds": 5
            },
            "provider_callback_public_key": "44".repeat(32),
            "operator_control_token_file": p("operator-token")
        });
        self.private_file(
            "config.json",
            &serde_json::to_vec(&config).expect("encode config"),
        )
    }
}

impl Drop for Workspace {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

struct Ramp(Child);

impl Ramp {
    fn start(config: &Path, address: SocketAddr) -> Self {
        let mut ramp = Self(
            Command::new(env!("CARGO_BIN_EXE_layerx-reference-ramp"))
                .arg(config)
                .stdout(Stdio::null())
                .stderr(Stdio::piped())
                .spawn()
                .expect("start ramp"),
        );
        let deadline = Instant::now() + Duration::from_secs(30);
        loop {
            if let Some(status) = ramp.0.try_wait().expect("poll ramp") {
                let mut stderr = String::new();
                ramp.0
                    .stderr
                    .take()
                    .expect("ramp stderr")
                    .read_to_string(&mut stderr)
                    .expect("read startup error");
                panic!("ramp exited before listening: {status}: {stderr}");
            }
            let probe = Command::new("curl")
                .args(["-fsS", "--max-time", "1", "--cacert"])
                .arg(config.parent().expect("workspace").join("cert.pem"))
                .arg(format!("https://localhost:{}/livez", address.port()))
                .output()
                .expect("probe real TLS listener");
            if probe.status.success() {
                let body: Value = serde_json::from_slice(&probe.stdout).expect("liveness JSON");
                assert_eq!(body["live"], true);
                return ramp;
            }
            assert!(Instant::now() < deadline, "ramp did not begin listening");
            thread::sleep(Duration::from_millis(50));
        }
    }
}

impl Drop for Ramp {
    fn drop(&mut self) {
        let _ = self.0.kill();
        let _ = self.0.wait();
    }
}

fn repo() -> PathBuf {
    Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../../..")
        .canonicalize()
        .expect("repository root")
}

fn consumer(workspace: &Workspace, address: SocketAddr, assertions: &str) {
    let script = format!(
        r#"
import json, pathlib, runpy, ssl, sys, urllib.error, urllib.request
module = runpy.run_path(str(pathlib.Path(sys.argv[1]) / 'apps/web/e2e/prepare-production.py'))
context = ssl.create_default_context(cafile=sys.argv[2])
opener = urllib.request.build_opener(urllib.request.ProxyHandler({{}}), module['NoRedirect'](),
                                    urllib.request.HTTPSHandler(context=context))
service = sys.argv[3]
request = lambda path='/readyz', body=None, token=None: module['ramp_request'](opener, service, path, body, token)
decode = module['decode_ramp_response']
token = 'work-route-operator-token'
work_path = '/internal/v1/work'
rebalance_path = '/internal/v1/rebalances'
key = [7] * 32
lookup = rebalance_path + '/' + '07' * 32
{assertions}
"#
    );
    let output = Command::new("python3")
        .args(["-B", "-c", &script])
        .arg(repo())
        .arg(workspace.path("cert.pem"))
        .arg(format!("https://localhost:{}", address.port()))
        .output()
        .expect("run actual Python ramp consumer");
    assert!(
        output.status.success(),
        "consumer exit {:?}: {}\n{}",
        output.status.code(),
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
}

fn wallet_consumer(workspace: &Workspace, address: SocketAddr, ready: bool) {
    let output = Command::new("bash")
        .arg(repo().join("scripts/wallet/check-live.sh"))
        .arg("ramp")
        .env(
            "CHECK_LIVE_RAMP_BASE",
            format!("https://localhost:{}", address.port()),
        )
        .env("CHECK_LIVE_CA", workspace.path("cert.pem"))
        .env("CHECK_LIVE_TIMEOUT", "5")
        .output()
        .expect("run deployed ramp check");
    assert_eq!(
        output.status.success(),
        ready,
        "{}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    if ready {
        let stdout = String::from_utf8(output.stdout).expect("check output");
        for path in ["/readyz", "/internal/v1/work", "/internal/v1/rebalances"] {
            assert!(stdout.contains(&format!("pass ramp {path}")), "{stdout}");
        }
    }
}

#[test]
fn method() {
    let workspace = Workspace::new();
    let reservation = TcpListener::bind("127.0.0.1:0").expect("reserve listener");
    let address = reservation.local_addr().expect("listener address");
    let config = workspace.config(address);
    drop(reservation);
    let ramp = Ramp::start(&config, address);
    for _ in 0..2 {
        wallet_consumer(&workspace, address, true);
    }
    consumer(
        &workspace,
        address,
        r#"
for _ in range(2):
    status, ready = request()
    assert status == 200 and ready['ready'] is True
    assert ready['chain_id'] == 125 and ready['journal']['record_count'] == 0
    assert ready['journal']['recovery_required'] is False
    with opener.open(service + '/readyz', timeout=5) as response:
        assert response.headers['Cache-Control'] == 'no-store'

# Consume the producer's actual response, then corrupt it at the decoding boundary.
for field, invalid in [('ready', 'true'), ('journal', []), ('chain_id', True),
                       ('readiness_scope', 'external-settlement')]:
    malformed = dict(ready, **{field: invalid})
    try:
        decode(200, json.dumps(malformed).encode(), '/readyz')
    except ValueError:
        pass
    else:
        raise AssertionError('consumer accepted malformed readiness: ' + field)

work = {'order_digest': [9] * 32, 'action': 'submit_layerx'}
for _ in range(2):
    for path, body in [(work_path, work), (rebalance_path, {'action': 'reconcile', 'idempotency_key': key})]:
        assert request(path, body) == (401, {'error': 'operator_authentication_required'})
        assert request(path, body, 'wrong-operator') == (403, {'error': 'operator_authentication_refused'})
        assert request(path, {}, token) == (400, {'error': 'work_invalid' if path == work_path else 'rebalance_invalid'})
    assert request(work_path, work, token) == (404, {'error': 'order_not_found'})
    # No method confusion: a GET cannot invoke an operator POST action.
    assert request(work_path) == (404, {'error': 'not_found'})
    assert request(rebalance_path) == (404, {'error': 'not_found'})
    assert request(lookup, token=token) == (404, {'error': 'rebalance_not_found'})

# The real HTTP parser and handler, with malformed JSON rather than a replacement transport.
for path in (work_path, rebalance_path):
    malformed = urllib.request.Request(service + path, data=b'{',
        headers={'Authorization': 'Bearer ' + token, 'Content-Type': 'application/json'})
    try:
        opener.open(malformed, timeout=5)
    except urllib.error.HTTPError as response:
        with response:
            assert response.status == 400
            assert decode(response.status, response.read(), path) == {
                'error': 'work_invalid' if path == work_path else 'rebalance_invalid'}
    else:
        raise AssertionError('malformed request was accepted')

submit = {'action': 'submit', 'idempotency_key': key, 'asset': [1] * 32, 'amount': 100}
for attempt in range(2):
    # A real broadcast to an unavailable custody endpoint is refused, never fabricated.
    result = request(rebalance_path, submit, token)
    assert result == (503, {'error': 'paxeer_unavailable'}), result
    result = request(rebalance_path, {'action': 'reconcile', 'idempotency_key': key}, token)
    assert result == (503, {'error': 'paxeer_unavailable'}), result
    code, pending = request(lookup, token=token)
    assert code == 200 and pending['idempotency_key'] == key
    assert pending['asset'] == [1] * 32 and pending['amount'] == 100
    assert pending['status'] == 'submission_planned' and pending['operation_id'] is None
    assert pending['transaction_hash'] is None and pending['rung'] is None
    assert request()[1]['journal']['record_count'] == 1
    conflict = dict(submit, amount=101)
    assert request(rebalance_path, conflict, token) == (409, {'error': 'operation_conflict'})
    # Decode the full u128 range before checking the retained transfer binding.
    assert request(rebalance_path, dict(submit, amount=2**128 - 1), token) == (
        409, {'error': 'operation_conflict'})
    for invalid_amount in (-1, 1.5, '100', True, None, 2**128):
        assert request(rebalance_path, dict(submit, amount=invalid_amount), token) == (
            400, {'error': 'rebalance_invalid'})
    for invalid in (dict(submit, unexpected=True),
                    {'action': 'reconcile', 'idempotency_key': key, 'amount': 100},
                    {'action': 'unknown', 'idempotency_key': key}):
        assert request(rebalance_path, invalid, token) == (400, {'error': 'rebalance_invalid'})
    assert request(rebalance_path, {'action': 'poll', 'idempotency_key': key,
        'operation_id': 'unknown-operation', 'transaction_hash': '0x' + '08' * 32}, token) == (
        409, {'error': 'operation_conflict'})

for field, invalid in [('idempotency_key', [7]), ('status', None), ('settlement_domain', 'layerx')]:
    try:
        decode(200, json.dumps(dict(pending, **{field: invalid})).encode(), lookup)
    except ValueError:
        pass
    else:
        raise AssertionError('consumer accepted malformed rebalance: ' + field)
for raw in (b'{', b'[]', b'{"error":false}', b'{"error":""}'):
    try:
        decode(503, raw, rebalance_path)
    except ValueError:
        pass
    else:
        raise AssertionError('consumer accepted malformed refusal')

# These inventoried /readyz callers own distinct documents, not ramp schemas.
spot = json.loads((pathlib.Path(sys.argv[1]) / 'apps/perps/swagger_spot.json').read_text())
schema = spot['paths']['/readyz']['get']['responses']['200']['content']['application/json']['schema']
assert set(schema['required']) == {'status', 'service'}
assert not set(schema['required']).issubset(ready)
"#,
    );
    drop(ramp);
    // Observe the durable producer result only after the real process has stopped
    // and reopened its journal. Unverified recovery must keep writes unavailable.
    let reservation = TcpListener::bind("127.0.0.1:0").expect("reserve reopened listener");
    let address = reservation.local_addr().expect("reopened listener address");
    let config = workspace.config(address);
    drop(reservation);
    let _reopened = Ramp::start(&config, address);
    for _ in 0..2 {
        wallet_consumer(&workspace, address, false);
    }
    consumer(
        &workspace,
        address,
        r#"
for _ in range(2):
    try:
        request()
    except ValueError as refusal:
        assert str(refusal) == 'Ramp readiness refused'
    else:
        raise AssertionError('unverified recovery was declared ready')
    with_error = urllib.request.Request(service + '/readyz')
    try:
        opener.open(with_error, timeout=5)
    except urllib.error.HTTPError as response:
        with response:
            assert response.status == 503
            health = json.loads(response.read())
            assert health['ready'] is False and health['journal']['record_count'] == 1
            assert health['journal']['recovery_required'] is True
    else:
        raise AssertionError('recovery did not refuse readiness')
    code, retained = request(lookup, token=token)
    assert code == 200 and retained['idempotency_key'] == key
    assert retained['amount'] == 100 and retained['status'] == 'submission_planned'
    assert retained['transaction_hash'] is None
    assert request(work_path, {'order_digest': [9] * 32, 'action': 'submit_layerx'}, token) == (
        503, {'error': 'journal_recovery_required'})
    assert request(rebalance_path, {'action': 'submit', 'idempotency_key': key,
        'asset': [1] * 32, 'amount': 100}, token) == (503, {'error': 'journal_recovery_required'})
"#,
    );

    let node = Command::new("node")
        .args([
            "--input-type=module",
            "-e",
            r#"
import assert from 'node:assert/strict';
import { pathToFileURL } from 'node:url';
const { default: config } = await import(pathToFileURL(process.argv[1]));
delete process.env.LAYERX_HUMAN_SERVICE_URL;
assert.deepEqual(await config.rewrites(), []);
process.env.LAYERX_HUMAN_SERVICE_URL = 'https://human.example.test';
const rules = await config.rewrites();
assert.deepEqual(rules.find(rule => rule.source === '/readyz'), {
    source: '/readyz', destination: 'https://human.example.test/readyz'
});
assert(!rules.some(rule => rule.source.startsWith('/internal/')));
assert.deepEqual((await config.headers()).find(rule => rule.source === '/readyz').headers,
    [{key: 'Cache-Control', value: 'no-store'}]);
"#,
        ])
        .arg(repo().join("apps/web/next.config.mjs"))
        .output()
        .expect("run real Next configuration functions");
    assert!(
        node.status.success(),
        "{}",
        String::from_utf8_lossy(&node.stderr)
    );
}
