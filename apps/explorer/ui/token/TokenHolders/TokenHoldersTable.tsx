import React from 'react';

import type { TokenHolder, TokenInfo } from 'types/api/token';

import { hasTokenIds, isConfidentialTokenType } from 'lib/token/tokenTypes';
import { TableBody, TableColumnHeader, TableHeaderSticky, TableRoot, TableRow } from 'toolkit/chakra/table';
import { ResponsiveTableRegion } from 'ui/token/ResponsiveTableRegion';
import TokenHoldersTableItem from 'ui/token/TokenHolders/TokenHoldersTableItem';

interface Props {
  data: Array<TokenHolder>;
  token: TokenInfo;
  top: number;
  isLoading?: boolean;
}

const TokenHoldersTable = ({ data, token, top, isLoading }: Props) => {
  return (
    <ResponsiveTableRegion label="Token holders" minWidth="2xl">
      <TableRoot variant="scan" data-token-holders-table>
        <TableHeaderSticky top={ top }>
          <TableRow>
            <TableColumnHeader w="70%">Holder</TableColumnHeader>
            { (hasTokenIds(token.type)) && <TableColumnHeader w="30%">ID#</TableColumnHeader> }
            <TableColumnHeader isNumeric width="56">Quantity</TableColumnHeader>
            { token.total_supply && token.type !== 'ERC-404' && !isConfidentialTokenType(token.type) && (
              <TableColumnHeader isNumeric width="44">Percentage</TableColumnHeader>
            ) }
          </TableRow>
        </TableHeaderSticky>
        <TableBody>
          { data.map((item, index) => {
            const tokenId = 'token_id' in item ? item.token_id : null;
            return (
              <TokenHoldersTableItem key={ item.address.hash + tokenId + (isLoading ? index : '') } holder={ item } token={ token } isLoading={ isLoading }/>
            );
          }) }
        </TableBody>
      </TableRoot>
    </ResponsiveTableRegion>
  );
};

export default React.memo(TokenHoldersTable);
