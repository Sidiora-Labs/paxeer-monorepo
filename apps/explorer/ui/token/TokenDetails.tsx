import { Box, chakra, Flex, Grid } from '@chakra-ui/react';
import type { UseQueryResult } from '@tanstack/react-query';
import { useRouter } from 'next/router';
import React, { useCallback } from 'react';

import type { TokenInfo } from 'types/api/token';

import config from 'configs/app';
import type { ResourceError } from 'lib/api/resources';
import useApiQuery from 'lib/api/useApiQuery';
import { useMultichainContext } from 'lib/contexts/multichain';
import throwOnResourceLoadError from 'lib/errors/throwOnResourceLoadError';
import useIsMounted from 'lib/hooks/useIsMounted';
import { isConfidentialTokenType } from 'lib/token/tokenTypes';
import { TOKEN_COUNTERS } from 'stubs/token';
import { Link } from 'toolkit/chakra/link';
import { Skeleton } from 'toolkit/chakra/skeleton';
import type { TokenTabs } from 'ui/pages/Token';
import AppActionButton from 'ui/shared/AppActionButton/AppActionButton';
import useAppActionData from 'ui/shared/AppActionButton/useAppActionData';
import CopyToClipboard from 'ui/shared/CopyToClipboard';
import * as DetailedInfo from 'ui/shared/DetailedInfo/DetailedInfo';
import DetailedInfoSponsoredItem from 'ui/shared/DetailedInfo/DetailedInfoSponsoredItem';
import AddressEntity from 'ui/shared/entities/address/AddressEntity';
import { ScanKeyValue } from 'ui/shared/scan';
import AssetValue from 'ui/shared/value/AssetValue';

import TokenNftMarketplaces from './TokenNftMarketplaces';

interface CardProps {
  title: string;
  children: React.ReactNode;
}

const DetailsCard = ({ title, children }: CardProps) => (
  <Box data-token-card={ title }>
    <DetailedInfo.Group label={ title }>{ children }</DetailedInfo.Group>
  </Box>
);

interface Props {
  tokenQuery: UseQueryResult<TokenInfo, ResourceError<unknown>>;
}

const TokenDetails = ({ tokenQuery }: Props) => {
  const router = useRouter();
  const isMounted = useIsMounted();

  const hash = router.query.hash?.toString();

  const multichainContext = useMultichainContext();
  const chainSlug = multichainContext?.chain?.slug;

  const tokenCountersQuery = useApiQuery('general:token_counters', {
    pathParams: { hash },
    queryOptions: { enabled: Boolean(router.query.hash), placeholderData: TOKEN_COUNTERS },
  });

  const appActionData = useAppActionData(hash);

  const changeUrl = useCallback((tab: TokenTabs) => () => {
    router.push(
      chainSlug ?
        { pathname: '/chain/[chain_slug]/token/[hash]', query: { hash: hash || '', tab, chain_slug: chainSlug } } :
        { pathname: '/token/[hash]', query: { hash: hash || '', tab } },
      undefined,
      { shallow: true },
    );
  }, [ chainSlug, hash, router ]);

  const countersItem = useCallback((item: 'token_holders_count' | 'transfers_count') => {
    const itemValue = tokenCountersQuery.data?.[item];
    if (!itemValue) {
      return 'N/A';
    }
    if (itemValue === '0') {
      return itemValue;
    }

    const tab: TokenTabs = item === 'token_holders_count' ? 'holders' : 'token_transfers';

    return (
      <Link onClick={ changeUrl(tab) } loading={ tokenCountersQuery.isPlaceholderData }>
        { Number(itemValue).toLocaleString() }
      </Link>
    );
  }, [ tokenCountersQuery.data, tokenCountersQuery.isPlaceholderData, changeUrl ]);

  throwOnResourceLoadError(tokenQuery);

  if (!isMounted) {
    return null;
  }

  const {
    address_hash: addressHash,
    total_supply: totalSupply,
    decimals,
    symbol,
    type,
    zilliqa,
  } = tokenQuery.data || {};

  const overviewCard = (
    <DetailsCard title="TOKEN INFO">
      { type && !isConfidentialTokenType(type) && (
        <ScanKeyValue
          label="Max total supply"
          hint="The total amount of tokens issued"
          isLoading={ tokenQuery.isPlaceholderData }
          multiRow
        >
          <AssetValue
            amount={ totalSupply }
            asset={ <chakra.span maxW="50%" overflow="hidden" textOverflow="ellipsis"> { symbol }</chakra.span> }
            accuracy={ 3 }
            decimals={ decimals ?? '0' }
            loading={ tokenQuery.isPlaceholderData }
            w="100%"
          />
        </ScanKeyValue>
      ) }

      <ScanKeyValue
        label="Holders"
        hint="Number of accounts holding the token"
        isLoading={ tokenQuery.isPlaceholderData }
      >
        <Skeleton loading={ tokenCountersQuery.isPlaceholderData }>
          { countersItem('token_holders_count') }
        </Skeleton>
      </ScanKeyValue>

      <ScanKeyValue
        label="Transfers"
        hint="Number of transfers of the token"
        isLoading={ tokenQuery.isPlaceholderData }
      >
        <Skeleton loading={ tokenCountersQuery.isPlaceholderData }>
          { countersItem('transfers_count') }
        </Skeleton>
      </ScanKeyValue>
    </DetailsCard>
  );

  const otherInfoCard = (
    <DetailsCard title="CONTRACT INFO">
      { addressHash && (
        <ScanKeyValue
          label={ decimals ? `Token contract (with ${ decimals } decimals)` : 'Token contract' }
          hint="The contract this token is issued by, and the number of digits that come after its decimal point"
          isLoading={ tokenQuery.isPlaceholderData }
        >
          <Flex alignItems="center" minW={ 0 } data-token-contract>
            <AddressEntity address={{ hash: addressHash }} isLoading={ tokenQuery.isPlaceholderData } noCopy/>
            <CopyToClipboard text={ addressHash } isLoading={ tokenQuery.isPlaceholderData }/>
          </Flex>
        </ScanKeyValue>
      ) }

      { zilliqa?.zrc2_address_hash && (
        <ScanKeyValue
          label="ZRC-2 address"
          hint="ZRC-2 address of the token"
          isLoading={ tokenQuery.isPlaceholderData }
        >
          <Skeleton loading={ tokenQuery.isPlaceholderData } display="inline-block">
            <AddressEntity address={{ hash: zilliqa.zrc2_address_hash }} isLoading={ tokenQuery.isPlaceholderData }/>
          </Skeleton>
        </ScanKeyValue>
      ) }

      { type !== 'ERC-20' && (
        <TokenNftMarketplaces
          hash={ hash }
          isLoading={ tokenQuery.isPlaceholderData }
          appActionData={ appActionData }
          source="NFT collection"
        />
      ) }

      { (type !== 'ERC-20' && config.UI.views.nft.marketplaces.length === 0 && appActionData) && (
        <ScanKeyValue label="Dapp" hint="Link to the dapp">
          <AppActionButton data={ appActionData } height="30px" source="NFT collection"/>
        </ScanKeyValue>
      ) }

      <DetailedInfoSponsoredItem isLoading={ tokenQuery.isPlaceholderData }/>
    </DetailsCard>
  );

  return (
    <Grid
      data-token-details
      templateColumns="minmax(0, 1fr)"
      gap={ 8 }
      mb={ 6 }
      alignItems="start"
    >
      { overviewCard }
      { otherInfoCard }
    </Grid>
  );
};

export default React.memo(TokenDetails);
