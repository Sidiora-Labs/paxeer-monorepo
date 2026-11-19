import type { DehydratedState } from '@tanstack/react-query';
import { QueryClient, dehydrate } from '@tanstack/react-query';
import type { GetServerSideProps, NextPage } from 'next';
import React from 'react';

import type { Route } from 'nextjs-routes';
import type { Props } from 'nextjs/getServerSideProps/handlers';
import * as gSSP from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';

import PaxeerXFeedMarket from 'ui/pages/PaxeerXFeedMarket';
import { PageQuerySnapshotProvider } from 'ui/shared/PageQuerySnapshot';
import prefetchPageQuery from 'ui/shared/prefetchPageQuery';

const pathname: Route['pathname'] = '/feed/market/[id]';

type PageProps = Props<typeof pathname> & { snapshot: DehydratedState };

const Page: NextPage<PageProps> = (props: PageProps) => {
  return (
    <PageQuerySnapshotProvider value={ props.snapshot }>
      <PageNextJs pathname={ pathname } query={ props.query }>
        <PaxeerXFeedMarket/>
      </PageNextJs>
    </PageQuerySnapshotProvider>
  );
};

export default Page;

export const getServerSideProps: GetServerSideProps<PageProps> = async(context) => {
  const result = await gSSP.paxeerXFeed(context);
  if (!('props' in result)) {
    return result;
  }
  const props = await result.props;
  const client = new QueryClient();
  const id = String(context.query.id || '');
  await Promise.all([
    prefetchPageQuery(client, 'general:paxeer_x_feed_markets', {}),
    prefetchPageQuery(client, 'general:paxeer_x_feed_market_updates', { pathParams: { id } }),
  ]);
  return { props: { ...props, snapshot: dehydrate(client) } };
};
