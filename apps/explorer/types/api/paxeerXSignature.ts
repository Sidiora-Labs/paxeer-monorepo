import type { PaxeerXStatusRung } from './paxeerXLists';

export type PaxeerXTxSignature = {
  rung: PaxeerXStatusRung;
  receipt: string;
  receipt_key: {
    key_id: string;
    public_key: string;
  };
  signature: string;
  committed_height?: number | null;
};
