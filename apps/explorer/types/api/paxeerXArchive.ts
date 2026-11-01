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

export type PaxeerXArchiveReceiptsItem = {
  id: string;
  account: string | null;
  frozen: true;
  block_number: number;
};

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

// One receipt carries, on top of the list item, the rung it has reached on the kernel's own
// verification lattice and the log that recorded it. `payload_hash` is an optional column of the
// receipt log, while the emitting transaction and the timestamp of its block are not.
export type PaxeerXArchiveReceipt = PaxeerXArchiveReceiptsItem & {
  verification_status: PaxeerXArchiveVerificationStatus;
  payload_hash: string | null;
  transaction_hash: string;
  timestamp: string;
};
