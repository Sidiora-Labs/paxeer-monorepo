import { Box } from '@chakra-ui/react';
import { clamp } from 'es-toolkit';
import React from 'react';

import { route } from 'nextjs-routes';

import config from 'configs/app';
import { AddressHighlightProvider } from 'lib/contexts/addressHighlight';
import { publicClient } from 'lib/web3/client';
import { TX } from 'stubs/tx';
import { TableBody } from 'toolkit/chakra/table';

import HomeTable from '../HomeTable';
import LatestTxsItem from '../LatestTxsItem';
import LatestTxsItemMobile from '../LatestTxsItemMobile';
import LatestTxsDegradedNewItems from './LatestTxsDegradedNewItems';
import LatestTxsFallback from './LatestTxsFallback';
import { useHomeRpcDataContext } from './rpcDataContext';

const zetachainFeature = config.features.zetachain;

interface Props {
  maxNum: number;
}

const LatestTxsDegraded = ({ maxNum }: Props) => {
  const { txs, totalTxs, isError, isLoading, enable } = useHomeRpcDataContext();

  React.useEffect(() => {
    enable(true, 'latest-txs');
    return () => {
      enable(false, 'latest-txs');
    };
  }, [ enable ]);

  if (isError || !publicClient) {
    return <LatestTxsFallback/>;
  }

  const items = isLoading ? Array(maxNum).fill(TX) : txs.slice(0, maxNum);

  if (items.length === 0) {
    return <Box textStyle="sm">No latest transactions found.</Box>;
  }

  const txsUrl = route({ pathname: `/txs`, query: zetachainFeature.isEnabled ? { tab: 'evm' } : undefined });
  const overflow = clamp(totalTxs - maxNum, 0, Infinity);

  return (
    <>
      <LatestTxsDegradedNewItems overflow={ overflow } url={ txsUrl } isLoading={ isLoading }/>
      <Box mb={ 3 } display={{ base: 'block', lg: 'none' }} textStyle="sm">
        { items.map(((tx, index) => (
          <LatestTxsItemMobile
            key={ tx.hash + (isLoading ? index : '') }
            tx={ tx }
            isLoading={ isLoading }
          />
        ))) }
      </Box>
      <AddressHighlightProvider>
        <Box mb={ 3 } display={{ base: 'none', lg: 'block' }} textStyle="sm">
          <HomeTable
            label="Latest transactions"
            columns={ [ { label: 'Transaction', width: '36%' }, { label: 'From / To' }, { label: 'Value / Fee', numeric: true, width: '28%' } ] }
          >
            <TableBody>
              { items.map(((tx, index) => (
                <LatestTxsItem
                  key={ tx.hash + (isLoading ? index : '') }
                  tx={ tx }
                  isLoading={ isLoading }
                />
              ))) }
            </TableBody>
          </HomeTable>
        </Box>
      </AddressHighlightProvider>

    </>
  );
};

export default React.memo(LatestTxsDegraded);
