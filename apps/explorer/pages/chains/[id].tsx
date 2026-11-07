import type { NextPage } from 'next';
import dynamic from 'next/dynamic';
import { useRouter } from 'next/router';
import React from 'react';

const ChainDetail = dynamic(() => import('ui/paxeerX/chains/ChainDetail'), { ssr: false });

const Page: NextPage = () => {
  const router = useRouter();
  if (!router.isReady) {
    return null;
  }
  const id = router.query.id;
  if (typeof id !== 'string' || !/^[1-9]\d*$/.test(id) || Number(id) > 0xFFFFFFFF) {
    return <p role="alert">Invalid virtual chain ID.</p>;
  }
  return <ChainDetail key={ id } id={ Number(id) }/>;
};

export default Page;

export { paxeerXLists as getServerSideProps } from 'nextjs/getServerSideProps/main';
