import { Box, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import { route } from 'nextjs-routes';

import useApiQuery from 'lib/api/useApiQuery';
import useIsMobile from 'lib/hooks/useIsMobile';
import { TX } from 'stubs/tx';
import { Link } from 'toolkit/chakra/link';
import { TableBody } from 'toolkit/chakra/table';
import useRedirectForInvalidAuthToken from 'ui/snippets/auth/useRedirectForInvalidAuthToken';

import LatestTxsFallback from './fallbacks/LatestTxsFallback';
import HomeTable from './HomeTable';
import LatestTxsItem from './LatestTxsItem';
import LatestTxsItemMobile from './LatestTxsItemMobile';

const LatestWatchlistTxs = () => {
  useRedirectForInvalidAuthToken();
  const isMobile = useIsMobile();
  const txsCount = isMobile ? 2 : 5;
  const { data, isPlaceholderData, isError } = useApiQuery('general:homepage_txs_watchlist', {
    queryOptions: {
      placeholderData: Array(txsCount).fill(TX),
    },
  });

  if (isError) {
    return <LatestTxsFallback/>;
  }

  if (!data?.length) {
    return <Text>No latest transactions found.</Text>;
  }

  if (data) {
    const txsUrl = route({ pathname: '/txs', query: { tab: 'watchlist' } });
    return (
      <>
        <Box mb={ 3 } display={{ base: 'block', lg: 'none' }} textStyle="sm">
          { data.slice(0, txsCount).map(((tx, index) => (
            <LatestTxsItemMobile
              key={ tx.hash + (isPlaceholderData ? index : '') }
              tx={ tx }
              isLoading={ isPlaceholderData }
            />
          ))) }
        </Box>
        <Box mb={ 4 } display={{ base: 'none', lg: 'block' }} textStyle="sm">
          <HomeTable
            label="Latest transactions"
            columns={ [ { label: 'Transaction', width: '36%' }, { label: 'From / To' }, { label: 'Value / Fee', numeric: true, width: '28%' } ] }
          >
            <TableBody>
              { data.slice(0, txsCount).map(((tx, index) => (
                <LatestTxsItem
                  key={ tx.hash + (isPlaceholderData ? index : '') }
                  tx={ tx }
                  isLoading={ isPlaceholderData }
                />
              ))) }
            </TableBody>
          </HomeTable>
        </Box>
        <Flex justifyContent="center">
          <Link textStyle="sm" href={ txsUrl }>View all watch list transactions</Link>
        </Flex>
      </>
    );
  }

  return null;
};

export default LatestWatchlistTxs;
