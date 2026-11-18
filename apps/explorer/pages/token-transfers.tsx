import type { NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import TokenTransfersContent from 'ui/pages/TokenTransfers';

const TokenTransfers = config.features.multichain.isEnabled ?
  dynamic(() => import('ui/multichain/tokenTransfers/MultichainTokenTransfers'), { ssr: false, loading: () => <ContentLoader/> }) :
  TokenTransfersContent;

const Page: NextPage = () => {
  return (
    <PageNextJs pathname="/token-transfers">
      <TokenTransfers/>
    </PageNextJs>
  );
};

export default Page;

export { base as getServerSideProps } from 'nextjs/getServerSideProps/main';
