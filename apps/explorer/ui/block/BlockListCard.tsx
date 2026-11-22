import { chakra } from '@chakra-ui/react';
import React from 'react';

import type { ScanTableCardProps } from 'ui/shared/scan';
import { ScanTableCard } from 'ui/shared/scan';

const StyledCard = chakra(ScanTableCard);

const BlockListCard = (props: ScanTableCardProps) => (
  <StyledCard { ...props } css={{
    overflowWrap: 'anywhere',
    '& a': { display: 'inline-flex', alignItems: 'center' },
    '& a, & button': {
      _motionReduce: { transitionProperty: 'none' },
    },
    '& button[role="combobox"]': { focusVisibleRing: 'outside', focusRingColor: 'focusRing' },
    '& [data-control]:not(:disabled):not([aria-disabled="true"])': {
      color: 'text.primary',
      borderColor: 'border.strong',
      '& svg': { color: 'text.primary' },
    },
    '& [data-block-mobile-grid] > div > div:not(:last-child)': {
      gridTemplateColumns: { base: 'minmax(0, 1fr)', md: 'var(--chakra-spacing-30) minmax(0, 1fr)' },
      px: 4,
      '& > div': { minWidth: 0 },
    },
    '& [data-block-mobile-grid] .chakra-skeleton:has(> button[aria-label="copy"])': {
      gridTemplateColumns: 'minmax(0, 1fr) auto',
      gap: 1,
      overflow: 'visible',
      minWidth: 0,
      '& > :first-child, & p': { minWidth: 0, whiteSpace: 'normal', overflowWrap: 'anywhere', overflow: 'visible' },
    },
    '& [data-entity]': { maxWidth: 'full', flexWrap: 'wrap' },
    '& [data-entity-link]': { maxWidth: 'full' },
    '& .chakra-badge > span': { color: 'text.primary' },
  }}/>
);

export default React.memo(BlockListCard);
