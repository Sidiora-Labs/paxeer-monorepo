import React from 'react';

import type { WatchlistAddress } from 'types/api/account';

import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import { ResponsiveTableRegion } from 'ui/token/ResponsiveTableRegion';

import WatchlistTableItem from './WatchListTableItem';

interface Props {
  data?: Array<WatchlistAddress>;
  isLoading?: boolean;
  onEditClick: (data: WatchlistAddress) => void;
  onDeleteClick: (data: WatchlistAddress) => void;
  top: number;
  hasEmail: boolean;
}

const WatchlistTable = ({ data, isLoading, onDeleteClick, onEditClick, top, hasEmail }: Props) => {
  return (
    <ResponsiveTableRegion label="Watchlist addresses" minWidth="xl">
      <TableRoot minWidth="xl">
        <TableHeaderSticky top={ top }>
          <TableRow>
            <TableColumnHeader width="70%">Address</TableColumnHeader>
            <TableColumnHeader width="30%">Private tag</TableColumnHeader>
            <TableColumnHeader width="40">Email notification</TableColumnHeader>
            <TableColumnHeader width="28"></TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { data?.map((item, index) => (
            <WatchlistTableItem
              key={ item.address_hash + (isLoading ? index : '') }
              item={ item }
              isLoading={ isLoading }
              onDeleteClick={ onDeleteClick }
              onEditClick={ onEditClick }
              hasEmail={ hasEmail }
            />
          )) }
        </TableBody>
      </TableRoot>
    </ResponsiveTableRegion>
  );
};

export default WatchlistTable;
