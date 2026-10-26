/**
 * Failure copy: decode the vault's custom errors and the precompile's
 * revert reasons into one plain sentence. Unknown failures are reported as
 * unknown — never dressed up as something specific.
 */
import { BaseError, decodeErrorResult, isHex, UserRejectedRequestError } from 'viem';

import { paxeerXVaultAbi } from './contracts';

function humanizePrecompileReason(reason: string): string | undefined {
  const normalized = reason.replace(/^execution reverted:\s*/i, '').trim();
  if (!normalized) return undefined;
  const match = normalized.match(/LXP_BRIDGE_([A-Z_]+):/);
  if (match) {
    const sentence = PRECOMPILE_CODES[match[1]];
    if (sentence) return sentence;
  }
  return normalized.endsWith('.') ? normalized : `${ normalized }.`;
}

const PRECOMPILE_CODES: Readonly<Record<string, string>> = {
  UNAUTHORIZED: 'This account is not authorized to make this bridge request.',
  UNKNOWN_CHAIN: 'That chain is not registered with the bridge.',
  CHAIN_DISABLED: 'The bridge is disabled for that chain.',
  UNKNOWN_ASSET: 'That asset is not registered with the bridge.',
  PAUSED: 'The bridge is paused on this chain. Try again after it reopens.',
  NULLIFIED: 'This deposit has already been credited.',
  CAP_EXCEEDED: 'That amount exceeds the bridge cap for this asset.',
  BAD_SIGNATURE: 'An attestor signature did not verify.',
  BELOW_THRESHOLD: 'Not enough attestor signatures were provided.',
  VAULT_MISMATCH: 'That vault does not match the registered vault for this chain.',
  INVALID_REQUEST: 'The bridge request is not valid.',
};

const VAULT_ERRORS: Record<string, (args: readonly unknown[]) => string> = {
  WhenPaused: () => 'The bridge is paused on this chain. Try again after it reopens.',
  ZeroAmount: () => 'Enter an amount greater than zero.',
  InvalidRecipient: () => 'That destination address is not valid for this bridge.',
  AssetNotEnabled: () => 'This asset is not enabled on the vault yet.',
  PerTxCapExceeded: () => 'That amount is above the per-transaction cap for this asset.',
  TotalCapExceeded: () => 'The bridge\'s total capacity for this asset is reached right now.',
  InsufficientOutstanding: () => 'The vault does not hold enough of this asset to release that amount.',
  NullifierUsed: () => 'This withdrawal has already been released.',
  BelowThreshold: () => 'Not enough attestor signatures were provided.',
  InvalidSignature: () => 'An attestor signature did not verify.',
  SignersNotAscending: () => 'The attestor signatures were not in order.',
  UnknownSigner: () => 'A signature came from outside the attestor set.',
  UseDepositNative: () => 'The native coin must go through the native deposit path.',
  TransferAmountMismatch: () => 'The vault did not receive the full amount — fee-on-transfer tokens are not supported.',
  NativeTransferFailed: () => 'The vault could not deliver the native coin to the recipient.',
};

export function describeWriteError(error: unknown): string {
  if (error instanceof BaseError) {
    if (error.walk((e) => e instanceof UserRejectedRequestError)) {
      return 'You rejected the request in your wallet.';
    }
    const revert = error.walk(
      (e) => e instanceof Error && e.name === 'ContractFunctionRevertedError',
    ) as { data?: { errorName?: string; args?: readonly unknown[] }; reason?: string } | null;
    if (revert?.data?.errorName) {
      const describe = VAULT_ERRORS[revert.data.errorName];
      if (describe) return describe(revert.data.args ?? []);
      return `The contract rejected this: ${ revert.data.errorName }.`;
    }
    if (typeof revert?.reason === 'string') {
      const text = humanizePrecompileReason(revert.reason);
      if (text) return text;
    }
    // Precompiles revert with reason strings; viem surfaces them verbatim.
    const match = error.shortMessage.match(/reverted with the following reason:\s*(.+)$/s);
    if (match?.[1]) {
      const text = humanizePrecompileReason(match[1].split('\n')[0]);
      if (text) return text;
    }
    const hexData = (error as { data?: unknown }).data;
    if (typeof hexData === 'string' && isHex(hexData) && hexData.length >= 10) {
      try {
        const decoded = decodeErrorResult({ abi: paxeerXVaultAbi, data: hexData });
        const describe = VAULT_ERRORS[decoded.errorName];
        if (describe) return describe(decoded.args ?? []);
        return `The contract rejected this: ${ decoded.errorName }.`;
      } catch { // Not a vault error selector — fall through to the generic line.
      }
    }
    return error.shortMessage ?
      `${ error.shortMessage.replace(/\.$/, '') }.` :
      'The transaction failed. No funds moved.';
  }
  if (error instanceof Error && error.message) return error.message;
  return 'Something went wrong. No funds moved.';
}
