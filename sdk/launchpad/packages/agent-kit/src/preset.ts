import { getAddress } from 'viem';
import { DEPLOYMENT, executionCalls, type LaunchpadDeployment } from './contracts.js';
import { requireThat } from './errors.js';

export interface PresetCaps {
  ownerWallet: string;
  maxTxValueWei: string;
  maxDailyValueWei: string;
  maxApproveWei: string;
  budgetCapWei: string;
  expiresInSeconds: number;
  sellTokens?: readonly string[];
  rateLimitPerMinute?: number;
}

export function rawAmount(value: string, positive = true): bigint {
  requireThat(
    typeof value === 'string' && /^(0|[1-9][0-9]{0,77})$/.test(value),
    'invalid_raw_amount',
  );
  const amount = BigInt(value);
  requireThat(amount <= (1n << 256n) - 1n && (!positive || amount > 0n), 'invalid_raw_amount');
  return amount;
}

export function launchpadPreset(caps: PresetCaps, deployment: LaunchpadDeployment = DEPLOYMENT) {
  const owner = getAddress(caps.ownerWallet);
  requireThat(BigInt(owner) !== 0n, 'invalid_owner_wallet');
  const perTx = rawAmount(caps.maxTxValueWei, false);
  const perDay = rawAmount(caps.maxDailyValueWei, false);
  rawAmount(caps.maxApproveWei, false);
  rawAmount(caps.budgetCapWei);
  requireThat(perDay >= perTx, 'invalid_daily_cap');
  requireThat(
    Number.isInteger(caps.expiresInSeconds) &&
      caps.expiresInSeconds > 0 &&
      caps.expiresInSeconds <= 31536000,
    'invalid_budget_expiry',
  );
  const rate = caps.rateLimitPerMinute ?? 60;
  requireThat(Number.isInteger(rate) && rate >= 1 && rate <= 10000, 'invalid_rate_limit');
  const tokens = [
    ...new Set(
      [deployment.usdl, ...(caps.sellTokens ?? [])].map((value) => getAddress(value).toLowerCase()),
    ),
  ];
  requireThat(
    tokens.length <= 101 &&
      tokens.every((value) => BigInt(value) !== 0n && value !== deployment.router.toLowerCase()),
    'invalid_sell_tokens',
  );
  const contracts = [deployment.router.toLowerCase(), ...tokens];
  const selectors = [
    ...new Set(Object.values(executionCalls(deployment)).map((call) => call.selector)),
  ];
  return {
    policy: {
      mode: 'trade_only' as const,
      max_tx_value_wei: caps.maxTxValueWei,
      max_daily_value_wei: caps.maxDailyValueWei,
      max_approve_wei: caps.maxApproveWei,
      rate_limit_per_min: rate,
      allow_native_transfer: false,
      withdrawal_allowlist_only: true,
    },
    rules: [
      {
        effect: 'allow' as const,
        subject: 'token' as const,
        value: deployment.usdl.toLowerCase(),
        max_value_wei: caps.maxTxValueWei,
      },
      ...contracts.map((value) => ({
        effect: 'allow' as const,
        subject: 'contract' as const,
        value,
      })),
      ...selectors.map((value) => ({
        effect: 'allow' as const,
        subject: 'selector' as const,
        value,
      })),
      {
        effect: 'allow' as const,
        subject: 'address' as const,
        value: deployment.spender.toLowerCase(),
      },
      { effect: 'allow' as const, subject: 'withdrawal' as const, value: owner.toLowerCase() },
    ],
    budget: {
      target_contract: deployment.router,
      token: deployment.usdl,
      cap_wei: caps.budgetCapWei,
      expires_in_seconds: caps.expiresInSeconds,
    },
  };
}
