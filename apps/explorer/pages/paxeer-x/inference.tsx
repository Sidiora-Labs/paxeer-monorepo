import type { NextPage } from 'next';
import dynamic from 'next/dynamic';
import { useRouter } from 'next/router';
import React from 'react';

import PageNextJs from 'nextjs/PageNextJs';

const PaxeerXInference = dynamic(() => import('ui/pages/PaxeerXInference'), { ssr: false });

const Page: NextPage = () => {
  const router = useRouter();
  return (
    <PageNextJs pathname={ router.pathname }>
      <PaxeerXInference/>
    </PageNextJs>
  );
};

export default Page;

export { paxeerXLists as getServerSideProps } from 'nextjs/getServerSideProps/main';
