import { Box } from '@chakra-ui/react';
import React from 'react';

import type { TokenTransfer } from 'types/api/tokenTransfer';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { useMultichainContext } from 'lib/contexts/multichain';
import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import * as SocketNewItemsNotice from 'ui/shared/SocketNewItemsNotice';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import { getTokenTransferKey } from 'ui/shared/TokenTransfer/helpers';
import TokenTransferTableItem from 'ui/shared/TokenTransfer/TokenTransferTableItem';

interface Props {
  data: Array<TokenTransfer>;
  baseAddress?: string;
  showTxInfo?: boolean;
  top: number;
  enableTimeIncrement?: boolean;
  showSocketInfo?: boolean;
  showSocketErrorAlert?: boolean;
  socketInfoNum?: number;
  isLoading?: boolean;
}

export const TokenTransferTable = ({
  data,
  baseAddress,
  showTxInfo,
  top,
  enableTimeIncrement,
  showSocketInfo,
  showSocketErrorAlert,
  socketInfoNum,
  isLoading,
}: Props) => {
  const multichainContext = useMultichainContext();
  const chainData = multichainContext?.chain;

  return (
    <AddressHighlightProvider>
      <Box minW={ 0 } containerType="inline-size" containerName="token-transfers" data-token-transfer-table>
        <Box
          role="region"
          aria-label="Token transfers"
          tabIndex={ 0 }
          overflowX="auto"
          minW={ 0 }
          maxW="100%"
          css={{
            [ `@container token-transfers (min-width: ${ chainData ? '62.5rem' : '59.375rem' })` ]: {
              overflow: 'visible',
            },
          }}
          _focusVisible={{ outline: 'var(--chakra-border-widths-2) solid', outlineColor: 'focusRing', outlineOffset: 'var(--chakra-spacing-0\\.5)' }}
        >
          <TableRoot minW={ chainData ? 'calc(var(--chakra-spacing-62\\.5) * 4)' : 'calc(var(--chakra-spacing-200) + var(--chakra-spacing-37\\.5))' }>
            <TableHeaderSticky top={ top }>
              <TableRow>
                { showTxInfo && <TableColumnHeader width={ 12 }></TableColumnHeader> }
                { chainData && <TableColumnHeader width={ showTxInfo ? 8 : 10 }/> }
                <TableColumnHeader width={ 'var(--chakra-spacing-62\\.5)' }>Token</TableColumnHeader>
                <TableColumnHeader width={ 40 }>Token ID</TableColumnHeader>
                { showTxInfo && (
                  <TableColumnHeader width="calc(var(--chakra-spacing-100) / 2)">
                    Txn hash
                    <TimeFormatToggle/>
                  </TableColumnHeader>
                ) }
                <TableColumnHeader width="60%">From/To</TableColumnHeader>
                <TableColumnHeader width="40%" isNumeric>Value</TableColumnHeader>
              </TableRow>
            </TableHeaderSticky>
            <TableBody>
              { showSocketInfo && (
                <SocketNewItemsNotice.Desktop
                  showErrorAlert={ showSocketErrorAlert }
                  num={ socketInfoNum }
                  type="token_transfer"
                  isLoading={ isLoading }
                />
              ) }
              { data.map((item, index) => (
                <TokenTransferTableItem
                  key={ getTokenTransferKey(item, isLoading ? index : undefined) }
                  { ...item }
                  baseAddress={ baseAddress }
                  showTxInfo={ showTxInfo }
                  enableTimeIncrement={ enableTimeIncrement }
                  isLoading={ isLoading }
                  chainData={ chainData }
                />
              )) }
            </TableBody>
          </TableRoot>
        </Box>
      </Box>
    </AddressHighlightProvider>
  );
};

export default React.memo(TokenTransferTable);
