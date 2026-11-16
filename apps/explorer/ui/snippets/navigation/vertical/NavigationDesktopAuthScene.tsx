import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import useProfileQuery from 'ui/snippets/auth/useProfileQuery';

import NavigationDesktop from './NavigationDesktop';

export default function NavigationDesktopAuthScene() {
  const profileQuery = useProfileQuery();

  return (
    <Flex
      w="100%"
      minH="100dvh"
      alignItems="stretch"
      aria-busy={ !profileQuery.isSuccess || profileQuery.isFetching }
    >
      <NavigationDesktop/>
      <Box bgColor="canvas" w="100%"/>
    </Flex>
  );
}
