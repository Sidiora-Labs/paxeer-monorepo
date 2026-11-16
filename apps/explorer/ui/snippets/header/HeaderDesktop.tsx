import { Box, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import NavigationDesktop from 'ui/snippets/navigation/horizontal/NavigationDesktop';
import NetworkIcon from 'ui/snippets/networkLogo/NetworkIcon';
import SearchBar from 'ui/snippets/searchBar/SearchBarDesktop';
import TopBar from 'ui/snippets/topBar/TopBar';
import UserProfileDesktop from 'ui/snippets/user/UserProfileDesktop';

type Props = {
  renderSearchBar?: () => React.ReactNode;
};

const HeaderDesktop = ({ renderSearchBar }: Props) => {
  return (
    <Box
      as="header"
      data-label="desktop-header"
      display={{ base: 'none', lg: 'block' }}
      position="sticky"
      top={ 0 }
      zIndex="sticky"
      bg="header.sticky.bg"
      backdropFilter="blur(var(--chakra-blurs-header-bar))"
      borderBottomWidth="var(--chakra-border-widths-1)"
      borderColor="hairline"
    >
      <Flex
        minH="var(--chakra-spacing-14)"
        py={ 1 }
        flexWrap="wrap"
        maxW={ config.UI.maxContentWidth ? 'calc(var(--chakra-spacing-80) * 4)' : 'none' }
        mx="auto"
        px={ 6 }
        align="center"
        gap={ 3 }
      >
        <Flex data-label="header-brand" align="center" gap={ 2 } flexShrink={ 0 }>
          <NetworkIcon/>
          <Text
            textStyle="body"
            fontWeight="semibold"
            maxW={{ lg: 'var(--chakra-spacing-20)', xl: 'calc(var(--chakra-spacing-30) + var(--chakra-spacing-5))' }}
            lineClamp={ 2 }
          >{ config.chain.name }</Text>
        </Flex>
        <Box mx="auto" minW={ 0 }><NavigationDesktop/></Box>
        <Flex data-label="header-controls" align="center" gap={ 2 } flex="1" minW={ 0 } flexWrap="wrap" justify="flex-end">
          <Box data-label="header-search" w="var(--chakra-spacing-80)" maxW="100%" minW={ 0 }>
            { renderSearchBar ? renderSearchBar() : <SearchBar compact/> }
          </Box>
          <TopBar/>
          <UserProfileDesktop buttonSize="sm" buttonVariant="header"/>
        </Flex>
      </Flex>
    </Box>
  );
};

export default React.memo(HeaderDesktop);
