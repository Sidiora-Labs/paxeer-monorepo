import { Box, Grid } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import { HOMEPAGE_STATS, HOMEPAGE_STATS_MICROSERVICE } from 'stubs/stats';
import StatsWidget from 'ui/shared/stats/StatsWidget';

import StatsDegraded from './fallbacks/StatsDegraded';
import ChainIndicatorsChart from './indicators/ChainIndicatorsChart';
import useChartDataQuery from './indicators/useChartDataQuery';

const HISTORY_DAYS = 14;

function formatCount(value: string | number | null | undefined) {
  if (value == null || value === '') return undefined;
  const number = new BigNumber(value);
  return number.isFinite() ? number.toFormat(0) : undefined;
}

const StatsHistory = React.memo(function StatsHistory() {
  const chartQuery = useChartDataQuery('daily_txs');
  return (
    <ChainIndicatorsChart
      isLoading={ false }
      title={ `${ config.chain.name } transaction history in ${ HISTORY_DAYS } days` }
      chartQuery={ chartQuery }
      days={ HISTORY_DAYS }
    />
  );
});

const Stats = () => {
  const statsQuery = useApiQuery('stats:pages_main', {
    queryOptions: {
      refetchOnMount: false,
      placeholderData: config.features.stats.isEnabled ? HOMEPAGE_STATS_MICROSERVICE : undefined,
      enabled: config.features.stats.isEnabled,
    },
  });
  const apiQuery = useApiQuery('general:stats', {
    queryOptions: { refetchOnMount: false, placeholderData: HOMEPAGE_STATS },
  });
  const apiData = apiQuery.isPlaceholderData ? undefined : apiQuery.data;
  const statsData = config.features.stats.isEnabled && !statsQuery.isPlaceholderData && !statsQuery.isError ? statsQuery.data : undefined;
  const isLoading = apiQuery.isPlaceholderData || apiQuery.isPending ||
    (config.features.stats.isEnabled && (statsQuery.isPlaceholderData || statsQuery.isPending));
  const rawBlockTime = statsData?.average_block_time?.value;
  let blockTime = apiData?.average_block_time != null ? apiData.average_block_time / 1000 : undefined;
  if (rawBlockTime != null && rawBlockTime !== '') {
    blockTime = Number(rawBlockTime);
  }
  const gasPrice = apiData?.gas_prices?.average?.price;
  const items = [
    {
      id: 'total_blocks',
      label: 'Total blocks',
      value: formatCount(statsData?.total_blocks?.value ?? apiData?.total_blocks),
      href: { pathname: '/blocks' as const },
    },
    {
      id: 'average_block_time',
      label: 'Average block time',
      value: blockTime != null && Number.isFinite(blockTime) ? `${ blockTime.toFixed(1) }s` : undefined,
    },
    {
      id: 'total_txs',
      label: 'Total transactions',
      value: formatCount(statsData?.total_transactions?.value ?? apiData?.total_transactions),
      href: { pathname: '/txs' as const },
    },
    {
      id: 'wallet_addresses',
      label: 'Wallet addresses',
      value: formatCount(statsData?.total_addresses?.value ?? apiData?.total_addresses),
      href: { pathname: '/accounts' as const },
    },
    {
      id: 'gas_tracker',
      label: 'Gas price',
      value: gasPrice != null && Number.isFinite(gasPrice) ? `${ new BigNumber(gasPrice).toFormat() } Gwei` : undefined,
    },
  ];

  return (
    <Box as="section" aria-label="Network statistics" data-label="home-stats">
      { apiQuery.isError && !statsData ? <StatsDegraded/> : (
        <Grid data-label="home-stat-tiles" templateColumns={{ base: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(5, minmax(0, 1fr))' }} gap={ 4 }>
          { items.map(({ id, ...item }) => (
            <Box key={ id } data-stat={ id }><StatsWidget { ...item } isLoading={ isLoading } isFallback={ item.value === undefined }/></Box>
          )) }
        </Grid>
      ) }
      <Box data-label="home-stats-history" mt={ 8 } p={ 4 } bg="bg.surface" borderWidth="1px" borderColor="border.divider" borderRadius="8px">
        <StatsHistory/>
      </Box>
    </Box>
  );
};

export default Stats;
