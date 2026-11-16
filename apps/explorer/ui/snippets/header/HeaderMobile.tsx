import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import NetworkIcon from 'ui/snippets/networkLogo/NetworkIcon';

import RollupStageBadge from '../navigation/RollupStageBadge';
import TestnetBadge from '../navigation/TestnetBadge';
import SearchBarMobile from '../searchBar/SearchBarMobile';
import Burger from './Burger';

type Props = {
  hideSearchButton?: boolean;
  onGoToSearchResults?: (searchTerm: string) => void;
};

const HeaderMobile = ({ hideSearchButton, onGoToSearchResults }: Props) => {
  return (
    <Box
      as="header"
      data-label="mobile-header"
      display={{ base: 'block', lg: 'none' }}
      position="sticky"
      top={ 0 }
      zIndex="sticky2"
      bg="header.sticky.bg"
      backdropFilter="blur(var(--chakra-blurs-header-bar))"
      borderBottomWidth="var(--chakra-border-widths-1)"
      borderColor="hairline"
      minH="var(--chakra-spacing-14)"
    >
      <Flex px={ 4 } minH="var(--chakra-spacing-14)" py={ 1 } flexWrap="wrap" alignItems="center" gap={ 2 }>
        <NetworkIcon/>
        <TestnetBadge/>
        <RollupStageBadge/>
        <Flex ml="auto" gap={ 2 } align="center">
          { !hideSearchButton && <SearchBarMobile onGoToSearchResults={ onGoToSearchResults }/> }
          <Burger/>
        </Flex>
      </Flex>
    </Box>
  );
};

export default React.memo(HeaderMobile);
