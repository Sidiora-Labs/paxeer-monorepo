import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import type { TokenTransfer } from 'types/api/tokenTransfer';

import { route } from 'nextjs-routes';

import { Link } from 'toolkit/chakra/link';
import breakpoints from 'toolkit/theme/foundations/breakpoints';
import IconSvg from 'ui/shared/IconSvg';
import TokenTransferSnippet from 'ui/shared/TokenTransferSnippet/TokenTransferSnippet';
import * as DetailedInfo from 'ui/tx/ActivityDetails';
interface Props {
  data: Array<TokenTransfer>;
  txHash: string;
  isOverflow: boolean;
}

const TOKEN_TRANSFERS_TYPES = [
  { title: 'Tokens transferred', hint: 'List of tokens transferred in the transaction', type: 'token_transfer' },
  { title: 'Tokens minted', hint: 'List of tokens minted in the transaction', type: 'token_minting' },
  { title: 'Tokens burnt', hint: 'List of tokens burnt in the transaction', type: 'token_burning' },
  { title: 'Tokens created', hint: 'List of tokens created in the transaction', type: 'token_spawning' },
];

const TxDetailsTokenTransfers = ({ data, txHash, isOverflow }: Props) => {
  const viewAllUrl = route({ pathname: '/tx/[hash]', query: { hash: txHash, tab: 'token_transfers' } });

  const transferGroups = TOKEN_TRANSFERS_TYPES.map((group) => ({
    ...group,
    items: data?.filter((token) => token.type === group.type) || [],
  }));

  return (
    <>
      { transferGroups.map(({ title, hint, type, items }) => {
        if (items.length === 0) {
          return null;
        }

        return (
          <React.Fragment key={ type }>
            <DetailedInfo.ItemLabel
              hint={ hint }
            >
              { title }
            </DetailedInfo.ItemLabel>
            <DetailedInfo.ItemValue position="relative" multiRow>
              <Flex
                flexDirection="column"
                alignItems="flex-start"
                rowGap={ 1 }
                w="100%"
              >
                { items.map((item, index) => (
                  <Box
                    key={ index }
                    w="full"
                    minW={ 0 }
                    css={{
                      '& > div > div:first-child': {
                        minW: 0,
                        maxW: 'full',
                        gridTemplateColumns: 'minmax(0, 1fr)',
                        rowGap: 2,
                        '& > :last-child': { ml: 0 },
                      },
                      [`@container activity-details (min-width: ${ breakpoints.sm })`]: {
                        '& > div > div:first-child': {
                          gridTemplateColumns: 'minmax(0, 1fr) auto minmax(0, 1fr)',
                          '& > :last-child': { ml: 3 },
                        },
                      },
                    }}
                  >
                    <TokenTransferSnippet data={ item }/>
                  </Box>
                )) }
              </Flex>
              { isOverflow && (
                <>
                  { /* FIXME use non-navigation icon */ }
                  <IconSvg name="navigation/tokens" boxSize={ 6 }/>
                  <Link href={ viewAllUrl }>
                    View all
                  </Link>
                </>
              ) }
            </DetailedInfo.ItemValue>
          </React.Fragment>
        );
      }) }
    </>
  );
};

export default React.memo(TxDetailsTokenTransfers);
