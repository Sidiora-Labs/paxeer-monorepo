import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import ts from 'typescript';
import {
  decodeFunctionData,
  encodeFunctionData,
  erc20Abi,
  getAddress,
  parseAbi,
  toFunctionSelector,
  zeroAddress,
} from 'viem';
import {
  AgentGatewayClient,
  DEPLOYMENT,
  EXECUTION_CALLS,
  LaunchpadAgentKit,
  SIMULATION_TTL_MS,
  QUOTE_TTL_MS,
  invokeTool,
  quoterAbi,
  routerAbi,
} from '../dist/index.js';
import {
  GatewayHarness,
  actionId,
  at,
  launchInput,
  pool,
  poolDetail,
  publicResponses,
  reference,
  root,
  seed,
  token,
  tradeInput,
  txHash,
  wallet,
} from './support.mjs';

async function quoteBuildsExactGatewaySimulation() {
  const h = new GatewayHarness();
  const quote = await h.kit.quote({ pool, side: 'buy', amountInRaw: '1000000' });
  assert.deepEqual(
    { ...quote, quoteId: 'id' },
    {
      pool: getAddress(pool),
      side: 'buy',
      amountInRaw: '1000000',
      quoteId: 'id',
      wallet: getAddress(wallet),
      token: getAddress(token),
      outputRaw: '100000',
      feeRaw: '10',
      priceImpactBps: 100,
      quotedAt: Date.parse(at),
      expiresAt: Date.parse(at) + QUOTE_TTL_MS,
      chainId: 125,
    },
  );
  assert.equal(h.signed().length, 0);
  const request = h.simulations().at(-1);
  const expected = encodeFunctionData({
    abi: quoterAbi,
    functionName: 'quoteExactInput',
    args: [getAddress(pool), 1000000n, true],
  });
  assert.deepEqual(
    { method: request.method, path: request.path, body: request.body },
    {
      method: 'POST',
      path: '/v1/agent/simulate',
      body: { tx: { to: DEPLOYMENT.quoter, data: expected, value: '0', chainId: 125 } },
    },
  );
  assert.equal(
    request.body.tx.data.slice(0, 10),
    toFunctionSelector('quoteExactInput(address,uint256,bool)'),
  );
  assert.deepEqual(decodeFunctionData({ abi: quoterAbi, data: request.body.tx.data }).args, [
    getAddress(pool),
    1000000n,
    true,
  ]);
}

async function buyAndSellSendExactReviewedTransactions() {
  for (const side of ['buy', 'sell']) {
    const h = new GatewayHarness();
    const quote = await h.kit.quote({ pool, side, amountInRaw: '1000000' });
    const result = await invokeTool(h.kit, side, tradeInput(quote));
    assert.equal(result.ok, true);
    assert.deepEqual(result.data, {
      submission: 'transaction',
      simulationPhase: 'call',
      result: { tx_hash: txHash },
    });
    const request = h.signed().at(-1);
    const args = [getAddress(pool), 1000000n, 99000n, BigInt(Date.parse(at) / 1000 + 120)];
    const data = encodeFunctionData({ abi: routerAbi, functionName: side, args });
    assert.deepEqual(
      { method: request.method, path: request.path, body: request.body },
      {
        method: 'POST',
        path: '/v1/agent/send',
        body: { tx: { to: DEPLOYMENT.router, data, value: '0', chainId: 125 } },
      },
    );
    assert.equal(request.body.tx.data.slice(0, 10), EXECUTION_CALLS[side].selector);
    assert.deepEqual(decodeFunctionData({ abi: routerAbi, data }).args, args);
    assert.deepEqual(h.simulations().at(-1).body, request.body);
    assert.deepEqual(h.requests.find((request) => request.path === '/v1/agent/allowance').query, {
      token: side === 'buy' ? DEPLOYMENT.usdl : getAddress(token),
      spender: DEPLOYMENT.spender,
    });
    const again = await invokeTool(h.kit, side, tradeInput(quote));
    assert.equal(again.ok, false);
    assert.equal(again.error.code, 'fresh_quote_required');
    assert.equal(h.signed().length, 1);
  }
}

async function allowanceActionsCarryExactApprovalAndCall() {
  for (const side of ['buy', 'sell']) {
    const h = new GatewayHarness();
    h.allowance = 0n;
    const quote = await h.kit.quote({ pool, side, amountInRaw: '1000000' });
    const result = await invokeTool(h.kit, side, tradeInput(quote));
    assert.equal(result.ok, true);
    assert.equal(result.data.submission, 'action');
    assert.equal(result.data.result.action_id, actionId);
    const request = h.signed().at(-1);
    const args = [getAddress(pool), '1000000', '99000', String(Date.parse(at) / 1000 + 120)];
    assert.deepEqual(
      { method: request.method, path: request.path, body: request.body },
      {
        method: 'POST',
        path: '/v1/agent/actions/allowance-and-call',
        body: {
          token: side === 'buy' ? DEPLOYMENT.usdl : getAddress(token),
          amount: '1000000',
          spender: DEPLOYMENT.spender,
          contract: DEPLOYMENT.router,
          method: EXECUTION_CALLS[side].method,
          args,
          idempotency_key: 'launchpad-trade-1',
        },
      },
    );
    const abi = parseAbi([`function ${request.body.method}`]);
    const encoded = encodeFunctionData({ abi, functionName: side, args: request.body.args });
    assert.equal(encoded.slice(0, 10), EXECUTION_CALLS[side].selector);
    assert.deepEqual(decodeFunctionData({ abi: routerAbi, data: encoded }).args, [
      getAddress(pool),
      1000000n,
      99000n,
      BigInt(args[3]),
    ]);
    const simulation = h.simulations().at(-1);
    assert.equal(simulation.body.tx.to, request.body.token);
    assert.equal(simulation.body.tx.data.slice(0, 10), EXECUTION_CALLS.approve.selector);
    assert.deepEqual(decodeFunctionData({ abi: erc20Abi, data: simulation.body.tx.data }).args, [
      getAddress(DEPLOYMENT.spender),
      1000000n,
    ]);
    assert.equal(simulation.body.tx.value, '0');
    assert.equal(simulation.body.tx.chainId, 125);
    const polled = await invokeTool(h.kit, 'action', { actionId });
    assert.equal(polled.ok, true);
    assert.equal(polled.data.terminal, true);
  }
}

async function launchSendsExactStandardMarketCall() {
  for (const allowance of [0n, 1000000n, 2000000n]) {
    const h = new GatewayHarness();
    h.allowance = allowance;
    const result = await invokeTool(h.kit, 'launch', launchInput);
    assert.equal(result.ok, true);
    const request = h.signed().at(-1);
    const args = [launchInput.name, launchInput.symbol, launchInput.feeStrategy, zeroAddress];
    const calldata = encodeFunctionData({ abi: routerAbi, functionName: 'createMarket', args });
    if (allowance === 1000000n) {
      assert.deepEqual(
        { method: request.method, path: request.path, body: request.body },
        {
          method: 'POST',
          path: '/v1/agent/send',
          body: { tx: { to: DEPLOYMENT.router, data: calldata, value: '0', chainId: 125 } },
        },
      );
      assert.deepEqual(h.simulations().at(-1).body, request.body);
      assert.deepEqual(
        decodeFunctionData({ abi: routerAbi, data: request.body.tx.data }).args,
        args,
      );
    } else {
      assert.deepEqual(
        { method: request.method, path: request.path, body: request.body },
        {
          method: 'POST',
          path: '/v1/agent/actions/allowance-and-call',
          body: {
            token: DEPLOYMENT.usdl,
            amount: '1000000',
            spender: DEPLOYMENT.spender,
            contract: DEPLOYMENT.router,
            method: EXECUTION_CALLS.launch.method,
            args: args.map(String),
            idempotency_key: launchInput.idempotencyKey,
          },
        },
      );
      const abi = parseAbi([`function ${request.body.method}`]);
      assert.equal(
        encodeFunctionData({ abi, functionName: 'createMarket', args: request.body.args }),
        calldata,
      );
      assert.deepEqual(
        decodeFunctionData({ abi: erc20Abi, data: h.simulations().at(-1).body.tx.data }).args,
        [getAddress(DEPLOYMENT.spender), 1000000n],
      );
    }
    assert.equal(calldata.slice(0, 10), EXECUTION_CALLS.launch.selector);
  }
}

async function foreignPoolsAreRefusedBeforeAnySignature() {
  for (const tool of ['quote', 'buy', 'sell']) {
    const h = new GatewayHarness();
    if (tool === 'quote') {
      h.responses = publicResponses('7');
      const result = await invokeTool(h.kit, tool, { pool, side: 'buy', amountInRaw: '1000000' });
      assert.equal(result.error.code, 'foreign_home_chain');
      assert.match(result.error.message, /125000007/);
      assert.match(result.error.message, new RegExp(pool));
      assert.equal(h.requests.length, 1);
    } else {
      const quote = await h.kit.quote({ pool, side: tool, amountInRaw: '1000000' });
      h.requests.length = 0;
      h.responses = publicResponses('7');
      const result = await invokeTool(h.kit, tool, tradeInput(quote));
      assert.equal(result.error.code, 'foreign_home_chain');
      assert.equal(h.requests.length, 1);
    }
    assert.equal(h.signed().length, 0);
  }
  const h = new GatewayHarness();
  h.home = 125000007n;
  const result = await invokeTool(h.kit, 'quote', { pool, side: 'buy', amountInRaw: '1000000' });
  assert.equal(result.error.code, 'foreign_home_chain');
  assert.equal(h.signed().length, 0);
}

async function quotesRefuseStaleMismatchedAndUnsafeTrades() {
  for (const scenario of [
    'missing',
    'expired',
    'side',
    'amount',
    'pool',
    'wallet',
    'slippage',
    'impact',
    'zero-minimum',
    'simulation-expired',
    'expired-during-simulation',
    'wallet-during-simulation',
    'gateway-chain',
  ]) {
    const h = new GatewayHarness();
    if (scenario === 'zero-minimum') h.outputRaw = 1n;
    if (scenario === 'impact') h.impactBps = 1001n;
    const quote = await h.kit.quote({ pool, side: 'buy', amountInRaw: '1000000' });
    const input = tradeInput(quote);
    let name = 'buy',
      expected;
    switch (scenario) {
      case 'missing':
        input.quoteId = '11111111-1111-4111-8111-111111111111';
        expected = 'fresh_quote_required';
        break;
      case 'expired':
        h.now += QUOTE_TTL_MS;
        expected = 'quote_expired';
        break;
      case 'side':
        name = 'sell';
        expected = 'quote_context_mismatch';
        break;
      case 'amount':
        input.amountInRaw = '1000001';
        expected = 'quote_context_mismatch';
        break;
      case 'pool':
        input.pool = DEPLOYMENT.usdl;
        expected = 'quote_context_mismatch';
        break;
      case 'wallet':
        h.address = DEPLOYMENT.usdl;
        expected = 'quote_context_mismatch';
        break;
      case 'slippage':
        input.maxSlippageBps = 501;
        expected = 'slippage_consent_required';
        break;
      case 'impact':
        expected = 'impact_consent_required';
        break;
      case 'zero-minimum':
        expected = 'zero_minimum';
        break;
      case 'gateway-chain':
        h.chainId = 126;
        expected = 'unsupported_gateway_chain';
        break;
      case 'simulation-expired':
        h.afterRequest = (request) => {
          if (request.path === '/v1/agent/simulate' && request.body.tx.to === DEPLOYMENT.router)
            h.now += SIMULATION_TTL_MS;
        };
        expected = 'simulation_expired';
        break;
      case 'expired-during-simulation':
        h.now += 25000;
        h.afterRequest = (request) => {
          if (request.path === '/v1/agent/simulate' && request.body.tx.to === DEPLOYMENT.router)
            h.now += 6000;
        };
        expected = 'quote_expired';
        break;
      case 'wallet-during-simulation':
        h.afterRequest = (request) => {
          if (request.path === '/v1/agent/simulate' && request.body.tx.to === DEPLOYMENT.router)
            h.address = DEPLOYMENT.usdl;
        };
        expected = 'agent_wallet_changed';
        break;
    }
    const result = await invokeTool(h.kit, name, input);
    assert.equal(result.ok, false, scenario);
    assert.equal(result.error.code, expected, scenario);
    assert.equal(h.signed().length, 0, scenario);
  }
  for (const maxSlippageBps of [0, -1, 5001, 1.5]) {
    const h = new GatewayHarness();
    const quote = await h.kit.quote({ pool, side: 'buy', amountInRaw: '1000000' });
    await assert.rejects(h.kit.buy(tradeInput(quote, { maxSlippageBps })), /invalid_slippage/);
    assert.equal(h.signed().length, 0);
  }
}

async function highRiskConsentRetainsTheRequestedBound() {
  const h = new GatewayHarness();
  h.impactBps = 1001n;
  const quote = await h.kit.quote({ pool, side: 'buy', amountInRaw: '1000000' });
  const result = await invokeTool(
    h.kit,
    'buy',
    tradeInput(quote, { maxSlippageBps: 5000, acceptHighSlippage: true, acceptHighImpact: true }),
  );
  assert.equal(result.ok, true);
  const call = decodeFunctionData({ abi: routerAbi, data: h.signed()[0].body.tx.data });
  assert.equal(call.args[2], 50000n);
}

function unionValues(file, name) {
  const tree = ts.createSourceFile(
    file,
    readFileSync(resolve(root, file), 'utf8'),
    ts.ScriptTarget.ES2022,
    true,
  );
  const node = tree.statements.find(
    (node) => ts.isTypeAliasDeclaration(node) && node.name.text === name,
  );
  assert.ok(node, name);
  return node.type.types.map((type) => {
    assert.ok(ts.isLiteralTypeNode(type));
    return type.literal.text;
  });
}

async function gatewayRefusalsRetainEveryDeclaredCode() {
  const codes = [
    ...new Set([
      ...unionValues('services/wallet/gateway/src/policy/agent.ts', 'AgentDenyCode'),
      ...unionValues('services/wallet/gateway/src/agent/verify.ts', 'AgentRequestRefusalCode'),
      ...unionValues('services/wallet/gateway/src/agent/actions/errors.ts', 'ActionErrorCode'),
    ]),
  ];
  assert.ok(codes.length >= 40);
  for (const name of ['quote', 'buy', 'sell', 'launch']) {
    for (const [index, code] of codes.entries()) {
      const h = new GatewayHarness();
      let input;
      if (name === 'quote') input = { pool, side: 'buy', amountInRaw: '1000000' };
      else if (name === 'launch') input = launchInput;
      else input = tradeInput(await h.kit.quote({ pool, side: name, amountInRaw: '1000000' }));
      if (name === 'sell' || name === 'launch') h.allowance = 0n;
      const body =
        index % 2
          ? {
              ok: false,
              error: { code, message: code, retry: { strategy: 'NEVER' }, action_id: actionId },
            }
          : { error: code, message: code };
      const path =
        name === 'quote'
          ? '/v1/agent/simulate'
          : name === 'buy'
            ? '/v1/agent/send'
            : '/v1/agent/actions/allowance-and-call';
      h.refusal = { path, body, status: 403 };
      const result = await invokeTool(h.kit, name, input);
      assert.equal(result.ok, false, `${name} ${code}`);
      assert.equal(result.error.code, code, `${name} ${code}`);
      assert.equal(result.error.status, 403);
      assert.deepEqual(result.error.details, body);
      assert.equal(h.signed().length, name === 'quote' ? 0 : 1);
    }
  }
}

async function simulationsAndActionErrorsPreventAdditionalWrites() {
  const h = new GatewayHarness();
  const quote = await h.kit.quote({ pool, side: 'buy', amountInRaw: '1000000' });
  h.refusal = {
    path: '/v1/agent/simulate',
    status: 200,
    body: { ok: false, gas: null, return_data: null, error: 'SIMULATION_REVERTED' },
  };
  const result = await invokeTool(h.kit, 'buy', tradeInput(quote));
  assert.equal(result.error.code, 'SIMULATION_REVERTED');
  assert.equal(h.signed().length, 0);
  h.refusal = {
    path: `/v1/agent/actions/${actionId}`,
    status: 200,
    body: {
      ok: false,
      action_id: actionId,
      terminal: true,
      error: {
        code: 'POLICY_DENIED',
        message: 'Owner policy refused the call.',
        retry: { strategy: 'NEVER' },
      },
    },
  };
  const action = await invokeTool(h.kit, 'action', { actionId });
  assert.equal(action.error.code, 'POLICY_DENIED');
  assert.equal(action.error.details.action_id, actionId);
  assert.equal(h.signed().length, 0);
}

async function readsUseServedSdkRoutesAndAgentWallet() {
  const h = new GatewayHarness();
  assert.equal((await invokeTool(h.kit, 'list_coins', { sort: 'new', limit: 10 })).ok, true);
  assert.equal((await invokeTool(h.kit, 'search_coins', { q: 'Science', limit: 10 })).ok, true);
  const coin = await invokeTool(h.kit, 'coin', { pool });
  assert.equal(coin.ok, true);
  assert.equal(coin.data.price, '2000000');
  assert.equal(coin.data.marketValueUsdlRaw, '5000000');
  assert.equal(coin.data.progress, null);
  assert.equal(coin.data.progressStatus, 'unavailable');
  assert.equal((await invokeTool(h.kit, 'trades', { pool, side: 'buy' })).ok, true);
  assert.equal(
    (
      await invokeTool(h.kit, 'candles', {
        pool,
        interval: '1h',
        from: '2026-01-01T12:00:00.000Z',
        to: at,
      })
    ).ok,
    true,
  );
  assert.equal((await invokeTool(h.kit, 'holders', { pool })).ok, true);
  const balances = await invokeTool(h.kit, 'balances', { tokens: [DEPLOYMENT.usdl] });
  assert.equal(balances.data.address, wallet);
  const positions = await invokeTool(h.kit, 'positions', {});
  assert.equal(positions.ok, true);
  assert.equal(positions.data.data.summary.wallet, wallet);
  assert.deepEqual(
    h.requests
      .filter((request) => request.path.startsWith('/v2') || request.path.startsWith('/v3'))
      .map((request) => request.path),
    [
      '/v2/pools',
      '/v2/search',
      `/v2/pools/${pool}`,
      `/v2/pools/${pool}/trades`,
      `/v2/pools/${pool}/candles`,
      `/v2/pools/${pool}/holders`,
      `/v3/wallets/${wallet}/portfolio`,
    ],
  );
  assert.deepEqual(h.requests.find((request) => request.path === '/v2/search').query, {
    q: 'Science',
    type: 'pool',
    limit: '10',
  });
  assert.equal(h.signed().length, 0);
  h.responses.set(`/v2/pools/${pool}`, { invalid: true });
  await assert.rejects(h.kit.coin(pool), /Invalid v2 contract/);
}

async function authenticationProvisionAndClaimUseAgentIdentity() {
  const h = new GatewayHarness();
  const client = new AgentGatewayClient({
    gatewayUrl: 'https://gateway.example.com',
    privateKey: seed,
    now: () => h.now,
    fetch: h.fetch,
  });
  await Promise.all([client.identity(), client.identity()]);
  assert.equal(h.auth, true);
  assert.equal(
    h.requests.filter((request) => request.path === '/v1/agent/auth/challenge').length,
    1,
  );
  await client.provision();
  assert.deepEqual(h.signed().at(-1).body, {});
  const claim = client.claim('launchpad-owner');
  assert.deepEqual(claim, {
    did: client.did,
    ...reference.buildClaim({
      privateKey: seed,
      did: client.did,
      ownerUserId: 'launchpad-owner',
      expiresAt: BigInt(Date.parse(at) / 1000 + 60),
    }),
  });
  h.now += 3600000;
  await client.identity();
  assert.equal(
    h.requests.filter((request) => request.path === '/v1/agent/auth/challenge').length,
    2,
  );
  const binding = new GatewayHarness();
  binding.bindingRequired = true;
  const provisioned = await invokeTool(binding.kit, 'provision_agent', {});
  assert.equal(provisioned.ok, true);
  assert.equal(provisioned.data.provision.state, 'active');
  assert.equal(binding.signed().length, 2);
}

async function operationIdsAndQuoteUseAreReservedBeforeDispatch() {
  const h = new GatewayHarness();
  const quote = await h.kit.quote({ pool, side: 'buy', amountInRaw: '1000000' });
  const results = await Promise.all([
    invokeTool(h.kit, 'buy', tradeInput(quote)),
    invokeTool(h.kit, 'buy', tradeInput(quote)),
  ]);
  assert.equal(results.filter((result) => result.ok).length, 1);
  assert.equal(h.signed().length, 1);
  const next = await h.kit.quote({ pool, side: 'buy', amountInRaw: '1000000' });
  const reused = await invokeTool(h.kit, 'buy', tradeInput(next));
  assert.equal(reused.error.code, 'submission_already_started');
  assert.equal(h.signed().length, 1);
}

async function invalidLaunchAndToolInputsNeverSign() {
  const h = new GatewayHarness();
  for (const input of [
    { ...launchInput, name: 'bad\u202ename' },
    { ...launchInput, symbol: '' },
    { ...launchInput, feeStrategy: 4 },
    { ...launchInput, optical: DEPLOYMENT.router },
  ]) {
    const result = await invokeTool(h.kit, 'launch', input);
    assert.equal(result.ok, false);
  }
  const extra = await invokeTool(h.kit, 'buy', { recipient: DEPLOYMENT.usdl });
  assert.equal(extra.error.code, 'invalid_parameters');
  assert.equal(h.signed().length, 0);
  h.creationFee = 2000000n;
  const changed = await invokeTool(h.kit, 'launch', launchInput);
  assert.equal(changed.error.code, 'creation_fee_changed');
  assert.equal(h.signed().length, 0);
}

test(quoteBuildsExactGatewaySimulation);
test(buyAndSellSendExactReviewedTransactions);
test(allowanceActionsCarryExactApprovalAndCall);
test(launchSendsExactStandardMarketCall);
test(foreignPoolsAreRefusedBeforeAnySignature);
test(quotesRefuseStaleMismatchedAndUnsafeTrades);
test(highRiskConsentRetainsTheRequestedBound);
test(gatewayRefusalsRetainEveryDeclaredCode);
test(simulationsAndActionErrorsPreventAdditionalWrites);
test(readsUseServedSdkRoutesAndAgentWallet);
test(authenticationProvisionAndClaimUseAgentIdentity);
test(operationIdsAndQuoteUseAreReservedBeforeDispatch);
test(invalidLaunchAndToolInputsNeverSign);
