import type { PaxeerXNativeRung, PaxeerXStatusRung } from 'types/api/paxeerXLists';
import { PAXEER_X_NATIVE_RUNGS, PAXEER_X_STATUS_RUNGS } from 'types/api/paxeerXLists';

import type { BadgeProps } from 'toolkit/chakra/badge';
import type { IconName } from 'ui/shared/IconSvg';

export interface RungDescriptor {
  rung: PaxeerXStatusRung;
  label: string;
  description: string;
  icon: IconName;
  colorPalette: BadgeProps['colorPalette'];
}

export const RUNGS: Record<PaxeerXStatusRung, RungDescriptor> = {
  submitted: {
    rung: 'submitted',
    label: 'Submitted',
    description: 'The transaction has been submitted to the node and is awaiting a receipt and a committed block.',
    icon: 'status/pending',
    colorPalette: 'gray',
  },
  receipted: {
    rung: 'receipted',
    label: 'Receipted',
    description: 'A transaction receipt is available, and finality requires its block to be committed.',
    icon: 'clock',
    colorPalette: 'blue',
  },
  committed: {
    rung: 'committed',
    label: 'Committed',
    description: 'The transaction receipt is final because its block is committed.',
    icon: 'status/success',
    colorPalette: 'green',
  },
};

export const RUNG_ORDER: Array<RungDescriptor> = PAXEER_X_STATUS_RUNGS.map((rung) => RUNGS[rung]);

export interface NativeRungDescriptor extends Omit<RungDescriptor, 'rung'> {
  rung: PaxeerXNativeRung;
}

export const NATIVE_RUNGS: Record<PaxeerXNativeRung, NativeRungDescriptor> = {
  submitted: {
    rung: 'submitted',
    label: 'Submitted',
    description: 'The transaction has been submitted to the node and is awaiting a receipt and a committed block.',
    icon: 'status/pending',
    colorPalette: 'gray',
  },
  receipted: {
    rung: 'receipted',
    label: 'Receipted',
    description: 'A transaction receipt is available, and finality requires its block to be committed.',
    icon: 'clock',
    colorPalette: 'blue',
  },
  committed: {
    rung: 'committed',
    label: 'Committed',
    description: 'The transaction receipt is final because its block is committed.',
    icon: 'status/success',
    colorPalette: 'green',
  },
};

export const NATIVE_RUNG_ORDER: Array<NativeRungDescriptor> = PAXEER_X_NATIVE_RUNGS.map((rung) => NATIVE_RUNGS[rung]);
