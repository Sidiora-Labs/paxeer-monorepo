import type { GetServerSidePropsContext, NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

import { base } from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';
import fetchApi from 'nextjs/utils/fetchApi';
import { httpLogger } from 'nextjs/utils/logger';

import type { InitialChainMetrics } from 'ui/multichain/ecosystems/MultichainEcosystems';
import { SORT_OPTIONS } from 'ui/multichain/ecosystems/utils';
import getSortParamsFromValue from 'ui/shared/sort/getSortParamsFromValue';
import getSortValueFromQuery from 'ui/shared/sort/getSortValueFromQuery';

const MultichainEcosystems = dynamic(() => import('ui/multichain/ecosystems/MultichainEcosystems'), { ssr: false });

const Page: NextPage<{ initialMetrics: InitialChainMetrics }> = ({ initialMetrics }) => {
  return (
    <PageNextJs pathname="/ecosystems">
      <MultichainEcosystems initialMetrics={ initialMetrics }/>
    </PageNextJs>
  );
};

export default Page;

export async function getServerSideProps(context: GetServerSidePropsContext) {
  const result = await base(context);
  if (!('props' in result)) {
    return result;
  }

  const sort = getSortValueFromQuery(context.query, SORT_OPTIONS);
  let initialMetrics: InitialChainMetrics;
  try {
    const data = await fetchApi({
      resource: 'multichainAggregator:chain_metrics',
      queryParams: getSortParamsFromValue(sort),
      timeout: 10_000,
    });
    initialMetrics = data ? { status: 'success', data } : { status: 'error' };
  } catch (error) {
    httpLogger.logger.warn({ message: 'Cannot load ecosystem metrics', err: error });
    initialMetrics = { status: 'error' };
  }

  return { props: { ...await result.props, initialMetrics } };
}
