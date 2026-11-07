import { Box, Text } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import SearchBar from 'ui/snippets/searchBar/SearchBarDesktop';
import SearchBarMobile from 'ui/snippets/searchBar/SearchBarMobile';

import PromotionalBanners from './PromotionalBanners';

export const BACKGROUND_DEFAULT = 'canvas';

const HeroBanner = () => {
  const title = config.meta.seo.enhancedDataEnabled ?
    `${ config.chain.name } blockchain explorer` :
    `${ config.chain.name } explorer`;

  return (
    <>
      <Box as="section" data-label="hero" bg={ BACKGROUND_DEFAULT } w="100%" py={{ base: 8, lg: 10 }}>
        <Box as="h1" data-label="hero-title" textStyle="display" color="text.primary">{ title }</Box>
        <Text mt={ 3 } textStyle="body" color="text.secondary">
          Explore blocks, transactions, addresses and tokens on { config.chain.name }.
        </Text>
        <Box data-label="hero-search" mt={ 6 } w="100%" maxW="640px">
          <Box display={{ base: 'block', lg: 'none' }}>
            <SearchBarMobile isHeroBanner/>
          </Box>
          <Box data-label="hero-search-desktop" display={{ base: 'none', lg: 'block' }}>
            <SearchBar isHeroBanner/>
          </Box>
        </Box>
      </Box>
      <PromotionalBanners/>
    </>
  );
};

export default React.memo(HeroBanner);
