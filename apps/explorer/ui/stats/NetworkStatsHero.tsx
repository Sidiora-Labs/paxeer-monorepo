import { Grid } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import type * as stats from '@blockscout/stats-types';

import useApiQuery from 'lib/api/useApiQuery';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import { ScanStatCard } from 'ui/shared/scan';

import useStatsService from './useStatsService';

function formatCount(value: string | null | undefined): string | undefined {
  if (value == null || value === '') {
    return undefined;
  }
  const number = new BigNumber(value);
  return number.isFinite() ? number.toFormat(0) : undefined;
}

export function getNetworkCounter(counters: Array<stats.Counter> | undefined, label: string): string | undefined {
  const counter = counters?.find((item) => item.title.toLowerCase() === label.toLowerCase());
  if (!counter || counter.value.trim() === '' || !Number.isFinite(Number(counter.value))) {
    return undefined;
  }
  return Number(counter.value).toLocaleString(undefined, { maximumFractionDigits: 3 });
}

const NetworkStatsHero = () => {
  const hasStatsService = useStatsService();
  const counters = useApiQuery('stats:counters', { queryOptions: { enabled: hasStatsService } });
  const stats = useApiQuery('general:stats', { queryOptions: { refetchOnMount: false } });

  const data = stats.data;
  const blockTime = data?.average_block_time != null ? data.average_block_time / 1000 : undefined;
  const epoch = data?.celo?.epoch_number;
  const serviceCounters = hasStatsService ? counters.data?.counters : undefined;
  const items = [
    { label: 'Latest block', value: formatCount(data?.total_blocks), isLoading: stats.isPending },
    {
      label: 'Average block time',
      value: blockTime !== undefined && Number.isFinite(blockTime) ? `${ blockTime.toFixed(1) }s` : undefined,
      isLoading: stats.isPending,
    },
    { label: 'Transactions (24h)', value: formatCount(data?.transactions_today), isLoading: stats.isPending },
    { label: 'Wallet addresses', value: formatCount(data?.total_addresses), isLoading: stats.isPending },
    {
      label: 'Transactions per second',
      value: getNetworkCounter(serviceCounters, 'Transactions per second'),
      isLoading: hasStatsService && counters.isPending,
    },
    {
      label: 'Active validators',
      value: getNetworkCounter(serviceCounters, 'Active validators'),
      isLoading: hasStatsService && counters.isPending,
    },
    {
      label: 'Current epoch',
      value: epoch !== undefined && Number.isFinite(epoch) ? epoch.toLocaleString() : undefined,
      isLoading: stats.isPending,
    },
  ].filter((item) => item.value !== undefined || item.isLoading);

  return (
    <>
      { ((hasStatsService && counters.isError) || stats.isError) && <DataFetchAlert/> }
      { items.length > 0 && (
        <Grid data-network-stats-hero templateColumns={{ base: '1fr', sm: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(4, minmax(0, 1fr))' }} gap={ 4 } mb={ 8 }>
          { items.map((item) => <ScanStatCard key={ item.label } { ...item }/>) }
        </Grid>
      ) }
    </>
  );
};

export default NetworkStatsHero;
