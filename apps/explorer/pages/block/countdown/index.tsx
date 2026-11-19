import type { NextPage } from 'next';
import React from 'react';

import type { Props } from 'nextjs/getServerSideProps/handlers';
import PageNextJs from 'nextjs/PageNextJs';

import BlockCountdownIndex from 'ui/pages/BlockCountdownIndex';

const Page: NextPage<Props> = (props: Props) => {
  return (
    <>
      <PageNextJs pathname="/block/countdown" query={ props.query }>{ null }</PageNextJs>
      <BlockCountdownIndex/>
    </>
  );
};

export default Page;

export { block as getServerSideProps } from 'nextjs/getServerSideProps/main';
