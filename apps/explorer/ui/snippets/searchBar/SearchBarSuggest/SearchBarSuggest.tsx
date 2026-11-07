import { Box, Flex, Text } from '@chakra-ui/react';
import type { UseQueryResult } from '@tanstack/react-query';
import { debounce } from 'es-toolkit';
import React from 'react';

import type { ListCctxsResponse } from '@blockscout/zetachain-cctx-types';
import type { QuickSearchResultItem } from 'types/client/search';

import config from 'configs/app';
import multichainConfig from 'configs/multichain';
import type { ResourceError } from 'lib/api/resources';
import { useSettingsContext } from 'lib/contexts/settings';
import useIsMobile from 'lib/hooks/useIsMobile';
import type { ExternalSearchItem as ExternalSearchItemType } from 'lib/search/externalSearch';
import { Button } from 'toolkit/chakra/button';
import AdaptiveTabs from 'toolkit/components/AdaptiveTabs/AdaptiveTabs';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import * as regexp from 'toolkit/utils/regexp';
import useMarketplaceApps from 'ui/marketplace/useMarketplaceApps';
import TextAd from 'ui/shared/ad/TextAd';
import ExternalSearchItem from 'ui/shared/search/ExternalSearchItem';
import type { ApiCategory, Category, ItemsCategoriesMap } from 'ui/shared/search/utils';
import { getItemCategory, searchCategories } from 'ui/shared/search/utils';

import type { useSearchAssistant } from '../useSearchQuery';

import SearchBarSuggestApp from './SearchBarSuggestApp';
import SearchBarSuggestBlockCountdown from './SearchBarSuggestBlockCountdown';
import SearchBarSuggestItem from './SearchBarSuggestItem';
import SearchBarSuggestZetaChainCCTX from './SearchBarSuggestZetaChainCCTX';

const TABS_HEIGHT = 52;
const PROMPT_ORDER: Array<Category> = [ 'transaction', 'address', 'block', 'token', 'app' ];
const PROMPT_CATEGORY: Partial<Record<Category, ApiCategory>> = {
  nft: 'token', confidential_token: 'token', public_tag: 'address', domain: 'address', cluster: 'address',
  user_operation: 'transaction', tac_operation: 'transaction',
};
const PROMPT_LABELS: Partial<Record<Category, string>> = {
  transaction: 'TRANSACTIONS', address: 'ADDRESSES', block: 'BLOCKS', token: 'TOKENS', app: 'APPS',
};

interface Props {
  assistant?: ReturnType<typeof useSearchAssistant>;
  query: UseQueryResult<Array<QuickSearchResultItem>, ResourceError<unknown>>;
  zetaChainCCTXQuery: UseQueryResult<ListCctxsResponse, ResourceError<unknown>>;
  externalSearchItem: ExternalSearchItemType;
  searchTerm: string;
  onItemClick: (event: React.MouseEvent<HTMLAnchorElement>) => void;
  category?: Category;
}

const SearchBarSuggest = ({ query, zetaChainCCTXQuery, externalSearchItem, searchTerm, onItemClick, category, assistant }: Props) => {
  const isMobile = useIsMobile();

  const scrollContainerRef = React.useRef<HTMLDivElement>(null);
  const marketplaceApps = useMarketplaceApps(searchTerm);
  const settingsContext = useSettingsContext();

  const categoriesRefs = React.useRef(new Map<Category, HTMLParagraphElement>());

  const [ currentTab, setCurrentTab ] = React.useState<Category | undefined>(undefined);

  const visibleCategories = React.useMemo(() => {
    const categories = category ? searchCategories.filter((cat) => cat.id === category) : searchCategories;
    const rank = (id: Category) => PROMPT_ORDER.includes(id) ? PROMPT_ORDER.indexOf(id) : PROMPT_ORDER.length;
    return categories.map(cat => ({ ...cat, title: PROMPT_LABELS[PROMPT_CATEGORY[cat.id] ?? cat.id] ?? cat.title.toUpperCase() }))
      .sort((a, b) => rank(a.id) - rank(b.id));
  }, [ category ]);

  const handleScroll = React.useCallback(() => {
    const container = scrollContainerRef.current;
    if (!container) {
      return;
    }
    const headings: Array<{ id: Category; element: HTMLParagraphElement }> = [];
    visibleCategories.forEach(({ id }) => {
      const element = categoriesRefs.current.get(id);
      if (element instanceof HTMLParagraphElement && element.isConnected) {
        headings.push({ id, element });
      }
    });
    const lastCategory = headings[headings.length - 1];
    if (!lastCategory) {
      return;
    }
    const topLimit = container.getBoundingClientRect().y + TABS_HEIGHT;
    if (lastCategory.element.getBoundingClientRect().y <= topLimit) {
      setCurrentTab(lastCategory.id);
      return;
    }
    for (let i = 0; i < headings.length - 1; i++) {
      if (headings[i].element.getBoundingClientRect().y <= topLimit && headings[i + 1].element.getBoundingClientRect().y > topLimit) {
        setCurrentTab(headings[i].id);
        break;
      }
    }
  }, [ visibleCategories ]);

  const debouncedHandleScroll = React.useMemo(() => debounce(handleScroll, 300), [ handleScroll ]);

  React.useEffect(() => () => debouncedHandleScroll.cancel(), [ debouncedHandleScroll ]);

  const itemsGroups = React.useMemo(() => {
    if (!query.data && !zetaChainCCTXQuery.data?.items.length && !marketplaceApps.displayedApps) {
      return {};
    }

    const map: Partial<ItemsCategoriesMap> = {};

    query.data?.forEach(item => {
      const itemCategory = getItemCategory(item) as ApiCategory;
      const cat = category ? itemCategory : PROMPT_CATEGORY[itemCategory] ?? itemCategory;
      if (cat) {
        if (cat in map) {
          map[cat]?.push(item);
        } else {
          map[cat] = [ item ];
        }
      }
    });

    if (marketplaceApps.displayedApps.length) {
      map.app = marketplaceApps.displayedApps;
    }

    if (zetaChainCCTXQuery.data?.items.length) {
      map.zetaChainCCTX = zetaChainCCTXQuery.data.items;
    }

    if (Object.keys(map).length > 0 && !map.block && regexp.BLOCK_HEIGHT.test(searchTerm) && !multichainConfig()) {
      map['block'] = [ {
        type: 'block',
        block_type: 'block',
        block_number: searchTerm,
        block_hash: '',
        timestamp: undefined,
      } ];
    }

    return map;
  }, [ query.data, marketplaceApps.displayedApps, searchTerm, zetaChainCCTXQuery.data?.items, category ]);

  React.useEffect(() => {
    const resultCategories = visibleCategories.filter(cat => itemsGroups[cat.id]);
    setCurrentTab(resultCategories[0]?.id);
  }, [ itemsGroups, visibleCategories ]);

  const handleTabsValueChange = React.useCallback(({ value }: { value: string }) => {
    setCurrentTab(value as Category);
    const container = scrollContainerRef.current;
    const targetElement = document.querySelector(`[data-scroll-target="cat_${ value }"]`);

    if (container && targetElement) {
      const containerRect = container.getBoundingClientRect();
      const targetRect = targetElement.getBoundingClientRect();
      const scrollTop = targetRect.top - containerRect.top + container.scrollTop;

      container.scrollTo({
        top: scrollTop,
        behavior: 'smooth',
      });
    }
  }, []);

  const categoryTabs = React.useMemo(() => {
    return visibleCategories.filter(cat => itemsGroups[cat.id]).map(cat => ({
      id: cat.id,
      value: cat.id,
      title: isMobile ? cat.tabTitle : cat.title,
      component: null,
    }));
  }, [ itemsGroups, isMobile, visibleCategories ]);

  const handleTabsFocusChange = React.useCallback(({ focusedValue }: { focusedValue: string | null }) => {
    if (focusedValue && focusedValue !== currentTab && categoryTabs.some(tab => tab.value === focusedValue)) {
      handleTabsValueChange({ value: focusedValue });
    }
  }, [ categoryTabs, currentTab, handleTabsValueChange ]);

  const content = (() => {
    if (query.isPending || marketplaceApps.isPlaceholderData || (config.features.zetachain.isEnabled && zetaChainCCTXQuery.isPending)) {
      return <ContentLoader text="We are searching, please wait... " fontSize="sm" maxW="250px"/>;
    }

    if (query.isError) {
      return <Text>Something went wrong. Try refreshing the page or come back later.</Text>;
    }

    const resultCategories = visibleCategories.filter(cat => itemsGroups[cat.id]);

    if (resultCategories.length === 0 && !externalSearchItem) {
      if (regexp.BLOCK_HEIGHT.test(searchTerm)) {
        return <SearchBarSuggestBlockCountdown blockHeight={ searchTerm } isMultichain={ Boolean(multichainConfig()) } onClick={ onItemClick }/>;
      }

      return <Text>No results found.</Text>;
    }

    return (
      <>
        { resultCategories.length > 1 && (
          <AdaptiveTabs
            tabs={ categoryTabs }
            onValueChange={ handleTabsValueChange }
            onFocusChange={ handleTabsFocusChange }
            defaultValue={ currentTab }
            variant="secondary"
            size="sm"
            pb={ 5 }
            w="100%"
            overflowX="hidden"
            minH="52px"
            h="52px"
            listProps={{
              overflowX: 'auto',
              mb: 0,
              mt: 0,
              bgColor: 'raised',
            }}
          />
        ) }
        <Flex flexDirection="column" overflowY="auto" ref={ scrollContainerRef } onScroll={ debouncedHandleScroll }>
          { resultCategories.map((cat, index) => {
            return (
              <Box key={ cat.id } data-scroll-target={ `cat_${ cat.id }` }>
                <Text
                  textStyle="label"
                  role="heading"
                  aria-level={ 3 }
                  mt={ index === 0 ? 1 : 4 }
                  mb={ 2 }
                  ref={ (el: HTMLParagraphElement | null) => {
                    if (el) {
                      categoriesRefs.current.set(cat.id, el);
                    } else {
                      categoriesRefs.current.delete(cat.id);
                    }
                  } }
                  data-id={ cat.id }
                >
                  { cat.title }
                </Text>
                { cat.id !== 'app' && cat.id !== 'zetaChainCCTX' && itemsGroups[cat.id]?.map((item, index) => (
                  <SearchBarSuggestItem
                    key={ index }
                    data={ item }
                    isMobile={ isMobile }
                    searchTerm={ searchTerm }
                    onClick={ onItemClick }
                    addressFormat={ settingsContext?.addressFormat }
                  />
                )) }
                { cat.id === 'app' && itemsGroups[cat.id]?.map((item, index) =>
                  <SearchBarSuggestApp key={ index } data={ item } isMobile={ isMobile } searchTerm={ searchTerm } onClick={ onItemClick }/>,
                ) }
                { cat.id === 'zetaChainCCTX' && itemsGroups[cat.id]?.map((item, index) =>
                  <SearchBarSuggestZetaChainCCTX key={ index } data={ item } isMobile={ isMobile } searchTerm={ searchTerm } onClick={ onItemClick }/>,
                ) }
              </Box>
            );
          }) }
          { externalSearchItem && <ExternalSearchItem item={ externalSearchItem }/> }
        </Flex>
      </>
    );
  })();

  return (
    <>
      { config.features.assistant.isEnabled && assistant && (assistant.canAsk || assistant.answer || assistant.error) && (
        <Box bgColor="raised" color="text.primary" borderColor="hairline" borderBottomWidth="1px" pb={ 3 } mb={ 3 }>
          <Text textStyle="label" role="heading" aria-level={ 3 } mb={ 2 }>ASK</Text>
          <Button
            type="button"
            variant="plain"
            color="text.primary"
            w="100%"
            justifyContent="flex-start"
            whiteSpace="normal"
            height="auto"
            minH="44px"
            textAlign="left"
            disabled={ assistant.pending }
            onClick={ assistant.ask }
          >
            { assistant.pending ? 'Asking…' : `Ask: ${ assistant.question }` }
          </Button>
          <Box role="status" aria-live="polite" overflowY="auto" maxH="30vh">
            { assistant.answer && (
              <>
                <Text whiteSpace="pre-wrap" overflowWrap="anywhere">{ assistant.answer.answer }</Text>
                <Text color="text.muted" textStyle="xs" mt={ 2 }>
                  model output, not chain data · { assistant.answer.model }
                </Text>
              </>
            ) }
          </Box>
          { assistant.error && <Text role="alert" color="text.muted">{ assistant.error }</Text> }
        </Box>
      ) }
      { !isMobile && (
        <Box pb={ 4 } mb={ 5 } borderColor="hairline" borderBottomWidth="1px" _empty={{ display: 'none' }}>
          <TextAd textStyle={{ lg: 'sm' }}/>
        </Box>
      ) }
      { content }
    </>
  );
};

export default SearchBarSuggest;
