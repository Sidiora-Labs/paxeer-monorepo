import assert from "node:assert/strict";
import test from "node:test";
import { accountIdentifiers } from "../dist/index.js";

test("the advertised account derives the payee the buyer requires an offer to name", async () => {
  const account = "agent:did:layerx:conformance-seller:main";
  const identifiers = await accountIdentifiers(account);
  assert.equal(identifiers.length, 2);
  for (const identifier of identifiers) assert.equal(identifier.length, 32);
  const native = Buffer.from(identifiers[identifiers.length - 1]).toString("hex");
  assert.match(native, /^[0-9a-f]{64}$/u);
  assert.notEqual(native, "aa".repeat(32));
});
