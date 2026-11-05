import { Box, Flex, chakra } from '@chakra-ui/react';
import { pickBy } from 'es-toolkit';
import { useRouter } from 'next/router';
import React from 'react';

import { TabsContent, TabsList, TabsRoot, TabsTrigger } from '../../chakra/tabs';
import type { Props as AdaptiveTabsProps } from '../AdaptiveTabs/AdaptiveTabs';
import AdaptiveTabs from '../AdaptiveTabs/AdaptiveTabs';
import { getTabValue } from '../AdaptiveTabs/utils';
import useActiveTabFromQuery from './useActiveTabFromQuery';

export interface RoutedTabsProps extends AdaptiveTabsProps {
  preservedParams?: Array<string>;
  displayMode?: 'adaptive' | 'entity';
  defaultTabId?: string;
}

const RoutedTabs = (props: RoutedTabsProps) => {
  const { tabs, defaultTabId, onValueChange, preservedParams, displayMode = 'adaptive', ...rest } = props;

  const router = useRouter();
  const activeTab = useActiveTabFromQuery(tabs, defaultTabId);
  const tabsRef = React.useRef<HTMLDivElement>(null);

  const handleValueChange = React.useCallback(({ value }: { value: string }) => {
    const nextTab = tabs.find((tab) => getTabValue(tab) === value);

    if (!nextTab) {
      return;
    }

    const queryForPathname = pickBy(router.query, (_, key) => {
      if (preservedParams?.includes(String(key))) {
        return true;
      }

      return router.pathname.includes(`[${ key }]`);
    });
    router.push(
      { pathname: router.pathname, query: { ...queryForPathname, tab: value } },
      undefined,
      { shallow: true },
    );

    onValueChange?.({ value });
  }, [ tabs, router, onValueChange, preservedParams ]);

  React.useEffect(() => {
    if (router.query.scroll_to_tabs) {
      tabsRef?.current?.scrollIntoView(true);
      delete router.query.scroll_to_tabs;
      router.push(
        {
          pathname: router.pathname,
          query: router.query,
        },
        undefined,
        { shallow: true },
      );
    }
  // replicate componentDidMount
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  if (displayMode === 'entity') {
    return (
      <TabsRoot
        ref={ tabsRef }
        value={ getTabValue(activeTab ?? tabs[0]) }
        onValueChange={ handleValueChange }
        variant="pill"
        size={ props.size ?? 'sm' }
        data-entity-tabs
      >
        <Flex alignItems="center" gap={ 3 } data-scan-section-tabs minW={ 0 }>
          <Box overflowX="auto" minW={ 0 } flex="1">
            <TabsList flexWrap="nowrap" aria-label="Entity sections">
              { tabs.map((tab) => (
                <TabsTrigger key={ getTabValue(tab) } value={ getTabValue(tab) } disabled={ props.isLoading } data-tab={ getTabValue(tab) }>
                  <span data-tab-label>{ typeof tab.title === 'function' ? tab.title() : tab.title }</span>
                  { tab.count !== null && tab.count !== undefined && (
                    <chakra.span color="text.muted" fontFamily="mono" data-tab-count>({ tab.count.toLocaleString() })</chakra.span>
                  ) }
                </TabsTrigger>
              )) }
            </TabsList>
          </Box>
          { props.rightSlot && <Flex alignItems="center" gap={ 2 } data-right-slot>{ props.rightSlot }</Flex> }
        </Flex>
        { tabs.map((tab) => (
          <TabsContent key={ getTabValue(tab) } value={ getTabValue(tab) } pt={ 6 }>
            { tab.component }
          </TabsContent>
        )) }
      </TabsRoot>
    );
  }

  return (
    <AdaptiveTabs
      { ...rest }
      tabs={ tabs }
      onValueChange={ handleValueChange }
      defaultValue={ activeTab ? getTabValue(activeTab) : getTabValue(tabs[ 0 ]) }
    />
  );
};

export default React.memo(RoutedTabs);
