import type { NextPage } from 'next';
import dynamic from 'next/dynamic';
import React from 'react';

const ChainsList = dynamic(() => import('ui/paxeerX/chains/ChainsList'), { ssr: false });

const Page: NextPage = () => <ChainsList/>;

export default Page;

export { paxeerXLists as getServerSideProps } from 'nextjs/getServerSideProps/main';
