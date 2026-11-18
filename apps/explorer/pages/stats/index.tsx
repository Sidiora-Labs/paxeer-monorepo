import { dehydrate, HydrationBoundary, QueryClient } from '@tanstack/react-query';
import type { DehydratedState } from '@tanstack/react-query';
import type { GetServerSideProps, NextPage } from 'next';
import React from 'react';

import type { Props as BaseProps } from 'nextjs/getServerSideProps/handlers';
import { stats as statsServerSideProps } from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';
import fetchApi from 'nextjs/utils/fetchApi';

import config from 'configs/app';
import MultichainStats from 'ui/multichain/stats/MultichainStats';
import Stats from 'ui/pages/Stats';

export function StatsPageContent() {
  return config.features.multichain.isEnabled ? <MultichainStats/> : <Stats/>;
}

type Props = BaseProps & { dehydratedState?: DehydratedState };

const Page: NextPage<Props> = (props) => {
  return (
    <HydrationBoundary state={ props.dehydratedState }>
      <PageNextJs pathname="/stats">
        <StatsPageContent/>
      </PageNextJs>
    </HydrationBoundary>
  );
};

export default Page;

export const getServerSideProps: GetServerSideProps<Props> = async(ctx) => {
  const response = await statsServerSideProps(ctx);
  if (!('props' in response) || config.features.multichain.isEnabled || !config.apis.stats) {
    return response;
  }

  const client = new QueryClient();
  await Promise.all(([ 'stats:lines', 'stats:counters', 'general:stats', 'general:homepage_blocks' ] as const).map(async(resource) => {
    const data = await fetchApi({ resource });
    if (data !== undefined) {
      client.setQueryData([ resource ], data);
    }
  }));
  return { props: { ...await response.props, dehydratedState: dehydrate(client) } };
};
