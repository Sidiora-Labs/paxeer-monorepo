import React from 'react';

import type { ScrollL2TxnBatch } from 'types/api/scrollL2';

import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import ScrollL2TxnBatchesTableItem from './ScrollL2TxnBatchesTableItem';

type Props = {
  items: Array<ScrollL2TxnBatch>;
  top: number;
  isLoading?: boolean;
};

const ScrollL2TxnBatchesTable = ({ items, top, isLoading }: Props) => {
  return (
    <ActivityTable label="Transaction batches" tableLayout="auto" minW="var(--chakra-sizes-5xl)">
      <TableHeaderSticky top={ top }>
        <TableRow>
          <TableColumnHeader isNumeric>Batch #</TableColumnHeader>
          <TableColumnHeader>Container</TableColumnHeader>
          <TableColumnHeader>Status</TableColumnHeader>
          <TableColumnHeader isNumeric>Committed block</TableColumnHeader>
          <TableColumnHeader>Committed txn hash</TableColumnHeader>
          <TableColumnHeader>
            Timestamp
            <TimeFormatToggle/>
          </TableColumnHeader>
          <TableColumnHeader isNumeric>Finalized block</TableColumnHeader>
          <TableColumnHeader>Finalized txn hash</TableColumnHeader>
          <TableColumnHeader isNumeric>Blocks</TableColumnHeader>
          <TableColumnHeader isNumeric>Txn</TableColumnHeader>
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { items.map((item, index) => (
          <ScrollL2TxnBatchesTableItem
            key={ item.number + (isLoading ? String(index) : '') }
            item={ item }
            isLoading={ isLoading }
          />
        )) }
      </TableBody>
    </ActivityTable>
  );
};

export default ScrollL2TxnBatchesTable;
