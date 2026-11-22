import assert from 'node:assert/strict';
import { createHash } from 'node:crypto';
import { generatePrivateKey, privateKeyToAccount } from 'viem/accounts';
import { verifyMessage } from 'viem';

const account = privateKeyToAccount(generatePrivateKey());
export const wallet = account.address.toLowerCase();
export const auth = { signMessage: message => account.signMessage({ message }) };
export const imageDigest = body => createHash('sha256').update(body).digest('hex');

export async function assertOperation(request, action, intent = request.body) {
  const encoded = request.headers['x-kindle-operation'];
  const raw = Buffer.from(encoded, 'base64url').toString('utf8');
  const envelope = JSON.parse(raw);
  assert.equal(envelope.version, 1);
  assert.equal(envelope.chainId, '125');
  assert.equal(envelope.environment, 'launchpad-user-legacy');
  assert.equal(envelope.actor, wallet);
  assert.equal(envelope.action, action);
  assert.equal(envelope.method, request.method);
  assert.equal(envelope.path, request.path ?? request.url);
  assert.equal(envelope.bodyHash, imageDigest(intent));
  assert.match(envelope.nonce, /^[a-f0-9]{64}$/);
  assert.ok(envelope.issuedAt <= Math.floor(Date.now() / 1000));
  assert.equal(envelope.expiresAt - envelope.issuedAt, 300);
  assert.equal(Buffer.from(raw).toString('base64url'), encoded);
  assert.equal(await verifyMessage({ address: wallet, message: `KindleLaunch signed operation v1\n${raw}`, signature: request.headers['x-kindle-operation-signature'] }), true);
}
