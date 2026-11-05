import { Box, Flex, Text } from '@chakra-ui/react';
import React from 'react';

import config from 'configs/app';
import { CONTENT_MAX_WIDTH } from 'ui/shared/layout/utils';
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
      backdropFilter="blur(12px)"
      borderBottomWidth="1px"
      borderColor="hairline"
    >
      <Flex h="56px" maxW={ `${ CONTENT_MAX_WIDTH }px` } mx="auto" px={ 6 } align="center" gap={ 3 }>
        <Flex data-label="header-brand" align="center" gap={ 2 } flexShrink={ 0 }>
          <NetworkIcon/>
          <Text textStyle="body" fontWeight={ 600 } maxW={{ lg: '80px', xl: '140px' }} lineClamp={ 2 }>{ config.chain.name }</Text>
        </Flex>
        <Box mx="auto"><NavigationDesktop/></Box>
        <Flex data-label="header-controls" align="center" gap={ 2 } flexShrink={ 0 }>
          <Box data-label="header-search" w="320px">
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
