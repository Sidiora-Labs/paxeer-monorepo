import React from 'react';

import type { TxAuthorization } from 'types/api/transaction';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import TxAuthorizationsTableItem from './TxAuthorizationsTableItem';

interface Props {
  data: Array<TxAuthorization> | undefined;
  isLoading?: boolean;
}

const TxAuthorizationsTable = ({ data, isLoading }: Props) => {
  return (
    <AddressHighlightProvider>
      <ActivityTable label="Authorizations">
        <TableHeaderSticky>
          <TableRow>
            <TableColumnHeader width="50%">Authority</TableColumnHeader>
            <TableColumnHeader width="50%">Delegated address</TableColumnHeader>
            <TableColumnHeader width="var(--chakra-spacing-30)">Chain</TableColumnHeader>
            <TableColumnHeader width="var(--chakra-spacing-30)" isNumeric>Nonce</TableColumnHeader>
            <TableColumnHeader width="var(--chakra-spacing-48)">Status</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { data?.map((item, index) => (
            <TxAuthorizationsTableItem key={ item.nonce.toString() + (isLoading ? index : '') } { ...item } isLoading={ isLoading }/>
          )) }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default TxAuthorizationsTable;
