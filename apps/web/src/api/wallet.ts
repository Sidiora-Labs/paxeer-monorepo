import {
  gasStation,
  GasStationError,
  ModuleError,
  type GasSubmissionStore,
  type SponsoredSubmissionEvidence,
} from '@paxeer/wallet';
import { sidioraAccountDid, sidioraStationMetadata } from './gas-station.ts';
import { windowWalletProvider } from './provider.ts';
import { gatewayTransactionReceipt } from './gateway-actions.ts';
import type { GatewayReceipt } from './gateway.ts';
import { copyEntry } from '../../copy/runtime.ts';
import { parseFeeAmount } from '../auth/native-fee-budget.ts';
import {
  SIDIORA_DECIMALS,
  SIDIORA_TOKEN,
  requestSidioraGasQuote,
  sponsoredBatchDigest,
  sendPrecompileCall,
  type PrecompileCall,
  type SidioraGasQuote,
} from './sdk.ts';

export type PrecompileSendOutcome =
  | Readonly<{ outcome: 'sent'; transactionHash: string; receipt?: GatewayReceipt }>
  | Readonly<{ outcome: 'unconfirmed'; transactionHash: string }>
  | Readonly<{ outcome: 'reverted'; transactionHash: string; receipt: GatewayReceipt }>
  | Readonly<{ outcome: 'cancelled' | 'rejected' | 'unavailable' | 'failed'; detail?: string }>;

/** Validates the server/wallet result before a market consumer reads it. */
export function isPrecompileSendOutcome(value: unknown): value is PrecompileSendOutcome {
  if (typeof value !== 'object' || value === null || Array.isArray(value) || !('outcome' in value))
    return false;
  const outcome = value as Record<string, unknown>;
  switch (outcome.outcome) {
    case 'cancelled':
    case 'rejected':
    case 'unavailable':
    case 'failed':
      return outcome.detail === undefined || typeof outcome.detail === 'string';
    case 'sent':
    case 'unconfirmed':
    case 'reverted': {
      if (
        typeof outcome.transactionHash !== 'string' ||
        !/^0x[0-9a-fA-F]{64}$/u.test(outcome.transactionHash)
      )
        return false;
      if (outcome.outcome === 'unconfirmed') return outcome.receipt === undefined;
      if (outcome.receipt === undefined) return outcome.outcome === 'sent';
      const receipt = outcome.receipt;
      if (typeof receipt !== 'object' || receipt === null || Array.isArray(receipt)) return false;
      const record = receipt as Record<string, unknown>;
      return (
        typeof record.transactionHash === 'string' &&
        record.transactionHash.toLowerCase() === outcome.transactionHash.toLowerCase() &&
        typeof record.blockNumber === 'bigint' &&
        record.blockNumber >= 0n &&
        record.status === (outcome.outcome === 'reverted' ? 'reverted' : 'success') &&
        Array.isArray(record.logs) &&
        record.logs.every((log: unknown) => {
          if (typeof log !== 'object' || log === null || Array.isArray(log)) return false;
          const entry = log as Record<string, unknown>;
          return (
            typeof entry.address === 'string' &&
            /^0x[0-9a-fA-F]{40}$/u.test(entry.address) &&
            typeof entry.data === 'string' &&
            /^0x(?:[0-9a-fA-F]{2})*$/u.test(entry.data) &&
            Array.isArray(entry.topics) &&
            entry.topics.every(
              (topic: unknown) => typeof topic === 'string' && /^0x[0-9a-fA-F]{64}$/u.test(topic)
            ) &&
            entry.blockNumber === record.blockNumber &&
            typeof entry.logIndex === 'bigint' &&
            entry.logIndex >= 0n &&
            typeof entry.transactionHash === 'string' &&
            typeof record.transactionHash === 'string' &&
            entry.transactionHash.toLowerCase() === record.transactionHash.toLowerCase()
          );
        })
      );
    }
    default:
      return false;
  }
}

export const RECEIPT_POLL_LIMIT = 20;
export const RECEIPT_POLL_INTERVAL_MS = 500;
export const SPONSORED_SUBMISSION_PENDING =
  'an earlier sponsored submission is still pending; try again once it settles';

const EVM_ADDRESS = /^0x[0-9a-fA-F]{40}$/u;
const USER_REJECTED_REQUEST = 4001;
const UNAUTHORIZED_REQUEST = 4100;
const PROVIDER_DISCONNECTED = 4900;
const CHAIN_DISCONNECTED = 4901;

export function walletSendFailure(error: unknown): PrecompileSendOutcome {
  const code =
    typeof error === 'object' && error !== null && 'code' in error ? error.code : undefined;
  const detail = error instanceof Error ? error.message : undefined;
  if (error instanceof GasStationError) {
    return {
      outcome:
        error.refusal.code === 'unavailable'
          ? 'unavailable'
          : error.refusal.code === 'cancelled'
            ? 'cancelled'
            : 'rejected',
    };
  }
  if (error instanceof ModuleError)
    return { outcome: error.code === 'unavailable' ? 'unavailable' : 'rejected' };
  if (code === USER_REJECTED_REQUEST) {
    return { outcome: 'cancelled' };
  }
  if (code === UNAUTHORIZED_REQUEST) {
    return { outcome: 'rejected' };
  }
  if (code === PROVIDER_DISCONNECTED || code === CHAIN_DISCONNECTED) {
    return { outcome: 'unavailable' };
  }
  return detail === undefined ? { outcome: 'failed' } : { outcome: 'failed', detail };
}

/** Asks the Paxeer wallet for its first account. */
export async function connectWalletAccount(): Promise<string | undefined> {
  const wallet = windowWalletProvider();
  if (wallet === undefined) {
    return undefined;
  }
  const response = await wallet.request({ method: 'eth_requestAccounts' });
  const first: unknown = Array.isArray(response) ? (response as readonly unknown[])[0] : undefined;
  return typeof first === 'string' && EVM_ADDRESS.test(first) ? first.toLowerCase() : undefined;
}

export type NativeSubmissionStage = 'approval' | 'write';
export type NativeSubmission = Readonly<{
  version: 1;
  chainId: '125';
  account: string;
  operation: string;
  stage: NativeSubmissionStage;
  transactionHash?: string;
}>;

const NATIVE_CONTEXT = 'layerx:private-native:v1:125:';

function nativeSubmissionKey(account: string): string {
  if (!EVM_ADDRESS.test(account)) throw new Error('Invalid native submission account');
  return NATIVE_CONTEXT + account.toLowerCase();
}

/** A reservation without a hash also blocks a second send after an interrupted wallet request. */
export function retainedWalletNativeSubmission(account: string): NativeSubmission | undefined {
  const raw = browserSubmissionStore().getItem(nativeSubmissionKey(account));
  if (raw === null) return undefined;
  if (raw.length > 1024) throw new Error('Invalid retained native submission');
  const value: unknown = JSON.parse(raw);
  if (typeof value !== 'object' || value === null || Array.isArray(value))
    throw new Error('Invalid retained native submission');
  const entry = value as Record<string, unknown>;
  if (
    entry.version !== 1 || entry.chainId !== '125' || entry.account !== account.toLowerCase() ||
    typeof entry.operation !== 'string' || !/^[a-z-]{1,32}$/u.test(entry.operation) ||
    (entry.stage !== 'approval' && entry.stage !== 'write') ||
    (entry.transactionHash !== undefined &&
      (typeof entry.transactionHash !== 'string' || !/^0x[0-9a-fA-F]{64}$/u.test(entry.transactionHash)))
  ) throw new Error('Invalid retained native submission');
  return entry as NativeSubmission;
}

/** Reconciles the existing identity only; this function never submits a transaction. */
export async function resumeWalletNativeSubmission(account: string): Promise<PrecompileSendOutcome | undefined> {
  const retained = retainedWalletNativeSubmission(account);
  if (retained === undefined) return undefined;
  if (retained.transactionHash === undefined)
    return { outcome: 'failed', detail: 'A wallet request was interrupted before its hash was retained. Reconcile it in the wallet before sending again.' };
  const transactionHash = retained.transactionHash;
  try {
    const wallet = windowWalletProvider();
    if (wallet === undefined || rpcQuantity(await wallet.request({ method: 'eth_chainId' })) !== 125n)
      return { outcome: 'unconfirmed', transactionHash };
    const receipt = await gatewayTransactionReceipt(transactionHash);
    if (receipt === undefined) return { outcome: 'sent', transactionHash };
    browserSubmissionStore().removeItem(nativeSubmissionKey(account));
    return { outcome: receipt.status === 'success' ? 'sent' : 'reverted', transactionHash, receipt };
  } catch {
    return { outcome: 'unconfirmed', transactionHash };
  }
}

/** Sends one SDK-built precompile call through the Paxeer wallet. */
export async function sendWalletPrecompileCall(
  from: string,
  build: () => PrecompileCall,
  sleep: (milliseconds: number) => Promise<void> = (milliseconds) =>
    new Promise((resolve) => setTimeout(resolve, milliseconds)),
  native?: Readonly<{ operation: string; stage: NativeSubmissionStage }>
): Promise<PrecompileSendOutcome> {
  const wallet = windowWalletProvider();
  if (wallet === undefined) {
    return { outcome: 'unavailable' };
  }
  let transactionHash: string | undefined;
  let reserved = false;
  try {
    if (native !== undefined) {
      const recovered = await resumeWalletNativeSubmission(from);
      if (recovered !== undefined) return recovered;
    }
    const preference = walletFeePreference();
    if (preference.currency === 'sidiora') {
      if (preference.maximum === undefined) return { outcome: 'rejected' };
      const call = build();
      const recovered = await resumeWalletSponsoredSubmission(from, call);
      if (recovered !== undefined) return recovered;
      const metadata = await sidioraStationMetadata();
      if (metadata === undefined) return { outcome: 'unavailable' };
      const chainId = rpcQuantity(await wallet.request({ method: 'eth_chainId' }));
      if (chainId.toString() !== metadata.chainId) return { outcome: 'rejected' };
      const module = gasStation(wallet, {
        chainId,
        sponsor: metadata.sponsor,
        paymaster: metadata.paymaster,
      });
      const nonce = await module.batchNonce(from);
      const gas = rpcQuantity(
        await wallet.request({
          method: 'eth_estimateGas',
          params: [
            {
              from,
              to: call.to,
              data: call.data,
              value: `0x${call.value.toString(16)}`,
            },
          ],
        })
      );
      const price = rpcQuantity(await wallet.request({ method: 'eth_gasPrice' }));
      const identity = await sidioraAccountDid(from);
      if (!identity.ok) return identity.failure;
      const quoted = await requestSidioraGasQuote(
        {
          account: from,
          did: identity.did,
          nonce,
          calls: [call],
          maxTokenAmount: preference.maximum,
          gasCost: gas * price,
        },
        chainId
      );
      if (!quoted.ok) return quoted.failure;
      const review = sidioraQuotePresentation(quoted.value);
      if (review === undefined) return { outcome: 'rejected' };
      const consent =
        typeof window !== 'undefined' &&
        window.confirm(
          `${review.consent}\nAccount: ${quoted.value.batch.account}\nNetwork: ${quoted.value.batch.chainId.toString()}`
        );
      return await sendWalletSponsoredBatch(quoted.value, consent ? review.identity : undefined);
    }
    const call = build();
    if (native !== undefined) {
      if (rpcQuantity(await wallet.request({ method: 'eth_chainId' })) !== 125n)
        return { outcome: 'rejected' };
      // Recheck after awaiting the provider so concurrent controls cannot overwrite an identity.
      if (retainedWalletNativeSubmission(from) !== undefined)
        return await resumeWalletNativeSubmission(from) ?? { outcome: 'unavailable' };
      const context: NativeSubmission = {
        version: 1, chainId: '125', account: from.toLowerCase(), ...native,
      };
      browserSubmissionStore().setItem(nativeSubmissionKey(from), JSON.stringify(context));
      reserved = true;
    }
    transactionHash = await sendPrecompileCall(wallet, from, call);
    if (native !== undefined) {
      const context: NativeSubmission = {
        version: 1, chainId: '125', account: from.toLowerCase(), ...native, transactionHash,
      };
      browserSubmissionStore().setItem(nativeSubmissionKey(from), JSON.stringify(context));
    }
    for (let attempt = 0; attempt < RECEIPT_POLL_LIMIT; attempt += 1) {
      if (attempt > 0) await sleep(RECEIPT_POLL_INTERVAL_MS);
      const receipt = await gatewayTransactionReceipt(transactionHash);
      if (receipt !== undefined) {
        if (native !== undefined) browserSubmissionStore().removeItem(nativeSubmissionKey(from));
        return {
          outcome: receipt.status === 'success' ? 'sent' : 'reverted',
          transactionHash,
          receipt,
        };
      }
    }
    return { outcome: 'sent', transactionHash };
  } catch (error) {
    // Only an explicit wallet refusal proves the reserved request was not broadcast.
    if (reserved && transactionHash === undefined && typeof error === 'object' && error !== null &&
      'code' in error && (error.code === 4001 || error.code === 4100)) {
      browserSubmissionStore().removeItem(nativeSubmissionKey(from));
    }
    return transactionHash === undefined
      ? walletSendFailure(error)
      : { outcome: 'unconfirmed', transactionHash };
  }
}

export type WalletFeePreference = Readonly<{ currency: 'paxeer' | 'sidiora'; maximum?: bigint }>;
const FEE_PREFERENCE = 'paxeer.wallet.fee';
let feePreference: WalletFeePreference = { currency: 'paxeer' };

export function walletFeePreference(): WalletFeePreference {
  if (typeof window === 'undefined') return feePreference;
  try {
    const stored = window.sessionStorage.getItem(FEE_PREFERENCE);
    if (stored === null) return feePreference;
    if (stored === 'paxeer') return { currency: 'paxeer' };
    const maximum = parseFeeAmount(stored, SIDIORA_DECIMALS);
    return maximum === undefined ? { currency: 'sidiora' } : { currency: 'sidiora', maximum };
  } catch {
    return feePreference;
  }
}

export function chooseWalletFee(
  currency: WalletFeePreference['currency'],
  maximum: string
): boolean {
  const parsed = parseFeeAmount(maximum, SIDIORA_DECIMALS);
  feePreference =
    currency === 'paxeer'
      ? { currency }
      : parsed === undefined
        ? { currency }
        : { currency, maximum: parsed };
  if (typeof window !== 'undefined') {
    try {
      window.sessionStorage.setItem(FEE_PREFERENCE, currency === 'paxeer' ? currency : maximum);
    } catch {
      return currency === 'paxeer' || parsed !== undefined;
    }
  }
  return currency === 'paxeer' || parsed !== undefined;
}

export function sidioraAmount(amount: bigint): string {
  const scale = 10n ** BigInt(SIDIORA_DECIMALS);
  return `${(amount / scale).toString()}.${(amount % scale).toString().padStart(SIDIORA_DECIMALS, '0')}`;
}

export function sidioraQuotePresentation(
  quoted: SidioraGasQuote,
  now = BigInt(Math.floor(Date.now() / 1000))
) {
  try {
    const quote = quoted.batch.quote;
    if (
      !EVM_ADDRESS.test(quoted.paymaster) ||
      !/^0x[0-9a-fA-F]{130}$/u.test(quoted.relayerSignature)
    )
      return undefined;
    const digest = sponsoredBatchDigest(quoted.batch);
    if (
      !digest.ok ||
      quote.token.toLowerCase() !== SIDIORA_TOKEN.toLowerCase() ||
      quote.decimals !== SIDIORA_DECIMALS ||
      quote.tokenAmount <= 0n ||
      quote.tokenAmount > quote.maxTokenAmount ||
      quote.deadline <= now ||
      quote.deadline > 8_640_000_000_000n
    )
      return undefined;
    const amount = sidioraAmount(quote.tokenAmount);
    const maximum = sidioraAmount(quote.maxTokenAmount);
    const deadline = new Date(Number(quote.deadline) * 1000).toUTCString();
    const consent = copyEntry('gas.sidiora.consent')
      .message.replace('{amount}', amount)
      .replace('{maximum}', maximum)
      .replace('{deadline}', deadline);
    return {
      amount,
      maximum,
      deadline,
      consent,
      identity: `${digest.value}:${quoted.paymaster.toLowerCase()}:${quoted.relayerSignature.toLowerCase()}`,
    };
  } catch {
    return undefined;
  }
}

function rpcQuantity(value: unknown): bigint {
  if (typeof value !== 'string' || !/^0x[0-9a-fA-F]+$/u.test(value))
    throw new Error(copyEntry('gas.sidiora.failed').message);
  return BigInt(value);
}

const SPONSORED_CONTEXT = 'layerx:private-sponsored:v1:';

export function sponsoredWalletModule(
  quoted: SidioraGasQuote,
  wallet: NonNullable<ReturnType<typeof windowWalletProvider>>,
  submissionStore?: GasSubmissionStore
) {
  return gasStation(wallet, {
    chainId: quoted.batch.chainId,
    sponsor: quoted.batch.quote.sponsor,
    paymaster: quoted.paymaster,
    ...(submissionStore === undefined ? {} : { submissionStore }),
  });
}

function browserSubmissionStore(): Storage {
  if (typeof window === 'undefined') throw new Error('Durable sponsored storage is unavailable');
  return window.localStorage;
}

function sponsoredContextKey(account: string): string {
  if (!EVM_ADDRESS.test(account)) throw new Error('Invalid sponsored account');
  return SPONSORED_CONTEXT + account.toLowerCase();
}

async function statusOutcome(report: SponsoredSubmissionEvidence): Promise<PrecompileSendOutcome> {
  if (report.status === 'cancelled') return { outcome: 'rejected' };
  if (report.tx_hash === null)
    return { outcome: report.status === 'reverted' ? 'rejected' : 'unavailable' };
  try {
    const receipt = await gatewayTransactionReceipt(report.tx_hash);
    if (receipt === undefined) {
      return {
        outcome: report.status === 'reverted' ? 'unconfirmed' : 'sent',
        transactionHash: report.tx_hash,
      };
    }
    return {
      outcome: receipt.status === 'success' ? 'sent' : 'reverted',
      transactionHash: report.tx_hash,
      receipt,
    };
  } catch {
    return { outcome: 'unconfirmed', transactionHash: report.tx_hash };
  }
}

function sameCalls(calls: readonly PrecompileCall[], call: PrecompileCall): boolean {
  const [first] = calls;
  return (
    calls.length === 1 && first !== undefined &&
    first.to.toLowerCase() === call.to.toLowerCase() &&
    first.value === call.value &&
    first.data.toLowerCase() === call.data.toLowerCase()
  );
}

/**
 * Resumes the account's retained sponsored submission. Given `call`, a retained
 * submission is driven to completion but never reported as a new call's outcome
 * once it has finished. A different call fails with a pending detail while it
 * is still pending.
 */
export async function resumeWalletSponsoredSubmission(
  account: string,
  call?: PrecompileCall
): Promise<PrecompileSendOutcome | undefined> {
  const wallet = windowWalletProvider();
  if (wallet === undefined) return { outcome: 'unavailable' };
  try {
    const store = browserSubmissionStore();
    const raw = store.getItem(sponsoredContextKey(account));
    if (raw === null) return undefined;
    if (raw.length > 1024) return { outcome: 'rejected' };
    const value: unknown = JSON.parse(raw);
    if (typeof value !== 'object' || value === null || Array.isArray(value))
      return { outcome: 'rejected' };
    const context = value as Record<string, unknown>;
    if (
      Object.keys(context).length !== 6 ||
      context.version !== 1 ||
      context.account !== account.toLowerCase() ||
      typeof context.chainId !== 'string' ||
      !/^[1-9][0-9]*$/.test(context.chainId) ||
      BigInt(context.chainId) >= 1n << 256n ||
      typeof context.sponsor !== 'string' ||
      !EVM_ADDRESS.test(context.sponsor) ||
      typeof context.paymaster !== 'string' ||
      !EVM_ADDRESS.test(context.paymaster) ||
      typeof context.identity !== 'string' ||
      !/^0x[0-9a-fA-F]{64}:0x[0-9a-fA-F]{40}:0x[0-9a-fA-F]{130}$/.test(context.identity)
    )
      return { outcome: 'rejected' };
    const module = gasStation(wallet, {
      chainId: BigInt(context.chainId),
      sponsor: context.sponsor,
      paymaster: context.paymaster,
      submissionStore: store,
    });
    const retained = await module.pending(account);
    if (retained === null) {
      store.removeItem(sponsoredContextKey(account));
      return undefined;
    }
    const digest = sponsoredBatchDigest(retained.batch);
    if (
      !digest.ok ||
      context.identity !==
        `${digest.value}:${context.paymaster.toLowerCase()}:${retained.relayerSignature.toLowerCase()}`
    )
      return { outcome: 'rejected' };
    const report = await module.resume(retained.batch, retained.relayerSignature);
    if (report.status !== 'pending') {
      store.removeItem(sponsoredContextKey(account));
      return undefined;
    }
    if (call !== undefined && !sameCalls(retained.batch.calls, call))
      return { outcome: 'failed', detail: SPONSORED_SUBMISSION_PENDING };
    return await statusOutcome(report);
  } catch (error) {
    return walletSendFailure(error);
  }
}

export async function sendWalletSponsoredBatch(
  quoted: SidioraGasQuote,
  consentIdentity: string | undefined
): Promise<PrecompileSendOutcome> {
  if (consentIdentity === undefined) return { outcome: 'cancelled' };
  let snapshot: SidioraGasQuote;
  try {
    snapshot = structuredClone(quoted);
  } catch {
    return { outcome: 'rejected' };
  }
  const review = sidioraQuotePresentation(snapshot);
  if (review === undefined || review.identity !== consentIdentity) return { outcome: 'rejected' };
  const wallet = windowWalletProvider();
  if (wallet === undefined) return { outcome: 'unavailable' };
  let transactionHash: string | undefined;
  try {
    const storage = browserSubmissionStore();
    const module = sponsoredWalletModule(snapshot, wallet, storage);
    const pending = await module.pending(snapshot.batch.account);
    if (pending !== null) {
      const digest = sponsoredBatchDigest(pending.batch);
      const expected = sponsoredBatchDigest(snapshot.batch);
      if (
        !digest.ok ||
        !expected.ok ||
        digest.value !== expected.value ||
        pending.relayerSignature.toLowerCase() !== snapshot.relayerSignature.toLowerCase()
      )
        return { outcome: 'rejected' };
      const report = await module.resume(pending.batch, pending.relayerSignature);
      if (report.status !== 'pending')
        storage.removeItem(sponsoredContextKey(snapshot.batch.account));
      return await statusOutcome(report);
    }
    const key = sponsoredContextKey(snapshot.batch.account);
    const context = {
      version: 1,
      account: snapshot.batch.account.toLowerCase(),
      chainId: snapshot.batch.chainId.toString(),
      sponsor: snapshot.batch.quote.sponsor.toLowerCase(),
      paymaster: snapshot.paymaster.toLowerCase(),
      identity: consentIdentity,
    };
    const existing = storage.getItem(key);
    if (existing !== null && existing !== JSON.stringify(context)) return { outcome: 'rejected' };
    storage.setItem(key, JSON.stringify(context));
    transactionHash = await module.submitFirstUse(snapshot.batch, snapshot.relayerSignature, {
      confirm: (consent) => {
        const current = sidioraQuotePresentation(snapshot);
        const digest = sponsoredBatchDigest(snapshot.batch);
        return (
          current !== undefined &&
          current.identity === consentIdentity &&
          digest.ok &&
          consent.batchDigest === digest.value
        );
      },
    });
    const report = await module.resume(snapshot.batch, snapshot.relayerSignature);
    if (report.status !== 'pending') storage.removeItem(key);
    const receipt = await gatewayTransactionReceipt(transactionHash);
    return receipt === undefined
      ? { outcome: 'sent', transactionHash }
      : { outcome: receipt.status === 'success' ? 'sent' : 'reverted', transactionHash, receipt };
  } catch (error) {
    return transactionHash === undefined
      ? walletSendFailure(error)
      : { outcome: 'unconfirmed', transactionHash };
  }
}
