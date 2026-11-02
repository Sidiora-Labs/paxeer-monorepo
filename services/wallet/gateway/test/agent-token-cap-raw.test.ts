import { encodeFunctionData, erc20Abi } from 'viem';
import { afterAll, beforeAll, expect, test } from 'vitest';
import type { EvaluateAgentInput } from '../src/policy/agent.js';
import { newAgentKey } from './support/identity.js';
import { startPostgres, type EphemeralPostgres } from './support/postgres.js';

const token = '0x00000000000000000000000000000000000000a1';
const sender = '0x00000000000000000000000000000000000000b2';
const recipient = '0x00000000000000000000000000000000000000c3';
let postgres: EphemeralPostgres;
let database: typeof import('../src/db/pool.js');
let policy: typeof import('../src/policy/agent.js');
let input: Omit<EvaluateAgentInput, 'intent'>;

beforeAll(async () => {
  postgres = await startPostgres();
  database = await import('../src/db/pool.js');
  policy = await import('../src/policy/agent.js');
  const agents = await import('../src/db/agents.js');
  const key = newAgentKey('raw-token-cap');
  await agents.upsertPrincipalOnVerify({
    did: key.did,
    label: 'raw-token-cap',
    keyFingerprint: key.publicKeyHex,
    publicKey: key.publicKeyHex,
  });
  await agents.setPrincipalFrozen(key.did, false);
  const principal = await agents.findPrincipal(key.did);
  if (!principal) throw new Error('agent principal was not persisted');
  const agentPolicy = await agents.upsertPolicy(key.did, { mode: 'trade_only' }, null);
  await agents.addRule({
    did: key.did,
    effect: 'allow',
    subject: 'token',
    value: token,
    maxValueWei: 100n,
  });
  await agents.addRule({
    did: key.did,
    effect: 'allow',
    subject: 'withdrawal',
    value: recipient,
  });
  input = { principal, policy: agentPolicy, rules: await agents.listRules(key.did) };
}, 120_000);

afterAll(async () => {
  await database?.closePool();
  await postgres?.stop();
}, 120_000);

async function assertRawTokenCap(functionName: 'transfer' | 'transferFrom'): Promise<void> {
  const argIndex = functionName === 'transfer' ? 1 : 2;
  const calldata = (amount: bigint): string =>
    functionName === 'transfer'
      ? encodeFunctionData({ abi: erc20Abi, functionName, args: [recipient, amount] })
      : encodeFunctionData({ abi: erc20Abi, functionName, args: [sender, recipient, amount] });
  const evaluate = (data: string) =>
    policy.evaluateAgent({ ...input, intent: { kind: 'transaction', to: token, data } });

  for (const amount of [101n, 1n << 255n, (1n << 256n) - 1n]) {
    const data = calldata(amount);
    expect(policy.decodeUintArg(data, argIndex)).toBe(amount);
    expect(await evaluate(data)).toMatchObject({
      allow: false,
      code: 'TOKEN_VALUE_CAP',
      detail: { token, amount: amount.toString(), cap: '100' },
    });
  }

  for (const amount of [100n, 0n]) {
    const data = calldata(amount);
    expect(policy.decodeUintArg(data, argIndex)).toBe(amount);
    expect(await evaluate(data)).toEqual({
      allow: true,
      reservedBudgetId: null,
      reservedValueWei: 0n,
    });
  }

  const data = calldata(101n);
  const operands = data.slice(0, -64);
  for (const malformed of [operands, data.slice(0, -1), `${operands}${'g'.repeat(64)}`]) {
    expect(policy.decodeUintArg(malformed, argIndex)).toBeNull();
    expect(await evaluate(malformed)).toMatchObject({ allow: false, code: 'INVALID_TX_SHAPE' });
  }
}

const rawTokenCapTests = {
  async 'raw transfer above the token cap is denied'() {
    await assertRawTokenCap('transfer');
  },
  async 'raw transferFrom above the token cap is denied'() {
    await assertRawTokenCap('transferFrom');
  },
};

test(
  rawTokenCapTests['raw transfer above the token cap is denied'],
  rawTokenCapTests['raw transfer above the token cap is denied'],
);
test(
  rawTokenCapTests['raw transferFrom above the token cap is denied'],
  rawTokenCapTests['raw transferFrom above the token cap is denied'],
);
