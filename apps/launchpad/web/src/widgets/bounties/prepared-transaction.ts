import { decodeFunctionData, encodeFunctionData, getAddress, type Address, type Hex } from 'viem';
import { erc20Abi } from 'sidiora-launchpad-v3';
import type { Bounty } from './bounties-contract';
import type { V2ContractTypes } from '@kindle/sdk';

export type TransactionRequest = V2ContractTypes['TransactionRequest'];

/** Deployment address map published by the runtime network authority (role name to address). */
export type DeploymentAddresses = Readonly<Record<string, string>>;

export type BountyTransactionIntent =
  | { action: 'funding'; bounty: Pick<Bounty, 'bountyId' | 'creatorWallet' | 'assetAddress' | 'budgetRaw'> }
  | { action: 'settlement'; bounty: Pick<Bounty, 'bountyId' | 'creatorWallet' | 'assetAddress' | 'budgetRaw'>; submissionId: string; winnerWallet: string; awardRaw: string };

function reviewedBounty(bounty: Bounty): BountyTransactionIntent['bounty'] {
  return Object.freeze({ bountyId: bounty.bountyId, creatorWallet: bounty.creatorWallet, assetAddress: bounty.assetAddress, budgetRaw: bounty.budgetRaw });
}

export function createFundingIntent(bounty: Bounty): BountyTransactionIntent {
  return Object.freeze({ action: 'funding', bounty: reviewedBounty(bounty) });
}

export function createSettlementIntent(bounty: Bounty, selection: { submissionId: string; winnerWallet: string; awardRaw: string }): BountyTransactionIntent {
  return Object.freeze({ action: 'settlement', bounty: reviewedBounty(bounty), submissionId: selection.submissionId, winnerWallet: selection.winnerWallet, awardRaw: selection.awardRaw });
}

export interface PreparedTransactionContext {
  /** Local review intent, independent of the prepared transaction and acceptance response. */
  intent: BountyTransactionIntent;
  /** Connected wallet that will sign; the prepared `from` must equal it. */
  wallet: string;
  /** Runtime authority deployment map; the prepared `to` must be one of these addresses. */
  deployment: DeploymentAddresses;
}

export interface ValidatedTransaction {
  chainId: 125;
  from: Address;
  to: Address;
  /** Authority role name of `to` (for the review step). */
  contractRole: string;
  data: Hex;
  value: bigint;
  gas: bigint | null;
  maxFeePerGas: bigint | null;
  maxPriorityFeePerGas: bigint | null;
}

export class PreparedTransactionError extends Error {
  constructor(readonly reason: 'chain' | 'from' | 'to' | 'calldata' | 'amount' | 'value' | 'intent' | 'operation', message: string) { super(message); }
}

const ADDRESS = /^0x[0-9a-fA-F]{40}$/;
const ZERO = /^0x0{40}$/;
const UINT = /^(0|[1-9][0-9]{0,77})$/;
/** Selector (4 bytes) plus whole bytes only. */
const CALLDATA = /^0x(?:[0-9a-fA-F]{2}){4,}$/;
const UINT256_LIMIT = 1n << 256n;

function uint(value: string, field: string): bigint {
  if (!UINT.test(value)) throw new PreparedTransactionError('amount', `Prepared ${field} is not an unsigned integer`);
  const parsed = BigInt(value);
  if (parsed >= UINT256_LIMIT) throw new PreparedTransactionError('amount', `Prepared ${field} exceeds uint256`);
  return parsed;
}

function optionalUint(value: string | null, field: string): bigint | null {
  return value === null ? null : uint(value, field);
}

/** Role name of `address` in the deployment map, or null when the authority does not publish it. */
export function deploymentRole(address: string, deployment: DeploymentAddresses): string | null {
  if (!ADDRESS.test(address)) return null;
  const target = address.toLowerCase();
  for (const [role, candidate] of Object.entries(deployment)) {
    if (typeof candidate === 'string' && ADDRESS.test(candidate) && !ZERO.test(candidate) && candidate.toLowerCase() === target) return role;
  }
  return null;
}

function authorizeBountyCall(tx: ValidatedTransaction, intent: BountyTransactionIntent): never {
  if (!intent || (intent.action !== 'funding' && intent.action !== 'settlement')) {
    throw new PreparedTransactionError('intent', 'A bounty funding or settlement intent is required');
  }
  const { bounty } = intent;
  if (!bounty.bountyId || !ADDRESS.test(bounty.creatorWallet) || ZERO.test(bounty.creatorWallet) || !ADDRESS.test(bounty.assetAddress) || ZERO.test(bounty.assetAddress)) {
    throw new PreparedTransactionError('intent', 'The reviewed bounty identity is invalid');
  }
  if (bounty.creatorWallet.toLowerCase() !== tx.from.toLowerCase()) {
    throw new PreparedTransactionError('intent', 'Only the reviewed bounty creator can authorize this operation');
  }
  const budget = uint(bounty.budgetRaw, 'bounty budget');
  const amount = intent.action === 'funding' ? budget : uint(intent.awardRaw, 'award');
  if (amount <= 0n || amount > budget) throw new PreparedTransactionError('amount', 'The reviewed amount is outside the bounty budget');
  if (intent.action === 'settlement' && (!intent.submissionId || !ADDRESS.test(intent.winnerWallet) || ZERO.test(intent.winnerWallet))) {
    throw new PreparedTransactionError('intent', 'The reviewed winner is invalid');
  }

  // The published token ABI can identify a malicious token movement, but cannot
  // establish a bounty escrow operation. Never promote a deployment role to one.
  if (tx.to.toLowerCase() !== bounty.assetAddress.toLowerCase()) {
    throw new PreparedTransactionError('operation', 'No deployed bounty escrow ABI authorizes this contract call');
  }
  let call: ReturnType<typeof decodeFunctionData<typeof erc20Abi>>;
  try {
    call = decodeFunctionData({ abi: erc20Abi, data: tx.data });
    if (encodeFunctionData({ abi: erc20Abi, functionName: call.functionName, args: call.args }).toLowerCase() !== tx.data.toLowerCase()) {
      throw new Error('Non-canonical calldata');
    }
  } catch {
    throw new PreparedTransactionError('calldata', 'Prepared calldata is not a canonical deployed token call');
  }
  if (call.functionName === 'approve') {
    throw new PreparedTransactionError('operation', 'No deployed bounty escrow authorizes this approval spender');
  }
  if (call.functionName !== 'transfer') {
    throw new PreparedTransactionError('operation', 'Prepared selector is not a bounty funding or settlement operation');
  }
  const [recipient, raw] = call.args;
  if (raw !== amount) throw new PreparedTransactionError('amount', 'Prepared token amount differs from the reviewed amount');
  if (intent.action === 'settlement' && recipient.toLowerCase() !== intent.winnerWallet.toLowerCase()) {
    throw new PreparedTransactionError('intent', 'Prepared token recipient differs from the reviewed winner');
  }
  // Chain 125's bounty producer returns null requests: there is no published
  // escrow contract/ABI that binds this transfer to bountyId (and submissionId).
  // Even an exact ERC20 transfer cannot fund or settle a bounty safely.
  throw new PreparedTransactionError('operation', 'Bounty funding and settlement are unavailable without a deployed escrow operation');
}

export function validatePreparedEnvelope(request: TransactionRequest, context: Omit<PreparedTransactionContext, 'intent'>): ValidatedTransaction {
  if (request.chainId !== '125') throw new PreparedTransactionError('chain', 'Prepared transaction targets another chain');
  if (!ADDRESS.test(context.wallet) || !ADDRESS.test(request.from) || request.from.toLowerCase() !== context.wallet.toLowerCase()) {
    throw new PreparedTransactionError('from', 'Prepared transaction signer is not the connected wallet');
  }
  if (!ADDRESS.test(request.to) || ZERO.test(request.to)) throw new PreparedTransactionError('to', 'Prepared transaction contract address is invalid');
  const contractRole = deploymentRole(request.to, context.deployment);
  if (contractRole === null) throw new PreparedTransactionError('to', 'Prepared transaction contract is not part of the attested deployment');
  if (typeof request.data !== 'string' || !CALLDATA.test(request.data)) throw new PreparedTransactionError('calldata', 'Prepared calldata is not valid hex');
  const value = uint(request.valueRaw, 'value');
  if (value !== 0n) throw new PreparedTransactionError('value', 'Prepared transaction would transfer native PAX principal');
  return {
    chainId: 125,
    from: getAddress(request.from),
    to: getAddress(request.to),
    contractRole,
    data: request.data as Hex,
    value,
    gas: optionalUint(request.gasLimitRaw, 'gas limit'),
    maxFeePerGas: optionalUint(request.maxFeePerGasRaw, 'max fee per gas'),
    maxPriorityFeePerGas: optionalUint(request.maxPriorityFeePerGasRaw, 'priority fee per gas'),
  };
}

/** Validate the envelope and bind it to the local review before any wallet access. */
export function validatePreparedTransaction(request: TransactionRequest, context: PreparedTransactionContext): ValidatedTransaction {
  const tx = validatePreparedEnvelope(request, context);
  authorizeBountyCall(tx, context.intent);
}

/** Upper bound of the PAX gas cost: gas limit times the fee per gas the wallet will be asked to pay. */
export function gasCostUpperBound(gas: bigint, feePerGas: bigint): bigint {
  if (gas <= 0n || feePerGas < 0n) throw new PreparedTransactionError('amount', 'Gas estimate is not positive');
  return gas * feePerGas;
}
