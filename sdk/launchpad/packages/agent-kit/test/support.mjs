import assert from 'node:assert/strict';
import { createPublicKey, verify } from 'node:crypto';
import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import ts from 'typescript';
import {
  decodeFunctionData,
  encodeFunctionData,
  encodeFunctionResult,
  erc20Abi,
  getAddress,
  parseAbi,
} from 'viem';
import {
  DEPLOYMENT,
  HOME_CHAIN_ABI,
  LaunchpadAgentKit,
  agentDid,
  agentPublicKey,
  routerAbi,
  quoterAbi,
  protocolConfigAbi,
} from '../dist/index.js';

export const root = fileURLToPath(new URL('../../../../../', import.meta.url));
export const seed = Buffer.from(
  '9d61b19deffd5a60ba844af492ec2cc44449c5697b326919703bac031cae7f60',
  'hex',
);
export const pool = DEPLOYMENT.quoter.toLowerCase();
export const token = DEPLOYMENT.protocolConfig.toLowerCase();
export const wallet = '0xbb82d74e071e03d8e68f1d78fae9efe8b6b7a723';
export const actionId = 'act_0123456789abcdef01234567';
export const txHash = `0x${'1'.repeat(64)}`;
export const at = '2026-01-02T12:00:00.000Z';
export const position = {
  chainId: '125',
  height: '100',
  blockHash: txHash,
  transactionHash: txHash,
  transactionIndex: '0',
  logIndex: '0',
};
export const watermark = {
  chainId: '125',
  height: '100',
  blockHash: txHash,
  integrity: 'healthy',
  observedAt: at,
  projectionRevision: '1',
};

function moduleUrl(source) {
  return (
    'data:text/javascript;base64,' +
    Buffer.from(
      ts.transpileModule(source, {
        compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext },
      }).outputText,
    ).toString('base64')
  );
}

export async function walletReference() {
  const errors = moduleUrl(readFileSync(resolve(root, 'sdk/wallet/src/errors.ts'), 'utf8'));
  let source = readFileSync(resolve(root, 'sdk/wallet/src/agent.ts'), 'utf8');
  source = source.replace("'./errors.js'", JSON.stringify(errors));
  for (const dependency of ['@noble/curves/ed25519.js', '@noble/hashes/sha2.js'])
    source = source.replace(`'${dependency}'`, JSON.stringify(import.meta.resolve(dependency)));
  return import(moduleUrl(source));
}

export const reference = await walletReference();

export function envelope(data) {
  return {
    data,
    generatedAt: at,
    nextCursor: null,
    quality: 'complete',
    requestId: '11111111-1111-4111-8111-111111111111',
    revision: '1',
    watermark,
  };
}

export function poolDetail(homeChain = '0') {
  return envelope({
    pool: {
      chainId: '125',
      homeChain,
      poolAddress: pool,
      tokenAddress: token,
      creator: wallet,
      createdPosition: position,
      createdAt: at,
      virtualUsdlReserveRaw: '1000000',
      realUsdlBalanceRaw: '1000000',
      effectivePricingUsdlReserveRaw: '2000000',
      tokenReserveRaw: '1000000000',
      quoteAssetAddress: DEPLOYMENT.usdl.toLowerCase(),
      quoteAssetDecimals: 6,
      feeBps: 100,
    },
    token: {
      address: token,
      name: 'Science',
      symbol: 'SCI',
      decimals: 18,
      decimalsQuality: 'verified',
      logoAssetId: null,
      metadataRevision: '1',
      sourcePosition: position,
      supplyQuality: 'verified',
      totalSupplyRaw: '1000000000',
    },
    stats: {
      poolAddress: pool,
      buys24h: '1',
      sells24h: '0',
      changeBaselineQuality: 'complete',
      circulatingMarketCapUsdlRaw: '5000000',
      holderCount: '1',
      indicativeTvlUsdlRaw: null,
      priceChangeBps24h: null,
      realUsdlBalanceRaw: '1000000',
      sourceUpperPosition: position,
      supplyValuationQuality: 'verified',
      totalSupplyValuationUsdlRaw: '10000000',
      valuationChangeUsdlRaw: null,
      valuationMethod: null,
      volume24hRaw: '1000000',
      watermark,
      windowStart: '2026-01-01T12:00:00.000Z',
      windowEnd: at,
      spotPriceUsdlRaw: '2000000',
    },
  });
}

export function publicResponses(homeChain = '0') {
  const detail = poolDetail(homeChain);
  return new Map([
    [`/v2/pools/${pool}`, detail],
    ['/v2/pools', envelope({ items: [detail.data.pool] })],
    [
      '/v2/search',
      envelope({
        items: [
          {
            id: pool,
            kind: 'pool',
            label: 'Science',
            poolAddress: pool,
            score: '1',
            tokenAddress: token,
            wallet: null,
          },
        ],
      }),
    ],
    [`/v2/pools/${pool}/trades`, envelope({ items: [] })],
    [
      `/v2/pools/${pool}/candles`,
      envelope({
        items: [],
        seriesModelVersion: '1',
        snapshotId: '11111111-1111-4111-8111-111111111111',
      }),
    ],
    [
      `/v2/pools/${pool}/holders`,
      envelope({
        coverage: 'complete',
        excludedProtocolAddresses: [],
        exclusionPolicyVersion: '1',
        fromPosition: null,
        toPosition: null,
        items: [{ balanceRaw: '12', share: null, tokenAddress: token, wallet }],
      }),
    ],
    [
      `/v3/wallets/${wallet}/portfolio`,
      {
        algorithm: 'fifo',
        data: {
          positions: [],
          summary: {
            basisQuality: 'complete',
            completeValueUsdlRaw: '0',
            knownRealizedSubtotalUsdlRaw: '0',
            positionCount: 0,
            pricedSubtotalUsdlRaw: '0',
            pricingQuality: 'complete',
            realizedPnlRaw: '0',
            snapshotId: '1',
            unpricedAssetCount: 0,
            wallet,
            watermark,
          },
        },
        decimals: 6,
        positionErrors: {},
        quoteAsset: DEPLOYMENT.usdl,
        version: 3,
      },
    ],
  ]);
}

export function verifySignature(request, body, now) {
  const headers = request.headers;
  assert.equal(headers.get('x-agent-key'), agentDid(seed));
  assert.match(headers.get('x-agent-nonce'), /^[0-9a-f]{32}$/);
  assert.equal(headers.get('x-agent-expires'), String(Math.floor(now / 1000) + 60));
  const digest = reference.agentRequestDigest({
    method: reference.agentRequestMethod(request.method, request.url),
    keyId: headers.get('x-agent-key'),
    nonce: Buffer.from(headers.get('x-agent-nonce'), 'hex'),
    expiry: BigInt(headers.get('x-agent-expires')),
    body,
  });
  const publicKey = createPublicKey({
    format: 'der',
    type: 'spki',
    key: Buffer.concat([Buffer.from('302a300506032b6570032100', 'hex'), agentPublicKey(seed)]),
  });
  assert.equal(
    verify(null, digest, publicKey, Buffer.from(headers.get('x-agent-signature'), 'hex')),
    true,
  );
}

export class GatewayHarness {
  requests = [];
  responses = publicResponses();
  allowance = 100000000n;
  home = 0n;
  chainId = 125;
  address = wallet;
  outputRaw = 100000n;
  impactBps = 100n;
  creationFee = 1000000n;
  refusal = undefined;
  afterRequest = undefined;
  now = Date.parse(at);
  auth = false;
  bindingRequired = false;

  constructor() {
    this.kit = new LaunchpadAgentKit({
      gatewayUrl: 'https://gateway.example.com',
      dataUrl: 'https://data.example.com',
      privateKey: seed,
      token: 'agent-read',
      now: () => this.now,
      fetch: this.fetch,
    });
  }

  fetch = async (input, init) => {
    const request = new Request(input, init);
    const url = new URL(request.url);
    const bodyText = await request.text();
    const body = bodyText ? JSON.parse(bodyText) : undefined;
    const entry = {
      method: request.method,
      path: url.pathname,
      query: Object.fromEntries(url.searchParams),
      body,
      headers: Object.fromEntries(request.headers),
      bodyText,
    };
    this.requests.push(entry);
    const respond = (data, status = 200) => {
      this.afterRequest?.(entry);
      return Response.json(data, { status });
    };
    if (url.hostname === 'data.example.com') {
      assert.equal(request.method, 'GET');
      assert.equal(request.headers.has('authorization'), false);
      assert.equal(request.headers.has('x-agent-signature'), false);
      assert.ok(this.responses.has(url.pathname), url.pathname);
      return respond(this.responses.get(url.pathname));
    }
    assert.equal(url.hostname, 'gateway.example.com');
    if (request.headers.has('x-agent-signature')) verifySignature(request, bodyText, this.now);
    else if (!url.pathname.startsWith('/v1/agent/auth/'))
      assert.equal(request.headers.get('authorization'), 'Bearer agent-read');
    if (this.refusal?.path === url.pathname) return respond(this.refusal.body, this.refusal.status);
    if (url.pathname === '/v1/agent/auth/challenge') {
      assert.deepEqual(body, { did: agentDid(seed) });
      return respond({
        did: body.did,
        nonce: 'launchpad-auth-nonce',
        message: `paxeer-agent-auth:${body.did}:launchpad-auth-nonce`,
        expires_in: 60,
      });
    }
    if (url.pathname === '/v1/agent/auth/verify') {
      assert.equal(body.public_key, agentPublicKey(seed).toString('hex'));
      const { ed25519 } = await import('@noble/curves/ed25519.js');
      assert.equal(
        body.signature,
        Buffer.from(
          ed25519.sign(Buffer.from(`paxeer-agent-auth:${body.did}:${body.nonce}`), seed),
        ).toString('hex'),
      );
      this.auth = true;
      return respond({ token: 'agent-read', expires_in: 3600, did: body.did });
    }
    if (url.pathname === '/v1/agent/me')
      return respond({
        did: agentDid(seed),
        wallet: { address: this.address, chain_id: this.chainId },
        chain: { id: this.chainId },
      });
    if (url.pathname === '/v1/agent/allowance') {
      assert.equal(url.searchParams.get('spender'), DEPLOYMENT.spender);
      return respond({ allowance: this.allowance.toString() });
    }
    if (url.pathname === '/v1/agent/balances')
      return respond({
        address: this.address,
        native_wei: '1000000000000000000',
        tokens: [{ token: DEPLOYMENT.usdl, balance: '1000000', decimals: 6, symbol: 'USDL' }],
      });
    if (url.pathname === '/v1/agent/simulate') {
      assert.deepEqual(Object.keys(body).sort(), ['tx']);
      assert.deepEqual(Object.keys(body.tx).sort(), ['chainId', 'data', 'to', 'value']);
      assert.equal(body.tx.chainId, 125);
      assert.equal(body.tx.value, '0');
      const target = body.tx.to.toLowerCase();
      let return_data;
      if (target === DEPLOYMENT.spender.toLowerCase()) {
        const call = decodeFunctionData({ abi: HOME_CHAIN_ABI, data: body.tx.data });
        assert.equal(call.functionName, 'homeChainOf');
        assert.deepEqual(call.args.map(getAddress), [getAddress(pool)]);
        return_data = encodeFunctionResult({
          abi: HOME_CHAIN_ABI,
          functionName: call.functionName,
          result: this.home,
        });
      } else if (target === DEPLOYMENT.quoter.toLowerCase()) {
        const call = decodeFunctionData({ abi: quoterAbi, data: body.tx.data });
        assert.equal(call.functionName, 'quoteExactInput');
        return_data = encodeFunctionResult({
          abi: quoterAbi,
          functionName: call.functionName,
          result: { amountOut: this.outputRaw, feeAmount: 10n, priceImpactBps: this.impactBps },
        });
      } else if (
        target === DEPLOYMENT.protocolConfig.toLowerCase() &&
        body.tx.data === encodeFunctionData({ abi: protocolConfigAbi, functionName: 'creationFee' })
      ) {
        return_data = encodeFunctionResult({
          abi: protocolConfigAbi,
          functionName: 'creationFee',
          result: this.creationFee,
        });
      } else {
        const abi = target === DEPLOYMENT.router.toLowerCase() ? routerAbi : erc20Abi;
        const call = decodeFunctionData({ abi, data: body.tx.data });
        assert.ok(['buy', 'sell', 'createMarket', 'approve'].includes(call.functionName));
        return_data = '0x';
      }
      return respond({ ok: true, gas: '50000', return_data, error: null });
    }
    if (url.pathname === '/v1/agent/send') return respond({ tx_hash: txHash });
    if (url.pathname === '/v1/agent/actions/allowance-and-call') {
      const abi = parseAbi([`function ${body.method}`]);
      const data = encodeFunctionData({
        abi,
        functionName: body.method.split('(')[0],
        args: body.args,
      });
      const call = decodeFunctionData({ abi: routerAbi, data });
      assert.ok(['buy', 'sell', 'createMarket'].includes(call.functionName));
      assert.equal(body.spender, DEPLOYMENT.spender);
      assert.equal(body.contract, DEPLOYMENT.router);
      return respond(
        { ok: null, action_id: actionId, status: 'pending', terminal: false, error: null },
        202,
      );
    }
    if (url.pathname === `/v1/agent/actions/${actionId}`)
      return respond({
        ok: true,
        action_id: actionId,
        status: 'confirmed',
        terminal: true,
        error: null,
      });
    if (url.pathname === '/v1/agent/provision') {
      if (this.bindingRequired && !body.bind_signature)
        return respond(
          {
            wallet: { address: wallet, chain_id: 125 },
            provision: {
              state: 'created',
              awaiting: 'agent_signature',
              address: wallet,
              bindMessage:
                '0x' + Buffer.from(reference.bindingMessage(125, wallet, 7n)).toString('hex'),
            },
          },
          202,
        );
      if (this.bindingRequired)
        assert.equal(
          body.bind_signature,
          Buffer.from(
            reference.signBinding({ privateKey: seed, chainId: 125, evmAddress: wallet, nonce: 7n })
              .signature,
          ).toString('hex'),
        );
      return respond({
        wallet: { address: wallet, chain_id: 125 },
        provision: { state: 'active' },
      });
    }
    assert.fail(`Unexpected request: ${entry.method} ${entry.path}`);
  };

  signed() {
    return this.requests.filter((request) => request.headers['x-agent-signature']);
  }
  simulations() {
    return this.requests.filter((request) => request.path === '/v1/agent/simulate');
  }
}

export function tradeInput(quote, overrides = {}) {
  return {
    pool,
    quoteId: quote.quoteId,
    amountInRaw: '1000000',
    maxSlippageBps: 100,
    idempotencyKey: 'launchpad-trade-1',
    ...overrides,
  };
}

export const launchInput = {
  name: 'Science',
  symbol: 'SCI',
  feeStrategy: 2,
  idempotencyKey: 'launchpad-create-1',
};
