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

const TokenTransferTable = ({ data, top, showSocketInfo, showSocketErrorAlert, socketInfoNum, tokenId, isLoading, token, instance }: Props) => {
  const multichainContext = useMultichainContext();
  const chainData = multichainContext?.chain;
  const tokenType = token.type;

  return (
    <AddressHighlightProvider>
      <ResponsiveTableRegion label="Token transfers" minWidth="4xl">
        <TableRoot variant="scan" minW="4xl" data-token-transfer-table>
          <TableHeaderSticky top={ top }>
            <TableRow>
              { chainData && <TableColumnHeader width="9"/> }
              <TableColumnHeader width="72">
                Txn hash
                <TimeFormatToggle/>
              </TableColumnHeader>
              <TableColumnHeader width="48">Method</TableColumnHeader>
              <TableColumnHeader width={{ lg: '56', xl: '96' }}>From/To</TableColumnHeader>
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
