import React from 'react';

import type { TokenTransfer } from 'types/api/tokenTransfer';

import { ACTION_BAR_HEIGHT_DESKTOP } from 'ui/shared/ActionBar';
import DataListDisplay from 'ui/shared/DataListDisplay';
import type { QueryWithPagesResult } from 'ui/shared/pagination/useQueryWithPages';
import TokenTransferList from 'ui/shared/TokenTransfer/TokenTransferList';
import TokenTransferTable from 'ui/shared/TokenTransfer/TokenTransferTable';
import ActivityViews from 'ui/txs/ActivityViews';

import ActivityLoading from '../ActivityLoading';
import type { TxQuery } from '../useTxQuery';

interface Props {
  txQuery: TxQuery;
  tokenTransferQuery: QueryWithPagesResult<'general:tx_token_transfers'>;
  tokenTransferFilter?: (data: TokenTransfer) => boolean;
  numActiveFilters: number;
}

const TxTokenTransferLocal = ({ txQuery, tokenTransferQuery, tokenTransferFilter, numActiveFilters }: Props) => {

  let items: Array<TokenTransfer> = [];

  if (tokenTransferQuery.data?.items) {
    if (tokenTransferQuery.isPlaceholderData) {
      items = tokenTransferQuery.data?.items;
    } else {
      items = tokenTransferFilter ? tokenTransferQuery.data.items.filter(tokenTransferFilter) : tokenTransferQuery.data.items;
    }
  }

  const loadedContent = tokenTransferQuery.data?.items ? (
    <ActivityViews
      list={
        <TokenTransferList data={ items } isLoading={ tokenTransferQuery.isPlaceholderData }/>
      }
      table={
        <TokenTransferTable data={ items } top={ ACTION_BAR_HEIGHT_DESKTOP } isLoading={ tokenTransferQuery.isPlaceholderData }/>
      }
    />
  ) : null;
  const content = tokenTransferQuery.isPlaceholderData ? <ActivityLoading label="Loading token transfers"/> : loadedContent;

  return (
    <DataListDisplay
      isError={ txQuery.isError || tokenTransferQuery.isError }
      itemsNum={ items.length }
      emptyText="There are no token transfers."
      hasActiveFilters={ Boolean(numActiveFilters) }
      emptyStateProps={{
        term: 'token transfer',
      }}
    >
      { content }
    </DataListDisplay>
  );
};

export default React.memo(TxTokenTransferLocal);
