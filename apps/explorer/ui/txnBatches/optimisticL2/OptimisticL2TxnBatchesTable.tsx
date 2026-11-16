import React from 'react';

import type { OptimisticL2TxnBatchesItem } from 'types/api/optimisticL2';

import { layerLabels } from 'lib/rollups/utils';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import OptimisticL2TxnBatchesTableItem from './OptimisticL2TxnBatchesTableItem';

type Props = {
  items: Array<OptimisticL2TxnBatchesItem>;
  top: number;
  isLoading?: boolean;
};

const OptimisticL2TxnBatchesTable = ({ items, top, isLoading }: Props) => {
  return (
    <ActivityTable label="Transaction batches" tableLayout="auto" minW="var(--chakra-sizes-4xl)">
      <TableHeaderSticky top={ top }>
        <TableRow>
          <TableColumnHeader isNumeric>Batch ID</TableColumnHeader>
          <TableColumnHeader>Storage</TableColumnHeader>
          <TableColumnHeader>
            Timestamp
            <TimeFormatToggle/>
          </TableColumnHeader>
          <TableColumnHeader isNumeric>{ layerLabels.parent } txn count</TableColumnHeader>
          <TableColumnHeader isNumeric>{ layerLabels.current } blocks</TableColumnHeader>
          <TableColumnHeader isNumeric>Txn</TableColumnHeader>
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { items.map((item, index) => (
          <OptimisticL2TxnBatchesTableItem
            key={ item.number + (isLoading ? String(index) : '') }
            item={ item }
            isLoading={ isLoading }
          />
        )) }
      </TableBody>
    </ActivityTable>
  );
};

export default OptimisticL2TxnBatchesTable;
