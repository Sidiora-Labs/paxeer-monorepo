// @vitest-environment node

import { apiResolver } from 'next/dist/server/api-utils/node/api-resolver';
import nodeFetch from 'node-fetch';
import { createServer } from 'node:http';

import assistant from 'pages/api/assistant';
import { afterAll, beforeAll, beforeEach, afterEach, expect, test, vi } from 'vitest';

const server = createServer((req, res) => {
  void apiResolver(req, res, {}, { 'default': assistant }, {
    dev: false,
    previewModeId: 'assistant-test',
    previewModeEncryptionKey: 'assistant-test',
    previewModeSigningKey: 'assistant-test',
  }, false);
});
let origin: string;
const secret = 'assistant-test-secret';

beforeAll(async() => {
  await new Promise<void>(resolve => server.listen(0, '127.0.0.1', resolve));
  const address = server.address();
  if (!address || typeof address === 'string') throw new Error('Expected a local TCP listener');
  origin = `http://127.0.0.1:${ address.port }`;
});

beforeEach(() => {
  vi.stubEnv('EXPLORER_ASSISTANT_BASE_URL', 'https://assistant.example/v1');
  vi.stubEnv('EXPLORER_ASSISTANT_API_KEY', secret);
  vi.stubEnv('EXPLORER_ASSISTANT_MODEL', 'test/model');
  fetchMock.resetMocks();
  fetchMock.mockResponse(JSON.stringify({ choices: [ { message: { content: 'Paxeer chain id is 125.' } } ] }));
});

afterEach(() => vi.unstubAllEnvs());
afterAll(async() => {
  await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
});

function post(body: unknown) {
  return nodeFetch(`${ origin }/explorer/api/assistant`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
}

type CompletionRequest = {
  model: string;
  messages: Array<{ role: string; content: string }>;
};

function isCompletionRequest(value: unknown): value is CompletionRequest {
  return typeof value === 'object' && value !== null && 'model' in value && typeof value.model === 'string' &&
    'messages' in value && Array.isArray(value.messages) && value.messages.every((message: unknown) => (
    typeof message === 'object' && message !== null && 'role' in message && typeof message.role === 'string' &&
      'content' in message && typeof message.content === 'string'
  ));
}

function parseCompletionRequest(raw: string): CompletionRequest {
  const value: unknown = JSON.parse(raw);
  expect(isCompletionRequest(value)).toBe(true);
  if (!isCompletionRequest(value)) throw new Error('Invalid captured completion request');
  return value;
}

async function assistantRejectsOverlongQuestion(): Promise<void> {
  const response = await post({ question: 'q'.repeat(2001) });
  expect(response.status).toBe(400);
  expect(await response.json()).toEqual({ error: 'Provide a question of 1 to 2000 characters' });
  expect(fetchMock).not.toHaveBeenCalled();
}
test(assistantRejectsOverlongQuestion.name, assistantRejectsOverlongQuestion);

async function assistantReturns503WithoutBaseUrl(): Promise<void> {
  vi.stubEnv('EXPLORER_ASSISTANT_BASE_URL', '');
  const response = await post({ question: 'What is Paxeer?' });
  expect(response.status).toBe(503);
  expect(await response.json()).toEqual({ error: 'Assistant is not configured' });
  expect(fetchMock).not.toHaveBeenCalled();
}
test(assistantReturns503WithoutBaseUrl.name, assistantReturns503WithoutBaseUrl);

async function assistantForwardsModelBearerQuestionAndContext(): Promise<void> {
  const context = { route: '/address/0x123', entityKind: 'address', identifier: '0x123' };
  const response = await post({ question: 'What network is this?', context });
  expect(response.status).toBe(200);
  expect(response.headers.get('cache-control')).toBe('no-store');
  expect(await response.json()).toEqual({ answer: 'Paxeer chain id is 125.', model: 'test/model' });
  expect(fetchMock).toHaveBeenCalledTimes(1);
  const [ url, init ] = fetchMock.mock.calls[0];
  expect(url).toBe('https://assistant.example/v1/chat/completions');
  expect(init?.method).toBe('POST');
  expect(init?.headers).toEqual({ 'Content-Type': 'application/json', Authorization: `Bearer ${ secret }` });
  expect(init?.redirect).toBe('error');
  const body = parseCompletionRequest(String(init?.body));
  expect(body.model).toBe('test/model');
  expect(body.messages).toHaveLength(2);
  expect(body.messages[0].role).toBe('system');
  for (const text of [ 'Paxeer X Network', '125', 'api-mainnet-beta.paxeer.network', 'api1.mainnet-beta.paxeer.network',
    'api14.mainnet-beta.paxeer.network' ]) {
    expect(body.messages[0].content).toContain(text);
  }
  expect(body.messages[1]).toEqual({ role: 'user', content: `What network is this?\nExplorer context: ${ JSON.stringify(context) }` });
  expect(JSON.stringify(body)).not.toContain(secret);
}
test(assistantForwardsModelBearerQuestionAndContext.name, assistantForwardsModelBearerQuestionAndContext);

async function assistantStripsCredentialsFromUpstreamReply(): Promise<void> {
  fetchMock.mockResponse(JSON.stringify({
    api_key: secret,
    model: secret,
    choices: [ { message: { content: `Answer ${ secret } ${ secret }`, api_key: secret } } ],
  }));
  const response = await post({ question: 'Explain?' });
  expect(response.status).toBe(200);
  const payload = await response.text();
  expect(payload).not.toContain(secret);
  expect(JSON.parse(payload)).toEqual({ answer: 'Answer [redacted] [redacted]', model: 'test/model' });
}
test(assistantStripsCredentialsFromUpstreamReply.name, assistantStripsCredentialsFromUpstreamReply);

async function assistantUsesDefaultModelAndAcceptsBoundaryQuestion(): Promise<void> {
  vi.stubEnv('EXPLORER_ASSISTANT_MODEL', '');
  vi.stubEnv('EXPLORER_ASSISTANT_BASE_URL', 'https://assistant.example/v1/');
  const response = await post({ question: 'q'.repeat(2000) });
  expect(response.status).toBe(200);
  expect(await response.json()).toEqual({ answer: 'Paxeer chain id is 125.', model: 'xai/grok-4.7' });
  expect(parseCompletionRequest(String(fetchMock.mock.calls[0][1]?.body)).model).toBe('xai/grok-4.7');
  expect(fetchMock.mock.calls[0][0]).toBe('https://assistant.example/v1/chat/completions');
}
test(assistantUsesDefaultModelAndAcceptsBoundaryQuestion.name, assistantUsesDefaultModelAndAcceptsBoundaryQuestion);

async function assistantRejectsMalformedBodiesAndOtherMethods(): Promise<void> {
  for (const body of [ null, [], {}, { question: 125 }, { question: '  ' },
    { question: 'Hello?', context: [] }, { question: 'Hello?', context: { route: 125 } } ]) {
    expect((await post(body)).status).toBe(400);
  }
  const response = await nodeFetch(`${ origin }/explorer/api/assistant`);
  expect(response.status).toBe(405);
  expect(response.headers.get('allow')).toBe('POST');
  expect(fetchMock).not.toHaveBeenCalled();
}
test(assistantRejectsMalformedBodiesAndOtherMethods.name, assistantRejectsMalformedBodiesAndOtherMethods);

async function assistantReportsUpstreamFailuresWithoutLeakingDetails(): Promise<void> {
  for (const body of [ { error: secret }, { choices: [] }, { choices: [ { message: { content: '' } } ] } ]) {
    fetchMock.mockResponse(JSON.stringify(body));
    const response = await post({ question: 'Explain?' });
    expect(response.status).toBe(502);
    expect(await response.json()).toEqual({ error: 'Assistant request failed. Please try again later.' });
  }
  fetchMock.mockResponse(secret, { status: 401 });
  const response = await post({ question: 'Explain?' });
  expect(response.status).toBe(502);
  expect(await response.text()).not.toContain(secret);
  fetchMock.mockReject(new Error(secret));
  expect((await post({ question: 'Explain?' })).status).toBe(502);
}
test(assistantReportsUpstreamFailuresWithoutLeakingDetails.name, assistantReportsUpstreamFailuresWithoutLeakingDetails);
