import React from 'react';

import type { TxBlob } from 'types/api/blobs';

import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import TxBlobsTableItem from './TxBlobsTableItem';

interface Props {
  data: Array<TxBlob>;
  top: number;
  isLoading?: boolean;
}

const TxBlobsTable = ({ data, top, isLoading }: Props) => {

  return (
    <ActivityTable label="Blob data">
      <TableHeaderSticky top={ top }>
        <TableRow>
          <TableColumnHeader width="60%">Blob hash</TableColumnHeader>
          <TableColumnHeader width="20%">Data type</TableColumnHeader>
          <TableColumnHeader width="20%">Size, bytes</TableColumnHeader>
        </TableRow>
      </TableHeaderSticky>
      <TableBody>
        { data.map((item, index) => (
          <TxBlobsTableItem key={ item.hash + (isLoading ? index : '') } data={ item } isLoading={ isLoading }/>
        )) }
      </TableBody>
    </ActivityTable>
  );
};

export default TxBlobsTable;
