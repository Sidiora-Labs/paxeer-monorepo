import { Text } from '@chakra-ui/react';
import React from 'react';

import type { PaxeerXActivityItem } from 'types/api/paxeerX';

import { TableBody, TableColumnHeader, TableHeader, TableRoot, TableRow } from 'toolkit/chakra/table';
import TableScrollArea from 'ui/blocks/TableScrollArea';
import Pagination from 'ui/shared/pagination/Pagination';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';

import AccountTableCard from './AccountTableCard';
import ActivityListItem from './ActivityListItem';

export interface Props {
  items: Array<PaxeerXActivityItem>;
  isLoading?: boolean;
  page: number;
  total: number | null;
  hasNextPage: boolean;
  canGoBackwards: boolean;
  onNextPageClick: () => void;
  onPrevPageClick: () => void;
  resetPage: () => void;
}

// Native network activity, each row carrying its rung on the status ladder.
const ActivityList = ({ items, isLoading, page, total, hasNextPage, canGoBackwards,
  onNextPageClick, onPrevPageClick, resetPage }: Props) => {
  const paginationNode = (
    <Pagination
      page={ page }
      onNextPageClick={ onNextPageClick }
      onPrevPageClick={ onPrevPageClick }
      resetPage={ resetPage }
      hasPages={ hasNextPage || page > 1 }
      hasNextPage={ hasNextPage }
      canGoBackwards={ canGoBackwards }
      isLoading={ Boolean(isLoading) }
      isVisible
    />
  );

  let title: string;
  if (isLoading) {
    title = 'Loading activity entries…';
  } else if (total !== null) {
    title = `A total of ${ total } activity entries found`;
  } else {
    title = `${ items.length } activity entries on this page`;
  }

  return (
    <AccountTableCard
      title={ title }
      note="Native network activity, newest first"
      pagination={ paginationNode }
    >
      { items.length === 0 ? (
        <Text role="status" color="text.secondary" px={ 4 } py={ 6 }>
          { page === 1 && total === 0 ? 'There is no Paxeer X activity for this account yet.' : 'There are no activity entries on this page.' }
        </Text>
      ) : (
        <TableScrollArea label="Account activity">
          <TableRoot aria-label="Account activity" variant="scan" minW="max-content" data-label="paxeer-x-activity">
            <TableHeader>
              <TableRow>
                <TableColumnHeader width="20%">Action</TableColumnHeader>
                <TableColumnHeader width="30%">Transaction</TableColumnHeader>
                <TableColumnHeader width="20%">
                  Block
                  <TimeFormatToggle/>
                </TableColumnHeader>
                <TableColumnHeader width="20%" isNumeric>Amount</TableColumnHeader>
                <TableColumnHeader width="10%">Status</TableColumnHeader>
              </TableRow>
            </TableHeader>
            <TableBody>
              { items.map((item) => (
                <ActivityListItem key={ `${ item.hash }-${ item.kind }-${ item.block_number }-${ item.ordinal }` } item={ item } isLoading={ isLoading }/>
              )) }
            </TableBody>
          </TableRoot>
        </TableScrollArea>
      ) }
    </AccountTableCard>
  );
};

export default React.memo(ActivityList);
