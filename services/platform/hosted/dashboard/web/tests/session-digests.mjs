import assert from "node:assert/strict";
import { test } from "node:test";
import { registrationBinding, sessionDigest, lowercaseHex, generateDeveloperKey } from "../app/browser-key.ts";

async function registration_binding_matches_the_gateway_digest() {
  const digest = await registrationBinding("beta", "ab".repeat(32));
  assert.equal(lowercaseHex(digest), "425fa6c5988efc16bce8a7931f1efbc039b75d10f0ffa617c3c2d3de2366e128");
  assert.notDeepEqual(digest, await registrationBinding("gamma", "ab".repeat(32)));
}

test(registration_binding_matches_the_gateway_digest);

async function session_digest_matches_the_gateway_digest_and_binds_the_time() {
  const digest = await sessionDigest("beta", "ab".repeat(32), "1700000000");
  assert.equal(lowercaseHex(digest), "03716e9e25f25c00c2fc0b1db517acba61c0dbc17e30463bee7f49733a2069e1");
  assert.notDeepEqual(digest, await registrationBinding("beta", "ab".repeat(32)));
  assert.notDeepEqual(digest, await sessionDigest("beta", "ab".repeat(32), "1700000001"));
  assert.notDeepEqual(digest, await sessionDigest("gamma", "ab".repeat(32), "1700000000"));
  assert.notDeepEqual(digest, await sessionDigest("beta", "ac".repeat(32), "1700000000"));
  for (const time of ["01", "-1", "1.5", "18446744073709551616"]) {
    assert.throws(() => sessionDigest("beta", "ab".repeat(32), time), /invalid_proof_time/);
  }
}

test(session_digest_matches_the_gateway_digest_and_binds_the_time);

async function signatures_cover_the_digest_with_a_nonexportable_ed25519_key() {
  const pair = await generateDeveloperKey();
  assert.equal(pair.privateKey.extractable, false);
  await assert.rejects(crypto.subtle.exportKey("pkcs8", pair.privateKey));
  const key = lowercaseHex(new Uint8Array(await crypto.subtle.exportKey("raw", pair.publicKey)));
  assert.match(key, /^[0-9a-f]{64}$/);
  for (const digest of [await registrationBinding("beta", key), await sessionDigest("beta", key, "1700000000")]) {
    const signature = await crypto.subtle.sign("Ed25519", pair.privateKey, digest);
    assert.equal(await crypto.subtle.verify("Ed25519", pair.publicKey, signature, digest), true);
    const altered = digest.slice();
    altered[0] ^= 1;
    assert.equal(await crypto.subtle.verify("Ed25519", pair.publicKey, signature, altered), false);
  }
}

test(signatures_cover_the_digest_with_a_nonexportable_ed25519_key);
