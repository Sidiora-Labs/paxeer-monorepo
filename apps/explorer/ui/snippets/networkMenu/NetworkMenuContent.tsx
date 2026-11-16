import { VStack, Flex, Box, Text } from '@chakra-ui/react';
import React from 'react';

import type { FeaturedNetwork, NetworkGroup } from 'types/networks';

import config from 'configs/app';
import { Link } from 'toolkit/chakra/link';
import { PopoverBody, PopoverContent } from 'toolkit/chakra/popover';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TabsContent, TabsList, TabsRoot, TabsTrigger } from 'toolkit/chakra/tabs';

import NetworkMenuLink from './NetworkMenuLink';

interface Props {
  tabs: Array<NetworkGroup>;
  items?: Array<FeaturedNetwork>;
}

const NetworkMenuContent = ({ items, tabs }: Props) => {
  const selectedNetwork = items?.find(({ isActive }) => isActive);
  const defaultTab = tabs.find((tab) => selectedNetwork?.group === tab);

  const [ value, setValue ] = React.useState<NetworkGroup>(defaultTab ?? 'Mainnets');

  const handleTabChange = React.useCallback(({ value }: { value: string }) => {
    setValue(value as NetworkGroup);
  }, []);

  const content = (() => {
    if (!items || items.length === 0) {
      return (
        <>
          <Flex alignItems="center">
            <Flex
              h="var(--chakra-spacing-8)"
              w="calc(var(--chakra-spacing-24) + var(--chakra-spacing-2) + var(--chakra-spacing-1) / 4)"
              bgColor="surface"
              borderRadius="base"
              px={ 4 }
              py={ 2 }
            >
              <Skeleton loading h="var(--chakra-spacing-4)" w="100%"/>
            </Flex>
            <Skeleton
              loading
              h="var(--chakra-spacing-4)"
              w="calc(calc(var(--chakra-spacing-8) + var(--chakra-spacing-1) / 2) * 2)"
              mx={ 4 }
            />
            <Skeleton
              loading
              h="var(--chakra-spacing-4)"
              w="calc(var(--chakra-spacing-11) + var(--chakra-spacing-1) / 4)"
              mx={ 4 }
            />
          </Flex>
          <Flex mt={ 3 } flexDir="column" rowGap={ 2 }>
            <Flex mx={ 3 } my={ 2 } alignItems="center">
              <Skeleton loading h={ 'var(--chakra-spacing-7\\.5)' } w={ 'var(--chakra-spacing-7\\.5)' } borderRadius="full"/>
              <Skeleton loading h="var(--chakra-spacing-4)" w="var(--chakra-spacing-30)" ml={ 3 }/>
            </Flex>
            <Flex mx={ 3 } my={ 2 } alignItems="center">
              <Skeleton loading h={ 'var(--chakra-spacing-7\\.5)' } w={ 'var(--chakra-spacing-7\\.5)' } borderRadius="full"/>
              <Skeleton loading h="var(--chakra-spacing-4)" w="calc(var(--chakra-spacing-40) + var(--chakra-spacing-5))" ml={ 3 }/>
            </Flex>
            <Flex mx={ 3 } my={ 2 } alignItems="center">
              <Skeleton loading h={ 'var(--chakra-spacing-7\\.5)' } w={ 'var(--chakra-spacing-7\\.5)' } borderRadius="full"/>
              <Skeleton loading h="var(--chakra-spacing-4)" w={ 'var(--chakra-spacing-37\\.5)' } ml={ 3 }/>
            </Flex>
          </Flex>
        </>
      );
    }

    const viewAllLink = config.UI.featuredNetworks.allLink && (
      <Link
        href={ config.UI.featuredNetworks.allLink }
        external
        noIcon
        variant="secondary"
        my={ 2 }
        px={ 2 }
        fontSize="xs"
      >
        View all chains
      </Link>
    );

    if (tabs.length === 1) {
      return (
        <VStack as="ul" gap={ 1 } alignItems="stretch" overflowY="auto" maxH="var(--available-height)">
          { items
            .filter((network) => network.group === tabs[0])
            .map((network) => (
              <NetworkMenuLink
                key={ network.title }
                { ...network }
              />
            )) }
          { viewAllLink }
        </VStack>
      );
    }

    if (config.UI.featuredNetworks.mode === 'list') {
      return (
        <VStack overflowY="auto" maxH="var(--available-height)" alignItems="stretch" gap={ 3 }>
          { tabs.map((tab, index) => {
            return (
              <Box key={ tab }>
                <Text fontSize="sm" fontWeight="semibold" mb={ 2 }>{ tab }</Text>
                <VStack key={ tab } as="ul" gap={ 1 } alignItems="stretch">
                  { items
                    .filter((network) => network.group === tab)
                    .map((network) => (
                      <NetworkMenuLink
                        key={ network.title }
                        { ...network }
                      />
                    )) }
                  { index === tabs.length - 1 && viewAllLink }
                </VStack>
              </Box>
            );
          }) }
        </VStack>
      );
    }

    return (
      <TabsRoot
        variant="segmented"
        width="full"
        fitted
        size="sm"
        lazyMount
        value={ value }
        onValueChange={ handleTabChange }
      >
        { tabs.length > 1 && (
          <TabsList mb={ 2 } width="full">
            { tabs.map((tab) => (
              <TabsTrigger
                key={ tab }
                textTransform="capitalize"
                value={ tab }
              >
                { tab }
              </TabsTrigger>
            )) }
          </TabsList>
        ) }
        <Box>
          { tabs.map((tab) => (
            <TabsContent key={ tab } value={ tab } p={ 0 }>
              <VStack as="ul" gap={ 1 } alignItems="stretch" overflowY="auto" maxH="var(--available-height)">
                { items
                  .filter((network) => network.group === tab)
                  .map((network) => (
                    <NetworkMenuLink
                      key={ network.title }
                      { ...network }
                    />
                  )) }
                { viewAllLink }
              </VStack>
            </TabsContent>
          )) }
        </Box>
      </TabsRoot>
    );
  })();

  return (
    <PopoverContent
      w="calc(var(--chakra-spacing-65) + var(--chakra-spacing-7) + var(--chakra-spacing-1) / 2)"
      maxH="var(--available-height)"
      overflowY="auto"
    >
      <PopoverBody>
        { content }
      </PopoverBody>
    </PopoverContent>
  );
};

export default React.memo(NetworkMenuContent);
