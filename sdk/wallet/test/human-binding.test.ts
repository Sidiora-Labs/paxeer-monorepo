import { createServer } from 'node:http';
import type { AddressInfo } from 'node:net';
import { expect, expectTypeOf, test } from 'vitest';
import { EndpointClient } from '../src/endpoint.js';
import { decodeIntentPlan, decodeJourney, HumanClient, HumanDecodeError } from '../src/human.js';
import { KernelAvailability, KernelClient } from '../src/kernel.js';
import type { EvidenceRef, IntentLegBinding, IntentPlan, IntentSubmission, Journey, SubmitPlanRequest } from '../src/types.js';

async function intent_binding_carries_the_pending_nonce_and_optional_tx_hash(): Promise<void> {
  expectTypeOf<IntentLegBinding['account_sequence']>().toEqualTypeOf<number>();
  expectTypeOf<IntentLegBinding['tx_hash']>().toEqualTypeOf<string | undefined>();
  const sender = `0x${'ab'.repeat(20)}`;
  const txHash = `0x${'aB'.repeat(32)}`;
  const leg = {
    index: 0,
    mechanism: 'transfer',
    domain: 'paxeer',
    source: { kind: 'paxeer-wallet' },
    destination: { kind: 'human', account: 'recipient' },
    money: { amount: '100', currency: 'apax' },
    fee: { amount: '1', currency: 'apax' },
  } satisfies IntentPlan['legs'][number];
  const plan: IntentPlan = {
    plan_digest: '11'.repeat(32),
    journey_kind: 'move',
    total_fee: { amount: '2', currency: 'apax' },
    legs: [{ ...leg, tx_hash: txHash }, { ...leg, index: 1 }],
    signing_requirements: [],
  };
  expect(decodeIntentPlan(plan)).toEqual(plan);
  expect(decodeIntentPlan(plan).legs[1]).not.toHaveProperty('tx_hash');
  for (const hash of [`0x${'00'.repeat(32)}`, `0x${'FF'.repeat(32)}`, txHash]) {
    const withHash = { ...plan, legs: [{ ...leg, tx_hash: hash }, plan.legs[1]] };
    expect(decodeIntentPlan(withHash).legs[0].tx_hash).toBe(hash);
  }
  for (const invalid of [
    undefined,
    null,
    false,
    42,
    {},
    [],
    '',
    txHash.slice(2),
    `0X${'ab'.repeat(32)}`,
    `0x${'ab'.repeat(31)}`,
    `0x${'ab'.repeat(33)}`,
    `0x${'gg'.repeat(32)}`,
    `${txHash}\n`,
    ` ${txHash}`,
    `${txHash} `,
  ]) {
    for (const position of [0, 1]) {
      const legs = plan.legs.map((entry, index) =>
        index === position ? { ...entry, tx_hash: invalid } : entry
      );
      expect(() => decodeIntentPlan({ ...plan, legs })).toThrowError(
        new HumanDecodeError('intent leg tx_hash')
      );
    }
  }

  const submission: IntentSubmission = {
    journey_id: 'binding-journey',
    plan_digest: plan.plan_digest,
    state: 'sending',
    state_copy_key: 'journey.sending',
  };
  const requests: {
    method: string | undefined;
    path: string | undefined;
    key: string | string[] | undefined;
    body: unknown;
  }[] = [];
  const server = createServer(async (req, res) => {
    let body = '';
    for await (const chunk of req) body += chunk;
    const payload = JSON.parse(body);
    requests.push({ method: req.method, path: req.url, key: req.headers['idempotency-key'], body: payload });
    res.writeHead(200, { 'content-type': 'application/json' });
    res.end(JSON.stringify(
      req.url === '/rpc'
        ? { jsonrpc: '2.0', id: payload.id, result: '0x2a' }
        : { ok: true, trace: 'binding-trace', result: submission }
    ));
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  try {
    const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    const endpoint = new EndpointClient({ url: `${url}/rpc` });
    const availability = new KernelAvailability(endpoint, { ttlMs: 60_000 });
    availability.record({ available: true, reason: 'available' });
    const pending = await new KernelClient(endpoint, availability).getSequence(sender);
    expect(pending).toEqual({ available: true, result: '42' });
    if (!pending.available) throw new Error('pending nonce unavailable');
    const binding: IntentLegBinding = {
      leg_index: 0,
      action_key: '22'.repeat(32),
      actor: sender,
      authority: 'sender',
      relationship: 'owner',
      account_sequence: Number(pending.result),
      not_before: 0,
      not_after: 100,
      fee_limit: leg.fee,
    };
    const request: SubmitPlanRequest = {
      plan_digest: plan.plan_digest,
      signed_digest: '33'.repeat(32),
      bindings: [{ ...binding, tx_hash: txHash }, { ...binding, leg_index: 1 }],
    };
    const client = new HumanClient({ url, kernel: availability });
    await expect(client.submitPlan(request, 'binding-key')).resolves.toEqual({
      available: true,
      result: submission,
    });
    expect(requests).toEqual([
      {
        method: 'POST',
        path: '/rpc',
        key: undefined,
        body: {
          jsonrpc: '2.0',
          id: expect.any(Number),
          method: 'eth_getTransactionCount',
          params: [sender, 'pending'],
        },
      },
      { method: 'POST', path: '/v1/intents/submit', key: 'binding-key', body: request },
    ]);
    expect(request.bindings[0].account_sequence).toBe(42);
    expect(request.bindings[1].account_sequence).toBe(42);
  } finally {
    await new Promise<void>((resolve, reject) =>
      server.close((error) => error ? reject(error) : resolve())
    );
  }
}

test(
  intent_binding_carries_the_pending_nonce_and_optional_tx_hash,
  intent_binding_carries_the_pending_nonce_and_optional_tx_hash
);

function journey_evidence_carries_an_optional_transaction_hash(): void {
  expectTypeOf<EvidenceRef['tx_hash']>().toEqualTypeOf<string | undefined>();
  const txHash = `0x${'aB'.repeat(32)}`;
  const ref: EvidenceRef = {
    evidence_id: 'receipt-evidence',
    class: 'layerx-receipt',
    verification: 'receipt-verified',
    settlement_domain: 'paxeer',
  };
  const journey: Journey = {
    journey_id: 'evidence-journey',
    kind: 'move',
    state: 'processing',
    state_copy_key: 'journey.processing',
    stages: [{
      stage_id: 'receipt-stage',
      copy_key: 'journey.receipt',
      state: 'processing',
      evidence: [{ ...ref, tx_hash: txHash }, { ...ref }],
    }],
    evidence: [{ ...ref, tx_hash: txHash }, { ...ref }],
    started_at: '2026-01-01T00:00:00Z',
    updated_at: '2026-01-01T00:00:01Z',
  };
  const decoded = decodeJourney(journey);
  expect(decoded).toEqual(journey);
  expect(decoded.evidence[1]).not.toHaveProperty('tx_hash');
  expect(decoded.stages[0].evidence[1]).not.toHaveProperty('tx_hash');

  for (const hash of [`0x${'00'.repeat(32)}`, `0x${'ab'.repeat(32)}`, `0x${'FF'.repeat(32)}`, txHash]) {
    const evidence = [{ ...ref, tx_hash: hash }, { ...ref }];
    const withHash = { ...journey, evidence, stages: [{ ...journey.stages[0], evidence }] };
    expect(decodeJourney(withHash)).toEqual(withHash);
  }
  for (const invalid of [
    undefined,
    null,
    false,
    42,
    {},
    [],
    '',
    txHash.slice(2),
    `0X${'ab'.repeat(32)}`,
    `0x${'ab'.repeat(31)}`,
    `0x${'ab'.repeat(33)}`,
    `0x${'gg'.repeat(32)}`,
    `${txHash}\n`,
    ` ${txHash}`,
    `${txHash} `,
  ]) {
    for (const position of [0, 1]) {
      const evidence = journey.evidence.map((entry, index) =>
        index === position ? { ...entry, tx_hash: invalid } : entry
      );
      expect(() => decodeJourney({ ...journey, evidence })).toThrowError(
        new HumanDecodeError('journey evidence tx_hash')
      );
      expect(() => decodeJourney({
        ...journey,
        stages: [{ ...journey.stages[0], evidence }],
      })).toThrowError(new HumanDecodeError('journey stage evidence tx_hash'));
    }
  }
}

test(
  journey_evidence_carries_an_optional_transaction_hash,
  journey_evidence_carries_an_optional_transaction_hash
);
