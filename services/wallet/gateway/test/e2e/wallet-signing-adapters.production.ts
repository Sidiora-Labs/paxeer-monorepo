import assert from 'node:assert/strict';
import {
  readFileSync,
  lstatSync,
  writeFileSync,
  openSync,
  closeSync,
  fsyncSync,
  renameSync,
} from 'node:fs';
import { request as httpsRequest } from 'node:https';
import { join, resolve, sep } from 'node:path';
import { pathToFileURL, fileURLToPath } from 'node:url';
import type { PaxeerProvider, PaxeerProviderConfig } from '../../../../../sdk/wallet/src/index.js';
import { randomUUID, createHash } from 'node:crypto';
import {
  keccak256,
  recoverAddress,
  serializeTransaction,
  parseTransaction,
  recoverTransactionAddress,
  type Hex,
} from 'viem';
import { Pool } from 'pg';

interface Runtime {
  gateway_url: string;
  rpc_url: string;
  real_rpc_url: string;
  control_url: string;
  sdk_entry: string;
  evidence_dir: string;
  fixture_dir: string;
  database_url: string;
  owner_token_file: string;
  foreign_owner_token_file: string;
  owner_subject: string;
  account: Hex;
  chain_id: number;
  sponsor: Hex;
  paymaster: Hex;
  quote_url: string;
  sponsor_call: { to: Hex; data: Hex; value: string };
  sponsor_maximum: string;
  sponsor_gas_cost: string;
  attestor_key_id: string;
  attestors: { id: string; url: string }[];
  attestor_ca_file: string;
  gateway_cert_file: string;
  gateway_key_file: string;
}
export async function walletSigningProvider(config: PaxeerProviderConfig): Promise<PaxeerProvider> {
  const { PaxeerProvider } = await import('../../../../../sdk/wallet/src/index.js');
  return new PaxeerProvider(config);
}

export type SigningAdapterRequest = (
  path: string,
  body?: unknown,
  token?: string | null,
  method?: 'POST' | 'GET'
) => Promise<{ status: number; body: Record<string, unknown> }>;

export interface EvmSigningReview {
  id: string;
  signing_hash: Hex;
  transaction: Hex;
  principal: string;
  key_id: string;
  session_id: string;
  participants: string[];
  state: string;
}

export async function assertRetainedSigningAuthentication(
  request: SigningAdapterRequest,
  ownerToken: string,
  invalidToken: string
): Promise<void> {
  for (const token of [null, ownerToken, invalidToken]) {
    const retired = await request(
      '/v1/wallet/sign-custody',
      {
        custody: '0x4c583a435553544f44593a7631' + '00'.repeat(32),
      },
      token
    );
    assert.equal(retired.status, 404);
  }
  for (const path of [
    '/v1/wallet/evm/review',
    '/v1/wallet/evm/approve',
    '/v1/wallet/evm/sign',
    '/v1/wallet/sign-digest',
  ]) {
    for (const token of [null, invalidToken]) {
      assert.equal((await request(path, {}, token)).status, 401);
    }
    assert.equal((await request(path, {}, ownerToken)).status, 400);
  }
  assert.equal(
    (
      await request(
        '/v1/wallet/sign-digest',
        {
          digest: '0x' + '01'.repeat(32),
        },
        ownerToken
      )
    ).status,
    400
  );
}

export async function reviewAndApproveEvmSigning(
  request: SigningAdapterRequest,
  transaction: Hex,
  ownerToken: string,
  foreignToken: string
): Promise<EvmSigningReview> {
  const reviewed = await request('/v1/wallet/evm/review', { transaction }, ownerToken);
  assert.equal(reviewed.status, 200);
  const review = reviewed.body;
  assert.equal(review.state, 'reviewed');
  assert.equal(review.signing_hash, keccak256(transaction));
  assert(typeof review.id === 'string');
  assert(typeof review.principal === 'string');
  assert(typeof review.key_id === 'string');
  assert(typeof review.session_id === 'string');
  assert(typeof review.transaction === 'string');
  assert(
    Array.isArray(review.participants) && review.participants.every((id) => typeof id === 'string')
  );
  assert.equal(review.participants.length, 3);
  assert.equal(new Set(review.participants).size, 3);
  const decoded = parseTransaction(transaction);
  assert.equal(
    review.transaction,
    serializeTransaction(decoded, { r: '0x0', s: '0x0', yParity: 0 })
  );
  assert.equal(review.chain_id, decoded.chainId);
  assert(typeof review.to === 'string');
  assert.equal(review.to.toLowerCase(), decoded.to!.toLowerCase());
  assert.equal(review.value, (decoded.value ?? 0n).toString());
  assert.equal(review.data, decoded.data ?? '0x');
  const id = review.id;
  const approval = { review_id: id, signing_hash: review.signing_hash, decision: 'approve' };
  assert.equal((await request('/v1/wallet/evm/sign', { approval_id: id }, ownerToken)).status, 403);
  for (const [path, body, method] of [
    ['/v1/wallet/evm/approve', approval, 'POST'],
    ['/v1/wallet/evm/sign', { approval_id: id }, 'POST'],
    [`/v1/wallet/evm/approvals/${id}`, undefined, 'GET'],
  ] as const) {
    assert.equal((await request(path, body, foreignToken, method)).status, 404);
  }
  const changed = serializeTransaction({ ...decoded, maxFeePerGas: decoded.maxFeePerGas! + 1n });
  const refused = await request(
    '/v1/wallet/evm/approve',
    {
      ...approval,
      signing_hash: keccak256(changed),
    },
    ownerToken
  );
  assert.equal(refused.status, 409);
  assert.equal(refused.body.error, 'evm_review_changed');
  const approved = await request('/v1/wallet/evm/approve', approval, ownerToken);
  assert.equal(approved.status, 200);
  assert.deepEqual(approved.body, {
    ...review,
    state: 'approved',
    approval: {
      version: 1,
      principal: review.principal,
      key_id: review.key_id,
      session_id: review.session_id,
      signing_hash: review.signing_hash,
      expires_at: review.expires_at,
    },
  });
  assert.equal(
    (
      await request(
        '/v1/wallet/evm/sign',
        {
          approval_id: id,
          transaction: changed,
        },
        ownerToken
      )
    ).status,
    400
  );
  return {
    id,
    signing_hash: review.signing_hash as Hex,
    transaction: review.transaction as Hex,
    principal: review.principal,
    key_id: review.key_id,
    session_id: review.session_id,
    participants: review.participants,
    state: 'approved',
  };
}

export async function recoverEvmSigningReceipt(
  request: SigningAdapterRequest,
  review: EvmSigningReview,
  hash: Hex,
  ownerToken: string,
  foreignToken: string
): Promise<Record<string, unknown>> {
  const path = `/v1/wallet/evm/approvals/${review.id}`;
  assert.equal((await request(path, undefined, null, 'GET')).status, 401);
  assert.equal((await request(path, undefined, foreignToken, 'GET')).status, 404);
  const recovered = await request(path, undefined, ownerToken, 'GET');
  assert.equal(recovered.status, 200);
  assert.equal(recovered.body.id, review.id);
  assert.equal(recovered.body.signing_hash, review.signing_hash);
  assert.equal(recovered.body.state, 'signed');
  assert.equal(recovered.body.tx_hash, hash);
  assert(['receipted', 'committed'].includes(String(recovered.body.rung)));
  const receipt = recovered.body.receipt;
  assert(receipt && typeof receipt === 'object' && !Array.isArray(receipt));
  assert.equal((receipt as Record<string, unknown>).txHash, hash);
  assert.equal((receipt as Record<string, unknown>).status, '0x1');
  assert.equal((receipt as Record<string, unknown>).rung, recovered.body.rung);
  const repeated = await request('/v1/wallet/evm/sign', { approval_id: review.id }, ownerToken);
  assert.equal(repeated.status, 200);
  assert.deepEqual(repeated.body, recovered.body);
  return recovered.body;
}

export async function walletSigningAdaptersProduction(runtime: Runtime): Promise<void> {
  function secret(name: string): string {
    const path = resolve(runtime.fixture_dir, name);
    assert(
      path.startsWith(resolve(runtime.fixture_dir) + sep),
      'fixture material must stay in the protected bundle'
    );
    const stat = lstatSync(path);
    assert(
      stat.isFile() && !stat.isSymbolicLink() && stat.nlink === 1 && (stat.mode & 0o077) === 0
    );
    assert.equal(stat.uid, process.getuid!());
    return readFileSync(path, 'utf8').trim();
  }
  const ownerToken = secret(runtime.owner_token_file);
  const foreignToken = secret(runtime.foreign_owner_token_file);
  const sdk = (await import(
    pathToFileURL(runtime.sdk_entry).href
  )) as typeof import('../../../../../sdk/wallet/src/index.js');
  const cases: string[] = [];
  const pool = new Pool({ connectionString: runtime.database_url });
  const submissionStore = {
    getItem(key: string): string | null {
      const path = join(
        runtime.evidence_dir,
        'submission-' + createHash('sha256').update(key).digest('hex') + '.json'
      );
      try {
        const info = lstatSync(path);
        assert(
          info.isFile() &&
            !info.isSymbolicLink() &&
            info.uid === process.getuid!() &&
            (info.mode & 0o077) === 0
        );
        return readFileSync(path, 'utf8');
      } catch (error) {
        if ((error as NodeJS.ErrnoException).code === 'ENOENT') return null;
        throw error;
      }
    },
    setItem(key: string, value: string): void {
      const path = join(
        runtime.evidence_dir,
        'submission-' + createHash('sha256').update(key).digest('hex') + '.json'
      );
      const temporary = path + '.' + randomUUID();
      const fd = openSync(temporary, 'wx', 0o600);
      try {
        writeFileSync(fd, value);
        fsyncSync(fd);
      } finally {
        closeSync(fd);
      }
      renameSync(temporary, path);
      const directory = openSync(runtime.evidence_dir, 'r');
      try {
        fsyncSync(directory);
      } finally {
        closeSync(directory);
      }
    },
  };
  let lastSponsorBody: Record<string, unknown> | undefined;
  let lastSponsorResponse: Record<string, unknown> | undefined;
  const realFetch: typeof fetch = async (input, init) => {
    const response = await fetch(input, init);
    if (String(input).endsWith('/v1/wallet/sponsored/submit')) {
      assert.equal(typeof init?.body, 'string');
      lastSponsorBody = JSON.parse(init!.body as string) as Record<string, unknown>;
      lastSponsorResponse = (await response.clone().json()) as Record<string, unknown>;
    }
    return response;
  };
  function provider() {
    return walletSigningProvider({
      gatewayUrl: runtime.gateway_url,
      rpcUrl: runtime.rpc_url,
      chainId: runtime.chain_id,
      token: () => ownerToken,
      fetch: realFetch,
      confirm: async (request) => {
        assert(request.method && Array.isArray(request.params));
        return true;
      },
    });
  }
  async function http(
    path: string,
    body?: unknown,
    token: string | null = ownerToken,
    method: 'POST' | 'GET' = 'POST'
  ) {
    const response = await fetch(runtime.gateway_url + path, {
      method,
      headers: {
        'content-type': 'application/json',
        ...(token === null ? {} : { authorization: `Bearer ${token}` }),
      },
      ...(body === undefined ? {} : { body: JSON.stringify(body) }),
    });
    return { status: response.status, body: (await response.json()) as Record<string, any> };
  }
  async function rpc(method: string, params: unknown[], real = false): Promise<any> {
    const response = await fetch(real ? runtime.real_rpc_url : runtime.rpc_url, {
      method: 'POST',
      headers: { 'content-type': 'application/json' },
      body: JSON.stringify({ jsonrpc: '2.0', id: randomUUID(), method, params }),
    });
    const body = (await response.json()) as { error?: unknown; result?: unknown };
    assert(!body.error, 'real chain refused ' + method);
    return body.result;
  }
  async function control(action: string) {
    const response = await fetch(runtime.control_url + '/control/' + action, {
      method: 'POST',
      body: '{}',
    });
    assert(response.ok);
    return response.json() as Promise<Record<string, any>>;
  }
  async function eventually<T>(read: () => Promise<T>, accept: (value: T) => boolean): Promise<T> {
    const end = Date.now() + 90_000;
    while (Date.now() < end) {
      const value = await read();
      if (accept(value)) return value;
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
    throw new Error('real production evidence deadline exceeded');
  }
  async function attestor(node: Runtime['attestors'][number], body: unknown) {
    const url = new URL('/v1/sign', node.url);
    const data = Buffer.from(JSON.stringify(body));
    return await new Promise<{ status: number; body: Record<string, any> }>((resolve, reject) => {
      const request = httpsRequest(
        url,
        {
          method: 'POST',
          minVersion: 'TLSv1.3',
          ca: secret(runtime.attestor_ca_file),
          cert: secret(runtime.gateway_cert_file),
          key: secret(runtime.gateway_key_file),
          headers: {
            'content-type': 'application/json',
            'content-length': data.length,
            authorization: `Bearer ${ownerToken}`,
          },
        },
        (response) => {
          const chunks: Buffer[] = [];
          response.on('data', (chunk) => chunks.push(Buffer.from(chunk)));
          response.on('end', () =>
            resolve({
              status: response.statusCode!,
              body: JSON.parse(Buffer.concat(chunks).toString('utf8')),
            })
          );
          response.on('error', reject);
        }
      );
      request.on('error', reject);
      request.end(data);
    });
  }

  try {
    assert.equal(runtime.attestors.length, 5);
    const p = await provider();
    const wallet = new sdk.WalletInterface(p);
    const accounts = await wallet.accounts();
    assert.equal(accounts[0]!.toLowerCase(), runtime.account.toLowerCase());
    await assertRetainedSigningAuthentication(http, ownerToken, 'invalid-token');
    const nonce = Number(
      BigInt(await rpc('eth_getTransactionCount', [runtime.account, 'pending']))
    );
    const gasLimit = BigInt(
      await rpc('eth_estimateGas', [
        {
          from: runtime.account,
          to: runtime.sponsor,
          value: '0x1',
          data: '0x',
        },
      ])
    );
    const tip = BigInt(await rpc('eth_maxPriorityFeePerGas', []));
    const transaction = serializeTransaction({
      type: 'eip1559',
      chainId: runtime.chain_id,
      nonce,
      to: runtime.sponsor,
      value: 1n,
      data: '0x',
      gas: gasLimit,
      maxFeePerGas: BigInt(await rpc('eth_gasPrice', [])) * 2n + tip,
      maxPriorityFeePerGas: tip,
    });
    const review = await reviewAndApproveEvmSigning(http, transaction, ownerToken, foreignToken);
    assert.equal(review.principal, runtime.owner_subject);
    assert.equal(review.key_id, runtime.attestor_key_id);
    await control('arm-evm');
    const evmSigned = await http('/v1/wallet/evm/sign', { approval_id: review.id });
    assert.equal(evmSigned.status, 200);
    assert.equal(evmSigned.body.state, 'signed');
    const hash = evmSigned.body.tx_hash as Hex;
    assert(/^0x[0-9a-fA-F]{64}$/.test(hash));
    assert.equal(evmSigned.body.rung, 'submitted');
    assert.equal(evmSigned.body.receipt, null);
    assert.equal((await control('state')).dropped_reply, true);
    await control('restart');
    await control('receipts');
    await eventually(
      async () =>
        (await http(`/v1/wallet/evm/approvals/${review.id}`, undefined, ownerToken, 'GET')).body,
      (answer) => ['receipted', 'committed'].includes(String(answer.rung))
    );
    const recovered = await recoverEvmSigningReceipt(http, review, hash, ownerToken, foreignToken);
    const journal = await pool.query<{
      evidence: { raw: Hex; result: { audit: { node_id: string; audit_sequence: number }[] } };
    }>("select evidence from wallet_approval_recovery where domain='evm' and approval_id=$1", [
      review.id,
    ]);
    assert.equal(journal.rows.length, 1);
    const raw = journal.rows[0]!.evidence.raw;
    assert.equal(keccak256(raw), hash);
    assert.equal(
      (await recoverTransactionAddress({ serializedTransaction: raw })).toLowerCase(),
      runtime.account.toLowerCase()
    );
    const decoded = parseTransaction(raw);
    assert.equal(
      serializeTransaction({
        ...decoded,
        r: undefined,
        s: undefined,
        v: undefined,
        yParity: undefined,
      }),
      transaction
    );
    const audit = journal.rows[0]!.evidence.result.audit;
    assert.equal(audit.length, 3);
    assert.deepEqual(audit.map((entry) => entry.node_id).sort(), [...review.participants].sort());
    assert(audit.every((entry) => entry.audit_sequence > 0));
    assert.equal(recovered.tx_hash, hash);
    const receipt = await rpc('eth_getTransactionReceipt', [hash], true);
    assert.equal(receipt.status, '0x1');
    assert.equal(receipt.from.toLowerCase(), runtime.account.toLowerCase());
    assert.equal(receipt.to.toLowerCase(), runtime.sponsor.toLowerCase());
    cases.push(
      'mounted-authentication',
      'retired-custody-absence',
      'evm-review-approve-sign',
      'original-owner-binding',
      'evm-exact-approved-fee-bound',
      'actual-selected-quorum-audit',
      'lost-broadcast-reply',
      'retained-restart-idempotency',
      'receipt-only-confirmation'
    );
    const resumed = await provider();
    await new sdk.WalletInterface(resumed).accounts();
    const signers = runtime.attestors.slice(0, 3).map((node) => node.id);

    const gas = sdk.gasStation(resumed, {
      chainId: BigInt(runtime.chain_id),
      sponsor: runtime.sponsor,
      paymaster: runtime.paymaster,
      quoteUrl: runtime.quote_url,
      gatewayUrl: runtime.gateway_url,
      accessToken: () => ownerToken,
      fetch: realFetch,
      submissionStore,
    });
    const calls = [
      {
        to: runtime.sponsor_call.to,
        value: BigInt(runtime.sponsor_call.value),
        data: runtime.sponsor_call.data,
      },
    ];
    const batchNonce = await gas.batchNonce(runtime.account);
    const signed = await gas.requestQuote({
      account: runtime.account,
      nonce: batchNonce,
      calls,
      maxTokenAmount: BigInt(runtime.sponsor_maximum),
      gasCost: BigInt(runtime.sponsor_gas_cost),
    });
    const batch = {
      chainId: BigInt(runtime.chain_id),
      account: runtime.account,
      nonce: batchNonce,
      calls,
      quote: signed.quote,
    };
    const fields = gas.construction(batch);
    const digest = gas.digest(batch);
    const digestSignature = (await gas.sign(batch)) as Hex;
    assert.equal(
      (await recoverAddress({ hash: digest, signature: digestSignature })).toLowerCase(),
      runtime.account.toLowerCase()
    );
    const tampered = JSON.parse(JSON.stringify(fields));
    tampered.account = '0x0000000000000000000000000000000000000001';
    const refusedOwner = await http('/v1/wallet/sign-digest', { construction: tampered });
    assert([400, 403].includes(refusedOwner.status));
    tampered.account = fields.account;
    tampered.quote.deadline = '0';
    const refusedExpiry = await http('/v1/wallet/sign-digest', { construction: tampered });
    assert([400, 403].includes(refusedExpiry.status));
    const changedDigest = await Promise.all(
      runtime.attestors.slice(0, 3).map((node) =>
        attestor(node, {
          session_id: randomUUID(),
          key_id: runtime.attestor_key_id,
          signers,
          kind: 'eth_sign_digest',
          digest: '00'.repeat(32),
          construction: fields,
        })
      )
    );
    assert(
      changedDigest.every(
        (answer) => answer.status === 403 && answer.body.error?.policy_code === 'digest_mismatch'
      ),
      'daemon did not independently recompute the digest'
    );
    let confirmedConsent = false;
    const sponsoredHash = await gas.submitFirstUse(batch, signed.relayerSignature, {
      confirm: async (consent) => {
        assert.equal(consent.batchDigest.toLowerCase(), digest.toLowerCase());
        confirmedConsent = true;
        return true;
      },
    });
    assert(confirmedConsent && /^0x[0-9a-fA-F]{64}$/.test(sponsoredHash));
    assert(lastSponsorBody?.authorization && lastSponsorBody?.quote_decimals === 6);
    assert.equal(lastSponsorResponse?.tx_hash, sponsoredHash);
    assert(['pending', 'confirmed'].includes(String(lastSponsorResponse?.status)));
    const exactSubmission = lastSponsorBody!;
    await control('restart');
    const restartedProvider = await provider();
    await new sdk.WalletInterface(restartedProvider).accounts();
    const restartedGas = sdk.gasStation(restartedProvider, {
      chainId: BigInt(runtime.chain_id),
      sponsor: runtime.sponsor,
      paymaster: runtime.paymaster,
      quoteUrl: runtime.quote_url,
      gatewayUrl: runtime.gateway_url,
      accessToken: () => ownerToken,
      fetch: realFetch,
      submissionStore,
    });
    const resumedSponsor = await restartedGas.resume(batch, signed.relayerSignature);
    assert.equal(resumedSponsor.tx_hash?.toLowerCase(), sponsoredHash.toLowerCase());
    const retry = await http('/v1/wallet/sponsored/submit', exactSubmission);
    assert([200, 202].includes(retry.status));
    assert.equal(String(retry.body.tx_hash).toLowerCase(), sponsoredHash.toLowerCase());
    const statusRequest = {
      account: runtime.account,
      sponsor: signed.quote.sponsor,
      quoteNonce: signed.quote.quoteNonce.toString(),
      relayerSignature: signed.relayerSignature,
    };
    const station = await eventually(
      async () => (await http('/v1/wallet/sponsored/status', statusRequest)).body,
      (answer) => answer.status === 'confirmed' || answer.status === 'reverted'
    );
    assert.equal(station.status, 'confirmed');
    const sponsoredReceipt = await rpc('eth_getTransactionReceipt', [sponsoredHash], true);
    assert.equal(sponsoredReceipt.status, '0x1');
    const changedSubmission = structuredClone(exactSubmission);
    changedSubmission.account_signature = '0x' + '00'.repeat(65);
    const refusal = await http('/v1/wallet/sponsored/submit', changedSubmission);
    assert([400, 403, 409, 422].includes(refusal.status));
    cases.push(
      'sdk-complete-digest-construction',
      'independent-daemon-digest-recompute',
      'expired-consent-refusal',
      'actual-station-first-use-protocol',
      'exact-sponsor-consent-idempotency',
      'station-restart-status'
    );

    await control('lose-quorum');
    await new Promise((resolve) => setTimeout(resolve, 1500));
    const unavailable = await http('/v1/wallet/evm/review', { transaction });
    assert.equal(unavailable.status, 503);
    assert(!unavailable.body.signature, 'unavailable quorum used local fallback');
    cases.push('quorum-loss-no-local-fallback');
    writeFileSync(
      join(runtime.evidence_dir, 'cases-result.json'),
      JSON.stringify({ complete: true, cases }, null, 2),
      { mode: 0o600 }
    );
  } finally {
    await pool.end();
  }
}

if (process.argv[1] && resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  const runtimePath = process.env.WALLET_SIGNING_ADAPTERS_RUNTIME;
  assert(runtimePath, 'protected runtime configuration is required');
  await walletSigningAdaptersProduction(JSON.parse(readFileSync(runtimePath, 'utf8')) as Runtime);
}
