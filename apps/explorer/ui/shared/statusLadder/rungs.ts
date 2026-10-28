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
  pending: {
    rung: 'pending',
    label: 'Pending',
    description: 'The transaction has not been included in a block yet.',
    icon: 'status/pending',
    colorPalette: 'gray',
  },
  instant: {
    rung: 'instant',
    label: 'Instant',
    description: 'The transaction is in a block that no anchor checkpoint covers yet.',
    icon: 'clock',
    colorPalette: 'blue',
  },
  sealed: {
    rung: 'sealed',
    label: 'Sealed',
    description: 'An anchor checkpoint covering the block has been submitted but is not finalized.',
    icon: 'lock',
    colorPalette: 'purple',
  },
  'final': {
    rung: 'final',
    label: 'Final',
    description: 'A finalized anchor checkpoint covers the block, which is past the finality height.',
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
    description: 'The transaction has been submitted and is awaiting a verified sequencer receipt or a committed block.',
    icon: 'status/pending',
    colorPalette: 'gray',
  },
  receipted: {
    rung: 'receipted',
    label: 'Receipted',
    description: 'The sequencer receipt verifies against the pinned receipt keys.',
    icon: 'clock',
    colorPalette: 'blue',
  },
  committed: {
    rung: 'committed',
    label: 'Committed',
    description: 'The committed head covers the transaction block or its sequencer receipt verifies commitment.',
    icon: 'status/success',
    colorPalette: 'green',
  },
};

export const NATIVE_RUNG_ORDER: Array<NativeRungDescriptor> = PAXEER_X_NATIVE_RUNGS.map((rung) => NATIVE_RUNGS[rung]);
