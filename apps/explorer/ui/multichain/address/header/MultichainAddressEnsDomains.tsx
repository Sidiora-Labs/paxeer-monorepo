import { Box, chakra, Flex, Grid, Text } from '@chakra-ui/react';
import React from 'react';

import type * as multichain from '@blockscout/multichain-aggregator-types';

import { Button } from 'toolkit/chakra/button';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import { useDisclosure } from 'toolkit/hooks/useDisclosure';
import { PopoverBody, PopoverContent, PopoverRoot, PopoverTrigger } from 'ui/marketplace/NativePopover';
import { Tooltip } from 'ui/marketplace/NativeTooltip';
import EnsEntity from 'ui/shared/entities/ens/EnsEntity';
import IconSvg from 'ui/shared/IconSvg';
import useLazyLoadedList from 'ui/shared/pagination/useLazyLoadedList';

const DomainsGrid = ({ data }: { data: Array<multichain.Domain> }) => {
  return (
    <Grid
      w="full"
      minW={ 0 }
      templateColumns="repeat(auto-fit, minmax(min(100%, var(--chakra-spacing-40)), 1fr))"
      columnGap={ 3 }
      rowGap="calc(var(--chakra-spacing-1) * 3.5)"
      mt={ 2 }
    >
      { data.map((domain) => <EnsEntity key={ domain.name } domain={ domain.name } protocol={ domain.protocol } noLink/>) }
    </Grid>
  );
};

interface Props {
  isLoading: boolean;
  mainDomain: multichain.BasicDomainInfo | undefined;
  hash: string;
}

const MultichainAddressEnsDomains = ({ mainDomain, isLoading, hash }: Props) => {
  const rootRef = React.useRef<HTMLDivElement>(null);

  const popover = useDisclosure();

  const { cutRef, query: { data, isFetching, isError } } = useLazyLoadedList({
    rootRef,
    resourceName: 'multichainAggregator:address_domains',
    pathParams: { hash },
    queryOptions: {
      refetchOnMount: false,
    },
  });

  const items = data?.pages.map((page) => page.items).flat();

  if (!isLoading && (!items || items.length === 0)) {
    return null;
  }

  const totalRecords = data?.pages[0]?.items.length ?? 0;
  const totalRecordsPostfix = data?.pages[0]?.next_page_params ? '+' : '';
  const ownedDomains = (items ?? []).filter((domain) => domain.name !== mainDomain?.name);

  return (
    <PopoverRoot open={ popover.open } onOpenChange={ popover.onOpenChange }>
      <Tooltip content="List of names resolved or owned by this address" disabled={ popover.open } disableOnMobile closeOnClick>
        <div>
          <PopoverTrigger>
            <Button
              size="sm"
              variant="dropdown"
              aria-label="Address domains"
              fontWeight="medium"
              flexShrink={ 0 }
              columnGap={ 1 }
              loadingSkeleton={ isLoading }
            >
              <IconSvg name="ENS" boxSize={ 5 }/>
              <chakra.span hideBelow="xl">{ totalRecords }{ totalRecordsPostfix } Domain{ totalRecords > 1 ? 's' : '' }</chakra.span>
              <chakra.span hideFrom="xl">{ totalRecords }{ totalRecordsPostfix }</chakra.span>
            </Button>
          </PopoverTrigger>
        </div>
      </Tooltip>
      <PopoverContent
        maxW="calc(100dvw - var(--chakra-spacing-8))"
        overscrollBehavior="contain"
        w={{ lg: 'calc(var(--chakra-spacing-1) * 125)' }}
        maxH="var(--chakra-spacing-100)"
        overflowY="auto"
        ref={ rootRef }
      >
        <PopoverBody textStyle="sm" display="flex" flexDir="column" rowGap={ 5 } alignItems="flex-start">
          { mainDomain && (
            <Box w="100%">
              <chakra.span color="text.secondary" textStyle="xs">Primary*</chakra.span>
              <Flex alignItems="center" textStyle="md" mt={ 2 }>
                <EnsEntity
                  domain={ mainDomain.name }
                  protocol={ mainDomain.protocol }
                  fontWeight="semibold"
                  noCopy
                  noLink
                />
              </Flex>
            </Box>
          ) }
          { ownedDomains.length > 0 && (
            <Box w="full" minW={ 0 }>
              <chakra.span color="text.secondary" textStyle="xs">Owned by this address</chakra.span>
              <DomainsGrid data={ ownedDomains }/>
            </Box>
          ) }

          { isFetching && <ContentLoader maxW="calc(var(--chakra-spacing-1) * 50)" mt={ 3 }/> }

          { isError && <Text color="text.error" mt={ 3 }>Something went wrong. Unable to load next page.</Text> }

          <Box h="0" w="calc(var(--chakra-spacing-1) * 25)" ref={ cutRef }/>

          { mainDomain && (
            <chakra.span textStyle="xs" mt={ -1 }>
              *A domain name is not necessarily held by a person popularly associated with the name
            </chakra.span>
          ) }
        </PopoverBody>
      </PopoverContent>
    </PopoverRoot>
  );
};

export default React.memo(MultichainAddressEnsDomains);
