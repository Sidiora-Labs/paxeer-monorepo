import assert from 'node:assert/strict';
import { test } from 'node:test';
import { createHash, randomBytes, randomUUID } from 'node:crypto';
import { register } from 'tsx/esm/api';
import { resolve } from 'node:path';
import { pathToFileURL } from 'node:url';
import { LaunchpadAgentKit, invokeTool, agentPublicKey, routerAbi } from '../dist/index.js';
import { encodeFunctionData } from 'viem';
import { startLaunchpadChain, repository } from './local-chain.mjs';
import { startChainData } from './chain-data.mjs';

function walletUserId(did) {
  const bytes = createHash('sha256').update(`paxeer-agent-wallet:${did}`).digest().subarray(0, 16);
  bytes[6] = (bytes[6] & 15) | 128;
  bytes[8] = (bytes[8] & 63) | 128;
  const hex = bytes.toString('hex');
  return `${hex.slice(0, 8)}-${hex.slice(8, 12)}-${hex.slice(12, 16)}-${hex.slice(16, 20)}-${hex.slice(20)}`;
}

async function agentWalksTheLaunchpadOnALocalChain(t) {
  const chain = await startLaunchpadChain();
  const unregister = register();
  let gateway;
  let data;
  t.after(async () => {
    try {
      await data?.stop();
    } finally {
      try {
        await gateway?.stop();
      } finally {
        await unregister();
        await chain.stop();
      }
    }
  });
  const { startJourneyGateway } = await import(
    pathToFileURL(resolve(repository, 'services/wallet/gateway/test/support/local-journey.ts'))
  );
  gateway = await startJourneyGateway(chain.rpcUrl, chain.usdl.target);
  data = await startChainData(chain);
  const seed = randomBytes(32);
  const config = {
    gatewayUrl: gateway.url,
    dataUrl: data.url,
    privateKey: seed,
    evmDeployment: chain.deployment,
    production: false,
    requestTimeoutMs: 120000,
  };
  const kit = new LaunchpadAgentKit(config);
  const did = kit.gateway.did;
  const ownerPath = `/v1/agents/${encodeURIComponent(did)}`;
  const owner = async (method, suffix, body) => {
    const response = await gateway.request(method, ownerPath + suffix, body, gateway.ownerToken);
    assert.ok(
      response.status >= 200 && response.status < 300,
      `${method} ${suffix}: ${JSON.stringify(response)}`,
    );
    return response.body;
  };
  const tool = async (name, input = {}) => {
    const answer = await invokeTool(kit, name, input);
    assert.equal(answer.ok, true, `${name}: ${JSON.stringify(answer)}`);
    return answer.data;
  };
  const { provider, usdl, ethers } = chain;
  let ownerAddress;
  let agentAddress;
  let pool;
  let token;
  let expectedQuote = 0n;
  let expectedCoin = 0n;
  let previousNative = 0n;
  let previousNonce = 0;
  let lastReceipt = chain.receipts.at(-1);
  const transactions = new Map();
  const activityHashes = [];
  async function receipt(hash, expectedFrom, expectedTo, expectedValue = 0n) {
    assert.match(hash, /^0x[0-9a-fA-F]{64}$/);
    const deadline = Date.now() + 120000;
    let mined;
    for (;;) {
      mined = await provider.getTransactionReceipt(hash);
      if (mined) break;
      if (Date.now() >= deadline) {
        const [blockNumber, transaction] = await Promise.all([
          provider.getBlockNumber(),
          provider.getTransaction(hash),
        ]);
        throw new Error(
          `Receipt timed out for ${hash}: block ${blockNumber}, transaction known: ${transaction !== null}`,
        );
      }
      await new Promise((done) => setTimeout(done, 50));
    }
    assert.equal(mined.status, 1, hash);
    const transaction = await provider.getTransaction(hash);
    assert.equal(transaction.from.toLowerCase(), expectedFrom.toLowerCase());
    assert.equal(transaction.to.toLowerCase(), expectedTo.toLowerCase());
    assert.equal(transaction.value, expectedValue);
    assert.equal(transaction.chainId, 125n);
    assert.equal((await provider.getBlock(mined.blockNumber)).hash, mined.blockHash);
    assert.equal(transactions.has(hash), false, 'Each transaction is observed once.');
    transactions.set(hash, { receipt: mined, transaction });
    lastReceipt = mined;
    return { receipt: mined, transaction };
  }
  async function step(name, toolName, route, hashes = [], nativeDelta = 0n) {
    const native = await provider.getBalance(agentAddress);
    const quote = await usdl.balanceOf(agentAddress);
    const coin = token ? await token.balanceOf(agentAddress) : null;
    const nonce = await provider.getTransactionCount(agentAddress);
    let cost = 0n;
    let sends = 0;
    for (const hash of hashes) {
      const item = transactions.get(hash);
      assert.ok(item);
      if (item.transaction.from.toLowerCase() === agentAddress.toLowerCase()) {
        cost += item.receipt.gasUsed * item.receipt.gasPrice;
        sends++;
      }
    }
    assert.equal(native, previousNative + nativeDelta - cost, `${name}: native balance`);
    assert.equal(quote, expectedQuote, `${name}: USDL balance`);
    if (token) assert.equal(coin, expectedCoin, `${name}: coin balance`);
    assert.equal(nonce, previousNonce + sends, `${name}: transaction count`);
    assert.equal((await provider.getTransactionReceipt(lastReceipt.hash)).status, 1);
    const balances = await tool('balances', {
      tokens: [usdl.target, ...(token ? [token.target] : [])],
    });
    assert.equal(balances.address.toLowerCase(), agentAddress.toLowerCase());
    assert.equal(balances.native_wei, String(native));
    assert.equal(balances.tokens[0].balance, String(quote));
    if (token) assert.equal(balances.tokens[1].balance, String(coin));
    previousNative = native;
    previousNonce = nonce;
    t.diagnostic(
      JSON.stringify({
        step: name,
        tool: toolName,
        route,
        transactions: hashes,
        chain: {
          nativeWei: String(native),
          quoteRaw: String(quote),
          coinRaw: coin === null ? null : String(coin),
          nonce,
          ownerQuoteRaw: String(await usdl.balanceOf(ownerAddress)),
          ownerCoinRaw: token ? String(await token.balanceOf(ownerAddress)) : null,
        },
      }),
    );
  }
  async function settle(submission, name, amount, side) {
    assert.equal(submission.submission, 'action');
    assert.equal(submission.simulationPhase, 'approval');
    const actionId = submission.result.action_id;
    assert.match(actionId, /^act_[0-9a-f]{24}$/);
    assert.equal(submission.result.terminal, false);
    let action;
    const deadline = Date.now() + 180000;
    do {
      action = await tool('action', { actionId });
      assert.equal(action.action_id, actionId);
      if (!action.terminal) await new Promise((done) => setTimeout(done, 100));
      assert.ok(Date.now() < deadline, `${name}: ${JSON.stringify(action)}`);
    } while (!action.terminal);
    assert.equal(action.status, 'confirmed');
    assert.equal(action.ok, true);
    assert.equal(action.error, null);
    const asset = side === 'sell' ? token : usdl;
    const approval = await receipt(action.approval.tx_hash, agentAddress, asset.target);
    assert.equal(
      approval.transaction.data,
      asset.interface.encodeFunctionData('approve', [chain.router.target, amount]),
    );
    assert.equal(await asset.allowance(agentAddress, chain.router.target), 0n);
    const call = await receipt(action.call.tx_hash, agentAddress, chain.router.target);
    const decoded = chain.router.interface.parseTransaction({ data: call.transaction.data });
    assert.equal(decoded.name, name === 'launch' ? 'createMarket' : side);
    if (side) {
      assert.equal(decoded.args[0].toLowerCase(), pool.toLowerCase());
      assert.equal(decoded.args[1], amount);
      assert.ok(decoded.args[2] > 0n);
      assert.ok(
        decoded.args[3] >= BigInt((await provider.getBlock(call.receipt.blockNumber)).timestamp),
      );
    }
    activityHashes.push(action.approval.tx_hash, action.call.tx_hash);
    return [action.approval.tx_hash, action.call.tx_hash];
  }
  async function quote(side, amount) {
    const answer = await tool('quote', { pool, side, amountInRaw: String(amount) });
    const actual = await chain.quoter.quoteExactInput(pool, amount, side === 'buy');
    assert.equal(answer.outputRaw, String(actual.amountOut));
    assert.equal(answer.feeRaw, String(actual.feeAmount));
    assert.equal(answer.priceImpactBps, Number(actual.priceImpactBps));
    assert.equal(answer.wallet.toLowerCase(), agentAddress.toLowerCase());
    assert.equal(answer.token.toLowerCase(), token.target.toLowerCase());
    assert.equal(answer.chainId, 125);
    assert.equal(answer.amountInRaw, String(amount));
    return answer;
  }
  const tradeInput = (q) => ({
    pool,
    quoteId: q.quoteId,
    amountInRaw: q.amountInRaw,
    maxSlippageBps: 100,
    acceptHighImpact: true,
    idempotencyKey: randomUUID(),
  });

  async function walletOnlyProvisioningKeepsLayerXBindingPending() {
    const ownerKey = `wallet:${gateway.ownerId}:standard:secp256k1:0`;
    await gateway.admitKey(ownerKey, gateway.ownerId);
    const ownerWallet = await gateway.request(
      'POST',
      '/v1/wallet/provision',
      { stage: 'wallet' },
      gateway.ownerToken,
    );
    assert.equal(ownerWallet.status, 200, JSON.stringify(ownerWallet));
    assert.equal(ownerWallet.body.wallet.binding_state, 'pending');
    assert.equal(ownerWallet.body.provisioning.state, 'evm_key');
    assert.equal(ownerWallet.body.identityBinding, null);
    ownerAddress = ownerWallet.body.wallet.address;
    await gateway.admitSigning(ownerKey);
    const identity = await tool('agent_identity');
    assert.equal(identity.did, did);
    assert.equal(identity.wallet, null);
    const claim = await tool('claim_agent', { ownerUserId: gateway.ownerId });
    const claimed = await owner('POST', '/claim', {
      expires: claim.expires,
      signature: claim.signature,
    });
    assert.equal(claimed.owner_user_id, gateway.ownerId);
    assert.equal((await owner('POST', '/unfreeze')).is_frozen, false);
    const agentKey = `wallet:${walletUserId(did)}:agent:secp256k1:0`;
    await gateway.admitKey(agentKey, `agent:${agentPublicKey(seed).toString('hex')}`);
    const agentWallet = await tool('provision_agent', { stage: 'wallet' });
    assert.equal(agentWallet.provision.state, 'evm_key');
    assert.equal(agentWallet.provision.did, null);
    assert.equal(agentWallet.provision.bindMessage, null);
    agentAddress = agentWallet.wallet.address;
    assert.notEqual(agentAddress.toLowerCase(), ownerAddress.toLowerCase());
    await gateway.admitSigning(agentKey);
    const repeated = await tool('provision_agent', { stage: 'wallet' });
    assert.equal(repeated.wallet.id, agentWallet.wallet.id);
    assert.equal((await tool('agent_identity')).owner_user_id, gateway.ownerId);
    assert.equal(await provider.getCode('0x0000000000000000000000000000000000001004'), '0x');
    await step(
      'create and claim',
      'agent_identity, claim_agent, provision_agent',
      'POST /v1/wallet/provision; POST /v1/agents/:did/claim; POST /v1/agent/provision',
    );
  }
  await t.test(walletOnlyProvisioningKeepsLayerXBindingPending);

  const caps = {
    ownerWallet: ownerAddress,
    maxTxValueWei: '10000000',
    maxDailyValueWei: '50000000',
    maxApproveWei: '1000000000000000',
    budgetCapWei: '50000000',
    expiresInSeconds: 3600,
    rateLimitPerMinute: 600,
  };
  const preset = await tool('preset', caps);
  assert.equal(preset.policy.mode, 'trade_only');
  assert.equal(preset.policy.allow_native_transfer, false);
  assert.equal(preset.policy.withdrawal_allowlist_only, true);
  assert.equal(preset.policy.max_tx_value_wei, caps.maxTxValueWei);
  assert.equal(preset.policy.max_daily_value_wei, caps.maxDailyValueWei);
  const savedPolicy = (await owner('PUT', '/policy', preset.policy)).policy;
  assert.equal(savedPolicy.max_tx_value_wei, caps.maxTxValueWei);
  assert.equal(savedPolicy.max_daily_value_wei, caps.maxDailyValueWei);
  for (const rule of preset.rules) await owner('POST', '/rules', rule);
  const budget = await owner('POST', '/budgets', preset.budget);
  assert.equal(budget.budget.cap_wei, caps.budgetCapWei);
  assert.ok(new Date(budget.budget.expires_at).getTime() > Date.now());
  assert.deepEqual(
    preset.rules.filter((rule) => rule.subject === 'withdrawal').map((rule) => rule.value),
    [ownerAddress.toLowerCase()],
  );
  assert.deepEqual(
    preset.rules.filter((rule) => rule.subject === 'address').map((rule) => rule.value),
    [chain.router.target.toLowerCase()],
  );
  await step('preset', 'preset', 'PUT /v1/agents/:did/policy; POST /rules; POST /budgets');

  const ownerNative = await chain.confirmed(
    chain.signer.sendTransaction({ to: ownerAddress, value: 10n ** 18n }),
  );
  const ownerQuote = await chain.confirmed(usdl.transfer(ownerAddress, 100000000n));
  assert.equal(ownerNative.status, 1);
  assert.equal(ownerQuote.status, 1);
  assert.equal(await provider.getBalance(ownerAddress), 10n ** 18n);
  assert.equal(await usdl.balanceOf(ownerAddress), 100000000n);
  await step('owner funded', 'balances', 'eth_sendTransaction (fixture treasury)', []);
  async function ownerFundingUsesMatchingSignatureAttribution() {
    const nativeFund = await owner('POST', '/fund', { amount: String(5n * 10n ** 17n) });
    await receipt(nativeFund.tx_hash, ownerAddress, agentAddress, 5n * 10n ** 17n);
    assert.equal(nativeFund.to.toLowerCase(), agentAddress.toLowerCase());
    activityHashes.push(nativeFund.tx_hash);
    await step(
      'native funding',
      'balances',
      'POST /v1/agents/:did/fund',
      [nativeFund.tx_hash],
      5n * 10n ** 17n,
    );
    const quoteFund = await owner('POST', '/fund', { token: usdl.target, amount: '50000000' });
    const funded = await receipt(quoteFund.tx_hash, ownerAddress, usdl.target);
    assert.equal(
      funded.transaction.data,
      usdl.interface.encodeFunctionData('transfer', [agentAddress, 50000000n]),
    );
    expectedQuote = 50000000n;
    activityHashes.push(quoteFund.tx_hash);
    await step('quote funding', 'balances', 'POST /v1/agents/:did/fund', [quoteFund.tx_hash]);
  }
  await t.test(ownerFundingUsesMatchingSignatureAttribution);
  assert.equal(expectedQuote, 50000000n);

  const empty = await tool('list_coins', { sort: 'new', limit: 10 });
  assert.deepEqual(empty.data.items, []);
  assert.equal(await chain.registry.getPoolCount(), 0n);
  await step('list coins', 'list_coins', 'GET /v2/pools');
  const launched = await tool('launch', {
    name: 'Journey',
    symbol: 'JRNY',
    feeStrategy: 0,
    idempotencyKey: randomUUID(),
  });
  const launchHashes = await settle(launched, 'launch', 1000000n);
  const launchReceipt = transactions.get(launchHashes[1]).receipt;
  const market = launchReceipt.logs
    .filter((log) => log.address.toLowerCase() === chain.factory.target.toLowerCase())
    .map((log) => {
      try {
        return chain.factory.interface.parseLog(log);
      } catch {
        return null;
      }
    })
    .filter((log) => log?.name === 'MarketCreated');
  assert.equal(market.length, 1);
  assert.equal(market[0].args.creator.toLowerCase(), agentAddress.toLowerCase());
  pool = market[0].args.pool;
  token = new ethers.Contract(market[0].args.token, chain.artifact('SidioraERC20').abi, provider);
  assert.equal(await chain.registry.isRegisteredPool(pool), true);
  assert.equal(await token.name(), 'Journey');
  assert.equal(await token.symbol(), 'JRNY');
  await gateway.admitAsset(token.target, caps.maxApproveWei, caps.maxApproveWei);
  expectedQuote -= 1000000n;
  await step(
    'launch',
    'launch, action',
    'POST /v1/agent/actions/allowance-and-call; GET /v1/agent/actions/:id',
    launchHashes,
  );
  const selected = await tool('list_coins', { sort: 'new', limit: 10 });
  assert.equal(selected.data.items.length, 1);
  assert.equal(selected.data.items[0].poolAddress, pool.toLowerCase());
  const coin = await tool('coin', { pool });
  assert.equal(coin.data.token.address, token.target.toLowerCase());
  const sellPreset = await tool('preset', { ...caps, sellTokens: [token.target] });
  const sellRule = sellPreset.rules.find(
    (rule) => rule.subject === 'contract' && rule.value === token.target.toLowerCase(),
  );
  assert.ok(sellRule);
  await owner('POST', '/rules', sellRule);
  await step(
    'select launched coin',
    'list_coins, coin, preset',
    'GET /v2/pools; GET /v2/pools/:pool; POST /v1/agents/:did/rules',
  );

  const buyQuote = await quote('buy', 5000000n);
  await step(
    'buy quote',
    'quote',
    'GET /v2/pools/:pool; GET /v1/agent/me; POST /v1/agent/simulate',
  );
  async function allowanceActionReauthorizesEachLegAndSurvivesRestart() {
    const submission = await tool('buy', tradeInput(buyQuote));
    assert.equal(submission.submission, 'action');
    const actionId = submission.result.action_id;
    let parked;
    const deadline = Date.now() + 60000;
    do {
      const answer = await invokeTool(kit, 'action', { actionId });
      assert.equal(answer.ok, true, JSON.stringify(answer));
      parked = answer.data;
      if (parked.approval) break;
      assert.ok(Date.now() < deadline, JSON.stringify(parked));
      await new Promise((done) => setTimeout(done, 100));
    } while (true);
    await gateway.restart();
    assert.equal(gateway.url, config.gatewayUrl);
    const resumed = await settle(submission, 'buy', 5000000n, 'buy');
    expectedQuote -= 5000000n;
    expectedCoin += BigInt(buyQuote.outputRaw);
    await step(
      'buy and gateway restart',
      'buy, action',
      'POST /v1/agent/actions/allowance-and-call; GET /v1/agent/actions/:id',
      resumed,
    );
    assert.equal(await provider.getTransactionCount(agentAddress), previousNonce);
  }
  await t.test(allowanceActionReauthorizesEachLegAndSurvivesRestart);
  const positions = await tool('positions');
  assert.equal(positions.data.summary.wallet, agentAddress.toLowerCase());
  assert.equal(positions.data.positions.length, 1);
  assert.equal(positions.data.positions[0].balanceRaw, String(expectedCoin));
  assert.equal(positions.data.positions[0].tokenAddress, token.target.toLowerCase());
  assert.equal(positions.data.positions[0].remainingCostUsdlRaw, null);
  await step('read position', 'positions', 'GET /v1/agent/me; GET /v3/wallets/:wallet/portfolio');

  for (const portion of ['part', 'rest']) {
    const amount = portion === 'part' ? expectedCoin / 2n : expectedCoin;
    assert.ok(amount > 0n);
    const q = await quote('sell', amount);
    const hashes = await settle(await tool('sell', tradeInput(q)), 'sell', amount, 'sell');
    expectedCoin -= amount;
    expectedQuote += BigInt(q.outputRaw);
    await step(
      `sell ${portion}`,
      'quote, sell, action',
      'POST /v1/agent/simulate; POST /v1/agent/actions/allowance-and-call; GET /v1/agent/actions/:id',
      hashes,
    );
  }
  assert.equal((await tool('positions')).data.positions.length, 0);

  async function launchpadBuyCapRefusesBeforeApprovalOrBroadcast() {
    const q = await quote('buy', 10000001n);
    const nonce = await provider.getTransactionCount(agentAddress);
    const height = await provider.getBlockNumber();
    const refused = await invokeTool(kit, 'buy', tradeInput(q));
    assert.equal(refused.ok, false);
    assert.equal(refused.error.status, 403);
    assert.equal(refused.error.code, 'TOKEN_VALUE_CAP');
    assert.equal(await provider.getTransactionCount(agentAddress), nonce);
    assert.equal(await provider.getBlockNumber(), height);
    assert.equal(await usdl.allowance(agentAddress, chain.router.target), 0n);
    await assert.rejects(
      kit.gateway.send({
        to: chain.router.target,
        value: '0',
        chainId: 125,
        data: encodeFunctionData({
          abi: routerAbi,
          functionName: 'buy',
          args: [pool, 10000001n, 1n, BigInt(Math.floor(Date.now() / 1000) + 120)],
        }),
      }),
      (error) => error.status === 403 && error.code === 'TOKEN_VALUE_CAP',
    );
    assert.equal(await provider.getTransactionCount(agentAddress), nonce);
    assert.equal(await provider.getBlockNumber(), height);
    await step(
      'over transaction cap',
      'buy',
      'POST /v1/agent/actions/allowance-and-call; POST /v1/agent/send',
    );
    t.diagnostic(
      JSON.stringify({
        refusal: 'over transaction cap',
        status: refused.error.status,
        code: refused.error.code,
      }),
    );
  }
  await t.test(launchpadBuyCapRefusesBeforeApprovalOrBroadcast);
  const outsider = ethers.Wallet.createRandom().address;
  const outsideNonce = await provider.getTransactionCount(agentAddress);
  const outsideHeight = await provider.getBlockNumber();
  await assert.rejects(
    kit.gateway.send({
      to: outsider,
      value: '0',
      chainId: 125,
      data: encodeFunctionData({
        abi: routerAbi,
        functionName: 'buy',
        args: [pool, 1000000n, 1n, BigInt(Math.floor(Date.now() / 1000) + 120)],
      }),
    }),
    (error) => error.status === 403 && error.code === 'CONTRACT_NOT_ALLOWLISTED',
  );
  assert.equal(await provider.getTransactionCount(agentAddress), outsideNonce);
  assert.equal(await provider.getBlockNumber(), outsideHeight);
  assert.equal(await usdl.allowance(agentAddress, chain.router.target), 0n);
  await step('outside preset', 'gateway.send', 'POST /v1/agent/send');
  t.diagnostic(
    JSON.stringify({ refusal: 'outside preset', status: 403, code: 'CONTRACT_NOT_ALLOWLISTED' }),
  );

  const frozen = await owner('POST', '/freeze');
  assert.equal(frozen.is_frozen, true);
  await step('freeze', 'agent_identity', 'POST /v1/agents/:did/freeze');
  assert.equal((await tool('agent_identity')).is_frozen, true);
  const frozenQuote = await quote('buy', 1000000n);
  const frozenHeight = await provider.getBlockNumber();
  const refusedFrozen = await invokeTool(kit, 'buy', tradeInput(frozenQuote));
  assert.equal(refusedFrozen.ok, false);
  assert.equal(refusedFrozen.error.status, 403);
  assert.equal(refusedFrozen.error.code, 'agent_frozen');
  assert.equal(await provider.getBlockNumber(), frozenHeight);
  assert.equal(await usdl.allowance(agentAddress, chain.router.target), 0n);
  await step('frozen buy', 'buy', 'POST /v1/agent/actions/allowance-and-call');
  t.diagnostic(JSON.stringify({ refusal: 'frozen buy', status: 403, code: 'agent_frozen' }));
  assert.equal((await owner('POST', '/unfreeze')).is_frozen, false);
  await step('unfreeze', 'agent_identity', 'POST /v1/agents/:did/unfreeze');
  const next = await quote('buy', 1000000n);
  const afterUnfreeze = await settle(await tool('buy', tradeInput(next)), 'buy', 1000000n, 'buy');
  expectedCoin += BigInt(next.outputRaw);
  expectedQuote -= 1000000n;
  await step(
    'unfrozen buy',
    'quote, buy, action',
    'POST /v1/agent/actions/allowance-and-call; GET /v1/agent/actions/:id',
    afterUnfreeze,
  );

  for (const asset of [token, usdl]) {
    const amount = await asset.balanceOf(agentAddress);
    const before = await asset.balanceOf(ownerAddress);
    const swept = await owner('POST', '/sweep', { token: asset.target, amount: String(amount) });
    assert.equal(swept.to.toLowerCase(), ownerAddress.toLowerCase());
    const sent = await receipt(swept.tx_hash, agentAddress, asset.target);
    assert.equal(
      sent.transaction.data,
      asset.interface.encodeFunctionData('transfer', [ownerAddress, amount]),
    );
    assert.equal(await asset.balanceOf(ownerAddress), before + amount);
    if (asset === token) expectedCoin = 0n;
    else expectedQuote = 0n;
    activityHashes.push(swept.tx_hash);
    await step(
      asset === token ? 'sweep coin' : 'sweep quote',
      'balances',
      'POST /v1/agents/:did/sweep',
      [swept.tx_hash],
    );
  }
  const reserve = 1000000000000000n;
  const nativeAmount = (await provider.getBalance(agentAddress)) - reserve;
  assert.ok(nativeAmount > 0n);
  const ownerBeforeSweep = await provider.getBalance(ownerAddress);
  const nativeSweep = await owner('POST', '/sweep', { amount: String(nativeAmount) });
  await receipt(nativeSweep.tx_hash, agentAddress, ownerAddress, nativeAmount);
  assert.equal(await provider.getBalance(ownerAddress), ownerBeforeSweep + nativeAmount);
  activityHashes.push(nativeSweep.tx_hash);
  await step(
    'sweep native',
    'balances',
    'POST /v1/agents/:did/sweep',
    [nativeSweep.tx_hash],
    -nativeAmount,
  );
  assert.ok((await provider.getBalance(agentAddress)) < reserve);
  async function agentActivityIncludesFundingAndExecution() {
    const activity = await owner('GET', '/activity');
    const hashes = activity.events.map((event) => event.tx_hash);
    for (const hash of activityHashes)
      assert.equal(hashes.filter((value) => value === hash).length, 1, hash);
    assert.equal(new Set(hashes).size, hashes.length);
    assert.equal(hashes.length, activityHashes.length);
  }
  await t.test(agentActivityIncludesFundingAndExecution);
  await step('activity', 'balances', 'GET /v1/agents/:did/activity');
  assert.ok(data.reads.some((read) => read.route === '/v2/pools'));
  assert.ok(data.reads.some((read) => read.route === `/v2/pools/${pool.toLowerCase()}`));
  assert.ok(
    data.reads.some((read) => read.route === `/v3/wallets/${agentAddress.toLowerCase()}/portfolio`),
  );
  t.diagnostic(
    'Chain-backed data stand-in serves list_coins, coin, quote pool metadata and positions. Quotes, balances, allowances, signatures and receipts use the gateway and Anvil. Portfolio cost basis is unknown. LayerX binding remains pending.',
  );
}

test(
  agentWalksTheLaunchpadOnALocalChain.name,
  { timeout: 1200000 },
  agentWalksTheLaunchpadOnALocalChain,
);
