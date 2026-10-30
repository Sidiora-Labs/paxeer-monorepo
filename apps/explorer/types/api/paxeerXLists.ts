export const PAXEER_X_NATIVE_RUNGS = [ 'submitted', 'receipted', 'committed' ] as const;

export type PaxeerXNativeRung = typeof PAXEER_X_NATIVE_RUNGS[number];

export const PAXEER_X_STATUS_RUNGS = [ 'submitted', 'receipted', 'committed' ] as const;

export type PaxeerXStatusRung = typeof PAXEER_X_STATUS_RUNGS[number];

export type PaxeerXTxStatus = {
  rung: PaxeerXStatusRung;
  block_number: number | null;
  committed_height: number | null;
};
