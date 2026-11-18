import { dehydrate, HydrationBoundary, QueryClient } from '@tanstack/react-query';
import type { DehydratedState } from '@tanstack/react-query';
import type { GetServerSideProps, NextPage } from 'next';
import React from 'react';

import { Resolution } from '@blockscout/stats-types';

import type { Props as BaseProps } from 'nextjs/getServerSideProps/handlers';
import { gasTracker as gasTrackerServerSideProps } from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';
import fetchApi from 'nextjs/utils/fetchApi';

import config from 'configs/app';
import { getResourceKey } from 'lib/api/useApiQuery';
import GasTracker from 'ui/pages/GasTracker';
import { formatDate } from 'ui/shared/chart/utils';
import { STATS_INTERVALS } from 'ui/stats/constants';

type Props = BaseProps & { dehydratedState?: DehydratedState };

const Page: NextPage<Props> = (props) => {
  return (
    <HydrationBoundary state={ props.dehydratedState }>
      <PageNextJs pathname="/gas-tracker">
        <GasTracker/>
      </PageNextJs>
    </HydrationBoundary>
  );
};

export default Page;

export const getServerSideProps: GetServerSideProps<Props> = async(ctx) => {
  const response = await gasTrackerServerSideProps(ctx);
  if (!('props' in response)) {
    return response;
  }

  const client = new QueryClient();
  const resources = [ 'general:stats', ...(config.features.stats.isEnabled && config.apis.stats ? [ 'stats:lines' ] as const : []) ] as const;
  await Promise.all(resources.map(async(resource) => {
    const data = await fetchApi({ resource });
    if (data !== undefined) {
      client.setQueryData([ resource ], data);
    }
  }));
  if (config.features.stats.isEnabled && config.apis.stats) {
    const pathParams = { id: 'averageGasPrice' };
    const queryParams = {
      from: STATS_INTERVALS.oneMonth.start ? formatDate(STATS_INTERVALS.oneMonth.start) : undefined,
      to: formatDate(new Date()),
      resolution: Resolution.DAY,
    };
    const data = await fetchApi({ resource: 'stats:line', pathParams, queryParams });
    if (data !== undefined) {
      client.setQueryData(getResourceKey('stats:line', { pathParams, queryParams }), data);
    }
  }
  return { props: { ...await response.props, dehydratedState: dehydrate(client) } };
};
