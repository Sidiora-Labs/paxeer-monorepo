import { Box, Grid, GridItem, Text } from '@chakra-ui/react';
import { useRouter } from 'next/router';
import React from 'react';

import type { CeloEpochDetails } from 'types/api/epochs';
import type { TokenInfo } from 'types/api/token';

import getQueryParamString from 'lib/router/getQueryParamString';
import { ContentLoader } from 'toolkit/components/loaders/ContentLoader';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import useLazyLoadedList from 'ui/shared/pagination/useLazyLoadedList';
import AssetValue from 'ui/shared/value/AssetValue';

import { formatRewardType, getRewardDetailsTableTitles } from './utils';

interface Props {
  type: keyof CeloEpochDetails['aggregated_election_rewards'];
  token: TokenInfo;
}

const CeloEpochElectionRewardDetailsDesktop = ({ type, token }: Props) => {
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

  const titles = getRewardDetailsTableTitles(type);

  return (
    <Box
      ref={ rootRef }
      role="region"
      aria-label="Election reward details"
      tabIndex={ 0 }
      p={ 4 }
      bgColor="surface"
      borderRadius="base"
      maxH="var(--chakra-spacing-80)"
      overflow="auto"
      overscrollBehavior="contain"
      scrollbarGutter="stable"
      fontWeight="normal"
      textStyle="body"
    >
      { query.data && (
        <Grid
          gridTemplateColumns="minmax(max-content, 1fr) max-content minmax(max-content, 1fr)"
          rowGap={ 5 }
          columnGap={ 5 }
        >
          <GridItem fontWeight="semibold" mb={ 1 } whiteSpace="nowrap">
            { titles[0] }
          </GridItem>
          <GridItem fontWeight="semibold" mb={ 1 } whiteSpace="nowrap" textAlign="right">
            Amount { token.symbol }
          </GridItem>
          <GridItem fontWeight="semibold" mb={ 1 } whiteSpace="nowrap">
            { titles[1] }
          </GridItem>

          { query.data?.pages
            .map((page) => page.items)
            .flat()
            .map((item, index) => {
              return (
                <React.Fragment key={ index }>
                  <GridItem>
                    <AddressEntity address={ item.account } noIcon truncation="constant"/>
                  </GridItem>
                  <GridItem textAlign="right" textStyle="number">
                    <AssetValue
                      amount={ item.amount }
                      decimals={ token.decimals }
                    />
                  </GridItem>
                  <GridItem>
                    <AddressEntity address={ item.associated_account } noIcon truncation="constant"/>
                  </GridItem>
                </React.Fragment>
              );
            }) }
        </Grid>
      ) }

      { query.isFetching && <ContentLoader maxW="var(--chakra-spacing-40\.5)" mt={ 3 }/> }

      { query.isError && <Text color="text.error" mt={ 3 }>Something went wrong. Unable to load next page.</Text> }

      <Box h="0" w="var(--chakra-spacing-24)" ref={ cutRef }/>
    </Box>
  );
};

export default React.memo(CeloEpochElectionRewardDetailsDesktop);
