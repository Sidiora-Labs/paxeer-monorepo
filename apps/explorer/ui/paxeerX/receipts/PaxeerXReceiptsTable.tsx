import React from 'react';

import type { PaxeerXArchiveReceiptsItem } from 'types/api/paxeerXArchive';

import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import TableScrollArea from 'ui/blocks/TableScrollArea';

import PaxeerXReceiptsTableItem from './PaxeerXReceiptsTableItem';

interface Props {
  items: Array<PaxeerXArchiveReceiptsItem>;
  top?: number;
  isLoading?: boolean;
}

const PaxeerXReceiptsTable = ({ items, top = 0, isLoading }: Props) => {
  return (
    <TableScrollArea label="Receipts">
      <TableRoot aria-label="Receipts" variant="scan" minW="max-content" data-label="paxeer-x-receipts">
        <TableHeaderSticky top={ top }>
          <TableRow>
            <TableColumnHeader width="40%">Receipt ID</TableColumnHeader>
            <TableColumnHeader width="40%">Account</TableColumnHeader>
            <TableColumnHeader width="20%">Block</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { items.map((item, index) => (
            <PaxeerXReceiptsTableItem
              key={ item.id + (isLoading ? index : '') }
              item={ item }
              isLoading={ isLoading }
            />
          )) }
        </TableBody>
      </TableRoot>
    </TableScrollArea>
  );
};

export default PaxeerXReceiptsTable;
