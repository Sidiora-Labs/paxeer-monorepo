// @vitest-environment jsdom

import { ChakraProvider } from '@chakra-ui/react';
import { focusManager, onlineManager, QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import fetch from 'node-fetch';
import { createServer } from 'node:http';
import React from 'react';

import { inferenceQueryKey } from 'lib/api/services/general/paxeerXInference';
import { inferenceResponse } from 'pages/api/inference';
import theme from 'toolkit/theme/theme';
import { afterEach, expect, test } from 'vitest';

import PaxeerXInference from './PaxeerXInference';

afterEach(cleanup);

test('inference deal validation follows interaction', function inferenceDealValidationFollowsInteraction() {
  const client = new QueryClient();
  client.setQueryData(inferenceQueryKey('px_getInferenceRooms'), []);
  try {
    render(
      <ChakraProvider value={ theme }>
        <QueryClientProvider client={ client }><PaxeerXInference/></QueryClientProvider>
      </ChakraProvider>,
    );
    const input = screen.getByLabelText('Deal ID');
    const submit = screen.getByRole('button', { name: 'Look up deal' });
    expect(input.getAttribute('aria-invalid')).toBe('false');
    expect(screen.queryByRole('alert')).toBeNull();

    fireEvent.change(input, { target: { value: 'invalid' } });
    expect(screen.queryByRole('alert')).toBeNull();
    submit.focus();
    fireEvent.click(submit);
    expect(input.getAttribute('aria-invalid')).toBe('true');
    expect(document.activeElement).toBe(input);
    expect(screen.getByRole('alert').textContent).toBe('Enter a nonzero deal ID of 64 lowercase hexadecimal characters.');
    expect(input.getAttribute('aria-describedby')?.split(' ')).toContain(screen.getByRole('alert').id);
    expect(client.isFetching()).toBe(0);

    fireEvent.change(input, { target: { value: '01'.repeat(32) } });
    expect(input.getAttribute('aria-invalid')).toBe('false');
    expect(screen.queryByRole('alert')).toBeNull();
    expect(input.getAttribute('aria-describedby')).toBe('inference-deal-hint');
  } finally {
    client.clear();
  }
});

test('inference inactive market stops reads', async function inferenceInactiveMarketStopsReads() {
  const answer = inferenceResponse('px_getInferenceRooms', [], {
    jsonrpc: '2.0', id: 1,
    error: { code: -32001, message: 'Paxeer read unavailable', data: { code: 'inference_program_not_configured' } },
  });
  const requests: Array<{ url: string | undefined; method: string | undefined; body: unknown }> = [];
  const server = createServer(async(req, res) => {
    const chunks: Array<string> = [];
    req.setEncoding('utf8');
    for await (const chunk of req) {
      if (typeof chunk !== 'string') throw new Error('Expected a UTF-8 request body');
      chunks.push(chunk);
    }
    requests.push({ url: req.url, method: req.method, body: JSON.parse(chunks.join('')) });
    res.statusCode = answer.status;
    res.setHeader('Content-Type', 'application/json');
    res.end(JSON.stringify(answer.body));
  });
  const previousFetch = globalThis.fetch;
  const client = new QueryClient();
  await new Promise<void>((resolve) => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Expected a local TCP listener');
  const origin = `http://127.0.0.1:${ address.port }`;
  globalThis.fetch = async(input, init) => {
    const response = await fetch(new URL(String(input), origin).toString(), {
      method: init?.method,
      headers: { 'Content-Type': 'application/json' },
      body: typeof init?.body === 'string' ? init.body : undefined,
      signal: init?.signal,
    });
    return new Response(await response.text(), { status: response.status });
  };
  try {
    const page = (
      <ChakraProvider value={ theme }>
        <QueryClientProvider client={ client }><PaxeerXInference/></QueryClientProvider>
      </ChakraProvider>
    );
    const view = render(page);
    await screen.findByText('The inference market is not active on this network.');
    expect(client.getQueryData(inferenceQueryKey('px_getInferenceRooms'))).toEqual({
      state: 'not_active', code: 'inference_program_not_configured',
    });
    expect(screen.getByRole('heading', { name: 'Inference' })).toBeTruthy();
    expect(screen.getByText('Room markets, layer assignments and settled receipt batches.')).toBeTruthy();
    expect(screen.getByText('The inference market is not active on this network.').getAttribute('role')).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();
    expect(screen.queryByRole('table')).toBeNull();
    expect(screen.queryByRole('textbox')).toBeNull();
    expect(screen.queryByRole('button', { name: 'Look up deal' })).toBeNull();
    expect(view.container.querySelector('form')).toBeNull();
    await act(async() => {
      await client.invalidateQueries({ queryKey: inferenceQueryKey('px_getInferenceRooms') });
      focusManager.setFocused(false);
      focusManager.setFocused(true);
      onlineManager.setOnline(false);
      onlineManager.setOnline(true);
    });
    view.unmount();
    render(page);
    await waitFor(() => expect(client.isFetching()).toBe(0));
    expect(requests).toEqual([ {
      url: '/explorer/api/inference', method: 'POST', body: { method: 'px_getInferenceRooms', params: [] },
    } ]);
    expect(client.getQueryCache().getAll().every((query) => query.queryKey[1] === 'px_getInferenceRooms' ||
      query.state.fetchStatus === 'idle')).toBe(true);
  } finally {
    cleanup();
    client.clear();
    globalThis.fetch = previousFetch;
    focusManager.setFocused(undefined);
    onlineManager.setOnline(true);
    await new Promise<void>((resolve, reject) => server.close((error) => error ? reject(error) : resolve()));
  }
});
