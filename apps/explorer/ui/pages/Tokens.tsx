import { Box } from '@chakra-ui/react';
import { omit } from 'es-toolkit';
import { useRouter } from 'next/router';
import React from 'react';

import type { TokenType } from 'types/api/token';
import type { TokensSortingValue, TokensSortingField, TokensSorting } from 'types/api/tokens';

import config from 'configs/app';
import useDebounce from 'lib/hooks/useDebounce';
import getQueryParamString from 'lib/router/getQueryParamString';
import { getTokenTypes, isFungibleTokenType, NFT_TOKEN_TYPE_IDS } from 'lib/token/tokenTypes';
import { TOKEN_INFO_ERC_20 } from 'stubs/token';
import { generateListStub } from 'stubs/utils';
import PopoverFilter from 'ui/shared/filters/PopoverFilter';
import TokenCategoryFilter from 'ui/shared/filters/TokenCategoryFilter';
import type { TokenCategory } from 'ui/shared/filters/TokenCategoryFilter';
import TokenTypeFilter from 'ui/shared/filters/TokenTypeFilter';
import PageTitle from 'ui/shared/Page/PageTitle';
import Pagination from 'ui/shared/pagination/Pagination';
import useQueryWithPages from 'ui/shared/pagination/useQueryWithPages';
import { ScanShowRows } from 'ui/shared/scan';
import getSortParamsFromValue from 'ui/shared/sort/getSortParamsFromValue';
import getSortValueFromQuery from 'ui/shared/sort/getSortValueFromQuery';
import TokensList from 'ui/tokens/Tokens';
import TokensActionBar from 'ui/tokens/TokensActionBar';
import TokensBridgedChainsFilter from 'ui/tokens/TokensBridgedChainsFilter';
import { SORT_OPTIONS, getTokenFilterValue, getBridgedChainsFilterValue } from 'ui/tokens/utils';

// The list endpoint answers a fixed page, so the row selector chooses how much of that page the card
// shows and never promises a page size the API cannot serve.
const API_PAGE_SIZE = 50;
const ROWS_OPTIONS = [ 25, API_PAGE_SIZE ];

const bridgedTokensFeature = config.features.bridgedTokens;

const TokensContent = () => {
  const router = useRouter();

  const tab = getQueryParamString(router.query.tab);
  const q = getQueryParamString(router.query.q);

  const [ searchTerm, setSearchTerm ] = React.useState<string>(q ?? '');
  const [ sort, setSort ] = React.useState<TokensSortingValue>(getSortValueFromQuery<TokensSortingValue>(router.query, SORT_OPTIONS) ?? 'default');
  const tokenTypes = React.useMemo(() => getTokenFilterValue(router.query.type), [ router.query.type ]);
  const bridgeChains = React.useMemo(() => getBridgedChainsFilterValue(router.query.chain_ids), [ router.query.chain_ids ]);
  const [ rowsCount, setRowsCount ] = React.useState<number>(API_PAGE_SIZE);

  const debouncedSearchTerm = useDebounce(searchTerm, 300);

  const tokensQuery = useQueryWithPages({
    resourceName: tab === 'bridged' ? 'general:tokens_bridged' : 'general:tokens',
    filters: tab === 'bridged' ? { q: debouncedSearchTerm, chain_ids: bridgeChains } : { q: debouncedSearchTerm, type: tokenTypes },
    sorting: getSortParamsFromValue<TokensSortingValue, TokensSortingField, TokensSorting['order']>(sort),
    options: {
      placeholderData: generateListStub<'general:tokens'>(
        TOKEN_INFO_ERC_20,
        50,
        {
          next_page_params: {
            holders_count: 81528,
            items_count: 50,
            name: '',
            market_cap: null,
          },
        },
      ),
    },
  });

  const handleSearchTermChange = React.useCallback((value: string) => {
    tab === 'bridged' ?
      tokensQuery.onFilterChange({ q: value, chain_ids: bridgeChains }) :
      tokensQuery.onFilterChange({ q: value, type: tokenTypes });
    setSearchTerm(value);
  }, [ bridgeChains, tab, tokenTypes, tokensQuery ]);

  const handleTokenTypesChange = React.useCallback((value: Array<TokenType>) => {
    tokensQuery.onFilterChange({ q: debouncedSearchTerm, type: value });
  }, [ debouncedSearchTerm, tokensQuery ]);

  const handleBridgeChainsChange = React.useCallback((value: Array<string>) => {
    tokensQuery.onFilterChange({ q: debouncedSearchTerm, chain_ids: value });
  }, [ debouncedSearchTerm, tokensQuery ]);

  const handleSortChange = React.useCallback((value: TokensSortingValue) => {
    setSort(value);
    tokensQuery.onSortingChange(getSortParamsFromValue(value));
  }, [ tokensQuery ]);

  const fungibleTypes = React.useMemo(() => Object.keys(getTokenTypes(false)).filter(isFungibleTokenType), []);
  const category: TokenCategory | undefined = (() => {
    if (tab === 'bridged') {
      return 'bridged';
    }
    if (!tokenTypes?.length) {
      return 'all';
    }
    if (tokenTypes.length === fungibleTypes.length && tokenTypes.every((type) => fungibleTypes.includes(type))) {
      return 'fungible';
    }
    if (tokenTypes.length === NFT_TOKEN_TYPE_IDS.length && tokenTypes.every((type) => NFT_TOKEN_TYPE_IDS.some((nftType) => nftType === type))) {
      return 'nft';
    }
    return undefined;
  })();

  const handleCategoryChange = React.useCallback((value: TokenCategory) => {
    let types: Array<TokenType> = [];
    if (value === 'fungible') {
      types = fungibleTypes;
    } else if (value === 'nft') {
      types = NFT_TOKEN_TYPE_IDS;
    }
    if (value === 'bridged' || tab === 'bridged') {
      router.push({ pathname: '/tokens', query: {
        ...omit(router.query, [ 'type', 'chain_ids', 'page', 'next_page_params' ]),
        tab: value === 'bridged' ? 'bridged' : 'all',
        ...(types.length ? { type: types.join(',') } : {}),
        q: searchTerm || undefined,
      } }, undefined, { shallow: true });
      return;
    }
    handleTokenTypesChange(types);
  }, [ fungibleTypes, handleTokenTypesChange, router, searchTerm, tab ]);

  const filter = tab === 'bridged' ? (
    <PopoverFilter contentProps={{ maxW: '350px' }} appliedFiltersNum={ bridgeChains?.length }>
      <TokensBridgedChainsFilter onChange={ handleBridgeChainsChange } defaultValue={ bridgeChains }/>
    </PopoverFilter>
  ) : (
    <PopoverFilter contentProps={{ w: '200px' }} appliedFiltersNum={ tokenTypes?.length }>
      <TokenTypeFilter<TokenType> key={ tokenTypes?.join(',') } onChange={ handleTokenTypesChange } defaultValue={ tokenTypes } nftOnly={ false }/>
    </PopoverFilter>
  );

  const actions = (
    <TokensActionBar
      key={ tab }
      filter={ filter }
      searchTerm={ searchTerm }
      onSearchChange={ handleSearchTermChange }
      sort={ sort }
      onSortChange={ handleSortChange }
    />
  );

  const pagination = <Pagination { ...tokensQuery.pagination }/>;

  const showRows = (
    <ScanShowRows
      value={ rowsCount }
      onValueChange={ setRowsCount }
      options={ ROWS_OPTIONS }
      isLoading={ tokensQuery.isPlaceholderData }
    />
  );

  const description = (() => {
    if (!bridgedTokensFeature.isEnabled) {
      return null;
    }

    const bridgesListText = bridgedTokensFeature.bridges.map((item, index, array) => {
      return item.title + (index < array.length - 2 ? ', ' : '') + (index === array.length - 2 ? ' and ' : '');
    });

    return (
      <Box textStyle="sm" mb={ 4 } mt={ 1 } whiteSpace="pre-wrap" flexWrap="wrap">
        List of the tokens bridged through { bridgesListText } extensions
      </Box>
    );
  })();

  const renderList = (hasActiveFilters: boolean, listDescription?: React.ReactNode) => (
    <TokensList
      query={ tokensQuery }
      sort={ sort }
      onSortChange={ handleSortChange }
      hasActiveFilters={ hasActiveFilters }
      description={ listDescription }
      actions={ actions }
      pagination={ pagination }
      showRows={ showRows }
      rowsCount={ rowsCount }
    />
  );

  return (
    <>
      <PageTitle
        title={ config.meta.seo.enhancedDataEnabled ? `Tokens on ${ config.chain.name }` : 'Tokens' }
        withTextAd
      />
      <TokenCategoryFilter value={ category } onChange={ handleCategoryChange } withBridged={ bridgedTokensFeature.isEnabled }/>
      { renderList(Boolean(searchTerm || tokenTypes?.length || bridgeChains?.length), tab === 'bridged' ? description : undefined) }
    </>
  );
};

const Tokens = () => {
  const router = useRouter();
  return <TokensContent key={ getQueryParamString(router.query.tab) === 'bridged' ? 'bridged' : 'all' }/>;
};

export default Tokens;
