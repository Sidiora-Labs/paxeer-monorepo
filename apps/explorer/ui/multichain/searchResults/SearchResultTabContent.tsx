import React from 'react';

import { Alert } from 'toolkit/chakra/alert';
import { Button } from 'toolkit/chakra/button';
import { EmptyState } from 'toolkit/chakra/empty-state';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';

import SearchResultsList from './SearchResultsList';
import SearchResultsTabAll from './SearchResultsTabAll';
import type { QueryType, SearchQueries } from './utils';

const EMPTY_SEARCH_NAME_MAP: Record<QueryType, string> = {
  addresses: 'addresses',
  tokens: 'tokens',
  blockNumbers: 'block numbers',
  blocks: 'blocks',
  transactions: 'transactions',
  nfts: 'NFTs',
  domains: 'names',
};

interface SearchQueryErrorProps {
  query: SearchQueries[QueryType];
  queryType: QueryType;
}

const SearchQueryError = ({ query, queryType }: SearchQueryErrorProps) => {
  const { refetch, isFetching } = query;
  const handleRetry = React.useCallback(() => {
    refetch();
  }, [ refetch ]);

  return (
    <Alert
      status="error"
      role="alert"
      title={ `Unable to load ${ EMPTY_SEARCH_NAME_MAP[queryType] }. Please try again.` }
      mb={ 4 }
      endElement={ (
        <Button
          size="sm"
          aria-label={ `Retry ${ EMPTY_SEARCH_NAME_MAP[queryType] }` }
          loading={ isFetching }
          onClick={ handleRetry }
        >
          Retry
        </Button>
      ) }
    />
  );
};

interface Props {
  isLoading: boolean;
  searchTerm: string;
  chainId?: string;
  queries: SearchQueries;
  queryType: QueryType | undefined;
  beforeContent?: React.ReactNode;
}

const SearchResultTabContent = ({ isLoading, searchTerm, chainId, queries, queryType, beforeContent }: Props) => {
  const content = (() => {
    if (isLoading && !searchTerm) {
      return <ContentLoader maxW="240px"/>;
    }

    if (!searchTerm) {
      return (
        <EmptyState
          title="Looking for something?"
          description="Try searching by address, smart contract, transaction, block, token or NFT"
        />
      );
    }

    const hasResults = queryType ?
      queries[queryType]?.data?.pages?.[0]?.items && queries[queryType]?.data?.pages?.[0]?.items?.length > 0 :
      Object.values(queries).some((query) => query.data?.pages?.[0]?.items?.length > 0);

    const failedQueryTypes = (queryType ? [ queryType ] : Object.keys(queries) as Array<QueryType>)
      .filter((type) => queries[type].isError);
    const isPending = queryType ? queries[queryType].isPending : isLoading;

    if (isPending && !hasResults && failedQueryTypes.length === 0) {
      return <ContentLoader maxW="240px"/>;
    }

    if (!hasResults && failedQueryTypes.length === 0) {
      return (
        <EmptyState
          title={ queryType ? `No ${ EMPTY_SEARCH_NAME_MAP[queryType] } found` : 'No results found' }
          description="It seems we can't find any results based on your request."
        />
      );
    }

    return (
      <>
        { failedQueryTypes.map((type) => (
          <SearchQueryError key={ type } query={ queries[type] } queryType={ type }/>
        )) }
        { hasResults && (queryType ?
          <SearchResultsList queryType={ queryType } query={ queries[queryType] }/> :
          <SearchResultsTabAll queries={ queries } searchTerm={ searchTerm } chainId={ chainId }/>) }
      </>
    );
  })();

  return (
    <>
      { beforeContent }
      { content }
    </>
  );
};

export default React.memo(SearchResultTabContent);
