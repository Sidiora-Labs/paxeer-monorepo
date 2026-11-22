import React from 'react';

import type { ZkEvmL2TxnBatchesItem } from 'types/api/zkEvmL2';

import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import ZkEvmTxnBatchesTableItem from './ZkEvmTxnBatchesTableItem';

type Props = {
  items: Array<ZkEvmL2TxnBatchesItem>;
  top: number;
  isLoading?: boolean;
};

const ZkEvmTxnBatchesTable = ({ items, top, isLoading }: Props) => {
  return (
    <ActivityTable label="Transaction batches" minW="var(--chakra-sizes-6xl)">
      <TableHeaderSticky top={ top }>
        <TableRow>
          <TableColumnHeader width="40%" isNumeric>Batch #</TableColumnHeader>
          <TableColumnHeader width="60%">Status</TableColumnHeader>
          <TableColumnHeader width="var(--chakra-spacing-44)">
            Timestamp
            <TimeFormatToggle/>
          </TableColumnHeader>
          <TableColumnHeader width="var(--chakra-spacing-24)" isNumeric>Txn count</TableColumnHeader>
          <TableColumnHeader width="var(--chakra-spacing-56)">Verify tx hash</TableColumnHeader>
          <TableColumnHeader width="var(--chakra-spacing-56)">Sequence hash</TableColumnHeader>
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { items.map((item, index) => (
          <ZkEvmTxnBatchesTableItem
            key={ item.number + (isLoading ? String(index) : '') }
            item={ item }
            isLoading={ isLoading }
          />
        )) }
      </TableBody>
    </ActivityTable>
  );
};

export default ZkEvmTxnBatchesTable;
