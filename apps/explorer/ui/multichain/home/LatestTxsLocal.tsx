import { Box, Separator } from '@chakra-ui/react';
import React from 'react';

import type { PriceUpdateItem } from 'types/api/paxeerXFeed';
import type { Transaction } from 'types/api/transaction';

import { route } from 'nextjs/routes';

import getSocketUrl from 'lib/api/getSocketUrl';
import useApiQuery from 'lib/api/useApiQuery';
import { useMultichainContext } from 'lib/contexts/multichain';
import { SocketProvider } from 'lib/socket/context';
import { TX } from 'stubs/tx';
import { Link } from 'toolkit/chakra/link';
import LatestTxsItem from 'ui/home/LatestTxsItem';
import LatestTxsItemMobile from 'ui/home/LatestTxsItemMobile';
import DataListDisplay from 'ui/shared/DataListDisplay';

const LatestTxsLocal = () => {
  const chain = useMultichainContext()?.chain;

  const query = useApiQuery('general:homepage_txs', {
    chain,
    queryOptions: {
      placeholderData: Array<Transaction | PriceUpdateItem>(3).fill(TX),
      select: (data) => data.slice(0, 3),
    },
  });

  return (
    <SocketProvider url={ getSocketUrl(chain?.app_config) }>
      <DataListDisplay isError={ query.isError } itemsNum={ query.data?.length } emptyText="There are no transactions.">
        <Box display={{ base: 'block', lg: 'none' }}>
          { query.data?.map((tx, index) => (
            <LatestTxsItemMobile
              key={ tx.hash + (query.isPlaceholderData ? index : '') }
              tx={ tx }
              isLoading={ query.isPlaceholderData }
            />
          )) }
        </Box>
        <Box display={{ base: 'none', lg: 'block' }}>
          { query.data?.map((tx, index) => (
            <LatestTxsItem
              key={ tx.hash + (query.isPlaceholderData ? index : '') }
              tx={ tx }
              isLoading={ query.isPlaceholderData }
            />
          )) }
        </Box>
      </DataListDisplay>
      <Separator orientation="horizontal" hideFrom="lg"/>
      <Link
        href={ route({ pathname: '/txs', query: { chain_id: chain?.id, tab: 'txs_local' } }) }
        w="full"
        justifyContent="center"
        textStyle="sm"
        mt={ 3 }
      >
        View all transactions
      </Link>
    </SocketProvider>
  );
};

export default React.memo(LatestTxsLocal);
