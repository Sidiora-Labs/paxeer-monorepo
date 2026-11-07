import { chakra, Grid } from '@chakra-ui/react';
import React from 'react';

import type * as stats from '@blockscout/stats-types';

import useApiQuery from 'lib/api/useApiQuery';
import DataFetchAlert from 'ui/shared/DataFetchAlert';
import { ScanStatCard } from 'ui/shared/scan';

export const NETWORK_HERO_LABELS = [ 'Transactions per second', 'Active validators', 'Current epoch', 'Latest block' ] as const;

export function getNetworkCounter(counters: Array<stats.Counter> | undefined, label: string): string | undefined {
  const counter = counters?.find((item) => item.title.toLowerCase() === label.toLowerCase());
  if (!counter || counter.value.trim() === '' || !Number.isFinite(Number(counter.value))) {
    return undefined;
  }
  return Number(counter.value).toLocaleString(undefined, { maximumFractionDigits: 3 });
}

const NetworkStatsHero = () => {
  const counters = useApiQuery('stats:counters');
  const stats = useApiQuery('general:stats', { queryOptions: { refetchOnMount: false } });

  return (
    <>
      { (counters.isError || stats.isError) && <DataFetchAlert/> }
      <Grid data-network-stats-hero templateColumns={{ base: '1fr', sm: 'repeat(2, minmax(0, 1fr))', lg: 'repeat(4, minmax(0, 1fr))' }} gap={ 4 } mb={ 8 }>
        { NETWORK_HERO_LABELS.map((label) => {
          const epoch = label === 'Current epoch' ? stats.data?.celo?.epoch_number : undefined;
          const value = epoch !== undefined && Number.isFinite(epoch) ? epoch.toLocaleString() : getNetworkCounter(counters.data?.counters, label);
          return (
            <ScanStatCard key={ label } label={ label } value={ value ?? <chakra.span color="text.muted">—</chakra.span> } isLoading={ counters.isPending || stats.isPending }/>
          );
        }) }
      </Grid>
    </>
  );
};

export default NetworkStatsHero;
