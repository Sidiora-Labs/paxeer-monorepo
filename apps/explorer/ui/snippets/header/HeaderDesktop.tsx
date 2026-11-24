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
        data-label="header-row"
        minH="var(--chakra-spacing-14)"
        py={ 0 }
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
            maxW={{ lg: 'var(--chakra-spacing-20)', xl: 'calc(var(--chakra-spacing-40) - var(--chakra-spacing-5))' }}
            textWrap="balance"
          >{ config.chain.name }</Text>
        </Flex>
        <Box data-label="header-menu" flexShrink={ 0 } mx="auto"><NavigationDesktop/></Box>
        <Box
          data-label="header-search"
          flex="1 1 100%"
          minW={ 0 }
          maxW="100%"
          order={ 1 }
          css={{
            '& form': { width: '100%' },
            '@media (min-width: 1280px)': {
              order: 0,
              flex: '0 1 var(--chakra-spacing-80)',
              minWidth: 'var(--chakra-spacing-62\\.5)',
              maxWidth: 'var(--chakra-spacing-80)',
            },
          }}
        >
          { renderSearchBar ? renderSearchBar() : <SearchBar compact/> }
        </Box>
        <Flex data-label="header-controls" align="center" gap={ 2 } flexShrink={ 0 } ml={ 0 } flexWrap="nowrap"
          css={{ '@media (min-width: 1280px)': { marginInlineStart: 'calc(-1 * var(--chakra-spacing-1))' } }}>
          <TopBar/>
          <UserProfileDesktop buttonSize="sm" buttonVariant="header"/>
        </Flex>
      </Flex>
    </Box>
  );
};

export default React.memo(HeaderDesktop);
