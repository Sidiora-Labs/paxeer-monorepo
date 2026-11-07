import { Flex } from '@chakra-ui/react';
import BigNumber from 'bignumber.js';
import React from 'react';

import type { TokenInfo } from 'types/api/token';
import type { AggregatedTokenInfo } from 'types/client/multichainAggregator';

import config from 'configs/app';
import multichainConfig from 'configs/multichain';
import getItemIndex from 'lib/getItemIndex';
import { getTokenTypeName } from 'lib/token/tokenTypes';
import { Skeleton } from 'toolkit/chakra/skeleton';
import { TableCell, TableRow } from 'toolkit/chakra/table';
import { Tag } from 'toolkit/chakra/tag';
import AddressAddToWallet from 'ui/shared/address/AddressAddToWallet';
import TokenEntity from 'ui/shared/entities/token/TokenEntity';
import IconSvg from 'ui/shared/IconSvg';
import SimpleValue from 'ui/shared/value/SimpleValue';
import { DEFAULT_ACCURACY_USD } from 'ui/shared/value/utils';

export type TokensTableToken = TokenInfo | AggregatedTokenInfo;

// The token list carries no figure for every column the scan layout shows, and a figure the payload
// never sent reads as this dash rather than as a number nobody measured.
export const TOKEN_VALUE_PLACEHOLDER = '—';

const PRICE_ACCURACY = 4;

// The onchain capitalisation is the supply the contract reports taken at the price the list carries,
// which is the capitalisation the chain itself can answer for.
export function getOnchainMarketCap(token: TokensTableToken): BigNumber | undefined {
  const { total_supply: totalSupply, decimals, exchange_rate: exchangeRate } = token;

  if (!totalSupply || !exchangeRate) {
    return undefined;
  }

  if (decimals === null || decimals.trim() === '') {
    return undefined;
  }

  const precision = Number(decimals);

  if (!Number.isFinite(precision) || !Number.isInteger(precision) || precision < 0) {
    return undefined;
  }

  const supply = BigNumber(totalSupply).div(BigNumber(10).pow(precision));
  const marketCap = supply.multipliedBy(BigNumber(exchangeRate));

  return marketCap.isFinite() ? marketCap : undefined;
}

// The price beneath the fiat price is the same price denominated in the network coin: the fiat price
// of the token over the fiat price of the coin.
export function getNativePrice(exchangeRate: string | null, coinPrice: string | null | undefined): BigNumber | undefined {
  if (!exchangeRate || !coinPrice) {
    return undefined;
  }

  const price = BigNumber(coinPrice);

  if (price.isZero() || !price.isFinite()) {
    return undefined;
  }

  const nativePrice = BigNumber(exchangeRate).div(price);

  return nativePrice.isFinite() ? nativePrice : undefined;
}

export function getTokenVolume(token: TokensTableToken): BigNumber | undefined {
  if (!('volume_24h' in token) || (typeof token.volume_24h !== 'string' && typeof token.volume_24h !== 'number')) {
    return undefined;
  }
  const volume = BigNumber(token.volume_24h);
  return volume.isFinite() ? volume : undefined;
}

export const TokenValuePlaceholder = ({ isLoading }: { isLoading?: boolean }) => (
  <Skeleton loading={ isLoading } display="inline-block" color="text.muted" data-token-value-placeholder>
    <span>{ TOKEN_VALUE_PLACEHOLDER }</span>
  </Skeleton>
);

interface TokenChangePercentProps {
  value?: number | null;
  isLoading?: boolean;
}

export const TokenChangePercent = ({ value, isLoading }: TokenChangePercentProps) => {
  if (value === undefined || value === null || !Number.isFinite(value)) {
    return <TokenValuePlaceholder isLoading={ isLoading }/>;
  }

  const isUp = value >= 0;

  return (
    <Skeleton
      loading={ isLoading }
      display="inline-flex"
      alignItems="center"
      columnGap={ 1 }
      color={ isUp ? 'positive' : 'negative' }
      textStyle="number"
      data-token-change={ isUp ? 'up' : 'down' }
    >
      <IconSvg name="arrows/up-head" boxSize={ 3 } transform={ isUp ? undefined : 'rotate(180deg)' }/>
      <span>{ `${ Math.abs(value).toFixed(2) }%` }</span>
    </Skeleton>
  );
};

interface TokenFiatValueProps {
  value?: BigNumber;
  isLoading?: boolean;
  accuracy?: number;
}

export const TokenFiatValue = ({ value, isLoading, accuracy = DEFAULT_ACCURACY_USD }: TokenFiatValueProps) => {
  if (!value) {
    return <TokenValuePlaceholder isLoading={ isLoading }/>;
  }

  return <SimpleValue value={ value } loading={ isLoading } prefix="$" accuracy={ accuracy }/>;
};

type Props = {
  token: TokensTableToken;
  index: number;
  page: number;
  isLoading?: boolean;
  coinPrice?: string | null;
};

const bridgedTokensFeature = config.features.bridgedTokens;

const TokensTableItem = ({
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

  return (
    <TableRow className="group" data-token-row>
      <TableCell isNumeric>
        <Skeleton loading={ isLoading } textStyle="sm" color="text.muted" display="inline-block" minW={ 5 } data-token-rank>
          { getItemIndex(index, page) }
        </Skeleton>
      </TableCell>
      <TableCell>
        <Flex alignItems="center" columnGap={ 2 } overflow="hidden">
          <TokenEntity
            token={ token }
            chain={ chainInfo }
            isLoading={ isLoading }
            jointSymbol
            textStyle="sm"
            fontWeight="600"
            noLink={ type === 'NATIVE' }
          />
          <Flex flexShrink={ 0 } columnGap={ 1 } alignItems="center">
            <Tag loading={ isLoading } variant="outlined">{ getTokenTypeName(type, chainInfo?.app_config) }</Tag>
            { bridgedChainTag && <Tag loading={ isLoading } variant="outlined">{ bridgedChainTag }</Tag> }
            { type !== 'NATIVE' && (
              <AddressAddToWallet
                token={ token }
                isLoading={ isLoading }
                iconSize={ 5 }
                opacity={ 0 }
                _groupHover={{ opacity: 1 }}
                chainConfig={ chainInfo?.app_config }
              />
            ) }
          </Flex>
        </Flex>
      </TableCell>
      <TableCell isNumeric data-market-field="price">
        <Flex flexDir="column" alignItems="flex-end" rowGap={ 1 } data-token-price>
          <TokenFiatValue
            value={ exchangeRate ? BigNumber(exchangeRate) : undefined }
            accuracy={ PRICE_ACCURACY }
            isLoading={ isLoading }
          />
        </Flex>
      </TableCell>
      <TableCell isNumeric data-market-field="change">
        <TokenChangePercent isLoading={ isLoading }/>
      </TableCell>
      <TableCell isNumeric data-market-field="market-cap">
        <TokenFiatValue value={ marketCap ? BigNumber(marketCap) : undefined } isLoading={ isLoading }/>
      </TableCell>
      <TableCell isNumeric data-market-field="volume">
        <TokenFiatValue value={ getTokenVolume(token) } isLoading={ isLoading }/>
      </TableCell>
      <TableCell isNumeric>
        <Skeleton loading={ isLoading } display="inline-block" data-token-holders>
          { (holdersCount === null || holdersCount === undefined) ? TOKEN_VALUE_PLACEHOLDER : Number(holdersCount).toLocaleString() }
        </Skeleton>
      </TableCell>
    </TableRow>
  );
};

export default React.memo(TokensTableItem);
