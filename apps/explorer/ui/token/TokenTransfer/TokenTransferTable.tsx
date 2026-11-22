import React from 'react';

import type { TokenInfo, TokenInstance } from 'types/api/token';
import type { TokenTransfer } from 'types/api/tokenTransfer';

import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { useMultichainContext } from 'lib/contexts/multichain';
import { hasTokenIds, hasTokenTransferValue, isConfidentialTokenType, isFungibleTokenType, NFT_TOKEN_TYPE_IDS } from 'lib/token/tokenTypes';
import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import { TruncatedText } from 'toolkit/components/truncation/TruncatedText';
import * as SocketNewItemsNotice from 'ui/shared/SocketNewItemsNotice';
import TimeFormatToggle from 'ui/shared/time/TimeFormatToggle';
import { ResponsiveTableRegion } from 'ui/token/ResponsiveTableRegion';
import TokenTransferTableItem from 'ui/token/TokenTransfer/TokenTransferTableItem';

interface Props {
  data: Array<TokenTransfer>;
  top: number;
  showSocketInfo: boolean;
  showSocketErrorAlert?: boolean;
  socketInfoNum?: number;
  tokenId?: string;
  isLoading?: boolean;
  token: TokenInfo;
  instance?: TokenInstance;
}

const TABLE_MIN_WIDTH = 'calc(var(--chakra-spacing-200) + var(--chakra-spacing-37\\.5))';

const TokenTransferTable = ({ data, top, showSocketInfo, showSocketErrorAlert, socketInfoNum, tokenId, isLoading, token, instance }: Props) => {
  const multichainContext = useMultichainContext();
  const chainData = multichainContext?.chain;
  const tokenType = token.type;

  return (
    <AddressHighlightProvider>
      <ResponsiveTableRegion label="Token transfers" minWidth={ TABLE_MIN_WIDTH }>
        <TableRoot variant="scan" minW={ TABLE_MIN_WIDTH } data-token-transfer-table>
          <TableHeaderSticky top={ top }>
            <TableRow>
              { chainData && <TableColumnHeader width={ 'calc(var(--chakra-spacing-9) + var(--chakra-spacing-0\\.5))' }/> }
              <TableColumnHeader width="calc(var(--chakra-spacing-65) + var(--chakra-spacing-5))">
                Txn hash
                <TimeFormatToggle/>
              </TableColumnHeader>
              <TableColumnHeader width="calc(var(--chakra-spacing-40) + var(--chakra-spacing-10))">Method</TableColumnHeader>
              <TableColumnHeader width={{
                lg: 'calc(var(--chakra-spacing-40) + var(--chakra-spacing-15))',
                xl: 'calc(var(--chakra-spacing-100) - var(--chakra-spacing-5))',
              }}>From/To</TableColumnHeader>
              { (NFT_TOKEN_TYPE_IDS.includes(tokenType)) &&
              <TableColumnHeader width={ hasTokenIds(tokenType) ? '50%' : '100%' }>Token ID</TableColumnHeader>
              }
              { hasTokenTransferValue(tokenType) && (
                <TableColumnHeader width={ (isFungibleTokenType(tokenType) || isConfidentialTokenType(tokenType)) ? '100%' : '50%' } isNumeric>
                  <TruncatedText text={ `Value ${ token?.symbol || '' }` } w="100%" verticalAlign="middle"/>
                </TableColumnHeader>
              ) }
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
                key={ item.transaction_hash + item.block_hash + item.log_index + '_' + index }
                { ...item }
                tokenId={ tokenId }
                instance={ instance }
                isLoading={ isLoading }
                chainData={ chainData }
              />
            )) }
          </TableBody>
        </TableRoot>
      </ResponsiveTableRegion>
    </AddressHighlightProvider>
  );
};

export default React.memo(TokenTransferTable);
