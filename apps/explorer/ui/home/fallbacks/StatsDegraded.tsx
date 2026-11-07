import { Grid } from '@chakra-ui/react';
import { useQuery } from '@tanstack/react-query';
import BigNumber from 'bignumber.js';
import React from 'react';

import dayjs from 'lib/date/dayjs';
import { publicClient } from 'lib/web3/client';
import FallbackRpcIcon from 'ui/shared/fallbacks/FallbackRpcIcon';
import StatsWidget from 'ui/shared/stats/StatsWidget';
import { GWEI } from 'ui/shared/value/utils';

import { useHomeRpcDataContext } from './rpcDataContext';

const StatsDegraded = () => {
  const { blocks, isLoading, enable } = useHomeRpcDataContext();
  const gasPriceQuery = useQuery({
    queryKey: [ 'RPC', 'gas-price' ],
    queryFn: async() => publicClient ? publicClient.getGasPrice() : null,
    enabled: Boolean(publicClient),
  });

  React.useEffect(() => {
    enable(true, 'stats-widgets');
    return () => enable(false, 'stats-widgets');
  }, [ enable ]);

  const durations = blocks.slice(1).flatMap((block, index) => {
    if (blocks[index].height !== block.height + 1) return [];
    const duration = dayjs(blocks[index].timestamp).diff(dayjs(block.timestamp), 'millisecond') / 1000;
    return duration >= 0 ? [ duration ] : [];
  });
  const averageBlockTime = durations.length > 0 ? durations.reduce((sum, duration) => sum + duration, 0) / durations.length : undefined;
  const gasPrice = gasPriceQuery.data != null ? `${ new BigNumber(gasPriceQuery.data.toString()).div(GWEI).toFormat() } Gwei` : undefined;
  const items = [
    { label: 'Total blocks', value: undefined },
    { label: 'Average block time', value: averageBlockTime != null ? `${ averageBlockTime.toFixed(1) }s` : undefined },
    { label: 'Total transactions', value: undefined },
    { label: 'Wallet addresses', value: undefined },
    { label: 'Gas price', value: gasPrice, isLoading: gasPriceQuery.isLoading },
  ];

  return (
    <Grid data-label="home-stat-tiles" templateColumns={{ base: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(5, minmax(0, 1fr))' }} gap={ 4 }>
      { items.map((item) => (
        <StatsWidget
          key={ item.label }
          { ...item }
          isFallback={ item.value === undefined }
          isLoading={ isLoading || item.isLoading }
          hint={ item.value !== undefined && !isLoading ? <FallbackRpcIcon/> : undefined }
        />
      )) }
    </Grid>
  );
};

export default React.memo(StatsDegraded);
