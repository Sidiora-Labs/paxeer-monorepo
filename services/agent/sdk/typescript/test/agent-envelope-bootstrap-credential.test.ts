import assert from "node:assert/strict";
import { test } from "node:test";

import { AgentSessionCredential, encodeAgentEnvelope } from "../src/agent-http.js";
import { idempotencyKey, PlatformSdkError, SecretBytes } from "../src/production.js";

function bootstrapOperationsCarryOwnerSessionCredential(): void {
  const session = new AgentSessionCredential(
    "owner-tenant",
    "11".repeat(32),
    new SecretBytes(Buffer.from("22".repeat(32), "hex")),
    9007199254740993n,
  );
  const key = idempotencyKey("33".repeat(32));

  for (const operation of ["agent.register", "session.open"] as const) {
    const body = encodeAgentEnvelope(operation, "1", {}, session, key);
    const envelope = JSON.parse(body.toString("utf8"));
    assert.deepEqual(envelope.credential, session.encode());
    assert.equal(envelope.operation, operation);
    assert.equal(envelope.idempotency_key, key);
  }
}

function bootstrapOperationsRefuseMissingSession(): void {
  const key = idempotencyKey("33".repeat(32));

  for (const operation of ["agent.register", "session.open"] as const) {
    assert.throws(
      () => encodeAgentEnvelope(operation, "1", {}, undefined, key),
      (error: unknown) => error instanceof PlatformSdkError && error.code === "invalid-argument",
    );
  }
}

test(bootstrapOperationsCarryOwnerSessionCredential.name, bootstrapOperationsCarryOwnerSessionCredential);
test(bootstrapOperationsRefuseMissingSession.name, bootstrapOperationsRefuseMissingSession);
