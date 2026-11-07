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

function agent_envelope_accepts_not_ready_verification_status(): void {
  const goldens = new URL("../../../../schema/agent-api/golden/", import.meta.url);
  for (const [filename, expectedStatus] of [
    ["agent-http-success-pending.json", { state: "pending", requested: Rung.Receipted, achieved: Rung.Submitted }],
    ["agent-http-success-absent.json", { state: "absent", requested: Rung.Receipted }],
  ] as const) {
    const bytes = readFileSync(new URL(filename, goldens));
    const envelope = JSON.parse(bytes.toString("utf8")) as {
      request_id: string;
      value: { found: boolean };
      verification_status: Record<string, unknown>;
    };
    const decoded = decodeAgentEnvelopeResponse(200, bytes, false, envelope.request_id);
    assert.equal(decoded.request_id, "7");
    assert.deepEqual(decoded.value, envelope.value);
    assert.equal(envelope.value.found, false);
    assert.deepEqual(decoded.verification_status, expectedStatus);
    assert.equal(Object.isFrozen(decoded), true);
    assert.equal(Object.isFrozen(decoded.verification_status), true);

    const status = envelope.verification_status;
    const invalidStatuses = [
      { ...status, unexpected: true },
      { ...status, state: "unknown" },
      { ...status, requested: "unknown" },
      ...(status.state === "pending" ? [
        { ...status, achieved: status.requested },
        { ...status, achieved: "committed" },
        { ...status, achieved: "unknown" },
        { ...status, reason: "receipt_pending" },
      ] : [
        { ...status, achieved: "submitted" },
      ]),
    ];
    for (const verification_status of invalidStatuses) {
      const invalidBytes = Buffer.from(JSON.stringify({ ...envelope, verification_status }));
      assert.throws(() => decodeAgentEnvelopeResponse(200, invalidBytes, false, envelope.request_id), (error: unknown) => {
        assert(error instanceof PlatformSdkError);
        assert.equal(error.code, "verification-failure");
        assert.equal(error.retry, "never");
        assert.equal(error.requestId, "7");
        return true;
      });
    }
  }
}

test(agent_envelope_matches_daemon_goldens);
test(agent_envelope_accepts_not_ready_verification_status);
