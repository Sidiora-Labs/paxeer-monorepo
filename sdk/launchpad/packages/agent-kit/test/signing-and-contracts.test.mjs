import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import ts from 'typescript';
import { getAbiItem, parseAbi, toFunctionSelector, encodeFunctionData, zeroAddress } from 'viem';
import {
  AGENT_REQUEST_DOMAIN,
  DEPLOYMENT,
  EXECUTION_CALLS,
  HOME_CHAIN_ABI,
  VIRTUAL_CHAIN_ID_BASE,
  TRADE_DEADLINE_SECONDS,
  agentRequestDigest,
  agentRequestMethod,
  signAgentRequest,
  buildClaim,
  routerAbi,
  quoterAbi,
  protocolConfigAbi,
  agentPublicKey,
  agentDid,
  launchpadPreset,
} from '../dist/index.js';
import { seed, reference, root, pool, wallet, token, at } from './support.mjs';

function signedRequestsMatchWalletSdk() {
  const inputs = [
    { method: 'POST', url: '/v1/agent/send?trace=1', body: '{"tx":{"value":"0"}}' },
    {
      method: 'get',
      url: 'https://gateway.example.com/v1/agent/balances?tokens=1#fragment',
      body: '',
    },
    {
      method: 'POST',
      url: '/v1/agent/actions/allowance-and-call',
      body: new Uint8Array([0, 128, 255]),
    },
  ];
  assert.equal(AGENT_REQUEST_DOMAIN, reference.AGENT_REQUEST_DOMAIN);
  assert.deepEqual(agentPublicKey(seed), Buffer.from(reference.agentPublicKey(seed)));
  assert.equal(
    agentPublicKey(seed).toString('hex'),
    'd75a980182b10ab7d54bfed3c964073a0ee172f3daa62325af021a68f707511a',
  );
  for (const input of inputs) {
    const fixed = {
      ...input,
      privateKey: seed,
      keyId: agentDid(seed),
      nonce: Uint8Array.from({ length: 16 }, (_, i) => i),
      expiresAt: 1767355260n,
    };
    const digestInput = {
      method: reference.agentRequestMethod(input.method, input.url),
      keyId: fixed.keyId,
      nonce: fixed.nonce,
      expiry: fixed.expiresAt,
      body: input.body,
    };
    assert.equal(
      agentRequestMethod(input.method, input.url),
      reference.agentRequestMethod(input.method, input.url),
    );
    assert.deepEqual(
      agentRequestDigest(digestInput),
      Buffer.from(reference.agentRequestDigest(digestInput)),
    );
    assert.deepEqual(signAgentRequest(fixed), reference.signAgentRequest(fixed));
  }
  const claim = {
    privateKey: seed,
    did: agentDid(seed),
    ownerUserId: 'launchpad-owner',
    expiresAt: 1767355260n,
  };
  assert.deepEqual(
    buildClaim(seed, claim.did, claim.ownerUserId, claim.expiresAt),
    reference.buildClaim(claim),
  );
  assert.throws(
    () =>
      signAgentRequest({
        privateKey: seed,
        keyId: 'id',
        method: 'POST',
        url: '/',
        nonce: new Uint8Array(15),
      }),
    /invalid_agent_nonce/,
  );
  assert.throws(
    () =>
      agentRequestDigest({
        method: 'GET /',
        keyId: 'id',
        nonce: new Uint8Array(16),
        expiry: -1n,
        body: '',
      }),
    /invalid_expiry/,
  );
}

function functionShape(item) {
  const parameter = ({ type, components }) => ({
    type,
    ...(components ? { components: components.map(parameter) } : {}),
  });
  return {
    name: item.name,
    stateMutability: item.stateMutability,
    inputs: item.inputs.map(parameter),
    outputs: item.outputs.map(parameter),
  };
}

function sourceAbi(source, name) {
  const tree = ts.createSourceFile('source.ts', source, ts.ScriptTarget.ES2022, true);
  let strings;
  function visit(node) {
    if (
      ts.isVariableDeclaration(node) &&
      node.name.getText(tree) === name &&
      node.initializer &&
      ts.isCallExpression(node.initializer)
    ) {
      const input = node.initializer.arguments[0];
      if (input && ts.isArrayLiteralExpression(input))
        strings = input.elements.map((value) => {
          assert.ok(ts.isStringLiteral(value));
          return value.text;
        });
    }
    ts.forEachChild(node, visit);
  }
  visit(tree);
  assert.ok(strings, name);
  return parseAbi(strings);
}

async function contractCallsMatchWebSources() {
  const read = (name) => readFileSync(resolve(root, `apps/launchpad/web/src/${name}`), 'utf8');
  const deployment = JSON.parse(
    read('core/network/deployment.ts').match(/Object.freeze\((\{[\s\S]*?\}) as const\)/)[1],
  );
  for (const role of ['chainId', 'router', 'quoter', 'protocolConfig', 'usdl', 'creationFeeRaw'])
    assert.equal(DEPLOYMENT[role], deployment[role]);
  assert.equal(DEPLOYMENT.spender, deployment.launchpadPrecompile);
  const domain = read('widgets/trading/transaction-controls/domain.ts');
  const reviewed = sourceAbi(domain, 'reviewedRouterAbi');
  for (const name of ['buy', 'sell'])
    assert.deepEqual(
      functionShape(getAbiItem({ abi: routerAbi, name })),
      functionShape(getAbiItem({ abi: reviewed, name })),
    );
  const creation = read('widgets/create/creation-adapters.ts');
  const creationRouter = sourceAbi(creation, 'routerAbi');
  assert.deepEqual(
    functionShape(getAbiItem({ abi: routerAbi, name: 'createMarket' })),
    functionShape(getAbiItem({ abi: creationRouter, name: 'createMarket' })),
  );
  assert.deepEqual(
    functionShape(getAbiItem({ abi: protocolConfigAbi, name: 'creationFee' })),
    functionShape(getAbiItem({ abi: sourceAbi(creation, 'configAbi'), name: 'creationFee' })),
  );
  assert.deepEqual(
    functionShape(HOME_CHAIN_ABI[0]),
    functionShape(
      getAbiItem({
        abi: sourceAbi(read('widgets/trade/home-chain.ts'), 'HOME_CHAIN_ABI'),
        name: 'homeChainOf',
      }),
    ),
  );
  const providers = read('widgets/trading/transaction-controls/providers.ts');
  assert.deepEqual(
    functionShape(getAbiItem({ abi: quoterAbi, name: 'quoteExactInput' })),
    functionShape(getAbiItem({ abi: sourceAbi(providers, 'quoterAbi'), name: 'quoteExactInput' })),
  );
  const chain = read('widgets/create-chain/chain-domain.ts');
  assert.equal(Number(chain.match(/VIRTUAL_CHAIN_ID_BASE = (\d+)/)[1]), VIRTUAL_CHAIN_ID_BASE);
  const compiled = ts.transpileModule(
    domain.replace("from 'viem'", `from '${import.meta.resolve('viem')}'`),
    { compilerOptions: { target: ts.ScriptTarget.ES2022, module: ts.ModuleKind.ESNext } },
  ).outputText;
  const web = await import(
    'data:text/javascript;base64,' + Buffer.from(compiled).toString('base64')
  );
  const context = {
    chainId: 125,
    manifestRevision: 'reviewed',
    pool,
    token,
    router: DEPLOYMENT.router,
    sender: wallet,
    recipient: wallet,
    inputRaw: 1000000n,
    isBuy: true,
    opticalRevision: '',
    opticalData: '0x',
  };
  const now = Date.parse(at);
  const quote = {
    quoteId: 'quote',
    context,
    blockHeight: 100n,
    quotedAt: now,
    expiresAt: now + 30000,
    outputRaw: 100000n,
    feeComponents: [],
    impactBps: 100,
  };
  const authority = {
    chainId: 125,
    manifestRevision: 'reviewed',
    router: DEPLOYMENT.router,
    pool,
    inputAsset: token,
    recipientMode: 'same_sender',
  };
  const tx = web.buildReviewedRouterTransaction(context, quote, authority, 100, now / 1000);
  assert.equal(tx.deadlineUnix, now / 1000 + TRADE_DEADLINE_SECONDS);
  assert.equal(tx.minimumOutputRaw, 99000n);
  assert.equal(
    tx.data,
    encodeFunctionData({
      abi: routerAbi,
      functionName: 'buy',
      args: [pool, 1000000n, 99000n, BigInt(tx.deadlineUnix)],
    }),
  );
  assert.match(creation, /args:\[draft\.name,draft\.symbol,draft\.feeStrategy,'0x0{40}'\]/);
  assert.equal(zeroAddress, '0x0000000000000000000000000000000000000000');
}

function presetUsesOnlyExecutionContractsAndSelectors() {
  const caps = {
    ownerWallet: wallet,
    maxTxValueWei: '100',
    maxDailyValueWei: '1000',
    maxApproveWei: '1000000',
    budgetCapWei: '1000000',
    expiresInSeconds: 3600,
    sellTokens: [token],
  };
  const preset = launchpadPreset(caps);
  assert.deepEqual(preset.policy, {
    mode: 'trade_only',
    max_tx_value_wei: '100',
    max_daily_value_wei: '1000',
    max_approve_wei: '1000000',
    rate_limit_per_min: 60,
    allow_native_transfer: false,
    withdrawal_allowlist_only: true,
  });
  assert.deepEqual(
    preset.rules
      .filter((rule) => rule.subject === 'contract')
      .map((rule) => rule.value)
      .sort(),
    [DEPLOYMENT.router, DEPLOYMENT.usdl, token].map((address) => address.toLowerCase()).sort(),
  );
  assert.deepEqual(
    preset.rules
      .filter((rule) => rule.subject === 'selector')
      .map((rule) => rule.value)
      .sort(),
    Object.values(EXECUTION_CALLS)
      .map((call) => call.selector)
      .sort(),
  );
  assert.deepEqual(
    preset.rules.filter((rule) => rule.subject === 'address'),
    [{ effect: 'allow', subject: 'address', value: DEPLOYMENT.spender.toLowerCase() }],
  );
  assert.deepEqual(
    preset.rules.filter((rule) => rule.subject === 'withdrawal'),
    [{ effect: 'allow', subject: 'withdrawal', value: wallet }],
  );
  assert.deepEqual(preset.budget, {
    target_contract: DEPLOYMENT.router,
    token: DEPLOYMENT.usdl,
    cap_wei: '1000000',
    expires_in_seconds: 3600,
  });
  for (const [name, call] of Object.entries(EXECUTION_CALLS))
    assert.equal(call.selector, toFunctionSelector(call.method), name);
  assert.throws(() => launchpadPreset({ ...caps, maxDailyValueWei: '99' }), /invalid_daily_cap/);
  assert.throws(() => launchpadPreset({ ...caps, expiresInSeconds: 0 }), /invalid_budget_expiry/);
  assert.throws(() => launchpadPreset({ ...caps, budgetCapWei: '0' }), /invalid_raw_amount/);
}

test(signedRequestsMatchWalletSdk);
test(contractCallsMatchWebSources);
test(presetUsesOnlyExecutionContractsAndSelectors);
