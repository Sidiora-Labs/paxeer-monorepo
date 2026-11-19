import React from 'react';

import type { NextPageWithLayout } from 'nextjs/types';

import PageNextJs from 'nextjs/PageNextJs';

import AppError from 'ui/shared/AppError/AppError';
import LayoutError from 'ui/shared/layout/LayoutError';

const NOT_FOUND_STATUS = 404;
const error = new Error('Not found', { cause: { status: NOT_FOUND_STATUS } });

const Page: NextPageWithLayout = () => {
  return (
    <PageNextJs pathname="/404">
      <AppError error={ error }/>
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
