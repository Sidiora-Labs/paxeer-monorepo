import type { DehydratedState } from '@tanstack/react-query';
import { QueryClient, dehydrate } from '@tanstack/react-query';
import type { GetServerSideProps, NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

import type { Props } from 'nextjs/getServerSideProps/handlers';
import * as gSSP from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import InternalTxsDefault from 'ui/pages/InternalTxs';
import { PageQuerySnapshotProvider } from 'ui/shared/PageQuerySnapshot';
import prefetchPageQuery from 'ui/shared/prefetchPageQuery';

const InternalTxs = config.features.multichain.isEnabled ?
  dynamic(() => import('ui/multichain/internalTxs/MultichainInternalTxs'), { ssr: false }) : InternalTxsDefault;

type PageProps = Props & { snapshot: DehydratedState };

const Page: NextPage<PageProps> = (props: PageProps) => {
  return (
    <PageQuerySnapshotProvider value={ props.snapshot }>
      <PageNextJs pathname="/internal-txs">
        <InternalTxs/>
      </PageNextJs>
    </PageQuerySnapshotProvider>
  );
};

export default Page;

export const getServerSideProps: GetServerSideProps<PageProps> = async(context) => {
  const result = await gSSP.base(context);
  if (!('props' in result)) {
    return result;
  }
  const props = await result.props;
  const client = new QueryClient();
  const transactionHash = typeof context.query.transaction_hash === 'string' ? context.query.transaction_hash : '';
  await Promise.all([
    !config.features.multichain.isEnabled && prefetchPageQuery(client, 'general:internal_txs', { queryParams: { transaction_hash: transactionHash } }),
  ]);
  return { props: { ...props, snapshot: dehydrate(client) } };
};
