import assert from 'node:assert/strict';
import { privateKeyToAccount, generatePrivateKey } from 'viem/accounts';
import { UsersHttpClient, UsersApiError } from '../dist/index.js';

const account = privateKeyToAccount(process.env.USERS_SDK_PRIVATE_KEY);
const wallet = account.address.toLowerCase();
const pool = `0x${'b'.repeat(40)}`;
const token = `0x${'c'.repeat(40)}`;
const baseUrl = process.env.USERS_SDK_BASE_URL;
const client = new UsersHttpClient({ baseUrl });
let signed;
const signer = {
  async signMessage(message) {
    const signature = await account.signMessage({ message });
    signed = { message, signature };
    return signature;
  },
};
const data = {
  display_name: 'Alice <&> 🐈', bio: 'Creator\nProfile', website: 'https://example.com/?a=1&b=2',
  twitter: '@alice', telegram: '', discord: 'alice',
};
const image = Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+/aXsAAAAASUVORK5CYII=', 'base64');
const file = new Blob([image], { type: 'image/png' });
const emptyWallet = privateKeyToAccount(generatePrivateKey()).address.toLowerCase();

function apiError(status, body) {
  return error => {
    assert.ok(error instanceof UsersApiError);
    assert.equal(error.status, status);
    assert.deepEqual(error.body, body);
    return true;
  };
}

async function requestError(call, status, rule) {
  await assert.rejects(call(), error => {
    assert.ok(error instanceof UsersApiError);
    assert.equal(error.status, status);
    assert.equal(error.body.code, 'FIELD_INVALID');
    assert.match(error.body.requestId, /^[a-f0-9-]{36}$/);
    assert.equal(error.body.retryable, false);
    assert.equal(error.body.retryAfterMs, null);
    assert.equal(error.body.details, null);
    assert.deepEqual(error.body.fieldErrors, [{ pointer: '', rule }]);
    assert.equal(typeof error.body.message, 'string');
    return true;
  });
}

async function profileRead() {
  const profile = await client.getProfile(wallet);
  assert.equal(profile.wallet_address, wallet);
  assert.equal(profile.display_name, data.display_name);
  assert.equal(profile.bio, data.bio);
  assert.deepEqual(profile.socials, { website: data.website, twitter: data.twitter, telegram: null, discord: data.discord });
  assert.deepEqual(profile.created_pools, [{ pool_address: pool, token_address: token, created_at: 1700000000 }]);
  assert.ok(Number.isSafeInteger(profile.created_at));
  assert.ok(profile.created_at > 1700000000 && profile.created_at < Date.now() / 1000 + 1);
  assert.ok(profile.updated_at >= profile.created_at);
  return profile;
}

async function imageReads(profile) {
  for (const kind of ['avatar', 'banner']) {
    const path = `/users/${wallet}/${kind}`;
    assert.equal(profile.images[kind], path);
    assert.equal(client.imageUrl(` ${wallet.toUpperCase()} `, kind), baseUrl + path);
    const response = await fetch(client.imageUrl(wallet, kind));
    assert.equal(response.status, 200);
    assert.equal(response.headers.get('content-type'), 'image/png');
    assert.equal(response.headers.get('cache-control'), 'public, no-cache, max-age=0, must-revalidate');
    assert.equal(Number(response.headers.get('content-length')), image.length);
    assert.deepEqual(Buffer.from(await response.arrayBuffer()), image);
  }
}

async function watchlistRead() {
  const list = await client.getWatchlist(wallet);
  assert.equal(list.wallet_address, wallet);
  assert.equal(list.pools.length, 1);
  assert.equal(list.pools[0].pool_address, pool);
  assert.ok(Number.isSafeInteger(list.pools[0].added_at));
  assert.ok(list.pools[0].added_at > 1700000000 && list.pools[0].added_at < Date.now() / 1000 + 1);
}

async function serviceCalls() {
  if (process.env.USERS_SDK_STAGE === 'read') {
    await imageReads(await profileRead());
    await watchlistRead();
    return;
  }
  assert.deepEqual(await client.getProfile(emptyWallet), {
    wallet_address: emptyWallet, display_name: null, bio: null,
    socials: { website: null, twitter: null, telegram: null, discord: null },
    images: { avatar: null, banner: null }, created_pools: [], created_at: null, updated_at: null,
  });
  assert.deepEqual(await client.getWatchlist(wallet), { wallet_address: wallet, pools: [] });
  const absent = await fetch(client.imageUrl(wallet, 'avatar'));
  assert.equal(absent.status, 404);
  assert.deepEqual(await absent.json(), { statusCode: 404, error: 'Not Found', message: 'No avatar found' });
  assert.deepEqual(await client.updateProfile(` ${wallet.toUpperCase()} `, { data }, signer), { success: true });
  const consent = signed;
  const headers = {
    'Content-Type': 'application/json',
    'X-Kindle-Operation': Buffer.from(consent.message.split('\n')[1]).toString('base64url'),
    'X-Kindle-Operation-Signature': consent.signature,
  };
  const replay = await fetch(`${baseUrl}/users/${wallet}`, { method: 'POST', headers, body: JSON.stringify({ data }) });
  assert.equal(replay.status, 403);
  assert.deepEqual(await replay.json(), { statusCode: 403, error: 'Forbidden', message: 'Operation already used' });
  const changed = await fetch(`${baseUrl}/users/${wallet}`, { method: 'POST', headers, body: JSON.stringify({ data: { ...data, bio: 'Changed' } }) });
  assert.equal(changed.status, 403);
  assert.deepEqual(await changed.json(), { statusCode: 403, error: 'Forbidden', message: 'Operation content mismatch' });
  assert.deepEqual((await profileRead()).images, { avatar: null, banner: null });

  assert.deepEqual(await client.addWatchlist(wallet, pool.toUpperCase(), signer), { success: true });
  await watchlistRead();
  assert.deepEqual(await client.addWatchlist(wallet, pool, signer), { success: true });
  await watchlistRead();
  assert.deepEqual(await client.removeWatchlist(wallet, pool, signer), { success: true });
  assert.deepEqual(await client.getWatchlist(wallet), { wallet_address: wallet, pools: [] });
  assert.deepEqual(await client.addWatchlist(wallet, pool, signer), { success: true });

  for (const kind of ['avatar', 'banner']) {
    assert.deepEqual(await client.uploadImage(wallet, kind, file, signer), { success: true, url: `/users/${wallet}/${kind}` });
  }
  await imageReads(await profileRead());
  await requestError(() => client.uploadImage(wallet, 'avatar', new Blob(['text'], { type: 'text/plain' }), signer), 415, 'media_type');
  await requestError(() => client.uploadImage(wallet, 'avatar', new Blob([new Uint8Array(2 * 1024 * 1024 + 1)], { type: 'image/png' }), signer), 413, 'max_bytes');
  await assert.rejects(client.uploadImage(wallet, 'banner', new Blob(['<svg><script/></svg>'], { type: 'image/svg+xml' }), signer), apiError(400, { statusCode: 400, error: 'Bad Request', message: 'SVG rejected: contains script, event handlers, or unsafe elements' }));
  const other = privateKeyToAccount(generatePrivateKey());
  await assert.rejects(client.removeWatchlist(wallet, pool, { signMessage: message => other.signMessage({ message }) }), apiError(403, { statusCode: 403, error: 'Forbidden', message: 'Invalid signed operation' }));
  await watchlistRead();
  const prefixed = new UsersHttpClient({ baseUrl: `${baseUrl}/media&user/` });
  assert.deepEqual(await prefixed.updateProfile(wallet, { data }, signer), { success: true });
  assert.equal(JSON.parse(signed.message.split('\n')[1]).path, `/media&user/users/${wallet}`);
  assert.ok(signed.message.includes('media\\u0026user'));
  assert.equal((await prefixed.getProfile(wallet)).display_name, data.display_name);
}

await serviceCalls();
