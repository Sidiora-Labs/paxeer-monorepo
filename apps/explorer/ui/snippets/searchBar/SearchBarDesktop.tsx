import { Box } from '@chakra-ui/react';
import { useRouter } from 'next/router';
import type { FormEvent } from 'react';
import React from 'react';

import { route } from 'nextjs-routes';

import config from 'configs/app';
import * as mixpanel from 'lib/mixpanel/index';
import { getRecentSearchKeywords, saveToRecentKeywords } from 'lib/recentSearchKeywords';
import { Button } from 'toolkit/chakra/button';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import type { Category } from 'ui/shared/search/utils';
import NativePopover from 'ui/snippets/overlays/NativePopover';

import SearchBarBackdrop from './SearchBarBackdrop';
import SearchBarInput from './SearchBarInput';
import SearchBarRecentKeywords from './SearchBarRecentKeywords';
import SearchBarSuggest from './SearchBarSuggest/SearchBarSuggest';
import { useSearchAssistant } from './useSearchQuery';
import useSearchShortcut from './useSearchShortcut';
import useSearchWithClusters from './useSearchWithClusters';
import { getSearchRedirectRoute, useSearchRedirect, useSearchRequestGuard } from './utils';

const paxeerXFeature = config.features.paxeerXLists;

type Props = {
  isHeroBanner?: boolean;
  compact?: boolean;
  category?: Category;
};

const SearchBarDesktop = ({ isHeroBanner, category, compact }: Props) => {
  const inputRef = React.useRef<HTMLFormElement>(null);
  const panelId = React.useId();
  const anchorName = `--search-${ panelId.replace(/[^\w-]/g, '') }`;

  const { open, onClose, onOpen } = useDisclosure();
  const router = useRouter();

  const recentSearchKeywords = getRecentSearchKeywords();

  const search = useSearchWithClusters();
  const { searchTerm, debouncedSearchTerm, handleSearchTermChange, query, zetaChainCCTXQuery, externalSearchItem } = search;
  const assistant = useSearchAssistant(search);
  const resolveSearchRoute = useSearchRedirect();
  const searchGuard = useSearchRequestGuard();
  const searchTermRef = React.useRef(searchTerm);

  React.useEffect(() => {
    searchTermRef.current = searchTerm;
  }, [ searchTerm ]);

  const handleTermChange = React.useCallback((value: string) => {
    searchGuard.discard();
    searchTermRef.current = value;
    handleSearchTermChange(value);
  }, [ searchGuard, handleSearchTermChange ]);

  const navigateToResults = React.useCallback(async(redirect: boolean) => {
    if (searchTerm) {
      const isCurrentRequest = searchGuard.start(searchTerm);
      const resultRoute = paxeerXFeature.isEnabled ?
        await resolveSearchRoute(searchTerm, redirect) :
        getSearchRedirectRoute(searchTerm, redirect);

      if (!isCurrentRequest(searchTermRef.current)) {
        return;
      }

      const url = route(resultRoute);
      mixpanel.logEvent(mixpanel.EventTypes.SEARCH_QUERY, {
        'Search query': searchTerm,
        'Source page type': mixpanel.getPageType(router.pathname),
        'Result URL': url,
      });
      saveToRecentKeywords(searchTerm);
      router.push(resultRoute, undefined, { shallow: resultRoute.pathname === '/search-results' });
    }
  }, [ searchTerm, router, resolveSearchRoute, searchGuard ]);

  const handleSubmit = React.useCallback((event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (assistant.canAsk) {
      searchGuard.discard();
      void assistant.ask();
      onOpen();
      return;
    }
    navigateToResults(true);
  }, [ assistant, navigateToResults, searchGuard, onOpen ]);

  const handleViewAllResultsClick = React.useCallback(() => {
    navigateToResults(false);
  }, [ navigateToResults ]);

  const handleFocus = React.useCallback(() => {
    onOpen();
  }, [ onOpen ]);

  const focusSearch = React.useCallback(() => {
    inputRef.current?.querySelector('input')?.focus();
    onOpen();
  }, [ onOpen ]);

  useSearchShortcut(inputRef, focusSearch);

  const handleDismiss = React.useCallback(() => {
    searchGuard.discard();
    onClose();
  }, [ searchGuard, onClose ]);

  const handleOpenChange = React.useCallback(({ open }: { open: boolean }) => {
    if (open) onOpen();
    else handleDismiss();
  }, [ onOpen, handleDismiss ]);

  const handleClear = React.useCallback(() => {
    handleTermChange('');
    inputRef.current?.querySelector('input')?.focus();
  }, [ handleTermChange ]);

  const handleItemClick = React.useCallback((event: React.MouseEvent<HTMLAnchorElement>) => {
    searchGuard.discard();
    mixpanel.logEvent(mixpanel.EventTypes.SEARCH_QUERY, {
      'Search query': searchTerm,
      'Source page type': mixpanel.getPageType(router.pathname),
      'Result URL': event.currentTarget.href,
    });
    saveToRecentKeywords(searchTerm);
    onClose();
  }, [ searchGuard, router.pathname, searchTerm, onClose ]);

  const routePath = router.asPath?.split('?')?.[0];
  React.useEffect(() => {
    handleTermChange('');
  }, [ routePath, handleTermChange ]);

  const showAllResultsLink = searchTerm.trim().length > 0 && (
    (query.data && query.data.length >= 50) ||
    (zetaChainCCTXQuery.data && zetaChainCCTXQuery.data?.items.length > 10)
  );

  const isSuggestOpen = open && (searchTerm.trim().length > 0 || recentSearchKeywords.length > 0);

  return (
    <>
      <SearchBarInput
        ref={ inputRef }
        onChange={ handleTermChange }
        onSubmit={ handleSubmit }
        onFocus={ handleFocus }
        onHide={ handleDismiss }
        onClear={ handleClear }
        isHeroBanner={ isHeroBanner }
        compact={ compact }
        value={ searchTerm }
        isSuggestOpen={ isSuggestOpen }
        suggestionId={ panelId }
        css={{ anchorName }}
      />
      <NativePopover
        id={ panelId }
        aria-label="Search suggestions"
        open={ isSuggestOpen }
        onOpenChange={ handleOpenChange }
        anchorRef={ inputRef }
        anchorName={ anchorName }
        matchAnchorWidth
        w={ isHeroBanner && !compact ? 'calc(var(--chakra-spacing-80) * 2)' : 'var(--chakra-spacing-80)' }
        p={ 4 }
      >
        <Box color="text.primary" maxH="50dvh" minH={ 0 } display="flex" flexDirection="column" overflowY="auto">
          { searchTerm.trim().length === 0 && recentSearchKeywords.length > 0 && (
            <SearchBarRecentKeywords onClick={ handleTermChange } onClear={ handleDismiss }/>
          ) }
          { searchTerm.trim().length > 0 && (
            <SearchBarSuggest
              assistant={ assistant }
              query={ query }
              searchTerm={ debouncedSearchTerm }
              onItemClick={ handleItemClick }
              zetaChainCCTXQuery={ zetaChainCCTXQuery }
              externalSearchItem={ externalSearchItem }
              category={ category }
            />
          ) }
        </Box>
        { showAllResultsLink && (
          <Box pt={ 2 } borderTopWidth="var(--chakra-border-widths-1)" borderColor="hairline">
            <Button
              variant="link"
              minH="touchTarget"
              minW="touchTarget"
              textStyle="sm"
              onClick={ handleViewAllResultsClick }
            >
              View all results
            </Button>
          </Box>
        ) }
      </NativePopover>
      <SearchBarBackdrop isOpen={ isSuggestOpen }/>
    </>
  );
};

export default SearchBarDesktop;
