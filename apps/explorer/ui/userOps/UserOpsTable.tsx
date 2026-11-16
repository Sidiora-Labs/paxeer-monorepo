import React from 'react';

import type { UserOpsItem } from 'types/api/userOps';

import config from 'configs/app';
import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { useMultichainContext } from 'lib/contexts/multichain';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

import UserOpsTableItem from './UserOpsTableItem';

type Props = {
  items: Array<UserOpsItem>;
  isLoading?: boolean;
  top: number;
  showTx: boolean;
  showSender: boolean;
};

const UserOpsTable = ({ items, isLoading, top, showTx, showSender }: Props) => {
  const multichainContext = useMultichainContext();
  const chainData = multichainContext?.chain;
  const chainConfig = (multichainContext?.chain.app_config || config);

  return (
    <AddressHighlightProvider>
      <ActivityTable label="User operations" minW="var(--chakra-sizes-5xl)">
        <TableHeaderSticky top={ top }>
          <TableRow>
            { chainData && <TableColumnHeader width="var(--chakra-spacing-9)"></TableColumnHeader> }
            <TableColumnHeader w="60%">User op hash</TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-44)">
              Timestamp
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader w="var(--chakra-spacing-36)">Status</TableColumnHeader>
            { showSender && <TableColumnHeader w="var(--chakra-spacing-40)">Sender</TableColumnHeader> }
            { showTx && <TableColumnHeader w="var(--chakra-spacing-40)">Tx hash</TableColumnHeader> }
            <TableColumnHeader w="40%" isNumeric>Block</TableColumnHeader>
            { !chainConfig.UI.views.tx.hiddenFields?.tx_fee &&
          <TableColumnHeader w="var(--chakra-spacing-30)" isNumeric>{ `Fee ${ chainConfig.chain.currency.symbol }` }</TableColumnHeader> }
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { items.map((item, index) => {
            return (
              <UserOpsTableItem
                key={ item.hash + (isLoading ? String(index) : '') }
                item={ item }
                isLoading={ isLoading }
                showSender={ showSender }
                showTx={ showTx }
                chainData={ chainData }
              />
            );
          }) }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default UserOpsTable;
