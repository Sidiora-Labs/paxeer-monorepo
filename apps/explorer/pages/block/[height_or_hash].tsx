import type { NextPage } from 'next';
import React from 'react';

import type { Props } from 'nextjs/getServerSideProps/handlers';
import PageNextJs from 'nextjs/PageNextJs';

import Block from 'ui/pages/Block';

const Page: NextPage<Props> = (props: Props) => {
  return (
    <>
      <PageNextJs pathname="/block/[height_or_hash]" query={ props.query }>{ null }</PageNextJs>
      <Block/>
    </>
  );
};

export default Page;

export { block as getServerSideProps } from 'nextjs/getServerSideProps/main';
