import { Box } from '@chakra-ui/react';
import React, { type PropsWithChildren } from 'react';

interface Props extends PropsWithChildren {

  /** A short accessible name that describes the records in the table. */
  label: string;
  minWidth: string;
}

/**
 * Keeps wide data tables usable in narrow containers. The region is keyboard
 * focusable so a keyboard user can scroll the same overflow area as a touch
 * user, while the table keeps its native header and cell semantics.
 */
export function ResponsiveTableRegion({ children, label, minWidth }: Props) {
  return (
    <Box
      role="region"
      aria-label={ label }
      tabIndex={ 0 }
      maxW="100%"
      maxH="calc(100dvh - var(--chakra-spacing-40))"
      overflow="auto"
      containerType="inline-size"
      css={{ '& thead': { position: 'sticky', top: 0, zIndex: 'sticky' } }}
      _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-border-widths-2)' }}
    >
      <Box minW={ minWidth }>{ children }</Box>
    </Box>
  );
}
