import { Box, Text } from '@chakra-ui/react';
import { chunk } from 'es-toolkit';
import React, { useMemo, useState } from 'react';

import type { PaginationParams } from 'ui/shared/pagination/types';

import useApiQuery from 'lib/api/useApiQuery';
import { NOVES_TRANSLATE } from 'stubs/noves/NovesTranslate';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import ActionBar from 'ui/shared/ActionBar';
import DataListDisplay from 'ui/shared/DataListDisplay';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import Pagination from 'ui/shared/pagination/Pagination';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import TxAssetFlowsListItem from './assetFlows/TxAssetFlowsListItem';
import TxAssetFlowsTableItem from './assetFlows/TxAssetFlowsTableItem';
import { generateFlowViewData } from './assetFlows/utils/generateFlowViewData';

interface FlowViewProps {
  hash: string;
}

export default function TxAssetFlows(props: FlowViewProps) {

  const { data: queryData, isPlaceholderData, isError } = useApiQuery('general:noves_transaction', {
    pathParams: { hash: props.hash },
    queryOptions: {
      enabled: Boolean(props.hash),
      placeholderData: NOVES_TRANSLATE,
    },
  });

  const [ page, setPage ] = useState<number>(1);

  const ViewData = useMemo(() => (queryData ? generateFlowViewData(queryData) : []), [ queryData ]);
  const chunkedViewData = chunk(ViewData, 50);

  const paginationProps: PaginationParams = useMemo(() => ({
    onNextPageClick: () => setPage(page + 1),
    onPrevPageClick: () => setPage(page - 1),
    resetPage: () => setPage(1),
    canGoBackwards: page > 1,
    isLoading: isPlaceholderData,
    page: page,
    hasNextPage: Boolean(chunkedViewData[page]),
    hasPages: Boolean(chunkedViewData[1]),
    isVisible: Boolean(chunkedViewData[1]),
  }), [ chunkedViewData, page, isPlaceholderData ]);

  const data = chunkedViewData [page - 1];

  const actionBar = (
    <ActionBar mt={ -6 } pb={{ base: 6, md: 5 }} flexDir={{ base: 'column', md: 'initial' }} gap={{ base: '2', md: 'initial' }} >
      <Box display="flex" alignItems="center" flexWrap="wrap" minW={ 0 } maxW="full" gap={ 1 }>
        <Skeleton borderRadius="sm" loading={ isPlaceholderData } >
          <Text fontWeight="normal" mr={ 1 }>
            Wallet
          </Text>
        </Skeleton>

        <AddressEntity
          address={{ hash: queryData?.accountAddress || '' }}
          fontWeight="normal"
          truncation="dynamic"
          minW={ 0 }
          maxW="full"
          css={{
            '& [data-entity-content]': { minW: 0, whiteSpace: 'normal', overflowWrap: 'anywhere' },
            '& button': { minW: 'touchTarget', minH: 'touchTarget' },
          }}
          isLoading={ isPlaceholderData }
        />
      </Box>
      <Pagination ml={{ base: 'auto', lg: 8 }} { ...paginationProps }/>
    </ActionBar>
  );

  const content = (
    <>
      <Box hideFrom="lg">
        { data?.map((item, i) => (
          <TxAssetFlowsListItem
            key={ `${ i }-${ item.accountAddress }` }
            item={ item }
            isPlaceholderData={ isPlaceholderData }
          />
        )) }
      </Box>

      <Box hideBelow="lg">
        <ActivityTable label="Asset flows">
          <TableHeaderSticky>
            <TableRow>
              <TableColumnHeader>
                Actions
              </TableColumnHeader>
              <TableColumnHeader width="var(--chakra-spacing-100)">
                From/To
              </TableColumnHeader>
            </TableRow>
          </TableHeaderSticky>
          <TableBody>
            { data?.map((item, i) => (
              <TxAssetFlowsTableItem
                key={ `${ i }-${ item.accountAddress }` }
                item={ item }
                isPlaceholderData={ isPlaceholderData }
              />
            )) }
          </TableBody>
        </ActivityTable>
      </Box>
    </>
  );

  return (
    <DataListDisplay
      isError={ isError }
      itemsNum={ data?.length }
      emptyText="There are no transfers."
      actionBar={ actionBar }
    >
      { content }
    </DataListDisplay>
  );
}
