import type { NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import VerifiedContracts from 'ui/pages/VerifiedContracts';

const MultichainVerifiedContracts = dynamic(() => import('ui/multichain/verifiedContracts/MultichainVerifiedContracts'), { ssr: false });

const Page: NextPage = () => {
  return (
    <PageNextJs pathname="/verified-contracts">
      { config.features.multichain.isEnabled ? <MultichainVerifiedContracts/> : <VerifiedContracts/> }
    </PageNextJs>
  );
};

export default Page;

export { base as getServerSideProps } from 'nextjs/getServerSideProps/main';
