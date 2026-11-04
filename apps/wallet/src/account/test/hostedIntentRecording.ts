import { expect } from 'vitest';
import { createHash } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { createServer } from 'node:http';
import { join } from 'node:path';
import type { AddressInfo } from 'node:net';
import { createElement as h } from 'react';
import {
  decodeAccountDocument,
  decodeIntentPlan,
  EndpointClient,
  HumanClient,
  KernelAvailability,
  type IntentPlan,
  type PlanIntentRequest,
} from '@paxeer/wallet';
import { legBindings } from '../KernelPlan';
import { FeeChoice } from '../../surfaces/FeeChoice';
import { useFeeSelection } from '../../surfaces/useSurface';
import { WalletProvider } from '../../wallet/WalletProvider';
import { mount } from './render';

interface RecordedBytes {
  base64: string;
  sha256: string;
}

interface RecordedObservation {
  key: string;
  row: RecordedBytes;
}

interface HostedIntentRecording {
  format: number;
  producer: string;
  store: string;
  transport: string;
  request: PlanIntentRequest;
  wallet_address: string;
  actor: string;
  sequence: number;
  not_after: number;
  before: RecordedBytes;
  after: RecordedBytes;
  recovered_before_replanning: RecordedObservation;
  recovered_after_replanning: RecordedObservation;
  refused_request: PlanIntentRequest;
  refusal: RecordedBytes;
  provenance: {
    revision: string;
    command: string;
    sources: Record<string, string>;
  };
}

interface HeldIntentObservation {
  profile: number;
  principal: string;
  tenant: string;
  session_id: string;
  plan_digest: number[];
  canonical_intent: number[];
  canonical_plan: number[];
  transactions: { nonce: number }[];
}

function bytes(record: RecordedBytes): Buffer {
  const result = Buffer.from(record.base64, 'base64');
  expect(createHash('sha256').update(result).digest('hex')).toBe(record.sha256);
  expect(result.length).toBeGreaterThan(0);
  return result;
}

function recordedPlan(record: RecordedBytes): IntentPlan {
  const response = bytes(record).toString('utf8');
  const separator = response.indexOf('\r\n\r\n');
  expect(separator).toBeGreaterThan(0);
  const headers = response.slice(0, separator);
  expect(headers).toMatch(/^HTTP\/1\.1 200 /);
  const body = response.slice(separator + 4);
  expect(headers).toContain(`Content-Length: ${Buffer.byteLength(body)}`);
  const envelope = JSON.parse(body);
  expect(envelope.ok).toBe(true);
  expect(envelope.trace).toEqual(expect.any(String));
  return decodeIntentPlan(envelope.result);
}

function PlanFees({ plan }: { plan: IntentPlan }) {
  const selection = useFeeSelection(null, null);
  return h(FeeChoice, {
    provider: null,
    address: null,
    transaction: null,
    selection,
    sidRate: null,
    legs: plan.legs,
  });
}

export async function proveHostedIntentReopen(): Promise<void> {
  const recording: HostedIntentRecording = JSON.parse(
    readFileSync(join(__dirname, '../../hosted-intent-22-33-152.recording.json'), 'utf8')
  );
  expect(recording.format).toBe(1);
  expect(recording.producer).toBe('layerx-human-service::server::ProductionComponents');
  expect(recording.store).toBe('layerx-human-service::store::PrincipalStore');
  expect(recording.transport).toBe(
    'Router::serve_one over one TCP listener before and after Fixture::reopen'
  );
  expect(recording.provenance.revision).toMatch(/^[0-9a-f]{40}$/);
  expect(recording.provenance.command).toBe('python3 apps/wallet/src/record-22-33-152.py');
  expect(Object.keys(recording.provenance.sources)).toEqual([
    'services/human/crates/layerx-human-service/tests/support/production_fixture.rs',
    'services/human/crates/layerx-human-service/src/server/production_components.rs',
    'services/human/crates/layerx-human-service/src/server/http.rs',
    'services/human/crates/layerx-human-service/src/store/mod.rs',
  ]);
  for (const [path, digest] of Object.entries(recording.provenance.sources)) {
    const producerSource = readFileSync(join(__dirname, '../../../../..', path));
    expect(createHash('sha256').update(producerSource).digest('hex')).toBe(digest);
  }
  const original = recordedPlan(recording.before);
  expect(original.journey_kind).toBe('move');
  expect(original.legs.length).toBeGreaterThan(0);
  expect(original.legs.every((leg) => leg.domain === 'layerx' && leg.mechanism === 'send')).toBe(
    true
  );
  expect(recordedPlan(recording.after)).toEqual(original);

  // The recorder closes the router, acquires its released store lock and reads
  // this row through PrincipalStore BEFORE any post-reopen plan request.
  const recovered = recording.recovered_before_replanning;
  const memo: HeldIntentObservation = JSON.parse(bytes(recovered.row).toString('utf8'));
  expect(memo.profile).toBe(1);
  expect(memo.principal).not.toBe('');
  expect(memo.tenant).not.toBe('');
  expect(memo.session_id).not.toBe('');
  expect(Buffer.from(memo.plan_digest).toString('hex')).toBe(original.plan_digest);
  expect(JSON.parse(Buffer.from(memo.canonical_intent).toString('utf8'))).toEqual(
    recording.request
  );
  expect(memo.canonical_plan.length).toBeGreaterThan(0);
  const observationKey = createHash('sha256')
    .update(Buffer.from(memo.plan_digest))
    .update(createHash('sha256').update(memo.session_id).digest())
    .digest('hex');
  expect(recovered.key).toBe(`intent-observation-${observationKey}`);
  expect(recording.recovered_after_replanning).toEqual(recovered);
  expect(bytes(recording.recovered_after_replanning.row)).toEqual(bytes(recovered.row));
  expect(memo.transactions.map((transaction) => transaction.nonce)).toEqual(
    original.legs.map((_, index) => recording.sequence + index)
  );

  const exchanges = [
    { request: recording.request, response: recording.before },
    { request: recording.request, response: recording.after },
    { request: recording.refused_request, response: recording.refusal },
  ];
  let served = 0;
  const failures: unknown[] = [];
  // Only the committed producer bytes are replayed; no envelope is constructed.
  const server = createServer((request, response) => {
    void (async () => {
      const entry = exchanges[served++];
      if (!entry) throw new Error('unexpected hosted intent request');
      expect(request.method).toBe('POST');
      expect(request.url).toBe('/v1/intents/plan');
      expect(request.headers['content-type']).toBe('application/json');
      let body = '';
      for await (const chunk of request) body += String(chunk);
      expect(JSON.parse(body)).toEqual(entry.request);
      request.socket.end(bytes(entry.response));
    })().catch((error: unknown) => {
      failures.push(error);
      response.writeHead(500);
      response.end('recorded exchange mismatch');
    });
  });
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  try {
    const url = `http://127.0.0.1:${(server.address() as AddressInfo).port}`;
    const human = new HumanClient({
      url,
      kernel: new KernelAvailability(new EndpointClient({ url })),
    });
    const document = decodeAccountDocument({
      evm_address: recording.wallet_address,
      pax_address: null,
      layerx_did: recording.actor,
      layerx_account: recording.request.source.account,
      bound: true,
    });
    for (let phase = 0; phase < 2; phase += 1) {
      const plan = await human.planIntent(recording.request);
      expect(plan).toEqual(original);
      const notBefore = recording.not_after - 300;
      const bindings = legBindings(
        plan,
        document,
        recording.sequence,
        { notBefore, notAfter: recording.not_after },
        recording.request.constraints.max_fee
      );
      expect(bindings).toEqual(
        plan.signing_requirements.map((requirement, index) => ({
          leg_index: requirement.leg_index,
          action_key: requirement.action_key,
          actor: recording.wallet_address,
          authority: requirement.authority,
          relationship: 'self',
          account_sequence: memo.transactions[index].nonce,
          not_before: notBefore,
          not_after: recording.not_after,
          fee_limit: recording.request.constraints.max_fee,
        }))
      );
      const view = await mount(
        h(WalletProvider, { config: null, identity: null, children: h(PlanFees, { plan }) })
      );
      try {
        expect(view.all('[data-layerx-leg]').map((node) => node.textContent)).toEqual(
          original.legs.map(
            (leg) => `Leg ${leg.index} · ${leg.mechanism}${leg.fee.amount} ${leg.fee.currency}`
          )
        );
        expect(view.query('[data-role="layerx-none"]')).toBeNull();
      } finally {
        await view.unmount();
      }
      expect(served).toBe(phase + 1);
    }
    await expect(human.planIntent(recording.refused_request)).rejects.toMatchObject({
      name: 'HumanServiceError',
      status: 400,
      code: 'invalid-request',
      body: { field: 'money.currency' },
    });
    expect(served).toBe(3);
    expect(failures).toEqual([]);
  } finally {
    await new Promise<void>((resolve, reject) => {
      server.close((error) => (error ? reject(error) : resolve()));
      server.closeAllConnections();
    });
  }
}
