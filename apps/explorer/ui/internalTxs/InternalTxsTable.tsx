import React from 'react';

import type { InternalTransaction } from 'types/api/internalTransaction';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { useMultichainContext } from 'lib/contexts/multichain';
import { currencyUnits } from 'lib/units';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import InternalTxsTableItem from './InternalTxsTableItem';

interface Props {
  data: Array<InternalTransaction>;
  currentAddress?: string;
  isLoading?: boolean;
  top?: number;
  showBlockInfo?: boolean;
}

const InternalTxsTable = ({ data, currentAddress, isLoading, top, showBlockInfo = true }: Props) => {
  const multichainContext = useMultichainContext();
  const chainData = multichainContext?.chain;

  return (
    <AddressHighlightProvider>
      <ActivityTable label="Internal transactions" minW="var(--chakra-sizes-4xl)">
        <TableHeaderSticky top={ top ?? 68 }>
          <TableRow>
            { chainData && <TableColumnHeader width="var(--chakra-spacing-9)"></TableColumnHeader> }
            <TableColumnHeader width="var(--chakra-spacing-72)">
              Parent txn hash
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader width="15%">Type</TableColumnHeader>
            { showBlockInfo && <TableColumnHeader width="15%" isNumeric>Block</TableColumnHeader> }
            <TableColumnHeader width="50%">From/To</TableColumnHeader>
            <TableColumnHeader width="20%" isNumeric>
              Value { currencyUnits.ether }
            </TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { data.map((item, index) => (
            <InternalTxsTableItem
              key={ item.transaction_hash + '_' + index }
              { ...item }
              currentAddress={ currentAddress }
              isLoading={ isLoading }
              showBlockInfo={ showBlockInfo }
              chainData={ chainData }
            />
          )) }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>

  );
};

export default InternalTxsTable;
