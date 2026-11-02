import ts from 'typescript';
import { expect, expectTypeOf, test } from 'vitest';
import type {
  WalletBudgetCap,
  WalletCapsHistory,
  WalletCapsSnapshot,
  WalletCapsState,
  WalletGrantCap,
} from '../src/kernel.js';

const packageDir = decodeURIComponent(new URL('..', import.meta.url).pathname);
const config = ts.readConfigFile(`${packageDir}tsconfig.json`, ts.sys.readFile);
const parsed = ts.parseJsonConfigFileContent(config.config, ts.sys, packageDir, {
  noEmit: true,
  rootDir: packageDir,
});
const testPath = decodeURIComponent(new URL(import.meta.url).pathname);
const program = ts.createProgram([testPath], parsed.options);
const testSource = program.getSourceFile(testPath);
const diagnostics = [...parsed.errors, ...ts.getPreEmitDiagnostics(program, testSource)];

function wallet_caps_snapshot_is_observed_at_a_block_on_a_rung() {
  const owner = `0x${'11'.repeat(20)}`;
  const budget: WalletBudgetCap = {
    id: '1',
    owner,
    asset: `0x${'22'.repeat(20)}`,
    account: '3',
    source_account: null,
    limit: '100',
    configured_limit: '100',
    spent: '25',
    remaining: '75',
    carry_cap: '0',
    carried: '0',
    period_start: '1000',
    period_length: '3600',
    expiry: '10000',
    revocation_height: '899',
    closed: false,
    revoked: false,
  };
  const grant: WalletGrantCap = {
    id: '4',
    owner,
    recipient: `0x${'33'.repeat(20)}`,
    asset: budget.asset,
    per_draw_maximum: '10',
    allowance: '100',
    drawn_total: '25',
    drawn_this_period: '5',
    recurring: true,
    window_length: '3600',
    window_start: '1000',
    expiration: '10000',
    revocation_height: '899',
    revoked_at_height: '0',
    revoked: false,
    invoice_settled: false,
  };
  const committed: WalletCapsState = {
    state: 'ready',
    did: `did:layerx:${'44'.repeat(32)}`,
    account_id: budget.account,
    network_id: 125,
    context: {
      principal: owner,
      session_id: 'session-1',
      address: owner,
      chain_id: 125,
      expires_at: '10000',
    },
    observation: {
      rung: 'committed',
      block_number: '900',
      state_root: `0x${'55'.repeat(32)}`,
    },
    budgets: [budget],
    grants: [grant],
    history: { source: 'indexer', indexed_height: '899', chain_height: '900' },
  };
  const receipted: WalletCapsSnapshot = {
    ...committed,
    observation: { ...committed.observation, rung: 'receipted' },
    history: { source: 'chain_scan', floor: '800', head: '900' },
  };
  const empty: WalletCapsState = { ...committed, state: 'empty', budgets: [], grants: [] };
  const unavailable: WalletCapsState = { state: 'unavailable', reason: 'consensus_unavailable' };

  expectTypeOf<WalletCapsSnapshot['observation']>().toEqualTypeOf<{
    readonly rung: 'receipted' | 'committed';
    readonly block_number: string;
    readonly state_root: string;
  }>();
  expectTypeOf<{
    verification: 'state_proven';
    state_root: string;
    sequence: string;
    observed_head: string;
    batch: string;
  }>().not.toMatchTypeOf<WalletCapsSnapshot['observation']>();
  expectTypeOf<WalletBudgetCap['revocation_height']>().toEqualTypeOf<string>();
  expectTypeOf<WalletGrantCap['revocation_height']>().toEqualTypeOf<string>();
  expectTypeOf<WalletGrantCap['revoked_at_height']>().toEqualTypeOf<string>();
  expectTypeOf<Extract<keyof WalletBudgetCap, 'revocation_sequence'>>().toEqualTypeOf<never>();
  expectTypeOf<Extract<keyof WalletGrantCap, 'revocation_sequence' | 'revoked_at_sequence'>>().toEqualTypeOf<never>();
  expectTypeOf<WalletCapsHistory>().toEqualTypeOf<
    | { readonly source: 'indexer'; readonly indexed_height: string | null; readonly chain_height: string }
    | { readonly source: 'chain_scan'; readonly floor: string; readonly head: string }
  >();
  expectTypeOf<Extract<WalletCapsState, { state: 'unavailable' }>['reason']>().toEqualTypeOf<'consensus_unavailable'>();

  expect(committed.state).toBe('ready');
  expect(committed.observation).toEqual({
    rung: 'committed',
    block_number: '900',
    state_root: `0x${'55'.repeat(32)}`,
  });
  expect(receipted.observation).toEqual({ ...committed.observation, rung: 'receipted' });
  expect(committed.grants).toEqual([grant]);
  expect(committed.budgets).toEqual([budget]);
  expect(grant.revocation_height).toBe('899');
  expect(grant.revoked_at_height).toBe('0');
  expect(budget.revocation_height).toBe('899');
  expect(committed.history).toEqual({ source: 'indexer', indexed_height: '899', chain_height: '900' });
  expect(receipted.history).toEqual({ source: 'chain_scan', floor: '800', head: '900' });
  expect(empty.state).toBe('empty');
  expect(unavailable).toEqual({ state: 'unavailable', reason: 'consensus_unavailable' });

  expect(config.error).toBeUndefined();
  expect(testSource).toBeDefined();
  expect(diagnostics.map((diagnostic) => ts.flattenDiagnosticMessageText(diagnostic.messageText, '\n'))).toEqual([]);
}

test(wallet_caps_snapshot_is_observed_at_a_block_on_a_rung, wallet_caps_snapshot_is_observed_at_a_block_on_a_rung);
