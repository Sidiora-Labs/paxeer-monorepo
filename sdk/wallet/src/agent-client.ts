import { signAgentRequest, type SignAgentRequestInput } from './agent.js';
import { GatewayHttpClient, gatewayBearer } from './gateway-client.js';
import type { AgentActivity, AgentPolicy } from './owner.js';
import type { SequencerReceiptView, SignTxResponse, SignMessageResponse } from './types.js';

export interface AgentChallengeRequest {
  did: string;
}
export interface AgentChallengeResponse {
  did: string;
  nonce: string;
  expires_in: number;
  message: string;
}
export interface AgentVerifyRequest {
  did: string;
  public_key: string;
  nonce: string;
  signature: string;
}
export interface AgentTokenResponse {
  token: string;
  token_type: 'Bearer';
  expires_in: number;
  did: string;
  scope: 'read';
  owner_user_id: string | null;
  is_frozen: boolean;
}
export interface AgentVerifyResponse extends AgentTokenResponse {
  wallet_provisioned: boolean;
}
export interface AgentWallet {
  id: string;
  address: `0x${string}`;
  chain_id: number;
}
export interface AgentWhoamiResponse {
  did: string;
  owner_user_id: string | null;
  label: string;
  is_frozen: boolean;
  created_at: string;
  last_seen_at: string | null;
  wallet: AgentWallet | null;
  policy: AgentPolicy;
}
export interface AgentMeResponse {
  did: string;
  owner_user_id: string | null;
  is_frozen: boolean;
  wallet: AgentWallet | null;
  policy: Omit<AgentPolicy, 'daily_reset_utc_hour'>;
  chain: { id: number; rpc_url: string };
}
export interface AgentProvisionRequest {
  bind_signature?: string;
}
export interface AgentProvisionResponse {
  wallet: AgentWallet & { kind: 'agent' };
  provision: {
    state:
      | 'new'
      | 'evm_key'
      | 'identity'
      | 'bind_signed'
      | 'funded'
      | 'bind_sent'
      | 'active'
      | 'refused';
    awaiting: 'owner' | 'agent_signature' | null;
    walletId: string;
    address: `0x${string}`;
    did: string | null;
    mainAccountId: string | null;
    bindMessage: `0x${string}` | null;
  };
}
export interface AgentBalancesResponse {
  address: `0x${string}`;
  native_wei: string | null;
  tokens: {
    token: string;
    symbol: string | null;
    decimals: number | null;
    balance: string | null;
  }[];
}

/** Wire transaction fields: amounts and fees are decimal integer strings. */
export interface AgentTransactionRequest {
  to?: `0x${string}`;
  value?: string;
  data?: `0x${string}`;
  gas?: string;
  maxFeePerGas?: string;
  maxPriorityFeePerGas?: string;
  nonce?: number;
  chainId?: number;
}
export interface AgentTransactionBody {
  tx: AgentTransactionRequest;
}
export interface AgentSimulationResponse {
  ok: boolean;
  gas: string | null;
  return_data: `0x${string}` | null;
  error: string | null;
}
export interface AgentAllowanceQuery {
  token: `0x${string}`;
  spender: `0x${string}`;
}
export interface AgentAllowanceResponse {
  token: string;
  spender: string;
  allowance: string;
}
export interface AgentTransactionReceipt {
  status: 'success' | 'reverted';
  block_number: string;
  gas_used: string;
  tx_hash: `0x${string}`;
}
export interface AgentSignResponse extends SignTxResponse {
  op: string;
  broadcast_deadline: string;
}
export type AgentGatewayReceipt = { txHash: `0x${string}`; nonce: `0x${string}` } & (
  | { rung: 'submitted' }
  | (Omit<SequencerReceiptView, 'paymentDenom' | 'signingKey'> & { rung: 'receipted' })
  | (Omit<SequencerReceiptView, 'paymentDenom' | 'signingKey'> & {
      rung: 'committed';
      blockNumber: `0x${string}`;
      blockHash: `0x${string}`;
    })
);
export interface AgentSendResponse {
  tx_hash: `0x${string}`;
  address: `0x${string}`;
  chain_id: number;
  op: string;
  receipt: AgentGatewayReceipt | null;
  rung: AgentGatewayReceipt['rung'];
}
/** An accepted broadcast whose receipt must be polled without resubmitting. */
export interface AgentBroadcastOutcomeUnknown {
  error: 'outcome_unknown';
  tx_hash: `0x${string}`;
  nonce: number;
  retry: 'poll_receipt';
  poll: 'eth_getTransactionReceipt';
  must_not_resubmit: true;
  member: number | null;
  members: number;
}
export type AgentBroadcastResponse = AgentSendResponse | AgentBroadcastOutcomeUnknown;
export interface AgentSignMessageRequest {
  message: string;
}
export interface AgentSignTypedDataRequest {
  typedData: {
    domain?: Record<string, unknown>;
    types: Record<string, unknown>;
    primaryType: string;
    message: Record<string, unknown>;
    [key: string]: unknown;
  };
}
export interface AgentTransferRequest {
  token?: `0x${string}` | null;
  to: `0x${string}`;
  amount: string;
}
export interface AgentApproveRequest {
  token: `0x${string}`;
  spender: `0x${string}`;
  amount: string;
}
export interface AgentAllowanceAndCallRequest extends AgentApproveRequest {
  contract: `0x${string}`;
  method: string;
  args: string[];
  idempotency_key: string;
}
export type AgentActionStatus =
  | 'received'
  | 'validating'
  | 'awaiting_approval'
  | 'approval_pending'
  | 'approval_confirmed'
  | 'simulating_call'
  | 'call_pending'
  | 'confirmed'
  | 'reconciling'
  | 'policy_denied'
  | 'invalid_request'
  | 'simulation_reverted'
  | 'transaction_reverted'
  | 'insufficient_balance'
  | 'infrastructure_unavailable'
  | 'failed_terminal';
export type AgentActionPhase =
  | 'intake'
  | 'validation'
  | 'balance_check'
  | 'allowance_check'
  | 'approval_broadcast'
  | 'approval_confirmation'
  | 'call_simulation'
  | 'call_broadcast'
  | 'call_confirmation'
  | 'credit_verification'
  | 'reconciliation';
export type AgentActionErrorCode =
  | 'POLICY_DENIED'
  | 'INVALID_REQUEST'
  | 'INVALID_AMOUNT'
  | 'FORBIDDEN_DESTINATION'
  | 'SIMULATION_REVERTED'
  | 'TRANSACTION_REVERTED'
  | 'CUSTODY_AUTHORITY_REQUIRED'
  | 'AGENT_REAUTHORIZATION_REQUIRED'
  | 'AGENT_FROZEN'
  | 'INSUFFICIENT_BALANCE'
  | 'INSUFFICIENT_GAS'
  | 'APPROVAL_CAP_EXCEEDED'
  | 'APPROVAL_PENDING'
  | 'CALL_PENDING'
  | 'RECONCILING'
  | 'RPC_UNAVAILABLE'
  | 'NONCE_WEDGED'
  | 'FEE_TOO_LOW';
export interface AgentActionError {
  ok: false;
  action_id: string;
  phase: AgentActionPhase;
  code: AgentActionErrorCode;
  message: string;
  cause: {
    tx_hash?: string | null;
    nonce?: number | null;
    detail?: string | null;
    [key: string]: unknown;
  };
  retry: {
    allowed: boolean;
    strategy: 'NEVER' | 'POLL_ACTION' | 'RETRY_SAME_ACTION' | 'USER_INTERVENTION';
    after_ms: number | null;
    reuse_idempotency_key: boolean;
    must_not_resubmit: boolean;
  };
  remedy: {
    action: 'GET_ACTION' | 'NONE' | 'OWNER_INTERVENTION' | 'AGENT_REAUTHORIZATION';
    endpoint: string | null;
  };
}
export interface AgentActionResponse {
  ok: boolean | null;
  action_id: string;
  kind: 'allowance_and_call';
  status: AgentActionStatus;
  phase: string | null;
  terminal: boolean;
  approval: { tx_hash: string; nonce: number | null } | null;
  call: { tx_hash: string; nonce: number | null } | null;
  credit_verified: boolean;
  credit: Record<string, unknown> | null;
  error: AgentActionError | null;
  poll: { endpoint: string; after_ms: number | null };
  created_at: string;
  updated_at: string;
  terminal_at: string | null;
  replay?: boolean;
}

export interface AgentSigningOptions extends Pick<
  SignAgentRequestInput,
  'privateKey' | 'keyId' | 'nonce' | 'expiresAt' | 'nowSeconds'
> {
  /** ID returned by the gateway's signing-authorization upload route. */
  authorizationId?: string;
}

/** Agent reads take an agent token; writes are signed over the transmitted JSON bytes. */
export class AgentClient extends GatewayHttpClient {
  private async readRequest<T>(
    token: string,
    method: string,
    path: string,
    body?: unknown
  ): Promise<T> {
    return this.request(
      method,
      path,
      gatewayBearer(token),
      body === undefined ? undefined : JSON.stringify(body)
    );
  }

  private async signedRequest<T>(
    path: string,
    body: unknown,
    signing: AgentSigningOptions
  ): Promise<T> {
    const serialized = JSON.stringify(body);
    const headers = signAgentRequest({
      ...signing,
      method: 'POST',
      url: `${this.apiUrl}${path}`,
      body: serialized,
    });
    return this.request(
      'POST',
      path,
      {
        ...headers,
        ...(signing.authorizationId === undefined
          ? {}
          : { 'x-agent-attestor-authorization-id': signing.authorizationId }),
      },
      serialized
    );
  }

  challenge(body: AgentChallengeRequest): Promise<AgentChallengeResponse> {
    return this.request('POST', '/v1/agent/auth/challenge', {}, JSON.stringify(body));
  }

  verify(body: AgentVerifyRequest): Promise<AgentVerifyResponse> {
    return this.request('POST', '/v1/agent/auth/verify', {}, JSON.stringify(body));
  }

  refresh(token: string): Promise<AgentTokenResponse> {
    return this.readRequest(token, 'POST', '/v1/agent/auth/refresh');
  }

  whoami(token: string): Promise<AgentWhoamiResponse> {
    return this.readRequest(token, 'GET', '/v1/agent/whoami');
  }

  provision(
    body: AgentProvisionRequest,
    signing: AgentSigningOptions
  ): Promise<AgentProvisionResponse> {
    return this.signedRequest('/v1/agent/provision', body, signing);
  }

  me(token: string): Promise<AgentMeResponse> {
    return this.readRequest(token, 'GET', '/v1/agent/me');
  }

  balances(token: string, tokens: readonly `0x${string}`[] = []): Promise<AgentBalancesResponse> {
    const query = tokens.length ? `?${new URLSearchParams({ tokens: tokens.join(',') })}` : '';
    return this.readRequest(token, 'GET', `/v1/agent/balances${query}`);
  }

  nonce(token: string): Promise<{ address: `0x${string}`; nonce: number }> {
    return this.readRequest(token, 'GET', '/v1/agent/nonce');
  }

  simulate(token: string, body: AgentTransactionBody): Promise<AgentSimulationResponse> {
    return this.readRequest(token, 'POST', '/v1/agent/simulate', body);
  }

  allowance(token: string, query: AgentAllowanceQuery): Promise<AgentAllowanceResponse> {
    return this.readRequest(
      token,
      'GET',
      `/v1/agent/allowance?${new URLSearchParams({ token: query.token, spender: query.spender })}`
    );
  }

  transaction(token: string, hash: `0x${string}`): Promise<AgentTransactionReceipt> {
    return this.readRequest(token, 'GET', `/v1/agent/tx/${encodeURIComponent(hash)}`);
  }

  activity(token: string): Promise<AgentActivity> {
    return this.readRequest(token, 'GET', '/v1/agent/activity');
  }

  sign(body: AgentTransactionBody, signing: AgentSigningOptions): Promise<AgentSignResponse> {
    return this.signedRequest('/v1/agent/sign', body, signing);
  }

  send(body: AgentTransactionBody, signing: AgentSigningOptions): Promise<AgentBroadcastResponse> {
    return this.signedRequest('/v1/agent/send', body, signing);
  }

  signMessage(
    body: AgentSignMessageRequest,
    signing: AgentSigningOptions
  ): Promise<SignMessageResponse> {
    return this.signedRequest('/v1/agent/sign-message', body, signing);
  }

  signTypedData(
    body: AgentSignTypedDataRequest,
    signing: AgentSigningOptions
  ): Promise<SignMessageResponse> {
    return this.signedRequest('/v1/agent/sign-typed-data', body, signing);
  }

  transfer(
    body: AgentTransferRequest,
    signing: AgentSigningOptions
  ): Promise<AgentBroadcastResponse> {
    return this.signedRequest('/v1/agent/transfer', body, signing);
  }

  approve(
    body: AgentApproveRequest,
    signing: AgentSigningOptions
  ): Promise<AgentBroadcastResponse> {
    return this.signedRequest('/v1/agent/approve', body, signing);
  }

  allowanceAndCall(
    body: AgentAllowanceAndCallRequest,
    signing: AgentSigningOptions
  ): Promise<AgentActionResponse> {
    return this.signedRequest('/v1/agent/actions/allowance-and-call', body, signing);
  }

  action(token: string, actionId: string): Promise<AgentActionResponse> {
    return this.readRequest(token, 'GET', `/v1/agent/actions/${encodeURIComponent(actionId)}`);
  }
}
