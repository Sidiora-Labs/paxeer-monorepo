import type { NextPage } from 'next';
import React from 'react';

import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import MultichainStats from 'ui/multichain/stats/MultichainStats';
import Stats from 'ui/pages/Stats';

export function StatsPageContent() {
  return config.features.multichain.isEnabled ? <MultichainStats/> : <Stats/>;
}

const Page: NextPage = () => {
  return (
    <PageNextJs pathname="/stats">
      <StatsPageContent/>
    </PageNextJs>
  );
};

export default Page;

export { stats as getServerSideProps } from 'nextjs/getServerSideProps/main';
