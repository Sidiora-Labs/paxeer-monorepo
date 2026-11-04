// `checkpoint_height` is the kernel height the checkpoint commits to and `sealed_height` the
// height it seals; both, like the state root, are optional columns of the anchor log.
export type PaxeerXArchiveAnchor = {
  frozen: true;
  batch_number: number;
  checkpoint_id: string;
  checkpoint_height: number | null;
  sealed_height: number | null;
  state_root: string | null;
  block_number: number;
  timestamp: string;
};

export type PaxeerXArchiveAnchorsResponse = {
  items: Array<PaxeerXArchiveAnchor>;
  next_page_params: {
    batch_number: number;
    items_count: number;
  } | null;
};

type PaxeerXArchiveReceiptCommon = {
  id: string;
  account: string | null;
  frozen: true;
  payload_hash: string | null;
};

export type PaxeerXArchiveEvmReceipt = PaxeerXArchiveReceiptCommon & {
  origin: 'evm';
  block_number: number;
  transaction_hash: string;
  timestamp: string | null;
  verification_status: PaxeerXArchiveVerificationStatus;
  verification: PaxeerXArchiveVerificationStatus;
  provenance: {
    evm: {
      transaction_hash: string;
      log_index: number;
      block_hash: string;
      block_number: number;
    };
  };
};

export type PaxeerXArchiveKernelReceipt = PaxeerXArchiveReceiptCommon & {
  origin: 'kernel';
  block_number: null;
  transaction_hash: null;
  timestamp: null;
  verification_status: 'batch_included';
  verification: 'sequencer_verified';
  provenance: {
    kernel: {
      batch_number: string;
      batch_id: string;
      sequence: string;
      activity_id: string;
      result_code: number;
      receipt_sha256: string;
      canonical_sha256: string;
      batch_raw_sha256: string;
      state_root: string;
      proof: {
        header_hex: string;
        signature_hex: string;
      };
    };
  };
};

export type PaxeerXArchiveReceiptsItem = PaxeerXArchiveEvmReceipt | PaxeerXArchiveKernelReceipt;

export type PaxeerXArchiveReceiptsResponse = {
  items: Array<PaxeerXArchiveReceiptsItem>;
  next_page_params: {
    id: string;
    items_count: number;
  } | null;
};

export const PAXEER_X_ARCHIVE_VERIFICATION_STATUSES = [
  'unverified',
  'sequencer_signed',
  'batch_included',
  'state_proven',
  'checkpoint_finalised',
  'settlement_anchored',
] as const;

export type PaxeerXArchiveVerificationStatus = typeof PAXEER_X_ARCHIVE_VERIFICATION_STATUSES[number];

export type PaxeerXArchiveReceipt = PaxeerXArchiveReceiptsItem;
