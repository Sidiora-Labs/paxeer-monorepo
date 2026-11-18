import type { NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import Accounts from 'ui/pages/Accounts';

const MultichainAccounts = dynamic(() => import('ui/multichain/accounts/MultichainAccounts'), { ssr: false });

const Page: NextPage = () => {
  return (
    <PageNextJs pathname="/accounts">
      { config.features.multichain.isEnabled ? <MultichainAccounts/> : <Accounts/> }
    </PageNextJs>
  );
};

export default Page;

export { accounts as getServerSideProps } from 'nextjs/getServerSideProps/main';
