import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import useApiQuery from 'lib/api/useApiQuery';
import { BLOCK } from 'stubs/block';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import BlockEntity from 'ui/shared/entities/block/BlockEntity';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';

const RecentBlocks = () => {
  const { data, isPlaceholderData, isError } = useApiQuery('general:homepage_blocks', {
    queryOptions: { placeholderData: Array.from({ length: 5 }, () => BLOCK) },
  });

  return (
    <Box mb={ 8 } data-network-recent-blocks
      css={{ '& a': { _motionReduce: { transitionProperty: 'none' } } }}>
      <DetailedInfo.Section label="Recent blocks">
        <Flex justifyContent="flex-end" mb={ 2 }>
          <Link href="/blocks" textStyle="small" flexShrink={ 0 } whiteSpace="nowrap" data-touch-inline
            css={{ '--touch-line-size': 'var(--chakra-line-heights-small)' }}>View all</Link>
        </Flex>
        { isError ? <DataFetchAlert/> : (
          <Skeleton loading={ isPlaceholderData }>
            <Box overflowX="auto" role="region" aria-label="Recent blocks" tabIndex={ 0 }>
              <TableRoot variant="scan"
                css={{
                  '& th': { whiteSpace: 'nowrap', wordBreak: 'normal', overflowWrap: 'normal' },
                  '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'canvas' },
                  '& tbody td': { py: 0 },
                  '& tbody td a': { marginBlock: 'calc(var(--chakra-border-widths-1) / -2)' },
                }}
                tableLayout="auto"
                w="100%"
                minW="calc(var(--chakra-spacing-100) + var(--chakra-spacing-20))">
                <TableHeader><TableRow>
                  <TableColumnHeader>Block</TableColumnHeader>
                  <TableColumnHeader>Age</TableColumnHeader>
                  <TableColumnHeader isNumeric wordBreak="normal" overflowWrap="normal">Txn</TableColumnHeader>
                  <TableColumnHeader isNumeric>Gas used</TableColumnHeader>
                </TableRow></TableHeader>
                <TableBody>
                  { data?.slice(0, 5).map((block, index) => (
                    <TableRow key={ isPlaceholderData ? index : block.hash }>
                      <TableCell><BlockEntity number={ block.height } noIcon textStyle="number"/></TableCell>
                      <TableCell><TimeWithTooltip timestamp={ block.timestamp } color="text.secondary"/></TableCell>
                      <TableCell isNumeric>{ block.transactions_count.toLocaleString() }</TableCell>
                      <TableCell isNumeric>{ block.gas_used === null ? '—' : Number(block.gas_used).toLocaleString() }</TableCell>
                    </TableRow>
                  )) }
                </TableBody>
              </TableRoot>
            </Box>
            { data?.length === 0 && <Box py={ 3 } color="text.muted">There are no recent blocks.</Box> }
          </Skeleton>
        ) }
      </DetailedInfo.Section>
    </Box>
  );
};

export default RecentBlocks;
