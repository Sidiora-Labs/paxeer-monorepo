//! Proves retirement of emulator-settlement-v2-contract through the real HTTP
//! listener and the current seller/buyer receipt path, including process reopen.

use std::error::Error;
use std::fs;
use std::io::{Read, Write};
use std::net::{TcpListener, TcpStream};
use std::path::{Path, PathBuf};
use std::process::{Child, Command, Stdio};
use std::thread;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};

const TEST_NAME: &str = "emulator_settlement_v2_retirement_and_current_authority_survive_restart";
const CHILD_ADDRESS: &str = "REPAIR_22_31_200_LISTEN";
type Result<T> = std::result::Result<T, Box<dyn Error>>;

struct Workspace(PathBuf);

impl Drop for Workspace {
    fn drop(&mut self) {
        let _ = fs::remove_dir_all(&self.0);
    }
}

struct Server {
    child: Child,
    address: String,
}

impl Drop for Server {
    fn drop(&mut self) {
        let _ = self.child.kill();
        let _ = self.child.wait();
    }
}

impl Server {
    fn start(directory: &Path, protocol: u16) -> Result<Self> {
        let listener = TcpListener::bind("127.0.0.1:0")?;
        let address = listener.local_addr()?.to_string();
        let seed = directory.join("sequencer.seed");
        fs::write(&seed, [0x42; 32])?;
        drop(listener);
        let child = Command::new(std::env::current_exe()?)
            .args(["--exact", TEST_NAME, "--nocapture"])
            .env(CHILD_ADDRESS, &address)
            .env("REPAIR_22_31_200_SEED", seed)
            .env("REPAIR_22_31_200_PROTOCOL", protocol.to_string())
            .env("RUST_MIN_STACK", "8388608")
            .stdin(Stdio::null())
            .spawn()?;
        let mut server = Self { child, address };
        let deadline = Instant::now() + Duration::from_secs(15);
        loop {
            if let Some(status) = server.child.try_wait()? {
                return Err(format!("emulator exited before readiness: {status}").into());
            }
            if let Ok((200, _)) = request(&server.address, "GET", "/healthz", "", &[]) {
                return Ok(server);
            }
            if Instant::now() >= deadline {
                return Err("emulator readiness timed out".into());
            }
            thread::sleep(Duration::from_millis(20));
        }
    }
}

fn request(
    address: &str,
    method: &str,
    path: &str,
    content_type: &str,
    body: &[u8],
) -> Result<(u16, Vec<u8>)> {
    let mut stream = TcpStream::connect(address)?;
    stream.set_read_timeout(Some(Duration::from_secs(10)))?;
    stream.set_write_timeout(Some(Duration::from_secs(10)))?;
    write!(stream, "{method} {path} HTTP/1.1\r\nHost: {address}\r\nContent-Type: {content_type}\r\nContent-Length: {}\r\nConnection: close\r\n\r\n", body.len())?;
    stream.write_all(body)?;
    let mut response = Vec::new();
    stream.read_to_end(&mut response)?;
    let split = response
        .windows(4)
        .position(|bytes| bytes == b"\r\n\r\n")
        .ok_or("missing HTTP header terminator")?;
    let headers = std::str::from_utf8(&response[..split])?;
    let status = headers
        .split_whitespace()
        .nth(1)
        .ok_or("missing HTTP status")?
        .parse()?;
    assert!(headers.contains("Cache-Control: no-store"));
    Ok((status, response[split + 4..].to_vec()))
}

fn assert_retired(status: u16, body: &[u8]) -> Result<()> {
    assert_eq!(status, 410, "{}", String::from_utf8_lossy(body));
    let value: serde_json::Value = serde_json::from_slice(body)?;
    assert_eq!(value["ok"], false);
    assert_eq!(value["error"]["code"], "settlement_authority_retired");
    assert_eq!(value["error"]["retry"], "never");
    let detail = value["error"]["detail"].as_str().ok_or("missing detail")?;
    assert!(detail.contains("ReceiptPayloadAuthority"));
    assert!(detail.contains("pax_sequencerReceipt"));
    assert!(detail.contains("block-commit"));
    assert!(value.get("result").is_none());
    assert!(!String::from_utf8_lossy(body).contains("authorized_batch"));
    Ok(())
}

fn middleware_path(root: &Path, directory: &Path, address: &str, phase: &str) -> Result<()> {
    let output = Command::new("node")
        .args(["--input-type=module", "--eval", MIDDLEWARE_PATH])
        .current_dir(root)
        .env("REPAIR_ROOT", root)
        .env("REPAIR_STATE", directory)
        .env("REPAIR_EMULATOR", format!("http://{address}"))
        .env("REPAIR_PHASE", phase)
        .output()?;
    assert!(
        output.status.success(),
        "real middleware path failed: {}\n{}",
        String::from_utf8_lossy(&output.stdout),
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(String::from_utf8_lossy(&output.stdout).contains("portable-v2-path-verified"));
    Ok(())
}

#[test]
fn emulator_settlement_v2_retirement_and_current_authority_survive_restart() -> Result<()> {
    if let Ok(address) = std::env::var(CHILD_ADDRESS) {
        layerx_platform_emulator::run([
            "up".to_owned(),
            "--listen".to_owned(),
            address,
            "--sequencer-seed-file".to_owned(),
            std::env::var("REPAIR_22_31_200_SEED")?,
            "--protocol-version".to_owned(),
            std::env::var("REPAIR_22_31_200_PROTOCOL")?,
        ])?;
        return Err("emulator unexpectedly stopped".into());
    }
    let root = Path::new(env!("CARGO_MANIFEST_DIR"))
        .join("../../..")
        .canonicalize()?;
    let nonce = SystemTime::now().duration_since(UNIX_EPOCH)?.as_nanos();
    let directory = Workspace(std::env::temp_dir().join(format!(
        "emulator-repair-22-31-200-{}-{nonce}",
        std::process::id()
    )));
    fs::create_dir(&directory.0)?;
    for protocol in [2, 3] {
        let state = directory.0.join(protocol.to_string());
        fs::create_dir(&state)?;
        let server = Server::start(&state, protocol)?;
        for (method, media_type, body) in [
            ("POST", "application/json", b"{}".as_slice()),
            ("POST", "application/octet-stream", b"invalid".as_slice()),
            ("GET", "", b"".as_slice()),
            ("PUT", "application/json", b"{}".as_slice()),
        ] {
            let (status, body) = request(&server.address, method, "/v1/settle", media_type, body)?;
            assert_retired(status, &body)?;
        }
        middleware_path(&root, &state, &server.address, "fresh")?;
        let prefund = serde_json::to_vec(&serde_json::json!({
            "did": "did:layerx:settlement-retirement",
            "public_key": "11".repeat(32), "amount_lo": 731
        }))?;
        let (status, body) = request(
            &server.address,
            "POST",
            "/__emulator/accounts/prefund",
            "application/json",
            &prefund,
        )?;
        assert_eq!(status, 200, "{}", String::from_utf8_lossy(&body));
        let (status, snapshot) = request(&server.address, "GET", "/__emulator/snapshot", "", &[])?;
        assert_eq!(status, 200);
        assert!(!snapshot.is_empty());
        let old_pid = server.child.id();
        drop(server);

        let server = Server::start(&state, protocol)?;
        assert_ne!(server.child.id(), old_pid);
        let (status, body) = request(
            &server.address,
            "PUT",
            "/__emulator/snapshot",
            "application/octet-stream",
            &snapshot,
        )?;
        assert_eq!(status, 200, "{}", String::from_utf8_lossy(&body));
        let (status, body) = request(&server.address, "GET", "/v1/state", "", &[])?;
        assert_eq!(status, 200);
        let state_body = String::from_utf8(body)?;
        assert!(state_body.contains("did:layerx:settlement-retirement"));
        assert!(state_body.contains("\"balance_lo\":731"));
        let produced = fs::read(state.join("seller-request.json"))?;
        let (status, body) = request(
            &server.address,
            "POST",
            "/v1/settle",
            "application/json",
            &produced,
        )?;
        assert_retired(status, &body)?;
        middleware_path(&root, &state, &server.address, "reopened")?;
    }
    Ok(())
}

const MIDDLEWARE_PATH: &str = r"
import assert from 'node:assert/strict';
import { registerHooks, stripTypeScriptTypes } from 'node:module';
import { existsSync, readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { pathToFileURL, fileURLToPath } from 'node:url';
import { join } from 'node:path';
import { once } from 'node:events';

// Load the real workspace sources without generating out-of-scope bundles.
const root = process.env.REPAIR_ROOT;
const state = process.env.REPAIR_STATE;
const emulator = process.env.REPAIR_EMULATOR;
const sdkParent = pathToFileURL(join(root, 'services/agent/sdk/typescript/src/index.ts')).href;
const packages = {
  '@sidiora/layerx-sdk': 'services/agent/sdk/typescript/src/index.ts',
  '@sidiora/layerx-seller-middleware': 'services/platform/middleware/seller/src/index.ts',
  '@sidiora/layerx-buyer-middleware': 'services/platform/middleware/buyer/src/index.ts',
};
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (packages[specifier]) return { url: pathToFileURL(join(root, packages[specifier])).href, shortCircuit: true };
    if (specifier.startsWith('@noble/') || specifier.startsWith('@scure/') || specifier === 'ws') {
      return nextResolve(specifier, { ...context, parentURL: sdkParent });
    }
    if (specifier.startsWith('.') && specifier.endsWith('.js') && context.parentURL?.startsWith('file:')) {
      const source = new URL(specifier.slice(0, -3) + '.ts', context.parentURL);
      if (existsSync(source)) return { url: source.href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (url.startsWith('file:') && url.endsWith('.ts')) {
      return { format: 'module', shortCircuit: true,
        source: stripTypeScriptTypes(readFileSync(fileURLToPath(url), 'utf8'), { mode: 'transform' }) };
    }
    return nextLoad(url, context);
  },
});
const sdk = await import('@sidiora/layerx-sdk');
const seller = await import('@sidiora/layerx-seller-middleware');
const { BuyerMiddleware, LayerXPaymentHttpTransport } = await import('@sidiora/layerx-buyer-middleware');
const { FileFulfillmentRepository, createPaidApiServer } = await import(
  pathToFileURL(join(root, 'services/platform/examples/paid-api/index.mjs')).href);
const bytes = value => Uint8Array.from(Buffer.from(value.slice(2), 'hex'));
const hex = value => `0x${Buffer.from(value).toString('hex')}`;
const document = JSON.parse(readFileSync(join(root, 'sdk/platform/conformance/fixtures/sequencer-receipt-v1.json'), 'utf8'));
assert.equal(document.provenance.generator, 'rpc/sequencer_receipt_vectors_test.go');
const vector = document.cases.find(entry => entry.id === 'V1-layerxledger-pay-exact');
assert.ok(vector);
const pin = { keyId: bytes(vector.pinnedKey.keyId), publicKey: bytes(vector.pinnedKey.publicKey),
  first_height: BigInt(vector.pinnedKey.firstHeight), last_height: BigInt(vector.pinnedKey.lastHeight) };
const verified = await sdk.verifySequencerReceipt(bytes(vector.signedReceipt), bytes(vector.rawTransaction), [pin]);
const proof = { format: 'layerx-receipt-proof-v2', verificationLevel: 'receipted',
  rawTransaction: vector.rawTransaction, signedReceipt: vector.signedReceipt,
  receiptDigest: hex(verified.receiptDigest),
  receiptKey: { keyId: vector.pinnedKey.keyId, publicKey: vector.pinnedKey.publicKey,
    firstHeight: `0x${pin.first_height.toString(16)}`, lastHeight: `0x${pin.last_height.toString(16)}` } };
const log = vector.receiptJson.logs[0];
const payment = seller.decodePaymentExactLog({ address: log.address, topics: log.topics.map(bytes), data: bytes(log.data) });
const requirements = { scheme: 'exact', network: 'eip155:125', amount: payment.amount.toString(),
  asset: hex(payment.assetId), payTo: hex(payment.to), maxTimeoutSeconds: 120,
  extra: { layerx: { commitment: 'receipted', account: 'agent:did:layerx:payee:main', currency: 'LXP',
    payer: hex(payment.from).slice(2), purposeHash: hex(payment.paymentReference).slice(2) } } };
const required = { x402Version: 2, resource: { url: 'https://merchant.example/paid' }, accepts: [requirements] };
const buyer = new BuyerMiddleware({
  client: new sdk.ProductionClient(new LayerXPaymentHttpTransport({ baseUrl: emulator,
    bearerToken: new sdk.SecretBytes(new TextEncoder().encode('local-retirement-test')) })),
  source: 'acct:buyer-retirement', receiptKeys: [pin], supported: [{ scheme: 'exact', network: 'eip155:125' }],
});
const offer = buyer.parseOffer(seller.encodePaymentRequiredHeader(required));
const payload = { x402Version: 2, resource: required.resource, payload: proof, accepted: offer.accepted, extensions: {} };
const paymentHeader = seller.encodePaymentPayloadHeader(payload);
assert.deepEqual(seller.decodePaymentPayloadHeader(paymentHeader).payload, proof);
const verification = await seller.verifyPaymentReceipt(proof, requirements, [pin]);
assert.equal(verification.rung, 'receipted');
assert.equal(verification.txHash, hex(sdk.keccak256(bytes(vector.rawTransaction))));

// This is a real HTTP authority transport: it forwards SellerMiddleware's
// actual request verbatim and propagates the terminal refusal to its consumer.
const retiredTransport = { async settle(request) {
  assert.deepEqual(request.payload.payload, proof);
  assert.match(request.requestDigest, /^[0-9a-f]{64}$/u);
  assert.match(request.idempotencyKey, /^[0-9a-f]{64}$/u);
  const serialized = JSON.stringify(request);
  writeFileSync(join(state, 'seller-request.json'), serialized);
  const response = await fetch(`${emulator}/v1/settle`, { method: 'POST',
    headers: { 'content-type': 'application/json' }, body: serialized });
  assert.equal(response.status, 410);
  const body = await response.json();
  assert.equal(body.ok, false);
  assert.equal(body.error.code, 'settlement_authority_retired');
  assert.equal(body.error.retry, 'never');
  assert.equal(body.result, undefined);
  return { kind: 'refused', reason: body.error.code };
} };
const retired = new seller.SellerMiddleware({ paymentRequired: required, receiptKeys: [pin],
  authority: retiredTransport, fulfillments: new FileFulfillmentRepository(join(state, 'retired-fulfillments')) });
const refusal = await retired.handle('public-paid-api', paymentHeader, async () => {
  assert.fail('retired authority must never release the resource');
});
assert.equal(refusal.kind, 'refused');
assert.equal(refusal.settlement.errorReason, 'settlement_authority_retired');
assert.equal(existsSync(join(state, 'retired-fulfillments')), false);
await assert.rejects(buyer.captureSettlement(refusal.headers[seller.PAYMENT_RESPONSE_HEADER],
  { ...proof, offer, txHash: verification.txHash }), error => error.code === 'payment-refused');

// Explicitly configure the current authority on a separate server; there is
// no fallback from the retired transport and no manufactured settlement reply.
const authority = new seller.ReceiptPayloadAuthority([pin]);
const current = new seller.SellerMiddleware({ paymentRequired: required, receiptKeys: [pin], authority,
  fulfillments: new FileFulfillmentRepository(join(state, 'current-fulfillments')) });
const api = createPaidApiServer(current, { resource: 'paid-content', producer: 'current-receipt-authority' });
api.listen(0, '127.0.0.1');
await once(api, 'listening');
try {
  const response = await fetch(`http://127.0.0.1:${api.address().port}/paid`, {
    headers: { [seller.PAYMENT_SIGNATURE_HEADER]: paymentHeader } });
  assert.equal(response.status, 200);
  assert.deepEqual(await response.json(), { resource: 'paid-content', producer: 'current-receipt-authority' });
  const captured = await buyer.captureSettlement(response.headers.get(seller.PAYMENT_RESPONSE_HEADER),
    { ...proof, offer, txHash: verification.txHash });
  assert.equal(captured.format, proof.format);
  assert.equal(captured.rawTransaction, proof.rawTransaction);
  assert.equal(captured.signedReceipt, proof.signedReceipt);
  assert.equal(captured.receiptDigest, proof.receiptDigest);
  assert.deepEqual(captured.receiptKey, proof.receiptKey);
  assert.equal(captured.verification.rung, 'receipted');
  assert.equal(captured.txHash, verification.txHash);
} finally {
  await new Promise((resolve, reject) => api.close(error => error ? reject(error) : resolve()));
}
const files = readdirSync(join(state, 'current-fulfillments'));
assert.equal(files.length, 1);
const storedText = readFileSync(join(state, 'current-fulfillments', files[0]), 'utf8');
const stored = JSON.parse(storedText);
assert.equal(stored.rawTransaction, proof.rawTransaction);
assert.equal(stored.signedReceipt, proof.signedReceipt);
assert.equal(stored.receiptDigest, proof.receiptDigest);
if (process.env.REPAIR_PHASE === 'fresh') writeFileSync(join(state, 'original-fulfillment.json'), storedText);
else assert.equal(storedText, readFileSync(join(state, 'original-fulfillment.json'), 'utf8'));

const request = JSON.parse(readFileSync(join(state, 'seller-request.json'), 'utf8'));
for (const changed of [
  { ...proof, receiptDigest: `0x${'ff'.repeat(32)}` },
  { ...proof, rawTransaction: `${proof.rawTransaction.slice(0, -2)}00` },
  { ...proof, receiptKey: { ...proof.receiptKey, publicKey: `0x${'ff'.repeat(32)}` } },
]) {
  await assert.rejects(authority.settle({ ...request, payload: { ...request.payload, payload: changed } }));
}
// Relabelling a signed receipt cannot create merged-network finality.
await assert.rejects(seller.verifyPaymentReceipt({ ...proof, verificationLevel: 'committed' },
  { ...requirements, extra: { layerx: { ...requirements.extra.layerx, commitment: 'committed' } } }, [pin]));
process.stdout.write('portable-v2-path-verified\n');
";
