import { Box } from '@chakra-ui/react';
import type { DehydratedState } from '@tanstack/react-query';
import { QueryClient, dehydrate } from '@tanstack/react-query';
import type { GetServerSideProps, NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

import type { Props } from 'nextjs/getServerSideProps/handlers';
import * as gSSP from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import { INLINE_CONTROL_STYLES } from 'toolkit/utils/inlineControl';
import TransactionsDefault from 'ui/pages/Transactions';
import { PageQuerySnapshotProvider } from 'ui/shared/PageQuerySnapshot';
import prefetchPageQuery from 'ui/shared/prefetchPageQuery';

const TransactionsAlternative = dynamic(() => {
  if (config.features.multichain.isEnabled) {
    return import('ui/multichain/txs/MultichainTxs');
  }

  if (config.features.zetachain.isEnabled) {
    return import('ui/pages/TransactionsZetaChain');
  }

  if (config.features.crossChainTxs.isEnabled) {
    return import('ui/crossChain/txs/Transactions');
  }

  return import('ui/pages/Transactions');
}, { ssr: false });

type PageProps = Props & { snapshot: DehydratedState };

const Page: NextPage<PageProps> = (props: PageProps) => {
  return (
    <PageQuerySnapshotProvider value={ props.snapshot }>
      <PageNextJs pathname="/txs">
        <Box minW={ 0 } w="full" css={{ '& [data-page-api-entry]': INLINE_CONTROL_STYLES }}>
          { config.features.multichain.isEnabled || config.features.zetachain.isEnabled || config.features.crossChainTxs.isEnabled ?
            <TransactionsAlternative/> : <TransactionsDefault/> }
        </Box>
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
  if (!config.features.multichain.isEnabled && !config.features.zetachain.isEnabled && !config.features.crossChainTxs.isEnabled) {
    await prefetchPageQuery(client, 'general:txs_validated', { queryParams: { filter: 'validated' } });
  }
  return { props: { ...props, snapshot: dehydrate(client) } };
};
