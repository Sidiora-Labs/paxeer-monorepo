import type { SystemStyleObject } from '@chakra-ui/react';
import { Box, Flex, chakra } from '@chakra-ui/react';
import React from 'react';

import type { ScanTableCardProps } from 'ui/shared/scan/ScanTableCard';

export const LIST_TABLE_STYLES: SystemStyleObject = {
  '& thead, & thead tr': { height: '36px', bg: 'canvas' },
  '& th': { height: '36px', py: 0, textStyle: 'label', bg: 'canvas', borderColor: 'hairline' },
  '& tbody tr': { height: '44px', bg: 'canvas', _hover: { bg: 'raised' } },
  '& td': { height: '44px', py: '2px', textStyle: 'small', borderColor: 'hairline', verticalAlign: 'middle' },
  '& td[data-is-numeric]': { textStyle: 'number', textAlign: 'right' },
  '& td[data-is-numeric] *': { fontFamily: 'mono', fontVariantNumeric: 'tabular-nums', lineHeight: '18px' },
  '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'inherit', zIndex: 1 },
  '& tbody tr:last-of-type td': { borderBottomWidth: '1px' },
  '& [data-entity-kind=address], & [data-entity-kind=tx], & [data-entity-kind=block]': { fontFamily: 'mono', fontVariantNumeric: 'tabular-nums' },
  '& button[aria-label=copy]': { opacity: { base: 1, lg: 0 } },
  '& tr:hover button[aria-label=copy], & tr:focus-within button[aria-label=copy]': { opacity: 1 },
};

const ListPage = ({ title, note, actions, pagination, showRows, children, className }: ScanTableCardProps) => (
  <Box className={ className } data-list-page bg="canvas" color="text.primary">
    <Flex data-header alignItems="center" justifyContent="space-between" flexWrap="wrap" gap={ 3 } pb={ 4 }>
      <Box minW={ 0 }>
        <chakra.p textStyle="small" data-title>{ title }</chakra.p>
        { note && <chakra.p textStyle="small" color="text.muted" data-note>{ note }</chakra.p> }
      </Box>
      <Flex data-actions alignItems="center" flexWrap="wrap" gap={ 2 }>
        { actions }
        { pagination && <Box hideBelow="md" data-header-pagination>{ pagination }</Box> }
      </Flex>
    </Flex>
    <Box data-body overflowX="auto" maxW="100%">{ children }</Box>
    { (showRows || pagination) && (
      <Flex data-footer alignItems="center" justifyContent="space-between" flexWrap="wrap" gap={ 3 } py={ 4 }>
        <Box data-footer-rows>{ showRows }</Box>
        <Box data-footer-pagination>{ pagination }</Box>
      </Flex>
    ) }
  </Box>
);

export default ListPage;
