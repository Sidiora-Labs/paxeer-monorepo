import type { DehydratedState } from '@tanstack/react-query';
import { QueryClient, dehydrate } from '@tanstack/react-query';
import type { GetServerSideProps, NextPage } from 'next';
import React from 'react';

import type { Props } from 'nextjs/getServerSideProps/handlers';
import * as gSSP from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';

import PaxeerXFeed from 'ui/pages/PaxeerXFeed';
import { PageQuerySnapshotProvider } from 'ui/shared/PageQuerySnapshot';
import prefetchPageQuery from 'ui/shared/prefetchPageQuery';

type PageProps = Props & { snapshot: DehydratedState };

const Page: NextPage<PageProps> = (props: PageProps) => {
  return (
    <PageQuerySnapshotProvider value={ props.snapshot }>
      <PageNextJs pathname="/feed">
        <PaxeerXFeed/>
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
  await Promise.all([
    prefetchPageQuery(client, 'general:paxeer_x_feed_markets', {}),
  ]);
  return { props: { ...props, snapshot: dehydrate(client) } };
};
