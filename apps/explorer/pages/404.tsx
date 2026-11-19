import React from 'react';

import type { NextPageWithLayout } from 'nextjs/types';

import PageNextJs from 'nextjs/PageNextJs';

import NotFound from 'ui/pages/NotFound';
import LayoutError from 'ui/shared/layout/LayoutError';

const Page: NextPageWithLayout = () => {
  return (
    <PageNextJs pathname="/404">
      <NotFound/>
    </PageNextJs>
  );
};

Page.getLayout = function getLayout(page: React.ReactElement) {
  return (
    <LayoutError mainLabel="Page not found">
      { page }
    </LayoutError>
  );
};

export default Page;
