import React from 'react';

import type { OptimisticL2WithdrawalsItem } from 'types/api/optimisticL2';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { layerLabels } from 'lib/rollups/utils';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import OptimisticL2WithdrawalsTableItem from './OptimisticL2WithdrawalsTableItem';

type Props = {
  items: Array<OptimisticL2WithdrawalsItem>;
  top: number;
  isLoading?: boolean;
};

const OptimisticL2WithdrawalsTable = ({ items, top, isLoading }: Props) => {
  return (
    <AddressHighlightProvider>
      <ActivityTable label="Withdrawals" tableLayout="auto" minW="var(--chakra-sizes-4xl)">
        <TableHeaderSticky top={ top }>
          <TableRow>
            <TableColumnHeader isNumeric>Msg nonce</TableColumnHeader>
            <TableColumnHeader>From</TableColumnHeader>
            <TableColumnHeader>{ layerLabels.current } txn hash</TableColumnHeader>
            <TableColumnHeader>
              Timestamp
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader>Status</TableColumnHeader>
            <TableColumnHeader>{ layerLabels.parent } txn hash</TableColumnHeader>
            <TableColumnHeader>Time left</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { items.map((item, index) => (
            <OptimisticL2WithdrawalsTableItem
              key={ String(item.msg_nonce_version) + item.msg_nonce + (isLoading ? index : '') }
              item={ item }
              isLoading={ isLoading }
            />
          )) }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default OptimisticL2WithdrawalsTable;
