import assert from "node:assert/strict";
import { once } from "node:events";
import * as http from "node:http";
import { test } from "node:test";

import {
  AGENT_ENVELOPE_PATH,
  AGENT_ENVELOPE_VERSION,
  AgentEnvelopeTransport,
  AgentSessionCredential,
  checkProofBundleResponse,
  encodeProofBundleRequest,
  type ProofBundleRequest,
  type ProofBundleVerificationLevel,
} from "../src/agent-http.js";
import { Rung } from "../src/generated/client.js";
import { idempotencyKey, PlatformSdkError, SecretBytes } from "../src/production.js";

function session(): AgentSessionCredential {
  return new AgentSessionCredential("major-two", "11".repeat(32), new SecretBytes(Buffer.alloc(32, 0x22)), 1n);
}

async function listen(server: http.Server): Promise<string> {
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  const address = server.address();
  assert(address !== null && typeof address === "object");
  return `http://127.0.0.1:${address.port}`;
}

async function close(server: http.Server): Promise<void> {
  const closed = once(server, "close");
  server.close();
  await closed;
}

function sdkError(code: PlatformSdkError["code"]): (error: unknown) => boolean {
  return (error) => error instanceof PlatformSdkError && error.code === code;
}

async function agent_envelope_requests_carry_contract_major_2(): Promise<void> {
  const received: Record<string, unknown>[] = [];
  let major: unknown = 2;
  let refusal = false;
  const credential = session();
  const key = idempotencyKey("33".repeat(32));
  const server = http.createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8")) as Record<string, unknown>;
    received.push(body);
    assert.equal(request.method, "POST");
    assert.equal(request.url, AGENT_ENVELOPE_PATH);
    assert.equal(request.headers["content-type"], "application/json");
    assert.equal(body.version, 1);
    assert.equal(body.contract_major, 2);
    assert.deepEqual(body.credential, credential.encode());
    response.writeHead(refusal ? 403 : 200, { "Content-Type": "application/json" });
    response.end(JSON.stringify(refusal ? {
      contract_major: major, request_id: body.request_id,
      class: "PolicyRefusal", protocol_result_code: null, retriability: "Terminal", reason: "policy.denied",
    } : {
      contract_major: major, request_id: body.request_id,
      value: { operation: body.operation }, verification_status: { state: "achieved", rung: "submitted" },
    }));
  });
  const endpoint = await listen(server);
  const transport = new AgentEnvelopeTransport({ endpoint, session: credential });
  try {
    assert.equal(AGENT_ENVELOPE_VERSION, 1);
    for (const operation of ["session.list", "session.close", "agent.register", "session.open"] as const) {
      const response = await transport.call<{}, { value: { operation: string } }>({
        plane: "agent", operation, request: {},
        ...(operation === "session.list" ? {} : { idempotencyKey: key }),
      });
      assert.equal(response.value.operation, operation);
      assert.equal(received.at(-1)?.idempotency_key, operation === "session.list" ? null : key);
    }
    for (refusal of [false, true]) {
      for (major of [1, 3, "2", null, undefined]) {
        for (const mutation of [false, true]) {
          await assert.rejects(transport.call({
            plane: "agent", operation: mutation ? "session.close" : "session.list", request: {},
            ...(mutation ? { idempotencyKey: key } : {}),
          }), (error: unknown) => {
            assert(error instanceof PlatformSdkError);
            assert.equal(error.code, "protocol-incompatibility");
            assert.equal(error.retry, mutation ? "unknown-outcome" : "never");
            return true;
          });
        }
      }
    }
    major = 2;
    await assert.rejects(transport.call({ plane: "agent", operation: "session.list", request: {} }), sdkError("policy-refusal"));
    assert.equal(received.length, 25);
  } finally {
    await close(server);
  }
}

async function proof_bundle_levels_are_rungs(): Promise<void> {
  const levels: ProofBundleVerificationLevel[] = [Rung.Submitted, Rung.Receipted, Rung.Committed];
  const target = `000101${"44".repeat(32)}`;
  const proof = Buffer.concat([
    Buffer.from("LXPB1", "ascii"), Buffer.from([1]),
    Buffer.from([0, 0, 0, 1, 0x44]), Buffer.from([0, 0, 0, 1, 0x55]),
  ]).toString("hex");
  let achieved: unknown = "receipted";
  let statusRung: unknown = "receipted";
  let replyTarget = target;
  let requests = 0;
  const server = http.createServer(async (request, response) => {
    const chunks: Buffer[] = [];
    for await (const chunk of request) chunks.push(Buffer.from(chunk));
    const body = JSON.parse(Buffer.concat(chunks).toString("utf8"));
    requests += 1;
    assert.equal(request.url, AGENT_ENVELOPE_PATH);
    assert.equal(body.contract_major, 2);
    assert.equal(body.operation, "read.proof_bundle");
    assert.equal(body.request.target, target);
    assert(["submitted", "receipted", "committed"].includes(body.request.requested_verification_level));
    response.writeHead(200, { "Content-Type": "application/json" });
    response.end(JSON.stringify({
      contract_major: 2, request_id: body.request_id,
      value: {
        value: { target: replyTarget, proofs: [proof] }, achieved_verification_level: achieved,
        freshness: {
          chain_head: "10", latest_sealed_batch: "2", latest_finalised_checkpoint: "66".repeat(32),
          value_sequence: "9", relative_to: { batch: "2" },
        },
      },
      verification_status: { state: "achieved", rung: statusRung },
    }));
  });
  const endpoint = await listen(server);
  const transport = new AgentEnvelopeTransport({ endpoint, session: session() });
  const request = { target, requested_verification_level: Rung.Receipted } satisfies ProofBundleRequest;
  try {
    for (const level of levels.slice(0, 2)) {
      const typedRequest: ProofBundleRequest = { target, requested_verification_level: level };
      const reply = await transport.readProofBundle(typedRequest);
      assert.equal(reply.value.achieved_verification_level, Rung.Receipted);
      assert.deepEqual(reply.verification_status, { state: "achieved", rung: Rung.Receipted });
      assert.deepEqual(checkProofBundleResponse(typedRequest, reply), {
        variant: 1, canonical_value: "44", native_proof: "55",
      });
    }
    await assert.rejects(transport.readProofBundle({ target, requested_verification_level: Rung.Committed }), sdkError("decode-failure"));
    for (const legacy of ["Unverified", "SequencerSigned", "BatchIncluded", "StateProven", "CheckpointFinalised", "SettlementAnchored"]) {
      assert.throws(() => encodeProofBundleRequest({ target, requested_verification_level: legacy }), sdkError("invalid-argument"));
      achieved = legacy;
      await assert.rejects(transport.readProofBundle(request), sdkError("decode-failure"));
      achieved = "receipted";
      statusRung = legacy;
      await assert.rejects(transport.readProofBundle(request), sdkError("verification-failure"));
      statusRung = "receipted";
    }
    for (const invalid of [-1, 3, 1.5, NaN, "receipted", null]) {
      assert.throws(() => encodeProofBundleRequest({ target, requested_verification_level: invalid }), sdkError("invalid-argument"));
    }
    statusRung = "committed";
    await assert.rejects(transport.readProofBundle(request), sdkError("decode-failure"));
    statusRung = "receipted";
    replyTarget = `000101${"77".repeat(32)}`;
    await assert.rejects(transport.readProofBundle(request), sdkError("decode-failure"));
    assert.equal(requests, 17);
  } finally {
    await close(server);
  }
}

test(agent_envelope_requests_carry_contract_major_2);
test(proof_bundle_levels_are_rungs);
