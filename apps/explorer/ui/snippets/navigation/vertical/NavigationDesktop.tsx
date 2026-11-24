import { Flex, Box, VStack } from '@chakra-ui/react';
import React from 'react';

import { useAppContext } from 'lib/contexts/app';
import * as cookies from 'lib/cookies';
import useNavItems, { isGroupItem } from 'lib/hooks/useNavItems';
import IconSvg from 'ui/shared/IconSvg';
import useIsAuth from 'ui/snippets/auth/useIsAuth';
import NetworkIcon from 'ui/snippets/networkLogo/NetworkIcon';
import NetworkLogo from 'ui/snippets/networkLogo/NetworkLogo';

import NavigationPromoBanner from '../promoBanner/NavigationPromoBanner';
import RollupStageBadge from '../RollupStageBadge';
import TestnetBadge from '../TestnetBadge';
import NavLink from './NavLink';
import NavLinkGroup from './NavLinkGroup';
import NavLinkRewards from './NavLinkRewards';

const NavigationDesktop = () => {
  const appProps = useAppContext();
  const cookiesString = appProps.cookies;

  const isNavBarCollapsedCookie = cookies.get(cookies.NAMES.NAV_BAR_COLLAPSED, cookiesString);
  let isNavBarCollapsed;
  if (isNavBarCollapsedCookie === 'true') {
    isNavBarCollapsed = true;
  }
  if (isNavBarCollapsedCookie === 'false') {
    isNavBarCollapsed = false;
  }

  const { mainNavItems, accountNavItems } = useNavItems();

  const isAuth = useIsAuth();

  const [ isCollapsed, setCollapsedState ] = React.useState<boolean | undefined>(isNavBarCollapsed);

  const handleTogglerClick = React.useCallback(() => {
    setCollapsedState((flag) => !flag);
    cookies.set(cookies.NAMES.NAV_BAR_COLLAPSED, isCollapsed ? 'false' : 'true');
  }, [ isCollapsed ]);

  const handleContainerClick = React.useCallback((event: React.MouseEvent) => {
    if (event.target === event.currentTarget) {
      handleTogglerClick();
    }
  }, [ handleTogglerClick ]);

  const isExpanded = isCollapsed === false;
  const expandedWidth = 'calc(var(--chakra-spacing-40) + var(--chakra-spacing-16) + var(--chakra-spacing-1) + var(--chakra-spacing-0\\.25))';
  const collapsedWidth = 'calc(var(--chakra-spacing-20) + var(--chakra-spacing-3))';
  const expandedToggleOffset = 'calc(var(--chakra-spacing-30) + var(--chakra-spacing-24))';

  return (
    <Flex
      display={{ base: 'none', lg: 'flex' }}
      className="group"
      position="relative"
      flexDirection="column"
      alignItems="stretch"
      borderRightWidth="var(--chakra-border-widths-1)"
      borderColor="border.divider"
      px={{ lg: isExpanded ? 6 : 4, xl: isCollapsed ? 4 : 6 }}
      pt={ 12 }
      pb={ 6 }
      width={{ lg: isExpanded ? expandedWidth : collapsedWidth, xl: isCollapsed ? collapsedWidth : expandedWidth }}
      onClick={ handleContainerClick }
      transitionProperty="width, padding"
      _motionSafe={{ transitionDuration: 'normal' }}
      transitionTimingFunction="ease"
    >
      <TestnetBadge
        position="absolute"
        pl={ 3 }
        w="calc(var(--chakra-spacing-12) + var(--chakra-spacing-1) / 4)"
        top={ 'var(--chakra-spacing-8\\.5)' }
      />
      <RollupStageBadge
        position="absolute"
        ml={{ lg: isExpanded ? 3 : 'var(--chakra-spacing-2\\.5)', xl: isCollapsed ? 'var(--chakra-spacing-2\\.5)' : 3 }}
        top={ 'var(--chakra-spacing-8\\.5)' }
      />
      <Box
        as="header"
        display="flex"
        justifyContent="flex-start"
        alignItems="center"
        flexDirection="row"
        w="100%"
        pl={{ lg: isExpanded ? 3 : 'var(--chakra-spacing-3\\.75)', xl: isCollapsed ? 'var(--chakra-spacing-3\\.75)' : 3 }}
        pr={{ lg: isExpanded ? 0 : 'var(--chakra-spacing-3\\.75)', xl: isCollapsed ? 'var(--chakra-spacing-3\\.75)' : 0 }}
        h={ 10 }
        transitionProperty="padding"
        _motionSafe={{ transitionDuration: 'normal' }}
        transitionTimingFunction="ease"
      >
        <Box display={{ base: 'none', lg: isCollapsed === false ? 'block' : 'none', xl: isCollapsed ? 'none' : 'block' }}>
          <NetworkLogo/>
        </Box>
        <Box display={{ base: 'none', lg: isCollapsed === false ? 'none' : 'block', xl: isCollapsed ? 'block' : 'none' }}>
          <NetworkIcon/>
        </Box>
      </Box>
      <Box as="nav" mt={ 6 } w="100%">
        <VStack as="ul" gap="1" alignItems="flex-start">
          { mainNavItems.map((item) => {
            if (isGroupItem(item)) {
              return <NavLinkGroup key={ item.text } item={ item } isCollapsed={ isCollapsed }/>;
            } else {
              return <NavLink key={ item.text } item={ item } isCollapsed={ isCollapsed }/>;
            }
          }) }
        </VStack>
      </Box>
      { isAuth && (
        <Box as="nav" borderTopWidth="var(--chakra-border-widths-1)" borderColor="border.divider" w="100%" mt={ 3 } pt={ 3 }>
          <VStack as="ul" gap="1" alignItems="flex-start">
            <NavLinkRewards isCollapsed={ isCollapsed }/>
            { accountNavItems.map((item) => <NavLink key={ item.text } item={ item } isCollapsed={ isCollapsed }/>) }
          </VStack>
        </Box>
      ) }
      <NavigationPromoBanner isCollapsed={ isCollapsed }/>
      <IconSvg
        name="arrows/east-mini"
        width={ 6 }
        height={ 6 }
        _hover={{ color: 'hover' }}
        borderRadius="base"
        bgColor="bg.primary"
        color="icon.secondary"
        borderWidth="var(--chakra-border-widths-1)"
        borderColor="border.divider"
        transform={{ lg: isExpanded ? 'rotate(0)' : 'rotate(180deg)', xl: isCollapsed ? 'rotate(180deg)' : 'rotate(0)' }}
        transformOrigin="center"
        position="absolute"
        top="calc(var(--chakra-spacing-24) + var(--chakra-spacing-2))"
        left={{
          lg: isExpanded ? expandedToggleOffset : 'var(--chakra-spacing-20)',
          xl: isCollapsed ? 'var(--chakra-spacing-20)' : expandedToggleOffset,
        }}
        cursor="pointer"
        onClick={ handleTogglerClick }
        aria-label="Expand/Collapse menu"
        display="none"
        _groupHover={{ display: 'block' }}
        transitionProperty="transform, left"
        _motionSafe={{ transitionDuration: 'normal' }}
        transitionTimingFunction="ease"
      />
    </Flex>
  );
};

export default NavigationDesktop;
