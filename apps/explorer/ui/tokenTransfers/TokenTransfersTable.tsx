import React from 'react';

import type { TokenTransfer } from 'types/api/tokenTransfer';
import type { ClusterChainConfig } from 'types/multichain';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { TableBody, TableColumnHeader, TableRow } from 'toolkit/chakra/table';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import TokenTransferTableItem from 'ui/tokenTransfers/TokenTransfersTableItem';
import ActivityTable, { ActivityTableHeader as TableHeaderSticky } from 'ui/txs/ActivityTable';

interface Props {
  items?: Array<TokenTransfer>;
  top: number;
  isLoading?: boolean;
  chainData?: ClusterChainConfig;
}

const TokenTransferTable = ({ items, top, isLoading, chainData }: Props) => {
  return (
    <AddressHighlightProvider>
      <ActivityTable label="Token transfers" minW="var(--chakra-sizes-4xl)" tableLayout="auto">
        <TableHeaderSticky top={ top }>
          <TableRow>
            { chainData && <TableColumnHeader width="var(--chakra-spacing-9)"/> }
            <TableColumnHeader>
              Txn hash
              <TimeFormatToggle/>
            </TableColumnHeader>
            <TableColumnHeader>Method</TableColumnHeader>
            <TableColumnHeader isNumeric>Block</TableColumnHeader>
            <TableColumnHeader>From/To</TableColumnHeader>
            <TableColumnHeader>Token ID</TableColumnHeader>
            <TableColumnHeader isNumeric>Amount</TableColumnHeader>
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { items?.map((item, index) => (
            <TokenTransferTableItem
              key={ item.transaction_hash + item.log_index + (isLoading ? index : '') + (chainData ? chainData.id : '') }
              item={ item }
              isLoading={ isLoading }
              chainData={ chainData }
            />
          )) }
        </TableBody>
      </ActivityTable>
    </AddressHighlightProvider>
  );
};

export default React.memo(TokenTransferTable);
