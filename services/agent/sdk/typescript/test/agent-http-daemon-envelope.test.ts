import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { test } from "node:test";

import { AgentSessionCredential, decodeAgentEnvelopeResponse, encodeAgentEnvelope } from "../src/agent-http.js";
import { Rung } from "../src/generated/client.js";
import { PlatformSdkError, SecretBytes } from "../src/production.js";

function agent_envelope_matches_daemon_goldens(): void {
  const goldens = new URL("../../../../schema/agent-api/golden/", import.meta.url);
  const requestGolden = JSON.parse(readFileSync(new URL("agent-http-request.json", goldens), "utf8")) as {
    credential: { tenant: string; session_id: string; token_id: string; generation: string };
  };
  const credential = requestGolden.credential;
  const session = new AgentSessionCredential(
    credential.tenant,
    credential.session_id,
    new SecretBytes(Uint8Array.from(Buffer.from(credential.token_id, "hex"))),
    credential.generation,
  );
  const encoded = encodeAgentEnvelope("tenant.readiness", "7", {}, session, undefined);
  const request = JSON.parse(encoded.toString("utf8")) as Record<string, unknown>;
  assert.deepEqual(Object.keys(request).sort(), Object.keys(requestGolden).sort());
  assert.deepEqual(request, requestGolden);
  assert.equal(Object.hasOwn(request, "contract_major"), false);

  const successBytes = readFileSync(new URL("agent-http-success.json", goldens));
  const success = JSON.parse(successBytes.toString("utf8")) as Record<string, unknown>;
  const decoded = decodeAgentEnvelopeResponse(200, successBytes, false, "7");
  assert.equal(decoded.request_id, "7");
  assert.deepEqual(decoded.value, success.value);
  assert.deepEqual(decoded.verification_status, { state: "achieved", rung: Rung.Submitted });

  const refusalBytes = readFileSync(new URL("agent-http-refusal.json", goldens));
  const refusal = JSON.parse(refusalBytes.toString("utf8")) as Record<string, unknown>;
  for (const mutation of [false, true]) {
    assert.throws(() => decodeAgentEnvelopeResponse(403, refusalBytes, mutation, "7"), (error: unknown) => {
      assert(error instanceof PlatformSdkError);
      assert.equal(error.code, "policy-refusal");
      assert.equal(error.retry, "never");
      assert.equal(error.requestId, "7");
      assert.equal(error.protocolResultCode, undefined);
      return true;
    });
    for (const [status, reply] of [[200, success], [403, refusal]] as const) {
      for (const extra of [{ unexpected: true }, { contract_major: 2 }]) {
        const bytes = Buffer.from(JSON.stringify({ ...reply, ...extra }));
        assert.throws(() => decodeAgentEnvelopeResponse(status, bytes, mutation, "7"), (error: unknown) => {
          assert(error instanceof PlatformSdkError);
          assert.equal(error.code, mutation ? "unknown-outcome" : "decode-failure");
          assert.equal(error.retry, mutation ? "unknown-outcome" : "never");
          return true;
        });
      }
    }
  }
}

test(agent_envelope_matches_daemon_goldens);
