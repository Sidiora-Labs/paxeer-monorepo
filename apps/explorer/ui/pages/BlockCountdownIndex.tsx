import { chakra, Box, Center } from '@chakra-ui/react';
import { useRouter } from 'next/router';
import React from 'react';

import { routeParams } from 'nextjs/routes';

import { useMultichainContext } from 'lib/contexts/multichain';
import { Heading } from 'toolkit/chakra/heading';
import ChainIcon from 'ui/shared/externalChains/ChainIcon';
import IconSvg from 'ui/shared/IconSvg';

import { PageFilterInput } from './PageFilterInput';

const BlockCountdownIndex = () => {
  const router = useRouter();
  const multichainContext = useMultichainContext();

  const handleFormSubmit = React.useCallback((event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();

    const formData = new FormData(event.currentTarget);
    const searchTerm = formData.get('search_term');
    if (typeof searchTerm === 'string' && searchTerm) {
      const url = routeParams({ pathname: '/block/countdown/[height]', query: { height: searchTerm } }, multichainContext);
      router.push(url, undefined, { shallow: true });
    }
  }, [ router, multichainContext ]);

  return (
    <Center h="100%" justifyContent={{ base: 'flex-start', lg: 'center' }} flexDir="column" textAlign="center" pt={{ base: 8, lg: 0 }}>
      <Box position="relative">
        <IconSvg
          name="block_countdown"
          color="icon.secondary"
          w={{ base: 'var(--chakra-spacing-40)', lg: 'var(--chakra-spacing-62\\.5)' }}
          h="auto"
          aspectRatio={ 241 / 185 }
        />
        { multichainContext?.chain && (
          <ChainIcon
            data={ multichainContext.chain }
            position="absolute"
            bottom={{ base: 'var(--chakra-spacing-3\\.75)', lg: 'var(--chakra-spacing-5)' }}
            left={{ base: 'var(--chakra-spacing-24)', lg: 'var(--chakra-spacing-37\\.5)' }}
            boxSize={{ lg: 'var(--chakra-spacing-15)' }}
            bgColor="bg.primary"
            borderRadius="full"
          />
        ) }
      </Box>
      <Heading
        level="1"
        mt={{ base: 3, lg: 6 }}
      >
        Block countdown
      </Heading>
      <Box mt={ 2 } textWrap="balance">
        The estimated time for a block to be created and added to the blockchain.
      </Box>
      <chakra.form
        onSubmit={ handleFormSubmit }
        w={{ base: '100%', lg: 'var(--chakra-spacing-80)' }}
        mt={{ base: 3, lg: 6 }}
      >
        <PageFilterInput
          placeholder="Search by block number"
          size="sm"
          type="number"
          name="search_term"
          inputProps={{ min: 0, step: 1, required: true }}
        />
      </chakra.form>
    </Center>
  );
};

export default React.memo(BlockCountdownIndex);
