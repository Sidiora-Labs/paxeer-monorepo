import { chakra, Flex } from '@chakra-ui/react';
import React from 'react';

import useNavItems from 'lib/hooks/useNavItems';

import NavigationPromoBanner from '../promoBanner/NavigationPromoBanner';
import NavLinkGroup from './NavLinkGroup';

const NavigationDesktop = () => {
  const { shellNavItems } = useNavItems();

  return (
    <chakra.nav aria-label="Main navigation" display="flex" alignItems="center" gap={ 2 }>
      <Flex as="ul" gap={ 1 } alignItems="center">
        { shellNavItems.map((item) => (
          <NavLinkGroup key={ item.text } item={ item }/>
        )) }
      </Flex>
      <NavigationPromoBanner/>
    </chakra.nav>
  );
};

export default React.memo(NavigationDesktop);
