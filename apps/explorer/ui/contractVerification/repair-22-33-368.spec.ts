// @vitest-environment jsdom

import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { act, renderHook, waitFor } from '@testing-library/react';
import { once } from 'node:events';
import type { IncomingMessage, ServerResponse } from 'node:http';
import { createServer } from 'node:http';
import React from 'react';
import { useForm } from 'react-hook-form';

import type { FormFields, FormFieldsFlattenSourceCode } from './types';
import type { SmartContractVerificationResponse } from 'types/api/contract';

import { SocketProvider, useSocket } from 'lib/socket/context';
import { afterAll, beforeAll, expect, test, vi } from 'vitest';
import { createTestSocket } from 'vitest/utils/socketServer';

import type { default as UseVerificationSubmission, VerificationSubmission } from './useVerificationSubmission';
let useVerificationSubmission: typeof UseVerificationSubmission;
let verificationPollInterval: number;
let verificationTimeout: number;

const ADDRESS = '0x1234567890123456789012345678901234567890';
const TOPIC = `addresses:${ ADDRESS }`;
const fields: FormFieldsFlattenSourceCode = {
  address: ADDRESS,
  method: [ 'flattened-code' ],
  license_type: [ 'mit' ],
  code: 'contract Example {}',
  name: 'Example',
  compiler: [ 'v0.8.20' ],
  evm_version: [ 'default' ],
  is_yul: false,
  is_optimization_enabled: false,
  optimization_runs: '200',
  autodetect_constructor_args: true,
  constructor_args: '',
  libraries: [],
};

let handleRequest: ((request: IncomingMessage, response: ServerResponse) => Promise<void>) | undefined;
const server = createServer((request, response) => {
  if (!handleRequest) {
    response.writeHead(500);
    response.end('No active verification fixture');
    return;
  }
  void handleRequest(request, response);
});
const initialEnvs = window.__envs;

beforeAll(async() => {
  fetchMock.disableMocks();
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const serverAddress = server.address();
  if (!serverAddress || typeof serverAddress === 'string') {
    throw new Error('Expected a loopback address');
  }
  window.__envs = {
    ...initialEnvs,
    NEXT_PUBLIC_API_PROTOCOL: 'http',
    NEXT_PUBLIC_API_HOST: '127.0.0.1',
    NEXT_PUBLIC_API_PORT: String(serverAddress.port),
    NEXT_PUBLIC_API_BASE_PATH: '',
    NEXT_PUBLIC_USE_NEXT_JS_PROXY: 'false',
  };
  const production = await import('./useVerificationSubmission');
  useVerificationSubmission = production.default;
  verificationPollInterval = production.VERIFICATION_POLL_INTERVAL;
  verificationTimeout = production.VERIFICATION_TIMEOUT;
});

afterAll(async() => {
  window.__envs = initialEnvs;
  fetchMock.enableMocks();
  await new Promise<void>((resolve, reject) => {
    server.close(error => error ? reject(error) : resolve());
    server.closeAllConnections();
  });
});

async function openVerification() {
  const socket = await createTestSocket();
  const requests: Array<{ method: string | undefined; url: string | undefined; body: string }> = [];
  const store = {
    verified: false,
    partial: false,
    readFailures: 0,
    postStatus: 200,
    holdPost: false,
    held: undefined as ServerResponse | undefined,
    resultDuringPost: undefined as SmartContractVerificationResponse | undefined,
  };
  handleRequest = async(request, response) => {
    request.setEncoding('utf8');
    let body = '';
    for await (const chunk of request) {
      body += chunk;
    }
    requests.push({ method: request.method, url: request.url, body });
    response.setHeader('content-type', 'application/json');
    if (request.method === 'POST') {
      if (store.resultDuringPost) {
        socket.send(TOPIC, 'verification_result', store.resultDuringPost);
      }
      if (store.holdPost) {
        store.held = response;
        return;
      }
      response.writeHead(store.postStatus);
      response.end(JSON.stringify(store.postStatus === 200 ? {} : { message: 'Verifier is unavailable' }));
      return;
    }
    if (store.readFailures > 0) {
      store.readFailures--;
      response.writeHead(503);
      response.end(JSON.stringify({ message: 'Status temporarily unavailable' }));
      return;
    }
    response.end(JSON.stringify({ is_verified: store.verified, is_partially_verified: store.partial }));
  };
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });

  function wrapper({ children }: React.PropsWithChildren) {
    return React.createElement(QueryClientProvider, { client }, React.createElement(SocketProvider, { url: socket.url, children }));
  }

  function useVerificationForm() {
    const connection = useSocket();
    const formApi = useForm<FormFields>({ defaultValues: fields });
    const submissionRef = React.useRef<VerificationSubmission>(undefined);
    const [ verifiedAddress, setVerifiedAddress ] = React.useState('');
    const submit = useVerificationSubmission({
      submissionRef, address: ADDRESS, isDisabled: false, formApi, onSuccess: setVerifiedAddress,
    });
    const { isSubmitting, errors } = formApi.formState;
    return { submit: formApi.handleSubmit(submit), isSubmitting, errors, verifiedAddress, submissionRef, connection, getFieldState: formApi.getFieldState };
  }

  let mounted = renderHook(useVerificationForm, { wrapper });
  await waitFor(() => expect(mounted.result.current.connection?.channelRegistry.current[TOPIC]?.channel.state).toBe('joined'));
  let socketClosed = false;

  return {
    store, socket, requests,
    get mounted() {
      return mounted;
    },
    async submit() {
      let completion: Promise<void> | undefined;
      act(() => {
        completion = mounted.result.current.submit();
      });
      await waitFor(() => expect(requests.filter(request => request.method === 'POST')).not.toHaveLength(0));
      return { completion: completion! };
    },
    async disconnect() {
      socketClosed = true;
      await socket.close();
    },
    async reopen() {
      mounted = renderHook(useVerificationForm, { wrapper });
      await waitFor(() => expect(mounted.result.current.connection?.channelRegistry.current[TOPIC]?.channel.state).toBe('joined'));
    },
    async close() {
      vi.useRealTimers();
      mounted.unmount();
      client.clear();
      if (!socketClosed) {
        await socket.close();
      }
      server.closeAllConnections();
      handleRequest = undefined;
    },
  };
}

async function verificationCompletionReleasesOnSocketClose() {
  for (const holdPost of [ false, true ]) {
    const fixture = await openVerification();
    try {
      fixture.store.holdPost = holdPost;
      const { completion } = await fixture.submit();
      await waitFor(() => expect(fixture.mounted.result.current.isSubmitting).toBe(true));
      await act(async() => {
        await fixture.disconnect();
      });
      await waitFor(() => expect(fixture.mounted.result.current.isSubmitting).toBe(false));
      await completion;
      expect(fixture.mounted.result.current.errors.root?.message).toContain('Verification connection lost');
      expect(fixture.mounted.result.current.submissionRef.current).toBeUndefined();
      expect(fixture.mounted.result.current.verifiedAddress).toBe('');
    } finally {
      await fixture.close();
    }
  }
}

async function verificationCompletionDisplaysPostFailure() {
  const fixture = await openVerification();
  try {
    fixture.store.postStatus = 503;
    const { completion } = await fixture.submit();
    await act(async() => {
      await completion;
    });
    expect(fixture.mounted.result.current.errors.root?.message).toBe('Verifier is unavailable');
    expect(fixture.mounted.result.current.isSubmitting).toBe(false);
    expect(fixture.mounted.result.current.submissionRef.current).toBeUndefined();
    expect(fixture.requests).toHaveLength(1);
    expect(fixture.requests[0].url).toBe(`/api/v2/smart-contracts/${ ADDRESS }/verification/via/flattened-code`);
    expect(JSON.parse(fixture.requests[0].body)).toMatchObject({ source_code: fields.code, compiler_version: fields.compiler[0] });
  } finally {
    await fixture.close();
  }
}

async function verificationCompletionHandlesResultBeforePostReturns() {
  const fixture = await openVerification();
  try {
    fixture.store.holdPost = true;
    fixture.store.resultDuringPost = { status: 'error', errors: { contract_source_code: [ 'Compilation failed' ] } };
    const { completion } = await fixture.submit();
    await waitFor(() => expect(fixture.mounted.result.current.getFieldState('code').error?.message).toBe('Compilation failed'));
    await completion;
    expect(fixture.mounted.result.current.isSubmitting).toBe(false);
    expect(fixture.mounted.result.current.submissionRef.current).toBeUndefined();
  } finally {
    await fixture.close();
  }
}

async function verificationCompletionConsumesSocketSuccessOnce() {
  const fixture = await openVerification();
  try {
    const { completion } = await fixture.submit();
    await act(async() => {
      fixture.socket.send(TOPIC, 'verification_result', { status: 'success' });
    });
    await waitFor(() => expect(fixture.mounted.result.current.verifiedAddress).toBe(ADDRESS));
    await completion;
    expect(fixture.mounted.result.current.isSubmitting).toBe(false);
    await act(async() => {
      fixture.socket.send(TOPIC, 'verification_result', { status: 'error', errors: { name: [ 'Late result' ] } });
    });
    expect(fixture.mounted.result.current.errors).toEqual({});
  } finally {
    await fixture.close();
  }
}

async function verificationCompletionRecoversLostResultByPolling() {
  const fixture = await openVerification();
  try {
    fixture.store.verified = true;
    fixture.store.partial = true;
    const { completion } = await fixture.submit();
    await waitFor(() => expect(fixture.requests.some(request => request.method === 'GET')).toBe(true), { timeout: 5000 });
    expect(fixture.mounted.result.current.isSubmitting).toBe(true);
    expect(fixture.mounted.result.current.verifiedAddress).toBe('');
    fixture.store.partial = false;
    fixture.store.readFailures = 1;
    await waitFor(() => expect(fixture.requests.filter(request => request.method === 'GET')).toHaveLength(2), { timeout: 5000 });
    expect(fixture.mounted.result.current.isSubmitting).toBe(true);
    await waitFor(() => expect(fixture.mounted.result.current.errors.root?.message).toContain('Retrying'));
    await waitFor(() => expect(fixture.mounted.result.current.verifiedAddress).toBe(ADDRESS), { timeout: 5000 });
    await completion;
    expect(fixture.mounted.result.current.isSubmitting).toBe(false);
    expect(fixture.requests.filter(request => request.method === 'GET').every(request => request.url === `/api/v2/smart-contracts/${ ADDRESS }`)).toBe(true);
  } finally {
    await fixture.close();
  }
}

async function verificationCompletionBoundsMissingResults() {
  const fixture = await openVerification();
  try {
    fixture.store.holdPost = true;
    vi.useFakeTimers({ toFake: [ 'setTimeout', 'clearTimeout' ] });
    let bounded: Promise<void> | undefined;
    await act(async() => {
      bounded = fixture.mounted.result.current.submit();
    });
    await act(async() => {
      await vi.advanceTimersByTimeAsync(verificationTimeout);
    });
    await bounded;
    expect(fixture.mounted.result.current.isSubmitting).toBe(false);
    expect(fixture.mounted.result.current.errors.root?.message).toContain('Verification result is taking too long');
    expect(fixture.mounted.result.current.submissionRef.current).toBeUndefined();
  } finally {
    await fixture.close();
  }
}

async function verificationCompletionUnmountReleasesAndReopens() {
  const fixture = await openVerification();
  try {
    fixture.store.holdPost = true;
    const { completion } = await fixture.submit();
    const pending = fixture.mounted.result.current.submissionRef.current;
    expect(pending).toBeDefined();
    fixture.mounted.unmount();
    await completion;
    expect(pending?.controller.signal.aborted).toBe(true);
    fixture.store.holdPost = false;
    await fixture.reopen();
    expect(fixture.mounted.result.current.isSubmitting).toBe(false);
    expect(fixture.mounted.result.current.errors).toEqual({});
    let second: Promise<void> | undefined;
    act(() => {
      second = fixture.mounted.result.current.submit();
    });
    await waitFor(() => expect(fixture.requests.filter(request => request.method === 'POST')).toHaveLength(2));
    await act(async() => {
      fixture.socket.send(TOPIC, 'verification_result', { status: 'success' });
    });
    await waitFor(() => expect(fixture.mounted.result.current.verifiedAddress).toBe(ADDRESS));
    await second;
    expect(fixture.mounted.result.current.isSubmitting).toBe(false);
    expect(verificationPollInterval).toBeLessThan(verificationTimeout);
  } finally {
    await fixture.close();
  }
}

test(verificationCompletionReleasesOnSocketClose, { timeout: 15000 }, verificationCompletionReleasesOnSocketClose);
test(verificationCompletionDisplaysPostFailure, verificationCompletionDisplaysPostFailure);
test(verificationCompletionHandlesResultBeforePostReturns, verificationCompletionHandlesResultBeforePostReturns);
test(verificationCompletionConsumesSocketSuccessOnce, verificationCompletionConsumesSocketSuccessOnce);
test(verificationCompletionRecoversLostResultByPolling, { timeout: 15000 }, verificationCompletionRecoversLostResultByPolling);
test(verificationCompletionBoundsMissingResults, { timeout: 15000 }, verificationCompletionBoundsMissingResults);
test(verificationCompletionUnmountReleasesAndReopens, verificationCompletionUnmountReleasesAndReopens);
