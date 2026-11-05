import { getAddress, isAddress, parseEventLogs } from 'viem';
import type { Address, Log, ParseEventLogsReturnType, TransactionReceipt } from 'viem';

import { bridgeChainById, type BridgeChain } from './chains';
import { bridgeConfig } from './config';
import { LAYERX_BRIDGE_ADDRESS, layerxBridgeAbi, paxeerXVaultAbi } from './contracts';
import { paxeerRecipient } from './digests';
import { base58Encode, recipientRecordAddress, recipientRecordFromResponse, solanaRecipientHandle } from './solana';
import type { SolanaCustodyConfig } from './solana-custody';
import { transferIdOf, type BridgeTransfer } from './transfers';

export function matchDepositLog(transfer: BridgeTransfer, logs: ReadonlyArray<Log>):
  ParseEventLogsReturnType<typeof paxeerXVaultAbi, 'BridgeDeposit', true>[number] | undefined {
  if (!transfer.vault || !isAddress(transfer.recipient)) return undefined;

  const recipient = paxeerRecipient(transfer.recipient).toLowerCase();
  const amount = BigInt(transfer.amount);
  const vault = transfer.vault.toLowerCase();
  const events = parseEventLogs({
    abi: paxeerXVaultAbi,
    eventName: 'BridgeDeposit',
    logs: logs.filter((log) => !log.removed && log.transactionHash?.toLowerCase() === transfer.sourceTxHash.toLowerCase() &&
      log.address.toLowerCase() === vault),
  });

  const matches = events.filter((event) =>
    event.args.sender.toLowerCase() === transfer.account.toLowerCase() &&
    event.args.amount === amount &&
    event.args.asset.toLowerCase() === transfer.assetId.toLowerCase() &&
    event.args.paxeerRecipient.toLowerCase() === recipient &&
    (transfer.logIndex === undefined || String(event.logIndex) === transfer.logIndex) &&
    (transfer.nonce === undefined || String(event.args.nonce) === transfer.nonce),
  );
  return matches.length === 1 ? matches[0] : undefined;
}

function bridgeOutEvents(logs: ReadonlyArray<Log>) {
  const events = parseEventLogs<typeof layerxBridgeAbi, true, string>({
    abi: layerxBridgeAbi,
    eventName: 'BridgeOut',
    logs: logs.filter((log) => !log.removed && log.address.toLowerCase() === LAYERX_BRIDGE_ADDRESS),
  });
  return events.flatMap((event) => {
    const args = event.args;
    if (!args ||
      !('chain' in args) || typeof args.chain !== 'bigint' ||
      !('nonce' in args) || typeof args.nonce !== 'bigint' ||
      !('amount' in args) || typeof args.amount !== 'bigint' ||
      !('asset' in args) || typeof args.asset !== 'string' || !isAddress(args.asset) ||
      !('recipient' in args) || typeof args.recipient !== 'string' || !isAddress(args.recipient)) return [];
    return [ { chain: args.chain, nonce: args.nonce, amount: args.amount,
      asset: args.asset as Address, recipient: args.recipient as Address } ];
  });
}

export function nonceFromBridgeOutLogs(logs: ReadonlyArray<Log>, nonce?: bigint): bigint | undefined {
  const events = bridgeOutEvents(logs).filter((event) => nonce === undefined || event.nonce === nonce);
  return events.length === 1 ? events[0].nonce : undefined;
}

export function importDeposit(chain: BridgeChain, receipt: TransactionReceipt, logIndex?: number): BridgeTransfer {
  if (!chain.vault || chain.kind !== 'evm') throw new Error(`No EVM vault is configured for ${ chain.label }.`);
  if (receipt.status !== 'success') throw new Error('The source transaction reverted.');

  const events = parseEventLogs({
    abi: paxeerXVaultAbi,
    eventName: 'BridgeDeposit',
    logs: receipt.logs.filter((log) => !log.removed && log.address.toLowerCase() === chain.vault?.toLowerCase() &&
      log.transactionHash?.toLowerCase() === receipt.transactionHash.toLowerCase()),
  }).filter((event) => logIndex === undefined || event.logIndex === logIndex);
  if (events.length > 1) throw new Error('This receipt contains multiple deposits. Select a BridgeDeposit log index.');
  const event = events[0];
  if (!event || event.logIndex === null) throw new Error(`No BridgeDeposit log from vault ${ chain.vault } was found.`);
  const { asset, amount, sender, paxeerRecipient: recipient32 } = event.args;
  const recipient = getAddress(`0x${ recipient32.slice(-40) }`);
  if (paxeerRecipient(recipient).toLowerCase() !== recipient32.toLowerCase()) {
    throw new Error('The deposit recipient is not a canonical Paxeer address.');
  }
  const registeredAsset = chain.assets.find((entry) => entry.address.toLowerCase() === asset.toLowerCase());
  if (!registeredAsset) throw new Error(`Asset ${ asset } is not configured for ${ chain.label }.`);

  const transfer: BridgeTransfer = {
    id: transferIdOf('in', receipt.transactionHash, String(chain.chainId), String(event.logIndex)),
    direction: 'in',
    foreignChainKey: chain.key,
    foreignChainId: String(chain.chainId),
    symbol: registeredAsset.symbol,
    decimals: registeredAsset.decimals,
    amount: String(amount),
    assetId: registeredAsset.assetId,
    vault: chain.vault,
    account: sender,
    recipient,
    sourceTxHash: receipt.transactionHash,
    requiredConfirmations: chain.finalityDepth,
    submittedAt: Date.now(),
    logIndex: String(event.logIndex),
    nonce: String(event.args.nonce),
  };
  const matched = matchDepositLog(transfer, receipt.logs);
  if (!matched) throw new Error(`Could not match the deposit from vault ${ chain.vault }.`);
  return transfer;
}

function withdrawalFromReceipt(receipt: TransactionReceipt, nonce?: bigint): BridgeTransfer {
  if (receipt.status !== 'success') throw new Error('The source transaction reverted.');
  const events = bridgeOutEvents(receipt.logs.filter((log) =>
    log.transactionHash?.toLowerCase() === receipt.transactionHash.toLowerCase()))
    .filter((event) => nonce === undefined || event.nonce === nonce);
  if (events.length > 1) throw new Error('This receipt contains multiple withdrawals. Select a BridgeOut nonce.');
  const args = events[0];
  if (!args) throw new Error(`No valid BridgeOut log from ${ LAYERX_BRIDGE_ADDRESS } was found.`);
  const chain = bridgeChainById(args.chain);
  if (!chain) throw new Error(`Bridge chain ${ args.chain } is not configured.`);
  const assetId = args.asset;
  const asset = chain.assets.find((entry) => entry.assetId.toLowerCase() === assetId.toLowerCase());
  if (!asset) throw new Error(`Asset ${ args.asset } is not configured for ${ chain.label }.`);

  return {
    id: transferIdOf('out', receipt.transactionHash, String(args.chain), String(args.nonce)),
    direction: 'out',
    foreignChainKey: chain.key,
    foreignChainId: String(args.chain),
    symbol: asset.symbol,
    decimals: asset.decimals,
    amount: String(args.amount),
    assetId,
    vault: chain.vault,
    account: receipt.from,
    recipient: args.recipient,
    sourceTxHash: receipt.transactionHash,
    requiredConfirmations: 1,
    submittedAt: Date.now(),
    bridgeOutNonce: String(args.nonce),
  };
}

export function importWithdrawal(receipt: TransactionReceipt, nonce?: bigint): BridgeTransfer {
  const transfer = withdrawalFromReceipt(receipt, nonce);
  if (transfer.foreignChainKey === 'solana') {
    throw new Error('Solana withdrawals require a custody recipient lookup. Use importWithdrawalResolved.');
  }
  return transfer;
}

export async function importWithdrawalResolved(
  receipt: TransactionReceipt,
  { programId, rpcUrl }: SolanaCustodyConfig = {
    programId: bridgeConfig.solanaProgramId,
    rpcUrl: bridgeConfig.foreignRpcUrls.solana,
  },
  nonce?: bigint,
): Promise<BridgeTransfer> {
  const transfer = withdrawalFromReceipt(receipt, nonce);
  if (transfer.foreignChainKey !== 'solana') return transfer;
  const handle = getAddress(transfer.recipient);
  const pda = programId && recipientRecordAddress(programId, handle);
  if (!programId || !rpcUrl || !pda) throw new Error('The Solana custody recipient lookup is not configured.');
  const response = await fetch(rpcUrl, {
    method: 'POST', headers: { 'Content-Type': 'application/json' }, cache: 'no-store',
    signal: AbortSignal.timeout(10_000),
    body: JSON.stringify({ jsonrpc: '2.0', id: 1, method: 'getAccountInfo',
      params: [ pda.address, { encoding: 'base64', commitment: 'finalized' } ] }),
  });
  if (!response.ok) throw new Error(`The Solana RPC answered ${ response.status }.`);
  const message: unknown = await response.json();
  const record = recipientRecordFromResponse(message, handle);
  if (!record.registered) throw new Error('The Solana recipient record is not registered.');
  // The decoder checks the RPC envelope and layout; custody ownership and the key-derived handle
  // must also be verified before a recovered destination can enter the ledger.
  const account = (message as { result: { value: { owner?: unknown; executable?: unknown } } }).result.value;
  if (account.owner !== programId || account.executable !== false) throw new Error('The Solana recipient record has the wrong owner or is executable.');
  const recipient = base58Encode(record.key);
  if (solanaRecipientHandle(recipient)?.toLowerCase() !== handle.toLowerCase()) {
    throw new Error('The Solana recipient key does not derive the burned handle.');
  }
  return { ...transfer, recipient, recipientHandle: handle };
}
