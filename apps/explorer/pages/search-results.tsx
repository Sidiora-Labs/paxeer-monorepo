import dynamic from 'next/dynamic';
import React from 'react';

import type { NextPageWithLayout } from 'nextjs/types';

import type { Route } from 'nextjs-routes';
import type { Props } from 'nextjs/getServerSideProps/handlers';
import PageNextJs from 'nextjs/PageNextJs';

import config from 'configs/app';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import SearchResultsContent from 'ui/pages/SearchResults';
import LayoutSearchResults from 'ui/shared/layout/LayoutSearchResults';

const pathname: Route['pathname'] = '/search-results';

const SearchResults = config.features.multichain.isEnabled ?
  dynamic(() => import('ui/multichain/searchResults/SearchResults'), { ssr: false, loading: () => <ContentLoader/> }) :
  SearchResultsContent;

const Page: NextPageWithLayout<Props> = (props: Props) => {
  return (
    <PageNextJs pathname={ pathname } query={ props.query }>
      <SearchResults/>
    </PageNextJs>
  );
};

Page.getLayout = function getLayout(page: React.ReactElement) {
  return (
    <LayoutSearchResults>
      { page }
    </LayoutSearchResults>
  );
};

export default Page;

export { base as getServerSideProps } from 'nextjs/getServerSideProps/main';
