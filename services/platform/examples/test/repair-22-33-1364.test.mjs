import assert from "node:assert/strict";
import { once } from "node:events";
import { existsSync, mkdtempSync, readFileSync, rmSync } from "node:fs";
import { createServer } from "node:http";
import { registerHooks } from "node:module";
import { tmpdir } from "node:os";
import { join } from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";
import ts from "typescript";

// The gate builds the SDK and middleware workspaces. Load only the remaining
// committed wallet and integration sources in place, preserving package-local
// dependency resolution (including viem's transitive dependencies).
const root = new URL("../../../../", import.meta.url);
const packages = new Map([
  ["@sidiora/layerx-agent-integrations", "sdk/integrations/agents/src/index.ts"],
]);
const sourceRoots = ["sdk/wallet/src/", "sdk/integrations/agents/src/"].map((path) => new URL(path, root).href);
const isProductionSource = (url) => sourceRoots.some((prefix) => url?.startsWith(prefix));
registerHooks({
  resolve(specifier, context, nextResolve) {
    if (specifier === "viem" || specifier === "@supabase/supabase-js") return nextResolve(specifier, { ...context, parentURL: new URL("../agent-spend/services.mjs", import.meta.url).href });
    if (packages.has(specifier)) return { url: new URL(packages.get(specifier), root).href, shortCircuit: true };
    if (specifier.startsWith(".") && specifier.endsWith(".js") && isProductionSource(context.parentURL)) {
      const source = new URL(specifier.slice(0, -3) + ".ts", context.parentURL);
      if (existsSync(source)) return { url: source.href, shortCircuit: true };
    }
    return nextResolve(specifier, context);
  },
  load(url, context, nextLoad) {
    if (isProductionSource(url) && url.endsWith(".ts")) {
      return { format: "module", shortCircuit: true, source: ts.transpileModule(readFileSync(new URL(url), "utf8"), {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
        fileName: fileURLToPath(url),
      }).outputText };
    }
    return nextLoad(url, context);
  },
});

const { keccak256, serializeTransaction } = await import("viem");
const { PaxeerWallet } = await import("../../../../sdk/wallet/src/index.ts");
const { OwnerStore, createOwnerWalletHandoff, preparedTransactionReference } = await import("../agent-spend/services.mjs");
const { walletReviewTransport } = await import("../agent-spend/wallet-review-transport.mjs");
const { assertAgentSpendRuntime } = await import("../agent-spend/runtime.mjs");
const principal = "5d0c7a4e-8f5b-4c71-9a3e-2b6f1d9e4a10";
const transaction = {
  type: "eip1559", chainId: 125, nonce: 7, maxPriorityFeePerGas: 1n, maxFeePerGas: 2n,
  gas: 100000n, to: "0x000000000000000000000000000000000000101c", value: 0n,
  data: "0x01020304", accessList: [],
};
const unsigned = serializeTransaction(transaction);
const hash = keccak256(unsigned);
const prepared = {
  preparation_ref: hash.slice(2), signing_hash: hash.slice(2), chain_id: "125", nonce: "7",
  max_priority_fee_per_gas: "1", max_fee_per_gas: "2", gas_limit: "100000",
  to: transaction.to.slice(2), data: transaction.data.slice(2),
};
const token = [
  Buffer.from(JSON.stringify({ alg: "HS256", typ: "JWT" })).toString("base64url"),
  Buffer.from(JSON.stringify({ sub: principal, exp: 4102444800 })).toString("base64url"),
  Buffer.from("loopback-session-fixture").toString("base64url"),
].join(".");

async function openWalletPeer(t) {
  const exchanges = [];
  const failures = [];
  const reviews = new Map();
  const controls = { dropCreationResponse: false, lookupError: null, tamper: null };
  const server = createServer(async (req, res) => {
    try {
      const chunks = [];
      for await (const chunk of req) chunks.push(chunk);
      const body = chunks.length ? JSON.parse(Buffer.concat(chunks)) : null;
      const entry = { method: req.method, path: req.url, authorization: req.headers.authorization, body,
        idempotencyKey: req.headers["idempotency-key"] };
      exchanges.push(entry);
      assert.equal(entry.authorization, `Bearer ${token}`);
      const reply = (status, artifact) => {
        entry.response = artifact;
        entry.status = status;
        res.writeHead(status, { "content-type": "application/json" }).end(JSON.stringify(artifact));
      };
      if (req.url === "/auth/v1/user") {
        assert.equal(req.method, "GET");
        return reply(200, { id: principal, aud: "authenticated", role: "authenticated",
          app_metadata: {}, user_metadata: {}, created_at: "1970-01-01T00:00:00.000Z" });
      }
      if (req.url === `/v1/wallet/evm/reviews/by-signing-hash/${hash}`) {
        assert.equal(req.method, "GET");
        assert.equal(body, null);
        assert.equal(entry.idempotencyKey, hash);
        if (controls.lookupError) return reply(controls.lookupError.status, { error: controls.lookupError.error });
        const retained = reviews.get(hash);
        return retained ? reply(200, { ...retained, ...controls.tamper }) : reply(404, { error: "evm_review_not_found" });
      }
      if (req.url === "/v1/wallet/evm/review") {
        assert.equal(req.method, "POST");
        assert.equal(req.headers["content-type"], "application/json");
        assert.deepEqual(body, { transaction: unsigned });
        assert.equal(entry.idempotencyKey, hash);
        assert.equal(keccak256(body.transaction), entry.idempotencyKey);
        if (!reviews.has(hash)) reviews.set(hash, {
          id: "b1f79a7f-ffbc-4d19-b235-3ccb7e4428ee", state: "reviewed",
          to: transaction.to, value: "0", data: transaction.data, signing_hash: hash,
          transaction: body.transaction, principal, chain_id: 125, method: "contract_call",
          destination: transaction.to, key_id: "retained-owner-key", public_key: "retained-public-key",
          epoch: 1, session_id: "retained-signing-session", expires_at: "4102444800",
          participants: ["one", "two", "three"],
        });
        if (controls.dropCreationResponse) { controls.dropCreationResponse = false; res.destroy(); return; }
        return reply(200, reviews.get(hash));
      }
      if (req.url === "/v1/wallet/evm/approvals/b1f79a7f-ffbc-4d19-b235-3ccb7e4428ee") {
        assert.equal(req.method, "GET");
        assert.equal(body, null);
        return reply(200, { ...reviews.get(hash), ...controls.tamper });
      }
      throw new Error(`unexpected request: ${req.method} ${req.url}`);
    } catch (error) {
      failures.push(error);
      res.writeHead(500).end(JSON.stringify({ error: error.message }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(async () => {
    await new Promise((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
    assert.deepEqual(failures, []);
  });
  return { url: `http://127.0.0.1:${server.address().port}`, exchanges, reviews, controls };
}

async function openWalkthrough(t, peer) {
  const directory = mkdtempSync(join(tmpdir(), "agent-spend-repair-"));
  const path = join(directory, "owner.sqlite");
  let component;
  const close = async () => {
    if (!component) return;
    const closing = component;
    component = undefined;
    await closing.wallet.supabase.auth.stopAutoRefresh();
    closing.store.close();
  };
  t.after(async () => {
    await close();
    rmSync(directory, { recursive: true, force: true });
  });
  const reopen = async () => {
    await close();
    const store = new OwnerStore(path, { principal, apiUrl: peer.url, tenant: "fixture-tenant" });
    const wallet = new PaxeerWallet({ apiUrl: peer.url, supabaseUrl: peer.url,
      supabaseAnonKey: "loopback-anon-key", evmReview: { storage: store }, fetch: walletReviewTransport(peer.url) });
    component = { store, wallet, handoff: createOwnerWalletHandoff(wallet, store) };
    const { data, error } = await wallet.supabase.auth.setSession({ access_token: token, refresh_token: "loopback-refresh" });
    assert.equal(error, null);
    assert.equal(data.session.user.id, principal);
    return component;
  };
  return { reopen, close, path };
}

async function agentSpendDependencyClosureLoadsRealTransport(t) {
  const manifest = JSON.parse(readFileSync(new URL("../agent-spend/package.json", import.meta.url)));
  assert.equal(manifest.dependencies["@sidiora/layerx-agent-integrations"], "0.1.0");
  for (const name of ["@sidiora/layerx-agent-integrations", "@sidiora/layerx-agent-middleware", "@sidiora/layerx-sdk"]) {
    const path = {
      "@sidiora/layerx-agent-integrations": "sdk/integrations/agents/package.json",
      "@sidiora/layerx-agent-middleware": "services/platform/middleware/agent/package.json",
      "@sidiora/layerx-sdk": "services/agent/sdk/typescript/package.json",
    }[name];
    const dependency = JSON.parse(readFileSync(new URL(path, root)));
    assert.equal(manifest.dependencies[name], dependency.version);
    assert.ok(await import(name));
  }
  const peer = await openWalletPeer(t);
  const walkthrough = await openWalkthrough(t, peer);
  const { handoff } = await walkthrough.reopen();
  const artifact = await handoff.review(prepared);
  assert.deepEqual(artifact, peer.reviews.get(hash));
  assert.deepEqual(peer.exchanges.filter((entry) => entry.path.startsWith("/v1/")).map(({ method, path, body, idempotencyKey, status }) => ({ method, path, body, idempotencyKey, status })), [
    { method: "GET", path: `/v1/wallet/evm/reviews/by-signing-hash/${hash}`, body: null, idempotencyKey: hash, status: 404 },
    { method: "POST", path: "/v1/wallet/evm/review", body: { transaction: unsigned }, idempotencyKey: hash, status: 200 },
  ]);
  assert.deepEqual(await handoff.status(prepared.preparation_ref, artifact.id), artifact);
}

function agentSpendNodeMinimumMatchesSetupAndQualification() {
  const manifest = JSON.parse(readFileSync(new URL("../agent-spend/package.json", import.meta.url)));
  const integrations = JSON.parse(readFileSync(new URL("sdk/integrations/agents/package.json", root)));
  assert.equal(manifest.engines.node, ">=22.18.0");
  assert.equal(manifest.engines.node, integrations.engines.node);
  for (const version of ["20.19.0", "22.0.0", "22.17.9", "22.18", "invalid"]) {
    assert.throws(() => assertAgentSpendRuntime(version), { message: "node_22_18_required" });
  }
  for (const version of ["22.18.0", "22.18.1", "22.19.0", "24.0.0"]) assertAgentSpendRuntime(version);
  assertAgentSpendRuntime();
  for (const path of ["index.mjs", "services.mjs"]) {
    assert.match(readFileSync(new URL(`../agent-spend/${path}`, import.meta.url), "utf8"), /assertAgentSpendRuntime\(\)/u);
  }
  assert.match(readFileSync(new URL("../agent-spend/README.md", import.meta.url), "utf8"), /Node\.js \*\*22\.18\.0 or newer\*\*/u);
}

async function walletReviewRecoveryRetainsOriginalAfterLostResponseAndRestart(t) {
  const peer = await openWalletPeer(t);
  const walkthrough = await openWalkthrough(t, peer);
  let component = await walkthrough.reopen();
  const firstWallet = component.wallet;
  peer.controls.dropCreationResponse = true;
  await assert.rejects(component.handoff.review(prepared), /fetch failed/u);
  assert.equal(peer.reviews.size, 1);
  const original = peer.reviews.get(hash);
  const key = `owner:preparation:${prepared.preparation_ref}`;
  assert.deepEqual(JSON.parse(component.store.getItem(key)), { prepared, reviewId: null });
  await walkthrough.close();
  component = await walkthrough.reopen();
  const recovered = await component.handoff.review(prepared);
  assert.deepEqual(recovered, original);
  assert.deepEqual(JSON.parse(component.store.getItem(key)), { prepared, reviewId: original.id });
  assert.notEqual(component.wallet, firstWallet);
  component = await walkthrough.reopen();
  assert.deepEqual(await component.handoff.review(prepared), original);
  assert.deepEqual(await component.handoff.status(prepared.preparation_ref, original.id), original);
  await assert.rejects(component.handoff.approve(prepared.preparation_ref, "another-review"), { message: "original_review_handoff_required" });
  assert.equal(peer.exchanges.filter((entry) => entry.method === "POST" && entry.path === "/v1/wallet/evm/review").length, 1);
  assert.equal(peer.exchanges.some((entry) => /\/(approve|sign)$/u.test(entry.path)), false);
  assert.equal(peer.reviews.size, 1);
}

async function walletReviewRecoverySerializesConcurrentRetries(t) {
  const peer = await openWalletPeer(t);
  const walkthrough = await openWalkthrough(t, peer);
  const { handoff } = await walkthrough.reopen();
  const results = await Promise.all([handoff.review(prepared), handoff.review(prepared), handoff.review(prepared)]);
  assert.deepEqual(results, [peer.reviews.get(hash), peer.reviews.get(hash), peer.reviews.get(hash)]);
  assert.equal(peer.exchanges.filter((entry) => entry.path === "/v1/wallet/evm/review").length, 1);
}

async function walletReviewRecoveryRejectsUnboundArtifactsAndUnsupportedLookup(t) {
  const peer = await openWalletPeer(t);
  const walkthrough = await openWalkthrough(t, peer);
  let component = await walkthrough.reopen();
  for (const lookupError of [{ status: 404, error: "unknown_route" }, { status: 503, error: "unavailable" }]) {
    peer.controls.lookupError = lookupError;
    await assert.rejects(component.handoff.review(prepared));
    assert.equal(peer.reviews.size, 0);
  }
  peer.controls.lookupError = null;
  peer.controls.dropCreationResponse = true;
  await assert.rejects(component.handoff.review(prepared), /fetch failed/u);
  component = await walkthrough.reopen();
  for (const tamper of [{ signing_hash: `0x${"aa".repeat(32)}` }, { principal: "another-owner" },
    { to: `0x${"11".repeat(20)}` }, { value: "1" }, { data: "0xab" }, { chain_id: 126 }]) {
    peer.controls.tamper = tamper;
    await assert.rejects(component.handoff.review(prepared), /review does not match the transaction/u);
    assert.equal(JSON.parse(component.store.getItem(`owner:preparation:${prepared.preparation_ref}`)).reviewId, null);
  }
  peer.controls.tamper = null;
  const original = await component.handoff.review(prepared);
  await assert.rejects(component.handoff.review({ ...prepared, disclosure: "changed" }), { message: "prepared_handoff_conflict" });
  peer.controls.tamper = { chain_id: 126 };
  await assert.rejects(component.handoff.status(prepared.preparation_ref, original.id), /approval does not match the review/u);
  assert.throws(() => preparedTransactionReference({ ...prepared, signing_hash: "bb".repeat(32) }), { message: "prepared_bytes_mismatch" });
  assert.equal(peer.exchanges.filter((entry) => entry.path === "/v1/wallet/evm/review").length, 1);
}

test(agentSpendDependencyClosureLoadsRealTransport);
test(agentSpendNodeMinimumMatchesSetupAndQualification);
test(walletReviewRecoveryRetainsOriginalAfterLostResponseAndRestart);
test(walletReviewRecoverySerializesConcurrentRetries);
test(walletReviewRecoveryRejectsUnboundArtifactsAndUnsupportedLookup);
