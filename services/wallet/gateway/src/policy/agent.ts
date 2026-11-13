import { decodeFunctionData, encodeFunctionData, type Hex } from 'viem';
import { PRECOMPILE_ADDRESSES, SCHEDULER_ABI, STREAMS_ABI } from '../precompiles.js';
import { publicClient } from '../chainReads.js';
import { agentWalletUserId, findWalletByUserId } from '../db/wallets.js';
import { env } from '../env.js';
import {
  countAgentSignaturesLastMinute,
  reserveBudget as reserveAgentBudget,
  sumAgentDailyValue,
  type AgentPolicyRow,
  type AgentPolicyRuleRow,
  type AgentPrincipalRow
} from '../db/agents.js';

export const ERC20_APPROVE_SELECTOR = '0x095ea7b3';
export const ERC20_TRANSFER_SELECTOR = '0xa9059cbb';
export const ERC20_TRANSFER_FROM_SELECTOR = '0x23b872dd';

export function decodeAddressArg(data: string, argIndex: number): string | null {
  const start = 2 + 8 + 64 * argIndex;
  const end = start + 64;
  if (data.length < end) return null;
  const slot = data.slice(start, end);
  if (!/^0{24}[0-9a-fA-F]{40}$/.test(slot)) return null;
  return `0x${slot.slice(24).toLowerCase()}`;
}

export function decodeUintArg(data: string, argIndex: number): bigint | null {
  const start = 2 + 8 + 64 * argIndex;
  const end = start + 64;
  if (data.length < end) return null;
  const slot = data.slice(start, end);
  if (!/^[0-9a-fA-F]{64}$/.test(slot)) return null;
  return BigInt(`0x${slot}`);
}

export function decodeMoverCounterparty(
  selector: string,
  data: string
): { counterparty: string | null; isApprove: boolean; known: boolean } {
  switch (selector.toLowerCase()) {
    case ERC20_APPROVE_SELECTOR:
    case '0x39509351':
    case '0x3177029f':
    case '0xcae9ca51':
    case '0xa22cb465':
      return {
        counterparty: decodeAddressArg(data, 0),
        isApprove: true,
        known: true
      };
    case ERC20_TRANSFER_SELECTOR:
    case '0x1296ee62':
    case '0x4000aea0':
      return {
        counterparty: decodeAddressArg(data, 0),
        isApprove: false,
        known: true
      };
    case ERC20_TRANSFER_FROM_SELECTOR:
    case '0x42842e0e':
    case '0xb88d4fde':
    case '0xf242432a':
    case '0x2eb2c2d6':
      return {
        counterparty: decodeAddressArg(data, 1),
        isApprove: false,
        known: true
      };
    case '0xac9650d8':
    case '0x5ae401dc':
      return { counterparty: null, isApprove: false, known: true };
    default:
      return { counterparty: null, isApprove: false, known: false };
  }
}

/**
 * Agent policy engine (v2).
 *
 * Evaluated top-to-bottom, fail-closed on every hard limit. Returns a
 * structured decision so routes can shape a precise 4xx and, on allow, release
 * any reserved budget if the subsequent send fails.
 *
 *   0. Freeze            → AGENT_FROZEN (owner kill switch)
 *   1. Mode              → MODE_READ_ONLY (read_only blocks all signing)
 *   2. Shape             → CONTRACT_CREATION_BLOCKED (creation is permitted only
 *                          for full-mode agents) / INVALID_TX_SHAPE
 *   3. Denylist          → CONTRACT_DENYLISTED / SELECTOR_DENYLISTED /
 *                          TOKEN_DENYLISTED / ADDRESS_DENYLISTED (deny beats allow)
 *   4. Allowlist         → CONTRACT_NOT_ALLOWLISTED (trade_only, when allow rules exist)
 *   5. Withdrawal        → WITHDRAWAL_NOT_ALLOWLISTED (recipient/spender must be allowed)
 *   6. Native transfer   → NATIVE_TRANSFER_BLOCKED (unless allow_native_transfer)
 *   7. Approve cap       → APPROVE_CAP
 *   8. Per-rule cap      → TOKEN_VALUE_CAP (allow-token rule's max_value_wei)
 *   9. Rate limit        → RATE_LIMIT
 *  10. Value caps        → TX_VALUE_CAP / DAILY_VALUE_CAP (budgets are the
 *                          escape hatch above these for native value)
 */

export type AgentDenyCode =
  | 'AGENT_FROZEN'
  | 'MODE_READ_ONLY'
  | 'CONTRACT_CREATION_BLOCKED'
  | 'INVALID_TX_SHAPE'
  | 'CONTRACT_DENYLISTED'
  | 'SELECTOR_DENYLISTED'
  | 'TOKEN_DENYLISTED'
  | 'ADDRESS_DENYLISTED'
  | 'CONTRACT_NOT_ALLOWLISTED'
  | 'WITHDRAWAL_NOT_ALLOWLISTED'
  | 'COUNTERPARTY_UNDECODED'
  | 'NATIVE_TRANSFER_BLOCKED'
  | 'APPROVE_CAP'
  | 'TOKEN_VALUE_CAP'
  | 'TX_VALUE_CAP'
  | 'DAILY_VALUE_CAP'
  | 'RATE_LIMIT';

export type AgentDecision =
  | { allow: true; reservedBudgetId: string | null; reservedValueWei: bigint }
  | {
      allow: false;
      code: AgentDenyCode;
      message: string;
      detail?: Record<string, unknown>;
    };

export interface AgentTxIntent {
  kind: 'transaction' | 'message' | 'typed_data';
  to?: string;
  value?: bigint;
  data?: string;
  /**
   * Structured annotations from high-level routes (transfer/approve). When
   * absent for a raw tx, the engine decodes asset-moving calldata itself.
   * Known movers always use the decoded counterparty for policy checks.
   */
  tokenContract?: string | null;
  tokenRecipient?: string | null;
  tokenAmount?: bigint | null;
  isApprove?: boolean;
}

export interface EffectivePolicy {
  mode: AgentPolicyRow['mode'];
  maxTxValueWei: bigint;
  maxDailyValueWei: bigint;
  rateLimitPerMin: number;
  maxApproveWei: bigint;
  allowNativeTransfer: boolean;
  withdrawalAllowlistOnly: boolean;
  dailyResetUtcHour: number;
}

/** Resolve a policy row against the env defaults (NULL → default). */
export function effectivePolicy(p: AgentPolicyRow | null): EffectivePolicy {
  return {
    mode: p?.mode ?? env.AGENT_DEFAULT_MODE,
    maxTxValueWei: p?.max_tx_value_wei ?? env.AGENT_DEFAULT_MAX_TX_VALUE_WEI,
    maxDailyValueWei: p?.max_daily_value_wei ?? env.AGENT_DEFAULT_MAX_DAILY_VALUE_WEI,
    rateLimitPerMin: p?.rate_limit_per_min ?? env.AGENT_DEFAULT_RATE_LIMIT_PER_MINUTE,
    maxApproveWei: p?.max_approve_wei ?? env.AGENT_DEFAULT_MAX_APPROVE_WEI,
    allowNativeTransfer: p?.allow_native_transfer ?? false,
    withdrawalAllowlistOnly: p?.withdrawal_allowlist_only ?? true,
    dailyResetUtcHour: p?.daily_reset_utc_hour ?? 0
  };
}

interface RuleIndex {
  denyContracts: Set<string>;
  denySelectors: Set<string>;
  denyTokens: Set<string>;
  denyAddresses: Set<string>;
  allowContracts: Set<string>;
  allowTokenCaps: Map<string, bigint | null>; // token -> per-tx cap (null = uncapped)
  hasAllowContracts: boolean;
  withdrawalAllowlist: Set<string>;
}

function indexRules(rules: AgentPolicyRuleRow[]): RuleIndex {
  const idx: RuleIndex = {
    denyContracts: new Set(),
    denySelectors: new Set(),
    denyTokens: new Set(),
    denyAddresses: new Set(),
    allowContracts: new Set(),
    allowTokenCaps: new Map(),
    hasAllowContracts: false,
    withdrawalAllowlist: new Set()
  };
  for (const r of rules) {
    const v = r.value.toLowerCase();
    if (r.effect === 'deny') {
      if (r.subject === 'contract') idx.denyContracts.add(v);
      else if (r.subject === 'selector') idx.denySelectors.add(v);
      else if (r.subject === 'token') idx.denyTokens.add(v);
      else if (r.subject === 'address') idx.denyAddresses.add(v);
    } else {
      if (r.subject === 'contract') {
        idx.allowContracts.add(v);
        idx.hasAllowContracts = true;
      } else if (r.subject === 'token') {
        idx.allowTokenCaps.set(v, r.max_value_wei);
      } else if (r.subject === 'withdrawal') {
        idx.withdrawalAllowlist.add(v);
      }
    }
  }
  return idx;
}

function deny(
  code: AgentDenyCode,
  message: string,
  detail?: Record<string, unknown>
): AgentDecision {
  return { allow: false, code, message, detail };
}

/** Most-recent UTC daily-reset boundary as an ISO string. */
function dailyWindowStartIso(hour: number): string {
  const now = new Date();
  const start = new Date(
    Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate(), hour, 0, 0, 0)
  );
  if (start.getTime() > now.getTime()) start.setUTCDate(start.getUTCDate() - 1);
  return start.toISOString();
}

export interface EvaluateAgentInput {
  principal: AgentPrincipalRow;
  policy: AgentPolicyRow | null;
  rules: AgentPolicyRuleRow[];
  intent: AgentTxIntent;
  reserveBudget?: typeof reserveAgentBudget;
}

/**
 * Evaluate an agent signing intent. On allow for a value-moving native tx that
 * exceeded the normal caps, `reservedBudgetId` carries the budget that was
 * atomically charged — the route MUST release it if the send fails.
 */
export async function evaluateAgent(input: EvaluateAgentInput): Promise<AgentDecision> {
  return evaluateAgentIntent(input, false, 0);
}

async function evaluateAgentIntent(
  input: EvaluateAgentInput,
  movementOnly: boolean,
  depth: number
): Promise<AgentDecision> {
  const { principal } = input;
  let intent = input.intent;
  const eff = effectivePolicy(input.policy);

  // 0. Freeze — the instant kill switch.
  if (principal.is_frozen) {
    return deny('AGENT_FROZEN', 'agent is frozen by its owner; no actions permitted');
  }

  // 1. Mode — read_only forbids all signing (tx, message, typed-data).
  if (eff.mode === 'read_only') {
    return deny('MODE_READ_ONLY', 'agent policy mode is read_only; signing is disabled');
  }

  // Messages / typed-data carry no destination or transferable value at this
  // layer; the mode gate above is the only check.
  if (intent.kind !== 'transaction') {
    return { allow: true, reservedBudgetId: null, reservedValueWei: 0n };
  }

  const idx = indexRules(input.rules);
  const value = intent.value ?? 0n;
  const data = (intent.data ?? '0x').toLowerCase();
  const isEmptyData = data === '0x' || data === '';

  // 2. Shape — contract creation. Permitted only for full-mode agents (the
  //    trusted tier that already bypasses the trade allowlist). A creation tx
  //    has no destination contract / selector / token to gate, so the
  //    denylist, allowlist and withdrawal checks below do not apply; only the
  //    native value caps + rate limit guard it.
  if (!intent.to) {
    if (eff.mode !== 'full') {
      return deny(
        'CONTRACT_CREATION_BLOCKED',
        'contract creation requires a full-mode agent policy'
      );
    }
    if (value > eff.maxTxValueWei) {
      return deny('TX_VALUE_CAP', 'deployment value exceeds the per-tx cap', {
        value: value.toString(),
        cap: eff.maxTxValueWei.toString()
      });
    }
    if (value > 0n) {
      const windowStart = dailyWindowStartIso(eff.dailyResetUtcHour);
      const dailySpent = await sumAgentDailyValue(principal.did, windowStart);
      if (dailySpent + value > eff.maxDailyValueWei) {
        return deny('DAILY_VALUE_CAP', 'daily value cap exceeded by this deployment', {
          used: dailySpent.toString(),
          request: value.toString(),
          cap: eff.maxDailyValueWei.toString()
        });
      }
    }
    const limited = await rateLimited(principal.did, eff.rateLimitPerMin);
    if (limited) return limited;
    return { allow: true, reservedBudgetId: null, reservedValueWei: value };
  }
  const to = intent.to.toLowerCase();
  if (idx.denyContracts.has(to)) {
    return deny('CONTRACT_DENYLISTED', 'target contract is on the deny list', {
      contract: to
    });
  }
  const signedSelector = !isEmptyData && data.length >= 10 ? data.slice(0, 10) : null;
  if (signedSelector && idx.denySelectors.has(signedSelector)) {
    return deny('SELECTOR_DENYLISTED', 'method selector is on the deny list', {
      selector: signedSelector
    });
  }

  let scheduled: AgentTxIntent | null = null;
  if (to === PRECOMPILE_ADDRESSES.scheduler || to === PRECOMPILE_ADDRESSES.streams) {
    if (depth >= 16)
      return deny('COUNTERPARTY_UNDECODED', 'scheduled call nesting exceeds the policy bound');
    const decoded = await decodePrecompilePolicy(intent, principal.did);
    if (!decoded)
      return deny(
        'COUNTERPARTY_UNDECODED',
        'precompile movement cannot be decoded from signed calldata'
      );
    intent = decoded.intent;
    scheduled = decoded.scheduled;
  }

  // Decode selector + asset-moving operands from calldata.
  let selector: string | null = null;
  let tokenContract = intent.tokenContract?.toLowerCase() ?? null;
  let tokenRecipient = intent.tokenRecipient?.toLowerCase() ?? null;
  let tokenAmount = intent.tokenAmount ?? null;
  let isApprove = intent.isApprove ?? false;
  let mover: ReturnType<typeof decodeMoverCounterparty> | null = null;

  if (!isEmptyData) {
    if (data.length < 10) {
      return deny('INVALID_TX_SHAPE', 'tx.data must be empty or contain a 4-byte selector');
    }
    selector = data.slice(0, 10);
    mover = decodeMoverCounterparty(selector, data);
    // Auto-decode known movers so token/withdrawal gates apply even
    // on a raw sign/send (not just the high-level helper routes).
    if (intent.tokenContract === undefined || mover.known) {
      if (selector === ERC20_TRANSFER_SELECTOR) {
        tokenContract = to;
        tokenRecipient = decodeAddressArg(data, 0);
        tokenAmount = decodeUintArg(data, 1);
      } else if (selector === ERC20_APPROVE_SELECTOR) {
        tokenContract = to;
        tokenRecipient = decodeAddressArg(data, 0);
        isApprove = true;
        tokenAmount = decodeUintArg(data, 1);
      } else if (selector === ERC20_TRANSFER_FROM_SELECTOR) {
        tokenContract = to;
        tokenRecipient = decodeAddressArg(data, 1);
        tokenAmount = decodeUintArg(data, 2);
      } else if (mover.known) {
        tokenContract = to;
        tokenRecipient = mover.counterparty;
        isApprove = mover.isApprove;
      }
    }
  }

  if (
    movementOnly &&
    mover?.known &&
    (mover.counterparty === null ||
      ![
        ERC20_APPROVE_SELECTOR,
        ERC20_TRANSFER_SELECTOR,
        ERC20_TRANSFER_FROM_SELECTOR
      ].includes(selector!))
  ) {
    return deny('COUNTERPARTY_UNDECODED', 'scheduled movement has no decodable counterparty');
  }
  if (
    movementOnly &&
    (selector === ERC20_TRANSFER_SELECTOR ||
      selector === ERC20_TRANSFER_FROM_SELECTOR ||
      selector === ERC20_APPROVE_SELECTOR) &&
    (tokenRecipient === null || tokenAmount === null)
  ) {
    return deny('COUNTERPARTY_UNDECODED', 'scheduled token movement is malformed');
  }

  // 3. Denylist — deny always beats allow.
  if (tokenContract && idx.denyTokens.has(tokenContract)) {
    return deny('TOKEN_DENYLISTED', 'token is on the deny list', {
      token: tokenContract
    });
  }
  if (tokenRecipient && idx.denyAddresses.has(tokenRecipient)) {
    return deny('ADDRESS_DENYLISTED', 'counterparty address is on the deny list', {
      address: tokenRecipient
    });
  }

  // 4. Allowlist — in trade_only, when explicit allow-contract rules exist, the
  //    target MUST be allowlisted. `full` mode skips the allowlist requirement.
  if (eff.mode === 'trade_only' && idx.hasAllowContracts && !idx.allowContracts.has(to)) {
    return deny('CONTRACT_NOT_ALLOWLISTED', 'target contract is not on the trade allowlist', {
      contract: to
    });
  }

  // 5. Withdrawal allowlist — any recipient that can RECEIVE funds (native
  //    transfer target, token recipient, spender or operator) must be
  //    on the allowlist when withdrawal_allowlist_only is set.
  if (eff.withdrawalAllowlistOnly) {
    if (mover?.known && mover.counterparty === null) {
      return deny(
        'COUNTERPARTY_UNDECODED',
        'known asset-moving call has no decodable counterparty',
        {
          selector
        }
      );
    }
    const recipientNeedingAllow: string | null =
      value > 0n && isEmptyData
        ? to // bare native transfer → funds go to `to`
        : (tokenRecipient ?? null); // token recipient / spender / operator
    if (recipientNeedingAllow && !idx.withdrawalAllowlist.has(recipientNeedingAllow)) {
      return deny(
        'WITHDRAWAL_NOT_ALLOWLISTED',
        'recipient/spender is not on the withdrawal allowlist',
        { recipient: recipientNeedingAllow }
      );
    }
  }

  // 6. Native transfer switch — a bare PAX transfer is a withdrawal vector.
  if (value > 0n && isEmptyData && !eff.allowNativeTransfer) {
    return deny('NATIVE_TRANSFER_BLOCKED', 'native PAX transfers are disabled for this agent');
  }

  // 7. Approve cap.
  if (isApprove) {
    const approveAmount = tokenAmount ?? decodeApproveAmount(data);
    if (approveAmount !== null && approveAmount > eff.maxApproveWei) {
      return deny('APPROVE_CAP', 'approve amount exceeds the per-agent approval cap', {
        amount: approveAmount.toString(),
        cap: eff.maxApproveWei.toString()
      });
    }
  }

  // 8. Per-rule token cap (allow-token rule's max_value_wei = per-tx ceiling).
  if (tokenContract && idx.allowTokenCaps.has(tokenContract)) {
    const cap = idx.allowTokenCaps.get(tokenContract);
    if (
      cap !== null &&
      cap !== undefined &&
      tokenAmount === null &&
      (selector === ERC20_TRANSFER_SELECTOR || selector === ERC20_TRANSFER_FROM_SELECTOR)
    ) {
      return deny('INVALID_TX_SHAPE', 'token transfer has no decodable amount', {
        token: tokenContract,
        selector
      });
    }
    if (cap !== null && cap !== undefined && tokenAmount !== null && tokenAmount > cap) {
      return deny('TOKEN_VALUE_CAP', 'token amount exceeds the per-tx cap for this token', {
        token: tokenContract,
        amount: tokenAmount.toString(),
        cap: cap.toString()
      });
    }
  }

  if (scheduled) {
    const nested = await evaluateAgentIntent({ ...input, intent: scheduled }, true, depth + 1);
    if (!nested.allow) return nested;
  }
  if (movementOnly) return { allow: true, reservedBudgetId: null, reservedValueWei: 0n };

  // 9. Rate limit, before any budget reservation.
  const limited = await rateLimited(principal.did, eff.rateLimitPerMin);
  if (limited) return limited;

  // 10. Native value caps. Budgets are the escape hatch ABOVE the normal caps.
  let reservedBudgetId: string | null = null;
  if (value > 0n) {
    const withinPerTx = value <= eff.maxTxValueWei;
    const windowStart = dailyWindowStartIso(eff.dailyResetUtcHour);
    const dailySpent = await sumAgentDailyValue(principal.did, windowStart);
    const withinDaily = dailySpent + value <= eff.maxDailyValueWei;

    if (!withinPerTx || !withinDaily) {
      // Try to draw the overage from an owner-granted budget covering `to`.
      const reserveBudget = input.reserveBudget ?? reserveAgentBudget;
      reservedBudgetId = await reserveBudget({
        did: principal.did,
        targetContract: to,
        token: null,
        valueWei: value
      });
      if (!reservedBudgetId) {
        if (!withinPerTx) {
          return deny(
            'TX_VALUE_CAP',
            'tx value exceeds the per-tx cap and no budget covers it',
            {
              value: value.toString(),
              cap: eff.maxTxValueWei.toString()
            }
          );
        }
        return deny('DAILY_VALUE_CAP', 'daily value cap exceeded and no budget covers it', {
          used: dailySpent.toString(),
          request: value.toString(),
          cap: eff.maxDailyValueWei.toString()
        });
      }
    }
  }

  return { allow: true, reservedBudgetId, reservedValueWei: value };
}

/**
 * Shared rate-limit gate: deny when the agent exceeded its per-minute signing
 * budget. Fails open on a transient DB error (the value caps are the hard
 * guard). Returns null when within the limit.
 */
async function rateLimited(did: string, perMin: number): Promise<AgentDecision | null> {
  try {
    const recent = await countAgentSignaturesLastMinute(did);
    if (recent >= perMin) {
      return deny('RATE_LIMIT', `more than ${perMin} signing requests in 60s`);
    }
  } catch {
    // fail-open
  }
  return null;
}

/** Decode the `amount` (2nd arg) of an ERC-20 approve(address,uint256) calldata. */
function decodeApproveAmount(data: string): bigint | null {
  // selector(8) + arg0 address(64) + arg1 uint256(64); 0x prefix => offsets +2.
  const start = 2 + 8 + 64;
  const end = start + 64;
  if (data.length < end) return null;
  const slot = data.slice(start, end);
  if (!/^[0-9a-f]{64}$/.test(slot)) return null;
  return BigInt(`0x${slot}`);
}

/** Decode only signed operands; helper annotations never grant precompile authority. */
async function decodePrecompilePolicy(
  intent: AgentTxIntent,
  did: string
): Promise<{
  intent: AgentTxIntent;
  scheduled: AgentTxIntent | null;
} | null> {
  const to = intent.to!.toLowerCase();
  const data = (intent.data ?? '0x').toLowerCase() as Hex;
  const clean: AgentTxIntent = {
    kind: 'transaction',
    to,
    data,
    value: intent.value ?? 0n,
    tokenContract: null,
    tokenRecipient: null,
    tokenAmount: null,
    isApprove: false
  };
  try {
    if (to === PRECOMPILE_ADDRESSES.scheduler) {
      const call = decodeFunctionData({ abi: SCHEDULER_ABI, data });
      if (encodeFunctionData({ abi: SCHEDULER_ABI, ...call }).toLowerCase() !== data)
        return null;
      if (call.functionName === 'schedule') {
        const [target, callData] = call.args;
        clean.tokenRecipient = target.toLowerCase();
        return {
          intent: clean,
          scheduled: { kind: 'transaction', to: target, data: callData, value: 0n }
        };
      }
      // Rescheduling by id cannot establish the retained job's destination.
      if (call.functionName === 'reschedule') return null;
      return { intent: clean, scheduled: null };
    }
    const call = decodeFunctionData({ abi: STREAMS_ABI, data });
    if (encodeFunctionData({ abi: STREAMS_ABI, ...call }).toLowerCase() !== data) return null;
    if (call.functionName === 'open') {
      const [payee, token, , , , cap] = call.args;
      // A stream must carry a positive spend bound.
      if (cap === 0n) return null;
      clean.tokenRecipient = payee.toLowerCase();
      clean.tokenContract = token.toLowerCase();
      clean.tokenAmount = cap;
      return { intent: clean, scheduled: null };
    }
    if (
      call.functionName === 'settle' ||
      call.functionName === 'close' ||
      call.functionName === 'updateRate'
    ) {
      const [id] = call.args;
      const stream = await publicClient().readContract({
        address: PRECOMPILE_ADDRESSES.streams,
        abi: STREAMS_ABI,
        functionName: 'getStream',
        args: [id]
      });
      const wallet = await findWalletByUserId(agentWalletUserId(did), 'agent');
      if (
        !wallet ||
        !stream.active ||
        stream.id !== id ||
        stream.payer.toLowerCase() !== wallet.address.toLowerCase() ||
        BigInt(stream.payee) === 0n ||
        BigInt(stream.token) === 0n ||
        stream.cap === 0n ||
        stream.settled > stream.cap
      )
        return null;
      clean.tokenRecipient = stream.payee.toLowerCase();
      clean.tokenContract = stream.token.toLowerCase();
      clean.tokenAmount = stream.cap;
      return { intent: clean, scheduled: null };
    }
    return { intent: clean, scheduled: null };
  } catch {
    return null;
  }
}
