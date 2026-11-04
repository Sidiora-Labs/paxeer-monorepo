import type { InterchainTransfer } from '@blockscout/interchain-indexer-types';

export const getItemKey = (data: InterchainTransfer, occurrence = 0) => {
  return JSON.stringify([
    data.bridge?.name,
    data.message_id,
    data.sender?.hash,
    data.source_token?.address_hash,
    data.source_amount,
    data.source_chain?.id,
    data.source_transaction_hash,
    data.send_timestamp,
    data.recipient?.hash,
    data.destination_token?.address_hash,
    data.destination_amount,
    data.destination_chain?.id,
    data.destination_transaction_hash,
    occurrence,
  ]);
};

export const getItemsWithKeys = (items: Array<InterchainTransfer>) => {
  const occurrences = new Map<string, number>();

  return items.map((item) => {
    const identity = getItemKey(item);
    const occurrence = occurrences.get(identity) ?? 0;
    occurrences.set(identity, occurrence + 1);

    return { item, key: getItemKey(item, occurrence) };
  });
};
