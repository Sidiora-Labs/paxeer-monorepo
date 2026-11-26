import type { PublicWallet } from './types';

export type AgentMode = 'read_only' | 'trade_only' | 'full';
export interface OwnedAgent {
  did: string;
  label: string;
  address: `0x${string}` | null;
  mode: AgentMode;
  is_frozen: boolean;
  created_at: string;
  last_seen_at: string | null;
}
export interface AgentPolicy {
  mode: AgentMode;
  max_tx_value_wei: string;
  max_daily_value_wei: string;
  max_approve_wei: string;
  rate_limit_per_min: number;
  allow_native_transfer: boolean;
  withdrawal_allowlist_only: boolean;
  daily_reset_utc_hour: number;
}
export interface AgentRuleInput {
  effect: 'allow' | 'deny';
  subject: 'contract' | 'selector' | 'token' | 'address' | 'withdrawal';
  value: string;
  max_value_wei?: string | null;
  note?: string | null;
}
export interface AgentRule extends AgentRuleInput { id: string }
export interface AgentBudgetInput {
  target_contract?: `0x${string}` | null;
  token?: `0x${string}` | null;
  cap_wei: string;
  expires_in_seconds: number;
}
export interface AgentBudget {
  id: string;
  target_contract: string | null;
  token: string | null;
  cap_wei: string;
  spent_wei: string;
  remaining_wei: string;
  expires_at: string;
}
export interface AgentDetail {
  did: string;
  label: string;
  owner_user_id: string;
  is_frozen: boolean;
  wallet: { id: string; address: `0x${string}`; native_wei: string | null } | null;
  policy: AgentPolicy;
  rules: AgentRule[];
  budgets: AgentBudget[];
}
export interface AgentActivity {
  kind: string;
  to_address: string | null;
  value_wei: string;
  tx_hash: `0x${string}` | null;
  created_at: string;
}
export interface AgentTransfer { amount: string; token?: `0x${string}` | null }
export interface AgentTransferResult { tx_hash: `0x${string}`; from: `0x${string}`; to: `0x${string}` }
export interface AgentClaim { expires: string; signature: string }
export interface AgentClaimResult { did: string; owner_user_id: string }
export interface AgentChallenge { did: string; nonce: string; expires_in: number; message: string }
export interface AgentVerification { did: string; public_key: string; nonce: string; signature: string }
export interface AgentVerificationResult { token: string; expires_in: number; owner_user_id: string | null; is_frozen: boolean; wallet_provisioned: boolean }
export interface AgentProvisionResult {
  wallet: PublicWallet;
  provision: {
    state: string;
    awaiting: 'owner' | 'agent_signature' | null;
    address: `0x${string}`;
    did: string | null;
    bindMessage: `0x${string}` | null;
  };
}
export interface AgentRequestHeaders {
  'x-agent-key': string;
  'x-agent-nonce': string;
  'x-agent-expires': string;
  'x-agent-signature': string;
}
