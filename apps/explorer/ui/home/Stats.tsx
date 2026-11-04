import { Box, Grid, GridItem } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import type { HomeStatsWidgetId } from 'types/homepage';

import config from 'configs/app';
import useApiQuery from 'lib/api/useApiQuery';
import { BLOCK } from 'stubs/block';
import { HOMEPAGE_STATS, HOMEPAGE_STATS_MICROSERVICE } from 'stubs/stats';
import GasPrice from 'ui/shared/gas/GasPrice';
import IconSvg from 'ui/shared/IconSvg';
import NativeTokenIcon from 'ui/shared/NativeTokenIcon';

import StatsDegraded from './fallbacks/StatsDegraded';
import Highlights from './Highlights';
import type { HighlightsItemProps } from './highlights/HighlightsItem';
import ChainIndicatorsChart from './indicators/ChainIndicatorsChart';
import useChartDataQuery from './indicators/useChartDataQuery';
import { isHomeStatsItemEnabled } from './utils';

const isStatsFeatureEnabled = config.features.stats.isEnabled;

const HISTORY_DAYS = 14;
const SECONDS_PER_DAY = 24 * 60 * 60;

// The latest-block counter follows the cadence the live block list flushes on, so the history column
// carries its own query behind a memo boundary and the chart is left alone by every flush that only
// moves the counters beside it.
const StatsHistory = React.memo(function StatsHistory({ isLoading }: { isLoading: boolean }) {
  const chartQuery = useChartDataQuery('daily_txs');

  return (
    <ChainIndicatorsChart
      isLoading={ isLoading }
      title={ `${ config.chain.name } transaction history in ${ HISTORY_DAYS } days` }
      chartQuery={ chartQuery }
      days={ HISTORY_DAYS }
    />
  );
});

const Stats = () => {
  // data from stats microservice is prioritized over data from stats api
  const statsQuery = useApiQuery('stats:pages_main', {
    queryOptions: {
      refetchOnMount: false,
      placeholderData: isStatsFeatureEnabled ? HOMEPAGE_STATS_MICROSERVICE : undefined,
      enabled: isStatsFeatureEnabled,
    },
  });

  const apiQuery = useApiQuery('general:stats', {
    queryOptions: {
      refetchOnMount: false,
      placeholderData: HOMEPAGE_STATS,
    },
  });

  const blocksQuery = useApiQuery('general:homepage_blocks', {
    queryOptions: {
      placeholderData: [ BLOCK ],
    },
  });

  const rollup = config.features.rollup;
  const latestBatchResource = (() => {
    if (rollup.isEnabled && rollup.type === 'zkEvm') return 'general:homepage_zkevm_latest_batch' as const;
    if (rollup.isEnabled && rollup.type === 'zkSync') return 'general:homepage_zksync_latest_batch' as const;
    return 'general:homepage_arbitrum_latest_batch' as const;
  })();
  const latestBatchQuery = useApiQuery(latestBatchResource, {
    queryOptions: { enabled: config.UI.homepage.stats.includes('latest_batch') && rollup.isEnabled && [ 'zkEvm', 'zkSync', 'arbitrum' ].includes(rollup.type) },
  });
  const stateBatchQuery = useApiQuery('general:optimistic_l2_output_roots', {
    queryOptions: { enabled: config.UI.homepage.stats.includes('latest_l1_state_batch') && rollup.isEnabled && rollup.type === 'optimistic' },
  });

  const isLoading = statsQuery.isPlaceholderData || apiQuery.isPlaceholderData || blocksQuery.isPlaceholderData;

  const apiData = apiQuery.data;
  const statsData = statsQuery.data;
  const latestBlock = blocksQuery.data?.[0];

  const coinItems: Array<HighlightsItemProps> = React.useMemo(() => {
    const items: Array<HighlightsItemProps> = [];

    if (config.UI.homepage.charts.includes('coin_price') && !config.UI.nativeCoinPrice.isHidden && typeof apiData?.coin_price === 'string') {
      items.push({
        id: 'coin_price',
        label: `${ config.chain.currency.symbol } price`,
        value: '$' + Number(apiData.coin_price).toLocaleString(undefined, { minimumFractionDigits: 2, maximumFractionDigits: 6 }),
        delta: typeof apiData.coin_price_change_percentage === 'number' ? {
          value: `${ apiData.coin_price_change_percentage > 0 ? '+' : '' }${ apiData.coin_price_change_percentage }%`,
          direction: apiData.coin_price_change_percentage >= 0 ? 'up' : 'down',
        } : undefined,
        icon: <NativeTokenIcon boxSize={ 5 }/>,
        isLoading,
      });
    }

    if (config.UI.homepage.charts.includes('market_cap') && typeof apiData?.market_cap === 'string') {
      items.push({
        id: 'market_cap',
        label: `${ config.chain.currency.symbol } market cap`,
        value: '$' + Number(apiData.market_cap).toLocaleString(undefined, { maximumFractionDigits: 2 }),
        icon: <IconSvg name="globe" boxSize={ 5 } color="icon.secondary"/>,
        isLoading,
      });
    }

    return items;
  }, [ apiData, isLoading ]);

  // Each counter carries its own memo, so the flush that moves the latest block leaves the transaction
  // counter beside it with the very same props and nothing of it re-renders.
  const totalTxsItem: HighlightsItemProps | undefined = React.useMemo(() => {
    const totalTxs = statsData?.total_transactions?.value || apiData?.total_transactions;
    const txsPerDay = statsData?.yesterday_transactions?.value || apiData?.transactions_today;

    if (!totalTxs) {
      return undefined;
    }

    const item: HighlightsItemProps = {
      id: 'total_txs',
      label: 'Transactions',
      value: Number(totalTxs).toLocaleString(undefined, { maximumFractionDigits: 2, notation: 'compact' }),
      secondary: txsPerDay ?
        `${ (Number(txsPerDay) / SECONDS_PER_DAY).toLocaleString(undefined, { maximumFractionDigits: 1 }) } TPS` :
        undefined,
      icon: <IconSvg name="transactions" boxSize={ 5 } color="icon.secondary"/>,
      href: { pathname: '/txs' as const },
      isLoading,
    };

    return isHomeStatsItemEnabled({ id: 'total_txs', label: item.label, value: item.value }) ? item : undefined;
  }, [ statsData, apiData, isLoading ]);

  const latestBlockItem: HighlightsItemProps | undefined = React.useMemo(() => {
    if (!latestBlock) {
      return undefined;
    }

    const blockTime = (() => {
      if (statsData?.average_block_time?.value) {
        return Number(statsData.average_block_time.value);
      }

      if (apiData?.average_block_time !== undefined) {
        return apiData.average_block_time / 1000;
      }

      return undefined;
    })();

    const item: HighlightsItemProps = {
      id: 'total_blocks',
      label: 'Latest block',
      value: latestBlock.height.toLocaleString(),
      secondary: blockTime !== undefined ? `${ blockTime.toFixed(1) }s` : undefined,
      icon: <IconSvg name="block" boxSize={ 5 } color="icon.secondary"/>,
      href: { pathname: '/blocks' as const },
      isLoading,
    };

    return isHomeStatsItemEnabled({ id: 'total_blocks', label: item.label, value: item.value }) ? item : undefined;
  }, [ statsData, apiData, latestBlock, isLoading ]);

  const chainItems: Array<HighlightsItemProps> = React.useMemo(() => {
    const items = new Map<HomeStatsWidgetId, HighlightsItemProps>();
    if (totalTxsItem) items.set('total_txs', totalTxsItem);
    if (latestBlockItem) items.set('total_blocks', latestBlockItem);
    const add = (id: HomeStatsWidgetId, label: string, value: React.ReactNode) => {
      if (value !== undefined && value !== null) items.set(id, { id, label, value, isLoading });
    };
    let blockTime = apiData?.average_block_time !== undefined ? apiData.average_block_time / 1000 : undefined;
    if (statsData?.average_block_time?.value !== undefined) blockTime = Number(statsData.average_block_time.value);
    add('average_block_time', 'Average block time', blockTime !== undefined ? `${ blockTime.toFixed(1) }s` : undefined);
    const addresses = statsData?.total_addresses?.value ?? apiData?.total_addresses;
    add('wallet_addresses', 'Wallet addresses', addresses !== undefined ? Number(addresses).toLocaleString() : undefined);
    if (config.features.gasTracker.isEnabled && apiData?.gas_prices?.average) {
      add('gas_tracker', 'Gas tracker', <GasPrice data={ apiData.gas_prices.average }/>);
    }
    add('btc_locked', 'BTC Locked in 2WP', apiData?.rootstock_locked_btc != null ?
      new BigNumber(apiData.rootstock_locked_btc).div(10 ** 18).toFormat(2) : undefined);
    add('current_epoch', 'Current epoch', apiData?.celo?.epoch_number?.toLocaleString());
    const operationalTxs = statsData?.total_operational_transactions?.value;
    add('total_operational_txs', 'Total operational transactions', operationalTxs !== undefined ? Number(operationalTxs).toLocaleString() : undefined);
    add('latest_batch', 'Latest batch', latestBatchQuery.data?.toLocaleString());
    add('latest_l1_state_batch', 'Latest L1 state batch', stateBatchQuery.data?.items[0]?.l2_output_index.toLocaleString());
    return config.UI.homepage.stats.flatMap((id) => {
      const item = items.get(id);
      return item ? [ item ] : [];
    });
  }, [ totalTxsItem, latestBlockItem, apiData, statsData, isLoading, latestBatchQuery.data, stateBatchQuery.data ]);

  if (apiQuery.isError || statsQuery.isError) {
    return <StatsDegraded/>;
  }

  const hasChart = config.UI.homepage.charts.includes('daily_txs');

  if (coinItems.length === 0 && chainItems.length === 0 && !hasChart) {
    return null;
  }

  return (
    <Box
      data-label="home-stats"
      bgColor="bg.surface"
      borderWidth="1px"
      borderStyle="solid"
      borderColor="border.divider"
      borderRadius="md"
      boxShadow="card"
      overflow="hidden"
    >
      <Grid templateColumns={{ base: '1fr', lg: 'repeat(3, minmax(0, 1fr))' }}>
        <GridItem
          data-label="home-stats-coin"
          borderBottomWidth={{ base: '1px', lg: '0' }}
          borderRightWidth={{ base: '0', lg: '1px' }}
          borderStyle="solid"
          borderColor="border.divider"
        >
          <Highlights items={ coinItems }/>
        </GridItem>
        <GridItem
          data-label="home-stats-chain"
          borderBottomWidth={{ base: '1px', lg: '0' }}
          borderRightWidth={{ base: '0', lg: '1px' }}
          borderStyle="solid"
          borderColor="border.divider"
        >
          <Highlights items={ chainItems }/>
        </GridItem>
        <GridItem data-label="home-stats-history" px={{ base: 4, lg: 5 }} py={{ base: 3, lg: 4 }}>
          { hasChart && <StatsHistory isLoading={ isLoading }/> }
        </GridItem>
      </Grid>
    </Box>
  );
};

export default Stats;
