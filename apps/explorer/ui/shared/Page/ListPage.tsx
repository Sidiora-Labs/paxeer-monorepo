import type { SystemStyleObject } from '@chakra-ui/react';
import { Box, Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import type { ScanTableCardProps } from 'ui/shared/scan/ScanTableCard';

export const LIST_TABLE_STYLES: SystemStyleObject = {
  '& thead, & thead tr': { height: 'var(--chakra-spacing-9)', bg: 'canvas' },
  '& th': { height: 'var(--chakra-spacing-9)', py: 0, textStyle: 'label', bg: 'canvas', borderColor: 'hairline' },
  '& tbody tr': { height: 'var(--chakra-spacing-11)', bg: 'canvas', _hover: { bg: 'raised' } },
  '& td': { height: 'var(--chakra-spacing-11)', py: 0.5, textStyle: 'small', borderColor: 'hairline', verticalAlign: 'middle' },
  '& td[data-is-numeric]': { textStyle: 'number', textAlign: 'right' },
  '& td[data-is-numeric] *': { textStyle: 'number' },
  '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'inherit', zIndex: 1 },
  '& tbody tr:last-of-type td': { borderBottomWidth: '1px' },
  '& [data-entity-kind=address], & [data-entity-kind=tx], & [data-entity-kind=block]': { fontFamily: 'mono', fontVariantNumeric: 'tabular-nums' },
  '& button[aria-label=copy]': { opacity: { base: 1, lg: 0 } },
  '& tr:hover button[aria-label=copy], & tr:focus-within button[aria-label=copy]': { opacity: 1 },
};

interface Props extends ScanTableCardProps {
  keepActionsInline?: boolean;
  isLoading?: boolean;
}

const ListPage = ({ title, note, actions, pagination, showRows, children, className, keepActionsInline, isLoading }: Props) => (
  <Box className={ className } data-list-page bg="canvas" color="text.primary" aria-busy={ isLoading }>
    <Flex data-header alignItems="center" justifyContent="space-between" flexWrap={ keepActionsInline ? 'nowrap' : 'wrap' } gap={ 3 } pb={ 4 }>
      <Box minW={ 0 }>
        <chakra.p textStyle="small" data-title>{ title }</chakra.p>
        { note && <chakra.p textStyle="small" color="text.muted" data-note>{ note }</chakra.p> }
      </Box>
      <Flex data-actions alignItems="center" flexWrap="wrap" gap={ 2 } flexShrink={ keepActionsInline ? 0 : undefined }>
        { actions }
        { pagination && <Box hideBelow="md" data-header-pagination>{ pagination }</Box> }
      </Flex>
    </Flex>
    <Box
      data-body
      overflowX="auto"
      maxW="100%"
      _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'calc(-1 * var(--chakra-border-widths-2))' }}
    >
      { children }
    </Box>
    { (showRows || pagination) && (
      <Flex data-footer alignItems="center" justifyContent="space-between" flexWrap="wrap" gap={ 3 } py={ 4 } visibility={ isLoading ? 'hidden' : undefined }>
        <Box data-footer-rows>{ showRows }</Box>
        <Box data-footer-pagination>{ pagination }</Box>
      </Flex>
    ) }
  </Box>
);

export default ListPage;
