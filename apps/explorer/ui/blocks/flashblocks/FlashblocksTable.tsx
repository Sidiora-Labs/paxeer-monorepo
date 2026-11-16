import React from 'react';

import type { FlashblockItem } from 'types/client/flashblocks';

import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import TableScrollArea from 'ui/blocks/TableScrollArea';
import * as SocketNewItemsNotice from 'ui/shared/SocketNewItemsNotice';

import FlashblocksTableItem from './FlashblocksTableItem';

interface Props {
  items: Array<FlashblockItem>;
  newItemsNum: number | undefined;
  showAlertError?: boolean;
  onAlertLinkClick?: () => void;
}

const FlashblocksTable = ({ items, newItemsNum, showAlertError, onAlertLinkClick }: Props) => {
  return (
    <TableScrollArea label="Flashblocks">
      <TableRoot aria-label="Flashblocks">
        <TableHeaderSticky top={ 0 }>
          <TableRow>
            <TableColumnHeader width="50%">
              Block
            </TableColumnHeader>
            <TableColumnHeader width="20%" isNumeric>
              Txn
            </TableColumnHeader>
            <TableColumnHeader width="30%" isNumeric>
              Gas used
            </TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { (newItemsNum !== undefined || showAlertError) && (
            <SocketNewItemsNotice.Desktop
              type="flashblock"
              num={ newItemsNum }
              showErrorAlert={ showAlertError }
              onLinkClick={ onAlertLinkClick }
            />
          ) }
          { items.map((item) => (
            <FlashblocksTableItem
              key={ `${ item.block_number }-${ item.index }` }
              data={ item }
            />
          )) }
        </TableBody>
      </TableRoot>
    </TableScrollArea>
  );
};

export default React.memo(FlashblocksTable);
