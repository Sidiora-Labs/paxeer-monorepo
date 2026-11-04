import { Box } from '@chakra-ui/react';
import React from 'react';

import type { TokenInfo } from 'types/api/token';

import useIsMobile from 'lib/hooks/useIsMobile';
import useIsMounted from 'lib/hooks/useIsMounted';
import AddressCsvExportLink from 'ui/address/AddressCsvExportLink';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import DataListDisplay from 'ui/shared/DataListDisplay';
import Pagination from 'ui/shared/pagination/Pagination';
import type { QueryWithPagesResult } from 'ui/shared/pagination/useQueryWithPages';
import { formatScanTableCount, ScanShowRows, ScanTableCard, SCAN_ROWS_PER_PAGE } from 'ui/shared/scan';

import TokenHoldersList from './TokenHoldersList';
import TokenHoldersTable from './TokenHoldersTable';

const DEFAULT_ROWS_TO_SHOW = 50;
const ROWS_PER_PAGE = SCAN_ROWS_PER_PAGE.filter((size) => size <= DEFAULT_ROWS_TO_SHOW);

type Props = {
  token?: TokenInfo;
  holdersQuery: QueryWithPagesResult<'general:token_holders'>;
  shouldRender?: boolean;
  holdersCount?: number;
};

const TokenHolders = ({ holdersQuery, token, shouldRender = true, holdersCount }: Props) => {
  const isMobile = useIsMobile();
  const isMounted = useIsMounted();
  const [ rowsToShow, setRowsToShow ] = React.useState(DEFAULT_ROWS_TO_SHOW);
  const [ localPage, setLocalPage ] = React.useState<{
    serverPage: number;
    tokenHash?: string;
    index: number | 'last';
  }>({ serverPage: holdersQuery.pagination.page, tokenHash: token?.address_hash, index: 0 });

  const serverPagination = holdersQuery.pagination;
  const localPageCount = Math.max(1, Math.ceil((holdersQuery.data?.items.length ?? 0) / rowsToShow));
  const samePage = localPage.serverPage === serverPagination.page && localPage.tokenHash === token?.address_hash;
  const localIndex = samePage ? Math.min(localPage.index === 'last' ? localPageCount - 1 : localPage.index, localPageCount - 1) : 0;
  const selectLocalPage = React.useCallback((index: number | 'last', serverPage = serverPagination.page) => {
    setLocalPage({ serverPage, tokenHash: token?.address_hash, index });
  }, [ serverPagination.page, token?.address_hash ]);
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

  if (!isMounted || !shouldRender) {
    return null;
  }

  if (holdersQuery.isError) {
    return <DataFetchAlert/>;
  }

  const items = holdersQuery.data?.items.slice(localIndex * rowsToShow, (localIndex + 1) * rowsToShow);

  const content = items && token ? (
    <>
      <Box display={{ base: 'none', lg: 'block' }}>
        <TokenHoldersTable
          data={ items }
          token={ token }
          top={ 0 }
          isLoading={ holdersQuery.isPlaceholderData }
        />
      </Box>
      <Box display={{ base: 'block', lg: 'none' }}>
        <TokenHoldersList
          data={ items }
          token={ token }
          isLoading={ holdersQuery.isPlaceholderData }
        />
      </Box>
    </>
  ) : null;

  const itemsNum = items?.length ?? 0;
  const title = holdersCount === undefined ? `${ itemsNum.toLocaleString() } holders on this page` :
    formatScanTableCount(itemsNum < holdersCount ?
      { kind: 'latest' as const, value: holdersCount, itemsName: 'holders', shownValue: itemsNum } :
      { kind: 'total' as const, value: holdersCount, itemsName: 'holders' });

  const actions = !isMobile && token ? (
    <AddressCsvExportLink
      address={ token.address_hash }
      label="Download Page Data"
      params={{ type: 'holders' }}
      isLoading={ holdersQuery.pagination.isLoading }
    />
  ) : null;

  return (
    <ScanTableCard
      title={ title }
      actions={ actions }
      pagination={ pagination.isVisible ? <Pagination { ...pagination }/> : null }
      showRows={ (
        <ScanShowRows
          value={ rowsToShow }
          onValueChange={ handleRowsChange }
          options={ ROWS_PER_PAGE }
          label="Show"
          suffix="Records"
          isLoading={ holdersQuery.pagination.isLoading }
        />
      ) }
    >
      <DataListDisplay
        isError={ holdersQuery.isError }
        itemsNum={ itemsNum }
        emptyText="There are no holders for this token."
      >
        { content }
      </DataListDisplay>
    </ScanTableCard>
  );
};

export default TokenHolders;
