import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import useApiQuery from 'lib/api/useApiQuery';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableBody, TableCell, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import BlockEntity from 'ui/shared/entities/block/BlockEntity';
import TimeWithTooltip from 'ui/shared/time/TimeWithTooltip';

const RecentBlocks = () => {
  const { data, isPending, isError } = useApiQuery('general:homepage_blocks');

  return (
    <Box mb={ 8 } data-network-recent-blocks>
      <DetailedInfo.Section label="Recent blocks">
        <Flex justifyContent="flex-end" mb={ 2 }><Link href="/blocks" textStyle="small">View all</Link></Flex>
        { isError ? <DataFetchAlert/> : (
          <Skeleton loading={ isPending } minH={ isPending ? '44px' : undefined }>
            <Box overflowX="auto">
              <TableRoot variant="scan" css={{ '& th:first-of-type, & td:first-of-type': { position: 'sticky', left: 0, bg: 'canvas' } }} minW="480px">
                <TableHeader><TableRow>
                  <TableColumnHeader>Block</TableColumnHeader>
                  <TableColumnHeader>Age</TableColumnHeader>
                  <TableColumnHeader isNumeric>Transactions</TableColumnHeader>
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
      </DetailedInfo.Section>
    </Box>
  );
};

export default RecentBlocks;
