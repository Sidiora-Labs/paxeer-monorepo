import { getAddress, isAddress, parseEventLogs } from 'viem';
import type { Log, TransactionReceipt } from 'viem';

import { bridgeChainById, type BridgeChain } from './chains';
import { LAYERX_BRIDGE_ADDRESS, layerxBridgeAbi, paxeerXVaultAbi } from './contracts';
import { paxeerRecipient } from './digests';
import { transferIdOf, type BridgeTransfer } from './transfers';

export function matchDepositLog(transfer: BridgeTransfer, logs: ReadonlyArray<Log>) {
  if (!transfer.vault || !isAddress(transfer.recipient)) return undefined;

  const recipient = paxeerRecipient(transfer.recipient).toLowerCase();
  const amount = BigInt(transfer.amount);
  const vault = transfer.vault.toLowerCase();
  const events = parseEventLogs({
    abi: paxeerXVaultAbi,
    eventName: 'BridgeDeposit',
    logs: logs.filter((log) => log.address.toLowerCase() === vault),
  });

  return events.find((event) =>
    event.args.sender.toLowerCase() === transfer.account.toLowerCase() &&
    event.args.amount === amount &&
    event.args.asset.toLowerCase() === transfer.assetId.toLowerCase() &&
    event.args.paxeerRecipient.toLowerCase() === recipient,
  );
}

export function nonceFromBridgeOutLogs(logs: ReadonlyArray<Log>): bigint | undefined {
  const events = parseEventLogs({
    abi: layerxBridgeAbi,
    logs: logs.filter((log) => log.address.toLowerCase() === LAYERX_BRIDGE_ADDRESS),
  });
  const args = events.find((event) => event.eventName === 'BridgeOut')?.args;
  return args && 'nonce' in args && typeof args.nonce === 'bigint' ? args.nonce : undefined;
}

export function importDeposit(chain: BridgeChain, receipt: TransactionReceipt): BridgeTransfer {
  if (!chain.vault || chain.kind !== 'evm') throw new Error(`No EVM vault is configured for ${ chain.label }.`);
  if (receipt.status !== 'success') throw new Error('The source transaction reverted.');

  const events = parseEventLogs({
    abi: paxeerXVaultAbi,
    eventName: 'BridgeDeposit',
    logs: receipt.logs.filter((log) => log.address.toLowerCase() === chain.vault?.toLowerCase()),
  });
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
    id: transferIdOf('in', receipt.transactionHash),
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
  };
  const matched = matchDepositLog(transfer, receipt.logs);
  if (!matched || matched.logIndex === null) throw new Error(`Could not match the deposit from vault ${ chain.vault }.`);
  return { ...transfer, logIndex: String(matched.logIndex), nonce: String(matched.args.nonce) };
}

export function importWithdrawal(receipt: TransactionReceipt): BridgeTransfer {
  if (receipt.status !== 'success') throw new Error('The source transaction reverted.');
  const nonce = nonceFromBridgeOutLogs(receipt.logs);
  const events = parseEventLogs({
    abi: layerxBridgeAbi,
    eventName: 'BridgeOut',
    logs: receipt.logs.filter((log) => log.address.toLowerCase() === LAYERX_BRIDGE_ADDRESS),
  });
  const event = events.find((entry) => entry.args && 'nonce' in entry.args && entry.args.nonce === nonce);
  const args = event?.args;
  if (nonce === undefined || !args ||
    !('chain' in args) || typeof args.chain !== 'bigint' ||
    !('asset' in args) || typeof args.asset !== 'string' || !isAddress(args.asset) ||
    !('amount' in args) || typeof args.amount !== 'bigint' ||
    !('recipient' in args) || typeof args.recipient !== 'string' || !isAddress(args.recipient)) {
    throw new Error(`No valid BridgeOut log from ${ LAYERX_BRIDGE_ADDRESS } was found.`);
  }
  const chain = bridgeChainById(args.chain);
  if (!chain) throw new Error(`Bridge chain ${ args.chain } is not configured.`);
  const assetId = args.asset;
  const asset = chain.assets.find((entry) => entry.assetId.toLowerCase() === assetId.toLowerCase());
  if (!asset) throw new Error(`Asset ${ args.asset } is not configured for ${ chain.label }.`);

  return {
    id: transferIdOf('out', receipt.transactionHash),
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
    bridgeOutNonce: String(nonce),
  };
}
