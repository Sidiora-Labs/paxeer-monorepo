import React from 'react';

import type { ZkSyncBatchesItem } from 'types/api/zkSyncL2';

import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import ZkSyncTxnBatchesTableItem from './ZkSyncTxnBatchesTableItem';

type Props = {
  items: Array<ZkSyncBatchesItem>;
  top: number;
  isLoading?: boolean;
};

const ZkSyncTxnBatchesTable = ({ items, top, isLoading }: Props) => {
  return (
    <ActivityTable label="Transaction batches" minW="var(--chakra-sizes-5xl)">
      <TableHeaderSticky top={ top }>
        <TableRow>
          <TableColumnHeader width="40%" isNumeric>Batch #</TableColumnHeader>
          <TableColumnHeader width="60%">Status</TableColumnHeader>
          <TableColumnHeader width="var(--chakra-spacing-44)">
            Timestamp
            <TimeFormatToggle/>
          </TableColumnHeader>
          <TableColumnHeader width="var(--chakra-spacing-30)" isNumeric>Txn count</TableColumnHeader>
          <TableColumnHeader width="var(--chakra-spacing-52)">Commit tx</TableColumnHeader>
          <TableColumnHeader width="var(--chakra-spacing-52)">Prove tx</TableColumnHeader>
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { items.map((item, index) => (
          <ZkSyncTxnBatchesTableItem
            key={ item.number + (isLoading ? String(index) : '') }
            item={ item }
            isLoading={ isLoading }
          />
        )) }
      </TableBody>
    </ActivityTable>
  );
};

export default ZkSyncTxnBatchesTable;
