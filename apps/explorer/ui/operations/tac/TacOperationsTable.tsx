import React from 'react';

import type * as tac from '@blockscout/tac-operation-lifecycle-types';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';

import TacOperationsTableItem from './TacOperationsTableItem';

type Props = {
  items: Array<tac.OperationBriefDetails>;
  isLoading?: boolean;
};

const TacOperationsTable = ({ items, isLoading }: Props) => {
  return (
    <AddressHighlightProvider>
      <TableRoot minW="calc(var(--chakra-spacing-1) * 237.5)">
        <TableHeaderSticky top={ 68 }>
          <TableRow>
            <TableColumnHeader w="calc(var(--chakra-spacing-1) * 50)">Status</TableColumnHeader>
            <TableColumnHeader w="100%">Operation</TableColumnHeader>
            <TableColumnHeader w="calc(var(--chakra-spacing-1) * 50)">
              Timestamp
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader w="calc(var(--chakra-spacing-1) * 62.5)">Sender</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { items.map((item, index) => (
            <TacOperationsTableItem key={ String(item.operation_id) + (isLoading ? index : '') } item={ item } isLoading={ isLoading }/>
          )) }
        </TableBody>
      </TableRoot>
    </AddressHighlightProvider>
  );
};

export default TacOperationsTable;
