import React from 'react';

import type { InternalTransaction } from 'types/api/internalTransaction';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { currencyUnits } from 'lib/units';
import { TableBody, TableColumnHeader, TableColumnHeaderSortable, TableRow } from 'toolkit/chakra/table';
import TxInternalsTableItem from 'ui/tx/internals/TxInternalsTableItem';
import type { Sort, SortField } from 'ui/tx/internals/utils';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

interface Props {
  data: Array<InternalTransaction>;
  sort: Sort;
  onSortToggle: (field: SortField) => void;
  top: number;
  isLoading?: boolean;
}

const TxInternalsTable = ({ data, sort, onSortToggle, top, isLoading }: Props) => {
  return (
    <AddressHighlightProvider>
      <ActivityTable label="Internal transactions">
        <TableHeaderSticky top={ top }>
          <TableRow>
            <TableColumnHeader width="28%">Type</TableColumnHeader>
            <TableColumnHeader width="40%">From/To</TableColumnHeader>
            <TableColumnHeaderSortable
              width="16%"
              isNumeric
              sortField="value"
              sortValue={ sort }
              onSortToggle={ onSortToggle }
            >
              Value { currencyUnits.ether }
            </TableColumnHeaderSortable>
            <TableColumnHeaderSortable
              width="16%"
              isNumeric
              sortField="gas-limit"
              sortValue={ sort }
              onSortToggle={ onSortToggle }
            >
              Gas limit
            </TableColumnHeaderSortable>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { data.map((item, index) => (
            <TxInternalsTableItem key={ item.index.toString() + (isLoading ? index : '') } { ...item } isLoading={ isLoading }/>
          )) }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default TxInternalsTable;
