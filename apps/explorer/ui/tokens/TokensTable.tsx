import React from 'react';

import type { TokenInfo } from 'types/api/token';
import type { TokensSortingField, TokensSortingValue } from 'types/api/tokens';
import type { AggregatedTokenInfo } from 'types/client/multichainAggregator';

import { TableBody, TableColumnHeader, TableColumnHeaderSortable, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import { LIST_TABLE_STYLES } from 'ui/shared/Page/ListPage';
import { default as getNextSortValueShared } from 'ui/shared/sort/getNextSortValue';

import TokensTableItem from './TokensTableItem';

const SORT_SEQUENCE: Record<TokensSortingField, Array<TokensSortingValue>> = {
  fiat_value: [ 'fiat_value-desc', 'fiat_value-asc', 'default' ],
  holders_count: [ 'holders_count-desc', 'holders_count-asc', 'default' ],
  circulating_market_cap: [ 'circulating_market_cap-desc', 'circulating_market_cap-asc', 'default' ],
};

const getNextSortValue = (getNextSortValueShared<TokensSortingField, TokensSortingValue>).bind(undefined, SORT_SEQUENCE);

type Props = {
  items: Array<TokenInfo> | Array<AggregatedTokenInfo>;
  page: number;
  sorting?: TokensSortingValue;
  setSorting?: (value: TokensSortingValue) => void;
  isLoading?: boolean;
  coinPrice?: string | null;
};

const TokensTable = ({ items, page, isLoading, sorting, setSorting }: Props) => {

  const hasSorting = setSorting && sorting;

  const sort = React.useCallback((field: TokensSortingField) => {
    if (!hasSorting) {
      return;
    }
    const value = getNextSortValue(field)(sorting);
    setSorting(value);
  }, [ sorting, setSorting, hasSorting ]);

  return (
    <TableRoot variant="scan" css={ LIST_TABLE_STYLES } data-tokens-table>
      <TableHeader>
        <TableRow>
          <TableColumnHeader w="56px" isNumeric>#</TableColumnHeader>
          <TableColumnHeader w="26%">Token</TableColumnHeader>
          { hasSorting ? (
            <TableColumnHeaderSortable
              isNumeric
              w="14%"
              sortField="fiat_value"
              sortValue={ sorting }
              onSortToggle={ sort }
              indicatorPosition="right"
            >
              Price
            </TableColumnHeaderSortable>
          ) : (
            <TableColumnHeader isNumeric w="14%">
              Price
            </TableColumnHeader>
          ) }
          <TableColumnHeader isNumeric w="12%">Change 24h</TableColumnHeader>
          { hasSorting ? (
            <TableColumnHeaderSortable
              isNumeric
              w="15%"
              sortField="circulating_market_cap"
              sortValue={ sorting }
              onSortToggle={ sort }
              indicatorPosition="right"
            >
              Market cap
            </TableColumnHeaderSortable>
          ) : (
            <TableColumnHeader isNumeric w="15%">
              Market cap
            </TableColumnHeader>
          ) }
          <TableColumnHeader isNumeric w="14%">Volume 24h</TableColumnHeader>
          { hasSorting ? (
            <TableColumnHeaderSortable
              isNumeric
              w="10%"
              sortField="holders_count"
              sortValue={ sorting }
              onSortToggle={ sort }
              indicatorPosition="right"
            >
              Holders
            </TableColumnHeaderSortable>
          ) : (
            <TableColumnHeader isNumeric w="10%">
              Holders
            </TableColumnHeader>
          ) }
        </TableRow>
      </TableHeader>
      <TableBody>
        { items.map((item, index) => {
          const chainIds = 'chain_infos' in item ? Object.keys(item.chain_infos).join(',') : undefined;

          return (
            <TokensTableItem
              key={ item.address_hash + (isLoading ? index : '') + (chainIds ? chainIds : '') }
              token={ item }
              index={ index }
              page={ page }
              isLoading={ isLoading }
            />
          );
        }) }
      </TableBody>
    </TableRoot>
  );
};

export default TokensTable;
