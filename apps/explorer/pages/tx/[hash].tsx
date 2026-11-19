import type { DehydratedState } from '@tanstack/react-query';
import { QueryClient, dehydrate, HydrationBoundary } from '@tanstack/react-query';
import type { GetServerSideProps, NextPage } from 'next';
import React from 'react';

import type { Props } from 'nextjs/getServerSideProps/handlers';
import * as gSSP from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import Transaction from 'ui/pages/Transaction';
import prefetchPageQuery from 'ui/shared/prefetchPageQuery';

type PageProps = Props & { snapshot: DehydratedState };

const Page: NextPage<PageProps> = (props: PageProps) => {
  return (
    <HydrationBoundary state={ props.snapshot }>
      <PageNextJs pathname="/tx/[hash]" query={ props.query }>
        <Transaction/>
      </PageNextJs>
    </HydrationBoundary>
  );
};

export default Page;

export const getServerSideProps: GetServerSideProps<PageProps> = async(context) => {
  const result = await gSSP.tx(context);
  if (!('props' in result)) {
    return result;
  }
  const props = await result.props;
  const client = new QueryClient();
  const hash = String(context.query.hash || '');
  await Promise.all([
    prefetchPageQuery(client, 'general:tx', { pathParams: { hash } }),
    config.features.paxeerXLists.isEnabled && prefetchPageQuery(client, 'general:paxeer_x_tx_signature', { pathParams: { hash } }),
    config.features.paxeerXLists.isEnabled && prefetchPageQuery(client, 'general:paxeer_x_tx_status', { pathParams: { hash } }),
    prefetchPageQuery(client, 'general:tx_token_transfers', { pathParams: { hash }, queryParams: { type: [] } }),
    prefetchPageQuery(client, 'general:tx_state_changes', { pathParams: { hash } }),
  ]);
  return { props: { ...props, snapshot: dehydrate(client) } };
};
