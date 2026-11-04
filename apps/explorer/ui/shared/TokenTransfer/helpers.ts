import type { TokenTransfer } from 'types/api/tokenTransfer';

export const getTokenTransferKey = (transfer: TokenTransfer, loadingIndex?: number) => {
  const tokenId = transfer.total && 'token_id' in transfer.total ? transfer.total.token_id : null;

  return JSON.stringify([
    transfer.transaction_hash,
    transfer.block_hash,
    transfer.log_index,
    tokenId,
    loadingIndex,
  ]);
};

export const getTokenTransferTypeText = (type: TokenTransfer['type']) => {
  switch (type) {
    case 'token_minting':
      return 'Token minting';
    case 'token_burning':
      return 'Token burning';
    case 'token_spawning':
      return 'Token creating';
    case 'token_transfer':
      return 'Token transfer';
  }
};
