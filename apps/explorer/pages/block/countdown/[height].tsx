import type { NextPage } from 'next';
import React from 'react';

import type { Props } from 'nextjs/getServerSideProps/handlers';
import PageNextJs from 'nextjs/PageNextJs';

import BlockCountdown from 'ui/pages/BlockCountdown';

const Page: NextPage<Props> = (props: Props) => {
  return (
    <>
      <PageNextJs pathname="/block/countdown/[height]" query={ props.query }>{ null }</PageNextJs>
      <BlockCountdown/>
    </>
  );
};

export default Page;

export { block as getServerSideProps } from 'nextjs/getServerSideProps/main';
