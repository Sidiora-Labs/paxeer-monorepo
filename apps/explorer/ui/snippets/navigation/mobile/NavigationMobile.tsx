import { Box, Flex, VStack } from '@chakra-ui/react';
import React from 'react';

import useNavItems from 'lib/hooks/useNavItems';
import useIsAuth from 'ui/snippets/auth/useIsAuth';
import TopBar from 'ui/snippets/topBar/TopBar';
import UserProfileDesktop from 'ui/snippets/user/UserProfileDesktop';

import NavLink from '../horizontal/NavLink';
import NavigationPromoBanner from '../promoBanner/NavigationPromoBanner';
import NavLinkRewards from '../vertical/NavLinkRewards';
import NavLinkGroup from './NavLinkGroup';

interface Props {
  onNavLinkClick?: () => void;
  isMarketplaceAppPage?: boolean;
}

const NavigationMobile = ({ onNavLinkClick, isMarketplaceAppPage }: Props) => {
  const { shellNavItems, accountNavItems } = useNavItems();
  const [ openedGroup, setOpenedGroup ] = React.useState<string | null>(null);
  const isAuth = useIsAuth();
  const handleGroupClick = React.useCallback((text: string) => {
    setOpenedGroup((current) => current === text ? null : text);
  }, []);

  return (
    <Flex flexDirection="column" flexGrow={ 1 } gap={ 4 } mt={ 4 }>
      <Box as="nav" aria-label="Mobile navigation">
        <VStack as="ul" gap={ 1 } alignItems="stretch">
          { shellNavItems.map((item) => (
            <NavLinkGroup
              key={ item.text }
              item={ item }
              onClick={ handleGroupClick }
              isExpanded={ openedGroup === item.text }
              onNavLinkClick={ onNavLinkClick }
            />
          )) }
        </VStack>
      </Box>
      { isAuth && (
        <Box as="nav" aria-label="Account navigation" pt={ 3 } borderTopWidth="1px" borderColor="hairline">
          <VStack as="ul" gap={ 1 } alignItems="stretch">
            <NavLinkRewards onClick={ onNavLinkClick } isCollapsed={ false }/>
            { accountNavItems.map((item) => <NavLink key={ item.text } item={ item } onClick={ onNavLinkClick }/>) }
          </VStack>
        </Box>
      ) }
      <Flex align="center" gap={ 3 } flexWrap="wrap">
        <TopBar/>
        <UserProfileDesktop buttonSize="sm"/>
      </Flex>
      <NavigationPromoBanner isCollapsed={ isMarketplaceAppPage ? false : undefined }/>
    </Flex>
  );
};

export default NavigationMobile;
