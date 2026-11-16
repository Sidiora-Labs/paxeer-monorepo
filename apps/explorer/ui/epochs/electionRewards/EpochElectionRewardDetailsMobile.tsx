import { Box, Flex, Text } from '@chakra-ui/react';
import { useRouter } from 'next/router';
import React from 'react';

import type { CeloEpochDetails } from 'types/api/epochs';
import type { TokenInfo } from 'types/api/token';

import getQueryParamString from 'lib/router/getQueryParamString';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import useLazyLoadedList from 'ui/shared/pagination/useLazyLoadedList';
import TokenValue from 'ui/shared/value/TokenValue';

import { formatRewardType } from './utils';

interface Props {
  type: keyof CeloEpochDetails['aggregated_election_rewards'];
  token: TokenInfo;
}

const CeloEpochElectionRewardDetailsMobile = ({ type, token }: Props) => {
  const rootRef = React.useRef<HTMLDivElement>(null);

  const router = useRouter();
  const number = getQueryParamString(router.query.number);

  const { cutRef, query } = useLazyLoadedList({
    rootRef,
    resourceName: 'general:epoch_celo_election_rewards',
    pathParams: { number: number, reward_type: formatRewardType(type) },
    queryOptions: {
      refetchOnMount: false,
    },
  });

  return (
    <Flex
      ref={ rootRef }
      role="region"
      aria-label="Election reward details"
      tabIndex={ 0 }
      flexDir="column"
      rowGap={ 3 }
      p={ 4 }
      bgColor="surface"
      borderRadius="base"
      maxH="var(--chakra-spacing-80)"
      overflowY="auto"
      overscrollBehavior="contain"
      scrollbarGutter="stable"
    >

      { query.data?.pages
        .map((page) => page.items)
        .flat()
        .map((item, index) => {
          return (
            <Flex key={ index } flexDir="column" alignItems="flex-start" rowGap={ 1 } fontWeight="normal">
              <AddressEntity address={ item.account } noIcon w="100%"/>
              <TokenValue
                amount={ item.amount }
                token={ token }
                tokenEntityProps={{ noIcon: true, ml: 1 }}
                startElement={ <Box flexShrink={ 0 } color="text.secondary">got </Box> }
              />
              <Flex columnGap={ 1 } alignItems="center" flexWrap="wrap" w="100%">
                <Box flexShrink={ 0 } color="text.secondary">on behalf of</Box>
                <AddressEntity address={ item.associated_account } noIcon/>
              </Flex>
            </Flex>
          );
        }) }

      { query.isFetching && <ContentLoader maxW="var(--chakra-spacing-40\.5)" mt={ 3 }/> }

      { query.isError && <Text color="text.error" mt={ 3 }>Something went wrong. Unable to load next page.</Text> }

      <Box h="0" w="var(--chakra-spacing-24)" mt="calc(var(--chakra-spacing-3) * -1)" ref={ cutRef }/>
    </Flex>
  );
};

export default React.memo(CeloEpochElectionRewardDetailsMobile);
