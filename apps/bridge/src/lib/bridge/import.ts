import { isAddress, parseEventLogs } from 'viem';
import type { Log } from 'viem';

import { LAYERX_BRIDGE_ADDRESS, layerxBridgeAbi, paxeerXVaultAbi } from './contracts';
import { paxeerRecipient } from './digests';
import type { BridgeTransfer } from './transfers';

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
