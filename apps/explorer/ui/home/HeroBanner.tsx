import { Box, createListCollection, Flex } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import { Button } from 'toolkit/chakra/button';
import { useColorModeValue } from 'toolkit/chakra/color-mode';
import { Heading } from 'toolkit/chakra/heading';
import type { SelectOption } from 'toolkit/chakra/select';
import { Select } from 'toolkit/chakra/select';
import RewardsButton from 'ui/rewards/RewardsButton';
import IconSvg from 'ui/shared/IconSvg';
import type { Category } from 'ui/shared/search/utils';
import { searchCategories } from 'ui/shared/search/utils';
import SearchBar from 'ui/snippets/searchBar/SearchBarDesktop';
import SearchBarMobile from 'ui/snippets/searchBar/SearchBarMobile';
import UserProfileDesktop from 'ui/snippets/user/UserProfileDesktop';

import PromotionalBanners from './PromotionalBanners';

export const BACKGROUND_DEFAULT =
  'radial-gradient(103.03% 103.03% at 0% 0%, rgba(183, 148, 244, 0.8) 0%, rgba(0, 163, 196, 0.8) 100%), var(--chakra-colors-blue-400)';

const ALL_FILTERS = 'all';
const FILTER_WIDTH = '168px';
const FILTER_CATEGORIES: Array<Category> = [ 'address', 'token', 'nft', 'public_tag', 'transaction', 'block' ];
const FILTER_CSS = { '--select-trigger-height': 'sizes.10' };

const filterCollection = createListCollection<SelectOption>({
  items: [
    { value: ALL_FILTERS, label: 'All filters' },
    ...searchCategories
      .filter((category) => FILTER_CATEGORIES.includes(category.id))
      .map((category) => ({ value: category.id, label: category.tabTitle })),
  ],
});

const HeroBanner = () => {
  const heroConfig = config.UI.homepage.heroBanner;
  const background = useColorModeValue(heroConfig?.background?.[0], heroConfig?.background?.[1] ?? heroConfig?.background?.[0]);
  const textColor = useColorModeValue(heroConfig?.text_color?.[0], heroConfig?.text_color?.[1] ?? heroConfig?.text_color?.[0]);
  const border = useColorModeValue(heroConfig?.border?.[0], heroConfig?.border?.[1] ?? heroConfig?.border?.[0]);
  const searchRef = React.useRef<HTMLDivElement>(null);
  const [ filter, setFilter ] = React.useState<string>(ALL_FILTERS);

  const title = config.meta.seo.enhancedDataEnabled ?
    `${ config.chain.name } blockchain explorer` :
    `${ config.chain.name } explorer`;

  const handleSubmitClick = React.useCallback(() => {
    searchRef.current?.querySelector('form')?.requestSubmit();
  }, []);

  const handleFilterChange = React.useCallback(({ value }: { value: Array<string> }) => {
    setFilter(value[0] ?? ALL_FILTERS);
  }, []);

  const filterValue = React.useMemo(() => [ filter ], [ filter ]);
  const category = filter === ALL_FILTERS ? undefined : filter as Category;

  return (
    <>
      <Box as="section" data-label="hero" w="100%" pb={{ base: 4, lg: 8 }} style={{ background, border, color: textColor }}>
        <Flex mb={{ base: 3, lg: 5 }} justifyContent="space-between" alignItems="center" columnGap={ 2 }>
          <Heading level="1" data-label="hero-title" style={{ color: textColor }}>{ title }</Heading>
          { config.UI.navigation.layout === 'vertical' && (
            <Box display={{ base: 'none', lg: 'flex' }} gap={ 2 }>
              { config.features.rewards.isEnabled && <RewardsButton variant="hero"/> }
              <UserProfileDesktop buttonVariant="hero"/>
            </Box>
          ) }
        </Flex>
        <Flex data-label="hero-search" alignItems="center" columnGap={ 2 } w="100%">
          <Box display={{ base: 'flex', lg: 'none' }} flexGrow={ 1 } minW={ 0 }>
            <SearchBarMobile isHeroBanner/>
          </Box>
          <Box data-label="hero-search-filter" display={{ base: 'none', lg: 'block' }} flexShrink={ 0 } w={ FILTER_WIDTH }>
            <Select
              collection={ filterCollection }
              placeholder="All filters"
              value={ filterValue }
              onValueChange={ handleFilterChange }
              css={ FILTER_CSS }
            />
          </Box>
          <Box ref={ searchRef } data-label="hero-search-desktop" display={{ base: 'none', lg: 'flex' }} flexGrow={ 1 } minW={ 0 }>
            <SearchBar isHeroBanner category={ category }/>
          </Box>
          <Button
            data-label="hero-search-submit"
            display={{ base: 'none', lg: 'inline-flex' }}
            variant="solid"
            size="md"
            aria-label="Search"
            onClick={ handleSubmitClick }
          >
            <IconSvg name="search" boxSize={ 5 }/>
          </Button>
        </Flex>
      </Box>
      <PromotionalBanners/>
    </>
  );
};

export default React.memo(HeroBanner);
