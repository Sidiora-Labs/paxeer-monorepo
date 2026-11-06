import { useRouter } from 'next/router';
import React from 'react';

import type { ListCctxsResponse } from '@blockscout/zetachain-cctx-types';
import type { QuickSearchResultItem } from 'types/client/search';

import config from 'configs/app';
import { fromBech32Address, isBech32Address } from 'lib/address/bech32';
import useApiQuery from 'lib/api/useApiQuery';
import useDebounce from 'lib/hooks/useDebounce';
import useUpdateValueEffect from 'lib/hooks/useUpdateValueEffect';
import getQueryParamString from 'lib/router/getQueryParamString';
import type { ExternalSearchItem } from 'lib/search/externalSearch';
import { getExternalSearchItem } from 'lib/search/externalSearch';
import { SEARCH_RESULT_ITEM, SEARCH_RESULT_NEXT_PAGE_PARAMS } from 'stubs/search';
import { generateListStub } from 'stubs/utils';
import useMarketplaceApps from 'ui/marketplace/useMarketplaceApps';
import useQueryWithPages from 'ui/shared/pagination/useQueryWithPages';

export default function useSearchQuery(withRedirectCheck?: boolean) {
  const router = useRouter();
  const q = React.useRef(getQueryParamString(router.query.q));
  const initialValue = q.current;

  const [ searchTerm, setSearchTerm ] = React.useState(initialValue);

  const debouncedSearchTerm = useDebounce(searchTerm, 300);
  const pathname = router.pathname;

  const query = useQueryWithPages({
    resourceName: 'general:search',
    filters: { q: isBech32Address(debouncedSearchTerm) ? fromBech32Address(debouncedSearchTerm) : debouncedSearchTerm },
    options: {
      enabled: debouncedSearchTerm.trim().length > 0,
      placeholderData: generateListStub<'general:search'>(SEARCH_RESULT_ITEM, 50, { next_page_params: SEARCH_RESULT_NEXT_PAGE_PARAMS }),
    },
  });

  const redirectCheckQuery = useApiQuery('general:search_check_redirect', {
    // on search result page we check redirect only once on mount
    queryParams: { q: q.current },
    queryOptions: { enabled: Boolean(q.current) && withRedirectCheck },
  });

  const zetaChainCCTXQuery = useApiQuery('zetachain:transactions', {
    queryParams: {
      hash: debouncedSearchTerm,
      limit: 50,
      offset: 0,
      direction: 'DESC',
    },
    queryOptions: { enabled: config.features.zetachain.isEnabled && debouncedSearchTerm.trim().length > 0 },
  });

  useUpdateValueEffect(() => {
    query.onFilterChange({ q: debouncedSearchTerm });
  }, debouncedSearchTerm);

  return React.useMemo(() => ({
    searchTerm,
    debouncedSearchTerm,
    handleSearchTermChange: setSearchTerm,
    query,
    redirectCheckQuery,
    pathname,
    zetaChainCCTXQuery,
    externalSearchItem: getExternalSearchItem(debouncedSearchTerm),
  }), [ debouncedSearchTerm, pathname, query, redirectCheckQuery, searchTerm, zetaChainCCTXQuery ]);
}

type AssistantSearch = {
  searchTerm: string;
  debouncedSearchTerm: string;
  query: {
    data?: ReadonlyArray<QuickSearchResultItem>;
    isSuccess: boolean;
    isFetching: boolean;
    isPlaceholderData: boolean;
  };
  zetaChainCCTXQuery: {
    data?: ListCctxsResponse;
    isSuccess: boolean;
    isFetching: boolean;
  };
  externalSearchItem: ExternalSearchItem;
};

type AssistantAnswer = { answer: string; model: string };

export function useSearchAssistant({ searchTerm, debouncedSearchTerm, query, zetaChainCCTXQuery, externalSearchItem }: AssistantSearch) {
  const router = useRouter();
  const marketplaceApps = useMarketplaceApps(debouncedSearchTerm);
  const question = searchTerm.trim();
  const enabled = config.features.assistant.isEnabled;
  const noMatch = searchTerm === debouncedSearchTerm && query.isSuccess && !query.isFetching && !query.isPlaceholderData &&
    query.data?.length === 0 && !externalSearchItem && !marketplaceApps.isPlaceholderData && !marketplaceApps.displayedApps.length &&
    (!config.features.zetachain.isEnabled || (zetaChainCCTXQuery.isSuccess && !zetaChainCCTXQuery.isFetching &&
      zetaChainCCTXQuery.data?.items.length === 0));
  const canAsk = enabled && question.length > 0 && (question.endsWith('?') || noMatch);
  const [ result, setResult ] = React.useState<{ question: string; value: AssistantAnswer }>();
  const [ error, setError ] = React.useState<{ question: string; message: string }>();
  const [ pending, setPending ] = React.useState(false);
  const requestRef = React.useRef<AbortController | undefined>(undefined);

  React.useEffect(() => {
    requestRef.current?.abort();
    requestRef.current = undefined;
    setPending(false);
    setResult(undefined);
    setError(undefined);
    return () => requestRef.current?.abort();
  }, [ question, enabled ]);

  const ask = React.useCallback(async() => {
    if (!canAsk || requestRef.current) return;
    const controller = new AbortController();
    requestRef.current = controller;
    setPending(true);
    setResult(undefined);
    setError(undefined);
    const entityKind = /^\/(address|tx|block|token)\//.exec(router.pathname)?.[1];
    const identifier = [ router.query.hash, router.query.height_or_hash, router.query.height, router.query.id ].find(value => typeof value === 'string');
    try {
      const response = await fetch(`${ config.app.basePath }/api/assistant`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        signal: controller.signal,
        body: JSON.stringify({
          question,
          context: { route: router.asPath, entityKind, identifier },
        }),
      });
      if (!response.ok) throw new Error('Assistant unavailable');
      const value: unknown = await response.json();
      if (typeof value !== 'object' || value === null || !('answer' in value) || !('model' in value) ||
        typeof value.answer !== 'string' || typeof value.model !== 'string' || !value.answer.trim()) {
        throw new Error('Invalid assistant response');
      }
      if (!controller.signal.aborted) setResult({ question, value: { answer: value.answer, model: value.model } });
    } catch (error) {
      if (!controller.signal.aborted) setError({ question, message: 'Assistant unavailable. Please try again later.' });
    } finally {
      if (requestRef.current === controller) {
        requestRef.current = undefined;
        setPending(false);
      }
    }
  }, [ canAsk, question, router.asPath, router.pathname, router.query.hash, router.query.height_or_hash, router.query.height, router.query.id ]);

  return {
    question,
    canAsk,
    pending,
    ask,
    answer: enabled && result?.question === question ? result.value : undefined,
    error: enabled && error?.question === question ? error.message : undefined,
  };
}
