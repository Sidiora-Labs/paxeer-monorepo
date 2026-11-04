import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { registerHooks, stripTypeScriptTypes } from "node:module";
import { DatabaseSync } from "node:sqlite";
import test from "node:test";
import { committedVector } from "../../seller/test/committed-fixture.mjs";

const sources = new URL("../src/", import.meta.url).href;
const hooks = registerHooks({
  resolve(specifier, context, next) {
    if (context.parentURL?.startsWith(sources) && specifier.startsWith("./") && specifier.endsWith(".js")) {
      return next(new URL(`${specifier.slice(0, -3)}.ts`, context.parentURL).href, context);
    }
    return next(specifier, context);
  },
  load(url, context, next) {
    if (!url.startsWith(sources) || !url.endsWith(".ts")) return next(url, context);
    return { format: "module", shortCircuit: true,
      source: stripTypeScriptTypes(readFileSync(new URL(url), "utf8"), { mode: "transform", sourceUrl: url }) };
  },
});
let AgentGrantMiddleware;
try {
  ({ AgentGrantMiddleware } = await import("../src/index.ts"));
} finally {
  hooks.deregister();
}

async function agentGrantResolutionUsesConfiguredStore(t) {
  const database = new DatabaseSync(":memory:");
  t.after(() => database.close());
  database.exec("CREATE TABLE grants (network TEXT, id TEXT, source TEXT, asset TEXT, recipient TEXT, executor TEXT, active INTEGER, PRIMARY KEY (network, id))");
  const grantId = `0x${"57".repeat(32)}`;
  const network = "eip155:125";
  const grant = { source: `0x${"11".repeat(32)}`, asset: `0x${"22".repeat(32)}`,
    recipient: `0x${"33".repeat(32)}`, executor: `0x${"44".repeat(20)}`, active: true };
  database.prepare("INSERT INTO grants VALUES (?, ?, ?, ?, ?, ?, ?)").run(
    network, grantId, grant.source, grant.asset, grant.recipient, grant.executor, 1,
  );
  const draws = {
    database,
    async resolveGrant(id, chain) {
      const row = this.database.prepare("SELECT source, asset, recipient, executor, active FROM grants WHERE network = ? AND id = ?").get(chain, id);
      return row === undefined ? undefined : { ...row, active: row.active === 1 };
    },
  };
  const key = committedVector.pinnedKey;
  const receiptKeys = [{ keyId: new Uint8Array(Buffer.from(key.keyId.slice(2), "hex")),
    publicKey: new Uint8Array(Buffer.from(key.publicKey.slice(2), "hex")),
    first_height: BigInt(key.firstHeight), last_height: BigInt(key.lastHeight) }];
  const middleware = new AgentGrantMiddleware({ tenant: "grant-resolution", receiptKeys, draws });
  assert.deepEqual(await middleware.resolveGrant(grantId, network), grant);
  assert.equal(await middleware.resolveGrant(`0x${"58".repeat(32)}`, network), undefined);
  assert.equal(await middleware.resolveGrant(grantId, "eip155:1"), undefined);
  database.prepare("UPDATE grants SET active = 0 WHERE network = ? AND id = ?").run(network, grantId);
  assert.deepEqual(await middleware.resolveGrant(grantId, network), { ...grant, active: false });
  database.prepare("DELETE FROM grants WHERE network = ? AND id = ?").run(network, grantId);
  assert.equal(await middleware.resolveGrant(grantId, network), undefined);
  database.exec("DROP TABLE grants");
  await assert.rejects(middleware.resolveGrant(grantId, network), /no such table/u);
}

test(agentGrantResolutionUsesConfiguredStore);
