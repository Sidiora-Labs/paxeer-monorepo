import { Box, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import useApiQuery from 'lib/api/useApiQuery';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import BlockEntity from 'ui/shared/entities/block/BlockEntity';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';

const RecentBlocks = () => {
  const headingId = React.useId();
  const { data, isPending, isError } = useApiQuery('general:homepage_blocks');

  return (
    <Box mb={ 8 } data-network-recent-blocks
      css={{ '& a': { minWidth: 'touchTarget',
        minHeight: 'touchTarget', _motionReduce: { transitionProperty: 'none' } } }}>
      <Box as="section" aria-labelledby={ headingId } minW={ 0 } data-section-group="Recent blocks">
        <Flex alignItems="center" justifyContent="space-between" flexWrap="nowrap" gap={ 2 } mb={ 2 }>
          <Text as="h2" id={ headingId } textStyle="label" color="text.muted" textTransform="uppercase" whiteSpace="nowrap">Recent blocks</Text>
          <Link href="/blocks" textStyle="small" flexShrink={ 0 } whiteSpace="nowrap">View all</Link>
        </Flex>
        { isError ? <DataFetchAlert/> : (
          <Skeleton loading={ isPending } minH={ isPending ? 'var(--chakra-spacing-11)' : undefined }>
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
                minW={ 0 }>
                <TableHeader><TableRow>
                  <TableColumnHeader>Block</TableColumnHeader>
                  <TableColumnHeader>Age</TableColumnHeader>
                  <TableColumnHeader isNumeric>Txn</TableColumnHeader>
                  <TableColumnHeader isNumeric>Gas used</TableColumnHeader>
                </TableRow></TableHeader>
                <TableBody>
                  { data?.slice(0, 5).map((block) => (
                    <TableRow key={ block.hash }>
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
      </Box>
    </Box>
  );
};

export default RecentBlocks;
