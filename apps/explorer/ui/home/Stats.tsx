import { Box, Grid } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import type { ChainIndicatorId } from 'types/homepage';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import { HOMEPAGE_STATS, HOMEPAGE_STATS_MICROSERVICE } from 'stubs/stats';
import StatsWidget from 'ui/shared/stats/StatsWidget';
import useStatsService from 'ui/stats/useStatsService';

import StatsDegraded from './fallbacks/StatsDegraded';
import ChainIndicatorsChart from './indicators/ChainIndicatorsChart';
import useChartDataQuery from './indicators/useChartDataQuery';

const HISTORY_DAYS = 14;

function formatCount(value: string | number | null | undefined) {
  if (value == null || value === '') return undefined;
  const number = new BigNumber(value);
  return number.isFinite() ? number.toFormat(0) : undefined;
}

const StatsHistory = React.memo(function StatsHistory({ indicatorId, title }: { indicatorId: ChainIndicatorId; title: string }) {
  const chartQuery = useChartDataQuery(indicatorId, HISTORY_DAYS);
  return (
    <ChainIndicatorsChart
      isLoading={ false }
      title={ title }
      chartQuery={ chartQuery }
    />
  );
});

const Stats = () => {
  const hasStatsService = useStatsService();
  const statsQuery = useApiQuery('stats:pages_main', {
    queryOptions: {
      refetchOnMount: false,
      placeholderData: hasStatsService ? HOMEPAGE_STATS_MICROSERVICE : undefined,
      enabled: hasStatsService,
    },
  });
  const apiQuery = useApiQuery('general:stats', {
    queryOptions: { refetchOnMount: false, placeholderData: HOMEPAGE_STATS },
  });
  const apiData = apiQuery.data;
  const statsData = hasStatsService && !statsQuery.isError ? statsQuery.data : undefined;
  const isLoading = apiQuery.isPlaceholderData || apiQuery.isPending ||
    (hasStatsService && (statsQuery.isPlaceholderData || statsQuery.isPending));
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
        <Grid data-label="home-stat-tiles" templateColumns="repeat(auto-fit, minmax(min(100%, var(--chakra-spacing-40)), 1fr))" gap={ 4 }>
          { items.map(({ id, ...item }) => (
            <Box key={ id } data-stat={ id } minW={ 0 } overflowWrap="anywhere">
              <StatsWidget
                { ...item }
                w="full"
                h="full"
                minW={ 0 }
                css={{ '& > div, & > div > div': { minWidth: 0 } }}
                isLoading={ isLoading }
                isFallback={ item.value === undefined }
              />
            </Box>
          )) }
        </Grid>
      ) }
      <Grid mt={ 8 } autoRows="1fr" templateColumns="repeat(auto-fit, minmax(min(100%, var(--chakra-spacing-80)), 1fr))" gap={ 4 }>
        { [
          { id: 'daily_txs' as const, title: `${ config.chain.name } transaction history in ${ HISTORY_DAYS } days` },
          { id: 'coin_price' as const, title: `${ config.chain.currency.symbol } price` },
          { id: 'market_cap' as const, title: `${ config.chain.currency.symbol } market value` },
          { id: 'secondary_coin_price' as const, title: `${ config.chain.secondaryCoin.symbol } price` },
          { id: 'secondary_coin_market_cap' as const, title: `${ config.chain.secondaryCoin.symbol } market value` },
        ].filter(({ id }) => id === 'daily_txs' || config.UI.homepage.charts.includes(id)).map(({ id, title }) => (
          <Box
            key={ id }
            data-label={ id === 'daily_txs' ? 'home-stats-history' : 'home-market-history' }
            data-indicator={ id }
            minW={ 0 }
            p={ 4 }
            bg="bg.surface"
            borderWidth="1"
            borderColor="border.divider"
            borderRadius="card"
          >
            <StatsHistory indicatorId={ id } title={ title }/>
          </Box>
        )) }
      </Grid>
    </Box>
  );
};

export default Stats;
