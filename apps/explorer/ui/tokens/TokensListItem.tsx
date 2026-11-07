import { Flex, HStack } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import type { TokenInfo } from 'types/api/token';
import type { AggregatedTokenInfo } from 'types/client/multichainAggregator';

import config from 'configs/app';
import multichainConfig from 'configs/multichain';
import getItemIndex from 'lib/getItemIndex';
import { getTokenTypeName } from 'lib/token/tokenTypes';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { Tag } from 'toolkit/chakra/tag';
import AddressAddToWallet from 'ui/shared/address/AddressAddToWallet';
import TokenEntity from 'ui/shared/entities/token/TokenEntity';
import ListItemMobile from 'ui/shared/ListItemMobile/ListItemMobile';

import {
  getTokenVolume,
  TokenChangePercent,
  TokenFiatValue,
  TOKEN_VALUE_PLACEHOLDER,
} from './TokensTableItem';

type Props = {
  token: TokenInfo | AggregatedTokenInfo;
  index: number;
  page: number;
  isLoading?: boolean;
  coinPrice?: string | null;
};

const PRICE_ACCURACY = 4;

const bridgedTokensFeature = config.features.bridgedTokens;

const TokensListItem = ({
  token,
  page,
  index,
  isLoading,
}: Props) => {

  const {
    exchange_rate: exchangeRate,
    type,
    holders_count: holdersCount,
    circulating_market_cap: marketCap,
  } = token;

  const originalChainId = 'origin_chain_id' in token ? token.origin_chain_id : undefined;
  const chainInfos = 'chain_infos' in token ? token.chain_infos : undefined;

  const bridgedChainTag = bridgedTokensFeature.isEnabled ?
    bridgedTokensFeature.chains.find(({ id }) => id === originalChainId)?.short_title :
    undefined;

  const chainInfo = React.useMemo(() => {
    if (!chainInfos) {
      return;
    }

    const chainId = Object.keys(chainInfos)[0];
    const chain = multichainConfig()?.chains.find((chain) => chain.id === chainId);
    return chain;
  }, [ chainInfos ]);

  const renderRow = (label: string, value: React.ReactNode) => (
    <HStack
      css={{ '& > :last-child': { textStyle: 'number', textAlign: 'right' } }}
      gap={ 3 }
      justifyContent="space-between"
      w="100%"
      alignItems="flex-start"
      data-token-field={ label }
    >
      <Skeleton loading={ isLoading } textStyle="sm" fontWeight={ 500 } flexShrink={ 0 }>{ label }</Skeleton>
      { value }
    </HStack>
  );

  const price = (
    <Flex flexDir="column" alignItems="flex-end" rowGap={ 1 } data-token-price>
      <TokenFiatValue
        value={ exchangeRate ? BigNumber(exchangeRate) : undefined }
        accuracy={ PRICE_ACCURACY }
        isLoading={ isLoading }
      />
    </Flex>
  );

  const holders = (
    <Skeleton loading={ isLoading } textStyle="sm" color="text.secondary" data-token-holders>
      <span>{ (holdersCount === null || holdersCount === undefined) ? TOKEN_VALUE_PLACEHOLDER : Number(holdersCount).toLocaleString() }</span>
    </Skeleton>
  );

  return (
    <ListItemMobile rowGap={ 3 }>
      <Flex w="100%" alignItems="center" columnGap={ 2 }>
        <Skeleton loading={ isLoading } textStyle="sm" color="text.secondary" minW={ 5 } data-token-rank>
          <span>{ getItemIndex(index, page) }</span>
        </Skeleton>
        <TokenEntity
          token={ token }
          chain={ chainInfo }
          isLoading={ isLoading }
          jointSymbol
          w="auto"
          textStyle="sm"
          fontWeight="600"
          noLink={ type === 'NATIVE' }
        />
        <Flex ml="auto" flexShrink={ 0 } columnGap={ 1 } alignItems="center">
          <Tag loading={ isLoading } variant="outlined">{ getTokenTypeName(type, chainInfo?.app_config) }</Tag>
          { bridgedChainTag && <Tag loading={ isLoading } variant="outlined">{ bridgedChainTag }</Tag> }
          { type !== 'NATIVE' && <AddressAddToWallet token={ token } isLoading={ isLoading } chainConfig={ chainInfo?.app_config }/> }
        </Flex>
      </Flex>
      { renderRow('Price', price) }
      { renderRow('Change 24h', <TokenChangePercent isLoading={ isLoading }/>) }
      { renderRow('Market cap', <TokenFiatValue value={ marketCap ? BigNumber(marketCap) : undefined } isLoading={ isLoading }/>) }
      { renderRow('Volume 24h', <TokenFiatValue value={ getTokenVolume(token) } isLoading={ isLoading }/>) }
      { renderRow('Holders', holders) }
    </ListItemMobile>
  );
};

export default TokensListItem;
