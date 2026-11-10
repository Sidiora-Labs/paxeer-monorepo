import type { PaxeerXArchiveEvmReceipt, PaxeerXArchiveKernelReceipt } from 'types/api/paxeerXArchive';
import type { PaxeerXTxStatus } from 'types/api/paxeerXLists';

import { TX_HASH } from './tx';

export const PAXEER_X_RECEIPTS_ITEM: PaxeerXArchiveEvmReceipt = {
  id: TX_HASH,
  account: '0x0000000000000000000000000000000000000000000000000000000000000000',
  frozen: true,
  block_number: 8341004,
  origin: 'evm',
  payload_hash: TX_HASH,
  transaction_hash: TX_HASH,
  timestamp: '2023-05-22T18:00:36.000000Z',
  verification_status: 'checkpoint_finalised',
  verification: 'checkpoint_finalised',
  provenance: {
    evm: {
      transaction_hash: TX_HASH,
      log_index: 0,
      block_hash: TX_HASH,
      block_number: 8341004,
    },
  },
};

export const PAXEER_X_KERNEL_RECEIPTS_ITEM: PaxeerXArchiveKernelReceipt = {
  id: '0x0000000000000000000000000000000000000000000000000000000000000033',
  account: '0x0000000000000000000000000000000000000000000000000000000000000063',
  frozen: true,
  origin: 'kernel',
  block_number: null,
  transaction_hash: null,
  timestamp: null,
  payload_hash: '0x0000000000000000000000000000000000000000000000000000000000000064',
  verification_status: 'batch_included',
  verification: 'sequencer_verified',
  provenance: {
    kernel: {
      batch_number: '51',
      batch_id: '0x0000000000000000000000000000000000000000000000000000000000000065',
      sequence: '1',
      activity_id: '0x0000000000000000000000000000000000000000000000000000000000000033',
      result_code: 0,
      receipt_sha256: '0x0000000000000000000000000000000000000000000000000000000000000066',
      canonical_sha256: '0x0000000000000000000000000000000000000000000000000000000000000067',
      batch_raw_sha256: '0x0000000000000000000000000000000000000000000000000000000000000068',
      state_root: '0x0000000000000000000000000000000000000000000000000000000000000069',
      proof: { header_hex: '0a0b', signature_hex: 'cd'.repeat(64) },
    },
  },
};

export const PAXEER_X_TX_STATUS: PaxeerXTxStatus = {
  rung: 'receipted',
  block_number: 8341004,
  committed_height: null,
};
