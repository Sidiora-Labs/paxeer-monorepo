import type { NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import TokensContent from 'ui/pages/Tokens';

const Tokens = config.features.multichain.isEnabled ?
  dynamic(() => import('ui/multichain/tokens/MultichainTokens'), { ssr: false, loading: () => <ContentLoader/> }) :
  TokensContent;

const Page: NextPage = () => {
  return (
    <PageNextJs pathname="/tokens">
      <Tokens/>
    </PageNextJs>
  );
};

export default Page;

export { base as getServerSideProps } from 'nextjs/getServerSideProps/main';
