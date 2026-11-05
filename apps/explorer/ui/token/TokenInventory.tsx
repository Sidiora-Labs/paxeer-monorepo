import { Flex, Grid, Text } from '@chakra-ui/react';
import type { UseQueryResult } from '@tanstack/react-query';
import React from 'react';

import type { TokenInfo } from 'types/api/token';

import type { ResourceError } from 'lib/api/resources';
import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import useIsMobile from 'lib/hooks/useIsMobile';
import useIsMounted from 'lib/hooks/useIsMounted';
import DataListDisplay from 'ui/shared/DataListDisplay';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import Pagination from 'ui/shared/pagination/Pagination';
import type { QueryWithPagesResult } from 'ui/shared/pagination/useQueryWithPages';
import ResetIconButton from 'ui/shared/ResetIconButton';
import { ScanShowRows, ScanTableCard, SCAN_ROWS_PER_PAGE } from 'ui/shared/scan';

import getTokenInventoryTitle from './getTokenInventoryTitle';
import TokenInventoryItem from './TokenInventoryItem';

const DEFAULT_ROWS_TO_SHOW = 50;
const ROWS_PER_PAGE = SCAN_ROWS_PER_PAGE.filter((size) => size <= DEFAULT_ROWS_TO_SHOW);

type Props = {
  inventoryQuery: QueryWithPagesResult<'general:token_inventory'>;
  tokenQuery: UseQueryResult<TokenInfo, ResourceError<unknown>>;
  ownerFilter?: string;
  shouldRender?: boolean;
  inventoryCount?: number;
};

const TokenInventory = ({ inventoryQuery, tokenQuery, ownerFilter, shouldRender = true, inventoryCount }: Props) => {
  const isMobile = useIsMobile();
  const isMounted = useIsMounted();
  const [ rowsToShow, setRowsToShow ] = React.useState(DEFAULT_ROWS_TO_SHOW);
  const inventoryKey = JSON.stringify([ tokenQuery.data?.address_hash, ownerFilter, inventoryQuery.chainValue ]);
  const [ localPage, setLocalPage ] = React.useState<{
    serverPage: number;
    inventoryKey: string;
    index: number | 'last';
  }>({ serverPage: inventoryQuery.pagination.page, inventoryKey, index: 0 });

  const serverPagination = inventoryQuery.pagination;
  const localPageCount = Math.max(1, Math.ceil((inventoryQuery.data?.items.length ?? 0) / rowsToShow));
  const samePage = localPage.serverPage === serverPagination.page && localPage.inventoryKey === inventoryKey;
  const localIndex = samePage ? Math.min(localPage.index === 'last' ? localPageCount - 1 : localPage.index, localPageCount - 1) : 0;
  const selectLocalPage = React.useCallback((index: number | 'last', serverPage = serverPagination.page) => {
    setLocalPage({ serverPage, inventoryKey, index });
  }, [ serverPagination.page, inventoryKey ]);
  const handleRowsChange = React.useCallback((size: number) => {
    setRowsToShow(size);
    selectLocalPage(0);
  }, [ selectLocalPage ]);
  const pagination = {
    ...serverPagination,
    page: (serverPagination.page - 1) * (DEFAULT_ROWS_TO_SHOW / rowsToShow) + localIndex + 1,
    hasPages: serverPagination.hasPages || localIndex > 0,
    hasNextPage: localIndex + 1 < localPageCount || serverPagination.hasNextPage,
    canGoBackwards: localIndex > 0 || serverPagination.canGoBackwards,
    isVisible: serverPagination.isVisible || localPageCount > 1,
    onNextPageClick: () => {
      if (localIndex + 1 < localPageCount) {
        selectLocalPage(localIndex + 1);
      } else {
        selectLocalPage(0, serverPagination.page + 1);
        serverPagination.onNextPageClick();
      }
    },
    onPrevPageClick: () => {
      if (localIndex > 0) {
        selectLocalPage(localIndex - 1);
      } else {
        selectLocalPage('last', serverPagination.page - 1);
        serverPagination.onPrevPageClick();
      }
    },
    resetPage: () => {
      selectLocalPage(0, 1);
      if (serverPagination.page !== 1) {
        serverPagination.resetPage();
      }
    },
  };

  const resetOwnerFilter = React.useCallback(() => {
    selectLocalPage(0, 1);
    inventoryQuery.onFilterChange({});
  }, [ inventoryQuery, selectLocalPage ]);

  if (!isMounted || !shouldRender) {
    return null;
  }

  const ownerFilterComponent = ownerFilter ? (
    <Flex alignItems="center" flexWrap="wrap" columnGap={ 2 } data-inventory-owner-filter>
      <Text whiteSpace="nowrap">Filtered by owner</Text>
      <Flex alignItems="center">
        <AddressEntity address={{ hash: ownerFilter }} truncation={ isMobile ? 'constant' : 'none' }/>
        <ResetIconButton onClick={ resetOwnerFilter }/>
      </Flex>
    </Flex>
  ) : null;

  const items = inventoryQuery.data?.items.slice(localIndex * rowsToShow, (localIndex + 1) * rowsToShow);
  const token = tokenQuery.data;

  const content = items && token ? (
    <AddressHighlightProvider>
      <Grid
        w="100%"
        p={ 4 }
        columnGap={{ base: 3, lg: 6 }}
        rowGap={{ base: 3, lg: 6 }}
        gridTemplateColumns={{ base: 'repeat(2, calc((100% - 12px)/2))', lg: 'repeat(auto-fill, minmax(210px, 1fr))' }}
        data-inventory-grid
      >
        { items.map((item, index) => (
          <TokenInventoryItem
            key={ item.id + '_' + index + (inventoryQuery.isPlaceholderData ? '_' + 'placeholder' : '') }
            item={ item }
            isLoading={ inventoryQuery.isPlaceholderData || tokenQuery.isPlaceholderData }
            token={ token }
          />
        )) }
      </Grid>
    </AddressHighlightProvider>
  ) : null;

  const itemsNum = items?.length ?? 0;
  const title = getTokenInventoryTitle({ itemsNum, hasNextPage: pagination.hasNextPage, inventoryCount });

  return (
    <ScanTableCard
      title={ title }
      actions={ ownerFilterComponent }
      pagination={ pagination.isVisible ? <Pagination { ...pagination }/> : null }
      showRows={ (
        <ScanShowRows
          value={ rowsToShow }
          onValueChange={ handleRowsChange }
          options={ ROWS_PER_PAGE }
          label="Show"
          suffix="Records"
          isLoading={ inventoryQuery.isPlaceholderData }
        />
      ) }
    >
      <DataListDisplay
        isError={ inventoryQuery.isError }
        itemsNum={ itemsNum }
        emptyText="There are no tokens."
        hasActiveFilters={ Boolean(ownerFilter) }
        emptyStateProps={{
          description: 'No tokens found for the selected owner.',
        }}
      >
        { content }
      </DataListDisplay>
    </ScanTableCard>
  );
};

export default TokenInventory;
