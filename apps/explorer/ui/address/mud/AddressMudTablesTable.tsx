import React from 'react';

import type { AddressMudTables } from 'types/api/address';

import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';

import AddressMudTablesTableItem from './AddressMudTablesTableItem';

type Props = {
  items: AddressMudTables['items'];
  isLoading: boolean;
  top: number;
  hash: string;
};

//sorry for the naming
const AddressMudTablesTable = ({ items, isLoading, top, hash }: Props) => {
  return (
    <TableRoot tableLayout="auto" aria-label="MUD tables" aria-busy={ isLoading }>
      <TableHeaderSticky top={ top }>
        <TableRow>
          <TableColumnHeader w={ 6 } aria-label="Expand table"></TableColumnHeader>
          <TableColumnHeader>Full name</TableColumnHeader>
          <TableColumnHeader>Table ID</TableColumnHeader>
          <TableColumnHeader>Type</TableColumnHeader>
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { items.map((item, index) => (
          <AddressMudTablesTableItem
            key={ item.table.table_id + (isLoading ? String(index) : '') }
            item={ item }
            isLoading={ isLoading }
            hash={ hash }
          />
        )) }
      </TableBody>
    </TableRoot>
  );
};

export default AddressMudTablesTable;
