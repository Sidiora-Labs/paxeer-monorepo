import type { NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import BlocksPageContent from 'ui/pages/Blocks';

const Blocks = config.features.multichain.isEnabled ?
  dynamic(() => import('ui/multichain/blocks/MultichainBlocks'), { ssr: false }) : BlocksPageContent;

const Page: NextPage = () => {
  return (
    <>
      <PageNextJs pathname="/blocks">{ null }</PageNextJs>
      <Blocks/>
    </>
  );
};

export default Page;

export { base as getServerSideProps } from 'nextjs/getServerSideProps/main';
