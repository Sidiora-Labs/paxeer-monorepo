import React from 'react';

import type { TxStateChange } from 'types/api/txStateChanges';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TxStateTableItem from 'ui/tx/state/TxStateTableItem';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

interface Props {
  data: Array<TxStateChange>;
  isLoading?: boolean;
  top: number;
}

const TxStateTable = ({ data, isLoading, top }: Props) => {
  return (
    <AddressHighlightProvider>
      <ActivityTable label="State changes" minWidth="var(--chakra-sizes-5xl)" w="100%">
        <TableHeaderSticky top={ top }>
          <TableRow>
            <TableColumnHeader width="var(--chakra-spacing-36)">Type</TableColumnHeader>
            <TableColumnHeader width="var(--chakra-spacing-40)">Address</TableColumnHeader>
            <TableColumnHeader width="33%" isNumeric>Before</TableColumnHeader>
            <TableColumnHeader width="33%" isNumeric>After</TableColumnHeader>
            <TableColumnHeader width="33%" isNumeric>Change</TableColumnHeader>
            <TableColumnHeader width="var(--chakra-spacing-37\.5)" minW="var(--chakra-spacing-20)" maxW="var(--chakra-spacing-37\.5)">
              Token ID
            </TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { data.map((item, index) => <TxStateTableItem data={ item } key={ index } isLoading={ isLoading }/>) }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default React.memo(TxStateTable);
