import type { AgentClaim, AgentClaimResult } from './agent.js';
import { GatewayHttpClient, gatewayBearer, type GatewayClientOptions } from './gateway-client.js';

export type AgentMode = 'read_only' | 'trade_only' | 'full';

export interface AgentPolicy {
  mode: AgentMode;
  max_tx_value_wei: string;
  max_daily_value_wei: string;
  rate_limit_per_min: number;
  max_approve_wei: string;
  allow_native_transfer: boolean;
  withdrawal_allowlist_only: boolean;
  daily_reset_utc_hour: number;
}

export interface AgentPolicyPatch {
  mode?: AgentMode;
  max_tx_value_wei?: string | null;
  max_daily_value_wei?: string | null;
  rate_limit_per_min?: number | null;
  max_approve_wei?: string | null;
  allow_native_transfer?: boolean;
  withdrawal_allowlist_only?: boolean;
  daily_reset_utc_hour?: number;
}

export interface AgentRuleInput {
  effect: 'allow' | 'deny';
  subject: 'contract' | 'selector' | 'token' | 'address' | 'withdrawal';
  value: string;
  max_value_wei?: string | null;
  note?: string | null;
}

export interface AgentRule extends Omit<AgentRuleInput, 'max_value_wei' | 'note'> {
  id: string;
  max_value_wei: string | null;
  note: string | null;
}

export interface AgentBudgetInput {
  target_contract?: `0x${string}` | null;
  token?: `0x${string}` | null;
  cap_wei: string;
  expires_in_seconds: number;
}

export interface AgentBudgetCreated {
  id: string;
  target_contract: string | null;
  token: string | null;
  cap_wei: string;
  expires_at: string;
}

export interface AgentBudget extends AgentBudgetCreated {
  spent_wei: string;
  remaining_wei: string;
}

export interface OwnedAgentSummary {
  did: string;
  label: string;
  is_frozen: boolean;
  mode: AgentMode;
  address: `0x${string}` | null;
  created_at: string;
  last_seen_at: string | null;
}

export interface OwnedAgent extends Omit<OwnedAgentSummary, 'mode' | 'address'> {
  owner_user_id: string | null;
  wallet: { id: string; address: `0x${string}`; native_wei: string | null } | null;
  policy: AgentPolicy;
  rules: AgentRule[];
  budgets: AgentBudget[];
}

export interface AgentFundingInput {
  token?: `0x${string}` | null;
  amount: string;
}

export interface AgentSweepInput extends AgentFundingInput {
  to?: `0x${string}`;
}

export interface AgentTreasuryResult {
  tx_hash: `0x${string}`;
  from: `0x${string}`;
  to: `0x${string}`;
}

export interface AgentActivity {
  did: string;
  events: {
    kind: string;
    to_address: string | null;
    value_wei: string;
    tx_hash: string | null;
    created_at: string;
  }[];
}

export interface OwnerClientOptions extends GatewayClientOptions {
  token: string | (() => string | Promise<string>);
}

/** Owner control plane authenticated with the caller's signed-in user token. */
export class OwnerClient extends GatewayHttpClient {
  private readonly token: OwnerClientOptions['token'];

  constructor(options: OwnerClientOptions) {
    super(options);
    this.token = options.token;
  }

  private async call<T>(method: string, path: string, body?: unknown): Promise<T> {
    const token = typeof this.token === 'function' ? await this.token() : this.token;
    return this.request(
      method,
      path,
      gatewayBearer(token),
      body === undefined ? undefined : JSON.stringify(body)
    );
  }

  private agentPath(did: string): string {
    return `/v1/agents/${encodeURIComponent(did)}`;
  }

  list(): Promise<{ agents: OwnedAgentSummary[] }> {
    return this.call('GET', '/v1/agents');
  }

  read(did: string): Promise<OwnedAgent> {
    return this.call('GET', this.agentPath(did));
  }

  claim(did: string, claim: AgentClaim): Promise<AgentClaimResult> {
    return this.call('POST', `${this.agentPath(did)}/claim`, claim);
  }

  updatePolicy(
    did: string,
    patch: AgentPolicyPatch
  ): Promise<{ did: string; policy: AgentPolicy }> {
    return this.call('PUT', `${this.agentPath(did)}/policy`, patch);
  }

  freeze(did: string): Promise<{ did: string; is_frozen: true }> {
    return this.call('POST', `${this.agentPath(did)}/freeze`);
  }

  unfreeze(did: string): Promise<{ did: string; is_frozen: false }> {
    return this.call('POST', `${this.agentPath(did)}/unfreeze`);
  }

  listRules(did: string): Promise<{ did: string; rules: AgentRule[] }> {
    return this.call('GET', `${this.agentPath(did)}/rules`);
  }

  addRule(did: string, rule: AgentRuleInput): Promise<{ rule: AgentRule }> {
    return this.call('POST', `${this.agentPath(did)}/rules`, rule);
  }

  removeRule(did: string, ruleId: string): Promise<{ deleted: boolean }> {
    return this.call('DELETE', `${this.agentPath(did)}/rules/${encodeURIComponent(ruleId)}`);
  }

  listBudgets(did: string): Promise<{ did: string; budgets: AgentBudget[] }> {
    return this.call('GET', `${this.agentPath(did)}/budgets`);
  }

  addBudget(did: string, budget: AgentBudgetInput): Promise<{ budget: AgentBudgetCreated }> {
    return this.call('POST', `${this.agentPath(did)}/budgets`, budget);
  }

  removeBudget(did: string, budgetId: string): Promise<{ deactivated: boolean }> {
    return this.call('DELETE', `${this.agentPath(did)}/budgets/${encodeURIComponent(budgetId)}`);
  }

  fund(did: string, funding: AgentFundingInput): Promise<AgentTreasuryResult> {
    return this.call('POST', `${this.agentPath(did)}/fund`, funding);
  }

  sweep(did: string, sweep: AgentSweepInput): Promise<AgentTreasuryResult> {
    return this.call('POST', `${this.agentPath(did)}/sweep`, sweep);
  }

  activity(did: string): Promise<AgentActivity> {
    return this.call('GET', `${this.agentPath(did)}/activity`);
  }
}
