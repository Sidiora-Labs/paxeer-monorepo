import React from 'react';

import type { ArbitrumL2TxnBatchesItem } from 'types/api/arbitrumL2';

import { layerLabels } from 'lib/rollups/utils';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import ArbitrumL2TxnBatchesTableItem from './ArbitrumL2TxnBatchesTableItem';

type Props = {
  items: Array<ArbitrumL2TxnBatchesItem>;
  top: number;
  isLoading?: boolean;
};

const ArbitrumL2TxnBatchesTable = ({ items, top, isLoading }: Props) => {
  return (
    <ActivityTable label="Transaction batches" tableLayout="auto" minW="var(--chakra-sizes-5xl)">
      <TableHeaderSticky top={ top }>
        <TableRow>
          <TableColumnHeader isNumeric>Batch #</TableColumnHeader>
          <TableColumnHeader>{ layerLabels.parent } status</TableColumnHeader>
          <TableColumnHeader isNumeric>{ layerLabels.parent } block</TableColumnHeader>
          <TableColumnHeader isNumeric>Block count</TableColumnHeader>
          <TableColumnHeader>{ layerLabels.parent } transaction</TableColumnHeader>
          <TableColumnHeader>
            Timestamp
            <TimeFormatToggle/>
          </TableColumnHeader>
          <TableColumnHeader isNumeric>Txn count</TableColumnHeader>
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { items.map((item, index) => (
          <ArbitrumL2TxnBatchesTableItem
            key={ item.number + (isLoading ? String(index) : '') }
            item={ item }
            isLoading={ isLoading }
          />
        )) }
      </TableBody>
    </ActivityTable>
  );
};

export default ArbitrumL2TxnBatchesTable;
