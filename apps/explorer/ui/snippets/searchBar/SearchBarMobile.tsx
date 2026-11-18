import { Box } from '@chakra-ui/react';
import { useRouter } from 'next/router';
import type { FormEvent } from 'react';
import React from 'react';

import { route } from 'nextjs-routes';

import config from 'configs/app';
import * as mixpanel from 'lib/mixpanel/index';
import { getRecentSearchKeywords, saveToRecentKeywords } from 'lib/recentSearchKeywords';
import { Button } from 'toolkit/chakra/button';
import {
  DrawerRoot,
  DrawerTrigger,
  DrawerContent,
  DrawerHeader,
  DrawerTitle,
  DrawerCloseTrigger,
  DrawerBody,
  DrawerFooter,
} from 'toolkit/chakra/drawer';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import IconSvg from 'ui/shared/IconSvg';
import SearchBarInput from 'ui/snippets/searchBar/SearchBarInput';

import SearchBarRecentKeywords from './SearchBarRecentKeywords';
import SearchBarSuggest from './SearchBarSuggest/SearchBarSuggest';
import useQuickSearchQuery from './useQuickSearchQuery';
import { useSearchAssistant } from './useSearchQuery';
import useSearchShortcut from './useSearchShortcut';
import { getSearchRedirectRoute, useSearchRedirect, useSearchRequestGuard } from './utils';

const paxeerXFeature = config.features.paxeerXLists;

type Props = {
  isHeroBanner?: boolean;
  onGoToSearchResults?: (searchTerm: string) => void;
};

const SearchBarMobile = ({ isHeroBanner, onGoToSearchResults }: Props) => {
  const triggerRef = React.useRef<HTMLDivElement>(null);
  const inputRef = React.useRef<HTMLFormElement>(null);
  const router = useRouter();

  const { open, onOpen, onClose, onOpenChange } = useDisclosure();
  const search = useQuickSearchQuery();
  const { searchTerm, debouncedSearchTerm, handleSearchTermChange, query, zetaChainCCTXQuery, externalSearchItem } = search;
  const assistant = useSearchAssistant(search);
  const recentSearchKeywords = getRecentSearchKeywords();
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

  const setSearchForm = React.useCallback((form: HTMLFormElement | null) => {
    inputRef.current = form;
    if (open) {
      form?.querySelector('input')?.focus();
    }
  }, [ open ]);

  const getInitialFocus = React.useCallback(() => inputRef.current?.querySelector('input') ?? null, []);

  const focusSearch = React.useCallback(() => {
    onOpen();
    inputRef.current?.querySelector('input')?.focus();
  }, [ onOpen ]);

  useSearchShortcut(open ? inputRef : triggerRef, focusSearch);

  const handleOpenChange = React.useCallback((details: { open: boolean }) => {
    if (!details.open) {
      searchGuard.discard();
    }
    onOpenChange(details);
  }, [ searchGuard, onOpenChange ]);

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
      onGoToSearchResults?.(searchTerm);
      onClose();
    }
  }, [ searchTerm, router, onGoToSearchResults, onClose, resolveSearchRoute, searchGuard ]);

  const handleSubmit = React.useCallback((event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    if (assistant.canAsk) {
      searchGuard.discard();
      void assistant.ask();

      return;
    }
    navigateToResults(true);
  }, [ assistant, navigateToResults, searchGuard ]);

  const handleViewAllResultsClick = React.useCallback(() => {
    navigateToResults(false);
  }, [ navigateToResults ]);

  const handleItemClick = React.useCallback((event: React.MouseEvent<HTMLAnchorElement>) => {
    searchGuard.discard();
    onClose();
    mixpanel.logEvent(mixpanel.EventTypes.SEARCH_QUERY, {
      'Search query': searchTerm,
      'Source page type': mixpanel.getPageType(router.pathname),
      'Result URL': event.currentTarget.href,
    });
    saveToRecentKeywords(searchTerm);
  }, [ searchGuard, router.pathname, searchTerm, onClose ]);

  const handleClear = React.useCallback(() => {
    handleTermChange('');
    inputRef.current?.querySelector('input')?.focus();
  }, [ handleTermChange ]);

  const trigger = (
    <Button
      aria-label="Search"
      variant="header"
      flexShrink={ 0 }
      p={ isHeroBanner ? 3 : 0 }
      w={ isHeroBanner ? '100%' : 'auto' }
      minW="touchTarget"
      minH={ isHeroBanner ? 'calc(var(--chakra-spacing-12) + var(--chakra-spacing-1))' : 'touchTarget' }
      justifyContent={ isHeroBanner ? 'flex-start' : 'center' }
      bg={ isHeroBanner ? 'raised' : 'transparent' }
      borderColor="border.strong"
      borderRadius={ isHeroBanner ? 'control' : 'pill' }
      textAlign="start"
      whiteSpace="nowrap"
      overflow="visible"
      height="auto"
    >
      <IconSvg aria-hidden={ true } name="search" boxSize={ 6 } flexShrink={ 0 }/>
      { isHeroBanner && <Box as="span" textStyle="small" color="text.muted">Search or ask…</Box> }
    </Button>
  );

  return (
    <Box ref={ triggerRef } w={ isHeroBanner ? 'calc(var(--chakra-spacing-80) * 2)' : 'auto' } maxW="100%">
      <DrawerRoot
        placement="bottom"
        open={ open }
        onOpenChange={ handleOpenChange }
        initialFocusEl={ getInitialFocus }
        unmountOnExit={ false }
        lazyMount={ true }
      >
        <DrawerTrigger asChild>
          { trigger }
        </DrawerTrigger>
        <DrawerContent h="75dvh" maxH="100dvh" overflowY="hidden" bg="raised" color="text.primary" borderColor="hairline">
          <DrawerHeader mb={ 0 }>
            <DrawerTitle>Search</DrawerTitle>
            <DrawerCloseTrigger minH="touchTarget" minW="touchTarget" top={ 4 } insetEnd={ 3 }/>
          </DrawerHeader>
          <DrawerBody minH={ 0 } overflow="auto" display="flex" flexDirection="column">
            <SearchBarInput
              ref={ setSearchForm }
              onChange={ handleTermChange }
              onClear={ handleClear }
              onSubmit={ handleSubmit }
              value={ searchTerm }
              mb={ 5 }
            />
            { searchTerm.trim().length === 0 && recentSearchKeywords.length > 0 && (
              <SearchBarRecentKeywords onClick={ handleTermChange }/>
            ) }
            { searchTerm.trim().length > 0 && (
              <SearchBarSuggest
                assistant={ assistant }
                query={ query }
                searchTerm={ debouncedSearchTerm }
                onItemClick={ handleItemClick }
                zetaChainCCTXQuery={ zetaChainCCTXQuery }
                externalSearchItem={ externalSearchItem }
              />
            ) }
          </DrawerBody>
          { (query.data && query.data?.length > 0) && (
            <DrawerFooter
              borderTopWidth="var(--chakra-border-widths-1)"
              borderColor="hairline"
              pt={ 3 }
              px={ 5 }
              pb={ 5 }
              justifyContent="center"
            >
              <Button
                variant="link"
                minH="touchTarget"
                minW="touchTarget"
                onClick={ handleViewAllResultsClick }
                textStyle="sm"
              >
                View all results
              </Button>
            </DrawerFooter>
          ) }
        </DrawerContent>
      </DrawerRoot>
    </Box>
  );
};

export default SearchBarMobile;
