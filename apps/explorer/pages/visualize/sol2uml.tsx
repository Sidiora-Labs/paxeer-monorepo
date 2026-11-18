import type { GetServerSidePropsContext, NextPage } from 'next';
import React from 'react';

import { base } from 'nextjs/getServerSideProps/main';
import PageNextJs from 'nextjs/PageNextJs';

import Sol2Uml from 'ui/pages/Sol2Uml';

const Page: NextPage = () => {
  return (
    <PageNextJs pathname="/visualize/sol2uml">
      <Sol2Uml/>
    </PageNextJs>
  );
};

export default Page;

export async function getServerSideProps(context: GetServerSidePropsContext) {
  if (typeof context.query.address !== 'string' || !context.query.address) {
    return { notFound: true };
  }

  return base(context);
}
