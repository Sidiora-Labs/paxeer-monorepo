// Proves: src/x402/receive.ts:encodeAssetRegister/encodeAccountOpen/encodeGrant -> src/index.ts exports -> test/web-search.test.ts:assertPayerGrantRoundTrip -> src/web-search.ts:decodePayerGrant.
// Repairs encodeAssetRegister accepting non-string symbols through TextEncoder coercion;
// both symbol and name must be strings. Account-open and asset-register have no SDK
// decoders, so their consumer-visible wire fields are checked directly. This path
// has no persistent state; each observation below uses newly produced bytes.
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { test } from "node:test";

import { ed25519 } from "@noble/curves/ed25519.js";

import {
  encodeAccountOpen,
  encodeAssetRegister,
  encodeGrant,
  grantAuthorizationMessage,
  WebSearchError,
  type AssetRegistration,
  type PayerGrant,
} from "../src/index.js";
import { assertPayerGrantRoundTrip } from "./web-search.test.js";

const hex = (value: Uint8Array): string => Buffer.from(value).toString("hex");
const identifier = (value: number): string => value.toString(16).padStart(2, "0").repeat(32);

function x402_receive_producers_reach_web_search_consumer(): void {
  const issuer = identifier(1);
  const registration: AssetRegistration = {
    salt: identifier(2), symbol: "LXT", name: "Récéption", decimals: 18,
    supply_cap: ((1n << 128n) - 1n).toString(), issuer_kind: 1,
    custody_ref: new Uint8Array(),
  };
  const expectedAsset = createHash("sha256").update("LX:ASSET:v1", "ascii")
    .update(Buffer.from(issuer, "hex")).update(Buffer.from(registration.salt, "hex")).digest("hex");
  const secret = new Uint8Array(32).fill(7);
  let previous: { registration: Uint8Array; account: Uint8Array; canonical: Uint8Array; grant: PayerGrant } | undefined;

  for (let observation = 0; observation < 2; observation += 1) {
    const registered = encodeAssetRegister(issuer, registration);
    assert.deepEqual(registered.subarray(0, 2), Uint8Array.of(0, 1));
    const asset = hex(registered.subarray(2, 34));
    assert.equal(asset, expectedAsset);
    assert.equal(hex(registered.subarray(34, 66)), registration.salt);
    assert.equal(registered[66], 3);
    assert.equal(Buffer.from(registered.subarray(67, 70)).toString("ascii"), registration.symbol);
    const name = new TextEncoder().encode(registration.name);
    assert.equal(registered[70], name.length);
    assert.deepEqual(registered.subarray(71, 71 + name.length), name);
    const tail = 71 + name.length;
    assert.equal(registered[tail], registration.decimals);
    assert.equal(hex(registered.subarray(tail + 1, tail + 17)), "ff".repeat(16));
    assert.equal(registered[tail + 17], registration.issuer_kind);
    assert.equal(registered[tail + 18], 0);
    assert.equal(registered.length, tail + 19);

    const account = encodeAccountOpen(asset);
    assert.equal(hex(account), `0001${expectedAsset}`);
    const unsigned: PayerGrant = {
      grant_id: identifier(3), from: identifier(4), recipient: identifier(5), asset: hex(account.subarray(2)),
      per_draw_maximum: "9007199254740993", allowance: registration.supply_cap,
      recurring: false, window_length: "0", expiration: "18446744073709551615",
      purpose_hash: identifier(6), has_reference: false, reference_hash: identifier(0),
      revocation_sequence: "9007199254740993", public_key: hex(ed25519.getPublicKey(secret)),
      signature: "00".repeat(64),
    };
    const grant: PayerGrant = {
      ...unsigned, signature: hex(ed25519.sign(grantAuthorizationMessage(unsigned), secret)),
    };
    const canonical = encodeGrant(grant);
    assert.equal(canonical.length, 346);
    const observed = assertPayerGrantRoundTrip(canonical);
    assert.deepEqual(observed, grant);
    assert.equal(observed.asset, hex(account.subarray(2)));
    assert.equal(observed.asset, asset);
    assert.ok(Object.isFrozen(observed));
    assert.ok(ed25519.verify(Buffer.from(observed.signature, "hex"),
      grantAuthorizationMessage(observed), Buffer.from(observed.public_key, "hex")));

    if (previous !== undefined) {
      assert.notStrictEqual(registered, previous.registration);
      assert.notStrictEqual(account, previous.account);
      assert.notStrictEqual(canonical, previous.canonical);
      assert.notStrictEqual(observed, previous.grant);
    }

    // Mutate actual producer output at each boolean, and truncate/append it.
    // The existing web-search consumer must refuse each noncanonical encoding.
    for (const offset of [160, 209]) {
      const malformed = canonical.slice();
      malformed[offset] = 2;
      assert.throws(() => assertPayerGrantRoundTrip(malformed),
        (error: unknown) => error instanceof WebSearchError && error.code === "invalid-grant");
    }
    for (const malformed of [canonical.subarray(0, canonical.length - 1), Uint8Array.from([...canonical, 0])]) {
      assert.throws(() => assertPayerGrantRoundTrip(malformed),
        (error: unknown) => error instanceof WebSearchError && error.code === "invalid-grant");
    }
    previous = { registration: registered, account, canonical, grant: observed };
    // Poison all returned buffers before producing and consuming the next result.
    registered.fill(0);
    account.fill(0);
    canonical.fill(0);
  }
}

function x402_receive_refuses_non_string_asset_metadata(): void {
  const registration: AssetRegistration = {
    salt: identifier(2), symbol: "LXT", name: "Receive proof", decimals: 18,
    supply_cap: "1000", issuer_kind: 1, custody_ref: new Uint8Array(),
  };
  for (const field of ["symbol", "name"] as const) {
    for (const value of [123, null, undefined, { toString: () => "LXT" }]) {
      const malformed = { ...registration, [field]: value } as unknown as AssetRegistration;
      assert.throws(() => encodeAssetRegister(identifier(1), malformed), /^Error: invalid-asset-register$/u);
    }
  }
  assert.throws(() => encodeAccountOpen(identifier(1).slice(2)), /^Error: invalid-receive$/u);
}

test(x402_receive_producers_reach_web_search_consumer);
test(x402_receive_refuses_non_string_asset_metadata);
