import { Box, Flex } from '@chakra-ui/react';
import React from 'react';

import useProfileQuery from 'ui/snippets/auth/useProfileQuery';

import NavigationDesktop from './NavigationDesktop';

export default function NavigationDesktopAuthScene() {
  const profileQuery = useProfileQuery();

  return (
    <Flex
      w="100%"
      minH="100vh"
      alignItems="stretch"
      aria-busy={ !profileQuery.isSuccess || profileQuery.isFetching }
    >
      <NavigationDesktop/>
      <Box bgColor="lightpink" w="100%"/>
    </Flex>
  );
}
