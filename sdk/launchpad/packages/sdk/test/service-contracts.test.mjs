import { test } from 'node:test';
import assert from 'node:assert/strict';
import { createHash, randomUUID } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { SdkClient, Operations, HttpError, decodeV2Contract, streamTopicFilterDigest, routeStatus } from '../dist/index.js';
import { operationDefinitions } from '../dist/operations.gen.js';
import { startProducer, readProducerCases } from './producer-process.mjs';

function configuration(origin, session) {
  const streamOrigin = origin.replace(/^http/, 'ws');
  return { apiOrigin: origin, identityOrigin: origin, mediaOrigin: origin, streamOrigin, approvedStreamOrigins: [streamOrigin], production: false, ...(session ? { session } : {}) };
}

function browserFetch(application) {
  const cookies = new Map();
  const requests = [];
  const fetcher = async (input, init) => {
    const headers = new Headers(init.headers);
    headers.set('Origin', application);
    if (init.credentials === 'same-origin' && cookies.size) headers.set('Cookie', [...cookies].map(([name, value]) => `${name}=${value}`).join('; '));
    const response = await fetch(input, { ...init, headers });
    if (init.credentials === 'same-origin') {
      for (const cookie of response.headers.getSetCookie()) {
        const pair = cookie.split(';', 1)[0];
        const equals = pair.indexOf('=');
        cookies.set(pair.slice(0, equals), pair.slice(equals + 1));
      }
    }
    requests.push({ url: new URL(input), method: init.method, headers, body: init.body, status: response.status, response: response.headers.get('Content-Type')?.includes('application/json') ? await response.clone().text() : '' });
    return response;
  };
  return { cookies, requests, fetcher };
}

async function identityOperationsReadGatewayResponses() {
  const identity = await startProducer('gateway', 'internal/auth', './identity-producer_test.go.txt', 'TestSDKIdentityProducer');
  let reward;
  try {
    reward = await startProducer('pnl-tracker', 'internal/app', './reward-producer_test.go.txt', 'TestSDKRewardProducer');
    const browser = browserFetch(identity.application);
    const session = { mode: 'cookie', applicationOrigin: identity.origin, csrfToken: () => browser.cookies.get('__Host-kindle_csrf') ?? '' };
    const cookieOperations = new Operations(new SdkClient(configuration(identity.origin, session), browser.fetcher));
    const wallet = identity.wallets[0];
    async function login() {
      const challenge = await cookieOperations.call('IF_CHALLENGE', { body: { wallet, origin: identity.application } }, { headers: { 'Idempotency-Key': randomUUID() } });
      assert.equal(challenge.data.challenge.wallet, wallet);
      assert.equal(challenge.data.challenge.chainId, '125');
      assert.equal(Date.parse(challenge.data.challenge.expiresAt) - Date.parse(challenge.data.challenge.issuedAt), 300000);
      assert.deepEqual(challenge, JSON.parse(browser.requests.at(-1).response));
      const proof = await identity.sign(wallet, challenge.data.challenge.message);
      const result = await cookieOperations.call('IF_LOGIN', { body: { challengeId: challenge.data.challenge.challengeId, proof, deviceLabel: 'SDK browser' } });
      assert.equal(result.data.session.wallet, wallet);
      assert.match(result.data.accessToken, /^[^.]+\.[^.]+\.[^.]+$/);
      assert.equal(result.data.session.revokedAt, null);
      assert.deepEqual(result, JSON.parse(browser.requests.at(-1).response));
      return result;
    }
    const first = await login();
    const second = await login();
    const refreshed = await cookieOperations.call('IF_REFRESH', { body: {} });
    const refreshRequest = browser.requests.at(-1);
    assert.equal(refreshRequest.url.pathname, '/v2/auth/sessions:refresh');
    assert.equal(refreshRequest.method, 'POST');
    assert.equal(refreshRequest.body, '{}');
    assert.equal(refreshRequest.headers.get('Content-Type'), 'application/json');
    assert.ok(refreshRequest.headers.get('X-CSRF-Token'));
    assert.ok(refreshRequest.headers.get('Cookie').includes('__Host-kindle_refresh='));
    assert.equal(refreshed.data.session.sessionId, second.data.session.sessionId);
    assert.notEqual(refreshed.data.accessToken, second.data.accessToken);
    assert.deepEqual(refreshed, JSON.parse(refreshRequest.response));
    await assert.rejects(cookieOperations.call('IF_REFRESH', {}), /Invalid v2 contract RefreshRequest/);

    const bearerConfig = { ...configuration(identity.origin, { mode: 'bearer', token: () => refreshed.data.accessToken }), apiOrigin: reward.origin };
    const operations = new Operations(new SdkClient(bearerConfig, browser.fetcher));

    const topics = [`private.notifications.${wallet}`];
    const filterDigest = await streamTopicFilterDigest(topics);
    assert.equal(filterDigest, createHash('sha256').update(JSON.stringify(topics)).digest('hex'));
    const ticket = await operations.call('IF_STREAM_TICKET_ISSUE', { body: { topics, filterDigest, schemaMajor: 2, origin: identity.application, endpoint: '/v2/stream' } });
    assert.match(ticket.data.ticket, /^[0-9a-f]{64}$/);
    assert.equal(ticket.data.subprotocol, 'sidiora.v2');
    assert.ok(Date.parse(ticket.data.expiresAt) > Date.now());
    assert.equal(browser.requests.at(-1).headers.get('Authorization'), `Bearer ${refreshed.data.accessToken}`);
    assert.deepEqual(ticket, JSON.parse(browser.requests.at(-1).response));

    const linkedWallet = identity.wallets[1];
    const challenge = await operations.call('IF_WALLET_LINK_CHALLENGE', { body: { wallet: linkedWallet, chainId: '125', visibility: 'private' } }, { headers: { 'Idempotency-Key': randomUUID() } });
    assert.equal(challenge.data.challenge.wallet, linkedWallet);
    assert.equal(challenge.data.challenge.purpose, 'link_wallet');
    assert.equal(browser.requests.at(-1).url.origin, identity.origin);
    assert.deepEqual(challenge, JSON.parse(browser.requests.at(-1).response));
    const linkId = challenge.data.linkId;
    const proof = await identity.sign(linkedWallet, challenge.data.challenge.message);
    const verified = await operations.call('IF_WALLET_LINK_VERIFY', { path: { linkId }, body: { challengeId: challenge.data.challenge.challengeId, proof } }, { headers: { 'Idempotency-Key': randomUUID() } });
    assert.equal(verified.data.link.wallet, linkedWallet);
    assert.equal(verified.data.link.chainId, '125');
    assert.equal(verified.data.link.state, 'verified');
    assert.equal(verified.data.link.visibility, 'private');
    assert.equal(verified.data.link.linkId, linkId);
    assert.ok(Number.isFinite(Date.parse(verified.data.link.linkedAt)));
    assert.equal(browser.requests.at(-1).url.origin, identity.origin);
    assert.deepEqual(verified, JSON.parse(browser.requests.at(-1).response));

    async function ownerPost(path, body) {
      const response = await browser.fetcher(new URL(path, identity.origin), { method: 'POST', headers: { Authorization: `Bearer ${refreshed.data.accessToken}`, 'Content-Type': 'application/json' }, body: JSON.stringify(body) });
      assert.equal(response.ok, true, await response.clone().text());
      return response.json();
    }
    const stepUp = await ownerPost('/v2/auth/reauth/challenges', { action: 'wallet_unlink' });
    const reauthenticated = await ownerPost('/v2/auth/reauth:verify', { action: 'wallet_unlink', challengeId: stepUp.data.challenge.challengeId, proof: await identity.sign(wallet, stepUp.data.challenge.message) });
    assert.equal(await operations.call('IF_WALLET_LINK_REMOVE', { path: { linkId }, body: { reauthProof: reauthenticated.data.reauthProof } }, { headers: { 'Idempotency-Key': randomUUID() } }), undefined);
    assert.equal(browser.requests.at(-1).url.origin, identity.origin);
    assert.equal(browser.requests.at(-1).status, 204);
    assert.equal(browser.requests.at(-1).response, '');
    const beforeMissingHeader = browser.requests.length;
    await assert.rejects(operations.call('IF_WALLET_LINK_CHALLENGE', { body: { wallet: linkedWallet, chainId: '125', visibility: 'private' } }), /Missing IF_WALLET_LINK_CHALLENGE operation header Idempotency-Key/);
    assert.equal(browser.requests.length, beforeMissingHeader);

    assert.equal(await operations.call('IF_LOGOUT', { path: { sessionId: first.data.session.sessionId } }), undefined);
    assert.equal(browser.requests.at(-1).status, 204);
    const remaining = await operations.call('IF_SESSIONS', {});
    assert.equal(remaining.data.items.filter(item => item.revokedAt === null).length, 1);
    assert.equal(remaining.data.items.find(item => item.sessionId === first.data.session.sessionId).revokedAt !== null, true);
    const page = await operations.call('IF_SESSIONS', { query: { limit: 1 } });
    assert.equal(page.data.items.length, 1);
    assert.equal(page.data.items[0].wallet, wallet);
    assert.equal(typeof page.nextCursor, 'string');
    assert.ok(page.nextCursor.length <= 2048);
    assert.deepEqual(decodeV2Contract('SessionsRequest', { cursor: page.nextCursor, limit: 1 }), { cursor: page.nextCursor, limit: 1 });
    const next = await operations.call('IF_SESSIONS', { query: { limit: 1, cursor: page.nextCursor } });
    assert.equal(next.data.items.length, 1);
    assert.notEqual(page.data.items[0].sessionId, next.data.items[0].sessionId);
    assert.equal(next.nextCursor, null);
    assert.deepEqual(new Set([page.data.items[0].sessionId, next.data.items[0].sessionId]), new Set([first.data.session.sessionId, refreshed.data.session.sessionId]));
    const cursor = 'a'.repeat(2048);
    assert.deepEqual(decodeV2Contract('SessionsRequest', { cursor }), { cursor });
    await assert.rejects(operations.call('IF_SESSIONS', { query: { cursor } }), error => error instanceof HttpError && error.status === 400 && error.code === 'CURSOR_INVALID');
    assert.equal(browser.requests.at(-1).url.searchParams.get('cursor'), cursor);
    const beforeOversizedCursor = browser.requests.length;
    assert.throws(() => decodeV2Contract('SessionsRequest', { cursor: cursor + 'a' }), /Invalid v2 contract SessionsRequest/);
    await assert.rejects(operations.call('IF_SESSIONS', { query: { cursor: cursor + 'a' } }), /Invalid IF_SESSIONS query/);
    assert.equal(browser.requests.length, beforeOversizedCursor);

  } finally { await Promise.all([identity.close(), ...(reward ? [reward.close()] : [])]); }
}

test(identityOperationsReadGatewayResponses);

async function snapshotsAndEventsReadAPIResponses() {
  const identity = await startProducer('gateway', 'internal/auth', './identity-producer_test.go.txt', 'TestSDKIdentityProducer');
  let api;
  try {
    api = await startProducer('api', 'internal/rest', './api-producer_test.go.txt', 'TestSDKAPIProducer', {
      SDK_APPLICATION_ORIGIN: identity.application,
      SDK_IDENTITY_ORIGIN: identity.origin,
      SDK_IDENTITY_SERVICE_TOKEN: identity.serviceToken,
    });
    const browser = browserFetch(identity.application);
    const anonymous = new Operations(new SdkClient(configuration(api.origin), browser.fetcher));
    const topic = `market.stats.${api.pool}`;
    const publicSnapshot = await anonymous.call('IF_TOPIC_SNAPSHOT_READ', { query: { topics: [topic], cursor: null, normalizedFilters: { poolAddress: api.pool } } });
    assert.equal(browser.requests.at(-1).headers.has('Authorization'), false);
    assert.deepEqual(publicSnapshot, JSON.parse(browser.requests.at(-1).response));
    assert.equal(publicSnapshot.data.snapshot.topics[0].topic, topic);
    const response = await anonymous.call('IF_EVENTS_SSE', { query: { topics: [topic] } });
    assert.equal(response.headers.get('Content-Type'), 'text/event-stream');
    assert.equal(browser.requests.at(-1).url.pathname, '/v2/stream/sse');
    assert.equal(browser.requests.at(-1).headers.get('Accept'), 'text/event-stream');
    const frame = JSON.parse((await response.text()).split('\n').find(line => line.startsWith('data: ')).slice(6));
    assert.equal(decodeV2Contract('StreamControl', frame).op, 'resnapshot_required');
    assert.equal(frame.code, 'SNAPSHOT_REQUIRED');
    const resumed = await anonymous.call('IF_EVENTS_SSE', { query: { topics: [topic] } }, { headers: { 'Last-Event-ID': publicSnapshot.data.snapshot.resumeCursor } });
    assert.equal(resumed.status, 200);
    assert.equal(browser.requests.at(-1).headers.get('Last-Event-ID'), publicSnapshot.data.snapshot.resumeCursor);
    await resumed.body.cancel();

    const privateTopics = [`private.notifications.${identity.wallets[0]}`];
    const config = configuration(api.origin, { mode: 'bearer', token: () => randomUUID() });
    const authenticated = new Operations(new SdkClient(config, browser.fetcher));
    await assert.rejects(authenticated.call('IF_TOPIC_SNAPSHOT_READ', { query: { topics: privateTopics } }), error => error instanceof HttpError && error.status === 401 && error.code === 'AUTH_EXPIRED');
    assert.match(browser.requests.at(-1).headers.get('Authorization'), /^Bearer /);
    const before = browser.requests.length;
    await assert.rejects(anonymous.call('IF_TOPIC_SNAPSHOT_READ', { query: { topics: privateTopics } }), /Session required/);
    assert.equal(browser.requests.length, before);
    const publicWithSession = await authenticated.call('IF_TOPIC_SNAPSHOT_READ', { query: { topics: [topic] } });
    assert.equal(publicWithSession.data.snapshot.topics[0].topic, topic);
    assert.equal(browser.requests.at(-1).headers.has('Authorization'), false);
  } finally { if (api) await api.close(); await identity.close(); }
}

test(snapshotsAndEventsReadAPIResponses);

async function rewardClaimReadsRegisteredProducerError() {
  const producer = await startProducer('pnl-tracker', 'internal/app', './reward-producer_test.go.txt', 'TestSDKRewardProducer');
  try {
    const browser = browserFetch(producer.origin);
    const operations = new Operations(new SdkClient(configuration(producer.origin, { mode: 'bearer', token: () => randomUUID() }), browser.fetcher));
    const wallet = '0x' + createHash('sha256').update(randomUUID()).digest('hex').slice(0, 40);
    await assert.rejects(operations.call('IF_REWARD_CLAIM', { body: { recipient: wallet, rewardIds: ['creator:1'] } }, { headers: { 'Idempotency-Key': randomUUID() } }), error => {
      assert.ok(error instanceof HttpError);
      assert.equal(error.status, 503);
      assert.equal(error.code, 'DEPENDENCY_UNAVAILABLE');
      assert.equal(error.envelope.retryable, true);
      return true;
    });
    assert.equal(browser.requests.length, 1);
    assert.equal(browser.requests[0].url.pathname, '/v2/me/rewards:prepare-claim');
    assert.equal(browser.requests[0].method, 'POST');
    const absent = await fetch(new URL('/v2/me/rewards/claim', producer.origin), { method: 'POST' });
    assert.equal(absent.status, 404);
  } finally { await producer.close(); }
}

test(rewardClaimReadsRegisteredProducerError);

async function operationAnswerSchemasEqualServiceValidators(t) {
  const root = new URL('../../../../../', import.meta.url);
  const go = readFileSync(new URL('sdk/launchpad/shared/interfaces/v2/validators.gen.go', root), 'utf8');
  const literal = go.slice(go.indexOf('const schemaDocument = ') + 'const schemaDocument = '.length).split('\n', 1)[0];
  const service = JSON.parse(JSON.parse(literal)).$defs;
  const sdk = JSON.parse(readFileSync(new URL('../src/schemas-v2.gen.json', import.meta.url), 'utf8')).$defs;
  const api = JSON.parse(readFileSync(new URL('services/launchpad/api/internal/rest/testdata/contracts/v2/openapi.json', root), 'utf8')).components.schemas;
  function localRefs(value) {
    if (Array.isArray(value)) return value.map(localRefs);
    if (value && typeof value === 'object') return Object.fromEntries(Object.entries(value).map(([key, child]) => [key, key === '$ref' ? child.replace('#/components/schemas/', '#/$defs/') : localRefs(child)]));
    return value;
  }
  const expected = { ...service, ...localRefs(api) };
  expected.NotificationListItem = structuredClone(expected.Notification);
  Object.assign(expected.NotificationListItem.properties, { summary: { type: 'string', minLength: 1, maxLength: 512 }, read: { type: 'boolean' } });
  expected.NotificationListItem.required.push('summary', 'read');
  expected.NotificationsData = structuredClone(expected.NotificationsData);
  expected.NotificationsData.properties.items.items = { $ref: '#/$defs/NotificationListItem' };
  const seen = new Set();
  function compare(name) {
    if (seen.has(name)) return;
    seen.add(name);
    assert.ok(expected[name], `Service schema ${name}`);
    assert.deepEqual(sdk[name], expected[name], name);
    const refs = JSON.stringify(sdk[name]).matchAll(/"\$ref":"#\/\$defs\/([^"]+)"/g);
    for (const [, dependency] of refs) compare(dependency);
  }
  for (const [name, operation] of Object.entries(operationDefinitions)) {
    await t.test(name, function operationMatchesServiceValidator() {
      if (operation.response) {
        compare(operation.response);
        assert.deepEqual(sdk[operation.response], expected[operation.response], name);
      } else assert.equal(operation.emptyResponse || operation.binaryResponse, true, name);
      if (operation.body) compare(operation.body);
    });
  }
  for (const name of ['Error', 'StreamEvent', 'StreamControl', 'TopicSnapshot', 'Subscribe']) compare(name);
}

test(operationAnswerSchemasEqualServiceValidators);

async function signedInGateway(identity) {
  const browser = browserFetch(identity.application);
  const operations = new Operations(new SdkClient(configuration(identity.origin), browser.fetcher));
  const wallet = identity.wallets[0];
  const challenge = await operations.call('IF_CHALLENGE', { body: { wallet, origin: identity.application } }, { headers: { 'Idempotency-Key': randomUUID() } });
  const login = await operations.call('IF_LOGIN', { body: { challengeId: challenge.data.challenge.challengeId, proof: await identity.sign(wallet, challenge.data.challenge.message), deviceLabel: 'SDK services' } });
  const response = await browser.fetcher(new URL('/v2/auth/session', identity.origin), { method: 'GET', headers: { Authorization: `Bearer ${login.data.accessToken}` } });
  assert.equal(response.status, 200);
  const principal = (await response.json()).data;
  return { browser, login, principal, env: {
    SDK_IDENTITY_ORIGIN: identity.origin,
    SDK_IDENTITY_SERVICE_TOKEN: identity.serviceToken,
    SDK_ACCOUNT_ID: principal.accountId,
    SDK_SESSION_ID: principal.sessionId,
    SDK_WALLET: wallet,
  } };
}

async function ownerOperationsReadUserResponses() {
  const identity = await startProducer('gateway', 'internal/auth', './identity-producer_test.go.txt', 'TestSDKIdentityProducer');
  let user;
  try {
    const { browser, login, principal, env } = await signedInGateway(identity);
    user = await startProducer('user', 'internal/account', './user-producer_test.go.txt', 'TestSDKUserProducer', env);
    const config = { ...configuration(identity.origin, { mode: 'bearer', token: () => login.data.accessToken }), mediaOrigin: user.origin };
    const client = new SdkClient(config, browser.fetcher);
    const operations = new Operations(client);
    const initial = await operations.call('IF_PREFERENCES_READ', {});
    assert.equal(initial.data.preferences.locale, 'en');
    const updated = await operations.call('IF_PREFERENCES_UPDATE', { body: { expectedRevision: initial.data.preferences.revision, locale: 'fr-CA', timezone: 'UTC', channels: { email: false, inapp: true, push: true } } });
    assert.equal(updated.data.preferences.locale, 'fr-CA');
    assert.deepEqual(updated.data.preferences.channels, ['inapp', 'push']);
    assert.deepEqual((await operations.call('IF_PREFERENCES_READ', {})).data.preferences, updated.data.preferences);
    const device = await operations.call('IF_DEVICE_REGISTER', { path: { deviceId: user.deviceId }, body: user.deviceBody }, { headers: { 'Idempotency-Key': randomUUID() } });
    assert.equal(device.data.deviceId, user.deviceId);
    assert.equal(device.data.state, 'active');
    assert.deepEqual(device, JSON.parse(browser.requests.at(-1).response));
    assert.equal(await operations.call('IF_DEVICE_REVOKE', { path: { deviceId: user.deviceId } }, { headers: { 'Idempotency-Key': randomUUID() } }), undefined);
    assert.equal(browser.requests.at(-1).status, 204);
    const exported = await operations.call('IF_ACCOUNT_EXPORT', { body: { format: 'json', includeMedia: false } }, { headers: { 'Idempotency-Key': randomUUID() } });
    assert.equal(exported.data.job.accountId, principal.accountId);
    assert.equal(exported.data.job.state, 'queued');
    const status = await operations.call('IF_ACCOUNT_EXPORT_STATUS', { path: { jobId: exported.data.job.jobId } });
    assert.deepEqual(status.data.job, exported.data.job);
    assert.equal(status.data.downloadUrl, null);
    await assert.rejects(operations.call('IF_ACCOUNT_DELETE', { body: { confirmAccountId: principal.accountId, expectedRevision: '0', reauthProof: randomUUID() } }, { headers: { 'Idempotency-Key': randomUUID() } }), error => error instanceof HttpError && error.status === 503 && error.code === 'DEPENDENCY_UNAVAILABLE');
    const absentJob = randomUUID();
    await assert.rejects(operations.call('IF_ACCOUNT_DELETE_STATUS', { path: { jobId: absentJob } }), error => error instanceof HttpError && error.status === 404 && error.code === 'NOT_FOUND');
    const recovery = new Operations(new SdkClient({ ...configuration(identity.origin), mediaOrigin: user.origin }, browser.fetcher));
    await assert.rejects(recovery.call('IF_ACCOUNT_DELETE_STATUS', { path: { jobId: absentJob } }, { headers: { 'X-Recovery-Receipt': randomUUID() } }), error => error instanceof HttpError && error.status === 403 && error.code === 'SCOPE_DENIED');
    assert.equal(browser.requests.at(-1).headers.has('Authorization'), false);
    assert.ok(browser.requests.at(-1).headers.get('X-Recovery-Receipt'));
    const start = browser.requests.length;
    await assert.rejects(client.request(`/v1/me/exports/${absentJob}`, 'AccountExportStatusResponse', { authenticated: true, service: 'media', idempotentRead: true }), error => {
      assert.ok(error instanceof HttpError);
      assert.equal(error.status, 404);
      assert.equal(error.code, 'NOT_FOUND');
      assert.equal(error.envelope.retryable, false);
      assert.equal(error.envelope.details, null);
      return true;
    });
    assert.equal(browser.requests.length, start + 1);
    assert.equal(browser.requests.at(-1).url.origin, user.origin);
    for (const request of browser.requests.filter(request => request.url.pathname.startsWith('/v2/me/'))) assert.equal(request.url.origin, user.origin, request.url.pathname);
  } finally { if (user) await user.close(); await identity.close(); }
}

test(ownerOperationsReadUserResponses);

async function squadsAlertsAndNotificationsReadServiceResponses() {
  const identity = await startProducer('gateway', 'internal/auth', './identity-producer_test.go.txt', 'TestSDKIdentityProducer');
  let discovery;
  try {
    const { browser, login, principal, env } = await signedInGateway(identity);
    discovery = await startProducer('notifications', '.', './discovery-producer_test.go.txt', 'TestSDKDiscoveryProducer', env);
    const operations = new Operations(new SdkClient({ ...configuration(discovery.origin, { mode: 'bearer', token: () => login.data.accessToken }), identityOrigin: identity.origin }, browser.fetcher));
    const squad = await operations.call('IF_SQUAD_CREATE', { body: { name: 'SDK research', description: 'Market research', visibility: 'private', joinPolicy: 'open' } }, { headers: { 'Idempotency-Key': randomUUID() } });
    assert.equal(squad.data.squad.memberCount, '1');
    assert.equal(squad.data.squad.name, 'SDK research');
    const squadId = squad.data.squad.squadId;
    const read = await operations.call('IF_SQUAD_READ', { path: { squadId } });
    assert.equal(read.data.squad.squadId, squadId);
    const invited = randomUUID();
    const membership = await operations.call('IF_SQUAD_MEMBERSHIP', { path: { squadId, accountId: invited }, body: { action: 'invite', expectedRevision: '0' } }, { headers: { 'Idempotency-Key': randomUUID() } });
    assert.equal(membership.data.membership.accountId, invited);
    assert.equal(membership.data.membership.state, 'invited');
    assert.equal(membership.data.squadRevision, read.data.squad.revision);
    assert.equal(membership.data.membership.revision, '1');
    assert.deepEqual(membership, JSON.parse(browser.requests.at(-1).response));

    const recorded = JSON.parse(readFileSync(new URL('../../../../../services/launchpad/api/internal/rest/testdata/contracts/v2/examples.json', import.meta.url), 'utf8'));
    const stats = recorded.endpoints.IF_STATS.successes[0].body.data.stats;
    const poolAddress = stats.poolAddress;
    const price = { numerator: '1', denominator: '1', baseAsset: { kind: 'erc20', chainId: '125', address: poolAddress, currencyCode: null, decimals: 18 }, quoteAsset: { kind: 'fiat_display', chainId: null, address: null, currencyCode: 'USD', decimals: 2 }, source: { kind: 'external_display', amountBasis: 'provider_display', sourceEventId: null, position: null, providerId: 'reference', observedAt: stats.windowEnd, sourceReference: 'reference' } };
    const created = await operations.call('IF_ALERT_CREATE', { body: { poolAddress, direction: 'crosses_above', threshold: price, cooldownSeconds: 300 } }, { headers: { 'Idempotency-Key': randomUUID() } });
    assert.equal(created.data.alert.poolAddress, poolAddress);
    assert.equal(created.data.alert.wallet, identity.wallets[0]);
    assert.deepEqual(created.data.alert.threshold, price);
    assert.equal(await operations.call('IF_ALERT_DELETE', { path: { alertId: created.data.alert.alertId } }), undefined);
    assert.equal(browser.requests.at(-1).status, 204);

    const first = await operations.call('IF_NOTIFICATIONS', { query: { limit: 2, unreadOnly: false } });
    assert.equal(first.data.items.length, 2);
    assert.equal(typeof first.nextCursor, 'string');
    const second = await operations.call('IF_NOTIFICATIONS', { query: { limit: 2, cursor: first.nextCursor } });
    assert.equal(second.data.items.length, 1);
    assert.equal(second.nextCursor, null);
    const ids = [...first.data.items, ...second.data.items].map(item => item.notificationId);
    assert.equal(new Set(ids).size, 3);
    for (const item of [...first.data.items, ...second.data.items]) {
      assert.equal(item.wallet, identity.wallets[0]);
      assert.equal(item.readAt, null);
      assert.equal(item.kind, 'price_alert');
      assert.equal(item.summary, 'Threshold crossed');
      assert.equal(item.read, false);
    }
    const marked = await operations.call('IF_NOTIFICATION_READ', { path: { notificationId: ids[0] } });
    assert.ok(Number.isFinite(Date.parse(marked.data.readAt)));
    assert.deepEqual(marked, JSON.parse(browser.requests.at(-1).response));
    const unread = await operations.call('IF_NOTIFICATIONS', { query: { unreadOnly: true } });
    assert.equal(unread.data.items.length, 2);
    assert.equal(unread.data.items.some(item => item.notificationId === ids[0]), false);
  } finally { if (discovery) await discovery.close(); await identity.close(); }
}

test(squadsAlertsAndNotificationsReadServiceResponses);

async function voiceOperationsReadServiceAnswers() {
  const records = await readProducerCases('social', 'internal/social', './voice-answers_test.go.txt', 'TestSDKVoiceAnswers');
  assert.deepEqual(records.map(record => record.operation).sort(), ['IF_VOICE_END', 'IF_VOICE_LEAVE', 'IF_VOICE_READ']);
  for (const { operation, route, body } of records) {
    const definition = operationDefinitions[operation];
    assert.equal(routeStatus(operation), 'served');
    assert.equal(definition.service, 'media');
    assert.equal(`${definition.method} ${definition.path}`, route);
    const decoded = decodeV2Contract(definition.response, body);
    assert.deepEqual(decoded, body);
    assert.equal(decoded.quality, 'complete');
    assert.equal(decoded.watermark, null);
    assert.equal(decoded.nextCursor, null);
    if (operation === 'IF_VOICE_LEAVE') {
      assert.equal(decoded.data.grantState, 'revoked');
      assert.equal(decoded.data.revision, '2');
    } else {
      assert.equal(decoded.data.room.state, 'ended');
      assert.equal(decoded.data.room.revision, '3');
      assert.equal(decoded.data.room.visibility, 'private');
    }
  }
}

test(voiceOperationsReadServiceAnswers);

async function socialOperationsReadGatewayServiceAnswers(t) {
  const records = await readProducerCases('social', 'internal/app', './social-answers_test.go.txt', 'TestSDKSocialAnswers');
  const names = Object.keys(operationDefinitions).filter(name => /^(IF_BOUNT|IF_CALLOUT|IF_COMMENT)/.test(name) || name === 'IF_REPORT_CREATE');
  assert.deepEqual(records.map(record => record.operation).sort(), names.sort());
  for (const record of records) {
    await t.test(record.operation, function socialOperationReadsProducerAnswer() {
      const definition = operationDefinitions[record.operation];
      assert.equal(definition.service, 'media');
      assert.equal(definition.method, record.method);
      const route = new RegExp('^' + definition.path.replace(/\{[^}]+\}/g, '[^/]+') + '$');
      assert.match(record.path.split('?')[0], route);
      if (record.status >= 400) {
        const error = decodeV2Contract('Error', record.body);
        assert.equal(error.code, 'NOT_FOUND');
        assert.equal(error.retryable, false);
      } else if (definition.emptyResponse) {
        assert.equal(record.status, 204);
        assert.equal(record.body, null);
      } else {
        const answer = decodeV2Contract(definition.response, record.body);
        assert.deepEqual(answer, record.body);
        assert.equal(answer.quality, 'complete');
        assert.equal(answer.watermark, null);
        assert.equal(answer.revision, null);
      }
    });
  }
}

test(socialOperationsReadGatewayServiceAnswers);
