import React from 'react';

import type { ScrollL2MessageItem } from 'types/api/scrollL2';

import config from 'configs/app';
import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { layerLabels } from 'lib/rollups/utils';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import ScrollL2WithdrawalsTableItem from './ScrollL2WithdrawalsTableItem';

type Props = {
  items: Array<ScrollL2MessageItem>;
  top: number;
  isLoading?: boolean;
};

const ScrollL2WithdrawalsTable = ({ items, top, isLoading }: Props) => {
  return (
    <AddressHighlightProvider>
      <ActivityTable label="Withdrawals" tableLayout="auto" minW="var(--chakra-sizes-4xl)">
        <TableHeaderSticky top={ top }>
          <TableRow>
            <TableColumnHeader isNumeric>{ layerLabels.current } block</TableColumnHeader>
            <TableColumnHeader isNumeric>Index</TableColumnHeader>
            <TableColumnHeader>{ layerLabels.current } txn hash</TableColumnHeader>
            <TableColumnHeader>
              Timestamp
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader>{ layerLabels.parent } txn hash</TableColumnHeader>
            <TableColumnHeader isNumeric>{ `Value ${ config.chain.currency.symbol }` }</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { items.map((item, index) => (
            <ScrollL2WithdrawalsTableItem key={ String(item.id) + (isLoading ? index : '') } item={ item } isLoading={ isLoading }/>
          )) }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default ScrollL2WithdrawalsTable;
