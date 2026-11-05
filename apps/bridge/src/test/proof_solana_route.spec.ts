/**
 * Proves: solana.ts recipientRecordAddress/recipientRecordRequest/parseRecipientRecord -> solana.ts recipientRecordFromResponse and use-bridge-state.ts useRecipientRecord -> solana-route.ts solanaWithdrawalBlocker.
 * Repairs: useRecipientRecord crashed for the declared undefined destination;
 * recipientRecordFromResponse accepted replies with the wrong JSON-RPC version/id.
 * Hosts the account store and JSON-RPC listener in this process. The six
 * TypeScript route exports run unchanged over real HTTP and React Query;
 * account bytes are inputs to the route, not substituted decoder results.
 * useRecipientRecord accepts explicit RPC settings so this path needs no
 * deployment configuration. Repeated requests are counted at the listener.
 */
// @vitest-environment jsdom

import { solanaRecipientFixtureReopensThroughRpcAndHook } from './repair-22-33-266.proofs';

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { createServer } from 'node:http';
import { createElement, type PropsWithChildren } from 'react';
import { hexToBytes } from 'viem';

import { expect, test } from 'vitest';

import {
  base58Encode, parseRecipientRecord, recipientRecordAddress,
  recipientRecordFromResponse, recipientRecordRequest, solanaRecipientHandle,
} from '../lib/bridge/solana';
import { solanaWithdrawalBlocker, type RecipientLookup } from '../lib/bridge/solana-route';
import { useRecipientRecord } from '../lib/bridge/use-bridge-state';

function recipientQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false, gcTime: 0 } },
  });
}

function recipientQueryWrapper(client: QueryClient) {
  return function RecipientQueryProvider({ children }: PropsWithChildren) {
    return createElement(QueryClientProvider, { client }, children);
  };
}

async function proofSolanaRouteRefusesAnUndefinedDestination() {
  const client = recipientQueryClient();
  const hook = renderHook(() => useRecipientRecord(undefined, { programId: undefined, rpcUrl: undefined }), {
    wrapper: recipientQueryWrapper(client),
  });
  try {
    expect(hook.result.current.fetchStatus).toBe('idle');
    expect(hook.result.current.data).toBeUndefined();
    expect(solanaWithdrawalBlocker(hook.result.current.data, '')).toBe(
      'The Solana recipient record has not been verified. Wait for the lookup to succeed before withdrawing.',
    );
    await act(async() => {
      const result = await hook.result.current.refetch();
      expect(result.isError).toBe(true);
      expect(result.error?.message).toBe('The Solana recipient lookup is not configured or the destination is invalid.');
      expect(result.data).toBeUndefined();
    });
    expect(client.getQueryCache().getAll()[0].state.dataUpdateCount).toBe(0);
  } finally {
    hook.unmount();
    client.clear();
  }
}

async function proofSolanaRouteProducerTransportConsumer() {
  const programId = base58Encode(Uint8Array.from({ length: 32 }, (_, index) => index));
  const key = Uint8Array.from({ length: 32 }, (_, index) => index + 32);
  const destination = base58Encode(key);
  const handle = solanaRecipientHandle(destination);
  if (!handle) throw new Error('solana.ts solanaRecipientHandle must produce a handle for a 32-byte key.');
  const address = recipientRecordAddress(programId, handle);
  const request = recipientRecordRequest(programId, destination);
  if (!address || !request) throw new Error('solana.ts recipientRecordRequest must produce a recipient PDA request.');

  // Input layout is contracts/bridge/solana/src/state.rs RecipientRecord::encode:
  // magic, big-endian version, 20-byte handle, then the 32-byte recipient key.
  const record = new Uint8Array(62);
  record.set(new TextEncoder().encode('PXBRREC0'));
  new DataView(record.buffer).setUint16(8, 1, false);
  record.set(hexToBytes(handle), 10);
  record.set(key, 30);
  expect(parseRecipientRecord(record, handle)).toEqual(key);

  const accounts = new Map([ [ address.address, record ] ]);
  const received: Array<unknown> = [];
  const failures: Array<unknown> = [];
  let responseId: number = request.id;
  let responseVersion: string = request.jsonrpc;
  const server = createServer(async(incoming, outgoing) => {
    try {
      expect(incoming.method).toBe('POST');
      expect(incoming.headers['content-type']).toBe('application/json');
      let body = '';
      for await (const chunk of incoming) body += chunk.toString();
      const message: unknown = JSON.parse(body);
      received.push(message);
      // This listener is the producer for getAccountInfo over the in-process
      // account store. The HTTP client, JSON decoder and hook are never replaced.
      expect(message).toEqual(request);
      const data = accounts.get(address.address);
      outgoing.setHeader('Content-Type', 'application/json');
      outgoing.end(JSON.stringify({
        jsonrpc: responseVersion,
        id: responseId,
        result: {
          context: { slot: received.length },
          value: data === undefined ? null : {
            data: [ Buffer.from(data).toString('base64'), 'base64' ],
            owner: programId,
            executable: false,
            lamports: 1,
            rentEpoch: 0,
          },
        },
      }));
    } catch (error) {
      failures.push(error);
      outgoing.statusCode = 500;
      outgoing.end('Recipient account producer failed.');
    }
  });
  await new Promise<void>((resolve, reject) => {
    server.once('error', reject);
    server.listen(0, '127.0.0.1', resolve);
  });
  const listening = server.address();
  if (!listening || typeof listening === 'string') throw new Error('proof_solana_route.spec.ts must host its RPC producer.');
  const rpcUrl = `http://127.0.0.1:${ listening.port }`;
  const config = { programId, rpcUrl };
  const client = recipientQueryClient();
  const wrapper = recipientQueryWrapper(client);
  const hook = renderHook(() => useRecipientRecord(destination, config), { wrapper });
  try {
    expect(request).toEqual({
      jsonrpc: '2.0', id: 1, method: 'getAccountInfo', params: [ address.address, { encoding: 'base64' } ],
    });
    expect(address.address).not.toBe(destination);

    const response = await fetch(rpcUrl, {
      method: 'POST', headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(request), cache: 'no-store',
    });
    expect(response.ok).toBe(true);
    const payload: {
      jsonrpc: string;
      id: number;
      result: { value: { data: [string, string] } };
    } = await response.json();
    const lookup: RecipientLookup = recipientRecordFromResponse(payload, handle);
    expect(lookup.registered).toBe(true);
    if (!lookup.registered) throw new Error('proof_solana_route.spec.ts account producer must expose its stored recipient.');
    expect(lookup.key).toEqual(key);
    const bytes = Uint8Array.from(atob(payload.result.value.data[0]), (char) => char.charCodeAt(0));
    expect(parseRecipientRecord(bytes, handle)).toEqual(lookup.key);
    expect(base58Encode(lookup.key)).toBe(destination.trim());

    await waitFor(() => expect(hook.result.current.isSuccess).toBe(true));
    expect(hook.result.current.data).toEqual(lookup);
    expect(solanaWithdrawalBlocker(hook.result.current.data, destination)).toBeUndefined();
    const query = client.getQueryCache().getAll()[0];
    expect(query.state.dataUpdateCount).toBe(1);
    expect(received).toHaveLength(2);

    // A second explicit fetch executes the real query function; no seeded cache.
    await act(async() => {
      const second = await hook.result.current.refetch();
      expect(second.isSuccess).toBe(true);
      expect(second.data).toEqual(lookup);
      expect(solanaWithdrawalBlocker(second.data, destination)).toBeUndefined();
    });
    expect(query.state.dataUpdateCount).toBe(2);
    expect(received).toHaveLength(3);
    expect(received).toEqual([ request, request, request ]);

    const otherKey = lookup.key.slice();
    otherKey[0] ^= 1;
    const otherDestination = base58Encode(otherKey);
    const otherHandle = solanaRecipientHandle(otherDestination);
    if (!otherHandle) throw new Error('A second full-length public key must produce a handle.');
    expect(parseRecipientRecord(bytes, otherHandle)).toBeUndefined();
    expect(() => recipientRecordFromResponse(payload, otherHandle)).toThrow(
      'The Solana recipient record does not match the expected layout and handle.',
    );
    expect(solanaWithdrawalBlocker(hook.result.current.data, otherDestination)).toBe(
      'The registered Solana recipient key does not match the destination address.',
    );

    // The next HTTP response carries malformed account bytes into the hook's
    // real decoder, including after a successful value has entered its cache.
    const malformedBytes = bytes.slice();
    malformedBytes[0] ^= 1;
    accounts.set(address.address, malformedBytes);
    expect(parseRecipientRecord(malformedBytes, handle)).toBeUndefined();
    const malformedResponse = await fetch(rpcUrl, {
      method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request),
    });
    const malformed: unknown = await malformedResponse.json();
    expect(() => recipientRecordFromResponse(malformed, handle)).toThrow(
      'The Solana recipient record does not match the expected layout and handle.',
    );
    await act(async() => {
      const refused = await hook.result.current.refetch();
      expect(refused.isError).toBe(true);
      expect(refused.error?.message).toBe('The Solana recipient record does not match the expected layout and handle.');
      // bridge-card.tsx uses solanaWithdrawalBlocker with undefined on a lookup
      // error, so React Query's retained successful data must not permit a burn.
      expect(solanaWithdrawalBlocker(refused.isError ? undefined : refused.data, destination)).toBe(
        'The Solana recipient record has not been verified. Wait for the lookup to succeed before withdrawing.',
      );
    });
    expect(query.state.dataUpdateCount).toBe(2);
    expect(received).toHaveLength(5);

    accounts.set(address.address, record);
    for (const invalid of [ 'id', 'jsonrpc' ]) {
      responseId = invalid === 'id' ? request.id + 1 : request.id;
      responseVersion = invalid === 'jsonrpc' ? '1.0' : request.jsonrpc;
      const invalidResponse = await fetch(rpcUrl, {
        method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(request),
      });
      const invalidPayload: unknown = await invalidResponse.json();
      expect(() => recipientRecordFromResponse(invalidPayload, handle)).toThrow(
        'The Solana RPC returned an invalid recipient record response.',
      );
      await act(async() => {
        const refused = await hook.result.current.refetch();
        expect(refused.isError).toBe(true);
        expect(refused.error?.message).toBe('The Solana RPC returned an invalid recipient record response.');
      });
    }
    expect(received).toHaveLength(9);
    expect(query.state.dataUpdateCount).toBe(2);

    responseId = request.id;
    responseVersion = request.jsonrpc;
    accounts.delete(address.address);
    await act(async() => {
      const unregistered = await hook.result.current.refetch();
      expect(unregistered.isSuccess).toBe(true);
      expect(unregistered.data).toEqual({ registered: false });
      expect(solanaWithdrawalBlocker(unregistered.data, destination)).toBe(
        'Register this destination\'s recipient record on Solana before withdrawing.',
      );
    });
    expect(received).toHaveLength(10);
    expect(failures).toEqual([]);
  } finally {
    hook.unmount();
    client.clear();
    await new Promise<void>((resolve, reject) => {
      server.close((error) => error ? reject(error) : resolve());
      server.closeAllConnections();
    });
  }
}

test(proofSolanaRouteRefusesAnUndefinedDestination.name, proofSolanaRouteRefusesAnUndefinedDestination);
test(proofSolanaRouteProducerTransportConsumer.name, proofSolanaRouteProducerTransportConsumer);

test(solanaRecipientFixtureReopensThroughRpcAndHook.name, solanaRecipientFixtureReopensThroughRpcAndHook);
