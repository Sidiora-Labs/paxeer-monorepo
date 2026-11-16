import { Text } from '@chakra-ui/react';
import React from 'react';

import type { PaxeerXBalance } from 'types/api/paxeerX';

import { TableBody, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import TableScrollArea from 'ui/blocks/TableScrollArea';
import Pagination from 'ui/shared/pagination/Pagination';
import { formatScanTableCount, SCAN_ROWS_PER_PAGE } from 'ui/shared/scan';

import AccountRowsSelect from './AccountRowsSelect';
import AccountTableCard from './AccountTableCard';
import AssetListItem from './AssetListItem';

export interface Props {
  items: Array<PaxeerXBalance>;
  isLoading?: boolean;
}

const AssetList = ({ items, isLoading }: Props) => {
  const [ rowsPerPage, setRowsPerPage ] = React.useState(SCAN_ROWS_PER_PAGE[0]);
  const [ page, setPage ] = React.useState(1);

  const pageCount = Math.max(1, Math.ceil(items.length / rowsPerPage));
  const currentPage = Math.min(page, pageCount);
  const rows = items.slice((currentPage - 1) * rowsPerPage, currentPage * rowsPerPage);

  const handleNextPageClick = React.useCallback(() => setPage((value) => value + 1), []);
  const handlePrevPageClick = React.useCallback(() => setPage((value) => Math.max(1, value - 1)), []);
  const handleResetPage = React.useCallback(() => setPage(1), []);
  const handleRowsPerPageChange = React.useCallback((value: number) => {
    setRowsPerPage(value);
    setPage(1);
  }, []);

  const paginationNode = (
    <Pagination
      page={ currentPage }
      pageCount={ pageCount }
      onNextPageClick={ handleNextPageClick }
      onPrevPageClick={ handlePrevPageClick }
      resetPage={ handleResetPage }
      hasPages={ pageCount > 1 }
      hasNextPage={ currentPage < pageCount }
      canGoBackwards={ currentPage > 1 }
      isLoading={ Boolean(isLoading) }
      isVisible={ pageCount > 1 }
    />
  );

  return (
    <AccountTableCard
      title={ formatScanTableCount({ kind: 'total', value: items.length, itemsName: 'assets' }) }
      note="Total balance for each asset held by this account"
      pagination={ paginationNode }
      showRows={ <AccountRowsSelect value={ rowsPerPage } onValueChange={ handleRowsPerPageChange } isLoading={ isLoading }/> }
    >
      { items.length === 0 ? (
        <Text role="status" color="text.secondary" px={ 4 } py={ 6 }>No assets are held by this account.</Text>
      ) : (
        <TableScrollArea label="Account assets">
          <TableRoot aria-label="Account assets" variant="scan" minW="max-content" data-label="paxeer-x-assets">
            <TableHeader>
              <TableRow>
                <TableColumnHeader width="40%">Asset</TableColumnHeader>
                <TableColumnHeader width="35%">Denom</TableColumnHeader>
                <TableColumnHeader width="25%" isNumeric>Total</TableColumnHeader>
              </TableRow>
            </TableHeader>
            <TableBody>
              { rows.map((item) => (
                <AssetListItem key={ item.asset.id } item={ item } isLoading={ isLoading }/>
              )) }
            </TableBody>
          </TableRoot>
        </TableScrollArea>
      ) }
    </AccountTableCard>
  );
};

export default React.memo(AssetList);
